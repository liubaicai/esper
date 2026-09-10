package parity

import (
	"context"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the dataflow eventbus sink family:
//   - EPLDataflowOpEventBusSink.EPLDataflowAllTypes
//     (java-runtime-e6b4bb618f70b384cf03): four representation sub-runs
//     (XML, ObjectArray, Map, POJO in suite order) through EventBusSink into
//     a deployed select statement, pinning two synchronous listener
//     deliveries in send order; the EventBusSink output-stream compile
//     rejection is pinned Go-side by a Build-rejection unit test. The
//     two-sink doc-sample flow (instantiate-only, the dataflow statement
//     itself named s0) is exercised only by the Java oracle — it contributes
//     zero records on both sides and is not replayed Go-side because the
//     class-name collector configuration has no Go surface.
//   - EPLDataflowOpEventBusSink.EPLDataflowBeacon
//     (java-runtime-1ecb769e10b4a818772c): a configured BeaconSource with
//     iterations 3 and constant p0='abc', p1=1 feeding EventBusSink; three
//     listener deliveries in generation order under the bounded-wait
//     adaptation. Java's loose 0<p1<10 bound is satisfied by the constant.
//   - EPLDataflowOpEventBusSink.EPLDataflowSendEventDynamicType
//     (java-runtime-11a7b321e6901dbad740): a sink collector routing raw
//     object-arrays on their leading type field to two @buseventtype
//     schemas, with full-row projections (including the type field) pinned —
//     stronger than Java's subset assertion.
//
// Record protocol: one record per listener callback
// {case, operation:"listener", statement, sequence, time, new:[rows]} with
// case-local sequences restarting at 1 and fixed epoch time.
const dataflowEventbusSinkJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowEventbusSinkJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpEventBusSink.java",
}

var dataflowEventbusSinkJavaRuntimeIDs = []string{
	"java-runtime-e6b4bb618f70b384cf03",
	"java-runtime-1ecb769e10b4a818772c",
	"java-runtime-11a7b321e6901dbad740",
}

var dataflowEventbusSinkJavaExecutions = []string{
	"EPLDataflowOpEventBusSink$EPLDataflowAllTypes",
	"EPLDataflowOpEventBusSink$EPLDataflowBeacon",
	"EPLDataflowOpEventBusSink$EPLDataflowSendEventDynamicType",
}

var dataflowEventbusSinkCases = []string{
	"eventbus-sink-all-types",
	"eventbus-sink-beacon",
	"eventbus-sink-dynamic-type",
}

// dataflowEventbusSinkGraphEvent mirrors the three-property graph event in
// bean, Map, object-array and XML representations.
type dataflowEventbusSinkGraphEvent struct {
	MyDouble float64 `esper:"myDouble"`
	MyInt    int     `esper:"myInt"`
	MyString string  `esper:"myString"`
}

// dataflowEventbusSinkBeaconBean mirrors the MyEventBeacon path schema.
type dataflowEventbusSinkBeaconBean struct {
	P0 string `esper:"p0"`
	P1 int64  `esper:"p1"`
}

// dataflowEventbusSinkDelivery is one subscribed listener batch tagged with
// the receiving statement name.
type dataflowEventbusSinkDelivery struct {
	statement string
	batch     esper.ResultBatch
}

func runDataflowEventbusSinkScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-eventbus-sink scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowEventbusSinkCases {
		caseTrace, err := runDataflowEventbusSinkCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-eventbus-sink case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowEventbusSinkCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	emit := func(statement string, batch esper.ResultBatch) {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			New:       compat.NormalizeResults(batch.New),
		})
	}

	switch caseIndex {
	case 0: // eventbus-sink-all-types — four representation sub-runs
		representations := []struct {
			eventTypeName string
			register      func(*esper.Environment) (esper.Schema, error)
			first         any
			second        any
			parse         func(esper.Schema, any) (esper.Event, error)
		}{
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
			{eventTypeName: "MyDefaultSupportGraphEvent",
				register: func(env *esper.Environment) (esper.Schema, error) {
					return esper.RegisterStruct[dataflowEventbusSinkGraphEvent](env, "MyDefaultSupportGraphEvent")
				},
				first:  dataflowEventbusSinkGraphEvent{MyDouble: 1.1, MyInt: 1, MyString: "one"},
				second: dataflowEventbusSinkGraphEvent{MyDouble: 2.2, MyInt: 2, MyString: "two"},
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
				esper.WithRuntimeURI(dataflowEventbusSinkJavaRuntimeIDs[caseIndex]),
				esper.WithStartTime(now),
			)
			definition, buildErr := esper.DefineDataflow(subEnv, "MyGraph").
				BeaconSource("source", first, second).
				EventBusSink("sink", representation.eventTypeName).
				Connect("source", "sink").
				Build()
			if buildErr != nil {
				_ = subEngine.Close(context.Background())
				return nil, buildErr
			}
			plan, err := subEnv.Build(esper.FromAny(subEnv, representation.eventTypeName).
				Query(esper.StatementName("s0")))
			if err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			deployment, err := subEngine.Deploy(ctx, plan)
			if err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			statement := deployment.Statements()[0]
			var deliveries []esper.ResultBatch
			if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				deliveries = append(deliveries, batch)
				return nil
			}); err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			instance, err := subEngine.InstantiateDataflow(ctx, definition)
			if err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			// The canned source drains synchronously inside the run: both
			// listener deliveries complete before Run returns.
			if err := instance.Run(ctx); err != nil {
				_ = subEngine.Close(context.Background())
				return nil, err
			}
			if len(deliveries) != 2 {
				_ = subEngine.Close(context.Background())
				return nil, fmt.Errorf("sub-run %s delivered %d batches, want 2", representation.eventTypeName, len(deliveries))
			}
			emit("s0", deliveries[0])
			emit("s0", deliveries[1])
			if err := subEngine.Close(context.Background()); err != nil {
				return nil, err
			}
		}
	case 1: // eventbus-sink-beacon — configured beacon through the sink
		subEnv := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[dataflowEventbusSinkBeaconBean](subEnv, "MyEventBeacon"); err != nil {
			return nil, err
		}
		definition, buildErr := esper.DefineDataflow(subEnv, "MyDataFlowOne").
			BeaconEventSource("source", "MyEventBeacon", esper.DataflowBeaconOptions{Iterations: 3},
				esper.Alias("p0", esper.Literal("abc")),
				esper.Alias("p1", esper.Literal(int64(1))),
			).
			EventBusSink("sink", "MyEventBeacon").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		plan, err := subEnv.Build(esper.Select(
			esper.From[dataflowEventbusSinkBeaconBean](subEnv, "MyEventBeacon"),
			esper.Alias("p0", esper.Field[dataflowEventbusSinkBeaconBean, string]("p0")),
			esper.Alias("p1", esper.Field[dataflowEventbusSinkBeaconBean, int64]("p1")),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		subEngine := esper.NewEngine(subEnv,
			esper.WithRuntimeURI(dataflowEventbusSinkJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = subEngine.Close(context.Background()) }()
		deployment, err := subEngine.Deploy(ctx, plan)
		if err != nil {
			return nil, err
		}
		statement := deployment.Statements()[0]
		batchChannel := make(chan esper.ResultBatch, 8)
		if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			batchChannel <- batch
			return nil
		}); err != nil {
			return nil, err
		}
		instance, err := subEngine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		// The configured beacon pumps on its own goroutine; bounded-wait
		// adaptation for exactly the three contracted deliveries.
		if err := instance.Start(ctx); err != nil {
			return nil, err
		}
		timer := time.NewTimer(5 * time.Second)
		for received := 0; received < 3; {
			select {
			case batch := <-batchChannel:
				received++
				emit("s0", batch)
			case <-timer.C:
				timer.Stop()
				return nil, fmt.Errorf("beacon delivered %d of 3 batches", received)
			}
		}
		timer.Stop()
		select {
		case extra := <-batchChannel:
			return nil, fmt.Errorf("unexpected fourth delivery: %#v", extra.New)
		default:
		}
		return records, nil
	case 2: // eventbus-sink-dynamic-type — collector routing by leading field
		subEnv := esper.NewEnvironment()
		for _, schema := range []struct {
			name   string
			fields []esper.FieldSpec
		}{
			{name: "MyEventOne", fields: []esper.FieldSpec{
				esper.FieldDef("type", reflect.TypeOf("")),
				esper.FieldDef("p0", reflect.TypeOf(int(0))),
				esper.FieldDef("p1", reflect.TypeOf("")),
			}},
			{name: "MyEventTwo", fields: []esper.FieldSpec{
				esper.FieldDef("type", reflect.TypeOf("")),
				esper.FieldDef("f0", reflect.TypeOf("")),
				esper.FieldDef("f1", reflect.TypeOf(int(0))),
			}},
		} {
			if _, err := esper.RegisterObjectArray(subEnv, schema.name, schema.fields); err != nil {
				return nil, err
			}
		}
		definition, buildErr := esper.DefineDataflow(subEnv, "MyDataFlow").
			BeaconSource("source",
				[]any{"type1", 100, "abc"},
				[]any{"type2", "GE", -1},
			).
			EventBusSinkWithCollector("sink", func(_ context.Context, value any) ([]esper.DataflowEventBusSinkEmission, error) {
				row, ok := value.([]any)
				if !ok {
					return nil, fmt.Errorf("collector received %#v, want object-array", value)
				}
				if row[0] == "type1" {
					return []esper.DataflowEventBusSinkEmission{{EventType: "MyEventOne", Value: row}}, nil
				}
				return []esper.DataflowEventBusSinkEmission{{EventType: "MyEventTwo", Value: row}}, nil
			}).
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		subEngine := esper.NewEngine(subEnv,
			esper.WithRuntimeURI(dataflowEventbusSinkJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = subEngine.Close(context.Background()) }()
		var deliveries []dataflowEventbusSinkDelivery
		for _, consumer := range []struct {
			typeName      string
			statementName string
		}{{typeName: "MyEventOne", statementName: "s0"}, {typeName: "MyEventTwo", statementName: "s1"}} {
			plan, err := subEnv.Build(esper.FromAny(subEnv, consumer.typeName).
				Query(esper.StatementName(consumer.statementName)))
			if err != nil {
				return nil, err
			}
			deployment, err := subEngine.Deploy(ctx, plan)
			if err != nil {
				return nil, err
			}
			name := consumer.statementName
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				deliveries = append(deliveries, dataflowEventbusSinkDelivery{statement: name, batch: batch})
				return nil
			}); err != nil {
				return nil, err
			}
		}
		instance, err := subEngine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		// Canned source drains synchronously inside the run; the type1 row
		// reaches s0 before the type2 row reaches s1.
		if err := instance.Run(ctx); err != nil {
			return nil, err
		}
		if len(deliveries) != 2 || deliveries[0].statement != "s0" || deliveries[1].statement != "s1" {
			return nil, fmt.Errorf("dynamic-type deliveries = %#v", deliveries)
		}
		emit("s0", deliveries[0].batch)
		emit("s1", deliveries[1].batch)
	default:
		return nil, fmt.Errorf("unsupported dataflow-eventbus-sink case index %d", caseIndex)
	}
	return records, nil
}

// dataflowEventbusSinkCapture is a no-op sink target used by the invalid
// Build-rejection test (the sink must refuse outgoing edges).
type dataflowEventbusSinkCapture struct{}

func (c *dataflowEventbusSinkCapture) Process(_ context.Context, _ esper.DataflowInput) ([]esper.DataflowEmission, error) {
	return nil, nil
}
