package libyear

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/assert"

	"github.com/nieomylnieja/go-libyear/internal"
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
		Versions       []string
		Expected       int
	}{
		{
			CurrentVersion: "v0.9.0",
			Versions: []string{
				"v0.9.0",
				"v0.9.1",
				"v0.9.2",
				"v0.10.0",
				"v1.0.0",
			},
			Expected: 4,
		},
		{
			CurrentVersion: "v0.10.0",
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
			Versions: []string{
				"v0.9.2",
				"v0.10.0",
				"v1.0.0",
			},
			Expected: 0,
		},
		{
			CurrentVersion: "v1.0.0",
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
		modulesRepo := &mockModulesRepo{
			getVersionsResponse: []*semver.Version{
				semver.MustParse("v1.0.0"),
				semver.MustParse("v2.0.0"),
			},
			getInfoResponse: &internal.Module{},
		}
		versionsGetter := &mockVersionsGetter{}
		cmd := Command{repo: modulesRepo, fallbackVersions: versionsGetter}

		err := cmd.runForModule(&internal.Module{Version: semver.MustParse("v1.0.0")})

		require.NoError(t, err)
		assert.Equal(t, 1, modulesRepo.getVersionsCalledTimes)
		assert.Equal(t, 0, versionsGetter.calledTimes)
	})
	t.Run("call fallback if repo doesn't return versions", func(t *testing.T) {
		modulesRepo := &mockModulesRepo{
			getVersionsResponse: []*semver.Version{},
			getInfoResponse:     &internal.Module{},
		}
		versionsGetter := &mockVersionsGetter{}
		cmd := Command{repo: modulesRepo, fallbackVersions: versionsGetter}

		// Don't call fallback if a version does not contain a prerelease.
		// We only expect GOPROXY to lack versions list when no semver version was released by a module.
		err := cmd.runForModule(&internal.Module{Version: semver.MustParse("v1.0.0")})

		require.NoError(t, err)
		assert.Equal(t, 1, modulesRepo.getVersionsCalledTimes)
		assert.Equal(t, 0, versionsGetter.calledTimes)

		err = cmd.runForModule(&internal.Module{Version: semver.MustParse("v0.0.0-20201216005158-039620a65673")})

		require.NoError(t, err)
		assert.Equal(t, 2, modulesRepo.getVersionsCalledTimes)
		assert.Equal(t, 1, versionsGetter.calledTimes)
	})
}

type mockModulesRepo struct {
	getVersionsCalledTimes int
	getVersionsResponse    []*semver.Version
	getInfoResponse        *internal.Module
}

func (m *mockModulesRepo) GetVersions(string) ([]*semver.Version, error) {
	m.getVersionsCalledTimes++
	return m.getVersionsResponse, nil
}

func (m *mockModulesRepo) GetModFile(string, *semver.Version) ([]byte, error) {
	panic("implement me")
}

func (m *mockModulesRepo) GetInfo(string, *semver.Version) (*internal.Module, error) {
	return m.getInfoResponse, nil
}

func (m *mockModulesRepo) GetLatestInfo(string) (*internal.Module, error) {
	panic("implement me")
}

type mockVersionsGetter struct {
	calledTimes int
}

func (m *mockVersionsGetter) GetVersions(string) ([]*semver.Version, error) {
	m.calledTimes++
	return nil, nil
}
