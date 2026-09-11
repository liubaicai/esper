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

// Parity coverage for the first differential slice of InfraNamedWindowViews
// (58 executions, previously implemented-not-differential): the keep-all
// named-window view with a projection type and the four on-delete alias
// spellings that share the tryCreateWindow consumer matrix.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 1  InfraKeepAllSceneTwo   java-runtime-14f3c7199cf587fea29a
//   - ord 39 InfraWithDeleteUseAs   java-runtime-6d4a6e1168f100add522
//   - ord 40 InfraWithDeleteFirstAs java-runtime-8fe8d92928db5eba5fc3
//   - ord 41 InfraWithDeleteSecondAs java-runtime-b47bdbdf2ac26de6cb0d
//   - ord 42 InfraWithDeleteNoAs    java-runtime-594ea469769e1990e60f
//
// The slice pins a keep-all named window fed by an insert-into projection,
// deleted through an on-trigger statement, and observed through the window's
// own statement listener plus three consumers: s0 doubles the value, s2
// groups sums per key with the Esper first-row insert/remove pair (the group's
// prior row is null when the group is created and disappears with it), and s3
// filters value >= 10. The four WithDelete* executions differ only in the EPL
// spelling of the window type (map schema versus projection of the map schema)
// and of the delete aliases; the typed Go API expresses all four with one plan
// shape because trigger and window fields are qualified by role rather than by
// stream alias.
//
// The Java suite attaches a listener to the on-delete statement but never
// asserts its payload: assertListenerInvoked("delete") reads
// getIsInvokedAndReset(), which is still true from the previous successful
// delete (on-delete delivers nothing at all when no row matches). The trace
// therefore excludes that listener on both sides and pins the deleted rows
// through the window statement's old rows, the consumer remove-stream rows and
// the window iterator snapshots.
const infraNWKDID = "infra-named-window-keepall-delete"

const infraNWKDDescription = "InfraNamedWindowViews keep-all named-window view slice: the legacy win:keepall() window over a theString/intBoxed projection with insert-into(intBoxed) and the four on-delete alias spellings over the key/value map schema with the shared tryCreateWindow consumer matrix (s0 value*2, s2 grouped sum with Esper first-row insert/remove pairs, s3 value >= 10 filter), captured from listener callbacks and ordered window or consumer iterator snapshots (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWKDJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWKDSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// Byte-exact EPL pins (InfraNamedWindowViews.java lines 195/199/203 for
// ordinal 1, 413/420/427/434 plus the shared 3516-3519 statements and the
// 414/421/428/435 delete statements for ordinals 39-42).
const (
	infraNWKDCreateSceneTwo = "@name('create') @public create window MyWindow.win:keepall() as select theString as key, intBoxed as value from SupportBean"
	infraNWKDInsertSceneTwo = "@name('insert') insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"
	infraNWKDDeleteSceneTwo = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"

	infraNWKDCreateUseAs    = "@name('create') @public create window MyWindow#keepall as MySimpleKeyValueMap"
	infraNWKDCreateFirstAs  = "@name('create') @public create window MyWindow#keepall as select key, value from MySimpleKeyValueMap"
	infraNWKDCreateSecondAs = "@name('create') @public create window MyWindow#keepall as MySimpleKeyValueMap"
	infraNWKDCreateNoAs     = "@name('create') @public create window MyWindow#keepall as select key as key, value as value from MySimpleKeyValueMap"

	infraNWKDInsertShared = "@name('insert') insert into MyWindow select theString as key, longBoxed as value from SupportBean"
	infraNWKDS0           = "@name('s0') select irstream key, value*2 as value from MyWindow"
	infraNWKDS2           = "@name('s2') select irstream key, sum(value) as value from MyWindow group by key"
	infraNWKDS3           = "@name('s3') select irstream key, value from MyWindow where value >= 10"

	infraNWKDDeleteUseAs    = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"
	infraNWKDDeleteFirstAs  = "@name('delete') on SupportMarketDataBean delete from MyWindow as s1 where symbol = s1.key"
	infraNWKDDeleteSecondAs = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow where s0.symbol = key"
	infraNWKDDeleteNoAs     = "@name('delete') on SupportMarketDataBean delete from MyWindow where symbol = key"
)

