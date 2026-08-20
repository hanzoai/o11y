package routerweb

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/factory/factorytest"
	"github.com/hanzoai/o11y/pkg/global"
	"github.com/hanzoai/o11y/pkg/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func expectedHTML(baseHref string, settings web.Settings) string {
	settingsJSON, _ := json.Marshal(settings)
	return `<html><head><base href="` + baseHref + `" /></head><body><script>window.o11yBootData={settings:` + string(settingsJSON) + `}</script>Welcome to test data!!!</body></html>`
}

// startServer serves the console EXACTLY as a host mounts it — as the terminal
// http.Handler, with no router in front of it. There is nothing else to wire:
// the provider is an http.Handler, so the test drives the shipped value rather
// than a stand-in for it.
func startServer(t *testing.T, config web.Config, globalConfig global.Config) string {
	t.Helper()

	provider, err := New(context.Background(), factorytest.NewSettings(), config, globalConfig)
	require.NoError(t, err)

	server := httptest.NewServer(provider)
	t.Cleanup(server.Close)

	return server.URL
}

func httpGet(t *testing.T, url string) string {
	t.Helper()

	return body(t, get(t, url))
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()

	res, err := http.DefaultClient.Get(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = res.Body.Close() })

	return res
}

func body(t *testing.T, res *http.Response) string {
	t.Helper()

	raw, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	return string(raw)
}

// spaFixture writes a minimal built-SPA tree — a templated shell and one
// content-hashed bundle beside it — so the console can be exercised without a
// frontend build.
func spaFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"),
		[]byte(`<html><head><base href="[[.BaseHref]]" /></head><body>console</body></html>`), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "assets"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "index-a1b2c3d4.js"),
		[]byte(`console.log("o11y")`), 0o600))

	return dir
}

// TestServeSPA is the console's contract, on a fixture rather than a build: the
// shell at the root, a hashed bundle as itself, any client-side route as the
// shell, and the API planes NOT swallowed by the fallback. The last one is the
// one that fails silently — a catch-all that answers /v1/… with index.html hands
// JSON clients 200 text/html.
func TestServeSPA(t *testing.T) {
	t.Parallel()

	base := startServer(t, web.Config{Index: "index.html", Directory: spaFixture(t)}, global.Config{})
	shell := `<html><head><base href="/" /></head><body>console</body></html>`

	t.Run("Root", func(t *testing.T) {
		res := get(t, base+"/")

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, "text/html; charset=utf-8", res.Header.Get("Content-Type"))
		assert.Equal(t, shell, body(t, res))
	})

	t.Run("HashedAsset", func(t *testing.T) {
		res := get(t, base+"/assets/index-a1b2c3d4.js")

		assert.Equal(t, http.StatusOK, res.StatusCode)
		contentType := res.Header.Get("Content-Type")
		assert.Contains(t, contentType, "javascript")
		assert.NotContains(t, contentType, "text/html")
		assert.Equal(t, `console.log("o11y")`, body(t, res))
	})

	t.Run("ClientSideRoute", func(t *testing.T) {
		res := get(t, base+"/services/frontend/overview")

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, "text/html; charset=utf-8", res.Header.Get("Content-Type"))
		assert.Equal(t, shell, body(t, res))
	})

	// Every API root, and both the bare prefix and a path under it. A matched API
	// route never reaches the console; an unmatched one must be a real 404 with
	// no HTML in it, on the SAME handler that answers the shell for /services.
	t.Run("APIPlaneNotSwallowed", func(t *testing.T) {
		for _, path := range []string{
			"/v1",
			"/v1/o11y/version",
			"/v1/o11y/pods/bogus",
			"/v1/sentinel/1/envelope/",
			"/ws",
			"/ws/query_progress",
		} {
			res := get(t, base+path)

			assert.Equal(t, http.StatusNotFound, res.StatusCode, path)
			assert.NotContains(t, res.Header.Get("Content-Type"), "text/html", path)
			assert.NotContains(t, body(t, res), "<html", path)
		}
	})

	// The boundary is the path SEGMENT, not the substring: a console route that
	// merely starts with the same letters stays the console's.
	t.Run("APIPrefixIsASegment", func(t *testing.T) {
		res := get(t, base+"/v1beta-console-route")

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, shell, body(t, res))
	})
}

