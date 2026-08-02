package impltracedetail_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dsmock "github.com/hanzo-ds/mock"
	"github.com/hanzoai/o11y/pkg/modules/tracedetail/impltracedetail"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/telemetrystoretest"
	"github.com/hanzoai/o11y/pkg/types/spantypes"
	"github.com/hanzoai/o11y/pkg/types/spantypes/spantypestest"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/stretchr/testify/assert"
)

var (
	testTraceID = "trace-abc123"
	testStart   = time.Unix(1000, 0).UTC()
	testEnd     = time.Unix(2000, 0).UTC()
	testSummary = &spantypes.TraceSummary{
		TraceID:  testTraceID,
		Start:    testStart,
		End:      testEnd,
		NumSpans: 10,
	}
	svcNameField = telemetrytypes.TelemetryFieldKey{
		Name:         "service.name",
		FieldContext: telemetrytypes.FieldContextResource,
	}
	unsupportedField = telemetrytypes.TelemetryFieldKey{
		Name:         "http.method",
		FieldContext: telemetrytypes.FieldContextSpan,
	}
)

func newTestStore(matcher sqlmock.QueryMatcher) *spantypestest.TraceStoreTest {
	ts := telemetrystoretest.New(telemetrystore.Config{}, matcher)
	return spantypestest.New(impltracedetail.NewTraceStore(ts), ts.Mock())
}

func TestGetTraceSummary(t *testing.T) {
	expectedSQL := "SELECT trace_id, min(start) AS start, max(end) AS end, sum(num_spans) AS num_spans FROM event.trace WHERE trace_id = ? GROUP BY trace_id"

	t.Run("ValidTraceID_GeneratesExpectedSQL", func(t *testing.T) {
		s := newTestStore(sqlmock.QueryMatcherRegexp)
		s.Mock().ExpectQueryRow(regexp.QuoteMeta(expectedSQL)).
			WillReturnRow(dsmock.NewRow(nil, nil))
		_, _ = s.Store().GetTraceSummary(context.Background(), testTraceID)
		assert.NoError(t, s.Mock().ExpectationsWereMet())
	})
}

func TestGetMinimalSpans(t *testing.T) {
	expectedSQL := "SELECT DISTINCT ON (span_id) span_id, parent AS parent_span_id, time AS timestamp, duration AS duration_nano, toBool(status = 'error') AS has_error, service FROM event.span WHERE trace_id = ? AND ts_bucket_start >= ? AND ts_bucket_start <= ? ORDER BY timestamp ASC, name ASC"

	t.Run("ValidRange_GeneratesExpectedSQL", func(t *testing.T) {
		s := newTestStore(sqlmock.QueryMatcherRegexp)
		s.Mock().ExpectSelect(regexp.QuoteMeta(expectedSQL)).
			WillReturnRows(dsmock.NewRows(nil, nil))
		_, _ = s.Store().GetMinimalSpans(context.Background(), testTraceID, testStart, testEnd)
		assert.NoError(t, s.Mock().ExpectationsWereMet())
	})
}

func TestGetSpanCountByField(t *testing.T) {
	expectedSQL := "SELECT service AS field_value, count(DISTINCT span_id) AS count FROM event.span WHERE trace_id = ? AND ts_bucket_start >= ? AND ts_bucket_start <= ? AND notEmpty(service) GROUP BY field_value"

	tests := []struct {
		name      string
		field     telemetrytypes.TelemetryFieldKey
		wantQuery bool
	}{
		{name: "ResourceField_GeneratesExpectedSQL", field: svcNameField, wantQuery: true},
		{name: "NonResourceField_NoSQLGenerated", field: unsupportedField},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(sqlmock.QueryMatcherRegexp)
			if tc.wantQuery {
				s.Mock().ExpectSelect(regexp.QuoteMeta(expectedSQL)).
					WillReturnRows(dsmock.NewRows(nil, nil))
			}
			_, _ = s.Store().GetSpanCountByField(context.Background(), testTraceID, testSummary, tc.field)
			assert.NoError(t, s.Mock().ExpectationsWereMet())
		})
	}
}

