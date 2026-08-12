package esper

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"
)

// Parity coverage for PatternOperatorMatchUntil (see docs
// esper-go-port-implementation-plan.md). Expectations mirror the Java suite:
// the PatternOp W-harness cases replay the EventCollectionFactory mixed set
// (A1@1000, B1@2000, C1@3000, B2@4000, A2@5000, D1@6000, E1@7000, F1@8000,
// D2@9000, B3@10000, G1@11000, D3@12000) advancing the virtual clock to each
// event's timestamp before sending it, so timer occurrences that fire during
// an advance are attributed to the upcoming event exactly like the Java
// harness attributes them in checkResults after sendEventBean.

// patternMuStreams bundles the typed streams of the mixed event set.
type patternMuStreams struct {
	a Stream[patternNotA]
	b Stream[patternNotB]
	c Stream[patternNotC]
	d Stream[patternNotD]
	e Stream[patternNotE]
	f Stream[patternNotF]
	g Stream[patternNotG]
}

func newPatternMuEnv(t *testing.T) (*Environment, *Engine, patternMuStreams) {
	t.Helper()
	env := NewEnvironment()
	registrations := []struct {
		name  string
		event any
	}{
		{"SupportBean_A", patternNotA{}},
		{"SupportBean_B", patternNotB{}},
		{"SupportBean_C", patternNotC{}},
		{"SupportBean_D", patternNotD{}},
		{"SupportBean_E", patternNotE{}},
		{"SupportBean_F", patternNotF{}},
		{"SupportBean_G", patternNotG{}},
	}
	for _, registration := range registrations {
		switch registration.name {
		case "SupportBean_A":
			if _, err := RegisterStruct[patternNotA](env, registration.name); err != nil {
				t.Fatal(err)
			}
		case "SupportBean_B":
			if _, err := RegisterStruct[patternNotB](env, registration.name); err != nil {
				t.Fatal(err)
			}
		case "SupportBean_C":
			if _, err := RegisterStruct[patternNotC](env, registration.name); err != nil {
				t.Fatal(err)
			}
		case "SupportBean_D":
			if _, err := RegisterStruct[patternNotD](env, registration.name); err != nil {
				t.Fatal(err)
			}
		case "SupportBean_E":
			if _, err := RegisterStruct[patternNotE](env, registration.name); err != nil {
				t.Fatal(err)
			}
		case "SupportBean_F":
			if _, err := RegisterStruct[patternNotF](env, registration.name); err != nil {
				t.Fatal(err)
			}
		case "SupportBean_G":
			if _, err := RegisterStruct[patternNotG](env, registration.name); err != nil {
				t.Fatal(err)
			}
		}
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	return env, engine, patternMuStreams{
		a: From[patternNotA](env, "SupportBean_A"),
		b: From[patternNotB](env, "SupportBean_B"),
		c: From[patternNotC](env, "SupportBean_C"),
		d: From[patternNotD](env, "SupportBean_D"),
		e: From[patternNotE](env, "SupportBean_E"),
		f: From[patternNotF](env, "SupportBean_F"),
		g: From[patternNotG](env, "SupportBean_G"),
	}
}

// patternMuEvent is one timed event of the Java mixed set.
type patternMuEvent struct {
	id    string
	ms    int64
	event any
}

func patternMuEventSet() []patternMuEvent {
	return []patternMuEvent{
		{"A1", 1000, patternNotA{ID: "A1"}}, {"B1", 2000, patternNotB{ID: "B1"}},
		{"C1", 3000, patternNotC{ID: "C1"}}, {"B2", 4000, patternNotB{ID: "B2"}},
		{"A2", 5000, patternNotA{ID: "A2"}}, {"D1", 6000, patternNotD{ID: "D1"}},
		{"E1", 7000, patternNotE{ID: "E1"}}, {"F1", 8000, patternNotF{ID: "F1"}},
		{"D2", 9000, patternNotD{ID: "D2"}}, {"B3", 10000, patternNotB{ID: "B3"}},
		{"G1", 11000, patternNotG{ID: "G1"}}, {"D3", 12000, patternNotD{ID: "D3"}},
	}
}

// patternMuCase describes one W-harness expression: array tags are projected
// as tag0..tag3 (TagFieldAt, out-of-range evaluates to Null and is absent
// from the flattened form), single tags as TagField. Fires within one event
// compare as a multiset, exactly like the Java harness compareLists.
type patternMuCase struct {
	name       string
	arrayTags  []string
	singleTags []string
	build      func(s patternMuStreams) PatternStream
	want       map[string][]string
}

func runPatternMuCase(t *testing.T, testCase patternMuCase) {
	t.Helper()
	env, engine, streams := newPatternMuEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	selections := make([]Selection, 0, len(testCase.arrayTags)*4+len(testCase.singleTags))
	for _, tag := range testCase.arrayTags {
		for index := 0; index < 4; index++ {
			selections = append(selections, Alias(fmt.Sprintf("%s%d", tag, index), TagFieldAt[string](tag, index, "id")))
		}
	}
	for _, tag := range testCase.singleTags {
		selections = append(selections, Alias(tag, TagField[string](tag, "id")))
	}
	plan, err := env.Build(testCase.build(streams).Select(selections...).Query(StatementName("s0")))
	if err != nil {
		t.Fatalf("%s: build: %v", testCase.name, err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("%s: deploy: %v", testCase.name, err)
	}
	firesByEvent := map[string][]string{}
	currentEvent := ""
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("pattern result is not a row: %#v", result)
			}
			parts := []string{}
			for _, selection := range selections {
				if value := row.Get(selection.Name); value.State() == ValuePresent {
					parts = append(parts, selection.Name+"="+value.Any().(string))
				}
			}
			sort.Strings(parts)
			firesByEvent[currentEvent] = append(firesByEvent[currentEvent], joinStrings(parts, " "))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, step := range patternMuEventSet() {
		currentEvent = step.id
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(step.ms).UTC()); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), step.event); err != nil {
			t.Fatal(err)
		}
	}
	assertPatternAndOrFires(t, testCase.name, testCase.want, firesByEvent)
}

