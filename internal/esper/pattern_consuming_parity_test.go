package esper

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Parity coverage for PatternConsumingPattern (see docs
// esper-go-port-implementation-plan.md): the @DiscardPartialsOnMatch and
// @SuppressOverlappingMatches pattern-level policies, expressed in the fluent
// API as the DiscardPartialsOnMatch()/SuppressOverlappingMatches() query
// options. Event beans mirror SupportIdEventA/B/C/D.

type patternIdEventA struct {
	ID    string `esper:"id"`
	Pa    string `esper:"pa"`
	Mysec int    `esper:"mysec"`
}

type patternIdEventB struct {
	ID string `esper:"id"`
	Pb string `esper:"pb"`
}

type patternIdEventC struct {
	ID string `esper:"id"`
	Pc string `esper:"pc"`
}

type patternIdEventD struct {
	ID string `esper:"id"`
}

type patternConsumeStreams struct {
	a Stream[patternIdEventA]
	b Stream[patternIdEventB]
	c Stream[patternIdEventC]
	d Stream[patternIdEventD]
}

func newPatternConsumeEnv(t *testing.T, start time.Time) (*Environment, *Engine, patternConsumeStreams) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternIdEventA](env, "SupportIdEventA"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternIdEventB](env, "SupportIdEventB"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternIdEventC](env, "SupportIdEventC"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternIdEventD](env, "SupportIdEventD"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(start.UTC()))
	return env, engine, patternConsumeStreams{
		a: From[patternIdEventA](env, "SupportIdEventA"),
		b: From[patternIdEventB](env, "SupportIdEventB"),
		c: From[patternIdEventC](env, "SupportIdEventC"),
		d: From[patternIdEventD](env, "SupportIdEventD"),
	}
}

// patternConsumeCollect deploys one consuming-pattern statement and returns a
// pointer to the flattened fire rows (each fire is the sorted tag=id join).
func patternConsumeCollect(t *testing.T, env *Environment, engine *Engine, stream PatternStream, discard bool, tags ...string) *[]string {
	t.Helper()
	selections := make([]Selection, 0, len(tags))
	for _, tag := range tags {
		selections = append(selections, Alias(tag, TagField[string](tag, "id")))
	}
	options := []QueryOption{StatementName("s0")}
	if discard {
		options = append(options, DiscardPartialsOnMatch())
	}
	plan, err := env.Build(stream.Select(selections...).Query(options...))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	fires := []string{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("pattern result is not a row: %#v", result)
			}
			parts := []string{}
			for _, tag := range tags {
				if value := row.Get(tag); value.State() == ValuePresent {
					parts = append(parts, tag+"="+value.Any().(string))
				}
			}
			fires = append(fires, joinStrings(parts, " "))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &fires
}

func patternConsumeSend(t *testing.T, engine *Engine, events ...any) {
	t.Helper()
	for _, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
}

func patternConsumeAssert(t *testing.T, step string, fires *[]string, want ...string) {
	t.Helper()
	got := fmt.Sprintf("%v", *fires)
	wantText := fmt.Sprintf("%v", want)
	if got != wantText {
		t.Fatalf("%s: fires = %s, want %s", step, got, wantText)
	}
	*fires = nil
}

