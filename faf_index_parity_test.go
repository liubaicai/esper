package esper

import (
	"context"
	"reflect"
	"testing"
)

type infraIndexArrayEvent struct {
	ID       string   `esper:"id"`
	ArrayOne []string `esper:"arrayOne"`
	ArrayTwo []string `esper:"arrayTwo"`
	Value    int64    `esper:"value"`
}

type infraIndexChoiceEvent struct {
	S1 string  `esper:"s1"`
	I1 int64   `esper:"i1"`
	D1 float64 `esper:"d1"`
	L1 int64   `esper:"l1"`
}

type infraIndexJoinLeft struct {
	ID  string `esper:"id"`
	Key string `esper:"key"`
}

type infraIndexJoinRight struct {
	ID     string `esper:"id"`
	Key    string `esper:"key"`
	Amount int64  `esper:"amount"`
}

func TestInfraFAFIndexMultikeyArrayParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env, engine, source := setupInfraIndexArrayStore(t, namedWindow, []TableOption{SecondaryBTreeIndex("MyInfraIndex", "arrayOne", "value")}, []NamedWindowOption{NamedWindowBTreeIndex("MyInfraIndex", "arrayOne", "value")})
			field := Field[any, []string]("arrayOne")
			arrayAB := ArrayOf[string](Literal("a"), Literal("b"))
			predicate := And(
				EqualOf(field, arrayAB),
				Greater[int64](Field[any, int64]("value"), Literal[int64](150)),
			)
			plan, err := env.Build(source.Filter(predicate).Select(
				Alias("id", Field[any, string]("id")),
			).Query(StatementName("faf-index-array")))
			if err != nil {
				t.Fatal(err)
			}
			selection, ok := plan.IndexPlan().ForSource(0)
			if !ok || selection.IndexName != "MyInfraIndex" || selection.Backing != IndexBackingBTree || selection.Access != IndexAccessRange || !reflect.DeepEqual(selection.MatchedColumns, []string{"arrayOne", "value"}) {
				t.Fatalf("array index plan = %#v, want MyInfraIndex btree range [arrayOne value]", selection)
			}
			result, err := engine.ExecuteFireAndForget(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Results()) != 1 || result.Results()[0].Get("id").Any() != "E2" {
				t.Fatalf("array range result = %#v, want E2", result.Results())
			}

			missPlan, err := env.Build(source.Filter(And(
				EqualOf(field, ArrayOf[string](Literal("a"), Literal("c"))),
				Less[int64](Field[any, int64]("value"), Literal[int64](150)),
			)).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			miss, err := engine.ExecuteFireAndForget(context.Background(), missPlan)
			if err != nil {
				t.Fatal(err)
			}
			if len(miss.Results()) != 0 {
				t.Fatalf("array mismatch result = %#v, want empty", miss.Results())
			}
		})
	}
}

func TestInfraFAFIndexCompositeArrayParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env, engine, source := setupInfraIndexArrayStore(t, namedWindow, []TableOption{SecondaryBTreeIndex("MyInfraIndex", "arrayOne", "arrayTwo", "value")}, []NamedWindowOption{NamedWindowBTreeIndex("MyInfraIndex", "arrayOne", "arrayTwo", "value")})
			arrayOne := Field[any, []string]("arrayOne")
			arrayTwo := Field[any, []string]("arrayTwo")
			predicate := And(
				EqualOf(arrayOne, ArrayOf[string](Literal("a"), Literal("b"))),
				And(
					EqualOf(arrayTwo, ArrayOf[string](Literal("e"), Literal("f"))),
					Greater[int64](Field[any, int64]("value"), Literal[int64](150)),
				),
			)
			plan, err := env.Build(source.Filter(predicate).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			selection, ok := plan.IndexPlan().ForSource(0)
			if !ok || selection.IndexName != "MyInfraIndex" || selection.Access != IndexAccessRange || !reflect.DeepEqual(selection.MatchedColumns, []string{"arrayOne", "arrayTwo", "value"}) {
				t.Fatalf("composite array index plan = %#v", selection)
			}
			result, err := engine.ExecuteFireAndForget(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Results()) != 1 || result.Results()[0].Get("id").Any() != "E2" {
				t.Fatalf("composite array result = %#v, want E2", result.Results())
			}

			exact, err := env.Build(source.Filter(And(
				EqualOf(arrayOne, ArrayOf[string](Literal("a"), Literal("b"))),
				EqualOf(arrayTwo, ArrayOf[string](Literal("c"), Literal("d"))),
			)).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			exactResult, err := engine.ExecuteFireAndForget(context.Background(), exact)
			if err != nil || len(exactResult.Results()) != 1 || exactResult.Results()[0].Get("id").Any() != "E1" {
				t.Fatalf("composite exact result = %#v, err=%v", exactResult.Results(), err)
			}
		})
	}
}

