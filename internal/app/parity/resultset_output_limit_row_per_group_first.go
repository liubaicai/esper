package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetOutputLimitRowPerGroupFirstJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetOutputLimitRowPerGroupFirstJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroup.java",
}

var resultsetOutputLimitRowPerGroupFirstJavaRuntimeIDs = []string{
	"java-runtime-ebcdaaea9afc00a686d0", // ResultSetOutputFirstWhenThen (ord 0)
	"java-runtime-e499360495e769be65ec", // ResultSetOutputFirstHavingJoinNoJoin (ord 36)
	"java-runtime-198f226a1080d3e02cf0", // ResultSetOutputFirstCrontab (ord 37)
	"java-runtime-97e7a2a759efa357a2af", // ResultSetOutputFirstEveryNEvents (ord 38)
}

var resultsetOutputLimitRowPerGroupFirstJavaExecutions = []string{
	"ResultSetOutputLimitRowPerGroup$ResultSetOutputFirstWhenThen",
	"ResultSetOutputLimitRowPerGroup$ResultSetOutputFirstHavingJoinNoJoin",
	"ResultSetOutputLimitRowPerGroup$ResultSetOutputFirstCrontab",
	"ResultSetOutputLimitRowPerGroup$ResultSetOutputFirstEveryNEvents",
}

const (
	resultsetOutputLimitRowPerGroupFirstID = "resultset-output-limit-row-per-group-first"

	resultsetOutputLimitRowPerGroupFirstWhenThenCase = "first-when-then"
	resultsetOutputLimitRowPerGroupFirstHavingCase   = "first-having"
	resultsetOutputLimitRowPerGroupFirstCrontabCase  = "first-crontab"
	resultsetOutputLimitRowPerGroupFirstEveryNCase   = "first-every-n"
)

var resultsetOutputLimitRowPerGroupFirstCaseOrder = []string{
	resultsetOutputLimitRowPerGroupFirstWhenThenCase,
	resultsetOutputLimitRowPerGroupFirstHavingCase,
	resultsetOutputLimitRowPerGroupFirstCrontabCase,
	resultsetOutputLimitRowPerGroupFirstEveryNCase,
}

// rowPerGroupFirstBean is the SupportBean carrier for the named-window rows.
type rowPerGroupFirstBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// rowPerGroupFirstMarket is the SupportMarketDataBean delete trigger.
type rowPerGroupFirstMarket struct {
	Symbol string  `esper:"symbol"`
	Volume int64   `esper:"volume"`
	Price  float64 `esper:"price"`
}

// rowPerGroupFirstBeanA is the SupportBean_A join partner (id only).
type rowPerGroupFirstBeanA struct {
	ID string `esper:"id"`
}

