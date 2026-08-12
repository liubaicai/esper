package esper

import (
	"context"
	"reflect"
	"testing"
)

type triggerPropertyEvalBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type triggerPropertyEvalContainer struct {
	Beans []triggerPropertyEvalBean `esper:"beans"`
}

func TestTriggerInfraPropertyEvalInsertNoMatch(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
			env := NewEnvironment()
			const (
				sourceName = "TriggerPropertyEvalOrder"
				bookName   = "TriggerPropertyEvalBook"
				targetName = "TriggerPropertyEvalTarget"
			)
			if _, err := RegisterStruct[containedAdvancedOrder](env, sourceName); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[containedAdvancedBook](env, bookName); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				targetSchema, err := RegisterMap(env, targetName, []FieldSpec{
					FieldDef("c1", reflect.TypeOf("")),
					FieldDef("c2", reflect.TypeOf("")),
				})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := CreateNamedWindow(env, targetName, targetSchema, NamedWindowRetention(KeepAll())); err != nil {
					t.Fatal(err)
				}
			} else if _, err := CreateTable(env, targetName, []TableColumn{
				PrimaryKeyColumn[string]("c1"),
				TableColumnOf[string]("c2"),
			}); err != nil {
				t.Fatal(err)
			}

			orders := From[containedAdvancedOrder](env, sourceName)
			books := Unnest[containedAdvancedOrder, containedAdvancedBook](orders,
				Property[[]containedAdvancedBook](EventValue[containedAdvancedOrder](), "books"))
			bookID := Field[containedAdvancedBook, string]("bookId")
			title := Field[containedAdvancedBook, string]("title")
			var plan Plan
			var err error
			if namedWindow {
				match := Equal[string](NamedWindowField[string]("c1"), bookID)
				plan, err = env.Build(OnEvent(books).MergeIntoNamedWindowWhen(targetName, match,
					WhenNotMatchedAny(SetColumn("c1", bookID), SetColumn("c2", title)),
				).Query(StatementName("trigger-property-eval-insert")))
			} else {
				plan, err = env.Build(OnEvent(books).MergeIntoTableWhen(targetName, []Expr{bookID},
					WhenNotMatchedAny(SetColumn("c1", bookID), SetColumn("c2", title)),
				).Query(StatementName("trigger-property-eval-insert")))
			}
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			var inserted []Event
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, result := range batch.New {
					if event, ok := result.Event(); ok {
						inserted = append(inserted, event)
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), containedAdvancedOrder{
				OrderID: "O-property-insert",
				Books: []containedAdvancedBook{
					{BookID: "10020", Title: "Enders Game"},
					{BookID: "10021", Title: "Foundation 1"},
					{BookID: "10022", Title: "Stranger in a Strange Land"},
				},
			}); err != nil {
				t.Fatal(err)
			}
			if len(inserted) != 3 {
				t.Fatalf("contained property-eval insert events = %#v, want 3", inserted)
			}
			for index, want := range []struct{ id, title string }{
				{"10020", "Enders Game"},
				{"10021", "Foundation 1"},
				{"10022", "Stranger in a Strange Land"},
			} {
				if inserted[index].Get("c1").Any() != want.id || inserted[index].Get("c2").Any() != want.title {
					t.Fatalf("contained property-eval insert event %d = %#v, want %#v", index, inserted[index], want)
				}
			}
			if namedWindow {
				window, ok := engine.NamedWindow(targetName)
				if !ok {
					t.Fatal("contained property-eval named window is missing")
				}
				events, err := window.Snapshot(context.Background())
				if err != nil || len(events) != 3 {
					t.Fatalf("contained property-eval named-window snapshot = %#v, err=%v", events, err)
				}
			} else {
				table, ok := engine.Table(targetName)
				if !ok {
					t.Fatal("contained property-eval table is missing")
				}
				rows, err := table.Snapshot(context.Background())
				if err != nil || len(rows) != 3 {
					t.Fatalf("contained property-eval table snapshot = %#v, err=%v", rows, err)
				}
			}
		})
	}
}