func TestInfraFAFIndexNullArrayAndCompositeEqualityParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env, engine, source := setupInfraIndexArrayStore(t, namedWindow, []TableOption{SecondaryIndex("MyInfraIndex", "arrayOne", "arrayTwo")}, []NamedWindowOption{NamedWindowIndex("MyInfraIndex", "arrayOne", "arrayTwo")})
			arrayOne := Field[any, []string]("arrayOne")
			arrayTwo := Field[any, []string]("arrayTwo")
			plan, err := env.Build(source.Filter(Is(arrayOne, NullLiteral[[]string]())).Select(
				Alias("id", Field[any, string]("id")),
			).Query())
			if err != nil {
				t.Fatal(err)
			}
			result, err := engine.ExecuteFireAndForget(context.Background(), plan)
			if err != nil || len(result.Results()) != 1 || result.Results()[0].Get("id").Any() != "E4" {
				t.Fatalf("null array result = %#v, err=%v", result.Results(), err)
			}

			exact, err := env.Build(source.Filter(And(
				EqualOf(arrayOne, ArrayOf[string](Literal("a"), Literal("b"))),
				EqualOf(arrayTwo, ArrayOf[string](Literal("c"), Literal("d"))),
			)).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			exactResult, err := engine.ExecuteFireAndForget(context.Background(), exact)
			if err != nil || len(exactResult.Results()) != 1 || exactResult.Results()[0].Get("id").Any() != "E1" {
				t.Fatalf("two-array equality result = %#v, err=%v", exactResult.Results(), err)
			}
		})
	}
}

func TestInfraFAFIndexChoiceAndHintParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env := NewEnvironment()
			schema, err := RegisterStruct[infraIndexChoiceEvent](env, "InfraIndexChoiceEvent")
			if err != nil {
				t.Fatal(err)
			}
			var source RecordStream
			if namedWindow {
				_, err = CreateNamedWindow(env, "MyInfra", schema, NamedWindowRetention(Unique(Field[any, string]("s1"))))
				source = FromNamedWindow(env, "MyInfra")
			} else {
				_, err = CreateTable(env, "MyInfra", []TableColumn{
					PrimaryKeyColumn[string]("s1"),
					PrimaryKeyColumn[int64]("i1"),
					PrimaryKeyColumn[float64]("d1"),
					PrimaryKeyColumn[int64]("l1"),
				}, UniqueIndex("One", "s1"), UniqueIndex("Two", "s1", "d1"), SecondaryBTreeIndex("Range", "l1"))
				source = FromTable(env, "MyInfra")
			}
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			ctx := context.Background()
			for _, event := range []infraIndexChoiceEvent{{S1: "E1", I1: 10, D1: 11, L1: 12}, {S1: "E2", I1: 20, D1: 21, L1: 22}} {
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, "MyInfra", event); err != nil {
						t.Fatal(err)
					}
					continue
				}
				table, ok := engine.Table("MyInfra")
				if !ok {
					t.Fatal("table MyInfra is missing")
				}
				if _, err := table.Insert(ctx, map[string]any{"s1": event.S1, "i1": event.I1, "d1": event.D1, "l1": event.L1}); err != nil {
					t.Fatal(err)
				}
			}

			where := And(
				EqualOf(Field[any, string]("s1"), Literal("E2")),
				EqualOf(Field[any, int64]("l1"), Literal[int64](22)),
			)
			plan, err := env.Build(source.Filter(where).Select(
				Alias("s1", Field[any, string]("s1")),
				Alias("i1", Field[any, int64]("i1")),
			).Query())
			if err != nil {
				t.Fatal(err)
			}
			selection, ok := plan.IndexPlan().ForSource(0)
			if !ok || selection.IndexName != "<retention-unique>" && namedWindow || !namedWindow && selection.IndexName != "One" || selection.Access != IndexAccessEquality {
				t.Fatalf("index choice plan = %#v", selection)
			}
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil || len(result.Results()) != 1 || result.Results()[0].Get("s1").Any() != "E2" {
				t.Fatalf("index choice result = %#v, err=%v", result.Results(), err)
			}

			hinted, err := env.Build(source.Filter(where).Select(Alias("s1", Field[any, string]("s1"))).Query(UseIndex("<retention-unique>")))
			if namedWindow {
				if err != nil {
					t.Fatal(err)
				}
				if selection, ok := hinted.IndexPlan().ForSource(0); !ok || !selection.Hinted || selection.IndexName != "<retention-unique>" {
					t.Fatalf("retention hint plan = %#v", selection)
				}
			} else if err == nil {
				t.Fatal("table unexpectedly accepted named-window retention hint")
			}

			rangePlan, err := env.Build(source.Filter(BetweenOf(Field[any, int64]("l1"), Literal[int64](20), Literal[int64](23))).Query())
			if err != nil {
				t.Fatal(err)
			}
			if !namedWindow {
				selection, ok := rangePlan.IndexPlan().ForSource(0)
				if !ok || selection.IndexName != "Range" || selection.Backing != IndexBackingBTree || selection.Access != IndexAccessRange {
					t.Fatalf("range index plan = %#v", selection)
				}
			}

			if _, err := env.Build(source.Filter(where).Query(UseIndex("missing"))); err == nil {
				t.Fatal("missing index hint unexpectedly built")
			}
			if _, err := env.Build(source.Filter(where).Query(UseIndexOn(1, "Range"))); err == nil {
				t.Fatal("out-of-range single-source index hint unexpectedly built")
			}
		})
	}
}

func TestInfraFAFIndexAggregateWhereAndJoinFilterPlanParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env, _, source := setupInfraIndexArrayStore(t, namedWindow,
				[]TableOption{SecondaryBTreeIndex("Range", "value")},
				[]NamedWindowOption{NamedWindowBTreeIndex("Range", "value")})
			plan, err := env.Build(source.Aggregate(
				Alias("total", Sum[int64](Field[any, int64]("value"))),
			).Where(Greater[int64](Field[any, int64]("value"), Literal[int64](150))).Query())
			if err != nil {
				t.Fatal(err)
			}
			selection, ok := plan.IndexPlan().ForSource(0)
			if !ok || selection.IndexName != "Range" || selection.Access != IndexAccessRange {
				t.Fatalf("aggregate where index plan = %#v", selection)
			}
		})
	}
}

func TestInfraFAFIndexJoinChoiceParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env := NewEnvironment()
			leftSchema, err := RegisterStruct[infraIndexJoinLeft](env, "InfraIndexJoinLeft")
			if err != nil {
				t.Fatal(err)
			}
			rightSchema, err := RegisterStruct[infraIndexJoinRight](env, "InfraIndexJoinRight")
			if err != nil {
				t.Fatal(err)
			}
			var left, right RecordStream
			if namedWindow {
				if _, err := CreateNamedWindow(env, "W1", leftSchema, NamedWindowIndex("W1Key", "key")); err != nil {
					t.Fatal(err)
				}
				if _, err := CreateNamedWindow(env, "W2", rightSchema, NamedWindowIndex("W2Key", "key")); err != nil {
					t.Fatal(err)
				}
				left, right = FromNamedWindow(env, "W1"), FromNamedWindow(env, "W2")
			} else {
				if _, err := CreateTable(env, "W1", []TableColumn{PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key")}, SecondaryIndex("W1Key", "key")); err != nil {
					t.Fatal(err)
				}
				if _, err := CreateTable(env, "W2", []TableColumn{PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"), TableColumnOf[int64]("amount")}, SecondaryIndex("W2Key", "key")); err != nil {
					t.Fatal(err)
				}
				left, right = FromTable(env, "W1"), FromTable(env, "W2")
			}
			engine := NewEngine(env)
			ctx := context.Background()
			if namedWindow {
				for _, row := range []any{infraIndexJoinLeft{ID: "L1", Key: "X"}, infraIndexJoinLeft{ID: "L2", Key: "Y"}} {
					if err := engine.InsertNamedWindow(ctx, "W1", row); err != nil {
						t.Fatal(err)
					}
				}
				for _, row := range []any{infraIndexJoinRight{ID: "R1", Key: "X", Amount: 10}, infraIndexJoinRight{ID: "R2", Key: "Y", Amount: 20}} {
					if err := engine.InsertNamedWindow(ctx, "W2", row); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				leftTable, _ := engine.Table("W1")
				rightTable, _ := engine.Table("W2")
				for _, row := range []map[string]any{{"id": "L1", "key": "X"}, {"id": "L2", "key": "Y"}} {
					if _, err := leftTable.Insert(ctx, row); err != nil {
						t.Fatal(err)
					}
				}
				for _, row := range []map[string]any{{"id": "R1", "key": "X", "amount": int64(10)}, {"id": "R2", "key": "Y", "amount": int64(20)}} {
					if _, err := rightTable.Insert(ctx, row); err != nil {
						t.Fatal(err)
					}
				}
			}
			join := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
			)
			plan, err := env.Build(join.Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query(UseIndexOn(0, "W1Key"), UseIndexOn(1, "W2Key")))
			if err != nil {
				t.Fatal(err)
			}
			for sourceIndex, expected := range []string{"W1Key", "W2Key"} {
				selection, ok := plan.IndexPlan().ForSource(sourceIndex)
				if !ok || selection.IndexName != expected || !selection.Hinted || selection.Access != IndexAccessEquality {
					t.Fatalf("join source %d index plan = %#v", sourceIndex, selection)
				}
			}
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil || len(result.Results()) != 2 {
				t.Fatalf("join index result = %#v, err=%v", result.Results(), err)
			}
		})
	}
}

