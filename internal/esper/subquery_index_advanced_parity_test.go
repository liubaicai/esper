package esper

import (
	"context"
	"reflect"
	"testing"
)

// TestInfraNWTableSubqCorrelIndexMultipleIndexHintsParity mirrors
// InfraNWTableSubqCorrelIndexMultipleIndexHints. Each correlated subquery
// carries its own structured index option; the two options must not leak into
// one another or collapse to the first matching index.
func TestInfraNWTableSubqCorrelIndexMultipleIndexHintsParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		name := indexStoreName(namedWindow)
		t.Run(name, func(t *testing.T) {
			env := NewEnvironment()
			schema := newFAFSubqueryIndexSchema(t, env, "SubqueryMultipleHintsSchema"+name)
			outerName := "SubqueryMultipleHintsOuter" + name
			innerName := "SubqueryMultipleHintsInner" + name
			if _, err := CreateNamedWindow(env, outerName, schema, NamedWindowRetention(KeepAll())); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				if _, err := CreateNamedWindow(env, innerName, schema,
					NamedWindowRetention(KeepAll()),
					NamedWindowSubqueryIndexSharing(),
					NamedWindowIndex("I1", "lookup"),
					NamedWindowIndex("I2", "id")); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := CreateTable(env, innerName, []TableColumn{
					PrimaryKeyColumn[int64]("id"),
					TableColumnOf[string]("lookup"),
					TableColumnOf[string]("value"),
				}, SecondaryIndex("I1", "lookup"), SecondaryIndex("I2", "id")); err != nil {
					t.Fatal(err)
				}
			}
			engine := NewEngine(env)
			ctx := context.Background()
			for _, row := range []map[string]any{
				{"id": int64(101), "lookup": "A", "value": "A-hit"},
				{"id": int64(102), "lookup": "B", "value": "B-hit"},
			} {
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, innerName, row); err != nil {
						t.Fatal(err)
					}
					continue
				}
				table, ok := engine.Table(innerName)
				if !ok {
					t.Fatal("multiple-hints table is missing")
				}
				if _, err := table.Insert(ctx, row); err != nil {
					t.Fatal(err)
				}
			}
			for _, row := range []map[string]any{
				{"id": int64(101), "key": "A"},
				{"id": int64(102), "key": "B"},
			} {
				if err := engine.InsertNamedWindow(ctx, outerName, row); err != nil {
					t.Fatal(err)
				}
			}

			inner := func() RecordStream {
				if namedWindow {
					return FromNamedWindow(env, innerName)
				}
				return FromTable(env, innerName)
			}()
			byLookup := SubqueryValueWithOptions[string](inner, Field[any, string]("value"),
				SubqueryWhere(Equal[string](Field[any, string]("lookup"), OuterField[string]("key"))),
				SubqueryUseIndex("I1"),
			)
			byID := SubqueryValueWithOptions[string](inner, Field[any, string]("value"),
				SubqueryWhere(Equal[int64](Field[any, int64]("id"), OuterField[int64]("id"))),
				SubqueryUseIndex("I2"),
			)
			plan, err := env.Build(FromNamedWindow(env, outerName).Select(
				Alias("id", Field[any, int64]("id")),
				Alias("byLookup", byLookup),
				Alias("byID", byID),
			).Query())
			if err != nil {
				t.Fatal(err)
			}

			var before uint64
			if namedWindow {
				window, ok := engine.NamedWindow(innerName)
				if !ok {
					t.Fatal("multiple-hints named window is missing")
				}
				before = window.state.indexLookups.Load()
			} else {
				table, ok := engine.Table(innerName)
				if !ok {
					t.Fatal("multiple-hints table is missing")
				}
				before = table.state.indexLookups.Load()
			}
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil {
				t.Fatal(err)
			}
			want := []struct {
				id       int64
				byLookup string
				byID     string
			}{
				{101, "A-hit", "A-hit"},
				{102, "B-hit", "B-hit"},
			}
			if len(result.Results()) != len(want) {
				t.Fatalf("multiple-hints result count = %d, want %d", len(result.Results()), len(want))
			}
			for index, row := range result.Results() {
				got := struct {
					id       int64
					byLookup string
					byID     string
				}{
					id:       row.Get("id").Any().(int64),
					byLookup: row.Get("byLookup").Any().(string),
					byID:     row.Get("byID").Any().(string),
				}
				if got != want[index] {
					t.Fatalf("multiple-hints row %d = %#v, want %#v", index, got, want[index])
				}
			}
			var after uint64
			if namedWindow {
				window, _ := engine.NamedWindow(innerName)
				after = window.state.indexLookups.Load()
			} else {
				table, _ := engine.Table(innerName)
				after = table.state.indexLookups.Load()
			}
			if after != before+4 {
				t.Fatalf("multiple-hints expected two explicit index probes per outer row: before=%d after=%d", before, after)
			}
		})
	}
}

