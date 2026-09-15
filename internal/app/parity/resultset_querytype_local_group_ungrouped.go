package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultSetQueryTypeLocalGroupUngroupedID          = "resultset-querytype-local-group-ungrouped"
	resultSetQueryTypeLocalGroupUngroupedJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeLocalGroupUngroupedSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
	resultSetQueryTypeLocalGroupUngroupedDescription = "ResultSetQueryTypeLocalGroupBy ordinals 3-6: ungrouped local-group aggregates across iterator, listener and statement-metadata observations."

	resultSetQueryTypeLocalGroupUngroupedIteratorRuntimeID  = "java-runtime-2116fa46dfb43cd6ec83"
	resultSetQueryTypeLocalGroupUngroupedSodaTextRuntimeID  = "java-runtime-1309ac2ab21826013abf"
	resultSetQueryTypeLocalGroupUngroupedSodaModelRuntimeID = "java-runtime-e9dbed674201a7324216"
	resultSetQueryTypeLocalGroupUngroupedColNameRuntimeID   = "java-runtime-748fbcf754462901d0e0"

	// The Java oracle pins the paren/SODA pair on one shared static candidate
	// because both variants are the same execution class.
	resultSetQueryTypeLocalGroupUngroupedIteratorStaticID = "java-15d21d1c89c691d1be52"
	resultSetQueryTypeLocalGroupUngroupedSodaStaticID     = "java-dac296ac610fa9db8d71"
	resultSetQueryTypeLocalGroupUngroupedColNameStaticID  = "java-4d4e4747a49e26787a4d"

	// The three EPL texts are the verbatim Java source statements with runs of
	// whitespace collapsed to single spaces, matching the sibling
	// resultset-querytype-local-group-by chain convention.
	resultSetQueryTypeLocalGroupUngroupedIteratorEPL = "@name('s0') select intPrimitive as c0, sum(intPrimitive, group_by:()) as sum0, sum(intPrimitive, group_by:(theString)) as sum1 from SupportBean#keepall"
	resultSetQueryTypeLocalGroupUngroupedParenEPL    = "@name('s0') select longPrimitive, sum(longPrimitive) as c0, sum(group_by:(),longPrimitive) as c1, sum(longPrimitive,group_by:()) as c2, sum(longPrimitive,group_by:theString) as c3, sum(longPrimitive,group_by:(theString,intPrimitive)) as c4 from SupportBean"
	resultSetQueryTypeLocalGroupUngroupedColNameEPL  = "@name('s0') select count(*, group_by:(theString, intPrimitive)), count(group_by:theString, *) from SupportBean"

	// Java auto-derives these two column names; the typed Go surface pins the
	// identical observable schema through explicit aliases.
	resultSetQueryTypeLocalGroupUngroupedColNameCountBoth = "count(*,group_by:(theString,intPrimitive))"
	resultSetQueryTypeLocalGroupUngroupedColNameCountKey  = "count(group_by:theString,*)"
)

type resultSetQueryTypeLocalGroupUngroupedCaseSpec struct {
	name              string
	ordinal           int
	runtimeID         string
	execution         string
	observation       string
	iteratorSnapshots int
	epl               string
}

var resultSetQueryTypeLocalGroupUngroupedCaseSpecs = []resultSetQueryTypeLocalGroupUngroupedCaseSpec{
	{
		name:              "ungrouped-agg-iterator",
		ordinal:           3,
		runtimeID:         resultSetQueryTypeLocalGroupUngroupedIteratorRuntimeID,
		execution:         "ResultSetLocalUngroupedAggIterator",
		observation:       "iterator",
		iteratorSnapshots: 3,
		epl:               resultSetQueryTypeLocalGroupUngroupedIteratorEPL,
	},
	{
		name:              "ungrouped-paren-soda-text",
		ordinal:           4,
		runtimeID:         resultSetQueryTypeLocalGroupUngroupedSodaTextRuntimeID,
		execution:         "ResultSetLocalUngroupedParenSODA{soda=false}",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupUngroupedParenEPL,
	},
	{
		name:              "ungrouped-paren-soda-model",
		ordinal:           5,
		runtimeID:         resultSetQueryTypeLocalGroupUngroupedSodaModelRuntimeID,
		execution:         "ResultSetLocalUngroupedParenSODA{soda=true}",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupUngroupedParenEPL,
	},
	{
		name:              "ungrouped-colname-rendering",
		ordinal:           6,
		runtimeID:         resultSetQueryTypeLocalGroupUngroupedColNameRuntimeID,
		execution:         "ResultSetLocalUngroupedColNameRendering",
		observation:       "statement-metadata",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupUngroupedColNameEPL,
	},
}

