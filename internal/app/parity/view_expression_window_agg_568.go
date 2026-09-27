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

// Parity coverage for the ViewExpressionWindow aggregate-keep and
// named-window-delete quartet: ords 7/8/9/10 of the pinned
// ViewExpressionWindow.java executions() collection — two statement-level
// aggregate #expr keeps and two three-statement named-window modules with
// #expr retention plus an on-delete trigger.
//
// Covered executions:
//   - ViewExpressionWindow ord 7 ViewExpressionWindowAggregationUngrouped
//     java-runtime-0201ff1e8d8eaabe883f (case aggregation-ungrouped)
//   - ViewExpressionWindow ord 8 ViewExpressionWindowAggregationWGroupwin
//     java-runtime-bef7f02bfc8cb8b18b86 (case aggregation-groupwin)
//   - ViewExpressionWindow ord 9 ViewExpressionWindowNamedWindowDelete
//     java-runtime-d9281b1cc6d48c4984e1 (case named-window-delete)
//   - ViewExpressionWindow ord 10 ViewExpressionWindowAggregationWOnDelete
//     java-runtime-e4c4569b44a3e479c37e (case aggregation-on-delete)
//
// aggregation-ungrouped keeps rows while sum(intPrimitive) < 10 and replays
// the full nine-send sequence the Go unit test truncated at E5: E2/9 evicts
// E1, the E3/11 and E4/12 sends self-expire (each posts its own event on
// both insert and remove streams while the iterator is empty), E5-E7
// accumulate {E5,E6,E7}, E8/6 evicts {E5,E6} and E9/9 evicts {E7,E8}.
// aggregation-groupwin partitions by intPrimitive and keeps rows while the
// per-group sum(longPrimitive) < 10: E5/2/6 evicts the whole group-2 pair
// {E2,E4} in one delivery and E6/1/2 evicts the group-1 head E1.
// named-window-delete deploys a three-statement module — @name('s0')
// create window NW#expr(true) as SupportBean, a wildcard insert-into and an
// on-SupportBean_A delete (theString = id) — then deletes E2 through the
// trigger; the s0 create-window statement carries the listener and the
// iterator. aggregation-on-delete swaps in #expr(sum(intPrimitive) < 10)
// retention: the A(E2) delete posts old {E2} and the keep predicate
// re-evaluates after the removal, so the E4/2 insert pops {E1}
// (sum(intPrimitive) = 1+7+2 = 10 fails < 10).
//
// The Java regression runs milestone() calls and listenerReset as
// regression-harness savepoints only; they restore identical state for
// these non-contextual executions, so the scenario omits them (they pin no
// observable). Snapshot records sit where Java asserts the iterator:
// aggregation-ungrouped asserts after every send (empty post-E3/post-E4
// windows included) and named-window-delete asserts the {E1,E2,E3} and
// {E1,E3} contents — all ordered. aggregation-groupwin and
// aggregation-on-delete pin assertPropsPerRowIRPairFlattened only, so their
// cases record listener deliveries without snapshots.
const viewExprWin568ID = "view-expression-window-agg-568"

const viewExprWin568Description = "ViewExpressionWindow ords 7/8/9/10 — the aggregate-keep and named-window-delete quartet. aggregation-ungrouped (ViewExpressionWindowAggregationUngrouped, ord 7) replays `@name('s0') select irstream theString from SupportBean#expr(sum(intPrimitive) < 10)`: sends E1/1 through E9/9 post the full nine-delivery sequence including both self-expiring inserts (E3/11 posts new {E3} with old {E2,E3} and an empty iterator; E4/12 posts new {E4} with old {E4}) before E5-E7 accumulate {E5,E6,E7}, E8/6 evicts {E5,E6} and E9/9 evicts {E7,E8}; every Java iterator assertion posts an ordered snapshot. aggregation-groupwin (ord 8) replays `@name('s0') select irstream theString from SupportBean#groupwin(intPrimitive)#expr(sum(longPrimitive) < 10)`: E1-E4 accumulate per group, E5/2/6 evicts the whole group-2 pair {E2,E4} and E6/1/2 evicts the group-1 head {E1}; the Java execution pins flattened IR pairs only, so the case records listener deliveries without snapshots. named-window-delete (ord 9) and aggregation-on-delete (ord 10) replay their three-statement modules `@name('s0') create window NW#expr(true)` / `NW#expr(sum(intPrimitive) < 10) as SupportBean` plus a wildcard insert-into and an on-SupportBean_A delete (theString = id): ord 9's A(E2) delete posts old {E2} between ordered iterator snapshots {E1,E2,E3} and {E1,E3}; ord 10's keep predicate re-evaluates after the delete-triggered removal so E4/2 pops {E1} (sum(intPrimitive) = 10 fails < 10). Java milestone() calls and listenerReset are regression-harness savepoints with identical restored state, so the scenario omits them."

