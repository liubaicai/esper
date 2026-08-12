package esper

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"
)

// Parity coverage for PatternOperatorEveryDistinct (see docs
// esper-go-port-implementation-plan.md). Expectations mirror the Java
// regression executions; key-reset and per-instance keyset semantics were
// verified against EvalEveryDistinctStateNode (spawnedNodes maps each spawned
// child to its own key set, copied on completion-spawn and emptied on
// falsification-respawn).

type patternIntArrayBean struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
}

func newPatternDistinctEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternOpBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternIntArrayBean](env, "SupportEventWithIntArray"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

// patternDistinctCollector subscribes to the deployed statement and flattens
// every fire into sorted field=value strings for multiset comparison.
type patternDistinctCollector struct {
	aliases []string
	fires   [][]string
}

func newPatternDistinctCollector(t *testing.T, env *Environment, engine *Engine, pattern PatternStream, selections ...Selection) *patternDistinctCollector {
	t.Helper()
	plan, err := env.Build(pattern.Select(selections...).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	collector := &patternDistinctCollector{}
	for _, selection := range selections {
		collector.aliases = append(collector.aliases, selection.Name)
	}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("pattern result is not a row: %#v", result)
			}
			parts := make([]string, 0, len(collector.aliases))
			for _, alias := range collector.aliases {
				if value := row.Get(alias); value.State() == ValuePresent {
					parts = append(parts, alias+"="+fmt.Sprintf("%v", value.Any()))
				}
			}
			sort.Strings(parts)
			collector.fires = append(collector.fires, parts)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return collector
}

// step runs an action and asserts the fires it produced, compared as a
// multiset of flattened field=value strings.
func (c *patternDistinctCollector) step(t *testing.T, action func(), want ...[]string) {
	t.Helper()
	before := len(c.fires)
	action()
	got := c.fires[before:]
	flatten := func(fires [][]string) []string {
		out := make([]string, 0, len(fires))
		for _, fire := range fires {
			out = append(out, fmt.Sprintf("%v", fire))
		}
		sort.Strings(out)
		return out
	}
	if fmt.Sprintf("%v", flatten(got)) != fmt.Sprintf("%v", flatten(want)) {
		t.Fatalf("fires = %v, want %v (all fires: %v)", got, want, c.fires)
	}
}

func sendSupportBeanDistinct(t *testing.T, engine *Engine, theString string, intPrim int) {
	t.Helper()
	if err := engine.Send(context.Background(), "SupportBean", patternOpBean{TheString: theString, IntPrimitive: intPrim}); err != nil {
		t.Fatal(err)
	}
}

func advancePatternDistinct(t *testing.T, engine *Engine, millis int64) {
	t.Helper()
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(millis).UTC()); err != nil {
		t.Fatal(err)
	}
}

func likeA(theString string) Expression[bool] {
	return LikeOf(Field[patternOpBean, string]("theString"), Literal(theString))
}

// TestPatternEveryDistinctSimpleMatchesEsper covers PatternEveryDistinctSimple:
// the first event per distinct key fires, later duplicates are swallowed.
func TestPatternEveryDistinctSimpleMatchesEsper(t *testing.T) {
	env, engine := newPatternDistinctEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	collector := newPatternDistinctCollector(t, env, engine,
		PatternFrom(base, "a", Literal[bool](true)).EveryDistinct(Field[patternOpBean, string]("theString")),
		Alias("c0", TagField[string]("a", "theString")),
	)
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 0) }, []string{"c0=E1"})
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 0) })
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E2", 0) }, []string{"c0=E2"})
	collector.step(t, func() {
		sendSupportBeanDistinct(t, engine, "E1", 0)
		sendSupportBeanDistinct(t, engine, "E2", 0)
	})
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E3", 0) }, []string{"c0=E3"})
}

// TestPatternEveryDistinctWTimeMatchesEsper covers PatternEveryDistinctWTime:
// keys expire exactly at first-seen + expiry on the virtual clock.
func TestPatternEveryDistinctWTimeMatchesEsper(t *testing.T) {
	env, _ := newPatternDistinctEnv(t)
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	collector := newPatternDistinctCollector(t, env, engine,
		PatternFrom(base, "a", Literal[bool](true)).EveryDistinctFor(5*time.Second, TagField[string]("a", "theString")),
		Alias("c0", TagField[string]("a", "theString")),
	)
	advancePatternDistinct(t, engine, 15000)
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 0) }, []string{"c0=E1"})
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 0) })
	advancePatternDistinct(t, engine, 18000)
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 0) })
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E2", 0) }, []string{"c0=E2"})
	advancePatternDistinct(t, engine, 19999)
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 0) })
	advancePatternDistinct(t, engine, 20000)
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 0) }, []string{"c0=E1"})
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E2", 0) })
	collector.step(t, func() {
		sendSupportBeanDistinct(t, engine, "E1", 0)
		sendSupportBeanDistinct(t, engine, "E2", 0)
	})
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E3", 0) }, []string{"c0=E3"})
}

