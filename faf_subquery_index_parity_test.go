package esper

import (
	"context"
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
