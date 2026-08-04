package telemetrylogs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/hanzoai/o11y/pkg/datastoresql"

	"github.com/hanzo-ds/sqlbuilder"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/querybuilder"
	"github.com/hanzoai/o11y/pkg/telemetryresourcefilter"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/types/featuretypes"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type logQueryStatementBuilder struct {
	logger                         *slog.Logger
	metadataStore                  telemetrytypes.MetadataStore
	fm                             qbtypes.FieldMapper
	cb                             qbtypes.ConditionBuilder
	resourceFilterResolver         *telemetryresourcefilter.ResourceFingerprintResolver[qbtypes.LogAggregation]
	aggExprRewriter                qbtypes.AggExprRewriter
	fl                             flagger.Flagger
	skipResourceFingerprintEnabled bool

	fullTextColumn *telemetrytypes.TelemetryFieldKey
	jsonKeyToKey   qbtypes.JsonKeyToFieldFunc
}

var _ qbtypes.StatementBuilder[qbtypes.LogAggregation] = (*logQueryStatementBuilder)(nil)

func NewLogQueryStatementBuilder(
	settings factory.ProviderSettings,
	metadataStore telemetrytypes.MetadataStore,
	fieldMapper qbtypes.FieldMapper,
	conditionBuilder qbtypes.ConditionBuilder,
	aggExprRewriter qbtypes.AggExprRewriter,
	fullTextColumn *telemetrytypes.TelemetryFieldKey,
	jsonKeyToKey qbtypes.JsonKeyToFieldFunc,
	fl flagger.Flagger,
	telemetryStore telemetrystore.TelemetryStore,
	skipResourceFingerprintEnable bool,
	skipResourceFingerprintThreshold uint64,
) *logQueryStatementBuilder {
	logsSettings := factory.NewScopedProviderSettings(settings, "github.com/hanzoai/o11y/pkg/telemetrylogs")

	resourceFilterResolver := telemetryresourcefilter.NewResolver[qbtypes.LogAggregation](
		settings,
		DBName,
		LogResourceTableName,
		telemetrytypes.SignalLogs,
		telemetrytypes.SourceUnspecified,
		metadataStore,
		fullTextColumn,
		jsonKeyToKey,
		fl,
		telemetryStore,
		skipResourceFingerprintThreshold,
	)

	return &logQueryStatementBuilder{
		logger:                         logsSettings.Logger(),
		metadataStore:                  metadataStore,
		fm:                             fieldMapper,
		cb:                             conditionBuilder,
		resourceFilterResolver:         resourceFilterResolver,
		aggExprRewriter:                aggExprRewriter,
		fl:                             fl,
		skipResourceFingerprintEnabled: skipResourceFingerprintEnable,
		fullTextColumn:                 fullTextColumn,
		jsonKeyToKey:                   jsonKeyToKey,
	}
}

// Build builds a SQL query for logs based on the given parameters.
func (b *logQueryStatementBuilder) Build(
	ctx context.Context,
	start uint64,
	end uint64,
	requestType qbtypes.RequestType,
	query qbtypes.QueryBuilderQuery[qbtypes.LogAggregation],
	variables map[string]qbtypes.VariableItem,
) (*qbtypes.Statement, error) {

	start = querybuilder.ToNanoSecs(start)
	end = querybuilder.ToNanoSecs(end)
	// TODO(Tushar): thread orgID here to evaluate correctly
	bodyJSONEnabled := b.fl.BooleanOrEmpty(ctx, flagger.FeatureUseJSONBody, featuretypes.NewFlaggerEvaluationContext(valuer.UUID{}))

	keySelectors, warnings := getKeySelectors(query, bodyJSONEnabled)
	keys, _, err := b.metadataStore.GetKeysMulti(ctx, keySelectors)
	if err != nil {
		return nil, err
	}

	query = b.adjustKeys(ctx, keys, query, requestType)

	// Create SQL builder
	q := sqlbuilder.NewSelectBuilder()

	var stmt *qbtypes.Statement
	switch requestType {
	case qbtypes.RequestTypeRaw, qbtypes.RequestTypeRawStream:
		stmt, err = b.buildListQuery(ctx, q, query, start, end, keys, variables)
	case qbtypes.RequestTypeTimeSeries:
		stmt, err = b.buildTimeSeriesQuery(ctx, q, query, start, end, keys, variables)
	case qbtypes.RequestTypeScalar:
		stmt, err = b.buildScalarQuery(ctx, q, query, start, end, keys, false, variables)
	default:
		return nil, errors.NewInvalidInputf(errors.CodeInvalidInput, "unsupported request type: %s", requestType)
	}

	if err != nil {
		return nil, err
	}

	stmt.Warnings = append(stmt.Warnings, warnings...)
	return stmt, nil
}

