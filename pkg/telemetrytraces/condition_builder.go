package telemetrytraces

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hanzo-ds/sqlbuilder"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/querybuilder"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	schema "github.com/hanzoai/otel-collector/cmd/o11yschemamigrator/schema_migrator"
	"golang.org/x/exp/maps"
)

type conditionBuilder struct {
	fm qbtypes.FieldMapper
}

var _ qbtypes.ConditionBuilder = (*conditionBuilder)(nil)

func NewConditionBuilder(fm qbtypes.FieldMapper) *conditionBuilder {
	return &conditionBuilder{fm: fm}
}

func (c *conditionBuilder) conditionFor(
	ctx context.Context,
	startNs uint64,
	endNs uint64,
	key *telemetrytypes.TelemetryFieldKey,
	operator qbtypes.FilterOperator,
	value any,
	sb *sqlbuilder.SelectBuilder,
) (string, error) {

	if operator.IsStringSearchOperator() {
		value = querybuilder.FormatValueForContains(value)
	}

	// first, locate the raw column type (so we can choose the right EXISTS logic)
	columns, err := c.fm.ColumnFor(ctx, startNs, endNs, key)
	if err != nil {
		return "", err
	}

	// then ask the mapper for the actual SQL reference
	fieldExpression, err := c.fm.FieldFor(ctx, startNs, endNs, key)
	if err != nil {
		return "", err
	}

	// TODO(srikanthccv): maybe extend this to every possible attribute
	if key.Name == "duration_nano" || key.Name == "durationNano" { // QoL improvement
		if strDuration, ok := value.(string); ok {
			duration, err := time.ParseDuration(strDuration)
			if err == nil {
				value = duration.Nanoseconds()
			} else {
				duration, err := strconv.ParseFloat(strDuration, 64)
				if err == nil {
					value = duration
				} else {
					return "", errors.WrapInvalidInputf(err, errors.CodeInvalidInput, "invalid duration value: %s", strDuration)
				}
			}
		}
	} else {
		fieldExpression, value = querybuilder.DataTypeCollisionHandledFieldName(key, value, fieldExpression, operator)
	}

	// regular operators
	switch operator {
	// regular operators
	case qbtypes.FilterOperatorEqual:
		return sb.E(fieldExpression, value), nil
	case qbtypes.FilterOperatorNotEqual:
		return sb.NE(fieldExpression, value), nil
	case qbtypes.FilterOperatorGreaterThan:
		return sb.G(fieldExpression, value), nil
	case qbtypes.FilterOperatorGreaterThanOrEq:
		return sb.GE(fieldExpression, value), nil
	case qbtypes.FilterOperatorLessThan:
		return sb.LT(fieldExpression, value), nil
	case qbtypes.FilterOperatorLessThanOrEq:
		return sb.LE(fieldExpression, value), nil

	// like and not like
	case qbtypes.FilterOperatorLike:
		return sb.Like(fieldExpression, value), nil
	case qbtypes.FilterOperatorNotLike:
		return sb.NotLike(fieldExpression, value), nil
	case qbtypes.FilterOperatorILike:
		return sb.ILike(fieldExpression, value), nil
	case qbtypes.FilterOperatorNotILike:
		return sb.NotILike(fieldExpression, value), nil

	case qbtypes.FilterOperatorContains:
		return sb.ILike(fieldExpression, fmt.Sprintf("%%%s%%", value)), nil
	case qbtypes.FilterOperatorNotContains:
		return sb.NotILike(fieldExpression, fmt.Sprintf("%%%s%%", value)), nil

	case qbtypes.FilterOperatorRegexp:
		// Note: Escape $$ to $$$$ to avoid sqlbuilder interpreting materialized $ signs
		// Only needed because we are using sprintf instead of sb.Match (not implemented in sqlbuilder)
		return fmt.Sprintf(`match(%s, %s)`, sqlbuilder.Escape(fieldExpression), sb.Var(value)), nil
	case qbtypes.FilterOperatorNotRegexp:
		// Note: Escape $$ to $$$$ to avoid sqlbuilder interpreting materialized $ signs
		// Only needed because we are using sprintf instead of sb.Match (not implemented in sqlbuilder)
		return fmt.Sprintf(`NOT match(%s, %s)`, sqlbuilder.Escape(fieldExpression), sb.Var(value)), nil
	// between and not between
	case qbtypes.FilterOperatorBetween:
		values, ok := value.([]any)
		if !ok {
			return "", qbtypes.ErrBetweenValues
		}
		if len(values) != 2 {
			return "", qbtypes.ErrBetweenValues
		}
		return sb.Between(fieldExpression, values[0], values[1]), nil
	case qbtypes.FilterOperatorNotBetween:
		values, ok := value.([]any)
		if !ok {
			return "", qbtypes.ErrBetweenValues
		}
		if len(values) != 2 {
			return "", qbtypes.ErrBetweenValues
		}
		return sb.NotBetween(fieldExpression, values[0], values[1]), nil

	// in and not in
	case qbtypes.FilterOperatorIn:
		values, ok := value.([]any)
		if !ok {
			return "", qbtypes.ErrInValues
		}
		// instead of using IN, we use `=` + `OR` to make use of index
		conditions := []string{}
		for _, value := range values {
			conditions = append(conditions, sb.E(fieldExpression, value))
		}
		return sb.Or(conditions...), nil
	case qbtypes.FilterOperatorNotIn:
		values, ok := value.([]any)
		if !ok {
			return "", qbtypes.ErrInValues
		}
		// instead of using NOT IN, we use `!=` + `AND` to make use of index
		conditions := []string{}
		for _, value := range values {
			conditions = append(conditions, sb.NE(fieldExpression, value))
		}
		return sb.And(conditions...), nil

	// exists and not exists
	// in the query builder, `exists` and `not exists` are used for
	// key membership checks, so depending on the column type, the condition changes
	case qbtypes.FilterOperatorExists, qbtypes.FilterOperatorNotExists:

		var value any
		column := columns[0]
		// An evolution selects BETWEEN columns; the envelope resolves every key to
		// exactly one column, so there is nothing to select and the evolution
		// metadata (which names OTLP-fork columns) does not apply.
		if len(key.Evolutions) > 0 && len(columns) > 1 {
			// we will use the corresponding column and its evolution entry for the query
			newColumns, _, err := qbtypes.SelectEvolutionsForColumns(columns, key.Evolutions, startNs, endNs)
			if err != nil {
				return "", err
			}

			if len(newColumns) == 0 {
				return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "no valid evolution found for field %s in the given time range", key.Name)
			}

			// Multiple columns means fieldExpression is a multiIf returning NULL when none match,
			// so a simple null check is sufficient.
			if len(newColumns) > 1 {
				if operator == qbtypes.FilterOperatorExists {
					return sb.IsNotNull(fieldExpression), nil
				} else {
					return sb.IsNull(fieldExpression), nil
				}
			}

			// otherwise we have to find the correct exist operator based on the column type
			column = newColumns[0]
		}

		// A resource label the envelope PROMOTED to a column of its own is always
		// structurally present, so its existence IS the column being non-empty —
		// and that needs no bound parameter, unlike a map-membership test.
		if key.FieldContext == telemetrytypes.FieldContextResource {
			if _, promoted := PromotedResourceColumn(key.Name); promoted {
				if operator == qbtypes.FilterOperatorExists {
					return fmt.Sprintf("notEmpty(%s)", fieldExpression), nil
				}
				return fmt.Sprintf("empty(%s)", fieldExpression), nil
			}
		}

		switch column.Type.GetType() {
		case schema.ColumnTypeEnumJSON:
			if operator == qbtypes.FilterOperatorExists {
				return sb.IsNotNull(fieldExpression), nil
			} else {
				return sb.IsNull(fieldExpression), nil
			}
		case schema.ColumnTypeEnumString,
			schema.ColumnTypeEnumFixedString,
			schema.ColumnTypeEnumDateTime64:
			value = ""
			if operator == qbtypes.FilterOperatorExists {
				return sb.NE(fieldExpression, value), nil
			} else {
				return sb.E(fieldExpression, value), nil
			}
		case schema.ColumnTypeEnumLowCardinality:
			switch elementType := column.Type.(schema.LowCardinalityColumnType).ElementType; elementType.GetType() {
			case schema.ColumnTypeEnumString:
				value = ""
				if operator == qbtypes.FilterOperatorExists {
					return sb.NE(fieldExpression, value), nil
				}
				return sb.E(fieldExpression, value), nil
			default:
				return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "exists operator is not supported for low cardinality column type %s", elementType)
			}

		case schema.ColumnTypeEnumUInt64,
			schema.ColumnTypeEnumUInt32,
			schema.ColumnTypeEnumUInt8,
			schema.ColumnTypeEnumInt8,
			schema.ColumnTypeEnumInt16,
			schema.ColumnTypeEnumBool:
			value = 0
			if operator == qbtypes.FilterOperatorExists {
				return sb.NE(fieldExpression, value), nil
			} else {
				return sb.E(fieldExpression, value), nil
			}
		case schema.ColumnTypeEnumMap:
			keyType := column.Type.(schema.MapColumnType).KeyType
			if _, ok := keyType.(schema.LowCardinalityColumnType); !ok {
				return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "key type %s is not supported for map column type %s", keyType, column.Type)
			}

			switch valueType := column.Type.(schema.MapColumnType).ValueType; valueType.GetType() {
			case schema.ColumnTypeEnumString, schema.ColumnTypeEnumBool, schema.ColumnTypeEnumFloat64:
				// The envelope has one attributes map and no materialized columns, so
				// membership is always a mapContains over it.
				leftOperand := fmt.Sprintf("mapContains(%s, '%s')", column.Name, querybuilder.EscapeLiteral(key.Name))
				if operator == qbtypes.FilterOperatorExists {
					return sb.E(leftOperand, true), nil
				} else {
					return sb.NE(leftOperand, true), nil
				}
			default:
				return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "exists operator is not supported for map column type %s", valueType)
			}
		default:
			return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "exists operator is not supported for column type %s", column.Type)
		}
	}
	return "", nil
}

