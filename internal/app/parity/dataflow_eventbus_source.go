package parity

import (
	"context"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the dataflow eventbus ingress family:
//   - EPLDataflowOpEventBusSource.EPLDataflowAllTypes
//     (java-runtime-dfb59d3bd4798d57cc0c): four representation sub-runs
//     (POJO, Map, XML, ObjectArray) through EventBusSource, pinning the
//     pre-start drop, the two-event fill in send order, and the post-cancel
//     drop, plus two invalid compile rejections and the doc-sample flow
//     (instantiate-only) asserted internally.
//   - EPLDataflowOpEventBusSource.EPLDataflowSchemaObjectArray
//     (java-runtime-37aed9aedcbbb25c4826): envelope, underlying, and
//     filter+collector sub-runs over a path-deployed object-array schema.
//     The underlying sub-run serializes the raw object-array positionally as
//     {"0":..,"1":..}; the collector re-submits the envelope so the row stays
//     typed. The Java emitter-context identity asserts have no Go surface
//     (documented difference; the manifest already lists collector modes).
//   - EPLDataflowOpFilter.EPLDataflowAllTypes
//     (java-runtime-73c4f6808b38087c5a59): four representation sub-runs
//     through Filter with a canned two-event source and a synchronous run,
//     the doc-sample flow (instantiate-only), and the two-stream captive
//     pass/fail shape. Java's assertSame identity at capture is not
//     representable Go-side (Filter requires Event/Row envelopes) — rows pin
//     normalized fields instead.
//   - EPLDataflowOpFilter.EPLDataflowInvalid
//     (java-runtime-0bc68f1db4dd07d89a66): invalidity policy — the five
//     compile-time rejections are asserted in-process by the oracle and
//     pinned Go-side by Build-rejection unit tests; zero trace rows.
const dataflowEventbusSourceJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowEventbusSourceJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpEventBusSource.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpFilter.java",
}

var dataflowEventbusSourceJavaRuntimeIDs = []string{
	"java-runtime-dfb59d3bd4798d57cc0c",
	"java-runtime-37aed9aedcbbb25c4826",
	"java-runtime-73c4f6808b38087c5a59",
	"java-runtime-0bc68f1db4dd07d89a66",
}

var dataflowEventbusSourceJavaExecutions = []string{
	"EPLDataflowOpEventBusSource$EPLDataflowAllTypes",
	"EPLDataflowOpEventBusSource$EPLDataflowSchemaObjectArray",
	"EPLDataflowOpFilter$EPLDataflowAllTypes",
	"EPLDataflowOpFilter$EPLDataflowInvalid",
}

var dataflowEventbusSourceCases = []string{
	"eventbus-all-types",
	"eventbus-schema-objectarray",
	"filter-all-types",
	"filter-invalid",
}

// dataflowEventbusSourceGraphEvent mirrors the three-property graph event in
// bean, Map, object-array and XML representations.
type dataflowEventbusSourceGraphEvent struct {
	MyDouble float64 `esper:"myDouble"`
	MyInt    int     `esper:"myInt"`
	MyString string  `esper:"myString"`
}

type dataflowEventbusSourceSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// dataflowEventbusSourceCapture records delivered values in arrival order.
type dataflowEventbusSourceCapture struct {
	values []any
}

func (c *dataflowEventbusSourceCapture) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	c.values = append(c.values, input.Value)
	return nil, nil
}

// dataflowEventbusSourceCollector tracks the last envelope the collector saw,
// mirroring the Java MyCollector last-context assertions.
type dataflowEventbusSourceCollector struct {
	seen bool
	last esper.Event
}

func (c *dataflowEventbusSourceCollector) Collect(_ context.Context, event esper.Event) ([]any, error) {
	c.seen = true
	c.last = event
	return []any{event}, nil
}

func runDataflowEventbusSourceScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-eventbus-source scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowEventbusSourceCases {
		caseTrace, err := runDataflowEventbusSourceCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-eventbus-source case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowEventbusSourceCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	emit := func(statement string, capture *dataflowEventbusSourceCapture) {
		values := capture.values
		sequence++
		rows := make([]compat.ResultRecord, 0, len(values))
		for _, value := range values {
			switch typed := value.(type) {
			case esper.Event:
				rows = append(rows, compat.NormalizeEvents([]esper.Event{typed})...)
			case []any:
				fields := make(map[string]any, len(typed))
				for index, item := range typed {
					fields[fmt.Sprintf("%d", index)] = item
				}
				rows = append(rows, compat.ResultRecord{Kind: "row", Fields: fields})
			default:
				rows = append(rows, compat.ResultRecord{Kind: "row", Fields: map[string]any{}})
			}
		}
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: "capture",
			Statement: statement,
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			New:       rows,
		})
		capture.values = nil
	}

	switch caseIndex {
	case 0: // eventbus-all-types — four representation sub-runs
		representations := []struct {
			eventTypeName string
			register      func(*esper.Environment) error
			first         any
			second        any
			send          func(*esper.Engine, any) error
		}{
			{eventTypeName: "MyDefaultSupportGraphEvent",
				register: func(env *esper.Environment) error {
					_, err := esper.RegisterStruct[dataflowEventbusSourceGraphEvent](env, "MyDefaultSupportGraphEvent")
					return err
				},
				first:  dataflowEventbusSourceGraphEvent{MyDouble: 1.1, MyInt: 1, MyString: "one"},
				second: dataflowEventbusSourceGraphEvent{MyDouble: 2.2, MyInt: 2, MyString: "two"},
				send: func(engine *esper.Engine, value any) error {
					return engine.SendEvent(ctx, value)
				}},
			{eventTypeName: "MyMapEvent",
				register: func(env *esper.Environment) error {
					_, err := esper.RegisterMap(env, "MyMapEvent", []esper.FieldSpec{
						esper.FieldDef("myDouble", reflect.TypeOf(float64(0))),
						esper.FieldDef("myInt", reflect.TypeOf(int(0))),
						esper.FieldDef("myString", reflect.TypeOf("")),
					})
					return err
				},
				first:  map[string]any{"myDouble": 1.1, "myInt": 1, "myString": "one"},
				second: map[string]any{"myDouble": 2.2, "myInt": 2, "myString": "two"},
				send: func(engine *esper.Engine, value any) error {
					return engine.Send(ctx, "MyMapEvent", value)
				}},
			{eventTypeName: "MyXMLEvent",
				register: func(env *esper.Environment) error {
					_, err := esper.RegisterXML(env, "MyXMLEvent", []esper.FieldSpec{
						esper.FieldDef("myDouble", reflect.TypeOf(float64(0))),
						esper.FieldDef("myInt", reflect.TypeOf(int(0))),
						esper.FieldDef("myString", reflect.TypeOf("")),
					})
					return err
				},
				first:  "<MyXMLEvent><myDouble>1.1</myDouble><myInt>1</myInt><myString>one</myString></MyXMLEvent>",
				second: "<MyXMLEvent><myDouble>2.2</myDouble><myInt>2</myInt><myString>two</myString></MyXMLEvent>",
				send: func(engine *esper.Engine, value any) error {
					return engine.SendXML(ctx, "MyXMLEvent", []byte(value.(string)))
				}},
			{eventTypeName: "MyOAEvent",
				register: func(env *esper.Environment) error {
					_, err := esper.RegisterObjectArray(env, "MyOAEvent", []esper.FieldSpec{
						esper.FieldDef("myDouble", reflect.TypeOf(float64(0))),
						esper.FieldDef("myInt", reflect.TypeOf(int(0))),
						esper.FieldDef("myString", reflect.TypeOf("")),
					})
					return err
				},
				first:  []any{1.1, 1, "one"},
				second: []any{2.2, 2, "two"},
				send: func(engine *esper.Engine, value any) error {
					return engine.SendObjectArray(ctx, "MyOAEvent", value.([]any))
				}},
		}
		for _, representation := range representations {
			subEnv := esper.NewEnvironment()
			if err := representation.register(subEnv); err != nil {
				return nil, err
			}
			subEngine := esper.NewEngine(subEnv,
				esper.WithRuntimeURI(dataflowEventbusSourceJavaRuntimeIDs[caseIndex]),
				esper.WithStartTime(now),
			)
			// Send before instantiate: no running instance exists, so the
			// event is dropped without error (Java asserts the empty capture
			// internally, no record).
			if err := representation.send(subEngine, representation.first); err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			capture := &dataflowEventbusSourceCapture{}
			definition, buildErr := esper.DefineDataflow(subEnv, "MyDataFlowOne").
				EventBusSource("source", representation.eventTypeName).
				Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
					return capture, nil
				}).
				Connect("source", "capture").
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
			// Send after instantiate but before start: the instance is not
			// running yet, so the event is dropped (pre-start empty read).
			if err := representation.send(subEngine, representation.first); err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			emit("flow:DefaultSupportCaptureOp", capture)
			if err := instance.Start(ctx); err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			if err := representation.send(subEngine, representation.first); err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			if err := representation.send(subEngine, representation.second); err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			emit("flow:DefaultSupportCaptureOp", capture)
			if err := instance.Cancel(ctx); err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			// Send after cancel: the subscription is detached synchronously
			// (post-cancel empty read).
			if err := representation.send(subEngine, representation.first); err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			emit("flow:DefaultSupportCaptureOp", capture)
			if err := subEngine.Close(context.Background()); err != nil {
				return nil, err
			}
		}
	case 1: // eventbus-schema-objectarray — envelope, underlying, filter+collector
		// Sub-run 1: envelope — the EventBean<T> port type keeps the typed
		// envelope in the stream.
		if err := dataflowEventbusSourceSchemaSubrunEnvelope(ctx, caseIndex, now, emit); err != nil {
			return nil, err
		}
		// Sub-run 2: underlying — the raw object-array is materialized.
		if err := dataflowEventbusSourceSchemaSubrunUnderlying(ctx, caseIndex, now, emit); err != nil {
			return nil, err
		}
		// Sub-run 3: filter + collector.
		if err := dataflowEventbusSourceSchemaSubrunCollector(ctx, caseIndex, now, emit); err != nil {
			return nil, err
		}
	case 2: // filter-all-types — canned source through Filter
		representations := []struct {
			eventTypeName string
			register      func(*esper.Environment) (esper.Schema, error)
			first         any
			second        any
			parse         func(esper.Schema, any) (esper.Event, error)
		}{
			{eventTypeName: "MyDefaultSupportGraphEvent",
				register: func(env *esper.Environment) (esper.Schema, error) {
					return esper.RegisterStruct[dataflowEventbusSourceGraphEvent](env, "MyDefaultSupportGraphEvent")
				},
				first:  dataflowEventbusSourceGraphEvent{MyDouble: 1.1, MyInt: 1, MyString: "one"},
				second: dataflowEventbusSourceGraphEvent{MyDouble: 2.2, MyInt: 2, MyString: "two"},
				parse: func(schema esper.Schema, underlying any) (esper.Event, error) {
					return esper.NewEvent(schema, underlying, now)
				}},
			{eventTypeName: "MyXMLEvent",
				register: func(env *esper.Environment) (esper.Schema, error) {
					return esper.RegisterXML(env, "MyXMLEvent", []esper.FieldSpec{
						esper.FieldDef("myDouble", reflect.TypeOf(float64(0))),
						esper.FieldDef("myInt", reflect.TypeOf(int(0))),
						esper.FieldDef("myString", reflect.TypeOf("")),
					})
				},
				first:  "<MyXMLEvent><myDouble>1.1</myDouble><myInt>1</myInt><myString>one</myString></MyXMLEvent>",
				second: "<MyXMLEvent><myDouble>2.2</myDouble><myInt>2</myInt><myString>two</myString></MyXMLEvent>",
				parse: func(schema esper.Schema, raw any) (esper.Event, error) {
					return esper.ParseXML(schema, []byte(raw.(string)), now)
				}},
			{eventTypeName: "MyOAEvent",
				register: func(env *esper.Environment) (esper.Schema, error) {
					return esper.RegisterObjectArray(env, "MyOAEvent", []esper.FieldSpec{
						esper.FieldDef("myDouble", reflect.TypeOf(float64(0))),
						esper.FieldDef("myInt", reflect.TypeOf(int(0))),
						esper.FieldDef("myString", reflect.TypeOf("")),
					})
				},
				first:  []any{1.1, 1, "one"},
				second: []any{2.2, 2, "two"},
				parse: func(schema esper.Schema, underlying any) (esper.Event, error) {
					return esper.ParseObjectArray(schema, underlying.([]any), now)
				}},
			{eventTypeName: "MyMapEvent",
				register: func(env *esper.Environment) (esper.Schema, error) {
					return esper.RegisterMap(env, "MyMapEvent", []esper.FieldSpec{
						esper.FieldDef("myDouble", reflect.TypeOf(float64(0))),
						esper.FieldDef("myInt", reflect.TypeOf(int(0))),
						esper.FieldDef("myString", reflect.TypeOf("")),
					})
				},
				first:  map[string]any{"myDouble": 1.1, "myInt": 1, "myString": "one"},
				second: map[string]any{"myDouble": 2.2, "myInt": 2, "myString": "two"},
				parse: func(schema esper.Schema, underlying any) (esper.Event, error) {
					return esper.NewEvent(schema, underlying, now)
				}},
		}
		for _, representation := range representations {
			subEnv := esper.NewEnvironment()
			schema, err := representation.register(subEnv)
			if err != nil {
				return nil, err
			}
			first, err := representation.parse(schema, representation.first)
			if err != nil {
				return nil, err
			}
			second, err := representation.parse(schema, representation.second)
			if err != nil {
				return nil, err
			}
			subEngine := esper.NewEngine(subEnv,
				esper.WithRuntimeURI(dataflowEventbusSourceJavaRuntimeIDs[caseIndex]),
				esper.WithStartTime(now),
			)
			capture := &dataflowEventbusSourceCapture{}
			definition, buildErr := esper.DefineDataflow(subEnv, "MySelect").
				BeaconSource("source", first, second).
				Filter("filter", esper.Equal[string](esper.Field[any, string]("myString"), esper.Literal("two"))).
				Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
					return capture, nil
				}).
				Connect("source", "filter").
				Connect("filter", "capture").
				Build()
			if buildErr != nil {
				_ = subEngine.Close(context.Background())
				return nil, buildErr
			}
			instance, err := subEngine.InstantiateDataflowWithOptions(ctx, definition, esper.DataflowOptions{
				InstanceID: "myinstanceid",
				UserObject: "myuserobject",
			})
			if err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			if instance.InstanceID() != "myinstanceid" || instance.UserObject() != "myuserobject" {
				_ = subEngine.Close(context.Background())
				return nil, fmt.Errorf("dataflow-eventbus-source case %q: instance identity mismatch", caseName)
			}
			if err := instance.Run(ctx); err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			emit("flow:DefaultSupportCaptureOp", capture)
			if err := subEngine.Close(context.Background()); err != nil {
				return nil, err
			}
		}
		// Two-streams captive: Emitter -> Filter(sb)->out.ok,out.fail with
		// static capture semantics on both ports.
		if err := dataflowEventbusSourceFilterTwoStreams(ctx, caseIndex, now, emit); err != nil {
			return nil, err
		}
	case 3: // filter-invalid — invalidity policy, zero records
		// The five compile-time rejections are pinned by
		// TestDataflowEventbusSourceFilterInvalidRejections.
		return records, nil
	default:
		return nil, fmt.Errorf("unsupported dataflow-eventbus-source case index %d", caseIndex)
	}
	return records, nil
}

