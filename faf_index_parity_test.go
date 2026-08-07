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

type infraIndexCompositeJoinEvent struct {
	ID  string `esper:"id"`
	Key string `esper:"key"`
	Seq int64  `esper:"seq"`
}

type infraIndexRangeJoinLeft struct {
	ID  string `esper:"id"`
	Key string `esper:"key"`
	Min int64  `esper:"min"`
	Max int64  `esper:"max"`
}

type infraIndexRangeJoinRight struct {
	ID    string `esper:"id"`
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type infraContextIndexEvent struct {
	ID    string `esper:"id"`
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
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
			var leftLookups, rightLookups func() uint64
			if namedWindow {
				leftWindow, ok := engine.NamedWindow("W1")
				if !ok {
					t.Fatal("W1 named window is missing")
				}
				rightWindow, ok := engine.NamedWindow("W2")
				if !ok {
					t.Fatal("W2 named window is missing")
				}
				leftLookups = func() uint64 { return leftWindow.state.indexLookups.Load() }
				rightLookups = func() uint64 { return rightWindow.state.indexLookups.Load() }
			} else {
				leftTable, ok := engine.Table("W1")
				if !ok {
					t.Fatal("W1 table is missing")
				}
				rightTable, ok := engine.Table("W2")
				if !ok {
					t.Fatal("W2 table is missing")
				}
				leftLookups = func() uint64 { return leftTable.state.indexLookups.Load() }
				rightLookups = func() uint64 { return rightTable.state.indexLookups.Load() }
			}
			leftBefore, rightBefore := leftLookups(), rightLookups()
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil || len(result.Results()) != 2 {
				t.Fatalf("join index result = %#v, err=%v", result.Results(), err)
			}
			if rightAfter := rightLookups(); rightAfter <= rightBefore {
				t.Fatalf("join right source did not use its hash candidate index: before=%d after=%d", rightBefore, rightAfter)
			}
			if leftAfter := leftLookups(); leftAfter != leftBefore {
				t.Fatalf("join left source unexpectedly probed its index: before=%d after=%d", leftBefore, leftAfter)
			}

			autoPlan, err := env.Build(join.Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query())
			if err != nil {
				t.Fatal(err)
			}
			for sourceIndex, expected := range []string{"W1Key", "W2Key"} {
				selection, ok := autoPlan.IndexPlan().ForSource(sourceIndex)
				if !ok || selection.IndexName != expected || selection.Access != IndexAccessEquality {
					t.Fatalf("automatic join source %d index plan = %#v", sourceIndex, selection)
				}
			}
			leftBefore, rightBefore = leftLookups(), rightLookups()
			autoResult, err := engine.ExecuteFireAndForget(ctx, autoPlan)
			if err != nil || len(autoResult.Results()) != 2 {
				t.Fatalf("automatic join index result = %#v, err=%v", autoResult.Results(), err)
			}
			if rightAfter := rightLookups(); rightAfter <= rightBefore {
				t.Fatalf("automatic join did not use its hash candidate index: before=%d after=%d", rightBefore, rightAfter)
			}
			if leftAfter := leftLookups(); leftAfter != leftBefore {
				t.Fatalf("automatic join left source unexpectedly probed its index: before=%d after=%d", leftBefore, leftAfter)
			}

			orPlan, err := env.Build(join.Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Where(Or(
				GreaterOrEqual[int64](JoinField[int64](1, "amount"), Literal[int64](10)),
				Less[int64](JoinField[int64](1, "amount"), Literal[int64](0)),
			)).Query(UseIndexOn(0, "W1Key"), UseIndexOn(1, "W2Key")))
			if err != nil {
				t.Fatal(err)
			}
			leftBefore, rightBefore = leftLookups(), rightLookups()
			orResult, err := engine.ExecuteFireAndForget(ctx, orPlan)
			if err != nil || len(orResult.Results()) != 2 {
				t.Fatalf("join OR fallback result = %#v, err=%v", orResult.Results(), err)
			}
			if rightAfter := rightLookups(); rightAfter != rightBefore {
				t.Fatalf("join OR fallback unexpectedly probed the right index: before=%d after=%d", rightBefore, rightAfter)
			}
			if leftAfter := leftLookups(); leftAfter != leftBefore {
				t.Fatalf("join OR fallback unexpectedly probed the left index: before=%d after=%d", leftBefore, leftAfter)
			}

			filteredJoin := JoinMany(JoinRecordSource(left), JoinRecordSource(right.Filter(
				EqualOf(Field[any, string]("key"), Literal("X")),
			))).On(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
			)
			filteredPlan, err := env.Build(filteredJoin.Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query(UseIndexOn(0, "W1Key"), UseIndexOn(1, "W2Key")))
			if err != nil {
				t.Fatal(err)
			}
			leftBefore, rightBefore = leftLookups(), rightLookups()
			filteredResult, err := engine.ExecuteFireAndForget(ctx, filteredPlan)
			if err != nil || len(filteredResult.Results()) != 1 {
				t.Fatalf("filtered join candidate result = %#v, err=%v", filteredResult.Results(), err)
			}
			if rightAfter := rightLookups(); rightAfter <= rightBefore {
				t.Fatalf("filtered join did not use its right candidate index: before=%d after=%d", rightBefore, rightAfter)
			}
			if leftAfter := leftLookups(); leftAfter != leftBefore {
				t.Fatalf("filtered join left source unexpectedly probed its index: before=%d after=%d", leftBefore, leftAfter)
			}

			identity := Func1[string, string]("identity-join-key", func(value string) string { return value }, Field[any, string]("key"))
			udfJoin := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(
				OnSourcesEqual(0, identity, 1, Field[any, string]("key")),
			)
			udfPlan, err := env.Build(udfJoin.Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query(UseIndexOn(1, "W2Key")))
			if err != nil {
				t.Fatal(err)
			}
			leftBefore, rightBefore = leftLookups(), rightLookups()
			udfResult, err := engine.ExecuteFireAndForget(ctx, udfPlan)
			if err != nil || len(udfResult.Results()) != 2 {
				t.Fatalf("join UDF fallback result = %#v, err=%v", udfResult.Results(), err)
			}
			if rightAfter := rightLookups(); rightAfter != rightBefore {
				t.Fatalf("join UDF fallback unexpectedly probed the right index: before=%d after=%d", rightBefore, rightAfter)
			}
			if leftAfter := leftLookups(); leftAfter != leftBefore {
				t.Fatalf("join UDF fallback unexpectedly probed the left index: before=%d after=%d", leftBefore, leftAfter)
			}
		})
	}
}

