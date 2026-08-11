package esper

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"time"
)

// Parity coverage for PatternUseResult (see docs
// esper-go-port-implementation-plan.md): tag-property correlation in
// followed-by filters (numeric, object-id, boolean), array-tag indexing in
// followed-by predicates, and the filter-optimization cases that verify tag
// references in pattern subexpressions.

type patternUseResultN struct {
	IntPrimitive  int     `esper:"intPrimitive"`
	IntBoxed      int     `esper:"intBoxed"`
	DoublePrim    float64 `esper:"doublePrimitive"`
	DoubleBoxed   float64 `esper:"doubleBoxed"`
	BoolPrimitive bool    `esper:"boolPrimitive"`
	BoolBoxed     bool    `esper:"boolBoxed"`
}

type patternUseResultS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

func patternUseResultNSet() []struct {
	id  string
	ms  int64
	evt patternUseResultN
} {
	return []struct {
		id  string
		ms  int64
		evt patternUseResultN
	}{
		{"N1", 1000, patternUseResultN{1, -56, 44.0, -60.5, true, true}},
		{"N2", 2000, patternUseResultN{66, 59, 48.0, 70.999, true, false}},
		{"N3", 3000, patternUseResultN{87, -5, 44.5, -23.5, false, true}},
		{"N4", 4000, patternUseResultN{86, -98, 42.1, -79.5, true, true}},
		{"N5", 5000, patternUseResultN{0, -33, 48.0, 44.45, true, false}},
		{"N6", 6000, patternUseResultN{55, -55, 44.0, -60.5, false, true}},
		{"N7", 7000, patternUseResultN{34, 92, 39.0, -66.5, false, true}},
		{"N8", 8000, patternUseResultN{100, 66, 47.5, 45.0, true, false}},
	}
}

func patternUseResultS0Set() []struct {
	id  string
	ms  int64
	evt patternUseResultS0
} {
	return []struct {
		id  string
		ms  int64
		evt patternUseResultS0
	}{
		{"e1", 1000, patternUseResultS0{1, "A"}},
		{"e2", 2000, patternUseResultS0{2, "B"}},
		{"e3", 3000, patternUseResultS0{3, "C"}},
		{"e4", 4000, patternUseResultS0{4, "D"}},
		{"e5", 5000, patternUseResultS0{5, "E"}},
		{"e6", 6000, patternUseResultS0{6, "B"}},
		{"e7", 7000, patternUseResultS0{7, "F"}},
		{"e8", 8000, patternUseResultS0{8, "C"}},
		{"e9", 9000, patternUseResultS0{9, "G"}},
		{"e10", 10000, patternUseResultS0{10, "G"}},
		{"e11", 11000, patternUseResultS0{11, "B"}},
		{"e12", 12000, patternUseResultS0{12, "F"}},
	}
}

type patternNCase struct {
	name  string
	tags  []string
	build func(s Stream[patternUseResultN]) PatternStream
	want  map[string][]string
}

type patternS0Case struct {
	name  string
	tags  []string
	build func(s Stream[patternUseResultS0]) PatternStream
	want  map[string][]string
}

func runPatternNCase(t *testing.T, testCase patternNCase) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternUseResultN](env, "SupportBean_N"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	stream := From[patternUseResultN](env, "SupportBean_N")
	selections := make([]Selection, 0, len(testCase.tags))
	for _, tag := range testCase.tags {
		selections = append(selections, Alias(tag, TagField[string](tag, "id")))
	}
	// SupportBean_N has no "id" field; use intPrimitive as the identifier
	selections = nil
	for _, tag := range testCase.tags {
		selections = append(selections, Alias(tag, TagField[int](tag, "intPrimitive")))
	}
	plan, err := env.Build(testCase.build(stream).Select(selections...).Query(StatementName("s0")))
	if err != nil {
		t.Fatalf("%s: build: %v", testCase.name, err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("%s: deploy: %v", testCase.name, err)
	}
	fires := map[string][]string{}
	currentEvent := ""
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("no row")
			}
			parts := []string{}
			for _, sel := range selections {
				v := row.Get(sel.Name)
				if v.State() == ValuePresent {
					parts = append(parts, fmt.Sprintf("%s=%v", sel.Name, v.Any()))
				}
			}
			sort.Strings(parts)
			fires[currentEvent] = append(fires[currentEvent], joinStrings(parts, " "))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, step := range patternUseResultNSet() {
		currentEvent = step.id
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(step.ms).UTC()); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), step.evt); err != nil {
			t.Fatal(err)
		}
	}
	assertPatternAndOrFires(t, testCase.name, testCase.want, fires)
}

