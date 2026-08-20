package esper

import (
	"context"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCastExpressionsMatchJavaNumericBooleanBigNumberAndDateSemantics(t *testing.T) {
	if got := Cast[string, int](Literal("0x0A")).eval(EvalContext{}); !got.Equal(Present(10)) {
		t.Fatalf("string integer cast = %v, want 10", got)
	}
	if got := Cast[float64, int16](Literal(12.75)).eval(EvalContext{}); !got.Equal(Present(int16(12))) {
		t.Fatalf("floating integer cast = %v, want 12", got)
	}
	if got := Cast[string, bool](Literal("true")).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("string boolean cast = %v, want true", got)
	}
	if got := Cast[string, rune](Literal("x")).eval(EvalContext{}); !got.Equal(Present(rune('x'))) {
		t.Fatalf("string rune cast = %v, want x", got)
	}
	if got := Cast[string, rune](Literal("true")).eval(EvalContext{}); !got.Equal(Present(rune('t'))) {
		t.Fatalf("multi-character string rune cast = %v, want t", got)
	}
	if got := Cast[string, big.Int](Literal("156.78")).eval(EvalContext{}); !got.Equal(Present(exactCastInt("156"))) {
		t.Fatalf("BigInteger cast = %v, want 156", got)
	}
	if got := Cast[string, big.Rat](Literal("1.25")).eval(EvalContext{}); !got.Equal(Present(exactCastRat("5/4"))) {
		t.Fatalf("BigDecimal cast = %v, want 5/4", got)
	}
	if got := Cast[[]any, []int](Literal([]any{1, int64(2), "3"})).eval(EvalContext{}); !got.Equal(Present([]int{1, 2, 3})) {
		t.Fatalf("recursive array cast = %v, want [1 2 3]", got)
	}
	if got := Cast[string, time.Time](Literal("2026-08-06T12:34:56.123Z")).eval(EvalContext{}); !got.Equal(Present(time.Date(2026, 8, 6, 12, 34, 56, 123000000, time.UTC))) {
		t.Fatalf("RFC3339 time cast = %v", got)
	}
	if got := CastWithLayout[string, time.Time](Literal("20260806"), "20060102").eval(EvalContext{}); !got.Equal(Present(time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC))) {
		t.Fatalf("custom-layout time cast = %v", got)
	}
	if got := CastWithFormat[string, time.Time](Literal("20260806123456"), Literal("20060102150405")).eval(EvalContext{}); !got.Equal(Present(time.Date(2026, 8, 6, 12, 34, 56, 0, time.UTC))) {
		t.Fatalf("dynamic-layout time cast = %v", got)
	}
	epoch := time.Date(2026, 8, 6, 12, 34, 56, 0, time.UTC)
	if got := Cast[int64, time.Time](Literal(epoch.UnixMilli())).eval(EvalContext{}); !got.Equal(Present(epoch)) {
		t.Fatalf("epoch time cast = %v, want %v", got, epoch)
	}
	var nilValue *int
	if got := Cast[*int, int](Literal(nilValue)).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("typed nil cast = %v, want null", got)
	}
	if got := Cast[time.Time, string](Literal(epoch)).eval(EvalContext{}); !got.Equal(Present(epoch.Format(time.RFC3339Nano))) {
		t.Fatalf("time string cast = %v", got)
	}
}

type castExpressionParityEvent struct {
	Raw        any    `esper:"raw"`
	BoolText   string `esper:"bool_text"`
	Numbers    []any  `esper:"numbers"`
	DateText   string `esper:"date_text"`
	DateLayout string `esper:"date_layout"`
}

func TestCastExpressionsBuildLiveProjectionAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[castExpressionParityEvent](env, "CastExpressionParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[castExpressionParityEvent](env, "CastExpressionParityEvent")
	raw := Field[castExpressionParityEvent, any]("raw")
	boolText := Field[castExpressionParityEvent, string]("bool_text")
	numbers := Field[castExpressionParityEvent, []any]("numbers")
	dateText := Field[castExpressionParityEvent, string]("date_text")
	dateLayout := Field[castExpressionParityEvent, string]("date_layout")
	query := Select(input,
		Alias("number", Cast[any, int64](raw)),
		Alias("boolean", Cast[string, bool](boolText)),
		Alias("numbers", Cast[[]any, []int](numbers)),
		Alias("date", CastWithFormat[string, time.Time](dateText, dateLayout)),
	).Query(StatementName("cast-expression-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent cast plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	different, err := env.Build(Select(input,
		Alias("number", Cast[any, int64](raw)),
		Alias("boolean", Cast[string, bool](boolText)),
		Alias("numbers", Cast[[]any, []int](numbers)),
		Alias("date", CastWithLayout[string, time.Time](dateText, "20060102")),
	).Query(StatementName("cast-expression-parity")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == different.Hash() {
		t.Fatal("cast format did not enter Plan identity")
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("cast result schema is missing")
	}
	wantTypes := map[string]reflect.Type{
		"number": typeOf[int64](), "boolean": typeOf[bool](), "numbers": typeOf[[]int](), "date": typeOf[time.Time](),
	}
	for name, want := range wantTypes {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != want {
			t.Fatalf("cast result field %q = %#v, want %s", name, field, want)
		}
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "cast result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), castExpressionParityEvent{
		Raw: "42", BoolText: "true", Numbers: []any{1, int64(2), "3"}, DateText: "20260806123456", DateLayout: "20060102150405",
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("cast rows = %d, want 1", len(rows))
	}
	wantDate := time.Date(2026, 8, 6, 12, 34, 56, 0, time.UTC)
	if !rows[0].Get("number").Equal(Present(int64(42))) ||
		!rows[0].Get("boolean").Equal(Present(true)) ||
		!rows[0].Get("numbers").Equal(Present([]int{1, 2, 3})) ||
		!rows[0].Get("date").Equal(Present(wantDate)) {
		t.Fatalf("cast projection row = %#v", rows[0].AsMap())
	}
}

func TestCastExpressionsRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[castExpressionParityEvent](env, "CastExpressionInvalidEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[castExpressionParityEvent](env, "CastExpressionInvalidEvent")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "nil-value", expr: Cast[int, int](nil), want: "requires a value expression"},
		{name: "empty-layout", expr: CastWithLayout[string, time.Time](Literal("20260806"), ""), want: "layout must not be empty"},
		{name: "nil-format", expr: CastWithFormat[string, time.Time](Literal("20260806"), nil), want: "format requires a layout expression"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("cast-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid cast error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func exactCastInt(text string) big.Int {
	var result big.Int
	if _, ok := result.SetString(text, 10); !ok {
		panic("invalid cast integer " + text)
	}
	return result
}

func exactCastRat(text string) big.Rat {
	var result big.Rat
	if _, ok := result.SetString(text); !ok {
		panic("invalid cast rational " + text)
	}
	return result
}
