package telemetrytraces

import (
	"context"
	"fmt"
	"strings"

	"github.com/hanzoai/o11y/pkg/errors"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	schema "github.com/hanzoai/otel-collector/cmd/o11yschemamigrator/schema_migrator"
	"github.com/hanzo-ds/sqlbuilder"
	"golang.org/x/exp/maps"
)

// The span plane is event.span: the 15-column envelope plus the span identity
// columns (trace_id, span_id, parent, duration, status) and ONE attributes map,
// Map(LowCardinality(String), String). Every OTLP-fork column a query can name
// is REWRITTEN onto that envelope here — the map key is the logical field name
// the query language keeps, the Column.Name is the envelope SQL that satisfies
// it. One naming scheme wins: no attributes_string/number/bool triplet, no
// materialized attribute_string_x$$y shortcut columns, no JSON resource column
// on the main table (resource labels live in event.span_resource, reached via
// the __resource_filter CTE).
const (
	// kind and status hold the STRING enums on the live plane ('client',
	// 'server', ... / 'ok', 'error'). Numeric kind/status_code predicates and
	// selects go through these translations, so old queries keep working.
	SpanKindNumberExpr       = "toInt8(multiIf(kind = 'internal', 1, kind = 'server', 2, kind = 'client', 3, kind = 'producer', 4, kind = 'consumer', 5, 0))"
	SpanStatusCodeNumberExpr = "toInt16(multiIf(status = 'ok', 1, status = 'error', 2, 0))"
	SpanHasErrorExpr         = "toBool(status = 'error')"
	SpanHTTPMethodExpr       = "if(attributes['http.request.method'] != '', attributes['http.request.method'], attributes['http.method'])"
	SpanResponseStatusExpr   = "if(attributes['http.response.status_code'] != '', attributes['http.response.status_code'], attributes['http.status_code'])"
	SpanEventsExpr           = "arrayFilter(x -> x != '', [attributes['events']])"
)

