package esper

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDataflowLifecycleAndBuiltinOperators(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[runtimeTestTrade](env, "Trade")
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, runtimeTestTrade{Symbol: "A", Price: 12}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "trade-flow").
		BeaconSource("beacon", event).
		Filter("positive", Greater[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))).
		Select("project", Alias("symbol", Field[runtimeTestTrade, string]("symbol"))).
		Emitter("emit").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if instance.State() != DataflowInstantiated {
		t.Fatalf("initial dataflow state = %v", instance.State())
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if instance.State() != DataflowComplete {
		t.Fatalf("completed dataflow state = %v", instance.State())
	}
	if stats := instance.Stats(); stats.Processed != 1 || stats.Emitted != 1 {
		t.Fatalf("dataflow stats = %#v", stats)
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 {
		t.Fatalf("dataflow outputs = %#v", outputs)
	}
	row, ok := outputs[0].(Row)
	if !ok || row.Get("symbol").Any() != "A" {
		t.Fatalf("dataflow output = %#v", outputs[0])
	}
	if _, ok := env.Dataflow("trade-flow"); !ok {
		t.Fatal("saved dataflow definition is missing")
	}
}

func TestEventBusDataflowRunsUntilCanceled(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "event-bus-flow").
		EventBusSource("source", "Trade").
		Filter("positive", Greater[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))).
		Emitter("emit").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if instance.State() != DataflowRunning {
		t.Fatalf("event-bus dataflow state = %v", instance.State())
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "low", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "high", Price: 11}); err != nil {
		t.Fatal(err)
	}
	if stats := instance.Stats(); stats.Processed != 2 || stats.Emitted != 1 {
		t.Fatalf("event-bus stats = %#v", stats)
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 || outputs[0].(Event).Underlying().(runtimeTestTrade).Symbol != "high" {
		t.Fatalf("event-bus outputs = %#v", outputs)
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "after", Price: 12}); err != nil {
		t.Fatal(err)
	}
	if instance.Stats().Processed != 2 {
		t.Fatalf("canceled dataflow processed an event: %#v", instance.Stats())
	}
}

func TestEventBusDataflowSinkRoutesToRegisteredEventType(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[runtimeTestTrade](env, "TradeCopy"); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "event-bus-sink").
		EventBusSource("source", "Trade").
		EventBusSink("sink", "TradeCopy").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "TradeCopy").Query(StatementName("copy-observer")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var observed int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		observed += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "routed", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if observed != 1 {
		t.Fatalf("event-bus sink observations = %d", observed)
	}
}

func TestEPStatementSourceFeedsDataflowAndUnsubscribesOnCancel(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("statement-source")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "statement-flow").
		EPStatementSource("source", deployment.Statements()[0]).
		Emitter("emit").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if instance.State() != DataflowRunning {
		t.Fatalf("statement-source dataflow state = %v", instance.State())
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "before"}); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 || outputs[0].(Event).Underlying().(runtimeTestTrade).Symbol != "before" {
		t.Fatalf("statement-source outputs = %#v", outputs)
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "after"}); err != nil {
		t.Fatal(err)
	}
	if len(instance.Outputs()) != 1 {
		t.Fatalf("canceled statement-source dataflow received output: %#v", instance.Outputs())
	}
}

func TestDataflowGraphBranchesAndNamedPorts(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[runtimeTestTrade](env, "Trade")
	if err != nil {
		t.Fatal(err)
	}
	high, err := newEvent(schema, runtimeTestTrade{Symbol: "high", Price: 12}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	low, err := newEvent(schema, runtimeTestTrade{Symbol: "low", Price: 3}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "trade-branches").
		BeaconSource("source", high, low).
		Filter("high", Greater[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))).
		Filter("low", LessOrEqual[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))).
		Emitter("high-emit").
		Emitter("low-emit").
		Connect("source", "high").
		ConnectPorts("source", "out", "low", "in").
		Connect("high", "high-emit").
		Connect("low", "low-emit").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.Edges()) != 4 {
		t.Fatalf("dataflow edges = %#v", definition.Edges())
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if instance.State() != DataflowComplete {
		t.Fatalf("branch dataflow state = %v", instance.State())
	}
	if stats := instance.Stats(); stats.Processed != 2 || stats.Emitted != 2 {
		t.Fatalf("branch dataflow stats = %#v", stats)
	}
	outputs := instance.Outputs()
	if len(outputs) != 2 {
		t.Fatalf("branch dataflow outputs = %#v", outputs)
	}
	if outputs[0].(Event).Underlying().(runtimeTestTrade).Symbol != "high" || outputs[1].(Event).Underlying().(runtimeTestTrade).Symbol != "low" {
		t.Fatalf("branch dataflow output order = %#v", outputs)
	}
}