// TestPatternExpireSeenBeforeKeyMatchesEsper covers PatternExpireSeenBeforeKey:
// per-key expiry measured from each key's first sighting.
func TestPatternExpireSeenBeforeKeyMatchesEsper(t *testing.T) {
	env, _ := newPatternDistinctEnv(t)
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	collector := newPatternDistinctCollector(t, env, engine,
		PatternFrom(base, "a", likeA("A%")).EveryDistinctFor(time.Second, TagField[int]("a", "intPrimitive")),
		Alias("c0", TagField[string]("a", "theString")),
	)
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A1", 1) }, []string{"c0=A1"})
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A2", 1) })
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A3", 2) }, []string{"c0=A3"})
	collector.step(t, func() {
		sendSupportBeanDistinct(t, engine, "A4", 1)
		sendSupportBeanDistinct(t, engine, "A5", 2)
	})
	advancePatternDistinct(t, engine, 1000)
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A4", 1) }, []string{"c0=A4"})
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A5", 2) }, []string{"c0=A5"})
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A6", 1) })
	advancePatternDistinct(t, engine, 1999)
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A7", 2) })
	advancePatternDistinct(t, engine, 2000)
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A7", 2) }, []string{"c0=A7"})
}

// TestPatternEveryDistinctOverFilterMatchesEsper covers
// PatternEveryDistinctOverFilter, both the plain and the time-bounded form
// (the Java case never advances the clock far enough to trigger the 2-minute
// expiry, so both variants share the expected sequence).
func TestPatternEveryDistinctOverFilterMatchesEsper(t *testing.T) {
	variants := []struct {
		name  string
		build func(base Stream[patternOpBean]) PatternStream
	}{
		{"no expiry", func(base Stream[patternOpBean]) PatternStream {
			return PatternFrom(base, "a", Literal[bool](true)).EveryDistinct(Field[patternOpBean, int]("intPrimitive"))
		}},
		{"2 minute expiry", func(base Stream[patternOpBean]) PatternStream {
			return PatternFrom(base, "a", Literal[bool](true)).EveryDistinctFor(2*time.Minute, Field[patternOpBean, int]("intPrimitive"))
		}},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			env, engine := newPatternDistinctEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			base := From[patternOpBean](env, "SupportBean")
			collector := newPatternDistinctCollector(t, env, engine, variant.build(base),
				Alias("c0", TagField[string]("a", "theString")),
			)
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 1) }, []string{"c0=E1"})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E2", 1) })
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E3", 2) }, []string{"c0=E3"})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E4", 3) }, []string{"c0=E4"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "E5", 2)
				sendSupportBeanDistinct(t, engine, "E6", 3)
				sendSupportBeanDistinct(t, engine, "E7", 1)
			})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E8", 0) }, []string{"c0=E8"})
		})
	}
}

// TestPatternEveryDistinctOverAndMatchesEsper covers PatternEveryDistinctOverAnd:
// the composite key combines both sides of the conjunction.
func TestPatternEveryDistinctOverAndMatchesEsper(t *testing.T) {
	variants := []struct {
		name  string
		build func(a, b PatternStream) PatternStream
	}{
		{"no expiry", func(a, b PatternStream) PatternStream {
			return a.And(b).EveryDistinct(TagField[int]("a", "intPrimitive"), TagField[int]("b", "intPrimitive"))
		}},
		{"1 hour expiry", func(a, b PatternStream) PatternStream {
			return a.And(b).EveryDistinctFor(time.Hour, TagField[int]("a", "intPrimitive"), TagField[int]("b", "intPrimitive"))
		}},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			env, engine := newPatternDistinctEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			base := From[patternOpBean](env, "SupportBean")
			a := PatternFrom(base, "a", likeA("A%"))
			b := PatternFrom(base, "b", likeA("B%"))
			collector := newPatternDistinctCollector(t, env, engine, variant.build(a, b),
				Alias("a", TagField[string]("a", "theString")),
				Alias("b", TagField[string]("b", "theString")),
			)
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A1", 1) })
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B1", 10) }, []string{"a=A1", "b=B1"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A2", 1)
				sendSupportBeanDistinct(t, engine, "B2", 10)
				sendSupportBeanDistinct(t, engine, "A3", 2)
			})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B3", 10) }, []string{"a=A3", "b=B3"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A4", 1)
				sendSupportBeanDistinct(t, engine, "B4", 20)
			}, []string{"a=A4", "b=B4"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A5", 2)
				sendSupportBeanDistinct(t, engine, "B5", 10)
			})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A6", 2)
				sendSupportBeanDistinct(t, engine, "B6", 20)
			}, []string{"a=A6", "b=B6"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A7", 2)
				sendSupportBeanDistinct(t, engine, "B7", 20)
			})
		})
	}
}