var (
	indexV3Columns = map[string]*schema.Column{
		"ts_bucket_start":      {Name: "ts_bucket_start", Type: schema.ColumnTypeUInt64},
		"resource_fingerprint": {Name: "resource_fingerprint", Type: schema.ColumnTypeString},

		// intrinsic columns
		"timestamp":          {Name: "time", Type: schema.DateTime64ColumnType{Precision: 9, Timezone: "UTC"}},
		"trace_id":           {Name: "trace_id", Type: schema.ColumnTypeString},
		"span_id":            {Name: "span_id", Type: schema.ColumnTypeString},
		"trace_state":        {Name: "attributes['trace_state']", Type: schema.ColumnTypeString},
		"parent_span_id":     {Name: "parent", Type: schema.ColumnTypeString},
		"flags":              {Name: "toUInt32OrZero(attributes['flags'])", Type: schema.ColumnTypeUInt32},
		"name":               {Name: "name", Type: schema.ColumnTypeString},
		"kind":               {Name: SpanKindNumberExpr, Type: schema.ColumnTypeInt8},
		"kind_string":        {Name: "kind", Type: schema.ColumnTypeString},
		"duration_nano":      {Name: "duration", Type: schema.ColumnTypeUInt64},
		"status_code":        {Name: SpanStatusCodeNumberExpr, Type: schema.ColumnTypeInt16},
		"status_message":     {Name: "attributes['status.message']", Type: schema.ColumnTypeString},
		"status_code_string": {Name: "status", Type: schema.ColumnTypeString},

		// attributes columns — one physical map; the logical value type decides
		// the wrapping (FieldFor): string -> attributes[k],
		// number -> toFloat64OrNull(attributes[k]), bool -> attributes[k] = 'true'.
		"attributes_string": {Name: "attributes", Type: schema.MapColumnType{
			KeyType:   schema.LowCardinalityColumnType{ElementType: schema.ColumnTypeString},
			ValueType: schema.ColumnTypeString,
		}},
		"attributes_number": {Name: "attributes", Type: schema.MapColumnType{
			KeyType:   schema.LowCardinalityColumnType{ElementType: schema.ColumnTypeString},
			ValueType: schema.ColumnTypeFloat64,
		}},
		"attributes_bool": {Name: "attributes", Type: schema.MapColumnType{
			KeyType:   schema.LowCardinalityColumnType{ElementType: schema.ColumnTypeString},
			ValueType: schema.ColumnTypeBool,
		}},
		// resource labels other than service.name ride the same attributes map
		"resources_string": {Name: "attributes", Type: schema.MapColumnType{
			KeyType:   schema.LowCardinalityColumnType{ElementType: schema.ColumnTypeString},
			ValueType: schema.ColumnTypeString,
		}},

		"events": {Name: SpanEventsExpr, Type: schema.ArrayColumnType{
			ElementType: schema.ColumnTypeString,
		}},
		"links": {Name: "attributes['links']", Type: schema.ColumnTypeString},
		// derived columns
		"response_status_code": {Name: SpanResponseStatusExpr, Type: schema.ColumnTypeString},
		"external_http_url":    {Name: "url", Type: schema.ColumnTypeString},
		"http_url":             {Name: "url", Type: schema.ColumnTypeString},
		"external_http_method": {Name: SpanHTTPMethodExpr, Type: schema.ColumnTypeString},
		"http_method":          {Name: SpanHTTPMethodExpr, Type: schema.ColumnTypeString},
		"http_host":            {Name: "host", Type: schema.LowCardinalityColumnType{ElementType: schema.ColumnTypeString}},
		"db_name":              {Name: "attributes['db.name']", Type: schema.ColumnTypeString},
		"db_operation":         {Name: "attributes['db.operation']", Type: schema.ColumnTypeString},
		"has_error":            {Name: SpanHasErrorExpr, Type: schema.ColumnTypeBool},
		"is_remote":            {Name: "attributes['is_remote']", Type: schema.ColumnTypeString},
		// former materialized shortcut columns -> envelope expressions
		"resource_string_service$$name":         {Name: "service", Type: schema.LowCardinalityColumnType{ElementType: schema.ColumnTypeString}},
		"attribute_string_http$$route":          {Name: "attributes['http.route']", Type: schema.ColumnTypeString},
		"attribute_string_messaging$$system":    {Name: "attributes['messaging.system']", Type: schema.ColumnTypeString},
		"attribute_string_messaging$$operation": {Name: "attributes['messaging.operation']", Type: schema.ColumnTypeString},
		"attribute_string_db$$system":           {Name: "attributes['db.system']", Type: schema.ColumnTypeString},
		"attribute_string_rpc$$system":          {Name: "attributes['rpc.system']", Type: schema.ColumnTypeString},
		"attribute_string_rpc$$service":         {Name: "attributes['rpc.service']", Type: schema.ColumnTypeString},
		"attribute_string_rpc$$method":          {Name: "attributes['rpc.method']", Type: schema.ColumnTypeString},
		"attribute_string_peer$$service":        {Name: "attributes['peer.service']", Type: schema.ColumnTypeString},

		// deprecated intrinsic columns — resolve through oldToNew first; these
		// direct entries exist for lookups that bypass it and MUST target the
		// same envelope expressions, never fork columns.
		"traceID":          {Name: "trace_id", Type: schema.ColumnTypeString},
		"spanID":           {Name: "span_id", Type: schema.ColumnTypeString},
		"parentSpanID":     {Name: "parent", Type: schema.ColumnTypeString},
		"spanKind":         {Name: "kind", Type: schema.ColumnTypeString},
		"durationNano":     {Name: "duration", Type: schema.ColumnTypeUInt64},
		"statusCode":       {Name: SpanStatusCodeNumberExpr, Type: schema.ColumnTypeInt16},
		"statusMessage":    {Name: "attributes['status.message']", Type: schema.ColumnTypeString},
		"statusCodeString": {Name: "status", Type: schema.ColumnTypeString},

		// deprecated derived columns
		"references":         {Name: "attributes['links']", Type: schema.ColumnTypeString},
		"responseStatusCode": {Name: SpanResponseStatusExpr, Type: schema.ColumnTypeString},
		"externalHttpUrl":    {Name: "url", Type: schema.ColumnTypeString},
		"httpUrl":            {Name: "url", Type: schema.ColumnTypeString},
		"externalHttpMethod": {Name: SpanHTTPMethodExpr, Type: schema.ColumnTypeString},
		"httpMethod":         {Name: SpanHTTPMethodExpr, Type: schema.ColumnTypeString},
		"httpHost":           {Name: "host", Type: schema.LowCardinalityColumnType{ElementType: schema.ColumnTypeString}},
		"dbName":             {Name: "attributes['db.name']", Type: schema.ColumnTypeString},
		"dbOperation":        {Name: "attributes['db.operation']", Type: schema.ColumnTypeString},
		"hasError":           {Name: SpanHasErrorExpr, Type: schema.ColumnTypeBool},
		"isRemote":           {Name: "attributes['is_remote']", Type: schema.ColumnTypeString},
		"serviceName":        {Name: "service", Type: schema.LowCardinalityColumnType{ElementType: schema.ColumnTypeString}},
		"httpRoute":          {Name: "attributes['http.route']", Type: schema.ColumnTypeString},
		"msgSystem":          {Name: "attributes['messaging.system']", Type: schema.ColumnTypeString},
		"msgOperation":       {Name: "attributes['messaging.operation']", Type: schema.ColumnTypeString},
		"dbSystem":           {Name: "attributes['db.system']", Type: schema.ColumnTypeString},
		"rpcSystem":          {Name: "attributes['rpc.system']", Type: schema.ColumnTypeString},
		"rpcService":         {Name: "attributes['rpc.service']", Type: schema.ColumnTypeString},
		"rpcMethod":          {Name: "attributes['rpc.method']", Type: schema.ColumnTypeString},
		"peerService":        {Name: "attributes['peer.service']", Type: schema.ColumnTypeString},

		// former materialized *_exists columns -> envelope membership checks
		"resource_string_service$$name_exists":         {Name: "service != ''", Type: schema.ColumnTypeBool},
		"attribute_string_http$$route_exists":          {Name: "mapContains(attributes, 'http.route')", Type: schema.ColumnTypeBool},
		"attribute_string_messaging$$system_exists":    {Name: "mapContains(attributes, 'messaging.system')", Type: schema.ColumnTypeBool},
		"attribute_string_messaging$$operation_exists": {Name: "mapContains(attributes, 'messaging.operation')", Type: schema.ColumnTypeBool},
		"attribute_string_db$$system_exists":           {Name: "mapContains(attributes, 'db.system')", Type: schema.ColumnTypeBool},
		"attribute_string_rpc$$system_exists":          {Name: "mapContains(attributes, 'rpc.system')", Type: schema.ColumnTypeBool},
		"attribute_string_rpc$$service_exists":         {Name: "mapContains(attributes, 'rpc.service')", Type: schema.ColumnTypeBool},
		"attribute_string_rpc$$method_exists":          {Name: "mapContains(attributes, 'rpc.method')", Type: schema.ColumnTypeBool},
		"attribute_string_peer$$service_exists":        {Name: "mapContains(attributes, 'peer.service')", Type: schema.ColumnTypeBool},
	}

	// TODO(srikanthccv): remove this mapping.
	oldToNew = map[string]string{
		// deprecated intrinsic -> new intrinsic
		"traceID":          "trace_id",
		"spanID":           "span_id",
		"parentSpanID":     "parent_span_id",
		"spanKind":         "kind_string",
		"durationNano":     "duration_nano",
		"statusCode":       "status_code",
		"statusMessage":    "status_message",
		"statusCodeString": "status_code_string",

		// deprecated derived -> new derived / materialized
		"references":         "links",
		"responseStatusCode": "response_status_code",
		"externalHttpUrl":    "external_http_url",
		"httpUrl":            "http_url",
		"externalHttpMethod": "external_http_method",
		"httpMethod":         "http_method",
		"httpHost":           "http_host",
		"dbName":             "db_name",
		"dbOperation":        "db_operation",
		"hasError":           "has_error",
		"isRemote":           "is_remote",
		"serviceName":        "resource_string_service$$name",
		"httpRoute":          "attribute_string_http$$route",
		"msgSystem":          "attribute_string_messaging$$system",
		"msgOperation":       "attribute_string_messaging$$operation",
		"dbSystem":           "attribute_string_db$$system",
		"rpcSystem":          "attribute_string_rpc$$system",
		"rpcService":         "attribute_string_rpc$$service",
		"rpcMethod":          "attribute_string_rpc$$method",
		"peerService":        "attribute_string_peer$$service",
	}
)

