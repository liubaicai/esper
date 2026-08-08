package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
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

// nwViewsBeanA mirrors SupportBean_A (id) delete triggers.
type nwViewsBeanA struct {
	ID string `esper:"id"`
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
	advance   func(millis int64)
	create    *nwViewsProbe
	consumer  *nwViewsProbe
}

func newNWViewsHarness(t *testing.T, windowName string, retention WindowSpec, valueField string, withConsumer bool, engineOptions ...EngineOption) *nwViewsHarness {
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

	engine := NewEngine(env, engineOptions...)
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
	h.advance = func(millis int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(millis).UTC()); err != nil {
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

// TestInfraNWViewsTimeWindowParity mirrors InfraTimeWindow: a time(10 sec)
// window expires events exactly at insert-time plus the period, firing old
// data on both the window and the irstream consumer listener, while
// on-delete removes retained events early.
func TestInfraNWViewsTimeWindowParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowTW", TimeWindow(10*time.Second), "longBoxed", true, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(1000)
	h.bean("E1", 1)
	nwViewsAssertNew(t, h.create, "create E1", []any{"E1", int64(1)})
	nwViewsAssertNew(t, h.consumer, "s0 E1", []any{"E1", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}})

	h.advance(5000)
	h.bean("E2", 2)
	nwViewsAssertNew(t, h.create, "create E2", []any{"E2", int64(2)})
	nwViewsAssertNew(t, h.consumer, "s0 E2", []any{"E2", int64(2)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(2)}})

	h.advance(10000)
	h.bean("E3", 3)
	nwViewsAssertNew(t, h.create, "create E3", []any{"E3", int64(3)})
	nwViewsAssertNew(t, h.consumer, "s0 E3", []any{"E3", int64(3)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(2)}, {"E3", int64(3)}})

	h.advance(10999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	h.advance(11000)
	nwViewsAssertOld(t, h.create, "create expire E1", []any{"E1", int64(1)})
	nwViewsAssertOld(t, h.consumer, "s0 expire E1", []any{"E1", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E2", int64(2)}, {"E3", int64(3)}})

	h.bean("E4", 4)
	nwViewsAssertNew(t, h.create, "create E4", []any{"E4", int64(4)})
	nwViewsAssertNew(t, h.consumer, "s0 E4", []any{"E4", int64(4)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E2", int64(2)}, {"E3", int64(3)}, {"E4", int64(4)}})

	h.market("E2")
	nwViewsAssertOld(t, h.create, "create delete E2", []any{"E2", int64(2)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E2", []any{"E2", int64(2)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E3", int64(3)}, {"E4", int64(4)}})

	h.advance(15000)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")

	h.advance(19999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	h.advance(20000)
	nwViewsAssertOld(t, h.create, "create expire E3", []any{"E3", int64(3)})
	nwViewsAssertOld(t, h.consumer, "s0 expire E3", []any{"E3", int64(3)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E4", int64(4)}})

	h.market("E4")
	nwViewsAssertOld(t, h.create, "create delete E4", []any{"E4", int64(4)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E4", []any{"E4", int64(4)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.advance(100000)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
}

// TestInfraNWViewsTimeWindowSceneTwoParity mirrors InfraTimeWindowSceneTwo:
// #time(10 sec) over an intBoxed projection with module-style deploy
// milestones, expiry and delete sequences.
func TestInfraNWViewsTimeWindowSceneTwoParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindow", TimeWindow(10*time.Second), "intBoxed", false, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(0)
	h.beanInt("G1", 10)
	nwViewsAssertNew(t, h.create, "create G1", []any{"G1", 10})

	h.advance(5000)
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})
	h.beanInt("G2", 20)
	nwViewsAssertNew(t, h.create, "create G2", []any{"G2", 20})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}, {"G2", 20}})
	h.market("G2")
	nwViewsAssertOld(t, h.create, "create delete G2", []any{"G2", 20})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})
	h.advance(10000)
	nwViewsAssertOld(t, h.create, "create expire G1", []any{"G1", 10})

	h.advance(25000)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.advance(25000)
	h.beanInt("G3", 30)
	h.advance(26000)
	h.beanInt("G4", 40)
	h.advance(27000)
	h.beanInt("G5", 50)
	h.create.reset()

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}, {"G4", 40}, {"G5", 50}})
	h.market("G3")
	nwViewsAssertOld(t, h.create, "create delete G3", []any{"G3", 30})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G4", 40}, {"G5", 50}})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G4", 40}, {"G5", 50}})
	h.advance(35999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	h.advance(36000)
	nwViewsAssertOld(t, h.create, "create expire G4", []any{"G4", 40})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G5", 50}})
	h.market("G5")
	nwViewsAssertOld(t, h.create, "create delete G5", []any{"G5", 50})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
}

