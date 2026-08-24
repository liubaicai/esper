package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type infraTableInsertIntoBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type infraTableInsertIntoS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type infraTableInsertIntoS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

type infraTableInsertIntoS2 struct {
	ID  int    `esper:"id"`
	P20 string `esper:"p20"`
}

const infraTableInsertIntoJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var infraTableInsertIntoJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableInsertInto.java",
}

// Case order fixes the runtime-ID index mapping below.
var infraTableInsertIntoJavaRuntimeIDs = []string{
	"java-runtime-185fe3d8699570122804",
	"java-runtime-6ee6e846c37d6ad8b769",
	"java-runtime-e8d546810d8419632df9",
	"java-runtime-ded301fd8afee9d9cfbd",
	"java-runtime-fa61d002dc3374573126",
}

var infraTableInsertIntoJavaExecutions = []string{
	"InfraInsertIntoAndDelete",
	"InfraInsertIntoSameModuleUnkeyed",
	"InfraInsertIntoTwoModulesUnkeyed",
	"InfraInsertIntoWildcard",
	"InfraInsertIntoSameModuleKeyed",
}

var infraTableInsertIntoCases = []string{
	"insert-delete",
	"same-module-unkeyed",
	"two-modules-unkeyed",
	"wildcard-map",
	"same-module-keyed",
}

// runInfraTableInsertIntoScenario replays five insert-into-table executions.
// The pinned oracle deploys each case's modules before its step stream, so
// the runner deploys at case setup and the steps carry send/snapshot/
// send-error observations only. The wildcard-map case replays the pinned
// DEFAULT representation iteration; the other five representation loops are
// an approved infrastructure difference (underlying-class matrix only).
func runInfraTableInsertIntoScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range infraTableInsertIntoCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runInfraTableInsertIntoCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("infra table insert-into case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("infra table insert-into scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runInfraTableInsertIntoCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	plans, snapshotTables, err := buildInfraTableInsertIntoCase(env, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(infraTableInsertIntoRuntimeID(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	for _, plan := range plans {
		if _, err := engine.Deploy(ctx, plan); err != nil {
			return compat.Trace{}, err
		}
	}

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeInfraTableInsertIntoPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "send-error":
			payload, err := decodeInfraTableInsertIntoPayload(step)
			if err != nil {
				return trace, err
			}
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "send-error",
				Statement: step.EventType,
			}
			// Mirror the oracle's ex.getMessage(): record the bare root
			// cause without the Go error wrapper.
			if sendErr := engine.Send(ctx, step.EventType, payload); sendErr != nil {
				var espErr *esper.Error
				if errors.As(sendErr, &espErr) && espErr.Message != "" {
					record.Value = espErr.Message
				} else {
					record.Value = sendErr.Error()
				}
			} else {
				record.Value = "<no-error>"
			}
			trace.Records = append(trace.Records, record)
		case "snapshot":
			table, ok := snapshotTables[step.Statement]
			if !ok {
				return trace, fmt.Errorf("unknown infra table insert-into snapshot statement %q", step.Statement)
			}
			snapshotPlan, err := env.Build(esper.FromTable(env, table).Query(esper.StatementName("snapshot-" + table)))
			if err != nil {
				return trace, err
			}
			result, err := engine.ExecuteFireAndForget(ctx, snapshotPlan)
			if err != nil {
				return trace, err
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: step.Statement,
				Time:      engine.Now().UTC().Format(time.RFC3339Nano),
				New:       compat.NormalizeResults(result.Results()),
			})
		default:
			return trace, fmt.Errorf("unsupported infra table insert-into step op %q", step.Op)
		}
	}
	return trace, nil
}