func TestInfraFAFIndexJoinOuterFallbackAndProbeBoundParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env := NewEnvironment()
			leftSchema, err := RegisterStruct[infraIndexJoinLeft](env, "InfraIndexOuterLeft")
			if err != nil {
				t.Fatal(err)
			}
			rightSchema, err := RegisterStruct[infraIndexJoinRight](env, "InfraIndexOuterRight")
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
				for _, row := range []infraIndexJoinRight{
					{ID: "R1", Key: "X", Amount: 10},
					{ID: "R2", Key: "Z", Amount: 20},
				} {
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
				for _, row := range []map[string]any{
					{"id": "R1", "key": "X", "amount": int64(10)},
					{"id": "R2", "key": "Z", "amount": int64(20)},
				} {
					if _, err := rightTable.Insert(ctx, row); err != nil {
						t.Fatal(err)
					}
				}
			}
			join := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
			).LeftOuter()
			plan, err := env.Build(join.Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query(UseIndexOn(0, "W1Key"), UseIndexOn(1, "W2Key")))
			if err != nil {
				t.Fatal(err)
			}
			var leftLookups, rightLookups func() uint64
			if namedWindow {
				leftWindow, _ := engine.NamedWindow("W1")
				rightWindow, _ := engine.NamedWindow("W2")
				leftLookups = func() uint64 { return leftWindow.state.indexLookups.Load() }
				rightLookups = func() uint64 { return rightWindow.state.indexLookups.Load() }
			} else {
				leftTable, _ := engine.Table("W1")
				rightTable, _ := engine.Table("W2")
				leftLookups = func() uint64 { return leftTable.state.indexLookups.Load() }
				rightLookups = func() uint64 { return rightTable.state.indexLookups.Load() }
			}
			leftBefore, rightBefore := leftLookups(), rightLookups()
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil || len(result.Results()) != 2 {
				t.Fatalf("outer join result = %#v, err=%v", result.Results(), err)
			}
			if leftLookups() != leftBefore || rightLookups() != rightBefore+1 {
				t.Fatalf("left outer join did not use the optional-side candidate path: left %d->%d, right %d->%d", leftBefore, leftLookups(), rightBefore, rightLookups())
			}

			rightOuter := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
			).RightOuter()
			rightPlan, err := env.Build(rightOuter.Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query(UseIndexOn(0, "W1Key"), UseIndexOn(1, "W2Key")))
			if err != nil {
				t.Fatal(err)
			}
			leftBefore, rightBefore = leftLookups(), rightLookups()
			rightResult, err := engine.ExecuteFireAndForget(ctx, rightPlan)
			if err != nil || len(rightResult.Results()) != 2 {
				t.Fatalf("right outer join result = %#v, err=%v", rightResult.Results(), err)
			}
			if leftAfter := leftLookups(); leftAfter != leftBefore+1 {
				t.Fatalf("right outer join did not probe optional left side: before=%d after=%d", leftBefore, leftAfter)
			}
			if rightAfter := rightLookups(); rightAfter != rightBefore {
				t.Fatalf("right outer join unexpectedly probed preserved right side: before=%d after=%d", rightBefore, rightAfter)
			}
			if got := rightResult.Results()[0].Get("left").Any(); got != "L1" {
				t.Fatalf("right outer matched row left id = %#v, want L1", got)
			}
			if got := rightResult.Results()[1].Get("right").Any(); got != "R2" {
				t.Fatalf("right outer unmatched row right id = %#v, want R2", got)
			}

			fullOuter := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
			).FullOuter()
			fullPlan, err := env.Build(fullOuter.Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query(UseIndexOn(0, "W1Key"), UseIndexOn(1, "W2Key")))
			if err != nil {
				t.Fatal(err)
			}
			leftBefore, rightBefore = leftLookups(), rightLookups()
			fullResult, err := engine.ExecuteFireAndForget(ctx, fullPlan)
			if err != nil || len(fullResult.Results()) != 3 {
				t.Fatalf("full outer join result = %#v, err=%v", fullResult.Results(), err)
			}
			if leftLookups() != leftBefore || rightLookups() != rightBefore {
				t.Fatalf("full outer join unexpectedly used an optional-side candidate path: left %d->%d, right %d->%d", leftBefore, leftLookups(), rightBefore, rightLookups())
			}
		})
	}

	sides := [][]storedEvent{make([]storedEvent, maxIndexProbeKeys+1), nil}
	tupleProbes, usable := joinIndexProbeTuples(sides, []bool{true, false}, 1)
	if usable || tupleProbes != nil {
		t.Fatalf("oversized join probe unexpectedly remained usable: tuples=%d usable=%v", len(tupleProbes), usable)
	}
	tupleProbes, usable = joinIndexProbeTuples([][]storedEvent{nil, nil}, []bool{true, false}, 1)
	if !usable || tupleProbes != nil {
		t.Fatalf("empty loaded join side was not represented as a usable empty probe: tuples=%v usable=%v", tupleProbes, usable)
	}
}

func TestInfraFAFContextIndexCandidateEqualityRangeParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env := NewEnvironment()
			schema, err := RegisterStruct[infraContextIndexEvent](env, "InfraContextIndexEvent")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := CreateKeyContext(env, "infra-context-index", Field[any, string]("key")); err != nil {
				t.Fatal(err)
			}
			var source RecordStream
			if namedWindow {
				if _, err := CreateNamedWindow(env, "ContextIndexStore", schema,
					NamedWindowIndex("by-key", "key"),
					NamedWindowBTreeIndex("by-value", "key", "value"),
				); err != nil {
					t.Fatal(err)
				}
				source = FromNamedWindow(env, "ContextIndexStore")
			} else {
				if _, err := CreateTable(env, "ContextIndexStore", []TableColumn{
					PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"), TableColumnOf[int64]("value"),
				}, SecondaryIndex("by-key", "key"), SecondaryBTreeIndex("by-value", "key", "value")); err != nil {
					t.Fatal(err)
				}
				source = FromTable(env, "ContextIndexStore")
			}
			engine := NewEngine(env)
			ctx := context.Background()
			rows := []infraContextIndexEvent{
				{ID: "A1", Key: "A", Value: 10},
				{ID: "A2", Key: "A", Value: 20},
				{ID: "B1", Key: "B", Value: 30},
				{ID: "B2", Key: "B", Value: 40},
			}
			for _, row := range rows {
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, "ContextIndexStore", row); err != nil {
						t.Fatal(err)
					}
					continue
				}
				table, ok := engine.Table("ContextIndexStore")
				if !ok {
					t.Fatal("context index table is missing")
				}
				if _, err := table.Insert(ctx, map[string]any{"id": row.ID, "key": row.Key, "value": row.Value}); err != nil {
					t.Fatal(err)
				}
			}

			keyField := Field[any, string]("key")
			equalityPlan, err := env.Build(source.Filter(
				EqualOf(keyField, Parameter[string]("wanted")),
			).Select(
				Alias("id", Field[any, string]("id")),
				Alias("key", keyField),
			).Query(WithContext("infra-context-index")))
			if err != nil {
				t.Fatal(err)
			}
			selection, ok := equalityPlan.IndexPlan().ForSource(0)
			if !ok || selection.IndexName != "by-key" || selection.Access != IndexAccessEquality {
				t.Fatalf("context equality index plan = %#v", selection)
			}
			var indexLookups func() uint64
			if namedWindow {
				window, ok := engine.NamedWindow("ContextIndexStore")
				if !ok {
					t.Fatal("context index named window is missing")
				}
				indexLookups = func() uint64 { return window.state.indexLookups.Load() }
			} else {
				table, ok := engine.Table("ContextIndexStore")
				if !ok {
					t.Fatal("context index table is missing")
				}
				indexLookups = func() uint64 { return table.state.indexLookups.Load() }
			}
			keyA := encodeKey([]any{ValuePresent, "A"})
			before := indexLookups()
			allEquality, err := engine.ExecuteFireAndForgetWithSelectorAndParameters(ctx, equalityPlan, ContextPartitionSelectorAll{}, ParameterValues{"wanted": "A"})
			if err != nil {
				t.Fatal(err)
			}
			if got := infraContextIndexResultIDs(allEquality.Results()); !reflect.DeepEqual(got, []string{"A1", "A2"}) {
				t.Fatalf("context equality all result = %#v", got)
			}
			if indexLookups() <= before {
				t.Fatalf("context equality did not use index: before=%d after=%d", before, indexLookups())
			}
			selectedEquality, err := engine.ExecuteFireAndForgetWithSelectorAndParameters(ctx, equalityPlan, SelectContextPartitions(keyA), ParameterValues{"wanted": "A"})
			if err != nil || !reflect.DeepEqual(infraContextIndexResultIDs(selectedEquality.Results()), []string{"A1", "A2"}) {
				t.Fatalf("context equality selected result = %#v, err=%v", selectedEquality.Results(), err)
			}
			keyBSelected, err := engine.ExecuteFireAndForgetWithSelectorAndParameters(ctx, equalityPlan, SelectContextPartitions(encodeKey([]any{ValuePresent, "B"})), ParameterValues{"wanted": "A"})
			if err != nil {
				t.Fatal(err)
			}
			if len(keyBSelected.Results()) != 0 {
				t.Fatalf("context equality wrong selector result = %#v", keyBSelected.Results())
			}

			rangePlan, err := env.Build(source.Filter(And(
				EqualOf(keyField, Literal("B")),
				GreaterOrEqual[int64](Field[any, int64]("value"), Parameter[int64]("minimum")),
			)).Select(
				Alias("id", Field[any, string]("id")),
				Alias("value", Field[any, int64]("value")),
			).Query(WithContext("infra-context-index")))
			if err != nil {
				t.Fatal(err)
			}
			selection, ok = rangePlan.IndexPlan().ForSource(0)
			if !ok || selection.IndexName != "by-value" || selection.Access != IndexAccessRange || !reflect.DeepEqual(selection.MatchedColumns, []string{"key", "value"}) {
				t.Fatalf("context range index plan = %#v", selection)
			}
			before = indexLookups()
			rangeResult, err := engine.ExecuteFireAndForgetWithSelectorAndParameters(ctx, rangePlan, ContextPartitionSelectorAll{}, ParameterValues{"minimum": int64(35)})
			if err != nil {
				t.Fatal(err)
			}
			if got := infraContextIndexResultIDs(rangeResult.Results()); !reflect.DeepEqual(got, []string{"B2"}) {
				t.Fatalf("context range all result = %#v", got)
			}
			if indexLookups() <= before {
				t.Fatalf("context range did not use index: before=%d after=%d", before, indexLookups())
			}
			selectedRange, err := engine.ExecuteFireAndForgetWithSelectorAndParameters(ctx, rangePlan, SelectContextPartitions(encodeKey([]any{ValuePresent, "A"})), ParameterValues{"minimum": int64(0)})
			if err != nil {
				t.Fatal(err)
			}
			if len(selectedRange.Results()) != 0 {
				t.Fatalf("context range wrong selector result = %#v", selectedRange.Results())
			}
		})
	}
}

