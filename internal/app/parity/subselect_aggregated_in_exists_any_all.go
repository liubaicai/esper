package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type subselectAggregatedBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type subselectAggregatedValue struct {
	Value int `esper:"value"`
}

type subselectAggregatedIDValue struct {
	ID    string `esper:"id"`
	Value int    `esper:"value"`
}

type subselectAggregatedWindowRow struct {
	Key   string `esper:"key"`
	Anint int    `esper:"anint"`
}

const subselectAggregatedJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var subselectAggregatedJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectAggregatedInExistsAnyAll.java",
}

var (
	subselectAggregatedJavaRuntimeIDs = []string{
		"java-runtime-65400a42c947bd3ae69e",
		"java-runtime-e174f8d39b7f89f46452",
		"java-runtime-ce0cf46523cc5d57751c",
		"java-runtime-8baf27223e966f5f35f5",
		"java-runtime-4af2c34842f5722967c8",
		"java-runtime-66a9022463735d95a070",
		"java-runtime-39015db22ebf36db9755",
		"java-runtime-ef3c5cb21a4dfa2d47da",
		"java-runtime-c447732f0b3c94049f43",
		"java-runtime-0fdce6e02cf49d65c418",
		"java-runtime-86696aa18175de593720",
		"java-runtime-f9bfad593078d0461b08",
		"java-runtime-111a24e93847238a7e3c",
	}
	subselectAggregatedJavaExecutions = []string{
		"EPLSubselectUngroupedWOHavingWIn",
		"EPLSubselectUngroupedWOHavingWRelOpAllAnySome",
		"EPLSubselectUngroupedWOHavingWExists",
		"EPLSubselectUngroupedWHavingWExists",
		"EPLSubselectUngroupedWHavingWIn",
		"EPLSubselectUngroupedWHavingWRelOpAllAnySome",
		"EPLSubselectUngroupedWHavingWEqualsAllAnySome",
		"EPLSubselectGroupedWOHavingWIn",
		"EPLSubselectGroupedWOHavingWEqualsAllAnySome",
		"EPLSubselectGroupedWHavingWIn",
		"EPLSubselectGroupedWHavingWEqualsAllAnySome",
		"EPLSubselectGroupedWOHavingWExists",
		"EPLSubselectGroupedWHavingWExists",
	}
)

