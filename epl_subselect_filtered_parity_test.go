package esper

import (
	"context"
	"fmt"
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

// subselectFilteredS3 mirrors SupportBean_S3 (id, p30).
type subselectFilteredS3 struct {
	ID  int    `esper:"id"`
	P30 string `esper:"p30"`
}

// subselectFilteredBean mirrors SupportBean (theString, intPrimitive and the
// boxed numeric properties used by the coercion scenarios).
type subselectFilteredBean struct {
	TheString     string  `esper:"theString"`
	IntPrimitive  int     `esper:"intPrimitive"`
	IntBoxed      int     `esper:"intBoxed"`
	LongBoxed     int64   `esper:"longBoxed"`
	DoubleBoxed   float64 `esper:"doubleBoxed"`
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
	allRows []Row
}

func (l *subselectFilteredListener) reset() {
	l.invoked = false
	l.lastNew = nil
	l.lastOld = nil
}

func (l *subselectFilteredListener) resetAll() {
	l.reset()
	l.allRows = nil
}

func newSubselectFilteredEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	registrations := []func() error{
		func() error { _, err := RegisterStruct[subselectFilteredS0](env, "SupportBean_S0"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredS1](env, "SupportBean_S1"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredS2](env, "SupportBean_S2"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredS3](env, "SupportBean_S3"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredBean](env, "SupportBean"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredMarketData](env, "SupportMarketDataBean"); return err },
		func() error { _, err := RegisterStruct[fcmEventWithManyArray](env, "SupportEventWithManyArray"); return err },
		func() error { _, err := RegisterStruct[rowRecogMultikeyArrayEvent](env, "SupportEventWithIntArray"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredSensorEvent](env, "SupportSensorEvent"); return err },
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
			listener.allRows = append(listener.allRows, row)
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

// TestSubselectFilteredMultikeyWArrayPrimitiveParity mirrors
// EPLSubselectWhereClauseMultikeyWArrayPrimitive: int-array content equality
// as the subquery correlation key. A null array matches a null array and
// multiple matches yield null.
func TestSubselectFilteredMultikeyWArrayPrimitiveParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[fcmEventWithManyArray](env, "SupportEventWithManyArray").Window(KeepAll()).AsRecord()
	query := Select(
		From[rowRecogMultikeyArrayEvent](env, "SupportEventWithIntArray"),
		Alias("value", SubqueryValueWithOptions[string](
			inner,
			Field[any, string]("id"),
			SubqueryWhere(Is(Field[any, any]("intOne"), OuterField[any]("array"))),
			SubqueryCardinalityMode(SubqueryNullOnMultiple),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendManyArray := func(id string, ints []int) {
		t.Helper()
		sendSubselectFiltered(t, engine, fcmEventWithManyArray{ID: id, IntOne: ints})
	}
	assertIntArray := func(id string, array []int, expected any) {
		t.Helper()
		listener.reset()
		sendSubselectFiltered(t, engine, rowRecogMultikeyArrayEvent{ID: id, Array: array})
		assertSubselectFilteredField(t, listener, "value", expected)
	}

	sendManyArray("MA1", []int{1, 2})
	assertIntArray("IA1", []int{1, 2}, "MA1")

	sendManyArray("MA2", []int{1, 2})
	sendManyArray("MA3", []int{1})
	sendManyArray("MA4", []int{})
	sendManyArray("MA5", nil)

	assertIntArray("IA2", []int{}, "MA4")
	assertIntArray("IA3", []int{1}, "MA3")
	assertIntArray("IA4", nil, "MA5")
	assertIntArray("IA5", []int{1, 2}, nil)
}

// TestSubselectFilteredMultikeyWArray2FieldParity mirrors
// EPLSubselectWhereClauseMultikeyWArray2Field: array equality composed with a
// scalar equality correlation.
func TestSubselectFilteredMultikeyWArray2FieldParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[fcmEventWithManyArray](env, "SupportEventWithManyArray").Window(KeepAll()).AsRecord()
	query := Select(
		From[rowRecogMultikeyArrayEvent](env, "SupportEventWithIntArray"),
		Alias("value", SubqueryValueWithOptions[string](
			inner,
			Field[any, string]("id"),
			SubqueryWhere(And(
				Is(Field[any, any]("intOne"), OuterField[any]("array")),
				Equal[int](Field[any, int]("value"), OuterField[int]("value")),
			)),
			SubqueryCardinalityMode(SubqueryNullOnMultiple),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendManyArray := func(id string, ints []int, value int) {
		t.Helper()
		sendSubselectFiltered(t, engine, fcmEventWithManyArray{ID: id, IntOne: ints, Value: value})
	}
	assertIntArray := func(id string, array []int, value int, expected any) {
		t.Helper()
		listener.reset()
		sendSubselectFiltered(t, engine, rowRecogMultikeyArrayEvent{ID: id, Array: array, Value: value})
		assertSubselectFilteredField(t, listener, "value", expected)
	}

	sendManyArray("MA1", []int{1, 2}, 10)
	sendManyArray("MA2", []int{1, 2}, 11)
	sendManyArray("MA3", []int{1}, 12)

	assertIntArray("IA1", []int{1}, 12, "MA3")
	assertIntArray("IA2", []int{1, 2}, 11, "MA2")
	assertIntArray("IA3", []int{1, 2}, 10, "MA1")
	assertIntArray("IA4", []int{1}, 10, nil)
	assertIntArray("IA5", []int{1, 2}, 12, nil)
}

// TestSubselectFilteredMultikeyWArrayCompositeParity mirrors
// EPLSubselectWhereClauseMultikeyWArrayComposite: array equality composed
// with a range correlation.
func TestSubselectFilteredMultikeyWArrayCompositeParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[fcmEventWithManyArray](env, "SupportEventWithManyArray").Window(KeepAll()).AsRecord()
	query := Select(
		From[rowRecogMultikeyArrayEvent](env, "SupportEventWithIntArray"),
		Alias("value", SubqueryValueWithOptions[string](
			inner,
			Field[any, string]("id"),
			SubqueryWhere(And(
				Is(Field[any, any]("intOne"), OuterField[any]("array")),
				Greater[int](Field[any, int]("value"), OuterField[int]("value")),
			)),
			SubqueryCardinalityMode(SubqueryNullOnMultiple),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendManyArray := func(id string, ints []int, value int) {
		t.Helper()
		sendSubselectFiltered(t, engine, fcmEventWithManyArray{ID: id, IntOne: ints, Value: value})
	}
	assertIntArray := func(id string, array []int, value int, expected any) {
		t.Helper()
		listener.reset()
		sendSubselectFiltered(t, engine, rowRecogMultikeyArrayEvent{ID: id, Array: array, Value: value})
		assertSubselectFilteredField(t, listener, "value", expected)
	}

	sendManyArray("MA1", []int{1, 2}, 100)
	sendManyArray("MA2", []int{1, 2}, 200)
	sendManyArray("MA3", []int{1}, 300)
	sendManyArray("MA4", []int{1, 2}, 400)

	assertIntArray("IA2", []int{1, 2}, 250, "MA4")
	assertIntArray("IA3", []int{1, 2}, 0, nil)
	assertIntArray("IA4", []int{1}, 299, "MA3")
		assertIntArray("IA5", []int{1, 2}, 500, nil)
}

// subselectFilteredSensorEvent mirrors SupportSensorEvent.
type subselectFilteredSensorEvent struct {
	ID          int     `esper:"id"`
	Type        string  `esper:"type"`
	Device      string  `esper:"device"`
	Measurement float64 `esper:"measurement"`
	Confidence  float64 `esper:"confidence"`
}

// assertSubselectFilteredJoinFilteredRow mirrors the tryJoinFiltered
// assertions shared by EPLSubselectJoinFilteredOne/Two.
func assertSubselectFilteredJoinFilteredRow(t *testing.T, listener *subselectFilteredListener, s0id, s1id int, s2p20, s2p20Prior, s2p20Prev any) {
	t.Helper()
	if !listener.invoked {
		t.Fatal("listener not invoked for join-filtered row")
	}
	if len(listener.lastNew) != 1 {
		t.Fatalf("expected one new row, got %d", len(listener.lastNew))
	}
	row := listener.lastNew[0]
	for field, expected := range map[string]any{"s0id": s0id, "s1id": s1id, "s2p20": s2p20, "s2p20Prior": s2p20Prior, "s2p20Prev": s2p20Prev} {
		value := row.Get(field)
		if expected == nil {
			if !value.IsNull() {
				t.Fatalf("expected %s = null, got %#v", field, value.Any())
			}
			continue
		}
		if !reflect.DeepEqual(value.Any(), expected) {
			t.Fatalf("expected %s = %#v, got %#v", field, expected, value.Any())
		}
	}
}

func runSubselectFilteredJoinFilteredScenario(t *testing.T, engine *Engine, listener *subselectFilteredListener) {
	t.Helper()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0, P00: "X"})
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 0, P10: "Y"})
	if listener.invoked {
		t.Fatal("listener must not be invoked while the subquery where has no match")
	}

	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 1, P20: "ab"})
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1, P00: "a"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 1, P10: "b"})
	assertSubselectFilteredJoinFilteredRow(t, listener, 1, 1, "ab", nil, nil)

	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 2, P20: "qx"})
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2, P00: "q"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 2, P10: "x"})
	assertSubselectFilteredJoinFilteredRow(t, listener, 2, 2, "qx", "ab", "ab")
}

