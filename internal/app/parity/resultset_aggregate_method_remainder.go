package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetAggregateMethodRemainderJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateMethodRemainderJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateRate.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateLeaving.java",
}

var resultsetAggregateMethodRemainderJavaRuntimeIDs = []string{
	"java-runtime-d0424628aa4aad2d80bc", // ResultSetAggregateRateDataNonWindowed (ResultSetAggregateRate ord 0)
	"java-runtime-b90c3df2e444e89e8cb6", // ResultSetAggregateRateDataWindowed (ResultSetAggregateRate ord 1)
	"java-runtime-276a60a1f8e8298c32ec", // ResultSetAggregateLeaving (ord 0)
}

var resultsetAggregateMethodRemainderJavaExecutions = []string{
	"ResultSetAggregateRateDataNonWindowed",
	"ResultSetAggregateRateDataWindowed",
	"ResultSetAggregateLeaving",
}

const (
	resultsetAggregateMethodRemainderID = "resultset-aggregate-method-remainder"

	resultsetAggregateMethodRemainderRateEverCase     = "rate-ever"
	resultsetAggregateMethodRemainderRateWindowedCase = "rate-windowed"
	resultsetAggregateMethodRemainderLeavingCase      = "leaving"
)

var resultsetAggregateMethodRemainderCaseOrder = []string{
	resultsetAggregateMethodRemainderRateEverCase,
	resultsetAggregateMethodRemainderRateWindowedCase,
	resultsetAggregateMethodRemainderLeavingCase,
}

// aggregateMethodRemainderBean is the SupportBean carrier across all three
// cases: rate-ever sends the default bean, rate-windowed reads timestamps and
// quantities from longPrimitive/intPrimitive, and leaving sends
// SupportBean("E", n) pairs.
type aggregateMethodRemainderBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// runResultSetAggregateMethodRemainderScenario replays the three executions
// of the frozen Draft 4.441 unit: ResultSetAggregateRate ordinal 0
// (ResultSetAggregateRateDataNonWindowed, constant rate(10) over virtual-time
// ever-points), ResultSetAggregateRate ordinal 1
// (ResultSetAggregateRateDataWindowed, timestamp-property
// RATE(longPrimitive)/RATE(longPrimitive, intPrimitive) over #length(3)) and
// ResultSetAggregateLeaving ordinal 0 (sticky leaving() over #length(3)).
// Each Java execution is one scenario case inside its own runtime.
// ResultSetAggregateRateDataNonWindowed and ResultSetAggregateLeaving each
// run their assertion twice (compileDeploy plus eplToModelCompileDeploy); the
// scenario replays the assertion once because the second pass only changes
// the compile path, not listener output.
//
// The rate-ever case is virtual-time driven: the Java execution calls
// env.advanceTime (an absolute setTime on the external timer) before each
// send, so the scenario carries an advance-time step before every send and
// the runner applies it through engine.AdvanceTime on the virtual clock.
// The rate-windowed case takes its timestamps from the longPrimitive event
// property and never advances the clock; the leaving case sends
// SupportBean("E", n) events and records the sticky boolean.
//
// All projected columns are scalars (float64 rate values, bool leaving
// flag), so the shared normalizer needs no host-specific projection: a null
// column emits {"state":"null"} on both sides.
func runResultSetAggregateMethodRemainderScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetAggregateMethodRemainderCaseOrder))
	for _, caseName := range resultsetAggregateMethodRemainderCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetAggregateMethodRemainderCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetAggregateMethodRemainderID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetAggregateMethodRemainderID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateMethodRemainderCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultsetAggregateMethodRemainderID}
	var sequence uint64

	// The engine is created up front (not lazily at deploy) because the
	// rate-ever case advances the virtual clock before its deploy step,
	// mirroring the Java execution's sendTimer(0) before compileDeploy.
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[aggregateMethodRemainderBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateMethodRemainderRuntimeURI(caseName)),
	)
	deployments := map[string]*esper.Deployment{}

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

	deploy := func(statement string) error {
		if statement != "s0" {
			return fmt.Errorf("unexpected deploy statement %q", statement)
		}
		query, err := resultsetAggregateMethodRemainderQuery(env, caseName)
		if err != nil {
			return err
		}
		plan, err := env.Build(query)
		if err != nil {
			return fmt.Errorf("build %q: %w", statement, err)
		}
		deployment, err := engine.DeployPlans(ctx, []esper.Plan{plan})
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
			if err := deploy(step.Statement); err != nil {
				return compat.Trace{}, err
			}
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
				return compat.Trace{}, fmt.Errorf("advance-time %q: %w", step.At, err)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return compat.Trace{}, err
			}
		case "send":
			payload, err := decodeResultSetAggregateMethodRemainderPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", resultsetAggregateMethodRemainderID, step.Op)
		}
	}
	_ = engine.Close(context.Background())
	return trace, nil
}

