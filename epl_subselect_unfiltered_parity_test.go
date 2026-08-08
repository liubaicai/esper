package esper

import (
	"context"
	"reflect"
	"testing"
)

// subselectUnfilteredS4 mirrors SupportBean_S4 (id, p40).
type subselectUnfilteredS4 struct {
	ID  int    `esper:"id"`
	P40 string `esper:"p40"`
}

func newSubselectUnfilteredEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	registrations := []func() error{
		func() error { _, err := RegisterStruct[subselectFilteredS0](env, "SupportBean_S0"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredS1](env, "SupportBean_S1"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredS3](env, "SupportBean_S3"); return err },
		func() error { _, err := RegisterStruct[subselectUnfilteredS4](env, "SupportBean_S4"); return err },
		func() error { _, err := RegisterStruct[subselectFilteredBean](env, "SupportBean"); return err },
	}
	for _, register := range registrations {
		if err := register(); err != nil {
			t.Fatal(err)
		}
	}
	return env
}

// TestSubselectUnfilteredSelfSubselectParity mirrors EPLSubselectSelfSubselect:
// an insert-into count stream read back through a lastevent subselect sees the
// count of the previously routed events, not the in-flight trigger.
func TestSubselectUnfilteredSelfSubselectParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	if _, err := RegisterMap(env, "MyCount", []FieldSpec{FieldDef("cnt", reflect.TypeOf(int64(0)))}); err != nil {
		t.Fatal(err)
	}
	insertQuery := From[subselectFilteredS0](env, "SupportBean_S0").Aggregate(
		Alias("cnt", CountAll()),
	).InsertInto("MyCount", StatementName("insert-count"))
	s0Query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("value", SubqueryValue[int64](FromAny(env, "MyCount").Window(LastEvent()), Field[any, int64]("cnt"))),
	).Query(StatementName("s0"))

	insertPlan, err := env.Build(insertQuery)
	if err != nil {
		t.Fatal(err)
	}
	s0Plan, err := env.Build(s0Query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	s0Deployment, err := engine.Deploy(context.Background(), s0Plan)
	if err != nil {
		t.Fatal(err)
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

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertSubselectFilteredField(t, listener, "value", nil)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	assertSubselectFilteredField(t, listener, "value", int64(1))
}

// TestSubselectUnfilteredStartStopStatementParity mirrors
// EPLSubselectStartStopStatement: a constant-true subselect gates the outer
// stream, and undeploy/redeploy resets the subquery window state.
func TestSubselectUnfilteredStartStopStatementParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	build := func() Plan {
		inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
		plan, err := env.Build(Select(
			From[subselectFilteredS0](env, "SupportBean_S0").Filter(
				SubqueryValue[bool](inner, Literal(true)),
			),
			Alias("id", Field[subselectFilteredS0, int]("id")),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	engine := NewEngine(env)
	deploy := func() (*Deployment, *subselectFilteredListener) {
		deployment, err := engine.Deploy(context.Background(), build())
		if err != nil {
			t.Fatal(err)
		}
		listener := &subselectFilteredListener{}
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
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
		return deployment, listener
	}

	deployment, listener := deploy()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	if listener.invoked {
		t.Fatal("listener must not be invoked while the subquery window is empty")
	}
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	assertSubselectFilteredField(t, listener, "id", 2)
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})

	_, listener = deploy()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	if listener.invoked {
		t.Fatal("listener must not be invoked after redeploy while the subquery window is empty")
	}
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 3})
	assertSubselectFilteredField(t, listener, "id", 3)
}

// TestSubselectUnfilteredWhereClauseReturningTrueParity mirrors
// EPLSubselectWhereClauseReturningTrue.
func TestSubselectUnfilteredWhereClauseReturningTrueParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0").Filter(
			SubqueryValue[bool](inner, Literal(true)),
		),
		Alias("id", Field[subselectFilteredS0, int]("id")),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	assertSubselectFilteredField(t, listener, "id", 2)
}

