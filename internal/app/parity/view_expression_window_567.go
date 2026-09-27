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

// Parity coverage for the ViewExpressionWindow scene trio: ords 0/1/2 of the
// pinned ViewExpressionWindow.java executions() collection — three
// expression-window (#expr keep-predicate) executions sharing one shape (one
// s0 statement over SupportBean, SupportBean sends, iterator and
// insert/remove-stream assertions, virtual-time advance).
//
// Covered executions:
//   - ViewExpressionWindow ord 0 ViewExpressionWindowSceneOne
//     java-runtime-feef242da145c59be1bb (case scene-one)
//   - ViewExpressionWindow ord 1 ViewExpressionWindowNewestEventOldestEvent
//     java-runtime-1fd41f23589a5af4132c (case newest-oldest)
//   - ViewExpressionWindow ord 2 ViewExpressionWindowLengthWindow
//     java-runtime-0c450c9d1c1fcf3b5523 (case length-window)
//
// scene-one keeps rows while newest_timestamp - oldest_timestamp < 1000 over
// row arrival timestamps: advanceTime(10000) alone removes nothing (the keep
// predicate reads event timestamps, not engine time) and the E6 send at
// t=10000 evicts {E3,E4,E5} in one delivery. newest-oldest keeps rows while
// newest_event.intPrimitive = oldest_event.intPrimitive: a falsifying send
// evicts the whole window (E3/2 flushes {E1,E2}; E7/2 flushes {E4,E5,E6}).
// length-window keeps at most current_count <= 2 rows (E3 appends and pops
// E1; the plain `select *` surfaces the insert stream only, so E3 posts a
// new-only delivery).
//
// The Java regression runs milestone() calls as regression-harness
// savepoints only; they restore identical state for these non-contextual
// executions, so the scenario omits them (they pin no observable).
// scene-one snapshots are mode "any" (assertPropsPerRowIteratorAnyOrder;
// the diff canonicalizes row order), newest-oldest and length-window are
// mode "ordered" (assertPropsPerRowIterator in-order). All three cases pin
// listener deliveries; for length-window the new-only records are a
// stronger pin than Java's iterator-only assertion — a representation
// choice, not a semantic difference.
const viewExprWin567ID = "view-expression-window-567"

const viewExprWin567Description = "ViewExpressionWindow ords 0/1/2 — the expression-window scene trio. scene-one (ViewExpressionWindowSceneOne, ord 0) replays `@Name('s0') select irstream theString as c0 from SupportBean#expr(newest_timestamp - oldest_timestamp < 1000)`: advanceTime(0) plus sends E1@1000, E2@1500, E3@2000 (IR pair {E3}/{E1}), E4@2499 (window {E2,E3,E4}), E5@2500 (IR pair {E5}/{E2}), advanceTime(10000) expiring nothing, then E6 (IR pair {E6}/{E3,E4,E5}); every iterator assertion posts a snapshot record (mode any). newest-oldest (ord 1) replays `@name('s0') select irstream * from SupportBean#expr(newest_event.intPrimitive = oldest_event.intPrimitive)`: E1/1 and E2/1 accumulate, E3/2 flushes {E1,E2}, E4/3 flushes {E3}, E5/3+E6/3 accumulate to {E4,E5,E6}, E7/2 flushes the triple — in-order iterator pins after every send. length-window (ord 2) replays `@name('s0') select * from SupportBean#expr(current_count <= 2)`: E1/1, E2/2, E3/3 with iterator pins {E1}, {E1,E2}, {E2,E3}; both oracles also record the new-only listener deliveries (the plain select surfaces the insert stream only, so E3's eviction rides the snapshots), a stronger pin than Java's iterator-only assertion. Java milestone() calls are regression-harness savepoints with identical restored state for these executions, so the scenario omits them."

const viewExprWin567JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const viewExprWin567JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java"