// resultsetAggregateMethodRemainderQuery builds the s0 select for one case.
func resultsetAggregateMethodRemainderQuery(env *esper.Environment, caseName string) (esper.Query, error) {
	switch caseName {
	case resultsetAggregateMethodRemainderRateEverCase:
		return resultsetAggregateMethodRemainderRateEverQuery(env), nil
	case resultsetAggregateMethodRemainderRateWindowedCase:
		return resultsetAggregateMethodRemainderRateWindowedQuery(env), nil
	case resultsetAggregateMethodRemainderLeavingCase:
		return resultsetAggregateMethodRemainderLeavingQuery(env), nil
	}
	return esper.Query{}, fmt.Errorf("%s has no case %q", resultsetAggregateMethodRemainderID, caseName)
}

// resultsetAggregateMethodRemainderRateEverQuery mirrors
// ResultSetAggregateRateDataNonWindowed: rate(10) over the unwindowed
// SupportBean stream.  The constant form keeps ever-points on the virtual
// clock; the fluent form expresses the 10-second interval as a duration.
func resultsetAggregateMethodRemainderRateEverQuery(env *esper.Environment) esper.Query {
	return esper.From[aggregateMethodRemainderBean](env, "SupportBean").
		Aggregate(
			esper.Alias("myrate", esper.Rate(10*time.Second)),
		).Query(esper.StatementName("s0"))
}

// resultsetAggregateMethodRemainderRateWindowedQuery mirrors
// ResultSetAggregateRateDataWindowed: RATE(longPrimitive) and
// RATE(longPrimitive, intPrimitive) over SupportBean#length(3).  The
// timestamp-property notation takes event timestamps from longPrimitive and
// the quantity from intPrimitive; the rate becomes available once the first
// event leaves the window.
func resultsetAggregateMethodRemainderRateWindowedQuery(env *esper.Environment) esper.Query {
	longPrimitive := esper.Field[aggregateMethodRemainderBean, int64]("longPrimitive")
	intPrimitive := esper.Field[aggregateMethodRemainderBean, int]("intPrimitive")
	return esper.From[aggregateMethodRemainderBean](env, "SupportBean").
		Window(esper.LengthWindow(3)).
		Aggregate(
			esper.Alias("myrate", esper.RateByTimestamp[int64](longPrimitive)),
			esper.Alias("myqtyrate", esper.RateQuantityByTimestamp[int64, int](longPrimitive, intPrimitive)),
		).Query(esper.StatementName("s0"))
}

// resultsetAggregateMethodRemainderLeavingQuery mirrors
// ResultSetAggregateLeaving: leaving() over SupportBean#length(3), the sticky
// flag that turns true once any event leaves the window.
func resultsetAggregateMethodRemainderLeavingQuery(env *esper.Environment) esper.Query {
	return esper.From[aggregateMethodRemainderBean](env, "SupportBean").
		Window(esper.LengthWindow(3)).
		Aggregate(
			esper.Alias("val", esper.Leaving()),
		).Query(esper.StatementName("s0"))
}

func resultsetAggregateMethodRemainderRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetAggregateMethodRemainderRateEverCase:
		return resultsetAggregateMethodRemainderJavaRuntimeIDs[0]
	case resultsetAggregateMethodRemainderRateWindowedCase:
		return resultsetAggregateMethodRemainderJavaRuntimeIDs[1]
	case resultsetAggregateMethodRemainderLeavingCase:
		return resultsetAggregateMethodRemainderJavaRuntimeIDs[2]
	}
	return "parity-" + resultsetAggregateMethodRemainderID + "-" + caseName
}

func decodeResultSetAggregateMethodRemainderPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value aggregateMethodRemainderBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
