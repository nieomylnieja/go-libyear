package libyear

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nieomylnieja/go-libyear/internal"
)

func TestHistoryTimestamps(t *testing.T) {
	from := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)

	timestamps, err := historyTimestamps(from, to, 24*time.Hour)

	require.NoError(t, err)
	assert.Equal(t, []time.Time{
		from,
		from.Add(24 * time.Hour),
		to,
	}, timestamps)
}

func TestHistoryTimestamps_AppendsUnalignedEnd(t *testing.T) {
	from := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2022, 1, 10, 0, 0, 0, 0, time.UTC)

	timestamps, err := historyTimestamps(from, to, 7*24*time.Hour)

	require.NoError(t, err)
	assert.Equal(t, []time.Time{
		from,
		from.Add(7 * 24 * time.Hour),
		to,
	}, timestamps)
}

func TestHistoryTimestamps_InvalidRange(t *testing.T) {
	from := time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

	_, err := historyTimestamps(from, to, 24*time.Hour)

	require.EqualError(t, err, "history from timestamp must be before or equal to to timestamp")
}

func TestHistoryTimestamps_RejectsTooManySamples(t *testing.T) {
	from := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add((maxHistorySamples + 1) * time.Nanosecond)

	_, err := historyTimestamps(from, to, time.Nanosecond)

	require.EqualError(
		t,
		err,
		"history range exceeds the limit of 10000 samples; increase the interval or reduce the range",
	)
}

func TestHistoryStructuredOutputs(t *testing.T) {
	history := sampleHistory()

	var csvOutput bytes.Buffer
	err := HistoryCSVOutput{Writer: &csvOutput}.SendHistory(history)
	require.NoError(t, err)
	assert.Equal(t, ""+
		"module,timestamp,date,libyear,packages\n"+
		"example.com/app,2022-01-01T00:00:00Z,2022-01-01,1.23,2\n"+
		"example.com/app/v2,2022-01-03T00:00:00Z,2022-01-03,2.50,1\n",
		csvOutput.String())

	var jsonOutput bytes.Buffer
	err = HistoryJSONOutput{Writer: &jsonOutput}.SendHistory(history)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"samples": [
			{
				"module": "example.com/app",
				"timestamp": "2022-01-01T00:00:00Z",
				"date": "2022-01-01",
				"libyear": 1.234,
				"packages": 2
			},
			{
				"module": "example.com/app/v2",
				"timestamp": "2022-01-03T00:00:00Z",
				"date": "2022-01-03",
				"libyear": 2.5,
				"packages": 1
			}
		]
	}`, jsonOutput.String())
}

func TestHistoryChartOutput(t *testing.T) {
	var output bytes.Buffer

	err := HistoryChartOutput{Writer: &output, Width: 40, Height: 4}.SendHistory(sampleHistory())

	require.NoError(t, err)
	assert.Contains(t, output.String(), "libyear history")
	assert.Contains(t, output.String(), "2022-01-01 -> 2022-01-03 | 2 samples")
}

func TestHistoryChartOutput_RendersDateXAxis(t *testing.T) {
	history := History{Samples: []HistorySample{
		{
			Timestamp: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
			Summary: Summary{Main: &internal.Module{
				Path:    "example.com/app",
				Libyear: 1.0,
			}},
		},
		{
			Timestamp: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC),
			Summary: Summary{Main: &internal.Module{
				Path:    "example.com/app",
				Libyear: 2.0,
			}},
		},
		{
			Timestamp: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC),
			Summary: Summary{Main: &internal.Module{
				Path:    "example.com/app",
				Libyear: 1.5,
			}},
		},
	}}
	var output bytes.Buffer

	err := HistoryChartOutput{Writer: &output, Width: 80, Height: 4}.SendHistory(history)

	require.NoError(t, err)
	assert.Contains(t, output.String(), "2022-01-02")
}

func TestHistoryChartOutput_FitsConfiguredWidth(t *testing.T) {
	const width = MinimumHistoryChartWidth
	var output bytes.Buffer

	err := HistoryChartOutput{Writer: &output, Width: width, Height: 4}.SendHistory(sampleHistory())

	require.NoError(t, err)
	for line := range strings.SplitSeq(strings.TrimSuffix(output.String(), "\n"), "\n") {
		assert.LessOrEqual(t, utf8.RuneCountInString(line), width, "line %q exceeds chart width", line)
	}
}

func TestHistoryChartOutput_RejectsUnsupportedWidth(t *testing.T) {
	var output bytes.Buffer

	err := HistoryChartOutput{
		Writer: &output,
		Width:  MinimumHistoryChartWidth - 1,
	}.SendHistory(sampleHistory())

	require.EqualError(t, err, "history chart width must be at least 40 columns")
}

func TestHistoryChartOutput_RejectsFooterWiderThanChart(t *testing.T) {
	history := History{Samples: make([]HistorySample, 100_000)}
	history.Samples[0].Timestamp = time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	history.Samples[len(history.Samples)-1].Timestamp = time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)
	var output bytes.Buffer

	err := HistoryChartOutput{
		Writer: &output,
		Width:  MinimumHistoryChartWidth,
	}.SendHistory(history)

	require.EqualError(t, err, "history chart needs at least 41 columns for its footer")
	assert.Empty(t, output.String())
}

func TestHistoryLibyearValuesAtWidthUsesTimestampSpacing(t *testing.T) {
	from := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	history := History{Samples: []HistorySample{
		{
			Timestamp: from,
			Summary:   Summary{Main: &internal.Module{Libyear: 0}},
		},
		{
			Timestamp: from.Add(7 * 24 * time.Hour),
			Summary:   Summary{Main: &internal.Module{Libyear: 7}},
		},
		{
			Timestamp: from.Add(9 * 24 * time.Hour),
			Summary:   Summary{Main: &internal.Module{Libyear: 9}},
		},
	}}

	values := historyLibyearValuesAtWidth(history, 10)

	assert.Equal(t, []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, values)
}

func TestHistoryCommand_Run(t *testing.T) {
	source := &countingSource{content: `module example.com/app

