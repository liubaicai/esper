package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
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

type eplOtherDistinctA struct {
	ID string `esper:"id"`
}

type eplOtherDistinctN struct {
	IntPrimitive int32 `esper:"intPrimitive"`
	IntBoxed     int32 `esper:"intBoxed"`
}

// eplOtherDistinctPatternBean carries longPrimitive for the pattern-join
// executions; it is registered as SupportBean only inside those case envs.
type eplOtherDistinctPatternBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int32  `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
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
		"java-runtime-d9d561caf48755690e5d",
		"java-runtime-17e049a70e6c5ca2a376",
		"java-runtime-526d4b64636e45cc051d",
		"java-runtime-c1d232f82f0a154d6241",
		"java-runtime-eb3cba6e0ccf8c3c9c83",
		"java-runtime-550dd87b54717508c0b9",
		"java-runtime-7b13ab2779d557eba691",
		"java-runtime-93ccd6033710a75f113a",
		"java-runtime-e21f5d942d6c4869bf1b",
		"java-runtime-6905746f98b2a3b39f8b",
		"java-runtime-5deb9547c58344716a93",
		"java-runtime-deca832debc5d94d4dcc",
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
		"EPLOtherOnDemandAndOnSelect",
		"EPLOtherOutputRateSnapshotColumn",
		"EPLOtherSubquery",
		"EPLOtherBeanEventWildcardThisProperty",
		"EPLOtherBeanEventWildcardSODA",
		"EPLOtherBeanEventWildcardPlusCols",
		"EPLOtherMapEventWildcard",
		"EPLOtherBatchWindowJoin",
		"EPLOtherBatchWindowInsertInto",
		"EPLOtherDistinctWildcardJoinPatternOne",
		"EPLOtherDistinctWildcardJoinPatternTwo",
		"EPLOtherDistinctVariantStream",
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
		"distinct-ondemand-onselect",
		"distinct-snapshot-column",
		"distinct-snapshot-column-join",
		"distinct-subquery",
		"distinct-wildcard-bean",
		"distinct-wildcard-soda",
		"distinct-wildcard-plus-cols",
		"distinct-wildcard-map",
		"distinct-batch-window-join",
		"distinct-batch-window-insert-into",
		"distinct-pattern-one",
		"distinct-pattern-two",
		"distinct-variant-stream",
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
	case "distinct-snapshot-column", "distinct-snapshot-column-join", "distinct-subquery":
		return runEplOtherDistinctSlice2Case(ctx, scenario, caseName)
	case "distinct-ondemand-onselect":
		return runEplOtherDistinctOnDemandCase(ctx, scenario, caseName)
	case "distinct-wildcard-bean", "distinct-wildcard-soda", "distinct-wildcard-plus-cols", "distinct-wildcard-map":
		return runEplOtherDistinctWildcardCase(ctx, scenario, caseName)
	case "distinct-batch-window-join", "distinct-batch-window-insert-into",
		"distinct-pattern-one", "distinct-pattern-two", "distinct-variant-stream":
		return runEplOtherDistinctFinaleCase(ctx, scenario, caseName)
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
	case "SupportBean_N":
		var event eplOtherDistinctN
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("epl-other-distinct SupportBean_N: %w", err)
		}
		return event, nil
	case "MyMapTypeKVDistinct":
		var payload struct {
			K1 string `json:"k1"`
			V1 int32  `json:"v1"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("epl-other-distinct MyMapTypeKVDistinct: %w", err)
		}
		return map[string]any{"k1": payload.K1, "v1": payload.V1}, nil
	case "SupportBean_A":
		var event eplOtherDistinctA
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("epl-other-distinct SupportBean_A: %w", err)
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