// TestPatternMatchUntilOpBasicMatchesEsper covers the unbounded until W-harness
// cases: a repeated tag collects matches into an array until the terminator
// fires, and the until branch started first so a shared event always resolves
// in the terminator's favor.
func TestPatternMatchUntilOpBasicMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	idA := func(id string) Expression[bool] {
		return Equal[string](Field[patternNotA, string]("id"), Literal(id))
	}
	idD := func(id string) Expression[bool] {
		return Equal[string](Field[patternNotD, string]("id"), Literal(id))
	}
	idG := func(id string) Expression[bool] {
		return Equal[string](Field[patternNotG, string]("id"), Literal(id))
	}
	cases := []patternMuCase{
		{
			name:       "a=A(id=A2) until D",
			arrayTags:  []string{"a"},
			singleTags: []string{"d"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.a, "a", idA("A2")).Until(PatternFrom(s.d, "d", trueExpr))
			},
			want: map[string][]string{"D1": {"a0=A2 d=D1"}},
		},
		{
			name:       "a=A until D",
			arrayTags:  []string{"a"},
			singleTags: []string{"d"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.a, "a", trueExpr).Until(PatternFrom(s.d, "d", trueExpr))
			},
			want: map[string][]string{"D1": {"a0=A1 a1=A2 d=D1"}},
		},
		{
			name:       "b=B until a=A",
			arrayTags:  []string{"b"},
			singleTags: []string{"a"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).Until(PatternFrom(s.a, "a", trueExpr))
			},
			want: map[string][]string{"A1": {"a=A1"}},
		},
		{
			name:       "b=B until D(id=D3)",
			arrayTags:  []string{"b"},
			singleTags: []string{"d"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).Until(PatternFrom(s.d, "d", idD("D3")))
			},
			want: map[string][]string{"D3": {"b0=B1 b1=B2 b2=B3 d=D3"}},
		},
		{
			name:       "(a or b) until d=D(id=D3)",
			arrayTags:  []string{"a", "b"},
			singleTags: []string{"d"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.a, "a", trueExpr).Or(PatternFrom(s.b, "b", trueExpr)).Until(PatternFrom(s.d, "d", idD("D3")))
			},
			want: map[string][]string{"D3": {"a0=A1 a1=A2 b0=B1 b1=B2 b2=B3 d=D3"}},
		},
		{
			name:       "(a or b) until (g or d)",
			arrayTags:  []string{"a", "b"},
			singleTags: []string{"d", "g"},
			build: func(s patternMuStreams) PatternStream {
				terminator := PatternFrom(s.g, "g", trueExpr).Or(PatternFrom(s.d, "d", trueExpr))
				return PatternFrom(s.a, "a", trueExpr).Or(PatternFrom(s.b, "b", trueExpr)).Until(terminator)
			},
			want: map[string][]string{"D1": {"a0=A1 a1=A2 b0=B1 b1=B2 d=D1"}},
		},
		{
			name:       "(d=D) until a=A(id=A1)",
			arrayTags:  []string{"d"},
			singleTags: []string{"a"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.d, "d", trueExpr).Until(PatternFrom(s.a, "a", idA("A1")))
			},
			want: map[string][]string{"A1": {"a=A1"}},
		},
		{
			name:       "a=A until G(id=GX)",
			arrayTags:  []string{"a"},
			singleTags: []string{"g"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.a, "a", trueExpr).Until(PatternFrom(s.g, "g", idG("GX")))
			},
			want: map[string][]string{},
		},
		{
			name:      "B until not B",
			arrayTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).Until(PatternFrom(s.b, "nb", trueExpr).Not())
			},
			want: map[string][]string{},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) { runPatternMuCase(t, testCase) })
	}
}