// TestSubselectUnfilteredWhereClauseWithExpressionParity mirrors
// EPLSubselectWhereClauseWithExpression: the where subselect projects a
// boolean expression over the inner event.
func TestSubselectUnfilteredWhereClauseWithExpressionParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0").Filter(
			SubqueryValue[bool](inner, Equal[string](Field[any, string]("p10"), Literal("X"))),
		),
		Alias("id", Field[subselectFilteredS0, int]("id")),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	if listener.invoked {
		t.Fatal("listener must not be invoked while the subquery window is empty")
	}

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10, P10: "X"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "id", 0)
}

// TestSubselectUnfilteredJoinUnfilteredParity mirrors
// EPLSubselectJoinUnfiltered: two uncorrelated subselects inside a two-stream
// keepall join, null on multiple inner rows.
func TestSubselectUnfilteredJoinUnfilteredParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	innerS3 := From[subselectFilteredS3](env, "SupportBean_S3").Window(LengthWindow(1000)).AsRecord()
	innerS4 := From[subselectUnfilteredS4](env, "SupportBean_S4").Window(LengthWindow(1000)).AsRecord()
	query := Join(
		From[subselectFilteredS0](env, "SupportBean_S0").Window(KeepAll()),
		From[subselectFilteredS1](env, "SupportBean_S1").Window(KeepAll()),
		OnEqual(Field[subselectFilteredS0, int]("id"), Field[subselectFilteredS1, int]("id")),
	).Select(
		SelectLeft("idS3", SubqueryValueWithOptions[int](
			innerS3,
			Field[any, int]("id"),
			SubqueryCardinalityMode(SubqueryNullOnMultiple),
		)),
		SelectRight("idS4", SubqueryValueWithOptions[int](
			innerS4,
			Field[any, int]("id"),
			SubqueryCardinalityMode(SubqueryNullOnMultiple),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	assertPair := func(s3, s4 any) {
		t.Helper()
		if !listener.invoked || len(listener.lastNew) != 1 {
			t.Fatalf("expected one fired row, listener=%+v", listener)
		}
		row := listener.lastNew[0]
		for field, expected := range map[string]any{"idS3": s3, "idS4": s4} {
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

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 0})
	assertPair(nil, nil)

	sendSubselectFiltered(t, engine, subselectFilteredS3{ID: -1})
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 1})
	assertPair(-1, nil)

	sendSubselectFiltered(t, engine, subselectUnfilteredS4{ID: -2})
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 2})
	assertPair(-1, -2)

	sendSubselectFiltered(t, engine, subselectUnfilteredS4{ID: -2})
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 3})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 3})
	assertPair(-1, nil)

	// The second S0(3) matches the retained S1(3) and the second S1(3) matches
	// both retained S0(3) events: three fires accumulate, all null/null.
	sendSubselectFiltered(t, engine, subselectFilteredS3{ID: -2})
	listener.resetAll()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 3})
	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 3})
	if len(listener.allRows) != 3 {
		t.Fatalf("expected 3 accumulated fires, got %d", len(listener.allRows))
	}
	for _, row := range listener.allRows {
		if !row.Get("idS3").IsNull() || !row.Get("idS4").IsNull() {
			t.Fatalf("expected null/null row, got %#v", row)
		}
	}
}

