package esper

import (
	"context"
	"errors"
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

// TestInfraNWViewsLengthWindowPerGroupParity mirrors
// InfraLengthWindowPerGroup: a #groupwin(value)#length(2) window retains
// the newest two events per group; the oldest event of an over-full group
// leaves as old data in the same delta as the triggering insert.
func TestInfraNWViewsLengthWindowPerGroupParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowWPG", GroupWindow(Field[nwViewsKVLong, int64]("value"), LengthWindow(2)), "longBoxed", true)

	h.bean("E1", 1)
	nwViewsAssertNew(t, h.create, "create E1", []any{"E1", int64(1)})
	nwViewsAssertNew(t, h.consumer, "s0 E1", []any{"E1", int64(1)})

	h.bean("E2", 1)
	nwViewsAssertNew(t, h.create, "create E2", []any{"E2", int64(1)})
	nwViewsAssertNew(t, h.consumer, "s0 E2", []any{"E2", int64(1)})

	h.bean("E3", 2)
	nwViewsAssertNew(t, h.create, "create E3", []any{"E3", int64(2)})
	nwViewsAssertNew(t, h.consumer, "s0 E3", []any{"E3", int64(2)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E2", int64(1)}, {"E3", int64(2)}})

	h.market("E2")
	nwViewsAssertOld(t, h.create, "create delete E2", []any{"E2", int64(1)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E2", []any{"E2", int64(1)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), [][]any{{"E1", int64(1)}, {"E3", int64(2)}})

	h.bean("E4", 1)
	nwViewsAssertNew(t, h.create, "create E4", []any{"E4", int64(1)})
	nwViewsAssertNew(t, h.consumer, "s0 E4", []any{"E4", int64(1)})

	h.bean("E5", 1)
	nwViewsAssertIRPair(t, h.create, "create E5", []any{"E5", int64(1)}, []any{"E1", int64(1)})
	nwViewsAssertIRPair(t, h.consumer, "s0 E5", []any{"E5", int64(1)}, []any{"E1", int64(1)})

	h.bean("E6", 2)
	nwViewsAssertNew(t, h.create, "create E6", []any{"E6", int64(2)})
	nwViewsAssertNew(t, h.consumer, "s0 E6", []any{"E6", int64(2)})

	h.market("E6")
	nwViewsAssertOld(t, h.create, "create delete E6", []any{"E6", int64(2)})
	nwViewsAssertOld(t, h.consumer, "s0 delete E6", []any{"E6", int64(2)})

	h.bean("E7", 2)
	nwViewsAssertNew(t, h.create, "create E7", []any{"E7", int64(2)})
	nwViewsAssertNew(t, h.consumer, "s0 E7", []any{"E7", int64(2)})

	h.bean("E8", 2)
	nwViewsAssertIRPair(t, h.create, "create E8", []any{"E8", int64(2)}, []any{"E3", int64(2)})
	nwViewsAssertIRPair(t, h.consumer, "s0 E8", []any{"E8", int64(2)}, []any{"E3", int64(2)})
}

// TestInfraNWViewsTimeBatchPerGroupParity mirrors InfraTimeBatchPerGroup:
// a #groupwin(value)#time_batch(10 sec) window accumulates silently against
// the anchored boundary schedule and delivers the completed batch with
// events grouped by the group key in first-seen group order.
func TestInfraNWViewsTimeBatchPerGroupParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowTBPG", GroupWindow(Field[nwViewsKVLong, int64]("value"), TimeBatch(10*time.Second)), "longBoxed", true, WithStartTime(time.Unix(0, 0).UTC()))

	h.advance(1000)
	h.bean("E1", 10)
	h.bean("E2", 20)
	h.bean("E3", 20)
	h.bean("E4", 10)
	nwViewsAssertNotInvoked(t, h.create, "create")
	nwViewsAssertNotInvoked(t, h.consumer, "s0")

	h.advance(11000)
	nwViewsAssertNew(t, h.create, "create flush", []any{"E1", int64(10)}, []any{"E4", int64(10)}, []any{"E2", int64(20)}, []any{"E3", int64(20)})
	nwViewsAssertNew(t, h.consumer, "s0 flush", []any{"E1", int64(10)}, []any{"E4", int64(10)}, []any{"E2", int64(20)}, []any{"E3", int64(20)})
	nwViewsAssertRows(t, "iterator", nwViewsSnapshot(t, h.window), nil)
}

// nwViewsSubscribeResults captures consumer statement rows (projection and
// aggregate results alike) into a plain probe for the WithDelete matrix.
func nwViewsSubscribeResults(t *testing.T, statement *Statement, fields []string, probe *nwViewsProbe) {
	t.Helper()
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		probe.count++
		probe.newRows = nil
		for _, result := range batch.New {
			row := make([]any, 0, len(fields))
			for _, field := range fields {
				row = append(row, result.Get(field).Any())
			}
			probe.newRows = append(probe.newRows, row)
		}
		probe.oldRows = nil
		for _, result := range batch.Old {
			row := make([]any, 0, len(fields))
			for _, field := range fields {
				row = append(row, result.Get(field).Any())
			}
			probe.oldRows = append(probe.oldRows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestInfraNWViewsWithDeleteParity mirrors InfraWithDeleteUseAs,
// InfraWithDeleteFirstAs, InfraWithDeleteSecondAs and InfraWithDeleteNoAs:
// the four EPL on-delete alias variants collapse into one Go chain
// construction because typed trigger/window fields make stream aliases
// unnecessary. The shared tryCreateWindow consumer matrix is asserted: s0
// doubles values, s2 groups sums per key with IR pairs, s3 filters
// value >= 10, and a no-op delete fires only the delete statement.
func TestInfraNWViewsWithDeleteParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[nwViewsBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwViewsMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := RegisterStruct[nwViewsKVLong](env, "MyWindowWDKV")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowWD", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	source := From[nwViewsBean](env, "SupportBean")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow(
		"MyWindowWD",
		SetColumn("key", Field[nwViewsBean, string]("theString")),
		SetColumn("value", Field[nwViewsBean, int64]("longBoxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[nwViewsMarket](env, "SupportMarketDataBean")).DeleteFromNamedWindow(
		"MyWindowWD",
		Equal[string](NamedWindowField[string]("key"), Field[nwViewsMarket, string]("symbol")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromNamedWindow(env, "MyWindowWD").Select(
		Alias("key", Field[any, string]("key")),
		Alias("value", Multiply[int64](Field[any, int64]("value"), Literal[int64](2))),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	s2Plan, err := env.Build(FromNamedWindow(env, "MyWindowWD").GroupBy(Field[any, string]("key")).Select(
		Alias("key", Field[any, string]("key")),
		Alias("value", Sum[int64](Field[any, int64]("value"))),
	).Query(StatementName("s2"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	s3Plan, err := env.Build(FromNamedWindow(env, "MyWindowWD").Filter(
		GreaterOrEqual[int64](Field[any, int64]("value"), Literal[int64](10)),
	).Query(StatementName("s3"), WithOldStream()))
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
	s0Deployment := deploy(s0Plan)
	s2Deployment := deploy(s2Plan)
	s3Deployment := deploy(s3Plan)
	deploy(insertPlan)
	deleteDeployment := deploy(deletePlan)

	window, ok := engine.NamedWindow("MyWindowWD")
	if !ok {
		t.Fatal("named window MyWindowWD is missing")
	}
	create := &nwViewsProbe{rowOf: nwViewsKVRow}
	nwViewsSubscribeWindow(t, window, create)
	s0 := &nwViewsProbe{}
	nwViewsSubscribeResults(t, s0Deployment.Statements()[0], []string{"key", "value"}, s0)
	s2 := &nwViewsProbe{}
	nwViewsSubscribeResults(t, s2Deployment.Statements()[0], []string{"key", "value"}, s2)
	s3 := &nwViewsProbe{}
	nwViewsSubscribeResults(t, s3Deployment.Statements()[0], []string{"key", "value"}, s3)
	// Go delete-trigger statements deliver deleted window events as the old
	// data of the trigger batch (Esper delivers them as the new data of the
	// on-delete statement); both fire exactly when rows matched, so the Java
	// invocation assertions are mirrored by counting batches.
	deleteCount := 0
	if _, err := deleteDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		deleteCount++
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendBean := func(theString string, value int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBean{TheString: theString, LongBoxed: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendMarket := func(symbol string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsMarket{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func(statement *Statement) [][]any {
		t.Helper()
		queryResult, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		results := queryResult.Results()
		rows := make([][]any, 0, len(results))
		for _, result := range results {
			rows = append(rows, []any{result.Get("key").Any(), result.Get("value").Any()})
		}
		return rows
	}

	sendBean("E1", 10)
	nwViewsAssertNew(t, s0, "s0 E1", []any{"E1", int64(20)})
	nwViewsAssertIRPair(t, s2, "s2 E1", []any{"E1", int64(10)}, []any{"E1", nil})
	nwViewsAssertNew(t, s3, "s3 E1", []any{"E1", int64(10)})
	nwViewsAssertNew(t, create, "create E1", []any{"E1", int64(10)})
	nwViewsAssertRows(t, "create iterator", nwViewsSnapshot(t, window), [][]any{{"E1", int64(10)}})
	nwViewsAssertRows(t, "s0 iterator", snapshot(s0Deployment.Statements()[0]), [][]any{{"E1", int64(20)}})

	sendBean("E2", 20)
	nwViewsAssertNew(t, s0, "s0 E2", []any{"E2", int64(40)})
	nwViewsAssertIRPair(t, s2, "s2 E2", []any{"E2", int64(20)}, []any{"E2", nil})
	nwViewsAssertNew(t, s3, "s3 E2", []any{"E2", int64(20)})
	nwViewsAssertNew(t, create, "create E2", []any{"E2", int64(20)})
	nwViewsAssertRows(t, "create iterator", nwViewsSnapshot(t, window), [][]any{{"E1", int64(10)}, {"E2", int64(20)}})
	nwViewsAssertRows(t, "s0 iterator", snapshot(s0Deployment.Statements()[0]), [][]any{{"E1", int64(20)}, {"E2", int64(40)}})

	sendBean("E3", 5)
	nwViewsAssertNew(t, s0, "s0 E3", []any{"E3", int64(10)})
	nwViewsAssertIRPair(t, s2, "s2 E3", []any{"E3", int64(5)}, []any{"E3", nil})
	nwViewsAssertNotInvoked(t, s3, "s3")
	nwViewsAssertNew(t, create, "create E3", []any{"E3", int64(5)})
	nwViewsAssertRows(t, "create iterator", nwViewsSnapshot(t, window), [][]any{{"E1", int64(10)}, {"E2", int64(20)}, {"E3", int64(5)}})

	sendMarket("E1")
	if deleteCount != 1 {
		t.Fatalf("delete invocations = %d, want 1", deleteCount)
	}
	nwViewsAssertOld(t, s0, "s0 delete E1", []any{"E1", int64(20)})
	nwViewsAssertIRPair(t, s2, "s2 delete E1", []any{"E1", nil}, []any{"E1", int64(10)})
	nwViewsAssertOld(t, s3, "s3 delete E1", []any{"E1", int64(10)})
	nwViewsAssertOld(t, create, "create delete E1", []any{"E1", int64(10)})
	nwViewsAssertRows(t, "create iterator", nwViewsSnapshot(t, window), [][]any{{"E2", int64(20)}, {"E3", int64(5)}})

	// Deleting the same key again matches nothing: neither the window and
	// its consumers nor the delete trigger fire (Esper on-delete child views
	// deliver deleted events only when rows matched; the Java
	// assertListenerInvoked here is satisfied by the lingering first-delete
	// invocation and does not prove a no-op dispatch).
	sendMarket("E1")
	if deleteCount != 1 {
		t.Fatalf("delete invocations = %d, want 1 after the no-op delete", deleteCount)
	}
	nwViewsAssertNotInvoked(t, s0, "s0")
	nwViewsAssertNotInvoked(t, s2, "s2")
	nwViewsAssertNotInvoked(t, create, "create")
	nwViewsAssertRows(t, "create iterator", nwViewsSnapshot(t, window), [][]any{{"E2", int64(20)}, {"E3", int64(5)}})

	sendMarket("E2")
	nwViewsAssertOld(t, s0, "s0 delete E2", []any{"E2", int64(40)})
	nwViewsAssertIRPair(t, s2, "s2 delete E2", []any{"E2", nil}, []any{"E2", int64(20)})
	nwViewsAssertOld(t, s3, "s3 delete E2", []any{"E2", int64(20)})
	nwViewsAssertOld(t, create, "create delete E2", []any{"E2", int64(20)})
	nwViewsAssertRows(t, "create iterator", nwViewsSnapshot(t, window), [][]any{{"E3", int64(5)}})

	sendMarket("E3")
	nwViewsAssertOld(t, s0, "s0 delete E3", []any{"E3", int64(10)})
	nwViewsAssertIRPair(t, s2, "s2 delete E3", []any{"E3", nil}, []any{"E3", int64(5)})
	nwViewsAssertNotInvoked(t, s3, "s3")
	nwViewsAssertOld(t, create, "create delete E3", []any{"E3", int64(5)})
	if deleteCount != 3 {
		t.Fatalf("delete invocations = %d, want 3", deleteCount)
	}
	nwViewsAssertRows(t, "create iterator", nwViewsSnapshot(t, window), nil)
}

// nwViewsStatementSnapshot reads a consumer statement iterator as property
// rows, mirroring assertPropsPerRowIterator on consumer statements.
func nwViewsStatementSnapshot(t *testing.T, statement *Statement, fields ...string) [][]any {
	t.Helper()
	queryResult, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	results := queryResult.Results()
	rows := make([][]any, 0, len(results))
	for _, result := range results {
		row := make([]any, 0, len(fields))
		for _, field := range fields {
			row = append(row, result.Get(field).Any())
		}
		rows = append(rows, row)
	}
	return rows
}

// TestInfraNWViewsDoubleInsertSameWindowParity mirrors
// InfraDoubleInsertSameWindow: two insert-into statements writing the same
// window each deliver their own window delta (two listener invocations per
// source event), and the consumer observes both rows flattened.
func TestInfraNWViewsDoubleInsertSameWindowParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[nwViewsBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := RegisterStruct[nwViewsKVLong](env, "MyWindowDISMKV")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowDISM", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	source := From[nwViewsBean](env, "SupportBean")
	insertOnePlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow(
		"MyWindowDISM",
		SetColumn("key", Field[nwViewsBean, string]("theString")),
		SetColumn("value", Add[int64](Field[nwViewsBean, int64]("longBoxed"), Literal[int64](1))),
	).Query(StatementName("insert-one")))
	if err != nil {
		t.Fatal(err)
	}
	insertTwoPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow(
		"MyWindowDISM",
		SetColumn("key", Field[nwViewsBean, string]("theString")),
		SetColumn("value", Add[int64](Field[nwViewsBean, int64]("longBoxed"), Literal[int64](2))),
	).Query(StatementName("insert-two")))
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(FromNamedWindow(env, "MyWindowDISM").Query(StatementName("s0"), WithOldStream()))
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
	s0Deployment := deploy(s0Plan)
	deploy(insertOnePlan)
	deploy(insertTwoPlan)

	window, ok := engine.NamedWindow("MyWindowDISM")
	if !ok {
		t.Fatal("named window MyWindowDISM is missing")
	}
	windowCalls := 0
	var windowFlattened [][]any
	if _, err := window.Subscribe(func(_ context.Context, delta NamedWindowDelta) error {
		windowCalls++
		for _, event := range delta.New {
			windowFlattened = append(windowFlattened, nwViewsKVRow(event))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var s0Flattened [][]any
	if _, err := s0Deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("consumer result is not an event: %#v", result)
			}
			s0Flattened = append(s0Flattened, nwViewsKVRow(event))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), nwViewsBean{TheString: "E1", LongBoxed: 10}); err != nil {
		t.Fatal(err)
	}
	if windowCalls != 2 {
		t.Fatalf("window listener invocations = %d, want 2 individual deltas", windowCalls)
	}
	nwViewsAssertRows(t, "window flattened", windowFlattened, [][]any{{"E1", int64(11)}, {"E1", int64(12)}})
	nwViewsAssertRows(t, "s0 flattened", s0Flattened, [][]any{{"E1", int64(11)}, {"E1", int64(12)}})
}

// TestInfraNWViewsFilteringConsumerParity mirrors InfraFilteringConsumer: a
// unique(key) window with a filtered irstream consumer (value > 0 and
// value < 10). The consumer sees only matching events as new and matching
// removals as old; a unique replacement whose incoming event fails the
// filter delivers only the old row.
func TestInfraNWViewsFilteringConsumerParity(t *testing.T) {
	h := newNWViewsHarness(t, "MyWindowFC", Unique(Field[nwViewsKVInt, string]("key")), "intBoxed", false)
	s0Plan, err := h.env.Build(FromNamedWindow(h.env, "MyWindowFC").Filter(
		And(
			Greater[int](Field[any, int]("value"), Literal[int](0)),
			Less[int](Field[any, int]("value"), Literal[int](10)),
		),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	s0Deployment, err := h.engine.Deploy(context.Background(), s0Plan)
	if err != nil {
		t.Fatal(err)
	}
	s0 := &nwViewsProbe{}
	nwViewsSubscribeResults(t, s0Deployment.Statements()[0], []string{"key", "value"}, s0)
	s0Snapshot := func() [][]any { return nwViewsStatementSnapshot(t, s0Deployment.Statements()[0], "key", "value") }

	h.beanInt("G1", 5)
	nwViewsAssertNew(t, s0, "s0 G1", []any{"G1", 5})
	nwViewsAssertNew(t, h.create, "create G1", []any{"G1", 5})

	h.beanInt("G1", 15)
	nwViewsAssertOld(t, s0, "s0 replace G1", []any{"G1", 5})
	nwViewsAssertIRPair(t, h.create, "create replace G1", []any{"G1", 15}, []any{"G1", 5})

	h.beanInt("G2", 8)
	nwViewsAssertNew(t, s0, "s0 G2", []any{"G2", 8})
	nwViewsAssertNew(t, h.create, "create G2", []any{"G2", 8})
	nwViewsAssertRowsAnyOrder(t, "create iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 15}, {"G2", 8}})
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), [][]any{{"G2", 8}})

	h.market("G2")
	nwViewsAssertOld(t, s0, "s0 delete G2", []any{"G2", 8})
	nwViewsAssertOld(t, h.create, "create delete G2", []any{"G2", 8})

	h.beanInt("G3", -1)
	nwViewsAssertNotInvoked(t, s0, "s0")
	nwViewsAssertNew(t, h.create, "create G3", []any{"G3", -1})
	nwViewsAssertRowsAnyOrder(t, "create iterator", nwViewsSnapshot(t, h.window), [][]any{{"G1", 15}, {"G3", -1}})
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), nil)

	h.market("G3")
	nwViewsAssertNotInvoked(t, s0, "s0")
	nwViewsAssertOld(t, h.create, "create delete G3", []any{"G3", -1})

	h.beanInt("G1", 6)
	nwViewsAssertNew(t, s0, "s0 replace G1 back", []any{"G1", 6})
	h.beanInt("G2", 7)
	nwViewsAssertNew(t, s0, "s0 G2 back", []any{"G2", 7})
	nwViewsAssertRowsAnyOrder(t, "s0 iterator", s0Snapshot(), [][]any{{"G1", 6}, {"G2", 7}})
}

// nwViewsBeanTTL mirrors SupportBean (theString, longPrimitive) for the
// InfraNamedWindowTimeToLiveDelete execution.
type nwViewsBeanTTL struct {
	TheString     string `esper:"theString"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// nwViewsBeanS0 mirrors SupportBean_S0 (id, p00) delete triggers.
type nwViewsBeanS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// TestInfraNWViewsTimeToLiveDeleteParity mirrors InfraNamedWindowTimeToLiveDelete:
// a whole-bean timetolive window retaining rows until
// current_timestamp()+longPrimitive expires rows dynamically while an
// on-delete trigger removes rows by p00.
func TestInfraNWViewsTimeToLiveDeleteParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[nwViewsBeanTTL](env, "SupportBean")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwViewsBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	ttl := Func2("ttlDeadline", func(now time.Time, delta int64) int64 {
		return now.UnixMilli() + delta
	}, CurrentTime(), Field[nwViewsBeanTTL, int64]("longPrimitive"))
	if _, err := CreateNamedWindow(env, "MyWindowTTL", schema, NamedWindowRetention(TimeToLiveAt(ttl))); err != nil {
		t.Fatal(err)
	}
	mergePlan, err := env.Build(OnEvent(From[nwViewsBeanTTL](env, "SupportBean")).MergeInsertIntoNamedWindow(
		"MyWindowTTL", Literal(false), CopyMatchingFields(),
	).Query(StatementName("merge")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[nwViewsBeanS0](env, "SupportBean_S0")).DeleteFromNamedWindow(
		"MyWindowTTL",
		Equal[string](NamedWindowField[string]("theString"), Field[nwViewsBeanS0, string]("p00")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	for _, plan := range []Plan{mergePlan, deletePlan} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	window, ok := engine.NamedWindow("MyWindowTTL")
	if !ok {
		t.Fatal("time-to-live window is missing")
	}
	assertIterate := func(want ...string) {
		t.Helper()
		events, err := window.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		rows := make([][]any, 0, len(events))
		for _, event := range events {
			rows = append(rows, []any{event.Get("theString").Any()})
		}
		wanted := make([][]any, 0, len(want))
		for _, value := range want {
			wanted = append(wanted, []any{value})
		}
		nwViewsAssertRowsAnyOrder(t, "iterate", rows, wanted)
	}
	sendBean := func(theString string, longPrimitive int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBeanTTL{TheString: theString, LongPrimitive: longPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0 := func(p00 string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBeanS0{P00: p00}); err != nil {
			t.Fatal(err)
		}
	}
	advance := func(millis int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(millis).UTC()); err != nil {
			t.Fatal(err)
		}
	}

	sendBean("E1", 2000)
	sendBean("E2", 3000)
	sendBean("E3", 1000)
	sendBean("E4", 2000)
	assertIterate("E1", "E2", "E3", "E4")

	advance(500)

	sendS0("E2")
	assertIterate("E1", "E3", "E4")

	sendS0("E1")
	assertIterate("E3", "E4")

	advance(1000)
	assertIterate("E4")

	advance(2000)
	assertIterate()
}

// TestInfraNWViewsInvalidParity mirrors the InfraInvalid,
// InfraNamedWindowInvalidAlreadyExists and
// InfraNamedWindowInvalidConsumerDataWindow executions: invalid named-window
// declarations and consumers fail at register or build time in Go (Esper
// reports the same boundaries at compile/deploy time).
func TestInfraNWViewsInvalidParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[nwViewsKVLong](env, "MySimpleKeyValueMap")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwViewsBeanA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	value := Field[nwViewsKVLong, int64]("value")

	// create window MyWindowI1#groupwin(value)#uni(value): the groupwin child
	// must be a data window view.
	if _, err := CreateNamedWindow(env, "MyWindowI1", schema, NamedWindowRetention(GroupWindow(value, Unique(value)))); err == nil {
		t.Fatal("grouped unique named-window retention was accepted")
	} else if !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("grouped unique retention error = %v, want %s", err, ErrorInvalidRule)
	}

	// on X delete from dummy: the named window has not been declared.
	if _, err := env.Build(OnEvent(From[nwViewsBeanA](env, "SupportBean_A")).DeleteFromNamedWindow(
		"dummy",
		Equal[string](NamedWindowField[string]("key"), Field[nwViewsBeanA, string]("id")),
	).Query(StatementName("delete-dummy"))); err == nil {
		t.Fatal("delete from unknown named window was accepted")
	} else if !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("delete from unknown window error = %v, want %s", err, ErrorUnknownName)
	}

	// A named window by the same name has already been created.
	if _, err := CreateNamedWindow(env, "MyWindowAE", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowAE", schema, NamedWindowRetention(KeepAll())); err == nil {
		t.Fatal("duplicate named window was accepted")
	} else if !errors.Is(err, ErrorDependency) {
		t.Fatalf("duplicate named window error = %v, want %s", err, ErrorDependency)
	}

	// select ... from MyWindowAE#time(10 sec): consumers cannot declare a data
	// window view onto the named window.
	if _, err := env.Build(FromNamedWindow(env, "MyWindowAE").Window(TimeWindow(10 * time.Second)).Query(
		StatementName("consumer-data-window"),
	)); err == nil {
		t.Fatal("named-window consumer data window was accepted")
	} else if !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("consumer data window error = %v, want %s", err, ErrorInvalidRule)
	}
}

// nwViewsLateConsumerWindow wires the shared keep-all MySimpleKeyValueMap
// window, insert trigger and window probe used by the late-consumer,
// prior-stats and late-consumer-join executions.
func nwViewsLateConsumerWindow(t *testing.T, windowName string) (*Environment, *Engine, *NamedWindow, *nwViewsProbe, func(string, int64)) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[nwViewsBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := RegisterStruct[nwViewsKVLong](env, "MySimpleKeyValueMap")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, windowName, windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[nwViewsBean](env, "SupportBean")).InsertIntoNamedWindow(
		windowName,
		SetColumn("key", Field[nwViewsBean, string]("theString")),
		SetColumn("value", Field[nwViewsBean, int64]("longBoxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow(windowName)
	if !ok {
		t.Fatalf("named window %s is missing", windowName)
	}
	probe := &nwViewsProbe{rowOf: nwViewsKVRow}
	nwViewsSubscribeWindow(t, window, probe)
	send := func(theString string, longBoxed int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBean{TheString: theString, LongBoxed: longBoxed}); err != nil {
			t.Fatal(err)
		}
	}
	return env, engine, window, probe, send
}

// TestInfraNWViewsLateConsumerParity mirrors InfraLateConsumer: a univariate
// statistics irstream consumer (Esper #uni(value) selecting the derived
// average property) and a count(*) consumer deployed after the window already
// holds rows replay the current window state into their iterators and track
// subsequent inserts as IR pairs.
func TestInfraNWViewsLateConsumerParity(t *testing.T) {
	env, engine, _, create, send := nwViewsLateConsumerWindow(t, "MyWindowLCL")

	send("E1", 1)
	nwViewsAssertNew(t, create, "create E1", []any{"E1", int64(1)})
	send("E2", 2)
	nwViewsAssertNew(t, create, "create E2", []any{"E2", int64(2)})

	// select irstream average from MyWindowLCL#uni(value)
	s0Plan, err := env.Build(FromNamedWindow(env, "MyWindowLCL").Aggregate(
		Alias("average", UnivariateStatistics[int64](Field[any, int64]("value")).Average()),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	s0Deployment, err := engine.Deploy(context.Background(), s0Plan)
	if err != nil {
		t.Fatal(err)
	}
	s0 := &nwViewsProbe{}
	nwViewsSubscribeResults(t, s0Deployment.Statements()[0], []string{"average"}, s0)
	s0Snapshot := func() [][]any { return nwViewsStatementSnapshot(t, s0Deployment.Statements()[0], "average") }
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), [][]any{{1.5}})

	send("E3", 2)
	nwViewsAssertIRPair(t, s0, "s0 E3", []any{5.0 / 3}, []any{3.0 / 2})
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), [][]any{{5.0 / 3}})

	send("E4", 2)
	nwViewsAssertIRPair(t, s0, "s0 E4", []any{7.0 / 4}, []any{5.0 / 3})
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), [][]any{{7.0 / 4}})

	// select count(*) as cnt from MyWindowLCL
	s2Plan, err := env.Build(FromNamedWindow(env, "MyWindowLCL").Aggregate(
		Alias("cnt", CountAll()),
	).Query(StatementName("s2")))
	if err != nil {
		t.Fatal(err)
	}
	s2Deployment, err := engine.Deploy(context.Background(), s2Plan)
	if err != nil {
		t.Fatal(err)
	}
	s2Snapshot := func() [][]any { return nwViewsStatementSnapshot(t, s2Deployment.Statements()[0], "cnt") }
	nwViewsAssertRows(t, "s2 iterator", s2Snapshot(), [][]any{{int64(4)}})
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), [][]any{{7.0 / 4}})

	send("E5", 3)
	nwViewsAssertIRPair(t, s0, "s0 E5", []any{10.0 / 5}, []any{7.0 / 4})
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), [][]any{{10.0 / 5}})
	nwViewsAssertRows(t, "s2 iterator", s2Snapshot(), [][]any{{int64(5)}})
}

// TestInfraNWViewsFilteringConsumerLateStartParity mirrors
// InfraFilteringConsumerLateStart: a filtered irstream sum consumer deployed
// after rows already exist replays only matching rows into its initial
// iterator, and on-delete removals move the sum only when the removed row
// passes the consumer filter.
func TestInfraNWViewsFilteringConsumerLateStartParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[nwViewsBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwViewsMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := RegisterStruct[nwViewsKVInt](env, "MyWindowFCLSSchema")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowFCLS", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[nwViewsBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindowFCLS",
		SetColumn("key", Field[nwViewsBean, string]("theString")),
		SetColumn("value", Field[nwViewsBean, int]("intBoxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	sendInt := func(theString string, value int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBean{TheString: theString, IntBoxed: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendInt("G1", 5)
	sendInt("G2", 15)
	sendInt("G3", 2)

	// select irstream sum(value) as sumvalue from MyWindowFCLS(value > 0, value < 10)
	s0Plan, err := env.Build(FromNamedWindow(env, "MyWindowFCLS").Filter(
		And(
			Greater[int](Field[any, int]("value"), Literal[int](0)),
			Less[int](Field[any, int]("value"), Literal[int](10)),
		),
	).Aggregate(
		Alias("sumvalue", Sum[int](Field[any, int]("value"))),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	s0Deployment, err := engine.Deploy(context.Background(), s0Plan)
	if err != nil {
		t.Fatal(err)
	}
	s0 := &nwViewsProbe{}
	nwViewsSubscribeResults(t, s0Deployment.Statements()[0], []string{"sumvalue"}, s0)
	s0Snapshot := func() [][]any { return nwViewsStatementSnapshot(t, s0Deployment.Statements()[0], "sumvalue") }
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), [][]any{{7}})

	sendInt("G4", 1)
	nwViewsAssertIRPair(t, s0, "s0 G4", []any{8}, []any{7})
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), [][]any{{8}})

	sendInt("G5", 20)
	nwViewsAssertNotInvoked(t, s0, "s0")
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), [][]any{{8}})

	sendInt("G6", 9)
	nwViewsAssertIRPair(t, s0, "s0 G6", []any{17}, []any{8})
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), [][]any{{17}})

	// on SupportMarketDataBean as s0 delete from MyWindowFCLS as s1 where s0.symbol = s1.key
	deletePlan, err := env.Build(OnEvent(From[nwViewsMarket](env, "SupportMarketDataBean")).DeleteFromNamedWindow(
		"MyWindowFCLS",
		Equal[string](NamedWindowField[string]("key"), Field[nwViewsMarket, string]("symbol")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
		t.Fatal(err)
	}
	sendMarket := func(symbol string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsMarket{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}

	sendMarket("G4")
	nwViewsAssertIRPair(t, s0, "s0 delete G4", []any{16}, []any{17})
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), [][]any{{16}})

	sendMarket("G5")
	nwViewsAssertNotInvoked(t, s0, "s0")
	nwViewsAssertRows(t, "s0 iterator", s0Snapshot(), [][]any{{16}})
}

// TestInfraNWViewsPriorStatsParity mirrors InfraPriorStats: prior(1, key) and
// prior(2, key) read the window stream history behind each inserted event
// while a univariate statistics consumer (Esper #uni(value) selecting
// average) tracks the running mean. Esper prior(N, x) maps to Go Prior(N-1, x).
func TestInfraNWViewsPriorStatsParity(t *testing.T) {
	env, engine, _, _, send := nwViewsLateConsumerWindow(t, "MyWindowPS")

	// select prior(1, key) as priorKeyOne, prior(2, key) as priorKeyTwo from MyWindowPS
	s0Plan, err := env.Build(FromNamedWindow(env, "MyWindowPS").Select(
		Alias("priorKeyOne", Prior[string](0, Field[any, string]("key"))),
		Alias("priorKeyTwo", Prior[string](1, Field[any, string]("key"))),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	// select average from MyWindowPS#uni(value)
	s3Plan, err := env.Build(FromNamedWindow(env, "MyWindowPS").Aggregate(
		Alias("average", UnivariateStatistics[int64](Field[any, int64]("value")).Average()),
	).Query(StatementName("s3")))
	if err != nil {
		t.Fatal(err)
	}
	s0Deployment, err := engine.Deploy(context.Background(), s0Plan)
	if err != nil {
		t.Fatal(err)
	}
	s3Deployment, err := engine.Deploy(context.Background(), s3Plan)
	if err != nil {
		t.Fatal(err)
	}
	s0 := &nwViewsProbe{}
	nwViewsSubscribeResults(t, s0Deployment.Statements()[0], []string{"priorKeyOne", "priorKeyTwo"}, s0)
	s3 := &nwViewsProbe{}
	nwViewsSubscribeResults(t, s3Deployment.Statements()[0], []string{"average"}, s3)
	s3Snapshot := func() [][]any { return nwViewsStatementSnapshot(t, s3Deployment.Statements()[0], "average") }

	send("E1", 1)
	nwViewsAssertNew(t, s0, "s0 E1", []any{nil, nil})
	nwViewsAssertNew(t, s3, "s3 E1", []any{1.0})
	nwViewsAssertRows(t, "s3 iterator", s3Snapshot(), [][]any{{1.0}})

	send("E2", 2)
	nwViewsAssertNew(t, s0, "s0 E2", []any{"E1", nil})
	nwViewsAssertNew(t, s3, "s3 E2", []any{1.5})
	nwViewsAssertRows(t, "s3 iterator", s3Snapshot(), [][]any{{1.5}})

	send("E3", 2)
	nwViewsAssertNew(t, s0, "s0 E3", []any{"E2", "E1"})
	nwViewsAssertNew(t, s3, "s3 E3", []any{5.0 / 3})
	nwViewsAssertRows(t, "s3 iterator", s3Snapshot(), [][]any{{5.0 / 3}})

	send("E4", 2)
	nwViewsAssertNew(t, s0, "s0 E4", []any{"E3", "E2"})
	nwViewsAssertNew(t, s3, "s3 E4", []any{1.75})
	nwViewsAssertRows(t, "s3 iterator", s3Snapshot(), [][]any{{1.75}})
}

// nwViewsMarketVol mirrors SupportMarketDataBean (symbol, volume) for the
// late-consumer join execution.
type nwViewsMarketVol struct {
	Symbol string `esper:"symbol"`
	Volume int64  `esper:"volume"`
}

// TestInfraNWViewsLateConsumerJoinParity mirrors InfraLateConsumerJoin: a
// named-window left outer join against a keep-all market stream deployed
// after rows exist replays current window rows with null join columns, joins
// later market arrivals against every retained window row, and joins later
// window inserts against the retained market rows.
func TestInfraNWViewsLateConsumerJoinParity(t *testing.T) {
	env, engine, _, create, send := nwViewsLateConsumerWindow(t, "MyWindowLCJ")
	if _, err := RegisterStruct[nwViewsMarketVol](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}

	send("E1", 1)
	nwViewsAssertNew(t, create, "create E1", []any{"E1", int64(1)})
	send("E2", 1)
	nwViewsAssertNew(t, create, "create E2", []any{"E2", int64(1)})

	// select key, value, symbol from MyWindowLCJ as s0
	//   left outer join SupportMarketDataBean#keepall as s1 on s0.value = s1.volume
	s2Plan, err := env.Build(JoinMany(
		JoinRecordSource(FromNamedWindow(env, "MyWindowLCJ")),
		JoinRecordSource(From[nwViewsMarketVol](env, "SupportMarketDataBean").Window(KeepAll()).AsRecord()),
	).On(
		OnSourcesEqual(0, Field[any, int64]("value"), 1, Field[any, int64]("volume")),
	).LeftOuter().Select(
		SelectFrom(0, "key", Field[any, string]("key")),
		SelectFrom(0, "value", Field[any, int64]("value")),
		SelectFrom(1, "symbol", Field[any, string]("symbol")),
	).Query(StatementName("s2"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	s2Deployment, err := engine.Deploy(context.Background(), s2Plan)
	if err != nil {
		t.Fatal(err)
	}
	s2 := &nwViewsProbe{}
	nwViewsSubscribeResults(t, s2Deployment.Statements()[0], []string{"key", "value", "symbol"}, s2)
	s2Snapshot := func() [][]any { return nwViewsStatementSnapshot(t, s2Deployment.Statements()[0], "key", "value", "symbol") }
	nwViewsAssertNotInvoked(t, s2, "s2")
	nwViewsAssertRows(t, "s2 iterator", s2Snapshot(), [][]any{{"E1", int64(1), nil}, {"E2", int64(1), nil}})

	sendMarket := func(symbol string, volume int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsMarketVol{Symbol: symbol, Volume: volume}); err != nil {
			t.Fatal(err)
		}
	}

	sendMarket("S1", 1)
	if s2.count != 1 || len(s2.newRows) != 2 {
		t.Fatalf("s2 S1 invocation = count %d new %#v", s2.count, s2.newRows)
	}
	nwViewsAssertRowsAnyOrder(t, "s2 S1 new", s2.newRows, [][]any{{"E1", int64(1), "S1"}, {"E2", int64(1), "S1"}})
	s2.reset()
	nwViewsAssertRowsAnyOrder(t, "s2 iterator", s2Snapshot(), [][]any{{"E1", int64(1), "S1"}, {"E2", int64(1), "S1"}})

	sendMarket("S2", 2)
	nwViewsAssertNotInvoked(t, s2, "s2")
	nwViewsAssertRowsAnyOrder(t, "s2 iterator", s2Snapshot(), [][]any{{"E1", int64(1), "S1"}, {"E2", int64(1), "S1"}})

	send("E3", 2)
	nwViewsAssertNew(t, create, "create E3", []any{"E3", int64(2)})
	nwViewsAssertNew(t, s2, "s2 E3", []any{"E3", int64(2), "S2"})
	nwViewsAssertRowsAnyOrder(t, "s2 iterator", s2Snapshot(), [][]any{{"E1", int64(1), "S1"}, {"E2", int64(1), "S1"}, {"E3", int64(2), "S2"}})
}

// nwViewsBeanFull mirrors SupportBean (theString, intPrimitive, longPrimitive,
// boolPrimitive) for the grouped late-start executions.
type nwViewsBeanFull struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
}

// nwViewsGroupedSchema mirrors the (theString, intPrimitive) window schema of
// InfraSelectGroupedViewLateStart.
type nwViewsGroupedSchema struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// nwViewsVariableSet mirrors SupportVariableSetEvent (variableName, value).
type nwViewsVariableSet struct {
	VariableName string `esper:"variableName"`
	Value        string `esper:"value"`
}

// nwViewsGroupedFull mirrors the window-created schema of
// InfraSelectGroupedViewLateStartVariableIterate (theString, intPrimitive,
// longPrimitive, boolPrimitive). Esper derives a distinct event type for the
// create-window select columns, so the Go port registers a distinct struct;
// reusing nwViewsBeanFull would re-point the Go-type -> event-type index and
// misroute SupportBean sends.
type nwViewsGroupedFull struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
}

// TestInfraNWViewsSelectGroupedViewLateStartParity mirrors
// InfraSelectGroupedViewLateStart: a #groupwin(theString, intPrimitive)#length(9)
// window retains every row (no group reaches the per-group limit) and a
// group-by count consumer deployed late replays the retained rows into its
// grouped aggregate state, iterating ten groups in order.
func TestInfraNWViewsSelectGroupedViewLateStartParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[nwViewsBeanFull](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := RegisterStruct[nwViewsGroupedSchema](env, "MyWindowSGVSSchema")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowSGVS", windowSchema, NamedWindowRetention(GroupWindowKeys(
		[]Expr{
			Field[nwViewsGroupedSchema, string]("theString"),
			Field[nwViewsGroupedSchema, int]("intPrimitive"),
		},
		LengthWindow(9),
	))); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[nwViewsBeanFull](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindowSGVS",
		SetColumn("theString", Field[nwViewsBeanFull, string]("theString")),
		SetColumn("intPrimitive", Field[nwViewsBeanFull, int]("intPrimitive")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBeanFull{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	for _, stringValue := range []string{"c0", "c1", "c2"} {
		for j := 0; j < 3; j++ {
			send(stringValue, j)
		}
	}
	send("c0", 1)
	send("c1", 2)
	send("c3", 3)

	window, ok := engine.NamedWindow("MyWindowSGVS")
	if !ok {
		t.Fatal("grouped late-start window is missing")
	}
	if events, err := window.Snapshot(context.Background()); err != nil || len(events) != 12 {
		t.Fatalf("create iterator length = %d err=%v, want 12", len(events), err)
	}

	// select theString, intPrimitive, count(*) from MyWindowSGVS
	//   group by theString, intPrimitive order by theString, intPrimitive
	theString := Field[any, string]("theString")
	intPrimitive := Field[any, int]("intPrimitive")
	s0Plan, err := env.Build(FromNamedWindow(env, "MyWindowSGVS").GroupBy(theString, intPrimitive).Select(
		Alias("theString", theString),
		Alias("intPrimitive", intPrimitive),
		Alias("count", CountAll()),
	).Query(
		StatementName("s0"),
		OrderBy(Ascending(theString), Ascending(intPrimitive)),
	))
	if err != nil {
		t.Fatal(err)
	}
	s0Deployment, err := engine.Deploy(context.Background(), s0Plan)
	if err != nil {
		t.Fatal(err)
	}
	nwViewsAssertRows(t, "s0 iterator", nwViewsStatementSnapshot(t, s0Deployment.Statements()[0], "theString", "intPrimitive", "count"), [][]any{
		{"c0", 0, int64(1)},
		{"c0", 1, int64(2)},
		{"c0", 2, int64(1)},
		{"c1", 0, int64(1)},
		{"c1", 1, int64(1)},
		{"c1", 2, int64(2)},
		{"c2", 0, int64(1)},
		{"c2", 1, int64(1)},
		{"c2", 2, int64(1)},
		{"c3", 3, int64(1)},
	})
}

// TestInfraNWViewsSelectGroupedViewLateStartVariableIterateParity mirrors
// InfraSelectGroupedViewLateStartVariableIterate: a variable-driven having
// clause over the late-started grouped aggregate narrows the iterator to the
// selected theString group each time the variable is set by an on-set trigger.
func TestInfraNWViewsSelectGroupedViewLateStartVariableIterateParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[nwViewsBeanFull](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwViewsVariableSet](env, "SupportVariableSetEvent"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := RegisterStruct[nwViewsGroupedFull](env, "MyWindowSGVLSSchema")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowSGVLS", windowSchema, NamedWindowRetention(GroupWindowKeys(
		[]Expr{
			Field[nwViewsGroupedFull, string]("theString"),
			Field[nwViewsGroupedFull, int]("intPrimitive"),
		},
		LengthWindow(9),
	))); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[nwViewsBeanFull](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindowSGVLS",
		SetColumn("theString", Field[nwViewsBeanFull, string]("theString")),
		SetColumn("intPrimitive", Field[nwViewsBeanFull, int]("intPrimitive")),
		SetColumn("longPrimitive", Field[nwViewsBeanFull, int64]("longPrimitive")),
		SetColumn("boolPrimitive", Field[nwViewsBeanFull, bool]("boolPrimitive")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var_1_1_1", ""); err != nil {
		t.Fatal(err)
	}
	setPlan, err := env.Build(OnEvent(From[nwViewsVariableSet](env, "SupportVariableSetEvent").Filter(
		Equal[string](Field[nwViewsVariableSet, string]("variableName"), Literal[string]("var_1_1_1")),
	)).SetVariables(
		SetVariableExpr("var_1_1_1", Field[nwViewsVariableSet, string]("value")),
	).Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, plan := range []Plan{insertPlan, setPlan} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	send := func(event nwViewsBeanFull) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	setVariable := func(value string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsVariableSet{VariableName: "var_1_1_1", Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	for _, stringValue := range []string{"c0", "c1", "c2"} {
		for j := 0; j < 3; j++ {
			send(nwViewsBeanFull{TheString: stringValue, IntPrimitive: j, LongPrimitive: int64(j), BoolPrimitive: true})
		}
	}
	send(nwViewsBeanFull{TheString: "c1", IntPrimitive: 1, LongPrimitive: 10, BoolPrimitive: true})

	window, ok := engine.NamedWindow("MyWindowSGVLS")
	if !ok {
		t.Fatal("variable-iterate window is missing")
	}
	if events, err := window.Snapshot(context.Background()); err != nil || len(events) != 10 {
		t.Fatalf("create iterator length = %d err=%v, want 10", len(events), err)
	}

	// select theString, intPrimitive, avg(longPrimitive) as avgLong, count(boolPrimitive) as cntBool
	//   from MyWindowSGVLS group by theString, intPrimitive
	//   having theString = var_1_1_1 order by theString, intPrimitive
	theString := Field[any, string]("theString")
	intPrimitive := Field[any, int]("intPrimitive")
	s0Plan, err := env.Build(FromNamedWindow(env, "MyWindowSGVLS").GroupBy(theString, intPrimitive).Select(
		Alias("theString", theString),
		Alias("intPrimitive", intPrimitive),
		Alias("avgLong", Avg[int64](Field[any, int64]("longPrimitive"))),
		Alias("cntBool", Count[bool](Field[any, bool]("boolPrimitive"))),
	).Having(
		Equal[string](theString, VariableRef[string]("var_1_1_1")),
	).Query(
		StatementName("s0"),
		OrderBy(Ascending(theString), Ascending(intPrimitive)),
	))
	if err != nil {
		t.Fatal(err)
	}
	s0Deployment, err := engine.Deploy(context.Background(), s0Plan)
	if err != nil {
		t.Fatal(err)
	}
	s0Snapshot := func() [][]any { return nwViewsStatementSnapshot(t, s0Deployment.Statements()[0], "theString", "intPrimitive", "avgLong", "cntBool") }

	setVariable("c0")
	nwViewsAssertRows(t, "s0 iterator c0", s0Snapshot(), [][]any{
		{"c0", 0, 0.0, int64(1)},
		{"c0", 1, 1.0, int64(1)},
		{"c0", 2, 2.0, int64(1)},
	})

	setVariable("c1")
	nwViewsAssertRows(t, "s0 iterator c1", s0Snapshot(), [][]any{
		{"c1", 0, 0.0, int64(1)},
		{"c1", 1, 5.5, int64(2)},
		{"c1", 2, 2.0, int64(1)},
	})
}

// TestInfraNWViewsPatternParity mirrors InfraPattern: a pattern consuming the
// named window's insert stream, every a=MyWindowPAT(key='S1') or
// a=MyWindowPAT(key='S2'). Esper parses the every onto the first branch only,
// so S1 matches fire repeatedly with isQuitted=false while the single S2
// match completes the or-expression permanently (EvalOrStateNode quits all
// child listeners), after which a later S1 stays silent.
func TestInfraNWViewsPatternParity(t *testing.T) {
	env, engine, _, _, send := nwViewsLateConsumerWindow(t, "MyWindowPAT")

	key := Field[any, string]("key")
	s1 := PatternFromRecord(FromNamedWindow(env, "MyWindowPAT"), "a",
		Equal[string](key, Literal[string]("S1")))
	s2 := PatternFromRecord(FromNamedWindow(env, "MyWindowPAT"), "a",
		Equal[string](key, Literal[string]("S2")))
	s0Plan, err := env.Build(s1.Every().Or(s2).Select(
		Alias("key", TagField[string]("a", "key")),
		Alias("value", TagField[int64]("a", "value")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	s0Deployment, err := engine.Deploy(context.Background(), s0Plan)
	if err != nil {
		t.Fatal(err)
	}
	s0 := &nwViewsProbe{}
	nwViewsSubscribeResults(t, s0Deployment.Statements()[0], []string{"key", "value"}, s0)

	send("E1", 1)
	nwViewsAssertNotInvoked(t, s0, "s0 E1")

	send("S1", 2)
	nwViewsAssertNew(t, s0, "s0 S1(2)", []any{"S1", int64(2)})

	send("S1", 3)
	nwViewsAssertNew(t, s0, "s0 S1(3)", []any{"S1", int64(3)})

	send("S2", 4)
	nwViewsAssertNew(t, s0, "s0 S2(4)", []any{"S2", int64(4)})

	// The S2 completion quitted the whole or-expression, including the every
	// branch; this S1 must stay silent (Java assertListenerNotInvoked).
	send("S1", 1)
	nwViewsAssertNotInvoked(t, s0, "s0 S1(1) after S2 quit")
}

// TestInfraNWViewsOnInsertPreemptiveTwoWindowParity mirrors
// InfraOnInsertPremptiveTwoWindow: one TypeTrigger event fires both
// on-trigger inserts preemptively. The routed OtherStream event is queued
// until every statement has processed the trigger, so the cascaded s0 select
// observes the WinTwo insert that a declaration-order cascade would have
// missed. Java creates OtherStream implicitly from the insert-into select
// columns; Go registers the map schema explicitly, matching the established
// explicit-schema convention.
func TestInfraNWViewsOnInsertPreemptiveTwoWindowParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[nwViewsBeanFull](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	intType := reflect.TypeOf(int(0))
	typeOneSchema, err := RegisterMap(env, "TypeOne", []FieldSpec{FieldDef("col1", intType)})
	if err != nil {
		t.Fatal(err)
	}
	typeTwoSchema, err := RegisterMap(env, "TypeTwo", []FieldSpec{FieldDef("col2", intType)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "TypeTrigger", []FieldSpec{FieldDef("trigger", intType)}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "OtherStream", []FieldSpec{FieldDef("col1", intType)}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "WinOne", typeOneSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "WinTwo", typeTwoSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	// insert into WinOne(col1) select intPrimitive from SupportBean
	insertOnePlan, err := env.Build(OnEvent(From[nwViewsBeanFull](env, "SupportBean")).InsertIntoNamedWindow(
		"WinOne",
		SetColumn("col1", Field[nwViewsBeanFull, int]("intPrimitive")),
	).Query(StatementName("insert-window-one")))
	if err != nil {
		t.Fatal(err)
	}
	// on TypeTrigger insert into OtherStream select col1 from WinOne
	insertOtherStreamPlan, err := env.Build(OnRecord(FromAny(env, "TypeTrigger")).SelectFromNamedWindow(
		"WinOne", nil,
		Alias("col1", NamedWindowField[int]("col1")),
	).Query(RouteTo("OtherStream"), StatementName("insert-otherstream")))
	if err != nil {
		t.Fatal(err)
	}
	// on TypeTrigger insert into WinTwo(col2) select col1 from WinOne
	insertTwoPlan, err := env.Build(OnRecord(FromAny(env, "TypeTrigger")).InsertIntoNamedWindowFrom(
		"WinTwo", "WinOne",
		SetColumn("col2", NamedWindowField[int]("col1")),
	).Query(StatementName("insert-window-two")))
	if err != nil {
		t.Fatal(err)
	}
	// on OtherStream select col2 from WinTwo
	s0Plan, err := env.Build(OnRecord(FromAny(env, "OtherStream")).SelectFromNamedWindow(
		"WinTwo", nil,
		Alias("col2", NamedWindowField[int]("col2")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	for _, plan := range []Plan{insertOnePlan, insertOtherStreamPlan, insertTwoPlan} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	s0Deployment, err := engine.Deploy(context.Background(), s0Plan)
	if err != nil {
		t.Fatal(err)
	}
	s0 := &nwViewsProbe{}
	nwViewsSubscribeResults(t, s0Deployment.Statements()[0], []string{"col2"}, s0)

	// populate WinOne
	if err := engine.SendEvent(context.Background(), nwViewsBeanFull{TheString: "E1", IntPrimitive: 9}); err != nil {
		t.Fatal(err)
	}
	nwViewsAssertNotInvoked(t, s0, "s0 before trigger")

	// fire trigger: insert-otherstream and insert-window-two both run
	// preemptively, so s0 sees col2=9 even though insert-otherstream is
	// declared (and named) before insert-window-two.
	if err := engine.Send(context.Background(), "TypeTrigger", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	nwViewsAssertNew(t, s0, "s0 trigger", []any{9})
}

// TestInfraNWViewsIntersectionParity mirrors InfraIntersection: a window with
// an intersecting view stack (#length(2)#unique(intPrimitive)) retains the
// intersection of the child views; inserting E3 expires E1 through the length
// view and replaces E2 through the unique view, so both leave as old data in
// any order. The delete/update steps are Java-probed Esper 9.0.0 behavior on
// the same window: on-delete removes the window contents from every child
// view, and on-update delivers the original rows as old data and the
// replacement rows as new data while the child views stay consistent for
// later inserts (E6 expires the updated rows through both children).
func TestInfraNWViewsIntersectionParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[nwViewsBeanFull](env, "SupportBean")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowINT", schema, NamedWindowRetention(
		IntersectWindows(LengthWindow(2), Unique(Field[nwViewsBeanFull, int]("intPrimitive"))),
	)); err != nil {
		t.Fatal(err)
	}
	intType := reflect.TypeOf(0)
	if _, err := RegisterMap(env, "TriggerD", []FieldSpec{FieldDef("trigger", intType)}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "TriggerU", []FieldSpec{FieldDef("trigger", intType)}); err != nil {
		t.Fatal(err)
	}

	source := From[nwViewsBeanFull](env, "SupportBean")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow(
		"MyWindowINT",
		SetColumn("theString", Field[nwViewsBeanFull, string]("theString")),
		SetColumn("intPrimitive", Field[nwViewsBeanFull, int]("intPrimitive")),
		SetColumn("longPrimitive", Field[nwViewsBeanFull, int64]("longPrimitive")),
		SetColumn("boolPrimitive", Field[nwViewsBeanFull, bool]("boolPrimitive")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "MyWindowINT").Query(
		StatementName("s0"),
		WithOldStream(),
	))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnRecord(FromAny(env, "TriggerD")).DeleteAllFromNamedWindow(
		"MyWindowINT",
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(OnRecord(FromAny(env, "TriggerU")).UpdateNamedWindow(
		"MyWindowINT",
		Literal(true),
		SetColumn("theString", Literal("UPD")),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	deployments := make([]*Deployment, 0, 3)
	for _, plan := range []Plan{insertPlan, deletePlan, updatePlan} {
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		deployments = append(deployments, deployment)
	}
	probe := &nwViewsProbe{rowOf: func(event Event) []any {
		return []any{event.Get("theString").Any(), event.Get("intPrimitive").Any()}
	}}
	nwViewsSubscribeStatement(t, consumerDeployment.Statements()[0], probe)

	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), nwViewsBeanFull{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	trigger := func(eventType string) {
		t.Helper()
		if err := engine.Send(context.Background(), eventType, map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}

	send("E1", 1)
	nwViewsAssertNew(t, probe, "s0 E1", []any{"E1", 1})
	send("E2", 2)
	nwViewsAssertNew(t, probe, "s0 E2", []any{"E2", 2})
	send("E3", 2)
	if probe.count != 1 {
		t.Fatalf("s0 E3 invoked %d times, want 1 (new=%#v old=%#v)", probe.count, probe.newRows, probe.oldRows)
	}
	nwViewsAssertRows(t, "s0 E3 new", probe.newRows, [][]any{{"E3", 2}})
	nwViewsAssertRowsAnyOrder(t, "s0 E3 old", probe.oldRows, [][]any{{"E1", 1}, {"E2", 2}})
	probe.reset()

	// on-delete empties the window: only the intersected contents leave.
	trigger("TriggerD")
	nwViewsAssertOld(t, probe, "s0 delete", []any{"E3", 2})

	send("E4", 1)
	nwViewsAssertNew(t, probe, "s0 E4", []any{"E4", 1})
	send("E5", 3)
	nwViewsAssertNew(t, probe, "s0 E5", []any{"E5", 3})

	// on-update delivers the original rows as old data and the replacements
	// as new data, keeping the child views consistent.
	trigger("TriggerU")
	if probe.count != 1 {
		t.Fatalf("s0 update invoked %d times, want 1 (new=%#v old=%#v)", probe.count, probe.newRows, probe.oldRows)
	}
	nwViewsAssertRows(t, "s0 update new", probe.newRows, [][]any{{"UPD", 1}, {"UPD", 3}})
	nwViewsAssertRows(t, "s0 update old", probe.oldRows, [][]any{{"E4", 1}, {"E5", 3}})
	probe.reset()

	// E6 shares the unique key with the second updated row; the length view
	// expires the first updated row, so both updated rows leave as old data.
	send("E6", 3)
	if probe.count != 1 {
		t.Fatalf("s0 E6 invoked %d times, want 1 (new=%#v old=%#v)", probe.count, probe.newRows, probe.oldRows)
	}
	nwViewsAssertRows(t, "s0 E6 new", probe.newRows, [][]any{{"E6", 3}})
	nwViewsAssertRowsAnyOrder(t, "s0 E6 old", probe.oldRows, [][]any{{"UPD", 1}, {"UPD", 3}})
	probe.reset()

	for _, deployment := range deployments {
		if err := deployment.Undeploy(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if err := consumerDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// nwViewsOverrideBase mirrors SupportOverrideBase: the only Esper-visible
// property of the hierarchy is val (getVal).
type nwViewsOverrideBase struct {
	Val string `esper:"val"`
}

// nwViewsOverrideOneA mirrors SupportOverrideOneA: getVal() is overridden at
// every level of the Java bean hierarchy (OneA extends One extends Base) and
// the most-derived override returns valOneA, so the single Esper property
// "val" carries the valOneA value. The valOne/valBase constructor state has
// no getter and is invisible to Esper, so the Go struct keeps only the
// Esper-visible surface. Go struct schemas cannot inherit parent fields
// (WithSchemaParent rejects duplicated struct fields; schema inheritance
// covers Map/JSON/ObjectArray), so the deep-supertype window insert is
// expressed with an explicit SetColumn assignment per the typed chain
// convention.
type nwViewsOverrideOneA struct {
	Val string `esper:"val"`
}

// TestInfraNWViewsDeepSupertypeInsertParity mirrors InfraDeepSupertypeInsert:
// create window MyWindowDSI#keepall as select * from SupportOverrideBase
// with insert into MyWindowDSI select * from SupportOverrideOneA; sending
// SupportOverrideOneA("1a", "1", "base") iterates val="1a" because the
// overridden getVal() returns valOneA through virtual dispatch.
func TestInfraNWViewsDeepSupertypeInsertParity(t *testing.T) {
	env := NewEnvironment()
	baseSchema, err := RegisterStruct[nwViewsOverrideBase](env, "SupportOverrideBase")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwViewsOverrideOneA](env, "SupportOverrideOneA"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowDSI", baseSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	source := From[nwViewsOverrideOneA](env, "SupportOverrideOneA")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow(
		"MyWindowDSI",
		SetColumn("val", Field[nwViewsOverrideOneA, string]("val")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("MyWindowDSI")
	if !ok {
		t.Fatal("deep-supertype window is missing")
	}

	// SupportOverrideOneA("1a", "1", "base"): the overridden getVal() returns
	// valOneA ("1a"), the only Esper-visible property value.
	if err := engine.SendEvent(context.Background(), nwViewsOverrideOneA{Val: "1a"}); err != nil {
		t.Fatal(err)
	}
	events, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("window holds %d events, want 1", len(events))
	}
	if got := events[0].Get("val").Any(); got != "1a" {
		t.Fatalf("window val = %#v, want %q", got, "1a")
	}

	if err := insertDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestInfraNWViewsSelectStreamDotStarInsertParity mirrors
// InfraSelectStreamDotStarInsert: an object-array window declared (p0 int)
// with insert into ... select intPrimitive as p0, sb.* as c0 from SupportBean
// as sb. Esper compiles the stream-star column and silently drops it at
// insert (Java-probed Esper 9.0.0: the object-array row holds p0 only). The
// Go typed chain deliberately validates insert columns against the window
// schema, so the undeclared c0 column is a Build-time ErrorUnknownName — an
// approved difference; the expressible p0-only form retains exactly p0,
// matching the Java runtime observation.
func TestInfraNWViewsSelectStreamDotStarInsertParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[nwViewsBeanFull](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := RegisterObjectArray(env, "MyNWWindowObjectArray", []FieldSpec{
		FieldDef("p0", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyNWWindowObjectArray", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	source := From[nwViewsBeanFull](env, "SupportBean")

	// sb.* as c0: the Go chain validates insert columns against the window
	// schema, so the undeclared stream-star column fails at Build time
	// instead of compiling and being silently dropped at insert.
	if _, err := env.Build(OnEvent(source).InsertIntoNamedWindow(
		"MyNWWindowObjectArray",
		SetColumn("p0", Field[nwViewsBeanFull, int]("intPrimitive")),
		SetColumn("c0", Field[nwViewsBeanFull, string]("theString")),
	).Query(StatementName("insert-c0"))); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("stream-star insert error = %v, want ErrorUnknownName", err)
	}

	// The expressible form retains exactly p0, matching the Java probe (the
	// object-array row holds p0 only).
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow(
		"MyNWWindowObjectArray",
		SetColumn("p0", Field[nwViewsBeanFull, int]("intPrimitive")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("MyNWWindowObjectArray")
	if !ok {
		t.Fatal("object-array window is missing")
	}
	if err := engine.SendEvent(context.Background(), nwViewsBeanFull{TheString: "E1", IntPrimitive: 5}); err != nil {
		t.Fatal(err)
	}
	events, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("window holds %d events, want 1", len(events))
	}
	if got := events[0].Get("p0").Any(); got != 5 {
		t.Fatalf("window p0 = %#v, want 5", got)
	}
	if names := events[0].Schema().PropertyNames(); !reflect.DeepEqual(names, []string{"p0"}) {
		t.Fatalf("window event properties = %#v, want [p0]", names)
	}
	if err := insertDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestInfraNWViewsBeanBackedParity mirrors InfraBeanBacked: a bean-backed
// window (create window MyWindowBB#keepall as SupportBean) delivers events
// whose event type carries the window name through the create listener, a
// plain consumer statement and an on-update trigger. Esper runs the
// execution under the object-array/map/default/Avro representation
// annotations, but the representation is inert for create-window-as-bean-type
// (every rep asserts BeanEventType with the SupportBean underlying), so one
// Go pass covers the matrix. Esper's BeanEventType/NAMED_WINDOW type-class
// metadata and the isStatelessSelect SPI flag have no Go public-API
// counterparts; the observable proxies asserted here are the window-named
// event type and the bean underlying on every delivered event.
func TestInfraNWViewsBeanBackedParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[nwViewsBean](env, "SupportBean")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwViewsBeanA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowBB", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	source := From[nwViewsBean](env, "SupportBean")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow(
		"MyWindowBB",
		SetColumn("theString", Field[nwViewsBean, string]("theString")),
		SetColumn("intBoxed", Field[nwViewsBean, int]("intBoxed")),
		SetColumn("longBoxed", Field[nwViewsBean, int64]("longBoxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "MyWindowBB").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(OnEvent(From[nwViewsBeanA](env, "SupportBean_A")).UpdateNamedWindow(
		"MyWindowBB",
		Literal(true),
		SetColumn("theString", Literal("s")),
	).Query(StatementName("update")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	updateDeployment, err := engine.Deploy(context.Background(), updatePlan)
	if err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("MyWindowBB")
	if !ok {
		t.Fatal("bean-backed window is missing")
	}

	// assertEvent(event, "MyWindowBB"): window-named event type with the
	// bean underlying.
	assertWindowEvent := func(label string, event Event) {
		t.Helper()
		if got := event.TypeName(); got != "MyWindowBB" {
			t.Fatalf("%s event type = %q, want %q", label, got, "MyWindowBB")
		}
		if _, isBean := event.Underlying().(nwViewsBean); !isBean {
			t.Fatalf("%s underlying = %T, want nwViewsBean", label, event.Underlying())
		}
	}
	var createNew []Event
	if _, err := window.Subscribe(func(_ context.Context, delta NamedWindowDelta) error {
		createNew = append(createNew, delta.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var consumerNew []Event
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("consumer result is not an event: %#v", result)
			}
			consumerNew = append(consumerNew, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), nwViewsBean{}); err != nil {
		t.Fatal(err)
	}
	if len(createNew) != 1 {
		t.Fatalf("create listener new count = %d, want 1", len(createNew))
	}
	assertWindowEvent("create", createNew[0])
	if len(consumerNew) != 1 {
		t.Fatalf("s0 listener new count = %d, want 1", len(consumerNew))
	}
	assertWindowEvent("s0", consumerNew[0])

	// on SupportBean_A update MyWindowBB set theString='s': the update
	// trigger statement delivers the updated row typed with the window name.
	var updateNew []Event
	if _, err := updateDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("update result is not an event: %#v", result)
			}
			updateNew = append(updateNew, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), nwViewsBeanA{ID: "A1"}); err != nil {
		t.Fatal(err)
	}
	if len(updateNew) != 1 {
		t.Fatalf("update listener new count = %d, want 1", len(updateNew))
	}
	assertWindowEvent("update", updateNew[0])
	if got := updateNew[0].Get("theString").Any(); got != "s" {
		t.Fatalf("update theString = %#v, want %q", got, "s")
	}

	for _, deployment := range []*Deployment{insertDeployment, consumerDeployment, updateDeployment} {
		if err := deployment.Undeploy(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}
