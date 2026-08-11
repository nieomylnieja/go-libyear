package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_findGoModPath(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setup func(t *testing.T, root string) (start, expected, expectedErr string)
	}{
		"current directory": {
			setup: func(t *testing.T, root string) (string, string, string) {
				t.Helper()

				writeFile(t, filepath.Join(root, "go.mod"), "module example.com/current\n")
				return root, filepath.Join(root, "go.mod"), ""
			},
		},
		"parent directory": {
			setup: func(t *testing.T, root string) (string, string, string) {
				t.Helper()

				start := filepath.Join(root, "child", "nested")
				mkdirAll(t, start)
				writeFile(t, filepath.Join(root, "go.mod"), "module example.com/parent\n")
				return start, filepath.Join(root, "go.mod"), ""
			},
		},
		"git boundary containing go.mod": {
			setup: func(t *testing.T, root string) (string, string, string) {
				t.Helper()

				start := filepath.Join(root, "child", "nested")
				mkdirAll(t, start)
				writeFile(t, filepath.Join(root, ".git"), "gitdir: ../.git/worktrees/example\n")
				writeFile(t, filepath.Join(root, "go.mod"), "module example.com/repo\n")
				return start, filepath.Join(root, "go.mod"), ""
			},
		},
		"nested git boundary without go.mod": {
			setup: func(t *testing.T, root string) (string, string, string) {
				t.Helper()

				repo := filepath.Join(root, "repo")
				nestedRepo := filepath.Join(repo, "nested")
				start := filepath.Join(nestedRepo, "child")
				mkdirAll(t, start)
				writeFile(t, filepath.Join(repo, "go.mod"), "module example.com/outer\n")
				writeFile(t, filepath.Join(nestedRepo, ".git"), "gitdir: ../.git/modules/nested\n")
				return start, "", "could not find go.mod before reaching git boundary at " + nestedRepo
			},
		},
		"go.mod directory": {
			setup: func(t *testing.T, root string) (string, string, string) {
				t.Helper()

				mkdirAll(t, filepath.Join(root, "go.mod"))
				return root, "", filepath.Join(root, "go.mod") + " is a directory"
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			start, expected, expectedErr := tt.setup(t, t.TempDir())
			actual, err := findGoModPath(start)
			if expectedErr != "" {
				require.ErrorContains(t, err, expectedErr)
				assert.Empty(t, actual)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}
}

func Test_isTerminalReturnsFalseForRegularFile(t *testing.T) {
	t.Parallel()

	file, err := os.CreateTemp(t.TempDir(), "stderr-*")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	assert.False(t, isTerminal(file))
}

func Test_historyCommandUsesHistoryUsageText(t *testing.T) {
	t.Parallel()

	cmd := historyCommand()

	assert.Equal(t, historyUsageText, cmd.UsageText)
	assert.Contains(t, cmd.UsageText, "go-libyear history [flags] [path]")
	assert.NotContains(t, usageText, "go-libyear history [flags] [path]")
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(path, 0o750))
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
}
