package internal

import (
	"bytes"
	"regexp"
	"testing"

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
	assert.Contains(t, actual, "Scanning 1/1 modules: example.com/module")
	assert.Regexp(t, regexp.MustCompile(`Scanning 1/1 modules: example\.com/module.*\r +\r$`), actual)
}
