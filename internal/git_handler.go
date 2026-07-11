package internal

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver"
	"golang.org/x/mod/module"
)

//go:generate go tool mockgen -destination mocks/git.go -package mocks -typed . GitCmdI

type GitCmdI interface {
	Clone(url, path string) error
	Pull(path string) error
	ListTags(path string) (io.Reader, error)
	Checkout(path, tag string) error
	GetHeadBranchName(path string) (string, error)
	GetHeadInfo(path string) (string, time.Time, error)
}

func NewGitVCS(cacheDir string, git GitCmdI) *GitHandler {
	return &GitHandler{
		git:        git,
		cacheDir:   cacheDir,
		pathToRepo: make(map[string]*gitRepo),
	}
}

// GitHandler is a module handler for git version control system.
type GitHandler struct {
	git        GitCmdI
	cacheDir   string
	pathToRepo map[string]*gitRepo
	mu         sync.RWMutex
}

// gitRepo is not concurrently safe.
// It is assumed that a single goroutine handles a single gitRepo.
// If we ever need to support concurrent access to a single gitRepo,
// a mutex will have to guard access to tags slice.
type gitRepo struct {
	URL       string
	DirPath   string
	tagPrefix string
	headRef   string
	tags      []gitTag
}

type gitTag struct {
	Version *semver.Version
	Date    time.Time
}

var githubRegexp = regexp.MustCompile(`^(?P<root>github\.com/[\w.\-]+/[\w.\-]+)(/[\w.\-]+)*$`)

