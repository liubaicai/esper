package esper

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

// The Java InfraNWTableFAFSubquery suite deliberately mixes Named Window and
// Table lookup sources.  These helpers keep the test rules in the same
// Go-native shape while allowing one case to exercise both storage backends.
func newFAFSubquerySchema(t *testing.T, env *Environment, name string) Schema {
	t.Helper()
	_, err := RegisterMap(env, name, []FieldSpec{
		FieldDef("id", typeOf[int64]()),
		FieldDef("p00", typeOf[string]()),
		FieldDef("p10", typeOf[string]()),
		FieldDef("theString", typeOf[string]()),
		FieldDef("intPrimitive", typeOf[int64]()),
		FieldDef("value", typeOf[string]()),
	})
	if err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema(name)
	if !ok {
		t.Fatalf("schema %q is missing", name)
	}
	return schema
}

func createFAFSubqueryStore(t *testing.T, env *Environment, engine *Engine, name string, schema Schema, namedWindow bool, retention WindowSpec, contextName string) RecordStream {
	t.Helper()
	if namedWindow {
		options := make([]NamedWindowOption, 0, 2)
		if retention != nil {
			options = append(options, NamedWindowRetention(retention))
		}
		if contextName != "" {
			options = append(options, NamedWindowContext(contextName))
		}
		if _, err := CreateNamedWindow(env, name, schema, options...); err != nil {
			t.Fatal(err)
		}
		// The production contract declares stores before NewEngine.  Keeping
		// the helper tolerant of either order makes the individual parity
		// cases easier to read without weakening that lifecycle contract.
		env.mu.RLock()
		definition, exists := env.namedWindows[name]
		env.mu.RUnlock()
		if engine != nil && exists {
			engine.namedWindows[name] = newNamedWindow(definition, engine)
		}
		return FromNamedWindow(env, name)
	}
	if _, err := CreateTable(env, name, []TableColumn{
		PrimaryKeyColumn[int64]("id"),
		OptionalTableColumnOf[string]("p00"),
		OptionalTableColumnOf[string]("p10"),
		OptionalTableColumnOf[string]("theString"),
		OptionalTableColumnOf[int64]("intPrimitive"),
		OptionalTableColumnOf[string]("value"),
	}); err != nil {
		t.Fatal(err)
	}
	env.mu.RLock()
	definition, exists := env.tables[name]
	env.mu.RUnlock()
	if engine != nil && exists {
		engine.tables[name] = newTable(definition)
	}
	return FromTable(env, name)
}

func insertFAFSubqueryRow(t *testing.T, engine *Engine, namedWindow bool, name string, row map[string]any) {
	t.Helper()
	ctx := context.Background()
	if namedWindow {
		if err := engine.InsertNamedWindow(ctx, name, row); err != nil {
			t.Fatal(err)
		}
		return
	}
	table, ok := engine.Table(name)
	if !ok {
		t.Fatalf("table %q is missing", name)
	}
	if _, err := table.Upsert(ctx, row); err != nil {
		t.Fatal(err)
	}
}

func executeFAFSubquery(t *testing.T, env *Environment, engine *Engine, query Query) QueryResult {
	t.Helper()
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func resultRowsByID(t *testing.T, result QueryResult, idField, valueField string) map[int64]any {
	t.Helper()
	rows := make(map[int64]any)
	for _, item := range result.Results() {
		id, ok := item.Get(idField).Any().(int64)
		if !ok {
			t.Fatalf("result id %q = %#v", idField, item.Get(idField))
		}
		rows[id] = item.Get(valueField).Any()
	}
	return rows
}

func TestInfraFAFSubquerySimpleMatchesEsper(t *testing.T) {
	for _, namedInner := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedInner], func(t *testing.T) {
			env := NewEnvironment()
			schema := newFAFSubquerySchema(t, env, "FAFSubquerySimpleSchema")
			engine := NewEngine(env)
			outer := createFAFSubqueryStore(t, env, engine, "WinSB", schema, true, LastEvent(), "")
			inner := createFAFSubqueryStore(t, env, engine, "InfraS0", schema, namedInner, LastEvent(), "")
			query := outer.Select(Alias("c0", SubqueryValue[string](inner, Field[any, string]("p00")))).Query()

			if result := executeFAFSubquery(t, env, engine, query); len(result.Results()) != 0 {
				t.Fatalf("empty outer snapshot = %#v", result.Results())
			}
			insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": int64(1), "theString": "E1"})
			if result := executeFAFSubquery(t, env, engine, query); len(result.Results()) != 1 || !result.Results()[0].Get("c0").IsNull() {
				t.Fatalf("empty inner scalar = %#v", result.Results())
			}
			insertFAFSubqueryRow(t, engine, namedInner, "InfraS0", map[string]any{"id": int64(1), "p00": "a"})
			if result := executeFAFSubquery(t, env, engine, query); len(result.Results()) != 1 || result.Results()[0].Get("c0").Any() != "a" {
				t.Fatalf("first inner scalar = %#v", result.Results())
			}
			insertFAFSubqueryRow(t, engine, namedInner, "InfraS0", map[string]any{"id": int64(1), "p00": "b"})
			if result := executeFAFSubquery(t, env, engine, query); len(result.Results()) != 1 || result.Results()[0].Get("c0").Any() != "b" {
				t.Fatalf("last inner scalar = %#v", result.Results())
			}
		})
	}
}

func TestInfraFAFSubquerySimpleJoinMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	schema := newFAFSubquerySchema(t, env, "FAFSubqueryJoinSchema")
	engine := NewEngine(env)
	s0 := createFAFSubqueryStore(t, env, engine, "WinS0", schema, true, KeepAll(), "")
	s1 := createFAFSubqueryStore(t, env, engine, "WinS1", schema, true, KeepAll(), "")
	inner := createFAFSubqueryStore(t, env, engine, "WinSB", schema, true, LastEvent(), "")
	scalar := SubqueryValue[string](inner, Field[any, string]("theString"))
	query := JoinMany(JoinRecordSource(s0), JoinRecordSource(s1)).Select(
		SelectFrom(0, "c0", scalar),
		SelectFrom(0, "p00", JoinField[string](0, "p00")),
		SelectFrom(1, "p10", JoinField[string](1, "p10")),
	).Query()

	if result := executeFAFSubquery(t, env, engine, query); len(result.Results()) != 0 {
		t.Fatalf("empty join snapshot = %#v", result.Results())
	}
	insertFAFSubqueryRow(t, engine, true, "WinS0", map[string]any{"id": int64(1), "p00": "S0_0"})
	insertFAFSubqueryRow(t, engine, true, "WinS1", map[string]any{"id": int64(2), "p10": "S1_0"})
	if result := executeFAFSubquery(t, env, engine, query); len(result.Results()) != 1 || !result.Results()[0].Get("c0").IsNull() {
		t.Fatalf("join without reference = %#v", result.Results())
	}
	insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": int64(1), "theString": "SB_0"})
	result := executeFAFSubquery(t, env, engine, query)
	if len(result.Results()) != 1 || result.Results()[0].Get("c0").Any() != "SB_0" {
		t.Fatalf("join with reference = %#v", result.Results())
	}
	insertFAFSubqueryRow(t, engine, true, "WinS0", map[string]any{"id": int64(3), "p00": "S0_1"})
	result = executeFAFSubquery(t, env, engine, query)
	if len(result.Results()) != 2 {
		t.Fatalf("join second outer row = %#v", result.Results())
	}
}

func TestInfraFAFSubqueryInsertMatchesEsper(t *testing.T) {
	for _, namedInner := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedInner], func(t *testing.T) {
			env := NewEnvironment()
			schema := newFAFSubquerySchema(t, env, "FAFSubqueryInsertSchema")
			engine := NewEngine(env)
			target := createFAFSubqueryStore(t, env, engine, "Win", schema, true, KeepAll(), "")
			inner := createFAFSubqueryStore(t, env, engine, "InfraSB", schema, namedInner, LastEvent(), "")
			scalar := SubqueryValue[string](inner, Field[any, string]("theString"))
			plan := target.OnDemand().Insert(SetColumn("value", scalar))
			query := target.Select(Alias("value", Field[any, string]("value"))).Query()

			executeFAFSubquery(t, env, engine, plan)
			insertFAFSubqueryRow(t, engine, namedInner, "InfraSB", map[string]any{"id": int64(1)})
			executeFAFSubquery(t, env, engine, plan)
			insertFAFSubqueryRow(t, engine, namedInner, "InfraSB", map[string]any{"id": int64(1), "theString": "E1"})
			executeFAFSubquery(t, env, engine, plan)
			result := executeFAFSubquery(t, env, engine, query)
			if len(result.Results()) != 3 {
				t.Fatalf("insert result rows = %#v", result.Results())
			}
			if result.Results()[0].Get("value").Any() != nil || result.Results()[1].Get("value").Any() != nil || result.Results()[2].Get("value").Any() != "E1" {
				t.Fatalf("insert subquery values = %#v", result.Results())
			}
		})
	}
}

