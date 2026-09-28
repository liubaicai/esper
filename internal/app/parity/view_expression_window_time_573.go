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

// Parity coverage for the view-expression remainder closer: the single
// replayable execution in the bundle, ViewExpressionWindow ord 3
// ViewExpressionWindowTimeWindow — the keep-form timestamp window — plus the
// disposition records for the two compile-error-only siblings that enter the
// manifest as intentionally-different rows (no scenario steps, no trace).
//
// Covered execution:
//   - ViewExpressionWindow ord 3 ViewExpressionWindowTimeWindow
//     java-runtime-5d04258cf2eb6e0025fd (case time-window)
//
// time-window keeps rows while oldest_timestamp > newest_timestamp - 2000
// over arrival timestamps (the KEEP form — the expression-batch trigger form
// newest_timestamp - oldest_timestamp > 2000 is a different predicate and
// must not be substituted). Expiry is lazy: the view re-evaluates the keep
// predicate when new data arrives (ExpressionWindowView.update -> expire),
// never on a bare advanceTime, so advanceTime(10000) before the E8 send
// evicts nothing by itself. The Java execution sends E3 with no
// advanceTime after E2, so E2 and E3 share the t=1500 arrival timestamp:
// when E7@3500 arrives the keep predicate fails for both (1500 > 1500 is
// false) and the pair expires in one delivery — the double-old
// discriminant. E8@10000 then mass-expires {E4,E5,E6,E7}.
//
// Per-send iterator pins: the Java execution asserts the iterator after
// E1 and E3..E8; the scenario also snapshots after E2, a stronger pin than
// the Java assertion set and a representation choice, not a semantic
// difference.
//
// The Java regression runs milestone(0) after the E3 send and milestone(1)
// after the E6 send as regression-harness savepoints; they restore
// identical state for this non-contextual execution, so the scenario omits
// them (they pin no observable). The listenerReset after the E4 iterator
// assertion gates Java's assert-helper accumulation only — the raw
// listener records every delivery, so no scenario op pins it.
//
// Dispositioned siblings (compile-error-only; intentionally-different —
// the typed Go API has no EPL-text compile path, so the verbatim Java
// diagnostics cannot be pinned in the Go trace; the Go-side representative
// checks live in internal/esper/view_expression_parity_test.go and assert
// structural Build rejection only):
//
//   - ViewExpressionWindow ord 5 ViewExpressionWindowInvalid
//     java-runtime-984f59c809d70e02b3da. Byte-exact probes:
//     `select * from SupportBean#expr(1)` ->
//     "Failed to validate data window declaration: Invalid return value
//     for expiry expression, expected a boolean return value but received
//     int [select * from SupportBean#expr(1)]";
//     `select * from SupportBean#expr((select * from SupportBean#lastevent))` ->
//     "Failed to validate data window declaration: Invalid expiry
//     expression: Sub-select, previous or prior functions are not
//     supported in this context [select * from
//     SupportBean#expr((select * from SupportBean#lastevent))]".
//     TestViewExpressionWindowInvalidParity is the representative check:
//     a non-bool predicate and a Prev-bearing predicate both fail Build.
//   - ViewExpressionBatch ord 4 ViewExpressionBatchInvalid
//     java-runtime-ad02b23eda34efdfd83e. Byte-exact probes: the same two
//     with #expr_batch, plus
//     `select * from SupportBean#expr_batch(null < 0)` ->
//     "Failed to validate data window declaration: Invalid parameter
//     expression 0 for Expression-batch view: Failed to validate view
//     parameter expression 'null<0': Null-type value is not allow for
//     relational operator" (Java's `is not allow` typo verbatim). The
//     null<0 probe is unrepresentable in the typed Go API;
//     TestViewExpressionBatchInvalidParity is the representative check.
const viewExprWinTime573ID = "view-expression-window-time-573"

const viewExprWinTime573Description = "ViewExpressionWindow ord 3 — the keep-form timestamp window. time-window (ViewExpressionWindowTimeWindow, ord 3) replays `@name('s0') select irstream * from SupportBean#expr(oldest_timestamp > newest_timestamp - 2000)` over virtual time: E1@1000, E2@1500, E3@1500 (the Java execution sends E3 with no advanceTime, so E2 and E3 share the 1500 arrival timestamp) and E4@2500 accumulate; E5@3000 evicts {E1} (oldest 1000 is not > newest 3000 - 2000); E6@3499 is retained (window {E2,E3,E4,E5,E6}); E7@3500 evicts the tied pair {E2,E3} (1500 is not > 1500) — the double-old discriminant; E8@10000 mass-expires {E4,E5,E6,E7}. Expiry is lazy on send — the advanceTime(10000) before E8 evicts nothing by itself. Every send posts an in-order iterator snapshot (the post-E2 pin is stronger than the Java assertions, which skip the iterator there). Java milestone() savepoints and the post-E4 listenerReset are omitted: milestones restore identical state for this non-contextual execution and the reset gates Java's assert helper only — the raw listener records every delivery. The sibling compile-error executions ViewExpressionWindowInvalid (ord 5) and ViewExpressionBatchInvalid (ord 4) carry no replayable steps and join the manifest as intentionally-different rows."