func (g *GitHandler) CanHandle(path string) (bool, error) {
	if g.getRepoForPath(path) != nil {
		return true, nil
	}
	m := githubRegexp.FindStringSubmatch(path)
	if m == nil {
		return false, nil
	}
	var root string
	for i, name := range githubRegexp.SubexpNames() {
		if name == "root" {
			root = m[i]
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	repo := &gitRepo{
		URL:       "https://" + root + ".git",
		DirPath:   filepath.Join(g.cacheDir, path),
		tagPrefix: gitTagPrefix(root, path),
	}
	if err := g.initializeRepo(path, repo); err != nil {
		return false, err
	}
	g.pathToRepo[path] = repo
	return true, nil
}

func (g *GitHandler) Name() string {
	return "git"
}

func (g *GitHandler) GetVersions(path string) ([]*semver.Version, error) {
	repo := g.getRepoForPath(path)
	tags, err := g.listAllTags(repo)
	if err != nil {
		return nil, err
	}
	versions := make([]*semver.Version, 0, len(tags))
	for _, tag := range tags {
		versions = append(versions, tag.Version)
	}
	return versions, nil
}

func (g *GitHandler) GetModFile(path string, version *semver.Version) ([]byte, error) {
	moduleNameRegexp := regexp.MustCompile(fmt.Sprintf(`(?m)^module %s$`, path))
	repo := g.getRepoForPath(path)
	ref, err := repo.refForVersion(version)
	if err != nil {
		return nil, err
	}
	if err := g.git.Checkout(repo.DirPath, ref); err != nil {
		return nil, fmt.Errorf("failed to checkout version %s of %s: %w", version.Original(), path, err)
	}
	root, err := os.OpenRoot(repo.DirPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = root.Close()
	}()

	var goMod []byte
	if err := fs.WalkDir(root.FS(), ".", func(walkPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "vendor" {
			return fs.SkipDir
		}
		if entry.Name() != "go.mod" {
			return nil
		}
		data, err := root.ReadFile(walkPath)
		if err != nil {
			return err
		}
		if moduleNameRegexp.Match(data) {
			goMod = data
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if len(goMod) == 0 {
		return nil, fmt.Errorf("no go.mod file found for %s module", path)
	}
	return goMod, nil
}

func (g *GitHandler) GetInfo(path string, version *semver.Version) (*Module, error) {
	repo := g.getRepoForPath(path)
	if module.IsPseudoVersion(version.Original()) {
		info, err := g.getPseudoVersionInfo(repo, path, version)
		if err != nil {
			return nil, err
		}
		return info, nil
	}
	tags, err := g.listAllTags(repo)
	if err != nil {
		return nil, err
	}
	for _, tag := range tags {
		if tag.Version.String() == version.String() {
			return &Module{
				Path:    path,
				Version: tag.Version,
				Time:    tag.Date,
			}, nil
		}
	}
	return nil, fmt.Errorf("%s version not found for %s path", version, path)
}

func (g *GitHandler) GetLatestInfo(path string) (*Module, error) {
	repo := g.getRepoForPath(path)
	tags, err := g.listAllTags(repo)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return g.getHeadPseudoVersionInfo(repo, path)
	}
	latestTag := tags[len(tags)-1]
	return &Module{
		Path:    path,
		Version: latestTag.Version,
		Time:    latestTag.Date,
	}, nil
}

func (g *GitHandler) getRepoForPath(path string) *gitRepo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.pathToRepo[path]
}

func (g *GitHandler) initializeRepo(path string, repo *gitRepo) error {
	if _, statErr := os.Stat(repo.DirPath); os.IsNotExist(statErr) {
		return g.git.Clone(repo.URL, repo.DirPath)
	}
	return g.checkoutHeadRef(path, repo)
}

func (g *GitHandler) checkoutHeadRef(path string, repo *gitRepo) error {
	if repo.headRef == "" {
		headBranchName, err := g.git.GetHeadBranchName(repo.DirPath)
		if err != nil {
			return err
		}
		repo.headRef = headBranchName
	}
	if err := g.git.Checkout(repo.DirPath, repo.headRef); err != nil {
		return fmt.Errorf("failed to checkout version %s of %s: %w", repo.headRef, path, err)
	}
	return g.git.Pull(repo.DirPath)
}

func (g *GitHandler) getHeadPseudoVersionInfo(repo *gitRepo, path string) (*Module, error) {
	if err := g.checkoutHeadRef(path, repo); err != nil {
		return nil, err
	}
	revision, commitTime, err := g.git.GetHeadInfo(repo.DirPath)
	if err != nil {
		return nil, err
	}
	version, err := semver.NewVersion(module.PseudoVersion(pseudoVersionMajor(path), "", commitTime, revision))
	if err != nil {
		return nil, err
	}
	return &Module{
		Path:    path,
		Version: version,
		Time:    commitTime,
	}, nil
}

func pseudoVersionMajor(path string) string {
	_, pathMajor, ok := module.SplitPathVersion(path)
	if !ok || pathMajor == "" {
		return "v0"
	}
	return strings.TrimPrefix(pathMajor, "/")
}

func (g *GitHandler) getPseudoVersionInfo(
	repo *gitRepo,
	path string,
	version *semver.Version,
) (*Module, error) {
	versionOriginal := version.Original()
	revision, err := module.PseudoVersionRev(versionOriginal)
	if err != nil {
		return nil, err
	}
	if err := g.git.Checkout(repo.DirPath, revision); err != nil {
		return nil, fmt.Errorf("failed to checkout revision %s of %s: %w", revision, path, err)
	}
	versionTime, err := module.PseudoVersionTime(versionOriginal)
	if err != nil {
		return nil, err
	}
	return &Module{
		Path:    path,
		Version: version,
		Time:    versionTime,
	}, nil
}

func (g *GitHandler) listAllTags(repo *gitRepo) ([]gitTag, error) {
	if len(repo.tags) > 0 {
		return repo.tags, nil
	}
	tagsReader, err := g.git.ListTags(repo.DirPath)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(tagsReader)
	tags := make([]gitTag, 0)
	for scanner.Scan() {
		line := scanner.Text()
		split := strings.Split(line, " ")
		if len(split) != 2 {
			return nil, fmt.Errorf("unexpected 'git for-each-ref' output line: %s, expected: '<date> <tag>'", line)
		}
		date, err := time.Parse(time.DateOnly, split[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse date for line: %s: %w", line, err)
		}
		version, ok := repo.versionFromTag(split[1])
		if !ok {
			continue
		}
		tags = append(tags, gitTag{
			Version: version,
			Date:    date,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Version.LessThan(tags[j].Version) })
	repo.tags = tags
	return tags, nil
}

func (r *gitRepo) refForVersion(version *semver.Version) (string, error) {
	versionOriginal := version.Original()
	if module.IsPseudoVersion(versionOriginal) {
		return module.PseudoVersionRev(versionOriginal)
	}
	return r.tagPrefix + versionOriginal, nil
}

func (r *gitRepo) versionFromTag(tag string) (*semver.Version, bool) {
	if r.tagPrefix != "" {
		if !strings.HasPrefix(tag, r.tagPrefix) {
			return nil, false
		}
		tag = strings.TrimPrefix(tag, r.tagPrefix)
	} else if strings.Contains(tag, "/") {
		return nil, false
	}
	version, err := semver.NewVersion(tag)
	return version, err == nil
}

func gitTagPrefix(root, modulePath string) string {
	modulePrefix, _, ok := module.SplitPathVersion(modulePath)
	if !ok {
		modulePrefix = modulePath
	}
	subdir := strings.TrimPrefix(modulePrefix, root)
	subdir = strings.TrimPrefix(subdir, "/")
	if subdir == "" {
		return ""
	}
	return subdir + "/"
}
