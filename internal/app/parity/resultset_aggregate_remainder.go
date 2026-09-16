package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultsetAggregateRemainderJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateRemainderJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFiltered.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateSortedMinMaxBy.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java",
}

var resultsetAggregateRemainderJavaRuntimeIDs = []string{
	"java-runtime-47dee40e6fe320005f49", // ResultSetAggregateFirstLastEver (ord 3)
	"java-runtime-dd3414775a8421e08a52", // ResultSetAggregateMultipleCriteria (ord 5)
	"java-runtime-c95393b13d253135c833", // ResultSetAggregateAuditAndReuse (ord 19)
}

var resultsetAggregateRemainderJavaExecutions = []string{
	"ResultSetAggregateFirstLastEver",
	"ResultSetAggregateMultipleCriteria",
	"ResultSetAggregateAuditAndReuse",
}

const (
	resultsetAggregateRemainderID = "resultset-aggregate-remainder"

	resultsetAggregateRemainderFirstLastEverCase    = "first-last-ever"
	resultsetAggregateRemainderMultipleCriteriaCase = "multiple-criteria"
	resultsetAggregateRemainderAuditReuseCase       = "audit-reuse"
)

var resultsetAggregateRemainderCaseOrder = []string{
	resultsetAggregateRemainderFirstLastEverCase,
	resultsetAggregateRemainderMultipleCriteriaCase,
	resultsetAggregateRemainderAuditReuseCase,
}

