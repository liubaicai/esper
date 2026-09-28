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

// Parity coverage for the ViewExpressionBatch non-aggregate-trigger
// quartet: ords 0/1/2/6 of the pinned ViewExpressionBatch.java
// executions() collection — four statement-level #expr_batch executions
// whose trigger predicate is evaluated over the accumulating batch and
// whose flush delivers the whole batch as new data plus the prior
// delivered batch as old data.
//
// Covered executions:
//   - ViewExpressionBatch ord 0 ViewExpressionBatchNewestEventOldestEvent
//     java-runtime-a4012797b6db56fc35ed (case newest-oldest)
//   - ViewExpressionBatch ord 1 ViewExpressionBatchLengthBatch
//     java-runtime-bc9403566ad19ca73ed9 (case length-batch)
//   - ViewExpressionBatch ord 2 ViewExpressionBatchTimeBatch
//     java-runtime-d68cff5ac536057a77b8 (case time-batch)
//   - ViewExpressionBatch ord 6 ViewExpressionBatchEventPropBatch
//     java-runtime-cfb5c3a23c3ab8bf2029 (case event-prop-batch)
//
// newest-oldest runs TWO deployments of the same trigger
// newest_event.intPrimitive != oldest_event.intPrimitive — first with
// the explicit exclude-trigger-event flag false, then after undeployAll
// with the explicit include flag true. Excluding the trigger event the
// flushed batch is the accumulation BEFORE the triggering send: E1/1
// accumulates, E2/1 stays silent, E3/2 flushes new {E1,E2}, E4/3 flushes
// new {E3}/old {E1,E2}, E5/3 and E6/3 stay silent and E7/2 flushes new
// {E4,E5,E6}/old {E3}. Including the trigger event the flushed batch
// carries the triggering send: E1/1 and E2/1 stay silent, E3/2 flushes
// new {E1,E2,E3}, E4/3, E5/3 and E6/3 stay silent and E7/2 flushes new
// {E4,E5,E6,E7}/old {E1,E2,E3}. length-batch pins the
// current_count >= 3 trigger with the explicit include flag: E1 and E2
// stay silent, then each third send flushes the triple — E3 posts
// new {E1,E2,E3}, E6 posts new {E4,E5,E6}/old {E1,E2,E3} and E9 posts
// new {E7,E8,E9}/old {E4,E5,E6}. time-batch drives the
// newest_timestamp - oldest_timestamp > 2000 trigger with virtual-clock
// sends: a clock advance alone never evaluates the trigger (the
// t=3100 and t=5101 advances deliver nothing) and only an arriving
// event can close the batch — E5@3100 flushes new {E1,E2,E3,E4,E5} and
// E8@5101 flushes new {E6,E7,E8}/old {E1,E2,E3,E4,E5}.
// event-prop-batch pins the per-event intPrimitive > 0 trigger over a
// theString as val0 projection: E1/1 flushes new {E1}, E2/1 flushes
// new {E2}/old {E1}, E3/-1 stays silent but is RETAINED in the pending
// batch, and E4/2 flushes new {E3,E4}/old {E2}.
//
// The Java regression runs milestone() calls as regression-harness
// savepoints only; they restore identical state for these non-contextual
// executions, so the scenario omits them (they pin no observable). All
// four cases pin listener deliveries only: the Java executions assert
// assertPropsPerRowIRPair / assertPropsPerRowLastNew /
// assertListenerNotInvoked and never the iterator, so no snapshots are
// recorded.
const viewExprBatchCore570ID = "view-expression-batch-core-570"

