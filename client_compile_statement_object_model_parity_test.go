package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

type clientCompileStatementModelEvent struct {
	Line    int  `esper:"line"`
	Age     int  `esper:"age"`
	WaverID *int `esper:"waverId"`
}

type clientCompileStatementModelReady struct {
	Line   int     `esper:"line"`
	AvgAge float64 `esper:"avgAge"`
}

func newClientCompileStatementModelEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompileStatementModelEvent](env, "ClientCompileStatementModelEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[clientCompileStatementModelReady](env, "ReadyStreamAvg"); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestClientCompileSODACreateFromOMMatchesEsper(t *testing.T) {
	env := newClientCompileStatementModelEnvironment(t)
	model := From[clientCompileStatementModelEvent](env, "ClientCompileStatementModelEvent").Query(StatementName("s0"))
	plan, err := env.Build(model)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Query().Name() != "s0" || plan.Query().TypedDescription() != model.TypedDescription() {
		t.Fatalf("compiled object model = name %q description %q", plan.Query().Name(), plan.Query().TypedDescription())
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var received any
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) != 1 {
			t.Fatalf("object-model listener batch = %#v", batch)
		}
		event, ok := batch.New[0].Event()
		if !ok {
			t.Fatalf("object-model result is not an event: %#v", batch.New[0])
		}
		received = event.Underlying()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	theEvent := clientCompileStatementModelEvent{Line: 1, Age: 10}
	if err := engine.Send(context.Background(), "ClientCompileStatementModelEvent", theEvent); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(received, theEvent) {
		t.Fatalf("object-model underlying = %#v, want %#v", received, theEvent)
	}
}

func TestClientCompileSODACreateFromOMCompleteMatchesEsper(t *testing.T) {
	env := newClientCompileStatementModelEnvironment(t)
	line := Field[clientCompileStatementModelEvent, int]("line")
	age := Field[clientCompileStatementModelEvent, int]("age")
	waverID := Field[clientCompileStatementModelEvent, *int]("waverId")
	averageAge := Avg[int](age)

	model := From[clientCompileStatementModelEvent](env, "ClientCompileStatementModelEvent").
		Filter(In[int](line, Literal(1), Literal(8), Literal(10))).
		Window(TimeWindow(10*time.Second)).
		GroupBy(line).
		Where(Not(IsNull[*int](waverID))).
		Select(
			Alias("line", line),
			Alias("avgAge", averageAge),
		).
		Having(Less[float64](averageAge, Literal(0.0))).
		InsertInto(
			"ReadyStreamAvg",
			WithOutput(OutputEveryTime(10*time.Second)),
			OrderBy(Ascending(line)),
		)
	plan, err := env.Build(model)
	if err != nil {
		t.Fatal(err)
	}
	description := model.TypedDescription()
	for _, fragment := range []string{
		"ClientCompileStatementModelEvent",
		"line in (1,8,10)",
		"time(10s)",
		"group-by(line)",
		"where((not (waverId is null)))",
		"having((avg(age) < 0))",
		"output(every-time(10s))",
		"order-by(line:asc)",
	} {
		if !strings.Contains(description, fragment) {
			t.Fatalf("complete object-model description %q does not contain %q", description, fragment)
		}
	}
	if plan.Hash() == "" || !strings.Contains(string(plan.Canonical()), "route(ReadyStreamAvg)") || plan.Query().TypedDescription() != description {
		t.Fatalf("complete object-model plan = hash %q canonical %q description %q", plan.Hash(), plan.Canonical(), plan.Query().TypedDescription())
	}
}

