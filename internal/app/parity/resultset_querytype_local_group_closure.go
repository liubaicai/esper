package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultSetQueryTypeLocalGroupClosureID          = "resultset-querytype-local-group-closure"
	resultSetQueryTypeLocalGroupClosureJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeLocalGroupClosureSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
	resultSetQueryTypeLocalGroupClosureDescription = "ResultSetQueryTypeLocalGroupBy ordinals 22/25 (file closure): grouped on-select over a named window with a statement-wide group_by:() sum, and ungrouped additional aggregates (countever/concatstring/sc/leaving/rate/nth) over local group keys."

	resultSetQueryTypeLocalGroupClosureOnSelectRuntimeID = "java-runtime-efa4ac181b956105b4bb"
	resultSetQueryTypeLocalGroupClosurePluginRuntimeID   = "java-runtime-27dff810bb25f959acbc"

	// Both executions live in the same Java executions() collection and share
	// one static inventory ID.
	resultSetQueryTypeLocalGroupClosureStaticID = "java-13f0da7834ee65870fb0"

	// The pinned EPL texts are the Java source statements verbatim.
	resultSetQueryTypeLocalGroupClosureOnSelectEPL = "create window MyWindow#keepall as SupportBean;\ninsert into MyWindow select * from SupportBean;\n@name('s0') on SupportBean_S0 select theString, sum(intPrimitive) as c0, sum(intPrimitive, group_by:()) as c1 from MyWindow group by theString;\n"
	resultSetQueryTypeLocalGroupClosurePluginEPL   = "@name('s0') select intPrimitive, countever(*, intPrimitive>0, group_by:(theString)) as c0, countever(*, intPrimitive>0, group_by:()) as c1, countever(*, group_by:(theString)) as c2, countever(*, group_by:()) as c3, concatstring(Integer.toString(intPrimitive), group_by:(theString)) as c4, concatstring(Integer.toString(intPrimitive), group_by:()) as c5, sc(intPrimitive, group_by:(theString)) as c6, sc(intPrimitive, group_by:()) as c7, leaving(group_by:(theString)) as c8, leaving(group_by:()) as c9, rate(3, group_by:(theString)) as c10, rate(3, group_by:()) as c11, nth(intPrimitive, 1, group_by:(theString)) as c12, nth(intPrimitive, 1, group_by:()) as c13 from SupportBean as sb\n"
)

type resultSetQueryTypeLocalGroupClosureCaseSpec struct {
	name          string
	ordinal       int
	runtimeID     string
	execution     string
	observation   string
	epl           string
	iteratorSnaps int
}

var resultSetQueryTypeLocalGroupClosureCaseSpecs = []resultSetQueryTypeLocalGroupClosureCaseSpec{
	{
		name:          "local-group-on-select",
		ordinal:       22,
		runtimeID:     resultSetQueryTypeLocalGroupClosureOnSelectRuntimeID,
		execution:     "ResultSetLocalGroupedOnSelect",
		observation:   "listener",
		epl:           resultSetQueryTypeLocalGroupClosureOnSelectEPL,
		iteratorSnaps: 0,
	},
	{
		name:          "local-group-agg-additional-plugin",
		ordinal:       25,
		runtimeID:     resultSetQueryTypeLocalGroupClosurePluginRuntimeID,
		execution:     "ResultSetLocalUngroupedAggAdditionalAndPlugin",
		observation:   "listener",
		epl:           resultSetQueryTypeLocalGroupClosurePluginEPL,
		iteratorSnaps: 0,
	},
}

var (
	resultSetQueryTypeLocalGroupClosureJavaSources   = []string{resultSetQueryTypeLocalGroupClosureSource}
	resultSetQueryTypeLocalGroupClosureJavaStaticIDs = []string{resultSetQueryTypeLocalGroupClosureStaticID}
)

func resultSetQueryTypeLocalGroupClosureJavaRuntimeIDs() []string {
	ids := make([]string, 0, len(resultSetQueryTypeLocalGroupClosureCaseSpecs))
	for _, spec := range resultSetQueryTypeLocalGroupClosureCaseSpecs {
		ids = append(ids, spec.runtimeID)
	}
	return ids
}

func resultSetQueryTypeLocalGroupClosureJavaExecutions() []string {
	names := make([]string, 0, len(resultSetQueryTypeLocalGroupClosureCaseSpecs))
	for _, spec := range resultSetQueryTypeLocalGroupClosureCaseSpecs {
		names = append(names, spec.execution)
	}
	return names
}

// resultSetQueryTypeLocalGroupClosureBean mirrors the two SupportBean members
// the pinned executions observe; the oracle registers the same map shape.
type resultSetQueryTypeLocalGroupClosureBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