// dataflowEventbusSourceSchemaSubrunEnvelope replays the EventBean<MyEventOA>
// envelope sub-run: one send, one typed row.
func dataflowEventbusSourceSchemaSubrunEnvelope(ctx context.Context, caseIndex int, now time.Time,
	emit func(statement string, capture *dataflowEventbusSourceCapture)) error {
	subEnv := esper.NewEnvironment()
	if _, err := esper.RegisterObjectArray(subEnv, "MyEventOA", []esper.FieldSpec{
		esper.FieldDef("p0", reflect.TypeOf("")),
		esper.FieldDef("p1", reflect.TypeOf(int64(0))),
	}); err != nil {
		return err
	}
	subEngine := esper.NewEngine(subEnv,
		esper.WithRuntimeURI(dataflowEventbusSourceJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(now),
	)
	defer func() { _ = subEngine.Close(context.Background()) }()
	capture := &dataflowEventbusSourceCapture{}
	definition, buildErr := esper.DefineDataflow(subEnv, "MyDataFlowOne").
		EventBusSource("source", "MyEventOA").
		Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
			return capture, nil
		}).
		Connect("source", "capture").
		Build()
	if buildErr != nil {
		return buildErr
	}
	instance, err := subEngine.InstantiateDataflow(ctx, definition)
	if err != nil {
		return err
	}
	if err := instance.Start(ctx); err != nil {
		return err
	}
	if err := subEngine.SendObjectArray(ctx, "MyEventOA", []any{"abc", int64(100)}); err != nil {
		return err
	}
	emit("flow:DefaultSupportCaptureOp", capture)
	return instance.Cancel(ctx)
}

