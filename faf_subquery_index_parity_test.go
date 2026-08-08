package esper

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func newFAFSubqueryIndexSchema(t *testing.T, env *Environment, name string) Schema {
	t.Helper()
	if _, err := RegisterMap(env, name, []FieldSpec{
		FieldDef("id", typeOf[int64]()),
		FieldDef("key", typeOf[string]()),
		FieldDef("lookup", typeOf[string]()),
		FieldDef("value", typeOf[string]()),
	}); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema(name)
	if !ok {
		t.Fatalf("schema %q is missing", name)
	}
	return schema
}

// TestInfraFAFSubqueryContextIndexCandidateParity covers the Java
// InfraFAFSubqueryContextSelect/ContextBothWindows boundary with a
// correlated scalar subquery.  The outer source is context-partitioned while
// the indexed inner source is global, so the candidate lookup must preserve
// both context row semantics and OuterField correlation.
func TestInfraFAFSubqueryContextIndexCandidateParity(t *testing.T) {
	for _, namedInner := range []bool{true, false} {
		t.Run(indexStoreName(namedInner), func(t *testing.T) {
			env := NewEnvironment()
			schema := newFAFSubqueryIndexSchema(t, env, "FAFSubqueryIndexSchema")
			if _, err := CreateKeyContext(env, "faf-subquery-index-context", Field[any, string]("key")); err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			outer := createFAFSubqueryStore(t, env, engine, "FAFSubqueryIndexOuter", schema, true, KeepAll(), "faf-subquery-index-context")

			var inner RecordStream
			if namedInner {
				if _, err := CreateNamedWindow(env, "FAFSubqueryIndexInner", schema,
					NamedWindowRetention(KeepAll()), NamedWindowIndex("by-lookup", "lookup")); err != nil {
					t.Fatal(err)
				}
				inner = FromNamedWindow(env, "FAFSubqueryIndexInner")
			} else {
				if _, err := CreateTable(env, "FAFSubqueryIndexInner", []TableColumn{
					PrimaryKeyColumn[int64]("id"), TableColumnOf[string]("lookup"), TableColumnOf[string]("value"),
				}, SecondaryIndex("by-lookup", "lookup")); err != nil {
					t.Fatal(err)
				}
				env.mu.RLock()
				definition, ok := env.tables["FAFSubqueryIndexInner"]
				env.mu.RUnlock()
				if !ok {
					t.Fatal("indexed subquery table definition is missing")
				}
				engine.tables[catalogKey(definition.moduleName, definition.name)] = newTable(definition)
				inner = FromTable(env, "FAFSubqueryIndexInner")
			}

			insertInner := func(row map[string]any) {
				t.Helper()
				if namedInner {
					if err := engine.InsertNamedWindow(context.Background(), "FAFSubqueryIndexInner", row); err != nil {
						t.Fatal(err)
					}
					return
				}
				table, ok := engine.Table("FAFSubqueryIndexInner")
				if !ok {
					t.Fatal("indexed subquery table is missing")
				}
				if _, err := table.Insert(context.Background(), row); err != nil {
					t.Fatal(err)
				}
			}
			insertOuter := func(row map[string]any) {
				t.Helper()
				if err := engine.InsertNamedWindow(context.Background(), "FAFSubqueryIndexOuter", row); err != nil {
					t.Fatal(err)
				}
			}

			insertInner(map[string]any{"id": int64(101), "lookup": "A", "value": "A-hit"})
			insertInner(map[string]any{"id": int64(102), "lookup": "B", "value": "B-hit"})
			insertInner(map[string]any{"id": int64(103), "lookup": "unused", "value": "not-selected"})
			insertOuter(map[string]any{"id": int64(1), "key": "A"})
			insertOuter(map[string]any{"id": int64(2), "key": "B"})

			correlated := SubqueryValue[string](inner, Field[any, string]("value"),
				Equal[string](Field[any, string]("lookup"), OuterField[string]("key")),
			)
			plan, err := env.Build(outer.Select(
				Alias("id", Field[any, int64]("id")),
				Alias("value", correlated),
			).Query(WithContext("faf-subquery-index-context")))
			if err != nil {
				t.Fatal(err)
			}

			indexLookups := func() uint64 {
				if namedInner {
					window, ok := engine.NamedWindow("FAFSubqueryIndexInner")
					if !ok {
						t.Fatal("indexed subquery named window is missing")
					}
					return window.state.indexLookups.Load()
				}
				table, ok := engine.Table("FAFSubqueryIndexInner")
				if !ok {
					t.Fatal("indexed subquery table is missing")
				}
				return table.state.indexLookups.Load()
			}

			before := indexLookups()
			result, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{})
			if err != nil {
				t.Fatal(err)
			}
			rows := make([][]any, 0, len(result.Results()))
			for _, item := range result.Results() {
				rows = append(rows, []any{item.Get("id").Any(), item.Get("value").Any()})
			}
			if want := [][]any{{int64(1), "A-hit"}, {int64(2), "B-hit"}}; !reflect.DeepEqual(rows, want) {
				t.Fatalf("context correlated subquery rows = %#v, want %#v", rows, want)
			}
			if after := indexLookups(); after != before+2 {
				t.Fatalf("context correlated subquery expected one lookup per outer row: before=%d after=%d", before, after)
			}

			keyA := encodeKey([]any{ValuePresent, "A"})
			before = indexLookups()
			selected, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitions(keyA))
			if err != nil {
				t.Fatal(err)
			}
			if len(selected.Results()) != 1 || selected.Results()[0].Get("value").Any() != "A-hit" {
				t.Fatalf("selected context correlated subquery = %#v", selected.Results())
			}
			if after := indexLookups(); after != before+1 {
				t.Fatalf("selected context correlated subquery expected one lookup: before=%d after=%d", before, after)
			}

			parameterized := SubqueryValue[string](inner, Field[any, string]("value"),
				Equal[string](Field[any, string]("lookup"), Parameter[string]("wanted")),
			)
			parameterPlan, err := env.Build(outer.Select(
				Alias("id", Field[any, int64]("id")),
				Alias("value", parameterized),
			).Query(WithContext("faf-subquery-index-context")))
			if err != nil {
				t.Fatal(err)
			}
			before = indexLookups()
			parameterResult, err := engine.ExecuteFireAndForgetWithSelectorAndParameters(
				context.Background(), parameterPlan, ContextPartitionSelectorAll{}, ParameterValues{"wanted": "A"},
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(parameterResult.Results()) != 2 || parameterResult.Results()[0].Get("value").Any() != "A-hit" || parameterResult.Results()[1].Get("value").Any() != "A-hit" {
				t.Fatalf("parameterized context subquery rows = %#v", parameterResult.Results())
			}
			if after := indexLookups(); after != before+2 {
				t.Fatalf("parameterized context subquery expected one lookup per outer row: before=%d after=%d", before, after)
			}

			inSubquery := SubqueryValue[string](inner, Field[any, string]("value"),
				In[string](Field[any, string]("lookup"), Literal("constant"), OuterField[string]("key")),
			)
			inPlan, err := env.Build(outer.Select(
				Alias("id", Field[any, int64]("id")),
				Alias("value", inSubquery),
			).Query(WithContext("faf-subquery-index-context")))
			if err != nil {
				t.Fatal(err)
			}
			before = indexLookups()
			inResult, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), inPlan, ContextPartitionSelectorAll{})
			if err != nil {
				t.Fatal(err)
			}
			if len(inResult.Results()) != 2 || inResult.Results()[0].Get("value").Any() != "A-hit" || inResult.Results()[1].Get("value").Any() != "B-hit" {
				t.Fatalf("IN context subquery rows = %#v", inResult.Results())
			}
			if after := indexLookups(); after != before+2 {
				t.Fatalf("IN context subquery expected one lookup per outer row: before=%d after=%d", before, after)
			}

			exists := SubqueryExists(inner,
				Equal[string](Field[any, string]("lookup"), OuterField[string]("key")),
			)
			existsPlan, err := env.Build(outer.Select(
				Alias("id", Field[any, int64]("id")),
				Alias("exists", exists),
			).Query(WithContext("faf-subquery-index-context")))
			if err != nil {
				t.Fatal(err)
			}
			before = indexLookups()
			existsResult, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), existsPlan, ContextPartitionSelectorAll{})
			if err != nil {
				t.Fatal(err)
			}
			if len(existsResult.Results()) != 2 || existsResult.Results()[0].Get("exists").Any() != true || existsResult.Results()[1].Get("exists").Any() != true {
				t.Fatalf("context exists subquery rows = %#v", existsResult.Results())
			}
			if after := indexLookups(); after != before+2 {
				t.Fatalf("context exists subquery expected one lookup per outer row: before=%d after=%d", before, after)
			}

			// A dynamic function is intentionally outside the physical probe
			// contract.  The result must remain identical while the index
			// counter proves that execution returned to the complete snapshot.
			fallback := SubqueryValue[string](inner, Field[any, string]("value"),
				Equal[string](Field[any, string]("lookup"), Func1[string, string]("identity-key", func(value string) string { return value }, OuterField[string]("key"))),
			)
			fallbackPlan, err := env.Build(outer.Select(
				Alias("id", Field[any, int64]("id")),
				Alias("value", fallback),
			).Query(WithContext("faf-subquery-index-context")))
			if err != nil {
				t.Fatal(err)
			}
			before = indexLookups()
			fallbackResult, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), fallbackPlan, ContextPartitionSelectorAll{})
			if err != nil {
				t.Fatal(err)
			}
			if len(fallbackResult.Results()) != 2 || fallbackResult.Results()[0].Get("value").Any() != "A-hit" || fallbackResult.Results()[1].Get("value").Any() != "B-hit" {
				t.Fatalf("context correlated fallback rows = %#v", fallbackResult.Results())
			}
			if after := indexLookups(); after != before {
				t.Fatalf("dynamic correlated subquery unexpectedly used the index: before=%d after=%d", before, after)
			}
		})
	}
}

