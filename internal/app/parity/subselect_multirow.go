package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Direct EPLSubselectMultirow uses a separate runner from the aggregated
// multirow family. The Java execution deliberately redeploys s0 while a
// named length window remains populated, so the first case is replayed in two
// phases against one engine.
type subselectDirectMultirowS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
}

type subselectDirectMultirowBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

var subselectDirectMultirowJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectMultirow.java",
}

var subselectDirectMultirowJavaRuntimeIDs = []string{
	"java-runtime-29c2087cc4243e9b7a50",
	"java-runtime-64eb1701d14bdbcefc86",
}

var subselectDirectMultirowJavaExecutions = []string{
	"EPLSubselectMultirowSingleColumn",
	"EPLSubselectMultirowUnderlyingCorrelated",
}

func runSubselectDirectMultirowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"multirow-single-column", "multirow-underlying-correlated"}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		caseTrace, err := runSubselectDirectMultirowCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("subselect-multirow case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("subselect-multirow scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runSubselectDirectMultirowCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[subselectDirectMultirowS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[subselectDirectMultirowBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	beanSchema, ok := env.Schema("SupportBean")
	if !ok {
		return compat.Trace{}, fmt.Errorf("SupportBean schema is missing")
	}
	if _, err := esper.CreateNamedWindow(env, "SupportWindow", beanSchema,
		esper.NamedWindowRetention(esper.LengthWindow(3))); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	if caseName == "multirow-single-column" {
		return runSubselectDirectMultirowSingleColumn(ctx, env, engine, caseScenario)
	}
	return runSubselectDirectMultirowUnderlying(ctx, env, engine, caseScenario)
}

func runSubselectDirectMultirowSingleColumn(ctx context.Context, env *esper.Environment, engine *esper.Engine, scenario compat.Scenario) (compat.Trace, error) {
	producerPlan, err := env.Build(esper.OnEvent(esper.From[subselectDirectMultirowBean](env, "SupportBean")).
		InsertIntoNamedWindow("SupportWindow",
			esper.SetColumn("theString", esper.Field[subselectDirectMultirowBean, string]("theString")),
			esper.SetColumn("intPrimitive", esper.Field[subselectDirectMultirowBean, int]("intPrimitive")),
		).Query(esper.StatementName("producer")))
	if err != nil {
		return compat.Trace{}, err
	}
	producerDeployment, err := engine.Deploy(ctx, producerPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = producerDeployment.Undeploy(context.Background()) }()

	beanValues := esper.Field[any, int]("intPrimitive")
	buildQuery := func(inner esper.RecordStream) (esper.Plan, error) {
		return env.Build(esper.Select(
			esper.From[subselectDirectMultirowS0](env, "SupportBean_S0"),
			esper.Alias("p00", esper.Field[subselectDirectMultirowS0, *string]("p00")),
			esper.Alias("val", esper.SubqueryValue[[]int](inner, esper.WindowValues[int](beanValues))),
		).Query(esper.StatementName("s0")))
	}

	directInner := esper.From[subselectDirectMultirowBean](env, "SupportBean").Window(esper.KeepAll()).AsRecord()
	directPlan, err := buildQuery(directInner)
	if err != nil {
		return compat.Trace{}, err
	}
	directDeployment, err := engine.Deploy(ctx, directPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	if len(directDeployment.Statements()) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one direct s0 statement")
	}
	firstSteps, secondSteps, err := splitSubselectDirectMultirowSingleColumnSteps(scenario)
	if err != nil {
		return compat.Trace{}, err
	}
	firstScenario := compat.Scenario{Version: scenario.Version, ID: scenario.ID, Steps: firstSteps}
	firstTrace, err := compat.ReplayWithStatements(ctx, engine, directDeployment.Statements()[0], firstScenario,
		decodeSubselectDirectMultirowPayload, func(name string) (*esper.Statement, error) {
			if name != "s0" {
				return nil, fmt.Errorf("unknown direct multirow statement %q", name)
			}
			return directDeployment.Statements()[0], nil
		})
	if err != nil {
		return compat.Trace{}, err
	}
	if err := directDeployment.Undeploy(ctx); err != nil {
		return compat.Trace{}, err
	}

	namedPlan, err := buildQuery(esper.FromNamedWindow(env, "SupportWindow"))
	if err != nil {
		return compat.Trace{}, err
	}
	namedDeployment, err := engine.Deploy(ctx, namedPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	if len(namedDeployment.Statements()) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one late s0 statement")
	}
	secondScenario := compat.Scenario{Version: scenario.Version, ID: scenario.ID, Steps: secondSteps}
	secondTrace, err := compat.ReplayWithStatements(ctx, engine, namedDeployment.Statements()[0], secondScenario,
		decodeSubselectDirectMultirowPayload, func(name string) (*esper.Statement, error) {
			if name != "s0" {
				return nil, fmt.Errorf("unknown late direct multirow statement %q", name)
			}
			return namedDeployment.Statements()[0], nil
		})
	if err != nil {
		return compat.Trace{}, err
	}
	if err := namedDeployment.Undeploy(ctx); err != nil {
		return compat.Trace{}, err
	}
	return mergeSubselectDirectMultirowTraces(firstTrace, secondTrace), nil
}

func splitSubselectDirectMultirowSingleColumnSteps(scenario compat.Scenario) ([]compat.Step, []compat.Step, error) {
	caseName := "multirow-single-column"
	var sends []compat.Step
	active := false
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			active = step.Case == caseName
			continue
		}
		if active {
			sends = append(sends, step)
		}
	}
	if len(sends) != 8 {
		return nil, nil, fmt.Errorf("%s has %d sends, want 8", caseName, len(sends))
	}
	first := []compat.Step{{Op: "case", Case: caseName}}
	first = append(first, sends[:5]...)
	second := []compat.Step{{Op: "case", Case: caseName}}
	second = append(second, sends[5:]...)
	return first, second, nil
}

