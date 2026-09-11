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

// Parity coverage for the fourth differential slice of InfraNamedWindowViews:
// the length and first-length data windows.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 11 InfraLengthWindow          java-runtime-d931bfbbbaa3ea632c2b
//   - ord 12 InfraLengthWindowSceneTwo  java-runtime-793f6ec7608e25556803
//   - ord 13 InfraLengthFirstWindow     java-runtime-b53dbf8f541e79537f32
//   - ord 25 InfraLengthWindowSceneThree java-runtime-c656dd0eeae6618a4162
//
// The slice pins two capacity rules: a length window evicts the oldest row in
// FIFO order, delivering one invocation that carries the new row and the
// evicted row together, while a first-length window drops inserts once full
// with no callback anywhere, freeing capacity only on delete and appending the
// next arrival at the tail. Ord 25 drives the window through a SupportBean_A
// delete trigger and observes a wildcard consumer projected to theString.
//
// As in the sibling chains, the Java suite attaches an on-delete listener for
// ords 11/12/13 but never asserts its payload, so the delete listener is
// excluded from the trace on both sides: matching deletes DO deliver the
// removed rows to that listener as new data, so the exclusion rests on the
// never-asserted payload rather than on silence. Ord 25 attaches no delete
// listener at all.
const infraNWRLId = "infra-named-window-length-views"

const infraNWRLDescription = "InfraNamedWindowViews length-slice: the MySimpleKeyValueMap length and firstlength windows, the projection length window and the bean-backed length window over a SupportBean_A delete trigger, captured from listener callbacks and ordered window iterator snapshots (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWRLJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWRLSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// Byte-exact EPL pins (Java lines 1099-1102 for ord 11, 1151/1155/1159 for
// ord 12, 1263-1266 for ord 13 and 2463-2466 for ord 25).
const (
	infraNWRLCreateLength = "@name('create') create window MyWindowLW#length(3) as MySimpleKeyValueMap"
	infraNWRLInsertLength = "insert into MyWindowLW select theString as key, longBoxed as value from SupportBean"
	infraNWRLSelectLength = "@name('s0') select irstream key, value as value from MyWindowLW"
	infraNWRLDeleteLength = "@name('delete') on SupportMarketDataBean delete from MyWindowLW where symbol = key"

	infraNWRLCreateLengthTwo = "@name('create') @public create window MyWindow.win:length(3) as select theString as key, intBoxed as value from SupportBean"
	infraNWRLInsertLengthTwo = "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"
	infraNWRLDeleteLengthTwo = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"

	infraNWRLCreateFirstLength = "@name('create') create window MyWindowLFW#firstlength(2) as MySimpleKeyValueMap"
	infraNWRLInsertFirstLength = "insert into MyWindowLFW select theString as key, longBoxed as value from SupportBean"
	infraNWRLSelectFirstLength = "@name('s0') select irstream key, value as value from MyWindowLFW"
	infraNWRLDeleteFirstLength = "@name('delete') on SupportMarketDataBean delete from MyWindowLFW where symbol = key"

	infraNWRLCreateLengthThree = "@public create window ABCWin#length(2) as SupportBean"
	infraNWRLInsertLengthThree = "insert into ABCWin select * from SupportBean"
	infraNWRLDeleteLengthThree = "on SupportBean_A delete from ABCWin where theString = id"
	infraNWRLSelectLengthThree = "@Name('s0') select irstream * from ABCWin"
)

// infraNWRLSendKind selects the SupportBean payload shape and the projected row
// fields of a case.
type infraNWRLSendKind int

const (
	infraNWRLLengthLong infraNWRLSendKind = iota
	infraNWRLLengthInt
	infraNWRLFirstLengthLong
	infraNWRLLengthBean
)

type infraNWRLCaseSpec struct {
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
	sendKind      infraNWRLSendKind
	sendFields    []string
	rowFields     []string
}

