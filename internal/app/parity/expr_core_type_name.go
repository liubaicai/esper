package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const exprCoreTypeNameJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprCoreTypeNameJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreTypeOf.java",
}

var exprCoreTypeNameJavaRuntimeIDs = []string{
	"java-runtime-eeeacbe2c7669e1e1207",
}

var exprCoreTypeNameJavaExecutions = []string{
	"ExprCoreTypeOfFragment",
}

var exprCoreTypeNameCaseOrder = []string{
	"type-name-fragment-object-array",
	"type-name-fragment-map",
	"type-name-fragment-avro",
	"type-name-fragment-json",
	"type-name-fragment-json-provided",
	"type-name-fragment-default",
}

type exprCoreTypeNameExpectedSend struct {
	state string
}

var exprCoreTypeNameExpectedSends = func() [][]exprCoreTypeNameExpectedSend {
	result := make([][]exprCoreTypeNameExpectedSend, 0, len(exprCoreTypeNameCaseOrder))
	for _, caseName := range exprCoreTypeNameCaseOrder {
		_ = caseName
		result = append(result, []exprCoreTypeNameExpectedSend{
			{state: "empty"},
			{state: "inside"},
			{state: "insidearr"},
		})
	}
	return result
}()

type exprCoreTypeNameJSONProvidedInner struct {
	Key string `json:"key"`
}

type exprCoreTypeNameJSONProvidedRoot struct {
	Inside    *exprCoreTypeNameJSONProvidedInner   `json:"inside"`
	InsideArr []*exprCoreTypeNameJSONProvidedInner `json:"insidearr"`
}

func runExprCoreTypeNameScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateExprCoreTypeNameScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprCoreTypeNameCaseOrder))
	for _, caseName := range exprCoreTypeNameCaseOrder {
		trace, err := runExprCoreTypeNameCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-core-type-name %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func validateExprCoreTypeNameScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != "expr-core-type-name" {
		return fmt.Errorf("expr-core-type-name scenario has id %q", scenario.ID)
	}
	stepIndex := 0
	for caseIndex, caseName := range exprCoreTypeNameCaseOrder {
		if stepIndex >= len(scenario.Steps) {
			return fmt.Errorf("expr-core-type-name scenario is missing case %q", caseName)
		}
		marker := scenario.Steps[stepIndex]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("expr-core-type-name step %d has case %q; want %q", stepIndex, marker.Case, caseName)
		}
		if err := validateExprCoreTypeNameStepMetadata(marker, "case"); err != nil {
			return fmt.Errorf("expr-core-type-name step %d: %w", stepIndex, err)
		}
		stepIndex++
		for sendIndex, expected := range exprCoreTypeNameExpectedSends[caseIndex] {
			if stepIndex >= len(scenario.Steps) {
				return fmt.Errorf("expr-core-type-name case %q is missing send %d", caseName, sendIndex)
			}
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.EventType != "MySchema" {
				return fmt.Errorf("expr-core-type-name case %q send %d has op %q/event type %q; want send/MySchema", caseName, sendIndex, step.Op, step.EventType)
			}
			if err := validateExprCoreTypeNameStepMetadata(step, "send"); err != nil {
				return fmt.Errorf("expr-core-type-name case %q send %d: %w", caseName, sendIndex, err)
			}
			if err := validateExprCoreTypeNamePayload(expected, step.Payload); err != nil {
				return fmt.Errorf("expr-core-type-name case %q send %d: %w", caseName, sendIndex, err)
			}
			stepIndex++
		}
	}
	if stepIndex != len(scenario.Steps) {
		return fmt.Errorf("expr-core-type-name scenario has %d trailing steps", len(scenario.Steps)-stepIndex)
	}
	return nil
}

func validateExprCoreTypeNameStepMetadata(step compat.Step, op string) error {
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

func validateExprCoreTypeNamePayload(expected exprCoreTypeNameExpectedSend, raw json.RawMessage) error {
	values := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("decode payload object: %w", err)
	}
	if len(values) != 1 {
		return fmt.Errorf("payload fields = %d; want 1", len(values))
	}
	value, ok := values["shape"]
	if !ok {
		return fmt.Errorf("payload is missing field %q", "shape")
	}
	var actual string
	if err := json.Unmarshal(value, &actual); err != nil {
		return fmt.Errorf("payload field %q is not a string: %w", "shape", err)
	}
	if actual != expected.state {
		return fmt.Errorf("payload field %q = %q; want %q", "shape", actual, expected.state)
	}
	return nil
}

func runExprCoreTypeNameCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	stringType := reflect.TypeOf("")
	inner, err := newExprCoreTypeNameSchema(caseName, "InnerSchema", []esper.FieldSpec{
		esper.FieldDef("key", stringType),
	})
	if err != nil {
		return compat.Trace{}, err
	}
	if err := env.RegisterSchema(inner); err != nil {
		return compat.Trace{}, err
	}
	root, err := newExprCoreTypeNameSchema(caseName, "MySchema", []esper.FieldSpec{
		esper.FieldDef("inside", reflect.TypeOf(map[string]any{})),
		esper.FieldDef("insidearr", reflect.TypeOf([]map[string]any{})),
	}, esper.WithNestedPropertySchema("inside", inner), esper.WithNestedPropertySchema("insidearr", inner))
	if err != nil {
		return compat.Trace{}, err
	}
	if err := env.RegisterSchema(root); err != nil {
		return compat.Trace{}, err
	}

	input := esper.FromAny(env, "MySchema")
	query := input.Select(
		esper.Alias("t0", esper.TypeName(esper.Field[any, any]("inside"))),
		esper.Alias("t1", esper.TypeName(esper.Field[any, any]("insidearr"))),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, func(step compat.Step) (any, error) {
		return decodeExprCoreTypeNamePayload(root, step)
	}, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown expr-core-type-name statement %q", name)
		}
		return statement, nil
	})
}

func newExprCoreTypeNameSchema(caseName, name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
	representation := caseName[len("type-name-fragment-"):]
	switch representation {
	case "object-array":
		return esper.NewObjectArraySchema(name, fields, opts...)
	case "map", "default":
		return esper.NewMapSchema(name, fields, opts...)
	case "avro":
		return esper.NewAvroSchema(name, fields, opts...)
	case "json":
		return esper.NewJSONSchema(name, fields, opts...)
	case "json-provided":
		if name == "MySchema" {
			return esper.NewJSONSchemaFor[exprCoreTypeNameJSONProvidedRoot](name, nil, opts...)
		}
		return esper.NewJSONSchema(name, fields, opts...)
	default:
		return esper.Schema{}, fmt.Errorf("unsupported expr-core-type-name representation %q", representation)
	}
}

func decodeExprCoreTypeNamePayload(root esper.Schema, step compat.Step) (any, error) {
	var payload struct {
		Shape string `json:"shape"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return nil, fmt.Errorf("expr-core-type-name: decode payload: %w", err)
	}
	if payload.Shape != "empty" && payload.Shape != "inside" && payload.Shape != "insidearr" {
		return nil, fmt.Errorf("expr-core-type-name: unsupported shape %q", payload.Shape)
	}
	if root.GoType() == reflect.TypeOf(exprCoreTypeNameJSONProvidedRoot{}) {
		value := exprCoreTypeNameJSONProvidedRoot{}
		switch payload.Shape {
		case "inside":
			value.Inside = &exprCoreTypeNameJSONProvidedInner{}
		case "insidearr":
			value.InsideArr = []*exprCoreTypeNameJSONProvidedInner{{}}
		}
		return value, nil
	}
	if root.Kind() == esper.SchemaObjectArray {
		switch payload.Shape {
		case "empty":
			return []any{nil, nil}, nil
		case "inside":
			return []any{map[string]any{}, nil}, nil
		case "insidearr":
			return []any{nil, []map[string]any{}}, nil
		}
	}
	values := map[string]any{}
	switch payload.Shape {
	case "inside":
		values["inside"] = map[string]any{}
	case "insidearr":
		values["insidearr"] = []map[string]any{}
	}
	if root.Kind() == esper.SchemaAvro {
		record, err := esper.NewAvroRecordFromMap(root, values)
		if err != nil {
			return nil, err
		}
		return record, nil
	}
	return values, nil
}