func TestGetFlamegraphSpans(t *testing.T) {
	baseSQL := "SELECT span_id, any(parent) AS parent_span_id, any(time) AS timestamp, any(duration) AS duration_nano, any(toBool(status = 'error')) AS has_error, any(name) AS name, any(JSONExtractArrayRaw(attributes['span.events'])) AS events, any(attributes) AS attributes_string, any(CAST(map() AS Map(String, Float64))) AS attributes_number, any(CAST(map() AS Map(String, Bool))) AS attributes_bool, any(map('service.name', toString(service), 'host', toString(host), 'url', url)) AS resources_string FROM event.span WHERE trace_id = ? AND ts_bucket_start >= ? AND ts_bucket_start <= ? GROUP BY span_id ORDER BY timestamp ASC, name ASC"
	withSpanIDsSQL := "SELECT span_id, any(parent) AS parent_span_id, any(time) AS timestamp, any(duration) AS duration_nano, any(toBool(status = 'error')) AS has_error, any(name) AS name, any(JSONExtractArrayRaw(attributes['span.events'])) AS events, any(attributes) AS attributes_string, any(CAST(map() AS Map(String, Float64))) AS attributes_number, any(CAST(map() AS Map(String, Bool))) AS attributes_bool, any(map('service.name', toString(service), 'host', toString(host), 'url', url)) AS resources_string FROM event.span WHERE trace_id = ? AND ts_bucket_start >= ? AND ts_bucket_start <= ? AND span_id IN (?, ?) GROUP BY span_id ORDER BY timestamp ASC, name ASC"

	tests := []struct {
		name    string
		spanIDs []string
		sql     string
	}{
		{name: "NoSpanIDs_GeneratesBaseSQL", spanIDs: nil, sql: baseSQL},
		{name: "WithSpanIDs_GeneratesInClauseSQL", spanIDs: []string{"span-1", "span-2"}, sql: withSpanIDsSQL},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(sqlmock.QueryMatcherRegexp)
			s.Mock().ExpectSelect(regexp.QuoteMeta(tc.sql)).
				WillReturnRows(dsmock.NewRows(nil, nil))
			_, _ = s.Store().GetFlamegraphSpans(context.Background(), testTraceID, testStart, testEnd, tc.spanIDs)
			assert.NoError(t, s.Mock().ExpectationsWereMet())
		})
	}
}

func TestGetSpanDurationByField(t *testing.T) {

	expectedSQL := "WITH all_spans AS (SELECT DISTINCT ON (span_id) service AS field_value, toUnixTimestamp64Nano(time) AS start_ns, start_ns + duration AS end_ns FROM event.span WHERE trace_id = ? AND ts_bucket_start >= ? AND ts_bucket_start <= ? AND notEmpty(field_value) ORDER BY time ASC, name ASC), effective_start AS (SELECT field_value, end_ns, greatest(start_ns, ifNull(max(end_ns) OVER (PARTITION BY field_value ORDER BY start_ns ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING), toUInt64(0))) AS effective_start_ns FROM all_spans) SELECT field_value, sum(toUInt64(greatest(end_ns - effective_start_ns, 0))) AS total_ns FROM effective_start GROUP BY field_value"

	tests := []struct {
		name      string
		field     telemetrytypes.TelemetryFieldKey
		wantQuery bool
	}{
		{name: "ResourceField_GeneratesExpectedSQL", field: svcNameField, wantQuery: true},
		{name: "NonResourceField_NoSQLGenerated", field: unsupportedField},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(sqlmock.QueryMatcherRegexp)
			if tc.wantQuery {
				s.Mock().ExpectSelect(regexp.QuoteMeta(expectedSQL)).
					WillReturnRows(dsmock.NewRows(nil, nil))
			}
			_, _ = s.Store().GetSpanDurationByField(context.Background(), testTraceID, testSummary, tc.field)
			assert.NoError(t, s.Mock().ExpectationsWereMet())
		})
	}
}
