package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type expressionDeleteSignal struct {
	ID string
}

type expressionGroupedBean struct {
	TheString string
	Group     int
	Amount    int
}

func expressionSend(t *testing.T, engine *Engine, name string, value int) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), viewParityBean{TheString: name, IntPrimitive: value}); err != nil {
		t.Fatal(err)
	}
}

func expressionNames(results []Result) []string {
	return expressionNamesByField(results, "theString")
}

func expressionNamesByField(results []Result, field string) []string {
	var names []string
	for _, result := range results {
		event, ok := result.Event()
		if !ok {
			continue
		}
		if value := event.Get(field); value.IsPresent() {
			names = append(names, value.Any().(string))
		}
	}
	return names
}

func expressionAssertNames(t *testing.T, statement *Statement, want ...string) {
	t.Helper()
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := expressionNames(snapshot.Results())
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot names = %#v, want %#v", got, want)
	}
}

func expressionAssertBatch(t *testing.T, batch ResultBatch, wantNew, wantOld []string) {
	t.Helper()
	if got := expressionNames(batch.New); !reflect.DeepEqual(got, wantNew) {
		t.Fatalf("new = %#v, want %#v", got, wantNew)
	}
	if got := expressionNames(batch.Old); !reflect.DeepEqual(got, wantOld) {
		t.Fatalf("old = %#v, want %#v", got, wantOld)
	}
}

func expressionAssertBatchField(t *testing.T, batch ResultBatch, field string, wantNew, wantOld []string) {
	t.Helper()
	if got := expressionNamesByField(batch.New, field); !reflect.DeepEqual(got, wantNew) {
		t.Fatalf("new %s = %#v, want %#v", field, got, wantNew)
	}
	if got := expressionNamesByField(batch.Old, field); !reflect.DeepEqual(got, wantOld) {
		t.Fatalf("old %s = %#v, want %#v", field, got, wantOld)
	}
}

func expressionAdvance(t *testing.T, engine *Engine, millis int64) {
	t.Helper()
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(millis).UTC()); err != nil {
		t.Fatal(err)
	}
}

func deployExpressionQuery(t *testing.T, env *Environment, engine *Engine, query Query) (*Statement, *[]ResultBatch) {
	t.Helper()
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
	return deployment.Statements()[0], batches
}

func expressionAssertRowValue(t *testing.T, result Result, field string, want Value) {
	t.Helper()
	row, ok := result.Row()
	if !ok {
		t.Fatalf("result is not a row: %#v", result)
	}
	if got := row.Get(field); !got.Equal(want) {
		t.Fatalf("row.%s = %v, want %v", field, got, want)
	}
}

// Java: java-runtime-feef242da145c59be1bb (ViewExpressionWindowSceneOne).
func TestViewExpressionWindowSceneOneParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	predicate := Less[int64](
		Subtract[int64](WindowNewestTimestamp(), WindowOldestTimestamp()),
		Literal(int64(1000)),
	)
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionWindow(predicate)), "s0")

	expressionAdvance(t, engine, 1000)
	expressionSend(t, engine, "E1", 0)
	expressionAssertNames(t, statement, "E1")
	expressionAssertBatch(t, (*batches)[0], []string{"E1"}, nil)

	expressionAdvance(t, engine, 1500)
	expressionSend(t, engine, "E2", 0)
	expressionAssertNames(t, statement, "E1", "E2")

	expressionAdvance(t, engine, 2000)
	expressionSend(t, engine, "E3", 0)
	expressionAssertNames(t, statement, "E2", "E3")
	expressionAssertBatch(t, (*batches)[2], []string{"E3"}, []string{"E1"})

	expressionAdvance(t, engine, 2499)
	expressionSend(t, engine, "E4", 0)
	expressionAssertNames(t, statement, "E2", "E3", "E4")

	expressionAdvance(t, engine, 2500)
	expressionSend(t, engine, "E5", 0)
	expressionAssertNames(t, statement, "E3", "E4", "E5")
	expressionAssertBatch(t, (*batches)[4], []string{"E5"}, []string{"E2"})
}

// Java: java-runtime-1fd41f23589a5af4132c (ViewExpressionWindowNewestEventOldestEvent).
func TestViewExpressionWindowNewestOldestParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	newest := NestedField[int](WindowNewestEvent(), "intPrimitive")
	oldest := NestedField[int](WindowOldestEvent(), "intPrimitive")
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionWindow(Equal[int](newest, oldest))), "s0")

	for _, event := range []struct {
		name  string
		value int
	}{{"E1", 1}, {"E2", 1}} {
		expressionSend(t, engine, event.name, event.value)
	}
	expressionAssertNames(t, statement, "E1", "E2")

	expressionSend(t, engine, "E3", 2)
	expressionAssertNames(t, statement, "E3")
	expressionAssertBatch(t, (*batches)[2], []string{"E3"}, []string{"E1", "E2"})

	expressionSend(t, engine, "E4", 3)
	expressionAssertNames(t, statement, "E4")
	expressionAssertBatch(t, (*batches)[3], []string{"E4"}, []string{"E3"})
}

