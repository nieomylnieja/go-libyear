package internal_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver"
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

func TestGitHandler_GetInfo_PseudoVersion(t *testing.T) {
	ctrl := gomock.NewController(t)

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "github.com/nobl9/n9/proto")
	path := "github.com/nobl9/n9/proto"
	version := semver.MustParse("v0.0.0-20240708155017-1912cdeb168f")

	gitCmd := mocks.NewMockGitCmdI(ctrl)
	gitCmd.EXPECT().
		Clone("https://github.com/nobl9/n9.git", dir).
		Times(1).
		Return(nil)
	gitCmd.EXPECT().
		Checkout(dir, "1912cdeb168f").
		Times(1).
		Return(nil)
	git := internal.NewGitVCS(tmpDir, gitCmd)

	canHandle, err := git.CanHandle(path)
	require.NoError(t, err)
	require.True(t, canHandle)

	module, err := git.GetInfo(path, version)

	require.NoError(t, err)
	assert.Equal(t, path, module.Path)
	assert.Equal(t, version, module.Version)
	assert.Equal(t, time.Date(2024, 7, 8, 15, 50, 17, 0, time.UTC), module.Time)
}

func TestGitHandler_GetModFile_PseudoVersion(t *testing.T) {
	ctrl := gomock.NewController(t)

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "github.com/nobl9/n9/proto")
	path := "github.com/nobl9/n9/proto"
	version := semver.MustParse("v0.0.0-20240708155017-1912cdeb168f")
	goMod := "module github.com/nobl9/n9/proto\n"

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proto"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proto", "go.mod"), []byte(goMod), 0o600))

	gitCmd := mocks.NewMockGitCmdI(ctrl)
	gomock.InOrder(
		gitCmd.EXPECT().
			GetHeadBranchName(dir).
			Times(1).
			Return("main", nil),
		gitCmd.EXPECT().
			Checkout(dir, "main").
			Times(1).
			Return(nil),
		gitCmd.EXPECT().
			Pull(dir).
			Times(1).
			Return(nil),
		gitCmd.EXPECT().
			Checkout(dir, "1912cdeb168f").
			Times(1).
			Return(nil),
	)
	gitCmd.EXPECT().
		Clone(gomock.Any(), gomock.Any()).
		Times(0)
	git := internal.NewGitVCS(tmpDir, gitCmd)

	canHandle, err := git.CanHandle(path)
	require.NoError(t, err)
	require.True(t, canHandle)

	data, err := git.GetModFile(path, version)

	require.NoError(t, err)
	assert.Equal(t, goMod, string(data))
}

func TestGitHandler_GetVersions_SubmoduleTags(t *testing.T) {
	ctrl := gomock.NewController(t)

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "github.com/nobl9/n9/proto")
	path := "github.com/nobl9/n9/proto"
	tags := strings.NewReader(`2024-01-01 v9.0.0
2024-02-01 proto/v0.1.0
2024-03-01 proto/v1.2.0
2024-04-01 other/v5.0.0
`)

	gitCmd := mocks.NewMockGitCmdI(ctrl)
	gitCmd.EXPECT().
		Clone("https://github.com/nobl9/n9.git", dir).
		Times(1).
		Return(nil)
	gitCmd.EXPECT().
		ListTags(dir).
		Times(1).
		Return(tags, nil)
	git := internal.NewGitVCS(tmpDir, gitCmd)

	canHandle, err := git.CanHandle(path)
	require.NoError(t, err)
	require.True(t, canHandle)

	versions, err := git.GetVersions(path)

	require.NoError(t, err)
	assert.Equal(t, []*semver.Version{
		semver.MustParse("v0.1.0"),
		semver.MustParse("v1.2.0"),
	}, versions)
}

func TestGitHandler_GetLatestInfo_NoVersionsForPathReturnsHeadPseudoVersion(t *testing.T) {
	ctrl := gomock.NewController(t)

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "github.com/acme/widgets/proto")
	path := "github.com/acme/widgets/proto"
	commitTime := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)

	gitCmd := mocks.NewMockGitCmdI(ctrl)
	gomock.InOrder(
		gitCmd.EXPECT().
			Clone("https://github.com/acme/widgets.git", dir).
			Times(1).
			Return(nil),
		gitCmd.EXPECT().
			ListTags(dir).
			Times(1).
			Return(strings.NewReader("2024-01-01 v9.0.0\n"), nil),
		gitCmd.EXPECT().
			GetHeadBranchName(dir).
			Times(1).
			Return("main", nil),
		gitCmd.EXPECT().
			Checkout(dir, "main").
			Times(1).
			Return(nil),
		gitCmd.EXPECT().
			Pull(dir).
			Times(1).
			Return(nil),
		gitCmd.EXPECT().
			GetHeadInfo(dir).
			Times(1).
			Return("abcdef123456", commitTime, nil),
	)
	git := internal.NewGitVCS(tmpDir, gitCmd)

	canHandle, err := git.CanHandle(path)
	require.NoError(t, err)
	require.True(t, canHandle)

	module, err := git.GetLatestInfo(path)

	require.NoError(t, err)
	assert.Equal(t, path, module.Path)
	assert.Equal(t, semver.MustParse("v0.0.0-20240506070809-abcdef123456"), module.Version)
	assert.Equal(t, commitTime, module.Time)
}

func TestGitHandler_GetLatestInfo_NoVersionsForPath(t *testing.T) {
	t.Skip("legacy expectation superseded by pseudo-version fallback coverage")
	ctrl := gomock.NewController(t)

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "github.com/nobl9/n9/proto")
	path := "github.com/nobl9/n9/proto"

	gitCmd := mocks.NewMockGitCmdI(ctrl)
	gitCmd.EXPECT().
		Clone("https://github.com/nobl9/n9.git", dir).
		Times(1).
		Return(nil)
	gitCmd.EXPECT().
		ListTags(dir).
		Times(1).
		Return(strings.NewReader("2024-01-01 v9.0.0\n"), nil)
	git := internal.NewGitVCS(tmpDir, gitCmd)

	canHandle, err := git.CanHandle(path)
	require.NoError(t, err)
	require.True(t, canHandle)

	_, err = git.GetLatestInfo(path)

	require.EqualError(t, err, "no versions found for path github.com/nobl9/n9/proto")
}
