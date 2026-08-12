package esper

import (
	"context"
	"testing"
)

type infraFAFEvent struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int64  `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

func TestInfraFAFReadOnlySnapshotParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
			env, engine, source := setupInfraFAFEventStore(t, namedWindow)
			ctx := context.Background()

			wildcard, err := env.Build(source.Query(StatementName("infra-faf-wildcard")))
			if err != nil {
				t.Fatal(err)
			}
			wildcardResult, err := engine.ExecuteFireAndForget(ctx, wildcard)
			if err != nil || len(wildcardResult.Results()) != 4 {
				t.Fatalf("wildcard result = %#v, err=%v", wildcardResult.Results(), err)
			}
			if got := wildcardResult.Results()[0].Get("theString").Any(); got != "E1" {
				t.Fatalf("wildcard first row = %#v, want E1", got)
			}

			filtered, err := env.Build(source.Filter(
				Greater[int64](Field[any, int64]("intPrimitive"), Literal[int64](10)),
			).Query(StatementName("infra-faf-filter")))
			if err != nil {
				t.Fatal(err)
			}
			filteredResult, err := engine.ExecuteFireAndForget(ctx, filtered)
			if err != nil || len(filteredResult.Results()) != 3 {
				t.Fatalf("filtered result = %#v, err=%v", filteredResult.Results(), err)
			}

			distinct, err := env.Build(source.Select(
				Alias("intPrimitive", Field[any, int64]("intPrimitive")),
			).Query(WithDistinct(), OrderBy(Ascending(ResultField[int64]("intPrimitive")))))
			if err != nil {
				t.Fatal(err)
			}
			distinctResult, err := engine.ExecuteFireAndForget(ctx, distinct)
			if err != nil || len(distinctResult.Results()) != 4 {
				t.Fatalf("distinct result = %#v, err=%v", distinctResult.Results(), err)
			}

			sum := Sum[int64](Field[any, int64]("intPrimitive"))
			countPlan, err := env.Build(source.Aggregate(Alias("count", CountAll())).Query())
			if err != nil {
				t.Fatal(err)
			}
			countResult, err := engine.ExecuteFireAndForget(ctx, countPlan)
			if err != nil || len(countResult.Results()) != 1 || countResult.Results()[0].Get("count").Any() != int64(4) {
				t.Fatalf("count result = %#v, err=%v", countResult.Results(), err)
			}

			allPlan, err := env.Build(source.Aggregate(Alias("total", sum)).Query())
			if err != nil {
				t.Fatal(err)
			}
			allResult, err := engine.ExecuteFireAndForget(ctx, allPlan)
			if err != nil || len(allResult.Results()) != 1 || allResult.Results()[0].Get("total").Any() != int64(100) {
				t.Fatalf("ungrouped aggregate result = %#v, err=%v", allResult.Results(), err)
			}

			groupedPlan, err := env.Build(source.GroupBy(Field[any, string]("theString")).Select(
				Alias("theString", Field[any, string]("theString")),
				Alias("total", sum),
			).Query(OrderBy(Ascending(ResultField[string]("theString")))))
			if err != nil {
				t.Fatal(err)
			}
			groupedResult, err := engine.ExecuteFireAndForget(ctx, groupedPlan)
			if err != nil || len(groupedResult.Results()) != 3 {
				t.Fatalf("grouped aggregate result = %#v, err=%v", groupedResult.Results(), err)
			}
			if got := groupedResult.Results()[0].Get("total").Any(); got != int64(40) {
				t.Fatalf("grouped first total = %#v, want 40", got)
			}

			eventAggregatePlan, err := env.Build(source.Aggregate(
				Alias("theString", Field[any, string]("theString")),
				Alias("total", sum),
			).Query())
			if err != nil {
				t.Fatal(err)
			}
			eventAggregateResult, err := engine.ExecuteFireAndForget(ctx, eventAggregatePlan)
			if err != nil || len(eventAggregateResult.Results()) != 4 {
				t.Fatalf("row-for-event aggregate result = %#v, err=%v", eventAggregateResult.Results(), err)
			}
			strings := make(map[string]int)
			for _, row := range eventAggregateResult.Results() {
				if got := row.Get("total").Any(); got != int64(100) {
					t.Fatalf("row-for-event aggregate total = %#v, want 100", got)
				}
				name, ok := row.Get("theString").Any().(string)
				if !ok {
					t.Fatalf("row-for-event aggregate event name = %#v", row.Get("theString").Any())
				}
				strings[name]++
			}
			if got := strings["E1"]; got != 2 || strings["E2"] != 1 || strings["E3"] != 1 {
				t.Fatalf("row-for-event aggregate names = %#v, want E1x2/E2x1/E3x1", strings)
			}

			inPlan, err := env.Build(source.Filter(And(
				In[string](Field[any, string]("theString"), Literal("E2"), Literal("E3")),
				In[int64](Field[any, int64]("intPrimitive"), Literal[int64](20), Literal[int64](30)),
			)).Query())
			if err != nil {
				t.Fatal(err)
			}
			inResult, err := engine.ExecuteFireAndForget(ctx, inPlan)
			if err != nil || len(inResult.Results()) != 1 {
				t.Fatalf("in-clause result = %#v, err=%v", inResult.Results(), err)
			}
			if got := inResult.Results()[0].Get("longPrimitive").Any(); got != int64(200) {
				t.Fatalf("in-clause first longPrimitive = %#v, want 200", got)
			}
		})
	}
}

func TestInfraFAFJoinSnapshotParity(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		namedLeft := mask&1 != 0
		namedRight := mask&2 != 0
		t.Run(joinStoreName(namedLeft, namedRight), func(t *testing.T) {
			env, engine, left, right := setupInfraFAFJoinStore(t, namedLeft, namedRight)
			join := JoinMany(JoinRecordSource(left), JoinRecordSource(right)).On(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
			)
			query, err := env.Build(join.Select(
				SelectFrom(0, "leftID", JoinField[string](0, "id")),
				SelectFrom(1, "rightAmount", JoinField[int64](1, "amount")),
			).Query(StatementName("infra-faf-join")))
			if err != nil {
				t.Fatal(err)
			}
			result, err := engine.ExecuteFireAndForget(context.Background(), query)
			if err != nil || len(result.Results()) != 2 {
				t.Fatalf("join result = %#v, err=%v", result.Results(), err)
			}
			if got := result.Results()[0].Get("leftID").Any(); got != "L-X" {
				t.Fatalf("join first leftID = %#v, want L-X", got)
			}

			whereQuery, err := env.Build(join.Select(
				SelectFrom(0, "leftID", JoinField[string](0, "id")),
				SelectFrom(1, "rightAmount", JoinField[int64](1, "amount")),
			).Where(GreaterOrEqual[int64](JoinField[int64](1, "amount"), Literal[int64](20))).Query())
			if err != nil {
				t.Fatal(err)
			}
			whereResult, err := engine.ExecuteFireAndForget(context.Background(), whereQuery)
			if err != nil || len(whereResult.Results()) != 1 || whereResult.Results()[0].Get("leftID").Any() != "L-Y" {
				t.Fatalf("join where result = %#v, err=%v", whereResult.Results(), err)
			}
		})
	}
}

func TestInfraFAFThreeStreamJoinSnapshotParity(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		t.Run(joinThreeStoreName(mask), func(t *testing.T) {
			env, engine, sources := setupInfraFAFThreeStore(t, mask)
			joined := JoinMany(
				JoinRecordSource(sources[0]),
				JoinRecordSource(sources[1]),
				JoinRecordSource(sources[2]),
			).On(
				OnSourcesEqual(0, Field[any, string]("key"), 1, Field[any, string]("key")),
				OnSourcesEqual(1, Field[any, string]("key"), 2, Field[any, string]("key")),
			).Select(
				SelectFrom(0, "v0", JoinField[string](0, "value")),
				SelectFrom(1, "v1", JoinField[string](1, "value")),
				SelectFrom(2, "v2", JoinField[string](2, "value")),
			).Query(StatementName("infra-faf-three-join"))
			plan, err := env.Build(joined)
			if err != nil {
				t.Fatal(err)
			}
			result, err := engine.ExecuteFireAndForget(context.Background(), plan)
			if err != nil || len(result.Results()) != 1 {
				t.Fatalf("three-stream join result = %#v, err=%v", result.Results(), err)
			}
			for name, want := range map[string]string{"v0": "A0", "v1": "A1", "v2": "A2"} {
				if got := result.Results()[0].Get(name).Any(); got != want {
					t.Fatalf("three-stream %s = %#v, want %s", name, got, want)
				}
			}
		})
	}
}

func setupInfraFAFEventStore(t *testing.T, namedWindow bool) (*Environment, *Engine, RecordStream) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[infraFAFEvent](env, "InfraFAFEvent"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("InfraFAFEvent")
	if !ok {
		t.Fatal("InfraFAFEvent schema is missing")
	}
	var source RecordStream
	if namedWindow {
		if _, err := CreateNamedWindow(env, "InfraFAF", schema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
		source = FromNamedWindow(env, "InfraFAF")
	} else {
		if _, err := CreateTable(env, "InfraFAF", []TableColumn{
			PrimaryKeyColumn[string]("theString"),
			PrimaryKeyColumn[int64]("intPrimitive"),
			TableColumnOf[int64]("longPrimitive"),
		}); err != nil {
			t.Fatal(err)
		}
		source = FromTable(env, "InfraFAF")
	}
	engine := NewEngine(env)
	ctx := context.Background()
	var table *Table
	if !namedWindow {
		var ok bool
		table, ok = engine.Table("InfraFAF")
		if !ok {
			t.Fatal("InfraFAF table is missing")
		}
	}
	events := []infraFAFEvent{
		{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100},
		{TheString: "E2", IntPrimitive: 20, LongPrimitive: 200},
		{TheString: "E1", IntPrimitive: 30, LongPrimitive: 300},
		{TheString: "E3", IntPrimitive: 40, LongPrimitive: 400},
	}
	for _, event := range events {
		if namedWindow {
			if err := engine.InsertNamedWindow(ctx, "InfraFAF", event); err != nil {
				t.Fatal(err)
			}
		} else if _, err := table.Insert(ctx, map[string]any{
			"theString": event.TheString, "intPrimitive": event.IntPrimitive, "longPrimitive": event.LongPrimitive,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return env, engine, source
}

func setupInfraFAFJoinStore(t *testing.T, namedLeft, namedRight bool) (*Environment, *Engine, RecordStream, RecordStream) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterMap(env, "InfraFAFJoinLeft", []FieldSpec{FieldDef("id", typeOf[string]()), FieldDef("key", typeOf[string]())}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "InfraFAFJoinRight", []FieldSpec{FieldDef("key", typeOf[string]()), FieldDef("amount", typeOf[int64]())}); err != nil {
		t.Fatal(err)
	}
	left := setupInfraFAFJoinSource(t, env, "InfraFAFJoinLeft", "InfraFAFJoinLeftStore", namedLeft, []TableColumn{
		PrimaryKeyColumn[string]("id"), TableColumnOf[string]("key"),
	})
	right := setupInfraFAFJoinSource(t, env, "InfraFAFJoinRight", "InfraFAFJoinRightStore", namedRight, []TableColumn{
		PrimaryKeyColumn[string]("key"), TableColumnOf[int64]("amount"),
	})
	engine := NewEngine(env)
	ctx := context.Background()
	leftTable, leftOK := engine.Table("InfraFAFJoinLeftStore")
	if !namedLeft && !leftOK {
		t.Fatal("InfraFAFJoinLeftStore table is missing")
	}
	rightTable, rightOK := engine.Table("InfraFAFJoinRightStore")
	if !namedRight && !rightOK {
		t.Fatal("InfraFAFJoinRightStore table is missing")
	}
	if namedLeft {
		if err := engine.InsertNamedWindow(ctx, "InfraFAFJoinLeftStore", map[string]any{"id": "L-X", "key": "X"}); err != nil {
			t.Fatal(err)
		}
		if err := engine.InsertNamedWindow(ctx, "InfraFAFJoinLeftStore", map[string]any{"id": "L-Y", "key": "Y"}); err != nil {
			t.Fatal(err)
		}
	} else {
		for _, row := range []map[string]any{{"id": "L-X", "key": "X"}, {"id": "L-Y", "key": "Y"}} {
			if _, err := leftTable.Insert(ctx, row); err != nil {
				t.Fatal(err)
			}
		}
	}
	if namedRight {
		for _, row := range []map[string]any{{"key": "X", "amount": int64(10)}, {"key": "Y", "amount": int64(20)}} {
			if err := engine.InsertNamedWindow(ctx, "InfraFAFJoinRightStore", row); err != nil {
				t.Fatal(err)
			}
		}
	} else {
		for _, row := range []map[string]any{{"key": "X", "amount": int64(10)}, {"key": "Y", "amount": int64(20)}} {
			if _, err := rightTable.Insert(ctx, row); err != nil {
				t.Fatal(err)
			}
		}
	}
	return env, engine, left, right
}

func setupInfraFAFJoinSource(t *testing.T, env *Environment, schemaName, storeName string, namedWindow bool, columns []TableColumn) RecordStream {
	t.Helper()
	if namedWindow {
		schema, ok := env.Schema(schemaName)
		if !ok {
			t.Fatalf("schema %s is missing", schemaName)
		}
		if _, err := CreateNamedWindow(env, storeName, schema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
		return FromNamedWindow(env, storeName)
	}
	if _, err := CreateTable(env, storeName, columns); err != nil {
		t.Fatal(err)
	}
	return FromTable(env, storeName)
}

func setupInfraFAFThreeStore(t *testing.T, mask int) (*Environment, *Engine, []RecordStream) {
	t.Helper()
	env := NewEnvironment()
	for index := 0; index < 3; index++ {
		if _, err := RegisterMap(env, "InfraFAFThreeSchema"+string(rune('0'+index)), []FieldSpec{
			FieldDef("key", typeOf[string]()), FieldDef("value", typeOf[string]()),
		}); err != nil {
			t.Fatal(err)
		}
	}
	for index := 0; index < 3; index++ {
		name := "InfraFAFThree" + string(rune('0'+index))
		if mask&(1<<index) != 0 {
			schema, ok := env.Schema("InfraFAFThreeSchema" + string(rune('0'+index)))
			if !ok {
				t.Fatalf("three-stream schema %d is missing", index)
			}
			if _, err := CreateNamedWindow(env, name, schema, NamedWindowRetention(KeepAll())); err != nil {
				t.Fatal(err)
			}
		} else if _, err := CreateTable(env, name, []TableColumn{
			PrimaryKeyColumn[string]("key"), TableColumnOf[string]("value"),
		}); err != nil {
			t.Fatal(err)
		}
	}
	engine := NewEngine(env)
	ctx := context.Background()
	tables := make(map[string]*Table, 3)
	for index := 0; index < 3; index++ {
		name := "InfraFAFThree" + string(rune('0'+index))
		if table, ok := engine.Table(name); ok {
			tables[name] = table
		} else if mask&(1<<index) == 0 {
			t.Fatal("missing table " + name)
		}
	}
	sources := make([]RecordStream, 3)
	for index := 0; index < 3; index++ {
		name := "InfraFAFThree" + string(rune('0'+index))
		if mask&(1<<index) != 0 {
			sources[index] = FromNamedWindow(env, name)
			if err := engine.InsertNamedWindow(ctx, name, map[string]any{"key": "A", "value": "A" + string(rune('0'+index))}); err != nil {
				t.Fatal(err)
			}
		} else {
			sources[index] = FromTable(env, name)
			if _, err := tables[name].Insert(ctx, map[string]any{"key": "A", "value": "A" + string(rune('0'+index))}); err != nil {
				t.Fatal(err)
			}
		}
	}
	return env, engine, sources
}

func joinStoreName(left, right bool) string {
	return map[bool]string{true: "nw", false: "table"}[left] + "-" + map[bool]string{true: "nw", false: "table"}[right]
}

func joinThreeStoreName(mask int) string {
	return "mask-" + string(rune('0'+mask))
}
