package libyear

import (
	"math"
	"strconv"
	"testing"
	"time"

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
