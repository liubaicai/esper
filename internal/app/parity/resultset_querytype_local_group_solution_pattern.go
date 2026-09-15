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
	resultSetQueryTypeLocalGroupSolutionID          = "resultset-querytype-local-group-solution-pattern"
	resultSetQueryTypeLocalGroupSolutionJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeLocalGroupSolutionSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
	resultSetQueryTypeLocalGroupSolutionDescription = "ResultSetQueryTypeLocalGroupBy ordinal 12: the grouped solution-pattern ratio, count(*) divided by the statement-wide count(*, group_by:()) over a 30-second time window with an output snapshot every 10 seconds, driven entirely by virtual time (0/10/20/30s) so the final boundary expires the batch sent at zero and the third snapshot's denominator is 12."

	resultSetQueryTypeLocalGroupSolutionRuntimeID = "java-runtime-ae96db5ed464e562e6d7"
	resultSetQueryTypeLocalGroupSolutionStaticID  = "java-13f0da7834ee65870fb0"
	resultSetQueryTypeLocalGroupSolutionCase      = "grouped-solution-pattern"
	resultSetQueryTypeLocalGroupSolutionExecution = "ResultSetLocalGroupedSolutionPattern"

	// The pinned EPL text is the Java source statement verbatim, including the
	// single spaces the Java line continuations leave between the clauses.
	resultSetQueryTypeLocalGroupSolutionEPL = "@name('s0') select theString, count(*) / count(*, group_by:()) as pct from SupportBean#time(30 sec) group by theString output snapshot every 10 seconds"
)

// resultSetQueryTypeLocalGroupSolutionRecords is the pinned listener callback
// count: one snapshot per 10-second boundary and no callback for any send.
const resultSetQueryTypeLocalGroupSolutionRecords = 3

var (
	resultSetQueryTypeLocalGroupSolutionRuntimes   = []string{resultSetQueryTypeLocalGroupSolutionRuntimeID}
	resultSetQueryTypeLocalGroupSolutionExecutions = []string{resultSetQueryTypeLocalGroupSolutionExecution}
	resultSetQueryTypeLocalGroupSolutionSources    = []string{resultSetQueryTypeLocalGroupSolutionSource}
	resultSetQueryTypeLocalGroupSolutionStaticIDs  = []string{resultSetQueryTypeLocalGroupSolutionStaticID}
)

func resultSetQueryTypeLocalGroupSolutionJavaRuntimeIDs() []string {
	return append([]string(nil), resultSetQueryTypeLocalGroupSolutionRuntimes...)
}

func resultSetQueryTypeLocalGroupSolutionJavaExecutions() []string {
	return append([]string(nil), resultSetQueryTypeLocalGroupSolutionExecutions...)
}

// resultSetQueryTypeLocalGroupSolutionStepSpec pins every scenario step in
// order; the loader rejects any drift before the runtime is touched.
type resultSetQueryTypeLocalGroupSolutionStepSpec struct {
	op        string
	caseName  string
	eventType string
	payload   string
	at        string
}

func resultSetQueryTypeLocalGroupSolutionBean(theString string) string {
	return fmt.Sprintf(`{"theString":"%s","intPrimitive":0,"longPrimitive":0}`, theString)
}

var resultSetQueryTypeLocalGroupSolutionStepSpecs = func() []resultSetQueryTypeLocalGroupSolutionStepSpec {
	rounds := [][]string{
		{"A", "B", "C", "B", "B", "C"},
		{"A", "B", "B", "B", "B", "A"},
		{"C", "A", "A", "A", "B", "A"},
	}
	boundaries := []string{
		"1970-01-01T00:00:00Z",
		"1970-01-01T00:00:10Z",
		"1970-01-01T00:00:20Z",
		"1970-01-01T00:00:30Z",
	}
	specs := make([]resultSetQueryTypeLocalGroupSolutionStepSpec, 0, 23)
	specs = append(specs, resultSetQueryTypeLocalGroupSolutionStepSpec{op: "case", caseName: resultSetQueryTypeLocalGroupSolutionCase})
	for index, round := range rounds {
		specs = append(specs, resultSetQueryTypeLocalGroupSolutionStepSpec{op: "advance-time", at: boundaries[index]})
		for _, theString := range round {
			specs = append(specs, resultSetQueryTypeLocalGroupSolutionStepSpec{
				op: "send", caseName: resultSetQueryTypeLocalGroupSolutionCase,
				eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupSolutionBean(theString),
			})
		}
	}
	specs = append(specs, resultSetQueryTypeLocalGroupSolutionStepSpec{op: "advance-time", at: boundaries[len(boundaries)-1]})
	return specs
}()

