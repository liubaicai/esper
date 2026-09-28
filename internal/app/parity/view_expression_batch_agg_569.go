package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the ViewExpressionBatch aggregate-trigger and
// named-window-delete quartet: ords 7/8/9/10 of the pinned
// ViewExpressionBatch.java executions() collection — two statement-level
// aggregate #expr_batch batch-flush executions and two three-statement
// named-window modules with #expr_batch retention plus an on-delete
// trigger.
//
// Covered executions:
//   - ViewExpressionBatch ord 7 ViewExpressionBatchAggregationUngrouped
//     java-runtime-c6280a36141e2f9ac8c7 (case aggregation-ungrouped)
//   - ViewExpressionBatch ord 8 ViewExpressionBatchAggregationWGroupwin
//     java-runtime-6301447e4e4eaa27af22 (case aggregation-groupwin)
//   - ViewExpressionBatch ord 9 ViewExpressionBatchAggregationOnDelete
//     java-runtime-a8e7fc62db8793496c86 (case aggregation-on-delete)
//   - ViewExpressionBatch ord 10 ViewExpressionBatchNamedWindowDelete
//     java-runtime-36fb2be2ea2ba4270df5 (case named-window-delete)
//
// aggregation-ungrouped accumulates rows silently until
// sum(intPrimitive) > 100, then flushes the WHOLE accumulated batch as new
// data and the prior delivered batch as old data: E1/1 and E2/90 post
// nothing, E3/10 flushes new {E1,E2,E3}, E4/101 flushes new {E4}/old
// {E1,E2,E3}, E5/1 and E6/99 stay silent and E7/1 flushes new
// {E5,E6,E7}/old {E4}. aggregation-groupwin partitions by intPrimitive
// and runs an independent batch per group under
// sum(longPrimitive) > 100: only the triggering group flushes (E6 posts
// new {E2,E4,E5,E6}, E8 new {E1,E3,E8}, E10 new {E10}/old {E1,E3,E8}, E11
// new {E7,E9,E11}/old {E2,E4,E5,E6}, E12 new {E12}/old {E10}).
// aggregation-on-delete deploys a three-statement module — @name('s0')
// create window NW#expr_batch(sum(intPrimitive) >= 10) as SupportBean, a
// wildcard insert-into and an on-SupportBean_A delete (theString = id) —
// where the accumulating batch is silent, the A(E2) delete removes the
// pending row without delivery and the E4/1 insert flushes new
// {E1,E3,E4}. named-window-delete swaps in
// NW#expr_batch(current_count > 3) retention: E1-E3 accumulate under two
// iterator pins {E1,E2,E3} and {E1,E3}, E4 still stays silent and E5
// flushes new {E1,E3,E4,E5} as the pending count reaches four.
//
// The Java regression runs milestone() calls as regression-harness
// savepoints only; they restore identical state for these non-contextual
// executions, so the scenario omits them (they pin no observable).
// Snapshot records sit where Java asserts the iterator: named-window-
// delete asserts {E1,E2,E3} and {E1,E3} (both ordered) while the other
// three cases pin assertPropsPerRowIRPair / assertListenerNotInvoked
// only, so they record listener deliveries without snapshots.
const viewExprBatchAgg569ID = "view-expression-batch-agg-569"

const viewExprBatchAgg569Description = "ViewExpressionBatch ords 7/8/9/10 — the aggregate-trigger and named-window-delete quartet. aggregation-ungrouped (ViewExpressionBatchAggregationUngrouped, ord 7) replays `@name('s0') select irstream theString from SupportBean#expr_batch(sum(intPrimitive) > 100)`: E1/1 and E2/90 accumulate silently, E3/10 flushes the whole batch as new {E1,E2,E3}, E4/101 flushes new {E4} with old {E1,E2,E3}, E5/1 and E6/99 stay silent and E7/1 flushes new {E5,E6,E7} with old {E4}. aggregation-groupwin (ord 8) replays `@name('s0') select irstream theString from SupportBean#groupwin(intPrimitive)#expr_batch(sum(longPrimitive) > 100)`: each intPrimitive group accumulates an independent batch and only the triggering group flushes — E6 posts new {E2,E4,E5,E6}, E8 new {E1,E3,E8}, E10 new {E10}/old {E1,E3,E8}, E11 new {E7,E9,E11}/old {E2,E4,E5,E6} and E12 new {E12}/old {E10}. aggregation-on-delete (ord 9) and named-window-delete (ord 10) replay their three-statement modules `@name('s0') create window NW#expr_batch(sum(intPrimitive) >= 10)` / `NW#expr_batch(current_count > 3) as SupportBean` plus a wildcard insert-into and an on-SupportBean_A delete (theString = id): ord 9 accumulates E1/1 and E2/8 silently, the A(E2) delete removes the pending row without delivery and E4/1 flushes new {E1,E3,E4} (sum = 10); ord 10 pins ordered iterator snapshots {E1,E2,E3} and {E1,E3}, E4 stays silent and E5 flushes new {E1,E3,E4,E5} as the pending count passes three. Java milestone() calls are regression-harness savepoints with identical restored state, so the scenario omits them."

