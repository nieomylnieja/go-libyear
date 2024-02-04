package libyear

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/nieomylnieja/go-libyear/internal"
	"github.com/nieomylnieja/go-libyear/internal/mocks"
)

func TestCommand_calculateLibyear(t *testing.T) {
	mustParseTime := func(date string) time.Time {
		t.Helper()
		parsed, _ := time.Parse(time.DateOnly, date)
		return parsed
	}

	tests := []struct {
		CurrentDate string
		LatestDate  string
		Expected    float64
	}{
		{
			CurrentDate: "2021-05-12",
			LatestDate:  "2022-01-01",
			Expected:    0.64,
		},
		{
			CurrentDate: "2021-01-01",
			LatestDate:  "2022-01-01",
			Expected:    1.0,
		},
		{
			CurrentDate: "2021-02-28",
			LatestDate:  "2023-09-15",
			Expected:    2.55,
		},
		{
			CurrentDate: "2022-01-01",
			LatestDate:  "2022-01-01",
			Expected:    0.0,
		},
		{
			CurrentDate: "2021-05-12",
			LatestDate:  "2021-05-14",
			Expected:    0.01,
		},
		// Security fix for older version could cause a potential negative libyear.
		// We round it to 0 instead.
		{
			CurrentDate: "2021-05-15",
			LatestDate:  "2021-05-14",
			Expected:    0.0,
		},
	}
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual := calculateLibyear(
				&internal.Module{Time: mustParseTime(test.CurrentDate)},
				&internal.Module{Time: mustParseTime(test.LatestDate)})
			if test.Expected == 0 {
				assert.Zero(t, actual)
			} else {
				assert.InEpsilon(t, test.Expected, math.Round(actual*100)/100, 0.01)
			}
		})
	}
}

func TestCommand_calculateReleases(t *testing.T) {
	tests := []struct {
		CurrentVersion string
		LatestVersion  string
		Versions       []string
		Expected       int
	}{
		{
			CurrentVersion: "v0.9.0",
			LatestVersion:  "v1.0.0",
			Versions: []string{
				"v0.9.0",
				"v0.9.1",
				"v0.9.2",
				"v0.10.0",
				"v1.0.0",
				"v2.0.0-incompatible1",
				"v2.0.0-incompatible2",
			},
			Expected: 4,
		},
		{
			CurrentVersion: "v0.10.0",
			LatestVersion:  "v1.0.0",
			Versions: []string{
				"v0.9.1",
				"v0.9.2",
				"v0.10.0",
				"v1.0.0",
			},
			Expected: 1,
		},
		{
			CurrentVersion: "v1.0.0",
			LatestVersion:  "v1.0.0",
			Versions: []string{
				"v0.9.2",
				"v0.10.0",
				"v1.0.0",
			},
			Expected: 0,
		},
		{
			CurrentVersion: "v1.0.0",
			LatestVersion:  "v1.0.0",
			Versions: []string{
				"v1.0.0",
			},
			Expected: 0,
		},
	}
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			versions := make([]*semver.Version, len(test.Versions))
			for i, v := range test.Versions {
				versions[i] = semver.MustParse(v)
			}
			actual := calculateReleases(
				&internal.Module{Version: semver.MustParse(test.CurrentVersion)},
				&internal.Module{Version: semver.MustParse(test.LatestVersion)},
				versions)
			assert.Equal(t, test.Expected, actual)
		})
	}
}

func TestCommand_calculateVersions(t *testing.T) {
	tests := []struct {
		CurrentVersion string
		LatestVersion  string
		Expected       internal.VersionsDiff
	}{
		{
			CurrentVersion: "v0.0.0",
			LatestVersion:  "v0.0.0",
			Expected:       internal.VersionsDiff{0, 0, 0},
		},
		{
			CurrentVersion: "v0.9.0",
			LatestVersion:  "v0.9.0",
			Expected:       internal.VersionsDiff{0, 0, 0},
		},
		{
			CurrentVersion: "v0.0.1",
			LatestVersion:  "v0.0.1",
			Expected:       internal.VersionsDiff{0, 0, 0},
		},
		{
			CurrentVersion: "v1.0.0",
			LatestVersion:  "v1.0.0",
			Expected:       internal.VersionsDiff{0, 0, 0},
		},
		{
			CurrentVersion: "v1.9.0",
			LatestVersion:  "v2.10.2",
			Expected:       internal.VersionsDiff{1, 0, 0},
		},
		{
			CurrentVersion: "v1.9.0",
			LatestVersion:  "v3.8.0",
			Expected:       internal.VersionsDiff{2, 0, 0},
		},
		{
			CurrentVersion: "v1.9.0",
			LatestVersion:  "v1.12.0",
			Expected:       internal.VersionsDiff{0, 3, 0},
		},
		{
			CurrentVersion: "v1.9.0",
			LatestVersion:  "v1.12.3",
			Expected:       internal.VersionsDiff{0, 3, 0},
		},
		{
			CurrentVersion: "v1.9.0",
			LatestVersion:  "v1.9.12",
			Expected:       internal.VersionsDiff{0, 0, 12},
		},
	}
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual := calculateVersions(
				&internal.Module{Version: semver.MustParse(test.CurrentVersion)},
				&internal.Module{Version: semver.MustParse(test.LatestVersion)})
			assert.Equal(t, test.Expected, actual)
		})
	}
}

