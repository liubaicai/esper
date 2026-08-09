package esper

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type splitStreamSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type splitStreamSupportBeanS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

type splitStreamIntArrayEvent struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

type splitStreamOrderItem struct {
	Amount    int     `esper:"amount"`
	ItemID    string  `esper:"itemId"`
	Price     float64 `esper:"price"`
	ProductID string  `esper:"productId"`
}

type splitStreamOrderDetail struct {
	OrderID string                 `esper:"orderId"`
	Items   []splitStreamOrderItem `esper:"items"`
}

type splitStreamOrder struct {
	OrderDetail splitStreamOrderDetail `esper:"orderdetail"`
}

type splitStreamRecorder struct {
	events []Event
	after  func(Event)
}

func newSplitStreamEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[splitStreamSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func registerSplitBeanTarget(t *testing.T, env *Environment, name string) {
	t.Helper()
	if _, err := RegisterMap(env, name, []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
}

func registerSplitStringTargets(t *testing.T, env *Environment, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := RegisterMap(env, name, []FieldSpec{FieldDef("theString", reflect.TypeOf(""))}); err != nil {
			t.Fatal(err)
		}
	}
}

func deploySplitPlan(t *testing.T, engine *Engine, plan Plan) (*Deployment, *splitStreamRecorder) {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	recorder := &splitStreamRecorder{}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("split-stream result is not an event: %#v", result)
			}
			recorder.events = append(recorder.events, event)
			if recorder.after != nil {
				recorder.after(event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return deployment, recorder
}

func deploySplitConsumer(t *testing.T, env *Environment, engine *Engine, eventType string) *splitStreamRecorder {
	t.Helper()
	plan, err := env.Build(FromAny(env, eventType).Query(StatementName("consume-" + eventType)))
	if err != nil {
		t.Fatal(err)
	}
	_, recorder := deploySplitPlan(t, engine, plan)
	return recorder
}

func splitEventStrings(recorder *splitStreamRecorder) []string {
	if recorder == nil {
		return nil
	}
	values := make([]string, 0, len(recorder.events))
	for _, event := range recorder.events {
		value, _ := event.Get("theString").Any().(string)
		values = append(values, value)
	}
	return values
}

func registerSplitOrderTypes(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[splitStreamOrder](env, "SplitStreamOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[splitStreamOrderItem](env, "SplitStreamOrderItem"); err != nil {
		t.Fatal(err)
	}
}

func splitOrderStream(env *Environment) (Stream[splitStreamOrder], Stream[splitStreamOrderItem]) {
	orders := From[splitStreamOrder](env, "SplitStreamOrder")
	detail := Field[splitStreamOrder, splitStreamOrderDetail]("orderdetail")
	items := Unnest[splitStreamOrder, splitStreamOrderItem](orders, Property[[]splitStreamOrderItem](detail, "items"))
	return orders, items
}

func splitOrder(orderID string, items ...splitStreamOrderItem) splitStreamOrder {
	return splitStreamOrder{OrderDetail: splitStreamOrderDetail{OrderID: orderID, Items: items}}
}

func registerSplitMapTarget(t *testing.T, env *Environment, name string, fields ...FieldSpec) {
	t.Helper()
	if _, err := RegisterMap(env, name, fields); err != nil {
		t.Fatal(err)
	}
}

func splitRecorderValues(recorder *splitStreamRecorder, field string) []any {
	values := make([]any, 0, len(recorder.events))
	for _, event := range recorder.events {
		values = append(values, event.Get(field).Any())
	}
	return values
}

func TestSplitStreamFromClauseParity(t *testing.T) {
	t.Run("begin-body-end", func(t *testing.T) {
		env := NewEnvironment()
		registerSplitOrderTypes(t, env)
		registerSplitMapTarget(t, env, "BeginEvent", FieldDef("orderId", reflect.TypeOf("")))
		registerSplitMapTarget(t, env, "OrderItem",
			FieldDef("amount", reflect.TypeOf(int(0))),
			FieldDef("itemId", reflect.TypeOf("")),
			FieldDef("price", reflect.TypeOf(float64(0))),
			FieldDef("productId", reflect.TypeOf("")),
			FieldDef("orderId", reflect.TypeOf("")),
		)
		registerSplitMapTarget(t, env, "EndEvent", FieldDef("orderId", reflect.TypeOf("")))

		orders, items := splitOrderStream(env)
		orderID := Property[string](Field[splitStreamOrder, splitStreamOrderDetail]("orderdetail"), "orderId")
		parentOrderID := Property[string](ContainedParentField[splitStreamOrderDetail]("orderdetail"), "orderId")
		plan, err := env.Build(OnEvent(orders).SplitAll(
			SplitFrom(orders).Into("BeginEvent", Alias("orderId", orderID)),
			SplitFrom(items).Into("OrderItem",
				Alias("amount", Field[splitStreamOrderItem, int]("amount")),
				Alias("itemId", Field[splitStreamOrderItem, string]("itemId")),
				Alias("price", Field[splitStreamOrderItem, float64]("price")),
				Alias("productId", Field[splitStreamOrderItem, string]("productId")),
				Alias("orderId", parentOrderID),
			),
			SplitFrom(orders).Into("EndEvent", Alias("orderId", orderID)),
		).Query(StatementName("split-from-begin-body-end")))
		if err != nil {
			t.Fatal(err)
		}

		engine := NewEngine(env)
		begin := deploySplitConsumer(t, env, engine, "BeginEvent")
		body := deploySplitConsumer(t, env, engine, "OrderItem")
		end := deploySplitConsumer(t, env, engine, "EndEvent")
		var routeOrder []string
		begin.after = func(Event) { routeOrder = append(routeOrder, "begin") }
		body.after = func(Event) { routeOrder = append(routeOrder, "item") }
		end.after = func(Event) { routeOrder = append(routeOrder, "end") }
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}

		inputs := []splitStreamOrder{
			splitOrder("PO200901",
				splitStreamOrderItem{Amount: 1, ItemID: "A001", Price: 10, ProductID: "10020"},
				splitStreamOrderItem{Amount: 2, ItemID: "A002", Price: 20, ProductID: "10021"},
				splitStreamOrderItem{Amount: 3, ItemID: "A003", Price: 30, ProductID: "10022"}),
			splitOrder("PO200902", splitStreamOrderItem{Amount: 1, ItemID: "B001", Price: 5, ProductID: "10022"}),
			splitOrder("PO200904"),
		}
		for _, input := range inputs {
			if err := engine.SendEvent(context.Background(), input); err != nil {
				t.Fatal(err)
			}
		}
		if got := splitRecorderValues(begin, "orderId"); !reflect.DeepEqual(got, []any{"PO200901", "PO200902", "PO200904"}) {
			t.Fatalf("begin order ids = %#v", got)
		}
		if got := splitRecorderValues(body, "itemId"); !reflect.DeepEqual(got, []any{"A001", "A002", "A003", "B001"}) {
			t.Fatalf("body item ids = %#v", got)
		}
		if got := splitRecorderValues(body, "orderId"); !reflect.DeepEqual(got, []any{"PO200901", "PO200901", "PO200901", "PO200902"}) {
			t.Fatalf("body order ids = %#v", got)
		}
		if got := splitRecorderValues(end, "orderId"); !reflect.DeepEqual(got, []any{"PO200901", "PO200902", "PO200904"}) {
			t.Fatalf("end order ids = %#v", got)
		}
		wantOrder := []string{"begin", "item", "item", "item", "end", "begin", "item", "end", "begin", "end"}
		if !reflect.DeepEqual(routeOrder, wantOrder) {
			t.Fatalf("branch route order = %#v, want %#v", routeOrder, wantOrder)
		}
	})

	t.Run("multiple-projections", func(t *testing.T) {
		env := NewEnvironment()
		registerSplitOrderTypes(t, env)
		registerSplitMapTarget(t, env, "StartEvent", FieldDef("oi", reflect.TypeOf("")))
		registerSplitMapTarget(t, env, "ThenEvent", FieldDef("oi", reflect.TypeOf("")), FieldDef("itemId", reflect.TypeOf("")))
		registerSplitMapTarget(t, env, "MoreEvent",
			FieldDef("oi", reflect.TypeOf("")), FieldDef("itemId", reflect.TypeOf("")), FieldDef("order", reflect.TypeOf(Event{})),
		)
		orders, items := splitOrderStream(env)
		orderID := Property[string](Field[splitStreamOrder, splitStreamOrderDetail]("orderdetail"), "orderId")
		parentOrderID := Property[string](ContainedParentField[splitStreamOrderDetail]("orderdetail"), "orderId")
		plan, err := env.Build(OnEvent(orders).SplitAll(
			SplitFrom(orders).Into("StartEvent", Alias("oi", orderID)),
			SplitFrom(items).Into("ThenEvent",
				Alias("oi", parentOrderID),
				Alias("itemId", Field[splitStreamOrderItem, string]("itemId"))),
			SplitFrom(items).Into("MoreEvent",
				Alias("oi", parentOrderID),
				Alias("itemId", Field[splitStreamOrderItem, string]("itemId")),
				Alias("order", ContainedParentEvent())),
		).Query(StatementName("split-from-multiple")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		start := deploySplitConsumer(t, env, engine, "StartEvent")
		thenEvents := deploySplitConsumer(t, env, engine, "ThenEvent")
		more := deploySplitConsumer(t, env, engine, "MoreEvent")
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
		input := splitOrder("PO200901", splitStreamOrderItem{ItemID: "A001"}, splitStreamOrderItem{ItemID: "A002"}, splitStreamOrderItem{ItemID: "A003"})
		if err := engine.SendEvent(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		if got := splitRecorderValues(start, "oi"); !reflect.DeepEqual(got, []any{"PO200901"}) {
			t.Fatalf("start = %#v", got)
		}
		if got := splitRecorderValues(thenEvents, "itemId"); !reflect.DeepEqual(got, []any{"A001", "A002", "A003"}) {
			t.Fatalf("then = %#v", got)
		}
		if got := splitRecorderValues(more, "itemId"); !reflect.DeepEqual(got, []any{"A001", "A002", "A003"}) {
			t.Fatalf("more = %#v", got)
		}
		for index, event := range more.events {
			parent, ok := event.Get("order").Any().(Event)
			if !ok || !reflect.DeepEqual(parent.Underlying(), input) {
				t.Fatalf("more parent %d = %#v, want original order", index, event.Get("order").Any())
			}
		}
	})

	t.Run("output-first-where", func(t *testing.T) {
		env := NewEnvironment()
		registerSplitOrderTypes(t, env)
		for _, target := range []string{"HeaderEvent", "StreamOne", "StreamTwo", "StreamThree"} {
			registerSplitMapTarget(t, env, target, FieldDef("orderId", reflect.TypeOf("")), FieldDef("itemId", reflect.TypeOf("")))
		}
		orders, items := splitOrderStream(env)
		orderID := Property[string](ContainedParentField[splitStreamOrderDetail]("orderdetail"), "orderId")
		project := func() []Selection {
			return []Selection{Alias("orderId", orderID), Alias("itemId", Field[splitStreamOrderItem, string]("itemId"))}
		}
		productID := Field[splitStreamOrderItem, string]("productId")
		plan, err := env.Build(OnEvent(orders).SplitFirst(
			SplitFrom(orders).IntoWhen(Literal(false), "HeaderEvent"),
			SplitFrom(items.Filter(Equal[string](productID, Literal("10020")))).Into("StreamOne", project()...),
			SplitFrom(items.Filter(Equal[string](productID, Literal("10022")))).Into("StreamTwo", project()...),
			SplitFrom(items.Filter(In[string](productID, Literal("10020"), Literal("10025"), Literal("10022")))).Into("StreamThree", project()...),
		).Query(StatementName("split-from-output-first")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		one := deploySplitConsumer(t, env, engine, "StreamOne")
		two := deploySplitConsumer(t, env, engine, "StreamTwo")
		three := deploySplitConsumer(t, env, engine, "StreamThree")
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
		inputs := []splitStreamOrder{
			splitOrder("PO200901", splitStreamOrderItem{ItemID: "A001", ProductID: "10020"}, splitStreamOrderItem{ItemID: "A002", ProductID: "10022"}),
			splitOrder("PO200902", splitStreamOrderItem{ItemID: "B001", ProductID: "10022"}),
			splitOrder("PO200903", splitStreamOrderItem{ItemID: "C001", ProductID: "10025"}),
			splitOrder("PO200904", splitStreamOrderItem{ItemID: "D001", ProductID: "99999"}),
		}
		for _, input := range inputs {
			if err := engine.SendEvent(context.Background(), input); err != nil {
				t.Fatal(err)
			}
		}
		if got := splitRecorderValues(one, "orderId"); !reflect.DeepEqual(got, []any{"PO200901"}) {
			t.Fatalf("stream one = %#v", got)
		}
		if got := splitRecorderValues(two, "orderId"); !reflect.DeepEqual(got, []any{"PO200902"}) {
			t.Fatalf("stream two = %#v", got)
		}
		if got := splitRecorderValues(three, "orderId"); !reflect.DeepEqual(got, []any{"PO200903"}) {
			t.Fatalf("stream three = %#v", got)
		}
	})

	t.Run("documentation-context-count", func(t *testing.T) {
		env := NewEnvironment()
		registerSplitOrderTypes(t, env)
		registerSplitMapTarget(t, env, "MyOrderBeginEvent", FieldDef("orderId", reflect.TypeOf("")))
		registerSplitMapTarget(t, env, "MyOrderItemEvent", FieldDef("orderId", reflect.TypeOf("")), FieldDef("itemId", reflect.TypeOf("")))
		registerSplitMapTarget(t, env, "MyOrderEndEvent", FieldDef("orderId", reflect.TypeOf("")))
		orders, items := splitOrderStream(env)
		rootOrderID := Property[string](Field[splitStreamOrder, splitStreamOrderDetail]("orderdetail"), "orderId")
		parentOrderID := Property[string](ContainedParentField[splitStreamOrderDetail]("orderdetail"), "orderId")
		splitPlan, err := env.Build(OnEvent(orders).SplitAll(
			SplitFrom(orders).Into("MyOrderBeginEvent", Alias("orderId", rootOrderID)),
			SplitFrom(items).Into("MyOrderItemEvent",
				Alias("orderId", parentOrderID),
				Alias("itemId", Field[splitStreamOrderItem, string]("itemId"))),
			SplitFrom(orders).Into("MyOrderEndEvent", Alias("orderId", rootOrderID)),
		).Query(StatementName("split-from-doc-route")))
		if err != nil {
			t.Fatal(err)
		}
		currentType := TypeName(EventValue[Event]())
		if _, err := CreateInitiatedTerminatedContext(
			env,
			"MyOrderContext",
			Field[Event, string]("orderId"),
			Equal[string](currentType, Literal("MyOrderBeginEvent")),
			Equal[string](currentType, Literal("MyOrderEndEvent")),
		); err != nil {
			t.Fatal(err)
		}
		countPlan, err := env.Build(FromAny(env, "MyOrderItemEvent").Aggregate(
			Alias("orderItemCount", CountAll()),
		).Query(
			StatementName("split-from-doc-count"),
			WithContext("MyOrderContext"),
			WithOutput(OutputSnapshotWhenTerminated()),
		))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		countDeployment, err := engine.Deploy(context.Background(), countPlan)
		if err != nil {
			t.Fatal(err)
		}
		var counts []int64
		if _, err := countDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					return NewError(ErrorTypeMismatch, "split context count result is not a row")
				}
				counts = append(counts, row.Get("orderItemCount").Any().(int64))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(context.Background(), splitPlan); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), splitOrder("1010", splitStreamOrderItem{ItemID: "A0001"})); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(counts, []int64{1}) {
			t.Fatalf("terminated context counts = %#v, want [1]", counts)
		}
	})
}

func TestSplitStream2SplitNoDefaultOutputFirstParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	registerSplitBeanTarget(t, env, "AStream2SP")
	registerSplitBeanTarget(t, env, "BStream2SP")

	source := From[splitStreamSupportBean](env, "SupportBean")
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	plan, err := env.Build(OnEvent(source).SplitFirst(
		SplitIntoWhen(Equal[int](intPrimitive, Literal(1)), "AStream2SP"),
		SplitIntoWhen(Or(Equal[int](intPrimitive, Literal(1)), Equal[int](intPrimitive, Literal(2))), "BStream2SP"),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	a := deploySplitConsumer(t, env, engine, "AStream2SP")
	b := deploySplitConsumer(t, env, engine, "BStream2SP")
	_, fallback := deploySplitPlan(t, engine, plan)
	for _, event := range []splitStreamSupportBean{{"E1", 1}, {"E2", 2}, {"E3", 1}, {"E4", -999}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if got := splitEventStrings(a); !reflect.DeepEqual(got, []string{"E1", "E3"}) {
		t.Fatalf("AStream2SP = %#v", got)
	}
	if got := splitEventStrings(b); !reflect.DeepEqual(got, []string{"E2"}) {
		t.Fatalf("BStream2SP = %#v", got)
	}
	if got := splitEventStrings(fallback); !reflect.DeepEqual(got, []string{"E4"}) {
		t.Fatalf("split fallback = %#v", got)
	}
}

func TestSplitStream1SplitDefaultParity(t *testing.T) {
	t.Run("wildcard", func(t *testing.T) {
		env := newSplitStreamEnvironment(t)
		registerSplitBeanTarget(t, env, "AStream")
		plan, err := env.Build(OnEvent(From[splitStreamSupportBean](env, "SupportBean")).SplitFirst(
			SplitInto("AStream"),
		).Query(StatementName("insert")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		a := deploySplitConsumer(t, env, engine, "AStream")
		_, fallback := deploySplitPlan(t, engine, plan)
		if err := engine.SendEvent(context.Background(), splitStreamSupportBean{"E1", 1}); err != nil {
			t.Fatal(err)
		}
		if got := splitEventStrings(a); !reflect.DeepEqual(got, []string{"E1"}) || len(fallback.events) != 0 {
			t.Fatalf("wildcard route=%#v fallback=%#v", got, fallback.events)
		}
	})

	t.Run("projection", func(t *testing.T) {
		env := newSplitStreamEnvironment(t)
		if _, err := RegisterMap(env, "BStreamABC", []FieldSpec{FieldDef("value", reflect.TypeOf(int(0)))}); err != nil {
			t.Fatal(err)
		}
		intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
		plan, err := env.Build(OnEvent(From[splitStreamSupportBean](env, "SupportBean")).SplitFirst(
			SplitInto("BStreamABC", Alias("value", Multiply[int](Literal(3), intPrimitive))),
		).Query(StatementName("s1")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		consumer := deploySplitConsumer(t, env, engine, "BStreamABC")
		_, fallback := deploySplitPlan(t, engine, plan)
		if err := engine.SendEvent(context.Background(), splitStreamSupportBean{"E1", 6}); err != nil {
			t.Fatal(err)
		}
		if len(consumer.events) != 1 || consumer.events[0].Get("value").Any() != 18 || len(fallback.events) != 0 {
			t.Fatalf("projection route=%#v fallback=%#v", consumer.events, fallback.events)
		}
	})
}

func TestSplitStream2SplitNoDefaultOutputAllParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	registerSplitStringTargets(t, env, "AStream2S", "BStream2S")
	source := From[splitStreamSupportBean](env, "SupportBean")
	theString := Field[splitStreamSupportBean, string]("theString")
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	plan, err := env.Build(OnEvent(source).SplitAll(
		SplitIntoWhen(Equal[int](intPrimitive, Literal(1)), "AStream2S", Alias("theString", theString)),
		SplitIntoWhen(Or(Equal[int](intPrimitive, Literal(1)), Equal[int](intPrimitive, Literal(2))), "BStream2S", Alias("theString", theString)),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	a := deploySplitConsumer(t, env, engine, "AStream2S")
	b := deploySplitConsumer(t, env, engine, "BStream2S")
	_, fallback := deploySplitPlan(t, engine, plan)
	for _, event := range []splitStreamSupportBean{{"E1", 1}, {"E2", 2}, {"E3", 1}, {"E4", -999}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if got := splitEventStrings(a); !reflect.DeepEqual(got, []string{"E1", "E3"}) {
		t.Fatalf("AStream2S = %#v", got)
	}
	if got := splitEventStrings(b); !reflect.DeepEqual(got, []string{"E1", "E2", "E3"}) {
		t.Fatalf("BStream2S = %#v", got)
	}
	if got := splitEventStrings(fallback); !reflect.DeepEqual(got, []string{"E4"}) {
		t.Fatalf("split fallback = %#v", got)
	}
}

func TestSplitStream3SplitOutputAllParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	registerSplitStringTargets(t, env, "AStream2S", "BStream2S", "CStream2S")
	source := From[splitStreamSupportBean](env, "SupportBean")
	theString := Field[splitStreamSupportBean, string]("theString")
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	plan, err := env.Build(OnEvent(source).SplitAll(
		SplitIntoWhen(Or(Equal[int](intPrimitive, Literal(1)), Equal[int](intPrimitive, Literal(2))), "AStream2S", Alias("theString", Concat(theString, Literal("_1")))),
		SplitIntoWhen(Or(Equal[int](intPrimitive, Literal(2)), Equal[int](intPrimitive, Literal(3))), "BStream2S", Alias("theString", Concat(theString, Literal("_2")))),
		SplitInto("CStream2S", Alias("theString", Concat(theString, Literal("_3")))),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	a := deploySplitConsumer(t, env, engine, "AStream2S")
	b := deploySplitConsumer(t, env, engine, "BStream2S")
	c := deploySplitConsumer(t, env, engine, "CStream2S")
	_, fallback := deploySplitPlan(t, engine, plan)
	for _, event := range []splitStreamSupportBean{{"E1", 2}, {"E2", 1}, {"E3", 3}, {"E4", -999}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if got := splitEventStrings(a); !reflect.DeepEqual(got, []string{"E1_1", "E2_1"}) {
		t.Fatalf("AStream2S = %#v", got)
	}
	if got := splitEventStrings(b); !reflect.DeepEqual(got, []string{"E1_2", "E3_2"}) {
		t.Fatalf("BStream2S = %#v", got)
	}
	if got := splitEventStrings(c); !reflect.DeepEqual(got, []string{"E1_3", "E2_3", "E3_3", "E4_3"}) || len(fallback.events) != 0 {
		t.Fatalf("CStream2S=%#v fallback=%#v", got, fallback.events)
	}
}

func TestSplitStream3SplitDefaultOutputFirstParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	registerSplitStringTargets(t, env, "AStream34", "BStream34", "CStream34")
	source := From[splitStreamSupportBean](env, "SupportBean")
	theString := Field[splitStreamSupportBean, string]("theString")
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	firstPlan, err := env.Build(OnEvent(source).SplitFirst(
		SplitIntoWhen(Equal[int](intPrimitive, Literal(1)), "AStream34", Alias("theString", Concat(theString, Literal("_1")))),
		SplitIntoWhen(Equal[int](intPrimitive, Literal(2)), "BStream34", Alias("theString", Concat(theString, Literal("_2")))),
		SplitInto("CStream34", Alias("theString", Concat(theString, Literal("_3")))),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}
	allPlan, err := env.Build(OnEvent(source).SplitAll(
		SplitIntoWhen(Equal[int](intPrimitive, Literal(1)), "AStream34", Alias("theString", theString)),
	).Query(StatementName("split-all-identity")))
	if err != nil {
		t.Fatal(err)
	}
	if firstPlan.Hash() == allPlan.Hash() {
		t.Fatal("split-first and split-all must have different plan identity")
	}

	engine := NewEngine(env)
	a := deploySplitConsumer(t, env, engine, "AStream34")
	b := deploySplitConsumer(t, env, engine, "BStream34")
	c := deploySplitConsumer(t, env, engine, "CStream34")
	_, fallback := deploySplitPlan(t, engine, firstPlan)
	for _, event := range []splitStreamSupportBean{{"E1", 1}, {"E2", 2}, {"E3", 1}, {"E4", -999}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if got := splitEventStrings(a); !reflect.DeepEqual(got, []string{"E1_1", "E3_1"}) {
		t.Fatalf("AStream34 = %#v", got)
	}
	if got := splitEventStrings(b); !reflect.DeepEqual(got, []string{"E2_2"}) {
		t.Fatalf("BStream34 = %#v", got)
	}
	if got := splitEventStrings(c); !reflect.DeepEqual(got, []string{"E4_3"}) || len(fallback.events) != 0 {
		t.Fatalf("CStream34=%#v fallback=%#v", got, fallback.events)
	}
}

func TestSplitStream4SplitParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	registerSplitStringTargets(t, env, "AStream34", "BStream34", "CStream34", "DStream34")
	source := From[splitStreamSupportBean](env, "SupportBean")
	theString := Field[splitStreamSupportBean, string]("theString")
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	plan, err := env.Build(OnEvent(source).SplitFirst(
		SplitIntoWhen(Equal[int](intPrimitive, Literal(10)), "AStream34", Alias("theString", Concat(theString, Literal("_1")))),
		SplitIntoWhen(Equal[int](intPrimitive, Literal(20)), "BStream34", Alias("theString", Concat(theString, Literal("_2")))),
		SplitIntoWhen(Less[int](intPrimitive, Literal(0)), "CStream34", Alias("theString", Concat(theString, Literal("_3")))),
		SplitInto("DStream34", Alias("theString", Concat(theString, Literal("_4")))),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	a := deploySplitConsumer(t, env, engine, "AStream34")
	b := deploySplitConsumer(t, env, engine, "BStream34")
	c := deploySplitConsumer(t, env, engine, "CStream34")
	d := deploySplitConsumer(t, env, engine, "DStream34")
	_, fallback := deploySplitPlan(t, engine, plan)
	for _, event := range []splitStreamSupportBean{{"E5", -999}, {"E6", 9999}, {"E7", 20}, {"E8", 10}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if got := splitEventStrings(a); !reflect.DeepEqual(got, []string{"E8_1"}) {
		t.Fatalf("AStream34 = %#v", got)
	}
	if got := splitEventStrings(b); !reflect.DeepEqual(got, []string{"E7_2"}) {
		t.Fatalf("BStream34 = %#v", got)
	}
	if got := splitEventStrings(c); !reflect.DeepEqual(got, []string{"E5_3"}) {
		t.Fatalf("CStream34 = %#v", got)
	}
	if got := splitEventStrings(d); !reflect.DeepEqual(got, []string{"E6_4"}) || len(fallback.events) != 0 {
		t.Fatalf("DStream34=%#v fallback=%#v", got, fallback.events)
	}
}

func TestSplitStreamInvalidParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	if _, err := RegisterMap(env, "AStream", []FieldSpec{FieldDef("value", reflect.TypeOf(int(0)))}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[splitStreamSupportBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	source := From[splitStreamSupportBean](env, "SupportBean")
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	if _, err := env.Build(OnEvent(source).SplitFirst().Query()); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("missing split branches error = %v", err)
	}
	if _, err := env.Build(OnEvent(source).SplitFirst(SplitInto("MissingStream")).Query()); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown split target error = %v", err)
	}
	if _, err := env.Build(OnEvent(source).SplitFirst(
		SplitIntoWhen(Literal(1), "AStream", Alias("value", intPrimitive)),
	).Query()); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("non-boolean split condition error = %v", err)
	}
	if _, err := env.Build(OnEvent(source).SplitFirst(
		SplitInto("AStream", Alias("value", Sum[int](intPrimitive))),
	).Query()); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("aggregate split projection error = %v", err)
	}
	if _, err := env.Build(OnEvent(source).SplitFirst(
		SplitFrom(From[splitStreamSupportBeanS0](env, "SupportBean_S0")).Into("AStream", Alias("value", Literal(1))),
	).Query()); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("unrelated split branch source error = %v", err)
	}
	if _, err := env.Build(OnEvent(source).SplitFirst(
		SplitFrom(source.Window(KeepAll())).Into("AStream", Alias("value", Literal(1))),
	).Query()); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("stateful split branch source error = %v", err)
	}
	otherEnv := newSplitStreamEnvironment(t)
	if _, err := env.Build(OnEvent(source).SplitFirst(
		SplitFrom(From[splitStreamSupportBean](otherEnv, "SupportBean")).Into("AStream", Alias("value", Literal(1))),
	).Query()); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("cross-environment split branch source error = %v", err)
	}
}

func TestSplitStreamSubqueryParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	if _, err := RegisterStruct[splitStreamSupportBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"AStreamSub", "BStreamSub"} {
		if _, err := RegisterMap(env, target, []FieldSpec{FieldDef("string", reflect.TypeOf((*any)(nil)).Elem())}); err != nil {
			t.Fatal(err)
		}
	}

	source := From[splitStreamSupportBean](env, "SupportBean")
	last := From[splitStreamSupportBeanS0](env, "SupportBean_S0").Window(LastEvent()).AsRecord()
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	lastID := SubqueryValue[int](last, Field[splitStreamSupportBeanS0, int]("id"))
	plan, err := env.Build(OnEvent(source).SplitFirst(
		SplitIntoWhen(
			Equal[int](intPrimitive, lastID),
			"AStreamSub",
			Alias("string", SubqueryValue[string](last, Field[splitStreamSupportBeanS0, string]("p00"))),
		),
		SplitIntoWhen(
			Or(Not(Equal[int](intPrimitive, lastID)), IsNull[int](lastID)),
			"BStreamSub",
			Alias("string", SubqueryValue[string](last, Field[splitStreamSupportBeanS0, string]("p01"))),
		),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	a := deploySplitConsumer(t, env, engine, "AStreamSub")
	b := deploySplitConsumer(t, env, engine, "BStreamSub")
	_, fallback := deploySplitPlan(t, engine, plan)
	if err := engine.SendEvent(context.Background(), splitStreamSupportBean{"E1", 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), splitStreamSupportBeanS0{ID: 10, P00: "x", P01: "y"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), splitStreamSupportBean{"E2", 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), splitStreamSupportBean{"E3", 9}); err != nil {
		t.Fatal(err)
	}
	if len(a.events) != 1 || a.events[0].Get("string").Any() != "x" {
		t.Fatalf("AStreamSub = %#v", a.events)
	}
	if len(b.events) != 2 || b.events[0].Get("string").Any() != nil || b.events[1].Get("string").Any() != "y" {
		t.Fatalf("BStreamSub = %#v", b.events)
	}
	if len(fallback.events) != 0 {
		t.Fatalf("split fallback = %#v", fallback.events)
	}
}

func TestSplitStreamSubqueryMultikeyWArrayParity(t *testing.T) {
	env := newSplitStreamEnvironment(t)
	if _, err := RegisterStruct[splitStreamIntArrayEvent](env, "SupportEventWithIntArray"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "AValue", []FieldSpec{FieldDef("value", reflect.TypeOf((*any)(nil)).Elem())}); err != nil {
		t.Fatal(err)
	}

	history := From[splitStreamIntArrayEvent](env, "SupportEventWithIntArray").Window(KeepAll()).AsRecord()
	groupedSum := SubqueryGroupScalar[[]int, int](
		history,
		Field[splitStreamIntArrayEvent, []int]("array"),
		Sum[int](Field[splitStreamIntArrayEvent, int]("value")),
	)
	intPrimitive := Field[splitStreamSupportBean, int]("intPrimitive")
	plan, err := env.Build(OnEvent(From[splitStreamSupportBean](env, "SupportBean")).SplitFirst(
		SplitIntoWhen(Greater[int](intPrimitive, Literal(0)), "AValue", Alias("value", groupedSum)),
		SplitIntoWhen(LessOrEqual[int](intPrimitive, Literal(0)), "AValue", Alias("value", Literal(0))),
	).Query(StatementName("split")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	values := deploySplitConsumer(t, env, engine, "AValue")
	_, fallback := deploySplitPlan(t, engine, plan)
	sendArray := func(id string, array []int, value int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), splitStreamIntArrayEvent{ID: id, Array: array, Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendTrigger := func(id string, value int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), splitStreamSupportBean{TheString: id, IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
	}

	sendArray("E1", []int{1, 2}, 10)
	sendArray("E2", []int{1, 2}, 11)
	sendTrigger("X", 0)
	sendTrigger("Y", 1)
	sendArray("E3", []int{1, 2}, 12)
	sendTrigger("Y", 1)
	sendArray("E4", []int{1}, 13)
	sendTrigger("Y", 1)

	want := []any{0, 21, 33, nil}
	if len(values.events) != len(want) {
		t.Fatalf("AValue events = %#v", values.events)
	}
	for index, expected := range want {
		if got := values.events[index].Get("value").Any(); got != expected {
			t.Fatalf("AValue[%d] = %#v, want %#v", index, got, expected)
		}
	}
	if len(fallback.events) != 0 {
		t.Fatalf("split fallback = %#v", fallback.events)
	}
}