// TestPatternEveryDistinctOverOrMatchesEsper covers PatternEveryDistinctOverOr:
// the key coalesces the firing branch, so either side feeds the same key space.
func TestPatternEveryDistinctOverOrMatchesEsper(t *testing.T) {
	key := func() Expression[int] {
		return Add[int](
			Coalesce[int](TagField[int]("a", "intPrimitive"), Literal(0)),
			Coalesce[int](TagField[int]("b", "intPrimitive"), Literal(0)),
		)
	}
	variants := []struct {
		name  string
		build func(a, b PatternStream) PatternStream
	}{
		{"no expiry", func(a, b PatternStream) PatternStream {
			return a.Or(b).EveryDistinct(key())
		}},
		{"1 hour expiry", func(a, b PatternStream) PatternStream {
			return a.Or(b).EveryDistinctFor(time.Hour, key())
		}},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			env, engine := newPatternDistinctEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			base := From[patternOpBean](env, "SupportBean")
			a := PatternFrom(base, "a", likeA("A%"))
			b := PatternFrom(base, "b", likeA("B%"))
			collector := newPatternDistinctCollector(t, env, engine, variant.build(a, b),
				Alias("a", TagField[string]("a", "theString")),
				Alias("b", TagField[string]("b", "theString")),
			)
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A1", 1) }, []string{"a=A1"})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B1", 2) }, []string{"b=B1"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "B2", 1)
				sendSupportBeanDistinct(t, engine, "A2", 2)
				sendSupportBeanDistinct(t, engine, "A3", 2)
				sendSupportBeanDistinct(t, engine, "B3", 1)
			})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B4", 3) }, []string{"b=B4"})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B5", 4) }, []string{"b=B5"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "B6", 3)
				sendSupportBeanDistinct(t, engine, "A4", 3)
				sendSupportBeanDistinct(t, engine, "A5", 4)
			})
		})
	}
}

// TestPatternEveryDistinctOverNotMatchesEsper covers PatternEveryDistinctOverNot:
// when the forbidden event falsifies the current attempt, the respawned
// attempt restarts with an empty key set (EvalEveryDistinctStateNode
// evaluateFalse spawns without copying keys), so A4 with a previously seen
// key fires again.
func TestPatternEveryDistinctOverNotMatchesEsper(t *testing.T) {
	variants := []struct {
		name  string
		build func(a, b PatternStream) PatternStream
	}{
		{"no expiry", func(a, b PatternStream) PatternStream {
			return a.And(b.Not()).EveryDistinct(TagField[int]("a", "intPrimitive"))
		}},
		{"1 hour expiry", func(a, b PatternStream) PatternStream {
			return a.And(b.Not()).EveryDistinctFor(time.Hour, TagField[int]("a", "intPrimitive"))
		}},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			env, engine := newPatternDistinctEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			base := From[patternOpBean](env, "SupportBean")
			a := PatternFrom(base, "a", likeA("A%"))
			b := PatternFrom(base, "b", likeA("B%"))
			collector := newPatternDistinctCollector(t, env, engine, variant.build(a, b),
				Alias("a", TagField[string]("a", "theString")),
			)
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A1", 1) }, []string{"a=A1"})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A2", 1) })
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A3", 2) }, []string{"a=A3"})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B1", 1) })
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A4", 1) }, []string{"a=A4"})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A5", 1) })
		})
	}
}