func getKeySelectors(query qbtypes.QueryBuilderQuery[qbtypes.LogAggregation], bodyJSONEnabled bool) ([]*telemetrytypes.FieldKeySelector, []string) {
	var keySelectors []*telemetrytypes.FieldKeySelector
	var warnings []string

	for idx := range query.Aggregations {
		aggExpr := query.Aggregations[idx]
		selectors := querybuilder.QueryStringToKeysSelectors(aggExpr.Expression)
		keySelectors = append(keySelectors, selectors...)
	}

	if query.Filter != nil && query.Filter.Expression != "" {
		whereClauseSelectors := querybuilder.QueryStringToKeysSelectors(query.Filter.Expression)
		keySelectors = append(keySelectors, whereClauseSelectors...)
	}

	for idx := range query.GroupBy {
		groupBy := query.GroupBy[idx]
		keySelectors = append(keySelectors, &telemetrytypes.FieldKeySelector{
			Name:          groupBy.Name,
			Signal:        telemetrytypes.SignalLogs,
			FieldContext:  groupBy.FieldContext,
			FieldDataType: groupBy.FieldDataType,
		})
	}

	for idx := range query.SelectFields {
		selectField := query.SelectFields[idx]
		keySelectors = append(keySelectors, &telemetrytypes.FieldKeySelector{
			Name:          selectField.Name,
			Signal:        telemetrytypes.SignalLogs,
			FieldContext:  selectField.FieldContext,
			FieldDataType: selectField.FieldDataType,
		})
	}

	for idx := range query.Order {
		keySelectors = append(keySelectors, &telemetrytypes.FieldKeySelector{
			Name:          query.Order[idx].Key.Name,
			Signal:        telemetrytypes.SignalLogs,
			FieldContext:  query.Order[idx].Key.FieldContext,
			FieldDataType: query.Order[idx].Key.FieldDataType,
		})
	}

	for idx := range keySelectors {
		keySelectors[idx].Signal = telemetrytypes.SignalLogs
		keySelectors[idx].SelectorMatchType = telemetrytypes.FieldSelectorMatchTypeExact
	}

	// When the new JSON body experience is enabled, warn the user if they use the bare
	// "body" key in the filter — queries on plain "body" default to body.message:string.
	// TODO(Piyush): Setup better for coming FTS support.
	if bodyJSONEnabled {
		for _, sel := range keySelectors {
			if sel.Name == LogsV2BodyColumn {
				warnings = append(warnings, bodySearchDefaultWarning)
				break
			}
		}
	}

	return keySelectors, warnings
}

func (b *logQueryStatementBuilder) adjustKeys(ctx context.Context, keys map[string][]*telemetrytypes.TelemetryFieldKey, query qbtypes.QueryBuilderQuery[qbtypes.LogAggregation], requestType qbtypes.RequestType) qbtypes.QueryBuilderQuery[qbtypes.LogAggregation] {

	// Always ensure timestamp and id are present in keys map
	keys["id"] = append([]*telemetrytypes.TelemetryFieldKey{{
		Name:          "id",
		Signal:        telemetrytypes.SignalLogs,
		FieldContext:  telemetrytypes.FieldContextLog,
		FieldDataType: telemetrytypes.FieldDataTypeString,
	}}, keys["id"]...)

	keys["timestamp"] = append([]*telemetrytypes.TelemetryFieldKey{{
		Name:          "timestamp",
		Signal:        telemetrytypes.SignalLogs,
		FieldContext:  telemetrytypes.FieldContextLog,
		FieldDataType: telemetrytypes.FieldDataTypeNumber,
	}}, keys["timestamp"]...)

	/*
		Adjust keys for alias expressions in aggregations
	*/
	actions := querybuilder.AdjustKeysForAliasExpressions(&query, requestType)

	/*
		Check if user is using multiple contexts or data types for same field name
		Idea is to use a super set of keys that can satisfy all the usages

		For example, lets consider model_id exists in both attributes and resources
		And user is trying to use `attribute.model_id` and `model_id`.

		In this case, we'll remove the context from `attribute.model_id`
		and make it just `model_id` and remove the duplicate entry.

		Same goes with data types.
		Consider user is using http.status_code:number and http.status_code
		In this case, we'll remove the data type from http.status_code:number
		and make it just http.status_code and remove the duplicate entry.
	*/

	actions = append(actions, querybuilder.AdjustDuplicateKeys(&query)...)

	/*
		Now adjust each key to have correct context and data type
		Here we try to make intelligent guesses which work for all users (not just majority)
		Reason for doing this is to not create an unexpected behavior for users
	*/
	for idx := range query.SelectFields {
		actions = append(actions, b.adjustKey(&query.SelectFields[idx], keys)...)
	}
	for idx := range query.GroupBy {
		actions = append(actions, b.adjustKey(&query.GroupBy[idx].TelemetryFieldKey, keys)...)
	}
	for idx := range query.Order {
		actions = append(actions, b.adjustKey(&query.Order[idx].Key.TelemetryFieldKey, keys)...)
	}

	for _, action := range actions {
		// TODO: change to debug level once we are confident about the behavior
		b.logger.InfoContext(ctx, "key adjustment action", slog.String("action", action))
	}

	return query
}

