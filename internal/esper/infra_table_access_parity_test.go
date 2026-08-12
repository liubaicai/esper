package esper

import (
	"context"
	"reflect"
	"testing"
)

type infraTableFilterS0 struct {
	ID int64 `esper:"id"`
}

type infraTableFilterBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int64  `esper:"intPrimitive"`
}

type infraTableWindowEvent struct {
	C0 int64 `esper:"c0"`
}

type infraTableWindowTrigger struct {
	ID int64 `esper:"id"`
}

type infraTableIndexedBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int64  `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type infraTableIndexedTrigger struct {
	ID int64 `esper:"id"`
}

type infraTableGroupedTrigger struct {
	ID  int64  `esper:"id"`
	P00 string `esper:"p00"`
}

// TestInfraTableAccessFilterBehaviorParity mirrors Java
// InfraTableAccessCore.InfraFilterBehavior:
//
//	create table varaggFB (total count(*))
//	into table varaggFB select count(*) from SupportBean_S0
//	select * from SupportBean(varaggFB.total = intPrimitive)
//
// The Go form keeps the same dataflow explicit: an aggregate statement
// materializes the table, and a typed scalar subquery reads its current row in
// the ordinary event filter.
func TestInfraTableAccessFilterBehaviorParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableFilterS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableFilterBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varaggFB", []TableColumn{
		TableColumnOf[int64]("total"),
	}); err != nil {
		t.Fatal(err)
	}

	countPlan, err := env.Build(
		From[infraTableFilterS0](env, "SupportBean_S0").
			Aggregate(Alias("total", CountAll())).
			IntoTable("varaggFB", StatementName("varaggFB-count")),
	)
	if err != nil {
		t.Fatal(err)
	}
	countDeployment, err := NewEngine(env).Deploy(context.Background(), countPlan)
	if err != nil {
		t.Fatal(err)
	}
	engine := countDeployment.engine
	if engine == nil {
		t.Fatal("count deployment has no engine")
	}

	total := SubqueryValue[int64](
		FromTable(env, "varaggFB"),
		Field[any, int64]("total"),
	)
	filterPlan, err := env.Build(
		From[infraTableFilterBean](env, "SupportBean").
			Filter(Equal[int64](total, Field[infraTableFilterBean, int64]("intPrimitive"))).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	filterDeployment, err := engine.Deploy(context.Background(), filterPlan)
	if err != nil {
		t.Fatal(err)
	}

	var matched []int64
	if _, err := filterDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("filter result is not an event: %#v", result)
			}
			matched = append(matched, event.Get("intPrimitive").Any().(int64))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := engine.SendEvent(ctx, infraTableFilterS0{ID: 0}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterS0{ID: 0}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterBean{TheString: "E1", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterBean{TheString: "E1", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}

	want := []int64{1, 2}
	if len(matched) != len(want) {
		t.Fatalf("table filter matches = %v, want %v", matched, want)
	}
	for index, value := range want {
		if matched[index] != value {
			t.Fatalf("table filter match[%d] = %d, want %d (all=%v)", index, matched[index], value, matched)
		}
	}

	table, ok := engine.Table("varaggFB")
	if !ok {
		t.Fatal("varaggFB table is missing")
	}
	rows, err := table.Snapshot(ctx)
	if err != nil || len(rows) != 1 || rows[0].Get("total").Any() != int64(2) {
		t.Fatalf("varaggFB snapshot = %#v, err=%v", rows, err)
	}
}

// TestInfraTableAccessUngroupedWindowAndSumParity mirrors Java
// InfraTableAccessCoreUnGroupedWindowAndSum. The table has one unkeyed row
// containing both a length-window access value and a running sum; a trigger
// stream reads that row after each insertion.
func TestInfraTableAccessUngroupedWindowAndSumParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableWindowEvent](env, "MyEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableWindowTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "windowAndTotal", []TableColumn{
		TableColumnOf[WindowAccessValue[infraTableWindowEvent]]("thewindow"),
		TableColumnOf[int64]("thetotal"),
	}); err != nil {
		t.Fatal(err)
	}

	c0 := Field[infraTableWindowEvent, int64]("c0")
	aggregatePlan, err := env.Build(
		From[infraTableWindowEvent](env, "MyEvent").
			Window(LengthWindow(2)).
			Aggregate(
				Alias("thewindow", WindowAccessBy[infraTableWindowEvent](EventValue[infraTableWindowEvent]())),
				Alias("thetotal", Sum[int64](c0)),
			).
			IntoTable("windowAndTotal", StatementName("window-and-total")),
	)
	if err != nil {
		t.Fatal(err)
	}

	triggerPlan, err := env.Build(
		OnEvent(From[infraTableWindowTrigger](env, "SupportBean_S0")).
			SelectFromTableWhere("windowAndTotal", Literal(true),
				Alias("thewindow", TableField[WindowAccessValue[infraTableWindowEvent]]("thewindow")),
				Alias("thetotal", TableField[int64]("thetotal")),
			).
			Query(StatementName("read-window-and-total")),
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
	rows := make([]Row, 0, 3)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("table access result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendAndRead := func(value, triggerID int64, wantValues []int64, wantTotal int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), infraTableWindowEvent{C0: value}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), infraTableWindowTrigger{ID: triggerID}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("table access trigger produced no row")
		}
		row := rows[len(rows)-1]
		if row.Get("thetotal").Any() != wantTotal {
			t.Fatalf("table total after %d = %v, want %d", value, row.Get("thetotal"), wantTotal)
		}
		access, ok := row.Get("thewindow").Any().(WindowAccessValue[infraTableWindowEvent])
		if !ok {
			t.Fatalf("table window type = %T", row.Get("thewindow").Any())
		}
		values := access.Values()
		if len(values) != len(wantValues) {
			t.Fatalf("table window after %d = %#v, want %#v", value, values, wantValues)
		}
		for index, want := range wantValues {
			if values[index].C0 != want {
				t.Fatalf("table window after %d index %d = %d, want %d", value, index, values[index].C0, want)
			}
		}
	}

	sendAndRead(10, 0, []int64{10}, 10)
	sendAndRead(20, 1, []int64{10, 20}, 30)
	sendAndRead(30, 2, []int64{20, 30}, 50)
}

// TestInfraTableAccessIntegerIndexedPropertyLookAlikeParity mirrors Java
// InfraTableAccessCore.InfraIntegerIndexedPropertyLookAlike. The Java
// varaggIIP[1] table access is expressed as an explicit typed primary-key
// lookup in the Go trigger chain; the remaining projections keep the row,
// access value and indexed window navigation visible in the plan.
func TestInfraTableAccessIntegerIndexedPropertyLookAlikeParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableIndexedBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableIndexedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varaggIIP", []TableColumn{
		PrimaryKeyColumn[int64]("key"),
		TableColumnOf[WindowAccessValue[infraTableIndexedBean]]("myevents"),
	}); err != nil {
		t.Fatal(err)
	}

	key := Field[infraTableIndexedBean, int64]("intPrimitive")
	window := WindowAccessBy[infraTableIndexedBean](EventValue[infraTableIndexedBean]())
	aggregatePlan, err := env.Build(
		From[infraTableIndexedBean](env, "SupportBean").
			Window(LengthWindow(3)).
			GroupBy(key).
			Select(
				Alias("key", key),
				Alias("myevents", window),
			).
			IntoTable("varaggIIP", StatementName("varagg-iip")),
	)
	if err != nil {
		t.Fatal(err)
	}

	tableField := TableField[WindowAccessValue[infraTableIndexedBean]]("myevents")
	values := Method[[]infraTableIndexedBean](tableField, "Values")
	triggerPlan, err := env.Build(
		OnEvent(From[infraTableIndexedTrigger](env, "SupportBean_S0")).
			SelectFromTable("varaggIIP", []Expr{Literal[int64](1)},
				Alias("c0", EventValue[Event]()),
				Alias("c1", tableField),
				Alias("c2", Method[infraTableIndexedBean](tableField, "Last")),
				Alias("c3", ArrayAt[infraTableIndexedBean](values, Subtract[int64](ArraySize(values), Literal[int64](2)))),
			).
			Query(StatementName("read-varagg-iip")),
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
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("integer-indexed table result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	e1 := infraTableIndexedBean{TheString: "E1", IntPrimitive: 1, LongPrimitive: 10}
	e2 := infraTableIndexedBean{TheString: "E2", IntPrimitive: 1, LongPrimitive: 20}
	ctx := context.Background()
	if err := engine.SendEvent(ctx, e1); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, e2); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableIndexedTrigger{ID: 0}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("integer-indexed table result count = %d, want 1", len(rows))
	}

	row := rows[0]
	c0, ok := row.Get("c0").Any().(Event)
	if !ok {
		t.Fatalf("c0 type = %T, want Event", row.Get("c0").Any())
	}
	c0Access, ok := c0.Get("myevents").Any().(WindowAccessValue[infraTableIndexedBean])
	if !ok {
		t.Fatalf("c0.myevents type = %T", c0.Get("myevents").Any())
	}
	c1, ok := row.Get("c1").Any().(WindowAccessValue[infraTableIndexedBean])
	if !ok {
		t.Fatalf("c1 type = %T", row.Get("c1").Any())
	}
	for name, access := range map[string]WindowAccessValue[infraTableIndexedBean]{"c0.myevents": c0Access, "c1": c1} {
		got := access.Values()
		if len(got) != 2 || got[0] != e1 || got[1] != e2 {
			t.Fatalf("%s = %#v, want [%#v %#v]", name, got, e1, e2)
		}
	}
	if got, ok := row.Get("c2").Any().(infraTableIndexedBean); !ok || got != e2 {
		t.Fatalf("c2 = %#v, want %#v", row.Get("c2").Any(), e2)
	}
	if got, ok := row.Get("c3").Any().(infraTableIndexedBean); !ok || got != e1 {
		t.Fatalf("c3 = %#v, want %#v", row.Get("c3").Any(), e1)
	}
}

// TestInfraTableAccessTopLevelReadUngroupedParity mirrors Java
// InfraTableAccessCore.InfraTopLevelReadUnGrouped. The top-level table row is
// represented as an explicit Go map projection, while the object-array event
// remains a schema-bound Event inside the window access value.
func TestInfraTableAccessTopLevelReadUngroupedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterObjectArray(env, "MyEventOATLRU", []FieldSpec{
		FieldDef("c0", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableWindowTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "windowAndTotalTLRUG", []TableColumn{
		TableColumnOf[WindowAccessValue[Event]]("thewindow"),
		TableColumnOf[int64]("thetotal"),
	}); err != nil {
		t.Fatal(err)
	}

	window := WindowAccessBy[Event](EventValue[Event]())
	aggregatePlan, err := env.Build(
		FromAny(env, "MyEventOATLRU").
			Window(LengthWindow(2)).
			Aggregate(
				Alias("thewindow", window),
				Alias("thetotal", Sum[int64](Field[any, int64]("c0"))),
			).
			IntoTable("windowAndTotalTLRUG", StatementName("tlrug-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}

	tableWindow := TableField[WindowAccessValue[Event]]("thewindow")
	rowProjection := StructOf(
		Alias("thewindow", Method[[]Event](tableWindow, "Values")),
		Alias("thetotal", TableField[int64]("thetotal")),
	)
	triggerPlan, err := env.Build(
		OnEvent(From[infraTableWindowTrigger](env, "SupportBean_S0")).
			SelectFromTableWhere("windowAndTotalTLRUG", Literal(true), Alias("val0", rowProjection)).
			Query(StatementName("tlrug-read")),
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
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("top-level table result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	sendAndRead := func(value, triggerID int64, wantValues []int64, wantTotal int64) {
		t.Helper()
		if err := engine.SendObjectArray(ctx, "MyEventOATLRU", []any{value}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(ctx, infraTableWindowTrigger{ID: triggerID}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("top-level table trigger produced no row")
		}
		row := rows[len(rows)-1]
		val0, ok := row.Get("val0").Any().(map[string]any)
		if !ok {
			t.Fatalf("val0 type = %T", row.Get("val0").Any())
		}
		if got := val0["thetotal"]; got != wantTotal {
			t.Fatalf("top-level table total after %d = %#v, want %d", value, got, wantTotal)
		}
		events, ok := val0["thewindow"].([]Event)
		if !ok {
			t.Fatalf("top-level table window type = %T", val0["thewindow"])
		}
		if len(events) != len(wantValues) {
			t.Fatalf("top-level table window after %d = %d events, want %d", value, len(events), len(wantValues))
		}
		for index, want := range wantValues {
			if got := events[index].Get("c0").Any(); got != want {
				t.Fatalf("top-level table window after %d index %d = %#v, want %d", value, index, got, want)
			}
		}
	}

	sendAndRead(10, 0, []int64{10}, 10)
	sendAndRead(20, 1, []int64{10, 20}, 30)
	sendAndRead(30, 2, []int64{20, 30}, 50)
}

// TestInfraTableAccessTopLevelReadGroupedTwoKeysParity mirrors Java
// InfraTableAccessCore.InfraTopLevelReadGrouped2Keys. The two primary-key
// columns are looked up explicitly and a missing group is represented by the
// same present-but-null map fields used by the ungrouped table projection.
func TestInfraTableAccessTopLevelReadGroupedTwoKeysParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterObjectArray(env, "MyEventOA", []FieldSpec{
		FieldDef("c0", reflect.TypeOf(int64(0))),
		FieldDef("c1", reflect.TypeOf("")),
		FieldDef("c2", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableGroupedTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "windowAndTotalTLP2K", []TableColumn{
		PrimaryKeyColumn[int64]("keyi"),
		PrimaryKeyColumn[string]("keys"),
		TableColumnOf[WindowAccessValue[Event]]("thewindow"),
		TableColumnOf[int64]("thetotal"),
	}); err != nil {
		t.Fatal(err)
	}

	keyInt := Field[any, int64]("c0")
	keyString := Field[any, string]("c1")
	window := WindowAccessBy[Event](EventValue[Event]())
	aggregatePlan, err := env.Build(
		FromAny(env, "MyEventOA").
			Window(LengthWindow(2)).
			GroupBy(keyInt, keyString).
			Select(
				Alias("keyi", keyInt),
				Alias("keys", keyString),
				Alias("thewindow", window),
				Alias("thetotal", Sum[int64](Field[any, int64]("c2"))),
			).
			IntoTable("windowAndTotalTLP2K", StatementName("tlp2k-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}

	tableWindow := TableField[WindowAccessValue[Event]]("thewindow")
	rowProjection := StructOf(
		Alias("thewindow", Method[[]Event](tableWindow, "Values")),
		Alias("thetotal", TableField[int64]("thetotal")),
	)
	triggerPlan, err := env.Build(
		OnEvent(From[infraTableGroupedTrigger](env, "SupportBean_S0")).
			SelectFromTable("windowAndTotalTLP2K", []Expr{
				Field[infraTableGroupedTrigger, int64]("id"),
				Field[infraTableGroupedTrigger, string]("p00"),
			}, Alias("val0", rowProjection)).
			Query(StatementName("tlp2k-read")),
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
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("grouped top-level table result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	read := func(event []any, id int64, key string) map[string]any {
		t.Helper()
		if event != nil {
			if err := engine.SendObjectArray(ctx, "MyEventOA", event); err != nil {
				t.Fatal(err)
			}
		}
		if err := engine.SendEvent(ctx, infraTableGroupedTrigger{ID: id, P00: key}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("grouped top-level table trigger produced no row")
		}
		value := rows[len(rows)-1].Get("val0").Any()
		projected, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("grouped val0 type = %T", value)
		}
		return projected
	}
	assertGroup := func(projected map[string]any, wantValues []int64, wantTotal any) {
		t.Helper()
		if projected["thetotal"] != wantTotal {
			t.Fatalf("grouped table total = %#v, want %#v", projected["thetotal"], wantTotal)
		}
		events, ok := projected["thewindow"].([]Event)
		if wantValues == nil {
			if projected["thewindow"] != nil {
				t.Fatalf("missing grouped table window = %#v, want nil", projected["thewindow"])
			}
			return
		}
		if !ok || len(events) != len(wantValues) {
			t.Fatalf("grouped table window = %#v, want %d events", projected["thewindow"], len(wantValues))
		}
		for index, want := range wantValues {
			if got := events[index].Get("c0").Any(); got != want {
				t.Fatalf("grouped table window index %d = %#v, want %d", index, got, want)
			}
		}
	}

	assertGroup(read([]any{int64(10), "G1", int64(100)}, 10, "G1"), []int64{10}, int64(100))
	assertGroup(read([]any{int64(20), "G2", int64(200)}, 20, "G2"), []int64{20}, int64(200))
	missing := read([]any{int64(20), "G2", int64(300)}, 10, "G1")
	assertGroup(missing, nil, nil)
	assertGroup(read(nil, 20, "G2"), []int64{20, 20}, int64(500))
}