// dataflowEventbusSourceSchemaSubrunUnderlying replays the raw MyEventOA
// underlying sub-run; the capture receives the raw object-array and the
// runner projects it positionally.
func dataflowEventbusSourceSchemaSubrunUnderlying(ctx context.Context, caseIndex int, now time.Time,
	emit func(statement string, capture *dataflowEventbusSourceCapture)) error {
	subEnv := esper.NewEnvironment()
	if _, err := esper.RegisterObjectArray(subEnv, "MyEventOA", []esper.FieldSpec{
		esper.FieldDef("p0", reflect.TypeOf("")),
		esper.FieldDef("p1", reflect.TypeOf(int64(0))),
	}); err != nil {
		return err
	}
	subEngine := esper.NewEngine(subEnv,
		esper.WithRuntimeURI(dataflowEventbusSourceJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(now),
	)
	defer func() { _ = subEngine.Close(context.Background()) }()
	capture := &dataflowEventbusSourceCapture{}
	definition, buildErr := esper.DefineDataflow(subEnv, "MyDataFlowOne").
		EventBusSourceWithUnderlying("source", "MyEventOA").
		Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
			return capture, nil
		}).
		Connect("source", "capture").
		Build()
	if buildErr != nil {
		return buildErr
	}
	instance, err := subEngine.InstantiateDataflow(ctx, definition)
	if err != nil {
		return err
	}
	if err := instance.Start(ctx); err != nil {
		return err
	}
	if err := subEngine.SendObjectArray(ctx, "MyEventOA", []any{"abc", int64(100)}); err != nil {
		return err
	}
	emit("flow:DefaultSupportCaptureOp", capture)
	return instance.Cancel(ctx)
}

