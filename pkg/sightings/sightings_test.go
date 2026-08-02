package sightings

import (
	"errors"
	"sync"
	"testing"
)

type row struct {
	bucket int64
	name   string
}

func id(r row) row { return r }

// The steady state must cost NO send: the second time the same sighting
// arrives, the writer does not touch the database.
func TestRepeatCostsNoSend(t *testing.T) {
	s := New[row](0)
	sends := 0
	rows := []row{{1, "a"}, {1, "b"}}

	for i := 0; i < 5; i++ {
		if err := Write(s, rows, id, func(got []row) error { sends++; return nil }); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if sends != 1 {
		t.Fatalf("send called %d times; the repeats must cost nothing", sends)
	}
}

func TestOnlyNovelRowsAreSent(t *testing.T) {
	s := New[row](0)
	var sent [][]row
	send := func(got []row) error { sent = append(sent, append([]row(nil), got...)); return nil }

	_ = Write(s, []row{{1, "a"}, {1, "b"}}, id, send)
	_ = Write(s, []row{{1, "a"}, {1, "c"}}, id, send)

	if len(sent) != 2 {
		t.Fatalf("sent %d batches, want 2", len(sent))
	}
	if len(sent[1]) != 1 || sent[1][0].name != "c" {
		t.Fatalf("second send = %v, want only the novel row c", sent[1])
	}
}

// A repeated row inside ONE batch is written once.
func TestDuplicatesWithinBatchCollapse(t *testing.T) {
	s := New[row](0)
	var got []row
	_ = Write(s, []row{{1, "a"}, {1, "a"}, {1, "a"}}, id, func(r []row) error { got = r; return nil })
	if len(got) != 1 {
		t.Fatalf("sent %d rows, want 1", len(got))
	}
}

// A different bucket is a different sighting: recency columns must refresh.
func TestBucketRotationRefreshes(t *testing.T) {
	s := New[row](0)
	sends := 0
	send := func([]row) error { sends++; return nil }
	_ = Write(s, []row{{1, "a"}}, id, send)
	_ = Write(s, []row{{1, "a"}}, id, send)
	_ = Write(s, []row{{2, "a"}}, id, send) // next bucket
	if sends != 2 {
		t.Fatalf("sends=%d, want 2 (once per bucket)", sends)
	}
}

// THE correctness rule: a failed write is never remembered, or the row is lost
// forever.
func TestFailedWriteIsRetried(t *testing.T) {
	s := New[row](0)
	attempts := 0
	fail := func([]row) error { attempts++; return errors.New("datastore down") }

	if err := Write(s, []row{{1, "a"}}, id, fail); err == nil {
		t.Fatal("Write must surface the send error")
	}
	if err := Write(s, []row{{1, "a"}}, id, fail); err == nil {
		t.Fatal("Write must surface the send error")
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d; a failed write must be retried, not remembered", attempts)
	}
	// And once it succeeds, it is remembered.
	ok := 0
	_ = Write(s, []row{{1, "a"}}, id, func([]row) error { ok++; return nil })
	_ = Write(s, []row{{1, "a"}}, id, func([]row) error { ok++; return nil })
	if ok != 1 {
		t.Fatalf("after success, sends=%d, want 1", ok)
	}
}

// Bounded: far more distinct keys than the budget must not grow without limit.
func TestBounded(t *testing.T) {
	s := New[row](100)
	for i := int64(0); i < 5000; i++ {
		_ = Write(s, []row{{i, "a"}}, id, func([]row) error { return nil })
	}
	s.mu.Lock()
	total := len(s.cur) + len(s.prev)
	s.mu.Unlock()
	if total > 200 {
		t.Fatalf("holding %d keys, want <= 200 (2 generations of 100)", total)
	}
}

func TestEmptyInputDoesNotSend(t *testing.T) {
	s := New[row](0)
	called := false
	if err := Write(s, nil, id, func([]row) error { called = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("send called for an empty slice")
	}
}

func TestConcurrentWriteIsSafe(t *testing.T) {
	s := New[row](0)
	var mu sync.Mutex
	sent := 0
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = Write(s, []row{{int64(n % 5), "a"}}, id, func(r []row) error {
				mu.Lock()
				sent += len(r)
				mu.Unlock()
				return nil
			})
		}(i)
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if sent < 5 || sent > 50 {
		t.Fatalf("sent %d rows; want between 5 (perfect dedup) and 50 (no dedup)", sent)
	}
}
