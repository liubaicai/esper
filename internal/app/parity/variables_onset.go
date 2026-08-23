package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type variablesOnsetSupportA struct {
	ID string `esper:"id"`
}

type variablesOnsetBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
	IntBoxed     *int32 `esper:"intBoxed"`
}

var variablesOnsetJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/EPLVariablesOnSet.java",
}

var (
	variablesOnsetJavaRuntimeIDs = []string{
		"java-runtime-4d17a2ee72e2f62f484f",
		"java-runtime-52902bb58deb8ee9f48b",
		"java-runtime-a1c711b411766252f401",
		"java-runtime-e7b63e853b22431a1ef3",
		"java-runtime-275279390f721078251a",
		"java-runtime-ce691b2f1d8368144c89",
	}
	variablesOnsetJavaExecutions = []string{
		"EPLVariableOnSetSimple",
		"EPLVariableOnSetWithFilter",
		"EPLVariableOnSetAssignmentOrderNoDup",
		"EPLVariableOnSetAssignmentOrderDup",
		"EPLVariableOnSetRuntimeOrderMultiple",
		"EPLVariableOnSetCoercion",
	}
)

// runVariablesOnsetScenario replays on-trigger variable assignment scenarios:
// ordered and duplicate assignments, numeric coercion, filtered triggers, and
// dependent select statements. EPLVariableOnSetInvalid is a compile-time
// error-only execution registered as an approved difference.
func runVariablesOnsetScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"onset-simple", "onset-with-filter",
		"onset-order-no-dup", "onset-order-dup",
		"onset-runtime-order-multiple", "onset-coercion",
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runVariablesOnsetCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("variables-onset case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("variables-onset scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runVariablesOnsetCase(ctx context.Context, caseScenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[variablesOnsetBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[variablesUseS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[variablesOnsetSupportA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}

	intPrimitive := esper.Field[variablesOnsetBean, int32]("intPrimitive")
	intBoxed := esper.Field[variablesOnsetBean, int32]("intBoxed")
	theString := esper.Field[variablesOnsetBean, string]("theString")

	var queries []esper.Query
	initialSnapshots := false
	switch caseName {
	case "onset-simple":
		if err := env.RegisterVariable("var_simple_set", true); err != nil {
			return compat.Trace{}, err
		}
		queries = append(queries,
			esper.OnEvent(esper.From[variablesUseS0](env, "SupportBean_S0")).SetVariable(
				"var_simple_set", esper.Literal(false),
			).Query(esper.StatementName("set")),
			esper.Select(esper.From[variablesOnsetBean](env, "SupportBean"),
				esper.Alias("c0", esper.VariableRef[bool]("var_simple_set")),
			).Query(esper.StatementName("s0")),
		)
	case "onset-with-filter":
		if err := env.RegisterVariable("papi_1", "begin"); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("papi_2", true); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("papi_3", "value"); err != nil {
			return compat.Trace{}, err
		}
		queries = append(queries,
			esper.OnEvent(esper.From[variablesOnsetBean](env, "SupportBean").Filter(
				esper.Like(theString, esper.Literal("S%")),
			)).SetVariables(
				esper.SetVariableExpr("papi_1", esper.Literal("end")),
				esper.SetVariableExpr("papi_2", esper.Literal(false)),
				esper.SetVariableExpr("papi_3", esper.NullLiteral[string]()),
			).Query(esper.StatementName("set")),
		)
		initialSnapshots = true
	case "onset-order-no-dup":
		typed := func(t reflect.Type) esper.VariableOption { return esper.VariableType(t) }
		intType := reflect.TypeOf(int32(0))
		if err := env.RegisterVariable("var1OND", int32(12)); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("var2OND", int32(2)); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("var3OND", nil, typed(intType)); err != nil {
			return compat.Trace{}, err
		}
		_ = intType
		queries = append(queries,
			esper.OnEvent(esper.From[variablesOnsetBean](env, "SupportBean")).SetVariables(
				esper.SetVariableExpr("var1OND", intPrimitive),
				esper.SetVariableExpr("var2OND", esper.Add[int32](esper.VariableRef[int32]("var1OND"), esper.Literal[int32](1))),
				esper.SetVariableExpr("var3OND", esper.Add[int32](
					esper.VariableRef[int32]("var1OND"),
					esper.VariableRef[int32]("var2OND"))),
			).Query(esper.StatementName("set")),
		)
		initialSnapshots = true
	case "onset-order-dup":
		if err := env.RegisterVariable("var1OD", int32(0)); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("var2OD", int32(1)); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("var3OD", int32(2)); err != nil {
			return compat.Trace{}, err
		}
		queries = append(queries,
			esper.OnEvent(esper.From[variablesOnsetBean](env, "SupportBean")).SetVariables(
				esper.SetVariableExpr("var1OD", intPrimitive),
				esper.SetVariableExpr("var2OD", esper.VariableRef[int32]("var2OD")),
				esper.SetVariableExpr("var1OD", intBoxed),
				esper.SetVariableExpr("var3OD", esper.Add[int32](
					esper.VariableRef[int32]("var3OD"),
					esper.Literal[int32](1))),
			).Query(esper.StatementName("set")),
		)
		initialSnapshots = true
	case "onset-runtime-order-multiple":
		typed := func(t reflect.Type) esper.VariableOption { return esper.VariableType(t) }
		intType := reflect.TypeOf(int32(0))
		if err := env.RegisterVariable("var1ROM", nil, typed(intType)); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("var2ROM", int32(1)); err != nil {
			return compat.Trace{}, err
		}
		queries = append(queries,
			esper.OnEvent(esper.From[variablesOnsetBean](env, "SupportBean").Filter(
				esper.Or(
					esper.Like(theString, esper.Literal("S%")),
					esper.Like(theString, esper.Literal("B%")),
				),
			)).SetVariables(
				esper.SetVariableExpr("var1ROM", intPrimitive),
				esper.SetVariableExpr("var2ROM", intBoxed),
			).Query(esper.StatementName("set")),
			esper.Select(
				esper.From[variablesOnsetBean](env, "SupportBean").Filter(
					esper.Or(
						esper.Like(theString, esper.Literal("E%")),
						esper.Like(theString, esper.Literal("B%")),
					),
				),
				esper.Alias("var1ROM", esper.VariableRef[int32]("var1ROM")),
				esper.Alias("var2ROM", esper.VariableRef[int32]("var2ROM")),
				esper.Alias("theString", theString),
			).Query(esper.StatementName("s0")),
		)
		initialSnapshots = true
	case "onset-coercion":
		typed := func(t reflect.Type) esper.VariableOption { return esper.VariableType(t) }
		if err := env.RegisterVariable("var1COE", nil, typed(reflect.TypeOf(float32(0)))); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("var2COE", nil, typed(reflect.TypeOf(float64(0)))); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("var3COE", nil, typed(reflect.TypeOf(int64(0)))); err != nil {
			return compat.Trace{}, err
		}
		aID := esper.Field[variablesOnsetSupportA, string]("id")
		queries = append(queries,
			esper.OnEvent(esper.From[variablesOnsetBean](env, "SupportBean")).SetVariables(
				esper.SetVariableExpr("var1COE", intPrimitive),
				esper.SetVariableExpr("var2COE", intPrimitive),
				esper.SetVariableExpr("var3COE", intBoxed),
			).Query(esper.StatementName("set")),
			esper.Select(
				esper.From[variablesOnsetSupportA](env, "SupportBean_A").Window(esper.LengthWindow(2)),
				esper.Alias("var1COE", esper.VariableRef[float32]("var1COE")),
				esper.Alias("var2COE", esper.VariableRef[float64]("var2COE")),
				esper.Alias("var3COE", esper.VariableRef[int64]("var3COE")),
				esper.Alias("id", aID),
			).Query(
				esper.StatementName("s0"),
				esper.WithOldStream(),
			),
		)
		initialSnapshots = true
	default:
		return compat.Trace{}, fmt.Errorf("unsupported variables-onset case %q", caseName)
	}

	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	statements := make(map[string]*esper.Statement)
	for _, query := range queries {
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return compat.Trace{}, err
		}
		for _, statement := range deployment.Statements() {
			statements[statement.Name()] = statement
		}
	}

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seq := uint64(0)
	record := func(operation, statementName string, batch esper.ResultBatch) {
		seq++
		rec := compat.TraceRecord{
			Case:      caseName,
			Operation: operation,
			Statement: statementName,
			Time:      currentTimeString(engine),
		}
		if operation == "snapshot" || operation == "listener" {
			rec.Sequence = seq
		}
		for _, row := range batch.New {
			fields, err := variablesOnsetSurfaceRow(row)
			if err != nil {
				panic(err) // subscription contract: no error path in fixtures
			}
			rec.New = append(rec.New, compat.ResultRecord{Kind: "row", Fields: fields})
		}
		for _, row := range batch.Old {
			fields, err := variablesOnsetSurfaceRow(row)
			if err != nil {
				panic(err)
			}
			rec.Old = append(rec.Old, compat.ResultRecord{Kind: "row", Fields: fields})
		}
		trace.Records = append(trace.Records, rec)
	}
	for _, statement := range statements {
		statement := statement
		if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			record("listener", statement.Name(), batch)
			return nil
		}); err != nil {
			return trace, err
		}
	}

	if initialSnapshots {
		setStatement := statements["set"]
		if setStatement == nil {
			return trace, fmt.Errorf("variables-onset: set statement missing for %q", caseName)
		}
		result, err := setStatement.Snapshot(ctx)
		if err != nil {
			return trace, err
		}
		seq++
		rec := compat.TraceRecord{
			Case:      caseName,
			Operation: "snapshot",
			Statement: "set",
			Time:      currentTimeString(engine),
			Sequence:  0,
		}
		for _, row := range result.Batch.New {
			fields := make(map[string]any)
			appendVariableSurfaceFields(fields, row)
			rec.New = append(rec.New, compat.ResultRecord{Kind: "row", Fields: fields})
		}
		trace.Records = append(trace.Records, rec)
		seq--
	}

	for _, step := range caseScenario.Steps {
		if step.Op == "case" {
			continue
		}
		if step.Op != "send" {
			return trace, fmt.Errorf("unsupported variables-onset step op %q", step.Op)
		}
		payload, err := decodeVariablesOnsetPayload(step)
		if err != nil {
			return trace, err
		}
		if err := engine.Send(ctx, step.EventType, payload); err != nil {
			return trace, err
		}
	}
	return trace, nil
}