// TestSubselectUnfilteredInvalidSubselectParity mirrors
// EPLSubselectInvalidSubselect: window-less, invalid-property, nested,
// aggregate-filter and non-boolean subselects are rejected at Build time. The
// unqualified-property and IN-type-mismatch cases are compile-time impossible
// in the typed Go builder (OuterField vs Field and SubqueryIn[T] make the
// distinction explicit), so they are registered as approved differences.
func TestSubselectUnfilteredInvalidSubselectParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	assertInvalid := func(label string, query Query) {
		t.Helper()
		if _, err := env.Build(query); err == nil {
			t.Fatalf("%s must be rejected", label)
		}
	}

	// Subquery without a limiting window in the select clause.
	assertInvalid("windowless-select", Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("idS1", SubqueryValue[int](
			From[subselectFilteredS1](env, "SupportBean_S1").AsRecord(),
			Field[any, int]("id"),
		)),
	).Query(StatementName("s0")))

	// Invalid property inside the subquery projection.
	assertInvalid("invalid-property", Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("idS1", SubqueryValue[int](
			From[subselectFilteredS1](env, "SupportBean_S1").Window(LastEvent()).AsRecord(),
			Field[any, int]("dummy"),
		)),
	).Query(StatementName("s0")))

	// Nested subquery-within-subquery in the scalar projection. Go keeps
	// documented extensions for subqueries inside filter predicates and
	// multi-column SubqueryRow fragments, so only this shape is rejected.
	nested := From[subselectFilteredS1](env, "SupportBean_S1").Window(LastEvent()).AsRecord()
	assertInvalid("nested-subquery", Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("idS1", SubqueryValue[int](
			nested,
			SubqueryValue[int](nested, Field[any, int]("id")),
		)),
	).Query(StatementName("s0")))

	// Aggregation inside the subquery filter.
	assertInvalid("aggregate-in-filter", Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("idS1", SubqueryValue[int](
			nested,
			Field[any, int]("id"),
			Equal[int64](Sum[int64](Field[any, int64]("id")), Literal(int64(5))),
		)),
	).Query(StatementName("s0")))

	// Subquery without a limiting window inside the outer filter.
	assertInvalid("windowless-filter", Select(
		From[subselectFilteredS0](env, "SupportBean_S0").Filter(And(
			Equal[int](Field[subselectFilteredS0, int]("id"), Literal(5)),
			SubqueryValue[bool](From[subselectFilteredS1](env, "SupportBean_S1").AsRecord(), Literal(true)),
		)),
		Alias("id", Field[subselectFilteredS0, int]("id")),
	).Query(StatementName("s0")))

	// Non-boolean subquery filter.
	assertInvalid("non-boolean-filter", Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("idS1", SubqueryValue[int](
			nested,
			Field[any, int]("id"),
			Literal("a"),
		)),
	).Query(StatementName("s0")))
}

// TestSubselectUnfilteredStreamPriorParity mirrors
// EPLSubselectUnfilteredStreamPriorOM/Compile: Esper prior(0, id) refers to the
// current subquery row, which the Go builder expresses as the row itself
// (Go Prior(0, x) matches Esper prior(1, x) by convention).
func TestSubselectUnfilteredStreamPriorParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("idS1", SubqueryValue[int](inner, Field[any, int]("id"))),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "idS1", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertSubselectFilteredField(t, listener, "idS1", 10)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	assertSubselectFilteredField(t, listener, "idS1", 10)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 3})
	assertSubselectFilteredField(t, listener, "idS1", 10)
}

// TestSubselectUnfilteredCustomFunctionParity mirrors
// EPLSubselectCustomFunction: a UDF over the inner property as the subquery
// projection.
func TestSubselectUnfilteredCustomFunctionParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	minusOne := Func1("minusOne", func(id int) float64 { return float64(id) - 1 }, Field[any, int]("id"))
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("idS1", SubqueryValueWithOptions[float64](
			inner,
			minusOne,
			SubqueryCardinalityMode(SubqueryNullOnMultiple),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "idS1", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertSubselectFilteredField(t, listener, "idS1", 9.0)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	assertSubselectFilteredField(t, listener, "idS1", 9.0)
}

// TestSubselectUnfilteredComputedResultParity mirrors
// EPLSubselectComputedResult: arithmetic over the scalar subquery result, null
// propagates through the computation.
func TestSubselectUnfilteredComputedResultParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("idS1", Multiply[int](Literal(100), SubqueryValueWithOptions[int](
			inner,
			Field[any, int]("id"),
			SubqueryCardinalityMode(SubqueryNullOnMultiple),
		))),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "idS1", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertSubselectFilteredField(t, listener, "idS1", 1000)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	assertSubselectFilteredField(t, listener, "idS1", 1000)
}

