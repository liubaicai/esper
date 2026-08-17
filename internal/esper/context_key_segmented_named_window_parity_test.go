package esper

import (
	"context"
	"testing"
)

// TestContextKeyedNamedWindowNonPatternParity locks the context-partitioned
// named window consumer verified against ContextKeyedNamedWindowNonPattern:
// `partition by theString from SupportBean` with a keepall named window,
// insert into window, and select from window directly.
// Each partition maintains its own window contents.
func TestContextKeyedNamedWindowNonPatternParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[keyedNWBean](env, "SupportBean")
	if err != nil {
		t.Fatal(err)
	}
	theString := Field[keyedNWBean, string]("theString")
	if _, err := CreateKeyContext(env, "Ctx", theString); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", schema,
		NamedWindowContext("Ctx"),
		NamedWindowRetention(KeepAll()),
	); err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "MyWindow").Select(
		Alias("c0", ContextField[string]("key1")),
		Alias("c1", Field[keyedNWBean, int]("intPrimitive")),
	).Query(StatementName("s0"), WithContext("Ctx")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	type row struct {
		c0 string
		c1 int
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					c0: rowValue.Get("c0").Any().(string),
					c1: rowValue.Get("c1").Any().(int),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	insert := func(s string, i int) {
		t.Helper()
		if err := engine.InsertNamedWindow(context.Background(), "MyWindow", keyedNWBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	insert("E1", 1) // E1 partition: window=[E1/1]
	insert("E2", 2) // E2 partition: window=[E2/2]
	insert("E1", 3) // E1 partition: window=[E1/1, E1/3]
	insert("E2", 4) // E2 partition: window=[E2/2, E2/4]
	want := []row{{"E1", 1}, {"E2", 2}, {"E1", 3}, {"E2", 4}}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, want %d: %#v", len(rows), len(want), rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("rows[%d] = %#v, want %#v", i, rows[i], want[i])
		}
	}
}

// TestContextKeyedNamedWindowPatternParity locks the context-partitioned
// named window consumer via pattern verified against
// ContextKeyedNamedWindowPattern: same as NonPattern but selects from
// `pattern [every a=MyWindow]` instead of directly from the window.
func TestContextKeyedNamedWindowPatternParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[keyedNWBean](env, "SupportBean")
	if err != nil {
		t.Fatal(err)
	}
	theString := Field[keyedNWBean, string]("theString")
	if _, err := CreateKeyContext(env, "Ctx", theString); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", schema,
		NamedWindowContext("Ctx"),
		NamedWindowRetention(KeepAll()),
	); err != nil {
		t.Fatal(err)
	}
	// Select from pattern [every a=MyWindow] instead of directly from window
	pattern := PatternFromRecord(
		FromNamedWindow(env, "MyWindow"),
		"a",
		Literal[bool](true),
	).Every()
	consumerPlan, err := env.Build(pattern.Select(
		Alias("c0", ContextField[string]("key1")),
		Alias("c1", Field[keyedNWBean, int]("intPrimitive")),
	).Query(StatementName("s0"), WithContext("Ctx")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	type row struct {
		c0 string
		c1 int
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					c0: rowValue.Get("c0").Any().(string),
					c1: rowValue.Get("c1").Any().(int),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	insert := func(s string, i int) {
		t.Helper()
		if err := engine.InsertNamedWindow(context.Background(), "MyWindow", keyedNWBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	insert("E1", 1)
	insert("E2", 2)
	insert("E1", 3)
	insert("E2", 4)
	want := []row{{"E1", 1}, {"E2", 2}, {"E1", 3}, {"E2", 4}}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, want %d: %#v", len(rows), len(want), rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("rows[%d] = %#v, want %#v", i, rows[i], want[i])
		}
	}
}

// NOTE: ContextKeyedNamedWindowFAF requires FAF on context-partitioned
// named windows which is not supported in the Go fluent API.
// The Java execution uses compileFAF("select * from MyWindow") on a
// context-scoped window; Go FAF does not currently support this pattern.

type keyedNWBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}