func runPatternS0Case(t *testing.T, testCase patternS0Case) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternUseResultS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	stream := From[patternUseResultS0](env, "SupportBean_S0")
	selections := make([]Selection, 0, len(testCase.tags))
	for _, tag := range testCase.tags {
		selections = append(selections, Alias(tag+"_p00", TagField[string](tag, "p00")))
	}
	plan, err := env.Build(testCase.build(stream).Select(selections...).Query(StatementName("s0")))
	if err != nil {
		t.Fatalf("%s: build: %v", testCase.name, err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("%s: deploy: %v", testCase.name, err)
	}
	fires := map[string][]string{}
	currentEvent := ""
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("no row")
			}
			parts := []string{}
			for _, sel := range selections {
				v := row.Get(sel.Name)
				if v.State() == ValuePresent {
					parts = append(parts, fmt.Sprintf("%s=%v", sel.Name, v.Any()))
				}
			}
			sort.Strings(parts)
			fires[currentEvent] = append(fires[currentEvent], joinStrings(parts, " "))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, step := range patternUseResultS0Set() {
		currentEvent = step.id
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(step.ms).UTC()); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), step.evt); err != nil {
			t.Fatal(err)
		}
	}
	assertPatternAndOrFires(t, testCase.name, testCase.want, fires)
}

