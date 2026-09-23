package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextStartEndTrioBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextStartEndTrioS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type contextStartEndTrioS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

const contextStartEndTrioID = "context-start-end-trio"

const contextStartEndTrioJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextStartEndTrioJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermTemporalFixed.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var contextStartEndTrioJavaRuntimeIDs = []string{
	"java-runtime-0319b4a966a4103e44ef", // ContextStartEndContextPartitionSelection
	"java-runtime-2bc9a5098743149a306e", // ContextStartEndPrevPriorAndAggregation
	"java-runtime-10686594f4fde577bfed", // ContextInitTermScheduleFilterResources
}

var contextStartEndTrioJavaExecutions = []string{
	"ContextStartEndContextPartitionSelection",
	"ContextStartEndPrevPriorAndAggregation",
	"ContextInitTermScheduleFilterResources",
}

// runContextStartEndTrioScenario replays three initiated/terminated-context
// executions: a non-overlapping start/end context with grouped keepall
// iterator and partition selectors (ContextStartEndContextPartitionSelection),
// a NineToFive crontab context projecting prev/prevwindow/prevtail/prior/sum
// with a full state reset across days (ContextStartEndPrevPriorAndAggregation),
// and a resource-lifecycle probe pinning schedule-count-overall across a plain
// time view and an initiated-by/terminated-after context
// (ContextInitTermScheduleFilterResources).
func runContextStartEndTrioScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextStartEndTrioCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-start-end-trio case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextStartEndTrioCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	// Cases 1-2 deploy their context+statement up front (the oracle deploys
	// them through pinned deploy steps sharing one module path) and the
	// deferred engine close covers undeploy-all; the replay loop only
	// consumes the remaining ops.
	if caseName != "initterm-schedule-filter-resources" {
		kept := caseScenario.Steps[:0]
		for _, step := range caseScenario.Steps {
			if step.Op != "deploy" && step.Op != "undeploy-all" {
				kept = append(kept, step)
			}
		}
		caseScenario.Steps = kept
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextStartEndTrioBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextStartEndTrioS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextStartEndTrioS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextStartEndTrioBean](env, "SupportBean")
	theString := esper.Field[contextStartEndTrioBean, string]("theString")
	intPrimitive := esper.Field[contextStartEndTrioBean, int]("intPrimitive")

	switch caseName {
	case "startend-partition-selection":
		// @public @public create context MyCtx as start SupportBean_S0 s0
		// end SupportBean_S1(id=s0.id)
		isS0 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
		end := esper.And(
			esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1")),
			esper.Equal[int](
				esper.Field[contextStartEndTrioS1, int]("id"),
				esper.Property[int](esper.ContextInitiatingEvent(), "id")))
		if _, err := esper.CreateInitiatedTerminatedContext(env, "MyCtx", esper.Literal("global"), isS0, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Window(esper.KeepAll()).GroupBy(theString).Select(
			esper.Alias("c0", esper.ContextField[int]("id")),
			esper.Alias("c1", esper.Property[string](esper.ContextInitiatingEvent(), "p00")),
			esper.Alias("c2", theString),
			esper.Alias("c3", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("MyCtx"))
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err := deployParityStatementAt(ctx, env, plan, scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextStartEndTrioPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-start-end-trio statement %q", name)
			}
			return statement, nil
		})
	case "startend-prev-prior-agg":
		// @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)
		nine, _ := esper.NewTimeOfDay(9, 0, 0)
		five, _ := esper.NewTimeOfDay(17, 0, 0)
		if _, err := esper.CreateDailyTimeContext(env, "NineToFive", nine, five); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Window(esper.KeepAll()).Aggregate(
			esper.Alias("col1", esper.Prev[string](1, theString)),
			esper.Alias("col2", esper.PrevWindow[esper.Event](esper.EventValue[esper.Event]())),
			esper.Alias("col3", esper.PrevTail[string](0, theString)),
			esper.Alias("col4", esper.Prior[string](0, theString)),
			esper.Alias("col5", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("NineToFive"))
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		engine, statement, err := deployParityStatementAt(ctx, env, plan, scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		defer func() { _ = engine.Close(context.Background()) }()
		return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextStartEndTrioPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-start-end-trio statement %q", name)
			}
			return statement, nil
		})
	case "initterm-schedule-filter-resources":
		return runContextStartEndTrioScheduleCase(ctx, env, beanSource, caseScenario, caseName)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-start-end-trio case %q", caseName)
	}
}

