package parity

import (
	"context"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EPLDataflowTypes: dataflow type declarations over a
// bean class and a map schema. Each execution declares a single-source graph
// fanned out to two capture sinks, submits one canned event, and drains via
// the blocking run — mirroring Java's DefaultSupportSourceOp instruction,
// DefaultSupportGraphOpProvider injection, and the port-0 delivery of the
// generic WPort sink (pinned in the trace as the capture-port-0 operation).
type dataflowTypesBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const dataflowTypesJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowTypesJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowTypes.java",
}

var dataflowTypesJavaRuntimeIDs = []string{
	"java-runtime-a277455fea9aac5b640b",
	"java-runtime-8006afc99e90b53a008c",
}

var dataflowTypesJavaExecutions = []string{
	"EPLDataflowBeanType",
	"EPLDataflowMapType",
}

var dataflowTypesCases = []string{
	"bean-type",
	"map-type",
}

// dataflowTypesCapture records the events an input delivered, mirroring
// MySupportBeanOutputOp/MyMapOutputOp/DefaultSupportCaptureOp captures and
// SupportGenericOutputOpWPort's (event, port) pairs.
type dataflowTypesCapture struct {
	schema esper.Schema
	now    time.Time
	events []esper.Event
	ports  []string
}

// Process implements the custom-operator contract: record the delivered
// event and its input port, emit nothing.
func (c *dataflowTypesCapture) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	c.ports = append(c.ports, input.Port)
	switch value := input.Value.(type) {
	case esper.Event:
		c.events = append(c.events, value)
	default:
		event, err := esper.NewEvent(c.schema, value, c.now)
		if err != nil {
			return nil, err
		}
		c.events = append(c.events, event)
	}
	return nil, nil
}

func runDataflowTypesScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-types scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowTypesCases {
		caseTrace, err := runDataflowTypesCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-types case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowTypesCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	var schema esper.Schema
	var literal any
	switch caseIndex {
	case 0:
		if _, err := esper.RegisterStruct[dataflowTypesBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		registered, ok := env.Schema("SupportBean")
		if !ok {
			return nil, fmt.Errorf("SupportBean schema missing")
		}
		schema = registered
		literal = dataflowTypesBean{TheString: "E1", IntPrimitive: 1}
	case 1:
		if _, err := esper.RegisterMap(env, "MyMap", []esper.FieldSpec{
			esper.FieldDef("p0", reflect.TypeOf("")),
			esper.FieldDef("p1", reflect.TypeOf(int(0))),
		}); err != nil {
			return nil, err
		}
		registered, ok := env.Schema("MyMap")
		if !ok {
			return nil, fmt.Errorf("MyMap schema missing")
		}
		schema = registered
		event, err := esper.NewEvent(schema, map[string]any{"p0": "E1", "p1": 1}, time.Unix(0, 0).UTC())
		if err != nil {
			return nil, err
		}
		literal = event
	default:
		return nil, fmt.Errorf("unsupported dataflow-types case index %d", caseIndex)
	}

	now := time.Unix(0, 0).UTC()
	sinkOne := &dataflowTypesCapture{schema: schema, now: now}
	sinkTwo := &dataflowTypesCapture{schema: schema, now: now}
	capture := func(sink *dataflowTypesCapture) esper.DataflowOperatorFactory {
		return func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
			return sink, nil
		}
	}

	definition, err := esper.DefineDataflow(env, "MyDataFlowOne").
		BeaconSource("source", literal).
		Custom("output-one", capture(sinkOne)).
		Custom("output-two", capture(sinkTwo)).
		Connect("source", "output-one").
		Connect("source", "output-two").
		Build()
	if err != nil {
		return nil, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(dataflowTypesJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(now),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	instance, err := engine.InstantiateDataflow(ctx, definition)
	if err != nil {
		return nil, err
	}
	if err := instance.Run(ctx); err != nil {
		return nil, err
	}

	// One sink row each; the WPort sink pins the numeric port as the
	// operation suffix (Java asserts delivery on port 0).
	sinks := []struct {
		operation string
		statement string
		capture   *dataflowTypesCapture
	}{
		{"capture", "flow:MySupportBeanOutputOp", sinkOne},
		{"capture-port-0", "flow:SupportGenericOutputOpWPort", sinkTwo},
	}
	if caseIndex == 1 {
		sinks[0].statement = "flow:MyMapOutputOp"
		sinks[1].statement = "flow:DefaultSupportCaptureOp"
		sinks[1].operation = "capture"
	}
	sequence := uint64(0)
	var records []compat.TraceRecord
	for _, sink := range sinks {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: sink.operation,
			Statement: sink.statement,
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			New:       compat.NormalizeEvents(sink.capture.events),
		})
	}
	return records, nil
}