const viewExprBatchCore570Description = "ViewExpressionBatch ords 0/1/2/6 — the non-aggregate-trigger quartet. newest-oldest (ViewExpressionBatchNewestEventOldestEvent, ord 0) runs TWO deployments of `@name('s0') select irstream * from SupportBean#expr_batch(newest_event.intPrimitive != oldest_event.intPrimitive, ...)`: the exclude-trigger-event (`false`) phase accumulates E1/1, keeps E2/1 silent, flushes new {E1,E2} at E3/2 and new {E3}/old {E1,E2} at E4/3, keeps E5/3+E6/3 silent and flushes new {E4,E5,E6}/old {E3} at E7/2; after undeployAll the include-trigger-event (`true`) phase keeps E1/1+E2/1 silent, flushes new {E1,E2,E3} at E3/2, keeps E4/3+E5/3+E6/3 silent and flushes new {E4,E5,E6,E7}/old {E1,E2,E3} at E7/2. length-batch (ord 1) replays `@name('s0') select irstream * from SupportBean#expr_batch(current_count >= 3, true)`: E1/1+E2/2 stay silent, E3/3 flushes new {E1,E2,E3}, E4/4+E5/5 stay silent, E6/6 flushes new {E4,E5,E6}/old {E1,E2,E3}, E7/7+E8/8 stay silent and E9/9 flushes new {E7,E8,E9}/old {E4,E5,E6}. time-batch (ord 2) replays `@name('s0') select irstream * from SupportBean#expr_batch(newest_timestamp - oldest_timestamp > 2000)` under a virtual clock starting at advanceTime(0): sends land at E1@1000, E2@1500, E3@1500, E4@3000, the lone advance to t=3100 flushes nothing, E5@3100 flushes new {E1,E2,E3,E4,E5}, E6@3100 starts the next batch, E7@5100 queues, the lone advance to t=5101 flushes nothing and E8@5101 flushes new {E6,E7,E8}/old {E1,E2,E3,E4,E5}; a clock advance alone never evaluates the trigger. event-prop-batch (ord 6) replays `@name('s0') select irstream theString as val0 from SupportBean#expr_batch(intPrimitive > 0)`: E1/1 flushes new {E1}, E2/1 flushes new {E2}/old {E1}, E3/-1 stays silent but is retained in the pending batch and E4/2 flushes new {E3,E4}/old {E2}. Java milestone() calls are regression-harness savepoints with identical restored state for these executions, so the scenario omits them."

const viewExprBatchCore570JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const viewExprBatchCore570JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionBatch.java"

// Byte-exact EPL pins (ViewExpressionBatch.java line 50 for the
// exclude-trigger-event deployment, line 87 for the include variant,
// line 124 for the length batch, line 168 for the time batch and
// line 369 for the event-property batch; the include/exclude literal is
// pinned verbatim).
const (
	viewExprBatchCore570EPLNewestOldestExclude = "@name('s0') select irstream * from SupportBean#expr_batch(newest_event.intPrimitive != oldest_event.intPrimitive, false)"
	viewExprBatchCore570EPLNewestOldestInclude = "@name('s0') select irstream * from SupportBean#expr_batch(newest_event.intPrimitive != oldest_event.intPrimitive, true)"
	viewExprBatchCore570EPLLengthBatch         = "@name('s0') select irstream * from SupportBean#expr_batch(current_count >= 3, true)"
	viewExprBatchCore570EPLTimeBatch           = "@name('s0') select irstream * from SupportBean#expr_batch(newest_timestamp - oldest_timestamp > 2000)"
	viewExprBatchCore570EPLEventProp           = "@name('s0') select irstream theString as val0 from SupportBean#expr_batch(intPrimitive > 0)"

	viewExprBatchCore570TimeEpoch        = "1970-01-01T00:00:00.000Z"
	viewExprBatchCore570Time1000         = "1970-01-01T00:00:01.000Z"
	viewExprBatchCore570Time1500         = "1970-01-01T00:00:01.500Z"
	viewExprBatchCore570Time3000         = "1970-01-01T00:00:03.000Z"
	viewExprBatchCore570Time3100         = "1970-01-01T00:00:03.100Z"
	viewExprBatchCore570Time5100         = "1970-01-01T00:00:05.100Z"
	viewExprBatchCore570Time5101         = "1970-01-01T00:00:05.101Z"
	viewExprBatchCore570SupportBeanEvent = "SupportBean"
)

var (
	viewExprBatchCore570JavaRuntimeIDs = []string{
		"java-runtime-a4012797b6db56fc35ed",
		"java-runtime-bc9403566ad19ca73ed9",
		"java-runtime-d68cff5ac536057a77b8",
		"java-runtime-cfb5c3a23c3ab8bf2029",
	}
	viewExprBatchCore570JavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionBatch.java",
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
	}
	viewExprBatchCore570JavaExecutions = []string{
		"ViewExpressionBatchNewestEventOldestEvent",
		"ViewExpressionBatchLengthBatch",
		"ViewExpressionBatchTimeBatch",
		"ViewExpressionBatchEventPropBatch",
	}
	// Deduplicated inventory id: all four runtime rows share static id
	// java-20551a17cb2af08c67fc, pinned once per runtimeId row.
	viewExprBatchCore570JavaStaticIDs = []string{
		"java-20551a17cb2af08c67fc",
		"java-20551a17cb2af08c67fc",
		"java-20551a17cb2af08c67fc",
		"java-20551a17cb2af08c67fc",
	}
	viewExprBatchCore570JavaFlags = []string{}
)