// runEplOtherDistinctSlice2Case covers the snapshot-column pair (plain and
// join variants) and the IN-subquery execution, all single-statement
// deployments over SupportBean with SupportBean_A support events.
func runEplOtherDistinctSlice2Case(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplOtherDistinctEvent](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplOtherDistinctA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}

	var query esper.Query
	switch caseName {
	case "distinct-snapshot-column":
		str := esper.Field[eplOtherDistinctEvent, string]("theString")
		num := esper.Field[eplOtherDistinctEvent, int32]("intPrimitive")
		ws := esper.From[eplOtherDistinctEvent](env, "SupportBean").Window(esper.KeepAll())
		query = esper.Select(ws,
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
		).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
			esper.WithOutput(esper.OutputSnapshotEveryEvents(3)),
			esper.OrderBy(esper.Ascending(str)),
		)
	case "distinct-snapshot-column-join":
		str := esper.Field[eplOtherDistinctEvent, string]("theString")
		aID := esper.Field[eplOtherDistinctA, string]("id")
		left := esper.From[eplOtherDistinctEvent](env, "SupportBean").Window(esper.KeepAll())
		right := esper.From[eplOtherDistinctA](env, "SupportBean_A").Window(esper.KeepAll())
		query = esper.Join(left, right,
			esper.OnSourcesEqual(0, str, 1, aID),
		).Select(
			esper.SelectFrom(0, "theString", esper.JoinField[string](0, "theString")),
			esper.SelectFrom(0, "intPrimitive", esper.JoinField[int32](0, "intPrimitive")),
		).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
			esper.WithOutput(esper.OutputSnapshotEveryEvents(3)),
			esper.OrderBy(esper.Ascending(esper.ResultField[string]("theString"))),
		)
	case "distinct-subquery":
		// The Java execution selects * but asserts only theString and
		// intPrimitive; the oracle records that assertion surface and the Go
		// projection matches it.
		str := esper.Field[eplOtherDistinctEvent, string]("theString")
		num := esper.Field[eplOtherDistinctEvent, int32]("intPrimitive")
		ws := esper.From[eplOtherDistinctEvent](env, "SupportBean").Filter(
			esper.SubqueryIn[string](
				str,
				esper.From[eplOtherDistinctA](env, "SupportBean_A").Window(esper.KeepAll()).AsRecord(),
				esper.Field[eplOtherDistinctA, string]("id"),
			),
		)
		query = esper.Select(ws,
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
		).Query(
			esper.StatementName("s0"),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported epl-other-distinct slice-2 case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeEplOtherDistinctPayload, resolveEplOtherDistinctSingle(statement))
}

// runEplOtherDistinctOnDemandCase replays the named-window on-demand surface:
// window + insert modules, an on-select trigger with distinct and order by,
// and a FAF distinct query executed after the window is populated.
func runEplOtherDistinctOnDemandCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplOtherDistinctEvent](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplOtherDistinctA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}
	schema, err := esper.StructSchema[eplOtherDistinctEvent]("SupportBean")
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateNamedWindow(env, "MyWindow", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
		return compat.Trace{}, err
	}
	strField := esper.Field[eplOtherDistinctEvent, string]("theString")
	numField := esper.Field[eplOtherDistinctEvent, int32]("intPrimitive")

	insertPlan, err := env.Build(esper.OnEvent(esper.From[eplOtherDistinctEvent](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindow",
		esper.SetColumn("theString", strField),
		esper.SetColumn("intPrimitive", numField),
	).Query(esper.StatementName("insert")))
	if err != nil {
		return compat.Trace{}, err
	}
	fafPlan, err := env.Build(esper.FromNamedWindow(env, "MyWindow").Select(
		esper.Alias("theString", esper.Field[any, string]("theString")),
		esper.Alias("intPrimitive", esper.Field[any, int32]("intPrimitive")),
	).Query(
		esper.WithDistinct(),
		esper.OrderBy(
			esper.Ascending(esper.ResultField[string]("theString")),
			esper.Ascending(esper.ResultField[int32]("intPrimitive")),
		),
	))
	if err != nil {
		return compat.Trace{}, err
	}
	onSelectPlan, err := env.Build(esper.OnEvent(esper.From[eplOtherDistinctA](env, "SupportBean_A")).SelectFromNamedWindow(
		"MyWindow",
		nil,
		esper.Alias("theString", esper.NamedWindowField[string]("theString")),
		esper.Alias("intPrimitive", esper.NamedWindowField[int32]("intPrimitive")),
	).Query(
		esper.StatementName("s0"),
		esper.WithDistinct(),
		esper.OrderBy(
			esper.Ascending(esper.ResultField[string]("theString")),
			esper.Ascending(esper.ResultField[int32]("intPrimitive")),
		),
	))
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
	for _, plan := range []esper.Plan{insertPlan, onSelectPlan} {
		if _, err := engine.Deploy(ctx, plan); err != nil {
			return compat.Trace{}, err
		}
	}
	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	deployed, err := engine.Statements(ctx)
	if err != nil {
		return trace, err
	}
	var onSelect *esper.Statement
	for _, candidate := range deployed {
		if candidate.Name() == "s0" {
			onSelect = candidate
			break
		}
	}
	if onSelect == nil {
		return compat.Trace{}, fmt.Errorf("epl-other-distinct: on-select statement s0 not found")
	}

	seq := uint64(0)
	if _, err := onSelect.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		seq++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: onSelect.Name(),
			Sequence:  seq,
			Time:      currentTimeString(engine),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
		return nil
	}); err != nil {
		return trace, err
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
		case "snapshot":
			result, err := engine.ExecuteFireAndForget(ctx, fafPlan)
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
		default:
			return trace, fmt.Errorf("unsupported epl-other-distinct on-demand step op %q", step.Op)
		}
	}
	cleanup = false
	return trace, nil
}

