package esper

import (
	"context"
	"fmt"
	"testing"
)

// exprFilterEnumValue is the Go representation of SupportEnum. The typed
// string keeps enum comparisons explicit while avoiding Java class literals.
type exprFilterEnumValue string

const (
	exprFilterEnumValueOne exprFilterEnumValue = "ENUM_VALUE_1"
	exprFilterEnumValueTwo exprFilterEnumValue = "ENUM_VALUE_2"
)

type exprFilterEnumBean struct {
	TheString   string              `esper:"theString"`
	SupportEnum exprFilterEnumValue `esper:"supportEnum"`
}

// TestExprFilterEnumSyntaxMatchesEsper covers the two Java enum filter
// executions. A Go enum value is a typed constant, so valueOf("...") and a
// direct enum member are represented by the same fluent Literal expression.
func TestExprFilterEnumSyntaxMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprFilterEnumBean](env, "SupportBeanWithEnum"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[exprFilterEnumBean](env, "SupportBeanWithEnum")
	t.Run("pattern-valueOf", func(t *testing.T) {
		pattern := PatternFrom(source, "event", Equal[exprFilterEnumValue](
			Field[exprFilterEnumBean, exprFilterEnumValue]("supportEnum"),
			Literal(exprFilterEnumValueOne),
		))
		plan, err := env.Build(pattern.Select(Alias("matched", Literal(true))).Query(StatementName("enum-pattern")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		matches := 0
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			matches += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), exprFilterEnumBean{TheString: "e1", SupportEnum: exprFilterEnumValueTwo}); err != nil {
			t.Fatal(err)
		}
		if matches != 0 {
			t.Fatalf("ENUM_VALUE_2 matches = %d, want 0", matches)
		}
		if err := engine.SendEvent(context.Background(), exprFilterEnumBean{TheString: "e1", SupportEnum: exprFilterEnumValueOne}); err != nil {
			t.Fatal(err)
		}
		if matches != 1 {
			t.Fatalf("ENUM_VALUE_1 matches = %d, want 1", matches)
		}
	})

	t.Run("pattern-and-where", func(t *testing.T) {
		pattern := PatternFrom(source, "event", Equal[exprFilterEnumValue](
			Field[exprFilterEnumBean, exprFilterEnumValue]("supportEnum"),
			Literal(exprFilterEnumValueTwo),
		))
		plan, err := env.Build(pattern.Select(Alias("matched", Literal(true))).Query(StatementName("enum-pattern-two")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		matches := 0
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			matches += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), exprFilterEnumBean{TheString: "e1", SupportEnum: exprFilterEnumValueTwo}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), exprFilterEnumBean{TheString: "e2", SupportEnum: exprFilterEnumValueOne}); err != nil {
			t.Fatal(err)
		}
		if matches != 1 {
			t.Fatalf("pattern ENUM_VALUE_2 matches = %d, want 1", matches)
		}

		if err := deployment.Undeploy(context.Background()); err != nil {
			t.Fatal(err)
		}
		wherePlan, err := env.Build(source.Filter(Equal[exprFilterEnumValue](
			Field[exprFilterEnumBean, exprFilterEnumValue]("supportEnum"),
			Literal(exprFilterEnumValueTwo),
		)).Query(StatementName("enum-where")))
		if err != nil {
			t.Fatal(err)
		}
		whereDeployment, err := engine.Deploy(context.Background(), wherePlan)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = whereDeployment.Undeploy(context.Background()) }()
		whereMatches := 0
		if _, err := whereDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			whereMatches += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), exprFilterEnumBean{TheString: "e1", SupportEnum: exprFilterEnumValueTwo}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), exprFilterEnumBean{TheString: "e2", SupportEnum: exprFilterEnumValueOne}); err != nil {
			t.Fatal(err)
		}
		if whereMatches != 1 {
			t.Fatalf("where ENUM_VALUE_2 matches = %d, want 1", whereMatches)
		}
	})
}

