package esper

import (
	"context"
	"math"
	"math/big"
	"testing"
)

type resultsetAggregateFilteredAllParityBean struct {
	IntBoxed        *int    `esper:"intBoxed"`
	BoolPrimitive   bool    `esper:"boolPrimitive"`
	FloatPrimitive  float32 `esper:"floatPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
	ShortPrimitive  int16   `esper:"shortPrimitive"`
}

type resultsetAggregateFilteredAllParityNumeric struct {
	BigInt big.Int `esper:"bigint"`
	BigDec big.Rat `esper:"bigdec"`
}

func collectResultSetAggregateFilteredAllRows(t *testing.T, statement *Statement) (*[]Row, *int) {
	t.Helper()
	rows := make([]Row, 0)
	oldRows := 0
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		oldRows += len(batch.Old)
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("aggregate result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &rows, &oldRows
}

func buildResultSetAggregateFilteredAllBeanPlan(t *testing.T, env *Environment, caseName string) Query {
	t.Helper()
	stream := From[resultsetAggregateFilteredAllParityBean](env, "SupportBean")
	predicate := Field[resultsetAggregateFilteredAllParityBean, bool]("boolPrimitive")
	intBoxed := Cast[*int, int](Field[resultsetAggregateFilteredAllParityBean, *int]("intBoxed"))
	switch caseName {
	case "filtered-length3-all":
		stream = stream.Window(LengthWindow(3))
		return stream.Aggregate(
			Alias("cavedev", FilterAggregate[float64](Avedev[int](intBoxed), predicate)),
			Alias("cavg", FilterAggregate[float64](Avg[int](intBoxed), predicate)),
			Alias("cmax", FilterAggregate[int](Max[int](intBoxed), predicate)),
			Alias("cmedian", FilterAggregate[float64](Median[int](intBoxed), predicate)),
			Alias("cmin", FilterAggregate[int](Min[int](intBoxed), predicate)),
			Alias("cstddev", FilterAggregate[float64](StdDev[int](intBoxed), predicate)),
			Alias("csum", FilterAggregate[int](Sum[int](intBoxed), predicate)),
			Alias("cfmaxever", FilterAggregate[int](MaxEver[int](intBoxed), predicate)),
			Alias("cfminever", FilterAggregate[int](MinEver[int](intBoxed), predicate)),
		).Query(StatementName("s0"))
	case "filtered-stateless":
		return stream.Aggregate(
			Alias("c1", FilterAggregate[int](Max[int](intBoxed), predicate)),
			Alias("c2", FilterAggregate[int](Min[int](intBoxed), predicate)),
		).Query(StatementName("s0"))
	case "filtered-distinct", "filtered-distinct-epl", "filtered-distinct-soda":
		stream = stream.Window(LengthWindow(3))
		return stream.Aggregate(
			Alias("cavedev", FilterAggregate[float64](DistinctAggregate[float64](Avedev[int](intBoxed), intBoxed), predicate)),
			Alias("cavg", FilterAggregate[float64](DistinctAggregate[float64](Avg[int](intBoxed), intBoxed), predicate)),
			Alias("cmax", FilterAggregate[int](DistinctAggregate[int](Max[int](intBoxed), intBoxed), predicate)),
			Alias("cmedian", FilterAggregate[float64](DistinctAggregate[float64](Median[int](intBoxed), intBoxed), predicate)),
			Alias("cmin", FilterAggregate[int](DistinctAggregate[int](Min[int](intBoxed), intBoxed), predicate)),
			Alias("cstddev", FilterAggregate[float64](DistinctAggregate[float64](StdDev[int](intBoxed), intBoxed), predicate)),
			Alias("csum", FilterAggregate[int](DistinctAggregate[int](Sum[int](intBoxed), intBoxed), predicate)),
		).Query(StatementName("s0"))
	default:
		t.Fatalf("unsupported bean case %q", caseName)
		return Query{}
	}
}

func deployResultSetAggregateFilteredAllBean(t *testing.T, caseName string) (*Engine, *Statement) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[resultsetAggregateFilteredAllParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	query := buildResultSetAggregateFilteredAllBeanPlan(t, env, caseName)
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		_ = engine.Close(context.Background())
		t.Fatal(err)
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		_ = engine.Close(context.Background())
		t.Fatalf("statement count = %d, want 1", len(statements))
	}
	return engine, statements[0]
}

func resultsetAggregateFilteredAllBeanEvent(value *int, flag bool) resultsetAggregateFilteredAllParityBean {
	return resultsetAggregateFilteredAllParityBean{IntBoxed: value, BoolPrimitive: flag}
}

func resultsetAggregateFilteredAllInt(value int) *int { return &value }

func TestResultSetAggregateFilteredAllLength3FunctionsParity(t *testing.T) {
	engine, statement := deployResultSetAggregateFilteredAllBean(t, "filtered-length3-all")
	defer func() { _ = engine.Close(context.Background()) }()
	rowBuffer, oldRows := collectResultSetAggregateFilteredAllRows(t, statement)

	events := []resultsetAggregateFilteredAllParityBean{
		resultsetAggregateFilteredAllBeanEvent(resultsetAggregateFilteredAllInt(100), false),
		resultsetAggregateFilteredAllBeanEvent(resultsetAggregateFilteredAllInt(10), true),
		resultsetAggregateFilteredAllBeanEvent(resultsetAggregateFilteredAllInt(11), false),
		resultsetAggregateFilteredAllBeanEvent(resultsetAggregateFilteredAllInt(20), true),
		resultsetAggregateFilteredAllBeanEvent(resultsetAggregateFilteredAllInt(30), true),
	}
	for _, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	rows := *rowBuffer
	if *oldRows != 0 || len(rows) != len(events) {
		t.Fatalf("length3 rows=%d old=%d, want %d new rows and no old rows", len(rows), *oldRows, len(events))
	}

	if !rows[0].Get("cavg").IsNull() || !rows[0].Get("cstddev").IsNull() || !rows[0].Get("cfmaxever").IsNull() {
		t.Fatalf("empty filtered row = %#v", rows[0].AsMap())
	}
	if rows[1].Get("cavg").Any() != float64(10) || !rows[1].Get("cstddev").IsNull() || rows[1].Get("csum").Any() != int(10) {
		t.Fatalf("singleton filtered row = %#v", rows[1].AsMap())
	}
	if rows[3].Get("cavedev").Any() != float64(5) || rows[3].Get("cavg").Any() != float64(15) ||
		rows[3].Get("cmax").Any() != int(20) || rows[3].Get("cmedian").Any() != float64(15) ||
		rows[3].Get("cmin").Any() != int(10) || rows[3].Get("csum").Any() != int(30) ||
		rows[3].Get("cfmaxever").Any() != int(20) || rows[3].Get("cfminever").Any() != int(10) {
		t.Fatalf("two-value filtered row = %#v", rows[3].AsMap())
	}
	if got, ok := rows[3].Get("cstddev").Any().(float64); !ok || math.Abs(got-math.Sqrt(50)) > 1e-12 {
		t.Fatalf("two-value stddev = %#v", rows[3].Get("cstddev").Any())
	}
	if rows[4].Get("cavg").Any() != float64(25) || rows[4].Get("cmin").Any() != int(20) ||
		rows[4].Get("cmax").Any() != int(30) || rows[4].Get("cfminever").Any() != int(10) {
		t.Fatalf("post-eviction filtered row = %#v", rows[4].AsMap())
	}
}

func TestResultSetAggregateFilteredAllNumericWidthsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[resultsetAggregateFilteredAllParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	predicate := Field[resultsetAggregateFilteredAllParityBean, bool]("boolPrimitive")
	stream := From[resultsetAggregateFilteredAllParityBean](env, "SupportBean").Window(LengthWindow(2))
	plan, err := env.Build(stream.Aggregate(
		Alias("c1", FilterAggregate[float32](Sum[float32](Field[resultsetAggregateFilteredAllParityBean, float32]("floatPrimitive")), predicate)),
		Alias("c2", FilterAggregate[float64](Sum[float64](Field[resultsetAggregateFilteredAllParityBean, float64]("doublePrimitive")), predicate)),
		Alias("c3", FilterAggregate[int64](Sum[int64](Field[resultsetAggregateFilteredAllParityBean, int64]("longPrimitive")), predicate)),
		Alias("c4", FilterAggregate[int16](Sum[int16](Field[resultsetAggregateFilteredAllParityBean, int16]("shortPrimitive")), predicate)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rowBuffer, oldRows := collectResultSetAggregateFilteredAllRows(t, deployment.Statements()[0])
	events := []resultsetAggregateFilteredAllParityBean{
		{BoolPrimitive: false, FloatPrimitive: 2, DoublePrimitive: 3, LongPrimitive: 4, ShortPrimitive: 5},
		{BoolPrimitive: true, FloatPrimitive: 3, DoublePrimitive: 4, LongPrimitive: 5, ShortPrimitive: 6},
		{BoolPrimitive: true, FloatPrimitive: 4, DoublePrimitive: 5, LongPrimitive: 6, ShortPrimitive: 7},
		{BoolPrimitive: true, FloatPrimitive: 1, DoublePrimitive: 1, LongPrimitive: 1, ShortPrimitive: 1},
	}
	for _, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	rows := *rowBuffer
	if *oldRows != 0 || len(rows) != len(events) {
		t.Fatalf("numeric-width rows=%d old=%d, want %d new rows and no old rows", len(rows), *oldRows, len(events))
	}

	if !rows[0].Get("c1").IsNull() || rows[1].Get("c1").Any() != float32(3) || rows[1].Get("c2").Any() != float64(4) || rows[1].Get("c3").Any() != int64(5) || rows[1].Get("c4").Any() != int16(6) {
		t.Fatalf("numeric-width initial rows = %#v / %#v", rows[0].AsMap(), rows[1].AsMap())
	}
	if rows[2].Get("c1").Any() != float32(7) || rows[2].Get("c2").Any() != float64(9) || rows[2].Get("c3").Any() != int64(11) || rows[2].Get("c4").Any() != int16(13) ||
		rows[3].Get("c1").Any() != float32(5) || rows[3].Get("c2").Any() != float64(6) || rows[3].Get("c3").Any() != int64(7) || rows[3].Get("c4").Any() != int16(8) {
		t.Fatalf("numeric-width rows = %#v / %#v", rows[2].AsMap(), rows[3].AsMap())
	}
}

func TestResultSetAggregateFilteredAllStatelessParity(t *testing.T) {
	engine, statement := deployResultSetAggregateFilteredAllBean(t, "filtered-stateless")
	defer func() { _ = engine.Close(context.Background()) }()
	rowBuffer, oldRows := collectResultSetAggregateFilteredAllRows(t, statement)
	for _, event := range []resultsetAggregateFilteredAllParityBean{
		{IntBoxed: resultsetAggregateFilteredAllInt(10), BoolPrimitive: true},
		{IntBoxed: resultsetAggregateFilteredAllInt(20), BoolPrimitive: true},
		{IntBoxed: resultsetAggregateFilteredAllInt(8), BoolPrimitive: false},
		{IntBoxed: resultsetAggregateFilteredAllInt(7), BoolPrimitive: true},
		{IntBoxed: resultsetAggregateFilteredAllInt(30), BoolPrimitive: false},
		{IntBoxed: resultsetAggregateFilteredAllInt(40), BoolPrimitive: true},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	rows := *rowBuffer
	if *oldRows != 0 || len(rows) != 6 {
		t.Fatalf("stateless rows=%d old=%d, want 6 new rows and no old rows", len(rows), *oldRows)
	}

	wantMax := []int{10, 20, 20, 20, 20, 40}
	wantMin := []int{10, 10, 10, 7, 7, 7}
	for index := range rows {
		if rows[index].Get("c1").Any() != wantMax[index] || rows[index].Get("c2").Any() != wantMin[index] {
			t.Fatalf("stateless row %d = %#v", index, rows[index].AsMap())
		}
	}
}

func TestResultSetAggregateFilteredAllExactBigParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[resultsetAggregateFilteredAllParityNumeric](env, "SupportBeanNumeric"); err != nil {
		t.Fatal(err)
	}
	integer := Field[resultsetAggregateFilteredAllParityNumeric, big.Int]("bigint")
	decimal := Field[resultsetAggregateFilteredAllParityNumeric, big.Rat]("bigdec")
	filter := LessExact[big.Int](integer, Literal(resultsetAggregateFilteredAllBigInt("100")))
	plan, err := env.Build(From[resultsetAggregateFilteredAllParityNumeric](env, "SupportBeanNumeric").Window(LengthWindow(2)).Aggregate(
		Alias("c1", FilterAggregate[big.Rat](AvgExact[big.Rat](decimal), filter)),
		Alias("c2", FilterAggregate[big.Rat](SumExact[big.Rat](decimal), filter)),
		Alias("c3", FilterAggregate[big.Int](SumExact[big.Int](integer), filter)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rowBuffer, oldRows := collectResultSetAggregateFilteredAllRows(t, deployment.Statements()[0])
	events := []resultsetAggregateFilteredAllParityNumeric{
		{BigInt: resultsetAggregateFilteredAllBigInt("10"), BigDec: resultsetAggregateFilteredAllBigRat("20")},
		{BigInt: resultsetAggregateFilteredAllBigInt("101"), BigDec: resultsetAggregateFilteredAllBigRat("101")},
		{BigInt: resultsetAggregateFilteredAllBigInt("20"), BigDec: resultsetAggregateFilteredAllBigRat("40")},
		{BigInt: resultsetAggregateFilteredAllBigInt("30"), BigDec: resultsetAggregateFilteredAllBigRat("50")},
	}
	for _, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	rows := *rowBuffer
	if *oldRows != 0 || len(rows) != len(events) {
		t.Fatalf("exact rows=%d old=%d, want %d new rows and no old rows", len(rows), *oldRows, len(events))
	}

	assertResultSetAggregateFilteredAllRat(t, rows[0].Get("c1"), "20")
	assertResultSetAggregateFilteredAllRat(t, rows[0].Get("c2"), "20")
	assertResultSetAggregateFilteredAllInt(t, rows[0].Get("c3"), "10")
	assertResultSetAggregateFilteredAllRat(t, rows[1].Get("c1"), "20")
	assertResultSetAggregateFilteredAllRat(t, rows[1].Get("c2"), "20")
	assertResultSetAggregateFilteredAllInt(t, rows[1].Get("c3"), "10")
	assertResultSetAggregateFilteredAllRat(t, rows[2].Get("c1"), "40")
	assertResultSetAggregateFilteredAllRat(t, rows[2].Get("c2"), "40")
	assertResultSetAggregateFilteredAllInt(t, rows[2].Get("c3"), "20")
	assertResultSetAggregateFilteredAllRat(t, rows[3].Get("c1"), "45")
	assertResultSetAggregateFilteredAllRat(t, rows[3].Get("c2"), "90")
	assertResultSetAggregateFilteredAllInt(t, rows[3].Get("c3"), "50")
}

func resultsetAggregateFilteredAllBigInt(text string) big.Int {
	var value big.Int
	if _, ok := value.SetString(text, 10); !ok {
		panic("invalid integer " + text)
	}
	return value
}

func resultsetAggregateFilteredAllBigRat(text string) big.Rat {
	var value big.Rat
	if _, ok := value.SetString(text); !ok {
		panic("invalid rational " + text)
	}
	return value
}

func assertResultSetAggregateFilteredAllRat(t *testing.T, value Value, expected string) {
	t.Helper()
	actual, err := As[big.Rat](value)
	if err != nil || !value.IsPresent() {
		t.Fatalf("rational = %v (%v), want %s", actual, err, expected)
	}
	want := resultsetAggregateFilteredAllBigRat(expected)
	if actual.Cmp(&want) != 0 {
		t.Fatalf("rational = %s, want %s", actual.RatString(), want.RatString())
	}
}

func assertResultSetAggregateFilteredAllInt(t *testing.T, value Value, expected string) {
	t.Helper()
	actual, err := As[big.Int](value)
	if err != nil || !value.IsPresent() {
		t.Fatalf("integer = %v (%v), want %s", actual, err, expected)
	}
	want := resultsetAggregateFilteredAllBigInt(expected)
	if actual.Cmp(&want) != 0 {
		t.Fatalf("integer = %s, want %s", actual.String(), want.String())
	}
}

func TestResultSetAggregateFilteredAllDistinctParity(t *testing.T) {
	for _, caseName := range []string{"filtered-distinct-epl", "filtered-distinct-soda"} {
		t.Run(caseName, func(t *testing.T) {
			// Java deploys this same typed projection through EPL and SODA. Go has
			// one typed object-model builder, so both lifecycle labels exercise the
			// same plan shape without introducing a parser-only duplicate API.
			engine, statement := deployResultSetAggregateFilteredAllBean(t, caseName)
			defer func() { _ = engine.Close(context.Background()) }()
			rowBuffer, oldRows := collectResultSetAggregateFilteredAllRows(t, statement)
			for _, value := range []int{100, 100, 200, 200, 200} {
				if err := engine.SendEvent(context.Background(), resultsetAggregateFilteredAllParityBean{IntBoxed: resultsetAggregateFilteredAllInt(value), BoolPrimitive: true}); err != nil {
					t.Fatal(err)
				}
			}
			rows := *rowBuffer
			if *oldRows != 0 || len(rows) != 5 {
				t.Fatalf("distinct rows=%d old=%d, want 5 new rows and no old rows", len(rows), *oldRows)
			}
			if rows[0].Get("cavg").Any() != float64(100) || !rows[0].Get("cstddev").IsNull() || rows[0].Get("csum").Any() != int(100) {
				t.Fatalf("distinct singleton row = %#v", rows[0].AsMap())
			}
			if rows[2].Get("cavedev").Any() != float64(50) || rows[2].Get("cavg").Any() != float64(150) || rows[2].Get("cmax").Any() != int(200) || rows[2].Get("cmedian").Any() != float64(150) || rows[2].Get("cmin").Any() != int(100) || rows[2].Get("csum").Any() != int(300) {
				t.Fatalf("distinct two-value row = %#v", rows[2].AsMap())
			}
			if got, ok := rows[2].Get("cstddev").Any().(float64); !ok || math.Abs(got-math.Sqrt(5000)) > 1e-12 {
				t.Fatalf("distinct stddev = %#v", rows[2].Get("cstddev").Any())
			}
			if rows[4].Get("cavedev").Any() != float64(0) || rows[4].Get("cavg").Any() != float64(200) || rows[4].Get("cmedian").Any() != float64(200) || rows[4].Get("cmin").Any() != int(200) || rows[4].Get("csum").Any() != int(200) || !rows[4].Get("cstddev").IsNull() {
				t.Fatalf("distinct post-eviction row = %#v", rows[4].AsMap())
			}
		})
	}
}
