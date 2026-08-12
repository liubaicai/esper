package esper

import (
	"context"
	"testing"
	"time"
)

func TestTimeOrderCalendarWindowUsesExactMonthBoundary(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2002, time.February, 1, 9, 0, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	timestamp := Field[externalTrade, int64]("timestamp")
	stream := From[externalTrade](env, "ExternalTrade").Window(TimeOrderCalendar(timestamp, 0, 1, 0))
	_, batches := deployViewTest(t, env, engine, stream, "time-order-calendar-month")

	if err := engine.SendEvent(context.Background(), externalTrade{Symbol: "E1", Timestamp: start.UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || len((*batches)[0].Old) != 0 {
		t.Fatalf("month-scoped insertion = %#v", *batches)
	}

	boundary := start.AddDate(0, 1, 0)
	if err := engine.AdvanceTime(context.Background(), boundary.Add(-time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 1 {
		t.Fatalf("month-scoped event expired before boundary: %#v", *batches)
	}
	if err := engine.AdvanceTime(context.Background(), boundary); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 2 || len((*batches)[1].New) != 0 || len((*batches)[1].Old) != 1 || eventSymbol((*batches)[1].Old[0]) != "E1" {
		t.Fatalf("month-scoped boundary expiry = %#v", *batches)
	}
}

func TestTimeOrderCalendarWindowRejectsInvalidPeriod(t *testing.T) {
	env, _ := newRuntimeTest(t)
	timestamp := Field[runtimeTestTrade, float64]("price")
	for name, window := range map[string]WindowSpec{
		"zero":     TimeOrderCalendar(timestamp, 0, 0, 0),
		"negative": TimeOrderCalendar(timestamp, -1, 0, 0),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(window).Query(StatementName("invalid-time-order-calendar-" + name))); err == nil {
				t.Fatal("expected invalid calendar period to be rejected")
			}
		})
	}
}

func TestGroupedTimeOrderWindowMatchesExpiryLifecycle(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[groupedExternalTrade](env, "GroupedExternalTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(20, 0).UTC()))
	groupID := Field[groupedExternalTrade, string]("groupId")
	timestamp := Field[groupedExternalTrade, int64]("timestamp")
	stream := From[groupedExternalTrade](env, "GroupedExternalTrade").Window(GroupWindow(groupID, TimeOrder(timestamp, 10*time.Second)))
	statement, batches := deployViewTest(t, env, engine, stream, "time-order-grouped")
	send := func(event groupedExternalTrade) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	advance := func(milliseconds int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(milliseconds).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	assertSnapshotSymbols := func(want []string) {
		t.Helper()
		snapshot, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Results()) != len(want) {
			t.Fatalf("grouped time-order snapshot = %#v, want %v", snapshot.Results(), want)
		}
		for index, result := range snapshot.Results() {
			if got := groupedEventSymbol(result); got != want[index] {
				t.Fatalf("grouped time-order snapshot[%d] = %q, want %q", index, got, want[index])
			}
		}
	}

	send(groupedExternalTrade{Symbol: "E1", GroupID: "G1", Timestamp: 10000})
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || len((*batches)[0].Old) != 1 {
		t.Fatalf("grouped immediate expiry = %#v", *batches)
	}
	assertSnapshotSymbols(nil)

	send(groupedExternalTrade{Symbol: "E2", GroupID: "G2", Timestamp: 10001})
	send(groupedExternalTrade{Symbol: "E3", GroupID: "G3", Timestamp: 20000})
	send(groupedExternalTrade{Symbol: "E4", GroupID: "G2", Timestamp: 20000})
	assertSnapshotSymbols([]string{"E2", "E4", "E3"})

	advance(20001)
	if len(*batches) != 5 || len((*batches)[4].Old) != 1 || groupedEventSymbol((*batches)[4].Old[0]) != "E2" {
		t.Fatalf("grouped first expiry = %#v", *batches)
	}
	assertSnapshotSymbols([]string{"E4", "E3"})

	advance(22000)
	send(groupedExternalTrade{Symbol: "E5", GroupID: "G2", Timestamp: 19000})
	assertSnapshotSymbols([]string{"E5", "E4", "E3"})

	advance(29000)
	if len(*batches) != 7 || len((*batches)[6].Old) != 1 || groupedEventSymbol((*batches)[6].Old[0]) != "E5" {
		t.Fatalf("grouped second expiry = %#v", *batches)
	}
	assertSnapshotSymbols([]string{"E4", "E3"})

	advance(30000)
	if len(*batches) != 8 || len((*batches)[7].Old) != 2 {
		t.Fatalf("grouped final expiry = %#v", *batches)
	}
	assertSnapshotSymbols(nil)
}

type groupedExternalTrade struct {
	Symbol    string `esper:"symbol"`
	GroupID   string `esper:"groupId"`
	Timestamp int64  `esper:"timestamp"`
}

func groupedEventSymbol(result Result) string {
	event, ok := result.Event()
	if !ok {
		return ""
	}
	trade, ok := event.Underlying().(groupedExternalTrade)
	if !ok {
		return ""
	}
	return trade.Symbol
}