// Java: java-runtime-0c450c9d1c1fcf3b5523 (ViewExpressionWindowLengthWindow).
func TestViewExpressionWindowCurrentCountParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	predicate := LessOrEqual[int64](WindowCurrentCount(), Literal(int64(2)))
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionWindow(predicate)), "s0")
	expressionSend(t, engine, "E1", 1)
	expressionSend(t, engine, "E2", 2)
	expressionAssertNames(t, statement, "E1", "E2")
	expressionSend(t, engine, "E3", 3)
	expressionAssertNames(t, statement, "E2", "E3")
	expressionAssertBatch(t, (*batches)[2], []string{"E3"}, []string{"E1"})
}

// Java: java-runtime-5d04258cf2eb6e0025fd (ViewExpressionWindowTimeWindow).
func TestViewExpressionWindowTimestampDifferenceParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	predicate := Greater[int64](
		WindowOldestTimestamp(),
		Subtract[int64](WindowNewestTimestamp(), Literal(int64(2000))),
	)
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionWindow(predicate)), "s0")
	for index, at := range []int64{1000, 1500, 2000, 2500, 3000} {
		expressionAdvance(t, engine, at)
		expressionSend(t, engine, "E"+string(rune('1'+index)), 0)
	}
	if len(*batches) != 5 {
		t.Fatalf("timestamp batches = %d, want 5", len(*batches))
	}
	expressionAdvance(t, engine, 3500)
	expressionSend(t, engine, "E6", 0)
	expressionAssertNames(t, statement, "E3", "E4", "E5", "E6")
	expressionAssertBatch(t, (*batches)[5], []string{"E6"}, []string{"E2"})
}

// Java: java-runtime-2ea3302a92f786153790 (ViewExpressionWindowUDFBuiltin).
func TestViewExpressionWindowUDFBuiltinsParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	allow := true
	var observed struct {
		key     string
		expired int64
		refLen  int
	}
	udf := Func1Ctx[[]Event, bool]("expression-window-udf", func(reference []Event, ctx EvalContext) bool {
		observed.key = ctx.Event.Get("theString").Any().(string)
		observed.expired = ctx.WindowExpiredCount
		observed.refLen = len(reference)
		return allow
	}, WindowReference())
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionWindow(udf)), "s0")

	expressionSend(t, engine, "E1", 0)
	if observed.key != "E1" || observed.expired != 0 || observed.refLen != 1 {
		t.Fatalf("first UDF observation = %#v", observed)
	}
	expressionSend(t, engine, "E2", 0)
	if observed.key != "E2" || observed.expired != 0 || observed.refLen != 2 {
		t.Fatalf("second UDF observation = %#v", observed)
	}
	allow = false
	expressionSend(t, engine, "E3", 0)
	if observed.key != "E3" || observed.expired != 2 || observed.refLen != 1 {
		t.Fatalf("expiry UDF observation = %#v", observed)
	}
	expressionAssertNames(t, statement)
	expressionAssertBatch(t, (*batches)[2], []string{"E3"}, []string{"E1", "E2", "E3"})
}

// Java: java-runtime-984f59c809d70e02b3da (ViewExpressionWindowInvalid).
func TestViewExpressionWindowInvalidParity(t *testing.T) {
	env, _ := newViewParityEnv(t)
	invalid := typedExpr[bool]{
		n:  &exprNode{kind: "invalid-return", typ: typeOf[int](), description: "invalid-return"},
		fn: func(EvalContext) Value { return Present(int64(1)) },
	}
	if _, err := env.Build(From[viewParityBean](env, "SupportBean").Window(ExpressionWindow(invalid)).Query()); err == nil {
		t.Fatal("expression window accepted a non-bool predicate")
	}
	previous := Prev[int](1, Field[viewParityBean, int]("intPrimitive"))
	if _, err := env.Build(From[viewParityBean](env, "SupportBean").Window(ExpressionWindow(Equal[int](previous, Literal(1)))).Query()); err == nil {
		t.Fatal("expression window accepted a previous-value predicate")
	}
}

// Java: java-runtime-9171e672cfde4fa06cb5 (ViewExpressionWindowPrev).
func TestViewExpressionWindowPrevProjectionParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean").Window(ExpressionWindow(Literal(true)))
	query := Select(source, Alias("val0", Prev[string](1, Field[viewParityBean, string]("theString")))).
		Query(StatementName("s0"), WithOldStream())
	_, batches := deployExpressionQuery(t, env, engine, query)

	expressionSend(t, engine, "E1", 1)
	expressionAssertRowValue(t, (*batches)[0].New[0], "val0", Null())
	expressionSend(t, engine, "E2", 2)
	expressionAssertRowValue(t, (*batches)[1].New[0], "val0", Present("E1"))
}

