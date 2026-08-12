package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// TestInfraTableAccessAggregationStateGroupedThreeKeyParity mirrors Java
// InfraTableAccessAggregationState.InfraTableAccessGroupedThreeKey. The
// aggregate row is keyed by string/int/long and projects sum(double) and
// count(*). A trigger event supplies the first two keys while the third key
// remains the literal 100L from the Java expression.
func TestInfraTableAccessAggregationStateGroupedThreeKeyParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedMultiBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varTotal", []TableColumn{
		PrimaryKeyColumn[string]("key0"),
		PrimaryKeyColumn[int64]("key1"),
		PrimaryKeyColumn[int64]("key2"),
		TableColumnOf[float64]("total"),
		TableColumnOf[int64]("cnt"),
	}); err != nil {
		t.Fatal(err)
	}

	groupString := Field[infraTableGroupedMultiBean, string]("theString")
	groupInt := Field[infraTableGroupedMultiBean, int64]("intPrimitive")
	groupLong := Field[infraTableGroupedMultiBean, int64]("longPrimitive")
	aggregatePlan, err := env.Build(
		From[infraTableGroupedMultiBean](env, "SupportBean").
			GroupBy(groupString, groupInt, groupLong).
			Select(
				Alias("key0", groupString),
				Alias("key1", groupInt),
				Alias("key2", groupLong),
				Alias("total", Sum[float64](Field[infraTableGroupedMultiBean, float64]("doublePrimitive"))),
				Alias("cnt", CountAll()),
			).
			IntoTable("varTotal", StatementName("agg-state-three-key")),
	)
	if err != nil {
		t.Fatal(err)
	}

	triggerPlan, err := env.Build(
		OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
			SelectFromTable("varTotal", []Expr{
				Field[infraTableGroupedTrigger, string]("p00"),
				Field[infraTableGroupedTrigger, int64]("id"),
				Literal[int64](100),
			},
				Alias("c0", TableField[float64]("total")),
				Alias("c1", TableField[int64]("cnt")),
			).
			Query(StatementName("agg-state-three-key-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("aggregation-state three-key result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	read := func(wantTotal float64, wantCount int64) {
		t.Helper()
		if err := engine.SendEvent(ctx, infraTableGroupedTrigger{ID: 10, P00: "E1"}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("aggregation-state three-key read produced no row")
		}
		row := rows[len(rows)-1]
		if row.Get("c0").Any() != wantTotal || row.Get("c1").Any() != wantCount {
			t.Fatalf("aggregation-state three-key read = %#v, want %v/%v", row.AsMap(), wantTotal, wantCount)
		}
	}

	if err := engine.SendEvent(ctx, infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100, DoublePrimitive: 1000}); err != nil {
		t.Fatal(err)
	}
	read(1000, 1)
	if err := engine.SendEvent(ctx, infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100, DoublePrimitive: 1001}); err != nil {
		t.Fatal(err)
	}
	read(2001, 2)
}

