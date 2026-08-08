package esper

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

type fcmSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type fcmBeanInt struct {
	ID  string `esper:"id"`
	P00 int    `esper:"p00"`
	P01 int    `esper:"p01"`
	P02 int    `esper:"p02"`
	P03 int    `esper:"p03"`
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

type fcmHistRow struct {
	Val   string `esper:"val"`
	Index int    `esper:"index"`
}

func newFCMEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[fcmSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func newFCMEnvironmentWithInt(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[fcmBeanInt](env, "SupportBeanInt"); err != nil {
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
	query := Join(stream, method).Select(
		SelectLeft("theString", Field[fcmSupportBean, string]("theString")),
		SelectRight("mapstring", Field[map[string]any, string]("mapstring")),
		SelectRight("mapint", Field[map[string]any, int]("mapint")),
	).Query(StatementName("fcm-diff-return-map"))
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

func TestFromClauseMethodOneStreamTwoHistJoinedKeepallParity(t *testing.T) {
	// Mirrors Java EPLFromClauseMethod1Stream2HistStarSubordinateJoinedKeepall:
	// the same assertion runs twice with different from-clause source orderings.
	t.Run("stream-first-order", func(t *testing.T) {
		runFCMOneStreamTwoHistJoinedKeepall(t, []int{0, 1, 2})
	})
	t.Run("hist-first-order", func(t *testing.T) {
		runFCMOneStreamTwoHistJoinedKeepall(t, []int{2, 1, 0})
	})
}

// runFCMOneStreamTwoHistJoinedKeepall deploys the joined keepall query with
// sources ordered by permutation (indices into [stream, h0, h1]) and asserts
// the Java event sequence: E1 matches, E2 produces no new rows, E3 matches.
func runFCMOneStreamTwoHistJoinedKeepall(t *testing.T, permutation []int) {
	t.Helper()
	env := newFCMEnvironmentWithInt(t)
	histSchema, err := NewMapSchema("FCMHistRow", []FieldSpec{
		FieldDef("val", reflect.TypeOf("")),
		FieldDef("index", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(histSchema); err != nil {
		t.Fatal(err)
	}
	makeProvider := func(prefix string, field string) MethodProvider {
		return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
			trigger := request.Trigger
			count := trigger.Get(field).Any().(int)
			if count == 0 {
				return nil, nil
			}
			rows := make([]map[string]any, 0, count)
			for i := 1; i <= count; i++ {
				rows = append(rows, map[string]any{"val": prefix + fmt.Sprintf("%d", i), "index": i})
			}
			return newEventsFrom(histSchema, rows, request.Now)
		})
	}
	h0 := FromMethodOn[map[string]any](env, "h0", "SupportBeanInt", histSchema, makeProvider("H0", "p00"))
	h1 := FromMethodOn[map[string]any](env, "h1", "SupportBeanInt", histSchema, makeProvider("H1", "p01"))
	stream := From[fcmBeanInt](env, "SupportBeanInt").Window(KeepAll())
	sources := make([]JoinInput, 3)
	sources[0] = JoinSource(stream)
	sources[1] = JoinSource(h0)
	sources[2] = JoinSource(h1)
	ordered := make([]JoinInput, 0, 3)
	for _, pos := range permutation {
		ordered = append(ordered, sources[pos])
	}
	indexOf := func(logical int) int {
		for i, pos := range permutation {
			if pos == logical {
				return i
			}
		}
		t.Fatalf("logical source %d missing from permutation %v", logical, permutation)
		return -1
	}
	query := JoinMany(ordered...).On(
		OnSourcesEqual(indexOf(1), Field[map[string]any, int]("index"), indexOf(2), Field[map[string]any, int]("index")),
		OnSourcesEqual(indexOf(1), Field[map[string]any, int]("index"), indexOf(0), Field[fcmBeanInt, int]("p02")),
	).Select(
		SelectFrom(indexOf(0), "id", Field[fcmBeanInt, string]("id")),
		SelectFrom(indexOf(1), "valh0", Field[map[string]any, string]("val")),
		SelectFrom(indexOf(2), "valh1", Field[map[string]any, string]("val")),
	).Query(StatementName("fcm-1s2h-joined-keepall"))
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
	if err := engine.SendEvent(context.Background(), fcmBeanInt{ID: "E1", P00: 20, P01: 20, P02: 3}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("id").Any() != "E1" || rows[0].Get("valh0").Any() != "H03" || rows[0].Get("valh1").Any() != "H13" {
		t.Fatalf("1s2h joined rows = %#v", rows)
	}
	// Java: E2 has p02=21 which matches no h0/h1 index, so no new rows.
	if err := engine.SendEvent(context.Background(), fcmBeanInt{ID: "E2", P00: 20, P01: 20, P02: 21}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("1s2h joined rows after E2 = %#v", rows)
	}
	// Java: E3 matches index 2 and the keepall window still retains E1.
	if err := engine.SendEvent(context.Background(), fcmBeanInt{ID: "E3", P00: 4, P01: 4, P02: 2}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1].Get("id").Any() != "E3" || rows[1].Get("valh0").Any() != "H02" || rows[1].Get("valh1").Any() != "H12" {
		t.Fatalf("1s2h joined rows after E3 = %#v", rows)
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

func fcmHistSchema(t *testing.T, env *Environment) Schema {
	t.Helper()
	histSchema, err := NewMapSchema("FCMHistRow", []FieldSpec{
		FieldDef("val", reflect.TypeOf("")),
		FieldDef("index", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(histSchema); err != nil {
		t.Fatal(err)
	}
	return histSchema
}

// makeFCMHistProvider returns a method provider that emits count rows with
// val = prefix + 1..count and index = 1..count, mirroring Java's SupportJoinMethods.fetchVal.
func makeFCMHistProvider(histSchema Schema, prefix, field string) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		count := request.Trigger.Get(field).Any().(int)
		if count == 0 {
			return nil, nil
		}
		rows := make([]map[string]any, 0, count)
		for i := 1; i <= count; i++ {
			rows = append(rows, map[string]any{"val": prefix + fmt.Sprintf("%d", i), "index": i})
		}
		return newEventsFrom(histSchema, rows, request.Now)
	})
}

// makeFCMDependentHistProvider returns a method provider that depends on depSource.
// Its val prefix is depEvent.val + suffix and it emits count rows, where count is read from field.
func makeFCMDependentHistProvider(histSchema Schema, depSource, suffix, field string) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		count := request.Trigger.Get(field).Any().(int)
		if count == 0 {
			return nil, nil
		}
		depEvent, ok := request.Dependency(depSource)
		if !ok {
			return nil, fmt.Errorf("missing dependency %s", depSource)
		}
		prefix := depEvent.Get("val").Any().(string) + suffix
		rows := make([]map[string]any, 0, count)
		for i := 1; i <= count; i++ {
			rows = append(rows, map[string]any{"val": prefix + fmt.Sprintf("%d", i), "index": i})
		}
		return newEventsFrom(histSchema, rows, request.Now)
	})
}

func lastNewRows(batch ResultBatch) []Row {
	rows := make([]Row, 0, len(batch.New))
	for _, result := range batch.New {
		if row, ok := result.Row(); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func fcmRowKey(row Row) string {
	var parts []string
	for _, field := range []string{"id", "ids0", "ids1", "ids2", "valh0", "valh1", "valh2"} {
		v := row.Get(field)
		if v.Any() == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%v", v.Any()))
	}
	return strings.Join(parts, "|")
}

func fcmAssertRows(t *testing.T, rows []Row, expected [][]string, msg string) {
	t.Helper()
	if len(rows) != len(expected) {
		t.Fatalf("%s: got %d rows, want %d: %#v", msg, len(rows), len(expected), rows)
	}
	actual := make([]string, len(rows))
	for i, row := range rows {
		actual[i] = fcmRowKey(row)
	}
	sort.Strings(actual)
	want := make([]string, len(expected))
	for i, exp := range expected {
		want[i] = strings.Join(exp, "|")
	}
	sort.Strings(want)
	for i := range actual {
		if actual[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", msg, actual, want)
		}
	}
}

// fcmEventID extracts the string value of the struct field tagged esper:"id".
func fcmEventID(event any) string {
	v := reflect.ValueOf(event)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		if tag := t.Field(i).Tag.Get("esper"); tag == "id" {
			return fmt.Sprintf("%v", v.Field(i).Interface())
		}
	}
	return ""
}

// fcmMakeSender returns a helper that sends an event and returns the new rows
// produced by the statement for that event. If no listener batch is produced,
// it returns nil.
func fcmMakeSender(t *testing.T, engine *Engine, stmt *Statement) func(event any) []Row {
	var lastNew []Row
	var sawNew bool
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		lastNew = lastNewRows(batch)
		sawNew = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func(event any) []Row {
		sawNew = false
		lastNew = nil
		eventID := fcmEventID(event)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if !sawNew {
			lastNew = nil
		}
		if eventID != "" {
			filtered := make([]Row, 0, len(lastNew))
			for _, row := range lastNew {
				if row.Get("id").Any() == eventID {
					filtered = append(filtered, row)
				}
			}
			lastNew = filtered
		}
		return lastNew
	}

}

func TestFromClauseMethodOneStreamTwoHistStarSubordinateCartesianLastParity(t *testing.T) {
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	stream := From[fcmBeanInt](env, "SupportBeanInt").Window(LastEvent())
	h0 := FromMethodOn[map[string]any](env, "h0", "SupportBeanInt", histSchema, makeFCMHistProvider(histSchema, "H0", "p00"))
	h1 := FromMethodOn[map[string]any](env, "h1", "SupportBeanInt", histSchema, makeFCMHistProvider(histSchema, "H1", "p01"))

	query := JoinMany(JoinSource(stream), JoinSource(h0), JoinSource(h1)).On(
		OnSourcesCompare(0, Field[fcmBeanInt, int]("p00"), 1, Field[map[string]any, int]("index"), JoinGreaterOrEqual),
		OnSourcesCompare(0, Field[fcmBeanInt, int]("p01"), 2, Field[map[string]any, int]("index"), JoinGreaterOrEqual),
	).Select(
		SelectFrom(0, "id", Field[fcmBeanInt, string]("id")),
		SelectFrom(1, "valh0", Field[map[string]any, string]("val")),
		SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
	).Query(StatementName("fcm-1s2h-cartesian-last"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close(context.Background()) })

	send := fcmMakeSender(t, engine, deployment.Statements()[0])

	rows := send(fcmBeanInt{ID: "E1", P00: 1, P01: 1})
	fcmAssertRows(t, rows, [][]string{{"E1", "H01", "H11"}}, "E1")

	rows = send(fcmBeanInt{ID: "E2", P00: 2, P01: 0})
	if len(rows) != 0 {
		t.Fatalf("E2 expected no rows, got %#v", rows)
	}

	rows = send(fcmBeanInt{ID: "E3", P00: 0, P01: 1})
	if len(rows) != 0 {
		t.Fatalf("E3(0,1) expected no rows, got %#v", rows)
	}

	rows = send(fcmBeanInt{ID: "E3", P00: 2, P01: 2})
	fcmAssertRows(t, rows, [][]string{
		{"E3", "H01", "H11"},
		{"E3", "H01", "H12"},
		{"E3", "H02", "H11"},
		{"E3", "H02", "H12"},
	}, "E3(2,2)")

	rows = send(fcmBeanInt{ID: "E4", P00: 2, P01: 1})
	fcmAssertRows(t, rows, [][]string{
		{"E4", "H01", "H11"},
		{"E4", "H02", "H11"},
	}, "E4(2,1)")
}

func TestFromClauseMethodOneStreamTwoHistForwardSubordinateParity(t *testing.T) {
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	stream := From[fcmBeanInt](env, "SupportBeanInt").Window(KeepAll())
	h0 := FromMethodOn[map[string]any](env, "h0", "SupportBeanInt", histSchema, makeFCMHistProvider(histSchema, "H0", "p00"))
	h1 := FromMethodOn[map[string]any](env, "h1", "SupportBeanInt", histSchema, makeFCMDependentHistProvider(histSchema, "h0", "", "p01")).DependingOn("h0")

	query := JoinMany(JoinSource(stream), JoinSource(h0), JoinSource(h1)).On(
		OnSourcesCompare(0, Field[fcmBeanInt, int]("p00"), 1, Field[map[string]any, int]("index"), JoinGreaterOrEqual),
		OnSourcesCompare(0, Field[fcmBeanInt, int]("p01"), 2, Field[map[string]any, int]("index"), JoinGreaterOrEqual),
	).Select(
		SelectFrom(0, "id", Field[fcmBeanInt, string]("id")),
		SelectFrom(1, "valh0", Field[map[string]any, string]("val")),
		SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
	).Query(StatementName("fcm-1s2h-forward"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close(context.Background()) })

	send := fcmMakeSender(t, engine, deployment.Statements()[0])

	rows := send(fcmBeanInt{ID: "E1", P00: 1, P01: 1})
	fcmAssertRows(t, rows, [][]string{{"E1", "H01", "H011"}}, "E1")

	rows = send(fcmBeanInt{ID: "E2", P00: 0, P01: 1})
	if len(rows) != 0 {
		t.Fatalf("E2 expected no rows, got %#v", rows)
	}

	rows = send(fcmBeanInt{ID: "E3", P00: 1, P01: 0})
	if len(rows) != 0 {
		t.Fatalf("E3 expected no rows, got %#v", rows)
	}

	rows = send(fcmBeanInt{ID: "E4", P00: 2, P01: 2})
	fcmAssertRows(t, rows, [][]string{
		{"E4", "H01", "H011"},
		{"E4", "H01", "H012"},
		{"E4", "H02", "H021"},
		{"E4", "H02", "H022"},
	}, "E4(2,2)")
}

func TestFromClauseMethodOneStreamThreeHistStarSubordinateCartesianLastParity(t *testing.T) {
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	stream := From[fcmBeanInt](env, "SupportBeanInt").Window(LastEvent())
	h0 := FromMethodOn[map[string]any](env, "h0", "SupportBeanInt", histSchema, makeFCMHistProvider(histSchema, "H0", "p00"))
	h1 := FromMethodOn[map[string]any](env, "h1", "SupportBeanInt", histSchema, makeFCMHistProvider(histSchema, "H1", "p01"))
	h2 := FromMethodOn[map[string]any](env, "h2", "SupportBeanInt", histSchema, makeFCMHistProvider(histSchema, "H2", "p02"))

	query := JoinMany(JoinSource(stream), JoinSource(h0), JoinSource(h1), JoinSource(h2)).On(
		OnSourcesCompare(0, Field[fcmBeanInt, int]("p00"), 1, Field[map[string]any, int]("index"), JoinGreaterOrEqual),
		OnSourcesCompare(0, Field[fcmBeanInt, int]("p01"), 2, Field[map[string]any, int]("index"), JoinGreaterOrEqual),
		OnSourcesCompare(0, Field[fcmBeanInt, int]("p02"), 3, Field[map[string]any, int]("index"), JoinGreaterOrEqual),
	).Select(
		SelectFrom(0, "id", Field[fcmBeanInt, string]("id")),
		SelectFrom(1, "valh0", Field[map[string]any, string]("val")),
		SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
		SelectFrom(3, "valh2", Field[map[string]any, string]("val")),
	).Query(StatementName("fcm-1s3h-cartesian-last"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close(context.Background()) })

	send := fcmMakeSender(t, engine, deployment.Statements()[0])

	rows := send(fcmBeanInt{ID: "E1", P00: 1, P01: 1, P02: 1})
	fcmAssertRows(t, rows, [][]string{{"E1", "H01", "H11", "H21"}}, "E1")

	rows = send(fcmBeanInt{ID: "E2", P00: 1, P01: 1, P02: 2})
	fcmAssertRows(t, rows, [][]string{
		{"E2", "H01", "H11", "H21"},
		{"E2", "H01", "H11", "H22"},
	}, "E2(1,1,2)")
}

func TestFromClauseMethodOneStreamThreeHistForwardSubordinateParity(t *testing.T) {
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	stream := From[fcmBeanInt](env, "SupportBeanInt").Window(KeepAll())
	h0 := FromMethodOn[map[string]any](env, "h0", "SupportBeanInt", histSchema, makeFCMHistProvider(histSchema, "H0", "p00"))
	h1 := FromMethodOn[map[string]any](env, "h1", "SupportBeanInt", histSchema, makeFCMHistProvider(histSchema, "H1", "p01"))
	h2 := FromMethodOn[map[string]any](env, "h2", "SupportBeanInt", histSchema, makeFCMDependentHistProvider(histSchema, "h0", "H2", "p02")).DependingOn("h0")

	query := JoinMany(JoinSource(stream), JoinSource(h0), JoinSource(h1), JoinSource(h2)).On(
		OnSourcesEqual(1, Field[map[string]any, int]("index"), 2, Field[map[string]any, int]("index")),
		OnSourcesEqual(2, Field[map[string]any, int]("index"), 3, Field[map[string]any, int]("index")),
		OnSourcesEqual(3, Field[map[string]any, int]("index"), 0, Field[fcmBeanInt, int]("p03")),
	).Select(
		SelectFrom(0, "id", Field[fcmBeanInt, string]("id")),
		SelectFrom(1, "valh0", Field[map[string]any, string]("val")),
		SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
		SelectFrom(3, "valh2", Field[map[string]any, string]("val")),
	).Query(StatementName("fcm-1s3h-forward"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close(context.Background()) })

	send := fcmMakeSender(t, engine, deployment.Statements()[0])

	rows := send(fcmBeanInt{ID: "E1", P00: 2, P01: 2, P02: 2, P03: 1})
	fcmAssertRows(t, rows, [][]string{{"E1", "H01", "H11", "H01H21"}}, "E1")

	rows = send(fcmBeanInt{ID: "E2", P00: 4, P01: 4, P02: 4, P03: 3})
	fcmAssertRows(t, rows, [][]string{{"E2", "H03", "H13", "H03H23"}}, "E2")
}

func TestFromClauseMethodOneStreamThreeHistChainSubordinateParity(t *testing.T) {
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	stream := From[fcmBeanInt](env, "SupportBeanInt").Window(KeepAll())
	h0 := FromMethodOn[map[string]any](env, "h0", "SupportBeanInt", histSchema, makeFCMHistProvider(histSchema, "H0", "p00"))
	h1 := FromMethodOn[map[string]any](env, "h1", "SupportBeanInt", histSchema, makeFCMDependentHistProvider(histSchema, "h0", "H1", "p01")).DependingOn("h0")
	h2 := FromMethodOn[map[string]any](env, "h2", "SupportBeanInt", histSchema, makeFCMDependentHistProvider(histSchema, "h1", "H2", "p02")).DependingOn("h1")

	query := JoinMany(JoinSource(stream), JoinSource(h0), JoinSource(h1), JoinSource(h2)).On(
		OnSourcesEqual(1, Field[map[string]any, int]("index"), 2, Field[map[string]any, int]("index")),
		OnSourcesEqual(2, Field[map[string]any, int]("index"), 3, Field[map[string]any, int]("index")),
		OnSourcesEqual(3, Field[map[string]any, int]("index"), 0, Field[fcmBeanInt, int]("p03")),
	).Select(
		SelectFrom(0, "id", Field[fcmBeanInt, string]("id")),
		SelectFrom(1, "valh0", Field[map[string]any, string]("val")),
		SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
		SelectFrom(3, "valh2", Field[map[string]any, string]("val")),
	).Query(StatementName("fcm-1s3h-chain"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close(context.Background()) })

	send := fcmMakeSender(t, engine, deployment.Statements()[0])

	rows := send(fcmBeanInt{ID: "E2", P00: 4, P01: 4, P02: 4, P03: 3})
	fcmAssertRows(t, rows, [][]string{{"E2", "H03", "H03H13", "H03H13H23"}}, "E2")

	rows = send(fcmBeanInt{ID: "E2", P00: 4, P01: 4, P02: 4, P03: 5})
	if len(rows) != 0 {
		t.Fatalf("E2(5) expected no rows, got %#v", rows)
	}

	rows = send(fcmBeanInt{ID: "E2", P00: 4, P01: 4, P02: 0, P03: 1})
	if len(rows) != 0 {
		t.Fatalf("E2(0) expected no rows, got %#v", rows)
	}
}

// makeFCMAliasedHistProvider returns a method provider that ignores the trigger
// event and instead reads its driving stream event from request.Dependencies.
// The dependency event's id is concatenated with suffix to form the val prefix;
// its p00 field drives the emitted row count. This mirrors Esper's method source
// bound to a specific stream alias.
func makeFCMAliasedHistProvider(histSchema Schema, depSource, suffix string) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		depEvent, ok := request.Dependency(depSource)
		if !ok {
			return nil, nil
		}
		id := depEvent.Get("id").Any().(string)
		count := depEvent.Get("p00").Any().(int)
		if count == 0 {
			return nil, nil
		}
		rows := make([]map[string]any, 0, count)
		for i := 1; i <= count; i++ {
			rows = append(rows, map[string]any{"val": id + suffix + fmt.Sprintf("%d", i), "index": i})
		}
		return newEventsFrom(histSchema, rows, request.Now)
	})
}

// fcmMakeSenderMatchFields returns a helper that sends an event and returns the
// new rows produced for that event. Rows are retained only if one of the named
// fields matches the event's id. This compensates for keep-all joins that emit
// the full window state on each new event.
func fcmMakeSenderMatchFields(t *testing.T, engine *Engine, stmt *Statement, fields []string) func(event any) []Row {
	var lastNew []Row
	var sawNew bool
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		lastNew = lastNewRows(batch)
		sawNew = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func(event any) []Row {
		sawNew = false
		lastNew = nil
		eventID := fcmEventID(event)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if !sawNew {
			lastNew = nil
		}
		if eventID != "" {
			filtered := make([]Row, 0, len(lastNew))
			for _, row := range lastNew {
				for _, field := range fields {
					if row.Get(field).Any() == eventID {
						filtered = append(filtered, row)
						break
					}
				}
			}
			lastNew = filtered
		}
		return lastNew
	}
}

func TestFromClauseMethodTwoStreamTwoHistStarSubordinateParity(t *testing.T) {
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)

	idField := Field[fcmBeanInt, string]("id")
	s0 := FromAs[fcmBeanInt](env, "s0").Filter(StartsWith(idField, Literal("S0"))).Window(KeepAll())
	s1 := FromAs[fcmBeanInt](env, "s1").Filter(StartsWith(idField, Literal("S1"))).Window(LastEvent())

	h0 := FromMethodOn[map[string]any](env, "h0", "SupportBeanInt", histSchema, makeFCMAliasedHistProvider(histSchema, "s0", "H1")).DependingOn("s0")
	h1 := FromMethodOn[map[string]any](env, "h1", "SupportBeanInt", histSchema, makeFCMAliasedHistProvider(histSchema, "s1", "H2")).DependingOn("s1")

	query := JoinMany(JoinSource(s0), JoinSource(s1), JoinSource(h0), JoinSource(h1)).Select(
		SelectFrom(0, "ids0", Field[fcmBeanInt, string]("id")),
		SelectFrom(1, "ids1", Field[fcmBeanInt, string]("id")),
		SelectFrom(2, "valh0", Field[map[string]any, string]("val")),
		SelectFrom(3, "valh1", Field[map[string]any, string]("val")),
	).Query(StatementName("fcm-2s2h-star"))

	plan, err := env.Build(query)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close(context.Background()) })

	send := fcmMakeSenderMatchFields(t, engine, deployment.Statements()[0], []string{"ids0", "ids1"})

	rows := send(fcmBeanInt{ID: "S00", P00: 1})
	if len(rows) != 0 {
		t.Fatalf("S00 expected no rows, got %#v", rows)
	}

	rows = send(fcmBeanInt{ID: "S10", P00: 1})
	fcmAssertRows(t, rows, [][]string{{"S00", "S10", "S00H11", "S10H21"}}, "S10")

	rows = send(fcmBeanInt{ID: "S01", P00: 1})
	fcmAssertRows(t, rows, [][]string{{"S01", "S10", "S01H11", "S10H21"}}, "S01")

	rows = send(fcmBeanInt{ID: "S11", P00: 1})
	fcmAssertRows(t, rows, [][]string{
		{"S00", "S11", "S00H11", "S11H21"},
		{"S01", "S11", "S01H11", "S11H21"},
	}, "S11")
}

