package implsentry

import (
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/modules/errortracking/implerrortracking"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// extractKey pulls the "<v>:<hmac>" credential out of a minted DSN.
func extractKey(t *testing.T, dsn string) string {
	t.Helper()
	at := strings.Index(dsn, "@")
	require.Greater(t, at, len("https://"))
	return dsn[len("https://"):at]
}

func TestMintDSN_ShapeAndVerify(t *testing.T) {
	secret := []byte("platform-ingest-secret")
	proj := valuer.GenerateUUID()

	dsn := mintDSN(secret, "api.hanzo.ai", proj, 1)
	// CLEAN path — no /api/ segment.
	assert.True(t, strings.HasPrefix(dsn, "https://"))
	assert.Contains(t, dsn, "@api.hanzo.ai/v1/sentry/"+proj.String())
	assert.NotContains(t, dsn, "/api/")

	// The embedded key verifies for THIS project at v1.
	key := extractKey(t, dsn)
	assert.True(t, implerrortracking.VerifyKey(secret, proj.String(), key, 1))
}

func TestMintDSN_ProjectDomainSeparation(t *testing.T) {
	secret := []byte("platform-ingest-secret")
	a, b := valuer.GenerateUUID(), valuer.GenerateUUID()

	keyA := extractKey(t, mintDSN(secret, "h", a, 1))
	// A's key must NOT verify for B — the project id is part of the HMAC domain.
	assert.False(t, implerrortracking.VerifyKey(secret, b.String(), keyA, 1))
	assert.True(t, implerrortracking.VerifyKey(secret, a.String(), keyA, 1))
}

func TestMintDSN_RotationWatermark(t *testing.T) {
	secret := []byte("s")
	proj := valuer.GenerateUUID()

	v1 := extractKey(t, mintDSN(secret, "h", proj, 1))
	v2 := extractKey(t, mintDSN(secret, "h", proj, 2))

	// After rotation to v2, a v1 key no longer verifies (minVersion=2); v2 does.
	assert.False(t, implerrortracking.VerifyKey(secret, proj.String(), v1, 2))
	assert.True(t, implerrortracking.VerifyKey(secret, proj.String(), v2, 2))
	// A wrong secret never verifies.
	assert.False(t, implerrortracking.VerifyKey([]byte("other"), proj.String(), v2, 1))
}
