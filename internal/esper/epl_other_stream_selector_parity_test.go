package esper

import (
	"context"
	"testing"
)

// Parity coverage for the stream selector (stream.*) executions of
// EPLOtherSelectExprStreamSelector:
//
// - EPLOtherNoJoinWildcardWithAlias: select *, win.* as s0 from SB#length(3) as win
// - EPLOtherJoinWildcardWithAlias: select *, s1.* as s1stream, s0.* as s0stream from SB as s0, Market as s1
// - EPLOtherAloneJoinAlias: select s1.* as s1 from SB as s0, Market as s1 (and reverse)
// - EPLOtherAloneJoinNoAlias: select s1.* from SB as s0, Market as s1 (and reverse)

type streamSelectorBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type streamSelectorMarket struct {
	Symbol string  `esper:"symbol"`
	Volume int64   `esper:"volume"`
	Price  float64 `esper:"price"`
}

func newStreamSelectorEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[streamSelectorBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[streamSelectorMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func streamSelectorSubscribe(t *testing.T, deployment *Deployment) *[]Result {
	t.Helper()
	got := &[]Result{}
	_, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*got = append(*got, batch.New...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestEPLOtherNoJoinWildcardWithAliasParity covers EPLOtherNoJoinWildcardWithAlias:
// select *, win.* as s0 from SupportBean#length(3) as win.
// The result has all SupportBean properties plus a "s0" column holding the event.
// Java runtime: java-runtime-70753b738456d049f005.
func TestEPLOtherNoJoinWildcardWithAliasParity(t *testing.T) {
	env := newStreamSelectorEnv(t)
	engine := NewEngine(env)

	source := From[streamSelectorBean](env, "SupportBean").Window(LengthWindow(3))
	plan, err := env.Build(
		Select(source,
			Alias("theString", Field[streamSelectorBean, string]("theString")),
			Alias("intPrimitive", Field[streamSelectorBean, int]("intPrimitive")),
			Alias("s0", EventValue[Event]()),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := streamSelectorSubscribe(t, deployment)

	bean := streamSelectorBean{TheString: "E1", IntPrimitive: 15}
	if err := engine.SendEvent(context.Background(), bean); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	row := (*got)[0]
	if row.Get("theString").Any() != "E1" {
		t.Fatalf("theString = %#v", row.Get("theString").Any())
	}
	if row.Get("intPrimitive").Any() != 15 {
		t.Fatalf("intPrimitive = %#v", row.Get("intPrimitive").Any())
	}
	event, ok := row.Get("s0").Any().(Event)
	if !ok {
		t.Fatalf("s0 is not an Event: %T", row.Get("s0").Any())
	}
	underlying, ok := event.Underlying().(streamSelectorBean)
	if !ok {
		t.Fatalf("s0 underlying is not streamSelectorBean: %T", event.Underlying())
	}
	if underlying.TheString != "E1" || underlying.IntPrimitive != 15 {
		t.Fatalf("s0 underlying = %#v", underlying)
	}
}

// TestEPLOtherJoinWildcardWithAliasParity covers EPLOtherJoinWildcardWithAlias:
// select *, s1.* as s1stream, s0.* as s0stream from SupportBean#length(3) as s0,
// SupportMarketDataBean#keepall as s1.
// The result has s0/s1 stream columns plus aliased s0stream/s1stream columns.
// Java runtime: java-runtime-bc1c24102a3db1950ec8.
func TestEPLOtherJoinWildcardWithAliasParity(t *testing.T) {
	env := newStreamSelectorEnv(t)
	engine := NewEngine(env)

	left := From[streamSelectorBean](env, "SupportBean").Window(LengthWindow(3))
	right := From[streamSelectorMarket](env, "SupportMarketDataBean").Window(KeepAll())
	plan, err := env.Build(
		JoinMany(
			JoinSource(left),
			JoinSource(right),
		).Select(
			SelectSourceEvent(0, "s0"),
			SelectSourceEvent(1, "s1"),
			SelectSourceEvent(0, "s0stream"),
			SelectSourceEvent(1, "s1stream"),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := streamSelectorSubscribe(t, deployment)

	bean := streamSelectorBean{TheString: "E1", IntPrimitive: 13}
	if err := engine.SendEvent(context.Background(), bean); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 0 {
		t.Fatalf("expected 0 results before market event, got %d", len(*got))
	}

	market := streamSelectorMarket{Symbol: "E2", Volume: 0, Price: 0}
	if err := engine.SendEvent(context.Background(), market); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	row := (*got)[0]

	s0Event, ok := row.Get("s0").Any().(Event)
	if !ok {
		t.Fatalf("s0 is not an Event: %T", row.Get("s0").Any())
	}
	if s0Event.Underlying().(streamSelectorBean).TheString != "E1" {
		t.Fatalf("s0 underlying theString = %#v", s0Event.Underlying())
	}

	s1Event, ok := row.Get("s1").Any().(Event)
	if !ok {
		t.Fatalf("s1 is not an Event: %T", row.Get("s1").Any())
	}
	if s1Event.Underlying().(streamSelectorMarket).Symbol != "E2" {
		t.Fatalf("s1 underlying symbol = %#v", s1Event.Underlying())
	}

	s0stream, ok := row.Get("s0stream").Any().(Event)
	if !ok {
		t.Fatalf("s0stream is not an Event: %T", row.Get("s0stream").Any())
	}
	if s0stream.Underlying().(streamSelectorBean).IntPrimitive != 13 {
		t.Fatalf("s0stream underlying intPrimitive = %#v", s0stream.Underlying())
	}

	s1stream, ok := row.Get("s1stream").Any().(Event)
	if !ok {
		t.Fatalf("s1stream is not an Event: %T", row.Get("s1stream").Any())
	}
	if s1stream.Underlying().(streamSelectorMarket).Symbol != "E2" {
		t.Fatalf("s1stream underlying symbol = %#v", s1stream.Underlying())
	}
}

// TestEPLOtherAloneJoinAliasParity covers EPLOtherAloneJoinAlias:
// select s1.* as s1 from SupportBean#length(3) as s0, SupportMarketDataBean#keepall as s1
// (and reverse: select s0.* as szero from same).
// The result has only the aliased event column.
// Java runtime: java-runtime-4a8e3f2dc067ae2b7602.
func TestEPLOtherAloneJoinAliasParity(t *testing.T) {
	env := newStreamSelectorEnv(t)
	engine := NewEngine(env)

	left := From[streamSelectorBean](env, "SupportBean").Window(LengthWindow(3))
	right := From[streamSelectorMarket](env, "SupportMarketDataBean").Window(KeepAll())

	// Forward: select s1.* as s1
	plan, err := env.Build(
		JoinMany(
			JoinSource(left),
			JoinSource(right),
		).Select(
			SelectSourceEvent(1, "s1"),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := streamSelectorSubscribe(t, deployment)

	if err := engine.SendEvent(context.Background(), streamSelectorBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 0 {
		t.Fatalf("expected 0 results before market event, got %d", len(*got))
	}

	market := streamSelectorMarket{Symbol: "E1"}
	if err := engine.SendEvent(context.Background(), market); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	row := (*got)[0]
	s1Event, ok := row.Get("s1").Any().(Event)
	if !ok {
		t.Fatalf("s1 is not an Event: %T", row.Get("s1").Any())
	}
	if s1Event.Underlying().(streamSelectorMarket).Symbol != "E1" {
		t.Fatalf("s1 underlying symbol = %#v", s1Event.Underlying())
	}

	// Reverse: select s0.* as szero
	reversePlan, err := env.Build(
		JoinMany(
			JoinSource(left),
			JoinSource(right),
		).Select(
			SelectSourceEvent(0, "szero"),
		).Query(StatementName("s1")),
	)
	if err != nil {
		t.Fatal(err)
	}
	reverseDeployment, err := engine.Deploy(context.Background(), reversePlan)
	if err != nil {
		t.Fatal(err)
	}
	reverseGot := &[]Result{}
	_, err = reverseDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*reverseGot = append(*reverseGot, batch.New...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Send market first, then bean — bean triggers join
	if err := engine.SendEvent(context.Background(), streamSelectorMarket{Symbol: "E2"}); err != nil {
		t.Fatal(err)
	}
	if len(*reverseGot) != 0 {
		t.Fatalf("expected 0 reverse results before bean event, got %d", len(*reverseGot))
	}
	if err := engine.SendEvent(context.Background(), streamSelectorBean{TheString: "E2", IntPrimitive: 42}); err != nil {
		t.Fatal(err)
	}
	if len(*reverseGot) != 1 {
		t.Fatalf("expected 1 reverse result, got %d", len(*reverseGot))
	}
	reverseRow := (*reverseGot)[0]
	szeroEvent, ok := reverseRow.Get("szero").Any().(Event)
	if !ok {
		t.Fatalf("szero is not an Event: %T", reverseRow.Get("szero").Any())
	}
	if szeroEvent.Underlying().(streamSelectorBean).IntPrimitive != 42 {
		t.Fatalf("szero underlying intPrimitive = %#v", szeroEvent.Underlying())
	}
}

// TestEPLOtherAloneJoinNoAliasParity covers EPLOtherAloneJoinNoAlias:
// select s1.* from SupportBean#length(3) as s0, SupportMarketDataBean#keepall as s1
// (and reverse: select s0.* from same).
// Without an explicit alias, stream.* expands to the event's individual
// properties (symbol, volume, price), not a single wrapped event column.
// Java runtime: java-runtime-f7193a89174e1e9c9ca0.
func TestEPLOtherAloneJoinNoAliasParity(t *testing.T) {
	env := newStreamSelectorEnv(t)
	engine := NewEngine(env)

	left := From[streamSelectorBean](env, "SupportBean").Window(LengthWindow(3))
	right := From[streamSelectorMarket](env, "SupportMarketDataBean").Window(KeepAll())

	// Forward: select s1.* — expands to individual properties, no "s1" event column
	plan, err := env.Build(
		JoinMany(
			JoinSource(left),
			JoinSource(right),
		).Select(
			SelectFrom(1, "symbol", JoinField[string](1, "symbol")),
			SelectFrom(1, "volume", JoinField[int64](1, "volume")),
			SelectFrom(1, "price", JoinField[float64](1, "price")),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := streamSelectorSubscribe(t, deployment)

	if err := engine.SendEvent(context.Background(), streamSelectorBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 0 {
		t.Fatalf("expected 0 results before market event, got %d", len(*got))
	}

	market := streamSelectorMarket{Symbol: "E1", Volume: 100}
	if err := engine.SendEvent(context.Background(), market); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	row := (*got)[0]
	if row.Get("symbol").Any() != "E1" {
		t.Fatalf("symbol = %#v", row.Get("symbol").Any())
	}
	if row.Get("volume").Any() != int64(100) {
		t.Fatalf("volume = %#v", row.Get("volume").Any())
	}
	if row.Get("price").Any() != float64(0) {
		t.Fatalf("price = %#v", row.Get("price").Any())
	}
	// Negative: no wrapped event column — s1.* without alias must NOT
	// produce a single "s1" Event column.
	if row.Get("s1").IsPresent() {
		t.Fatalf("unexpected s1 event column in noalias expansion: %#v", row.Get("s1").Any())
	}

	// Reverse: select s0.* — expands to SupportBean properties
	reversePlan, err := env.Build(
		JoinMany(
			JoinSource(left),
			JoinSource(right),
		).Select(
			SelectFrom(0, "theString", JoinField[string](0, "theString")),
			SelectFrom(0, "intPrimitive", JoinField[int](0, "intPrimitive")),
		).Query(StatementName("s1")),
	)
	if err != nil {
		t.Fatal(err)
	}
	reverseDeployment, err := engine.Deploy(context.Background(), reversePlan)
	if err != nil {
		t.Fatal(err)
	}
	reverseGot := &[]Result{}
	_, err = reverseDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*reverseGot = append(*reverseGot, batch.New...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Send market first, then bean — bean triggers join
	if err := engine.SendEvent(context.Background(), streamSelectorMarket{Symbol: "E2"}); err != nil {
		t.Fatal(err)
	}
	if len(*reverseGot) != 0 {
		t.Fatalf("expected 0 reverse results before bean event, got %d", len(*reverseGot))
	}
	if err := engine.SendEvent(context.Background(), streamSelectorBean{TheString: "E2", IntPrimitive: 42}); err != nil {
		t.Fatal(err)
	}
	if len(*reverseGot) != 1 {
		t.Fatalf("expected 1 reverse result, got %d", len(*reverseGot))
	}
	reverseRow := (*reverseGot)[0]
	if reverseRow.Get("theString").Any() != "E2" {
		t.Fatalf("reverse theString = %#v", reverseRow.Get("theString").Any())
	}
	if reverseRow.Get("intPrimitive").Any() != 42 {
		t.Fatalf("reverse intPrimitive = %#v", reverseRow.Get("intPrimitive").Any())
	}
	// Negative: no wrapped event column for reverse either
	if reverseRow.Get("s0").IsPresent() {
		t.Fatalf("unexpected s0 event column in noalias expansion: %#v", reverseRow.Get("s0").Any())
	}
}