// Java: java-runtime-0201ff1e8d8eaabe883f (ViewExpressionWindowAggregationUngrouped).
func TestViewExpressionWindowAggregationParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	sum := Sum[int](Field[viewParityBean, int]("intPrimitive"))
	predicate := Less[int](sum, Literal(10))
	statement, batches := deployViewTest(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionWindow(predicate)), "s0")

	for _, event := range []struct {
		name  string
		value int
	}{{"E1", 1}, {"E2", 9}, {"E3", 11}, {"E4", 12}, {"E5", 1}} {
		expressionSend(t, engine, event.name, event.value)
	}
	expressionAssertNames(t, statement, "E5")
	expressionAssertBatch(t, (*batches)[1], []string{"E2"}, []string{"E1"})
	expressionAssertBatch(t, (*batches)[2], []string{"E3"}, []string{"E2", "E3"})
	expressionAssertBatch(t, (*batches)[3], []string{"E4"}, []string{"E4"})
}

// Java: java-runtime-bef7f02bfc8cb8b18b86 (ViewExpressionWindowAggregationWGroupwin).
func TestViewExpressionWindowGroupedAggregationParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[expressionGroupedBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	key := Field[expressionGroupedBean, int]("Group")
	amount := Field[expressionGroupedBean, int]("Amount")
	predicate := Less[int](Sum[int](amount), Literal(10))
	stream := From[expressionGroupedBean](env, "SupportBean").Window(GroupWindow(key, ExpressionWindow(predicate)))
	statement, batches := deployViewTest(t, env, engine, stream, "s0")
	send := func(name string, group, value int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), expressionGroupedBean{TheString: name, Group: group, Amount: value}); err != nil {
			t.Fatal(err)
		}
	}

	send("E1", 1, 5)
	send("E2", 2, 2)
	send("E3", 1, 3)
	send("E4", 2, 4)
	send("E5", 2, 6)
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := expressionNamesByField(snapshot.Results(), "TheString"); !reflect.DeepEqual(got, []string{"E1", "E3", "E5"}) {
		t.Fatalf("grouped snapshot names = %#v", got)
	}
	expressionAssertBatchField(t, (*batches)[4], "TheString", []string{"E5"}, []string{"E2", "E4"})
	send("E6", 1, 2)
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := expressionNamesByField(snapshot.Results(), "TheString"); !reflect.DeepEqual(got, []string{"E3", "E6", "E5"}) {
		t.Fatalf("grouped snapshot after expiry = %#v", got)
	}
	expressionAssertBatchField(t, (*batches)[5], "TheString", []string{"E6"}, []string{"E1"})
}

func deployExpressionNamedWindow(t *testing.T, env *Environment, engine *Engine, name string, retention WindowSpec) *NamedWindow {
	t.Helper()
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema is missing")
	}
	if _, err := CreateNamedWindow(env, name, schema, NamedWindowRetention(retention)); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[viewParityBean](env, "SupportBean")).InsertIntoNamedWindow(
		name,
		SetColumn("theString", Field[viewParityBean, string]("theString")),
		SetColumn("intPrimitive", Field[viewParityBean, int]("intPrimitive")),
	).Query(StatementName(name + "-insert")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[expressionDeleteSignal](env, "SupportBean_A")).DeleteFromNamedWindow(
		name,
		Equal[string](NamedWindowField[string]("theString"), Field[expressionDeleteSignal, string]("ID")),
	).Query(StatementName(name + "-delete")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow(name)
	if !ok {
		t.Fatalf("named window %q is missing", name)
	}
	return window
}

// Java: java-runtime-d9281b1cc6d48c4984e1 (ViewExpressionWindowNamedWindowDelete).
func TestViewExpressionWindowNamedWindowDeleteParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	if _, err := RegisterStruct[expressionDeleteSignal](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	window := deployExpressionNamedWindow(t, env, engine, "expression-window-delete", ExpressionWindow(Literal(true)))
	expressionSend(t, engine, "E1", 1)
	expressionSend(t, engine, "E2", 2)
	expressionSend(t, engine, "E3", 3)
	if got, err := window.Snapshot(context.Background()); err != nil || len(got) != 3 {
		t.Fatalf("named window before delete = %#v, err=%v", got, err)
	}
	deltas := make([]NamedWindowDelta, 0, 4)
	if _, err := window.Subscribe(func(_ context.Context, delta NamedWindowDelta) error {
		deltas = append(deltas, delta)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), expressionDeleteSignal{ID: "E2"}); err != nil {
		t.Fatal(err)
	}
	got, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if names := expressionNames(func() []Result {
		results := make([]Result, 0, len(got))
		for _, event := range got {
			results = append(results, resultEvent(event))
		}
		return results
	}()); !reflect.DeepEqual(names, []string{"E1", "E3"}) {
		t.Fatalf("named window after delete = %#v", names)
	}
	if len(deltas) != 1 || expressionNames(func() []Result {
		results := make([]Result, 0, len(deltas[0].Old))
		for _, event := range deltas[0].Old {
			results = append(results, resultEvent(event))
		}
		return results
	}())[0] != "E2" {
		t.Fatalf("named window delete delta = %#v", deltas)
	}
}