const viewExprBatchAgg569JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const viewExprBatchAgg569JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionBatch.java"

// Byte-exact EPL pins (ViewExpressionBatch.java line 392 for the ungrouped
// aggregation, line 428 for the groupwin variant, lines 465-467 for the
// aggregation-on-delete module and lines 326-328 for the named-window-
// delete module; the three-statement module text is pinned verbatim
// including its trailing statement separators).
const (
	viewExprBatchAgg569EPLAggUngrouped = "@name('s0') select irstream theString from SupportBean#expr_batch(sum(intPrimitive) > 100)"
	viewExprBatchAgg569EPLAggGroupwin  = "@name('s0') select irstream theString from SupportBean#groupwin(intPrimitive)#expr_batch(sum(longPrimitive) > 100)"

	viewExprBatchAgg569EPLNWCreateAgg   = "@name('s0') create window NW#expr_batch(sum(intPrimitive) >= 10) as SupportBean"
	viewExprBatchAgg569EPLNWCreateCount = "@name('s0') create window NW#expr_batch(current_count > 3) as SupportBean"
	viewExprBatchAgg569EPLNWInsert      = "insert into NW select * from SupportBean"
	viewExprBatchAgg569EPLNWDelete      = "on SupportBean_A delete from NW where theString = id"

	viewExprBatchAgg569SupportBeanEvent  = "SupportBean"
	viewExprBatchAgg569SupportBeanAEvent = "SupportBean_A"
)

// viewExprBatchAgg569ModuleEPL joins one named-window module's statements
// in the byte-exact shape the Java executions pass to compileDeploy: each
// statement terminated by `;` plus a newline.
func viewExprBatchAgg569ModuleEPL(createEPL string) string {
	return createEPL + ";\n" + viewExprBatchAgg569EPLNWInsert + ";\n" + viewExprBatchAgg569EPLNWDelete + ";\n"
}

var (
	viewExprBatchAgg569JavaRuntimeIDs = []string{
		"java-runtime-c6280a36141e2f9ac8c7",
		"java-runtime-6301447e4e4eaa27af22",
		"java-runtime-a8e7fc62db8793496c86",
		"java-runtime-36fb2be2ea2ba4270df5",
	}
	viewExprBatchAgg569JavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionBatch.java",
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
	}
	viewExprBatchAgg569JavaExecutions = []string{
		"ViewExpressionBatchAggregationUngrouped",
		"ViewExpressionBatchAggregationWGroupwin",
		"ViewExpressionBatchAggregationOnDelete",
		"ViewExpressionBatchNamedWindowDelete",
	}
	// Deduplicated inventory id: all four runtime rows share static id
	// java-20551a17cb2af08c67fc, pinned once per runtimeId row.
	viewExprBatchAgg569JavaStaticIDs = []string{
		"java-20551a17cb2af08c67fc",
		"java-20551a17cb2af08c67fc",
		"java-20551a17cb2af08c67fc",
		"java-20551a17cb2af08c67fc",
	}
	viewExprBatchAgg569JavaFlags = []string{}
)

// viewExprBatchAgg569Bean mirrors the SupportBean properties the
// executions use: theString (the iterator/listener field), intPrimitive
// (the sum term and the groupwin key) and longPrimitive (the groupwin sum
// term).
type viewExprBatchAgg569Bean struct {
	TheString     string `json:"theString" esper:"theString"`
	IntPrimitive  int    `json:"intPrimitive" esper:"intPrimitive"`
	LongPrimitive int64  `json:"longPrimitive,omitempty" esper:"longPrimitive"`
}

// viewExprBatchAgg569BeanA mirrors SupportBean_A: the delete trigger's id.
type viewExprBatchAgg569BeanA struct {
	ID string `json:"id" esper:"id"`
}

// viewExprBatchAgg569DeploySpec binds one scenario deploy label to its
// byte-exact statement text inside the case's module.
type viewExprBatchAgg569DeploySpec struct {
	label string
	epl   string
}

