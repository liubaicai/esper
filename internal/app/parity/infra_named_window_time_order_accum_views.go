package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the eighth differential slice of InfraNamedWindowViews:
// the time-ordered and accumulating windows under virtual time.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 9  InfraTimeOrderWindow        java-runtime-320f1b03eddb24520308
//   - ord 10 InfraTimeOrderSceneTwo      java-runtime-ed5bfc247d088a603224
//   - ord 14 InfraTimeAccum              java-runtime-d74b2c9c614b26973637
//   - ord 15 InfraTimeAccumSceneTwo      java-runtime-8b4e2e7d81bb990c7d72
//
// These executions advance the virtual clock, and the expiry waves fire during
// the advance with no send. The slice pins the sliding time_order rule (rows are
// retained while value >= clock - 9999 and released old-only once the clock
// reaches value + 10000, with ascending-value iterators and immediate releases
// on delete), the pass-through of a stale arriving row as one callback carrying
// both streams, and the time_accum timeline where every arrival is delivered
// immediately, re-arms the flush to its own arrival plus ten seconds, and the
// flush releases all retained rows as old data in ONE callback in insertion
// order (deleting the newest row re-anchors the flush to the newest retained row's
// stored arrival plus ten seconds, deleting the last row cancels the timer).
//
// Delete and consume listeners are never asserted by the suite and stay
// excluded from the trace, as in the sibling slices.
const infraNWRAId = "infra-named-window-time-order-accum-views"

const infraNWRADescription = "InfraNamedWindowViews time-order/accum-slice: the time_order(value, 10 sec) sliding window and its ext:time_order projection variant with pass-through rows, plus the time_accum map and projection windows whose multi-row expiry bursts release in insertion order under virtual time (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWRAJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWRASource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// Byte-exact EPL pins (Java lines 923-926 for ord 9, 980/984/988 for ord 10,
// 1306-1309 for ord 14 and 1425/1429/1433/1437 for ord 15).
const (
	infraNWRACreateTimeOrder = "@name('create') @public create window MyWindowTOW#time_order(value, 10 sec) as MySimpleKeyValueMap"
	infraNWRAInsertTimeOrder = "insert into MyWindowTOW select theString as key, longBoxed as value from SupportBean"
	infraNWRASelectTimeOrder = "@name('s0') select irstream key, value as value from MyWindowTOW"
	infraNWRADeleteTimeOrder = "@name('delete') on SupportMarketDataBean delete from MyWindowTOW where symbol = key"

	infraNWRACreateTimeOrderTwo = "@name('create') @public create window MyWindow.ext:time_order(value, 10) as select theString as key, longBoxed as value from SupportBean"
	infraNWRAInsertTimeOrderTwo = "insert into MyWindow(key, value) select irstream theString, longBoxed from SupportBean"
	infraNWRADeleteTimeOrderTwo = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"

	infraNWRACreateTimeAccum = "@name('create') create window MyWindowTA#time_accum(10 sec) as MySimpleKeyValueMap"
	infraNWRAInsertTimeAccum = "insert into MyWindowTA select theString as key, longBoxed as value from SupportBean"
	infraNWRASelectTimeAccum = "@name('s0') select irstream key, value as value from MyWindowTA"
	infraNWRADeleteTimeAccum = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowTA as s1 where s0.symbol = s1.key"

	infraNWRACreateTimeAccumTwo  = "@name('create') @public create window MyWindow.win:time_accum(10 sec) as select theString as key, intBoxed as value from SupportBean"
	infraNWRAInsertTimeAccumTwo  = "@name('insert') insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"
	infraNWRAConsumeTimeAccumTwo = "@name('consume') select irstream key, value as value from MyWindow"
	infraNWRADeleteTimeAccumTwo  = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"
)

type infraNWRACaseSpec struct {
	name          string
	ordinal       int
	runtimeID     string
	execution     string
	windowName    string
	description   string
	createEPL     string
	insertEPL     string
	selectEPL     string
	consumeEPL    string
	deleteEPL     string
	snapModes     []string
	snapStatement string
	timeOrdered   bool
	accum         bool
	accumInt      bool
	deploys       []string
	listened      map[string]bool
	snapshots     int
	sendFields    []string
	rowFields     []string
}