func TestDataflowGraphValidation(t *testing.T) {
	env := NewEnvironment()
	if _, err := DefineDataflow(env, "unknown-edge").
		BeaconSource("source").
		Emitter("sink").
		Connect("source", "missing").
		Build(); err == nil {
		t.Fatal("expected unknown dataflow edge operator to fail")
	}
	if _, err := DefineDataflow(env, "cycle").
		BeaconSource("source").
		Filter("middle", Literal(true)).
		Connect("source", "middle").
		Connect("middle", "source").
		Build(); err == nil {
		t.Fatal("expected cyclic dataflow graph to fail")
	}
	if _, err := DefineDataflow(env, "duplicate-edge").
		BeaconSource("source").
		Emitter("sink").
		Connect("source", "sink").
		Connect("source", "sink").
		Build(); err == nil {
		t.Fatal("expected duplicate dataflow edge to fail")
	}
	if _, err := DefineDataflow(env, "unknown-signal-handler").
		BeaconSource("source").
		OnSignal("missing", func(context.Context, DataflowSignal) error { return nil }).
		Build(); err == nil {
		t.Fatal("expected unknown dataflow signal handler operator to fail")
	}
	if _, err := DefineDataflow(env, "nil-custom-factory").
		BeaconSource("source").
		Custom("custom", nil).
		Build(); err == nil {
		t.Fatal("expected nil custom dataflow factory to fail")
	}
	if _, err := DefineDataflow(env, "unknown-port").
		BeaconSource("source").
		CustomPorts("custom", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return &dataflowTestRuntime{}, nil
		}, []string{"in"}, []string{"out"}).
		Emitter("sink").
		ConnectPorts("custom", "missing", "sink", "in").
		Build(); err == nil {
		t.Fatal("expected unknown dataflow output port to fail")
	}
}

func TestDataflowSignalsPropagateAndCanBeSubmitted(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[runtimeTestTrade](env, "Trade")
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, runtimeTestTrade{Symbol: "A", Price: 12}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	definition, err := DefineDataflow(env, "signal-flow").
		BeaconSource("source", event, FinalMarker{}).
		OnSignal("emit", func(_ context.Context, signal DataflowSignal) error {
			seen = append(seen, signal.SignalType())
			return nil
		}).
		Emitter("emit").
		Connect("source", "emit").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, want := seen, []string{"final"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("beacon signal handlers = %#v, want %#v", got, want)
	}
	if signals := instance.Signals(); len(signals) != 1 || signals[0].SignalType() != "final" {
		t.Fatalf("beacon signals = %#v", signals)
	}

	definition, err = DefineDataflow(env, "running-signal-flow").
		EventBusSource("source", "Trade").
		OnSignal("emit", func(_ context.Context, signal DataflowSignal) error {
			seen = append(seen, signal.SignalType())
			return nil
		}).
		Emitter("emit").
		Connect("source", "emit").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	running, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := running.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := running.SubmitSignal(context.Background(), CustomSignal{Type: "window", Payload: "tick-1"}); err != nil {
		t.Fatal(err)
	}
	if signals := running.Signals(); len(signals) != 1 || signals[0].SignalType() != "window" {
		t.Fatalf("submitted signals = %#v", signals)
	}
	if err := running.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type dataflowTestRuntime struct {
	lifecycle *[]string
}

func (r *dataflowTestRuntime) Open(context.Context) error {
	*r.lifecycle = append(*r.lifecycle, "open")
	return nil
}

func (r *dataflowTestRuntime) Close(context.Context) error {
	*r.lifecycle = append(*r.lifecycle, "close")
	return nil
}

func (r *dataflowTestRuntime) Process(_ context.Context, input DataflowInput) ([]DataflowEmission, error) {
	*r.lifecycle = append(*r.lifecycle, "process")
	return []DataflowEmission{Emit(input.Value)}, nil
}

func (r *dataflowTestRuntime) OnSignal(_ context.Context, signal DataflowSignal) ([]DataflowEmission, error) {
	*r.lifecycle = append(*r.lifecycle, "signal:"+signal.SignalType())
	return []DataflowEmission{Emit(signal)}, nil
}

type dataflowPortRuntime struct {
	ports *[]string
}

func (r *dataflowPortRuntime) Process(_ context.Context, input DataflowInput) ([]DataflowEmission, error) {
	*r.ports = append(*r.ports, input.Port)
	return []DataflowEmission{EmitPort("left", input.Value), EmitPort("right", input.Value)}, nil
}

func TestDataflowCustomOperatorNamedPortsRouteEmissions(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[runtimeTestTrade](env, "Trade")
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, runtimeTestTrade{Symbol: "ports", Price: 1}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	var ports []string
	definition, err := DefineDataflow(env, "port-flow").
		BeaconSource("source", event).
		CustomPorts("split", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return &dataflowPortRuntime{ports: &ports}, nil
		}, []string{"in"}, []string{"left", "right"}).
		Emitter("left-sink").
		Emitter("right-sink").
		Connect("source", "split").
		ConnectPorts("split", "left", "left-sink", "in").
		ConnectPorts("split", "right", "right-sink", "in").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 || ports[0] != "in" {
		t.Fatalf("custom input ports = %#v", ports)
	}
	if len(instance.Outputs()) != 2 || instance.Stats().Emitted != 2 {
		t.Fatalf("named-port outputs/stats = %#v/%#v", instance.Outputs(), instance.Stats())
	}
}