func (b *logQueryStatementBuilder) adjustKey(key *telemetrytypes.TelemetryFieldKey, keys map[string][]*telemetrytypes.TelemetryFieldKey) []string {
	// First check if it matches with any intrinsic fields
	var intrinsicOrCalculatedField telemetrytypes.TelemetryFieldKey
	if _, ok := IntrinsicFields[key.Name]; ok {
		intrinsicOrCalculatedField = IntrinsicFields[key.Name]
		return querybuilder.AdjustKey(key, keys, &intrinsicOrCalculatedField)
	}

	return querybuilder.AdjustKey(key, keys, nil)
}

// buildListQuery builds a query for list panel type.
func (b *logQueryStatementBuilder) buildListQuery(
	ctx context.Context,
	sb *sqlbuilder.SelectBuilder,
	query qbtypes.QueryBuilderQuery[qbtypes.LogAggregation],
	start, end uint64,
	keys map[string][]*telemetrytypes.TelemetryFieldKey,
	variables map[string]qbtypes.VariableItem,
) (*qbtypes.Statement, error) {

	var (
		cteFragments []string
		cteArgs      [][]any
		// TODO(Tushar): thread orgID here to evaluate correctly
		bodyJSONEnabled = b.fl.BooleanOrEmpty(ctx, flagger.FeatureUseJSONBody, featuretypes.NewFlaggerEvaluationContext(valuer.UUID{}))
	)

	frag, args, skipResourceFilter, err := b.maybeAttachResourceFilter(ctx, sb, query, start, end, variables)
	if err != nil {
		return nil, err
	}
	if frag != "" {
		cteFragments = append(cteFragments, frag)
		cteArgs = append(cteArgs, args)
	}

	// Select timestamp and id by default. The envelope carries time as DateTime64,
	// so the historic UInt64-nanosecond `timestamp` the consumer expects is
	// reconstructed with toUnixTimestamp64Nano; every other default column is an
	// envelope column or an expression over the one attributes map, aliased back to
	// the name the consumer reads.
	sb.Select("toUnixTimestamp64Nano(time) AS timestamp")
	sb.SelectMore(LogsV2IDColumn)
	if len(query.SelectFields) == 0 {
		// Select all default columns
		sb.SelectMore(LogsV2TraceIDColumn)
		sb.SelectMore(LogsV2SpanIDColumn)
		sb.SelectMore("toUInt32OrZero(attributes['trace_flags']) AS trace_flags")
		sb.SelectMore(LogsV2SeverityTextColumn)
		sb.SelectMore(LogsV2SeverityNumberColumn)
		sb.SelectMore("attributes['scope.name'] AS scope_name")
		sb.SelectMore("attributes['scope.version'] AS scope_version")
		sb.SelectMore(bodyAliasExpression(bodyJSONEnabled))
		sb.SelectMore("attributes AS attributes_string")
		sb.SelectMore("CAST(map() AS Map(String, Float64)) AS attributes_number")
		sb.SelectMore("CAST(map() AS Map(String, Bool)) AS attributes_bool")
		sb.SelectMore("map('service.name', toString(service), 'host', toString(host)) AS resources_string")
		sb.SelectMore("CAST(map() AS Map(String, String)) AS scope_string")

	} else {
		// Select specified columns
		for index := range query.SelectFields {
			if query.SelectFields[index].Name == LogsV2TimestampColumn || query.SelectFields[index].Name == LogsV2IDColumn {
				continue
			}

			// get column expression for the field - use array index directly to avoid pointer to loop variable
			colExpr, err := b.fm.ColumnExpressionFor(ctx, start, end, &query.SelectFields[index], keys)
			if err != nil {
				return nil, err
			}
			sb.SelectMore(colExpr)
		}
	}

	sb.From(fmt.Sprintf("%s.%s", DBName, LogTableName))
	// Add filter conditions
	preparedWhereClause, err := b.addFilterCondition(ctx, sb, start, end, query, keys, variables, skipResourceFilter)

	if err != nil {
		return nil, err
	}

	// Add order by
	for _, orderBy := range query.Order {

		colExpr, err := b.fm.ColumnExpressionFor(ctx, start, end, &orderBy.Key.TelemetryFieldKey, keys)
		if err != nil {
			return nil, err
		}
		sb.OrderBy(fmt.Sprintf("%s %s", colExpr, orderBy.Direction.StringValue()))
	}

	// Add limit and offset
	if query.Limit > 0 {
		sb.Limit(query.Limit)
	} else {
		sb.Limit(100)
	}

	if query.Offset > 0 {
		sb.Offset(query.Offset)
	}

	mainSQL, mainArgs := sb.BuildWithFlavor(datastoresql.Flavor)

	finalSQL := querybuilder.CombineCTEs(cteFragments) + mainSQL
	finalArgs := querybuilder.PrependArgs(cteArgs, mainArgs)

	stmt := &qbtypes.Statement{
		Query:          finalSQL,
		Args:           finalArgs,
		Warnings:       preparedWhereClause.Warnings,
		WarningsDocURL: preparedWhereClause.WarningsDocURL,
	}

	return stmt, nil
}