func infraContextIndexResultIDs(results []Result) []string {
	ids := make([]string, 0, len(results))
	for _, result := range results {
		ids = append(ids, result.Get("id").Any().(string))
	}
	return ids
}

func TestInfraFAFContextJoinIndexCandidateEqualityParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env := NewEnvironment()
			leftSchema, err := RegisterStruct[infraIndexJoinLeft](env, "InfraContextJoinLeft")
			if err != nil {
				t.Fatal(err)
			}
			rightSchema, err := RegisterStruct[infraIndexJoinRight](env, "InfraContextJoinRight")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := CreateKeyContext(env, "infra-context-join", Field[any, string]("key")); err != nil {
				t.Fatal(err)
			}
			var left, right RecordStream
			if namedWindow {
				if _, err := CreateNamedWindow(env, "ContextJoinLeft", leftSchema); err != nil {
					t.Fatal(err)
				}
				if _, err := CreateNamedWindow(env, "ContextJoinRight", rightSchema, NamedWindowIndex("by-key", "key")); err != nil {
					t.Fatal(err)
				}
				left, right = FromNamedWindow(env, "ContextJoinLeft"), FromNamedWindow(env, "ContextJoinRight")
			} else {
				if _, err := CreateTable(env, "ContextJoinLeft", []TableColumn{
					PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"),
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := CreateTable(env, "ContextJoinRight", []TableColumn{
					PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"), TableColumnOf[int64]("amount"),
				}, SecondaryIndex("by-key", "key")); err != nil {
					t.Fatal(err)
				}
				left, right = FromTable(env, "ContextJoinLeft"), FromTable(env, "ContextJoinRight")
			}

			engine := NewEngine(env)
			ctx := context.Background()
			insertLeft := func(row infraIndexJoinLeft) {
				t.Helper()
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, "ContextJoinLeft", row); err != nil {
						t.Fatal(err)
					}
					return
				}
				table, ok := engine.Table("ContextJoinLeft")
				if !ok {
					t.Fatal("context join left table is missing")
				}
				if _, err := table.Insert(ctx, map[string]any{"id": row.ID, "key": row.Key}); err != nil {
					t.Fatal(err)
				}
			}
			insertRight := func(row infraIndexJoinRight) {
				t.Helper()
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, "ContextJoinRight", row); err != nil {
						t.Fatal(err)
					}
					return
				}
				table, ok := engine.Table("ContextJoinRight")
				if !ok {
					t.Fatal("context join right table is missing")
				}
				if _, err := table.Insert(ctx, map[string]any{"id": row.ID, "key": row.Key, "amount": row.Amount}); err != nil {
					t.Fatal(err)
				}
			}
			insertLeft(infraIndexJoinLeft{ID: "L1", Key: "A"})
			insertLeft(infraIndexJoinLeft{ID: "L2", Key: "A"})
			insertLeft(infraIndexJoinLeft{ID: "L3", Key: "B"})
			insertRight(infraIndexJoinRight{ID: "R1", Key: "A", Amount: 10})
			insertRight(infraIndexJoinRight{ID: "R2", Key: "B", Amount: 20})
			insertRight(infraIndexJoinRight{ID: "R3", Key: "C", Amount: 30})

			plan, err := env.Build(JoinMany(
				JoinRecordSource(left), JoinRecordSource(right),
			).On(OnSourcesEqual(
				0, Field[any, string]("key"), 1, Field[any, string]("key"),
			)).Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query(WithContext("infra-context-join")))
			if err != nil {
				t.Fatal(err)
			}
			selection, ok := plan.IndexPlan().ForSource(1)
			if !ok || selection.IndexName != "by-key" || selection.Access != IndexAccessEquality {
				t.Fatalf("context join equality index plan = %#v", selection)
			}

			var rightLookups func() uint64
			if namedWindow {
				window, ok := engine.NamedWindow("ContextJoinRight")
				if !ok {
					t.Fatal("context join right named window is missing")
				}
				rightLookups = func() uint64 { return window.state.indexLookups.Load() }
			} else {
				table, ok := engine.Table("ContextJoinRight")
				if !ok {
					t.Fatal("context join right table is missing")
				}
				rightLookups = func() uint64 { return table.state.indexLookups.Load() }
			}

			before := rightLookups()
			all, err := engine.ExecuteFireAndForgetWithSelector(ctx, plan, ContextPartitionSelectorAll{})
			if err != nil {
				t.Fatal(err)
			}
			pairs := make([]string, 0, len(all.Results()))
			for _, result := range all.Results() {
				pairs = append(pairs, result.Get("left").Any().(string)+"/"+result.Get("right").Any().(string))
			}
			if !reflect.DeepEqual(pairs, []string{"L1/R1", "L2/R1", "L3/R2"}) {
				t.Fatalf("context equality join result = %#v", pairs)
			}
			if rightLookups() <= before {
				t.Fatalf("context equality join did not use right index: before=%d after=%d", before, rightLookups())
			}

			keyA := encodeKey([]any{ValuePresent, "A"})
			before = rightLookups()
			selected, err := engine.ExecuteFireAndForgetWithSelector(ctx, plan, SelectContextPartitions(keyA))
			if err != nil {
				t.Fatal(err)
			}
			selectedPairs := make([]string, 0, len(selected.Results()))
			for _, result := range selected.Results() {
				selectedPairs = append(selectedPairs, result.Get("left").Any().(string)+"/"+result.Get("right").Any().(string))
			}
			if !reflect.DeepEqual(selectedPairs, []string{"L1/R1", "L2/R1"}) {
				t.Fatalf("context equality selected join result = %#v", selectedPairs)
			}
			if rightLookups() != before+1 {
				t.Fatalf("context equality selected join expected one lookup: before=%d after=%d", before, rightLookups())
			}

			keyB := encodeKey([]any{ValuePresent, "B"})
			wrong, err := engine.ExecuteFireAndForgetWithSelector(ctx, plan, SelectContextPartitions(keyB))
			if err != nil {
				t.Fatal(err)
			}
			if len(wrong.Results()) != 1 || wrong.Results()[0].Get("left").Any() != "L3" || wrong.Results()[0].Get("right").Any() != "R2" {
				t.Fatalf("context equality B selector result = %#v", wrong.Results())
			}
		})
	}
}