// TestPatternConsumingOrOpMatchesEsper mirrors PatternOrOp: with
// @DiscardPartialsOnMatch the completed A2 match discards the partial A1
// branch (they share B1), so the second correlated C does not fire.
func TestPatternConsumingOrOpMatchesEsper(t *testing.T) {
	env, engine, streams := newPatternConsumeEnv(t, time.UnixMilli(0))
	defer func() { _ = engine.Close(context.Background()) }()
	trueExpr := Literal[bool](true)
	pcEq := Equal[string](Field[patternIdEventC, string]("pc"), TagField[string]("a", "pa"))
	orBranch := PatternFrom(streams.b, "b", trueExpr).
		Then(PatternFrom(streams.c, "c", pcEq)).
		Or(TimerInterval(streams.a, 1000*time.Second))
	pattern := PatternFrom(streams.a, "a", trueExpr).Every().Then(orBranch)
	fires := patternConsumeCollect(t, env, engine, pattern, true, "a", "b", "c")
	patternConsumeSend(t, engine, patternIdEventA{ID: "A1", Pa: "x"}, patternIdEventA{ID: "A2", Pa: "y"})
	patternConsumeSend(t, engine, patternIdEventB{ID: "B1"})
	patternConsumeSend(t, engine, patternIdEventC{ID: "C1", Pc: "y"})
	patternConsumeAssert(t, "C1(y)", fires, "a=A2 b=B1 c=C1")
	patternConsumeSend(t, engine, patternIdEventC{ID: "C1", Pc: "x"})
	patternConsumeAssert(t, "C1(x)", fires)
}

