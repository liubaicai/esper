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
	resultSetQueryTypeLocalGroupExtendedID          = "resultset-querytype-local-group-extended"
	resultSetQueryTypeLocalGroupExtendedJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeLocalGroupExtendedSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
	resultSetQueryTypeLocalGroupExtendedDescription = "ResultSetQueryTypeLocalGroupBy ordinals 8-9: ungrouped local aggregation over a unidirectional join and over multi-level window access."

	resultSetQueryTypeLocalGroupExtendedJoinRuntimeID = "java-runtime-a1b494ebb2cb69d4c90a"
	resultSetQueryTypeLocalGroupExtendedWTopRuntimeID = "java-runtime-344d65c309eddd141cc5"
	resultSetQueryTypeLocalGroupExtendedJoinStaticID  = "java-d4c45f0591f6b53f8f74"
	resultSetQueryTypeLocalGroupExtendedWTopStaticID  = "java-3e936d8e8328dbd8a2ed"

	// The two EPL texts are the Java source statements with runs of whitespace
	// collapsed to single spaces, matching the sibling local-group chains.
	resultSetQueryTypeLocalGroupExtendedJoinEPL = "@name('s0') select theString, sum(intPrimitive, group_by:theString) as c0 from SupportBean#keepall, SupportBean_S0 unidirectional"
	// The ordinal-9 statement concatenates its projection list without
	// whitespace after the separating commas, so the pinned text keeps that
	// shape.
	resultSetQueryTypeLocalGroupExtendedWTopEPL = "@Name('s0') select sum(longPrimitive, group_by:theString) as c0,count(*, group_by:theString) as c1,window(*, group_by:theString) as c2,sum(longPrimitive, group_by:intPrimitive) as c3,count(*, group_by:intPrimitive) as c4,window(*, group_by:intPrimitive) as c5,sum(longPrimitive, group_by:(theString, intPrimitive)) as c6,count(*, group_by:(theString, intPrimitive)) as c7,window(*, group_by:(theString, intPrimitive)) as c8,sum(longPrimitive) as c9 from SupportBean#length(4)"
)

type resultSetQueryTypeLocalGroupExtendedCaseSpec struct {
	name              string
	ordinal           int
	runtimeID         string
	execution         string
	observation       string
	iteratorSnapshots int
	epl               string
}

var resultSetQueryTypeLocalGroupExtendedCaseSpecs = []resultSetQueryTypeLocalGroupExtendedCaseSpec{
	{
		name:              "ungrouped-unidirectional-join",
		ordinal:           8,
		runtimeID:         resultSetQueryTypeLocalGroupExtendedJoinRuntimeID,
		execution:         "ResultSetLocalUngroupedUnidirectionalJoin",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupExtendedJoinEPL,
	},
	{
		name:              "ungrouped-three-level-wtop",
		ordinal:           9,
		runtimeID:         resultSetQueryTypeLocalGroupExtendedWTopRuntimeID,
		execution:         "ResultSetLocalUngroupedThreeLevelWTop",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupExtendedWTopEPL,
	},
}

var (
	resultSetQueryTypeLocalGroupExtendedJavaSources   = []string{resultSetQueryTypeLocalGroupExtendedSource}
	resultSetQueryTypeLocalGroupExtendedJavaStaticIDs = []string{
		resultSetQueryTypeLocalGroupExtendedJoinStaticID,
		resultSetQueryTypeLocalGroupExtendedWTopStaticID,
	}
)

func resultSetQueryTypeLocalGroupExtendedJavaRuntimeIDs() []string {
	ids := make([]string, 0, len(resultSetQueryTypeLocalGroupExtendedCaseSpecs))
	for _, spec := range resultSetQueryTypeLocalGroupExtendedCaseSpecs {
		ids = append(ids, spec.runtimeID)
	}
	return ids
}

func resultSetQueryTypeLocalGroupExtendedJavaExecutions() []string {
	names := make([]string, 0, len(resultSetQueryTypeLocalGroupExtendedCaseSpecs))
	for _, spec := range resultSetQueryTypeLocalGroupExtendedCaseSpecs {
		names = append(names, spec.execution)
	}
	return names
}

