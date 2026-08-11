package esper

import (
	"strings"
	"testing"
	"time"
)

// Parity coverage for regression-lib/suite/pattern/PatternExpressionText.java.
// The Java suite compiles ~150 pattern texts and asserts the AST-to-text
// rendering (EvalRootForgeNode.toEPL). The fluent API builds the same ASTs
// directly: each case asserts the detached model shape (Kind/children), a
// stable canonical description across inspections, the operator-specific
// description markers that carry the Java distinction (followed-by maximum,
// match-until bounds, within-max, consume levels, every-distinct expiry),
// and a successful Build (the env.compile counterpart).
//
// Go-style differences: Java untagged filters and terminators take distinct
// placeholder tags in the fluent API (never projected); parenthesized EPL
// variants of one AST collapse to a single fluent form, so grouping-only
// duplicates share one subtest; the canonical description is the Go
// compiler's typed pattern text (tag:predicate), not EPL.

type patternTextCase struct {
	name         string
	java         string
	build        func() PatternStream
	shape        string
	descContains []string
}

func runPatternTextCases(t *testing.T, env *Environment, tests []patternTextCase) {
	t.Helper()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pattern := test.build()
			model := pattern.Model()
			if got := patternModelShape(model); got != test.shape {
				t.Fatalf("pattern %s [%s] shape = %q, want %q (description %s)", test.name, test.java, got, test.shape, model.Description())
			}
			description := model.Description()
			if description == "" {
				t.Fatalf("pattern %s [%s] has no canonical description", test.name, test.java)
			}
			if again := pattern.Model().Description(); again != description {
				t.Fatalf("pattern %s [%s] description changed between inspections: %q vs %q", test.name, test.java, description, again)
			}
			for _, marker := range test.descContains {
				if !strings.Contains(description, marker) {
					t.Fatalf("pattern %s [%s] description = %q, want marker %q", test.name, test.java, description, marker)
				}
			}
			if _, err := env.Build(pattern.Select(Alias("c", Literal(1))).Query(StatementName("pattern-text-" + test.name))); err != nil {
				t.Fatalf("pattern %s [%s] failed to build: %v", test.name, test.java, err)
			}
		})
	}
}

func newPatternTextEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	registrations := []struct {
		name  string
		apply func() error
	}{
		{"SupportBean", func() error { _, err := RegisterStruct[patternOpBean](env, "SupportBean"); return err }},
		{"SupportBean_A", func() error { _, err := RegisterStruct[patternNotA](env, "SupportBean_A"); return err }},
		{"SupportBean_B", func() error { _, err := RegisterStruct[patternNotB](env, "SupportBean_B"); return err }},
		{"SupportBean_C", func() error { _, err := RegisterStruct[patternNotC](env, "SupportBean_C"); return err }},
		{"SupportBean_D", func() error { _, err := RegisterStruct[patternNotD](env, "SupportBean_D"); return err }},
		{"SupportBean_E", func() error { _, err := RegisterStruct[patternNotE](env, "SupportBean_E"); return err }},
		{"SupportBean_G", func() error { _, err := RegisterStruct[patternNotG](env, "SupportBean_G"); return err }},
	}
	for _, registration := range registrations {
		if err := registration.apply(); err != nil {
			t.Fatalf("register %s: %v", registration.name, err)
		}
	}
	return env
}