// TestPatternConsumingMatchUntilOpMatchesEsper mirrors PatternMatchUntilOp:
// bounded repeats with correlation under the discard policy, a repeat of a
// child sequence, and a ranged repeat terminated by a timer-and-not guard.
func TestPatternConsumingMatchUntilOpMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)

	runBoundOp := func(t *testing.T, discard bool) {
		env, engine, streams := newPatternConsumeEnv(t, time.UnixMilli(0))
		defer func() { _ = engine.Close(context.Background()) }()
		pbIn := InOf(Field[patternIdEventB, string]("pb"), TagField[string]("a", "pa"), Literal("-"))
		repeat := PatternFrom(streams.b, "b", pbIn).MatchUntil(2, 2)
		pattern := PatternFrom(streams.a, "a", trueExpr).Every().Then(repeat)
		selections := []Selection{
			Alias("a", TagField[string]("a", "id")),
			Alias("b0", TagFieldAt[string]("b", 0, "id")),
			Alias("b1", TagFieldAt[string]("b", 1, "id")),
		}
		options := []QueryOption{StatementName("s0")}
		if discard {
			options = append(options, DiscardPartialsOnMatch())
		}
		plan, err := env.Build(pattern.Select(selections...).Query(options...))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		fires := []string{}
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					return fmt.Errorf("pattern result is not a row: %#v", result)
				}
				parts := []string{}
				for _, name := range []string{"a", "b0", "b1"} {
					if value := row.Get(name); value.State() == ValuePresent {
						parts = append(parts, name+"="+value.Any().(string))
					}
				}
				fires = append(fires, joinStrings(parts, " "))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		patternConsumeSend(t, engine, patternIdEventA{ID: "A1", Pa: "x"}, patternIdEventA{ID: "A2", Pa: "y"})
		patternConsumeSend(t, engine, patternIdEventB{ID: "B1", Pb: "-"})
		patternConsumeSend(t, engine, patternIdEventB{ID: "B2", Pb: "y"})
		patternConsumeAssert(t, "B2(y)", &fires, "a=A2 b0=B1 b1=B2")
		patternConsumeSend(t, engine, patternIdEventB{ID: "B3", Pb: "x"})
		if discard {
			patternConsumeAssert(t, "B3(x)", &fires)
		} else {
			patternConsumeAssert(t, "B3(x)", &fires, "a=A1 b0=B1 b1=B3")
		}
	}
	t.Run("bound-op-discard", func(t *testing.T) { runBoundOp(t, true) })
	t.Run("bound-op-keep", func(t *testing.T) { runBoundOp(t, false) })

	runChildMatcher := func(t *testing.T, discard bool) {
		env, engine, streams := newPatternConsumeEnv(t, time.UnixMilli(0))
		defer func() { _ = engine.Close(context.Background()) }()
		pcEq := Equal[string](Field[patternIdEventC, string]("pc"), TagField[string]("a", "pa"))
		repeat := PatternFrom(streams.b, "b", trueExpr).
			Then(PatternFrom(streams.c, "c", pcEq)).
			MatchUntil(1, 1)
		pattern := PatternFrom(streams.a, "a", trueExpr).Every().Then(repeat)
		selections := []Selection{
			Alias("a", TagField[string]("a", "id")),
			Alias("b0", TagFieldAt[string]("b", 0, "id")),
			Alias("c0", TagFieldAt[string]("c", 0, "id")),
		}
		options := []QueryOption{StatementName("s0")}
		if discard {
			options = append(options, DiscardPartialsOnMatch())
		}
		plan, err := env.Build(pattern.Select(selections...).Query(options...))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		fires := []string{}
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					return fmt.Errorf("pattern result is not a row: %#v", result)
				}
				parts := []string{}
				for _, name := range []string{"a", "b0", "c0"} {
					if value := row.Get(name); value.State() == ValuePresent {
						parts = append(parts, name+"="+value.Any().(string))
					}
				}
				fires = append(fires, joinStrings(parts, " "))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		patternConsumeSend(t, engine, patternIdEventA{ID: "A1", Pa: "x"}, patternIdEventA{ID: "A2", Pa: "y"})
		patternConsumeSend(t, engine, patternIdEventB{ID: "B1"})
		patternConsumeSend(t, engine, patternIdEventC{ID: "C1", Pc: "y"})
		patternConsumeAssert(t, "C1(y)", &fires, "a=A2 b0=B1 c0=C1")
		patternConsumeSend(t, engine, patternIdEventC{ID: "C2", Pc: "x"})
		if discard {
			patternConsumeAssert(t, "C2(x)", &fires)
		} else {
			patternConsumeAssert(t, "C2(x)", &fires, "a=A1 b0=B1 c0=C2")
		}
	}
	t.Run("child-matcher-discard", func(t *testing.T) { runChildMatcher(t, true) })
	t.Run("child-matcher-keep", func(t *testing.T) { runChildMatcher(t, false) })

	t.Run("range-op-with-time", func(t *testing.T) {
		env, engine, streams := newPatternConsumeEnv(t, time.UnixMilli(0))
		defer func() { _ = engine.Close(context.Background()) }()
		terminator := TimerInterval(streams.a, 10*time.Second).And(PatternFrom(streams.b, "nb", trueExpr).Not())
		repeat := PatternFrom(streams.a, "aarr", trueExpr).MatchUntil(0, 100).Until(terminator)
		pattern := PatternFrom(streams.a, "a1", trueExpr).Every().Then(repeat)
		plan, err := env.Build(pattern.Select(
			Alias("a1", TagField[string]("a1", "id")),
			Alias("aarr0", TagFieldAt[string]("aarr", 0, "id")),
		).Query(StatementName("s0"), DiscardPartialsOnMatch()))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		fires := []string{}
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					return fmt.Errorf("pattern result is not a row: %#v", result)
				}
				parts := []string{}
				for _, name := range []string{"a1", "aarr0"} {
					if value := row.Get(name); value.State() == ValuePresent {
						parts = append(parts, name+"="+value.Any().(string))
					}
				}
				fires = append(fires, joinStrings(parts, " "))
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
		patternConsumeSend(t, engine, patternIdEventA{ID: "A1"})
		advance(1000)
		patternConsumeSend(t, engine, patternIdEventA{ID: "A2"})
		advance(10000)
		patternConsumeAssert(t, "t=10000", &fires, "a1=A1 aarr0=A2")
		advance(11000)
		patternConsumeAssert(t, "t=11000", &fires)
	})
}