const viewExprWin568JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const viewExprWin568JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java"

// Byte-exact EPL pins (ViewExpressionWindow.java lines 351-353 and 393 for
// the named-window module and the ungrouped aggregation, line 447 for the
// groupwin variant, lines 477-479 for the aggregation-on-delete module; the
// three-statement module text is pinned verbatim including its trailing
// statement separators).
const (
	viewExprWin568EPLAggUngrouped = "@name('s0') select irstream theString from SupportBean#expr(sum(intPrimitive) < 10)"
	viewExprWin568EPLAggGroupwin  = "@name('s0') select irstream theString from SupportBean#groupwin(intPrimitive)#expr(sum(longPrimitive) < 10)"

	viewExprWin568EPLNWCreateKeep = "@name('s0') create window NW#expr(true) as SupportBean"
	viewExprWin568EPLNWCreateAgg  = "@name('s0') create window NW#expr(sum(intPrimitive) < 10) as SupportBean"
	viewExprWin568EPLNWInsert     = "insert into NW select * from SupportBean"
	viewExprWin568EPLNWDelete     = "on SupportBean_A delete from NW where theString = id"

	viewExprWin568SupportBeanEvent  = "SupportBean"
	viewExprWin568SupportBeanAEvent = "SupportBean_A"
)

// viewExprWin568ModuleEPL joins one named-window module's statements in the
// byte-exact shape the Java executions pass to compileDeploy: each
// statement terminated by `;` plus a newline.
func viewExprWin568ModuleEPL(createEPL string) string {
	return createEPL + ";\n" + viewExprWin568EPLNWInsert + ";\n" + viewExprWin568EPLNWDelete + ";\n"
}

var (
	viewExprWin568JavaRuntimeIDs = []string{
		"java-runtime-0201ff1e8d8eaabe883f",
		"java-runtime-bef7f02bfc8cb8b18b86",
		"java-runtime-d9281b1cc6d48c4984e1",
		"java-runtime-e4c4569b44a3e479c37e",
	}
	viewExprWin568JavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java",
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
	}
	viewExprWin568JavaExecutions = []string{
		"ViewExpressionWindowAggregationUngrouped",
		"ViewExpressionWindowAggregationWGroupwin",
		"ViewExpressionWindowNamedWindowDelete",
		"ViewExpressionWindowAggregationWOnDelete",
	}
	// Deduplicated inventory id: all four runtime rows share static id
	// java-06e6b1f6c905b8f12b82, pinned once per runtimeId row.
	viewExprWin568JavaStaticIDs = []string{
		"java-06e6b1f6c905b8f12b82",
		"java-06e6b1f6c905b8f12b82",
		"java-06e6b1f6c905b8f12b82",
		"java-06e6b1f6c905b8f12b82",
	}
	viewExprWin568JavaFlags = []string{}
)

// viewExprWin568Bean mirrors the SupportBean properties the executions use:
// theString (the iterator/listener field), intPrimitive (the sum term and
// the groupwin key) and longPrimitive (the groupwin sum term).
type viewExprWin568Bean struct {
	TheString     string `json:"theString" esper:"theString"`
	IntPrimitive  int    `json:"intPrimitive" esper:"intPrimitive"`
	LongPrimitive int64  `json:"longPrimitive" esper:"longPrimitive"`
}

// viewExprWin568BeanA mirrors SupportBean_A: the delete trigger's id.
type viewExprWin568BeanA struct {
	ID string `json:"id" esper:"id"`
}

