package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EPLDataflowOpSelectWrapper: the select-star
// representation flows over an EventBusSource feeding a passthrough Select.
// The no-additional-props variant projects the plain map surface {value};
// the additional-props variant's insert-into makes B a decorated type whose
// capture surface is the union {value, hello} — the Java Pair split
// (underlying vs additional namespaces) is representation-only metadata and
// is documented in the manifest difference rather than pinned here.
type dataflowSelectRepresentationA struct {
	Value int `esper:"value"`
}

type dataflowSelectRepresentationB struct {
	Value int    `esper:"value"`
	Hello string `esper:"hello"`
}

const dataflowSelectRepresentationJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowSelectRepresentationJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpSelect.java",
}

var dataflowSelectRepresentationJavaRuntimeIDs = []string{
	"java-runtime-9550df89e223e839e414",
	"java-runtime-13cbd82acb5791b30d07",
}

var dataflowSelectRepresentationJavaExecutions = []string{
	"EPLDataflowOpSelectWrapper{wrapperWithAdditionalProps=false}",
	"EPLDataflowOpSelectWrapper{wrapperWithAdditionalProps=true}",
}

var dataflowSelectRepresentationCases = []string{
	"select-wrapper-no-additional-props",
	"select-wrapper-additional-props",
}

// dataflowSelectRepresentationCapture records delivered events in arrival
// order.
type dataflowSelectRepresentationCapture struct {
	events []esper.Event
}

func (c *dataflowSelectRepresentationCapture) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	if event, ok := input.Value.(esper.Event); ok {
		c.events = append(c.events, event)
		return nil, nil
	}
	return nil, fmt.Errorf("capture expected an event, got %T", input.Value)
}

func runDataflowSelectRepresentationScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-select-representation scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowSelectRepresentationCases {
		caseTrace, err := runDataflowSelectRepresentationCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-select-representation case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowSelectRepresentationCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	now := time.Unix(0, 0).UTC()
	capture := &dataflowSelectRepresentationCapture{}

	definition, err := func() (esper.DataflowDefinition, error) {
		switch caseIndex {
		case 0: // select-wrapper-no-additional-props: plain A passthrough
			if _, err := esper.RegisterStruct[dataflowSelectRepresentationA](env, "A"); err != nil {
				return esper.DataflowDefinition{}, err
			}
			return esper.DefineDataflow(env, "OutputFlow").
				Emitter("source").
				SelectPassThrough("select").
				Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
					return capture, nil
				}).
				Connect("source", "select").
				Connect("select", "capture").
				Build()
		case 1: // select-wrapper-additional-props: B carries value + hello
			if _, err := esper.RegisterStruct[dataflowSelectRepresentationB](env, "B"); err != nil {
				return esper.DataflowDefinition{}, err
			}
			return esper.DefineDataflow(env, "OutputFlow").
				Emitter("source").
				SelectPassThrough("select").
				Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
					return capture, nil
				}).
				Connect("source", "select").
				Connect("select", "capture").
				Build()
		default:
			return esper.DataflowDefinition{}, fmt.Errorf("unsupported dataflow-select-representation case index %d", caseIndex)
		}
	}()
	if err != nil {
		return nil, err
	}

	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(dataflowSelectRepresentationJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(now),
	)
	defer func() { _ = engine.Close(context.Background()) }()

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
	switch caseIndex {
	case 0:
		if err := emitter.Submit(ctx, dataflowSelectRepresentationA{Value: 10}); err != nil {
			return nil, err
		}
	case 1:
		if err := emitter.Submit(ctx, dataflowSelectRepresentationB{Value: 10, Hello: "a"}); err != nil {
			return nil, err
		}
	}
	if err := instance.Cancel(ctx); err != nil {
		return nil, err
	}

	sequence := uint64(1)
	return []compat.TraceRecord{{
		Case:      caseName,
		Operation: "capture",
		Statement: "flow:DefaultSupportCaptureOp",
		Sequence:  sequence,
		Time:      compat.FormatTraceTime(now),
		New:       compat.NormalizeEvents(capture.events),
	}}, nil
}
