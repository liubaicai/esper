package esper

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type patternNotA struct {
	ID string `esper:"id"`
}

type patternNotB struct {
	ID string `esper:"id"`
}

type patternNotC struct {
	ID string `esper:"id"`
}

type patternNotD struct {
	ID string `esper:"id"`
}

type patternNotE struct {
	ID string `esper:"id"`
}

type patternNotF struct {
	ID string `esper:"id"`
}

type patternNotG struct {
	ID string `esper:"id"`
}

type patternNotMD struct {
	Symbol string `esper:"symbol"`
	Volume int64  `esper:"volume"`
}

func newPatternNotEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternNotA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternNotB](env, "SupportBean_B"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternNotC](env, "SupportBean_C"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternNotD](env, "SupportBean_D"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternNotE](env, "SupportBean_E"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternNotF](env, "SupportBean_F"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternNotG](env, "SupportBean_G"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

// sendPatternNotSet replays the Java EventCollectionFactory mixed set in
// order: A1, B1, C1, B2, A2, D1, E1, F1, D2, B3, G1, D3.
func sendPatternNotSet(t *testing.T, engine *Engine) {
	t.Helper()
	events := []any{
		patternNotA{ID: "A1"}, patternNotB{ID: "B1"}, patternNotC{ID: "C1"},
		patternNotB{ID: "B2"}, patternNotA{ID: "A2"}, patternNotD{ID: "D1"},
		patternNotE{ID: "E1"}, patternNotF{ID: "F1"}, patternNotD{ID: "D2"},
		patternNotB{ID: "B3"}, patternNotG{ID: "G1"}, patternNotD{ID: "D3"},
	}
	for _, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
}

// TestPatternOperatorNotWHarnessMatchesEsper covers the shared case list of
// PatternOperatorNotWHarness and PatternOp (both executions run the same
// expressions over the mixed event set). The object-model text round-trip
// case is covered by the Go-side pattern model shape tests; here each case
// checks the exact match sequence Esper produces.
func TestPatternOperatorNotWHarnessMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)

	cases := []struct {
		name  string
		build func(env *Environment) PatternStream
		want  [][]string // each fire as flattened tag=id pairs in order
	}{
		{
			name: "b and not d",
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).And(PatternFrom(d, "d", trueExpr).Not())
			},
			want: [][]string{{"b", "B1"}},
		},
		{
			name: "every b and not g",
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				g := From[patternNotG](env, "SupportBean_G")
				return PatternFrom(b, "b", trueExpr).Every().And(PatternFrom(g, "g", trueExpr).Not())
			},
			want: [][]string{{"b", "B1"}, {"b", "B2"}, {"b", "B3"}},
		},
		{
			name: "every b and not d",
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).Every().And(PatternFrom(d, "d", trueExpr).Not())
			},
			want: [][]string{{"b", "B1"}, {"b", "B2"}},
		},
		{
			name: "b and not a(A1)",
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				a := From[patternNotA](env, "SupportBean_A")
				return PatternFrom(b, "b", trueExpr).And(PatternFrom(a, "a", Equal[string](
					Field[patternNotA, string]("id"), Literal("A1"),
				)).Not())
			},
			want: nil,
		},
		{
			name: "b and not a2(A2)",
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				a := From[patternNotA](env, "SupportBean_A")
				return PatternFrom(b, "b", trueExpr).And(PatternFrom(a, "a2", Equal[string](
					Field[patternNotA, string]("id"), Literal("A2"),
				)).Not())
			},
			want: [][]string{{"b", "B1"}},
		},
		{
			name: "every (b and not b3(B3))",
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				return PatternFrom(b, "b", trueExpr).And(PatternFrom(b, "b3", Equal[string](
					Field[patternNotB, string]("id"), Literal("B3"),
				)).Not()).Every()
			},
			want: [][]string{{"b", "B1"}, {"b", "B2"}},
		},
		{
			name: "every (b or not d)",
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).Or(PatternFrom(d, "d", trueExpr).Not()).Every()
			},
			want: nil,
		},
		{
			name: "every (every b and not B(B2))",
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				return PatternFrom(b, "b", trueExpr).Every().And(PatternFrom(b, "nb", Equal[string](
					Field[patternNotB, string]("id"), Literal("B2"),
				)).Not()).Every()
			},
			want: [][]string{{"b", "B1"}, {"b", "B3"}, {"b", "B3"}},
		},
		{
			name: "every (b and not B(B2))",
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				return PatternFrom(b, "b", trueExpr).And(PatternFrom(b, "nb", Equal[string](
					Field[patternNotB, string]("id"), Literal("B2"),
				)).Not()).Every()
			},
			want: [][]string{{"b", "B1"}, {"b", "B3"}},
		},
		{
			name: "(b -> d) and not a",
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				a := From[patternNotA](env, "SupportBean_A")
				return PatternFrom(b, "b", trueExpr).Then(PatternFrom(d, "d", trueExpr)).And(PatternFrom(a, "a", trueExpr).Not())
			},
			want: nil,
		},
		{
			name: "every (b -> d) and not g",
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				g := From[patternNotG](env, "SupportBean_G")
				return PatternFrom(b, "b", trueExpr).Then(PatternFrom(d, "d", trueExpr)).Every().And(PatternFrom(g, "g", trueExpr).Not())
			},
			want: [][]string{{"b", "B1", "d", "D1"}},
		},
		{
			name: "every (b -> d) and not g(x)",
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				g := From[patternNotG](env, "SupportBean_G")
				return PatternFrom(b, "b", trueExpr).Then(PatternFrom(d, "d", trueExpr)).Every().And(PatternFrom(g, "g", Equal[string](
					Field[patternNotG, string]("id"), Literal("x"),
				)).Not())
			},
			want: [][]string{{"b", "B1", "d", "D1"}, {"b", "B3", "d", "D3"}},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env, engine := newPatternNotEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			pattern := testCase.build(env)
			plan, err := env.Build(pattern.Select(
				Alias("b", TagField[string]("b", "id")),
				Alias("d", TagField[string]("d", "id")),
			).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			var got [][]string
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, result := range batch.New {
					row, ok := result.Row()
					if !ok {
						return fmt.Errorf("pattern result is not a row: %#v", result)
					}
					fire := []string{}
					if value := row.Get("b"); value.State() == ValuePresent {
						fire = append(fire, "b", value.Any().(string))
					}
					if value := row.Get("d"); value.State() == ValuePresent {
						fire = append(fire, "d", value.Any().(string))
					}
					got = append(got, fire)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			sendPatternNotSet(t, engine)
			if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", testCase.want) {
				t.Fatalf("%s: fires = %v, want %v", testCase.name, got, testCase.want)
			}
		})
	}
}

