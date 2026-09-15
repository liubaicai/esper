package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for ViewLengthWinWPropertyDetail: mapped/indexed/nested/
// array property projections over a length(3) window, gated by a
// three-property where clause; the mutated resend with indexed[1] at
// Integer.MIN_VALUE is filtered out, the restored resend is admitted.
var viewLengthWinPropertyDetailJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewLengthWin.java",
}

type viewLengthWinPropertyDetailProps struct {
	Mapped        map[string]any `esper:"mapped"`
	Indexed       []any          `esper:"indexed"`
	Nested        map[string]any `esper:"nested"`
	MapProperty   map[string]any `esper:"mapProperty"`
	ArrayProperty []any          `esper:"arrayProperty"`
}

var (
	viewLengthWinPropertyDetailJavaRuntimeIDs = []string{
		"java-runtime-9b050d42ae8cdfd3fa0d", // ViewLengthWinWPropertyDetail
	}
	viewLengthWinPropertyDetailJavaExecutions = []string{
		"ViewLengthWinWPropertyDetail",
	}
)

func runViewLengthWinPropertyDetailScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range []string{"w-property-detail"} {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		caseTrace, err := runViewLengthWinPropertyDetailCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("view-length-win-property-detail case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("view-length-win-property-detail scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runViewLengthWinPropertyDetailCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewLengthWinPropertyDetailProps](env, "SupportBeanComplexProps"); err != nil {
		return compat.Trace{}, err
	}

	root := esper.EventValue[esper.Event]()
	mappedKeyOne := esper.Property[string](root, "mapped('keyOne')")
	indexedOne := esper.Property[int32](root, "indexed[1]")
	nestedValue := esper.Property[string](root, "nested.nestedNested.nestedNestedValue")
	query, buildErr := env.Build(esper.Select(
		esper.From[viewLengthWinPropertyDetailProps](env, "SupportBeanComplexProps").
			Filter(esper.And(
				esper.And(
					esper.Equal[string](mappedKeyOne, esper.Literal("valueOne")),
					esper.Equal[int32](indexedOne, esper.Literal(int32(2))),
				),
				esper.Equal[string](nestedValue, esper.Literal("nestedNestedValue")),
			)).
			Window(esper.LengthWindow(3)),
		esper.Alias("a", mappedKeyOne),
		esper.Alias("b", indexedOne),
		esper.Alias("c", nestedValue),
		esper.Alias("mapProperty", esper.Property[any](root, "mapProperty")),
		esper.Alias("arrayProperty[0]", esper.Property[any](root, "arrayProperty[0]")),
	).Query(esper.StatementName("s0"), esper.WithOldStream()))
	if buildErr != nil {
		return compat.Trace{}, buildErr
	}

	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seq := uint64(0)
	statementFound := false
	var statement *esper.Statement
	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("view-length-win-property-detail decode %s: %w", step.EventType, err)
			}
			if err := engine.SendRecord(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "deployed":
			deployed, deployErr := engine.Deploy(ctx, query)
			if deployErr != nil {
				return trace, deployErr
			}
			statement = deployed.Statements()[0]
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
		case "snapshot":
			if !statementFound {
				return trace, fmt.Errorf("snapshot statement %q not found", step.Statement)
			}
			result, snapErr := statement.Snapshot(ctx)
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
			return trace, fmt.Errorf("unsupported view-length-win-property-detail step op %q", step.Op)
		}
	}
	return trace, nil
}