// Java: java-runtime-e4c4569b44a3e479c37e (ViewExpressionWindowAggregationWOnDelete).
func TestViewExpressionWindowAggregationOnDeleteParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	if _, err := RegisterStruct[expressionDeleteSignal](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	predicate := Less[int](Sum[int](Field[viewParityBean, int]("intPrimitive")), Literal(10))
	window := deployExpressionNamedWindow(t, env, engine, "expression-window-aggregate-delete", ExpressionWindow(predicate))
	expressionSend(t, engine, "E1", 1)
	expressionSend(t, engine, "E2", 8)
	if err := engine.SendEvent(context.Background(), expressionDeleteSignal{ID: "E2"}); err != nil {
		t.Fatal(err)
	}
	expressionSend(t, engine, "E3", 7)
	expressionSend(t, engine, "E4", 2)
	got, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	results := make([]Result, 0, len(got))
	for _, event := range got {
		results = append(results, resultEvent(event))
	}
	if names := expressionNames(results); !reflect.DeepEqual(names, []string{"E3", "E4"}) {
		t.Fatalf("aggregate named window = %#v", names)
	}
}

// Java: java-runtime-8f167cedfd9b5d56fe67 (ViewExpressionWindowVariable).
func TestViewExpressionWindowVariableParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	if err := env.RegisterVariable("keep-expression", true); err != nil {
		t.Fatal(err)
	}
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionWindow(VariableRef[bool]("keep-expression"))), "s0")
	expressionAdvance(t, engine, 1000)
	expressionSend(t, engine, "E1", 1)
	if err := engine.SetVariable(context.Background(), "keep-expression", false); err != nil {
		t.Fatal(err)
	}
	expressionAdvance(t, engine, 1001)
	expressionAssertNames(t, statement)
	expressionAssertBatch(t, (*batches)[1], nil, []string{"E1"})
	expressionSend(t, engine, "E2", 2)
	expressionAssertBatch(t, (*batches)[2], []string{"E2"}, []string{"E2"})
	if err := engine.SetVariable(context.Background(), "keep-expression", true); err != nil {
		t.Fatal(err)
	}
	expressionSend(t, engine, "E3", 3)
	expressionAssertNames(t, statement, "E3")
}

// Java: java-runtime-5ccd88d79fe264701059 (ViewExpressionWindowDynamicTimeWindow).
func TestViewExpressionWindowDynamicTimestampParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	if err := env.RegisterVariable("expression-size", int64(1000)); err != nil {
		t.Fatal(err)
	}
	predicate := Less[int64](
		Subtract[int64](WindowNewestTimestamp(), WindowOldestTimestamp()),
		VariableRef[int64]("expression-size"),
	)
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionWindow(predicate)), "s0")
	expressionAdvance(t, engine, 1000)
	expressionSend(t, engine, "E1", 0)
	expressionAdvance(t, engine, 2000)
	expressionSend(t, engine, "E2", 0)
	expressionAssertNames(t, statement, "E2")
	if err := engine.SetVariable(context.Background(), "expression-size", int64(10000)); err != nil {
		t.Fatal(err)
	}
	expressionAdvance(t, engine, 5000)
	expressionSend(t, engine, "E3", 0)
	expressionAssertNames(t, statement, "E2", "E3")
	if err := engine.SetVariable(context.Background(), "expression-size", int64(2000)); err != nil {
		t.Fatal(err)
	}
	expressionAdvance(t, engine, 6000)
	expressionSend(t, engine, "E4", 0)
	expressionAssertNames(t, statement, "E3", "E4")
	if len(*batches) < 4 {
		t.Fatalf("dynamic timestamp batches = %#v", *batches)
	}
}

