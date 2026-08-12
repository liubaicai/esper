package esper

import (
	"context"
	"math/big"
	"testing"
)

type exactAggregateEvent struct {
	BigInteger big.Int `esper:"bigint"`
	BigDecimal big.Rat `esper:"bigdec"`
}

func exactBigInt(text string) big.Int {
	var value big.Int
	if _, ok := value.SetString(text, 10); !ok {
		panic("invalid exact integer " + text)
	}
	return value
}

func exactBigRat(text string) big.Rat {
	var value big.Rat
	if _, ok := value.SetString(text); !ok {
		panic("invalid exact rational " + text)
	}
	return value
}

func assertExactRat(t *testing.T, value Value, expected string) {
	t.Helper()
	actual, err := As[big.Rat](value)
	if err != nil {
		t.Fatalf("exact rational = %v, %v", actual, err)
	}
	want := exactBigRat(expected)
	if !value.IsPresent() || actual.Cmp(&want) != 0 {
		t.Fatalf("exact rational = %s, want %s (state=%v)", actual.RatString(), want.RatString(), value.State())
	}
}

func assertExactInt(t *testing.T, value Value, expected string) {
	t.Helper()
	actual, err := As[big.Int](value)
	if err != nil {
		t.Fatalf("exact integer = %v, %v", actual, err)
	}
	want := exactBigInt(expected)
	if !value.IsPresent() || actual.Cmp(&want) != 0 {
		t.Fatalf("exact integer = %s, want %s (state=%v)", actual.String(), want.String(), value.State())
	}
}

// This follows ResultSetAggregateFiltered's BigDecimal/BigInteger section:
// avg(bigdec), sum(bigdec), and sum(bigint) are filtered by bigint < 100 over
// a length-two window. Java keeps these values exact; the Go exact family
// makes that precision an explicit part of the fluent API.
func TestFilteredExactAggregatesMatchJavaBigNumberTrace(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exactAggregateEvent](env, "ExactAggregateEvent"); err != nil {
		t.Fatal(err)
	}
	integer := Field[exactAggregateEvent, big.Int]("bigint")
	decimal := Field[exactAggregateEvent, big.Rat]("bigdec")
	filter := LessExact[big.Int](integer, Literal(exactBigInt("100")))
	plan, err := env.Build(From[exactAggregateEvent](env, "ExactAggregateEvent").Window(LengthWindow(2)).Aggregate(
		Alias("avgDecimal", FilterAggregate[big.Rat](AvgExact[big.Rat](decimal), filter)),
		Alias("sumDecimal", FilterAggregate[big.Rat](SumExact[big.Rat](decimal), filter)),
		Alias("sumInteger", FilterAggregate[big.Int](SumExact[big.Int](integer), filter)),
		Alias("avgInteger", FilterAggregate[big.Rat](AvgExact[big.Int](integer), filter)),
		Alias("minDecimal", FilterAggregate[big.Rat](MinExact[big.Rat](decimal), filter)),
		Alias("maxDecimal", FilterAggregate[big.Rat](MaxExact[big.Rat](decimal), filter)),
	).Query(StatementName("filtered-exact-aggregate-java-trace")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 4)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("exact aggregate result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, event := range []exactAggregateEvent{
		{BigInteger: exactBigInt("10"), BigDecimal: exactBigRat("20")},
		{BigInteger: exactBigInt("101"), BigDecimal: exactBigRat("101")},
		{BigInteger: exactBigInt("20"), BigDecimal: exactBigRat("40")},
		{BigInteger: exactBigInt("30"), BigDecimal: exactBigRat("50")},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 4 {
		t.Fatalf("exact aggregate rows = %d", len(rows))
	}

	assertExactRat(t, rows[0].Get("avgDecimal"), "20")
	assertExactRat(t, rows[0].Get("sumDecimal"), "20")
	assertExactInt(t, rows[0].Get("sumInteger"), "10")
	assertExactRat(t, rows[0].Get("avgInteger"), "10")
	assertExactRat(t, rows[0].Get("minDecimal"), "20")
	assertExactRat(t, rows[0].Get("maxDecimal"), "20")

	assertExactRat(t, rows[1].Get("avgDecimal"), "20")
	assertExactRat(t, rows[1].Get("sumDecimal"), "20")
	assertExactInt(t, rows[1].Get("sumInteger"), "10")

	assertExactRat(t, rows[2].Get("avgDecimal"), "40")
	assertExactRat(t, rows[2].Get("sumDecimal"), "40")
	assertExactInt(t, rows[2].Get("sumInteger"), "20")
	assertExactRat(t, rows[2].Get("avgInteger"), "20")
	assertExactRat(t, rows[2].Get("minDecimal"), "40")
	assertExactRat(t, rows[2].Get("maxDecimal"), "40")

	assertExactRat(t, rows[3].Get("avgDecimal"), "45")
	assertExactRat(t, rows[3].Get("sumDecimal"), "90")
	assertExactInt(t, rows[3].Get("sumInteger"), "50")
	assertExactRat(t, rows[3].Get("avgInteger"), "25")
	assertExactRat(t, rows[3].Get("minDecimal"), "40")
	assertExactRat(t, rows[3].Get("maxDecimal"), "50")
}

func TestExactAggregatesDoNotRoundLargeIntegers(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exactAggregateEvent](env, "ExactLargeAggregateEvent"); err != nil {
		t.Fatal(err)
	}
	integer := Field[exactAggregateEvent, big.Int]("bigint")
	plan, err := env.Build(From[exactAggregateEvent](env, "ExactLargeAggregateEvent").Window(KeepAll()).Aggregate(
		Alias("sum", SumExact[big.Int](integer)),
		Alias("avg", AvgExact[big.Int](integer)),
	).Query(StatementName("exact-large-aggregate")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var latest Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			latest, _ = batch.New[len(batch.New)-1].Row()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), exactAggregateEvent{
		BigInteger: exactBigInt("9007199254740993"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), exactAggregateEvent{
		BigInteger: exactBigInt("9007199254740994"),
	}); err != nil {
		t.Fatal(err)
	}
	assertExactInt(t, latest.Get("sum"), "18014398509481987")
	assertExactRat(t, latest.Get("avg"), "18014398509481987/2")
}
