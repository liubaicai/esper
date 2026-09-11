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

// Parity coverage for the fifth differential slice of InfraNamedWindowViews:
// the length-batch and sort data windows.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 19 InfraLengthBatch           java-runtime-81f4d92e62dbafdd7f2f
//   - ord 20 InfraLengthBatchSceneTwo   java-runtime-ce4a95ec7941743a7bfe
//   - ord 21 InfraSortWindow            java-runtime-7edfd8382e4eee6892dc
//   - ord 22 InfraSortWindowSceneTwo    java-runtime-5fb3e9a4d20bc3964f5f
//
// The slice pins two retention families: a length_batch window buffers arrivals
// and releases only at the size boundary, where the window statement delivers
// the buffered rows as new data together with the rows the batch replaced as
// old data (the first flush carries no old data), while deletes of unflushed
// rows stay silent; a sort window keeps the N smallest values, expelling the
// current maximum on overflow (one invocation carrying new and old), ordering
// equal values newest-first and reflecting deletes in the ordered snapshots.
//
// Consumers in this slice are istream-only (`select key, value as value`), so
// the delete waves reach the window statement alone. The Java suite attaches
// an on-delete listener that it never asserts, excluded from the trace on both
// sides as in the sibling slices.
const infraNWRBId = "infra-named-window-lengthbatch-sort-views"

const infraNWRBDescription = "InfraNamedWindowViews length-batch/sort-slice: the MySimpleKeyValueMap length_batch and sort windows, their projection variants and the sort-window delete trigger, captured from listener callbacks and ordered window iterator snapshots (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWRBJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWRBSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// Byte-exact EPL pins (Java lines 1827-1831 for ord 19, 1911/1917/1923 for
// ord 20, 2100-2104 for ord 21 and 2165/2169/2173 for ord 22).
const (
	infraNWRBCreateLengthBatch = "@name('create') create window MyWindowLB#length_batch(3) as MySimpleKeyValueMap"
	infraNWRBInsertLengthBatch = "insert into MyWindowLB select theString as key, longBoxed as value from SupportBean"
	infraNWRBSelectLengthBatch = "@name('s0') select key, value as value from MyWindowLB"
	infraNWRBDeleteLengthBatch = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowLB as s1 where s0.symbol = s1.key"

	infraNWRBCreateLengthBatchTwo = "@name('create') @public create window MyWindow.win:length_batch(3) as select theString as key, intBoxed as value from SupportBean"
	infraNWRBInsertLengthBatchTwo = "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"
	infraNWRBDeleteLengthBatchTwo = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"

	infraNWRBCreateSort = "@name('create') create window MyWindowSW#sort(3, value asc) as MySimpleKeyValueMap"
	infraNWRBInsertSort = "insert into MyWindowSW select theString as key, longBoxed as value from SupportBean"
	infraNWRBSelectSort = "@name('s0') select key, value as value from MyWindowSW"
	infraNWRBDeleteSort = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowSW as s1 where s0.symbol = s1.key"

	infraNWRBCreateSortTwo = "@name('create') @public create window MyWindow.ext:sort(3, value) as select theString as key, intBoxed as value from SupportBean"
	infraNWRBInsertSortTwo = "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"
	infraNWRBDeleteSortTwo = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"
)

// infraNWRBSendKind selects the SupportBean payload shape and the projected row
// fields of a case.
type infraNWRBSendKind int

const (
	infraNWRBBatchLong infraNWRBSendKind = iota
	infraNWRBBatchInt
	infraNWRBSortLong
	infraNWRBSortInt
)

type infraNWRBCaseSpec struct {
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
	istreamOnly   bool
	deploys       []string
	listened      map[string]bool
	snapshots     int
	sendKind      infraNWRBSendKind
	sendFields    []string
	rowFields     []string
}

