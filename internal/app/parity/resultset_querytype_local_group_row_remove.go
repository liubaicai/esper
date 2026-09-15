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
	resultSetQueryTypeLocalGroupRowRemoveID          = "resultset-querytype-local-group-row-remove"
	resultSetQueryTypeLocalGroupRowRemoveJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeLocalGroupRowRemoveSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
	resultSetQueryTypeLocalGroupRowRemoveDescription = "ResultSetQueryTypeLocalGroupBy ordinals 20/21: named-window row removal with ungrouped and grouped local-group aggregates."

	resultSetQueryTypeLocalGroupRowRemoveUngroupedRuntimeID = "java-runtime-3a47ac428a21e66d8614"
	resultSetQueryTypeLocalGroupRowRemoveGroupedRuntimeID   = "java-runtime-07d68fdd9a8dffc8c7f4"

	resultSetQueryTypeLocalGroupRowRemoveUngroupedStaticID = "java-3da6ea3da5df0d53578a"
	resultSetQueryTypeLocalGroupRowRemoveGroupedStaticID   = "java-ce99d7bbb48728947c59"

	// The pinned EPL texts are the Java source statements verbatim, including
	// the newlines that separate the five deployed statements and the double
	// spaces left by the Java line continuations.
	resultSetQueryTypeLocalGroupRowRemoveUngroupedEPL = "create window MyWindow#keepall as SupportBean;\ninsert into MyWindow select * from SupportBean;\non SupportBean_S0 delete from MyWindow where p00 = theString and id = intPrimitive;\non SupportBean_S1 delete from MyWindow;\n@name('s0') select theString, intPrimitive, sum(longPrimitive) as c0,   sum(longPrimitive, group_by:theString) as c1 from MyWindow;\n"
	resultSetQueryTypeLocalGroupRowRemoveGroupedEPL   = "create window MyWindow#keepall as SupportBean;\ninsert into MyWindow select * from SupportBean;\non SupportBean_S0 delete from MyWindow where p00 = theString and id = intPrimitive;\non SupportBean_S1 delete from MyWindow;\n@name('s0') select theString, intPrimitive, sum(longPrimitive) as c0,   sum(longPrimitive, group_by:theString) as c1   from MyWindow group by theString, intPrimitive;\n"
)

type resultSetQueryTypeLocalGroupRowRemoveCaseSpec struct {
	name              string
	ordinal           int
	runtimeID         string
	execution         string
	observation       string
	iteratorSnapshots int
	epl               string
}

var resultSetQueryTypeLocalGroupRowRemoveCaseSpecs = []resultSetQueryTypeLocalGroupRowRemoveCaseSpec{
	{
		name:              "ungrouped-row-remove",
		ordinal:           20,
		runtimeID:         resultSetQueryTypeLocalGroupRowRemoveUngroupedRuntimeID,
		execution:         "ResultSetLocalUngroupedRowRemove",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupRowRemoveUngroupedEPL,
	},
	{
		name:              "grouped-row-remove",
		ordinal:           21,
		runtimeID:         resultSetQueryTypeLocalGroupRowRemoveGroupedRuntimeID,
		execution:         "ResultSetLocalGroupedRowRemove",
		observation:       "listener",
		iteratorSnapshots: 0,
		epl:               resultSetQueryTypeLocalGroupRowRemoveGroupedEPL,
	},
}

var (
	resultSetQueryTypeLocalGroupRowRemoveJavaSources   = []string{resultSetQueryTypeLocalGroupRowRemoveSource}
	resultSetQueryTypeLocalGroupRowRemoveJavaStaticIDs = []string{
		resultSetQueryTypeLocalGroupRowRemoveUngroupedStaticID,
		resultSetQueryTypeLocalGroupRowRemoveGroupedStaticID,
	}
)

func resultSetQueryTypeLocalGroupRowRemoveJavaRuntimeIDs() []string {
	ids := make([]string, 0, len(resultSetQueryTypeLocalGroupRowRemoveCaseSpecs))
	for _, spec := range resultSetQueryTypeLocalGroupRowRemoveCaseSpecs {
		ids = append(ids, spec.runtimeID)
	}
	return ids
}

func resultSetQueryTypeLocalGroupRowRemoveJavaExecutions() []string {
	names := make([]string, 0, len(resultSetQueryTypeLocalGroupRowRemoveCaseSpecs))
	for _, spec := range resultSetQueryTypeLocalGroupRowRemoveCaseSpecs {
		names = append(names, spec.execution)
	}
	return names
}

