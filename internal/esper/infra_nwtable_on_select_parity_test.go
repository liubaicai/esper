package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type nwOnSelectSupportBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type nwOnSelectSupportA struct {
	ID string `esper:"id"`
}

type nwOnSelectSupportB struct {
	ID string `esper:"id"`
}

type nwOnSelectSupportS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

func newNwOnSelectEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[nwOnSelectSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwOnSelectSupportA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwOnSelectSupportB](env, "SupportBean_B"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[nwOnSelectSupportS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	return env
}

func nwOnSelectResults(t *testing.T, stmt *Statement) func() []Result {
	t.Helper()
	var results []Result
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		results = append(results, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func() []Result {
		return append([]Result(nil), results...)
	}
}

func nwOnSelectCreateInfra(t *testing.T, env *Environment, name string, namedWindow bool, fields []FieldSpec, tableColumns []TableColumn) {
	t.Helper()
	if namedWindow {
		schema, err := NewMapSchema(name+"Schema", fields)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.RegisterSchema(schema); err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNamedWindow(env, name, schema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
		return
	}
	if _, err := CreateTable(env, name, tableColumns); err != nil {
		t.Fatal(err)
	}
}

func nwOnSelectInsertAB(t *testing.T, env *Environment, infra string, namedWindow bool) Plan {
	t.Helper()
	source := From[nwOnSelectSupportBean](env, "SupportBean")
	assignments := []TableAssignment{
		SetColumn("a", Field[nwOnSelectSupportBean, string]("theString")),
		SetColumn("b", Field[nwOnSelectSupportBean, int]("intPrimitive")),
	}
	var plan Plan
	var err error
	if namedWindow {
		plan, err = env.Build(OnEvent(source).InsertIntoNamedWindow(infra, assignments...).Query(StatementName("insert")))
	} else {
		plan, err = env.Build(OnEvent(source).InsertIntoTable(infra, assignments...).Query(StatementName("insert")))
	}
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

// TestInfraOnSelectIndexSimpleParity mirrors InfraOnSelectIndexSimple for
// named-window and table variants: an on-select predicate uses the indexable
// value column and emits the matching row.
func TestInfraOnSelectIndexSimpleParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		name := "named-window"
		if !namedWindow {
			name = "table"
		}
		t.Run(name, func(t *testing.T) {
			env := newNwOnSelectEnvironment(t)

			nwOnSelectCreateInfra(t, env, "MyInfra", namedWindow, []FieldSpec{
				FieldDef("numericKey", reflect.TypeOf(0)),
				FieldDef("value", reflect.TypeOf("")),
			}, []TableColumn{
				PrimaryKeyColumn[int]("numericKey"),
				TableColumnOf[string]("value"),
			})
			engine := NewEngine(env)
			defer func() { _ = engine.Close(context.Background()) }()
			source := From[nwOnSelectSupportBean](env, "SupportBean")
			assignments := []TableAssignment{
				SetColumn("numericKey", Field[nwOnSelectSupportBean, int]("intPrimitive")),
				SetColumn("value", Field[nwOnSelectSupportBean, string]("theString")),
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
			trigger := From[nwOnSelectSupportS0](env, "SupportBean_S0")
			predicate := Equal[string](NamedWindowField[string]("value"), Field[nwOnSelectSupportS0, string]("p00"))
			var selectPlan Plan
			if namedWindow {
				selectPlan, err = env.Build(OnEvent(trigger).SelectFromNamedWindow("MyInfra", predicate,
					Alias("value", NamedWindowField[string]("value")),
				).Query(StatementName("out")))
			} else {
				selectPlan, err = env.Build(OnEvent(trigger).SelectFromTableWhere("MyInfra", predicate,
					Alias("value", NamedWindowField[string]("value")),
				).Query(StatementName("out")))
			}
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), selectPlan)
			if err != nil {
				t.Fatal(err)
			}
			results := nwOnSelectResults(t, nwOnSelectStatement(t, deployment, "out"))
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportS0{ID: 1, P00: "E1"}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportBean{TheString: "E2", IntPrimitive: 2}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportS0{ID: 2, P00: "E2"}); err != nil {
				t.Fatal(err)
			}
			got := results()
			if len(got) != 2 || got[0].Get("value").Any() != "E1" || got[1].Get("value").Any() != "E2" {
				t.Fatalf("index-simple rows = %#v", got)
			}
		})
	}
}

func nwOnSelectStatement(t *testing.T, deployment *Deployment, name string) *Statement {
	t.Helper()
	for _, statement := range deployment.Statements() {
		if statement.Name() == name {
			return statement
		}
	}
	t.Fatalf("statement %q not found", name)
	return nil
}