func TestNamedWindowDeclaredIndexLookupTracksMutation(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[infraIndexArrayEvent](env, "InfraIndexLookupEvent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "LookupWindow", schema,
		NamedWindowIndex("by-id", "id"),
		NamedWindowIndex("by-array", "arrayOne", "arrayTwo"),
	); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	ctx := context.Background()
	if err := engine.InsertNamedWindow(ctx, "LookupWindow", infraIndexArrayEvent{ID: "E1", ArrayOne: []string{"a"}, ArrayTwo: []string{"b"}, Value: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(ctx, "LookupWindow", infraIndexArrayEvent{ID: "E2", ArrayOne: []string{"c"}, ArrayTwo: []string{"d"}, Value: 2}); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("LookupWindow")
	if !ok {
		t.Fatal("LookupWindow is missing")
	}
	rows, err := window.Lookup(ctx, "by-array", []string{"a"}, []string{"b"})
	if err != nil || len(rows) != 1 || rows[0].Get("id").Any() != "E1" {
		t.Fatalf("initial named-window index lookup = %#v, err=%v", rows, err)
	}
	if _, err := window.UpdateWhere(ctx, func(event Event) bool {
		return event.Get("id").Any() == "E1"
	}, func(Event) (any, error) {
		return infraIndexArrayEvent{ID: "E1", ArrayOne: []string{"x"}, ArrayTwo: []string{"y"}, Value: 10}, nil
	}); err != nil {
		t.Fatal(err)
	}
	oldKey, err := window.Lookup(ctx, "by-array", []string{"a"}, []string{"b"})
	if err != nil || len(oldKey) != 0 {
		t.Fatalf("stale named-window index lookup = %#v, err=%v", oldKey, err)
	}
	newKey, err := window.Lookup(ctx, "by-array", []string{"x"}, []string{"y"})
	if err != nil || len(newKey) != 1 || newKey[0].Get("value").Any() != int64(10) {
		t.Fatalf("updated named-window index lookup = %#v, err=%v", newKey, err)
	}
	if _, err := window.DeleteWhere(ctx, func(event Event) bool {
		return event.Get("id").Any() == "E2"
	}); err != nil {
		t.Fatal(err)
	}
	deleted, err := window.Lookup(ctx, "by-id", "E2")
	if err != nil || len(deleted) != 0 {
		t.Fatalf("deleted named-window index lookup = %#v, err=%v", deleted, err)
	}
}

func setupInfraIndexArrayStore(t *testing.T, namedWindow bool, tableOptions []TableOption, windowOptions []NamedWindowOption) (*Environment, *Engine, RecordStream) {
	t.Helper()
	env := NewEnvironment()
	schema, err := RegisterStruct[infraIndexArrayEvent](env, "InfraIndexArrayEvent")
	if err != nil {
		t.Fatal(err)
	}
	var source RecordStream
	if namedWindow {
		options := append([]NamedWindowOption{NamedWindowRetention(KeepAll())}, windowOptions...)
		if _, err := CreateNamedWindow(env, "MyInfra", schema, options...); err != nil {
			t.Fatal(err)
		}
		source = FromNamedWindow(env, "MyInfra")
	} else {
		columns := []TableColumn{
			PrimaryKeyColumn[string]("id"),
			TableColumnOf[[]string]("arrayOne"),
			TableColumnOf[[]string]("arrayTwo"),
			TableColumnOf[int64]("value"),
		}
		if _, err := CreateTable(env, "MyInfra", columns, tableOptions...); err != nil {
			t.Fatal(err)
		}
		source = FromTable(env, "MyInfra")
	}
	engine := NewEngine(env)
	ctx := context.Background()
	events := []infraIndexArrayEvent{
		{ID: "E1", ArrayOne: []string{"a", "b"}, ArrayTwo: []string{"c", "d"}, Value: 100},
		{ID: "E2", ArrayOne: []string{"a", "b"}, ArrayTwo: []string{"e", "f"}, Value: 200},
		{ID: "E3", ArrayOne: []string{"a"}, ArrayTwo: []string{"b"}, Value: 300},
		{ID: "E4", ArrayOne: nil, ArrayTwo: nil, Value: 400},
	}
	if namedWindow {
		for _, event := range events {
			if err := engine.InsertNamedWindow(ctx, "MyInfra", event); err != nil {
				t.Fatal(err)
			}
		}
	} else {
		table, ok := engine.Table("MyInfra")
		if !ok {
			t.Fatal("MyInfra table is missing")
		}
		for _, event := range events {
			if _, err := table.Insert(ctx, map[string]any{"id": event.ID, "arrayOne": event.ArrayOne, "arrayTwo": event.ArrayTwo, "value": event.Value}); err != nil {
				t.Fatal(err)
			}
		}
	}
	return env, engine, source
}

func indexStoreName(namedWindow bool) string {
	if namedWindow {
		return "named-window"
	}
	return "table"
}