// resultSetQueryTypeLocalGroupExtendedStepSpec pins every scenario step in
// order; the loader rejects any drift before the runtime is touched.
type resultSetQueryTypeLocalGroupExtendedStepSpec struct {
	op        string
	caseName  string
	eventType string
	payload   resultSetQueryTypeLocalGroupByPayload
	s0ID      *int
}

var resultSetQueryTypeLocalGroupExtendedStepSpecs = []resultSetQueryTypeLocalGroupExtendedStepSpec{
	{op: "case", caseName: "ungrouped-unidirectional-join"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 0}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 20, LongPrimitive: 0}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 30, LongPrimitive: 0}},
	{op: "send", eventType: "SupportBean_S0", s0ID: intPointer(1)},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 40, LongPrimitive: 0}},
	{op: "send", eventType: "SupportBean_S0", s0ID: intPointer(1)},
	{op: "case", caseName: "ungrouped-three-level-wtop"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 10, LongPrimitive: 101}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 20, LongPrimitive: 102}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 103}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 104}},
}

func intPointer(value int) *int { return &value }

func loadResultSetQueryTypeLocalGroupExtendedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultSetQueryTypeLocalGroupExtendedID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultSetQueryTypeLocalGroupExtendedID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupExtendedID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupExtendedID, err)
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
	if version != compat.ScenarioVersion || id != resultSetQueryTypeLocalGroupExtendedID ||
		description != resultSetQueryTypeLocalGroupExtendedDescription ||
		javaCommit != resultSetQueryTypeLocalGroupExtendedJavaCommit ||
		javaSource != resultSetQueryTypeLocalGroupExtendedSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultSetQueryTypeLocalGroupExtendedID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], resultSetQueryTypeLocalGroupExtendedJavaRuntimeIDs(), "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], resultSetQueryTypeLocalGroupExtendedJavaExecutions(), "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], resultSetQueryTypeLocalGroupExtendedJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultSetQueryTypeLocalGroupExtendedCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", resultSetQueryTypeLocalGroupExtendedID, len(resultSetQueryTypeLocalGroupExtendedCaseSpecs))
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
		spec := resultSetQueryTypeLocalGroupExtendedCaseSpecs[index]
		if err := json.Unmarshal(rawCase, &caseMeta); err != nil ||
			caseMeta.Case != spec.name || ordinal != spec.ordinal || caseMeta.Ordinal != ordinal ||
			caseMeta.RuntimeID != spec.runtimeID || caseMeta.ExecutionName != spec.execution ||
			caseMeta.Observation != spec.observation || iteratorSnapshots != spec.iteratorSnapshots ||
			caseMeta.IteratorSnapshots != iteratorSnapshots || caseMeta.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultSetQueryTypeLocalGroupExtendedID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(resultSetQueryTypeLocalGroupExtendedStepSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", resultSetQueryTypeLocalGroupExtendedID, len(resultSetQueryTypeLocalGroupExtendedStepSpecs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := resultSetQueryTypeLocalGroupExtendedStepSpecs[index]
		var expected []string
		switch spec.op {
		case "case":
			expected = []string{"op", "case"}
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
	if err := validateResultSetQueryTypeLocalGroupExtendedScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeLocalGroupExtendedScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeLocalGroupExtendedID ||
		len(scenario.Steps) != len(resultSetQueryTypeLocalGroupExtendedStepSpecs) {
		return fmt.Errorf("%s scenario steps are not pinned", resultSetQueryTypeLocalGroupExtendedID)
	}
	for index, spec := range resultSetQueryTypeLocalGroupExtendedStepSpecs {
		step := scenario.Steps[index]
		if step.Op != spec.op {
			return fmt.Errorf("%s scenario step %d must be %q", resultSetQueryTypeLocalGroupExtendedID, index, spec.op)
		}
		if spec.op == "case" {
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", resultSetQueryTypeLocalGroupExtendedID, index, spec.caseName)
			}
			continue
		}
		if step.EventType != spec.eventType || step.Case != "" {
			return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", resultSetQueryTypeLocalGroupExtendedID, index, spec.eventType)
		}
		if step.EventType != "SupportBean" {
			continue
		}
		actual, err := decodeResultSetQueryTypeLocalGroupByPayload(step)
		if err != nil {
			return fmt.Errorf("%s scenario step %d: %w", resultSetQueryTypeLocalGroupExtendedID, index, err)
		}
		if actual != spec.payload {
			return fmt.Errorf("%s scenario step %d payload is not pinned", resultSetQueryTypeLocalGroupExtendedID, index)
		}
	}
	if err := validateResultSetQueryTypeLocalGroupExtendedS0Payloads(scenario); err != nil {
		return err
	}
	return nil
}