// TestInfraSelectCorrelationDeleteParity mirrors InfraSelectCorrelationDelete
// for named-window and table variants: correlated on-select plus delete.
func TestInfraSelectCorrelationDeleteParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		name := "named-window"
		if !namedWindow {
			name = "table"
		}
		t.Run(name, func(t *testing.T) {
			env := newNwOnSelectEnvironment(t)

			nwOnSelectCreateInfra(t, env, "MyInfraSCD", namedWindow, []FieldSpec{
				FieldDef("a", reflect.TypeOf("")),
				FieldDef("b", reflect.TypeOf(0)),
			}, []TableColumn{
				PrimaryKeyColumn[string]("a"),
				PrimaryKeyColumn[int]("b"),
			})
			engine := NewEngine(env)
			defer func() { _ = engine.Close(context.Background()) }()
			insertPlan := nwOnSelectInsertAB(t, env, "MyInfraSCD", namedWindow)
			trigger := From[nwOnSelectSupportA](env, "SupportBean_A")
			predicate := Equal[string](NamedWindowField[string]("a"), Field[nwOnSelectSupportA, string]("id"))
			selections := []Selection{
				Alias("a", NamedWindowField[string]("a")),
				Alias("b", NamedWindowField[int]("b")),
			}
			var selectPlan Plan
			var err error
			if namedWindow {
				selectPlan, err = env.Build(OnEvent(trigger).SelectFromNamedWindow("MyInfraSCD", predicate, selections...).Query(StatementName("select")))
			} else {
				selectPlan, err = env.Build(OnEvent(trigger).SelectFromTableWhere("MyInfraSCD", predicate, selections...).Query(StatementName("select")))
			}
			if err != nil {
				t.Fatal(err)
			}
			deleteTrigger := From[nwOnSelectSupportB](env, "SupportBean_B")
			deletePredicate := Equal[string](NamedWindowField[string]("a"), Field[nwOnSelectSupportB, string]("id"))
			var deletePlan Plan
			if namedWindow {
				deletePlan, err = env.Build(OnEvent(deleteTrigger).DeleteFromNamedWindow("MyInfraSCD", deletePredicate).Query(StatementName("delete")))
			} else {
				deletePlan, err = env.Build(OnEvent(deleteTrigger).DeleteFromTableWhere("MyInfraSCD", deletePredicate).Query(StatementName("delete")))
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.DeployPlans(context.Background(), []Plan{selectPlan, deletePlan})
			if err != nil {
				t.Fatal(err)
			}
			results := nwOnSelectResults(t, nwOnSelectStatement(t, deployment, "select"))
			for _, event := range []nwOnSelectSupportBean{
				{TheString: "E1", IntPrimitive: 1},
				{TheString: "E2", IntPrimitive: 2},
				{TheString: "E3", IntPrimitive: 3},
			} {
				if err := engine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportA{ID: "X1"}); err != nil {
				t.Fatal(err)
			}
			if len(results()) != 0 {
				t.Fatalf("X1 unexpectedly selected %#v", results())
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportA{ID: "E2"}); err != nil {
				t.Fatal(err)
			}
			if got := results(); len(got) != 1 || got[0].Get("a").Any() != "E2" || got[0].Get("b").Any() != 2 {
				t.Fatalf("E2 selected = %#v", got)
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportA{ID: "E1"}); err != nil {
				t.Fatal(err)
			}
			if got := results(); len(got) != 2 || got[1].Get("a").Any() != "E1" {
				t.Fatalf("E1 selected = %#v", got)
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportB{ID: "E1"}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportA{ID: "E1"}); err != nil {
				t.Fatal(err)
			}
			if got := results(); len(got) != 2 {
				t.Fatalf("deleted E1 still selected = %#v", got)
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportA{ID: "E2"}); err != nil {
				t.Fatal(err)
			}
			if got := results(); len(got) != 3 || got[2].Get("a").Any() != "E2" {
				t.Fatalf("E2 after delete selected = %#v", got)
			}
		})
	}
}

