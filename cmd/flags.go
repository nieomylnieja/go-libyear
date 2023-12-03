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
	flagIndirect.Name:  golibyear.OptionIncludeIndirect,
	flagSkipFresh.Name: golibyear.OptionSkipFresh,
	flagReleases.Name:  golibyear.OptionShowReleases,
	flagVersions.Name:  golibyear.OptionShowVersions,
	flagUseGoList.Name: golibyear.OptionUseGoList,
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
		Action: func(c *cli.Context, path cli.Path) error {
			if !c.IsSet("cache") {
				return errors.Errorf("--cache-file-path flag can only be used in conjunction with --cache")
			}
			return nil
		},
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
	flagVersion = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{"v"},
		Usage:   "Show the program version",
		Action: func(context *cli.Context, b bool) error {
			fmt.Printf("Version: %s\nGitTag: %s\nBuildDate: %s\n",
				BuildVersion, BuildGitTag, BuildDate)
			return nil
		},
	}
)