func TestClientCompileSODAEPLToOMToStmtMatchesEsper(t *testing.T) {
	env := newClientCompileStatementModelEnvironment(t)
	model := From[clientCompileStatementModelEvent](env, "ClientCompileStatementModelEvent").Query(StatementName("s0"))
	first, err := env.Build(model)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := env.Build(first.Query())
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash() != rebuilt.Hash() || string(first.Canonical()) != string(rebuilt.Canonical()) {
		t.Fatalf("typed model round-trip changed plan:\nfirst  %s\nsecond %s", first.Canonical(), rebuilt.Canonical())
	}
	if rebuilt.Query().Metadata().Name != "s0" {
		t.Fatalf("round-trip statement metadata = %#v", rebuilt.Query().Metadata())
	}
}

func TestClientCompileSODAPrecedenceExpressionsMatchesEsper(t *testing.T) {
	tests := []struct {
		name       string
		expression Expr
		want       string
	}{
		{name: "division before multiplication", expression: DivideFloat(Literal(7.29), Multiply[int](Literal(3), Literal(100))), want: "divide(literal,multiply(literal,literal))"},
		{name: "addition with multiplication", expression: Add[int](Literal(1), Multiply[int](Literal(2), Literal(3))), want: "add(literal,multiply(literal,literal))"},
		{name: "parenthesized addition with multiplication", expression: Add[int](Literal(1), Multiply[int](Literal(2), Literal(3))), want: "add(literal,multiply(literal,literal))"},
		{name: "left associative subtraction", expression: Subtract[int](Subtract[int](Literal(2), Divide[int](Literal(2), Literal(3))), Literal(4)), want: "subtract(subtract(literal,divide(literal,literal)),literal)"},
		{name: "explicit subtraction grouping", expression: Subtract[int](Subtract[int](Literal(2), Divide[int](Literal(2), Literal(3))), Literal(4)), want: "subtract(subtract(literal,divide(literal,literal)),literal)"},
		{name: "in after addition", expression: In[int](Add[int](Literal(1), Literal(2)), Literal(4), Literal(5)), want: "in(add(literal,literal),literal,literal)"},
		{name: "explicit in grouping", expression: In[int](Add[int](Literal(1), Literal(2)), Literal(4), Literal(5)), want: "in(add(literal,literal),literal,literal)"},
		{name: "and before or", expression: Or(And(Literal(true), Literal(false)), Literal(true)), want: "or(and(literal,literal),literal)"},
		{name: "explicit and before or", expression: Or(And(Literal(true), Literal(false)), Literal(true)), want: "or(and(literal,literal),literal)"},
		{name: "or grouped under and", expression: And(Literal(true), Or(Literal(false), Literal(true))), want: "and(literal,or(literal,literal))"},
		{name: "extra parentheses disappear structurally", expression: And(Literal(true), Or(Literal(false), Literal(true))), want: "and(literal,or(literal,literal))"},
		{name: "or chain with nested and", expression: Or(Or(Literal(false), And(Literal(false), Literal(true))), Literal(false)), want: "or(or(literal,and(literal,literal)),literal)"},
		{name: "explicit or chain", expression: Or(Or(Literal(false), And(Literal(false), Literal(true))), Literal(false)), want: "or(or(literal,and(literal,literal)),literal)"},
		{name: "concat before equality", expression: Equal[string](Concat(Literal("a"), Literal("b")), Literal("ab")), want: "eq(concat(literal,literal),literal)"},
		{name: "explicit concat grouping", expression: Equal[string](Concat(Literal("a"), Literal("b")), Literal("ab")), want: "eq(concat(literal,literal),literal)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := InspectExpression(test.expression)
			if got := expressionModelShape(model); got != test.want {
				t.Fatalf("expression model shape = %q, want %q (%s)", got, test.want, model.Description())
			}
		})
	}
}