// TestInfraSelectConditionParity mirrors InfraSelectCondition for named-window
// and table variants: predicate filtering, trigger-id projection and ordering.
func TestInfraSelectConditionParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		name := "named-window"
		if !namedWindow {
			name = "table"
		}
		t.Run(name, func(t *testing.T) {
			env := newNwOnSelectEnvironment(t)
			infra := "MyInfraSC"
			if !namedWindow {
				infra = "MyInfraTbl"
			}
			nwOnSelectCreateInfra(t, env, infra, namedWindow, []FieldSpec{
				FieldDef("a", reflect.TypeOf("")),
				FieldDef("b", reflect.TypeOf(0)),
			}, []TableColumn{
				PrimaryKeyColumn[string]("a"),
				TableColumnOf[int]("b"),
			})
			engine := NewEngine(env)
			defer func() { _ = engine.Close(context.Background()) }()
			insertPlan := nwOnSelectInsertAB(t, env, infra, namedWindow)
			trigger := From[nwOnSelectSupportA](env, "SupportBean_A")
			predicate := Less[int](NamedWindowField[int]("b"), Literal(3))
			selections := []Selection{
				Alias("a", NamedWindowField[string]("a")),
				Alias("b", NamedWindowField[int]("b")),
				Alias("id", Field[nwOnSelectSupportA, string]("id")),
			}
			var selectPlan Plan
			var err error
			if namedWindow {
				selectPlan, err = env.Build(OnEvent(trigger).SelectFromNamedWindow(infra, predicate, selections...).Query(
					StatementName("select"), OrderBy(Ascending(ResultField[string]("a")))))
			} else {
				selectPlan, err = env.Build(OnEvent(trigger).SelectFromTableWhere(infra, predicate, selections...).Query(
					StatementName("select"), OrderBy(Ascending(ResultField[string]("a")))))
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), selectPlan)
			if err != nil {
				t.Fatal(err)
			}
			results := nwOnSelectResults(t, nwOnSelectStatement(t, deployment, "select"))
			for _, event := range []nwOnSelectSupportBean{
				{TheString: "E1", IntPrimitive: 1},
				{TheString: "E2", IntPrimitive: 2},
				{TheString: "E3", IntPrimitive: 3},
			} {
				if err := engine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportA{ID: "A1"}); err != nil {
				t.Fatal(err)
			}
			got := results()
			if len(got) != 2 || got[0].Get("a").Any() != "E1" || got[1].Get("a").Any() != "E2" || got[0].Get("id").Any() != "A1" {
				t.Fatalf("condition select = %#v", got)
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportBean{TheString: "E4", IntPrimitive: 0}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportA{ID: "A2"}); err != nil {
				t.Fatal(err)
			}
			got = results()
			if len(got) != 5 || got[2].Get("a").Any() != "E1" || got[3].Get("a").Any() != "E2" || got[4].Get("a").Any() != "E4" {
				t.Fatalf("condition select after E4 = %#v", got)
			}
		})
	}
}

// TestInfraSelectJoinColumnsLimitParity mirrors InfraSelectJoinColumnsLimit
// for named-window and table variants: trigger + window projections with
// ordering, then a limit-one variant.
func TestInfraSelectJoinColumnsLimitParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		name := "named-window"
		if !namedWindow {
			name = "table"
		}
		t.Run(name, func(t *testing.T) {
			env := newNwOnSelectEnvironment(t)
			infra := "MyInfraSA"
			nwOnSelectCreateInfra(t, env, infra, namedWindow, []FieldSpec{
				FieldDef("a", reflect.TypeOf("")),
				FieldDef("b", reflect.TypeOf(0)),
			}, []TableColumn{
				PrimaryKeyColumn[string]("a"),
				TableColumnOf[int]("b"),
			})
			engine := NewEngine(env)
			defer func() { _ = engine.Close(context.Background()) }()
			insertPlan := nwOnSelectInsertAB(t, env, infra, namedWindow)
			trigger := From[nwOnSelectSupportA](env, "SupportBean_A")
			selections := []Selection{
				Alias("triggerid", Field[nwOnSelectSupportA, string]("id")),
				Alias("wina", NamedWindowField[string]("a")),
				Alias("b", NamedWindowField[int]("b")),
			}
			buildSelect := func(limit bool) Plan {
				options := []QueryOption{StatementName("select"), OrderBy(Ascending(ResultField[string]("wina")))}
				if limit {
					options = append(options, Limit(1))
				}
				var plan Plan
				var err error
				if namedWindow {
					plan, err = env.Build(OnEvent(trigger).SelectFromNamedWindow(infra, Literal[bool](true), selections...).Query(options...))
				} else {
					plan, err = env.Build(OnEvent(trigger).SelectFromTableWhere(infra, Literal[bool](true), selections...).Query(options...))
				}
				if err != nil {
					t.Fatal(err)
				}
				return plan
			}
			if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), buildSelect(false))
			if err != nil {
				t.Fatal(err)
			}
			results := nwOnSelectResults(t, nwOnSelectStatement(t, deployment, "select"))
			for _, event := range []nwOnSelectSupportBean{
				{TheString: "E1", IntPrimitive: 1},
				{TheString: "E2", IntPrimitive: 2},
			} {
				if err := engine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportA{ID: "A1"}); err != nil {
				t.Fatal(err)
			}
			got := results()
			if len(got) != 2 || got[0].Get("triggerid").Any() != "A1" || got[0].Get("wina").Any() != "E1" || got[1].Get("wina").Any() != "E2" {
				t.Fatalf("join-columns select = %#v", got)
			}
			if err := deployment.Undeploy(context.Background()); err != nil {
				t.Fatal(err)
			}
			limitDeployment, err := engine.Deploy(context.Background(), buildSelect(true))
			if err != nil {
				t.Fatal(err)
			}
			limitResults := nwOnSelectResults(t, nwOnSelectStatement(t, limitDeployment, "select"))
			if err := engine.SendEvent(context.Background(), nwOnSelectSupportA{ID: "A2"}); err != nil {
				t.Fatal(err)
			}
			limited := limitResults()
			if len(limited) != 1 || limited[0].Get("wina").Any() != "E1" {
				t.Fatalf("limited join-columns select = %#v", limited)
			}
		})
	}
}