// TestPatternMatchUntilOpBoundsMatchesEsper covers the bounded-repeat
// W-harness cases: exact [N] repeats fire on the Nth match, a tightly-bound
// [N:N] with an until fires on the bound without waiting for the terminator,
// looser ranges only fire through the terminator, and the until branch wins
// when one event matches both sides.
func TestPatternMatchUntilOpBoundsMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	idA := func(id string) Expression[bool] {
		return Equal[string](Field[patternNotA, string]("id"), Literal(id))
	}
	idB := func(id string) Expression[bool] {
		return Equal[string](Field[patternNotB, string]("id"), Literal(id))
	}
	idG := func(id string) Expression[bool] {
		return Equal[string](Field[patternNotG, string]("id"), Literal(id))
	}
	bUntilG1 := func(s patternMuStreams, minimum, maximum int) PatternStream {
		return PatternFrom(s.b, "b", trueExpr).MatchUntil(minimum, maximum).Until(PatternFrom(s.g, "g", idG("G1")))
	}
	cases := []patternMuCase{
		{
			name:      "[2] a",
			arrayTags: []string{"a"},
			build:     func(s patternMuStreams) PatternStream { return PatternFrom(s.a, "a", trueExpr).MatchUntil(2, 2) },
			want:      map[string][]string{"A2": {"a0=A1 a1=A2"}},
		},
		{
			name:      "[1] a",
			arrayTags: []string{"a"},
			build:     func(s patternMuStreams) PatternStream { return PatternFrom(s.a, "a", trueExpr).MatchUntil(1, 1) },
			want:      map[string][]string{"A1": {"a0=A1"}},
		},
		{
			name:      "[3] a",
			arrayTags: []string{"a"},
			build:     func(s patternMuStreams) PatternStream { return PatternFrom(s.a, "a", trueExpr).MatchUntil(3, 3) },
			want:      map[string][]string{},
		},
		{
			name:      "[3] b",
			arrayTags: []string{"b"},
			build:     func(s patternMuStreams) PatternStream { return PatternFrom(s.b, "b", trueExpr).MatchUntil(3, 3) },
			want:      map[string][]string{"B3": {"b0=B1 b1=B2 b2=B3"}},
		},
		{
			name:      "[4] (a or b)",
			arrayTags: []string{"a", "b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.a, "a", trueExpr).Or(PatternFrom(s.b, "b", trueExpr)).MatchUntil(4, 4)
			},
			want: map[string][]string{"A2": {"a0=A1 a1=A2 b0=B1 b1=B2"}},
		},
		{
			name:      "[2] (a or b)",
			arrayTags: []string{"a", "b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.a, "a", trueExpr).Or(PatternFrom(s.b, "b", trueExpr)).MatchUntil(2, 2)
			},
			want: map[string][]string{"B1": {"a0=A1 b0=B1"}},
		},
		{
			name:      "[3] (a or b)",
			arrayTags: []string{"a", "b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.a, "a", trueExpr).Or(PatternFrom(s.b, "b", trueExpr)).MatchUntil(3, 3)
			},
			want: map[string][]string{"B2": {"a0=A1 b0=B1 b1=B2"}},
		},
		{
			name:      "[2] b until a=A(id=A1)",
			arrayTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(2, 2).Until(PatternFrom(s.a, "a", idA("A1")))
			},
			want: map[string][]string{},
		},
		{
			name:      "[2] b until c=C",
			arrayTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(2, 2).Until(PatternFrom(s.c, "c", trueExpr))
			},
			want: map[string][]string{},
		},
		{
			name:       "[2:2] b until g=G(id=G1)",
			arrayTags:  []string{"b"},
			singleTags: []string{"g"},
			build:      func(s patternMuStreams) PatternStream { return bUntilG1(s, 2, 2) },
			want:       map[string][]string{"B2": {"b0=B1 b1=B2"}},
		},
		{
			name:       "[:4] b until g=G(id=G1)",
			arrayTags:  []string{"b"},
			singleTags: []string{"g"},
			build:      func(s patternMuStreams) PatternStream { return bUntilG1(s, 0, 4) },
			want:       map[string][]string{"G1": {"b0=B1 b1=B2 b2=B3 g=G1"}},
		},
		{
			name:       "[:3] b until g=G(id=G1)",
			arrayTags:  []string{"b"},
			singleTags: []string{"g"},
			build:      func(s patternMuStreams) PatternStream { return bUntilG1(s, 0, 3) },
			want:       map[string][]string{"G1": {"b0=B1 b1=B2 b2=B3 g=G1"}},
		},
		{
			name:       "[:2] b until g=G(id=G1)",
			arrayTags:  []string{"b"},
			singleTags: []string{"g"},
			build:      func(s patternMuStreams) PatternStream { return bUntilG1(s, 0, 2) },
			want:       map[string][]string{"G1": {"b0=B1 b1=B2 g=G1"}},
		},
		{
			name:       "[:1] b until g=G(id=G1)",
			arrayTags:  []string{"b"},
			singleTags: []string{"g"},
			build:      func(s patternMuStreams) PatternStream { return bUntilG1(s, 0, 1) },
			want:       map[string][]string{"G1": {"b0=B1 g=G1"}},
		},
		{
			name:       "[:1] b until a=A(id=A1)",
			arrayTags:  []string{"b"},
			singleTags: []string{"a"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(0, 1).Until(PatternFrom(s.a, "a", idA("A1")))
			},
			want: map[string][]string{"A1": {"a=A1"}},
		},
		{
			name:       "[1:] b until g=G(id=G1)",
			arrayTags:  []string{"b"},
			singleTags: []string{"g"},
			build:      func(s patternMuStreams) PatternStream { return bUntilG1(s, 1, 0) },
			want:       map[string][]string{"G1": {"b0=B1 b1=B2 b2=B3 g=G1"}},
		},
		{
			name:      "[1:] b until a=A",
			arrayTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(1, 0).Until(PatternFrom(s.a, "a", trueExpr))
			},
			want: map[string][]string{},
		},
		{
			name:       "[2:] b until a=A(id=A2)",
			arrayTags:  []string{"b"},
			singleTags: []string{"a"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(2, 0).Until(PatternFrom(s.a, "a", idA("A2")))
			},
			want: map[string][]string{"A2": {"a=A2 b0=B1 b1=B2"}},
		},
		{
			name:      "[2:] b until c=C",
			arrayTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(2, 0).Until(PatternFrom(s.c, "c", trueExpr))
			},
			want: map[string][]string{},
		},
		{
			name:      "[2:] b until e=B(id=B2)",
			arrayTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(2, 0).Until(PatternFrom(s.e, "e", Equal[string](Field[patternNotB, string]("id"), Literal("B2"))))
			},
			want: map[string][]string{},
		},
		{
			name:      "[1:] b until e=B(id=B1)",
			arrayTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(1, 0).Until(PatternFrom(s.e, "e", idB("B1")))
			},
			want: map[string][]string{},
		},
		{
			name:       "[1:2] b until a=A(id=A2)",
			arrayTags:  []string{"b"},
			singleTags: []string{"a"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(1, 2).Until(PatternFrom(s.a, "a", idA("A2")))
			},
			want: map[string][]string{"A2": {"a=A2 b0=B1 b1=B2"}},
		},
		{
			name:       "[1:3] b until G",
			arrayTags:  []string{"b"},
			singleTags: []string{"g"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(1, 3).Until(PatternFrom(s.g, "g", trueExpr))
			},
			want: map[string][]string{"G1": {"b0=B1 b1=B2 b2=B3 g=G1"}},
		},
		{
			name:       "[1:2] b until G",
			arrayTags:  []string{"b"},
			singleTags: []string{"g"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(1, 2).Until(PatternFrom(s.g, "g", trueExpr))
			},
			want: map[string][]string{"G1": {"b0=B1 b1=B2 g=G1"}},
		},
		{
			name:       "[1:10] b until F",
			arrayTags:  []string{"b"},
			singleTags: []string{"f"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(1, 10).Until(PatternFrom(s.f, "f", trueExpr))
			},
			want: map[string][]string{"F1": {"b0=B1 b1=B2 f=F1"}},
		},
		{
			name:       "[1:10] b until C",
			arrayTags:  []string{"b"},
			singleTags: []string{"c"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(1, 10).Until(PatternFrom(s.c, "c", trueExpr))
			},
			want: map[string][]string{"C1": {"b0=B1 c=C1"}},
		},
		{
			name:       "[0:1] b until C",
			arrayTags:  []string{"b"},
			singleTags: []string{"c"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.b, "b", trueExpr).MatchUntil(0, 1).Until(PatternFrom(s.c, "c", trueExpr))
			},
			want: map[string][]string{"C1": {"b0=B1 c=C1"}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) { runPatternMuCase(t, testCase) })
	}
}