func (c *conditionBuilder) ConditionFor(
	ctx context.Context,
	startNs uint64,
	endNs uint64,
	key *telemetrytypes.TelemetryFieldKey,
	operator qbtypes.FilterOperator,
	value any,
	sb *sqlbuilder.SelectBuilder,
) (string, error) {
	if c.isSpanScopeField(key.Name) {
		return c.buildSpanScopeCondition(key, operator, value, startNs)
	}

	condition, err := c.conditionFor(ctx, startNs, endNs, key, operator, value, sb)
	if err != nil {
		return "", err
	}

	if operator.AddDefaultExistsFilter() {
		// skip adding exists filter for intrinsic/calculated fields. Keyed on the
		// logical field name, not the emitted column: envelope columns no longer
		// coincide with the field-map keys the way the OTLP-fork column names did.
		if slices.Contains(maps.Keys(IntrinsicFields), key.Name) ||
			slices.Contains(maps.Keys(IntrinsicFieldsDeprecated), key.Name) ||
			slices.Contains(maps.Keys(CalculatedFields), key.Name) ||
			slices.Contains(maps.Keys(CalculatedFieldsDeprecated), key.Name) {
			return condition, nil
		}

		// A resource label the envelope PROMOTED to a column of its own is always
		// present — there is no map entry that could be missing — so an exists
		// filter over it is a tautology, not a guard.
		if key.FieldContext == telemetrytypes.FieldContextResource {
			if _, promoted := PromotedResourceColumn(key.Name); promoted {
				return condition, nil
			}
		}

		existsCondition, err := c.conditionFor(ctx, startNs, endNs, key, qbtypes.FilterOperatorExists, nil, sb)
		if err != nil {
			return "", err
		}
		return sb.And(condition, existsCondition), nil
	}
	return condition, nil
}