func TestInfraFAFSubqueryUncorrelatedMutationMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	schema := newFAFSubquerySchema(t, env, "FAFSubqueryMutationSchema")
	engine := NewEngine(env)
	target := createFAFSubqueryStore(t, env, engine, "Win", schema, true, KeepAll(), "")
	inner := createFAFSubqueryStore(t, env, engine, "WinSB", schema, true, LastEvent(), "")
	insertFAFSubqueryRow(t, engine, true, "Win", map[string]any{"id": int64(1), "intPrimitive": int64(1), "value": "one"})
	insertFAFSubqueryRow(t, engine, true, "Win", map[string]any{"id": int64(2), "intPrimitive": int64(2), "value": "two"})
	insertFAFSubqueryRow(t, engine, true, "Win", map[string]any{"id": int64(3), "intPrimitive": int64(3), "value": "three"})
	scalar := SubqueryValue[int64](inner, Field[any, int64]("intPrimitive"))

	delete := target.OnDemand().DeleteWhere(Equal[int64](NamedWindowField[int64]("intPrimitive"), scalar))
	if result := executeFAFSubquery(t, env, engine, delete); len(result.Results()) != 0 {
		t.Fatalf("delete with empty scalar = %#v", result.Results())
	}
	insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": int64(1), "intPrimitive": int64(2)})
	result := executeFAFSubquery(t, env, engine, delete)
	if len(result.Results()) != 1 || result.Results()[0].Get("intPrimitive").Any() != int64(2) {
		t.Fatalf("uncorrelated delete = %#v", result.Results())
	}

	update := target.OnDemand().UpdateWhere(Literal(true), SetColumn("intPrimitive", scalar))
	insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": int64(2), "intPrimitive": int64(10)})
	result = executeFAFSubquery(t, env, engine, update)
	if len(result.Results()) != 2 {
		t.Fatalf("uncorrelated update = %#v", result.Results())
	}
	query := target.Select(Alias("id", Field[any, int64]("id")), Alias("value", Field[any, int64]("intPrimitive"))).Query()
	rows := resultRowsByID(t, executeFAFSubquery(t, env, engine, query), "id", "value")
	if !reflect.DeepEqual(rows, map[int64]any{int64(1): int64(10), int64(3): int64(10)}) {
		t.Fatalf("uncorrelated update snapshot = %#v", rows)
	}
}

func TestInfraFAFSubquerySelectCorrelatedMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	schema := newFAFSubquerySchema(t, env, "FAFSubqueryCorrelatedSchema")
	engine := NewEngine(env)
	outer := createFAFSubqueryStore(t, env, engine, "WinS0", schema, true, KeepAll(), "")
	inner := createFAFSubqueryStore(t, env, engine, "WinSB", schema, true, Unique(Field[any, int64]("intPrimitive")), "")
	for id, p00 := range map[int64]string{1: "a", 2: "b", 3: "c"} {
		insertFAFSubqueryRow(t, engine, true, "WinS0", map[string]any{"id": id, "p00": p00})
	}
	for id, value := range map[int64]string{2: "X", 1: "Y", 3: "Z"} {
		insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": id, "intPrimitive": id, "theString": value})
	}
	scalar := SubqueryValue[string](inner, Field[any, string]("theString"), Equal[int64](Field[any, int64]("intPrimitive"), OuterField[int64]("id")))
	query := outer.Select(
		Alias("id", Field[any, int64]("id")),
		Alias("theString", scalar),
	).Query()
	result := executeFAFSubquery(t, env, engine, query)
	got := resultRowsByID(t, result, "id", "theString")
	if !reflect.DeepEqual(got, map[int64]any{1: "Y", 2: "X", 3: "Z"}) {
		t.Fatalf("correlated select = %#v", got)
	}

	for id, value := range map[int64]string{1: "Q", 3: "R", 2: "S"} {
		insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": id + 10, "intPrimitive": id, "theString": value})
	}
	result = executeFAFSubquery(t, env, engine, query)
	got = resultRowsByID(t, result, "id", "theString")
	if !reflect.DeepEqual(got, map[int64]any{1: "Q", 2: "S", 3: "R"}) {
		t.Fatalf("correlated select after replacement = %#v", got)
	}
}