var infraNWRBCaseSpecs = []infraNWRBCaseSpec{
	{
		name:          "length-batch",
		ordinal:       19,
		runtimeID:     "java-runtime-81f4d92e62dbafdd7f2f",
		execution:     "InfraLengthBatch",
		windowName:    "MyWindowLB",
		description:   "length_batch(3) window over the key/value map schema: the batch releases only at the size boundary, carrying the buffered rows together with the rows replaced by the batch, and deletes remove rows without a pending flush",
		createEPL:     infraNWRBCreateLengthBatch,
		insertEPL:     infraNWRBInsertLengthBatch,
		selectEPL:     infraNWRBSelectLengthBatch,
		deleteEPL:     infraNWRBDeleteLengthBatch,
		deploys:       []string{"create", "insert", "s0", "delete"},
		listened:      map[string]bool{"create": true, "s0": true},
		snapshots:     5,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		istreamOnly:   true,
		sendKind:      infraNWRBBatchLong,
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "length-batch-scene-two",
		ordinal:       20,
		runtimeID:     "java-runtime-ce4a95ec7941743a7bfe",
		execution:     "InfraLengthBatchSceneTwo",
		windowName:    "MyWindow",
		description:   "legacy win:length_batch(3) projection window over theString/intBoxed with three module deployments and only the window statement listened across the full three-flush timeline",
		createEPL:     infraNWRBCreateLengthBatchTwo,
		insertEPL:     infraNWRBInsertLengthBatchTwo,
		deleteEPL:     infraNWRBDeleteLengthBatchTwo,
		deploys:       []string{"create", "insert", "delete"},
		listened:      map[string]bool{"create": true},
		snapshots:     23,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		sendKind:      infraNWRBBatchInt,
		sendFields:    []string{"theString", "intBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "sort-window",
		ordinal:       21,
		runtimeID:     "java-runtime-7edfd8382e4eee6892dc",
		execution:     "InfraSortWindow",
		windowName:    "MyWindowSW",
		description:   "sort(3, value asc) window over the key/value map schema: retention keeps the three lowest values with recency tie ordering, expelling the maximum on overflow and reflecting deletes in the ordered snapshots",
		createEPL:     infraNWRBCreateSort,
		insertEPL:     infraNWRBInsertSort,
		selectEPL:     infraNWRBSelectSort,
		deleteEPL:     infraNWRBDeleteSort,
		deploys:       []string{"create", "insert", "s0", "delete"},
		listened:      map[string]bool{"create": true, "s0": true},
		snapshots:     9,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		istreamOnly:   true,
		sendKind:      infraNWRBSortLong,
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "sort-window-scene-two",
		ordinal:       22,
		runtimeID:     "java-runtime-5fb3e9a4d20bc3964f5f",
		execution:     "InfraSortWindowSceneTwo",
		windowName:    "MyWindow",
		description:   "legacy ext:sort(3, value) projection window over theString/intBoxed with three module deployments, ordered retention and the delete trigger",
		createEPL:     infraNWRBCreateSortTwo,
		insertEPL:     infraNWRBInsertSortTwo,
		deleteEPL:     infraNWRBDeleteSortTwo,
		deploys:       []string{"create", "insert", "delete"},
		listened:      map[string]bool{"create": true},
		snapshots:     6,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		sendKind:      infraNWRBSortInt,
		sendFields:    []string{"theString", "intBoxed"},
		rowFields:     []string{"key", "value"},
	},
}

var (
	infraNWRBJavaSources = []string{
		infraNWRBSource,
	}
	infraNWRBJavaRuntimeIDs = []string{
		"java-runtime-81f4d92e62dbafdd7f2f",
		"java-runtime-ce4a95ec7941743a7bfe",
		"java-runtime-7edfd8382e4eee6892dc",
		"java-runtime-5fb3e9a4d20bc3964f5f",
	}
	infraNWRBJavaExecutions = []string{
		"InfraLengthBatch",
		"InfraLengthBatchSceneTwo",
		"InfraSortWindow",
		"InfraSortWindowSceneTwo",
	}
	infraNWRBCases = []string{
		"length-batch",
		"length-batch-scene-two",
		"sort-window",
		"sort-window-scene-two",
	}
)

// infraNWRBBean mirrors SupportBean; each case validates the exact send fields
// it pins (every case sets theString plus one value field, either intBoxed or
// longBoxed, and the unset fields stay at their Go zero value).
type infraNWRBBean struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
	LongBoxed int64  `esper:"longBoxed"`
}

type infraNWRBMarket struct {
	Symbol string `esper:"symbol"`
}

type infraNWRBKVLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type infraNWRBKVInt struct {
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

func infraNWRBCaseSpecFor(name string) (infraNWRBCaseSpec, bool) {
	for _, spec := range infraNWRBCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWRBCaseSpec{}, false
}

func loadInfraNWRBScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWRBId)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWRBId, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRBId, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRBId, err)
	}
	if err := requireInfraNWRBFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWRBId, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWRBId ||
		metadata.Description != infraNWRBDescription ||
		metadata.JavaCommit != infraNWRBJavaCommit || metadata.JavaSource != infraNWRBSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWRBId)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWRBId)
	}
	if err := infraNWRBRequireEqual(metadata.JavaRuntimes, infraNWRBJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWRBRequireEqual(metadata.JavaNames, infraNWRBJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWRBId, err)
	}
	if len(rawCases) != len(infraNWRBCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWRBId, len(rawCases), len(infraNWRBCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWRBFields(object,
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
		spec := infraNWRBCaseSpecs[index]
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
		if err := infraNWRBRequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWRBRequireListened(definition.Listened, spec); err != nil {
			return compat.Scenario{}, err
		}
		if definition.IteratorSnapshots != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q iteratorSnapshots = %d, want %d",
				spec.name, definition.IteratorSnapshots, spec.snapshots)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWRBId, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWRBId)
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
			if err := requireInfraNWRBFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWRBFields(object, "op", "case", "statement", "epl"); err != nil {
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
			spec, ok := infraNWRBCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWRBEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWRBFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWRBFields(object, "op", "case", "statement", "mode"); err != nil {
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
			spec, ok := infraNWRBCaseSpecFor(step.Case)
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
			if err := requireInfraNWRBFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWRBFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	for _, spec := range infraNWRBCaseSpecs {
		if deployCounts[spec.name] != len(spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d deploy steps, want %d",
				spec.name, deployCounts[spec.name], len(spec.deploys))
		}
		if snapshotCounts[spec.name] != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d snapshot steps, want %d",
				spec.name, snapshotCounts[spec.name], spec.snapshots)
		}
		if err := infraNWRBRequireEqual(snapshotModes[spec.name], spec.snapModes, spec.name+" snapshot modes"); err != nil {
			return compat.Scenario{}, err
		}
	}
	var steps []compat.Step
	if err := json.Unmarshal(root["steps"], &steps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", infraNWRBId, err)
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWRBEPLForStep returns the byte-exact EPL of one deploy step.
func infraNWRBEPLForStep(spec infraNWRBCaseSpec, statement string) (string, bool) {
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

func infraNWRBRequireEqual(got, want []string, label string) error {
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

func infraNWRBRequireListened(got []string, spec infraNWRBCaseSpec) error {
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

func requireInfraNWRBFields(object map[string]json.RawMessage, names ...string) error {
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

func runInfraNWRBScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWRBId)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWRBCaseSpecs {
		caseTrace, err := runInfraNWRBCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWRBId, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWRBCase(ctx context.Context, scenario compat.Scenario, spec infraNWRBCaseSpec) ([]compat.TraceRecord, error) {
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
	if _, err := esper.RegisterStruct[infraNWRBBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWRBMarket](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	var windowSchema esper.Schema
	var err error
	switch spec.sendKind {
	case infraNWRBBatchInt, infraNWRBSortInt:
		windowSchema, err = esper.RegisterStruct[infraNWRBKVInt](env, spec.windowName)
	default:
		windowSchema, err = esper.RegisterStruct[infraNWRBKVLong](env, spec.windowName)
	}
	if err != nil {
		return nil, err
	}
	sortKey := esper.Ascending(esper.Field[infraNWRBKVLong, int64]("value"))
	if spec.sendKind == infraNWRBSortInt {
		sortKey = esper.Ascending(esper.Field[infraNWRBKVInt, int]("value"))
	}
	var retention esper.WindowSpec
	switch spec.ordinal {
	case 19, 20:
		retention = esper.LengthBatch(3)
	case 21, 22:
		retention = esper.SortWindow(3, sortKey)
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
			plan, err := infraNWRBBuildPlan(env, spec, step.Statement)
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
			payload, err := infraNWRBDecodePayload(spec, step)
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

// infraNWRBBuildPlan maps one scenario deploy statement onto the typed chain.
func infraNWRBBuildPlan(env *esper.Environment, spec infraNWRBCaseSpec, statement string) (esper.Plan, error) {
	switch statement {
	case "create":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		source := esper.From[infraNWRBBean](env, "SupportBean")
		switch spec.sendKind {
		case infraNWRBBatchInt, infraNWRBSortInt:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWRBBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWRBBean, int]("intBoxed")),
			).Query(esper.StatementName("insert")))
		default:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWRBBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWRBBean, int64]("longBoxed")),
			).Query(esper.StatementName("insert")))
		}
	case "s0":
		// select key, value as value (istream-only): the delete waves reach the
		// window statement alone, so this consumer selects new rows only.
		options := []esper.QueryOption{esper.StatementName("s0")}
		if !spec.istreamOnly {
			options = append(options, esper.WithOldStream())
		}
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int64]("value")),
		).Query(options...))
	case "delete":
		return env.Build(esper.OnEvent(esper.From[infraNWRBMarket](env, "SupportMarketDataBean")).
			DeleteFromNamedWindow(spec.windowName,
				esper.Equal[string](esper.NamedWindowField[string]("key"),
					esper.Field[infraNWRBMarket, string]("symbol"))).
			Query(esper.StatementName("delete")))
	default:
		return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
	}
}

func infraNWRBDecodePayload(spec infraNWRBCaseSpec, step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWRBFields(fields, spec.sendFields...); err != nil {
			return nil, fmt.Errorf("case %q SupportBean payload: %w", spec.name, err)
		}
		var bean infraNWRBBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWRBFields(fields, "symbol"); err != nil {
			return nil, fmt.Errorf("SupportMarketDataBean payload: %w", err)
		}
		var market infraNWRBMarket
		if err := json.Unmarshal(step.Payload, &market); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return market, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