// viewExprWin568DeploySpec binds one scenario deploy label to its byte-exact
// statement text inside the case's module.
type viewExprWin568DeploySpec struct {
	label string
	epl   string
}

// viewExprWin568CaseSpec pins one Java execution: case identity, observation
// text, byte-exact EPL, the deploy-label sequence and the pinned
// listener/snapshot field projection.
type viewExprWin568CaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
	deploys     []viewExprWin568DeploySpec
	fields      []string
}

var viewExprWin568CaseSpecs = []viewExprWin568CaseSpec{
	{
		name:      "aggregation-ungrouped",
		ordinal:   7,
		runtimeID: "java-runtime-0201ff1e8d8eaabe883f",
		execution: "ViewExpressionWindowAggregationUngrouped",
		observation: "deployed+listener+snapshot; ungrouped sum(intPrimitive) < 10 keep over the full " +
			"nine-send sequence: E1/1 keeps, E2/9 evicts {E1}, E3/11 and E4/12 self-expire " +
			"(new {E3}/old {E2,E3} and new {E4}/old {E4} with empty iterators), E5/1-E7/3 " +
			"accumulate {E5,E6,E7}, E8/6 evicts {E5,E6} and E9/9 evicts {E7,E8}; ordered " +
			"iterator snapshots pin theString after every send",
		epl: viewExprWin568EPLAggUngrouped,
		deploys: []viewExprWin568DeploySpec{
			{label: "s0", epl: viewExprWin568EPLAggUngrouped},
		},
		fields: []string{"theString"},
	},
	{
		name:      "aggregation-groupwin",
		ordinal:   8,
		runtimeID: "java-runtime-bef7f02bfc8cb8b18b86",
		execution: "ViewExpressionWindowAggregationWGroupwin",
		observation: "deployed+listener; #groupwin(intPrimitive)#expr(sum(longPrimitive) < 10) keeps " +
			"rows per group: E1/1/5-E4/2/4 accumulate, E5/2/6 evicts the whole group-2 pair " +
			"{E2,E4} in one delivery and E6/1/2 evicts the group-1 head {E1}; the Java " +
			"execution pins flattened IR pairs only, so no snapshots are recorded",
		epl: viewExprWin568EPLAggGroupwin,
		deploys: []viewExprWin568DeploySpec{
			{label: "s0", epl: viewExprWin568EPLAggGroupwin},
		},
		fields: []string{"theString"},
	},
	{
		name:      "named-window-delete",
		ordinal:   9,
		runtimeID: "java-runtime-d9281b1cc6d48c4984e1",
		execution: "ViewExpressionWindowNamedWindowDelete",
		observation: "deployed+listener+snapshot; one three-statement module: @name('s0') create " +
			"window NW#expr(true) as SupportBean, a wildcard insert-into and an " +
			"on-SupportBean_A delete (theString = id); sends E1-E3 accumulate, the ordered " +
			"iterator pins {E1,E2,E3}, the A(E2) delete posts old {E2} and the iterator " +
			"pins {E1,E3}",
		epl: viewExprWin568ModuleEPL(viewExprWin568EPLNWCreateKeep),
		deploys: []viewExprWin568DeploySpec{
			{label: "s0", epl: viewExprWin568EPLNWCreateKeep},
			{label: "insert", epl: viewExprWin568EPLNWInsert},
			{label: "delete", epl: viewExprWin568EPLNWDelete},
		},
		fields: []string{"theString"},
	},
	{
		name:      "aggregation-on-delete",
		ordinal:   10,
		runtimeID: "java-runtime-e4c4569b44a3e479c37e",
		execution: "ViewExpressionWindowAggregationWOnDelete",
		observation: "deployed+listener; the named-window-delete module with " +
			"NW#expr(sum(intPrimitive) < 10) retention: E1/1 and E2/8 insert, the A(E2) " +
			"delete posts old {E2} with the keep predicate re-evaluated over {E1}, E3/7 " +
			"inserts and E4/2 posts new {E4} while the retention pops {E1} " +
			"(sum(intPrimitive) = 10 fails < 10); the Java execution pins flattened IR " +
			"pairs only, so no snapshots are recorded",
		epl: viewExprWin568ModuleEPL(viewExprWin568EPLNWCreateAgg),
		deploys: []viewExprWin568DeploySpec{
			{label: "s0", epl: viewExprWin568EPLNWCreateAgg},
			{label: "insert", epl: viewExprWin568EPLNWInsert},
			{label: "delete", epl: viewExprWin568EPLNWDelete},
		},
		fields: []string{"theString"},
	},
}

