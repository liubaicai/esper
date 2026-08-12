package esper

import (
	"context"
	"reflect"
	"testing"
)

type variantOrder struct {
	ID     string `esper:"id"`
	Common string `esper:"common"`
	Amount int64  `esper:"amount"`
}

type variantQuote struct {
	ID     string  `esper:"id"`
	Common string  `esper:"common"`
	Bid    float64 `esper:"bid"`
}

type variantUnrelated struct {
	ID string `esper:"id"`
}

type variantPayment struct {
	ID     string  `esper:"id"`
	Amount float64 `esper:"amount"`
}

func routeVariantMember(t *testing.T, engine *Engine, target string, schema Schema, underlying any) {
	t.Helper()
	event, err := newEvent(schema, underlying, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Route(context.Background(), target, event); err != nil {
		t.Fatal(err)
	}
}

func TestVariantPredefinedRoutesMembersAndExposesCommonFields(t *testing.T) {
	env := NewEnvironment()
	orderSchema, err := RegisterStruct[variantOrder](env, "VariantOrder")
	if err != nil {
		t.Fatal(err)
	}
	quoteSchema, err := RegisterStruct[variantQuote](env, "VariantQuote")
	if err != nil {
		t.Fatal(err)
	}
	variantSchema, err := RegisterVariant(env, "OrderOrQuote", orderSchema, quoteSchema)
	if err != nil {
		t.Fatal(err)
	}
	if got := variantSchema.VariantMembers(); !reflect.DeepEqual(got, []string{"VariantOrder", "VariantQuote"}) {
		t.Fatalf("variant members = %#v", got)
	}
	if variantSchema.IsVariantAny() {
		t.Fatal("predefined variant reported ANY mode")
	}
	if _, ok := variantSchema.Field("amount"); ok {
		t.Fatal("member-only field must not be declared on predefined variant")
	}
	if _, ok := variantSchema.Field("common"); !ok {
		t.Fatal("common field is missing from predefined variant")
	}

	stream := FromAny(env, "OrderOrQuote").Filter(
		Equal[string](Field[any, string]("common"), Literal("same")),
	)
	plan, err := env.Build(stream.Query(StatementName("variant-common")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var received []Event
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if ok {
				received = append(received, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	routeVariantMember(t, engine, "OrderOrQuote", orderSchema, variantOrder{ID: "O1", Common: "same", Amount: 10})
	routeVariantMember(t, engine, "OrderOrQuote", quoteSchema, variantQuote{ID: "Q1", Common: "same", Bid: 12.5})
	routeVariantMember(t, engine, "OrderOrQuote", orderSchema, variantOrder{ID: "O2", Common: "other", Amount: 20})
	if len(received) != 2 || received[0].TypeName() != "VariantOrder" || received[1].TypeName() != "VariantQuote" {
		t.Fatalf("variant received = %#v", received)
	}
	if got := received[0].Get("amount").Any(); got != int64(10) {
		t.Fatalf("member property amount = %#v", got)
	}
	if got := received[1].Get("bid").Any(); got != 12.5 {
		t.Fatalf("member property bid = %#v", got)
	}
}

func TestVariantAnyResolvesDynamicPropertiesAndRoutesExplicitEvent(t *testing.T) {
	env := NewEnvironment()
	orderSchema, err := RegisterStruct[variantOrder](env, "AnyOrder")
	if err != nil {
		t.Fatal(err)
	}
	quoteSchema, err := RegisterStruct[variantQuote](env, "AnyQuote")
	if err != nil {
		t.Fatal(err)
	}
	variantSchema, err := RegisterVariantAny(env, "AnyEvent")
	if err != nil {
		t.Fatal(err)
	}
	if !variantSchema.IsVariantAny() || len(variantSchema.Fields()) != 0 {
		t.Fatalf("ANY variant metadata = mode:%v fields:%v", variantSchema.VariantMode(), variantSchema.Fields())
	}

	stream := FromAny(env, "AnyEvent").Filter(
		Equal[float64](Field[any, float64]("bid"), Literal(12.5)),
	)
	plan, err := env.Build(stream.Query(StatementName("variant-any")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var received []Event
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				received = append(received, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	routeVariantMember(t, engine, "AnyEvent", orderSchema, variantOrder{ID: "O1", Common: "same", Amount: 10})
	routeVariantMember(t, engine, "AnyEvent", quoteSchema, variantQuote{ID: "Q1", Common: "same", Bid: 12.5})
	if len(received) != 1 || received[0].TypeName() != "AnyQuote" {
		t.Fatalf("ANY routed events = %#v", received)
	}
	if got := received[0].Get("bid").Any(); got != 12.5 {
		t.Fatalf("ANY routed bid = %#v", got)
	}

	other, err := NewSchema("NotAMember", FieldDef("id", reflect.TypeOf("")))
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := newEvent(other, map[string]any{"id": "X"}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Route(context.Background(), "AnyEvent", foreign); err == nil {
		t.Fatal("ANY route should reject an event type that is not registered")
	}
}

func TestVariantPredefinedRejectsMemberOnlyExpressionAndDuplicateMembers(t *testing.T) {
	env := NewEnvironment()
	orderSchema, err := RegisterStruct[variantOrder](env, "RejectOrder")
	if err != nil {
		t.Fatal(err)
	}
	quoteSchema, err := RegisterStruct[variantQuote](env, "RejectQuote")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewVariantSchema("DuplicateVariant", orderSchema, orderSchema); err == nil {
		t.Fatal("duplicate variant members must be rejected")
	}
	if _, err := RegisterVariant(env, "RejectVariant", orderSchema, quoteSchema); err != nil {
		t.Fatal(err)
	}
	query := FromAny(env, "RejectVariant").Filter(
		Equal[int64](Field[any, int64]("amount"), Literal(int64(10))),
	).Query(StatementName("member-only"))
	if _, err := env.Build(query); err == nil {
		t.Fatal("predefined variant must reject a member-only field")
	}
}

func TestVariantMemberMatchingFlowsThroughJoinPatternAndNamedWindow(t *testing.T) {
	env := NewEnvironment()
	orderSchema, err := RegisterStruct[variantOrder](env, "FlowOrder")
	if err != nil {
		t.Fatal(err)
	}
	quoteSchema, err := RegisterStruct[variantQuote](env, "FlowQuote")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariant(env, "FlowVariant", orderSchema, quoteSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[variantPayment](env, "FlowPayment"); err != nil {
		t.Fatal(err)
	}
	windowSchema, ok := env.Schema("FlowVariant")
	if !ok {
		t.Fatal("flow variant schema is missing")
	}
	if _, err := CreateNamedWindow(env, "flow-window", windowSchema); err != nil {
		t.Fatal(err)
	}

	joinPlan, err := env.Build(Join(
		From[variantOrder](env, "FlowVariant"),
		From[variantPayment](env, "FlowPayment"),
		OnEqual(Field[any, string]("id"), Field[variantPayment, string]("id")),
	).Select(
		SelectLeft("variant-id", Field[any, string]("id")),
		SelectRight("amount", Field[variantPayment, float64]("amount")),
	).Query(StatementName("variant-flow-join")))
	if err != nil {
		t.Fatal(err)
	}
	patternPlan, err := env.Build(PatternFrom(
		From[variantOrder](env, "FlowVariant"),
		"event", Equal[string](Field[any, string]("common"), Literal("pattern")),
	).Select(Alias("id", TagField[string]("event", "id"))).Query(StatementName("variant-flow-pattern")))
	if err != nil {
		t.Fatal(err)
	}
	windowPlan, err := env.Build(FromNamedWindow(env, "flow-window").Filter(
		Equal[string](Field[any, string]("common"), Literal("window")),
	).Query(StatementName("variant-flow-window")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	joinDeployment, err := engine.Deploy(context.Background(), joinPlan)
	if err != nil {
		t.Fatal(err)
	}
	patternDeployment, err := engine.Deploy(context.Background(), patternPlan)
	if err != nil {
		t.Fatal(err)
	}
	windowDeployment, err := engine.Deploy(context.Background(), windowPlan)
	if err != nil {
		t.Fatal(err)
	}
	var joinBatches, patternBatches, windowBatches []ResultBatch
	if _, err := joinDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		joinBatches = append(joinBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := patternDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		patternBatches = append(patternBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := windowDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		windowBatches = append(windowBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	routeVariantMember(t, engine, "FlowVariant", quoteSchema, variantQuote{ID: "J1", Common: "join", Bid: 4})
	if err := engine.SendEvent(context.Background(), variantPayment{ID: "J1", Amount: 9}); err != nil {
		t.Fatal(err)
	}
	if len(joinBatches) != 1 || len(joinBatches[0].New) != 1 {
		t.Fatalf("variant join batches = %#v", joinBatches)
	}
	joinRow, ok := joinBatches[0].New[0].Row()
	if !ok || joinRow.Get("variant-id").Any() != "J1" || joinRow.Get("amount").Any() != float64(9) {
		t.Fatalf("variant join row = %#v", joinBatches[0].New[0])
	}

	routeVariantMember(t, engine, "FlowVariant", orderSchema, variantOrder{ID: "P1", Common: "pattern", Amount: 1})
	if len(patternBatches) != 1 || len(patternBatches[0].New) != 1 {
		t.Fatalf("variant pattern batches = %#v", patternBatches)
	}
	patternRow, ok := patternBatches[0].New[0].Row()
	if !ok || patternRow.Get("id").Any() != "P1" {
		t.Fatalf("variant pattern row = %#v", patternBatches[0].New[0])
	}

	memberEvent, err := newEvent(orderSchema, variantOrder{ID: "W1", Common: "window", Amount: 2}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "flow-window", memberEvent); err != nil {
		t.Fatal(err)
	}
	if len(windowBatches) != 1 || len(windowBatches[0].New) != 1 {
		t.Fatalf("variant named-window batches = %#v", windowBatches)
	}
}

func TestVariantDataflowEventBusSourceMatchesMembers(t *testing.T) {
	env := NewEnvironment()
	orderSchema, err := RegisterStruct[variantOrder](env, "DataflowOrder")
	if err != nil {
		t.Fatal(err)
	}
	quoteSchema, err := RegisterStruct[variantQuote](env, "DataflowQuote")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariant(env, "DataflowVariant", orderSchema, quoteSchema); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "variant-dataflow").
		EventBusSource("source", "DataflowVariant").
		Filter("common", Equal[string](Field[any, string]("common"), Literal("keep"))).
		Emitter("emit").Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	routeVariantMember(t, engine, "DataflowVariant", orderSchema, variantOrder{ID: "O1", Common: "keep", Amount: 1})
	routeVariantMember(t, engine, "DataflowVariant", quoteSchema, variantQuote{ID: "Q1", Common: "drop", Bid: 2})
	if stats := instance.Stats(); stats.Processed != 2 || stats.Emitted != 1 {
		t.Fatalf("variant dataflow stats = %#v", stats)
	}
	outputs := instance.Outputs()
	if len(outputs) != 1 {
		t.Fatalf("variant dataflow outputs = %#v", outputs)
	}
	event, ok := outputs[0].(Event)
	if !ok || event.TypeName() != "DataflowOrder" || event.Get("id").Any() != "O1" {
		t.Fatalf("variant dataflow output = %#v", outputs[0])
	}
}
