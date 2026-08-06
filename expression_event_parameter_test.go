package esper

import (
	"context"
	"reflect"
	"testing"
)

type eventParameterParityEvent struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
	P10 string `esper:"p10"`
	P11 string `esper:"p11"`
}

type eventParameterParityTrigger struct {
	ID string `esper:"id"`
}

func deployEventParameterRows(t *testing.T, env *Environment, query Query) (*Engine, *[]Row) {
	t.Helper()
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("event parameter result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return engine, &rows
}

func TestParameterizedNamedExpressionReferenceSupportsEventValueParameters(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[eventParameterParityEvent](env, "EventParameterCurrentEvent"); err != nil {
		t.Fatal(err)
	}
	eventParam := ExpressionParam[eventParameterParityEvent]("event")
	valueParam := ExpressionParam[string]("value")
	body := Concat(
		Property[string](eventParam, "p00"),
		valueParam,
		Property[string](eventParam, "p01"),
	)
	if err := DefineExpression[string](env, "combine-event-value", body); err != nil {
		t.Fatal(err)
	}
	engine, rows := deployEventParameterRows(t, env, Select(
		From[eventParameterParityEvent](env, "EventParameterCurrentEvent"),
		Alias("value", ExpressionRef[string](env, "combine-event-value", EventValue[eventParameterParityEvent](), Literal("x"))),
	).Query(StatementName("declared-event-value")))
	for _, event := range []eventParameterParityEvent{{P00: "A", P01: "B"}, {P00: "C", P01: "D"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 2 || (*rows)[0].Get("value").Any() != "AxB" || (*rows)[1].Get("value").Any() != "CxD" {
		t.Fatalf("current event parameter rows = %#v", *rows)
	}
}

func TestParameterizedNamedExpressionReferenceSupportsJoinedEventValues(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[eventParameterParityEvent](env, "EventParameterJoinLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[eventParameterParityEvent](env, "EventParameterJoinRight"); err != nil {
		t.Fatal(err)
	}
	eventOne := ExpressionParam[eventParameterParityEvent]("eventOne")
	eventTwo := ExpressionParam[eventParameterParityEvent]("eventTwo")
	valueOne := ExpressionParam[string]("valueOne")
	valueTwo := ExpressionParam[string]("valueTwo")
	body := Concat(
		valueOne,
		Property[string](eventOne, "p00"),
		valueTwo,
		Property[string](eventTwo, "p11"),
	)
	if err := DefineExpression[string](env, "combine-join-events", body); err != nil {
		t.Fatal(err)
	}
	left := From[eventParameterParityEvent](env, "EventParameterJoinLeft").Window(LastEvent())
	right := From[eventParameterParityEvent](env, "EventParameterJoinRight").Window(LastEvent())
	joined := Join(left, right, OnEqual(
		Field[eventParameterParityEvent, int]("id"),
		Field[eventParameterParityEvent, int]("id"),
	))
	query := joined.Select(SelectFrom(0, "value", ExpressionRef[string](env, "combine-join-events",
		Literal("x"), JoinEventValue[eventParameterParityEvent](0),
		Literal("y"), JoinEventValue[eventParameterParityEvent](1),
	))).Query(StatementName("declared-join-events"))
	engine, rows := deployEventParameterRows(t, env, query)
	if err := engine.Send(context.Background(), "EventParameterJoinRight", eventParameterParityEvent{ID: 1, P11: "R"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "EventParameterJoinLeft", eventParameterParityEvent{ID: 1, P00: "L"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || (*rows)[0].Get("value").Any() != "xLyR" {
		t.Fatalf("joined event parameter rows = %#v", *rows)
	}
}

func TestParameterizedNamedExpressionReferenceSupportsPatternEventValues(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[eventParameterParityEvent](env, "EventParameterPattern"); err != nil {
		t.Fatal(err)
	}
	eventParam := ExpressionParam[Event]("event")
	if err := DefineExpression[string](env, "pattern-event-value", Property[string](eventParam, "p00")); err != nil {
		t.Fatal(err)
	}
	input := From[eventParameterParityEvent](env, "EventParameterPattern")
	pattern := PatternFrom(input, "a", Literal(true)).FollowedBy("b", Literal(true))
	query := pattern.Select(Alias("value", ExpressionRef[string](env, "pattern-event-value", PatternEvent("a")))).Query(StatementName("declared-pattern-event"))
	engine, rows := deployEventParameterRows(t, env, query)
	if err := engine.SendEvent(context.Background(), eventParameterParityEvent{P00: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), eventParameterParityEvent{P00: "second"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || (*rows)[0].Get("value").Any() != "first" {
		t.Fatalf("pattern event parameter rows = %#v", *rows)
	}
}

func TestParameterizedNamedExpressionReferenceSupportsSubqueryEventValues(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[eventParameterParityEvent](env, "EventParameterSubquerySource"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[eventParameterParityTrigger](env, "EventParameterSubqueryTrigger"); err != nil {
		t.Fatal(err)
	}
	eventParam := ExpressionParam[Event]("event")
	if err := DefineExpression[string](env, "subquery-event-value", Concat(Property[string](eventParam, "p00"), Property[string](eventParam, "p01"))); err != nil {
		t.Fatal(err)
	}
	inner := Select(From[eventParameterParityEvent](env, "EventParameterSubquerySource")).Window(LastEvent())
	argument := SubqueryValue[Event](inner, EventValue[Event]())
	engine, rows := deployEventParameterRows(t, env, Select(
		From[eventParameterParityTrigger](env, "EventParameterSubqueryTrigger"),
		Alias("value", ExpressionRef[string](env, "subquery-event-value", argument)),
	).Query(StatementName("declared-subquery-event")))
	if err := engine.SendEvent(context.Background(), eventParameterParityTrigger{ID: "before"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || (*rows)[0].Get("value").Any() != nil {
		t.Fatalf("empty subquery event parameter rows = %#v", *rows)
	}
	if err := engine.SendEvent(context.Background(), eventParameterParityEvent{P00: "A", P01: "B"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), eventParameterParityTrigger{ID: "after"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 2 || (*rows)[1].Get("value").Any() != "AB" {
		t.Fatalf("subquery event parameter rows = %#v", *rows)
	}
}

func TestParameterizedNamedExpressionReferenceSupportsMapPatternEventValues(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{FieldDef("p00", reflect.TypeOf("")), FieldDef("p01", reflect.TypeOf(""))}
	if _, err := RegisterMap(env, "EventParameterMapPattern", fields); err != nil {
		t.Fatal(err)
	}
	eventParam := ExpressionParam[Event]("event")
	if err := DefineExpression[string](env, "map-pattern-event-value", Concat(Property[string](eventParam, "p00"), Property[string](eventParam, "p01"))); err != nil {
		t.Fatal(err)
	}
	pattern := PatternFromRecord(FromAny(env, "EventParameterMapPattern"), "a", Literal(true)).FollowedBy("b", Literal(true))
	query := pattern.Select(Alias("value", ExpressionRef[string](env, "map-pattern-event-value", PatternEvent("a")))).Query(StatementName("declared-map-pattern-event"))
	engine, rows := deployEventParameterRows(t, env, query)
	if err := engine.SendRecord(context.Background(), "EventParameterMapPattern", map[string]any{"p00": "A", "p01": "B"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendRecord(context.Background(), "EventParameterMapPattern", map[string]any{"p00": "C", "p01": "D"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || (*rows)[0].Get("value").Any() != "AB" {
		t.Fatalf("map pattern event parameter rows = %#v", *rows)
	}
}

func TestParameterizedNamedExpressionReferenceSupportsMapSubqueryEventValues(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{FieldDef("p00", reflect.TypeOf("")), FieldDef("p01", reflect.TypeOf(""))}
	if _, err := RegisterMap(env, "EventParameterMapSource", fields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "EventParameterMapTrigger", nil); err != nil {
		t.Fatal(err)
	}
	eventParam := ExpressionParam[Event]("event")
	if err := DefineExpression[string](env, "map-subquery-event-value", Concat(Property[string](eventParam, "p00"), Property[string](eventParam, "p01"))); err != nil {
		t.Fatal(err)
	}
	inner := FromAny(env, "EventParameterMapSource").Window(LastEvent())
	argument := SubqueryValue[Event](inner, EventValue[Event]())
	engine, rows := deployEventParameterRows(t, env, FromAny(env, "EventParameterMapTrigger").Select(
		Alias("value", ExpressionRef[string](env, "map-subquery-event-value", argument)),
	).Query(StatementName("declared-map-subquery-event")))
	if err := engine.SendRecord(context.Background(), "EventParameterMapTrigger", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || (*rows)[0].Get("value").Any() != nil {
		t.Fatalf("empty map subquery event parameter rows = %#v", *rows)
	}
	if err := engine.SendRecord(context.Background(), "EventParameterMapSource", map[string]any{"p00": "A", "p01": "B"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendRecord(context.Background(), "EventParameterMapTrigger", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 2 || (*rows)[1].Get("value").Any() != "AB" {
		t.Fatalf("map subquery event parameter rows = %#v", *rows)
	}
}
