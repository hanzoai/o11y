package binding

import (
	"fmt"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestQueryBinding_BindQuery(t *testing.T) {
	one := 1
	zero := 0
	testCases := []struct {
		name     string
		query    map[string][]string
		obj      any
		expected any
	}{
		{name: "SingleIntField_NonEmptyValue", query: map[string][]string{"a": {"1"}}, obj: &struct {
			A int `query:"a"`
		}{}, expected: &struct{ A int }{A: 1}},
		{name: "SingleIntField_EmptyValue", query: map[string][]string{"a": {""}}, obj: &struct {
			A int `query:"a"`
		}{}, expected: &struct{ A int }{A: 0}},
		{name: "SingleIntField_MissingField", query: map[string][]string{}, obj: &struct {
			A int `query:"a"`
		}{}, expected: &struct{ A int }{A: 0}},
		{name: "SinglePointerIntField_NonEmptyValue", query: map[string][]string{"a": {"1"}}, obj: &struct {
			A *int `query:"a"`
		}{}, expected: &struct{ A *int }{A: &one}},
		{name: "SinglePointerIntField_EmptyValue", query: map[string][]string{"a": {""}}, obj: &struct {
			A *int `query:"a"`
		}{}, expected: &struct{ A *int }{A: &zero}},
		{name: "SinglePointerIntField_MissingField", query: map[string][]string{}, obj: &struct {
			A *int `query:"a"`
		}{}, expected: &struct{ A *int }{A: nil}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := Query.BindQuery(testCase.query, testCase.obj)
			assert.NoError(t, err)
			assert.EqualValues(t, testCase.expected, testCase.obj)
		})
	}
}

// param stands in for a valuer: a named type that decodes itself from one raw
// query value. Every enum in the codebase binds through this path.
type param struct{ decoded string }

func (p *param) UnmarshalParam(raw string) error {
	if raw == "boom" {
		return fmt.Errorf("param: refusing %q", raw)
	}

	p.decoded = "<" + raw + ">"
	return nil
}

type embedded struct {
	Inner    string `query:"inner"`
	InnerPtr *int64 `query:"innerPtr"`
}

// kinds carries one field of every kind bound anywhere in this repo, plus the
// widths a future handler may reach for.
type kinds struct {
	String      string   `query:"string"`
	Bool        bool     `query:"bool"`
	Int         int      `query:"int"`
	Int8        int8     `query:"int8"`
	Int16       int16    `query:"int16"`
	Int32       int32    `query:"int32"`
	Int64       int64    `query:"int64"`
	Uint        uint     `query:"uint"`
	Uint8       uint8    `query:"uint8"`
	Uint16      uint16   `query:"uint16"`
	Uint32      uint32   `query:"uint32"`
	Uint64      uint64   `query:"uint64"`
	Float32     float32  `query:"float32"`
	Float64     float64  `query:"float64"`
	Strings     []string `query:"strings"`
	Ints        []int    `query:"ints"`
	PtrInt      *int     `query:"ptrInt"`
	PtrBool     *bool    `query:"ptrBool"`
	PtrString   *string  `query:"ptrString"`
	Custom      param    `query:"custom"`
	PtrCustom   *param   `query:"ptrCustom"`
	CustomSlice []param  `query:"customSlice"`
	Required    string   `query:"required" required:"true"`
	Skipped     string   `query:"-"`
	Untagged    string
	hidden      string `query:"hidden"`
	embedded
}

