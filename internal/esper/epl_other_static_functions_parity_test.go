package esper

import (
	"context"
	"math"
	"strconv"
	"testing"
	"time"
)

// eplOtherStaticBean mirrors Java's SupportBean for EPLOtherStaticFunctions.
type eplOtherStaticBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	IntBoxed      *int   `esper:"intBoxed"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// eplOtherStaticMD mirrors SupportMarketDataBean (symbol, price, volume).
type eplOtherStaticMD struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

// eplOtherStaticS0 mirrors SupportBean_S0 (id).
type eplOtherStaticS0 struct {
	ID int64 `esper:"id"`
}

// eplOtherStaticTemp mirrors SupportTemperatureBean (geom).
type eplOtherStaticTemp struct {
	Geom string `esper:"geom"`
}

// eplOtherStaticLevelOne models the LevelZero->LevelOne chain of the Java
// EPLOtherChainedInstance execution. The field is deliberately mutable so two
// events observe different values, matching Java's static setter.
type eplOtherStaticLevelOne struct{}

var eplOtherStaticLevelOneValue string

type eplOtherStaticChainTop struct{}
type eplOtherStaticChainOne struct {
	text  string
	value int
}
type eplOtherStaticChainTwo struct {
	text string
}

func newEPLOtherStaticEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[eplOtherStaticBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[eplOtherStaticMD](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[eplOtherStaticS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[eplOtherStaticTemp](env, "SupportTemperatureBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

// eplOtherStaticCollect returns a function that accumulates projected new rows
// for one deployed statement.
func eplOtherStaticCollect(t *testing.T, stmt *Statement) func() []Row {
	t.Helper()
	var rows []Row
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func() []Row {
		return append([]Row(nil), rows...)
	}
}

func eplOtherStaticAssertRows(t *testing.T, rows []Row, field string, want []any, label string) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("%s: got %d rows, want %d: %#v", label, len(rows), len(want), rows)
	}
	for index, expected := range want {
		got := rows[index].Get(field).Any()
		if expected == nil {
			if !rows[index].Get(field).IsNull() {
				t.Fatalf("%s row %d: got %#v, want null", label, index, got)
			}
			continue
		}
		if got != expected {
			t.Fatalf("%s row %d: got %#v, want %#v", label, index, got, expected)
		}
	}
}

func eplOtherStaticDouble(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

func eplOtherStaticSumAny(values []any) float64 {
	sum := 0.0
	for _, value := range values {
		switch v := value.(type) {
		case int:
			sum += float64(v)
		case float64:
			sum += v
		case string:
			parsed, _ := strconv.ParseFloat(v, 64)
			sum += parsed
		}
	}
	return sum
}

// TestEPLOtherSingleParameterParity mirrors EPLOtherSingleParameter,
// EPLOtherSingleParameterOM and EPLOtherSingleParameterCompile: constant
// static-method calls with one argument.
func TestEPLOtherSingleParameterParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	build := func(name string, value Expression[string]) Query {
		return Select(
			From[eplOtherStaticMD](env, "SupportMarketDataBean").Window(LengthWindow(5)),
			Alias("value", value),
		).Query(StatementName(name))
	}

	t.Run("to-binary-string", func(t *testing.T) {
		plan, err := env.Build(build("s0", Func1[int, string]("toBinaryString",
			func(v int) string { return strconv.FormatInt(int64(v), 2) }, Literal(7))))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := eplOtherStaticCollect(t, deployment.Statements()[0])
		if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10, Volume: 4}); err != nil {
			t.Fatal(err)
		}
		eplOtherStaticAssertRows(t, rows(), "value", []any{"111"}, "toBinaryString")
	})

	t.Run("parse-int", func(t *testing.T) {
		plan, err := env.Build(build("s0", Func1[string, int]("parseInt",
			func(v string) int { parsed, _ := strconv.Atoi(v); return parsed }, Literal("6"))))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := eplOtherStaticCollect(t, deployment.Statements()[0])
		if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10, Volume: 4}); err != nil {
			t.Fatal(err)
		}
		eplOtherStaticAssertRows(t, rows(), "value", []any{6}, "parseInt")
	})

	t.Run("string-value-of", func(t *testing.T) {
		plan, err := env.Build(build("s0", Func1[string, string]("stringValueOf",
			func(v string) string { return v }, Literal("a"))))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := eplOtherStaticCollect(t, deployment.Statements()[0])
		if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10, Volume: 4}); err != nil {
			t.Fatal(err)
		}
		eplOtherStaticAssertRows(t, rows(), "value", []any{"a"}, "stringValueOf")
	})
}

// TestEPLOtherTwoParametersParity mirrors EPLOtherTwoParameters: static
// methods with two arguments, including mixed numeric promotion.
func TestEPLOtherTwoParametersParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(Select(
		From[eplOtherStaticMD](env, "SupportMarketDataBean").Window(LengthWindow(5)),
		Alias("maxInt", Func2[int, int, int]("maxInt",
			func(a, b int) int { return max(a, b) }, Literal(2), Literal(3))),
		Alias("maxMixed", Func2[int, float64, float64]("maxMixed",
			func(a int, b float64) float64 { return math.Max(float64(a), b) }, Literal(2), Literal(3.0))),
		Alias("parseLong", Func2[string, int, int64]("parseLong",
			func(v string, radix int) int64 { parsed, _ := strconv.ParseInt(v, radix, 64); return parsed }, Literal("123"), Literal(10))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := eplOtherStaticCollect(t, deployment.Statements()[0])
	if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10, Volume: 4}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	if len(got) != 1 {
		t.Fatalf("two-parameter rows = %#v", got)
	}
	eplOtherStaticAssertRows(t, got, "maxInt", []any{3}, "maxInt")
	eplOtherStaticAssertRows(t, got, "maxMixed", []any{3.0}, "maxMixed")
	eplOtherStaticAssertRows(t, got, "parseLong", []any{int64(123)}, "parseLong")
}

// TestEPLOtherNoParametersParity mirrors EPLOtherNoParameters' current-time
// branch: a zero-argument static call returns a fresh value per event.
func TestEPLOtherNoParametersParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(Select(
		From[eplOtherStaticMD](env, "SupportMarketDataBean").Window(LengthWindow(5)),
		Alias("currentTimeMillis", Func0[int64]("currentTimeMillis", func() int64 { return time.Now().UnixMilli() })),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := eplOtherStaticCollect(t, deployment.Statements()[0])
	before := time.Now().UnixMilli()
	if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10, Volume: 4}); err != nil {
		t.Fatal(err)
	}
	after := time.Now().UnixMilli()
	got := rows()
	if len(got) != 1 {
		t.Fatalf("no-parameter rows = %#v", got)
	}
	value, ok := got[0].Get("currentTimeMillis").Any().(int64)
	if !ok || value < before || value > after {
		t.Fatalf("currentTimeMillis = %#v, want between %d and %d", got[0].Get("currentTimeMillis").Any(), before, after)
	}
}

// TestEPLOtherUserDefinedParity mirrors EPLOtherUserDefined: a named UDF and
// a context-aware UDF that receives statement metadata.
func TestEPLOtherUserDefinedParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	var gotName, gotRuntime string
	var gotPartition int
	plan, err := env.Build(Select(
		From[eplOtherStaticMD](env, "SupportMarketDataBean").Window(LengthWindow(5)),
		Alias("static", Func1[int, int]("staticMethod",
			func(v int) int { return v }, Literal(2))),
		Alias("context", Func1Ctx[int, int]("staticMethodWithContext", func(v int, ctx EvalContext) int {
			gotName = ctx.Metadata.StatementName
			gotRuntime = ctx.Metadata.RuntimeURI
			gotPartition = ctx.Metadata.ContextPartitionID
			return v
		}, Literal(2))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := eplOtherStaticCollect(t, deployment.Statements()[0])
	if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10, Volume: 4}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	if len(got) != 1 {
		t.Fatalf("user-defined rows = %#v", got)
	}
	eplOtherStaticAssertRows(t, got, "static", []any{2}, "static")
	eplOtherStaticAssertRows(t, got, "context", []any{2}, "context")
	if gotName != "s0" || gotPartition != -1 {
		t.Fatalf("UDF context = name %q partition %d, want s0/-1", gotName, gotPartition)
	}
	if gotRuntime == "" {
		t.Fatalf("UDF runtime URI is empty")
	}
}

// TestEPLOtherComplexParametersParity mirrors EPLOtherComplexParameters:
// UDF arguments include event fields, arithmetic and nested calls.
func TestEPLOtherComplexParametersParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	price := Field[eplOtherStaticMD, float64]("price")
	volume := Field[eplOtherStaticMD, int64]("volume")
	plan, err := env.Build(Select(
		From[eplOtherStaticMD](env, "SupportMarketDataBean").Window(LengthWindow(5)),
		Alias("c0", Func1[float64, string]("stringValueOf", eplOtherStaticDouble, price)),
		Alias("c1", Func1[float64, string]("stringValueOfExpr", eplOtherStaticDouble,
			Add[float64](Literal(2.0), Multiply[float64](Literal(3.0), Literal(5.0))))),
		Alias("c2", Func1[float64, string]("stringValueOfExpr2", eplOtherStaticDouble,
			Add[float64](Multiply[float64](price, float64Expr(volume)), float64Expr(volume)))),
		Alias("c3", Func1[float64, string]("stringValueOfPow", eplOtherStaticDouble,
			Func2[float64, float64, float64]("pow", math.Pow, price,
				Func1[string, float64]("doubleValueOf", func(v string) float64 { parsed, _ := strconv.ParseFloat(v, 64); return parsed }, Literal("2"))))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := eplOtherStaticCollect(t, deployment.Statements()[0])
	if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10, Volume: 4}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	if len(got) != 1 {
		t.Fatalf("complex rows = %#v", got)
	}
	for index, want := range []string{"10.0", "17.0", "44.0", "100.0"} {
		field := "c" + strconv.Itoa(index)
		eplOtherStaticAssertRows(t, got, field, []any{want}, field)
	}
}

func float64Expr(value Expression[int64]) Expression[float64] {
	return Cast[int64, float64](value)
}

// TestEPLOtherMultipleMethodInvocationsParity mirrors
// EPLOtherMultipleMethodInvocations: several static calls in one select.
func TestEPLOtherMultipleMethodInvocationsParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	price := Field[eplOtherStaticMD, float64]("price")
	volume := Field[eplOtherStaticMD, int64]("volume")
	plan, err := env.Build(Select(
		From[eplOtherStaticMD](env, "SupportMarketDataBean").Window(LengthWindow(5)),
		Alias("c0", Func2[float64, float64, float64]("maxD", math.Max, Literal(2.0), price)),
		Alias("c1", Func2[int64, float64, float64]("maxVD",
			func(v int64, d float64) float64 { return math.Max(float64(v), d) }, volume, Literal(4.0))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := eplOtherStaticCollect(t, deployment.Statements()[0])
	if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10, Volume: 4}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	eplOtherStaticAssertRows(t, got, "c0", []any{10.0}, "c0")
	eplOtherStaticAssertRows(t, got, "c1", []any{4.0}, "c1")
}

// TestEPLOtherOtherClausesParity mirrors EPLOtherOtherClauses: UDFs in
// where, group-by, having and order-by.
func TestEPLOtherOtherClausesParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	price := Field[eplOtherStaticMD, float64]("price")
	symbol := Field[eplOtherStaticMD, string]("symbol")

	t.Run("where", func(t *testing.T) {
		plan, err := env.Build(Select(
			From[eplOtherStaticMD](env, "SupportMarketDataBean").Window(LengthWindow(5)).
				Filter(Greater[float64](Func1[float64, float64]("sqrt", math.Sqrt, price), Literal(2.0))),
			Alias("symbol", symbol),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := eplOtherStaticCollect(t, deployment.Statements()[0])
		if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "CAT", Price: 4}); err != nil {
			t.Fatal(err)
		}
		eplOtherStaticAssertRows(t, rows(), "symbol", []any{"IBM"}, "where")
	})

	t.Run("group-by", func(t *testing.T) {
		plan, err := env.Build(From[eplOtherStaticMD](env, "SupportMarketDataBean").Window(LengthWindow(5)).
			GroupBy(Func1[string, string]("groupString", func(v string) string { return v }, symbol)).
			Select(
				Alias("symbol", symbol),
				Alias("sum", Sum[float64](price)),
			).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := eplOtherStaticCollect(t, deployment.Statements()[0])
		if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 4}); err != nil {
			t.Fatal(err)
		}
		eplOtherStaticAssertRows(t, rows(), "sum", []any{10.0, 14.0}, "group-by")
	})

	t.Run("having", func(t *testing.T) {
		plan, err := env.Build(From[eplOtherStaticMD](env, "SupportMarketDataBean").Window(LengthWindow(5)).
			GroupBy(symbol).
			Select(
				Alias("symbol", symbol),
				Alias("sum", Sum[float64](price)),
			).Having(Greater[float64](Func1[float64, float64]("sqrtSum", math.Sqrt, Sum[float64](price)), Literal(3.0))).
			Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := eplOtherStaticCollect(t, deployment.Statements()[0])
		if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 100}); err != nil {
			t.Fatal(err)
		}
		eplOtherStaticAssertRows(t, rows(), "sum", []any{10.0, 110.0}, "having")
	})

	t.Run("order-by", func(t *testing.T) {
		plan, err := env.Build(Select(
			From[eplOtherStaticMD](env, "SupportMarketDataBean").Window(LengthWindow(5)),
			Alias("symbol", symbol),
			Alias("price", price),
		).Query(StatementName("s0"), WithOutput(OutputEvery(3)), OrderBy(Ascending(
			Func2[float64, float64, float64]("pow2", math.Pow, price, Literal(2.0))))))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := eplOtherStaticCollect(t, deployment.Statements()[0])
		if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "CAT", Price: 10}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "MAT", Price: 3}); err != nil {
			t.Fatal(err)
		}
		eplOtherStaticAssertRows(t, rows(), "symbol", []any{"MAT", "IBM", "CAT"}, "order-by")
	})
}

// TestEPLOtherNestedFunctionParity mirrors EPLOtherNestedFunction: a UDF
// receives another UDF's result and an event property.
func TestEPLOtherNestedFunctionParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(Select(
		From[eplOtherStaticTemp](env, "SupportTemperatureBean"),
		Alias("val", Func2[string, string, string]("appendPipe", func(a, b string) string { return a + "|" + b },
			Func1[string, string]("delimitPipe", func(v string) string { return "|" + v + "|" },
				Literal("POLYGON ((100.0 100, \", 100 100, 400 400))")),
			Field[eplOtherStaticTemp, string]("geom"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := eplOtherStaticCollect(t, deployment.Statements()[0])
	if err := engine.SendEvent(context.Background(), eplOtherStaticTemp{Geom: "a"}); err != nil {
		t.Fatal(err)
	}
	eplOtherStaticAssertRows(t, rows(), "val", []any{"|POLYGON ((100.0 100, \", 100 100, 400 400))||a"}, "nested")
}

// TestEPLOtherPassthruParity mirrors EPLOtherPassthru: one UDF passes its
// event-property argument through unchanged.
func TestEPLOtherPassthruParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(Select(
		From[eplOtherStaticS0](env, "SupportBean_S0"),
		Alias("val", Func1[int64, int64]("passthru", func(v int64) int64 { return v }, Field[eplOtherStaticS0, int64]("id"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := eplOtherStaticCollect(t, deployment.Statements()[0])
	if err := engine.SendEvent(context.Background(), eplOtherStaticS0{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), eplOtherStaticS0{ID: 2}); err != nil {
		t.Fatal(err)
	}
	eplOtherStaticAssertRows(t, rows(), "val", []any{int64(1), int64(2)}, "passthru")
}

// TestEPLOtherRuntimeExceptionParity mirrors EPLOtherRuntimeException: a
// panicking UDF is caught and projected as Null.
func TestEPLOtherRuntimeExceptionParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(Select(
		From[eplOtherStaticMD](env, "SupportMarketDataBean").Window(LengthWindow(5)),
		Alias("price", Field[eplOtherStaticMD, float64]("price")),
		Alias("value", Func0[any]("throwException", func() any { panic("boom") })),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := eplOtherStaticCollect(t, deployment.Statements()[0])
	if err := engine.SendEvent(context.Background(), eplOtherStaticMD{Symbol: "IBM", Price: 10, Volume: 4}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	if len(got) != 1 || !got[0].Get("value").IsNull() {
		t.Fatalf("runtime exception rows = %#v, want one null value", got)
	}
}

// TestEPLOtherNullPrimitiveParity mirrors EPLOtherNullPrimitive: a Null input
// is propagated through the UDF instead of panicking.
func TestEPLOtherNullPrimitiveParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(Select(
		From[eplOtherStaticBean](env, "SupportBean"),
		Alias("value", Func1[*int, int]("nullPrimitive", func(v *int) int {
			if v == nil {
				return 0
			}
			return *v + 10
		}, Field[eplOtherStaticBean, *int]("intBoxed"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := eplOtherStaticCollect(t, deployment.Statements()[0])
	if err := engine.SendEvent(context.Background(), eplOtherStaticBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	if len(got) != 1 || !got[0].Get("value").IsNull() {
		t.Fatalf("null primitive rows = %#v, want one null value", got)
	}
}

// TestEPLOtherArrayParameterParity mirrors EPLOtherArrayParameter: UDFs
// receive typed array literals, including Null elements and mixed objects.
func TestEPLOtherArrayParameterParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	sumAny := func(values []any) float64 { return eplOtherStaticSumAny(values) }
	sumFloat := func(values []float64) float64 {
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum
	}
	plan, err := env.Build(Select(
		From[eplOtherStaticBean](env, "SupportBean"),
		Alias("v1", Func1[[]any, float64]("arraySumIntBoxed", sumAny, ArrayOf[any](
			Literal(1), Literal(2), Literal[any](nil), Literal(3), Literal(4)))),
		Alias("v2", Func1[[]float64, float64]("arraySumDouble", sumFloat, ArrayOf[float64](
			Literal(1.0), Literal(2.0), Literal(3.0), Literal(4.0)))),
		Alias("v3", Func1[[]any, float64]("arraySumString", sumAny, ArrayOf[any](
			Literal("1"), Literal("2"), Literal("3"), Literal("4")))),
		Alias("v4", Func1[[]any, float64]("arraySumObject", sumAny, ArrayOf[any](
			Literal("1"), Literal(2), Literal(3.0), Literal("4.0")))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := eplOtherStaticCollect(t, deployment.Statements()[0])
	if err := engine.SendEvent(context.Background(), eplOtherStaticBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	for index, want := range []float64{10.0, 10.0, 10.0, 10.0} {
		field := "v" + strconv.Itoa(index+1)
		eplOtherStaticAssertRows(t, got, field, []any{want}, field)
	}
}

// TestEPLOtherPrimitiveConversionParity mirrors EPLOtherPrimitiveConversion:
// a primitive int is accepted through Object/Number/Comparable/Serializable
// positions.
func TestEPLOtherPrimitiveConversionParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	pass := func(v any) int { return v.(int) }
	plan, err := env.Build(Select(
		From[eplOtherStaticBean](env, "SupportBean"),
		Alias("c0", Func1[any, int]("passIntAsObject", pass, Field[eplOtherStaticBean, int]("intPrimitive"))),
		Alias("c1", Func1[any, int]("passIntAsNumber", pass, Field[eplOtherStaticBean, int]("intPrimitive"))),
		Alias("c2", Func1[any, int]("passIntAsComparable", pass, Field[eplOtherStaticBean, int]("intPrimitive"))),
		Alias("c3", Func1[any, int]("passIntAsSerializable", pass, Field[eplOtherStaticBean, int]("intPrimitive"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := eplOtherStaticCollect(t, deployment.Statements()[0])
	if err := engine.SendEvent(context.Background(), eplOtherStaticBean{TheString: "E1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	for index := 0; index < 4; index++ {
		field := "c" + strconv.Itoa(index)
		eplOtherStaticAssertRows(t, got, field, []any{10}, field)
	}
}

// TestEPLOtherChainedInstanceAndStaticParity mirrors EPLOtherChainedInstance
// and EPLOtherChainedStatic: nested UDF calls produce a chained value that
// changes with mutable external state.
func TestEPLOtherChainedInstanceAndStaticParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	t.Run("instance", func(t *testing.T) {
		plan, err := env.Build(Select(
			From[eplOtherStaticBean](env, "SupportBean"),
			Alias("val", Func1[eplOtherStaticLevelOne, string]("getLevelTwoValue",
				func(eplOtherStaticLevelOne) string { return eplOtherStaticLevelOneValue },
				Func0[eplOtherStaticLevelOne]("getLevelOne", func() eplOtherStaticLevelOne { return eplOtherStaticLevelOne{} }))),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := eplOtherStaticCollect(t, deployment.Statements()[0])
		eplOtherStaticLevelOneValue = "v1"
		if err := engine.SendEvent(context.Background(), eplOtherStaticBean{TheString: "E1"}); err != nil {
			t.Fatal(err)
		}
		eplOtherStaticLevelOneValue = "v2"
		if err := engine.SendEvent(context.Background(), eplOtherStaticBean{TheString: "E2"}); err != nil {
			t.Fatal(err)
		}
		eplOtherStaticAssertRows(t, rows(), "val", []any{"v1", "v2"}, "chained-instance")
	})

	t.Run("static", func(t *testing.T) {
		plan, err := env.Build(Select(
			From[eplOtherStaticBean](env, "SupportBean"),
			Alias("val", Func1[eplOtherStaticChainTwo, string]("getText",
				func(v eplOtherStaticChainTwo) string { return v.text },
				Func2[eplOtherStaticChainOne, string, eplOtherStaticChainTwo]("getChildTwo",
					func(v eplOtherStaticChainOne, text string) eplOtherStaticChainTwo {
						return eplOtherStaticChainTwo{text: v.text + text}
					},
					Func3[eplOtherStaticChainTop, string, int, eplOtherStaticChainOne]("getChildOne",
						func(eplOtherStaticChainTop, string, int) eplOtherStaticChainOne {
							return eplOtherStaticChainOne{text: "abc", value: 1}
						},
						Func0[eplOtherStaticChainTop]("make", func() eplOtherStaticChainTop { return eplOtherStaticChainTop{} }),
						Literal("abc"), Literal(1)),
					Literal("def")))),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := eplOtherStaticCollect(t, deployment.Statements()[0])
		if err := engine.SendEvent(context.Background(), eplOtherStaticBean{TheString: "E1"}); err != nil {
			t.Fatal(err)
		}
		eplOtherStaticAssertRows(t, rows(), "val", []any{"abcdef"}, "chained-static")
	})
}

// TestEPLOtherEscapeAndPatternParity mirrors EPLOtherEscape and the first
// branch of EPLOtherPattern: a named UDF consumes the whole event, and a UDF
// predicate drives a pattern filter.
func TestEPLOtherEscapeAndPatternParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	t.Run("escape", func(t *testing.T) {
		plan, err := env.Build(Select(
			From[eplOtherStaticBean](env, "SupportBean"),
			Alias("value", Func1[eplOtherStaticBean, string]("join",
				func(b eplOtherStaticBean) string { return b.TheString + " " + strconv.Itoa(b.IntPrimitive) },
				EventValue[eplOtherStaticBean]())),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := eplOtherStaticCollect(t, deployment.Statements()[0])
		if err := engine.SendEvent(context.Background(), eplOtherStaticBean{TheString: "E1", IntPrimitive: 99}); err != nil {
			t.Fatal(err)
		}
		eplOtherStaticAssertRows(t, rows(), "value", []any{"E1 99"}, "escape")
	})

	t.Run("pattern", func(t *testing.T) {
		delimit := Func1[eplOtherStaticBean, string]("delimitPipe",
			func(b eplOtherStaticBean) string { return "|" + b.TheString + "|" },
			EventValue[eplOtherStaticBean]())
		pattern := PatternFrom(From[eplOtherStaticBean](env, "SupportBean"), "myevent",
			Equal[string](delimit, Literal("|a|"))).Select(
			Alias("theString", TagField[string]("myevent", "theString")),
		)
		plan, err := env.Build(pattern.Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := eplOtherStaticCollect(t, deployment.Statements()[0])
		if err := engine.SendEvent(context.Background(), eplOtherStaticBean{TheString: "b"}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), eplOtherStaticBean{TheString: "a"}); err != nil {
			t.Fatal(err)
		}
		eplOtherStaticAssertRows(t, rows(), "theString", []any{"a"}, "pattern")
	})
}

// TestEPLOtherStaticFuncWCurrentTimeStampParity mirrors
// EPLOtherStaticFuncWCurrentTimeStamp: a UDF formats the current statement
// timestamp.
func TestEPLOtherStaticFuncWCurrentTimeStampParity(t *testing.T) {
	env := newEPLOtherStaticEnvironment(t)
	engine := NewEngine(env, WithStartTime(time.UnixMilli(1000)))
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(Select(
		From[eplOtherStaticBean](env, "SupportBean"),
		Alias("c0", Func1[int64, string]("formatIsoInstant",
			func(ms int64) string { return time.UnixMilli(ms).UTC().Format(time.RFC3339) },
			CurrentTimestamp())),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := eplOtherStaticCollect(t, deployment.Statements()[0])
	if err := engine.SendEvent(context.Background(), eplOtherStaticBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	eplOtherStaticAssertRows(t, rows(), "c0", []any{"1970-01-01T00:00:01Z"}, "timestamp")
}