// resultSetQueryTypeLocalGroupClosureS0 mirrors the on-select trigger event;
// the Java execution only reads the id member.
type resultSetQueryTypeLocalGroupClosureS0 struct {
	ID int32 `esper:"id"`
}

// resultSetQueryTypeLocalGroupClosureStepSpec pins every scenario step in
// order; the loader rejects any drift before the runtime is touched.
type resultSetQueryTypeLocalGroupClosureStepSpec struct {
	op       string
	caseName string
	kind     string
	bean     resultSetQueryTypeLocalGroupClosureBean
	s0       resultSetQueryTypeLocalGroupClosureS0
}

func resultSetQueryTypeLocalGroupClosureStepSpecs() []resultSetQueryTypeLocalGroupClosureStepSpec {
	specs := []resultSetQueryTypeLocalGroupClosureStepSpec{
		{op: "case", caseName: "local-group-on-select"},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupClosureBean{TheString: "E1", IntPrimitive: 10}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupClosureBean{TheString: "E2", IntPrimitive: 20}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupClosureBean{TheString: "E1", IntPrimitive: 30}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupClosureBean{TheString: "E3", IntPrimitive: 40}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupClosureBean{TheString: "E2", IntPrimitive: 50}},
		{op: "send", kind: "SupportBean_S0", s0: resultSetQueryTypeLocalGroupClosureS0{ID: 0}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupClosureBean{TheString: "E1", IntPrimitive: 60}},
		{op: "send", kind: "SupportBean_S0", s0: resultSetQueryTypeLocalGroupClosureS0{ID: 0}},
		{op: "case", caseName: "local-group-agg-additional-plugin"},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupClosureBean{TheString: "E1", IntPrimitive: 10}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupClosureBean{TheString: "E2", IntPrimitive: 20}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupClosureBean{TheString: "E1", IntPrimitive: -1}},
		{op: "send", kind: "SupportBean", bean: resultSetQueryTypeLocalGroupClosureBean{TheString: "E2", IntPrimitive: 30}},
	}
	return specs
}

func loadResultSetQueryTypeLocalGroupClosureScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultSetQueryTypeLocalGroupClosureID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultSetQueryTypeLocalGroupClosureID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupClosureID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupClosureID, err)
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
	if version != compat.ScenarioVersion || id != resultSetQueryTypeLocalGroupClosureID ||
		description != resultSetQueryTypeLocalGroupClosureDescription ||
		javaCommit != resultSetQueryTypeLocalGroupClosureJavaCommit ||
		javaSource != resultSetQueryTypeLocalGroupClosureSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultSetQueryTypeLocalGroupClosureID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], resultSetQueryTypeLocalGroupClosureJavaRuntimeIDs(), "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], resultSetQueryTypeLocalGroupClosureJavaExecutions(), "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], resultSetQueryTypeLocalGroupClosureJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	stepSpecs := resultSetQueryTypeLocalGroupClosureStepSpecs()

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultSetQueryTypeLocalGroupClosureCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", resultSetQueryTypeLocalGroupClosureID, len(resultSetQueryTypeLocalGroupClosureCaseSpecs))
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
		spec := resultSetQueryTypeLocalGroupClosureCaseSpecs[index]
		if err := json.Unmarshal(rawCase, &caseMeta); err != nil ||
			caseMeta.Case != spec.name || ordinal != spec.ordinal || caseMeta.Ordinal != ordinal ||
			caseMeta.RuntimeID != spec.runtimeID || caseMeta.ExecutionName != spec.execution ||
			caseMeta.Observation != spec.observation || iteratorSnapshots != spec.iteratorSnaps ||
			caseMeta.IteratorSnapshots != iteratorSnapshots || caseMeta.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultSetQueryTypeLocalGroupClosureID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(stepSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", resultSetQueryTypeLocalGroupClosureID, len(stepSpecs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := stepSpecs[index]
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
	if err := validateResultSetQueryTypeLocalGroupClosureScenario(scenario, stepSpecs); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeLocalGroupClosureScenario(scenario compat.Scenario, stepSpecs []resultSetQueryTypeLocalGroupClosureStepSpec) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeLocalGroupClosureID ||
		len(scenario.Steps) != len(stepSpecs) {
		return fmt.Errorf("%s scenario steps are not pinned", resultSetQueryTypeLocalGroupClosureID)
	}
	for index, spec := range stepSpecs {
		step := scenario.Steps[index]
		if step.Op != spec.op {
			return fmt.Errorf("%s scenario step %d must be %q", resultSetQueryTypeLocalGroupClosureID, index, spec.op)
		}
		if spec.op == "case" {
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", resultSetQueryTypeLocalGroupClosureID, index, spec.caseName)
			}
			continue
		}
		if step.EventType != spec.kind || step.Case != "" {
			return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", resultSetQueryTypeLocalGroupClosureID, index, spec.kind)
		}
		var payloadFields map[string]json.RawMessage
		if err := strictObject(step.Payload, &payloadFields); err != nil {
			return fmt.Errorf("%s scenario step %d payload: %w", resultSetQueryTypeLocalGroupClosureID, index, err)
		}
		switch spec.kind {
		case "SupportBean":
			if err := requireResultSetQueryTypeLocalGroupByFields(payloadFields, "theString", "intPrimitive"); err != nil {
				return fmt.Errorf("%s scenario step %d SupportBean payload: %w", resultSetQueryTypeLocalGroupClosureID, index, err)
			}
			var payload resultSetQueryTypeLocalGroupClosureBean
			if err := json.Unmarshal(step.Payload, &payload); err != nil || payload != spec.bean {
				return fmt.Errorf("%s scenario step %d SupportBean payload is not pinned", resultSetQueryTypeLocalGroupClosureID, index)
			}
		case "SupportBean_S0":
			if err := requireResultSetQueryTypeLocalGroupByFields(payloadFields, "id"); err != nil {
				return fmt.Errorf("%s scenario step %d SupportBean_S0 payload: %w", resultSetQueryTypeLocalGroupClosureID, index, err)
			}
			var payload resultSetQueryTypeLocalGroupClosureS0
			if err := json.Unmarshal(step.Payload, &payload); err != nil || payload != spec.s0 {
				return fmt.Errorf("%s scenario step %d SupportBean_S0 payload is not pinned", resultSetQueryTypeLocalGroupClosureID, index)
			}
		}
	}
	return nil
}