// Java: java-runtime-a4012797b6db56fc35ed (ViewExpressionBatchNewestEventOldestEvent).
func TestViewExpressionBatchNewestOldestParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	newest := NestedField[int](WindowNewestEvent(), "intPrimitive")
	oldest := NestedField[int](WindowOldestEvent(), "intPrimitive")
	trigger := NotEqual[int](newest, oldest)
	excluded, batches := deployExpressionQuery(t, env, engine, From[viewParityBean](env, "SupportBean").
		Window(ExpressionBatch(trigger, ExcludeTriggerEvent())).
		Query(StatementName("s0"), WithOldStream()))
	_ = excluded

	expressionSend(t, engine, "E1", 1)
	expressionSend(t, engine, "E2", 1)
	expressionSend(t, engine, "E3", 2)
	expressionAssertBatch(t, (*batches)[0], []string{"E1", "E2"}, nil)
	expressionSend(t, engine, "E4", 3)
	expressionAssertBatch(t, (*batches)[1], []string{"E3"}, []string{"E1", "E2"})
	expressionSend(t, engine, "E5", 3)
	expressionSend(t, engine, "E6", 3)
	expressionSend(t, engine, "E7", 2)
	expressionAssertBatch(t, (*batches)[2], []string{"E4", "E5", "E6"}, []string{"E3"})

	trigger = NotEqual[int](newest, oldest)
	_, included := deployExpressionQuery(t, env, engine, From[viewParityBean](env, "SupportBean").
		Window(ExpressionBatch(trigger, IncludeTriggerEvent())).
		Query(StatementName("s0"), WithOldStream()))
	expressionSend(t, engine, "E1", 1)
	expressionSend(t, engine, "E2", 1)
	expressionSend(t, engine, "E3", 2)
	expressionAssertBatch(t, (*included)[0], []string{"E1", "E2", "E3"}, nil)
	expressionSend(t, engine, "E4", 3)
	expressionSend(t, engine, "E5", 3)
	expressionSend(t, engine, "E6", 3)
	expressionSend(t, engine, "E7", 2)
	expressionAssertBatch(t, (*included)[1], []string{"E4", "E5", "E6", "E7"}, []string{"E1", "E2", "E3"})
}

// Java: java-runtime-bc9403566ad19ca73ed9 (ViewExpressionBatchLengthBatch).
func TestViewExpressionBatchLengthBatchParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	trigger := GreaterOrEqual[int64](WindowCurrentCount(), Literal(int64(3)))
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionBatch(trigger)), "s0")
	expressionSend(t, engine, "E1", 1)
	expressionSend(t, engine, "E2", 2)
	if len(*batches) != 0 {
		t.Fatalf("premature length batch = %#v", *batches)
	}
	expressionSend(t, engine, "E3", 3)
	expressionAssertBatch(t, (*batches)[0], []string{"E1", "E2", "E3"}, nil)
	expressionSend(t, engine, "E4", 4)
	expressionSend(t, engine, "E5", 5)
	expressionSend(t, engine, "E6", 6)
	expressionAssertBatch(t, (*batches)[1], []string{"E4", "E5", "E6"}, []string{"E1", "E2", "E3"})
	expressionSend(t, engine, "E7", 7)
	expressionSend(t, engine, "E8", 8)
	expressionSend(t, engine, "E9", 9)
	expressionAssertBatch(t, (*batches)[2], []string{"E7", "E8", "E9"}, []string{"E4", "E5", "E6"})
	expressionAssertNames(t, statement)
}

// Java: java-runtime-d68cff5ac536057a77b8 (ViewExpressionBatchTimeBatch).
func TestViewExpressionBatchTimeBatchParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	trigger := Greater[int64](
		Subtract[int64](WindowNewestTimestamp(), WindowOldestTimestamp()),
		Literal(int64(2000)),
	)
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionBatch(trigger)), "s0")
	expressionAdvance(t, engine, 1000)
	expressionSend(t, engine, "E1", 1)
	expressionAdvance(t, engine, 1500)
	expressionSend(t, engine, "E2", 2)
	expressionSend(t, engine, "E3", 3)
	expressionAdvance(t, engine, 3000)
	expressionSend(t, engine, "E4", 4)
	expressionAdvance(t, engine, 3100)
	if len(*batches) != 0 {
		t.Fatalf("premature time batch = %#v", *batches)
	}
	expressionSend(t, engine, "E5", 5)
	expressionAssertBatch(t, (*batches)[0], []string{"E1", "E2", "E3", "E4", "E5"}, nil)
	expressionSend(t, engine, "E6", 6)
	expressionAdvance(t, engine, 5100)
	expressionSend(t, engine, "E7", 7)
	expressionAdvance(t, engine, 5101)
	if len(*batches) != 1 {
		t.Fatalf("premature second time batch = %#v", *batches)
	}
	expressionSend(t, engine, "E8", 8)
	expressionAssertBatch(t, (*batches)[1], []string{"E6", "E7", "E8"}, []string{"E1", "E2", "E3", "E4", "E5"})
	expressionAssertNames(t, statement)
}

// Java: java-runtime-ad02b23eda34efdfd83e (ViewExpressionBatchUDFBuiltin).
func TestViewExpressionBatchUDFBuiltinsParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	var observed struct {
		key     string
		expired int64
		refLen  int
	}
	udf := Func1Ctx[[]Event, bool]("expression-batch-udf", func(reference []Event, ctx EvalContext) bool {
		observed.key = ctx.Event.Get("theString").Any().(string)
		observed.expired = ctx.WindowExpiredCount
		observed.refLen = len(reference)
		return true
	}, WindowReference())
	_, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionBatch(udf)), "s0")
	expressionSend(t, engine, "E1", 0)
	if observed.key != "E1" || observed.expired != 0 || observed.refLen != 1 {
		t.Fatalf("first batch UDF observation = %#v", observed)
	}
	if len(*batches) != 1 {
		t.Fatalf("first trigger flush missing = %#v", *batches)
	}
	expressionSend(t, engine, "E2", 0)
	if observed.key != "E2" || observed.expired != 0 || observed.refLen != 1 {
		t.Fatalf("second batch UDF observation = %#v", observed)
	}
	expressionAssertBatch(t, (*batches)[1], []string{"E2"}, []string{"E1"})
}

