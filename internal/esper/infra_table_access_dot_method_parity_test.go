package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type infraTableDotMyBean struct{}

func (infraTableDotMyBean) GetMyProperty() string { return "x" }

func infraTableDotMinuteOfHourFootprint() []DateTimeMethodFootprint {
	return []DateTimeMethodFootprint{{Input: DateTimeInputEpochMillis}}
}

func infraTableDotMinuteOfHourFactory(DateTimePluginFactoryContext) DateTimePluginOps {
	return DateTimePluginOpsFunc(func(input Value, _ []Value) (Value, error) {
		millis, err := As[int64](input)
		if err != nil {
			return Null(), err
		}
		return Present(int64(time.UnixMilli(millis).UTC().Minute())), nil
	})
}

func infraTableDotAfterFootprint() []DateTimeMethodFootprint {
	return []DateTimeMethodFootprint{{Input: DateTimeInputAny, Parameters: []DateTimeMethodParameter{
		{Description: "threshold", Expected: DateTimeParameterSpecific, Type: reflect.TypeOf(int64(0))},
	}}}
}

func infraTableDotAfterFactory(DateTimePluginFactoryContext) DateTimePluginOps {
	return DateTimePluginOpsFunc(func(input Value, arguments []Value) (Value, error) {
		millis, err := As[int64](input)
		if err != nil {
			return Null(), err
		}
		threshold, err := As[int64](arguments[0])
		if err != nil {
			return Null(), err
		}
		return Present(millis > threshold), nil
	})
}

// TestInfraTableAccessDotMethodPlainPropertyParity mirrors Java
// InfraTableAccessDotMethod.InfraPlainPropDatetimeAndEnumerationAndMethod.
// A PopulateEvent object-array merge materializes datetime, bean, bean-array,
// event and event-array table columns; the trigger read applies getMinuteOfHour,
// getMyProperty, takeLast, nested p0 and selectFrom(i => i.p0) in one row.
func TestInfraTableAccessDotMethodPlainPropertyParity(t *testing.T) {
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
				testInfraTableAccessDotMethodPlainProperty(t, grouped)
			})
		}
	}
}