func loadResultSetQueryTypeLocalGroupSolutionScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultSetQueryTypeLocalGroupSolutionID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultSetQueryTypeLocalGroupSolutionID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupSolutionID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupSolutionID, err)
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
	if version != compat.ScenarioVersion || id != resultSetQueryTypeLocalGroupSolutionID ||
		description != resultSetQueryTypeLocalGroupSolutionDescription ||
		javaCommit != resultSetQueryTypeLocalGroupSolutionJavaCommit ||
		javaSource != resultSetQueryTypeLocalGroupSolutionSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultSetQueryTypeLocalGroupSolutionID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], resultSetQueryTypeLocalGroupSolutionRuntimes, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], resultSetQueryTypeLocalGroupSolutionExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], resultSetQueryTypeLocalGroupSolutionStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != 1 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly one case", resultSetQueryTypeLocalGroupSolutionID)
	}
	var caseObject map[string]json.RawMessage
	if err := strictObject(rawCases[0], &caseObject); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	if err := requireResultSetQueryTypeLocalGroupByFields(caseObject,
		"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
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
	if err := json.Unmarshal(rawCases[0], &caseMeta); err != nil ||
		caseMeta.Case != resultSetQueryTypeLocalGroupSolutionCase || ordinal != 12 || caseMeta.Ordinal != ordinal ||
		caseMeta.RuntimeID != resultSetQueryTypeLocalGroupSolutionRuntimeID ||
		caseMeta.ExecutionName != resultSetQueryTypeLocalGroupSolutionExecution ||
		caseMeta.Observation != "listener" || iteratorSnapshots != 0 || caseMeta.IteratorSnapshots != iteratorSnapshots ||
		caseMeta.EPL != resultSetQueryTypeLocalGroupSolutionEPL {
		return compat.Scenario{}, fmt.Errorf("%s scenario case metadata is not pinned", resultSetQueryTypeLocalGroupSolutionID)
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(resultSetQueryTypeLocalGroupSolutionStepSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", resultSetQueryTypeLocalGroupSolutionID, len(resultSetQueryTypeLocalGroupSolutionStepSpecs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := resultSetQueryTypeLocalGroupSolutionStepSpecs[index]
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
	if err := validateResultSetQueryTypeLocalGroupSolutionScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeLocalGroupSolutionScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeLocalGroupSolutionID ||
		len(scenario.Steps) != len(resultSetQueryTypeLocalGroupSolutionStepSpecs) {
		return fmt.Errorf("%s scenario steps are not pinned", resultSetQueryTypeLocalGroupSolutionID)
	}
	for index, spec := range resultSetQueryTypeLocalGroupSolutionStepSpecs {
		step := scenario.Steps[index]
		if step.Op != spec.op {
			return fmt.Errorf("%s scenario step %d must be %q", resultSetQueryTypeLocalGroupSolutionID, index, spec.op)
		}
		switch spec.op {
		case "case":
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", resultSetQueryTypeLocalGroupSolutionID, index, spec.caseName)
			}
			continue
		case "advance-time":
			if step.At != spec.at || step.Case != "" {
				return fmt.Errorf("%s scenario step %d must advance to %s", resultSetQueryTypeLocalGroupSolutionID, index, spec.at)
			}
			continue
		}
		if step.EventType != spec.eventType || step.Case != "" {
			return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", resultSetQueryTypeLocalGroupSolutionID, index, spec.eventType)
		}
		compact, err := resultSetQueryTypeLocalGroupKeysCompactJSON(step.Payload)
		if err != nil {
			return fmt.Errorf("%s scenario step %d: %w", resultSetQueryTypeLocalGroupSolutionID, index, err)
		}
		if compact != spec.payload {
			return fmt.Errorf("%s scenario step %d %s payload is not pinned", resultSetQueryTypeLocalGroupSolutionID, index, spec.eventType)
		}
	}
	return nil
}

func runResultSetQueryTypeLocalGroupSolutionScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeLocalGroupSolutionScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	caseScenario, err := scenarioForCase(scenario, resultSetQueryTypeLocalGroupSolutionCase)
	if err != nil {
		return compat.Trace{}, err
	}
	records, err := runResultSetQueryTypeLocalGroupSolutionCase(ctx, caseScenario)
	if err != nil {
		return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultSetQueryTypeLocalGroupSolutionID, resultSetQueryTypeLocalGroupSolutionCase, err)
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultSetQueryTypeLocalGroupSolutionID, Records: records}
	if err := validateResultSetQueryTypeLocalGroupSolutionTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func runResultSetQueryTypeLocalGroupSolutionCase(ctx context.Context, caseScenario compat.Scenario) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupByBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	theString := esper.Field[resultSetQueryTypeLocalGroupByBean, string]("theString")
	// Esper's default division is floating (Java's DivideDouble), so the ratio
	// must be computed in float64: Divide[int64] would truncate every value to
	// 0 or 1. The denominator is count(*, group_by:()), the statement-wide
	// level, while the numerator counts the current outer theString group.
	pct := esper.Divide[float64](
		esper.Cast[int64, float64](esper.CountAll()),
		esper.Cast[int64, float64](esper.LocalGroupBy[int64](esper.CountAll())),
	)
	query := esper.From[resultSetQueryTypeLocalGroupByBean](env, "SupportBean").
		Window(esper.TimeWindow(30*time.Second)).
		GroupBy(theString).
		Select(
			esper.Alias("theString", theString),
			esper.Alias("pct", pct),
		).
		Query(esper.StatementName("s0"), esper.WithOutput(esper.OutputSnapshotEvery(10*time.Second)))
	plan, err := env.Build(query)
	if err != nil {
		return nil, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, resultSetQueryTypeLocalGroupSolutionRuntimeID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	records := make([]compat.TraceRecord, 0, resultSetQueryTypeLocalGroupSolutionRecords)
	invocations := 0
	emptyCallbacks := 0
	var sequence uint64
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		invocations++
		newRows := resultSetQueryTypeLocalGroupUngroupedRows(batch.New)
		oldRows := resultSetQueryTypeLocalGroupUngroupedRows(batch.Old)
		if len(newRows) == 0 && len(oldRows) == 0 {
			emptyCallbacks++
			return nil
		}
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      resultSetQueryTypeLocalGroupSolutionCase,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       resultSetQueryTypeLocalGroupSolutionCanonicalRows(newRows),
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
			return nil, fmt.Errorf("unsupported %s step op %q", resultSetQueryTypeLocalGroupSolutionID, step.Op)
		}
	}
	// The Java execution delivers exactly one snapshot callback per advance and
	// no callback for any send, so an empty delivery or a callback-count drift
	// is a parity failure rather than something to filter away.
	if emptyCallbacks != 0 {
		return nil, fmt.Errorf("%s delivered %d empty listener callbacks", resultSetQueryTypeLocalGroupSolutionID, emptyCallbacks)
	}
	if invocations != resultSetQueryTypeLocalGroupSolutionRecords || len(records) != resultSetQueryTypeLocalGroupSolutionRecords {
		return nil, fmt.Errorf("%s delivered %d listener callbacks (%d records), want %d",
			resultSetQueryTypeLocalGroupSolutionID, invocations, len(records), resultSetQueryTypeLocalGroupSolutionRecords)
	}
	return records, nil
}

