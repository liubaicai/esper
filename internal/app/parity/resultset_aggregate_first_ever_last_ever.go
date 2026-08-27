package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetAggregateFirstEverLastEverBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	IntBoxed      *int   `esper:"intBoxed"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
}

type resultsetAggregateFirstEverLastEverTrigger struct {
	ID string `esper:"id"`
}

const resultsetAggregateFirstEverLastEverJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateFirstEverLastEverJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFirstEverLastEver.java",
}

var (
	resultsetAggregateFirstEverLastEverJavaRuntimeIDs = []string{
		"java-runtime-b1fdc8c4a3c8eaef8273", // ResultSetAggregateFirstLastEver{soda=true}
		"java-runtime-2927c28d6990516aebea", // ResultSetAggregateFirstLastEver{soda=false}
		"java-runtime-2dcfc3045d996cb718ab", // ResultSetAggregateOnDelete
	}
	resultsetAggregateFirstEverLastEverJavaExecutions = []string{
		"ResultSetAggregateFirstLastEver{soda=true}",
		"ResultSetAggregateFirstLastEver{soda=false}",
		"ResultSetAggregateOnDelete",
	}
)

const (
	resultsetAggregateFirstEverLastEverSODATrueCase  = "first-last-ever-soda-true"
	resultsetAggregateFirstEverLastEverSODAFalseCase = "first-last-ever-soda-false"
	resultsetAggregateFirstEverLastEverOnDeleteCase  = "on-delete"
)

// runResultSetAggregateFirstEverLastEverScenario replays the two first/last
// ever aggregate statements and the named-window deletion statement from
// ResultSetAggregateFirstEverLastEver. The invalid countever(distinct ...)
// execution is intentionally not represented by this runtime scenario.
func runResultSetAggregateFirstEverLastEverScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		resultsetAggregateFirstEverLastEverSODATrueCase,
		resultsetAggregateFirstEverLastEverSODAFalseCase,
		resultsetAggregateFirstEverLastEverOnDeleteCase,
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetAggregateFirstEverLastEverCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-first-ever-last-ever case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-first-ever-last-ever scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateFirstEverLastEverCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}

	env := esper.NewEnvironment()
	beanSchema, err := esper.RegisterStruct[resultsetAggregateFirstEverLastEverBean](env, "SupportBean")
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetAggregateFirstEverLastEverTrigger](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateFirstEverLastEverRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	var statement *esper.Statement
	if caseName == resultsetAggregateFirstEverLastEverOnDeleteCase {
		if _, err := esper.CreateNamedWindow(env, "MyWindow", beanSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		beanSource := esper.From[resultsetAggregateFirstEverLastEverBean](env, "SupportBean")
		insertPlan, err := env.Build(esper.OnEvent(beanSource).InsertIntoNamedWindow(
			"MyWindow", esper.CopyMatchingFields(),
		).Query(esper.StatementName("insert")))
		if err != nil {
			return compat.Trace{}, err
		}
		deletePlan, err := env.Build(esper.OnEvent(
			esper.From[resultsetAggregateFirstEverLastEverTrigger](env, "SupportBean_A"),
		).DeleteFromNamedWindow(
			"MyWindow",
			esper.Equal[string](
				esper.NamedWindowField[string]("theString"),
				esper.Field[resultsetAggregateFirstEverLastEverTrigger, string]("id"),
			),
		).Query(esper.StatementName("delete")))
		if err != nil {
			return compat.Trace{}, err
		}
		theString := esper.Field[any, string]("theString")
		aggregatePlan, err := env.Build(esper.FromNamedWindow(env, "MyWindow").Aggregate(
			esper.Alias("firsteverstring", esper.FirstEver[string](theString)),
			esper.Alias("lasteverstring", esper.LastEver[string](theString)),
			esper.Alias("counteverall", esper.CountEver()),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return compat.Trace{}, err
		}
		for _, plan := range []esper.Plan{insertPlan, deletePlan} {
			if _, err := engine.Deploy(ctx, plan); err != nil {
				return compat.Trace{}, err
			}
		}
		deployment, err := engine.Deploy(ctx, aggregatePlan)
		if err != nil {
			return compat.Trace{}, err
		}
		statements := deployment.Statements()
		if len(statements) != 1 {
			return compat.Trace{}, fmt.Errorf("expected one resultset aggregate statement, got %d", len(statements))
		}
		statement = statements[0]
	} else {
		theString := esper.Field[resultsetAggregateFirstEverLastEverBean, string]("theString")
		intBoxed := esper.Field[resultsetAggregateFirstEverLastEverBean, *int]("intBoxed")
		boolPrimitive := esper.Field[resultsetAggregateFirstEverLastEverBean, bool]("boolPrimitive")
		query := esper.From[resultsetAggregateFirstEverLastEverBean](env, "SupportBean").
			Window(esper.LengthWindow(2)).
			Aggregate(
				esper.Alias("firsteverstring", esper.FirstEver[string](theString)),
				esper.Alias("lasteverstring", esper.LastEver[string](theString)),
				esper.Alias("firststring", esper.First[string](theString)),
				esper.Alias("laststring", esper.Last[string](theString)),
				esper.Alias("cntstar", esper.CountEver()),
				esper.Alias("cntexpr", esper.CountEver(intBoxed)),
				esper.Alias("cntstarfiltered", esper.FilterAggregate[int64](esper.CountEver(), boolPrimitive)),
				esper.Alias("cntexprfiltered", esper.FilterAggregate[int64](esper.CountEver(intBoxed), boolPrimitive)),
			).
			Query(esper.StatementName("s0"), esper.StatementAudit())
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return compat.Trace{}, err
		}
		statements := deployment.Statements()
		if len(statements) != 1 {
			return compat.Trace{}, fmt.Errorf("expected one resultset aggregate statement, got %d", len(statements))
		}
		statement = statements[0]
	}
	if statement == nil {
		return compat.Trace{}, fmt.Errorf("resultset aggregate case %q did not deploy statement s0", caseName)
	}

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateFirstEverLastEverPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-first-ever-last-ever statement %q", name)
		}
		return statement, nil
	})
}

func resultsetAggregateFirstEverLastEverRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetAggregateFirstEverLastEverSODATrueCase:
		return resultsetAggregateFirstEverLastEverJavaRuntimeIDs[0]
	case resultsetAggregateFirstEverLastEverSODAFalseCase:
		return resultsetAggregateFirstEverLastEverJavaRuntimeIDs[1]
	case resultsetAggregateFirstEverLastEverOnDeleteCase:
		return resultsetAggregateFirstEverLastEverJavaRuntimeIDs[2]
	default:
		return "parity-resultset-aggregate-first-ever-last-ever-" + caseName
	}
}

func decodeResultSetAggregateFirstEverLastEverPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value resultsetAggregateFirstEverLastEverBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_A":
		var value resultsetAggregateFirstEverLastEverTrigger
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported resultset-aggregate-first-ever-last-ever event type %q", step.EventType)
	}
}
