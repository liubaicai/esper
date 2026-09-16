package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for ViewParameterizedByContext: context-parameterized view
// sizes — #length(context.miewl.intSize) under an overlapping initiated
// context, where each initiating event (P1=2, P2=4, P3=3) starts a partition
// whose length window caps the count(*) aggregate at its own size.
var viewParameterizedByContextJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewParameterizedByContext.java",
}

type viewParameterizedByContextInit struct {
	ID      string `esper:"id"`
	IntSize int32  `esper:"intSize"`
}

type viewParameterizedByContextBean struct {
	TheString     *string `esper:"theString"`
	IntPrimitive  int32   `esper:"intPrimitive"`
	LongPrimitive int64   `esper:"longPrimitive"`
}

var (
	viewParameterizedByContextJavaRuntimeIDs = []string{
		"java-runtime-71761cb17e7d22393efc", // LengthWindow
		"java-runtime-de2ad7eb74b76b16867a", // DocSample
		"java-runtime-6d60bed2a335972423d2", // MoreWindows
	}
	viewParameterizedByContextJavaExecutions = []string{
		"ViewParameterizedByContextLengthWindow",
		"ViewParameterizedByContextDocSample",
		"ViewParameterizedByContextMoreWindows",
	}
)

var viewParameterizedByContextCaseOrder = []string{
	"length-window", "doc-sample", "more-windows",
}

func runViewParameterizedByContextScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range viewParameterizedByContextCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		caseTrace, err := runViewParameterizedByContextCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("view-parameterized-by-context case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("view-parameterized-by-context scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runViewParameterizedByContextCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewParameterizedByContextInit](env, "SupportContextInitEventWLength"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[viewParameterizedByContextBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	// `initiated by SupportContextInitEventWLength as miewl terminated after
	// 1 year`: every initiating event starts an overlapping partition; the
	// calendar-year timer end never fires on the pinned epoch clock.
	isInit := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportContextInitEventWLength"))
	endTimer := esper.TimerIntervalCalendar(esper.From[viewParameterizedByContextInit](env, "SupportContextInitEventWLength"), 1, 0, 0)
	if _, err := esper.CreateOverlappingPatternTerminatedContext(env, "CtxInitToTerm", esper.Literal("global"), isInit, endTimer); err != nil {
		return compat.Trace{}, err
	}

	initID := func() esper.Expr { return esper.Property[*string](esper.ContextInitiatingEvent(), "id") }
	intSize := func() esper.Expr { return esper.Property[int](esper.ContextInitiatingEvent(), "intSize") }
	query, buildErr := env.Build(esper.From[viewParameterizedByContextBean](env, "SupportBean").
		Filter(esper.EqualOf(esper.Field[viewParameterizedByContextBean, *string]("theString"), initID())).
		Window(esper.LengthWindowExpr(intSize())).
		Aggregate(
			esper.Alias("id", initID()),
			esper.Alias("cnt", esper.CountAll()),
		).Query(esper.StatementName("s0"), esper.WithContext("CtxInitToTerm"), esper.WithOldStream()))
	if buildErr != nil {
		return compat.Trace{}, buildErr
	}

	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	if caseName == "more-windows" {
		return runViewParameterizedByContextMoreWindows(ctx, engine, caseScenario, caseName, env, trace)
	}

	seq := uint64(0)
	statementFound := false
	var statement *esper.Statement
	// Single deployment up front: the pinned modules deploy once and the
	// remaining steps are sends and snapshots.
	deployment, deployErr := engine.Deploy(ctx, query)
	if deployErr != nil {
		return trace, deployErr
	}
	statement = deployment.Statements()[0]
	statementFound = true
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		hasNew := len(batch.New) > 0
		hasOld := len(batch.Old) > 0
		if !hasNew && !hasOld {
			return nil
		}
		seq++
		record := compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: statement.Name(),
			Time:      compat.FormatTraceTime(batch.Time),
			Sequence:  seq,
		}
		record.New = compat.NormalizeResults(batch.New)
		record.Old = compat.NormalizeResults(batch.Old)
		trace.Records = append(trace.Records, record)
		return nil
	}); err != nil {
		return trace, err
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("view-parameterized-by-context decode %s: %w", step.EventType, err)
			}
			if err := engine.SendRecord(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "snapshot":
			if !statementFound {
				return trace, fmt.Errorf("snapshot statement %q not found", step.Statement)
			}
			result, snapErr := statement.Snapshot(ctx)
			if snapErr != nil {
				return trace, snapErr
			}
			rows := compat.NormalizeResults(result.Batch.New)
			if rows == nil {
				rows = []compat.ResultRecord{}
			}
			// mode "any": Java asserts these iterator rows any-order; both
			// sides emit canonical (sorted) row order.
			if step.Mode == "any" {
				sortRowsCanonical(rows)
			}
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: step.Statement,
			}
			record.New = rows
			trace.Records = append(trace.Records, record)
		default:
			return trace, fmt.Errorf("unsupported view-parameterized-by-context step op %q", step.Op)
		}
	}
	return trace, nil
}

