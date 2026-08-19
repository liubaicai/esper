package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type infraFAFSceneTwoBean struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
}

const infraFAFSceneTwoJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var infraFAFSceneTwoJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableFAF.java",
}

var (
	infraFAFSceneTwoJavaRuntimeIDs = []string{
		"java-runtime-04b48c1da9e5a7b3355a",
		"java-runtime-d4ef17a6d0f527d8ecad",
	}
	infraFAFSceneTwoJavaExecutions = []string{
		"InfraSelectWildcardSceneTwo{namedWindow=true}",
		"InfraSelectWildcardSceneTwo{namedWindow=false}",
	}
)

func runInfraFAFSceneTwoScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range []string{"named-window", "table"} {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseTrace, err := runInfraFAFSceneTwoCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("infra FAF scene-two case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("infra FAF scene-two scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runInfraFAFSceneTwoCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraFAFSceneTwoBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	namedWindow := caseName == "named-window"
	var source esper.RecordStream
	if namedWindow {
		schema, err := esper.NewMapSchema("InfraFAFSceneTwoSchema", []esper.FieldSpec{
			esper.FieldDef("key", reflect.TypeOf("")),
			esper.FieldDef("value", reflect.TypeOf(0)),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterSchema(schema); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyInfra", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		source = esper.FromNamedWindow(env, "MyInfra")
	} else {
		if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("key"),
			esper.TableColumnOf[int]("value"),
		}); err != nil {
			return compat.Trace{}, err
		}
		source = esper.FromTable(env, "MyInfra")
	}

	eventSource := esper.From[infraFAFSceneTwoBean](env, "SupportBean")
	assignments := []esper.TableAssignment{
		esper.SetColumn("key", esper.Field[infraFAFSceneTwoBean, string]("theString")),
		esper.SetColumn("value", esper.Field[infraFAFSceneTwoBean, int]("intBoxed")),
	}
	var insertPlan esper.Plan
	if namedWindow {
		insertPlan, err = env.Build(esper.OnEvent(eventSource).InsertIntoNamedWindow("MyInfra", assignments...).Query(esper.StatementName("insert")))
	} else {
		insertPlan, err = env.Build(esper.OnEvent(eventSource).InsertIntoTable("MyInfra", assignments...).Query(esper.StatementName("insert")))
	}
	if err != nil {
		return compat.Trace{}, err
	}

	positivePlan, err := env.Build(source.Filter(
		esper.Greater[int](esper.Field[any, int]("value"), esper.Literal[int](0)),
	).Query(esper.StatementName("faf-positive")))
	if err != nil {
		return compat.Trace{}, err
	}
	negativePlan, err := env.Build(source.Filter(
		esper.Less[int](esper.Field[any, int]("value"), esper.Literal[int](0)),
	).Query(esper.StatementName("faf-negative")))
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env, esper.WithRuntimeURI(infraFAFSceneTwoRuntimeID(caseName)))
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(ctx, insertPlan); err != nil {
		return compat.Trace{}, err
	}
	plans := map[string]esper.Plan{
		"faf-positive": positivePlan,
		"faf-negative": negativePlan,
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, step := range caseScenario.Steps {
		if err := infraFAFSceneTwoContextErr(ctx); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeInfraFAFSceneTwoPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "snapshot":
			plan, ok := plans[step.Statement]
			if !ok {
				return trace, fmt.Errorf("unknown FAF snapshot statement %q", step.Statement)
			}
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil {
				return trace, err
			}
			rows := compat.NormalizeResults(result.Results())
			if step.Statement != "faf-positive" || !namedWindow {
				rows = sortInfraFAFSceneTwoRows(rows)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: step.Statement,
				Time:      engine.Now().UTC().Format(time.RFC3339Nano),
				New:       rows,
			})
		default:
			return trace, fmt.Errorf("unsupported infra FAF scene-two step %q", step.Op)
		}
	}
	return trace, nil
}

func infraFAFSceneTwoRuntimeID(caseName string) string {
	if caseName == "named-window" {
		return infraFAFSceneTwoJavaRuntimeIDs[0]
	}
	return infraFAFSceneTwoJavaRuntimeIDs[1]
}

func decodeInfraFAFSceneTwoPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported infra FAF scene-two event type %q", step.EventType)
	}
	var value infraFAFSceneTwoBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}

func sortInfraFAFSceneTwoRows(rows []compat.ResultRecord) []compat.ResultRecord {
	rows = append([]compat.ResultRecord(nil), rows...)
	sort.SliceStable(rows, func(i, j int) bool {
		leftKey := fmt.Sprint(rows[i].Fields["key"])
		rightKey := fmt.Sprint(rows[j].Fields["key"])
		if leftKey != rightKey {
			return leftKey < rightKey
		}
		return fmt.Sprint(rows[i].Fields["value"]) < fmt.Sprint(rows[j].Fields["value"])
	})
	return rows
}

func infraFAFSceneTwoContextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