func TestQueryBinding_Kinds(t *testing.T) {
	ptr := func(i int) *int { return &i }
	yes := true
	empty := ""
	seven := int64(7)

	testCases := []struct {
		name     string
		query    map[string][]string
		expected kinds
	}{
		{name: "String", query: map[string][]string{"string": {" keeps spaces "}}, expected: kinds{String: " keeps spaces "}},
		{name: "Bool", query: map[string][]string{"bool": {"true"}}, expected: kinds{Bool: true}},
		{name: "BoolOne", query: map[string][]string{"bool": {"1"}}, expected: kinds{Bool: true}},
		{name: "Int", query: map[string][]string{"int": {"42"}}, expected: kinds{Int: 42}},
		{name: "IntNegative", query: map[string][]string{"int": {"-42"}}, expected: kinds{Int: -42}},
		{name: "IntTrimsSpace", query: map[string][]string{"int": {"  42  "}}, expected: kinds{Int: 42}},
		{name: "Int8", query: map[string][]string{"int8": {"127"}}, expected: kinds{Int8: 127}},
		{name: "Int16", query: map[string][]string{"int16": {"32767"}}, expected: kinds{Int16: 32767}},
		{name: "Int32", query: map[string][]string{"int32": {"2147483647"}}, expected: kinds{Int32: 2147483647}},
		{name: "Int64", query: map[string][]string{"int64": {"9223372036854775807"}}, expected: kinds{Int64: 9223372036854775807}},
		{name: "Uint", query: map[string][]string{"uint": {"42"}}, expected: kinds{Uint: 42}},
		{name: "Uint8", query: map[string][]string{"uint8": {"255"}}, expected: kinds{Uint8: 255}},
		{name: "Uint16", query: map[string][]string{"uint16": {"65535"}}, expected: kinds{Uint16: 65535}},
		{name: "Uint32", query: map[string][]string{"uint32": {"4294967295"}}, expected: kinds{Uint32: 4294967295}},
		{name: "Uint64", query: map[string][]string{"uint64": {"18446744073709551615"}}, expected: kinds{Uint64: 18446744073709551615}},
		{name: "Float32", query: map[string][]string{"float32": {"1.5"}}, expected: kinds{Float32: 1.5}},
		{name: "Float64", query: map[string][]string{"float64": {"-0.25"}}, expected: kinds{Float64: -0.25}},
		{name: "EmptyValueIsZero", query: map[string][]string{"int": {""}, "bool": {""}, "float64": {""}, "uint": {""}}, expected: kinds{}},
		{name: "EmptyValueListIsZero", query: map[string][]string{"int": {}}, expected: kinds{}},
		{name: "ScalarTakesFirstValue", query: map[string][]string{"int": {"1", "2"}}, expected: kinds{Int: 1}},
		{name: "SliceOfOne", query: map[string][]string{"strings": {"a"}}, expected: kinds{Strings: []string{"a"}}},
		{name: "SliceOfMany", query: map[string][]string{"strings": {"a", "b"}}, expected: kinds{Strings: []string{"a", "b"}}},
		{name: "SliceIsNotSplitOnComma", query: map[string][]string{"strings": {"a,b"}}, expected: kinds{Strings: []string{"a,b"}}},
		{name: "SliceOfInts", query: map[string][]string{"ints": {"1", "2"}}, expected: kinds{Ints: []int{1, 2}}},
		{name: "SliceEmptyValueList", query: map[string][]string{"strings": {}}, expected: kinds{}},
		{name: "SliceOfEmptyValue", query: map[string][]string{"strings": {""}}, expected: kinds{Strings: []string{""}}},
		{name: "PointerSet", query: map[string][]string{"ptrInt": {"9"}, "ptrBool": {"true"}}, expected: kinds{PtrInt: ptr(9), PtrBool: &yes}},
		{name: "PointerEmptyValueAllocatesZero", query: map[string][]string{"ptrInt": {""}, "ptrString": {""}}, expected: kinds{PtrInt: ptr(0), PtrString: &empty}},
		{name: "PointerMissingStaysNil", query: map[string][]string{}, expected: kinds{}},
		{name: "Custom", query: map[string][]string{"custom": {"abc"}}, expected: kinds{Custom: param{decoded: "<abc>"}}},
		{name: "CustomEmptyValue", query: map[string][]string{"custom": {""}}, expected: kinds{Custom: param{decoded: "<>"}}},
		{name: "CustomMissingIsUntouched", query: map[string][]string{}, expected: kinds{}},
		{name: "CustomPointer", query: map[string][]string{"ptrCustom": {"abc"}}, expected: kinds{PtrCustom: &param{decoded: "<abc>"}}},
		{name: "CustomSlice", query: map[string][]string{"customSlice": {"a", "b"}}, expected: kinds{CustomSlice: []param{{decoded: "<a>"}, {decoded: "<b>"}}}},
		{name: "Embedded", query: map[string][]string{"inner": {"deep"}, "innerPtr": {"7"}}, expected: kinds{embedded: embedded{Inner: "deep", InnerPtr: &seven}}},
		{name: "DashTagIsSkipped", query: map[string][]string{"-": {"x"}, "Skipped": {"x"}}, expected: kinds{}},
		{name: "UntaggedFieldUsesFieldName", query: map[string][]string{"Untagged": {"x"}}, expected: kinds{Untagged: "x"}},
		{name: "UnexportedFieldIsSkipped", query: map[string][]string{"hidden": {"x"}}, expected: kinds{}},
		{name: "UnknownKeyIsIgnored", query: map[string][]string{"nope": {"x"}}, expected: kinds{}},
		// `required` is openapi metadata; the binder never reads it, so an empty
		// value on a required field is the zero value and not an error.
		{name: "RequiredEmptyValueIsNotAnError", query: map[string][]string{"required": {""}}, expected: kinds{}},
		{name: "RequiredMissingIsNotAnError", query: map[string][]string{}, expected: kinds{}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := kinds{}
			assert.NoError(t, Query.BindQuery(testCase.query, &actual))
			assert.Equal(t, testCase.expected, actual)
		})
	}
}

type defaulted struct {
	String string   `query:"string,default=fallback"`
	Int    int      `query:"int,default=10"`
	Custom param    `query:"custom,default=seed"`
	Slice  []string `query:"slice,default=one"`
	Unused string   `query:"unused,other=ignored"`
}

