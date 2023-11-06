package libyear

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Masterminds/semver"

	"github.com/pkg/errors"
)

type Source interface {
	Read() ([]byte, error)
}

type PkgSource struct {
	Pkg  string
	repo ModulesRepo
}

func (p *PkgSource) Read() ([]byte, error) {
	path := p.Pkg
	var version *semver.Version
	if strings.Contains(p.Pkg, "@") {
		split := strings.Split(path, "@")
		if len(split) != 2 {
			return nil, errors.New("invalid pkg name provided, expected version after @ char")
		}
		path = split[0]
		var err error
		version, err = semver.NewVersion(split[1])
		if err != nil {
			return nil, err
		}
	} else {
		// .mod endpoint does not support 'latest' version literal, we need an exact semver.
		latest, err := p.repo.GetLatestInfo(path)
		if err != nil {
			return nil, err
		}
		version = latest.Version
	}
	return p.repo.GetModFile(path, version)
}

func (p *PkgSource) SetModulesRepo(repo ModulesRepo) {
	p.repo = repo
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
		return nil, errors.Errorf(
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

type StdinSource struct{}

func (s StdinSource) Read() ([]byte, error) {
	return io.ReadAll(os.Stdin)
}