// buildInfraTableInsertIntoCase registers the case's schemas and table and
// returns the deployment plans in pinned module order plus the snapshot
// statement-label -> table-name map.
func buildInfraTableInsertIntoCase(env *esper.Environment, caseName string) ([]esper.Plan, map[string]string, error) {
	switch caseName {
	case "insert-delete":
		if _, err := esper.RegisterStruct[infraTableInsertIntoBean](env, "SupportBean"); err != nil {
			return nil, nil, err
		}
		if _, err := esper.RegisterStruct[infraTableInsertIntoS0](env, "SupportBean_S0"); err != nil {
			return nil, nil, err
		}
		if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
			esper.TableColumnOf[int64]("c0"),
			esper.PrimaryKeyColumn[int]("pkey1"),
			esper.PrimaryKeyColumn[string]("pkey0"),
		}); err != nil {
			return nil, nil, err
		}
		insertPlan, err := env.Build(esper.OnEvent(esper.From[infraTableInsertIntoBean](env, "SupportBean")).
			InsertIntoTable("MyTable",
				esper.SetColumn("pkey1", esper.Field[infraTableInsertIntoBean, int]("intPrimitive")),
				esper.SetColumn("c0", esper.Field[infraTableInsertIntoBean, int64]("longPrimitive")),
				esper.SetColumn("pkey0", esper.Field[infraTableInsertIntoBean, string]("theString")),
			).Query(esper.StatementName("Insert-Into-Table")))
		if err != nil {
			return nil, nil, err
		}
		deletePlan, err := env.Build(esper.OnEvent(esper.From[infraTableInsertIntoS0](env, "SupportBean_S0")).
			DeleteFromTableWhere("MyTable",
				esper.And(
					esper.Equal[int](esper.TableField[int]("pkey1"), esper.Field[infraTableInsertIntoS0, int]("id")),
					esper.Equal[string](esper.TableField[string]("pkey0"), esper.Field[infraTableInsertIntoS0, string]("p00")),
				),
			).Query(esper.StatementName("Delete-Table")))
		if err != nil {
			return nil, nil, err
		}
		return []esper.Plan{insertPlan, deletePlan}, map[string]string{"table": "MyTable"}, nil
	case "same-module-unkeyed":
		return buildInfraTableInsertIntoUnkeyed(env, "MyTableSM")
	case "two-modules-unkeyed":
		return buildInfraTableInsertIntoUnkeyed(env, "MyTableIIU")
	case "wildcard-map":
		if _, err := esper.RegisterMap(env, "MySchema", []esper.FieldSpec{
			esper.FieldDef("p0", reflect.TypeOf("")),
			esper.FieldDef("p1", reflect.TypeOf("")),
		}); err != nil {
			return nil, nil, err
		}
		if _, err := esper.CreateTable(env, "TheTable", []esper.TableColumn{
			esper.TableColumnOf[string]("p0"),
			esper.TableColumnOf[string]("p1"),
		}); err != nil {
			return nil, nil, err
		}
		insertPlan, err := env.Build(esper.OnRecord(esper.FromAny(env, "MySchema")).
			InsertIntoTable("TheTable", esper.CopyMatchingFields()).
			Query(esper.StatementName("insert-wildcard")))
		if err != nil {
			return nil, nil, err
		}
		return []esper.Plan{insertPlan}, map[string]string{"create": "TheTable"}, nil
	case "same-module-keyed":
		if _, err := esper.RegisterStruct[infraTableInsertIntoBean](env, "SupportBean"); err != nil {
			return nil, nil, err
		}
		if _, err := esper.RegisterStruct[infraTableInsertIntoS0](env, "SupportBean_S0"); err != nil {
			return nil, nil, err
		}
		if _, err := esper.RegisterStruct[infraTableInsertIntoS1](env, "SupportBean_S1"); err != nil {
			return nil, nil, err
		}
		if _, err := esper.RegisterStruct[infraTableInsertIntoS2](env, "SupportBean_S2"); err != nil {
			return nil, nil, err
		}
		if _, err := esper.CreateTable(env, "MyTableIIK", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("pkey"),
			esper.OptionalTableColumnOf[int]("thesum"),
		}); err != nil {
			return nil, nil, err
		}
		insertPlan, err := env.Build(esper.OnEvent(esper.From[infraTableInsertIntoBean](env, "SupportBean")).
			InsertIntoTable("MyTableIIK",
				esper.SetColumn("pkey", esper.Field[infraTableInsertIntoBean, string]("theString")),
			).Query(esper.StatementName("tbl-insert")))
		if err != nil {
			return nil, nil, err
		}
		aggregatePlan, err := env.Build(
			esper.From[infraTableInsertIntoS0](env, "SupportBean_S0").
				GroupBy(esper.Field[infraTableInsertIntoS0, string]("p00")).
				Select(esper.Alias("thesum", esper.Sum[int](esper.Field[infraTableInsertIntoS0, int]("id")))).
				IntoTable("MyTableIIK", esper.StatementName("tbl-aggregate")),
		)
		if err != nil {
			return nil, nil, err
		}
		onInsertPlan, err := env.Build(esper.OnEvent(esper.From[infraTableInsertIntoS1](env, "SupportBean_S1")).
			InsertIntoTable("MyTableIIK",
				esper.SetColumn("pkey", esper.Field[infraTableInsertIntoS1, string]("p10")),
			).Query(esper.StatementName("on-insert")))
		if err != nil {
			return nil, nil, err
		}
		onMergePlan, err := env.Build(esper.OnEvent(esper.From[infraTableInsertIntoS2](env, "SupportBean_S2")).
			MergeIntoTableWhen("MyTableIIK",
				[]esper.Expr{esper.Field[infraTableInsertIntoS2, string]("p20")},
				esper.WhenNotMatchedAny(esper.SetColumn("pkey", esper.Field[infraTableInsertIntoS2, string]("p20"))),
			).Query(esper.StatementName("on-merge")))
		if err != nil {
			return nil, nil, err
		}
		return []esper.Plan{insertPlan, aggregatePlan, onInsertPlan, onMergePlan}, map[string]string{"create": "MyTableIIK"}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported infra table insert-into case %q", caseName)
	}
}