// resultSetQueryTypeLocalGroupRowRemoveS0 mirrors the driver bean used by the
// delete trigger; the predicate compares p00 with the window's theString and
// id with the window's intPrimitive.
type resultSetQueryTypeLocalGroupRowRemoveS0 struct {
	ID  int32  `esper:"id"`
	P00 string `esper:"p00"`
}

// resultSetQueryTypeLocalGroupRowRemoveS1 mirrors the delete-all driver.
type resultSetQueryTypeLocalGroupRowRemoveS1 struct {
	ID int32 `esper:"id"`
}

// resultSetQueryTypeLocalGroupRowRemoveStepSpec pins every scenario step in
// order; the loader rejects any drift before the runtime is touched.
type resultSetQueryTypeLocalGroupRowRemoveStepSpec struct {
	op       string
	caseName string
	kind     string
	bean     resultSetQueryTypeLocalGroupByPayload
	s0       resultSetQueryTypeLocalGroupRowRemoveS0
	s1       resultSetQueryTypeLocalGroupRowRemoveS1
}

func resultSetQueryTypeLocalGroupRowRemoveSends() []resultSetQueryTypeLocalGroupRowRemoveStepSpec {
	return []resultSetQueryTypeLocalGroupRowRemoveStepSpec{
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 10, LongPrimitive: 101}},
		{op: "send", kind: "SupportBean_S0", s0: resultSetQueryTypeLocalGroupRowRemoveS0{ID: 10, P00: "E1"}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 20, LongPrimitive: 102}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E2", IntPrimitive: 30, LongPrimitive: 103}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 40, LongPrimitive: 104}},
		{op: "send", kind: "SupportBean_S0", s0: resultSetQueryTypeLocalGroupRowRemoveS0{ID: 40, P00: "E1"}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 50, LongPrimitive: 105}},
		{op: "send", kind: "SupportBean_S1", s1: resultSetQueryTypeLocalGroupRowRemoveS1{ID: -1}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupByPayload{TheString: "E1", IntPrimitive: 60, LongPrimitive: 106}},
	}
}

var resultSetQueryTypeLocalGroupRowRemoveStepSpecs = func() []resultSetQueryTypeLocalGroupRowRemoveStepSpec {
	specs := []resultSetQueryTypeLocalGroupRowRemoveStepSpec{{op: "case", caseName: "ungrouped-row-remove"}}
	specs = append(specs, resultSetQueryTypeLocalGroupRowRemoveSends()...)
	specs = append(specs, resultSetQueryTypeLocalGroupRowRemoveStepSpec{op: "case", caseName: "grouped-row-remove"})
	return append(specs, resultSetQueryTypeLocalGroupRowRemoveSends()...)
}()

