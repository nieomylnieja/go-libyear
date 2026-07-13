package libyear

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/Masterminds/semver"

	"github.com/nieomylnieja/go-libyear/internal"
)

// HistorySample contains one libyear calculation at a specific timestamp.
type HistorySample struct {
	Timestamp time.Time
	Summary   Summary
}

// History contains ordered libyear samples for a time range.
type History struct {
	Samples []HistorySample
}

// HistoryOutput receives historical libyear samples.
type HistoryOutput interface {
	SendHistory(history History) error
}

// HistoryProgress reports progress while historical samples are calculated.
type HistoryProgress interface {
	Start(total int)
	AdvanceSample(timestamp time.Time)
	Finish()
}

type historicalSource interface {
	readHistory(ctx context.Context, timestamp time.Time) ([]byte, error)
}

const maxHistorySamples = 10_000

// HistoryCommand calculates libyear at regular timestamps in a time range.
type HistoryCommand struct {
	source                Source
	output                HistoryOutput
	repo                  ModulesRepo
	fallbackVersions      VersionsGetter
	opts                  Option
	vcs                   *VCSRegistry
	from                  time.Time
	to                    time.Time
	interval              time.Duration
	progress              HistoryProgress
	moduleProgressFactory func() ModuleProgress
	ignoreModuleErrors    bool
	moduleErrorWriter     io.Writer
}

// NewHistoryCommandBuilder creates a builder for a historical libyear command.
func NewHistoryCommandBuilder(source Source, output HistoryOutput) HistoryCommandBuilder {
	return HistoryCommandBuilder{
		source: source,
		output: output,
	}
}

// HistoryCommandBuilder configures a historical libyear command.
type HistoryCommandBuilder struct {
	source                Source
	output                HistoryOutput
	repo                  ModulesRepo
	fallback              VersionsGetter
	withCache             bool
	cacheFilePath         string
	opts                  Option
	vcsRegistry           *VCSRegistry
	from                  time.Time
	to                    time.Time
	interval              time.Duration
	progress              HistoryProgress
	moduleProgressFactory func() ModuleProgress
	ignoreModuleErrors    bool
	moduleErrorWriter     io.Writer
}

// WithRange configures the inclusive history range and sample interval.
func (b HistoryCommandBuilder) WithRange(from, to time.Time, interval time.Duration) HistoryCommandBuilder {
	b.from = from
	b.to = to
	b.interval = interval
	return b
}

// WithCache enables the file-based module cache.
func (b HistoryCommandBuilder) WithCache(cacheFilePath string) HistoryCommandBuilder {
	b.withCache = true
	b.cacheFilePath = cacheFilePath
	return b
}

// WithModulesRepo configures the module repository used for module metadata.
func (b HistoryCommandBuilder) WithModulesRepo(repo ModulesRepo) HistoryCommandBuilder {
	b.repo = repo
	return b
}

// WithFallbackVersionsGetter configures the fallback version list provider.
func (b HistoryCommandBuilder) WithFallbackVersionsGetter(getter VersionsGetter) HistoryCommandBuilder {
	b.fallback = getter
	return b
}

// WithOptions enables libyear calculation options.
func (b HistoryCommandBuilder) WithOptions(opts ...Option) HistoryCommandBuilder {
	for _, opt := range opts {
		b.opts |= opt
	}
	return b
}

// WithVCSRegistry configures the VCS registry used for private modules.
func (b HistoryCommandBuilder) WithVCSRegistry(registry *VCSRegistry) HistoryCommandBuilder {
	b.vcsRegistry = registry
	return b
}

// WithHistoryProgress configures progress reporting while history samples are calculated.
func (b HistoryCommandBuilder) WithHistoryProgress(progress HistoryProgress) HistoryCommandBuilder {
	b.progress = progress
	return b
}

// WithHistoryModuleProgress configures module scan progress for each history sample.
func (b HistoryCommandBuilder) WithHistoryModuleProgress(factory func() ModuleProgress) HistoryCommandBuilder {
	b.moduleProgressFactory = factory
	return b
}

// WithIgnoredModuleErrors reports module calculation errors without failing history samples.
func (b HistoryCommandBuilder) WithIgnoredModuleErrors(writer io.Writer) HistoryCommandBuilder {
	b.ignoreModuleErrors = true
	b.moduleErrorWriter = writer
	return b
}

