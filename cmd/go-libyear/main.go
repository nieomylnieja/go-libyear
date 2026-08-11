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
	"golang.org/x/term"
)

// Set by build ldflags.
var (
	BuildVersion string
	BuildGitTag  string
	BuildDate    string
)

//go:embed usage.txt
var usageText string

//go:embed history-usage.txt
var historyUsageText string

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
		Flags:    rootFlags(),
		Commands: []*cli.Command{historyCommand()},
		Suggest:  true,
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

	source, err := newSource(cliCmd, stdinUsed)
	if err != nil {
		return err
	}
	output := newOutput(cliCmd)

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

func historyCommand() *cli.Command {
	return &cli.Command{
		Name:      "history",
		Usage:     "Calculate and chart historical libyear samples",
		UsageText: historyUsageText,
		Action:    runHistory,
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
			flagFindLatestMajor,
			flagNoLibyearCompensation,
			flagHistoryFrom,
			flagHistoryTo,
			flagHistoryInterval,
			flagHistoryWidth,
		},
	}
}

func rootFlags() []cli.Flag {
	return []cli.Flag{
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
	}
}

func runHistory(ctx context.Context, cliCmd *cli.Command) error {
	ctx, watch := setupContextHandling(ctx, cliCmd)
	go watch()

	stdinUsed := isStdinUsed()
	if err := validateHistoryArgs(cliCmd, stdinUsed); err != nil {
		return err
	}

	source, err := newSource(cliCmd, stdinUsed)
	if err != nil {
		return err
	}
	output, err := newHistoryOutput(cliCmd)
	if err != nil {
		return err
	}
	from, to, interval, err := historyRangeFromFlags(cliCmd)
	if err != nil {
		return err
	}

	builder := golibyear.NewHistoryCommandBuilder(source, output).WithRange(from, to, interval)
	if cliCmd.IsSet(flagCache.Name) {
		builder = builder.WithCache(cliCmd.String(flagCacheFilePath.Name))
	}
	for flag, option := range historyFlagToOption {
		if cliCmd.IsSet(flag) {
			builder = builder.WithOptions(option)
		}
	}
	if cliCmd.IsSet(flagVCSCacheDir.Name) {
		registry := golibyear.NewVCSRegistry(cliCmd.String(flagVCSCacheDir.Name))
		builder = builder.WithVCSRegistry(registry)
	}
	if progress := newHistoryProgress(); progress != nil {
		builder = builder.
			WithHistoryProgress(progress).
			WithHistoryModuleProgress(func() golibyear.ModuleProgress {
				return progress.NewModuleProgress()
			})
	}
	builder = builder.WithIgnoredModuleErrors(os.Stderr)

	cmd, err := builder.Build()
	if err != nil {
		return err
	}
	return cmd.Run(ctx)
}

func newSource(cliCmd *cli.Command, stdinUsed bool) (golibyear.Source, error) {
	sourceArg := cliCmd.Args().Get(0)
	if sourceArg == "" && !stdinUsed {
		var err error
		sourceArg, err = defaultGoModPath()
		if err != nil {
			return nil, err
		}
	}
	switch {
	case cliCmd.IsSet(flagPkg.Name):
		return &golibyear.PkgSource{Pkg: sourceArg}, nil
	case cliCmd.IsSet(flagURL.Name):
		return golibyear.URLSource{RawURL: sourceArg, HTTP: http.Client{Timeout: 10 * time.Second}}, nil
	case stdinUsed:
		return golibyear.StdinSource{}, nil
	default:
		return golibyear.FileSource{Path: sourceArg}, nil
	}
}

func newOutput(cliCmd *cli.Command) golibyear.Output {
	switch {
	case cliCmd.IsSet(flagJSON.Name):
		return golibyear.JSONOutput{}
	case cliCmd.IsSet(flagCSV.Name):
		return golibyear.CSVOutput{}
	default:
		return golibyear.TableOutput{}
	}
}

func newHistoryOutput(cliCmd *cli.Command) (golibyear.HistoryOutput, error) {
	switch {
	case cliCmd.IsSet(flagJSON.Name):
		return golibyear.HistoryJSONOutput{}, nil
	case cliCmd.IsSet(flagCSV.Name):
		return golibyear.HistoryCSVOutput{}, nil
	default:
		width, err := historyChartWidth(cliCmd)
		if err != nil {
			return nil, err
		}
		return golibyear.HistoryChartOutput{Width: width}, nil
	}
}

func historyRangeFromFlags(cliCmd *cli.Command) (
	from time.Time,
	to time.Time,
	interval time.Duration,
	err error,
) {
	if !cliCmd.IsSet(flagHistoryFrom.Name) {
		return time.Time{}, time.Time{}, 0, errors.New("--from is required")
	}
	from = cliCmd.Timestamp(flagHistoryFrom.Name)
	to = time.Now().UTC()
	if cliCmd.IsSet(flagHistoryTo.Name) {
		to = cliCmd.Timestamp(flagHistoryTo.Name)
	}
	return from, to, cliCmd.Duration(flagHistoryInterval.Name), nil
}

func historyChartWidth(cliCmd *cli.Command) (int, error) {
	if cliCmd.IsSet(flagHistoryWidth.Name) {
		width := cliCmd.Int(flagHistoryWidth.Name)
		if width < golibyear.MinimumHistoryChartWidth {
			return 0, fmt.Errorf("--width must be at least %d", golibyear.MinimumHistoryChartWidth)
		}
		return width, nil
	}
	if !isTerminal(os.Stdout) {
		return 80, nil
	}
	width, _, _ := term.GetSize(int(os.Stdout.Fd()))
	if width <= 0 {
		return 80, nil
	}
	if width < golibyear.MinimumHistoryChartWidth {
		return 0, fmt.Errorf(
			"terminal is %d columns wide; history chart requires at least %d",
			width,
			golibyear.MinimumHistoryChartWidth,
		)
	}
	return width, nil
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
	if err := validateSourceArgs(cliCmd, stdinUsed); err != nil {
		return err
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

func validateHistoryArgs(cliCmd *cli.Command, stdinUsed bool) error {
	if err := validateSourceArgs(cliCmd, stdinUsed); err != nil {
		return err
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
	if !cliCmd.IsSet(flagHistoryFrom.Name) {
		return errors.New("--from is required")
	}
	from := cliCmd.Timestamp(flagHistoryFrom.Name)
	to := time.Now().UTC()
	if cliCmd.IsSet(flagHistoryTo.Name) {
		to = cliCmd.Timestamp(flagHistoryTo.Name)
	}
	if from.After(to) {
		return errors.New("--from must be before or equal to --to")
	}
	if interval := cliCmd.Duration(flagHistoryInterval.Name); interval <= 0 {
		return errors.New("--interval must be greater than zero")
	}
	if cliCmd.IsSet(flagHistoryWidth.Name) && cliCmd.Int(flagHistoryWidth.Name) < golibyear.MinimumHistoryChartWidth {
		return fmt.Errorf("--width must be at least %d", golibyear.MinimumHistoryChartWidth)
	}
	return nil
}

func validateSourceArgs(cliCmd *cli.Command, stdinUsed bool) error {
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

func newHistoryProgress() *internal.HistoryProgressBar {
	if !isTerminal(os.Stderr) {
		return nil
	}
	return internal.NewHistoryProgressBar(os.Stderr)
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