func testInfraTableAccessDotMethodPlainProperty(t *testing.T, grouped bool) {
	base := time.Date(2002, time.May, 30, 9, 55, 0, 0, time.UTC)
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	myEventSchema, err := RegisterObjectArray(env, "MyEvent", []FieldSpec{
		FieldDef("p0", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterObjectArray(env, "PopulateEvent", []FieldSpec{
		FieldDef("key", reflect.TypeOf("")),
		FieldDef("ts", reflect.TypeOf(int64(0))),
		FieldDef("mb", reflect.TypeOf(infraTableDotMyBean{})),
		FieldDef("mbarr", reflect.TypeOf([]infraTableDotMyBean{})),
		FieldDef("me", reflect.TypeOf(Event{})),
		FieldDef("mearr", reflect.TypeOf([]Event{})),
	}); err != nil {
		t.Fatal(err)
	}
	if err := RegisterDateTimePlugin(env, "getMinuteOfHour", infraTableDotMinuteOfHourFootprint(), infraTableDotMinuteOfHourFactory); err != nil {
		t.Fatal(err)
	}
	columns := make([]TableColumn, 0, 6)
	if grouped {
		columns = append(columns, PrimaryKeyColumn[string]("key"))
	}
	columns = append(columns,
		TableColumnOf[int64]("ts"),
		TableColumnOf[infraTableDotMyBean]("mb"),
		TableColumnOf[[]infraTableDotMyBean]("mbarr"),
		TableColumnOf[Event]("me"),
		TableColumnOf[[]Event]("mearr"),
	)
	if _, err := CreateTable(env, "varaggPWD", columns); err != nil {
		t.Fatal(err)
	}

	minute := DateTimePluginRef[int64](env, "getMinuteOfHour", TableField[int64]("ts"))
	methodValue := Method[string](TableField[infraTableDotMyBean]("mb"), "GetMyProperty")
	beans := TableField[[]infraTableDotMyBean]("mbarr")
	takeLast := EnumTakeLast[infraTableDotMyBean](beans, 1)
	nestedEvent := TableField[Event]("me")
	nestedP0 := Property[string](nestedEvent, "p0")
	events := TableField[[]Event]("mearr")
	selectedP0 := EnumSelect[Event, string](events, NestedField[string](EnumElement[Event](), "p0"))

	trigger := From[infraTableGroupedTrigger](env, "SupportBean_S0")
	var readPlan Plan
	if grouped {
		readPlan, err = env.Build(OnEvent(trigger).SelectFromTable("varaggPWD",
			[]Expr{Field[infraTableGroupedTrigger, string]("p00")},
			Alias("c0", minute),
			Alias("c1", methodValue),
			Alias("c2", takeLast),
			Alias("c3", nestedP0),
			Alias("c4", selectedP0),
		).Query(StatementName("dot-method-plain-read")))
	} else {
		readPlan, err = env.Build(OnEvent(trigger).SelectFromTableWhere("varaggPWD", Literal(true),
			Alias("c0", minute),
			Alias("c1", methodValue),
			Alias("c2", takeLast),
			Alias("c3", nestedP0),
			Alias("c4", selectedP0),
		).Query(StatementName("dot-method-plain-read")))
	}
	if err != nil {
		t.Fatal(err)
	}

	populate := FromAny(env, "PopulateEvent")
	assignments := []TableAssignment{
		SetColumn("ts", Field[any, int64]("ts")),
		SetColumn("mb", Field[any, infraTableDotMyBean]("mb")),
		SetColumn("mbarr", Field[any, []infraTableDotMyBean]("mbarr")),
		SetColumn("me", Field[any, Event]("me")),
		SetColumn("mearr", Field[any, []Event]("mearr")),
	}
	if grouped {
		assignments = append([]TableAssignment{SetColumn("key", Field[any, string]("key"))}, assignments...)
	}
	var keys []Expr
	if grouped {
		keys = []Expr{Field[any, string]("key")}
	}
	mergePlan, err := env.Build(OnRecord(populate).MergeIntoTableWhen("varaggPWD", keys,
		WhenNotMatchedAny(assignments...),
	).Query(StatementName("dot-method-plain-merge")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	readDeployment, err := engine.Deploy(context.Background(), readPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), mergePlan); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	var rows []Row
	if _, err := readDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("dot-method plain result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	me, err := ParseObjectArray(myEventSchema, []any{"p0value"}, base)
	if err != nil {
		t.Fatal(err)
	}
	meOne, err := ParseObjectArray(myEventSchema, []any{"0_p0"}, base)
	if err != nil {
		t.Fatal(err)
	}
	meTwo, err := ParseObjectArray(myEventSchema, []any{"1_p0"}, base)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendObjectArray(context.Background(), "PopulateEvent", []any{
		"E1", base.UnixMilli(), infraTableDotMyBean{}, []infraTableDotMyBean{{}, {}}, me, []Event{meOne, meTwo},
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), infraTableGroupedTrigger{P00: "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("dot-method plain rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if got := row.Get("c0").Any(); got != int64(55) {
		t.Fatalf("dot-method plain c0 = %#v, want 55", got)
	}
	if got := row.Get("c1").Any(); got != "x" {
		t.Fatalf("dot-method plain c1 = %#v, want x", got)
	}
	taken, ok := row.Get("c2").Any().([]infraTableDotMyBean)
	if !ok || len(taken) != 1 {
		t.Fatalf("dot-method plain c2 = %#v, want one bean", row.Get("c2").Any())
	}
	if got := row.Get("c3").Any(); got != "p0value" {
		t.Fatalf("dot-method plain c3 = %#v, want p0value", got)
	}
	selected, ok := row.Get("c4").Any().([]string)
	if !ok || !reflect.DeepEqual(selected, []string{"0_p0", "1_p0"}) {
		t.Fatalf("dot-method plain c4 = %#v, want [0_p0 1_p0]", row.Get("c4").Any())
	}
}

// TestInfraTableAccessDotMethodAggregationParity mirrors Java
// InfraTableAccessDotMethod.InfraAggDatetimeAndEnumerationAndMethod. A table
// stores lastever(long) and full-event window access; the trigger read applies
// the registered after date-time plugin and countOf over the window.
func TestInfraTableAccessDotMethodAggregationParity(t *testing.T) {
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
				testInfraTableAccessDotMethodAggregation(t, grouped)
			})
		}
	}
}

func testInfraTableAccessDotMethodAggregation(t *testing.T, grouped bool) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedMultiBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if err := RegisterDateTimePlugin(env, "after", infraTableDotAfterFootprint(), infraTableDotAfterFactory); err != nil {
		t.Fatal(err)
	}
	columns := make([]TableColumn, 0, 3)
	if grouped {
		columns = append(columns, PrimaryKeyColumn[string]("key"))
	}
	columns = append(columns,
		TableColumnOf[int64]("a1"),
		TableColumnOf[WindowAccessValue[infraTableGroupedMultiBean]]("a2"),
	)
	if _, err := CreateTable(env, "varaggWDE", columns); err != nil {
		t.Fatal(err)
	}

	source := From[infraTableGroupedMultiBean](env, "SupportBean").Window(TimeWindow(10 * time.Second))
	window := WindowAccessBy[infraTableGroupedMultiBean](EventValue[infraTableGroupedMultiBean]())
	longValue := Field[infraTableGroupedMultiBean, int64]("longPrimitive")
	var aggregatePlan Plan
	var err error
	if grouped {
		key := Field[infraTableGroupedMultiBean, string]("theString")
		aggregatePlan, err = env.Build(source.GroupBy(key).Select(
			Alias("key", key),
			Alias("a1", LastEver[int64](longValue)),
			Alias("a2", window),
		).IntoTable("varaggWDE", StatementName("dot-method-aggregate")))
	} else {
		aggregatePlan, err = env.Build(source.Aggregate(
			Alias("a1", LastEver[int64](longValue)),
			Alias("a2", window),
		).IntoTable("varaggWDE", StatementName("dot-method-aggregate")))
	}
	if err != nil {
		t.Fatal(err)
	}

	access := TableField[WindowAccessValue[infraTableGroupedMultiBean]]("a2")
	values := Method[[]infraTableGroupedMultiBean](access, "Values")
	afterValue := DateTimePluginRef[bool](env, "after", TableField[int64]("a1"), DateTimeArgument(Literal[int64](150)))
	countValue := EnumCount[infraTableGroupedMultiBean](values)
	trigger := From[infraTableGroupedTrigger](env, "SupportBean_S0")
	var readPlan Plan
	if grouped {
		readPlan, err = env.Build(OnEvent(trigger).SelectFromTable("varaggWDE",
			[]Expr{Field[infraTableGroupedTrigger, string]("p00")},
			Alias("c0", afterValue),
			Alias("c1", countValue),
		).Query(StatementName("dot-method-aggregate-read")))
	} else {
		readPlan, err = env.Build(OnEvent(trigger).SelectFromTableWhere("varaggWDE", Literal(true),
			Alias("c0", afterValue),
			Alias("c1", countValue),
		).Query(StatementName("dot-method-aggregate-read")))
	}
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
				t.Fatalf("dot-method aggregate result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	read := func(wantAfter bool, wantCount int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), infraTableGroupedTrigger{P00: "E1"}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("dot-method aggregate read produced no row")
		}
		row := rows[len(rows)-1]
		if row.Get("c0").Any() != wantAfter || row.Get("c1").Any() != wantCount {
			t.Fatalf("dot-method aggregate read = %#v, want %v/%d", row.AsMap(), wantAfter, wantCount)
		}
	}

	if err := engine.SendEvent(context.Background(), infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100}); err != nil {
		t.Fatal(err)
	}
	read(false, 1)
	if err := engine.SendEvent(context.Background(), infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 20, LongPrimitive: 200}); err != nil {
		t.Fatal(err)
	}
	read(true, 2)
}