func TestInfraFAFContextJoinIndexCandidateRangeParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env := NewEnvironment()
			leftSchema, err := RegisterStruct[infraIndexRangeJoinLeft](env, "InfraContextRangeJoinLeft")
			if err != nil {
				t.Fatal(err)
			}
			rightSchema, err := RegisterStruct[infraIndexRangeJoinRight](env, "InfraContextRangeJoinRight")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := CreateKeyContext(env, "infra-context-range-join", Field[any, string]("key")); err != nil {
				t.Fatal(err)
			}
			var left, right RecordStream
			if namedWindow {
				if _, err := CreateNamedWindow(env, "ContextRangeJoinLeft", leftSchema); err != nil {
					t.Fatal(err)
				}
				if _, err := CreateNamedWindow(env, "ContextRangeJoinRight", rightSchema, NamedWindowBTreeIndex("range", "key", "value")); err != nil {
					t.Fatal(err)
				}
				left, right = FromNamedWindow(env, "ContextRangeJoinLeft"), FromNamedWindow(env, "ContextRangeJoinRight")
			} else {
				if _, err := CreateTable(env, "ContextRangeJoinLeft", []TableColumn{
					PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"), TableColumnOf[int64]("min"), TableColumnOf[int64]("max"),
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := CreateTable(env, "ContextRangeJoinRight", []TableColumn{
					PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"), TableColumnOf[int64]("value"),
				}, SecondaryBTreeIndex("range", "key", "value")); err != nil {
					t.Fatal(err)
				}
				left, right = FromTable(env, "ContextRangeJoinLeft"), FromTable(env, "ContextRangeJoinRight")
			}

			engine := NewEngine(env)
			ctx := context.Background()
			insertLeft := func(row infraIndexRangeJoinLeft) {
				t.Helper()
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, "ContextRangeJoinLeft", row); err != nil {
						t.Fatal(err)
					}
					return
				}
				table, ok := engine.Table("ContextRangeJoinLeft")
				if !ok {
					t.Fatal("context range left table is missing")
				}
				if _, err := table.Insert(ctx, map[string]any{"id": row.ID, "key": row.Key, "min": row.Min, "max": row.Max}); err != nil {
					t.Fatal(err)
				}
			}
			insertRight := func(row infraIndexRangeJoinRight) {
				t.Helper()
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, "ContextRangeJoinRight", row); err != nil {
						t.Fatal(err)
					}
					return
				}
				table, ok := engine.Table("ContextRangeJoinRight")
				if !ok {
					t.Fatal("context range right table is missing")
				}
				if _, err := table.Insert(ctx, map[string]any{"id": row.ID, "key": row.Key, "value": row.Value}); err != nil {
					t.Fatal(err)
				}
			}
			for _, row := range []infraIndexRangeJoinLeft{
				{ID: "L1", Key: "X", Min: 10, Max: 20},
				{ID: "L2", Key: "X", Min: 20, Max: 30},
				{ID: "L3", Key: "Y", Min: 0, Max: 5},
			} {
				insertLeft(row)
			}
			for _, row := range []infraIndexRangeJoinRight{
				{ID: "R1", Key: "X", Value: 5},
				{ID: "R2", Key: "X", Value: 10},
				{ID: "R3", Key: "X", Value: 15},
				{ID: "R4", Key: "X", Value: 20},
				{ID: "R5", Key: "X", Value: 25},
				{ID: "R6", Key: "Y", Value: 5},
				{ID: "R7", Key: "Y", Value: 6},
			} {
				insertRight(row)
			}

			plan, err := env.Build(JoinMany(
				JoinRecordSource(left), JoinRecordSource(right),
			).On(AllJoin(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
				OnSourcesCompare(0, Field[any, int64]("min"), 1, Field[any, int64]("value"), JoinLessOrEqual),
				OnSourcesCompare(0, Field[any, int64]("max"), 1, Field[any, int64]("value"), JoinGreaterOrEqual),
			)).Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query(WithContext("infra-context-range-join")))
			if err != nil {
				t.Fatal(err)
			}
			selection, ok := plan.IndexPlan().ForSource(1)
			if !ok || selection.IndexName != "range" || selection.Access != IndexAccessRange || selection.Backing != IndexBackingBTree {
				t.Fatalf("context join range index plan = %#v", selection)
			}

			var rightLookups func() uint64
			if namedWindow {
				window, ok := engine.NamedWindow("ContextRangeJoinRight")
				if !ok {
					t.Fatal("context range right named window is missing")
				}
				rightLookups = func() uint64 { return window.state.indexLookups.Load() }
			} else {
				table, ok := engine.Table("ContextRangeJoinRight")
				if !ok {
					t.Fatal("context range right table is missing")
				}
				rightLookups = func() uint64 { return table.state.indexLookups.Load() }
			}
			before := rightLookups()
			result, err := engine.ExecuteFireAndForgetWithSelector(ctx, plan, ContextPartitionSelectorAll{})
			if err != nil {
				t.Fatal(err)
			}
			pairs := make([]string, 0, len(result.Results()))
			for _, row := range result.Results() {
				pairs = append(pairs, row.Get("left").Any().(string)+"/"+row.Get("right").Any().(string))
			}
			if !reflect.DeepEqual(pairs, []string{"L1/R2", "L1/R3", "L1/R4", "L2/R4", "L2/R5", "L3/R6"}) {
				t.Fatalf("context range join result = %#v", pairs)
			}
			if rightLookups() <= before {
				t.Fatalf("context range join did not use right index: before=%d after=%d", before, rightLookups())
			}

			keyA := encodeKey([]any{ValuePresent, "X"})
			selected, err := engine.ExecuteFireAndForgetWithSelector(ctx, plan, SelectContextPartitions(keyA))
			if err != nil {
				t.Fatal(err)
			}
			if len(selected.Results()) != 5 {
				t.Fatalf("context range selected X result = %#v", selected.Results())
			}
		})
	}
}

func TestInfraFAFIndexJoinCompositeEqualityAndDuplicateProbeParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env := NewEnvironment()
			schema, err := RegisterStruct[infraIndexCompositeJoinEvent](env, "InfraIndexCompositeJoinEvent")
			if err != nil {
				t.Fatal(err)
			}
			var left, right RecordStream
			if namedWindow {
				if _, err := CreateNamedWindow(env, "W1", schema, NamedWindowIndex("composite", "key", "seq")); err != nil {
					t.Fatal(err)
				}
				if _, err := CreateNamedWindow(env, "W2", schema, NamedWindowIndex("composite", "key", "seq")); err != nil {
					t.Fatal(err)
				}
				left, right = FromNamedWindow(env, "W1"), FromNamedWindow(env, "W2")
			} else {
				columns := []TableColumn{
					PrimaryKeyColumn[string]("id"),
					TableColumnOf[string]("key"),
					TableColumnOf[int64]("seq"),
				}
				if _, err := CreateTable(env, "W1", columns, SecondaryIndex("composite", "key", "seq")); err != nil {
					t.Fatal(err)
				}
				if _, err := CreateTable(env, "W2", columns, SecondaryIndex("composite", "key", "seq")); err != nil {
					t.Fatal(err)
				}
				left, right = FromTable(env, "W1"), FromTable(env, "W2")
			}
			engine := NewEngine(env)
			ctx := context.Background()
			leftRows := []infraIndexCompositeJoinEvent{
				{ID: "L1", Key: "X", Seq: 7},
				{ID: "L2", Key: "X", Seq: 7},
				{ID: "L3", Key: "Y", Seq: 8},
			}
			rightRows := []infraIndexCompositeJoinEvent{
				{ID: "R1", Key: "X", Seq: 7},
				{ID: "R2", Key: "X", Seq: 9},
				{ID: "R3", Key: "Y", Seq: 8},
			}
			insert := func(source string, rows []infraIndexCompositeJoinEvent) {
				t.Helper()
				for _, row := range rows {
					if namedWindow {
						if err := engine.InsertNamedWindow(ctx, source, row); err != nil {
							t.Fatal(err)
						}
						continue
					}
					table, ok := engine.Table(source)
					if !ok {
						t.Fatalf("table %s is missing", source)
					}
					if _, err := table.Insert(ctx, map[string]any{"id": row.ID, "key": row.Key, "seq": row.Seq}); err != nil {
						t.Fatal(err)
					}
				}
			}
			insert("W1", leftRows)
			insert("W2", rightRows)

			join := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
				OnSourcesEqual(0, Field[any, int64]("seq"), 1, Field[any, int64]("seq")),
			)
			plan, err := env.Build(join.Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query())
			if err != nil {
				t.Fatal(err)
			}
			selection, ok := plan.IndexPlan().ForSource(1)
			if !ok || selection.IndexName != "composite" || selection.Access != IndexAccessEquality || !reflect.DeepEqual(selection.Columns, []string{"key", "seq"}) || !reflect.DeepEqual(selection.MatchedColumns, []string{"key", "seq"}) {
				t.Fatalf("composite join index plan = %#v", selection)
			}
			var leftLookups, rightLookups func() uint64
			if namedWindow {
				leftWindow, _ := engine.NamedWindow("W1")
				rightWindow, _ := engine.NamedWindow("W2")
				leftLookups = func() uint64 { return leftWindow.state.indexLookups.Load() }
				rightLookups = func() uint64 { return rightWindow.state.indexLookups.Load() }
			} else {
				leftTable, _ := engine.Table("W1")
				rightTable, _ := engine.Table("W2")
				leftLookups = func() uint64 { return leftTable.state.indexLookups.Load() }
				rightLookups = func() uint64 { return rightTable.state.indexLookups.Load() }
			}
			leftBefore, rightBefore := leftLookups(), rightLookups()
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Results()) != 3 {
				t.Fatalf("composite join result = %#v, want three rows", result.Results())
			}
			pairs := make([]string, 0, len(result.Results()))
			for _, row := range result.Results() {
				pairs = append(pairs, row.Get("left").Any().(string)+"/"+row.Get("right").Any().(string))
			}
			if !reflect.DeepEqual(pairs, []string{"L1/R1", "L2/R1", "L3/R3"}) {
				t.Fatalf("composite join result order = %#v", pairs)
			}
			if rightAfter := rightLookups(); rightAfter != rightBefore+1 {
				t.Fatalf("composite join expected one deduplicated right lookup: before=%d after=%d", rightBefore, rightAfter)
			}
			if leftAfter := leftLookups(); leftAfter != leftBefore {
				t.Fatalf("composite join left source unexpectedly probed its index: before=%d after=%d", leftBefore, leftAfter)
			}
		})
	}
}