// TestPatternExpressionTextOperatorsMatchesEsper covers the operator,
// repetition, until, consume, while and every-distinct text forms.
func TestPatternExpressionTextOperatorsMatchesEsper(t *testing.T) {
	env := newPatternTextEnv(t)
	sb := From[patternOpBean](env, "SupportBean")
	sa := From[patternNotA](env, "SupportBean_A")
	sbB := From[patternNotB](env, "SupportBean_B")
	sc := From[patternNotC](env, "SupportBean_C")
	sd := From[patternNotD](env, "SupportBean_D")
	se := From[patternNotE](env, "SupportBean_E")
	sg := From[patternNotG](env, "SupportBean_G")

	event := func(tag string) PatternStream { return PatternFrom(sb, tag, Literal(true)) }
	eventA := func(tag string) PatternStream { return PatternFrom(sa, tag, Literal(true)) }
	eventB := func(tag string) PatternStream { return PatternFrom(sbB, tag, Literal(true)) }
	eventC := func(tag string) PatternStream { return PatternFrom(sc, tag, Literal(true)) }
	eventD := func(tag string) PatternStream { return PatternFrom(sd, tag, Literal(true)) }
	eventE := func(tag string) PatternStream { return PatternFrom(se, tag, Literal(true)) }
	eventG := func(tag string) PatternStream { return PatternFrom(sg, tag, Literal(true)) }
	idB := func(value string) Expression[bool] {
		return Equal[string](Field[patternNotB, string]("id"), Literal(value))
	}
	idA := func(value string) Expression[bool] {
		return Equal[string](Field[patternNotA, string]("id"), Literal(value))
	}
	intF := Field[patternOpBean, int]("intPrimitive")
	theString := Field[patternOpBean, string]("theString")
	likeA := LikeOf(theString, Literal("A%"))
	likeB := LikeOf(theString, Literal("B%"))
	keyA := TagField[int]("a", "intPrimitive")
	keyB := TagField[int]("b", "intPrimitive")

	tests := []patternTextCase{
		// every a=SupportBean -> b=SupportBean@consume (asserted twice in Java,
		// plus the @consume(2) level form).
		{name: "consume-default", java: "every a=SupportBean -> b=SupportBean@consume",
			build: func() PatternStream { return event("a").Every().Then(event("b").Consume()) },
			shape: "followed-by(every(event),event)", descContains: []string{".consume("}},
		{name: "consume-level-2", java: "every a=SupportBean -> b=SupportBean@consume(2)",
			build: func() PatternStream { return event("a").Every().Then(event("b").Consume(2)) },
			shape: "followed-by(every(event),event)", descContains: []string{".consume(2)"}},
		{name: "followed-ab", java: "a=SupportBean_A -> b=SupportBean_B",
			build: func() PatternStream { return eventA("a").Then(eventB("b")) },
			shape: "followed-by(event,event)"},

		// and-combinations with every.
		{name: "and-b-every-d", java: "b=SupportBean_B and every d=SupportBean_D",
			build: func() PatternStream { return eventB("b").And(eventD("d").Every()) },
			shape: "and(event,every(event))"},
		{name: "and-every-b-d", java: "every b=SupportBean_B and d=SupportBean_B",
			build: func() PatternStream { return eventB("b").Every().And(eventD("d")) },
			shape: "and(every(event),event)"},
		{name: "and-b-d", java: "b=SupportBean_B and d=SupportBean_D",
			build: func() PatternStream { return eventB("b").And(eventD("d")) },
			shape: "and(event,event)"},
		{name: "every-and-b-d", java: "every (b=SupportBean_B and d=SupportBean_D)",
			build: func() PatternStream { return eventB("b").And(eventD("d")).Every() },
			shape: "every(and(event,event))"},
		{name: "every-and-b-every-d", java: "every (b=SupportBean_B and every d=SupportBean_D)",
			build: func() PatternStream { return eventB("b").And(eventD("d").Every()).Every() },
			shape: "every(and(event,every(event)))"},
		{name: "and-every-b-every-d", java: "every b=SupportBean_B and every d=SupportBean_D",
			build: func() PatternStream { return eventB("b").Every().And(eventD("d").Every()) },
			shape: "and(every(event),every(event))"},
		{name: "every-and-every-b-d", java: "every (every b=SupportBean_B and d=SupportBean_D)",
			build: func() PatternStream { return eventB("b").Every().And(eventD("d")).Every() },
			shape: "every(and(every(event),event))"},
		{name: "and-every-a-d-b", java: "every a=SupportBean_A and d=SupportBean_D and b=SupportBean_B",
			build: func() PatternStream { return eventA("a").Every().And(eventD("d")).And(eventB("b")) },
			shape: "and(and(every(event),event),event)"},
		{name: "every-and-every-b-every-d", java: "every (every b=SupportBean_B and every d=SupportBean_D)",
			build: func() PatternStream { return eventB("b").Every().And(eventD("d").Every()).Every() },
			shape: "every(and(every(event),every(event)))"},
		{name: "and-a-d-b", java: "a=SupportBean_A and d=SupportBean_D and b=SupportBean_B",
			build: func() PatternStream { return eventA("a").And(eventD("d")).And(eventB("b")) },
			shape: "and(and(event,event),event)"},
		{name: "and-every-a-every-d-b", java: "every a=SupportBean_A and every d=SupportBean_D and b=SupportBean_B",
			build: func() PatternStream { return eventA("a").Every().And(eventD("d").Every()).And(eventB("b")) },
			shape: "and(and(every(event),every(event)),event)"},
		{name: "and-b-b", java: "b=SupportBean_B and b=SupportBean_B",
			build: func() PatternStream { return eventB("b1").And(eventB("b2")) },
			shape: "and(event,event)"},
		{name: "and-every-a-every-d-every-b", java: "every a=SupportBean_A and every d=SupportBean_D and every b=SupportBean_B",
			build: func() PatternStream { return eventA("a").Every().And(eventD("d").Every()).And(eventB("b").Every()) },
			shape: "and(and(every(event),every(event)),every(event))"},
		{name: "every-and-a-every-d-b", java: "every (a=SupportBean_A and every d=SupportBean_D and b=SupportBean_B)",
			build: func() PatternStream { return eventA("a").And(eventD("d").Every()).And(eventB("b")).Every() },
			shape: "every(and(and(event,every(event)),event))"},
		{name: "every-and-b-b", java: "every (b=SupportBean_B and b=SupportBean_B)",
			build: func() PatternStream { return eventB("b1").And(eventB("b2")).Every() },
			shape: "every(and(event,event))"},
		{name: "every-b", java: "every b=SupportBean_B",
			build: func() PatternStream { return eventB("b").Every() },
			shape: "every(event)"},
		{name: "bare-b", java: "b=SupportBean_B",
			build: func() PatternStream { return eventB("b") },
			shape: "event"},
		// every (every (every b)) renders as every every every b in Java.
		{name: "every-nested-thrice", java: "every (every (every b=SupportBean_B))",
			build: func() PatternStream { return eventB("b").Every().Every().Every() },
			shape: "every(every(every(event)))"},
		{name: "every-nested-twice", java: "every (every b=SupportBean_B())",
			build: func() PatternStream { return eventB("b").Every().Every() },
			shape: "every(every(event))"},

		// followed-by combinations.
		{name: "or-followed-bd-not-d", java: "b=SupportBean_B -> d=SupportBean_D or not d=SupportBean_D",
			build: func() PatternStream { return eventB("b").Then(eventD("d")).Or(eventD("d").Not()) },
			shape: "or(followed-by(event,event),not(event))"},
		{name: "followed-b-or-d-not-d", java: "b=SupportBean_B -> (d=SupportBean_D or not d=SupportBean_D)",
			build: func() PatternStream { return eventB("b").Then(eventD("d").Or(eventD("d").Not())) },
			shape: "followed-by(event,or(event,not(event)))"},
		{name: "or-followedmax-bd-not-d", java: "b=SupportBean_B -[1000]> d=SupportBean_D or not d=SupportBean_D",
			build: func() PatternStream { return eventB("b").ThenMax(1000, eventD("d")).Or(eventD("d").Not()) },
			shape: "or(followed-by(event,event),not(event))", descContains: []string{"-[1000]>"}},
		{name: "followed-b-every-d", java: "b=SupportBean_B -> every d=SupportBean_D",
			build: func() PatternStream { return eventB("b").Then(eventD("d").Every()) },
			shape: "followed-by(event,every(event))"},
		{name: "followed-b-d", java: "b=SupportBean_B -> d=SupportBean_D",
			build: func() PatternStream { return eventB("b").Then(eventD("d")) },
			shape: "followed-by(event,event)"},
		{name: "followed-b-not-d", java: "b=SupportBean_B -> not d=SupportBean_D",
			build: func() PatternStream { return eventB("b").Then(eventD("d").Not()) },
			shape: "followed-by(event,not(event))"},
		{name: "followedmax-b-not-d", java: "b=SupportBean_B -[1000]> not d=SupportBean_D",
			build: func() PatternStream { return eventB("b").ThenMax(1000, eventD("d").Not()) },
			shape: "followed-by(event,not(event))", descContains: []string{"-[1000]>"}},
		{name: "followed-every-b-every-d", java: "every b=SupportBean_B -> every d=SupportBean_D",
			build: func() PatternStream { return eventB("b").Every().Then(eventD("d").Every()) },
			shape: "followed-by(every(event),every(event))"},
		{name: "followed-every-b-d", java: "every b=SupportBean_B -> d=SupportBean_D",
			build: func() PatternStream { return eventB("b").Every().Then(eventD("d")) },
			shape: "followed-by(every(event),event)"},
		{name: "followedmax-every-b-d", java: "every b=SupportBean_B -[10]> d=SupportBean_D",
			build: func() PatternStream { return eventB("b").Every().ThenMax(10, eventD("d")) },
			shape: "followed-by(every(event),event)", descContains: []string{"-[10]>"}},
		{name: "every-followed-b-every-d", java: "every (b=SupportBean_B -> every d=SupportBean_D)",
			build: func() PatternStream { return eventB("b").Then(eventD("d").Every()).Every() },
			shape: "every(followed-by(event,every(event)))"},
		{name: "every-followed-chain-aba", java: "every (a_1=SupportBean_A -> b=SupportBean_B -> a_2=SupportBean_A)",
			build: func() PatternStream { return eventA("a_1").Then(eventB("b")).Then(eventA("a_2")).Every() },
			shape: "every(followed-by(followed-by(event,event),event))"},
		{name: "followed-chain-cda", java: "c=SupportBean_C -> d=SupportBean_D -> a=SupportBean_A",
			build: func() PatternStream { return eventC("c").Then(eventD("d")).Then(eventA("a")) },
			shape: "followed-by(followed-by(event,event),event)"},
		{name: "every-followedmax-chain-aba", java: "every (a_1=SupportBean_A -[10]> b=SupportBean_B -[10]> a_2=SupportBean_A)",
			build: func() PatternStream { return eventA("a_1").ThenMax(10, eventB("b")).ThenMax(10, eventA("a_2")).Every() },
			shape: "every(followed-by(followed-by(event,event),event))", descContains: []string{"-[10]>"}},
		{name: "every-followed-every-a-every-b", java: "every (every a=SupportBean_A -> every b=SupportBean_B)",
			build: func() PatternStream { return eventA("a").Every().Then(eventB("b").Every()).Every() },
			shape: "every(followed-by(every(event),every(event)))"},
		{name: "every-followed-a-every-b", java: "every (a=SupportBean_A -> every b=SupportBean_B)",
			build: func() PatternStream { return eventA("a").Then(eventB("b").Every()).Every() },
			shape: "every(followed-by(event,every(event)))"},

		// until and match-until forms.
		{name: "until-a-d", java: "a=SupportBean_A(id='A2') until SupportBean_D",
			build: func() PatternStream { return PatternFrom(sa, "a", idA("A2")).Until(eventD("d")) },
			shape: "until(event,event)"},
		{name: "until-b-a", java: "b=SupportBean_B until a=SupportBean_A",
			build: func() PatternStream { return eventB("b").Until(eventA("a")) },
			shape: "until(event,event)"},
		{name: "until-b-d", java: "b=SupportBean_B until SupportBean_D",
			build: func() PatternStream { return eventB("b").Until(eventD("d")) },
			shape: "until(event,event)"},
		{name: "until-or-ab-d", java: "(a=SupportBean_A or b=SupportBean_B) until d=SupportBean_D",
			build: func() PatternStream { return eventA("a").Or(eventB("b")).Until(eventD("d")) },
			shape: "until(or(event,event),event)"},
		{name: "until-or-ab-or-gd", java: "(a=SupportBean_A or b=SupportBean_B) until (g=SupportBean_G or d=SupportBean_D)",
			build: func() PatternStream { return eventA("a").Or(eventB("b")).Until(eventG("g").Or(eventD("d"))) },
			shape: "until(or(event,event),or(event,event))"},
		{name: "until-a-g", java: "a=SupportBean_A until SupportBean_G",
			build: func() PatternStream { return eventA("a").Until(eventG("g")) },
			shape: "until(event,event)"},
		{name: "repeat-2-a", java: "[2] a=SupportBean_A",
			build: func() PatternStream { return eventA("a").MatchUntil(2, 2) },
			shape: "match-until(event)", descContains: []string{"match-until(2:2,"}},
		{name: "repeat-1-1-a", java: "[1:1] a=SupportBean_A",
			build: func() PatternStream { return eventA("a").MatchUntil(1, 1) },
			shape: "match-until(event)", descContains: []string{"match-until(1:1,"}},
		{name: "repeat-4-or-ab", java: "[4] (a=SupportBean_A or b=SupportBean_B)",
			build: func() PatternStream { return eventA("a").Or(eventB("b")).MatchUntil(4, 4) },
			shape: "match-until(or(event,event))", descContains: []string{"match-until(4:4,"}},
		{name: "repeat-2-b-until-a", java: "[2] b=SupportBean_B until a=SupportBean_A",
			build: func() PatternStream { return eventB("b").MatchUntil(2, 2).Until(eventA("a")) },
			shape: "match-until(event,event)", descContains: []string{"match-until(2:2,"}},
		{name: "repeat-2-2-b-until-g", java: "[2:2] b=SupportBean_B until g=SupportBean_G",
			build: func() PatternStream { return eventB("b").MatchUntil(2, 2).Until(eventG("g")) },
			shape: "match-until(event,event)", descContains: []string{"match-until(2:2,"}},
		{name: "repeat-max4-b-until-g", java: "[:4] b=SupportBean_B until g=SupportBean_G",
			build: func() PatternStream { return eventB("b").MatchUntil(0, 4).Until(eventG("g")) },
			shape: "match-until(event,event)", descContains: []string{"match-until(0:4,"}},
		{name: "repeat-min1-b-until-g", java: "[1:] b=SupportBean_B until g=SupportBean_G",
			build: func() PatternStream { return eventB("b").MatchUntil(1, 0).Until(eventG("g")) },
			shape: "match-until(event,event)", descContains: []string{"match-until(1:*,"}},
		{name: "repeat-1-2-b-until-a", java: "[1:2] b=SupportBean_B until a=SupportBean_A",
			build: func() PatternStream { return eventB("b").MatchUntil(1, 2).Until(eventA("a")) },
			shape: "match-until(event,event)", descContains: []string{"match-until(1:2,"}},
		{name: "followed-c-repeat-b-d", java: "c=SupportBean_C -> [2] b=SupportBean_B -> d=SupportBean_D",
			build: func() PatternStream { return eventC("c").Then(eventB("b").MatchUntil(2, 2)).Then(eventD("d")) },
			shape: "followed-by(followed-by(event,match-until(event)),event)"},
		{name: "every-until-d-b", java: "every (d=SupportBean_D until b=SupportBean_B)",
			build: func() PatternStream { return eventD("d").Until(eventB("b")).Every() },
			shape: "every(until(event,event))"},
		// Java normalizes (every d) until b and every d until b to one text;
		// the fluent forms share the same AST.
		{name: "until-every-d-b", java: "every d=SupportBean_D until b=SupportBean_B",
			build: func() PatternStream { return eventD("d").Every().Until(eventB("b")) },
			shape: "until(every(event),event)"},
		{name: "repeat-2-or-ab", java: "[2] (a=SupportBean_A or b=SupportBean_B)",
			build: func() PatternStream { return eventA("a").Or(eventB("b")).MatchUntil(2, 2) },
			shape: "match-until(or(event,event))", descContains: []string{"match-until(2:2,"}},
		// ESPER-339: every has precedence over the repetition in text form,
		// every [2] a == every ([2] a); until applies outside the every.
		{name: "every-repeat-2-a", java: "every [2] a=SupportBean_A",
			build: func() PatternStream { return eventA("a").MatchUntil(2, 2).Every() },
			shape: "every(match-until(event))"},
		{name: "until-every-repeat-2-a-d", java: "every [2] a=SupportBean_A until d=SupportBean_D",
			build: func() PatternStream { return eventA("a").MatchUntil(2, 2).Every().Until(eventD("d")) },
			shape: "until(every(match-until(event)),event)"},
		{name: "repeat-3-or-ab", java: "[3] (a=SupportBean_A or b=SupportBean_B)",
			build: func() PatternStream { return eventA("a").Or(eventB("b")).MatchUntil(3, 3) },
			shape: "match-until(or(event,event))", descContains: []string{"match-until(3:3,"}},
		{name: "until-until-ab-c", java: "(a=SupportBean_A until b=SupportBean_B) until c=SupportBean_C",
			build: func() PatternStream { return eventA("a").Until(eventB("b")).Until(eventC("c")) },
			shape: "until(until(event,event),event)"},

		// not combinations.
		{name: "and-b-not-d", java: "b=SupportBean_B and not d=SupportBean_D",
			build: func() PatternStream { return eventB("b").And(eventD("d").Not()) },
			shape: "and(event,not(event))"},
		{name: "and-every-b-not-g", java: "every b=SupportBean_B and not g=SupportBean_G",
			build: func() PatternStream { return eventB("b").Every().And(eventG("g").Not()) },
			shape: "and(every(event),not(event))"},
		{name: "and-b-not-a", java: "b=SupportBean_B and not a=SupportBean_A(id=\"A1\")",
			build: func() PatternStream { return eventB("b").And(PatternFrom(sa, "a", idA("A1")).Not()) },
			shape: "and(event,not(event))"},
		{name: "every-and-b-not-b3", java: "every (b=SupportBean_B and not b3=SupportBean_B(id=\"B3\"))",
			build: func() PatternStream { return eventB("b").And(PatternFrom(sbB, "b3", idB("B3")).Not()).Every() },
			shape: "every(and(event,not(event)))"},
		{name: "every-or-b-not-d", java: "every (b=SupportBean_B or not SupportBean_D)",
			build: func() PatternStream { return eventB("b").Or(eventD("d").Not()).Every() },
			shape: "every(or(event,not(event)))"},
		{name: "every-and-every-b-not-b", java: "every (every b=SupportBean_B and not SupportBean_B)",
			build: func() PatternStream { return eventB("b").Every().And(eventB("b2").Not()).Every() },
			shape: "every(and(every(event),not(event)))"},
		{name: "every-and-b-not-b", java: "every (b=SupportBean_B and not SupportBean_B)",
			build: func() PatternStream { return eventB("b").And(eventB("b2").Not()).Every() },
			shape: "every(and(event,not(event)))"},
		{name: "and-followed-bd-g", java: "(b=SupportBean_B -> d=SupportBean_D) and SupportBean_G",
			build: func() PatternStream { return eventB("b").Then(eventD("d")).And(eventG("g")) },
			shape: "and(followed-by(event,event),event)"},
		{name: "and-followed-bd-followed-ae", java: "(b=SupportBean_B -> d=SupportBean_D) and (a=SupportBean_A -> e=SupportBean_E)",
			build: func() PatternStream { return eventB("b").Then(eventD("d")).And(eventA("a").Then(eventE("e"))) },
			shape: "and(followed-by(event,event),followed-by(event,event))"},
		{name: "followed-b-or-d-a", java: "b=SupportBean_B -> (d=SupportBean_D() or a=SupportBean_A)",
			build: func() PatternStream { return eventB("b").Then(eventD("d").Or(eventA("a"))) },
			shape: "followed-by(event,or(event,event))"},
		{name: "followed-b-or-followed-da-followed-ae", java: "b=SupportBean_B -> ((d=SupportBean_D -> a=SupportBean_A) or (a=SupportBean_A -> e=SupportBean_E))",
			build: func() PatternStream {
				return eventB("b").Then(eventD("d").Then(eventA("a")).Or(eventA("a").Then(eventE("e"))))
			},
			shape: "followed-by(event,or(followed-by(event,event),followed-by(event,event)))"},
		{name: "or-followed-bd-a", java: "(b=SupportBean_B -> d=SupportBean_D) or a=SupportBean_A",
			build: func() PatternStream { return eventB("b").Then(eventD("d")).Or(eventA("a")) },
			shape: "or(followed-by(event,event),event)"},
		{name: "or-and-bd-a", java: "(b=SupportBean_B and d=SupportBean_D) or a=SupportBean_A",
			build: func() PatternStream { return eventB("b").And(eventD("d")).Or(eventA("a")) },
			shape: "or(and(event,event),event)"},
		{name: "or-a-a", java: "a=SupportBean_A or a=SupportBean_A",
			build: func() PatternStream { return eventA("a1").Or(eventA("a2")) },
			shape: "or(event,event)"},
		{name: "or-a-b-c", java: "a=SupportBean_A or b=SupportBean_B or c=SupportBean_C",
			build: func() PatternStream { return eventA("a").Or(eventB("b")).Or(eventC("c")) },
			shape: "or(or(event,event),event)"},
		{name: "or-every-b-every-d", java: "every b=SupportBean_B or every d=SupportBean_D",
			build: func() PatternStream { return eventB("b").Every().Or(eventD("d").Every()) },
			shape: "or(every(event),every(event))"},
		{name: "or-a-b", java: "a=SupportBean_A or b=SupportBean_B",
			build: func() PatternStream { return eventA("a").Or(eventB("b")) },
			shape: "or(event,event)"},
		{name: "or-a-every-b", java: "a=SupportBean_A or every b=SupportBean_B",
			build: func() PatternStream { return eventA("a").Or(eventB("b").Every()) },
			shape: "or(event,every(event))"},
		{name: "or-every-a-d", java: "every a=SupportBean_A or d=SupportBean_D",
			build: func() PatternStream { return eventA("a").Every().Or(eventD("d")) },
			shape: "or(every(event),event)"},
		{name: "every-or-every-b-d", java: "every (every b=SupportBean_B or d=SupportBean_D)",
			build: func() PatternStream { return eventB("b").Every().Or(eventD("d")).Every() },
			shape: "every(or(every(event),event))"},
		{name: "every-or-b-every-d", java: "every (b=SupportBean_B or every d=SupportBean_D)",
			build: func() PatternStream { return eventB("b").Or(eventD("d").Every()).Every() },
			shape: "every(or(event,every(event)))"},
		{name: "every-or-every-d-every-b", java: "every (every d=SupportBean_D or every b=SupportBean_B)",
			build: func() PatternStream { return eventD("d").Every().Or(eventB("b").Every()).Every() },
			shape: "every(or(every(event),every(event)))"},

		// while guard text forms (AST-level expression guard).
		{name: "followed-a-while-every-b", java: "a=SupportBean_A -> (every b=SupportBean_B) while (b.id!=\"B3\")",
			build: func() PatternStream {
				return eventA("a").Then(eventB("b").Every().WhileGuard(NotEqual[string](TagField[string]("b", "id"), Literal("B3"))))
			},
			shape: "followed-by(event,while-guard(every(event)))", descContains: []string{"while-guard("}},
		{name: "while-every-b", java: "(every b=SupportBean_B) while (b.id!=\"B1\")",
			build: func() PatternStream {
				return eventB("b").Every().WhileGuard(NotEqual[string](TagField[string]("b", "id"), Literal("B1")))
			},
			shape: "while-guard(every(event))", descContains: []string{"while-guard("}},

		// every-distinct text forms.
		{name: "distinct-key-expiry-like", java: "every-distinct(a.intPrimitive,1) a=SupportBean(theString like \"A%\")",
			build: func() PatternStream { return PatternFrom(sb, "a", likeA).EveryDistinctFor(time.Second, keyA) },
			shape: "every(event)", descContains: []string{"every-distinct(", ",1s,"}},
		{name: "distinct-untagged-key", java: "every-distinct(intPrimitive) a=SupportBean",
			build: func() PatternStream { return PatternFrom(sb, "a", Literal(true)).EveryDistinct(intF) },
			shape: "every(event)", descContains: []string{"every-distinct("}},
		{name: "repeat-2-distinct", java: "[2] every-distinct(a.intPrimitive) a=SupportBean",
			build: func() PatternStream { return PatternFrom(sb, "a", Literal(true)).EveryDistinct(keyA).MatchUntil(2, 2) },
			shape: "match-until(every(event))"},
		{name: "distinct-array-tag-repeat", java: "every-distinct(a[0].intPrimitive) ([2] a=SupportBean)",
			build: func() PatternStream {
				return PatternFrom(sb, "a", Literal(true)).MatchUntil(2, 2).EveryDistinct(TagFieldAt[int]("a", 0, "intPrimitive"))
			},
			shape: "every(match-until(event))"},
		{name: "distinct-array-tag-expiry-repeat", java: "every-distinct(a[0].intPrimitive,a[0].intPrimitive,1 hours) ([2] a=SupportBean)",
			build: func() PatternStream {
				return PatternFrom(sb, "a", Literal(true)).MatchUntil(2, 2).EveryDistinctFor(time.Hour, TagFieldAt[int]("a", 0, "intPrimitive"), TagFieldAt[int]("a", 0, "intPrimitive"))
			},
			shape: "every(match-until(event))", descContains: []string{",1h0m0s,"}},
		{name: "within-distinct", java: "(every-distinct(a.intPrimitive) a=SupportBean) where timer:within(10 seconds)",
			build: func() PatternStream {
				return PatternFrom(sb, "a", Literal(true)).EveryDistinct(keyA).Within(10 * time.Second)
			},
			shape: "within(every(event))"},
		{name: "distinct-within", java: "every-distinct(a.intPrimitive) a=SupportBean where timer:within(10)",
			build: func() PatternStream {
				return PatternFrom(sb, "a", Literal(true)).Within(10 * time.Second).EveryDistinct(keyA)
			},
			shape: "every(within(event))"},
		{name: "distinct-within-expiry", java: "every-distinct(a.intPrimitive,1 hours) a=SupportBean where timer:within(10)",
			build: func() PatternStream {
				return PatternFrom(sb, "a", Literal(true)).Within(10*time.Second).EveryDistinctFor(time.Hour, keyA)
			},
			shape: "every(within(event))", descContains: []string{",1h0m0s,"}},
		{name: "distinct-and-likes", java: "every-distinct(a.intPrimitive,b.intPrimitive) (a=SupportBean(theString like \"A%\") and b=SupportBean(theString like \"B%\"))",
			build: func() PatternStream {
				return PatternFrom(sb, "a", likeA).And(PatternFrom(sb, "b", likeB)).EveryDistinct(keyA, keyB)
			},
			shape: "every(and(event,event))"},
		{name: "distinct-and-not", java: "every-distinct(a.intPrimitive) (a=SupportBean and not SupportBean)",
			build: func() PatternStream {
				return PatternFrom(sb, "a", Literal(true)).And(event("n").Not()).EveryDistinct(keyA)
			},
			shape: "every(and(event,not(event)))"},
		{name: "distinct-and-not-expiry", java: "every-distinct(a.intPrimitive,1 hours) (a=SupportBean and not SupportBean)",
			build: func() PatternStream {
				return PatternFrom(sb, "a", Literal(true)).And(event("n").Not()).EveryDistinctFor(time.Hour, keyA)
			},
			shape: "every(and(event,not(event)))", descContains: []string{",1h0m0s,"}},
		{name: "distinct-sum-key-followed", java: "every-distinct(a.intPrimitive+b.intPrimitive,1 hours) (a=SupportBean -> b=SupportBean)",
			build: func() PatternStream {
				return event("a").Then(event("b")).EveryDistinctFor(time.Hour, Add[int](keyA, keyB))
			},
			shape: "every(followed-by(event,event))", descContains: []string{",1h0m0s,"}},
		{name: "distinct-followed-correlated", java: "every-distinct(a.intPrimitive) a=SupportBean -> b=SupportBean(intPrimitive=a.intPrimitive)",
			build: func() PatternStream {
				return PatternFrom(sb, "a", Literal(true)).EveryDistinct(keyA).Then(PatternFrom(sb, "b", Equal[int](intF, keyA)))
			},
			shape: "followed-by(every(event),event)"},
		{name: "distinct-followed-distinct", java: "every-distinct(a.intPrimitive) a=SupportBean -> every-distinct(b.intPrimitive) b=SupportBean(theString like \"B%\")",
			build: func() PatternStream {
				return PatternFrom(sb, "a", Literal(true)).EveryDistinct(keyA).Then(PatternFrom(sb, "b", likeB).EveryDistinct(keyB))
			},
			shape: "followed-by(every(event),every(event))"},
	}
	runPatternTextCases(t, env, tests)
}