// TestInfraNWViewsLengthBatchParity mirrors InfraLengthBatch: a
// length_batch(3) window accumulates silently and completes each batch as
// one new-data delivery; the previously completed batch leaves as old data
// in the same callback, and deletes of unflushed events are silent.
func TestInfraNWViewsLengthBatchParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowLB", LengthBatch(3), "longBoxed", true)

	h.bean("E1", 1)
	h.bean("E2", 2)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(2)}})

	h.market("E2")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}})

	h.bean("E3", 3)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E3", int64(3)}})

	h.bean("E4", 4)
	nwViewsAssertNew(t, h.create, "create flush", []any{"E1", int64(1)}, []any{"E3", int64(3)}, []any{"E4", int64(4)})
	nwViewsAssertNew(t, h.consumer, "s0 flush", []any{"E1", int64(1)}, []any{"E3", int64(3)}, []any{"E4", int64(4)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.bean("E5", 5)
	h.bean("E6", 6)
	h.market("E5")
	h.market("E6")
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.bean("E7", 7)
	h.bean("E8", 8)
	h.bean("E9", 9)
	if len(h.create.newRows) != 3 || len(h.create.oldRows) != 3 {
		t.Fatalf("create flush = new %#v old %#v, want new 3 old 3", h.create.newRows, h.create.oldRows)
	}
	nwViewsAssertRows(t, "create flush new", h.create.newRows, [][]any{{"E7", int64(7)}, {"E8", int64(8)}, {"E9", int64(9)}})
	nwViewsAssertRows(t, "create flush old", h.create.oldRows, [][]any{{"E1", int64(1)}, {"E3", int64(3)}, {"E4", int64(4)}})
	h.create.reset()
	h.consumer.reset()

	h.bean("E10", 10)
	h.bean("E10", 11)
	h.market("E10")

	h.bean("E21", 21)
	h.bean("E22", 22)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	h.bean("E23", 23)
	if len(h.create.newRows) != 3 || len(h.create.oldRows) != 3 {
		t.Fatalf("create flush = new %#v old %#v, want new 3 old 3", h.create.newRows, h.create.oldRows)
	}
	nwViewsAssertRows(t, "create flush new", h.create.newRows, [][]any{{"E21", int64(21)}, {"E22", int64(22)}, {"E23", int64(23)}})
	nwViewsAssertRows(t, "create flush old", h.create.oldRows, [][]any{{"E7", int64(7)}, {"E8", int64(8)}, {"E9", int64(9)}})
	h.create.reset()
}

// TestInfraNWViewsLengthBatchSceneTwoParity mirrors InfraLengthBatchSceneTwo:
// win:length_batch(3) over an intBoxed projection with deletes between
// accumulating events and a final batch flush.
func TestInfraNWViewsLengthBatchSceneTwoParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindow", LengthBatch(3), "intBoxed", false)

	h.beanInt("G1", 10)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.beanInt("G2", 20)
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}, {"G2", 20}})
	h.market("G2")
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 10}})
	h.market("G1")
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
	h.beanInt("G3", 30)
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}})
	h.beanInt("G4", 40)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}, {"G4", 40}})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}, {"G4", 40}})
	h.market("G4")
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}})

	h.beanInt("G5", 50)
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 30}, {"G5", 50}})
	h.beanInt("G6", 60)
	nwViewsAssertNew(t, h.create, "create flush", []any{"G3", 30}, []any{"G5", 50}, []any{"G6", 60})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
}