// TestSubselectUnfilteredFilterInsideParity mirrors EPLSubselectFilterInside:
// a filter on the subquery's inner stream.
func TestSubselectUnfilteredFilterInsideParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").
		Filter(Equal[string](Field[subselectFilteredS1, string]("p10"), Literal("A"))).
		Window(LengthWindow(1000)).
		AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("idS1", SubqueryValue[int](inner, Field[any, int]("id"))),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 1, P10: "X"})
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertSubselectFilteredField(t, listener, "idS1", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 1, P10: "A"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertSubselectFilteredField(t, listener, "idS1", 1)
}

// runSubselectUnfilteredMultiRowScenario mirrors tryAssertMultiRowUnfiltered.
func runSubselectUnfilteredMultiRowScenario(t *testing.T, engine *Engine, listener *subselectFilteredListener) {
	t.Helper()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, "idS1", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertSubselectFilteredField(t, listener, "idS1", 10)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	assertSubselectFilteredField(t, listener, "idS1", 10)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 999})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 3})
	assertSubselectFilteredField(t, listener, "idS1", nil)
}

// TestSubselectUnfilteredUnlimitedStreamParity mirrors
// EPLSubselectUnfilteredUnlimitedStream: multiple inner rows yield null.
func TestSubselectUnfilteredUnlimitedStreamParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("idS1", SubqueryValueWithOptions[int](
			inner,
			Field[any, int]("id"),
			SubqueryCardinalityMode(SubqueryNullOnMultiple),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)
	runSubselectUnfilteredMultiRowScenario(t, engine, listener)
}

// TestSubselectUnfilteredLengthWindowParity mirrors
// EPLSubselectUnfilteredLengthWindow: a small length window with two rows
// yields null.
func TestSubselectUnfilteredLengthWindowParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LengthWindow(2)).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("idS1", SubqueryValueWithOptions[int](
			inner,
			Field[any, int]("id"),
			SubqueryCardinalityMode(SubqueryNullOnMultiple),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)
	runSubselectUnfilteredMultiRowScenario(t, engine, listener)
}

// runSubselectUnfilteredSingleRowScenario mirrors tryAssertSingleRowUnfiltered.
func runSubselectUnfilteredSingleRowScenario(t *testing.T, engine *Engine, listener *subselectFilteredListener, column string) {
	t.Helper()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 0})
	assertSubselectFilteredField(t, listener, column, nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertSubselectFilteredField(t, listener, column, 10)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	assertSubselectFilteredField(t, listener, column, 10)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 999})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 3})
	assertSubselectFilteredField(t, listener, column, 999)
}

func newSubselectUnfilteredSingleRowQuery(env *Environment, column string) Query {
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LastEvent()).AsRecord()
	return Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias(column, SubqueryValue[int](inner, Field[any, int]("id"))),
	).Query(StatementName("s0"))
}

// TestSubselectUnfilteredAsAfterSubselectParity mirrors
// EPLSubselectUnfilteredAsAfterSubselect.
func TestSubselectUnfilteredAsAfterSubselectParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	engine, listener := deploySubselectFiltered(t, env, newSubselectUnfilteredSingleRowQuery(env, "idS1"))
	runSubselectUnfilteredSingleRowScenario(t, engine, listener, "idS1")
}

// TestSubselectUnfilteredWithAsWithinSubselectParity mirrors
// EPLSubselectUnfilteredWithAsWithinSubselect: the inner alias names the
// output column; Go makes that name explicit on the selection alias.
func TestSubselectUnfilteredWithAsWithinSubselectParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	engine, listener := deploySubselectFiltered(t, env, newSubselectUnfilteredSingleRowQuery(env, "myId"))
	runSubselectUnfilteredSingleRowScenario(t, engine, listener, "myId")
}

// TestSubselectUnfilteredNoAsParity mirrors EPLSubselectUnfilteredNoAs: the
// output column keeps the inner property name; Go makes that name explicit.
func TestSubselectUnfilteredNoAsParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	engine, listener := deploySubselectFiltered(t, env, newSubselectUnfilteredSingleRowQuery(env, "id"))
	runSubselectUnfilteredSingleRowScenario(t, engine, listener, "id")
}

