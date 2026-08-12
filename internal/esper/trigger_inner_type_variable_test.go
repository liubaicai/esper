package esper

import (
	"context"
	"reflect"
	"testing"
)

type triggerInnerVariableValue struct {
	In1 string `esper:"in1"`
	In2 int    `esper:"in2"`
}

type triggerInnerVariableEvent struct {
	Col1 string                     `esper:"col1"`
	Col2 *triggerInnerVariableValue `esper:"col2"`
}

type triggerInnerVariableTarget struct {
	C1 string                     `esper:"c1"`
	C2 *triggerInnerVariableValue `esper:"c2"`
}

func TestTriggerInnerTypeAndVariableBranchesMatchInfraInnerTypeAndVariable(t *testing.T) {
	for _, representation := range []string{"struct", "map", "object-array"} {
		for _, namedWindow := range []bool{true, false} {
			t.Run(representation+"/"+map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
				env := NewEnvironment()
				const sourceName = "TriggerInnerVariableSource"
				const targetName = "TriggerInnerVariableTarget"
				var (
					innerSchema  Schema
					targetSchema Schema
					col1         Expr
					matchCol1    Expression[string]
					col2         Expr
					nullInner    Expr
					send         func(*Engine, string, string, int) error
					tableC2Type  reflect.Type
				)

				switch representation {
				case "struct":
					var err error
					innerSchema, err = RegisterStruct[triggerInnerVariableValue](env, "TriggerInnerVariableValue")
					if err != nil {
						t.Fatal(err)
					}
					if _, err := RegisterStruct[triggerInnerVariableEvent](env, sourceName); err != nil {
						t.Fatal(err)
					}
					targetSchema, err = RegisterStruct[triggerInnerVariableTarget](env, targetName)
					if err != nil {
						t.Fatal(err)
					}
					col1 = Field[triggerInnerVariableEvent, string]("col1")
					matchCol1 = Field[triggerInnerVariableEvent, string]("col1")
					col2 = Field[triggerInnerVariableEvent, *triggerInnerVariableValue]("col2")
					nullInner = NullLiteral[*triggerInnerVariableValue]()
					tableC2Type = reflect.TypeOf((*triggerInnerVariableValue)(nil))
					send = func(engine *Engine, value, inner string, number int) error {
						return engine.SendEvent(context.Background(), triggerInnerVariableEvent{
							Col1: value,
							Col2: &triggerInnerVariableValue{In1: inner, In2: number},
						})
					}
				case "map":
					var err error
					innerSchema, err = RegisterMap(env, "TriggerInnerVariableValue", []FieldSpec{
						FieldDef("in1", reflect.TypeOf("")),
						FieldDef("in2", reflect.TypeOf(int(0))),
					})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := RegisterMap(env, sourceName, []FieldSpec{
						FieldDef("col1", reflect.TypeOf("")),
						FieldDef("col2", reflect.TypeOf(map[string]any{})),
					}, WithNestedPropertySchema("col2", innerSchema)); err != nil {
						t.Fatal(err)
					}
					targetSchema, err = RegisterMap(env, targetName, []FieldSpec{
						FieldDef("c1", reflect.TypeOf("")),
						FieldDef("c2", reflect.TypeOf(map[string]any{})),
					}, WithNestedPropertySchema("c2", innerSchema))
					if err != nil {
						t.Fatal(err)
					}
					col1 = Field[any, string]("col1")
					matchCol1 = Field[any, string]("col1")
					col2 = Field[any, map[string]any]("col2")
					nullInner = NullLiteral[map[string]any]()
					tableC2Type = reflect.TypeOf(map[string]any{})
					send = func(engine *Engine, value, inner string, number int) error {
						return engine.SendRecord(context.Background(), sourceName, map[string]any{
							"col1": value,
							"col2": map[string]any{"in1": inner, "in2": number},
						})
					}
				case "object-array":
					var err error
					innerSchema, err = RegisterObjectArray(env, "TriggerInnerVariableValue", []FieldSpec{
						FieldDef("in1", reflect.TypeOf("")),
						FieldDef("in2", reflect.TypeOf(int(0))),
					})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := RegisterObjectArray(env, sourceName, []FieldSpec{
						FieldDef("col1", reflect.TypeOf("")),
						FieldDef("col2", reflect.TypeOf([]any{})),
					}, WithNestedPropertySchema("col2", innerSchema)); err != nil {
						t.Fatal(err)
					}
					targetSchema, err = RegisterObjectArray(env, targetName, []FieldSpec{
						FieldDef("c1", reflect.TypeOf("")),
						FieldDef("c2", reflect.TypeOf([]any{})),
					}, WithNestedPropertySchema("c2", innerSchema))
					if err != nil {
						t.Fatal(err)
					}
					col1 = Field[any, string]("col1")
					matchCol1 = Field[any, string]("col1")
					col2 = Field[any, []any]("col2")
					nullInner = NullLiteral[[]any]()
					tableC2Type = reflect.TypeOf([]any{})
					send = func(engine *Engine, value, inner string, number int) error {
						return engine.SendObjectArray(context.Background(), sourceName, []any{
							value,
							[]any{inner, number},
						})
					}
				}

				if err := env.RegisterVariable("myvar", false); err != nil {
					t.Fatal(err)
				}
				if namedWindow {
					if _, err := CreateNamedWindow(env, targetName, targetSchema, NamedWindowRetention(KeepAll())); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err := CreateTable(env, targetName, []TableColumn{
						PrimaryKeyColumn[string]("c1"),
						{Name: "c2", Type: tableC2Type, Nested: innerSchema},
					}); err != nil {
						t.Fatal(err)
					}
				}

				myvar := VariableRef[bool]("myvar")
				clauses := []TableMergeClause{
					WhenNotMatched(myvar,
						SetColumn("c1", col1),
						SetColumn("c2", col2),
					),
					WhenNotMatched(Equal[bool](myvar, Literal(false)),
						SetColumn("c1", Literal("A")),
						SetColumn("c2", nullInner),
					),
					WhenNotMatched(IsNull[bool](myvar),
						SetColumn("c1", Literal("B")),
						SetColumn("c2", col2),
					),
					WhenMatchedDeleteAny(),
				}

				var plan Plan
				var err error
				if representation == "struct" {
					source := From[triggerInnerVariableEvent](env, sourceName)
					if namedWindow {
						plan, err = env.Build(OnEvent(source).MergeIntoNamedWindowWhen(targetName,
							Equal[string](NamedWindowField[string]("c1"), matchCol1), clauses...).Query(StatementName("infra-inner-variable-merge")))
					} else {
						plan, err = env.Build(OnEvent(source).MergeIntoTableWhen(targetName, []Expr{col1}, clauses...).Query(StatementName("infra-inner-variable-merge")))
					}
				} else {
					source := FromAny(env, sourceName)
					if namedWindow {
						plan, err = env.Build(OnRecord(source).MergeIntoNamedWindowWhen(targetName,
							Equal[string](NamedWindowField[string]("c1"), matchCol1), clauses...).Query(StatementName("infra-inner-variable-merge")))
					} else {
						plan, err = env.Build(OnRecord(source).MergeIntoTableWhen(targetName, []Expr{col1}, clauses...).Query(StatementName("infra-inner-variable-merge")))
					}
				}
				if err != nil {
					t.Fatal(err)
				}

				engine := NewEngine(env)
				if err := engine.SetVariable(context.Background(), "myvar", nil); err != nil {
					t.Fatal(err)
				}
				deployment, err := engine.Deploy(context.Background(), plan)
				if err != nil {
					t.Fatal(err)
				}
				var batches []ResultBatch
				if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
					batches = append(batches, batch)
					return nil
				}); err != nil {
					t.Fatal(err)
				}

				sendEvent := func(value, inner string, number int) {
					t.Helper()
					if err := send(engine, value, inner, number); err != nil {
						t.Fatal(err)
					}
				}
				assertNew := func(wantC1, wantInner string, wantNull bool) {
					t.Helper()
					if len(batches) == 0 {
						t.Fatal("inner-type merge produced no result batch")
					}
					batch := batches[len(batches)-1]
					if len(batch.New) != 1 || len(batch.Old) != 0 {
						t.Fatalf("inner-type merge new batch = %#v", batch)
					}
					event, ok := batch.New[0].Event()
					if !ok || event.Get("c1").Any() != wantC1 {
						t.Fatalf("inner-type merge new event = %#v, want c1=%q", batch.New[0], wantC1)
					}
					if wantNull {
						if event.Get("c2").State() != ValueNull {
							t.Fatalf("inner-type null nested value = %#v", event.Get("c2"))
						}
					} else if event.Get("c2.in1").Any() != wantInner {
						t.Fatalf("inner-type nested value = %#v, want %q", event.Get("c2.in1"), wantInner)
					}
				}
				assertDelete := func(wantC1 string) {
					t.Helper()
					if len(batches) == 0 {
						t.Fatal("inner-type merge delete produced no result batch")
					}
					batch := batches[len(batches)-1]
					if len(batch.New) != 0 || len(batch.Old) != 1 {
						t.Fatalf("inner-type merge delete batch = %#v", batch)
					}
					event, ok := batch.Old[0].Event()
					if !ok || event.Get("c1").Any() != wantC1 {
						t.Fatalf("inner-type merge old event = %#v, want c1=%q", batch.Old[0], wantC1)
					}
				}

				sendEvent("X1", "Y1", 10)
				assertNew("B", "Y1", false)
				sendEvent("B", "ignored", 0)
				assertDelete("B")
				if err := engine.SetVariable(context.Background(), "myvar", true); err != nil {
					t.Fatal(err)
				}
				sendEvent("X2", "Y2", 11)
				assertNew("X2", "Y2", false)
				if err := engine.SetVariable(context.Background(), "myvar", false); err != nil {
					t.Fatal(err)
				}
				sendEvent("X3", "Y3", 12)
				assertNew("A", "", true)

				if err := deployment.Undeploy(context.Background()); err != nil {
					t.Fatal(err)
				}
				batches = nil
				if err := engine.SetVariable(context.Background(), "myvar", true); err != nil {
					t.Fatal(err)
				}
				deployment, err = engine.Deploy(context.Background(), plan)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
					batches = append(batches, batch)
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				sendEvent("X4", "Y4", 11)
				assertNew("X4", "Y4", false)

				if namedWindow {
					window, ok := engine.NamedWindow(targetName)
					if !ok {
						t.Fatal("inner-type named window is missing")
					}
					events, err := window.Snapshot(context.Background())
					if err != nil || len(events) != 3 {
						t.Fatalf("inner-type named window snapshot = %#v, err=%v", events, err)
					}
					switch representation {
					case "struct":
						if _, ok := events[0].Underlying().(triggerInnerVariableTarget); !ok {
							t.Fatalf("struct target representation = %#v", events[0].Underlying())
						}
					case "map":
						if _, ok := events[0].Underlying().(map[string]any); !ok {
							t.Fatalf("map target representation = %#v", events[0].Underlying())
						}
					case "object-array":
						if _, ok := events[0].Underlying().([]any); !ok {
							t.Fatalf("object-array target representation = %#v", events[0].Underlying())
						}
					}
				} else {
					table, ok := engine.Table(targetName)
					if !ok {
						t.Fatal("inner-type table is missing")
					}
					rows, err := table.Snapshot(context.Background())
					if err != nil || len(rows) != 3 {
						t.Fatalf("inner-type table snapshot = %#v, err=%v", rows, err)
					}
				}
			})
		}
	}
}
