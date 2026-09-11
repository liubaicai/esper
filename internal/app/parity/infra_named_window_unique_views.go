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

// Parity coverage for the third differential slice of InfraNamedWindowViews:
// the unique and first-unique data windows over the key/value map schema and a
// projection.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 32 InfraUnique          java-runtime-63daa6cfa27a786c0480
//   - ord 33 InfraUniqueSceneTwo  java-runtime-1ee31df7428f7c8e85eb
//   - ord 34 InfraFirstUnique     java-runtime-bb670bc21e9d4333cefd
//
// The slice pins two key-retention rules: a unique window keeps the most recent
// row per key, so re-inserting a key delivers one invocation carrying the new
// row and the replaced row and frees the key again on delete, while a
// first-unique window swallows duplicate inserts with no callback anywhere and
// frees the key once the stored row is deleted. Multi-row iterator snapshots
// are non-contractual (the Java views iterate a HashMap), so the suite asserts
// them any-order; the chain carries those steps as mode "any" and sorts the
// rows canonically on both sides, while single-row snapshots stay ordered.
//
// As in the sibling chains, the Java suite attaches an on-delete listener but
// never asserts its payload, so the delete listener is excluded from the trace
// on both sides: matching deletes DO deliver the removed rows to that listener
// as new data, so the exclusion rests on the never-asserted payload rather than
// on silence, and deleted rows are observed through the window statement's old
// rows, the consumer old rows and the iterator.
const infraNWRUId = "infra-named-window-unique-views"

const infraNWRUDescription = "InfraNamedWindowViews unique-slice: the MySimpleKeyValueMap unique and firstunique windows and the projection unique window with insert-into projections, on-delete triggers and irstream consumers, captured from listener callbacks and ordered or canonical any-mode window iterator snapshots (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWRUJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWRUSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// Byte-exact EPL pins (Java lines 2793-2797 for ord 32, 2837/2843/2849/2855
// for ord 33 and 2908-2912 for ord 34).
const (
	infraNWRUCreateUnique = "@name('create') create window MyWindowUN#unique(key) as MySimpleKeyValueMap"
	infraNWRUInsertUnique = "insert into MyWindowUN select theString as key, longBoxed as value from SupportBean"
	infraNWRUSelectUnique = "@name('s0') select irstream key, value as value from MyWindowUN"
	infraNWRUDeleteUnique = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowUN as s1 where s0.symbol = s1.key"

	infraNWRUCreateUniqueTwo  = "@name('create') @public create window MyWindow#unique(key) as select theString as key, intBoxed as value from SupportBean"
	infraNWRUInsertUniqueTwo  = "@name('insert') insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"
	infraNWRUConsumeUniqueTwo = "@name('consume') select irstream key, value as value from MyWindow"
	infraNWRUDeleteUniqueTwo  = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"

	infraNWRUCreateFirstUnique = "@name('create') create window MyWindowFU#firstunique(key) as MySimpleKeyValueMap"
	infraNWRUInsertFirstUnique = "insert into MyWindowFU select theString as key, longBoxed as value from SupportBean"
	infraNWRUSelectFirstUnique = "@name('s0') select irstream key, value as value from MyWindowFU"
	infraNWRUDeleteFirstUnique = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowFU as s1 where s0.symbol = s1.key"
)

// infraNWRUSendKind selects the SupportBean payload shape and the projected row
// fields of a case.
type infraNWRUSendKind int

const (
	infraNWRUUniqueLong infraNWRUSendKind = iota
	infraNWRUUniqueInt
	infraNWRUFirstUniqueLong
)

type infraNWRUCaseSpec struct {
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
	snapModes   []string
	deploys     []string
	listened    map[string]bool
	snapshots   int
	sendKind    infraNWRUSendKind
	sendFields  []string
	rowFields   []string
}