func TestInfraFAFIndexJoinBTreeRangeCandidateParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env := NewEnvironment()
			leftSchema, err := RegisterStruct[infraIndexRangeJoinLeft](env, "InfraIndexRangeJoinLeft")
			if err != nil {
				t.Fatal(err)
			}
			rightSchema, err := RegisterStruct[infraIndexRangeJoinRight](env, "InfraIndexRangeJoinRight")
			if err != nil {
				t.Fatal(err)
			}
			var left, right RecordStream
			if namedWindow {
				if _, err := CreateNamedWindow(env, "W1", leftSchema, NamedWindowBTreeIndex("left-range", "key", "min")); err != nil {
					t.Fatal(err)
				}
				if _, err := CreateNamedWindow(env, "W2", rightSchema, NamedWindowBTreeIndex("range", "key", "value")); err != nil {
					t.Fatal(err)
				}
				left, right = FromNamedWindow(env, "W1"), FromNamedWindow(env, "W2")
			} else {
				if _, err := CreateTable(env, "W1", []TableColumn{
					PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"), TableColumnOf[int64]("min"), TableColumnOf[int64]("max"),
				}, SecondaryBTreeIndex("left-range", "key", "min")); err != nil {
					t.Fatal(err)
				}
				if _, err := CreateTable(env, "W2", []TableColumn{
					PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"), TableColumnOf[int64]("value"),
				}, SecondaryBTreeIndex("range", "key", "value")); err != nil {
					t.Fatal(err)
				}
				left, right = FromTable(env, "W1"), FromTable(env, "W2")
			}
			engine := NewEngine(env)
			ctx := context.Background()
			leftRows := []infraIndexRangeJoinLeft{
				{ID: "L1", Key: "X", Min: 10, Max: 20},
				{ID: "L2", Key: "X", Min: 20, Max: 30},
				{ID: "L3", Key: "Y", Min: 0, Max: 5},
			}
			rightRows := []infraIndexRangeJoinRight{
				{ID: "R1", Key: "X", Value: 5},
				{ID: "R2", Key: "X", Value: 10},
				{ID: "R3", Key: "X", Value: 15},
				{ID: "R4", Key: "X", Value: 20},
				{ID: "R5", Key: "X", Value: 25},
				{ID: "R6", Key: "Y", Value: 5},
				{ID: "R7", Key: "Y", Value: 6},
			}
			insertLeft := func(row infraIndexRangeJoinLeft) {
				t.Helper()
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, "W1", row); err != nil {
						t.Fatal(err)
					}
					return
				}
				table, _ := engine.Table("W1")
				if _, err := table.Insert(ctx, map[string]any{"id": row.ID, "key": row.Key, "min": row.Min, "max": row.Max}); err != nil {
					t.Fatal(err)
				}
			}
			insertRight := func(row infraIndexRangeJoinRight) {
				t.Helper()
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, "W2", row); err != nil {
						t.Fatal(err)
					}
					return
				}
				table, _ := engine.Table("W2")
				if _, err := table.Insert(ctx, map[string]any{"id": row.ID, "key": row.Key, "value": row.Value}); err != nil {
					t.Fatal(err)
				}
			}
			for _, row := range leftRows {
				insertLeft(row)
			}
			for _, row := range rightRows {
				insertRight(row)
			}

			join := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(AllJoin(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
				OnSourcesCompare(0, Field[any, int64]("min"), 1, Field[any, int64]("value"), JoinLessOrEqual),
				OnSourcesCompare(0, Field[any, int64]("max"), 1, Field[any, int64]("value"), JoinGreaterOrEqual),
			)).Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query()
			plan, err := env.Build(join)
			if err != nil {
				t.Fatal(err)
			}
			selection, ok := plan.IndexPlan().ForSource(1)
			if !ok || selection.IndexName != "range" || selection.Access != IndexAccessRange || selection.Backing != IndexBackingBTree || !reflect.DeepEqual(selection.Columns, []string{"key", "value"}) || !reflect.DeepEqual(selection.MatchedColumns, []string{"key", "value"}) {
				t.Fatalf("range join index plan = %#v", selection)
			}
			var leftLookups, rightLookups func() uint64
			if namedWindow {
				leftWindow, _ := engine.NamedWindow("W1")
				rightWindow, _ := engine.NamedWindow("W2")
				leftLookups = func() uint64 { return leftWindow.state.indexLookups.Load() }
				rightLookups = func() uint64 { return rightWindow.state.indexLookups.Load() }
			} else {
				leftTable, _ := engine.Table("W1")
				rightTable, _ := engine.Table("W2")
				leftLookups = func() uint64 { return leftTable.state.indexLookups.Load() }
				rightLookups = func() uint64 { return rightTable.state.indexLookups.Load() }
			}
			leftBefore, rightBefore := leftLookups(), rightLookups()
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil {
				t.Fatal(err)
			}
			pairs := make([]string, 0, len(result.Results()))
			for _, row := range result.Results() {
				pairs = append(pairs, row.Get("left").Any().(string)+"/"+row.Get("right").Any().(string))
			}
			if !reflect.DeepEqual(pairs, []string{"L1/R2", "L1/R3", "L1/R4", "L2/R4", "L2/R5", "L3/R6"}) {
				t.Fatalf("range join result = %#v", pairs)
			}
			if rightAfter := rightLookups(); rightAfter != rightBefore+1 {
				t.Fatalf("range join expected one batched right lookup: before=%d after=%d", rightBefore, rightAfter)
			}
			if leftAfter := leftLookups(); leftAfter != leftBefore {
				t.Fatalf("range join left source unexpectedly probed its index: before=%d after=%d", leftBefore, leftAfter)
			}

			reversedQuery := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(AllJoin(
				OnSourcesEqual(1, Field[any, string]("key"), 0, Field[any, string]("key")),
				OnSourcesCompare(1, Field[any, int64]("value"), 0, Field[any, int64]("min"), JoinGreaterOrEqual),
				OnSourcesCompare(1, Field[any, int64]("value"), 0, Field[any, int64]("max"), JoinLessOrEqual),
			)).Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query()
			reversedPlan, err := env.Build(reversedQuery)
			if err != nil {
				t.Fatal(err)
			}
			selection, ok = reversedPlan.IndexPlan().ForSource(1)
			if !ok || selection.IndexName != "range" || selection.Access != IndexAccessRange {
				t.Fatalf("reversed range join index plan = %#v", selection)
			}
			leftBefore, rightBefore = leftLookups(), rightLookups()
			reversedResult, err := engine.ExecuteFireAndForget(ctx, reversedPlan)
			if err != nil {
				t.Fatal(err)
			}
			reversedPairs := make([]string, 0, len(reversedResult.Results()))
			for _, row := range reversedResult.Results() {
				reversedPairs = append(reversedPairs, row.Get("left").Any().(string)+"/"+row.Get("right").Any().(string))
			}
			if !reflect.DeepEqual(reversedPairs, []string{"L1/R2", "L1/R3", "L1/R4", "L2/R4", "L2/R5", "L3/R6"}) {
				t.Fatalf("reversed range join result = %#v", reversedPairs)
			}
			if rightAfter := rightLookups(); rightAfter != rightBefore+1 {
				t.Fatalf("reversed range join expected one batched right lookup: before=%d after=%d", rightBefore, rightAfter)
			}
			if leftAfter := leftLookups(); leftAfter != leftBefore {
				t.Fatalf("reversed range join left source unexpectedly probed its index: before=%d after=%d", leftBefore, leftAfter)
			}

			whereQuery := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
			).Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Where(And(
				GreaterOrEqual[int64](JoinField[int64](1, "value"), JoinField[int64](0, "min")),
				LessOrEqual[int64](JoinField[int64](1, "value"), JoinField[int64](0, "max")),
			)).Query()
			wherePlan, err := env.Build(whereQuery)
			if err != nil {
				t.Fatal(err)
			}
			selection, ok = wherePlan.IndexPlan().ForSource(1)
			if !ok || selection.IndexName != "range" || selection.Access != IndexAccessRange {
				t.Fatalf("join-where range index plan = %#v", selection)
			}
			leftBefore, rightBefore = leftLookups(), rightLookups()
			whereResult, err := engine.ExecuteFireAndForget(ctx, wherePlan)
			if err != nil {
				t.Fatal(err)
			}
			wherePairs := make([]string, 0, len(whereResult.Results()))
			for _, row := range whereResult.Results() {
				wherePairs = append(wherePairs, row.Get("left").Any().(string)+"/"+row.Get("right").Any().(string))
			}
			if !reflect.DeepEqual(wherePairs, []string{"L1/R2", "L1/R3", "L1/R4", "L2/R4", "L2/R5", "L3/R6"}) {
				t.Fatalf("join-where range result = %#v", wherePairs)
			}
			if rightAfter := rightLookups(); rightAfter != rightBefore+1 {
				t.Fatalf("join-where range expected one batched right lookup: before=%d after=%d", rightBefore, rightAfter)
			}
			if leftAfter := leftLookups(); leftAfter != leftBefore {
				t.Fatalf("join-where range left source unexpectedly probed its index: before=%d after=%d", leftBefore, leftAfter)
			}

			leftOuterQuery := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(AllJoin(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
				OnSourcesCompare(0, Field[any, int64]("min"), 1, Field[any, int64]("value"), JoinLessOrEqual),
				OnSourcesCompare(0, Field[any, int64]("max"), 1, Field[any, int64]("value"), JoinGreaterOrEqual),
			)).LeftOuter().Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query()
			leftOuterPlan, err := env.Build(leftOuterQuery)
			if err != nil {
				t.Fatal(err)
			}
			selection, ok = leftOuterPlan.IndexPlan().ForSource(1)
			if !ok || selection.IndexName != "range" || selection.Access != IndexAccessRange {
				t.Fatalf("left outer range index plan = %#v", selection)
			}
			leftBefore, rightBefore = leftLookups(), rightLookups()
			leftOuterResult, err := engine.ExecuteFireAndForget(ctx, leftOuterPlan)
			if err != nil {
				t.Fatal(err)
			}
			leftOuterPairs := make([]string, 0, len(leftOuterResult.Results()))
			for _, row := range leftOuterResult.Results() {
				leftOuterPairs = append(leftOuterPairs, row.Get("left").Any().(string)+"/"+row.Get("right").Any().(string))
			}
			if !reflect.DeepEqual(leftOuterPairs, []string{"L1/R2", "L1/R3", "L1/R4", "L2/R4", "L2/R5", "L3/R6"}) {
				t.Fatalf("left outer range result = %#v", leftOuterPairs)
			}
			if leftLookups() != leftBefore || rightLookups() != rightBefore+1 {
				t.Fatalf("left outer range candidate counters = left %d->%d right %d->%d", leftBefore, leftLookups(), rightBefore, rightLookups())
			}

			rightOuterQuery := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(AllJoin(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
				OnSourcesCompare(0, Field[any, int64]("min"), 1, Field[any, int64]("value"), JoinLessOrEqual),
				OnSourcesCompare(0, Field[any, int64]("max"), 1, Field[any, int64]("value"), JoinGreaterOrEqual),
			)).RightOuter().Select(
				SelectFrom(0, "left", JoinField[string](0, "id")),
				SelectFrom(1, "right", JoinField[string](1, "id")),
			).Query()
			rightOuterPlan, err := env.Build(rightOuterQuery)
			if err != nil {
				t.Fatal(err)
			}
			selection, ok = rightOuterPlan.IndexPlan().ForSource(0)
			if !ok || selection.IndexName != "left-range" || selection.Access != IndexAccessRange {
				t.Fatalf("right outer range index plan = %#v", selection)
			}
			leftBefore, rightBefore = leftLookups(), rightLookups()
			rightOuterResult, err := engine.ExecuteFireAndForget(ctx, rightOuterPlan)
			if err != nil {
				t.Fatal(err)
			}
			formatID := func(value Value) string {
				if !value.IsPresent() {
					return "<missing>"
				}
				return value.Any().(string)
			}
			rightOuterPairs := make([]string, 0, len(rightOuterResult.Results()))
			for _, row := range rightOuterResult.Results() {
				rightOuterPairs = append(rightOuterPairs, formatID(row.Get("left"))+"/"+formatID(row.Get("right")))
			}
			if !reflect.DeepEqual(rightOuterPairs, []string{"L1/R2", "L1/R3", "L1/R4", "L2/R4", "L2/R5", "L3/R6", "<missing>/R1", "<missing>/R7"}) {
				t.Fatalf("right outer range result = %#v", rightOuterPairs)
			}
			if leftLookups() != leftBefore+1 || rightLookups() != rightBefore {
				t.Fatalf("right outer range candidate counters = left %d->%d right %d->%d", leftBefore, leftLookups(), rightBefore, rightLookups())
			}
		})
	}
}