const viewExprWinTime573JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const viewExprWinTime573JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java"

// Byte-exact EPL pin (ViewExpressionWindow.java line 181; the statement
// carries no trailing `;\n`).
const (
	viewExprWinTime573EPL              = "@name('s0') select irstream * from SupportBean#expr(oldest_timestamp > newest_timestamp - 2000)"
	viewExprWinTime573TimeEpoch        = "1970-01-01T00:00:00.000Z"
	viewExprWinTime573Time1000         = "1970-01-01T00:00:01.000Z"
	viewExprWinTime573Time1500         = "1970-01-01T00:00:01.500Z"
	viewExprWinTime573Time2500         = "1970-01-01T00:00:02.500Z"
	viewExprWinTime573Time3000         = "1970-01-01T00:00:03.000Z"
	viewExprWinTime573Time3499         = "1970-01-01T00:00:03.499Z"
	viewExprWinTime573Time3500         = "1970-01-01T00:00:03.500Z"
	viewExprWinTime573Time10000        = "1970-01-01T00:00:10.000Z"
	viewExprWinTime573SupportBeanEvent = "SupportBean"
)

var (
	viewExprWinTime573JavaRuntimeIDs = []string{
		"java-runtime-5d04258cf2eb6e0025fd",
	}
	viewExprWinTime573JavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java",
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
	}
	viewExprWinTime573JavaExecutions = []string{
		"ViewExpressionWindowTimeWindow",
	}
	// Deduplicated inventory id: the runtime row shares static id
	// java-06e6b1f6c905b8f12b82 with the other ViewExpressionWindow
	// executions, pinned once per runtimeId row.
	viewExprWinTime573JavaStaticIDs = []string{
		"java-06e6b1f6c905b8f12b82",
	}
	viewExprWinTime573JavaFlags = []string{}
)

// viewExprWinTime573Bean mirrors the SupportBean properties the execution
// uses: theString (the iterator/listener field) and intPrimitive (carried
// by the pinned sends but not projected).
type viewExprWinTime573Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// viewExprWinTime573CaseSpec pins the Java execution: case identity,
// observation text, byte-exact EPL, deploy label and the pinned
// listener/snapshot field projection.
type viewExprWinTime573CaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
	deploys     []string
	fields      []string
}

var viewExprWinTime573CaseSpecs = []viewExprWinTime573CaseSpec{
	{
		name:      "time-window",
		ordinal:   3,
		runtimeID: "java-runtime-5d04258cf2eb6e0025fd",
		execution: "ViewExpressionWindowTimeWindow",
		observation: "deployed+listener+snapshot; keep-form timestamp window (oldest_timestamp > newest_timestamp - 2000 " +
			"over row arrival timestamps; expiry is lazy — it re-evaluates on send, never on a bare advance): " +
			"E1@1000, E2@1500, E3@1500 (tied arrival with E2), E4@2500 accumulate, E5@3000 evicts {E1}, " +
			"E6@3499 is retained {E2..E6}, E7@3500 evicts the t=1500-tied pair {E2,E3} (double-old), " +
			"E8@10000 mass-expires {E4,E5,E6,E7}; per-send in-order iterator snapshots project theString",
		epl:     viewExprWinTime573EPL,
		deploys: []string{"s0"},
		fields:  []string{"theString"},
	},
}

func viewExprWinTime573CaseSpecFor(name string) (viewExprWinTime573CaseSpec, bool) {
	for _, spec := range viewExprWinTime573CaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return viewExprWinTime573CaseSpec{}, false
}

// viewExprWinTime573StepPin pins one scenario step's shape.
type viewExprWinTime573StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	at        string
	mode      string
}

func viewExprWinTime573DeployPin(statement, epl string) viewExprWinTime573StepPin {
	return viewExprWinTime573StepPin{op: "deploy", statement: statement, epl: epl}
}

func viewExprWinTime573DeployedPin(statement string) viewExprWinTime573StepPin {
	return viewExprWinTime573StepPin{op: "deployed", statement: statement}
}

func viewExprWinTime573SendPin(theString string, intPrimitive int) viewExprWinTime573StepPin {
	return viewExprWinTime573StepPin{
		op:        "send",
		eventType: viewExprWinTime573SupportBeanEvent,
		payload:   map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)},
	}
}

