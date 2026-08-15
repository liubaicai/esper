package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type rowPerGroupBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
	IntBoxed      int    `esper:"intBoxed"`
}

func (bean rowPerGroupBean) TheStringValue() string { return bean.TheString }

type rowPerGroupString struct {
	TheString string `esper:"theString"`
}

type rowPerGroupBeanA struct {
	ID string `esper:"id"`
}

type rowPerGroupIntArray struct {
	Value int   `esper:"value"`
	Array []int `esper:"array"`
}

type rowPerGroupNullEvent struct {
	ID       string `esper:"id"`
	Value    int    `esper:"value"`
	GroupKey any    `esper:"groupkey"`
}

func newRowPerGroupEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[rowPerGroupBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[viewTimeWinMarket](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[rowPerGroupIntArray](env, "SupportEventWithIntArray"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[rowPerGroupString](env, "SupportBeanString"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

func sendRowPerGroupBean(t *testing.T, engine *Engine, bean rowPerGroupBean) {
	t.Helper()
	if err := engine.Send(context.Background(), "SupportBean", bean); err != nil {
		t.Fatal(err)
	}
}

func sendRowPerGroupMarket(t *testing.T, engine *Engine, symbol string, price float64) {
	t.Helper()
	if err := engine.Send(context.Background(), "SupportMarketDataBean", viewTimeWinMarket{Symbol: symbol, Price: price}); err != nil {
		t.Fatal(err)
	}
}

func lastRowPerGroupNew(batches *[]ResultBatch, field string) []any {
	if len(*batches) == 0 {
		return nil
	}
	last := (*batches)[len(*batches)-1]
	values := make([]any, 0, len(last.New))
	for _, result := range last.New {
		values = append(values, result.Get(field).Any())
	}
	return values
}

func lastRowPerGroupOld(batches *[]ResultBatch, field string) []any {
	if len(*batches) == 0 {
		return nil
	}
	last := (*batches)[len(*batches)-1]
	values := make([]any, 0, len(last.Old))
	for _, result := range last.Old {
		values = append(values, result.Get(field).Any())
	}
	return values
}

// TestResultSetRowPerGroupSumOneViewParity covers
// ResultSetQueryTypeRowPerGroupSumOneView: irstream grouped sum/avg over a
// length(3) window with old rows on eviction.
func TestResultSetRowPerGroupSumOneViewParity(t *testing.T) {
	env, engine := newRowPerGroupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	origin := time.Unix(0, 0).UTC()
	engine = NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[viewTimeWinMarket, string]("symbol")
	price := Field[viewTimeWinMarket, float64]("price")
	plan, err := env.Build(From[viewTimeWinMarket](env, "SupportMarketDataBean").Window(LengthWindow(3)).
		Filter(Or(
			Equal[string](symbol, Literal("DELL")),
			Or(Equal[string](symbol, Literal("IBM")), Equal[string](symbol, Literal("GE"))),
		)).
		GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("mySum", Sum[float64](price)),
		Alias("myAvg", Avg[float64](price)),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := new([]ResultBatch)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendRowPerGroupMarket(t, engine, "DELL", 10)
	assertRowPerGroupSum(t, batches, "DELL", nil, nil, 10.0, 10.0)
	sendRowPerGroupMarket(t, engine, "DELL", 20)
	assertRowPerGroupSum(t, batches, "DELL", 10.0, 10.0, 30.0, 15.0)
	sendRowPerGroupMarket(t, engine, "DELL", 100)
	assertRowPerGroupSum(t, batches, "DELL", 30.0, 15.0, 130.0, 130.0/3.0)
	sendRowPerGroupMarket(t, engine, "DELL", 50)
	assertRowPerGroupSum(t, batches, "DELL", 130.0, 130.0/3.0, 170.0, 170.0/3.0)
	sendRowPerGroupMarket(t, engine, "DELL", 5)
	assertRowPerGroupSum(t, batches, "DELL", 170.0, 170.0/3.0, 155.0, 155.0/3.0)
	sendRowPerGroupMarket(t, engine, "AAA", 1000)
	assertRowPerGroupSum(t, batches, "DELL", 155.0, 155.0/3.0, 55.0, 27.5)
	sendRowPerGroupMarket(t, engine, "IBM", 70)
	// The AAA event leaves the DELL window group in the same batch as IBM.
	last := (*batches)[len(*batches)-1]
	if len(last.New) != 2 || len(last.Old) != 2 {
		t.Fatalf("IBM batch = %#v", last)
	}
	// Order: IBM old/new then DELL old/new. Java's row-per-group processor
	// iterates an internal HashMap for row order (assertPropsPerRowAnyOrder),
	// so the group order is unconstrained there; Go emits the incoming
	// event's group first, matching the aggregate-grouped irstream contract
	// verified by the resultset-aggregate-count-sum oracle.
	if last.Old[0].Get("symbol").Any() != "IBM" || last.New[0].Get("symbol").Any() != "IBM" ||
		last.Old[1].Get("symbol").Any() != "DELL" || last.New[1].Get("symbol").Any() != "DELL" {
		t.Fatalf("IBM batch order = %#v", last)
	}
	if last.Old[0].Get("mySum").Any() != nil || last.New[0].Get("mySum").Any() != 70.0 ||
		last.Old[1].Get("mySum").Any() != 55.0 || last.New[1].Get("mySum").Any() != 5.0 {
		t.Fatalf("IBM batch values = %#v", last)
	}
	sendRowPerGroupMarket(t, engine, "AAA", 2000)
	last = (*batches)[len(*batches)-1]
	if len(last.New) != 1 || len(last.Old) != 1 || last.Old[0].Get("symbol").Any() != "DELL" ||
		last.Old[0].Get("mySum").Any() != 5.0 || last.New[0].Get("symbol").Any() != "DELL" || last.New[0].Get("mySum").Any() != nil {
		t.Fatalf("DELL eviction batch = %#v", last)
	}
}

// TestResultSetRowPerGroupSumJoinParity covers
// ResultSetQueryTypeRowPerGroupSumJoin: the same grouped sum/avg trajectory
// over a two-stream join.
func TestResultSetRowPerGroupSumJoinParity(t *testing.T) {
	env, engine := newRowPerGroupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	origin := time.Unix(0, 0).UTC()
	engine = NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	left := JoinSource(From[rowPerGroupString](env, "SupportBeanString").Window(LengthWindow(100)))
	right := JoinSource(From[viewTimeWinMarket](env, "SupportMarketDataBean").Window(LengthWindow(3)))
	symbol := JoinField[string](1, "symbol")
	price := JoinField[float64](1, "price")
	plan, err := env.Build(JoinMany(left, right).
		On(OnSourcesEqual(0, Field[rowPerGroupString, string]("theString"), 1, Field[viewTimeWinMarket, string]("symbol"))).
		GroupBy(symbol).Where(Or(
		Equal[string](symbol, Literal("DELL")),
		Or(Equal[string](symbol, Literal("IBM")), Equal[string](symbol, Literal("GE"))),
	)).Select(
		Alias("symbol", symbol),
		Alias("mySum", Sum[float64](price)),
		Alias("myAvg", Avg[float64](price)),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := new([]ResultBatch)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendString := func(theString string) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBeanString", rowPerGroupString{TheString: theString}); err != nil {
			t.Fatal(err)
		}
	}
	sendString("DELL")
	sendString("IBM")
	sendString("AAA")
	sendRowPerGroupMarket(t, engine, "DELL", 10)
	assertRowPerGroupSum(t, batches, "DELL", nil, nil, 10.0, 10.0)
	sendRowPerGroupMarket(t, engine, "DELL", 20)
	assertRowPerGroupSum(t, batches, "DELL", 10.0, 10.0, 30.0, 15.0)
	sendRowPerGroupMarket(t, engine, "DELL", 100)
	assertRowPerGroupSum(t, batches, "DELL", 30.0, 15.0, 130.0, 130.0/3.0)
	sendRowPerGroupMarket(t, engine, "DELL", 50)
	assertRowPerGroupSum(t, batches, "DELL", 130.0, 130.0/3.0, 170.0, 170.0/3.0)
	sendRowPerGroupMarket(t, engine, "DELL", 5)
	assertRowPerGroupSum(t, batches, "DELL", 170.0, 170.0/3.0, 155.0, 155.0/3.0)
	sendRowPerGroupMarket(t, engine, "AAA", 1000)
	assertRowPerGroupSum(t, batches, "DELL", 155.0, 155.0/3.0, 55.0, 27.5)
	sendRowPerGroupMarket(t, engine, "IBM", 70)
	last := (*batches)[len(*batches)-1]
	if len(last.New) != 2 || len(last.Old) != 2 {
		t.Fatalf("IBM batch = %#v", last)
	}
	sendRowPerGroupMarket(t, engine, "AAA", 2000)
	last = (*batches)[len(*batches)-1]
	if len(last.New) != 1 || len(last.Old) != 1 || last.Old[0].Get("symbol").Any() != "DELL" ||
		last.New[0].Get("symbol").Any() != "DELL" || last.New[0].Get("mySum").Any() != nil {
		t.Fatalf("DELL eviction batch = %#v", last)
	}
}

func assertRowPerGroupSum(t *testing.T, batches *[]ResultBatch, symbol string, oldSum, oldAvg, newSum, newAvg any) {
	t.Helper()
	last := (*batches)[len(*batches)-1]
	if len(last.New) != 1 || len(last.Old) != 1 {
		t.Fatalf("batch = %#v", last)
	}
	newRow, oldRow := last.New[0], last.Old[0]
	if oldRow.Get("symbol").Any() != symbol || oldRow.Get("mySum").Any() != oldSum || oldRow.Get("myAvg").Any() != oldAvg {
		t.Fatalf("old row = %v %v %v, want %s %v %v", oldRow.Get("symbol").Any(), oldRow.Get("mySum").Any(), oldRow.Get("myAvg").Any(), symbol, oldSum, oldAvg)
	}
	if newRow.Get("symbol").Any() != symbol || newRow.Get("mySum").Any() != newSum || newRow.Get("myAvg").Any() != newAvg {
		t.Fatalf("new row = %v %v %v, want %s %v %v", newRow.Get("symbol").Any(), newRow.Get("mySum").Any(), newRow.Get("myAvg").Any(), symbol, newSum, newAvg)
	}
}

// TestResultSetRowPerGroupAggregateGroupedPropsParity covers
// ResultSetQueryTypeAggregateGroupedProps: count(price) grouped by price with
// irstream rows and iterator order.
func TestResultSetRowPerGroupAggregateGroupedPropsParity(t *testing.T) {
	env, engine := newRowPerGroupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	price := Field[viewTimeWinMarket, float64]("price")
	plan, err := env.Build(From[viewTimeWinMarket](env, "SupportMarketDataBean").Window(LengthWindow(5)).
		GroupBy(price).Select(
		Alias("mycount", Count[float64](price)),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	batches := new([]ResultBatch)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendRowPerGroupMarket(t, engine, "DELL", 10)
	if got := lastRowPerGroupNew(batches, "mycount"); !reflect.DeepEqual(got, []any{int64(1)}) {
		t.Fatalf("first new = %v", got)
	}
	if got := lastRowPerGroupOld(batches, "mycount"); !reflect.DeepEqual(got, []any{int64(0)}) {
		t.Fatalf("first old = %v", got)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 1 || snapshot.Results()[0].Get("mycount").Any() != int64(1) {
		t.Fatalf("snapshot = %#v", snapshot.Results())
	}
	sendRowPerGroupMarket(t, engine, "DELL", 11)
	if got := lastRowPerGroupNew(batches, "mycount"); !reflect.DeepEqual(got, []any{int64(1)}) {
		t.Fatalf("second new = %v", got)
	}
	sendRowPerGroupMarket(t, engine, "IBM", 10)
	if got := lastRowPerGroupNew(batches, "mycount"); !reflect.DeepEqual(got, []any{int64(2)}) {
		t.Fatalf("third new = %v", got)
	}
	if got := lastRowPerGroupOld(batches, "mycount"); !reflect.DeepEqual(got, []any{int64(1)}) {
		t.Fatalf("third old = %v", got)
	}
}

// TestResultSetRowPerGroupAggregateGroupedPropsPerGroupParity covers
// ResultSetQueryTypeAggregateGroupedPropsPerGroup: two-column group keys.
func TestResultSetRowPerGroupAggregateGroupedPropsPerGroupParity(t *testing.T) {
	env, engine := newRowPerGroupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[viewTimeWinMarket, string]("symbol")
	price := Field[viewTimeWinMarket, float64]("price")
	plan, err := env.Build(From[viewTimeWinMarket](env, "SupportMarketDataBean").Window(LengthWindow(5)).
		GroupBy(symbol, price).Select(
		Alias("mycount", Count[float64](price)),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	batches := new([]ResultBatch)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendRowPerGroupMarket(t, engine, "DELL", 10)
	if got := lastRowPerGroupNew(batches, "mycount"); !reflect.DeepEqual(got, []any{int64(1)}) {
		t.Fatalf("first new = %v", got)
	}
	sendRowPerGroupMarket(t, engine, "DELL", 11)
	if got := lastRowPerGroupNew(batches, "mycount"); !reflect.DeepEqual(got, []any{int64(1)}) {
		t.Fatalf("second new = %v", got)
	}
	sendRowPerGroupMarket(t, engine, "DELL", 10)
	if got := lastRowPerGroupNew(batches, "mycount"); !reflect.DeepEqual(got, []any{int64(2)}) {
		t.Fatalf("third new = %v", got)
	}
	if got := lastRowPerGroupOld(batches, "mycount"); !reflect.DeepEqual(got, []any{int64(1)}) {
		t.Fatalf("third old = %v", got)
	}
	sendRowPerGroupMarket(t, engine, "IBM", 10)
	if got := lastRowPerGroupNew(batches, "mycount"); !reflect.DeepEqual(got, []any{int64(1)}) {
		t.Fatalf("fourth new = %v", got)
	}
}

// TestResultSetRowPerGroupAggregationOverGroupedPropsParity covers
// ResultSetQueryTypeAggregationOverGroupedProps with order-by and eviction.
func TestResultSetRowPerGroupAggregationOverGroupedPropsParity(t *testing.T) {
	env, engine := newRowPerGroupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[viewTimeWinMarket, string]("symbol")
	price := Field[viewTimeWinMarket, float64]("price")
	plan, err := env.Build(From[viewTimeWinMarket](env, "SupportMarketDataBean").Window(LengthWindow(5)).
		GroupBy(symbol, price).Select(
		Alias("symbol", symbol),
		Alias("price", price),
		Alias("mycount", Count[float64](price)),
	).Query(StatementName("s0"), WithOldStream(), OrderBy(Ascending(symbol))))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	batches := new([]ResultBatch)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendRowPerGroupMarket(t, engine, "DELL", 10)
	sendRowPerGroupMarket(t, engine, "DELL", 11)
	sendRowPerGroupMarket(t, engine, "DELL", 10)
	sendRowPerGroupMarket(t, engine, "IBM", 5)
	sendRowPerGroupMarket(t, engine, "IBM", 5)
	// Third IBM evicts DELL/10 (old count 2 -> new count 1) and increments IBM.
	sendRowPerGroupMarket(t, engine, "IBM", 5)
	last := (*batches)[len(*batches)-1]
	if len(last.New) != 2 || len(last.Old) != 2 {
		t.Fatalf("eviction batch = %#v", last)
	}
	// Order by symbol: DELL new/old first, then IBM.
	if last.New[0].Get("symbol").Any() != "DELL" || last.Old[0].Get("symbol").Any() != "DELL" ||
		last.New[1].Get("symbol").Any() != "IBM" || last.Old[1].Get("symbol").Any() != "IBM" {
		t.Fatalf("eviction order = %#v", last)
	}
	if last.New[0].Get("mycount").Any() != int64(1) || last.Old[0].Get("mycount").Any() != int64(2) ||
		last.New[1].Get("mycount").Any() != int64(3) || last.Old[1].Get("mycount").Any() != int64(2) {
		t.Fatalf("eviction counts = %#v", last)
	}
	// Fourth IBM evicts DELL/11.
	sendRowPerGroupMarket(t, engine, "IBM", 5)
	last = (*batches)[len(*batches)-1]
	if len(last.New) != 2 || len(last.Old) != 2 {
		t.Fatalf("second eviction batch = %#v", last)
	}
	if last.New[0].Get("symbol").Any() != "DELL" || last.Old[0].Get("symbol").Any() != "DELL" ||
		last.New[1].Get("symbol").Any() != "IBM" || last.Old[1].Get("symbol").Any() != "IBM" {
		t.Fatalf("second eviction order = %#v", last)
	}
	if last.New[0].Get("mycount").Any() != int64(0) || last.Old[0].Get("mycount").Any() != int64(1) ||
		last.New[1].Get("mycount").Any() != int64(4) || last.Old[1].Get("mycount").Any() != int64(3) {
		t.Fatalf("second eviction counts = %#v", last)
	}
}

// TestResultSetRowPerGroupSelectAvgExprGroupByParity covers
// ResultSetQueryTypeSelectAvgExprGroupBy: avg over length(2) grouped by symbol.
func TestResultSetRowPerGroupSelectAvgExprGroupByParity(t *testing.T) {
	env, engine := newRowPerGroupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[viewTimeWinMarket, string]("symbol")
	price := Field[viewTimeWinMarket, float64]("price")
	plan, err := env.Build(From[viewTimeWinMarket](env, "SupportMarketDataBean").Window(LengthWindow(2)).
		GroupBy(symbol).Select(
		Alias("aprice", Avg[float64](price)),
		Alias("symbol", symbol),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := new([]ResultBatch)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendRowPerGroupMarket(t, engine, "A", 1)
	sendRowPerGroupMarket(t, engine, "B", 3)
	sendRowPerGroupMarket(t, engine, "B", 5)
	last := (*batches)[len(*batches)-1]
	if len(last.New) != 2 {
		t.Fatalf("B batch = %#v", last)
	}
	rowBySymbol := map[string]any{}
	for _, row := range last.New {
		rowBySymbol[row.Get("symbol").Any().(string)] = row.Get("aprice").Any()
	}
	if rowBySymbol["A"] != nil || rowBySymbol["B"] != 4.0 {
		t.Fatalf("B batch rows = %#v", last)
	}
	sendRowPerGroupMarket(t, engine, "A", 10)
	last = (*batches)[len(*batches)-1]
	rowBySymbol = map[string]any{}
	for _, row := range last.New {
		rowBySymbol[row.Get("symbol").Any().(string)] = row.Get("aprice").Any()
	}
	if len(last.New) != 2 || rowBySymbol["A"] != 10.0 || rowBySymbol["B"] != 5.0 {
		t.Fatalf("A10 batch = %#v", last)
	}
	sendRowPerGroupMarket(t, engine, "A", 20)
	last = (*batches)[len(*batches)-1]
	rowBySymbol = map[string]any{}
	for _, row := range last.New {
		rowBySymbol[row.Get("symbol").Any().(string)] = row.Get("aprice").Any()
	}
	if len(last.New) != 2 || rowBySymbol["A"] != 15.0 || rowBySymbol["B"] != nil {
		t.Fatalf("A20 batch = %#v", last)
	}
}

// TestResultSetRowPerGroupCriteriaByDotMethodParity covers
// ResultSetQueryTypeCriteriaByDotMethod: group key from a method call on the
// event, over a length_batch(2) window.
func TestResultSetRowPerGroupCriteriaByDotMethodParity(t *testing.T) {
	env, engine := newRowPerGroupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	key := Method[string](EventValue[rowPerGroupBean](), "TheStringValue")
	plan, err := env.Build(From[rowPerGroupBean](env, "SupportBean").Window(LengthBatch(2)).
		GroupBy(key).Select(
		Alias("c0", key),
		Alias("c1", Sum[int](Field[rowPerGroupBean, int]("intPrimitive"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := new([]ResultBatch)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E1", IntPrimitive: 10})
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E1", IntPrimitive: 20})
	if len(*batches) != 1 {
		t.Fatalf("batches = %d", len(*batches))
	}
	if got := lastRowPerGroupNew(batches, "c0"); !reflect.DeepEqual(got, []any{"E1"}) {
		t.Fatalf("c0 = %v", got)
	}
	if got := lastRowPerGroupNew(batches, "c1"); !reflect.DeepEqual(got, []any{30}) {
		t.Fatalf("c1 = %v", got)
	}
}

// TestResultSetRowPerGroupUnboundStreamIterateParity covers
// ResultSetQueryTypeUnboundStreamIterate: output snapshot every 3 events with
// iterator views, order-by and the reclaim hint.
func TestResultSetRowPerGroupUnboundStreamIterateParity(t *testing.T) {
	t.Run("snapshot", func(t *testing.T) {
		env, engine := newRowPerGroupEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		theString := Field[rowPerGroupBean, string]("theString")
		intPrimitive := Field[rowPerGroupBean, int]("intPrimitive")
		plan, err := env.Build(From[rowPerGroupBean](env, "SupportBean").
			GroupBy(theString).Select(
			Alias("c0", theString),
			Alias("c1", Sum[int](intPrimitive)),
		).Query(StatementName("s0"), WithOutput(OutputSnapshotEveryEvents(3))))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		batches := new([]ResultBatch)
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			*batches = append(*batches, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E1", IntPrimitive: 10})
		if len(*batches) != 0 {
			t.Fatalf("snapshot emitted early: %#v", *batches)
		}
		assertRowPerGroupIteratorRows(t, statement, [][]any{{"E1", 10}})
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E2", IntPrimitive: 20})
		assertRowPerGroupIteratorRows(t, statement, [][]any{{"E1", 10}, {"E2", 20}})
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E1", IntPrimitive: 11})
		if len(*batches) != 1 {
			t.Fatalf("snapshot batches = %#v", *batches)
		}
		if got := lastRowPerGroupNew(batches, "c0"); len(got) != 2 {
			t.Fatalf("snapshot new = %v", got)
		}
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E0", IntPrimitive: 30})
		assertRowPerGroupIteratorRows(t, statement, [][]any{{"E1", 21}, {"E2", 20}, {"E0", 30}})
		if len(*batches) != 1 {
			t.Fatalf("second snapshot emitted early: %#v", *batches)
		}
	})

	t.Run("order-by", func(t *testing.T) {
		env, engine := newRowPerGroupEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		theString := Field[rowPerGroupBean, string]("theString")
		intPrimitive := Field[rowPerGroupBean, int]("intPrimitive")
		plan, err := env.Build(From[rowPerGroupBean](env, "SupportBean").
			GroupBy(theString).Select(
			Alias("c0", theString),
			Alias("c1", Sum[int](intPrimitive)),
		).Query(StatementName("s0"), WithOutput(OutputSnapshotEveryEvents(3)), OrderBy(Ascending(theString))))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E1", IntPrimitive: 10})
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E2", IntPrimitive: 20})
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E1", IntPrimitive: 11})
		assertRowPerGroupIteratorRows(t, statement, [][]any{{"E1", 21}, {"E2", 20}})
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E0", IntPrimitive: 30})
		assertRowPerGroupIteratorRows(t, statement, [][]any{{"E0", 30}, {"E1", 21}, {"E2", 20}})
	})

	t.Run("ungrouped-snapshot", func(t *testing.T) {
		env, engine := newRowPerGroupEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		intPrimitive := Field[rowPerGroupBean, int]("intPrimitive")
		plan, err := env.Build(From[rowPerGroupBean](env, "SupportBean").
			Aggregate(
				Alias("c0", NullLiteral[any]()),
				Alias("c1", Sum[int](intPrimitive)),
			).Query(StatementName("s0"), WithOutput(OutputSnapshotEveryEvents(3))))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E1", IntPrimitive: 10})
		assertRowPerGroupIteratorRows(t, statement, [][]any{{nil, 10}})
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E2", IntPrimitive: 20})
		assertRowPerGroupIteratorRows(t, statement, [][]any{{nil, 30}})
	})

	t.Run("reclaim", func(t *testing.T) {
		env, engine := newRowPerGroupEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		origin := time.Unix(0, 0).UTC()
		engine = NewEngine(env, WithStartTime(origin))
		defer func() { _ = engine.Close(context.Background()) }()
		theString := Field[rowPerGroupBean, string]("theString")
		intPrimitive := Field[rowPerGroupBean, int]("intPrimitive")
		plan, err := env.Build(From[rowPerGroupBean](env, "SupportBean").
			GroupBy(theString).Select(
			Alias("c0", theString),
			Alias("c1", Sum[int](intPrimitive)),
		).Query(StatementName("s0"),
			WithOutput(OutputSnapshotEveryEvents(3)),
			WithStatementHints(
				mustRowPerGroupReclaimHint(t, HintReclaimGroupAged, "1"),
				mustRowPerGroupReclaimHint(t, HintReclaimGroupFreq, "1"),
			)))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		batches := new([]ResultBatch)
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			*batches = append(*batches, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := engine.AdvanceTime(context.Background(), origin.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E1", IntPrimitive: 10})
		if err := engine.AdvanceTime(context.Background(), origin.Add(1500*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E0", IntPrimitive: 11})
		if err := engine.AdvanceTime(context.Background(), origin.Add(1800*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E2", IntPrimitive: 12})
		if len(*batches) != 1 {
			t.Fatalf("reclaim snapshot missing: %#v", *batches)
		}
		assertRowPerGroupIteratorRows(t, statement, [][]any{{"E1", 10}, {"E0", 11}, {"E2", 12}})
		if err := engine.AdvanceTime(context.Background(), origin.Add(2200*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E2", IntPrimitive: 13})
		assertRowPerGroupIteratorRows(t, statement, [][]any{{"E0", 11}, {"E2", 25}})
	})
}

func assertRowPerGroupIteratorRows(t *testing.T, statement *Statement, want [][]any) {
	t.Helper()
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := make([][]any, 0, len(snapshot.Results()))
	for _, result := range snapshot.Results() {
		got = append(got, []any{result.Get("c0").Any(), result.Get("c1").Any()})
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("iterator rows = %v, want %v", got, want)
	}
}

func mustRowPerGroupReclaimHint(t *testing.T, kind StatementHintKind, parameters ...string) StatementHint {
	t.Helper()
	hint, err := NewStatementHint(kind, parameters...)
	if err != nil {
		t.Fatal(err)
	}
	return hint
}

// TestResultSetRowPerGroupNamedWindowDeleteParity covers
// ResultSetQueryTypeNamedWindowDelete: grouped aggregate over a named window
// with on-delete resetting the group row to null.
func TestResultSetRowPerGroupNamedWindowDeleteParity(t *testing.T) {
	env, engine := newRowPerGroupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := RegisterStruct[rowPerGroupBeanA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema missing")
	}
	if _, err := CreateNamedWindow(env, "MyWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[rowPerGroupBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindow",
		SetColumn("theString", Field[rowPerGroupBean, string]("theString")),
		SetColumn("intPrimitive", Field[rowPerGroupBean, int]("intPrimitive")),
		SetColumn("longPrimitive", Field[rowPerGroupBean, int64]("longPrimitive")),
		SetColumn("intBoxed", Field[rowPerGroupBean, int]("intBoxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[rowPerGroupBeanA](env, "SupportBean_A")).DeleteFromNamedWindow(
		"MyWindow",
		Equal[string](NamedWindowField[string]("theString"), Field[rowPerGroupBeanA, string]("id")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	theString := Field[any, string]("theString")
	intPrimitive := Field[any, int]("intPrimitive")
	queryPlan, err := env.Build(FromNamedWindow(env, "MyWindow").
		GroupBy(theString).Select(
		Alias("theString", theString),
		Alias("mysum", Sum[int](intPrimitive)),
	).Query(StatementName("s0"), WithOldStream(), OrderBy(Ascending(theString))))
	if err != nil {
		t.Fatal(err)
	}
	deployment := deployInfraNWTPlans(t, engine, []Plan{insertPlan, deletePlan, queryPlan})
	statement := infraNWTStatementByDeploymentName(t, deployment, "s0")
	batches := new([]ResultBatch)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "A", IntPrimitive: 100})
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "B", IntPrimitive: 20})
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "A", IntPrimitive: 101})
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "B", IntPrimitive: 21})
	if err := engine.Send(context.Background(), "SupportBean_A", rowPerGroupBeanA{ID: "A"}); err != nil {
		t.Fatal(err)
	}
	if got := lastRowPerGroupNew(batches, "mysum"); !reflect.DeepEqual(got, []any{nil}) {
		t.Fatalf("A delete new = %v", got)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 1 || snapshot.Results()[0].Get("theString").Any() != "B" || snapshot.Results()[0].Get("mysum").Any() != 41 {
		t.Fatalf("snapshot after A delete = %#v", snapshot.Results())
	}
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "A", IntPrimitive: 102})
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 2 {
		t.Fatalf("snapshot after reinsert = %#v", snapshot.Results())
	}
}

// TestResultSetRowPerGroupMultikeyWArrayParity covers the three
// ResultSetQueryTypeRowPerGrpMultikeyWArray variants: unbound, keepall and
// join streams grouped by an int-array key.
func TestResultSetRowPerGroupMultikeyWArrayParity(t *testing.T) {
	for name, build := range map[string]func(*Environment) (Query, error){
		"unbound": func(env *Environment) (Query, error) {
			return From[rowPerGroupIntArray](env, "SupportEventWithIntArray").
				GroupBy(Field[rowPerGroupIntArray, []int]("array")).
				Select(Alias("thesum", Sum[int](Field[rowPerGroupIntArray, int]("value")))).
				Query(StatementName("s0")), nil
		},
		"keepall": func(env *Environment) (Query, error) {
			return From[rowPerGroupIntArray](env, "SupportEventWithIntArray").Window(KeepAll()).
				GroupBy(Field[rowPerGroupIntArray, []int]("array")).
				Select(Alias("thesum", Sum[int](Field[rowPerGroupIntArray, int]("value")))).
				Query(StatementName("s0")), nil
		},
		"join": func(env *Environment) (Query, error) {
			return JoinMany(
				JoinSource(From[rowPerGroupIntArray](env, "SupportEventWithIntArray").Window(KeepAll())),
				JoinSource(From[rowPerGroupBean](env, "SupportBean").Window(KeepAll())),
			).GroupBy(JoinField[[]int](0, "array")).
				Select(Alias("thesum", Sum[int](JoinField[int](0, "value")))).
				Query(StatementName("s0")), nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			env, engine := newRowPerGroupEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			query, err := build(env)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			batches := new([]ResultBatch)
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				*batches = append(*batches, batch)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if name == "join" {
				if err := engine.Send(context.Background(), "SupportBean", rowPerGroupBean{}); err != nil {
					t.Fatal(err)
				}
			}
			sendArray := func(value int, array []int) {
				t.Helper()
				if err := engine.Send(context.Background(), "SupportEventWithIntArray", rowPerGroupIntArray{Value: value, Array: array}); err != nil {
					t.Fatal(err)
				}
			}
			sendArray(5, []int{1, 2})
			if got := lastRowPerGroupNew(batches, "thesum"); !reflect.DeepEqual(got, []any{5}) {
				t.Fatalf("first = %v", got)
			}
			sendArray(10, []int{1, 2})
			if got := lastRowPerGroupNew(batches, "thesum"); !reflect.DeepEqual(got, []any{15}) {
				t.Fatalf("second = %v", got)
			}
			sendArray(11, []int{1})
			if got := lastRowPerGroupNew(batches, "thesum"); !reflect.DeepEqual(got, []any{11}) {
				t.Fatalf("third = %v", got)
			}
			sendArray(12, []int{1, 3})
			if got := lastRowPerGroupNew(batches, "thesum"); !reflect.DeepEqual(got, []any{12}) {
				t.Fatalf("fourth = %v", got)
			}
			sendArray(13, []int{1})
			if got := lastRowPerGroupNew(batches, "thesum"); !reflect.DeepEqual(got, []any{24}) {
				t.Fatalf("fifth = %v", got)
			}
		})
	}
}

// TestResultSetRowPerGroupNullGroupKeyParity covers
// ResultSetQueryTypeRowPerGrpNullGroupKey: null-typed group key.
func TestResultSetRowPerGroupNullGroupKeyParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[rowPerGroupNullEvent](env, "MyEventWNullType"); err != nil {
		t.Fatal(err)
	}
	theString := Field[rowPerGroupNullEvent, string]("id")
	value := Field[rowPerGroupNullEvent, int]("value")
	groupKey := Field[rowPerGroupNullEvent, any]("groupkey")
	plan, err := env.Build(From[rowPerGroupNullEvent](env, "MyEventWNullType").
		GroupBy(groupKey).Select(
		Alias("thesum", Sum[int](value)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := new([]ResultBatch)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(id string, val int) {
		t.Helper()
		if err := engine.Send(context.Background(), "MyEventWNullType", rowPerGroupNullEvent{ID: id, Value: val}); err != nil {
			t.Fatal(err)
		}
	}
	send("G1", 10)
	if got := lastRowPerGroupNew(batches, "thesum"); !reflect.DeepEqual(got, []any{10}) {
		t.Fatalf("first = %v", got)
	}
	send("G1", 11)
	if got := lastRowPerGroupNew(batches, "thesum"); !reflect.DeepEqual(got, []any{21}) {
		t.Fatalf("second = %v", got)
	}
	_ = theString
}

// TestResultSetRowPerGroupReclaimSideBySideParity covers
// ResultSetQueryTypeReclaimSideBySide: per-group window access with
// disable_reclaim_group and a configured reclaim statement.
func TestResultSetRowPerGroupReclaimSideBySideParity(t *testing.T) {
	env, engine := newRowPerGroupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	theString := Field[rowPerGroupBean, string]("theString")
	intPrimitive := Field[rowPerGroupBean, int]("intPrimitive")
	sum := func(name string, extra ...StatementHint) (Query, error) {
		return From[rowPerGroupBean](env, "SupportBean").Window(KeepAll()).
			GroupBy(theString).Select(
			Alias("val", Sum[int](intPrimitive)),
		).Query(StatementName(name), WithStatementHints(extra...)), nil
	}
	windowVal := func(name string) (Query, error) {
		return From[rowPerGroupBean](env, "SupportBean").Window(KeepAll()).
			GroupBy(theString).Select(
			Alias("val", WindowAccessBy[int](intPrimitive).Values()),
		).Query(StatementName(name), WithStatementHints(
			mustRowPerGroupReclaimHint(t, HintDisableReclaimGroup),
		)), nil
	}
	both := func(name string, hints ...StatementHint) (Query, error) {
		return From[rowPerGroupBean](env, "SupportBean").Window(KeepAll()).
			GroupBy(theString).Select(
			Alias("val1", Sum[int](intPrimitive)),
			Alias("val2", WindowAccessBy[int](intPrimitive).Values()),
		).Query(StatementName(name), WithStatementHints(hints...)), nil
	}
	disable := mustRowPerGroupReclaimHint(t, HintDisableReclaimGroup)
	reclaimAged := mustRowPerGroupReclaimHint(t, HintReclaimGroupAged, "10")
	reclaimFreq := mustRowPerGroupReclaimHint(t, HintReclaimGroupFreq, "5")
	builders := []func() (Query, error){
		func() (Query, error) { return sum("S0", disable) },
		func() (Query, error) { return windowVal("S1") },
		func() (Query, error) { return both("S2", disable) },
		func() (Query, error) { return both("S3", reclaimAged, reclaimFreq) },
	}
	queries := make([]Query, 0, len(builders))
	for _, builder := range builders {
		query, buildErr := builder()
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		queries = append(queries, query)
	}
	plans := make([]Plan, 0, len(queries))
	for _, query := range queries {
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	deployment := deployInfraNWTPlans(t, engine, plans)
	batches := make([]*[]ResultBatch, 0, len(plans))
	for _, statement := range deployment.Statements() {
		collected := new([]ResultBatch)
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			*collected = append(*collected, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		batches = append(batches, collected)
	}
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E1", IntPrimitive: 1})
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E1", IntPrimitive: 2})
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E2", IntPrimitive: 4})
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E2", IntPrimitive: 5})
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "E1", IntPrimitive: 6})
	wantSums := []any{1, 3, 4, 9, 9}
	if len(*batches[0]) != len(wantSums) {
		t.Fatalf("S0 batches = %d, want %d", len(*batches[0]), len(wantSums))
	}
	for index, want := range wantSums {
		batch := (*batches[0])[index]
		if len(batch.New) != 1 || batch.New[0].Get("val").Any() != want {
			t.Fatalf("S0 batch %d = %#v, want %v", index, batch, want)
		}
	}
	lastS1 := (*batches[1])[len(*batches[1])-1]
	if len(lastS1.New) != 1 {
		t.Fatalf("S1 last batch = %#v", lastS1)
	}
	values, ok := lastS1.New[0].Get("val").Any().([]int)
	if !ok || !reflect.DeepEqual(values, []int{1, 2, 6}) {
		t.Fatalf("S1 window val = %#v", lastS1.New[0].Get("val"))
	}
}

func mustRowPerGroupQuery(t *testing.T, query Query, err error) Query {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	return query
}

// TestResultSetRowPerGroupMultikeyWReclaimParity covers
// ResultSetQueryTypeRowPerGrpMultikeyWReclaim: reclaim resets a group's
// aggregate after the aged window.
func TestResultSetRowPerGroupMultikeyWReclaimParity(t *testing.T) {
	env, engine := newRowPerGroupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	origin := time.Unix(0, 0).UTC()
	engine = NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	theString := Field[rowPerGroupBean, string]("theString")
	intPrimitive := Field[rowPerGroupBean, int]("intPrimitive")
	longPrimitive := Field[rowPerGroupBean, int64]("longPrimitive")
	plan, err := env.Build(From[rowPerGroupBean](env, "SupportBean").
		GroupBy(theString, intPrimitive).Select(
		Alias("theString", theString),
		Alias("intPrimitive", intPrimitive),
		Alias("thesum", Sum[int64](longPrimitive)),
	).Query(StatementName("s0"),
		WithStatementHints(
			mustRowPerGroupReclaimHint(t, HintReclaimGroupAged, "10"),
			mustRowPerGroupReclaimHint(t, HintReclaimGroupFreq, "1"),
		)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := new([]ResultBatch)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "A", IntPrimitive: 0, LongPrimitive: 100})
	if got := lastRowPerGroupNew(batches, "thesum"); !reflect.DeepEqual(got, []any{int64(100)}) {
		t.Fatalf("first = %v", got)
	}
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "A", IntPrimitive: 0, LongPrimitive: 101})
	if got := lastRowPerGroupNew(batches, "thesum"); !reflect.DeepEqual(got, []any{int64(201)}) {
		t.Fatalf("second = %v", got)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(11*time.Second)); err != nil {
		t.Fatal(err)
	}
	sendRowPerGroupBean(t, engine, rowPerGroupBean{TheString: "A", IntPrimitive: 0, LongPrimitive: 104})
	if got := lastRowPerGroupNew(batches, "thesum"); !reflect.DeepEqual(got, []any{int64(104)}) {
		t.Fatalf("after reclaim = %v", got)
	}
}

// TestResultSetRowPerGroupUnboundStreamUnlimitedKeyParity covers a reduced
// ResultSetQueryTypeUnboundStreamUnlimitedKey: aged group reclaim over many
// unbound groups plus the invalid hint matrix.
func TestResultSetRowPerGroupUnboundStreamUnlimitedKeyParity(t *testing.T) {
	env, engine := newRowPerGroupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	origin := time.Unix(0, 0).UTC()
	engine = NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	longPrimitive := Field[rowPerGroupBean, int64]("longPrimitive")
	plan, err := env.Build(From[rowPerGroupBean](env, "SupportBean").
		GroupBy(longPrimitive).Select(
		Alias("longPrimitive", longPrimitive),
		Alias("cnt", CountAll()),
	).Query(StatementName("s0"),
		WithStatementHints(
			mustRowPerGroupReclaimHint(t, HintReclaimGroupAged, "30"),
			mustRowPerGroupReclaimHint(t, HintReclaimGroupFreq, "5"),
		)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := new([]ResultBatch)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	const count = 40
	for i := 0; i < count; i++ {
		if err := engine.AdvanceTime(context.Background(), origin.Add(time.Duration(1000+i*1000)*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		sendRowPerGroupBean(t, engine, rowPerGroupBean{LongPrimitive: int64(i * 1000)})
	}
	*batches = nil
	for i := 0; i < 5; i++ {
		sendRowPerGroupBean(t, engine, rowPerGroupBean{LongPrimitive: int64(i * 1000)})
		if got := lastRowPerGroupNew(batches, "cnt"); !reflect.DeepEqual(got, []any{int64(1)}) {
			t.Fatalf("reclaimed key %d count = %v", i, got)
		}
	}
	for i := 5; i < count; i++ {
		sendRowPerGroupBean(t, engine, rowPerGroupBean{LongPrimitive: int64(i * 1000)})
		if got := lastRowPerGroupNew(batches, "cnt"); !reflect.DeepEqual(got, []any{int64(2)}) {
			t.Fatalf("fresh key %d count = %v", i, got)
		}
	}

	// invalid hint parameters are rejected at Build.
	invalidEnv, _ := newRowPerGroupEnv(t)
	if _, err := invalidEnv.Build(From[rowPerGroupBean](invalidEnv, "SupportBean").
		GroupBy(longPrimitive).Select(Alias("cnt", CountAll())).
		Query(StatementName("bad"),
			WithStatementHints(mustRowPerGroupReclaimHint(t, HintReclaimGroupAged, "xyz")))); err == nil {
		t.Fatal("invalid aged hint accepted")
	}
	if _, err := invalidEnv.Build(From[rowPerGroupBean](invalidEnv, "SupportBean").
		GroupBy(longPrimitive).Select(Alias("cnt", CountAll())).
		Query(StatementName("bad"),
			WithStatementHints(mustRowPerGroupReclaimHint(t, HintReclaimGroupAged, "-30")))); err == nil {
		t.Fatal("negative aged hint accepted")
	}
}

// TestResultSetRowPerGroupUniqueInBatchParity covers
// ResultSetQueryTypeUniqueInBatch: a unique view over a time-batch window
// grouped by symbol delivers one row per flush.
func TestResultSetRowPerGroupUniqueInBatchParity(t *testing.T) {
	env, engine := newRowPerGroupEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	origin := time.Unix(0, 0).UTC()
	engine = NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := RegisterMap(env, "MyStream", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	producerPlan, err := env.Build(Select(
		From[viewTimeWinMarket](env, "SupportMarketDataBean").Window(TimeBatch(time.Second)),
		Alias("symbol", Field[viewTimeWinMarket, string]("symbol")),
		Alias("price", Field[viewTimeWinMarket, float64]("price")),
	).InsertInto("MyStream", StatementName("producer")))
	if err != nil {
		t.Fatal(err)
	}
	symbol := Field[any, string]("symbol")
	consumerPlan, err := env.Build(FromAny(env, "MyStream").
		Window(IntersectWindows(TimeBatch(time.Second), Unique(symbol))).
		GroupBy(symbol).Select(
		Alias("symbol", symbol),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment := deployInfraNWTPlans(t, engine, []Plan{producerPlan, consumerPlan})
	statement := infraNWTStatementByDeploymentName(t, deployment, "s0")
	batches := new([]ResultBatch)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendRowPerGroupMarket(t, engine, "IBM", 100)
	sendRowPerGroupMarket(t, engine, "IBM", 101)
	sendRowPerGroupMarket(t, engine, "IBM", 102)
	if err := engine.AdvanceTime(context.Background(), origin.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 0 {
		t.Fatalf("early output = %#v", *batches)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 1 {
		t.Fatalf("flush batches = %#v", *batches)
	}
	if got := lastRowPerGroupNew(batches, "symbol"); !reflect.DeepEqual(got, []any{"IBM"}) {
		t.Fatalf("flush symbol = %v", got)
	}
}