func TestInfraFAFPhysicalHashIndexEqualityInAndPreparedParameterParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env, engine, source, indexLookups := setupInfraPhysicalHashStore(t, namedWindow)
			ctx := context.Background()
			field := Field[any, string]("key")

			equalityPlan, err := env.Build(source.Filter(
				EqualOf(field, Literal("X")),
			).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			selection, ok := equalityPlan.IndexPlan().ForSource(0)
			if !ok || selection.IndexName != "by-key" || selection.Access != IndexAccessEquality || selection.Backing != IndexBackingHash {
				t.Fatalf("physical equality index plan = %#v", selection)
			}
			before := indexLookups()
			equalityResult, err := engine.ExecuteFireAndForget(ctx, equalityPlan)
			if err != nil {
				t.Fatal(err)
			}
			if got := infraResultIDs(equalityResult.Results()); !reflect.DeepEqual(got, []string{"L1", "L3"}) {
				t.Fatalf("physical equality result order = %#v, want [L1 L3]", got)
			}
			if after := indexLookups(); after <= before {
				t.Fatalf("physical equality did not use the hash index: before=%d after=%d", before, after)
			}

			inPlan, err := env.Build(source.Filter(
				In[string](field, Literal("Y"), Literal("X")),
			).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			selection, ok = inPlan.IndexPlan().ForSource(0)
			if !ok || selection.IndexName != "by-key" || selection.Access != IndexAccessIn || selection.Backing != IndexBackingHash {
				t.Fatalf("physical IN index plan = %#v", selection)
			}
			before = indexLookups()
			inResult, err := engine.ExecuteFireAndForget(ctx, inPlan)
			if err != nil {
				t.Fatal(err)
			}
			// Probe order is Y then X, but Esper's snapshot result order is the
			// source insertion order and must not depend on IN operand order.
			if got := infraResultIDs(inResult.Results()); !reflect.DeepEqual(got, []string{"L1", "L2", "L3"}) {
				t.Fatalf("physical IN result order = %#v, want [L1 L2 L3]", got)
			}
			if after := indexLookups(); after <= before {
				t.Fatalf("physical IN did not use the hash index: before=%d after=%d", before, after)
			}

			parameterPlan, err := env.Build(source.Filter(
				EqualOf(field, Parameter[string]("wanted")),
			).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := engine.PrepareFireAndForget(parameterPlan)
			if err != nil {
				t.Fatal(err)
			}
			before = indexLookups()
			parameterResult, err := prepared.ExecuteWithParameters(ctx, ParameterValues{"wanted": "Y"})
			if err != nil {
				t.Fatal(err)
			}
			if got := infraResultIDs(parameterResult.Results()); !reflect.DeepEqual(got, []string{"L2"}) {
				t.Fatalf("physical prepared-parameter result = %#v, want [L2]", got)
			}
			if after := indexLookups(); after <= before {
				t.Fatalf("physical prepared parameter did not use the hash index: before=%d after=%d", before, after)
			}
		})
	}
}

