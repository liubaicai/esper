package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const exprCoreExistsCastJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreExistsCastJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreExists.java",
}

var exprCoreExistsCastJavaRuntimeIDs = []string{
	"java-runtime-d77c035088ce635538c7",
	"java-runtime-4202039fb2f1ea65fd49",
	"java-runtime-8fa7f5076dde9d791d08",
	"java-runtime-afd826c7d955eb538001",
}

var exprCoreExistsCastJavaExecutions = []string{
	"ExprCoreExistsSimple",
	"ExprCoreExistsInner",
	"ExprCoreCastDoubleAndNullOM",
	"ExprCoreCastStringAndNullCompile",
}

var exprCoreExistsCastCaseOrder = []string{
	"exists-simple",
	"exists-inner",
	"exists-om",
	"exists-compile",
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
}

type exprCoreExistsCastSupportBean struct {
	TheString    *string  `esper:"theString"`
	IntPrimitive int      `esper:"intPrimitive"`
	IntBoxed     *int     `esper:"intBoxed"`
	FloatBoxed   *float32 `esper:"floatBoxed"`
}

type exprCoreExistsCastNestedNested struct {
	NestedNestedValue string `esper:"nestedNestedValue"`
}

type exprCoreExistsCastNested struct {
	NestedValue  string                          `esper:"nestedValue"`
	NestedNested *exprCoreExistsCastNestedNested `esper:"nestedNested"`
}

type exprCoreExistsCastComplexProps struct {
	Indexed []int                     `esper:"indexed"`
	Mapped  map[string]string         `esper:"mapped"`
	Nested  *exprCoreExistsCastNested `esper:"nested"`
}

type exprCoreExistsCastSupportBeanA struct {
	ID string `esper:"id"`
}

type exprCoreExistsCastDynamicRoot struct {
	Item any `esper:"item"`
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeExprCoreExistsCastPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown expr-core-exists-cast statement %q", name)
		}
		return statement, nil
	})
}

func decodeExprCoreExistsCastPayload(step compat.Step) (any, error) {
	if step.EventType == "SupportBean" {
		var payload struct {
			TheString    *string  `json:"theString"`
			IntPrimitive int      `json:"intPrimitive"`
			IntBoxed     *int     `json:"intBoxed"`
			FloatBoxed   *float32 `json:"floatBoxed"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("expr-core-exists-cast: decode SupportBean: %w", err)
		}
		return map[string]any{
			"theString":    payload.TheString,
			"intPrimitive": payload.IntPrimitive,
			"intBoxed":     payload.IntBoxed,
			"floatBoxed":   payload.FloatBoxed,
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
