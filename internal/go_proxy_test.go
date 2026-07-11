package internal

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoProxyClient_GetLatestInfo_FallbackToVersionList(t *testing.T) {
	t.Parallel()

	client := newTestGoProxyClient(t, func(r *http.Request) (int, string) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/@latest"):
			return http.StatusNotFound, invalidGoSQLMockVersion
		case strings.HasSuffix(r.URL.Path, "/@v/list"):
			return http.StatusOK, "v1.3.0\nv1.5.2\n"
		case strings.HasSuffix(r.URL.Path, "/@v/v1.5.2.info"):
			return http.StatusNotFound, invalidGoSQLMockVersion
		case strings.HasSuffix(r.URL.Path, "/@v/v1.3.0.info"):
			return http.StatusOK, `{"Version":"v1.3.0","Time":"2017-09-01T07:34:10Z"}`
		default:
			return http.StatusNotFound, "not found"
		}
	})

	module, err := client.GetLatestInfo("gopkg.in/DATA-DOG/go-sqlmock.v1")

	require.NoError(t, err)
	assert.Equal(t, "gopkg.in/DATA-DOG/go-sqlmock.v1", module.Path)
	assert.Equal(t, semver.MustParse("v1.3.0"), module.Version)
}

func TestGoProxyClient_GetLatestInfo_DoesNotFallbackForServerError(t *testing.T) {
	t.Parallel()

	client := newTestGoProxyClient(t, func(_ *http.Request) (int, string) {
		return http.StatusInternalServerError, "server error\n"
	})

	_, err := client.GetLatestInfo("gopkg.in/DATA-DOG/go-sqlmock.v1")

	require.EqualError(
		t,
		err,
		"unexpected response status code from GET "+
			"https://proxy.test/gopkg.in%2F%21d%21a%21t%21a-%21d%21o%21g%2Fgo-sqlmock.v1/@latest: "+
			"500, body: server error\n",
	)
}

const invalidGoSQLMockVersion = `not found: gopkg.in/DATA-DOG/go-sqlmock.v1@v1.5.2: ` +
	`invalid version: go.mod has non-....v1 module path ` +
	`"github.com/DATA-DOG/go-sqlmock" at revision v1.5.2`

func newTestGoProxyClient(
	t *testing.T,
	handler func(*http.Request) (int, string),
) GoProxyClient {
	t.Helper()

	u, err := url.Parse("https://proxy.test")
	require.NoError(t, err)
	return GoProxyClient{
		http: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				statusCode, body := handler(r)
				return &http.Response{
					StatusCode: statusCode,
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    r,
				}, nil
			}),
		},
		apiURL: *u,
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
