package internal

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDepsDevClient_GetVersions(t *testing.T) {
	t.Run("filter out non canonical versions", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`
{
  "versions": [
    {"versionKey": {"version": "v0.0.0-20201216005158-039620a65673"}},
    {"versionKey": {"version": "v1"}},
    {"versionKey": {"version": "v1.0.0"}},
    {"versionKey": {"version": "v1.2"}},
    {"versionKey": {"version": "v1.2.0"}}
  ]
}
`))
		}))

		u, err := url.Parse(srv.URL)
		require.NoError(t, err)
		client := DepsDevClient{
			http:   new(http.Client),
			apiURL: *u,
		}

		versions, err := client.GetVersions("test")
		require.NoError(t, err)
		assert.ElementsMatch(t,
			[]*semver.Version{
				semver.MustParse("v0.0.0-20201216005158-039620a65673"),
				semver.MustParse("v1.0.0"),
				semver.MustParse("v1.2.0"),
			},
			versions)
	})
}
