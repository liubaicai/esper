package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultSetQueryTypeLocalGroupCtxTermID          = "resultset-querytype-local-group-context-terminated"
	resultSetQueryTypeLocalGroupCtxTermJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeLocalGroupCtxTermSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
	resultSetQueryTypeLocalGroupCtxTermDescription = "ResultSetQueryTypeLocalGroupBy ordinals 17/23: context-terminated snapshots over fully aggregated, ungrouped, grouped and locally grouped variants plus aggregate order-by."

	resultSetQueryTypeLocalGroupCtxTermFullyAggRuntimeID = "java-runtime-ee681560ebeda52abbb1"
	resultSetQueryTypeLocalGroupCtxTermOrderByRuntimeID  = "java-runtime-377240544dc6ec554ab2"

	resultSetQueryTypeLocalGroupCtxTermFullyAggStaticID = "java-d7190dd845b29e121075"
	resultSetQueryTypeLocalGroupCtxTermOrderByStaticID  = "java-ae42d2957d2e50829f37"

	// The pinned EPL texts are the Java source statements verbatim: the context
	// declaration and the query are concatenated exactly as the Java
	// executions concatenate them, including the double spaces left by the
	// Java line continuations.
	resultSetQueryTypeLocalGroupCtxTermFullyAggUngroupedEPL = "@public create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;@name('s0') context StartS0EndS1 select sum(group_by:(),intPrimitive) as c0 from SupportBean output snapshot when terminated;"
	resultSetQueryTypeLocalGroupCtxTermAggUngroupedEPL      = "@public create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;@name('s0') context StartS0EndS1 select sum(group_by:theString, intPrimitive) as c0 from SupportBean#keepall output snapshot when terminated;"
	resultSetQueryTypeLocalGroupCtxTermFullyAggGroupedEPL   = "@public create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;@name('s0') context StartS0EndS1 select sum(intPrimitive, group_by:()) as c0, sum(group_by:theString, intPrimitive) as c1, theString from SupportBean group by theString output snapshot when terminated;"
	resultSetQueryTypeLocalGroupCtxTermAggGroupedEPL        = "@public create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;@name('s0') context StartS0EndS1 select sum(longPrimitive, group_by:()) as c0, sum(longPrimitive, group_by:theString) as c1,  sum(longPrimitive, group_by:intPrimitive) as c2,  theString from SupportBean#keepall group by theString output snapshot when terminated;"
	resultSetQueryTypeLocalGroupCtxTermOrderByEPL           = "create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;@name('s0') context StartS0EndS1 select theString, sum(intPrimitive, group_by:theString) as c0  from SupportBean#keepall  output snapshot when terminated order by sum(intPrimitive, group_by:theString);"
)

type resultSetQueryTypeLocalGroupCtxTermCaseSpec struct {
	name              string
	ordinal           int
	runtimeID         string
	execution         string
	observation       string
	iteratorSnapshots int
	epl               string
}

var resultSetQueryTypeLocalGroupCtxTermCaseSpecs = []resultSetQueryTypeLocalGroupCtxTermCaseSpec{
	{
		name:              "fully-agg-ungrouped",
		ordinal:           17,
		runtimeID:         resultSetQueryTypeLocalGroupCtxTermFullyAggRuntimeID,
		execution:         "ResultSetAggregateFullyVersusNotFullyAgg",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupCtxTermFullyAggUngroupedEPL,
	},
	{
		name:              "agg-ungrouped",
		ordinal:           17,
		runtimeID:         resultSetQueryTypeLocalGroupCtxTermFullyAggRuntimeID,
		execution:         "ResultSetAggregateFullyVersusNotFullyAgg",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupCtxTermAggUngroupedEPL,
	},
	{
		name:              "fully-agg-grouped",
		ordinal:           17,
		runtimeID:         resultSetQueryTypeLocalGroupCtxTermFullyAggRuntimeID,
		execution:         "ResultSetAggregateFullyVersusNotFullyAgg",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupCtxTermFullyAggGroupedEPL,
	},
	{
		name:              "agg-grouped",
		ordinal:           17,
		runtimeID:         resultSetQueryTypeLocalGroupCtxTermFullyAggRuntimeID,
		execution:         "ResultSetAggregateFullyVersusNotFullyAgg",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupCtxTermAggGroupedEPL,
	},
	{
		name:              "ungrouped-order-by",
		ordinal:           23,
		runtimeID:         resultSetQueryTypeLocalGroupCtxTermOrderByRuntimeID,
		execution:         "ResultSetLocalUngroupedOrderBy",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupCtxTermOrderByEPL,
	},
}

