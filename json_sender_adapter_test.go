package esper

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

type jsonAdapterPoint struct {
	X int
	Y int
}

type jsonAdapterSource struct {
	Point  jsonAdapterPoint `esper:"point"`
	MyDate time.Time        `esper:"myDate"`
}

func TestJSONEventSenderParseSendAndRawShapeMatchEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterJSON(env, "SenderEvent", []FieldSpec{
		FieldDef("p1", reflect.TypeOf("")),
		FieldDef("score", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "SenderEvent").Query(StatementName("json-sender-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	received := make(chan Event, 2)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				received <- event
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sender, err := engine.JSONSender("SenderEvent")
	if err != nil {
		t.Fatal(err)
	}
	if sender.EventType() != "SenderEvent" || sender.Schema().Name() != "SenderEvent" {
		t.Fatalf("sender metadata = %q/%q", sender.EventType(), sender.Schema().Name())
	}
	parsed, err := sender.Parse([]byte(`{"p1":"abc","score":1.0}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.SendEvent(context.Background(), parsed); err != nil {
		t.Fatal(err)
	}
	first := <-received
	if first.Get("p1").Any() != "abc" || first.Get("score").Any() != 1.0 {
		t.Fatalf("sender parsed event = %#v", first.Underlying())
	}
	if rendered, err := RenderJSON(first); err != nil || rendered != `{"p1":"abc","score":1.0}` {
		t.Fatalf("sender rendered event = %s (%v)", rendered, err)
	}
	if err := sender.Send(context.Background(), []byte(`{"p1":"def","score":2}`)); err != nil {
		t.Fatal(err)
	}
	if second := <-received; second.Get("p1").Any() != "def" {
		t.Fatalf("sender.Send event = %#v", second.Underlying())
	}
	if err := sender.Route(context.Background(), []byte(`{"p1":"route","score":2.5}`)); err != nil {
		t.Fatal(err)
	}
	if routed := <-received; routed.Get("p1").Any() != "route" {
		t.Fatalf("sender.Route event = %#v", routed.Underlying())
	}
	if err := sender.SendUnderlying(context.Background(), map[string]any{"p1": "map", "score": 3.0}); err != nil {
		t.Fatal(err)
	}
	if sentUnderlying := <-received; sentUnderlying.Get("p1").Any() != "map" {
		t.Fatalf("sender.SendUnderlying event = %#v", sentUnderlying.Underlying())
	}
	if err := sender.RouteUnderlying(context.Background(), map[string]any{"p1": "route-map", "score": 4.0}); err != nil {
		t.Fatal(err)
	}
	if routedUnderlying := <-received; routedUnderlying.Get("p1").Any() != "route-map" {
		t.Fatalf("sender.RouteUnderlying event = %#v", routedUnderlying.Underlying())
	}

	foreignSchema, err := NewJSONSchema("SenderEvent", []FieldSpec{
		FieldDef("p1", reflect.TypeOf("")),
		FieldDef("score", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	foreignEvent, err := ParseJSON(foreignSchema, []byte(`{"p1":"foreign","score":"not-a-number"}`), engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.SendEvent(context.Background(), foreignEvent); err == nil {
		t.Fatal("sender accepted an event from a separately constructed same-name schema")
	}

	otherSchema, err := NewJSONSchema("OtherSenderEvent", []FieldSpec{FieldDef("p1", reflect.TypeOf(""))})
	if err != nil {
		t.Fatal(err)
	}
	other, err := ParseJSON(otherSchema, []byte(`{"p1":"wrong"}`), engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.SendEvent(context.Background(), other); err == nil {
		t.Fatal("sender accepted an event parsed for another schema")
	}
	if _, err := engine.JSONSender("MissingJSONEvent"); err == nil {
		t.Fatal("sender resolved an unknown event type")
	}
}

func TestJSONSchemaGetterSupportsMappedAccessAndRejectsUndeclaredDotPath(t *testing.T) {
	schema, err := NewJSONSchema("GetterJSON", []FieldSpec{
		FieldDef("prop", reflect.TypeOf(map[string]string{})),
		FieldDef("p1", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(`{"prop":{"x":"y"},"p1":"z"}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	mapped, ok := schema.Getter(`prop('x')`)
	if !ok || mapped.Type() != reflect.TypeOf("") {
		t.Fatalf("mapped getter metadata = %#v, ok=%t", mapped.Property(), ok)
	}
	if got := mapped.Get(event.Underlying()); got.Any() != "y" {
		t.Fatalf("mapped getter value = %v", got)
	}
	root, ok := schema.Getter("p1")
	if !ok || root.Get(event.Underlying()).Any() != "z" {
		t.Fatalf("root getter = %#v, ok=%t", root.Get(event.Underlying()), ok)
	}
	if _, ok := schema.Getter("prop.somefield?"); ok {
		t.Fatal("plain map dot path should not advertise a getter")
	}
	if _, ok := schema.Getter("missing"); ok {
		t.Fatal("undeclared property should not advertise a getter")
	}
}

func TestJSONCreateSchemaSpecialNamesAndInvalidDeclarationsMatchEsper(t *testing.T) {
	stringType := reflect.TypeOf("")
	schema, err := NewJSONSchema("SpecialNames", []FieldSpec{
		FieldDef("p q", stringType),
		FieldDef("ABC", stringType),
		FieldDef("abc", stringType),
		FieldDef("AbC", stringType),
	})
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(`{"p q":"v1","ABC":"v2","abc":"v3","AbC":"v4"}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"p q": "v1", "ABC": "v2", "abc": "v3", "AbC": "v4"} {
		if got := event.Get(name).Any(); got != want {
			t.Fatalf("special JSON property %q = %#v, want %q", name, got, want)
		}
	}
	if rendered, err := RenderJSON(event); err != nil || rendered != `{"p q":"v1","ABC":"v2","abc":"v3","AbC":"v4"}` {
		t.Fatalf("special-name JSON = %s (%v)", rendered, err)
	}
	if _, err := NewJSONSchema("InvalidEmptyField", []FieldSpec{FieldDef("", stringType)}); err == nil || !strings.Contains(err.Error(), "empty field") {
		t.Fatalf("empty field declaration error = %v", err)
	}
	unsupported := reflect.TypeOf((*interface{ Compare(any) int })(nil)).Elem()
	if _, err := NewJSONSchema("InvalidComparableField", []FieldSpec{FieldDef("comparable", unsupported)}); err == nil || !strings.Contains(err.Error(), "unsupported JSON type") {
		t.Fatalf("unsupported comparable declaration error = %v", err)
	}
}

func TestJSONFieldAdapterRoundTripAndSchemaValidation(t *testing.T) {
	pointType := reflect.TypeOf(jsonAdapterPoint{})
	dateType := reflect.TypeOf(time.Time{})
	pointAdapter := NewJSONFieldAdapter(func(text string) (jsonAdapterPoint, error) {
		var x, y int
		if _, err := fmt.Sscanf(text, "%d,%d", &x, &y); err != nil {
			return jsonAdapterPoint{}, fmt.Errorf("invalid point %q: %w", text, err)
		}
		return jsonAdapterPoint{X: x, Y: y}, nil
	}, func(point jsonAdapterPoint) (string, error) {
		return fmt.Sprintf("%d,%d", point.X, point.Y), nil
	})
	dateAdapter := NewJSONFieldAdapter(func(text string) (time.Time, error) {
		return time.Parse("02-01-2006", text)
	}, func(value time.Time) (string, error) {
		return value.Format("02-01-2006"), nil
	})
	schema, err := NewJSONSchema("AdapterJSON", []FieldSpec{
		FieldDef("point", pointType),
		FieldDef("myDate", dateType),
		FieldDef("nullable", dateType),
	}, WithJSONFieldAdapter("point", pointAdapter), WithJSONFieldAdapter("myDate", dateAdapter))
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(`{"point":"7,14","myDate":"22-09-2018","nullable":null}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	point, err := As[jsonAdapterPoint](event.Get("point"))
	if err != nil || point != (jsonAdapterPoint{X: 7, Y: 14}) {
		t.Fatalf("adapted point = %#v (%v)", point, err)
	}
	date, err := As[time.Time](event.Get("myDate"))
	if err != nil || date.Format("02-01-2006") != "22-09-2018" {
		t.Fatalf("adapted date = %v (%v)", date, err)
	}
	if !event.Get("nullable").IsNull() {
		t.Fatal("adapter null field must remain Null")
	}
	if rendered, err := RenderJSON(event); err != nil || rendered != `{"point":"7,14","myDate":"22-09-2018","nullable":null}` {
		t.Fatalf("adapted JSON = %s (%v)", rendered, err)
	}

	env := NewEnvironment()
	if _, err := RegisterStruct[jsonAdapterSource](env, "LocalEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterJSON(env, "AdapterTarget", []FieldSpec{
		FieldDef("point", pointType),
		FieldDef("myDate", dateType),
	}, WithJSONFieldAdapter("point", pointAdapter), WithJSONFieldAdapter("myDate", dateAdapter)); err != nil {
		t.Fatal(err)
	}
	routePlan, err := env.Build(Select(
		From[jsonAdapterSource](env, "LocalEvent"),
		Alias("point", Field[jsonAdapterSource, jsonAdapterPoint]("point")),
		Alias("myDate", Field[jsonAdapterSource, time.Time]("myDate")),
	).InsertInto("AdapterTarget", StatementName("adapter-insert")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "AdapterTarget").Query(StatementName("adapter-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	if _, err := engine.Deploy(context.Background(), routePlan); err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer consumerDeployment.Undeploy(context.Background())
	var routed Event
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			routed, _ = batch.New[0].Event()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), jsonAdapterSource{
		Point:  jsonAdapterPoint{X: 7, Y: 14},
		MyDate: time.Date(2018, time.September, 22, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if !routed.Schema().valid() {
		t.Fatal("adapter insert-into did not produce a target event")
	}
	if rendered, err := RenderJSON(routed); err != nil || rendered != `{"point":"7,14","myDate":"22-09-2018"}` {
		t.Fatalf("adapter insert-into JSON = %s (%v), event=%#v", rendered, err, routed.Underlying())
	}
	if _, err := ParseJSON(schema, []byte(`{"point":"bad","myDate":"22-09-2018"}`), time.Unix(0, 0)); err == nil || !strings.Contains(err.Error(), "adapter parse") {
		t.Fatalf("invalid adapter input error = %v", err)
	}
	if _, err := ParseJSON(schema, []byte(`{"point":7,"myDate":"22-09-2018"}`), time.Unix(0, 0)); err == nil || !strings.Contains(err.Error(), "expects a JSON string") {
		t.Fatalf("non-string adapter input error = %v", err)
	}
	if adapter, ok := schema.JSONFieldAdapter("MYDATE"); ok || adapter != nil {
		// JSON field names are case-sensitive by default.
		t.Fatalf("case-sensitive adapter lookup = %#v, ok=%t", adapter, ok)
	}

	objectArray, err := NewObjectArraySchema("AdapterObjectArray", []FieldSpec{FieldDef("x", reflect.TypeOf(int(0)))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewJSONSchema("InvalidNestedJSON", []FieldSpec{FieldDef("inner", reflect.TypeOf(map[string]any{}))}, WithNestedPropertySchema("inner", objectArray)); err == nil || !strings.Contains(err.Error(), "JSON or Map") {
		t.Fatalf("invalid nested JSON schema error = %v", err)
	}
	unsupported := reflect.TypeOf((*interface{ Marker() })(nil)).Elem()
	if _, err := NewJSONSchema("InvalidInterfaceJSON", []FieldSpec{FieldDef("value", unsupported)}); err == nil || !strings.Contains(err.Error(), "unsupported JSON type") {
		t.Fatalf("unsupported JSON type error = %v", err)
	}
	if _, err := NewJSONSchema("InvalidAdapterType", []FieldSpec{FieldDef("myDate", reflect.TypeOf(""))}, WithJSONFieldAdapter("myDate", dateAdapter)); err == nil || !strings.Contains(err.Error(), "produces") {
		t.Fatalf("adapter type mismatch error = %v", err)
	}
	if _, err := NewJSONSchema("InvalidNilAdapter", []FieldSpec{FieldDef("value", reflect.TypeOf(""))}, WithJSONFieldAdapter("value", nil)); err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("nil adapter error = %v", err)
	}
}

func TestJSONFieldAdapterEntersStablePlanIdentity(t *testing.T) {
	makePlan := func(adapterName string) Plan {
		env := NewEnvironment()
		adapter := JSONFieldAdapterFuncs{
			Name: adapterName,
			Type: reflect.TypeOf(jsonAdapterPoint{}),
			ParseFunc: func(value string) (any, error) {
				return jsonAdapterPoint{X: len(value)}, nil
			},
			WriteFunc: func(value any) (string, error) {
				point := value.(jsonAdapterPoint)
				return fmt.Sprintf("%d,%d", point.X, point.Y), nil
			},
		}
		if _, err := RegisterJSON(env, "PlanAdapterJSON", []FieldSpec{
			FieldDef("point", reflect.TypeOf(jsonAdapterPoint{})),
		}, WithJSONFieldAdapter("point", adapter)); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(FromAny(env, "PlanAdapterJSON").Query(StatementName("plan-adapter")))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}

	first := makePlan("point-v1")
	same := makePlan("point-v1")
	different := makePlan("point-v2")
	if first.Hash() != same.Hash() || string(first.Canonical()) != string(same.Canonical()) {
		t.Fatalf("same named adapter changed plan identity: %s != %s", first.Hash(), same.Hash())
	}
	if first.Hash() == different.Hash() || string(first.Canonical()) == string(different.Canonical()) {
		t.Fatalf("different named adapters share plan identity: %s", first.Hash())
	}
	if !strings.Contains(string(first.Canonical()), "schema-adapters(PlanAdapterJSON:point:name:point-v1)") {
		t.Fatalf("adapter identity missing from plan canonical: %s", first.Canonical())
	}
}