func (c *conditionBuilder) isSpanScopeField(name string) bool {
	keyName := strings.ToLower(name)
	return keyName == SpanSearchScopeRoot || keyName == SpanSearchScopeEntryPoint
}

func (c *conditionBuilder) buildSpanScopeCondition(key *telemetrytypes.TelemetryFieldKey, operator qbtypes.FilterOperator, value any, startNs uint64) (string, error) {
	if operator != qbtypes.FilterOperatorEqual {
		return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "span scope field %s only supports '=' operator", key.Name)
	}

	var isTrue bool
	switch v := value.(type) {
	case bool:
		isTrue = v
	case string:
		isTrue = strings.ToLower(v) == "true"
	default:
		return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "span scope field %s expects boolean value, got %T", key.Name, value)
	}

	if !isTrue {
		return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "span scope field %s can only be filtered with value 'true'", key.Name)
	}

	keyName := strings.ToLower(key.Name)
	switch keyName {
	case SpanSearchScopeRoot:
		return "parent = ''", nil
	case SpanSearchScopeEntryPoint:
		if startNs > 0 { // only add time filter if it is a valid time, else do not add
			startS := int64(startNs / 1_000_000_000)
			return fmt.Sprintf("((name, service) GLOBAL IN (SELECT DISTINCT name, serviceName from %s.%s WHERE time >= toDateTime(%d))) AND parent != ''",
				DBName, OperationTableName, startS), nil
		}
		return fmt.Sprintf("((name, service) GLOBAL IN (SELECT DISTINCT name, serviceName from %s.%s)) AND parent != ''",
			DBName, OperationTableName), nil
	default:
		return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "invalid span search scope: %s", key.Name)
	}
}