// TestSubselectFilteredJoinFilteredOneParity mirrors EPLSubselectJoinFilteredOne:
// scalar/prior/prev subqueries in the select clause plus a scalar subquery
// compared against a concat inside the join where clause.
func TestSubselectFilteredJoinFilteredOneParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	innerLong := From[subselectFilteredS2](env, "SupportBean_S2").Window(LengthWindow(1000)).AsRecord()
	correlated := Equal[int](Field[any, int]("id"), JoinField[int](0, "id"))
	query := Join(
		From[subselectFilteredS0](env, "SupportBean_S0").Window(KeepAll()),
		From[subselectFilteredS1](env, "SupportBean_S1").Window(KeepAll()),
		OnEqual(Field[subselectFilteredS0, int]("id"), Field[subselectFilteredS1, int]("id")),
	).Select(
		SelectLeft("s0id", JoinField[int](0, "id")),
		SelectRight("s1id", JoinField[int](1, "id")),
		SelectLeft("s2p20", SubqueryValue[string](innerLong, Field[any, string]("p20"), correlated)),
		SelectLeft("s2p20Prior", SubqueryValue[string](innerLong, Prior[string](0, Field[any, string]("p20")), correlated)),
		SelectLeft("s2p20Prev", SubqueryValue[string](
			From[subselectFilteredS2](env, "SupportBean_S2").Window(LengthWindow(10)).AsRecord(),
			Prev[string](1, Field[any, string]("p20")),
			correlated,
		)),
	).Where(EqualOf(
		Concat(JoinField[string](0, "p00"), JoinField[string](1, "p10")),
		SubqueryValue[string](innerLong, Field[any, string]("p20"), correlated),
	)).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)
	runSubselectFilteredJoinFilteredScenario(t, engine, listener)
}

