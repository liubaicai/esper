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

// Parity coverage for the ninth differential slice of InfraNamedWindowViews:
// the batch families under virtual time (time_batch and time_length_batch).
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 16 InfraTimeBatch               java-runtime-1ad42a8ed8025c4a0730
//   - ord 17 InfraTimeBatchSceneTwo       java-runtime-1eb11f5cf275069ef6b5
//   - ord 18 InfraTimeBatchLateConsumer   java-runtime-bb926d092e7110db203f
//   - ord 23 InfraTimeLengthBatch         java-runtime-22bf6b3644a24862df7d
//   - ord 24 InfraTimeLengthBatchSceneTwo java-runtime-dd924e1b7e500df135f8
//
// A batch window buffers arrivals with no callback; the boundary flush delivers
// the buffered rows as one new-data batch (insertion order) together with the
// previous batch as old data, the first flush carries no old data and an
// all-empty flush produces no callback at all. Deletes remove pending rows
// silently. The slice also pins the time_length_batch dual trigger (an immediate
// size flush that cancels and re-arms the pending time callback) and the late
// consumer whose batch preload is skipped, so its first aggregate row covers the
// whole batch including rows that arrived before it existed.
//
// Delete listeners are never asserted and stay excluded from the trace.
const infraNWRTBId = "infra-named-window-time-batch-views"

const infraNWRTBDescription = "InfraNamedWindowViews time-batch-slice: the time_batch(10 sec) map and projection windows with buffered arrivals and boundary flushes, the late aggregate consumer whose preload is skipped, and the time_length_batch(10 sec, 3|4) windows with their size-or-time dual trigger under virtual time (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWRTBJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWRTBSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// Byte-exact EPL pins (Java lines 1593-1596 for ord 16, 1662/1666/1670 for
// ord 17, 1797/1798 + 1808 for ord 18, 2245-2248 for ord 23 and
// 2320/2324/2328 for ord 24).
const (
	infraNWRTBCreateTimeBatch = "@name('create') create window MyWindowTB#time_batch(10 sec) as MySimpleKeyValueMap"
	infraNWRTBInsertTimeBatch = "insert into MyWindowTB select theString as key, longBoxed as value from SupportBean"
	infraNWRTBSelectTimeBatch = "@name('s0') select key, value as value from MyWindowTB"
	infraNWRTBDeleteTimeBatch = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowTB as s1 where s0.symbol = s1.key"

	infraNWRTBCreateTimeBatchTwo = "@name('create') @public create window MyWindow.win:time_batch(10) as select theString as key, intBoxed as value from SupportBean"
	infraNWRTBInsertTimeBatchTwo = "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"
	infraNWRTBDeleteTimeBatchTwo = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"

	infraNWRTBCreateLateConsumer = "@name('create') @public create window MyWindowTBLC#time_batch(10 sec) as MySimpleKeyValueMap"
	infraNWRTBInsertLateConsumer = "insert into MyWindowTBLC select theString as key, longBoxed as value from SupportBean"
	infraNWRTBSelectLateConsumer = "@name('s0') select sum(value) as value from MyWindowTBLC"

	infraNWRTBCreateTimeLengthBatch = "@name('create') create window MyWindowTLB#time_length_batch(10 sec, 3) as MySimpleKeyValueMap"
	infraNWRTBInsertTimeLengthBatch = "insert into MyWindowTLB select theString as key, longBoxed as value from SupportBean"
	infraNWRTBSelectTimeLengthBatch = "@name('s0') select key, value as value from MyWindowTLB"
	infraNWRTBDeleteTimeLengthBatch = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowTLB as s1 where s0.symbol = s1.key"

	infraNWRTBCreateTimeLengthBatchTwo = "@name('create') @public create window MyWindow.win:time_length_batch(10 sec, 4) as select theString as key, intBoxed as value from SupportBean"
	infraNWRTBInsertTimeLengthBatchTwo = "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"
	infraNWRTBDeleteTimeLengthBatchTwo = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"
)

type infraNWRTBCaseSpec struct {
	name          string
	ordinal       int
	runtimeID     string
	execution     string
	windowName    string
	description   string
	createEPL     string
	insertEPL     string
	selectEPL     string
	deleteEPL     string
	snapModes     []string
	snapStatement string
	istreamOnly   bool
	timeBatch     bool
	timeLength    int
	aggregate     bool
	accumInt      bool
	deploys       []string
	listened      map[string]bool
	snapshots     int
	sendFields    []string
	rowFields     []string
}

