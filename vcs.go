package libyear

import (
	"strings"

	"github.com/nieomylnieja/go-libyear/internal"
	"github.com/pkg/errors"
)

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
			internal.NewGitVCS(cacheDir),
		},
	}
}

// VCSRegistry implementes [command.ModulesRepo] and delegates handling of an
// invoked method to the registered VCS handler which supports the given path.
type VCSRegistry struct {
	vcsHandlers []VCSHandler
}

// GetHandler returns the VCS handler which supports the given path.
func (v *VCSRegistry) GetHandler(path string) (ModulesRepo, error) {
	var handler VCSHandler
	for _, handler = range v.vcsHandlers {
		canHandle, err := handler.CanHandle(path)
		if err != nil {
			return nil, err
		}
		if canHandle {
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
