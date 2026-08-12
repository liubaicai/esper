package esper

import (
	"context"
	"reflect"
	"testing"
)

type localGroupEvent struct {
	ID    string `esper:"id"`
	Group string `esper:"group"`
	Level int    `esper:"level"`
	Value int64  `esper:"value"`
}

type localGroupDeleteEvent struct {
	ID string `esper:"id"`
}

func newLocalGroupTest(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[localGroupEvent](env, "LocalGroupEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[localGroupDeleteEvent](env, "LocalGroupDeleteEvent"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func TestLocalGroupByMultiKeyTrace(t *testing.T) {
	env, engine := newLocalGroupTest(t)
	group := Field[localGroupEvent, string]("group")
	level := Field[localGroupEvent, int]("level")
	value := Field[localGroupEvent, int64]("value")
	plan, err := env.Build(From[localGroupEvent](env, "LocalGroupEvent").Aggregate(
		Alias("pairSum", LocalGroupBy[int64](Sum[int64](value), group, level)),
		Alias("groupSum", LocalGroupBy[int64](Sum[int64](value), group)),
		Alias("levelSum", LocalGroupBy[int64](Sum[int64](value), level)),
		Alias("total", Sum[int64](value)),
	).Query(StatementName("local-group-multi-key")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var latest Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			latest, _ = batch.New[len(batch.New)-1].Row()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	expected := []struct {
		event                     localGroupEvent
		pair, group, level, total int64
	}{
		{localGroupEvent{"E1-1", "E1", 1, 10}, 10, 10, 10, 10},
		{localGroupEvent{"E2-2", "E2", 2, 11}, 11, 11, 11, 21},
		{localGroupEvent{"E1-2", "E1", 2, 12}, 12, 22, 23, 33},
		{localGroupEvent{"E1-1b", "E1", 1, 13}, 23, 35, 23, 46},
		{localGroupEvent{"E2-1", "E2", 1, 14}, 14, 25, 37, 60},
	}
	for index, item := range expected {
		if err := engine.SendEvent(context.Background(), item.event); err != nil {
			t.Fatal(err)
		}
		if latest.Get("pairSum").Any() != item.pair || latest.Get("groupSum").Any() != item.group || latest.Get("levelSum").Any() != item.level || latest.Get("total").Any() != item.total {
			t.Fatalf("local group row %d = %#v", index, latest.AsMap())
		}
	}
}

func TestLocalGroupByOuterGroupsShareCrossGroupState(t *testing.T) {
	env, engine := newLocalGroupTest(t)
	group := Field[localGroupEvent, string]("group")
	level := Field[localGroupEvent, int]("level")
	value := Field[localGroupEvent, int64]("value")
	event := EventValue[localGroupEvent]()
	plan, err := env.Build(From[localGroupEvent](env, "LocalGroupEvent").Window(KeepAll()).GroupBy(group, level).Select(
		Alias("group", group),
		Alias("level", level),
		Alias("groupSum", LocalGroupBy[int64](Sum[int64](value), group)),
		Alias("levelSum", LocalGroupBy[int64](Sum[int64](value), level)),
		Alias("allSum", LocalGroupBy[int64](Sum[int64](value))),
		Alias("groupValues", LocalGroupBy[[]localGroupEvent](WindowValues[localGroupEvent](event), group)),
		Alias("levelValues", LocalGroupBy[[]localGroupEvent](WindowValues[localGroupEvent](event), level)),
		Alias("allValues", LocalGroupBy[[]localGroupEvent](WindowValues[localGroupEvent](event))),
	).Query(StatementName("local-group-cross-outer")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var latest Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			latest, _ = batch.New[len(batch.New)-1].Row()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	events := []localGroupEvent{
		{ID: "1", Group: "E1", Level: 10, Value: 100},
		{ID: "2", Group: "E1", Level: 20, Value: 202},
		{ID: "3", Group: "E2", Level: 10, Value: 303},
		{ID: "4", Group: "E1", Level: 10, Value: 404},
		{ID: "5", Group: "E2", Level: 10, Value: 505},
	}
	expected := []struct {
		group, level               any
		groupSum, levelSum, allSum int64
		groupValues, levelValues   []localGroupEvent
		allValues                  []localGroupEvent
	}{
		{"E1", 10, 100, 100, 100, []localGroupEvent{events[0]}, []localGroupEvent{events[0]}, []localGroupEvent{events[0]}},
		{"E1", 20, 302, 202, 302, []localGroupEvent{events[0], events[1]}, []localGroupEvent{events[1]}, []localGroupEvent{events[0], events[1]}},
		{"E2", 10, 303, 403, 605, []localGroupEvent{events[2]}, []localGroupEvent{events[0], events[2]}, []localGroupEvent{events[0], events[1], events[2]}},
		{"E1", 10, 706, 807, 1009, []localGroupEvent{events[0], events[1], events[3]}, []localGroupEvent{events[0], events[2], events[3]}, []localGroupEvent{events[0], events[1], events[2], events[3]}},
		{"E2", 10, 808, 1312, 1514, []localGroupEvent{events[2], events[4]}, []localGroupEvent{events[0], events[2], events[3], events[4]}, events},
	}
	for index, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if latest.Get("group").Any() != expected[index].group || latest.Get("level").Any() != expected[index].level ||
			latest.Get("groupSum").Any() != expected[index].groupSum || latest.Get("levelSum").Any() != expected[index].levelSum || latest.Get("allSum").Any() != expected[index].allSum {
			t.Fatalf("cross-group local aggregate row %d = %#v", index, latest.AsMap())
		}
		for _, name := range []string{"groupValues", "levelValues", "allValues"} {
			if got, ok := latest.Get(name).Any().([]localGroupEvent); !ok {
				t.Fatalf("cross-group local aggregate %d %s type = %#v", index, name, latest.Get(name).Any())
			} else {
				var want []localGroupEvent
				switch name {
				case "groupValues":
					want = expected[index].groupValues
				case "levelValues":
					want = expected[index].levelValues
				case "allValues":
					want = expected[index].allValues
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("cross-group local aggregate row %d %s = %#v, want %#v", index, name, got, want)
				}
			}
		}
	}
}

func TestLocalGroupByNamedWindowDeleteRecomputesOuterAndLocalState(t *testing.T) {
	env, _ := newLocalGroupTest(t)
	schema, ok := env.Schema("LocalGroupEvent")
	if !ok {
		t.Fatal("local group schema is missing")
	}
	if _, err := CreateNamedWindow(env, "local-group-window", schema); err != nil {
		t.Fatal(err)
	}
	source := From[localGroupEvent](env, "LocalGroupEvent")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow("local-group-window",
		SetColumn("id", Field[localGroupEvent, string]("id")),
		SetColumn("group", Field[localGroupEvent, string]("group")),
		SetColumn("level", Field[localGroupEvent, int]("level")),
		SetColumn("value", Field[localGroupEvent, int64]("value")),
	).Query(StatementName("local-group-insert")))
	if err != nil {
		t.Fatal(err)
	}
	group := Field[any, string]("group")
	level := Field[any, int]("level")
	value := Field[any, int64]("value")
	aggregatePlan, err := env.Build(FromNamedWindow(env, "local-group-window").GroupBy(group, level).Select(
		Alias("group", group),
		Alias("level", level),
		Alias("groupSum", LocalGroupBy[int64](Sum[int64](value), group)),
		Alias("levelSum", LocalGroupBy[int64](Sum[int64](value), level)),
		Alias("allSum", LocalGroupBy[int64](Sum[int64](value))),
	).Query(StatementName("local-group-delete-query")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[localGroupDeleteEvent](env, "LocalGroupDeleteEvent")).DeleteFromNamedWindow(
		"local-group-window",
		Equal[string](NamedWindowField[string]("id"), Field[localGroupDeleteEvent, string]("id")),
	).Query(StatementName("local-group-delete")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), aggregatePlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 5)
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
	for _, event := range []localGroupEvent{
		{ID: "1", Group: "E1", Level: 10, Value: 100},
		{ID: "2", Group: "E1", Level: 20, Value: 202},
		{ID: "3", Group: "E2", Level: 10, Value: 303},
		{ID: "4", Group: "E1", Level: 10, Value: 404},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), localGroupDeleteEvent{ID: "1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("local group delete rows = %d", len(rows))
	}
	last := rows[len(rows)-1]
	if last.Get("group").Any() != "E1" || last.Get("level").Any() != 10 || last.Get("groupSum").Any() != int64(606) || last.Get("levelSum").Any() != int64(707) || last.Get("allSum").Any() != int64(909) {
		t.Fatalf("local group delete row = %#v", last.AsMap())
	}
}

func TestLocalGroupByRejectsInvalidKeys(t *testing.T) {
	env, _ := newLocalGroupTest(t)
	value := Field[localGroupEvent, int64]("value")
	group := Field[localGroupEvent, string]("group")
	if _, err := env.Build(From[localGroupEvent](env, "LocalGroupEvent").Aggregate(
		Alias("bad", LocalGroupBy[int64](Sum[int64](value), nil)),
	).Query(StatementName("local-group-nil-key"))); err == nil {
		t.Fatal("local group aggregate with nil key unexpectedly built")
	}
	if _, err := env.Build(From[localGroupEvent](env, "LocalGroupEvent").Aggregate(
		Alias("bad", LocalGroupBy[int64](Sum[int64](value), CountAll())),
	).Query(StatementName("local-group-aggregate-key"))); err == nil {
		t.Fatal("local group aggregate with aggregate key unexpectedly built")
	}
	if _, err := env.Build(From[localGroupEvent](env, "LocalGroupEvent").Aggregate(
		Alias("good", LocalGroupBy[int64](Sum[int64](value), group)),
	).Query(StatementName("local-group-valid-key"))); err != nil {
		t.Fatalf("valid local group key rejected: %v", err)
	}
}