go 1.22

require example.com/dep v1.0.0
`}
	output := &collectingHistoryOutput{}
	repo := newFakeHistoryRepo()
	from := time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)
	cmd, err := NewHistoryCommandBuilder(source, output).
		WithModulesRepo(repo).
		WithFallbackVersionsGetter(emptyVersionsGetter{}).
		WithVCSRegistry(&VCSRegistry{}).
		WithRange(from, to, 365*24*time.Hour).
		Build()
	require.NoError(t, err)

	err = cmd.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, output.history.Samples, 2)
	assert.Equal(t, 1, source.reads)
	assert.Equal(t, 1, repo.versionCalls["example.com/dep"])
	assert.Equal(t, from, output.history.Samples[0].Timestamp)
	assert.Equal(t, to, output.history.Samples[1].Timestamp)
	assert.InEpsilon(t, 0.41, output.history.Samples[0].Summary.Main.Libyear, 0.01)
	assert.InEpsilon(t, 1.00, output.history.Samples[1].Summary.Main.Libyear, 0.01)
}

func TestHistoryCommand_RunUsesHistoricalSourceForEachSample(t *testing.T) {
	from := time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)
	source := &recordingHistoricalSource{
		current: `module example.com/app

go 1.22

require example.com/dep v1.2.0
`,
		snapshots: map[time.Time]string{
			from: `module example.com/app

go 1.22

require example.com/dep v1.0.0
`,
			to: `module example.com/app

go 1.22

require example.com/dep v1.1.0
`,
		},
	}
	output := &collectingHistoryOutput{}
	repo := newFakeHistoryRepo()
	cmd, err := NewHistoryCommandBuilder(source, output).
		WithModulesRepo(repo).
		WithFallbackVersionsGetter(emptyVersionsGetter{}).
		WithVCSRegistry(&VCSRegistry{}).
		WithRange(from, to, 365*24*time.Hour).
		Build()
	require.NoError(t, err)

	err = cmd.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, output.history.Samples, 2)
	assert.Equal(t, 0, source.reads)
	assert.Equal(t, []time.Time{from, to}, source.historyReads)
	assert.InEpsilon(t, 0.41, output.history.Samples[0].Summary.Main.Libyear, 0.01)
	assert.InEpsilon(t, 0.59, output.history.Samples[1].Summary.Main.Libyear, 0.01)
}

func TestHistoryCommand_RunReportsAndIgnoresModuleErrors(t *testing.T) {
	source := &countingSource{content: `module example.com/app

go 1.22

