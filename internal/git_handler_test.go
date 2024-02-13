package internal_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/nieomylnieja/go-libyear/internal"
	"github.com/nieomylnieja/go-libyear/internal/mocks"
)

func TestGitHandler_CanHandle_CloneIfNotPresent(t *testing.T) {
	ctrl := gomock.NewController(t)

	tmpDir, err := os.MkdirTemp(os.TempDir(), "go-libyear-test")
	require.NoError(t, err)
	dir := filepath.Join(tmpDir, "github.com/nieomylnieja/go-libyear")

	gitCmd := mocks.NewMockGitCmdI(ctrl)
	gitCmd.EXPECT().
		Clone("https://github.com/nieomylnieja/go-libyear.git", dir).
		Times(1).
		Return(nil)
	gitCmd.EXPECT().
		Pull(gomock.Any()).
		Times(0)
	git := internal.NewGitVCS(tmpDir, gitCmd)

	canHandle, err := git.CanHandle("github.com/nieomylnieja/go-libyear")
	require.NoError(t, err)
	assert.True(t, canHandle)
}

func TestGitHandler_CanHandle_PullIfCloned(t *testing.T) {
	ctrl := gomock.NewController(t)

	tmpDir, err := os.MkdirTemp(os.TempDir(), "go-libyear-test")
	require.NoError(t, err)
	dir := filepath.Join(tmpDir, "github.com/nieomylnieja/go-libyear")
	err = os.MkdirAll(dir, 0o700)
	require.NoError(t, err)

	gitCmd := mocks.NewMockGitCmdI(ctrl)
	gitCmd.EXPECT().
		Clone(gomock.Any(), gomock.Any()).
		Times(0)
	gitCmd.EXPECT().
		GetHeadBranchName(dir).
		Times(1).
		Return("main", nil)
	gitCmd.EXPECT().
		Checkout(dir, "main").
		Times(1).
		Return(nil)
	gitCmd.EXPECT().
		Pull(dir).
		Times(1).
		Return(nil)
	git := internal.NewGitVCS(tmpDir, gitCmd)

	canHandle, err := git.CanHandle("github.com/nieomylnieja/go-libyear")
	require.NoError(t, err)
	assert.True(t, canHandle)
}
