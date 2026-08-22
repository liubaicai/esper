package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type eplOtherDistinctEvent struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

type eplOtherDistinctManyArray struct {
	ID     string `esper:"id"`
	IntOne []int  `esper:"intOne"`
	IntTwo []int  `esper:"intTwo"`
}

type eplOtherDistinctS0 struct {
	ID int `esper:"id"`
}

type eplOtherDistinctS1 struct {
	ID int `esper:"id"`
}

var eplOtherDistinctJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherDistinct.java",
}

var (
	eplOtherDistinctJavaRuntimeIDs = []string{
		"java-runtime-4b62c52a89cf86a3843a",
		"java-runtime-2e0f0a1356ea1b8e3299",
		"java-runtime-0318c1bce4d5e806aa11",
		"java-runtime-fe86653eacb81851d04f",
		"java-runtime-f9a1dafb621a6605ef47",
		"java-runtime-c752f36ef07cc6280306",
		"java-runtime-4fab4842341b8021b1a2",
		"java-runtime-9066a6d1932dae5ff6aa",
	}
	eplOtherDistinctJavaExecutions = []string{
		"EPLOtherOutputSimpleColumn",
		"EPLOtherOutputLimitEveryColumn",
		"EPLOtherBatchWindow",
		"EPLOtherDistinctOutputLimitMultikeyWArraySingleArray",
		"EPLOtherDistinctOutputLimitMultikeyWArrayTwoArray",
		"EPLOtherDistinctFireAndForgetMultikeyWArray",
		"EPLOtherDistinctIterateMultikeyWArray",
		"EPLOtherDistinctOnSelectMultikeyWArray",
	}
)

// runEplOtherDistinctScenario replays select-distinct view-flow scenarios over
// SupportBean, covering keep-all dedup, output-every batching with dedup, and
// length-batch flush dedup. Rows preserve first-seen insertion order.
func runEplOtherDistinctScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"distinct-simple-column",
		"distinct-output-every-column",
		"distinct-batch-window",
		"distinct-mwarray-output-limit-single",
		"distinct-mwarray-output-limit-two",
		"distinct-mwarray-faf",
		"distinct-mwarray-iterate",
		"distinct-mwarray-on-select",
	}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("epl-other-distinct scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		trace, err := runEplOtherDistinctCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl-other-distinct case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runEplOtherDistinctCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplOtherDistinctEvent](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	str := esper.Field[eplOtherDistinctEvent, string]("theString")
	num := esper.Field[eplOtherDistinctEvent, int32]("intPrimitive")

	var query esper.Query
	switch caseName {
	case "distinct-simple-column":
		ws := esper.From[eplOtherDistinctEvent](env, "SupportBean").Window(esper.KeepAll())
		query = esper.Select(ws,
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
		).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
		)
	case "distinct-output-every-column":
		ws := esper.From[eplOtherDistinctEvent](env, "SupportBean")
		query = esper.Select(ws,
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
		).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
			esper.WithOutput(esper.OutputAllEveryEvents(3)),
		)
	case "distinct-batch-window":
		ws := esper.From[eplOtherDistinctEvent](env, "SupportBean").Window(esper.LengthBatch(3))
		query = esper.Select(ws,
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
		).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
		)
	default:
		if len(caseName) > 16 && caseName[:16] == "distinct-mwarray" {
			return runEplOtherDistinctMultikeyCase(ctx, scenario, caseName)
		}
		return compat.Trace{}, fmt.Errorf("unsupported epl-other-distinct case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeEplOtherDistinctPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown epl-other-distinct statement %q", name)
		}
		return statement, nil
	})
}

func decodeEplOtherDistinctPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var event eplOtherDistinctEvent
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("epl-other-distinct SupportBean: %w", err)
		}
		return event, nil
	case "SupportEventWithManyArray":
		var event eplOtherDistinctManyArray
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("epl-other-distinct SupportEventWithManyArray: %w", err)
		}
		return event, nil
	case "SupportBean_S0":
		var event eplOtherDistinctS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("epl-other-distinct SupportBean_S0: %w", err)
		}
		return event, nil
	case "SupportBean_S1":
		var event eplOtherDistinctS1
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("epl-other-distinct SupportBean_S1: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("epl-other-distinct: unsupported event type %q", step.EventType)
	}
}