require (
	example.com/dep v1.0.0
	example.com/broken v1.0.0
)
`}
	output := &collectingHistoryOutput{}
	warnings := &bytes.Buffer{}
	repo := newFakeHistoryRepo()
	brokenKey := moduleVersionKey("example.com/broken", semver.MustParse("v1.0.0"))
	repo.infoErrors[brokenKey] = errors.New("module metadata not found")
	from := time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)
	cmd, err := NewHistoryCommandBuilder(source, output).
		WithModulesRepo(repo).
		WithFallbackVersionsGetter(emptyVersionsGetter{}).
		WithVCSRegistry(&VCSRegistry{}).
		WithRange(from, from, 24*time.Hour).
		WithIgnoredModuleErrors(warnings).
		Build()
	require.NoError(t, err)

	err = cmd.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, output.history.Samples, 1)
	require.Len(t, output.history.Samples[0].Summary.Modules, 1)
	assert.Equal(t, "example.com/dep", output.history.Samples[0].Summary.Modules[0].Path)
	assert.Contains(
		t,
		warnings.String(),
		"Warning: history sample 2022-07-01T00:00:00Z: "+
			"ignoring module example.com/broken: module metadata not found",
	)
}

func TestHistoryCommand_RunReportsAndIgnoresEmptyVersionMetadata(t *testing.T) {
	source := &countingSource{content: `module example.com/app

go 1.22

require example.com/dep v1.0.0
`}
	output := &collectingHistoryOutput{}
	warnings := &bytes.Buffer{}
	repo := newFakeHistoryRepo()
	repo.versions["example.com/dep"] = nil
	from := time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)
	cmd, err := NewHistoryCommandBuilder(source, output).
		WithModulesRepo(repo).
		WithFallbackVersionsGetter(emptyVersionsGetter{}).
		WithVCSRegistry(&VCSRegistry{}).
		WithRange(from, from, 24*time.Hour).
		WithIgnoredModuleErrors(warnings).
		Build()
	require.NoError(t, err)

	err = cmd.Run(t.Context())

	require.NoError(t, err)
	require.Len(t, output.history.Samples, 1)
	assert.Empty(t, output.history.Samples[0].Summary.Modules)
	assert.Contains(
		t,
		warnings.String(),
		"Warning: history sample 2022-07-01T00:00:00Z: "+
			"ignoring module example.com/dep: no versions found",
	)
}

func TestHistoryCommand_RunDoesNotIgnoreSamplesBeforeCurrentVersion(t *testing.T) {
	source := &countingSource{content: `module example.com/app

go 1.22

require example.com/dep v1.0.0
`}
	output := &collectingHistoryOutput{}
	warnings := &bytes.Buffer{}
	repo := newFakeHistoryRepo()
	from := time.Date(2021, 7, 1, 0, 0, 0, 0, time.UTC)
	cmd, err := NewHistoryCommandBuilder(source, output).
		WithModulesRepo(repo).
		WithFallbackVersionsGetter(emptyVersionsGetter{}).
		WithVCSRegistry(&VCSRegistry{}).
		WithRange(from, from, 24*time.Hour).
		WithIgnoredModuleErrors(warnings).
		Build()
	require.NoError(t, err)

	err = cmd.Run(t.Context())

	require.EqualError(
		t,
		err,
		"history sample 2021-07-01T00:00:00Z cannot be calculated: "+
			"go.mod requires example.com/dep@v1.0.0 released on 2022-01-01, "+
			"after cutoff 2021-07-01; cannot calculate libyear before that version existed; "+
			"use a cutoff on or after 2022-01-01T00:00:00Z or analyze a go.mod from the requested date",
	)
	assert.Empty(t, warnings.String())
	assert.Empty(t, output.history.Samples)
}

func TestHistoryCommand_RunDoesNotIgnoreContextErrors(t *testing.T) {
	tests := map[string]error{
		"canceled":          context.Canceled,
		"deadline exceeded": context.DeadlineExceeded,
	}
	for name, contextErr := range tests {
		t.Run(name, func(t *testing.T) {
			source := &countingSource{content: `module example.com/app

go 1.22

require example.com/dep v1.0.0
`}
			output := &collectingHistoryOutput{}
			warnings := &bytes.Buffer{}
			repo := newFakeHistoryRepo()
			key := moduleVersionKey("example.com/dep", semver.MustParse("v1.0.0"))
			repo.infoErrors[key] = contextErr
			from := time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)
			cmd, err := NewHistoryCommandBuilder(source, output).
				WithModulesRepo(repo).
				WithFallbackVersionsGetter(emptyVersionsGetter{}).
				WithVCSRegistry(&VCSRegistry{}).
				WithRange(from, from, 24*time.Hour).
				WithIgnoredModuleErrors(warnings).
				Build()
			require.NoError(t, err)

			err = cmd.Run(t.Context())

			require.ErrorIs(t, err, contextErr)
			assert.Empty(t, warnings.String())
			assert.Empty(t, output.history.Samples)
		})
	}
}

func TestHistoryCommand_RunReportsIgnoredModuleErrorsBeforeLaterSampleFailure(t *testing.T) {
	from := time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	source := &recordingHistoricalSource{snapshots: map[time.Time]string{
		from: `module example.com/app

