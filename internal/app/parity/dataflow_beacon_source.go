package parity

import (
	"context"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EPLDataflowOpBeaconSource: four BeaconSource operator
// configurations exercising the fundamental dataflow source operator.
type dataflowBeaconSourceBean struct {
	MyField string `esper:"myfield"`
}

// dataflowBeaconSourceNoDefaultCtor mirrors the Java MyEventNoDefaultCtor
// sibling type; Go materializes it by zero-value allocation instead of Java
// constructor selection (documented intentional difference in the case's
// manifest difference field).
type dataflowBeaconSourceNoDefaultCtor struct {
	MyField string `esper:"myfield"`
}

const dataflowBeaconSourceJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowBeaconSourceJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpBeaconSource.java",
}

var dataflowBeaconSourceJavaRuntimeIDs = []string{
	"java-runtime-5438f7be9b56ca119b0c",
	"java-runtime-f54de0ae037b90b24551",
	"java-runtime-aac755fc7592bf62718d",
	"java-runtime-f1fc7e290467bab36366",
}

var dataflowBeaconSourceJavaExecutions = []string{
	"EPLDataflowBeaconWithBeans",
	"EPLDataflowBeaconVariable",
	"EPLDataflowBeaconFields",
	"EPLDataflowBeaconNoType",
}

var dataflowBeaconSourceCases = []string{
	"beacon-with-beans",
	"beacon-variable",
	"beacon-no-type-iterations",
	"beacon-no-type-instantiate-only",
}

// dataflowBeaconSourceCapture records delivered values in arrival order.
type dataflowBeaconSourceCapture struct {
	values []any
}

func (c *dataflowBeaconSourceCapture) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	c.values = append(c.values, input.Value)
	return nil, nil
}

func runDataflowBeaconSourceScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-beacon-source scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowBeaconSourceCases {
		caseTrace, err := runDataflowBeaconSourceCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-beacon-source case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowBeaconSourceCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(dataflowBeaconSourceJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	emit := func(values []any) {
		sequence++
		rows := make([]compat.ResultRecord, 0, len(values))
		for _, value := range values {
			if event, ok := value.(esper.Event); ok {
				rows = append(rows, compat.NormalizeEvents([]esper.Event{event})...)
			} else {
				rows = append(rows, compat.ResultRecord{Kind: "row", Fields: map[string]any{}})
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

	switch caseIndex {
	case 0: // beacon-with-beans — two standalone sub-runs, each 1 event
		// {myfield:"abc"}, one per legacy bean type (Java runAssertionBeans
		// per type with undeployAll between sub-runs, mirrored by a fresh
		// environment per sub-run since the Go definition registry has no
		// removal API).
		for run := 0; run < 2; run++ {
			typeName := "MyLegacyEvent"
			if run == 1 {
				typeName = "MyEventNoDefaultCtor"
			}
			subEnv := esper.NewEnvironment()
			if _, err := esper.RegisterStruct[dataflowBeaconSourceBean](subEnv, "MyLegacyEvent"); err != nil {
				return nil, err
			}
			if _, err := esper.RegisterStruct[dataflowBeaconSourceNoDefaultCtor](subEnv, "MyEventNoDefaultCtor"); err != nil {
				return nil, err
			}
			subEngine := esper.NewEngine(subEnv,
				esper.WithRuntimeURI(dataflowBeaconSourceJavaRuntimeIDs[caseIndex]),
				esper.WithStartTime(time.Unix(0, 0).UTC()),
			)
			capture := &dataflowBeaconSourceCapture{}
			definition, buildErr := esper.DefineDataflow(subEnv, "MyDataFlowOne").
				BeaconEventSource("BeaconSource", typeName,
					esper.DataflowBeaconOptions{Iterations: 1},
					esper.Alias("myfield", esper.Literal("abc"))).
				Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
					return capture, nil
				}).
				Connect("BeaconSource", "capture").
				Build()
			if buildErr != nil {
				_ = subEngine.Close(context.Background())
				return nil, buildErr
			}
			instance, err := subEngine.InstantiateDataflow(ctx, definition)
			if err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			if err := instance.Run(ctx); err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			emit(capture.values)
			if err := subEngine.Close(context.Background()); err != nil {
				return nil, err
			}
		}
	case 1: // beacon-variable — iterations from a variable
		if err := env.RegisterVariable("var_iterations", int64(3)); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterMap(env, "SomeEvent", nil); err != nil {
			return nil, err
		}
		capture := &dataflowBeaconSourceCapture{}
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowOne").
			BeaconEventSource("BeaconSource", "SomeEvent",
				esper.DataflowBeaconOptions{IterationsExpression: esper.VariableRef[int64]("var_iterations")}).
			Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			Connect("BeaconSource", "capture").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		// Finite variable-derived iterations drain deterministically under
		// the blocking run, matching the Java latch join.
		if err := instance.Run(ctx); err != nil {
			return nil, err
		}
		emit(capture.values)
	case 2: // beacon-no-type-iterations — 5 empty events
		capture := &dataflowBeaconSourceCapture{}
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowTwo").
			BeaconSourceWithOptions("BeaconSource", esper.DataflowBeaconOptions{Iterations: 5}).
			Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			Connect("BeaconSource", "capture").
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
		emit(capture.values)
	case 3: // beacon-no-type-instantiate-only — never started; the Java graph
		// has no sink, so instantiation succeeding is the observable.
		if _, err := esper.RegisterObjectArray(env, "MyTestOAType", []esper.FieldSpec{
			esper.FieldDef("p1", reflect.TypeOf("")),
		}); err != nil {
			return nil, err
		}
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowFive").
			BeaconEventSourceWithUnderlying("BeaconSource", "MyTestOAType",
				esper.DataflowBeaconOptions{Interval: 500 * time.Millisecond},
				esper.Alias("p1", esper.Literal("abc"))).
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		// Never started — instantiation success is the observable.
		_ = instance
	default:
		return nil, fmt.Errorf("unsupported dataflow-beacon-source case index %d", caseIndex)
	}
	return records, nil
}
