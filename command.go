package libyear

import (
	"context"
	"log"
	"os"
	pathlib "path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nieomylnieja/go-libyear/internal"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
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

//go:generate mockgen -destination internal/mocks/command.go -package mocks -typed . ModulesRepo,VersionsGetter

type ModulesRepo interface {
	VersionsGetter
	GetModFile(path string, version *semver.Version) ([]byte, error)
	GetInfo(path string, version *semver.Version) (*internal.Module, error)
	GetLatestInfo(path string) (*internal.Module, error)
}

type VersionsGetter interface {
	GetVersions(path string) ([]*semver.Version, error)
}

type Command struct {
	source           Source
	output           Output
	repo             ModulesRepo
	fallbackVersions VersionsGetter
	opts             Option
	vcs              *VCSRegistry
	before           time.Time
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

	group, _ := c.newErrGroup(ctx)
	for _, module := range modules {
		module := module
		group.Go(func() error { return c.runForModule(module) })
	}
	if err = group.Wait(); err != nil {
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

const secondsInYear = float64(365 * 24 * 60 * 60)

func (c Command) runForModule(module *internal.Module) error {
	repo := c.repo
	// We skip this module, unless we get to the end and manage to calculate libyear.
	module.Skipped = true

	// Verify if the module is private.
	if c.vcs.IsPrivate(module.Path) {
		var err error
		repo, err = c.vcs.GetHandler(module.Path)
		if err != nil {
			return err
		}
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

	// Since we're parsing the go.mod file directly, we might need to fetch the Module.Time.
	if module.Time.IsZero() {
		fetchedModule, err := repo.GetInfo(module.Path, module.Version)
		if err != nil {
			return err
		}
		module.Time = fetchedModule.Time
	}

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
		if err == errNoVersions {
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
		return nil, err
	}
	if len(versions) > 0 {
		return versions, nil
	}
	if !isPrerelease {
		return nil, errNoVersions
	}
	// Try fetching the versions from deps.dev.
	// Go list does not list prerelease versions, which is fine,
	// unless we're dealing with a prerelease version ourselves.
	versions, err = c.fallbackVersions.GetVersions(path)
	if err != nil {
		return nil, err
	}
	// Check again.
	if len(versions) == 0 {
		return nil, errNoVersions
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
		if c.before.IsZero() {
			lts, err = repo.GetLatestInfo(path)
		} else {
			// If this is the first iteration, optimize findLatestBefore by passing it the current version module.
			if latest == nil {
				lts, err = c.findLatestBefore(path, current)
			} else {
				lts, err = c.findLatestBefore(path, nil)
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
		return nil, err
	}
	if len(versions) == 0 {
		return nil, errors.Errorf("no versions found for path %s, expected at least one", path)
	}
	sort.Sort(semver.Collection(versions))
	return repo.GetInfo(path, versions[0])
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
	maxProcs, _ := strconv.Atoi(os.Getenv("GOMAXPROCS"))
	if maxProcs == 0 {
		maxProcs = 4
	}
	group.SetLimit(maxProcs)
	return group, ctx
}

func (c Command) optionIsSet(option Option) bool {
	return c.opts&option != 0
}

var errNoMatchingVersions = errors.New("no matching versions")

// findLatestBefore uses binary search to find the latest module published before the given time.
// It is highly recommended to use cache when calling this function.
// current argument is optional, if it is provided, the function optimizes its search by skipping
// every version preceeding current version.
func (c Command) findLatestBefore(path string, current *internal.Module) (*internal.Module, error) {
	if current != nil && c.before.Before(current.Time) {
		return nil, errors.Errorf("current module release time: %s is after the before flag value: %s",
			current.Time.Format(time.DateOnly), c.before.Format(time.DateOnly))
	}
	versions, err := c.repo.GetVersions(path)
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
		lts, err := c.repo.GetInfo(path, versions[mid])
		if err != nil {
			return nil, err
		}
		if lts.Time.After(c.before) {
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
