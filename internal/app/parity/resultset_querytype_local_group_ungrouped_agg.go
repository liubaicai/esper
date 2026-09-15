package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultSetQueryTypeLocalGroupUngroupedAggID          = "resultset-querytype-local-group-ungrouped-agg"
	resultSetQueryTypeLocalGroupUngroupedAggJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeLocalGroupUngroupedAggSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
	resultSetQueryTypeLocalGroupUngroupedAggDescription = "ResultSetQueryTypeLocalGroupBy ordinals 0/1/2/7: ungrouped local-group sums, SQL-standard aggregate selections, event-valued local aggregates and local-group HAVING."

	resultSetQueryTypeLocalGroupUngroupedAggSumRuntimeID    = "java-runtime-a40ad8ec1c03959b5f33"
	resultSetQueryTypeLocalGroupUngroupedAggSQLRuntimeID    = "java-runtime-6bec44d03e954b1cd52c"
	resultSetQueryTypeLocalGroupUngroupedAggEventRuntimeID  = "java-runtime-9ad9539a7e8f5581b192"
	resultSetQueryTypeLocalGroupUngroupedAggHavingRuntimeID = "java-runtime-1dc3599a7c07036be603"

	resultSetQueryTypeLocalGroupUngroupedAggSumStaticID    = "java-3feeb6f7d6769e71aad1"
	resultSetQueryTypeLocalGroupUngroupedAggSQLStaticID    = "java-d2a1aaee0aaf06dee915"
	resultSetQueryTypeLocalGroupUngroupedAggEventStaticID  = "java-79a9c055e0f90284c89e"
	resultSetQueryTypeLocalGroupUngroupedAggHavingStaticID = "java-ee7ae61064ad0a2279ed"

	// The pinned EPL texts are the Java source statements with whitespace runs
	// collapsed; the ordinal-1 and ordinal-2 texts keep the comma adjacency of
	// their Java concatenations.
	resultSetQueryTypeLocalGroupUngroupedAggSumEPL    = "@Name('s0') select sum(longPrimitive, group_by:(theString, intPrimitive)) as c0, sum(longPrimitive, group_by:(theString)) as c1, sum(longPrimitive, group_by:(intPrimitive)) as c2, sum(longPrimitive) as c3 from SupportBean"
	resultSetQueryTypeLocalGroupUngroupedAggSQLEPL    = "@name('s0') select intPrimitive as c0, sum(intPrimitive, group_by:()) as sum0, sum(intPrimitive, group_by:(theString)) as sum1,avedev(intPrimitive, group_by:(theString)) as avedev0,avg(intPrimitive, group_by:(theString)) as avg0,max(intPrimitive, group_by:(theString)) as max0,fmax(intPrimitive, intPrimitive>0, group_by:(theString)) as fmax0,min(intPrimitive, group_by:(theString)) as min0,fmin(intPrimitive, intPrimitive>0, group_by:(theString)) as fmin0,maxever(intPrimitive, group_by:(theString)) as maxever0,fmaxever(intPrimitive, intPrimitive>0, group_by:(theString)) as fmaxever0,minever(intPrimitive, group_by:(theString)) as minever0,fminever(intPrimitive, intPrimitive>0, group_by:(theString)) as fminever0,median(intPrimitive, group_by:(theString)) as median0,Math.round(coalesce(stddev(intPrimitive, group_by:(theString)), 0)) as stddev0 from SupportBean#keepall"
	resultSetQueryTypeLocalGroupUngroupedAggEventEPL  = "@name('s0') select intPrimitive as c0, first(sb, group_by:(theString)) as first0, first(sb, group_by:()) as first1, last(sb, group_by:(theString)) as last0, last(sb, group_by:()) as last1, window(sb, group_by:(theString)) as window0, window(sb, group_by:()) as window1, maxby(intPrimitive, group_by:(theString)) as maxby0, maxby(intPrimitive, group_by:()) as maxby1, minby(intPrimitive, group_by:(theString)) as minby0, minby(intPrimitive, group_by:()) as minby1, sorted(intPrimitive, group_by:(theString)) as sorted0, sorted(intPrimitive, group_by:()) as sorted1, maxbyever(intPrimitive, group_by:(theString)) as maxbyever0, maxbyever(intPrimitive, group_by:()) as maxbyever1, minbyever(intPrimitive, group_by:(theString)) as minbyever0, minbyever(intPrimitive, group_by:()) as minbyever1, firstever(sb, group_by:(theString)) as firstever0, firstever(sb, group_by:()) as firstever1, lastever(sb, group_by:(theString)) as lastever0, lastever(sb, group_by:()) as lastever1 from SupportBean#length(3) as sb"
	resultSetQueryTypeLocalGroupUngroupedAggHavingEPL = "@name('s0') select * from SupportBean having sum(intPrimitive, group_by:theString) > 100"
)

