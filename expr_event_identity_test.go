package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

type eventIdentityParityEvent struct {
	ID   int    `esper:"id"`
	Text string `esper:"text"`
}

func TestEventIdentityExpressionsMatchJavaEnvelopeIdentitySemantics(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[eventIdentityParityEvent](env, "EventIdentityParityEvent"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("EventIdentityParityEvent")
	if !ok {
		t.Fatal("event identity schema is missing")
	}
	eventOne, err := NewEvent(schema, eventIdentityParityEvent{ID: 1, Text: "same"}, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	eventOneCopy := eventOne
	eventTwo, err := NewEvent(schema, eventIdentityParityEvent{ID: 1, Text: "same"}, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := EventIdentityEquals(Literal(eventOne), Literal(eventOneCopy)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("same envelope identity = %v, want true", got)
	}
	if got := EventIdentityEquals(Literal(eventOne), Literal(eventTwo)).eval(EvalContext{}); !got.Equal(Present(false)) {
		t.Fatalf("equal payload distinct identity = %v, want false", got)
	}
	if got := EventIdentityEquals(NullLiteral[Event](), Literal(eventOne)).eval(EvalContext{}); !got.Equal(Present(false)) {
		t.Fatalf("null event identity = %v, want false", got)
	}
}

func TestEventIdentityExpressionsBuildLiveProjectionAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[eventIdentityParityEvent](env, "EventIdentityLive"); err != nil {
		t.Fatal(err)
	}
	input := From[eventIdentityParityEvent](env, "EventIdentityLive")
	current := EventValue[Event]()
	query := Select(input,
		Alias("same", EventIdentityEquals(current, current)),
		Alias("prior", EventIdentityEquals(current, Prior[Event](0, current))),
	).Query(StatementName("event-identity-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent event identity plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	different, err := env.Build(Select(input,
		Alias("same", EventIdentityEquals(current, current)),
		Alias("prior", EventIdentityEquals(current, Prev[Event](0, current))),
	).Query(StatementName("event-identity-different")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == different.Hash() {
		t.Fatal("event identity previous operator did not enter Plan identity")
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("event identity result schema is missing")
	}
	for _, name := range []string{"same", "prior"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != typeOf[bool]() {
			t.Fatalf("event identity result %q = %#v, want bool", name, field)
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
				return NewError(ErrorTypeMismatch, "event identity result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), eventIdentityParityEvent{ID: 1, Text: "same"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), eventIdentityParityEvent{ID: 1, Text: "same"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[0].Get("same").Equal(Present(true)) || !rows[0].Get("prior").Equal(Present(false)) || !rows[1].Get("same").Equal(Present(true)) || !rows[1].Get("prior").Equal(Present(false)) {
		t.Fatalf("event identity rows = %#v", rows)
	}
}

func TestEventIdentityExpressionsRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[eventIdentityParityEvent](env, "EventIdentityInvalid"); err != nil {
		t.Fatal(err)
	}
	input := From[eventIdentityParityEvent](env, "EventIdentityInvalid")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "nil-left", expr: EventIdentityEquals(nil, EventValue[Event]()), want: "left operand"},
		{name: "nil-right", expr: EventIdentityEquals(EventValue[Event](), nil), want: "right operand"},
		{name: "scalar-left", expr: EventIdentityEquals(Literal(1), EventValue[Event]()), want: "must resolve to an event"},
		{name: "scalar-right", expr: EventIdentityEquals(EventValue[Event](), Literal("x")), want: "must resolve to an event"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("event-identity-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid event identity error = %v, want %q", err, testCase.want)
			}
		})
	}
}