func mergeSubselectDirectMultirowTraces(first, second compat.Trace) compat.Trace {
	merged := compat.Trace{Version: first.Version, ID: first.ID}
	merged.Records = append(merged.Records, first.Records...)
	merged.Records = append(merged.Records, second.Records...)
	for index := range merged.Records {
		merged.Records[index].Case = "multirow-single-column"
		merged.Records[index].Sequence = uint64(index + 1)
	}
	return merged
}

func runSubselectDirectMultirowUnderlying(ctx context.Context, env *esper.Environment, engine *esper.Engine, scenario compat.Scenario) (compat.Trace, error) {
	inner := esper.From[subselectDirectMultirowBean](env, "SupportBean").Window(esper.KeepAll()).AsRecord()
	key := esper.Field[any, string]("theString")
	query := esper.Select(
		esper.From[subselectDirectMultirowS0](env, "SupportBean_S0"),
		esper.Alias("p00", esper.Field[subselectDirectMultirowS0, *string]("p00")),
		esper.Alias("val", esper.SubqueryValueWithOptions[[]esper.Event](inner, esper.WindowEvents(),
			esper.SubqueryWhere(esper.Equal[string](key, esper.OuterField[string]("p00"))))),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one direct correlated statement, got %d", len(statements))
	}
	statement := statements[0]
	trace, err := compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeSubselectDirectMultirowPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown direct multirow statement %q", name)
		}
		return statement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	return normalizeSubselectDirectMultirowUnderlyingTrace(trace), nil
}

func normalizeSubselectDirectMultirowUnderlyingTrace(trace compat.Trace) compat.Trace {
	for recordIndex := range trace.Records {
		record := &trace.Records[recordIndex]
		if record.Case != "multirow-underlying-correlated" {
			continue
		}
		normalizeResults := func(results []compat.ResultRecord) {
			for resultIndex := range results {
				rows, ok := results[resultIndex].Fields["val"].([]any)
				if !ok || len(rows) < 2 {
					continue
				}
				sort.SliceStable(rows, func(left, right int) bool {
					leftJSON, _ := json.Marshal(rows[left])
					rightJSON, _ := json.Marshal(rows[right])
					return string(leftJSON) < string(rightJSON)
				})
			}
		}
		normalizeResults(record.New)
		normalizeResults(record.Old)
	}
	return trace
}

func decodeSubselectDirectMultirowPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean_S0":
		var value subselectDirectMultirowS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("subselect-multirow SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value subselectDirectMultirowBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("subselect-multirow SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("subselect-multirow: unsupported event type %q", step.EventType)
	}
}
