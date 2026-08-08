package esper

import (
	"context"
	"reflect"
	"testing"
)

// nwViewsBean mirrors SupportBean (theString, intBoxed, longBoxed) for the
// InfraNamedWindowViews retention matrix.
type nwViewsBean struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
	LongBoxed int64  `esper:"longBoxed"`
}

// nwViewsMarket mirrors SupportMarketDataBean (symbol) delete triggers.
type nwViewsMarket struct {
	Symbol string `esper:"symbol"`
}

// nwViewsKVLong mirrors the MySimpleKeyValueMap (key, value long) window
// schema used by the longBoxed-based view executions.
type nwViewsKVLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

// nwViewsKVInt mirrors the scene-two window schema projected from
// theString/intBoxed.
type nwViewsKVInt struct {
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

// nwViewsProbe captures the most recent named-window delta or consumer batch
// as property rows, mirroring SupportUpdateListener assertions. count tracks
// invocations since the last reset so assertListenerNotInvoked can be
// mirrored exactly.
type nwViewsProbe struct {
	rowOf   func(Event) []any
	newRows [][]any
	oldRows [][]any
	count   int
}

func (p *nwViewsProbe) reset() {
	p.newRows = nil
	p.oldRows = nil
	p.count = 0
}

func nwViewsKVRow(event Event) []any {
	return []any{event.Get("key").Any(), event.Get("value").Any()}
}

func nwViewsSubscribeWindow(t *testing.T, window *NamedWindow, probe *nwViewsProbe) {
	t.Helper()
	if _, err := window.Subscribe(func(_ context.Context, delta NamedWindowDelta) error {
		probe.count++
		probe.newRows = nil
		for _, event := range delta.New {
			probe.newRows = append(probe.newRows, probe.rowOf(event))
		}
		probe.oldRows = nil
		for _, event := range delta.Old {
			probe.oldRows = append(probe.oldRows, probe.rowOf(event))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func nwViewsSubscribeStatement(t *testing.T, statement *Statement, probe *nwViewsProbe) {
	t.Helper()
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		probe.count++
		probe.newRows = nil
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("consumer result is not an event: %#v", result)
			}
			probe.newRows = append(probe.newRows, probe.rowOf(event))
		}
		probe.oldRows = nil
		for _, result := range batch.Old {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("consumer old result is not an event: %#v", result)
			}
			probe.oldRows = append(probe.oldRows, probe.rowOf(event))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func nwViewsAssertRows(t *testing.T, label string, got, want [][]any) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
}

func nwViewsAssertRowsAnyOrder(t *testing.T, label string, got, want [][]any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %#v, want any-order %#v", label, got, want)
	}
	remaining := append([][]any(nil), want...)
	for _, row := range got {
		matched := -1
		for index, candidate := range remaining {
			if reflect.DeepEqual(row, candidate) {
				matched = index
				break
			}
		}
		if matched < 0 {
			t.Fatalf("%s = %#v, want any-order %#v", label, got, want)
		}
		remaining = append(remaining[:matched], remaining[matched+1:]...)
	}
}

func nwViewsAssertNew(t *testing.T, probe *nwViewsProbe, label string, want ...[]any) {
	t.Helper()
	nwViewsAssertRows(t, label+" new", probe.newRows, want)
	nwViewsAssertRows(t, label+" old", probe.oldRows, nil)
	probe.reset()
}

func nwViewsAssertOld(t *testing.T, probe *nwViewsProbe, label string, want ...[]any) {
	t.Helper()
	nwViewsAssertRows(t, label+" old", probe.oldRows, want)
	nwViewsAssertRows(t, label+" new", probe.newRows, nil)
	probe.reset()
}

func nwViewsAssertIRPair(t *testing.T, probe *nwViewsProbe, label string, wantNew, wantOld []any) {
	t.Helper()
	nwViewsAssertRows(t, label+" new", probe.newRows, [][]any{wantNew})
	nwViewsAssertRows(t, label+" old", probe.oldRows, [][]any{wantOld})
	probe.reset()
}

func nwViewsAssertNotInvoked(t *testing.T, probe *nwViewsProbe, label string) {
	t.Helper()
	if probe.count != 0 {
		t.Fatalf("%s invoked %d times, want silence (new=%#v old=%#v)", label, probe.count, probe.newRows, probe.oldRows)
	}
	probe.reset()
}

func nwViewsSnapshot(t *testing.T, window *NamedWindow) [][]any {
	t.Helper()
	events, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rows := make([][]any, 0, len(events))
	for _, event := range events {
		rows = append(rows, nwViewsKVRow(event))
	}
	return rows
}

// nwViewsHarness wires the shared environment, window, insert trigger,
// optional irstream consumer and on-delete trigger used across the
// InfraNamedWindowViews parity tests.
type nwViewsHarness struct {
	env       *Environment
	engine    *Engine
	window    *NamedWindow
	bean      func(string, int64)
	beanInt   func(string, int)
	market    func(string)
	create    *nwViewsProbe
	consumer  *nwViewsProbe
}

func newNWViewsHarness(t *testing.T, windowName string, retention WindowSpec, valueField string, withConsumer bool) *nwViewsHarness {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[nwViewsBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwViewsMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	var windowSchema Schema
	var err error
	if valueField == "intBoxed" {
		windowSchema, err = RegisterStruct[nwViewsKVInt](env, windowName+"KV")
	} else {
		windowSchema, err = RegisterStruct[nwViewsKVLong](env, windowName+"KV")
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, windowName, windowSchema, NamedWindowRetention(retention)); err != nil {
		t.Fatal(err)
	}

	source := From[nwViewsBean](env, "SupportBean")
	key := Field[nwViewsBean, string]("theString")
	insertAssignments := []TableAssignment{SetColumn("key", key)}
	if valueField == "intBoxed" {
		insertAssignments = append(insertAssignments, SetColumn("value", Field[nwViewsBean, int]("intBoxed")))
	} else {
		insertAssignments = append(insertAssignments, SetColumn("value", Field[nwViewsBean, int64]("longBoxed")))
	}
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow(
		windowName, insertAssignments...,
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[nwViewsMarket](env, "SupportMarketDataBean")).DeleteFromNamedWindow(
		windowName,
		Equal[string](NamedWindowField[string]("key"), Field[nwViewsMarket, string]("symbol")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	var consumerPlan Plan
	if withConsumer {
		consumerPlan, err = env.Build(FromNamedWindow(env, windowName).Query(
			StatementName("s0"),
			WithOldStream(),
		))
		if err != nil {
			t.Fatal(err)
		}
	}

	engine := NewEngine(env)
	deploy := func(plan Plan) *Deployment {
		t.Helper()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		return deployment
	}
	var consumerDeployment *Deployment
	if withConsumer {
		consumerDeployment = deploy(consumerPlan)
	}
	deploy(insertPlan)
	deploy(deletePlan)

	window, ok := engine.NamedWindow(windowName)
	if !ok {
		t.Fatalf("named window %s is missing", windowName)
	}
	h := &nwViewsHarness{
		env:    env,
		engine: engine,
		window: window,
		create: &nwViewsProbe{rowOf: nwViewsKVRow},
	}
	nwViewsSubscribeWindow(t, window, h.create)
	if withConsumer {
		h.consumer = &nwViewsProbe{rowOf: nwViewsKVRow}
		nwViewsSubscribeStatement(t, consumerDeployment.Statements()[0], h.consumer)
	}
	h.bean = func(theString string, value int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBean{TheString: theString, LongBoxed: value}); err != nil {
			t.Fatal(err)
		}
	}
	h.beanInt = func(theString string, value int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBean{TheString: theString, IntBoxed: value}); err != nil {
			t.Fatal(err)
		}
	}
	h.market = func(symbol string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsMarket{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

// TestInfraNWViewsKeepAllSimpleParity mirrors InfraKeepAllSimple: a keep-all
// window holding whole SupportBean events, filled by an insert-into select *
// equivalent, delivers each inserted event to the window listener.
func TestInfraNWViewsKeepAllSimpleParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[nwViewsBean](env, "SupportBean")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	source := From[nwViewsBean](env, "SupportBean")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow(
		"MyWindow",
		SetColumn("theString", Field[nwViewsBean, string]("theString")),
		SetColumn("intBoxed", Field[nwViewsBean, int]("intBoxed")),
		SetColumn("longBoxed", Field[nwViewsBean, int64]("longBoxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("MyWindow")
	if !ok {
		t.Fatal("keep-all window is missing")
	}
	probe := &nwViewsProbe{rowOf: func(event Event) []any { return []any{event.Get("theString").Any()} }}
	nwViewsSubscribeWindow(t, window, probe)

	send := func(theString string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBean{TheString: theString}); err != nil {
			t.Fatal(err)
		}
	}
	send("E1")
	nwViewsAssertNew(t, probe, "create E1", []any{"E1"})
	send("E2")
	nwViewsAssertNew(t, probe, "create E2", []any{"E2"})

	if err := insertDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestInfraNWViewsKeepAllSceneTwoParity mirrors InfraKeepAllSceneTwo: keep-all
// window with an on-delete trigger; window iterator assertions between
// inserts and deletes.
func TestInfraNWViewsKeepAllSceneTwoParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindow", KeepAll(), "intBoxed", false)

	h.beanInt("G1", 10)
	nwViewsAssertNew(t, h.create, "create G1", []any{"G1", 10})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})
	h.beanInt("G2", 20)
	nwViewsAssertNew(t, h.create, "create G2", []any{"G2", 20})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}, {"G2", 20}})
	h.market("G2")
	nwViewsAssertOld(t, h.create, "create delete G2", []any{"G2", 20})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})
	h.beanInt("G3", 30)
	nwViewsAssertNew(t, h.create, "create G3", []any{"G3", 30})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}, {"G3", 30}})
	h.market("G1")
	nwViewsAssertOld(t, h.create, "create delete G1", []any{"G1", 10})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}})
	h.beanInt("G4", 40)
	nwViewsAssertNew(t, h.create, "create G4", []any{"G4", 40})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}, {"G4", 40}})
	h.beanInt("G5", 50)
	nwViewsAssertNew(t, h.create, "create G5", []any{"G5", 50})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}, {"G4", 40}, {"G5", 50}})
	h.beanInt("G6", 60)
	nwViewsAssertNew(t, h.create, "create G6", []any{"G6", 60})
}