type resultSetQueryTypeLocalGroupUngroupedAggCaseSpec struct {
	name              string
	ordinal           int
	runtimeID         string
	execution         string
	observation       string
	iteratorSnapshots int
	epl               string
}

var resultSetQueryTypeLocalGroupUngroupedAggCaseSpecs = []resultSetQueryTypeLocalGroupUngroupedAggCaseSpec{
	{
		name:              "ungrouped-sum-simple",
		ordinal:           0,
		runtimeID:         resultSetQueryTypeLocalGroupUngroupedAggSumRuntimeID,
		execution:         "ResultSetLocalUngroupedSumSimple",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupUngroupedAggSumEPL,
	},
	{
		name:              "ungrouped-agg-sql-standard",
		ordinal:           1,
		runtimeID:         resultSetQueryTypeLocalGroupUngroupedAggSQLRuntimeID,
		execution:         "ResultSetLocalUngroupedAggSQLStandard",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupUngroupedAggSQLEPL,
	},
	{
		name:              "ungrouped-agg-event",
		ordinal:           2,
		runtimeID:         resultSetQueryTypeLocalGroupUngroupedAggEventRuntimeID,
		execution:         "ResultSetLocalUngroupedAggEvent",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupUngroupedAggEventEPL,
	},
	{
		name:              "ungrouped-having",
		ordinal:           7,
		runtimeID:         resultSetQueryTypeLocalGroupUngroupedAggHavingRuntimeID,
		execution:         "ResultSetLocalUngroupedHaving",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupUngroupedAggHavingEPL,
	},
}

var (
	resultSetQueryTypeLocalGroupUngroupedAggJavaSources   = []string{resultSetQueryTypeLocalGroupUngroupedAggSource}
	resultSetQueryTypeLocalGroupUngroupedAggJavaStaticIDs = []string{
		resultSetQueryTypeLocalGroupUngroupedAggSumStaticID,
		resultSetQueryTypeLocalGroupUngroupedAggSQLStaticID,
		resultSetQueryTypeLocalGroupUngroupedAggEventStaticID,
		resultSetQueryTypeLocalGroupUngroupedAggHavingStaticID,
	}
)

func resultSetQueryTypeLocalGroupUngroupedAggJavaRuntimeIDs() []string {
	ids := make([]string, 0, len(resultSetQueryTypeLocalGroupUngroupedAggCaseSpecs))
	for _, spec := range resultSetQueryTypeLocalGroupUngroupedAggCaseSpecs {
		ids = append(ids, spec.runtimeID)
	}
	return ids
}

func resultSetQueryTypeLocalGroupUngroupedAggJavaExecutions() []string {
	names := make([]string, 0, len(resultSetQueryTypeLocalGroupUngroupedAggCaseSpecs))
	for _, spec := range resultSetQueryTypeLocalGroupUngroupedAggCaseSpecs {
		names = append(names, spec.execution)
	}
	return names
}

// resultSetQueryTypeLocalGroupUngroupedAggStepSpec pins every scenario step in
// order; the loader rejects any drift before the runtime is touched.
type resultSetQueryTypeLocalGroupUngroupedAggStepSpec struct {
	op        string
	caseName  string
	at        string
	eventType string
	payload   resultSetQueryTypeLocalGroupByPayload
}

var resultSetQueryTypeLocalGroupUngroupedAggStepSpecs = []resultSetQueryTypeLocalGroupUngroupedAggStepSpec{
	{op: "case", caseName: "ungrouped-sum-simple"},
	{op: "advance-time", at: "1970-01-01T00:00:00Z"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 1, LongPrimitive: 10}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 2, LongPrimitive: 11}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 2, LongPrimitive: 12}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 1, LongPrimitive: 13}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 1, LongPrimitive: 14}},
	{op: "case", caseName: "ungrouped-agg-sql-standard"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 0}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 20, LongPrimitive: 0}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 30, LongPrimitive: 0}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 40, LongPrimitive: 0}},
	{op: "case", caseName: "ungrouped-agg-event"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 0}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 20, LongPrimitive: 0}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 15, LongPrimitive: 0}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E3", IntPrimitive: 16, LongPrimitive: 0}},
	{op: "case", caseName: "ungrouped-having"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 95, LongPrimitive: 0}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 10, LongPrimitive: 0}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 0}},
}

