package datastoreprometheus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dsmock "github.com/hanzo-ds/mock"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/telemetrymetrics"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/telemetrystoretest"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/ctxtypes"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeriesCache_HitAndNoDuplicateLoad(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cache := newSeriesCacheWithClock(16, 60*time.Second, func() time.Time { return now })

	want := map[uint64][]prompb.Label{
		42: {{Name: "__name__", Value: "k8s.node.filesystem.usage"}, {Name: "host", Value: "node-1"}},
	}

	var calls atomic.Int32
	loader := func(context.Context) (map[uint64][]prompb.Label, error) {
		calls.Add(1)
		return want, nil
	}

	got1, err := cache.getOrLoad(context.Background(), "key-1", loader)
	require.NoError(t, err)
	assert.Equal(t, want, got1)
	assert.Equal(t, int32(1), calls.Load())

	// Second lookup within TTL must hit memory without calling loader again.
	now = now.Add(30 * time.Second)
	got2, err := cache.getOrLoad(context.Background(), "key-1", loader)
	require.NoError(t, err)
	assert.Equal(t, want, got2)
	assert.Equal(t, int32(1), calls.Load())
}

func TestSeriesCache_TTLExpiration(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cache := newSeriesCacheWithClock(16, 60*time.Second, func() time.Time { return now })

	var calls atomic.Int32
	loader := func(context.Context) (map[uint64][]prompb.Label, error) {
		n := calls.Add(1)
		return map[uint64][]prompb.Label{
			uint64(n): {{Name: "gen", Value: string(rune('0' + n))}},
		}, nil
	}

	got1, err := cache.getOrLoad(context.Background(), "k", loader)
	require.NoError(t, err)
	require.Contains(t, got1, uint64(1))
	assert.Equal(t, int32(1), calls.Load())

	// Still fresh at 59s.
	now = now.Add(59 * time.Second)
	got2, err := cache.getOrLoad(context.Background(), "k", loader)
	require.NoError(t, err)
	require.Contains(t, got2, uint64(1))
	assert.Equal(t, int32(1), calls.Load())

	// Expired at 60s -> reloads fresh data.
	now = now.Add(1 * time.Second)
	got3, err := cache.getOrLoad(context.Background(), "k", loader)
	require.NoError(t, err)
	require.Contains(t, got3, uint64(2))
	assert.Equal(t, int32(2), calls.Load())
}

func TestSeriesCache_BoundedLRUEviction(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cache := newSeriesCacheWithClock(3, 60*time.Second, func() time.Time { return now })

	makeVal := func(fp uint64) map[uint64][]prompb.Label {
		return map[uint64][]prompb.Label{fp: {{Name: "k", Value: "v"}}}
	}

	cache.set("k1", makeVal(1))
	cache.set("k2", makeVal(2))
	cache.set("k3", makeVal(3))
	assert.Equal(t, 3, cache.len())

	// Touch k1 so k2 becomes the least-recently-used entry.
	_, ok := cache.get("k1")
	require.True(t, ok)

	// Inserting k4 must evict k2 while keeping size bounded at 3.
	cache.set("k4", makeVal(4))
	assert.Equal(t, 3, cache.len())

	_, ok1 := cache.get("k1")
	_, ok2 := cache.get("k2")
	_, ok3 := cache.get("k3")
	_, ok4 := cache.get("k4")
	assert.True(t, ok1, "k1 was recently touched and must remain")
	assert.False(t, ok2, "k2 was LRU and must be evicted")
	assert.True(t, ok3, "k3 must remain")
	assert.True(t, ok4, "k4 must be present")
}

func TestSeriesCache_SingleflightCoalescing(t *testing.T) {
	cache := newSeriesCache(32, 60*time.Second)

	const callers = 20
	started := make(chan struct{})
	release := make(chan struct{})
	var loads atomic.Int32

	want := map[uint64][]prompb.Label{
		99: {{Name: "__name__", Value: "hanzo_service_up"}},
	}

	loader := func(context.Context) (map[uint64][]prompb.Label, error) {
		if loads.Add(1) == 1 {
			close(started)
		}
		<-release
		return want, nil
	}

	var wg sync.WaitGroup
	wg.Add(callers)
	results := make([]map[uint64][]prompb.Label, callers)
	errs := make([]error, callers)

	for i := 0; i < callers; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = cache.getOrLoad(context.Background(), "shared-key", loader)
		}(i)
	}

	<-started
	// Give sibling goroutines a brief moment to join the in-flight call.
	time.Sleep(15 * time.Millisecond)
	close(release)
	wg.Wait()

	assert.Equal(t, int32(1), loads.Load(), "concurrent identical misses must coalesce into one datastore query")
	for i := 0; i < callers; i++ {
		require.NoError(t, errs[i])
		assert.Equal(t, want, results[i])
	}
}