func TestClientCompileSODAPrecedencePatternsMatchesEsper(t *testing.T) {
	env := newClientCompileStatementModelEnvironment(t)
	event := func(tag string) PatternStream {
		return PatternFrom(From[clientCompileStatementModelEvent](env, "ClientCompileStatementModelEvent"), tag, Literal(true))
	}
	tests := []struct {
		name    string
		pattern PatternStream
		want    string
	}{
		{name: "or with nested and", pattern: event("A").Or(event("B").And(event("C"))), want: "or(event,and(event,event))"},
		{name: "and over grouped or", pattern: event("A").Or(event("B")).And(event("C")), want: "and(or(event,event),event)"},
		{name: "repeated grouped or", pattern: event("A").Or(event("B")).And(event("C")), want: "and(or(event,event),event)"},
		{name: "or of every branches", pattern: event("A").Every().Or(event("B").Every()), want: "or(every(event),every(event))"},
		{name: "followed-by over or", pattern: event("B").Then(event("D").Or(event("A"))), want: "followed-by(event,or(event,event))"},
		{name: "every and not", pattern: event("A").Every().And(event("B").Not()), want: "and(every(event),not(event))"},
		{name: "repeated every and not", pattern: event("A").Every().And(event("B").Not()), want: "and(every(event),not(event))"},
		{name: "every followed-by", pattern: event("A").Every().Then(event("B")), want: "followed-by(every(event),event)"},
		{name: "within guard", pattern: event("A").Within(10 * time.Second), want: "within(event)"},
		{name: "every grouped and", pattern: event("A").And(event("B")).Every(), want: "every(and(event,event))"},
		{name: "every outside within", pattern: event("A").Within(10 * time.Second).Every(), want: "every(within(event))"},
		{name: "or with until right", pattern: event("A").Or(event("B").Until(event("C"))), want: "or(event,until(event,event))"},
		{name: "explicit or with until", pattern: event("A").Or(event("B").Until(event("C"))), want: "or(event,until(event,event))"},
		{name: "nested every normalizes root", pattern: event("A").Every(), want: "every(event)"},
		{name: "nested until", pattern: event("A").Until(event("B")).Until(event("C")), want: "until(until(event,event),event)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := test.pattern.Model()
			if got := patternModelShape(model); got != test.want {
				t.Fatalf("pattern model shape = %q, want %q (%s)", got, test.want, model.Description())
			}
			queryModel, ok := test.pattern.Query().PatternModel()
			if !ok || patternModelShape(queryModel) != test.want {
				t.Fatalf("query pattern model = %q, %t", patternModelShape(queryModel), ok)
			}
		})
	}
}

func TestStatementObjectModelsAreDetachedAndNilSafe(t *testing.T) {
	expression := Add[int](Literal(1), Multiply[int](Literal(2), Literal(3)))
	model := InspectExpression(expression)
	children := model.Children()
	children[0] = ExpressionNodeModel{}
	if got := expressionModelShape(model); got != "add(literal,multiply(literal,literal))" {
		t.Fatalf("mutating expression children changed model: %q", got)
	}
	if InspectExpression(nil).Valid() {
		t.Fatal("nil expression produced a valid model")
	}

	var emptyPattern PatternStream
	if emptyPattern.Model().Valid() {
		t.Fatal("empty pattern produced a valid model")
	}
	if _, ok := (Query{}).PatternModel(); ok {
		t.Fatal("non-pattern query reported a pattern model")
	}
}

func expressionModelShape(model ExpressionNodeModel) string {
	if !model.Valid() {
		return "<nil>"
	}
	children := model.Children()
	if len(children) == 0 {
		return model.Kind()
	}
	parts := make([]string, len(children))
	for index, child := range children {
		parts[index] = expressionModelShape(child)
	}
	return model.Kind() + "(" + strings.Join(parts, ",") + ")"
}

func patternModelShape(model PatternNodeModel) string {
	if !model.Valid() {
		return "<nil>"
	}
	children := model.Children()
	if len(children) == 0 {
		return model.Kind()
	}
	parts := make([]string, len(children))
	for index, child := range children {
		parts[index] = patternModelShape(child)
	}
	return model.Kind() + "(" + strings.Join(parts, ",") + ")"
}
