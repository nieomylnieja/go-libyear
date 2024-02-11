package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	golibyear "github.com/nieomylnieja/go-libyear"
	"github.com/nieomylnieja/go-libyear/internal"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

// Set by build ldflags.
var (
	BuildVersion string
	BuildGitTag  string
	BuildDate    string
)

//go:embed usage.txt
var usageText string

func main() {
	log.SetOutput(os.Stderr)
	app := &cli.App{
		Usage:     "Calculate Go module's libyear!",
		UsageText: usageText,
		Action:    run,
		Name:      internal.ProgramName,
		Flags: []cli.Flag{
			flagURL,
			flagPkg,
			flagCSV,
			flagJSON,
			flagCache,
			flagCacheFilePath,
			flagVCSCacheDir,
			flagTimeout,
			flagUseGoList,
			flagIndirect,
			flagSkipFresh,
			flagReleases,
			flagVersions,
			flagVersion,
			flagFindLatestMajor,
			flagNoLibyearCompensation,
		},
		Suggest: true,
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cliCtx *cli.Context) error {
	if cliCtx.IsSet(flagVersion.Name) {
		return nil
	}

	ctx, watch := setupContextHandling(cliCtx)
	go watch()

	stdinUsed := isStdinUsed()
	if err := validateArgs(cliCtx, stdinUsed); err != nil {
		return err
	}

	var source golibyear.Source
	sourceArg := cliCtx.Args().Get(0)
	switch {
	case cliCtx.IsSet(flagPkg.Name):
		source = &golibyear.PkgSource{Pkg: sourceArg}
	case cliCtx.IsSet(flagURL.Name):
		source = golibyear.URLSource{RawURL: sourceArg, HTTP: http.Client{Timeout: 10 * time.Second}}
	case stdinUsed:
		source = golibyear.StdinSource{}
	default:
		source = golibyear.FileSource{Path: sourceArg}
	}

	var output golibyear.Output
	switch {
	case cliCtx.IsSet(flagJSON.Name):
		output = golibyear.JSONOutput{}
	case cliCtx.IsSet(flagCSV.Name):
		output = golibyear.CSVOutput{}
	default:
		output = golibyear.TableOutput{}
	}

	builder := golibyear.NewCommandBuilder(source, output)
	if cliCtx.IsSet(flagCache.Name) {
		builder = builder.WithCache(flagCacheFilePath.Get(cliCtx))
	}
	for flag, option := range flagToOption {
		if cliCtx.IsSet(flag) {
			builder = builder.WithOptions(option)
		}
	}
	if cliCtx.IsSet(flagVCSCacheDir.Name) {
		registry := golibyear.NewVCSRegistry(flagVCSCacheDir.Get(cliCtx))
		builder = builder.WithVCSRegistry(registry)
	}

	cmd, err := builder.Build()
	if err != nil {
		return err
	}
	return cmd.Run(ctx)
}

func setupContextHandling(cliCtx *cli.Context) (ctx context.Context, handler func()) {
	ctx = cliCtx.Context
	errTimeout := errors.New("timeout")
	timeout := flagTimeout.Get(cliCtx)
	ctx, cancel := context.WithTimeoutCause(ctx, timeout, errTimeout)
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	return ctx, func() {
		select {
		case sig := <-sigCh:
			cancel()
			fmt.Fprintf(os.Stderr, "\r%s signal detected, shutting down...\n", sig)
			os.Exit(0)
		case <-ctx.Done():
			cause := context.Cause(ctx)
			if errors.Is(cause, errTimeout) {
				fmt.Fprintf(os.Stderr,
					"\r%s timeout exceeded, consider increasing the timeout value via --timeout flag\n", timeout)
			} else {
				fmt.Fprintf(os.Stderr, "\r%s, shutting down...\n", ctx.Err())
			}
			os.Exit(1)
		}
	}
}

func validateArgs(cliCtx *cli.Context, stdinUsed bool) error {
	if cliCtx.NArg() != 1 && !stdinUsed {
		return errors.New("invalid number of arguments provided, expected a single argument, path to go.mod")
	}
	if stdinUsed && (cliCtx.NArg() != 0 || cliCtx.IsSet(flagURL.Name) || cliCtx.IsSet(flagPkg.Name)) {
		return errors.Errorf(
			"when reading go.mod from stdin no arguments or output related flags should be provided")
	}

	for _, flags := range [][]string{
		{flagUseGoList.Name, flagPkg.Name},
		{flagCSV.Name, flagJSON.Name},
		{flagURL.Name, flagPkg.Name},
	} {
		if err := validateFlagsMutualExclusion(cliCtx, flags); err != nil {
			return err
		}
	}
	return nil
}

func isStdinUsed() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice == 0
}

func validateFlagsMutualExclusion(cliCtx *cli.Context, flags []string) error {
	var flagSet string
	for _, flag := range flags {
		if !cliCtx.IsSet(flag) {
			continue
		}
		if flagSet != "" {
			return errors.Errorf("use either --%s or --%s flag, but not both", flagSet, flag)
		}
		flagSet = flag
	}
	return nil
}