// TestInfraNWViewsLengthFirstWindowParity mirrors InfraLengthFirstWindow: a
// firstlength(2) window admits only the first two events, drops later inserts
// silently and admits again after deletes free a slot.
func TestInfraNWViewsLengthFirstWindowParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowLFW", FirstLength(2), "longBoxed", true)

	h.bean("E1", 1)
	nwViewsAssertNew(t, h.create, "create E1", []any{"E1", int64(1)})
	nwViewsAssertNew(t, h.consumer, "s0 E1", []any{"E1", int64(1)})

	h.bean("E2", 2)
	nwViewsAssertNew(t, h.create, "create E2", []any{"E2", int64(2)})
	nwViewsAssertNew(t, h.consumer, "s0 E2", []any{"E2", int64(2)})

	h.bean("E3", 3)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(2)}})

	h.market("E2")
	nwViewsAssertOld(t, h.create, "create delete E2", []any{"E2", int64(2)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E2", []any{"E2", int64(2)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}})

	h.bean("E4", 4)
	nwViewsAssertNew(t, h.create, "create E4", []any{"E4", int64(4)})
	nwViewsAssertNew(t, h.consumer, "s0 E4", []any{"E4", int64(4)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E4", int64(4)}})

	h.bean("E5", 5)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E4", int64(4)}})
}

// TestInfraNWViewsLengthWindowSceneThreeParity mirrors
// InfraLengthWindowSceneThree: a length(2) window holding whole SupportBean
// events with an irstream wildcard consumer and an on-delete trigger keyed
// by SupportBean_A.id against theString.
func TestInfraNWViewsLengthWindowSceneThreeParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[nwViewsBean](env, "SupportBean")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwViewsBeanA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "ABCWin", schema, NamedWindowRetention(LengthWindow(2))); err != nil {
		t.Fatal(err)
	}
	source := From[nwViewsBean](env, "SupportBean")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow(
		"ABCWin",
		SetColumn("theString", Field[nwViewsBean, string]("theString")),
		SetColumn("intBoxed", Field[nwViewsBean, int]("intBoxed")),
		SetColumn("longBoxed", Field[nwViewsBean, int64]("longBoxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[nwViewsBeanA](env, "SupportBean_A")).DeleteFromNamedWindow(
		"ABCWin",
		Equal[string](NamedWindowField[string]("theString"), Field[nwViewsBeanA, string]("id")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "ABCWin").Query(
		StatementName("s0"),
		WithOldStream(),
	))
	if err != nil {
		t.Fatal(err)
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
	consumerDeployment := deploy(consumerPlan)
	deploy(insertPlan)
	deploy(deletePlan)

	probe := &nwViewsProbe{rowOf: func(event Event) []any { return []any{event.Get("theString").Any()} }}
	nwViewsSubscribeStatement(t, consumerDeployment.Statements()[0], probe)

	window, ok := engine.NamedWindow("ABCWin")
	if !ok {
		t.Fatal("ABCWin is missing")
	}
	snapshot := func() [][]any {
		t.Helper()
		events, err := window.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		rows := make([][]any, 0, len(events))
		for _, event := range events {
			rows = append(rows, []any{event.Get("theString").Any()})
		}
		return rows
	}
	send := func(theString string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBean{TheString: theString}); err != nil {
			t.Fatal(err)
		}
	}
	sendA := func(id string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBeanA{ID: id}); err != nil {
			t.Fatal(err)
		}
	}

	nwViewsAssertRows(t, "s0 iterator", snapshot(), nil)

	send("E1")
	nwViewsAssertNew(t, probe, "s0 E1", []any{"E1"})

	sendA("E1")
	nwViewsAssertOld(t, probe, "s0 delete E1", []any{"E1"})

	nwViewsAssertRows(t, "s0 iterator", snapshot(), nil)
	send("E2")
	nwViewsAssertNew(t, probe, "s0 E2", []any{"E2"})
	send("E3")
	nwViewsAssertNew(t, probe, "s0 E3", []any{"E3"})

	nwViewsAssertRows(t, "s0 iterator", snapshot(), [][]any{{"E2"}, {"E3"}})
	sendA("E3")
	nwViewsAssertOld(t, probe, "s0 delete E3", []any{"E3"})

	nwViewsAssertRows(t, "s0 iterator", snapshot(), [][]any{{"E2"}})
	send("E4")
	nwViewsAssertNew(t, probe, "s0 E4", []any{"E4"})
	send("E5")
	nwViewsAssertIRPair(t, probe, "s0 E5", []any{"E5"}, []any{"E2"})
}

// TestInfraNWViewsTimeAccumParity mirrors InfraTimeAccum: a time_accum(10
// sec) window delivers inserts immediately and expires all retained events
// together at the newest retained event plus the period; deleting the newest
// event re-anchors the shared expiry.
func TestInfraNWViewsTimeAccumParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowTA", TimeAccum(10*time.Second), "longBoxed", true, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(1000)
	h.bean("E1", 1)
	nwViewsAssertNew(t, h.create, "create E1", []any{"E1", int64(1)})
	nwViewsAssertNew(t, h.consumer, "s0 E1", []any{"E1", int64(1)})

	h.advance(5000)
	h.bean("E2", 2)
	nwViewsAssertNew(t, h.create, "create E2", []any{"E2", int64(2)})
	nwViewsAssertNew(t, h.consumer, "s0 E2", []any{"E2", int64(2)})

	h.advance(10000)
	h.bean("E3", 3)
	nwViewsAssertNew(t, h.create, "create E3", []any{"E3", int64(3)})
	nwViewsAssertNew(t, h.consumer, "s0 E3", []any{"E3", int64(3)})

	h.advance(15000)
	h.bean("E4", 4)
	nwViewsAssertNew(t, h.create, "create E4", []any{"E4", int64(4)})
	nwViewsAssertNew(t, h.consumer, "s0 E4", []any{"E4", int64(4)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(2)}, {"E3", int64(3)}, {"E4", int64(4)}})

	h.market("E2")
	nwViewsAssertOld(t, h.create, "create delete E2", []any{"E2", int64(2)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E2", []any{"E2", int64(2)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E3", int64(3)}, {"E4", int64(4)}})

	h.advance(24999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")

	h.advance(25000)
	nwViewsAssertOld(t, h.create, "create expire all", []any{"E1", int64(1)}, []any{"E3", int64(3)}, []any{"E4", int64(4)})
	nwViewsAssertOld(t, h.consumer, "s0 expire all", []any{"E1", int64(1)}, []any{"E3", int64(3)}, []any{"E4", int64(4)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.market("E4")
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")

	h.advance(30000)
	h.bean("E5", 5)
	nwViewsAssertNew(t, h.create, "create E5", []any{"E5", int64(5)})
	nwViewsAssertNew(t, h.consumer, "s0 E5", []any{"E5", int64(5)})

	h.advance(31000)
	h.bean("E6", 6)
	nwViewsAssertNew(t, h.create, "create E6", []any{"E6", int64(6)})
	nwViewsAssertNew(t, h.consumer, "s0 E6", []any{"E6", int64(6)})

	h.advance(38000)
	h.bean("E7", 7)
	nwViewsAssertNew(t, h.create, "create E7", []any{"E7", int64(7)})
	nwViewsAssertNew(t, h.consumer, "s0 E7", []any{"E7", int64(7)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E5", int64(5)}, {"E6", int64(6)}, {"E7", int64(7)}})

	h.market("E7")
	nwViewsAssertOld(t, h.create, "create delete E7", []any{"E7", int64(7)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E7", []any{"E7", int64(7)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E5", int64(5)}, {"E6", int64(6)}})

	h.advance(40999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")

	h.advance(41000)
	nwViewsAssertOld(t, h.create, "create expire rest", []any{"E5", int64(5)}, []any{"E6", int64(6)})
	nwViewsAssertOld(t, h.consumer, "s0 expire rest", []any{"E5", int64(5)}, []any{"E6", int64(6)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.advance(50000)
	h.bean("E8", 8)
	nwViewsAssertNew(t, h.create, "create E8", []any{"E8", int64(8)})
	nwViewsAssertNew(t, h.consumer, "s0 E8", []any{"E8", int64(8)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E8", int64(8)}})

	h.advance(55000)
	h.market("E8")
	nwViewsAssertOld(t, h.create, "create delete E8", []any{"E8", int64(8)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E8", []any{"E8", int64(8)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.advance(100000)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
}

// TestInfraNWViewsTimeAccumSceneTwoParity mirrors InfraTimeAccumSceneTwo:
// win:time_accum(10 sec) over an intBoxed projection with deletes re-anchoring
// the shared expiry to the newest retained event.
func TestInfraNWViewsTimeAccumSceneTwoParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindow", TimeAccum(10*time.Second), "intBoxed", false, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(1000)
	h.beanInt("G1", 1)
	nwViewsAssertNew(t, h.create, "create G1", []any{"G1", 1})

	h.advance(5000)
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 1}})
	h.beanInt("G2", 2)
	nwViewsAssertNew(t, h.create, "create G2", []any{"G2", 2})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 1}, {"G2", 2}})
	h.market("G2")
	nwViewsAssertOld(t, h.create, "create delete G2", []any{"G2", 2})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 1}})

	h.advance(10999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 1}})
	h.advance(11000)
	nwViewsAssertOld(t, h.create, "create expire G1", []any{"G1", 1})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
	h.advance(20000)
	h.beanInt("G3", 3)
	nwViewsAssertNew(t, h.create, "create G3", []any{"G3", 3})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 3}})
	h.advance(29999)
	h.beanInt("G4", 4)
	nwViewsAssertNew(t, h.create, "create G4", []any{"G4", 4})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 3}, {"G4", 4}})
	h.market("G3")
	nwViewsAssertOld(t, h.create, "create delete G3", []any{"G3", 3})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G4", 4}})
	h.market("G4")
	nwViewsAssertOld(t, h.create, "create delete G4", []any{"G4", 4})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
	h.advance(40000)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.advance(41000)
	h.beanInt("G5", 5)
	nwViewsAssertNew(t, h.create, "create G5", []any{"G5", 5})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G5", 5}})
	h.advance(42000)
	h.beanInt("G6", 6)
	nwViewsAssertNew(t, h.create, "create G6", []any{"G6", 6})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G5", 5}, {"G6", 6}})
	h.advance(43000)
	h.beanInt("G7", 7)
	nwViewsAssertNew(t, h.create, "create G7", []any{"G7", 7})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G5", 5}, {"G6", 6}, {"G7", 7}})
	h.advance(44000)
	h.beanInt("G8", 8)
	nwViewsAssertNew(t, h.create, "create G8", []any{"G8", 8})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G5", 5}, {"G6", 6}, {"G7", 7}, {"G8", 8}})
	h.market("G6")
	nwViewsAssertOld(t, h.create, "create delete G6", []any{"G6", 6})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G5", 5}, {"G7", 7}, {"G8", 8}})
	h.market("G8")
	nwViewsAssertOld(t, h.create, "create delete G8", []any{"G8", 8})

	h.advance(52999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	h.advance(53000)
	nwViewsAssertOld(t, h.create, "create expire rest", []any{"G5", 5}, []any{"G7", 7})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
}