// runEplOtherDistinctWildcardCase replays the select-distinct-* family over
// keepall windows: bean wildcard, SupportBean_A wildcard, computed-column
// wildcard (intBoxed%5), and a Map event type. All four executions assert
// iterator contents only, so the replay records iterator snapshots after each
// send instead of listener deliveries.
func runEplOtherDistinctWildcardCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	var query esper.Query
	switch caseName {
	case "distinct-wildcard-bean":
		if _, err := esper.RegisterStruct[eplOtherDistinctEvent](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		ws := esper.From[eplOtherDistinctEvent](env, "SupportBean").Window(esper.KeepAll())
		query = esper.Select(ws).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
		)
	case "distinct-wildcard-soda":
		if _, err := esper.RegisterStruct[eplOtherDistinctA](env, "SupportBean_A"); err != nil {
			return compat.Trace{}, err
		}
		ws := esper.From[eplOtherDistinctA](env, "SupportBean_A").Window(esper.KeepAll())
		query = esper.Select(ws).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
		)
	case "distinct-wildcard-plus-cols":
		if _, err := esper.RegisterStruct[eplOtherDistinctN](env, "SupportBean_N"); err != nil {
			return compat.Trace{}, err
		}
		intPrimitive := esper.Field[eplOtherDistinctN, int32]("intPrimitive")
		intBoxed := esper.Field[eplOtherDistinctN, int32]("intBoxed")
		ws := esper.From[eplOtherDistinctN](env, "SupportBean_N").Window(esper.KeepAll())
		query = esper.Select(ws,
			esper.Alias("intPrimitive", intPrimitive),
			esper.Alias("val1", esper.Modulo[int32](intBoxed, esper.Literal[int32](5))),
			esper.Alias("val2", intBoxed),
		).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
		)
	case "distinct-wildcard-map":
		if _, err := esper.RegisterMap(env, "MyMapTypeKVDistinct", []esper.FieldSpec{
			esper.FieldDef("k1", reflect.TypeOf("")),
			esper.FieldDef("v1", reflect.TypeOf(int32(0))),
		}); err != nil {
			return compat.Trace{}, err
		}
		ws := esper.From[map[string]any](env, "MyMapTypeKVDistinct").Window(esper.KeepAll())
		query = esper.Select(ws).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported epl-other-distinct wildcard case %q", caseName)
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

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
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
		case "snapshot":
			result, err := statement.Snapshot(ctx)
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
		default:
			return trace, fmt.Errorf("unsupported epl-other-distinct wildcard step op %q", step.Op)
		}
	}
	return trace, nil
}