func TestInfraFAFSubqueryContextBoundIndexCandidatePartitionParity(t *testing.T) {
	env := NewEnvironment()
	schema := newFAFSubqueryIndexSchema(t, env, "FAFSubqueryContextBoundIndexSchema")
	if _, err := CreateKeyContext(env, "faf-subquery-context-index", Field[any, string]("key")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	outer := createFAFSubqueryStore(t, env, engine, "FAFSubqueryContextOuter", schema, true, KeepAll(), "faf-subquery-context-index")
	if _, err := CreateNamedWindow(env, "FAFSubqueryContextInner", schema,
		NamedWindowRetention(KeepAll()),
		NamedWindowContext("faf-subquery-context-index"),
		NamedWindowIndex("by-lookup", "lookup")); err != nil {
		t.Fatal(err)
	}
	inner := FromNamedWindow(env, "FAFSubqueryContextInner")
	insert := func(name string, row map[string]any) {
		t.Helper()
		if err := engine.InsertNamedWindow(context.Background(), name, row); err != nil {
			t.Fatal(err)
		}
	}
	insert("FAFSubqueryContextInner", map[string]any{"id": int64(101), "key": "A", "lookup": "A", "value": "A-local"})
	insert("FAFSubqueryContextInner", map[string]any{"id": int64(102), "key": "B", "lookup": "B", "value": "B-local"})
	insert("FAFSubqueryContextInner", map[string]any{"id": int64(103), "key": "C", "lookup": "C", "value": "wrong-partition"})
	insert("FAFSubqueryContextOuter", map[string]any{"id": int64(1), "key": "A"})
	insert("FAFSubqueryContextOuter", map[string]any{"id": int64(2), "key": "B"})

	scalar := SubqueryValue[string](inner, Field[any, string]("value"),
		Equal[string](Field[any, string]("lookup"), OuterField[string]("key")),
	)
	plan, err := env.Build(outer.Select(
		Alias("id", Field[any, int64]("id")),
		Alias("value", scalar),
	).Query(WithContext("faf-subquery-context-index")))
	if err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("FAFSubqueryContextInner")
	if !ok {
		t.Fatal("context-bound indexed subquery window is missing")
	}
	indexLookups := func() uint64 {
		var total uint64
		for _, partition := range window.contextPartitionStates() {
			total += partition.indexLookups.Load()
		}
		return total
	}
	before := indexLookups()
	result, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	rows := make([][]any, 0, len(result.Results()))
	for _, item := range result.Results() {
		rows = append(rows, []any{item.Get("id").Any(), item.Get("value").Any()})
	}
	if want := [][]any{{int64(1), "A-local"}, {int64(2), "B-local"}}; !reflect.DeepEqual(rows, want) {
		t.Fatalf("context-bound correlated subquery rows = %#v, want %#v", rows, want)
	}
	if after := indexLookups(); after != before+2 {
		t.Fatalf("context-bound correlated subquery expected one partition lookup per outer row: before=%d after=%d", before, after)
	}

	keyA := encodeKey([]any{ValuePresent, "A"})
	before = indexLookups()
	selected, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitions(keyA))
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.Results()) != 1 || selected.Results()[0].Get("value").Any() != "A-local" {
		t.Fatalf("selected context-bound correlated subquery = %#v", selected.Results())
	}
	if after := indexLookups(); after != before+1 {
		t.Fatalf("selected context-bound correlated subquery expected one lookup: before=%d after=%d", before, after)
	}
}