func (b *logQueryStatementBuilder) buildTimeSeriesQuery(
	ctx context.Context,
	sb *sqlbuilder.SelectBuilder,
	query qbtypes.QueryBuilderQuery[qbtypes.LogAggregation],
	start, end uint64,
	keys map[string][]*telemetrytypes.TelemetryFieldKey,
	variables map[string]qbtypes.VariableItem,
) (*qbtypes.Statement, error) {

	var (
		cteFragments []string
		cteArgs      [][]any
		// TODO(Tushar): thread orgID here to evaluate correctly
		bodyJSONEnabled = b.fl.BooleanOrEmpty(ctx, flagger.FeatureUseJSONBody, featuretypes.NewFlaggerEvaluationContext(valuer.UUID{}))
	)

	frag, args, skipResourceFilter, err := b.maybeAttachResourceFilter(ctx, sb, query, start, end, variables)
	if err != nil {
		return nil, err
	}
	if frag != "" {
		cteFragments = append(cteFragments, frag)
		cteArgs = append(cteArgs, args)
	}

	sb.SelectMore(fmt.Sprintf(
		"toStartOfInterval(time, INTERVAL %d SECOND) AS ts",
		int64(query.StepInterval.Seconds()),
	))

	var allGroupByArgs []any

	// Keep original column expressions so we can build the tuple
	fieldNames := make([]string, 0, len(query.GroupBy))
	for _, gb := range query.GroupBy {
		expr, args, err := querybuilder.CollisionHandledFinalExpr(ctx, start, end, &gb.TelemetryFieldKey, b.fm, b.cb, keys, telemetrytypes.FieldDataTypeString, b.jsonKeyToKey, bodyJSONEnabled)
		if err != nil {
			return nil, err
		}

		colExpr := fmt.Sprintf("toString(%s) AS %s", expr, querybuilder.QuoteIdent(gb.Name))
		allGroupByArgs = append(allGroupByArgs, args...)
		sb.SelectMore(colExpr)
		fieldNames = append(fieldNames, querybuilder.QuoteIdent(gb.Name))
	}

	// Aggregations
	allAggChArgs := make([]any, 0)
	for i, agg := range query.Aggregations {
		rewritten, chArgs, err := b.aggExprRewriter.Rewrite(
			ctx, start, end, agg.Expression,
			uint64(query.StepInterval.Seconds()),
			keys,
		)
		if err != nil {
			return nil, err
		}
		allAggChArgs = append(allAggChArgs, chArgs...)
		sb.SelectMore(fmt.Sprintf("%s AS __result_%d", rewritten, i))
	}

	// Add FROM clause
	sb.From(fmt.Sprintf("%s.%s", DBName, LogTableName))

	preparedWhereClause, err := b.addFilterCondition(ctx, sb, start, end, query, keys, variables, skipResourceFilter)

	if err != nil {
		return nil, err
	}

	var finalSQL string
	var finalArgs []any

	if query.Limit > 0 && len(query.GroupBy) > 0 {
		// build the scalar “top/bottom-N” query in its own builder.
		cteSB := sqlbuilder.NewSelectBuilder()
		cteStmt, err := b.buildScalarQuery(ctx, cteSB, query, start, end, keys, true, variables)
		if err != nil {
			return nil, err
		}

		cteFragments = append(cteFragments, fmt.Sprintf("__limit_cte AS (%s)", cteStmt.Query))
		cteArgs = append(cteArgs, cteStmt.Args)

		// Constrain the main query to the rows that appear in the CTE.
		tuple := fmt.Sprintf("(%s)", strings.Join(fieldNames, ", "))
		sb.Where(fmt.Sprintf("%s GLOBAL IN (SELECT %s FROM __limit_cte)", tuple, strings.Join(fieldNames, ", ")))

		// Group by all dimensions
		sb.GroupBy("ts")
		sb.GroupBy(querybuilder.GroupByKeys(query.GroupBy)...)
		if query.Having != nil && query.Having.Expression != "" {
			// Rewrite having expression to use SQL column names
			rewriter := querybuilder.NewHavingExpressionRewriter()
			rewrittenExpr, err := rewriter.RewriteForLogs(query.Having.Expression, query.Aggregations)
			if err != nil {
				return nil, err
			}
			sb.Having(rewrittenExpr)
		}

		if len(query.Order) != 0 {
			for _, orderBy := range query.Order {
				_, ok := aggOrderBy(orderBy, query)
				if !ok {
					sb.OrderBy(fmt.Sprintf("%s %s", querybuilder.QuoteIdent(orderBy.Key.Name), orderBy.Direction.StringValue()))
				}
			}
			sb.OrderBy("ts desc")
		}

		combinedArgs := append(allGroupByArgs, allAggChArgs...)
		mainSQL, mainArgs := sb.BuildWithFlavor(datastoresql.Flavor, combinedArgs...)

		// Stitch it all together:  WITH … SELECT …
		finalSQL = querybuilder.CombineCTEs(cteFragments) + mainSQL
		finalArgs = querybuilder.PrependArgs(cteArgs, mainArgs)

	} else {
		sb.GroupBy("ts")
		sb.GroupBy(querybuilder.GroupByKeys(query.GroupBy)...)
		if query.Having != nil && query.Having.Expression != "" {
			rewriter := querybuilder.NewHavingExpressionRewriter()
			rewrittenExpr, err := rewriter.RewriteForLogs(query.Having.Expression, query.Aggregations)
			if err != nil {
				return nil, err
			}
			sb.Having(rewrittenExpr)
		}

		if len(query.Order) != 0 {
			for _, orderBy := range query.Order {
				_, ok := aggOrderBy(orderBy, query)
				if !ok {
					sb.OrderBy(fmt.Sprintf("%s %s", querybuilder.QuoteIdent(orderBy.Key.Name), orderBy.Direction.StringValue()))
				}
			}
			sb.OrderBy("ts desc")
		}

		combinedArgs := append(allGroupByArgs, allAggChArgs...)
		mainSQL, mainArgs := sb.BuildWithFlavor(datastoresql.Flavor, combinedArgs...)

		// Stitch it all together:  WITH … SELECT …
		finalSQL = querybuilder.CombineCTEs(cteFragments) + mainSQL
		finalArgs = querybuilder.PrependArgs(cteArgs, mainArgs)
	}

	stmt := &qbtypes.Statement{
		Query:          finalSQL,
		Args:           finalArgs,
		Warnings:       preparedWhereClause.Warnings,
		WarningsDocURL: preparedWhereClause.WarningsDocURL,
	}

	return stmt, nil
}