func viewExprWin568CaseSpecFor(name string) (viewExprWin568CaseSpec, bool) {
	for _, spec := range viewExprWin568CaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return viewExprWin568CaseSpec{}, false
}

// viewExprWin568StepPin pins one scenario step's shape.
type viewExprWin568StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	mode      string
}

func viewExprWin568DeployPin(statement, epl string) viewExprWin568StepPin {
	return viewExprWin568StepPin{op: "deploy", statement: statement, epl: epl}
}

func viewExprWin568DeployedPin(statement string) viewExprWin568StepPin {
	return viewExprWin568StepPin{op: "deployed", statement: statement}
}

func viewExprWin568SendPin(theString string, intPrimitive int) viewExprWin568StepPin {
	return viewExprWin568StepPin{
		op:        "send",
		eventType: viewExprWin568SupportBeanEvent,
		payload:   map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)},
	}
}

func viewExprWin568SendLongPin(theString string, intPrimitive int, longPrimitive int64) viewExprWin568StepPin {
	return viewExprWin568StepPin{
		op:        "send",
		eventType: viewExprWin568SupportBeanEvent,
		payload: map[string]any{
			"theString":     theString,
			"intPrimitive":  float64(intPrimitive),
			"longPrimitive": float64(longPrimitive),
		},
	}
}

func viewExprWin568SendAPin(id string) viewExprWin568StepPin {
	return viewExprWin568StepPin{
		op:        "send",
		eventType: viewExprWin568SupportBeanAEvent,
		payload:   map[string]any{"id": id},
	}
}

func viewExprWin568SnapshotPin(statement, mode string) viewExprWin568StepPin {
	return viewExprWin568StepPin{op: "snapshot", statement: statement, mode: mode}
}

func viewExprWin568UndeployAllPin() viewExprWin568StepPin {
	return viewExprWin568StepPin{op: "undeploy-all"}
}

// viewExprWin568ModulePins builds the pinned step sequence for one
// three-statement named-window module deploy: three contiguous deploy steps
// (the oracle batches them into one module compileDeploy) followed by their
// deployed markers in statement order.
func viewExprWin568ModulePins(createEPL string) []viewExprWin568StepPin {
	return []viewExprWin568StepPin{
		viewExprWin568DeployPin("s0", createEPL),
		viewExprWin568DeployPin("insert", viewExprWin568EPLNWInsert),
		viewExprWin568DeployPin("delete", viewExprWin568EPLNWDelete),
		viewExprWin568DeployedPin("s0"),
		viewExprWin568DeployedPin("insert"),
		viewExprWin568DeployedPin("delete"),
	}
}