// viewExprBatchAgg569CaseSpec pins one Java execution: case identity,
// observation text, byte-exact EPL, the deploy-label sequence and the
// pinned listener/snapshot field projection.
type viewExprBatchAgg569CaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
	deploys     []viewExprBatchAgg569DeploySpec
	fields      []string
}

var viewExprBatchAgg569CaseSpecs = []viewExprBatchAgg569CaseSpec{
	{
		name:      "aggregation-ungrouped",
		ordinal:   7,
		runtimeID: "java-runtime-c6280a36141e2f9ac8c7",
		execution: "ViewExpressionBatchAggregationUngrouped",
		observation: "deployed+listener; ungrouped sum(intPrimitive) > 100 batch trigger: E1/1 " +
			"and E2/90 accumulate silently, E3/10 flushes the whole accumulated batch as " +
			"new {E1,E2,E3}, E4/101 flushes new {E4} with the prior batch old {E1,E2,E3}, " +
			"E5/1 and E6/99 stay silent and E7/1 flushes new {E5,E6,E7} with old {E4}; " +
			"the Java execution pins flattened IR pairs only, so no snapshots are recorded",
		epl: viewExprBatchAgg569EPLAggUngrouped,
		deploys: []viewExprBatchAgg569DeploySpec{
			{label: "s0", epl: viewExprBatchAgg569EPLAggUngrouped},
		},
		fields: []string{"theString"},
	},
	{
		name:      "aggregation-groupwin",
		ordinal:   8,
		runtimeID: "java-runtime-6301447e4e4eaa27af22",
		execution: "ViewExpressionBatchAggregationWGroupwin",
		observation: "deployed+listener; #groupwin(intPrimitive)#expr_batch(" +
			"sum(longPrimitive) > 100) accumulates an independent batch per group and " +
			"only the triggering group flushes: E6 posts new {E2,E4,E5,E6}, E8 new " +
			"{E1,E3,E8}, E10 new {E10} with old {E1,E3,E8}, E11 new {E7,E9,E11} with " +
			"old {E2,E4,E5,E6} and E12 new {E12} with old {E10}; the Java execution " +
			"pins flattened IR pairs only, so no snapshots are recorded",
		epl: viewExprBatchAgg569EPLAggGroupwin,
		deploys: []viewExprBatchAgg569DeploySpec{
			{label: "s0", epl: viewExprBatchAgg569EPLAggGroupwin},
		},
		fields: []string{"theString"},
	},
	{
		name:      "aggregation-on-delete",
		ordinal:   9,
		runtimeID: "java-runtime-a8e7fc62db8793496c86",
		execution: "ViewExpressionBatchAggregationOnDelete",
		observation: "deployed+listener; one three-statement module: @name('s0') create " +
			"window NW#expr_batch(sum(intPrimitive) >= 10) as SupportBean, a wildcard " +
			"insert-into and an on-SupportBean_A delete (theString = id); E1/1 and E2/8 " +
			"accumulate silently, the A(E2) delete removes the pending row without " +
			"delivery, E3/8 still stays silent and E4/1 flushes new {E1,E3,E4} " +
			"(sum(intPrimitive) = 10 reaches >= 10); the Java execution pins flattened " +
			"IR pairs only, so no snapshots are recorded",
		epl: viewExprBatchAgg569ModuleEPL(viewExprBatchAgg569EPLNWCreateAgg),
		deploys: []viewExprBatchAgg569DeploySpec{
			{label: "s0", epl: viewExprBatchAgg569EPLNWCreateAgg},
			{label: "insert", epl: viewExprBatchAgg569EPLNWInsert},
			{label: "delete", epl: viewExprBatchAgg569EPLNWDelete},
		},
		fields: []string{"theString"},
	},
	{
		name:      "named-window-delete",
		ordinal:   10,
		runtimeID: "java-runtime-36fb2be2ea2ba4270df5",
		execution: "ViewExpressionBatchNamedWindowDelete",
		observation: "deployed+listener+snapshot; the named-window-delete module with " +
			"NW#expr_batch(current_count > 3) retention: E1-E3 accumulate and the " +
			"ordered iterator pins {E1,E2,E3}, the A(E2) delete leaves {E1,E3} without " +
			"a listener delivery, E4/4 keeps the batch at three and E5/5 flushes new " +
			"{E1,E3,E4,E5} as the pending count passes three",
		epl: viewExprBatchAgg569ModuleEPL(viewExprBatchAgg569EPLNWCreateCount),
		deploys: []viewExprBatchAgg569DeploySpec{
			{label: "s0", epl: viewExprBatchAgg569EPLNWCreateCount},
			{label: "insert", epl: viewExprBatchAgg569EPLNWInsert},
			{label: "delete", epl: viewExprBatchAgg569EPLNWDelete},
		},
		fields: []string{"theString"},
	},
}