// TestInfraNWViewsLengthWindowParity mirrors InfraLengthWindow: a length(3)
// window with irstream consumer and on-delete; expiring the oldest event
// produces IR pairs on both the window and the consumer listener.
func TestInfraNWViewsLengthWindowParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowLW", LengthWindow(3), "longBoxed", true)

	h.bean("E1", 1)
	nwViewsAssertNew(t, h.create, "create E1", []any{"E1", int64(1)})
	nwViewsAssertNew(t, h.consumer, "s0 E1", []any{"E1", int64(1)})

	h.bean("E2", 2)
	nwViewsAssertNew(t, h.create, "create E2", []any{"E2", int64(2)})
	nwViewsAssertNew(t, h.consumer, "s0 E2", []any{"E2", int64(2)})

	h.bean("E3", 3)
	nwViewsAssertNew(t, h.create, "create E3", []any{"E3", int64(3)})
	nwViewsAssertNew(t, h.consumer, "s0 E3", []any{"E3", int64(3)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(2)}, {"E3", int64(3)}})

	h.market("E2")
	nwViewsAssertOld(t, h.create, "create delete E2", []any{"E2", int64(2)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E2", []any{"E2", int64(2)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E3", int64(3)}})

	h.bean("E4", 4)
	nwViewsAssertNew(t, h.create, "create E4", []any{"E4", int64(4)})
	nwViewsAssertNew(t, h.consumer, "s0 E4", []any{"E4", int64(4)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E3", int64(3)}, {"E4", int64(4)}})

	h.bean("E5", 5)
	nwViewsAssertIRPair(t, h.create, "create E5", []any{"E5", int64(5)}, []any{"E1", int64(1)})
	nwViewsAssertIRPair(t, h.consumer, "s0 E5", []any{"E5", int64(5)}, []any{"E1", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E3", int64(3)}, {"E4", int64(4)}, {"E5", int64(5)}})

	h.bean("E6", 6)
	nwViewsAssertIRPair(t, h.create, "create E6", []any{"E6", int64(6)}, []any{"E3", int64(3)})
	nwViewsAssertIRPair(t, h.consumer, "s0 E6", []any{"E6", int64(6)}, []any{"E3", int64(3)})

	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
}

// TestInfraNWViewsLengthWindowSceneTwoParity mirrors InfraLengthWindowSceneTwo:
// length(3) window over an intBoxed projection with iterator assertions and a
// final delete of the most recent event.
func TestInfraNWViewsLengthWindowSceneTwoParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindow", LengthWindow(3), "intBoxed", false)

	h.beanInt("G1", 10)
	nwViewsAssertNew(t, h.create, "create G1", []any{"G1", 10})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})
	h.beanInt("G2", 20)
	nwViewsAssertNew(t, h.create, "create G2", []any{"G2", 20})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}, {"G2", 20}})
	h.market("G2")
	nwViewsAssertOld(t, h.create, "create delete G2", []any{"G2", 20})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})
	h.beanInt("G3", 30)
	nwViewsAssertNew(t, h.create, "create G3", []any{"G3", 30})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}, {"G3", 30}})
	h.market("G1")
	nwViewsAssertOld(t, h.create, "create delete G1", []any{"G1", 10})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}})
	h.beanInt("G4", 40)
	nwViewsAssertNew(t, h.create, "create G4", []any{"G4", 40})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}, {"G4", 40}})
	h.beanInt("G5", 50)
	nwViewsAssertNew(t, h.create, "create G5", []any{"G5", 50})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}, {"G4", 40}, {"G5", 50}})
	h.beanInt("G6", 60)
	nwViewsAssertIRPair(t, h.create, "create G6", []any{"G6", 60}, []any{"G3", 30})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G4", 40}, {"G5", 50}, {"G6", 60}})
	h.market("G6")
	nwViewsAssertOld(t, h.create, "create delete G6", []any{"G6", 60})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G4", 40}, {"G5", 50}})
}