// TestSubselectFilteredJoinFilteredTwoParity mirrors EPLSubselectJoinFilteredTwo:
// the join where clause is itself a scalar subquery whose projection compares
// the outer concat against the inner property.
func TestSubselectFilteredJoinFilteredTwoParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	innerLong := From[subselectFilteredS2](env, "SupportBean_S2").Window(LengthWindow(1000)).AsRecord()
	correlated := Equal[int](Field[any, int]("id"), JoinField[int](0, "id"))
	query := Join(
		From[subselectFilteredS0](env, "SupportBean_S0").Window(KeepAll()),
		From[subselectFilteredS1](env, "SupportBean_S1").Window(KeepAll()),
		OnEqual(Field[subselectFilteredS0, int]("id"), Field[subselectFilteredS1, int]("id")),
	).Select(
		SelectLeft("s0id", JoinField[int](0, "id")),
		SelectRight("s1id", JoinField[int](1, "id")),
		SelectLeft("s2p20", SubqueryValue[string](innerLong, Field[any, string]("p20"), correlated)),
		SelectLeft("s2p20Prior", SubqueryValue[string](innerLong, Prior[string](0, Field[any, string]("p20")), correlated)),
		SelectLeft("s2p20Prev", SubqueryValue[string](
			From[subselectFilteredS2](env, "SupportBean_S2").Window(LengthWindow(10)).AsRecord(),
			Prev[string](1, Field[any, string]("p20")),
			correlated,
		)),
	).Where(SubqueryValue[bool](
		innerLong,
		EqualOf(
			Concat(JoinField[string](0, "p00"), JoinField[string](1, "p10")),
			Field[any, string]("p20"),
		),
		correlated,
	)).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)
	runSubselectFilteredJoinFilteredScenario(t, engine, listener)
}

