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
)

//go:generate go tool mockgen -destination mocks/git.go -package mocks -typed . GitCmdI

type GitCmdI interface {
	Clone(url, path string) error
	Pull(path string) error
	ListTags(path string) (io.Reader, error)
	Checkout(path, tag string) error
	GetHeadBranchName(path string) (string, error)
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
	URL     string
	DirPath string
	tags    []gitTag
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
		URL:     "https://" + root + ".git",
		DirPath: filepath.Join(g.cacheDir, path),
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
	if err := g.git.Checkout(repo.DirPath, version.Original()); err != nil {
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
	headBranchName, err := g.git.GetHeadBranchName(repo.DirPath)
	if err != nil {
		return err
	}
	if err := g.git.Checkout(repo.DirPath, headBranchName); err != nil {
		return fmt.Errorf("failed to checkout version %s of %s: %w", headBranchName, path, err)
	}
	return g.git.Pull(repo.DirPath)
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
		version, err := semver.NewVersion(split[1])
		if err != nil {
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