// TestPatternConsumingObserverOpMatchesEsper mirrors PatternObserverOp: a
// per-branch dynamic timer under the discard policy; the completed A2 match
// discards the A1 branch (they share B1), so the 5-second timer never fires.
func TestPatternConsumingObserverOpMatchesEsper(t *testing.T) {
	env, engine, streams := newPatternConsumeEnv(t, time.UnixMilli(0))
	defer func() { _ = engine.Close(context.Background()) }()
	trueExpr := Literal[bool](true)
	pattern := PatternFrom(streams.a, "a", trueExpr).Every().
		Then(PatternFrom(streams.b, "b", trueExpr)).
		Then(TimerIntervalExpr(streams.a, DurationSeconds[int](TagField[int]("a", "mysec"))))
	fires := patternConsumeCollect(t, env, engine, pattern, true, "a", "b")
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(ms).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	patternConsumeSend(t, engine, patternIdEventA{ID: "A1", Mysec: 5})
	patternConsumeSend(t, engine, patternIdEventA{ID: "A2", Mysec: 1})
	patternConsumeSend(t, engine, patternIdEventB{ID: "B1"})
	advance(1000)
	patternConsumeAssert(t, "t=1000", fires, "a=A2 b=B1")
	advance(5000)
	patternConsumeAssert(t, "t=5000", fires)
}

// TestPatternConsumingAndOpMatchesEsper mirrors PatternAndOp: and-branches
// with correlation under the discard policy, both with the and at sequence
// head and with a child sequence as one and-side.
func TestPatternConsumingAndOpMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	run := func(t *testing.T, discard bool, build func(streams patternConsumeStreams) PatternStream, stepC2Want ...string) {
		t.Helper()
		env, engine, streams := newPatternConsumeEnv(t, time.UnixMilli(0))
		defer func() { _ = engine.Close(context.Background()) }()
		fires := patternConsumeCollect(t, env, engine, build(streams), discard, "a", "b", "c")
		patternConsumeSend(t, engine, patternIdEventA{ID: "A1", Pa: "x"}, patternIdEventA{ID: "A2", Pa: "y"})
		patternConsumeSend(t, engine, patternIdEventB{ID: "B1"})
		patternConsumeSend(t, engine, patternIdEventC{ID: "C1", Pc: "y"})
		patternConsumeAssert(t, "C1(y)", fires, "a=A2 b=B1 c=C1")
		patternConsumeSend(t, engine, patternIdEventC{ID: "C2", Pc: "x"})
		patternConsumeAssert(t, "C2(x)", fires, stepC2Want...)
	}
	pcEq := Equal[string](Field[patternIdEventC, string]("pc"), TagField[string]("a", "pa"))
	t.Run("and-state-discard", func(t *testing.T) {
		run(t, true, func(streams patternConsumeStreams) PatternStream {
			andBranch := PatternFrom(streams.b, "b", trueExpr).And(PatternFrom(streams.c, "c", pcEq))
			return PatternFrom(streams.a, "a", trueExpr).Every().Then(andBranch)
		})
	})
	t.Run("and-state-keep", func(t *testing.T) {
		run(t, false, func(streams patternConsumeStreams) PatternStream {
			andBranch := PatternFrom(streams.b, "b", trueExpr).And(PatternFrom(streams.c, "c", pcEq))
			return PatternFrom(streams.a, "a", trueExpr).Every().Then(andBranch)
		}, "a=A1 b=B1 c=C2")
	})
}

