package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultSetQueryTypeLocalGroupGroupedID          = "resultset-querytype-local-group-grouped"
	resultSetQueryTypeLocalGroupGroupedJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeLocalGroupGroupedSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
	resultSetQueryTypeLocalGroupGroupedDescription = "ResultSetQueryTypeLocalGroupBy ordinals 10/11/14: grouped local-group aggregates over a length window and under snapshot-every output."

	resultSetQueryTypeLocalGroupGroupedSimpleRuntimeID    = "java-runtime-49be8402f85fb067edc3"
	resultSetQueryTypeLocalGroupGroupedMethodRuntimeID    = "java-runtime-9a3385eedb2df5f1627c"
	resultSetQueryTypeLocalGroupGroupedNoDefaultRuntimeID = "java-runtime-6f77d02f924ecaa4c3ca"

	resultSetQueryTypeLocalGroupGroupedSimpleStaticID    = "java-c2618f1757daf7867b21"
	resultSetQueryTypeLocalGroupGroupedMethodStaticID    = "java-c4b89d0fa19bf787a80e"
	resultSetQueryTypeLocalGroupGroupedNoDefaultStaticID = "java-59de6fa8771761db231b"

	// The pinned EPL texts are the Java source statements with whitespace runs
	// collapsed; the ordinal-10 projection keeps the Java concatenation's
	// comma adjacency and its missing space before the group-by clause.
	resultSetQueryTypeLocalGroupGroupedSimpleEPL    = "@Name('s0') select sum(longPrimitive, group_by:theString) as c0,count(*, group_by:theString) as c1,window(*, group_by:theString) as c2,sum(longPrimitive, group_by:intPrimitive) as c3,count(*, group_by:intPrimitive) as c4,window(*, group_by:intPrimitive) as c5,sum(longPrimitive, group_by:()) as c6,count(*, group_by:()) as c7,window(*, group_by:()) as c8,sum(longPrimitive) as c9 from SupportBean#length(4)group by theString, intPrimitive"
	resultSetQueryTypeLocalGroupGroupedMethodEPL    = "@name('s0') select theString, intPrimitive, sum(longPrimitive, group_by:(intPrimitive, theString)) as c0, sum(longPrimitive) as c1, sum(longPrimitive, group_by:(theString)) as c2, sum(longPrimitive, group_by:(intPrimitive)) as c3, sum(longPrimitive, group_by:()) as c4 from SupportBean group by theString, intPrimitive output snapshot every 10 seconds"
	resultSetQueryTypeLocalGroupGroupedNoDefaultEPL = "@name('s0') select theString, intPrimitive, sum(longPrimitive, group_by:(theString)) as c0, sum(longPrimitive, group_by:(intPrimitive)) as c1, sum(longPrimitive, group_by:()) as c2 from SupportBean group by theString, intPrimitive output snapshot every 10 seconds"
)

type resultSetQueryTypeLocalGroupGroupedCaseSpec struct {
	name              string
	ordinal           int
	runtimeID         string
	execution         string
	observation       string
	iteratorSnapshots int
	epl               string
}

var resultSetQueryTypeLocalGroupGroupedCaseSpecs = []resultSetQueryTypeLocalGroupGroupedCaseSpec{
	{
		name:              "grouped-simple",
		ordinal:           10,
		runtimeID:         resultSetQueryTypeLocalGroupGroupedSimpleRuntimeID,
		execution:         "ResultSetLocalGroupedSimple",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupGroupedSimpleEPL,
	},
	{
		name:              "grouped-multi-level-method",
		ordinal:           11,
		runtimeID:         resultSetQueryTypeLocalGroupGroupedMethodRuntimeID,
		execution:         "ResultSetLocalGroupedMultiLevelMethod",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupGroupedMethodEPL,
	},
	{
		name:              "grouped-multi-level-no-default-lvl",
		ordinal:           14,
		runtimeID:         resultSetQueryTypeLocalGroupGroupedNoDefaultRuntimeID,
		execution:         "ResultSetLocalGroupedMultiLevelNoDefaultLvl",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupGroupedNoDefaultEPL,
	},
}

var (
	resultSetQueryTypeLocalGroupGroupedJavaSources   = []string{resultSetQueryTypeLocalGroupGroupedSource}
	resultSetQueryTypeLocalGroupGroupedJavaStaticIDs = []string{
		resultSetQueryTypeLocalGroupGroupedSimpleStaticID,
		resultSetQueryTypeLocalGroupGroupedMethodStaticID,
		resultSetQueryTypeLocalGroupGroupedNoDefaultStaticID,
	}
)