// TestSubselectFilteredSubselectMixMaxParity mirrors EPLSubselectSubselectMixMax:
// sort-window wildcard subselects return the highest/lowest event payloads.
func TestSubselectFilteredSubselectMixMaxParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	measurement := Field[subselectFilteredSensorEvent, float64]("measurement")
	highInner := From[subselectFilteredSensorEvent](env, "SupportSensorEvent").
		Window(SortWindow(1, Descending(measurement))).
		AsRecord()
	lowInner := From[subselectFilteredSensorEvent](env, "SupportSensorEvent").
		Window(SortWindow(1, Ascending(measurement))).
		AsRecord()
	query := Select(
		From[subselectFilteredSensorEvent](env, "SupportSensorEvent"),
		Alias("high", SubqueryValue[subselectFilteredSensorEvent](highInner, EventValue[subselectFilteredSensorEvent]())),
		Alias("low", SubqueryValue[subselectFilteredSensorEvent](lowInner, EventValue[subselectFilteredSensorEvent]())),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	assertHighLow := func(expectedHigh, expectedLow float64) {
		t.Helper()
		if !listener.invoked || len(listener.lastNew) != 1 {
			t.Fatalf("expected one new row, listener=%+v", listener)
		}
		high, ok := listener.lastNew[0].Get("high").Any().(subselectFilteredSensorEvent)
		if !ok {
			t.Fatalf("high is not a sensor event: %#v", listener.lastNew[0].Get("high").Any())
		}
		low, ok := listener.lastNew[0].Get("low").Any().(subselectFilteredSensorEvent)
		if !ok {
			t.Fatalf("low is not a sensor event: %#v", listener.lastNew[0].Get("low").Any())
		}
		if high.Measurement != expectedHigh || low.Measurement != expectedLow {
			t.Fatalf("high/low = %v/%v, want %v/%v", high.Measurement, low.Measurement, expectedHigh, expectedLow)
		}
	}

	sendSubselectFiltered(t, engine, subselectFilteredSensorEvent{ID: 1, Type: "Temp", Device: "Dev1", Measurement: 68.0, Confidence: 96.5})
	assertHighLow(68, 68)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredSensorEvent{ID: 2, Type: "Temp", Device: "Dev2", Measurement: 70.0, Confidence: 98.5})
	assertHighLow(70, 68)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredSensorEvent{ID: 3, Type: "Temp", Device: "Dev2", Measurement: 65.0, Confidence: 99.5})
	assertHighLow(70, 65)
}


// TestSubselectFilteredSelectWhereJoined2StreamsParity mirrors
// EPLSubselectSelectWhereJoined2Streams: the subquery correlates to both
// streams of the outer two-stream join.
func TestSubselectFilteredSelectWhereJoined2StreamsParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[subselectFilteredS0](env, "SupportBean_S0").Window(LengthWindow(1000)).AsRecord()
	query := Join(
		From[subselectFilteredS1](env, "SupportBean_S1").Window(KeepAll()),
		From[subselectFilteredS2](env, "SupportBean_S2").Window(KeepAll()),
		OnEqual(Field[subselectFilteredS1, int]("id"), Field[subselectFilteredS2, int]("id")),
	).Select(
		SelectLeft("ids0", SubqueryValue[int](
			inner,
			Field[any, int]("id"),
			And(
				Equal[string](Field[any, string]("p00"), JoinField[string](0, "p10")),
				Equal[string](Field[any, string]("p00"), JoinField[string](1, "p20")),
			),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10, P10: "s0_1"})
	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 10, P20: "s0_1"})
	assertSubselectFilteredField(t, listener, "ids0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 99, P00: "s0_1"})
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 11, P10: "s0_1"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 11, P20: "s0_1"})
	assertSubselectFilteredField(t, listener, "ids0", 99)
}

// TestSubselectFilteredSelectWhereJoined3StreamsParity mirrors
// EPLSubselectSelectWhereJoined3Streams: the subquery correlates to two of the
// three outer join streams.
func TestSubselectFilteredSelectWhereJoined3StreamsParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[subselectFilteredS0](env, "SupportBean_S0").Window(LengthWindow(1000)).AsRecord()
	query := JoinMany(
		JoinSource(From[subselectFilteredS1](env, "SupportBean_S1").Window(KeepAll())),
		JoinSource(From[subselectFilteredS2](env, "SupportBean_S2").Window(KeepAll())),
		JoinSource(From[subselectFilteredS3](env, "SupportBean_S3").Window(KeepAll())),
	).On(
		OnSourcesEqual(0, Field[subselectFilteredS1, int]("id"), 1, Field[subselectFilteredS2, int]("id")),
		OnSourcesEqual(1, Field[subselectFilteredS2, int]("id"), 2, Field[subselectFilteredS3, int]("id")),
	).Select(
		SelectLeft("ids0", SubqueryValue[int](
			inner,
			Field[any, int]("id"),
			And(
				Equal[string](Field[any, string]("p00"), JoinField[string](0, "p10")),
				Equal[string](Field[any, string]("p00"), JoinField[string](2, "p30")),
			),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10, P10: "s0_1"})
	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 10, P20: "s0_1"})
	sendSubselectFiltered(t, engine, subselectFilteredS3{ID: 10, P30: "s0_1"})
	assertSubselectFilteredField(t, listener, "ids0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 99, P00: "s0_1"})
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 11, P10: "s0_1"})
	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 11, P20: "xxx"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS3{ID: 11, P30: "s0_1"})
	assertSubselectFilteredField(t, listener, "ids0", 99)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 98, P00: "s0_2"})
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 12, P10: "s0_x"})
	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 12, P20: "s0_2"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS3{ID: 12, P30: "s0_1"})
	assertSubselectFilteredField(t, listener, "ids0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 13, P10: "s0_2"})
	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 13, P20: "s0_2"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS3{ID: 13, P30: "s0_x"})
	assertSubselectFilteredField(t, listener, "ids0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 14, P10: "s0_2"})
	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 14, P20: "xx"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS3{ID: 14, P30: "s0_2"})
	assertSubselectFilteredField(t, listener, "ids0", 98)
}