// TestPatternRepeatOverDistinctMatchesEsper covers PatternRepeatOverDistinct:
// [2] repeat over an every-distinct filter collects the first two distinct
// matches into the tag array.
func TestPatternRepeatOverDistinctMatchesEsper(t *testing.T) {
	variants := []struct {
		name  string
		build func(base Stream[patternOpBean]) PatternStream
	}{
		{"no expiry", func(base Stream[patternOpBean]) PatternStream {
			return PatternFrom(base, "a", Literal[bool](true)).EveryDistinct(TagField[int]("a", "intPrimitive")).MatchUntil(2, 2)
		}},
		{"1 hour expiry", func(base Stream[patternOpBean]) PatternStream {
			return PatternFrom(base, "a", Literal[bool](true)).EveryDistinctFor(time.Hour, TagField[int]("a", "intPrimitive")).MatchUntil(2, 2)
		}},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			env, engine := newPatternDistinctEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			base := From[patternOpBean](env, "SupportBean")
			collector := newPatternDistinctCollector(t, env, engine, variant.build(base),
				Alias("a0", TagFieldAt[string]("a", 0, "theString")),
				Alias("a1", TagFieldAt[string]("a", 1, "theString")),
			)
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "E1", 1)
				sendSupportBeanDistinct(t, engine, "E2", 1)
			})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E3", 2) }, []string{"a0=E1", "a1=E3"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "E4", 3)
				sendSupportBeanDistinct(t, engine, "E5", 2)
			})
		})
	}
}

// TestPatternEveryDistinctOverRepeatMatchesEsper covers
// PatternEveryDistinctOverRepeat: the key reads the first element of the
// repeated tag array, so a repeated start value suppresses the completion.
func TestPatternEveryDistinctOverRepeatMatchesEsper(t *testing.T) {
	variants := []struct {
		name  string
		build func(base Stream[patternOpBean]) PatternStream
	}{
		{"no expiry", func(base Stream[patternOpBean]) PatternStream {
			return PatternFrom(base, "a", Literal[bool](true)).MatchUntil(2, 2).EveryDistinct(TagFieldAt[int]("a", 0, "intPrimitive"))
		}},
		{"1 hour expiry multikey", func(base Stream[patternOpBean]) PatternStream {
			return PatternFrom(base, "a", Literal[bool](true)).MatchUntil(2, 2).EveryDistinctFor(time.Hour, TagFieldAt[int]("a", 0, "intPrimitive"), TagFieldAt[int]("a", 0, "intPrimitive"))
		}},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			env, engine := newPatternDistinctEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			base := From[patternOpBean](env, "SupportBean")
			collector := newPatternDistinctCollector(t, env, engine, variant.build(base),
				Alias("a0", TagFieldAt[string]("a", 0, "theString")),
				Alias("a1", TagFieldAt[string]("a", 1, "theString")),
			)
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 1) })
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E2", 1) }, []string{"a0=E1", "a1=E2"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "E3", 1)
				sendSupportBeanDistinct(t, engine, "E4", 2)
				sendSupportBeanDistinct(t, engine, "E5", 2)
			})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E6", 1) }, []string{"a0=E5", "a1=E6"})
		})
	}
}

// TestPatternEveryDistinctOverFollowedByMatchesEsper covers
// PatternEveryDistinctOverFollowedBy: the key sums both sides of the
// followed-by, so identical sums suppress later completions.
func TestPatternEveryDistinctOverFollowedByMatchesEsper(t *testing.T) {
	variants := []struct {
		name  string
		build func(a, b PatternStream) PatternStream
	}{
		{"no expiry", func(a, b PatternStream) PatternStream {
			return a.Then(b).EveryDistinct(Add[int](TagField[int]("a", "intPrimitive"), TagField[int]("b", "intPrimitive")))
		}},
		{"1 hour expiry", func(a, b PatternStream) PatternStream {
			return a.Then(b).EveryDistinctFor(time.Hour, Add[int](TagField[int]("a", "intPrimitive"), TagField[int]("b", "intPrimitive")))
		}},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			env, engine := newPatternDistinctEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			base := From[patternOpBean](env, "SupportBean")
			a := PatternFrom(base, "a", likeA("A%"))
			b := PatternFrom(base, "b", likeA("B%"))
			collector := newPatternDistinctCollector(t, env, engine, variant.build(a, b),
				Alias("a", TagField[string]("a", "theString")),
				Alias("b", TagField[string]("b", "theString")),
			)
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A1", 1)
				sendSupportBeanDistinct(t, engine, "B1", 1)
			}, []string{"a=A1", "b=B1"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A2", 1)
				sendSupportBeanDistinct(t, engine, "B2", 1)
			})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A3", 10)
				sendSupportBeanDistinct(t, engine, "B3", -8)
			})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A4", 2) })
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B4", 1) }, []string{"a=A4", "b=B4"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A5", 3)
				sendSupportBeanDistinct(t, engine, "B5", 0)
			})
		})
	}
}

