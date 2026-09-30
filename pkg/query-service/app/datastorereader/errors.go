package datastorereader

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hanzo-ds/go"
	errorsV2 "github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/query-service/constants"
	"github.com/hanzoai/o11y/pkg/query-service/model"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/ctxtypes"
	"github.com/hanzoai/o11y/pkg/types/instrumentationtypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
)

// Exceptions are rows of the event plane's fact table with signal 'error'
// (deploy/datastore/migrations/0002_event_fact.sql). The exceptions pages read
// them in their own vocabulary, so each envelope column is aliased to the name
// the page reads: issue is the group, class the exception type, and the stack
// is rendered from the frames. Every read binds the tenant on ctx as org.

// errorService is the service an exception is listed under: the service that
// raised it, or the product surface for a browser error, which names none.
const errorService = "if(service != '', service, product)"

// errorStack renders the frames innermost first, one "at" line each.
const errorStack = "concat(class, ': ', message, arrayStringConcat(arrayMap((f, p, l, c) -> " +
	"concat('\\n    at ', if(f = '', '', concat(f, ' (')), p, ':', toString(l), ':', toString(c), if(f = '', '', ')')), " +
	"frames.function, frames.file, frames.line, frames.column)))"

// errorColumns is one exception with its span, as model.ErrorWithSpan scans it.
const errorColumns = "id AS errorID, class AS exceptionType, " + errorStack + " AS exceptionStacktrace, " +
	"NOT handled AS exceptionEscaped, message AS exceptionMessage, time AS timestamp, " +
	"span_id AS spanID, trace_id AS traceID, " + errorService + " AS serviceName, issue AS groupID"

// errorScope is the predicate every exception read starts with.
const errorScope = "org = @org AND signal = 'error'"

// errorFacts returns the tenant's scope arguments and the table exceptions are
// read from.
func (r *DatastoreReader) errorFacts(ctx context.Context) (string, []any, *model.ApiError) {
	tenant, err := authtypes.TenantFromContext(ctx)
	if err != nil {
		return "", nil, &model.ApiError{Typ: model.ErrorForbidden, Err: err}
	}
	return r.TraceDB + "." + r.errorTable, []any{datastore.Named("org", tenant)}, nil
}

func errorWindow(start, end *time.Time) []any {
	return []any{
		datastore.Named("timestampL", strconv.FormatInt(start.UnixNano(), 10)),
		datastore.Named("timestampU", strconv.FormatInt(end.UnixNano(), 10)),
	}
}

// errorTagColumn is the fact-table expression for a tag map the exceptions
// filter names. Attributes are one string map, so number and bool tags read it
// through a typed view; resource attributes are not stored on the fact.
func errorTagColumn(tagMap string) (string, error) {
	switch tagMap {
	case model.StringTagMapCol:
		return "attributes", nil
	case model.NumberTagMapCol:
		return "mapApply((k, v) -> (k, toFloat64OrZero(v)), attributes)", nil
	case model.BoolTagMapCol:
		return "mapApply((k, v) -> (k, v = 'true'), attributes)", nil
	}
	return "", fmt.Errorf("exceptions cannot be filtered on %s", tagMap)
}

// errorFilter is the service, type and tag predicates a list or count adds.
func errorFilter(ctx context.Context, serviceName, exceptionType string, tagParams []model.TagQueryParam) (string, []any, *model.ApiError) {
	query := ""
	var args []any
	if serviceName != "" {
		query += " AND " + errorService + " ilike @serviceName"
		args = append(args, datastore.Named("serviceName", "%"+serviceName+"%"))
	}
	if exceptionType != "" {
		query += " AND class ilike @exceptionType"
		args = append(args, datastore.Named("exceptionType", "%"+exceptionType+"%"))
	}
	sub, subArgs, apiErr := buildQueryWithTagParams(ctx, createTagQueryFromTagQueryParams(tagParams), errorTagColumn)
	if apiErr != nil {
		return "", nil, apiErr
	}
	return query + sub, append(args, subArgs...), nil
}

