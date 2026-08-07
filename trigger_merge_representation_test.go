package esper

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

type triggerMergeRepresentationEvent struct {
	Name  string  `esper:"name" json:"name"`
	Value float64 `esper:"value" json:"value"`
}

type triggerMergeRepresentationJSONEvent struct {
	Name  string  `esper:"name" json:"name"`
	Value float64 `esper:"value" json:"value"`
}

func TestTriggerMergeInsertOtherStreamRepresentationMatrix(t *testing.T) {
	for _, representation := range []string{"map", "object-array", "avro", "json", "json-class-provided", "default"} {
		for _, namedWindow := range []bool{true, false} {
			t.Run(representation+"/"+map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
				env := NewEnvironment()
				const (
					sourceName = "TriggerMergeRepresentationSource"
					targetName = "TriggerMergeRepresentationTarget"
					outputName = "TriggerMergeRepresentationOutput"
				)
				fields := []FieldSpec{
					FieldDef("name", reflect.TypeOf("")),
					FieldDef("value", reflect.TypeOf(float64(0))),
				}

				var (
					sourceSchema Schema
					targetSchema Schema
					send         func(*Engine, string, float64) error
					underlying   func(string, float64) (any, error)
					err          error
				)
				switch representation {
				case "map":
					sourceSchema, err = RegisterMap(env, sourceName, fields)
					if err != nil {
						t.Fatal(err)
					}
					if namedWindow {
						targetSchema, err = RegisterMap(env, targetName, fields)
						if err != nil {
							t.Fatal(err)
						}
					}
					send = func(engine *Engine, name string, value float64) error {
						return engine.SendRecord(context.Background(), sourceName, map[string]any{"name": name, "value": value})
					}
					underlying = func(name string, value float64) (any, error) {
						return map[string]any{"name": name, "value": value}, nil
					}
				case "object-array":
					sourceSchema, err = RegisterObjectArray(env, sourceName, fields)
					if err != nil {
						t.Fatal(err)
					}
					if namedWindow {
						targetSchema, err = RegisterObjectArray(env, targetName, fields)
						if err != nil {
							t.Fatal(err)
						}
					}
					send = func(engine *Engine, name string, value float64) error {
						return engine.SendObjectArray(context.Background(), sourceName, []any{name, value})
					}
					underlying = func(name string, value float64) (any, error) {
						return []any{name, value}, nil
					}
				case "avro":
					sourceSchema, err = RegisterAvro(env, sourceName, fields)
					if err != nil {
						t.Fatal(err)
					}
					if namedWindow {
						targetSchema, err = RegisterAvro(env, targetName, fields)
						if err != nil {
							t.Fatal(err)
						}
					}
					makeRecord := func(schema Schema, name string, value float64) (*AvroRecord, error) {
						record, recordErr := NewAvroRecord(schema)
						if recordErr != nil {
							return nil, recordErr
						}
						if recordErr = record.Set("name", name); recordErr != nil {
							return nil, recordErr
						}
						if recordErr = record.Set("value", value); recordErr != nil {
							return nil, recordErr
						}
						return record, nil
					}
					send = func(engine *Engine, name string, value float64) error {
						record, recordErr := makeRecord(sourceSchema, name, value)
						if recordErr != nil {
							return recordErr
						}
						return engine.SendAvro(context.Background(), sourceName, record)
					}
					underlying = func(name string, value float64) (any, error) {
						return makeRecord(targetSchema, name, value)
					}
				case "json":
					sourceSchema, err = RegisterJSON(env, sourceName, fields)
					if err != nil {
						t.Fatal(err)
					}
					if namedWindow {
						targetSchema, err = RegisterJSON(env, targetName, fields)
						if err != nil {
							t.Fatal(err)
						}
					}
					send = func(engine *Engine, name string, value float64) error {
						data, marshalErr := json.Marshal(map[string]any{"name": name, "value": value})
						if marshalErr != nil {
							return marshalErr
						}
						return engine.SendJSON(context.Background(), sourceName, data)
					}
					underlying = func(name string, value float64) (any, error) {
						return map[string]any{"name": name, "value": value}, nil
					}
				case "json-class-provided":
					sourceSchema, err = RegisterJSONFor[triggerMergeRepresentationJSONEvent](env, sourceName, nil)
					if err != nil {
						t.Fatal(err)
					}
					if namedWindow {
						targetSchema, err = RegisterJSONFor[triggerMergeRepresentationJSONEvent](env, targetName, nil)
						if err != nil {
							t.Fatal(err)
						}
					}
					send = func(engine *Engine, name string, value float64) error {
						data, marshalErr := json.Marshal(triggerMergeRepresentationJSONEvent{Name: name, Value: value})
						if marshalErr != nil {
							return marshalErr
						}
						return engine.SendJSON(context.Background(), sourceName, data)
					}
					underlying = func(name string, value float64) (any, error) {
						return triggerMergeRepresentationJSONEvent{Name: name, Value: value}, nil
					}
				case "default":
					sourceSchema, err = RegisterStruct[triggerMergeRepresentationEvent](env, sourceName)
					if err != nil {
						t.Fatal(err)
					}
					if namedWindow {
						targetSchema, err = RegisterStruct[triggerMergeRepresentationEvent](env, targetName)
						if err != nil {
							t.Fatal(err)
						}
					}
					send = func(engine *Engine, name string, value float64) error {
						return engine.SendRecord(context.Background(), sourceName, map[string]any{"name": name, "value": value})
					}
					underlying = func(name string, value float64) (any, error) {
						return triggerMergeRepresentationEvent{Name: name, Value: value}, nil
					}
				}

				if sourceSchema.Name() == "" {
					t.Fatal("representation source schema is missing")
				}
				if _, err := RegisterMap(env, outputName, []FieldSpec{
					FieldDef("event_name", reflect.TypeOf("")),
					FieldDef("status", reflect.TypeOf(float64(0))),
				}); err != nil {
					t.Fatal(err)
				}
				if namedWindow {
					if _, err := CreateNamedWindow(env, targetName, targetSchema, NamedWindowRetention(Unique(Field[any, string]("name")))); err != nil {
						t.Fatal(err)
					}
				} else if _, err := CreateTable(env, targetName, []TableColumn{
					PrimaryKeyColumn[string]("name"),
					TableColumnOf[float64]("value"),
				}); err != nil {
					t.Fatal(err)
				}

				source := FromAny(env, sourceName)
				sourceNameExpr := Field[any, string]("name")
				sourceValueExpr := Field[any, float64]("value")
				var match Expr
				var targetValue Expr
				if namedWindow {
					match = Equal[string](NamedWindowField[string]("name"), sourceNameExpr)
					targetValue = NamedWindowField[float64]("value")
				} else {
					match = nil
					targetValue = TableField[float64]("value")
				}
				clauses := []TableMergeClause{
					WhenMatchedActions(ThenInsertInto(
						outputName,
						Alias("event_name", sourceNameExpr),
						Alias("status", targetValue),
					)),
					WhenNotMatchedActions(
						ThenInsertInto(outputName,
							Alias("event_name", sourceNameExpr),
							Alias("status", Literal(0.0)),
						),
						ThenInsertIntoTarget(
							SetColumn("name", sourceNameExpr),
							SetColumn("value", sourceValueExpr),
						),
					),
				}
				var mergePlan Plan
				if namedWindow {
					mergePlan, err = env.Build(OnRecord(source).MergeIntoNamedWindowWhen(targetName,
						match.(Expression[bool]), clauses...).Query(StatementName("trigger-merge-representation")))
				} else {
					mergePlan, err = env.Build(OnRecord(source).MergeIntoTableWhen(targetName,
						[]Expr{sourceNameExpr}, clauses...).Query(StatementName("trigger-merge-representation")))
				}
				if err != nil {
					t.Fatal(err)
				}
				consumerPlan, err := env.Build(FromAny(env, outputName).Query(StatementName("trigger-merge-representation-output")))
				if err != nil {
					t.Fatal(err)
				}

				engine := NewEngine(env)
				if !namedWindow {
					table, ok := engine.Table(targetName)
					if !ok {
						t.Fatal("representation target table is missing")
					}
					if _, err := table.Insert(context.Background(), map[string]any{"name": "name1", "value": 10.0}); err != nil {
						t.Fatal(err)
					}
				}
				consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
				if err != nil {
					t.Fatal(err)
				}
				var statuses []float64
				if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
					for _, result := range batch.New {
						event, ok := result.Event()
						if !ok {
							return NewError(ErrorTypeMismatch, "representation side-stream result is not an event")
						}
						value, ok := event.Get("status").Any().(float64)
						if !ok {
							return NewError(ErrorTypeMismatch, "representation side-stream status is not float64")
						}
						statuses = append(statuses, value)
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				mergeDeployment, err := engine.Deploy(context.Background(), mergePlan)
				if err != nil {
					t.Fatal(err)
				}

				if err := send(engine, "name1", 10); err != nil {
					t.Fatal(err)
				}
				wantFirst := 0.0
				if !namedWindow {
					wantFirst = 10.0
				}
				if len(statuses) != 1 || statuses[0] != wantFirst {
					t.Fatalf("%s/%t first side-stream statuses = %#v, want %v", representation, namedWindow, statuses, wantFirst)
				}

				if namedWindow {
					window, ok := engine.NamedWindow(targetName)
					if !ok {
						t.Fatal("representation target named window is missing")
					}
					events, err := window.Snapshot(context.Background())
					if err != nil || len(events) != 1 || events[0].Get("value").Any() != 10.0 {
						t.Fatalf("%s target after not-matched insert = %#v, err=%v", representation, events, err)
					}
					replacement, err := underlying("name1", 11)
					if err != nil {
						t.Fatal(err)
					}
					if err := engine.InsertNamedWindow(context.Background(), targetName, replacement); err != nil {
						t.Fatal(err)
					}
					if err := send(engine, "name1", 12); err != nil {
						t.Fatal(err)
					}
					if len(statuses) != 2 || statuses[1] != 11.0 {
						t.Fatalf("%s named-window matched side-stream statuses = %#v, want [0 11]", representation, statuses)
					}
				} else {
					table, ok := engine.Table(targetName)
					if !ok {
						t.Fatal("representation target table is missing")
					}
					rows, err := table.Snapshot(context.Background())
					if err != nil || len(rows) != 1 || rows[0].Get("value").Any() != 10.0 {
						t.Fatalf("%s table after matched side-stream = %#v, err=%v", representation, rows, err)
					}
				}
				if err := mergeDeployment.Undeploy(context.Background()); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