func TestInfraFAFSubqueryUpdateDeleteCorrelatedMatchesEsper(t *testing.T) {
	newFixture := func(t *testing.T) (*Environment, *Engine, RecordStream, RecordStream) {
		t.Helper()
		env := NewEnvironment()
		schema := newFAFSubquerySchema(t, env, "FAFSubqueryCorrelatedMutationSchema")
		engine := NewEngine(env)
		outer := createFAFSubqueryStore(t, env, engine, "WinS0", schema, true, KeepAll(), "")
		inner := createFAFSubqueryStore(t, env, engine, "WinSB", schema, true, Unique(Field[any, int64]("intPrimitive")), "")
		for id, p00 := range map[int64]string{1: "a", 2: "b", 3: "c"} {
			insertFAFSubqueryRow(t, engine, true, "WinS0", map[string]any{"id": id, "p00": p00})
		}
		for id, value := range map[int64]string{1: "a", 2: "b"} {
			intPrimitive := id
			if id == 1 {
				intPrimitive = 0
			}
			insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": id, "intPrimitive": intPrimitive, "theString": value})
		}
		return env, engine, outer, inner
	}

	t.Run("update-set", func(t *testing.T) {
		env, engine, outer, inner := newFixture(t)
		insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": int64(11), "intPrimitive": int64(1), "theString": "X"})
		insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": int64(12), "intPrimitive": int64(2), "theString": "Y"})
		setValue := SubqueryValue[string](inner, Field[any, string]("theString"), Equal[int64](Field[any, int64]("intPrimitive"), OuterField[int64]("id")))
		plan := outer.OnDemand().UpdateWhere(Literal(true), SetColumn("p00", setValue))
		result := executeFAFSubquery(t, env, engine, plan)
		if len(result.Results()) != 3 {
			t.Fatalf("correlated update-set result = %#v", result.Results())
		}
		query := outer.Select(Alias("id", Field[any, int64]("id")), Alias("p00", Field[any, string]("p00"))).Query()
		got := resultRowsByID(t, executeFAFSubquery(t, env, engine, query), "id", "p00")
		if !reflect.DeepEqual(got, map[int64]any{1: "X", 2: "Y", 3: nil}) {
			t.Fatalf("correlated update-set snapshot = %#v", got)
		}
	})

	t.Run("update-where", func(t *testing.T) {
		env, engine, outer, inner := newFixture(t)
		match := Equal[int64](NamedWindowField[int64]("id"), SubqueryValue[int64](inner, Field[any, int64]("intPrimitive"), Equal[string](Field[any, string]("theString"), OuterField[string]("p00"))))
		plan := outer.OnDemand().UpdateWhere(match, SetColumn("p00", Literal("x")))
		result := executeFAFSubquery(t, env, engine, plan)
		if len(result.Results()) != 1 {
			t.Fatalf("correlated update-where result = %#v", result.Results())
		}
		query := outer.Select(Alias("id", Field[any, int64]("id")), Alias("p00", Field[any, string]("p00"))).Query()
		got := resultRowsByID(t, executeFAFSubquery(t, env, engine, query), "id", "p00")
		if !reflect.DeepEqual(got, map[int64]any{1: "a", 2: "x", 3: "c"}) {
			t.Fatalf("correlated update-where snapshot = %#v", got)
		}
	})

	t.Run("delete-where", func(t *testing.T) {
		env, engine, outer, inner := newFixture(t)
		match := Equal[int64](NamedWindowField[int64]("id"), SubqueryValue[int64](inner, Field[any, int64]("intPrimitive"), Equal[string](Field[any, string]("theString"), OuterField[string]("p00"))))
		plan := outer.OnDemand().DeleteWhere(match)
		result := executeFAFSubquery(t, env, engine, plan)
		if len(result.Results()) != 1 {
			t.Fatalf("correlated delete-where result = %#v", result.Results())
		}
		query := outer.Select(Alias("id", Field[any, int64]("id")), Alias("p00", Field[any, string]("p00"))).Query()
		got := resultRowsByID(t, executeFAFSubquery(t, env, engine, query), "id", "p00")
		if !reflect.DeepEqual(got, map[int64]any{1: "a", 3: "c"}) {
			t.Fatalf("correlated delete-where snapshot = %#v", got)
		}
	})
}

