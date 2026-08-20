package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type exprDTBetweenSupportBean struct {
	LongPrimitive int64  `esper:"longPrimitive"`
	LongBoxed     *int64 `esper:"longBoxed"`
}

type exprDTBetweenStartEndPayload struct {
	Key      string  `json:"key"`
	Start    *string `json:"start"`
	Duration int64   `json:"duration"`
}

type exprDTBetweenDateTimePayload struct {
	Date          *string `json:"date"`
	LongPrimitive int64   `json:"longPrimitive"`
	LongBoxed     *int64  `json:"longBoxed"`
}

type exprDTBetweenSupportBeanPayload struct {
	LongPrimitive int64  `json:"longPrimitive"`
	LongBoxed     *int64 `json:"longBoxed"`
}

const exprDTBetweenJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprDTBetweenJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTBetween.java",
}

var (
	exprDTBetweenJavaRuntimeIDs = []string{
		"java-runtime-6593e0f0cc79ed906f52",
		"java-runtime-5e744f4720368eb6585b",
		"java-runtime-c0d2bebfa077f7e51474",
	}
	exprDTBetweenJavaExecutions = []string{
		"ExprDTBetweenIncludeEndpoints",
		"ExprDTBetweenExcludeEndpoints",
		"ExprDTBetweenTypes",
	}
)

var exprDTBetweenCaseOrder = []string{
	"include-current",
	"include-constants",
	"exclude-long",
	"exclude-util",
	"exclude-cal",
	"exclude-ldt",
	"exclude-zdt",
	"types",
	"nulls",
}

func runExprDTBetweenScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	excludesRun := false
	for _, caseName := range exprDTBetweenCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		if strings.HasPrefix(caseName, "exclude-") {
			if excludesRun {
				continue
			}
			caseTrace, err := runExprDTBetweenExcludeCases(ctx, scenario)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("expr-dt-between exclude cases: %w", err)
			}
			trace.Records = append(trace.Records, caseTrace.Records...)
			excludesRun = true
			continue
		}
		caseTrace, err := runExprDTBetweenCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-dt-between case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("expr-dt-between scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runExprDTBetweenExcludeCases(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterMap(env, "SupportTimeStartEndA", exprDTBetweenStartEndFields()); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterMap(env, "SupportDateTime", exprDTBetweenDateTimeFields()); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[exprDTBetweenSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if err := env.RegisterVariable("VAR_TRUE", true); err != nil {
		return compat.Trace{}, err
	}
	if err := env.RegisterVariable("VAR_FALSE", false); err != nil {
		return compat.Trace{}, err
	}
	input := esper.From[map[string]any](env, "SupportTimeStartEndA")
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range exprDTBetweenCaseOrder {
		if !strings.HasPrefix(caseName, "exclude-") || !scenarioHasCase(scenario, caseName) {
			continue
		}
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		plan, err := buildExprDTBetweenStartEndPlan(env, input, caseName, false)
		if err != nil {
			return compat.Trace{}, err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return compat.Trace{}, err
		}
		statements := deployment.Statements()
		if len(statements) != 1 {
			return compat.Trace{}, fmt.Errorf("expected one expr-dt-between statement, got %d", len(statements))
		}
		statement := statements[0]
		before, after, err := splitExprDTBetweenDeployment(caseScenario)
		if err != nil {
			return compat.Trace{}, err
		}
		first, err := compat.ReplayWithStatements(ctx, engine, statement, before, decodeExprDTBetweenPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown expr-dt-between statement %q", name)
			}
			return statement, nil
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return compat.Trace{}, err
		}
		constantPlan, err := buildExprDTBetweenStartEndPlan(env, input, caseName, true)
		if err != nil {
			return compat.Trace{}, err
		}
		constantDeployment, err := engine.Deploy(ctx, constantPlan)
		if err != nil {
			return compat.Trace{}, err
		}
		constantStatements := constantDeployment.Statements()
		if len(constantStatements) != 1 {
			return compat.Trace{}, fmt.Errorf("expected one expr-dt-between constants statement, got %d", len(constantStatements))
		}
		constantStatement := constantStatements[0]
		second, err := compat.ReplayWithStatements(ctx, engine, constantStatement, after, decodeExprDTBetweenPayload, func(name string) (*esper.Statement, error) {
			if name != constantStatement.Name() {
				return nil, fmt.Errorf("unknown expr-dt-between constants statement %q", name)
			}
			return constantStatement, nil
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if err := constantDeployment.Undeploy(ctx); err != nil {
			return compat.Trace{}, err
		}
		trace.Records = append(trace.Records, first.Records...)
		trace.Records = append(trace.Records, second.Records...)
	}
	return trace, nil
}

func runExprDTBetweenCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterMap(env, "SupportTimeStartEndA", exprDTBetweenStartEndFields()); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterMap(env, "SupportDateTime", exprDTBetweenDateTimeFields()); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[exprDTBetweenSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if caseName == "exclude-long" || caseName == "exclude-util" || caseName == "exclude-cal" || caseName == "exclude-ldt" || caseName == "exclude-zdt" || caseName == "nulls" {
		if err := env.RegisterVariable("VAR_TRUE", true); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("VAR_FALSE", false); err != nil {
			return compat.Trace{}, err
		}
	}
	if caseName == "nulls" {
		if err := env.RegisterVariable("VAR_NULL", nil); err != nil {
			return compat.Trace{}, err
		}
	}

	input := esper.From[map[string]any](env, "SupportTimeStartEndA")
	var plan esper.Plan
	if caseName == "types" {
		plan, err = buildExprDTBetweenTypesPlan(env)
	} else {
		plan, err = buildExprDTBetweenStartEndPlan(env, input, caseName, caseName == "include-constants")
	}
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one expr-dt-between statement, got %d", len(statements))
	}
	statement := statements[0]
	if caseName != "exclude-long" && caseName != "exclude-util" && caseName != "exclude-cal" && caseName != "exclude-ldt" && caseName != "exclude-zdt" {
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeExprDTBetweenPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown expr-dt-between statement %q", name)
			}
			return statement, nil
		})
	}

	before, after, err := splitExprDTBetweenDeployment(caseScenario)
	if err != nil {
		return compat.Trace{}, err
	}
	first, err := compat.ReplayWithStatements(ctx, engine, statement, before, decodeExprDTBetweenPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown expr-dt-between statement %q", name)
		}
		return statement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	if err := deployment.Undeploy(ctx); err != nil {
		return compat.Trace{}, err
	}
	constantPlan, err := buildExprDTBetweenStartEndPlan(env, input, caseName, true)
	if err != nil {
		return compat.Trace{}, err
	}
	constantDeployment, err := engine.Deploy(ctx, constantPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	constantStatements := constantDeployment.Statements()
	if len(constantStatements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one expr-dt-between constants statement, got %d", len(constantStatements))
	}
	second, err := compat.ReplayWithStatements(ctx, engine, constantStatements[0], after, decodeExprDTBetweenPayload, func(name string) (*esper.Statement, error) {
		if name != constantStatements[0].Name() {
			return nil, fmt.Errorf("unknown expr-dt-between constants statement %q", name)
		}
		return constantStatements[0], nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	first.Records = append(first.Records, second.Records...)
	return first, nil
}

func exprDTBetweenStartEndFields() []esper.FieldSpec {
	fields := []esper.FieldSpec{esper.FieldDef("key", reflect.TypeOf(""))}
	for _, name := range []string{
		"longdateStart", "longdateEnd",
	} {
		fields = append(fields, esper.OptionalFieldDef(name, reflect.TypeOf(int64(0))))
	}
	for _, name := range []string{
		"utildateStart", "caldateStart", "ldtStart", "zdtStart",
		"utildateEnd", "caldateEnd", "ldtEnd", "zdtEnd",
	} {
		fields = append(fields, esper.OptionalFieldDef(name, reflect.TypeOf(time.Time{})))
	}
	return fields
}

func exprDTBetweenDateTimeFields() []esper.FieldSpec {
	fields := make([]esper.FieldSpec, 0, 7)
	for _, name := range []string{"longdate", "utildate", "caldate", "localdate", "zoneddate"} {
		fields = append(fields, esper.OptionalFieldDef(name, reflect.TypeOf(time.Time{})))
	}
	fields = append(fields,
		esper.FieldDef("longPrimitive", reflect.TypeOf(int64(0))),
		esper.OptionalFieldDef("longBoxed", reflect.TypeOf(int64(0))),
	)
	return fields
}

func buildExprDTBetweenStartEndPlan(env *esper.Environment, input esper.Stream[map[string]any], caseName string, constants bool) (esper.Plan, error) {
	current := esper.CurrentTimestamp()
	if constants {
		if strings.HasPrefix(caseName, "exclude-") {
			lower := esper.Literal(time.Date(2002, 5, 30, 9, 0, 0, 0, time.UTC))
			upper := esper.Literal(time.Date(2002, 5, 30, 9, 1, 0, 0, time.UTC))
			trueValue := esper.Literal(true)
			falseValue := esper.Literal(false)
			value := esper.Expr(esper.Field[map[string]any, int64]("longdateStart"))
			return env.Build(esper.Select(input,
				esper.Alias("val0", esper.DateTimeBetweenWithEndpoints(value, lower, upper, trueValue, trueValue)),
				esper.Alias("val1", esper.DateTimeBetweenWithEndpoints(value, lower, upper, trueValue, falseValue)),
				esper.Alias("val2", esper.DateTimeBetweenWithEndpoints(value, lower, upper, falseValue, trueValue)),
				esper.Alias("val3", esper.DateTimeBetweenWithEndpoints(value, lower, upper, falseValue, falseValue)),
			).Query(esper.StatementName("s0")))
		}
		lower := esper.Literal(time.Date(2002, 5, 30, 9, 0, 0, 0, time.UTC))
		upper := esper.Literal(time.Date(2002, 5, 30, 9, 1, 0, 0, time.UTC))
		query := esper.Select(input,
			esper.Alias("val0", esper.DateTimeBetween(esper.Field[map[string]any, int64]("longdateStart"), lower, upper)),
			esper.Alias("val1", esper.DateTimeBetween(esper.Field[map[string]any, time.Time]("utildateStart"), lower, upper)),
			esper.Alias("val2", esper.DateTimeBetween(esper.Field[map[string]any, time.Time]("caldateStart"), lower, upper)),
			esper.Alias("val3", esper.DateTimeBetween(esper.Field[map[string]any, time.Time]("ldtStart"), lower, upper)),
			esper.Alias("val4", esper.DateTimeBetween(esper.Field[map[string]any, time.Time]("zdtStart"), lower, upper)),
			esper.Alias("val5", esper.DateTimeBetween(esper.Field[map[string]any, int64]("longdateStart"), upper, lower)),
		).Query(esper.StatementName("s0"))
		return env.Build(query)
	}

	longStart := esper.Field[map[string]any, int64]("longdateStart")
	longEnd := esper.Field[map[string]any, int64]("longdateEnd")
	utilStart := esper.Field[map[string]any, time.Time]("utildateStart")
	utilEnd := esper.Field[map[string]any, time.Time]("utildateEnd")
	calStart := esper.Field[map[string]any, time.Time]("caldateStart")
	calEnd := esper.Field[map[string]any, time.Time]("caldateEnd")
	ldtStart := esper.Field[map[string]any, time.Time]("ldtStart")
	ldtEnd := esper.Field[map[string]any, time.Time]("ldtEnd")
	zdtStart := esper.Field[map[string]any, time.Time]("zdtStart")
	zdtEnd := esper.Field[map[string]any, time.Time]("zdtEnd")
	if caseName == "include-current" {
		return env.Build(esper.Select(input,
			esper.Alias("val0", esper.DateTimeAfter(current, longStart)),
			esper.Alias("val1", esper.DateTimeBetween(current, longStart, longEnd)),
			esper.Alias("val2", esper.DateTimeBetween(current, utilStart, calEnd)),
			esper.Alias("val3", esper.DateTimeBetween(current, calStart, utilEnd)),
			esper.Alias("val4", esper.DateTimeBetween(current, utilStart, utilEnd)),
			esper.Alias("val5", esper.DateTimeBetween(current, calStart, calEnd)),
			esper.Alias("val6", esper.DateTimeBetween(current, calEnd, calStart)),
			esper.Alias("val7", esper.DateTimeBetween(current, ldtStart, ldtEnd)),
			esper.Alias("val8", esper.DateTimeBetween(current, zdtStart, zdtEnd)),
		).Query(esper.StatementName("s0")))
	}

	var start, end esper.Expr = longStart, longEnd
	switch caseName {
	case "exclude-util":
		start, end = utilStart, utilEnd
	case "exclude-cal":
		start, end = calStart, calEnd
	case "exclude-ldt":
		start, end = ldtStart, ldtEnd
	case "exclude-zdt":
		start, end = zdtStart, zdtEnd
	case "nulls":
		return env.Build(esper.Select(input,
			esper.Alias("val0", esper.DateTimeBetweenWithEndpoints(esper.CurrentTimestamp(), longStart, longEnd, esper.VariableRef[bool]("VAR_NULL"), esper.Literal(true))),
			esper.Alias("val1", esper.DateTimeBetween(esper.CurrentTimestamp(), longStart, longEnd)),
		).Query(esper.StatementName("s0")))
	case "exclude-long":
	default:
		return esper.Plan{}, fmt.Errorf("unsupported expr-dt-between start/end case %q", caseName)
	}
	trueValue := esper.Literal(true)
	falseValue := esper.Literal(false)
	varTrue := esper.VariableRef[bool]("VAR_TRUE")
	varFalse := esper.VariableRef[bool]("VAR_FALSE")
	return env.Build(esper.Select(input,
		esper.Alias("val0", esper.DateTimeBetweenWithEndpoints(current, start, end, trueValue, trueValue)),
		esper.Alias("val1", esper.DateTimeBetweenWithEndpoints(current, start, end, trueValue, falseValue)),
		esper.Alias("val2", esper.DateTimeBetweenWithEndpoints(current, start, end, falseValue, trueValue)),
		esper.Alias("val3", esper.DateTimeBetweenWithEndpoints(current, start, end, falseValue, falseValue)),
		esper.Alias("val4", esper.DateTimeBetweenWithEndpoints(current, start, end, varTrue, varTrue)),
		esper.Alias("val5", esper.DateTimeBetweenWithEndpoints(current, start, end, varTrue, varFalse)),
		esper.Alias("val6", esper.DateTimeBetweenWithEndpoints(current, start, end, varFalse, varTrue)),
		esper.Alias("val7", esper.DateTimeBetweenWithEndpoints(current, start, end, varFalse, varFalse)),
	).Query(esper.StatementName("s0")))
}

func buildExprDTBetweenTypesPlan(env *esper.Environment) (esper.Plan, error) {
	dateInput := esper.From[map[string]any](env, "SupportDateTime")
	beanInput := esper.From[exprDTBetweenSupportBean](env, "SupportBean").Window(esper.LastEvent())
	primitive := esper.JoinField[int64](1, "longPrimitive")
	boxed := esper.JoinField[*int64](1, "longBoxed")
	return env.Build(esper.Join(dateInput, beanInput).Unidirectional(esper.JoinLeft).Select(
		esper.SelectFrom(0, "c0", esper.DateTimeBetween(esper.JoinField[*time.Time](0, "longdate"), primitive, boxed)),
		esper.SelectFrom(0, "c1", esper.DateTimeBetween(esper.JoinField[*time.Time](0, "utildate"), primitive, boxed)),
		esper.SelectFrom(0, "c2", esper.DateTimeBetween(esper.JoinField[*time.Time](0, "caldate"), primitive, boxed)),
		esper.SelectFrom(0, "c3", esper.DateTimeBetween(esper.JoinField[*time.Time](0, "localdate"), primitive, boxed)),
		esper.SelectFrom(0, "c4", esper.DateTimeBetween(esper.JoinField[*time.Time](0, "zoneddate"), primitive, boxed)),
	).Query(esper.StatementName("s0")))
}

func splitExprDTBetweenDeployment(scenario compat.Scenario) (compat.Scenario, compat.Scenario, error) {
	before := compat.Scenario{Version: scenario.Version, ID: scenario.ID}
	after := compat.Scenario{Version: scenario.Version, ID: scenario.ID}
	seenDeploy := false
	for _, step := range scenario.Steps {
		if step.Op == "deploy" {
			seenDeploy = true
			continue
		}
		if !seenDeploy {
			before.Steps = append(before.Steps, step)
		} else {
			after.Steps = append(after.Steps, step)
		}
	}
	if !seenDeploy {
		return compat.Scenario{}, compat.Scenario{}, fmt.Errorf("expr-dt-between scenario has no constants deployment marker")
	}
	if len(before.Steps) == 0 || before.Steps[0].Op != "case" {
		return compat.Scenario{}, compat.Scenario{}, fmt.Errorf("expr-dt-between deployment split lost case marker")
	}
	after.Steps = append([]compat.Step{{Op: "case", Case: before.Steps[0].Case}}, after.Steps...)
	if err := before.Validate(); err != nil {
		return compat.Scenario{}, compat.Scenario{}, err
	}
	if err := after.Validate(); err != nil {
		return compat.Scenario{}, compat.Scenario{}, err
	}
	return before, after, nil
}

func decodeExprDTBetweenPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportTimeStartEndA":
		var payload exprDTBetweenStartEndPayload
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportTimeStartEndA: %w", err)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(step.Payload, &raw); err != nil {
			return nil, fmt.Errorf("decode SupportTimeStartEndA presence: %w", err)
		}
		event := map[string]any{"key": payload.Key}
		startValue, hasStart := raw["start"]
		if !hasStart {
			return event, nil
		}
		if bytes.Equal(bytes.TrimSpace(startValue), []byte("null")) {
			for _, name := range []string{
				"longdateStart", "longdateEnd", "utildateStart", "caldateStart", "ldtStart", "zdtStart",
				"utildateEnd", "caldateEnd", "ldtEnd", "zdtEnd",
			} {
				event[name] = nil
			}
			return event, nil
		}
		if payload.Start == nil {
			return nil, fmt.Errorf("decode SupportTimeStartEndA: start must be a string or null")
		}
		start, err := time.Parse(time.RFC3339Nano, *payload.Start)
		if err != nil {
			return nil, fmt.Errorf("parse SupportTimeStartEndA start: %w", err)
		}
		end := start.Add(time.Duration(payload.Duration) * time.Millisecond)
		event["longdateStart"] = start.UnixMilli()
		event["utildateStart"] = start
		event["caldateStart"] = start
		event["ldtStart"] = start
		event["zdtStart"] = start
		event["longdateEnd"] = end.UnixMilli()
		event["utildateEnd"] = end
		event["caldateEnd"] = end
		event["ldtEnd"] = end
		event["zdtEnd"] = end
		return event, nil
	case "SupportDateTime":
		var payload exprDTBetweenDateTimePayload
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportDateTime: %w", err)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(step.Payload, &raw); err != nil {
			return nil, fmt.Errorf("decode SupportDateTime presence: %w", err)
		}
		event := map[string]any{"longPrimitive": payload.LongPrimitive, "longBoxed": payload.LongBoxed}
		dateValue, hasDate := raw["date"]
		if !hasDate {
			return event, nil
		}
		if bytes.Equal(bytes.TrimSpace(dateValue), []byte("null")) {
			for _, name := range []string{"longdate", "utildate", "caldate", "localdate", "zoneddate"} {
				event[name] = nil
			}
			return event, nil
		}
		if payload.Date == nil {
			return nil, fmt.Errorf("decode SupportDateTime: date must be a string or null")
		}
		date, err := time.Parse(time.RFC3339Nano, *payload.Date)
		if err != nil {
			return nil, fmt.Errorf("parse SupportDateTime date: %w", err)
		}
		event["longdate"] = date
		event["utildate"] = date
		event["caldate"] = date
		event["localdate"] = date
		event["zoneddate"] = date
		return event, nil
	case "SupportBean":
		var payload exprDTBetweenSupportBeanPayload
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return exprDTBetweenSupportBean{LongPrimitive: payload.LongPrimitive, LongBoxed: payload.LongBoxed}, nil
	default:
		return nil, fmt.Errorf("expr-dt-between: unsupported event type %q", step.EventType)
	}
}