var (
	resultSetQueryTypeLocalGroupUngroupedJavaSources   = []string{resultSetQueryTypeLocalGroupUngroupedSource}
	resultSetQueryTypeLocalGroupUngroupedJavaStaticIDs = []string{
		resultSetQueryTypeLocalGroupUngroupedIteratorStaticID,
		resultSetQueryTypeLocalGroupUngroupedSodaStaticID,
		resultSetQueryTypeLocalGroupUngroupedColNameStaticID,
	}
)

func resultSetQueryTypeLocalGroupUngroupedJavaRuntimeIDs() []string {
	ids := make([]string, 0, len(resultSetQueryTypeLocalGroupUngroupedCaseSpecs))
	for _, spec := range resultSetQueryTypeLocalGroupUngroupedCaseSpecs {
		ids = append(ids, spec.runtimeID)
	}
	return ids
}

func resultSetQueryTypeLocalGroupUngroupedJavaExecutions() []string {
	names := make([]string, 0, len(resultSetQueryTypeLocalGroupUngroupedCaseSpecs))
	for _, spec := range resultSetQueryTypeLocalGroupUngroupedCaseSpecs {
		names = append(names, spec.execution)
	}
	return names
}

// resultSetQueryTypeLocalGroupUngroupedStepSpec pins every scenario step in
// order; the loader rejects any drift before the runtime is touched.
type resultSetQueryTypeLocalGroupUngroupedStepSpec struct {
	op        string
	caseName  string
	eventType string
	payload   resultSetQueryTypeLocalGroupByPayload
	statement string
	mode      string
}

var resultSetQueryTypeLocalGroupUngroupedStepSpecs = []resultSetQueryTypeLocalGroupUngroupedStepSpec{
	{op: "case", caseName: "ungrouped-agg-iterator"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 0}},
	{op: "snapshot", statement: "s0", mode: "any"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 20, LongPrimitive: 0}},
	{op: "snapshot", statement: "s0", mode: "any"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 30, LongPrimitive: 0}},
	{op: "snapshot", statement: "s0", mode: "any"},
	{op: "case", caseName: "ungrouped-paren-soda-text"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 1, LongPrimitive: 10}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 2, LongPrimitive: 11}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 1, LongPrimitive: 12}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 2, LongPrimitive: 13}},
	{op: "case", caseName: "ungrouped-paren-soda-model"},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 1, LongPrimitive: 10}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 2, LongPrimitive: 11}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 1, LongPrimitive: 12}},
	{op: "send", eventType: "SupportBean", payload: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 2, LongPrimitive: 13}},
	{op: "case", caseName: "ungrouped-colname-rendering"},
	{op: "deployed", statement: "s0"},
	{op: "types", statement: "s0"},
}

