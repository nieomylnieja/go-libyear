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
	spinner.Advance()
	spinner.Finish()

	actual := output.String()
	assert.Contains(t, actual, "Scanning 1/1 modules")
	assert.Regexp(t, regexp.MustCompile(`Scanning 1/1 modules.*\r +\r\n$`), actual)
}
