package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetRollupHavingOrderByJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetRollupHavingOrderByJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupHavingAndOrderBy.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupGroupingFuncs.java",
}

var resultsetRollupHavingOrderByJavaRuntimeIDs = []string{
	"java-runtime-23e2e441fc8898fe0303", // ResultSetQueryTypeHaving{join=false} (ord 0)
	"java-runtime-5c2e7accf8d815fe3ced", // ResultSetQueryTypeHaving{join=true} (ord 1)
	"java-runtime-7c4135247329f06e2e40", // ResultSetQueryTypeGroupingFuncExpressionUse (ord 3)
}

var resultsetRollupHavingOrderByJavaExecutions = []string{
	"ResultSetQueryTypeHaving{join=false}",
	"ResultSetQueryTypeHaving{join=true}",
	"ResultSetQueryTypeGroupingFuncExpressionUse",
}

const (
	resultsetRollupHavingOrderByID = "resultset-rollup-having-orderby"

	resultsetRollupHavingOrderByNoJoinCase   = "having-nojoin"
	resultsetRollupHavingOrderByJoinCase     = "having-join"
	resultsetRollupHavingOrderByGroupingCase = "grouping-func-expr"
)

var resultsetRollupHavingOrderByCaseOrder = []string{
	resultsetRollupHavingOrderByNoJoinCase,
	resultsetRollupHavingOrderByJoinCase,
	resultsetRollupHavingOrderByGroupingCase,
}

// rollupHavingOrderByBean is the SupportBean carrier (theString, intPrimitive,
// longPrimitive).
type rollupHavingOrderByBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// rollupHavingOrderByS0 is the SupportBean_S0 join partner (id only).
type rollupHavingOrderByS0 struct {
	ID int `esper:"id"`
}

// rollupHavingOrderByCarEvent is the SupportCarEvent carrier for the
// grouping-function case.
type rollupHavingOrderByCarEvent struct {
	Name  string `esper:"name"`
	Place string `esper:"place"`
	Count int    `esper:"count"`
}

// rollupHavingOrderByCarInfo is the SupportCarInfoEvent subquery source.
type rollupHavingOrderByCarInfo struct {
	Name  string `esper:"name"`
	Place string `esper:"place"`
	RefID string `esper:"refId"`
}

