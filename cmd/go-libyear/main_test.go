package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

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

func TestCommandContextReturnsSignalCause(t *testing.T) {
	signalChannel := make(chan os.Signal, 1)
	ctx, cancel := newCommandContext(t.Context(), time.Hour, signalChannel, func() {})
	t.Cleanup(cancel)

	signalChannel <- syscall.SIGINT
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("command context was not canceled after SIGINT")
	}

	err := commandContextError(ctx, ctx.Err())
	signalErr, ok := errors.AsType[*commandSignalError](err)
	require.True(t, ok)
	assert.Equal(t, os.Signal(syscall.SIGINT), signalErr.signal)
	assert.Equal(t, 130, commandExitCode(err))
}

func TestCommandContextReturnsTimeoutCause(t *testing.T) {
	ctx, cancel := newCommandContext(t.Context(), time.Nanosecond, make(chan os.Signal), func() {})
	t.Cleanup(cancel)

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("command context did not time out")
	}

	err := commandContextError(ctx, ctx.Err())
	assert.EqualError(
		t,
		err,
		"1ns timeout exceeded, consider increasing the timeout value via --timeout flag",
	)
	assert.Equal(t, 1, commandExitCode(err))
}

func TestCommandExitCodeReturnsSignalStatus(t *testing.T) {
	tests := map[string]struct {
		err      error
		expected int
	}{
		"generic error": {
			err:      errors.New("failed"),
			expected: 1,
		},
		"SIGINT": {
			err:      fmt.Errorf("wrapped: %w", &commandSignalError{signal: syscall.SIGINT}),
			expected: 130,
		},
		"SIGTERM": {
			err:      fmt.Errorf("wrapped: %w", &commandSignalError{signal: syscall.SIGTERM}),
			expected: 143,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.expected, commandExitCode(tt.err))
		})
	}
}

func TestCommandSecondSignalTerminates(t *testing.T) {
	const helperEnvironment = "GO_LIBYEAR_SIGNAL_HELPER"
	if os.Getenv(helperEnvironment) == "1" {
		ctx, cancel := newProcessCommandContext(context.Background(), time.Hour)
		defer cancel()
		_, err := fmt.Fprintln(os.Stdout, "ready")
		require.NoError(t, err)
		<-ctx.Done()
		_, err = fmt.Fprintln(os.Stdout, "canceled")
		require.NoError(t, err)
		select {}
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	// #nosec G204,G702 -- the subprocess is the current test binary.
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCommandSecondSignalTerminates$")
	cmd.Env = append(os.Environ(), helperEnvironment+"=1")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.ProcessState != nil {
			return
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	scanner := bufio.NewScanner(stdout)
	require.True(t, scanner.Scan(), "helper did not become ready: %v", scanner.Err())
	assert.Equal(t, "ready", scanner.Text())
	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))
	require.True(t, scanner.Scan(), "helper did not report cancellation: %v", scanner.Err())
	assert.Equal(t, "canceled", scanner.Text())
	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))

	err = cmd.Wait()
	require.Error(t, err)
	require.NotErrorIs(t, context.Cause(ctx), context.DeadlineExceeded)
	waitStatus, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	require.True(t, ok)
	require.True(t, waitStatus.Signaled())
	assert.Equal(t, syscall.SIGINT, waitStatus.Signal())
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(path, 0o750))
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
}
