package main

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

	golibyear "github.com/nieomylnieja/go-libyear"
)

const (
	categorySource = "Source:"
	categoryOutput = "Output:"
	categoryCache  = "Cache:"
)

var flagToOption = map[string]golibyear.Option{
	flagIndirect.Name:              golibyear.OptionIncludeIndirect,
	flagSkipFresh.Name:             golibyear.OptionSkipFresh,
	flagReleases.Name:              golibyear.OptionShowReleases,
	flagVersions.Name:              golibyear.OptionShowVersions,
	flagUseGoList.Name:             golibyear.OptionUseGoList,
	flagFindLatestMajor.Name:       golibyear.OptionFindLatestMajor,
	flagNoLibyearCompensation.Name: golibyear.OptionNoLibyearCompensation,
}

var (
	flagURL = &cli.BoolFlag{
		Name:     "url",
		Aliases:  []string{"u"},
		Usage:    "Fetch go.mod from URL",
		Category: categorySource,
	}
	flagPkg = &cli.BoolFlag{
		Name:     "pkg",
		Aliases:  []string{"p"},
		Usage:    "Fetch go.mod from pkg index",
		Category: categorySource,
	}
	flagJSON = &cli.BoolFlag{
		Name:     "json",
		Usage:    "Output using JSON format",
		Category: categoryOutput,
	}
	flagCSV = &cli.BoolFlag{
		Name:     "csv",
		Usage:    "Output using CSV format",
		Category: categoryOutput,
	}
	flagCache = &cli.BoolFlag{
		Name:     "cache",
		Usage:    "Use cache",
		Category: categoryCache,
	}
	flagCacheFilePath = &cli.PathFlag{
		Name:        "cache-file-path",
		Usage:       "Use custom cache file path",
		DefaultText: "$XDG_CACHE_HOME/go-libyear/modules or $HOME/.cache/go-libyear/modules",
		Category:    categoryCache,
		Action:      useOnlyWith[cli.Path]("cache-file-path", flagCache.Name),
	}
	flagVCSCacheDir = &cli.PathFlag{
		Name:        "vcs-cache-dir",
		Usage:       "Use custom cache directory for VCS modules (downloaded due to GOPRIVATE settings)",
		DefaultText: "$XDG_CACHE_HOME/go-libyear/vcs or $HOME/.cache/go-libyear/vcs",
		Category:    categoryCache,
	}
	flagTimeout = &cli.DurationFlag{
		Name:    "timeout",
		Aliases: []string{"t"},
		Value:   1 * time.Minute,
		Usage:   "Set timeout for the command",
	}
	flagUseGoList = &cli.BoolFlag{
		Name:  "go-list",
		Usage: "Use 'go list -m' instead of GOPROXY API",
	}
	flagIndirect = &cli.BoolFlag{
		Name:     "indirect",
		Aliases:  []string{"i"},
		Usage:    "Include indirect dependencies",
		Category: categoryOutput,
	}
	flagSkipFresh = &cli.BoolFlag{
		Name:     "skip-fresh",
		Usage:    "Skip up-to-date dependencies",
		Category: categoryOutput,
	}
	flagReleases = &cli.BoolFlag{
		Name:     "releases",
		Usage:    "Display the number of releases between current and newest versions",
		Category: categoryOutput,
	}
	flagVersions = &cli.BoolFlag{
		Name:     "versions",
		Usage:    "Display the number of major, minor, and patch versions between current and newest versions",
		Category: categoryOutput,
	}
	flagFindLatestMajor = &cli.BoolFlag{
		Name:    "find-latest-major",
		Aliases: []string{"M"},
		Usage:   "Use next, greater than or equal to v2 version as the latest",
	}
	flagNoLibyearCompensation = &cli.BoolFlag{
		Name: "no-libyear-compensation",
		Usage: "Do not compensate for negative or zero libyear " +
			"values if latest version was published before current version",
		Action: useOnlyWith[bool]("no-libyear-compensation", flagFindLatestMajor.Name),
	}
	flagAgeLimit = &cli.TimestampFlag{
		Name:   "age-limit",
		Layout: time.RFC3339,
		Usage:  "Only consider versions which were published before or at the specified date",
	}
	flagVersion = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{"v"},
		Usage:   "Show the program version",
		Action: func(_ *cli.Context, _ bool) error {
			fmt.Printf("Version: %s\nGitTag: %s\nBuildDate: %s\n",
				BuildVersion, BuildGitTag, BuildDate)
			return nil
		},
	}
)

// useOnlyWith creates an action which will verify if this flag was used with the dependent flag.
func useOnlyWith[T any](this, dependent string) func(*cli.Context, T) error {
	return func(ctx *cli.Context, _ T) error {
		if !ctx.IsSet(dependent) {
			return errors.Errorf("--%s flag can only be used in conjunction with --%s", this, dependent)
		}
		return nil
	}
}
