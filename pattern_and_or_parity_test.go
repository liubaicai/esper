package esper

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"
)

// Parity coverage for PatternOperatorAnd / PatternOperatorOr (see docs
// esper-go-port-implementation-plan.md). Every expectation below was captured
// from the Java runtime with the regression event set (A1,B1,C1,B2,A2,D1,E1,
// F1,D2,B3,G1,D3) and mirrors PatternOperatorAndWHarness /
// PatternOperatorOrWHarness, including the per-combination match
// multiplicities produced by Esper's eventsPerChild bookkeeping.

// patternAndOrCase describes one W-harness expression: the fires expected per
// event, each fire flattened to sorted tag=value pairs. Fires within one
// event compare as a multiset, exactly like the Java harness compareLists.
type patternAndOrCase struct {
	name  string
	tags  []string
	build func(env *Environment) PatternStream
	want  map[string][]string
}

func runPatternAndOrCase(t *testing.T, testCase patternAndOrCase) {
	t.Helper()
	env, engine := newPatternNotEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	selections := make([]Selection, 0, len(testCase.tags))
	for _, tag := range testCase.tags {
		selections = append(selections, Alias(tag, TagField[string](tag, "id")))
	}
	plan, err := env.Build(testCase.build(env).Select(selections...).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	firesByEvent := map[string][]string{}
	currentEvent := ""
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("pattern result is not a row: %#v", result)
			}
			parts := make([]string, 0, len(testCase.tags))
			for _, tag := range testCase.tags {
				if value := row.Get(tag); value.State() == ValuePresent {
					parts = append(parts, tag+"="+value.Any().(string))
				}
			}
			firesByEvent[currentEvent] = append(firesByEvent[currentEvent], joinStrings(parts, " "))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, step := range patternAndOrEventSet() {
		currentEvent = step.id
		if err := engine.SendEvent(context.Background(), step.event); err != nil {
			t.Fatal(err)
		}
	}
	assertPatternAndOrFires(t, testCase.name, testCase.want, firesByEvent)
}

func assertPatternAndOrFires(t *testing.T, name string, want, got map[string][]string) {
	t.Helper()
	events := map[string]bool{}
	for event := range want {
		events[event] = true
	}
	for event := range got {
		events[event] = true
	}
	ordered := make([]string, 0, len(events))
	for event := range events {
		ordered = append(ordered, event)
	}
	sort.Strings(ordered)
	for _, event := range ordered {
		wantFires := append([]string(nil), want[event]...)
		gotFires := append([]string(nil), got[event]...)
		sort.Strings(wantFires)
		sort.Strings(gotFires)
		if fmt.Sprintf("%v", wantFires) != fmt.Sprintf("%v", gotFires) {
			t.Fatalf("%s after %s: fires = %v, want %v", name, event, gotFires, wantFires)
		}
	}
}

type patternAndOrEvent struct {
	id    string
	event any
}

// patternAndOrEventSet replays the Java EventCollectionFactory mixed set in
// order: A1, B1, C1, B2, A2, D1, E1, F1, D2, B3, G1, D3.
func patternAndOrEventSet() []patternAndOrEvent {
	return []patternAndOrEvent{
		{"A1", patternNotA{ID: "A1"}}, {"B1", patternNotB{ID: "B1"}}, {"C1", patternNotC{ID: "C1"}},
		{"B2", patternNotB{ID: "B2"}}, {"A2", patternNotA{ID: "A2"}}, {"D1", patternNotD{ID: "D1"}},
		{"E1", patternNotE{ID: "E1"}}, {"F1", patternNotF{ID: "F1"}}, {"D2", patternNotD{ID: "D2"}},
		{"B3", patternNotB{ID: "B3"}}, {"G1", patternNotG{ID: "G1"}}, {"D3", patternNotD{ID: "D3"}},
	}
}

func joinStrings(parts []string, sep string) string {
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += sep
		}
		out += part
	}
	return out
}

