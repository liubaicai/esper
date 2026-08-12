package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestOutputAtSnapshotRebuildsAggregateState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	policy := OutputAt(CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}, OutputSnapshot())
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(5)).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("count", CountAll()),
		Alias("sum", Sum[float64](price)),
	).Query(StatementName("aggregate-output-snapshot"), WithOutput(policy)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 2},
		{Symbol: "A", Price: 3},
		{Symbol: "B", Price: 5},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	assertAggregateSnapshotRows(t, batches, map[string]aggregateSnapshotExpectation{
		"A": {Count: 2, Sum: 5},
		"B": {Count: 1, Sum: 5},
	})

	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 4}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 30, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	assertAggregateSnapshotRows(t, batches, map[string]aggregateSnapshotExpectation{
		"A": {Count: 3, Sum: 9},
		"B": {Count: 1, Sum: 5},
	})
}

type aggregateSnapshotExpectation struct {
	Count int64
	Sum   float64
}

func assertAggregateSnapshotRows(t *testing.T, batches []ResultBatch, expected map[string]aggregateSnapshotExpectation) {
	t.Helper()
	if len(batches) == 0 {
		t.Fatal("aggregate snapshot did not emit")
	}
	batch := batches[len(batches)-1]
	if len(batch.New) != len(expected) || len(batch.Old) != 0 {
		t.Fatalf("aggregate snapshot batch = %#v", batch)
	}
	seen := make(map[string]struct{}, len(batch.New))
	for _, result := range batch.New {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("aggregate snapshot result is not a row: %#v", result)
		}
		symbol, ok := row.Get("symbol").Any().(string)
		if !ok {
			t.Fatalf("aggregate snapshot symbol = %#v", row.Get("symbol"))
		}
		want, ok := expected[symbol]
		if !ok {
			t.Fatalf("unexpected aggregate snapshot group %q", symbol)
		}
		if got := row.Get("count").Any(); got != want.Count {
			t.Fatalf("aggregate snapshot %s count = %#v, want %d", symbol, got, want.Count)
		}
		if got := row.Get("sum").Any(); got != want.Sum {
			t.Fatalf("aggregate snapshot %s sum = %#v, want %v", symbol, got, want.Sum)
		}
		seen[symbol] = struct{}{}
	}
	if len(seen) != len(expected) {
		t.Fatalf("aggregate snapshot groups = %#v, want %#v", seen, expected)
	}
}

func TestOutputAtSnapshotRebuildsJoinState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	policy := OutputAt(CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}, OutputSnapshot())
	query := Join(
		From[joinOrder](env, "Order").Window(KeepAll()),
		From[joinPayment](env, "Payment").Window(KeepAll()),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).Select(
		SelectLeft("symbol", Field[joinOrder, string]("symbol")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("join-output-snapshot"), WithOutput(policy))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), joinOrder{OrderID: "O-1", Symbol: "ESPER"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), joinPayment{OrderID: "O-1", Amount: 9.5}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		t.Fatalf("join snapshot batch = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("symbol").Any() != "ESPER" || row.Get("amount").Any() != 9.5 {
		t.Fatalf("join snapshot row = %#v", row)
	}
}

func TestOutputSnapshotEveryRebuildsCurrentAggregateState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(WithOutput(OutputSnapshotEvery(0)))); err == nil {
		t.Fatal("zero snapshot-every interval was accepted")
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(WithOutput(OutputPolicy{Kind: OutputSnapshotPolicy, Snapshot: true}))); err == nil {
		t.Fatal("snapshot flag on a non-time policy was accepted")
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(KeepAll()).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("count", CountAll()),
		Alias("sum", Sum[float64](price)),
	).Query(StatementName("aggregate-snapshot-every"), WithOutput(OutputSnapshotEvery(time.Second))))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 2}, {Symbol: "A", Price: 3}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	assertAggregateSnapshotRows(t, batches, map[string]aggregateSnapshotExpectation{"A": {Count: 2, Sum: 5}})
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 4}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	assertAggregateSnapshotRows(t, batches, map[string]aggregateSnapshotExpectation{"A": {Count: 3, Sum: 9}})
}