func TestTriggerInfraPropertyEvalUpdate(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
			env := NewEnvironment()
			const (
				sourceName = "TriggerPropertyEvalContainer"
				beanName   = "TriggerPropertyEvalBean"
				targetName = "TriggerPropertyEvalUpdateTarget"
			)
			if _, err := RegisterStruct[triggerPropertyEvalContainer](env, sourceName); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[triggerPropertyEvalBean](env, beanName); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				targetSchema, err := RegisterMap(env, targetName, []FieldSpec{
					FieldDef("p0", reflect.TypeOf("")),
					FieldDef("p1", reflect.TypeOf(int(0))),
				})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := CreateNamedWindow(env, targetName, targetSchema, NamedWindowRetention(KeepAll())); err != nil {
					t.Fatal(err)
				}
			} else if _, err := CreateTable(env, targetName, []TableColumn{
				PrimaryKeyColumn[string]("p0"),
				TableColumnOf[int]("p1"),
			}); err != nil {
				t.Fatal(err)
			}

			container := From[triggerPropertyEvalContainer](env, sourceName)
			beans := Unnest[triggerPropertyEvalContainer, triggerPropertyEvalBean](container,
				Property[[]triggerPropertyEvalBean](EventValue[triggerPropertyEvalContainer](), "beans"))
			key := Field[triggerPropertyEvalBean, string]("theString")
			value := Field[triggerPropertyEvalBean, int]("intPrimitive")
			var plan Plan
			var err error
			if namedWindow {
				match := Equal[string](NamedWindowField[string]("p0"), key)
				plan, err = env.Build(OnEvent(beans).MergeIntoNamedWindowWhen(targetName, match,
					WhenMatchedAny(SetColumn("p1", value)),
				).Query(StatementName("trigger-property-eval-update")))
			} else {
				plan, err = env.Build(OnEvent(beans).MergeIntoTableWhen(targetName, []Expr{key},
					WhenMatchedAny(SetColumn("p1", value)),
				).Query(StatementName("trigger-property-eval-update")))
			}
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			if namedWindow {
				if err := engine.InsertNamedWindow(context.Background(), targetName, map[string]any{"p0": "A", "p1": 1}); err != nil {
					t.Fatal(err)
				}
			} else {
				table, ok := engine.Table(targetName)
				if !ok {
					t.Fatal("property-eval update table is missing")
				}
				if _, err := table.Insert(context.Background(), map[string]any{"p0": "A", "p1": 1}); err != nil {
					t.Fatal(err)
				}
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), triggerPropertyEvalContainer{
				Beans: []triggerPropertyEvalBean{{TheString: "A", IntPrimitive: 20}, {TheString: "A", IntPrimitive: 30}},
			}); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				window, ok := engine.NamedWindow(targetName)
				if !ok {
					t.Fatal("property-eval update named window is missing")
				}
				events, err := window.Snapshot(context.Background())
				if err != nil || len(events) != 1 || events[0].Get("p1").Any() != 30 {
					t.Fatalf("property-eval update named-window snapshot = %#v, err=%v", events, err)
				}
			} else {
				table, ok := engine.Table(targetName)
				if !ok {
					t.Fatal("property-eval update table is missing")
				}
				rows, err := table.Snapshot(context.Background())
				if err != nil || len(rows) != 1 || rows[0].Get("p1").Any() != 30 {
					t.Fatalf("property-eval update table snapshot = %#v, err=%v", rows, err)
				}
			}
			_ = deployment
		})
	}
}

func TestTriggerInfraDeleteThenUpdateMatchesEsperTargetSemantics(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
			env := NewEnvironment()
			const (
				sourceName = "TriggerDeleteThenUpdateSource"
				targetName = "TriggerDeleteThenUpdateTarget"
			)
			if _, err := RegisterStruct[triggerMultiActionEvent](env, sourceName); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				schema, err := RegisterMap(env, targetName, []FieldSpec{
					FieldDef("p0", reflect.TypeOf("")),
					FieldDef("p1", reflect.TypeOf(int(0))),
				})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := CreateNamedWindow(env, targetName, schema, NamedWindowRetention(KeepAll())); err != nil {
					t.Fatal(err)
				}
			} else if _, err := CreateTable(env, targetName, []TableColumn{
				PrimaryKeyColumn[string]("p0"),
				TableColumnOf[int]("p1"),
			}); err != nil {
				t.Fatal(err)
			}
			source := From[triggerMultiActionEvent](env, sourceName)
			key := Field[triggerMultiActionEvent, string]("key")
			value := Field[triggerMultiActionEvent, int]("p00")
			actions := WhenMatchedActions(
				ThenDelete(Literal(true)),
				ThenUpdate(Literal(true), SetColumn("p1", value)),
			)
			var plan Plan
			var err error
			if namedWindow {
				plan, err = env.Build(OnEvent(source).MergeIntoNamedWindowWhen(targetName,
					Equal[string](NamedWindowField[string]("p0"), key), actions,
				).Query(StatementName("trigger-delete-then-update")))
			} else {
				plan, err = env.Build(OnEvent(source).MergeIntoTableWhen(targetName, []Expr{key}, actions).
					Query(StatementName("trigger-delete-then-update")))
			}
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			if namedWindow {
				if err := engine.InsertNamedWindow(context.Background(), targetName, map[string]any{"p0": "A", "p1": 1}); err != nil {
					t.Fatal(err)
				}
			} else {
				table, ok := engine.Table(targetName)
				if !ok {
					t.Fatal("delete-then-update table is missing")
				}
				if _, err := table.Insert(context.Background(), map[string]any{"p0": "A", "p1": 1}); err != nil {
					t.Fatal(err)
				}
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), triggerMultiActionEvent{Key: "A", P00: 10}); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				window, ok := engine.NamedWindow(targetName)
				if !ok {
					t.Fatal("delete-then-update named window is missing")
				}
				events, err := window.Snapshot(context.Background())
				if err != nil || len(events) != 1 || events[0].Get("p0").Any() != "A" || events[0].Get("p1").Any() != 10 {
					t.Fatalf("delete-then-update named-window snapshot = %#v, err=%v", events, err)
				}
			} else {
				table, ok := engine.Table(targetName)
				if !ok {
					t.Fatal("delete-then-update table is missing")
				}
				rows, err := table.Snapshot(context.Background())
				if err != nil || len(rows) != 0 {
					t.Fatalf("delete-then-update table snapshot = %#v, err=%v", rows, err)
				}
			}
			_ = deployment
		})
	}
}