// TestPatternMatchUntilOpCompositionMatchesEsper covers the composition
// W-harness cases: match-until inside sequences, and/or combinations, timer
// terminators, every precedence (ESPER-339: every binds tighter than until),
// and nested until expressions collecting inner tag arrays.
func TestPatternMatchUntilOpCompositionMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	cases := []patternMuCase{
		{
			name:       "c -> [2] b -> d",
			arrayTags:  []string{"b"},
			singleTags: []string{"c", "d"},
			build: func(s patternMuStreams) PatternStream {
				middle := PatternFrom(s.b, "b", trueExpr).MatchUntil(2, 2)
				return PatternFrom(s.c, "c", trueExpr).Then(middle).Then(PatternFrom(s.d, "d", trueExpr))
			},
			want: map[string][]string{"D3": {"b0=B2 b1=B3 c=C1 d=D3"}},
		},
		{
			name:      "[3] d or [3] b",
			arrayTags: []string{"d", "b"},
			build: func(s patternMuStreams) PatternStream {
				left := PatternFrom(s.d, "d", trueExpr).MatchUntil(3, 3)
				right := PatternFrom(s.b, "b", trueExpr).MatchUntil(3, 3)
				return left.Or(right)
			},
			want: map[string][]string{"B3": {"b0=B1 b1=B2 b2=B3"}},
		},
		{
			name:      "[3] d or [4] b",
			arrayTags: []string{"d", "b"},
			build: func(s patternMuStreams) PatternStream {
				left := PatternFrom(s.d, "d", trueExpr).MatchUntil(3, 3)
				right := PatternFrom(s.b, "b", trueExpr).MatchUntil(4, 4)
				return left.Or(right)
			},
			want: map[string][]string{"D3": {"d0=D1 d1=D2 d2=D3"}},
		},
		{
			name:      "[2] d and [2] b",
			arrayTags: []string{"d", "b"},
			build: func(s patternMuStreams) PatternStream {
				left := PatternFrom(s.d, "d", trueExpr).MatchUntil(2, 2)
				right := PatternFrom(s.b, "b", trueExpr).MatchUntil(2, 2)
				return left.And(right)
			},
			want: map[string][]string{"D2": {"b0=B1 b1=B2 d0=D1 d1=D2"}},
		},
		{
			name:      "d until timer:interval(7 sec)",
			arrayTags: []string{"d"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.d, "d", trueExpr).Until(TimerInterval(s.d, 7*time.Second))
			},
			want: map[string][]string{"E1": {"d0=D1"}},
		},
		{
			name:       "every (d until b)",
			arrayTags:  []string{"d"},
			singleTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.d, "d", trueExpr).Until(PatternFrom(s.b, "b", trueExpr)).Every()
			},
			want: map[string][]string{
				"B1": {"b=B1"},
				"B2": {"b=B2"},
				"B3": {"b=B3 d0=D1 d1=D2"},
			},
		},
		{
			name:       "every d until b",
			arrayTags:  []string{"d"},
			singleTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.d, "d", trueExpr).Every().Until(PatternFrom(s.b, "b", trueExpr))
			},
			want: map[string][]string{"B1": {"b=B1"}},
		},
		{
			name:       "(every d) until b",
			arrayTags:  []string{"d"},
			singleTags: []string{"b"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.d, "d", trueExpr).Every().Until(PatternFrom(s.b, "b", trueExpr))
			},
			want: map[string][]string{"B1": {"b=B1"}},
		},
		{
			name:      "a until every(timer:interval(6 sec) and not A)",
			arrayTags: []string{"a"},
			build: func(s patternMuStreams) PatternStream {
				terminator := TimerInterval(s.a, 6*time.Second).And(PatternFrom(s.a, "na", trueExpr).Not()).Every()
				return PatternFrom(s.a, "a", trueExpr).Until(terminator)
			},
			want: map[string][]string{"G1": {"a0=A1 a1=A2"}},
		},
		{
			name:      "A until every(timer:interval(7 sec) and not A)",
			arrayTags: []string{"u"},
			build: func(s patternMuStreams) PatternStream {
				terminator := TimerInterval(s.a, 7*time.Second).And(PatternFrom(s.a, "na", trueExpr).Not()).Every()
				return PatternFrom(s.a, "u", trueExpr).Until(terminator)
			},
			want: map[string][]string{"D3": {"u0=A1 u1=A2"}},
		},
		{
			name:       "every [2] a",
			arrayTags:  []string{"a"},
			singleTags: []string{},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.a, "a", trueExpr).MatchUntil(2, 2).Every()
			},
			want: map[string][]string{"A2": {"a0=A1 a1=A2"}},
		},
		{
			name:       "every [2] a until d",
			arrayTags:  []string{"a"},
			singleTags: []string{"d"},
			build: func(s patternMuStreams) PatternStream {
				return PatternFrom(s.a, "a", trueExpr).MatchUntil(2, 2).Every().Until(PatternFrom(s.d, "d", trueExpr))
			},
			want: map[string][]string{"D1": {"a0=A1 a1=A2 d=D1"}},
		},
		{
			name:       "(a until b) until c",
			arrayTags:  []string{"a", "b"},
			singleTags: []string{"c"},
			build: func(s patternMuStreams) PatternStream {
				inner := PatternFrom(s.a, "a", trueExpr).Until(PatternFrom(s.b, "b", trueExpr))
				return inner.Until(PatternFrom(s.c, "c", trueExpr))
			},
			want: map[string][]string{"C1": {"a0=A1 b0=B1 c=C1"}},
		},
		{
			name:       "(a until b) until g",
			arrayTags:  []string{"a", "b"},
			singleTags: []string{"g"},
			build: func(s patternMuStreams) PatternStream {
				inner := PatternFrom(s.a, "a", trueExpr).Until(PatternFrom(s.b, "b", trueExpr))
				return inner.Until(PatternFrom(s.g, "g", trueExpr))
			},
			want: map[string][]string{"G1": {"a0=A1 a1=A2 b0=B1 b1=B2 b2=B3 g=G1"}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) { runPatternMuCase(t, testCase) })
	}
}