// runSubselectAggregatedInExistsAnyAllScenario replays the 13 executions of
// EPLSubselectAggregatedInExistsAnyAll: SupportValueEvent triggers project
// IN/NOT IN, EXISTS/NOT EXISTS and quantified ALL/ANY/SOME comparisons
// against ungrouped and grouped aggregate subselects over
// SupportBean#keepall, with and without having clauses reading
// last/first(theString); the grouped-exists cases read a named window and
// include a fire-and-forget delete-all step.
func runSubselectAggregatedInExistsAnyAllScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"ungrouped-in", "ungrouped-any-all", "ungrouped-exists", "ungrouped-having-exists",
		"ungrouped-having-in", "ungrouped-having-any-all", "ungrouped-having-equals",
		"grouped-in", "grouped-equals", "grouped-having-in", "grouped-having-equals",
		"grouped-exists", "grouped-having-exists"}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("subselect-aggregated-in-exists-any-all scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runSubselectAggregatedCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("subselect-aggregated-in-exists-any-all case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runSubselectAggregatedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[subselectAggregatedBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectAggregatedValue](env, "SupportValueEvent"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectAggregatedIDValue](env, "SupportIdAndValueEvent"); err != nil {
		return compat.Trace{}, err
	}

	inner := esper.From[subselectAggregatedBean](env, "SupportBean").Window(esper.KeepAll()).AsRecord()
	theString := esper.Field[any, string]("theString")
	sum := esper.Sum[int](esper.Field[any, int]("intPrimitive"))
	value := esper.Field[subselectAggregatedValue, int]("value")
	lastNotE1 := esper.NotEqual[string](esper.Last[string](theString), esper.Literal("E1"))
	firstNotE1 := esper.NotEqual[string](esper.First[string](theString), esper.Literal("E1"))
	lastNotE1E3 := esper.Not(esper.Or(
		esper.Equal[string](esper.Last[string](theString), esper.Literal("E1")),
		esper.Equal[string](esper.Last[string](theString), esper.Literal("E3")),
	))
	var selections []esper.Selection
	switch caseName {
	case "ungrouped-in":
		selections = []esper.Selection{
			esper.Alias("c0", esper.SubqueryInWithOptions[int](value, inner, sum)),
			esper.Alias("c1", esper.Not(esper.SubqueryInWithOptions[int](value, inner, sum))),
		}
	case "ungrouped-any-all":
		selections = []esper.Selection{
			esper.Alias("c0", esper.SubqueryAll[int](value, inner, sum, esper.SubqueryLess)),
			esper.Alias("c1", esper.SubqueryAny[int](value, inner, sum, esper.SubqueryLess)),
			esper.Alias("c2", esper.SubquerySome[int](value, inner, sum, esper.SubqueryLess)),
		}
	case "ungrouped-exists":
		selections = []esper.Selection{
			esper.Alias("c0", esper.SubqueryExistsValue[int](inner, sum)),
			esper.Alias("c1", esper.Not(esper.SubqueryExistsValue[int](inner, sum))),
		}
	case "ungrouped-having-exists":
		having := esper.SubqueryHaving(esper.Less[int](sum, esper.Literal(15)))
		selections = []esper.Selection{
			esper.Alias("c0", esper.SubqueryExistsValue[int](inner, sum, having)),
			esper.Alias("c1", esper.Not(esper.SubqueryExistsValue[int](inner, sum, having))),
		}
	case "ungrouped-having-in":
		having := esper.SubqueryHaving(lastNotE1)
		selections = []esper.Selection{
			esper.Alias("c0", esper.SubqueryInWithOptions[int](value, inner, sum, having)),
			esper.Alias("c1", esper.Not(esper.SubqueryInWithOptions[int](value, inner, sum, having))),
		}
	case "ungrouped-having-any-all":
		having := esper.SubqueryHaving(lastNotE1E3)
		selections = []esper.Selection{
			esper.Alias("c0", esper.SubqueryAllWithOptions[int](value, inner, sum, esper.SubqueryLess, having)),
			esper.Alias("c1", esper.SubqueryAnyWithOptions[int](value, inner, sum, esper.SubqueryLess, having)),
			esper.Alias("c2", esper.SubquerySomeWithOptions[int](value, inner, sum, esper.SubqueryLess, having)),
		}
	case "ungrouped-having-equals":
		having := esper.SubqueryHaving(lastNotE1)
		selections = []esper.Selection{
			esper.Alias("c0", esper.SubqueryAllWithOptions[int](value, inner, sum, esper.SubqueryEqual, having)),
			esper.Alias("c1", esper.SubqueryAnyWithOptions[int](value, inner, sum, esper.SubqueryEqual, having)),
			esper.Alias("c2", esper.SubquerySomeWithOptions[int](value, inner, sum, esper.SubqueryEqual, having)),
		}
	case "grouped-in":
		grouped := esper.SubqueryGroupKey(theString)
		selections = []esper.Selection{
			esper.Alias("c0", esper.SubqueryInWithOptions[int](value, inner, sum, grouped)),
			esper.Alias("c1", esper.Not(esper.SubqueryInWithOptions[int](value, inner, sum, grouped))),
		}
	case "grouped-equals":
		grouped := esper.SubqueryGroupKey(theString)
		selections = []esper.Selection{
			esper.Alias("c0", esper.SubqueryAllWithOptions[int](value, inner, sum, esper.SubqueryEqual, grouped)),
			esper.Alias("c1", esper.SubqueryAnyWithOptions[int](value, inner, sum, esper.SubqueryEqual, grouped)),
			esper.Alias("c2", esper.SubquerySomeWithOptions[int](value, inner, sum, esper.SubqueryEqual, grouped)),
		}
	case "grouped-having-in":
		grouped := esper.SubqueryGroupKey(theString)
		having := esper.SubqueryHaving(lastNotE1)
		selections = []esper.Selection{
			esper.Alias("c0", esper.SubqueryInWithOptions[int](value, inner, sum, grouped, having)),
			esper.Alias("c1", esper.Not(esper.SubqueryInWithOptions[int](value, inner, sum, grouped, having))),
		}
	case "grouped-having-equals":
		grouped := esper.SubqueryGroupKey(theString)
		having := esper.SubqueryHaving(firstNotE1)
		selections = []esper.Selection{
			esper.Alias("c0", esper.SubqueryAllWithOptions[int](value, inner, sum, esper.SubqueryEqual, grouped, having)),
			esper.Alias("c1", esper.SubqueryAnyWithOptions[int](value, inner, sum, esper.SubqueryEqual, grouped, having)),
			esper.Alias("c2", esper.SubquerySomeWithOptions[int](value, inner, sum, esper.SubqueryEqual, grouped, having)),
		}
	case "grouped-exists", "grouped-having-exists":
		rowSchema, err := esper.RegisterStruct[subselectAggregatedWindowRow](env, "MyWindowRow")
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindow", rowSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		insertPlan, err := env.Build(esper.OnEvent(esper.From[subselectAggregatedIDValue](env, "SupportIdAndValueEvent")).InsertIntoNamedWindow(
			"MyWindow",
			esper.SetColumn("key", esper.Field[subselectAggregatedIDValue, string]("id")),
			esper.SetColumn("anint", esper.Field[subselectAggregatedIDValue, int]("value")),
		).Query(esper.StatementName("insert")))
		if err != nil {
			return compat.Trace{}, err
		}
		window := esper.FromNamedWindow(env, "MyWindow")
		anintSum := esper.Sum[int](esper.Field[any, int]("anint"))
		grouped := esper.SubqueryGroupKey(esper.Field[any, string]("key"))
		options := []esper.SubqueryOption{grouped}
		if caseName == "grouped-having-exists" {
			options = append(options, esper.SubqueryHaving(esper.Less[int](anintSum, esper.Literal(15))))
		}
		selections = []esper.Selection{
			esper.Alias("c0", esper.SubqueryExistsValue[int](window, anintSum, options...)),
			esper.Alias("c1", esper.Not(esper.SubqueryExistsValue[int](window, anintSum, options...))),
		}
		plan, err := env.Build(esper.Select(esper.From[subselectAggregatedValue](env, "SupportValueEvent"),
			selections...).Query(esper.StatementName("s0")))
		if err != nil {
			return compat.Trace{}, err
		}
		engine := esper.NewEngine(env)
		cleanup := true
		defer func() {
			if cleanup {
				_ = engine.Close(context.Background())
			}
		}()
		deploy := func(plan esper.Plan) (*esper.Statement, error) {
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return nil, err
			}
			if len(deployment.Statements()) != 1 {
				return nil, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
			}
			return deployment.Statements()[0], nil
		}
		if _, err := deploy(insertPlan); err != nil {
			return compat.Trace{}, err
		}
		statement, err := deploy(plan)
		if err != nil {
			return compat.Trace{}, err
		}
		fafPlan, err := env.Build(esper.FromNamedWindow(env, "MyWindow").OnDemand().DeleteAll())
		if err != nil {
			return compat.Trace{}, err
		}
		cleanup = false
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatementsAndFaf(ctx, engine, statement, caseScenario, decodeSubselectAggregatedPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown subselect-aggregated statement %q", name)
			}
			return statement, nil
		}, func(step compat.Step) error {
			if _, err := engine.ExecuteFireAndForget(ctx, fafPlan); err != nil {
				return fmt.Errorf("faf delete MyWindow: %w", err)
			}
			return nil
		})
	default:
		return compat.Trace{}, fmt.Errorf("unsupported subselect-aggregated case %q", caseName)
	}

	query := esper.Select(esper.From[subselectAggregatedValue](env, "SupportValueEvent"),
		selections...).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeSubselectAggregatedPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown subselect-aggregated statement %q", name)
		}
		return statement, nil
	})
}

func decodeSubselectAggregatedPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value subselectAggregatedBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportValueEvent":
		var value subselectAggregatedValue
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportValueEvent: %w", err)
		}
		return value, nil
	case "SupportIdAndValueEvent":
		var value subselectAggregatedIDValue
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportIdAndValueEvent: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported subselect-aggregated event type %q", step.EventType)
	}
}