// runResultSetRollupHavingOrderByScenario replays the three executions of the
// frozen Draft 4.437 unit: ResultSetQueryTypeRollupHavingAndOrderBy ordinals
// 0/1 (ResultSetQueryTypeHaving join=false/join=true) and
// ResultSetQueryTypeRollupGroupingFuncs ordinal 3
// (ResultSetQueryTypeGroupingFuncExpressionUse).  Each Java execution is one
// scenario case running two sequential deploy/undeploy cycles inside one
// runtime.  The grouping-func-expr case observes phase A through "capture"
// records: Java asserts the void UDF's per-row argument tuples, so the Go
// select projects the same eight expressions as columns c0..c7 and the
// listener emits one capture record per delivered row.  (Go FuncN UDFs
// short-circuit on null arguments and can never observe the grouped-out key
// levels, so listener rows carry the captured tuple instead.)
func runResultSetRollupHavingOrderByScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetRollupHavingOrderByCaseOrder))
	for _, caseName := range resultsetRollupHavingOrderByCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetRollupHavingOrderByCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetRollupHavingOrderByID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetRollupHavingOrderByID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetRollupHavingOrderByCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultsetRollupHavingOrderByID}
	var sequence uint64

	var env *esper.Environment
	var engine *esper.Engine
	deployments := map[string]*esper.Deployment{}
	deployIndex := 0 // phase index: 0 phase A, 1 phase B

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
	// capture mirrors the Java UDF drain: each delivered phase-A row becomes
	// one capture record carrying the eight projected expressions (c0..c7),
	// the same tuple the void myfunc receives per grouping-set row.
	capture := func(batch esper.ResultBatch) {
		for _, row := range compat.NormalizeResults(batch.New) {
			sequence++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "capture",
				Statement: "s0",
				Sequence:  sequence,
				Time:      compat.FormatTraceTime(batch.Time),
				New:       []compat.ResultRecord{row},
			})
		}
	}

	freshEngine := func() error {
		if engine != nil {
			_ = engine.Close(context.Background())
		}
		env = esper.NewEnvironment()
		if _, err := esper.RegisterStruct[rollupHavingOrderByBean](env, "SupportBean"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[rollupHavingOrderByS0](env, "SupportBean_S0"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[rollupHavingOrderByCarEvent](env, "SupportCarEvent"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[rollupHavingOrderByCarInfo](env, "SupportCarInfoEvent"); err != nil {
			return err
		}
		engine = esper.NewEngine(env,
			esper.WithStartTime(time.Unix(0, 0).UTC()),
			esper.WithRuntimeURI(resultsetRollupHavingOrderByRuntimeURI(caseName)),
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
		query, err := resultsetRollupHavingOrderByQuery(env, caseName, phase)
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
				listener := record
				if caseName == resultsetRollupHavingOrderByGroupingCase && phase == 0 {
					listener = capture
				}
				if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					listener(batch)
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
			payload, err := decodeResultSetRollupHavingOrderByPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", resultsetRollupHavingOrderByID, step.Op)
		}
	}
	if engine != nil {
		_ = engine.Close(context.Background())
	}
	return trace, nil
}

// resultsetRollupHavingOrderByQuery builds the s0 select for one case and
// phase.  deployIndex 0 is phase A, 1 is phase B.
func resultsetRollupHavingOrderByQuery(env *esper.Environment, caseName string, phase int) (esper.Query, error) {
	switch caseName {
	case resultsetRollupHavingOrderByNoJoinCase, resultsetRollupHavingOrderByJoinCase:
		return resultsetRollupHavingOrderByHavingQuery(env, caseName == resultsetRollupHavingOrderByJoinCase, phase), nil
	case resultsetRollupHavingOrderByGroupingCase:
		return resultsetRollupHavingOrderByGroupingQuery(env, phase), nil
	}
	return esper.Query{}, fmt.Errorf("%s has no case %q", resultsetRollupHavingOrderByID, caseName)
}

// resultsetRollupHavingOrderByHavingQuery mirrors the two EPL phases of
// ResultSetQueryTypeHaving: phase A groups rollup(theString, intPrimitive)
// with having sum(longPrimitive) > 1000; phase B groups rollup(theString)
// with the null-key disjunction.  The join variant adds the cartesian
// SupportBean_S0#lastevent stream.
func resultsetRollupHavingOrderByHavingQuery(env *esper.Environment, join bool, phase int) esper.Query {
	bean := esper.From[rollupHavingOrderByBean](env, "SupportBean").Window(esper.KeepAll())
	if !join {
		theString := esper.Field[rollupHavingOrderByBean, string]("theString")
		intPrimitive := esper.Field[rollupHavingOrderByBean, int]("intPrimitive")
		if phase == 0 {
			sum := esper.Sum[int64](esper.Field[rollupHavingOrderByBean, int64]("longPrimitive"))
			return bean.GroupByRollup(theString, intPrimitive).
				Select(
					esper.Alias("c0", theString),
					esper.Alias("c1", intPrimitive),
					esper.Alias("c2", sum),
				).Having(esper.Greater[int64](sum, esper.Literal[int64](1000))).
				Query(esper.StatementName("s0"))
		}
		sum := esper.Sum[int](intPrimitive)
		return bean.GroupByRollup(theString).
			Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", sum),
			).Having(esper.Or(
			esper.And(esper.IsNull[string](theString), esper.Greater[int](sum, esper.Literal(100))),
			esper.And(esper.Not(esper.IsNull[string](theString)), esper.Greater[int](sum, esper.Literal(200))),
		)).Query(esper.StatementName("s0"))
	}
	s0 := esper.From[rollupHavingOrderByS0](env, "SupportBean_S0").Window(esper.LastEvent())
	joined := esper.Join(bean, s0)
	theString := esper.JoinField[string](0, "theString")
	intPrimitive := esper.JoinField[int](0, "intPrimitive")
	if phase == 0 {
		sum := esper.Sum[int64](esper.JoinField[int64](0, "longPrimitive"))
		return joined.Aggregate().Rollup(theString, intPrimitive).
			Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", sum),
			).Having(esper.Greater[int64](sum, esper.Literal[int64](1000))).
			Query(esper.StatementName("s0"))
	}
	sum := esper.Sum[int](intPrimitive)
	return joined.Aggregate().Rollup(theString).
		Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", sum),
		).Having(esper.Or(
		esper.And(esper.IsNull[string](theString), esper.Greater[int](sum, esper.Literal(100))),
		esper.And(esper.Not(esper.IsNull[string](theString)), esper.Greater[int](sum, esper.Literal(200))),
	)).Query(esper.StatementName("s0"))
}

