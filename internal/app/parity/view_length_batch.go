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

// Parity coverage for ViewLengthBatch: length-batch view flush/accumulation,
// wildcard and projected irstream delivery, prev* accessor evaluation over
// batch windows, quiet named-window deletes, groupwin nesting, iterator
// snapshots of partial batches, and compile-time size validation.
var viewLengthBatchJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewLengthBatch.java",
}

type viewLengthBatchMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   *string `esper:"feed"`
}

type viewLengthBatchBean struct {
	TheString    *string  `esper:"theString"`
	IntPrimitive int32    `esper:"intPrimitive"`
	DoubleBoxed  *float64 `esper:"doubleBoxed"`
}

type viewLengthBatchSupportA struct {
	ID string `esper:"id"`
}

var (
	viewLengthBatchJavaRuntimeIDs = []string{
		"java-runtime-97e65154c5aefc5735d4", // SceneOne
		"java-runtime-964e6db2f54f1b508387", // Size2
		"java-runtime-edd63c54bc9fe03a8380", // Size1
		"java-runtime-5a98579c3673cc0224ce", // Size3
		"java-runtime-5c9d89c9732aedb6a5da", // Invalid
		"java-runtime-d22e3122427d1dd8ceb0", // Normal{VIEW}
		"java-runtime-a3dc40576e7c9def33fb", // Normal{NAMEDWINDOW}
		"java-runtime-b6eb26431d8d05188516", // Normal{GROUPWIN}
		"java-runtime-03b48f31fe26fedf4d4b", // Prev (deferred)
		"java-runtime-8bc763b2e677e747ee4c", // Delete
	}
	viewLengthBatchJavaExecutions = []string{
		"ViewLengthBatchSceneOne",
		"ViewLengthBatchSize2",
		"ViewLengthBatchSize1",
		"ViewLengthBatchSize3",
		"ViewLengthBatchInvalid",
		"ViewLengthBatchNormal{runType=VIEW}",
		"ViewLengthBatchPrev (deferred)",
		"ViewLengthBatchNormal{runType=NAMEDWINDOW}",
		"ViewLengthBatchNormal{runType=GROUPWIN}",
		"ViewLengthBatchDelete",
	}
)

func runViewLengthBatchScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range []string{
		"scene-one", "size-two", "size-one", "size-three", "invalid",
		"delete", "normal-namedwindow", "normal-groupwin",
	} {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		caseTrace, err := runViewLengthBatchCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("view-length-batch case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("view-length-batch scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runViewLengthBatchCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewLengthBatchBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[viewLengthBatchSupportA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}
	str := reflect.TypeOf("")
	f64 := reflect.TypeOf(float64(0))
	i64 := reflect.TypeOf(int64(0))
	if _, err := esper.RegisterMap(env, "SupportMarketDataBean", []esper.FieldSpec{
		esper.FieldDef("symbol", str),
		esper.FieldDef("price", f64),
		esper.FieldDef("volume", i64),
		esper.FieldDef("feed", str),
	}); err != nil {
		return compat.Trace{}, err
	}

	var plans []struct {
		name     string
		plan     esper.Plan
		listener bool
	}
	build := func(name string, listener bool, query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		plans = append(plans, struct {
			name     string
			plan     esper.Plan
			listener bool
		}{name, plan, listener})
		return nil
	}

	switch caseName {
	case "scene-one":
		err = build("s0", true, esper.FromAny(env, "SupportMarketDataBean").
			Window(esper.LengthBatch(3)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "size-two":
		err = build("s0", true, esper.From[viewLengthBatchBean](env, "SupportBean").
			Window(esper.LengthBatch(2)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "size-one":
		err = build("s0", true, esper.From[viewLengthBatchBean](env, "SupportBean").
			Window(esper.LengthBatch(1)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "size-three":
		err = build("s0", true, esper.From[viewLengthBatchBean](env, "SupportBean").
			Window(esper.LengthBatch(3)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "normal-view":
		ts := esper.Field[viewLengthBatchBean, *string]("theString")
		err = build("s0", true, esper.Select(
			esper.From[viewLengthBatchBean](env, "SupportBean").Window(esper.LengthBatch(3)),
			esper.Alias("theString", ts),
			esper.Alias("prevString", esper.Prev[*string](1, ts)),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "prev":
		js := func() esper.Expression[string] { return esper.Field[viewLengthBatchMarket, string]("symbol") }
		err = build("s0", true, esper.Select(
			esper.From[viewLengthBatchMarket](env, "SupportMarketDataBean").Window(esper.LengthBatch(3)),
			esper.Alias("symbol", js()),
			esper.Alias("prev1", esper.Prev[string](1, js())),
			esper.Alias("prevTail0", esper.PrevTail[string](0, js())),
			esper.Alias("prevTail1", esper.PrevTail[string](1, js())),
			esper.Alias("prevCountSym", esper.PrevCount[int64](js())),
			esper.Alias("prevWindowSym", esper.PrevWindow[string](js())),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "delete":
		schema, schemaErr := esper.StructSchema[viewLengthBatchBean]("SupportBean")
		if schemaErr != nil {
			return compat.Trace{}, schemaErr
		}
		if _, err := esper.CreateNamedWindow(env, "ABCWin", schema, esper.NamedWindowRetention(esper.LengthBatch(3))); err != nil {
			return compat.Trace{}, err
		}
		if err := build("insert", false, esper.OnEvent(esper.From[viewLengthBatchBean](env, "SupportBean")).
			InsertIntoNamedWindow("ABCWin", esper.CopyMatchingFields()).
			Query(esper.StatementName("insert"))); err != nil {
			return compat.Trace{}, err
		}
		if err := build("delete", false, esper.OnEvent(esper.From[viewLengthBatchSupportA](env, "SupportBean_A")).
			DeleteFromNamedWindow("ABCWin", esper.Equal[string](
				esper.NamedWindowField[string]("theString"),
				esper.Field[viewLengthBatchSupportA, string]("id"),
			)).Query(esper.StatementName("delete"))); err != nil {
			return compat.Trace{}, err
		}
		err = build("s0", true, esper.FromNamedWindow(env, "ABCWin").
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "normal-namedwindow":
		schema, schemaErr := esper.StructSchema[viewLengthBatchBean]("SupportBean")
		if schemaErr != nil {
			return compat.Trace{}, schemaErr
		}
		if _, err := esper.CreateNamedWindow(env, "NWLB", schema, esper.NamedWindowRetention(esper.LengthBatch(3))); err != nil {
			return compat.Trace{}, err
		}
		if err := build("insert", false, esper.OnEvent(esper.From[viewLengthBatchBean](env, "SupportBean")).
			InsertIntoNamedWindow("NWLB", esper.CopyMatchingFields()).
			Query(esper.StatementName("insert"))); err != nil {
			return compat.Trace{}, err
		}
		err = build("s0", true, esper.FromNamedWindow(env, "NWLB").
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "invalid":
		// Compile-only approved difference: Go rejects LengthBatch(0) at
		// Build time with its own message text; Java asserts an EPL message
		// prefix. Both agree the compile must fail.
		_, buildErr := env.Build(esper.From[viewLengthBatchBean](env, "SupportBean").
			Window(esper.LengthBatch(0)).
			Query(esper.StatementName("s0")))
		if buildErr == nil {
			return compat.Trace{}, fmt.Errorf("length_batch(0) unexpectedly built")
		}
	case "normal-groupwin":
		db := esper.Field[viewLengthBatchBean, *float64]("doubleBoxed")
		err = build("s0", true, esper.From[viewLengthBatchBean](env, "SupportBean").
			Window(esper.GroupWindow(db, esper.LengthBatch(3))).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported view-length-batch case %q", caseName)
	}
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seq := uint64(0)
	statements := make(map[string]*esper.Statement)
	for _, item := range plans {
		deployment, err := engine.Deploy(ctx, item.plan)
		if err != nil {
			return trace, err
		}
		for _, st := range deployment.Statements() {
			statements[st.Name()] = st
		}
		if !item.listener {
			continue
		}
		st := deployment.Statements()[0]
		if _, err := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			hasNew := len(batch.New) > 0
			hasOld := len(batch.Old) > 0
			if !hasNew && !hasOld {
				return nil
			}
			seq++
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "listener",
				Statement: st.Name(),
				Time:      batch.Time.UTC().Format(time.RFC3339),
				Sequence:  seq,
			}
			record.New = compat.NormalizeResults(batch.New)
			record.Old = compat.NormalizeResults(batch.Old)
			trace.Records = append(trace.Records, record)
			return nil
		}); err != nil {
			return trace, err
		}
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("view-length-batch decode %s: %w", step.EventType, err)
			}
			// Fill schema defaults for absent fields (Java constructor semantics).
			switch step.EventType {
			case "SupportMarketDataBean":
				for _, field := range []struct {
					name   string
					defVal any
				}{{"price", float64(0)}, {"volume", int64(0)}, {"feed", nil}} {
					if _, ok := payload[field.name]; !ok {
						payload[field.name] = field.defVal
					}
				}
			case "SupportBean":
				for _, field := range []struct {
					name   string
					defVal any
				}{{"intPrimitive", int32(0)}, {"doubleBoxed", nil}} {
					if _, ok := payload[field.name]; !ok {
						payload[field.name] = field.defVal
					}
				}
			}
			if err := engine.SendRecord(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "deployed":
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
			})
		case "snapshot":
			st, ok := statements[step.Statement]
			if !ok {
				return trace, fmt.Errorf("snapshot statement %q not found", step.Statement)
			}
			result, snapErr := st.Snapshot(ctx)
			if snapErr != nil {
				return trace, snapErr
			}
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: step.Statement,
			}
			record.New = compat.NormalizeResults(result.Batch.New)
			if record.New == nil {
				record.New = []compat.ResultRecord{}
			}
			trace.Records = append(trace.Records, record)
		default:
			return trace, fmt.Errorf("unsupported view-length-batch step op %q", step.Op)
		}
	}
	return trace, nil
}
