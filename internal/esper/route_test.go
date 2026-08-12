package esper

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestInsertIntoRoutesOriginalEventIntoVariantAndPreservesIdentity(t *testing.T) {
	env := NewEnvironment()
	member, err := RegisterStruct[variantOrder](env, "InsertOrder")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariant(env, "InsertVariant", member); err != nil {
		t.Fatal(err)
	}

	insertPlan, err := env.Build(From[variantOrder](env, "InsertOrder").InsertInto("InsertVariant", StatementName("insert-order")))
	if err != nil {
		t.Fatal(err)
	}
	targetPlan, err := env.Build(FromAny(env, "InsertVariant").Query(StatementName("insert-target")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	targetDeployment, err := engine.Deploy(context.Background(), targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	var target []Event
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				target = append(target, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := insertDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) != 1 {
			t.Fatalf("insert statement batch = %#v", batch)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), variantOrder{ID: "I1", Common: "same", Amount: 7}); err != nil {
		t.Fatal(err)
	}
	if len(target) != 1 || target[0].TypeName() != "InsertOrder" || target[0].Get("id").Any() != "I1" {
		t.Fatalf("insert target events = %#v", target)
	}
}

func TestProjectedInsertIntoBuildsMapAndAnyVariantDerivedEvents(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[variantOrder](env, "ProjectionOrder"); err != nil {
		t.Fatal(err)
	}
	mapTarget, err := RegisterMap(env, "ProjectionMap", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		FieldDef("kind", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	anyTarget, err := RegisterVariantAny(env, "ProjectionAny")
	if err != nil {
		t.Fatal(err)
	}
	_ = mapTarget
	_ = anyTarget

	project := Select(
		From[variantOrder](env, "ProjectionOrder"),
		Alias("id", Field[variantOrder, string]("id")),
		Alias("kind", Literal("derived")),
	)
	mapPlan, err := env.Build(project.InsertInto("ProjectionMap", StatementName("projection-map")))
	if err != nil {
		t.Fatal(err)
	}
	anyPlan, err := env.Build(Select(
		From[variantOrder](env, "ProjectionOrder"),
		Alias("id", Field[variantOrder, string]("id")),
		Alias("kind", Literal("any-derived")),
	).InsertInto("ProjectionAny", StatementName("projection-any")))
	if err != nil {
		t.Fatal(err)
	}
	targetMapPlan, err := env.Build(FromAny(env, "ProjectionMap").Query(StatementName("projection-map-target")))
	if err != nil {
		t.Fatal(err)
	}
	targetAnyPlan, err := env.Build(FromAny(env, "ProjectionAny").Query(StatementName("projection-any-target")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, plan := range []Plan{mapPlan, anyPlan} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	mapTargetDeployment, err := engine.Deploy(context.Background(), targetMapPlan)
	if err != nil {
		t.Fatal(err)
	}
	anyTargetDeployment, err := engine.Deploy(context.Background(), targetAnyPlan)
	if err != nil {
		t.Fatal(err)
	}
	var mapEvents, anyEvents []Event
	if _, err := mapTargetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				mapEvents = append(mapEvents, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := anyTargetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				anyEvents = append(anyEvents, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), variantOrder{ID: "P1", Common: "same", Amount: 3}); err != nil {
		t.Fatal(err)
	}
	if len(mapEvents) != 1 || mapEvents[0].Schema().Name() != "ProjectionMap" || mapEvents[0].Get("kind").Any() != "derived" {
		t.Fatalf("projected map events = %#v", mapEvents)
	}
	if len(anyEvents) != 1 || anyEvents[0].Schema().Kind() != SchemaMap || anyEvents[0].Get("kind").Any() != "any-derived" {
		t.Fatalf("projected ANY events = %#v", anyEvents)
	}
}

func TestInsertIntoRejectsInvalidTargetAndPredefinedVariantProjection(t *testing.T) {
	env := NewEnvironment()
	member, err := RegisterStruct[variantOrder](env, "InvalidInsertOrder")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariant(env, "InvalidInsertVariant", member); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(From[variantOrder](env, "InvalidInsertOrder").InsertInto("missing", StatementName("missing-target"))); err == nil {
		t.Fatal("missing insert target must be rejected")
	}
	query := Select(
		From[variantOrder](env, "InvalidInsertOrder"),
		Alias("id", Field[variantOrder, string]("id")),
	).InsertInto("InvalidInsertVariant", StatementName("invalid-variant-projection"))
	if _, err := env.Build(query); err == nil {
		t.Fatal("projected row into predefined variant must be rejected")
	}
}

func TestInsertIntoRoutesJoinProjection(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "RouteOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "RoutePayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "RouteJoin", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("amount", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	query := Join(
		From[joinOrder](env, "RouteOrder"),
		From[joinPayment](env, "RoutePayment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).Select(
		SelectLeft("symbol", Field[joinOrder, string]("symbol")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).InsertInto("RouteJoin", StatementName("route-join"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	targetPlan, err := env.Build(FromAny(env, "RouteJoin").Query(StatementName("route-join-target")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	targetDeployment, err := engine.Deploy(context.Background(), targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	var routed []Event
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("join route result is not an event: %#v", result)
			}
			routed = append(routed, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "RoutePayment", joinPayment{OrderID: "R-1", Amount: 9.5}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "RouteOrder", joinOrder{OrderID: "R-1", Symbol: "ESPER"}); err != nil {
		t.Fatal(err)
	}
	if len(routed) != 1 || routed[0].Schema().Name() != "RouteJoin" || routed[0].Get("symbol").Any() != "ESPER" || routed[0].Get("amount").Any() != 9.5 {
		t.Fatalf("join routed events = %#v", routed)
	}
}

func TestInsertIntoRoutesAggregateProjectionAndOnlyNewStream(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := RegisterMap(env, "RouteAggregate", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("count", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	query := From[runtimeTestTrade](env, "Trade").GroupBy(
		Field[runtimeTestTrade, string]("symbol"),
	).Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
		Alias("count", CountAll()),
	).InsertInto("RouteAggregate", StatementName("route-aggregate"), WithNewStreamOnly())
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	targetPlan, err := env.Build(FromAny(env, "RouteAggregate").Query(StatementName("route-aggregate-target")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	targetDeployment, err := engine.Deploy(context.Background(), targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	var routed []Event
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("aggregate route result is not an event: %#v", result)
			}
			routed = append(routed, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, trade := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "A", Price: 2}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(routed) != 2 || routed[0].Get("count").Any() != int64(1) || routed[1].Get("count").Any() != int64(2) {
		t.Fatalf("aggregate routed events = %#v", routed)
	}
}

func TestInsertIntoRoutesPatternProjection(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := RegisterMap(env, "RoutePattern", []FieldSpec{
		FieldDef("first", reflect.TypeOf("")),
		FieldDef("second", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	pattern := PatternFrom(
		From[runtimeTestTrade](env, "Trade"),
		"a",
		Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")),
	).FollowedBy(
		"b",
		Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("B")),
	)
	query := pattern.Select(
		Alias("first", TagField[string]("a", "symbol")),
		Alias("second", TagField[string]("b", "symbol")),
	).InsertInto("RoutePattern", StatementName("route-pattern"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	targetPlan, err := env.Build(FromAny(env, "RoutePattern").Query(StatementName("route-pattern-target")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	targetDeployment, err := engine.Deploy(context.Background(), targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	var routed []Event
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("pattern route result is not an event: %#v", result)
			}
			routed = append(routed, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, trade := range []runtimeTestTrade{{Symbol: "A"}, {Symbol: "B"}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(routed) != 1 || routed[0].Get("first").Any() != "A" || routed[0].Get("second").Any() != "B" {
		t.Fatalf("pattern routed events = %#v", routed)
	}
}

func TestInsertIntoRouteHonorsOutputSortAndBatchPolicy(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := RegisterMap(env, "RouteBatch", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	query := From[runtimeTestTrade](env, "Trade").InsertInto(
		"RouteBatch",
		StatementName("route-batch"),
		WithOutput(OutputEvery(2)),
		OrderBy(Descending(Field[runtimeTestTrade, float64]("price"))),
	)
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	targetPlan, err := env.Build(FromAny(env, "RouteBatch").Query(StatementName("route-batch-target")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	targetDeployment, err := engine.Deploy(context.Background(), targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	var routed []Event
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("batch route result is not an event: %#v", result)
			}
			routed = append(routed, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "low", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(routed) != 0 {
		t.Fatalf("route output should be buffered: %#v", routed)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "high", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if len(routed) != 2 || routed[0].Get("symbol").Any() != "high" || routed[1].Get("symbol").Any() != "low" {
		t.Fatalf("sorted batched routed events = %#v", routed)
	}
}

func TestInsertIntoRouteRejectsUnboundedCycle(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "RouteLoop", []FieldSpec{
		FieldDef("value", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "RouteLoop").InsertInto("RouteLoop", StatementName("route-loop")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	err = engine.Send(context.Background(), "RouteLoop", map[string]any{"value": int64(1)})
	if err == nil || !strings.Contains(err.Error(), "route event limit") {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestInsertIntoRoutesNamedWindowConsumerEvents(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "RouteTrade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("RouteTrade")
	if !ok {
		t.Fatal("RouteTrade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "RouteLive", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "RouteNamed", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromNamedWindow(env, "RouteLive").InsertInto("RouteNamed", StatementName("route-named")))
	if err != nil {
		t.Fatal(err)
	}
	targetPlan, err := env.Build(FromAny(env, "RouteNamed").Query(StatementName("route-named-target")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	targetDeployment, err := engine.Deploy(context.Background(), targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	var routed []Event
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("named-window route result is not an event: %#v", result)
			}
			routed = append(routed, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "RouteLive", runtimeTestTrade{Symbol: "NW", Price: 4}); err != nil {
		t.Fatal(err)
	}
	if len(routed) != 1 || routed[0].Get("symbol").Any() != "NW" || routed[0].Get("price").Any() != 4.0 {
		t.Fatalf("named-window routed events = %#v", routed)
	}
}

func TestInsertIntoRoutesRemoveStreamFromWindowEviction(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := RegisterMap(env, "RouteRemove", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(1)).InsertInto(
		"RouteRemove",
		StatementName("route-remove"),
		WithRemoveStreamOnly(),
	))
	if err != nil {
		t.Fatal(err)
	}
	targetPlan, err := env.Build(FromAny(env, "RouteRemove").Query(StatementName("route-remove-target")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	targetDeployment, err := engine.Deploy(context.Background(), targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	var routed []Event
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("remove-stream route result is not an event: %#v", result)
			}
			routed = append(routed, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "first", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(routed) != 0 {
		t.Fatalf("remove-stream route emitted on insert: %#v", routed)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "second", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if len(routed) != 1 || routed[0].Get("symbol").Any() != "first" || routed[0].Get("price").Any() != 1.0 {
		t.Fatalf("remove-stream routed events = %#v", routed)
	}
}

func TestTimeOrderRemoveStreamRouteMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[externalTrade](env, "SupportBeanTimestamp"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "OrderedStream", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}

	timestamp := Field[externalTrade, int64]("timestamp")
	routePlan, err := env.Build(Select(
		From[externalTrade](env, "SupportBeanTimestamp").Window(TimeOrder(timestamp, 10*time.Second)),
		Alias("id", Field[externalTrade, string]("symbol")),
	).InsertInto("OrderedStream", StatementName("time-order-remove-route"), WithRemoveStreamOnly()))
	if err != nil {
		t.Fatal(err)
	}
	targetPlan, err := env.Build(FromAny(env, "OrderedStream").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	if _, err := engine.Deploy(context.Background(), routePlan); err != nil {
		t.Fatal(err)
	}
	targetDeployment, err := engine.Deploy(context.Background(), targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				return fmt.Errorf("time-order route result is not an event: %#v", result)
			}
			ids = append(ids, event.Get("id").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	advance := func(milliseconds int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(milliseconds).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	send := func(id string, timestamp int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), externalTrade{Symbol: id, Timestamp: timestamp}); err != nil {
			t.Fatal(err)
		}
	}
	assertIDs := func(want ...string) {
		t.Helper()
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("time-order routed ids = %#v, want %#v", ids, want)
		}
	}

	advance(1000)
	advance(21000)
	send("E1", 21000)
	advance(22000)
	send("E2", 22000)
	advance(28000)
	send("E3", 28000)
	advance(30000)
	send("E4", 27000)
	send("E5", 22000)
	advance(30999)
	assertIDs()
	advance(31000)
	assertIDs("E1")

	// Events already ten seconds old are emitted as remove-stream rows on send.
	send("E6", 21000)
	assertIDs("E1", "E6")
	send("E7", 21300)
	advance(31299)
	assertIDs("E1", "E6")
	advance(31300)
	assertIDs("E1", "E6", "E7")
	advance(31999)
	assertIDs("E1", "E6", "E7")
	advance(32000)
	assertIDs("E1", "E6", "E7", "E2", "E5")
	advance(36999)
	assertIDs("E1", "E6", "E7", "E2", "E5")
	advance(37000)
	assertIDs("E1", "E6", "E7", "E2", "E5", "E4")
	send("E8", 21000)
	assertIDs("E1", "E6", "E7", "E2", "E5", "E4", "E8")
	send("E9", 28000)
	advance(37999)
	assertIDs("E1", "E6", "E7", "E2", "E5", "E4", "E8")
	advance(38000)
	assertIDs("E1", "E6", "E7", "E2", "E5", "E4", "E8", "E3", "E9")
}

func TestFireAndForgetRouteIsExplicitAndPreservesProjectionOrder(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
	}
	sourceSchema, err := RegisterMap(env, "FAFRouteSource", fields)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "FAFRouteWindow", sourceSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "FAFRouteTarget", fields); err != nil {
		t.Fatal(err)
	}
	routePlan, err := env.Build(FromNamedWindow(env, "FAFRouteWindow").InsertInto("FAFRouteTarget", StatementName("faf-route")))
	if err != nil {
		t.Fatal(err)
	}
	targetPlan, err := env.Build(FromAny(env, "FAFRouteTarget").Query(StatementName("faf-route-target")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	targetDeployment, err := engine.Deploy(context.Background(), targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	var routed []Event
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("FAF route result is not an event: %#v", result)
			}
			routed = append(routed, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []map[string]any{
		{"symbol": "A", "price": 10.5},
		{"symbol": "B", "price": 11.5},
	} {
		if err := engine.InsertNamedWindow(context.Background(), "FAFRouteWindow", event); err != nil {
			t.Fatal(err)
		}
	}

	readOnly, err := engine.ExecuteFireAndForget(context.Background(), routePlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(readOnly.Batch.New) != 2 || len(routed) != 0 {
		t.Fatalf("read-only FAF result/routing = %d/%d", len(readOnly.Batch.New), len(routed))
	}
	if err := engine.RouteFireAndForget(context.Background(), routePlan, readOnly); err != nil {
		t.Fatal(err)
	}
	if len(routed) != 2 || routed[0].Get("symbol").Any() != "A" || routed[1].Get("symbol").Any() != "B" {
		t.Fatalf("explicit FAF routed events = %#v", routed)
	}

	if _, err := engine.ExecuteFireAndForgetAndRoute(context.Background(), routePlan); err != nil {
		t.Fatal(err)
	}
	if len(routed) != 4 || routed[2].Get("price").Any() != 10.5 || routed[3].Get("price").Any() != 11.5 {
		t.Fatalf("convenience FAF routed events = %#v", routed)
	}
}

func TestFireAndForgetRouteWithParametersAndRejectsImplicitSideEffect(t *testing.T) {
	env := NewEnvironment()
	if _, err := CreateTable(env, "FAFRouteTable", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "FAFParameterTarget", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	table, ok := engine.Table("FAFRouteTable")
	if !ok {
		t.Fatal("FAFRouteTable is missing")
	}
	for _, values := range []map[string]any{
		{"symbol": "A", "price": 10.5},
		{"symbol": "B", "price": 11.5},
	} {
		if _, err := table.Upsert(context.Background(), values); err != nil {
			t.Fatal(err)
		}
	}
	routePlan, err := env.Build(FromTable(env, "FAFRouteTable").Filter(
		Equal[string](Field[any, string]("symbol"), Parameter[string]("wanted")),
	).InsertInto("FAFParameterTarget", StatementName("faf-parameter-route")))
	if err != nil {
		t.Fatal(err)
	}
	targetPlan, err := env.Build(FromAny(env, "FAFParameterTarget").Query(StatementName("faf-parameter-target")))
	if err != nil {
		t.Fatal(err)
	}
	targetDeployment, err := engine.Deploy(context.Background(), targetPlan)
	if err != nil {
		t.Fatal(err)
	}
	var routed []Event
	if _, err := targetDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("parameter FAF route result is not an event: %#v", result)
			}
			routed = append(routed, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	readOnly, err := engine.ExecuteFireAndForgetWithParameters(context.Background(), routePlan, ParameterValues{"wanted": "B"})
	if err != nil {
		t.Fatal(err)
	}
	if len(readOnly.Batch.New) != 1 || readOnly.Batch.New[0].Get("symbol").Any() != "B" || len(routed) != 0 {
		t.Fatalf("parameter FAF read-only result/routing = %#v/%d", readOnly.Batch.New, len(routed))
	}
	if _, err := engine.ExecuteFireAndForgetAndRouteWithParameters(context.Background(), routePlan, ParameterValues{"wanted": "B"}); err != nil {
		t.Fatal(err)
	}
	if len(routed) != 1 || routed[0].Get("symbol").Any() != "B" {
		t.Fatalf("parameter FAF routed events = %#v", routed)
	}
	prepared, err := engine.PrepareFireAndForget(routePlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.ExecuteAndRouteWithParameters(context.Background(), ParameterValues{"wanted": "B"}); err != nil {
		t.Fatal(err)
	}
	if len(routed) != 2 || routed[1].Get("symbol").Any() != "B" {
		t.Fatalf("prepared parameter FAF routed events = %#v", routed)
	}
	if err := engine.RouteFireAndForget(context.Background(), targetPlan, readOnly); err == nil {
		t.Fatal("a plan without RouteTo must be rejected")
	}
}
