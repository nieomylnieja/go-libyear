package libyear

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/mod/module"

	"github.com/nieomylnieja/go-libyear/internal"
)

//go:generate mockgen -destination internal/mocks/vcs.go -package mocks -typed . VCSHandler

// VCSHandler is an interface that can be implemented by specifc VCS handler.
type VCSHandler interface {
	ModulesRepo
	// CanHandle reports whether the vcs can handle the given path.
	CanHandle(path string) (bool, error)
	// Name reports the name of the VCS system.
	Name() string
}

func NewVCSRegistry(cacheDir string) *VCSRegistry {
	return &VCSRegistry{
		vcsHandlers: []VCSHandler{
			internal.NewGitVCS(cacheDir, internal.GitCmd{}),
		},
		goprivate: os.Getenv("GOPRIVATE"),
	}
}

// VCSRegistry implements [command.ModulesRepo] and delegates handling of an
// invoked method to the registered VCS handler which supports the given path.
type VCSRegistry struct {
	vcsHandlers []VCSHandler
	goprivate   string
}

func (v *VCSRegistry) IsPrivate(path string) bool {
	return module.MatchPrefixPatterns(v.goprivate, path)
}

// GetHandler returns the VCS handler which supports the given path.
// nolint: ireturn
func (v *VCSRegistry) GetHandler(path string) (ModulesRepo, error) {
	var handler VCSHandler
	for _, h := range v.vcsHandlers {
		canHandle, err := h.CanHandle(path)
		if err != nil {
			return nil, err
		}
		if canHandle {
			handler = h
			break
		}
	}
	if handler == nil {
		return nil, errors.Errorf(
			"private module path: '%s' cannot be handled by any supported VCS [%s]",
			path, v.supportedVCS())
	}
	return handler, nil
}

func (v *VCSRegistry) supportedVCS() string {
	strs := make([]string, 0, len(v.vcsHandlers))
	for _, handler := range v.vcsHandlers {
		strs = append(strs, handler.Name())
	}
	return strings.Join(strs, ", ")
}