func loadResultSetQueryTypeLocalGroupRowRemoveScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultSetQueryTypeLocalGroupRowRemoveID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultSetQueryTypeLocalGroupRowRemoveID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupRowRemoveID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupRowRemoveID, err)
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
	if version != compat.ScenarioVersion || id != resultSetQueryTypeLocalGroupRowRemoveID ||
		description != resultSetQueryTypeLocalGroupRowRemoveDescription ||
		javaCommit != resultSetQueryTypeLocalGroupRowRemoveJavaCommit ||
		javaSource != resultSetQueryTypeLocalGroupRowRemoveSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultSetQueryTypeLocalGroupRowRemoveID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], resultSetQueryTypeLocalGroupRowRemoveJavaRuntimeIDs(), "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], resultSetQueryTypeLocalGroupRowRemoveJavaExecutions(), "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], resultSetQueryTypeLocalGroupRowRemoveJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultSetQueryTypeLocalGroupRowRemoveCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", resultSetQueryTypeLocalGroupRowRemoveID, len(resultSetQueryTypeLocalGroupRowRemoveCaseSpecs))
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
		spec := resultSetQueryTypeLocalGroupRowRemoveCaseSpecs[index]
		if err := json.Unmarshal(rawCase, &caseMeta); err != nil ||
			caseMeta.Case != spec.name || ordinal != spec.ordinal || caseMeta.Ordinal != ordinal ||
			caseMeta.RuntimeID != spec.runtimeID || caseMeta.ExecutionName != spec.execution ||
			caseMeta.Observation != spec.observation || iteratorSnapshots != spec.iteratorSnapshots ||
			caseMeta.IteratorSnapshots != iteratorSnapshots || caseMeta.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultSetQueryTypeLocalGroupRowRemoveID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(resultSetQueryTypeLocalGroupRowRemoveStepSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", resultSetQueryTypeLocalGroupRowRemoveID, len(resultSetQueryTypeLocalGroupRowRemoveStepSpecs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := resultSetQueryTypeLocalGroupRowRemoveStepSpecs[index]
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
	if err := validateResultSetQueryTypeLocalGroupRowRemoveScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeLocalGroupRowRemoveScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeLocalGroupRowRemoveID ||
		len(scenario.Steps) != len(resultSetQueryTypeLocalGroupRowRemoveStepSpecs) {
		return fmt.Errorf("%s scenario steps are not pinned", resultSetQueryTypeLocalGroupRowRemoveID)
	}
	for index, spec := range resultSetQueryTypeLocalGroupRowRemoveStepSpecs {
		step := scenario.Steps[index]
		if step.Op != spec.op {
			return fmt.Errorf("%s scenario step %d must be %q", resultSetQueryTypeLocalGroupRowRemoveID, index, spec.op)
		}
		if spec.op == "case" {
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", resultSetQueryTypeLocalGroupRowRemoveID, index, spec.caseName)
			}
			continue
		}
		if step.EventType != spec.kind || step.Case != "" {
			return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", resultSetQueryTypeLocalGroupRowRemoveID, index, spec.kind)
		}
		event, err := decodeResultSetQueryTypeLocalGroupRowRemovePayload(step)
		if err != nil {
			return fmt.Errorf("%s scenario step %d: %w", resultSetQueryTypeLocalGroupRowRemoveID, index, err)
		}
		switch typed := event.(type) {
		case resultSetQueryTypeLocalGroupByBean:
			payload := spec.bean
			got := resultSetQueryTypeLocalGroupByPayload{TheString: typed.TheString, IntPrimitive: typed.IntPrimitive, LongPrimitive: typed.LongPrimitive}
			if got != payload {
				return fmt.Errorf("%s scenario step %d SupportBean payload is not pinned", resultSetQueryTypeLocalGroupRowRemoveID, index)
			}
		case resultSetQueryTypeLocalGroupRowRemoveS0:
			if typed != spec.s0 {
				return fmt.Errorf("%s scenario step %d SupportBean_S0 payload is not pinned", resultSetQueryTypeLocalGroupRowRemoveID, index)
			}
		case resultSetQueryTypeLocalGroupRowRemoveS1:
			if typed != spec.s1 {
				return fmt.Errorf("%s scenario step %d SupportBean_S1 payload is not pinned", resultSetQueryTypeLocalGroupRowRemoveID, index)
			}
		default:
			return fmt.Errorf("%s scenario step %d has an unpinned payload", resultSetQueryTypeLocalGroupRowRemoveID, index)
		}
	}
	return nil
}

// decodeResultSetQueryTypeLocalGroupRowRemovePayload decodes one replay step
// into its registered event type.
func decodeResultSetQueryTypeLocalGroupRowRemovePayload(step compat.Step) (any, error) {
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
		if err := requireResultSetQueryTypeLocalGroupByFields(fields, "id", "p00"); err != nil {
			return nil, fmt.Errorf("SupportBean_S0 payload: %w", err)
		}
		var payload resultSetQueryTypeLocalGroupRowRemoveS0
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("SupportBean_S0 payload must carry id and p00")
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
		return nil, fmt.Errorf("unknown %s event type %q", resultSetQueryTypeLocalGroupRowRemoveID, step.EventType)
	}
}