// TestInfraNWViewsExtTimeWindowParity mirrors InfraExtTimeWindow: an
// ext_timed(value, 10 sec) window slides with arriving event timestamps and
// expires events whose timestamp plus the period is reached by the newest
// timestamp seen, in the same delta as the triggering insert.
func TestInfraNWViewsExtTimeWindowParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowETW", ExternallyTimed(Field[nwViewsKVLong, int64]("value"), 10*time.Second), "longBoxed", true, WithStartTime(time.Unix(0, 0).UTC()))

	h.bean("E1", 1000)
	nwViewsAssertNew(t, h.create, "create E1", []any{"E1", int64(1000)})
	nwViewsAssertNew(t, h.consumer, "s0 E1", []any{"E1", int64(1000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1000)}})

	h.bean("E2", 5000)
	nwViewsAssertNew(t, h.create, "create E2", []any{"E2", int64(5000)})
	nwViewsAssertNew(t, h.consumer, "s0 E2", []any{"E2", int64(5000)})

	h.bean("E3", 10000)
	nwViewsAssertNew(t, h.create, "create E3", []any{"E3", int64(10000)})
	nwViewsAssertNew(t, h.consumer, "s0 E3", []any{"E3", int64(10000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1000)}, {"E2", int64(5000)}, {"E3", int64(10000)}})

	h.bean("E4", 11000)
	nwViewsAssertIRPair(t, h.create, "create E4", []any{"E4", int64(11000)}, []any{"E1", int64(1000)})
	nwViewsAssertIRPair(t, h.consumer, "s0 E4", []any{"E4", int64(11000)}, []any{"E1", int64(1000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E2", int64(5000)}, {"E3", int64(10000)}, {"E4", int64(11000)}})

	h.market("E2")
	nwViewsAssertOld(t, h.create, "create delete E2", []any{"E2", int64(5000)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E2", []any{"E2", int64(5000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E3", int64(10000)}, {"E4", int64(11000)}})

	h.bean("E5", 15000)
	nwViewsAssertNew(t, h.create, "create E5", []any{"E5", int64(15000)})
	nwViewsAssertNew(t, h.consumer, "s0 E5", []any{"E5", int64(15000)})
}

// TestInfraNWViewsExtTimeWindowSceneTwoParity mirrors
// InfraExtTimeWindowSceneTwo: win:ext_timed(value, 10 sec) with milestones
// and on-delete between sliding expiries.
func TestInfraNWViewsExtTimeWindowSceneTwoParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindow", ExternallyTimed(Field[nwViewsKVLong, int64]("value"), 10*time.Second), "longBoxed", false, WithStartTime(time.Unix(0, 0).UTC()))

	h.bean("G1", 0)
	nwViewsAssertNew(t, h.create, "create G1", []any{"G1", int64(0)})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(0)}})
	h.bean("G2", 5000)
	nwViewsAssertNew(t, h.create, "create G2", []any{"G2", int64(5000)})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(0)}, {"G2", int64(5000)}})
	h.market("G2")
	nwViewsAssertOld(t, h.create, "create delete G2", []any{"G2", int64(5000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(0)}})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(0)}})
	h.bean("G3", 10000)
	nwViewsAssertIRPair(t, h.create, "create G3", []any{"G3", int64(10000)}, []any{"G1", int64(0)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", int64(10000)}})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", int64(10000)}})
	h.bean("G4", 15000)
	nwViewsAssertNew(t, h.create, "create G4", []any{"G4", int64(15000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", int64(10000)}, {"G4", int64(15000)}})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", int64(10000)}, {"G4", int64(15000)}})
	h.market("G3")
	nwViewsAssertOld(t, h.create, "create delete G3", []any{"G3", int64(10000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G4", int64(15000)}})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G4", int64(15000)}})
	h.bean("G5", 21000)
	nwViewsAssertNew(t, h.create, "create G5", []any{"G5", int64(21000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G4", int64(15000)}, {"G5", int64(21000)}})
}

// TestInfraNWViewsExtTimeWindowSceneThreeParity mirrors
// InfraExtTimeWindowSceneThree: despite the class name this is a
// win:time(10 sec) window holding whole SupportBean events with an irstream
// wildcard consumer and an on-delete trigger keyed by SupportBean_A.id.
func TestInfraNWViewsExtTimeWindowSceneThreeParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[nwViewsBean](env, "SupportBean")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwViewsBeanA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "ABCWin", schema, NamedWindowRetention(TimeWindow(10*time.Second))); err != nil {
		t.Fatal(err)
	}
	source := From[nwViewsBean](env, "SupportBean")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow(
		"ABCWin",
		SetColumn("theString", Field[nwViewsBean, string]("theString")),
		SetColumn("intBoxed", Field[nwViewsBean, int]("intBoxed")),
		SetColumn("longBoxed", Field[nwViewsBean, int64]("longBoxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[nwViewsBeanA](env, "SupportBean_A")).DeleteFromNamedWindow(
		"ABCWin",
		Equal[string](NamedWindowField[string]("theString"), Field[nwViewsBeanA, string]("id")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "ABCWin").Query(
		StatementName("s0"),
		WithOldStream(),
	))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deploy := func(plan Plan) *Deployment {
		t.Helper()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		return deployment
	}
	consumerDeployment := deploy(consumerPlan)
	deploy(insertPlan)
	deploy(deletePlan)

	probe := &nwViewsProbe{rowOf: func(event Event) []any { return []any{event.Get("theString").Any()} }}
	nwViewsSubscribeStatement(t, consumerDeployment.Statements()[0], probe)

	advance := func(millis int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(millis).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	send := func(theString string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBean{TheString: theString}); err != nil {
			t.Fatal(err)
		}
	}
	sendA := func(id string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBeanA{ID: id}); err != nil {
			t.Fatal(err)
		}
	}

	advance(0)

	advance(1000)
	send("E1")
	nwViewsAssertNew(t, probe, "s0 E1", []any{"E1"})

	advance(2000)
	sendA("E1")
	nwViewsAssertOld(t, probe, "s0 delete E1", []any{"E1"})

	advance(3000)
	send("E2")
	nwViewsAssertNew(t, probe, "s0 E2", []any{"E2"})

	advance(3000)
	send("E3")
	nwViewsAssertNew(t, probe, "s0 E3", []any{"E3"})

	sendA("E3")
	nwViewsAssertOld(t, probe, "s0 delete E3", []any{"E3"})

	advance(12999)
	nwViewsAssertNotInvoked(t, probe, "s0")
	advance(13000)
	nwViewsAssertOld(t, probe, "s0 expire E2", []any{"E2"})
}

// TestInfraNWViewsTimeOrderWindowParity mirrors InfraTimeOrderWindow: a
// time_order(value, 10 sec) window keeps events sorted by their external
// timestamp and expires them under the engine clock at timestamp plus the
// period.
func TestInfraNWViewsTimeOrderWindowParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowTOW", TimeOrder(Field[nwViewsKVLong, int64]("value"), 10*time.Second), "longBoxed", true, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(5000)
	h.bean("E1", 3000)
	nwViewsAssertNew(t, h.create, "create E1", []any{"E1", int64(3000)})
	nwViewsAssertNew(t, h.consumer, "s0 E1", []any{"E1", int64(3000)})

	h.advance(6000)
	h.bean("E2", 2000)
	nwViewsAssertNew(t, h.create, "create E2", []any{"E2", int64(2000)})
	nwViewsAssertNew(t, h.consumer, "s0 E2", []any{"E2", int64(2000)})

	h.advance(10000)
	h.bean("E3", 1000)
	nwViewsAssertNew(t, h.create, "create E3", []any{"E3", int64(1000)})
	nwViewsAssertNew(t, h.consumer, "s0 E3", []any{"E3", int64(1000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E3", int64(1000)}, {"E2", int64(2000)}, {"E1", int64(3000)}})

	h.advance(11000)
	nwViewsAssertOld(t, h.create, "create expire E3", []any{"E3", int64(1000)})
	nwViewsAssertOld(t, h.consumer, "s0 expire E3", []any{"E3", int64(1000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E2", int64(2000)}, {"E1", int64(3000)}})

	h.market("E2")
	nwViewsAssertOld(t, h.create, "create delete E2", []any{"E2", int64(2000)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E2", []any{"E2", int64(2000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(3000)}})

	h.advance(12999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	h.advance(13000)
	nwViewsAssertOld(t, h.create, "create expire E1", []any{"E1", int64(3000)})
	nwViewsAssertOld(t, h.consumer, "s0 expire E1", []any{"E1", int64(3000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.advance(100000)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
}

// TestInfraNWViewsTimeOrderSceneTwoParity mirrors InfraTimeOrderSceneTwo:
// ext:time_order(value, 10) with out-of-order arrivals; an event whose
// external timestamp is already expired under the engine clock passes
// straight through as an IR pair with itself.
func TestInfraNWViewsTimeOrderSceneTwoParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindow", TimeOrder(Field[nwViewsKVLong, int64]("value"), 10*time.Second), "longBoxed", false, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(20000)
	h.bean("G1", 23000)
	nwViewsAssertNew(t, h.create, "create G1", []any{"G1", int64(23000)})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(23000)}})
	h.advance(20000)
	h.bean("G2", 19000)
	nwViewsAssertNew(t, h.create, "create G2", []any{"G2", int64(19000)})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G2", int64(19000)}, {"G1", int64(23000)}})
	h.advance(21000)
	h.bean("G3", 10000)
	nwViewsAssertIRPair(t, h.create, "create G3", []any{"G3", int64(10000)}, []any{"G3", int64(10000)})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G2", int64(19000)}, {"G1", int64(23000)}})
	h.advance(21000)
	h.market("G2")
	nwViewsAssertOld(t, h.create, "create delete G2", []any{"G2", int64(19000)})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(23000)}})
	h.advance(22000)
	h.bean("G4", 18000)
	nwViewsAssertNew(t, h.create, "create G4", []any{"G4", int64(18000)})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G4", int64(18000)}, {"G1", int64(23000)}})
	h.advance(23000)
	h.bean("G5", 22000)
	nwViewsAssertNew(t, h.create, "create G5", []any{"G5", int64(22000)})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G4", int64(18000)}, {"G5", int64(22000)}, {"G1", int64(23000)}})
	h.advance(27999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	h.advance(28000)
	nwViewsAssertOld(t, h.create, "create expire G4", []any{"G4", int64(18000)})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G5", int64(22000)}, {"G1", int64(23000)}})
	h.advance(31999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	h.advance(32000)
	nwViewsAssertOld(t, h.create, "create expire G5", []any{"G5", int64(22000)})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(23000)}})
	h.advance(32000)
	h.bean("G6", 25000)
	nwViewsAssertNew(t, h.create, "create G6", []any{"G6", int64(25000)})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", int64(23000)}, {"G6", int64(25000)}})
	h.advance(32000)
	h.market("G1")
	nwViewsAssertOld(t, h.create, "create delete G1", []any{"G1", int64(23000)})

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G6", int64(25000)}})
	h.advance(34999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	h.advance(35000)
	nwViewsAssertOld(t, h.create, "create expire G6", []any{"G6", int64(25000)})
}

// TestInfraNWViewsTimeLengthBatchParity mirrors InfraTimeLengthBatch: a
// time_length_batch(10 sec, 3) window completes batches either at the size
// boundary or at the anchored time boundary, whichever comes first; each
// completed batch is one new-data delivery and the previous batch leaves as
// old data in the same callback.
func TestInfraNWViewsTimeLengthBatchParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowTLB", TimeLengthBatch(10*time.Second, 3), "longBoxed", true, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(1000)
	h.bean("E1", 1)
	h.bean("E2", 2)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(2)}})

	h.market("E2")
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}})

	h.bean("E3", 3)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E3", int64(3)}})

	h.bean("E4", 4)
	nwViewsAssertNew(t, h.create, "create size flush", []any{"E1", int64(1)}, []any{"E3", int64(3)}, []any{"E4", int64(4)})
	nwViewsAssertNew(t, h.consumer, "s0 size flush", []any{"E1", int64(1)}, []any{"E3", int64(3)}, []any{"E4", int64(4)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.advance(5000)
	h.bean("E5", 5)
	h.bean("E6", 6)
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E5", int64(5)}, {"E6", int64(6)}})

	h.market("E5")
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E6", int64(6)}})

	h.advance(10999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")

	h.advance(11000)
	if len(h.create.newRows) != 1 || len(h.create.oldRows) != 3 {
		t.Fatalf("create time flush = new %#v old %#v, want new=[E6] old=[E1,E3,E4]", h.create.newRows, h.create.oldRows)
	}
	nwViewsAssertRows(t, "create time flush new", h.create.newRows, [][]any{{"E6", int64(6)}})
	nwViewsAssertRows(t, "create time flush old", h.create.oldRows, [][]any{{"E1", int64(1)}, {"E3", int64(3)}, {"E4", int64(4)}})
	h.create.reset()
	h.consumer.reset()
}

// TestInfraNWViewsTimeLengthBatchSceneTwoParity mirrors
// InfraTimeLengthBatchSceneTwo: win:time_length_batch(10 sec, 4); an
// all-empty time rollover disarms the schedule and the next insert re-arms
// it (first flush at 25000 after re-arm at 15000, not at the 21000 the
// original schedule would have produced).
func TestInfraNWViewsTimeLengthBatchSceneTwoParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindow", TimeLengthBatch(10*time.Second, 4), "intBoxed", false, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(1000)
	h.beanInt("G1", 1)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.advance(5000)
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 1}})
	h.beanInt("G2", 2)
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 1}, {"G2", 2}})
	h.market("G2")
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 1}})
	h.market("G1")
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
	h.advance(11000)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.advance(15000)
	h.beanInt("G3", 3)
	h.beanInt("G4", 4)
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 3}, {"G4", 4}})
	h.advance(16000)
	h.beanInt("G5", 5)
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 3}, {"G4", 4}, {"G5", 5}})
	h.market("G5")
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 3}, {"G4", 4}})
	h.advance(18000)
	h.beanInt("G6", 6)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.advance(24999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	h.advance(25000)
	nwViewsAssertNew(t, h.create, "create flush", []any{"G3", 3}, []any{"G4", 4}, []any{"G6", 6})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.advance(28000)
	h.beanInt("G7", 7)
	h.beanInt("G8", 8)
	h.beanInt("G9", 9)
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G7", 7}, {"G8", 8}, {"G9", 9}})
	h.market("G7")
	h.market("G9")
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.advance(34999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G8", 8}})
	h.advance(35000)
	if len(h.create.newRows) != 1 || len(h.create.oldRows) != 3 {
		t.Fatalf("create flush = new %#v old %#v, want new=[G8] old=[G3,G4,G6]", h.create.newRows, h.create.oldRows)
	}
	nwViewsAssertRows(t, "create flush new", h.create.newRows, [][]any{{"G8", 8}})
	nwViewsAssertRows(t, "create flush old", h.create.oldRows, [][]any{{"G3", 3}, {"G4", 4}, {"G6", 6}})
	h.create.reset()
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
}

// TestInfraNWViewsExternallyTimedBatchParity mirrors
// InfraExternallyTimedBatch: an ext_timed_batch(value, 10 sec, 0L) window
// accumulates events against epoch-anchored boundaries; deletes of
// accumulated events fire old data, and an arriving event whose timestamp
// crosses the next boundary flushes the accumulated batch as new data and
// the previously completed batch as old data.
func TestInfraNWViewsExternallyTimedBatchParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowETB", ExternallyTimedBatch(Field[nwViewsKVLong, int64]("value"), 10*time.Second), "longBoxed", true, WithStartTime(time.Unix(0, 0).UTC()))

	h.bean("E1", 1000)
	h.bean("E2", 8000)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")

	h.bean("E3", 9999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1000)}, {"E2", int64(8000)}, {"E3", int64(9999)}})

	h.market("E2")
	nwViewsAssertOld(t, h.create, "create delete E2", []any{"E2", int64(8000)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E2", []any{"E2", int64(8000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1000)}, {"E3", int64(9999)}})

	h.bean("E4", 10000)
	nwViewsAssertNew(t, h.create, "create flush", []any{"E1", int64(1000)}, []any{"E3", int64(9999)})
	nwViewsAssertNew(t, h.consumer, "s0 flush", []any{"E1", int64(1000)}, []any{"E3", int64(9999)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E4", int64(10000)}})

	h.market("E4")
	nwViewsAssertOld(t, h.create, "create delete E4", []any{"E4", int64(10000)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E4", []any{"E4", int64(10000)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.bean("E5", 14000)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E5", int64(14000)}})

	h.bean("E6", 21000)
	if len(h.create.newRows) != 1 || len(h.create.oldRows) != 2 {
		t.Fatalf("create flush = new %#v old %#v, want new=[E5] old=[E1,E3]", h.create.newRows, h.create.oldRows)
	}
	nwViewsAssertRows(t, "create flush new", h.create.newRows, [][]any{{"E5", int64(14000)}})
	nwViewsAssertRows(t, "create flush old", h.create.oldRows, [][]any{{"E1", int64(1000)}, {"E3", int64(9999)}})
	h.create.reset()
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E6", int64(21000)}})
}

// TestInfraNWViewsTimeFirstWindowParity mirrors InfraTimeFirstWindow: a
// firsttime(10 sec) window admits events only before the oldest retained
// event plus the period, drops later inserts silently and never expires
// events on the timer.
func TestInfraNWViewsTimeFirstWindowParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowTFW", FirstTime(10*time.Second), "longBoxed", true, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(1000)
	h.bean("E1", 1)
	nwViewsAssertNew(t, h.create, "create E1", []any{"E1", int64(1)})
	nwViewsAssertNew(t, h.consumer, "s0 E1", []any{"E1", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}})

	h.advance(5000)
	h.bean("E2", 2)
	nwViewsAssertNew(t, h.create, "create E2", []any{"E2", int64(2)})
	nwViewsAssertNew(t, h.consumer, "s0 E2", []any{"E2", int64(2)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(2)}})

	h.advance(10000)
	h.bean("E3", 3)
	nwViewsAssertNew(t, h.create, "create E3", []any{"E3", int64(3)})
	nwViewsAssertNew(t, h.consumer, "s0 E3", []any{"E3", int64(3)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(2)}, {"E3", int64(3)}})

	h.advance(12000)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(2)}, {"E3", int64(3)}})

	h.bean("E4", 4)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(2)}, {"E3", int64(3)}})

	h.market("E2")
	nwViewsAssertOld(t, h.create, "create delete E2", []any{"E2", int64(2)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E2", []any{"E2", int64(2)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E3", int64(3)}})

	h.advance(100000)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
}

