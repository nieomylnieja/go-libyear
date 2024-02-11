package internal

import (
	"io"
	"regexp"

	"github.com/pkg/errors"
)

// GitCmd is a wrapper over git command calls.
type GitCmd struct{}

func (g GitCmd) Clone(url, path string) error {
	_, err := execCmd("git", "clone", "--", url, path)
	return err
}

func (g GitCmd) Pull(path string) error {
	_, err := execCmd("git", "-C", path, "pull", "--ff-only")
	return err
}

func (g GitCmd) ListTags(path string) (io.Reader, error) {
	return execCmd(
		"git", "-C", path,
		"for-each-ref",
		"--sort=authordate",
		"--format=%(if)%(authordate)%(then)%(authordate:short)%(else)%(taggerdate:short)%(end) %(refname:short)",
		"refs/tags")
}

func (g GitCmd) Checkout(path, tag string) error {
	_, err := execCmd("git", "-C", path, "checkout", tag)
	return err
}

var gitHeadBranchRegexp = regexp.MustCompile(`(?m)^\s*origin/HEAD\s*->\s*origin/(?P<branch>.*)\s*$`)

func (g GitCmd) GetHeadBranchName(path string) (string, error) {
	buf, err := execCmd("git", "-C", path, "branch", "-rl", "*/HEAD")
	if err != nil {
		return "", err
	}
	return getHeadBranchName(buf)
}

func getHeadBranchName(reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	m := gitHeadBranchRegexp.FindStringSubmatch(string(data))
	if m == nil {
		return "", errors.Errorf("failed to parse git head branch: '%q'", string(data))
	}
	var branch string
	for i, name := range gitHeadBranchRegexp.SubexpNames() {
		if name == "branch" {
			branch = m[i]
		}
	}
	if branch == "" {
		return "", errors.Errorf("failed extract git head branch from '%q' using '%s' regexp",
			string(data), gitHeadBranchRegexp)
	}
	return branch, err
}
