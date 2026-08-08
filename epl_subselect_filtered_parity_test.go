package esper

import (
	"context"
	"reflect"
	"testing"
)

// subselectFilteredS0 mirrors SupportBean_S0 (id, p00).
type subselectFilteredS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// subselectFilteredS1 mirrors SupportBean_S1 (id, p10, p11).
type subselectFilteredS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
	P11 string `esper:"p11"`
}

// subselectFilteredS2 mirrors SupportBean_S2 (id, p20).
type subselectFilteredS2 struct {
	ID  int    `esper:"id"`
	P20 string `esper:"p20"`
}

// subselectFilteredBean mirrors SupportBean (theString, intPrimitive).
type subselectFilteredBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// subselectFilteredMarketData mirrors SupportMarketDataBean (symbol, price, volume).
type subselectFilteredMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

type subselectFilteredListener struct {
	invoked bool
	lastNew []Row
	lastOld []Row
}

func (l *subselectFilteredListener) reset() {
	l.invoked = false
	l.lastNew = nil
	l.lastOld = nil
}

func newSubselectFilteredEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	registrations := []func() error{
		func() error { _, err := RegisterStruct[subselectFilteredS0](env, "SupportBean_S0"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredS1](env, "SupportBean_S1"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredS2](env, "SupportBean_S2"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredBean](env, "SupportBean"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredMarketData](env, "SupportMarketDataBean"); return err },
	}
	for _, register := range registrations {
		if err := register(); err != nil {
			t.Fatal(err)
		}
	}
	return env
}

func deploySubselectFiltered(t *testing.T, env *Environment, query Query) (*Engine, *subselectFilteredListener) {
	t.Helper()
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	listener := &subselectFilteredListener{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		listener.invoked = true
		listener.lastNew = listener.lastNew[:0]
		listener.lastOld = listener.lastOld[:0]
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("subselect filtered result is not a row: %#v", result)
			}
			listener.lastNew = append(listener.lastNew, row)
		}
		for _, result := range batch.Old {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("subselect filtered old result is not a row: %#v", result)
			}
			listener.lastOld = append(listener.lastOld, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return engine, listener
}

func sendSubselectFiltered(t *testing.T, engine *Engine, event any) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
}

func assertSubselectFilteredField(t *testing.T, listener *subselectFilteredListener, field string, expected any) {
	t.Helper()
	if !listener.invoked {
		t.Fatalf("listener not invoked, expected %s = %v", field, expected)
	}
	if len(listener.lastNew) != 1 {
		t.Fatalf("expected exactly one new row, got %d", len(listener.lastNew))
	}
	value := listener.lastNew[0].Get(field)
	if expected == nil {
		if !value.IsNull() {
			t.Fatalf("expected %s = null, got %#v", field, value.Any())
		}
		return
	}
	if !reflect.DeepEqual(value.Any(), expected) {
		t.Fatalf("expected %s = %#v, got %#v", field, expected, value.Any())
	}
}

// TestSubselectFilteredHavingNoAggNoFilterNoWhereParity mirrors
// EPLSubselectHavingNoAggNoFilterNoWhere: a non-aggregated having filters
// subquery rows after the (absent) where clause.
func TestSubselectFilteredHavingNoAggNoFilterNoWhereParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[subselectFilteredBean](env, "SupportBean").Window(KeepAll()).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("c0", SubqueryValueWithOptions[int](
			inner,
			Field[any, int]("intPrimitive"),
			SubqueryHaving(Equal[string](Field[any, string]("theString"), Literal("ID1"))),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "c0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "ID2", IntPrimitive: 10})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "c0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "ID1", IntPrimitive: 11})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "c0", 11)
}

