package libyear

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	pathlib "path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nieomylnieja/go-libyear/internal"

	"github.com/Masterminds/semver"
	"golang.org/x/sync/errgroup"
)

type Option int

const (
	OptionShowReleases          Option = 1 << iota // 1
	OptionShowVersions                             // 2
	OptionSkipFresh                                // 4
	OptionIncludeIndirect                          // 8
	OptionUseGoList                                // 16
	OptionFindLatestMajor                          // 32
	OptionNoLibyearCompensation                    // 32
)

//go:generate go tool mockgen -destination internal/mocks/command.go -package mocks -typed . ModulesRepo,VersionsGetter

type ModulesRepo interface {
	VersionsGetter
	GetModFile(path string, version *semver.Version) ([]byte, error)
	GetInfo(path string, version *semver.Version) (*internal.Module, error)
	GetLatestInfo(path string) (*internal.Module, error)
}

type VersionsGetter interface {
	GetVersions(path string) ([]*semver.Version, error)
}

// ModuleProgress reports dependency module scanning progress.
// Implementations must be safe for concurrent use.
type ModuleProgress interface {
	Start(total int)
	Advance()
	Finish()
}

type Command struct {
	source             Source
	output             Output
	repo               ModulesRepo
	fallbackVersions   VersionsGetter
	opts               Option
	vcs                *VCSRegistry
	ageLimit           time.Time
	progress           ModuleProgress
	ignoreModuleErrors bool
	moduleErrorWriter  io.Writer
	moduleErrorPrefix  string
}

func (c Command) Run(ctx context.Context) error {
	data, err := c.source.Read()
	if err != nil {
		return err
	}

	mainModule, modules, err := internal.ReadGoMod(data)
	if err != nil {
		return err
	}
	mainModule.Time = time.Now()
	if !c.optionIsSet(OptionIncludeIndirect) {
		// Filter out indirect.
		modules = slices.DeleteFunc(modules, func(module *internal.Module) bool { return module.Indirect })
	}

	progress := c.progress
	if progress != nil && len(modules) > 0 {
		progress.Start(len(modules))
	} else {
		progress = nil
	}

	ignoredErrors := &ignoredModuleErrors{}
	group, _ := c.newErrGroup(ctx)
	for _, module := range modules {
		module := module
		group.Go(func() error {
			return c.runModule(module, progress, ignoredErrors)
		})
	}
	if err = group.Wait(); err != nil {
		if progress != nil {
			progress.Finish()
		}
		return err
	}
	if progress != nil {
		progress.Finish()
	}
	modules, err = c.removeIgnoredModules(modules, ignoredErrors.all())
	if err != nil {
		return err
	}
	// Remove skipped modules.
	if c.optionIsSet(OptionSkipFresh) {
		modules = slices.DeleteFunc(modules, func(module *internal.Module) bool { return module.Skipped })
	}

	// Aggregate results for main module.
	for _, module := range modules {
		mainModule.Libyear += module.Libyear
		mainModule.ReleasesDiff += module.ReleasesDiff
		mainModule.VersionsDiff = mainModule.VersionsDiff.Add(module.VersionsDiff)
	}

	// Prepare and send summary.
	return c.output.Send(Summary{
		Modules:  modules,
		Main:     mainModule,
		releases: c.optionIsSet(OptionShowReleases),
		versions: c.optionIsSet(OptionShowVersions),
	})
}

func (c Command) runModule(
	module *internal.Module,
	progress ModuleProgress,
	ignoredErrors *ignoredModuleErrors,
) error {
	if progress != nil {
		defer advanceModuleProgress(progress, module.Path)
	}
	if err := c.runForModule(module); err != nil {
		if c.ignoreModuleErrors && isModuleMetadataError(err) {
			ignoredErrors.add(module.Path, err)
			return nil
		}
		return err
	}
	return nil
}

func advanceModuleProgress(progress ModuleProgress, path string) {
	if moduleProgress, ok := progress.(interface{ AdvanceModule(path string) }); ok {
		moduleProgress.AdvanceModule(path)
		return
	}
	progress.Advance()
}

func (c Command) removeIgnoredModules(
	modules []*internal.Module,
	ignored []ignoredModuleError,
) ([]*internal.Module, error) {
	if len(ignored) == 0 {
		return modules, nil
	}
	ignoredPaths := make(map[string]struct{}, len(ignored))
	for _, moduleError := range ignored {
		ignoredPaths[moduleError.path] = struct{}{}
	}
	modules = slices.DeleteFunc(modules, func(module *internal.Module) bool {
		_, ignored := ignoredPaths[module.Path]
		return ignored
	})
	if err := c.writeIgnoredModuleErrors(ignored); err != nil {
		return nil, err
	}
	return modules, nil
}