// TestPatternOperatorAndWHarnessMatchesEsper covers all fifteen expressions
// of PatternOperatorAndWHarness over the mixed event set. The Java object
// model round-trip case (SerializableObjectCopier on every(b and d)) is
// covered by the Go-side pattern model shape tests; the text and model cases
// share expectations, so only the text form runs here.
func TestPatternOperatorAndWHarnessMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	cases := []patternAndOrCase{
		{
			name: "b and d",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b", trueExpr).And(PatternFrom(d, "d", trueExpr))
			},
			want: map[string][]string{
				"D1": {"b=B1 d=D1"},
			},
		},
		{
			name: "b and every d",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b", trueExpr).And(PatternFrom(d, "d", trueExpr).Every())
			},
			want: map[string][]string{
				"D1": {"b=B1 d=D1"},
				"D2": {"b=B1 d=D2"},
				"D3": {"b=B1 d=D3"},
			},
		},
		{
			name: "every b and d",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b", trueExpr).Every().And(PatternFrom(d, "d", trueExpr))
			},
			want: map[string][]string{
				"D1": {"b=B1 d=D1", "b=B2 d=D1"},
				"B3": {"b=B3 d=D1"},
			},
		},
		{
			name: "every (b and d)",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b", trueExpr).And(PatternFrom(d, "d", trueExpr)).Every()
			},
			want: map[string][]string{
				"D1": {"b=B1 d=D1"},
				"B3": {"b=B3 d=D2"},
			},
		},
		{
			name: "every (b and every d)",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b", trueExpr).And(PatternFrom(d, "d", trueExpr).Every()).Every()
			},
			want: map[string][]string{
				"D1": {"b=B1 d=D1"},
				"D2": {"b=B1 d=D2"},
				"B3": {"b=B3 d=D2"},
				"D3": {"b=B1 d=D3", "b=B3 d=D3", "b=B3 d=D3"},
			},
		},
		{
			name: "every b and every d",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b", trueExpr).Every().And(PatternFrom(d, "d", trueExpr).Every())
			},
			want: map[string][]string{
				"D1": {"b=B1 d=D1", "b=B2 d=D1"},
				"D2": {"b=B1 d=D2", "b=B2 d=D2"},
				"B3": {"b=B3 d=D1", "b=B3 d=D2"},
				"D3": {"b=B1 d=D3", "b=B2 d=D3", "b=B3 d=D3"},
			},
		},
		{
			name: "every (every b and d)",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b", trueExpr).Every().And(PatternFrom(d, "d", trueExpr)).Every()
			},
			want: map[string][]string{
				"D1": {"b=B1 d=D1", "b=B2 d=D1"},
				"B3": {"b=B3 d=D1", "b=B3 d=D2", "b=B3 d=D2"},
			},
		},
		{
			name: "every a and d and b",
			tags: []string{"a", "b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(a, "a", trueExpr).Every().And(PatternFrom(d, "d", trueExpr)).And(PatternFrom(b, "b", trueExpr))
			},
			want: map[string][]string{
				"D1": {"a=A1 b=B1 d=D1", "a=A2 b=B1 d=D1"},
			},
		},
		{
			name: "every (every b and every d)",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b", trueExpr).Every().And(PatternFrom(d, "d", trueExpr).Every()).Every()
			},
			want: map[string][]string{
				"D1": {"b=B1 d=D1", "b=B2 d=D1"},
				"D2": {"b=B1 d=D2", "b=B2 d=D2"},
				"B3": {"b=B3 d=D1", "b=B3 d=D2", "b=B3 d=D2", "b=B3 d=D2"},
				"D3": {"b=B1 d=D3", "b=B2 d=D3", "b=B3 d=D3", "b=B3 d=D3", "b=B3 d=D3", "b=B3 d=D3", "b=B3 d=D3"},
			},
		},
		{
			name: "a and d and b",
			tags: []string{"a", "b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(a, "a", trueExpr).And(PatternFrom(d, "d", trueExpr)).And(PatternFrom(b, "b", trueExpr))
			},
			want: map[string][]string{
				"D1": {"a=A1 b=B1 d=D1"},
			},
		},
		{
			name: "every a and every d and b",
			tags: []string{"a", "b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(a, "a", trueExpr).Every().And(PatternFrom(d, "d", trueExpr).Every()).And(PatternFrom(b, "b", trueExpr))
			},
			want: map[string][]string{
				"D1": {"a=A1 b=B1 d=D1", "a=A2 b=B1 d=D1"},
				"D2": {"a=A1 b=B1 d=D2", "a=A2 b=B1 d=D2"},
				"D3": {"a=A1 b=B1 d=D3", "a=A2 b=B1 d=D3"},
			},
		},
		{
			name: "b and b",
			tags: []string{"b1", "b2"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b1", trueExpr).And(PatternFrom(b, "b2", trueExpr))
			},
			want: map[string][]string{
				"B1": {"b1=B1 b2=B1"},
			},
		},
		{
			name: "every a and every d and every b",
			tags: []string{"a", "b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(a, "a", trueExpr).Every().And(PatternFrom(d, "d", trueExpr).Every()).And(PatternFrom(b, "b", trueExpr).Every())
			},
			want: map[string][]string{
				"D1": {"a=A1 b=B1 d=D1", "a=A1 b=B2 d=D1", "a=A2 b=B1 d=D1", "a=A2 b=B2 d=D1"},
				"D2": {"a=A1 b=B1 d=D2", "a=A1 b=B2 d=D2", "a=A2 b=B1 d=D2", "a=A2 b=B2 d=D2"},
				"B3": {"a=A1 b=B3 d=D1", "a=A1 b=B3 d=D2", "a=A2 b=B3 d=D1", "a=A2 b=B3 d=D2"},
				"D3": {"a=A1 b=B1 d=D3", "a=A1 b=B2 d=D3", "a=A1 b=B3 d=D3", "a=A2 b=B1 d=D3", "a=A2 b=B2 d=D3", "a=A2 b=B3 d=D3"},
			},
		},
		{
			name: "every (a and every d and b)",
			tags: []string{"a", "b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(a, "a", trueExpr).And(PatternFrom(d, "d", trueExpr).Every()).And(PatternFrom(b, "b", trueExpr)).Every()
			},
			want: map[string][]string{
				"D1": {"a=A1 b=B1 d=D1"},
				"D2": {"a=A1 b=B1 d=D2"},
				"D3": {"a=A1 b=B1 d=D3"},
			},
		},
		{
			name: "every (b and b)",
			tags: []string{"b1", "b2"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b1", trueExpr).And(PatternFrom(b, "b2", trueExpr)).Every()
			},
			want: map[string][]string{
				"B1": {"b1=B1 b2=B1"},
				"B2": {"b1=B2 b2=B2"},
				"B3": {"b1=B3 b2=B3"},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runPatternAndOrCase(t, testCase)
		})
	}
}