var infraNWRACaseSpecs = []infraNWRACaseSpec{
	{
		name:          "time-order-window",
		ordinal:       9,
		runtimeID:     "java-runtime-320f1b03eddb24520308",
		execution:     "InfraTimeOrderWindow",
		windowName:    "MyWindowTOW",
		description:   "time_order(value, 10 sec) window over the key/value map schema: rows are retained while value >= clock - 9999 and released old-only by timer waves during advance-time, with deletes releasing immediately and ascending-value iterators",
		createEPL:     infraNWRACreateTimeOrder,
		insertEPL:     infraNWRAInsertTimeOrder,
		selectEPL:     infraNWRASelectTimeOrder,
		deleteEPL:     infraNWRADeleteTimeOrder,
		deploys:       []string{"create", "insert", "s0", "delete"},
		listened:      map[string]bool{"create": true, "s0": true},
		snapshots:     4,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		accumInt:      false,
		timeOrdered:   true,
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "time-order-scene-two",
		ordinal:       10,
		runtimeID:     "java-runtime-ed5bfc247d088a603224",
		execution:     "InfraTimeOrderSceneTwo",
		windowName:    "MyWindow",
		description:   "legacy ext:time_order(value, 10) projection window over theString/longBoxed with three module deployments and only the window statement listened; a stale arriving row passes through in one callback carrying both streams",
		createEPL:     infraNWRACreateTimeOrderTwo,
		insertEPL:     infraNWRAInsertTimeOrderTwo,
		deleteEPL:     infraNWRADeleteTimeOrderTwo,
		deploys:       []string{"create", "insert", "delete"},
		listened:      map[string]bool{"create": true},
		snapshots:     10,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		timeOrdered:   true,
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key"},
	},
	{
		name:          "time-accum",
		ordinal:       14,
		runtimeID:     "java-runtime-d74b2c9c614b26973637",
		execution:     "InfraTimeAccum",
		windowName:    "MyWindowTA",
		description:   "time_accum(10 sec) window over the key/value map schema: arrivals deliver immediately and re-arm the flush to their arrival plus ten seconds, whose bursts release all retained rows as old data in insertion order, with newest-row deletes re-arming and last-row deletes cancelling the timer",
		createEPL:     infraNWRACreateTimeAccum,
		insertEPL:     infraNWRAInsertTimeAccum,
		selectEPL:     infraNWRASelectTimeAccum,
		deleteEPL:     infraNWRADeleteTimeAccum,
		deploys:       []string{"create", "insert", "s0", "delete"},
		listened:      map[string]bool{"create": true, "s0": true},
		snapshots:     8,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		accum:         true,
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "time-accum-scene-two",
		ordinal:       15,
		runtimeID:     "java-runtime-8b4e2e7d81bb990c7d72",
		execution:     "InfraTimeAccumSceneTwo",
		windowName:    "MyWindow",
		description:   "legacy win:time_accum(10 sec) projection window over theString/intBoxed with four module deployments and only the window statement listened across the delete, re-arm and two-row burst timeline",
		createEPL:     infraNWRACreateTimeAccumTwo,
		insertEPL:     infraNWRAInsertTimeAccumTwo,
		consumeEPL:    infraNWRAConsumeTimeAccumTwo,
		deleteEPL:     infraNWRADeleteTimeAccumTwo,
		deploys:       []string{"create", "insert", "consume", "delete"},
		listened:      map[string]bool{"create": true},
		snapshots:     15,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		accum:         true,
		accumInt:      true,
		sendFields:    []string{"theString", "intBoxed"},
		rowFields:     []string{"key", "value"},
	},
}

var (
	infraNWRAJavaSources = []string{
		infraNWRASource,
	}
	infraNWRAJavaRuntimeIDs = []string{
		"java-runtime-320f1b03eddb24520308",
		"java-runtime-ed5bfc247d088a603224",
		"java-runtime-d74b2c9c614b26973637",
		"java-runtime-8b4e2e7d81bb990c7d72",
	}
	infraNWRAJavaExecutions = []string{
		"InfraTimeOrderWindow",
		"InfraTimeOrderSceneTwo",
		"InfraTimeAccum",
		"InfraTimeAccumSceneTwo",
	}
	infraNWRACases = []string{
		"time-order-window",
		"time-order-scene-two",
		"time-accum",
		"time-accum-scene-two",
	}
)

// infraNWRABean mirrors SupportBean; each case validates the exact send fields
// it pins (every case sets theString plus one value field, either intBoxed or
// longBoxed, and the unset fields stay at their Go zero value).
type infraNWRABean struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
	LongBoxed int64  `esper:"longBoxed"`
}