// Java: java-runtime-3bec407608d81eebcc12 (ViewExpressionBatchInvalid).
func TestViewExpressionBatchInvalidParity(t *testing.T) {
	env, _ := newViewParityEnv(t)
	invalid := typedExpr[bool]{
		n:  &exprNode{kind: "invalid-return", typ: typeOf[int](), description: "invalid-return"},
		fn: func(EvalContext) Value { return Present(int64(1)) },
	}
	if _, err := env.Build(From[viewParityBean](env, "SupportBean").Window(ExpressionBatch(invalid)).Query()); err == nil {
		t.Fatal("expression batch accepted a non-bool trigger")
	}
	previous := Prev[int](1, Field[viewParityBean, int]("intPrimitive"))
	if _, err := env.Build(From[viewParityBean](env, "SupportBean").Window(ExpressionBatch(Equal[int](previous, Literal(1)))).Query()); err == nil {
		t.Fatal("expression batch accepted a previous-value trigger")
	}
}

// Java: java-runtime-cfb5c3a23c3ab8bf2029 (ViewExpressionBatchPrev).
func TestViewExpressionBatchPrevParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[viewParityBean](env, "SupportBean").Window(ExpressionBatch(Greater[int64](WindowCurrentCount(), Literal(int64(2)))))
	query := Select(source, Alias("val0", Prev[string](1, Field[viewParityBean, string]("theString")))).
		Query(StatementName("s0"), WithOldStream())
	_, batches := deployExpressionQuery(t, env, engine, query)

	expressionSend(t, engine, "E1", 1)
	expressionSend(t, engine, "E2", 2)
	if len(*batches) != 0 {
		t.Fatalf("premature prev batch = %#v", *batches)
	}
	expressionSend(t, engine, "E3", 3)
	if len((*batches)[0].New) != 3 {
		t.Fatalf("prev batch rows = %#v", (*batches)[0])
	}
	expressionAssertRowValue(t, (*batches)[0].New[0], "val0", Null())
	expressionAssertRowValue(t, (*batches)[0].New[1], "val0", Present("E1"))
	expressionAssertRowValue(t, (*batches)[0].New[2], "val0", Present("E2"))
}

// Java: java-runtime-c6280a36141e2f9ac8c7 (ViewExpressionBatchEventPropBatch).
func TestViewExpressionBatchEventPropBatchParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	trigger := Greater[int](Field[viewParityBean, int]("intPrimitive"), Literal(0))
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionBatch(trigger)), "s0")
	expressionSend(t, engine, "E1", 1)
	expressionAssertBatch(t, (*batches)[0], []string{"E1"}, nil)
	expressionSend(t, engine, "E2", 1)
	expressionAssertBatch(t, (*batches)[1], []string{"E2"}, []string{"E1"})
	expressionSend(t, engine, "E3", -1)
	if len(*batches) != 2 {
		t.Fatalf("negative event should not trigger = %#v", *batches)
	}
	expressionSend(t, engine, "E4", 2)
	expressionAssertBatch(t, (*batches)[2], []string{"E3", "E4"}, []string{"E2"})
	expressionAssertNames(t, statement)
}

// Java: java-runtime-6301447e4e4eaa27af22 (ViewExpressionBatchAggregationUngrouped).
func TestViewExpressionBatchAggregationUngroupedParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	sum := Sum[int](Field[viewParityBean, int]("intPrimitive"))
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionBatch(Greater[int](sum, Literal(100)))), "s0")
	expressionSend(t, engine, "E1", 1)
	expressionSend(t, engine, "E2", 90)
	if len(*batches) != 0 {
		t.Fatalf("premature aggregate batch = %#v", *batches)
	}
	expressionSend(t, engine, "E3", 10)
	expressionAssertBatch(t, (*batches)[0], []string{"E1", "E2", "E3"}, nil)
	expressionSend(t, engine, "E4", 101)
	expressionAssertBatch(t, (*batches)[1], []string{"E4"}, []string{"E1", "E2", "E3"})
	expressionSend(t, engine, "E5", 1)
	expressionSend(t, engine, "E6", 99)
	if len(*batches) != 2 {
		t.Fatalf("premature second aggregate batch = %#v", *batches)
	}
	expressionSend(t, engine, "E7", 1)
	expressionAssertBatch(t, (*batches)[2], []string{"E5", "E6", "E7"}, []string{"E4"})
	expressionAssertNames(t, statement)
}