// buildInfraTableInsertIntoUnkeyed mirrors the unkeyed single-row table
// cases: a theString table fed by an insert-into from SupportBean.
func buildInfraTableInsertIntoUnkeyed(env *esper.Environment, tableName string) ([]esper.Plan, map[string]string, error) {
	if _, err := esper.RegisterStruct[infraTableInsertIntoBean](env, "SupportBean"); err != nil {
		return nil, nil, err
	}
	if _, err := esper.CreateTable(env, tableName, []esper.TableColumn{
		esper.TableColumnOf[string]("theString"),
	}); err != nil {
		return nil, nil, err
	}
	insertPlan, err := env.Build(esper.OnEvent(esper.From[infraTableInsertIntoBean](env, "SupportBean")).
		InsertIntoTable(tableName,
			esper.SetColumn("theString", esper.Field[infraTableInsertIntoBean, string]("theString")),
		).Query(esper.StatementName("tbl-insert")))
	if err != nil {
		return nil, nil, err
	}
	return []esper.Plan{insertPlan}, map[string]string{"create": tableName}, nil
}

func decodeInfraTableInsertIntoPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value infraTableInsertIntoBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value infraTableInsertIntoS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value infraTableInsertIntoS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	case "SupportBean_S2":
		var value infraTableInsertIntoS2
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S2: %w", err)
		}
		return value, nil
	case "MySchema":
		var value map[string]any
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode MySchema: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported infra table insert-into event type %q", step.EventType)
	}
}

func infraTableInsertIntoRuntimeID(caseName string) string {
	for index, name := range infraTableInsertIntoCases {
		if name == caseName {
			return infraTableInsertIntoJavaRuntimeIDs[index]
		}
	}
	return "infra-table-insert-into-unknown"
}
