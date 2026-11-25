package datastoreprometheus

import (
	"container/list"
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/o11y/pkg/telemetrymetrics"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/ctxtypes"
	"github.com/prometheus/prometheus/prompb"
	"golang.org/x/sync/singleflight"
)

const (
	// defaultSeriesCacheMaxEntries bounds memory usage for cached
	// event.series fingerprint -> labels maps.
	defaultSeriesCacheMaxEntries = 512

	// defaultSeriesCacheTTL keeps repeated 30-60s alert evaluations in memory
	// while ensuring newly registered series appear within one minute.
	defaultSeriesCacheTTL = 60 * time.Second

	// defaultSeriesLoadTimeout bounds detached singleflight ClickHouse lookups
	// so a single canceled caller does not abort coalesced siblings.
	defaultSeriesLoadTimeout = 30 * time.Second
)

type seriesCacheEntry struct {
	key          string
	fingerprints map[uint64][]prompb.Label
	expiresAt    time.Time
}

// seriesCache is a thread-safe, bounded-size LRU+TTL cache with singleflight
// coalescing for event.series fingerprint -> labels lookups.
//
// Returned maps and their label slices are shared read-only across callers
// within the same tenant scope and must not be mutated.
type seriesCache struct {
	mu         sync.Mutex
	entries    map[string]*list.Element
	order      *list.List // front = MRU, back = LRU
	sf         singleflight.Group
	maxEntries int
	ttl        time.Duration
	now        func() time.Time
}

func newSeriesCache(maxEntries int, ttl time.Duration) *seriesCache {
	return newSeriesCacheWithClock(maxEntries, ttl, time.Now)
}

func newSeriesCacheWithClock(maxEntries int, ttl time.Duration, now func() time.Time) *seriesCache {
	if now == nil {
		now = time.Now
	}
	return &seriesCache{
		entries:    make(map[string]*list.Element),
		order:      list.New(),
		maxEntries: maxEntries,
		ttl:        ttl,
		now:        now,
	}
}

func (c *seriesCache) get(key string) (map[uint64][]prompb.Label, bool) {
	if c == nil || c.maxEntries <= 0 || c.ttl <= 0 {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	entry := elem.Value.(*seriesCacheEntry)
	if !c.now().Before(entry.expiresAt) {
		c.order.Remove(elem)
		delete(c.entries, key)
		return nil, false
	}

	c.order.MoveToFront(elem)
	return entry.fingerprints, true
}

func (c *seriesCache) set(key string, fingerprints map[uint64][]prompb.Label) {
	if c == nil || c.maxEntries <= 0 || c.ttl <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	expiresAt := c.now().Add(c.ttl)
	if elem, ok := c.entries[key]; ok {
		entry := elem.Value.(*seriesCacheEntry)
		entry.fingerprints = fingerprints
		entry.expiresAt = expiresAt
		c.order.MoveToFront(elem)
		return
	}

	for c.order.Len() >= c.maxEntries {
		back := c.order.Back()
		if back == nil {
			break
		}
		evicted := back.Value.(*seriesCacheEntry)
		c.order.Remove(back)
		delete(c.entries, evicted.key)
	}

	elem := c.order.PushFront(&seriesCacheEntry{
		key:          key,
		fingerprints: fingerprints,
		expiresAt:    expiresAt,
	})
	c.entries[key] = elem
}

func (c *seriesCache) len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

func (c *seriesCache) getOrLoad(
	ctx context.Context,
	key string,
	load func(context.Context) (map[uint64][]prompb.Label, error),
) (map[uint64][]prompb.Label, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if fps, ok := c.get(key); ok {
		return fps, nil
	}

	ch := c.sf.DoChan(key, func() (any, error) {
		if fps, ok := c.get(key); ok {
			return fps, nil
		}

		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultSeriesLoadTimeout)
		defer cancel()

		fps, err := load(loadCtx)
		if err != nil {
			return nil, err
		}
		c.set(key, fps)
		return fps, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}
		return res.Val.(map[uint64][]prompb.Label), nil
	}
}