func (r *DatastoreReader) ListErrors(ctx context.Context, queryParams *model.ListErrorsParams) (*[]model.Error, *model.ApiError) {
	ctx = ctxtypes.NewContextWithCommentVals(ctx, map[string]string{
		instrumentationtypes.TelemetrySignal:  telemetrytypes.SignalTraces.StringValue(),
		instrumentationtypes.CodeNamespace:    "datastore-reader",
		instrumentationtypes.CodeFunctionName: "ListErrors",
	})
	table, args, apiErr := r.errorFacts(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	filter, filterArgs, apiErr := errorFilter(ctx, queryParams.ServiceName, queryParams.ExceptionType, queryParams.Tags)
	if apiErr != nil {
		return nil, apiErr
	}
	query := listErrorsQuery(table, filter, queryParams)
	args = append(append(args, errorWindow(queryParams.Start, queryParams.End)...), filterArgs...)
	if queryParams.Limit > 0 {
		args = append(args, datastore.Named("limit", queryParams.Limit))
	}
	if queryParams.Offset > 0 {
		args = append(args, datastore.Named("offset", queryParams.Offset))
	}

	var out []model.Error
	if err := r.db.Select(ctx, &out, query, args...); err != nil {
		r.logger.Error("Error in processing sql query", errorsV2.Attr(err))
		return nil, &model.ApiError{Typ: model.ErrorExec, Err: fmt.Errorf("error in processing sql query")}
	}
	return &out, nil
}

// listErrorsQuery groups the tenant's exceptions by issue over the window.
func listErrorsQuery(table, filter string, p *model.ListErrorsParams) string {
	query := "SELECT any(message) AS exceptionMessage, count() AS exceptionCount, min(time) AS firstSeen, max(time) AS lastSeen, issue AS groupID, " +
		"any(" + errorService + ") AS serviceName, any(class) AS exceptionType" +
		" FROM " + table + " WHERE " + errorScope + " AND time >= @timestampL AND time <= @timestampU" + filter +
		" GROUP BY issue"
	if p.OrderParam != "" {
		switch p.Order {
		case constants.Descending:
			query += " ORDER BY " + p.OrderParam + " DESC"
		case constants.Ascending:
			query += " ORDER BY " + p.OrderParam + " ASC"
		}
	}
	if p.Limit > 0 {
		query += " LIMIT @limit"
	}
	if p.Offset > 0 {
		query += " OFFSET @offset"
	}
	return query
}

func (r *DatastoreReader) CountErrors(ctx context.Context, queryParams *model.CountErrorsParams) (uint64, *model.ApiError) {
	ctx = ctxtypes.NewContextWithCommentVals(ctx, map[string]string{
		instrumentationtypes.TelemetrySignal:  telemetrytypes.SignalTraces.StringValue(),
		instrumentationtypes.CodeNamespace:    "datastore-reader",
		instrumentationtypes.CodeFunctionName: "CountErrors",
	})
	table, args, apiErr := r.errorFacts(ctx)
	if apiErr != nil {
		return 0, apiErr
	}
	filter, filterArgs, apiErr := errorFilter(ctx, queryParams.ServiceName, queryParams.ExceptionType, queryParams.Tags)
	if apiErr != nil {
		return 0, apiErr
	}
	query := "SELECT uniqExact(issue) FROM " + table + " WHERE " + errorScope + " AND time >= @timestampL AND time <= @timestampU" + filter
	args = append(append(args, errorWindow(queryParams.Start, queryParams.End)...), filterArgs...)

	var count uint64
	if err := r.db.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		r.logger.Error("Error in processing sql query", errorsV2.Attr(err))
		return 0, &model.ApiError{Typ: model.ErrorExec, Err: fmt.Errorf("error in processing sql query")}
	}
	return count, nil
}

