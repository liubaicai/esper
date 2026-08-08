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
	for _, field := range []string{"id", "valh0", "valh1", "valh2"} {
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
