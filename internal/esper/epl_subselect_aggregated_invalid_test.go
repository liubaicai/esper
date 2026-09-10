package esper

import (
	"context"
	"errors"
	"testing"
)

// Mirrors EPLSubselectAggregatedSingleValue's EPLSubselectAggregatedInvalid
// execution (work unit 4.363 dispositions): six ungrouped single-value
// subselect shapes Java rejects at compile time. The Go builder rejects the
// same invariants at Build since the 4.363 validation — correlated
// properties inside aggregation arguments, bare inner-stream fields outside
// aggregation boundaries once an aggregate is present, and non-boolean
// having predicates.
func TestSubselectAggregatedInvalidParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "SubqueryHavingTrade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subqueryHavingTrigger](env, "SubqueryHavingTrigger"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	price := Field[any, float64]("price")

	assertRejected := func(tag string, wantCode error, build func() error) {
		t.Helper()
		err := build()
		if err == nil {
			t.Fatalf("%s: built successfully, want rejection", tag)
		}
		if wantCode != nil && !errors.Is(err, wantCode) {
			t.Fatalf("%s: error = %v, want class %v", tag, err, wantCode)
		}
	}

	// 1. Correlated property inside the aggregate argument: the outer
	//    Trigger threshold lands inside the aggregate over the inner
	//    length-window (the outer stream is the flow's trigger).
	assertRejected("correlated-aggregate-argument", ErrorInvalidRule, func() error {
		inner := From[subqueryHavingTrigger](env, "SubqueryHavingTrigger").Window(LengthWindow(3)).AsRecord()
		query := Select(
			From[runtimeTestTrade](env, "SubqueryHavingTrade"),
			Alias("value", SubqueryValueWithOptions[float64](inner,
				Sum[float64](OuterField[float64]("threshold")))),
		).Query(StatementName("s0"))
		_, err := env.Build(query)
		return err
	})

	// 2. Mixed bare inner property and aggregate in the scalar projection:
	//    select (select price + sum(price) from Trade#length(3)) as value
	//    from Trade.
	assertRejected("mixed-projection", ErrorInvalidRule, func() error {
		inner := From[runtimeTestTrade](env, "SubqueryHavingTrade").Window(LengthWindow(3)).AsRecord()
		query := Select(
			From[runtimeTestTrade](env, "SubqueryHavingTrade"),
			Alias("value", SubqueryValueWithOptions[float64](inner,
				Add[float64](Field[any, float64]("price"),
					Sum[float64](Field[any, float64]("price"))))),
		).Query(StatementName("s0"))
		_, err := env.Build(query)
		return err
	})

	// 3. Correlated compound property inside the aggregate argument:
	//    select (select sum(threshold + price) from Trigger#length(3)) as
	//    value from Trigger.
	assertRejected("correlated-compound-argument", ErrorInvalidRule, func() error {
		inner := From[subqueryHavingTrigger](env, "SubqueryHavingTrigger").Window(LengthWindow(3)).AsRecord()
		query := Select(
			From[runtimeTestTrade](env, "SubqueryHavingTrade"),
			Alias("value", SubqueryValueWithOptions[float64](inner,
				Sum[float64](Add[float64](OuterField[float64]("threshold"),
					Field[any, float64]("price"))))),
		).Query(StatementName("s0"))
		_, err := env.Build(query)
		return err
	})

	// 4. Having-clause aggregate over an outer property (Java rejects the
	//    String coercion of sum(s0.p00); the outer-field-in-aggregate class
	//    is the Go-pinned invariant).
	assertRejected("having-outer-aggregate", ErrorInvalidRule, func() error {
		inner := From[runtimeTestTrade](env, "SubqueryHavingTrade").Window(KeepAll()).AsRecord()
		query := Select(
			From[runtimeTestTrade](env, "SubqueryHavingTrade"),
			Alias("value", SubqueryValueWithOptions[float64](inner,
				Sum[float64](OuterField[float64]("threshold")),
				SubqueryHaving(Equal[float64](
					Sum[float64](OuterField[float64]("threshold")), Literal(1))))),
		).Query(StatementName("s0"))
		_, err := env.Build(query)
		return err
	})

	// 5. Inner field outside aggregation in the ungrouped having:
	//    having sum(price) = price.
	assertRejected("having-inner-property-unaggregated", ErrorInvalidRule, func() error {
		inner := From[runtimeTestTrade](env, "SubqueryHavingTrade").Window(KeepAll()).AsRecord()
		query := Select(
			From[runtimeTestTrade](env, "SubqueryHavingTrade"),
			Alias("value", SubqueryValueWithOptions[float64](inner,
				Sum[float64](price),
				SubqueryHaving(Equal[float64](
					Sum[float64](price), price)))),
		).Query(StatementName("s0"))
		_, err := env.Build(query)
		return err
	})

	// 6. Non-boolean having: having sum(price).
	assertRejected("having-non-boolean", ErrorTypeMismatch, func() error {
		inner := From[runtimeTestTrade](env, "SubqueryHavingTrade").Window(KeepAll()).AsRecord()
		query := Select(
			From[runtimeTestTrade](env, "SubqueryHavingTrade"),
			Alias("value", SubqueryValueWithOptions[float64](inner,
				Sum[float64](price),
				SubqueryHaving(Sum[float64](price)))),
		).Query(StatementName("s0"))
		_, err := env.Build(query)
		return err
	})
}