var infraNWRTBCaseSpecs = []infraNWRTBCaseSpec{
	{
		name:          "time-batch",
		ordinal:       16,
		runtimeID:     "java-runtime-1ad42a8ed8025c4a0730",
		execution:     "InfraTimeBatch",
		windowName:    "MyWindowTB",
		description:   "time_batch(10 sec) window over the key/value map schema: arrivals buffer silently, deletes remove pending rows without a callback, and boundary flushes deliver the buffered rows as one new-data batch followed by an old-only flush of the previous batch",
		createEPL:     infraNWRTBCreateTimeBatch,
		insertEPL:     infraNWRTBInsertTimeBatch,
		selectEPL:     infraNWRTBSelectTimeBatch,
		deleteEPL:     infraNWRTBDeleteTimeBatch,
		deploys:       []string{"create", "insert", "s0", "delete"},
		listened:      map[string]bool{"create": true, "s0": true},
		snapshots:     5,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		istreamOnly:   true,
		timeBatch:     true,
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "time-batch-scene-two",
		ordinal:       17,
		runtimeID:     "java-runtime-1eb11f5cf275069ef6b5",
		execution:     "InfraTimeBatchSceneTwo",
		windowName:    "MyWindow",
		description:   "legacy win:time_batch(10) projection window over theString/intBoxed with three module deployments and only the window statement listened; the anchor stays at the first arrival even across a silent empty flush",
		createEPL:     infraNWRTBCreateTimeBatchTwo,
		insertEPL:     infraNWRTBInsertTimeBatchTwo,
		deleteEPL:     infraNWRTBDeleteTimeBatchTwo,
		deploys:       []string{"create", "insert", "delete"},
		listened:      map[string]bool{"create": true},
		snapshots:     8,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		timeBatch:     true,
		accumInt:      true,
		sendFields:    []string{"theString", "intBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "time-batch-late-consumer",
		ordinal:       18,
		runtimeID:     "java-runtime-bb926d092e7110db203f",
		execution:     "InfraTimeBatchLateConsumer",
		windowName:    "MyWindowTBLC",
		description:   "time_batch(10 sec) window over the key/value map schema with a late-deployed aggregate consumer whose batch preload is skipped, so the first flush reports the sum of the whole batch including rows that arrived before the consumer existed",
		createEPL:     infraNWRTBCreateLateConsumer,
		insertEPL:     infraNWRTBInsertLateConsumer,
		selectEPL:     infraNWRTBSelectLateConsumer,
		deploys:       []string{"create", "insert", "s0"},
		listened:      map[string]bool{"s0": true},
		snapshots:     1,
		snapModes:     []string{"ordered"},
		snapStatement: "create",
		timeBatch:     true,
		aggregate:     true,
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"value"},
	},
	{
		name:          "time-length-batch",
		ordinal:       23,
		runtimeID:     "java-runtime-22bf6b3644a24862df7d",
		execution:     "InfraTimeLengthBatch",
		windowName:    "MyWindowTLB",
		description:   "time_length_batch(10 sec, 3) window over the key/value map schema: the size trigger flushes immediately on the third arrival and re-arms the boundary, after which deletes stay silent and the time trigger flushes the remaining row with the prior batch as old data",
		createEPL:     infraNWRTBCreateTimeLengthBatch,
		insertEPL:     infraNWRTBInsertTimeLengthBatch,
		selectEPL:     infraNWRTBSelectTimeLengthBatch,
		deleteEPL:     infraNWRTBDeleteTimeLengthBatch,
		deploys:       []string{"create", "insert", "s0", "delete"},
		listened:      map[string]bool{"create": true, "s0": true},
		snapshots:     6,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		istreamOnly:   true,
		timeLength:    3,
		sendFields:    []string{"theString", "longBoxed"},
		rowFields:     []string{"key", "value"},
	},
	{
		name:          "time-length-batch-scene-two",
		ordinal:       24,
		runtimeID:     "java-runtime-dd924e1b7e500df135f8",
		execution:     "InfraTimeLengthBatchSceneTwo",
		windowName:    "MyWindow",
		description:   "legacy win:time_length_batch(10 sec, 4) projection window over theString/intBoxed with three module deployments and only the window statement listened across the silent-delete, size and time trigger timeline",
		createEPL:     infraNWRTBCreateTimeLengthBatchTwo,
		insertEPL:     infraNWRTBInsertTimeLengthBatchTwo,
		deleteEPL:     infraNWRTBDeleteTimeLengthBatchTwo,
		deploys:       []string{"create", "insert", "delete"},
		listened:      map[string]bool{"create": true},
		snapshots:     11,
		snapModes:     []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatement: "create",
		accumInt:      true,
		timeLength:    4,
		sendFields:    []string{"theString", "intBoxed"},
		rowFields:     []string{"key", "value"},
	},
}

