package esper

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestTableUpsertSnapshotAndIndexes(t *testing.T) {
	env := NewEnvironment()
	columns := []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}
	if _, err := CreateTable(env, "latest", columns, UniqueIndex("price-index", "price")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	table, ok := engine.Table("latest")
	if !ok {
		t.Fatal("table was not created")
	}
	ctx := context.Background()
	if _, err := table.Insert(ctx, map[string]any{"symbol": "A", "price": 10.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Insert(ctx, map[string]any{"symbol": "B", "price": 10.0}); err == nil {
		t.Fatal("unique secondary index accepted duplicate")
	} else if !errors.Is(err, ErrorState) {
		t.Fatalf("duplicate index error = %v", err)
	}
	if _, err := table.Update(ctx, []any{"A"}, map[string]any{"price": 11.0}); err != nil {
		t.Fatal(err)
	}
	row, found, err := table.Get(ctx, "A")
	if err != nil || !found || !row.Get("price").Equal(Present(11.0)) {
		t.Fatalf("updated row = %#v, found=%v, err=%v", row, found, err)
	}
	rows, err := table.Lookup(ctx, "price-index", 11.0)
	if err != nil || len(rows) != 1 || !rows[0].Get("symbol").Equal(Present("A")) {
		t.Fatalf("index lookup = %#v, err=%v", rows, err)
	}
	snapshot, err := table.Snapshot(ctx)
	if err != nil || len(snapshot) != 1 {
		t.Fatalf("table snapshot = %#v, err=%v", snapshot, err)
	}
	values := snapshot[0].Values()
	values["price"] = Present(999.0)
	current, _, _ := table.Get(ctx, "A")
	if current.Get("price").Equal(Present(999.0)) {
		t.Fatal("mutating table snapshot changed table state")
	}
	if _, deleted, err := table.Delete(ctx, "A"); err != nil || !deleted {
		t.Fatalf("delete = deleted=%v, err=%v", deleted, err)
	}
}

func TestTableKeyLookupCoercesNumericPrimaryKey(t *testing.T) {
	env := NewEnvironment()
	if _, err := CreateTable(env, "numeric-keys", []TableColumn{
		PrimaryKeyColumn[int64]("id"),
		TableColumnOf[string]("value"),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	table, ok := engine.Table("numeric-keys")
	if !ok {
		t.Fatal("numeric-keys table is missing")
	}
	if _, err := table.Insert(context.Background(), map[string]any{"id": int64(7), "value": "ok"}); err != nil {
		t.Fatal(err)
	}
	row, found, err := table.Get(context.Background(), int(7))
	if err != nil || !found || row.Get("value").Any() != "ok" {
		t.Fatalf("coerced numeric key lookup = %#v, found=%v, err=%v", row.Values(), found, err)
	}
}

func TestTableReplaceIsAtomicAcrossValidationAndCancellation(t *testing.T) {
	env := NewEnvironment()
	if _, err := CreateTable(env, "replace-atomic", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	table, ok := NewEngine(env).Table("replace-atomic")
	if !ok {
		t.Fatal("replace-atomic table is missing")
	}
	ctx := context.Background()
	if _, err := table.Insert(ctx, map[string]any{"symbol": "A", "price": 1.0}); err != nil {
		t.Fatal(err)
	}
	if err := table.Replace(ctx, []map[string]any{{"symbol": "B"}}); err == nil {
		t.Fatal("invalid table replacement succeeded")
	}
	rows, err := table.Snapshot(ctx)
	if err != nil || len(rows) != 1 || rows[0].Get("symbol").Any() != "A" {
		t.Fatalf("failed replacement changed table = %#v, err=%v", rows, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := table.Replace(cancelled, []map[string]any{{"symbol": "C", "price": 3.0}}); err == nil {
		t.Fatal("cancelled table replacement succeeded")
	}
	rows, err = table.Snapshot(ctx)
	if err != nil || len(rows) != 1 || rows[0].Get("symbol").Any() != "A" {
		t.Fatalf("cancelled replacement changed table = %#v, err=%v", rows, err)
	}
}

func TestTableSourceFireAndForgetReadsConsistentRows(t *testing.T) {
	env := NewEnvironment()
	if _, err := CreateTable(env, "positions", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	table, ok := engine.Table("positions")
	if !ok {
		t.Fatal("positions table is missing")
	}
	for _, values := range []map[string]any{{"symbol": "A", "price": 4.0}, {"symbol": "B", "price": 9.0}} {
		if _, err := table.Insert(context.Background(), values); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := env.Build(FromTable(env, "positions").Filter(
		Greater[float64](Field[any, float64]("price"), Literal[float64](5)),
	).Query(StatementName("table-faf")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil || len(result.Results()) != 1 {
		t.Fatalf("table source FAF result = %#v, err=%v", result.Results(), err)
	}
	event, ok := result.Results()[0].Event()
	if !ok || event.TypeName() != "positions" || event.Get("symbol").Any() != "B" {
		t.Fatalf("table source event = %#v", result.Results()[0])
	}
}

func TestPreparedFireAndForgetBindsParametersPerExecution(t *testing.T) {
	env := NewEnvironment()
	if _, err := CreateTable(env, "positions", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	table, ok := engine.Table("positions")
	if !ok {
		t.Fatal("positions table is missing")
	}
	for _, values := range []map[string]any{
		{"symbol": "A", "price": 4.0},
		{"symbol": "B", "price": 9.0},
	} {
		if _, err := table.Insert(context.Background(), values); err != nil {
			t.Fatal(err)
		}
	}
	minimum := Parameter[float64]("minimum")
	plan, err := env.Build(FromTable(env, "positions").Filter(
		Greater[float64](Field[any, float64]("price"), minimum),
	).Query(StatementName("parameterized-table-faf")))
	if err != nil {
		t.Fatal(err)
	}
	withoutParameter, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err == nil {
		t.Fatalf("unbound parameter query returned %#v", withoutParameter.Results())
	}
	if _, err := engine.ExecuteFireAndForgetWithParameters(context.Background(), plan, ParameterValues{"other": 5.0}); err == nil {
		t.Fatal("parameterized FAF accepted an undeclared parameter")
	}
	prepared, err := engine.PrepareFireAndForget(plan)
	if err != nil {
		t.Fatal(err)
	}
	first, err := prepared.ExecuteWithParameters(context.Background(), ParameterValues{"minimum": 5.0})
	if err != nil || len(first.Results()) != 1 {
		t.Fatalf("first parameterized result = %#v, err=%v", first.Results(), err)
	}
	firstEvent, ok := first.Results()[0].Event()
	if !ok || firstEvent.Get("symbol").Any() != "B" {
		t.Fatalf("first parameterized event = %#v", first.Results()[0])
	}
	second, err := prepared.ExecuteWithParameters(context.Background(), ParameterValues{"minimum": 10.0})
	if err != nil || len(second.Results()) != 0 {
		t.Fatalf("second parameterized result = %#v, err=%v", second.Results(), err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.ExecuteWithParameters(context.Background(), ParameterValues{"minimum": 0.0}); err == nil {
		t.Fatal("closed prepared query accepted parameter execution")
	}
}

func TestParameterizedFireAndForgetValidatesTypesAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := CreateTable(env, "typed-positions", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
		TableColumnOf[int64]("quantity"),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	table, ok := engine.Table("typed-positions")
	if !ok {
		t.Fatal("typed-positions table is missing")
	}
	if _, err := table.Insert(context.Background(), map[string]any{"symbol": "A", "price": 4.0, "quantity": int64(2)}); err != nil {
		t.Fatal(err)
	}
	pricePlan, err := env.Build(FromTable(env, "typed-positions").Filter(
		Greater[float64](Field[any, float64]("price"), Parameter[float64]("minimum")),
	).Query(StatementName("typed-parameter")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ExecuteFireAndForgetWithParameters(context.Background(), pricePlan, ParameterValues{"minimum": 4}); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("incompatible parameter type error = %v", err)
	}
	if result, err := engine.ExecuteFireAndForgetWithParameters(context.Background(), pricePlan, ParameterValues{"minimum": nil}); err != nil || len(result.Results()) != 0 {
		t.Fatalf("nil parameter result = %#v, err=%v", result.Results(), err)
	}

	quantityPlan, err := env.Build(FromTable(env, "typed-positions").Filter(
		Greater[int64](Field[any, int64]("quantity"), Parameter[int64]("minimum")),
	).Query(StatementName("typed-parameter")))
	if err != nil {
		t.Fatal(err)
	}
	if pricePlan.Hash() == quantityPlan.Hash() {
		t.Fatal("parameter type changes must change the canonical plan identity")
	}

	if _, err := env.Build(SelectOnce(env,
		Alias("as-int", Parameter[int]("same")),
		Alias("as-string", Parameter[string]("same")),
	)); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("incompatible repeated parameter declaration error = %v", err)
	}
}

func TestDeployedStatementBindsImmutableParameters(t *testing.T) {
	env, engine := newRuntimeTest(t)
	minimum := Parameter[float64]("minimum")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Filter(
		Greater[float64](Field[runtimeTestTrade, float64]("price"), minimum),
	).Query(StatementName("deployed-parameters")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("unbound deployed parameter error = %v", err)
	}
	parameters := ParameterValues{"minimum": 5.0}
	deployment, err := engine.DeployWithParameters(context.Background(), plan, parameters)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	parameters["minimum"] = 100.0
	var observed []Event
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if ok {
				observed = append(observed, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 7}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if len(observed) != 1 || observed[0].Get("symbol").Any() != "A" {
		t.Fatalf("deployed parameter results = %#v", observed)
	}
	if _, err := engine.DeployWithParameters(context.Background(), plan, ParameterValues{"minimum": 5}); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("deployed parameter type error = %v", err)
	}
}

func TestParameterizedNamedWindowFireAndForgetSupportsSliceAndCancellation(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "parameterized-trades", schema); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromNamedWindow(env, "parameterized-trades").Filter(
		InSlice[string](Field[any, string]("symbol"), Parameter[[]string]("symbols")),
	).Query(StatementName("parameterized-named-window-faf")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, trade := range []runtimeTestTrade{
		{Symbol: "A", Price: 1},
		{Symbol: "B", Price: 2},
		{Symbol: "C", Price: 3},
	} {
		if err := engine.InsertNamedWindow(context.Background(), "parameterized-trades", trade); err != nil {
			t.Fatal(err)
		}
	}
	prepared, err := engine.PrepareFireAndForget(plan)
	if err != nil {
		t.Fatal(err)
	}
	result, err := prepared.ExecuteWithParameters(context.Background(), ParameterValues{"symbols": []string{"C", "A"}})
	if err != nil || len(result.Results()) != 2 {
		t.Fatalf("parameterized named-window result = %#v, err=%v", result.Results(), err)
	}
	if !resultHasSymbol(result.Results()[0], "A") {
		t.Fatalf("first parameterized named-window result = %#v", result.Results()[0])
	}
	if !resultHasSymbol(result.Results()[1], "C") {
		t.Fatalf("second parameterized named-window result = %#v", result.Results()[1])
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := engine.ExecuteFireAndForgetWithParameters(cancelled, plan, ParameterValues{"symbols": []string{"A"}}); err == nil {
		t.Fatal("cancelled parameterized FAF query succeeded")
	}
}

func resultHasSymbol(result Result, symbol string) bool {
	event, ok := result.Event()
	return ok && event.Get("symbol").Any() == symbol
}

func TestNamedWindowLengthAndTimeRetention(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "trades", envSchema(t, env, "Trade"), NamedWindowRetention(LengthWindow(2))); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "live", envSchema(t, env, "Trade")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	livePlan, err := env.Build(FromNamedWindow(env, "live").Filter(
		Equal[string](Field[any, string]("symbol"), Literal("keep")),
	).Query(StatementName("named-window-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	liveDeployment, err := engine.Deploy(context.Background(), livePlan)
	if err != nil {
		t.Fatal(err)
	}
	var liveBatches []ResultBatch
	if _, err := liveDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		liveBatches = append(liveBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("trades")
	if !ok {
		t.Fatal("named window was not created")
	}
	var deltas []NamedWindowDelta
	if _, err := window.Subscribe(func(_ context.Context, delta NamedWindowDelta) error {
		copyDelta := NamedWindowDelta{New: append([]Event(nil), delta.New...), Old: append([]Event(nil), delta.Old...), Time: delta.Time}
		deltas = append(deltas, copyDelta)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"A", "B", "C"} {
		if err := engine.InsertNamedWindow(context.Background(), "trades", runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.InsertNamedWindow(context.Background(), "live", runtimeTestTrade{Symbol: "drop"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "live", runtimeTestTrade{Symbol: "keep"}); err != nil {
		t.Fatal(err)
	}
	if len(liveBatches) != 1 || len(liveBatches[0].New) != 1 {
		t.Fatalf("named-window consumer batches = %#v", liveBatches)
	}
	fafPlan, err := env.Build(FromNamedWindow(env, "live").Query(StatementName("faf")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), fafPlan)
	if err != nil || len(result.Results()) != 2 {
		t.Fatalf("fire-and-forget result = %#v, err=%v", result, err)
	}
	prepared, err := engine.PrepareFireAndForget(fafPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Execute(context.Background()); err == nil || !errors.Is(err, ErrorState) {
		t.Fatalf("closed prepared query error = %v", err)
	}
	if len(deltas) != 3 || len(deltas[2].Old) != 1 {
		t.Fatalf("named-window deltas = %#v", deltas)
	}
	events, err := window.Snapshot(context.Background())
	if err != nil || len(events) != 2 || events[0].Underlying().(runtimeTestTrade).Symbol != "B" {
		t.Fatalf("named-window snapshot = %#v, err=%v", events, err)
	}

	clockEnv := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](clockEnv, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, _ := clockEnv.Schema("Trade")
	if _, err := CreateNamedWindow(clockEnv, "timed", schema, NamedWindowRetention(TimeWindow(time.Second))); err != nil {
		t.Fatal(err)
	}
	clockEngine := NewEngine(clockEnv, WithStartTime(time.Unix(0, 0).UTC()))
	timed, _ := clockEngine.NamedWindow("timed")
	if err := clockEngine.InsertNamedWindow(context.Background(), "timed", runtimeTestTrade{Symbol: "T"}); err != nil {
		t.Fatal(err)
	}
	if err := clockEngine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	remaining, err := timed.Snapshot(context.Background())
	if err != nil || len(remaining) != 0 {
		t.Fatalf("timed named-window snapshot = %#v, err=%v", remaining, err)
	}
}

func TestNamedWindowTimeToLiveAtUsesAbsoluteEventExpiry(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "ExternalTrade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("ExternalTrade")
	if !ok {
		t.Fatal("external-trade schema was not registered")
	}
	expiry := Field[externalTrade, int64]("timestamp")
	if _, err := CreateNamedWindow(env, "ttl-at", schema, NamedWindowRetention(TimeToLiveAt(expiry))); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	window, ok := engine.NamedWindow("ttl-at")
	if !ok {
		t.Fatal("time-to-live-at named window was not created")
	}
	var deltas []NamedWindowDelta
	if _, err := window.Subscribe(func(_ context.Context, delta NamedWindowDelta) error {
		deltas = append(deltas, NamedWindowDelta{
			New:  append([]Event(nil), delta.New...),
			Old:  append([]Event(nil), delta.Old...),
			Time: delta.Time,
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	insert := func(symbol string, timestamp int64) {
		t.Helper()
		if err := engine.InsertNamedWindow(context.Background(), "ttl-at", externalTrade{Symbol: symbol, Timestamp: timestamp}); err != nil {
			t.Fatal(err)
		}
	}
	insert("E1", 1000)
	insert("E2", 500)
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(500).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 3 || len(deltas[2].Old) != 1 || eventSymbol(resultEvent(deltas[2].Old[0])) != "E2" {
		t.Fatalf("named-window first expiry = %#v", deltas)
	}
	insert("E3", 200)
	if len(deltas) != 4 || len(deltas[3].New) != 1 || len(deltas[3].Old) != 1 || eventSymbol(resultEvent(deltas[3].Old[0])) != "E3" {
		t.Fatalf("named-window expired insertion = %#v", deltas)
	}
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(1000).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 5 || len(deltas[4].Old) != 1 || eventSymbol(resultEvent(deltas[4].Old[0])) != "E1" {
		t.Fatalf("named-window second expiry = %#v", deltas)
	}
	remaining, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("named-window remaining = %#v", remaining)
	}
}

func TestNamedWindowCanParticipateInJoinMany(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinPayment](env, "Payment"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := NewMapSchema("LiveOrder", []FieldSpec{
		{Name: "orderID", Type: reflect.TypeOf("")},
		{Name: "symbol", Type: reflect.TypeOf("")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "live-orders", windowSchema); err != nil {
		t.Fatal(err)
	}
	joined := JoinMany(
		JoinRecordSource(FromNamedWindow(env, "live-orders")),
		JoinSource(From[joinPayment](env, "Payment")),
	).On(OnSourcesEqual(0, Field[any, string]("orderID"), 1, Field[joinPayment, string]("orderID")))
	plan, err := env.Build(joined.Select(
		SelectFrom(0, "symbol", Field[any, string]("symbol")),
		SelectFrom(1, "amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("named-window-join")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
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
	if err := engine.InsertNamedWindow(context.Background(), "live-orders", map[string]any{"orderID": "O-5", "symbol": "ESPER"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "Payment", joinPayment{OrderID: "O-5", Amount: 7}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("named-window join batches = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("symbol").Any() != "ESPER" || row.Get("amount").Any() != float64(7) {
		t.Fatalf("named-window join row = %#v", row.AsMap())
	}
}

func envSchema(t *testing.T, env *Environment, name string) Schema {
	t.Helper()
	schema, ok := env.Schema(name)
	if !ok {
		t.Fatalf("schema %q is not registered", name)
	}
	return schema
}