func loadResultSetQueryTypeLocalGroupUngroupedAggScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultSetQueryTypeLocalGroupUngroupedAggID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultSetQueryTypeLocalGroupUngroupedAggID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupUngroupedAggID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupUngroupedAggID, err)
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
	if version != compat.ScenarioVersion || id != resultSetQueryTypeLocalGroupUngroupedAggID ||
		description != resultSetQueryTypeLocalGroupUngroupedAggDescription ||
		javaCommit != resultSetQueryTypeLocalGroupUngroupedAggJavaCommit ||
		javaSource != resultSetQueryTypeLocalGroupUngroupedAggSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultSetQueryTypeLocalGroupUngroupedAggID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], resultSetQueryTypeLocalGroupUngroupedAggJavaRuntimeIDs(), "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], resultSetQueryTypeLocalGroupUngroupedAggJavaExecutions(), "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], resultSetQueryTypeLocalGroupUngroupedAggJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultSetQueryTypeLocalGroupUngroupedAggCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", resultSetQueryTypeLocalGroupUngroupedAggID, len(resultSetQueryTypeLocalGroupUngroupedAggCaseSpecs))
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
		spec := resultSetQueryTypeLocalGroupUngroupedAggCaseSpecs[index]
		if err := json.Unmarshal(rawCase, &caseMeta); err != nil ||
			caseMeta.Case != spec.name || ordinal != spec.ordinal || caseMeta.Ordinal != ordinal ||
			caseMeta.RuntimeID != spec.runtimeID || caseMeta.ExecutionName != spec.execution ||
			caseMeta.Observation != spec.observation || iteratorSnapshots != spec.iteratorSnapshots ||
			caseMeta.IteratorSnapshots != iteratorSnapshots || caseMeta.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultSetQueryTypeLocalGroupUngroupedAggID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(resultSetQueryTypeLocalGroupUngroupedAggStepSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", resultSetQueryTypeLocalGroupUngroupedAggID, len(resultSetQueryTypeLocalGroupUngroupedAggStepSpecs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := resultSetQueryTypeLocalGroupUngroupedAggStepSpecs[index]
		var expected []string
		switch spec.op {
		case "case":
			expected = []string{"op", "case"}
		case "advance-time":
			expected = []string{"op", "at"}
		default:
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
	if err := validateResultSetQueryTypeLocalGroupUngroupedAggScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeLocalGroupUngroupedAggScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeLocalGroupUngroupedAggID ||
		len(scenario.Steps) != len(resultSetQueryTypeLocalGroupUngroupedAggStepSpecs) {
		return fmt.Errorf("%s scenario steps are not pinned", resultSetQueryTypeLocalGroupUngroupedAggID)
	}
	for index, spec := range resultSetQueryTypeLocalGroupUngroupedAggStepSpecs {
		step := scenario.Steps[index]
		if step.Op != spec.op {
			return fmt.Errorf("%s scenario step %d must be %q", resultSetQueryTypeLocalGroupUngroupedAggID, index, spec.op)
		}
		switch spec.op {
		case "case":
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", resultSetQueryTypeLocalGroupUngroupedAggID, index, spec.caseName)
			}
		case "advance-time":
			if step.At != spec.at || step.Case != "" {
				return fmt.Errorf("%s scenario step %d must advance time to %s", resultSetQueryTypeLocalGroupUngroupedAggID, index, spec.at)
			}
		default:
			if step.EventType != spec.eventType || step.Case != "" {
				return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", resultSetQueryTypeLocalGroupUngroupedAggID, index, spec.eventType)
			}
			actual, err := decodeResultSetQueryTypeLocalGroupByPayload(step)
			if err != nil {
				return fmt.Errorf("%s scenario step %d: %w", resultSetQueryTypeLocalGroupUngroupedAggID, index, err)
			}
			if actual != spec.payload {
				return fmt.Errorf("%s scenario step %d payload is not pinned", resultSetQueryTypeLocalGroupUngroupedAggID, index)
			}
		}
	}
	return nil
}