// runViewParameterizedByContextMoreWindows replays ViewParameterizedByContextMoreWindows:
// twelve context-parameterized window kinds, each deployed in its own
// deploy→init(P1=2)→init(P2=20)→undeploy cycle with no listener attached.
// The pinned observable is one deployed record per kind.
func runViewParameterizedByContextMoreWindows(ctx context.Context, engine *esper.Engine, caseScenario compat.Scenario, caseName string, env *esper.Environment, trace compat.Trace) (compat.Trace, error) {
	intSize := func() esper.Expr { return esper.Property[int](esper.ContextInitiatingEvent(), "intSize") }
	ts := esper.Field[viewParameterizedByContextBean, int64]("longPrimitive")
	theString := esper.Field[viewParameterizedByContextBean, *string]("theString")
	ip := esper.Field[viewParameterizedByContextBean, int32]("intPrimitive")

	specs := []esper.WindowSpec{
		esper.LengthBatchExpr(intSize()),
		esper.TimeWindowExpr(intSize()),
		esper.ExternallyTimedExpr(ts, intSize()),
		esper.TimeBatchExpr(intSize()),
		esper.ExternallyTimedBatchExpr(ts, intSize()),
		esper.TimeLengthBatchExpr(intSize(), intSize()),
		esper.TimeAccumExpr(intSize()),
		esper.FirstLengthExpr(intSize()),
		esper.FirstTimeExpr(intSize()),
		esper.SortWindowExpr(intSize(), esper.Ascending(ip)),
		esper.RankWindowExpr(intSize(), []esper.Expr{theString}, esper.Ascending(theString)),
		esper.TimeOrderExpr(ts, intSize()),
	}

	deployedCount := 0
	undeployedCount := 0
	var current *esper.Deployment
	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "deployed":
			if deployedCount == len(specs) {
				return trace, fmt.Errorf("more-windows: unexpected deployed step beyond the twelve kinds")
			}
			plan, buildErr := env.Build(esper.From[viewParameterizedByContextBean](env, "SupportBean").
				Window(specs[deployedCount]).
				Query(esper.StatementName("s0"), esper.WithContext("CtxInitToTerm")))
			if buildErr != nil {
				return trace, buildErr
			}
			deployment, deployErr := engine.Deploy(ctx, plan)
			if deployErr != nil {
				return trace, deployErr
			}
			deployedCount++
			current = deployment
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
			})
		case "undeploy":
			if current == nil {
				return trace, fmt.Errorf("more-windows: undeploy without deployment")
			}
			if err := current.Undeploy(ctx); err != nil {
				return trace, err
			}
			current = nil
			undeployedCount++
		case "send":
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("view-parameterized-by-context decode %s: %w", step.EventType, err)
			}
			if err := engine.SendRecord(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("unsupported more-windows step op %q", step.Op)
		}
	}
	if deployedCount != len(specs) || undeployedCount != len(specs) {
		return trace, fmt.Errorf("more-windows: deployed=%d undeployed=%d, want 12/12", deployedCount, undeployedCount)
	}
	return trace, nil
}
