// Copyright (C) 2025-2026, Hanzo AI Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package datastorelogs

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hanzo-ds/go/lib/column"
	"github.com/hanzo-ds/go/lib/driver"
)

// countingConn records every INSERT the writer opens, so a test can assert on
// ROUND TRIPS rather than on rows. The receiver calls the writer synchronously
// on its connection handler, so round-trips per batch are intake latency.
type countingConn struct {
	prepared []string
	fail     bool
}

var tableRe = regexp.MustCompile(`INTO\s+(\S+)`)

func (c *countingConn) PrepareBatch(_ context.Context, query string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	name := query
	if m := tableRe.FindStringSubmatch(query); m != nil {
		name = m[1]
	}
	c.prepared = append(c.prepared, name)
	return &countingBatch{fail: c.fail}, nil
}

func (c *countingConn) tables() string { return strings.Join(c.prepared, " ") }

func (c *countingConn) Contributors() []string                            { return nil }
func (c *countingConn) ServerVersion() (*driver.ServerVersion, error)     { return nil, nil }
func (c *countingConn) Select(context.Context, any, string, ...any) error { return nil }
func (c *countingConn) Query(context.Context, string, ...any) (driver.Rows, error) {
	return nil, nil
}
func (c *countingConn) QueryRow(context.Context, string, ...any) driver.Row { return nil }
func (c *countingConn) Exec(context.Context, string, ...any) error          { return nil }
func (c *countingConn) AsyncInsert(context.Context, string, bool, ...any) error {
	return nil
}
func (c *countingConn) Ping(context.Context) error { return nil }
func (c *countingConn) Stats() driver.Stats        { return driver.Stats{} }
func (c *countingConn) Close() error               { return nil }

type countingBatch struct {
	n    int
	fail bool
}

func (b *countingBatch) Append(...any) error           { b.n++; return nil }
func (b *countingBatch) Send() error                   { return nil }
func (b *countingBatch) Abort() error                  { return nil }
func (b *countingBatch) AppendStruct(any) error        { return nil }
func (b *countingBatch) Column(int) driver.BatchColumn { return nil }
func (b *countingBatch) Flush() error                  { return nil }
func (b *countingBatch) IsSent() bool                  { return false }
func (b *countingBatch) Rows() int                     { return b.n }
func (b *countingBatch) Columns() []column.Interface   { return nil }
func (b *countingBatch) Close() error                  { return nil }

func testWriter(c *countingConn) *Writer {
	return NewWriter(c, WithNow(func() time.Time { return testNow }))
}

// The FIRST batch pays for every table. Every batch after it, with the same
// services and attribute keys, costs ONE round-trip — the fact table alone.
// That is the whole point: four of the five INSERTs carried ~0.3% new
// information and sat in the intake path.
func TestSteadyStateCostsOneRoundTrip(t *testing.T) {
	c := &countingConn{}
	w := testWriter(c)
	ctx := context.Background()

	if err := w.WriteLogs(ctx, testBatch()); err != nil {
		t.Fatalf("first WriteLogs: %v", err)
	}
	first := len(c.prepared)
	if first != 5 {
		t.Fatalf("first batch opened %d inserts (%s); want 5 (log + 4 dimensions)", first, c.tables())
	}

	c.prepared = nil
	for i := 0; i < 10; i++ {
		if err := w.WriteLogs(ctx, testBatch()); err != nil {
			t.Fatalf("repeat WriteLogs: %v", err)
		}
	}
	if got := len(c.prepared); got != 10 {
		t.Fatalf("10 repeat batches opened %d inserts (%s); want 10 (the fact table only)", got, c.tables())
	}
	for _, tbl := range c.prepared {
		if !strings.HasSuffix(tbl, "."+"log") && !strings.Contains(tbl, ".log") {
			t.Fatalf("repeat batch wrote %q; only the fact table should repeat", tbl)
		}
		if strings.Contains(tbl, "log_") {
			t.Fatalf("repeat batch re-wrote dimension table %q", tbl)
		}
	}
}

// The fact table is NEVER suppressed: every log line must be written, however
// many times an identical line repeats.
func TestFactTableIsNeverSuppressed(t *testing.T) {
	c := &countingConn{}
	w := testWriter(c)
	for i := 0; i < 4; i++ {
		if err := w.WriteLogs(context.Background(), testBatch()); err != nil {
			t.Fatal(err)
		}
	}
	logWrites := 0
	for _, tbl := range c.prepared {
		if strings.HasSuffix(tbl, ".log") {
			logWrites++
		}
	}
	if logWrites != 4 {
		t.Fatalf("event.log written %d times for 4 batches; want 4", logWrites)
	}
}

// log_key and log_resource_key share the keyRow type. They must NOT share a
// sighting Set, or a key seen as a tag would suppress the same name as a
// resource key and the read plane would lose it.
func TestKeyTablesDoNotSuppressEachOther(t *testing.T) {
	c := &countingConn{}
	w := testWriter(c)
	if err := w.WriteLogs(context.Background(), testBatch()); err != nil {
		t.Fatal(err)
	}
	var sawKey, sawResourceKey bool
	for _, tbl := range c.prepared {
		switch {
		case strings.HasSuffix(tbl, ".log_resource_key"):
			sawResourceKey = true
		case strings.HasSuffix(tbl, ".log_key"):
			sawKey = true
		}
	}
	if !sawKey || !sawResourceKey {
		t.Fatalf("log_key=%v log_resource_key=%v; both must be written (%s)", sawKey, sawResourceKey, c.tables())
	}
}

// A new attribute key appearing later must still reach the dimension table —
// suppression is per sighting, not a one-shot latch.
func TestNovelDimensionStillWritesAfterWarmup(t *testing.T) {
	c := &countingConn{}
	w := testWriter(c)
	ctx := context.Background()
	if err := w.WriteLogs(ctx, testBatch()); err != nil {
		t.Fatal(err)
	}
	c.prepared = nil

	b := testBatch()
	b.Records[0].Attributes["brand.new.key"] = "v"
	if err := w.WriteLogs(ctx, b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.tables(), "log_attribute") {
		t.Fatalf("a new attribute key did not reach log_attribute (%s)", c.tables())
	}
}
