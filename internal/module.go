package internal

import (
	"fmt"
	"slices"
	"time"

	"github.com/Masterminds/semver"

	"golang.org/x/mod/modfile"
)

const ProgramName = "go-libyear"

// Module is a container used to decode GOPROXY and 'go list' responses
// and transport calculated metrics.
type Module struct {
	Path    string          `json:"Path"`
	Version *semver.Version `json:"Version"`
	Time    time.Time       `json:"Time"`

	Indirect bool    `json:"-"`
	Skipped  bool    `json:"-"`
	Latest   *Module `json:"-"`
	Libyear  float64 `json:"-"`
	// ReleasesDiff is the number of release versions between latest and current.
	ReleasesDiff int `json:"-"`
	// VersionsDiff is an array of 3 elements: major, minor and patch versions.
	VersionsDiff VersionsDiff `json:"-"`
	// AllPaths preceding this version, if any.
	// This field is only set for latest version.
	AllPaths []string `json:"-"`
}

type VersionsDiff [3]int64

func (v VersionsDiff) String() string {
	return fmt.Sprintf("[%d, %d, %d]", v[0], v[1], v[2])
}

func (v VersionsDiff) Add(y VersionsDiff) VersionsDiff {
	result := VersionsDiff{}
	for i := 0; i < 3; i++ {
		result[i] = v[i] + y[i]
	}
	return result
}

func ReadGoMod(content []byte) (mainModule *Module, modules []*Module, err error) {
	// Parse the go.mod file.
	modFile, err := modfile.Parse("go.mod", content, nil)
	if err != nil {
		return nil, nil, err
	}

	modules = make([]*Module, 0, len(modFile.Require))
	// List all dependencies, including any replace blocks, from the parsed go.mod file.
	for _, require := range modFile.Require {
		// Filter out replaced modules.
		if slices.ContainsFunc(modFile.Replace, func(replaced *modfile.Replace) bool {
			return replaced.Old.Path == require.Mod.Path
		}) {
			continue
		}
		version, err := semver.NewVersion(require.Mod.Version)
		if err != nil {
			return nil, nil, err
		}
		modules = append(modules, &Module{
			Path:     require.Mod.Path,
			Version:  version,
			Indirect: require.Indirect,
		})
	}
	if modFile.Module == nil {
		return nil, nil, fmt.Errorf("go.mod file does not contain module declaration")
	}
	mainModule = &Module{Path: modFile.Module.Mod.Path}
	return mainModule, modules, nil
}