// viewExprBatchCore570Bean mirrors the SupportBean properties the
// executions use: theString (the listener field and the val0 projection
// source) and intPrimitive (the boundary-event trigger field and the
// event-property trigger term).
type viewExprBatchCore570Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// viewExprBatchCore570DeploySpec binds one scenario deploy label to its
// byte-exact statement text. newest-oldest deploys the s0 label twice —
// once per include-trigger-event phase — so deployments pin positionally.
type viewExprBatchCore570DeploySpec struct {
	label string
	epl   string
}

// viewExprBatchCore570CaseSpec pins one Java execution: case identity,
// observation text, the case-level EPL pin, the positional deploy-label
// sequence and the pinned listener field projection.
type viewExprBatchCore570CaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
	deploys     []viewExprBatchCore570DeploySpec
	fields      []string
}

var viewExprBatchCore570CaseSpecs = []viewExprBatchCore570CaseSpec{
	{
		name:      "newest-oldest",
		ordinal:   0,
		runtimeID: "java-runtime-a4012797b6db56fc35ed",
		execution: "ViewExpressionBatchNewestEventOldestEvent",
		observation: "deployed+listener; TWO deployments of " +
			"#expr_batch(newest_event.intPrimitive != oldest_event.intPrimitive): " +
			"the explicit exclude-trigger-event (`false`) phase flushes the batch " +
			"accumulated BEFORE the triggering send (E3/2 posts new {E1,E2}, E4/3 " +
			"posts new {E3}/old {E1,E2}, E7/2 posts new {E4,E5,E6}/old {E3}), then " +
			"undeployAll swaps in the explicit include-trigger-event (`true`) EPL " +
			"which folds the triggering send into the flushed batch (E3/2 posts new " +
			"{E1,E2,E3}, E7/2 posts new {E4,E5,E6,E7}/old {E1,E2,E3}); the Java " +
			"execution pins flattened IR pairs only, so no snapshots are recorded",
		epl: viewExprBatchCore570EPLNewestOldestExclude,
		deploys: []viewExprBatchCore570DeploySpec{
			{label: "s0", epl: viewExprBatchCore570EPLNewestOldestExclude},
			{label: "s0", epl: viewExprBatchCore570EPLNewestOldestInclude},
		},
		fields: []string{"theString"},
	},
	{
		name:      "length-batch",
		ordinal:   1,
		runtimeID: "java-runtime-bc9403566ad19ca73ed9",
		execution: "ViewExpressionBatchLengthBatch",
		observation: "deployed+listener; #expr_batch(current_count >= 3, true) " +
			"flushes every third send as one batch: E1/1 and E2/2 stay silent, E3/3 " +
			"posts new {E1,E2,E3}, E4/4 and E5/5 stay silent, E6/6 posts new " +
			"{E4,E5,E6}/old {E1,E2,E3}, E7/7 and E8/8 stay silent and E9/9 posts " +
			"new {E7,E8,E9}/old {E4,E5,E6}; the Java execution pins flattened IR " +
			"pairs only, so no snapshots are recorded",
		epl: viewExprBatchCore570EPLLengthBatch,
		deploys: []viewExprBatchCore570DeploySpec{
			{label: "s0", epl: viewExprBatchCore570EPLLengthBatch},
		},
		fields: []string{"theString"},
	},
	{
		name:      "time-batch",
		ordinal:   2,
		runtimeID: "java-runtime-d68cff5ac536057a77b8",
		execution: "ViewExpressionBatchTimeBatch",
		observation: "deployed+listener; advanceTime(0) then " +
			"#expr_batch(newest_timestamp - oldest_timestamp > 2000) under the " +
			"virtual clock: sends land at E1@1000, E2@1500, E3@1500, E4@3000, the " +
			"lone advance to t=3100 delivers nothing, E5@3100 posts new " +
			"{E1,E2,E3,E4,E5}, E6@3100 starts the next batch, E7@5100 queues, the " +
			"lone advance to t=5101 delivers nothing and E8@5101 posts new " +
			"{E6,E7,E8}/old {E1,E2,E3,E4,E5}; a clock advance alone never " +
			"evaluates the trigger",
		epl: viewExprBatchCore570EPLTimeBatch,
		deploys: []viewExprBatchCore570DeploySpec{
			{label: "s0", epl: viewExprBatchCore570EPLTimeBatch},
		},
		fields: []string{"theString"},
	},
	{
		name:      "event-prop-batch",
		ordinal:   6,
		runtimeID: "java-runtime-cfb5c3a23c3ab8bf2029",
		execution: "ViewExpressionBatchEventPropBatch",
		observation: "deployed+listener; #expr_batch(intPrimitive > 0) over a " +
			"theString as val0 projection: E1/1 posts new {E1}, E2/1 posts new " +
			"{E2}/old {E1}, E3/-1 stays silent but is RETAINED in the pending " +
			"batch and E4/2 posts new {E3,E4}/old {E2}; the Java execution pins " +
			"flattened IR pairs only, so no snapshots are recorded",
		epl: viewExprBatchCore570EPLEventProp,
		deploys: []viewExprBatchCore570DeploySpec{
			{label: "s0", epl: viewExprBatchCore570EPLEventProp},
		},
		fields: []string{"val0"},
	},
}