type infraNWRAMarket struct {
	Symbol string `esper:"symbol"`
}

type infraNWRAKVLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type infraNWRAKVInt struct {
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

func infraNWRACaseSpecFor(name string) (infraNWRACaseSpec, bool) {
	for _, spec := range infraNWRACaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWRACaseSpec{}, false
}

func loadInfraNWRAScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWRAId)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWRAId, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRAId, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRAId, err)
	}
	if err := requireInfraNWRAFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version      string   `json:"version"`
		ID           string   `json:"id"`
		Description  string   `json:"description"`
		JavaCommit   string   `json:"javaCommit"`
		JavaSource   string   `json:"javaSource"`
		JavaRuntimes []string `json:"javaRuntimes"`
		JavaNames    []string `json:"javaNames"`
		JavaFlags    []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWRAId, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWRAId ||
		metadata.Description != infraNWRADescription ||
		metadata.JavaCommit != infraNWRAJavaCommit || metadata.JavaSource != infraNWRASource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWRAId)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWRAId)
	}
	if err := infraNWRARequireEqual(metadata.JavaRuntimes, infraNWRAJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWRARequireEqual(metadata.JavaNames, infraNWRAJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWRAId, err)
	}
	if len(rawCases) != len(infraNWRACaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWRAId, len(rawCases), len(infraNWRACaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWRAFields(object,
			"case", "ordinal", "runtimeId", "executionName", "description", "createEpl",
			"insertEpl", "s0Epl", "consumeEpl", "deleteEpl", "deploys", "listened", "iteratorSnapshots"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case              string   `json:"case"`
			Ordinal           int      `json:"ordinal"`
			RuntimeID         string   `json:"runtimeId"`
			ExecutionName     string   `json:"executionName"`
			Description       string   `json:"description"`
			CreateEPL         string   `json:"createEpl"`
			InsertEPL         string   `json:"insertEpl"`
			SelectEPL         string   `json:"s0Epl"`
			ConsumeEPL        string   `json:"consumeEpl"`
			DeleteEPL         string   `json:"deleteEpl"`
			Deploys           []string `json:"deploys"`
			Listened          []string `json:"listened"`
			IteratorSnapshots int      `json:"iteratorSnapshots"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := infraNWRACaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
		}
		if definition.Description != spec.description {
			return compat.Scenario{}, fmt.Errorf("scenario case %q description does not match the pinned slice description", spec.name)
		}
		if definition.CreateEPL != spec.createEPL || definition.InsertEPL != spec.insertEPL ||
			definition.SelectEPL != spec.selectEPL || definition.ConsumeEPL != spec.consumeEPL ||
			definition.DeleteEPL != spec.deleteEPL {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source (create=%t insert=%t select=%t consume=%t delete=%t)",
				spec.name, definition.CreateEPL == spec.createEPL, definition.InsertEPL == spec.insertEPL,
				definition.SelectEPL == spec.selectEPL, definition.ConsumeEPL == spec.consumeEPL,
				definition.DeleteEPL == spec.deleteEPL)
		}
		if err := infraNWRARequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWRARequireListened(definition.Listened, spec); err != nil {
			return compat.Scenario{}, err
		}
		if definition.IteratorSnapshots != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q iteratorSnapshots = %d, want %d",
				spec.name, definition.IteratorSnapshots, spec.snapshots)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWRAId, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWRAId)
	}
	deployCounts := map[string]int{}
	snapshotCounts := map[string]int{}
	snapshotModes := map[string][]string{}
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		switch operation {
		case "case":
			if err := requireInfraNWRAFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWRAFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
				EPL       string `json:"epl"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWRACaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWRAEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWRAFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWRAFields(object, "op", "case", "statement", "mode"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
				Mode      string `json:"mode"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWRACaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshots unknown case %q", index, step.Case)
			}
			if step.Statement != spec.snapStatement {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot statement %q must be %q for case %q",
					index, step.Statement, spec.snapStatement, step.Case)
			}
			snapshotModes[step.Case] = append(snapshotModes[step.Case], step.Mode)
			snapshotCounts[step.Case]++
		case "undeploy":
			if err := requireInfraNWRAFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "advance-time":
			if err := requireInfraNWRAFields(object, "op", "case", "at"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
				At   string `json:"at"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := time.Parse(time.RFC3339Nano, step.At); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advance-time at %q: %w", index, step.At, err)
			}
			// Java parses Instants (ISO_INSTANT), which rejects numeric offsets;
			// require the UTC form so both sides accept the same spellings.
			if !strings.HasSuffix(step.At, "Z") {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advance-time at %q must be in UTC (Z) form", index, step.At)
			}
			if _, ok := infraNWRACaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advances unknown case %q", index, step.Case)
			}
		case "undeploy-all":
			if err := requireInfraNWRAFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	for _, spec := range infraNWRACaseSpecs {
		if deployCounts[spec.name] != len(spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d deploy steps, want %d",
				spec.name, deployCounts[spec.name], len(spec.deploys))
		}
		if snapshotCounts[spec.name] != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d snapshot steps, want %d",
				spec.name, snapshotCounts[spec.name], spec.snapshots)
		}
		if err := infraNWRARequireEqual(snapshotModes[spec.name], spec.snapModes, spec.name+" snapshot modes"); err != nil {
			return compat.Scenario{}, err
		}
	}
	var steps []compat.Step
	if err := json.Unmarshal(root["steps"], &steps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", infraNWRAId, err)
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWRAEPLForStep returns the byte-exact EPL of one deploy step.
func infraNWRAEPLForStep(spec infraNWRACaseSpec, statement string) (string, bool) {
	switch statement {
	case "create":
		return spec.createEPL, true
	case "insert":
		return spec.insertEPL, true
	case "s0":
		if spec.selectEPL == "" {
			return "", false
		}
		return spec.selectEPL, true
	case "consume":
		if spec.consumeEPL == "" {
			return "", false
		}
		return spec.consumeEPL, true
	case "delete":
		if spec.deleteEPL == "" {
			return "", false
		}
		return spec.deleteEPL, true
	default:
		return "", false
	}
}

func infraNWRARequireEqual(got, want []string, label string) error {
	if len(got) != len(want) {
		return fmt.Errorf("%s = %v, want %v", label, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			return fmt.Errorf("%s = %v, want %v", label, got, want)
		}
	}
	return nil
}

func infraNWRARequireListened(got []string, spec infraNWRACaseSpec) error {
	if len(got) != len(spec.listened) {
		return fmt.Errorf("scenario case %q listened = %v, want %d statements", spec.name, got, len(spec.listened))
	}
	for _, name := range got {
		if !spec.listened[name] {
			return fmt.Errorf("scenario case %q listens to %q which is not in the pinned listener set", spec.name, name)
		}
	}
	return nil
}

func requireInfraNWRAFields(object map[string]json.RawMessage, names ...string) error {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	for name := range object {
		if !allowed[name] {
			return fmt.Errorf("unexpected field %q", name)
		}
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("missing field %q", name)
		}
	}
	return nil
}

func runInfraNWRAScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWRAId)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWRACaseSpecs {
		caseTrace, err := runInfraNWRACase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWRAId, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWRACase(ctx context.Context, scenario compat.Scenario, spec infraNWRACaseSpec) ([]compat.TraceRecord, error) {
	// Virtual clock: starts at the epoch and moves only on advance-time steps,
	// so every record carries the clock value at delivery.
	now := time.Unix(0, 0).UTC()
	sequence := map[string]uint64{}
	var records []compat.TraceRecord
	recordListener := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		records = append(records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(now),
			New:       projectRecords(compat.NormalizeResults(batch.New), spec.rowFields),
			Old:       projectRecords(compat.NormalizeResults(batch.Old), spec.rowFields),
		})
	}

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWRABean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWRAMarket](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	var windowSchema esper.Schema
	var err error
	if spec.accumInt {
		windowSchema, err = esper.RegisterStruct[infraNWRAKVInt](env, spec.windowName)
	} else {
		windowSchema, err = esper.RegisterStruct[infraNWRAKVLong](env, spec.windowName)
	}
	if err != nil {
		return nil, err
	}
	timestamp := esper.Field[any, int64]("value")
	var retention esper.WindowSpec
	switch {
	case spec.timeOrdered:
		retention = esper.TimeOrder(timestamp, 10*time.Second)
	case spec.accum:
		retention = esper.TimeAccum(10 * time.Second)
	default:
		return nil, fmt.Errorf("unexpected ordinal %d", spec.ordinal)
	}
	if _, err := esper.CreateNamedWindow(env, spec.windowName, windowSchema,
		esper.NamedWindowRetention(retention)); err != nil {
		return nil, err
	}

	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(spec.runtimeID),
		esper.WithStartTime(now),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	deployments := map[string]*esper.Deployment{}
	statements := map[string]*esper.Statement{}
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
			plan, err := infraNWRABuildPlan(env, spec, step.Statement)
			if err != nil {
				return nil, err
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return nil, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			for _, statement := range deployment.Statements() {
				if statement.Name() == step.Statement {
					statements[step.Statement] = statement
					deployments[step.Statement] = deployment
				}
			}
			if spec.listened[step.Statement] {
				statement, ok := statements[step.Statement]
				if !ok {
					return nil, fmt.Errorf("deploy %q did not register the statement", step.Statement)
				}
				name := step.Statement
				if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					recordListener(name, batch)
					return nil
				}); err != nil {
					return nil, err
				}
			}
		case "send":
			payload, err := infraNWRADecodePayload(spec, step)
			if err != nil {
				return nil, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "snapshot":
			statement, ok := statements[step.Statement]
			if !ok {
				return nil, fmt.Errorf("snapshot statement %q was not deployed", step.Statement)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return nil, err
			}
			rows := projectRecords(compat.NormalizeResults(result.Batch.New), spec.rowFields)
			// Shared protocol hook from the sibling chains: multi-row
			// HashMap-backed snapshots mark their steps "any" and both sides
			// emit canonical order. Every snapshot in this slice is ordered
			// (the length views iterate insertion-ordered collections), so the
			// branch stays unreached here.
			if step.Mode == "any" {
				sortRowsCanonical(rows)
			}
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "snapshot",
				Statement: step.Statement,
				Sequence:  0,
				Time:      compat.FormatTraceTime(now),
				New:       rows,
			})
		case "undeploy":
			deployment, ok := deployments[step.Statement]
			if !ok {
				return nil, fmt.Errorf("no deployment for statement %q", step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return nil, err
			}
			delete(deployments, step.Statement)
			delete(statements, step.Statement)
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("advance-time %q: %w", step.At, err)
			}
			// The clock moves first: expiry deliveries produced by the advance
			// carry the boundary time, matching the Java trace.
			now = at.UTC()
			if err := engine.AdvanceTime(ctx, now); err != nil {
				return nil, err
			}
		case "undeploy-all":
			// The Java suite tears the module down at this point; teardown
			// emits no trace records and the next case uses a fresh
			// environment, so nothing further is replayed here.
		default:
			return nil, fmt.Errorf("unsupported step op %q", step.Op)
		}
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("case %q produced no records", spec.name)
	}
	return records, nil
}

