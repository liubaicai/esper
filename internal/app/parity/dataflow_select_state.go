package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EPLDataflowOpSelect's time-based Select executions:
//   - EPLDataflowOutputRateLimit: `output snapshot every 1 minute` — captive
//     submissions alone emit nothing; each virtual-time snapshot tick emits
//     the current cumulative projection; cancel suppresses later ticks.
//   - EPLDataflowTimeWindowTriggered: `#time(1 minute)` — insert-time output
//     with a cumulative window and boundary-inclusive expiry (an event
//     inserted at t expires exactly at t+window).
//
// Record `time` carries the live virtual clock at each read so the phase
// boundaries are observable; the tick instants themselves are not
// observable through the capture sink and are not recorded.
type dataflowSelectStateBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const dataflowSelectStateJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowSelectStateJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpSelect.java",
}

var dataflowSelectStateJavaRuntimeIDs = []string{
	"java-runtime-64f78048eb114d0e9cf9",
	"java-runtime-553841d9498fc8d6d1c7",
}

var dataflowSelectStateJavaExecutions = []string{
	"EPLDataflowOutputRateLimit",
	"EPLDataflowTimeWindowTriggered",
}

var dataflowSelectStateCases = []string{
	"select-output-rate-limit",
	"select-time-window-triggered",
}

// dataflowSelectStateCapture records delivered values in arrival order.
type dataflowSelectStateCapture struct {
	values []any
}

func (c *dataflowSelectStateCapture) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	c.values = append(c.values, input.Value)
	return nil, nil
}

func runDataflowSelectStateScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-select-state scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowSelectStateCases {
		caseTrace, err := runDataflowSelectStateCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-select-state case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowSelectStateCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[dataflowSelectStateBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(dataflowSelectStateJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	record := func(values []any) {
		sequence++
		rows := make([]compat.ResultRecord, 0, len(values))
		for _, value := range values {
			switch payload := value.(type) {
			case esper.Event:
				rows = append(rows, compat.NormalizeEvents([]esper.Event{payload})...)
			case esper.Row:
				rows = append(rows, compat.NormalizeRows([]esper.Row{payload})...)
			}
		}
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: "capture",
			Statement: "flow:DefaultSupportCaptureOp",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			New:       rows,
		})
	}
	advance := func(at time.Time) error {
		now = at
		return engine.AdvanceTime(ctx, at)
	}
	sumInt := func() esper.Selection {
		return esper.Alias("sumInt", esper.Sum[int](esper.Field[dataflowSelectStateBean, int]("intPrimitive")))
	}

	switch caseIndex {
	case 0: // select-output-rate-limit
		capture := &dataflowSelectStateCapture{}
		definition, buildErr := esper.DefineDataflow(env, "MySelect").
			Emitter("source").
			SelectSnapshotEvery("select", time.Minute, sumInt()).
			Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			Connect("source", "select").
			Connect("select", "capture").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		if err := instance.Start(ctx); err != nil {
			return nil, err
		}
		emitter, err := instance.CaptiveEmitter("source")
		if err != nil {
			return nil, err
		}
		if err := advance(time.Unix(5, 0).UTC()); err != nil {
			return nil, err
		}
		for _, primitive := range []int{5, 3, 6} {
			if err := emitter.Submit(ctx, dataflowSelectStateBean{TheString: "E", IntPrimitive: primitive}); err != nil {
				return nil, err
			}
		}
		record(capture.values)
		capture.values = nil
		if err := advance(time.Unix(65, 0).UTC()); err != nil {
			return nil, err
		}
		record(capture.values)
		capture.values = nil
		for _, primitive := range []int{3, 6} {
			if err := emitter.Submit(ctx, dataflowSelectStateBean{TheString: "E", IntPrimitive: primitive}); err != nil {
				return nil, err
			}
		}
		record(capture.values)
		capture.values = nil
		if err := advance(time.Unix(125, 0).UTC()); err != nil {
			return nil, err
		}
		record(capture.values)
		capture.values = nil
		if err := instance.Cancel(ctx); err != nil {
			return nil, err
		}
		// Post-cancel submissions return ErrorState (suppressed) and
		// post-cancel advances dispatch nothing.
		_ = emitter.Submit(ctx, dataflowSelectStateBean{TheString: "E", IntPrimitive: 6})
		if err := advance(time.Unix(245, 0).UTC()); err != nil {
			return nil, err
		}
		record(capture.values)
	case 1: // select-time-window-triggered
		capture := &dataflowSelectStateCapture{}
		definition, buildErr := esper.DefineDataflow(env, "MySelect").
			Emitter("source").
			SelectTimeWindow("select", time.Minute, sumInt()).
			Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			Connect("source", "select").
			Connect("select", "capture").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		if err := instance.Start(ctx); err != nil {
			return nil, err
		}
		emitter, err := instance.CaptiveEmitter("source")
		if err != nil {
			return nil, err
		}
		if err := advance(time.Unix(5, 0).UTC()); err != nil {
			return nil, err
		}
		if err := emitter.Submit(ctx, dataflowSelectStateBean{TheString: "E1", IntPrimitive: 2}); err != nil {
			return nil, err
		}
		record(capture.values)
		capture.values = nil
		if err := advance(time.Unix(10, 0).UTC()); err != nil {
			return nil, err
		}
		if err := emitter.Submit(ctx, dataflowSelectStateBean{TheString: "E2", IntPrimitive: 5}); err != nil {
			return nil, err
		}
		record(capture.values)
		capture.values = nil
		if err := advance(time.Unix(65, 0).UTC()); err != nil {
			return nil, err
		}
		record(capture.values)
		if err := instance.Cancel(ctx); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported dataflow-select-state case index %d", caseIndex)
	}
	return records, nil
}
