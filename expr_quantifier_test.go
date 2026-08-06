package esper

import (
	"context"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

type quantifiedParityEvent struct {
	IntPrimitive int      `esper:"int_primitive"`
	IntBoxed     *int     `esper:"int_boxed"`
	LongBoxed    *int64   `esper:"long_boxed"`
	IntArr       []int    `esper:"int_arr"`
	LongCol      []int64  `esper:"long_col"`
	DoubleBoxed  *float64 `esper:"double_boxed"`
	BigInteger   big.Int  `esper:"big_integer"`
}

func TestQuantifiedExpressionsMatchJavaAnyAllSomeTruthTables(t *testing.T) {
	if got := AnyOf(Literal(2), QuantifierEqual, Literal([]int{1, 2})).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("array any equality = %v, want true", got)
	}
	if got := AnyOf(Literal(2), QuantifierEqual, Literal(map[int]string{1: "one", 2: "two"})).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("map-key any equality = %v, want true", got)
	}
	if got := SomeOf(Literal(0), QuantifierGreater, Literal(1), Literal(2)); !got.eval(EvalContext{}).Equal(Present(false)) {
		t.Fatalf("scalar some comparison = %v, want false", got.eval(EvalContext{}))
	}
	if got := AllOf(Literal(3), QuantifierGreater, Literal([]int{1, 2})).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("array all relation = %v, want true", got)
	}
	if got := AllOf(Literal(2), QuantifierEqual, Literal(2), NullLiteral[int]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("equality all with null = %v, want null", got)
	}
	if got := AnyOf(Literal(0), QuantifierEqual, Literal(1), NullLiteral[int]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("equality any with null = %v, want null", got)
	}
	if got := AllOf(Literal(2), QuantifierGreater, Literal(1), NullLiteral[int]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("relational all with null = %v, want null", got)
	}
	if got := AnyOf(Literal(0), QuantifierGreater, Literal(1), NullLiteral[int]()).eval(EvalContext{}); !got.Equal(Present(false)) {
		t.Fatalf("relational any with null = %v, want false", got)
	}
	if got := AnyOf(Literal(2), QuantifierEqual, Literal([]int{})).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("empty array any = %v, want null", got)
	}
	if got := AllOf(Literal(2), QuantifierEqual, Literal([]int{})).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("empty array all = %v, want null", got)
	}

	bigOne := exactMathInt("1")
	if got := AnyOf(Literal(bigOne), QuantifierEqual, Literal(1)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("BigInteger any equality = %v, want true", got)
	}
}