func viewExprBatchCore570CaseSpecFor(name string) (viewExprBatchCore570CaseSpec, bool) {
	for _, spec := range viewExprBatchCore570CaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return viewExprBatchCore570CaseSpec{}, false
}

// viewExprBatchCore570StepPin pins one scenario step's shape.
type viewExprBatchCore570StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	at        string
}

func viewExprBatchCore570DeployPin(statement, epl string) viewExprBatchCore570StepPin {
	return viewExprBatchCore570StepPin{op: "deploy", statement: statement, epl: epl}
}

func viewExprBatchCore570DeployedPin(statement string) viewExprBatchCore570StepPin {
	return viewExprBatchCore570StepPin{op: "deployed", statement: statement}
}

func viewExprBatchCore570SendPin(theString string, intPrimitive int) viewExprBatchCore570StepPin {
	return viewExprBatchCore570StepPin{
		op:        "send",
		eventType: viewExprBatchCore570SupportBeanEvent,
		payload:   map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)},
	}
}

func viewExprBatchCore570AdvancePin(at string) viewExprBatchCore570StepPin {
	return viewExprBatchCore570StepPin{op: "advance-time", at: at}
}

func viewExprBatchCore570UndeployAllPin() viewExprBatchCore570StepPin {
	return viewExprBatchCore570StepPin{op: "undeploy-all"}
}

