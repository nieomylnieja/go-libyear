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
				mustParseTime(t, test.CurrentDate),
				mustParseTime(t, test.LatestDate))
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

func TestCommand_GetLatestInfo(t *testing.T) {
	// Call is a single ModulesRepo.GetLatestInfo call.
	type Call struct {
		Input        string
		OutputModule *internal.Module
		OutputError  error
	}
	tests := map[string]struct {
		Input          string
		Calls          []Call
		ExpectedLatest *semver.Version
		Options        Option
	}{
		"don't check for next major": {
			Input: "github.com/golang/mock",
			Calls: []Call{
				{
					Input:        "github.com/golang/mock",
					OutputModule: &internal.Module{Version: semver.MustParse("v1.0.0")},
				},
			},
			ExpectedLatest: semver.MustParse("v1.0.0"),
		},
		"next major not found": {
			Input: "github.com/golang/mock",
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
		"current v0, found v2": {
			Input: "github.com/golang/mock",
			Calls: []Call{
				{
					Input:        "github.com/golang/mock",
					OutputModule: &internal.Module{Version: semver.MustParse("v0.1.0")},
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
		"current v1, found v2": {
			Input: "github.com/golang/mock",
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
		"current v2, found v3": {
			Input: "github.com/golang/mock/v2",
			Calls: []Call{
				{
					Input:        "github.com/golang/mock/v2",
					OutputModule: &internal.Module{Version: semver.MustParse("v2.0.0")},
				},
				{
					Input:        "github.com/golang/mock/v3",
					OutputModule: &internal.Module{Version: semver.MustParse("v3.0.0")},
				},
				{
					Input:       "github.com/golang/mock/v4",
					OutputError: errors.New("no matching versions found"),
				},
			},
			ExpectedLatest: semver.MustParse("v3.0.0"),
			Options:        OptionFindLatestMajor,
		},
		"current v2, found v4": {
			Input: "github.com/golang/mock/v2",
			Calls: []Call{
				{
					Input:        "github.com/golang/mock/v2",
					OutputModule: &internal.Module{Version: semver.MustParse("v2.0.0")},
				},
				{
					Input:        "github.com/golang/mock/v3",
					OutputModule: &internal.Module{Version: semver.MustParse("v3.0.0")},
				},
				{
					Input:        "github.com/golang/mock/v4",
					OutputModule: &internal.Module{Version: semver.MustParse("v4.0.0")},
				},
				{
					Input:       "github.com/golang/mock/v5",
					OutputError: errors.New("no matching versions found"),
				},
			},
			ExpectedLatest: semver.MustParse("v4.0.0"),
			Options:        OptionFindLatestMajor,
		},
		// Happens with old, pre-module projects.
		// Back then there was no requirement to have major version suffix in project path.
		"version greater than 2, but module path is the same": {
			Input: "github.com/go-playground/validator",
			Calls: []Call{
				{
					Input:        "github.com/go-playground/validator",
					OutputModule: &internal.Module{Version: semver.MustParse("v9.4.0")},
				},
				{
					Input:        "github.com/go-playground/validator/v10",
					OutputModule: &internal.Module{Version: semver.MustParse("v10.0.0")},
				},
				{
					Input:       "github.com/go-playground/validator/v11",
					OutputError: errors.New("no matching versions found"),
				},
			},
			ExpectedLatest: semver.MustParse("v10.0.0"),
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
			cmd := Command{opts: test.Options}
			latest, err := cmd.getLatestInfo(
				&internal.Module{Path: test.Input},
				modulesRepo)

			require.NoError(t, err)
			assert.Equal(t, test.ExpectedLatest, latest.Version)
		})
	}
}

func TestCommand_GetVersions(t *testing.T) {
	// Call is a single ModulesRepo.GetLatestInfo call.
	type Call struct {
		Input          string
		OutputVersions []*semver.Version
	}
	tests := map[string]struct {
		Latest       *internal.Module
		Calls        []Call
		Expected     []*semver.Version
		CallFallback bool
	}{
		"v1": {
			Latest: &internal.Module{
				Version:  semver.MustParse("v1.1.0"),
				AllPaths: []string{"github.com/golang/mock"},
			},
			Calls: []Call{
				{
					Input: "github.com/golang/mock",
					OutputVersions: []*semver.Version{
						semver.MustParse("v0.1.0"),
						semver.MustParse("v1.0.0"),
						semver.MustParse("v1.1.0"),
					},
				},
			},
			Expected: []*semver.Version{
				semver.MustParse("v0.1.0"),
				semver.MustParse("v1.0.0"),
				semver.MustParse("v1.1.0"),
			},
		},
		"v2 to v3": {
			Latest: &internal.Module{
				Version: semver.MustParse("v3.0.1"),
				AllPaths: []string{
					"github.com/golang/mock/v2",
					"github.com/golang/mock/v3",
				},
			},
			Calls: []Call{
				{
					Input: "github.com/golang/mock/v2",
					OutputVersions: []*semver.Version{
						semver.MustParse("v2.0.0"),
						semver.MustParse("v2.1.0"),
					},
				},
				{
					Input: "github.com/golang/mock/v3",
					OutputVersions: []*semver.Version{
						semver.MustParse("v3.0.0"),
						semver.MustParse("v3.0.1"),
					},
				},
			},
			Expected: []*semver.Version{
				semver.MustParse("v2.0.0"),
				semver.MustParse("v2.1.0"),
				semver.MustParse("v3.0.0"),
				semver.MustParse("v3.0.1"),
			},
		},
		"v1 to v2": {
			Latest: &internal.Module{
				Version: semver.MustParse("v2.0.1"),
				AllPaths: []string{
					"github.com/golang/mock",
					"github.com/golang/mock/v2",
				},
			},
			Calls: []Call{
				{
					Input: "github.com/golang/mock",
					OutputVersions: []*semver.Version{
						semver.MustParse("v0.1.0"),
						semver.MustParse("v1.1.0"),
					},
				},
				{
					Input: "github.com/golang/mock/v2",
					OutputVersions: []*semver.Version{
						semver.MustParse("v2.0.0"),
						semver.MustParse("v2.0.1"),
					},
				},
			},
			Expected: []*semver.Version{
				semver.MustParse("v0.1.0"),
				semver.MustParse("v1.1.0"),
				semver.MustParse("v2.0.0"),
				semver.MustParse("v2.0.1"),
			},
		},
		"prerelease": {
			Latest: &internal.Module{
				Version: semver.MustParse("v0.0.0-20201216005158-039620a65673"),
				AllPaths: []string{
					"github.com/golang/mock",
				},
			},
			CallFallback: true,
			Calls: []Call{
				{
					Input: "github.com/golang/mock",
					OutputVersions: []*semver.Version{
						semver.MustParse("v0.0.0-20201116005158-029620a65673"),
						semver.MustParse("v0.0.0-20201216005158-039620a65673"),
					},
				},
			},
			Expected: []*semver.Version{
				semver.MustParse("v0.0.0-20201116005158-029620a65673"),
				semver.MustParse("v0.0.0-20201216005158-039620a65673"),
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			modulesRepo := mocks.NewMockModulesRepo(ctrl)
			versionsGetter := mocks.NewMockVersionsGetter(ctrl)
			for _, call := range test.Calls {
				if test.CallFallback {
					modulesRepo.EXPECT().
						GetVersions(call.Input).
						Times(1).
						Return(nil, nil)
					versionsGetter.EXPECT().
						GetVersions(call.Input).
						Times(1).
						Return(call.OutputVersions, nil)
				} else {
					modulesRepo.EXPECT().
						GetVersions(call.Input).
						Times(1).
						Return(call.OutputVersions, nil)
					versionsGetter.EXPECT().
						GetVersions(gomock.Any()).
						Times(0)
				}
			}
			cmd := Command{
				fallbackVersions: versionsGetter,
				vcs:              &VCSRegistry{},
			}
			versions, err := cmd.getAllVersions(modulesRepo, test.Latest)

			require.NoError(t, err)
			assert.Equal(t, test.Expected, versions)
		})
	}
}

func TestCommand_HandleFixVersionsWhenNewMajorIsAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	currentLatest := &internal.Module{
		Path:    "github.com/go-playground/validator",
		Version: semver.MustParse("v9.4.1+incompatible"),
		Time:    mustParseTime(t, "2023-01-08"),
	}
	modulesRepo := mocks.NewMockModulesRepo(ctrl)
	modulesRepo.EXPECT().
		GetLatestInfo("github.com/go-playground/validator").
		Times(1).
		Return(currentLatest, nil)
	modulesRepo.EXPECT().
		GetLatestInfo("github.com/go-playground/validator/v10").
		Times(1).
		Return(&internal.Module{
			Path:    "github.com/go-playground/validator/v10",
			Version: semver.MustParse("v10.1.0"),
			Time:    mustParseTime(t, "2023-01-10"),
		}, nil)
	modulesRepo.EXPECT().
		GetLatestInfo("github.com/go-playground/validator/v11").
		Times(1).
		Return(nil, errors.New("no matching versions found"))
	modulesRepo.EXPECT().
		GetVersions("github.com/go-playground/validator/v10").
		Times(1).
		// Not sorted on purpose.
		Return([]*semver.Version{
			semver.MustParse("v10.0.1"),
			semver.MustParse("v10.0.0"),
			semver.MustParse("v10.1.0"),
		}, nil)
	modulesRepo.EXPECT().
		GetInfo("github.com/go-playground/validator/v10", semver.MustParse("v10.0.0")).
		Times(1).
		Return(&internal.Module{
			Version: semver.MustParse("v10.0.0"),
			Time:    mustParseTime(t, "2023-01-01"),
		}, nil)
	cmd := Command{
		repo: modulesRepo,
		opts: OptionFindLatestMajor,
		vcs:  &VCSRegistry{},
	}

	module := currentLatest
	err := cmd.runForModule(module)

	require.NoError(t, err)
	assert.InEpsilon(t, 9./365., module.Libyear, 0.1)
}

func TestCommand_HandleFixVersionsWhenNewMajorIsAvailable_NoCompensate(t *testing.T) {
	ctrl := gomock.NewController(t)
	currentLatest := &internal.Module{
		Path:    "github.com/go-playground/validator",
		Version: semver.MustParse("v9.4.1+incompatible"),
		Time:    mustParseTime(t, "2023-01-14"),
	}
	modulesRepo := mocks.NewMockModulesRepo(ctrl)
	modulesRepo.EXPECT().
		GetLatestInfo("github.com/go-playground/validator").
		Times(1).
		Return(currentLatest, nil)
	modulesRepo.EXPECT().
		GetLatestInfo("github.com/go-playground/validator/v10").
		Times(1).
		Return(&internal.Module{
			Path:    "github.com/go-playground/validator/v10",
			Version: semver.MustParse("v10.1.0"),
			Time:    mustParseTime(t, "2023-01-10"),
		}, nil)
	modulesRepo.EXPECT().
		GetLatestInfo("github.com/go-playground/validator/v11").
		Times(1).
		Return(nil, errors.New("no matching versions found"))
	cmd := Command{
		repo: modulesRepo,
		opts: OptionFindLatestMajor | OptionNoLibyearCompensation,
		vcs:  &VCSRegistry{},
	}

	module := currentLatest
	err := cmd.runForModule(module)

	require.NoError(t, err)
	assert.Zero(t, module.Libyear)
}

func TestCommand_FindLatestBefore_CheckCurrentTime(t *testing.T) {
	cmd := Command{ageLimit: mustParseTime(t, "2023-01-12")}

	_, err := cmd.findLatestBefore(nil, "", &internal.Module{Time: mustParseTime(t, "2023-01-13")})
	require.EqualError(t, err, "current module release time: 2023-01-13 is after the before flag value: 2023-01-12")
}

func TestCommand_FindLatestBefore_NoMatchingVersions(t *testing.T) {
	ctrl := gomock.NewController(t)
	path := "github.com/nieomylnieja/go-libyear"

	modulesRepo := mocks.NewMockModulesRepo(ctrl)
	modulesRepo.EXPECT().
		GetVersions(path).
		Times(1).
		Return([]*semver.Version{semver.MustParse("v1.0.0")}, nil)
	modulesRepo.EXPECT().
		GetInfo(path, semver.MustParse("v1.0.0")).
		Times(1).
		Return(&internal.Module{Time: mustParseTime(t, "2023-01-14")}, nil)
	cmd := Command{ageLimit: mustParseTime(t, "2023-01-13")}

	_, err := cmd.findLatestBefore(modulesRepo, path, nil)
	require.ErrorIs(t, err, errNoMatchingVersions)
}

func TestCommand_FindLatestBefore(t *testing.T) {
	tests := map[string]struct {
		Before           time.Time
		Current          *internal.Module
		Expected         *internal.Module
		Versions         []*semver.Version
		FallbackVersions []*semver.Version
		GetInfoResponses []*internal.Module
	}{
		"current supplied, no new versions": {
			Before: mustParseTime(t, "2023-01-08"),
			Current: &internal.Module{
				Version: semver.MustParse("v1.0.0"),
				Time:    mustParseTime(t, "2023-01-05"),
			},
			Expected: &internal.Module{
				Version: semver.MustParse("v1.0.0"),
				Time:    mustParseTime(t, "2023-01-05"),
			},
			Versions: []*semver.Version{semver.MustParse("v1.0.0")},
		},
		"current supplied, single version, choose current": {
			Before: mustParseTime(t, "2023-01-08"),
			Current: &internal.Module{
				Version: semver.MustParse("v1.0.0"),
				Time:    mustParseTime(t, "2023-01-05"),
			},
			Expected: &internal.Module{
				Version: semver.MustParse("v1.0.0"),
				Time:    mustParseTime(t, "2023-01-05"),
			},
			Versions: []*semver.Version{
				semver.MustParse("v1.0.0"),
				semver.MustParse("v0.9.0"),
			},
		},
		"current supplied, single version, choose latest before": {
			Before: mustParseTime(t, "2023-01-08"),
			Current: &internal.Module{
				Version: semver.MustParse("v1.0.0"),
				Time:    mustParseTime(t, "2023-01-02"),
			},
			Expected: &internal.Module{
				Version: semver.MustParse("v1.1.0"),
				Time:    mustParseTime(t, "2023-01-05"),
			},
			Versions: []*semver.Version{
				semver.MustParse("v1.0.0"),
				semver.MustParse("v1.1.0"),
			},
			GetInfoResponses: []*internal.Module{
				{
					Version: semver.MustParse("v1.1.0"),
					Time:    mustParseTime(t, "2023-01-05"),
				},
			},
		},
		"current supplied, many versions, choose latest before, var 1": {
			Before: mustParseTime(t, "2023-01-08"),
			Current: &internal.Module{
				Version: semver.MustParse("v1.1.0"),
				Time:    mustParseTime(t, "2023-01-02"),
			},
			Expected: &internal.Module{
				Version: semver.MustParse("v1.4.0"),
				Time:    mustParseTime(t, "2023-01-07"),
			},
			Versions: []*semver.Version{
				semver.MustParse("v1.0.0"),
				semver.MustParse("v1.1.0"),
				semver.MustParse("v1.2.0"),
				semver.MustParse("v1.3.0"),
				semver.MustParse("v1.4.0"),
				semver.MustParse("v1.5.0"),
				semver.MustParse("v1.6.0"),
			},
			GetInfoResponses: []*internal.Module{
				{
					Version: semver.MustParse("v1.4.0"),
					Time:    mustParseTime(t, "2023-01-07"),
				},
				{
					Version: semver.MustParse("v1.5.0"),
					Time:    mustParseTime(t, "2023-01-09"),
				},
			},
		},
		"current supplied, many versions, choose latest before, var 2": {
			Before: mustParseTime(t, "2023-01-08"),
			Current: &internal.Module{
				Version: semver.MustParse("v1.1.0"),
				Time:    mustParseTime(t, "2023-01-02"),
			},
			Expected: &internal.Module{
				Version: semver.MustParse("v1.4.0"),
				Time:    mustParseTime(t, "2023-01-08"),
			},
			Versions: []*semver.Version{
				semver.MustParse("v1.0.0"),
				semver.MustParse("v1.1.0"),
				semver.MustParse("v1.2.0"),
				semver.MustParse("v1.3.0"),
				semver.MustParse("v1.5.0"),
				semver.MustParse("v1.4.0"),
				semver.MustParse("v1.6.0"),
			},
			GetInfoResponses: []*internal.Module{
				{
					Version: semver.MustParse("v1.4.0"),
					Time:    mustParseTime(t, "2023-01-08"),
				},
				{
					Version: semver.MustParse("v1.5.0"),
					Time:    mustParseTime(t, "2023-01-09"),
				},
			},
		},
		"current supplied, many versions, choose latest before, var 3": {
			Before: mustParseTime(t, "2023-01-08"),
			Current: &internal.Module{
				Version: semver.MustParse("v1.1.0"),
				Time:    mustParseTime(t, "2023-01-02"),
			},
			Expected: &internal.Module{
				Version: semver.MustParse("v1.3.0"),
				Time:    mustParseTime(t, "2023-01-05"),
			},
			Versions: []*semver.Version{
				semver.MustParse("v1.0.0"),
				semver.MustParse("v1.1.0"),
				semver.MustParse("v1.2.0"),
				semver.MustParse("v1.3.0"),
				semver.MustParse("v1.5.0"),
				semver.MustParse("v1.4.0"),
				semver.MustParse("v1.6.0"),
			},
			GetInfoResponses: []*internal.Module{
				{
					Version: semver.MustParse("v1.2.0"),
					Time:    mustParseTime(t, "2023-01-04"),
				},
				{
					Version: semver.MustParse("v1.3.0"),
					Time:    mustParseTime(t, "2023-01-05"),
				},
				{
					Version: semver.MustParse("v1.4.0"),
					Time:    mustParseTime(t, "2023-01-09"),
				},
			},
		},
		"single version": {
			Before: mustParseTime(t, "2023-01-08"),
			Expected: &internal.Module{
				Version: semver.MustParse("v1.0.0"),
				Time:    mustParseTime(t, "2023-01-04"),
			},
			Versions: []*semver.Version{
				semver.MustParse("v1.0.0"),
			},
			GetInfoResponses: []*internal.Module{
				{
					Version: semver.MustParse("v1.0.0"),
					Time:    mustParseTime(t, "2023-01-04"),
				},
			},
		},
		"two version": {
			Before: mustParseTime(t, "2023-01-08"),
			Expected: &internal.Module{
				Version: semver.MustParse("v1.0.0"),
				Time:    mustParseTime(t, "2023-01-07"),
			},
			Versions: []*semver.Version{
				semver.MustParse("v1.0.0"),
				semver.MustParse("v1.1.0"),
			},
			GetInfoResponses: []*internal.Module{
				{
					Version: semver.MustParse("v1.0.0"),
					Time:    mustParseTime(t, "2023-01-07"),
				},
				{
					Version: semver.MustParse("v1.1.0"),
					Time:    mustParseTime(t, "2023-01-09"),
				},
			},
		},
		"many version, var 1": {
			Before: mustParseTime(t, "2023-01-08"),
			Expected: &internal.Module{
				Version: semver.MustParse("v1.3.0"),
				Time:    mustParseTime(t, "2023-01-05"),
			},
			Versions: []*semver.Version{
				semver.MustParse("v1.0.0"),
				semver.MustParse("v1.1.0"),
				semver.MustParse("v1.2.0"),
				semver.MustParse("v1.3.0"),
				semver.MustParse("v1.5.0"),
				semver.MustParse("v1.4.0"),
				semver.MustParse("v1.6.0"),
			},
			GetInfoResponses: []*internal.Module{
				{
					Version: semver.MustParse("v1.3.0"),
					Time:    mustParseTime(t, "2023-01-05"),
				},
				{
					Version: semver.MustParse("v1.4.0"),
					Time:    mustParseTime(t, "2023-01-09"),
				},
				{
					Version: semver.MustParse("v1.5.0"),
					Time:    mustParseTime(t, "2023-01-10"),
				},
			},
		},
		"many version, var 2": {
			Before: mustParseTime(t, "2023-01-08"),
			Expected: &internal.Module{
				Version: semver.MustParse("v1.0.0"),
				Time:    mustParseTime(t, "2023-01-05"),
			},
			Versions: []*semver.Version{
				semver.MustParse("v1.0.0"),
				semver.MustParse("v1.1.0"),
				semver.MustParse("v1.2.0"),
				semver.MustParse("v1.3.0"),
				semver.MustParse("v1.5.0"),
				semver.MustParse("v1.4.0"),
				semver.MustParse("v1.6.0"),
			},
			GetInfoResponses: []*internal.Module{
				{
					Version: semver.MustParse("v1.0.0"),
					Time:    mustParseTime(t, "2023-01-05"),
				},
				{
					Version: semver.MustParse("v1.1.0"),
					Time:    mustParseTime(t, "2023-01-10"),
				},
				{
					Version: semver.MustParse("v1.3.0"),
					Time:    mustParseTime(t, "2023-01-11"),
				},
			},
		},
		"many version, var 3": {
			Before: mustParseTime(t, "2023-01-08"),
			Expected: &internal.Module{
				Version: semver.MustParse("v1.6.0"),
				Time:    mustParseTime(t, "2023-01-07"),
			},
			Versions: []*semver.Version{
				semver.MustParse("v1.0.0"),
				semver.MustParse("v1.1.0"),
				semver.MustParse("v1.2.0"),
				semver.MustParse("v1.3.0"),
				semver.MustParse("v1.5.0"),
				semver.MustParse("v1.4.0"),
				semver.MustParse("v1.6.0"),
			},
			GetInfoResponses: []*internal.Module{
				{
					Version: semver.MustParse("v1.3.0"),
					Time:    mustParseTime(t, "2023-01-05"),
				},
				{
					Version: semver.MustParse("v1.5.0"),
					Time:    mustParseTime(t, "2023-01-06"),
				},
				{
					Version: semver.MustParse("v1.6.0"),
					Time:    mustParseTime(t, "2023-01-07"),
				},
			},
		},
		"current supplied, fallback versions": {
			Before: mustParseTime(t, "2023-01-08"),
			Current: &internal.Module{
				Version: semver.MustParse("v0.0.0-20200723181607-f06e43cca1ab"),
				Time:    mustParseTime(t, "2020-07-23"),
			},
			Expected: &internal.Module{
				Version: semver.MustParse("v0.0.0-20201216005158-039620a65673"),
				Time:    mustParseTime(t, "2020-12-16"),
			},
			FallbackVersions: []*semver.Version{
				semver.MustParse("v0.0.0-20170218160415-a3153f7040e9"),
				semver.MustParse("v0.0.0-20200723181607-f06e43cca1ab"),
				semver.MustParse("v0.0.0-20200730060457-89a2a8a1fb0b"),
				semver.MustParse("v0.0.0-20201216005158-039620a65673"),
				semver.MustParse("v0.0.0-20231213231151-1d8dd44e695e"),
			},
			GetInfoResponses: []*internal.Module{
				{
					Version: semver.MustParse("v0.0.0-20201216005158-039620a65673"),
					Time:    mustParseTime(t, "2020-12-16"),
				},
				{
					Version: semver.MustParse("v0.0.0-20231213231151-1d8dd44e695e"),
					Time:    mustParseTime(t, "2023-12-13"),
				},
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			path := "github.com/nieomylnieja/go-libyear"

			modulesRepo := mocks.NewMockModulesRepo(ctrl)
			modulesRepo.EXPECT().
				GetVersions(path).
				Times(1).
				Return(test.Versions, nil)
			versionsGetter := mocks.NewMockVersionsGetter(ctrl)
			if len(test.FallbackVersions) > 0 {
				versionsGetter.EXPECT().
					GetVersions(path).
					Times(1).
					Return(test.FallbackVersions, nil)
			}
			for _, module := range test.GetInfoResponses {
				modulesRepo.EXPECT().
					GetInfo(path, module.Version).
					Times(1).
					Return(module, nil)
			}
			cmd := Command{ageLimit: test.Before, fallbackVersions: versionsGetter}

			module, err := cmd.findLatestBefore(modulesRepo, path, test.Current)
			require.NoError(t, err)
			assert.Equal(t, test.Expected, module)
		})
	}
}

// Fixes: https://github.com/nieomylnieja/go-libyear/issues/56
func TestCommand_UseDefaultHandlerForPrivateRepoWithGoList(t *testing.T) {
	ctrl := gomock.NewController(t)
	currentLatest := &internal.Module{
		Path:    "github.com/nieomylnieja/go-libyear",
		Version: semver.MustParse("v0.5.0"),
		Time:    mustParseTime(t, "2024-05-20"),
	}
	modulesRepo := mocks.NewMockModulesRepo(ctrl)
	modulesRepo.EXPECT().
		GetLatestInfo("github.com/nieomylnieja/go-libyear").
		Times(1).
		Return(currentLatest, nil)
	modulesRepo.EXPECT().
		GetLatestInfo("github.com/nieomylnieja/go-libyear/v2").
		Times(1).
		Return(nil, errors.New("no matching versions"))
	vcsHandler := mocks.NewMockVCSHandler(ctrl)
	vcsHandler.EXPECT().
		CanHandle(gomock.Any()).
		Times(0)
	cmd := Command{
		repo: modulesRepo,
		opts: OptionFindLatestMajor,
		vcs: &VCSRegistry{
			vcsHandlers: []VCSHandler{vcsHandler},
		},
	}

	module := currentLatest
	err := cmd.runForModule(module)

	require.NoError(t, err)
}

func mustParseTime(t *testing.T, date string) time.Time {
	t.Helper()
	parsed, _ := time.Parse(time.DateOnly, date)
	return parsed
}

type mockVCSHandler struct {
	ModulesRepo
}

// CanHandle reports whether the vcs can handle the given path.
func (m *mockVCSHandler) CanHandle(path string) (bool, error) {
	return true, nil
}

// Name reports the name of the VCS system.
func (m *mockVCSHandler) Name() string {
	return ""
}