var infraNWRLCaseSpecs = []infraNWRLCaseSpec{
	{
		name:          "length-window",
		ordinal:       11,
		runtimeID:     "java-runtime-d931bfbbbaa3ea632c2b",
		execution:     "InfraLengthWindow",
		windowName:    "MyWindowLW",
		description:   "length(3) window over the key/value map schema: FIFO eviction delivers one invocation carrying the new row and the evicted row, with ordered snapshots around every delete and eviction",
		createEPL:     infraNWRLCreateLength,
		insertEPL:     infraNWRLInsertLength,
		selectEPL:     infraNWRLSelectLength,
		deleteEPL:     infraNWRLDeleteLength,
		deploys:       []string{"create", "insert", "s0", "delete"},
		listened:      map[string]bool{"create": true, "s0": true},
		snapshots:     4,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		sendKind:      infraNWRLLengthLong,
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "length-window-scene-two",
		ordinal:       12,
		runtimeID:     "java-runtime-793f6ec7608e25556803",
		execution:     "InfraLengthWindowSceneTwo",
		windowName:    "MyWindow",
		description:   "legacy win:length(3) projection window over theString/intBoxed with three module deployments and only the window statement listened across ten delete and eviction waves",
		createEPL:     infraNWRLCreateLengthTwo,
		insertEPL:     infraNWRLInsertLengthTwo,
		deleteEPL:     infraNWRLDeleteLengthTwo,
		deploys:       []string{"create", "insert", "delete"},
		listened:      map[string]bool{"create": true},
		snapshots:     13,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		sendKind:      infraNWRLLengthInt,
		sendFields:    []string{"theString", "intBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "length-first-window",
		ordinal:       13,
		runtimeID:     "java-runtime-b53dbf8f541e79537f32",
		execution:     "InfraLengthFirstWindow",
		windowName:    "MyWindowLFW",
		description:   "firstlength(2) window over the key/value map schema: inserts into the full window are dropped without any callback, deletes free a slot, and the freed slot is filled at the tail by the next arrival",
		createEPL:     infraNWRLCreateFirstLength,
		insertEPL:     infraNWRLInsertFirstLength,
		selectEPL:     infraNWRLSelectFirstLength,
		deleteEPL:     infraNWRLDeleteFirstLength,
		deploys:       []string{"create", "insert", "s0", "delete"},
		listened:      map[string]bool{"create": true, "s0": true},
		snapshots:     4,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		sendKind:      infraNWRLFirstLengthLong,
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "length-window-scene-three",
		ordinal:       25,
		runtimeID:     "java-runtime-c656dd0eeae6618a4162",
		execution:     "InfraLengthWindowSceneThree",
		windowName:    "ABCWin",
		description:   "bean-backed length(2) window with a SupportBean_A delete trigger and a wildcard irstream consumer projected to theString; the first two snapshots are empty",
		createEPL:     infraNWRLCreateLengthThree,
		insertEPL:     infraNWRLInsertLengthThree,
		deleteEPL:     infraNWRLDeleteLengthThree,
		selectEPL:     infraNWRLSelectLengthThree,
		deploys:       []string{"create", "insert", "delete", "s0"},
		listened:      map[string]bool{"s0": true},
		snapshots:     4,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered"},
		snapStatement: "s0",
		beanTrigger:   true,
		sendKind:      infraNWRLLengthBean,
		sendFields:    []string{"theString"},
		rowFields:     []string{"theString"},
	},
}

var (
	infraNWRLJavaSources = []string{
		infraNWRLSource,
	}
	infraNWRLJavaRuntimeIDs = []string{
		"java-runtime-d931bfbbbaa3ea632c2b",
		"java-runtime-793f6ec7608e25556803",
		"java-runtime-b53dbf8f541e79537f32",
		"java-runtime-c656dd0eeae6618a4162",
	}
	infraNWRLJavaExecutions = []string{
		"InfraLengthWindow",
		"InfraLengthWindowSceneTwo",
		"InfraLengthFirstWindow",
		"InfraLengthWindowSceneThree",
	}
	infraNWRLCases = []string{
		"length-window",
		"length-window-scene-two",
		"length-first-window",
		"length-window-scene-three",
	}
)

// infraNWRLBean mirrors SupportBean; each case validates the exact send fields
// it pins (every case sets theString plus one value field, either intBoxed or
// longBoxed, and the unset fields stay at their Go zero value).
type infraNWRLBean struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
	LongBoxed int64  `esper:"longBoxed"`
}

