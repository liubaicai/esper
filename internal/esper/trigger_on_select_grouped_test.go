package esper

import (
	"context"
	"sort"
	"strings"
	"testing"
)

// Engine coverage for ResultSetQueryTypeLocalGroupBy.ResultSetLocalGroupedOnSelect:
// a plain grouped on-select over a keep-all named window with an ordinary sum
// and a zero-key LocalGroupBy sum. The zero-key local group binds the
// statement-level scope of every taken row, so its aggregate is identical on
// every group row while ordinary aggregates stay per group.

func TestOnSelectGroupedNamedWindow(t *testing.T) {
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
		SelectFromNamedWindowGroupBy("MyWindow", nil,
			[]Expr{theString},
			Alias("c0", theString),
			Alias("c1", Sum[int32](intPrimitive)),
			Alias("c2", LocalGroupBy[int32](Sum[int32](intPrimitive))),
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
			t.Fatal("grouped on-select produced no batch")
		}
		last := (*batches)[len(*batches)-1]
		if len(last.New) != len(want) {
			t.Fatalf("rows = %d, want %d", len(last.New), len(want))
		}
		type rowValues struct {
			key string
			c1  int64
			c2  int64
		}
		rows := make([]rowValues, 0, len(last.New))
		for i, result := range last.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("row %d is not a Row result", i)
			}
			key, ok := row.Get("c0").Any().(string)
			if !ok {
				t.Fatalf("row %d c0 = %v, want string", i, row.Get("c0").Any())
			}
			rows = append(rows, rowValues{key: key, c1: asInt64(t, row.Get("c1")), c2: asInt64(t, row.Get("c2"))})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].key < rows[j].key })
		for i, expected := range want {
			if rows[i].key != expected[0].(string) {
				t.Fatalf("row %d c0 = %v, want %v", i, rows[i].key, expected[0])
			}
			if rows[i].c1 != int64(expected[1].(int)) {
				t.Fatalf("row %d c1 = %v, want %v", i, rows[i].c1, expected[1])
			}
			if rows[i].c2 != int64(expected[2].(int)) {
				t.Fatalf("row %d c2 = %v, want %v", i, rows[i].c2, expected[2])
			}
		}
	}

	// E1/10, E2/20, E1/30, E3/40, E2/50 then the trigger. Per-group sums are
	// 40/70/40; the zero-key local group sums all taken rows (150) and is
	// identical on every group row.
	sendBean("E1", 10)
	sendBean("E2", 20)
	sendBean("E1", 30)
	sendBean("E3", 40)
	sendBean("E2", 50)
	if err := engine.SendEvent(context.Background(), onsetSelectS0{ID: "1"}); err != nil {
		t.Fatal(err)
	}
	assertRows([][]any{
		{"E1", 40, 150},
		{"E2", 70, 150},
		{"E3", 40, 150},
	})

	// E1/60 then re-trigger: E1 sums to 100, the statement-wide local group
	// advances to 210, E2/E3 stay unchanged.
	sendBean("E1", 60)
	if err := engine.SendEvent(context.Background(), onsetSelectS0{ID: "2"}); err != nil {
		t.Fatal(err)
	}
	assertRows([][]any{
		{"E1", 100, 210},
		{"E2", 70, 210},
		{"E3", 40, 210},
	})
}

// TestRollupLocalGroupRejected mirrors ResultSetQueryTypeLocalGroupBy's
// "not allowed in combination with roll-up" compile rejection with Java's
// exact sentence, both for the grouped on-select trigger form and for the
// ordinary rollup aggregate query.
func TestRollupLocalGroupRejected(t *testing.T) {
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
	theString := Field[onsetArrayBean, string]("theString")
	intPrimitive := Field[onsetArrayBean, int32]("intPrimitive")
	message := "Roll-up and group-by parameters cannot be combined"

	rollupSelect := OnEvent(From[onsetSelectS0](env, "SupportBean_S0")).
		SelectFromNamedWindowRollup("MyWindow", nil,
			[]Expr{theString},
			Alias("c0", theString),
			Alias("c1", LocalGroupBy[int32](Sum[int32](intPrimitive))),
		).Query(StatementName("s0"))
	_, err := env.Build(rollupSelect)
	if err == nil {
		t.Fatal("rollup on-select with local group-by unexpectedly built")
	}
	if !strings.Contains(err.Error(), message) {
		t.Fatalf("rollup on-select error = %q, want sentence %q", err.Error(), message)
	}

	rollupQuery := From[onsetArrayBean](env, "SupportBean").
		GroupByRollup(theString).
		Select(Alias("c0", LocalGroupBy[int32](Sum[int32](intPrimitive)))).Query()
	_, err = env.Build(rollupQuery)
	if err == nil {
		t.Fatal("rollup query with local group-by unexpectedly built")
	}
	if !strings.Contains(err.Error(), message) {
		t.Fatalf("rollup query error = %q, want sentence %q", err.Error(), message)
	}

	// The plain grouped on-select accepts local group-by parameters: it is
	// the form the grouped scenario relies on.
	groupedSelect := OnEvent(From[onsetSelectS0](env, "SupportBean_S0")).
		SelectFromNamedWindowGroupBy("MyWindow", nil,
			[]Expr{theString},
			Alias("c0", theString),
			Alias("c1", LocalGroupBy[int32](Sum[int32](intPrimitive))),
		).Query(StatementName("s1"))
	if _, err := env.Build(groupedSelect); err != nil {
		t.Fatalf("grouped on-select with local group-by failed to build: %v", err)
	}
}
