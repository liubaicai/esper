package esper

import (
	"context"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

type inBetweenParityEvent struct {
	Text         string         `esper:"text"`
	Number       int            `esper:"number"`
	Low          *int64         `esper:"low"`
	High         *int64         `esper:"high"`
	LongValues   []int64        `esper:"long_values"`
	ObjectValues []any          `esper:"object_values"`
	IntKeys      map[int]string `esper:"int_keys"`
}

func TestInBetweenExpressionsMatchJavaCollectionNumericBigNumberAndNullSemantics(t *testing.T) {
	if got := InOf(Literal(1), Literal(int64(2)), Literal(int16(1))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("mixed scalar IN = %v, want true", got)
	}
	if got := InOf(Literal(0), Literal(int64(2)), NullLiteral[int]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("IN with null candidate = %v, want null", got)
	}
	if got := NotInOf(Literal(0), Literal(int64(2)), NullLiteral[int]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("NOT IN with null candidate = %v, want null", got)
	}
	if got := InOf(Literal(1), Literal([]int64{2, 1})).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("typed numeric collection IN = %v, want true", got)
	}
	if got := NotInOf(Literal(1), Literal([]int64{2, 3})).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("typed numeric collection NOT IN = %v, want true", got)
	}
	if got := InOf(Literal(1), Literal(map[int64]string{1: "hit"})).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("numeric map-key IN = %v, want true", got)
	}
	if got := InOf(Literal(1), Literal([]any{float64(1), 2, nil})).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("object-array strict equality with null = %v, want null", got)
	}
	if got := InOf(Literal(2), Literal([]any{float64(1), 2, nil})).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("object-array exact integer match = %v, want true", got)
	}
	if got := InOf(Literal(9), Literal([]int{})).eval(EvalContext{}); !got.Equal(Present(false)) {
		t.Fatalf("empty collection IN = %v, want false", got)
	}
	if got := NotInOf(Literal(9), Literal(map[int]string{})).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("empty map NOT IN = %v, want true", got)
	}
	if got := InOf(Literal(true), Literal(false), Literal(true)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("boolean IN = %v, want true", got)
	}

	if got := BetweenOf(Literal("b9"), Literal("b9"), Literal("a0")).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("reversed string BETWEEN = %v, want true", got)
	}
	if got := NotBetweenOf(Literal("c"), Literal("a0"), Literal("b9")).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("string NOT BETWEEN = %v, want true", got)
	}
	if got := BetweenOf(Literal(2), Literal(int16(3)), Literal(int64(1))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("reversed mixed numeric BETWEEN = %v, want true", got)
	}
	if got := BetweenOf(Literal(inBetweenBigInt("2")), Literal(inBetweenBigInt("1")), Literal(inBetweenBigInt("3"))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("BigInteger BETWEEN = %v, want true", got)
	}
	if got := BetweenOf(Literal(inBetweenBigRat("2/3")), Literal(inBetweenBigRat("1/3")), Literal(inBetweenBigRat("1"))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("BigDecimal BETWEEN = %v, want true", got)
	}
	if got := BetweenOf(Literal(2), NullLiteral[int](), Literal(3)).eval(EvalContext{}); !got.Equal(Present(false)) {
		t.Fatalf("null BETWEEN bound = %v, want false", got)
	}
	if got := NotBetweenOf(NullLiteral[int](), Literal(1), Literal(3)).eval(EvalContext{}); !got.Equal(Present(false)) {
		t.Fatalf("null BETWEEN value = %v, want false", got)
	}
}

func TestInBetweenExpressionsBuildLiveProjectionAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[inBetweenParityEvent](env, "InBetweenParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[inBetweenParityEvent](env, "InBetweenParityEvent")
	number := Field[inBetweenParityEvent, int]("number")
	low := Field[inBetweenParityEvent, *int64]("low")
	high := Field[inBetweenParityEvent, *int64]("high")
	longValues := Field[inBetweenParityEvent, []int64]("long_values")
	objectValues := Field[inBetweenParityEvent, []any]("object_values")
	intKeys := Field[inBetweenParityEvent, map[int]string]("int_keys")
	query := Select(input,
		Alias("in", InOf(number, longValues, intKeys)),
		Alias("not_in", NotInOf(number, longValues)),
		Alias("between", BetweenOf(number, low, high)),
		Alias("object_in", InOf(number, objectValues)),
	).Query(StatementName("in-between-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent IN/BETWEEN plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	different, err := env.Build(Select(input,
		Alias("in", InOf(number, longValues, intKeys)),
		Alias("not_in", NotInOf(number, longValues)),
		Alias("between", NotBetweenOf(number, low, high)),
		Alias("object_in", InOf(number, objectValues)),
	).Query(StatementName("in-between-different")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == different.Hash() {
		t.Fatal("BETWEEN negation did not enter Plan identity")
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("IN/BETWEEN result schema is missing")
	}
	for _, name := range []string{"in", "not_in", "between", "object_in"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != typeOf[bool]() {
			t.Fatalf("IN/BETWEEN result %q = %#v, want bool", name, field)
		}
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 2)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "IN/BETWEEN result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	lowValue, highValue := int64(1), int64(3)
	if err := engine.SendEvent(context.Background(), inBetweenParityEvent{
		Number: 2, Low: &lowValue, High: &highValue, LongValues: []int64{4, 2},
		ObjectValues: []any{float64(2), 2, nil}, IntKeys: map[int]string{7: "other", 2: "hit"},
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("IN/BETWEEN rows = %d, want 1", len(rows))
	}
	for name, expected := range map[string]Value{
		"in": Present(true), "not_in": Present(false), "between": Present(true), "object_in": Present(true),
	} {
		if !rows[0].Get(name).Equal(expected) {
			t.Fatalf("IN/BETWEEN row %s = %v, want %v", name, rows[0].Get(name), expected)
		}
	}

	if err := engine.SendEvent(context.Background(), inBetweenParityEvent{Number: 9, Low: nil, High: &highValue, LongValues: []int64{4}, ObjectValues: []any{float64(9), nil}, IntKeys: nil}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[1].Get("in").Equal(Present(false)) || !rows[1].Get("not_in").Equal(Present(true)) || !rows[1].Get("between").Equal(Present(false)) || !rows[1].Get("object_in").IsNull() {
		t.Fatalf("IN/BETWEEN second row = %#v", rows)
	}
}

func TestInBetweenExpressionsRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[inBetweenParityEvent](env, "InBetweenInvalid"); err != nil {
		t.Fatal(err)
	}
	input := From[inBetweenParityEvent](env, "InBetweenInvalid")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "collection-left", expr: InOf(Literal([]int{1}), Literal(1)), want: "left operand must be scalar"},
		{name: "no-candidate", expr: InOf(Literal(1)), want: "at least one candidate"},
		{name: "nil-candidate", expr: InOf(Literal(1), nil), want: "candidate 0 is required"},
		{name: "bool-between", expr: BetweenOf(Literal(true), Literal(1), Literal(2)), want: "must be ordered"},
		{name: "nil-bound", expr: BetweenOf(Literal(1), nil, Literal(2)), want: "operand 1 is required"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("in-between-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid IN/BETWEEN error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func inBetweenBigInt(text string) big.Int {
	var value big.Int
	if _, ok := value.SetString(text, 10); !ok {
		panic("invalid BigInteger " + text)
	}
	return value
}

func inBetweenBigRat(text string) big.Rat {
	var value big.Rat
	if _, ok := value.SetString(text); !ok {
		panic("invalid BigDecimal " + text)
	}
	return value
}