// resultSetQueryTypeLocalGroupCtxTermRuntimes lists the distinct Java runtimes
// the scenario covers; the four ordinal-17 sub-cases share one runtime.
var resultSetQueryTypeLocalGroupCtxTermRuntimes = []string{
	resultSetQueryTypeLocalGroupCtxTermFullyAggRuntimeID,
	resultSetQueryTypeLocalGroupCtxTermOrderByRuntimeID,
}

var resultSetQueryTypeLocalGroupCtxTermExecutions = []string{
	"ResultSetAggregateFullyVersusNotFullyAgg",
	"ResultSetLocalUngroupedOrderBy",
}

var (
	resultSetQueryTypeLocalGroupCtxTermJavaSources   = []string{resultSetQueryTypeLocalGroupCtxTermSource}
	resultSetQueryTypeLocalGroupCtxTermJavaStaticIDs = []string{
		resultSetQueryTypeLocalGroupCtxTermFullyAggStaticID,
		resultSetQueryTypeLocalGroupCtxTermOrderByStaticID,
	}
)

func resultSetQueryTypeLocalGroupCtxTermJavaRuntimeIDs() []string {
	return append([]string(nil), resultSetQueryTypeLocalGroupCtxTermRuntimes...)
}

func resultSetQueryTypeLocalGroupCtxTermJavaExecutions() []string {
	return append([]string(nil), resultSetQueryTypeLocalGroupCtxTermExecutions...)
}

// resultSetQueryTypeLocalGroupCtxTermStepSpec pins every scenario step in
// order; the loader rejects any drift before the runtime is touched.
type resultSetQueryTypeLocalGroupCtxTermStepSpec struct {
	op       string
	caseName string
	kind     string
	bean     resultSetQueryTypeLocalGroupByPayload
	driverID int32
}

var resultSetQueryTypeLocalGroupCtxTermStepSpecs = func() []resultSetQueryTypeLocalGroupCtxTermStepSpec {
	shared := func() []resultSetQueryTypeLocalGroupCtxTermStepSpec {
		return []resultSetQueryTypeLocalGroupCtxTermStepSpec{
			{op: "send", kind: "SupportBean_S0", driverID: 0},
			{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100}},
			{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 20, LongPrimitive: 200}},
			{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 30, LongPrimitive: 300}},
			{op: "send", kind: "SupportBean_S1", driverID: 0},
			{op: "send", kind: "SupportBean_S0", driverID: 1},
			{op: "send", kind: "SupportBean_S1", driverID: 1},
		}
	}
	orderBy := []resultSetQueryTypeLocalGroupCtxTermStepSpec{
		{op: "send", kind: "SupportBean_S0", driverID: 0},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 20}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 30}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E3", IntPrimitive: 40}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 50}},
		{op: "send", kind: "SupportBean_S1", driverID: 0},
		{op: "send", kind: "SupportBean_S0", driverID: 1},
		{op: "send", kind: "SupportBean_S1", driverID: 1},
	}
	specs := make([]resultSetQueryTypeLocalGroupCtxTermStepSpec, 0, 42)
	for _, spec := range resultSetQueryTypeLocalGroupCtxTermCaseSpecs {
		specs = append(specs, resultSetQueryTypeLocalGroupCtxTermStepSpec{op: "case", caseName: spec.name})
		if spec.name == "ungrouped-order-by" {
			specs = append(specs, orderBy...)
			continue
		}
		specs = append(specs, shared()...)
	}
	return specs
}()