// TestPatternOperatorOrWHarnessMatchesEsper covers all nine expressions of
// PatternOperatorOrWHarness over the mixed event set, including the
// nested-every multiplicity chains (every completion of a non-quitting or
// branch spawns one more attempt in Esper).
func TestPatternOperatorOrWHarnessMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	cases := []patternAndOrCase{
		{
			name: "a or a",
			tags: []string{"a", "a2"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(a, "a", trueExpr).Or(PatternFrom(a, "a2", trueExpr))
			},
			want: map[string][]string{
				"A1": {"a=A1 a2=A1"},
			},
		},
		{
			name: "a or b or c",
			tags: []string{"a", "b", "c"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(a, "a", trueExpr).Or(PatternFrom(b, "b", trueExpr)).Or(PatternFrom(c, "c", trueExpr))
			},
			want: map[string][]string{
				"A1": {"a=A1"},
			},
		},
		{
			name: "every b or every d",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b", trueExpr).Every().Or(PatternFrom(d, "d", trueExpr).Every())
			},
			want: map[string][]string{
				"B1": {"b=B1"},
				"B2": {"b=B2"},
				"D1": {"d=D1"},
				"D2": {"d=D2"},
				"B3": {"b=B3"},
				"D3": {"d=D3"},
			},
		},
		{
			name: "a or b",
			tags: []string{"a", "b"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(a, "a", trueExpr).Or(PatternFrom(b, "b", trueExpr))
			},
			want: map[string][]string{
				"A1": {"a=A1"},
			},
		},
		{
			name: "a or every b",
			tags: []string{"a", "b"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(a, "a", trueExpr).Or(PatternFrom(b, "b", trueExpr).Every())
			},
			want: map[string][]string{
				"A1": {"a=A1"},
			},
		},
		{
			name: "every a or d",
			tags: []string{"a", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(a, "a", trueExpr).Every().Or(PatternFrom(d, "d", trueExpr))
			},
			want: map[string][]string{
				"A1": {"a=A1"},
				"A2": {"a=A2"},
				"D1": {"d=D1"},
			},
		},
		{
			name: "every (every b or d)",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b", trueExpr).Every().Or(PatternFrom(d, "d", trueExpr)).Every()
			},
			want: map[string][]string{
				"B1": {"b=B1"},
				"B2": {"b=B2", "b=B2"},
				"D1": {"d=D1", "d=D1", "d=D1", "d=D1"},
				"D2": {"d=D2", "d=D2", "d=D2", "d=D2"},
				"B3": {"b=B3", "b=B3", "b=B3", "b=B3"},
				"D3": {"d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3"},
			},
		},
		{
			name: "every (b or every d)",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(b, "b", trueExpr).Or(PatternFrom(d, "d", trueExpr).Every()).Every()
			},
			want: map[string][]string{
				"B1": {"b=B1"},
				"D1": {"d=D1"},
				"B2": {"b=B2"},
				"D2": {"d=D2", "d=D2"},
				"B3": {"b=B3", "b=B3", "b=B3", "b=B3"},
				"D3": {"d=D3", "d=D3", "d=D3", "d=D3"},
			},
		},
		{
			name: "every (every d or every b)",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				_, _, _, _ = a, b, c, d
				return PatternFrom(d, "d", trueExpr).Every().Or(PatternFrom(b, "b", trueExpr).Every()).Every()
			},
			want: map[string][]string{
				"B1": {"b=B1"},
				"B2": {"b=B2", "b=B2"},
				"D1": {"d=D1", "d=D1", "d=D1", "d=D1"},
				"D2": {"d=D2", "d=D2", "d=D2", "d=D2", "d=D2", "d=D2", "d=D2", "d=D2"},
				"B3": {"b=B3", "b=B3", "b=B3", "b=B3", "b=B3", "b=B3", "b=B3", "b=B3", "b=B3", "b=B3", "b=B3", "b=B3", "b=B3", "b=B3", "b=B3", "b=B3"},
				"D3": {"d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3", "d=D3"},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runPatternAndOrCase(t, testCase)
		})
	}
}

