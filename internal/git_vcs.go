package internal

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
)

func NewGitVCS(cacheDir string) *GitVCS {
	return &GitVCS{
		cacheDir:   cacheDir,
		pathToRepo: make(map[string]*gitRepo),
	}
}

type GitVCS struct {
	cacheDir   string
	pathToRepo map[string]*gitRepo
	mu         sync.RWMutex
}

// gitRepo is not concurrently safe.
// It is assumed that a single goroutine handles a single gitRepo.
// If we ever need to support concurrent access to a single gitRepo,
// a mutex will have to guard access to tags slice.
type gitRepo struct {
	URL  string
	Path string
	tags []gitTag
}

type gitTag struct {
	Version *semver.Version
	Date    time.Time
}

var githubRegexp = regexp.MustCompile(`^(?P<root>github\.com/[\w.\-]+/[\w.\-]+)(/[\w.\-]+)*$`)

func (g *GitVCS) CanHandle(path string) (bool, error) {
	g.mu.RLock()
	_, ok := g.pathToRepo[path]
	g.mu.RUnlock()
	if ok {
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
		URL:  "https://" + root + ".git",
		Path: filepath.Join(g.cacheDir, path),
	}
	if err := g.initializeRepo(repo); err != nil {
		return false, err
	}
	g.pathToRepo[path] = repo
	return true, nil
}

func (g *GitVCS) Name() string {
	return "git"
}

func (g *GitVCS) GetVersions(path string) ([]*semver.Version, error) {
	repo := g.getRepoForPath(path)
	tags, err := repo.listAllTags()
	if err != nil {
		return nil, err
	}
	versions := make([]*semver.Version, 0, len(tags))
	for _, tag := range tags {
		versions = append(versions, tag.Version)
	}
	return versions, nil
}

func (g *GitVCS) GetModFile(path string, version *semver.Version) ([]byte, error) {
	panic("not implemented") // TODO: Implement
}

func (g *GitVCS) GetInfo(path string, version *semver.Version) (*Module, error) {
	panic("not implemented") // TODO: Implement
}

func (g *GitVCS) GetLatestInfo(path string) (*Module, error) {
	repo := g.getRepoForPath(path)
	latestTag, err := repo.execGitCmd("describe", "--tags", "--abbrev=0")
	if err != nil {
		return nil, err
	}
	repo.execGitCmd("checkout", latestTag.String())
	panic("not implemented") // TODO: Implement
}

func (g *GitVCS) getRepoForPath(path string) *gitRepo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.pathToRepo[path]
}

func (g *GitVCS) initializeRepo(repo *gitRepo) error {
	if _, statErr := os.Stat(repo.Path); os.IsNotExist(statErr) {
		_, err := repo.execGitCmd("clone", "--", repo.URL, repo.Path)
		return err
	}
	_, err := repo.execGitCmd("-C", repo.Path, "pull", "--ff-only")
	return err
}

func (g *gitRepo) listAllTags() ([]gitTag, error) {
	if len(g.tags) > 0 {
		return g.tags, nil
	}
	tagsReader, err := g.execGitCmd("for-each-ref", "--sort=authordate", "--format", "'%(authordate:short) %(refname:short)'", "refs/tags")
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(tagsReader)
	tags := make([]gitTag, 0)
	for scanner.Scan() {
		line := scanner.Text()
		split := strings.Split(line, " ")
		if len(split) != 2 {
			return nil, errors.Errorf("unexpected 'git for-each-ref' output line: %s, expected: '<date> <tag>'", line)
		}
		date, err := time.Parse(time.DateOnly, split[0])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse date for line: %s", line)
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
	g.tags = tags
	return tags, nil
}

func (g *gitRepo) execGitCmd(args ...string) (*bytes.Buffer, error) {
	return execCmd("git", append([]string{"-C", g.Path}, args...)...)
}