// Java: java-runtime-a8e7fc62db8793496c86 (ViewExpressionBatchAggregationWGroupwin).
func TestViewExpressionBatchGroupedAggregationParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[expressionGroupedBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	key := Field[expressionGroupedBean, int]("Group")
	amount := Field[expressionGroupedBean, int]("Amount")
	trigger := Greater[int](Sum[int](amount), Literal(100))
	stream := From[expressionGroupedBean](env, "SupportBean").Window(GroupWindow(key, ExpressionBatch(trigger)))
	statement, batches := deployViewTest(t, env, engine, stream, "s0")
	send := func(name string, group, value int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), expressionGroupedBean{TheString: name, Group: group, Amount: value}); err != nil {
			t.Fatal(err)
		}
	}

	send("E1", 1, 10)
	send("E2", 2, 10)
	send("E3", 1, 90)
	send("E4", 2, 80)
	send("E5", 2, 10)
	if len(*batches) != 0 {
		t.Fatalf("premature grouped batch = %#v", *batches)
	}
	send("E6", 2, 1)
	expressionAssertBatchField(t, (*batches)[0], "TheString", []string{"E2", "E4", "E5", "E6"}, nil)
	send("E7", 2, 50)
	if len(*batches) != 1 {
		t.Fatalf("grouped batch emitted on non-trigger = %#v", *batches)
	}
	send("E8", 1, 2)
	expressionAssertBatchField(t, (*batches)[1], "TheString", []string{"E1", "E3", "E8"}, nil)
	send("E9", 2, 50)
	send("E10", 1, 101)
	expressionAssertBatchField(t, (*batches)[2], "TheString", []string{"E10"}, []string{"E1", "E3", "E8"})
	send("E11", 2, 1)
	expressionAssertBatchField(t, (*batches)[3], "TheString", []string{"E7", "E9", "E11"}, []string{"E2", "E4", "E5", "E6"})
	send("E12", 1, 102)
	expressionAssertBatchField(t, (*batches)[4], "TheString", []string{"E12"}, []string{"E10"})
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := expressionNamesByField(snapshot.Results(), "TheString"); !reflect.DeepEqual(got, []string(nil)) {
		t.Fatalf("grouped batch snapshot = %#v", got)
	}
}

// Java: java-runtime-36fb2be2ea2ba4270df5 (ViewExpressionBatchNamedWindowDelete).
func TestViewExpressionBatchNamedWindowDeleteParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	if _, err := RegisterStruct[expressionDeleteSignal](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	trigger := Greater[int64](WindowCurrentCount(), Literal(int64(3)))
	window := deployExpressionNamedWindow(t, env, engine, "expression-batch-delete", ExpressionBatch(trigger))
	expressionSend(t, engine, "E1", 1)
	expressionSend(t, engine, "E2", 2)
	expressionSend(t, engine, "E3", 3)
	got, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if names := expressionNames(func() []Result {
		results := make([]Result, 0, len(got))
		for _, event := range got {
			results = append(results, resultEvent(event))
		}
		return results
	}()); !reflect.DeepEqual(names, []string{"E1", "E2", "E3"}) {
		t.Fatalf("named batch before delete = %#v", names)
	}
	if err := engine.SendEvent(context.Background(), expressionDeleteSignal{ID: "E2"}); err != nil {
		t.Fatal(err)
	}
	got, err = window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := expressionNames(func() []Result {
		results := make([]Result, 0, len(got))
		for _, event := range got {
			results = append(results, resultEvent(event))
		}
		return results
	}())
	if !reflect.DeepEqual(names, []string{"E1", "E3"}) {
		t.Fatalf("named batch after delete = %#v", names)
	}
	expressionSend(t, engine, "E4", 4)
	if got, err := window.Snapshot(context.Background()); err != nil || len(got) != 3 {
		t.Fatalf("named batch after E4 = %#v, err=%v", got, err)
	}
	expressionSend(t, engine, "E5", 5)
	if got, err := window.Snapshot(context.Background()); err != nil || len(got) != 0 {
		t.Fatalf("named batch after flush = %#v, err=%v", got, err)
	}
}