func TestSeriesCache_SingleflightCallerCancelDoesNotFailSiblings(t *testing.T) {
	cache := newSeriesCache(32, 60*time.Second)

	started := make(chan struct{})
	release := make(chan struct{})
	want := map[uint64][]prompb.Label{
		7: {{Name: "__name__", Value: "system.filesystem.usage"}},
	}

	loader := func(loadCtx context.Context) (map[uint64][]prompb.Label, error) {
		close(started)
		<-release
		// Load context must stay alive even though caller 1 canceled.
		if err := loadCtx.Err(); err != nil {
			return nil, err
		}
		return want, nil
	}

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	errCh1 := make(chan error, 1)
	go func() {
		_, err := cache.getOrLoad(ctx1, "k-cancel", loader)
		errCh1 <- err
	}()

	<-started

	type res2 struct {
		fps map[uint64][]prompb.Label
		err error
	}
	resCh2 := make(chan res2, 1)
	go func() {
		fps, err := cache.getOrLoad(context.Background(), "k-cancel", loader)
		resCh2 <- res2{fps: fps, err: err}
	}()

	time.Sleep(10 * time.Millisecond)
	cancel1()
	assert.ErrorIs(t, <-errCh1, context.Canceled)

	close(release)
	r2 := <-resCh2
	require.NoError(t, r2.err)
	assert.Equal(t, want, r2.fps)
}

func TestSeriesCache_ErrorsNotCached(t *testing.T) {
	cache := newSeriesCache(16, 60*time.Second)

	var calls atomic.Int32
	boom := errors.New("clickhouse transient error")
	want := map[uint64][]prompb.Label{1: {{Name: "a", Value: "b"}}}

	_, err := cache.getOrLoad(context.Background(), "k-err", func(context.Context) (map[uint64][]prompb.Label, error) {
		calls.Add(1)
		return nil, boom
	})
	require.ErrorIs(t, err, boom)
	assert.Equal(t, 0, cache.len())

	got, err := cache.getOrLoad(context.Background(), "k-err", func(context.Context) (map[uint64][]prompb.Label, error) {
		calls.Add(1)
		return want, nil
	})
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, int32(2), calls.Load())
}

func TestSeriesCache_TenantIsolation(t *testing.T) {
	hour := time.Hour.Milliseconds()
	start := int64(500) * hour
	end := start + 30*time.Minute.Milliseconds()
	matchers := []*prompb.LabelMatcher{
		{Type: prompb.LabelMatcher_EQ, Name: "__name__", Value: "hanzo_service_up"},
	}

	keyFor := func(ctx context.Context) string {
		return buildSeriesCacheKey(ctx, "hanzo_service_up", telemetrymetrics.SeriesTableName, start, end, false, matchers)
	}

	ctxNone := context.Background()
	ctxTenantA := authtypes.NewContextWithTenant(ctxNone, "org-a")
	ctxTenantB := authtypes.NewContextWithTenant(ctxNone, "org-b")
	ctxClaimsA := authtypes.NewContextWithClaims(ctxNone, authtypes.Claims{OrgID: "org-1"})
	ctxClaimsB := authtypes.NewContextWithClaims(ctxNone, authtypes.Claims{OrgID: "org-2"})
	ctxRuleOrgA := ctxtypes.NewContextWithCommentVals(ctxNone, map[string]string{"org_id": "rule-org-1"})
	ctxRuleOrgB := ctxtypes.NewContextWithCommentVals(ctxNone, map[string]string{"org_id": "rule-org-2"})

	// Delimiter-injection check: crafted tenant string must not collide with separate tenant+claims fields.
	ctxCrafted := authtypes.NewContextWithTenant(ctxNone, "a|c=1:b")
	ctxSplit := authtypes.NewContextWithClaims(authtypes.NewContextWithTenant(ctxNone, "a"), authtypes.Claims{OrgID: "b"})

	keys := map[string]string{
		"none":      keyFor(ctxNone),
		"tenantA":   keyFor(ctxTenantA),
		"tenantB":   keyFor(ctxTenantB),
		"claimsA":   keyFor(ctxClaimsA),
		"claimsB":   keyFor(ctxClaimsB),
		"ruleOrgA":  keyFor(ctxRuleOrgA),
		"ruleOrgB":  keyFor(ctxRuleOrgB),
		"crafted":   keyFor(ctxCrafted),
		"split":     keyFor(ctxSplit),
	}

	seen := make(map[string]string, len(keys))
	for name, k := range keys {
		if prev, dup := seen[k]; dup {
			t.Fatalf("tenant cache key collision between %q and %q: %s", prev, name, k)
		}
		seen[k] = name
	}
}

