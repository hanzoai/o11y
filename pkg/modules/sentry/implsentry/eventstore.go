package implsentry

import (
	"context"
	"fmt"
	"time"

	"github.com/hanzo-ds/go/lib/driver"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Sentry error events are rows of event.error — the ONE error table, sharing the
// envelope with event.event / event.log / event.span. The database is named for what
// it holds and the table for what it is, so a query reads FROM error.
const (
	defaultEventsDB    = "event"
	defaultEventsTable = "error"
)

// errorKind is the envelope's kind discriminator for a captured exception. event.error
// holds errors; kind says which sort of error a row is.
const errorKind = "exception"

// insertColumns is the fixed-order column list the batch sink appends to. Order is
// identical to the Append call in Insert. Columns the Sentry wire has no value for
// (session_id, distinct_id, anonymous_id, url, el) are omitted so the table's own
// defaults apply.
const insertColumns = "org, product, id, time, ingested_at, name, kind, level, class, " +
	"message, site, " + groupCol + ", handled, environment, release, service, path, " +
	"trace_id, span_id, person_id, attributes, " +
	"frames.function, frames.file, frames.line, frames.column, frames.own"

// eventStore is the datastore-backed EventStore over event.error.
//
// It does NOT create its schema. The event plane is applied as one schema; a reader
// that also runs CREATE DATABASE / CREATE TABLE quietly resurrects whatever it names
// the moment the process restarts, which is how a dropped database comes back from the
// dead. Schema is a deploy artifact, this is a client.
type eventStore struct {
	store telemetrystore.TelemetryStore
	scope Scope
	db    string
	table string
	now   func() time.Time
}

// Option configures the event store.
type Option func(*eventStore)

func WithDatabase(db string) Option { return func(s *eventStore) { s.db = db } }
func WithTable(t string) Option     { return func(s *eventStore) { s.table = t } }

// NewEventStore builds the events plane over the shared datastore connection.
//
// `scope` is REQUIRED, not an Option, because it is the tenant boundary: it is what
// turns the ids this process works in into the names the plane stores (see plane.go).
// A store that could be built without one could write a row under a spelling no
// reader binds, which is exactly the split this seam exists to close.
func NewEventStore(store telemetrystore.TelemetryStore, scope Scope, opts ...Option) sentrytypes.EventStore {
	s := &eventStore{
		store: store,
		scope: scope,
		db:    defaultEventsDB,
		table: defaultEventsTable,
		now:   func() time.Time { return time.Now().UTC() },
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// names is every operation's first act: the (org, product) this one is about, in the
// plane's words. It fails CLOSED — an id with no name yields an error, never a row
// under a placeholder — so an unnamed tenant is refused rather than commingled.
func (s *eventStore) names(ctx context.Context, orgID, projectID valuer.UUID) (string, string, error) {
	if s.scope == nil {
		return "", "", errors.Newf(errors.TypeInternal, sentrytypes.ErrCodeSentryInvalidInput,
			"event store has no scope: the plane's names cannot be resolved")
	}
	return s.scope(ctx, orgID, projectID)
}

func (s *eventStore) Insert(ctx context.Context, orgID, projectID valuer.UUID, events []*sentrytypes.Event) error {
	if len(events) == 0 {
		return nil
	}
	org, product, err := s.names(ctx, orgID, projectID)
	if err != nil {
		return err
	}
	batch, err := s.store.Datastore().PrepareBatch(ctx,
		fmt.Sprintf("INSERT INTO %s.%s (%s)", s.db, s.table, insertColumns), driver.WithReleaseConnection())
	if err != nil {
		return err
	}
	defer func() { _ = batch.Abort() }()

	received := s.now()
	for _, e := range events {
		if err := batch.Append(row(org, product, received, e)...); err != nil {
			return err
		}
	}
	return batch.Send()
}

// row is ONE occurrence as the plane stores it, in insertColumns order. Pure, so the
// row a write produces can be asserted without a sink — and the two facts that make a
// row legible, the tenant's NAME and the product's, are read off the result rather
// than inferred from the call.
func row(org, product string, received time.Time, e *sentrytypes.Event) []any {
	fn, file, line, col, own := unzipFrames(e.Frames)
	return []any{
		org, product, e.EventID, e.Timestamp, received, e.Type, errorKind, e.Level, e.Type,
		e.Message, e.Culprit, e.Fingerprint, e.Handled, e.Environment, e.Release,
		e.ServiceName, e.Transaction, e.TraceID, e.SpanID, e.UserID, attributesOf(e),
		fn, file, line, col, own,
	}
}

func (s *eventStore) Discover(ctx context.Context, orgID, projectID valuer.UUID, req *sentrytypes.DiscoverRequest, w sentrytypes.Window) (*sentrytypes.DiscoverResult, error) {
	org, product, nameErr := s.names(ctx, orgID, projectID)
	if nameErr != nil {
		return nil, nameErr
	}
	sql, args, cols, err := buildDiscover(s.db, s.table, org, product, req, w)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.Datastore().Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := &sentrytypes.DiscoverResult{Columns: make([]string, len(cols))}
	for i, c := range cols {
		out.Columns[i] = c.Name
	}
	for rows.Next() {
		dest, boxes := scanTargets(cols)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, boxes())
	}
	return out, rows.Err()
}

func (s *eventStore) GetEvent(ctx context.Context, orgID, projectID valuer.UUID, eventID string) (*sentrytypes.Event, error) {
	org, product, err := s.names(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	sql, args := buildGetEvent(s.db, s.table, org, product, eventID)
	events, err := s.queryEvents(ctx, sql, args)
	if err != nil || len(events) == 0 {
		return nil, err
	}
	return events[0], nil
}

func (s *eventStore) ListForFingerprint(ctx context.Context, orgID, projectID valuer.UUID, fingerprint string, limit int) ([]*sentrytypes.Event, error) {
	org, product, err := s.names(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	sql, args := buildListForFingerprint(s.db, s.table, org, product, fingerprint, limit)
	return s.queryEvents(ctx, sql, args)
}

func (s *eventStore) ListForTrace(ctx context.Context, orgID, projectID valuer.UUID, traceID string, limit int) ([]*sentrytypes.Event, error) {
	if traceID == "" {
		return nil, nil
	}
	org, product, err := s.names(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	sql, args := buildListForTrace(s.db, s.table, org, product, traceID, limit)
	return s.queryEvents(ctx, sql, args)
}

func (s *eventStore) ListLogs(ctx context.Context, orgID, projectID valuer.UUID, query string, w sentrytypes.Window, limit int) ([]*sentrytypes.Event, error) {
	org, product, err := s.names(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	sql, args := buildListLogs(s.db, s.table, org, product, query, w, limit)
	return s.queryEvents(ctx, sql, args)
}

func (s *eventStore) DistinctFingerprints(ctx context.Context, orgID, projectID valuer.UUID, w sentrytypes.Window) ([]string, error) {
	org, product, err := s.names(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	sql, args := buildDistinctFingerprints(s.db, s.table, org, product, w)
	rows, err := s.store.Datastore().Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var fps []string
	for rows.Next() {
		var fp string
		if err := rows.Scan(&fp); err != nil {
			return nil, err
		}
		fps = append(fps, fp)
	}
	return fps, rows.Err()
}

func (s *eventStore) ListTraces(ctx context.Context, orgID, projectID valuer.UUID, w sentrytypes.Window, limit int) ([]*sentrytypes.TraceSummary, error) {
	org, product, err := s.names(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	sql, args := buildListTraces(s.db, s.table, org, product, w, limit)
	rows, err := s.store.Datastore().Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*sentrytypes.TraceSummary
	for rows.Next() {
		t := new(sentrytypes.TraceSummary)
		if err := rows.Scan(&t.TraceID, &t.Count, &t.FirstSeen, &t.LastSeen, &t.Message); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *eventStore) Stats(ctx context.Context, orgID, projectID valuer.UUID, field string, w sentrytypes.Window) ([]sentrytypes.StatsPoint, error) {
	org, product, nameErr := s.names(ctx, orgID, projectID)
	if nameErr != nil {
		return nil, nameErr
	}
	sql, args, err := buildStats(s.db, s.table, org, product, field, w)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.Datastore().Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sentrytypes.StatsPoint
	for rows.Next() {
		var p sentrytypes.StatsPoint
		if err := rows.Scan(&p.Time, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// queryEvents runs a fixed-projection (selectColumns) query and scans rows into Events.
func (s *eventStore) queryEvents(ctx context.Context, sql string, args []any) ([]*sentrytypes.Event, error) {
	rows, err := s.store.Datastore().Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*sentrytypes.Event
	for rows.Next() {
		e := new(sentrytypes.Event)
		e.Tags = map[string]string{}
		var fn, file []string
		var line, col []uint32
		var own []bool
		if err := rows.Scan(
			&e.OrgID, &e.ProjectID, &e.EventID, &e.Timestamp, &e.ReceivedAt,
			&e.Level, &e.Type, &e.Message, &e.Culprit, &e.Fingerprint, &e.Handled,
			&e.Environment, &e.Release, &e.ServiceName, &e.Transaction,
			&e.TraceID, &e.SpanID, &e.Platform, &e.ServerName, &e.UserID,
			&e.UserEmail, &e.UserIP, &e.Tags,
			&fn, &file, &line, &col, &own,
		); err != nil {
			return nil, err
		}
		e.Frames = zipFrames(fn, file, line, col, own)
		out = append(out, e)
	}
	return out, rows.Err()
}

// attributesOf folds an event's tags plus the envelope-adjacent values that live in
// the attributes map (platform, server, user contact) into the one map column. Tags
// are copied, never mutated in place, so the caller's map is untouched.
func attributesOf(e *sentrytypes.Event) map[string]string {
	attrs := make(map[string]string, len(e.Tags)+4)
	for k, v := range e.Tags {
		attrs[k] = v
	}
	for k, v := range map[string]string{
		"platform":   e.Platform,
		"server":     e.ServerName,
		"user_email": e.UserEmail,
		"user_ip":    e.UserIP,
	} {
		if v != "" {
			attrs[k] = v
		}
	}
	return attrs
}

// unzipFrames splits []Frame into the five parallel arrays event.error stores. A nil
// slice yields empty (not nil) arrays so a batch column is never null.
func unzipFrames(frames []sentrytypes.Frame) (fn, file []string, line, col []uint32, own []bool) {
	fn, file = make([]string, len(frames)), make([]string, len(frames))
	line, col = make([]uint32, len(frames)), make([]uint32, len(frames))
	own = make([]bool, len(frames))
	for i, f := range frames {
		fn[i], file[i], line[i], col[i], own[i] = f.Function, f.File, f.Line, f.Column, f.Own
	}
	return
}

// zipFrames rebuilds []Frame from the parallel arrays, tolerating a short array (a
// row written before a column existed reads as a zero value, never a panic).
func zipFrames(fn, file []string, line, col []uint32, own []bool) []sentrytypes.Frame {
	if len(fn) == 0 {
		return nil
	}
	out := make([]sentrytypes.Frame, len(fn))
	for i := range fn {
		out[i] = sentrytypes.Frame{
			Function: fn[i],
			File:     at(file, i),
			Line:     at(line, i),
			Column:   at(col, i),
			Own:      at(own, i),
		}
	}
	return out
}

// at reads index i of a parallel array, yielding the zero value when the array is
// shorter than the frame count.
func at[T any](s []T, i int) T {
	var zero T
	if i < len(s) {
		return s[i]
	}
	return zero
}

// scanTargets allocates a typed scan destination per Discover output column and
// returns the destinations plus a boxing closure that reads their values into []any
// (the untyped result row).
func scanTargets(cols []discoverCol) ([]any, func() []any) {
	dest := make([]any, len(cols))
	for i, c := range cols {
		switch c.Kind {
		case kindTime:
			dest[i] = new(time.Time)
		case kindUint:
			dest[i] = new(uint64)
		case kindBool:
			dest[i] = new(bool)
		default:
			dest[i] = new(string)
		}
	}
	return dest, func() []any {
		row := make([]any, len(dest))
		for i, d := range dest {
			switch v := d.(type) {
			case *time.Time:
				row[i] = *v
			case *uint64:
				row[i] = *v
			case *bool:
				row[i] = *v
			case *string:
				row[i] = *v
			}
		}
		return row
	}
}
