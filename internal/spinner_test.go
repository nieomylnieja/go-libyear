package internal

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestModuleSpinner_FinishClearsProgressLine(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	spinner := NewModuleSpinner(&output)

	spinner.Start(1)
	spinner.AdvanceModule("example.com/module")
	spinner.Finish()

	actual := output.String()
	assert.Regexp(t, regexp.MustCompile(`Scanning 1/1 modules \[[^]]+\]: example\.com/module`), actual)
	assert.NotRegexp(t, regexp.MustCompile(`example\.com/module\s+\[[^]]+\]`), actual)
	assert.Regexp(t, regexp.MustCompile(`Scanning 1/1 modules \[[^]]+\]: example\.com/module.*\r +\r$`), actual)
}

func TestHistoryProgressBar_FinishClearsProgressLines(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	progress := NewHistoryProgressBar(&output)
	timestamp := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

	progress.Start(2)
	progress.AdvanceSample(timestamp)
	progress.Finish()

	actual := output.String()
	assert.Regexp(t, regexp.MustCompile(`Sampling 1/2 history \[[^]]+\]: 2022-01-01`), actual)
	assert.True(t, strings.HasSuffix(actual, historyProgressClearSequence()))
}

func TestHistoryProgressBar_StartRendersImmediately(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	progress := NewHistoryProgressBar(&output)

	progress.Start(2)
	progress.Finish()

	actual := output.String()
	assert.Regexp(t, regexp.MustCompile(`\r\x1b\[2KSampling 0/2 history \[[^]]+\]`), actual)
	assert.Contains(t, actual, "\n"+ansiClearLine)
}

func TestHistoryProgressBar_StartSampleRendersCurrentSample(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	progress := NewHistoryProgressBar(&output)
	timestamp := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

	progress.Start(2)
	progress.StartSample(timestamp)
	progress.Finish()

	actual := output.String()
	assert.Regexp(t, regexp.MustCompile(`Sampling 0/2 history \[[^]]+\]: 2022-01-01`), actual)
	assert.Contains(t, actual, ansiCursorUpOneLine+ansiClearLine)
}

func TestHistoryProgressBar_ModuleProgressRendersBelowHistory(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	progress := NewHistoryProgressBar(&output)
	timestamp := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

	progress.Start(2)
	defer progress.Finish()
	progress.StartSample(timestamp)
	moduleProgress := progress.NewModuleProgress()
	moduleProgress.Start(2)
	moduleProgress.AdvanceModule("example.com/module")

	actual := output.String()
	assert.Regexp(
		t,
		regexp.MustCompile(
			"\r\x1b\\[2KSampling 0/2 history \\[[^]]+\\]: 2022-01-01.*\n"+
				"\r\x1b\\[2K- Scanning 1/2 modules \\[[^]]+\\]: example\\.com/module",
		),
		actual,
	)
}

func historyProgressClearSequence() string {
	return ansiCursorUpOneLine + ansiClearLine + "\n" + ansiClearLine + ansiCursorUpOneLine + "\r"
}
