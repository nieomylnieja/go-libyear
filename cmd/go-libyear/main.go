package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	golibyear "github.com/nieomylnieja/go-libyear"
	"github.com/nieomylnieja/go-libyear/internal"

	"github.com/urfave/cli/v3"
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
	cmd := &cli.Command{
		Usage:     "Calculate Go module's libyear!",
		UsageText: usageText,
		Action:    run,
		Name:      internal.ProgramName,
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return fmt.Errorf("parse error: %w", err)
		},
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
			flagFindLatestMajor,
			flagNoLibyearCompensation,
			flagAgeLimit,
			flagVersion,
		},
		Suggest: true,
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cliCmd *cli.Command) error {
	if cliCmd.IsSet(flagVersion.Name) {
		return nil
	}

	ctx, watch := setupContextHandling(ctx, cliCmd)
	go watch()

	stdinUsed := isStdinUsed()
	if err := validateArgs(cliCmd, stdinUsed); err != nil {
		return err
	}

	var source golibyear.Source
	sourceArg := cliCmd.Args().Get(0)
	if sourceArg == "" && !stdinUsed {
		var err error
		sourceArg, err = defaultGoModPath()
		if err != nil {
			return err
		}
	}
	switch {
	case cliCmd.IsSet(flagPkg.Name):
		source = &golibyear.PkgSource{Pkg: sourceArg}
	case cliCmd.IsSet(flagURL.Name):
		source = golibyear.URLSource{RawURL: sourceArg, HTTP: http.Client{Timeout: 10 * time.Second}}
	case stdinUsed:
		source = golibyear.StdinSource{}
	default:
		source = golibyear.FileSource{Path: sourceArg}
	}

	var output golibyear.Output
	switch {
	case cliCmd.IsSet(flagJSON.Name):
		output = golibyear.JSONOutput{}
	case cliCmd.IsSet(flagCSV.Name):
		output = golibyear.CSVOutput{}
	default:
		output = golibyear.TableOutput{}
	}

	builder := golibyear.NewCommandBuilder(source, output)
	if cliCmd.IsSet(flagCache.Name) {
		builder = builder.WithCache(cliCmd.String(flagCacheFilePath.Name))
	}
	for flag, option := range flagToOption {
		if cliCmd.IsSet(flag) {
			builder = builder.WithOptions(option)
		}
	}
	if cliCmd.IsSet(flagVCSCacheDir.Name) {
		registry := golibyear.NewVCSRegistry(cliCmd.String(flagVCSCacheDir.Name))
		builder = builder.WithVCSRegistry(registry)
	}
	if cliCmd.IsSet(flagAgeLimit.Name) {
		builder = builder.WithAgeLimit(cliCmd.Timestamp(flagAgeLimit.Name))
	}
	if progress := newModuleProgress(); progress != nil {
		builder = builder.WithModuleProgress(progress)
	}

	cmd, err := builder.Build()
	if err != nil {
		return err
	}
	return cmd.Run(ctx)
}

func setupContextHandling(ctx context.Context, cliCmd *cli.Command) (handledCtx context.Context, handler func()) {
	errTimeout := errors.New("timeout")
	timeout := cliCmd.Duration(flagTimeout.Name)
	handledCtx, cancel := context.WithTimeoutCause(ctx, timeout, errTimeout)
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	return handledCtx, func() {
		select {
		case sig := <-sigCh:
			cancel()
			fmt.Fprintf(os.Stderr, "\r%s signal detected, shutting down...\n", sig)
			os.Exit(0)
		case <-handledCtx.Done():
			cause := context.Cause(handledCtx)
			if errors.Is(cause, errTimeout) {
				fmt.Fprintf(os.Stderr,
					"\r%s timeout exceeded, consider increasing the timeout value via --timeout flag\n", timeout)
			} else {
				fmt.Fprintf(os.Stderr, "\r%s, shutting down...\n", handledCtx.Err())
			}
			os.Exit(1)
		}
	}
}

func validateArgs(cliCmd *cli.Command, stdinUsed bool) error {
	if stdinUsed && (cliCmd.NArg() != 0 || cliCmd.IsSet(flagURL.Name) || cliCmd.IsSet(flagPkg.Name)) {
		return fmt.Errorf(
			"when reading go.mod from stdin no arguments or output related flags should be provided")
	}
	if cliCmd.NArg() > 1 {
		return errors.New("invalid number of arguments provided, expected at most one argument, path to go.mod")
	}
	if !stdinUsed && cliCmd.NArg() == 0 && (cliCmd.IsSet(flagURL.Name) || cliCmd.IsSet(flagPkg.Name)) {
		return errors.New("invalid number of arguments provided, expected a source argument")
	}

	for _, flags := range [][]string{
		{flagUseGoList.Name, flagPkg.Name},
		{flagCSV.Name, flagJSON.Name},
		{flagURL.Name, flagPkg.Name},
	} {
		if err := validateFlagsMutualExclusion(cliCmd, flags); err != nil {
			return err
		}
	}
	return nil
}

func defaultGoModPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findGoModPath(wd)
}

func findGoModPath(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path for %s: %w", start, err)
	}
	for {
		goModPath := filepath.Join(dir, "go.mod")
		info, err := os.Stat(goModPath)
		switch {
		case err == nil && info.IsDir():
			return "", fmt.Errorf("%s is a directory, expected go.mod file", goModPath)
		case err == nil:
			return goModPath, nil
		case !errors.Is(err, os.ErrNotExist):
			return "", fmt.Errorf("stat %s: %w", goModPath, err)
		}

		gitBoundary, err := isGitBoundary(dir)
		if err != nil {
			return "", err
		}
		if gitBoundary {
			return "", fmt.Errorf("could not find go.mod before reaching git boundary at %s", dir)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find go.mod from %s or any parent directory", start)
		}
		dir = parent
	}
}

func isGitBoundary(dir string) (bool, error) {
	gitPath := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", gitPath, err)
	}
	return true, nil
}

func isStdinUsed() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice == 0
}

func newModuleProgress() golibyear.ModuleProgress {
	if !isTerminal(os.Stderr) {
		return nil
	}
	return internal.NewModuleSpinner(os.Stderr)
}

func isTerminal(file *os.File) bool {
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

func validateFlagsMutualExclusion(cliCmd *cli.Command, flags []string) error {
	var flagSet string
	for _, flag := range flags {
		if !cliCmd.IsSet(flag) {
			continue
		}
		if flagSet != "" {
			return fmt.Errorf("use either --%s or --%s flag, but not both", flagSet, flag)
		}
		flagSet = flag
	}
	return nil
}
