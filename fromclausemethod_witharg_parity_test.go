package esper

import (
	"context"
	"reflect"
	"testing"
)

// fcmIRListener captures the last new/old rows of one irstream statement,
// mirroring Java's assertPropsPerRowIRPair bookkeeping.
type fcmIRListener struct {
	lastNew []Row
	lastOld []Row
	invoked bool
}

func fcmSubscribeIR(t *testing.T, stmt *Statement) *fcmIRListener {
	t.Helper()
	listener := &fcmIRListener{}
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		listener.lastNew = lastNewRows(batch)
		listener.lastOld = make([]Row, 0, len(batch.Old))
		for _, result := range batch.Old {
			if row, ok := result.Row(); ok {
				listener.lastOld = append(listener.lastOld, row)
			}
		}
		listener.invoked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return listener
}

// fcmIRStep sends one event and asserts the irstream pair; a nil expectedNew
// expects no listener invocation at all.
func fcmIRStep(t *testing.T, engine *Engine, listener *fcmIRListener, fields []string, event any, expectedNew, expectedOld [][]string, label string) {
	t.Helper()
	listener.lastNew = nil
	listener.lastOld = nil
	listener.invoked = false
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if expectedNew == nil {
		if listener.invoked {
			t.Fatalf("%s: listener invoked unexpectedly with %#v", label, listener.lastNew)
		}
		return
	}
	if !listener.invoked {
		t.Fatalf("%s: listener not invoked, want %#v", label, expectedNew)
	}
	fcmOuterAssertRows(t, listener.lastNew, fields, expectedNew, label+" new")
	fcmOuterAssertRows(t, listener.lastOld, fields, expectedOld, label+" old")
}

// makeFCMFetchArrayGenProvider mirrors SupportStaticMethodLib.fetchArrayGen:
// a negative argument yields null, zero an empty array, otherwise rows
// id = "A".."A"+numGenerate-1 read from the trigger's intPrimitive.
func makeFCMFetchArrayGenProvider(schema Schema) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		count := request.Trigger.Get("intPrimitive").Any().(int)
		if count <= 0 {
			return nil, nil
		}
		rows := make([]map[string]any, 0, count)
		for i := 0; i < count; i++ {
			rows = append(rows, map[string]any{"id": string(rune('A' + i))})
		}
		return newEventsFrom(schema, rows, request.Now)
	})
}

// makeFCMFetchObjectProvider mirrors SupportStaticMethodLib.fetchObject: a
// null argument yields null (modeled by the empty string), otherwise one row
// id = "|" + theString + "|".
func makeFCMFetchObjectProvider(schema Schema) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		id := request.Trigger.Get("theString").Any().(string)
		if id == "" {
			return nil, nil
		}
		return newEventsFrom(schema, []map[string]any{{"id": "|" + id + "|"}}, request.Now)
	})
}

// TestFromClauseMethodArrayWithArgParity mirrors
// EPLFromClauseMethodArrayWithArg: fetchArrayGen(intPrimitive) joined with a
// length(3) event window in both textual orders, asserting the exact irstream
// pairs as events expire from the window.
func TestFromClauseMethodArrayWithArgParity(t *testing.T) {
	fields := []string{"id", "theString"}

	build := func(t *testing.T, methodFirst bool) (*Environment, Query) {
		env := newFCMEnvironment(t)
		schema := fcmMapSchema(t, env, "FCMArrayGenRow", FieldDef("id", reflect.TypeOf("")))
		stream := From[fcmSupportBean](env, "SupportBean").Window(LengthWindow(3))
		method := FromMethod[map[string]any](env, "arrayGen", schema, makeFCMFetchArrayGenProvider(schema))
		var query Query
		if methodFirst {
			query = Join(method, stream).Select(
				SelectLeft("id", Field[map[string]any, string]("id")),
				SelectRight("theString", Field[fcmSupportBean, string]("theString")),
			).Query(StatementName("fcm-array-witharg-first"), WithOldStream())
		} else {
			query = Join(stream, method).Select(
				SelectRight("id", Field[map[string]any, string]("id")),
				SelectLeft("theString", Field[fcmSupportBean, string]("theString")),
			).Query(StatementName("fcm-array-witharg"), WithOldStream())
		}
		return env, query
	}

	for _, methodFirst := range []bool{false, true} {
		name := "declared"
		if methodFirst {
			name = "method-first"
		}
		t.Run(name, func(t *testing.T) {
			env, query := build(t, methodFirst)
			plan, err := env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			t.Cleanup(func() { _ = engine.Close(context.Background()) })
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			listener := fcmSubscribeIR(t, deployment.Statements()[0])

			send := func(theString string, intPrimitive int) fcmSupportBean {
				return fcmSupportBean{TheString: theString, IntPrimitive: intPrimitive}
			}

			fcmIRStep(t, engine, listener, fields, send("E1", -1), nil, nil, "E1")
			fcmIRStep(t, engine, listener, fields, send("E2", 0), nil, nil, "E2")
			fcmIRStep(t, engine, listener, fields, send("E3", 1), [][]string{{"A", "E3"}}, nil, "E3")
			fcmIRStep(t, engine, listener, fields, send("E4", 2), [][]string{{"A", "E4"}, {"B", "E4"}}, nil, "E4")
			fcmIRStep(t, engine, listener, fields, send("E5", 3), [][]string{{"A", "E5"}, {"B", "E5"}, {"C", "E5"}}, nil, "E5")
			fcmIRStep(t, engine, listener, fields, send("E6", 1), [][]string{{"A", "E6"}}, [][]string{{"A", "E3"}}, "E6")
			fcmIRStep(t, engine, listener, fields, send("E7", 1), [][]string{{"A", "E7"}}, [][]string{{"A", "E4"}, {"B", "E4"}}, "E7")
		})
	}
}

