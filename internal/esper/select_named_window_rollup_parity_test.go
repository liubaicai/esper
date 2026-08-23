package esper

import (
	"context"
	"fmt"
	"testing"
)

func asInt64(t *testing.T, value Value) int64 {
	t.Helper()
	switch number := value.Any().(type) {
	case nil:
		t.Fatal("expected numeric value, got nil")
	case int32:
		return int64(number)
	case int64:
		return number
	case int:
		return int64(number)
	default:
		t.Fatalf("unexpected numeric type %T", value.Any())
	}
	return 0
}

// onsetSelectS0 mirrors the trigger-side SupportBean_S0 event.
type onsetSelectS0 struct {
	ID string `esper:"id"`
}

// TestSelectFromNamedWindowRollupParity mirrors ResultSetQueryTypeOnSelect:
// an on-trigger select over a keep-all named window with a grouped rollup
// emits one row per first-seen group per rollup level, the overall level
// last, and re-fires on each trigger event.
// Java runtime: java-runtime-58abe8e5ebfa57510a04.
func TestSelectFromNamedWindowRollupParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[onsetArrayBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[onsetSelectS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema missing")
	}
	if _, err := CreateNamedWindow(env, "MyWindow", schema); err != nil {
		t.Fatal(err)
	}

	insertPlan, err := env.Build(OnEvent(From[onsetArrayBean](env, "SupportBean")).
		InsertIntoNamedWindow("MyWindow", CopyMatchingFields()).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	theString := Field[onsetArrayBean, string]("theString")
	intPrimitive := Field[onsetArrayBean, int32]("intPrimitive")
	selectPlan, err := env.Build(OnEvent(From[onsetSelectS0](env, "SupportBean_S0")).
		SelectFromNamedWindowRollup("MyWindow", nil,
			[]Expr{theString},
			Alias("c0", theString),
			// Aggregate inputs read each group row through plain fields;
			// named-window-qualified expressions resolve the representative
			// row of the group.
			Alias("c1", Sum[int32](intPrimitive)),
			Alias("c2", CountAll()),
		).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	selectDeployment, err := engine.Deploy(context.Background(), selectPlan)
	if err != nil {
		t.Fatal(err)
	}
	batches := subscribeOnSetVarCapture(t, selectDeployment.Statements()[0])

	sendBean := func(theStringValue string, primitive int32) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), onsetArrayBean{TheString: theStringValue, IntPrimitive: primitive}); err != nil {
			t.Fatal(err)
		}
	}
	assertRows := func(want [][]any) {
		t.Helper()
		if len(*batches) == 0 {
			t.Fatal("on-select rollup produced no batch")
		}
		last := (*batches)[len(*batches)-1]
		if len(last.New) != len(want) {
			t.Fatalf("rows = %d, want %d", len(last.New), len(want))
		}
		for i, row := range last.New {
			got, ok := row.Row()
			if !ok {
				t.Fatalf("row %d is not a Row result", i)
			}
			c0 := got.Get("c0")
			c1 := got.Get("c1")
			c2 := got.Get("c2")
			expected := want[i]
			switch expectedC0 := expected[0].(type) {
			case nil:
				if !c0.IsNull() && !c0.IsMissing() {
					t.Fatalf("row %d c0 = %v, want null", i, c0.Any())
				}
			case string:
				if c0.Any() != expectedC0 {
					t.Fatalf("row %d c0 = %v, want %v", i, c0.Any(), expectedC0)
				}
			}
			if got := asInt64(t, c1); got != int64(expected[1].(int)) {
				t.Fatalf("row %d c1 = %v, want %v", i, got, expected[1])
			}
			if got := asInt64(t, c2); got != int64(expected[2].(int)) {
				t.Fatalf("row %d c2 = %v, want %v", i, got, expected[2])
			}
		}
	}

	// Seeds i=0..9: E0:[0,3,6,9] sum 18 count 4; E1:[1,4,7] sum 12 count 3;
	// E2:[2,5,8] sum 15 count 3.
	for i := int32(0); i < 10; i++ {
		sendBean(fmt.Sprintf("E%d", i%3), i)
	}
	if err := engine.SendEvent(context.Background(), onsetSelectS0{ID: "1"}); err != nil {
		t.Fatal(err)
	}
	assertRows([][]any{
		{"E0", 18, 4},
		{"E1", 12, 3},
		{"E2", 15, 3},
		{nil, 45, 10},
	})

	// A SupportBean event fires only the insert statement, never s0.
	before := len(*batches)
	sendBean("E1", 6)
	if len(*batches) != before {
		t.Fatal("insert unexpectedly fired the on-select statement")
	}
	if err := engine.SendEvent(context.Background(), onsetSelectS0{ID: "2"}); err != nil {
		t.Fatal(err)
	}
	assertRows([][]any{
		{"E0", 18, 4},
		{"E1", 18, 4},
		{"E2", 15, 3},
		{nil, 51, 11},
	})
}