// TestPatternEveryDistinctWithinFollowedByMatchesEsper covers
// PatternEveryDistinctWithinFollowedBy: the left every-distinct spawns one
// waiting branch per fresh key, each correlating b on its own a value.
func TestPatternEveryDistinctWithinFollowedByMatchesEsper(t *testing.T) {
	variants := []struct {
		name  string
		build func(a, b PatternStream) PatternStream
	}{
		{"no expiry", func(a, b PatternStream) PatternStream {
			return a.EveryDistinct(TagField[int]("a", "intPrimitive")).Then(b)
		}},
		{"2h1m expiry", func(a, b PatternStream) PatternStream {
			return a.EveryDistinctFor(2*time.Hour+time.Minute, TagField[int]("a", "intPrimitive")).Then(b)
		}},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			env, engine := newPatternDistinctEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			base := From[patternOpBean](env, "SupportBean")
			a := PatternFrom(base, "a", likeA("A%"))
			b := PatternFrom(base, "b", Equal[int](Field[patternOpBean, int]("intPrimitive"), TagField[int]("a", "intPrimitive")))
			collector := newPatternDistinctCollector(t, env, engine, variant.build(a, b),
				Alias("a", TagField[string]("a", "theString")),
				Alias("b", TagField[string]("b", "theString")),
			)
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A1", 1)
				sendSupportBeanDistinct(t, engine, "B1", 0)
			})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B2", 1) }, []string{"a=A1", "b=B2"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A2", 2)
				sendSupportBeanDistinct(t, engine, "A3", 3)
				sendSupportBeanDistinct(t, engine, "A4", 1)
			})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B3", 3) }, []string{"a=A3", "b=B3"})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B4", 1) })
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B5", 2) }, []string{"a=A2", "b=B5"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A5", 2)
				sendSupportBeanDistinct(t, engine, "B6", 2)
			})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A6", 4)
				sendSupportBeanDistinct(t, engine, "B7", 4)
			}, []string{"a=A6", "b=B7"})
		})
	}
}

// TestPatternFollowedByWithDistinctMatchesEsper covers
// PatternFollowedByWithDistinct: both followed-by sides carry their own
// every-distinct; each waiting branch owns an independent right-side keyset,
// so one B can complete several retained branches.
func TestPatternFollowedByWithDistinctMatchesEsper(t *testing.T) {
	variants := []struct {
		name  string
		build func(a, b PatternStream) PatternStream
	}{
		{"no expiry", func(a, b PatternStream) PatternStream {
			return a.EveryDistinct(TagField[int]("a", "intPrimitive")).Then(b.EveryDistinct(TagField[int]("b", "intPrimitive")))
		}},
		{"1 day expiry", func(a, b PatternStream) PatternStream {
			return a.EveryDistinctFor(24*time.Hour, TagField[int]("a", "intPrimitive")).Then(b.EveryDistinctFor(24*time.Hour, TagField[int]("b", "intPrimitive")))
		}},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			env, engine := newPatternDistinctEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			base := From[patternOpBean](env, "SupportBean")
			a := PatternFrom(base, "a", likeA("A%"))
			b := PatternFrom(base, "b", likeA("B%"))
			collector := newPatternDistinctCollector(t, env, engine, variant.build(a, b),
				Alias("a", TagField[string]("a", "theString")),
				Alias("b", TagField[string]("b", "theString")),
			)
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A1", 1) })
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B1", 0) }, []string{"a=A1", "b=B1"})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B2", 1) }, []string{"a=A1", "b=B2"})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B3", 0) })
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "A2", 1) })
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B4", 2) }, []string{"a=A1", "b=B4"})
			collector.step(t, func() {
				sendSupportBeanDistinct(t, engine, "A3", 2)
				sendSupportBeanDistinct(t, engine, "B5", 1)
			}, []string{"a=A3", "b=B5"})
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B6", 1) })
			collector.step(t, func() { sendSupportBeanDistinct(t, engine, "B7", 3) },
				[]string{"a=A1", "b=B7"}, []string{"a=A3", "b=B7"})
		})
	}
}

