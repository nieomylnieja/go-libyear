package libyear

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSourceReadHistoryReadsCommittedGoModAtTimestamp(t *testing.T) {
	repo := t.TempDir()
	runTestGit(t, repo, nil, "init", "--quiet")
	runTestGit(t, repo, nil, "config", "user.email", "test@example.com")
	runTestGit(t, repo, nil, "config", "user.name", "go-libyear test")

	goModPath := filepath.Join(repo, "go.mod")
	writeTestFile(t, goModPath, `module example.com/app

go 1.22

require example.com/dep v1.0.0
`)
	runTestGit(t, repo, nil, "add", "go.mod")
	runTestGit(t, repo, gitDateEnv("2022-01-01T00:00:00Z"), "commit", "--quiet", "-m", "initial go.mod")

	writeTestFile(t, goModPath, `module example.com/app

go 1.22

require example.com/dep v1.1.0
`)
	runTestGit(t, repo, nil, "add", "go.mod")
	runTestGit(t, repo, gitDateEnv("2022-08-01T00:00:00Z"), "commit", "--quiet", "-m", "update go.mod")

	source := FileSource{Path: goModPath}
	first, err := source.readHistory(t.Context(), time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Contains(t, string(first), "require example.com/dep v1.0.0")

	second, err := source.readHistory(t.Context(), time.Date(2022, 8, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Contains(t, string(second), "require example.com/dep v1.1.0")
}

func TestFileSourceReadHistoryReportsMissingRevision(t *testing.T) {
	repo := t.TempDir()
	runTestGit(t, repo, nil, "init", "--quiet")
	runTestGit(t, repo, nil, "config", "user.email", "test@example.com")
	runTestGit(t, repo, nil, "config", "user.name", "go-libyear test")

	goModPath := filepath.Join(repo, "go.mod")
	writeTestFile(t, goModPath, `module example.com/app

go 1.22
`)
	runTestGit(t, repo, nil, "add", "go.mod")
	runTestGit(t, repo, gitDateEnv("2022-01-01T00:00:00Z"), "commit", "--quiet", "-m", "initial go.mod")

	source := FileSource{Path: goModPath}
	_, err := source.readHistory(t.Context(), time.Date(2021, 12, 31, 0, 0, 0, 0, time.UTC))

	require.EqualError(t, err, "no git revision found for go.mod at or before 2021-12-31T00:00:00Z")
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
}

func runTestGit(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	// #nosec G204
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed:\n%s", args, output)
}

func gitDateEnv(timestamp string) []string {
	return []string{
		"GIT_AUTHOR_DATE=" + timestamp,
		"GIT_COMMITTER_DATE=" + timestamp,
	}
}