func resultSetQueryTypeLocalGroupGroupedJavaRuntimeIDs() []string {
	ids := make([]string, 0, len(resultSetQueryTypeLocalGroupGroupedCaseSpecs))
	for _, spec := range resultSetQueryTypeLocalGroupGroupedCaseSpecs {
		ids = append(ids, spec.runtimeID)
	}
	return ids
}

func resultSetQueryTypeLocalGroupGroupedJavaExecutions() []string {
	names := make([]string, 0, len(resultSetQueryTypeLocalGroupGroupedCaseSpecs))
	for _, spec := range resultSetQueryTypeLocalGroupGroupedCaseSpecs {
		names = append(names, spec.execution)
	}
	return names
}

// resultSetQueryTypeLocalGroupGroupedStepSpec pins every scenario step in
// order; the loader rejects any drift before the runtime is touched.
type resultSetQueryTypeLocalGroupGroupedStepSpec struct {
	op        string
	caseName  string
	at        string
	eventType string
	payload   resultSetQueryTypeLocalGroupByPayload
}

var resultSetQueryTypeLocalGroupGroupedStepSpecs = []resultSetQueryTypeLocalGroupGroupedStepSpec{
	{op: "case", caseName: "grouped-simple"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 10, LongPrimitive: 101}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 20, LongPrimitive: 102}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 103}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 104}},
	{op: "case", caseName: "grouped-multi-level-method"},
	{op: "advance-time", at: "1970-01-01T00:00:00Z"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 20, LongPrimitive: 202}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 10, LongPrimitive: 303}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 404}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 10, LongPrimitive: 505}},
	{op: "advance-time", at: "1970-01-01T00:00:10Z"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 1}},
	{op: "advance-time", at: "1970-01-01T00:00:20Z"},
	{op: "case", caseName: "grouped-multi-level-no-default-lvl"},
	{op: "advance-time", at: "1970-01-01T00:00:00Z"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 20, LongPrimitive: 202}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 10, LongPrimitive: 303}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 404}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 10, LongPrimitive: 505}},
	{op: "advance-time", at: "1970-01-01T00:00:10Z"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 1}},
	{op: "advance-time", at: "1970-01-01T00:00:20Z"},
}

func loadResultSetQueryTypeLocalGroupGroupedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultSetQueryTypeLocalGroupGroupedID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultSetQueryTypeLocalGroupGroupedID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupGroupedID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupGroupedID, err)
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
	if version != compat.ScenarioVersion || id != resultSetQueryTypeLocalGroupGroupedID ||
		description != resultSetQueryTypeLocalGroupGroupedDescription ||
		javaCommit != resultSetQueryTypeLocalGroupGroupedJavaCommit ||
		javaSource != resultSetQueryTypeLocalGroupGroupedSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultSetQueryTypeLocalGroupGroupedID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], resultSetQueryTypeLocalGroupGroupedJavaRuntimeIDs(), "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], resultSetQueryTypeLocalGroupGroupedJavaExecutions(), "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], resultSetQueryTypeLocalGroupGroupedJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultSetQueryTypeLocalGroupGroupedCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", resultSetQueryTypeLocalGroupGroupedID, len(resultSetQueryTypeLocalGroupGroupedCaseSpecs))
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
		spec := resultSetQueryTypeLocalGroupGroupedCaseSpecs[index]
		if err := json.Unmarshal(rawCase, &caseMeta); err != nil ||
			caseMeta.Case != spec.name || ordinal != spec.ordinal || caseMeta.Ordinal != ordinal ||
			caseMeta.RuntimeID != spec.runtimeID || caseMeta.ExecutionName != spec.execution ||
			caseMeta.Observation != spec.observation || iteratorSnapshots != spec.iteratorSnapshots ||
			caseMeta.IteratorSnapshots != iteratorSnapshots || caseMeta.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultSetQueryTypeLocalGroupGroupedID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(resultSetQueryTypeLocalGroupGroupedStepSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", resultSetQueryTypeLocalGroupGroupedID, len(resultSetQueryTypeLocalGroupGroupedStepSpecs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := resultSetQueryTypeLocalGroupGroupedStepSpecs[index]
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
	if err := validateResultSetQueryTypeLocalGroupGroupedScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeLocalGroupGroupedScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeLocalGroupGroupedID ||
		len(scenario.Steps) != len(resultSetQueryTypeLocalGroupGroupedStepSpecs) {
		return fmt.Errorf("%s scenario steps are not pinned", resultSetQueryTypeLocalGroupGroupedID)
	}
	for index, spec := range resultSetQueryTypeLocalGroupGroupedStepSpecs {
		step := scenario.Steps[index]
		if step.Op != spec.op {
			return fmt.Errorf("%s scenario step %d must be %q", resultSetQueryTypeLocalGroupGroupedID, index, spec.op)
		}
		switch spec.op {
		case "case":
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", resultSetQueryTypeLocalGroupGroupedID, index, spec.caseName)
			}
		case "advance-time":
			if step.At != spec.at || step.Case != "" {
				return fmt.Errorf("%s scenario step %d must advance time to %s", resultSetQueryTypeLocalGroupGroupedID, index, spec.at)
			}
		default:
			if step.EventType != spec.eventType || step.Case != "" {
				return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", resultSetQueryTypeLocalGroupGroupedID, index, spec.eventType)
			}
			actual, err := decodeResultSetQueryTypeLocalGroupByPayload(step)
			if err != nil {
				return fmt.Errorf("%s scenario step %d: %w", resultSetQueryTypeLocalGroupGroupedID, index, err)
			}
			if actual != spec.payload {
				return fmt.Errorf("%s scenario step %d payload is not pinned", resultSetQueryTypeLocalGroupGroupedID, index)
			}
		}
	}
	return nil
}