func TestQueryBinding_Defaults(t *testing.T) {
	testCases := []struct {
		name     string
		query    map[string][]string
		expected defaulted
	}{
		{name: "MissingKeysTakeDefaults", query: map[string][]string{}, expected: defaulted{String: "fallback", Int: 10, Custom: param{decoded: "<seed>"}, Slice: []string{"one"}}},
		{name: "EmptyValueTakesDefault", query: map[string][]string{"string": {""}, "int": {""}, "custom": {""}}, expected: defaulted{String: "fallback", Int: 10, Custom: param{decoded: "<seed>"}, Slice: []string{"one"}}},
		{name: "SuppliedValueWins", query: map[string][]string{"string": {"given"}, "int": {"3"}, "custom": {"given"}, "slice": {"a", "b"}}, expected: defaulted{String: "given", Int: 3, Custom: param{decoded: "<given>"}, Slice: []string{"a", "b"}}},
		{name: "UnknownTagOptionIsIgnored", query: map[string][]string{"unused": {"x"}}, expected: defaulted{String: "fallback", Int: 10, Custom: param{decoded: "<seed>"}, Slice: []string{"one"}, Unused: "x"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := defaulted{}
			assert.NoError(t, Query.BindQuery(testCase.query, &actual))
			assert.Equal(t, testCase.expected, actual)
		})
	}
}

type unsupported struct {
	Time time.Time `query:"time"`
}

func TestQueryBinding_BindQueryErrors(t *testing.T) {
	testCases := []struct {
		name       string
		query      map[string][]string
		obj        any
		additional string
	}{
		{name: "UnparseableInt", query: map[string][]string{"int": {"abc"}}, obj: &kinds{}, additional: `strconv.ParseInt: parsing "abc": invalid syntax`},
		{name: "IntOverflowsWidth", query: map[string][]string{"int8": {"128"}}, obj: &kinds{}, additional: `strconv.ParseInt: parsing "128": value out of range`},
		{name: "NegativeUint", query: map[string][]string{"uint": {"-1"}}, obj: &kinds{}, additional: `strconv.ParseUint: parsing "-1": invalid syntax`},
		{name: "UnparseableBool", query: map[string][]string{"bool": {"yes please"}}, obj: &kinds{}, additional: `strconv.ParseBool: parsing "yes please": invalid syntax`},
		{name: "UnparseableFloat", query: map[string][]string{"float64": {"abc"}}, obj: &kinds{}, additional: `strconv.ParseFloat: parsing "abc": invalid syntax`},
		{name: "UnparseablePointer", query: map[string][]string{"ptrInt": {"abc"}}, obj: &kinds{}, additional: `strconv.ParseInt: parsing "abc": invalid syntax`},
		{name: "UnparseableSliceElement", query: map[string][]string{"ints": {"1", "x"}}, obj: &kinds{}, additional: `strconv.ParseInt: parsing "x": invalid syntax`},
		{name: "UnparseableEmbeddedField", query: map[string][]string{"innerPtr": {"abc"}}, obj: &kinds{}, additional: `strconv.ParseInt: parsing "abc": invalid syntax`},
		{name: "CustomUnmarshalerRejects", query: map[string][]string{"custom": {"boom"}}, obj: &kinds{}, additional: `param: refusing "boom"`},
		{name: "UnsupportedKind", query: map[string][]string{"time": {"now"}}, obj: &unsupported{}, additional: `cannot bind "now" into time.Time`},
		{name: "NonPointerTarget", query: map[string][]string{}, obj: kinds{}, additional: "binding target must be a non-nil pointer, got binding.kinds"},
		{name: "NilPointerTarget", query: map[string][]string{}, obj: (*kinds)(nil), additional: "binding target must be a non-nil pointer, got *binding.kinds"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := Query.BindQuery(testCase.query, testCase.obj)
			assert.Error(t, err)

			typ, code, message, _, _, _ := errors.Unwrapb(err)
			assert.Equal(t, errors.TypeInvalidInput, typ)
			assert.Equal(t, ErrCodeInvalidRequestQuery, code)
			assert.Equal(t, ErrMessageInvalidQuery, message)

			messages := []string{}
			for _, additional := range errors.AsJSON(err).Errors {
				messages = append(messages, additional.Message)
			}
			assert.Equal(t, []string{testCase.additional}, messages)
		})
	}
}

// A field that fails to parse must not leave earlier fields half-written under
// a returned error: binding is all-or-nothing from the caller's point of view.
func TestQueryBinding_BindQueryStopsAtFirstError(t *testing.T) {
	actual := kinds{}
	assert.Error(t, Query.BindQuery(map[string][]string{"int8": {"128"}}, &actual))
	assert.Equal(t, 0, int(actual.Int8))
}