// validateResultSetQueryTypeLocalGroupExtendedS0Payloads pins the driver
// payloads, which the SupportBean decoder above does not cover.
func validateResultSetQueryTypeLocalGroupExtendedS0Payloads(scenario compat.Scenario) error {
	s0Count := 0
	for index, spec := range resultSetQueryTypeLocalGroupExtendedStepSpecs {
		if spec.s0ID == nil {
			continue
		}
		payload, err := decodeResultSetQueryTypeLocalGroupExtendedS0Payload(scenario.Steps[index])
		if err != nil {
			return fmt.Errorf("%s scenario step %d: %w", resultSetQueryTypeLocalGroupExtendedID, index, err)
		}
		if payload != *spec.s0ID {
			return fmt.Errorf("%s scenario step %d SupportBean_S0 payload is not pinned", resultSetQueryTypeLocalGroupExtendedID, index)
		}
		s0Count++
	}
	if s0Count == 0 {
		return fmt.Errorf("%s scenario must send at least one SupportBean_S0", resultSetQueryTypeLocalGroupExtendedID)
	}
	return nil
}

func decodeResultSetQueryTypeLocalGroupExtendedS0Payload(step compat.Step) (int, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return 0, err
	}
	if err := requireResultSetQueryTypeLocalGroupByFields(fields, "id"); err != nil {
		return 0, fmt.Errorf("SupportBean_S0 payload: %w", err)
	}
	var payload struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return 0, fmt.Errorf("SupportBean_S0 id must be an integer")
	}
	return payload.ID, nil
}

// resultSetQueryTypeLocalGroupExtendedPayload decodes one replay step into
// its registered event type. The scenario validator pins the expected values
// by step index before any of this runs.
func resultSetQueryTypeLocalGroupExtendedPayload(step compat.Step) (any, error) {
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
		id, err := decodeResultSetQueryTypeLocalGroupExtendedS0Payload(step)
		if err != nil {
			return nil, err
		}
		return resultSetQueryTypeRollupOrderByS0{ID: id}, nil
	default:
		return nil, fmt.Errorf("unknown %s event type %q", resultSetQueryTypeLocalGroupExtendedID, step.EventType)
	}
}

