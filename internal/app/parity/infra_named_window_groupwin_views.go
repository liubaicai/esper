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

// Parity coverage for the tenth differential slice of InfraNamedWindowViews:
// the per-group retention views (length and time_batch nested in #groupwin)
// and the late-started grouped views over an already-populated window.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 26 InfraLengthWindowPerGroup                      java-runtime-083a289ee5f82dd87ad3
//   - ord 27 InfraTimeBatchPerGroup                         java-runtime-accaf82c3c846493832b
//   - ord 44 InfraSelectGroupedViewLateStart                java-runtime-3326973240f20b92dced
//   - ord 55 InfraSelectGroupedViewLateStartVariableIterate java-runtime-0ec49d4098c9796540b0
//
// A #groupwin(view) window keeps one sub-view per group key: the window
// iterator walks the groups in group-creation order and each group in
// insertion order, a length sub-view expires only on insert (a delete removes
// without expiring), and a time_batch sub-view anchors its boundary at the
// group's first arrival so the same-instant flushes of all groups concatenate
// into one new-data callback in group-creation order. The two projection
// windows retain nine rows per (theString, intPrimitive) group; their grouped
// consumers are deployed after the window is populated, so the preload replays
// the whole window into the grouped aggregate state, and the second consumer
// evaluates its having clause against the runtime variable at iterate time, so
// the on-set trigger switches the visible theString group without rebuilding
// that state.
//
// The ord-26 delete listener is attached but never asserted and stays excluded
// from the trace.
const infraNWGWId = "infra-named-window-groupwin-views"

const infraNWGWDescription = "InfraNamedWindowViews groupwin-slice: the per-group #groupwin(value)#length(2) map window whose iterator walks group-creation order and whose delete removes without expiring, the per-group #groupwin(value)#time_batch(10 sec) map window whose boundary flush concatenates the group batches as [E1,E4,E2,E3], and the #groupwin(theString, intPrimitive)#length(9) projection windows whose late grouped count(*)/avg/count consumers preload the populated window, the second one having-filtered on a runtime variable (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWGWJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWGWSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// infraNWGWVariableName is the ord-55 runtime variable written by the on-set
// trigger and read by the grouped consumer's having clause.
const infraNWGWVariableName = "var_1_1_1"

// Byte-exact EPL pins (Java lines 2510-2513 for ord 26, 2586-2588 for ord 27,
// 3001-3002 + 3021 for ord 44 and 3051/3055/3059/3060 + 3083-3084 for
// ord 55). Each pin is the statement text without the trailing ";" and
// newline of the deployment that joins the statements.
const (
	infraNWGWCreateLengthPerGroup = "@name('create') create window MyWindowWPG#groupwin(value)#length(2) as MySimpleKeyValueMap"
	infraNWGWInsertLengthPerGroup = "insert into MyWindowWPG select theString as key, longBoxed as value from SupportBean"
	infraNWGWSelectLengthPerGroup = "@name('s0') select irstream key, value as value from MyWindowWPG"
	infraNWGWDeleteLengthPerGroup = "@name('delete') on SupportMarketDataBean delete from MyWindowWPG where symbol = key"

	infraNWGWCreateTimeBatchPerGroup = "@name('create') create window MyWindowTBPG#groupwin(value)#time_batch(10 sec) as MySimpleKeyValueMap"
	infraNWGWInsertTimeBatchPerGroup = "insert into MyWindowTBPG select theString as key, longBoxed as value from SupportBean"
	infraNWGWSelectTimeBatchPerGroup = "@name('s0') select key, value as value from MyWindowTBPG"

	infraNWGWCreateGroupedLateStart = "@name('create') @public create window MyWindowSGVS#groupwin(theString, intPrimitive)#length(9) as select theString, intPrimitive from SupportBean"
	infraNWGWInsertGroupedLateStart = "@name('insert') insert into MyWindowSGVS select theString, intPrimitive from SupportBean"
	infraNWGWSelectGroupedLateStart = "@name('s0') select theString, intPrimitive, count(*) from MyWindowSGVS group by theString, intPrimitive order by theString, intPrimitive"

	infraNWGWCreateGroupedLateStartVariable  = "@name('create') @public create window MyWindowSGVLS#groupwin(theString, intPrimitive)#length(9) as select theString, intPrimitive, longPrimitive, boolPrimitive from SupportBean"
	infraNWGWInsertGroupedLateStartVariable  = "insert into MyWindowSGVLS select theString, intPrimitive, longPrimitive, boolPrimitive from SupportBean"
	infraNWGWDeclareGroupedLateStartVariable = "@public create variable string var_1_1_1"
	infraNWGWOnSetGroupedLateStartVariable   = "on SupportVariableSetEvent(variableName='var_1_1_1') set var_1_1_1 = value"
	infraNWGWSelectGroupedLateStartVariable  = "@name('s0') select theString, intPrimitive, avg(longPrimitive) as avgLong, count(boolPrimitive) as cntBool from MyWindowSGVLS group by theString, intPrimitive having theString = var_1_1_1 order by theString, intPrimitive"
)

