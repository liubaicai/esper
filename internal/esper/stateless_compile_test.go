package esper

import (
	"context"
	"testing"
	"time"
)

// The compiled stateless chain must be observationally identical to the
// generic closure walk for every event representation. Each case below
// deploys one eligible statement, forces the plan to resolve, and then
// evaluates both paths over the same event matrix: shapes cover string and
// numeric comparisons, containment, pure builtins, logic combinations,
// membership, null probes, case-insensitive resolution, and the nil-pointer
// anonymous-candidate fallback.

type statelessCompileEvent struct {
	Category string  `esper:"category"`
	Label    *string `esper:"label"`
	Count    int     `esper:"count"`
	Score    float64 `esper:"score"`
	Empty    string  `esper:"empty"`
}

type statelessCompileEmbedded struct {
	*statelessCompileBase
	Leaf string `esper:"leaf"`
}

type statelessCompileBase struct {
	Value string `esper:"value"`
}

type statelessCompileShadowed struct {
	statelessCompileBase
	Value string `esper:"override"`
}

func statelessCompileDeploy(t *testing.T, predicate Expression[bool]) (*Engine, *Statement, func()) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[statelessCompileEvent](env, "StatelessCompileEvent"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[statelessCompileEvent](env, "StatelessCompileEvent").
		Filter(predicate).
		Query(StatementName("stateless-compile")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	ctx := context.Background()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if err := engine.SendEvent(ctx, statelessCompileEvent{Category: "keep"}); err != nil {
		t.Fatal(err)
	}
	if statement.statelessPlan == nil {
		t.Fatalf("predicate %s did not resolve a stateless plan", predicate.Description())
	}
	return engine, statement, func() { _ = engine.Close(ctx) }
}

// statelessCompileAssertEquivalence checks the compiled chain against the
// generic filter walk event by event.
func statelessCompileAssertEquivalence(t *testing.T, engine *Engine, statement *Statement, events []statelessCompileEvent) {
	t.Helper()
	plan := statement.statelessPlan
	if plan.compiled == nil {
		t.Fatalf("predicate did not compile; plan=%v", plan)
	}
	env := engine.env
	now := time.Now()
	scope := variablesWithEngineLockState(nil, engine, true)
	for _, value := range events {
		event, err := newEvent(mustSchemaOf(env, "StatelessCompileEvent"), value, now)
		if err != nil {
			t.Fatal(err)
		}
		compiled := statelessCompiledMatch(plan.compiled, event)
		generic := sourceNodeMatchesEventFilter(env, plan.input, event, now, scope, engine)
		if compiled != generic {
			t.Fatalf("compiled=%v generic=%v for event %#v", compiled, generic, value)
		}
	}
}

// mustSchemaOf fetches a registered schema or panics; test helper only.
func mustSchemaOf(env *Environment, name string) Schema {
	schema, ok := env.Schema(name)
	if !ok || !schema.valid() {
		panic("schema " + name + " must be registered and valid")
	}
	return schema
}
func TestStatelessCompiledMatchesGenericMatrix(t *testing.T) {
	label := "alpha"
	zero := ""
	present := func(v string) *string { return &v }
	events := []statelessCompileEvent{
		{Category: "keep", Label: &label, Count: 5, Score: 1.5},
		{Category: "Keep", Count: 0},
		{Category: "drop", Label: &zero, Count: -3, Score: -0.5},
		{Label: nil, Count: 7, Score: 0},
		{Category: "category", Empty: ""},
		{Category: "keep", Label: present("ALPHA"), Count: 10, Score: 2},
	}
	cases := []struct {
		name      string
		predicate Expression[bool]
	}{
		{"eq-string", Equal[string](Field[statelessCompileEvent, string]("category"), Literal("keep"))},
		{"eq-pointer-field", Equal[string](Field[statelessCompileEvent, string]("label"), Literal("alpha"))},
		{"neq-string", NotEqual[string](Field[statelessCompileEvent, string]("category"), Literal("keep"))},
		{"gt-int", Greater[int](Field[statelessCompileEvent, int]("count"), Literal(0))},
		{"lte-float", LessOrEqual[float64](Field[statelessCompileEvent, float64]("score"), Literal(1.5))},
		{"mixed-numeric", Greater[float64](Field[statelessCompileEvent, float64]("count"), Literal(0.5))},
		{"contains", Contains(Field[statelessCompileEvent, string]("category"), Literal("ee"))},
		{"contains-null-operand", Contains(Field[statelessCompileEvent, string]("label"), Literal("al"))},
		{"starts-with", StartsWith(Field[statelessCompileEvent, string]("category"), Literal("ke"))},
		{"ends-with", EndsWith(Field[statelessCompileEvent, string]("category"), Literal("ep"))},
		{"lower-contains", Contains(Lower(Field[statelessCompileEvent, string]("category")), Literal("KEE"))},
		{"upper-eq", Equal[string](Upper(Field[statelessCompileEvent, string]("category")), Literal("KEEP"))},
		{"trim-eq", Equal[string](Trim(Field[statelessCompileEvent, string]("category")), Literal("keep"))},
		{"length-gt", Greater[int64](StringLength(Field[statelessCompileEvent, string]("category")), Literal(int64(3)))},
		{"and-or", And(Or(Equal[string](Field[statelessCompileEvent, string]("category"), Literal("keep")), Greater[int](Field[statelessCompileEvent, int]("count"), Literal(8))), Not(Equal[string](Field[statelessCompileEvent, string]("label"), NullLiteral[string]())))},
		{"in-literals", In[string](Field[statelessCompileEvent, string]("category"), Literal("keep"), Literal("hold"))},
		{"in-with-null", In[string](Field[statelessCompileEvent, string]("label"), Literal("alpha"), NullLiteral[string]())},
		{"is-null-label", IsNull[string](Field[statelessCompileEvent, string]("label"))},
		{"is-missing", IsMissing[string](Field[statelessCompileEvent, string]("category"))},
		{"nested-logic", Not(And(IsNull[string](Field[statelessCompileEvent, string]("label")), GreaterOrEqual[int](Field[statelessCompileEvent, int]("count"), Literal(5))))},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			engine, statement, closeEngine := statelessCompileDeploy(t, testCase.predicate)
			defer closeEngine()
			statelessCompileAssertEquivalence(t, engine, statement, events)
		})
	}
}