func runResultSetQueryTypeLocalGroupClosureScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeLocalGroupClosureScenario(scenario, resultSetQueryTypeLocalGroupClosureStepSpecs()); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultSetQueryTypeLocalGroupClosureID}
	for index, spec := range resultSetQueryTypeLocalGroupClosureCaseSpecs {
		caseScenario, err := scenarioForCase(scenario, spec.name)
		if err != nil {
			return compat.Trace{}, err
		}
		records, err := runResultSetQueryTypeLocalGroupClosureCase(ctx, spec, caseScenario, index)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultSetQueryTypeLocalGroupClosureID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	if err := validateResultSetQueryTypeLocalGroupClosureTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

// runResultSetQueryTypeLocalGroupClosureCase deploys the pinned statements for
// one execution and replays its send sequence.
func runResultSetQueryTypeLocalGroupClosureCase(ctx context.Context, spec resultSetQueryTypeLocalGroupClosureCaseSpec, caseScenario compat.Scenario, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupClosureBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupClosureS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}

	var plans []esper.Plan
	if caseIndex == 0 {
		// Ordinal 22: keep-all named window fed by insert-into, observed by a
		// plain grouped on-select. sum(intPrimitive, group_by:()) is
		// statement-wide over all taken rows, so the zero-key LocalGroupBy
		// evaluates through the trigger's AllGroup binding.
		windowSchema, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupClosureBean](env, "MyWindowType")
		if err != nil {
			return nil, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindow", windowSchema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
		theString := esper.Field[resultSetQueryTypeLocalGroupClosureBean, string]("theString")
		intPrimitive := esper.Field[resultSetQueryTypeLocalGroupClosureBean, int32]("intPrimitive")
		insert := esper.OnEvent(esper.From[resultSetQueryTypeLocalGroupClosureBean](env, "SupportBean")).
			InsertIntoNamedWindow("MyWindow", esper.CopyMatchingFields()).
			Query(esper.StatementName("insert"))
		onSelect := esper.OnEvent(esper.From[resultSetQueryTypeLocalGroupClosureS0](env, "SupportBean_S0")).
			SelectFromNamedWindowGroupBy("MyWindow", nil,
				[]esper.Expr{theString},
				esper.Alias("theString", theString),
				esper.Alias("c0", esper.Sum[int32](intPrimitive)),
				esper.Alias("c1", esper.LocalGroupBy[int32](esper.Sum[int32](intPrimitive))),
			).Query(esper.StatementName("s0"))
		for _, built := range []esper.Query{insert, onSelect} {
			plan, err := env.Build(built)
			if err != nil {
				return nil, err
			}
			plans = append(plans, plan)
		}
	} else {
		// Ordinal 25: ungrouped row-per-event aggregate over additional
		// aggregate forms. countever/leaving/rate/nth exist as facades;
		// concatstring and sc() are mirrored as stateful plugin aggregates
		// (the Java suite registers both as configuration plug-ins: sc is an
		// aggregation multi-function whose "sc" accessor is an append-only
		// ever scalar collection). The factory form keeps per-local-group
		// state isolation through the runtime's group replay.
		positive := esper.Greater[int32](esper.Field[resultSetQueryTypeLocalGroupClosureBean, int32]("intPrimitive"), esper.Literal(0))
		theString := esper.Field[resultSetQueryTypeLocalGroupClosureBean, string]("theString")
		intPrimitive := esper.Field[resultSetQueryTypeLocalGroupClosureBean, int32]("intPrimitive")
		// Each column gets its own plugin instance: the runtime keys plugin
		// state per expression node, and the two LocalGroupBy wrappers scope
		// the same plugin differently (per-key vs statement-wide).
		newScalarCollection := func() esper.AggregateExpression[[]int32] {
			return esper.PluginAggregateWithFactory[[]int32]("sc", intPrimitive,
				func(esper.AggregatePluginFactoryContext) esper.AggregatePluginState[[]int32] {
					return &resultSetQueryTypeLocalGroupClosureScalarCollectionState{}
				})
		}
		newConcat := func() esper.AggregateExpression[string] {
			return esper.PluginAggregateWithFactory[string]("concatstring",
				esper.Func1[int32, string]("toString", func(value int32) string { return strconv.Itoa(int(value)) }, intPrimitive),
				func(esper.AggregatePluginFactoryContext) esper.AggregatePluginState[string] {
					return &resultSetQueryTypeLocalGroupClosureConcatState{}
				})
		}
		leaving := esper.PluginAggregate[bool]("leaving", func(ctx esper.EvalContext) (bool, bool) {
			// Java's leaving() aggregate reads the row's leaving flag; the
			// pinned istream-only query never retires rows.
			return ctx.IsLeaving, true
		})
		countEverFiltered := esper.FilterAggregate[int64](esper.CountEver(), positive)
		query := esper.From[resultSetQueryTypeLocalGroupClosureBean](env, "SupportBean").Aggregate(
			esper.Alias("intPrimitive", intPrimitive),
			esper.Alias("c0", esper.LocalGroupBy[int64](countEverFiltered, theString)),
			esper.Alias("c1", esper.LocalGroupBy[int64](countEverFiltered)),
			esper.Alias("c2", esper.LocalGroupBy[int64](esper.CountEver(), theString)),
			esper.Alias("c3", esper.LocalGroupBy[int64](esper.CountEver())),
			esper.Alias("c4", esper.LocalGroupBy[string](newConcat(), theString)),
			esper.Alias("c5", esper.LocalGroupBy[string](newConcat())),
			esper.Alias("c6", esper.LocalGroupBy[[]int32](newScalarCollection(), theString)),
			esper.Alias("c7", esper.LocalGroupBy[[]int32](newScalarCollection())),
			esper.Alias("c8", esper.LocalGroupBy[bool](leaving, theString)),
			esper.Alias("c9", esper.LocalGroupBy[bool](leaving)),
			esper.Alias("c10", esper.LocalGroupBy[float64](esper.Rate(3*time.Second), theString)),
			esper.Alias("c11", esper.LocalGroupBy[float64](esper.Rate(3*time.Second))),
			esper.Alias("c12", esper.LocalGroupBy[int32](esper.Nth[int32](intPrimitive, 1), theString)),
			esper.Alias("c13", esper.LocalGroupBy[int32](esper.Nth[int32](intPrimitive, 1))),
		).Query(esper.StatementName("s0"))
		plan, err := env.Build(query)
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
				return nil, fmt.Errorf("%s deployment did not expose statement s0", resultSetQueryTypeLocalGroupClosureID)
			}
			statement = statements[0]
		}
	}
	if statement == nil {
		return nil, fmt.Errorf("%s statement s0 was not deployed", resultSetQueryTypeLocalGroupClosureID)
	}

	records := make([]compat.TraceRecord, 0, 6)
	var sequence uint64
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		newRows := compat.NormalizeResults(batch.New)
		if len(newRows) == 0 {
			return nil
		}
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       resultSetQueryTypeLocalGroupClosureCanonicalRows(newRows),
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
			switch step.EventType {
			case "SupportBean":
				var payload resultSetQueryTypeLocalGroupClosureBean
				if err := json.Unmarshal(step.Payload, &payload); err != nil {
					return nil, fmt.Errorf("%s decode SupportBean: %w", resultSetQueryTypeLocalGroupClosureID, err)
				}
				if err := engine.Send(ctx, "SupportBean", payload); err != nil {
					return nil, err
				}
			case "SupportBean_S0":
				var payload resultSetQueryTypeLocalGroupClosureS0
				if err := json.Unmarshal(step.Payload, &payload); err != nil {
					return nil, fmt.Errorf("%s decode SupportBean_S0: %w", resultSetQueryTypeLocalGroupClosureID, err)
				}
				if err := engine.Send(ctx, "SupportBean_S0", payload); err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("%s unknown event type %q", resultSetQueryTypeLocalGroupClosureID, step.EventType)
			}
		default:
			return nil, fmt.Errorf("unsupported %s step op %q", resultSetQueryTypeLocalGroupClosureID, step.Op)
		}
	}
	return records, nil
}