// TestSubselectFilteredHavingNoAggWWhereParity mirrors EPLSubselectHavingNoAggWWhere:
// where and non-aggregated having compose as consecutive row filters.
func TestSubselectFilteredHavingNoAggWWhereParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[subselectFilteredBean](env, "SupportBean").Window(KeepAll()).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("c0", SubqueryValueWithOptions[int](
			inner,
			Field[any, int]("intPrimitive"),
			SubqueryWhere(Greater[int](Field[any, int]("intPrimitive"), Literal(15))),
			SubqueryHaving(Equal[string](Field[any, string]("theString"), Literal("ID1"))),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "c0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "ID2", IntPrimitive: 10})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "c0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "ID1", IntPrimitive: 11})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "c0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "ID1", IntPrimitive: 20})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "c0", 20)
}

// TestSubselectFilteredHavingNoAggWFilterWWhereParity mirrors
// EPLSubselectHavingNoAggWFilterWWhere: stream filter, where and having all
// compose on the subquery source.
func TestSubselectFilteredHavingNoAggWFilterWWhereParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[subselectFilteredBean](env, "SupportBean").
		Filter(Less[int](Field[subselectFilteredBean, int]("intPrimitive"), Literal(20))).
		Window(KeepAll()).
		AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("c0", SubqueryValueWithOptions[int](
			inner,
			Field[any, int]("intPrimitive"),
			SubqueryWhere(Greater[int](Field[any, int]("intPrimitive"), Literal(15))),
			SubqueryHaving(Equal[string](Field[any, string]("theString"), Literal("ID1"))),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "c0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "ID2", IntPrimitive: 10})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "c0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "ID1", IntPrimitive: 11})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "c0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "ID1", IntPrimitive: 20})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "c0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "ID1", IntPrimitive: 19})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "c0", 19)
}

// TestSubselectFilteredSelectWithWhereJoinedParity mirrors
// EPLSubselectSelectWithWhereJoined: correlated scalar subquery keyed by the
// outer event property.
func TestSubselectFilteredSelectWithWhereJoinedParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("ids1", SubqueryValue[int](
			inner,
			Field[any, int]("id"),
			Equal[string](Field[any, string]("p10"), OuterField[string]("p00")),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "ids1", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 1, P10: "X"})
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 2, P10: "Y"})
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 3, P10: "Z"})

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "ids1", nil)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0, P00: "X"})
	assertSubselectFilteredField(t, listener, "ids1", 1)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0, P00: "Y"})
	assertSubselectFilteredField(t, listener, "ids1", 2)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0, P00: "Z"})
	assertSubselectFilteredField(t, listener, "ids1", 3)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0, P00: "A"})
	assertSubselectFilteredField(t, listener, "ids1", nil)
}

// TestSubselectFilteredWhereConstantParity mirrors EPLSubselectWhereConstant:
// constant filters (single-column, two-column and range) on the subquery.
func TestSubselectFilteredWhereConstantParity(t *testing.T) {
	t.Run("single-column constant", func(t *testing.T) {
		env := newSubselectFilteredEnvironment(t)
		inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
		query := Select(
			From[subselectFilteredS0](env, "SupportBean_S0"),
			Alias("ids1", SubqueryValueWithOptions[int](
				inner,
				Field[any, int]("id"),
				SubqueryWhere(Equal[string](Field[any, string]("p10"), Literal("X"))),
				SubqueryCardinalityMode(SubqueryNullOnMultiple),
			)),
		).Query(StatementName("s0"))
		engine, listener := deploySubselectFiltered(t, env, query)

		sendSubselectFiltered(t, engine, subselectFilteredS1{ID: -1, P10: "Y"})
		sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
		assertSubselectFilteredField(t, listener, "ids1", nil)

		sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 1, P10: "X"})
		sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 2, P10: "Y"})
		sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 3, P10: "Z"})

		listener.reset()
		sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
		assertSubselectFilteredField(t, listener, "ids1", 1)

		listener.reset()
		sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
		assertSubselectFilteredField(t, listener, "ids1", 1)

		sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 2, P10: "X"})
		listener.reset()
		sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
		assertSubselectFilteredField(t, listener, "ids1", nil)
	})

	t.Run("two-column constant", func(t *testing.T) {
		env := newSubselectFilteredEnvironment(t)
		inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
		query := Select(
			From[subselectFilteredS0](env, "SupportBean_S0"),
			Alias("ids1", SubqueryValue[int](
				inner,
				Field[any, int]("id"),
				And(
					Equal[string](Field[any, string]("p10"), Literal("X")),
					Equal[string](Field[any, string]("p11"), Literal("Y")),
				),
			)),
		).Query(StatementName("s0"))
		engine, listener := deploySubselectFiltered(t, env, query)

		sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 1, P10: "X", P11: "Y"})
		sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
		assertSubselectFilteredField(t, listener, "ids1", 1)
	})

	t.Run("single range", func(t *testing.T) {
		env := newSubselectFilteredEnvironment(t)
		inner := From[subselectFilteredBean](env, "SupportBean").Window(LastEvent()).AsRecord()
		query := Select(
			From[subselectFilteredS0](env, "SupportBean_S0"),
			Alias("ids1", SubqueryValue[string](
				inner,
				Field[any, string]("theString"),
				Between[int](Field[any, int]("intPrimitive"), Literal(10), Literal(20)),
			)),
		).Query(StatementName("s0"))
		engine, listener := deploySubselectFiltered(t, env, query)

		sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "E1", IntPrimitive: 15})
		sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
		assertSubselectFilteredField(t, listener, "ids1", "E1")
	})
}