// TestPatternOperatorNotUniformEventsMatchesEsper covers PatternUniformEvents:
// every a=SupportBean_A() and not a1=SupportBean_A(id='A4') over the uniform
// event set — the same event A4 completes the every leg and trips the
// negative branch atomically, so A4 does not fire and the pattern stops.
func TestPatternOperatorNotUniformEventsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternNotA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	a := From[patternNotA](env, "SupportBean_A")
	pattern := PatternFrom(a, "a", Literal[bool](true)).Every().And(PatternFrom(a, "a1", Equal[string](
		Field[patternNotA, string]("id"), Literal("A4"),
	)).Not())
	plan, err := env.Build(pattern.Select(
		Alias("c0", TagField[string]("a", "id")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			got = append(got, result.Get("c0").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"B1", "B2", "B3", "A4", "A5"} {
		if err := engine.SendEvent(context.Background(), patternNotA{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	if fmt.Sprintf("%v", got) != "[B1 B2 B3]" {
		t.Fatalf("fires = %v, want [B1 B2 B3]", got)
	}
}

// TestPatternOperatorNotTimeIntervalMatchesEsper covers PatternNotTimeInterval:
// every A=SupportBean(intPrimitive=123) -> (timer:interval(30 seconds) and not
// SupportMarketDataBean(volume=123, symbol=A.theString)). The forbidden market
// data event cancels only the branch whose a-tag correlates; the surviving
// branch fires when its 30 second timer expires.
func TestPatternOperatorNotTimeIntervalMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternOpBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternNotMD](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	sb := From[patternOpBean](env, "SupportBean")
	md := From[patternNotMD](env, "SupportMarketDataBean")
	pattern := PatternFrom(sb, "a", Equal[int](
		Field[patternOpBean, int]("intPrimitive"), Literal(123),
	)).Every().Then(
		TimerInterval(md, 30*time.Second).And(PatternFrom(md, "md", And(
			Equal[int64](Field[patternNotMD, int64]("volume"), Literal(int64(123))),
			Equal[string](Field[patternNotMD, string]("symbol"), TagField[string]("a", "theString")),
		)).Not()),
	)
	plan, err := env.Build(pattern.Select(
		Alias("theString", TagField[string]("a", "theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			got = append(got, result.Get("theString").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(ms).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	sendBean := func(id string, intPrimitive int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), patternOpBean{TheString: id, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}

	sendBean("E1", 123)
	advance(10000)
	sendBean("E2", 123)
	advance(20000)
	if err := engine.SendEvent(context.Background(), patternNotMD{Symbol: "E1", Volume: 123}); err != nil {
		t.Fatal(err)
	}
	advance(30000)
	sendBean("E3", 123)
	if len(got) != 0 {
		t.Fatalf("cancelled/pending branches must not fire, got = %v", got)
	}
	advance(40000)
	if len(got) != 1 || got[0] != "E2" {
		t.Fatalf("got = %v, want [E2]", got)
	}
}

// TestPatternOperatorNotFollowedByMatchesEsper covers PatternNotFollowedBy:
// every (SupportBean(intPrimitive>0) -> (SupportMarketDataBean and not
// SupportBean(intPrimitive=0))). The zero event cancels the live attempt; the
// every restarts and a later market data event completes the next attempt.
func TestPatternOperatorNotFollowedByMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternOpBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternNotMD](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	sb := From[patternOpBean](env, "SupportBean")
	md := From[patternNotMD](env, "SupportMarketDataBean")
	pattern := PatternFrom(sb, "a", Greater[int](
		Field[patternOpBean, int]("intPrimitive"), Literal(0),
	)).Then(
		PatternFrom(md, "md", Literal[bool](true)).And(PatternFrom(sb, "z", Equal[int](
			Field[patternOpBean, int]("intPrimitive"), Literal(0),
		)).Not()),
	).Every()
	plan, err := env.Build(pattern.Select(
		Alias("c0", TagField[string]("a", "theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			got = append(got, result.Get("c0").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []patternOpBean{
		{TheString: "E1", IntPrimitive: 1},
		{TheString: "E2", IntPrimitive: 2},
		{TheString: "E3", IntPrimitive: 0},
		{TheString: "E4", IntPrimitive: 1},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(got) != 0 {
		t.Fatalf("no market data event yet, got = %v", got)
	}
	if err := engine.SendEvent(context.Background(), patternNotMD{Symbol: "E5", Volume: 1}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "E4" {
		t.Fatalf("got = %v, want [E4]", got)
	}
}

// TestPatternOperatorNotWithEveryMatchesEsper covers PatternNotWithEvery:
// (every a=SupportBean(intPrimitive>=0)) and not SupportBean(intPrimitive<0).
// The and fires per non-negative event until a negative event kills the whole
// expression permanently.
func TestPatternOperatorNotWithEveryMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternOpBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	sb := From[patternOpBean](env, "SupportBean")
	pattern := PatternFrom(sb, "a", GreaterOrEqual[int](
		Field[patternOpBean, int]("intPrimitive"), Literal(0),
	)).Every().And(PatternFrom(sb, "neg", Less[int](
		Field[patternOpBean, int]("intPrimitive"), Literal(0),
	)).Not())
	plan, err := env.Build(pattern.Select(
		Alias("c0", TagField[string]("a", "theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			got = append(got, result.Get("c0").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []patternOpBean{
		{TheString: "E1", IntPrimitive: 1},
		{TheString: "E2", IntPrimitive: 2},
		{TheString: "E3", IntPrimitive: 3},
		{TheString: "E4", IntPrimitive: -1},
		{TheString: "E5", IntPrimitive: 3},
		{TheString: "E6", IntPrimitive: -1},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if fmt.Sprintf("%v", got) != "[E1 E2 E3]" {
		t.Fatalf("got = %v, want [E1 E2 E3]", got)
	}
}
