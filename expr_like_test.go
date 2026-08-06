package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type likeRegexParityEvent struct {
	Text         string   `esper:"text"`
	Pattern      string   `esper:"pattern"`
	Escape       string   `esper:"escape"`
	RegexText    string   `esper:"regex_text"`
	RegexPattern string   `esper:"regex_pattern"`
	IntValue     int      `esper:"int_value"`
	DoubleValue  *float64 `esper:"double_value"`
	Flag         bool     `esper:"flag"`
}

func TestLikeAndRegexpExpressionsMatchJavaWildcardNumericEscapeAndNullSemantics(t *testing.T) {
	if got := LikeOf(Literal("Ayyy"), Literal("A%")).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("like wildcard = %v, want true", got)
	}
	if got := LikeOf(Literal(100), Literal("1%")).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("numeric like = %v, want true", got)
	}
	if got := RegexpMatchOf(Literal(11.0), Literal("[0-9][0-9].[0-9]")).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("numeric regexp = %v, want true", got)
	}
	if got := LikeWithEscape(Literal("%dex"), Literal("!%de_"), Literal("!")).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("escaped like = %v, want true", got)
	}
	if got := LikeWithEscape(Literal("!adex"), Literal("!%de_"), Literal("!")).eval(EvalContext{}); !got.Equal(Present(false)) {
		t.Fatalf("escaped like non-match = %v, want false", got)
	}
	if got := RegexpMatchOf(Literal("TBT-ABC"), Literal(`\w*-ABC`)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("escaped regexp = %v, want true", got)
	}
	if got := LikeOf(NullLiteral[string](), Literal("x%")).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("null like = %v, want null", got)
	}
	if got := RegexpMatchOf(Literal("x"), NullLiteral[string]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("null regexp pattern = %v, want null", got)
	}
	if got := LikeWithEscape(Literal("abc"), Literal("abc"), Literal("!!")).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("invalid escape width = %v, want null", got)
	}
	if got := RegexpMatchOf(Literal("x"), Literal("[")).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("invalid regexp = %v, want null", got)
	}
}

func TestLikeAndRegexpExpressionsBuildLiveProjectionAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[likeRegexParityEvent](env, "LikeRegexParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[likeRegexParityEvent](env, "LikeRegexParityEvent")
	textValue := Field[likeRegexParityEvent, string]("text")
	patternValue := Field[likeRegexParityEvent, string]("pattern")
	escapeValue := Field[likeRegexParityEvent, string]("escape")
	regexText := Field[likeRegexParityEvent, string]("regex_text")
	regexPattern := Field[likeRegexParityEvent, string]("regex_pattern")
	intValue := Field[likeRegexParityEvent, int]("int_value")
	doubleValue := Field[likeRegexParityEvent, *float64]("double_value")
	query := Select(input,
		Alias("like", LikeOf(textValue, patternValue)),
		Alias("like_escape", LikeOfWithEscape(textValue, patternValue, escapeValue)),
		Alias("regexp", RegexpMatchOf(regexText, regexPattern)),
		Alias("numeric_like", LikeOf(intValue, Literal("1%"))),
		Alias("numeric_regexp", RegexpMatchOf(doubleValue, Literal("[0-9][0-9].[0-9]"))),
	).Query(StatementName("like-regex-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent like/regexp plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	different, err := env.Build(Select(input,
		Alias("like", LikeOf(textValue, patternValue)),
		Alias("like_escape", LikeOfWithEscape(textValue, patternValue, Literal("#"))),
		Alias("regexp", RegexpMatchOf(regexText, regexPattern)),
		Alias("numeric_like", LikeOf(intValue, Literal("1%"))),
		Alias("numeric_regexp", RegexpMatchOf(doubleValue, Literal("[0-9][0-9].[0-9]"))),
	).Query(StatementName("like-regex-parity")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == different.Hash() {
		t.Fatal("LIKE escape did not enter Plan identity")
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("like/regexp result schema is missing")
	}
	for _, name := range []string{"like", "like_escape", "regexp", "numeric_like", "numeric_regexp"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != typeOf[bool]() {
			t.Fatalf("like/regexp result %q = %#v, want bool", name, field)
		}
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 2)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "like/regexp result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	doubleNumber := 11.0
	if err := engine.SendEvent(context.Background(), likeRegexParityEvent{
		Text: "abcdef", Pattern: "%de_", Escape: "!", RegexText: "TBT-ABC", RegexPattern: `\w*-ABC`, IntValue: 100, DoubleValue: &doubleNumber,
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), likeRegexParityEvent{
		Text: "%dex", Pattern: "!%de_", Escape: "!", RegexText: "TBT-BC", RegexPattern: `\w*-ABC`, IntValue: 0, DoubleValue: nil,
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("like/regexp rows = %d, want 2", len(rows))
	}
	for name, expected := range map[string]Value{
		"like": Present(true), "like_escape": Present(true), "regexp": Present(true),
		"numeric_like": Present(true), "numeric_regexp": Present(true),
	} {
		if !rows[0].Get(name).Equal(expected) {
			t.Fatalf("like/regexp first row %s = %v, want %v", name, rows[0].Get(name), expected)
		}
	}
	if !rows[1].Get("like").Equal(Present(false)) || !rows[1].Get("like_escape").Equal(Present(true)) || !rows[1].Get("regexp").Equal(Present(false)) || !rows[1].Get("numeric_like").Equal(Present(false)) || !rows[1].Get("numeric_regexp").IsNull() {
		t.Fatalf("like/regexp second row = %#v", rows[1])
	}
}

func TestLikeAndRegexpExpressionsRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[likeRegexParityEvent](env, "LikeRegexInvalid"); err != nil {
		t.Fatal(err)
	}
	input := From[likeRegexParityEvent](env, "LikeRegexInvalid")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "like-bool", expr: LikeOf(Literal(true), Literal("x")), want: "text-compatible"},
		{name: "regexp-bool", expr: RegexpMatchOf(Literal("x"), Literal(true)), want: "text-compatible"},
		{name: "like-nil-pattern", expr: LikeOf(Literal("x"), nil), want: "pattern expression is required"},
		{name: "like-null-escape", expr: LikeWithEscape(Literal("x"), Literal("x"), NullLiteral[string]()), want: "must not be null"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("like-regex-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid like/regexp error = %v, want %q", err, testCase.want)
			}
		})
	}
}
