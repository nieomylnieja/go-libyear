package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver"
)

func NewGoProxyClient(useCache bool, cacheFilePath string) (*GoProxyClient, error) {
	var cache modulesCache
	if useCache {
		var err error
		cache, err = NewCache(cacheFilePath)
		if err != nil {
			return nil, err
		}
	}
	apiURL := url.URL{Scheme: "https", Host: "proxy.golang.org"}
	if proxyURL, isSet := os.LookupEnv("GOPROXY"); isSet {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse $GOPROXY url: %w", err)
		}
		apiURL = *u
	}
	return &GoProxyClient{
		http:   &http.Client{Timeout: 10 * time.Second},
		apiURL: apiURL,
		cache:  cache,
	}, nil
}

// GoProxyClient is used to interact with Golang proxy server.
// Details on GOPROXY protocol can be found here: https://go.dev/ref/mod#goproxy-protocol.
type GoProxyClient struct {
	http   *http.Client
	apiURL url.URL
	cache  modulesCache
}

const (
	getModFileFmt     = "%s/@v/v%s.mod"
	getLatestInfoFmt  = "%s/@latest"
	getVersionInfoFmt = "%s/@v/v%s.info"
	getVersionsFmt    = "%s/@v/list"
)

func (c *GoProxyClient) GetInfo(path string, version *semver.Version) (*Module, error) {
	return c.getInfo(path, version, false)
}

func (c *GoProxyClient) GetLatestInfo(path string) (*Module, error) {
	m, err := c.getInfo(path, nil, true)
	if err == nil {
		return m, nil
	}
	if !isProxyNotFound(err) {
		return nil, err
	}
	m, fallbackErr := c.getLatestInfoFromVersions(path)
	if fallbackErr != nil {
		return nil, err
	}
	return m, nil
}

func (c *GoProxyClient) getInfo(path string, version *semver.Version, latest bool) (*Module, error) {
	// Try loading from cache.
	if version != nil && c.cache != nil {
		m, loaded := c.cache.Load(path, version)
		if loaded {
			return m, nil
		}
	}
	escapedPath := escapePath(path)
	var urlPath string
	if latest {
		urlPath = fmt.Sprintf(getLatestInfoFmt, escapedPath)
	} else {
		urlPath = fmt.Sprintf(getVersionInfoFmt, escapedPath, version)
	}
	data, err := c.query(urlPath)
	if err != nil {
		return nil, err
	}
	var m Module
	if err = json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	m.Path = path
	// Save to cache.
	if c.cache != nil {
		if err = c.cache.Save(&m); err != nil {
			return nil, err
		}
	}
	return &m, nil
}

func (c *GoProxyClient) GetVersions(path string) ([]*semver.Version, error) {
	path = escapePath(path)
	data, err := c.query(fmt.Sprintf(getVersionsFmt, path))
	if err != nil {
		return nil, err
	}
	rawVersions := strings.Split(string(data), "\n")
	versions := make([]*semver.Version, 0, len(rawVersions))
	for _, raw := range rawVersions {
		if raw == "" {
			continue
		}
		v, err := semver.NewVersion(raw)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func (c *GoProxyClient) getLatestInfoFromVersions(path string) (*Module, error) {
	versions, err := c.GetVersions(path)
	if err != nil {
		return nil, err
	}
	sort.Sort(semver.Collection(versions))
	var latestErr error
	for i := len(versions) - 1; i >= 0; i-- {
		m, err := c.GetInfo(path, versions[i])
		if err == nil {
			return m, nil
		}
		if !isProxyNotFound(err) {
			return nil, err
		}
		latestErr = err
	}
	if latestErr != nil {
		return nil, latestErr
	}
	return nil, fmt.Errorf("no versions found for path %s", path)
}

func (c *GoProxyClient) GetModFile(path string, version *semver.Version) ([]byte, error) {
	urlPath := fmt.Sprintf(getModFileFmt, escapePath(path), version)
	return c.query(urlPath)
}

func (c *GoProxyClient) query(urlPath string) ([]byte, error) {
	resp, err := c.http.Get(c.apiURL.JoinPath(urlPath).String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, &proxyResponseError{
			method:     http.MethodGet,
			url:        resp.Request.URL.String(),
			statusCode: resp.StatusCode,
			body:       string(data),
		}
	}
	return io.ReadAll(resp.Body)
}

type proxyResponseError struct {
	method     string
	url        string
	statusCode int
	body       string
}

func (e *proxyResponseError) Error() string {
	return fmt.Sprintf(
		"unexpected response status code from %s %s: %d, body: %s",
		e.method, e.url, e.statusCode, e.body)
}

func isProxyNotFound(err error) bool {
	var responseErr *proxyResponseError
	if !errors.As(err, &responseErr) {
		return false
	}
	return responseErr.statusCode == http.StatusNotFound ||
		responseErr.statusCode == http.StatusGone
}

var uppercaseRegex = regexp.MustCompile(`[A-Z]`)

func escapePath(path string) string {
	// Escape uppercase characters by converting them to lowercase and prefixing with '!' as per GOPROXY spec.
	path = uppercaseRegex.ReplaceAllStringFunc(path, func(s string) string {
		return "!" + strings.ToLower(s)
	})
	return url.PathEscape(path)
}