// infraNWGWWindowShape selects the window schema registered for a case.
type infraNWGWWindowShape int

const (
	// infraNWGWKVRows is the Map schema MySimpleKeyValueMap {key, value}.
	infraNWGWKVRows infraNWGWWindowShape = iota
	// infraNWGWGroupedRows is the (theString, intPrimitive) projection of
	// SupportBean declared by the ord-44 create window.
	infraNWGWGroupedRows
	// infraNWGWGroupedFullRows is the (theString, intPrimitive, longPrimitive,
	// boolPrimitive) projection declared by the ord-55 create window.
	infraNWGWGroupedFullRows
)

// infraNWGWRetention selects the named window's #groupwin retention.
type infraNWGWRetention int

const (
	infraNWGWGroupValueLength2 infraNWGWRetention = iota
	infraNWGWGroupValueTimeBatch
	infraNWGWGroupKeysLength9
)

// infraNWGWConsumer selects the window consumer statement of a case.
type infraNWGWConsumer int

const (
	// infraNWGWConsumerPlain projects key/value without a remove stream.
	infraNWGWConsumerPlain infraNWGWConsumer = iota
	// infraNWGWConsumerIstream projects key/value with the remove stream.
	infraNWGWConsumerIstream
	// infraNWGWConsumerGroupedCount is the ord-44 group-by count(*) consumer.
	infraNWGWConsumerGroupedCount
	// infraNWGWConsumerGroupedVariableAvgCount is the ord-55 grouped
	// avg/count consumer whose having clause reads the runtime variable.
	infraNWGWConsumerGroupedVariableAvgCount
)

type infraNWGWCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	windowName  string
	description string
	createEPL   string
	insertEPL   string
	selectEPL   string
	consumeEPL  string
	deleteEPL   string
	varEPL      string
	onSetEPL    string
	deploys     []string
	listened    map[string]bool
	snapshots   int
	// sends and advances pin the per-case step counts of the send and
	// advance-time ops so raw step drift (an injected or dropped event or
	// clock move) is rejected at load time rather than surfacing later as a
	// differential difference.
	sends    int
	advances int
	// snapModes, snapStatements and snapFields are the per-snapshot pins in
	// scenario order: the comparison mode, the statement iterated and the
	// exact field set the Java assertion reads (empty for a count-only
	// iterator assertion).
	snapModes      []string
	snapStatements []string
	snapFields     [][]string
	shape          infraNWGWWindowShape
	retention      infraNWGWRetention
	consumer       infraNWGWConsumer
	sendFields     []string
	rowFields      []string
}

