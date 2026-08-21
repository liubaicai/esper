package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const exprCoreExistsCastJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreExistsCastJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreExists.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCast.java",
}

var exprCoreExistsCastJavaRuntimeIDs = []string{
	"java-runtime-d77c035088ce635538c7",
	"java-runtime-4202039fb2f1ea65fd49",
	"java-runtime-8fa7f5076dde9d791d08",
	"java-runtime-afd826c7d955eb538001",
	"java-runtime-2ee2b8ab1bf9bb2c4e90",
	"java-runtime-3fc2cde530f321dc3994",
	"java-runtime-5c9fdd17b5480f9d78e2",
	"java-runtime-52fb8d57dd380f5efe6e",
	"java-runtime-45b9d5a2e216f8063b75",
	"java-runtime-2012a048edc6511a33e0",
	"java-runtime-9957cb6d9cd9ea836d4d",
	"java-runtime-0b91403db9a899efde99",
	"java-runtime-9babbbb6f96faf389bb7",
	"java-runtime-53d0455ef4e377c9c2f9",
	"java-runtime-58852773df609efe7d69",
	"java-runtime-2fcd2094aaf18ca4ce03",
	"java-runtime-f44847213060b8eb3949",
}

var exprCoreExistsCastJavaExecutions = []string{
	"ExprCoreExistsSimple",
	"ExprCoreExistsInner",
	"ExprCoreCastDoubleAndNullOM",
	"ExprCoreCastStringAndNullCompile",
	"ExprCoreCastDates",
	"ExprCoreCastSimple",
	"ExprCoreCastSimpleMoreTypes",
	"ExprCoreCastAsParse",
	"ExprCoreCastDoubleAndNullOM",
	"ExprCoreCastInterface",
	"ExprCoreCastStringAndNullCompile",
	"ExprCoreCastBoolean",
	"ExprCoreCastWStaticType",
	"ExprCoreCastWArray{soda=false}",
	"ExprCoreCastWArray{soda=true}",
	"ExprCoreCastGeneric",
	"ExprCoreCastBigDecimalBigInt",
}

var exprCoreExistsCastCaseOrder = []string{
	"exists-simple",
	"exists-inner",
	"exists-om",
	"exists-compile",
	"cast-simple",
	"cast-simple-more-types",
	"cast-as-parse",
	"cast-double-null-om",
	"cast-interface",
	"cast-string-and-null",
	"cast-boolean",
	"cast-w-static-type",
	"cast-bigdecimal-bigint",
	"cast-warray",
	"cast-warray-soda",
	"cast-generic",
	"cast-dates-base",
	"cast-dates-java8",
	"cast-dates-constant",
}

type exprCoreExistsCastExpectedSend struct {
	eventType string
	payload   map[string]string
}

var exprCoreExistsCastExpectedSends = [][]exprCoreExistsCastExpectedSend{
	{
		{eventType: "SupportBean", payload: map[string]string{
			"theString":    `"abc"`,
			"intPrimitive": "100",
			"intBoxed":     "3",
			"floatBoxed":   "9.5",
		}},
	},
	{
		{eventType: "SupportMarkerInterface", payload: map[string]string{"shape": `"null"`}},
		{eventType: "SupportMarkerInterface", payload: map[string]string{"shape": `"complex"`}},
		{eventType: "SupportMarkerInterface", payload: map[string]string{"shape": `"complex"`}},
		{eventType: "SupportMarkerInterface", payload: map[string]string{"shape": `"nested-support-bean"`}},
		{eventType: "SupportMarkerInterface", payload: map[string]string{"shape": `"support-bean-a"`}},
	},
	{
		{eventType: "SupportMarkerInterface", payload: map[string]string{"shape": `"support-bean"`}},
		{eventType: "SupportMarkerInterface", payload: map[string]string{"shape": `"null"`}},
		{eventType: "SupportMarkerInterface", payload: map[string]string{"shape": `"string"`}},
	},
	{
		{eventType: "SupportMarkerInterface", payload: map[string]string{"shape": `"support-bean"`}},
		{eventType: "SupportMarkerInterface", payload: map[string]string{"shape": `"null"`}},
		{eventType: "SupportMarkerInterface", payload: map[string]string{"shape": `"string"`}},
	},
	{
		{eventType: "SupportBean", payload: map[string]string{
			"theString":    `"abc"`,
			"intPrimitive": "100",
			"intBoxed":     "3",
			"floatBoxed":   "9.5",
		}},
		{eventType: "SupportBean", payload: map[string]string{
			"theString":    "null",
			"intPrimitive": "100",
			"intBoxed":     "null",
			"floatBoxed":   "null",
		}},
	},
	{
		{eventType: "SupportBean", payload: map[string]string{
			"theString":       `"true"`,
			"intPrimitive":    "1",
			"doublePrimitive": "1",
		}},
	},
	{
		{eventType: "SupportBean", payload: map[string]string{
			"theString":    `"12"`,
			"intPrimitive": "1",
		}},
	},
	{
		{eventType: "SupportBeanDynRoot", payload: map[string]string{
			"itemType":  `"int"`,
			"itemValue": "100",
		}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{
			"itemType":  `"byte"`,
			"itemValue": "2",
		}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{
			"itemType":  `"double"`,
			"itemValue": "77.7777",
		}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{
			"itemType":  `"int64"`,
			"itemValue": "6",
		}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{
			"itemType": `"null"`,
		}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{
			"itemType":  `"string"`,
			"itemValue": `"abc"`,
		}},
	},
	{
		{eventType: "SupportBeanDynRoot", payload: map[string]string{"shape": `"dyn-root"`}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{"shape": `"isupportd"`}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{"shape": `"isupportbc"`}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{"shape": `"isupportplus"`}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{"shape": `"isupportbaseab"`}},
	},
	{
		{eventType: "SupportBeanDynRoot", payload: map[string]string{
			"itemType":  `"int"`,
			"itemValue": "100",
		}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{
			"itemType":  `"byte"`,
			"itemValue": "2",
		}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{
			"itemType":  `"double"`,
			"itemValue": "77.7777",
		}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{
			"itemType":  `"int64"`,
			"itemValue": "6",
		}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{
			"itemType": `"null"`,
		}},
		{eventType: "SupportBeanDynRoot", payload: map[string]string{
			"itemType":  `"string"`,
			"itemValue": `"abc"`,
		}},
	},
	{
		{eventType: "SupportBean", payload: map[string]string{
			"theString":     `"abc"`,
			"intPrimitive":  "100",
			"boolPrimitive": "true",
			"boolBoxed":     "true",
		}},
		{eventType: "SupportBean", payload: map[string]string{
			"theString":     "null",
			"intPrimitive":  "100",
			"boolPrimitive": "false",
			"boolBoxed":     "false",
		}},
		{eventType: "SupportBean", payload: map[string]string{
			"theString":     "null",
			"intPrimitive":  "100",
			"boolPrimitive": "true",
			"boolBoxed":     "null",
		}},
	},
	{
		{eventType: "StaticTypeMapEvent", payload: map[string]string{
			"anInt":        `"100"`,
			"anDouble":     `"1.4E-1"`,
			"anLong":       `"-10"`,
			"anFloat":      `"1.001"`,
			"anByte":       `"0x0A"`,
			"anShort":      `"223"`,
			"intPrimitive": "10",
			"intBoxed":     "11",
		}},
	},
	{
		{eventType: "MyEvent", payload: map[string]string{
			"kind":  `"int"`,
			"value": "1",
		}},
		{eventType: "MyEvent", payload: map[string]string{
			"kind":  `"long"`,
			"value": "2",
		}},
		{eventType: "MyEvent", payload: map[string]string{
			"kind":  `"double"`,
			"value": "2.4",
		}},
		{eventType: "MyEvent", payload: map[string]string{
			"kind": `"decimal"`,
			"text": `"156.78"`,
		}},
		{eventType: "MyEvent", payload: map[string]string{
			"kind": `"bigint"`,
			"text": `"200"`,
		}},
		{eventType: "MyEvent", payload: map[string]string{
			"kind":     `"pow2-decimal"`,
			"exponent": "500500",
		}},
		{eventType: "MyEvent", payload: map[string]string{
			"kind":     `"pow2-bigint"`,
			"exponent": "500500",
		}},
		{eventType: "MyEvent", payload: map[string]string{
			"kind": `"null"`,
		}},
	},
	{
		{eventType: "MyEventWArray", payload: map[string]string{
			"shape":              `"full"`,
			"arr_string":         `["a"]`,
			"arr_primitive":      `[1]`,
			"arr_boxed_one":      `[2]`,
			"arr_boxed_two":      `[3]`,
			"arr_object":         `[{"theString":"E1","intPrimitive":0}]`,
			"arr_2dim_primitive": `[[10]]`,
			"arr_2dim_object":    `[[11]]`,
			"arr_3dim_primitive": `[[[12]]]`,
			"arr_3dim_object":    `[[[13]]]`,
		}},
		{eventType: "MyEventWArray", payload: map[string]string{
			"shape": `"empty"`,
		}},
	},
	{
		{eventType: "MyEventWArray", payload: map[string]string{
			"shape":              `"full"`,
			"arr_string":         `["a"]`,
			"arr_primitive":      `[1]`,
			"arr_boxed_one":      `[2]`,
			"arr_boxed_two":      `[3]`,
			"arr_object":         `[{"theString":"E1","intPrimitive":0}]`,
			"arr_2dim_primitive": `[[10]]`,
			"arr_2dim_object":    `[[11]]`,
			"arr_3dim_primitive": `[[[12]]]`,
			"arr_3dim_object":    `[[[13]]]`,
		}},
		{eventType: "MyEventWArray", payload: map[string]string{
			"shape": `"empty"`,
		}},
	},
	{
		{eventType: "MyEventGeneric", payload: map[string]string{
			"shape":                 `"full"`,
			"listOfString":          `["a"]`,
			"listOfOptionalInteger": `[10]`,
			"mapOfStringAndInteger": `{"k":20}`,
			"listArrayOfString":     `[["b"]]`,
			"listOfStringArray":     `[["c"]]`,
			"listArray2DimOfString": `[[["b"]]]`,
			"listOfStringArray2Dim": `[[["c"]]]`,
			"listOfT":               `["x"]`,
		}},
		{eventType: "MyEventGeneric", payload: map[string]string{
			"shape": `"empty"`,
		}},
	},
	{
		{eventType: "MyDateType", payload: map[string]string{
			"yyyymmdd": `"20100510"`,
		}},
	},
	{
		{eventType: "MyDateType", payload: map[string]string{
			"yyyymmdd":       `"20100510"`,
			"yyyymmddhhmmss": `"20100510141516"`,
			"hhmmss":         `"141516"`,
		}},
	},
	{
		{eventType: "SupportBean", payload: map[string]string{
			"theString":    `"E1"`,
			"intPrimitive": `1`,
		}},
	},
}