// Byte-exact EPL pins (ViewExpressionWindow.java line 52 for scene-one's
// `@Name` spelling, line 108 for newest-oldest, line 158 for length-window;
// the casing difference is pinned verbatim).
const (
	viewExprWin567EPLSceneOne      = "@Name('s0') select irstream theString as c0 from SupportBean#expr(newest_timestamp - oldest_timestamp < 1000)"
	viewExprWin567EPLNewestOldest  = "@name('s0') select irstream * from SupportBean#expr(newest_event.intPrimitive = oldest_event.intPrimitive)"
	viewExprWin567EPLLengthWindow  = "@name('s0') select * from SupportBean#expr(current_count <= 2)"
	viewExprWin567TimeEpoch        = "1970-01-01T00:00:00.000Z"
	viewExprWin567Time1000         = "1970-01-01T00:00:01.000Z"
	viewExprWin567Time1500         = "1970-01-01T00:00:01.500Z"
	viewExprWin567Time2000         = "1970-01-01T00:00:02.000Z"
	viewExprWin567Time2499         = "1970-01-01T00:00:02.499Z"
	viewExprWin567Time2500         = "1970-01-01T00:00:02.500Z"
	viewExprWin567Time10000        = "1970-01-01T00:00:10.000Z"
	viewExprWin567SupportBeanEvent = "SupportBean"
)

var (
	viewExprWin567JavaRuntimeIDs = []string{
		"java-runtime-feef242da145c59be1bb",
		"java-runtime-1fd41f23589a5af4132c",
		"java-runtime-0c450c9d1c1fcf3b5523",
	}
	viewExprWin567JavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java",
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
	}
	viewExprWin567JavaExecutions = []string{
		"ViewExpressionWindowSceneOne",
		"ViewExpressionWindowNewestEventOldestEvent",
		"ViewExpressionWindowLengthWindow",
	}
	// Deduplicated inventory id: all three runtime rows share static id
	// java-06e6b1f6c905b8f12b82, pinned once per runtimeId row.
	viewExprWin567JavaStaticIDs = []string{
		"java-06e6b1f6c905b8f12b82",
		"java-06e6b1f6c905b8f12b82",
		"java-06e6b1f6c905b8f12b82",
	}
	viewExprWin567JavaFlags = []string{}
)

// viewExprWin567Bean mirrors the SupportBean properties the executions use:
// theString (the iterator/listener field and the c0 projection source) and
// intPrimitive (the newest_event/oldest_event equality field).
type viewExprWin567Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// viewExprWin567CaseSpec pins one Java execution: case identity, observation
// text, byte-exact EPL, deploy label and the pinned listener/snapshot field
// projection.
type viewExprWin567CaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
	deploys     []string
	fields      []string
}

var viewExprWin567CaseSpecs = []viewExprWin567CaseSpec{
	{
		name:      "scene-one",
		ordinal:   0,
		runtimeID: "java-runtime-feef242da145c59be1bb",
		execution: "ViewExpressionWindowSceneOne",
		observation: "deployed+listener+snapshot; timestamp-spread keep (newest_timestamp - oldest_timestamp < 1000 over row arrival times): " +
			"E1@1000/E2@1500 accumulate, E3@2000 posts the {E3}/{E1} pair, E4@2499 is retained (window {E2,E3,E4}), " +
			"E5@2500 posts {E5}/{E2}, advanceTime(10000) removes nothing and the E6 send posts {E6}/{E3,E4,E5}; " +
			"seven iterator assertions post canonical-sorted snapshots (empty pre-E1 included) on the c0 projection",
		epl:     viewExprWin567EPLSceneOne,
		deploys: []string{"s0"},
		fields:  []string{"c0"},
	},
	{
		name:      "newest-oldest",
		ordinal:   1,
		runtimeID: "java-runtime-1fd41f23589a5af4132c",
		execution: "ViewExpressionWindowNewestEventOldestEvent",
		observation: "deployed+listener+snapshot; boundary-event equality keep (newest_event.intPrimitive = oldest_event.intPrimitive): " +
			"E1/1,E2/1 accumulate, E3/2 flushes {E1,E2}, E4/3 flushes {E3}, E5/3+E6/3 accumulate to {E4,E5,E6}, " +
			"E7/2 flushes the whole triple; per-send in-order iterator pins project theString",
		epl:     viewExprWin567EPLNewestOldest,
		deploys: []string{"s0"},
		fields:  []string{"theString"},
	},
	{
		name:      "length-window",
		ordinal:   2,
		runtimeID: "java-runtime-0c450c9d1c1fcf3b5523",
		execution: "ViewExpressionWindowLengthWindow",
		observation: "deployed+listener+snapshot; current_count <= 2 keep: E1/1 and E2/2 deliver new-only, E3/3 appends and evicts E1 " +
			"— the plain select delivers the insert stream only so E3 posts a new-only record; the Java test asserts " +
			"iterator-only, both oracles pin the stronger delivery plus the in-order iterator {E1}, {E1,E2}, {E2,E3}",
		epl:     viewExprWin567EPLLengthWindow,
		deploys: []string{"s0"},
		fields:  []string{"theString"},
	},
}

