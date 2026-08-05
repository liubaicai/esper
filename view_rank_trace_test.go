package esper

import (
	"context"
	"testing"
)

func TestRankWindowRankedSceneMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	symbol := Field[externalTrade, string]("symbol")
	price := Field[externalTrade, float64]("price")
	statement, batches := deployViewTest(t, env, engine, From[externalTrade](env, "ExternalTrade").Window(
		RankWindowBy(4, []Expr{symbol}, Descending(price)),
	), "rank-ranked-scene")

	assertTrade := func(result Result, want externalTrade) {
		t.Helper()
		event, ok := result.Event()
		if !ok {
			t.Fatalf("rank result is not an event: %#v", result)
		}
		got, ok := event.Underlying().(externalTrade)
		if !ok || got != want {
			t.Fatalf("rank result = %#v, want %#v", event.Underlying(), want)
		}
	}
	assertSnapshot := func(want ...externalTrade) {
		t.Helper()
		snapshot, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Results()) != len(want) {
			t.Fatalf("rank snapshot = %#v, want %#v", snapshot.Results(), want)
		}
		for index, result := range snapshot.Results() {
			assertTrade(result, want[index])
		}
	}
	send := func(trade externalTrade, wantOld ...externalTrade) {
		t.Helper()
		before := len(*batches)
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
		if len(*batches) != before+1 {
			t.Fatalf("rank batch count = %d, want %d", len(*batches), before+1)
		}
		batch := (*batches)[len(*batches)-1]
		if len(batch.New) != 1 || len(batch.Old) != len(wantOld) {
			t.Fatalf("rank batch = %#v, want one new and %d old", batch, len(wantOld))
		}
		assertTrade(batch.New[0], trade)
		for index, want := range wantOld {
			assertTrade(batch.Old[index], want)
		}
	}

	send(externalTrade{Symbol: "E1", Price: 10}, nil...)
	assertSnapshot(externalTrade{Symbol: "E1", Price: 10})
	send(externalTrade{Symbol: "E2", Price: 30}, nil...)
	assertSnapshot(
		externalTrade{Symbol: "E2", Price: 30},
		externalTrade{Symbol: "E1", Price: 10},
	)
	send(externalTrade{Symbol: "E1", Price: 50}, externalTrade{Symbol: "E1", Price: 10})
	assertSnapshot(externalTrade{Symbol: "E1", Price: 50}, externalTrade{Symbol: "E2", Price: 30})
	send(externalTrade{Symbol: "E3", Price: 40}, nil...)
	assertSnapshot(
		externalTrade{Symbol: "E1", Price: 50},
		externalTrade{Symbol: "E3", Price: 40},
		externalTrade{Symbol: "E2", Price: 30},
	)
	send(externalTrade{Symbol: "E2", Price: 45}, externalTrade{Symbol: "E2", Price: 30})
	assertSnapshot(
		externalTrade{Symbol: "E1", Price: 50},
		externalTrade{Symbol: "E2", Price: 45},
		externalTrade{Symbol: "E3", Price: 40},
	)
	send(externalTrade{Symbol: "E1", Price: 43}, externalTrade{Symbol: "E1", Price: 50})
	assertSnapshot(
		externalTrade{Symbol: "E2", Price: 45},
		externalTrade{Symbol: "E1", Price: 43},
		externalTrade{Symbol: "E3", Price: 40},
	)
	send(externalTrade{Symbol: "E3", Price: 50}, externalTrade{Symbol: "E3", Price: 40})
	assertSnapshot(
		externalTrade{Symbol: "E3", Price: 50},
		externalTrade{Symbol: "E2", Price: 45},
		externalTrade{Symbol: "E1", Price: 43},
	)
	send(externalTrade{Symbol: "E3", Price: 10}, externalTrade{Symbol: "E3", Price: 50})
	assertSnapshot(
		externalTrade{Symbol: "E2", Price: 45},
		externalTrade{Symbol: "E1", Price: 43},
		externalTrade{Symbol: "E3", Price: 10},
	)
	send(externalTrade{Symbol: "E4", Price: 43}, nil...)
	assertSnapshot(
		externalTrade{Symbol: "E2", Price: 45},
		externalTrade{Symbol: "E1", Price: 43},
		externalTrade{Symbol: "E4", Price: 43},
		externalTrade{Symbol: "E3", Price: 10},
	)
	send(externalTrade{Symbol: "E4", Price: 43, Timestamp: 1}, externalTrade{Symbol: "E4", Price: 43})
	assertSnapshot(
		externalTrade{Symbol: "E2", Price: 45},
		externalTrade{Symbol: "E1", Price: 43},
		externalTrade{Symbol: "E4", Price: 43, Timestamp: 1},
		externalTrade{Symbol: "E3", Price: 10},
	)
	send(externalTrade{Symbol: "E2", Price: 45, Timestamp: 1}, externalTrade{Symbol: "E2", Price: 45})
	assertSnapshot(
		externalTrade{Symbol: "E2", Price: 45, Timestamp: 1},
		externalTrade{Symbol: "E1", Price: 43},
		externalTrade{Symbol: "E4", Price: 43, Timestamp: 1},
		externalTrade{Symbol: "E3", Price: 10},
	)
	send(externalTrade{Symbol: "E1", Price: 43, Timestamp: 1}, externalTrade{Symbol: "E1", Price: 43})
	assertSnapshot(
		externalTrade{Symbol: "E2", Price: 45, Timestamp: 1},
		externalTrade{Symbol: "E4", Price: 43, Timestamp: 1},
		externalTrade{Symbol: "E1", Price: 43, Timestamp: 1},
		externalTrade{Symbol: "E3", Price: 10},
	)
	send(externalTrade{Symbol: "E5", Price: 10, Timestamp: 2}, externalTrade{Symbol: "E3", Price: 10})
	assertSnapshot(
		externalTrade{Symbol: "E2", Price: 45, Timestamp: 1},
		externalTrade{Symbol: "E4", Price: 43, Timestamp: 1},
		externalTrade{Symbol: "E1", Price: 43, Timestamp: 1},
		externalTrade{Symbol: "E5", Price: 10, Timestamp: 2},
	)
	send(externalTrade{Symbol: "E5", Price: 11, Timestamp: 3}, externalTrade{Symbol: "E5", Price: 10, Timestamp: 2})
	assertSnapshot(
		externalTrade{Symbol: "E2", Price: 45, Timestamp: 1},
		externalTrade{Symbol: "E4", Price: 43, Timestamp: 1},
		externalTrade{Symbol: "E1", Price: 43, Timestamp: 1},
		externalTrade{Symbol: "E5", Price: 11, Timestamp: 3},
	)
	send(externalTrade{Symbol: "E6", Price: 43}, externalTrade{Symbol: "E5", Price: 11, Timestamp: 3})
	assertSnapshot(
		externalTrade{Symbol: "E2", Price: 45, Timestamp: 1},
		externalTrade{Symbol: "E4", Price: 43, Timestamp: 1},
		externalTrade{Symbol: "E1", Price: 43, Timestamp: 1},
		externalTrade{Symbol: "E6", Price: 43},
	)
	send(externalTrade{Symbol: "E7", Price: 50}, externalTrade{Symbol: "E4", Price: 43, Timestamp: 1})
	assertSnapshot(
		externalTrade{Symbol: "E7", Price: 50},
		externalTrade{Symbol: "E2", Price: 45, Timestamp: 1},
		externalTrade{Symbol: "E1", Price: 43, Timestamp: 1},
		externalTrade{Symbol: "E6", Price: 43},
	)
	send(externalTrade{Symbol: "E8", Price: 45}, externalTrade{Symbol: "E1", Price: 43, Timestamp: 1})
	assertSnapshot(
		externalTrade{Symbol: "E7", Price: 50},
		externalTrade{Symbol: "E2", Price: 45, Timestamp: 1},
		externalTrade{Symbol: "E8", Price: 45},
		externalTrade{Symbol: "E6", Price: 43},
	)
	send(externalTrade{Symbol: "E8", Price: 46, Timestamp: 1}, externalTrade{Symbol: "E8", Price: 45})
	assertSnapshot(
		externalTrade{Symbol: "E7", Price: 50},
		externalTrade{Symbol: "E8", Price: 46, Timestamp: 1},
		externalTrade{Symbol: "E2", Price: 45, Timestamp: 1},
		externalTrade{Symbol: "E6", Price: 43},
	)
}