// loadResultSetQueryTypeLocalGroupUngroupedScenario performs the strict,
// duplicate-rejecting load used by the parity dispatcher.
func loadResultSetQueryTypeLocalGroupUngroupedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultSetQueryTypeLocalGroupUngroupedID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultSetQueryTypeLocalGroupUngroupedID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupUngroupedID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupUngroupedID, err)
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
	if version != compat.ScenarioVersion || id != resultSetQueryTypeLocalGroupUngroupedID ||
		description != resultSetQueryTypeLocalGroupUngroupedDescription ||
		javaCommit != resultSetQueryTypeLocalGroupUngroupedJavaCommit ||
		javaSource != resultSetQueryTypeLocalGroupUngroupedSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultSetQueryTypeLocalGroupUngroupedID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], resultSetQueryTypeLocalGroupUngroupedJavaRuntimeIDs(), "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], resultSetQueryTypeLocalGroupUngroupedJavaExecutions(), "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], resultSetQueryTypeLocalGroupUngroupedJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultSetQueryTypeLocalGroupUngroupedCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", resultSetQueryTypeLocalGroupUngroupedID, len(resultSetQueryTypeLocalGroupUngroupedCaseSpecs))
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
		spec := resultSetQueryTypeLocalGroupUngroupedCaseSpecs[index]
		if err := json.Unmarshal(rawCase, &caseMeta); err != nil ||
			caseMeta.Case != spec.name || ordinal != spec.ordinal || caseMeta.Ordinal != ordinal ||
			caseMeta.RuntimeID != spec.runtimeID || caseMeta.ExecutionName != spec.execution ||
			caseMeta.Observation != spec.observation || iteratorSnapshots != spec.iteratorSnapshots ||
			caseMeta.IteratorSnapshots != iteratorSnapshots || caseMeta.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultSetQueryTypeLocalGroupUngroupedID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(resultSetQueryTypeLocalGroupUngroupedStepSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", resultSetQueryTypeLocalGroupUngroupedID, len(resultSetQueryTypeLocalGroupUngroupedStepSpecs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := resultSetQueryTypeLocalGroupUngroupedStepSpecs[index]
		var expected []string
		switch spec.op {
		case "case":
			expected = []string{"op", "case"}
		case "send":
			expected = []string{"op", "eventType", "payload"}
		case "snapshot":
			expected = []string{"op", "statement", "mode"}
		default:
			expected = []string{"op", "statement"}
		}
		if err := requireResultSetQueryTypeLocalGroupByFields(object, expected...); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateResultSetQueryTypeLocalGroupUngroupedScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeLocalGroupUngroupedScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeLocalGroupUngroupedID ||
		len(scenario.Steps) != len(resultSetQueryTypeLocalGroupUngroupedStepSpecs) {
		return fmt.Errorf("%s scenario steps are not pinned", resultSetQueryTypeLocalGroupUngroupedID)
	}
	for index, spec := range resultSetQueryTypeLocalGroupUngroupedStepSpecs {
		step := scenario.Steps[index]
		if step.Op != spec.op {
			return fmt.Errorf("%s scenario step %d must be %q", resultSetQueryTypeLocalGroupUngroupedID, index, spec.op)
		}
		switch spec.op {
		case "case":
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", resultSetQueryTypeLocalGroupUngroupedID, index, spec.caseName)
			}
		case "send":
			if step.EventType != spec.eventType || step.Case != "" {
				return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", resultSetQueryTypeLocalGroupUngroupedID, index, spec.eventType)
			}
			actual, err := decodeResultSetQueryTypeLocalGroupByPayload(step)
			if err != nil {
				return fmt.Errorf("%s scenario step %d: %w", resultSetQueryTypeLocalGroupUngroupedID, index, err)
			}
			if actual != spec.payload {
				return fmt.Errorf("%s scenario step %d payload is not pinned", resultSetQueryTypeLocalGroupUngroupedID, index)
			}
		case "snapshot":
			if step.Statement != spec.statement || step.Mode != spec.mode || step.Case != "" {
				return fmt.Errorf("%s scenario step %d snapshot metadata is not pinned", resultSetQueryTypeLocalGroupUngroupedID, index)
			}
		default:
			if step.Statement != spec.statement || step.Case != "" {
				return fmt.Errorf("%s scenario step %d %s metadata is not pinned", resultSetQueryTypeLocalGroupUngroupedID, index, spec.op)
			}
		}
	}
	return nil
}