// TestInfraNWTableSubqCorrelIndexChoiceParity mirrors the observable part of
// InfraNWTableSubqCorrelIndexShareIndexChoice and its no-share counterpart.
// The fluent API exposes the selected path through the structured subquery
// definition rather than a JVM query-plan callback: a complete composite
// equality key wins over a single-column candidate, an equality-prefix range
// uses a B-tree, and an explicit SubqueryUseIndex overrides the automatic
// choice while the final predicate still preserves correctness.
func TestInfraNWTableSubqCorrelIndexChoiceParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env := NewEnvironment()
			name := indexStoreName(namedWindow)
			schema, err := RegisterStruct[infraIndexChoiceEvent](env, "SubqueryIndexChoiceSchema"+name)
			if err != nil {
				t.Fatal(err)
			}
			outerName := "SubqueryIndexChoiceOuter" + name
			innerName := "SubqueryIndexChoiceInner" + name
			if _, err := CreateNamedWindow(env, outerName, schema, NamedWindowRetention(KeepAll())); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				if _, err := CreateNamedWindow(env, innerName, schema,
					NamedWindowRetention(KeepAll()),
					NamedWindowIndex("by-s1", "s1"),
					NamedWindowIndex("by-s1-d1", "s1", "d1"),
					NamedWindowBTreeIndex("by-s1-d1-range", "s1", "d1"),
					NamedWindowBTreeIndex("by-l1", "l1")); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := CreateTable(env, innerName, []TableColumn{
					PrimaryKeyColumn[string]("s1"),
					TableColumnOf[int64]("i1"),
					TableColumnOf[float64]("d1"),
					TableColumnOf[int64]("l1"),
				}, SecondaryIndex("by-s1", "s1"), SecondaryIndex("by-s1-d1", "s1", "d1"),
					SecondaryBTreeIndex("by-s1-d1-range", "s1", "d1"), SecondaryBTreeIndex("by-l1", "l1")); err != nil {
					t.Fatal(err)
				}
			}

			engine := NewEngine(env)
			ctx := context.Background()
			innerRows := []infraIndexChoiceEvent{
				{S1: "A", I1: 10, D1: 11, L1: 12},
				{S1: "B", I1: 20, D1: 21, L1: 22},
			}
			for _, row := range innerRows {
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, innerName, row); err != nil {
						t.Fatal(err)
					}
					continue
				}
				table, ok := engine.Table(innerName)
				if !ok {
					t.Fatal("subquery index-choice table is missing")
				}
				if _, err := table.Insert(ctx, map[string]any{
					"s1": row.S1, "i1": row.I1, "d1": row.D1, "l1": row.L1,
				}); err != nil {
					t.Fatal(err)
				}
			}
			for _, row := range []infraIndexChoiceEvent{
				{S1: "A", D1: 11},
				{S1: "B", D1: 21},
			} {
				if err := engine.InsertNamedWindow(ctx, outerName, row); err != nil {
					t.Fatal(err)
				}
			}

			var inner RecordStream
			if namedWindow {
				inner = FromNamedWindow(env, innerName)
			} else {
				inner = FromTable(env, innerName)
			}
			outer := FromNamedWindow(env, outerName)
			assertSelection := func(t *testing.T, predicate Expression[bool], wantName string, wantAccess IndexAccessKind, wantColumns []string, options ...SubqueryOption) {
				t.Helper()
				subqueryOptions := append([]SubqueryOption{SubqueryWhere(predicate)}, options...)
				value := SubqueryValueWithOptions[int64](inner, Field[any, int64]("i1"), subqueryOptions...)
				plan, err := env.Build(outer.Select(
					Alias("s1", Field[any, string]("s1")),
					Alias("value", value),
				).Query())
				if err != nil {
					t.Fatal(err)
				}
				definition := value.node().subquery
				base, err := subqueryRootSource(definition.source)
				if err != nil {
					t.Fatal(err)
				}
				selection, ok := subqueryIndexSelection(env, base, definition)
				if !ok || selection.IndexName != wantName || selection.Access != wantAccess || !reflect.DeepEqual(selection.Columns, wantColumns) {
					t.Fatalf("subquery index choice = %#v, want name=%q access=%s columns=%v", selection, wantName, wantAccess, wantColumns)
				}
				result, err := engine.ExecuteFireAndForget(ctx, plan)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Results()) != 2 || result.Results()[0].Get("value").Any() != int64(10) || result.Results()[1].Get("value").Any() != int64(20) {
					t.Fatalf("subquery index-choice result = %#v, want [10 20]", result.Results())
				}
			}

			assertSelection(t, And(
				EqualOf(Field[any, string]("s1"), OuterField[string]("s1")),
				EqualOf(Field[any, float64]("d1"), OuterField[float64]("d1")),
			), "by-s1-d1", IndexAccessEquality, []string{"s1", "d1"})
			assertSelection(t, And(
				EqualOf(Field[any, string]("s1"), OuterField[string]("s1")),
				GreaterOrEqual[float64](Field[any, float64]("d1"), OuterField[float64]("d1")),
			), "by-s1-d1-range", IndexAccessRange, []string{"s1", "d1"})
			assertSelection(t, And(
				EqualOf(Field[any, string]("s1"), OuterField[string]("s1")),
				EqualOf(Field[any, float64]("d1"), OuterField[float64]("d1")),
			), "by-s1", IndexAccessEquality, []string{"s1"}, SubqueryUseIndex("by-s1"))
		})
	}
}

