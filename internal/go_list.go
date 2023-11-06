package internal

import (
	"bytes"
	"encoding/json"
	"os/exec"

	"github.com/Masterminds/semver"

	"github.com/pkg/errors"
)

func NewGoListExecutor(useCache bool, cacheFilePath string) (*GoListExecutor, error) {
	var cache modulesCache
	if useCache {
		var err error
		cache, err = NewCache(cacheFilePath)
		if err != nil {
			return nil, err
		}
	}
	return &GoListExecutor{cache: cache}, nil
}

type GoListExecutor struct {
	cache modulesCache
}

func (e *GoListExecutor) GetVersions(path string) ([]*semver.Version, error) {
	out, err := e.exec("-versions", path)
	if err != nil {
		return nil, err
	}
	var versions struct {
		Path     string            `json:"Path"`
		Versions []*semver.Version `json:"Versions"`
	}
	if err = json.NewDecoder(out).Decode(&versions); err != nil {
		return nil, err
	}
	return versions.Versions, nil
}

func (e *GoListExecutor) GetInfo(path string, version *semver.Version) (*Module, error) {
	return e.getInfo(path, version, false)
}

func (e *GoListExecutor) GetLatestInfo(path string) (*Module, error) {
	return e.getInfo(path, nil, true)
}

func (e *GoListExecutor) getInfo(path string, version *semver.Version, latest bool) (*Module, error) {
	var versionStr string
	if latest {
		versionStr = "latest"
	} else {
		versionStr = version.String()
	}
	// Try loading from cache.
	if e.cache != nil {
		m, loaded := e.cache.Load(path, version)
		if loaded {
			return m, nil
		}
	}
	// Fetch module details.
	out, err := e.exec(path + "@v" + versionStr)
	if err != nil {
		return nil, err
	}
	var m Module
	if err = json.NewDecoder(out).Decode(&m); err != nil {
		return nil, err
	}
	// Save to cache.
	if e.cache != nil {
		if err = e.cache.Save(&m); err != nil {
			return nil, err
		}
	}
	return &m, nil
}

func (e *GoListExecutor) GetModFile(_ string, _ *semver.Version) ([]byte, error) {
	return nil, errors.New("retrieving go.mod file using GoListExecutor is not supported")
}

func (e *GoListExecutor) exec(args ...string) (*bytes.Buffer, error) {
	// #nosec G204
	cmd := exec.Command("go", append([]string{"list", "-json", "-m", "-mod=readonly"}, args...)...)
	if cmd.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if cmd.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, errors.Errorf("Failed to execute '%s' command: %s", cmd, stderr.String())
	}
	return &stdout, nil
}
