package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"golang.org/x/mod/module"
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
			return nil, errors.Wrap(err, "failed to parse $GOPROXY url")
		}
		apiURL = *u
	}
	return &GoProxyClient{
		http:      &http.Client{Timeout: 10 * time.Second},
		apiURL:    apiURL,
		cache:     cache,
		goprivate: os.Getenv("GOPRIVATE"),
	}, nil
}

// GoProxyClient is used to interact with Golang proxy server.
// Details on GOPROXY protocol can be found here: https://go.dev/ref/mod#goproxy-protocol.
type GoProxyClient struct {
	http      *http.Client
	apiURL    url.URL
	cache     modulesCache
	goprivate string
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
	return c.getInfo(path, nil, true)
}

var githubRegexp = regexp.MustCompile(`^(?P<root>github\.com/[\w.\-]+/[\w.\-]+)(/[\w.\-]+)*$`)

func (c *GoProxyClient) getInfo(path string, version *semver.Version, latest bool) (*Module, error) {
	if c.isPrivate(path) {
		m := githubRegexp.FindStringSubmatch(path)
		if m == nil {
			return nil, errors.Errorf(
				"unsupported private module path: %s, private modules must match '%s' regexp",
				path, githubRegexp)
		}

		var root string
		for i, name := range githubRegexp.SubexpNames() {
			if name == "root" {
				root = m[i]
			}
		}
		repoURL := "https://" + root + ".git"
		dst := filepath.Join("/home/mh/lol", root)
		err := execGitCmd("clone", "--", repoURL, dst)
		if err != nil {
			panic(err)
		}
		panic("ye")
	}
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

func (c *GoProxyClient) GetModFile(path string, version *semver.Version) ([]byte, error) {
	urlPath := fmt.Sprintf(getModFileFmt, escapePath(path), version)
	return c.query(urlPath)
}

func (c *GoProxyClient) query(urlPath string) ([]byte, error) {
	resp, err := c.http.Get(c.apiURL.JoinPath(urlPath).String())
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf(
			"unexpected response status code from %s %s: %d, body: %s",
			http.MethodGet, resp.Request.URL.String(), resp.StatusCode, string(data))
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}

func (c GoProxyClient) isPrivate(path string) bool {
	return module.MatchPrefixPatterns(c.goprivate, path)
}

var uppercaseRegex = regexp.MustCompile(`[A-Z]`)

func escapePath(path string) string {
	// Escape uppercase characters by converting them to lowercase and prefixing with '!' as per GOPROXY spec.
	path = uppercaseRegex.ReplaceAllStringFunc(path, func(s string) string {
		return "!" + strings.ToLower(s)
	})
	return url.PathEscape(path)
}

func execGitCmd(args ...string) error {
	// #nosec G204
	cmd := exec.Command("git", args...)
	if cmd.Stderr != nil {
		return errors.New("exec: Stderr already set")
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return errors.Errorf("Failed to execute '%s' command: %s", cmd, stderr.String())
	}
	return nil
}