// runResultSetOutputLimitRowPerGroupFirstScenario replays the four
// output-first executions from ResultSetOutputLimitRowPerGroup ordinals
// 0/36/37/38 over a named-window grouped source.  Each Java execution is one
// scenario case; the first-having case runs the four EPL variants (plain,
// join, order-by, order-by-join) as sequential deploy/undeploy cycles inside
// one runtime, matching the single Java execution.
func runResultSetOutputLimitRowPerGroupFirstScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetOutputLimitRowPerGroupFirstCaseOrder))
	for _, caseName := range resultsetOutputLimitRowPerGroupFirstCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetOutputLimitRowPerGroupFirstCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitRowPerGroupFirstID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOutputLimitRowPerGroupFirstID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetOutputLimitRowPerGroupFirstCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultsetOutputLimitRowPerGroupFirstID}
	var sequence uint64
	record := func(batch esper.ResultBatch) {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return
		}
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
	}

	var env *esper.Environment
	var engine *esper.Engine
	deployments := map[string]*esper.Deployment{}
	deployIndex := 0 // first-having variant index

	freshEngine := func() error {
		if engine != nil {
			_ = engine.Close(context.Background())
		}
		env = esper.NewEnvironment()
		if _, err := esper.RegisterStruct[rowPerGroupFirstBean](env, "SupportBean"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[rowPerGroupFirstMarket](env, "SupportMarketDataBean"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[rowPerGroupFirstBeanA](env, "SupportBean_A"); err != nil {
			return err
		}
		switch caseName {
		case resultsetOutputLimitRowPerGroupFirstWhenThenCase:
			if err := env.RegisterVariable("varoutone", false); err != nil {
				return err
			}
		case resultsetOutputLimitRowPerGroupFirstEveryNCase:
			if err := env.RegisterVariable("myvar_local", 1); err != nil {
				return err
			}
		}
		engine = esper.NewEngine(env,
			esper.WithStartTime(time.Unix(0, 0).UTC()),
			esper.WithRuntimeURI(resultsetOutputLimitRowPerGroupFirstRuntimeURI(caseName)),
		)
		deployments = map[string]*esper.Deployment{}
		return nil
	}

	deploy := func(statement string) error {
		var plans []esper.Plan
		var err error
		switch statement {
		case "infra":
			plans, err = rowPerGroupFirstInfraPlans(env)
		case "s0", "s0-var":
			var query esper.Query
			query, err = rowPerGroupFirstQuery(env, caseName, deployIndex)
			if err == nil {
				plans = []esper.Plan{}
				var plan esper.Plan
				plan, err = env.Build(query)
				if err == nil {
					plans = append(plans, plan)
				}
			}
			deployIndex++
		case "all":
			plans, err = rowPerGroupFirstInfraPlans(env)
			if err == nil {
				var query esper.Query
				query, err = rowPerGroupFirstQuery(env, caseName, deployIndex)
				if err == nil {
					var plan esper.Plan
					plan, err = env.Build(query)
					if err == nil {
						plans = append(plans, plan)
					}
				}
				deployIndex++
			}
		default:
			return fmt.Errorf("unexpected deploy statement %q", statement)
		}
		if err != nil {
			return fmt.Errorf("build %q: %w", statement, err)
		}
		deployment, err := engine.DeployPlans(ctx, plans)
		if err != nil {
			return fmt.Errorf("deploy %q: %w", statement, err)
		}
		deployments[statement] = deployment
		for _, stmt := range deployment.Statements() {
			if stmt.Name() == "s0" {
				if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					record(batch)
					return nil
				}); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
		case "deploy":
			if engine == nil || step.Statement == "infra" || step.Statement == "all" {
				if err := freshEngine(); err != nil {
					return compat.Trace{}, err
				}
			}
			if err := deploy(step.Statement); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy":
			deployment, ok := deployments[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("%s undeploys unknown statement %q", caseName, step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return compat.Trace{}, fmt.Errorf("undeploy %q: %w", step.Statement, err)
			}
			delete(deployments, step.Statement)
		case "undeploy-all":
			for name, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, fmt.Errorf("undeploy-all %q: %w", name, err)
				}
			}
			deployments = map[string]*esper.Deployment{}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("%s advance-time: %w", resultsetOutputLimitRowPerGroupFirstID, err)
			}
			if engine != nil {
				if err := engine.AdvanceTime(ctx, at); err != nil {
					return compat.Trace{}, err
				}
			}
		case "set-variable":
			value, err := decodeResultSetOutputLimitRowPerGroupFirstVariable(step)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("%s set-variable: %w", resultsetOutputLimitRowPerGroupFirstID, err)
			}
			if err := engine.SetVariable(ctx, step.Name, value); err != nil {
				return compat.Trace{}, err
			}
		case "send":
			payload, err := decodeResultSetOutputLimitRowPerGroupFirstPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", resultsetOutputLimitRowPerGroupFirstID, step.Op)
		}
	}
	if engine != nil {
		_ = engine.Close(context.Background())
	}
	return trace, nil
}

// rowPerGroupFirstInfraPlans builds the three named-window infra statements:
// create MyWindow#keepall as SupportBean, insert into MyWindow select * from
// SupportBean, on SupportMarketDataBean delete from MyWindow where
// intPrimitive = price.
func rowPerGroupFirstInfraPlans(env *esper.Environment) ([]esper.Plan, error) {
	schema, ok := env.Schema("SupportBean")
	if !ok {
		return nil, fmt.Errorf("SupportBean schema not registered")
	}
	if _, err := esper.CreateNamedWindow(env, "MyWindow", schema,
		esper.NamedWindowRetention(esper.KeepAll())); err != nil {
		return nil, err
	}
	createPlan, err := env.Build(esper.FromNamedWindow(env, "MyWindow").
		CreateNamedWindowQuery(esper.StatementName("create")))
	if err != nil {
		return nil, err
	}
	insertPlan, err := env.Build(esper.OnEvent(esper.From[rowPerGroupFirstBean](env, "SupportBean")).
		InsertIntoNamedWindow("MyWindow",
			esper.SetColumn("theString", esper.Field[rowPerGroupFirstBean, string]("theString")),
			esper.SetColumn("intPrimitive", esper.Field[rowPerGroupFirstBean, int]("intPrimitive")),
		).Query(esper.StatementName("insert")))
	if err != nil {
		return nil, err
	}
	deletePlan, err := env.Build(esper.OnEvent(esper.From[rowPerGroupFirstMarket](env, "SupportMarketDataBean")).
		DeleteFromNamedWindow("MyWindow",
			esper.Equal[float64](
				esper.Cast[int, float64](esper.NamedWindowField[int]("intPrimitive")),
				esper.Field[rowPerGroupFirstMarket, float64]("price"))).
		Query(esper.StatementName("delete")))
	if err != nil {
		return nil, err
	}
	return []esper.Plan{createPlan, insertPlan, deletePlan}, nil
}