// appendVariableSurfaceFields mirrors the oracle's iterator projection:
// numbers collapse to their long value, other scalars stringify.
func appendVariableSurfaceFields(fields map[string]any, result esper.Result) {
	collect := func(name string, value esper.Value) {
		raw := value.Any()
		switch typed := raw.(type) {
		case nil:
			fields[name] = nil
		case int:
			fields[name] = int64(typed)
		case int8:
			fields[name] = int64(typed)
		case int16:
			fields[name] = int64(typed)
		case int32:
			fields[name] = int64(typed)
		case int64:
			fields[name] = typed
		default:
			fields[name] = fmt.Sprintf("%v", raw)
		}
	}
	if event, ok := result.Event(); ok {
		for _, field := range event.Schema().Fields() {
			collect(field.Name, event.Get(field.Name))
		}
		return
	}
	if row, ok := result.Row(); ok {
		for _, field := range row.Schema().Fields() {
			collect(field.Name, row.Get(field.Name))
		}
	}
}

// variablesOnsetSurfaceRow renders listener rows with Java toString semantics;
// snapshot rows pass through the numeric truncation of the oracle's iterator
// projection via the same function (fixtures only carry strings, booleans and
// integers there).
func variablesOnsetSurfaceRow(result esper.Result) (map[string]any, error) {
	fields := make(map[string]any)
	collect := func(name string, value esper.Value) {
		raw := value.Any()
		switch typed := raw.(type) {
		case nil:
			fields[name] = nil
		case float32:
			fields[name] = javaDoubleString(float64(typed))
		case float64:
			fields[name] = javaDoubleString(typed)
		default:
			fields[name] = fmt.Sprintf("%v", raw)
		}
	}
	if event, ok := result.Event(); ok {
		for _, field := range event.Schema().Fields() {
			collect(field.Name, event.Get(field.Name))
		}
		return fields, nil
	}
	row, ok := result.Row()
	if !ok {
		return nil, fmt.Errorf("variables-onset result is neither event nor row")
	}
	for _, field := range row.Schema().Fields() {
		collect(field.Name, row.Get(field.Name))
	}
	return fields, nil
}

func javaDoubleString(v float64) string {
	rendered := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(rendered, ".eE") {
		rendered += ".0"
	}
	return strings.Replace(rendered, "e", "E", 1)
}

func decodeVariablesOnsetPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var event variablesOnsetBean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("variables-onset SupportBean: %w", err)
		}
		return event, nil
	case "SupportBean_S0":
		var event variablesUseS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("variables-onset SupportBean_S0: %w", err)
		}
		return event, nil
	case "SupportBean_A":
		var event variablesOnsetSupportA
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("variables-onset SupportBean_A: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("variables-onset: unsupported event type %q", step.EventType)
	}
}
