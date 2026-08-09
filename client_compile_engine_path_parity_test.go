package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type clientCompileEnginePathBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type clientCompileEnginePathSignal struct {
	ID string `esper:"id"`
}

func newClientCompileEnginePathEnvironment(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompileEnginePathBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[clientCompileEnginePathSignal](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[clientCompileEnginePathSignal](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func activateClientCompileEnginePathModule(t *testing.T, env *Environment, engine *Engine, module Module) *Deployment {
	t.Helper()
	plan, err := module.Build(From[clientCompileEnginePathBean](env, "SupportBean").Query(
		StatementName("activate-" + module.Name()),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	return deployment
}

func TestClientCompileEnginePathObjectTypesMatchEsper(t *testing.T) {
	env, engine := newClientCompileEnginePathEnvironment(t)
	if err := env.RegisterVariable("preconfigured_variable", 5); err != nil {
		t.Fatal(err)
	}
	module, err := env.RegisterModule("runtime-objects", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if err := module.RegisterVariable("myvariable", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterMap("MySchema", []FieldSpec{FieldDef("id", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	if err := module.DefineExpression("myExpr", Literal("abc")); err != nil {
		t.Fatal(err)
	}
	if err := RegisterModuleScript[int](module, "myScript", "go", func(ScriptContext) (int, error) {
		return 2, nil
	}, ScriptArgumentTypes()); err != nil {
		t.Fatal(err)
	}
	if err := module.DefineExpression("MyClass.doIt", Literal("def")); err != nil {
		t.Fatal(err)
	}
	supportSchema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema is missing")
	}
	if _, err := module.RegisterNamedWindow("MyWindow", supportSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterTable("MyTable", []TableColumn{TableColumnOf[string]("y")}); err != nil {
		t.Fatal(err)
	}
	contextDefinition, err := NewKeyContext("MyContext", Field[clientCompileEnginePathBean, string]("theString"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterContextDefinition("MyContext", contextDefinition); err != nil {
		t.Fatal(err)
	}

	before := engine.RuntimePath()
	if len(before.ModuleNames()) != 0 {
		t.Fatalf("pre-deployment runtime modules = %v", before.ModuleNames())
	}
	if _, err := before.Variable("myvariable"); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("inactive runtime variable error = %v", err)
	}
	activation := activateClientCompileEnginePathModule(t, env, engine, module)
	runtimePath := engine.RuntimePath()
	if !reflect.DeepEqual(runtimePath.ModuleNames(), []string{"runtime-objects"}) || !reflect.DeepEqual(runtimePath.DeploymentIDs(), []string{activation.ID()}) {
		t.Fatalf("runtime path modules=%v deployments=%v", runtimePath.ModuleNames(), runtimePath.DeploymentIDs())
	}
	if _, err := runtimePath.EventType("MySchema"); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimePath.NamedWindow("MyWindow"); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimePath.Table("MyTable"); err != nil {
		t.Fatal(err)
	}
	contextName, err := runtimePath.Context("MyContext")
	if err != nil {
		t.Fatal(err)
	}

	myVariable, err := RuntimePathVariableRef[int](runtimePath, "myvariable")
	if err != nil {
		t.Fatal(err)
	}
	preconfigured, err := RuntimePathVariableRef[int](runtimePath, "preconfigured_variable")
	if err != nil {
		t.Fatal(err)
	}
	declared, err := RuntimePathExpressionRef[string](runtimePath, "myExpr")
	if err != nil {
		t.Fatal(err)
	}
	script, err := RuntimePathScriptCall[int](runtimePath, "myScript")
	if err != nil {
		t.Fatal(err)
	}
	inlineEquivalent, err := RuntimePathExpressionRef[string](runtimePath, "MyClass.doIt")
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := runtimePath.Build(Select(
		From[clientCompileEnginePathBean](env, "SupportBean"),
		Alias("c0", myVariable),
		Alias("c1", declared),
		Alias("c2", script),
		Alias("c3", preconfigured),
		Alias("c4", inlineEquivalent),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(consumerPlan.Query().ModuleUses(), []string{"runtime-objects"}) || !strings.Contains(string(consumerPlan.Canonical()), "uses(runtime-objects)") {
		t.Fatalf("runtime-path Plan identity = %v canonical=%s", consumerPlan.Query().ModuleUses(), consumerPlan.Canonical())
	}
	contextPlan, err := runtimePath.Build(From[clientCompileEnginePathBean](env, "SupportBean").Query(
		StatementName("context-consumer"), WithContext(contextName),
	))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), contextPlan); err != nil {
		t.Fatal(err)
	}
	consumer, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := consumer.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientCompileEnginePathBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("runtime-path rows = %d", len(rows))
	}
	for name, expected := range map[string]any{"c0": 10, "c1": "abc", "c2": 2, "c3": 5, "c4": "def"} {
		if actual := rows[0].Get(name).Any(); actual != expected {
			t.Fatalf("runtime-path %s = %#v, want %#v", name, actual, expected)
		}
	}

	if err := activation.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if modules := engine.RuntimePath().ModuleNames(); len(modules) != 0 {
		t.Fatalf("post-undeploy runtime modules = %v", modules)
	}
	if _, err := runtimePath.Variable("myvariable"); err != nil {
		t.Fatalf("immutable runtime-path snapshot lost module: %v", err)
	}
}

func TestClientCompileEnginePathInfraWithIndexMatchesEsper(t *testing.T) {
	env, engine := newClientCompileEnginePathEnvironment(t)
	module, err := env.RegisterModule("runtime-indexes", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterTable("MyTable", []TableColumn{
		PrimaryKeyColumn[string]("id"), PrimaryKeyColumn[int]("theGroup"),
	}, UniqueIndex("I1", "id")); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema is missing")
	}
	if _, err := module.RegisterNamedWindow("MyWindow", schema,
		NamedWindowRetention(KeepAll()), NamedWindowUniqueIndex("I1", "theString")); err != nil {
		t.Fatal(err)
	}
	activateClientCompileEnginePathModule(t, env, engine, module)
	runtimePath := engine.RuntimePath()
	table, err := runtimePath.Table("MyTable")
	if err != nil {
		t.Fatal(err)
	}
	tablePlan, err := runtimePath.Build(table.Filter(
		Equal[string](Field[any, string]("id"), Literal("A")),
	).Select(Alias("id", Field[any, string]("id"))).Query(StatementName("table-index"), UseIndex("I1")))
	if err != nil {
		t.Fatal(err)
	}
	window, err := runtimePath.NamedWindow("MyWindow")
	if err != nil {
		t.Fatal(err)
	}
	windowPlan, err := runtimePath.Build(window.Filter(
		Equal[string](Field[any, string]("theString"), Literal("A")),
	).Select(Alias("theString", Field[any, string]("theString"))).Query(StatementName("window-index"), UseIndex("I1")))
	if err != nil {
		t.Fatal(err)
	}
	for name, plan := range map[string]Plan{"table": tablePlan, "window": windowPlan} {
		selection, ok := plan.IndexPlan().ForSource(0)
		if !ok || selection.IndexName != "I1" || selection.Access != IndexAccessEquality || selection.Backing != IndexBackingUniqueHash {
			t.Fatalf("%s runtime-path index = %#v", name, selection)
		}
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatalf("deploy %s runtime-path index: %v", name, err)
		}
	}
}

func TestClientCompileEnginePathPreconfiguredEventTypeFromPathMatchesEsper(t *testing.T) {
	env, engine := newClientCompileEnginePathEnvironment(t)
	module, err := env.RegisterModule("runtime-table-aggregate", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterTable("MyTableAggs", []TableColumn{
		PrimaryKeyColumn[string]("theString"),
		TableColumnOf[int64]("thecnt"),
		TableColumnOf[[]Event]("thewin"),
	}); err != nil {
		t.Fatal(err)
	}
	activateClientCompileEnginePathModule(t, env, engine, module)
	runtimePath := engine.RuntimePath()
	if _, err := runtimePath.Table("MyTableAggs"); err != nil {
		t.Fatal(err)
	}
	theString := Field[clientCompileEnginePathBean, string]("theString")
	aggregate := From[clientCompileEnginePathBean](env, "SupportBean").Window(KeepAll()).GroupBy(theString).Select(
		Alias("theString", theString),
		Alias("thecnt", CountAll()),
		Alias("thewin", WindowEvents()),
	)
	plan, err := runtimePath.Build(aggregate.IntoTable(
		module.QualifiedName("MyTableAggs"), StatementName("B"),
	))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for _, event := range []clientCompileEnginePathBean{{TheString: "E1"}, {TheString: "E1"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	table, ok := engine.TableInModule(module.Name(), "MyTableAggs")
	if !ok {
		t.Fatal("runtime-path aggregate table is missing")
	}
	rows, err := table.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("thecnt").Any() != int64(2) {
		t.Fatalf("runtime-path aggregate rows = %#v", rows)
	}
	events, ok := rows[0].Get("thewin").Any().([]Event)
	if !ok || len(events) != 2 {
		t.Fatalf("runtime-path aggregate window = %#v", rows[0].Get("thewin").Any())
	}
}

func TestClientCompileEnginePathNamedWindowUseMatchesEsper(t *testing.T) {
	env, engine := newClientCompileEnginePathEnvironment(t)
	module, err := env.RegisterModule("runtime-window", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	eventSchema, err := module.RegisterMap("Event", []FieldSpec{
		FieldDef("string_field", reflect.TypeOf("")),
		FieldDef("double_field", reflect.TypeOf(float64(0))),
	}, BusEventType())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterNamedWindow("EventWindow", eventSchema,
		NamedWindowRetention(TimeWindow(600*time.Millisecond))); err != nil {
		t.Fatal(err)
	}
	activateClientCompileEnginePathModule(t, env, engine, module)
	runtimePath := engine.RuntimePath()
	eventType, err := runtimePath.EventType("Event")
	if err != nil {
		t.Fatal(err)
	}
	window, err := runtimePath.NamedWindow("EventWindow")
	if err != nil {
		t.Fatal(err)
	}
	insertPlan, err := runtimePath.Build(OnRecord(FromAny(env, eventType)).InsertIntoNamedWindow(
		"EventWindow",
		SetColumn("string_field", Field[any, string]("string_field")),
		SetColumn("double_field", Field[any, float64]("double_field")),
	).InModule(module).Query(StatementName("insert-window")))
	if err != nil {
		t.Fatal(err)
	}
	aggregatePlan, err := runtimePath.Build(window.Aggregate(
		Alias("sum_double_field", Sum[float64](Field[any, float64]("double_field"))),
		Alias("string_field", Last[string](Field[any, string]("string_field"))),
		Alias("window", WindowEvents()),
	).Query(StatementName("window-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	consumer, err := engine.Deploy(context.Background(), aggregatePlan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := consumer.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, record := range []map[string]any{
		{"string_field": "E1", "double_field": 10.0},
		{"string_field": "E2", "double_field": 20.0},
	} {
		if err := engine.SendBusRecord(context.Background(), module.EventType("Event"), record); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("runtime-path named-window rows = %d", len(rows))
	}
	last := rows[len(rows)-1]
	if last.Get("sum_double_field").Any() != 30.0 || last.Get("string_field").Any() != "E2" {
		t.Fatalf("runtime-path named-window last row = %#v", last)
	}
	events, ok := last.Get("window").Any().([]Event)
	if !ok || len(events) != 2 {
		t.Fatalf("runtime-path named-window contents = %#v", last.Get("window").Any())
	}
}