type infraNWRLMarket struct {
	Symbol string `esper:"symbol"`
}

// infraNWRLA mirrors SupportBean_A, the ord-25 delete trigger (id is compared to
// the window row's theString).
type infraNWRLA struct {
	ID string `esper:"id"`
}

type infraNWRLKVLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type infraNWRLKVInt struct {
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

func infraNWRLCaseSpecFor(name string) (infraNWRLCaseSpec, bool) {
	for _, spec := range infraNWRLCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWRLCaseSpec{}, false
}

func loadInfraNWRLScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWRLId)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWRLId, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRLId, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRLId, err)
	}
	if err := requireInfraNWRLFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWRLId, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWRLId ||
		metadata.Description != infraNWRLDescription ||
		metadata.JavaCommit != infraNWRLJavaCommit || metadata.JavaSource != infraNWRLSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWRLId)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWRLId)
	}
	if err := infraNWRLRequireEqual(metadata.JavaRuntimes, infraNWRLJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWRLRequireEqual(metadata.JavaNames, infraNWRLJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWRLId, err)
	}
	if len(rawCases) != len(infraNWRLCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWRLId, len(rawCases), len(infraNWRLCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWRLFields(object,
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
		spec := infraNWRLCaseSpecs[index]
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
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source", spec.name)
		}
		if err := infraNWRLRequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWRLRequireListened(definition.Listened, spec); err != nil {
			return compat.Scenario{}, err
		}
		if definition.IteratorSnapshots != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q iteratorSnapshots = %d, want %d",
				spec.name, definition.IteratorSnapshots, spec.snapshots)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWRLId, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWRLId)
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
			if err := requireInfraNWRLFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWRLFields(object, "op", "case", "statement", "epl"); err != nil {
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
			spec, ok := infraNWRLCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWRLEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWRLFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWRLFields(object, "op", "case", "statement", "mode"); err != nil {
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
			spec, ok := infraNWRLCaseSpecFor(step.Case)
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
			if err := requireInfraNWRLFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWRLFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	for _, spec := range infraNWRLCaseSpecs {
		if deployCounts[spec.name] != len(spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d deploy steps, want %d",
				spec.name, deployCounts[spec.name], len(spec.deploys))
		}
		if snapshotCounts[spec.name] != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d snapshot steps, want %d",
				spec.name, snapshotCounts[spec.name], spec.snapshots)
		}
		if err := infraNWRLRequireEqual(snapshotModes[spec.name], spec.snapModes, spec.name+" snapshot modes"); err != nil {
			return compat.Scenario{}, err
		}
	}
	var steps []compat.Step
	if err := json.Unmarshal(root["steps"], &steps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", infraNWRLId, err)
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWRLEPLForStep returns the byte-exact EPL of one deploy step.
func infraNWRLEPLForStep(spec infraNWRLCaseSpec, statement string) (string, bool) {
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
	case "delete":
		if spec.deleteEPL == "" {
			return "", false
		}
		return spec.deleteEPL, true
	default:
		return "", false
	}
}