// TestInfraFAFSubqueryContextScopedTableIndexCandidateParity covers the
// Context-table counterpart of the Java context subquery cases. The inner
// Table has the same primary key in two Context partitions, so a global Table
// lookup would either reject the second write or return the wrong partition's
// value. The correlated FAF subquery must probe the current scoped index.
func TestInfraFAFSubqueryContextScopedTableIndexCandidateParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[infraContextIndexEvent](env, "FAFSubqueryScopedTableEvent")
	if err != nil {
		t.Fatal(err)
	}
	const contextName = "faf-subquery-scoped-table"
	if _, err := CreateKeyContext(env, contextName, Field[any, string]("key")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "FAFSubqueryScopedTableOuter", schema,
		NamedWindowRetention(KeepAll()), NamedWindowContext(contextName)); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "FAFSubqueryScopedTableInner", []TableColumn{
		PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"), TableColumnOf[int64]("value"),
	}, SecondaryIndex("by-key", "key")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	ctx := context.Background()
	source := From[infraContextIndexEvent](env, "FAFSubqueryScopedTableEvent")
	insertPlan, err := env.Build(OnRecord(source.AsRecord()).InsertIntoTable("FAFSubqueryScopedTableInner",
		SetColumn("id", Field[any, string]("id")),
		SetColumn("key", Field[any, string]("key")),
		SetColumn("value", Field[any, int64]("value")),
	).Query(StatementName("faf-subquery-scoped-table-insert"), WithContext(contextName)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(ctx, insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []infraContextIndexEvent{
		{ID: "same", Key: "A", Value: 10},
		{ID: "same", Key: "B", Value: 20},
	} {
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	if err := deployment.Undeploy(ctx); err != nil {
		t.Fatal(err)
	}
	for _, event := range []infraContextIndexEvent{
		{ID: "outer-a", Key: "A"},
		{ID: "outer-b", Key: "B"},
	} {
		if err := engine.InsertNamedWindow(ctx, "FAFSubqueryScopedTableOuter", event); err != nil {
			t.Fatal(err)
		}
	}

	inner := FromTable(env, "FAFSubqueryScopedTableInner")
	correlated := SubqueryValue[int64](inner, Field[any, int64]("value"),
		Equal[string](Field[any, string]("key"), OuterField[string]("key")),
	)
	plan, err := env.Build(FromNamedWindow(env, "FAFSubqueryScopedTableOuter").Select(
		Alias("id", Field[any, string]("id")),
		Alias("value", correlated),
	).Query(WithContext(contextName)))
	if err != nil {
		t.Fatal(err)
	}
	table, ok := engine.Table("FAFSubqueryScopedTableInner")
	if !ok {
		t.Fatal("scoped subquery table is missing")
	}
	indexLookups := func() uint64 {
		total := table.state.indexLookups.Load()
		table.scopesMu.RLock()
		for _, state := range table.scopedState {
			total += state.indexLookups.Load()
		}
		table.scopesMu.RUnlock()
		return total
	}
	before := indexLookups()
	result, err := engine.ExecuteFireAndForgetWithSelector(ctx, plan, ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]int64, len(result.Results()))
	for _, row := range result.Results() {
		got[row.Get("id").Any().(string)] = row.Get("value").Any().(int64)
	}
	want := map[string]int64{"outer-a": 10, "outer-b": 20}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scoped table correlated subquery rows = %#v, want %#v", got, want)
	}
	if after := indexLookups(); after != before+2 {
		t.Fatalf("scoped table correlated subquery expected one scoped lookup per outer row: before=%d after=%d", before, after)
	}

	// A root row may have been written through the public Table API before a
	// context statement was deployed. The scoped candidate path must give way
	// to the ownership-aware snapshot path rather than silently omitting it.
	if _, err := table.Upsert(ctx, map[string]any{"id": "legacy", "key": "C", "value": int64(99)}); err != nil {
		t.Fatal(err)
	}
	before = indexLookups()
	legacyResult, err := engine.ExecuteFireAndForgetWithSelector(ctx, plan, ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	legacyGot := make(map[string]int64, len(legacyResult.Results()))
	for _, row := range legacyResult.Results() {
		legacyGot[row.Get("id").Any().(string)] = row.Get("value").Any().(int64)
	}
	if !reflect.DeepEqual(legacyGot, want) {
		t.Fatalf("legacy-root scoped subquery rows = %#v, want %#v", legacyGot, want)
	}
	if after := indexLookups(); after != before {
		t.Fatalf("legacy-root scoped subquery unexpectedly used the scoped index: before=%d after=%d", before, after)
	}
}

// TestInfraFAFSubqueryContextScopedTableRangeIndexCandidateParity is the
// ordered-index counterpart. It exercises an equality prefix plus a
// correlated range against each partition-local B-tree.
func TestInfraFAFSubqueryContextScopedTableRangeIndexCandidateParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[infraContextIndexEvent](env, "FAFSubqueryScopedTableRangeEvent")
	if err != nil {
		t.Fatal(err)
	}
	const contextName = "faf-subquery-scoped-table-range"
	if _, err := CreateKeyContext(env, contextName, Field[any, string]("key")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "FAFSubqueryScopedTableRangeOuter", schema,
		NamedWindowRetention(KeepAll()), NamedWindowContext(contextName)); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "FAFSubqueryScopedTableRangeInner", []TableColumn{
		PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"), TableColumnOf[int64]("value"),
	}, SecondaryBTreeIndex("by-key-value", "key", "value")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	ctx := context.Background()
	source := From[infraContextIndexEvent](env, "FAFSubqueryScopedTableRangeEvent")
	insertPlan, err := env.Build(OnRecord(source.AsRecord()).InsertIntoTable("FAFSubqueryScopedTableRangeInner",
		SetColumn("id", Field[any, string]("id")),
		SetColumn("key", Field[any, string]("key")),
		SetColumn("value", Field[any, int64]("value")),
	).Query(StatementName("faf-subquery-scoped-table-range-insert"), WithContext(contextName)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(ctx, insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []infraContextIndexEvent{
		{ID: "a-10", Key: "A", Value: 10},
		{ID: "a-11", Key: "A", Value: 11},
		{ID: "a-12", Key: "A", Value: 12},
		{ID: "b-20", Key: "B", Value: 20},
		{ID: "b-21", Key: "B", Value: 21},
	} {
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	if err := deployment.Undeploy(ctx); err != nil {
		t.Fatal(err)
	}
	for _, event := range []infraContextIndexEvent{
		{ID: "outer-a", Key: "A", Value: 11},
		{ID: "outer-b", Key: "B", Value: 21},
	} {
		if err := engine.InsertNamedWindow(ctx, "FAFSubqueryScopedTableRangeOuter", event); err != nil {
			t.Fatal(err)
		}
	}

	inner := FromTable(env, "FAFSubqueryScopedTableRangeInner")
	values := SubqueryValues[int64](inner, Field[any, int64]("value"), SubqueryWhere(And(
		Equal[string](Field[any, string]("key"), OuterField[string]("key")),
		GreaterOrEqual[int64](Field[any, int64]("value"), OuterField[int64]("value")),
	)))
	plan, err := env.Build(FromNamedWindow(env, "FAFSubqueryScopedTableRangeOuter").Select(
		Alias("id", Field[any, string]("id")),
		Alias("values", values),
	).Query(WithContext(contextName)))
	if err != nil {
		t.Fatal(err)
	}
	table, ok := engine.Table("FAFSubqueryScopedTableRangeInner")
	if !ok {
		t.Fatal("scoped range subquery table is missing")
	}
	indexLookups := func() uint64 {
		total := table.state.indexLookups.Load()
		table.scopesMu.RLock()
		for _, state := range table.scopedState {
			total += state.indexLookups.Load()
		}
		table.scopesMu.RUnlock()
		return total
	}
	before := indexLookups()
	result, err := engine.ExecuteFireAndForgetWithSelector(ctx, plan, ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]int64{"outer-a": {11, 12}, "outer-b": {21}}
	if len(result.Results()) != len(want) {
		t.Fatalf("scoped table range result = %#v, want %#v", result.Results(), want)
	}
	for _, row := range result.Results() {
		id := row.Get("id").Any().(string)
		values, ok := row.Get("values").Any().([]int64)
		if !ok || !reflect.DeepEqual(values, want[id]) {
			t.Fatalf("scoped table range row = %#v, want id=%s values=%#v", row, id, want[id])
		}
	}
	if after := indexLookups(); after != before+2 {
		t.Fatalf("scoped table range subquery expected one B-tree lookup per outer row: before=%d after=%d", before, after)
	}
}

// TestInfraFAFSubqueryContextRangeIndexCandidateParity closes the B-tree
// counterpart of the equality candidate path.  The index has an equality
// prefix (lookup) followed by a correlated range (id); both operands are
// fixed for the current outer row and the final subquery evaluator still
// owns predicate and collection semantics.
func TestInfraFAFSubqueryContextRangeIndexCandidateParity(t *testing.T) {
	for _, namedInner := range []bool{true, false} {
		t.Run(indexStoreName(namedInner), func(t *testing.T) {
			env := NewEnvironment()
			schema := newFAFSubqueryIndexSchema(t, env, "FAFSubqueryRangeSchema")
			if _, err := CreateKeyContext(env, "faf-subquery-range-context", Field[any, string]("key")); err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			outer := createFAFSubqueryStore(t, env, engine, "FAFSubqueryRangeOuter", schema, true, KeepAll(), "faf-subquery-range-context")

			var inner RecordStream
			if namedInner {
				if _, err := CreateNamedWindow(env, "FAFSubqueryRangeInner", schema,
					NamedWindowRetention(KeepAll()), NamedWindowBTreeIndex("by-lookup-id", "lookup", "id")); err != nil {
					t.Fatal(err)
				}
				inner = FromNamedWindow(env, "FAFSubqueryRangeInner")
			} else {
				if _, err := CreateTable(env, "FAFSubqueryRangeInner", []TableColumn{
					PrimaryKeyColumn[int64]("id"), TableColumnOf[string]("lookup"), TableColumnOf[string]("value"),
				}, SecondaryBTreeIndex("by-lookup-id", "lookup", "id")); err != nil {
					t.Fatal(err)
				}
				env.mu.RLock()
				definition, ok := env.tables["FAFSubqueryRangeInner"]
				env.mu.RUnlock()
				if !ok {
					t.Fatal("range-indexed subquery table definition is missing")
				}
				engine.tables[catalogKey(definition.moduleName, definition.name)] = newTable(definition)
				inner = FromTable(env, "FAFSubqueryRangeInner")
			}

			insertInner := func(row map[string]any) {
				t.Helper()
				if namedInner {
					if err := engine.InsertNamedWindow(context.Background(), "FAFSubqueryRangeInner", row); err != nil {
						t.Fatal(err)
					}
					return
				}
				table, ok := engine.Table("FAFSubqueryRangeInner")
				if !ok {
					t.Fatal("range-indexed subquery table is missing")
				}
				if _, err := table.Insert(context.Background(), row); err != nil {
					t.Fatal(err)
				}
			}
			insertOuter := func(row map[string]any) {
				t.Helper()
				if err := engine.InsertNamedWindow(context.Background(), "FAFSubqueryRangeOuter", row); err != nil {
					t.Fatal(err)
				}
			}

			insertInner(map[string]any{"id": int64(10), "lookup": "A", "value": "A-10"})
			insertInner(map[string]any{"id": int64(11), "lookup": "A", "value": "A-11"})
			insertInner(map[string]any{"id": int64(12), "lookup": "A", "value": "A-12"})
			insertInner(map[string]any{"id": int64(20), "lookup": "B", "value": "B-20"})
			insertInner(map[string]any{"id": int64(21), "lookup": "B", "value": "B-21"})
			insertOuter(map[string]any{"id": int64(11), "key": "A"})
			insertOuter(map[string]any{"id": int64(20), "key": "B"})

			predicate := And(
				Equal[string](Field[any, string]("lookup"), OuterField[string]("key")),
				GreaterOrEqual[int64](Field[any, int64]("id"), OuterField[int64]("id")),
			)
			values := SubqueryValues[string](inner, Field[any, string]("value"), SubqueryWhere(predicate))
			plan, err := env.Build(outer.Select(
				Alias("id", Field[any, int64]("id")),
				Alias("values", values),
			).Query(WithContext("faf-subquery-range-context")))
			if err != nil {
				t.Fatal(err)
			}

			indexLookups := func() uint64 {
				if namedInner {
					window, ok := engine.NamedWindow("FAFSubqueryRangeInner")
					if !ok {
						t.Fatal("range-indexed subquery named window is missing")
					}
					return window.state.indexLookups.Load()
				}
				table, ok := engine.Table("FAFSubqueryRangeInner")
				if !ok {
					t.Fatal("range-indexed subquery table is missing")
				}
				return table.state.indexLookups.Load()
			}

			before := indexLookups()
			result, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Results()) != 2 {
				t.Fatalf("range correlated subquery rows = %#v", result.Results())
			}
			want := []struct {
				id     int64
				values []string
			}{
				{11, []string{"A-11", "A-12"}},
				{20, []string{"B-20", "B-21"}},
			}
			for index, row := range result.Results() {
				gotValues, ok := row.Get("values").Any().([]string)
				if !ok || row.Get("id").Any() != want[index].id || !reflect.DeepEqual(gotValues, want[index].values) {
					t.Fatalf("range correlated row %d = %#v, want id=%d values=%#v", index, row, want[index].id, want[index].values)
				}
			}
			if after := indexLookups(); after != before+2 {
				t.Fatalf("range correlated subquery expected one B-tree lookup per outer row: before=%d after=%d", before, after)
			}

			parameterized := SubqueryValues[string](inner, Field[any, string]("value"), SubqueryWhere(And(
				Equal[string](Field[any, string]("lookup"), Parameter[string]("wanted")),
				GreaterOrEqual[int64](Field[any, int64]("id"), Parameter[int64]("minimum")),
			)))
			parameterPlan, err := env.Build(outer.Select(
				Alias("id", Field[any, int64]("id")),
				Alias("values", parameterized),
			).Query(WithContext("faf-subquery-range-context")))
			if err != nil {
				t.Fatal(err)
			}
			before = indexLookups()
			parameterResult, err := engine.ExecuteFireAndForgetWithSelectorAndParameters(
				context.Background(), parameterPlan, ContextPartitionSelectorAll{},
				ParameterValues{"wanted": "A", "minimum": int64(11)},
			)
			if err != nil {
				t.Fatal(err)
			}
			for index, row := range parameterResult.Results() {
				gotValues, ok := row.Get("values").Any().([]string)
				if !ok || !reflect.DeepEqual(gotValues, []string{"A-11", "A-12"}) {
					t.Fatalf("parameterized range row %d = %#v", index, row)
				}
			}
			if after := indexLookups(); after != before+2 {
				t.Fatalf("parameterized range subquery expected one lookup per outer row: before=%d after=%d", before, after)
			}
		})
	}
}

func TestInfraFAFSubqueryContextBoundRangeIndexCandidatePartitionParity(t *testing.T) {
	env := NewEnvironment()
	schema := newFAFSubqueryIndexSchema(t, env, "FAFSubqueryContextBoundRangeSchema")
	if _, err := CreateKeyContext(env, "faf-subquery-context-bound-range", Field[any, string]("key")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	outer := createFAFSubqueryStore(t, env, engine, "FAFSubqueryContextBoundRangeOuter", schema, true, KeepAll(), "faf-subquery-context-bound-range")
	if _, err := CreateNamedWindow(env, "FAFSubqueryContextBoundRangeInner", schema,
		NamedWindowRetention(KeepAll()),
		NamedWindowContext("faf-subquery-context-bound-range"),
		NamedWindowBTreeIndex("by-key-id", "key", "id")); err != nil {
		t.Fatal(err)
	}
	inner := FromNamedWindow(env, "FAFSubqueryContextBoundRangeInner")
	insert := func(name string, row map[string]any) {
		t.Helper()
		if err := engine.InsertNamedWindow(context.Background(), name, row); err != nil {
			t.Fatal(err)
		}
	}
	insert("FAFSubqueryContextBoundRangeInner", map[string]any{"id": int64(10), "key": "A", "value": "A-10"})
	insert("FAFSubqueryContextBoundRangeInner", map[string]any{"id": int64(11), "key": "A", "value": "A-11"})
	insert("FAFSubqueryContextBoundRangeInner", map[string]any{"id": int64(20), "key": "B", "value": "B-20"})
	insert("FAFSubqueryContextBoundRangeInner", map[string]any{"id": int64(21), "key": "B", "value": "B-21"})
	insert("FAFSubqueryContextBoundRangeOuter", map[string]any{"id": int64(11), "key": "A"})
	insert("FAFSubqueryContextBoundRangeOuter", map[string]any{"id": int64(21), "key": "B"})

	values := SubqueryValues[string](inner, Field[any, string]("value"), SubqueryWhere(And(
		Equal[string](Field[any, string]("key"), OuterField[string]("key")),
		GreaterOrEqual[int64](Field[any, int64]("id"), OuterField[int64]("id")),
	)))
	plan, err := env.Build(outer.Select(
		Alias("id", Field[any, int64]("id")),
		Alias("values", values),
	).Query(WithContext("faf-subquery-context-bound-range")))
	if err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("FAFSubqueryContextBoundRangeInner")
	if !ok {
		t.Fatal("context-bound range subquery window is missing")
	}
	indexLookups := func() uint64 {
		var total uint64
		for _, partition := range window.contextPartitionStates() {
			total += partition.indexLookups.Load()
		}
		return total
	}
	before := indexLookups()
	result, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"A-11"}, {"B-21"}}
	if len(result.Results()) != len(want) {
		t.Fatalf("context-bound range rows = %#v", result.Results())
	}
	for index, row := range result.Results() {
		gotValues, ok := row.Get("values").Any().([]string)
		if !ok || !reflect.DeepEqual(gotValues, want[index]) {
			t.Fatalf("context-bound range row %d = %#v, want %#v", index, row, want[index])
		}
	}
	if after := indexLookups(); after != before+2 {
		t.Fatalf("context-bound range subquery expected one partition lookup per outer row: before=%d after=%d", before, after)
	}

	keyA := encodeKey([]any{ValuePresent, "A"})
	before = indexLookups()
	selected, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitions(keyA))
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.Results()) != 1 {
		t.Fatalf("selected context-bound range rows = %#v", selected.Results())
	}
	if gotValues, ok := selected.Results()[0].Get("values").Any().([]string); !ok || !reflect.DeepEqual(gotValues, []string{"A-11"}) {
		t.Fatalf("selected context-bound range values = %#v", selected.Results()[0])
	}
	if after := indexLookups(); after != before+1 {
		t.Fatalf("selected context-bound range expected one partition lookup: before=%d after=%d", before, after)
	}
}

// TestInfraNWTableSubqCorrelIndexSharingParity covers the equality/hash
// subquery-index-sharing slice of Java InfraNWTableSubqCorrelIndex. The same
// fluent subquery is exercised with automatic sharing, consumer-side sharing
// disable, set-no-index, an explicit declared index, and no sharing at all.
func TestInfraNWTableSubqCorrelIndexSharingParity(t *testing.T) {
	type testCase struct {
		name          string
		share         bool
		declaredIndex bool
		option        SubqueryOption
		wantLookup    bool
	}
	cases := []testCase{
		{name: "share", share: true, wantLookup: true},
		{name: "no-share", wantLookup: false},
		{name: "disable-consumer-share", share: true, option: SubqueryDisableIndexSharing(), wantLookup: false},
		{name: "set-noindex", share: true, option: SubqueryNoIndex(), wantLookup: false},
		{name: "explicit-index", declaredIndex: true, option: SubqueryUseIndex("by-lookup"), wantLookup: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := NewEnvironment()
			schema := newFAFSubqueryIndexSchema(t, env, "NWSubqSharingSchema")
			outerName := "NWSubqSharingOuter"
			innerName := "NWSubqSharingInner"
			if _, err := CreateNamedWindow(env, outerName, schema, NamedWindowRetention(KeepAll())); err != nil {
				t.Fatal(err)
			}
			innerOptions := []NamedWindowOption{NamedWindowRetention(Unique(Field[any, string]("lookup")))}
			if tc.share {
				innerOptions = append(innerOptions, NamedWindowSubqueryIndexSharing())
			}
			if tc.declaredIndex {
				innerOptions = append(innerOptions, NamedWindowIndex("by-lookup", "lookup"))
			}
			if _, err := CreateNamedWindow(env, innerName, schema, innerOptions...); err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			ctx := context.Background()
			for _, row := range []map[string]any{
				{"id": int64(101), "key": "A", "lookup": "A", "value": "A-hit"},
				{"id": int64(102), "key": "B", "lookup": "B", "value": "B-hit"},
				{"id": int64(103), "key": "X", "lookup": "unused", "value": "not-selected"},
			} {
				if err := engine.InsertNamedWindow(ctx, innerName, row); err != nil {
					t.Fatal(err)
				}
			}
			for _, row := range []map[string]any{
				{"id": int64(1), "key": "A"},
				{"id": int64(2), "key": "B"},
			} {
				if err := engine.InsertNamedWindow(ctx, outerName, row); err != nil {
					t.Fatal(err)
				}
			}

			inner := FromNamedWindow(env, innerName)
			where := Equal[string](Field[any, string]("lookup"), OuterField[string]("key"))
			options := []SubqueryOption{SubqueryWhere(where)}
			if tc.option != nil {
				options = append(options, tc.option)
			}
			first := SubqueryValueWithOptions[string](inner, Field[any, string]("value"), options...)
			second := SubqueryValueWithOptions[string](inner, Field[any, string]("value"), options...)
			plan, err := env.Build(FromNamedWindow(env, outerName).Select(
				Alias("id", Field[any, int64]("id")),
				Alias("first", first),
				Alias("second", second),
			).Query())
			if err != nil {
				t.Fatal(err)
			}
			window, ok := engine.NamedWindow(innerName)
			if !ok {
				t.Fatal("shared-index inner named window is missing")
			}
			before := window.state.indexLookups.Load()
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Results()) != 2 {
				t.Fatalf("shared-index result count = %d, want 2", len(result.Results()))
			}
			for index, want := range []struct {
				id    int64
				value string
			}{
				{id: 1, value: "A-hit"},
				{id: 2, value: "B-hit"},
			} {
				row := result.Results()[index]
				if row.Get("id").Any() != want.id || row.Get("first").Any() != want.value || row.Get("second").Any() != want.value {
					t.Fatalf("shared-index row %d = %#v, want id=%d value=%q", index, row, want.id, want.value)
				}
			}
			after := window.state.indexLookups.Load()
			if tc.wantLookup {
				if after != before+4 {
					t.Fatalf("shared-index expected two lookups per outer row: before=%d after=%d", before, after)
				}
			} else if after != before {
				t.Fatalf("subquery unexpectedly used an index: before=%d after=%d", before, after)
			}
			internalIndexes := 0
			for _, index := range window.state.def.indexes {
				if isSubquerySharedIndex(index.Name) {
					internalIndexes++
				}
			}
			wantInternal := 0
			if tc.name == "share" {
				wantInternal = 1
			}
			if internalIndexes != wantInternal {
				t.Fatalf("shared-index definitions = %d, want %d", internalIndexes, wantInternal)
			}
		})
	}
}