type ignoredModuleError struct {
	path string
	err  error
}

type ignoredModuleErrors struct {
	mu     sync.Mutex
	errors []ignoredModuleError
}

type moduleMetadataError struct {
	err error
}

func (e *moduleMetadataError) Error() string {
	return e.err.Error()
}

func (e *moduleMetadataError) Unwrap() error {
	return e.err
}

func wrapModuleMetadataError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*moduleMetadataError](err); ok {
		return err
	}
	return &moduleMetadataError{err: err}
}

func isModuleMetadataError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	_, ok := errors.AsType[*moduleMetadataError](err)
	return ok
}

func (e *ignoredModuleErrors) add(path string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errors = append(e.errors, ignoredModuleError{path: path, err: err})
}

func (e *ignoredModuleErrors) all() []ignoredModuleError {
	e.mu.Lock()
	defer e.mu.Unlock()
	moduleErrors := slices.Clone(e.errors)
	sort.Slice(moduleErrors, func(i, j int) bool {
		return moduleErrors[i].path < moduleErrors[j].path
	})
	return moduleErrors
}

func (c Command) writeIgnoredModuleErrors(moduleErrors []ignoredModuleError) error {
	if c.moduleErrorWriter == nil {
		return nil
	}
	for _, moduleError := range moduleErrors {
		prefix := "Warning"
		if c.moduleErrorPrefix != "" {
			prefix += ": " + c.moduleErrorPrefix
		}
		if _, err := fmt.Fprintf(
			c.moduleErrorWriter,
			"%s: ignoring module %s: %v\n",
			prefix,
			moduleError.path,
			moduleError.err,
		); err != nil {
			return err
		}
	}
	return nil
}

const secondsInYear = float64(365 * 24 * 60 * 60)

func (c Command) runForModule(module *internal.Module) error {
	repo := c.repo
	// We skip this module, unless we get to the end and manage to calculate libyear.
	module.Skipped = true

	// Verify if the module is private.
	// Use default handler for go-list.
	if !c.optionIsSet(OptionUseGoList) && c.vcs.IsPrivate(module.Path) {
		var err error
		repo, err = c.vcs.GetHandler(module.Path)
		if err != nil {
			return wrapModuleMetadataError(err)
		}
	}

	// Since we're parsing the go.mod file directly, we might need to fetch the Module.Time.
	if module.Time.IsZero() {
		fetchedModule, err := repo.GetInfo(module.Path, module.Version)
		if err != nil {
			return wrapModuleMetadataError(err)
		}
		module.Time = fetchedModule.Time
	}

	// Fetch latest.
	latest, err := c.getLatestInfo(module, repo)
	if err != nil {
		return err
	}
	// It returns -1 (smaller), 0 (larger), or 1 (greater) when compared.
	if module.Version.Compare(latest.Version) != -1 {
		module.Latest = module
		module.Time = latest.Time
		return nil
	}
	module.Latest = latest

	currentTime := module.Time
	if c.optionIsSet(OptionFindLatestMajor) &&
		!c.optionIsSet(OptionNoLibyearCompensation) &&
		module.Path != latest.Path {
		first, err := c.findFirstModule(repo, latest.Path)
		if err != nil {
			return err
		}
		if module.Time.After(first.Time) {
			log.Printf("INFO: current module version %s is newer than latest version %s; "+
				"libyear will be calculated from the first version of latest major (%s) to the latest version (%s); "+
				"if you wish to disable this behavior, use --allow-negative-libyear flag",
				module.Version, latest.Version, first.Version, module.Version)
			currentTime = first.Time
		}
	}
	// The following calculations are based on https://ericbouwers.github.io/papers/icse15.pdf.
	module.Libyear = calculateLibyear(currentTime, latest.Time)
	if c.optionIsSet(OptionShowReleases) {
		versions, err := c.getAllVersions(repo, latest)
		if errors.Is(err, errNoVersions) {
			log.Printf("WARN: module '%s' does not have any versions", module.Path)
			return nil
		}
		module.ReleasesDiff = calculateReleases(module, latest, versions)
	}
	if c.optionIsSet(OptionShowVersions) {
		module.VersionsDiff = calculateVersions(module, latest)
	}

	module.Skipped = false
	return nil
}