// resultSetQueryTypeLocalGroupSolutionCanonicalRows orders the snapshot rows by
// their group key. Java delivers snapshot-every batches in HashMap group order,
// which is not a contract (the execution asserts with
// assertPropsPerRowLastNewAnyOrder); both traces therefore pin the same
// ascending-theString order, matching the oracle's comparator.
func resultSetQueryTypeLocalGroupSolutionCanonicalRows(rows []compat.ResultRecord) []compat.ResultRecord {
	if len(rows) < 2 {
		return rows
	}
	ordered := append([]compat.ResultRecord(nil), rows...)
	sort.SliceStable(ordered, func(left, right int) bool {
		leftString, _ := ordered[left].Fields["theString"].(string)
		rightString, _ := ordered[right].Fields["theString"].(string)
		return leftString < rightString
	})
	return ordered
}

func validateResultSetQueryTypeLocalGroupSolutionTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultSetQueryTypeLocalGroupSolutionID {
		return fmt.Errorf("%s trace identity is not pinned", resultSetQueryTypeLocalGroupSolutionID)
	}
	if len(trace.Records) != resultSetQueryTypeLocalGroupSolutionRecords {
		return fmt.Errorf("%s trace must contain exactly %d records", resultSetQueryTypeLocalGroupSolutionID, resultSetQueryTypeLocalGroupSolutionRecords)
	}
	wantTimes := []string{
		"1970-01-01T00:00:10Z",
		"1970-01-01T00:00:20Z",
		"1970-01-01T00:00:30Z",
	}
	wantKeys := []string{"A", "B", "C"}
	for index, record := range trace.Records {
		if record.Case != resultSetQueryTypeLocalGroupSolutionCase || record.Operation != "listener" ||
			record.Statement != "s0" || record.Sequence != uint64(index+1) || record.Time != wantTimes[index] {
			return fmt.Errorf("%s trace record %d is not pinned", resultSetQueryTypeLocalGroupSolutionID, index)
		}
		if len(record.Old) != 0 || len(record.New) != len(wantKeys) {
			return fmt.Errorf("%s trace record %d must carry three new rows and no old rows", resultSetQueryTypeLocalGroupSolutionID, index)
		}
		for rowIndex, key := range wantKeys {
			if value, ok := record.New[rowIndex].Fields["theString"].(string); !ok || value != key {
				return fmt.Errorf("%s trace record %d row %d must be group %s", resultSetQueryTypeLocalGroupSolutionID, index, rowIndex, key)
			}
			if _, ok := record.New[rowIndex].Fields["pct"]; !ok {
				return fmt.Errorf("%s trace record %d row %d must carry a pct column", resultSetQueryTypeLocalGroupSolutionID, index, rowIndex)
			}
		}
	}
	return nil
}
