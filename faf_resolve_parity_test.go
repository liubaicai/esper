package esper

import (
	"context"
	"errors"
	"testing"
)

// InfraNWTableFAFResolve compiles the same logical MyInfra name against two
// protected Java modules. Go keeps the same resolution boundary explicit in
// the source handle instead of parsing module/EPL text.
func TestInfraFAFResolveModuleBoundNamedWindowsMatchEsper(t *testing.T) {
	env := NewEnvironment()
	moduleA, err := env.RegisterModule("A")
	if err != nil {
		t.Fatal(err)
	}
	moduleB, err := env.RegisterModule("B")
	if err != nil {
		t.Fatal(err)
	}

	schemaA := mustFAFResolveSchema(t, env, "InfraResolveWindowA", []FieldSpec{
		FieldDef("c0", typeOf[string]()),
		FieldDef("c1", typeOf[int64]()),
	})
	schemaB := mustFAFResolveSchema(t, env, "InfraResolveWindowB", []FieldSpec{
		FieldDef("c2", typeOf[int64]()),
		FieldDef("c3", typeOf[string]()),
	})
	if _, err := moduleA.RegisterNamedWindow("MyInfra", schemaA, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	if _, err := moduleB.RegisterNamedWindow("MyInfra", schemaB, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	ctx := context.Background()
	insertA, err := env.Build(moduleA.NamedWindow("MyInfra").OnDemand().Insert(
		SetColumn("c0", Literal("A1")),
		SetColumn("c1", Literal[int64](10)),
	))
	if err != nil {
		t.Fatal(err)
	}
	insertB, err := env.Build(moduleB.NamedWindow("MyInfra").OnDemand().Insert(
		SetColumn("c2", Literal[int64](20)),
		SetColumn("c3", Literal("B1")),
	))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ExecuteFireAndForget(ctx, insertA); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ExecuteFireAndForget(ctx, insertB); err != nil {
		t.Fatal(err)
	}

	selectA, err := env.Build(moduleA.NamedWindow("MyInfra").Query(StatementName("resolve-window-a")))
	if err != nil {
		t.Fatal(err)
	}
	selectB, err := env.Build(moduleB.NamedWindow("MyInfra").Query(StatementName("resolve-window-b")))
	if err != nil {
		t.Fatal(err)
	}
	if selectA.Hash() == selectB.Hash() || string(selectA.Canonical()) == string(selectB.Canonical()) {
		t.Fatal("module-bound named-window plans must have distinct identities")
	}
	resultA, err := engine.ExecuteFireAndForget(ctx, selectA)
	if err != nil {
		t.Fatal(err)
	}
	resultB, err := engine.ExecuteFireAndForget(ctx, selectB)
	if err != nil {
		t.Fatal(err)
	}
	assertFAFResolveRow(t, resultA, map[string]any{"c0": "A1", "c1": int64(10)})
	assertFAFResolveRow(t, resultB, map[string]any{"c2": int64(20), "c3": "B1"})

	if _, ok := engine.NamedWindowInModule("A", "MyInfra"); !ok {
		t.Fatal("module A named window is missing")
	}
	if _, ok := engine.NamedWindowInModule("B", "MyInfra"); !ok {
		t.Fatal("module B named window is missing")
	}
}

func TestInfraFAFResolveModuleBoundTablesMatchEsper(t *testing.T) {
	env := NewEnvironment()
	moduleA, err := RegisterModule(env, "A")
	if err != nil {
		t.Fatal(err)
	}
	moduleB, err := RegisterModule(env, "B")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := moduleA.RegisterTable("MyInfra", []TableColumn{
		PrimaryKeyColumn[string]("c0"),
		PrimaryKeyColumn[int64]("c1"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := moduleB.RegisterTable("MyInfra", []TableColumn{
		PrimaryKeyColumn[int64]("c2"),
		PrimaryKeyColumn[string]("c3"),
	}); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	ctx := context.Background()
	insertA, err := env.Build(moduleA.Table("MyInfra").OnDemand().Insert(
		SetColumn("c0", Literal("A1")),
		SetColumn("c1", Literal[int64](10)),
	))
	if err != nil {
		t.Fatal(err)
	}
	insertB, err := env.Build(moduleB.Table("MyInfra").OnDemand().Insert(
		SetColumn("c2", Literal[int64](20)),
		SetColumn("c3", Literal("B1")),
	))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ExecuteFireAndForget(ctx, insertA); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ExecuteFireAndForget(ctx, insertB); err != nil {
		t.Fatal(err)
	}

	selectA, err := env.Build(moduleA.Table("MyInfra").Query())
	if err != nil {
		t.Fatal(err)
	}
	selectB, err := env.Build(moduleB.Table("MyInfra").Query())
	if err != nil {
		t.Fatal(err)
	}
	resultA, err := engine.ExecuteFireAndForget(ctx, selectA)
	if err != nil {
		t.Fatal(err)
	}
	resultB, err := engine.ExecuteFireAndForget(ctx, selectB)
	if err != nil {
		t.Fatal(err)
	}
	assertFAFResolveRow(t, resultA, map[string]any{"c0": "A1", "c1": int64(10)})
	assertFAFResolveRow(t, resultB, map[string]any{"c2": int64(20), "c3": "B1"})

	if _, ok := engine.TableInModule("A", "MyInfra"); !ok {
		t.Fatal("module A table is missing")
	}
	if _, ok := engine.TableInModule("B", "MyInfra"); !ok {
		t.Fatal("module B table is missing")
	}
}

func TestInfraFAFResolveRejectsUnknownModule(t *testing.T) {
	env := NewEnvironment()
	if _, err := env.Build(FromNamedWindowInModule(env, "missing", "MyInfra").Query()); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown module source error = %v", err)
	}
	if _, err := env.RegisterTableInModule("missing", "MyInfra", []TableColumn{TableColumnOf[string]("c0")}); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown module table error = %v", err)
	}
	if _, err := env.RegisterModule(" "); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("blank module error = %v", err)
	}
}

func mustFAFResolveSchema(t *testing.T, env *Environment, name string, fields []FieldSpec) Schema {
	t.Helper()
	if _, err := RegisterMap(env, name, fields); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema(name)
	if !ok {
		t.Fatalf("schema %q is missing", name)
	}
	return schema
}

func assertFAFResolveRow(t *testing.T, result QueryResult, expected map[string]any) {
	t.Helper()
	rows := result.Results()
	if len(rows) != 1 {
		t.Fatalf("result rows = %#v, want one row", rows)
	}
	for field, want := range expected {
		if got := rows[0].Get(field).Any(); got != want {
			t.Fatalf("result field %s = %#v, want %#v", field, got, want)
		}
	}
}