func (r *DatastoreReader) GetErrorFromErrorID(ctx context.Context, queryParams *model.GetErrorParams) (*model.ErrorWithSpan, *model.ApiError) {
	ctx = ctxtypes.NewContextWithCommentVals(ctx, map[string]string{
		instrumentationtypes.TelemetrySignal:  telemetrytypes.SignalTraces.StringValue(),
		instrumentationtypes.CodeNamespace:    "datastore-reader",
		instrumentationtypes.CodeFunctionName: "GetErrorFromErrorID",
	})
	if queryParams.ErrorID == "" {
		return nil, &model.ApiError{Typ: model.ErrorBadData, Err: fmt.Errorf("ErrorID missing from params")}
	}
	table, args, apiErr := r.errorFacts(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	query := "SELECT " + errorColumns + " FROM " + table + " WHERE " + errorScope +
		" AND time = @timestamp AND issue = @groupID AND id = @errorID LIMIT 1"
	args = append(args, errorPoint(queryParams)...)
	return r.oneError(ctx, query, args)
}

func (r *DatastoreReader) GetErrorFromGroupID(ctx context.Context, queryParams *model.GetErrorParams) (*model.ErrorWithSpan, *model.ApiError) {
	ctx = ctxtypes.NewContextWithCommentVals(ctx, map[string]string{
		instrumentationtypes.TelemetrySignal:  telemetrytypes.SignalTraces.StringValue(),
		instrumentationtypes.CodeNamespace:    "datastore-reader",
		instrumentationtypes.CodeFunctionName: "GetErrorFromGroupID",
	})
	table, args, apiErr := r.errorFacts(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	query := "SELECT " + errorColumns + " FROM " + table + " WHERE " + errorScope +
		" AND time = @timestamp AND issue = @groupID LIMIT 1"
	args = append(args,
		datastore.Named("groupID", queryParams.GroupID),
		datastore.Named("timestamp", strconv.FormatInt(queryParams.Timestamp.UnixNano(), 10)))
	return r.oneError(ctx, query, args)
}

func errorPoint(p *model.GetErrorParams) []any {
	return []any{
		datastore.Named("errorID", p.ErrorID),
		datastore.Named("groupID", p.GroupID),
		datastore.Named("timestamp", strconv.FormatInt(p.Timestamp.UnixNano(), 10)),
	}
}

func (r *DatastoreReader) oneError(ctx context.Context, query string, args []any) (*model.ErrorWithSpan, *model.ApiError) {
	var rows []model.ErrorWithSpan
	if err := r.db.Select(ctx, &rows, query, args...); err != nil {
		r.logger.Error("Error in processing sql query", errorsV2.Attr(err))
		return nil, &model.ApiError{Typ: model.ErrorExec, Err: fmt.Errorf("error in processing sql query")}
	}
	if len(rows) == 0 {
		return nil, &model.ApiError{Typ: model.ErrorNotFound, Err: fmt.Errorf("Error/Exception not found")}
	}
	return &rows[0], nil
}

func (r *DatastoreReader) GetNextPrevErrorIDs(ctx context.Context, queryParams *model.GetErrorParams) (*model.NextPrevErrorIDs, *model.ApiError) {
	if queryParams.ErrorID == "" {
		return nil, &model.ApiError{Typ: model.ErrorBadData, Err: fmt.Errorf("ErrorID missing from params")}
	}
	table, scope, apiErr := r.errorFacts(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	out := model.NextPrevErrorIDs{GroupID: queryParams.GroupID}
	out.NextErrorID, out.NextTimestamp, apiErr = r.adjacentError(ctx, table, scope, queryParams, true)
	if apiErr != nil {
		return nil, apiErr
	}
	out.PrevErrorID, out.PrevTimestamp, apiErr = r.adjacentError(ctx, table, scope, queryParams, false)
	if apiErr != nil {
		return nil, apiErr
	}
	return &out, nil
}

// adjacentError is the exception after (next) or before the given one in its
// group, ordered by time and then by id among exceptions that share a time.
func (r *DatastoreReader) adjacentError(ctx context.Context, table string, scope []any, p *model.GetErrorParams, next bool) (string, time.Time, *model.ApiError) {
	name := "getPrevErrorID"
	cmp, idCmp, dir := "<", "<", "DESC"
	if next {
		name = "getNextErrorID"
		cmp, idCmp, dir = ">", ">", "ASC"
	}
	ctx = ctxtypes.NewContextWithCommentVals(ctx, map[string]string{
		instrumentationtypes.TelemetrySignal:  telemetrytypes.SignalTraces.StringValue(),
		instrumentationtypes.CodeNamespace:    "datastore-reader",
		instrumentationtypes.CodeFunctionName: name,
	})
	query := "SELECT id AS errorID, time AS timestamp FROM " + table + " WHERE " + errorScope +
		" AND issue = @groupID AND (time " + cmp + " @timestamp OR (time = @timestamp AND id " + idCmp + " @errorID))" +
		" ORDER BY time " + dir + ", id " + dir + " LIMIT 1"
	args := append(append([]any{}, scope...), errorPoint(p)...)

	var rows []struct {
		ErrorID   string    `ch:"errorID"`
		Timestamp time.Time `ch:"timestamp"`
	}
	if err := r.db.Select(ctx, &rows, query, args...); err != nil {
		r.logger.Error("Error in processing sql query", errorsV2.Attr(err))
		return "", time.Time{}, &model.ApiError{Typ: model.ErrorExec, Err: fmt.Errorf("error in processing sql query")}
	}
	if len(rows) == 0 {
		return "", time.Time{}, nil
	}
	return rows[0].ErrorID, rows[0].Timestamp, nil
}