// viewExprWin568CaseSteps pins the complete step sequence per case in Java
// source order. Milestones and the listenerReset are omitted
// (regression-harness savepoints with identical restored state); snapshots
// sit where Java asserts the iterator.
var viewExprWin568CaseSteps = map[string][]viewExprWin568StepPin{
	"aggregation-ungrouped": {
		viewExprWin568DeployPin("s0", viewExprWin568EPLAggUngrouped),
		viewExprWin568DeployedPin("s0"),
		viewExprWin568SendPin("E1", 1),
		viewExprWin568SnapshotPin("s0", "ordered"),
		viewExprWin568SendPin("E2", 9),
		viewExprWin568SnapshotPin("s0", "ordered"),
		viewExprWin568SendPin("E3", 11),
		viewExprWin568SnapshotPin("s0", "ordered"),
		viewExprWin568SendPin("E4", 12),
		viewExprWin568SnapshotPin("s0", "ordered"),
		viewExprWin568SendPin("E5", 1),
		viewExprWin568SnapshotPin("s0", "ordered"),
		viewExprWin568SendPin("E6", 2),
		viewExprWin568SnapshotPin("s0", "ordered"),
		viewExprWin568SendPin("E7", 3),
		viewExprWin568SnapshotPin("s0", "ordered"),
		viewExprWin568SendPin("E8", 6),
		viewExprWin568SnapshotPin("s0", "ordered"),
		viewExprWin568SendPin("E9", 9),
		viewExprWin568SnapshotPin("s0", "ordered"),
		viewExprWin568UndeployAllPin(),
	},
	"aggregation-groupwin": {
		viewExprWin568DeployPin("s0", viewExprWin568EPLAggGroupwin),
		viewExprWin568DeployedPin("s0"),
		viewExprWin568SendLongPin("E1", 1, 5),
		viewExprWin568SendLongPin("E2", 2, 2),
		viewExprWin568SendLongPin("E3", 1, 3),
		viewExprWin568SendLongPin("E4", 2, 4),
		viewExprWin568SendLongPin("E5", 2, 6),
		viewExprWin568SendLongPin("E6", 1, 2),
		viewExprWin568UndeployAllPin(),
	},
	"named-window-delete": func() []viewExprWin568StepPin {
		steps := viewExprWin568ModulePins(viewExprWin568EPLNWCreateKeep)
		return append(steps,
			viewExprWin568SendPin("E1", 1),
			viewExprWin568SendPin("E2", 2),
			viewExprWin568SendPin("E3", 3),
			viewExprWin568SnapshotPin("s0", "ordered"),
			viewExprWin568SendAPin("E2"),
			viewExprWin568SnapshotPin("s0", "ordered"),
			viewExprWin568UndeployAllPin(),
		)
	}(),
	"aggregation-on-delete": func() []viewExprWin568StepPin {
		steps := viewExprWin568ModulePins(viewExprWin568EPLNWCreateAgg)
		return append(steps,
			viewExprWin568SendPin("E1", 1),
			viewExprWin568SendPin("E2", 8),
			viewExprWin568SendAPin("E2"),
			viewExprWin568SendPin("E3", 7),
			viewExprWin568SendPin("E4", 2),
			viewExprWin568UndeployAllPin(),
		)
	}(),
}

func loadViewExprWin568Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", viewExprWin568ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", viewExprWin568ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprWin568ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprWin568ID, err)
	}
	if err := requireViewExprWin568Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", viewExprWin568ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != viewExprWin568ID ||
		metadata.Description != viewExprWin568Description ||
		metadata.JavaCommit != viewExprWin568JavaCommit ||
		metadata.JavaSource != viewExprWin568JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", viewExprWin568ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, viewExprWin568JavaSources},
		{metadata.JavaRuntimes, viewExprWin568JavaRuntimeIDs},
		{metadata.JavaNames, viewExprWin568JavaExecutions},
		{metadata.JavaStaticIDs, viewExprWin568JavaStaticIDs},
		{metadata.JavaFlags, viewExprWin568JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", viewExprWin568ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(viewExprWin568CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			viewExprWin568ID, len(viewExprWin568CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireViewExprWin568Fields(object, "case", "ordinal", "runtimeId",
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
		spec := viewExprWin568CaseSpecs[index]
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
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", viewExprWin568ID)
	}
	if err := validateViewExprWin568RawSteps(rawSteps); err != nil {
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

// validateViewExprWin568RawSteps pins the complete step sequence per case:
// op field whitelists per step kind plus positional comparison against the
// pinned sequence.
func validateViewExprWin568RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(viewExprWin568CaseSpecs))
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
			if err := requireViewExprWin568Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := viewExprWin568CaseSpecFor(marker.Case); !ok {
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
		pins := viewExprWin568CaseSteps[currentCase]
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
			if err := requireViewExprWin568Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "deployed":
			if err := requireViewExprWin568Fields(object, "op", "case", "statement"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deployed marker is not pinned for case %q",
					index, currentCase)
			}
		case "send":
			if err := requireViewExprWin568Fields(object, "op", "case", "eventType", "payload"); err != nil {
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
			if err := requireViewExprWin568Fields(object, "op", "case", "statement", "mode"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.Mode != pin.mode {
				return fmt.Errorf("scenario step %d snapshot is not pinned for case %q",
					index, currentCase)
			}
		case "undeploy-all":
			if err := requireViewExprWin568Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(viewExprWin568CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(viewExprWin568CaseSpecs))
	}
	for index, spec := range viewExprWin568CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(viewExprWin568CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(viewExprWin568CaseSteps[spec.name]))
		}
	}
	return nil
}