type exprFilterPatternPairCase struct {
	name      string
	predicate func(Stream[filterTestBean]) Expression[bool]
	first     filterTestBean
	second    filterTestBean
	want      bool
}

func runExprFilterPatternPair(t *testing.T, testCase exprFilterPatternPairCase) {
	t.Helper()
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[filterTestBean](env, "SupportBean")
	pattern := PatternFrom(source, "a", Literal(true)).Then(
		PatternFrom(source, "b", testCase.predicate(source)),
	)
	plan, err := env.Build(pattern.Select(Alias("matched", Literal(true))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	matches := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		matches += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), testCase.first); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), testCase.second); err != nil {
		t.Fatal(err)
	}
	if got := matches > 0; got != testCase.want {
		t.Fatalf("matches = %v, want %v (count=%d)", got, testCase.want, matches)
	}
}

// TestExprFilterPatternFuncMatchesEsper mirrors all predicate forms in
// ExprFilterPatternFunc, including correlated equality, null-safe equality,
// open/closed ranges, mixed IN candidates and arithmetic correlation.
func TestExprFilterPatternFuncMatchesEsper(t *testing.T) {
	currentInt := func(s Stream[filterTestBean]) Expression[*int] {
		return Field[filterTestBean, *int]("intBoxed")
	}
	currentDouble := func(s Stream[filterTestBean]) Expression[*float64] {
		return Field[filterTestBean, *float64]("doubleBoxed")
	}
	aInt := func() Expression[*int] { return TagField[*int]("a", "intBoxed") }
	aDouble := func() Expression[*float64] { return TagField[*float64]("a", "doubleBoxed") }
	patternPair := func(predicate func(Stream[filterTestBean]) Expression[bool], first, second filterTestBean, want bool) exprFilterPatternPairCase {
		return exprFilterPatternPairCase{predicate: predicate, first: first, second: second, want: want}
	}

	commonA := []filterTestBean{
		{IntBoxed: nil, DoubleBoxed: floatPtr(2)},
		{IntBoxed: intPtr(2), DoubleBoxed: floatPtr(2)},
		{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(2)},
		{IntBoxed: nil, DoubleBoxed: floatPtr(1)},
		{IntBoxed: intPtr(8), DoubleBoxed: floatPtr(5)},
		{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(6)},
		{IntBoxed: intPtr(2), DoubleBoxed: floatPtr(7)},
	}
	commonB := []filterTestBean{
		{IntBoxed: nil, DoubleBoxed: floatPtr(2)},
		{IntBoxed: intPtr(3), DoubleBoxed: floatPtr(3)},
		{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(2)},
		{IntBoxed: intPtr(8), DoubleBoxed: floatPtr(1)},
		{IntBoxed: nil, DoubleBoxed: floatPtr(5)},
		{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(6)},
		{IntBoxed: intPtr(2), DoubleBoxed: floatPtr(8)},
	}

	cases := []exprFilterPatternPairCase{
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return And(EqualOf(currentInt(s), aInt()), EqualOf(currentDouble(s), aDouble()))
		}, commonA[0], commonB[0], false),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return And(EqualOf(currentInt(s), aInt()), EqualOf(currentDouble(s), aDouble()))
		}, commonA[1], commonB[1], false),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return And(EqualOf(currentInt(s), aInt()), EqualOf(currentDouble(s), aDouble()))
		}, commonA[2], commonB[2], true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return And(EqualOf(currentInt(s), aInt()), EqualOf(currentDouble(s), aDouble()))
		}, commonA[3], commonB[3], false),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return And(EqualOf(currentInt(s), aInt()), EqualOf(currentDouble(s), aDouble()))
		}, commonA[4], commonB[4], false),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return And(EqualOf(currentInt(s), aInt()), EqualOf(currentDouble(s), aDouble()))
		}, commonA[5], commonB[5], true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return And(EqualOf(currentInt(s), aInt()), EqualOf(currentDouble(s), aDouble()))
		}, commonA[6], commonB[6], false),

		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return And(Is(currentInt(s), aInt()), EqualOf(currentDouble(s), aDouble()))
		}, commonA[0], commonB[0], true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return And(Is(currentInt(s), aInt()), EqualOf(currentDouble(s), aDouble()))
		}, commonA[1], commonB[1], false),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return And(Is(currentInt(s), aInt()), EqualOf(currentDouble(s), aDouble()))
		}, commonA[2], commonB[2], true),

		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return EqualOf(aDouble(), currentDouble(s))
		}, filterTestBean{DoubleBoxed: floatPtr(2)}, filterTestBean{DoubleBoxed: floatPtr(2)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return EqualOf(aDouble(), currentDouble(s))
		}, filterTestBean{DoubleBoxed: floatPtr(2)}, filterTestBean{DoubleBoxed: floatPtr(3)}, false),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return NotEqualOf(aDouble(), currentDouble(s))
		}, filterTestBean{DoubleBoxed: floatPtr(2)}, filterTestBean{DoubleBoxed: floatPtr(2)}, false),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return NotEqualOf(aDouble(), currentDouble(s))
		}, filterTestBean{DoubleBoxed: floatPtr(2)}, filterTestBean{DoubleBoxed: floatPtr(3)}, true),

		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return BetweenRangeOf(currentDouble(s), aDouble(), aInt(), true, true)
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(0)}, false),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return BetweenRangeOf(currentDouble(s), aDouble(), aInt(), true, true)
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(1)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return BetweenRangeOf(currentDouble(s), aDouble(), aInt(), true, true)
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(10)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return BetweenRangeOf(currentDouble(s), aDouble(), aInt(), false, true)
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(1)}, false),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return BetweenRangeOf(currentDouble(s), aDouble(), aInt(), false, true)
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(2)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return BetweenRangeOf(currentDouble(s), aDouble(), aInt(), false, false)
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(10)}, false),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return BetweenRangeOf(currentDouble(s), aDouble(), aInt(), true, false)
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(1)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return NotBetweenRangeOf(currentDouble(s), aDouble(), aInt(), true, true)
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(0)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return NotBetweenRangeOf(currentDouble(s), aDouble(), aInt(), false, true)
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(1)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return NotBetweenRangeOf(currentDouble(s), aDouble(), aInt(), false, false)
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(10)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return NotBetweenRangeOf(currentDouble(s), aDouble(), aInt(), true, false)
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(1)}, false),

		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return NotInOf(currentDouble(s), aDouble(), aInt(), Literal(9))
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(0)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return NotInOf(currentDouble(s), aDouble(), aInt(), Literal(9))
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(1)}, false),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return NotInOf(currentDouble(s), aDouble(), Literal(10), aInt())
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(2)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return InOf(currentDouble(s), aDouble(), aInt(), Literal(9))
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(9)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return InOf(currentDouble(s), currentDouble(s), aInt(), Literal(9))
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(11)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return NotInOf(currentDouble(s), currentDouble(s), aInt(), Literal(9))
		}, filterTestBean{IntBoxed: intPtr(1), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(11)}, false),

		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return EqualOf(currentDouble(s), Subtract[float64](Cast[*float64, float64](aDouble()), Literal(float64(1))))
		}, filterTestBean{DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(9)}, true),
		patternPair(func(s Stream[filterTestBean]) Expression[bool] {
			return Or(
				EqualOf(currentDouble(s), Subtract[float64](Cast[*float64, float64](aDouble()), Literal(float64(1)))),
				EqualOf(currentDouble(s), Subtract[float64](Cast[*int, float64](aInt()), Literal(float64(1)))),
			)
		}, filterTestBean{IntBoxed: intPtr(12), DoubleBoxed: floatPtr(10)}, filterTestBean{DoubleBoxed: floatPtr(11)}, true),
	}

	for index := range cases {
		cases[index].name = fmt.Sprintf("pattern-case-%d", index)
		t.Run(cases[index].name, func(t *testing.T) {
			runExprFilterPatternPair(t, cases[index])
		})
	}
}