// TestSubselectFilteredSelectWhereJoined3SceneTwoParity mirrors
// EPLSubselectSelectWhereJoined3SceneTwo: the subquery correlates to all three
// outer join streams.
func TestSubselectFilteredSelectWhereJoined3SceneTwoParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	inner := From[subselectFilteredS0](env, "SupportBean_S0").Window(LengthWindow(1000)).AsRecord()
	query := JoinMany(
		JoinSource(From[subselectFilteredS1](env, "SupportBean_S1").Window(KeepAll())),
		JoinSource(From[subselectFilteredS2](env, "SupportBean_S2").Window(KeepAll())),
		JoinSource(From[subselectFilteredS3](env, "SupportBean_S3").Window(KeepAll())),
	).On(
		OnSourcesEqual(0, Field[subselectFilteredS1, int]("id"), 1, Field[subselectFilteredS2, int]("id")),
		OnSourcesEqual(1, Field[subselectFilteredS2, int]("id"), 2, Field[subselectFilteredS3, int]("id")),
	).Select(
		SelectLeft("ids0", SubqueryValue[int](
			inner,
			Field[any, int]("id"),
			And(
				Equal[string](Field[any, string]("p00"), JoinField[string](0, "p10")),
				And(
					Equal[string](Field[any, string]("p00"), JoinField[string](2, "p30")),
					Equal[string](Field[any, string]("p00"), JoinField[string](1, "p20")),
				),
			),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10, P10: "s0_1"})
	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 10, P20: "s0_1"})
	sendSubselectFiltered(t, engine, subselectFilteredS3{ID: 10, P30: "s0_1"})
	assertSubselectFilteredField(t, listener, "ids0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 99, P00: "s0_1"})
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 11, P10: "s0_1"})
	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 11, P20: "xxx"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS3{ID: 11, P30: "s0_1"})
	assertSubselectFilteredField(t, listener, "ids0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 98, P00: "s0_2"})
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 12, P10: "s0_x"})
	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 12, P20: "s0_2"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS3{ID: 12, P30: "s0_1"})
	assertSubselectFilteredField(t, listener, "ids0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 13, P10: "s0_2"})
	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 13, P20: "s0_2"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS3{ID: 13, P30: "s0_x"})
	assertSubselectFilteredField(t, listener, "ids0", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 14, P10: "s0_2"})
	sendSubselectFiltered(t, engine, subselectFilteredS2{ID: 14, P20: "s0_2"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS3{ID: 14, P30: "s0_2"})
	assertSubselectFilteredField(t, listener, "ids0", 98)
}

// subselectFilteredSendCoercionBean mirrors the Java sendBean helper.
func subselectFilteredSendCoercionBean(t *testing.T, engine *Engine, theString string, intPrimitive, intBoxed int, longBoxed int64, doubleBoxed float64) {
	t.Helper()
	sendSubselectFiltered(t, engine, subselectFilteredBean{
		TheString:     theString,
		IntPrimitive:  intPrimitive,
		IntBoxed:      intBoxed,
		LongBoxed:     longBoxed,
		DoubleBoxed:   doubleBoxed,
	})
}

