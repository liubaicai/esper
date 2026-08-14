package esper

import (
	"context"
	"reflect"
	"testing"
)

type deploymentRestartMarket struct {
	Symbol string `esper:"symbol"`
	Volume int64  `esper:"volume"`
}

// TestDeploymentRestartResetsJoinStateParity mirrors the shared
// deployment-restart-window parity scenario (Java EPLJoinStartStopSceneOne):
// undeploy/redeploy resets length(3) join state, so no stale rows remain.
func TestDeploymentRestartResetsJoinStateParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[deploymentRestartMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	ibm := From[deploymentRestartMarket](env, "SupportMarketDataBean").Filter(
		Equal[string](Field[deploymentRestartMarket, string]("symbol"), Literal("IBM")),
	).Window(LengthWindow(3))
	csco := From[deploymentRestartMarket](env, "SupportMarketDataBean").Filter(
		Equal[string](Field[deploymentRestartMarket, string]("symbol"), Literal("CSCO")),
	).Window(LengthWindow(3))
	plan, err := env.Build(Join(
		ibm,
		csco,
		OnEqual(Field[deploymentRestartMarket, int64]("volume"), Field[deploymentRestartMarket, int64]("volume")),
	).Select(
		SelectLeft("s0volume", Field[deploymentRestartMarket, int64]("volume")),
		SelectRight("s1volume", Field[deploymentRestartMarket, int64]("volume")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	ctx := context.Background()
	deploy := func() (*Statement, *Deployment) {
		t.Helper()
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			t.Fatal(err)
		}
		return deployment.Statements()[0], deployment
	}
	rows := func(t *testing.T, statement *Statement) []map[string]any {
		t.Helper()
		snapshot, err := statement.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]map[string]any, 0, len(snapshot.Batch.New))
		for _, result := range snapshot.Batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("snapshot result is not a row: %#v", result)
			}
			out = append(out, map[string]any{"s0volume": row.Get("s0volume").Any(), "s1volume": row.Get("s1volume").Any()})
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
	send := func(symbol string, volume int64) {
		t.Helper()
		if err := engine.SendEvent(ctx, deploymentRestartMarket{Symbol: symbol, Volume: volume}); err != nil {
			t.Fatal(err)
		}
	}

	// First deployment: matching pair produces one row.
	statement, deployment := deploy()
	var batches [][]map[string]any
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows := make([]map[string]any, 0, len(batch.New))
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("listener result is not a row: %#v", result)
			}
			rows = append(rows, map[string]any{"s0volume": row.Get("s0volume").Any(), "s1volume": row.Get("s1volume").Any()})
		}
		batches = append(batches, rows)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send("IBM", 10)
	send("CSCO", 10)
	assertRows(t, batches[0], map[string]any{"s0volume": int64(10), "s1volume": int64(10)})
	assertRows(t, rows(t, statement), map[string]any{"s0volume": int64(10), "s1volume": int64(10)})
	if err := deployment.Undeploy(ctx); err != nil {
		t.Fatal(err)
	}

	// Second deployment: fresh state, IBM-only and then CSCO mismatch leave it empty.
	statement, deployment = deploy()
	var secondBatches [][]map[string]any
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows := make([]map[string]any, 0, len(batch.New))
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("listener result is not a row: %#v", result)
			}
			rows = append(rows, map[string]any{"s0volume": row.Get("s0volume").Any(), "s1volume": row.Get("s1volume").Any()})
		}
		secondBatches = append(secondBatches, rows)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send("IBM", 20)
	assertRows(t, rows(t, statement))
	send("CSCO", 30)
	assertRows(t, rows(t, statement))
	if len(secondBatches) != 0 {
		t.Fatalf("second deployment batches = %#v, want none", secondBatches)
	}
	if err := deployment.Undeploy(ctx); err != nil {
		t.Fatal(err)
	}

	// Third deployment: matching pair again produces exactly one row.
	statement, deployment = deploy()
	var thirdBatches [][]map[string]any
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows := make([]map[string]any, 0, len(batch.New))
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("listener result is not a row: %#v", result)
			}
			rows = append(rows, map[string]any{"s0volume": row.Get("s0volume").Any(), "s1volume": row.Get("s1volume").Any()})
		}
		thirdBatches = append(thirdBatches, rows)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send("IBM", 50)
	send("CSCO", 50)
	assertRows(t, thirdBatches[0], map[string]any{"s0volume": int64(50), "s1volume": int64(50)})
	assertRows(t, rows(t, statement), map[string]any{"s0volume": int64(50), "s1volume": int64(50)})
}