// exprCoreCastOptional models java.util.Optional for the cast-generic
// differential case: presence is part of the observable value and the token
// renders as Java's own Optional.toString() ("Optional[10]" / "Optional.empty").
type exprCoreCastOptional struct {
	value   any
	present bool
}

// toAnySlice widens a typed slice to []any elementwise so the erased Java
// List casts hit the AssignableTo identity path.
func toAnySlice[T any](values []T) []any {
	widened := make([]any, 0, len(values))
	for _, value := range values {
		widened = append(widened, value)
	}
	return widened
}

type exprCoreExistsCastSupportBean struct {
	TheString     *string  `esper:"theString"`
	IntPrimitive  int      `esper:"intPrimitive"`
	IntBoxed      *int     `esper:"intBoxed"`
	FloatBoxed    *float32 `esper:"floatBoxed"`
	BoolPrimitive bool     `esper:"boolPrimitive"`
	BoolBoxed     *bool    `esper:"boolBoxed"`
}

// exprCoreExistsCastMyArrayEvent models the Java MyArrayEvent bean the
// insert-into statement projects c0..c8 onto. Each field is the array element
// type the corresponding Cast targets, mirroring Java's array component
// classes (String[], int[] primitive, Integer[] boxed as int32[], Object[],
// and the multi-dim variants).
type exprCoreExistsCastMyArrayEvent struct {
	C0 []string  `esper:"c0"`
	C1 []int     `esper:"c1"`
	C2 []int     `esper:"c2"`
	C3 []int     `esper:"c3"`
	C4 []any     `esper:"c4"`
	C5 [][]int   `esper:"c5"`
	C6 [][]int   `esper:"c6"`
	C7 [][][]int `esper:"c7"`
	C8 [][][]int `esper:"c8"`
}

type exprCoreExistsCastNestedNested struct {
	NestedNestedValue string `esper:"nestedNestedValue"`
}

type exprCoreExistsCastNested struct {
	NestedValue  string                          `esper:"nestedValue"`
	NestedNested *exprCoreExistsCastNestedNested `esper:"nestedNested"`
}

type exprCoreExistsCastComplexProps struct {
	Indexed []int             `esper:"indexed"`
	Mapped  map[string]string `esper:"mapped"`

	Nested *exprCoreExistsCastNested `esper:"nested"`
}

type exprCoreExistsCastSupportBeanA struct {
	ID string `esper:"id"`
}

type exprCoreExistsCastDynamicRoot struct {
	Item any `esper:"item"`
}

// exprCoreCastInterfaceMarker is the Go counterpart of the Java
// SupportMarkerInterface empty marker (t0 target). It is implemented only by
// SupportBeanDynRoot wrappers, matching the Java identity semantics where the
// outer event class (not the raw item value) satisfies the marker.
type exprCoreCastInterfaceMarker interface {
	exprCoreCastInterfaceMarker()
}

// exprCoreCastInterfaceBaseAB models Java ISupportBaseAB. ISupportA and
// ISupportB extend it, so any bean satisfying those also satisfies it.
type exprCoreCastInterfaceBaseAB interface {
	exprCoreCastInterfaceBaseAB()
}

// exprCoreCastInterfaceA models Java ISupportA (extends ISupportBaseAB).
type exprCoreCastInterfaceA interface {
	exprCoreCastInterfaceBaseAB
	exprCoreCastInterfaceA()
}

// exprCoreCastInterfaceC models Java ISupportC (t4 target).
type exprCoreCastInterfaceC interface {
	exprCoreCastInterfaceC()
}

// exprCoreCastInterfaceD models Java ISupportD (t5 target).
type exprCoreCastInterfaceD interface {
	exprCoreCastInterfaceD()
}

// exprCoreCastInterfaceSuperG models Java ISupportAImplSuperG (abstract class
// implementing ISupportA). In Go this is a marker interface implemented only
// by the concrete subclass, preserving Java's superclass match (t6 target).
type exprCoreCastInterfaceSuperG interface {
	exprCoreCastInterfaceBaseAB
	exprCoreCastInterfaceA
	exprCoreCastInterfaceSuperG()
}

// exprCoreCastInterfaceBaseABImpl is the flattened Go counterpart of the Java
// ISupportBaseABImpl class target (t3). The unexported marker ensures only the
// exact ISupportBaseABImpl bean satisfies it — Java class casts are exact-class
// identity and never succeed through an interface.
type exprCoreCastInterfaceBaseABImpl interface {
	exprCoreCastInterfaceBaseAB()
	exprCoreCastInterfaceBaseABImpl()
}

// exprCoreCastInterfaceImplPlus is the flattened Go counterpart of the Java
// ISupportAImplSuperGImplPlus class target (t7). Only the exact ImplPlus bean
// satisfies it.
type exprCoreCastInterfaceImplPlus interface {
	exprCoreCastInterfaceBaseAB()
	exprCoreCastInterfaceImplPlus()
}

// exprCoreCastInterfaceDynRoot models a SupportBeanDynRoot wrapper; it is the
// only value satisfying exprCoreCastInterfaceMarker (send 1 inner bean).
type exprCoreCastInterfaceDynRoot struct {
	inner string
}

func (*exprCoreCastInterfaceDynRoot) exprCoreCastInterfaceMarker() {}