var infraNWRUCaseSpecs = []infraNWRUCaseSpec{
	{
		name:        "unique",
		ordinal:     32,
		runtimeID:   "java-runtime-63daa6cfa27a786c0480",
		execution:   "InfraUnique",
		windowName:  "MyWindowUN",
		description: "unique window over the key/value map schema: replacement delivers one invocation carrying the new row and the replaced row, re-admitted keys after delete, and any-mode two-row snapshots",
		createEPL:   infraNWRUCreateUnique,
		insertEPL:   infraNWRUInsertUnique,
		selectEPL:   infraNWRUSelectUnique,
		deleteEPL:   infraNWRUDeleteUnique,
		deploys:     []string{"create", "insert", "s0", "delete"},
		listened:    map[string]bool{"create": true, "s0": true},
		snapshots:   6,
		snapModes:   []string{"ordered", "any", "ordered", "any", "any", "ordered"},
		sendKind:    infraNWRUUniqueLong,
		sendFields:  []string{"theString", "longBoxed"},
		rowFields:   []string{"key", "value"},
	},
	{
		name:        "unique-scene-two",
		ordinal:     33,
		runtimeID:   "java-runtime-1ee31df7428f7c8e85eb",
		execution:   "InfraUniqueSceneTwo",
		windowName:  "MyWindow",
		description: "projection unique window over theString/intBoxed with four module deployments and only the window statement listened; every snapshot is any-mode",
		createEPL:   infraNWRUCreateUniqueTwo,
		insertEPL:   infraNWRUInsertUniqueTwo,
		consumeEPL:  infraNWRUConsumeUniqueTwo,
		deleteEPL:   infraNWRUDeleteUniqueTwo,
		deploys:     []string{"create", "insert", "consume", "delete"},
		listened:    map[string]bool{"create": true},
		snapshots:   5,
		snapModes:   []string{"any", "any", "any", "any", "any"},
		sendKind:    infraNWRUUniqueInt,
		sendFields:  []string{"theString", "intBoxed"},
		rowFields:   []string{"key", "value"},
	},
	{
		name:        "firstunique",
		ordinal:     34,
		runtimeID:   "java-runtime-bb670bc21e9d4333cefd",
		execution:   "InfraFirstUnique",
		windowName:  "MyWindowFU",
		description: "firstunique window over the key/value map schema: duplicate keys are swallowed without any callback, deletes free the key for re-admission, and any-mode snapshots cover the two-row phases",
		createEPL:   infraNWRUCreateFirstUnique,
		insertEPL:   infraNWRUInsertFirstUnique,
		selectEPL:   infraNWRUSelectFirstUnique,
		deleteEPL:   infraNWRUDeleteFirstUnique,
		deploys:     []string{"create", "insert", "s0", "delete"},
		listened:    map[string]bool{"create": true, "s0": true},
		snapshots:   7,
		snapModes:   []string{"ordered", "any", "ordered", "ordered", "any", "any", "ordered"},
		sendKind:    infraNWRUFirstUniqueLong,
		sendFields:  []string{"theString", "longBoxed"},
		rowFields:   []string{"key", "value"},
	},
}

var (
	infraNWRUJavaSources = []string{
		infraNWRUSource,
	}
	infraNWRUJavaRuntimeIDs = []string{
		"java-runtime-63daa6cfa27a786c0480",
		"java-runtime-1ee31df7428f7c8e85eb",
		"java-runtime-bb670bc21e9d4333cefd",
	}
	infraNWRUJavaExecutions = []string{
		"InfraUnique",
		"InfraUniqueSceneTwo",
		"InfraFirstUnique",
	}
	infraNWRUCases = []string{
		"unique",
		"unique-scene-two",
		"firstunique",
	}
)

// infraNWRUBean mirrors SupportBean; each case validates the exact send fields
// it pins (every case sets theString plus one value field, either intBoxed or
// longBoxed, and the unset fields stay at their Go zero value).
type infraNWRUBean struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
	LongBoxed int64  `esper:"longBoxed"`
}

type infraNWRUMarket struct {
	Symbol string `esper:"symbol"`
}

type infraNWRUKVLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type infraNWRUKVInt struct {
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

func infraNWRUCaseSpecFor(name string) (infraNWRUCaseSpec, bool) {
	for _, spec := range infraNWRUCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWRUCaseSpec{}, false
}

func loadInfraNWRUScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWRUId)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWRUId, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRUId, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRUId, err)
	}
	if err := requireInfraNWRUFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWRUId, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWRUId ||
		metadata.Description != infraNWRUDescription ||
		metadata.JavaCommit != infraNWRUJavaCommit || metadata.JavaSource != infraNWRUSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWRUId)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWRUId)
	}
	if err := infraNWRURequireEqual(metadata.JavaRuntimes, infraNWRUJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWRURequireEqual(metadata.JavaNames, infraNWRUJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWRUId, err)
	}
	if len(rawCases) != len(infraNWRUCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWRUId, len(rawCases), len(infraNWRUCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWRUFields(object,
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
		spec := infraNWRUCaseSpecs[index]
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
		if err := infraNWRURequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWRURequireListened(definition.Listened, spec); err != nil {
			return compat.Scenario{}, err
		}
		if definition.IteratorSnapshots != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q iteratorSnapshots = %d, want %d",
				spec.name, definition.IteratorSnapshots, spec.snapshots)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWRUId, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWRUId)
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
			if err := requireInfraNWRUFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWRUFields(object, "op", "case", "statement", "epl"); err != nil {
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
			spec, ok := infraNWRUCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWRUEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWRUFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWRUFields(object, "op", "case", "statement", "mode"); err != nil {
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
			if step.Statement != "create" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot must target the create statement", index)
			}
			snapshotModes[step.Case] = append(snapshotModes[step.Case], step.Mode)
			snapshotCounts[step.Case]++
		case "undeploy":
			if err := requireInfraNWRUFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWRUFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	for _, spec := range infraNWRUCaseSpecs {
		if deployCounts[spec.name] != len(spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d deploy steps, want %d",
				spec.name, deployCounts[spec.name], len(spec.deploys))
		}
		if snapshotCounts[spec.name] != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d snapshot steps, want %d",
				spec.name, snapshotCounts[spec.name], spec.snapshots)
		}
		if err := infraNWRURequireEqual(snapshotModes[spec.name], spec.snapModes, spec.name+" snapshot modes"); err != nil {
			return compat.Scenario{}, err
		}
	}
	var steps []compat.Step
	if err := json.Unmarshal(root["steps"], &steps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", infraNWRUId, err)
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWRUEPLForStep returns the byte-exact EPL of one deploy step.
func infraNWRUEPLForStep(spec infraNWRUCaseSpec, statement string) (string, bool) {
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

func infraNWRURequireEqual(got, want []string, label string) error {
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

func infraNWRURequireListened(got []string, spec infraNWRUCaseSpec) error {
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

func requireInfraNWRUFields(object map[string]json.RawMessage, names ...string) error {
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

func runInfraNWRUScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWRUId)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWRUCaseSpecs {
		caseTrace, err := runInfraNWRUCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWRUId, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWRUCase(ctx context.Context, scenario compat.Scenario, spec infraNWRUCaseSpec) ([]compat.TraceRecord, error) {
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
	if _, err := esper.RegisterStruct[infraNWRUBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWRUMarket](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	var windowSchema esper.Schema
	var err error
	switch spec.sendKind {
	case infraNWRUUniqueInt:
		windowSchema, err = esper.RegisterStruct[infraNWRUKVInt](env, spec.windowName)
	default:
		windowSchema, err = esper.RegisterStruct[infraNWRUKVLong](env, spec.windowName)
	}
	if err != nil {
		return nil, err
	}
	var retention esper.WindowSpec
	switch spec.ordinal {
	case 32, 33:
		retention = esper.Unique(esper.Field[any, string]("key"))
	case 34:
		retention = esper.FirstUnique(esper.Field[any, string]("key"))
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
			plan, err := infraNWRUBuildPlan(env, spec, step.Statement)
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
			payload, err := infraNWRUDecodePayload(spec, step)
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
			// Multi-row snapshots ride the Java views' HashMap iteration
			// order, which the suite asserts any-order only; the scenario
			// marks those steps "any" and both sides emit canonical order.
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

// infraNWRUBuildPlan maps one scenario deploy statement onto the typed chain.
func infraNWRUBuildPlan(env *esper.Environment, spec infraNWRUCaseSpec, statement string) (esper.Plan, error) {
	switch statement {
	case "create":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		source := esper.From[infraNWRUBean](env, "SupportBean")
		switch spec.sendKind {
		case infraNWRUUniqueInt:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWRUBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWRUBean, int]("intBoxed")),
			).Query(esper.StatementName("insert")))
		default:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWRUBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWRUBean, int64]("longBoxed")),
			).Query(esper.StatementName("insert")))
		}
	case "s0":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int64]("value")),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "consume":
		// The projection unique window's consumer is deployed for fidelity with
		// the Java module fan-out; it carries no listener and no snapshot, and
		// its value column is the window's int (Integer intBoxed) projection.
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int]("value")),
		).Query(esper.StatementName("consume"), esper.WithOldStream()))
	case "delete":
		return env.Build(esper.OnEvent(esper.From[infraNWRUMarket](env, "SupportMarketDataBean")).
			DeleteFromNamedWindow(spec.windowName,
				esper.Equal[string](esper.NamedWindowField[string]("key"),
					esper.Field[infraNWRUMarket, string]("symbol"))).
			Query(esper.StatementName("delete")))
	default:
		return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
	}
}

func infraNWRUDecodePayload(spec infraNWRUCaseSpec, step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWRUFields(fields, spec.sendFields...); err != nil {
			return nil, fmt.Errorf("case %q SupportBean payload: %w", spec.name, err)
		}
		var bean infraNWRUBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWRUFields(fields, "symbol"); err != nil {
			return nil, fmt.Errorf("SupportMarketDataBean payload: %w", err)
		}
		var market infraNWRUMarket
		if err := json.Unmarshal(step.Payload, &market); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return market, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