func viewExprWin567CaseSpecFor(name string) (viewExprWin567CaseSpec, bool) {
	for _, spec := range viewExprWin567CaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return viewExprWin567CaseSpec{}, false
}

// viewExprWin567StepPin pins one scenario step's shape.
type viewExprWin567StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	at        string
	mode      string
}

func viewExprWin567DeployPin(statement, epl string) viewExprWin567StepPin {
	return viewExprWin567StepPin{op: "deploy", statement: statement, epl: epl}
}

func viewExprWin567DeployedPin(statement string) viewExprWin567StepPin {
	return viewExprWin567StepPin{op: "deployed", statement: statement}
}

func viewExprWin567SendPin(theString string, intPrimitive int) viewExprWin567StepPin {
	return viewExprWin567StepPin{
		op:        "send",
		eventType: viewExprWin567SupportBeanEvent,
		payload:   map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)},
	}
}

func viewExprWin567SnapshotPin(statement, mode string) viewExprWin567StepPin {
	return viewExprWin567StepPin{op: "snapshot", statement: statement, mode: mode}
}

func viewExprWin567AdvancePin(at string) viewExprWin567StepPin {
	return viewExprWin567StepPin{op: "advance-time", at: at}
}

func viewExprWin567UndeployAllPin() viewExprWin567StepPin {
	return viewExprWin567StepPin{op: "undeploy-all"}
}

// viewExprWin567CaseSteps pins the complete step sequence per case in Java
// source order. Milestones are omitted (regression-harness savepoints with
// identical restored state); snapshots sit where Java asserts the iterator.
var viewExprWin567CaseSteps = map[string][]viewExprWin567StepPin{
	"scene-one": {
		viewExprWin567AdvancePin(viewExprWin567TimeEpoch),
		viewExprWin567DeployPin("s0", viewExprWin567EPLSceneOne),
		viewExprWin567DeployedPin("s0"),
		viewExprWin567AdvancePin(viewExprWin567Time1000),
		viewExprWin567SnapshotPin("s0", "any"),
		viewExprWin567SendPin("E1", 0),
		viewExprWin567AdvancePin(viewExprWin567Time1500),
		viewExprWin567SnapshotPin("s0", "any"),
		viewExprWin567SendPin("E2", 0),
		viewExprWin567AdvancePin(viewExprWin567Time2000),
		viewExprWin567SnapshotPin("s0", "any"),
		viewExprWin567SendPin("E3", 0),
		viewExprWin567SnapshotPin("s0", "any"),
		viewExprWin567AdvancePin(viewExprWin567Time2499),
		viewExprWin567SnapshotPin("s0", "any"),
		viewExprWin567SendPin("E4", 0),
		viewExprWin567SnapshotPin("s0", "any"),
		viewExprWin567AdvancePin(viewExprWin567Time2500),
		viewExprWin567SnapshotPin("s0", "any"),
		viewExprWin567SendPin("E5", 0),
		viewExprWin567AdvancePin(viewExprWin567Time10000),
		viewExprWin567SendPin("E6", 0),
		viewExprWin567UndeployAllPin(),
	},
	"newest-oldest": {
		viewExprWin567DeployPin("s0", viewExprWin567EPLNewestOldest),
		viewExprWin567DeployedPin("s0"),
		viewExprWin567SendPin("E1", 1),
		viewExprWin567SnapshotPin("s0", "ordered"),
		viewExprWin567SendPin("E2", 1),
		viewExprWin567SnapshotPin("s0", "ordered"),
		viewExprWin567SendPin("E3", 2),
		viewExprWin567SnapshotPin("s0", "ordered"),
		viewExprWin567SendPin("E4", 3),
		viewExprWin567SnapshotPin("s0", "ordered"),
		viewExprWin567SendPin("E5", 3),
		viewExprWin567SnapshotPin("s0", "ordered"),
		viewExprWin567SendPin("E6", 3),
		viewExprWin567SnapshotPin("s0", "ordered"),
		viewExprWin567SendPin("E7", 2),
		viewExprWin567SnapshotPin("s0", "ordered"),
		viewExprWin567UndeployAllPin(),
	},
	"length-window": {
		viewExprWin567DeployPin("s0", viewExprWin567EPLLengthWindow),
		viewExprWin567DeployedPin("s0"),
		viewExprWin567SendPin("E1", 1),
		viewExprWin567SnapshotPin("s0", "ordered"),
		viewExprWin567SendPin("E2", 2),
		viewExprWin567SnapshotPin("s0", "ordered"),
		viewExprWin567SendPin("E3", 3),
		viewExprWin567SnapshotPin("s0", "ordered"),
		viewExprWin567UndeployAllPin(),
	},
}