func runResultSetQueryTypeLocalGroupUngroupedAggScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeLocalGroupUngroupedAggScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultSetQueryTypeLocalGroupUngroupedAggID}
	for index, spec := range resultSetQueryTypeLocalGroupUngroupedAggCaseSpecs {
		caseScenario, err := scenarioForCase(scenario, spec.name)
		if err != nil {
			return compat.Trace{}, err
		}
		records, err := runResultSetQueryTypeLocalGroupUngroupedAggCase(ctx, spec, caseScenario, index)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultSetQueryTypeLocalGroupUngroupedAggID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	if err := validateResultSetQueryTypeLocalGroupUngroupedAggTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

// resultSetQueryTypeLocalGroupUngroupedAggRound mirrors Java's Math.round for
// the SQL-standard standard-deviation column; the Go facade exposes no
// equivalent scalar function.
func resultSetQueryTypeLocalGroupUngroupedAggRound(value float64) int64 {
	return int64(math.Round(value))
}

func runResultSetQueryTypeLocalGroupUngroupedAggQuery(env *esper.Environment, caseIndex int) (esper.Query, error) {
	longPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int64]("longPrimitive")
	theString := esper.Field[resultSetQueryTypeLocalGroupByBean, string]("theString")
	intPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int32]("intPrimitive")
	localSum := func(keys ...esper.Expr) esper.AggregateExpression[int64] {
		return esper.LocalGroupBy[int64](esper.Sum[int64](longPrimitive), keys...)
	}
	from := esper.From[resultSetQueryTypeLocalGroupByBean](env, "SupportBean")
	switch caseIndex {
	case 0:
		return from.Aggregate(
			esper.Alias("c0", localSum(theString, intPrimitive)),
			esper.Alias("c1", localSum(theString)),
			esper.Alias("c2", localSum(intPrimitive)),
			esper.Alias("c3", esper.Sum[int64](longPrimitive)),
		).Query(esper.StatementName("s0")), nil
	case 1:
		positive := esper.Greater[int32](intPrimitive, esper.Literal(int32(0)))
		local := func(aggregate esper.AggregateExpression[int32]) esper.AggregateExpression[int32] {
			return esper.LocalGroupBy[int32](aggregate, theString)
		}
		filtered := func(aggregate esper.AggregateExpression[int32]) esper.AggregateExpression[int32] {
			return esper.LocalGroupBy[int32](esper.FilterAggregate[int32](aggregate, positive), theString)
		}
		localDouble := func(aggregate esper.AggregateExpression[float64]) esper.AggregateExpression[float64] {
			return esper.LocalGroupBy[float64](aggregate, theString)
		}
		// Java: Math.round(coalesce(stddev(intPrimitive, group_by:(theString)), 0)) -
		// the sample standard deviation is null for a single-value group, so
		// the coalesce substitutes 0 before the rounding step.
		stdDev := esper.Func1[float64, int64]("Math.round", resultSetQueryTypeLocalGroupUngroupedAggRound,
			esper.Coalesce[float64](
				localDouble(esper.StdDev[int32](intPrimitive)),
				esper.Literal(float64(0)),
			))
		return from.Window(esper.KeepAll()).Aggregate(
			esper.Alias("c0", intPrimitive),
			esper.Alias("sum0", esper.LocalGroupBy[int32](esper.Sum[int32](intPrimitive))),
			esper.Alias("sum1", esper.LocalGroupBy[int32](esper.Sum[int32](intPrimitive), theString)),
			esper.Alias("avedev0", localDouble(esper.Avedev[int32](intPrimitive))),
			esper.Alias("avg0", localDouble(esper.Avg[int32](intPrimitive))),
			esper.Alias("max0", local(esper.Max[int32](intPrimitive))),
			esper.Alias("fmax0", filtered(esper.Max[int32](intPrimitive))),
			esper.Alias("min0", local(esper.Min[int32](intPrimitive))),
			esper.Alias("fmin0", filtered(esper.Min[int32](intPrimitive))),
			esper.Alias("maxever0", local(esper.MaxEver[int32](intPrimitive))),
			esper.Alias("fmaxever0", filtered(esper.MaxEver[int32](intPrimitive))),
			esper.Alias("minever0", local(esper.MinEver[int32](intPrimitive))),
			esper.Alias("fminever0", filtered(esper.MinEver[int32](intPrimitive))),
			esper.Alias("median0", localDouble(esper.Median[int32](intPrimitive))),
			esper.Alias("stddev0", stdDev),
		).Query(esper.StatementName("s0")), nil
	case 2:
		event := esper.EventValue[esper.Event]()
		localEvent := func(aggregate esper.AggregateExpression[esper.Event]) esper.AggregateExpression[esper.Event] {
			return esper.LocalGroupBy[esper.Event](aggregate, theString)
		}
		localEvents := func(aggregate esper.AggregateExpression[[]esper.Event]) esper.AggregateExpression[[]esper.Event] {
			return esper.LocalGroupBy[[]esper.Event](aggregate, theString)
		}
		return from.Window(esper.LengthWindow(3)).Aggregate(
			esper.Alias("c0", intPrimitive),
			esper.Alias("first0", localEvent(esper.FirstEventValue())),
			esper.Alias("first1", esper.LocalGroupBy[esper.Event](esper.FirstEventValue())),
			esper.Alias("last0", localEvent(esper.LastEventValue())),
			esper.Alias("last1", esper.LocalGroupBy[esper.Event](esper.LastEventValue())),
			esper.Alias("window0", localEvents(esper.WindowEvents())),
			esper.Alias("window1", esper.LocalGroupBy[[]esper.Event](esper.WindowEvents())),
			esper.Alias("maxby0", esper.LocalGroupBy[esper.Event](esper.MaxBy[esper.Event, int32](event, intPrimitive), theString)),
			esper.Alias("maxby1", esper.LocalGroupBy[esper.Event](esper.MaxBy[esper.Event, int32](event, intPrimitive))),
			esper.Alias("minby0", esper.LocalGroupBy[esper.Event](esper.MinBy[esper.Event, int32](event, intPrimitive), theString)),
			esper.Alias("minby1", esper.LocalGroupBy[esper.Event](esper.MinBy[esper.Event, int32](event, intPrimitive))),
			esper.Alias("sorted0", localEvents(esper.SortedEvents(esper.Ascending(intPrimitive)))),
			esper.Alias("sorted1", esper.LocalGroupBy[[]esper.Event](esper.SortedEvents(esper.Ascending(intPrimitive)))),
			esper.Alias("maxbyever0", esper.LocalGroupBy[esper.Event](esper.MaxByEver[esper.Event, int32](event, intPrimitive), theString)),
			esper.Alias("maxbyever1", esper.LocalGroupBy[esper.Event](esper.MaxByEver[esper.Event, int32](event, intPrimitive))),
			esper.Alias("minbyever0", esper.LocalGroupBy[esper.Event](esper.MinByEver[esper.Event, int32](event, intPrimitive), theString)),
			esper.Alias("minbyever1", esper.LocalGroupBy[esper.Event](esper.MinByEver[esper.Event, int32](event, intPrimitive))),
			esper.Alias("firstever0", esper.LocalGroupBy[esper.Event](esper.FirstEver[esper.Event](event), theString)),
			esper.Alias("firstever1", esper.LocalGroupBy[esper.Event](esper.FirstEver[esper.Event](event))),
			esper.Alias("lastever0", esper.LocalGroupBy[esper.Event](esper.LastEver[esper.Event](event), theString)),
			esper.Alias("lastever1", esper.LocalGroupBy[esper.Event](esper.LastEver[esper.Event](event))),
		).Query(esper.StatementName("s0")), nil
	case 3:
		// Java's `select *` flattens the whole SupportBean surface; the typed Go
		// surface spells every property explicitly (established precedent), and
		// the HAVING clause carries the local-group aggregate that decides
		// delivery.
		return from.Aggregate(
			esper.Alias("theString", theString),
			esper.Alias("boolPrimitive", esper.Field[resultSetQueryTypeLocalGroupByBean, bool]("boolPrimitive")),
			esper.Alias("intPrimitive", intPrimitive),
			esper.Alias("longPrimitive", longPrimitive),
			esper.Alias("charPrimitive", esper.Field[resultSetQueryTypeLocalGroupByBean, string]("charPrimitive")),
			esper.Alias("shortPrimitive", esper.Field[resultSetQueryTypeLocalGroupByBean, int16]("shortPrimitive")),
			esper.Alias("bytePrimitive", esper.Field[resultSetQueryTypeLocalGroupByBean, int8]("bytePrimitive")),
			esper.Alias("floatPrimitive", esper.Field[resultSetQueryTypeLocalGroupByBean, float32]("floatPrimitive")),
			esper.Alias("doublePrimitive", esper.Field[resultSetQueryTypeLocalGroupByBean, float64]("doublePrimitive")),
			esper.Alias("boolBoxed", esper.Field[resultSetQueryTypeLocalGroupByBean, *bool]("boolBoxed")),
			esper.Alias("intBoxed", esper.Field[resultSetQueryTypeLocalGroupByBean, *int32]("intBoxed")),
			esper.Alias("longBoxed", esper.Field[resultSetQueryTypeLocalGroupByBean, *int64]("longBoxed")),
			esper.Alias("charBoxed", esper.Field[resultSetQueryTypeLocalGroupByBean, *string]("charBoxed")),
			esper.Alias("shortBoxed", esper.Field[resultSetQueryTypeLocalGroupByBean, *int16]("shortBoxed")),
			esper.Alias("byteBoxed", esper.Field[resultSetQueryTypeLocalGroupByBean, *int8]("byteBoxed")),
			esper.Alias("floatBoxed", esper.Field[resultSetQueryTypeLocalGroupByBean, *float32]("floatBoxed")),
			esper.Alias("doubleBoxed", esper.Field[resultSetQueryTypeLocalGroupByBean, *float64]("doubleBoxed")),
			esper.Alias("bigDecimal", esper.Field[resultSetQueryTypeLocalGroupByBean, *big.Rat]("bigDecimal")),
			esper.Alias("bigInteger", esper.Field[resultSetQueryTypeLocalGroupByBean, *big.Int]("bigInteger")),
			esper.Alias("enumValue", esper.Field[resultSetQueryTypeLocalGroupByBean, *string]("enumValue")),
		).Having(esper.Greater[int32](
			esper.LocalGroupBy[int32](esper.Sum[int32](intPrimitive), theString),
			esper.Literal(int32(100)),
		)).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("unknown %s case index %d", resultSetQueryTypeLocalGroupUngroupedAggID, caseIndex)
	}
}