// buildScalarQuery builds a query for scalar panel type.
func (b *logQueryStatementBuilder) buildScalarQuery(
	ctx context.Context,
	sb *sqlbuilder.SelectBuilder,
	query qbtypes.QueryBuilderQuery[qbtypes.LogAggregation],
	start, end uint64,
	keys map[string][]*telemetrytypes.TelemetryFieldKey,
	skipResourceCTE bool,
	variables map[string]qbtypes.VariableItem,
) (*qbtypes.Statement, error) {

	var (
		cteFragments []string
		cteArgs      [][]any
		// TODO(Tushar): thread orgID here to evaluate correctly
		bodyJSONEnabled = b.fl.BooleanOrEmpty(ctx, flagger.FeatureUseJSONBody, featuretypes.NewFlaggerEvaluationContext(valuer.UUID{}))
	)

	frag, args, skipResourceFilter, err := b.maybeAttachResourceFilter(ctx, sb, query, start, end, variables)
	if err != nil {
		return nil, err
	}
	if frag != "" && !skipResourceCTE {
		cteFragments = append(cteFragments, frag)
		cteArgs = append(cteArgs, args)
	}

	allAggChArgs := []any{}

	var allGroupByArgs []any

	for _, gb := range query.GroupBy {
		expr, args, err := querybuilder.CollisionHandledFinalExpr(ctx, start, end, &gb.TelemetryFieldKey, b.fm, b.cb, keys, telemetrytypes.FieldDataTypeString, b.jsonKeyToKey, bodyJSONEnabled)
		if err != nil {
			return nil, err
		}

		colExpr := fmt.Sprintf("toString(%s) AS %s", expr, querybuilder.QuoteIdent(gb.Name))
		allGroupByArgs = append(allGroupByArgs, args...)
		sb.SelectMore(colExpr)
	}

	// for scalar queries, the rate would be end-start
	rateInterval := (end - start) / querybuilder.NsToSeconds

	// Add aggregation
	if len(query.Aggregations) > 0 {
		for idx := range query.Aggregations {
			aggExpr := query.Aggregations[idx]
			rewritten, chArgs, err := b.aggExprRewriter.Rewrite(
				ctx, start, end, aggExpr.Expression,
				rateInterval,
				keys,
			)
			if err != nil {
				return nil, err
			}
			allAggChArgs = append(allAggChArgs, chArgs...)
			sb.SelectMore(fmt.Sprintf("%s AS __result_%d", rewritten, idx))
		}
	}

	sb.From(fmt.Sprintf("%s.%s", DBName, LogTableName))

	// Add filter conditions
	preparedWhereClause, err := b.addFilterCondition(ctx, sb, start, end, query, keys, variables, skipResourceFilter)

	if err != nil {
		return nil, err
	}

	// Group by dimensions
	sb.GroupBy(querybuilder.GroupByKeys(query.GroupBy)...)

	// Add having clause if needed
	if query.Having != nil && query.Having.Expression != "" {
		rewriter := querybuilder.NewHavingExpressionRewriter()
		rewrittenExpr, err := rewriter.RewriteForLogs(query.Having.Expression, query.Aggregations)
		if err != nil {
			return nil, err
		}
		sb.Having(rewrittenExpr)
	}

	// Add order by
	for _, orderBy := range query.Order {
		idx, ok := aggOrderBy(orderBy, query)
		if ok {
			sb.OrderBy(fmt.Sprintf("__result_%d %s", idx, orderBy.Direction.StringValue()))
		} else {
			sb.OrderBy(fmt.Sprintf("%s %s", querybuilder.QuoteIdent(orderBy.Key.Name), orderBy.Direction.StringValue()))
		}
	}

	// if there is no order by, then use the __result_0 as the order by
	if len(query.Order) == 0 {
		sb.OrderBy("__result_0 DESC")
	}

	// Add limit and offset
	if query.Limit > 0 {
		sb.Limit(query.Limit)
	}

	combinedArgs := append(allGroupByArgs, allAggChArgs...)

	mainSQL, mainArgs := sb.BuildWithFlavor(datastoresql.Flavor, combinedArgs...)

	finalSQL := querybuilder.CombineCTEs(cteFragments) + mainSQL
	finalArgs := querybuilder.PrependArgs(cteArgs, mainArgs)

	stmt := &qbtypes.Statement{
		Query:          finalSQL,
		Args:           finalArgs,
		Warnings:       preparedWhereClause.Warnings,
		WarningsDocURL: preparedWhereClause.WarningsDocURL,
	}

	return stmt, nil
}