// TestInfraTableAccessDotMethodNestedParity mirrors Java
// InfraTableAccessDotMethod.InfraNestedDotMethod. A table holds a length(2)
// full-event window; nested last(*).intPrimitive, window(*).countOf() and
// window(intPrimitive).take(1) are composed through explicit projections.
func TestInfraTableAccessDotMethodNestedParity(t *testing.T) {
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
				testInfraTableAccessDotMethodNested(t, grouped)
			})
		}
	}
}

func testInfraTableAccessDotMethodNested(t *testing.T, grouped bool) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableGroupedMultiBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	columns := make([]TableColumn, 0, 2)
	if grouped {
		columns = append(columns, PrimaryKeyColumn[string]("key"))
	}
	columns = append(columns, TableColumnOf[WindowAccessValue[infraTableGroupedMultiBean]]("windowSupportBean"))
	if _, err := CreateTable(env, "varaggNDM", columns); err != nil {
		t.Fatal(err)
	}

	source := From[infraTableGroupedMultiBean](env, "SupportBean").Window(LengthWindow(2))
	window := WindowAccessBy[infraTableGroupedMultiBean](EventValue[infraTableGroupedMultiBean]())
	var aggregatePlan Plan
	var err error
	if grouped {
		key := Field[infraTableGroupedMultiBean, string]("theString")
		aggregatePlan, err = env.Build(source.GroupBy(key).Select(
			Alias("key", key),
			Alias("windowSupportBean", window),
		).IntoTable("varaggNDM", StatementName("dot-method-nested-aggregate")))
	} else {
		aggregatePlan, err = env.Build(source.Aggregate(
			Alias("windowSupportBean", window),
		).IntoTable("varaggNDM", StatementName("dot-method-nested-aggregate")))
	}
	if err != nil {
		t.Fatal(err)
	}

	access := TableField[WindowAccessValue[infraTableGroupedMultiBean]]("windowSupportBean")
	lastBean := Method[infraTableGroupedMultiBean](access, "Last")
	lastInt := Property[int64](lastBean, "intPrimitive")
	values := Method[[]infraTableGroupedMultiBean](access, "Values")
	countValue := EnumCount[infraTableGroupedMultiBean](values)
	intValues := EnumSelect[infraTableGroupedMultiBean, int64](values, EnumField[infraTableGroupedMultiBean, int64]("intPrimitive"))
	taken := EnumTake[int64](intValues, 1)

	trigger := From[infraTableGroupedTrigger](env, "SupportBean_S0")
	var readPlan Plan
	if grouped {
		readPlan, err = env.Build(OnEvent(trigger).SelectFromTable("varaggNDM",
			[]Expr{Field[infraTableGroupedTrigger, string]("p00")},
			Alias("c0", lastInt),
			Alias("c1", countValue),
			Alias("c2", taken),
		).Query(StatementName("dot-method-nested-read")))
	} else {
		readPlan, err = env.Build(OnEvent(trigger).SelectFromTableWhere("varaggNDM", Literal(true),
			Alias("c0", lastInt),
			Alias("c1", countValue),
			Alias("c2", taken),
		).Query(StatementName("dot-method-nested-read")))
	}
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
				t.Fatalf("dot-method nested result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	read := func(wantLast int64, wantCount int64, wantTaken []int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), infraTableGroupedTrigger{P00: "E1"}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("dot-method nested read produced no row")
		}
		row := rows[len(rows)-1]
		if row.Get("c0").Any() != wantLast || row.Get("c1").Any() != wantCount {
			t.Fatalf("dot-method nested read = %#v, want %d/%d", row.AsMap(), wantLast, wantCount)
		}
		gotTaken, ok := row.Get("c2").Any().([]int64)
		if !ok || !reflect.DeepEqual(gotTaken, wantTaken) {
			t.Fatalf("dot-method nested taken = %#v, want %#v", row.Get("c2").Any(), wantTaken)
		}
	}

	if err := engine.SendEvent(context.Background(), infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	read(10, 1, []int64{10})
	if err := engine.SendEvent(context.Background(), infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	read(20, 2, []int64{10})
	if err := engine.SendEvent(context.Background(), infraTableGroupedMultiBean{TheString: "E1", IntPrimitive: 30}); err != nil {
		t.Fatal(err)
	}
	read(30, 2, []int64{20})
}