var errNoVersions = errors.New("no versions found")

func (c Command) getAllVersions(repo ModulesRepo, latest *internal.Module) ([]*semver.Version, error) {
	allVersions := make([]*semver.Version, 0)
	for _, path := range latest.AllPaths {
		versions, err := c.getVersionsForPath(repo, path, latest.Version.Prerelease() != "")
		if err != nil {
			return nil, err
		}
		allVersions = append(allVersions, versions...)
	}
	sort.Sort(semver.Collection(allVersions))
	return allVersions, nil
}

func (c Command) getVersionsForPath(repo ModulesRepo, path string, isPrerelease bool) ([]*semver.Version, error) {
	versions, err := repo.GetVersions(path)
	if err != nil {
		return nil, wrapModuleMetadataError(err)
	}
	if len(versions) > 0 {
		return versions, nil
	}
	if !isPrerelease {
		return nil, wrapModuleMetadataError(errNoVersions)
	}
	fallback := c.fallbackVersions
	// Alternative is the fallback as na argument to the function, which makes it even more messy.
	if _, ok := repo.(VCSHandler); ok {
		fallback = repo
	}
	// Try fetching the versions from deps.dev.
	// Go list does not list prerelease versions, which is fine,
	// unless we're dealing with a prerelease version ourselves.
	versions, err = fallback.GetVersions(path)
	if err != nil {
		return nil, wrapModuleMetadataError(err)
	}
	// Check again.
	if len(versions) == 0 {
		return nil, wrapModuleMetadataError(errNoVersions)
	}
	return versions, nil
}

func (c Command) getLatestInfo(current *internal.Module, repo ModulesRepo) (*internal.Module, error) {
	var (
		path   = current.Path
		paths  []string
		latest *internal.Module
	)
	for {
		var (
			lts *internal.Module
			err error
		)
		if c.ageLimit.IsZero() {
			lts, err = repo.GetLatestInfo(path)
			err = wrapModuleMetadataError(err)
		} else {
			// If this is the first iteration, optimize findLatestBefore by passing it the current version module.
			if latest == nil {
				lts, err = c.findLatestBefore(repo, path, current)
			} else {
				lts, err = c.findLatestBefore(repo, path, nil)
			}
		}
		if err != nil {
			if strings.Contains(err.Error(), "no matching versions") {
				break
			}
			return nil, err
		}
		// In case for whatever reason we start endlessly looping here, break it.
		if latest != nil && latest.Version.Compare(lts.Version) == 0 {
			return latest, nil
		}
		latest = lts
		if !c.optionIsSet(OptionFindLatestMajor) {
			break
		}
		// Increment major version.
		var newMajor int64
		if latest.Version.Major() > 1 {
			newMajor = latest.Version.Major() + 1
		} else {
			newMajor = 2
		}
		paths = append(paths, path)
		path = updatePathVersion(path, latest.Version.Major(), newMajor)
	}
	// In case we don't have v2 or above.
	if len(paths) == 0 {
		paths = append(paths, latest.Path)
	}
	latest.AllPaths = paths
	return latest, nil
}

// findFirstModule finds the first module in the given path.
// If the path has /v2 or higher suffix it will find the first module in this version.
func (c Command) findFirstModule(repo ModulesRepo, path string) (*internal.Module, error) {
	versions, err := repo.GetVersions(path)
	if err != nil {
		return nil, wrapModuleMetadataError(err)
	}
	if len(versions) == 0 {
		return nil, wrapModuleMetadataError(
			fmt.Errorf("no versions found for path %s, expected at least one", path),
		)
	}
	sort.Sort(semver.Collection(versions))
	module, err := repo.GetInfo(path, versions[0])
	return module, wrapModuleMetadataError(err)
}

func updatePathVersion(path string, currentMajor, newMajor int64) string {
	if currentMajor > 1 {
		// Only trim the suffix from post-modules version paths.
		if strings.HasSuffix(path, strconv.Itoa(int(currentMajor))) {
			path = pathlib.Dir(path)
		}
	}
	return pathlib.Join(path, "v"+strconv.Itoa(int(newMajor)))
}

func calculateLibyear(moduleTime, latestTime time.Time) float64 {
	diff := latestTime.Sub(moduleTime)
	libyear := diff.Seconds() / secondsInYear
	if libyear < 0 {
		libyear = 0
	}
	return libyear
}