func TestDataflowCustomOperatorFactoryAndLifecycle(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[runtimeTestTrade](env, "Trade")
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, runtimeTestTrade{Symbol: "custom", Price: 1}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	var lifecycle []string
	var factoryContext DataflowOperatorContext
	factoryCalls := 0
	definition, err := DefineDataflow(env, "custom-flow").
		BeaconSource("source", event, WindowMarker{}).
		Custom("custom", func(ctx DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			factoryCalls++
			factoryContext = ctx
			return &dataflowTestRuntime{lifecycle: &lifecycle}, nil
		}).
		Emitter("emit").
		Connect("source", "custom").
		Connect("custom", "emit").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if factoryCalls != 1 || factoryContext.DataflowName != "custom-flow" || factoryContext.OperatorName != "custom" {
		t.Fatalf("custom factory context = %#v, calls=%d", factoryContext, factoryCalls)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, want := lifecycle, []string{"open", "process", "signal:window", "close"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Fatalf("custom lifecycle = %#v, want %#v", got, want)
	}
	if len(instance.Outputs()) != 1 || len(instance.Signals()) != 1 {
		t.Fatalf("custom outputs/signals = %#v/%#v", instance.Outputs(), instance.Signals())
	}
}

type dataflowErrorRuntime struct{ err error }

func (r dataflowErrorRuntime) Process(context.Context, DataflowInput) ([]DataflowEmission, error) {
	return nil, r.err
}

func TestDataflowOptionsExceptionsAndSavedConfiguration(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[runtimeTestTrade](env, "Trade")
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, runtimeTestTrade{Symbol: "failure", Price: 1}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("operator failure")
	definition, err := DefineDataflow(env, "error-flow").
		BeaconSource("source", event).
		Custom("failing", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return dataflowErrorRuntime{err: sentinel}, nil
		}).
		Emitter("sink").
		Connect("source", "failing").
		Connect("failing", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := env.SaveDataflowConfiguration("error-flow"); err != nil {
		t.Fatal(err)
	}
	if names := env.SavedDataflowConfigurations(); len(names) != 1 || names[0] != "error-flow" {
		t.Fatalf("saved dataflows = %#v", names)
	}
	loaded, ok := env.LoadDataflowConfiguration("error-flow")
	if !ok || loaded.Name() != definition.Name() || len(loaded.Operators()) != len(definition.Operators()) {
		t.Fatalf("loaded dataflow = %#v, ok=%v", loaded, ok)
	}

	var observed DataflowError
	engine := NewEngine(env)
	instance, err := engine.InstantiateSavedDataflowWithOptions(context.Background(), "error-flow", DataflowOptions{
		InstanceID:  "error-instance-1",
		UserObject:  "owner",
		ErrorPolicy: DataflowErrorContinue,
		ExceptionHandler: func(_ context.Context, failure DataflowError) error {
			observed = failure
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if instance.InstanceID() != "error-instance-1" || instance.UserObject() != "owner" {
		t.Fatalf("dataflow instance options = %q/%#v", instance.InstanceID(), instance.UserObject())
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if instance.State() != DataflowComplete {
		t.Fatalf("continued dataflow state = %v", instance.State())
	}
	if stats := instance.Stats(); stats.Processed != 1 || stats.Errors != 1 || stats.Dropped != 1 || stats.Emitted != 0 {
		t.Fatalf("continued dataflow stats = %#v", stats)
	}
	if observed.OperatorName != "failing" || !errors.Is(observed, sentinel) {
		t.Fatalf("dataflow exception = %#v", observed)
	}
	if last, ok := instance.LastError(); !ok || !errors.Is(last, sentinel) {
		t.Fatalf("last dataflow error = %#v/%v", last, ok)
	}
	if err := env.DeleteDataflowConfiguration("error-flow"); err != nil {
		t.Fatal(err)
	}
	if _, ok := env.LoadDataflowConfiguration("error-flow"); ok {
		t.Fatal("deleted dataflow configuration is still available")
	}
}