// TestPatternOperatorAndNotDefaultTrueMatchesEsper covers ESPER-402
// (PatternOperatorAndNotDefaultTrue): an and of two correlated not-branches
// is satisfied as soon as the enclosing followed-by attempt starts, so the
// statement reports one waiting call per A event as long as neither the
// correlated B nor C arrives.
func TestPatternOperatorAndNotDefaultTrueMatchesEsper(t *testing.T) {
	env, engine := newPatternNotEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	trueExpr := Literal[bool](true)
	a := From[patternNotA](env, "SupportBean_A")
	b := From[patternNotB](env, "SupportBean_B")
	c := From[patternNotC](env, "SupportBean_C")
	notB := PatternFrom(b, "b", Equal[string](
		Field[patternNotB, string]("id"), TagField[string]("call", "id"),
	)).Not()
	notC := PatternFrom(c, "c", Equal[string](
		Field[patternNotC, string]("id"), TagField[string]("call", "id"),
	)).Not()
	pattern := PatternFrom(a, "call", trueExpr).Every().Then(notB.And(notC))

	plan, err := env.Build(pattern.Select(
		Alias("calls", CountAll()),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
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

	// A1 completes the and-of-nots immediately: neither SupportBean_B("A1")
	// nor SupportBean_C("A1") exists, so the vacant not matches fire.
	if err := engine.Send(context.Background(), "SupportBean_A", patternNotA{ID: "A1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("calls").Any() != int64(1) {
		t.Fatalf("rows after A1 = %#v, want one row with calls=1", rows)
	}

	// B1 and C1 do not correlate with call.id=A1, so the not-branches stay
	// satisfied and no further fires occur.
	if err := engine.Send(context.Background(), "SupportBean_B", patternNotB{ID: "B1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean_C", patternNotC{ID: "C1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows after B1/C1 = %#v, want the single A1 fire", rows)
	}
}

// TestPatternOperatorAndWithEveryAndTerminationOptimizationMatchesEsper
// covers PatternOperatorAndWithEveryAndTerminationOptimization: once the
// plain a-side quit, its match stays cached in the and's eventsPerChild and
// every later B fires with that same A1.
func TestPatternOperatorAndWithEveryAndTerminationOptimizationMatchesEsper(t *testing.T) {
	env, engine := newPatternNotEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	trueExpr := Literal[bool](true)
	a := From[patternNotA](env, "SupportBean_A")
	b := From[patternNotB](env, "SupportBean_B")
	pattern := PatternFrom(a, "a", trueExpr).And(PatternFrom(b, "b", trueExpr).Every())

	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "id")),
		Alias("b", TagField[string]("b", "id")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var fires []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("pattern result is not a row: %#v", result)
			}
			fires = append(fires, row.Get("a").Any().(string)+"/"+row.Get("b").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.Send(context.Background(), "SupportBean_A", patternNotA{ID: "A1"}); err != nil {
		t.Fatal(err)
	}
	want := make([]string, 0, 11)
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("B%d", i)
		if err := engine.Send(context.Background(), "SupportBean_B", patternNotB{ID: id}); err != nil {
			t.Fatal(err)
		}
		want = append(want, "A1/"+id)
	}
	if err := engine.Send(context.Background(), "SupportBean_B", patternNotB{ID: "B_last"}); err != nil {
		t.Fatal(err)
	}
	want = append(want, "A1/B_last")
	if fmt.Sprintf("%v", fires) != fmt.Sprintf("%v", want) {
		t.Fatalf("fires = %v, want %v", fires, want)
	}
}

// TestPatternOperatorOrAndNotAndZeroStartMatchesEsper covers
// PatternOrAndNotAndZeroStart: the or-branch whose right side is a not fires
// with a vacant b as soon as a arrives, the positive branch still completes
// when B arrives later, and a zero-length interval arms a fire at the
// deployment clock.
func TestPatternOperatorOrAndNotAndZeroStartMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)
	t.Run("(a -> b) or (a -> not b)", func(t *testing.T) {
		env, engine := newPatternNotEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		a := From[patternNotA](env, "SupportBean_A")
		b := From[patternNotB](env, "SupportBean_B")
		positive := PatternFrom(a, "a", trueExpr).Then(PatternFrom(b, "b", trueExpr))
		negative := PatternFrom(a, "a", trueExpr).Then(PatternFrom(b, "b", trueExpr).Not())
		assertOrAndNotFires(t, env, engine, positive.Or(negative))
	})
	t.Run("a -> (b or not b)", func(t *testing.T) {
		env, engine := newPatternNotEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		a := From[patternNotA](env, "SupportBean_A")
		b := From[patternNotB](env, "SupportBean_B")
		pattern := PatternFrom(a, "a", trueExpr).Then(
			PatternFrom(b, "b", trueExpr).Or(PatternFrom(b, "nb", trueExpr).Not()),
		)
		assertOrAndNotFires(t, env, engine, pattern)
	})
	t.Run("timer:interval(0) or every timer:interval(1 min)", func(t *testing.T) {
		env, _ := newPatternNotEnv(t)
		base := From[patternNotA](env, "SupportBean_A")
		pattern := TimerInterval(base, 0).Or(TimerInterval(base, time.Minute).Every())
		plan, err := env.Build(pattern.Select(Alias("now", CurrentTime())).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		var rows []Row
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				if row, ok := result.Row(); ok {
					rows = append(rows, row)
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC()); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("zero-length interval did not fire at the deployment clock")
		}
	})
}