// SpanFieldExpression returns the envelope SQL expression that satisfies the
// given logical span field name (e.g. "duration_nano" -> "duration",
// "status_message" -> "attributes['status.message']"). It is the ONE lookup
// every reader of event.span shares — the trace-detail store aliases these
// back to the logical names its scan structs expect. Unknown names return
// the input unchanged.
func SpanFieldExpression(logicalName string) string {
	if col, ok := indexV3Columns[logicalName]; ok {
		return col.Name
	}
	return logicalName
}

type defaultFieldMapper struct {
}

var _ qbtypes.FieldMapper = (*defaultFieldMapper)(nil)

func NewFieldMapper() *defaultFieldMapper {
	return &defaultFieldMapper{}
}

func (m *defaultFieldMapper) getColumn(
	_ context.Context,
	_, _ uint64,
	key *telemetrytypes.TelemetryFieldKey,
) ([]*schema.Column, error) {
	switch key.FieldContext {
	case telemetrytypes.FieldContextResource:
		// service.name is the one resource label promoted to an envelope
		// column; every other resource label rides the attributes map on the
		// row. The full label set lives in event.span_resource (labels JSON),
		// reached through the __resource_filter CTE, not on the main table.
		if key.Name == "service.name" {
			return []*schema.Column{indexV3Columns["resource_string_service$$name"]}, nil
		}
		return []*schema.Column{indexV3Columns["resources_string"]}, nil
	case telemetrytypes.FieldContextScope:
		return []*schema.Column{}, qbtypes.ErrColumnNotFound
	case telemetrytypes.FieldContextAttribute:
		switch key.FieldDataType {
		case telemetrytypes.FieldDataTypeString:
			return []*schema.Column{indexV3Columns["attributes_string"]}, nil
		case telemetrytypes.FieldDataTypeInt64,
			telemetrytypes.FieldDataTypeFloat64,
			telemetrytypes.FieldDataTypeNumber:
			return []*schema.Column{indexV3Columns["attributes_number"]}, nil
		case telemetrytypes.FieldDataTypeBool:
			return []*schema.Column{indexV3Columns["attributes_bool"]}, nil
		}
	case telemetrytypes.FieldContextSpan, telemetrytypes.FieldContextUnspecified:
		/*
			TODO: This is incorrect, we cannot assume all unspecified context fields are span context.
			User could be referring to attributes, but we cannot fix this until we fix where_clause vistior
			https://github.com/hanzoai/o11y/pull/10102
		*/
		// Check if this is a span scope field
		if strings.ToLower(key.Name) == SpanSearchScopeRoot || strings.ToLower(key.Name) == SpanSearchScopeEntryPoint {
			// The actual SQL will be generated in the condition builder
			return []*schema.Column{{Name: key.Name, Type: schema.ColumnTypeBool}}, nil
		}

		// TODO(srikanthccv): remove this when it's safe to remove
		// issue with Datastore aliasing

		/*
			NOTE: There are fields which are deprecated for only to not show up as user suggestion and is possible that
			they don't have a mapping in oldToNew map. So we need to look up in indexV3Columns directly for those fields.
			For example: kind, timestamp etc.
		*/
		if _, ok := CalculatedFieldsDeprecated[key.Name]; ok {
			// Check if we have a mapping for the deprecated calculated field
			if col, ok := indexV3Columns[oldToNew[key.Name]]; ok {
				return []*schema.Column{col}, nil
			}
		}
		if _, ok := IntrinsicFieldsDeprecated[key.Name]; ok {
			// Check if we have a mapping for the deprecated intrinsic field
			if col, ok := indexV3Columns[oldToNew[key.Name]]; ok {
				return []*schema.Column{col}, nil
			}
		}

		if col, ok := indexV3Columns[key.Name]; ok {
			return []*schema.Column{col}, nil
		}
	}
	return nil, qbtypes.ErrColumnNotFound
}