func TestInfraFAFPhysicalIndexRangeAndFallbackParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env, engine, source, indexLookups := setupInfraPhysicalHashStore(t, namedWindow)
			ctx := context.Background()
			field := Field[any, string]("key")

			// The planner can select the hinted equality index, but the runtime
			// must not guess at a UDF result. It falls back to the complete
			// snapshot and lets the normal predicate evaluator decide.
			udfKey := Func1[string, string]("identity-key", func(value string) string { return value }, Literal("X"))
			udfPlan, err := env.Build(source.Filter(
				EqualOf(field, udfKey),
			).Select(Alias("id", Field[any, string]("id"))).Query(UseIndex("by-key")))
			if err != nil {
				t.Fatal(err)
			}
			selection, ok := udfPlan.IndexPlan().ForSource(0)
			if !ok || selection.IndexName != "by-key" || selection.Access != IndexAccessEquality {
				t.Fatalf("UDF fallback index plan = %#v", selection)
			}
			before := indexLookups()
			udfResult, err := engine.ExecuteFireAndForget(ctx, udfPlan)
			if err != nil {
				t.Fatal(err)
			}
			if got := infraResultIDs(udfResult.Results()); !reflect.DeepEqual(got, []string{"L1", "L3"}) {
				t.Fatalf("UDF fallback result = %#v, want [L1 L3]", got)
			}
			if after := indexLookups(); after != before {
				t.Fatalf("UDF fallback unexpectedly used the index: before=%d after=%d", before, after)
			}

			// B-tree range planning and the physical ordered candidate path must
			// agree. The returned rows still pass through the ordinary predicate
			// evaluator, so the candidate path cannot change semantics.
			rangeEnv, rangeEngine, rangeSource, rangeLookups := setupInfraPhysicalBTreeStore(t, namedWindow)
			// The B-tree store has an independent environment/source; rebuild
			// the same predicate against it so the fallback assertion is not
			// accidentally coupled to the hash-only fixture above.
			rangePlan, err := rangeEnv.Build(rangeSource.Filter(BetweenOf(
				Field[any, string]("key"), Literal("X"), Literal("Y"),
			)).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			selection, ok = rangePlan.IndexPlan().ForSource(0)
			if !ok || selection.IndexName != "by-key" || selection.Access != IndexAccessRange || selection.Backing != IndexBackingBTree {
				t.Fatalf("range fallback index plan = %#v", selection)
			}
			before = rangeLookups()
			rangeResult, err := rangeEngine.ExecuteFireAndForget(ctx, rangePlan)
			if err != nil {
				t.Fatal(err)
			}
			if got := infraResultIDs(rangeResult.Results()); !reflect.DeepEqual(got, []string{"L1", "L2", "L3"}) {
				t.Fatalf("range fallback result = %#v, want [L1 L2 L3]", got)
			}
			if after := rangeLookups(); after <= before {
				t.Fatalf("B-tree range did not use the ordered index: before=%d after=%d", before, after)
			}

			exclusivePlan, err := rangeEnv.Build(rangeSource.Filter(And(
				Greater[string](Field[any, string]("key"), Literal("X")),
				LessOrEqual[string](Field[any, string]("key"), Literal("Y")),
			)).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			before = rangeLookups()
			exclusiveResult, err := rangeEngine.ExecuteFireAndForget(ctx, exclusivePlan)
			if err != nil {
				t.Fatal(err)
			}
			if got := infraResultIDs(exclusiveResult.Results()); !reflect.DeepEqual(got, []string{"L2"}) {
				t.Fatalf("exclusive/intersected range result = %#v, want [L2]", got)
			}
			if after := rangeLookups(); after <= before {
				t.Fatalf("exclusive/intersected range did not use the ordered index: before=%d after=%d", before, after)
			}

			reversedPlan, err := rangeEnv.Build(rangeSource.Filter(BetweenOf(
				Field[any, string]("key"), Literal("Y"), Literal("X"),
			)).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			before = rangeLookups()
			reversedResult, err := rangeEngine.ExecuteFireAndForget(ctx, reversedPlan)
			if err != nil {
				t.Fatal(err)
			}
			if got := infraResultIDs(reversedResult.Results()); !reflect.DeepEqual(got, []string{"L1", "L2", "L3"}) {
				t.Fatalf("reversed range result = %#v, want [L1 L2 L3]", got)
			}
			if after := rangeLookups(); after <= before {
				t.Fatalf("reversed range did not use the ordered index: before=%d after=%d", before, after)
			}

			// A comparison may put the indexed field on the right-hand side.
			// The runtime must reverse the operator before constructing the
			// candidate bound, while the final predicate still owns semantics.
			reversedOperandPlan, err := rangeEnv.Build(rangeSource.Filter(
				LessOrEqual[string](Literal("Y"), field),
			).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			before = rangeLookups()
			reversedOperandResult, err := rangeEngine.ExecuteFireAndForget(ctx, reversedOperandPlan)
			if err != nil {
				t.Fatal(err)
			}
			if got := infraResultIDs(reversedOperandResult.Results()); !reflect.DeepEqual(got, []string{"L2"}) {
				t.Fatalf("reversed-operand range result = %#v, want [L2]", got)
			}
			if after := rangeLookups(); after <= before {
				t.Fatalf("reversed-operand range did not use the ordered index: before=%d after=%d", before, after)
			}

			// Prepared bounds are extracted at execution time, not at Build time,
			// and must use the same physical range path as literal bounds.
			preparedRangePlan, err := rangeEnv.Build(rangeSource.Filter(BetweenOf(
				field, Parameter[string]("low"), Parameter[string]("high"),
			)).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			preparedRange, err := rangeEngine.PrepareFireAndForget(preparedRangePlan)
			if err != nil {
				t.Fatal(err)
			}
			before = rangeLookups()
			preparedRangeResult, err := preparedRange.ExecuteWithParameters(ctx, ParameterValues{"low": "X", "high": "Y"})
			if err != nil {
				t.Fatal(err)
			}
			if got := infraResultIDs(preparedRangeResult.Results()); !reflect.DeepEqual(got, []string{"L1", "L2", "L3"}) {
				t.Fatalf("prepared range result = %#v, want [L1 L2 L3]", got)
			}
			if after := rangeLookups(); after <= before {
				t.Fatalf("prepared range did not use the ordered index: before=%d after=%d", before, after)
			}

			// A Null/Missing bound is not a safe physical probe. The plan may
			// still advertise a range access path, but execution must preserve
			// the normal false/null predicate semantics through snapshot fallback.
			nullBoundPlan, err := rangeEnv.Build(rangeSource.Filter(BetweenOf(
				field, NullLiteral[string](), Literal("Y"),
			)).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			before = rangeLookups()
			nullBoundResult, err := rangeEngine.ExecuteFireAndForget(ctx, nullBoundPlan)
			if err != nil {
				t.Fatal(err)
			}
			if len(nullBoundResult.Results()) != 0 {
				t.Fatalf("null-bound range result = %#v, want empty", nullBoundResult.Results())
			}
			if after := rangeLookups(); after != before {
				t.Fatalf("null-bound range unexpectedly used the index: before=%d after=%d", before, after)
			}
		})
	}
}

func TestInfraFAFPhysicalBTreeRangeTracksMutation(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env, engine, source, indexLookups := setupInfraPhysicalBTreeStore(t, namedWindow)
			ctx := context.Background()
			plan, err := env.Build(source.Filter(BetweenOf(
				Field[any, string]("key"), Literal("X"), Literal("Y"),
			)).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			run := func(want []string) {
				t.Helper()
				result, runErr := engine.ExecuteFireAndForget(ctx, plan)
				if runErr != nil {
					t.Fatal(runErr)
				}
				if got := infraResultIDs(result.Results()); !reflect.DeepEqual(got, want) {
					t.Fatalf("range mutation result = %#v, want %#v", got, want)
				}
			}
			run([]string{"L1", "L2", "L3"})

			if namedWindow {
				window, ok := engine.NamedWindow("PhysicalStore")
				if !ok {
					t.Fatal("PhysicalStore named window is missing")
				}
				if _, err := window.UpdateWhere(ctx, func(event Event) bool {
					return event.Get("id").Any() == "L1"
				}, func(Event) (any, error) {
					return infraIndexJoinLeft{ID: "L1", Key: "A"}, nil
				}); err != nil {
					t.Fatal(err)
				}
			} else {
				table, ok := engine.Table("PhysicalStore")
				if !ok {
					t.Fatal("PhysicalStore table is missing")
				}
				if _, err := table.Update(ctx, []any{"L1"}, map[string]any{"key": "A"}); err != nil {
					t.Fatal(err)
				}
			}
			run([]string{"L2", "L3"})

			if namedWindow {
				window, _ := engine.NamedWindow("PhysicalStore")
				if _, err := window.UpdateWhere(ctx, func(event Event) bool {
					return event.Get("id").Any() == "L1"
				}, func(Event) (any, error) {
					return infraIndexJoinLeft{ID: "L1", Key: "Y"}, nil
				}); err != nil {
					t.Fatal(err)
				}
			} else {
				table, _ := engine.Table("PhysicalStore")
				if _, err := table.Update(ctx, []any{"L1"}, map[string]any{"key": "Y"}); err != nil {
					t.Fatal(err)
				}
			}
			run([]string{"L1", "L2", "L3"})

			if namedWindow {
				window, _ := engine.NamedWindow("PhysicalStore")
				if _, err := window.DeleteWhere(ctx, func(event Event) bool {
					return event.Get("id").Any() == "L2"
				}); err != nil {
					t.Fatal(err)
				}
			} else {
				table, _ := engine.Table("PhysicalStore")
				if _, _, err := table.Delete(ctx, "L2"); err != nil {
					t.Fatal(err)
				}
			}
			run([]string{"L1", "L3"})

			before := indexLookups()
			if namedWindow {
				window, _ := engine.NamedWindow("PhysicalStore")
				if _, err := window.DeleteWhere(ctx, func(Event) bool { return true }); err != nil {
					t.Fatal(err)
				}
			} else {
				table, _ := engine.Table("PhysicalStore")
				if _, err := table.Clear(ctx); err != nil {
					t.Fatal(err)
				}
			}
			run([]string{})
			if after := indexLookups(); after <= before {
				t.Fatalf("empty range after clear did not use the maintained index: before=%d after=%d", before, after)
			}
		})
	}
}

