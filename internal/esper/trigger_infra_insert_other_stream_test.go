package esper

import (
	"context"
	"reflect"
	"testing"
)

// TestTriggerInfraInsertOtherStreamBootstrapMatchesInfraInsertOtherStream
// mirrors InfraNWTableOnMerge.InfraInsertOtherStream for the Go-native
// representations that already have a stable schema/trigger path. The Java
// statement uses an insert-into bootstrap plus a side-stream-only merge. The
// statement order is observable: a Table must be populated before merge runs,
// while a Named Window merge must run before its unique insert route.
func TestTriggerInfraInsertOtherStreamBootstrapMatchesInfraInsertOtherStream(t *testing.T) {
	fields := []FieldSpec{
		FieldDef("name", reflect.TypeOf("")),
		FieldDef("value", reflect.TypeOf(float64(0))),
	}
	for _, representation := range []struct {
		name     string
		register func(*Environment, string, []FieldSpec) (Schema, error)
		send     func(context.Context, *Engine, string, string, float64) error
		assert   func(*testing.T, Event)
	}{
		{
			name: "struct",
			register: func(env *Environment, name string, _ []FieldSpec) (Schema, error) {
				return RegisterStruct[triggerMergeOtherStreamEvent](env, name)
			},
			send: func(ctx context.Context, engine *Engine, eventType, name string, value float64) error {
				return engine.SendEvent(ctx, triggerMergeOtherStreamEvent{Name: name, Value: value})
			},
			assert: func(t *testing.T, event Event) {
				if _, ok := event.Underlying().(triggerMergeOtherStreamEvent); !ok {
					t.Fatalf("struct named-window underlying = %#v", event.Underlying())
				}
			},
		},
		{
			name: "map",
			register: func(env *Environment, name string, fields []FieldSpec) (Schema, error) {
				return RegisterMap(env, name, fields)
			},
			send: func(ctx context.Context, engine *Engine, eventType, name string, value float64) error {
				return engine.SendRecord(ctx, eventType, map[string]any{"name": name, "value": value})
			},
			assert: func(t *testing.T, event Event) {
				underlying, ok := event.Underlying().(map[string]any)
				if !ok || underlying["name"] != "name1" || underlying["value"] != float64(12) {
					t.Fatalf("map named-window underlying = %#v", event.Underlying())
				}
			},
		},
		{
			name: "object-array",
			register: func(env *Environment, name string, fields []FieldSpec) (Schema, error) {
				return RegisterObjectArray(env, name, fields)
			},
			send: func(ctx context.Context, engine *Engine, eventType, name string, value float64) error {
				return engine.SendObjectArray(ctx, eventType, []any{name, value})
			},
			assert: func(t *testing.T, event Event) {
				underlying, ok := event.Underlying().([]any)
				if !ok || len(underlying) != 2 || underlying[0] != "name1" || underlying[1] != float64(12) {
					t.Fatalf("object-array named-window underlying = %#v", event.Underlying())
				}
			},
		},
	} {
		t.Run(representation.name, func(t *testing.T) {
			for _, target := range []struct {
				name        string
				namedWindow bool
			}{
				{name: "table"},
				{name: "named-window", namedWindow: true},
			} {
				t.Run(target.name, func(t *testing.T) {
					const (
						sourceName = "InfraOtherStreamSource"
						targetName = "InfraOtherStreamTarget"
						outputName = "InfraOtherStreamOutput"
					)
					env := NewEnvironment()
					sourceSchema, err := representation.register(env, sourceName, fields)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := RegisterMap(env, outputName, []FieldSpec{
						FieldDef("event_name", reflect.TypeOf("")),
						FieldDef("status", reflect.TypeOf(float64(0))),
					}); err != nil {
						t.Fatal(err)
					}

					if target.namedWindow {
						if _, err := CreateNamedWindow(env, targetName, sourceSchema,
							NamedWindowRetention(Unique(Field[any, string]("name"))),
						); err != nil {
							t.Fatal(err)
						}
					} else if _, err := CreateTable(env, targetName, []TableColumn{
						PrimaryKeyColumn[string]("name"),
						TableColumnOf[float64]("value"),
					}); err != nil {
						t.Fatal(err)
					}

					name := Field[any, string]("name")
					value := Field[any, float64]("value")
					insertQuery := OnRecord(FromAny(env, sourceName))
					var insertPlan Plan
					if target.namedWindow {
						insertPlan, err = env.Build(insertQuery.InsertIntoNamedWindow(targetName,
							SetColumn("name", name),
							SetColumn("value", value),
						).Query(StatementName("infra-other-stream-insert")))
					} else {
						insertPlan, err = env.Build(insertQuery.InsertIntoTable(targetName,
							SetColumn("name", name),
							SetColumn("value", value),
						).Query(StatementName("infra-other-stream-insert")))
					}
					if err != nil {
						t.Fatal(err)
					}

					var mergePlan Plan
					mergeSource := OnRecord(FromAny(env, sourceName))
					if target.namedWindow {
						match := Equal[string](NamedWindowField[string]("name"), name)
						mergePlan, err = env.Build(mergeSource.MergeIntoNamedWindowWhen(targetName, match,
							WhenMatchedActions(
								ThenInsertInto(outputName,
									Alias("event_name", name),
									Alias("status", NamedWindowField[float64]("value")),
								),
							),
							WhenNotMatchedActions(
								ThenInsertInto(outputName,
									Alias("event_name", name),
									Alias("status", Literal(float64(0))),
								),
							),
						).Query(StatementName("infra-other-stream-merge")))
					} else {
						mergePlan, err = env.Build(mergeSource.MergeIntoTableWhen(targetName, []Expr{name},
							WhenMatchedActions(
								ThenInsertInto(outputName,
									Alias("event_name", name),
									Alias("status", TableField[float64]("value")),
								),
							),
							WhenNotMatchedActions(
								ThenInsertInto(outputName,
									Alias("event_name", name),
									Alias("status", Literal(float64(0))),
								),
							),
						).Query(StatementName("infra-other-stream-merge")))
					}
					if err != nil {
						t.Fatal(err)
					}

					consumerPlan, err := env.Build(FromAny(env, outputName).Query(StatementName("infra-other-stream-consumer")))
					if err != nil {
						t.Fatal(err)
					}
					engine := NewEngine(env)
					consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
					if err != nil {
						t.Fatal(err)
					}
					defer func() { _ = consumerDeployment.Undeploy(context.Background()) }()
					var sideEvents []Event
					if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
						for _, result := range batch.New {
							if event, ok := result.Event(); ok {
								sideEvents = append(sideEvents, event)
							}
						}
						return nil
					}); err != nil {
						t.Fatal(err)
					}

					var insertDeployment, mergeDeployment *Deployment
					if target.namedWindow {
						mergeDeployment, err = engine.Deploy(context.Background(), mergePlan)
						if err == nil {
							insertDeployment, err = engine.Deploy(context.Background(), insertPlan)
						}
					} else {
						insertDeployment, err = engine.Deploy(context.Background(), insertPlan)
						if err == nil {
							mergeDeployment, err = engine.Deploy(context.Background(), mergePlan)
						}
					}
					if err != nil {
						if mergeDeployment != nil {
							_ = mergeDeployment.Undeploy(context.Background())
						}
						if insertDeployment != nil {
							_ = insertDeployment.Undeploy(context.Background())
						}
						t.Fatal(err)
					}
					defer func() {
						_ = mergeDeployment.Undeploy(context.Background())
						_ = insertDeployment.Undeploy(context.Background())
					}()

					send := func(name string, value float64) {
						t.Helper()
						if err := representation.send(context.Background(), engine, sourceName, name, value); err != nil {
							t.Fatal(err)
						}
					}
					send("name1", 10)
					wantStatuses := []float64{10}
					if target.namedWindow {
						wantStatuses = []float64{0, 10, 11}
						send("name1", 11)
						send("name1", 12)
					}
					if len(sideEvents) != len(wantStatuses) {
						t.Fatalf("side-stream events = %#v, want %d", sideEvents, len(wantStatuses))
					}
					for index, event := range sideEvents {
						if event.Get("event_name").Any() != "name1" || event.Get("status").Any() != wantStatuses[index] {
							t.Fatalf("side-stream event %d = %#v, want name1/%v", index, event, wantStatuses[index])
						}
					}

					if target.namedWindow {
						window, ok := engine.NamedWindow(targetName)
						if !ok {
							t.Fatal("infra other-stream named window is missing")
						}
						events, snapshotErr := window.Snapshot(context.Background())
						if snapshotErr != nil || len(events) != 1 {
							t.Fatalf("named-window snapshot = %#v, err=%v", events, snapshotErr)
						}
						if events[0].Get("name").Any() != "name1" || events[0].Get("value").Any() != float64(12) {
							t.Fatalf("named-window target = %#v, want name1/12", events[0])
						}
						representation.assert(t, events[0])
					} else {
						table, ok := engine.Table(targetName)
						if !ok {
							t.Fatal("infra other-stream table is missing")
						}
						row, found, snapshotErr := table.Get(context.Background(), "name1")
						if snapshotErr != nil || !found || row.Get("value").Any() != float64(10) {
							t.Fatalf("table target = %#v, found=%v, err=%v, want name1/10", row, found, snapshotErr)
						}
					}
				})
			}
		})
	}
}
