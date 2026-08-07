package esper

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

type namedWindowMergeContainer struct {
	Beans []runtimeTestTrade `esper:"beans"`
}

type namedWindowMergeEvent struct {
	ID string `esper:"id"`
}

type namedWindowMergeDispatchTrade struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type namedWindowMergeBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func (b namedWindowMergeBean) ExtractString() string { return b.TheString }

type namedWindowMergeUpdateSignal struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type namedWindowMergeSetterTarget struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int     `esper:"intPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
	DoubleBoxed     float64 `esper:"doubleBoxed"`
}

func (b *namedWindowMergeSetterTarget) SetDoublePrimitive(value float64) {
	b.DoublePrimitive = value
}

type namedWindowMergeDocOrder struct {
	OrderID     string  `esper:"orderId" json:"orderId"`
	ProductID   string  `esper:"productId" json:"productId"`
	Price       float64 `esper:"price" json:"price"`
	Quantity    int     `esper:"quantity" json:"quantity"`
	DeletedFlag bool    `esper:"deletedFlag" json:"deletedFlag"`
}

type namedWindowMergeDocProduct struct {
	ProductID  string  `esper:"productId" json:"productId"`
	TotalPrice float64 `esper:"totalPrice" json:"totalPrice"`
}

type namedWindowMergeSubqueryEvent struct {
	In1 string `esper:"in1" json:"in1"`
	In2 int    `esper:"in2" json:"in2"`
}

type namedWindowMergeSubquerySchema struct {
	Col1 string `esper:"col1" json:"col1"`
	Col2 int    `esper:"col2" json:"col2"`
}

type namedWindowMergeSubquerySupport struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type namedWindowMergeSubqueryDelete struct {
	ID string `esper:"id"`
}

// TestNamedWindowMergeInsertOnlyContainedEventsMatchesEsper covers the
// insert-select-star/no-where and transpose shapes in
// InfraNamedWindowOnMerge. Unnest is the explicit Go counterpart of the Java
// contained property source and CopyMatchingFields is the fluent wildcard
// projection.
func TestNamedWindowMergeInsertOnlyContainedEventsMatchesEsper(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		match       Expression[bool]
		javaRuntime string
	}{
		{name: "where-false", match: Literal(false), javaRuntime: "java-runtime-be3de825a8f0e5500e65"},
		{name: "no-where", match: nil, javaRuntime: "java-runtime-7df664f5a9da669db1ed"},
		{name: "transpose-equivalent", match: Literal(false), javaRuntime: "java-runtime-cd06e6370855e3ded695"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[runtimeTestTrade](env, "NamedWindowMergeTrade"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[namedWindowMergeContainer](env, "NamedWindowMergeContainer"); err != nil {
				t.Fatal(err)
			}
			targetSchema, err := RegisterMap(env, "NamedWindowMergeContainedTarget", []FieldSpec{
				FieldDef("symbol", reflect.TypeOf("")),
				FieldDef("price", reflect.TypeOf(float64(0))),
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := CreateNamedWindow(env, "NamedWindowMergeContainedTarget", targetSchema, NamedWindowRetention(KeepAll())); err != nil {
				t.Fatal(err)
			}

			children := Unnest[namedWindowMergeContainer, runtimeTestTrade](
				From[namedWindowMergeContainer](env, "NamedWindowMergeContainer"),
				Property[[]runtimeTestTrade](EventValue[namedWindowMergeContainer](), "beans"),
			)
			symbol := Field[runtimeTestTrade, string]("symbol")
			plan, err := env.Build(OnEvent(children).MergeIntoNamedWindowWhen(
				"NamedWindowMergeContainedTarget",
				testCase.match,
				WhenNotMatchedAny(
					SetColumn("symbol", symbol),
					SetColumn("price", Field[runtimeTestTrade, float64]("price")),
				),
			).Query(StatementName("named-window-merge-contained-" + testCase.name)))
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			if _, err := engine.Deploy(context.Background(), plan); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), namedWindowMergeContainer{Beans: []runtimeTestTrade{
				{Symbol: "E1", Price: 10},
				{Symbol: "E2", Price: 20},
			}}); err != nil {
				t.Fatal(err)
			}
			window, ok := engine.NamedWindow("NamedWindowMergeContainedTarget")
			if !ok {
				t.Fatal("contained target named window is missing")
			}
			events, err := window.Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 2 {
				t.Fatalf("%s contained insert snapshot = %#v, want two rows", testCase.javaRuntime, events)
			}
			for index, want := range []struct {
				symbol string
				price  float64
			}{{"E1", 10}, {"E2", 20}} {
				if events[index].Get("symbol").Any() != want.symbol || events[index].Get("price").Any() != want.price {
					t.Fatalf("%s contained row %d = %#v, want %#v", testCase.javaRuntime, index, events[index], want)
				}
			}
		})
	}
}

// TestNamedWindowMergePropertyInsertAndMethodProjectionMatchesEsper maps
// InfraPropertyInsertBean and InfraNamedWindowOnMergeSamePropertyNameAsStreamName.
// The first deployment intentionally assigns only the numeric field so the
// target's omitted property remains Null; the second deployment uses a Go
// receiver method as the explicit, non-EPL projection and preserves both rows.
func TestNamedWindowMergePropertyInsertAndMethodProjectionMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[namedWindowMergeBean](env, "NamedWindowMergePropertyInput"); err != nil {
		t.Fatal(err)
	}
	targetSchema, err := RegisterMap(env, "NamedWindowMergePropertyTarget", []FieldSpec{
		OptionalFieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int(0))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "NamedWindowMergePropertyTarget", targetSchema, NamedWindowRetention(Unique(Field[any, string]("theString")))); err != nil {
		t.Fatal(err)
	}

	source := From[namedWindowMergeBean](env, "NamedWindowMergePropertyInput")
	key := Field[namedWindowMergeBean, string]("theString")
	value := Field[namedWindowMergeBean, int]("intPrimitive")
	match := Equal[string](NamedWindowField[string]("theString"), key)
	firstPlan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("NamedWindowMergePropertyTarget", match,
		WhenNotMatchedAny(SetColumn("intPrimitive", value)),
	).Query(StatementName("named-window-merge-property-first")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	firstDeployment, err := engine.Deploy(context.Background(), firstPlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), namedWindowMergeBean{TheString: "E1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	firstDeployment.Undeploy(context.Background())

	methodProjection := Method[string](EventValue[namedWindowMergeBean](), "ExtractString")
	secondPlan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("NamedWindowMergePropertyTarget", match,
		WhenNotMatchedAny(SetColumn("theString", methodProjection), SetColumn("intPrimitive", value)),
	).Query(StatementName("named-window-merge-property-second")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), secondPlan); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), namedWindowMergeBean{TheString: "E2", IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("NamedWindowMergePropertyTarget")
	if !ok {
		t.Fatal("property target named window is missing")
	}
	events, err := window.Snapshot(context.Background())
	if err != nil || len(events) != 2 {
		t.Fatalf("property insert snapshot = %#v, err=%v", events, err)
	}
	if !events[0].Get("theString").IsNull() || events[0].Get("intPrimitive").Any() != 10 {
		t.Fatalf("property insert first row = %#v", events[0])
	}
	if events[1].Get("theString").Any() != "E2" || events[1].Get("intPrimitive").Any() != 20 {
		t.Fatalf("property insert second row = %#v", events[1])
	}
}

// TestNamedWindowMergeUpdateNonPropertySetMatchesEsper maps
// InfraNamedWindowOnMergeUpdateNonPropertySet. The Java rule invokes a
// setter and a side-effecting UDF; the Go form keeps those effects explicit in
// the fluent assignment list: the registered setter receives the trigger id,
// the initial target value drives intPrimitive, and the working target value
// drives doubleBoxed.
func TestNamedWindowMergeUpdateNonPropertySetMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[namedWindowMergeUpdateSignal](env, "NamedWindowMergeUpdateSignal"); err != nil {
		t.Fatal(err)
	}
	targetSchema, err := RegisterStruct[namedWindowMergeSetterTarget](env, "NamedWindowMergeSetterTarget", WithAccessorStyle(AccessorJavaBean))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "NamedWindowMergeSetterTarget", targetSchema, NamedWindowRetention(Unique(Field[any, string]("theString")))); err != nil {
		t.Fatal(err)
	}

	source := From[namedWindowMergeUpdateSignal](env, "NamedWindowMergeUpdateSignal")
	match := Equal[string](NamedWindowField[string]("theString"), Field[namedWindowMergeUpdateSignal, string]("p00"))
	plan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("NamedWindowMergeSetterTarget", match,
		WhenMatchedActions(
			ThenUpdate(Literal(true),
				SetColumn("doublePrimitive", Cast[int, float64](Field[namedWindowMergeUpdateSignal, int]("id"))),
				SetColumn("intPrimitive", Add[int](InitialNamedWindowField[int]("intPrimitive"), Literal(1))),
				SetColumn("doubleBoxed", NamedWindowField[float64]("doublePrimitive")),
			),
		),
	).Query(StatementName("named-window-merge-setter")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.InsertNamedWindow(context.Background(), "NamedWindowMergeSetterTarget", namedWindowMergeSetterTarget{
		TheString: "E1", IntPrimitive: 10, DoublePrimitive: 2,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), namedWindowMergeUpdateSignal{ID: 5, P00: "E1"}); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("NamedWindowMergeSetterTarget")
	if !ok {
		t.Fatal("setter target named window is missing")
	}
	events, err := window.Snapshot(context.Background())
	if err != nil || len(events) != 1 {
		t.Fatalf("setter target snapshot = %#v, err=%v", events, err)
	}
	if got := events[0]; got.Get("intPrimitive").Any() != 11 || got.Get("doublePrimitive").Any() != 5.0 || got.Get("doubleBoxed").Any() != 5.0 {
		t.Fatalf("setter target updated row = %#v", got)
	}
}

// TestNamedWindowMergeDocExampleRepresentationMatrix maps InfraDocExample.
// The same fluent merge plan is built for the six Java event representations;
// only registration and ingestion change at the boundary.
func TestNamedWindowMergeDocExampleRepresentationMatrix(t *testing.T) {
	fields := []FieldSpec{
		FieldDef("orderId", reflect.TypeOf("")),
		FieldDef("productId", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
		FieldDef("quantity", reflect.TypeOf(int(0))),
		FieldDef("deletedFlag", reflect.TypeOf(false)),
	}
	productFields := []FieldSpec{
		FieldDef("productId", reflect.TypeOf("")),
		FieldDef("totalPrice", reflect.TypeOf(float64(0))),
	}
	for _, representation := range []string{"map", "object-array", "avro", "json", "json-class-provided", "default"} {
		t.Run(representation, func(t *testing.T) {
			env := NewEnvironment()
			var (
				orderSchema   Schema
				productSchema Schema
				sendOrder     func(*Engine, namedWindowMergeDocOrder) error
			)
			var err error
			switch representation {
			case "map":
				orderSchema, err = RegisterMap(env, "NamedWindowMergeDocOrder", fields)
				if err != nil {
					t.Fatal(err)
				}
				productSchema, err = RegisterMap(env, "NamedWindowMergeDocProduct", productFields)
				if err != nil {
					t.Fatal(err)
				}
				sendOrder = func(engine *Engine, order namedWindowMergeDocOrder) error {
					return engine.SendRecord(context.Background(), "NamedWindowMergeDocOrder", map[string]any{
						"orderId": order.OrderID, "productId": order.ProductID, "price": order.Price,
						"quantity": order.Quantity, "deletedFlag": order.DeletedFlag,
					})
				}
			case "object-array":
				orderSchema, err = RegisterObjectArray(env, "NamedWindowMergeDocOrder", fields)
				if err != nil {
					t.Fatal(err)
				}
				productSchema, err = RegisterObjectArray(env, "NamedWindowMergeDocProduct", productFields)
				if err != nil {
					t.Fatal(err)
				}
				sendOrder = func(engine *Engine, order namedWindowMergeDocOrder) error {
					return engine.SendObjectArray(context.Background(), "NamedWindowMergeDocOrder", []any{
						order.OrderID, order.ProductID, order.Price, order.Quantity, order.DeletedFlag,
					})
				}
			case "avro":
				orderSchema, err = RegisterAvro(env, "NamedWindowMergeDocOrder", fields)
				if err != nil {
					t.Fatal(err)
				}
				productSchema, err = RegisterAvro(env, "NamedWindowMergeDocProduct", productFields)
				if err != nil {
					t.Fatal(err)
				}
				sendOrder = func(engine *Engine, order namedWindowMergeDocOrder) error {
					record, recordErr := NewAvroRecord(orderSchema)
					if recordErr != nil {
						return recordErr
					}
					for name, value := range map[string]any{
						"orderId": order.OrderID, "productId": order.ProductID, "price": order.Price,
						"quantity": order.Quantity, "deletedFlag": order.DeletedFlag,
					} {
						if recordErr = record.Set(name, value); recordErr != nil {
							return recordErr
						}
					}
					return engine.SendAvro(context.Background(), "NamedWindowMergeDocOrder", record)
				}
			case "json":
				orderSchema, err = RegisterJSON(env, "NamedWindowMergeDocOrder", fields)
				if err != nil {
					t.Fatal(err)
				}
				productSchema, err = RegisterJSON(env, "NamedWindowMergeDocProduct", productFields)
				if err != nil {
					t.Fatal(err)
				}
				sendOrder = func(engine *Engine, order namedWindowMergeDocOrder) error {
					data, marshalErr := json.Marshal(order)
					if marshalErr != nil {
						return marshalErr
					}
					return engine.SendJSON(context.Background(), "NamedWindowMergeDocOrder", data)
				}
			case "json-class-provided":
				orderSchema, err = RegisterJSONFor[namedWindowMergeDocOrder](env, "NamedWindowMergeDocOrder", nil)
				if err != nil {
					t.Fatal(err)
				}
				productSchema, err = RegisterJSONFor[namedWindowMergeDocProduct](env, "NamedWindowMergeDocProduct", nil)
				if err != nil {
					t.Fatal(err)
				}
				sendOrder = func(engine *Engine, order namedWindowMergeDocOrder) error {
					data, marshalErr := json.Marshal(order)
					if marshalErr != nil {
						return marshalErr
					}
					return engine.SendJSON(context.Background(), "NamedWindowMergeDocOrder", data)
				}
			case "default":
				orderSchema, err = RegisterStruct[namedWindowMergeDocOrder](env, "NamedWindowMergeDocOrder")
				if err != nil {
					t.Fatal(err)
				}
				productSchema, err = RegisterStruct[namedWindowMergeDocProduct](env, "NamedWindowMergeDocProduct")
				if err != nil {
					t.Fatal(err)
				}
				sendOrder = func(engine *Engine, order namedWindowMergeDocOrder) error {
					return engine.SendEvent(context.Background(), order)
				}
			}

			if _, err := CreateNamedWindow(env, "NamedWindowMergeDocProductWindow", productSchema, NamedWindowRetention(Unique(Field[any, string]("productId")))); err != nil {
				t.Fatal(err)
			}
			if _, err := CreateNamedWindow(env, "NamedWindowMergeDocOrderWindow", orderSchema, NamedWindowRetention(KeepAll())); err != nil {
				t.Fatal(err)
			}
			source := FromAny(env, "NamedWindowMergeDocOrder")
			orderID := Field[any, string]("orderId")
			productID := Field[any, string]("productId")
			price := Field[any, float64]("price")
			quantity := Field[any, int]("quantity")
			deleted := Field[any, bool]("deletedFlag")
			productPlan, err := env.Build(OnRecord(source).MergeIntoNamedWindowWhen("NamedWindowMergeDocProductWindow",
				Equal[string](NamedWindowField[string]("productId"), productID),
				WhenMatchedAny(SetColumn("totalPrice", Add[float64](NamedWindowField[float64]("totalPrice"), price))),
				WhenNotMatchedAny(SetColumn("productId", productID), SetColumn("totalPrice", price)),
			).Query(StatementName("named-window-merge-doc-product")))
			if err != nil {
				t.Fatal(err)
			}
			orderPlan, err := env.Build(OnRecord(source).MergeIntoNamedWindowWhen("NamedWindowMergeDocOrderWindow",
				Equal[string](NamedWindowField[string]("orderId"), orderID),
				WhenMatchedActions(
					ThenDelete(deleted),
					ThenUpdate(Literal(true), SetColumn("quantity", quantity), SetColumn("price", price)),
				),
				WhenNotMatchedAny(CopyMatchingFields()),
			).Query(StatementName("named-window-merge-doc-order")))
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			if _, err := engine.Deploy(context.Background(), productPlan); err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), orderPlan); err != nil {
				t.Fatal(err)
			}
			for _, order := range []namedWindowMergeDocOrder{
				{OrderID: "O1", ProductID: "P1", Price: 10, Quantity: 100},
				{OrderID: "O1", ProductID: "P1", Price: 11, Quantity: 200},
				{OrderID: "O2", ProductID: "P2", Price: 3, Quantity: 300},
			} {
				if err := sendOrder(engine, order); err != nil {
					t.Fatal(err)
				}
			}
			productWindow, ok := engine.NamedWindow("NamedWindowMergeDocProductWindow")
			if !ok {
				t.Fatal("doc product named window is missing")
			}
			products, err := productWindow.Snapshot(context.Background())
			if err != nil || len(products) != 2 {
				t.Fatalf("%s product snapshot = %#v, err=%v", representation, products, err)
			}
			if products[0].Get("productId").Any() != "P1" || products[0].Get("totalPrice").Any() != 21.0 || products[1].Get("productId").Any() != "P2" || products[1].Get("totalPrice").Any() != 3.0 {
				t.Fatalf("%s product rows = %#v", representation, products)
			}
			orderWindow, ok := engine.NamedWindow("NamedWindowMergeDocOrderWindow")
			if !ok {
				t.Fatal("doc order named window is missing")
			}
			orders, err := orderWindow.Snapshot(context.Background())
			if err != nil || len(orders) != 2 {
				t.Fatalf("%s order snapshot = %#v, err=%v", representation, orders, err)
			}
			if orders[0].Get("orderId").Any() != "O1" || orders[0].Get("quantity").Any() != 200 || orders[1].Get("orderId").Any() != "O2" || orders[1].Get("quantity").Any() != 300 {
				t.Fatalf("%s order rows = %#v", representation, orders)
			}
		})
	}
}

// TestNamedWindowMergeSubselectRepresentationMatrix maps InfraSubselect. Each
// representation uses three independent filtered LastEvent subquery sources
// for the A/B/C control streams, reproducing the Java rule's insert, update
// and delete branches without embedding a textual query.
func TestNamedWindowMergeSubselectRepresentationMatrix(t *testing.T) {
	fields := []FieldSpec{
		FieldDef("in1", reflect.TypeOf("")),
		FieldDef("in2", reflect.TypeOf(int(0))),
	}
	targetFields := []FieldSpec{
		FieldDef("col1", reflect.TypeOf("")),
		FieldDef("col2", reflect.TypeOf(int(0))),
	}
	for _, representation := range []string{"map", "object-array", "avro", "json", "json-class-provided", "default"} {
		t.Run(representation, func(t *testing.T) {
			env := NewEnvironment()
			var (
				eventSchema  Schema
				targetSchema Schema
				sendEvent    func(*Engine, string, int) error
			)
			var err error
			switch representation {
			case "map":
				eventSchema, err = RegisterMap(env, "NamedWindowMergeSubqueryEvent", fields)
				if err != nil {
					t.Fatal(err)
				}
				targetSchema, err = RegisterMap(env, "NamedWindowMergeSubquerySchema", targetFields)
				if err != nil {
					t.Fatal(err)
				}
				sendEvent = func(engine *Engine, value string, number int) error {
					return engine.SendRecord(context.Background(), "NamedWindowMergeSubqueryEvent", map[string]any{"in1": value, "in2": number})
				}
			case "object-array":
				eventSchema, err = RegisterObjectArray(env, "NamedWindowMergeSubqueryEvent", fields)
				if err != nil {
					t.Fatal(err)
				}
				targetSchema, err = RegisterObjectArray(env, "NamedWindowMergeSubquerySchema", targetFields)
				if err != nil {
					t.Fatal(err)
				}
				sendEvent = func(engine *Engine, value string, number int) error {
					return engine.SendObjectArray(context.Background(), "NamedWindowMergeSubqueryEvent", []any{value, number})
				}
			case "avro":
				eventSchema, err = RegisterAvro(env, "NamedWindowMergeSubqueryEvent", fields)
				if err != nil {
					t.Fatal(err)
				}
				targetSchema, err = RegisterAvro(env, "NamedWindowMergeSubquerySchema", targetFields)
				if err != nil {
					t.Fatal(err)
				}
				sendEvent = func(engine *Engine, value string, number int) error {
					record, recordErr := NewAvroRecord(eventSchema)
					if recordErr != nil {
						return recordErr
					}
					if recordErr = record.Set("in1", value); recordErr != nil {
						return recordErr
					}
					if recordErr = record.Set("in2", number); recordErr != nil {
						return recordErr
					}
					return engine.SendAvro(context.Background(), "NamedWindowMergeSubqueryEvent", record)
				}
			case "json":
				eventSchema, err = RegisterJSON(env, "NamedWindowMergeSubqueryEvent", fields)
				if err != nil {
					t.Fatal(err)
				}
				targetSchema, err = RegisterJSON(env, "NamedWindowMergeSubquerySchema", targetFields)
				if err != nil {
					t.Fatal(err)
				}
				sendEvent = func(engine *Engine, value string, number int) error {
					data, marshalErr := json.Marshal(namedWindowMergeSubqueryEvent{In1: value, In2: number})
					if marshalErr != nil {
						return marshalErr
					}
					return engine.SendJSON(context.Background(), "NamedWindowMergeSubqueryEvent", data)
				}
			case "json-class-provided":
				eventSchema, err = RegisterJSONFor[namedWindowMergeSubqueryEvent](env, "NamedWindowMergeSubqueryEvent", nil)
				if err != nil {
					t.Fatal(err)
				}
				targetSchema, err = RegisterJSONFor[namedWindowMergeSubquerySchema](env, "NamedWindowMergeSubquerySchema", nil)
				if err != nil {
					t.Fatal(err)
				}
				sendEvent = func(engine *Engine, value string, number int) error {
					data, marshalErr := json.Marshal(namedWindowMergeSubqueryEvent{In1: value, In2: number})
					if marshalErr != nil {
						return marshalErr
					}
					return engine.SendJSON(context.Background(), "NamedWindowMergeSubqueryEvent", data)
				}
			case "default":
				eventSchema, err = RegisterStruct[namedWindowMergeSubqueryEvent](env, "NamedWindowMergeSubqueryEvent")
				if err != nil {
					t.Fatal(err)
				}
				targetSchema, err = RegisterStruct[namedWindowMergeSubquerySchema](env, "NamedWindowMergeSubquerySchema")
				if err != nil {
					t.Fatal(err)
				}
				sendEvent = func(engine *Engine, value string, number int) error {
					return engine.SendEvent(context.Background(), namedWindowMergeSubqueryEvent{In1: value, In2: number})
				}
			}
			if eventSchema.Name() == "" || targetSchema.Name() == "" {
				t.Fatal("subquery representation schemas are missing")
			}
			if _, err := RegisterStruct[namedWindowMergeSubquerySupport](env, "NamedWindowMergeSubquerySupport"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[namedWindowMergeSubqueryDelete](env, "NamedWindowMergeSubqueryDelete"); err != nil {
				t.Fatal(err)
			}
			if _, err := CreateNamedWindow(env, "NamedWindowMergeSubqueryTarget", targetSchema, NamedWindowRetention(LastEvent())); err != nil {
				t.Fatal(err)
			}

			support := From[namedWindowMergeSubquerySupport](env, "NamedWindowMergeSubquerySupport")
			fieldString := Field[namedWindowMergeSubquerySupport, string]("theString")
			supportA := support.Filter(StartsWith(fieldString, Literal("A"))).Window(LastEvent()).AsRecord()
			supportB := support.Filter(StartsWith(fieldString, Literal("B"))).Window(LastEvent()).AsRecord()
			supportC := support.Filter(StartsWith(fieldString, Literal("C"))).Window(LastEvent()).AsRecord()
			aValue := SubqueryValue[int](supportA, Field[any, int]("intPrimitive"))
			aName := SubqueryValue[string](supportA, Field[any, string]("theString"))
			bValue := SubqueryValue[int](supportB, Field[any, int]("intPrimitive"))
			bName := SubqueryValue[string](supportB, Field[any, string]("theString"))
			cValue := SubqueryValue[int](supportC, Field[any, int]("intPrimitive"))
			deletePlan, err := env.Build(OnEvent(From[namedWindowMergeSubqueryDelete](env, "NamedWindowMergeSubqueryDelete")).DeleteAllFromNamedWindow("NamedWindowMergeSubqueryTarget").Query(StatementName("named-window-merge-subquery-delete")))
			if err != nil {
				t.Fatal(err)
			}
			mergePlan, err := env.Build(OnRecord(FromAny(env, "NamedWindowMergeSubqueryEvent")).MergeIntoNamedWindowWhen("NamedWindowMergeSubqueryTarget", nil,
				WhenMatched(Greater[int](bValue, Literal(0)), SetColumn("col1", bName), SetColumn("col2", bValue)),
				WhenMatchedDelete(Greater[int](cValue, Literal(0))),
				WhenNotMatched(Greater[int](aValue, Literal(0)), SetColumn("col1", aName), SetColumn("col2", aValue)),
			).Query(StatementName("named-window-merge-subquery")))
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), mergePlan); err != nil {
				t.Fatal(err)
			}
			sendSupport := func(name string, value int) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), namedWindowMergeSubquerySupport{TheString: name, IntPrimitive: value}); err != nil {
					t.Fatal(err)
				}
			}
			snapshot := func() []Event {
				t.Helper()
				window, ok := engine.NamedWindow("NamedWindowMergeSubqueryTarget")
				if !ok {
					t.Fatal("subquery target named window is missing")
				}
				events, snapshotErr := window.Snapshot(context.Background())
				if snapshotErr != nil {
					t.Fatal(snapshotErr)
				}
				return events
			}
			send := func(name string, value int) {
				t.Helper()
				if err := sendEvent(engine, name, value); err != nil {
					t.Fatal(err)
				}
			}
			send("X1", 1)
			if got := snapshot(); len(got) != 0 {
				t.Fatalf("%s X1 target = %#v", representation, got)
			}
			sendSupport("A1", 0)
			send("X2", 2)
			if got := snapshot(); len(got) != 0 {
				t.Fatalf("%s X2 target = %#v", representation, got)
			}
			sendSupport("A2", 20)
			send("X3", 3)
			if got := snapshot(); len(got) != 1 || got[0].Get("col1").Any() != "A2" || got[0].Get("col2").Any() != 20 {
				t.Fatalf("%s A insert target = %#v", representation, got)
			}
			if err := engine.SendEvent(context.Background(), namedWindowMergeSubqueryDelete{ID: "Y1"}); err != nil {
				t.Fatal(err)
			}
			if got := snapshot(); len(got) != 0 {
				t.Fatalf("%s delete target = %#v", representation, got)
			}
			sendSupport("A3", 30)
			send("X4", 4)
			sendSupport("A4", 40)
			send("X5", 5)
			if got := snapshot(); len(got) != 1 || got[0].Get("col1").Any() != "A3" || got[0].Get("col2").Any() != 30 {
				t.Fatalf("%s matched-without-B target = %#v", representation, got)
			}
			sendSupport("B1", 50)
			send("X6", 6)
			sendSupport("B2", 60)
			send("X7", 7)
			sendSupport("B2", 0)
			send("X8", 8)
			if got := snapshot(); len(got) != 1 || got[0].Get("col1").Any() != "B2" || got[0].Get("col2").Any() != 60 {
				t.Fatalf("%s B update target = %#v", representation, got)
			}
			sendSupport("C1", 1)
			send("X9", 9)
			if got := snapshot(); len(got) != 0 {
				t.Fatalf("%s C delete target = %#v", representation, got)
			}
			sendSupport("C1", 0)
			send("X10", 10)
			if got := snapshot(); len(got) != 1 || got[0].Get("col1").Any() != "A4" || got[0].Get("col2").Any() != 40 {
				t.Fatalf("%s final A fallback target = %#v", representation, got)
			}
		})
	}
}

// TestNamedWindowMergeAssignsCurrentAndPreviousEventsMatchesEsper maps
// InfraOnMergeSetRHSEvent. Event-valued target columns retain the original
// Event envelope so a later matched assignment can shift current to previous
// without losing the underlying source event.
func TestNamedWindowMergeAssignsCurrentAndPreviousEventsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[namedWindowMergeEvent](env, "NamedWindowMergeRHSInput"); err != nil {
		t.Fatal(err)
	}
	targetSchema, err := RegisterMap(env, "NamedWindowMergeRHSTarget", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		OptionalFieldDef("current", reflect.TypeOf(Event{})),
		OptionalFieldDef("previous", reflect.TypeOf(Event{})),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "NamedWindowMergeRHSTarget", targetSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	source := From[namedWindowMergeEvent](env, "NamedWindowMergeRHSInput")
	input := EventValue[Event]()
	match := Equal[string](NamedWindowField[string]("id"), Literal("id"))
	plan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("NamedWindowMergeRHSTarget", match,
		WhenNotMatchedAny(
			SetColumn("id", Literal("id")),
			SetColumn("current", input),
			SetColumn("previous", input),
		),
		WhenMatchedAny(
			SetColumn("previous", NamedWindowField[Event]("current")),
			SetColumn("current", input),
		),
	).Query(StatementName("named-window-merge-rhs-event")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"A", "B", "C"} {
		if err := engine.SendEvent(context.Background(), namedWindowMergeEvent{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	window, ok := engine.NamedWindow("NamedWindowMergeRHSTarget")
	if !ok {
		t.Fatal("RHS target named window is missing")
	}
	events, err := window.Snapshot(context.Background())
	if err != nil || len(events) != 1 {
		t.Fatalf("RHS event snapshot = %#v, err=%v", events, err)
	}
	current, currentOK := events[0].Get("current").Any().(Event)
	previous, previousOK := events[0].Get("previous").Any().(Event)
	if !currentOK || !previousOK || current.Underlying().(namedWindowMergeEvent).ID != "C" || previous.Underlying().(namedWindowMergeEvent).ID != "B" {
		t.Fatalf("RHS current/previous = %#v/%#v", events[0].Get("current"), events[0].Get("previous"))
	}
}

// TestNamedWindowMergeTriggeredByNamedWindowDispatchMatchesEsper covers the
// first half of InfraMergeTriggeredByAnotherWindow: a named-window insert is
// itself a source for a second merge and a side stream is emitted only for the
// not-matched branch.
func TestNamedWindowMergeTriggeredByNamedWindowDispatchMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{
		FieldDef("id", reflect.TypeOf(int(0))),
	}
	for _, name := range []string{"NamedWindowMergeDispatchA", "NamedWindowMergeDispatchB"} {
		schema, err := RegisterMap(env, name, fields)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNamedWindow(env, name, schema, NamedWindowRetention(Unique(Field[any, int]("id")))); err != nil {
			t.Fatal(err)
		}
	}
	outputSchema, err := RegisterMap(env, "NamedWindowMergeDispatchOutput", []FieldSpec{
		FieldDef("id", reflect.TypeOf(int(0))),
		FieldDef("insert", reflect.TypeOf(true)),
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = outputSchema
	input := FromNamedWindow(env, "NamedWindowMergeDispatchA")
	id := Field[any, int]("id")
	plan, err := env.Build(OnRecord(input).MergeIntoNamedWindowWhen("NamedWindowMergeDispatchB",
		Equal[int](NamedWindowField[int]("id"), id),
		WhenNotMatchedActions(
			ThenInsertInto("NamedWindowMergeDispatchOutput",
				Alias("id", id),
				Alias("insert", Literal(true)),
			),
			ThenInsertIntoTarget(CopyMatchingFields()),
		),
	).Query(StatementName("named-window-merge-dispatch")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var side []Event
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				side = append(side, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, idValue := range []int{1, 2, 1} {
		if err := engine.InsertNamedWindow(context.Background(), "NamedWindowMergeDispatchA", map[string]any{"id": idValue}); err != nil {
			t.Fatal(err)
		}
	}
	if len(side) != 2 || side[0].Get("id").Any() != 1 || side[1].Get("id").Any() != 2 {
		t.Fatalf("named-window dispatch side stream = %#v", side)
	}
	target, ok := engine.NamedWindow("NamedWindowMergeDispatchB")
	if !ok {
		t.Fatal("dispatch target named window is missing")
	}
	events, err := target.Snapshot(context.Background())
	if err != nil || len(events) != 2 {
		t.Fatalf("named-window dispatch target = %#v, err=%v", events, err)
	}

	// The same Java execution also verifies insert-stream-only dispatch from
	// one named window to another. In Go, inserting into W1 is an explicit
	// runtime operation and the merge source remains a named-window stream.
	tradeSchema, err := RegisterStruct[namedWindowMergeDispatchTrade](env, "NamedWindowMergeDispatchTrade")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "NamedWindowMergeDispatchW1", tradeSchema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "NamedWindowMergeDispatchW2", tradeSchema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}
	if _, ok := engine.NamedWindow("NamedWindowMergeDispatchW1"); !ok {
		t.Fatal("insert-stream source named window is missing")
	}
	if _, ok := engine.NamedWindow("NamedWindowMergeDispatchW2"); !ok {
		t.Fatal("insert-stream target named window is missing")
	}
	if _, err := RegisterMap(env, "NamedWindowMergeDispatchOut", []FieldSpec{
		FieldDef("c0", reflect.TypeOf("")),
		FieldDef("c1", reflect.TypeOf(true)),
	}); err != nil {
		t.Fatal(err)
	}
	mergePlan, err := env.Build(OnRecord(FromNamedWindow(env, "NamedWindowMergeDispatchW1")).MergeIntoNamedWindowWhen("NamedWindowMergeDispatchW2", nil,
		WhenNotMatchedActions(
			ThenInsertInto("NamedWindowMergeDispatchOut",
				Alias("c0", Field[any, string]("theString")),
				Alias("c1", Literal(true)),
			),
			ThenInsertIntoTarget(CopyMatchingFields()),
		),
	).Query(StatementName("named-window-merge-dispatch-side-stream")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "NamedWindowMergeDispatchOut").Query(StatementName("named-window-merge-dispatch-side-stream-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var sideStream []Event
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				sideStream = append(sideStream, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), mergePlan); err != nil {
		t.Fatal(err)
	}
	for _, trade := range []namedWindowMergeDispatchTrade{{TheString: "E1", IntPrimitive: 1}, {TheString: "E2", IntPrimitive: 2}} {
		if err := engine.InsertNamedWindow(context.Background(), "NamedWindowMergeDispatchW1", trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(sideStream) != 2 || sideStream[0].Get("c0").Any() != "E1" || sideStream[1].Get("c0").Any() != "E2" || sideStream[0].Get("c1").Any() != true || sideStream[1].Get("c1").Any() != true {
		t.Fatalf("named-window insert-stream-only side stream = %#v", sideStream)
	}
}