// runEplOtherDistinctFinaleCase replays the capability-finale slice: the
// batch-window join, the batch-window insert-into route, both pattern-join
// executions, and the variant-schema stream. Rows are scoped to each
// execution's assertion surface.
func runEplOtherDistinctFinaleCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	isPattern := caseName == "distinct-pattern-one" || caseName == "distinct-pattern-two"
	finaleDecode := decodeEplOtherDistinctPayload
	if isPattern {
		finaleDecode = func(step compat.Step) (any, error) {
			if step.EventType == "SupportBean" {
				var event eplOtherDistinctPatternBean
				if err := json.Unmarshal(step.Payload, &event); err != nil {
					return nil, fmt.Errorf("epl-other-distinct SupportBean: %w", err)
				}
				return event, nil
			}
			return decodeEplOtherDistinctPayload(step)
		}
	}
	if !isPattern {
		if _, err := esper.RegisterStruct[eplOtherDistinctEvent](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
	}
	if _, err := esper.RegisterStruct[eplOtherDistinctA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}

	switch caseName {
	case "distinct-batch-window-join":
		str := esper.Field[eplOtherDistinctEvent, string]("theString")
		aID := esper.Field[eplOtherDistinctA, string]("id")
		left := esper.From[eplOtherDistinctEvent](env, "SupportBean").Window(esper.LengthBatch(3))
		right := esper.From[eplOtherDistinctA](env, "SupportBean_A").Window(esper.KeepAll())
		query := esper.Join(left, right,
			esper.OnSourcesEqual(0, str, 1, aID),
		).Select(
			esper.SelectFrom(0, "theString", esper.JoinField[string](0, "theString")),
			esper.SelectFrom(0, "intPrimitive", esper.JoinField[int32](0, "intPrimitive")),
		).Query(
			esper.StatementName("s0"),
			esper.WithDistinct(),
		)
		return replayFinaleStandard(ctx, env, query, caseScenario, caseName, finaleDecode)
	case "distinct-batch-window-insert-into":
		// Java auto-declares MyStream from the insert-into clause; the Go
		// route target must be declared up front with the projected shape.
		if _, err := esper.RegisterStruct[eplOtherDistinctEvent](env, "MyStream"); err != nil {
			return compat.Trace{}, err
		}
		str := esper.Field[eplOtherDistinctEvent, string]("theString")
		num := esper.Field[eplOtherDistinctEvent, int32]("intPrimitive")
		ws := esper.From[eplOtherDistinctEvent](env, "SupportBean").Window(esper.LengthBatch(3))
		insertQuery := esper.Select(ws,
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
		).InsertInto("MyStream",
			esper.StatementName("insert"),
			esper.WithDistinct(),
		)
		downstream := esper.FromAny(env, "MyStream")
		downstreamQuery := downstream.Select().Query(
			esper.StatementName("s0"),
		)
		return replayFinaleTwoStatement(ctx, env, insertQuery, downstreamQuery, caseScenario, caseName, finaleDecode)
	case "distinct-pattern-one", "distinct-pattern-two":
		if _, err := esper.RegisterStruct[eplOtherDistinctPatternBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		query := esperPatternJoinQuery(env, caseName == "distinct-pattern-two")
		if caseName == "distinct-pattern-one" {
			return replayFinalePatternWeak(ctx, env, query, caseScenario, caseName, finaleDecode)
		}
		return replayFinalePatternOrdered(ctx, env, query, caseScenario, caseName, finaleDecode)
	case "distinct-variant-stream":
		if _, err := esper.RegisterStruct[eplOtherDistinctManyArray](env, "SupportEventWithManyArray"); err != nil {
			return compat.Trace{}, err
		}
		manyArraySchema, ok := env.Schema("SupportEventWithManyArray")
		if !ok {
			return compat.Trace{}, fmt.Errorf("epl-other-distinct: SupportEventWithManyArray schema missing")
		}
		if _, err := esper.RegisterVariant(env, "MyVariant", manyArraySchema); err != nil {
			return compat.Trace{}, err
		}
		insertPlan, err := env.Build(esper.From[eplOtherDistinctManyArray](env, "SupportEventWithManyArray").InsertInto(
			"MyVariant", esper.StatementName("insert")))
		if err != nil {
			return compat.Trace{}, err
		}
		intOne := esper.Field[any, []int]("intOne")
		intTwo := esper.Field[any, []int]("intTwo")
		s0Plan, err := env.Build(esper.FromAny(env, "MyVariant").Window(esper.KeepAll()).Select().Query(
			esper.StatementName("s0"), esper.WithDistinct()))
		if err != nil {
			return compat.Trace{}, err
		}
		s1Plan, err := env.Build(esper.FromAny(env, "MyVariant").Window(esper.KeepAll()).Select(
			esper.Alias("intOne", intOne),
		).Query(
			esper.StatementName("s1"), esper.WithDistinct()))
		if err != nil {
			return compat.Trace{}, err
		}
		s2Plan, err := env.Build(esper.FromAny(env, "MyVariant").Window(esper.KeepAll()).Select(
			esper.Alias("intOne", intOne),
			esper.Alias("intTwo", intTwo),
		).Query(
			esper.StatementName("s2"), esper.WithDistinct()))
		if err != nil {
			return compat.Trace{}, err
		}
		engine := esper.NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		statements := make(map[string]*esper.Statement)
		for _, plan := range []esper.Plan{insertPlan, s0Plan, s1Plan, s2Plan} {
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, err
			}
			for _, statement := range deployment.Statements() {
				statements[statement.Name()] = statement
			}
		}
		trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
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
			case "snapshot":
				target := statements[step.Statement]
				if target == nil {
					return trace, fmt.Errorf("unknown epl-other-distinct variant snapshot statement %q", step.Statement)
				}
				result, err := target.Snapshot(ctx)
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
			default:
				return trace, fmt.Errorf("unsupported epl-other-distinct variant step op %q", step.Op)
			}
		}
		return trace, nil
	default:
		return compat.Trace{}, fmt.Errorf("unsupported epl-other-distinct finale case %q", caseName)
	}
}