// TestSubselectFilteredWherePreviousParity mirrors EPLSubselectWherePrevious
// (and its object-model/compile variants, which share runtime semantics): prev
// evaluates against the matching row position inside the subquery window.
func TestSubselectFilteredWherePreviousParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("value", SubqueryValue[int](
			inner,
			Prev[int](1, Field[any, int]("id")),
			Equal[int](Field[any, int]("id"), OuterField[int]("id")),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 1})
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "value", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 2})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	assertSubselectFilteredField(t, listener, "value", 1)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 3})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 3})
	assertSubselectFilteredField(t, listener, "value", 2)
}

// TestSubselectFilteredSameEventParity mirrors EPLSubselectSameEvent (and its
// object-model/compile variants): a wildcard scalar subselect over the same
// stream returns the triggering event itself.
func TestSubselectFilteredSameEventParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	query := Select(
		From[subselectFilteredS1](env, "SupportBean_S1"),
		Alias("events1", SubqueryValue[subselectFilteredS1](inner, EventValue[subselectFilteredS1]())),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: -1, P10: "Y"})
	assertSubselectFilteredField(t, listener, "events1", subselectFilteredS1{ID: -1, P10: "Y"})
}

// TestSubselectFilteredSelectWildcardParity mirrors EPLSubselectSelectWildcard:
// a wildcard scalar subselect returns the last inner event payload.
func TestSubselectFilteredSelectWildcardParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("events1", SubqueryValue[subselectFilteredS1](inner, EventValue[subselectFilteredS1]())),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: -1, P10: "Y"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "events1", subselectFilteredS1{ID: -1, P10: "Y"})
}

// TestSubselectFilteredSelectWithWhere2SubqueryParity mirrors
// EPLSubselectSelectWithWhere2Subqery: two scalar subqueries compare against
// the outer id inside the outer where clause.
func TestSubselectFilteredSelectWithWhere2SubqueryParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	innerS1 := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	innerS2 := From[subselectFilteredS2](env, "SupportBean_S2").Window(LengthWindow(1000)).AsRecord()
	id := Field[subselectFilteredS0, int]("id")
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0").Filter(Or(
			Equal[int](id, SubqueryValue[int](
				innerS1,
				Field[any, int]("id"),
				Equal[int](Field[any, int]("id"), OuterField[int]("id")),
			)),
			Equal[int](id, SubqueryValue[int](
				innerS2,
				Field[any, int]("id"),
				Equal[int](Field[any, int]("id"), OuterField[int]("id")),
			)),
		)),
		Alias("id", id),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	if listener.invoked {
		t.Fatal("listener must not be invoked for unmatched S0(0)")
	}

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 1})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertSubselectFilteredField(t, listener, "id", 1)

	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 2})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	assertSubselectFilteredField(t, listener, "id", 2)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 3})
	if listener.invoked {
		t.Fatal("listener must not be invoked for unmatched S0(3)")
	}

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 3})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 3})
	assertSubselectFilteredField(t, listener, "id", 3)
}