// viewExprBatchCore570CaseSteps pins the complete step sequence per case in
// Java source order. Milestones are omitted (regression-harness savepoints
// with identical restored state); advance-time steps sit exactly where the
// Java execution calls env.advanceTime and the two assertListenerNotInvoked
// advances at t=3100/5101 pin that a clock advance alone never flushes.
var viewExprBatchCore570CaseSteps = map[string][]viewExprBatchCore570StepPin{
	"newest-oldest": {
		// Exclude-trigger-event phase: the triggering send starts the
		// next batch instead of closing the current one.
		viewExprBatchCore570DeployPin("s0", viewExprBatchCore570EPLNewestOldestExclude),
		viewExprBatchCore570DeployedPin("s0"),
		viewExprBatchCore570SendPin("E1", 1),
		viewExprBatchCore570SendPin("E2", 1),
		viewExprBatchCore570SendPin("E3", 2),
		viewExprBatchCore570SendPin("E4", 3),
		viewExprBatchCore570SendPin("E5", 3),
		viewExprBatchCore570SendPin("E6", 3),
		viewExprBatchCore570SendPin("E7", 2),
		viewExprBatchCore570UndeployAllPin(),
		// Include-trigger-event phase: the triggering send closes the
		// batch it fired on.
		viewExprBatchCore570DeployPin("s0", viewExprBatchCore570EPLNewestOldestInclude),
		viewExprBatchCore570DeployedPin("s0"),
		viewExprBatchCore570SendPin("E1", 1),
		viewExprBatchCore570SendPin("E2", 1),
		viewExprBatchCore570SendPin("E3", 2),
		viewExprBatchCore570SendPin("E4", 3),
		viewExprBatchCore570SendPin("E5", 3),
		viewExprBatchCore570SendPin("E6", 3),
		viewExprBatchCore570SendPin("E7", 2),
		viewExprBatchCore570UndeployAllPin(),
	},
	"length-batch": {
		viewExprBatchCore570DeployPin("s0", viewExprBatchCore570EPLLengthBatch),
		viewExprBatchCore570DeployedPin("s0"),
		viewExprBatchCore570SendPin("E1", 1),
		viewExprBatchCore570SendPin("E2", 2),
		viewExprBatchCore570SendPin("E3", 3),
		viewExprBatchCore570SendPin("E4", 4),
		viewExprBatchCore570SendPin("E5", 5),
		viewExprBatchCore570SendPin("E6", 6),
		viewExprBatchCore570SendPin("E7", 7),
		viewExprBatchCore570SendPin("E8", 8),
		viewExprBatchCore570SendPin("E9", 9),
		viewExprBatchCore570UndeployAllPin(),
	},
	"time-batch": {
		viewExprBatchCore570AdvancePin(viewExprBatchCore570TimeEpoch),
		viewExprBatchCore570DeployPin("s0", viewExprBatchCore570EPLTimeBatch),
		viewExprBatchCore570DeployedPin("s0"),
		viewExprBatchCore570AdvancePin(viewExprBatchCore570Time1000),
		viewExprBatchCore570SendPin("E1", 1),
		viewExprBatchCore570AdvancePin(viewExprBatchCore570Time1500),
		viewExprBatchCore570SendPin("E2", 2),
		viewExprBatchCore570SendPin("E3", 3),
		viewExprBatchCore570AdvancePin(viewExprBatchCore570Time3000),
		viewExprBatchCore570SendPin("E4", 4),
		viewExprBatchCore570AdvancePin(viewExprBatchCore570Time3100),
		viewExprBatchCore570SendPin("E5", 5),
		viewExprBatchCore570SendPin("E6", 6),
		viewExprBatchCore570AdvancePin(viewExprBatchCore570Time5100),
		viewExprBatchCore570SendPin("E7", 7),
		viewExprBatchCore570AdvancePin(viewExprBatchCore570Time5101),
		viewExprBatchCore570SendPin("E8", 8),
		viewExprBatchCore570UndeployAllPin(),
	},
	"event-prop-batch": {
		viewExprBatchCore570DeployPin("s0", viewExprBatchCore570EPLEventProp),
		viewExprBatchCore570DeployedPin("s0"),
		viewExprBatchCore570SendPin("E1", 1),
		viewExprBatchCore570SendPin("E2", 1),
		viewExprBatchCore570SendPin("E3", -1),
		viewExprBatchCore570SendPin("E4", 2),
		viewExprBatchCore570UndeployAllPin(),
	},
}

func loadViewExprBatchCore570Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", viewExprBatchCore570ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", viewExprBatchCore570ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprBatchCore570ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprBatchCore570ID, err)
	}
	if err := requireViewExprBatchCore570Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", viewExprBatchCore570ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != viewExprBatchCore570ID ||
		metadata.Description != viewExprBatchCore570Description ||
		metadata.JavaCommit != viewExprBatchCore570JavaCommit ||
		metadata.JavaSource != viewExprBatchCore570JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", viewExprBatchCore570ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, viewExprBatchCore570JavaSources},
		{metadata.JavaRuntimes, viewExprBatchCore570JavaRuntimeIDs},
		{metadata.JavaNames, viewExprBatchCore570JavaExecutions},
		{metadata.JavaStaticIDs, viewExprBatchCore570JavaStaticIDs},
		{metadata.JavaFlags, viewExprBatchCore570JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", viewExprBatchCore570ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(viewExprBatchCore570CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			viewExprBatchCore570ID, len(viewExprBatchCore570CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireViewExprBatchCore570Fields(object, "case", "ordinal", "runtimeId",
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
		spec := viewExprBatchCore570CaseSpecs[index]
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
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", viewExprBatchCore570ID)
	}
	if err := validateViewExprBatchCore570RawSteps(rawSteps); err != nil {
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

// validateViewExprBatchCore570RawSteps pins the complete step sequence per
// case: op field whitelists per step kind plus positional comparison
// against the pinned sequence.
func validateViewExprBatchCore570RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(viewExprBatchCore570CaseSpecs))
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
			At        string          `json:"at"`
		}
		if operation == "case" {
			if err := requireViewExprBatchCore570Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := viewExprBatchCore570CaseSpecFor(marker.Case); !ok {
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
		pins := viewExprBatchCore570CaseSteps[currentCase]
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
			if err := requireViewExprBatchCore570Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "deployed":
			if err := requireViewExprBatchCore570Fields(object, "op", "case", "statement"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deployed marker is not pinned for case %q",
					index, currentCase)
			}
		case "send":
			if err := requireViewExprBatchCore570Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "advance-time":
			if err := requireViewExprBatchCore570Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time is not pinned for case %q",
					index, currentCase)
			}
		case "undeploy-all":
			if err := requireViewExprBatchCore570Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(viewExprBatchCore570CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(viewExprBatchCore570CaseSpecs))
	}
	for index, spec := range viewExprBatchCore570CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(viewExprBatchCore570CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(viewExprBatchCore570CaseSteps[spec.name]))
		}
	}
	return nil
}