// exprCoreCastInterfaceDImpl models Java ISupportDImpl (implements ISupportD).
// It satisfies only the t5 target, matching Java's identity matrix.
type exprCoreCastInterfaceDImpl struct {
	valueD         string
	valueBaseD     string
	valueBaseDBase string
}

func (*exprCoreCastInterfaceDImpl) exprCoreCastInterfaceD() {}

// exprCoreCastInterfaceBCImpl models Java ISupportBCImpl (implements
// ISupportB and ISupportC). ISupportB extends ISupportBaseAB, so it satisfies
// t2 and t4.
type exprCoreCastInterfaceBCImpl struct {
	valueB      string
	valueBaseAB string
	valueC      string
}

func (*exprCoreCastInterfaceBCImpl) exprCoreCastInterfaceBaseAB() {}
func (*exprCoreCastInterfaceBCImpl) exprCoreCastInterfaceC()      {}

// exprCoreCastInterfaceImplPlusBean models Java ISupportAImplSuperGImplPlus
// (extends ISupportAImplSuperG which implements ISupportA, and implements
// ISupportB and ISupportC). It satisfies t1, t2, t4, t6, and t7.
type exprCoreCastInterfaceImplPlusBean struct {
	valueG      string
	valueA      string
	valueBaseAB string
	valueB      string
	valueC      string
}

func (*exprCoreCastInterfaceImplPlusBean) exprCoreCastInterfaceBaseAB()   {}
func (*exprCoreCastInterfaceImplPlusBean) exprCoreCastInterfaceA()        {}
func (*exprCoreCastInterfaceImplPlusBean) exprCoreCastInterfaceC()        {}
func (*exprCoreCastInterfaceImplPlusBean) exprCoreCastInterfaceSuperG()   {}
func (*exprCoreCastInterfaceImplPlusBean) exprCoreCastInterfaceImplPlus() {}

// exprCoreCastInterfaceBaseABImplBean models Java ISupportBaseABImpl
// (implements ISupportBaseAB). It satisfies t2 and t3.
type exprCoreCastInterfaceBaseABImplBean struct {
	valueBaseAB string
}

func (*exprCoreCastInterfaceBaseABImplBean) exprCoreCastInterfaceBaseAB()     {}
func (*exprCoreCastInterfaceBaseABImplBean) exprCoreCastInterfaceBaseABImpl() {}

type exprCoreExistsCastStaticTypeMapEvent struct {
	AnInt        *string `esper:"anInt"`
	AnDouble     *string `esper:"anDouble"`
	AnLong       *string `esper:"anLong"`
	AnFloat      *string `esper:"anFloat"`
	AnByte       *string `esper:"anByte"`
	AnShort      *string `esper:"anShort"`
	IntPrimitive int     `esper:"intPrimitive"`
	IntBoxed     *int    `esper:"intBoxed"`
}

func runExprCoreExistsCastScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateExprCoreExistsCastScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprCoreExistsCastCaseOrder))
	for _, caseName := range exprCoreExistsCastCaseOrder {
		trace, err := runExprCoreExistsCastCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-exists-cast %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func validateExprCoreExistsCastScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "expr-core-exists-cast" {
		return fmt.Errorf("expr-core-exists-cast scenario has id %q", scenario.ID)
	}
	stepIndex := 0
	for caseIndex, caseName := range exprCoreExistsCastCaseOrder {
		if stepIndex >= len(scenario.Steps) {
			return fmt.Errorf("expr-core-exists-cast scenario is missing case %q", caseName)
		}
		marker := scenario.Steps[stepIndex]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("expr-core-exists-cast step %d has case %q; want %q", stepIndex, marker.Case, caseName)
		}
		if err := validateExprCoreExistsCastStepMetadata(marker, "case"); err != nil {
			return fmt.Errorf("expr-core-exists-cast step %d: %w", stepIndex, err)
		}
		stepIndex++
		for sendIndex, expected := range exprCoreExistsCastExpectedSends[caseIndex] {
			if stepIndex >= len(scenario.Steps) {
				return fmt.Errorf("expr-core-exists-cast case %q is missing send %d", caseName, sendIndex)
			}
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.EventType != expected.eventType {
				return fmt.Errorf("expr-core-exists-cast case %q send %d has op %q/event type %q; want send/%q", caseName, sendIndex, step.Op, step.EventType, expected.eventType)
			}
			if err := validateExprCoreExistsCastStepMetadata(step, "send"); err != nil {
				return fmt.Errorf("expr-core-exists-cast case %q send %d: %w", caseName, sendIndex, err)
			}
			if err := validateExprCoreExistsCastPayload(expected.payload, step.Payload); err != nil {
				return fmt.Errorf("expr-core-exists-cast case %q send %d: %w", caseName, sendIndex, err)
			}
			stepIndex++
		}
	}
	if stepIndex != len(scenario.Steps) {
		return fmt.Errorf("expr-core-exists-cast scenario has %d trailing steps", len(scenario.Steps)-stepIndex)
	}
	return nil
}

func validateExprCoreExistsCastStepMetadata(step compat.Step, op string) error {
	if op == "case" {
		if step.Statement != "" || step.Selector != "" || len(step.Hashes) != 0 || len(step.IDs) != 0 ||
			step.FilterProperty != "" || step.FilterValue != "" || step.ExpectError != "" ||
			step.EventType != "" || step.At != "" || step.Name != "" || step.Source != "" ||
			len(step.PropertyOrder) != 0 || len(step.PropertyTypes) != 0 || len(step.Payload) != 0 {
			return fmt.Errorf("case step has unsupported metadata")
		}
		return nil
	}
	if step.Case != "" || step.Statement != "" || step.Selector != "" || len(step.Hashes) != 0 || len(step.IDs) != 0 ||
		step.FilterProperty != "" || step.FilterValue != "" || step.ExpectError != "" || step.At != "" ||
		step.Name != "" || step.Source != "" || len(step.PropertyOrder) != 0 || len(step.PropertyTypes) != 0 {
		return fmt.Errorf("send step has unsupported metadata")
	}
	return nil
}

func validateExprCoreExistsCastPayload(expected map[string]string, raw json.RawMessage) error {
	actual := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &actual); err != nil {
		return fmt.Errorf("decode payload object: %w", err)
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("payload fields = %d; want %d", len(actual), len(expected))
	}
	for field, want := range expected {
		value, ok := actual[field]
		if !ok {
			return fmt.Errorf("payload is missing field %q", field)
		}
		if string(value) != want {
			return fmt.Errorf("payload field %q = %s; want %s", field, value, want)
		}
	}
	return nil
}

func runExprCoreExistsCastCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	var query esper.Query
	switch caseName {
	case "exists-simple":
		if _, err := esper.RegisterMap(env, "SupportBean", []esper.FieldSpec{
			esper.FieldDef("theString", reflect.TypeOf((*string)(nil))),
			esper.FieldDef("intPrimitive", reflect.TypeOf(int(0))),
			esper.FieldDef("intBoxed", reflect.TypeOf((*int)(nil))),
			esper.FieldDef("floatBoxed", reflect.TypeOf((*float32)(nil))),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportBean")
		query = esper.Select(input,
			esper.Alias("c0", esper.Exists(esper.Field[map[string]any, any]("theString"))),
			esper.Alias("c1", esper.Exists(esper.Field[map[string]any, any]("intBoxed"))),
			esper.Alias("c2", esper.Exists(esper.Field[map[string]any, any]("dummy"))),
			esper.Alias("c3", esper.Exists(esper.Field[map[string]any, any]("intPrimitive"))),
			esper.Alias("c4", esper.Exists(esper.Field[map[string]any, any]("intPrimitive"))),
		).Query(esper.StatementName("s0"))
	case "exists-inner", "exists-om", "exists-compile":
		if _, err := esper.RegisterMap(env, "SupportMarkerInterface", []esper.FieldSpec{
			esper.FieldDef("item", reflect.TypeOf((*any)(nil)).Elem()),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportMarkerInterface")
		item := esper.Field[map[string]any, any]("item")
		if caseName == "exists-inner" {
			nestedItem := esper.OptionalProperty[any](item, "item")
			nested := esper.OptionalProperty[any](item, "nested?")
			nestedNested := esper.OptionalProperty[any](nested, "nestedNested?")
			nestedNestedValue := esper.OptionalProperty[any](nestedNested, "nestedNestedValue?")
			query = esper.Select(input,
				esper.Alias("t0", esper.Exists(esper.OptionalProperty[any](item, "id"))),
				esper.Alias("t1", esper.Exists(esper.OptionalProperty[any](item, "id?"))),
				esper.Alias("t2", esper.Exists(esper.Property[any](nestedItem, "intBoxed"))),
				esper.Alias("t3", esper.Exists(esper.OptionalProperty[any](item, "indexed[0]?"))),
				esper.Alias("t4", esper.Exists(esper.OptionalProperty[any](item, "mapped('keyOne')?"))),
				esper.Alias("t5", esper.Exists(nested)),
				esper.Alias("t6", esper.Exists(esper.OptionalProperty[any](nested, "nestedValue?"))),
				esper.Alias("t7", esper.Exists(nestedNested)),
				esper.Alias("t8", esper.Exists(nestedNestedValue)),
				esper.Alias("t9", esper.Exists(esper.OptionalProperty[any](nestedNestedValue, "dummy?"))),
				esper.Alias("t10", esper.Exists(esper.OptionalProperty[any](nestedNested, "dummy?"))),
			).Query(esper.StatementName("s0"))
		} else {
			query = esper.Select(input,
				esper.Alias("t0", esper.Exists(esper.OptionalProperty[any](item, "intBoxed"))),
			).Query(esper.StatementName("s0"))
		}
	case "cast-simple":
		if _, err := esper.RegisterMap(env, "SupportBean", []esper.FieldSpec{
			esper.FieldDef("theString", reflect.TypeOf((*string)(nil))),
			esper.FieldDef("intPrimitive", reflect.TypeOf(int(0))),
			esper.FieldDef("intBoxed", reflect.TypeOf((*int)(nil))),
			esper.FieldDef("floatBoxed", reflect.TypeOf((*float32)(nil))),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportBean")
		theString := esper.Field[map[string]any, any]("theString")
		intPrimitive := esper.Field[map[string]any, any]("intPrimitive")
		intBoxed := esper.Field[map[string]any, any]("intBoxed")
		floatBoxed := esper.Field[map[string]any, any]("floatBoxed")
		query = esper.Select(input,
			esper.Alias("c0", esper.Cast[any, string](theString)),
			esper.Alias("c1", esper.Cast[any, int](intBoxed)),
			esper.Alias("c2", esper.Cast[any, float32](floatBoxed)),
			esper.Alias("c3", esper.Cast[any, string](theString)),
			esper.Alias("c4", esper.Cast[any, int](intPrimitive)),
			esper.Alias("c5", esper.Cast[any, int64](intPrimitive)),
			esper.Alias("c6", esper.Cast[any, any](intPrimitive)),
			esper.Alias("c7", esper.Cast[any, int64](floatBoxed)),
		).Query(esper.StatementName("s0"))
	case "cast-simple-more-types":
		if _, err := esper.RegisterMap(env, "SupportBean", []esper.FieldSpec{
			esper.FieldDef("theString", reflect.TypeOf((*string)(nil))),
			esper.FieldDef("intPrimitive", reflect.TypeOf(int(0))),
			esper.FieldDef("doublePrimitive", reflect.TypeOf(float64(0))),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportBean")
		theString := esper.Field[map[string]any, any]("theString")
		intPrimitive := esper.Field[map[string]any, any]("intPrimitive")
		doublePrimitive := esper.Field[map[string]any, any]("doublePrimitive")
		query = esper.Select(input,
			esper.Alias("c0", esper.Cast[any, float32](intPrimitive)),
			esper.Alias("c1", esper.Cast[any, int16](intPrimitive)),
			esper.Alias("c2", esper.Cast[any, int8](intPrimitive)),
			esper.Alias("c3", esper.Cast[any, rune](theString)),
			esper.Alias("c4", esper.Cast[any, bool](theString)),
			esper.Alias("c5", esper.Cast[any, big.Int](intPrimitive)),
			esper.Alias("c6", esper.Cast[any, big.Rat](intPrimitive)),
			esper.Alias("c7", esper.Cast[any, big.Rat](doublePrimitive)),
			esper.Alias("c8", esper.Cast[any, rune](theString)),
		).Query(esper.StatementName("s0"))
	case "cast-as-parse":
		if _, err := esper.RegisterMap(env, "SupportBean", []esper.FieldSpec{
			esper.FieldDef("theString", reflect.TypeOf((*string)(nil))),
			esper.FieldDef("intPrimitive", reflect.TypeOf(int(0))),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportBean")
		query = esper.Select(input,
			esper.Alias("t0", esper.Cast[any, int](esper.Field[map[string]any, any]("theString"))),
		).Query(esper.StatementName("s0"))
	case "cast-double-null-om":
		if _, err := esper.RegisterMap(env, "SupportBeanDynRoot", []esper.FieldSpec{
			esper.FieldDef("item", reflect.TypeOf((*any)(nil)).Elem()),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportBeanDynRoot")
		query = esper.Select(input,
			esper.Alias("t0", esper.Cast[any, float64](esper.Field[map[string]any, any]("item"))),
		).Query(esper.StatementName("s0"))
	case "cast-interface":
		if _, err := esper.RegisterMap(env, "SupportBeanDynRoot", []esper.FieldSpec{
			esper.FieldDef("item", reflect.TypeOf((*any)(nil)).Elem()),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		item := esper.Field[map[string]any, any]("item")
		input := esper.From[map[string]any](env, "SupportBeanDynRoot")
		query = esper.Select(input,
			esper.Alias("t0", esper.Cast[any, exprCoreCastInterfaceMarker](item)),
			esper.Alias("t1", esper.Cast[any, exprCoreCastInterfaceA](item)),
			esper.Alias("t2", esper.Cast[any, exprCoreCastInterfaceBaseAB](item)),
			esper.Alias("t3", esper.Cast[any, exprCoreCastInterfaceBaseABImpl](item)),
			esper.Alias("t4", esper.Cast[any, exprCoreCastInterfaceC](item)),
			esper.Alias("t5", esper.Cast[any, exprCoreCastInterfaceD](item)),
			esper.Alias("t6", esper.Cast[any, exprCoreCastInterfaceSuperG](item)),
			esper.Alias("t7", esper.Cast[any, exprCoreCastInterfaceImplPlus](item)),
		).Query(esper.StatementName("s0"))
	case "cast-string-and-null":
		if _, err := esper.RegisterMap(env, "SupportBeanDynRoot", []esper.FieldSpec{
			esper.FieldDef("item", reflect.TypeOf((*any)(nil)).Elem()),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportBeanDynRoot")
		query = esper.Select(input,
			esper.Alias("t0", esper.Cast[any, string](esper.Field[map[string]any, any]("item"))),
		).Query(esper.StatementName("s0"))
	case "cast-boolean":
		if _, err := esper.RegisterMap(env, "SupportBean", []esper.FieldSpec{
			esper.FieldDef("theString", reflect.TypeOf((*string)(nil))),
			esper.FieldDef("intPrimitive", reflect.TypeOf(int(0))),
			esper.FieldDef("boolPrimitive", reflect.TypeOf(false)),
			esper.FieldDef("boolBoxed", reflect.TypeOf((*bool)(nil))),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportBean")
		boolPrimitive := esper.Field[map[string]any, bool]("boolPrimitive")
		boolBoxed := esper.Field[map[string]any, *bool]("boolBoxed")
		query = esper.Select(input,
			esper.Alias("t0", esper.Cast[any, bool](esper.Field[map[string]any, any]("boolPrimitive"))),
			esper.Alias("t1", esper.BitwiseOrOf[bool](boolBoxed, boolPrimitive)),
			esper.Alias("t2", esper.Cast[any, string](esper.Field[map[string]any, any]("boolBoxed"))),
		).Query(esper.StatementName("s0"))
	case "cast-w-static-type":
		if _, err := esper.RegisterMap(env, "StaticTypeMapEvent", []esper.FieldSpec{
			esper.FieldDef("anInt", reflect.TypeOf((*string)(nil))),
			esper.FieldDef("anDouble", reflect.TypeOf((*string)(nil))),
			esper.FieldDef("anLong", reflect.TypeOf((*string)(nil))),
			esper.FieldDef("anFloat", reflect.TypeOf((*string)(nil))),
			esper.FieldDef("anByte", reflect.TypeOf((*string)(nil))),
			esper.FieldDef("anShort", reflect.TypeOf((*string)(nil))),
			esper.FieldDef("intPrimitive", reflect.TypeOf(int(0))),
			esper.FieldDef("intBoxed", reflect.TypeOf((*int)(nil))),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "StaticTypeMapEvent")
		query = esper.Select(input,
			esper.Alias("intVal", esper.Cast[any, int](esper.Field[map[string]any, any]("anInt"))),
			esper.Alias("doubleVal", esper.Cast[any, float64](esper.Field[map[string]any, any]("anDouble"))),
			esper.Alias("longVal", esper.Cast[any, int64](esper.Field[map[string]any, any]("anLong"))),
			esper.Alias("floatVal", esper.Cast[any, float32](esper.Field[map[string]any, any]("anFloat"))),
			esper.Alias("byteVal", esper.Cast[any, int8](esper.Field[map[string]any, any]("anByte"))),
			esper.Alias("shortVal", esper.Cast[any, int16](esper.Field[map[string]any, any]("anShort"))),
			esper.Alias("intOne", esper.Cast[any, int](esper.Field[map[string]any, any]("intPrimitive"))),
			esper.Alias("intTwo", esper.Cast[any, int](esper.Field[map[string]any, any]("intBoxed"))),
			esper.Alias("longOne", esper.Cast[any, int64](esper.Field[map[string]any, any]("intPrimitive"))),
			esper.Alias("longTwo", esper.Cast[any, int64](esper.Field[map[string]any, any]("intBoxed"))),
		).Query(esper.StatementName("s0"))
	case "cast-bigdecimal-bigint":
		if _, err := esper.RegisterMap(env, "MyEvent", []esper.FieldSpec{
			esper.FieldDef("value", reflect.TypeOf((*any)(nil)).Elem()),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "MyEvent")
		query = esper.Select(input,
			esper.Alias("c0", esper.Cast[any, big.Rat](esper.Field[map[string]any, any]("value"))),
			esper.Alias("c1", esper.Cast[any, big.Int](esper.Field[map[string]any, any]("value"))),
		).Query(esper.StatementName("s0"))
	case "cast-warray", "cast-warray-soda":
		if _, err := esper.RegisterMap(env, "MyEventWArray", []esper.FieldSpec{
			esper.FieldDef("arr_string", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("arr_primitive", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("arr_boxed_one", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("arr_boxed_two", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("arr_object", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("arr_2dim_primitive", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("arr_2dim_object", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("arr_3dim_primitive", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("arr_3dim_object", reflect.TypeOf((*any)(nil)).Elem()),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.RegisterStruct[exprCoreExistsCastMyArrayEvent](env, "MyArrayEvent"); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "MyEventWArray")
		query = esper.Select(input,
			esper.Alias("c0", esper.Cast[any, []string](esper.Field[map[string]any, any]("arr_string"))),
			esper.Alias("c1", esper.Cast[any, []int](esper.Field[map[string]any, any]("arr_primitive"))),
			esper.Alias("c2", esper.Cast[any, []int](esper.Field[map[string]any, any]("arr_boxed_one"))),
			esper.Alias("c3", esper.Cast[any, []int](esper.Field[map[string]any, any]("arr_boxed_two"))),
			esper.Alias("c4", esper.Cast[any, []any](esper.Field[map[string]any, any]("arr_object"))),
			esper.Alias("c5", esper.Cast[any, [][]int](esper.Field[map[string]any, any]("arr_2dim_primitive"))),
			esper.Alias("c6", esper.Cast[any, [][]int](esper.Field[map[string]any, any]("arr_2dim_object"))),
			esper.Alias("c7", esper.Cast[any, [][][]int](esper.Field[map[string]any, any]("arr_3dim_primitive"))),
			esper.Alias("c8", esper.Cast[any, [][][]int](esper.Field[map[string]any, any]("arr_3dim_object"))),
		).InsertInto("MyArrayEvent", esper.StatementName("s0"))
	case "cast-generic":
		if _, err := esper.RegisterMap(env, "MyEventGeneric", []esper.FieldSpec{
			esper.FieldDef("listOfString", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("listOfOptionalInteger", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("mapOfStringAndInteger", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("listArrayOfString", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("listOfStringArray", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("listArray2DimOfString", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("listOfStringArray2Dim", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("listOfT", reflect.TypeOf((*any)(nil)).Elem()),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "MyEventGeneric")
		query = esper.Select(input,
			esper.Alias("listOfString", esper.Cast[any, []any](esper.Field[map[string]any, any]("listOfString"))),
			esper.Alias("listOfOptionalInteger", esper.Cast[any, []any](esper.Field[map[string]any, any]("listOfOptionalInteger"))),
			esper.Alias("mapOfStringAndInteger", esper.Cast[any, map[string]any](esper.Field[map[string]any, any]("mapOfStringAndInteger"))),
			esper.Alias("listArrayOfString", esper.Cast[any, [][]any](esper.Field[map[string]any, any]("listArrayOfString"))),
			esper.Alias("listOfStringArray", esper.Cast[any, [][]any](esper.Field[map[string]any, any]("listOfStringArray"))),
			esper.Alias("listArray2DimOfString", esper.Cast[any, [][][]any](esper.Field[map[string]any, any]("listArray2DimOfString"))),
			esper.Alias("listOfStringArray2Dim", esper.Cast[any, [][][]any](esper.Field[map[string]any, any]("listOfStringArray2Dim"))),
			esper.Alias("listOfT", esper.Cast[any, []any](esper.Field[map[string]any, any]("listOfT"))),
		).Query(esper.StatementName("s0"))
	case "cast-dates-base":
		if _, err := esper.RegisterMap(env, "MyDateType", []esper.FieldSpec{
			esper.FieldDef("yyyymmdd", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("yyyymmddhhmmss", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("hhmmss", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("yyyymmddhhmmssvv", reflect.TypeOf((*any)(nil)).Elem()),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "MyDateType")
		dateCast := esper.CastWithLayout[string, time.Time](esper.Field[map[string]any, string]("yyyymmdd"), "20060102")
		query = esper.Select(input,
			esper.Alias("c0", dateCast),
			esper.Alias("c1", dateCast),
			esper.Alias("c2", esper.UnixMillis(dateCast)),
			esper.Alias("c3", esper.UnixMillis(dateCast)),
			esper.Alias("c4", dateCast),
			esper.Alias("c5", dateCast),
			esper.Alias("c6", esper.Subtract[int64](esper.Month(dateCast), esper.Literal[int64](1))),
			esper.Alias("c7", esper.Subtract[int64](esper.Month(dateCast), esper.Literal[int64](1))),
			esper.Alias("c8", esper.Subtract[int64](esper.Month(dateCast), esper.Literal[int64](1))),
		).Query(esper.StatementName("s0"))
	case "cast-dates-java8":
		if _, err := esper.RegisterMap(env, "MyDateType", []esper.FieldSpec{
			esper.FieldDef("yyyymmdd", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("yyyymmddhhmmss", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("hhmmss", reflect.TypeOf((*any)(nil)).Elem()),
			esper.FieldDef("yyyymmddhhmmssvv", reflect.TypeOf((*any)(nil)).Elem()),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "MyDateType")
		query = esper.Select(input,
			esper.Alias("c0", esper.CastWithLayout[string, time.Time](esper.Field[map[string]any, string]("yyyymmdd"), "20060102")),
			esper.Alias("c1", esper.CastWithLayout[string, time.Time](esper.Field[map[string]any, string]("yyyymmdd"), "20060102")),
			esper.Alias("c2", esper.CastWithLayout[string, time.Time](esper.Field[map[string]any, string]("yyyymmddhhmmss"), "20060102150405")),
			esper.Alias("c3", esper.CastWithLayout[string, time.Time](esper.Field[map[string]any, string]("yyyymmddhhmmss"), "20060102150405")),
			esper.Alias("c4", esper.CastWithLayout[string, time.Time](esper.Field[map[string]any, string]("hhmmss"), "150405")),
			esper.Alias("c5", esper.CastWithLayout[string, time.Time](esper.Field[map[string]any, string]("hhmmss"), "150405")),
		).Query(esper.StatementName("s0"))
	case "cast-dates-constant":
		if _, err := esper.RegisterMap(env, "SupportBean", []esper.FieldSpec{
			esper.FieldDef("theString", reflect.TypeOf((*string)(nil))),
			esper.FieldDef("intPrimitive", reflect.TypeOf(int(0))),
		}, esper.AllowDynamicFields()); err != nil {
			return compat.Trace{}, err
		}
		input := esper.From[map[string]any](env, "SupportBean")
		query = esper.Select(input,
			esper.Alias("c0", esper.CastWithLayout[string, time.Time](esper.Literal[string]("20030201"), "20060102")),
		).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-core-exists-cast case %q", caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	trace, err := compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeExprCoreExistsCastPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown expr-core-exists-cast statement %q", name)
		}
		return statement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	return normalizeExprCoreExistsCastTrace(trace), nil
}

func decodeExprCoreExistsCastPayload(step compat.Step) (any, error) {
	if step.EventType == "SupportBean" {
		var payload struct {
			TheString       *string  `json:"theString"`
			IntPrimitive    int      `json:"intPrimitive"`
			IntBoxed        *int     `json:"intBoxed"`
			FloatBoxed      *float32 `json:"floatBoxed"`
			DoublePrimitive float64  `json:"doublePrimitive"`
			BoolPrimitive   bool     `json:"boolPrimitive"`
			BoolBoxed       *bool    `json:"boolBoxed"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("expr-core-exists-cast: decode SupportBean: %w", err)
		}
		return map[string]any{
			"theString":       payload.TheString,
			"intPrimitive":    payload.IntPrimitive,
			"intBoxed":        payload.IntBoxed,
			"floatBoxed":      payload.FloatBoxed,
			"doublePrimitive": payload.DoublePrimitive,
			"boolPrimitive":   payload.BoolPrimitive,
			"boolBoxed":       payload.BoolBoxed,
		}, nil
	}
	if step.EventType == "StaticTypeMapEvent" {
		var payload struct {
			AnInt        *string `json:"anInt"`
			AnDouble     *string `json:"anDouble"`
			AnLong       *string `json:"anLong"`
			AnFloat      *string `json:"anFloat"`
			AnByte       *string `json:"anByte"`
			AnShort      *string `json:"anShort"`
			IntPrimitive int     `json:"intPrimitive"`
			IntBoxed     *int    `json:"intBoxed"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("expr-core-exists-cast: decode StaticTypeMapEvent: %w", err)
		}
		return map[string]any{
			"anInt":        payload.AnInt,
			"anDouble":     payload.AnDouble,
			"anLong":       payload.AnLong,
			"anFloat":      payload.AnFloat,
			"anByte":       payload.AnByte,
			"anShort":      payload.AnShort,
			"intPrimitive": payload.IntPrimitive,
			"intBoxed":     payload.IntBoxed,
		}, nil
	}
	if step.EventType == "SupportBeanDynRoot" {
		var payload struct {
			Shape     string          `json:"shape"`
			ItemType  string          `json:"itemType"`
			ItemValue json.RawMessage `json:"itemValue"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("expr-core-exists-cast: decode dynamic cast payload: %w", err)
		}
		// The cast-interface case encodes the bean shape structurally rather
		// than through a scalar itemType/itemValue pair.
		if payload.Shape != "" {
			item := decodeExprCoreExistsCastInterfaceBean(payload.Shape)
			if item == nil {
				return nil, fmt.Errorf("expr-core-exists-cast: unsupported cast-interface shape %q", payload.Shape)
			}
			return map[string]any{"item": item}, nil
		}
		var item any
		switch payload.ItemType {
		case "int":
			var value int
			if err := json.Unmarshal(payload.ItemValue, &value); err != nil {
				return nil, fmt.Errorf("expr-core-exists-cast: decode int item: %w", err)
			}
			item = value
		case "byte":
			var value int8
			if err := json.Unmarshal(payload.ItemValue, &value); err != nil {
				return nil, fmt.Errorf("expr-core-exists-cast: decode byte item: %w", err)
			}
			item = value
		case "double":
			var value float64
			if err := json.Unmarshal(payload.ItemValue, &value); err != nil {
				return nil, fmt.Errorf("expr-core-exists-cast: decode double item: %w", err)
			}
			item = value
		case "int64":
			var value int64
			if err := json.Unmarshal(payload.ItemValue, &value); err != nil {
				return nil, fmt.Errorf("expr-core-exists-cast: decode int64 item: %w", err)
			}
			item = value
		case "null":
			item = nil
		case "string":
			var value string
			if err := json.Unmarshal(payload.ItemValue, &value); err != nil {
				return nil, fmt.Errorf("expr-core-exists-cast: decode string item: %w", err)
			}
			item = value
		default:
			return nil, fmt.Errorf("expr-core-exists-cast: unsupported cast item type %q", payload.ItemType)
		}
		return map[string]any{"item": item}, nil
	}
	if step.EventType == "MyEvent" {
		var payload struct {
			Kind     string `json:"kind"`
			Value    any    `json:"value"`
			Text     string `json:"text"`
			Exponent int    `json:"exponent"`
		}
		decoder := json.NewDecoder(bytes.NewReader(step.Payload))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			return nil, fmt.Errorf("expr-core-exists-cast: decode MyEvent: %w", err)
		}
		var value any
		switch payload.Kind {
		case "int":
			number := payload.Value.(json.Number)
			parsed, err := number.Int64()
			if err != nil {
				return nil, fmt.Errorf("expr-core-exists-cast: decode MyEvent int: %w", err)
			}
			value = int(parsed)
		case "long":
			number := payload.Value.(json.Number)
			parsed, err := number.Int64()
			if err != nil {
				return nil, fmt.Errorf("expr-core-exists-cast: decode MyEvent long: %w", err)
			}
			value = parsed
		case "double":
			number := payload.Value.(json.Number)
			parsed, err := number.Float64()
			if err != nil {
				return nil, fmt.Errorf("expr-core-exists-cast: decode MyEvent double: %w", err)
			}
			value = parsed
		case "decimal":
			rat, ok := new(big.Rat).SetString(payload.Text)
			if !ok {
				return nil, fmt.Errorf("expr-core-exists-cast: decode MyEvent decimal %q", payload.Text)
			}
			value = big.Rat(*rat)
		case "bigint":
			integer, ok := new(big.Int).SetString(payload.Text, 10)
			if !ok {
				return nil, fmt.Errorf("expr-core-exists-cast: decode MyEvent bigint %q", payload.Text)
			}
			value = *integer
		case "pow2-decimal":
			pow := new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(payload.Exponent)), nil)
			rat := new(big.Rat).SetInt(pow)
			rat.Add(rat, new(big.Rat).SetFrac(big.NewInt(1), big.NewInt(10)))
			value = *rat
		case "pow2-bigint":
			pow := new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(payload.Exponent)), nil)
			value = *pow
		case "null":
			value = nil
		default:
			return nil, fmt.Errorf("expr-core-exists-cast: unsupported MyEvent kind %q", payload.Kind)
		}
		return map[string]any{"value": value}, nil
	}
	if step.EventType == "MyEventWArray" {
		var payload struct {
			Shape            string            `json:"shape"`
			ArrString        []string          `json:"arr_string"`
			ArrPrimitive     []int             `json:"arr_primitive"`
			ArrBoxedOne      []int             `json:"arr_boxed_one"`
			ArrBoxedTwo      []int             `json:"arr_boxed_two"`
			ArrObject        []json.RawMessage `json:"arr_object"`
			Arr2DimPrimitive [][]int           `json:"arr_2dim_primitive"`
			Arr2DimObject    [][]int           `json:"arr_2dim_object"`
			Arr3DimPrimitive [][][]int         `json:"arr_3dim_primitive"`
			Arr3DimObject    [][][]int         `json:"arr_3dim_object"`
		}
		decoder := json.NewDecoder(bytes.NewReader(step.Payload))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			return nil, fmt.Errorf("expr-core-exists-cast: decode MyEventWArray: %w", err)
		}
		result := map[string]any{
			"arr_string":         payload.ArrString,
			"arr_primitive":      payload.ArrPrimitive,
			"arr_boxed_one":      payload.ArrBoxedOne,
			"arr_boxed_two":      payload.ArrBoxedTwo,
			"arr_object":         nil,
			"arr_2dim_primitive": payload.Arr2DimPrimitive,
			"arr_2dim_object":    payload.Arr2DimObject,
			"arr_3dim_primitive": payload.Arr3DimPrimitive,
			"arr_3dim_object":    payload.Arr3DimObject,
		}
		if payload.Shape == "full" {
			objects := make([]any, 0, len(payload.ArrObject))
			for _, raw := range payload.ArrObject {
				var bean struct {
					TheString    *string `json:"theString"`
					IntPrimitive int     `json:"intPrimitive"`
				}
				if err := json.Unmarshal(raw, &bean); err != nil {
					return nil, fmt.Errorf("expr-core-exists-cast: decode MyEventWArray arr_object: %w", err)
				}
				objects = append(objects, &exprCoreExistsCastSupportBean{
					TheString:    bean.TheString,
					IntPrimitive: bean.IntPrimitive,
				})
			}
			result["arr_object"] = objects
		}
		return result, nil
	}
	if step.EventType == "MyEventGeneric" {
		var payload struct {
			Shape                 string         `json:"shape"`
			ListOfString          []string       `json:"listOfString"`
			ListOfOptionalInteger []int          `json:"listOfOptionalInteger"`
			MapOfStringAndInteger map[string]int `json:"mapOfStringAndInteger"`
			ListArrayOfString     [][]string     `json:"listArrayOfString"`
			ListOfStringArray     [][]string     `json:"listOfStringArray"`
			ListArray2DimOfString [][][]string   `json:"listArray2DimOfString"`
			ListOfStringArray2Dim [][][]string   `json:"listOfStringArray2Dim"`
			ListOfT               []string       `json:"listOfT"`
		}
		decoder := json.NewDecoder(bytes.NewReader(step.Payload))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			return nil, fmt.Errorf("expr-core-exists-cast: decode MyEventGeneric: %w", err)
		}
		if payload.Shape == "empty" {
			return map[string]any{
				"listOfString":          nil,
				"listOfOptionalInteger": nil,
				"mapOfStringAndInteger": nil,
				"listArrayOfString":     nil,
				"listOfStringArray":     nil,
				"listArray2DimOfString": nil,
				"listOfStringArray2Dim": nil,
				"listOfT":               nil,
			}, nil
		}
		optionals := make([]any, 0, len(payload.ListOfOptionalInteger))
		for _, value := range payload.ListOfOptionalInteger {
			optionals = append(optionals, exprCoreCastOptional{value: value, present: true})
		}
		genericMap := make(map[string]any, len(payload.MapOfStringAndInteger))
		for name, value := range payload.MapOfStringAndInteger {
			genericMap[name] = value
		}
		listOfT := make([]any, 0, len(payload.ListOfT))
		for _, value := range payload.ListOfT {
			listOfT = append(listOfT, value)
		}
		listArrayOfString := make([][]any, 0, len(payload.ListArrayOfString))
		for _, inner := range payload.ListArrayOfString {
			listArrayOfString = append(listArrayOfString, toAnySlice(inner))
		}
		listOfStringArray := make([][]any, 0, len(payload.ListOfStringArray))
		for _, inner := range payload.ListOfStringArray {
			listOfStringArray = append(listOfStringArray, toAnySlice(inner))
		}
		listArray2Dim := make([][][]any, 0, len(payload.ListArray2DimOfString))
		for _, inner := range payload.ListArray2DimOfString {
			outer := make([][]any, 0, len(inner))
			for _, nested := range inner {
				outer = append(outer, toAnySlice(nested))
			}
			listArray2Dim = append(listArray2Dim, outer)
		}
		listOfStringArray2Dim := make([][][]any, 0, len(payload.ListOfStringArray2Dim))
		for _, inner := range payload.ListOfStringArray2Dim {
			outer := make([][]any, 0, len(inner))
			for _, nested := range inner {
				outer = append(outer, toAnySlice(nested))
			}
			listOfStringArray2Dim = append(listOfStringArray2Dim, outer)
		}
		return map[string]any{
			"listOfString":          toAnySlice(payload.ListOfString),
			"listOfOptionalInteger": optionals,
			"mapOfStringAndInteger": genericMap,
			"listArrayOfString":     listArrayOfString,
			"listOfStringArray":     listOfStringArray,
			"listArray2DimOfString": listArray2Dim,
			"listOfStringArray2Dim": listOfStringArray2Dim,
			"listOfT":               listOfT,
		}, nil
	}
	if step.EventType == "MyDateType" {
		var payload struct {
			Yyyymmdd         *string `json:"yyyymmdd"`
			Yyyymmddhhmmss   *string `json:"yyyymmddhhmmss"`
			Hhmmss           *string `json:"hhmmss"`
			Yyyymmddhhmmssvv *string `json:"yyyymmddhhmmssvv"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("expr-core-exists-cast: decode MyDateType: %w", err)
		}
		return map[string]any{
			"yyyymmdd":         payload.Yyyymmdd,
			"yyyymmddhhmmss":   payload.Yyyymmddhhmmss,
			"hhmmss":           payload.Hhmmss,
			"yyyymmddhhmmssvv": payload.Yyyymmddhhmmssvv,
		}, nil
	}
	if step.EventType != "SupportMarkerInterface" {
		return nil, fmt.Errorf("expr-core-exists-cast: unsupported event type %q", step.EventType)
	}
	var payload struct {
		Shape string `json:"shape"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return nil, fmt.Errorf("expr-core-exists-cast: decode dynamic payload: %w", err)
	}
	var item any
	switch payload.Shape {
	case "null":
		item = nil
	case "complex":
		item = &exprCoreExistsCastComplexProps{
			Indexed: []int{1, 2},
			Mapped:  map[string]string{"keyOne": "valueOne", "keyTwo": "valueTwo"},
			Nested: &exprCoreExistsCastNested{
				NestedValue:  "nestedValue",
				NestedNested: &exprCoreExistsCastNestedNested{NestedNestedValue: "nestedNestedValue"},
			},
		}
	case "nested-support-bean":
		item = &exprCoreExistsCastDynamicRoot{Item: &exprCoreExistsCastSupportBean{}}
	case "support-bean-a":
		item = &exprCoreExistsCastSupportBeanA{ID: "10"}
	case "support-bean":
		item = &exprCoreExistsCastSupportBean{}
	case "string":
		item = "abc"
	default:
		return nil, fmt.Errorf("expr-core-exists-cast: unsupported shape %q", payload.Shape)
	}
	return map[string]any{"item": item}, nil
}

// decodeExprCoreExistsCastInterfaceBean maps a cast-interface scenario shape
// to the Go bean that models the corresponding Java support-bean instance. It
// returns nil for an unknown shape so the caller can report an error.
func decodeExprCoreExistsCastInterfaceBean(shape string) any {
	switch shape {
	case "dyn-root":
		return &exprCoreCastInterfaceDynRoot{inner: "abc"}
	case "isupportd":
		return &exprCoreCastInterfaceDImpl{}
	case "isupportbc":
		return &exprCoreCastInterfaceBCImpl{}
	case "isupportplus":
		return &exprCoreCastInterfaceImplPlusBean{}
	case "isupportbaseab":
		return &exprCoreCastInterfaceBaseABImplBean{}
	}
	return nil
}

func normalizeExprCoreExistsCastTrace(trace compat.Trace) compat.Trace {
	for recordIndex := range trace.Records {
		record := &trace.Records[recordIndex]
		for resultIndex := range record.New {
			for name, value := range record.New[resultIndex].Fields {
				record.New[resultIndex].Fields[name] = normalizeExprCoreExistsCastField(record.Case, name, value)
			}
		}
		for resultIndex := range record.Old {
			for name, value := range record.Old[resultIndex].Fields {
				record.Old[resultIndex].Fields[name] = normalizeExprCoreExistsCastField(record.Case, name, value)
			}
		}
	}
	return trace
}

// normalizeExprCoreExistsCastField renders temporal cells the way the Java
// oracle's TraceWriter does: java.util.Date/Calendar columns as epoch-millis
// JSON numbers, java.time columns as their ISO toString forms (LocalDate
// "2006-01-02", LocalDateTime "2006-01-02T15:04:05", LocalTime "15:04:05").
func normalizeExprCoreExistsCastField(caseName, fieldName string, value any) any {
	if current, ok := value.(time.Time); ok {
		switch caseName {
		case "cast-dates-java8":
			switch fieldName {
			case "c0", "c1":
				return current.Format("2006-01-02")
			case "c2", "c3":
				return current.Format("2006-01-02T15:04:05")
			case "c4", "c5":
				return current.Format("15:04:05")
			}
		default:
			return current.UnixMilli()
		}
	}
	return normalizeExprCoreExistsCastValue(value)
}

func normalizeExprCoreExistsCastValue(value any) any {
	switch current := value.(type) {
	case rune:
		return string(current)
	case float32:
		return formatExprCoreExistsCastFloat(float64(current), 32)
	case float64:
		return formatExprCoreExistsCastFloat(current, 64)
	case int:
		return strconv.FormatInt(int64(current), 10)
	case int8:
		return strconv.FormatInt(int64(current), 10)
	case int16:
		return strconv.FormatInt(int64(current), 10)
	case int64:
		return strconv.FormatInt(current, 10)
	case uint:
		return strconv.FormatUint(uint64(current), 10)
	case uint8:
		return strconv.FormatUint(uint64(current), 10)
	case uint16:
		return strconv.FormatUint(uint64(current), 10)
	case uint32:
		return strconv.FormatUint(uint64(current), 10)
	case uint64:
		return strconv.FormatUint(current, 10)
	case big.Int:
		return current.String()
	case big.Rat:
		return bigRatExactDecimal(current)
	case map[string]any:
		for name, nested := range current {
			current[name] = normalizeExprCoreExistsCastValue(nested)
		}
	case []any:
		for index, nested := range current {
			current[index] = normalizeExprCoreExistsCastValue(nested)
		}
	}
	// Typed slices and arrays ([]string, []int, [][]int, multi-dim, etc.)
	// render as JSON arrays of normalized elements, matching the oracle's
	// array branch in TraceWriter.normalize. Booleans stay booleans, nested
	// arrays recurse, all other elements stringify.
	valueReflect := reflect.ValueOf(value)
	for valueReflect.IsValid() && (valueReflect.Kind() == reflect.Pointer || valueReflect.Kind() == reflect.Interface) {
		if valueReflect.IsNil() {
			return value
		}
		valueReflect = valueReflect.Elem()
	}
	if valueReflect.IsValid() && (valueReflect.Kind() == reflect.Slice || valueReflect.Kind() == reflect.Array) {
		items := make([]any, 0, valueReflect.Len())
		for index := 0; index < valueReflect.Len(); index++ {
			elem := valueReflect.Index(index)
			if !elem.IsValid() || !elem.CanInterface() {
				return value
			}
			items = append(items, normalizeExprCoreExistsCastValue(elem.Interface()))
		}
		return items
	}
	if token := exprCoreExistsCastInterfaceBeanToken(value); token != "" {
		return token
	}
	return value
}

// exprCoreExistsCastInterfaceBeanToken maps a matched cast-interface bean to
// the deterministic Java simple-class-name token the oracle also renders. Both
// sides identifying the bean class (rather than an identity-hash) is what makes
// the bean-present cells byte-exact in the differential diff.
func exprCoreExistsCastInterfaceBeanToken(value any) string {
	switch value.(type) {
	case *exprCoreCastInterfaceDynRoot:
		return "SupportBeanDynRoot"
	case *exprCoreCastInterfaceDImpl:
		return "ISupportDImpl"
	case *exprCoreCastInterfaceBCImpl:
		return "ISupportBCImpl"
	case *exprCoreCastInterfaceImplPlusBean:
		return "ISupportAImplSuperGImplPlus"
	case *exprCoreCastInterfaceBaseABImplBean:
		return "ISupportBaseABImpl"
	}
	switch value := value.(type) {
	case *exprCoreExistsCastSupportBean:
		theString := ""
		if value.TheString != nil {
			theString = *value.TheString
		}
		return "SupportBean(" + theString + "," + strconv.Itoa(value.IntPrimitive) + ")"
	case exprCoreCastOptional:
		if !value.present {
			return "Optional.empty"
		}
		inner := normalizeExprCoreExistsCastValue(value.value)
		if text, ok := inner.(string); ok {
			return "Optional[" + text + "]"
		}
		return "Optional[" + fmt.Sprint(inner) + "]"
	}
	return ""
}

func formatExprCoreExistsCastFloat(value float64, bitSize int) string {
	formatted := strconv.FormatFloat(value, 'g', -1, bitSize)
	if !strings.ContainsAny(formatted, ".eE") {
		formatted += ".0"
	}
	return formatted
}

// bigRatExactDecimal renders a big.Rat value as its exact terminating
// decimal string, matching Java BigDecimal.toString() for the values Esper's
// BigDecimal casts produce (doubles convert to a denominator that is a power
// of two; decimal strings keep a terminating denominator of 2s and 5s;
// integer inputs have denominator 1). If the denominator has a factor other
// than 2 or 5 the decimal would repeat; that case falls back to the rational
// string and cannot arise from the BigDecimal cast surface.
func bigRatExactDecimal(rat big.Rat) string {
	if rat.IsInt() {
		return rat.Num().String()
	}
	num := new(big.Int).Set(rat.Num())
	den := new(big.Int).Set(rat.Denom())
	// Reduce common factors.
	gcd := new(big.Int).GCD(nil, nil, num, den)
	num.Quo(num, gcd)
	den.Quo(den, gcd)
	// Strip 2 and 5 factors from the denominator.
	twos, fives := 0, 0
	remaining := new(big.Int).Set(den)
	two := big.NewInt(2)
	five := big.NewInt(5)
	tmp := new(big.Int)
	for tmp.Mod(remaining, two).Sign() == 0 {
		remaining.Quo(remaining, two)
		twos++
	}
	for tmp.Mod(remaining, five).Sign() == 0 {
		remaining.Quo(remaining, five)
		fives++
	}
	if remaining.Cmp(big.NewInt(1)) != 0 {
		// Repeating decimal: not producible by the BigDecimal cast surface;
		// fall back to the rational string to stay deterministic.
		return rat.String()
	}
	fractionDigits := twos
	if fives > fractionDigits {
		fractionDigits = fives
	}
	// Integer part and remainder by floor division.
	intPart := new(big.Int).Quo(num, den)
	remainder := new(big.Int).Rem(num, den)
	negative := intPart.Sign() < 0 || (intPart.Sign() == 0 && remainder.Sign() < 0)
	if negative {
		intPart.Abs(intPart)
		remainder.Abs(remainder)
	}
	// Long-divide a digit at a time to get exactly fractionDigits decimals.
	digits := make([]byte, 0, fractionDigits)
	ten := big.NewInt(10)
	digit := new(big.Int)
	for range fractionDigits {
		remainder.Mul(remainder, ten)
		digit.Quo(remainder, den)
		remainder.Mod(remainder, den)
		digits = append(digits, byte('0'+digit.Int64()))
	}
	result := intPart.String() + "." + string(digits)
	if negative {
		result = "-" + result
	}
	return result
}
