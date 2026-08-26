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
		"java-runtime-eb01093e6db83f057d77",
		"java-runtime-8457cbd989256d935b22",
		"java-runtime-4373a1c6d8c903357942",
		"java-runtime-80cb4763680bc91e38ce",
	}
	variablesUseJavaExecutions = []string{
		"EPLVariableUseVariableInFilter",
		"EPLVariableUseVariableInFilterBoolean",
		"EPLVariableUseSimpleSameModule",
		"EPLVariableUseSimplePreconfigured",
		"EPLVariableUseSimpleTwoModules",
		"EPLVariableUseInvokeMethod",
		"EPLVariableUseFilterConstantCustomTypePreconfigured",
	}
)

// runVariablesUseScenario replays variable-use scenarios: variables assigned
// by on-set statements and consumed inside equality / or filters.
func runVariablesUseScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"variable-in-filter", "variable-in-filter-boolean", "simple-same-module", "simple-preconfigured", "simple-two-modules", "invoke-method", "filter-constant-custom-type"}
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
	case "simple-preconfigured":
		if err := env.RegisterVariable("var_simple_preconfig_const", true, esper.ConstantVariable()); err != nil {
			return compat.Trace{}, err
		}
		query = esper.Select(
			esper.From[variablesUseBean](env, "SupportBean"),
			esper.Alias("c0", esper.VariableRef[bool]("var_simple_preconfig_const")),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{query}, caseScenario, caseName)
	case "simple-two-modules":
		// Module A's @public create-variable is modeled as the environment
		// registration (the established representation of `create variable`),
		// which module B's deployment then reads; Java pins @public + path.
		if err := env.RegisterVariable("var_simple_twomodule_const", true); err != nil {
			return compat.Trace{}, err
		}
		query = esper.Select(
			esper.From[variablesUseBean](env, "SupportBean"),
			esper.Alias("c0", esper.VariableRef[bool]("var_simple_twomodule_const")),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{query}, caseScenario, caseName)
	case "invoke-method":
		// Constant variable initialized from the factory plus the
		// preconfigured instance variable; dot-invocation is represented by
		// function expressions over the variable-hosted values.
		if err := env.RegisterVariable("myService", variablesUseServiceFactory().makeService(), esper.ConstantVariable()); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("myInitService", variablesUseServiceFactory().makeService()); err != nil {
			return compat.Trace{}, err
		}
		query = esper.Select(
			esper.From[variablesUseBean](env, "SupportBean"),
			esper.Alias("c0", variablesUseDoSomething("myService")),
			esper.Alias("c1", variablesUseDoSomething("myInitService")),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{query}, caseScenario, caseName)
	case "filter-constant-custom-type":
		if _, err := esper.RegisterStruct[variablesUseCustomEvent](env, "MyVariableCustomEvent"); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("my_variable_custom_typed", variablesUseCustomType{Value: "abc"}, esper.ConstantVariable()); err != nil {
			return compat.Trace{}, err
		}
		query = esper.Select(
			esper.From[variablesUseCustomEvent](env, "MyVariableCustomEvent").Filter(
				esper.Equal[variablesUseCustomType](
					esper.Field[variablesUseCustomEvent, variablesUseCustomType]("name"),
					esper.VariableRef[variablesUseCustomType]("my_variable_custom_typed"),
				),
			),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{query}, caseScenario, caseName)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported variables-use case %q", caseName)
	}
}

// variablesUseService mirrors MySimpleVariableService: the dot-call surface
// hosts a hello responder on a variable.
type variablesUseService struct{}

func (variablesUseService) doSomething() string { return "hello" }

type variablesUseServiceFactoryType struct{}

func (variablesUseServiceFactoryType) makeService() variablesUseService {
	return variablesUseService{}
}

func variablesUseServiceFactory() variablesUseServiceFactoryType {
	return variablesUseServiceFactoryType{}
}

// variablesUseDoSomething reads the variable-hosted service inside the
// evaluation context and invokes it, mirroring the Java dot-call. A missing
// or mistyped variable degrades to the empty string here (Java fails at
// deploy); registration drift surfaces as a trace mismatch.
func variablesUseDoSomething(variableName string) esper.Expression[string] {
	return esper.Func1Ctx[esper.Event, string]("variable-dot-"+variableName,
		func(event esper.Event, ctx esper.EvalContext) string {
			if ctx.Variables == nil {
				return ""
			}
			value, ok := ctx.Variables[variableName]
			if !ok {
				return ""
			}
			if service, ok := value.Any().(variablesUseService); ok {
				return service.doSomething()
			}
			return ""
		}, esper.EventValue[esper.Event]())
}

// variablesUseCustomType mirrors MyVariableCustomType's value equality.
type variablesUseCustomType struct {
	Value string `esper:"value"`
}

type variablesUseCustomEvent struct {
	Name variablesUseCustomType `esper:"name"`
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
					fields[field.Name] = variablesUseRender(event.Get(field.Name).Any())
				}
			} else if r, ok := row.Row(); ok {
				for _, field := range r.Schema().Fields() {
					fields[field.Name] = variablesUseRender(r.Get(field.Name).Any())
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
	case "MyVariableCustomEvent":
		// The payload carries the raw value; MyVariableCustomType.of wraps it.
		var shim struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(step.Payload, &shim); err != nil {
			return nil, fmt.Errorf("variables-use MyVariableCustomEvent: %w", err)
		}
		return variablesUseCustomEvent{Name: variablesUseCustomType{Value: shim.Name}}, nil
	default:
		return nil, fmt.Errorf("variables-use: unsupported event type %q", step.EventType)
	}
}

// variablesUseRender mirrors the oracle's value conventions: the custom
// variable-hosted type renders as the canonical sorted-public-field object.
func variablesUseRender(value any) any {
	if custom, ok := value.(variablesUseCustomType); ok {
		return map[string]any{"name": custom.Value}
	}
	return streamSelectorRender(value)
}