// Java: java-runtime-a8e7fc62db8793496c86 (ViewExpressionBatchAggregationOnDelete).
func TestViewExpressionBatchAggregationOnDeleteParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	if _, err := RegisterStruct[expressionDeleteSignal](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	trigger := GreaterOrEqual[int](Sum[int](Field[viewParityBean, int]("intPrimitive")), Literal(10))
	window := deployExpressionNamedWindow(t, env, engine, "expression-batch-aggregate-delete", ExpressionBatch(trigger))
	deltas := make([]NamedWindowDelta, 0, 4)
	if _, err := window.Subscribe(func(_ context.Context, delta NamedWindowDelta) error {
		deltas = append(deltas, delta)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	expressionSend(t, engine, "E1", 1)
	expressionSend(t, engine, "E2", 8)
	if err := engine.SendEvent(context.Background(), expressionDeleteSignal{ID: "E2"}); err != nil {
		t.Fatal(err)
	}
	expressionSend(t, engine, "E3", 8)
	if len(deltas) != 0 {
		t.Fatalf("premature aggregate named batch = %#v", deltas)
	}
	expressionSend(t, engine, "E4", 1)
	if len(deltas) != 1 {
		t.Fatalf("aggregate named batch deltas = %#v", deltas)
	}
	names := expressionNames(func() []Result {
		results := make([]Result, 0, len(deltas[0].New))
		for _, event := range deltas[0].New {
			results = append(results, resultEvent(event))
		}
		return results
	}())
	if !reflect.DeepEqual(names, []string{"E1", "E3", "E4"}) {
		t.Fatalf("aggregate named batch new = %#v", names)
	}
	got, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("aggregate named batch snapshot = %#v", got)
	}
}

// Java: java-runtime-ace804f86e16f62ae8f5 (ViewExpressionBatchDynamicTimeBatch).
func TestViewExpressionBatchDynamicTimeParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	if err := env.RegisterVariable("expression-size", int64(1000)); err != nil {
		t.Fatal(err)
	}
	trigger := Greater[int64](
		Subtract[int64](WindowNewestTimestamp(), WindowOldestTimestamp()),
		VariableRef[int64]("expression-size"),
	)
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionBatch(trigger)), "s0")
	expressionAdvance(t, engine, 1000)
	expressionSend(t, engine, "E1", 0)
	expressionAdvance(t, engine, 1900)
	expressionSend(t, engine, "E2", 0)
	if len(*batches) != 0 {
		t.Fatalf("premature dynamic batch = %#v", *batches)
	}
	if err := engine.SetVariable(context.Background(), "expression-size", int64(500)); err != nil {
		t.Fatal(err)
	}
	expressionAdvance(t, engine, 1901)
	expressionAssertBatch(t, (*batches)[0], []string{"E1", "E2"}, nil)
	expressionSend(t, engine, "E3", 0)
	expressionAdvance(t, engine, 2300)
	expressionSend(t, engine, "E4", 0)
	expressionAdvance(t, engine, 2500)
	if len(*batches) != 1 {
		t.Fatalf("premature dynamic second batch = %#v", *batches)
	}
	expressionSend(t, engine, "E5", 0)
	expressionAssertBatch(t, (*batches)[1], []string{"E3", "E4", "E5"}, []string{"E1", "E2"})
	expressionAdvance(t, engine, 3100)
	expressionSend(t, engine, "E6", 0)
	if len(*batches) != 2 {
		t.Fatalf("premature dynamic third batch = %#v", *batches)
	}
	if err := engine.SetVariable(context.Background(), "expression-size", int64(999)); err != nil {
		t.Fatal(err)
	}
	expressionAdvance(t, engine, 3700)
	expressionSend(t, engine, "E7", 0)
	if len(*batches) != 2 {
		t.Fatalf("premature dynamic fourth batch = %#v", *batches)
	}
	expressionAdvance(t, engine, 4100)
	expressionSend(t, engine, "E8", 0)
	expressionAssertBatch(t, (*batches)[2], []string{"E6", "E7", "E8"}, []string{"E3", "E4", "E5"})
	expressionAssertNames(t, statement)
}

// Java: java-runtime-afb34438de3f25c964cb (ViewExpressionBatchVariableBatch).
func TestViewExpressionBatchVariableParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	if err := env.RegisterVariable("batch-post", false); err != nil {
		t.Fatal(err)
	}
	statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(ExpressionBatch(VariableRef[bool]("batch-post"))), "s0")
	expressionAdvance(t, engine, 1000)
	expressionSend(t, engine, "E1", 1)
	if len(*batches) != 0 {
		t.Fatalf("premature variable batch = %#v", *batches)
	}
	if err := engine.SetVariable(context.Background(), "batch-post", true); err != nil {
		t.Fatal(err)
	}
	expressionAdvance(t, engine, 1001)
	expressionAssertBatch(t, (*batches)[0], []string{"E1"}, nil)
	expressionSend(t, engine, "E2", 1)
	expressionAssertBatch(t, (*batches)[1], []string{"E2"}, []string{"E1"})
	expressionSend(t, engine, "E3", 1)
	expressionAssertBatch(t, (*batches)[2], []string{"E3"}, []string{"E2"})
	if err := engine.SetVariable(context.Background(), "batch-post", false); err != nil {
		t.Fatal(err)
	}
	expressionSend(t, engine, "E4", 1)
	expressionSend(t, engine, "E5", 2)
	expressionAdvance(t, engine, 2000)
	if len(*batches) != 3 {
		t.Fatalf("premature variable second batch = %#v", *batches)
	}
	if err := engine.SetVariable(context.Background(), "batch-post", true); err != nil {
		t.Fatal(err)
	}
	expressionAdvance(t, engine, 2001)
	expressionAssertBatch(t, (*batches)[3], []string{"E4", "E5"}, []string{"E3"})
	expressionSend(t, engine, "E6", 1)
	expressionAssertBatch(t, (*batches)[4], []string{"E6"}, []string{"E4", "E5"})
	expressionAssertNames(t, statement)
}
