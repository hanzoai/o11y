// Copyright (C) 2025-2026, Hanzo AI Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package sightings suppresses the dimension writes an ingest batch would
// otherwise repeat forever.
//
// The event plane stores each signal as one FACT table plus several DIMENSION
// tables: event.log beside log_attribute / log_key / log_resource_key /
// log_resource, event.span beside span_attribute / span_key / span_resource /
// operation. The fact row is new every time. The dimension rows are
// "sightings" — this org uses this attribute key, this resource exists — and
// they are overwhelmingly the SAME rows batch after batch. They are
// ReplacingMergeTree, so re-writing them is harmless; it is merely expensive.
//
// Measured on 24h of live traffic, one 14-minute production window:
//
//	event.log_attribute     21,475 rows written →     52 distinct    413x
//	event.log_key            3,902 rows written →      5 distinct    780x
//	event.log_resource_key  17,572 rows written →     17 distinct  1,034x
//	event.log_resource       1,302 rows written →     65 distinct     20x
//
// That redundancy is not free. A writer issues one INSERT round-trip per
// table, and the receiver calls the writer SYNCHRONOUSLY on its connection
// handler — so five round-trips per batch is five times the latency on the
// path that drains the socket, and the sender feels it as backpressure. Four
// of those five carried ~0.3% new information.
//
// A Set remembers what has already been written and drops the repeats, taking
// the steady state from five round-trips per batch to one.
//
// TWO RULES, both load-bearing:
//
//   - Only DIMENSION rows may pass through here. A partial contribution to an
//     aggregate — event.trace's per-batch span count, folded by an
//     AggregatingMergeTree — is not a sighting, and suppressing a repeat would
//     silently lose spans from the total.
//   - A row is recorded only after its write SUCCEEDS. Remembering a row whose
//     INSERT failed would drop it permanently. Write enforces this so no
//     caller has to.
//
// Keys carry a time bucket so a sighting refreshes periodically rather than
// never: the tables' recency columns (log_attribute.unix_milli,
// log_resource.seen_at_ts_bucket_start) stay accurate to one bucket.
package sightings

import "sync"

// DefaultSize is the per-Set entry budget. Live cardinality is ~10^2 distinct
// sightings per table; this leaves four orders of magnitude of headroom while
// staying bounded, because the key includes a rotating bucket and would
// otherwise grow without limit.
const DefaultSize = 20000

// Set records the dimension rows already written for ONE table. One Set per
// table, never shared: two tables can share a Go row type (log_key and
// log_resource_key are both keyRow), and a single Set would let a sighting in
// one suppress the same sighting in the other.
//
// Bounded by two generations rather than an LRU: when the live generation
// fills, it becomes the previous generation and a fresh one starts. Lookups
// consult both, so an entry survives at least one full generation and at most
// two are ever held. Cost is a map swap instead of per-entry bookkeeping, and
// the failure mode of forgetting early is a redundant write, not a lost row.
type Set[K comparable] struct {
	mu   sync.Mutex
	max  int
	cur  map[K]struct{}
	prev map[K]struct{}
}

// New returns a Set holding at most 2*size keys. size <= 0 takes DefaultSize.
func New[K comparable](size int) *Set[K] {
	if size <= 0 {
		size = DefaultSize
	}
	return &Set[K]{max: size, cur: make(map[K]struct{}), prev: make(map[K]struct{})}
}

func (s *Set[K]) seen(k K) bool {
	if _, ok := s.cur[k]; ok {
		return true
	}
	if _, ok := s.prev[k]; ok {
		// Promote, so a still-live sighting is not forgotten by rotation.
		s.cur[k] = struct{}{}
		return true
	}
	return false
}

func (s *Set[K]) record(k K) {
	if len(s.cur) >= s.max {
		s.prev, s.cur = s.cur, make(map[K]struct{}, s.max/4)
	}
	s.cur[k] = struct{}{}
}

// Write sends only the rows this Set has not already recorded, then records
// them — and only if the send succeeded. Duplicates within rows are collapsed
// too, so a batch repeating one attribute writes it once.
//
// send is not called at all when nothing is novel: that is the whole point,
// and it is why the steady state costs no round-trip.
func Write[T any, K comparable](s *Set[K], rows []T, key func(T) K, send func([]T) error) error {
	if len(rows) == 0 {
		return nil
	}
	novel := make([]T, 0, len(rows))
	keys := make([]K, 0, len(rows))

	s.mu.Lock()
	batch := make(map[K]struct{}, len(rows))
	for _, r := range rows {
		k := key(r)
		if _, dup := batch[k]; dup {
			continue
		}
		batch[k] = struct{}{}
		if s.seen(k) {
			continue
		}
		novel = append(novel, r)
		keys = append(keys, k)
	}
	s.mu.Unlock()

	if len(novel) == 0 {
		return nil
	}
	if err := send(novel); err != nil {
		return err // deliberately NOT recorded: the next batch must retry it
	}

	s.mu.Lock()
	for _, k := range keys {
		s.record(k)
	}
	s.mu.Unlock()
	return nil
}