// dataflowEventbusSourceSchemaSubrunCollector replays the filter+collector
// sub-run: p0 like 'A%' suppresses the B row (explicit empty record) and the
// collector re-submits the A envelope.
func dataflowEventbusSourceSchemaSubrunCollector(ctx context.Context, caseIndex int, now time.Time,
	emit func(statement string, capture *dataflowEventbusSourceCapture)) error {
	subEnv := esper.NewEnvironment()
	if _, err := esper.RegisterObjectArray(subEnv, "MyEventOA", []esper.FieldSpec{
		esper.FieldDef("p0", reflect.TypeOf("")),
		esper.FieldDef("p1", reflect.TypeOf(int64(0))),
	}); err != nil {
		return err
	}
	subEngine := esper.NewEngine(subEnv,
		esper.WithRuntimeURI(dataflowEventbusSourceJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(now),
	)
	defer func() { _ = subEngine.Close(context.Background()) }()
	collector := &dataflowEventbusSourceCollector{}
	capture := &dataflowEventbusSourceCapture{}
	definition, buildErr := esper.DefineDataflow(subEnv, "MyDataFlowOne").
		EventBusSourceWithFilterAndCollector("source", "MyEventOA",
			esper.Like(esper.Field[any, string]("p0"), esper.Literal("A%")),
			collector.Collect).
		Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
			return capture, nil
		}).
		Connect("source", "capture").
		Build()
	if buildErr != nil {
		return buildErr
	}
	instance, err := subEngine.InstantiateDataflow(ctx, definition)
	if err != nil {
		return err
	}
	if err := instance.Start(ctx); err != nil {
		return err
	}
	// The B row fails the filter: the collector never sees it (Java
	// assertNull(collector.getLast()) plus an empty capture read).
	if err := subEngine.SendObjectArray(ctx, "MyEventOA", []any{"B", int64(100)}); err != nil {
		return err
	}
	if collector.seen {
		return fmt.Errorf("collector invoked for filtered-out event")
	}
	emit("flow:DefaultSupportCaptureOp", capture)
	if err := subEngine.SendObjectArray(ctx, "MyEventOA", []any{"A", int64(101)}); err != nil {
		return err
	}
	if !collector.seen {
		return fmt.Errorf("collector not invoked for accepted event")
	}
	if collector.last.TypeName() != "MyEventOA" {
		return fmt.Errorf("collector envelope type %q, want MyEventOA", collector.last.TypeName())
	}
	emit("flow:DefaultSupportCaptureOp", capture)
	return instance.Cancel(ctx)
}