func viewExprWinTime573SnapshotPin(statement, mode string) viewExprWinTime573StepPin {
	return viewExprWinTime573StepPin{op: "snapshot", statement: statement, mode: mode}
}

func viewExprWinTime573AdvancePin(at string) viewExprWinTime573StepPin {
	return viewExprWinTime573StepPin{op: "advance-time", at: at}
}

func viewExprWinTime573UndeployAllPin() viewExprWinTime573StepPin {
	return viewExprWinTime573StepPin{op: "undeploy-all"}
}

// viewExprWinTime573CaseSteps pins the complete step sequence in Java
// source order. milestone(0) after E3 and milestone(1) after E6 are
// omitted (regression-harness savepoints restoring identical state);
// snapshots sit where Java asserts the iterator plus the stronger post-E2
// pin; there is NO send between the 1500 advance and E3, so E3 inherits
// E2's t=1500 arrival timestamp.
var viewExprWinTime573CaseSteps = map[string][]viewExprWinTime573StepPin{
	"time-window": {
		viewExprWinTime573AdvancePin(viewExprWinTime573TimeEpoch),
		viewExprWinTime573DeployPin("s0", viewExprWinTime573EPL),
		viewExprWinTime573DeployedPin("s0"),
		viewExprWinTime573AdvancePin(viewExprWinTime573Time1000),
		viewExprWinTime573SendPin("E1", 1),
		viewExprWinTime573SnapshotPin("s0", "ordered"),
		viewExprWinTime573AdvancePin(viewExprWinTime573Time1500),
		viewExprWinTime573SendPin("E2", 2),
		viewExprWinTime573SnapshotPin("s0", "ordered"),
		viewExprWinTime573SendPin("E3", 3),
		viewExprWinTime573SnapshotPin("s0", "ordered"),
		viewExprWinTime573AdvancePin(viewExprWinTime573Time2500),
		viewExprWinTime573SendPin("E4", 4),
		viewExprWinTime573SnapshotPin("s0", "ordered"),
		viewExprWinTime573AdvancePin(viewExprWinTime573Time3000),
		viewExprWinTime573SendPin("E5", 5),
		viewExprWinTime573SnapshotPin("s0", "ordered"),
		viewExprWinTime573AdvancePin(viewExprWinTime573Time3499),
		viewExprWinTime573SendPin("E6", 6),
		viewExprWinTime573SnapshotPin("s0", "ordered"),
		viewExprWinTime573AdvancePin(viewExprWinTime573Time3500),
		viewExprWinTime573SendPin("E7", 7),
		viewExprWinTime573SnapshotPin("s0", "ordered"),
		viewExprWinTime573AdvancePin(viewExprWinTime573Time10000),
		viewExprWinTime573SendPin("E8", 8),
		viewExprWinTime573SnapshotPin("s0", "ordered"),
		viewExprWinTime573UndeployAllPin(),
	},
}

func loadViewExprWinTime573Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", viewExprWinTime573ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", viewExprWinTime573ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprWinTime573ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprWinTime573ID, err)
	}
	if err := requireViewExprWinTime573Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", viewExprWinTime573ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != viewExprWinTime573ID ||
		metadata.Description != viewExprWinTime573Description ||
		metadata.JavaCommit != viewExprWinTime573JavaCommit ||
		metadata.JavaSource != viewExprWinTime573JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", viewExprWinTime573ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, viewExprWinTime573JavaSources},
		{metadata.JavaRuntimes, viewExprWinTime573JavaRuntimeIDs},
		{metadata.JavaNames, viewExprWinTime573JavaExecutions},
		{metadata.JavaStaticIDs, viewExprWinTime573JavaStaticIDs},
		{metadata.JavaFlags, viewExprWinTime573JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", viewExprWinTime573ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(viewExprWinTime573CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			viewExprWinTime573ID, len(viewExprWinTime573CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireViewExprWinTime573Fields(object, "case", "ordinal", "runtimeId",
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
		spec := viewExprWinTime573CaseSpecs[index]
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
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", viewExprWinTime573ID)
	}
	if err := validateViewExprWinTime573RawSteps(rawSteps); err != nil {
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

// validateViewExprWinTime573RawSteps pins the complete step sequence:
// op field whitelists per step kind plus positional comparison against
// the pinned sequence.
func validateViewExprWinTime573RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(viewExprWinTime573CaseSpecs))
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
			if err := requireViewExprWinTime573Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := viewExprWinTime573CaseSpecFor(marker.Case); !ok {
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
		pins := viewExprWinTime573CaseSteps[currentCase]
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
			if err := requireViewExprWinTime573Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "deployed":
			if err := requireViewExprWinTime573Fields(object, "op", "case", "statement"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deployed marker is not pinned for case %q",
					index, currentCase)
			}
		case "send":
			if err := requireViewExprWinTime573Fields(object, "op", "case", "eventType", "payload"); err != nil {
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
			if err := requireViewExprWinTime573Fields(object, "op", "case", "statement", "mode"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.Mode != pin.mode {
				return fmt.Errorf("scenario step %d snapshot is not pinned for case %q",
					index, currentCase)
			}
		case "advance-time":
			if err := requireViewExprWinTime573Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time is not pinned for case %q",
					index, currentCase)
			}
		case "undeploy-all":
			if err := requireViewExprWinTime573Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(viewExprWinTime573CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(viewExprWinTime573CaseSpecs))
	}
	for index, spec := range viewExprWinTime573CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(viewExprWinTime573CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(viewExprWinTime573CaseSteps[spec.name]))
		}
	}
	return nil
}