func viewExprBatchAgg569CaseSpecFor(name string) (viewExprBatchAgg569CaseSpec, bool) {
	for _, spec := range viewExprBatchAgg569CaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return viewExprBatchAgg569CaseSpec{}, false
}

// viewExprBatchAgg569StepPin pins one scenario step's shape.
type viewExprBatchAgg569StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	mode      string
}

func viewExprBatchAgg569DeployPin(statement, epl string) viewExprBatchAgg569StepPin {
	return viewExprBatchAgg569StepPin{op: "deploy", statement: statement, epl: epl}
}

func viewExprBatchAgg569DeployedPin(statement string) viewExprBatchAgg569StepPin {
	return viewExprBatchAgg569StepPin{op: "deployed", statement: statement}
}

func viewExprBatchAgg569SendPin(theString string, intPrimitive int) viewExprBatchAgg569StepPin {
	return viewExprBatchAgg569StepPin{
		op:        "send",
		eventType: viewExprBatchAgg569SupportBeanEvent,
		payload:   map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)},
	}
}

func viewExprBatchAgg569SendLongPin(theString string, intPrimitive int, longPrimitive int64) viewExprBatchAgg569StepPin {
	return viewExprBatchAgg569StepPin{
		op:        "send",
		eventType: viewExprBatchAgg569SupportBeanEvent,
		payload: map[string]any{
			"theString":     theString,
			"intPrimitive":  float64(intPrimitive),
			"longPrimitive": float64(longPrimitive),
		},
	}
}

func viewExprBatchAgg569SendAPin(id string) viewExprBatchAgg569StepPin {
	return viewExprBatchAgg569StepPin{
		op:        "send",
		eventType: viewExprBatchAgg569SupportBeanAEvent,
		payload:   map[string]any{"id": id},
	}
}

func viewExprBatchAgg569SnapshotPin(statement, mode string) viewExprBatchAgg569StepPin {
	return viewExprBatchAgg569StepPin{op: "snapshot", statement: statement, mode: mode}
}

func viewExprBatchAgg569UndeployAllPin() viewExprBatchAgg569StepPin {
	return viewExprBatchAgg569StepPin{op: "undeploy-all"}
}

// viewExprBatchAgg569ModulePins builds the pinned step sequence for one
// three-statement named-window module deploy: three contiguous deploy
// steps (the oracle batches them into one module compileDeploy) followed
// by their deployed markers in statement order.
func viewExprBatchAgg569ModulePins(createEPL string) []viewExprBatchAgg569StepPin {
	return []viewExprBatchAgg569StepPin{
		viewExprBatchAgg569DeployPin("s0", createEPL),
		viewExprBatchAgg569DeployPin("insert", viewExprBatchAgg569EPLNWInsert),
		viewExprBatchAgg569DeployPin("delete", viewExprBatchAgg569EPLNWDelete),
		viewExprBatchAgg569DeployedPin("s0"),
		viewExprBatchAgg569DeployedPin("insert"),
		viewExprBatchAgg569DeployedPin("delete"),
	}
}