go 1.22

require example.com/broken v1.0.0
`,
	}}
	warnings := &bytes.Buffer{}
	repo := newFakeHistoryRepo()
	brokenKey := moduleVersionKey("example.com/broken", semver.MustParse("v1.0.0"))
	repo.infoErrors[brokenKey] = errors.New("module metadata not found")
	cmd, err := NewHistoryCommandBuilder(source, &collectingHistoryOutput{}).
		WithModulesRepo(repo).
		WithFallbackVersionsGetter(emptyVersionsGetter{}).
		WithVCSRegistry(&VCSRegistry{}).
		WithRange(from, to, 24*time.Hour).
		WithIgnoredModuleErrors(warnings).
		Build()
	require.NoError(t, err)

	err = cmd.Run(context.Background())

	require.EqualError(
		t,
		err,
		"run history sample at 2022-07-02T00:00:00Z: "+
			"missing history snapshot for 2022-07-02T00:00:00Z",
	)
	assert.Contains(
		t,
		warnings.String(),
		"Warning: history sample 2022-07-01T00:00:00Z: "+
			"ignoring module example.com/broken: module metadata not found",
	)
}

func TestHistoryCommand_RunReportsProgress(t *testing.T) {
	source := &countingSource{content: `module example.com/app

go 1.22

require example.com/dep v1.0.0
`}
	progress := &recordingHistoryProgress{}
	output := &collectingHistoryOutput{progress: progress}
	repo := newFakeHistoryRepo()
	from := time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)
	cmd, err := NewHistoryCommandBuilder(source, output).
		WithModulesRepo(repo).
		WithFallbackVersionsGetter(emptyVersionsGetter{}).
		WithVCSRegistry(&VCSRegistry{}).
		WithRange(from, to, 365*24*time.Hour).
		WithHistoryProgress(progress).
		Build()
	require.NoError(t, err)

	err = cmd.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []int{2}, progress.started)
	assert.Equal(t, []time.Time{from, to}, progress.currentSamples)
	assert.Equal(t, []time.Time{from, to}, progress.samples)
	assert.Equal(t, 1, progress.finished)
	assert.Equal(t, 1, output.progressFinishedOnSend)
}

func TestHistoryCommand_RunReportsModuleProgressForEachSample(t *testing.T) {
	source := &countingSource{content: `module example.com/app

go 1.22

require example.com/dep v1.0.0
`}
	output := &collectingHistoryOutput{}
	repo := newFakeHistoryRepo()
	from := time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)
	var moduleProgresses []*recordingProgress
	cmd, err := NewHistoryCommandBuilder(source, output).
		WithModulesRepo(repo).
		WithFallbackVersionsGetter(emptyVersionsGetter{}).
		WithVCSRegistry(&VCSRegistry{}).
		WithRange(from, to, 365*24*time.Hour).
		WithHistoryModuleProgress(func() ModuleProgress {
			progress := &recordingProgress{}
			moduleProgresses = append(moduleProgresses, progress)
			return progress
		}).
		Build()
	require.NoError(t, err)

	err = cmd.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, moduleProgresses, 2)
	for _, progress := range moduleProgresses {
		assert.Equal(t, []int{1}, progress.started)
		assert.Equal(t, []string{"example.com/dep"}, progress.modules)
		assert.Equal(t, 1, progress.advanced)
		assert.Equal(t, 1, progress.finished)
	}
}

func TestHistoryCommand_RunExplainsSamplesBeforeCurrentVersion(t *testing.T) {
	source := &countingSource{content: `module example.com/app

go 1.22

