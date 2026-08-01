package telemetrytraces

import (
	"context"
	"fmt"
	"strings"

	"github.com/hanzo-ds/sqlbuilder"
	"github.com/hanzoai/o11y/pkg/errors"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/spantypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	schema "github.com/hanzoai/otel-collector/cmd/o11yschemamigrator/schema_migrator"
	"golang.org/x/exp/maps"
)

var (
	// indexV3Columns maps every logical span field to its physical form on the ONE
	// event.span envelope (org,time,…,attributes Map(LowCardinality(String),String),
	// host,service,trace_id,span_id,parent,duration,status,kind,…). The OTLP-fork
	// column names are gone: intrinsics rename to their envelope column, everything
	// else is an expression over the single `attributes` map or the service/host/url
	// columns. FieldFor returns Name verbatim for scalar/low-card types and
	// `Name['key']` for maps, so an envelope expression placed in Name IS the emitted
	// SQL. Types stay faithful to each expression's RESULT so the exists logic in the
	// condition builder picks the right emptiness test.
	attributesMap = &schema.Column{Name: "attributes", Type: schema.MapColumnType{
		KeyType:   schema.LowCardinalityColumnType{ElementType: schema.ColumnTypeString},
		ValueType: schema.ColumnTypeString,
	}}
	lowCardString  = schema.LowCardinalityColumnType{ElementType: schema.ColumnTypeString}
	indexV3Columns = map[string]*schema.Column{
		"ts_bucket_start":      {Name: "ts_bucket_start", Type: schema.ColumnTypeUInt64},
		"resource_fingerprint": {Name: "resource_fingerprint", Type: schema.ColumnTypeString},

		// intrinsic columns -> envelope columns / expressions
		"timestamp":      {Name: "time", Type: schema.DateTime64ColumnType{Precision: 9, Timezone: "UTC"}},
		"trace_id":       {Name: "trace_id", Type: schema.FixedStringColumnType{Length: 32}},
		"span_id":        {Name: "span_id", Type: schema.ColumnTypeString},
		"trace_state":    {Name: "attributes['trace_state']", Type: schema.ColumnTypeString},
		"parent_span_id": {Name: "parent", Type: schema.ColumnTypeString},
		"flags":          {Name: spantypes.ExprFlags, Type: schema.ColumnTypeUInt32},
		"name":           {Name: "name", Type: lowCardString},
		// kind and status are the writer's normalised lowercase enums
		// (datastoretraces normEnum), so the numeric views decode from those
		// spellings and cast to the width the consumer scans into.
		"kind":               {Name: spantypes.ExprKind, Type: schema.ColumnTypeInt8},
		"kind_string":        {Name: "kind", Type: lowCardString},
		"duration_nano":      {Name: "duration", Type: schema.ColumnTypeUInt64},
		"status_code":        {Name: spantypes.ExprStatusCode, Type: schema.ColumnTypeInt16},
		"status_message":     {Name: "attributes['status.message']", Type: schema.ColumnTypeString},
		"status_code_string": {Name: "status", Type: lowCardString},

		// attributes columns: all three data-type views address the ONE envelope
		// map; FieldFor wraps number/bool by ValueType (toFloat64OrNull / = 'true').
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
		// resource labels beyond service/host live in the same envelope map.
		"attributes": attributesMap,

		"events": {Name: spantypes.ExprEvents, Type: schema.ArrayColumnType{ElementType: schema.ColumnTypeString}},
		"links":  {Name: "attributes['links']", Type: schema.ColumnTypeString},
		// derived columns -> envelope expressions
		"response_status_code": {Name: spantypes.ExprResponseStatusCode, Type: lowCardString},
		"external_http_url":    {Name: "url", Type: lowCardString},
		"http_url":             {Name: "url", Type: lowCardString},
		"external_http_method": {Name: spantypes.ExprHTTPMethod, Type: lowCardString},
		"http_method":          {Name: spantypes.ExprHTTPMethod, Type: lowCardString},
		"http_host":            {Name: "host", Type: lowCardString},
		"db_name":              {Name: "attributes['db.name']", Type: lowCardString},
		"db_operation":         {Name: "attributes['db.operation']", Type: lowCardString},
		"has_error":            {Name: spantypes.ExprHasError, Type: schema.ColumnTypeBool},
		"is_remote":            {Name: "attributes['is_remote']", Type: lowCardString},
		// materialized shortcuts -> envelope service column / attributes map
		"resource_string_service$$name":         {Name: "service", Type: schema.ColumnTypeString},
		"attribute_string_http$$route":          {Name: "attributes['http.route']", Type: schema.ColumnTypeString},
		"attribute_string_messaging$$system":    {Name: "attributes['messaging.system']", Type: schema.ColumnTypeString},
		"attribute_string_messaging$$operation": {Name: "attributes['messaging.operation']", Type: schema.ColumnTypeString},
		"attribute_string_db$$system":           {Name: "attributes['db.system']", Type: schema.ColumnTypeString},
		"attribute_string_rpc$$system":          {Name: "attributes['rpc.system']", Type: schema.ColumnTypeString},
		"attribute_string_rpc$$service":         {Name: "attributes['rpc.service']", Type: schema.ColumnTypeString},
		"attribute_string_rpc$$method":          {Name: "attributes['rpc.method']", Type: schema.ColumnTypeString},
		"attribute_string_peer$$service":        {Name: "attributes['peer.service']", Type: schema.ColumnTypeString},

		// deprecated intrinsic columns (resolved through oldToNew to the envelope)
		"traceID":          {Name: "trace_id", Type: schema.FixedStringColumnType{Length: 32}},
		"spanID":           {Name: "span_id", Type: schema.ColumnTypeString},
		"parentSpanID":     {Name: "parent", Type: schema.ColumnTypeString},
		"spanKind":         {Name: "kind", Type: lowCardString},
		"durationNano":     {Name: "duration", Type: schema.ColumnTypeUInt64},
		"statusCode":       {Name: spantypes.ExprStatusCode, Type: schema.ColumnTypeInt16},
		"statusMessage":    {Name: "attributes['status.message']", Type: schema.ColumnTypeString},
		"statusCodeString": {Name: "status", Type: lowCardString},

		// deprecated derived columns (resolved through oldToNew to the envelope)
		"references":         {Name: "attributes['links']", Type: schema.ColumnTypeString},
		"responseStatusCode": {Name: spantypes.ExprResponseStatusCode, Type: schema.ColumnTypeString},
		"externalHttpUrl":    {Name: "url", Type: schema.ColumnTypeString},
		"httpUrl":            {Name: "url", Type: schema.ColumnTypeString},
		"externalHttpMethod": {Name: spantypes.ExprHTTPMethod, Type: schema.ColumnTypeString},
		"httpMethod":         {Name: spantypes.ExprHTTPMethod, Type: lowCardString},
		"httpHost":           {Name: "host", Type: lowCardString},
		"dbName":             {Name: "attributes['db.name']", Type: lowCardString},
		"dbOperation":        {Name: "attributes['db.operation']", Type: lowCardString},
		"hasError":           {Name: spantypes.ExprHasError, Type: schema.ColumnTypeBool},
		"isRemote":           {Name: "attributes['is_remote']", Type: lowCardString},
		"serviceName":        {Name: "service", Type: lowCardString},
		"httpRoute":          {Name: "attributes['http.route']", Type: lowCardString},
		"msgSystem":          {Name: "attributes['messaging.system']", Type: lowCardString},
		"msgOperation":       {Name: "attributes['messaging.operation']", Type: lowCardString},
		"dbSystem":           {Name: "attributes['db.system']", Type: lowCardString},
		"rpcSystem":          {Name: "attributes['rpc.system']", Type: lowCardString},
		"rpcService":         {Name: "attributes['rpc.service']", Type: lowCardString},
		"rpcMethod":          {Name: "attributes['rpc.method']", Type: lowCardString},
		"peerService":        {Name: "attributes['peer.service']", Type: lowCardString},

		// deprecated exists shortcuts -> envelope membership tests
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

// PromotedResourceColumn returns the envelope column a resource label is
// promoted to. The envelope lifts service and host OUT of the attributes map
// into typed columns of their own; every other resource label stays in the map.
// This is the single declaration of that promotion: the field mapper resolves
// through it, and the condition builder consults it to know that a promoted
// column always exists and therefore needs no exists filter.
func PromotedResourceColumn(name string) (*schema.Column, bool) {
	switch name {
	case "service.name":
		return &schema.Column{Name: "service", Type: lowCardString}, true
	case "host.name", "host":
		return &schema.Column{Name: "host", Type: lowCardString}, true
	}
	return nil, false
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
		if col, ok := PromotedResourceColumn(key.Name); ok {
			return []*schema.Column{col}, nil
		}
		return []*schema.Column{attributesMap}, nil
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
	// An evolution selects BETWEEN columns; the envelope resolves every key to
	// exactly one column, so there is nothing to select and the evolution
	// metadata (which names OTLP-fork columns) does not apply.
	if len(key.Evolutions) > 0 && len(columns) > 1 {
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

			switch valueType := column.Type.(schema.MapColumnType).ValueType; valueType.GetType() {
			case schema.ColumnTypeEnumString, schema.ColumnTypeEnumFloat64, schema.ColumnTypeEnumBool:
				// The envelope keeps ONE attributes map with String values and no
				// materialized columns, so every key is a map access; the requested
				// value type decides the cast (number -> toFloat64OrNull, bool -> = 'true').
				access := fmt.Sprintf("%s['%s']", columnName, key.Name)
				switch valueType.GetType() {
				case schema.ColumnTypeEnumFloat64:
					access = fmt.Sprintf("toFloat64OrNull(%s)", access)
				case schema.ColumnTypeEnumBool:
					access = fmt.Sprintf("%s = 'true'", access)
				}
				exprs = append(exprs, access)
				existExpr = append(existExpr, fmt.Sprintf("mapContains(%s, '%s')", columnName, key.Name))
			default:
				return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "value type %s is not supported for map column type %s", valueType, column.Type)
			}
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
