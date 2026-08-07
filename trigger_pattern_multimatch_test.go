package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestTriggerPatternMultimatchMatchesInfraPatternMultimatch(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
			env := NewEnvironment()
			const (
				sourceName = "TriggerPatternSource"
				routeName  = "TriggerPatternMergeSource"
				targetName = "TriggerPatternTarget"
			)
			if _, err := RegisterStruct[triggerInfraFlowEvent](env, sourceName); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterMap(env, routeName, []FieldSpec{
				FieldDef("c1", reflect.TypeOf("")),
				FieldDef("c2", reflect.TypeOf("")),
			}); err != nil {
				t.Fatal(err)
			}

			var targetSchema Schema
			if namedWindow {
				var err error
				targetSchema, err = RegisterMap(env, targetName, []FieldSpec{
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
				PrimaryKeyColumn[string]("c2"),
			}); err != nil {
				t.Fatal(err)
			}

			input := From[triggerInfraFlowEvent](env, sourceName)
			theString := Field[triggerInfraFlowEvent, string]("theString")
			intPrimitive := Field[triggerInfraFlowEvent, int]("intPrimitive")
			pattern := PatternFrom(
				input,
				"a",
				LikeOf(theString, Literal("A%")),
			).FollowedBy(
				"b",
				And(
					LikeOf(theString, Literal("B%")),
					Equal[int](intPrimitive, TagField[int]("a", "intPrimitive")),
				),
			).Every()
			patternPlan, err := env.Build(pattern.Select(
				Alias("c1", TagField[string]("a", "theString")),
				Alias("c2", TagField[string]("b", "theString")),
			).InsertInto(routeName, StatementName("trigger-pattern-source")))
			if err != nil {
				t.Fatal(err)
			}

			route := FromAny(env, routeName)
			c1 := Field[any, string]("c1")
			c2 := Field[any, string]("c2")
			assignments := []TableAssignment{
				SetColumn("c1", c1),
				SetColumn("c2", c2),
			}
			var mergePlan Plan
			if namedWindow {
				match := And(
					Equal[string](NamedWindowField[string]("c1"), c1),
					Equal[string](NamedWindowField[string]("c2"), c2),
				)
				mergePlan, err = env.Build(OnRecord(route).MergeIntoNamedWindowWhen(
					targetName,
					match,
					WhenNotMatchedAny(assignments...),
				).Query(StatementName("trigger-pattern-merge")))
			} else {
				mergePlan, err = env.Build(OnRecord(route).MergeIntoTableWhen(
					targetName,
					[]Expr{c1, c2},
					WhenNotMatchedAny(assignments...),
				).Query(StatementName("trigger-pattern-merge")))
			}
			if err != nil {
				t.Fatal(err)
			}

			engine := NewEngine(env)
			if _, err := engine.Deploy(context.Background(), patternPlan); err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), mergePlan); err != nil {
				t.Fatal(err)
			}
			send := func(name string, value int) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), triggerInfraFlowEvent{TheString: name, IntPrimitive: value}); err != nil {
					t.Fatal(err)
				}
			}
			snapshotSize := func() int {
				t.Helper()
				if namedWindow {
					window, ok := engine.NamedWindow(targetName)
					if !ok {
						t.Fatal("pattern target named window is missing")
					}
					events, err := window.Snapshot(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					return len(events)
				}
				table, ok := engine.Table(targetName)
				if !ok {
					t.Fatal("pattern target table is missing")
				}
				rows, err := table.Snapshot(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				return len(rows)
			}
			assertKeys := func(want map[string]struct{}) {
				t.Helper()
				if namedWindow {
					window, ok := engine.NamedWindow(targetName)
					if !ok {
						t.Fatal("pattern target named window is missing")
					}
					events, err := window.Snapshot(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					if len(events) != len(want) {
						t.Fatalf("pattern named-window rows = %#v, want %d", events, len(want))
					}
					for _, event := range events {
						key := event.Get("c1").Any().(string) + "/" + event.Get("c2").Any().(string)
						if _, ok := want[key]; !ok {
							t.Fatalf("unexpected pattern named-window row %q: %#v", key, event)
						}
					}
					return
				}
				table, ok := engine.Table(targetName)
				if !ok {
					t.Fatal("pattern target table is missing")
				}
				rows, err := table.Snapshot(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != len(want) {
					t.Fatalf("pattern table rows = %#v, want %d", rows, len(want))
				}
				for _, row := range rows {
					key := row.Get("c1").Any().(string) + "/" + row.Get("c2").Any().(string)
					if _, ok := want[key]; !ok {
						t.Fatalf("unexpected pattern table row %q: %#v", key, row)
					}
				}
			}

			send("A1", 1)
			send("A2", 1)
			if got := snapshotSize(); got != 0 {
				t.Fatalf("pattern target populated before right-side event: %d", got)
			}
			send("B1", 1)
			assertKeys(map[string]struct{}{"A1/B1": {}, "A2/B1": {}})

			send("A3", 2)
			send("A4", 2)
			send("B2", 2)
			assertKeys(map[string]struct{}{
				"A1/B1": {},
				"A2/B1": {},
				"A3/B2": {},
				"A4/B2": {},
			})

			// A duplicate completed pattern has the same composite key and must
			// remain a no-op in both target implementations.
			send("A1", 1)
			send("B1", 1)
			assertKeys(map[string]struct{}{
				"A1/B1": {},
				"A2/B1": {},
				"A3/B2": {},
				"A4/B2": {},
			})
		})
	}
}
