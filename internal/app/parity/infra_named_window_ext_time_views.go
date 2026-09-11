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

// Parity coverage for the seventh differential slice of InfraNamedWindowViews:
// the externally-timed (event-time) named windows.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 6  InfraExtTimeWindow            java-runtime-c960628819cddf06d0bd
//   - ord 7  InfraExtTimeWindowSceneTwo    java-runtime-aa7fe5003096f5bf1cf8
//   - ord 53 InfraExternallyTimedBatch     java-runtime-3af75eefff14ce16ae81
//
// These executions are driven purely by the timestamp property of the arriving
// event: the engine clock stays pinned at the epoch and no timer exists, so the
// chain advances nothing. The slice pins the sliding threshold (newest - 10000
// + 1 releases expired rows as old data together with the arriving row in one
// invocation), the tombstone-sweep behaviour of deletes, and the
// epoch-referenced batch window where arrivals stay silent until the ten-second
// boundary and the flush releases the batch as new data with the replaced batch
// as old data.
//
// The Java suite attaches an on-delete listener that it never asserts, excluded
// from the trace as in the sibling slices; ord 7 additionally deploys a consume
// consumer that the suite never observes, so only the window statement is
// traced there.
const infraNWRXId = "infra-named-window-ext-time-views"

const infraNWRXDescription = "InfraNamedWindowViews ext-time-slice: the ext_timed(value, 10 sec) sliding window, its projection variant with a separately deployed consumer and the epoch-referenced ext_timed_batch window, all driven purely by event timestamps with the clock pinned at the epoch (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWRXJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWRXSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// Byte-exact EPL pins (Java lines 710-713 for ord 6, 761/767/773/779 for ord 7
// and 3403-3406 for ord 53).
const (
	infraNWRXCreateExt = "@name('create') create window MyWindowETW#ext_timed(value, 10 sec) as MySimpleKeyValueMap"
	infraNWRXInsertExt = "insert into MyWindowETW select theString as key, longBoxed as value from SupportBean"
	infraNWRXSelectExt = "@name('s0') select irstream key, value as value from MyWindowETW"
	infraNWRXDeleteExt = "@name('delete') on SupportMarketDataBean delete from MyWindowETW where symbol = key"

	infraNWRXCreateExtTwo  = "@name('create') @public create window MyWindow.win:ext_timed(value, 10 sec) as select theString as key, longBoxed as value from SupportBean"
	infraNWRXInsertExtTwo  = "insert into MyWindow(key, value) select irstream theString, longBoxed from SupportBean"
	infraNWRXConsumeExtTwo = "@name('consume') select irstream key, value as value from MyWindow"
	infraNWRXDeleteExtTwo  = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"

	infraNWRXCreateExtBatch = "@name('create') create window MyWindowETB#ext_timed_batch(value, 10 sec, 0L) as MySimpleKeyValueMap"
	infraNWRXInsertExtBatch = "insert into MyWindowETB select theString as key, longBoxed as value from SupportBean"
	infraNWRXSelectExtBatch = "@name('s0') select irstream key, value as value from MyWindowETB"
	infraNWRXDeleteExtBatch = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowETB as s1 where s0.symbol = s1.key"
)

type infraNWRXCaseSpec struct {
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
	deploys       []string
	listened      map[string]bool
	snapshots     int
	sendFields    []string
	rowFields     []string
}