// buildSeriesCacheKey builds a collision-free, tenant-isolated cache key for
// event.series* fingerprint -> labels lookups.
//
// Because event.series* timestamps are aligned to exact multiples of the table
// grain (1h / 6h / 1d / 1w), bucketing end to floor((end-1)/grain)*grain keeps
// repeated 30-60s alert evaluations on the same key within each grain window
// while rolling over cleanly at grain boundaries.
func buildSeriesCacheKey(
	ctx context.Context,
	metricName string,
	tableName string,
	start int64,
	end int64,
	normalized bool,
	matchers []*prompb.LabelMatcher,
) string {
	tenant, claimsOrgID, commentOrgID := tenantScopeParts(ctx)

	endBucket := end
	if end > start {
		endBucket = floorBucket(end-1, seriesTableGrainMs(tableName))
	}

	var b strings.Builder
	b.Grow(128)
	writeLenPrefixed(&b, "t", tenant)
	writeLenPrefixed(&b, "c", claimsOrgID)
	writeLenPrefixed(&b, "o", commentOrgID)
	writeLenPrefixed(&b, "tbl", tableName)
	b.WriteString("|s=")
	b.WriteString(strconv.FormatInt(start, 10))
	b.WriteString("|e=")
	b.WriteString(strconv.FormatInt(endBucket, 10))
	if normalized {
		b.WriteString("|n=1")
	} else {
		b.WriteString("|n=0")
	}
	writeLenPrefixed(&b, "m", metricName)

	if len(matchers) > 0 {
		sorted := make([]*prompb.LabelMatcher, len(matchers))
		copy(sorted, matchers)
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Name != sorted[j].Name {
				return sorted[i].Name < sorted[j].Name
			}
			if sorted[i].Type != sorted[j].Type {
				return sorted[i].Type < sorted[j].Type
			}
			return sorted[i].Value < sorted[j].Value
		})

		for _, m := range sorted {
			b.WriteString("|f=")
			b.WriteString(strconv.Itoa(int(m.Type)))
			b.WriteByte(':')
			b.WriteString(strconv.Itoa(len(m.Name)))
			b.WriteByte(':')
			b.WriteString(m.Name)
			b.WriteByte('=')
			b.WriteString(strconv.Itoa(len(m.Value)))
			b.WriteByte(':')
			b.WriteString(m.Value)
		}
	}

	return b.String()
}

func writeLenPrefixed(b *strings.Builder, tag, val string) {
	if b.Len() > 0 {
		b.WriteByte('|')
	}
	b.WriteString(tag)
	b.WriteByte('=')
	b.WriteString(strconv.Itoa(len(val)))
	b.WriteByte(':')
	b.WriteString(val)
}

// tenantScopeParts extracts every tenant/org marker that may scope ctx so cache
// entries are never shared across organizations.
func tenantScopeParts(ctx context.Context) (tenant, claimsOrgID, commentOrgID string) {
	if ctx == nil {
		return "", "", ""
	}
	if t, err := authtypes.TenantFromContext(ctx); err == nil {
		tenant = t
	}
	if claims, err := authtypes.ClaimsFromContext(ctx); err == nil {
		claimsOrgID = claims.OrgID
	}
	if comment := ctxtypes.CommentFromContext(ctx); comment != nil {
		if v, ok := comment.Map()["org_id"]; ok {
			commentOrgID = v
		}
	}
	return tenant, claimsOrgID, commentOrgID
}

func seriesTableGrainMs(tableName string) int64 {
	switch tableName {
	case telemetrymetrics.Series6hTableName:
		return (6 * time.Hour).Milliseconds()
	case telemetrymetrics.Series1dTableName:
		return (24 * time.Hour).Milliseconds()
	case telemetrymetrics.Series1wTableName:
		return (7 * 24 * time.Hour).Milliseconds()
	default:
		return time.Hour.Milliseconds()
	}
}

func floorBucket(tsMs, grainMs int64) int64 {
	if grainMs <= 0 {
		return tsMs
	}
	rem := tsMs % grainMs
	if rem < 0 {
		rem += grainMs
	}
	return tsMs - rem
}