func TestBuildSeriesCacheKey_MatcherOrderAndWindowBuckets(t *testing.T) {
	ctx := authtypes.NewContextWithTenant(context.Background(), "hanzo")
	hour := time.Hour.Milliseconds()
	start := int64(500) * hour

	m1 := []*prompb.LabelMatcher{
		{Type: prompb.LabelMatcher_EQ, Name: "host", Value: "n1"},
		{Type: prompb.LabelMatcher_RE, Name: "mountpoint", Value: "/|/data"},
	}
	m2 := []*prompb.LabelMatcher{
		{Type: prompb.LabelMatcher_RE, Name: "mountpoint", Value: "/|/data"},
		{Type: prompb.LabelMatcher_EQ, Name: "host", Value: "n1"},
	}
	m3 := []*prompb.LabelMatcher{
		{Type: prompb.LabelMatcher_EQ, Name: "host", Value: "n2"},
		{Type: prompb.LabelMatcher_RE, Name: "mountpoint", Value: "/|/data"},
	}

	// Reordered matchers + 30s-apart evals within the same hour must share a key.
	k1 := buildSeriesCacheKey(ctx, "system.filesystem.usage", telemetrymetrics.SeriesTableName, start, start+10*time.Minute.Milliseconds(), false, m1)
	k2 := buildSeriesCacheKey(ctx, "system.filesystem.usage", telemetrymetrics.SeriesTableName, start, start+10*time.Minute.Milliseconds()+30*time.Second.Milliseconds(), false, m2)
	assert.Equal(t, k1, k2)

	// Different matcher value must produce a distinct key.
	k3 := buildSeriesCacheKey(ctx, "system.filesystem.usage", telemetrymetrics.SeriesTableName, start, start+10*time.Minute.Milliseconds(), false, m3)
	assert.NotEqual(t, k1, k3)

	// Crossing an hour boundary must roll over to a new cache key.
	kBeforeRollover := buildSeriesCacheKey(ctx, "system.filesystem.usage", telemetrymetrics.SeriesTableName, start, start+hour-30*time.Second.Milliseconds(), false, m1)
	kAfterRollover := buildSeriesCacheKey(ctx, "system.filesystem.usage", telemetrymetrics.SeriesTableName, start, start+hour+30*time.Second.Milliseconds(), false, m1)
	assert.Equal(t, k1, kBeforeRollover)
	assert.NotEqual(t, kBeforeRollover, kAfterRollover)

	// Different table or normalized flag must produce distinct keys.
	k6h := buildSeriesCacheKey(ctx, "system.filesystem.usage", telemetrymetrics.Series6hTableName, start, start+10*time.Minute.Milliseconds(), false, m1)
	kNorm := buildSeriesCacheKey(ctx, "system.filesystem.usage", telemetrymetrics.SeriesTableName, start, start+10*time.Minute.Milliseconds(), true, m1)
	assert.NotEqual(t, k1, k6h)
	assert.NotEqual(t, k1, kNorm)
}