func TestInfraFAFSubquerySelectWhereAndGroupByMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	schema := newFAFSubquerySchema(t, env, "FAFSubqueryWhereGroupSchema")
	engine := NewEngine(env)
	outer := createFAFSubqueryStore(t, env, engine, "WinS0", schema, true, KeepAll(), "")
	inner := createFAFSubqueryStore(t, env, engine, "WinSB", schema, true, KeepAll(), "")
	insertFAFSubqueryRow(t, engine, true, "WinS0", map[string]any{"id": int64(1)})
	whereScalar := SubqueryValueWithOptions[int64](inner, Field[any, int64]("intPrimitive"),
		SubqueryWhere(Equal[string](Field[any, string]("theString"), Literal("x"))),
		SubqueryCardinalityMode(SubqueryNullOnMultiple),
	)
	query := outer.Select(Alias("c0", whereScalar)).Query()
	insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": int64(1), "theString": "E1", "intPrimitive": int64(1)})
	if result := executeFAFSubquery(t, env, engine, query); result.Results()[0].Get("c0").Any() != nil {
		t.Fatalf("where scalar before match = %#v", result.Results())
	}
	insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": int64(2), "theString": "x", "intPrimitive": int64(2)})
	if result := executeFAFSubquery(t, env, engine, query); result.Results()[0].Get("c0").Any() != int64(2) {
		t.Fatalf("where scalar match = %#v", result.Results())
	}
	insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": int64(3), "theString": "x", "intPrimitive": int64(3)})
	if result := executeFAFSubquery(t, env, engine, query); result.Results()[0].Get("c0").Any() != nil {
		t.Fatalf("where scalar multiple = %#v", result.Results())
	}

	groupRows := SubqueryGroupRows(inner, Field[any, string]("theString"), []Selection{
		Alias("theString", Field[any, string]("theString")),
		Alias("thesum", Sum[int64](Field[any, int64]("intPrimitive"))),
	})
	groupQuery := outer.Select(Alias("c0", groupRows)).Query()
	result := executeFAFSubquery(t, env, engine, groupQuery)
	if len(result.Results()) != 1 {
		t.Fatalf("grouped scalar rows = %#v", result.Results())
	}
	rows, ok := result.Results()[0].Get("c0").Any().([]map[string]any)
	if !ok || len(rows) != 2 {
		t.Fatalf("grouped scalar shape = %#v", result.Results()[0].Get("c0"))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["theString"].(string) < rows[j]["theString"].(string) })
	if !reflect.DeepEqual(rows, []map[string]any{{"theString": "E1", "thesum": int64(1)}, {"theString": "x", "thesum": int64(5)}}) {
		t.Fatalf("grouped scalar values = %#v", rows)
	}
}

func TestInfraFAFSubqueryContextMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	outerSchema := newFAFSubquerySchema(t, env, "FAFSubqueryContextOuterSchema")
	innerSchema := newFAFSubquerySchema(t, env, "FAFSubqueryContextInnerSchema")
	if _, err := CreateKeyContext(env, "MyContext", Field[any, int64]("id")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	outer := createFAFSubqueryStore(t, env, engine, "WinS0", outerSchema, true, KeepAll(), "MyContext")
	inner := createFAFSubqueryStore(t, env, engine, "WinS1", innerSchema, true, KeepAll(), "MyContext")
	for id, p00 := range map[int64]string{1: "a", 2: "b", 3: "c"} {
		insertFAFSubqueryRow(t, engine, true, "WinS0", map[string]any{"id": id, "p00": p00})
	}
	for id, p10 := range map[int64]string{1: "X", 2: "Y", 3: "Z"} {
		insertFAFSubqueryRow(t, engine, true, "WinS1", map[string]any{"id": id, "p10": p10})
	}
	scalar := SubqueryValue[string](inner, Field[any, string]("p10"))
	query := outer.Select(
		Alias("p00", Field[any, string]("p00")),
		Alias("p10", scalar),
	).Query(WithContext("MyContext"))
	result := executeFAFSubquery(t, env, engine, query)
	got := make(map[string]string)
	for _, row := range result.Results() {
		got[row.Get("p00").Any().(string)] = row.Get("p10").Any().(string)
	}
	if !reflect.DeepEqual(got, map[string]string{"a": "X", "b": "Y", "c": "Z"}) {
		t.Fatalf("context both-window scalar = %#v", got)
	}

	global := createFAFSubqueryStore(t, env, engine, "WinSB", innerSchema, true, LastEvent(), "")
	insertFAFSubqueryRow(t, engine, true, "WinSB", map[string]any{"id": int64(1), "theString": "E1"})
	globalScalar := SubqueryValue[string](global, Field[any, string]("theString"))
	globalQuery := outer.Select(Alias("p00", Field[any, string]("p00")), Alias("theString", globalScalar)).Query(WithContext("MyContext"))
	result = executeFAFSubquery(t, env, engine, globalQuery)
	if len(result.Results()) != 3 {
		t.Fatalf("context global scalar rows = %#v", result.Results())
	}
	for _, row := range result.Results() {
		if row.Get("theString").Any() != "E1" {
			t.Fatalf("context global scalar row = %#v", row)
		}
	}
}

func TestInfraFAFSubqueryInvalidMatchesEsper(t *testing.T) {
	newEnvironment := func(t *testing.T) (*Environment, *Engine, RecordStream, Schema) {
		t.Helper()
		env := NewEnvironment()
		schema := newFAFSubquerySchema(t, env, "FAFSubqueryInvalidSchema")
		engine := NewEngine(env)
		outer := createFAFSubqueryStore(t, env, engine, "WinSB", schema, true, KeepAll(), "")
		return env, engine, outer, schema
	}

	t.Run("event-stream-source", func(t *testing.T) {
		env, engine, outer, schema := newEnvironment(t)
		if _, err := RegisterMap(env, "FAFSubqueryEventSource", schema.Fields()); err != nil {
			t.Fatal(err)
		}
		inner := FromAny(env, "FAFSubqueryEventSource").Window(LastEvent())
		plan, err := env.Build(outer.Select(Alias("c0", SubqueryValue[string](inner, Field[any, string]("theString")))).Query())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.ExecuteFireAndForget(context.Background(), plan); err == nil {
			t.Fatal("expected event-stream FAF subquery to be rejected")
		}
	})

	t.Run("source-filter", func(t *testing.T) {
		env, engine, outer, schema := newEnvironment(t)
		createFAFSubqueryStore(t, env, engine, "FilteredInfra", schema, true, KeepAll(), "")
		inner := FromNamedWindow(env, "FilteredInfra").Filter(Equal[string](Field[any, string]("theString"), Literal("x")))
		plan, err := env.Build(outer.Select(Alias("c0", SubqueryValue[string](inner, Field[any, string]("theString")))).Query())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.ExecuteFireAndForget(context.Background(), plan); err == nil {
			t.Fatal("expected source-filter FAF subquery to be rejected")
		}
	})

	t.Run("context-mismatch", func(t *testing.T) {
		env, engine, outer, schema := newEnvironment(t)
		if _, err := CreateKeyContext(env, "MyContext", Field[any, int64]("id")); err != nil {
			t.Fatal(err)
		}
		createFAFSubqueryStore(t, env, engine, "PartitionedInfra", schema, true, KeepAll(), "MyContext")
		inner := FromNamedWindow(env, "PartitionedInfra")
		plan, err := env.Build(outer.Select(Alias("c0", SubqueryValue[string](inner, Field[any, string]("theString")))).Query())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.ExecuteFireAndForget(context.Background(), plan); err == nil {
			t.Fatal("expected context-mismatch FAF subquery to be rejected")
		}
	})
}
