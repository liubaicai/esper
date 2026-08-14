package esper

import (
	"context"
	"reflect"
	"testing"
)

// TestInfraNWTableCreateIndexParity covers the late-create, drop/recreate,
// multiple-index and invalid boundaries from InfraNWTableCreateIndex for both
// named-window and table variants.
func TestInfraNWTableCreateIndexParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		name := "named-window"
		if !namedWindow {
			name = "table"
		}
		t.Run(name, func(t *testing.T) {
			env := newNwOnSelectEnvironment(t)
			nwOnSelectCreateInfra(t, env, "MyInfra", namedWindow, []FieldSpec{
				FieldDef("theString", reflect.TypeOf("")),
				FieldDef("intPrimitive", reflect.TypeOf(0)),
			}, []TableColumn{
				PrimaryKeyColumn[string]("theString"),
				TableColumnOf[int]("intPrimitive"),
			})
			engine := NewEngine(env)
			defer func() { _ = engine.Close(context.Background()) }()

			source := From[nwOnSelectSupportBean](env, "SupportBean")
			assignments := []TableAssignment{
				SetColumn("theString", Field[nwOnSelectSupportBean, string]("theString")),
				SetColumn("intPrimitive", Field[nwOnSelectSupportBean, int]("intPrimitive")),
			}
			var insertPlan Plan
			var err error
			if namedWindow {
				insertPlan, err = env.Build(OnEvent(source).InsertIntoNamedWindow("MyInfra", assignments...).Query(StatementName("insert")))
			} else {
				insertPlan, err = env.Build(OnEvent(source).InsertIntoTable("MyInfra", assignments...).Query(StatementName("insert")))
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
				t.Fatal(err)
			}
			for _, event := range []nwOnSelectSupportBean{
				{TheString: "E1", IntPrimitive: 1},
				{TheString: "E2", IntPrimitive: 2},
			} {
				if err := engine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}

			if namedWindow {
				window, ok := engine.NamedWindow("MyInfra")
				if !ok {
					t.Fatal("named window missing")
				}
				if err := window.CreateIndex("I1", []string{"theString"}, IndexHash, false); err != nil {
					t.Fatal(err)
				}
				if !infraNWIndexContains(window.Definition().Indexes(), "I1") {
					t.Fatalf("late index missing: %#v", window.Definition().Indexes())
				}
			} else {
				table, ok := engine.Table("MyInfra")
				if !ok {
					t.Fatal("table missing")
				}
				if err := table.CreateIndex("I1", []string{"theString"}, IndexHash, false); err != nil {
					t.Fatal(err)
				}
				if !infraTableIndexContains(table.Definition().Indexes(), "I1") {
					t.Fatalf("late table index missing: %#v", table.Definition().Indexes())
				}
			}
			assertInfraNWTCreateIndexFAF(t, engine, env, namedWindow, 1)

			if namedWindow {
				window, _ := engine.NamedWindow("MyInfra")
				if err := window.DropIndex("I1"); err != nil {
					t.Fatal(err)
				}
				if err := window.CreateIndex("I2", []string{"intPrimitive"}, IndexHash, false); err != nil {
					t.Fatal(err)
				}
				if err := window.CreateIndex("I3", []string{"theString", "intPrimitive"}, IndexBTree, false); err != nil {
					t.Fatal(err)
				}
				if err := window.CreateIndex("I1", []string{"theString"}, IndexHash, false); err != nil {
					t.Fatal(err)
				}
				window.DropIndex("I1")
				window.DropIndex("I2")
				window.DropIndex("I3")
				if len(window.Definition().Indexes()) != 0 {
					t.Fatalf("drop/recreate indexes = %#v", window.Definition().Indexes())
				}
			} else {
				table, _ := engine.Table("MyInfra")
				if err := table.DropIndex("I1"); err != nil {
					t.Fatal(err)
				}
				if err := table.CreateIndex("I2", []string{"intPrimitive"}, IndexHash, false); err != nil {
					t.Fatal(err)
				}
				if err := table.CreateIndex("I3", []string{"theString", "intPrimitive"}, IndexBTree, false); err != nil {
					t.Fatal(err)
				}
				if err := table.CreateIndex("I1", []string{"theString"}, IndexHash, false); err != nil {
					t.Fatal(err)
				}
				table.DropIndex("I1")
				table.DropIndex("I2")
				table.DropIndex("I3")
				if len(table.Definition().Indexes()) != 0 {
					t.Fatalf("drop/recreate table indexes = %#v", table.Definition().Indexes())
				}
			}
			assertInfraNWTCreateIndexFAF(t, engine, env, namedWindow, 1)

			// Invalid boundaries: duplicate name, unknown column, unique btree,
			// and empty column list.
			if namedWindow {
				window, _ := engine.NamedWindow("MyInfra")
				if err := window.CreateIndex("Dup", []string{"theString"}, IndexHash, false); err != nil {
					t.Fatal(err)
				}
				if err := window.CreateIndex("Dup", []string{"intPrimitive"}, IndexHash, false); err == nil {
					t.Fatal("duplicate named-window index accepted")
				}
				if err := window.CreateIndex("Bad", []string{"missing"}, IndexHash, false); err == nil {
					t.Fatal("unknown named-window column accepted")
				}
				if err := window.CreateIndex("UniqueBTree", []string{"theString"}, IndexBTree, true); err == nil {
					t.Fatal("unique btree named-window index accepted")
				}
				if err := window.CreateIndex("Empty", nil, IndexHash, false); err == nil {
					t.Fatal("empty named-window index accepted")
				}
			} else {
				table, _ := engine.Table("MyInfra")
				if err := table.CreateIndex("Dup", []string{"theString"}, IndexHash, false); err != nil {
					t.Fatal(err)
				}
				if err := table.CreateIndex("Dup", []string{"intPrimitive"}, IndexHash, false); err == nil {
					t.Fatal("duplicate table index accepted")
				}
				if err := table.CreateIndex("Bad", []string{"missing"}, IndexHash, false); err == nil {
					t.Fatal("unknown table column accepted")
				}
				if err := table.CreateIndex("UniqueBTree", []string{"theString"}, IndexBTree, true); err == nil {
					t.Fatal("unique btree table index accepted")
				}
				if err := table.CreateIndex("Empty", nil, IndexHash, false); err == nil {
					t.Fatal("empty table index accepted")
				}
			}
		})
	}
}

func infraNWIndexContains(indexes []NamedWindowIndexDefinition, name string) bool {
	for _, index := range indexes {
		if index.Name == name {
			return true
		}
	}
	return false
}

func infraTableIndexContains(indexes []TableIndexDefinition, name string) bool {
	for _, index := range indexes {
		if index.Name == name {
			return true
		}
	}
	return false
}

func assertInfraNWTCreateIndexFAF(t *testing.T, engine *Engine, env *Environment, namedWindow bool, want int) {
	t.Helper()
	var source RecordStream
	if namedWindow {
		source = FromNamedWindow(env, "MyInfra")
	} else {
		source = FromTable(env, "MyInfra")
	}
	plan, err := env.Build(source.Filter(
		Equal[string](Field[any, string]("theString"), Literal("E1")),
	).Query(StatementName("faf")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != want {
		t.Fatalf("FAF results = %#v, want %d", result.Results(), want)
	}
}