// resultsetRollupHavingOrderByGroupingQuery mirrors
// ResultSetQueryTypeGroupingFuncExpressionUse.  Phase A projects the eight
// myfunc argument expressions as columns c0..c7 so the delivered rows carry
// the same tuples the Java UDF captures: the raw event name for the
// myExpr(ce) equivalent reads through the event (NestedField) because the
// declared expression sees the event property, not the rolled-up key.  Phase
// B is the prev/prior rollup select.
func resultsetRollupHavingOrderByGroupingQuery(env *esper.Environment, phase int) esper.Query {
	car := esper.From[rollupHavingOrderByCarEvent](env, "SupportCarEvent")
	name := esper.Field[rollupHavingOrderByCarEvent, string]("name")
	place := esper.Field[rollupHavingOrderByCarEvent, string]("place")
	count := esper.Field[rollupHavingOrderByCarEvent, int]("count")
	if phase == 0 {
		info := esper.Select(esper.From[rollupHavingOrderByCarInfo](env, "SupportCarInfoEvent")).Window(esper.LastEvent())
		return car.GroupByGroupingSets(
			esper.GroupingSet(name, place), esper.GroupingSet(name), esper.GroupingSet(place), esper.GroupingSet(),
		).Select(
			esper.Alias("c0", name),
			esper.Alias("c1", place),
			esper.Alias("c2", esper.Sum[int](count)),
			esper.Alias("c3", esper.Grouping(name)),
			esper.Alias("c4", esper.Grouping(place)),
			esper.Alias("c5", esper.GroupingID(name, place)),
			esper.Alias("c6", esper.SubqueryValue[string](info, esper.Field[rollupHavingOrderByCarInfo, string]("refId"))),
			esper.Alias("c7", esper.Concat(
				esper.Literal("|"),
				esper.NestedField[string](esper.EventValue[esper.Event](), "name"),
				esper.Literal("|"),
			)),
		).Query(esper.StatementName("s0"))
	}
	return car.Window(esper.KeepAll()).GroupByRollup(name).
		Select(
			esper.Alias("c0", esper.Prev[string](1, name)),
			// Go prior offsets are 0-based: Prior(0) is Java prior(1).
			esper.Alias("c1", esper.Prior[string](0, name)),
			esper.Alias("c2", name),
			esper.Alias("c3", esper.Sum[int](count)),
		).Query(esper.StatementName("s0"))
}

func resultsetRollupHavingOrderByRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetRollupHavingOrderByNoJoinCase:
		return resultsetRollupHavingOrderByJavaRuntimeIDs[0]
	case resultsetRollupHavingOrderByJoinCase:
		return resultsetRollupHavingOrderByJavaRuntimeIDs[1]
	case resultsetRollupHavingOrderByGroupingCase:
		return resultsetRollupHavingOrderByJavaRuntimeIDs[2]
	}
	return "parity-" + resultsetRollupHavingOrderByID + "-" + caseName
}

func decodeResultSetRollupHavingOrderByPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value rollupHavingOrderByBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value rollupHavingOrderByS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportCarEvent":
		var value rollupHavingOrderByCarEvent
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportCarEvent: %w", err)
		}
		return value, nil
	case "SupportCarInfoEvent":
		var value rollupHavingOrderByCarInfo
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportCarInfoEvent: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
