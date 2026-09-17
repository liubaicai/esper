package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const eplOtherPatternQueriesJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplOtherPatternQueriesJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherPatternQueries.java",
}

var eplOtherPatternQueriesJavaRuntimeIDs = []string{
	"java-runtime-00e2f9b1ff1f7568065e", // EPLOtherWhereOM (ord 0)
	"java-runtime-2d9518b6894d4c46972d", // EPLOtherWhereCompile (ord 1)
	"java-runtime-5b43884f1025d0953a3c", // EPLOtherWhere (ord 2)
	"java-runtime-ba57386eec69d7a3e1f4", // EPLOtherAggregation (ord 3)
}

var eplOtherPatternQueriesJavaExecutions = []string{
	"EPLOtherWhereOM",
	"EPLOtherWhereCompile",
	"EPLOtherWhere",
	"EPLOtherAggregation",
}

const (
	eplOtherPatternQueriesID = "epl-other-pattern-queries"

	eplOtherPatternQueriesWhereCase       = "pattern-where"
	eplOtherPatternQueriesAggregationCase = "pattern-aggregation"
)

var eplOtherPatternQueriesCaseOrder = []string{
	eplOtherPatternQueriesWhereCase,
	eplOtherPatternQueriesAggregationCase,
}

// eplOtherPatternQueriesS0 is the SupportBean_S0 carrier; only id is
// projected or filtered by the two executions.
type eplOtherPatternQueriesS0 struct {
	ID int `esper:"id"`
}

// eplOtherPatternQueriesS1 is the SupportBean_S1 carrier; only id is
// projected or filtered by the two executions.
type eplOtherPatternQueriesS1 struct {
	ID int `esper:"id"`
}

// runEplOtherPatternQueriesScenario replays the four executions of the
// frozen Draft 4.443 unit: EPLOtherPatternQueries ordinals 0/1/2
// (EPLOtherWhereOM, EPLOtherWhereCompile, EPLOtherWhere) and ordinal 3
// (EPLOtherAggregation).  Ordinals 0 and 1 are SODA-OM compile-path variants
// of ordinal 2's semantics — the object-model and eplToModel executions
// deploy the same every-or pattern with the same where predicate and assert
// the same listener output — so one scenario pass of ordinal 2's fluent
// equivalent covers all three.  Each Java execution is one scenario case
// inside its own runtime.
//
// Two shape notes mirror the Java oracle's trace.  The or-pattern binds
// exactly one tag per match, so the unbound tag's column is null:
// pattern-where emits {idS0,idS1} with one side null and
// pattern-aggregation's sum(s0.id + s1.id) stays null because the addition
// sees a null operand.  The where clause suppresses non-matching sends (S0
// id 101 and S1 id 1), so those sends emit no trace record.
func runEplOtherPatternQueriesScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(eplOtherPatternQueriesCaseOrder))
	for _, caseName := range eplOtherPatternQueriesCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runEplOtherPatternQueriesCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eplOtherPatternQueriesID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", eplOtherPatternQueriesID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runEplOtherPatternQueriesCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: eplOtherPatternQueriesID}
	var sequence uint64

	var env *esper.Environment
	var engine *esper.Engine
	deployments := map[string]*esper.Deployment{}
	deployIndex := 0 // each case deploys s0 exactly once

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

	freshEngine := func() error {
		if engine != nil {
			_ = engine.Close(context.Background())
		}
		env = esper.NewEnvironment()
		if _, err := esper.RegisterStruct[eplOtherPatternQueriesS0](env, "SupportBean_S0"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[eplOtherPatternQueriesS1](env, "SupportBean_S1"); err != nil {
			return err
		}
		engine = esper.NewEngine(env,
			esper.WithStartTime(time.Unix(0, 0).UTC()),
			esper.WithRuntimeURI(eplOtherPatternQueriesRuntimeURI(caseName)),
		)
		deployments = map[string]*esper.Deployment{}
		deployIndex = 0
		return nil
	}

	deploy := func(statement string) error {
		if statement != "s0" {
			return fmt.Errorf("unexpected deploy statement %q", statement)
		}
		phase := deployIndex
		deployIndex++
		query, err := eplOtherPatternQueriesQuery(env, caseName, phase)
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
			if engine == nil {
				if err := freshEngine(); err != nil {
					return compat.Trace{}, err
				}
			}
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
		case "send":
			payload, err := decodeEplOtherPatternQueriesPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", eplOtherPatternQueriesID, step.Op)
		}
	}
	if engine != nil {
		_ = engine.Close(context.Background())
	}
	return trace, nil
}