func TestClient_Read_SeriesCacheEndToEnd(t *testing.T) {
	store := telemetrystoretest.New(telemetrystore.Config{Provider: "datastore"}, sqlmock.QueryMatcherRegexp)
	settings := factory.NewScopedProviderSettings(instrumentationtest.New().ToProviderSettings(), "test")
	rc := NewReadClient(settings, store)

	seriesCols := []dsmock.ColumnType{
		{Name: "fingerprint", Type: "UInt64"},
		{Name: "any(labels)", Type: "String"},
	}
	sampleCols := []dsmock.ColumnType{
		{Name: "metric_name", Type: "String"},
		{Name: "fingerprint", Type: "UInt64"},
		{Name: "unix_milli", Type: "Int64"},
		{Name: "value", Type: "Float64"},
		{Name: "flags", Type: "UInt32"},
	}

	hour := time.Hour.Milliseconds()
	t1Start := int64(500)*hour + 10*time.Minute.Milliseconds()
	t1End := t1Start + 5*time.Minute.Milliseconds()
	t2Start := t1Start + 30*time.Second.Milliseconds()
	t2End := t1End + 30*time.Second.Milliseconds()

	mock := store.Mock()

	// Org A, eval 1: cache miss -> 1 series query + 1 samples query.
	mock.ExpectQuery("SELECT fingerprint, any\\(labels\\)").
		WithArgs("k8s.node.filesystem.usage", "__name__", "k8s.node.filesystem.usage").
		WillReturnRows(dsmock.NewRows(seriesCols, [][]any{
			{uint64(101), `{"__name__":"k8s.node.filesystem.usage","tenant":"org-a"}`},
		}))
	mock.ExpectQuery("SELECT metric_name, fingerprint, unix_milli, value, flags").
		WithArgs("k8s.node.filesystem.usage", "k8s.node.filesystem.usage", "__name__", "k8s.node.filesystem.usage", t1Start, t1End).
		WillReturnRows(dsmock.NewRows(sampleCols, [][]any{
			{"k8s.node.filesystem.usage", uint64(101), t1End - 1000, 75.5, uint32(0)},
		}))

	// Org A, eval 2 (30s later): SERIES CACHE HIT -> ONLY samples query runs!
	mock.ExpectQuery("SELECT metric_name, fingerprint, unix_milli, value, flags").
		WithArgs("k8s.node.filesystem.usage", "k8s.node.filesystem.usage", "__name__", "k8s.node.filesystem.usage", t2Start, t2End).
		WillReturnRows(dsmock.NewRows(sampleCols, [][]any{
			{"k8s.node.filesystem.usage", uint64(101), t2End - 1000, 76.0, uint32(0)},
		}))

	// Org B, same window and metric: MUST NOT reuse Org A's cache entry -> fresh series + samples queries.
	mock.ExpectQuery("SELECT fingerprint, any\\(labels\\)").
		WithArgs("k8s.node.filesystem.usage", "__name__", "k8s.node.filesystem.usage").
		WillReturnRows(dsmock.NewRows(seriesCols, [][]any{
			{uint64(202), `{"__name__":"k8s.node.filesystem.usage","tenant":"org-b"}`},
		}))
	mock.ExpectQuery("SELECT metric_name, fingerprint, unix_milli, value, flags").
		WithArgs("k8s.node.filesystem.usage", "k8s.node.filesystem.usage", "__name__", "k8s.node.filesystem.usage", t2Start, t2End).
		WillReturnRows(dsmock.NewRows(sampleCols, [][]any{
			{"k8s.node.filesystem.usage", uint64(202), t2End - 1000, 42.0, uint32(0)},
		}))

	matchers := []*prompb.LabelMatcher{
		{Type: prompb.LabelMatcher_EQ, Name: "__name__", Value: "k8s.node.filesystem.usage"},
	}

	ctxOrgA := authtypes.NewContextWithTenant(context.Background(), "org-a")
	ctxOrgB := authtypes.NewContextWithTenant(context.Background(), "org-b")

	ss1, err := rc.Read(ctxOrgA, &prompb.Query{StartTimestampMs: t1Start, EndTimestampMs: t1End, Matchers: matchers}, true)
	require.NoError(t, err)
	require.True(t, ss1.Next())
	assert.Equal(t, "org-a", ss1.At().Labels().Get("tenant"))

	ss2, err := rc.Read(ctxOrgA, &prompb.Query{StartTimestampMs: t2Start, EndTimestampMs: t2End, Matchers: matchers}, true)
	require.NoError(t, err)
	require.True(t, ss2.Next())
	assert.Equal(t, "org-a", ss2.At().Labels().Get("tenant"))

	ss3, err := rc.Read(ctxOrgB, &prompb.Query{StartTimestampMs: t2Start, EndTimestampMs: t2End, Matchers: matchers}, true)
	require.NoError(t, err)
	require.True(t, ss3.Next())
	assert.Equal(t, "org-b", ss3.At().Labels().Get("tenant"))

	require.NoError(t, mock.ExpectationsWereMet())
}