// TestInfraNWViewsTimeBatchParity mirrors InfraTimeBatch: a time_batch(10
// sec) window accumulates silently, delivers the completed batch as new data
// at the batch boundary and as old data at the following boundary; deletes
// of unflushed events are silent.
func TestInfraNWViewsTimeBatchParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowTB", TimeBatch(10*time.Second), "longBoxed", true, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(1000)
	h.bean("E1", 1)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")

	h.advance(5000)
	h.bean("E2", 2)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.advance(10000)
	h.bean("E3", 3)
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(2)}, {"E3", int64(3)}})

	h.market("E2")
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E3", int64(3)}})

	h.advance(10999)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")

	h.advance(11000)
	nwViewsAssertNew(t, h.create, "create flush", []any{"E1", int64(1)}, []any{"E3", int64(3)})
	nwViewsAssertNew(t, h.consumer, "s0 flush", []any{"E1", int64(1)}, []any{"E3", int64(3)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.advance(21000)
	nwViewsAssertOld(t, h.create, "create batch leave", []any{"E1", int64(1)}, []any{"E3", int64(3)})
	nwViewsAssertOld(t, h.consumer, "s0 batch leave", []any{"E1", int64(1)}, []any{"E3", int64(3)})

	h.bean("E4", 4)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E4", int64(4)}})

	h.market("E4")
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.advance(31000)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")
}

// TestInfraNWViewsTimeBatchSceneTwoParity mirrors InfraTimeBatchSceneTwo:
// win:time_batch(10) over an intBoxed projection; the boundary schedule
// anchored at the first insert slides by the period even while batches are
// empty, and a boundary with both a completed and a new batch delivers old
// and new data in one callback.
func TestInfraNWViewsTimeBatchSceneTwoParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindow", TimeBatch(10*time.Second), "intBoxed", false, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(1000)
	h.beanInt("G1", 1)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.advance(5000)
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 1}})
	h.beanInt("G2", 2)
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 1}, {"G2", 2}})
	h.market("G2")
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 1}})
	h.market("G1")
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
	h.advance(11000)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.advance(15000)
	h.beanInt("G3", 3)
	h.beanInt("G4", 4)
	h.beanInt("G5", 5)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.market("G5")
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G3", 3}, {"G4", 4}})
	h.advance(18000)
	h.beanInt("G6", 6)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.advance(21000)
	nwViewsAssertNew(t, h.create, "create flush", []any{"G3", 3}, []any{"G4", 4}, []any{"G6", 6})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)

	h.advance(22000)
	h.beanInt("G7", 7)
	h.beanInt("G8", 8)
	h.beanInt("G9", 9)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.market("G7")
	h.market("G9")
	nwViewsAssertNotInvoked(t, h.create, "create")

	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"G8", 8}})
	h.advance(31000)
	if len(h.create.newRows) != 1 || len(h.create.oldRows) != 3 {
		t.Fatalf("create flush batch = new %#v old %#v, want new=[G8] old=[G3,G4,G6]", h.create.newRows, h.create.oldRows)
	}
	nwViewsAssertRows(t, "create flush new", h.create.newRows, [][]any{{"G8", 8}})
	nwViewsAssertRows(t, "create flush old", h.create.oldRows, [][]any{{"G3", 3}, {"G4", 4}, {"G6", 6}})
	h.create.reset()
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
}

