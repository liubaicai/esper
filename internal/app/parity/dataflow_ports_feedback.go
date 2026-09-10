package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the dataflow ports/feedback family — all three
// executions of EPLDataflowInputOutputVariations:
//   - EPLDataflowFanInOut (java-runtime-6309a6b7e0f0ba981a97): four beacon
//     sources into a 2-in/2-out custom operator whose routing deliberately
//     crosses the declared port types (S0 strings → the SchemaTwo-typed
//     OutTwo, S1 ints → the SchemaOne-typed OutOne — Java's numeric
//     submitPort does no type check; Go mirrors with untyped CustomPorts and
//     maps numeric indexes to declaration order OutOne=0/OutTwo=1). Four
//     captures each read their full two-row current batch.
//   - EPLDataflowFactorial (java-runtime-98c5bdc6afa84705b9db): a feedback
//     self-edge (TempResult output back into the TempResult input) recursing
//     5·4·3·2 until the current counter reaches 1, then submitting 120 on
//     FinalResult; the instance completes on its own.
//   - EPLDataflowLargeNumOpsDataFlow (java-runtime-bddc8c72bd9d2a2c8869): a
//     17-stage identity chain. Java builds the stages via the select:
//     subquery parameter, which has no typed-API analogue — Go models the
//     stages as identity customs (documented difference; the deep-linear
//     shape is already engine-pinned).
//
// Record protocol: capture records with case-local sequences and fixed epoch
// time; rows are positional {p0:<value>} projections of the raw Object[]
// passthrough, and each capture read's rows are SORTED by the canonical
// fields-JSON comparator at emission on BOTH sides — Java's
// assertEqualsAnyOrder is thereby frozen deterministically (configured
// beacons arrive in racy order across the four source threads/goroutines).
// Capture reads are current-batch semantics: BeaconSource lacks
// @DataFlowOpProvideSignal, so its final markers die at the source channel
// and never flush the captures (contrast the captive-emitter chain where the
// signal was submitted directly).
const dataflowPortsFeedbackJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowPortsFeedbackJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowInputOutputVariations.java",
}

var dataflowPortsFeedbackJavaRuntimeIDs = []string{
	"java-runtime-6309a6b7e0f0ba981a97",
	"java-runtime-98c5bdc6afa84705b9db",
	"java-runtime-bddc8c72bd9d2a2c8869",
}

var dataflowPortsFeedbackJavaExecutions = []string{
	"EPLDataflowFanInOut",
	"EPLDataflowFactorial",
	"EPLDataflowLargeNumOpsDataFlow",
}

var dataflowPortsFeedbackCases = []string{
	"fan-in-out",
	"factorial",
	"large-num-ops",
}

// dataflowPortsFeedbackSchemaOne mirrors the SchemaOne object-array schema.
type dataflowPortsFeedbackSchemaOne struct {
	P1 string `esper:"p1"`
}

// dataflowPortsFeedbackSchemaTwo mirrors the SchemaTwo object-array schema.
type dataflowPortsFeedbackSchemaTwo struct {
	P2 int `esper:"p2"`
}

// dataflowPortsFeedbackInputSchema mirrors the InputSchema object-array.
type dataflowPortsFeedbackInputSchema struct {
	Number int `esper:"number"`
}

// dataflowPortsFeedbackCapture mirrors DefaultSupportCaptureOp's current
// batch (no signal flush: BeaconSource markers die at the source channel).
type dataflowPortsFeedbackCapture struct {
	current []any
}

func (c *dataflowPortsFeedbackCapture) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	c.current = append(c.current, input.Value)
	return nil, nil
}

// dataflowPortsFeedbackRuntime adapts a function to the operator runtime.
type dataflowPortsFeedbackRuntime struct {
	process func(context.Context, esper.DataflowInput) ([]esper.DataflowEmission, error)
}

func (r dataflowPortsFeedbackRuntime) Process(ctx context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	return r.process(ctx, input)
}

// sortRowsCanonical orders rows by their canonical fields JSON — the shared
// freeze for Java's assertEqualsAnyOrder.
func sortRowsCanonical(rows []compat.ResultRecord) {
	sort.SliceStable(rows, func(i, j int) bool {
		left, _ := json.Marshal(rows[i].Fields)
		right, _ := json.Marshal(rows[j].Fields)
		return string(left) < string(right)
	})
}

func runDataflowPortsFeedbackScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-ports-feedback scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowPortsFeedbackCases {
		caseTrace, err := runDataflowPortsFeedbackCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-ports-feedback case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowPortsFeedbackCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	emit := func(statement string, values []any) {
		sequence++
		rows := make([]compat.ResultRecord, 0, len(values))
		for _, value := range values {
			rows = append(rows, compat.ResultRecord{Kind: "row", Fields: map[string]any{"p0": value}})
		}
		sortRowsCanonical(rows)
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: "capture",
			Statement: statement,
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			New:       rows,
		})
	}

	switch caseIndex {
	case 0: // fan-in-out — 2-in/2-out custom routing, four captures
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowPortsFeedbackSchemaOne](env, "SchemaOne"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[dataflowPortsFeedbackSchemaTwo](env, "SchemaTwo"); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowPortsFeedbackJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		captures := map[string]*dataflowPortsFeedbackCapture{}
		for _, name := range []string{"SupportOpCountFutureOneA", "SupportOpCountFutureOneB", "SupportOpCountFutureTwoA", "SupportOpCountFutureTwoB"} {
			captures[name] = &dataflowPortsFeedbackCapture{}
		}
		captureFactory := func(capture *dataflowPortsFeedbackCapture) esper.DataflowOperatorFactory {
			return func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}
		}
		// Java routes numerically: submitPort(1) → OutTwo (declaration
		// index 1), submitPort(0) → OutOne (index 0) — deliberately crossing
		// the declared port types with untyped passthrough values.
		myCustomOp := func(_ esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
			process := func(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
				event, ok := input.Value.(esper.Event)
				if !ok {
					return nil, fmt.Errorf("MyCustomOp requires Event input")
				}
				switch input.Port {
				case "S0":
					return []esper.DataflowEmission{esper.EmitPort("OutTwo", "S0-"+fmt.Sprint(event.Get("p1").Any()))}, nil
				case "S1":
					return []esper.DataflowEmission{esper.EmitPort("OutOne", "S1-"+fmt.Sprint(event.Get("p2").Any()))}, nil
				default:
					return nil, fmt.Errorf("MyCustomOp input port %q", input.Port)
				}
			}
			return dataflowPortsFeedbackRuntime{process: process}, nil
		}
		definition, buildErr := esper.DefineDataflow(env, "MultiInMultiOutGraph").
			BeaconEventSource("InOne", "SchemaOne", esper.DataflowBeaconOptions{Iterations: 1},
				esper.Alias("p1", esper.Literal("A1"))).
			BeaconEventSource("InTwo", "SchemaOne", esper.DataflowBeaconOptions{Iterations: 1},
				esper.Alias("p1", esper.Literal("A2"))).
			BeaconEventSource("InThree", "SchemaTwo", esper.DataflowBeaconOptions{Iterations: 1},
				esper.Alias("p2", esper.Literal(10))).
			BeaconEventSource("InFour", "SchemaTwo", esper.DataflowBeaconOptions{Iterations: 1},
				esper.Alias("p2", esper.Literal(20))).
			CustomPorts("MyCustomOp", myCustomOp, []string{"S0", "S1"}, []string{"OutOne", "OutTwo"}).
			Custom("SupportOpCountFutureOneA", captureFactory(captures["SupportOpCountFutureOneA"])).
			Custom("SupportOpCountFutureOneB", captureFactory(captures["SupportOpCountFutureOneB"])).
			Custom("SupportOpCountFutureTwoA", captureFactory(captures["SupportOpCountFutureTwoA"])).
			Custom("SupportOpCountFutureTwoB", captureFactory(captures["SupportOpCountFutureTwoB"])).
			ConnectPorts("InOne", "out", "MyCustomOp", "S0").
			ConnectPorts("InTwo", "out", "MyCustomOp", "S0").
			ConnectPorts("InThree", "out", "MyCustomOp", "S1").
			ConnectPorts("InFour", "out", "MyCustomOp", "S1").
			ConnectPorts("MyCustomOp", "OutOne", "SupportOpCountFutureOneA", "in").
			ConnectPorts("MyCustomOp", "OutOne", "SupportOpCountFutureOneB", "in").
			ConnectPorts("MyCustomOp", "OutTwo", "SupportOpCountFutureTwoA", "in").
			ConnectPorts("MyCustomOp", "OutTwo", "SupportOpCountFutureTwoB", "in").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		// The configured beacons pump on goroutines; Run joins them. Rows
		// within a capture arrive in racy order — the canonical sort freezes
		// Java's assertEqualsAnyOrder.
		if err := instance.Run(ctx); err != nil {
			return nil, err
		}
		for _, name := range []string{"SupportOpCountFutureOneA", "SupportOpCountFutureOneB", "SupportOpCountFutureTwoA", "SupportOpCountFutureTwoB"} {
			if len(captures[name].current) != 2 {
				return nil, fmt.Errorf("capture %q delivered %d rows, want 2", name, len(captures[name].current))
			}
			emit("flow:"+name, captures[name].current)
		}
	case 1: // factorial — feedback self-edge, natural completion
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowPortsFeedbackInputSchema](env, "InputSchema"); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowPortsFeedbackJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowPortsFeedbackCapture{}
		// step travels the feedback edge as a raw two-element array
		// {current, temp} — the custom operator has no env handle, so the
		// runner projects the final row positionally (p0), mirroring the
		// Java oracle's raw passthrough.
		type factStep struct {
			current int
			temp    int64
		}
		myFactorialOp := func(_ esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
			process := func(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
				switch input.Port {
				case "InputData":
					event, ok := input.Value.(esper.Event)
					if !ok {
						return nil, fmt.Errorf("MyFactorialOp requires Event input")
					}
					number, ok := event.Get("number").Any().(int)
					if !ok {
						return nil, fmt.Errorf("MyFactorialOp input number %#v", event.Get("number").Any())
					}
					return []esper.DataflowEmission{esper.EmitPort("TempResult", factStep{current: number, temp: int64(number)})}, nil
				case "TempResult":
					step, ok := input.Value.(factStep)
					if !ok {
						return nil, fmt.Errorf("MyFactorialOp feedback input %#v", input.Value)
					}
					if step.current == 1 {
						return []esper.DataflowEmission{esper.EmitPort("FinalResult", step.temp)}, nil
					}
					next := factStep{current: step.current - 1, temp: step.temp * int64(step.current-1)}
					return []esper.DataflowEmission{esper.EmitPort("TempResult", next)}, nil
				default:
					return nil, fmt.Errorf("MyFactorialOp input port %q", input.Port)
				}
			}
			return dataflowPortsFeedbackRuntime{process: process}, nil
		}
		definition, buildErr := esper.DefineDataflow(env, "FactorialGraph").
			BeaconEventSource("InputData", "InputSchema", esper.DataflowBeaconOptions{Iterations: 1},
				esper.Alias("number", esper.Literal(5))).
			CustomPorts("MyFactorialOp", myFactorialOp, []string{"InputData", "TempResult"}, []string{"TempResult", "FinalResult"}).
			Custom("DefaultSupportCaptureOp", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			ConnectPorts("InputData", "out", "MyFactorialOp", "InputData").
			ConnectFeedbackPorts("MyFactorialOp", "TempResult", "MyFactorialOp", "TempResult").
			ConnectPorts("MyFactorialOp", "FinalResult", "DefaultSupportCaptureOp", "in").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		// The feedback recursion drains and the source finishes: the
		// instance completes on its own, no cancel.
		if err := instance.Run(ctx); err != nil {
			return nil, err
		}
		if len(capture.current) != 1 {
			return nil, fmt.Errorf("factorial delivered %d rows, want 1", len(capture.current))
		}
		emit("flow:DefaultSupportCaptureOp", capture.current)
	case 2: // large-num-ops — 17-stage identity chain
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowPortsFeedbackSchemaOne](env, "SchemaOne"); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(dataflowPortsFeedbackJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		capture := &dataflowPortsFeedbackCapture{}
		identityFactory := func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
			process := func(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
				return []esper.DataflowEmission{esper.Emit(input.Value)}, nil
			}
			return dataflowPortsFeedbackRuntime{process: process}, nil
		}
		builder := esper.DefineDataflow(env, "MyGraph").
			BeaconEventSource("Source", "SchemaOne", esper.DataflowBeaconOptions{Iterations: 1},
				esper.Alias("p1", esper.Literal("A1")))
		previous := "Source"
		for stage := 0; stage < 17; stage++ {
			name := fmt.Sprintf("SelectStage%d", stage)
			builder = builder.Custom(name, identityFactory).
				Connect(previous, name)
			previous = name
		}
		definition, buildErr := builder.
			Custom("DefaultSupportCaptureOp", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			Connect(previous, "DefaultSupportCaptureOp").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		if err := instance.Run(ctx); err != nil {
			return nil, err
		}
		if len(capture.current) != 1 {
			return nil, fmt.Errorf("large-num-ops delivered %d rows, want 1", len(capture.current))
		}
		// The identity stages forward the beacon's materialized event; the
		// Java capture reads the raw Object[] underlying {"A1"} — project
		// positionally via the event's underlying.
		event, ok := capture.current[0].(esper.Event)
		if !ok {
			return nil, fmt.Errorf("large-num-ops capture %#v, want event", capture.current[0])
		}
		emit("flow:DefaultSupportCaptureOp", []any{event.Get("p1").Any()})
	default:
		return nil, fmt.Errorf("unsupported dataflow-ports-feedback case index %d", caseIndex)
	}
	return records, nil
}

// OnSignal swallows signals: Java's MyCustomOp/MyFactorialOp define no
// onSignal handler, so BeaconSource final markers die at the custom
// operator (the captures' current batches are never flushed — load-bearing
// for the capture-read semantics).
func (r dataflowPortsFeedbackRuntime) OnSignal(_ context.Context, _ esper.DataflowSignal) ([]esper.DataflowEmission, error) {
	return nil, nil
}
