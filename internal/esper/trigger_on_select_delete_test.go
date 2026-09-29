package esper

import (
	"context"
	"testing"
)

// Engine coverage for InfraNWTableOnSelectWDelete: the on-trigger
// select-and-delete form projects the matched store rows (here an ungrouped
// sum over the match set) and removes exactly those rows in the same trigger
// action, mirroring Java's `on S0 select and delete <projection> from Store
// where <predicate>`.

type seldelBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

type seldelS0 struct {
	ID  int32  `esper:"id"`
	P00 string `esper:"p00"`
}

func subscribeSelectDeleteBatches(t *testing.T, stmt *Statement) *[]ResultBatch {
	t.Helper()
	batches := subscribeOnSetVarCapture(t, stmt)
	return batches
}

func selectDeleteRows(t *testing.T, stmt *Statement, fields ...string) [][]any {
	t.Helper()
	result, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	rows := make([][]any, 0, len(result.Batch.New))
	for _, res := range result.Batch.New {
		event, ok := res.Event()
		if !ok {
			continue
		}
		row := make([]any, 0, len(fields))
		for _, field := range fields {
			row = append(row, event.Get(field).Any())
		}
		rows = append(rows, row)
	}
	return rows
}

func TestOnSelectDeleteNamedWindow(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[seldelBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[seldelS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema missing")
	}
	if _, err := CreateNamedWindow(env, "MyInfra", schema); err != nil {
		t.Fatal(err)
	}

	createPlan, err := env.Build(FromNamedWindow(env, "MyInfra").Query(StatementName("create")))
	if err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[seldelBean](env, "SupportBean")).
		InsertIntoNamedWindow("MyInfra", CopyMatchingFields()).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	// on SupportBean_S0 select and delete
	//   window(win.*).aggregate(0,(result,value) => result+value.intPrimitive) as c0
	//   from MyInfra as win where s0.p00 = win.theString
	selectPlan, err := env.Build(OnEvent(From[seldelS0](env, "SupportBean_S0")).
		SelectDeleteFromNamedWindow("MyInfra",
			Equal[string](Field[seldelS0, string]("p00"), NamedWindowField[string]("theString")),
			Alias("c0", Sum[int32](NamedWindowField[int32]("intPrimitive")))).
		Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	createDeployment, err := engine.Deploy(context.Background(), createPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	selectDeployment, err := engine.Deploy(context.Background(), selectPlan)
	if err != nil {
		t.Fatal(err)
	}
	batches := subscribeSelectDeleteBatches(t, selectDeployment.Statements()[0])
	createStmt := createDeployment.Statements()[0]

	send := func(theString string, primitive int32) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), seldelBean{TheString: theString, IntPrimitive: primitive}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0 := func(id int32, p00 string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), seldelS0{ID: id, P00: p00}); err != nil {
			t.Fatal(err)
		}
	}
	fields := []string{"theString", "intPrimitive"}

	send("E1", 1)
	send("E2", 2)
	if rows := selectDeleteRows(t, createStmt, fields...); len(rows) != 2 {
		t.Fatalf("create rows = %v, want 2 rows", rows)
	}

	// Select and delete E1: the listener sees the matched sum and the row is
	// removed in the same trigger action.
	sendS0(100, "E1")
	if len(*batches) != 1 {
		t.Fatalf("s0 batches = %d, want 1", len(*batches))
	}
	if got := len((*batches)[0].New); got != 1 {
		t.Fatalf("s0 new rows = %d, want 1", got)
	}
	if c0 := (*batches)[0].New[0].Get("c0").Any(); c0 != int32(1) {
		t.Fatalf("c0 = %v, want 1", c0)
	}
	rows := selectDeleteRows(t, createStmt, fields...)
	if len(rows) != 1 || rows[0][0] != "E2" {
		t.Fatalf("create rows after delete = %v, want [[E2 2]]", rows)
	}

	// Two more E2 rows land, then the second select-delete folds all three.
	send("E2", 3)
	send("E2", 4)
	sendS0(101, "E2")
	if len(*batches) != 2 {
		t.Fatalf("s0 batches = %d, want 2", len(*batches))
	}
	if c0 := (*batches)[1].New[0].Get("c0").Any(); c0 != int32(9) {
		t.Fatalf("c0 = %v, want 9", c0)
	}
	if rows := selectDeleteRows(t, createStmt, fields...); len(rows) != 0 {
		t.Fatalf("create rows after second delete = %v, want empty", rows)
	}
}

func TestOnSelectDeleteTable(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[seldelBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[seldelS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "MyInfra", []TableColumn{
		PrimaryKeyColumn[string]("theString"),
		PrimaryKeyColumn[int32]("intPrimitive"),
	}); err != nil {
		t.Fatal(err)
	}

	createPlan, err := env.Build(FromTable(env, "MyInfra").Query(StatementName("create")))
	if err != nil {
		t.Fatal(err)
	}
	theString := Field[seldelBean, string]("theString")
	intPrimitive := Field[seldelBean, int32]("intPrimitive")
	insertPlan, err := env.Build(OnEvent(From[seldelBean](env, "SupportBean")).
		InsertIntoTable("MyInfra",
			SetColumn("theString", theString),
			SetColumn("intPrimitive", intPrimitive)).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	selectPlan, err := env.Build(OnEvent(From[seldelS0](env, "SupportBean_S0")).
		SelectDeleteFromTable("MyInfra",
			Equal[string](Field[seldelS0, string]("p00"), TableField[string]("theString")),
			Alias("c0", Sum[int32](TableField[int32]("intPrimitive")))).
		Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	createDeployment, err := engine.Deploy(context.Background(), createPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	selectDeployment, err := engine.Deploy(context.Background(), selectPlan)
	if err != nil {
		t.Fatal(err)
	}
	batches := subscribeSelectDeleteBatches(t, selectDeployment.Statements()[0])
	createStmt := createDeployment.Statements()[0]

	send := func(theString string, primitive int32) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), seldelBean{TheString: theString, IntPrimitive: primitive}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0 := func(id int32, p00 string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), seldelS0{ID: id, P00: p00}); err != nil {
			t.Fatal(err)
		}
	}
	fields := []string{"theString", "intPrimitive"}

	send("E1", 1)
	send("E2", 2)
	sendS0(100, "E1")
	if len(*batches) != 1 {
		t.Fatalf("s0 batches = %d, want 1", len(*batches))
	}
	if c0 := (*batches)[0].New[0].Get("c0").Any(); c0 != int32(1) {
		t.Fatalf("c0 = %v, want 1", c0)
	}
	if rows := selectDeleteRows(t, createStmt, fields...); len(rows) != 1 || rows[0][0] != "E2" {
		t.Fatalf("create rows after delete = %v, want [[E2 2]]", rows)
	}
	send("E2", 3)
	send("E2", 4)
	sendS0(101, "E2")
	if c0 := (*batches)[1].New[0].Get("c0").Any(); c0 != int32(9) {
		t.Fatalf("c0 = %v, want 9", c0)
	}
	if rows := selectDeleteRows(t, createStmt, fields...); len(rows) != 0 {
		t.Fatalf("create rows after second delete = %v, want empty", rows)
	}
}
