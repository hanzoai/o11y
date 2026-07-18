package implsentry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hanzo-ds/go/lib/driver"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Default database + table for the columnar events plane. The o11y convention is one
// database per telemetry plane (o11y_traces / o11y_logs / o11y_metrics); the Sentry
// events plane is o11y_sentry, mirroring it.
const (
	defaultEventsDB    = "o11y_sentry"
	defaultEventsTable = "o11y_sentry_events"
)

// insertSQL is the fixed-order INSERT the batch sink appends to. Column order is
// identical to selectColumns / scanEvent so a written row reads back field-for-field.
const insertSQL = "INSERT INTO %s.%s (org_id, project_id, event_id, timestamp, received_at, level, type, value, message, culprit, fingerprint, platform, environment, release, service_name, transaction, trace_id, span_id, server_name, user_id, user_email, user_ip, tags, sample)"

// createSchemaDDL is the events-plane schema.
//
// HONEST STATUS — NOT LIVE-VERIFIED. This DDL was designed against the o11y datastore
// conventions and is exercised by the hanzo-ds/mock round-trip test, but it has
// NOT been byte-verified against a live datastore in this build (no datastore was
// reachable — localhost:9000 closed, no O11Y_DATASTORE_DSN). Two things a live run
// must confirm before this is called done:
//  1. the datastore accepts this exact DDL (types, INDEX/TTL syntax);
//  2. on a MULTI-SHARD datastore this plain MergeTree must become the o11y
//     local + Distributed(ON CLUSTER) split (the distributed_* convention) so a read
//     on any node sees all shards. It is correct as-is only for a single-shard /
//     replicated topology, which is why the engine + database are configurable
//     (WithDatabase/WithEngine) — the follow-on is config, not a rewrite.
const createSchemaDDL = `CREATE TABLE IF NOT EXISTS %s.%s (
	org_id       String,
	project_id   String,
	event_id     String,
	timestamp    DateTime64(9),
	received_at  DateTime64(3),
	level        LowCardinality(String),
	type         LowCardinality(String),
	value        String,
	message      String,
	culprit      String,
	fingerprint  String,
	platform     LowCardinality(String),
	environment  LowCardinality(String),
	release      String,
	service_name LowCardinality(String),
	transaction  String,
	trace_id     String,
	span_id      String,
	server_name  String,
	user_id      String,
	user_email   String,
	user_ip      String,
	tags         Map(String, String),
	sample       String,
	INDEX idx_fingerprint fingerprint TYPE bloom_filter GRANULARITY 4,
	INDEX idx_trace_id trace_id TYPE bloom_filter GRANULARITY 4
) ENGINE = %s
PARTITION BY toDate(timestamp)
ORDER BY (org_id, project_id, timestamp)
TTL toDateTime(timestamp) + INTERVAL %d DAY
SETTINGS index_granularity = 8192`

const defaultEngine = "MergeTree"
const defaultRetentionDays = 30

// eventStore is the datastore-backed EventStore. It ensures its schema lazily (once,
// retried until it succeeds) so it works identically in the standalone runtime and the
// cloud embed with no boot wiring, and touches the datastore only when actually used.
type eventStore struct {
	store         telemetrystore.TelemetryStore
	db            string
	table         string
	engine        string
	retentionDays int
	now           func() time.Time

	ensureMu   sync.Mutex
	ensureDone bool
}

// Option configures the event store.
type Option func(*eventStore)

func WithDatabase(db string) Option { return func(s *eventStore) { s.db = db } }
func WithTable(t string) Option     { return func(s *eventStore) { s.table = t } }
func WithEngine(e string) Option    { return func(s *eventStore) { s.engine = e } }
func WithRetentionDays(d int) Option {
	return func(s *eventStore) {
		if d > 0 {
			s.retentionDays = d
		}
	}
}