// TestInfraNWTableSubqIndexShareMultikeyArrayParity mirrors the single-array
// and two-array executions. Go slices remain value keys in the internal
// encodeKey representation, so equal contents match regardless of backing
// array identity and a non-matching order/content remains a miss.
func TestInfraNWTableSubqIndexShareMultikeyArrayParity(t *testing.T) {
	for _, mode := range []struct {
		name        string
		namedWindow bool
		explicit    bool
	}{
		{name: "named-window-explicit", namedWindow: true, explicit: true},
		{name: "named-window-auto", namedWindow: true},
		{name: "table"},
	} {
		for _, multiKey := range []bool{false, true} {
			name := mode.name
			if multiKey {
				name += "-two-array"
			} else {
				name += "-one-array"
			}
			t.Run(name, func(t *testing.T) {
				env := NewEnvironment()
				schema, err := RegisterStruct[infraIndexArrayEvent](env, "SubqueryArraySchema"+name)
				if err != nil {
					t.Fatal(err)
				}
				outerName := "SubqueryArrayOuter" + name
				innerName := "SubqueryArrayInner" + name
				if _, err := CreateNamedWindow(env, outerName, schema, NamedWindowRetention(KeepAll())); err != nil {
					t.Fatal(err)
				}
				if mode.namedWindow {
					options := []NamedWindowOption{NamedWindowRetention(KeepAll()), NamedWindowSubqueryIndexSharing()}
					if mode.explicit {
						columns := []string{"arrayOne"}
						if multiKey {
							columns = append(columns, "arrayTwo")
						}
						options = append(options, NamedWindowIndex("by-array", columns...))
					}
					if _, err := CreateNamedWindow(env, innerName, schema, options...); err != nil {
						t.Fatal(err)
					}
				} else {
					columns := []TableColumn{PrimaryKeyColumn[[]string]("arrayOne"), TableColumnOf[[]string]("arrayTwo"), TableColumnOf[int64]("value")}
					if multiKey {
						columns = []TableColumn{PrimaryKeyColumn[[]string]("arrayOne"), PrimaryKeyColumn[[]string]("arrayTwo"), TableColumnOf[int64]("value")}
					}
					if _, err := CreateTable(env, innerName, columns); err != nil {
						t.Fatal(err)
					}
				}
				engine := NewEngine(env)
				ctx := context.Background()
				innerRows := []infraIndexArrayEvent{
					{ID: "E1", ArrayOne: []string{"a", "b"}, ArrayTwo: []string{"c", "d"}, Value: 10},
					{ID: "E2", ArrayOne: []string{"a", "c"}, ArrayTwo: []string{"b"}, Value: 20},
					{ID: "E3", ArrayOne: []string{"a"}, ArrayTwo: []string{"c", "d"}, Value: 30},
				}
				for _, row := range innerRows {
					if mode.namedWindow {
						if err := engine.InsertNamedWindow(ctx, innerName, row); err != nil {
							t.Fatal(err)
						}
						continue
					}
					table, ok := engine.Table(innerName)
					if !ok {
						t.Fatal("array subquery table is missing")
					}
					values := map[string]any{"arrayOne": row.ArrayOne, "arrayTwo": row.ArrayTwo, "value": row.Value}
					if _, err := table.Insert(ctx, values); err != nil {
						t.Fatal(err)
					}
				}
				outerRows := []infraIndexArrayEvent{
					{ID: "O1", ArrayOne: []string{"a", "c"}, ArrayTwo: []string{"b"}},
					{ID: "O2", ArrayOne: []string{"a", "b"}, ArrayTwo: []string{"c", "d"}},
					{ID: "O3", ArrayOne: []string{"a"}, ArrayTwo: []string{"c", "d"}},
					{ID: "O4", ArrayOne: []string{"a", "d"}, ArrayTwo: []string{"b"}},
				}
				for _, row := range outerRows {
					if err := engine.InsertNamedWindow(ctx, outerName, row); err != nil {
						t.Fatal(err)
					}
				}

				inner := func() RecordStream {
					if mode.namedWindow {
						return FromNamedWindow(env, innerName)
					}
					return FromTable(env, innerName)
				}()
				var predicate Expression[bool]
				if multiKey {
					predicate = And(
						EqualOf(Field[any, []string]("arrayOne"), OuterField[[]string]("arrayOne")),
						EqualOf(Field[any, []string]("arrayTwo"), OuterField[[]string]("arrayTwo")),
					)
				} else {
					predicate = EqualOf(Field[any, []string]("arrayOne"), OuterField[[]string]("arrayOne"))
				}
				options := []SubqueryOption{SubqueryWhere(predicate)}
				if mode.explicit {
					options = append(options, SubqueryUseIndex("by-array"))
				}
				value := SubqueryValueWithOptions[int64](inner, Field[any, int64]("value"), options...)
				plan, err := env.Build(FromNamedWindow(env, outerName).Select(
					Alias("id", Field[any, string]("id")),
					Alias("value", value),
				).Query())
				if err != nil {
					t.Fatal(err)
				}

				var before uint64
				if mode.namedWindow {
					window, ok := engine.NamedWindow(innerName)
					if !ok {
						t.Fatal("array subquery named window is missing")
					}
					before = window.state.indexLookups.Load()
				} else {
					table, ok := engine.Table(innerName)
					if !ok {
						t.Fatal("array subquery table is missing")
					}
					before = table.state.indexLookups.Load()
				}
				result, err := engine.ExecuteFireAndForget(ctx, plan)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Results()) != len(outerRows) {
					t.Fatalf("array subquery result count = %d, want %d", len(result.Results()), len(outerRows))
				}
				want := []any{int64(20), int64(10), int64(30), nil}
				for index, row := range result.Results() {
					if row.Get("id").Any() != outerRows[index].ID {
						t.Fatalf("array subquery row %d id = %#v", index, row.Get("id").Any())
					}
					if want[index] == nil {
						if !row.Get("value").IsNull() {
							t.Fatalf("array subquery miss row %d = %#v, want Null", index, row)
						}
						continue
					}
					if row.Get("value").Any() != want[index] {
						t.Fatalf("array subquery row %d value = %#v, want %#v", index, row.Get("value"), want[index])
					}
				}
				var after uint64
				if mode.namedWindow {
					window, _ := engine.NamedWindow(innerName)
					after = window.state.indexLookups.Load()
				} else {
					table, _ := engine.Table(innerName)
					after = table.state.indexLookups.Load()
				}
				if after != before+uint64(len(outerRows)) {
					t.Fatalf("array subquery expected one index lookup per outer row: before=%d after=%d", before, after)
				}
				if mode.namedWindow && !mode.explicit {
					window, _ := engine.NamedWindow(innerName)
					found := 0
					columns := []string{"arrayOne"}
					if multiKey {
						columns = []string{"arrayOne", "arrayTwo"}
					}
					for _, index := range window.state.def.indexes {
						if isSubquerySharedIndex(index.Name) && index.Kind == IndexHash && reflect.DeepEqual(index.Columns, columns) {
							found++
						}
					}
					if found != 1 {
						t.Fatalf("array auto-shared index definitions = %d, want 1", found)
					}
				}
			})
		}
	}
}