func runResultSetQueryTypeLocalGroupUngroupedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeLocalGroupUngroupedScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultSetQueryTypeLocalGroupUngroupedID}
	for _, spec := range resultSetQueryTypeLocalGroupUngroupedCaseSpecs {
		caseScenario, err := scenarioForCase(scenario, spec.name)
		if err != nil {
			return compat.Trace{}, err
		}
		records, err := runResultSetQueryTypeLocalGroupUngroupedCase(ctx, spec, caseScenario)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultSetQueryTypeLocalGroupUngroupedID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	if err := validateResultSetQueryTypeLocalGroupUngroupedTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func runResultSetQueryTypeLocalGroupUngroupedQuery(env *esper.Environment, caseName string) (esper.Query, error) {
	// The SupportBean mirror and payload decoded by the sibling
	// resultset-querytype-local-group-by chain expose exactly the three fields
	// these statements select, so both chains share one event type.
	theString := esper.Field[resultSetQueryTypeLocalGroupByBean, string]("theString")
	intPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int32]("intPrimitive")
	longPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int64]("longPrimitive")
	from := esper.From[resultSetQueryTypeLocalGroupByBean](env, "SupportBean")
	switch caseName {
	case "ungrouped-agg-iterator":
		return from.Window(esper.KeepAll()).Aggregate(
			esper.Alias("c0", intPrimitive),
			esper.Alias("sum0", esper.LocalGroupBy[int32](esper.Sum[int32](intPrimitive))),
			esper.Alias("sum1", esper.LocalGroupBy[int32](esper.Sum[int32](intPrimitive), theString)),
		).Query(esper.StatementName("s0")), nil
	case "ungrouped-paren-soda-text", "ungrouped-paren-soda-model":
		return from.Aggregate(
			esper.Alias("longPrimitive", longPrimitive),
			esper.Alias("c0", esper.Sum[int64](longPrimitive)),
			esper.Alias("c1", esper.LocalGroupBy[int64](esper.Sum[int64](longPrimitive))),
			esper.Alias("c2", esper.LocalGroupBy[int64](esper.Sum[int64](longPrimitive))),
			esper.Alias("c3", esper.LocalGroupBy[int64](esper.Sum[int64](longPrimitive), theString)),
			esper.Alias("c4", esper.LocalGroupBy[int64](esper.Sum[int64](longPrimitive), theString, intPrimitive)),
		).Query(esper.StatementName("s0")), nil
	case "ungrouped-colname-rendering":
		return from.Aggregate(
			esper.Alias(resultSetQueryTypeLocalGroupUngroupedColNameCountBoth,
				esper.LocalGroupBy[int64](esper.CountAll(), theString, intPrimitive)),
			esper.Alias(resultSetQueryTypeLocalGroupUngroupedColNameCountKey,
				esper.LocalGroupBy[int64](esper.CountAll(), theString)),
		).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("unknown %s case %q", resultSetQueryTypeLocalGroupUngroupedID, caseName)
	}
}

func runResultSetQueryTypeLocalGroupUngroupedCase(ctx context.Context, spec resultSetQueryTypeLocalGroupUngroupedCaseSpec, caseScenario compat.Scenario) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupByBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	query, err := runResultSetQueryTypeLocalGroupUngroupedQuery(env, spec.name)
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

	records := make([]compat.TraceRecord, 0, 4)
	var sequence uint64
	switch spec.observation {
	case "listener":
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
	case "iterator", "statement-metadata":
	default:
		return nil, fmt.Errorf("%s case %q has unsupported observation %q", resultSetQueryTypeLocalGroupUngroupedID, spec.name, spec.observation)
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
			// CharPrimitive pins Java's default '\u0000' character so the
			// shared SupportBean mirror renders like the sibling chain.
			bean := resultSetQueryTypeLocalGroupByBean{
				TheString: payload.TheString, IntPrimitive: payload.IntPrimitive,
				LongPrimitive: payload.LongPrimitive, CharPrimitive: "\u0000",
			}
			if err := engine.Send(ctx, step.EventType, bean); err != nil {
				return nil, err
			}
		case "snapshot":
			if spec.observation != "iterator" {
				return nil, fmt.Errorf("%s case %q must not take iterator snapshots", resultSetQueryTypeLocalGroupUngroupedID, spec.name)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return nil, err
			}
			sequence++
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "snapshot",
				Statement: statement.Name(),
				Sequence:  sequence,
				Time:      compat.FormatTraceTime(engine.Now()),
				New:       resultSetQueryTypeLocalGroupUngroupedRows(result.Batch.New),
			})
		case "deployed":
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  0,
			})
		case "types":
			// Java auto-names both unaliased count(*) projections; the typed
			// Go surface pins the identical names as explicit aliases, so the
			// ordered names and boxed Long tokens are differential.
			schema, ok := plan.ResultSchema()
			if !ok {
				return nil, fmt.Errorf("%s case %q statement has no result schema", resultSetQueryTypeLocalGroupUngroupedID, spec.name)
			}
			fields := schema.Fields()
			if len(fields) != 2 {
				return nil, fmt.Errorf("%s case %q schema has %d fields, want 2", resultSetQueryTypeLocalGroupUngroupedID, spec.name, len(fields))
			}
			want := []string{resultSetQueryTypeLocalGroupUngroupedColNameCountBoth, resultSetQueryTypeLocalGroupUngroupedColNameCountKey}
			value := make([]map[string]any, 0, len(fields))
			for index, field := range fields {
				if field.Name != want[index] {
					return nil, fmt.Errorf("%s case %q field %d = %q, want %q", resultSetQueryTypeLocalGroupUngroupedID, spec.name, index, field.Name, want[index])
				}
				token := javaTypeName(field.Type)
				if token != "Long" {
					return nil, fmt.Errorf("%s case %q field %q type = %q, want Long", resultSetQueryTypeLocalGroupUngroupedID, spec.name, field.Name, token)
				}
				value = append(value, map[string]any{"name": field.Name, "type": token})
			}
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "types",
				Statement: step.Statement,
				Sequence:  0,
				Value:     value,
			})
		default:
			return nil, fmt.Errorf("unsupported %s step op %q", resultSetQueryTypeLocalGroupUngroupedID, step.Op)
		}
	}
	return records, nil
}

