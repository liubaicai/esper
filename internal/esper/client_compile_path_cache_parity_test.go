package esper

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type clientCompilePathCacheBean struct {
	TheString string `esper:"theString"`
}

type clientCompilePathCacheSignal struct {
	ID string `esper:"id"`
}

func newClientCompilePathCacheEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompilePathCacheBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[clientCompilePathCacheSignal](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[clientCompilePathCacheSignal](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestClientCompilePathCacheObjectTypesMatchesEsper(t *testing.T) {
	env := newClientCompilePathCacheEnvironment(t)
	module, err := env.RegisterModule("path-cache-objects", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if err := module.RegisterVariable("myvariable", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterMap("MySchema", nil); err != nil {
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
	signalSchema, ok := env.Schema("SupportBean_S0")
	if !ok {
		t.Fatal("SupportBean_S0 schema is missing")
	}
	if _, err := module.RegisterNamedWindow("MyWindow", signalSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterTable("MyTable", []TableColumn{TableColumnOf[string]("y")}); err != nil {
		t.Fatal(err)
	}
	contextDefinition, err := NewKeyContext("MyContext", Field[clientCompilePathCacheBean, string]("theString"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterContextDefinition("MyContext", contextDefinition); err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterMap("CarLocUpdateEvent", []FieldSpec{
		FieldDef("carId", reflect.TypeOf("")),
		FieldDef("direction", reflect.TypeOf(0)),
	}, BusEventType()); err != nil {
		t.Fatal(err)
	}

	cache := NewCompilePathCache()
	cached, err := cache.Prepare(env.Uses(module))
	if err != nil {
		t.Fatal(err)
	}
	if cached.Fingerprint() == "" {
		t.Fatal("cached path fingerprint is empty")
	}
	myVariable, err := CachedPathVariableRef[int](cached, "myvariable")
	if err != nil {
		t.Fatal(err)
	}
	declared, err := CachedPathExpressionRef[string](cached, "myExpr")
	if err != nil {
		t.Fatal(err)
	}
	script, err := CachedPathScriptCall[int](cached, "myScript")
	if err != nil {
		t.Fatal(err)
	}
	inlineEquivalent, err := CachedPathExpressionRef[string](cached, "MyClass.doIt")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := cached.Build(Select(
		From[clientCompilePathCacheBean](env, "SupportBean"),
		Alias("c0", myVariable),
		Alias("c1", declared),
		Alias("c2", script),
		Alias("c4", inlineEquivalent),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plan.Canonical()), "uses(path-cache-objects)") {
		t.Fatalf("cached path identity = %s", plan.Canonical())
	}

	eventType, err := cached.EventType("MySchema")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cached.Build(FromAny(env, eventType).Query(StatementName("schema-consumer"))); err != nil {
		t.Fatal(err)
	}
	window, err := cached.NamedWindow("MyWindow")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cached.Build(window.Query(StatementName("window-consumer"))); err != nil {
		t.Fatal(err)
	}
	table, err := cached.Table("MyTable")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cached.Build(table.Select(Alias("y", Field[any, string]("y"))).Query(StatementName("table-consumer"))); err != nil {
		t.Fatal(err)
	}
	contextName, err := cached.Context("MyContext")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cached.Build(From[clientCompilePathCacheBean](env, "SupportBean").Query(
		StatementName("context-consumer"), WithContext(contextName),
	)); err != nil {
		t.Fatal(err)
	}
	signal := From[clientCompilePathCacheSignal](env, "SupportBean_S1")
	if _, err := cached.Build(OnEvent(signal).DeleteAllFromNamedWindow("MyWindow").InModule(module).Query(StatementName("delete-window"))); err != nil {
		t.Fatal(err)
	}
	if _, err := cached.Build(OnEvent(signal).DeleteAllFromTable("MyTable").InModule(module).Query(StatementName("delete-table"))); err != nil {
		t.Fatal(err)
	}
	carType, err := cached.EventType("CarLocUpdateEvent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cached.Build(FromAny(env, carType).Filter(
		Equal[int](Field[any, int]("direction"), Literal(1)),
	).Window(TimeWindow(time.Minute)).Aggregate(
		Alias("carId", Field[any, string]("carId")), Alias("direction", Field[any, int]("direction")), Alias("cnt", CountAll()),
	).Query(StatementName("car-count"))); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	// Java's runtime deployments provide module objects through a deployment
	// of the owning module; deploy the provider before the cached consumer.
	providerPlan, err := module.Build(
		Select(From[clientCompilePathCacheBean](env, "SupportBean"),
			Alias("theString", Field[clientCompilePathCacheBean, string]("theString"))).Query(StatementName("provider")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), providerPlan); err != nil {
		t.Fatal(err)
	}
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
	if err := engine.SendEvent(context.Background(), clientCompilePathCacheBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("c0").Any() != 10 || rows[0].Get("c1").Any() != "abc" || rows[0].Get("c2").Any() != 2 || rows[0].Get("c4").Any() != "def" {
		t.Fatalf("cached object result = %#v", rows)
	}

	again, err := cache.Prepare(env.Uses(module))
	if err != nil {
		t.Fatal(err)
	}
	if again.Fingerprint() != cached.Fingerprint() {
		t.Fatalf("cached path fingerprint changed: %s != %s", again.Fingerprint(), cached.Fingerprint())
	}
	if stats := cache.Stats(); stats.Hits != 1 || stats.Misses != 1 || stats.Entries != 1 {
		t.Fatalf("path cache stats = %#v", stats)
	}
}

func TestClientCompilePathCacheProtectedMatchesEsper(t *testing.T) {
	env := newClientCompilePathCacheEnvironment(t)
	module, err := env.RegisterModule("a.b.c", ProtectedModule())
	if err != nil {
		t.Fatal(err)
	}
	if err := module.RegisterVariable("myvariable", 10); err != nil {
		t.Fatal(err)
	}
	cache := NewCompilePathCache()
	first, err := cache.Prepare(module.Path())
	if err != nil {
		t.Fatal(err)
	}
	variable, err := CachedPathVariableRef[int](first, "myvariable")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Build(Select(
		From[clientCompilePathCacheBean](env, "SupportBean"),
		Alias("myvariable", variable),
	).Query(StatementName("protected-consumer"))); err != nil {
		t.Fatal(err)
	}
	second, err := cache.Prepare(module.Path())
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint() != second.Fingerprint() {
		t.Fatal("protected module path did not reuse its snapshot")
	}
	if stats := cache.Stats(); stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("protected path cache stats = %#v", stats)
	}
}

func TestClientCompilePathCacheFillByCompileMatchesEsper(t *testing.T) {
	env := newClientCompilePathCacheEnvironment(t)
	provider, err := env.RegisterModule("path-cache-provider", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.RegisterMap("Provided", nil); err != nil {
		t.Fatal(err)
	}
	cache := NewCompilePathCache()
	first, err := cache.Prepare(env.Uses(provider))
	if err != nil {
		t.Fatal(err)
	}
	provided, err := first.EventType("Provided")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Build(FromAny(env, provided).Query(StatementName("first-compile"))); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Prepare(env.Uses(provider)); err != nil {
		t.Fatal(err)
	}

	unrelated, err := env.RegisterModule("unrelated-private")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unrelated.RegisterMap("Hidden", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Prepare(env.Uses(provider)); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.RegisterMap("ProvidedLater", nil); err != nil {
		t.Fatal(err)
	}
	changed, err := cache.Prepare(env.Uses(provider))
	if err != nil {
		t.Fatal(err)
	}
	if changed.Fingerprint() == first.Fingerprint() {
		t.Fatal("selected catalog mutation did not invalidate cached path")
	}
	if stats := cache.Stats(); stats.Hits != 2 || stats.Misses != 2 || stats.Entries != 2 {
		t.Fatalf("fill-by-compile cache stats = %#v", stats)
	}
}

func TestClientCompilePathCacheEventTypeChainMatchesEsper(t *testing.T) {
	env := newClientCompilePathCacheEnvironment(t)
	cache := NewCompilePathCache()
	modules := make([]Module, 0, 10)
	var previous Schema
	for index := 0; index < 10; index++ {
		module, err := env.RegisterModule(fmt.Sprintf("path-chain-%d", index), PublicModule())
		if err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("L%d", index)
		var schema Schema
		if index == 0 {
			schema, err = module.RegisterMap(name, nil)
		} else {
			property := fmt.Sprintf("l%d", index-1)
			schema, err = module.RegisterMap(name, []FieldSpec{FieldDef(property, reflect.TypeOf(map[string]any{}))}, WithNestedPropertySchema(property, previous))
		}
		if err != nil {
			t.Fatal(err)
		}
		previous = schema
		modules = append(modules, module)
		cached, err := cache.Prepare(env.Uses(modules...))
		if err != nil {
			t.Fatal(err)
		}
		identity, err := cached.EventType(name)
		if err != nil {
			t.Fatal(err)
		}
		resolved, ok := env.Schema(identity)
		if !ok {
			t.Fatalf("cached event type %s is missing", identity)
		}
		if index > 0 {
			property := fmt.Sprintf("l%d", index-1)
			nested, ok := resolved.NestedSchema(property)
			if !ok || nested.Name() != previousNestedName(index-1) {
				t.Fatalf("event chain L%d nested = %q, %t", index, nested.Name(), ok)
			}
		}
	}
	if _, err := cache.Prepare(env.Uses(modules...)); err != nil {
		t.Fatal(err)
	}
	if stats := cache.Stats(); stats.Hits != 1 || stats.Misses != 10 || stats.Entries != 10 {
		t.Fatalf("event-chain cache stats = %#v", stats)
	}
}

func previousNestedName(index int) string {
	return fmt.Sprintf("path-chain-%d::L%d", index, index)
}

func TestClientCompilePathCacheInvalidMatchesEsper(t *testing.T) {
	env := newClientCompilePathCacheEnvironment(t)
	one, err := env.RegisterModule("duplicate-one", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	two, err := env.RegisterModule("duplicate-two", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := one.RegisterMap("A", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := two.RegisterMap("A", nil); err != nil {
		t.Fatal(err)
	}
	path := env.Uses(one, two)
	if _, err := path.EventType("A"); err == nil || !errors.Is(err, ErrorAmbiguous) {
		t.Fatalf("uncached duplicate path error = %v", err)
	}
	cache := NewCompilePathCache()
	if _, err := cache.Prepare(path); err == nil || !errors.Is(err, ErrorAmbiguous) {
		t.Fatalf("cached duplicate path error = %v", err)
	}
}

func TestCompilePathCacheConcurrentPrepareAndClear(t *testing.T) {
	env := newClientCompilePathCacheEnvironment(t)
	module, err := env.RegisterModule("concurrent-cache", PublicModule())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.RegisterMap("Visible", nil); err != nil {
		t.Fatal(err)
	}
	var cache CompilePathCache
	path := env.Uses(module)
	const workers = 16
	fingerprints := make(chan string, workers)
	errorsSeen := make(chan error, workers)
	var group sync.WaitGroup
	for index := 0; index < workers; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			cached, prepareErr := cache.Prepare(path)
			if prepareErr != nil {
				errorsSeen <- prepareErr
				return
			}
			fingerprints <- cached.Fingerprint()
		}()
	}
	group.Wait()
	close(errorsSeen)
	close(fingerprints)
	for prepareErr := range errorsSeen {
		t.Fatal(prepareErr)
	}
	want := ""
	for fingerprint := range fingerprints {
		if want == "" {
			want = fingerprint
		}
		if fingerprint == "" || fingerprint != want {
			t.Fatalf("concurrent fingerprint = %q, want %q", fingerprint, want)
		}
	}
	if stats := cache.Stats(); stats.Misses != 1 || stats.Hits != workers-1 || stats.Entries != 1 {
		t.Fatalf("concurrent path cache stats = %#v", stats)
	}
	cache.Clear()
	if stats := cache.Stats(); stats != (CompilePathCacheStats{}) {
		t.Fatalf("cleared path cache stats = %#v", stats)
	}
}