// infraNWKDValueKind selects the window value type of a case: the scene-two
// window projects SupportBean.intBoxed (Java Integer) while the WithDelete
// family declares value as the MySimpleKeyValueMap long column.
type infraNWKDValueKind int

const (
	infraNWKDValueInt infraNWKDValueKind = iota
	infraNWKDValueLong
)

// infraNWKDCaseSpec pins one Java execution: identity, EPL spelling, the
// deployment fan-out whose order the trace pins, the window value type and the
// listened statements. The listened set excludes "delete" for the reason
// documented on the chain id.
type infraNWKDCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	createEPL   string
	deleteEPL   string
	deploys     []string
	valueKind   infraNWKDValueKind
	listened    map[string]bool
	snapshots   int
	sendKind    string
	sendFields  []string
	description string
}

var infraNWKDCaseSpecs = []infraNWKDCaseSpec{
	{
		name:        "keepall-scene-two",
		ordinal:     1,
		runtimeID:   "java-runtime-14f3c7199cf587fea29a",
		execution:   "InfraKeepAllSceneTwo",
		createEPL:   infraNWKDCreateSceneTwo,
		deleteEPL:   infraNWKDDeleteSceneTwo,
		deploys:     []string{"create", "insert", "delete"},
		valueKind:   infraNWKDValueInt,
		listened:    map[string]bool{"create": true},
		snapshots:   11,
		sendKind:    "intBoxed",
		sendFields:  []string{"theString", "intBoxed"},
		description: "keep-all projection window over theString/intBoxed with the legacy win:keepall() spelling; insert-into column list with irstream, on-delete alias on both sides, window listener new/old rows across seven inserts and three deletes with eleven ordered iterator snapshots",
	},
	{
		name:        "with-delete-use-as",
		ordinal:     39,
		runtimeID:   "java-runtime-6d4a6e1168f100add522",
		execution:   "InfraWithDeleteUseAs",
		createEPL:   infraNWKDCreateUseAs,
		deleteEPL:   infraNWKDDeleteUseAs,
		deploys:     []string{"create", "insert", "s0", "s2", "s3", "delete"},
		valueKind:   infraNWKDValueLong,
		listened:    map[string]bool{"create": true, "s0": true, "s2": true, "s3": true},
		snapshots:   9,
		sendKind:    "longBoxed",
		sendFields:  []string{"theString", "longBoxed"},
		description: "map-schema window with aliases on both trigger and window (as s0 / as s1) in the on-delete statement",
	},
	{
		name:        "with-delete-first-as",
		ordinal:     40,
		runtimeID:   "java-runtime-8fe8d92928db5eba5fc3",
		execution:   "InfraWithDeleteFirstAs",
		createEPL:   infraNWKDCreateFirstAs,
		deleteEPL:   infraNWKDDeleteFirstAs,
		deploys:     []string{"create", "insert", "s0", "s2", "s3", "delete"},
		valueKind:   infraNWKDValueLong,
		listened:    map[string]bool{"create": true, "s0": true, "s2": true, "s3": true},
		snapshots:   9,
		sendKind:    "longBoxed",
		sendFields:  []string{"theString", "longBoxed"},
		description: "projection window (select key, value from the map type) with the window-side alias only (as s1) and an unqualified trigger property",
	},
	{
		name:        "with-delete-second-as",
		ordinal:     41,
		runtimeID:   "java-runtime-b47bdbdf2ac26de6cb0d",
		execution:   "InfraWithDeleteSecondAs",
		createEPL:   infraNWKDCreateSecondAs,
		deleteEPL:   infraNWKDDeleteSecondAs,
		deploys:     []string{"create", "insert", "s0", "s2", "s3", "delete"},
		valueKind:   infraNWKDValueLong,
		listened:    map[string]bool{"create": true, "s0": true, "s2": true, "s3": true},
		snapshots:   9,
		sendKind:    "longBoxed",
		sendFields:  []string{"theString", "longBoxed"},
		description: "map-schema window with the trigger-side alias only (as s0) and an unqualified window property",
	},
	{
		name:        "with-delete-no-as",
		ordinal:     42,
		runtimeID:   "java-runtime-594ea469769e1990e60f",
		execution:   "InfraWithDeleteNoAs",
		createEPL:   infraNWKDCreateNoAs,
		deleteEPL:   infraNWKDDeleteNoAs,
		deploys:     []string{"create", "insert", "s0", "s2", "s3", "delete"},
		valueKind:   infraNWKDValueLong,
		listened:    map[string]bool{"create": true, "s0": true, "s2": true, "s3": true},
		snapshots:   9,
		sendKind:    "longBoxed",
		sendFields:  []string{"theString", "longBoxed"},
		description: "projection window with the value alias preserved and no aliases in the on-delete statement",
	},
}