// TestSubselectUnfilteredLastEventParity mirrors
// EPLSubselectUnfilteredLastEvent: lastevent subselect over a second stream
// with a two-column projection.
func TestSubselectUnfilteredLastEventParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	inner := From[subselectFilteredS0](env, "SupportBean_S0").Window(LastEvent()).AsRecord()
	query := Select(
		From[subselectFilteredBean](env, "SupportBean"),
		Alias("theString", Field[subselectFilteredBean, string]("theString")),
		Alias("col", SubqueryValue[string](inner, Field[any, string]("p00"))),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	assertRow := func(expectedString string, expectedCol any) {
		t.Helper()
		if !listener.invoked || len(listener.lastNew) != 1 {
			t.Fatalf("expected one fired row, listener=%+v", listener)
		}
		row := listener.lastNew[0]
		if got := row.Get("theString").Any(); got != expectedString {
			t.Fatalf("theString = %#v, want %s", got, expectedString)
		}
		col := row.Get("col")
		if expectedCol == nil {
			if !col.IsNull() {
				t.Fatalf("col = %#v, want null", col.Any())
			}
			return
		}
		if !reflect.DeepEqual(col.Any(), expectedCol) {
			t.Fatalf("col = %#v, want %#v", col.Any(), expectedCol)
		}
	}

	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "E1", IntPrimitive: 1})
	assertRow("E1", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 11, P00: "S01"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "E2", IntPrimitive: 2})
	assertRow("E2", "S01")

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "E3", IntPrimitive: 3})
	assertRow("E3", "S01")

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 12, P00: "S02"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredBean{TheString: "E4", IntPrimitive: 4})
	assertRow("E4", "S02")
}

// TestSubselectUnfilteredExpressionParity mirrors
// EPLSubselectUnfilteredExpression: concat as the subquery projection.
func TestSubselectUnfilteredExpressionParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	inner := From[subselectFilteredS1](env, "SupportBean_S1").Window(LastEvent()).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("value", SubqueryValue[string](
			inner,
			Concat(Field[any, string]("p10"), Field[any, string]("p11")),
		)),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertSubselectFilteredField(t, listener, "value", nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: -1, P10: "a", P11: "b"})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertSubselectFilteredField(t, listener, "value", "ab")
}

// TestSubselectUnfilteredTwoSubqSelectParity mirrors
// EPLSubselectTwoSubqSelect: two computed subselects in the same select
// clause.
func TestSubselectUnfilteredTwoSubqSelectParity(t *testing.T) {
	env := newSubselectUnfilteredEnvironment(t)
	innerOne := From[subselectFilteredS1](env, "SupportBean_S1").Window(LastEvent()).AsRecord()
	innerTwo := From[subselectFilteredS1](env, "SupportBean_S1").Window(LastEvent()).AsRecord()
	query := Select(
		From[subselectFilteredS0](env, "SupportBean_S0"),
		Alias("idS1_0", SubqueryValue[int](innerOne, Add[int](Field[any, int]("id"), Literal(1)))),
		Alias("idS1_1", SubqueryValue[int](innerTwo, Add[int](Field[any, int]("id"), Literal(2)))),
	).Query(StatementName("s0"))
	engine, listener := deploySubselectFiltered(t, env, query)

	assertPair := func(expected0, expected1 any) {
		t.Helper()
		if !listener.invoked || len(listener.lastNew) != 1 {
			t.Fatalf("expected one fired row, listener=%+v", listener)
		}
		row := listener.lastNew[0]
		for field, expected := range map[string]any{"idS1_0": expected0, "idS1_1": expected1} {
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

	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertPair(nil, nil)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 10})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 1})
	assertPair(11, 12)

	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 2})
	assertPair(11, 12)

	sendSubselectFiltered(t, engine, subselectFilteredS1{ID: 999})
	listener.reset()
	sendSubselectFiltered(t, engine, subselectFilteredS0{ID: 3})
	assertPair(1000, 1001)
}
