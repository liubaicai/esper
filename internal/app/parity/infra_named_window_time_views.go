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

// Parity coverage for the sixth differential slice of InfraNamedWindowViews:
// the time-driven named windows (sliding time(10 sec), firsttime(10 sec) and a
// bean-backed time(10 sec) window under a SupportBean_A delete trigger).
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 4 InfraTimeWindowSceneTwo       java-runtime-879d6ad8aee378657a02
//   - ord 5 InfraTimeFirstWindow          java-runtime-5f5a65dd5f72c8e24619
//   - ord 8 InfraExtTimeWindowSceneThree  java-runtime-a6bfdd3389c077127861
//
// Unlike the sibling slices these executions run under virtual time: the chain
// replays `advance-time` steps through the engine clock and stamps every record
// with the clock value at delivery. The slice pins sliding-window expiry at the
// exact boundary (old-only deliveries at 10000 and 36000 while 35999 stays
// silent), the separately deployed irstream consumer of ord 4, firsttime's
// deploy-anchored close (the window stops admitting at 11000 without delivering
// anything, and retained rows survive until deleted) and the bean-backed
// window's delete-trigger plus expiry timeline.
//
// The Java suite attaches an on-delete listener for ords 4 and 5 but never
// asserts its payload, so it is excluded from the trace on both sides as in the
// sibling slices; ord 8 attaches none.
const infraNWRTId = "infra-named-window-time-views"

const infraNWRTDescription = "InfraNamedWindowViews time-slice: the projection time(10 sec) window with a separately deployed irstream consumer, the firsttime(10 sec) map window and the bean-backed time(10 sec) window over a SupportBean_A delete trigger, captured from listener callbacks and ordered window iterator snapshots under virtual time (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWRTJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWRTSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// Byte-exact EPL pins (Java lines 534/540/546/552 for ord 4, 656-659 for ord 5
// and 865-868 for ord 8).
const (
	infraNWRTCreateTimeTwo  = "@name('create') @public create window MyWindow#time(10 sec) as select theString as key, intBoxed as value from SupportBean"
	infraNWRTInsertTimeTwo  = "@name('insert') insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"
	infraNWRTConsumeTimeTwo = "@name('consume') select irstream key, value as value from MyWindow"
	infraNWRTDeleteTimeTwo  = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"

	infraNWRTCreateFirstTime = "@name('create') create window MyWindowTFW#firsttime(10 sec) as MySimpleKeyValueMap"
	infraNWRTInsertFirstTime = "insert into MyWindowTFW select theString as key, longBoxed as value from SupportBean"
	infraNWRTSelectFirstTime = "@name('s0') select irstream key, value as value from MyWindowTFW"
	infraNWRTDeleteFirstTime = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowTFW as s1 where s0.symbol = s1.key"

	infraNWRTCreateTimeThree = "@public create window ABCWin.win:time(10 sec) as SupportBean"
	infraNWRTInsertTimeThree = "insert into ABCWin select * from SupportBean"
	infraNWRTSelectTimeThree = "@Name('s0') select irstream * from ABCWin"
	infraNWRTDeleteTimeThree = "on SupportBean_A delete from ABCWin where theString = id"
)

// infraNWRTSendKind selects the SupportBean payload shape and the projected row
// fields of a case.
type infraNWRTSendKind int

const (
	infraNWRTTimeInt infraNWRTSendKind = iota
	infraNWRTFirstTimeLong
	infraNWRTTimeBean
)

type infraNWRTCaseSpec struct {
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
	beanTrigger   bool
	deploys       []string
	listened      map[string]bool
	snapshots     int
	sendKind      infraNWRTSendKind
	sendFields    []string
	rowFields     []string
}