func requireViewExprBatchCore570Fields(object map[string]json.RawMessage, names ...string) error {
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

// viewExprBatchCore570CaseState carries per-case replay state: the
// deployed statements (label -> statement), deployments for undeploy-all,
// the positional deploy counter (newest-oldest redeploys s0 per phase),
// the listener sequence counters and the delivery records.
type viewExprBatchCore570CaseState struct {
	spec        viewExprBatchCore570CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	now         time.Time
	statements  map[string]*esper.Statement
	deployments []*esper.Deployment
	deployCount int
	sequence    map[string]uint64
	records     []compat.TraceRecord
}

func viewExprBatchCore570StartEnvironment() (*esper.Environment, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewExprBatchCore570Bean](env, "SupportBean"); err != nil {
		return nil, err
	}
	return env, nil
}

// runViewExprBatchCore570Scenario replays all four executions, one fresh
// engine per case like the Java oracle's per-execution runtime.
func runViewExprBatchCore570Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateViewExprBatchCore570Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range viewExprBatchCore570CaseSpecs {
		records, err := runViewExprBatchCore570Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", viewExprBatchCore570ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validateViewExprBatchCore570Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != viewExprBatchCore570ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, viewExprBatchCore570ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validateViewExprBatchCore570RawSteps(rawSteps)
}