func resultSetQueryTypeLocalGroupUngroupedRows(results []esper.Result) []compat.ResultRecord {
	rows := compat.NormalizeResults(results)
	if rows == nil {
		return []compat.ResultRecord{}
	}
	return rows
}

func validateResultSetQueryTypeLocalGroupUngroupedTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultSetQueryTypeLocalGroupUngroupedID {
		return fmt.Errorf("%s trace identity is not pinned", resultSetQueryTypeLocalGroupUngroupedID)
	}
	type expectation struct {
		operation string
		sequence  uint64
		time      string
	}
	var want []expectation
	for _, spec := range resultSetQueryTypeLocalGroupUngroupedCaseSpecs {
		switch spec.observation {
		case "iterator":
			for index := 1; index <= spec.iteratorSnapshots; index++ {
				want = append(want, expectation{operation: "snapshot", sequence: uint64(index), time: time.Unix(0, 0).UTC().Format(time.RFC3339)})
			}
		case "listener":
			for index := 1; index <= 4; index++ {
				want = append(want, expectation{operation: "listener", sequence: uint64(index), time: time.Unix(0, 0).UTC().Format(time.RFC3339)})
			}
		case "statement-metadata":
			want = append(want, expectation{operation: "deployed"}, expectation{operation: "types"})
		}
	}
	if len(trace.Records) != len(want) {
		return fmt.Errorf("%s trace must contain exactly %d records", resultSetQueryTypeLocalGroupUngroupedID, len(want))
	}
	index := 0
	for _, spec := range resultSetQueryTypeLocalGroupUngroupedCaseSpecs {
		count := 0
		switch spec.observation {
		case "iterator":
			count = spec.iteratorSnapshots
		case "listener":
			count = 4
		case "statement-metadata":
			count = 2
		}
		for offset := 0; offset < count; offset++ {
			record := trace.Records[index]
			expected := want[index]
			if record.Case != spec.name || record.Operation != expected.operation ||
				record.Statement != "s0" || record.Sequence != expected.sequence || record.Time != expected.time {
				return fmt.Errorf("%s trace record %d is not pinned", resultSetQueryTypeLocalGroupUngroupedID, index)
			}
			if expected.operation == "deployed" || expected.operation == "types" {
				if len(record.New) != 0 || len(record.Old) != 0 {
					return fmt.Errorf("%s %s record must not carry rows", resultSetQueryTypeLocalGroupUngroupedID, expected.operation)
				}
				if expected.operation == "deployed" && record.Value != nil {
					return fmt.Errorf("%s deployed record must not carry a value", resultSetQueryTypeLocalGroupUngroupedID)
				}
				if expected.operation == "types" {
					entries, ok := record.Value.([]map[string]any)
					if !ok || len(entries) != 2 {
						return fmt.Errorf("%s types record must carry the two ordered column entries", resultSetQueryTypeLocalGroupUngroupedID)
					}
				}
			} else if len(record.New) == 0 {
				return fmt.Errorf("%s trace record %d must carry rows", resultSetQueryTypeLocalGroupUngroupedID, index)
			}
			index++
		}
	}
	return nil
}