// buildFilterCondition builds SQL condition from filter expression.
func (b *logQueryStatementBuilder) addFilterCondition(
	ctx context.Context,
	sb *sqlbuilder.SelectBuilder,
	start, end uint64,
	query qbtypes.QueryBuilderQuery[qbtypes.LogAggregation],
	keys map[string][]*telemetrytypes.TelemetryFieldKey,
	variables map[string]qbtypes.VariableItem,
	skipResourceFilter bool,
) (querybuilder.PreparedWhereClause, error) {

	var preparedWhereClause querybuilder.PreparedWhereClause
	var err error
	// TODO(Tushar): thread orgID here to evaluate correctly
	bodyJSONEnabled := b.fl.BooleanOrEmpty(ctx, flagger.FeatureUseJSONBody, featuretypes.NewFlaggerEvaluationContext(valuer.UUID{}))

	if query.Filter != nil && query.Filter.Expression != "" {
		// add filter expression
		preparedWhereClause, err = querybuilder.PrepareWhereClause(query.Filter.Expression, querybuilder.FilterExprVisitorOpts{
			Context:            ctx,
			Logger:             b.logger,
			FieldMapper:        b.fm,
			ConditionBuilder:   b.cb,
			FieldKeys:          keys,
			BodyJSONEnabled:    bodyJSONEnabled,
			SkipResourceFilter: skipResourceFilter,
			FullTextColumn:     b.fullTextColumn,
			JsonKeyToKey:       b.jsonKeyToKey,
			Variables:          variables,
			StartNs:            start,
			EndNs:              end,
		})

		if err != nil {
			return preparedWhereClause, err
		}
	}

	if !preparedWhereClause.IsEmpty() {
		sb.AddWhereClause(preparedWhereClause.WhereClause)
	}

	// add time filter
	startBucket := start/querybuilder.NsToSeconds - querybuilder.BucketAdjustment
	var endBucket uint64
	if end != 0 {
		endBucket = end / querybuilder.NsToSeconds
	}

	// The range predicate is on the bare `time` column, not on a function of it:
	// time is DateTime64(9) and the bound is its nanosecond tick count, so the
	// comparison is exact AND stays a sort-key/minmax prefix seek. Wrapping time
	// in toUnixTimestamp64Nano would give the same answer and scan the table.
	if start != 0 {
		sb.Where(sb.GE("time", fmt.Sprintf("%d", start)), sb.GE("ts_bucket_start", startBucket))
	}
	if end != 0 {
		sb.Where(sb.L("time", fmt.Sprintf("%d", end)), sb.LE("ts_bucket_start", endBucket))
	}

	return preparedWhereClause, nil
}