var infraNWRTCaseSpecs = []infraNWRTCaseSpec{
	{
		name:          "time-window-scene-two",
		ordinal:       4,
		runtimeID:     "java-runtime-879d6ad8aee378657a02",
		execution:     "InfraTimeWindowSceneTwo",
		windowName:    "MyWindow",
		description:   "projection time(10 sec) window over theString/intBoxed with four module deployments; the window statement and the consume consumer both carry irstream, expiry at t=10000 and t=36000 delivers old-only rows and equal-time advances stay silent",
		createEPL:     infraNWRTCreateTimeTwo,
		insertEPL:     infraNWRTInsertTimeTwo,
		deleteEPL:     infraNWRTDeleteTimeTwo,
		consumeEPL:    infraNWRTConsumeTimeTwo,
		deploys:       []string{"create", "insert", "consume", "delete"},
		listened:      map[string]bool{"create": true, "consume": true},
		snapshots:     10,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		sendKind:      infraNWRTTimeInt,
		sendFields:    []string{"theString", "intBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "time-first-window",
		ordinal:       5,
		runtimeID:     "java-runtime-5f5a65dd5f72c8e24619",
		execution:     "InfraTimeFirstWindow",
		windowName:    "MyWindowTFW",
		description:   "firsttime(10 sec) window over the key/value map schema anchored at deploy time (clock 1000): it closes silently at 11000, later sends are dropped while retained rows survive until deleted",
		createEPL:     infraNWRTCreateFirstTime,
		insertEPL:     infraNWRTInsertFirstTime,
		selectEPL:     infraNWRTSelectFirstTime,
		deleteEPL:     infraNWRTDeleteFirstTime,
		deploys:       []string{"create", "insert", "s0", "delete"},
		listened:      map[string]bool{"create": true, "s0": true},
		snapshots:     7,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		sendKind:      infraNWRTFirstTimeLong,
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "ext-time-window-scene-three",
		ordinal:       8,
		runtimeID:     "java-runtime-a6bfdd3389c077127861",
		execution:     "InfraExtTimeWindowSceneThree",
		windowName:    "ABCWin",
		description:   "bean-backed win:time(10 sec) window with a SupportBean_A delete trigger and a wildcard irstream consumer projected to theString; expiry at the 13000 boundary releases the retained row",
		createEPL:     infraNWRTCreateTimeThree,
		insertEPL:     infraNWRTInsertTimeThree,
		selectEPL:     infraNWRTSelectTimeThree,
		deleteEPL:     infraNWRTDeleteTimeThree,
		deploys:       []string{"create", "insert", "s0", "delete"},
		listened:      map[string]bool{"s0": true},
		snapshots:     6,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "s0",
		beanTrigger:   true,
		sendKind:      infraNWRTTimeBean,
		sendFields:    []string{"theString"},
		rowFields:     []string{"theString"},
	},
}

var (
	infraNWRTJavaSources = []string{
		infraNWRTSource,
	}
	infraNWRTJavaRuntimeIDs = []string{
		"java-runtime-879d6ad8aee378657a02",
		"java-runtime-5f5a65dd5f72c8e24619",
		"java-runtime-a6bfdd3389c077127861",
	}
	infraNWRTJavaExecutions = []string{
		"InfraTimeWindowSceneTwo",
		"InfraTimeFirstWindow",
		"InfraExtTimeWindowSceneThree",
	}
	infraNWRTCases = []string{
		"time-window-scene-two",
		"time-first-window",
		"ext-time-window-scene-three",
	}
)

// infraNWRTBean mirrors SupportBean; each case validates the exact send fields
// it pins (every case sets theString plus one value field, either intBoxed or
// longBoxed, and the unset fields stay at their Go zero value).
type infraNWRTBean struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
	LongBoxed int64  `esper:"longBoxed"`
}

type infraNWRTMarket struct {
	Symbol string `esper:"symbol"`
}

// infraNWRTA mirrors SupportBean_A, the ord-8 delete trigger (id compared to
// the window row's theString).
type infraNWRTA struct {
	ID string `esper:"id"`
}

type infraNWRTKVLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type infraNWRTKVInt struct {
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

func infraNWRTCaseSpecFor(name string) (infraNWRTCaseSpec, bool) {
	for _, spec := range infraNWRTCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWRTCaseSpec{}, false
}

func loadInfraNWRTScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWRTId)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWRTId, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRTId, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRTId, err)
	}
	if err := requireInfraNWRTFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWRTId, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWRTId ||
		metadata.Description != infraNWRTDescription ||
		metadata.JavaCommit != infraNWRTJavaCommit || metadata.JavaSource != infraNWRTSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWRTId)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWRTId)
	}
	if err := infraNWRTRequireEqual(metadata.JavaRuntimes, infraNWRTJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWRTRequireEqual(metadata.JavaNames, infraNWRTJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWRTId, err)
	}
	if len(rawCases) != len(infraNWRTCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWRTId, len(rawCases), len(infraNWRTCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWRTFields(object,
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
		spec := infraNWRTCaseSpecs[index]
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
		if err := infraNWRTRequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWRTRequireListened(definition.Listened, spec); err != nil {
			return compat.Scenario{}, err
		}
		if definition.IteratorSnapshots != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q iteratorSnapshots = %d, want %d",
				spec.name, definition.IteratorSnapshots, spec.snapshots)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWRTId, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWRTId)
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
			if err := requireInfraNWRTFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWRTFields(object, "op", "case", "statement", "epl"); err != nil {
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
			spec, ok := infraNWRTCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWRTEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWRTFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWRTFields(object, "op", "case", "statement", "mode"); err != nil {
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
			spec, ok := infraNWRTCaseSpecFor(step.Case)
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
			if err := requireInfraNWRTFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "advance-time":
			if err := requireInfraNWRTFields(object, "op", "case", "at"); err != nil {
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
			if _, ok := infraNWRTCaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advances unknown case %q", index, step.Case)
			}
		case "undeploy-all":
			if err := requireInfraNWRTFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	for _, spec := range infraNWRTCaseSpecs {
		if deployCounts[spec.name] != len(spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d deploy steps, want %d",
				spec.name, deployCounts[spec.name], len(spec.deploys))
		}
		if snapshotCounts[spec.name] != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d snapshot steps, want %d",
				spec.name, snapshotCounts[spec.name], spec.snapshots)
		}
		if err := infraNWRTRequireEqual(snapshotModes[spec.name], spec.snapModes, spec.name+" snapshot modes"); err != nil {
			return compat.Scenario{}, err
		}
	}
	var steps []compat.Step
	if err := json.Unmarshal(root["steps"], &steps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", infraNWRTId, err)
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWRTEPLForStep returns the byte-exact EPL of one deploy step.
func infraNWRTEPLForStep(spec infraNWRTCaseSpec, statement string) (string, bool) {
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

func infraNWRTRequireEqual(got, want []string, label string) error {
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

func infraNWRTRequireListened(got []string, spec infraNWRTCaseSpec) error {
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

func requireInfraNWRTFields(object map[string]json.RawMessage, names ...string) error {
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

func runInfraNWRTScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWRTId)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWRTCaseSpecs {
		caseTrace, err := runInfraNWRTCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWRTId, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWRTCase(ctx context.Context, scenario compat.Scenario, spec infraNWRTCaseSpec) ([]compat.TraceRecord, error) {
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
	if _, err := esper.RegisterStruct[infraNWRTBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWRTMarket](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWRTA](env, "SupportBean_A"); err != nil {
		return nil, err
	}
	var windowSchema esper.Schema
	var err error
	switch spec.sendKind {
	case infraNWRTTimeInt:
		windowSchema, err = esper.RegisterStruct[infraNWRTKVInt](env, spec.windowName)
	case infraNWRTTimeBean:
		windowSchema, err = esper.RegisterStruct[infraNWRTBean](env, spec.windowName)
	default:
		windowSchema, err = esper.RegisterStruct[infraNWRTKVLong](env, spec.windowName)
	}
	if err != nil {
		return nil, err
	}
	var retention esper.WindowSpec
	switch spec.ordinal {
	case 4, 8:
		retention = esper.TimeWindow(10 * time.Second)
	case 5:
		retention = esper.FirstTime(10 * time.Second)
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
			plan, err := infraNWRTBuildPlan(env, spec, step.Statement)
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
			payload, err := infraNWRTDecodePayload(spec, step)
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

// infraNWRTBuildPlan maps one scenario deploy statement onto the typed chain.
func infraNWRTBuildPlan(env *esper.Environment, spec infraNWRTCaseSpec, statement string) (esper.Plan, error) {
	switch statement {
	case "create":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		source := esper.From[infraNWRTBean](env, "SupportBean")
		switch spec.sendKind {
		case infraNWRTTimeBean:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(
				spec.windowName, esper.CopyMatchingFields(),
			).Query(esper.StatementName("insert")))
		case infraNWRTTimeInt:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWRTBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWRTBean, int]("intBoxed")),
			).Query(esper.StatementName("insert")))
		default:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWRTBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWRTBean, int64]("longBoxed")),
			).Query(esper.StatementName("insert")))
		}
	case "s0":
		if spec.sendKind == infraNWRTTimeBean {
			return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
				esper.Alias("theString", esper.Field[any, string]("theString")),
			).Query(esper.StatementName("s0"), esper.WithOldStream()))
		}
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int64]("value")),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "consume":
		// The ord-4 window column is the Integer intBoxed projection, so the
		// consumer reads it as int.
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int]("value")),
		).Query(esper.StatementName("consume"), esper.WithOldStream()))
	case "delete":
		if spec.beanTrigger {
			return env.Build(esper.OnEvent(esper.From[infraNWRTA](env, "SupportBean_A")).
				DeleteFromNamedWindow(spec.windowName,
					esper.Equal[string](esper.NamedWindowField[string]("theString"),
						esper.Field[infraNWRTA, string]("id"))).
				Query(esper.StatementName("delete")))
		}
		return env.Build(esper.OnEvent(esper.From[infraNWRTMarket](env, "SupportMarketDataBean")).
			DeleteFromNamedWindow(spec.windowName,
				esper.Equal[string](esper.NamedWindowField[string]("key"),
					esper.Field[infraNWRTMarket, string]("symbol"))).
			Query(esper.StatementName("delete")))
	default:
		return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
	}
}

func infraNWRTDecodePayload(spec infraNWRTCaseSpec, step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWRTFields(fields, spec.sendFields...); err != nil {
			return nil, fmt.Errorf("case %q SupportBean payload: %w", spec.name, err)
		}
		var bean infraNWRTBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_A":
		if err := requireInfraNWRTFields(fields, "id"); err != nil {
			return nil, fmt.Errorf("SupportBean_A payload: %w", err)
		}
		var bean infraNWRTA
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWRTFields(fields, "symbol"); err != nil {
			return nil, fmt.Errorf("SupportMarketDataBean payload: %w", err)
		}
		var market infraNWRTMarket
		if err := json.Unmarshal(step.Payload, &market); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return market, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