require example.com/dep v1.0.0
`}
	output := &collectingHistoryOutput{}
	repo := newFakeHistoryRepo()
	from := time.Date(2021, 7, 1, 0, 0, 0, 0, time.UTC)
	cmd, err := NewHistoryCommandBuilder(source, output).
		WithModulesRepo(repo).
		WithFallbackVersionsGetter(emptyVersionsGetter{}).
		WithVCSRegistry(&VCSRegistry{}).
		WithRange(from, from, 24*time.Hour).
		Build()
	require.NoError(t, err)

	err = cmd.Run(context.Background())

	require.EqualError(
		t,
		err,
		"history sample 2021-07-01T00:00:00Z cannot be calculated: "+
			"go.mod requires example.com/dep@v1.0.0 released on 2022-01-01, "+
			"after cutoff 2021-07-01; cannot calculate libyear before that version existed; "+
			"use a cutoff on or after 2022-01-01T00:00:00Z or analyze a go.mod from the requested date",
	)
}

func sampleHistory() History {
	return History{Samples: []HistorySample{
		{
			Timestamp: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
			Summary: Summary{
				Main: &internal.Module{
					Path:    "example.com/app",
					Libyear: 1.234,
				},
				Modules: []*internal.Module{
					{Path: "example.com/one"},
					{Path: "example.com/two"},
				},
			},
		},
		{
			Timestamp: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC),
			Summary: Summary{
				Main: &internal.Module{
					Path:    "example.com/app/v2",
					Libyear: 2.5,
				},
				Modules: []*internal.Module{
					{Path: "example.com/one"},
				},
			},
		},
	}}
}

type countingSource struct {
	content string
	reads   int
}

func (s *countingSource) Read() ([]byte, error) {
	s.reads++
	return []byte(s.content), nil
}

type recordingHistoricalSource struct {
	current      string
	snapshots    map[time.Time]string
	reads        int
	historyReads []time.Time
}

func (s *recordingHistoricalSource) Read() ([]byte, error) {
	s.reads++
	return []byte(s.current), nil
}

func (s *recordingHistoricalSource) readHistory(_ context.Context, timestamp time.Time) ([]byte, error) {
	s.historyReads = append(s.historyReads, timestamp)
	snapshot, ok := s.snapshots[timestamp]
	if !ok {
		return nil, fmt.Errorf("missing history snapshot for %s", timestamp.UTC().Format(time.RFC3339))
	}
	return []byte(snapshot), nil
}

type collectingHistoryOutput struct {
	history                History
	progress               *recordingHistoryProgress
	progressFinishedOnSend int
}

func (o *collectingHistoryOutput) SendHistory(history History) error {
	if o.progress != nil {
		o.progressFinishedOnSend = o.progress.Finished()
	}
	o.history = history
	return nil
}

type recordingHistoryProgress struct {
	started        []int
	currentSamples []time.Time
	samples        []time.Time
	finished       int
}

func (p *recordingHistoryProgress) Start(total int) {
	p.started = append(p.started, total)
}

func (p *recordingHistoryProgress) StartSample(timestamp time.Time) {
	p.currentSamples = append(p.currentSamples, timestamp)
}

func (p *recordingHistoryProgress) AdvanceSample(timestamp time.Time) {
	p.samples = append(p.samples, timestamp)
}

func (p *recordingHistoryProgress) Finish() {
	p.finished++
}

func (p *recordingHistoryProgress) Finished() int {
	return p.finished
}

type emptyVersionsGetter struct{}

func (g emptyVersionsGetter) GetVersions(_ string) ([]*semver.Version, error) {
	return nil, nil
}

type fakeHistoryRepo struct {
	infos        map[string]*internal.Module
	infoErrors   map[string]error
	versions     map[string][]*semver.Version
	versionCalls map[string]int
}

func newFakeHistoryRepo() *fakeHistoryRepo {
	infos := map[string]*internal.Module{}
	for _, module := range []*internal.Module{
		{
			Path:    "example.com/dep",
			Version: semver.MustParse("v1.0.0"),
			Time:    time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Path:    "example.com/dep",
			Version: semver.MustParse("v1.1.0"),
			Time:    time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Path:    "example.com/dep",
			Version: semver.MustParse("v1.2.0"),
			Time:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	} {
		infos[moduleVersionKey(module.Path, module.Version)] = module
	}
	return &fakeHistoryRepo{
		infos:      infos,
		infoErrors: map[string]error{},
		versions: map[string][]*semver.Version{
			"example.com/dep": {
				semver.MustParse("v1.0.0"),
				semver.MustParse("v1.1.0"),
				semver.MustParse("v1.2.0"),
			},
		},
		versionCalls: make(map[string]int),
	}
}

func (r *fakeHistoryRepo) GetModFile(_ string, _ *semver.Version) ([]byte, error) {
	return nil, nil
}

func (r *fakeHistoryRepo) GetInfo(path string, version *semver.Version) (*internal.Module, error) {
	key := moduleVersionKey(path, version)
	if err := r.infoErrors[key]; err != nil {
		return nil, err
	}
	module := r.infos[key]
	if module == nil {
		return nil, fmt.Errorf("missing info fixture for %s", key)
	}
	return cloneModule(module), nil
}

func (r *fakeHistoryRepo) GetLatestInfo(_ string) (*internal.Module, error) {
	return nil, nil
}

func (r *fakeHistoryRepo) GetVersions(path string) ([]*semver.Version, error) {
	r.versionCalls[path]++
	return r.versions[path], nil
}