func TestServeTemplatedIndex(t *testing.T) {
	t.Parallel()

	emptySettings := web.Settings{}

	testCases := []struct {
		name         string
		path         string
		globalConfig global.Config
		webConfig    web.Config
		expected     string
	}{
		{
			name:         "RootBaseHrefAtRoot",
			path:         "/",
			globalConfig: global.Config{},
			webConfig:    web.Config{Index: "valid_template.html", Directory: "testdata"},
			expected:     expectedHTML("/", emptySettings),
		},
		{
			name:         "RootBaseHrefAtNonExistentPath",
			path:         "/does-not-exist",
			globalConfig: global.Config{},
			webConfig:    web.Config{Index: "valid_template.html", Directory: "testdata"},
			expected:     expectedHTML("/", emptySettings),
		},
		{
			name:         "RootBaseHrefAtDirectory",
			path:         "/assets",
			globalConfig: global.Config{},
			webConfig:    web.Config{Index: "valid_template.html", Directory: "testdata"},
			expected:     expectedHTML("/", emptySettings),
		},
		{
			name:         "SubPathBaseHrefAtRoot",
			path:         "/",
			globalConfig: global.Config{ExternalURL: &url.URL{Scheme: "https", Host: "example.com", Path: "/o11y"}},
			webConfig:    web.Config{Index: "valid_template.html", Directory: "testdata"},
			expected:     expectedHTML("/o11y/", emptySettings),
		},
		{
			name:         "SubPathBaseHrefAtNonExistentPath",
			path:         "/does-not-exist",
			globalConfig: global.Config{ExternalURL: &url.URL{Scheme: "https", Host: "example.com", Path: "/o11y"}},
			webConfig:    web.Config{Index: "valid_template.html", Directory: "testdata"},
			expected:     expectedHTML("/o11y/", emptySettings),
		},
		{
			name:         "SubPathBaseHrefAtDirectory",
			path:         "/assets",
			globalConfig: global.Config{ExternalURL: &url.URL{Scheme: "https", Host: "example.com", Path: "/o11y"}},
			webConfig:    web.Config{Index: "valid_template.html", Directory: "testdata"},
			expected:     expectedHTML("/o11y/", emptySettings),
		},
		{
			name:         "WithPopulatedSettings",
			path:         "/",
			globalConfig: global.Config{},
			webConfig: web.Config{
				Index:     "valid_template.html",
				Directory: "testdata",
				Settings: web.SettingsConfig{
					Sentry: web.SentryConfig{
						Enabled: true,
						DSN:     "https://examplePublicKey@o0.ingest.sentry.io/0",
						Tunnel:  "https://example.com/tunnel",
					},
				},
			},
			expected: expectedHTML("/", web.Settings{
				Sentry: web.Sentry{
					Enabled: true,
					DSN:     "https://examplePublicKey@o0.ingest.sentry.io/0",
					Tunnel:  "https://example.com/tunnel",
				},
			}),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			base := startServer(t, testCase.webConfig, testCase.globalConfig)

			assert.Equal(t, testCase.expected, strings.TrimSuffix(httpGet(t, base+testCase.path), "\n"))
		})
	}
}

func TestServeNoTemplateIndex(t *testing.T) {
	t.Parallel()

	expected, err := os.ReadFile(filepath.Join("testdata", "no_template.html"))
	require.NoError(t, err)

	testCases := []struct {
		name string
		path string
	}{
		{
			name: "Root",
			path: "/",
		},
		{
			name: "NonExistentPath",
			path: "/does-not-exist",
		},
		{
			name: "Directory",
			path: "/assets",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			base := startServer(t, web.Config{Index: "no_template.html", Directory: "testdata"}, global.Config{})

			assert.Equal(t, string(expected), httpGet(t, base+testCase.path))
		})
	}
}

func TestServeInvalidTemplateIndex(t *testing.T) {
	t.Parallel()

	expected, err := os.ReadFile(filepath.Join("testdata", "invalid_template.html"))
	require.NoError(t, err)

	testCases := []struct {
		name string
		path string
	}{
		{
			name: "Root",
			path: "/",
		},
		{
			name: "NonExistentPath",
			path: "/does-not-exist",
		},
		{
			name: "Directory",
			path: "/assets",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			base := startServer(t, web.Config{Index: "invalid_template.html", Directory: "testdata"}, global.Config{ExternalURL: &url.URL{Path: "/o11y"}})

			assert.Equal(t, string(expected), httpGet(t, base+testCase.path))
		})
	}
}

func TestServeStaticFilesUnchanged(t *testing.T) {
	t.Parallel()

	expected, err := os.ReadFile(filepath.Join("testdata", "assets", "style.css"))
	require.NoError(t, err)

	base := startServer(t, web.Config{Index: "valid_template.html", Directory: "testdata"}, global.Config{ExternalURL: &url.URL{Path: "/o11y"}})

	assert.Equal(t, string(expected), httpGet(t, base+"/assets/style.css"))
}