func newSubselectFilteredCoercionQuery(env *Environment, predicate Expression[bool]) Query {
	inner := From[subselectFilteredBean](env, "SupportBean").
		Filter(Equal[string](Field[subselectFilteredBean, string]("theString"), Literal("S"))).
		Window(LengthWindow(1000)).
		AsRecord()
	filtered := func(name string) Stream[subselectFilteredBean] {
		return From[subselectFilteredBean](env, "SupportBean").
			Filter(Equal[string](Field[subselectFilteredBean, string]("theString"), Literal(name)))
	}
	return JoinMany(
		JoinSource(filtered("A").Window(KeepAll())),
		JoinSource(filtered("B").Window(KeepAll())),
		JoinSource(filtered("C").Window(KeepAll())),
	).On(
		OnSourcesEqual(0, Field[subselectFilteredBean, int]("intPrimitive"), 1, Field[subselectFilteredBean, int]("intPrimitive")),
		OnSourcesEqual(1, Field[subselectFilteredBean, int]("intPrimitive"), 2, Field[subselectFilteredBean, int]("intPrimitive")),
	).Select(
		SelectLeft("ids0", SubqueryValue[int](inner, Field[any, int]("intPrimitive"), predicate)),
	).Query(StatementName("s0"))
}

// TestSubselectFilteredSelectWhereJoined4CoercionParity mirrors
// EPLSubselectSelectWhereJoined4Coercion: correlated predicates coerce between
// int, long and double properties across the outer join streams.
func TestSubselectFilteredSelectWhereJoined4CoercionParity(t *testing.T) {
	predicates := []Expression[bool]{
		And(
			EqualOf(Field[any, any]("intBoxed"), JoinField[any](0, "longBoxed")),
			And(
				EqualOf(Field[any, any]("intBoxed"), JoinField[any](1, "doubleBoxed")),
				EqualOf(Field[any, any]("doubleBoxed"), JoinField[any](2, "intBoxed")),
			),
		),
		And(
			EqualOf(Field[any, any]("doubleBoxed"), JoinField[any](2, "intBoxed")),
			And(
				EqualOf(Field[any, any]("intBoxed"), JoinField[any](1, "doubleBoxed")),
				EqualOf(Field[any, any]("intBoxed"), JoinField[any](0, "longBoxed")),
			),
		),
		And(
			EqualOf(Field[any, any]("doubleBoxed"), JoinField[any](2, "intBoxed")),
			And(
				EqualOf(Field[any, any]("intBoxed"), JoinField[any](0, "longBoxed")),
				EqualOf(Field[any, any]("intBoxed"), JoinField[any](1, "doubleBoxed")),
			),
		),
	}
	for variant, predicate := range predicates {
		t.Run(fmt.Sprintf("variant-%d", variant), func(t *testing.T) {
			env := newSubselectFilteredEnvironment(t)
			engine, listener := deploySubselectFiltered(t, env, newSubselectFilteredCoercionQuery(env, predicate))

			subselectFilteredSendCoercionBean(t, engine, "A", 1, 10, 200, 3000)
			subselectFilteredSendCoercionBean(t, engine, "B", 1, 10, 200, 3000)
			subselectFilteredSendCoercionBean(t, engine, "C", 1, 10, 200, 3000)
			assertSubselectFilteredField(t, listener, "ids0", nil)

			subselectFilteredSendCoercionBean(t, engine, "S", -2, 11, 0, 3001)
			subselectFilteredSendCoercionBean(t, engine, "A", 2, 0, 11, 0)
			subselectFilteredSendCoercionBean(t, engine, "B", 2, 0, 0, 11)
			listener.reset()
			subselectFilteredSendCoercionBean(t, engine, "C", 2, 3001, 0, 0)
			assertSubselectFilteredField(t, listener, "ids0", -2)

			subselectFilteredSendCoercionBean(t, engine, "S", -3, 12, 0, 3002)
			subselectFilteredSendCoercionBean(t, engine, "A", 3, 0, 12, 0)
			subselectFilteredSendCoercionBean(t, engine, "B", 3, 0, 0, 12)
			listener.reset()
			subselectFilteredSendCoercionBean(t, engine, "C", 3, 3003, 0, 0)
			assertSubselectFilteredField(t, listener, "ids0", nil)

			subselectFilteredSendCoercionBean(t, engine, "S", -4, 11, 0, 3003)
			subselectFilteredSendCoercionBean(t, engine, "A", 4, 0, 0, 0)
			subselectFilteredSendCoercionBean(t, engine, "B", 4, 0, 0, 11)
			listener.reset()
			subselectFilteredSendCoercionBean(t, engine, "C", 4, 3003, 0, 0)
			assertSubselectFilteredField(t, listener, "ids0", nil)

			subselectFilteredSendCoercionBean(t, engine, "S", -5, 14, 0, 3004)
			subselectFilteredSendCoercionBean(t, engine, "A", 5, 0, 14, 0)
			subselectFilteredSendCoercionBean(t, engine, "B", 5, 0, 0, 11)
			listener.reset()
			subselectFilteredSendCoercionBean(t, engine, "C", 5, 3004, 0, 0)
			assertSubselectFilteredField(t, listener, "ids0", nil)
		})
	}
}

