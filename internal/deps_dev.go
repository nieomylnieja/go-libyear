package internal

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
)

func NewDepsDevClient() *DepsDevClient {
	return &DepsDevClient{
		http:   &http.Client{Timeout: 10 * time.Second},
		apiURL: url.URL{Scheme: "https", Host: "api.deps.dev"},
	}
}

type DepsDevClient struct {
	http   *http.Client
	apiURL url.URL
}

func (c DepsDevClient) GetVersions(path string) ([]*semver.Version, error) {
	path = url.PathEscape(path)
	resp, err := c.http.Get(c.apiURL.JoinPath("v3alpha/systems/go/packages", path).String())
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
	var data struct {
		Versions []struct {
			VersionKey struct {
				Version *semver.Version `json:"version"`
			} `json:"versionKey"`
		} `json:"versions"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	versions := make([]*semver.Version, 0, len(data.Versions))
	for _, v := range data.Versions {
		if v.VersionKey.Version == nil {
			continue
		}
		versions = append(versions, v.VersionKey.Version)
	}
	return versions, nil
}