type exprFilterPatternTripleCase struct {
	name      string
	predicate func(Stream[filterTestBean]) Expression[bool]
	a         filterTestBean
	b         filterTestBean
	c         filterTestBean
	want      bool
}

func runExprFilterPatternTriple(t *testing.T, testCase exprFilterPatternTripleCase) {
	t.Helper()
	env := newFilterTestEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	source := From[filterTestBean](env, "SupportBean")
	pattern := PatternFrom(source, "a", Literal(true)).Then(
		PatternFrom(source, "b", Literal(true)).Then(
			PatternFrom(source, "c", testCase.predicate(source)),
		),
	)
	plan, err := env.Build(pattern.Select(Alias("matched", Literal(true))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	matches := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		matches += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []filterTestBean{testCase.a, testCase.b, testCase.c} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if got := matches > 0; got != testCase.want {
		t.Fatalf("matches = %v, want %v (count=%d)", got, testCase.want, matches)
	}
}

// TestExprFilterPatternFunc3StreamMatchesEsper covers the three-event
// correlated filter matrix, including null-safe equality, OR, IN and range
// normalization across the a/b/c tags.
func TestExprFilterPatternFunc3StreamMatchesEsper(t *testing.T) {
	ci := func(s Stream[filterTestBean]) Expression[*int] {
		return Field[filterTestBean, *int]("intBoxed")
	}
	cd := func(s Stream[filterTestBean]) Expression[*float64] {
		return Field[filterTestBean, *float64]("doubleBoxed")
	}
	ai := func() Expression[*int] { return TagField[*int]("a", "intBoxed") }
	bi := func() Expression[*int] { return TagField[*int]("b", "intBoxed") }
	bd := func() Expression[*float64] { return TagField[*float64]("b", "doubleBoxed") }

	baseA := []filterTestBean{{}, {IntBoxed: intPtr(2)}, {IntBoxed: intPtr(1)}, {}, {IntBoxed: intPtr(8)}, {IntBoxed: intPtr(1)}, {IntBoxed: intPtr(2)}}
	baseB := []filterTestBean{{}, {IntBoxed: intPtr(3)}, {IntBoxed: intPtr(1)}, {IntBoxed: intPtr(8)}, {}, {IntBoxed: intPtr(4)}, {IntBoxed: intPtr(-2)}}
	baseC := []filterTestBean{{}, {IntBoxed: intPtr(3)}, {IntBoxed: intPtr(1)}, {IntBoxed: intPtr(8)}, {}, {IntBoxed: intPtr(5)}, {}}

	cases := []exprFilterPatternTripleCase{
		{name: "equal-and-not-null", predicate: func(s Stream[filterTestBean]) Expression[bool] {
			return And(EqualOf(ci(s), ai()), And(EqualOf(ci(s), bi()), NotEqualOf(ci(s), NullLiteral[*int]())))
		}, a: baseA[2], b: baseB[2], c: baseC[2], want: false},
		{name: "is-and-not-null", predicate: func(s Stream[filterTestBean]) Expression[bool] {
			return And(Is(ci(s), ai()), And(Is(ci(s), bi()), Not(IsNull[*int](ci(s)))))
		}, a: baseA[2], b: baseB[2], c: baseC[2], want: true},
		{name: "or", predicate: func(s Stream[filterTestBean]) Expression[bool] {
			return Or(EqualOf(ci(s), ai()), EqualOf(ci(s), bi()))
		}, a: baseA[1], b: baseB[1], c: baseC[1], want: true},
		{name: "equal-both", predicate: func(s Stream[filterTestBean]) Expression[bool] {
			return And(EqualOf(ci(s), ai()), EqualOf(ci(s), bi()))
		}, a: baseA[2], b: baseB[2], c: baseC[2], want: true},
		{name: "not-equal-both", predicate: func(s Stream[filterTestBean]) Expression[bool] {
			return And(NotEqualOf(ci(s), ai()), NotEqualOf(ci(s), bi()))
		}, a: baseA[5], b: baseB[5], c: baseC[5], want: true},
		{name: "not-equal-a", predicate: func(s Stream[filterTestBean]) Expression[bool] {
			return NotEqualOf(ci(s), ai())
		}, a: baseA[0], b: baseB[0], c: baseC[0], want: false},
		{name: "is-not-a", predicate: func(s Stream[filterTestBean]) Expression[bool] {
			return IsNot(ci(s), ai())
		}, a: filterTestBean{IntBoxed: intPtr(2)}, b: filterTestBean{IntBoxed: intPtr(-2)}, c: filterTestBean{}, want: true},
		{name: "int-a-double-b", predicate: func(s Stream[filterTestBean]) Expression[bool] {
			return And(EqualOf(ci(s), ai()), EqualOf(cd(s), bd()))
		}, a: filterTestBean{IntBoxed: intPtr(2)}, b: filterTestBean{DoubleBoxed: floatPtr(1)}, c: filterTestBean{IntBoxed: intPtr(2), DoubleBoxed: floatPtr(1)}, want: true},
		{name: "in", predicate: func(s Stream[filterTestBean]) Expression[bool] {
			return InOf(ci(s), ai(), bi())
		}, a: filterTestBean{IntBoxed: intPtr(2)}, b: filterTestBean{IntBoxed: intPtr(1)}, c: filterTestBean{IntBoxed: intPtr(2)}, want: true},
		{name: "closed-range", predicate: func(s Stream[filterTestBean]) Expression[bool] {
			return BetweenRangeOf(ci(s), ai(), bi(), true, true)
		}, a: filterTestBean{IntBoxed: intPtr(2)}, b: filterTestBean{IntBoxed: intPtr(1)}, c: filterTestBean{IntBoxed: intPtr(1)}, want: true},
		{name: "not-closed-range", predicate: func(s Stream[filterTestBean]) Expression[bool] {
			return NotBetweenRangeOf(ci(s), ai(), bi(), true, true)
		}, a: filterTestBean{IntBoxed: intPtr(2)}, b: filterTestBean{IntBoxed: intPtr(1)}, c: filterTestBean{IntBoxed: intPtr(3)}, want: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) { runExprFilterPatternTriple(t, testCase) })
	}
}

type exprFilterPatternLongBean struct {
	LongBoxed *int64 `esper:"longBoxed"`
}

type exprFilterPatternMarketData struct {
	Volume int64 `esper:"volume"`
}

// TestExprFilterPatternWithExprMatchesEsper covers an expression on the
// current b-event correlated with a.longBoxed in both operand orders.
func TestExprFilterPatternWithExprMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprFilterPatternLongBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[exprFilterPatternMarketData](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	bean := From[exprFilterPatternLongBean](env, "SupportBean")
	market := From[exprFilterPatternMarketData](env, "SupportMarketDataBean")
	longValue := TagField[*int64]("a", "longBoxed")
	volumeTwice := Multiply[int64](Field[exprFilterPatternMarketData, int64]("volume"), Literal[int64](2))
	cases := []struct {
		name      string
		predicate Expression[bool]
	}{
		{name: "left-tag", predicate: EqualOf(longValue, volumeTwice)},
		{name: "right-tag", predicate: EqualOf(volumeTwice, longValue)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			pattern := PatternFrom(bean, "a", Literal(true)).Every().Then(
				PatternFrom(market, "b", testCase.predicate),
			)
			plan, err := env.Build(pattern.Select(Alias("matched", Literal(true))).Query(StatementName(testCase.name)))
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			matches := 0
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				matches += len(batch.New)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			sendBean := func(value int64) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), exprFilterPatternLongBean{LongBoxed: &value}); err != nil {
					t.Fatal(err)
				}
			}
			sendMarket := func(volume int64) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), exprFilterPatternMarketData{Volume: volume}); err != nil {
					t.Fatal(err)
				}
			}
			sendBean(10)
			sendMarket(0)
			sendMarket(5)
			sendBean(0)
			sendMarket(0)
			sendMarket(1)
			sendBean(20)
			sendMarket(10)
			if matches != 3 {
				t.Fatalf("matches = %d, want 3", matches)
			}
			if err := deployment.Undeploy(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