// TestPatternExpressionTextTimersMatchesEsper covers the timer:at,
// timer:interval, timer:within and timer:withinmax text forms.
func TestPatternExpressionTextTimersMatchesEsper(t *testing.T) {
	env := newPatternTextEnv(t)
	sb := From[patternOpBean](env, "SupportBean")
	sa := From[patternNotA](env, "SupportBean_A")
	sbB := From[patternNotB](env, "SupportBean_B")
	sd := From[patternNotD](env, "SupportBean_D")
	sg := From[patternNotG](env, "SupportBean_G")

	eventB := func(tag string) PatternStream { return PatternFrom(sbB, tag, Literal(true)) }
	eventD := func(tag string) PatternStream { return PatternFrom(sd, tag, Literal(true)) }
	eventG := func(tag string) PatternStream { return PatternFrom(sg, tag, Literal(true)) }
	idB := func(value string) Expression[bool] {
		return Equal[string](Field[patternNotB, string]("id"), Literal(value))
	}
	cron := func(minute, hour, dom, month, weekday CronField) PatternStream {
		return TimerCron(sb, NewCronSchedule(minute, hour, dom, month, weekday))
	}
	cronSeconds := func(second, minute, hour, dom, month, weekday CronField) PatternStream {
		return TimerCron(sb, NewCronScheduleWithSeconds(second, minute, hour, dom, month, weekday))
	}
	wild := CronWildcard()
	interval := func(duration time.Duration) PatternStream { return TimerInterval(sb, duration) }

	tests := []patternTextCase{
		{name: "cron-5-field", java: "timer:at(10,8,*,*,*)",
			build: func() PatternStream { return cron(CronValues(10), CronValues(8), wild, wild, wild) },
			shape: "timer-cron", descContains: []string{"timer-cron("}},
		{name: "every-cron-step", java: "every timer:at(*/5,*,*,*,*,*)",
			build: func() PatternStream { return cronSeconds(wild, CronEvery(5), wild, wild, wild, wild).Every() },
			shape: "every(timer-cron)"},
		{name: "or-crons", java: "timer:at(10,9,*,*,*,10) or timer:at(30,9,*,*,*,*)",
			build: func() PatternStream {
				return cronSeconds(CronValues(10), CronValues(10), CronValues(9), wild, wild, wild).Or(cronSeconds(wild, CronValues(30), CronValues(9), wild, wild, wild))
			},
			shape: "or(timer-cron,timer-cron)"},
		{name: "followed-b-cron", java: "b=SupportBean_B(id=\"B3\") -> timer:at(20,9,*,*,*,*)",
			build: func() PatternStream {
				return PatternFrom(sbB, "b", idB("B3")).Then(cronSeconds(wild, CronValues(20), CronValues(9), wild, wild, wild))
			},
			shape: "followed-by(event,timer-cron)"},
		{name: "followed-cron-d", java: "timer:at(59,8,*,*,*,59) -> d=SupportBean_D",
			build: func() PatternStream {
				return cronSeconds(CronValues(59), CronValues(59), CronValues(8), wild, wild, wild).Then(eventD("d"))
			},
			shape: "followed-by(timer-cron,event)"},
		{name: "followed-cron-b-cron", java: "timer:at(22,8,*,*,*) -> b=SupportBean_B -> timer:at(55,*,*,*,*)",
			build: func() PatternStream {
				return cron(CronValues(22), CronValues(8), wild, wild, wild).Then(eventB("b")).Then(cron(CronValues(55), wild, wild, wild, wild))
			},
			shape: "followed-by(followed-by(timer-cron,event),timer-cron)"},
		{name: "and-cron-b", java: "timer:at(40,*,*,*,*,1) and b=SupportBean_B",
			build: func() PatternStream {
				return cronSeconds(CronValues(1), CronValues(40), wild, wild, wild, wild).And(eventB("b"))
			},
			shape: "and(timer-cron,event)"},
		{name: "or-cron-d", java: "timer:at(40,9,*,*,*,1) or d=SupportBean_D",
			build: func() PatternStream {
				return cronSeconds(CronValues(1), CronValues(40), CronValues(9), wild, wild, wild).Or(eventD("d"))
			},
			shape: "or(timer-cron,event)"},
		{name: "followed-cron-b-cron-hour", java: "timer:at(22,8,*,*,*) -> b=SupportBean_B -> timer:at(55,8,*,*,*)",
			build: func() PatternStream {
				return cron(CronValues(22), CronValues(8), wild, wild, wild).Then(eventB("b")).Then(cron(CronValues(55), CronValues(8), wild, wild, wild))
			},
			shape: "followed-by(followed-by(timer-cron,event),timer-cron)"},
		{name: "within-cron", java: "timer:at(22,8,*,*,*,1) where timer:within(30 minutes)",
			build: func() PatternStream {
				return cronSeconds(CronValues(1), CronValues(22), CronValues(8), wild, wild, wild).Within(30 * time.Minute)
			},
			shape: "within(timer-cron)", descContains: []string{"within(30m0s,"}},
		{name: "and-cron-cron", java: "timer:at(*,9,*,*,*) and timer:at(55,*,*,*,*)",
			build: func() PatternStream {
				return cron(wild, CronValues(9), wild, wild, wild).And(cron(CronValues(55), wild, wild, wild, wild))
			},
			shape: "and(timer-cron,timer-cron)"},
		{name: "and-cron8-b", java: "timer:at(40,8,*,*,*,1) and b=SupportBean_B",
			build: func() PatternStream {
				return cronSeconds(CronValues(1), CronValues(40), CronValues(8), wild, wild, wild).And(eventB("b"))
			},
			shape: "and(timer-cron,event)"},

		{name: "interval-2s", java: "timer:interval(2 seconds)",
			build: func() PatternStream { return interval(2 * time.Second) },
			shape: "timer-interval", descContains: []string{"timer-interval(2s)"}},
		{name: "interval-2001ms", java: "timer:interval(2.001)",
			build: func() PatternStream { return interval(2001 * time.Millisecond) },
			shape: "timer-interval", descContains: []string{"timer-interval(2.001s)"}},
		{name: "interval-2999ms", java: "timer:interval(2999 milliseconds)",
			build: func() PatternStream { return interval(2999 * time.Millisecond) },
			shape: "timer-interval", descContains: []string{"timer-interval(2.999s)"}},
		{name: "followed-interval-b", java: "timer:interval(4 seconds) -> b=SupportBean_B",
			build: func() PatternStream { return interval(4 * time.Second).Then(eventB("b")) },
			shape: "followed-by(timer-interval,event)"},
		{name: "followed-b-interval-zero", java: "b=SupportBean_B -> timer:interval(0)",
			build: func() PatternStream { return eventB("b").Then(interval(0)) },
			shape: "followed-by(event,timer-interval)", descContains: []string{"timer-interval(0s)"}},
		{name: "followed-b-interval-d", java: "b=SupportBean_B -> timer:interval(6.0) -> d=SupportBean_D",
			build: func() PatternStream { return eventB("b").Then(interval(6 * time.Second)).Then(eventD("d")) },
			shape: "followed-by(followed-by(event,timer-interval),event)"},
		{name: "every-followed-b-interval-d", java: "every (b=SupportBean_B -> timer:interval(2.0) -> d=SupportBean_D)",
			build: func() PatternStream { return eventB("b").Then(interval(2 * time.Second)).Then(eventD("d")).Every() },
			shape: "every(followed-by(followed-by(event,timer-interval),event))"},
		{name: "or-b-interval", java: "b=SupportBean_B or timer:interval(2.001)",
			build: func() PatternStream { return eventB("b").Or(interval(2001 * time.Millisecond)) },
			shape: "or(event,timer-interval)"},
		{name: "or-b-interval-8-5", java: "b=SupportBean_B or timer:interval(8.5)",
			build: func() PatternStream { return eventB("b").Or(interval(8500 * time.Millisecond)) },
			shape: "or(event,timer-interval)"},
		{name: "or-intervals", java: "timer:interval(8.5) or timer:interval(7.5)",
			build: func() PatternStream { return interval(8500 * time.Millisecond).Or(interval(7500 * time.Millisecond)) },
			shape: "or(timer-interval,timer-interval)"},
		{name: "or-interval-g", java: "timer:interval(999999 milliseconds) or g=SupportBean_G",
			build: func() PatternStream { return interval(999999 * time.Millisecond).Or(eventG("g")) },
			shape: "or(timer-interval,event)"},
		{name: "and-b-interval", java: "b=SupportBean_B and timer:interval(4000 milliseconds)",
			build: func() PatternStream { return eventB("b").And(interval(4 * time.Second)) },
			shape: "and(event,timer-interval)"},

		{name: "within-b", java: "b=SupportBean_B(id=\"B1\") where timer:within(2 seconds)",
			build: func() PatternStream { return PatternFrom(sbB, "b", idB("B1")).Within(2 * time.Second) },
			shape: "within(event)", descContains: []string{"within(2s,"}},
		{name: "within-every-b", java: "(every b=SupportBean_B) where timer:within(2.001)",
			build: func() PatternStream { return eventB("b").Every().Within(2001 * time.Millisecond) },
			shape: "within(every(event))", descContains: []string{"within(2.001s,"}},
		{name: "within-every-b-6s", java: "every (b=SupportBean_B) where timer:within(6.001)",
			build: func() PatternStream { return eventB("b").Every().Within(6001 * time.Millisecond) },
			shape: "within(every(event))", descContains: []string{"within(6.001s,"}},
		{name: "followed-b-within-d", java: "b=SupportBean_B -> d=SupportBean_D where timer:within(4001 milliseconds)",
			build: func() PatternStream { return eventB("b").Then(eventD("d").Within(4001 * time.Millisecond)) },
			shape: "followed-by(event,within(event))", descContains: []string{"within(4.001s,"}},
		{name: "followed-b-within-d-4s", java: "b=SupportBean_B -> d=SupportBean_D where timer:within(4 seconds)",
			build: func() PatternStream { return eventB("b").Then(eventD("d").Within(4 * time.Second)) },
			shape: "followed-by(event,within(event))"},
		{name: "every-and-withins", java: "every (b=SupportBean_B where timer:within(4.001) and d=SupportBean_D where timer:within(6.001))",
			build: func() PatternStream {
				return eventB("b").Within(4001 * time.Millisecond).And(eventD("d").Within(6001 * time.Millisecond)).Every()
			},
			shape: "every(and(within(event),within(event)))"},
		{name: "followed-every-b-within-d", java: "every b=SupportBean_B -> d=SupportBean_D where timer:within(4000 seconds)",
			build: func() PatternStream { return eventB("b").Every().Then(eventD("d").Within(4000 * time.Second)) },
			shape: "followed-by(every(event),within(event))"},
		{name: "followed-every-b-within-every-d", java: "every b=SupportBean_B -> every d=SupportBean_D where timer:within(4000 seconds)",
			build: func() PatternStream { return eventB("b").Every().Then(eventD("d").Every().Within(4000 * time.Second)) },
			shape: "followed-by(every(event),within(every(event)))"},
		{name: "followed-b-within-d-3999", java: "b=SupportBean_B -> d=SupportBean_D where timer:within(3999 seconds)",
			build: func() PatternStream { return eventB("b").Then(eventD("d").Within(3999 * time.Second)) },
			shape: "followed-by(event,within(event))"},
		{name: "followed-every-b-within-every-d-2001", java: "every b=SupportBean_B -> (every d=SupportBean_D) where timer:within(2001)",
			build: func() PatternStream {
				return eventB("b").Every().Then(eventD("d").Every().Within(2001 * time.Millisecond))
			},
			shape: "followed-by(every(event),within(every(event)))"},
		{name: "within-followed-bd", java: "every (b=SupportBean_B -> d=SupportBean_D) where timer:within(6001)",
			build: func() PatternStream { return eventB("b").Then(eventD("d")).Every().Within(6001 * time.Millisecond) },
			shape: "within(every(followed-by(event,event)))"},
		{name: "or-within-b-within-d", java: "b=SupportBean_B where timer:within(2000) or d=SupportBean_D where timer:within(6000)",
			build: func() PatternStream {
				return eventB("b").Within(2 * time.Second).Or(eventD("d").Within(6 * time.Second))
			},
			shape: "or(within(event),within(event))"},
		{name: "within-or-withins", java: "(b=SupportBean_B where timer:within(2000) or d=SupportBean_D where timer:within(6000)) where timer:within(1999)",
			build: func() PatternStream {
				return eventB("b").Within(2 * time.Second).Or(eventD("d").Within(6 * time.Second)).Within(1999 * time.Millisecond)
			},
			shape: "within(or(within(event),within(event)))", descContains: []string{"within(1.999s,"}},
		{name: "every-and-withins-2001", java: "every (b=SupportBean_B where timer:within(2001) and d=SupportBean_D where timer:within(6001))",
			build: func() PatternStream {
				return eventB("b").Within(2001 * time.Millisecond).And(eventD("d").Within(6001 * time.Millisecond)).Every()
			},
			shape: "every(and(within(event),within(event)))"},
		{name: "or-withins-2001", java: "b=SupportBean_B where timer:within(2001) or d=SupportBean_D where timer:within(6001)",
			build: func() PatternStream {
				return eventB("b").Within(2001 * time.Millisecond).Or(eventD("d").Within(6001 * time.Millisecond))
			},
			shape: "or(within(event),within(event))"},
		{name: "or-withins-untagged", java: "SupportBean_B where timer:within(2000) or d=SupportBean_D where timer:within(6001)",
			build: func() PatternStream {
				return eventB("b").Within(2 * time.Second).Or(eventD("d").Within(6001 * time.Millisecond))
			},
			shape: "or(within(event),within(event))"},
		{name: "and-within-every-b-within-every-d", java: "every b=SupportBean_B where timer:within(2001) and every d=SupportBean_D where timer:within(6001)",
			build: func() PatternStream {
				return eventB("b").Every().Within(2001 * time.Millisecond).And(eventD("d").Every().Within(6001 * time.Millisecond))
			},
			shape: "and(within(every(event)),within(every(event)))"},
		{name: "and-within-every-b-within-every-d-2000", java: "(every b=SupportBean_B) where timer:within(2000) and every d=SupportBean_D where timer:within(6001)",
			build: func() PatternStream {
				return eventB("b").Every().Within(2 * time.Second).And(eventD("d").Every().Within(6001 * time.Millisecond))
			},
			shape: "and(within(every(event)),within(every(event)))"},

		{name: "withinmax-b", java: "b=SupportBean_B(id=\"B1\") where timer:withinmax(2 seconds,100)",
			build: func() PatternStream { return PatternFrom(sbB, "b", idB("B1")).WithinOrMax(2*time.Second, 100) },
			shape: "within(event)", descContains: []string{"within-max(2s,100,"}},
		{name: "withinmax-every-b", java: "(every b=SupportBean_B) where timer:withinmax(4.001,2)",
			build: func() PatternStream { return eventB("b").Every().WithinOrMax(4001*time.Millisecond, 2) },
			shape: "within(every(event))", descContains: []string{"within-max(4.001s,2,"}},
		{name: "withinmax-every-b-2-001", java: "every b=SupportBean_B where timer:withinmax(2.001,4)",
			build: func() PatternStream { return eventB("b").Every().WithinOrMax(2001*time.Millisecond, 4) },
			shape: "within(every(event))", descContains: []string{"within-max(2.001s,4,"}},
		{name: "every-withinmax-b-zero", java: "every (b=SupportBean_B where timer:withinmax(2001,0))",
			build: func() PatternStream { return eventB("b").WithinOrMax(2001*time.Millisecond, 0).Every() },
			shape: "every(within(event))", descContains: []string{"within-max(2.001s,0,"}},
		{name: "followed-every-b-withinmax-d", java: "every b=SupportBean_B -> d=SupportBean_D where timer:withinmax(4000 milliseconds,1)",
			build: func() PatternStream { return eventB("b").Every().Then(eventD("d").WithinOrMax(4*time.Second, 1)) },
			shape: "followed-by(every(event),within(event))", descContains: []string{"within-max(4s,1,"}},
		{name: "followed-every-b-withinmax-every-d", java: "every b=SupportBean_B -> every d=SupportBean_D where timer:withinmax(4000,1)",
			build: func() PatternStream {
				return eventB("b").Every().Then(eventD("d").Every().WithinOrMax(4000*time.Second, 1))
			},
			shape: "followed-by(every(event),within(every(event)))"},
		{name: "followed-every-b-withinmax-every-d-days", java: "every b=SupportBean_B -> (every d=SupportBean_D) where timer:withinmax(1 days,3)",
			build: func() PatternStream {
				return eventB("b").Every().Then(eventD("d").Every().WithinOrMax(24*time.Hour, 3))
			},
			shape: "followed-by(every(event),within(every(event)))", descContains: []string{"within-max(24h0m0s,3,"}},

		{name: "until-d-interval", java: "d=SupportBean_D until timer:interval(7 sec)",
			build: func() PatternStream { return eventD("d").Until(interval(7 * time.Second)) },
			shape: "until(event,timer-interval)", descContains: []string{"timer-interval(7s)"}},
		{name: "until-a-every-interval-not-a", java: "a=SupportBean_A until (every (timer:interval(6 sec) and not SupportBean_A))",
			build: func() PatternStream {
				return PatternFrom(sa, "a", Literal(true)).Until(interval(6 * time.Second).And(PatternFrom(sa, "na", Literal(true)).Not()).Every())
			},
			shape: "until(event,every(and(timer-interval,not(event))))"},
	}
	runPatternTextCases(t, env, tests)
}