// TestPatternConsumingAndChildSequenceMatchesEsper mirrors the D-event
// ordering of runAndWChild: the D event arrives between the two A events,
// before any B, so both and-states cache it.
func TestPatternConsumingAndChildSequenceMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	pcEq := Equal[string](Field[patternIdEventC, string]("pc"), TagField[string]("a", "pa"))
	run := func(t *testing.T, discard bool, stepC2Want ...string) {
		t.Helper()
		env, engine, streams := newPatternConsumeEnv(t, time.UnixMilli(0))
		defer func() { _ = engine.Close(context.Background()) }()
		child := PatternFrom(streams.b, "b", trueExpr).Then(PatternFrom(streams.c, "c", pcEq))
		andBranch := PatternFrom(streams.d, "dd", trueExpr).And(child)
		pattern := PatternFrom(streams.a, "a", trueExpr).Every().Then(andBranch)
		fires := patternConsumeCollect(t, env, engine, pattern, discard, "a", "b", "c")
		patternConsumeSend(t, engine, patternIdEventA{ID: "A1", Pa: "x"})
		patternConsumeSend(t, engine, patternIdEventA{ID: "A2", Pa: "y"}, patternIdEventD{ID: "D1"})
		patternConsumeSend(t, engine, patternIdEventB{ID: "B1"})
		patternConsumeSend(t, engine, patternIdEventC{ID: "C1", Pc: "y"})
		patternConsumeAssert(t, "C1(y)", fires, "a=A2 b=B1 c=C1")
		patternConsumeSend(t, engine, patternIdEventC{ID: "C2", Pc: "x"})
		patternConsumeAssert(t, "C2(x)", fires, stepC2Want...)
	}
	t.Run("discard", func(t *testing.T) { run(t, true) })
	t.Run("keep", func(t *testing.T) { run(t, false, "a=A1 b=B1 c=C2") })
}

// TestPatternConsumingNotOpMatchesEsper mirrors PatternNotOpNotImpacted:
// discard does not reach into a not-branch; the pending 5-second branch is
// discarded by the completed 1-second match, and the not would otherwise
// have been falsified by the B->C completion.
func TestPatternConsumingNotOpMatchesEsper(t *testing.T) {
	env, engine, streams := newPatternConsumeEnv(t, time.UnixMilli(0))
	defer func() { _ = engine.Close(context.Background()) }()
	trueExpr := Literal[bool](true)
	notBranch := PatternFrom(streams.b, "nb", trueExpr).Then(PatternFrom(streams.c, "nc", trueExpr)).Not()
	andBranch := TimerIntervalExpr(streams.a, DurationSeconds[int](TagField[int]("a", "mysec"))).And(notBranch)
	pattern := PatternFrom(streams.a, "a", trueExpr).Every().Then(andBranch)
	fires := patternConsumeCollect(t, env, engine, pattern, true, "a")
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(ms).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	patternConsumeSend(t, engine, patternIdEventA{ID: "A1", Mysec: 5})
	patternConsumeSend(t, engine, patternIdEventA{ID: "A2", Mysec: 1})
	patternConsumeSend(t, engine, patternIdEventB{ID: "B1"})
	advance(1000)
	patternConsumeAssert(t, "t=1000", fires, "a=A2")
	patternConsumeSend(t, engine, patternIdEventC{ID: "C1"})
	advance(5000)
	patternConsumeAssert(t, "t=5000", fires)
}

