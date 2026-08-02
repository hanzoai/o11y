package noopweb

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y/pkg/factory/factorytest"
	"github.com/hanzoai/o11y/pkg/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The null console is MOUNTED like any other now, so "no console here" has to be
// a status. These are the bytes gorilla's default NotFoundHandler wrote when the
// null provider registered nothing at all — a headless deployment
// (web.enabled=false, the shipped image) answers exactly what it always did, and
// never a 200 of no bytes.
func TestNoopServesNotFound(t *testing.T) {
	t.Parallel()

	provider, err := New(context.Background(), factorytest.NewSettings(), web.Config{})
	require.NoError(t, err)

	server := httptest.NewServer(provider)
	t.Cleanup(server.Close)

	for _, path := range []string{"/", "/services", "/v1/o11y/version"} {
		res, err := http.DefaultClient.Get(server.URL + path)
		require.NoError(t, err)
		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.NoError(t, res.Body.Close())

		assert.Equal(t, http.StatusNotFound, res.StatusCode, path)
		assert.Equal(t, "404 page not found\n", string(body), path)
	}
}