// TestPatternNumericMatchesEsper mirrors PatternNumeric: numeric-property
// correlation in followed-by filters using SupportBean_N.
func TestPatternNumericMatchesEsper(t *testing.T) {
	intF := Field[patternUseResultN, int]("intPrimitive")
	dblF := Field[patternUseResultN, float64]("doublePrimitive")
	boolPF := Field[patternUseResultN, bool]("boolPrimitive")
	boolBF := Field[patternUseResultN, bool]("boolBoxed")
	intB := Field[patternUseResultN, int]("intBoxed")

	cases := []patternNCase{
		{"na.dbl->nb.dbl", []string{"na", "nb"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Literal(true)).Then(PatternFrom(s, "nb",
				Equal[float64](dblF, TagField[float64]("na", "doublePrimitive"))))
		}, map[string][]string{"N6": {"na=1 nb=55"}}},
		{"na(87)->nb(>na.int)", []string{"na", "nb"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Equal[int](intF, Literal(87))).Then(PatternFrom(s, "nb",
				Greater[int](intF, TagField[int]("na", "intPrimitive"))))
		}, map[string][]string{"N8": {"na=87 nb=100"}}},
		{"na(87)->nb(<na.int)", []string{"na", "nb"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Equal[int](intF, Literal(87))).Then(PatternFrom(s, "nb",
				Less[int](intF, TagField[int]("na", "intPrimitive"))))
		}, map[string][]string{"N4": {"na=87 nb=86"}}},
		{"na(66)->every nb(>=na.int)", []string{"na", "nb"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Equal[int](intF, Literal(66))).Then(PatternFrom(s, "nb",
				GreaterOrEqual[int](intF, TagField[int]("na", "intPrimitive"))).Every())
		}, map[string][]string{
			"N3": {"na=66 nb=87"}, "N4": {"na=66 nb=86"}, "N8": {"na=66 nb=100"},
		}},
		{"na(boolBoxedF)->every nb(boolP=na.boolP)", []string{"na", "nb"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Equal[bool](boolBF, Literal(false))).Then(PatternFrom(s, "nb",
				Equal[bool](boolPF, TagField[bool]("na", "boolPrimitive"))).Every())
		}, map[string][]string{
			"N4": {"na=66 nb=86"}, "N5": {"na=66 nb=0"}, "N8": {"na=66 nb=100"},
		}},
		{"every na -> every nb(int=na.int) — no match", []string{"na", "nb"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Literal(true)).Every().Then(PatternFrom(s, "nb",
				Equal[int](intF, TagField[int]("na", "intPrimitive"))).Every())
		}, map[string][]string{}},
		{"every na -> every nb(dbl=na.dbl)", []string{"na", "nb"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Literal(true)).Every().Then(PatternFrom(s, "nb",
				Equal[float64](dblF, TagField[float64]("na", "doublePrimitive"))).Every())
		}, map[string][]string{
			"N5": {"na=66 nb=0"}, "N6": {"na=1 nb=55"},
		}},
		{"every na(boolBoxedF) -> every nb(boolBoxed=na.boolBoxed)", []string{"na", "nb"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Equal[bool](boolBF, Literal(false))).Every().Then(PatternFrom(s, "nb",
				Equal[bool](boolBF, TagField[bool]("na", "boolBoxed"))).Every())
		}, map[string][]string{
			"N5": {"na=66 nb=0"}, "N8": {"na=0 nb=100", "na=66 nb=100"},
		}},
		{"na->nb->nc triple followed-by", []string{"na", "nb", "nc"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Equal[bool](boolBF, Literal(false))).
				Then(PatternFrom(s, "nb", Less[int](intF, TagField[int]("na", "intPrimitive")))).
				Then(PatternFrom(s, "nc", Greater[int](intF, TagField[int]("nb", "intPrimitive"))))
		}, map[string][]string{"N6": {"na=66 nb=0 nc=55"}}},
		{"na(86)->nb->nc", []string{"na", "nb", "nc"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Equal[int](intF, Literal(86))).
				Then(PatternFrom(s, "nb", Less[int](intF, TagField[int]("na", "intPrimitive")))).
				Then(PatternFrom(s, "nc", Greater[int](intF, TagField[int]("na", "intPrimitive"))))
		}, map[string][]string{"N8": {"na=86 nb=0 nc=100"}}},
		{"na(86)->(nb or nc)", []string{"na", "nb", "nc"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Equal[int](intF, Literal(86))).
				Then(PatternFrom(s, "nb", Less[int](intF, TagField[int]("na", "intPrimitive"))).
					Or(PatternFrom(s, "nc", Greater[int](intF, TagField[int]("na", "intPrimitive")))))
		}, map[string][]string{"N5": {"na=86 nb=0"}}},
		{"na(86)->(nb(>na) or nc(intBoxed<na.intBoxed))", []string{"na", "nb", "nc"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Equal[int](intF, Literal(86))).
				Then(PatternFrom(s, "nb", Greater[int](intF, TagField[int]("na", "intPrimitive"))).
					Or(PatternFrom(s, "nc", Less[int](intB, TagField[int]("na", "intBoxed")))))
		}, map[string][]string{"N8": {"na=86 nb=100"}}},
		{"na(86)->(nb(>na) and nc(intBoxed<na.intBoxed)) — no match", []string{"na", "nb", "nc"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Equal[int](intF, Literal(86))).
				Then(PatternFrom(s, "nb", Greater[int](intF, TagField[int]("na", "intPrimitive"))).
					And(PatternFrom(s, "nc", Less[int](intB, TagField[int]("na", "intBoxed")))))
		}, map[string][]string{}},
		{"na->every nb(dbl in [0:na.dbl])", []string{"na", "nb"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Literal(true)).Then(PatternFrom(s, "nb",
				BetweenOf(dblF, Literal(0.0), TagField[float64]("na", "doublePrimitive"))).Every())
		}, map[string][]string{
			"N4": {"na=1 nb=86"}, "N6": {"na=1 nb=55"}, "N7": {"na=1 nb=34"},
		}},
		{"na->every nb(dbl in (0:na.dbl))", []string{"na", "nb"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Literal(true)).Then(PatternFrom(s, "nb",
				And(Greater[float64](dblF, Literal(0.0)), Less[float64](dblF, TagField[float64]("na", "doublePrimitive")))).Every())
		}, map[string][]string{
			"N4": {"na=1 nb=86"}, "N7": {"na=1 nb=34"},
		}},
		{"na->every nb(int in (na.int:na.dbl))", []string{"na", "nb"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Literal(true)).Then(PatternFrom(s, "nb",
				And(Greater[float64](Cast[int, float64](intF), Cast[int, float64](TagField[int]("na", "intPrimitive"))),
					Less[float64](Cast[int, float64](intF), TagField[float64]("na", "doublePrimitive")))).Every())
		}, map[string][]string{
			"N7": {"na=1 nb=34"},
		}},
		{"na->every nb(int in (na.int:60) v2)", []string{"na", "nb"}, func(s Stream[patternUseResultN]) PatternStream {
			return PatternFrom(s, "na", Literal(true)).Then(PatternFrom(s, "nb",
				And(Greater[int](intF, TagField[int]("na", "intPrimitive")), Less[int](intF, Literal(60)))).Every())
		}, map[string][]string{
			"N6": {"na=1 nb=55"}, "N7": {"na=1 nb=34"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runPatternNCase(t, tc)
		})
	}
}