// Build initializes dependencies and creates the historical libyear command.
func (b HistoryCommandBuilder) Build() (*HistoryCommand, error) {
	if b.repo == nil {
		var err error
		if b.opts&OptionUseGoList != 0 {
			b.repo, err = internal.NewGoListExecutor(b.withCache, b.cacheFilePath)
		} else {
			b.repo, err = internal.NewGoProxyClient(b.withCache, b.cacheFilePath)
		}
		if err != nil {
			return nil, err
		}
	}
	b.repo = newMemoryModulesRepo(b.repo)
	if b.fallback == nil {
		b.fallback = internal.NewDepsDevClient()
	}
	b.fallback = newMemoryVersionsGetter(b.fallback)
	if v, ok := b.source.(interface{ SetModulesRepo(repo ModulesRepo) }); ok {
		v.SetModulesRepo(b.repo)
	}
	if b.vcsRegistry == nil {
		cacheBase, err := internal.GetDefaultCacheBasePath()
		if err != nil {
			return nil, err
		}
		cacheDir := filepath.Join(cacheBase, "vcs")
		b.vcsRegistry = NewVCSRegistry(cacheDir)
	}
	if v, ok := b.source.(interface{ SetVCSRegistry(registry *VCSRegistry) }); ok {
		v.SetVCSRegistry(b.vcsRegistry)
	}
	return &HistoryCommand{
		source:                b.source,
		output:                b.output,
		repo:                  b.repo,
		fallbackVersions:      b.fallback,
		opts:                  b.opts,
		vcs:                   b.vcsRegistry,
		from:                  b.from,
		to:                    b.to,
		interval:              b.interval,
		progress:              b.progress,
		moduleProgressFactory: b.moduleProgressFactory,
		ignoreModuleErrors:    b.ignoreModuleErrors,
		moduleErrorWriter:     b.moduleErrorWriter,
	}, nil
}

// Run calculates historical libyear samples and sends them to the configured output.
func (c HistoryCommand) Run(ctx context.Context) (runErr error) {
	timestamps, err := historyTimestamps(c.from, c.to, c.interval)
	if err != nil {
		return err
	}

	progress := c.progress
	progress = startHistoryProgress(progress, len(timestamps))
	finishProgress := historyProgressFinisher(progress)
	defer finishProgress()

	historyData, err := c.historyData()
	if err != nil {
		return err
	}

	history := History{Samples: make([]HistorySample, 0, len(timestamps))}
	moduleErrorOutput, moduleErrorWriter := c.moduleErrorOutput()
	moduleErrorsHandled := false
	defer func() {
		finishProgress()
		if runErr == nil || moduleErrorsHandled {
			return
		}
		if err := c.writeModuleErrorOutput(moduleErrorOutput); err != nil {
			runErr = errors.Join(runErr, err)
		}
	}()
	for _, timestamp := range timestamps {
		sample, err := c.runSample(ctx, timestamp, historyData, progress, moduleErrorWriter)
		if err != nil {
			return err
		}
		history.Samples = append(history.Samples, sample)
	}
	finishProgress()
	moduleErrorsHandled = true
	if err := c.writeModuleErrorOutput(moduleErrorOutput); err != nil {
		return err
	}
	return c.output.SendHistory(history)
}

func startHistoryProgress(progress HistoryProgress, total int) HistoryProgress {
	if progress == nil || total == 0 {
		return nil
	}
	progress.Start(total)
	return progress
}

func historyProgressFinisher(progress HistoryProgress) func() {
	finished := false
	return func() {
		if progress == nil || finished {
			return
		}
		progress.Finish()
		finished = true
	}
}

func (c HistoryCommand) historyData() ([]byte, error) {
	if _, ok := c.source.(historicalSource); ok {
		return nil, nil
	}
	return c.source.Read()
}

func (c HistoryCommand) moduleErrorOutput() (*bytes.Buffer, io.Writer) {
	if c.moduleErrorWriter == nil {
		return nil, nil
	}
	var output bytes.Buffer
	return &output, &output
}