func TestCommand_Fallback(t *testing.T) {
	t.Run("don't call fallback if repo returns versions", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		modulesRepo := mocks.NewMockModulesRepo(ctrl)
		modulesRepo.EXPECT().
			GetVersions(gomock.Any()).
			Times(2).
			Return([]*semver.Version{
				semver.MustParse("v1.0.0"),
				semver.MustParse("v2.0.0"),
			}, nil)
		modulesRepo.EXPECT().
			GetInfo(gomock.Any(), gomock.Any()).
			Return(&internal.Module{}, nil)
		modulesRepo.EXPECT().
			GetLatestInfo(gomock.Any()).
			Return(&internal.Module{Version: semver.MustParse("v2.0.0")}, nil)

		versionsGetter := mocks.NewMockVersionsGetter(ctrl)
		versionsGetter.EXPECT().GetVersions(gomock.Any()).Times(0)

		cmd := Command{repo: modulesRepo, fallbackVersions: versionsGetter, opts: OptionShowReleases}

		err := cmd.runForModule(&internal.Module{Version: semver.MustParse("v1.0.0")})
		require.NoError(t, err)
	})
	t.Run("call fallback if repo doesn't return versions", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		modulesRepo := mocks.NewMockModulesRepo(ctrl)
		modulesRepo.EXPECT().
			GetVersions(gomock.Any()).
			Times(1).
			Return([]*semver.Version{}, nil)
		modulesRepo.EXPECT().
			GetInfo(gomock.Any(), gomock.Any()).
			Times(2).
			Return(&internal.Module{}, nil)
		modulesRepo.EXPECT().
			GetLatestInfo(gomock.Any()).
			Times(2).
			Return(&internal.Module{Version: semver.MustParse("v2.0.0")}, nil)

		versionsGetter := mocks.NewMockVersionsGetter(ctrl)
		versionsGetter.EXPECT().GetVersions(gomock.Any()).Times(0)

		cmd := Command{repo: modulesRepo, fallbackVersions: versionsGetter, opts: OptionShowReleases}

		// Don't call fallback if a version does not contain a prerelease.
		// We only expect GOPROXY to lack versions list when no semver version was released by a module.
		err := cmd.runForModule(&internal.Module{Version: semver.MustParse("v1.0.0")})
		require.NoError(t, err)

		modulesRepo.EXPECT().
			GetVersions(gomock.Any()).
			Times(1).
			Return([]*semver.Version{}, nil)
		versionsGetter.EXPECT().GetVersions(gomock.Any()).Times(1)

		err = cmd.runForModule(&internal.Module{Version: semver.MustParse("v0.0.0-20201216005158-039620a65673")})
		require.NoError(t, err)
	})
}

func TestCommand_GetLatestInfo(t *testing.T) {
	// Call is a single ModulesRepo.GetLatestInfo call.
	type Call struct {
		Input        string
		OutputModule *internal.Module
		OutputError  error
	}
	tests := map[string]struct {
		Calls          []Call
		ExpectedLatest *semver.Version
		Options        Option
	}{
		"don't check for next major": {
			Calls: []Call{
				{
					Input:        "github.com/golang/mock",
					OutputModule: &internal.Module{Version: semver.MustParse("v1.0.0")},
				},
			},
			ExpectedLatest: semver.MustParse("v1.0.0"),
		},
		"check for next major, not found": {
			Calls: []Call{
				{
					Input:        "github.com/golang/mock",
					OutputModule: &internal.Module{Version: semver.MustParse("v1.0.0")},
				},
				{
					Input:       "github.com/golang/mock/v2",
					OutputError: errors.New("no matching versions found"),
				},
			},
			ExpectedLatest: semver.MustParse("v1.0.0"),
			Options:        OptionFindLatestMajor,
		},
		"check for next major, found": {
			Calls: []Call{
				{
					Input:        "github.com/golang/mock",
					OutputModule: &internal.Module{Version: semver.MustParse("v1.0.0")},
				},
				{
					Input:        "github.com/golang/mock/v2",
					OutputModule: &internal.Module{Version: semver.MustParse("v2.0.0")},
				},
				{
					Input:       "github.com/golang/mock/v3",
					OutputError: errors.New("no matching versions found"),
				},
			},
			ExpectedLatest: semver.MustParse("v2.0.0"),
			Options:        OptionFindLatestMajor,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			modulesRepo := mocks.NewMockModulesRepo(ctrl)
			for _, call := range test.Calls {
				modulesRepo.EXPECT().
					GetLatestInfo(call.Input).
					Times(1).
					Return(call.OutputModule, call.OutputError)
			}
			cmd := Command{repo: modulesRepo, opts: test.Options}
			latest, err := cmd.getLatestInfo("github.com/golang/mock")

			require.NoError(t, err)
			assert.Equal(t, test.ExpectedLatest, latest.Version)
		})
	}
}