// resultSetQueryTypeLocalGroupClosureScalarCollectionState mirrors the pinned
// sc() aggregation observable: an insertion-ordered collection of the
// received scalars. The runtime's plugin-state replay is delta-based (Leave
// over the previous scope, Enter over the new one), so Leave removes one
// matching entry per call; the pinned unbounded stream never retires events,
// so the observable behavior stays the Java ever-collection.
type resultSetQueryTypeLocalGroupClosureScalarCollectionState struct {
	entries []int32
}

func (s *resultSetQueryTypeLocalGroupClosureScalarCollectionState) Enter(value esper.Value) {
	if !value.IsPresent() {
		return
	}
	if typed, ok := value.Any().(int32); ok {
		s.entries = append(s.entries, typed)
	}
}

func (s *resultSetQueryTypeLocalGroupClosureScalarCollectionState) Leave(value esper.Value) {
	if !value.IsPresent() {
		return
	}
	typed, ok := value.Any().(int32)
	if !ok {
		return
	}
	for index := len(s.entries) - 1; index >= 0; index-- {
		if s.entries[index] == typed {
			s.entries = append(s.entries[:index], s.entries[index+1:]...)
			return
		}
	}
}

func (s *resultSetQueryTypeLocalGroupClosureScalarCollectionState) Value() ([]int32, bool) {
	if len(s.entries) == 0 {
		return nil, false
	}
	collection := make([]int32, len(s.entries))
	copy(collection, s.entries)
	return collection, true
}