func TestInfraFAFPhysicalCompositeBTreePrefixRangeParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env := NewEnvironment()
			schema, err := RegisterStruct[infraIndexJoinRight](env, "InfraPhysicalCompositeRangeEvent")
			if err != nil {
				t.Fatal(err)
			}
			var source RecordStream
			if namedWindow {
				if _, err := CreateNamedWindow(env, "CompositeStore", schema,
					NamedWindowRetention(KeepAll()), NamedWindowBTreeIndex("by-key-amount", "key", "amount")); err != nil {
					t.Fatal(err)
				}
				source = FromNamedWindow(env, "CompositeStore")
			} else {
				if _, err := CreateTable(env, "CompositeStore", []TableColumn{
					PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"), TableColumnOf[int64]("amount"),
				}, SecondaryBTreeIndex("by-key-amount", "key", "amount")); err != nil {
					t.Fatal(err)
				}
				source = FromTable(env, "CompositeStore")
			}
			engine := NewEngine(env)
			ctx := context.Background()
			rows := []infraIndexJoinRight{
				{ID: "R1", Key: "X", Amount: 10},
				{ID: "R2", Key: "X", Amount: 20},
				{ID: "R3", Key: "Y", Amount: 30},
			}
			for _, row := range rows {
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, "CompositeStore", row); err != nil {
						t.Fatal(err)
					}
					continue
				}
				table, ok := engine.Table("CompositeStore")
				if !ok {
					t.Fatal("CompositeStore table is missing")
				}
				if _, err := table.Insert(ctx, map[string]any{"id": row.ID, "key": row.Key, "amount": row.Amount}); err != nil {
					t.Fatal(err)
				}
			}

			plan, err := env.Build(source.Filter(And(
				EqualOf(Field[any, string]("key"), Literal("X")),
				Greater[int64](Field[any, int64]("amount"), Literal[int64](15)),
			)).Select(Alias("id", Field[any, string]("id"))).Query())
			if err != nil {
				t.Fatal(err)
			}
			selection, ok := plan.IndexPlan().ForSource(0)
			if !ok || selection.IndexName != "by-key-amount" || selection.Access != IndexAccessRange || !reflect.DeepEqual(selection.MatchedColumns, []string{"key", "amount"}) {
				t.Fatalf("composite range index plan = %#v", selection)
			}
			var indexLookups func() uint64
			if namedWindow {
				window, ok := engine.NamedWindow("CompositeStore")
				if !ok {
					t.Fatal("CompositeStore named window is missing")
				}
				indexLookups = func() uint64 { return window.state.indexLookups.Load() }
			} else {
				table, ok := engine.Table("CompositeStore")
				if !ok {
					t.Fatal("CompositeStore table is missing")
				}
				indexLookups = func() uint64 { return table.state.indexLookups.Load() }
			}
			before := indexLookups()
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil {
				t.Fatal(err)
			}
			if got := infraResultIDs(result.Results()); !reflect.DeepEqual(got, []string{"R2"}) {
				t.Fatalf("composite prefix range result = %#v, want [R2]", got)
			}
			if after := indexLookups(); after <= before {
				t.Fatalf("composite prefix range did not use the ordered index: before=%d after=%d", before, after)
			}
		})
	}
}

func setupInfraPhysicalHashStore(t *testing.T, namedWindow bool) (*Environment, *Engine, RecordStream, func() uint64) {
	t.Helper()
	env := NewEnvironment()
	schema, err := RegisterStruct[infraIndexJoinLeft](env, "InfraPhysicalHashEvent")
	if err != nil {
		t.Fatal(err)
	}
	var source RecordStream
	if namedWindow {
		if _, err := CreateNamedWindow(env, "PhysicalStore", schema,
			NamedWindowRetention(KeepAll()), NamedWindowIndex("by-key", "key")); err != nil {
			t.Fatal(err)
		}
		source = FromNamedWindow(env, "PhysicalStore")
	} else {
		if _, err := CreateTable(env, "PhysicalStore", []TableColumn{
			PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"),
		}, SecondaryIndex("by-key", "key")); err != nil {
			t.Fatal(err)
		}
		source = FromTable(env, "PhysicalStore")
	}
	engine := NewEngine(env)
	ctx := context.Background()
	rows := []infraIndexJoinLeft{
		{ID: "L1", Key: "X"},
		{ID: "L2", Key: "Y"},
		{ID: "L3", Key: "X"},
	}
	for _, row := range rows {
		if namedWindow {
			if err := engine.InsertNamedWindow(ctx, "PhysicalStore", row); err != nil {
				t.Fatal(err)
			}
			continue
		}
		table, ok := engine.Table("PhysicalStore")
		if !ok {
			t.Fatal("PhysicalStore table is missing")
		}
		if _, err := table.Insert(ctx, map[string]any{"id": row.ID, "key": row.Key}); err != nil {
			t.Fatal(err)
		}
	}
	if namedWindow {
		window, ok := engine.NamedWindow("PhysicalStore")
		if !ok {
			t.Fatal("PhysicalStore named window is missing")
		}
		return env, engine, source, func() uint64 { return window.state.indexLookups.Load() }
	}
	table, ok := engine.Table("PhysicalStore")
	if !ok {
		t.Fatal("PhysicalStore table is missing")
	}
	return env, engine, source, func() uint64 { return table.state.indexLookups.Load() }
}

func setupInfraPhysicalBTreeStore(t *testing.T, namedWindow bool) (*Environment, *Engine, RecordStream, func() uint64) {
	t.Helper()
	env := NewEnvironment()
	schema, err := RegisterStruct[infraIndexJoinLeft](env, "InfraPhysicalBTreeEvent")
	if err != nil {
		t.Fatal(err)
	}
	var source RecordStream
	if namedWindow {
		if _, err := CreateNamedWindow(env, "PhysicalStore", schema,
			NamedWindowRetention(KeepAll()), NamedWindowBTreeIndex("by-key", "key")); err != nil {
			t.Fatal(err)
		}
		source = FromNamedWindow(env, "PhysicalStore")
	} else {
		if _, err := CreateTable(env, "PhysicalStore", []TableColumn{
			PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"),
		}, SecondaryBTreeIndex("by-key", "key")); err != nil {
			t.Fatal(err)
		}
		source = FromTable(env, "PhysicalStore")
	}
	engine := NewEngine(env)
	ctx := context.Background()
	for _, row := range []infraIndexJoinLeft{
		{ID: "L1", Key: "X"},
		{ID: "L2", Key: "Y"},
		{ID: "L3", Key: "X"},
	} {
		if namedWindow {
			if err := engine.InsertNamedWindow(ctx, "PhysicalStore", row); err != nil {
				t.Fatal(err)
			}
			continue
		}
		table, ok := engine.Table("PhysicalStore")
		if !ok {
			t.Fatal("PhysicalStore table is missing")
		}
		if _, err := table.Insert(ctx, map[string]any{"id": row.ID, "key": row.Key}); err != nil {
			t.Fatal(err)
		}
	}
	if namedWindow {
		window, ok := engine.NamedWindow("PhysicalStore")
		if !ok {
			t.Fatal("PhysicalStore named window is missing")
		}
		return env, engine, source, func() uint64 { return window.state.indexLookups.Load() }
	}
	table, ok := engine.Table("PhysicalStore")
	if !ok {
		t.Fatal("PhysicalStore table is missing")
	}
	return env, engine, source, func() uint64 { return table.state.indexLookups.Load() }
}

func infraResultIDs(results []Result) []string {
	ids := make([]string, 0, len(results))
	for _, result := range results {
		ids = append(ids, result.Get("id").Any().(string))
	}
	return ids
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