func runViewExprBatchCore570Case(ctx context.Context, scenario compat.Scenario, spec viewExprBatchCore570CaseSpec) ([]compat.TraceRecord, error) {
	env, err := viewExprBatchCore570StartEnvironment()
	if err != nil {
		return nil, err
	}
	now := time.Unix(0, 0).UTC()
	state := &viewExprBatchCore570CaseState{
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
					viewExprBatchCore570ID, step.Statement)
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
			payload, err := viewExprBatchCore570DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse advance-time %q: %w", viewExprBatchCore570ID, step.At, err)
			}
			if err := state.engine.AdvanceTime(ctx, at); err != nil {
				return nil, err
			}
			state.now = at
		case "undeploy-all":
			for _, deployment := range state.deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return nil, err
				}
			}
			state.deployments = nil
			state.statements = make(map[string]*esper.Statement)
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", viewExprBatchCore570ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps each pinned deploy step onto the fluent Go equivalent of
// the Java statement. Deployments pin positionally because newest-oldest
// redeploys the s0 label with the include-trigger-event variant: the
// first deploy builds the exclude-trigger-event window, the second the
// include variant, and every other case deploys its single EPL. The
// byte-exact EPL text of the Java compileDeploy call is pinned by the
// loader.
func (s *viewExprBatchCore570CaseState) deploy(ctx context.Context, step compat.Step) error {
	if s.deployCount >= len(s.spec.deploys) {
		return fmt.Errorf("%s: case %q deploy %q exceeds the pinned deployments",
			viewExprBatchCore570ID, s.spec.name, step.Statement)
	}
	pin := s.spec.deploys[s.deployCount]
	s.deployCount++
	if pin.label != step.Statement || pin.epl != step.Epl {
		return fmt.Errorf("%s: case %q deploy %q is not pinned",
			viewExprBatchCore570ID, s.spec.name, step.Statement)
	}
	if _, ok := s.statements[pin.label]; ok {
		return fmt.Errorf("%s: label %q is already deployed", viewExprBatchCore570ID, pin.label)
	}
	query, err := s.deployQuery(step.Epl)
	if err != nil {
		return err
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return fmt.Errorf("%s: build %q: %w", viewExprBatchCore570ID, pin.label, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", viewExprBatchCore570ID, pin.label, err)
	}
	s.deployments = append(s.deployments, deployment)
	statements := deployment.Statements()
	if len(statements) != 1 {
		return fmt.Errorf("%s: deploy %q produced %d statements",
			viewExprBatchCore570ID, pin.label, len(statements))
	}
	statement := statements[0]
	s.statements[pin.label] = statement
	label := pin.label
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

// deployQuery builds the typed query matching one pinned EPL. All four
// are `select irstream` statements over a single SupportBean expr_batch
// window; WithOldStream surfaces both streams like Java's irstream and
// every statement carries Java's addListener("s0").
func (s *viewExprBatchCore570CaseState) deployQuery(epl string) (esper.Query, error) {
	from := esper.From[viewExprBatchCore570Bean](s.env, viewExprBatchCore570SupportBeanEvent)
	switch epl {
	case viewExprBatchCore570EPLNewestOldestExclude:
		// `..., false)`: the flush emits the batch accumulated before
		// the triggering send and the trigger event starts the next
		// batch — Java's explicit exclude-trigger-event form.
		return from.Window(esper.ExpressionBatch(
			viewExprBatchCore570BoundaryTrigger(), esper.ExcludeTriggerEvent())).
			Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	case viewExprBatchCore570EPLNewestOldestInclude:
		// `..., true)`: the flush folds the triggering send into the
		// batch it fired on — Java's explicit include-trigger-event form.
		return from.Window(esper.ExpressionBatch(
			viewExprBatchCore570BoundaryTrigger(), esper.IncludeTriggerEvent())).
			Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	case viewExprBatchCore570EPLLengthBatch:
		// `(current_count >= 3, true)`: every third accumulating row
		// closes the batch and is part of it.
		trigger := esper.GreaterOrEqual[int64](esper.WindowCurrentCount(), esper.Literal(int64(3)))
		return from.Window(esper.ExpressionBatch(trigger, esper.IncludeTriggerEvent())).
			Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	case viewExprBatchCore570EPLTimeBatch:
		// `(newest_timestamp - oldest_timestamp > 2000)`: the span of
		// arrival timestamps inside the pending batch must exceed two
		// seconds; a clock advance alone never evaluates the trigger.
		trigger := esper.Greater[int64](
			esper.Subtract[int64](esper.WindowNewestTimestamp(), esper.WindowOldestTimestamp()),
			esper.Literal(int64(2000)))
		return from.Window(esper.ExpressionBatch(trigger)).
			Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	case viewExprBatchCore570EPLEventProp:
		// `(intPrimitive > 0)`: each arriving event is itself the trigger
		// probe; a non-positive row stays silent but remains pending, so
		// the retained E3/-1 rides the next flush's new data.
		trigger := esper.Greater[int](
			esper.Field[viewExprBatchCore570Bean, int]("intPrimitive"), esper.Literal(0))
		return esper.Select(
			from.Window(esper.ExpressionBatch(trigger)),
			esper.Alias("val0", esper.Field[viewExprBatchCore570Bean, string]("theString")),
		).Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	}
	return esper.Query{}, fmt.Errorf("%s: case %q EPL is not pinned", viewExprBatchCore570ID, s.spec.name)
}

// viewExprBatchCore570BoundaryTrigger renders ord 0's
// `newest_event.intPrimitive != oldest_event.intPrimitive` predicate over
// the pending batch's boundary events.
func viewExprBatchCore570BoundaryTrigger() esper.Expression[bool] {
	newest := esper.NestedField[int](esper.WindowNewestEvent(), "intPrimitive")
	oldest := esper.NestedField[int](esper.WindowOldestEvent(), "intPrimitive")
	return esper.NotEqual[int](newest, oldest)
}

// viewExprBatchCore570DecodePayload converts a scenario send payload into
// the typed host object: a SupportBean struct with the pinned theString
// and intPrimitive fields.
func viewExprBatchCore570DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case viewExprBatchCore570SupportBeanEvent:
		if err := requireViewExprBatchCore570Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean viewExprBatchCore570Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", viewExprBatchCore570ID, step.EventType)
	}
}