// TestPatternObjectIdMatchesEsper mirrors PatternObjectId: string-property
// correlation in followed-by filters using SupportBean_S0.
func TestPatternObjectIdMatchesEsper(t *testing.T) {
	p00 := Field[patternUseResultS0, string]("p00")
	cases := []patternS0Case{
		{"X1->X2(p00=X1.p00) — no B before first match", []string{"X1", "X2"}, func(s Stream[patternUseResultS0]) PatternStream {
			return PatternFrom(s, "X1", Literal(true)).Then(PatternFrom(s, "X2",
				Equal[string](p00, TagField[string]("X1", "p00"))))
		}, map[string][]string{}},
		{"X1(p00=B)->X2(p00=X1.p00)", []string{"X1", "X2"}, func(s Stream[patternUseResultS0]) PatternStream {
			return PatternFrom(s, "X1", Equal[string](p00, Literal("B"))).Then(PatternFrom(s, "X2",
				Equal[string](p00, TagField[string]("X1", "p00"))))
		}, map[string][]string{"e6": {"X1_p00=B X2_p00=B"}}},
		{"X1(p00=B)->every X2(p00=X1.p00)", []string{"X1", "X2"}, func(s Stream[patternUseResultS0]) PatternStream {
			return PatternFrom(s, "X1", Equal[string](p00, Literal("B"))).Then(PatternFrom(s, "X2",
				Equal[string](p00, TagField[string]("X1", "p00"))).Every())
		}, map[string][]string{
			"e6": {"X1_p00=B X2_p00=B"}, "e11": {"X1_p00=B X2_p00=B"},
		}},
		{"every X1(p00=B)->every X2(p00=X1.p00)", []string{"X1", "X2"}, func(s Stream[patternUseResultS0]) PatternStream {
			return PatternFrom(s, "X1", Equal[string](p00, Literal("B"))).Every().Then(PatternFrom(s, "X2",
				Equal[string](p00, TagField[string]("X1", "p00"))).Every())
		}, map[string][]string{
			"e6": {"X1_p00=B X2_p00=B"}, "e11": {"X1_p00=B X2_p00=B", "X1_p00=B X2_p00=B"},
		}},
		{"every X1->X2(p00=X1.p00)", []string{"X1", "X2"}, func(s Stream[patternUseResultS0]) PatternStream {
			return PatternFrom(s, "X1", Literal(true)).Every().Then(PatternFrom(s, "X2",
				Equal[string](p00, TagField[string]("X1", "p00"))))
		}, map[string][]string{
			"e6": {"X1_p00=B X2_p00=B"}, "e8": {"X1_p00=C X2_p00=C"},
			"e10": {"X1_p00=G X2_p00=G"}, "e11": {"X1_p00=B X2_p00=B"},
			"e12": {"X1_p00=F X2_p00=F"},
		}},
		{"every X1->every X2(p00=X1.p00)", []string{"X1", "X2"}, func(s Stream[patternUseResultS0]) PatternStream {
			return PatternFrom(s, "X1", Literal(true)).Every().Then(PatternFrom(s, "X2",
				Equal[string](p00, TagField[string]("X1", "p00"))).Every())
		}, map[string][]string{
			"e6": {"X1_p00=B X2_p00=B"}, "e8": {"X1_p00=C X2_p00=C"},
			"e10": {"X1_p00=G X2_p00=G"}, "e11": {"X1_p00=B X2_p00=B", "X1_p00=B X2_p00=B"},
			"e12": {"X1_p00=F X2_p00=F"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runPatternS0Case(t, tc)
		})
	}
}

// TestPatternBooleanExprRemoveConsiderTagMatchesEsper mirrors
// PatternBooleanExprRemoveConsiderTag: every sb=SupportBean -> SupportBean_A(id
// like sb.theString) fires with sb.intPrimitive for each A-event whose id
// matches a prior sb tag's theString.
func TestPatternBooleanExprRemoveConsiderTagMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	if _, err := RegisterStruct[patternNotA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	sbStream := From[patternOpBean](env, "SupportBean")
	aStream := From[patternNotA](env, "SupportBean_A")
	pattern := PatternFrom(sbStream, "sb", Literal(true)).Every().
		Then(PatternFrom(aStream, "_", LikeOf(
			Field[patternNotA, string]("id"),
			TagField[string]("sb", "theString"))))
	plan, err := env.Build(pattern.Select(
		Alias("c0", TagField[int]("sb", "intPrimitive")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var fired []int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatal("expected row")
			}
			fired = append(fired, row.Get("c0").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Send E0..E9 as SupportBean events
	for i := 0; i < 10; i++ {
		sendPatternOpBean(t, engine, fmt.Sprintf("E%d", i), i)
	}

	// Send A-events and verify c0 values
	sendAAssert := func(id string, wantC0 int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), patternNotA{ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(fired) == 0 || fired[len(fired)-1] != wantC0 {
			t.Fatalf("send A(%s): fired = %v, want last = %d", id, fired, wantC0)
		}
	}
	sendAAssert("E5", 5)
	sendAAssert("E3", 3)
	sendAAssert("E1", 1)
	sendAAssert("E8", 8)
	sendAAssert("E4", 4)
	sendAAssert("E2", 2)
	sendAAssert("E9", 9)
	sendAAssert("E7", 7)
	sendAAssert("E0", 0)
	sendAAssert("E6", 6)

	// After all tags consumed, sending A-events should not fire
	for i := 0; i < 10; i++ {
		if err := engine.SendEvent(context.Background(), patternNotA{ID: fmt.Sprintf("E%d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(fired) != 10 {
		t.Fatalf("post-consume fired = %d, want 10", len(fired))
	}
}

// TestPatternBooleanExprRemoveConsiderArrayTagMatchesEsper mirrors
// PatternBooleanExprRemoveConsiderArrayTag: every [2] sb=SupportBean ->
// SupportBean_A(id like sb[1].theString) fires with sb[1].intPrimitive.
func TestPatternBooleanExprRemoveConsiderArrayTagMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	if _, err := RegisterStruct[patternNotA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	sbStream := From[patternOpBean](env, "SupportBean")
	aStream := From[patternNotA](env, "SupportBean_A")
	pattern := PatternFrom(sbStream, "sb", Literal(true)).MatchUntil(2, 2).Every().
		Then(PatternFrom(aStream, "_", LikeOf(
			Field[patternNotA, string]("id"),
			TagFieldAt[string]("sb", 1, "theString"))))
	plan, err := env.Build(pattern.Select(
		Alias("c0", TagFieldAt[int]("sb", 1, "intPrimitive")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var fired []int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatal("expected row")
			}
			fired = append(fired, row.Get("c0").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Send 6 pairs: X0,Y0, X1,Y1, ... X5,Y5
	for i := 0; i < 6; i++ {
		sendPatternOpBean(t, engine, fmt.Sprintf("X%d", i), i)
		sendPatternOpBean(t, engine, fmt.Sprintf("Y%d", i), i)
	}

	// Java expects: sendBeanAAssert("Y2", 2, 5) then sendBeanAMiss("Y2")
	sendAAssert := func(id string, wantC0 int) {
		t.Helper()
		before := len(fired)
		if err := engine.SendEvent(context.Background(), patternNotA{ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(fired) != before+1 || fired[before] != wantC0 {
			t.Fatalf("send A(%s): fired = %v, want one new = %d", id, fired, wantC0)
		}
	}
	sendAMiss := func(id string) {
		t.Helper()
		before := len(fired)
		if err := engine.SendEvent(context.Background(), patternNotA{ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(fired) != before {
			t.Fatalf("send A(%s): fired = %v, expected no new fire", id, fired)
		}
	}

	sendAAssert("Y2", 2)
	sendAMiss("Y2")
	sendAAssert("Y1", 1)
	sendAMiss("Y1")
	sendAMiss("Y2")
	sendAAssert("Y4", 4)
	sendAMiss("Y1")
	sendAMiss("Y2")
	sendAMiss("Y4")
	sendAAssert("Y0", 0)
	sendAAssert("Y5", 5)
	sendAAssert("Y3", 3)
}

// patternUseResultTrade mirrors SupportTradeEvent for PatternFollowedByFilter.
type patternUseResultTrade struct {
	ID        int    `esper:"id"`
	UserID    string `esper:"userId"`
	CcyPair   string `esper:"ccypair"`
	Direction string `esper:"direction"`
}

// TestPatternFollowedByFilterMatchesEsper mirrors PatternFollowedByFilter:
// every t1 -> (t2 -> t3) where timer:within(600 sec) with correlated
// userId/ccypair/direction filters must never produce a match that reuses a
// userId across the three tagged events. Java drives 100 events with an
// unseeded Random and asserts only the invariant; the Go replay uses a fixed
// seed for determinism and asserts the same invariant plus that the pattern
// did fire.
func TestPatternFollowedByFilterMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternUseResultTrade](env, "SupportTradeEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	stream := From[patternUseResultTrade](env, "SupportTradeEvent")
	userF := Field[patternUseResultTrade, string]("userId")
	ccyF := Field[patternUseResultTrade, string]("ccypair")
	dirF := Field[patternUseResultTrade, string]("direction")
	inUsers := func() Expression[bool] {
		return InOf(userF, Literal("U1000"), Literal("U1001"), Literal("U1002"))
	}
	sameCcy := Equal[string](ccyF, TagField[string]("tradeevent1", "ccypair"))
	sameDir := Equal[string](dirF, TagField[string]("tradeevent1", "direction"))
	neT1 := NotEqual[string](userF, TagField[string]("tradeevent1", "userId"))
	neT2 := NotEqual[string](userF, TagField[string]("tradeevent2", "userId"))
	filter2 := And(inUsers(), And(neT1, And(sameCcy, sameDir)))
	filter3 := And(inUsers(), And(neT1, And(neT2, And(sameCcy, sameDir))))
	inner := PatternFrom(stream, "tradeevent2", filter2).
		Then(PatternFrom(stream, "tradeevent3", filter3)).
		Within(600 * time.Second)
	pattern := PatternFrom(stream, "tradeevent1", inUsers()).Every().Then(inner)
	plan, err := env.Build(pattern.Select(
		Alias("u1", TagField[string]("tradeevent1", "userId")),
		Alias("u2", TagField[string]("tradeevent2", "userId")),
		Alias("u3", TagField[string]("tradeevent3", "userId")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	badMatches := 0
	fires := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("no row")
			}
			u1 := row.Get("u1").Any()
			u2 := row.Get("u2").Any()
			u3 := row.Get("u3").Any()
			fires++
			if u1 == u2 || u1 == u3 || u2 == u3 {
				badMatches++
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rng := rand.New(rand.NewSource(20260811))
	users := []string{"U1000", "U1001", "U1002"}
	ccy := []string{"USD", "JPY", "EUR"}
	direction := []string{"B", "S"}
	for i := 0; i < 100; i++ {
		if err := engine.SendEvent(context.Background(), patternUseResultTrade{
			ID:        i,
			UserID:    users[rng.Intn(len(users))],
			CcyPair:   ccy[rng.Intn(len(ccy))],
			Direction: direction[rng.Intn(len(direction))],
		}); err != nil {
			t.Fatal(err)
		}
	}
	if badMatches != 0 {
		t.Fatalf("badMatchCount = %d, want 0", badMatches)
	}
	if fires == 0 {
		t.Fatal("pattern never fired over 100 seeded events")
	}
}

// TestPatternTypeCacheForRepeatMatchesEsper mirrors
// PatternPatternTypeCacheForRepeat (UEJ-229-28464): two statements in one
// deployment repeat-match dissimilar object-array types — [2] a=TypeOne
// selects a[0].symbol while [2] a=TypeTwo selects a[0].market — verifying tag
// array property access does not confuse the two type shapes.
func TestPatternTypeCacheForRepeatMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterObjectArray(env, "TypeOne", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterObjectArray(env, "TypeTwo", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("market", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	planOne, err := env.Build(PatternFromRecord(FromAny(env, "TypeOne"), "a", Literal(true)).MatchUntil(2, 2).
		Select(Alias("c0", TagFieldAt[string]("a", 0, "symbol"))).Query(StatementName("Out2")))
	if err != nil {
		t.Fatal(err)
	}
	planTwo, err := env.Build(PatternFromRecord(FromAny(env, "TypeTwo"), "a", Literal(true)).MatchUntil(2, 2).
		Select(Alias("c0", TagFieldAt[string]("a", 0, "market"))).Query(StatementName("Out3")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.DeployPlans(context.Background(), []Plan{planOne, planTwo})
	if err != nil {
		t.Fatal(err)
	}
	fired := map[string][]string{}
	for _, statement := range deployment.Statements() {
		name := statement.Name()
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					return fmt.Errorf("no row")
				}
				fired[name] = append(fired[name], fmt.Sprintf("%v", row.Get("c0").Any()))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := engine.SendObjectArray(context.Background(), "TypeOne", []any{"GE", 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendObjectArray(context.Background(), "TypeOne", []any{"GE", 10}); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%v", fired["Out2"]) != "[GE]" {
		t.Fatalf("Out2 fires = %v, want [GE]", fired["Out2"])
	}
	if len(fired["Out3"]) != 0 {
		t.Fatalf("Out3 fired early: %v", fired["Out3"])
	}

	if err := engine.SendObjectArray(context.Background(), "TypeTwo", []any{"GE", "m1", 5}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendObjectArray(context.Background(), "TypeTwo", []any{"GE", "m2", 5}); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%v", fired["Out3"]) != "[m1]" {
		t.Fatalf("Out3 fires = %v, want [m1]", fired["Out3"])
	}
	if fmt.Sprintf("%v", fired["Out2"]) != "[GE]" {
		t.Fatalf("Out2 fires after TypeTwo = %v, want [GE]", fired["Out2"])
	}
}