// TestPatternEveryDistinctInvalidMatchesEsper covers PatternInvalid: constant
// keys and keys referencing tags outside the distinct subexpression fail at
// Build time (Go-style error text; Java messages name the EPL property).
func TestPatternEveryDistinctInvalidMatchesEsper(t *testing.T) {
	env, engine := newPatternDistinctEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")

	// Java: a=SupportBean_A -> every-distinct(a.intPrimitive) SupportBean_B --
	// the key references a tag the distinct subexpression does not produce.
	foreignTag := PatternFrom(base, "a", likeA("A%")).Then(
		PatternFrom(base, "b", likeA("B%")).EveryDistinct(TagField[int]("a", "intPrimitive")),
	)
	if _, err := env.Build(foreignTag.Select(Alias("b", TagField[string]("b", "theString"))).Query()); err == nil {
		t.Fatal("every-distinct with a foreign-tag key was accepted")
	}

	// Java: every-distinct(dummy) SupportBean_A -- unknown property.
	unknownField := PatternFrom(base, "a", Literal[bool](true)).EveryDistinct(Field[patternOpBean, int]("dummy"))
	if _, err := env.Build(unknownField.Select(Alias("c0", TagField[string]("a", "theString"))).Query()); err == nil {
		t.Fatal("every-distinct with an unknown-field key was accepted")
	}

	// Java: every-distinct(2 sec) SupportBean_A -- constant key.
	constantKey := PatternFrom(base, "a", Literal[bool](true)).EveryDistinct(Literal(2 * time.Second))
	if _, err := env.Build(constantKey.Select(Alias("c0", TagField[string]("a", "theString"))).Query()); err == nil {
		t.Fatal("every-distinct with a constant key was accepted")
	}
}

// TestPatternEveryDistinctMonthScopedMatchesEsper covers PatternMonthScoped:
// a calendar-month expiry shifts the first-seen time by one month.
func TestPatternEveryDistinctMonthScopedMatchesEsper(t *testing.T) {
	env, _ := newPatternDistinctEnv(t)
	start := time.Date(2002, 2, 1, 9, 0, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternOpBean](env, "SupportBean")
	collector := newPatternDistinctCollector(t, env, engine,
		PatternFrom(base, "a", Literal[bool](true)).EveryDistinctForCalendar(0, 1, 0, TagField[string]("a", "theString")),
		Alias("c0", TagField[string]("a", "theString")),
		Alias("c1", TagField[int]("a", "intPrimitive")),
	)
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 1) }, []string{"c0=E1", "c1=1"})
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 2) })
	// One millisecond before the one-month mark the key is still alive.
	if err := engine.AdvanceTime(context.Background(), time.Date(2002, 3, 1, 9, 0, 0, 0, time.UTC).Add(-time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 3) })
	if err := engine.AdvanceTime(context.Background(), time.Date(2002, 3, 1, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	collector.step(t, func() { sendSupportBeanDistinct(t, engine, "E1", 4) }, []string{"c0=E1", "c1=4"})
}

// TestPatternEveryDistinctMultikeyWArrayMatchesEsper covers
// PatternEveryDistinctMultikeyWArray: array-valued keys compare by content
// and a null array is its own key.
func TestPatternEveryDistinctMultikeyWArrayMatchesEsper(t *testing.T) {
	env, engine := newPatternDistinctEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	base := From[patternIntArrayBean](env, "SupportEventWithIntArray")
	collector := newPatternDistinctCollector(t, env, engine,
		PatternFrom(base, "a", Literal[bool](true)).EveryDistinct(Field[patternIntArrayBean, []int]("array")),
		Alias("c0", TagField[string]("a", "id")),
	)
	send := func(id string, array []int) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportEventWithIntArray", patternIntArrayBean{ID: id, Array: array}); err != nil {
			t.Fatal(err)
		}
	}
	collector.step(t, func() { send("E1", []int{1, 2}) }, []string{"c0=E1"})
	collector.step(t, func() { send("E2", []int{1, 2}) })
	collector.step(t, func() { send("E3", []int{1}) }, []string{"c0=E3"})
	collector.step(t, func() { send("E4", []int{}) }, []string{"c0=E4"})
	collector.step(t, func() { send("E5", nil) }, []string{"c0=E5"})
	collector.step(t, func() {
		send("E10", []int{1, 2})
		send("E11", []int{1})
		send("E12", []int{})
		send("E13", nil)
	})
}