func makeFCMThreeStreamHistProvider(histSchema Schema) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		s0, ok0 := request.Dependency("s0")
		s1, ok1 := request.Dependency("s1")
		s2, ok2 := request.Dependency("s2")
		if !ok0 || !ok1 || !ok2 {
			return nil, nil
		}
		prefix := s1.Get("id").Any().(string) + s2.Get("id").Any().(string) + "H1"
		count := s0.Get("p00").Any().(int)
		if count == 0 {
			return nil, nil
		}
		rows := make([]map[string]any, 0, count)
		for i := 1; i <= count; i++ {
			rows = append(rows, map[string]any{"val": prefix + fmt.Sprintf("%d", i), "index": i})
		}
		return newEventsFrom(histSchema, rows, request.Now)
	})
}

func TestFromClauseMethodThreeStreamOneHistSubordinateParity(t *testing.T) {
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)

	idField := Field[fcmBeanInt, string]("id")
	s0 := FromAs[fcmBeanInt](env, "s0").Filter(StartsWith(idField, Literal("S0"))).Window(KeepAll())
	s1 := FromAs[fcmBeanInt](env, "s1").Filter(StartsWith(idField, Literal("S1"))).Window(LastEvent())
	s2 := FromAs[fcmBeanInt](env, "s2").Filter(StartsWith(idField, Literal("S2"))).Window(LastEvent())

	h0 := FromMethodOn[map[string]any](env, "h0", "SupportBeanInt", histSchema, makeFCMThreeStreamHistProvider(histSchema)).DependingOn("s0", "s1", "s2")

	query := JoinMany(JoinSource(s0), JoinSource(s1), JoinSource(s2), JoinSource(h0)).Select(
		SelectFrom(0, "ids0", Field[fcmBeanInt, string]("id")),
		SelectFrom(1, "ids1", Field[fcmBeanInt, string]("id")),
		SelectFrom(2, "ids2", Field[fcmBeanInt, string]("id")),
		SelectFrom(3, "valh0", Field[map[string]any, string]("val")),
	).Query(StatementName("fcm-3s1h"))

	plan, err := env.Build(query)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close(context.Background()) })

	send := fcmMakeSenderMatchFields(t, engine, deployment.Statements()[0], []string{"ids0", "ids1", "ids2"})

	_ = send(fcmBeanInt{ID: "S00", P00: 2})
	_ = send(fcmBeanInt{ID: "S10", P00: 1})

	rows := send(fcmBeanInt{ID: "S20", P00: 1})
	fcmAssertRows(t, rows, [][]string{
		{"S00", "S10", "S20", "S10S20H11"},
		{"S00", "S10", "S20", "S10S20H12"},
	}, "S20")

	rows = send(fcmBeanInt{ID: "S01", P00: 1})
	fcmAssertRows(t, rows, [][]string{{"S01", "S10", "S20", "S10S20H11"}}, "S01")

	rows = send(fcmBeanInt{ID: "S21", P00: 1})
	fcmAssertRows(t, rows, [][]string{
		{"S00", "S10", "S21", "S10S21H11"},
		{"S00", "S10", "S21", "S10S21H12"},
		{"S01", "S10", "S21", "S10S21H11"},
	}, "S21")
}