var (
	infraNWKDJavaSources = []string{
		infraNWKDSource,
	}
	infraNWKDJavaRuntimeIDs = []string{
		"java-runtime-14f3c7199cf587fea29a",
		"java-runtime-6d4a6e1168f100add522",
		"java-runtime-8fe8d92928db5eba5fc3",
		"java-runtime-b47bdbdf2ac26de6cb0d",
		"java-runtime-594ea469769e1990e60f",
	}
	infraNWKDJavaExecutions = []string{
		"InfraKeepAllSceneTwo",
		"InfraWithDeleteUseAs",
		"InfraWithDeleteFirstAs",
		"InfraWithDeleteSecondAs",
		"InfraWithDeleteNoAs",
	}
	infraNWKDCases = []string{
		"keepall-scene-two",
		"with-delete-use-as",
		"with-delete-first-as",
		"with-delete-second-as",
		"with-delete-no-as",
	}
)

// infraNWKDBean mirrors SupportBean for both window flavors; each case
// validates that its sends carry exactly the pinned value field.
type infraNWKDBean struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
	LongBoxed int64  `esper:"longBoxed"`
}

type infraNWKDMarket struct {
	Symbol string `esper:"symbol"`
}

type infraNWKDKVLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type infraNWKDKVInt struct {
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

func infraNWKDCaseSpecFor(name string) (infraNWKDCaseSpec, bool) {
	for _, spec := range infraNWKDCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWKDCaseSpec{}, false
}

func loadInfraNWKDScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWKDID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWKDID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWKDID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWKDID, err)
	}
	if err := requireInfraNWKDFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWKDID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWKDID ||
		metadata.Description != infraNWKDDescription ||
		metadata.JavaCommit != infraNWKDJavaCommit || metadata.JavaSource != infraNWKDSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWKDID)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWKDID)
	}
	if err := infraNWKDRequireEqual(metadata.JavaRuntimes, infraNWKDJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWKDRequireEqual(metadata.JavaNames, infraNWKDJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWKDID, err)
	}
	if len(rawCases) != len(infraNWKDCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWKDID, len(rawCases), len(infraNWKDCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWKDFields(object,
			"case", "ordinal", "runtimeId", "executionName", "description", "createEpl",
			"deleteEpl", "deploys", "listened", "iteratorSnapshots"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case              string   `json:"case"`
			Ordinal           int      `json:"ordinal"`
			RuntimeID         string   `json:"runtimeId"`
			ExecutionName     string   `json:"executionName"`
			Description       string   `json:"description"`
			CreateEPL         string   `json:"createEpl"`
			DeleteEPL         string   `json:"deleteEpl"`
			Deploys           []string `json:"deploys"`
			Listened          []string `json:"listened"`
			IteratorSnapshots int      `json:"iteratorSnapshots"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := infraNWKDCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
		}
		if definition.CreateEPL != spec.createEPL || definition.DeleteEPL != spec.deleteEPL {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source", spec.name)
		}
		if definition.Description != spec.description {
			return compat.Scenario{}, fmt.Errorf("scenario case %q description does not match the pinned slice description", spec.name)
		}
		if err := infraNWKDRequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWKDRequireListened(definition.Listened, spec); err != nil {
			return compat.Scenario{}, err
		}
		if definition.IteratorSnapshots != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q iteratorSnapshots = %d, want %d",
				spec.name, definition.IteratorSnapshots, spec.snapshots)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWKDID, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWKDID)
	}
	deployCounts := map[string]int{}
	snapshotCounts := map[string]int{}
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
			if err := requireInfraNWKDFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWKDFields(object, "op", "case", "statement", "epl"); err != nil {
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
			spec, ok := infraNWKDCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWKDEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWKDFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWKDFields(object, "op", "case", "statement", "mode"); err != nil {
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
			if step.Mode != "ordered" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot mode %q is not ordered", index, step.Mode)
			}
			if step.Statement != "create" && step.Statement != "s0" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshots unexpected statement %q", index, step.Statement)
			}
			snapshotCounts[step.Case]++
		case "undeploy-all":
			if err := requireInfraNWKDFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	for _, spec := range infraNWKDCaseSpecs {
		if deployCounts[spec.name] != len(spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d deploy steps, want %d",
				spec.name, deployCounts[spec.name], len(spec.deploys))
		}
		if snapshotCounts[spec.name] != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d snapshot steps, want %d",
				spec.name, snapshotCounts[spec.name], spec.snapshots)
		}
	}
	var steps []compat.Step
	if err := json.Unmarshal(root["steps"], &steps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", infraNWKDID, err)
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWKDEPLForStep returns the byte-exact EPL of one deploy step.
func infraNWKDEPLForStep(spec infraNWKDCaseSpec, statement string) (string, bool) {
	switch statement {
	case "create":
		return spec.createEPL, true
	case "delete":
		return spec.deleteEPL, true
	case "insert":
		if spec.valueKind == infraNWKDValueInt {
			return infraNWKDInsertSceneTwo, true
		}
		return infraNWKDInsertShared, true
	case "s0":
		if spec.valueKind == infraNWKDValueLong {
			return infraNWKDS0, true
		}
		return "", false
	case "s2":
		if spec.valueKind == infraNWKDValueLong {
			return infraNWKDS2, true
		}
		return "", false
	case "s3":
		if spec.valueKind == infraNWKDValueLong {
			return infraNWKDS3, true
		}
		return "", false
	default:
		return "", false
	}
}

func infraNWKDRequireEqual(got, want []string, label string) error {
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

func infraNWKDRequireListened(got []string, spec infraNWKDCaseSpec) error {
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

func requireInfraNWKDFields(object map[string]json.RawMessage, names ...string) error {
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

// runInfraNWKDScenario replays the five pinned executions, one fresh
// environment, window and engine per case.
func runInfraNWKDScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWKDID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWKDCaseSpecs {
		caseTrace, err := runInfraNWKDCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWKDID, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWKDCase(ctx context.Context, scenario compat.Scenario, spec infraNWKDCaseSpec) ([]compat.TraceRecord, error) {
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
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
	}

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWKDBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWKDMarket](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	var windowSchema esper.Schema
	var err error
	if spec.valueKind == infraNWKDValueInt {
		windowSchema, err = esper.RegisterStruct[infraNWKDKVInt](env, "MyWindow")
	} else {
		windowSchema, err = esper.RegisterStruct[infraNWKDKVLong](env, "MyWindow")
	}
	if err != nil {
		return nil, err
	}
	if _, err := esper.CreateNamedWindow(env, "MyWindow", windowSchema,
		esper.NamedWindowRetention(esper.KeepAll())); err != nil {
		return nil, err
	}

	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(spec.runtimeID),
		esper.WithStartTime(now),
	)
	defer func() { _ = engine.Close(context.Background()) }()

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
			plan, err := infraNWKDBuildPlan(env, spec, step.Statement)
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
			payload, err := infraNWKDDecodePayload(spec, step)
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
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "snapshot",
				Statement: step.Statement,
				Sequence:  0,
				Time:      compat.FormatTraceTime(now),
				New:       compat.NormalizeResults(result.Batch.New),
			})
		case "undeploy-all":
			// The Java suite tears the module down at this point; teardown
			// emits no trace records, and the next case uses a fresh
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

// infraNWKDBuildPlan maps one scenario deploy statement onto the typed chain.
func infraNWKDBuildPlan(env *esper.Environment, spec infraNWKDCaseSpec, statement string) (esper.Plan, error) {
	switch statement {
	case "create":
		return env.Build(esper.FromNamedWindow(env, "MyWindow").
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		source := esper.From[infraNWKDBean](env, "SupportBean")
		if spec.valueKind == infraNWKDValueInt {
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyWindow",
				esper.SetColumn("key", esper.Field[infraNWKDBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWKDBean, int]("intBoxed")),
			).Query(esper.StatementName("insert")))
		}
		return env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyWindow",
			esper.SetColumn("key", esper.Field[infraNWKDBean, string]("theString")),
			esper.SetColumn("value", esper.Field[infraNWKDBean, int64]("longBoxed")),
		).Query(esper.StatementName("insert")))
	case "delete":
		return env.Build(esper.OnEvent(esper.From[infraNWKDMarket](env, "SupportMarketDataBean")).
			DeleteFromNamedWindow("MyWindow",
				esper.Equal[string](esper.NamedWindowField[string]("key"),
					esper.Field[infraNWKDMarket, string]("symbol"))).
			Query(esper.StatementName("delete")))
	case "s0":
		if spec.valueKind == infraNWKDValueInt {
			return env.Build(esper.FromNamedWindow(env, "MyWindow").Select(
				esper.Alias("key", esper.Field[any, string]("key")),
				esper.Alias("value", esper.Multiply[int](esper.Field[any, int]("value"), esper.Literal(2))),
			).Query(esper.StatementName("s0"), esper.WithOldStream()))
		}
		return env.Build(esper.FromNamedWindow(env, "MyWindow").Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Multiply[int64](esper.Field[any, int64]("value"), esper.Literal[int64](2))),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "s2":
		if spec.valueKind == infraNWKDValueInt {
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				GroupBy(esper.Field[any, string]("key")).
				Select(
					esper.Alias("key", esper.Field[any, string]("key")),
					esper.Alias("value", esper.Sum[int](esper.Field[any, int]("value"))),
				).Query(esper.StatementName("s2"), esper.WithOldStream()))
		}
		return env.Build(esper.FromNamedWindow(env, "MyWindow").
			GroupBy(esper.Field[any, string]("key")).
			Select(
				esper.Alias("key", esper.Field[any, string]("key")),
				esper.Alias("value", esper.Sum[int64](esper.Field[any, int64]("value"))),
			).Query(esper.StatementName("s2"), esper.WithOldStream()))
	case "s3":
		if spec.valueKind == infraNWKDValueInt {
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				Filter(esper.GreaterOrEqual[int](esper.Field[any, int]("value"), esper.Literal(10))).
				Select(
					esper.Alias("key", esper.Field[any, string]("key")),
					esper.Alias("value", esper.Field[any, int]("value")),
				).Query(esper.StatementName("s3"), esper.WithOldStream()))
		}
		return env.Build(esper.FromNamedWindow(env, "MyWindow").
			Filter(esper.GreaterOrEqual[int64](esper.Field[any, int64]("value"), esper.Literal[int64](10))).
			Select(
				esper.Alias("key", esper.Field[any, string]("key")),
				esper.Alias("value", esper.Field[any, int64]("value")),
			).Query(esper.StatementName("s3"), esper.WithOldStream()))
	default:
		return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
	}
}

func infraNWKDDecodePayload(spec infraNWKDCaseSpec, step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWKDFields(fields, spec.sendFields...); err != nil {
			return nil, fmt.Errorf("case %q SupportBean payload: %w", spec.name, err)
		}
		var bean infraNWKDBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWKDFields(fields, "symbol"); err != nil {
			return nil, fmt.Errorf("SupportMarketDataBean payload: %w", err)
		}
		var market infraNWKDMarket
		if err := json.Unmarshal(step.Payload, &market); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return market, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