// rowPerGroupFirstQuery builds the s0 select for one case.  deployIndex
// selects the first-having EPL variant: 0 plain, 1 join, 2 order-by,
// 3 order-by-join.  Other cases ignore the index.
func rowPerGroupFirstQuery(env *esper.Environment, caseName string, deployIndex int) (esper.Query, error) {
	theString := esper.Field[rowPerGroupFirstBean, string]("theString")
	intPrimitive := esper.Field[rowPerGroupFirstBean, int]("intPrimitive")
	window := esper.FromNamedWindowAs[rowPerGroupFirstBean](env, "MyWindow")

	switch caseName {
	case resultsetOutputLimitRowPerGroupFirstWhenThenCase:
		return window.GroupBy(theString).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("value", esper.Sum[int](intPrimitive)),
			).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputFirstWhen(
				esper.VariableRef[bool]("varoutone"),
				esper.SetOutputVariable("varoutone", esper.Literal(false)),
			)),
		), nil

	case resultsetOutputLimitRowPerGroupFirstHavingCase:
		isJoin := deployIndex == 1 || deployIndex == 3
		hasOrderBy := deployIndex == 2 || deployIndex == 3
		options := []esper.QueryOption{
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputFirstEveryEvents(2)),
		}
		if isJoin {
			joinTheString := esper.JoinField[string](0, "theString")
			joinSum := esper.Sum[int](esper.JoinField[int](0, "intPrimitive"))
			grouped := esper.Join(
				window,
				esper.From[rowPerGroupFirstBeanA](env, "SupportBean_A").Window(esper.KeepAll()),
				esper.OnEqual(theString, esper.Field[rowPerGroupFirstBeanA, string]("id")),
			).GroupBy(joinTheString).Select(
				esper.Alias("theString", joinTheString),
				esper.Alias("value", joinSum),
			).Having(esper.Greater[int](joinSum, esper.Literal(20)))
			if hasOrderBy {
				options = append(options, esper.OrderBy(esper.Ascending(esper.ResultField[string]("theString"))))
			}
			return grouped.Query(options...), nil
		}
		sum := esper.Sum[int](intPrimitive)
		grouped := window.GroupBy(theString).Select(
			esper.Alias("theString", theString),
			esper.Alias("value", sum),
		).Having(esper.Greater[int](sum, esper.Literal(20)))
		if hasOrderBy {
			options = append(options, esper.OrderBy(esper.Ascending(esper.ResultField[string]("theString"))))
		}
		return grouped.Query(options...), nil

	case resultsetOutputLimitRowPerGroupFirstCrontabCase:
		return window.GroupBy(theString).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("value", esper.Sum[int](intPrimitive)),
			).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputFirstAt(esper.NewCronSchedule(
				esper.CronEvery(2),
				esper.CronWildcard(),
				esper.CronWildcard(),
				esper.CronWildcard(),
				esper.CronWildcard(),
			))),
		), nil

	case resultsetOutputLimitRowPerGroupFirstEveryNCase:
		if deployIndex == 0 {
			return window.GroupBy(theString).
				Select(
					esper.Alias("theString", theString),
					esper.Alias("value", esper.Sum[int](intPrimitive)),
				).Query(
				esper.StatementName("s0"),
				esper.WithOutput(esper.OutputFirstEveryEvents(3)),
			), nil
		}
		return window.GroupBy(theString).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("value", esper.Sum[int](intPrimitive)),
			).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputFirstEveryEventsExpr(esper.VariableRef[int]("myvar_local"))),
		), nil
	}
	return esper.Query{}, fmt.Errorf("%s has no case %q", resultsetOutputLimitRowPerGroupFirstID, caseName)
}

func resultsetOutputLimitRowPerGroupFirstRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetOutputLimitRowPerGroupFirstWhenThenCase:
		return resultsetOutputLimitRowPerGroupFirstJavaRuntimeIDs[0]
	case resultsetOutputLimitRowPerGroupFirstHavingCase:
		return resultsetOutputLimitRowPerGroupFirstJavaRuntimeIDs[1]
	case resultsetOutputLimitRowPerGroupFirstCrontabCase:
		return resultsetOutputLimitRowPerGroupFirstJavaRuntimeIDs[2]
	case resultsetOutputLimitRowPerGroupFirstEveryNCase:
		return resultsetOutputLimitRowPerGroupFirstJavaRuntimeIDs[3]
	}
	return "parity-" + resultsetOutputLimitRowPerGroupFirstID + "-" + caseName
}

func decodeResultSetOutputLimitRowPerGroupFirstPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value rowPerGroupFirstBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportMarketDataBean":
		var value rowPerGroupFirstMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	case "SupportBean_A":
		var value rowPerGroupFirstBeanA
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

func decodeResultSetOutputLimitRowPerGroupFirstVariable(step compat.Step) (any, error) {
	switch step.Name {
	case "varoutone":
		var value bool
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode varoutone: %w", err)
		}
		return value, nil
	case "myvar_local":
		var value int
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode myvar_local: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported variable %q", step.Name)
	}
}

// normalizeResultSetOutputLimitRowPerGroupFirstTrace blanks the record time for
// the cases that never advance engine time: Java records wall-clock time there
// (non-deterministic, not part of the asserted contract), while Go records the
// engine start time. The first-crontab case keeps its virtual times because
// they are deterministic and observable.
func normalizeResultSetOutputLimitRowPerGroupFirstTrace(trace compat.Trace) compat.Trace {
	for index := range trace.Records {
		record := &trace.Records[index]
		if record.Case == "first-crontab" {
			continue
		}
		record.Time = ""
	}
	return trace
}