func (m *defaultFieldMapper) ColumnFor(
	ctx context.Context,
	startNs, endNs uint64,
	key *telemetrytypes.TelemetryFieldKey,
) ([]*schema.Column, error) {
	return m.getColumn(ctx, startNs, endNs, key)
}

// FieldFor returns the table field name for the given key if it exists
// otherwise it returns qbtypes.ErrColumnNotFound.
func (m *defaultFieldMapper) FieldFor(
	ctx context.Context,
	startNs, endNs uint64,
	key *telemetrytypes.TelemetryFieldKey,
) (string, error) {
	// Special handling for span scope fields
	if key.FieldContext == telemetrytypes.FieldContextSpan &&
		(strings.ToLower(key.Name) == SpanSearchScopeRoot || strings.ToLower(key.Name) == SpanSearchScopeEntryPoint) {
		// Return the field name as-is, the condition builder will handle the SQL generation
		return key.Name, nil
	}

	columns, err := m.getColumn(ctx, startNs, endNs, key)
	if err != nil {
		return "", err
	}

	var newColumns []*schema.Column
	var evolutionsEntries []*telemetrytypes.EvolutionEntry
	if len(key.Evolutions) > 0 {
		// we will use the corresponding column and its evolution entry for the query
		newColumns, evolutionsEntries, err = qbtypes.SelectEvolutionsForColumns(columns, key.Evolutions, startNs, endNs)
		if err != nil {
			return "", err
		}
	} else {
		newColumns = columns
	}

	exprs := []string{}
	existExpr := []string{}
	for i, column := range newColumns {
		// Use evolution column name if available, otherwise use the column name
		columnName := column.Name
		if evolutionsEntries != nil && evolutionsEntries[i] != nil {
			columnName = evolutionsEntries[i].ColumnName
		}

		switch column.Type.GetType() {
		case schema.ColumnTypeEnumJSON:
			// json is only supported for resource context as of now
			if key.FieldContext != telemetrytypes.FieldContextResource {
				return "", errors.Newf(errors.TypeInvalidInput, errors.CodeInvalidInput, "only resource context fields are supported for json columns, got %s", key.FieldContext.String)
			}
			// have to add ::string as datastore throws an error :- data types Variant/Dynamic are not allowed in GROUP BY
			// once datastore dependency is updated, we need to check if we can remove it.
			exprs = append(exprs, fmt.Sprintf("%s.`%s`::String", columnName, key.Name))
			existExpr = append(existExpr, fmt.Sprintf("%s.`%s` IS NOT NULL", columnName, key.Name))
		case schema.ColumnTypeEnumString,
			schema.ColumnTypeEnumUInt64,
			schema.ColumnTypeEnumUInt32,
			schema.ColumnTypeEnumInt8,
			schema.ColumnTypeEnumInt16,
			schema.ColumnTypeEnumBool,
			schema.ColumnTypeEnumDateTime64,
			schema.ColumnTypeEnumFixedString:
			exprs = append(exprs, column.Name)
		case schema.ColumnTypeEnumLowCardinality:
			switch elementType := column.Type.(schema.LowCardinalityColumnType).ElementType; elementType.GetType() {
			case schema.ColumnTypeEnumString:
				exprs = append(exprs, column.Name)
			default:
				return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "value type %s is not supported for low cardinality column type %s", elementType, column.Type)
			}
		case schema.ColumnTypeEnumMap:
			keyType := column.Type.(schema.MapColumnType).KeyType
			if _, ok := keyType.(schema.LowCardinalityColumnType); !ok {
				return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "key type %s is not supported for map column type %s", keyType, column.Type)
			}

			// One physical map of strings; the LOGICAL value type decides the
			// read expression. No materialized per-key columns exist on the
			// envelope, so key.Materialized is deliberately ignored.
			switch valueType := column.Type.(schema.MapColumnType).ValueType; valueType.GetType() {
			case schema.ColumnTypeEnumString:
				exprs = append(exprs, fmt.Sprintf("%s['%s']", columnName, key.Name))
			case schema.ColumnTypeEnumFloat64:
				exprs = append(exprs, fmt.Sprintf("toFloat64OrNull(%s['%s'])", columnName, key.Name))
			case schema.ColumnTypeEnumBool:
				exprs = append(exprs, fmt.Sprintf("toBool(%s['%s'] = 'true')", columnName, key.Name))
			default:
				return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "value type %s is not supported for map column type %s", valueType, column.Type)
			}
			existExpr = append(existExpr, fmt.Sprintf("mapContains(%s, '%s')", columnName, key.Name))
		}
	}

	if len(exprs) == 1 {
		return exprs[0], nil
	} else if len(exprs) > 1 {
		// Ensure existExpr has the same length as exprs
		if len(existExpr) != len(exprs) {
			return "", errors.New(errors.TypeInternal, errors.CodeInternal, "length of exist exprs doesn't match to that of exprs")
		}
		finalExprs := []string{}
		for i, expr := range exprs {
			finalExprs = append(finalExprs, fmt.Sprintf("%s, %s", existExpr[i], expr))
		}
		return "multiIf(" + strings.Join(finalExprs, ", ") + ", NULL)", nil
	}

	// should not reach here
	return columns[0].Name, nil
}

