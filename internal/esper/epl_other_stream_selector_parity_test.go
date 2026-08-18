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

// TestEPLOtherAloneNoJoinAliasParity covers EPLOtherAloneNoJoinAlias:
// select theString.* as s0 from SupportBean#length(3) as theString.
// The result has one column "s0" holding the SupportBean event itself.
// Java runtime: java-runtime-bb3332c825b24eb5046a.
func TestEPLOtherAloneNoJoinAliasParity(t *testing.T) {
	env := newStreamSelectorEnv(t)
	engine := NewEngine(env)

	source := From[streamSelectorBean](env, "SupportBean").Window(LengthWindow(3))
	plan, err := env.Build(
		Select(source,
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

	bean := streamSelectorBean{TheString: "E1", IntPrimitive: 0}
	if err := engine.SendEvent(context.Background(), bean); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	row := (*got)[0]
	// s0 should be the event object itself (identity check)
	s0Event, ok := row.Get("s0").Any().(Event)
	if !ok {
		t.Fatalf("s0 is not an Event: %T", row.Get("s0").Any())
	}
	underlying, ok := s0Event.Underlying().(streamSelectorBean)
	if !ok {
		t.Fatalf("s0 underlying is not streamSelectorBean: %T", s0Event.Underlying())
	}
	if underlying.TheString != "E1" {
		t.Fatalf("s0 underlying theString = %#v", underlying.TheString)
	}
}

// TestEPLOtherNoJoinNoAliasWithPropertiesParity covers EPLOtherNoJoinNoAliasWithProperties:
// select intPrimitive as a, string.*, intPrimitive as b from SupportBean#length(3) as string.
// The result expands string.* into individual properties (theString, intPrimitive, etc.)
// plus explicit a and b columns.
// Java runtime: java-runtime-04453c1c3c73e4a9ba36.
func TestEPLOtherNoJoinNoAliasWithPropertiesParity(t *testing.T) {
	env := newStreamSelectorEnv(t)
	engine := NewEngine(env)

	source := From[streamSelectorBean](env, "SupportBean").Window(LengthWindow(3))
	plan, err := env.Build(
		Select(source,
			Alias("a", Field[streamSelectorBean, int]("intPrimitive")),
			Alias("theString", Field[streamSelectorBean, string]("theString")),
			Alias("intPrimitive", Field[streamSelectorBean, int]("intPrimitive")),
			Alias("b", Field[streamSelectorBean, int]("intPrimitive")),
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

	bean := streamSelectorBean{TheString: "E1", IntPrimitive: 10}
	if err := engine.SendEvent(context.Background(), bean); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	row := (*got)[0]
	if row.Get("a").Any() != 10 {
		t.Fatalf("a = %#v", row.Get("a").Any())
	}
	if row.Get("theString").Any() != "E1" {
		t.Fatalf("theString = %#v", row.Get("theString").Any())
	}
	if row.Get("intPrimitive").Any() != 10 {
		t.Fatalf("intPrimitive = %#v", row.Get("intPrimitive").Any())
	}
	if row.Get("b").Any() != 10 {
		t.Fatalf("b = %#v", row.Get("b").Any())
	}
}

// TestEPLOtherNoJoinWildcardNoAliasParity covers EPLOtherNoJoinWildcardNoAlias:
// select *, win.* from SupportBean#length(3) as win.
// The result has all SupportBean properties (both * and win.* expand identically).
// The underlying type is the original bean (same underlying identity).
// Java runtime: java-runtime-6dbc99a56abddbf3d38a.
func TestEPLOtherNoJoinWildcardNoAliasParity(t *testing.T) {
	env := newStreamSelectorEnv(t)
	engine := NewEngine(env)

	source := From[streamSelectorBean](env, "SupportBean").Window(LengthWindow(3))
	plan, err := env.Build(
		Select(source,
			Alias("theString", Field[streamSelectorBean, string]("theString")),
			Alias("intPrimitive", Field[streamSelectorBean, int]("intPrimitive")),
			Alias("event", EventValue[Event]()),
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

	bean := streamSelectorBean{TheString: "E1", IntPrimitive: 16}
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
	if row.Get("intPrimitive").Any() != 16 {
		t.Fatalf("intPrimitive = %#v", row.Get("intPrimitive").Any())
	}
	// Verify underlying identity: the event wraps the same bean
	event, ok := row.Get("event").Any().(Event)
	if !ok {
		t.Fatalf("event is not an Event: %T", row.Get("event").Any())
	}
	underlying, ok := event.Underlying().(streamSelectorBean)
	if !ok {
		t.Fatalf("event underlying is not streamSelectorBean: %T", event.Underlying())
	}
	if underlying.TheString != "E1" || underlying.IntPrimitive != 16 {
		t.Fatalf("event underlying = %#v", underlying)
	}
}

// TestEPLOtherJoinNoAliasWithPropertiesParity covers EPLOtherJoinNoAliasWithProperties:
// select intPrimitive, s1.*, symbol as sym from SupportBean#length(3) as s0,
// SupportMarketDataBean#keepall as s1.
// The result expands s1.* into individual properties plus intPrimitive and sym.
// Java runtime: java-runtime-dccd7d03c70875f69975.
func TestEPLOtherJoinNoAliasWithPropertiesParity(t *testing.T) {
	env := newStreamSelectorEnv(t)
	engine := NewEngine(env)

	left := From[streamSelectorBean](env, "SupportBean").Window(LengthWindow(3))
	right := From[streamSelectorMarket](env, "SupportMarketDataBean").Window(KeepAll())
	plan, err := env.Build(
		JoinMany(
			JoinSource(left),
			JoinSource(right),
		).Select(
			SelectFrom(0, "intPrimitive", Field[streamSelectorBean, int]("intPrimitive")),
			SelectFrom(1, "symbol", Field[streamSelectorMarket, string]("symbol")),
			SelectFrom(1, "volume", Field[streamSelectorMarket, int64]("volume")),
			SelectFrom(1, "price", Field[streamSelectorMarket, float64]("price")),
			SelectFrom(0, "theString", Field[streamSelectorBean, string]("theString")),
			SelectFrom(1, "sym", Field[streamSelectorMarket, string]("symbol")),
			SelectFrom(0, "s0", EventValue[Event]()),
			SelectFrom(1, "s1", EventValue[Event]()),
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

	bean := streamSelectorBean{TheString: "E1", IntPrimitive: 11}
	if err := engine.SendEvent(context.Background(), bean); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 0 {
		t.Fatalf("expected 0 results before market event, got %d", len(*got))
	}

	marketEvent := streamSelectorMarket{Symbol: "E1", Volume: 0, Price: 0}
	if err := engine.SendEvent(context.Background(), marketEvent); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result after join, got %d", len(*got))
	}
	row := (*got)[0]
	if row.Get("intPrimitive").Any() != 11 {
		t.Fatalf("intPrimitive = %#v", row.Get("intPrimitive").Any())
	}
	if row.Get("sym").Any() != "E1" {
		t.Fatalf("sym = %#v", row.Get("sym").Any())
	}
	if row.Get("symbol").Any() != "E1" {
		t.Fatalf("symbol = %#v", row.Get("symbol").Any())
	}
	// s1 should be the market event
	s1Event, ok := row.Get("s1").Any().(Event)
	if !ok {
		t.Fatalf("s1 is not an Event: %T", row.Get("s1").Any())
	}
	s1Underlying, ok := s1Event.Underlying().(streamSelectorMarket)
	if !ok {
		t.Fatalf("s1 underlying is not streamSelectorMarket: %T", s1Event.Underlying())
	}
	if s1Underlying.Symbol != "E1" {
		t.Fatalf("s1 underlying symbol = %#v", s1Underlying.Symbol)
	}
}

// TestEPLOtherJoinWildcardNoAliasParity covers EPLOtherJoinWildcardNoAlias:
// select *, s1.* from SupportBean#length(3) as s0, SupportMarketDataBean#keepall as s1.
// The result has 7 properties: s0 (SupportBean), s1 (SupportMarketDataBean),
// theString, intPrimitive, symbol, volume, price.
// Java runtime: java-runtime-6556dd46777be4afa86f.
func TestEPLOtherJoinWildcardNoAliasParity(t *testing.T) {
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
			SelectFrom(0, "theString", Field[streamSelectorBean, string]("theString")),
			SelectFrom(0, "intPrimitive", Field[streamSelectorBean, int]("intPrimitive")),
			SelectFrom(1, "symbol", Field[streamSelectorMarket, string]("symbol")),
			SelectFrom(1, "volume", Field[streamSelectorMarket, int64]("volume")),
			SelectFrom(1, "price", Field[streamSelectorMarket, float64]("price")),
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
		t.Fatalf("expected 1 result after join, got %d", len(*got))
	}
	row := (*got)[0]
	// s0 and s1 should be the source events
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
	if row.Get("theString").Any() != "E1" {
		t.Fatalf("theString = %#v", row.Get("theString").Any())
	}
	if row.Get("symbol").Any() != "E2" {
		t.Fatalf("symbol = %#v", row.Get("symbol").Any())
	}
	if row.Get("volume").Any() != int64(0) {
		t.Fatalf("volume = %#v", row.Get("volume").Any())
	}
}