// TestSubselectFilteredSelectWhereJoined4BackCoercionParity mirrors
// EPLSubselectSelectWhereJoined4BackCoercion: the same coercions with the
// inner property on the wider side of the comparison.
func TestSubselectFilteredSelectWhereJoined4BackCoercionParity(t *testing.T) {
	predicates := []Expression[bool]{
		And(
			EqualOf(Field[any, any]("longBoxed"), JoinField[any](0, "intBoxed")),
			And(
				EqualOf(Field[any, any]("longBoxed"), JoinField[any](1, "doubleBoxed")),
				EqualOf(Field[any, any]("intBoxed"), JoinField[any](2, "longBoxed")),
			),
		),
		And(
			EqualOf(Field[any, any]("longBoxed"), JoinField[any](1, "doubleBoxed")),
			And(
				EqualOf(Field[any, any]("intBoxed"), JoinField[any](2, "longBoxed")),
				EqualOf(Field[any, any]("longBoxed"), JoinField[any](0, "intBoxed")),
			),
		),
	}
	for variant, predicate := range predicates {
		t.Run(fmt.Sprintf("variant-%d", variant), func(t *testing.T) {
			env := newSubselectFilteredEnvironment(t)
			engine, listener := deploySubselectFiltered(t, env, newSubselectFilteredCoercionQuery(env, predicate))

			subselectFilteredSendCoercionBean(t, engine, "A", 1, 10, 200, 3000)
			subselectFilteredSendCoercionBean(t, engine, "B", 1, 10, 200, 3000)
			subselectFilteredSendCoercionBean(t, engine, "C", 1, 10, 200, 3000)
			assertSubselectFilteredField(t, listener, "ids0", nil)

			subselectFilteredSendCoercionBean(t, engine, "S", -1, 11, 201, 0)
			subselectFilteredSendCoercionBean(t, engine, "A", 2, 201, 0, 0)
			subselectFilteredSendCoercionBean(t, engine, "B", 2, 0, 0, 201)
			listener.reset()
			subselectFilteredSendCoercionBean(t, engine, "C", 2, 0, 11, 0)
			assertSubselectFilteredField(t, listener, "ids0", -1)

			subselectFilteredSendCoercionBean(t, engine, "S", -2, 12, 202, 0)
			subselectFilteredSendCoercionBean(t, engine, "A", 3, 202, 0, 0)
			subselectFilteredSendCoercionBean(t, engine, "B", 3, 0, 0, 202)
			listener.reset()
			subselectFilteredSendCoercionBean(t, engine, "C", 3, 0, -1, 0)
			assertSubselectFilteredField(t, listener, "ids0", nil)

			subselectFilteredSendCoercionBean(t, engine, "S", -3, 13, 203, 0)
			subselectFilteredSendCoercionBean(t, engine, "A", 4, 203, 0, 0)
			subselectFilteredSendCoercionBean(t, engine, "B", 4, 0, 0, 203.0001)
			listener.reset()
			subselectFilteredSendCoercionBean(t, engine, "C", 4, 0, 13, 0)
			assertSubselectFilteredField(t, listener, "ids0", nil)

			subselectFilteredSendCoercionBean(t, engine, "S", -4, 14, 204, 0)
			subselectFilteredSendCoercionBean(t, engine, "A", 5, 205, 0, 0)
			subselectFilteredSendCoercionBean(t, engine, "B", 5, 0, 0, 204)
			listener.reset()
			subselectFilteredSendCoercionBean(t, engine, "C", 5, 0, 14, 0)
			assertSubselectFilteredField(t, listener, "ids0", nil)
		})
	}
}