func runResultSetQueryTypeLocalGroupUngroupedAggCase(ctx context.Context, spec resultSetQueryTypeLocalGroupUngroupedAggCaseSpec, caseScenario compat.Scenario, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupByBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	query, err := runResultSetQueryTypeLocalGroupUngroupedAggQuery(env, caseIndex)
	if err != nil {
		return nil, err
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

	records := make([]compat.TraceRecord, 0, 5)
	var sequence uint64
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       resultSetQueryTypeLocalGroupUngroupedRows(batch.New),
			Old:       resultSetQueryTypeLocalGroupUngroupedRows(batch.Old),
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
			payload, err := decodeResultSetQueryTypeLocalGroupByPayload(step)
			if err != nil {
				return nil, err
			}
			bean := resultSetQueryTypeLocalGroupByBean{
				TheString: payload.TheString, IntPrimitive: payload.IntPrimitive,
				LongPrimitive: payload.LongPrimitive, CharPrimitive: "\u0000",
			}
			if err := engine.Send(ctx, step.EventType, bean); err != nil {
				return nil, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, err
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported %s step op %q", resultSetQueryTypeLocalGroupUngroupedAggID, step.Op)
		}
	}
	return records, nil
}

func validateResultSetQueryTypeLocalGroupUngroupedAggTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultSetQueryTypeLocalGroupUngroupedAggID {
		return fmt.Errorf("%s trace identity is not pinned", resultSetQueryTypeLocalGroupUngroupedAggID)
	}
	if len(trace.Records) != 14 {
		return fmt.Errorf("%s trace must contain exactly fourteen records", resultSetQueryTypeLocalGroupUngroupedAggID)
	}
	for _, batch := range []struct {
		name  string
		start int
		count int
	}{
		{name: "ungrouped-sum-simple", start: 0, count: 5},
		{name: "ungrouped-agg-sql-standard", start: 5, count: 4},
		{name: "ungrouped-agg-event", start: 9, count: 4},
		{name: "ungrouped-having", start: 13, count: 1},
	} {
		for offset := 0; offset < batch.count; offset++ {
			record := trace.Records[batch.start+offset]
			if record.Case != batch.name || record.Operation != "listener" ||
				record.Statement != "s0" || record.Sequence != uint64(offset+1) || len(record.New) != 1 {
				return fmt.Errorf("%s %s record %d is not pinned", resultSetQueryTypeLocalGroupUngroupedAggID, batch.name, offset)
			}
		}
	}
	return nil
}
