package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EPLDataflowOpSelect iterate and join flows:
//   - EPLDataflowIterateFinalMarker: an iterate Select accumulates grouped
//     rows and emits them only on the final marker (pre-marker reads empty).
//   - EPLDataflowFromClauseJoinOrder: the inner-join wait-for-all-inputs and
//     the from-clause order independence of the fully-cross lastevent join.
//   - EPLDataflowOuterJoinMultirow: the full-outer keep-all unmatched row with
//     a null missing side.
//
// All three run captive: StartCaptive, submit through the captive emitters,
// observe the capture sink, and cancel.
type dataflowSelectFlowBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type dataflowSelectFlowS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type dataflowSelectFlowS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

type dataflowSelectFlowS2 struct {
	ID  int    `esper:"id"`
	P20 string `esper:"p20"`
}

const dataflowSelectFlowsJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowSelectFlowsJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpSelect.java",
}

var dataflowSelectFlowsJavaRuntimeIDs = []string{
	"java-runtime-59ce9d5f4b9ca475c272",
	"java-runtime-323dd1ec14f5ed5c0586",
	"java-runtime-21dcd981de8124bf387c",
}

var dataflowSelectFlowsJavaExecutions = []string{
	"EPLDataflowIterateFinalMarker",
	"EPLDataflowFromClauseJoinOrder",
	"EPLDataflowOuterJoinMultirow",
}

var dataflowSelectFlowsCases = []string{
	"select-iterate-final-marker",
	"select-join-order",
	"select-outer-join-multirow",
}

// dataflowSelectFlowCapture records delivered values in arrival order.
type dataflowSelectFlowCapture struct {
	values []any
}

func (c *dataflowSelectFlowCapture) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	c.values = append(c.values, input.Value)
	return nil, nil
}

// dataflowSelectFlowEmit renders the captured values into one trace record.
// Empty reads are recorded with an empty row list — the wait-for-inputs and
// post-cancel emptiness are the join-order observables.
func dataflowSelectFlowEmit(records []compat.TraceRecord, caseName, operation string, sequence uint64, now time.Time, values []any) ([]compat.TraceRecord, uint64) {
	rows := make([]compat.ResultRecord, 0, len(values))
	for _, value := range values {
		switch payload := value.(type) {
		case esper.Event:
			rows = append(rows, compat.NormalizeEvents([]esper.Event{payload})...)
		case esper.Row:
			rows = append(rows, compat.NormalizeRows([]esper.Row{payload})...)
		}
	}
	return append(records, compat.TraceRecord{
		Case:      caseName,
		Operation: operation,
		Statement: "flow:DefaultSupportCaptureOp",
		Sequence:  sequence + 1,
		Time:      compat.FormatTraceTime(now),
		New:       rows,
	}), sequence + 1
}

func runDataflowSelectFlowsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-select-flows scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowSelectFlowsCases {
		caseTrace, err := runDataflowSelectFlowsCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-select-flows case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowSelectFlowsCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[dataflowSelectFlowBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[dataflowSelectFlowS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[dataflowSelectFlowS1](env, "SupportBean_S1"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[dataflowSelectFlowS2](env, "SupportBean_S2"); err != nil {
		return nil, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(dataflowSelectFlowsJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	// Reads mirror DefaultSupportCaptureOp.getCurrentAndReset: each read
	// returns the accumulated rows and clears the sink.
	emit := func(capture *dataflowSelectFlowCapture, operation string) {
		records, sequence = dataflowSelectFlowEmit(records, caseName, operation, sequence, now, capture.values)
		capture.values = nil
	}

	switch caseIndex {
	case 0: // select-iterate-final-marker
		capture := &dataflowSelectFlowCapture{}
		key := esper.Field[dataflowSelectFlowBean, string]("theString")
		definition, err := esper.DefineDataflow(env, "MySelect").
			Emitter("source").
			SelectIterate("select", []esper.Expr{key}, []esper.SortKey{esper.Ascending(key)},
				esper.Alias("theString", key),
				esper.Alias("sumInt", esper.Sum[int](esper.Field[dataflowSelectFlowBean, int]("intPrimitive")))).
			Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			Connect("source", "select").
			Connect("select", "capture").
			Build()
		if err != nil {
			return nil, err
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
		for _, event := range []dataflowSelectFlowBean{
			{TheString: "E3", IntPrimitive: 4},
			{TheString: "E2", IntPrimitive: 3},
			{TheString: "E1", IntPrimitive: 1},
			{TheString: "E2", IntPrimitive: 2},
			{TheString: "E1", IntPrimitive: 5},
		} {
			if err := emitter.Submit(ctx, event); err != nil {
				return nil, err
			}
		}
		emit(capture, "capture")
		if err := emitter.SubmitSignal(ctx, esper.FinalMarker{}); err != nil {
			return nil, err
		}
		emit(capture, "iterate")
		if err := instance.Cancel(ctx); err != nil {
			return nil, err
		}
	case 1: // select-join-order — three from-clause permutations
		// Each sub-run rewires which input port each emitter connects to;
		// the projections read the tuple side their source landed on.
		permutations := []struct {
			wiring [3]int // emitter index -> select input port
			s0Port int
			s1Port int
			s2Port int
		}{
			{wiring: [3]int{2, 1, 0}, s0Port: 2, s1Port: 1, s2Port: 0}, // from S2, S1, S0
			{wiring: [3]int{0, 1, 2}, s0Port: 0, s1Port: 1, s2Port: 2}, // from S0, S1, S2
			{wiring: [3]int{2, 0, 1}, s0Port: 2, s1Port: 0, s2Port: 1}, // from S1, S2, S0
		}
		for run, perm := range permutations {
			capture := &dataflowSelectFlowCapture{}
			s0id := esper.JoinField[int](perm.s0Port, "id")
			s1id := esper.JoinField[int](perm.s1Port, "id")
			s2id := esper.JoinField[int](perm.s2Port, "id")
			definition, buildErr := esper.DefineDataflow(env, fmt.Sprintf("MySelect-%d", run)).
				Emitter("s0").Emitter("s1").Emitter("s2").
				SelectJoin("select", esper.DataflowJoinOptions{Inputs: 3},
					esper.Alias("s0id", s0id),
					esper.Alias("s1id", s1id),
					esper.Alias("s2id", s2id)).
				Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
					return capture, nil
				}).
				ConnectInput("s0", "select", perm.wiring[0]).
				ConnectInput("s1", "select", perm.wiring[1]).
				ConnectInput("s2", "select", perm.wiring[2]).
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
			emitterS0, err := instance.CaptiveEmitter("s0")
			if err != nil {
				return nil, err
			}
			emitterS1, err := instance.CaptiveEmitter("s1")
			if err != nil {
				return nil, err
			}
			emitterS2, err := instance.CaptiveEmitter("s2")
			if err != nil {
				return nil, err
			}
			if err := emitterS0.Submit(ctx, dataflowSelectFlowS0{ID: 1, P00: "S0_1"}); err != nil {
				return nil, err
			}
			if err := emitterS1.Submit(ctx, dataflowSelectFlowS1{ID: 10, P10: "S1_1"}); err != nil {
				return nil, err
			}
			emit(capture, "capture")
			if err := emitterS2.Submit(ctx, dataflowSelectFlowS2{ID: 100, P20: "S2_1"}); err != nil {
				return nil, err
			}
			emit(capture, "capture")
			if err := instance.Cancel(ctx); err != nil {
				return nil, err
			}
			// Post-cancel submissions produce nothing (ErrorState is the
			// expected engine response and is deliberately not surfaced).
			_ = emitterS2.Submit(ctx, dataflowSelectFlowS2{ID: 101, P20: "S2_2"})
			emit(capture, "capture")
		}
	case 2: // select-outer-join-multirow
		capture := &dataflowSelectFlowCapture{}
		definition, err := esper.DefineDataflow(env, "MySelect").
			Emitter("s0").Emitter("s1").
			SelectJoin("select", esper.DataflowJoinOptions{
				Inputs:    2,
				Kind:      esper.DataflowJoinFullOuter,
				Retention: esper.DataflowJoinKeepAll,
			},
				esper.Alias("p00", esper.JoinField[string](0, "p00")),
				esper.Alias("p10", esper.JoinField[string](1, "p10"))).
			Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			ConnectInput("s0", "select", 0).
			ConnectInput("s1", "select", 1).
			Connect("select", "capture").
			Build()
		if err != nil {
			return nil, err
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		if err := instance.Start(ctx); err != nil {
			return nil, err
		}
		emitterS0, err := instance.CaptiveEmitter("s0")
		if err != nil {
			return nil, err
		}
		if err := emitterS0.Submit(ctx, dataflowSelectFlowS0{ID: 1, P00: "S0_1"}); err != nil {
			return nil, err
		}
		emit(capture, "capture")
		if err := instance.Cancel(ctx); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported dataflow-select-flows case index %d", caseIndex)
	}
	return records, nil
}