// TestInfraTableAccessAggregationStateGroupedMixedParity mirrors Java
// InfraTableAccessAggregationState.InfraTableAccessGroupedMixed. The grouped
// table holds count, count-distinct, window access and sum columns in one row,
// and a typed table lookup verifies both scalar and access-aggregate state.
func TestInfraTableAccessAggregationStateGroupedMixedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedMultiBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varMyAggState", []TableColumn{
		PrimaryKeyColumn[string]("key"),
		TableColumnOf[int64]("c0"),
		TableColumnOf[int64]("c1"),
		TableColumnOf[WindowAccessValue[infraTableGroupedMultiBean]]("c2"),
		TableColumnOf[int64]("c3"),
	}); err != nil {
		t.Fatal(err)
	}

	groupKey := Field[infraTableGroupedMultiBean, string]("theString")
	intValue := Field[infraTableGroupedMultiBean, int64]("intPrimitive")
	longValue := Field[infraTableGroupedMultiBean, int64]("longPrimitive")
	window := WindowAccessBy[infraTableGroupedMultiBean](EventValue[infraTableGroupedMultiBean]())
	aggregatePlan, err := env.Build(
		From[infraTableGroupedMultiBean](env, "SupportBean").
			Window(LengthWindow(3)).
			GroupBy(groupKey).
			Select(
				Alias("key", groupKey),
				Alias("c0", CountAll()),
				Alias("c1", CountDistinct[int64](intValue)),
				Alias("c2", window),
				Alias("c3", Sum[int64](longValue)),
			).
			IntoTable("varMyAggState", StatementName("agg-state-grouped-mixed")),
	)
	if err != nil {
		t.Fatal(err)
	}

	triggerPlan, err := env.Build(
		OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
			SelectFromTable("varMyAggState", []Expr{Field[infraTableGroupedTrigger, string]("p00")},
				Alias("c0", TableField[int64]("c0")),
				Alias("c1", TableField[int64]("c1")),
				Alias("c2", TableField[WindowAccessValue[infraTableGroupedMultiBean]]("c2")),
				Alias("c3", TableField[int64]("c3")),
			).
			Query(StatementName("agg-state-grouped-mixed-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("aggregation-state mixed result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	read := func(key string, wantCount, wantDistinct, wantSum any, wantWindow []infraTableGroupedMultiBean) {
		t.Helper()
		if err := engine.SendEvent(ctx, infraTableGroupedTrigger{P00: key}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("aggregation-state mixed read produced no row")
		}
		row := rows[len(rows)-1]
		if row.Get("c0").Any() != wantCount || row.Get("c1").Any() != wantDistinct || row.Get("c3").Any() != wantSum {
			t.Fatalf("aggregation-state mixed read %s = %#v, want %v/%v/%v", key, row.AsMap(), wantCount, wantDistinct, wantSum)
		}
		if wantWindow == nil {
			if row.Get("c2").State() != ValueNull {
				t.Fatalf("aggregation-state mixed missing window %s = %#v, want null", key, row.Get("c2"))
			}
			return
		}
		access, ok := row.Get("c2").Any().(WindowAccessValue[infraTableGroupedMultiBean])
		if !ok || !reflect.DeepEqual(access.Values(), wantWindow) {
			t.Fatalf("aggregation-state mixed window %s = %#v, want %#v", key, row.Get("c2").Any(), wantWindow)
		}
	}

	b1 := infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100}
	b2 := infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 11, LongPrimitive: 101}
	b3 := infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10, LongPrimitive: 102}
	for _, event := range []infraTableGroupedMultiBean{b1, b2, b3} {
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	read("E1", int64(3), int64(2), int64(303), []infraTableGroupedMultiBean{b1, b2, b3})
	read("E2", nil, nil, nil, nil)

	b4 := infraTableGroupedMultiBean{TheString: "E2", IntPrimitive: 20, LongPrimitive: 200}
	if err := engine.SendEvent(ctx, b4); err != nil {
		t.Fatal(err)
	}
	read("E2", int64(1), int64(1), int64(200), []infraTableGroupedMultiBean{b4})
}

// TestInfraTableAccessAggregationStateAggShareParity mirrors Java
// InfraTableAccessAggregationState.InfraAccessAggShare. The into-table
// statement materializes a shared window access aggregate; a second table
// reader observes the same retained SupportBean values after each event.
func TestInfraTableAccessAggregationStateAggShareParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedMultiBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varaggShare", []TableColumn{
		TableColumnOf[WindowAccessValue[infraTableGroupedMultiBean]]("mywin"),
	}); err != nil {
		t.Fatal(err)
	}

	window := WindowAccessBy[infraTableGroupedMultiBean](EventValue[infraTableGroupedMultiBean]())
	aggregatePlan, err := env.Build(
		From[infraTableGroupedMultiBean](env, "SupportBean").
			Window(TimeWindow(10*time.Second)).
			Aggregate(Alias("mywin", window)).
			IntoTable("varaggShare", StatementName("agg-state-share-into")),
	)
	if err != nil {
		t.Fatal(err)
	}
	readPlan, err := env.Build(
		OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
			SelectFromTableWhere("varaggShare", Literal(true),
				Alias("c0", TableField[WindowAccessValue[infraTableGroupedMultiBean]]("mywin")),
			).
			Query(StatementName("agg-state-share-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), readPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("aggregation-state share result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	read := func(want []infraTableGroupedMultiBean) {
		t.Helper()
		if err := engine.SendEvent(ctx, infraTableGroupedTrigger{ID: 1}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("aggregation-state share read produced no row")
		}
		row := rows[len(rows)-1]
		access, ok := row.Get("c0").Any().(WindowAccessValue[infraTableGroupedMultiBean])
		if !ok || !reflect.DeepEqual(access.Values(), want) {
			t.Fatalf("aggregation-state share window = %#v, want %#v", row.Get("c0").Any(), want)
		}
	}

	b1 := infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10}
	if err := engine.SendEvent(ctx, b1); err != nil {
		t.Fatal(err)
	}
	read([]infraTableGroupedMultiBean{b1})

	b2 := infraTableGroupedMultiBean{TheString: "E2", IntPrimitive: 20}
	if err := engine.SendEvent(ctx, b2); err != nil {
		t.Fatal(err)
	}
	read([]infraTableGroupedMultiBean{b1, b2})
}

// TestInfraTableAccessAggregationStateNestedMultivalueAccessParity mirrors
// Java InfraTableAccessAggregationState.InfraNestedMultivalueAccess for both
// grouped and ungrouped tables. A single table column stores the full-event
// window access while a second column stores the scalar intPrimitive window;
// the table read projects first/last/window forms of both access states.
func TestInfraTableAccessAggregationStateNestedMultivalueAccessParity(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		for _, soda := range []bool{false, true} {
			name := "ungrouped"
			if grouped {
				name = "grouped"
			}
			if soda {
				name += "-soda"
			}
			t.Run(name, func(t *testing.T) {
				testInfraTableAccessAggregationStateNestedMultivalueAccess(t, grouped)
			})
		}
	}
}