// runEplOtherDistinctMultikeyCase replays the MultikeyWArray family over
// SupportEventWithManyArray whose int[] keys compare by deep content. It covers
// time-driven output batches (advance-time flush), fire-and-forget queries
// against a named window, iterator snapshots, and on-select triggers.
func runEplOtherDistinctMultikeyCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplOtherDistinctManyArray](env, "SupportEventWithManyArray"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplOtherDistinctS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplOtherDistinctS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}

	intOne := esper.Field[eplOtherDistinctManyArray, []int]("intOne")
	intTwo := esper.Field[eplOtherDistinctManyArray, []int]("intTwo")

	switch caseName {
	case "distinct-mwarray-output-limit-single", "distinct-mwarray-output-limit-two":
		ws := esper.From[eplOtherDistinctManyArray](env, "SupportEventWithManyArray")
		selections := []esper.Selection{esper.Alias("intOne", intOne)}
		if caseName == "distinct-mwarray-output-limit-two" {
			selections = append(selections, esper.Alias("intTwo", intTwo))
		}
		query := esper.Select(ws,
			selections...,
		).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
			esper.WithOutput(esper.OutputEveryTime(time.Second)),
		)
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err := deployParityStatement(ctx, env, plan)
		if err != nil {
			return compat.Trace{}, err
		}
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeEplOtherDistinctPayload, resolveEplOtherDistinctSingle(statement))
	case "distinct-mwarray-faf", "distinct-mwarray-iterate", "distinct-mwarray-on-select":
		return runEplOtherDistinctMultikeyWindowCase(ctx, env, caseScenario, caseName)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported epl-other-distinct multikey case %q", caseName)
	}
}

func resolveEplOtherDistinctSingle(statement *esper.Statement) compat.StatementResolver {
	return func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown epl-other-distinct statement %q", name)
		}
		return statement, nil
	}
}