// makeFCMVariableHistProvider returns a method provider whose row count is read
// from a registered variable, mirroring Java's SupportJoinMethods.fetchVal over
// variable arguments set by an on-trigger statement.
func makeFCMVariableHistProvider(histSchema Schema, prefix, varName string) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		value, ok := request.Variables[varName]
		if !ok || value.Any() == nil {
			return nil, nil
		}
		count := value.Any().(int)
		if count == 0 {
			return nil, nil
		}
		rows := make([]map[string]any, 0, count)
		for i := 1; i <= count; i++ {
			rows = append(rows, map[string]any{"val": prefix + fmt.Sprintf("%d", i), "index": i})
		}
		return newEventsFrom(histSchema, rows, request.Now)
	})
}

// makeFCMVariableDependentHistProvider is the variable-count counterpart of
// makeFCMDependentHistProvider: the dependency event supplies the val prefix.
func makeFCMVariableDependentHistProvider(histSchema Schema, depSource, suffix, varName string) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		value, ok := request.Variables[varName]
		if !ok || value.Any() == nil {
			return nil, nil
		}
		count := value.Any().(int)
		if count == 0 {
			return nil, nil
		}
		depEvent, ok := request.Dependency(depSource)
		if !ok {
			return nil, fmt.Errorf("missing dependency %s", depSource)
		}
		prefix := depEvent.Get("val").Any().(string) + suffix
		rows := make([]map[string]any, 0, count)
		for i := 1; i <= count; i++ {
			rows = append(rows, map[string]any{"val": prefix + fmt.Sprintf("%d", i), "index": i})
		}
		return newEventsFrom(histSchema, rows, request.Now)
	})
}