func requireViewExprWin568Fields(object map[string]json.RawMessage, names ...string) error {
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

// viewExprWin568CaseState carries per-case replay state: the deployed
// statements (label -> statement), deployments for undeploy-all, the
// listener sequence counters and the delivery records.
type viewExprWin568CaseState struct {
	spec        viewExprWin568CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	now         time.Time
	statements  map[string]*esper.Statement
	deployments []*esper.Deployment
	sequence    map[string]uint64
	records     []compat.TraceRecord
}

func viewExprWin568StartEnvironment() (*esper.Environment, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewExprWin568Bean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[viewExprWin568BeanA](env, "SupportBean_A"); err != nil {
		return nil, err
	}
	return env, nil
}

// runViewExprWin568Scenario replays all four executions, one fresh engine
// per case like the Java oracle's per-execution runtime.
func runViewExprWin568Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateViewExprWin568Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range viewExprWin568CaseSpecs {
		records, err := runViewExprWin568Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", viewExprWin568ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validateViewExprWin568Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != viewExprWin568ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, viewExprWin568ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validateViewExprWin568RawSteps(rawSteps)
}

func runViewExprWin568Case(ctx context.Context, scenario compat.Scenario, spec viewExprWin568CaseSpec) ([]compat.TraceRecord, error) {
	env, err := viewExprWin568StartEnvironment()
	if err != nil {
		return nil, err
	}
	now := time.Unix(0, 0).UTC()
	state := &viewExprWin568CaseState{
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
					viewExprWin568ID, step.Statement)
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
			payload, err := viewExprWin568DecodePayload(step)
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
			return nil, fmt.Errorf("%s: unsupported step op %q", viewExprWin568ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps each pinned deploy label onto the typed Go call or plan
// equivalent of the Java statement: s0 builds the case's irstream select
// (ords 7/8) or registers the NW named window plus its create-window
// statement (ords 9/10), while the insert/delete labels build the
// insert-into and on-delete trigger statements. The byte-exact EPL text of
// the Java compileDeploy call is pinned by the loader.
func (s *viewExprWin568CaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	if _, ok := s.statements[label]; ok {
		return fmt.Errorf("%s: label %q is already deployed", viewExprWin568ID, label)
	}
	pinned := false
	for _, pin := range s.spec.deploys {
		if pin.label == label {
			pinned = pin.epl == step.Epl
			break
		}
	}
	if !pinned {
		return fmt.Errorf("%s: case %q deploy %q is not pinned", viewExprWin568ID, s.spec.name, label)
	}
	switch s.spec.name {
	case "aggregation-ungrouped":
		// `select irstream theString from SupportBean#expr(sum(intPrimitive)
		// < 10)`: keep rows while the window's intPrimitive sum stays under
		// ten; WithOldStream surfaces both streams like Java's irstream.
		predicate := esper.Less[int](
			esper.Sum[int](esper.Field[viewExprWin568Bean, int]("intPrimitive")),
			esper.Literal(10))
		return s.deployQuery(ctx, label, esper.Select(
			esper.From[viewExprWin568Bean](s.env, "SupportBean").Window(esper.ExpressionWindow(predicate)),
			esper.Alias("theString", esper.Field[viewExprWin568Bean, string]("theString")),
		).Query(esper.StatementName("s0"), esper.WithOldStream()), true)
	case "aggregation-groupwin":
		// `select irstream theString from
		// SupportBean#groupwin(intPrimitive)#expr(sum(longPrimitive) < 10)`:
		// the keep predicate holds per intPrimitive group and evicts the
		// whole group's expired prefix in one delivery.
		predicate := esper.Less[int64](
			esper.Sum[int64](esper.Field[viewExprWin568Bean, int64]("longPrimitive")),
			esper.Literal(int64(10)))
		return s.deployQuery(ctx, label, esper.Select(
			esper.From[viewExprWin568Bean](s.env, "SupportBean").Window(esper.GroupWindow(
				esper.Field[viewExprWin568Bean, int]("intPrimitive"),
				esper.ExpressionWindow(predicate))),
			esper.Alias("theString", esper.Field[viewExprWin568Bean, string]("theString")),
		).Query(esper.StatementName("s0"), esper.WithOldStream()), true)
	case "named-window-delete", "aggregation-on-delete":
		switch label {
		case "s0":
			// `@name('s0') create window NW#expr(...) as SupportBean`:
			// the s0 statement IS the create-window statement — Java's
			// addListener("s0") receives the window's insert/remove deltas
			// and its iterator enumerates the window contents.
			schema, ok := s.env.Schema("SupportBean")
			if !ok {
				return fmt.Errorf("%s: SupportBean schema is not registered", viewExprWin568ID)
			}
			retention := esper.ExpressionWindow(esper.Literal(true))
			if s.spec.name == "aggregation-on-delete" {
				retention = esper.ExpressionWindow(esper.Less[int](
					esper.Sum[int](esper.Field[viewExprWin568Bean, int]("intPrimitive")),
					esper.Literal(10)))
			}
			if _, err := esper.CreateNamedWindow(s.env, "NW", schema,
				esper.NamedWindowRetention(retention)); err != nil {
				return err
			}
			if _, ok := s.engine.NamedWindow("NW"); !ok {
				return fmt.Errorf("%s: named window NW was not materialized", viewExprWin568ID)
			}
			return s.deployQuery(ctx, label,
				esper.FromNamedWindow(s.env, "NW").
					CreateNamedWindowQuery(esper.StatementName("s0"), esper.WithOldStream()), true)
		case "insert":
			// `insert into NW select * from SupportBean`.
			return s.deployQuery(ctx, label,
				esper.OnEvent(esper.From[viewExprWin568Bean](s.env, "SupportBean")).
					InsertIntoNamedWindow("NW", esper.CopyMatchingFields()).
					Query(esper.StatementName("insert")), false)
		case "delete":
			// `on SupportBean_A delete from NW where theString = id`.
			return s.deployQuery(ctx, label,
				esper.OnEvent(esper.From[viewExprWin568BeanA](s.env, "SupportBean_A")).
					DeleteFromNamedWindow("NW",
						esper.Equal[string](
							esper.NamedWindowField[string]("theString"),
							esper.Field[viewExprWin568BeanA, string]("id"))).
					Query(esper.StatementName("delete")), false)
		}
	}
	return fmt.Errorf("%s: unknown case %q deploy %q", viewExprWin568ID, s.spec.name, label)
}

// deployQuery builds and deploys one query, registering its single
// statement under the scenario label and attaching the delivery recorder
// when the label carries Java's addListener.
func (s *viewExprWin568CaseState) deployQuery(ctx context.Context, label string, query esper.Query, listen bool) error {
	plan, err := s.env.Build(query)
	if err != nil {
		return fmt.Errorf("%s: build %q: %w", viewExprWin568ID, label, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", viewExprWin568ID, label, err)
	}
	s.deployments = append(s.deployments, deployment)
	statements := deployment.Statements()
	if len(statements) != 1 {
		return fmt.Errorf("%s: deploy %q produced %d statements", viewExprWin568ID, label, len(statements))
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
func (s *viewExprWin568CaseState) snapshot(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot statement %q was not deployed",
			viewExprWin568ID, step.Statement)
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

// viewExprWin568DecodePayload converts a scenario send payload into the
// typed host object: a SupportBean struct with the pinned theString,
// intPrimitive and optional longPrimitive fields, or a SupportBean_A
// struct for the delete triggers.
func viewExprWin568DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case viewExprWin568SupportBeanEvent:
		base := []string{"theString", "intPrimitive"}
		if _, ok := fields["longPrimitive"]; ok {
			base = append(base, "longPrimitive")
		}
		if err := requireViewExprWin568Fields(fields, base...); err != nil {
			return nil, err
		}
		var bean viewExprWin568Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case viewExprWin568SupportBeanAEvent:
		if err := requireViewExprWin568Fields(fields, "id"); err != nil {
			return nil, err
		}
		var bean viewExprWin568BeanA
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", viewExprWin568ID, step.EventType)
	}
}