// TestFromClauseMethodObjectWithArgParity mirrors
// EPLFromClauseMethodObjectWithArg: fetchObject(theString) returns a single
// row, and a null argument produces no row and no listener invocation.
func TestFromClauseMethodObjectWithArgParity(t *testing.T) {
	env := newFCMEnvironment(t)
	schema := fcmMapSchema(t, env, "FCMObjectRow", FieldDef("id", reflect.TypeOf("")))
	stream := From[fcmSupportBean](env, "SupportBean").Window(LengthWindow(3))
	method := FromMethod[map[string]any](env, "object", schema, makeFCMFetchObjectProvider(schema))
	query := Join(stream, method).Select(
		SelectRight("id", Field[map[string]any, string]("id")),
		SelectLeft("theString", Field[fcmSupportBean, string]("theString")),
	).Query(StatementName("fcm-object-witharg"), WithOldStream())

	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	listener := fcmSubscribeIR(t, deployment.Statements()[0])
	fields := []string{"id", "theString"}

	fcmIRStep(t, engine, listener, fields, fcmSupportBean{TheString: "E1"}, [][]string{{"|E1|", "E1"}}, nil, "E1")
	fcmIRStep(t, engine, listener, fields, fcmSupportBean{TheString: ""}, nil, nil, "null-string")
	fcmIRStep(t, engine, listener, fields, fcmSupportBean{TheString: "E2"}, [][]string{{"|E2|", "E2"}}, nil, "E2")
}

// TestFromClauseMethod2StreamMaxAggregationParity mirrors
// EPLFromClauseMethod2StreamMaxAggregation: max(col1) over the join of a
// unique(theString) window and the constant 100-row method grid.
func TestFromClauseMethod2StreamMaxAggregationParity(t *testing.T) {
	env := newFCMEnvironment(t)
	schema := fcmMapSchema(t, env, "FCMResult100Row",
		FieldDef("col1", reflect.TypeOf(0)),
		FieldDef("col2", reflect.TypeOf(0)))
	stream := From[fcmSupportBean](env, "SupportBean").Window(Unique(Field[fcmSupportBean, string]("theString")))
	method := FromMethod[map[string]any](env, "result100", schema, makeFCMFetchResult100Provider(schema))
	query := Join(stream, method).Aggregate(
		Alias("maxcol1", Max[int](JoinField[int](1, "col1"))),
	).Query(StatementName("fcm-2stream-max-aggregation"))

	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	listener := fcmSubscribeIR(t, deployment.Statements()[0])
	fields := []string{"maxcol1"}

	send := func() {
		t.Helper()
		if err := engine.SendEvent(context.Background(), fcmSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
			t.Fatal(err)
		}
	}

	send()
	if !listener.invoked {
		t.Fatal("first send: listener not invoked")
	}
	fcmOuterAssertRows(t, listener.lastNew, fields, [][]string{{"9"}}, "first send lastNew")

	// The duplicate-key send matches Java's assertPropsPerRowLastNew either by
	// refiring the same aggregate row or by leaving the previous batch in
	// place; both are observably {9} for the last new data.
	send()
	fcmOuterAssertRows(t, listener.lastNew, fields, [][]string{{"9"}}, "duplicate send lastNew")
}

// TestFromClauseMethodNoJoinIterateVariablesParity mirrors
// EPLFromClauseMethodNoJoinIterateVariables: a single variable-driven method
// source whose rows are iterator-only; the listener never fires.
func TestFromClauseMethodNoJoinIterateVariablesParity(t *testing.T) {
	env := newFCMBoxedEnvironment(t)
	engine := NewEngine(env)
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	fcmDeployLowerUpperSetter(t, env, engine)

	schema := fcmMapSchema(t, env, "FCMBetweenRow", FieldDef("value", reflect.TypeOf(0)))
	method := FromMethod[map[string]any](env, "between", schema, makeFCMFetchBetweenProvider(schema, "lower", "upper"))
	query := Select(method,
		Alias("value", Field[map[string]any, int]("value")),
	).Query(StatementName("fcm-nojoin-iterate-variables"))

	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt := deployment.Statements()[0]
	listener := fcmOuterSubscribe(t, stmt)
	fields := []string{"value"}

	send := func(intPrimitive, intBoxed int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), fcmSupportBeanBoxed{IntPrimitive: intPrimitive, IntBoxed: intBoxed}); err != nil {
			t.Fatal(err)
		}
	}
	assertIterator := func(expected [][]string, label string) {
		t.Helper()
		fcmOuterAssertRows(t, fcmOuterSnapshot(t, stmt), fields, expected, label+" iterator")
		if listener.invoked {
			t.Fatalf("%s: listener invoked unexpectedly with %#v", label, listener.lastNew)
		}
	}

	assertIterator(nil, "initial")

	send(5, 10)
	assertIterator([][]string{{"5"}, {"6"}, {"7"}, {"8"}, {"9"}, {"10"}}, "bean(5,10)")

	send(10, 5)
	assertIterator(nil, "bean(10,5)")

	send(4, 4)
	assertIterator([][]string{{"4"}}, "bean(4,4)")
}