// TestSubselectFilteredSubselectPriorParity mirrors EPLSubselectSubselectPrior:
// an insert-into chain whose consumer filters duplicates through coalesced
// scalar subqueries over the target stream's last inserted row. Esper's
// wildcard join fragments ("a.id") map to inserted Event values navigated with
// NestedField.
func TestSubselectFilteredSubselectPriorParity(t *testing.T) {
	env := newSubselectFilteredEnvironment(t)
	pairFields := []FieldSpec{
		FieldDef("a", reflect.TypeOf(Event{})),
		FieldDef("b", reflect.TypeOf(Event{})),
	}
	if _, err := RegisterMap(env, "Pair", pairFields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "PairDuplicatesRemoved", pairFields); err != nil {
		t.Fatal(err)
	}

	device := Field[subselectFilteredSensorEvent, string]("device")
	sensorType := Field[subselectFilteredSensorEvent, string]("type")
	pairQuery := Join(
		From[subselectFilteredSensorEvent](env, "SupportSensorEvent").Filter(Equal[string](device, Literal("A"))).Window(LastEvent()),
		From[subselectFilteredSensorEvent](env, "SupportSensorEvent").Filter(Equal[string](device, Literal("B"))).Window(LastEvent()),
		OnEqual(sensorType, sensorType),
	).Select(
		SelectLeft("a", JoinEventValue[Event](0)),
		SelectRight("b", JoinEventValue[Event](1)),
	).InsertInto("Pair", StatementName("pair"))

	seedQuery := FromAny(env, "Pair").Filter(Literal(false)).Select(
		Alias("a", Field[any, Event]("a")),
		Alias("b", Field[any, Event]("b")),
	).InsertInto("PairDuplicatesRemoved", StatementName("pair-duplicates-removed-seed"))

	pdrLast := FromAny(env, "PairDuplicatesRemoved").Window(LastEvent())
	aID := NestedField[int](Field[any, Event]("a"), "id")
	bID := NestedField[int](Field[any, Event]("b"), "id")
	s0Query := FromAny(env, "Pair").Filter(And(
		NotEqual[int](aID, Coalesce[int](SubqueryValue[int](pdrLast, aID), Literal(-1))),
		NotEqual[int](bID, Coalesce[int](SubqueryValue[int](pdrLast, bID), Literal(-1))),
	)).Select(
		Alias("a", Field[any, Event]("a")),
		Alias("b", Field[any, Event]("b")),
	).InsertInto("PairDuplicatesRemoved", StatementName("s0"))

	var plans []Plan
	for _, query := range []Query{pairQuery, seedQuery, s0Query} {
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	engine := NewEngine(env)
	var s0Deployment *Deployment
	for index, plan := range plans {
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		if index == len(plans)-1 {
			s0Deployment = deployment
		}
	}
	listener := &subselectFilteredListener{}
	if _, err := s0Deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		listener.invoked = true
		listener.lastNew = listener.lastNew[:0]
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("s0 result is not a row: %#v", result)
			}
			listener.lastNew = append(listener.lastNew, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendSensor := func(id int, sensorType, device string, measurement, confidence float64) {
		t.Helper()
		sendSubselectFiltered(t, engine, subselectFilteredSensorEvent{ID: id, Type: sensorType, Device: device, Measurement: measurement, Confidence: confidence})
	}
	assertFired := func(expectedAID, expectedBID int) {
		t.Helper()
		if !listener.invoked || len(listener.lastNew) != 1 {
			t.Fatalf("expected one fired row, listener=%+v", listener)
		}
		row := listener.lastNew[0]
		aEvent, ok := row.Get("a").Any().(Event)
		if !ok {
			t.Fatalf("a is not an event: %#v", row.Get("a").Any())
		}
		bEvent, ok := row.Get("b").Any().(Event)
		if !ok {
			t.Fatalf("b is not an event: %#v", row.Get("b").Any())
		}
		if got := aEvent.Get("id").Any(); !reflect.DeepEqual(got, expectedAID) {
			t.Fatalf("a.id = %#v, want %d", got, expectedAID)
		}
		if got := bEvent.Get("id").Any(); !reflect.DeepEqual(got, expectedBID) {
			t.Fatalf("b.id = %#v, want %d", got, expectedBID)
		}
	}

	sendSensor(1, "Temperature", "A", 51, 94.5)
	if listener.invoked {
		t.Fatal("listener must not be invoked before any B event")
	}
	sendSensor(2, "Temperature", "A", 57, 95.5)
	if listener.invoked {
		t.Fatal("listener must not be invoked before any B event")
	}
	sendSensor(3, "Humidity", "B", 29, 67.5)
	if listener.invoked {
		t.Fatal("listener must not be invoked for mismatched types")
	}

	listener.reset()
	sendSensor(4, "Temperature", "B", 55, 88.0)
	assertFired(2, 4)

	listener.reset()
	sendSensor(5, "Temperature", "B", 65, 85.0)
	if listener.invoked {
		t.Fatal("listener must not be invoked while a.id is still the last inserted id")
	}
	sendSensor(6, "Temperature", "B", 49, 87.0)
	if listener.invoked {
		t.Fatal("listener must not be invoked while a.id is still the last inserted id")
	}

	listener.reset()
	sendSensor(7, "Temperature", "A", 51, 99.5)
	assertFired(7, 6)
}