func loadResultSetQueryTypeLocalGroupCtxTermScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultSetQueryTypeLocalGroupCtxTermID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultSetQueryTypeLocalGroupCtxTermID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupCtxTermID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupCtxTermID, err)
	}
	if err := requireResultSetQueryTypeLocalGroupByFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}

	version, err := decodeResultSetQueryTypeLocalGroupByString(root["version"], "version")
	if err != nil {
		return compat.Scenario{}, err
	}
	id, err := decodeResultSetQueryTypeLocalGroupByString(root["id"], "id")
	if err != nil {
		return compat.Scenario{}, err
	}
	description, err := decodeResultSetQueryTypeLocalGroupByString(root["description"], "description")
	if err != nil {
		return compat.Scenario{}, err
	}
	javaCommit, err := decodeResultSetQueryTypeLocalGroupByString(root["javaCommit"], "javaCommit")
	if err != nil {
		return compat.Scenario{}, err
	}
	javaSource, err := decodeResultSetQueryTypeLocalGroupByString(root["javaSource"], "javaSource")
	if err != nil {
		return compat.Scenario{}, err
	}
	if version != compat.ScenarioVersion || id != resultSetQueryTypeLocalGroupCtxTermID ||
		description != resultSetQueryTypeLocalGroupCtxTermDescription ||
		javaCommit != resultSetQueryTypeLocalGroupCtxTermJavaCommit ||
		javaSource != resultSetQueryTypeLocalGroupCtxTermSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultSetQueryTypeLocalGroupCtxTermID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], resultSetQueryTypeLocalGroupCtxTermRuntimes, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], resultSetQueryTypeLocalGroupCtxTermExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], resultSetQueryTypeLocalGroupCtxTermJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultSetQueryTypeLocalGroupCtxTermCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", resultSetQueryTypeLocalGroupCtxTermID, len(resultSetQueryTypeLocalGroupCtxTermCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var caseObject map[string]json.RawMessage
		if err := strictObject(rawCase, &caseObject); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultSetQueryTypeLocalGroupByFields(caseObject,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		ordinal, err := decodeResultSetQueryTypeLocalGroupByInteger(caseObject["ordinal"], "ordinal")
		if err != nil {
			return compat.Scenario{}, err
		}
		iteratorSnapshots, err := decodeResultSetQueryTypeLocalGroupByInteger(caseObject["iteratorSnapshots"], "iteratorSnapshots")
		if err != nil {
			return compat.Scenario{}, err
		}
		var caseMeta struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		spec := resultSetQueryTypeLocalGroupCtxTermCaseSpecs[index]
		if err := json.Unmarshal(rawCase, &caseMeta); err != nil ||
			caseMeta.Case != spec.name || ordinal != spec.ordinal || caseMeta.Ordinal != ordinal ||
			caseMeta.RuntimeID != spec.runtimeID || caseMeta.ExecutionName != spec.execution ||
			caseMeta.Observation != spec.observation || iteratorSnapshots != spec.iteratorSnapshots ||
			caseMeta.IteratorSnapshots != iteratorSnapshots || caseMeta.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultSetQueryTypeLocalGroupCtxTermID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(resultSetQueryTypeLocalGroupCtxTermStepSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", resultSetQueryTypeLocalGroupCtxTermID, len(resultSetQueryTypeLocalGroupCtxTermStepSpecs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := resultSetQueryTypeLocalGroupCtxTermStepSpecs[index]
		var expected []string
		if spec.op == "case" {
			expected = []string{"op", "case"}
		} else {
			expected = []string{"op", "eventType", "payload"}
		}
		if err := requireResultSetQueryTypeLocalGroupByFields(object, expected...); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateResultSetQueryTypeLocalGroupCtxTermScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeLocalGroupCtxTermScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeLocalGroupCtxTermID ||
		len(scenario.Steps) != len(resultSetQueryTypeLocalGroupCtxTermStepSpecs) {
		return fmt.Errorf("%s scenario steps are not pinned", resultSetQueryTypeLocalGroupCtxTermID)
	}
	for index, spec := range resultSetQueryTypeLocalGroupCtxTermStepSpecs {
		step := scenario.Steps[index]
		if step.Op != spec.op {
			return fmt.Errorf("%s scenario step %d must be %q", resultSetQueryTypeLocalGroupCtxTermID, index, spec.op)
		}
		if spec.op == "case" {
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", resultSetQueryTypeLocalGroupCtxTermID, index, spec.caseName)
			}
			continue
		}
		if step.EventType != spec.kind || step.Case != "" {
			return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", resultSetQueryTypeLocalGroupCtxTermID, index, spec.kind)
		}
		event, err := decodeResultSetQueryTypeLocalGroupCtxTermPayload(step)
		if err != nil {
			return fmt.Errorf("%s scenario step %d: %w", resultSetQueryTypeLocalGroupCtxTermID, index, err)
		}
		switch typed := event.(type) {
		case resultSetQueryTypeLocalGroupByBean:
			got := resultSetQueryTypeLocalGroupByPayload{TheString: typed.TheString, IntPrimitive: typed.IntPrimitive, LongPrimitive: typed.LongPrimitive}
			if got != spec.bean {
				return fmt.Errorf("%s scenario step %d SupportBean payload is not pinned", resultSetQueryTypeLocalGroupCtxTermID, index)
			}
		case resultSetQueryTypeLocalGroupRowRemoveS0:
			if typed.ID != spec.driverID {
				return fmt.Errorf("%s scenario step %d SupportBean_S0 payload is not pinned", resultSetQueryTypeLocalGroupCtxTermID, index)
			}
		case resultSetQueryTypeLocalGroupRowRemoveS1:
			if typed.ID != spec.driverID {
				return fmt.Errorf("%s scenario step %d SupportBean_S1 payload is not pinned", resultSetQueryTypeLocalGroupCtxTermID, index)
			}
		default:
			return fmt.Errorf("%s scenario step %d has an unpinned payload", resultSetQueryTypeLocalGroupCtxTermID, index)
		}
	}
	return nil
}