// viewExprBatchAgg569CaseSteps pins the complete step sequence per case in
// Java source order. Milestones are omitted (regression-harness savepoints
// with identical restored state); snapshots sit where Java asserts the
// iterator.
var viewExprBatchAgg569CaseSteps = map[string][]viewExprBatchAgg569StepPin{
	"aggregation-ungrouped": {
		viewExprBatchAgg569DeployPin("s0", viewExprBatchAgg569EPLAggUngrouped),
		viewExprBatchAgg569DeployedPin("s0"),
		viewExprBatchAgg569SendPin("E1", 1),
		viewExprBatchAgg569SendPin("E2", 90),
		viewExprBatchAgg569SendPin("E3", 10),
		viewExprBatchAgg569SendPin("E4", 101),
		viewExprBatchAgg569SendPin("E5", 1),
		viewExprBatchAgg569SendPin("E6", 99),
		viewExprBatchAgg569SendPin("E7", 1),
		viewExprBatchAgg569UndeployAllPin(),
	},
	"aggregation-groupwin": {
		viewExprBatchAgg569DeployPin("s0", viewExprBatchAgg569EPLAggGroupwin),
		viewExprBatchAgg569DeployedPin("s0"),
		viewExprBatchAgg569SendLongPin("E1", 1, 10),
		viewExprBatchAgg569SendLongPin("E2", 2, 10),
		viewExprBatchAgg569SendLongPin("E3", 1, 90),
		viewExprBatchAgg569SendLongPin("E4", 2, 80),
		viewExprBatchAgg569SendLongPin("E5", 2, 10),
		viewExprBatchAgg569SendLongPin("E6", 2, 1),
		viewExprBatchAgg569SendLongPin("E7", 2, 50),
		viewExprBatchAgg569SendLongPin("E8", 1, 2),
		viewExprBatchAgg569SendLongPin("E9", 2, 50),
		viewExprBatchAgg569SendLongPin("E10", 1, 101),
		viewExprBatchAgg569SendLongPin("E11", 2, 1),
		viewExprBatchAgg569SendLongPin("E12", 1, 102),
		viewExprBatchAgg569UndeployAllPin(),
	},
	"aggregation-on-delete": func() []viewExprBatchAgg569StepPin {
		steps := viewExprBatchAgg569ModulePins(viewExprBatchAgg569EPLNWCreateAgg)
		return append(steps,
			viewExprBatchAgg569SendPin("E1", 1),
			viewExprBatchAgg569SendPin("E2", 8),
			viewExprBatchAgg569SendAPin("E2"),
			viewExprBatchAgg569SendPin("E3", 8),
			viewExprBatchAgg569SendPin("E4", 1),
			viewExprBatchAgg569UndeployAllPin(),
		)
	}(),
	"named-window-delete": func() []viewExprBatchAgg569StepPin {
		steps := viewExprBatchAgg569ModulePins(viewExprBatchAgg569EPLNWCreateCount)
		return append(steps,
			viewExprBatchAgg569SendPin("E1", 1),
			viewExprBatchAgg569SendPin("E2", 2),
			viewExprBatchAgg569SendPin("E3", 3),
			viewExprBatchAgg569SnapshotPin("s0", "ordered"),
			viewExprBatchAgg569SendAPin("E2"),
			viewExprBatchAgg569SnapshotPin("s0", "ordered"),
			viewExprBatchAgg569SendPin("E4", 4),
			viewExprBatchAgg569SendPin("E5", 5),
			viewExprBatchAgg569UndeployAllPin(),
		)
	}(),
}