func aggOrderBy(k qbtypes.OrderBy, q qbtypes.QueryBuilderQuery[qbtypes.LogAggregation]) (int, bool) {
	for i, agg := range q.Aggregations {
		if k.Key.Name == agg.Alias ||
			k.Key.Name == agg.Expression ||
			k.Key.Name == fmt.Sprintf("%d", i) {
			return i, true
		}
	}
	return 0, false
}

func (b *logQueryStatementBuilder) maybeAttachResourceFilter(
	ctx context.Context,
	sb *sqlbuilder.SelectBuilder,
	query qbtypes.QueryBuilderQuery[qbtypes.LogAggregation],
	start, end uint64,
	variables map[string]qbtypes.VariableItem,
) (cteSQL string, cteArgs []any, skipResourceFilter bool, err error) {

	if b.skipResourceFingerprintEnabled {
		decision, err := b.resourceFilterResolver.Resolve(ctx, query, start, end, variables)
		if err != nil {
			return "", nil, true, err
		}
		switch decision {
		case qbtypes.ResourceFilterResolveKindNoOp:
			return "", nil, true, nil
		case qbtypes.ResourceFilterResolveKindFallback:
			return "", nil, false, nil
		}
	}

	stmt, err := b.resourceFilterResolver.StatementBuilder().Build(
		ctx, start, end, qbtypes.RequestTypeRaw, query, variables,
	)
	if err != nil {
		return "", nil, true, err
	}
	if stmt == nil {
		return "", nil, true, nil
	}
	sb.Where("resource_fingerprint GLOBAL IN (SELECT fingerprint FROM __resource_filter)")
	return fmt.Sprintf("__resource_filter AS (%s)", stmt.Query), stmt.Args, true, nil
}