func TestInfraNWTableSubqCorrelIndexSharingContextPartitionParity(t *testing.T) {
	env := NewEnvironment()
	schema := newFAFSubqueryIndexSchema(t, env, "NWSubqSharingContextSchema")
	const contextName = "nw-subq-sharing-context"
	if _, err := CreateKeyContext(env, contextName, Field[any, string]("key")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "NWSubqSharingContextOuter", schema,
		NamedWindowRetention(KeepAll()), NamedWindowContext(contextName)); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "NWSubqSharingContextInner", schema,
		NamedWindowRetention(KeepAll()), NamedWindowContext(contextName), NamedWindowSubqueryIndexSharing()); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	ctx := context.Background()
	for _, row := range []map[string]any{
		{"id": int64(101), "key": "A", "lookup": "A", "value": "A-local"},
		{"id": int64(102), "key": "B", "lookup": "B", "value": "B-local"},
	} {
		if err := engine.InsertNamedWindow(ctx, "NWSubqSharingContextInner", row); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []map[string]any{
		{"id": int64(1), "key": "A"},
		{"id": int64(2), "key": "B"},
	} {
		if err := engine.InsertNamedWindow(ctx, "NWSubqSharingContextOuter", row); err != nil {
			t.Fatal(err)
		}
	}
	inner := FromNamedWindow(env, "NWSubqSharingContextInner")
	plan, err := env.Build(FromNamedWindow(env, "NWSubqSharingContextOuter").Select(
		Alias("id", Field[any, int64]("id")),
		Alias("value", SubqueryValueWithOptions[string](inner, Field[any, string]("value"),
			SubqueryWhere(Equal[string](Field[any, string]("lookup"), OuterField[string]("key"))),
		)),
	).Query(WithContext(contextName)))
	if err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("NWSubqSharingContextInner")
	if !ok {
		t.Fatal("context shared-index named window is missing")
	}
	indexLookups := func() uint64 {
		var total uint64
		for _, partition := range window.contextPartitionStates() {
			total += partition.indexLookups.Load()
		}
		return total
	}
	before := indexLookups()
	result, err := engine.ExecuteFireAndForgetWithSelector(ctx, plan, ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[int64]string, len(result.Results()))
	for _, row := range result.Results() {
		got[row.Get("id").Any().(int64)] = row.Get("value").Any().(string)
	}
	want := map[int64]string{1: "A-local", 2: "B-local"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("context shared-index rows = %#v, want %#v", got, want)
	}
	if after := indexLookups(); after != before+2 {
		t.Fatalf("context shared-index expected one lookup per partition row: before=%d after=%d", before, after)
	}
	for _, partition := range window.contextPartitionStates() {
		found := false
		for _, index := range partition.def.indexes {
			if isSubquerySharedIndex(index.Name) && index.Kind == IndexHash && reflect.DeepEqual(index.Columns, []string{"lookup"}) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("context partition %q did not inherit the shared hash index", partition.contextKey)
		}
	}

	// The same physical index must be rebuilt after update/delete mutations.
	if _, err := window.UpdateWhere(ctx, func(event Event) bool {
		return event.Get("lookup").Any() == "A"
	}, func(event Event) (any, error) {
		return map[string]any{
			"id":     event.Get("id").Any(),
			"key":    event.Get("key").Any(),
			"lookup": event.Get("lookup").Any(),
			"value":  "A-updated",
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	updated, err := engine.ExecuteFireAndForgetWithSelector(ctx, plan, ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	updatedValues := make(map[int64]string, len(updated.Results()))
	for _, row := range updated.Results() {
		updatedValues[row.Get("id").Any().(int64)] = row.Get("value").Any().(string)
	}
	if want := map[int64]string{1: "A-updated", 2: "B-local"}; !reflect.DeepEqual(updatedValues, want) {
		t.Fatalf("updated context shared-index rows = %#v, want %#v", updatedValues, want)
	}
	if _, err := window.DeleteWhere(ctx, func(event Event) bool {
		return event.Get("lookup").Any() == "A"
	}); err != nil {
		t.Fatal(err)
	}
	deleted, err := engine.ExecuteFireAndForgetWithSelector(ctx, plan, ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range deleted.Results() {
		if row.Get("id").Any().(int64) == 1 && row.Get("value").IsPresent() {
			t.Fatalf("deleted context shared-index row still matched: %#v", row)
		}
	}
}

// TestInfraNWTableSubqCorrelIndexSharingBTreeParity covers the ordered
// shared-index slice of Java InfraNWTableSubqCorrelIndex.  The generated
// access path is canonicalized as equality prefix + range column, while the
// final subquery evaluator still owns predicate and collection semantics.
func TestInfraNWTableSubqCorrelIndexSharingBTreeParity(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		name := "global"
		if scoped {
			name = "context"
		}
		t.Run(name, func(t *testing.T) {
			env := NewEnvironment()
			schema := newFAFSubqueryIndexSchema(t, env, "NWSubqSharingBTreeSchema"+name)
			const contextName = "nw-subq-sharing-btree-context"
			if scoped {
				if _, err := CreateKeyContext(env, contextName, Field[any, string]("key")); err != nil {
					t.Fatal(err)
				}
			}
			outerOptions := []NamedWindowOption{NamedWindowRetention(KeepAll())}
			innerOptions := []NamedWindowOption{NamedWindowRetention(KeepAll()), NamedWindowSubqueryIndexSharing()}
			if scoped {
				outerOptions = append(outerOptions, NamedWindowContext(contextName))
				innerOptions = append(innerOptions, NamedWindowContext(contextName))
			}
			outerName := "NWSubqSharingBTreeOuter" + name
			innerName := "NWSubqSharingBTreeInner" + name
			if _, err := CreateNamedWindow(env, outerName, schema, outerOptions...); err != nil {
				t.Fatal(err)
			}
			if _, err := CreateNamedWindow(env, innerName, schema, innerOptions...); err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			ctx := context.Background()
			for _, row := range []map[string]any{
				{"id": int64(12), "key": "A", "lookup": "A", "value": "A-12"},
				{"id": int64(11), "key": "A", "lookup": "A", "value": "A-11"},
				{"id": int64(20), "key": "B", "lookup": "B", "value": "B-20"},
				{"id": int64(21), "key": "B", "lookup": "B", "value": "B-21"},
			} {
				if err := engine.InsertNamedWindow(ctx, innerName, row); err != nil {
					t.Fatal(err)
				}
			}
			for _, row := range []map[string]any{
				{"id": int64(11), "key": "A"},
				{"id": int64(20), "key": "B"},
			} {
				if err := engine.InsertNamedWindow(ctx, outerName, row); err != nil {
					t.Fatal(err)
				}
			}

			inner := FromNamedWindow(env, innerName)
			predicate := And(
				Equal[string](Field[any, string]("lookup"), OuterField[string]("key")),
				GreaterOrEqual[int64](Field[any, int64]("id"), OuterField[int64]("id")),
			)
			values := SubqueryValues[string](inner, Field[any, string]("value"), SubqueryWhere(predicate))
			queryBuilder := FromNamedWindow(env, outerName).Select(
				Alias("id", Field[any, int64]("id")),
				Alias("values", values),
			)
			var query Query
			if scoped {
				query = queryBuilder.Query(WithContext(contextName))
			} else {
				query = queryBuilder.Query()
			}
			var plan Plan
			var err error
			plan, err = env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			window, ok := engine.NamedWindow(innerName)
			if !ok {
				t.Fatal("B-tree shared-index named window is missing")
			}
			indexLookups := func() uint64 {
				if !scoped {
					return window.state.indexLookups.Load()
				}
				var total uint64
				for _, partition := range window.contextPartitionStates() {
					total += partition.indexLookups.Load()
				}
				return total
			}
			execute := func() QueryResult {
				t.Helper()
				if scoped {
					result, executeErr := engine.ExecuteFireAndForgetWithSelector(ctx, plan, ContextPartitionSelectorAll{})
					if executeErr != nil {
						t.Fatal(executeErr)
					}
					return result
				}
				result, executeErr := engine.ExecuteFireAndForget(ctx, plan)
				if executeErr != nil {
					t.Fatal(executeErr)
				}
				return result
			}
			assertValues := func(result QueryResult, want map[int64][]string) {
				t.Helper()
				if len(result.Results()) != len(want) {
					t.Fatalf("B-tree shared-index result count = %d, want %d: %#v", len(result.Results()), len(want), result.Results())
				}
				for _, row := range result.Results() {
					id, ok := row.Get("id").Any().(int64)
					if !ok {
						t.Fatalf("B-tree shared-index row id = %#v", row)
					}
					values, ok := row.Get("values").Any().([]string)
					if !ok || !reflect.DeepEqual(values, want[id]) {
						t.Fatalf("B-tree shared-index row = %#v, want id=%d values=%#v", row, id, want[id])
					}
				}
			}

			before := indexLookups()
			result := execute()
			assertValues(result, map[int64][]string{
				11: {"A-12", "A-11"},
				20: {"B-20", "B-21"},
			})
			if after := indexLookups(); after != before+2 {
				t.Fatalf("B-tree shared-index expected one lookup per outer row: before=%d after=%d", before, after)
			}

			for _, partition := range window.contextPartitionStates() {
				found := 0
				for _, index := range partition.def.indexes {
					if isSubquerySharedIndex(index.Name) && index.Kind == IndexBTree && reflect.DeepEqual(index.Columns, []string{"lookup", "id"}) {
						found++
					}
				}
				if found != 1 {
					t.Fatalf("B-tree shared-index definitions in partition %q = %d, want 1", partition.contextKey, found)
				}
			}

			if _, err := window.UpdateWhere(ctx, func(event Event) bool {
				return event.Get("value").Any() == "A-12"
			}, func(event Event) (any, error) {
				return map[string]any{
					"id":     event.Get("id").Any(),
					"key":    event.Get("key").Any(),
					"lookup": "moved",
					"value":  "A-moved",
				}, nil
			}); err != nil {
				t.Fatal(err)
			}
			before = indexLookups()
			updated := execute()
			assertValues(updated, map[int64][]string{
				11: {"A-11"},
				20: {"B-20", "B-21"},
			})
			if after := indexLookups(); after != before+2 {
				t.Fatalf("B-tree shared-index update expected one lookup per outer row: before=%d after=%d", before, after)
			}

			if _, err := window.DeleteWhere(ctx, func(event Event) bool {
				return event.Get("key").Any() == "B"
			}); err != nil {
				t.Fatal(err)
			}
			deleted := execute()
			assertValues(deleted, map[int64][]string{
				11: {"A-11"},
				20: {},
			})
		})
	}
}

// TestInfraNWTableSubqCorrelIndexOptionValidationParity keeps the invalid
// option boundary explicit. Esper rejects a subquery index hint when the
// named index is missing, when the predicate cannot expose a probe key, or
// when no-index and an explicit index are combined. These are Build-time
// contracts and must not be silently converted to a snapshot fallback.
func TestInfraNWTableSubqCorrelIndexOptionValidationParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env := NewEnvironment()
			schema := newFAFSubqueryIndexSchema(t, env, "NWSubqSharingInvalidSchema")
			if _, err := CreateNamedWindow(env, "NWSubqSharingInvalidOuter", schema, NamedWindowRetention(KeepAll())); err != nil {
				t.Fatal(err)
			}
			var inner RecordStream
			if namedWindow {
				if _, err := CreateNamedWindow(env, "NWSubqSharingInvalidInner", schema,
					NamedWindowRetention(KeepAll()), NamedWindowIndex("by-lookup", "lookup")); err != nil {
					t.Fatal(err)
				}
				inner = FromNamedWindow(env, "NWSubqSharingInvalidInner")
			} else {
				if _, err := CreateTable(env, "NWSubqSharingInvalidInner", []TableColumn{
					PrimaryKeyColumn[int64]("id"), TableColumnOf[string]("lookup"), TableColumnOf[string]("value"),
				}, SecondaryIndex("by-lookup", "lookup")); err != nil {
					t.Fatal(err)
				}
				inner = FromTable(env, "NWSubqSharingInvalidInner")
			}

			outer := FromNamedWindow(env, "NWSubqSharingInvalidOuter")
			validPredicate := Equal[string](Field[any, string]("lookup"), OuterField[string]("key"))
			build := func(options ...SubqueryOption) error {
				subquery := SubqueryValueWithOptions[string](inner, Field[any, string]("value"), options...)
				_, err := env.Build(outer.Select(Alias("value", subquery)).Query())
				return err
			}

			if err := build(SubqueryWhere(validPredicate), SubqueryUseIndex("missing")); err == nil || !errors.Is(err, ErrorUnknownName) {
				t.Fatalf("missing subquery index error = %v, want ErrorUnknownName", err)
			}
			// Both operands are inner fields, so no value is fixed by the outer
			// row and the equality cannot be used as a correlated probe key.
			invalidPredicate := Equal[string](Field[any, string]("lookup"), Field[any, string]("value"))
			if err := build(SubqueryWhere(invalidPredicate), SubqueryUseIndex("by-lookup")); err == nil || !errors.Is(err, ErrorInvalidRule) {
				t.Fatalf("unusable subquery predicate error = %v, want ErrorInvalidRule", err)
			}
			if err := build(SubqueryWhere(validPredicate), SubqueryNoIndex(), SubqueryUseIndex("by-lookup")); err == nil || !errors.Is(err, ErrorInvalidRule) {
				t.Fatalf("no-index plus explicit index error = %v, want ErrorInvalidRule", err)
			}
			if err := build(SubqueryWhere(validPredicate), SubqueryUseIndex("by-lookup"), SubqueryDisableIndexSharing()); err != nil {
				t.Fatalf("explicit index with sharing disabled unexpectedly failed: %v", err)
			}
		})
	}
}