// eplOtherPatternQueriesQuery builds the s0 select for one case.  Each case
// deploys exactly once, so deployIndex must be 0.
func eplOtherPatternQueriesQuery(env *esper.Environment, caseName string, phase int) (esper.Query, error) {
	if phase != 0 {
		return esper.Query{}, fmt.Errorf("%s case %q has no deploy phase %d", eplOtherPatternQueriesID, caseName, phase)
	}
	switch caseName {
	case eplOtherPatternQueriesWhereCase:
		return eplOtherPatternQueriesWhereQuery(env), nil
	case eplOtherPatternQueriesAggregationCase:
		return eplOtherPatternQueriesAggregationQuery(env), nil
	}
	return esper.Query{}, fmt.Errorf("%s has no case %q", eplOtherPatternQueriesID, caseName)
}

// eplOtherPatternQueriesWhereQuery mirrors EPLOtherWhere (ordinal 2, also
// covering the ordinal 0/1 SODA-OM compile-path variants): an every-or
// pattern over SupportBean_S0/SupportBean_S1 whose matches are filtered by
// (s0.id is not null and s0.id < 100) or (s1.id is not null and s1.id >=
// 100).  The fluent form applies Every to each branch before Or so each
// side is its own repeating leg, and expresses the where clause through
// PatternQuery.Where over the match's tags.
func eplOtherPatternQueriesWhereQuery(env *esper.Environment) esper.Query {
	s0 := esper.From[eplOtherPatternQueriesS0](env, "SupportBean_S0")
	s1 := esper.From[eplOtherPatternQueriesS1](env, "SupportBean_S1")
	s0ID := esper.TagField[int]("s0", "id")
	s1ID := esper.TagField[int]("s1", "id")
	pattern := esper.PatternFrom(s0, "s0", esper.Literal(true)).Every().
		Or(esper.PatternFrom(s1, "s1", esper.Literal(true)).Every())
	return pattern.Select(
		esper.Alias("idS0", s0ID),
		esper.Alias("idS1", s1ID),
	).Where(
		esper.Or(
			esper.And(esper.Not(esper.IsNull[int](s0ID)), esper.Less[int](s0ID, esper.Literal(100))),
			esper.And(esper.Not(esper.IsNull[int](s1ID)), esper.GreaterOrEqual[int](s1ID, esper.Literal(100))),
		),
	).Query(esper.StatementName("s0"))
}

// eplOtherPatternQueriesAggregationQuery mirrors EPLOtherAggregation
// (ordinal 3): sum(s0.id), sum(s1.id) and sum(s0.id + s1.id) over the same
// every-or pattern.  Each match binds exactly one tag, so the cross-tag sum
// always sees a null operand and stays null.
func eplOtherPatternQueriesAggregationQuery(env *esper.Environment) esper.Query {
	s0 := esper.From[eplOtherPatternQueriesS0](env, "SupportBean_S0")
	s1 := esper.From[eplOtherPatternQueriesS1](env, "SupportBean_S1")
	s0ID := esper.TagField[int]("s0", "id")
	s1ID := esper.TagField[int]("s1", "id")
	pattern := esper.PatternFrom(s0, "s0", esper.Literal(true)).Every().
		Or(esper.PatternFrom(s1, "s1", esper.Literal(true)).Every())
	return pattern.Select(
		esper.Alias("sumS0", esper.Sum[int](s0ID)),
		esper.Alias("sumS1", esper.Sum[int](s1ID)),
		esper.Alias("sumS0S1", esper.Sum[int](esper.Add[int](s0ID, s1ID))),
	).Query(esper.StatementName("s0"))
}

func eplOtherPatternQueriesRuntimeURI(caseName string) string {
	switch caseName {
	case eplOtherPatternQueriesWhereCase:
		return eplOtherPatternQueriesJavaRuntimeIDs[2]
	case eplOtherPatternQueriesAggregationCase:
		return eplOtherPatternQueriesJavaRuntimeIDs[3]
	}
	return "parity-" + eplOtherPatternQueriesID + "-" + caseName
}

func decodeEplOtherPatternQueriesPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean_S0":
		var value eplOtherPatternQueriesS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value eplOtherPatternQueriesS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
