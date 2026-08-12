package esper

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

type subqueryEventStreamTrigger struct {
	ID string `esper:"id"`
}

func TestEventStreamUniqueSubqueryWindowMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[namedWindowArrayTrade](env, "SubqueryEventStreamTrade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subqueryEventStreamTrigger](env, "SubqueryEventStreamTrigger"); err != nil {
		t.Fatal(err)
	}

	inner := Select(From[namedWindowArrayTrade](env, "SubqueryEventStreamTrade")).Window(
		Unique(Field[namedWindowArrayTrade, []int64]("coll")),
	)
	query := Select(
		From[subqueryEventStreamTrigger](env, "SubqueryEventStreamTrigger"),
		Alias("ids", SubqueryValue[[]string](inner, WindowValues[string](Field[any, string]("id")))),
	).Query(StatementName("event-stream-unique-subquery"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("event-stream subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendInner := func(event namedWindowArrayTrade) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	trigger := func(id string, want []string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), subqueryEventStreamTrigger{ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatalf("event-stream subquery did not emit for %q", id)
		}
		got, ok := rows[len(rows)-1].Get("ids").Any().([]string)
		gotAnyOrder := append([]string(nil), got...)
		wantAnyOrder := append([]string(nil), want...)
		sort.Strings(gotAnyOrder)
		sort.Strings(wantAnyOrder)
		if !ok || !reflect.DeepEqual(gotAnyOrder, wantAnyOrder) {
			t.Fatalf("event-stream subquery ids for %q = %#v, want %#v", id, got, want)
		}
	}

	sendInner(namedWindowArrayTrade{ID: "E0", Coll: []int64{1, 2}})
	sendInner(namedWindowArrayTrade{ID: "E1", Coll: nil})
	sendInner(namedWindowArrayTrade{ID: "E2", Coll: []int64{1}})
	sendInner(namedWindowArrayTrade{ID: "E3", Coll: []int64{}})
	sendInner(namedWindowArrayTrade{ID: "E4", Coll: []int64{1}})
	trigger("T0", []string{"E0", "E1", "E3", "E4"})

	sendInner(namedWindowArrayTrade{ID: "E10", Coll: []int64{1, 2}})
	sendInner(namedWindowArrayTrade{ID: "E13", Coll: []int64{}})
	sendInner(namedWindowArrayTrade{ID: "E14", Coll: []int64{1}})
	trigger("T1", []string{"E10", "E1", "E13", "E14"})
}

func TestEventStreamUniqueSubqueryCountFilterMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[namedWindowArrayTrade](env, "SubqueryEventCountTrade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subqueryEventStreamTrigger](env, "SubqueryEventCountTrigger"); err != nil {
		t.Fatal(err)
	}
	inner := Select(From[namedWindowArrayTrade](env, "SubqueryEventCountTrade")).Window(
		Unique(Field[namedWindowArrayTrade, []int64]("coll")),
	)
	trigger := From[subqueryEventStreamTrigger](env, "SubqueryEventCountTrigger")
	query := Select(
		trigger.Filter(Equal[int64](SubqueryCount(inner), Literal(int64(2)))),
		Alias("id", Field[subqueryEventStreamTrigger, string]("id")),
	).Query(StatementName("event-stream-unique-subquery-count-filter"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("event-stream count result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendInner := func(event namedWindowArrayTrade) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	sendTrigger := func(id string, wantRows int) {
		t.Helper()
		before := len(rows)
		if err := engine.SendEvent(context.Background(), subqueryEventStreamTrigger{ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(rows)-before != wantRows {
			t.Fatalf("event-stream count rows for %q = %d, want %d", id, len(rows)-before, wantRows)
		}
	}

	sendTrigger("T0", 0)
	sendInner(namedWindowArrayTrade{ID: "E0", Coll: []int64{1, 2}})
	sendInner(namedWindowArrayTrade{ID: "E1", Coll: []int64{1}})
	sendTrigger("T1", 1)
	sendInner(namedWindowArrayTrade{ID: "E2", Coll: []int64{1, 2}})
	sendTrigger("T2", 1)
	sendInner(namedWindowArrayTrade{ID: "E3", Coll: []int64{2, 1}})
	sendTrigger("T3", 0)
}