func runResultSetQueryTypeLocalGroupExtendedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeLocalGroupExtendedScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultSetQueryTypeLocalGroupExtendedID}
	for index, spec := range resultSetQueryTypeLocalGroupExtendedCaseSpecs {
		caseScenario, err := scenarioForCase(scenario, spec.name)
		if err != nil {
			return compat.Trace{}, err
		}
		records, err := runResultSetQueryTypeLocalGroupExtendedCase(ctx, spec, caseScenario, index)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultSetQueryTypeLocalGroupExtendedID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	if err := validateResultSetQueryTypeLocalGroupExtendedTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func runResultSetQueryTypeLocalGroupExtendedQuery(env *esper.Environment, caseIndex int) (esper.Query, error) {
	switch caseIndex {
	case 0:
		// Java: from SupportBean#keepall, SupportBean_S0 unidirectional - the
		// S0 stream drives, every retained bean matches, and the local-group
		// sum is evaluated per trigger over the joined tuples.
		passive := esper.From[resultSetQueryTypeLocalGroupByBean](env, "SupportBean").Window(esper.KeepAll())
		driver := esper.From[resultSetQueryTypeRollupOrderByS0](env, "SupportBean_S0")
		joined := esper.JoinMany(
			esper.JoinSource(driver).Unidirectional(),
			esper.JoinSource(passive),
		)
		theString := esper.JoinField[string](1, "theString")
		intPrimitive := esper.JoinField[int32](1, "intPrimitive")
		return joined.Aggregate(
			esper.Alias("theString", theString),
			esper.Alias("c0", esper.LocalGroupBy[int32](esper.Sum[int32](intPrimitive), theString)),
		).Query(esper.StatementName("s0")), nil
	case 1:
		longPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int64]("longPrimitive")
		theString := esper.Field[resultSetQueryTypeLocalGroupByBean, string]("theString")
		intPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int32]("intPrimitive")
		windows := func(keys ...esper.Expr) esper.AggregateExpression[[]esper.Event] {
			return esper.LocalGroupBy[[]esper.Event](esper.WindowEvents(), keys...)
		}
		sum := func(keys ...esper.Expr) esper.AggregateExpression[int64] {
			return esper.LocalGroupBy[int64](esper.Sum[int64](longPrimitive), keys...)
		}
		count := func(keys ...esper.Expr) esper.AggregateExpression[int64] {
			return esper.LocalGroupBy[int64](esper.CountAll(), keys...)
		}
		return esper.From[resultSetQueryTypeLocalGroupByBean](env, "SupportBean").
			Window(esper.LengthWindow(4)).
			Aggregate(
				esper.Alias("c0", sum(theString)),
				esper.Alias("c1", count(theString)),
				esper.Alias("c2", windows(theString)),
				esper.Alias("c3", sum(intPrimitive)),
				esper.Alias("c4", count(intPrimitive)),
				esper.Alias("c5", windows(intPrimitive)),
				esper.Alias("c6", sum(theString, intPrimitive)),
				esper.Alias("c7", count(theString, intPrimitive)),
				esper.Alias("c8", windows(theString, intPrimitive)),
				esper.Alias("c9", esper.Sum[int64](longPrimitive)),
			).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("unknown %s case index %d", resultSetQueryTypeLocalGroupExtendedID, caseIndex)
	}
}

func runResultSetQueryTypeLocalGroupExtendedCase(ctx context.Context, spec resultSetQueryTypeLocalGroupExtendedCaseSpec, caseScenario compat.Scenario, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupByBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if caseIndex == 0 {
		if _, err := esper.RegisterStruct[resultSetQueryTypeRollupOrderByS0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
	}
	query, err := runResultSetQueryTypeLocalGroupExtendedQuery(env, caseIndex)
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
			payload, err := resultSetQueryTypeLocalGroupExtendedPayload(step)
			if err != nil {
				return nil, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported %s step op %q", resultSetQueryTypeLocalGroupExtendedID, step.Op)
		}
	}
	return records, nil
}

func validateResultSetQueryTypeLocalGroupExtendedTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultSetQueryTypeLocalGroupExtendedID {
		return fmt.Errorf("%s trace identity is not pinned", resultSetQueryTypeLocalGroupExtendedID)
	}
	if len(trace.Records) != 7 {
		return fmt.Errorf("%s trace must contain exactly seven records", resultSetQueryTypeLocalGroupExtendedID)
	}
	joinRows := []int{3, 4}
	for index, want := range joinRows {
		record := trace.Records[index]
		if record.Case != "ungrouped-unidirectional-join" || record.Operation != "listener" ||
			record.Statement != "s0" || record.Sequence != uint64(index+1) ||
			record.Time != "1970-01-01T00:00:00Z" || len(record.Old) != 0 || len(record.New) != want {
			return fmt.Errorf("%s join record %d is not pinned", resultSetQueryTypeLocalGroupExtendedID, index)
		}
	}
	for offset := 0; offset < 5; offset++ {
		record := trace.Records[2+offset]
		if record.Case != "ungrouped-three-level-wtop" || record.Operation != "listener" ||
			record.Statement != "s0" || record.Sequence != uint64(offset+1) ||
			record.Time != "1970-01-01T00:00:00Z" || len(record.Old) != 0 || len(record.New) != 1 {
			return fmt.Errorf("%s wtop record %d is not pinned", resultSetQueryTypeLocalGroupExtendedID, offset)
		}
	}
	return nil
}
