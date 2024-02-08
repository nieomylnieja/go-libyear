package internal

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
)

func NewGitVCS(cacheDir string) *GitVCS {
	return &GitVCS{
		cacheDir:   cacheDir,
		pathToRepo: make(map[string]gitRepo),
	}
}

type GitVCS struct {
	cacheDir   string
	pathToRepo map[string]gitRepo
	mu         sync.RWMutex
}

type gitRepo struct {
	URL  string
	Path string
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
	repo := gitRepo{
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
	panic("not implemented") // TODO: Implement
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
	panic("not implemented") // TODO: Implement
}

func (g *GitVCS) getRepoForPath(path string) gitRepo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.pathToRepo[path]
}

func (g *GitVCS) initializeRepo(repo gitRepo) error {
	if _, statErr := os.Stat(repo.Path); os.IsNotExist(statErr) {
		_, err := repo.execGitCmd("clone", "--", repo.URL, repo.Path)
		return err
	}
	_, err := repo.execGitCmd("-C", repo.Path, "pull", "--ff-only")
	return err
}

func (g gitRepo) execGitCmd(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", g.Path}, args...)...)
	if cmd.Stderr != nil {
		return "", errors.New("exec: Stderr already set")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", errors.Errorf("Failed to execute '%s' command: %s", cmd, stderr.String())
	}
	return stdout.String(), nil
}