// TestStatelessCompiledCaseInsensitiveAndFallback pins two structural edges:
// case-insensitive field resolution must select the same candidate as the
// generic walk, and the anonymous nil-pointer candidate fallback must agree
// when the first embedded candidate is a nil pointer.
func TestStatelessCompiledCaseInsensitiveAndFallback(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[statelessCompileEmbedded](env, "StatelessCompileEmbedded", WithPropertyResolution(PropertyCaseInsensitive)); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[statelessCompileShadowed](env, "StatelessCompileShadowed"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	base := "base-value"

	plan, err := env.Build(From[statelessCompileEmbedded](env, "StatelessCompileEmbedded").
		Filter(Equal[string](Field[statelessCompileEmbedded, string]("value"), Literal("base-value"))).
		Query(StatementName("embedded")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if err := engine.SendEvent(context.Background(), statelessCompileEmbedded{}); err != nil {
		t.Fatal(err)
	}
	if statement.statelessPlan == nil || statement.statelessPlan.compiled == nil {
		t.Fatal("embedded predicate did not compile")
	}
	scope := variablesWithEngineLockState(nil, engine, true)
	for _, value := range []statelessCompileEmbedded{
		{statelessCompileBase: &statelessCompileBase{Value: base}, Leaf: "x"},
		{Leaf: "x"},
	} {
		event, err := newEvent(mustSchemaOf(env, "StatelessCompileEmbedded"), value, now)
		if err != nil {
			t.Fatal(err)
		}
		compiled := statelessCompiledMatch(statement.statelessPlan.compiled, event)
		generic := sourceNodeMatchesEventFilter(env, statement.statelessPlan.input, event, now, scope, engine)
		if compiled != generic {
			t.Fatalf("embedded compiled=%v generic=%v for %#v", compiled, generic, value)
		}
	}
}