func runResultSetQueryTypeLocalGroupGroupedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeLocalGroupGroupedScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultSetQueryTypeLocalGroupGroupedID}
	for index, spec := range resultSetQueryTypeLocalGroupGroupedCaseSpecs {
		caseScenario, err := scenarioForCase(scenario, spec.name)
		if err != nil {
			return compat.Trace{}, err
		}
		records, err := runResultSetQueryTypeLocalGroupGroupedCase(ctx, spec, caseScenario, index)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultSetQueryTypeLocalGroupGroupedID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	if err := validateResultSetQueryTypeLocalGroupGroupedTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func runResultSetQueryTypeLocalGroupGroupedQuery(env *esper.Environment, caseIndex int) (esper.Query, error) {
	longPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int64]("longPrimitive")
	theString := esper.Field[resultSetQueryTypeLocalGroupByBean, string]("theString")
	intPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int32]("intPrimitive")
	sum := func(keys ...esper.Expr) esper.AggregateExpression[int64] {
		return esper.LocalGroupBy[int64](esper.Sum[int64](longPrimitive), keys...)
	}
	count := func(keys ...esper.Expr) esper.AggregateExpression[int64] {
		return esper.LocalGroupBy[int64](esper.CountAll(), keys...)
	}
	windows := func(keys ...esper.Expr) esper.AggregateExpression[[]esper.Event] {
		return esper.LocalGroupBy[[]esper.Event](esper.WindowEvents(), keys...)
	}
	from := esper.From[resultSetQueryTypeLocalGroupByBean](env, "SupportBean")
	switch caseIndex {
	case 0:
		return from.Window(esper.LengthWindow(4)).
			GroupBy(theString, intPrimitive).
			Select(
				esper.Alias("c0", sum(theString)),
				esper.Alias("c1", count(theString)),
				esper.Alias("c2", windows(theString)),
				esper.Alias("c3", sum(intPrimitive)),
				esper.Alias("c4", count(intPrimitive)),
				esper.Alias("c5", windows(intPrimitive)),
				esper.Alias("c6", sum()),
				esper.Alias("c7", count()),
				esper.Alias("c8", windows()),
				// Java's unqualified sum(longPrimitive) stays scoped to the
				// outer group while group_by:() spans the whole window.
				esper.Alias("c9", esper.Sum[int64](longPrimitive)),
			).Query(esper.StatementName("s0")), nil
	case 1:
		return from.GroupBy(theString, intPrimitive).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("intPrimitive", intPrimitive),
				esper.Alias("c0", sum(intPrimitive, theString)),
				esper.Alias("c1", esper.Sum[int64](longPrimitive)),
				esper.Alias("c2", sum(theString)),
				esper.Alias("c3", sum(intPrimitive)),
				esper.Alias("c4", sum()),
			).
			Query(esper.StatementName("s0"), esper.WithOutput(esper.OutputSnapshotEvery(10*time.Second))), nil
	case 2:
		return from.GroupBy(theString, intPrimitive).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("intPrimitive", intPrimitive),
				esper.Alias("c0", sum(theString)),
				esper.Alias("c1", sum(intPrimitive)),
				esper.Alias("c2", sum()),
			).
			Query(esper.StatementName("s0"), esper.WithOutput(esper.OutputSnapshotEvery(10*time.Second))), nil
	default:
		return esper.Query{}, fmt.Errorf("unknown %s case index %d", resultSetQueryTypeLocalGroupGroupedID, caseIndex)
	}
}