func runResultSetQueryTypeLocalGroupRowRemoveScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeLocalGroupRowRemoveScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultSetQueryTypeLocalGroupRowRemoveID}
	for index, spec := range resultSetQueryTypeLocalGroupRowRemoveCaseSpecs {
		caseScenario, err := scenarioForCase(scenario, spec.name)
		if err != nil {
			return compat.Trace{}, err
		}
		records, err := runResultSetQueryTypeLocalGroupRowRemoveCase(ctx, spec, caseScenario, index)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultSetQueryTypeLocalGroupRowRemoveID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	if err := validateResultSetQueryTypeLocalGroupRowRemoveTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

// runResultSetQueryTypeLocalGroupRowRemoveCase deploys the insert and delete
// triggers plus the observed query over one keep-all named window, mirroring
// the Java execution's single five-statement deployment.
func runResultSetQueryTypeLocalGroupRowRemoveCase(ctx context.Context, spec resultSetQueryTypeLocalGroupRowRemoveCaseSpec, caseScenario compat.Scenario, caseIndex int) ([]compat.TraceRecord, error) {
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
	windowSchema, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupByBean](env, "MyWindow")
	if err != nil {
		return nil, err
	}
	if _, err := esper.CreateNamedWindow(env, "MyWindow", windowSchema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
		return nil, err
	}

	theString := esper.Field[resultSetQueryTypeLocalGroupByBean, string]("theString")
	intPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int32]("intPrimitive")
	longPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int64]("longPrimitive")
	insert := esper.OnEvent(esper.From[resultSetQueryTypeLocalGroupByBean](env, "SupportBean")).
		InsertIntoNamedWindow("MyWindow", esper.CopyMatchingFields()).
		Query(esper.StatementName("insert"))
	deleteMatching := esper.OnEvent(esper.From[resultSetQueryTypeLocalGroupRowRemoveS0](env, "SupportBean_S0")).
		DeleteFromNamedWindow("MyWindow", esper.And(
			esper.Equal[string](
				esper.NamedWindowField[string]("theString"),
				esper.Field[resultSetQueryTypeLocalGroupRowRemoveS0, string]("p00"),
			),
			esper.Equal[int32](
				esper.NamedWindowField[int32]("intPrimitive"),
				esper.Field[resultSetQueryTypeLocalGroupRowRemoveS0, int32]("id"),
			),
		)).
		Query(esper.StatementName("deleteMatching"))
	deleteAll := esper.OnEvent(esper.From[resultSetQueryTypeLocalGroupRowRemoveS1](env, "SupportBean_S1")).
		DeleteAllFromNamedWindow("MyWindow").
		Query(esper.StatementName("deleteAll"))
	window := esper.FromNamedWindowAs[resultSetQueryTypeLocalGroupByBean](env, "MyWindow")
	var query esper.Query
	if caseIndex == 0 {
		query = window.Aggregate(
			esper.Alias("theString", theString),
			esper.Alias("intPrimitive", intPrimitive),
			esper.Alias("c0", esper.Sum[int64](longPrimitive)),
			esper.Alias("c1", esper.LocalGroupBy[int64](esper.Sum[int64](longPrimitive), theString)),
		).Query(esper.StatementName("s0"))
	} else {
		query = window.GroupBy(theString, intPrimitive).Select(
			esper.Alias("theString", theString),
			esper.Alias("intPrimitive", intPrimitive),
			esper.Alias("c0", esper.Sum[int64](longPrimitive)),
			esper.Alias("c1", esper.LocalGroupBy[int64](esper.Sum[int64](longPrimitive), theString)),
		).Query(esper.StatementName("s0"))
	}

	plans := make([]esper.Plan, 0, 4)
	for _, built := range []esper.Query{insert, deleteMatching, deleteAll, query} {
		plan, err := env.Build(built)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(spec.runtimeID),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	var statement *esper.Statement
	for index, plan := range plans {
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return nil, err
		}
		if index == len(plans)-1 {
			statements := deployment.Statements()
			if len(statements) != 1 || statements[0].Name() != "s0" {
				return nil, fmt.Errorf("%s deployment did not expose statement s0", resultSetQueryTypeLocalGroupRowRemoveID)
			}
			statement = statements[0]
		}
	}
	if statement == nil {
		return nil, fmt.Errorf("%s statement s0 was not deployed", resultSetQueryTypeLocalGroupRowRemoveID)
	}

	records := make([]compat.TraceRecord, 0, 7)
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
			payload, err := decodeResultSetQueryTypeLocalGroupRowRemovePayload(step)
			if err != nil {
				return nil, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported %s step op %q", resultSetQueryTypeLocalGroupRowRemoveID, step.Op)
		}
	}
	return records, nil
}

func validateResultSetQueryTypeLocalGroupRowRemoveTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultSetQueryTypeLocalGroupRowRemoveID {
		return fmt.Errorf("%s trace identity is not pinned", resultSetQueryTypeLocalGroupRowRemoveID)
	}
	if len(trace.Records) != 15 {
		return fmt.Errorf("%s trace must contain exactly fifteen records", resultSetQueryTypeLocalGroupRowRemoveID)
	}
	for index, record := range trace.Records {
		if record.Operation != "listener" || record.Statement != "s0" || len(record.New) == 0 || len(record.Old) != 0 {
			return fmt.Errorf("%s trace record %d is not pinned", resultSetQueryTypeLocalGroupRowRemoveID, index)
		}
	}
	return nil
}
