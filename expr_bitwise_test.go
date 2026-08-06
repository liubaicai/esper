package esper

import (
	"context"
	"testing"
)

type bitwiseParityEvent struct {
	ByteLeft   int8  `esper:"byte_left"`
	ByteRight  int8  `esper:"byte_right"`
	ShortLeft  int16 `esper:"short_left"`
	ShortRight int16 `esper:"short_right"`
	IntLeft    int32 `esper:"int_left"`
	IntRight   int32 `esper:"int_right"`
	LongLeft   int64 `esper:"long_left"`
	LongRight  int64 `esper:"long_right"`
	BoolLeft   bool  `esper:"bool_left"`
	BoolRight  bool  `esper:"bool_right"`
}

type bitwiseBoxedEvent struct {
	Primitive int8  `esper:"primitive"`
	Boxed     *int8 `esper:"boxed"`
	Bool      bool  `esper:"bool"`
	BoolBoxed *bool `esper:"bool_boxed"`
}

func TestBitwiseExpressionsPreserveIntegerWidthAndBooleanSemantics(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[bitwiseParityEvent](env, "BitwiseParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[bitwiseParityEvent](env, "BitwiseParityEvent")
	plan, err := env.Build(Select(input,
		Alias("byte_and", BitwiseAnd[int8](Field[bitwiseParityEvent, int8]("byte_left"), Field[bitwiseParityEvent, int8]("byte_right"))),
		Alias("short_or", BitwiseOr[int16](Field[bitwiseParityEvent, int16]("short_left"), Field[bitwiseParityEvent, int16]("short_right"))),
		Alias("int_or", BinaryOr[int32](Field[bitwiseParityEvent, int32]("int_left"), Field[bitwiseParityEvent, int32]("int_right"))),
		Alias("long_xor", BitwiseXor[int64](Field[bitwiseParityEvent, int64]("long_left"), Field[bitwiseParityEvent, int64]("long_right"))),
		Alias("bool_and", BitwiseAnd[bool](Field[bitwiseParityEvent, bool]("bool_left"), Field[bitwiseParityEvent, bool]("bool_right"))),
		Alias("bool_or", BitwiseOr[bool](Field[bitwiseParityEvent, bool]("bool_left"), Field[bitwiseParityEvent, bool]("bool_right"))),
		Alias("bool_xor", BinaryXor[bool](Field[bitwiseParityEvent, bool]("bool_left"), Field[bitwiseParityEvent, bool]("bool_right"))),
	).Query(StatementName("bitwise-parity")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var output Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		var ok bool
		output, ok = batch.New[0].Row()
		if !ok {
			t.Fatalf("bitwise result is not a row: %#v", batch.New[0])
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), bitwiseParityEvent{
		ByteLeft: 1, ByteRight: 3,
		ShortLeft: 2, ShortRight: 4,
		IntLeft: 1, IntRight: 2,
		LongLeft: 3, LongRight: 4,
		BoolLeft: false, BoolRight: true,
	}); err != nil {
		t.Fatal(err)
	}

	assertBitwiseValue(t, output.Get("byte_and"), int8(1))
	assertBitwiseValue(t, output.Get("short_or"), int16(6))
	assertBitwiseValue(t, output.Get("int_or"), int32(3))
	assertBitwiseValue(t, output.Get("long_xor"), int64(7))
	assertBitwiseValue(t, output.Get("bool_and"), false)
	assertBitwiseValue(t, output.Get("bool_or"), true)
	assertBitwiseValue(t, output.Get("bool_xor"), true)
}

func TestBitwiseExpressionsPropagateNullAndRejectInvalidBuilders(t *testing.T) {
	if got := BitwiseAnd[int8](NullLiteral[int8](), Literal[int8](1)).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("numeric bitwise null = %v", got)
	}
	if got := BitwiseOr[bool](Literal(true), NullLiteral[bool]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("boolean bitwise null = %v", got)
	}
	if got := BitwiseXor[uint16](Literal[uint16](0x55), Literal[uint16](0xff)).eval(EvalContext{}); !got.Equal(Present(uint16(0xaa))) {
		t.Fatalf("unsigned bitwise xor = %v", got)
	}

	env := NewEnvironment()
	if _, err := RegisterStruct[bitwiseParityEvent](env, "BitwiseInvalidEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[bitwiseParityEvent](env, "BitwiseInvalidEvent")
	invalid := Select(input,
		Alias("bad", BitwiseAnd[int8](nil, Literal[int8](1))),
	).Query(StatementName("bitwise-invalid"))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("nil bitwise operand must fail during Build")
	}
	wrongType := Select(input,
		Alias("bad", BitwiseAndOf[int8](Literal("x"), Literal("y"))),
	).Query(StatementName("bitwise-wrong-type"))
	if _, err := env.Build(wrongType); err == nil {
		t.Fatal("incompatible BitwiseAndOf operand must fail during Build")
	}
}

func TestBitwiseExpressionsHandlePrimitiveAndBoxedOperands(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[bitwiseBoxedEvent](env, "BitwiseBoxedEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[bitwiseBoxedEvent](env, "BitwiseBoxedEvent")
	plan, err := env.Build(Select(input,
		Alias("numeric", BitwiseAndOf[int8](
			Field[bitwiseBoxedEvent, int8]("primitive"),
			Field[bitwiseBoxedEvent, *int8]("boxed"),
		)),
		Alias("boolean", BinaryAndOf[bool](
			Field[bitwiseBoxedEvent, bool]("bool"),
			Field[bitwiseBoxedEvent, *bool]("bool_boxed"),
		)),
	).Query(StatementName("bitwise-boxed")))
	if err != nil {
		t.Fatal(err)
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
				return NewError(ErrorTypeMismatch, "boxed bitwise result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	boxed := int8(3)
	boxedBool := true
	if err := engine.SendEvent(context.Background(), bitwiseBoxedEvent{Primitive: 1, Boxed: &boxed, Bool: true, BoolBoxed: &boxedBool}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), bitwiseBoxedEvent{Primitive: 1, Bool: false}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("boxed bitwise rows = %d, want 2", len(rows))
	}
	assertBitwiseValue(t, rows[0].Get("numeric"), int8(1))
	assertBitwiseValue(t, rows[0].Get("boolean"), true)
	if !rows[1].Get("numeric").IsNull() || !rows[1].Get("boolean").IsNull() {
		t.Fatalf("boxed null propagation = numeric=%v boolean=%v", rows[1].Get("numeric"), rows[1].Get("boolean"))
	}
}

func assertBitwiseValue[T comparable](t *testing.T, value Value, want T) {
	t.Helper()
	got, err := As[T](value)
	if err != nil || got != want {
		t.Fatalf("bitwise value = %#v (%v), want %#v", value, err, want)
	}
}