func calculateReleases(module, latest *internal.Module, versions []*semver.Version) int {
	currentIndex := slices.IndexFunc(versions, func(v *semver.Version) bool { return module.Version.Equal(v) })
	latestIndex := slices.IndexFunc(versions, func(v *semver.Version) bool { return latest.Version.Equal(v) })
	// Example:
	// v:  v1 | v2 | v3 | v4
	// i:  0    1    2    3   > len == 4
	//          ^         ^
	//    current (i:1)   latest (i:3)
	return latestIndex - currentIndex
}

func calculateVersions(module, latest *internal.Module) internal.VersionsDiff {
	// This takes a form of 3 element array.
	// The delta is defined as the absolute difference between the
	// highest-order version number which has changed compared to
	// the previous version number tuple.
	// Example:
	// v1:   v2.3.4
	// v2:   v3.6.4
	// diff: [(3-2), 0, 0] = [1, 0, 0]
	switch {
	case latest.Version.Major() != module.Version.Major():
		return internal.VersionsDiff{
			latest.Version.Major() - module.Version.Major(),
			0,
			0,
		}
	case latest.Version.Minor() != module.Version.Minor():
		return internal.VersionsDiff{
			0,
			latest.Version.Minor() - module.Version.Minor(),
			0,
		}
	default:
		return internal.VersionsDiff{
			0,
			0,
			latest.Version.Patch() - module.Version.Patch(),
		}
	}
}

func (c Command) newErrGroup(ctx context.Context) (*errgroup.Group, context.Context) {
	group, ctx := errgroup.WithContext(ctx)
	limit, _ := strconv.Atoi(os.Getenv("GOMAXPROCS"))
	if limit == 0 {
		limit = 4
	}
	group.SetLimit(limit)
	return group, ctx
}

func (c Command) optionIsSet(option Option) bool {
	return c.opts&option != 0
}

var errNoMatchingVersions = errors.New("no matching versions")

type moduleReleasedAfterCutoffError struct {
	path        string
	version     *semver.Version
	releaseTime time.Time
	cutoff      time.Time
}

func (e *moduleReleasedAfterCutoffError) Error() string {
	version := "<unknown>"
	if e.version != nil {
		version = e.version.Original()
		if version == "" {
			version = e.version.String()
		}
	}
	return fmt.Sprintf(
		"go.mod requires %s@%s released on %s, after cutoff %s; "+
			"cannot calculate libyear before that version existed; "+
			"use a cutoff on or after %s or analyze a go.mod from the requested date",
		e.path,
		version,
		e.releaseTime.Format(time.DateOnly),
		e.cutoff.Format(time.DateOnly),
		e.releaseTime.UTC().Format(time.RFC3339),
	)
}

// findLatestBefore uses binary search to find the latest module published before the given time.
// It is highly recommended to use cache when calling this function.
// current argument is optional, if it is provided, the function optimizes its search by skipping
// every version preceding current version.
func (c Command) findLatestBefore(repo ModulesRepo, path string, current *internal.Module) (*internal.Module, error) {
	if current != nil && c.ageLimit.Before(current.Time) {
		return nil, &moduleReleasedAfterCutoffError{
			path:        path,
			version:     current.Version,
			releaseTime: current.Time,
			cutoff:      c.ageLimit,
		}
	}
	// Make sure we handle prerelease versions as well.
	isPrerelease := current != nil && current.Version.Prerelease() != ""
	versions, err := c.getVersionsForPath(repo, path, isPrerelease)
	if err != nil {
		return nil, err
	}
	sort.Sort(semver.Collection(versions))
	// Optimize the search if current was provided.
	if current != nil {
		currentIndex := slices.IndexFunc(versions, func(v *semver.Version) bool { return current.Version.Equal(v) })
		versions = versions[currentIndex+1:]
	}
	start, end := 0, (len(versions) - 1)
	latest := current
	for start <= end {
		mid := (start + end) / 2
		lts, err := repo.GetInfo(path, versions[mid])
		if err != nil {
			return nil, wrapModuleMetadataError(err)
		}
		if lts.Time.After(c.ageLimit) {
			// Investigate the lower half of the range.
			end = mid - 1
		} else {
			// Investigate the upper half of the range.
			// If the potential latest (lts) is after current latest candidate, update latest.
			if latest == nil || lts.Time.After(latest.Time) {
				latest = lts
			}
			start = mid + 1
		}
	}
	if latest == nil {
		return nil, errNoMatchingVersions
	}
	return latest, nil
}