func loadViewExprBatchAgg569Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", viewExprBatchAgg569ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", viewExprBatchAgg569ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprBatchAgg569ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprBatchAgg569ID, err)
	}
	if err := requireViewExprBatchAgg569Fields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaSourceFiles",
		"javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version        string   `json:"version"`
		ID             string   `json:"id"`
		Description    string   `json:"description"`
		JavaCommit     string   `json:"javaCommit"`
		JavaSource     string   `json:"javaSource"`
		JavaSourceFile []string `json:"javaSourceFiles"`
		JavaRuntimes   []string `json:"javaRuntimes"`
		JavaNames      []string `json:"javaNames"`
		JavaStaticIDs  []string `json:"javaStaticIds"`
		JavaFlags      []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", viewExprBatchAgg569ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != viewExprBatchAgg569ID ||
		metadata.Description != viewExprBatchAgg569Description ||
		metadata.JavaCommit != viewExprBatchAgg569JavaCommit ||
		metadata.JavaSource != viewExprBatchAgg569JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", viewExprBatchAgg569ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, viewExprBatchAgg569JavaSources},
		{metadata.JavaRuntimes, viewExprBatchAgg569JavaRuntimeIDs},
		{metadata.JavaNames, viewExprBatchAgg569JavaExecutions},
		{metadata.JavaStaticIDs, viewExprBatchAgg569JavaStaticIDs},
		{metadata.JavaFlags, viewExprBatchAgg569JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", viewExprBatchAgg569ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(viewExprBatchAgg569CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			viewExprBatchAgg569ID, len(viewExprBatchAgg569CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireViewExprBatchAgg569Fields(object, "case", "ordinal", "runtimeId",
			"executionName", "observation", "epl", "deploys"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string   `json:"case"`
			Ordinal       int      `json:"ordinal"`
			RuntimeID     string   `json:"runtimeId"`
			ExecutionName string   `json:"executionName"`
			Observation   string   `json:"observation"`
			EPL           string   `json:"epl"`
			Deploys       []string `json:"deploys"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := viewExprBatchAgg569CaseSpecs[index]
		labels := make([]string, 0, len(spec.deploys))
		for _, pin := range spec.deploys {
			labels = append(labels, pin.label)
		}
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID ||
			definition.ExecutionName != spec.execution ||
			definition.Observation != spec.observation || definition.EPL != spec.epl ||
			!reflect.DeepEqual(definition.Deploys, labels) {
			return compat.Scenario{}, fmt.Errorf("scenario case %d metadata is not pinned", index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", viewExprBatchAgg569ID)
	}
	if err := validateViewExprBatchAgg569RawSteps(rawSteps); err != nil {
		return compat.Scenario{}, err
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateViewExprBatchAgg569RawSteps pins the complete step sequence per
// case: op field whitelists per step kind plus positional comparison
// against the pinned sequence.
func validateViewExprBatchAgg569RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(viewExprBatchAgg569CaseSpecs))
	positions := make(map[string]int)
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return fmt.Errorf("scenario step %d op must be a string: %w", index, err)
		}
		var step struct {
			Case      string          `json:"case"`
			Statement string          `json:"statement"`
			EPL       string          `json:"epl"`
			EventType string          `json:"eventType"`
			Payload   json.RawMessage `json:"payload"`
			Mode      string          `json:"mode"`
		}
		if operation == "case" {
			if err := requireViewExprBatchAgg569Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := viewExprBatchAgg569CaseSpecFor(marker.Case); !ok {
				return fmt.Errorf("scenario step %d selects unknown case %q", index, marker.Case)
			}
			currentCase = marker.Case
			caseOrder = append(caseOrder, marker.Case)
			continue
		}
		if currentCase == "" {
			return fmt.Errorf("scenario step %d is outside any case block", index)
		}
		if err := json.Unmarshal(rawStep, &step); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		if step.Case != currentCase {
			return fmt.Errorf("scenario step %d declares case %q inside the %q block",
				index, step.Case, currentCase)
		}
		pins := viewExprBatchAgg569CaseSteps[currentCase]
		position := positions[currentCase]
		if position >= len(pins) {
			return fmt.Errorf("scenario step %d exceeds the pinned %s step sequence",
				index, currentCase)
		}
		pin := pins[position]
		if operation != pin.op {
			return fmt.Errorf("scenario step %d op %q is not the pinned %q for case %q position %d",
				index, operation, pin.op, currentCase, position)
		}
		switch operation {
		case "deploy":
			if err := requireViewExprBatchAgg569Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "deployed":
			if err := requireViewExprBatchAgg569Fields(object, "op", "case", "statement"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deployed marker is not pinned for case %q",
					index, currentCase)
			}
		case "send":
			if err := requireViewExprBatchAgg569Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "snapshot":
			if err := requireViewExprBatchAgg569Fields(object, "op", "case", "statement", "mode"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.Mode != pin.mode {
				return fmt.Errorf("scenario step %d snapshot is not pinned for case %q",
					index, currentCase)
			}
		case "undeploy-all":
			if err := requireViewExprBatchAgg569Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(viewExprBatchAgg569CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(viewExprBatchAgg569CaseSpecs))
	}
	for index, spec := range viewExprBatchAgg569CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(viewExprBatchAgg569CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(viewExprBatchAgg569CaseSteps[spec.name]))
		}
	}
	return nil
}

func requireViewExprBatchAgg569Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has %d fields, want %d", len(object), len(names))
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

// viewExprBatchAgg569CaseState carries per-case replay state: the
// deployed statements (label -> statement), deployments for undeploy-all,
// the listener sequence counters and the delivery records.
type viewExprBatchAgg569CaseState struct {
	spec        viewExprBatchAgg569CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	now         time.Time
	statements  map[string]*esper.Statement
	deployments []*esper.Deployment
	sequence    map[string]uint64
	records     []compat.TraceRecord
}

func viewExprBatchAgg569StartEnvironment() (*esper.Environment, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewExprBatchAgg569Bean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[viewExprBatchAgg569BeanA](env, "SupportBean_A"); err != nil {
		return nil, err
	}
	return env, nil
}

// runViewExprBatchAgg569Scenario replays all four executions, one fresh
// engine per case like the Java oracle's per-execution runtime.
func runViewExprBatchAgg569Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateViewExprBatchAgg569Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range viewExprBatchAgg569CaseSpecs {
		records, err := runViewExprBatchAgg569Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", viewExprBatchAgg569ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validateViewExprBatchAgg569Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != viewExprBatchAgg569ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, viewExprBatchAgg569ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validateViewExprBatchAgg569RawSteps(rawSteps)
}

func runViewExprBatchAgg569Case(ctx context.Context, scenario compat.Scenario, spec viewExprBatchAgg569CaseSpec) ([]compat.TraceRecord, error) {
	env, err := viewExprBatchAgg569StartEnvironment()
	if err != nil {
		return nil, err
	}
	now := time.Unix(0, 0).UTC()
	state := &viewExprBatchAgg569CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(spec.runtimeID), esper.WithStartTime(now)),
		now:        now,
		statements: make(map[string]*esper.Statement),
		sequence:   make(map[string]uint64),
	}
	defer func() { _ = state.engine.Close(context.Background()) }()

	inCase := false
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			inCase = step.Case == spec.name
			continue
		}
		if !inCase {
			continue
		}
		switch step.Op {
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return nil, err
			}
		case "deployed":
			if _, ok := state.statements[step.Statement]; !ok {
				return nil, fmt.Errorf("%s: deployed marker for unknown statement %q",
					viewExprBatchAgg569ID, step.Statement)
			}
			state.sequence[step.Statement+":deployed"]++
			state.records = append(state.records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  state.sequence[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(state.now),
			})
		case "send":
			payload, err := viewExprBatchAgg569DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "snapshot":
			if err := state.snapshot(ctx, step); err != nil {
				return nil, err
			}
		case "undeploy-all":
			for _, deployment := range state.deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return nil, err
				}
			}
			state.deployments = nil
			state.statements = make(map[string]*esper.Statement)
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", viewExprBatchAgg569ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps each pinned deploy label onto the typed Go call or plan
// equivalent of the Java statement: s0 builds the case's irstream select
// (ords 7/8) or registers the NW named window plus its create-window
// statement (ords 9/10), while the insert/delete labels build the
// insert-into and on-delete trigger statements. The byte-exact EPL text
// of the Java compileDeploy call is pinned by the loader.
func (s *viewExprBatchAgg569CaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	if _, ok := s.statements[label]; ok {
		return fmt.Errorf("%s: label %q is already deployed", viewExprBatchAgg569ID, label)
	}
	pinned := false
	for _, pin := range s.spec.deploys {
		if pin.label == label {
			pinned = pin.epl == step.Epl
			break
		}
	}
	if !pinned {
		return fmt.Errorf("%s: case %q deploy %q is not pinned", viewExprBatchAgg569ID, s.spec.name, label)
	}
	switch s.spec.name {
	case "aggregation-ungrouped":
		// `select irstream theString from SupportBean#expr_batch(
		// sum(intPrimitive) > 100)`: accumulate rows silently until the
		// batch sum passes 100, then deliver the whole batch as new data
		// and the prior batch as old data; WithOldStream surfaces both
		// streams like Java's irstream.
		trigger := esper.Greater[int](
			esper.Sum[int](esper.Field[viewExprBatchAgg569Bean, int]("intPrimitive")),
			esper.Literal(100))
		return s.deployQuery(ctx, label, esper.Select(
			esper.From[viewExprBatchAgg569Bean](s.env, "SupportBean").Window(esper.ExpressionBatch(trigger)),
			esper.Alias("theString", esper.Field[viewExprBatchAgg569Bean, string]("theString")),
		).Query(esper.StatementName("s0"), esper.WithOldStream()), true)
	case "aggregation-groupwin":
		// `select irstream theString from
		// SupportBean#groupwin(intPrimitive)#expr_batch(
		// sum(longPrimitive) > 100)`: each intPrimitive group accumulates
		// an independent batch and only the group whose longPrimitive sum
		// passes 100 flushes.
		trigger := esper.Greater[int64](
			esper.Sum[int64](esper.Field[viewExprBatchAgg569Bean, int64]("longPrimitive")),
			esper.Literal(int64(100)))
		return s.deployQuery(ctx, label, esper.Select(
			esper.From[viewExprBatchAgg569Bean](s.env, "SupportBean").Window(esper.GroupWindow(
				esper.Field[viewExprBatchAgg569Bean, int]("intPrimitive"),
				esper.ExpressionBatch(trigger))),
			esper.Alias("theString", esper.Field[viewExprBatchAgg569Bean, string]("theString")),
		).Query(esper.StatementName("s0"), esper.WithOldStream()), true)
	case "aggregation-on-delete", "named-window-delete":
		switch label {
		case "s0":
			// `@name('s0') create window NW#expr_batch(...) as
			// SupportBean`: the s0 statement IS the create-window
			// statement — Java's addListener("s0") receives the window's
			// batch-flush deliveries and its iterator enumerates the
			// pending batch.
			schema, ok := s.env.Schema("SupportBean")
			if !ok {
				return fmt.Errorf("%s: SupportBean schema is not registered", viewExprBatchAgg569ID)
			}
			retention := esper.ExpressionBatch(esper.GreaterOrEqual[int](
				esper.Sum[int](esper.Field[viewExprBatchAgg569Bean, int]("intPrimitive")),
				esper.Literal(10)))
			if s.spec.name == "named-window-delete" {
				retention = esper.ExpressionBatch(esper.Greater[int64](
					esper.WindowCurrentCount(),
					esper.Literal(int64(3))))
			}
			if _, err := esper.CreateNamedWindow(s.env, "NW", schema,
				esper.NamedWindowRetention(retention)); err != nil {
				return err
			}
			if _, ok := s.engine.NamedWindow("NW"); !ok {
				return fmt.Errorf("%s: named window NW was not materialized", viewExprBatchAgg569ID)
			}
			return s.deployQuery(ctx, label,
				esper.FromNamedWindow(s.env, "NW").
					CreateNamedWindowQuery(esper.StatementName("s0"), esper.WithOldStream()), true)
		case "insert":
			// `insert into NW select * from SupportBean`.
			return s.deployQuery(ctx, label,
				esper.OnEvent(esper.From[viewExprBatchAgg569Bean](s.env, "SupportBean")).
					InsertIntoNamedWindow("NW", esper.CopyMatchingFields()).
					Query(esper.StatementName("insert")), false)
		case "delete":
			// `on SupportBean_A delete from NW where theString = id`.
			return s.deployQuery(ctx, label,
				esper.OnEvent(esper.From[viewExprBatchAgg569BeanA](s.env, "SupportBean_A")).
					DeleteFromNamedWindow("NW",
						esper.Equal[string](
							esper.NamedWindowField[string]("theString"),
							esper.Field[viewExprBatchAgg569BeanA, string]("id"))).
					Query(esper.StatementName("delete")), false)
		}
	}
	return fmt.Errorf("%s: unknown case %q deploy %q", viewExprBatchAgg569ID, s.spec.name, label)
}