// esperPatternJoinQuery builds the every-distinct pattern-join query shared
// by the two pattern executions; orderByWooA adds the deterministic ordering
// of the second execution.
func esperPatternJoinQuery(env *esper.Environment, orderByWooA bool) esper.Query {
	base := esper.From[eplOtherDistinctPatternBean](env, "SupportBean")
	intIs := func(want int32) esper.Expression[bool] {
		return esper.Equal[int32](esper.Field[eplOtherDistinctPatternBean, int32]("intPrimitive"), esper.Literal(want))
	}
	key := esper.Field[eplOtherDistinctPatternBean, string]("theString")
	// Java's timer:within postfix binds to the preceding pattern atom, so
	// the one-hour guard scopes the wooA branch before the sequence composes.
	pattern := esper.PatternFrom(base, "fooA", intIs(1)).EveryDistinct(key).
		Then(esper.PatternFrom(base, "wooA", intIs(2)).EveryDistinct(key).Within(time.Hour))
	options := []esper.QueryOption{esper.StatementName("s0"), esper.WithDistinct()}
	if orderByWooA {
		options = append(options, esper.OrderBy(esper.Ascending(esper.JoinPatternField[string](1, "wooA", "theString"))))
	}
	return esper.JoinMany(
		esper.JoinSource(base.Filter(intIs(0))).Unidirectional(),
		esper.JoinPatternSource(pattern).Window(esper.TimeWindow(time.Hour)),
	).On(esper.OnSourcesEqual(
		0, esper.Field[eplOtherDistinctPatternBean, int64]("longPrimitive"),
		1, esper.JoinPatternField[int64](1, "fooA", "longPrimitive"),
	)).Select(
		esper.SelectSourceEvent(0, "fooB"),
		esper.SelectSourceEvent(1, "fooWooPair"),
	).Query(options...)
}

// patternJoinSurfaceRow projects one wildcard join row onto the Java
// subscriber's assertion surface: (fooB.theString, fooA.theString, wooA.theString).
func patternJoinSurfaceRow(row esper.Result) (string, string, string, error) {
	fooBEvent, ok := row.Get("fooB").Any().(esper.Event)
	if !ok {
		return "", "", "", fmt.Errorf("fooB is not an event: %#v", row.Get("fooB").Any())
	}
	pairEvent, ok := row.Get("fooWooPair").Any().(esper.Event)
	if !ok {
		return "", "", "", fmt.Errorf("fooWooPair is not an event: %#v", row.Get("fooWooPair").Any())
	}
	fooAEvent, ok := pairEvent.Get("fooA").Any().(esper.Event)
	if !ok {
		return "", "", "", fmt.Errorf("fooWooPair.fooA is not an event: %#v", pairEvent.Get("fooA").Any())
	}
	wooAEvent, ok := pairEvent.Get("wooA").Any().(esper.Event)
	if !ok {
		return "", "", "", fmt.Errorf("fooWooPair.wooA is not an event: %#v", pairEvent.Get("wooA").Any())
	}
	fooB, _ := fooBEvent.Get("theString").Any().(string)
	fooA, _ := fooAEvent.Get("theString").Any().(string)
	wooA, _ := wooAEvent.Get("theString").Any().(string)
	return fooB, fooA, wooA, nil
}

