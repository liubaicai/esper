 package esper
 
 import (
 	"context"
 	"fmt"
 	"reflect"
 	"testing"
 	"time"
 )
 
 type fcmSupportBean struct {
 	TheString    string `esper:"theString"`
 	IntPrimitive int    `esper:"intPrimitive"`
 }
 
 type fcmMethodRow struct {
 	Symbol string `esper:"symbol"`
 	Value  int    `esper:"value"`
 }
 
 type fcmMethodReturn struct {
 	Col1 string `esper:"col1"`
 	Col2 string `esper:"col2"`
 }
 
 type fcmDifferentReturnBean struct {
 	MapString string `esper:"mapstring"`
 	MapInt    int    `esper:"mapint"`
 }
 
 func newFCMEnvironment(t *testing.T) *Environment {
 	t.Helper()
 	env := NewEnvironment()
 	if _, err := RegisterStruct[fcmSupportBean](env, "SupportBean"); err != nil {
 		t.Fatal(err)
 	}
 	return env
 }
 
 func newEventsFrom[T any](schema Schema, values []T, now time.Time) ([]Event, error) {
 	events := make([]Event, 0, len(values))
 	for _, value := range values {
 		event, err := newEvent(schema, value, now)
 		if err != nil {
 			return nil, err
 		}
 		events = append(events, event)
 	}
 	return events, nil
 }
 
 func runFromClauseMethodJoinCase[T any](t *testing.T, env *Environment, name string, methodStream Stream[T], stream Stream[fcmSupportBean]) []Row {
 	t.Helper()
 	query := Join(stream, methodStream).Select(
 		SelectLeft("theString", Field[fcmSupportBean, string]("theString")),
 		SelectRight("mapstring", Field[map[string]any, string]("mapstring")),
 		SelectRight("mapint", Field[map[string]any, int]("mapint")),
 	).Query(StatementName(name))
 	plan, err := env.Build(query)
 	if err != nil {
 		t.Fatalf("build %s failed: %v", name, err)
 	}
 	engine := NewEngine(env)
 	deployment, err := engine.Deploy(context.Background(), plan)
 	if err != nil {
 		t.Fatalf("deploy %s failed: %v", name, err)
 	}
 	var rows []Row
 	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
 		for _, result := range batch.New {
 			if row, ok := result.Row(); ok {
 				rows = append(rows, row)
 			}
 		}
 		return nil
 	}); err != nil {
 		t.Fatal(err)
 	}
 	if err := engine.SendEvent(context.Background(), fcmSupportBean{TheString: "E1", IntPrimitive: 10}); err != nil {
 		t.Fatal(err)
 	}
 	return rows
 }
 
 func TestFromClauseMethodArrayNoArgParity(t *testing.T) {
 	env := newFCMEnvironment(t)
 	schema, err := StructSchema[fcmMethodRow]("FCMArrayNoArgRow")
 	if err != nil {
 		t.Fatal(err)
 	}
 	if err := env.RegisterSchema(schema); err != nil {
 		t.Fatal(err)
 	}
 	provider := MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
 		return newEventsFrom(schema, []fcmMethodRow{{Symbol: "A", Value: 1}}, request.Now)
 	})
 	method := FromMethodOn[fcmMethodRow](env, "method", "SupportBean", schema, provider)
 	stream := From[fcmSupportBean](env, "SupportBean").Window(LengthWindow(3))
 	query := Join(stream, method).Select(
 		SelectLeft("theString", Field[fcmSupportBean, string]("theString")),
 		SelectRight("symbol", Field[fcmMethodRow, string]("symbol")),
 	).Query(StatementName("fcm-array-noarg"))
 	plan, err := env.Build(query)
 	if err != nil {
 		t.Fatal(err)
 	}
 	engine := NewEngine(env)
 	deployment, err := engine.Deploy(context.Background(), plan)
 	if err != nil {
 		t.Fatal(err)
 	}
 	var rows []Row
 	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
 		for _, result := range batch.New {
 			if row, ok := result.Row(); ok {
 				rows = append(rows, row)
 			}
 		}
 		return nil
 	}); err != nil {
 		t.Fatal(err)
 	}
 	if err := engine.SendEvent(context.Background(), fcmSupportBean{TheString: "E1", IntPrimitive: 0}); err != nil {
 		t.Fatal(err)
 	}
 	if len(rows) != 1 || rows[0].Get("theString").Any() != "E1" || rows[0].Get("symbol").Any() != "A" {
 		t.Fatalf("array noarg rows = %#v", rows)
 	}
 }
 
 func TestFromClauseMethodObjectNoArgParity(t *testing.T) {
 	env := newFCMEnvironment(t)
 	schema, err := StructSchema[fcmMethodRow]("FCMObjectNoArgRow")
 	if err != nil {
 		t.Fatal(err)
 	}
 	if err := env.RegisterSchema(schema); err != nil {
 		t.Fatal(err)
 	}
 	provider := MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
 		return newEventsFrom(schema, []fcmMethodRow{{Symbol: "B", Value: 2}}, request.Now)
 	})
 	method := FromMethodOn[fcmMethodRow](env, "method", "SupportBean", schema, provider)
 	stream := From[fcmSupportBean](env, "SupportBean").Window(LengthWindow(3))
 	query := Join(stream, method).Select(
 		SelectLeft("theString", Field[fcmSupportBean, string]("theString")),
 		SelectRight("symbol", Field[fcmMethodRow, string]("symbol")),
 	).Query(StatementName("fcm-object-noarg"))
 	plan, err := env.Build(query)
 	if err != nil {
 		t.Fatal(err)
 	}
 	engine := NewEngine(env)
 	deployment, err := engine.Deploy(context.Background(), plan)
 	if err != nil {
 		t.Fatal(err)
 	}
 	var rows []Row
 	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
 		for _, result := range batch.New {
 			if row, ok := result.Row(); ok {
 				rows = append(rows, row)
 			}
 		}
 		return nil
 	}); err != nil {
 		t.Fatal(err)
 	}
 	if err := engine.SendEvent(context.Background(), fcmSupportBean{TheString: "E1", IntPrimitive: 0}); err != nil {
 		t.Fatal(err)
 	}
 	if err := engine.SendEvent(context.Background(), fcmSupportBean{TheString: "E2", IntPrimitive: 0}); err != nil {
 		t.Fatal(err)
 	}
 	if len(rows) != 2 || rows[0].Get("symbol").Any() != "B" || rows[1].Get("symbol").Any() != "B" {
 		t.Fatalf("object noarg rows = %#v", rows)
 	}
 }
 
 func TestFromClauseMethodDifferentReturnTypesMapParity(t *testing.T) {
 	env := newFCMEnvironment(t)
 	mapSchema, err := NewMapSchema("FCMMapReturn", []FieldSpec{
 		FieldDef("mapstring", reflect.TypeOf("")),
 		FieldDef("mapint", reflect.TypeOf(0)),
 	})
 	if err != nil {
 		t.Fatal(err)
 	}
 	if err := env.RegisterSchema(mapSchema); err != nil {
 		t.Fatal(err)
 	}
 	provider := MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
 		theString := request.Trigger.Get("theString").Any().(string)
 		primitive := request.Trigger.Get("intPrimitive").Any().(int)
 		return newEventsFrom(mapSchema, []map[string]any{{"mapstring": "|" + theString + "|", "mapint": primitive + 1}}, request.Now)
 	})
 	method := FromMethodOn[map[string]any](env, "method-map", "SupportBean", mapSchema, provider)
 	stream := From[fcmSupportBean](env, "SupportBean")
 	rows := runFromClauseMethodJoinCase(t, env, "fcm-diff-return-map", method, stream)
 	if len(rows) != 1 || rows[0].Get("mapstring").Any() != "|E1|" || rows[0].Get("mapint").Any() != 11 {
 		t.Fatalf("map rows = %#v", rows)
 	}
 }
 
 func TestFromClauseMethodDifferentReturnTypesPOJOParity(t *testing.T) {
 	env := newFCMEnvironment(t)
 	pojoSchema, err := StructSchema[fcmDifferentReturnBean]("FCMPOJOReturn")
 	if err != nil {
 		t.Fatal(err)
 	}
 	if err := env.RegisterSchema(pojoSchema); err != nil {
 		t.Fatal(err)
 	}
 	provider := MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
 		theString := request.Trigger.Get("theString").Any().(string)
 		primitive := request.Trigger.Get("intPrimitive").Any().(int)
 		return newEventsFrom(pojoSchema, []fcmDifferentReturnBean{{MapString: "|" + theString + "|", MapInt: primitive + 1}}, request.Now)
 	})
 	method := FromMethodOn[fcmDifferentReturnBean](env, "method-pojo", "SupportBean", pojoSchema, provider)
 	stream := From[fcmSupportBean](env, "SupportBean")
 	query := Join(stream, method).Select(
 		SelectLeft("theString", Field[fcmSupportBean, string]("theString")),
 		SelectRight("mapstring", Field[fcmDifferentReturnBean, string]("mapstring")),
 		SelectRight("mapint", Field[fcmDifferentReturnBean, int]("mapint")),
 	).Query(StatementName("fcm-diff-return-pojo"))
 	plan, err := env.Build(query)
 	if err != nil {
 		t.Fatal(err)
 	}
 	engine := NewEngine(env)
 	deployment, err := engine.Deploy(context.Background(), plan)
 	if err != nil {
 		t.Fatal(err)
 	}
 	var rows []Row
 	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
 		for _, result := range batch.New {
 			if row, ok := result.Row(); ok {
 				rows = append(rows, row)
 			}
 		}
 		return nil
 	}); err != nil {
 		t.Fatal(err)
 	}
 	if err := engine.SendEvent(context.Background(), fcmSupportBean{TheString: "E1", IntPrimitive: 10}); err != nil {
 		t.Fatal(err)
 	}
 	if len(rows) != 1 || rows[0].Get("mapstring").Any() != "|E1|" || rows[0].Get("mapint").Any() != 11 {
 		t.Fatalf("pojo rows = %#v", rows)
 	}
 }
 
 func TestFromClauseMethodDifferentReturnTypesObjectArrayParity(t *testing.T) {
 	env := newFCMEnvironment(t)
 	objectArraySchema, err := NewObjectArraySchema("FCMOAReturn", []FieldSpec{
 		FieldDef("mapstring", reflect.TypeOf("")),
 		FieldDef("mapint", reflect.TypeOf(0)),
 	})
 	if err != nil {
 		t.Fatal(err)
 	}
 	if err := env.RegisterSchema(objectArraySchema); err != nil {
 		t.Fatal(err)
 	}
 	provider := MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
 		theString := request.Trigger.Get("theString").Any().(string)
 		primitive := request.Trigger.Get("intPrimitive").Any().(int)
 		return newEventsFrom(objectArraySchema, [][]any{{"|" + theString + "|", primitive + 1}}, request.Now)
 	})
 	method := FromMethodOn[[]any](env, "method-oa", "SupportBean", objectArraySchema, provider)
 	stream := From[fcmSupportBean](env, "SupportBean")
 	query := Join(stream, method).Select(
 		SelectLeft("theString", Field[fcmSupportBean, string]("theString")),
 		SelectRight("mapstring", Field[[]any, string]("mapstring")),
 		SelectRight("mapint", Field[[]any, int]("mapint")),
 	).Query(StatementName("fcm-diff-return-oa"))
 	plan, err := env.Build(query)
 	if err != nil {
 		t.Fatal(err)
 	}
 	engine := NewEngine(env)
 	deployment, err := engine.Deploy(context.Background(), plan)
 	if err != nil {
 		t.Fatal(err)
 	}
 	var rows []Row
 	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
 		for _, result := range batch.New {
 			if row, ok := result.Row(); ok {
 				rows = append(rows, row)
 			}
 		}
 		return nil
 	}); err != nil {
 		t.Fatal(err)
 	}
 	if err := engine.SendEvent(context.Background(), fcmSupportBean{TheString: "E1", IntPrimitive: 10}); err != nil {
 		t.Fatal(err)
 	}
 	if len(rows) != 1 || rows[0].Get("mapstring").Any() != "|E1|" || rows[0].Get("mapint").Any() != 11 {
 		t.Fatalf("object array rows = %#v", rows)
 	}
 }
 
 func TestFromClauseMethodOverloadedParity(t *testing.T) {
 	env := newFCMEnvironment(t)
 	schema, err := StructSchema[fcmMethodReturn]("FCMOverloadedReturn")
 	if err != nil {
 		t.Fatal(err)
 	}
 	if err := env.RegisterSchema(schema); err != nil {
 		t.Fatal(err)
 	}
 	cases := []struct {
 		params         []any
 		expectedFirst  string
 		expectedSecond string
 	}{
 		{nil, "A", "B"},
 		{[]any{10}, "10", "B"},
 		{[]any{10, 20}, "10", "20"},
 		{[]any{"x"}, "x", "B"},
 		{[]any{"x", 50}, "x", "50"},
 	}
 	for _, tc := range cases {
 		t.Run(fmt.Sprintf("params-%v", tc.params), func(t *testing.T) {
 			provider := MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
 				first := "A"
 				second := "B"
 				if len(tc.params) > 0 {
 					first = paramString(tc.params[0])
 				}
 				if len(tc.params) > 1 {
 					second = paramString(tc.params[1])
 				}
 				return newEventsFrom(schema, []fcmMethodReturn{{Col1: first, Col2: second}}, request.Now)
 			})
 			method := FromMethodOn[fcmMethodReturn](env, "method", "SupportBean", schema, provider)
 			stream := From[fcmSupportBean](env, "SupportBean")
 			query := Join(stream, method).Select(
 				SelectRight("col1", Field[fcmMethodReturn, string]("col1")),
 				SelectRight("col2", Field[fcmMethodReturn, string]("col2")),
 			).Query(StatementName("fcm-overloaded"))
 			plan, err := env.Build(query)
 			if err != nil {
 				t.Fatalf("build failed: %v", err)
 			}
 			engine := NewEngine(env)
 			deployment, err := engine.Deploy(context.Background(), plan)
 			if err != nil {
 				t.Fatalf("deploy failed: %v", err)
 			}
 			var rows []Row
 			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
 				for _, result := range batch.New {
 					if row, ok := result.Row(); ok {
 						rows = append(rows, row)
 					}
 				}
 				return nil
 			}); err != nil {
 				t.Fatal(err)
 			}
 			if err := engine.SendEvent(context.Background(), fcmSupportBean{TheString: "X", IntPrimitive: 0}); err != nil {
 				t.Fatal(err)
 			}
 			if len(rows) != 1 || rows[0].Get("col1").Any() != tc.expectedFirst || rows[0].Get("col2").Any() != tc.expectedSecond {
 				t.Fatalf("overloaded rows = %#v", rows)
 			}
 			if err := deployment.Undeploy(context.Background()); err != nil {
 				t.Fatal(err)
 			}
 		})
 	}
 }
 
 func TestFromClauseMethodInvalidParity(t *testing.T) {
 	env := newFCMEnvironment(t)
 	schema, err := StructSchema[fcmMethodRow]("FCMInvalidRow")
 	if err != nil {
 		t.Fatal(err)
 	}
 	provider := MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
 		return newEventsFrom(schema, []fcmMethodRow{{Symbol: "x", Value: 1}}, request.Now)
 	})
 	method := FromMethodOn[fcmMethodRow](env, "method", "MissingTrigger", schema, provider)
 	if _, err := env.Build(method.Query(StatementName("fcm-invalid-trigger"))); err == nil {
 		t.Fatal("expected error for unknown trigger stream")
 	}
 	if _, err := env.Build(FromMethod[fcmMethodRow](env, "missing-provider", schema, nil).Query(StatementName("fcm-invalid-provider"))); err == nil {
 		t.Fatal("expected error for nil provider")
 	}
 }
 
 func paramString(v any) string {
 	switch x := v.(type) {
 	case string:
 		return x
 	case int:
 		return fmt.Sprintf("%d", x)
 	default:
 		return fmt.Sprintf("%v", v)
 	}
 }
 