func loadViewExprWin567Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", viewExprWin567ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", viewExprWin567ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprWin567ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprWin567ID, err)
	}
	if err := requireViewExprWin567Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", viewExprWin567ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != viewExprWin567ID ||
		metadata.Description != viewExprWin567Description ||
		metadata.JavaCommit != viewExprWin567JavaCommit ||
		metadata.JavaSource != viewExprWin567JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", viewExprWin567ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, viewExprWin567JavaSources},
		{metadata.JavaRuntimes, viewExprWin567JavaRuntimeIDs},
		{metadata.JavaNames, viewExprWin567JavaExecutions},
		{metadata.JavaStaticIDs, viewExprWin567JavaStaticIDs},
		{metadata.JavaFlags, viewExprWin567JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", viewExprWin567ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(viewExprWin567CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			viewExprWin567ID, len(viewExprWin567CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireViewExprWin567Fields(object, "case", "ordinal", "runtimeId",
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
		spec := viewExprWin567CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID ||
			definition.ExecutionName != spec.execution ||
			definition.Observation != spec.observation || definition.EPL != spec.epl ||
			!reflect.DeepEqual(definition.Deploys, spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %d metadata is not pinned", index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", viewExprWin567ID)
	}
	if err := validateViewExprWin567RawSteps(rawSteps); err != nil {
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

// validateViewExprWin567RawSteps pins the complete step sequence per case:
// op field whitelists per step kind plus positional comparison against the
// pinned sequence.
func validateViewExprWin567RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(viewExprWin567CaseSpecs))
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
			At        string          `json:"at"`
		}
		if operation == "case" {
			if err := requireViewExprWin567Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := viewExprWin567CaseSpecFor(marker.Case); !ok {
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
		pins := viewExprWin567CaseSteps[currentCase]
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
			if err := requireViewExprWin567Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "deployed":
			if err := requireViewExprWin567Fields(object, "op", "case", "statement"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deployed marker is not pinned for case %q",
					index, currentCase)
			}
		case "send":
			if err := requireViewExprWin567Fields(object, "op", "case", "eventType", "payload"); err != nil {
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
			if err := requireViewExprWin567Fields(object, "op", "case", "statement", "mode"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.Mode != pin.mode {
				return fmt.Errorf("scenario step %d snapshot is not pinned for case %q",
					index, currentCase)
			}
		case "advance-time":
			if err := requireViewExprWin567Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time is not pinned for case %q",
					index, currentCase)
			}
		case "undeploy-all":
			if err := requireViewExprWin567Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(viewExprWin567CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(viewExprWin567CaseSpecs))
	}
	for index, spec := range viewExprWin567CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(viewExprWin567CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(viewExprWin567CaseSteps[spec.name]))
		}
	}
	return nil
}

func requireViewExprWin567Fields(object map[string]json.RawMessage, names ...string) error {
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

// viewExprWin567CaseState carries per-case replay state: the deployed
// statement, the listener sequence counter and the delivery records.
type viewExprWin567CaseState struct {
	spec       viewExprWin567CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	now        time.Time
	statements map[string]*esper.Statement
	deployment *esper.Deployment
	sequence   map[string]uint64
	records    []compat.TraceRecord
}

func viewExprWin567StartEnvironment() (*esper.Environment, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewExprWin567Bean](env, "SupportBean"); err != nil {
		return nil, err
	}
	return env, nil
}

// runViewExprWin567Scenario replays all three executions, one fresh engine
// per case like the Java oracle's per-execution runtime.
func runViewExprWin567Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateViewExprWin567Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range viewExprWin567CaseSpecs {
		records, err := runViewExprWin567Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", viewExprWin567ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validateViewExprWin567Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != viewExprWin567ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, viewExprWin567ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validateViewExprWin567RawSteps(rawSteps)
}