func decodeResultSetQueryTypeLocalGroupCtxTermPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		payload, err := decodeResultSetQueryTypeLocalGroupByPayload(step)
		if err != nil {
			return nil, err
		}
		return resultSetQueryTypeLocalGroupByBean{
			TheString: payload.TheString, IntPrimitive: payload.IntPrimitive,
			LongPrimitive: payload.LongPrimitive, CharPrimitive: "\u0000",
		}, nil
	case "SupportBean_S0":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireResultSetQueryTypeLocalGroupByFields(fields, "id"); err != nil {
			return nil, fmt.Errorf("SupportBean_S0 payload: %w", err)
		}
		var payload resultSetQueryTypeLocalGroupRowRemoveS0
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("SupportBean_S0 payload must carry id")
		}
		return payload, nil
	case "SupportBean_S1":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireResultSetQueryTypeLocalGroupByFields(fields, "id"); err != nil {
			return nil, fmt.Errorf("SupportBean_S1 payload: %w", err)
		}
		var payload resultSetQueryTypeLocalGroupRowRemoveS1
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("SupportBean_S1 payload must carry id")
		}
		return payload, nil
	default:
		return nil, fmt.Errorf("unknown %s event type %q", resultSetQueryTypeLocalGroupCtxTermID, step.EventType)
	}
}