func requireViewExprWinTime573Fields(object map[string]json.RawMessage, names ...string) error {
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

// viewExprWinTime573CaseState carries per-case replay state: the deployed
// statement, the listener sequence counter and the delivery records.
type viewExprWinTime573CaseState struct {
	spec       viewExprWinTime573CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	now        time.Time
	statements map[string]*esper.Statement
	deployment *esper.Deployment
	sequence   map[string]uint64
	records    []compat.TraceRecord
}

func viewExprWinTime573StartEnvironment() (*esper.Environment, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewExprWinTime573Bean](env, "SupportBean"); err != nil {
		return nil, err
	}
	return env, nil
}

// runViewExprWinTime573Scenario replays the execution against a fresh
// engine like the Java oracle's per-execution runtime.
func runViewExprWinTime573Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateViewExprWinTime573Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range viewExprWinTime573CaseSpecs {
		records, err := runViewExprWinTime573Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", viewExprWinTime573ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validateViewExprWinTime573Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != viewExprWinTime573ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, viewExprWinTime573ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validateViewExprWinTime573RawSteps(rawSteps)
}

func runViewExprWinTime573Case(ctx context.Context, scenario compat.Scenario, spec viewExprWinTime573CaseSpec) ([]compat.TraceRecord, error) {
	env, err := viewExprWinTime573StartEnvironment()
	if err != nil {
		return nil, err
	}
	now := time.Unix(0, 0).UTC()
	state := &viewExprWinTime573CaseState{
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
					viewExprWinTime573ID, step.Statement)
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
			payload, err := viewExprWinTime573DecodePayload(step)
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
				return nil, fmt.Errorf("%s: parse advance-time %q: %w", viewExprWinTime573ID, step.At, err)
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
			return nil, fmt.Errorf("%s: unsupported step op %q", viewExprWinTime573ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned EPL onto the fluent Go equivalent: the keep-form
// predicate oldest_timestamp > newest_timestamp - 2000 —
// Greater(WindowOldestTimestamp(), Subtract(WindowNewestTimestamp(), 2000))
// — over an `irstream` select (WithOldStream). The trigger-form
// newest-oldest spread predicate used by the expression-batch time window
// is NOT equivalent and must not be substituted.
func (s *viewExprWinTime573CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" || step.Epl != s.spec.epl {
		return fmt.Errorf("%s: case %q deploy is not pinned", viewExprWinTime573ID, s.spec.name)
	}
	if _, ok := s.statements[step.Statement]; ok {
		return fmt.Errorf("%s: label %q is already deployed", viewExprWinTime573ID, step.Statement)
	}
	predicate := esper.Greater[int64](
		esper.WindowOldestTimestamp(),
		esper.Subtract[int64](esper.WindowNewestTimestamp(), esper.Literal(int64(2000))))
	query := esper.From[viewExprWinTime573Bean](s.env, "SupportBean").
		Window(esper.ExpressionWindow(predicate)).
		Query(esper.StatementName("s0"), esper.WithOldStream())
	plan, err := s.env.Build(query)
	if err != nil {
		return err
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	statement := deployment.Statements()[0]
	s.deployment = deployment
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
// pinned theString field. Row order stays engine-native; the in-order
// iterator pins the retained rows oldest-first.
func (s *viewExprWinTime573CaseState) snapshot(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot statement %q was not deployed",
			viewExprWinTime573ID, step.Statement)
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

// viewExprWinTime573DecodePayload converts a scenario send payload into
// the typed host object: a SupportBean struct with the pinned theString
// and intPrimitive fields.
func viewExprWinTime573DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case viewExprWinTime573SupportBeanEvent:
		if err := requireViewExprWinTime573Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean viewExprWinTime573Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", viewExprWinTime573ID, step.EventType)
	}
}