func (c HistoryCommand) runSample(
	ctx context.Context,
	timestamp time.Time,
	historyData []byte,
	progress HistoryProgress,
	moduleErrorWriter io.Writer,
) (HistorySample, error) {
	if err := historyContextErr(ctx); err != nil {
		return HistorySample{}, err
	}
	startHistorySample(progress, timestamp)
	sampleData, err := c.sampleData(ctx, timestamp, historyData)
	if err != nil {
		return HistorySample{}, historySampleError(timestamp, err)
	}
	output := &recordingHistoryOutput{}
	cmd := c.sampleCommand(timestamp, sampleData, output, moduleErrorWriter)
	if err := cmd.Run(ctx); err != nil {
		return HistorySample{}, historySampleError(timestamp, err)
	}
	if progress != nil {
		progress.AdvanceSample(timestamp)
	}
	return HistorySample{
		Timestamp: timestamp,
		Summary:   output.summary,
	}, nil
}

func historyContextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func startHistorySample(progress HistoryProgress, timestamp time.Time) {
	if progress == nil {
		return
	}
	if sampleProgress, ok := progress.(interface{ StartSample(timestamp time.Time) }); ok {
		sampleProgress.StartSample(timestamp)
	}
}

func (c HistoryCommand) sampleData(
	ctx context.Context,
	timestamp time.Time,
	historyData []byte,
) ([]byte, error) {
	historySource, ok := c.source.(historicalSource)
	if !ok {
		return historyData, nil
	}
	return historySource.readHistory(ctx, timestamp)
}

func (c HistoryCommand) sampleCommand(
	timestamp time.Time,
	sampleData []byte,
	output *recordingHistoryOutput,
	moduleErrorWriter io.Writer,
) Command {
	cmd := Command{
		source:             bytesSource(sampleData),
		output:             output,
		repo:               c.repo,
		fallbackVersions:   c.fallbackVersions,
		opts:               c.opts,
		vcs:                c.vcs,
		ageLimit:           timestamp,
		ignoreModuleErrors: c.ignoreModuleErrors,
		moduleErrorWriter:  moduleErrorWriter,
		moduleErrorPrefix:  fmt.Sprintf("history sample %s", timestamp.UTC().Format(time.RFC3339)),
	}
	if c.moduleProgressFactory != nil {
		cmd.progress = c.moduleProgressFactory()
	}
	return cmd
}

func (c HistoryCommand) writeModuleErrorOutput(moduleErrorOutput *bytes.Buffer) error {
	if c.moduleErrorWriter == nil || moduleErrorOutput.Len() == 0 {
		return nil
	}
	_, err := c.moduleErrorWriter.Write(moduleErrorOutput.Bytes())
	return err
}

func historySampleError(timestamp time.Time, err error) error {
	var releaseErr *moduleReleasedAfterCutoffError
	if errors.As(err, &releaseErr) {
		return fmt.Errorf("history sample %s cannot be calculated: %w", timestamp.UTC().Format(time.RFC3339), err)
	}
	return fmt.Errorf("run history sample at %s: %w", timestamp.UTC().Format(time.RFC3339), err)
}

func historyTimestamps(from, to time.Time, interval time.Duration) ([]time.Time, error) {
	switch {
	case from.IsZero():
		return nil, errors.New("history from timestamp is required")
	case to.IsZero():
		return nil, errors.New("history to timestamp is required")
	case interval <= 0:
		return nil, errors.New("history interval must be greater than zero")
	case from.After(to):
		return nil, errors.New("history from timestamp must be before or equal to to timestamp")
	}

	timestamps := make([]time.Time, 0, historyTimestampCapacity(from, to, interval))
	timestamps = append(timestamps, from)
	for next := from.Add(interval); !next.After(to); next = next.Add(interval) {
		if len(timestamps) == maxHistorySamples {
			return nil, tooManyHistorySamplesError()
		}
		timestamps = append(timestamps, next)
	}
	if !timestamps[len(timestamps)-1].Equal(to) {
		if len(timestamps) == maxHistorySamples {
			return nil, tooManyHistorySamplesError()
		}
		timestamps = append(timestamps, to)
	}
	return timestamps, nil
}

func historyTimestampCapacity(from, to time.Time, interval time.Duration) int {
	if from.Equal(to) {
		return 1
	}
	estimatedIntervals := int64(to.Sub(from) / interval)
	if estimatedIntervals >= int64(maxHistorySamples-1) {
		return maxHistorySamples
	}
	return int(estimatedIntervals) + 2
}