// ColumnExpressionFor returns the column expression for the given field
// if it exists otherwise it returns qbtypes.ErrColumnNotFound.
func (m *defaultFieldMapper) ColumnExpressionFor(
	ctx context.Context,
	startNs, endNs uint64,
	field *telemetrytypes.TelemetryFieldKey,
	keys map[string][]*telemetrytypes.TelemetryFieldKey,
) (string, error) {

	fieldExpression, err := m.FieldFor(ctx, startNs, endNs, field)
	if errors.Is(err, qbtypes.ErrColumnNotFound) {
		// the key didn't have the right context to be added to the query
		// we try to use the context we know of
		keysForField := keys[field.Name]
		if len(keysForField) == 0 {
			// is it a static field?
			if _, ok := indexV3Columns[field.Name]; ok {
				// if it is, attach the column name directly
				field.FieldContext = telemetrytypes.FieldContextSpan
				fieldExpression, _ = m.FieldFor(ctx, startNs, endNs, field)
			} else {
				// - the context is not provided
				// - there are not keys for the field
				// - it is not a static field
				// - the next best thing to do is see if there is a typo
				// and suggest a correction
				wrappedErr := errors.Wrapf(err, errors.TypeInvalidInput, errors.CodeInvalidInput, "field `%s` not found", field.Name).WithSuggestions(errors.NewSuggestionsOnLevenshteinDistance(field.Name, errors.NounKeys, maps.Keys(keys))...)
				return "", wrappedErr
			}
		} else if len(keysForField) == 1 {
			// we have a single key for the field, use it
			fieldExpression, _ = m.FieldFor(ctx, startNs, endNs, keysForField[0])
		} else {
			// select any non-empty value from the keys
			args := []string{}
			for _, key := range keysForField {
				fieldExpression, _ = m.FieldFor(ctx, startNs, endNs, key)
				args = append(args, fmt.Sprintf("toString(%s) != '', toString(%s)", fieldExpression, fieldExpression))
			}
			fieldExpression = fmt.Sprintf("multiIf(%s, NULL)", strings.Join(args, ", "))
		}
	}

	return fmt.Sprintf("%s AS `%s`", sqlbuilder.Escape(fieldExpression), field.Name), nil
}