// aggregateRemainderBean is the SupportBean carrier across all three cases:
// theString/intPrimitive/longPrimitive serve the sorted and minby/maxby
// executions while intBoxed/boolPrimitive serve the ever-filtered execution.
type aggregateRemainderBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
	IntBoxed      *int64 `esper:"intBoxed"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
}

// runResultSetAggregateRemainderScenario replays the three executions of the
// frozen Draft 4.438 unit: ResultSetAggregateFiltered ordinal 3
// (ResultSetAggregateFirstLastEver), ResultSetAggregateSortedMinMaxBy ordinal 5
// (ResultSetAggregateMultipleCriteria) and
// ResultSetAggregateFilterNamedParameter ordinal 19
// (ResultSetAggregateAuditAndReuse).  Each Java execution is one scenario case
// inside its own runtime; multiple-criteria runs two sequential
// deploy/undeploy cycles (phase A sorted multi-criteria, phase B multi-key
// minby/maxby/minbyever/maxbyever) matching the single Java execution.
// ResultSetAggregateFirstLastEver runs its assertion twice under
// soda=true/false; the scenario replays it once because soda only changes
// compile-time serialization, not listener output.
//
// Two normalization rules mirror the Java oracle's trace shape.  Esper's
// sorted()/window(*) accessors return the stream's underlying beans, so the
// oracle projects each bean to {theString,intPrimitive}; the Go listener keeps
// the same two fields on every event-array element.  Esper access aggregates
// also return null when the retained state is empty (the audit-reuse
// window(*, filter:intPrimitive=1) columns see zero matching events), so the
// oracle emits [] for a null array-typed column and the Go listener emits []
// for a null slice-typed field.
func runResultSetAggregateRemainderScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetAggregateRemainderCaseOrder))
	for _, caseName := range resultsetAggregateRemainderCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetAggregateRemainderCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetAggregateRemainderID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetAggregateRemainderID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateRemainderCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultsetAggregateRemainderID}
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
			New:       normalizeAggregateRemainderResults(batch.New),
			Old:       normalizeAggregateRemainderResults(batch.Old),
		})
	}

	freshEngine := func() error {
		if engine != nil {
			_ = engine.Close(context.Background())
		}
		env = esper.NewEnvironment()
		if _, err := esper.RegisterStruct[aggregateRemainderBean](env, "SupportBean"); err != nil {
			return err
		}
		engine = esper.NewEngine(env,
			esper.WithStartTime(time.Unix(0, 0).UTC()),
			esper.WithRuntimeURI(resultsetAggregateRemainderRuntimeURI(caseName)),
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
		query, err := resultsetAggregateRemainderQuery(env, caseName, phase)
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
			payload, err := decodeResultSetAggregateRemainderPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", resultsetAggregateRemainderID, step.Op)
		}
	}
	if engine != nil {
		_ = engine.Close(context.Background())
	}
	return trace, nil
}

// resultsetAggregateRemainderQuery builds the s0 select for one case and
// phase.  deployIndex 0 is phase A, 1 is phase B.
func resultsetAggregateRemainderQuery(env *esper.Environment, caseName string, phase int) (esper.Query, error) {
	switch caseName {
	case resultsetAggregateRemainderFirstLastEverCase:
		return resultsetAggregateRemainderFirstLastEverQuery(env), nil
	case resultsetAggregateRemainderMultipleCriteriaCase:
		return resultsetAggregateRemainderMultipleCriteriaQuery(env, phase), nil
	case resultsetAggregateRemainderAuditReuseCase:
		return resultsetAggregateRemainderAuditReuseQuery(env), nil
	}
	return esper.Query{}, fmt.Errorf("%s has no case %q", resultsetAggregateRemainderID, caseName)
}

// resultsetAggregateRemainderFirstLastEverQuery mirrors
// ResultSetAggregateFirstLastEver: firstever/lastever/countever over
// SupportBean#length(3) with boolPrimitive as the positional filter.  The Go
// fluent form expresses the positional filter as FilterAggregate over the
// ever-aggregates.
func resultsetAggregateRemainderFirstLastEverQuery(env *esper.Environment) esper.Query {
	intBoxed := esper.Field[aggregateRemainderBean, *int64]("intBoxed")
	boolPrimitive := esper.Field[aggregateRemainderBean, bool]("boolPrimitive")
	return esper.From[aggregateRemainderBean](env, "SupportBean").
		Window(esper.LengthWindow(3)).
		Aggregate(
			esper.Alias("c1", esper.FilterAggregate[*int64](esper.FirstEver[*int64](intBoxed), boolPrimitive)),
			esper.Alias("c2", esper.FilterAggregate[*int64](esper.LastEver[*int64](intBoxed), boolPrimitive)),
			esper.Alias("c3", esper.FilterAggregate[int64](esper.CountEver(), boolPrimitive)),
		).Query(esper.StatementName("s0"))
}

// resultsetAggregateRemainderMultipleCriteriaQuery mirrors
// ResultSetAggregateMultipleCriteria.  Phase A projects four sorted(*)
// multi-criteria columns over #keepall; phase B projects the winning event's
// longPrimitive for the eight heterogeneous-key minby/maxby/minbyever/
// maxbyever combinations.
func resultsetAggregateRemainderMultipleCriteriaQuery(env *esper.Environment, phase int) esper.Query {
	theString := esper.Field[aggregateRemainderBean, string]("theString")
	intPrimitive := esper.Field[aggregateRemainderBean, int]("intPrimitive")
	stream := esper.From[aggregateRemainderBean](env, "SupportBean").Window(esper.KeepAll())
	if phase == 0 {
		return stream.Aggregate(
			esper.Alias("c0", esper.SortedEvents(esper.Descending(theString), esper.Descending(intPrimitive))),
			esper.Alias("c1", esper.SortedEvents(esper.Ascending(theString), esper.Ascending(intPrimitive))),
			esper.Alias("c2", esper.SortedEvents(esper.Ascending(theString), esper.Ascending(intPrimitive))),
			esper.Alias("c3", esper.SortedEvents(esper.Descending(theString), esper.Ascending(intPrimitive))),
		).Query(esper.StatementName("s0"))
	}
	event := esper.EventValue[esper.Event]()
	return stream.Aggregate(
		esper.Alias("c0", esper.Property[int64](esper.MaxByEverMulti[esper.Event](event, intPrimitive, theString), "longPrimitive")),
		esper.Alias("c1", esper.Property[int64](esper.MinByEverMulti[esper.Event](event, intPrimitive, theString), "longPrimitive")),
		esper.Alias("c2", esper.Property[int64](esper.MaxByEverMulti[esper.Event](event, theString, intPrimitive), "longPrimitive")),
		esper.Alias("c3", esper.Property[int64](esper.MinByEverMulti[esper.Event](event, theString, intPrimitive), "longPrimitive")),
		esper.Alias("c4", esper.Property[int64](esper.MaxByMulti[esper.Event](event, intPrimitive, theString), "longPrimitive")),
		esper.Alias("c5", esper.Property[int64](esper.MinByMulti[esper.Event](event, intPrimitive, theString), "longPrimitive")),
		esper.Alias("c6", esper.Property[int64](esper.MaxByMulti[esper.Event](event, theString, intPrimitive), "longPrimitive")),
		esper.Alias("c7", esper.Property[int64](esper.MinByMulti[esper.Event](event, theString, intPrimitive), "longPrimitive")),
	).Query(esper.StatementName("s0"))
}

// resultsetAggregateRemainderAuditReuseQuery mirrors
// ResultSetAggregateAuditAndReuse: two identical filtered sums and two
// identical filtered windows over SupportBean#length(3).  The shared
// expression instances mirror the EPL's literal reuse of the same filtered
// aggregate, which is the audit path under test.
func resultsetAggregateRemainderAuditReuseQuery(env *esper.Environment) esper.Query {
	intPrimitive := esper.Field[aggregateRemainderBean, int]("intPrimitive")
	filter := esper.Equal[int](intPrimitive, esper.Literal(1))
	sumFiltered := esper.FilterAggregate[int](esper.Sum[int](intPrimitive), filter)
	windowFiltered := esper.FilterAggregate[[]esper.Event](esper.WindowEvents(), filter)
	return esper.From[aggregateRemainderBean](env, "SupportBean").
		Window(esper.LengthWindow(3)).
		Aggregate(
			esper.Alias("c0", sumFiltered),
			esper.Alias("c1", sumFiltered),
			esper.Alias("c2", windowFiltered),
			esper.Alias("c3", windowFiltered),
		).Query(esper.StatementName("s0"))
}

// normalizeAggregateRemainderResults applies the two trace-shape rules the
// Java oracle applies: a null value on a slice-typed field (an empty
// sorted/window access-aggregate state) normalizes to [], and every element
// of an event-array field projects to {theString,intPrimitive} so the Go
// trace matches the oracle's underlying-bean projection element-wise.
func normalizeAggregateRemainderResults(results []esper.Result) []compat.ResultRecord {
	normalized := compat.NormalizeResults(results)
	if len(normalized) == 0 {
		return normalized
	}
	index := 0
	for _, result := range results {
		var fields []esper.FieldSpec
		if event, ok := result.Event(); ok {
			fields = event.Schema().Fields()
		} else if row, ok := result.Row(); ok {
			fields = row.Schema().Fields()
		} else {
			continue
		}
		record := normalized[index]
		index++
		for _, field := range fields {
			value, present := record.Fields[field.Name]
			if !present {
				continue
			}
			if isAggregateRemainderNull(value) && field.Type != nil && field.Type.Kind() == reflect.Slice {
				record.Fields[field.Name] = []any{}
				continue
			}
			record.Fields[field.Name] = projectAggregateRemainderEventRows(value)
		}
	}
	return normalized
}

func isAggregateRemainderNull(value any) bool {
	object, ok := value.(map[string]any)
	return ok && object["state"] == "null"
}

// projectAggregateRemainderEventRows rewrites each {"kind":"row","fields":…}
// element of an event-array value to the two sort-key fields the Java oracle
// emits for the underlying SupportBean elements.
func projectAggregateRemainderEventRows(value any) any {
	rows, ok := value.([]any)
	if !ok {
		return value
	}
	projected := make([]any, 0, len(rows))
	changed := false
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			projected = append(projected, item)
			continue
		}
		kind, _ := row["kind"].(string)
		fields, fieldsOK := row["fields"].(map[string]any)
		if kind != "row" || !fieldsOK {
			projected = append(projected, item)
			continue
		}
		projected = append(projected, map[string]any{
			"kind": "row",
			"fields": map[string]any{
				"theString":    fields["theString"],
				"intPrimitive": fields["intPrimitive"],
			},
		})
		changed = true
	}
	if !changed {
		return value
	}
	return projected
}

func resultsetAggregateRemainderRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetAggregateRemainderFirstLastEverCase:
		return resultsetAggregateRemainderJavaRuntimeIDs[0]
	case resultsetAggregateRemainderMultipleCriteriaCase:
		return resultsetAggregateRemainderJavaRuntimeIDs[1]
	case resultsetAggregateRemainderAuditReuseCase:
		return resultsetAggregateRemainderJavaRuntimeIDs[2]
	}
	return "parity-" + resultsetAggregateRemainderID + "-" + caseName
}

func decodeResultSetAggregateRemainderPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value aggregateRemainderBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