func testInfraTableAccessAggregationStateNestedMultivalueAccess(t *testing.T, grouped bool) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedMultiBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	tableName := "varaggNested"
	if grouped {
		tableName = "varaggNestedGrouped"
	}
	var columns []TableColumn
	if grouped {
		columns = append(columns, PrimaryKeyColumn[string]("key"))
	}
	columns = append(columns,
		TableColumnOf[WindowAccessValue[infraTableGroupedMultiBean]]("windowSupportBean"),
		TableColumnOf[WindowAccessValue[int64]]("windowInt"),
	)
	if _, err := CreateTable(env, tableName, columns); err != nil {
		t.Fatal(err)
	}

	eventWindow := WindowAccessBy[infraTableGroupedMultiBean](EventValue[infraTableGroupedMultiBean]())
	intWindow := WindowAccessBy[int64](Field[infraTableGroupedMultiBean, int64]("intPrimitive"))
	source := From[infraTableGroupedMultiBean](env, "SupportBean").Window(LengthWindow(2))
	var aggregatePlan Plan
	var err error
	if grouped {
		key := Field[infraTableGroupedMultiBean, string]("theString")
		aggregatePlan, err = env.Build(
			source.GroupBy(key).
				Select(
					Alias("key", key),
					Alias("windowSupportBean", eventWindow),
					Alias("windowInt", intWindow),
				).
				IntoTable(tableName, StatementName("nested-multivalue-aggregate")),
		)
	} else {
		aggregatePlan, err = env.Build(
			source.Aggregate(
				Alias("windowSupportBean", eventWindow),
				Alias("windowInt", intWindow),
			).
				IntoTable(tableName, StatementName("nested-multivalue-aggregate")),
		)
	}
	if err != nil {
		t.Fatal(err)
	}

	var triggerPlan Plan
	if grouped {
		triggerPlan, err = env.Build(
			OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
				SelectFromTable(tableName, []Expr{Field[infraTableGroupedTrigger, string]("p00")},
					Alias("c0", TableField[WindowAccessValue[infraTableGroupedMultiBean]]("windowSupportBean")),
					Alias("c1", TableField[WindowAccessValue[int64]]("windowInt")),
				).
				Query(StatementName("nested-multivalue-read")),
		)
	} else {
		triggerPlan, err = env.Build(
			OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
				SelectFromTableWhere(tableName, Literal(true),
					Alias("c0", TableField[WindowAccessValue[infraTableGroupedMultiBean]]("windowSupportBean")),
					Alias("c1", TableField[WindowAccessValue[int64]]("windowInt")),
				).
				Query(StatementName("nested-multivalue-read")),
		)
	}
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("nested multivalue result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	read := func(wantEvents []infraTableGroupedMultiBean, wantInts []int64) {
		t.Helper()
		if err := engine.SendEvent(ctx, infraTableGroupedTrigger{P00: "E1"}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("nested multivalue read produced no row")
		}
		row := rows[len(rows)-1]
		events, ok := row.Get("c0").Any().(WindowAccessValue[infraTableGroupedMultiBean])
		if !ok || !reflect.DeepEqual(events.Values(), wantEvents) {
			t.Fatalf("nested multivalue full window = %#v, want %#v", row.Get("c0").Any(), wantEvents)
		}
		ints, ok := row.Get("c1").Any().(WindowAccessValue[int64])
		if !ok || !reflect.DeepEqual(ints.Values(), wantInts) {
			t.Fatalf("nested multivalue scalar window = %#v, want %#v", row.Get("c1").Any(), wantInts)
		}
	}

	b1 := infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10}
	b2 := infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 20}
	b3 := infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 30}
	for _, event := range []infraTableGroupedMultiBean{b1, b2} {
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	read([]infraTableGroupedMultiBean{b1, b2}, []int64{10, 20})
	if err := engine.SendEvent(ctx, b3); err != nil {
		t.Fatal(err)
	}
	read([]infraTableGroupedMultiBean{b2, b3}, []int64{20, 30})
}