// TestPatternConsumingGuardOpMatchesEsper mirrors PatternGuardOp: the
// timer:within guard attached to the last filter (begin state) and to the
// child sequence (child state) under the discard policy.
func TestPatternConsumingGuardOpMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	pcEq := Equal[string](Field[patternIdEventC, string]("pc"), TagField[string]("a", "pa"))
	run := func(t *testing.T, discard bool, build func(streams patternConsumeStreams) PatternStream, stepC2Want ...string) {
		t.Helper()
		env, engine, streams := newPatternConsumeEnv(t, time.UnixMilli(0))
		defer func() { _ = engine.Close(context.Background()) }()
		fires := patternConsumeCollect(t, env, engine, build(streams), discard, "a", "b", "c")
		patternConsumeSend(t, engine, patternIdEventA{ID: "A1", Pa: "x"}, patternIdEventA{ID: "A2", Pa: "y"})
		patternConsumeSend(t, engine, patternIdEventB{ID: "B1"})
		patternConsumeSend(t, engine, patternIdEventC{ID: "C1", Pc: "y"})
		patternConsumeAssert(t, "C1(y)", fires, "a=A2 b=B1 c=C1")
		patternConsumeSend(t, engine, patternIdEventC{ID: "C2", Pc: "x"})
		patternConsumeAssert(t, "C2(x)", fires, stepC2Want...)
	}
	t.Run("begin-state-discard", func(t *testing.T) {
		run(t, true, func(streams patternConsumeStreams) PatternStream {
			return PatternFrom(streams.a, "a", trueExpr).Every().
				Then(PatternFrom(streams.b, "b", trueExpr)).
				Then(PatternFrom(streams.c, "c", pcEq).Within(time.Second))
		})
	})
	t.Run("begin-state-keep", func(t *testing.T) {
		run(t, false, func(streams patternConsumeStreams) PatternStream {
			return PatternFrom(streams.a, "a", trueExpr).Every().
				Then(PatternFrom(streams.b, "b", trueExpr)).
				Then(PatternFrom(streams.c, "c", pcEq).Within(time.Second))
		}, "a=A1 b=B1 c=C2")
	})
	t.Run("child-state-discard", func(t *testing.T) {
		run(t, true, func(streams patternConsumeStreams) PatternStream {
			child := PatternFrom(streams.b, "b", trueExpr).Then(PatternFrom(streams.c, "c", pcEq)).Within(time.Second)
			return PatternFrom(streams.a, "a", trueExpr).Every().Then(child)
		})
	})
	t.Run("child-state-keep", func(t *testing.T) {
		run(t, false, func(streams patternConsumeStreams) PatternStream {
			child := PatternFrom(streams.b, "b", trueExpr).Then(PatternFrom(streams.c, "c", pcEq)).Within(time.Second)
			return PatternFrom(streams.a, "a", trueExpr).Every().Then(child)
		}, "a=A1 b=B1 c=C2")
	})
}