func runResultSetQueryTypeLocalGroupGroupedCase(ctx context.Context, spec resultSetQueryTypeLocalGroupGroupedCaseSpec, caseScenario compat.Scenario, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupByBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	query, err := runResultSetQueryTypeLocalGroupGroupedQuery(env, caseIndex)
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
			New:       resultSetQueryTypeLocalGroupGroupedCanonicalRows(newRows),
			Old:       resultSetQueryTypeLocalGroupGroupedCanonicalRows(oldRows),
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
			return nil, fmt.Errorf("unsupported %s step op %q", resultSetQueryTypeLocalGroupGroupedID, step.Op)
		}
	}
	return records, nil
}

// resultSetQueryTypeLocalGroupGroupedCanonicalRows orders group rows by their
// group key. Java delivers snapshot-every batches in HashMap group order, which
// is not a contract; both traces therefore pin the same canonical order (the
// string key bytewise, then the integer key numerically, matching the oracle's
// String.compareTo/Integer.compare comparator) while the per-send batches stay
// untouched because they carry a single row.
func resultSetQueryTypeLocalGroupGroupedCanonicalRows(rows []compat.ResultRecord) []compat.ResultRecord {
	if len(rows) < 2 {
		return rows
	}
	ordered := append([]compat.ResultRecord(nil), rows...)
	sort.SliceStable(ordered, func(left, right int) bool {
		leftString, _ := ordered[left].Fields["theString"].(string)
		rightString, _ := ordered[right].Fields["theString"].(string)
		if leftString != rightString {
			return leftString < rightString
		}
		return resultSetQueryTypeLocalGroupGroupedIntPrimitive(ordered[left]) <
			resultSetQueryTypeLocalGroupGroupedIntPrimitive(ordered[right])
	})
	return ordered
}

// resultSetQueryTypeLocalGroupGroupedIntPrimitive reads the numeric group key
// regardless of the concrete normalization type the runtime produced.
func resultSetQueryTypeLocalGroupGroupedIntPrimitive(row compat.ResultRecord) int64 {
	switch typed := row.Fields["intPrimitive"].(type) {
	case int32:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0
		}
		return parsed
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func validateResultSetQueryTypeLocalGroupGroupedTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultSetQueryTypeLocalGroupGroupedID {
		return fmt.Errorf("%s trace identity is not pinned", resultSetQueryTypeLocalGroupGroupedID)
	}
	if len(trace.Records) != 9 {
		return fmt.Errorf("%s trace must contain exactly nine records", resultSetQueryTypeLocalGroupGroupedID)
	}
	for index := 0; index < 5; index++ {
		record := trace.Records[index]
		if record.Case != "grouped-simple" || record.Operation != "listener" ||
			record.Statement != "s0" || record.Sequence != uint64(index+1) ||
			len(record.New) != 1 {
			return fmt.Errorf("%s grouped-simple record %d is not pinned", resultSetQueryTypeLocalGroupGroupedID, index)
		}
	}
	for _, spec := range []struct {
		name  string
		start int
	}{
		{name: "grouped-multi-level-method", start: 5},
		{name: "grouped-multi-level-no-default-lvl", start: 7},
	} {
		for offset := 0; offset < 2; offset++ {
			record := trace.Records[spec.start+offset]
			if record.Case != spec.name || record.Operation != "listener" ||
				record.Statement != "s0" || record.Sequence != uint64(offset+1) || len(record.New) != 3 {
				return fmt.Errorf("%s %s record %d is not pinned", resultSetQueryTypeLocalGroupGroupedID, spec.name, offset)
			}
		}
	}
	return nil
}