var infraNWRXCaseSpecs = []infraNWRXCaseSpec{
	{
		name:          "ext-time-window",
		ordinal:       6,
		runtimeID:     "java-runtime-c960628819cddf06d0bd",
		execution:     "InfraExtTimeWindow",
		windowName:    "MyWindowETW",
		description:   "ext_timed(value, 10 sec) window over the key/value map schema: the sliding threshold newest-9999 releases expired rows as old data together with the arriving row in one invocation, and a delete releases old-only rows",
		createEPL:     infraNWRXCreateExt,
		insertEPL:     infraNWRXInsertExt,
		selectEPL:     infraNWRXSelectExt,
		deleteEPL:     infraNWRXDeleteExt,
		deploys:       []string{"create", "insert", "s0", "delete"},
		listened:      map[string]bool{"create": true, "s0": true},
		snapshots:     4,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "ext-time-window-scene-two",
		ordinal:       7,
		runtimeID:     "java-runtime-aa7fe5003096f5bf1cf8",
		execution:     "InfraExtTimeWindowSceneTwo",
		windowName:    "MyWindow",
		description:   "legacy win:ext_timed(value, 10 sec) projection window over theString/longBoxed with four module deployments and only the window statement listened across the expiry, delete and reinsert waves",
		createEPL:     infraNWRXCreateExtTwo,
		insertEPL:     infraNWRXInsertExtTwo,
		consumeEPL:    infraNWRXConsumeExtTwo,
		deleteEPL:     infraNWRXDeleteExtTwo,
		deploys:       []string{"create", "insert", "consume", "delete"},
		listened:      map[string]bool{"create": true},
		snapshots:     12,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "ext-timed-batch",
		ordinal:       53,
		runtimeID:     "java-runtime-3af75eefff14ce16ae81",
		execution:     "InfraExternallyTimedBatch",
		windowName:    "MyWindowETB",
		description:   "epoch-referenced ext_timed_batch(value, 10 sec, 0L) window over the key/value map schema: arrivals stay silent until the 10 second boundary, the flush releases the batch as new data with the replaced batch as old data, and deletes never touch the pending batch",
		createEPL:     infraNWRXCreateExtBatch,
		insertEPL:     infraNWRXInsertExtBatch,
		selectEPL:     infraNWRXSelectExtBatch,
		deleteEPL:     infraNWRXDeleteExtBatch,
		deploys:       []string{"create", "insert", "s0", "delete"},
		listened:      map[string]bool{"create": true, "s0": true},
		snapshots:     6,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key", "value"},
	},
}

var (
	infraNWRXJavaSources = []string{
		infraNWRXSource,
	}
	infraNWRXJavaRuntimeIDs = []string{
		"java-runtime-c960628819cddf06d0bd",
		"java-runtime-aa7fe5003096f5bf1cf8",
		"java-runtime-3af75eefff14ce16ae81",
	}
	infraNWRXJavaExecutions = []string{
		"InfraExtTimeWindow",
		"InfraExtTimeWindowSceneTwo",
		"InfraExternallyTimedBatch",
	}
	infraNWRXCases = []string{
		"ext-time-window",
		"ext-time-window-scene-two",
		"ext-timed-batch",
	}
)

// infraNWRXBean mirrors SupportBean; each case validates the exact send fields
// it pins (every case sets theString plus one value field, either intBoxed or
// longBoxed, and the unset fields stay at their Go zero value).
type infraNWRXBean struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
	LongBoxed int64  `esper:"longBoxed"`
}

type infraNWRXMarket struct {
	Symbol string `esper:"symbol"`
}

type infraNWRXKVLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

func infraNWRXCaseSpecFor(name string) (infraNWRXCaseSpec, bool) {
	for _, spec := range infraNWRXCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWRXCaseSpec{}, false
}

func loadInfraNWRXScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWRXId)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWRXId, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRXId, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRXId, err)
	}
	if err := requireInfraNWRXFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWRXId, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWRXId ||
		metadata.Description != infraNWRXDescription ||
		metadata.JavaCommit != infraNWRXJavaCommit || metadata.JavaSource != infraNWRXSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWRXId)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWRXId)
	}
	if err := infraNWRXRequireEqual(metadata.JavaRuntimes, infraNWRXJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWRXRequireEqual(metadata.JavaNames, infraNWRXJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWRXId, err)
	}
	if len(rawCases) != len(infraNWRXCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWRXId, len(rawCases), len(infraNWRXCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWRXFields(object,
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
		spec := infraNWRXCaseSpecs[index]
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
		if err := infraNWRXRequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWRXRequireListened(definition.Listened, spec); err != nil {
			return compat.Scenario{}, err
		}
		if definition.IteratorSnapshots != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q iteratorSnapshots = %d, want %d",
				spec.name, definition.IteratorSnapshots, spec.snapshots)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWRXId, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWRXId)
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
			if err := requireInfraNWRXFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWRXFields(object, "op", "case", "statement", "epl"); err != nil {
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
			spec, ok := infraNWRXCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWRXEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWRXFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWRXFields(object, "op", "case", "statement", "mode"); err != nil {
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
			spec, ok := infraNWRXCaseSpecFor(step.Case)
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
			if err := requireInfraNWRXFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "advance-time":
			if err := requireInfraNWRXFields(object, "op", "case", "at"); err != nil {
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
			if _, ok := infraNWRXCaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advances unknown case %q", index, step.Case)
			}
		case "undeploy-all":
			if err := requireInfraNWRXFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	for _, spec := range infraNWRXCaseSpecs {
		if deployCounts[spec.name] != len(spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d deploy steps, want %d",
				spec.name, deployCounts[spec.name], len(spec.deploys))
		}
		if snapshotCounts[spec.name] != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d snapshot steps, want %d",
				spec.name, snapshotCounts[spec.name], spec.snapshots)
		}
		if err := infraNWRXRequireEqual(snapshotModes[spec.name], spec.snapModes, spec.name+" snapshot modes"); err != nil {
			return compat.Scenario{}, err
		}
	}
	var steps []compat.Step
	if err := json.Unmarshal(root["steps"], &steps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", infraNWRXId, err)
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWRXEPLForStep returns the byte-exact EPL of one deploy step.
func infraNWRXEPLForStep(spec infraNWRXCaseSpec, statement string) (string, bool) {
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

func infraNWRXRequireEqual(got, want []string, label string) error {
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

func infraNWRXRequireListened(got []string, spec infraNWRXCaseSpec) error {
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

func requireInfraNWRXFields(object map[string]json.RawMessage, names ...string) error {
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

func runInfraNWRXScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWRXId)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWRXCaseSpecs {
		caseTrace, err := runInfraNWRXCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWRXId, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWRXCase(ctx context.Context, scenario compat.Scenario, spec infraNWRXCaseSpec) ([]compat.TraceRecord, error) {
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
	if _, err := esper.RegisterStruct[infraNWRXBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWRXMarket](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	var windowSchema esper.Schema
	windowSchema, err := esper.RegisterStruct[infraNWRXKVLong](env, spec.windowName)
	if err != nil {
		return nil, err
	}
	timestamp := esper.Field[any, int64]("value")
	var retention esper.WindowSpec
	switch spec.ordinal {
	case 6, 7:
		retention = esper.ExternallyTimed(timestamp, 10*time.Second)
	case 53:
		// The Java EPL pins the reference point explicitly (0L) because the
		// named-window batch path anchors at the epoch.
		retention = esper.ExternallyTimedBatchWithReference(timestamp, 10*time.Second, 0)
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
			plan, err := infraNWRXBuildPlan(env, spec, step.Statement)
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
			payload, err := infraNWRXDecodePayload(spec, step)
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

// infraNWRXBuildPlan maps one scenario deploy statement onto the typed chain.
func infraNWRXBuildPlan(env *esper.Environment, spec infraNWRXCaseSpec, statement string) (esper.Plan, error) {
	switch statement {
	case "create":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		source := esper.From[infraNWRXBean](env, "SupportBean")
		return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
			esper.SetColumn("key", esper.Field[infraNWRXBean, string]("theString")),
			esper.SetColumn("value", esper.Field[infraNWRXBean, int64]("longBoxed")),
		).Query(esper.StatementName("insert")))
	case "s0":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int64]("value")),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "consume":
		// The ord-7 projection window stores longBoxed; the suite never observes
		// this separately deployed consumer (only the window statement is traced).
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int64]("value")),
		).Query(esper.StatementName("consume"), esper.WithOldStream()))

	case "delete":
		return env.Build(esper.OnEvent(esper.From[infraNWRXMarket](env, "SupportMarketDataBean")).
			DeleteFromNamedWindow(spec.windowName,
				esper.Equal[string](esper.NamedWindowField[string]("key"),
					esper.Field[infraNWRXMarket, string]("symbol"))).
			Query(esper.StatementName("delete")))
	default:
		return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
	}
}

func infraNWRXDecodePayload(spec infraNWRXCaseSpec, step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWRXFields(fields, spec.sendFields...); err != nil {
			return nil, fmt.Errorf("case %q SupportBean payload: %w", spec.name, err)
		}
		var bean infraNWRXBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWRXFields(fields, "symbol"); err != nil {
			return nil, fmt.Errorf("SupportMarketDataBean payload: %w", err)
		}
		var market infraNWRXMarket
		if err := json.Unmarshal(step.Payload, &market); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return market, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
