package libyear

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver"
)

type Source interface {
	Read() ([]byte, error)
}

type PkgSource struct {
	Pkg  string
	repo ModulesRepo
	vcs  *VCSRegistry
}

func (p *PkgSource) Read() ([]byte, error) {
	path := p.Pkg
	repo := p.repo
	var version *semver.Version
	if strings.Contains(p.Pkg, "@") {
		split := strings.Split(path, "@")
		if len(split) != 2 {
			return nil, errors.New("invalid pkg name provided, expected version after @ char")
		}
		path = split[0]
		if split[1] != "latest" {
			var err error
			version, err = semver.NewVersion(split[1])
			if err != nil {
				return nil, err
			}
		}
	}
	if p.vcs.IsPrivate(path) {
		var err error
		repo, err = p.vcs.GetHandler(path)
		if err != nil {
			return nil, err
		}
	}
	if version == nil {
		// .mod endpoint does not support 'latest' version literal, we need an exact semver.
		latest, err := repo.GetLatestInfo(path)
		if err != nil {
			return nil, err
		}
		version = latest.Version
	}
	return repo.GetModFile(path, version)
}

func (p *PkgSource) SetModulesRepo(repo ModulesRepo) {
	p.repo = repo
}

func (p *PkgSource) SetVCSRegistry(registry *VCSRegistry) {
	p.vcs = registry
}

type URLSource struct {
	HTTP   http.Client
	RawURL string
}

func (s URLSource) Read() ([]byte, error) {
	u, err := url.Parse(s.RawURL)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTP.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"unexpected response status code: %d, body: %s",
			resp.StatusCode, string(data))
	}
	return io.ReadAll(resp.Body)
}

type FileSource struct {
	Path string
}

func (s FileSource) Read() ([]byte, error) {
	return os.ReadFile(s.Path)
}

func (s FileSource) readHistory(ctx context.Context, timestamp time.Time) ([]byte, error) {
	absPath, err := filepath.Abs(s.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute path for %s: %w", s.Path, err)
	}
	repoRoot, err := gitOutputString(ctx, filepath.Dir(absPath), "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("find git repository for %s: %w", s.Path, err)
	}
	repoRoot = filepath.Clean(repoRoot)

	relPath, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return nil, fmt.Errorf("resolve %s relative to git repository %s: %w", absPath, repoRoot, err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
		return nil, fmt.Errorf("%s is outside git repository %s", absPath, repoRoot)
	}
	relPath = filepath.ToSlash(relPath)

	cutoff := timestamp.UTC().Format(time.RFC3339)
	commit, err := gitOutputString(
		ctx,
		repoRoot,
		"rev-list",
		"-1",
		"--before="+cutoff,
		"HEAD",
		"--",
		relPath,
	)
	if err != nil {
		return nil, fmt.Errorf("find git revision for %s at or before %s: %w", relPath, cutoff, err)
	}
	if commit == "" {
		return nil, fmt.Errorf("no git revision found for %s at or before %s", relPath, cutoff)
	}

	data, err := gitOutput(ctx, repoRoot, "show", commit+":"+relPath)
	if err != nil {
		return nil, fmt.Errorf("read %s from git revision %s: %w", relPath, commit, err)
	}
	return data, nil
}

func gitOutputString(ctx context.Context, dir string, args ...string) (string, error) {
	data, err := gitOutput(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func gitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	// #nosec G204
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return nil, fmt.Errorf("%w: %s", err, stderrText)
		}
		return nil, err
	}
	return data, nil
}

type StdinSource struct{}

func (s StdinSource) Read() ([]byte, error) {
	return io.ReadAll(os.Stdin)
}
