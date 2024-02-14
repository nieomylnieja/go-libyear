package internal

import (
	"bytes"
	"os/exec"

	"github.com/pkg/errors"
)

func execCmd(name string, arg ...string) (*bytes.Buffer, error) {
	// #nosec G204
	cmd := exec.Command(name, arg...)
	if cmd.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if cmd.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, errors.Errorf("Failed to execute '%s' command: %s", cmd, stderr.String())
	}
	return &stdout, nil
}