// NewEventStore builds the events plane over the shared datastore connection.
func NewEventStore(store telemetrystore.TelemetryStore, opts ...Option) sentrytypes.EventStore {
	s := &eventStore{
		store:         store,
		db:            defaultEventsDB,
		table:         defaultEventsTable,
		engine:        defaultEngine,
		retentionDays: defaultRetentionDays,
		now:           func() time.Time { return time.Now().UTC() },
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// ensureSchema creates the database + table idempotently. It caches success so the
// DDL runs once; a failure is returned (and retried next call) so a transient datastore
// outage does not permanently disable the plane.
func (s *eventStore) ensureSchema(ctx context.Context) error {
	s.ensureMu.Lock()
	defer s.ensureMu.Unlock()
	if s.ensureDone {
		return nil
	}
	conn := s.store.Datastore()
	if err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", s.db)); err != nil {
		return err
	}
	if err := conn.Exec(ctx, fmt.Sprintf(createSchemaDDL, s.db, s.table, s.engine, s.retentionDays)); err != nil {
		return err
	}
	s.ensureDone = true
	return nil
}

func (s *eventStore) Insert(ctx context.Context, orgID, projectID valuer.UUID, events []*sentrytypes.Event) error {
	if len(events) == 0 {
		return nil
	}
	if err := s.ensureSchema(ctx); err != nil {
		return err
	}
	batch, err := s.store.Datastore().PrepareBatch(ctx,
		fmt.Sprintf(insertSQL, s.db, s.table), driver.WithReleaseConnection())
	if err != nil {
		return err
	}
	defer func() { _ = batch.Abort() }()

	received := s.now()
	org, proj := orgID.String(), projectID.String()
	for _, e := range events {
		if err := batch.Append(
			org, proj, e.EventID, e.Timestamp, received,
			e.Level, e.Type, e.Value, e.Message, e.Culprit, e.Fingerprint,
			e.Platform, e.Environment, e.Release, e.ServiceName, e.Transaction,
			e.TraceID, e.SpanID, e.ServerName, e.UserID, e.UserEmail, e.UserIP,
			mapOrEmpty(e.Tags), e.Sample,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *eventStore) Discover(ctx context.Context, orgID, projectID valuer.UUID, req *sentrytypes.DiscoverRequest, w sentrytypes.Window) (*sentrytypes.DiscoverResult, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	sql, args, cols, err := buildDiscover(s.db, s.table, orgID.String(), projectID.String(), req, w)
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
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	sql, args := buildGetEvent(s.db, s.table, orgID.String(), projectID.String(), eventID)
	events, err := s.queryEvents(ctx, sql, args)
	if err != nil || len(events) == 0 {
		return nil, err
	}
	return events[0], nil
}

func (s *eventStore) ListForFingerprint(ctx context.Context, orgID, projectID valuer.UUID, fingerprint string, limit int) ([]*sentrytypes.Event, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	sql, args := buildListForFingerprint(s.db, s.table, orgID.String(), projectID.String(), fingerprint, limit)
	return s.queryEvents(ctx, sql, args)
}

func (s *eventStore) ListForTrace(ctx context.Context, orgID, projectID valuer.UUID, traceID string, limit int) ([]*sentrytypes.Event, error) {
	if traceID == "" {
		return nil, nil
	}
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	sql, args := buildListForTrace(s.db, s.table, orgID.String(), projectID.String(), traceID, limit)
	return s.queryEvents(ctx, sql, args)
}

func (s *eventStore) ListLogs(ctx context.Context, orgID, projectID valuer.UUID, query string, w sentrytypes.Window, limit int) ([]*sentrytypes.Event, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	sql, args := buildListLogs(s.db, s.table, orgID.String(), projectID.String(), query, w, limit)
	return s.queryEvents(ctx, sql, args)
}

func (s *eventStore) DistinctFingerprints(ctx context.Context, orgID, projectID valuer.UUID, w sentrytypes.Window) ([]string, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	sql, args := buildDistinctFingerprints(s.db, s.table, orgID.String(), projectID.String(), w)
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
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	sql, args := buildListTraces(s.db, s.table, orgID.String(), projectID.String(), w, limit)
	rows, err := s.store.Datastore().Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*sentrytypes.TraceSummary
	for rows.Next() {
		t := new(sentrytypes.TraceSummary)
		if err := rows.Scan(&t.TraceID, &t.Count, &t.FirstSeen, &t.LastSeen, &t.Sample); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *eventStore) Stats(ctx context.Context, orgID, projectID valuer.UUID, field string, w sentrytypes.Window) ([]sentrytypes.StatsPoint, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	sql, args, err := buildStats(s.db, s.table, orgID.String(), projectID.String(), field, w)
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
		if e.Tags == nil {
			e.Tags = map[string]string{}
		}
		if err := rows.Scan(
			&e.OrgID, &e.ProjectID, &e.EventID, &e.Timestamp, &e.ReceivedAt,
			&e.Level, &e.Type, &e.Value, &e.Message, &e.Culprit, &e.Fingerprint,
			&e.Platform, &e.Environment, &e.Release, &e.ServiceName, &e.Transaction,
			&e.TraceID, &e.SpanID, &e.ServerName, &e.UserID, &e.UserEmail, &e.UserIP,
			&e.Tags, &e.Sample,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
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
			case *string:
				row[i] = *v
			}
		}
		return row
	}
}

func mapOrEmpty(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
