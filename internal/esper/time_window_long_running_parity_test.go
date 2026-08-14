package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type timeWindowLongBean struct {
	TheString string `esper:"theString"`
	LongBoxed int64  `esper:"longBoxed"`
}

type timeWindowLongRow struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type timeWindowLongMarket struct {
	Symbol string `esper:"symbol"`
}

// TestTimeWindowLongRunningParity mirrors the shared time-window-long-running
// parity scenario (Java InfraTimeWindow): a time(10 sec) named window expires
// old rows at virtual-clock boundaries, and deletes remove rows immediately.
func TestTimeWindowLongRunningParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[timeWindowLongBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[timeWindowLongMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	rowSchema, err := RegisterStruct[timeWindowLongRow](env, "MyWindowRow")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowTW", rowSchema,
		NamedWindowRetention(TimeWindow(10*time.Second))); err != nil {
		t.Fatal(err)
	}
	beanSource := From[timeWindowLongBean](env, "SupportBean")
	insertPlan, err := env.Build(OnEvent(beanSource).InsertIntoNamedWindow(
		"MyWindowTW",
		SetColumn("key", Field[timeWindowLongBean, string]("theString")),
		SetColumn("value", Field[timeWindowLongBean, int64]("longBoxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[timeWindowLongMarket](env, "SupportMarketDataBean")).DeleteFromNamedWindow(
		"MyWindowTW",
		Equal[string](NamedWindowField[string]("key"), Field[timeWindowLongMarket, string]("symbol")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "MyWindowTW").Select(
		Alias("key", Field[any, string]("key")),
		Alias("value", Field[any, int64]("value")),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deploy := func(plan Plan) *Statement {
		t.Helper()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		return deployment.Statements()[0]
	}
	deploy(insertPlan)
	deploy(deletePlan)
	statement := deploy(consumerPlan)

	type batch struct {
		newRows []map[string]any
		oldRows []map[string]any
	}
	var batches []batch
	if _, err := statement.Subscribe(func(_ context.Context, received ResultBatch) error {
		convert := func(results []Result) []map[string]any {
			if len(results) == 0 {
				return nil
			}
			out := make([]map[string]any, 0, len(results))
			for _, result := range results {
				row, ok := result.Row()
				if !ok {
					t.Fatalf("consumer result is not a row: %#v", result)
				}
				out = append(out, map[string]any{"key": row.Get("key").Any(), "value": row.Get("value").Any()})
			}
			return out
		}
		batches = append(batches, batch{newRows: convert(received.New), oldRows: convert(received.Old)})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	snapshotRows := func(t *testing.T) []map[string]any {
		t.Helper()
		result, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		out := make([]map[string]any, 0, len(result.Batch.New))
		for _, item := range result.Batch.New {
			row, ok := item.Row()
			if !ok {
				t.Fatalf("snapshot result is not a row: %#v", item)
			}
			out = append(out, map[string]any{"key": row.Get("key").Any(), "value": row.Get("value").Any()})
		}
		return out
	}
	assertRows := func(t *testing.T, got []map[string]any, want ...map[string]any) {
		t.Helper()
		if len(got) == 0 {
			got = nil
		}
		if len(want) == 0 {
			want = nil
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("rows = %#v, want %#v", got, want)
		}
	}
	row := func(key string, value int64) map[string]any {
		return map[string]any{"key": key, "value": value}
	}
	ctx := context.Background()
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(ctx, time.UnixMilli(ms).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	sendBean := func(key string, value int64) {
		t.Helper()
		if err := engine.SendEvent(ctx, timeWindowLongBean{TheString: key, LongBoxed: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendMarket := func(symbol string) {
		t.Helper()
		if err := engine.SendEvent(ctx, timeWindowLongMarket{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	advance(1000)
	sendBean("E1", 1)
	advance(5000)
	sendBean("E2", 2)
	advance(10000)
	sendBean("E3", 3)
	advance(10999)
	advance(11000)
	sendBean("E4", 4)
	sendMarket("E2")
	advance(20000)
	assertRows(t, snapshotRows(t), row("E4", 4))
	sendMarket("E4")
	assertRows(t, snapshotRows(t))

	want := []batch{
		{newRows: []map[string]any{row("E1", 1)}},
		{newRows: []map[string]any{row("E2", 2)}},
		{newRows: []map[string]any{row("E3", 3)}},
		{oldRows: []map[string]any{row("E1", 1)}},
		{newRows: []map[string]any{row("E4", 4)}},
		{oldRows: []map[string]any{row("E2", 2)}},
		{oldRows: []map[string]any{row("E3", 3)}},
		{oldRows: []map[string]any{row("E4", 4)}},
	}
	if len(batches) != len(want) {
		t.Fatalf("batches = %#v, want %#v", batches, want)
	}
	for i := range want {
		if !reflect.DeepEqual(batches[i], want[i]) {
			t.Fatalf("batches[%d] = %#v, want %#v", i, batches[i], want[i])
		}
	}
}