var (
	infraNWRTBJavaSources = []string{
		infraNWRTBSource,
	}
	infraNWRTBJavaRuntimeIDs = []string{
		"java-runtime-1ad42a8ed8025c4a0730",
		"java-runtime-1eb11f5cf275069ef6b5",
		"java-runtime-bb926d092e7110db203f",
		"java-runtime-22bf6b3644a24862df7d",
		"java-runtime-dd924e1b7e500df135f8",
	}
	infraNWRTBJavaExecutions = []string{
		"InfraTimeBatch",
		"InfraTimeBatchSceneTwo",
		"InfraTimeBatchLateConsumer",
		"InfraTimeLengthBatch",
		"InfraTimeLengthBatchSceneTwo",
	}
	infraNWRTBCases = []string{
		"time-batch",
		"time-batch-scene-two",
		"time-batch-late-consumer",
		"time-length-batch",
		"time-length-batch-scene-two",
	}
)

// infraNWRTBBean mirrors SupportBean; each case validates the exact send fields
// it pins (every case sets theString plus one value field, either intBoxed or
// longBoxed, and the unset fields stay at their Go zero value).
type infraNWRTBBean struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
	LongBoxed int64  `esper:"longBoxed"`
}

type infraNWRTBMarket struct {
	Symbol string `esper:"symbol"`
}

type infraNWRTBKVLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type infraNWRTBKVInt struct {
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

func infraNWRTBCaseSpecFor(name string) (infraNWRTBCaseSpec, bool) {
	for _, spec := range infraNWRTBCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWRTBCaseSpec{}, false
}

func loadInfraNWRTBScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWRTBId)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWRTBId, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRTBId, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRTBId, err)
	}
	if err := requireInfraNWRTBFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWRTBId, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWRTBId ||
		metadata.Description != infraNWRTBDescription ||
		metadata.JavaCommit != infraNWRTBJavaCommit || metadata.JavaSource != infraNWRTBSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWRTBId)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWRTBId)
	}
	if err := infraNWRTBRequireEqual(metadata.JavaRuntimes, infraNWRTBJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWRTBRequireEqual(metadata.JavaNames, infraNWRTBJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWRTBId, err)
	}
	if len(rawCases) != len(infraNWRTBCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWRTBId, len(rawCases), len(infraNWRTBCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWRTBFields(object,
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
		spec := infraNWRTBCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
		}
		if definition.Description != spec.description {
			return compat.Scenario{}, fmt.Errorf("scenario case %q description does not match the pinned slice description", spec.name)
		}
		if definition.CreateEPL != spec.createEPL || definition.InsertEPL != spec.insertEPL ||
			definition.SelectEPL != spec.selectEPL ||
			definition.DeleteEPL != spec.deleteEPL {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source (create=%t insert=%t select=%t delete=%t)",
				spec.name, definition.CreateEPL == spec.createEPL, definition.InsertEPL == spec.insertEPL,
				definition.SelectEPL == spec.selectEPL,
				definition.DeleteEPL == spec.deleteEPL)
		}
		if err := infraNWRTBRequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWRTBRequireListened(definition.Listened, spec); err != nil {
			return compat.Scenario{}, err
		}
		if definition.IteratorSnapshots != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q iteratorSnapshots = %d, want %d",
				spec.name, definition.IteratorSnapshots, spec.snapshots)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWRTBId, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWRTBId)
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
			if err := requireInfraNWRTBFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWRTBFields(object, "op", "case", "statement", "epl"); err != nil {
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
			spec, ok := infraNWRTBCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWRTBEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWRTBFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWRTBFields(object, "op", "case", "statement", "mode"); err != nil {
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
			spec, ok := infraNWRTBCaseSpecFor(step.Case)
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
			if err := requireInfraNWRTBFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "advance-time":
			if err := requireInfraNWRTBFields(object, "op", "case", "at"); err != nil {
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
			if _, ok := infraNWRTBCaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advances unknown case %q", index, step.Case)
			}
		case "undeploy-all":
			if err := requireInfraNWRTBFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	for _, spec := range infraNWRTBCaseSpecs {
		if deployCounts[spec.name] != len(spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d deploy steps, want %d",
				spec.name, deployCounts[spec.name], len(spec.deploys))
		}
		if snapshotCounts[spec.name] != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d snapshot steps, want %d",
				spec.name, snapshotCounts[spec.name], spec.snapshots)
		}
		if err := infraNWRTBRequireEqual(snapshotModes[spec.name], spec.snapModes, spec.name+" snapshot modes"); err != nil {
			return compat.Scenario{}, err
		}
	}
	var steps []compat.Step
	if err := json.Unmarshal(root["steps"], &steps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", infraNWRTBId, err)
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWRTBEPLForStep returns the byte-exact EPL of one deploy step.
func infraNWRTBEPLForStep(spec infraNWRTBCaseSpec, statement string) (string, bool) {
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

func infraNWRTBRequireEqual(got, want []string, label string) error {
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

func infraNWRTBRequireListened(got []string, spec infraNWRTBCaseSpec) error {
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

func requireInfraNWRTBFields(object map[string]json.RawMessage, names ...string) error {
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

func runInfraNWRTBScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWRTBId)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWRTBCaseSpecs {
		caseTrace, err := runInfraNWRTBCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWRTBId, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWRTBCase(ctx context.Context, scenario compat.Scenario, spec infraNWRTBCaseSpec) ([]compat.TraceRecord, error) {
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
	if _, err := esper.RegisterStruct[infraNWRTBBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWRTBMarket](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	var windowSchema esper.Schema
	var err error
	if spec.accumInt {
		windowSchema, err = esper.RegisterStruct[infraNWRTBKVInt](env, spec.windowName)
	} else {
		windowSchema, err = esper.RegisterStruct[infraNWRTBKVLong](env, spec.windowName)
	}
	if err != nil {
		return nil, err
	}
	var retention esper.WindowSpec
	switch {
	case spec.timeBatch:
		retention = esper.TimeBatch(10 * time.Second)
	case spec.timeLength > 0:
		retention = esper.TimeLengthBatch(10*time.Second, spec.timeLength)
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
			plan, err := infraNWRTBBuildPlan(env, spec, step.Statement)
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
			payload, err := infraNWRTBDecodePayload(spec, step)
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

// infraNWRTBBuildPlan maps one scenario deploy statement onto the typed chain.
func infraNWRTBBuildPlan(env *esper.Environment, spec infraNWRTBCaseSpec, statement string) (esper.Plan, error) {
	switch statement {
	case "create":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		source := esper.From[infraNWRTBBean](env, "SupportBean")
		if spec.accumInt {
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWRTBBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWRTBBean, int]("intBoxed")),
			).Query(esper.StatementName("insert")))
		}
		return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
			esper.SetColumn("key", esper.Field[infraNWRTBBean, string]("theString")),
			esper.SetColumn("value", esper.Field[infraNWRTBBean, int64]("longBoxed")),
		).Query(esper.StatementName("insert")))
	case "s0":
		if spec.aggregate {
			return env.Build(esper.FromNamedWindow(env, spec.windowName).Aggregate(
				esper.Alias("value", esper.Sum[int64](esper.Field[any, int64]("value"))),
			).Query(esper.StatementName("s0")))
		}
		if spec.istreamOnly {
			return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
				esper.Alias("key", esper.Field[any, string]("key")),
				esper.Alias("value", esper.Field[any, int64]("value")),
			).Query(esper.StatementName("s0")))
		}
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int64]("value")),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "delete":
		return env.Build(esper.OnEvent(esper.From[infraNWRTBMarket](env, "SupportMarketDataBean")).
			DeleteFromNamedWindow(spec.windowName,
				esper.Equal[string](esper.NamedWindowField[string]("key"),
					esper.Field[infraNWRTBMarket, string]("symbol"))).
			Query(esper.StatementName("delete")))
	default:
		return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
	}
}

func infraNWRTBDecodePayload(spec infraNWRTBCaseSpec, step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWRTBFields(fields, spec.sendFields...); err != nil {
			return nil, fmt.Errorf("case %q SupportBean payload: %w", spec.name, err)
		}
		var bean infraNWRTBBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWRTBFields(fields, "symbol"); err != nil {
			return nil, fmt.Errorf("SupportMarketDataBean payload: %w", err)
		}
		var market infraNWRTBMarket
		if err := json.Unmarshal(step.Payload, &market); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return market, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