// dataflowEventbusSourceFilterTwoStreams replays the two-stream captive
// Filter shape: pass rows route to out.ok, fail rows to out.fail.
func dataflowEventbusSourceFilterTwoStreams(ctx context.Context, caseIndex int, now time.Time,
	emit func(statement string, capture *dataflowEventbusSourceCapture)) error {
	subEnv := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[dataflowEventbusSourceSupportBean](subEnv, "SupportBean"); err != nil {
		return err
	}
	subEngine := esper.NewEngine(subEnv,
		esper.WithRuntimeURI(dataflowEventbusSourceJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(now),
	)
	defer func() { _ = subEngine.Close(context.Background()) }()
	okCapture := &dataflowEventbusSourceCapture{}
	failCapture := &dataflowEventbusSourceCapture{}
	definition, buildErr := esper.DefineDataflow(subEnv, "MyFilter").
		Emitter("e1").
		FilterWithPorts("filter",
			esper.Equal[string](esper.Field[any, string]("theString"), esper.Literal("x")),
			"out.ok", "out.fail").
		Custom("ok", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
			return okCapture, nil
		}).
		Custom("fail", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
			return failCapture, nil
		}).
		Connect("e1", "filter").
		ConnectPorts("filter", "out.ok", "ok", "in").
		ConnectPorts("filter", "out.fail", "fail", "in").
		Build()
	if buildErr != nil {
		return buildErr
	}
	instance, err := subEngine.InstantiateDataflow(ctx, definition)
	if err != nil {
		return err
	}
	captive, err := instance.StartCaptive(ctx)
	if err != nil {
		return err
	}
	emitter, ok := captive.Emitters()["e1"]
	if !ok || emitter == nil {
		return fmt.Errorf("dataflow-eventbus-source case filter-all-types: captive emitter e1 missing")
	}
	if err := emitter.Submit(ctx, dataflowEventbusSourceSupportBean{TheString: "x", IntPrimitive: 10}); err != nil {
		return err
	}
	emit("flow:DefaultSupportCaptureOpStatic", okCapture)
	if err := emitter.Submit(ctx, dataflowEventbusSourceSupportBean{TheString: "y", IntPrimitive: 11}); err != nil {
		return err
	}
	emit("flow:DefaultSupportCaptureOpStatic", failCapture)
	return instance.Cancel(ctx)
}
