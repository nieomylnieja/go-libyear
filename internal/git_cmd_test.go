package internal

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetHeadBranchName(t *testing.T) {
	tests := []struct {
		Input  string
		Branch string
	}{
		{Input: "origin/HEAD -> origin/main", Branch: "main"},
		{Input: "  origin/HEAD -> origin/main\n", Branch: "main"},
		{
			Input: `  origin/HEAD -> origin/main
  origin/HEAD -> origin`,
			Branch: "main",
		},
	}
	for _, test := range tests {
		branch, err := getHeadBranchName(bytes.NewBufferString(test.Input))
		require.NoError(t, err)
		assert.Equal(t, test.Branch, branch)
	}
}
