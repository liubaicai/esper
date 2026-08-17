package esper

import (
	"context"
	"reflect"
	"testing"
)

// Parity coverage for the subquery-populated bean event-type column
// executions of EPLInsertIntoPopulateEventTypeColumnBean:
// FromSubquerySingle (objectarray/map x filter/no-filter) and
// FromSubqueryMulti (objectarray/map x filter/no-filter where the filter is a
// no-op where 1=1). Oracle: testdata/parity/insertinto-eventcol-subquery-bean
// (16 listener records across 8 cases).
//
// Java semantics pinned by this slice:
//   - single-row scalar subquery into an event-typed column yields NULL when
//     zero rows match the inner where (filter) and NULL when more than one
//     row matches (no-filter #length(2) second send) — strict single-row
//     cardinality, encoded via SubqueryCardinalityMode(SubqueryNullOnMultiple);
//   - multi-row subquery into an array-typed column accumulates every
//     retained row in window order (keepall), and the where 1=1 variant is a
//     behavioral no-op identical to the unfiltered form.

type evtColSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type evtColSupportBeanS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

func evtColRegisterSource(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[evtColSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[evtColSupportBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
}

// evtColRegisterEventOne registers EventOne with a single event-typed column
// (single=true -> sb Event) or an array-typed column (single=false -> sbarr
// []Event), as either an object-array or a map schema.
func evtColRegisterEventOne(t *testing.T, env *Environment, objectArray, single bool) {
	t.Helper()
	var fieldType reflect.Type
	var name string
	if single {
		name = "sb"
		fieldType = reflect.TypeOf(Event{})
	} else {
		name = "sbarr"
		fieldType = reflect.TypeOf([]Event{})
	}
	fields := []FieldSpec{FieldDef(name, fieldType)}
	var err error
	if objectArray {
		_, err = RegisterObjectArray(env, "EventOne", fields)
	} else {
		_, err = RegisterMap(env, "EventOne", fields)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func evtColSendSupportBean(t *testing.T, engine *Engine, theString string, intPrimitive int) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), evtColSupportBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
		t.Fatal(err)
	}
}

func evtColSendS0(t *testing.T, engine *Engine, id int, p00, p01 string) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), evtColSupportBeanS0{ID: id, P00: p00, P01: p01}); err != nil {
		t.Fatal(err)
	}
}

// evtColDeploy builds the insert-into producer (subquery into the EventOne
// event-typed column) and a FromAny consumer that observes the routed EventOne
// events, returning the consumer-collected results.
func evtColDeploy(t *testing.T, env *Environment, single, filter bool) (*Engine, *[]Result) {
	t.Helper()
	inner := From[evtColSupportBeanS0](env, "SupportBean_S0")
	options := []SubqueryOption{}
	var projection Expr
	var alias string
	if single {
		windowed := inner.Window(LengthWindow(2)).AsRecord()
		options = append(options, SubqueryCardinalityMode(SubqueryNullOnMultiple))
		if filter {
			options = append(options, SubqueryWhere(
				GreaterOrEqual[int](Field[evtColSupportBeanS0, int]("id"), Literal(100))))
		}
		projection = SubqueryValueWithOptions[Event](windowed, EventValue[Event](), options...)
		alias = "sb"
	} else {
		windowed := inner.Window(KeepAll()).AsRecord()
		if filter {
			options = append(options, SubqueryWhere(
				Equal[int](Literal(1), Literal(1))))
		}
		projection = SubqueryValues[Event](windowed, EventValue[Event](), options...)
		alias = "sbarr"
	}
	producer := Select(From[evtColSupportBean](env, "SupportBean"),
		Alias(alias, projection),
	).InsertInto("EventOne", StatementName("s0"))
	consumer := FromAny(env, "EventOne").Query(StatementName("s1"))

	producerPlan, err := env.Build(producer)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(consumer)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	producerDeployment, err := engine.Deploy(context.Background(), producerPlan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { producerDeployment.Undeploy(context.Background()) })
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { consumerDeployment.Undeploy(context.Background()) })

	var results []Result
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		results = append(results, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return engine, &results
}