func infraNWRLRequireEqual(got, want []string, label string) error {
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

func infraNWRLRequireListened(got []string, spec infraNWRLCaseSpec) error {
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

func requireInfraNWRLFields(object map[string]json.RawMessage, names ...string) error {
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

func runInfraNWRLScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWRLId)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWRLCaseSpecs {
		caseTrace, err := runInfraNWRLCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWRLId, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWRLCase(ctx context.Context, scenario compat.Scenario, spec infraNWRLCaseSpec) ([]compat.TraceRecord, error) {
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
	if _, err := esper.RegisterStruct[infraNWRLBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWRLMarket](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWRLA](env, "SupportBean_A"); err != nil {
		return nil, err
	}
	var windowSchema esper.Schema
	var err error
	switch spec.sendKind {
	case infraNWRLLengthBean:
		windowSchema, err = esper.RegisterStruct[infraNWRLBean](env, spec.windowName)
	case infraNWRLLengthInt:
		windowSchema, err = esper.RegisterStruct[infraNWRLKVInt](env, spec.windowName)
	default:
		windowSchema, err = esper.RegisterStruct[infraNWRLKVLong](env, spec.windowName)
	}
	if err != nil {
		return nil, err
	}
	var retention esper.WindowSpec
	switch spec.ordinal {
	case 11, 12:
		retention = esper.LengthWindow(3)
	case 13:
		retention = esper.FirstLength(2)
	case 25:
		retention = esper.LengthWindow(2)
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
			plan, err := infraNWRLBuildPlan(env, spec, step.Statement)
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
			payload, err := infraNWRLDecodePayload(spec, step)
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

// infraNWRLBuildPlan maps one scenario deploy statement onto the typed chain.
func infraNWRLBuildPlan(env *esper.Environment, spec infraNWRLCaseSpec, statement string) (esper.Plan, error) {
	switch statement {
	case "create":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		source := esper.From[infraNWRLBean](env, "SupportBean")
		switch spec.sendKind {
		case infraNWRLLengthBean:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(
				spec.windowName, esper.CopyMatchingFields(),
			).Query(esper.StatementName("insert")))
		case infraNWRLLengthInt:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWRLBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWRLBean, int]("intBoxed")),
			).Query(esper.StatementName("insert")))
		default:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWRLBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWRLBean, int64]("longBoxed")),
			).Query(esper.StatementName("insert")))
		}
	case "s0":
		if spec.sendKind == infraNWRLLengthBean {
			// @Name('s0') select irstream * from ABCWin, projected to theString.
			return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
				esper.Alias("theString", esper.Field[any, string]("theString")),
			).Query(esper.StatementName("s0"), esper.WithOldStream()))
		}
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int64]("value")),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "delete":
		if spec.beanTrigger {
			return env.Build(esper.OnEvent(esper.From[infraNWRLA](env, "SupportBean_A")).
				DeleteFromNamedWindow(spec.windowName,
					esper.Equal[string](esper.NamedWindowField[string]("theString"),
						esper.Field[infraNWRLA, string]("id"))).
				Query(esper.StatementName("delete")))
		}
		return env.Build(esper.OnEvent(esper.From[infraNWRLMarket](env, "SupportMarketDataBean")).
			DeleteFromNamedWindow(spec.windowName,
				esper.Equal[string](esper.NamedWindowField[string]("key"),
					esper.Field[infraNWRLMarket, string]("symbol"))).
			Query(esper.StatementName("delete")))
	default:
		return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
	}
}

func infraNWRLDecodePayload(spec infraNWRLCaseSpec, step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWRLFields(fields, spec.sendFields...); err != nil {
			return nil, fmt.Errorf("case %q SupportBean payload: %w", spec.name, err)
		}
		var bean infraNWRLBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWRLFields(fields, "symbol"); err != nil {
			return nil, fmt.Errorf("SupportMarketDataBean payload: %w", err)
		}
		var market infraNWRLMarket
		if err := json.Unmarshal(step.Payload, &market); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return market, nil
	case "SupportBean_A":
		if err := requireInfraNWRLFields(fields, "id"); err != nil {
			return nil, fmt.Errorf("SupportBean_A payload: %w", err)
		}
		var bean infraNWRLA
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