func TestQuantifiedExpressionsExpandArraysCollectionsAndMaps(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[quantifiedParityEvent](env, "QuantifiedParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[quantifiedParityEvent](env, "QuantifiedParityEvent")
	intPrimitive := Field[quantifiedParityEvent, int]("int_primitive")
	intBoxed := Field[quantifiedParityEvent, *int]("int_boxed")
	longBoxed := Field[quantifiedParityEvent, *int64]("long_boxed")
	intArray := Field[quantifiedParityEvent, []int]("int_arr")
	longCollection := Field[quantifiedParityEvent, []int64]("long_col")
	doubleBoxed := Field[quantifiedParityEvent, *float64]("double_boxed")
	bigInteger := Field[quantifiedParityEvent, big.Int]("big_integer")
	query := Select(input,
		Alias("eq_any", AnyOf(intPrimitive, QuantifierEqual, Literal(1), intBoxed)),
		Alias("eq_all", AllOf(intPrimitive, QuantifierEqual, Literal(1), intBoxed)),
		Alias("neq_any", AnyOf(intPrimitive, QuantifierNotEqual, Literal(1), intBoxed)),
		Alias("array_any", AnyOf(longBoxed, QuantifierEqual, Literal([]int64{1, 1}), intArray, longCollection)),
		Alias("array_all", AllOf(longBoxed, QuantifierGreater, Literal([]int64{0, 0}), intArray, longCollection)),
		Alias("null_any", AnyOf(intBoxed, QuantifierGreaterOrEqual, doubleBoxed, longBoxed)),
		Alias("big_any", AnyOf(bigInteger, QuantifierEqual, Literal(1))),
	).Query(StatementName("quantified-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent quantified plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("quantified result schema is missing")
	}
	for _, name := range []string{"eq_any", "eq_all", "neq_any", "array_any", "array_all", "null_any", "big_any"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != typeOf[bool]() {
			t.Fatalf("quantified result %q = %#v, want bool", name, field)
		}
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 3)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "quantified result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	boxedInt := 1
	boxedLong := int64(2)
	secondLong := int64(3)
	boxedDouble := 1.0
	if err := engine.SendEvent(context.Background(), quantifiedParityEvent{
		IntPrimitive: 1,
		IntBoxed:     &boxedInt,
		LongBoxed:    &boxedLong,
		IntArr:       []int{1, 2},
		LongCol:      []int64{1, 2},
		DoubleBoxed:  &boxedDouble,
		BigInteger:   exactMathInt("1"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), quantifiedParityEvent{
		IntPrimitive: 2,
		LongBoxed:    &secondLong,
		IntArr:       []int{1, 2},
		LongCol:      []int64{1, 2},
		BigInteger:   exactMathInt("2"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), quantifiedParityEvent{
		IntPrimitive: 0,
		IntArr:       []int{},
		LongCol:      []int64{},
		BigInteger:   exactMathInt("3"),
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("quantified rows = %d, want 3", len(rows))
	}
	want := []map[string]Value{
		{
			"eq_any": Present(true), "eq_all": Present(true), "neq_any": Present(false),
			"array_any": Present(true), "array_all": Present(false), "null_any": Present(true), "big_any": Present(true),
		},
		{
			"eq_any": Null(), "eq_all": Present(false), "neq_any": Present(true),
			"array_any": Present(false), "array_all": Present(true), "null_any": Null(), "big_any": Present(false),
		},
		{
			"eq_any": Null(), "eq_all": Present(false), "neq_any": Present(true),
			"array_any": Null(), "array_all": Null(), "null_any": Null(), "big_any": Present(false),
		},
	}
	for index, row := range rows {
		for name, expected := range want[index] {
			if !row.Get(name).Equal(expected) {
				t.Fatalf("quantified row %d %s = %v, want %v", index, name, row.Get(name), expected)
			}
		}
	}

	invalid := Select(input, Alias("bad", AnyOf(intArray, QuantifierEqual, Literal(1)))).Query(StatementName("invalid-quantified-left"))
	if _, err := env.Build(invalid); err == nil || !strings.Contains(err.Error(), "left operand must be scalar") {
		t.Fatalf("collection left invalid error = %v", err)
	}
	invalid = Select(input, Alias("bad", AnyOf(intPrimitive, QuantifierComparison(99), Literal(1)))).Query(StatementName("invalid-quantified-op"))
	if _, err := env.Build(invalid); err == nil || !strings.Contains(err.Error(), "invalid operator") {
		t.Fatalf("invalid quantified operator error = %v", err)
	}
	invalid = Select(input, Alias("bad", AnyOf(NullLiteral[int](), QuantifierEqual, Literal(1)))).Query(StatementName("invalid-quantified-null-left"))
	if _, err := env.Build(invalid); err == nil || !strings.Contains(err.Error(), "left operand") {
		t.Fatalf("null quantified left error = %v", err)
	}
	invalid = Select(input, Alias("bad", AllOf(intPrimitive, QuantifierEqual, nil))).Query(StatementName("invalid-quantified-candidate"))
	if _, err := env.Build(invalid); err == nil || !strings.Contains(err.Error(), "candidate 0 is required") {
		t.Fatalf("nil quantified candidate error = %v", err)
	}
}