func TestOutputSnapshotEveryEventsRebuildsCurrentAggregateState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(WithOutput(OutputSnapshotEveryEvents(0)))); err == nil {
		t.Fatal("zero snapshot-every-events count was accepted")
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(KeepAll()).GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("count", CountAll()),
		Alias("sum", Sum[float64](price)),
	).Query(StatementName("aggregate-snapshot-every-events"), WithOutput(OutputSnapshotEveryEvents(3))))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 2},
		{Symbol: "A", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 0 {
		t.Fatalf("snapshot emitted before event threshold: %#v", batches)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 4}); err != nil {
		t.Fatal(err)
	}
	assertAggregateSnapshotRows(t, batches, map[string]aggregateSnapshotExpectation{"A": {Count: 3, Sum: 9}})
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 5}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("event snapshot emitted after one event in next period: %#v", batches)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 6}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 7}); err != nil {
		t.Fatal(err)
	}
	assertAggregateSnapshotRows(t, batches, map[string]aggregateSnapshotExpectation{"A": {Count: 6, Sum: 27}})
}

func TestOutputAtSnapshotRebuildsNamedWindowState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "snapshot-trades", schema); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	policy := OutputAt(CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}, OutputSnapshot())
	stream := FromNamedWindow(env, "snapshot-trades").Select(
		Alias("symbol", Field[any, string]("symbol")),
		Alias("price", Field[any, float64]("price")),
	)
	plan, err := env.Build(stream.Query(StatementName("named-window-output-snapshot"), WithOutput(policy)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "snapshot-trades", runtimeTestTrade{Symbol: "A", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	assertSnapshotSymbols(t, batches, []string{"A"})
	window, ok := engine.NamedWindow("snapshot-trades")
	if !ok {
		t.Fatal("snapshot-trades window is missing")
	}
	if _, err := window.DeleteWhere(context.Background(), func(event Event) bool {
		return event.Get("symbol").Any() == "A"
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "snapshot-trades", runtimeTestTrade{Symbol: "B", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 30, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	assertSnapshotSymbols(t, batches, []string{"B"})
}

func TestOutputAtSnapshotRebuildsTableState(t *testing.T) {
	env := NewEnvironment()
	if _, err := env.RegisterTable("snapshot-positions", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	policy := OutputAt(CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}, OutputSnapshot())
	plan, err := env.Build(FromTable(env, "snapshot-positions").Select(
		Alias("symbol", Field[any, string]("symbol")),
		Alias("price", Field[any, float64]("price")),
	).Query(StatementName("table-output-snapshot"), WithOutput(policy)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	table, ok := engine.Table("snapshot-positions")
	if !ok {
		t.Fatal("snapshot-positions table is missing")
	}
	if _, err := table.Insert(context.Background(), map[string]any{"symbol": "A", "price": 4.5}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	assertSnapshotSymbols(t, batches, []string{"A"})
	if _, err := table.Upsert(context.Background(), map[string]any{"symbol": "B", "price": 7.5}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 30, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	assertSnapshotSymbols(t, batches, []string{"A", "B"})
}

func TestOutputAtSnapshotRebuildsTableAggregateState(t *testing.T) {
	env := NewEnvironment()
	if _, err := env.RegisterTable("snapshot-aggregate-positions", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	policy := OutputAt(CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}, OutputSnapshot())
	symbol := Field[any, string]("symbol")
	price := Field[any, float64]("price")
	plan, err := env.Build(FromTable(env, "snapshot-aggregate-positions").GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("count", CountAll()),
		Alias("sum", Sum[float64](price)),
	).Query(StatementName("table-aggregate-output-snapshot"), WithOutput(policy)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	table, ok := engine.Table("snapshot-aggregate-positions")
	if !ok {
		t.Fatal("snapshot-aggregate-positions table is missing")
	}
	for _, row := range []map[string]any{
		{"symbol": "A", "price": 2.0},
		{"symbol": "A2", "price": 3.0},
		{"symbol": "B", "price": 5.0},
	} {
		if _, err := table.Upsert(context.Background(), row); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 3 {
		t.Fatalf("table aggregate snapshot batch = %#v", batches)
	}
}

func TestOutputAtSnapshotRebuildsTableJoinState(t *testing.T) {
	env := NewEnvironment()
	if _, err := env.RegisterTable("snapshot-left", []TableColumn{
		PrimaryKeyColumn[string]("orderID"),
		TableColumnOf[string]("symbol"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterTable("snapshot-right", []TableColumn{
		PrimaryKeyColumn[string]("orderID"),
		TableColumnOf[float64]("amount"),
	}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	policy := OutputAt(CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}, OutputSnapshot())
	orderID := Field[any, string]("orderID")
	joined := JoinMany(
		JoinRecordSource(FromTable(env, "snapshot-left")),
		JoinRecordSource(FromTable(env, "snapshot-right")),
	).On(OnSourcesEqual(0, orderID, 1, orderID)).Select(
		SelectFrom(0, "symbol", Field[any, string]("symbol")),
		SelectFrom(1, "amount", Field[any, float64]("amount")),
	)
	plan, err := env.Build(joined.Query(StatementName("table-join-output-snapshot"), WithOutput(policy)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	left, ok := engine.Table("snapshot-left")
	if !ok {
		t.Fatal("snapshot-left table is missing")
	}
	right, ok := engine.Table("snapshot-right")
	if !ok {
		t.Fatal("snapshot-right table is missing")
	}
	if _, err := left.Upsert(context.Background(), map[string]any{"orderID": "O-1", "symbol": "ESPER"}); err != nil {
		t.Fatal(err)
	}
	if _, err := right.Upsert(context.Background(), map[string]any{"orderID": "O-1", "amount": 9.5}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		t.Fatalf("table join snapshot batch = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("symbol").Any() != "ESPER" || row.Get("amount").Any() != 9.5 {
		t.Fatalf("table join snapshot row = %#v", batches[0].New[0])
	}
}

func TestOutputSnapshotEveryRebuildsJoinAggregateState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "Order"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	plan, err := env.Build(Join(
		From[joinOrder](env, "Order").Window(TimeWindow(10*time.Second)),
		From[joinPayment](env, "Payment").Window(KeepAll()),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).Aggregate(
		Alias("count", CountAll()),
	).Query(StatementName("join-aggregate-output-snapshot"), WithOutput(OutputSnapshotEvery(time.Second))))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-1", Amount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Order", joinOrder{OrderID: "O-1", Symbol: "ESPER"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	assertJoinAggregateSnapshotCount(t, batches, 1)
	if err := engine.Send(context.Background(), "Order", joinOrder{OrderID: "O-1", Symbol: "ESPER-2"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	assertJoinAggregateSnapshotCount(t, batches, 2)
}

func assertJoinAggregateSnapshotCount(t *testing.T, batches []ResultBatch, expected int64) {
	t.Helper()
	if len(batches) == 0 || len(batches[len(batches)-1].New) != 1 {
		t.Fatalf("join aggregate snapshot batches = %#v", batches)
	}
	row, ok := batches[len(batches)-1].New[0].Row()
	if !ok || row.Get("count").Any() != expected {
		t.Fatalf("join aggregate snapshot row = %#v, want count %d", batches[len(batches)-1].New[0], expected)
	}
}

func TestOutputAtSnapshotRebuildsContextNamedWindowState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "context-snapshot-trades", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "snapshot-by-symbol", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	policy := OutputAt(CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}, OutputSnapshot())
	plan, err := env.Build(FromNamedWindow(env, "context-snapshot-trades").Select(
		Alias("symbol", Field[any, string]("symbol")),
		Alias("price", Field[any, float64]("price")),
	).Query(StatementName("context-named-window-output-snapshot"), WithContext("snapshot-by-symbol"), WithOutput(policy)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 2}, {Symbol: "B", Price: 3}} {
		if err := engine.InsertNamedWindow(context.Background(), "context-snapshot-trades", event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	assertSnapshotSymbols(t, batches, []string{"A", "B"})
}

func TestOutputAtSnapshotRebuildsContextWindowState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "window-snapshot-by-symbol", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	policy := OutputAt(CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}, OutputSnapshot())
	plan, err := env.Build(Select(From[runtimeTestTrade](env, "Trade").Window(KeepAll()),
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
		Alias("price", Field[runtimeTestTrade, float64]("price")),
	).Query(StatementName("context-window-output-snapshot"), WithContext("window-snapshot-by-symbol"), WithOutput(policy)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 2}, {Symbol: "B", Price: 3}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	assertSnapshotSymbols(t, batches, []string{"A", "B"})
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 4}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 30, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	assertSnapshotSymbols(t, batches, []string{"A", "A", "B"})
}

func assertSnapshotSymbols(t *testing.T, batches []ResultBatch, expected []string) {
	t.Helper()
	if len(batches) == 0 {
		t.Fatal("snapshot did not emit")
	}
	batch := batches[len(batches)-1]
	if len(batch.New) != len(expected) || len(batch.Old) != 0 {
		t.Fatalf("snapshot batch = %#v", batch)
	}
	got := make([]string, 0, len(batch.New))
	for _, result := range batch.New {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("snapshot result is not a row: %#v", result)
		}
		symbol, ok := row.Get("symbol").Any().(string)
		if !ok {
			t.Fatalf("snapshot symbol = %#v", row.Get("symbol"))
		}
		got = append(got, symbol)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("snapshot symbols = %#v, want %#v", got, expected)
	}
}