func (s *resultSetQueryTypeLocalGroupClosureScalarCollectionState) Clear() { s.entries = nil }

// resultSetQueryTypeLocalGroupClosureConcatState mirrors the pinned
// concatstring aggregation: space-joined non-null received strings with
// leave-based removal.
type resultSetQueryTypeLocalGroupClosureConcatState struct {
	parts []string
}

func (s *resultSetQueryTypeLocalGroupClosureConcatState) Enter(value esper.Value) {
	if !value.IsPresent() {
		return
	}
	if typed, ok := value.Any().(string); ok {
		s.parts = append(s.parts, typed)
	}
}

func (s *resultSetQueryTypeLocalGroupClosureConcatState) Leave(value esper.Value) {
	if !value.IsPresent() {
		return
	}
	typed, ok := value.Any().(string)
	if !ok {
		return
	}
	for index := len(s.parts) - 1; index >= 0; index-- {
		if s.parts[index] == typed {
			s.parts = append(s.parts[:index], s.parts[index+1:]...)
			return
		}
	}
}

func (s *resultSetQueryTypeLocalGroupClosureConcatState) Value() (string, bool) {
	if len(s.parts) == 0 {
		return "", false
	}
	return strings.Join(s.parts, " "), true
}

func (s *resultSetQueryTypeLocalGroupClosureConcatState) Clear() { s.parts = nil }

// resultSetQueryTypeLocalGroupClosureCanonicalRows pins the grouped on-select
// delivery order to the ascending theString key both sides agree on.
func resultSetQueryTypeLocalGroupClosureCanonicalRows(rows []compat.ResultRecord) []compat.ResultRecord {
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

func validateResultSetQueryTypeLocalGroupClosureTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultSetQueryTypeLocalGroupClosureID {
		return fmt.Errorf("%s trace identity is not pinned", resultSetQueryTypeLocalGroupClosureID)
	}
	if len(trace.Records) != 6 {
		return fmt.Errorf("%s trace must contain exactly six records", resultSetQueryTypeLocalGroupClosureID)
	}
	for index, record := range trace.Records {
		if record.Operation != "listener" || record.Statement != "s0" || len(record.New) == 0 || len(record.Old) != 0 {
			return fmt.Errorf("%s trace record %d is not pinned", resultSetQueryTypeLocalGroupClosureID, index)
		}
	}
	return nil
}