var infraNWGWCaseSpecs = []infraNWGWCaseSpec{
	{
		name:           "length-per-group",
		ordinal:        26,
		runtimeID:      "java-runtime-083a289ee5f82dd87ad3",
		execution:      "InfraLengthWindowPerGroup",
		windowName:     "MyWindowWPG",
		description:    "per-group #groupwin(value)#length(2) window over the key/value map schema: each group keeps its newest two rows, an over-full group expels its oldest row as old data in the same delta as the triggering insert, a delete removes without expiring, and the iterator walks group-creation order",
		createEPL:      infraNWGWCreateLengthPerGroup,
		insertEPL:      infraNWGWInsertLengthPerGroup,
		selectEPL:      infraNWGWSelectLengthPerGroup,
		deleteEPL:      infraNWGWDeleteLengthPerGroup,
		deploys:        []string{"create", "insert", "s0", "delete"},
		listened:       map[string]bool{"create": true, "s0": true},
		sends:          10,
		advances:       0,
		snapshots:      2,
		snapModes:      []string{"ordered", "ordered"},
		snapStatements: []string{"create", "create"},
		snapFields:     [][]string{{"key", "value"}, {"key", "value"}},
		shape:          infraNWGWKVRows,
		retention:      infraNWGWGroupValueLength2,
		consumer:       infraNWGWConsumerIstream,
		sendFields:     []string{"theString", "longBoxed"},
		rowFields:      []string{"key", "value"},
	},
	{
		name:        "time-batch-per-group",
		ordinal:     27,
		runtimeID:   "java-runtime-accaf82c3c846493832b",
		execution:   "InfraTimeBatchPerGroup",
		windowName:  "MyWindowTBPG",
		description: "per-group #groupwin(value)#time_batch(10 sec) window over the key/value map schema: arrivals buffer silently, each group anchors its boundary at its first arrival, and the 11000 flush concatenates the completed group batches as [E1,E4,E2,E3] for both the window stream and the plain consumer",
		createEPL:   infraNWGWCreateTimeBatchPerGroup,
		insertEPL:   infraNWGWInsertTimeBatchPerGroup,
		selectEPL:   infraNWGWSelectTimeBatchPerGroup,
		deploys:     []string{"create", "insert", "s0"},
		listened:    map[string]bool{"create": true, "s0": true},
		sends:       4,
		advances:    3,
		shape:       infraNWGWKVRows,
		retention:   infraNWGWGroupValueTimeBatch,
		consumer:    infraNWGWConsumerPlain,
		sendFields:  []string{"theString", "longBoxed"},
		rowFields:   []string{"key", "value"},
	},
	{
		name:           "select-grouped-view-late-start",
		ordinal:        44,
		runtimeID:      "java-runtime-3326973240f20b92dced",
		execution:      "InfraSelectGroupedViewLateStart",
		windowName:     "MyWindowSGVS",
		description:    "grouped #groupwin(theString, intPrimitive)#length(9) projection window whose count(*) consumer is deployed after the window is populated: the late preload builds the per-group counts, the window iterator keeps the twelve rows of the ten groups, and the consumer iterates one sorted row per group",
		createEPL:      infraNWGWCreateGroupedLateStart,
		insertEPL:      infraNWGWInsertGroupedLateStart,
		selectEPL:      infraNWGWSelectGroupedLateStart,
		deploys:        []string{"create", "insert", "s0"},
		listened:       map[string]bool{},
		sends:          12,
		advances:       0,
		snapshots:      2,
		snapModes:      []string{"any", "ordered"},
		snapStatements: []string{"create", "s0"},
		snapFields:     [][]string{{}, {"theString", "intPrimitive", "count(*)"}},
		shape:          infraNWGWGroupedRows,
		retention:      infraNWGWGroupKeysLength9,
		consumer:       infraNWGWConsumerGroupedCount,
		sendFields:     []string{"theString", "intPrimitive"},
	},
	{
		name:           "select-grouped-view-late-start-variable-iterate",
		ordinal:        55,
		runtimeID:      "java-runtime-0ec49d4098c9796540b0",
		execution:      "InfraSelectGroupedViewLateStartVariableIterate",
		windowName:     "MyWindowSGVLS",
		description:    "grouped #groupwin(theString, intPrimitive)#length(9) projection window whose variable-filtered avg/count consumer is deployed after the window is populated: the late preload builds the group state including both rows of the non-uniform (c1,1) group, and the having clause reads the runtime variable at iterate time so the on-set trigger switches the visible theString group",
		createEPL:      infraNWGWCreateGroupedLateStartVariable,
		insertEPL:      infraNWGWInsertGroupedLateStartVariable,
		selectEPL:      infraNWGWSelectGroupedLateStartVariable,
		varEPL:         infraNWGWDeclareGroupedLateStartVariable,
		onSetEPL:       infraNWGWOnSetGroupedLateStartVariable,
		deploys:        []string{"create", "insert", "var", "on-set", "s0"},
		listened:       map[string]bool{},
		sends:          12,
		advances:       0,
		snapshots:      3,
		snapModes:      []string{"any", "ordered", "ordered"},
		snapStatements: []string{"create", "s0", "s0"},
		snapFields: [][]string{
			{},
			{"theString", "intPrimitive", "avgLong", "cntBool"},
			{"theString", "intPrimitive", "avgLong", "cntBool"},
		},
		shape:      infraNWGWGroupedFullRows,
		retention:  infraNWGWGroupKeysLength9,
		consumer:   infraNWGWConsumerGroupedVariableAvgCount,
		sendFields: []string{"theString", "intPrimitive", "longPrimitive", "boolPrimitive"},
	},
}

