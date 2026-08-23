package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type variablesUseBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

type variablesUseS0 struct {
	ID  int32  `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

var variablesUseJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/EPLVariablesUse.java",
}

var (
	variablesUseJavaRuntimeIDs = []string{
		"java-runtime-5a38cfa84dadd7dd6f61",
		"java-runtime-849ebec4996c28823d57",
		"java-runtime-5a91cdbc149502c6fb7a",
	}
	variablesUseJavaExecutions = []string{
		"EPLVariableUseVariableInFilter",
		"EPLVariableUseVariableInFilterBoolean",
		"EPLVariableUseSimpleSameModule",
	}
)

// runVariablesUseScenario replays variable-use scenarios: variables assigned
// by on-set statements and consumed inside equality / or filters.
func runVariablesUseScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"variable-in-filter", "variable-in-filter-boolean", "simple-same-module"}
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
		caseTrace, err := runVariablesUseCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("variables-use case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("variables-use scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runVariablesUseCase(ctx context.Context, caseScenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[variablesUseBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[variablesUseS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}

	var query esper.Query
	switch caseName {
	case "variable-in-filter":
		// Java registers var1IF as String with null initial; the Go runner
		// uses an unreachable sentinel so equality never matches before the
		// first assignment (established precedent).
		if err := env.RegisterVariable("var1IF", "\x00UNSET"); err != nil {
			return compat.Trace{}, err
		}
		p00 := esper.Field[variablesUseS0, string]("p00")
		setQuery := esper.OnEvent(esper.From[variablesUseS0](env, "SupportBean_S0")).SetVariable(
			"var1IF", p00,
		).Query(esper.StatementName("set"))
		theString := esper.Field[variablesUseBean, string]("theString")
		num := esper.Field[variablesUseBean, int32]("intPrimitive")
		query = esper.Select(
			esper.From[variablesUseBean](env, "SupportBean").Filter(
				esper.Equal[string](theString, esper.VariableRef[string]("var1IF")),
			),
			esper.Alias("theString", theString),
			esper.Alias("intPrimitive", num),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{setQuery, query}, caseScenario, caseName)
	case "variable-in-filter-boolean":
		if err := env.RegisterVariable("var1IFB", "\x00UNSET"); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("var2IFB", "\x00UNSET"); err != nil {
			return compat.Trace{}, err
		}
		p00 := esper.Field[variablesUseS0, string]("p00")
		p01 := esper.Field[variablesUseS0, string]("p01")
		setQuery := esper.OnEvent(esper.From[variablesUseS0](env, "SupportBean_S0")).SetVariables(
			esper.SetVariableExpr("var1IFB", p00),
			esper.SetVariableExpr("var2IFB", p01),
		).Query(esper.StatementName("set"))
		theString := esper.Field[variablesUseBean, string]("theString")
		num := esper.Field[variablesUseBean, int32]("intPrimitive")
		query = esper.Select(
			esper.From[variablesUseBean](env, "SupportBean").Filter(
				esper.Or(
					esper.Equal[string](theString, esper.VariableRef[string]("var1IFB")),
					esper.Equal[string](theString, esper.VariableRef[string]("var2IFB")),
				),
			),
			esper.Alias("theString", theString),
			esper.Alias("intPrimitive", num),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{setQuery, query}, caseScenario, caseName)
	case "simple-same-module":
		if err := env.RegisterVariable("var_simple_module_const", true); err != nil {
			return compat.Trace{}, err
		}
		query = esper.Select(
			esper.From[variablesUseBean](env, "SupportBean"),
			esper.Alias("c0", esper.VariableRef[bool]("var_simple_module_const")),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{query}, caseScenario, caseName)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported variables-use case %q", caseName)
	}
}

func replayVariablesUse(ctx context.Context, env *esper.Environment, queries []esper.Query, caseScenario compat.Scenario, caseName string) (compat.Trace, error) {
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	var selectStatement *esper.Statement
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
			if statement.Name() == "s0" {
				selectStatement = statement
			}
		}
	}
	if selectStatement == nil {
		return compat.Trace{}, fmt.Errorf("variables-use: statement s0 not found")
	}

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seq := uint64(0)
	if _, err := selectStatement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		for _, row := range batch.New {
			seq++
			fields := map[string]any{}
			if event, ok := row.Event(); ok {
				for _, field := range event.Schema().Fields() {
					fields[field.Name] = streamSelectorRender(event.Get(field.Name).Any())
				}
			} else if r, ok := row.Row(); ok {
				for _, field := range r.Schema().Fields() {
					fields[field.Name] = streamSelectorRender(r.Get(field.Name).Any())
				}
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "listener",
				Statement: selectStatement.Name(),
				Sequence:  seq,
				Time:      currentTimeString(engine),
				New:       []compat.ResultRecord{{Kind: "row", Fields: fields}},
			})
		}
		return nil
	}); err != nil {
		return trace, err
	}
	for _, step := range caseScenario.Steps {
		if step.Op == "case" {
			continue
		}
		if step.Op != "send" {
			return trace, fmt.Errorf("unsupported variables-use step op %q", step.Op)
		}
		payload, err := decodeVariablesUsePayload(step)
		if err != nil {
			return trace, err
		}
		if err := engine.Send(ctx, step.EventType, payload); err != nil {
			return trace, err
		}
	}
	return trace, nil
}

func decodeVariablesUsePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var event variablesUseBean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("variables-use SupportBean: %w", err)
		}
		return event, nil
	case "SupportBean_S0":
		var event variablesUseS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("variables-use SupportBean_S0: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("variables-use: unsupported event type %q", step.EventType)
	}
}