// assertOrAndNotFires replays the shared OrAndNotAndZeroStart flow: A1 fires
// with a vacant b, then B1 fires with both tags populated.
func assertOrAndNotFires(t *testing.T, env *Environment, engine *Engine, pattern PatternStream) {
	t.Helper()
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "id")),
		Alias("b", TagField[string]("b", "id")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var fires [][]string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("pattern result is not a row: %#v", result)
			}
			fire := []string{}
			if value := row.Get("a"); value.State() == ValuePresent {
				fire = append(fire, "a="+value.Any().(string))
			}
			if value := row.Get("b"); value.State() == ValuePresent {
				fire = append(fire, "b="+value.Any().(string))
			}
			fires = append(fires, fire)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean_A", patternNotA{ID: "A1"}); err != nil {
		t.Fatal(err)
	}
	if len(fires) != 1 || fmt.Sprintf("%v", fires[0]) != "[a=A1]" {
		t.Fatalf("fires after A1 = %v, want [[a=A1]]", fires)
	}
	if err := engine.Send(context.Background(), "SupportBean_B", patternNotB{ID: "B1"}); err != nil {
		t.Fatal(err)
	}
	if len(fires) != 2 || fmt.Sprintf("%v", fires[1]) != "[a=A1 b=B1]" {
		t.Fatalf("fires after B1 = %v, want second fire [a=A1 b=B1]", fires)
	}
}