// TestPatternConsumingEveryOpMatchesEsper mirrors PatternEveryOp: discard at
// the every begin state (plain and every-distinct forms) and at the every
// child state with and without a distinct key or expiry.
func TestPatternConsumingEveryOpMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	runBeginState := func(t *testing.T, wrap func(base PatternStream) PatternStream) {
		t.Helper()
		env, engine, streams := newPatternConsumeEnv(t, time.UnixMilli(0))
		defer func() { _ = engine.Close(context.Background()) }()
		pattern := PatternFrom(streams.a, "a", trueExpr).Every().Then(wrap(PatternFrom(streams.b, "b", trueExpr)))
		fires := patternConsumeCollect(t, env, engine, pattern, true, "a", "b")
		patternConsumeSend(t, engine, patternIdEventA{ID: "A1"})
		patternConsumeSend(t, engine, patternIdEventB{ID: "B1"})
		patternConsumeAssert(t, "B1", fires, "a=A1 b=B1")
		patternConsumeSend(t, engine, patternIdEventB{ID: "B2"})
		patternConsumeAssert(t, "B2", fires)
		patternConsumeSend(t, engine, patternIdEventA{ID: "A2"})
		patternConsumeSend(t, engine, patternIdEventB{ID: "B3"})
		patternConsumeAssert(t, "B3", fires, "a=A2 b=B3")
		patternConsumeSend(t, engine, patternIdEventB{ID: "B4"})
		patternConsumeAssert(t, "B4", fires)
	}
	t.Run("begin-state", func(t *testing.T) {
		runBeginState(t, func(base PatternStream) PatternStream { return base.Every() })
	})
	t.Run("begin-state-distinct", func(t *testing.T) {
		runBeginState(t, func(base PatternStream) PatternStream { return base.EveryDistinct(TagField[string]("b", "id")) })
	})
	t.Run("begin-state-distinct-expiry", func(t *testing.T) {
		runBeginState(t, func(base PatternStream) PatternStream {
			return base.EveryDistinctFor(10*time.Second, TagField[string]("b", "id"))
		})
	})

	runChildState := func(t *testing.T, discard bool, wrap func(base PatternStream) PatternStream, stepC2Want ...string) {
		t.Helper()
		env, engine, streams := newPatternConsumeEnv(t, time.UnixMilli(0))
		defer func() { _ = engine.Close(context.Background()) }()
		pcEq := Equal[string](Field[patternIdEventC, string]("pc"), TagField[string]("a", "pa"))
		child := PatternFrom(streams.b, "b", trueExpr).Then(PatternFrom(streams.c, "c", pcEq))
		pattern := PatternFrom(streams.a, "a", trueExpr).Every().Then(wrap(child))
		fires := patternConsumeCollect(t, env, engine, pattern, discard, "a", "b", "c")
		patternConsumeSend(t, engine, patternIdEventA{ID: "A1", Pa: "x"}, patternIdEventA{ID: "A2", Pa: "y"})
		patternConsumeSend(t, engine, patternIdEventB{ID: "B1"})
		patternConsumeSend(t, engine, patternIdEventC{ID: "C1", Pc: "y"})
		patternConsumeAssert(t, "C1(y)", fires, "a=A2 b=B1 c=C1")
		patternConsumeSend(t, engine, patternIdEventC{ID: "C2", Pc: "x"})
		patternConsumeAssert(t, "C2(x)", fires, stepC2Want...)
	}
	t.Run("child-state-discard", func(t *testing.T) {
		runChildState(t, true, func(base PatternStream) PatternStream { return base.Every() })
	})
	t.Run("child-state-keep", func(t *testing.T) {
		runChildState(t, false, func(base PatternStream) PatternStream { return base.Every() }, "a=A1 b=B1 c=C2")
	})
	t.Run("child-state-distinct-discard", func(t *testing.T) {
		runChildState(t, true, func(base PatternStream) PatternStream { return base.EveryDistinct(TagField[string]("b", "id")) })
	})
	t.Run("child-state-distinct-keep", func(t *testing.T) {
		runChildState(t, false, func(base PatternStream) PatternStream { return base.EveryDistinct(TagField[string]("b", "id")) }, "a=A1 b=B1 c=C2")
	})
	t.Run("child-state-distinct-expiry-discard", func(t *testing.T) {
		runChildState(t, true, func(base PatternStream) PatternStream {
			return base.EveryDistinctFor(10*time.Second, TagField[string]("b", "id"))
		})
	})
	t.Run("child-state-distinct-expiry-keep", func(t *testing.T) {
		runChildState(t, false, func(base PatternStream) PatternStream {
			return base.EveryDistinctFor(10*time.Second, TagField[string]("b", "id"))
		}, "a=A1 b=B1 c=C2")
	})
}

// TestPatternConsumingInvalidMatchesEsper mirrors PatternInvalid for the
// checks the fluent API can express: consumption policies require a pattern
// query and are rejected with context, joins or actions at Build. The
// unknown-annotation case is an EPL-parser concern with no fluent surface
// (registered Go-style difference).
func TestPatternConsumingInvalidMatchesEsper(t *testing.T) {
	env, _, streams := newPatternConsumeEnv(t, time.UnixMilli(0))
	for name, option := range map[string]QueryOption{
		"discard-partials":     DiscardPartialsOnMatch(),
		"suppress-overlapping": SuppressOverlappingMatches(),
	} {
		if _, err := env.Build(Select(streams.a, Alias("id", Field[patternIdEventA, string]("id"))).Query(option)); err == nil {
			t.Fatalf("%s was accepted for a non-pattern query", name)
		}
	}
	if _, err := env.Build(PatternFrom(streams.a, "a", Literal[bool](true)).Select(Alias("id", TagField[string]("a", "id"))).Query(DiscardPartialsOnMatch())); err != nil {
		t.Fatalf("pattern query with discard-partials must build: %v", err)
	}
	if _, err := env.Build(PatternFrom(streams.a, "a", Literal[bool](true)).Select(Alias("id", TagField[string]("a", "id"))).Query(SuppressOverlappingMatches())); err != nil {
		t.Fatalf("pattern query with suppress-overlapping must build: %v", err)
	}
}