// fcmDeployVariableSetter registers var1..var4 and deploys an on-trigger that
// mirrors Java's "on SupportBeanInt set var1=p00, var2=p01, var3=p02, var4=p03".
func fcmDeployVariableSetter(t *testing.T, env *Environment, engine *Engine) {
	t.Helper()
	for _, name := range []string{"var1", "var2", "var3", "var4"} {
		if err := env.RegisterVariable(name, 0); err != nil {
			t.Fatal(err)
		}
	}
	setterPlan, err := env.Build(OnEvent(From[fcmBeanInt](env, "SupportBeanInt")).SetVariables(
		SetVariableExpr("var1", Field[fcmBeanInt, int]("p00")),
		SetVariableExpr("var2", Field[fcmBeanInt, int]("p01")),
		SetVariableExpr("var3", Field[fcmBeanInt, int]("p02")),
		SetVariableExpr("var4", Field[fcmBeanInt, int]("p03")),
	).Query(StatementName("fcm-set-vars")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), setterPlan); err != nil {
		t.Fatal(err)
	}
}

// fcmMakeMethodOnlySender sends one event and returns the current snapshot rows,
// mirroring Java's assertPropsPerRowIteratorAnyOrder for method-only joins.
func fcmMakeMethodOnlySender(t *testing.T, engine *Engine, stmt *Statement) func(event any) []Row {
	t.Helper()
	return func(event any) []Row {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		snapshot, err := stmt.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		rows := make([]Row, 0, len(snapshot.Results()))
		for _, result := range snapshot.Results() {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return rows
	}
}

func TestFromClauseMethodThreeHistPureNoSubordinateParity(t *testing.T) {
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	engine := NewEngine(env)
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	fcmDeployVariableSetter(t, env, engine)

	h0 := FromMethod[map[string]any](env, "h0", histSchema, makeFCMVariableHistProvider(histSchema, "H0", "var1"))
	h1 := FromMethod[map[string]any](env, "h1", histSchema, makeFCMVariableHistProvider(histSchema, "H1", "var2"))
	h2 := FromMethod[map[string]any](env, "h2", histSchema, makeFCMVariableHistProvider(histSchema, "H2", "var3"))

	query := JoinMany(JoinSource(h0), JoinSource(h1), JoinSource(h2)).Select(
		SelectFrom(0, "valh0", Field[map[string]any, string]("val")),
		SelectFrom(1, "valh1", Field[map[string]any, string]("val")),
		SelectFrom(2, "valh2", Field[map[string]any, string]("val")),
	).Query(StatementName("fcm-3h-pure"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	send := fcmMakeMethodOnlySender(t, engine, deployment.Statements()[0])

	rows := send(fcmBeanInt{ID: "S00", P00: 1, P01: 1, P02: 1})
	fcmAssertRows(t, rows, [][]string{{"H01", "H11", "H21"}}, "S00")

	rows = send(fcmBeanInt{ID: "S01", P00: 0, P01: 1, P02: 1})
	fcmAssertRows(t, rows, nil, "S01")

	rows = send(fcmBeanInt{ID: "S02", P00: 1, P01: 1, P02: 0})
	fcmAssertRows(t, rows, nil, "S02")

	rows = send(fcmBeanInt{ID: "S03", P00: 1, P01: 1, P02: 2})
	fcmAssertRows(t, rows, [][]string{{"H01", "H11", "H21"}, {"H01", "H11", "H22"}}, "S03")

	rows = send(fcmBeanInt{ID: "S04", P00: 2, P01: 2, P02: 1})
	fcmAssertRows(t, rows, [][]string{
		{"H01", "H11", "H21"},
		{"H02", "H11", "H21"},
		{"H01", "H12", "H21"},
		{"H02", "H12", "H21"},
	}, "S04")
}

func TestFromClauseMethodThreeHistOneSubordinateParity(t *testing.T) {
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	engine := NewEngine(env)
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	fcmDeployVariableSetter(t, env, engine)

	h0 := FromMethod[map[string]any](env, "h0", histSchema, makeFCMVariableHistProvider(histSchema, "H0", "var1"))
	h1 := FromMethod[map[string]any](env, "h1", histSchema, makeFCMVariableHistProvider(histSchema, "H1", "var2"))
	h2 := FromMethod[map[string]any](env, "h2", histSchema, makeFCMVariableDependentHistProvider(histSchema, "h0", "-H2", "var3")).DependingOn("h0")

	query := JoinMany(JoinSource(h0), JoinSource(h1), JoinSource(h2)).Select(
		SelectFrom(0, "valh0", Field[map[string]any, string]("val")),
		SelectFrom(1, "valh1", Field[map[string]any, string]("val")),
		SelectFrom(2, "valh2", Field[map[string]any, string]("val")),
	).Query(StatementName("fcm-3h-1sub"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	send := fcmMakeMethodOnlySender(t, engine, deployment.Statements()[0])

	rows := send(fcmBeanInt{ID: "S00", P00: 1, P01: 1, P02: 1})
	fcmAssertRows(t, rows, [][]string{{"H01", "H11", "H01-H21"}}, "S00")

	rows = send(fcmBeanInt{ID: "S01", P00: 0, P01: 1, P02: 1})
	fcmAssertRows(t, rows, nil, "S01")

	rows = send(fcmBeanInt{ID: "S02", P00: 1, P01: 1, P02: 0})
	fcmAssertRows(t, rows, nil, "S02")

	rows = send(fcmBeanInt{ID: "S03", P00: 1, P01: 1, P02: 2})
	fcmAssertRows(t, rows, [][]string{{"H01", "H11", "H01-H21"}, {"H01", "H11", "H01-H22"}}, "S03")

	rows = send(fcmBeanInt{ID: "S04", P00: 2, P01: 2, P02: 1})
	fcmAssertRows(t, rows, [][]string{
		{"H01", "H11", "H01-H21"},
		{"H02", "H11", "H02-H21"},
		{"H01", "H12", "H01-H21"},
		{"H02", "H12", "H02-H21"},
	}, "S04")
}

func TestFromClauseMethodThreeHistTwoSubordinateChainParity(t *testing.T) {
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	engine := NewEngine(env)
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	fcmDeployVariableSetter(t, env, engine)

	h0 := FromMethod[map[string]any](env, "h0", histSchema, makeFCMVariableHistProvider(histSchema, "H0", "var1"))
	h1 := FromMethod[map[string]any](env, "h1", histSchema, makeFCMVariableDependentHistProvider(histSchema, "h0", "-H1", "var2")).DependingOn("h0")
	h2 := FromMethod[map[string]any](env, "h2", histSchema, makeFCMVariableDependentHistProvider(histSchema, "h1", "-H2", "var3")).DependingOn("h1")

	query := JoinMany(JoinSource(h0), JoinSource(h1), JoinSource(h2)).Select(
		SelectFrom(0, "valh0", Field[map[string]any, string]("val")),
		SelectFrom(1, "valh1", Field[map[string]any, string]("val")),
		SelectFrom(2, "valh2", Field[map[string]any, string]("val")),
	).Query(StatementName("fcm-3h-chain"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	send := fcmMakeMethodOnlySender(t, engine, deployment.Statements()[0])

	rows := send(fcmBeanInt{ID: "S00", P00: 1, P01: 1, P02: 1})
	fcmAssertRows(t, rows, [][]string{{"H01", "H01-H11", "H01-H11-H21"}}, "S00")

	rows = send(fcmBeanInt{ID: "S01", P00: 0, P01: 1, P02: 1})
	fcmAssertRows(t, rows, nil, "S01")

	rows = send(fcmBeanInt{ID: "S02", P00: 1, P01: 1, P02: 0})
	fcmAssertRows(t, rows, nil, "S02")

	rows = send(fcmBeanInt{ID: "S03", P00: 1, P01: 1, P02: 2})
	fcmAssertRows(t, rows, [][]string{{"H01", "H01-H11", "H01-H11-H21"}, {"H01", "H01-H11", "H01-H11-H22"}}, "S03")

	rows = send(fcmBeanInt{ID: "S04", P00: 2, P01: 2, P02: 1})
	fcmAssertRows(t, rows, [][]string{
		{"H01", "H01-H11", "H01-H11-H21"},
		{"H02", "H02-H11", "H02-H11-H21"},
		{"H01", "H01-H12", "H01-H12-H21"},
		{"H02", "H02-H12", "H02-H12-H21"},
	}, "S04")
}

type fcmTradeEventWithSide struct {
	TradeID string `esper:"tradeId"`
	Side    string `esper:"side"`
}

// makeFCMCorrelationProvider mirrors Java's
// EPLFromClauseMethodNStream.computeCorrelation(us, them): one row whose
// correlation is 1 when both dependency events are present and 0 otherwise.
func makeFCMCorrelationProvider(corrSchema Schema) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		_, hasUS := request.Dependency("us")
		_, hasThem := request.Dependency("them")
		correlation := 0
		if hasUS && hasThem {
			correlation = 1
		}
		return newEventsFrom(corrSchema, []map[string]any{{"correlation": correlation}}, request.Now)
	})
}

// TestFromClauseMethodThreeStreamOneHistStreamNWTwiceParity mirrors Java's
// EPLFromClauseMethod3Stream1HistStreamNWTwice: the same keepall named window
// appears twice in one join under distinct aliases (us/them) and a method
// source joins over both aliased sides. Java fills the window through an
// insert-into statement; the Go port inserts through InsertNamedWindow, which
// is the fluent equivalent of "insert into AllTrades select *".
func TestFromClauseMethodThreeStreamOneHistStreamNWTwiceParity(t *testing.T) {
	env := NewEnvironment()
	tradeSchema, err := RegisterStruct[fcmTradeEventWithSide](env, "SupportTradeEventWithSide")
	if err != nil {
		t.Fatal(err)
	}
	corrSchema, err := NewMapSchema("FCMCorrelationRow", []FieldSpec{
		FieldDef("correlation", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(corrSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "AllTrades", tradeSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	us := FromNamedWindowAs[fcmTradeEventWithSide](env, "AllTrades").As("us")
	them := FromNamedWindowAs[fcmTradeEventWithSide](env, "AllTrades").As("them")
	corr := FromMethod[map[string]any](env, "corr", corrSchema, makeFCMCorrelationProvider(corrSchema)).DependingOn("us", "them")

	query := JoinMany(JoinSource(us), JoinSource(them), JoinSource(corr)).On(
		OnSourcesCompare(0, Field[fcmTradeEventWithSide, string]("side"), 1, Field[fcmTradeEventWithSide, string]("side"), JoinNotEqual),
	).Select(
		SelectSourceEvent(0, "us"),
		SelectSourceEvent(1, "them"),
		SelectFrom(2, "crl", Field[map[string]any, int]("correlation")),
	).Where(
		Greater[int](JoinField[int](2, "correlation"), Literal(0)),
	).Query(StatementName("fcm-nw-twice"))

	plan, err := env.Build(query)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close(context.Background()) })

	var lastNew []Row
	sawNew := false
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		lastNew = lastNewRows(batch)
		sawNew = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	insert := func(trade fcmTradeEventWithSide) []Row {
		t.Helper()
		sawNew = false
		lastNew = nil
		if err := engine.InsertNamedWindow(context.Background(), "AllTrades", trade); err != nil {
			t.Fatal(err)
		}
		if !sawNew {
			return nil
		}
		return lastNew
	}

	if rows := insert(fcmTradeEventWithSide{TradeID: "T1", Side: "B"}); len(rows) != 0 {
		t.Fatalf("T1/B expected no rows, got %#v", rows)
	}

	rows := insert(fcmTradeEventWithSide{TradeID: "T2", Side: "S"})
	if len(rows) != 2 {
		t.Fatalf("T2/S expected 2 rows, got %d: %#v", len(rows), rows)
	}
	actual := make([]string, 0, len(rows))
	for _, row := range rows {
		usEvent, ok := row.Get("us").Any().(Event)
		if !ok {
			t.Fatalf("us projection is not an Event: %#v", row.Get("us").Any())
		}
		themEvent, ok := row.Get("them").Any().(Event)
		if !ok {
			t.Fatalf("them projection is not an Event: %#v", row.Get("them").Any())
		}
		actual = append(actual, fmt.Sprintf("%v|%v|%v",
			usEvent.Get("tradeId").Any(), themEvent.Get("tradeId").Any(), row.Get("crl").Any()))
	}
	sort.Strings(actual)
	want := []string{"T1|T2|1", "T2|T1|1"}
	for index := range want {
		if actual[index] != want[index] {
			t.Fatalf("got %v, want %v", actual, want)
		}
	}
}
