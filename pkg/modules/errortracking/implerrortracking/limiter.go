package implerrortracking

import (
	"sync"
	"time"

	"github.com/hanzoai/o11y/pkg/valuer"
)

// Per-org token-bucket rate limit on the public ingest path — sustained-flood
// backpressure that complements the per-request event cap and per-org issue ceiling
// (which bound a SINGLE request). It is applied AFTER DSN verification, so a bucket
// is only ever created for an authenticated org (a forged project fails HMAC first
// and never allocates state). In-process/per-replica by design; a cross-replica
// distributed quota is a fast-follow.
const (
	ingestRatePerSec = 50  // steady-state events/requests per org per replica
	ingestBurst      = 100 // burst allowance
)

type tokenBucket struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func (b *tokenBucket) allow(rate, burst float64, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens += now.Sub(b.last).Seconds() * rate
	if b.tokens > burst {
		b.tokens = burst
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

type rateLimiter struct {
	rate    float64
	burst   float64
	buckets sync.Map // orgID string -> *tokenBucket
}

func newRateLimiter(rate, burst float64) *rateLimiter {
	return &rateLimiter{rate: rate, burst: burst}
}

func (l *rateLimiter) allow(org valuer.UUID) bool {
	v, _ := l.buckets.LoadOrStore(org.String(), &tokenBucket{tokens: l.burst, last: time.Now()})
	return v.(*tokenBucket).allow(l.rate, l.burst, time.Now())
}