// infraNWRABuildPlan maps one scenario deploy statement onto the typed chain.
func infraNWRABuildPlan(env *esper.Environment, spec infraNWRACaseSpec, statement string) (esper.Plan, error) {
	switch statement {
	case "create":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		source := esper.From[infraNWRABean](env, "SupportBean")
		if spec.accumInt {
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWRABean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWRABean, int]("intBoxed")),
			).Query(esper.StatementName("insert")))
		}
		return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
			esper.SetColumn("key", esper.Field[infraNWRABean, string]("theString")),
			esper.SetColumn("value", esper.Field[infraNWRABean, int64]("longBoxed")),
		).Query(esper.StatementName("insert")))
	case "s0":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int64]("value")),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "consume":
		// The scene-two projection window stores intBoxed; the suite never
		// observes this separately deployed consumer (only the window statement
		// is traced).
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int]("value")),
		).Query(esper.StatementName("consume"), esper.WithOldStream()))

	case "delete":
		return env.Build(esper.OnEvent(esper.From[infraNWRAMarket](env, "SupportMarketDataBean")).
			DeleteFromNamedWindow(spec.windowName,
				esper.Equal[string](esper.NamedWindowField[string]("key"),
					esper.Field[infraNWRAMarket, string]("symbol"))).
			Query(esper.StatementName("delete")))
	default:
		return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
	}
}

func infraNWRADecodePayload(spec infraNWRACaseSpec, step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWRAFields(fields, spec.sendFields...); err != nil {
			return nil, fmt.Errorf("case %q SupportBean payload: %w", spec.name, err)
		}
		var bean infraNWRABean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWRAFields(fields, "symbol"); err != nil {
			return nil, fmt.Errorf("SupportMarketDataBean payload: %w", err)
		}
		var market infraNWRAMarket
		if err := json.Unmarshal(step.Payload, &market); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return market, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
