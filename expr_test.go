package esper

import (
	"testing"
	"time"
)

func TestCoreExpressionOperators(t *testing.T) {
	schema, err := StructSchema[runtimeTestTrade]("Trade")
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, runtimeTestTrade{Symbol: "ESPER", Price: 12}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	ctx := EvalContext{Event: event}
	price := Field[runtimeTestTrade, float64]("price")
	symbol := Field[runtimeTestTrade, string]("symbol")
	if got := Greater[float64](price, Literal(10.0)).eval(ctx); !got.Equal(Present(true)) {
		t.Fatalf("greater = %v", got)
	}
	if got := Between[float64](price, Literal(10.0), Literal(12.0)).eval(ctx); !got.Equal(Present(true)) {
		t.Fatalf("between = %v", got)
	}
	if got := In[string](symbol, Literal("OTHER"), Literal("ESPER")).eval(ctx); !got.Equal(Present(true)) {
		t.Fatalf("in = %v", got)
	}
	if got := Like(symbol, Literal("ESP%R")).eval(ctx); !got.Equal(Present(true)) {
		t.Fatalf("like = %v", got)
	}
	if got := RegexpMatch(symbol, Literal("ES.*")).eval(ctx); !got.Equal(Present(true)) {
		t.Fatalf("regexp = %v", got)
	}
	if got := Add[float64](price, Literal(3.0)).eval(ctx); !got.Equal(Present(15.0)) {
		t.Fatalf("add = %v", got)
	}
	if got := Divide[float64](price, Literal(0.0)).eval(ctx); !got.IsNull() {
		t.Fatalf("divide by zero = %v", got)
	}
	if got := Lower(symbol).eval(ctx); !got.Equal(Present("esper")) {
		t.Fatalf("lower = %v", got)
	}
	if got := StringLength(symbol).eval(ctx); !got.Equal(Present(int64(5))) {
		t.Fatalf("string length = %v", got)
	}
	if got := Contains(symbol, Literal("PER")).eval(ctx); !got.Equal(Present(true)) {
		t.Fatalf("contains = %v", got)
	}
}

func TestExpressionNullPropagationAndCoalesce(t *testing.T) {
	value := NullLiteral[string]()
	if got := Coalesce[string](value, Literal("fallback")).eval(EvalContext{}); !got.Equal(Present("fallback")) {
		t.Fatalf("coalesce = %v", got)
	}
	if got := Like(value, Literal("x%")).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("like null = %v", got)
	}
	if got := And(IsNull[string](value), NullLiteral[bool]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("three-valued expression = %v", got)
	}
}

func TestExpressionArithmeticConditionalAndTimeFunctions(t *testing.T) {
	now := time.Date(2026, time.August, 4, 12, 34, 56, 789000000, time.UTC)
	ctx := EvalContext{Now: now}
	if got := Modulo[float64](Literal(10.0), Literal(3.0)).eval(ctx); !got.Equal(Present(1.0)) {
		t.Fatalf("modulo = %v", got)
	}
	if got := Negate[float64](Literal(4.0)).eval(ctx); !got.Equal(Present(-4.0)) {
		t.Fatalf("negate = %v", got)
	}
	if got := Concat(Literal("es"), Literal("per")).eval(ctx); !got.Equal(Present("esper")) {
		t.Fatalf("concat = %v", got)
	}
	if got := IfThenElse[string](Literal(true), Literal("yes"), Literal("no")).eval(ctx); !got.Equal(Present("yes")) {
		t.Fatalf("if = %v", got)
	}
	current := CurrentTime()
	if got := current.eval(ctx); !got.Equal(Present(now)) {
		t.Fatalf("current time = %v", got)
	}
	if got := Year(current).eval(ctx); !got.Equal(Present(int64(2026))) {
		t.Fatalf("year = %v", got)
	}
	if got := Month(current).eval(ctx); !got.Equal(Present(int64(8))) {
		t.Fatalf("month = %v", got)
	}
	if got := DayOfMonth(current).eval(ctx); !got.Equal(Present(int64(4))) {
		t.Fatalf("day = %v", got)
	}
	if got := UnixMillis(current).eval(ctx); !got.Equal(Present(now.UnixNano() / int64(time.Millisecond))) {
		t.Fatalf("unix millis = %v", got)
	}
}

func TestExpressionCastExistenceTypeAndArrayAccess(t *testing.T) {
	if got := Cast[float64, int64](Literal(12.75)).eval(EvalContext{}); !got.Equal(Present(int64(12))) {
		t.Fatalf("cast numeric = %v", got)
	}
	if got := Cast[string, float64](Literal("12.5")).eval(EvalContext{}); !got.Equal(Present(12.5)) {
		t.Fatalf("cast string numeric = %v", got)
	}
	if got := Exists(NullLiteral[string]()).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("exists null = %v", got)
	}
	schema, err := StructSchema[runtimeTestTrade]("Trade")
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, runtimeTestTrade{}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got := Exists(Field[runtimeTestTrade, string]("missing")).eval(EvalContext{Event: event}); !got.Equal(Present(false)) {
		t.Fatalf("exists missing = %v", got)
	}
	if got := TypeName(Literal(int64(3))).eval(EvalContext{}); !got.Equal(Present("int64")) {
		t.Fatalf("type-of = %v", got)
	}
	if got := InstanceOf[int64](Literal(int64(3))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("instance-of = %v", got)
	}
	items := Literal([]string{"a", "b"})
	if got := ArrayAt[string](items, Literal(int64(1))).eval(EvalContext{}); !got.Equal(Present("b")) {
		t.Fatalf("array-at = %v", got)
	}
	if got := ArrayAt[string](items, Literal(int64(2))).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("array-at out of range = %v", got)
	}
}