// evtColFragment reads the event-typed column value (sb or sbarr) of a routed
// EventOne event. It returns the fragment Event for a single column.
func evtColFragment(t *testing.T, result Result, name string) (Event, bool) {
	t.Helper()
	event, ok := result.Event()
	if !ok {
		t.Fatalf("routed result is not an event: %#v", result)
	}
	value := event.Get(name)
	if value.IsNull() {
		return Event{}, false
	}
	fragment, ok := value.Any().(Event)
	if !ok {
		t.Fatalf("column %s is not an event fragment: %#v", name, value.Any())
	}
	return fragment, true
}

// evtColSingleCase runs one FromSubquerySingle variant (objectarray/map x
// filter/no-filter) and asserts the strict single-row cardinality contract.
func evtColSingleCase(t *testing.T, objectArray, filter bool) {
	t.Helper()
	env := NewEnvironment()
	evtColRegisterSource(t, env)
	evtColRegisterEventOne(t, env, objectArray, true)
	engine, events := evtColDeploy(t, env, true, filter)

	// S0(1, x1) then SB(E1, 1): no-filter routes the single matching row;
	// filter (id>=100) routes null because id=1 fails the inner where.
	evtColSendS0(t, engine, 1, "x1", "")
	evtColSendSupportBean(t, engine, "E1", 1)
	// S0(100, x2) then SB(E2, 2): filter routes {id:100,p00:x2}; no-filter
	// routes null because #length(2) now holds two rows (null-on-multiple).
	evtColSendS0(t, engine, 100, "x2", "")
	evtColSendSupportBean(t, engine, "E2", 2)

	if len(*events) != 2 {
		t.Fatalf("single events = %#v", *events)
	}
	first, firstOK := evtColFragment(t, (*events)[0], "sb")
	second, secondOK := evtColFragment(t, (*events)[1], "sb")
	if filter {
		// First: id=1 fails id>=100 -> null. Second: id=100 passes -> x2.
		if firstOK {
			t.Fatalf("filter first sb = %#v, want null", first)
		}
		if !secondOK {
			t.Fatalf("filter second sb is null, want x2")
		}
		if got := second.Get("p00").Any(); got != "x2" {
			t.Fatalf("filter second sb.p00 = %#v, want x2", got)
		}
		if got := second.Get("id").Any(); got != 100 {
			t.Fatalf("filter second sb.id = %#v, want 100", got)
		}
	} else {
		// First: exactly one row -> x1. Second: two rows -> null.
		if !firstOK {
			t.Fatalf("no-filter first sb is null, want x1")
		}
		if got := first.Get("p00").Any(); got != "x1" {
			t.Fatalf("no-filter first sb.p00 = %#v, want x1", got)
		}
		if got := first.Get("id").Any(); got != 1 {
			t.Fatalf("no-filter first sb.id = %#v, want 1", got)
		}
		if secondOK {
			t.Fatalf("no-filter second sb = %#v, want null (null-on-multiple)", second)
		}
	}
}