var (
	infraNWGWJavaSources = []string{
		infraNWGWSource,
	}
	infraNWGWJavaRuntimeIDs = []string{
		"java-runtime-083a289ee5f82dd87ad3",
		"java-runtime-accaf82c3c846493832b",
		"java-runtime-3326973240f20b92dced",
		"java-runtime-0ec49d4098c9796540b0",
	}
	infraNWGWJavaExecutions = []string{
		"InfraLengthWindowPerGroup",
		"InfraTimeBatchPerGroup",
		"InfraSelectGroupedViewLateStart",
		"InfraSelectGroupedViewLateStartVariableIterate",
	}
	infraNWGWCases = []string{
		"length-per-group",
		"time-batch-per-group",
		"select-grouped-view-late-start",
		"select-grouped-view-late-start-variable-iterate",
	}
)

// infraNWGWBean mirrors SupportBean; each case validates the exact send fields
// it pins, so the unset fields stay at their Go zero value.
type infraNWGWBean struct {
	TheString     string `esper:"theString"`
	IntBoxed      int    `esper:"intBoxed"`
	LongBoxed     int64  `esper:"longBoxed"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
}

type infraNWGWMarket struct {
	Symbol string `esper:"symbol"`
}

// infraNWGWVariableSet mirrors SupportVariableSetEvent (variableName, value).
type infraNWGWVariableSet struct {
	VariableName string `esper:"variableName"`
	Value        string `esper:"value"`
}

// infraNWGWKV mirrors the MySimpleKeyValueMap window schema of ords 26/27.
type infraNWGWKV struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

// infraNWGWGrouped mirrors the (theString, intPrimitive) window projection of
// ord 44. The Go-type to event-type index points one Go type at one event type,
// so the create-window projection needs its own type rather than reusing
// infraNWGWBean.
type infraNWGWGrouped struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// infraNWGWGroupedFull mirrors the ord-55 window projection.
type infraNWGWGroupedFull struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
}

func infraNWGWCaseSpecFor(name string) (infraNWGWCaseSpec, bool) {
	for _, spec := range infraNWGWCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWGWCaseSpec{}, false
}

func loadInfraNWGWScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWGWId)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWGWId, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWGWId, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWGWId, err)
	}
	if err := requireInfraNWGWFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWGWId, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWGWId ||
		metadata.Description != infraNWGWDescription ||
		metadata.JavaCommit != infraNWGWJavaCommit || metadata.JavaSource != infraNWGWSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWGWId)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWGWId)
	}
	if err := infraNWGWRequireEqual(metadata.JavaRuntimes, infraNWGWJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWGWRequireEqual(metadata.JavaNames, infraNWGWJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWGWId, err)
	}
	if len(rawCases) != len(infraNWGWCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWGWId, len(rawCases), len(infraNWGWCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWGWFields(object,
			"case", "ordinal", "runtimeId", "executionName", "description", "createEpl",
			"insertEpl", "s0Epl", "consumeEpl", "deleteEpl", "varEpl", "onSetEpl",
			"deploys", "listened", "iteratorSnapshots"); err != nil {
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
			VarEPL            string   `json:"varEpl"`
			OnSetEPL          string   `json:"onSetEpl"`
			Deploys           []string `json:"deploys"`
			Listened          []string `json:"listened"`
			IteratorSnapshots int      `json:"iteratorSnapshots"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := infraNWGWCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
		}
		if definition.Description != spec.description {
			return compat.Scenario{}, fmt.Errorf("scenario case %q description does not match the pinned slice description", spec.name)
		}
		if drifted := infraNWGWEPLDrift(spec, definition.CreateEPL, definition.InsertEPL, definition.SelectEPL,
			definition.ConsumeEPL, definition.DeleteEPL, definition.VarEPL, definition.OnSetEPL); len(drifted) != 0 {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source (slots %s)",
				spec.name, strings.Join(drifted, ", "))
		}
		if err := infraNWGWRequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWGWRequireListened(definition.Listened, spec); err != nil {
			return compat.Scenario{}, err
		}
		if definition.IteratorSnapshots != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q iteratorSnapshots = %d, want %d",
				spec.name, definition.IteratorSnapshots, spec.snapshots)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWGWId, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWGWId)
	}
	deployCounts := map[string]int{}
	sendCounts := map[string]int{}
	advanceCounts := map[string]int{}
	caseOrder := make([]string, 0, len(infraNWGWCaseSpecs))
	currentCase := ""
	snapshotCounts := map[string]int{}
	snapshotModes := map[string][]string{}
	snapshotStatements := map[string][]string{}
	snapshotFields := map[string][][]string{}
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
			if err := requireInfraNWGWFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := infraNWGWCaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d selects unknown case %q", index, step.Case)
			}
			currentCase = step.Case
			caseOrder = append(caseOrder, step.Case)
		case "deploy":
			if err := requireInfraNWGWFields(object, "op", "case", "statement", "epl"); err != nil {
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
			spec, ok := infraNWGWCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWGWEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWGWFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := infraNWGWCaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d sends for unknown case %q", index, step.Case)
			}
			if step.Case != currentCase {
				return compat.Scenario{}, fmt.Errorf("scenario step %d sends for case %q outside its block (current case %q)", index, step.Case, currentCase)
			}
			sendCounts[step.Case]++
		case "snapshot":
			if err := requireInfraNWGWFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string   `json:"case"`
				Statement string   `json:"statement"`
				Mode      string   `json:"mode"`
				Fields    []string `json:"fields"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWGWCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshots unknown case %q", index, step.Case)
			}
			if step.Statement != spec.snapStatements[snapshotCounts[step.Case]] {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot statement %q must be %q for case %q",
					index, step.Statement, spec.snapStatements[snapshotCounts[step.Case]], step.Case)
			}
			if step.Mode != spec.snapModes[snapshotCounts[step.Case]] {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot mode %q must be %q for case %q",
					index, step.Mode, spec.snapModes[snapshotCounts[step.Case]], step.Case)
			}
			if err := infraNWGWRequireEqual(step.Fields, spec.snapFields[snapshotCounts[step.Case]], step.Case+" snapshot fields"); err != nil {
				return compat.Scenario{}, err
			}
			snapshotModes[step.Case] = append(snapshotModes[step.Case], step.Mode)
			snapshotStatements[step.Case] = append(snapshotStatements[step.Case], step.Statement)
			snapshotFields[step.Case] = append(snapshotFields[step.Case], step.Fields)
			snapshotCounts[step.Case]++
		case "undeploy":
			if err := requireInfraNWGWFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWGWCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d undeploys unknown case %q", index, step.Case)
			}
			if !infraNWGWContains(spec.deploys, step.Statement) {
				return compat.Scenario{}, fmt.Errorf("scenario step %d undeploys statement %q which case %q never deploys",
					index, step.Statement, step.Case)
			}
		case "advance-time":
			if err := requireInfraNWGWFields(object, "op", "case", "at"); err != nil {
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
			if _, ok := infraNWGWCaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advances unknown case %q", index, step.Case)
			}
			if step.Case != currentCase {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advances case %q outside its block (current case %q)", index, step.Case, currentCase)
			}
			advanceCounts[step.Case]++
		case "undeploy-all":
			if err := requireInfraNWGWFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	for index, spec := range infraNWGWCaseSpecs {
		if index >= len(caseOrder) || caseOrder[index] != spec.name {
			return compat.Scenario{}, fmt.Errorf("scenario case order = %v, want %v", caseOrder, infraNWGWCaseNames())
		}
		if sendCounts[spec.name] != spec.sends {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d send steps, want %d",
				spec.name, sendCounts[spec.name], spec.sends)
		}
		if advanceCounts[spec.name] != spec.advances {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d advance-time steps, want %d",
				spec.name, advanceCounts[spec.name], spec.advances)
		}
		if deployCounts[spec.name] != len(spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d deploy steps, want %d",
				spec.name, deployCounts[spec.name], len(spec.deploys))
		}
		if snapshotCounts[spec.name] != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d snapshot steps, want %d",
				spec.name, snapshotCounts[spec.name], spec.snapshots)
		}
		if err := infraNWGWRequireEqual(snapshotModes[spec.name], spec.snapModes, spec.name+" snapshot modes"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWGWRequireEqual(snapshotStatements[spec.name], spec.snapStatements, spec.name+" snapshot statements"); err != nil {
			return compat.Scenario{}, err
		}
		for index := range spec.snapFields {
			if err := infraNWGWRequireEqual(snapshotFields[spec.name][index], spec.snapFields[index],
				fmt.Sprintf("%s snapshot %d fields", spec.name, index)); err != nil {
				return compat.Scenario{}, err
			}
		}
	}
	var steps []compat.Step
	if err := json.Unmarshal(root["steps"], &steps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", infraNWGWId, err)
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWGWEPLForStep returns the byte-exact EPL of one deploy step; a case
// that does not deploy the statement has an empty slot and rejects it.
func infraNWGWEPLForStep(spec infraNWGWCaseSpec, statement string) (string, bool) {
	switch statement {
	case "create":
		return spec.createEPL, spec.createEPL != ""
	case "insert":
		return spec.insertEPL, spec.insertEPL != ""
	case "s0":
		return spec.selectEPL, spec.selectEPL != ""
	case "delete":
		return spec.deleteEPL, spec.deleteEPL != ""
	case "var":
		return spec.varEPL, spec.varEPL != ""
	case "on-set":
		return spec.onSetEPL, spec.onSetEPL != ""
	default:
		return "", false
	}
}

// infraNWGWEPLDrift names the case-definition EPL slots that differ from the
// pinned Java statement text.
func infraNWGWEPLDrift(spec infraNWGWCaseSpec, create, insert, selectEPL, consume, deleteEPL, variable, onSet string) []string {
	var drifted []string
	slots := []struct {
		name string
		got  string
		want string
	}{
		{"createEpl", create, spec.createEPL},
		{"insertEpl", insert, spec.insertEPL},
		{"s0Epl", selectEPL, spec.selectEPL},
		{"consumeEpl", consume, spec.consumeEPL},
		{"deleteEpl", deleteEPL, spec.deleteEPL},
		{"varEpl", variable, spec.varEPL},
		{"onSetEpl", onSet, spec.onSetEPL},
	}
	for _, slot := range slots {
		if slot.got != slot.want {
			drifted = append(drifted, slot.name)
		}
	}
	return drifted
}

func infraNWGWRequireEqual(got, want []string, label string) error {
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

func infraNWGWRequireListened(got []string, spec infraNWGWCaseSpec) error {
	if len(got) != len(spec.listened) {
		return fmt.Errorf("scenario case %q listened = %v, want %d statements", spec.name, got, len(spec.listened))
	}
	seen := make(map[string]bool, len(got))
	for _, name := range got {
		if !spec.listened[name] {
			return fmt.Errorf("scenario case %q listens to %q which is not in the pinned listener set", spec.name, name)
		}
		if seen[name] {
			return fmt.Errorf("scenario case %q listens to %q twice", spec.name, name)
		}
		seen[name] = true
	}
	return nil
}

// infraNWGWCaseNames returns the pinned case order used by the loader's
// ordering check.
func infraNWGWCaseNames() []string {
	names := make([]string, 0, len(infraNWGWCaseSpecs))
	for _, spec := range infraNWGWCaseSpecs {
		names = append(names, spec.name)
	}
	return names
}

func infraNWGWContains(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

func requireInfraNWGWFields(object map[string]json.RawMessage, names ...string) error {
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

func runInfraNWGWScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWGWId)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWGWCaseSpecs {
		caseTrace, err := runInfraNWGWCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWGWId, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWGWCase(ctx context.Context, scenario compat.Scenario, spec infraNWGWCaseSpec) ([]compat.TraceRecord, error) {
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
	if _, err := esper.RegisterStruct[infraNWGWBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if spec.deleteEPL != "" {
		if _, err := esper.RegisterStruct[infraNWGWMarket](env, "SupportMarketDataBean"); err != nil {
			return nil, err
		}
	}
	if spec.consumer == infraNWGWConsumerGroupedVariableAvgCount {
		if _, err := esper.RegisterStruct[infraNWGWVariableSet](env, "SupportVariableSetEvent"); err != nil {
			return nil, err
		}
	}
	var windowSchema esper.Schema
	var err error
	switch spec.shape {
	case infraNWGWKVRows:
		windowSchema, err = esper.RegisterStruct[infraNWGWKV](env, spec.windowName)
	case infraNWGWGroupedRows:
		windowSchema, err = esper.RegisterStruct[infraNWGWGrouped](env, spec.windowName)
	case infraNWGWGroupedFullRows:
		windowSchema, err = esper.RegisterStruct[infraNWGWGroupedFull](env, spec.windowName)
	default:
		return nil, fmt.Errorf("unexpected window shape %d for case %q", spec.shape, spec.name)
	}
	if err != nil {
		return nil, err
	}
	retention, err := infraNWGWRetentionSpec(spec)
	if err != nil {
		return nil, err
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
	snapshotCount := 0
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
			if step.Statement == "var" {
				// "create variable" carries no statement in the Go chain: the
				// declaration is the environment-level variable registration.
				if spec.varEPL == "" {
					return nil, fmt.Errorf("case %q does not declare a variable", spec.name)
				}
				if err := env.RegisterVariable(infraNWGWVariableName, ""); err != nil {
					return nil, err
				}
				continue
			}
			plan, err := infraNWGWBuildPlan(env, spec, step.Statement)
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
			payload, err := infraNWGWDecodePayload(spec, step)
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
			if snapshotCount >= len(spec.snapFields) {
				return nil, fmt.Errorf("case %q has more snapshot steps than the pinned field sets", spec.name)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return nil, err
			}
			rows := projectRecords(compat.NormalizeResults(result.Batch.New), spec.snapFields[snapshotCount])
			snapshotCount++
			// The count-only iterator assertions pin no fields; both sides
			// emit empty rows and mark the step "any" so the comparison is a
			// count comparison.
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
			// The Java suite tears the deployment down at this point; teardown
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

// infraNWGWRetentionSpec maps a case onto its pinned #groupwin retention.
func infraNWGWRetentionSpec(spec infraNWGWCaseSpec) (esper.WindowSpec, error) {
	switch spec.retention {
	case infraNWGWGroupValueLength2:
		return esper.GroupWindow(esper.Field[infraNWGWKV, int64]("value"), esper.LengthWindow(2)), nil
	case infraNWGWGroupValueTimeBatch:
		return esper.GroupWindow(esper.Field[infraNWGWKV, int64]("value"), esper.TimeBatch(10*time.Second)), nil
	case infraNWGWGroupKeysLength9:
		switch spec.shape {
		case infraNWGWGroupedRows:
			return esper.GroupWindowKeys([]esper.Expr{
				esper.Field[infraNWGWGrouped, string]("theString"),
				esper.Field[infraNWGWGrouped, int]("intPrimitive"),
			}, esper.LengthWindow(9)), nil
		case infraNWGWGroupedFullRows:
			return esper.GroupWindowKeys([]esper.Expr{
				esper.Field[infraNWGWGroupedFull, string]("theString"),
				esper.Field[infraNWGWGroupedFull, int]("intPrimitive"),
			}, esper.LengthWindow(9)), nil
		default:
			return nil, fmt.Errorf("case %q: grouped retention needs a grouped window shape", spec.name)
		}
	default:
		return nil, fmt.Errorf("case %q: unexpected retention %d", spec.name, spec.retention)
	}
}

// infraNWGWBuildPlan maps one scenario deploy statement onto the typed chain.
func infraNWGWBuildPlan(env *esper.Environment, spec infraNWGWCaseSpec, statement string) (esper.Plan, error) {
	switch statement {
	case "create":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		source := esper.From[infraNWGWBean](env, "SupportBean")
		switch spec.shape {
		case infraNWGWKVRows:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWGWBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWGWBean, int64]("longBoxed")),
			).Query(esper.StatementName("insert")))
		case infraNWGWGroupedRows:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("theString", esper.Field[infraNWGWBean, string]("theString")),
				esper.SetColumn("intPrimitive", esper.Field[infraNWGWBean, int]("intPrimitive")),
			).Query(esper.StatementName("insert")))
		case infraNWGWGroupedFullRows:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("theString", esper.Field[infraNWGWBean, string]("theString")),
				esper.SetColumn("intPrimitive", esper.Field[infraNWGWBean, int]("intPrimitive")),
				esper.SetColumn("longPrimitive", esper.Field[infraNWGWBean, int64]("longPrimitive")),
				esper.SetColumn("boolPrimitive", esper.Field[infraNWGWBean, bool]("boolPrimitive")),
			).Query(esper.StatementName("insert")))
		default:
			return esper.Plan{}, fmt.Errorf("case %q: unexpected insert for window shape %d", spec.name, spec.shape)
		}
	case "s0":
		return infraNWGWBuildConsumer(env, spec)
	case "delete":
		return env.Build(esper.OnEvent(esper.From[infraNWGWMarket](env, "SupportMarketDataBean")).
			DeleteFromNamedWindow(spec.windowName,
				esper.Equal[string](esper.NamedWindowField[string]("key"),
					esper.Field[infraNWGWMarket, string]("symbol"))).
			Query(esper.StatementName("delete")))
	case "on-set":
		return env.Build(esper.OnEvent(esper.From[infraNWGWVariableSet](env, "SupportVariableSetEvent").
			Filter(esper.Equal[string](
				esper.Field[infraNWGWVariableSet, string]("variableName"),
				esper.Literal(infraNWGWVariableName)))).
			SetVariable(infraNWGWVariableName, esper.Field[infraNWGWVariableSet, string]("value")).
			Query(esper.StatementName("on-set")))
	default:
		return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
	}
}

// infraNWGWBuildConsumer maps the case's window consumer onto the typed chain.
func infraNWGWBuildConsumer(env *esper.Environment, spec infraNWGWCaseSpec) (esper.Plan, error) {
	switch spec.consumer {
	case infraNWGWConsumerPlain, infraNWGWConsumerIstream:
		query := esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int64]("value")),
		)
		if spec.consumer == infraNWGWConsumerIstream {
			return env.Build(query.Query(esper.StatementName("s0"), esper.WithOldStream()))
		}
		return env.Build(query.Query(esper.StatementName("s0")))
	case infraNWGWConsumerGroupedCount:
		theString := esper.Field[any, string]("theString")
		intPrimitive := esper.Field[any, int]("intPrimitive")
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			GroupBy(theString, intPrimitive).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("intPrimitive", intPrimitive),
				esper.Alias("count(*)", esper.CountAll()),
			).
			Query(
				esper.StatementName("s0"),
				esper.OrderBy(esper.Ascending(theString), esper.Ascending(intPrimitive)),
			))
	case infraNWGWConsumerGroupedVariableAvgCount:
		theString := esper.Field[any, string]("theString")
		intPrimitive := esper.Field[any, int]("intPrimitive")
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			GroupBy(theString, intPrimitive).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("intPrimitive", intPrimitive),
				esper.Alias("avgLong", esper.Avg[int64](esper.Field[any, int64]("longPrimitive"))),
				esper.Alias("cntBool", esper.Count[bool](esper.Field[any, bool]("boolPrimitive"))),
			).
			Having(esper.Equal[string](theString, esper.VariableRef[string](infraNWGWVariableName))).
			Query(
				esper.StatementName("s0"),
				esper.OrderBy(esper.Ascending(theString), esper.Ascending(intPrimitive)),
			))
	default:
		return esper.Plan{}, fmt.Errorf("case %q: unexpected consumer %d", spec.name, spec.consumer)
	}
}

func infraNWGWDecodePayload(spec infraNWGWCaseSpec, step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWGWFields(fields, spec.sendFields...); err != nil {
			return nil, fmt.Errorf("case %q SupportBean payload: %w", spec.name, err)
		}
		var bean infraNWGWBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWGWFields(fields, "symbol"); err != nil {
			return nil, fmt.Errorf("case %q SupportMarketDataBean payload: %w", spec.name, err)
		}
		var market infraNWGWMarket
		if err := json.Unmarshal(step.Payload, &market); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return market, nil
	case "SupportVariableSetEvent":
		if err := requireInfraNWGWFields(fields, "variableName", "value"); err != nil {
			return nil, fmt.Errorf("case %q SupportVariableSetEvent payload: %w", spec.name, err)
		}
		var variableSet infraNWGWVariableSet
		if err := json.Unmarshal(step.Payload, &variableSet); err != nil {
			return nil, fmt.Errorf("decode SupportVariableSetEvent: %w", err)
		}
		return variableSet, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
