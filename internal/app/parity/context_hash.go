package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextHashBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type contextHashS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

const (
	contextHashCaseNoPreallocate = "no-preallocate"
	contextHashCaseManyCRC32     = "many-arg-crc32"
	contextHashCaseManyHashCode  = "many-arg-hash-code"
	contextHashCaseSelection     = "partition-selection"
)

const (
	contextHashJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	contextHashJavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextHashSegmented.java"
)

var (
	contextHashJavaRuntimeIDs = []string{
		"java-runtime-64ed27d8c2b0325c1cbd",
		"java-runtime-5d429404e8e55f30567a",
		"java-runtime-c5406f66fdd34533bcb2",
	}
	contextHashJavaExecutions = []string{
		"ContextHashNoPreallocate",
		"ContextHashSegmentedManyArg",
		"ContextHashPartitionSelection",
	}
)

func splitMetadata(value string, fallback []string) []string {
	if strings.TrimSpace(value) == "" {
		return append([]string(nil), fallback...)
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			result = append(result, strings.TrimSpace(part))
		}
	}
	return result
}

func runContextHashScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		contextHashCaseNoPreallocate,
		contextHashCaseManyCRC32,
		contextHashCaseManyHashCode,
		contextHashCaseSelection,
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runContextHashCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context hash case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("context hash scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func scenarioHasCase(scenario compat.Scenario, name string) bool {
	for _, step := range scenario.Steps {
		if step.Op == "case" && step.Case == name {
			return true
		}
	}
	return false
}

func runContextHashCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextHashBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if caseName == contextHashCaseManyCRC32 || caseName == contextHashCaseManyHashCode {
		if _, err := esper.RegisterStruct[contextHashS0](env, "SupportBean_S0"); err != nil {
			return compat.Trace{}, err
		}
	}

	var statement *esper.Statement
	var engine *esper.Engine
	switch caseName {
	case contextHashCaseNoPreallocate:
		if _, err := esper.CreateHashContextWithAlgorithm(env, "CtxHash", esper.HashAlgorithmCRC32, 16,
			esper.Field[contextHashBean, string]("theString")); err != nil {
			return compat.Trace{}, err
		}
		key := esper.Field[contextHashBean, string]("theString")
		plan, err := env.Build(esper.From[contextHashBean](env, "SupportBean").GroupBy(key).Select(
			esper.Alias("c0", esper.ContextID()),
			esper.Alias("c1", key),
			esper.Alias("c2", esper.Sum[int](esper.Field[contextHashBean, int]("intPrimitive"))),
		).Query(esper.StatementName("s0"), esper.WithContext("CtxHash")))
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err = deployParityStatement(ctx, env, plan)
		if err != nil {
			return compat.Trace{}, err
		}
	case contextHashCaseManyCRC32, contextHashCaseManyHashCode:
		algorithm := esper.HashAlgorithmCRC32
		if caseName == contextHashCaseManyHashCode {
			algorithm = esper.HashAlgorithmJavaHashCode
		}
		if _, err := esper.CreateHashContextWithAlgorithm(env, "Ctx1", algorithm, 1000000,
			esper.Field[contextHashBean, string]("theString"),
			esper.Field[contextHashBean, int]("intPrimitive")); err != nil {
			return compat.Trace{}, err
		}
		inner := esper.From[contextHashS0](env, "SupportBean_S0").Window(esper.LengthWindow(2)).AsRecord()
		longValue := esper.Field[contextHashBean, int64]("longPrimitive")
		plan, err := env.Build(esper.From[contextHashBean](env, "SupportBean").Window(esper.LengthWindow(3)).Aggregate(
			esper.Alias("c1", esper.Field[contextHashBean, int]("intPrimitive")),
			esper.Alias("c2", esper.Sum[int64](longValue)),
			esper.Alias("c3", esper.Prev[int64](1, longValue)),
			esper.Alias("c4", esper.Prior[int64](0, longValue)),
			esper.Alias("c5", esper.SubqueryValue[string](inner, esper.Field[contextHashS0, string]("p00"))),
		).Query(esper.StatementName("s0"), esper.WithContext("Ctx1")))
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err = deployParityStatement(ctx, env, plan)
		if err != nil {
			return compat.Trace{}, err
		}
	case contextHashCaseSelection:
		if _, err := esper.CreatePreallocatedHashContextWithAlgorithm(env, "MyCtx", esper.HashAlgorithmCRC32, 16,
			esper.Field[contextHashBean, string]("theString")); err != nil {
			return compat.Trace{}, err
		}
		key := esper.Field[contextHashBean, string]("theString")
		plan, err := env.Build(esper.From[contextHashBean](env, "SupportBean").Window(esper.KeepAll()).GroupBy(key).Select(
			esper.Alias("c0", esper.ContextID()),
			esper.Alias("c1", key),
			esper.Alias("c2", esper.Sum[int](esper.Field[contextHashBean, int]("intPrimitive"))),
		).Query(esper.StatementName("s0"), esper.WithContext("MyCtx")))
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err = deployParityStatement(ctx, env, plan)
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context hash case %q", caseName)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextHashPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context hash statement %q", name)
		}
		return statement, nil
	})
}

func scenarioForCase(scenario compat.Scenario, wanted string) (compat.Scenario, error) {
	result := compat.Scenario{Version: scenario.Version, ID: scenario.ID}
	active := false
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			active = step.Case == wanted
			if active {
				result.Steps = append(result.Steps, step)
			}
			continue
		}
		if active {
			result.Steps = append(result.Steps, step)
		}
	}
	if !active && len(result.Steps) == 0 {
		return compat.Scenario{}, fmt.Errorf("context hash scenario %q has no case %q", scenario.ID, wanted)
	}
	if err := result.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return result, nil
}

func deployParityStatement(ctx context.Context, env *esper.Environment, plan esper.Plan) (*esper.Engine, *esper.Statement, error) {
	engine := esper.NewEngine(env)
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		_ = engine.Close(context.Background())
		return nil, nil, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		_ = engine.Close(context.Background())
		return nil, nil, fmt.Errorf("expected one parity statement, got %d", len(statements))
	}
	return engine, statements[0], nil
}

func decodeContextHashPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextHashBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextHashS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context hash event type %q", step.EventType)
	}
}
