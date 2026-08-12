package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type rowRecogMultikeyArrayEvent struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

type rowRecogMultikeyPlainEvent struct {
	ID              string  `esper:"id"`
	IntPrimitive    int     `esper:"intPrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

func TestRowRecogPartitionMultikeyWithArrayContent(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogMultikeyArrayEvent](env, "SupportEventWithIntArray"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	stream := From[rowRecogMultikeyArrayEvent](env, "SupportEventWithIntArray").Window(KeepAll())
	array := Field[rowRecogMultikeyArrayEvent, []int]("array")
	value := Field[rowRecogMultikeyArrayEvent, int]("value")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		PartitionBy(array).
		Define("A", Equal[int](value, Literal(1))).
		Define("B", Equal[int](value, Literal(2))).
		Measures(
			Alias("a", TagField[string]("A", "id")),
			Alias("b", TagField[string]("B", "id")),
		).
		Query(StatementName("rowrecog-partition-multikey-array"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)

	send := func(event rowRecogMultikeyArrayEvent, expected ...string) {
		t.Helper()
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		got := (*rows)[before:]
		if len(got) != 0 && len(expected) == 0 {
			t.Fatalf("array partition event %s produced unexpected rows %#v", event.ID, got)
		}
		if len(expected) != 0 {
			if len(got) != 1 {
				t.Fatalf("array partition event %s rows = %#v, want one row", event.ID, got)
			}
			key := fmt.Sprintf("%v/%v", got[0].Get("a").Any(), got[0].Get("b").Any())
			if key != fmt.Sprintf("%s/%s", expected[0], expected[1]) {
				t.Fatalf("array partition event %s row = %s, want %s/%s", event.ID, key, expected[0], expected[1])
			}
		}
	}

	send(rowRecogMultikeyArrayEvent{ID: "E1", Array: []int{1, 2}, Value: 1})
	send(rowRecogMultikeyArrayEvent{ID: "E2", Array: []int{1}, Value: 1})
	send(rowRecogMultikeyArrayEvent{ID: "E3", Array: nil, Value: 1})
	send(rowRecogMultikeyArrayEvent{ID: "E4", Array: []int{}, Value: 1})
	send(rowRecogMultikeyArrayEvent{ID: "E10", Array: []int{1, 2}, Value: 2}, "E1", "E10")
	send(rowRecogMultikeyArrayEvent{ID: "E11", Array: []int{}, Value: 2}, "E4", "E11")
	send(rowRecogMultikeyArrayEvent{ID: "E12", Array: []int{1}, Value: 2}, "E2", "E12")
	send(rowRecogMultikeyArrayEvent{ID: "E13", Array: nil, Value: 2}, "E3", "E13")

	if got := len(*rows); got != 4 {
		t.Fatalf("array partition total rows = %d, want 4", got)
	}
	if !reflect.DeepEqual((*rows)[0].Get("a").Any(), "E1") || !reflect.DeepEqual((*rows)[0].Get("b").Any(), "E10") {
		t.Fatalf("array partition first row = %#v", (*rows)[0])
	}
}

func TestRowRecogPartitionMultikeyPlainTuple(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogMultikeyPlainEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	stream := From[rowRecogMultikeyPlainEvent](env, "SupportBean").Window(KeepAll())
	intPrimitive := Field[rowRecogMultikeyPlainEvent, int]("intPrimitive")
	longPrimitive := Field[rowRecogMultikeyPlainEvent, int64]("longPrimitive")
	doublePrimitive := Field[rowRecogMultikeyPlainEvent, float64]("doublePrimitive")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		PartitionBy(intPrimitive, longPrimitive).
		Define("A", Equal[float64](doublePrimitive, Literal(1.0))).
		Define("B", Equal[float64](doublePrimitive, Literal(2.0))).
		Measures(
			Alias("a", TagField[string]("A", "id")),
			Alias("b", TagField[string]("B", "id")),
		).
		Query(StatementName("rowrecog-partition-multikey-plain"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)

	for _, event := range []rowRecogMultikeyPlainEvent{
		{ID: "E1", IntPrimitive: 1, LongPrimitive: 2, DoublePrimitive: 1},
		{ID: "E2", IntPrimitive: 1, LongPrimitive: 3, DoublePrimitive: 1},
		{ID: "E3", IntPrimitive: 2, LongPrimitive: 2, DoublePrimitive: 1},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	send := func(event rowRecogMultikeyPlainEvent, expectedA, expectedB string) {
		t.Helper()
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		got := (*rows)[before:]
		if len(got) != 1 || got[0].Get("a").Any() != expectedA || got[0].Get("b").Any() != expectedB {
			t.Fatalf("plain tuple event %s rows = %#v, want %s/%s", event.ID, got, expectedA, expectedB)
		}
	}
	send(rowRecogMultikeyPlainEvent{ID: "E10", IntPrimitive: 2, LongPrimitive: 2, DoublePrimitive: 2}, "E3", "E10")
	send(rowRecogMultikeyPlainEvent{ID: "E11", IntPrimitive: 1, LongPrimitive: 3, DoublePrimitive: 2}, "E2", "E11")
	send(rowRecogMultikeyPlainEvent{ID: "E12", IntPrimitive: 1, LongPrimitive: 2, DoublePrimitive: 2}, "E1", "E12")

	if got := len(*rows); got != 3 {
		t.Fatalf("plain tuple total rows = %d, want 3", got)
	}
}