// runContextStartEndTrioScheduleCase replays the schedule/filter resource
// lifecycle: phase A deploys a plain 30-second time view (schedule count 1,
// back to 0 after undeploy); phase B deploys an initiated-by/terminated-after
// context whose per-partition termination timer and per-partition S0 time
// view drive the schedule count 0 → 1 → 2 → 0.
func runContextStartEndTrioScheduleCase(ctx context.Context, env *esper.Environment, beanSource esper.Stream[contextStartEndTrioBean], caseScenario compat.Scenario, caseName string) (compat.Trace, error) {
	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployments := map[string]*esper.Deployment{}
	deployOrder := []string{}

	deploy := func(step compat.Step) error {
		var plan esper.Plan
		var err error
		switch step.Statement {
		case "s0":
			// @name('s0') select * from SupportBean#time(30)
			plan, err = env.Build(beanSource.Window(esper.TimeWindow(30 * time.Second)).Query(esper.StatementName("s0")))
		case "ctx":
			// @name('ctx') @public create context EverySupportBean as
			// initiated by SupportBean as sb terminated after 1 minutes
			isBean := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean"))
			end := esper.TimerInterval(beanSource, time.Minute)
			_, err = esper.CreateOverlappingPatternTerminatedContext(env, "EverySupportBean", esper.Literal("global"), isBean, end)
			if err != nil {
				return err
			}
			return nil
		case "stmt":
			// context EverySupportBean select * from SupportBean_S0#time(2 min) sb0
			s0Source := esper.From[contextStartEndTrioS0](env, "SupportBean_S0")
			plan, err = env.Build(s0Source.Window(esper.TimeWindow(2 * time.Minute)).Query(esper.WithContext("EverySupportBean")))
		default:
			return fmt.Errorf("%s: unknown deploy label %q", contextStartEndTrioID, step.Statement)
		}
		if err != nil {
			return err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		deployments[step.Statement] = deployment
		deployOrder = append(deployOrder, step.Statement)
		return nil
	}

	for _, step := range caseScenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			// scenarioForCase keeps the case marker; nothing to do.
		case "deploy":
			if err := deploy(step); err != nil {
				return trace, err
			}
		case "send":
			event, err := decodeContextStartEndTrioPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return trace, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return trace, err
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return trace, err
			}
		case "schedule-count-overall":
			n, err := engine.ScheduleCountOverall(ctx)
			if err != nil {
				return trace, err
			}
			count := int64(n)
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: step.Op,
				Count:     &count,
			})
		case "undeploy":
			deployment, ok := deployments[step.Statement]
			if !ok {
				return trace, fmt.Errorf("%s: undeploy unknown label %q", contextStartEndTrioID, step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return trace, err
			}
			delete(deployments, step.Statement)
		case "undeploy-all":
			for index := len(deployOrder) - 1; index >= 0; index-- {
				label := deployOrder[index]
				deployment, ok := deployments[label]
				if !ok {
					continue
				}
				if err := deployment.Undeploy(ctx); err != nil {
					return trace, err
				}
				delete(deployments, label)
			}
			deployOrder = nil
		default:
			return trace, fmt.Errorf("%s: unsupported step op %q", contextStartEndTrioID, step.Op)
		}
	}
	return trace, nil
}

func decodeContextStartEndTrioPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextStartEndTrioBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextStartEndTrioS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextStartEndTrioS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-start-end-trio event type %q", step.EventType)
	}
}