func runResultSetQueryTypeLocalGroupCtxTermScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeLocalGroupCtxTermScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultSetQueryTypeLocalGroupCtxTermID}
	for index, spec := range resultSetQueryTypeLocalGroupCtxTermCaseSpecs {
		caseScenario, err := scenarioForCase(scenario, spec.name)
		if err != nil {
			return compat.Trace{}, err
		}
		records, err := runResultSetQueryTypeLocalGroupCtxTermCase(ctx, spec, caseScenario, index)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultSetQueryTypeLocalGroupCtxTermID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	if err := validateResultSetQueryTypeLocalGroupCtxTermTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func runResultSetQueryTypeLocalGroupCtxTermCase(ctx context.Context, spec resultSetQueryTypeLocalGroupCtxTermCaseSpec, caseScenario compat.Scenario, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupByBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupRowRemoveS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupRowRemoveS1](env, "SupportBean_S1"); err != nil {
		return nil, err
	}
	isStart := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
	isEnd := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1"))
	if _, err := esper.CreateInitiatedTerminatedContext(env, "StartS0EndS1", esper.Literal("global"), isStart, isEnd); err != nil {
		return nil, err
	}

	theString := esper.Field[resultSetQueryTypeLocalGroupByBean, string]("theString")
	intPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int32]("intPrimitive")
	longPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int64]("longPrimitive")
	from := esper.From[resultSetQueryTypeLocalGroupByBean](env, "SupportBean")
	options := []esper.QueryOption{
		esper.StatementName("s0"),
		esper.WithContext("StartS0EndS1"),
		esper.WithOutput(esper.OutputSnapshotWhenTerminated()),
	}
	var query esper.Query
	switch caseIndex {
	case 0:
		query = from.Aggregate(
			esper.Alias("c0", esper.LocalGroupBy[int32](esper.Sum[int32](intPrimitive))),
		).Query(options...)
	case 1:
		query = from.Window(esper.KeepAll()).Aggregate(
			esper.Alias("c0", esper.LocalGroupBy[int32](esper.Sum[int32](intPrimitive), theString)),
		).Query(options...)
	case 2:
		query = from.GroupBy(theString).Select(
			esper.Alias("theString", theString),
			esper.Alias("c0", esper.LocalGroupBy[int32](esper.Sum[int32](intPrimitive))),
			esper.Alias("c1", esper.LocalGroupBy[int32](esper.Sum[int32](intPrimitive), theString)),
		).Query(options...)
	case 3:
		query = from.Window(esper.KeepAll()).GroupBy(theString).Select(
			esper.Alias("theString", theString),
			esper.Alias("c0", esper.LocalGroupBy[int64](esper.Sum[int64](longPrimitive))),
			esper.Alias("c1", esper.LocalGroupBy[int64](esper.Sum[int64](longPrimitive), theString)),
			esper.Alias("c2", esper.LocalGroupBy[int64](esper.Sum[int64](longPrimitive), intPrimitive)),
		).Query(options...)
	case 4:
		// Java's ORDER BY restates the local-group aggregate; the typed surface
		// orders by the projected column, which carries the same per-row value
		// (an aggregate expression bound directly would read the group's
		// current event instead of the row).
		query = from.Window(esper.KeepAll()).Aggregate(
			esper.Alias("theString", theString),
			esper.Alias("c0", esper.LocalGroupBy[int32](esper.Sum[int32](intPrimitive), theString)),
		).Query(append(options, esper.OrderBy(esper.Ascending(esper.ResultField[int32]("c0"))))...)
	default:
		return nil, fmt.Errorf("unknown %s case index %d", resultSetQueryTypeLocalGroupCtxTermID, caseIndex)
	}
	plan, err := env.Build(query)
	if err != nil {
		return nil, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, spec.runtimeID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	records := make([]compat.TraceRecord, 0, 2)
	var sequence uint64
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		newRows := resultSetQueryTypeLocalGroupUngroupedRows(batch.New)
		oldRows := resultSetQueryTypeLocalGroupUngroupedRows(batch.Old)
		if len(newRows) == 0 && len(oldRows) == 0 {
			return nil
		}
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       newRows,
			Old:       oldRows,
		})
		return nil
	}); err != nil {
		return nil, err
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeResultSetQueryTypeLocalGroupCtxTermPayload(step)
			if err != nil {
				return nil, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported %s step op %q", resultSetQueryTypeLocalGroupCtxTermID, step.Op)
		}
	}
	return records, nil
}

func validateResultSetQueryTypeLocalGroupCtxTermTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultSetQueryTypeLocalGroupCtxTermID {
		return fmt.Errorf("%s trace identity is not pinned", resultSetQueryTypeLocalGroupCtxTermID)
	}
	if len(trace.Records) == 0 {
		return fmt.Errorf("%s trace must contain at least one record", resultSetQueryTypeLocalGroupCtxTermID)
	}
	for index, record := range trace.Records {
		if record.Operation != "listener" || record.Statement != "s0" || len(record.Old) != 0 {
			return fmt.Errorf("%s trace record %d is not pinned", resultSetQueryTypeLocalGroupCtxTermID, index)
		}
	}
	return nil
}