func replayFinaleStandard(ctx context.Context, env *esper.Environment, query esper.Query, caseScenario compat.Scenario, caseName string, decode compat.DecodePayload) (compat.Trace, error) {
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decode, resolveEplOtherDistinctSingle(statement))
}

func replayFinaleTwoStatement(ctx context.Context, env *esper.Environment, insertQuery, downstreamQuery esper.Query, caseScenario compat.Scenario, caseName string, decode compat.DecodePayload) (compat.Trace, error) {
	insertPlan, err := env.Build(insertQuery)
	if err != nil {
		return compat.Trace{}, err
	}
	downstreamPlan, err := env.Build(downstreamQuery)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	var downstream *esper.Statement
	for _, plan := range []esper.Plan{insertPlan, downstreamPlan} {
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return compat.Trace{}, err
		}
		for _, statement := range deployment.Statements() {
			if statement.Name() == "s0" {
				downstream = statement
			}
		}
	}
	if downstream == nil {
		return compat.Trace{}, fmt.Errorf("epl-other-distinct: downstream s0 not found")
	}
	return compat.ReplayWithStatements(ctx, engine, downstream, caseScenario, decode, resolveEplOtherDistinctSingle(downstream))
}

// replayFinalePatternWeak records a single listener-invoked marker: the Java
// suite asserts only the invoked flag for the first pattern execution.
func replayFinalePatternWeak(ctx context.Context, env *esper.Environment, query esper.Query, caseScenario compat.Scenario, caseName string, decode compat.DecodePayload) (compat.Trace, error) {
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	recorded := false
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if recorded || len(batch.New) == 0 {
			return nil
		}
		recorded = true
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener-invoked",
			Statement: statement.Name(),
			Sequence:  1,
			Time:      currentTimeString(engine),
		})
		return nil
	}); err != nil {
		return trace, err
	}
	for _, step := range caseScenario.Steps {
		if step.Op == "case" {
			continue
		}
		if step.Op != "send" {
			return trace, fmt.Errorf("unsupported epl-other-distinct pattern step op %q", step.Op)
		}
		payload, err := decode(step)
		if err != nil {
			return trace, err
		}
		if err := engine.Send(ctx, step.EventType, payload); err != nil {
			return trace, err
		}
	}
	return trace, nil
}

// replayFinalePatternOrdered projects the ordered MRD payload of the second
// pattern execution: exactly one insert batch of two rows keyed by
// (fooB.theString, fooA.theString, wooA.theString).
func replayFinalePatternOrdered(ctx context.Context, env *esper.Environment, query esper.Query, caseScenario compat.Scenario, caseName string, decode compat.DecodePayload) (compat.Trace, error) {
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		rows := make([]compat.ResultRecord, 0, len(batch.New))
		for _, row := range batch.New {
			fooB, fooA, wooA, err := patternJoinSurfaceRow(row)
			if err != nil {
				return err
			}
			rows = append(rows, compat.ResultRecord{Kind: "row", Fields: map[string]any{
				"theString": fooB,
				"fooA":      fooA,
				"wooA":      wooA,
			}})
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  1,
			Time:      currentTimeString(engine),
			New:       rows,
		})
		return nil
	}); err != nil {
		return trace, err
	}
	for _, step := range caseScenario.Steps {
		if step.Op == "case" {
			continue
		}
		if step.Op != "send" {
			return trace, fmt.Errorf("unsupported epl-other-distinct pattern step op %q", step.Op)
		}
		payload, err := decode(step)
		if err != nil {
			return trace, err
		}
		if err := engine.Send(ctx, step.EventType, payload); err != nil {
			return trace, err
		}
	}
	return trace, nil
}
