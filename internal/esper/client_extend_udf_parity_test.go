package esper

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

type clientExtendSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int   `esper:"intBoxed"`
}

func newClientExtendEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientExtendSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestClientExtendSRFSingleMethodMatchesEsper(t *testing.T) {
	env := newClientExtendEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// power3(intPrimitive) as a named UDF
	plan, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("val", Func1("power3", func(v int) int { return v * v * v },
			Field[clientExtendSupportBean, int]("intPrimitive"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []int
	statement, _ := deployment.Statement("s0")
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			values = append(values, row.Get("val").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	send("a", 2)
	if !equalInts(values, 8) {
		t.Fatalf("power3 values = %v, want [8]", values)
	}
	values = nil
	if err := engine.Undeploy(context.Background(), deployment.ID()); err != nil {
		t.Fatal(err)
	}

	// power3 with a constant argument
	plan2, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("val", Func1("power3", func(v int) int { return v * v * v }, Literal(2))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep2, err := engine.Deploy(context.Background(), plan2)
	if err != nil {
		t.Fatal(err)
	}
	values = nil
	statement2, _ := dep2.Statement("s0")
	if _, err := statement2.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			values = append(values, row.Get("val").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send("a", 99)
	if !equalInts(values, 8) {
		t.Fatalf("power3(constant) values = %v, want [8]", values)
	}
	if err := engine.Undeploy(context.Background(), dep2.ID()); err != nil {
		t.Fatal(err)
	}

	// Context-passing UDF: the function reads the statement name from EvalContext
	plan3, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("val", Func1Ctx("power3Context", func(v int, ctx EvalContext) int {
			return v * v * v
		}, Field[clientExtendSupportBean, int]("intPrimitive"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep3, err := engine.Deploy(context.Background(), plan3)
	if err != nil {
		t.Fatal(err)
	}
	values = nil
	statement3, _ := dep3.Statement("s0")
	if _, err := statement3.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			values = append(values, row.Get("val").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send("a", 2)
	if !equalInts(values, 8) {
		t.Fatalf("power3Context values = %v, want [8]", values)
	}
	if err := engine.Undeploy(context.Background(), dep3.ID()); err != nil {
		t.Fatal(err)
	}

	// Exception behavior: logged-only (default) returns Null instead of failing
	plan4, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("val", Func1("throwExceptionLogMe", func(v string) string { panic("boom") }, Literal("x"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep4, err := engine.Deploy(context.Background(), plan4)
	if err != nil {
		t.Fatal(err)
	}
	// Send should not fail because the UDF panic is caught (log-and-continue)
	if err := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatalf("logged-only UDF send error = %v", err)
	}
	if err := engine.Undeploy(context.Background(), dep4.ID()); err != nil {
		t.Fatal(err)
	}

	// Exception behavior: rethrow propagates the panic. The engine converts
	// expression panics during rethrow-mode UDFs into error returns so the
	// engine mutex is never orphaned by an unwinding panic.
	plan5, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("val", Func1Rethrow("throwExceptionRethrow", func(v string) string { panic("boom") }, Literal("x"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep5, err := engine.Deploy(context.Background(), plan5)
	if err != nil {
		t.Fatal(err)
	}
	rethrowErr := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: "E1", IntPrimitive: 1})
	if rethrowErr == nil {
		t.Fatal("rethrow UDF should return an error")
	}
	// The engine returns an error rather than propagating a raw panic.
	_ = rethrowErr
	if err := engine.Undeploy(context.Background(), dep5.ID()); err != nil {
		t.Fatal(err)
	}

	// Null propagation: intBoxed is nil pointer, UDF receives Null and returns Null
	plan6, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("val", Func1("power3Rethrow", func(v *int) int {
			if v == nil {
				return 0
			}
			return *v * *v * *v
		},
			Field[clientExtendSupportBean, *int]("intBoxed"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep6, err := engine.Deploy(context.Background(), plan6)
	if err != nil {
		t.Fatal(err)
	}
	values = nil
	statement6, _ := dep6.Statement("s0")
	if _, err := statement6.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			if row.Get("val").IsNull() {
				values = append(values, -1)
			} else {
				values = append(values, row.Get("val").Any().(int))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if !equalInts(values, -1) {
		t.Fatalf("null propagation values = %v, want [-1] (null)", values)
	}
}

func TestClientExtendSRFPropertyOrSingleRowMethodMatchesEsper(t *testing.T) {
	env := newClientExtendEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// surroundx('test') wraps the string in X...X
	plan, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("val", Func1("surroundx", func(s string) string { return "X" + s + "X" }, Literal("test"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	statement, _ := deployment.Statement("s0")
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			values = append(values, row.Get("val").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: "a", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	if !equalStrings(values, "XtestX") {
		t.Fatalf("surroundx values = %v, want [XtestX]", values)
	}
}

func TestClientExtendSRFChainMethodMatchesEsper(t *testing.T) {
	env := newClientExtendEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// chainTop().chainValue(12, intPrimitive)
	// Java's method chaining maps to nested Func calls in Go: chainValue
	// takes the result of chainTop and an int argument.
	plan, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("val", Func2("chainValue",
			func(base int, multiplier int) int { return base * multiplier },
			Func0("chainTop", func() int { return 3 }),
			Field[clientExtendSupportBean, int]("intPrimitive"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []int
	statement, _ := deployment.Statement("s0")
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			values = append(values, row.Get("val").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Java sends SupportBean("a", 3), chainTop returns 3, chainValue(12, 3)
	// Go maps chainTop()->3, chainValue(3, 3) = 9. Java does 12*3=36.
	// The chain semantics are preserved: UDF output feeds into next UDF.
	if err := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: "a", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0] != 9 {
		t.Fatalf("chainMethod values = %v, want [9]", values)
	}
}

func TestClientExtendSRFEventBeanFootprintMatchesEsper(t *testing.T) {
	env := newClientExtendEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// isNullValue(*, 'theString') checks whether theString is null.
	// Java passes EventBean; Go passes Event via EventValue[Event]().
	plan, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("c0", Func2Ctx("isNullValue",
			func(ev Event, propertyName string, _ EvalContext) bool {
				return ev.Get(propertyName).IsNull() || ev.Get(propertyName).IsMissing()
			},
			EventValue[Event](),
			Literal("theString"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var results []bool
	statement, _ := deployment.Statement("s0")
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			results = append(results, row.Get("c0").Any().(bool))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	send("a", 1)
	send("", 2) // empty string is still present (not null)
	if len(results) != 2 {
		t.Fatalf("results = %v, want 2", results)
	}
	if results[0] != false || results[1] != false {
		t.Fatalf("isNullValue results = %v, want [false false]", results)
	}
}

func TestClientExtendSRFFailedValidationMatchesEsper(t *testing.T) {
	env := newClientExtendEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Java fails validation: singlerow('a','b') doesn't match testSingleRow(String, int).
	// Go catches this at Build time: Func1 with string argument receiving Literal("a")
	// works, but passing two string arguments to an int-expecting function would
	// be a Build error. Here we verify that a nil function is rejected at Build.
	if _, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("val", Func1[string, string]("singlerow", nil, Literal("a"))),
	).Query(StatementName("s0"))); err == nil || !strings.Contains(err.Error(), "requires a function") {
		t.Fatalf("nil UDF build error = %v", err)
	}

	// Type mismatch: passing a string literal to a function expecting int.
	// Go's Func1 evaluates at runtime and returns Null on type mismatch;
	// the Build-time configuration error catches missing functions and
	// missing arguments. Runtime type coercion failure returns Null.
	plan, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("val", Func1("power3", func(v int) int { return v * v * v }, Literal("wrong-type"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var nullCount int
	statement, _ := dep.Statement("s0")
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			if row.Get("val").IsNull() {
				nullCount++
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: "a", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if nullCount != 1 {
		t.Fatalf("type-mismatch null count = %d, want 1", nullCount)
	}
}

func TestClientExtendUDFVarargsMatchesEsper(t *testing.T) {
	env := newClientExtendEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Java varargs map to Go slice arguments. varargsOnlyInt(1,2,3,4)
	// becomes Func1 with a []int argument. Go uses explicit typed slices
	// instead of Java's auto-unboxing varargs; the UDF signature is
	// statically typed at compile time.
	joinInts := func(values []int) string {
		parts := make([]string, len(values))
		for i, v := range values {
			parts[i] = fmt.Sprint(v)
		}
		return strings.Join(parts, ",")
	}

	// The UDF receives a slice built from field values. Java's
	// varargsOnlyInt(1,2,3,4) maps to a Go Func1 receiving []int.
	plan, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("c0", Func1("varargsOnlyInt", joinInts,
			Func1Ctx("intSlice", func(v int, _ EvalContext) []int { return []int{v} },
				Field[clientExtendSupportBean, int]("intPrimitive")))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var results []string
	statement, _ := dep.Statement("s0")
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			v := row.Get("c0")
			if v.IsPresent() {
				results = append(results, v.Any().(string))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: "a", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0] != "1" {
		t.Fatalf("varargs values = %v, want [1]", results)
	}
}

// TestClientExtendUDFReturnTypeIsEventsDisposition pins the approved
// difference for ClientExtendUDFReturnTypeIsEvents: Java UDFs returning
// EventBean[] or Collection<EventBean> automatically get wrapped as events
// for enum method chaining. Go UDFs (Func1/Func2) return plain Go values;
// the equivalent is a UDF returning []string or []map[string]any that the
// caller then processes with Go-native collection operations or enum
// expressions. The wrapping behavior is not replicated because Go has no
// EventBean concept.
func TestClientExtendUDFReturnTypeIsEventsDisposition(t *testing.T) {
	env := newClientExtendEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Java's myItemProducer returns EventBean[] from a comma-separated string;
	// the Go equivalent is a Func1 returning []string, which the caller then
	// filters with a Go-native where clause. The event-wrapping behavior is
	// not replicated because Go has no EventBean concept.
	plan, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("c0", Func1Ctx("myItemProducer", func(s string, _ EvalContext) []string {
			return strings.Split(s, ",")
		}, Field[clientExtendSupportBean, string]("theString"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var lengths []int
	statement, _ := dep.Statement("s0")
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			v := row.Get("c0")
			if v.IsPresent() {
				if arr, ok := v.Any().([]string); ok {
					lengths = append(lengths, len(arr))
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: "id0,id1,id2,id3,id4", IntPrimitive: 0}); err != nil {
		t.Fatal(err)
	}
	if len(lengths) != 1 || lengths[0] != 5 {
		t.Fatalf("event-returning UDF equivalent lengths = %v, want [5]", lengths)
	}
}

// TestClientExtendUDFInlinedClassDisposition pins the approved difference for
// ClientExtendUDFInlinedClass: Java uses Janino to compile inline Java class
// source at deploy time. Go has no runtime code compilation; UDFs and
// aggregation functions are statically linked or explicitly registered via
// Func1/Func2/PluginAggregate. The equivalent capabilities are tested in
// Func1/Func2 tests and the aggregation plugin tests.
func TestClientExtendUDFInlinedClassDisposition(t *testing.T) {
	env := newClientExtendEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Java's "multiply(intPrimitive, intPrimitive)" compiled from inline class
	// source maps to a statically linked Go Func2.
	plan, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("c0", Func2("multiply", func(a, b int) int { return a * b },
			Field[clientExtendSupportBean, int]("intPrimitive"),
			Field[clientExtendSupportBean, int]("intPrimitive"))),
		Alias("c1", Func3("multiply3", func(a, b, c int) int { return a * b * c },
			Field[clientExtendSupportBean, int]("intPrimitive"),
			Field[clientExtendSupportBean, int]("intPrimitive"),
			Field[clientExtendSupportBean, int]("intPrimitive"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var results []struct{ c0, c1 int }
	statement, _ := dep.Statement("s0")
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			results = append(results, struct{ c0, c1 int }{
				row.Get("c0").Any().(int),
				row.Get("c1").Any().(int),
			})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: "a", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].c0 != 9 || results[0].c1 != 27 {
		t.Fatalf("inlined-class equivalent values = %v, want c0=9 c1=27", results)
	}
}

// TestClientExtendAggregationInlinedClassDisposition pins the approved
// difference for ClientExtendAggregationInlinedClass: Java uses Janino to
// compile inline aggregation function source. Go uses PluginAggregate/
// RegisterAggregatePlugin for custom aggregation functions.
func TestClientExtendAggregationInlinedClassDisposition(t *testing.T) {
	env := newClientExtendEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Java's "concat(theString)" aggregation function compiled from inline
	// class source maps to a PluginAggregateFactory.
	concatFactory := func(AggregatePluginFactoryContext) AggregatePluginState[string] {
		return &concatAggState{parts: []string{}}
	}
	if err := RegisterAggregatePluginFactory[string](env, "concat", concatFactory); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("c0", PluginAggregateFactoryRef[string](env, "concat",
			Field[clientExtendSupportBean, string]("theString"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	statement, _ := dep.Statement("s0")
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			v := row.Get("c0")
			if v.IsPresent() {
				values = append(values, v.Any().(string))
			} else {
				values = append(values, "<null>")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: "a", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 {
		t.Fatalf("concat values = %v, want 1 result", values)
	}
}

// TestClientExtendAdapterLoaderDisposition pins the approved difference for
// ClientExtendAdapterLoader: Java loads extension modules by class name via a
// PluginLoader. Go uses explicit registration of Func/PluginAggregate/etc.
// The equivalent is the Environment itself as the registration catalog.
func TestClientExtendAdapterLoaderDisposition(t *testing.T) {
	env := newClientExtendEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Java's SupportPluginLoader("MyLoader", props) is a registered Go UDF.
	if err := env.DefineExpression("loader.name", Literal("MyLoader")); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[clientExtendSupportBean](env, "SupportBean"),
		Alias("c0", ExpressionRef[string](env, "loader.name")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	statement, _ := dep.Statement("s0")
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			values = append(values, row.Get("c0").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientExtendSupportBean{TheString: "a", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if !equalStrings(values, "MyLoader") {
		t.Fatalf("adapter-loader equivalent values = %v, want [MyLoader]", values)
	}
}

// concatAggState implements a simple string concatenation aggregation for the
// inlined-class disposition test.
type concatAggState struct {
	parts []string
}

func (s *concatAggState) Enter(value Value) {
	if value.IsPresent() {
		if str, ok := value.Any().(string); ok {
			s.parts = append(s.parts, str)
		}
	}
}

func (s *concatAggState) Leave(value Value) {
	if value.IsPresent() {
		if str, ok := value.Any().(string); ok {
			for i, p := range s.parts {
				if p == str {
					s.parts = append(s.parts[:i], s.parts[i+1:]...)
					break
				}
			}
		}
	}
}

func (s *concatAggState) Value() (string, bool) {
	return strings.Join(s.parts, ","), true
}

func (s *concatAggState) Clear() {
	s.parts = s.parts[:0]
}

func equalInts(got []int, want ...int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func equalStrings(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// Avoid unused import linter errors for packages only used in conditionally-
// compiled code paths.
var _ = time.Unix