// TestSubselectFilteredSceneOneParity mirrors EPLSubselectSelectSceneOne:
// irstream output with a correlated subquery over a windowed, filtered outer
// stream.
func TestSubselectFilteredSceneOneParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	symbol := Field[subselectFilteredMarketData, string]("symbol")
	outer := From[subselectFilteredMarketData](env, "SupportMarketDataBean").
		Filter(Equal[string](symbol, Literal("S0"))).
		Window(LengthWindow(2))
	inner := From[subselectFilteredMarketData](env, "SupportMarketDataBean").
		Filter(Equal[string](symbol, Literal("S1"))).
		Window(LengthWindow(10)).
		AsRecord()
	query := Select(
		outer,
		Alias("s0price", Field[subselectFilteredMarketData, float64]("price")),
		Alias("s1price", SubqueryValue[float64](
			inner,
			Field[any, float64]("price"),
			Equal[int64](Field[any, int64]("volume"), OuterField[int64]("volume")),
		)),
	).Query(StatementName("s0"), WithOldStream())
	engine, listener := deploySubselectFiltered(t, env, query)

	sendMarketData := func(symbol string, price float64, volume int64) {
		t.Helper()
		sendSubselectFiltered(t, engine, subselectFilteredMarketData{Symbol: symbol, Price: price, Volume: volume})
	}
	assertPair := func(label string, expectedS0 float64, expectedS1 any, expectedOld int) {
		t.Helper()
		if !listener.invoked {
			t.Fatalf("%s: listener not invoked", label)
		}
		if len(listener.lastNew) != 1 {
			t.Fatalf("%s: expected one new row, got %d", label, len(listener.lastNew))
		}
		if got := listener.lastNew[0].Get("s0price").Any(); !reflect.DeepEqual(got, expectedS0) {
			t.Fatalf("%s: s0price = %#v, want %#v", label, got, expectedS0)
		}
		s1 := listener.lastNew[0].Get("s1price")
		if expectedS1 == nil {
			if !s1.IsNull() {
				t.Fatalf("%s: s1price = %#v, want null", label, s1.Any())
			}
		} else if !reflect.DeepEqual(s1.Any(), expectedS1) {
			t.Fatalf("%s: s1price = %#v, want %#v", label, s1.Any(), expectedS1)
		}
		if len(listener.lastOld) != expectedOld {
			t.Fatalf("%s: expected %d old rows, got %d", label, expectedOld, len(listener.lastOld))
		}
	}

	sendMarketData("S0", 100, 1)
	assertPair("first S0", 100.0, nil, 0)

	listener.reset()
	sendMarketData("S1", -10, 2)
	if listener.invoked {
		t.Fatal("S1 event must not trigger the S0-filtered statement")
	}

	listener.reset()
	sendMarketData("S0", 200, 2)
	assertPair("second S0", 200.0, -10.0, 0)

	listener.reset()
	sendMarketData("S1", -20, 3)
	if listener.invoked {
		t.Fatal("S1 event must not trigger the S0-filtered statement")
	}

	listener.reset()
	sendMarketData("S0", 300, 3)
	assertPair("third S0", 300.0, -20.0, 1)
	if old := listener.lastOld[0].Get("s0price").Any(); !reflect.DeepEqual(old, 100.0) {
		t.Fatalf("old row s0price = %#v, want 100", old)
	}
	if oldS1 := listener.lastOld[0].Get("s1price"); !oldS1.IsNull() {
		t.Fatalf("old row s1price = %#v, want null", oldS1.Any())
	}
}
