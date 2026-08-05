package esper

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

type namedWindowArraySubqueryTrigger struct {
	ID string `esper:"id"`
}

func TestNamedWindowArrayUniqueSubqueryWindowMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	tradeSchema, err := RegisterStruct[namedWindowArrayTrade](env, "NamedWindowArraySubqueryTrade")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[namedWindowArraySubqueryTrigger](env, "NamedWindowArraySubqueryTrigger"); err != nil {
		t.Fatal(err)
	}
	const windowName = "named-array-subquery-window"
	key := Field[namedWindowArrayTrade, []int64]("coll")
	if _, err := CreateNamedWindow(env, windowName, tradeSchema, NamedWindowRetention(Unique(key))); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	window := FromNamedWindow(env, windowName)
	ids := WindowValues[string](Field[any, string]("id"))
	query := Select(From[namedWindowArraySubqueryTrigger](env, "NamedWindowArraySubqueryTrigger"),
		Alias("ids", SubqueryValue[[]string](window, ids)),
	).Query(StatementName("named-array-subquery-window"), WithOldStream())
	plan, err := env.Build(query)
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
				t.Fatalf("array subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	insert := func(event namedWindowArrayTrade) {
		t.Helper()
		if err := engine.InsertNamedWindow(context.Background(), windowName, event); err != nil {
			t.Fatal(err)
		}
	}
	trigger := func(id string, want []string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), namedWindowArraySubqueryTrigger{ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatalf("array subquery did not emit for trigger %q", id)
		}
		got, ok := rows[len(rows)-1].Get("ids").Any().([]string)
		gotAnyOrder := append([]string(nil), got...)
		wantAnyOrder := append([]string(nil), want...)
		sort.Strings(gotAnyOrder)
		sort.Strings(wantAnyOrder)
		if !ok || !reflect.DeepEqual(gotAnyOrder, wantAnyOrder) {
			t.Fatalf("array subquery ids for %q = %#v, want %#v", id, got, want)
		}
	}

	insert(namedWindowArrayTrade{ID: "E0", Coll: []int64{1, 2}})
	insert(namedWindowArrayTrade{ID: "E1", Coll: nil})
	insert(namedWindowArrayTrade{ID: "E2", Coll: []int64{1}})
	insert(namedWindowArrayTrade{ID: "E3", Coll: []int64{}})
	insert(namedWindowArrayTrade{ID: "E4", Coll: []int64{1}})
	trigger("T0", []string{"E0", "E1", "E3", "E4"})

	insert(namedWindowArrayTrade{ID: "E10", Coll: []int64{1, 2}})
	insert(namedWindowArrayTrade{ID: "E13", Coll: []int64{}})
	insert(namedWindowArrayTrade{ID: "E14", Coll: []int64{1}})
	trigger("T1", []string{"E10", "E1", "E13", "E14"})
}

func TestNamedWindowArrayUniqueSubqueryCountFilterMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	tradeSchema, err := RegisterStruct[namedWindowArrayTrade](env, "NamedWindowArrayCountTrade")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[namedWindowArraySubqueryTrigger](env, "NamedWindowArrayCountTrigger"); err != nil {
		t.Fatal(err)
	}
	const windowName = "named-array-subquery-count"
	key := Field[namedWindowArrayTrade, []int64]("coll")
	if _, err := CreateNamedWindow(env, windowName, tradeSchema, NamedWindowRetention(Unique(key))); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	window := FromNamedWindow(env, windowName)
	triggerStream := From[namedWindowArraySubqueryTrigger](env, "NamedWindowArrayCountTrigger")
	query := Select(triggerStream.Filter(
		Equal[int64](SubqueryCount(window), Literal(int64(2))),
	),
		Alias("id", Field[namedWindowArraySubqueryTrigger, string]("id")),
	).Query(StatementName("named-array-subquery-count-filter"))
	plan, err := env.Build(query)
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
				t.Fatalf("array count subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	trigger := func(id string, wantRows int) {
		t.Helper()
		before := len(rows)
		if err := engine.SendEvent(context.Background(), namedWindowArraySubqueryTrigger{ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(rows)-before != wantRows {
			t.Fatalf("array count subquery rows for %q = %d, want %d", id, len(rows)-before, wantRows)
		}
	}
	insert := func(event namedWindowArrayTrade) {
		t.Helper()
		if err := engine.InsertNamedWindow(context.Background(), windowName, event); err != nil {
			t.Fatal(err)
		}
	}

	trigger("T0", 0)
	insert(namedWindowArrayTrade{ID: "E0", Coll: []int64{1, 2}})
	insert(namedWindowArrayTrade{ID: "E1", Coll: []int64{1}})
	trigger("T1", 1)
	insert(namedWindowArrayTrade{ID: "E2", Coll: []int64{1, 2}})
	trigger("T2", 1)
	insert(namedWindowArrayTrade{ID: "E3", Coll: []int64{2, 1}})
	trigger("T3", 0)
}