// TestInfraInvalidParity mirrors InfraInvalid for named-window and table
// variants: aggregate-in-where, unknown infra and prev-in-select are rejected.
func TestInfraInvalidParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		name := "named-window"
		if !namedWindow {
			name = "table"
		}
		t.Run(name, func(t *testing.T) {
			env := newNwOnSelectEnvironment(t)
			nwOnSelectCreateInfra(t, env, "MyInfraInvalid", namedWindow, []FieldSpec{
				FieldDef("theString", reflect.TypeOf("")),
				FieldDef("intPrimitive", reflect.TypeOf(0)),
			}, []TableColumn{
				PrimaryKeyColumn[string]("theString"),
				TableColumnOf[int]("intPrimitive"),
			})
			engine := NewEngine(env)
			defer func() { _ = engine.Close(context.Background()) }()
			source := From[nwOnSelectSupportBean](env, "SupportBean")
			insertAssignments := []TableAssignment{
				SetColumn("theString", Field[nwOnSelectSupportBean, string]("theString")),
				SetColumn("intPrimitive", Field[nwOnSelectSupportBean, int]("intPrimitive")),
			}
			var insertPlan Plan
			var err error
			if namedWindow {
				insertPlan, err = env.Build(OnEvent(source).InsertIntoNamedWindow("MyInfraInvalid", insertAssignments...).Query(StatementName("insert")))
			} else {
				insertPlan, err = env.Build(OnEvent(source).InsertIntoTable("MyInfraInvalid", insertAssignments...).Query(StatementName("insert")))
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
				t.Fatal(err)
			}
			trigger := From[nwOnSelectSupportA](env, "SupportBean_A")
			aggregatePredicate := Greater[int](Sum[int](NamedWindowField[int]("intPrimitive")), Literal(100))
			if namedWindow {
				aggregatePlan, buildErr := env.Build(OnEvent(trigger).SelectFromNamedWindow("MyInfraInvalid", aggregatePredicate,
					Alias("total", NamedWindowField[int]("intPrimitive"))).Query(StatementName("s0")))
				if buildErr != nil {
					err = buildErr
				} else {
					_, err = engine.Deploy(context.Background(), aggregatePlan)
				}
			} else {
				aggregatePlan, buildErr := env.Build(OnEvent(trigger).SelectFromTableWhere("MyInfraInvalid", aggregatePredicate,
					Alias("total", NamedWindowField[int]("intPrimitive"))).Query(StatementName("s0")))
				if buildErr != nil {
					err = buildErr
				} else {
					_, err = engine.Deploy(context.Background(), aggregatePlan)
				}
			}
			if err == nil {
				t.Fatal("aggregate-in-where plan was accepted")
			}
			if !strings.Contains(err.Error(), "aggregate") {
				t.Fatalf("aggregate-in-where error = %v", err)
			}
			unknownPlan, unknownErr := env.Build(OnEvent(trigger).SelectFromNamedWindow("DUMMY", Literal[bool](true)).Query(StatementName("s0")))
			if unknownErr == nil {
				_, unknownErr = engine.Deploy(context.Background(), unknownPlan)
			}
			if unknownErr == nil {
				t.Fatal("unknown infra select was accepted")
			}
			prevSelection := Alias("x", Prev[string](1, NamedWindowField[string]("theString")))
			var prevPlan Plan
			if namedWindow {
				prevPlan, err = env.Build(OnEvent(trigger).SelectFromNamedWindow("MyInfraInvalid", Literal[bool](true), prevSelection).Query(StatementName("s0")))
			} else {
				prevPlan, err = env.Build(OnEvent(trigger).SelectFromTableWhere("MyInfraInvalid", Literal[bool](true), prevSelection).Query(StatementName("s0")))
			}
			if err == nil {
				_, err = engine.Deploy(context.Background(), prevPlan)
			}
			if err == nil || !strings.Contains(err.Error(), "Previous") {
				t.Fatalf("prev-in-select error = %v", err)
			}
		})
	}
}