// TestInfraNWViewsTimeBatchLateConsumerParity mirrors
// InfraTimeBatchLateConsumer: a sum aggregation consumer deployed after the
// window started accumulating receives the completed batch as one new-data
// delivery at the boundary and aggregates it to 6.
func TestInfraNWViewsTimeBatchLateConsumerParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowTBLC", TimeBatch(10*time.Second), "longBoxed", false, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(0)
	h.bean("E1", 1)
	nwViewsAssertNotInvoked(t, h.create, "create")

	h.advance(5000)
	h.bean("E2", 2)
	nwViewsAssertNotInvoked(t, h.create, "create")

	// Late aggregation consumer: select sum(value) as value from MyWindowTBLC.
	consumerPlan, err := h.env.Build(FromNamedWindow(h.env, "MyWindowTBLC").Aggregate(
		Alias("value", Sum[int64](Field[any, int64]("value"))),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := h.engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var s0Values []any
	s0Count := 0
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		s0Count++
		s0Values = s0Values[:0]
		for _, result := range batch.New {
			s0Values = append(s0Values, result.Get("value").Any())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	h.advance(8000)
	h.bean("E3", 3)
	if s0Count != 0 {
		t.Fatalf("s0 invoked %d times before the batch boundary", s0Count)
	}

	h.advance(10000)
	if s0Count != 1 || len(s0Values) != 1 || s0Values[0] != int64(6) {
		t.Fatalf("s0 batch aggregate = values %#v count %d, want [6] once", s0Values, s0Count)
	}
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
}