// runEplOtherDistinctMultikeyWindowCase handles the named-window surfaces:
// FAF queries (no listener output), keepall iterator snapshots (no listener
// output), and on-select triggers (listener output on trigger events only).
func runEplOtherDistinctMultikeyWindowCase(ctx context.Context, env *esper.Environment, caseScenario compat.Scenario, caseName string) (compat.Trace, error) {
	schema, err := esper.StructSchema[eplOtherDistinctManyArray]("SupportEventWithManyArray")
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateNamedWindow(env, "MyWindow", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
		return compat.Trace{}, err
	}
	idField := esper.Field[eplOtherDistinctManyArray, string]("id")
	intOneField := esper.Field[eplOtherDistinctManyArray, []int]("intOne")
	intTwoField := esper.Field[eplOtherDistinctManyArray, []int]("intTwo")

	insertPlan, err := env.Build(esper.OnEvent(esper.From[eplOtherDistinctManyArray](env, "SupportEventWithManyArray")).InsertIntoNamedWindow(
		"MyWindow",
		esper.SetColumn("id", idField),
		esper.SetColumn("intOne", intOneField),
		esper.SetColumn("intTwo", intTwoField),
	).Query(esper.StatementName("insert")))
	if err != nil {
		return compat.Trace{}, err
	}

	fafPlans := make(map[string]esper.Plan)
	buildFaf := func(key string, selections ...esper.Selection) error {
		plan, err := env.Build(esper.FromNamedWindow(env, "MyWindow").Select(
			selections...,
		).Query(
			esper.StatementName(key),
			esper.WithDistinct(),
		))
		if err != nil {
			return err
		}
		fafPlans[key] = plan
		return nil
	}

	engine := esper.NewEngine(env)
	cleanup := true
	defer func() {
		if cleanup {
			_ = engine.Close(context.Background())
		}
	}()

	statements := make([]*esper.Statement, 0, 3)
	insertDeployment, err := engine.Deploy(ctx, insertPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements = append(statements, insertDeployment.Statements()...)

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	// pending collects listener records emitted during send/advance-time steps
	// so they interleave with snapshot records in replay order.
	pending := make([]compat.TraceRecord, 0, 4)
	drain := func() {
		trace.Records = append(trace.Records, pending...)
		pending = pending[:0]
	}

	switch caseName {
	case "distinct-mwarray-faf":
		if err := buildFaf("faf-single",
			esper.Alias("intOne", esper.Field[any, []int]("intOne"))); err != nil {
			return trace, err
		}
		if err := buildFaf("faf-two",
			esper.Alias("intOne", esper.Field[any, []int]("intOne")),
			esper.Alias("intTwo", esper.Field[any, []int]("intTwo"))); err != nil {
			return trace, err
		}
	case "distinct-mwarray-iterate":
		for _, spec := range []struct {
			name       string
			selections []esper.Selection
		}{
			{"s0", []esper.Selection{esper.Alias("intOne", intOneField)}},
			{"s1", []esper.Selection{esper.Alias("intOne", intOneField), esper.Alias("intTwo", intTwoField)}},
		} {
			plan, err := env.Build(esper.Select(
				esper.From[eplOtherDistinctManyArray](env, "SupportEventWithManyArray").Window(esper.KeepAll()),
				spec.selections...,
			).Query(
				esper.StatementName(spec.name),
				esper.WithDistinct(),
			))
			if err != nil {
				return trace, err
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return trace, err
			}
			statements = append(statements, deployment.Statements()...)
		}
	case "distinct-mwarray-on-select":
		for _, spec := range []struct {
			name       string
			selections []esper.Selection
		}{
			{"s0", []esper.Selection{esper.Alias("intOne", esper.NamedWindowField[[]int]("intOne"))}},
			{"s1", []esper.Selection{esper.Alias("intOne", esper.NamedWindowField[[]int]("intOne")), esper.Alias("intTwo", esper.NamedWindowField[[]int]("intTwo"))}},
		} {
			var query esper.Query
			if spec.name == "s0" {
				query = esper.OnEvent(esper.From[eplOtherDistinctS0](env, "SupportBean_S0")).SelectFromNamedWindow(
					"MyWindow", nil, spec.selections...).Query(
					esper.StatementName(spec.name),
					esper.WithDistinct(),
				)
			} else {
				query = esper.OnEvent(esper.From[eplOtherDistinctS1](env, "SupportBean_S1")).SelectFromNamedWindow(
					"MyWindow", nil, spec.selections...).Query(
					esper.StatementName(spec.name),
					esper.WithDistinct(),
				)
			}
			plan, err := env.Build(query)
			if err != nil {
				return trace, err
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return trace, err
			}
			statement := deployment.Statements()[0]
			statements = append(statements, statement)
			seq := uint64(0)
			target := statement
			if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				record := compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: target.Name(),
					Sequence:  seq + 1,
					Time:      currentTimeString(engine),
					New:       compat.NormalizeResults(batch.New),
					Old:       compat.NormalizeResults(batch.Old),
				}
				seq++
				pending = append(pending, record)
				return nil
			}); err != nil {
				return trace, err
			}
		}
	default:
		return trace, fmt.Errorf("unsupported epl-other-distinct window case %q", caseName)
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeEplOtherDistinctPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
			drain()
		case "advance-time":
			at, _ := time.Parse(time.RFC3339Nano, step.At)
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return trace, err
			}
			drain()
		case "snapshot":
			drain()
			if plan, ok := fafPlans[step.Statement]; ok {
				result, err := engine.ExecuteFireAndForget(ctx, plan)
				if err != nil {
					return trace, err
				}
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case:      caseName,
					Operation: "snapshot",
					Statement: step.Statement,
					Time:      currentTimeString(engine),
					New:       compat.NormalizeResults(result.Batch.New),
				})
				continue
			}
			var target *esper.Statement
			for _, candidate := range statements {
				if candidate.Name() == step.Statement {
					target = candidate
					break
				}
			}
			if target == nil {
				return trace, fmt.Errorf("unknown epl-other-distinct snapshot statement %q", step.Statement)
			}
			result, err := target.Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: target.Name(),
				Time:      currentTimeString(engine),
				New:       compat.NormalizeResults(result.Batch.New),
			})
		default:
			return trace, fmt.Errorf("unsupported epl-other-distinct step op %q", step.Op)
		}
	}
	cleanup = false
	return trace, nil
}