// TestPatternMatchUntilSimpleMatchesEsper mirrors PatternMatchUntilSimple:
// a=X(intPrimitive=0) until b=X(intPrimitive=1) collects the repeated tag
// into an array, fires once when the terminator arrives, and is permanently
// false afterwards (later A/B pairs do not fire again).
func TestPatternMatchUntilSimpleMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	source := From[patternOpBean](env, "SupportBean")
	patternA := PatternFrom(source, "a", Equal[int](Field[patternOpBean, int]("intPrimitive"), Literal(0)))
	patternB := PatternFrom(source, "b", Equal[int](Field[patternOpBean, int]("intPrimitive"), Literal(1)))
	plan, err := env.Build(patternA.Until(patternB).Select(
		Alias("c0", TagFieldAt[string]("a", 0, "theString")),
		Alias("c1", TagFieldAt[string]("a", 1, "theString")),
		Alias("c2", TagField[string]("b", "theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := [][]any{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("pattern result is not a row: %#v", result)
			}
			rows = append(rows, []any{rowAnyValue(row, "c0"), rowAnyValue(row, "c1"), rowAnyValue(row, "c2")})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendPatternOpBean(t, engine, "A1", 0)
	sendPatternOpBean(t, engine, "A2", 0)
	if len(rows) != 0 {
		t.Fatalf("no terminator yet: rows = %v", rows)
	}
	sendPatternOpBean(t, engine, "B1", 1)
	if len(rows) != 1 || fmt.Sprintf("%v", rows[0]) != "[A1 A2 B1]" {
		t.Fatalf("after B1: rows = %v, want [[A1 A2 B1]]", rows)
	}
	sendPatternOpBean(t, engine, "A1", 0)
	sendPatternOpBean(t, engine, "B1", 1)
	if len(rows) != 1 {
		t.Fatalf("fired pattern must stay permanently false: rows = %v", rows)
	}
}

func rowAnyValue(row Row, name string) any {
	value := row.Get(name)
	if value.State() != ValuePresent {
		return nil
	}
	return value.Any()
}

// TestPatternMatchUntilSelectArrayMatchesEsper mirrors PatternSelectArray:
// repeated-tag array element access in the select clause, including an
// out-of-range index evaluating to Null, plus the wildcard-form projection
// that the fluent API expresses with explicit TagEvents/TagFieldAt
// selections (Java materializes fragment EventBeans for select *).
func TestPatternMatchUntilSelectArrayMatchesEsper(t *testing.T) {
	env, engine, streams := newPatternMuEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	pattern := PatternFrom(streams.a, "a", Literal[bool](true)).Until(PatternFrom(streams.b, "b", Literal[bool](true)))
	plan, err := env.Build(pattern.Select(
		Alias("a", TagEvents("a")),
		Alias("a0Id", TagFieldAt[string]("a", 0, "id")),
		Alias("a1Id", TagFieldAt[string]("a", 1, "id")),
		Alias("a2Id", TagFieldAt[string]("a", 2, "id")),
		Alias("b", TagField[string]("b", "id")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := []Row{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("pattern result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternNotA{ID: "A1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternNotA{ID: "A2"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("no terminator yet: rows = %v", rows)
	}
	if err := engine.SendEvent(context.Background(), patternNotB{ID: "B1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("after B1: rows = %v", rows)
	}
	row := rows[0]
	if got := rowAnyValue(row, "a0Id"); got != "A1" {
		t.Fatalf("a0Id = %v, want A1", got)
	}
	if got := rowAnyValue(row, "a1Id"); got != "A2" {
		t.Fatalf("a1Id = %v, want A2", got)
	}
	if got := rowAnyValue(row, "a2Id"); got != nil {
		t.Fatalf("a2Id = %v, want nil", got)
	}
	if got := rowAnyValue(row, "b"); got != "B1" {
		t.Fatalf("b = %v, want B1", got)
	}
	events, ok := rowAnyValue(row, "a").([]Event)
	if !ok || len(events) != 2 {
		t.Fatalf("a = %v, want 2 captured events", rowAnyValue(row, "a"))
	}
	if got := events[0].Get("id"); got.State() != ValuePresent || got.Any() != "A1" {
		t.Fatalf("a[0].id = %v, want A1", got.Any())
	}
	if got := events[1].Get("id"); got.State() != ValuePresent || got.Any() != "A2" {
		t.Fatalf("a[1].id = %v, want A2", got.Any())
	}
}

// patternMuBeanEnv registers the letter types plus the SupportBean type used
// by the filter-correlation executions.
func newPatternMuBeanEnv(t *testing.T) (*Environment, *Engine, patternMuStreams, Stream[patternOpBean]) {
	t.Helper()
	env, engine, streams := newPatternMuEnv(t)
	if _, err := RegisterStruct[patternOpBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env, engine, streams, From[patternOpBean](env, "SupportBean")
}

// TestPatternMatchUntilUseFilterMatchesEsper mirrors PatternUseFilter:
// follow-on branches correlate against repeated-tag array elements
// (a[0].id/a[1].id/a[2].id) captured by the match-until, covering concat,
// equals, in, not-in and between predicate forms.
func TestPatternMatchUntilUseFilterMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	sendLetters := func(t *testing.T, engine *Engine, events ...any) {
		t.Helper()
		for _, event := range events {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
	}
	deploy := func(t *testing.T, env *Environment, engine *Engine, stream PatternStream, selections ...Selection) *[]Row {
		t.Helper()
		plan, err := env.Build(stream.Select(selections...).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := []Row{}
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					return fmt.Errorf("pattern result is not a row: %#v", result)
				}
				rows = append(rows, row)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return &rows
	}

	t.Run("concat-correlation", func(t *testing.T) {
		env, engine, streams, _ := newPatternMuBeanEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		repeated := PatternFrom(streams.a, "a", trueExpr).Until(PatternFrom(streams.b, "b", trueExpr))
		correlated := PatternFrom(streams.c, "c", Equal[string](
			Field[patternNotC, string]("id"),
			Concat(
				Literal("C"),
				TagFieldAt[string]("a", 0, "id"),
				TagFieldAt[string]("a", 1, "id"),
				TagField[string]("b", "id"),
			),
		))
		rowsPtr := deploy(t, env, engine, repeated.Then(correlated),
			Alias("a0", TagFieldAt[string]("a", 0, "id")),
			Alias("a1", TagFieldAt[string]("a", 1, "id")),
			Alias("a2", TagFieldAt[string]("a", 2, "id")),
			Alias("b", TagField[string]("b", "id")),
			Alias("c", TagField[string]("c", "id")),
		)
		sendLetters(t, engine, patternNotA{ID: "A1"}, patternNotA{ID: "A2"}, patternNotB{ID: "B1"}, patternNotC{ID: "C1"})
		if len(*rowsPtr) != 0 {
			t.Fatalf("C1 must not correlate: rows = %v", *rowsPtr)
		}
		sendLetters(t, engine, patternNotC{ID: "CA1A2B1"})
		if len(*rowsPtr) != 1 {
			t.Fatalf("after CA1A2B1: rows = %v", *rowsPtr)
		}
		row := (*rowsPtr)[0]
		if got := rowAnyValue(row, "c"); got != "CA1A2B1" {
			t.Fatalf("c = %v, want CA1A2B1", got)
		}
		if got := rowAnyValue(row, "a0"); got != "A1" {
			t.Fatalf("a0 = %v, want A1", got)
		}
		if got := rowAnyValue(row, "a1"); got != "A2" {
			t.Fatalf("a1 = %v, want A2", got)
		}
		if got := rowAnyValue(row, "a2"); got != nil {
			t.Fatalf("a2 = %v, want nil", got)
		}
		if got := rowAnyValue(row, "b"); got != "B1" {
			t.Fatalf("b = %v, want B1", got)
		}
	})

	t.Run("equals-correlation", func(t *testing.T) {
		env, engine, streams, bean := newPatternMuBeanEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		repeated := PatternFrom(streams.a, "a", trueExpr).Until(PatternFrom(streams.b, "b", trueExpr))
		correlated := PatternFrom(bean, "c", Equal[string](Field[patternOpBean, string]("theString"), TagFieldAt[string]("a", 1, "id")))
		rowsPtr := deploy(t, env, engine, repeated.Then(correlated), Alias("cInt", TagField[int]("c", "intPrimitive")))
		sendLetters(t, engine, patternNotA{ID: "A1"}, patternNotA{ID: "A2"}, patternNotB{ID: "B1"}, patternOpBean{TheString: "A3", IntPrimitive: 20})
		if len(*rowsPtr) != 0 {
			t.Fatalf("A3 must not correlate: rows = %v", *rowsPtr)
		}
		sendLetters(t, engine, patternOpBean{TheString: "A2", IntPrimitive: 10})
		if len(*rowsPtr) != 1 || rowAnyValue((*rowsPtr)[0], "cInt") != 10 {
			t.Fatalf("after A2/10: rows = %v", *rowsPtr)
		}
	})

	t.Run("in-correlation", func(t *testing.T) {
		env, engine, streams, bean := newPatternMuBeanEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		repeated := PatternFrom(streams.a, "a", trueExpr).Until(PatternFrom(streams.b, "b", trueExpr))
		correlated := PatternFrom(bean, "c", InOf(Field[patternOpBean, string]("theString"), TagFieldAt[string]("a", 2, "id")))
		rowsPtr := deploy(t, env, engine, repeated.Then(correlated), Alias("cInt", TagField[int]("c", "intPrimitive")))
		sendLetters(t, engine,
			patternNotA{ID: "A1"}, patternNotA{ID: "A2"}, patternNotA{ID: "A3"}, patternNotB{ID: "B1"},
			patternOpBean{TheString: "A2", IntPrimitive: 20},
		)
		if len(*rowsPtr) != 0 {
			t.Fatalf("A2 must not match a[2]=A3: rows = %v", *rowsPtr)
		}
		sendLetters(t, engine, patternOpBean{TheString: "A3", IntPrimitive: 5})
		if len(*rowsPtr) != 1 || rowAnyValue((*rowsPtr)[0], "cInt") != 5 {
			t.Fatalf("after A3/5: rows = %v", *rowsPtr)
		}
	})

	t.Run("not-in-correlation", func(t *testing.T) {
		env, engine, streams, bean := newPatternMuBeanEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		repeated := PatternFrom(streams.a, "a", trueExpr).Until(PatternFrom(streams.b, "b", trueExpr))
		correlated := PatternFrom(bean, "c", And(
			And(
				NotEqual[string](Field[patternOpBean, string]("theString"), TagFieldAt[string]("a", 0, "id")),
				NotEqual[string](Field[patternOpBean, string]("theString"), TagFieldAt[string]("a", 1, "id")),
			),
			NotEqual[string](Field[patternOpBean, string]("theString"), TagFieldAt[string]("a", 2, "id")),
		))
		rowsPtr := deploy(t, env, engine, repeated.Then(correlated), Alias("cInt", TagField[int]("c", "intPrimitive")))
		sendLetters(t, engine,
			patternNotA{ID: "A1"}, patternNotA{ID: "A2"}, patternNotA{ID: "A3"}, patternNotB{ID: "B1"},
			patternOpBean{TheString: "A2", IntPrimitive: 20},
			patternOpBean{TheString: "A1", IntPrimitive: 20},
		)
		if len(*rowsPtr) != 0 {
			t.Fatalf("a[0]/a[1] ids must not match: rows = %v", *rowsPtr)
		}
		sendLetters(t, engine, patternOpBean{TheString: "A6", IntPrimitive: 5})
		if len(*rowsPtr) != 1 || rowAnyValue((*rowsPtr)[0], "cInt") != 5 {
			t.Fatalf("after A6/5: rows = %v", *rowsPtr)
		}
	})

	t.Run("range-correlation", func(t *testing.T) {
		env, engine, _, bean := newPatternMuBeanEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		likeA := LikeOf(Field[patternOpBean, string]("theString"), Literal("A%"))
		likeB := LikeOf(Field[patternOpBean, string]("theString"), Literal("B%"))
		repeated := PatternFrom(bean, "a", likeA).Until(PatternFrom(bean, "b", likeB))
		correlated := PatternFrom(bean, "c", BetweenOf(
			Field[patternOpBean, int]("intPrimitive"),
			TagFieldAt[int]("a", 0, "intPrimitive"),
			TagFieldAt[int]("a", 1, "intPrimitive"),
		))
		rowsPtr := deploy(t, env, engine, repeated.Then(correlated), Alias("cInt", TagField[int]("c", "intPrimitive")))
		sendLetters(t, engine,
			patternOpBean{TheString: "A1", IntPrimitive: 5},
			patternOpBean{TheString: "A2", IntPrimitive: 8},
			patternOpBean{TheString: "B1", IntPrimitive: -1},
			patternOpBean{TheString: "E1", IntPrimitive: 20},
			patternOpBean{TheString: "E2", IntPrimitive: 3},
		)
		if len(*rowsPtr) != 0 {
			t.Fatalf("out-of-range values must not match: rows = %v", *rowsPtr)
		}
		sendLetters(t, engine, patternOpBean{TheString: "E3", IntPrimitive: 5})
		if len(*rowsPtr) != 1 || rowAnyValue((*rowsPtr)[0], "cInt") != 5 {
			t.Fatalf("after E3/5: rows = %v", *rowsPtr)
		}
	})
}

// TestPatternMatchUntilRepeatUseTagsMatchesEsper mirrors PatternRepeatUseTags:
// bounded repeats of correlated sequences under every, a timer-driven
// until/every chain exercised for lifecycle safety, and a three-stream
// followed-by whose later legs correlate against the first leg's repeated
// tag array element A[0].intPrimitive.
func TestPatternMatchUntilRepeatUseTagsMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)

	t.Run("every-repeat-correlated-sequence", func(t *testing.T) {
		env, engine, streams := newPatternMuEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		sequence := PatternFrom(streams.a, "a", trueExpr).Then(
			PatternFrom(streams.b, "b", Equal[string](Field[patternNotB, string]("id"), TagField[string]("a", "id"))),
		)
		plan, err := env.Build(sequence.MatchUntil(2, 2).Every().Select(
			Alias("a0", TagFieldAt[string]("a", 0, "id")),
			Alias("a1", TagFieldAt[string]("a", 1, "id")),
			Alias("b0", TagFieldAt[string]("b", 0, "id")),
			Alias("b1", TagFieldAt[string]("b", 1, "id")),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := []Row{}
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					return fmt.Errorf("pattern result is not a row: %#v", result)
				}
				rows = append(rows, row)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		for _, event := range []any{patternNotA{ID: "A1"}, patternNotB{ID: "A1"}, patternNotA{ID: "A2"}, patternNotB{ID: "A2"}} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		if len(rows) != 1 {
			t.Fatalf("rows = %v, want one fire", rows)
		}
		row := rows[0]
		if got := rowAnyValue(row, "a0"); got != "A1" {
			t.Fatalf("a0 = %v, want A1", got)
		}
		if got := rowAnyValue(row, "a1"); got != "A2" {
			t.Fatalf("a1 = %v, want A2", got)
		}
		if got := rowAnyValue(row, "b0"); got != "A1" {
			t.Fatalf("b0 = %v, want A1", got)
		}
		if got := rowAnyValue(row, "b1"); got != "A2" {
			t.Fatalf("b1 = %v, want A2", got)
		}
	})

	t.Run("timer-until-chain-lifecycle", func(t *testing.T) {
		env := newPatternOpEnv(t)
		engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		source := From[patternOpBean](env, "SupportBean")
		strIs := func(value string) Expression[bool] {
			return Equal[string](Field[patternOpBean, string]("theString"), Literal(value))
		}
		first := PatternFrom(source, "e1", strIs("2")).MatchUntil(2, 0).Until(TimerInterval(source, 5*time.Second)).Every()
		second := PatternFrom(source, "e2", strIs("3")).MatchUntil(2, 0).Until(TimerInterval(source, 2*time.Second))
		plan, err := env.Build(first.Then(second).Select(Alias("n", TagCount("e1"))).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error { return nil }); err != nil {
			t.Fatal(err)
		}
		// Replay the Java lifecycle: sends and clock advances exercise the
		// every/until/timer interaction; the suite asserts no listener
		// behavior for this segment, only that it runs clean.
		steps := []struct {
			advanceTo int64
			event     *patternOpBean
		}{
			{-1, &patternOpBean{TheString: "2"}},
			{-1, &patternOpBean{TheString: "2"}},
			{5000, nil},
			{-1, &patternOpBean{TheString: "3"}},
			{-1, &patternOpBean{TheString: "3"}},
			{-1, &patternOpBean{TheString: "3"}},
			{-1, &patternOpBean{TheString: "3"}},
			{10000, nil},
			{-1, &patternOpBean{TheString: "2"}},
			{-1, &patternOpBean{TheString: "2"}},
			{15000, nil},
		}
		for _, step := range steps {
			if step.advanceTo >= 0 {
				if err := engine.AdvanceTime(context.Background(), time.UnixMilli(step.advanceTo).UTC()); err != nil {
					t.Fatal(err)
				}
				continue
			}
			if err := engine.SendEvent(context.Background(), *step.event); err != nil {
				t.Fatal(err)
			}
		}
		if err := deployment.Undeploy(context.Background()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("three-stream-repeat-correlation", func(t *testing.T) {
		env := newPatternOpEnv(t)
		engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		source := From[patternOpBean](env, "SupportBean")
		strIs := func(value string) Expression[bool] {
			return Equal[string](Field[patternOpBean, string]("theString"), Literal(value))
		}
		intIsA0 := Equal[int](Field[patternOpBean, int]("intPrimitive"), TagFieldAt[int]("A", 0, "intPrimitive"))
		first := PatternFrom(source, "A", strIs("1")).MatchUntil(2, 2).Every()
		second := PatternFrom(source, "B", And(strIs("2"), intIsA0)).MatchUntil(2, 2)
		third := PatternFrom(source, "C", And(strIs("3"), intIsA0)).MatchUntil(2, 2)
		plan, err := env.Build(first.Then(second).Then(third).Select(Alias("n", TagCount("C"))).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		fires := 0
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			fires += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		events := []patternOpBean{
			{TheString: "1", IntPrimitive: 10}, {TheString: "1", IntPrimitive: 20},
			{TheString: "2", IntPrimitive: 10}, {TheString: "2", IntPrimitive: 10},
			{TheString: "3", IntPrimitive: 10}, {TheString: "3", IntPrimitive: 10},
		}
		for _, event := range events {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		if fires != 1 {
			t.Fatalf("fires = %d, want 1", fires)
		}
	})
}

// TestPatternMatchUntilArrayFunctionRepeatMatchesEsper mirrors
// PatternArrayFunctionRepeat: Java reads the repeated tag array length
// through SupportStaticMethodLib.arrayLength and
// java.lang.reflect.Array.getLength; the fluent API exposes the same value
// through TagCount (registered Go-style mapping for the static-method
// array functions).
func TestPatternMatchUntilArrayFunctionRepeatMatchesEsper(t *testing.T) {
	env, engine, streams := newPatternMuEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	pattern := PatternFrom(streams.a, "a", Literal[bool](true)).MatchUntil(1, 0).Until(PatternFrom(streams.b, "b", Literal[bool](true)))
	plan, err := env.Build(pattern.Select(
		Alias("length", TagCount("a")),
		Alias("l2", TagCount("a")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := []Row{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("pattern result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []any{patternNotA{ID: "A1"}, patternNotA{ID: "A2"}, patternNotA{ID: "A3"}, patternNotB{ID: "A2"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %v, want one fire", rows)
	}
	if got := rowAnyValue(rows[0], "length"); got != int64(3) {
		t.Fatalf("length = %v, want 3", got)
	}
	if got := rowAnyValue(rows[0], "l2"); got != int64(3) {
		t.Fatalf("l2 = %v, want 3", got)
	}
}

// TestPatternMatchUntilInvalidMatchesEsper mirrors PatternInvalid for the
// validations the fluent API can express: inverted and negative bounds are
// rejected at Build, and a tag declared inside a match-until cannot be
// redeclared by sibling filters. Java-only checks (zero-valued range
// literals, until-required for variable ranges, tag-array references inside
// an own filter expression, non-numeric bounds expressions) map to Go
// compile-time or registered API-shape differences.
func TestPatternMatchUntilInvalidMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	t.Run("inverted-bounds", func(t *testing.T) {
		env, _, streams := newPatternMuEnv(t)
		_, err := env.Build(PatternFrom(streams.a, "a", trueExpr).MatchUntil(10, 4).Select(Alias("n", TagCount("a"))).Query())
		if err == nil {
			t.Fatal("expected inverted bounds to be rejected")
		}
	})
	t.Run("negative-bounds", func(t *testing.T) {
		env, _, streams := newPatternMuEnv(t)
		_, err := env.Build(PatternFrom(streams.a, "a", trueExpr).MatchUntil(-1, -1).Select(Alias("n", TagCount("a"))).Query())
		if err == nil {
			t.Fatal("expected negative bounds to be rejected")
		}
	})
	t.Run("zero-lower-bound-accepted", func(t *testing.T) {
		env, _, streams := newPatternMuEnv(t)
		_, err := env.Build(PatternFrom(streams.b, "b", trueExpr).MatchUntil(0, 4).Until(PatternFrom(streams.g, "g", trueExpr)).Select(Alias("n", TagCount("b"))).Query())
		if err != nil {
			t.Fatalf("[:4] form must build: %v", err)
		}
	})
	t.Run("tag-redeclared-after-until", func(t *testing.T) {
		env, _, streams := newPatternMuEnv(t)
		repeated := PatternFrom(streams.a, "a", trueExpr).Until(PatternFrom(streams.b, "c", trueExpr))
		_, err := env.Build(repeated.Then(PatternFrom(streams.c, "c", trueExpr)).Select(Alias("n", TagCount("a"))).Query())
		if err == nil {
			t.Fatal("expected duplicate tag across until and follow-on filter to be rejected")
		}
	})
	t.Run("tag-reused-inside-nested-until", func(t *testing.T) {
		env, _, streams := newPatternMuEnv(t)
		inner := PatternFrom(streams.a, "a", trueExpr).Until(PatternFrom(streams.b, "b", trueExpr))
		_, err := env.Build(inner.Until(PatternFrom(streams.a, "a", trueExpr)).Select(Alias("n", TagCount("a"))).Query())
		if err == nil {
			t.Fatal("expected tag reused across nested until to be rejected")
		}
	})
}