func tooManyHistorySamplesError() error {
	return fmt.Errorf(
		"history range exceeds the limit of %d samples; increase the interval or reduce the range",
		maxHistorySamples,
	)
}

type bytesSource []byte

func (s bytesSource) Read() ([]byte, error) {
	return s, nil
}

type recordingHistoryOutput struct {
	summary Summary
}

func (o *recordingHistoryOutput) Send(summary Summary) error {
	o.summary = summary
	return nil
}

type memoryModulesRepo struct {
	repo     ModulesRepo
	mu       sync.Mutex
	modFiles map[string][]byte
	info     map[string]*internal.Module
	latest   map[string]*internal.Module
	versions map[string][]*semver.Version
}

func newMemoryModulesRepo(repo ModulesRepo) *memoryModulesRepo {
	return &memoryModulesRepo{
		repo:     repo,
		modFiles: make(map[string][]byte),
		info:     make(map[string]*internal.Module),
		latest:   make(map[string]*internal.Module),
		versions: make(map[string][]*semver.Version),
	}
}

func (r *memoryModulesRepo) GetModFile(path string, version *semver.Version) ([]byte, error) {
	key := moduleVersionKey(path, version)
	r.mu.Lock()
	if data, ok := r.modFiles[key]; ok {
		r.mu.Unlock()
		return slices.Clone(data), nil
	}
	r.mu.Unlock()

	data, err := r.repo.GetModFile(path, version)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.modFiles[key] = slices.Clone(data)
	r.mu.Unlock()
	return slices.Clone(data), nil
}

func (r *memoryModulesRepo) GetInfo(path string, version *semver.Version) (*internal.Module, error) {
	key := moduleVersionKey(path, version)
	r.mu.Lock()
	if module, ok := r.info[key]; ok {
		r.mu.Unlock()
		return cloneModule(module), nil
	}
	r.mu.Unlock()

	module, err := r.repo.GetInfo(path, version)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.info[key] = cloneModule(module)
	r.mu.Unlock()
	return cloneModule(module), nil
}

func (r *memoryModulesRepo) GetLatestInfo(path string) (*internal.Module, error) {
	r.mu.Lock()
	if module, ok := r.latest[path]; ok {
		r.mu.Unlock()
		return cloneModule(module), nil
	}
	r.mu.Unlock()

	module, err := r.repo.GetLatestInfo(path)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.latest[path] = cloneModule(module)
	r.mu.Unlock()
	return cloneModule(module), nil
}

func (r *memoryModulesRepo) GetVersions(path string) ([]*semver.Version, error) {
	r.mu.Lock()
	if versions, ok := r.versions[path]; ok {
		r.mu.Unlock()
		return slices.Clone(versions), nil
	}
	r.mu.Unlock()

	versions, err := r.repo.GetVersions(path)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.versions[path] = slices.Clone(versions)
	r.mu.Unlock()
	return slices.Clone(versions), nil
}

type memoryVersionsGetter struct {
	getter   VersionsGetter
	mu       sync.Mutex
	versions map[string][]*semver.Version
}

func newMemoryVersionsGetter(getter VersionsGetter) *memoryVersionsGetter {
	return &memoryVersionsGetter{
		getter:   getter,
		versions: make(map[string][]*semver.Version),
	}
}

func (g *memoryVersionsGetter) GetVersions(path string) ([]*semver.Version, error) {
	g.mu.Lock()
	if versions, ok := g.versions[path]; ok {
		g.mu.Unlock()
		return slices.Clone(versions), nil
	}
	g.mu.Unlock()

	versions, err := g.getter.GetVersions(path)
	if err != nil {
		return nil, err
	}

	g.mu.Lock()
	g.versions[path] = slices.Clone(versions)
	g.mu.Unlock()
	return slices.Clone(versions), nil
}

func moduleVersionKey(path string, version *semver.Version) string {
	if version == nil {
		return path + "@"
	}
	return path + "@" + version.String()
}

func cloneModule(module *internal.Module) *internal.Module {
	if module == nil {
		return nil
	}
	clone := *module
	clone.AllPaths = slices.Clone(module.AllPaths)
	return &clone
}