// evtColMultiCase runs one FromSubqueryMulti variant. The where 1=1 filter is
// a behavioral no-op, so the filter and no-filter variants share the same
// assertions; both are exercised to cover the four Java runtime variants.
func evtColMultiCase(t *testing.T, objectArray, filter bool) {
	t.Helper()
	env := NewEnvironment()
	evtColRegisterSource(t, env)
	evtColRegisterEventOne(t, env, objectArray, false)
	engine, events := evtColDeploy(t, env, false, filter)

	// S0(1, x1); SB(E1, 1) -> sbarr == [s0(1,x1)].
	evtColSendS0(t, engine, 1, "x1", "")
	evtColSendSupportBean(t, engine, "E1", 1)
	// S0(2, x2, y2); SB(E2, 2) -> sbarr == [s0(1,x1), s0(2,x2)] (keepall
	// accumulates in window order).
	evtColSendS0(t, engine, 2, "x2", "y2")
	evtColSendSupportBean(t, engine, "E2", 2)

	if len(*events) != 2 {
		t.Fatalf("multi events = %#v", *events)
	}
	assertSbarr := func(result Result, wantIDs []int, wantP00 []string) {
		t.Helper()
		event, ok := result.Event()
		if !ok {
			t.Fatalf("routed result is not an event: %#v", result)
		}
		value := event.Get("sbarr")
		if value.IsNull() {
			t.Fatalf("sbarr is null, want %d rows", len(wantIDs))
		}
		fragments, ok := value.Any().([]Event)
		if !ok {
			t.Fatalf("sbarr is not an event array: %#v", value.Any())
		}
		if len(fragments) != len(wantIDs) {
			t.Fatalf("sbarr length = %d, want %d (%#v)", len(fragments), len(wantIDs), fragments)
		}
		for i, fragment := range fragments {
			if got := fragment.Get("id").Any(); got != wantIDs[i] {
				t.Fatalf("sbarr[%d].id = %#v, want %d", i, got, wantIDs[i])
			}
			if got := fragment.Get("p00").Any(); got != wantP00[i] {
				t.Fatalf("sbarr[%d].p00 = %#v, want %s", i, got, wantP00[i])
			}
		}
	}
	assertSbarr((*events)[0], []int{1}, []string{"x1"})
	assertSbarr((*events)[1], []int{1, 2}, []string{"x1", "x2"})
}

// FromSubquerySingle over objectarray EventOne, no inner filter.
// Oracle case bean-subquery-single-objectarray-nofilter: first sb=x1, second
// sb=null (two rows in #length(2) -> null-on-multiple).
func TestEPLInsertIntoColBeanFromSubquerySingleObjectArrayNoFilterParity(t *testing.T) {
	evtColSingleCase(t, true, false)
}

// FromSubquerySingle over objectarray EventOne with inner where id>=100.
// Oracle case bean-subquery-single-objectarray-filter: first sb=null (id=1
// filtered out), second sb=x2.
func TestEPLInsertIntoColBeanFromSubquerySingleObjectArrayFilterParity(t *testing.T) {
	evtColSingleCase(t, true, true)
}

// FromSubquerySingle over map EventOne, no inner filter.
func TestEPLInsertIntoColBeanFromSubquerySingleMapNoFilterParity(t *testing.T) {
	evtColSingleCase(t, false, false)
}

// FromSubquerySingle over map EventOne with inner where id>=100.
func TestEPLInsertIntoColBeanFromSubquerySingleMapFilterParity(t *testing.T) {
	evtColSingleCase(t, false, true)
}

// FromSubqueryMulti over objectarray EventOne, no inner filter.
// Oracle case bean-subquery-multi-objectarray-nofilter: sbarr accumulates
// [s0(1,x1)] then [s0(1,x1), s0(2,x2)].
func TestEPLInsertIntoColBeanFromSubqueryMultiObjectArrayNoFilterParity(t *testing.T) {
	evtColMultiCase(t, true, false)
}

// FromSubqueryMulti over objectarray EventOne with the no-op where 1=1.
// Oracle case bean-subquery-multi-objectarray-filter is behaviorally identical
// to the no-filter variant.
func TestEPLInsertIntoColBeanFromSubqueryMultiObjectArrayFilterParity(t *testing.T) {
	evtColMultiCase(t, true, true)
}

// FromSubqueryMulti over map EventOne, no inner filter.
func TestEPLInsertIntoColBeanFromSubqueryMultiMapNoFilterParity(t *testing.T) {
	evtColMultiCase(t, false, false)
}

// FromSubqueryMulti over map EventOne with the no-op where 1=1.
func TestEPLInsertIntoColBeanFromSubqueryMultiMapFilterParity(t *testing.T) {
	evtColMultiCase(t, false, true)
}