func runViewExprWin567Case(ctx context.Context, scenario compat.Scenario, spec viewExprWin567CaseSpec) ([]compat.TraceRecord, error) {
	env, err := viewExprWin567StartEnvironment()
	if err != nil {
		return nil, err
	}
	now := time.Unix(0, 0).UTC()
	state := &viewExprWin567CaseState{
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
					viewExprWin567ID, step.Statement)
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
			payload, err := viewExprWin567DecodePayload(step)
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
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse advance-time %q: %w", viewExprWin567ID, step.At, err)
			}
			if err := state.engine.AdvanceTime(ctx, at); err != nil {
				return nil, err
			}
			state.now = at
		case "undeploy-all":
			if state.deployment != nil {
				if err := state.deployment.Undeploy(ctx); err != nil {
					return nil, err
				}
				state.deployment = nil
			}
			state.statements = make(map[string]*esper.Statement)
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", viewExprWin567ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned EPL onto the fluent Go equivalent: scene-one
// projects theString as c0 over a timestamp-spread keep, newest-oldest keeps
// on boundary-event intPrimitive equality, length-window keeps at most two
// rows. scene-one and newest-oldest are `irstream` selects (WithOldStream);
// length-window is a plain `select *` — Java's listener receives the
// insert stream only, so the default SelectIStream selector matches.
func (s *viewExprWin567CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" || step.Epl != s.spec.epl {
		return fmt.Errorf("%s: case %q deploy is not pinned", viewExprWin567ID, s.spec.name)
	}
	if _, ok := s.statements[step.Statement]; ok {
		return fmt.Errorf("%s: label %q is already deployed", viewExprWin567ID, step.Statement)
	}
	var query esper.Query
	switch s.spec.name {
	case "scene-one":
		// #expr(newest_timestamp - oldest_timestamp < 1000): keep while the
		// retained rows' arrival-timestamp spread is under one second.
		predicate := esper.Less[int64](
			esper.Subtract[int64](esper.WindowNewestTimestamp(), esper.WindowOldestTimestamp()),
			esper.Literal(int64(1000)))
		query = esper.Select(
			esper.From[viewExprWin567Bean](s.env, "SupportBean").Window(esper.ExpressionWindow(predicate)),
			esper.Alias("c0", esper.Field[viewExprWin567Bean, string]("theString")),
		).Query(esper.StatementName("s0"), esper.WithOldStream())
	case "newest-oldest":
		// #expr(newest_event.intPrimitive = oldest_event.intPrimitive): keep
		// while the boundary events' intPrimitive is equal.
		newest := esper.NestedField[int](esper.WindowNewestEvent(), "intPrimitive")
		oldest := esper.NestedField[int](esper.WindowOldestEvent(), "intPrimitive")
		query = esper.From[viewExprWin567Bean](s.env, "SupportBean").
			Window(esper.ExpressionWindow(esper.Equal[int](newest, oldest))).
			Query(esper.StatementName("s0"), esper.WithOldStream())
	case "length-window":
		// #expr(current_count <= 2): keep at most the two newest rows.
		predicate := esper.LessOrEqual[int64](esper.WindowCurrentCount(), esper.Literal(int64(2)))
		query = esper.From[viewExprWin567Bean](s.env, "SupportBean").
			Window(esper.ExpressionWindow(predicate)).
			Query(esper.StatementName("s0"))
	default:
		return fmt.Errorf("%s: unknown case %q", viewExprWin567ID, s.spec.name)
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return err
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	statement := deployment.Statements()[0]
	s.statements[step.Statement] = statement
	label := step.Statement
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
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
	}); err != nil {
		return err
	}
	return nil
}

// snapshot emits one iterator record for the statement, projecting the
// pinned field set. Row order stays engine-native; the differential
// canonicalizeAnyModeRows sorts the "any"-mode snapshots before comparing.
func (s *viewExprWin567CaseState) snapshot(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot statement %q was not deployed",
			viewExprWin567ID, step.Statement)
	}
	result, err := statement.Snapshot(ctx)
	if err != nil {
		return err
	}
	rows := projectRecords(compat.NormalizeResults(result.Batch.New), s.spec.fields)
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

// viewExprWin567DecodePayload converts a scenario send payload into the
// typed host object: a SupportBean struct with the pinned theString and
// intPrimitive fields.
func viewExprWin567DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case viewExprWin567SupportBeanEvent:
		if err := requireViewExprWin567Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean viewExprWin567Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", viewExprWin567ID, step.EventType)
	}
}