// deployQuery builds and deploys one query, registering its single
// statement under the scenario label and attaching the delivery recorder
// when the label carries Java's addListener.
func (s *viewExprBatchAgg569CaseState) deployQuery(ctx context.Context, label string, query esper.Query, listen bool) error {
	plan, err := s.env.Build(query)
	if err != nil {
		return fmt.Errorf("%s: build %q: %w", viewExprBatchAgg569ID, label, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", viewExprBatchAgg569ID, label, err)
	}
	s.deployments = append(s.deployments, deployment)
	statements := deployment.Statements()
	if len(statements) != 1 {
		return fmt.Errorf("%s: deploy %q produced %d statements", viewExprBatchAgg569ID, label, len(statements))
	}
	statement := statements[0]
	s.statements[label] = statement
	if !listen {
		return nil
	}
	_, err = statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		s.sequence[label]++
		s.records = append(s.records, compat.TraceRecord{
			Case:      s.spec.name,
			Operation: "listener",
			Statement: label,
			Sequence:  s.sequence[label],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       projectRecords(compat.NormalizeResults(batch.New), s.spec.fields),
			Old:       projectRecords(compat.NormalizeResults(batch.Old), s.spec.fields),
		})
		return nil
	})
	return err
}

// snapshot emits one iterator record for the statement, projecting the
// pinned field set. All pinned snapshots are mode "ordered"; the "any"
// branch mirrors the sorter for completeness with the shared convention.
func (s *viewExprBatchAgg569CaseState) snapshot(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot statement %q was not deployed",
			viewExprBatchAgg569ID, step.Statement)
	}
	result, err := statement.Snapshot(ctx)
	if err != nil {
		return err
	}
	rows := projectRecords(compat.NormalizeResults(result.Batch.New), s.spec.fields)
	if step.Mode == "any" {
		sortRowsCanonical(rows)
	}
	record := compat.TraceRecord{
		Case:      s.spec.name,
		Operation: "snapshot",
		Statement: step.Statement,
		Sequence:  0,
		Time:      compat.FormatTraceTime(s.now),
	}
	if len(rows) > 0 {
		record.New = rows
	}
	s.records = append(s.records, record)
	return nil
}

// viewExprBatchAgg569DecodePayload converts a scenario send payload into
// the typed host object: a SupportBean struct with the pinned theString,
// intPrimitive and optional longPrimitive fields, or a SupportBean_A
// struct for the delete triggers.
func viewExprBatchAgg569DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case viewExprBatchAgg569SupportBeanEvent:
		base := []string{"theString", "intPrimitive"}
		if _, ok := fields["longPrimitive"]; ok {
			base = append(base, "longPrimitive")
		}
		if err := requireViewExprBatchAgg569Fields(fields, base...); err != nil {
			return nil, err
		}
		var bean viewExprBatchAgg569Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case viewExprBatchAgg569SupportBeanAEvent:
		if err := requireViewExprBatchAgg569Fields(fields, "id"); err != nil {
			return nil, err
		}
		var bean viewExprBatchAgg569BeanA
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", viewExprBatchAgg569ID, step.EventType)
	}
}