// TestInfraNWViewsLastEventParity mirrors InfraLastEvent: a lastevent window
// replaces the retained event (IR pair) and on-delete empties the window.
func TestInfraNWViewsLastEventParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowLE", LastEvent(), "longBoxed", true)

	h.bean("E1", 1)
	nwViewsAssertNew(t, h.consumer, "s0 E1", []any{"E1", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}})

	h.bean("E2", 2)
	nwViewsAssertIRPair(t, h.consumer, "s0 E2", []any{"E2", int64(2)}, []any{"E1", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E2", int64(2)}})

	h.market("E2")
	nwViewsAssertOld(t, h.consumer, "s0 delete E2", []any{"E2", int64(2)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.bean("E3", 3)
	nwViewsAssertNew(t, h.consumer, "s0 E3", []any{"E3", int64(3)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E3", int64(3)}})

	h.market("E3")
	nwViewsAssertOld(t, h.consumer, "s0 delete E3", []any{"E3", int64(3)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.bean("E4", 4)
	nwViewsAssertNew(t, h.consumer, "s0 E4", []any{"E4", int64(4)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E4", int64(4)}})

	h.market("E1")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
}

// TestInfraNWViewsLastEventSceneTwoParity mirrors InfraLastEventSceneTwo:
// std:lastevent over an intBoxed projection with delete and empty-iterator
// assertions on the window listener.
func TestInfraNWViewsLastEventSceneTwoParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindow", LastEvent(), "intBoxed", false)

	h.beanInt("G1", 1)
	nwViewsAssertNew(t, h.create, "create G1", []any{"G1", 1})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 1}})
	h.beanInt("G2", 2)
	nwViewsAssertIRPair(t, h.create, "create G2", []any{"G2", 2}, []any{"G1", 1})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G2", 2}})
	h.market("G2")
	nwViewsAssertOld(t, h.create, "create delete G2", []any{"G2", 2})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
	h.beanInt("G3", 3)
	nwViewsAssertNew(t, h.create, "create G3", []any{"G3", 3})
}

// TestInfraNWViewsFirstEventParity mirrors InfraFirstEvent: a firstevent
// window keeps the first event until it is deleted; further inserts are
// dropped silently without listener callbacks.
func TestInfraNWViewsFirstEventParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowFE", FirstEvent(), "longBoxed", true)

	h.bean("E1", 1)
	nwViewsAssertNew(t, h.consumer, "s0 E1", []any{"E1", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}})

	h.bean("E2", 2)
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}})

	h.market("E1")
	nwViewsAssertOld(t, h.consumer, "s0 delete E1", []any{"E1", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.bean("E3", 3)
	nwViewsAssertNew(t, h.consumer, "s0 E3", []any{"E3", int64(3)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E3", int64(3)}})

	h.market("E2") // no effect: E2 is not retained
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	h.market("E3")
	nwViewsAssertOld(t, h.consumer, "s0 delete E3", []any{"E3", int64(3)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.bean("E4", 4)
	nwViewsAssertNew(t, h.consumer, "s0 E4", []any{"E4", int64(4)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E4", int64(4)}})

	h.market("E1")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
}

// TestInfraNWViewsUniqueParity mirrors InfraUnique: a unique(key) window
// replaces the retained event for a duplicate key (IR pair), keeps one event
// per key and supports on-delete.
func TestInfraNWViewsUniqueParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowUN", Unique(Field[nwViewsKVLong, string]("key")), "longBoxed", true)

	h.bean("G1", 1)
	nwViewsAssertNew(t, h.consumer, "s0 G1", []any{"G1", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(1)}})

	h.bean("G2", 20)
	nwViewsAssertNew(t, h.consumer, "s0 G2", []any{"G2", int64(20)})
	nwViewsAssertRowsAnyOrder(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(1)}, {"G2", int64(20)}})

	h.market("G2")
	nwViewsAssertOld(t, h.consumer, "s0 delete G2", []any{"G2", int64(20)})

	h.bean("G1", 2)
	nwViewsAssertIRPair(t, h.consumer, "s0 replace G1", []any{"G1", int64(2)}, []any{"G1", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(2)}})

	h.bean("G2", 21)
	nwViewsAssertNew(t, h.consumer, "s0 G2-21", []any{"G2", int64(21)})
	nwViewsAssertRowsAnyOrder(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(2)}, {"G2", int64(21)}})

	h.bean("G2", 22)
	nwViewsAssertIRPair(t, h.consumer, "s0 replace G2", []any{"G2", int64(22)}, []any{"G2", int64(21)})
	nwViewsAssertRowsAnyOrder(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(2)}, {"G2", int64(22)}})

	h.market("G1")
	nwViewsAssertOld(t, h.consumer, "s0 delete G1", []any{"G1", int64(2)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G2", int64(22)}})
}

// TestInfraNWViewsUniqueSceneTwoParity mirrors InfraUniqueSceneTwo: a
// unique(key) window over an intBoxed projection with late milestones and
// any-order iterator assertions.
func TestInfraNWViewsUniqueSceneTwoParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindow", Unique(Field[nwViewsKVInt, string]("key")), "intBoxed", false)

	h.beanInt("G1", 10)
	nwViewsAssertNew(t, h.create, "create G1", []any{"G1", 10})

	nwViewsAssertRowsAnyOrder(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})
	h.beanInt("G2", 20)
	nwViewsAssertNew(t, h.create, "create G2", []any{"G2", 20})
	nwViewsAssertRowsAnyOrder(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}, {"G2", 20}})

	nwViewsAssertRowsAnyOrder(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}, {"G2", 20}})
	h.market("G1")
	nwViewsAssertOld(t, h.create, "create delete G1", []any{"G1", 10})
	nwViewsAssertRowsAnyOrder(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G2", 20}})

	nwViewsAssertRowsAnyOrder(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G2", 20}})
	h.market("G2")
	nwViewsAssertOld(t, h.create, "create delete G2", []any{"G2", 20})
}

// TestInfraNWViewsFirstUniqueParity mirrors InfraFirstUnique: a
// firstunique(key) window keeps the first event per key and silently drops
// later duplicates without listener callbacks.
func TestInfraNWViewsFirstUniqueParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowFU", FirstUnique(Field[nwViewsKVLong, string]("key")), "longBoxed", true)

	h.bean("G1", 1)
	nwViewsAssertNew(t, h.consumer, "s0 G1", []any{"G1", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(1)}})

	h.bean("G2", 20)
	nwViewsAssertNew(t, h.consumer, "s0 G2", []any{"G2", int64(20)})
	nwViewsAssertRowsAnyOrder(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(1)}, {"G2", int64(20)}})

	h.market("G2")
	nwViewsAssertOld(t, h.consumer, "s0 delete G2", []any{"G2", int64(20)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(1)}})

	h.bean("G1", 2) // ignored
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(1)}})

	h.bean("G2", 21)
	nwViewsAssertNew(t, h.consumer, "s0 G2-21", []any{"G2", int64(21)})
	nwViewsAssertRowsAnyOrder(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(1)}, {"G2", int64(21)}})

	h.bean("G2", 22) // ignored
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRowsAnyOrder(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(1)}, {"G2", int64(21)}})

	h.market("G1")
	nwViewsAssertOld(t, h.consumer, "s0 delete G1", []any{"G1", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G2", int64(21)}})
}

// TestInfraNWViewsSortWindowParity mirrors InfraSortWindow: a sort(3, value
// asc) window iterates in sort order, expels the highest event when full (IR
// pair) and orders newer equal-key events before older ones.
func TestInfraNWViewsSortWindowParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowSW", SortWindow(3, Ascending(Field[nwViewsKVLong, int64]("value"))), "longBoxed", true)

	h.bean("E1", 10)
	nwViewsAssertNew(t, h.create, "create E1", []any{"E1", int64(10)})
	nwViewsAssertNew(t, h.consumer, "s0 E1", []any{"E1", int64(10)})

	h.bean("E2", 20)
	nwViewsAssertNew(t, h.create, "create E2", []any{"E2", int64(20)})

	h.bean("E3", 15)
	nwViewsAssertNew(t, h.create, "create E3", []any{"E3", int64(15)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(10)}, {"E3", int64(15)}, {"E2", int64(20)}})

	h.market("E2")
	nwViewsAssertOld(t, h.create, "create delete E2", []any{"E2", int64(20)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(10)}, {"E3", int64(15)}})

	h.bean("E4", 18)
	nwViewsAssertNew(t, h.create, "create E4", []any{"E4", int64(18)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(10)}, {"E3", int64(15)}, {"E4", int64(18)}})

	h.bean("E5", 17)
	nwViewsAssertIRPair(t, h.create, "create E5", []any{"E5", int64(17)}, []any{"E4", int64(18)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(10)}, {"E3", int64(15)}, {"E5", int64(17)}})

	h.market("E1")
	nwViewsAssertOld(t, h.create, "create delete E1", []any{"E1", int64(10)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E3", int64(15)}, {"E5", int64(17)}})

	h.bean("E6", 16)
	nwViewsAssertNew(t, h.create, "create E6", []any{"E6", int64(16)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E3", int64(15)}, {"E6", int64(16)}, {"E5", int64(17)}})

	h.bean("E7", 16)
	nwViewsAssertIRPair(t, h.create, "create E7", []any{"E7", int64(16)}, []any{"E5", int64(17)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E3", int64(15)}, {"E7", int64(16)}, {"E6", int64(16)}})

	h.market("E7")
	nwViewsAssertOld(t, h.create, "create delete E7", []any{"E7", int64(16)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E3", int64(15)}, {"E6", int64(16)}})

	h.bean("E8", 1)
	nwViewsAssertNew(t, h.create, "create E8", []any{"E8", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E8", int64(1)}, {"E3", int64(15)}, {"E6", int64(16)}})

	h.bean("E9", 1)
	nwViewsAssertIRPair(t, h.create, "create E9", []any{"E9", int64(1)}, []any{"E6", int64(16)})
}

// TestInfraNWViewsSortWindowSceneTwoParity mirrors InfraSortWindowSceneTwo:
// ext:sort(3, value) over an intBoxed projection; an event that sorts beyond
// the retained size when the window is full is inserted and removed in the
// same delta (IR pair with itself).
func TestInfraNWViewsSortWindowSceneTwoParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindow", SortWindow(3, Ascending(Field[nwViewsKVInt, int]("value"))), "intBoxed", false)

	h.beanInt("G1", 10)
	nwViewsAssertNew(t, h.create, "create G1", []any{"G1", 10})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})
	h.beanInt("G2", 9)
	nwViewsAssertNew(t, h.create, "create G2", []any{"G2", 9})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G2", 9}, {"G1", 10}})
	h.market("G2")
	nwViewsAssertOld(t, h.create, "create delete G2", []any{"G2", 9})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})
	h.beanInt("G3", 3)
	nwViewsAssertNew(t, h.create, "create G3", []any{"G3", 3})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 3}, {"G1", 10}})
	h.beanInt("G4", 4)
	nwViewsAssertNew(t, h.create, "create G4", []any{"G4", 4})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 3}, {"G4", 4}, {"G1", 10}})
	h.beanInt("G5", 5)
	nwViewsAssertIRPair(t, h.create, "create G5", []any{"G5", 5}, []any{"G1", 10})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 3}, {"G4", 4}, {"G5", 5}})
	h.beanInt("G6", 6)
	nwViewsAssertIRPair(t, h.create, "create G6", []any{"G6", 6}, []any{"G6", 6})
}
