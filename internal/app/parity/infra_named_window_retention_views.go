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

// Parity coverage for the second differential slice of InfraNamedWindowViews:
// the retention-view basics — keep-all over a bean type, and the last-event and
// first-event data windows over the key/value map schema.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 0  InfraKeepAllSimple    java-runtime-26c44410c8018a34d696
//   - ord 29 InfraLastEvent        java-runtime-b85cc831b5c23a570cf7
//   - ord 30 InfraLastEventSceneTwo java-runtime-59100403b3c6affd0729
//   - ord 31 InfraFirstEvent       java-runtime-c2f54d9eb104d061950c
//
// The slice pins three retention rules through listener callbacks and ordered
// window iterator snapshots: keep-all retains every inserted bean (ord 0, whose
// only observable is the window statement plus the two-module teardown), a
// last-event replacement delivers one invocation carrying both the new row and
// the replaced row (ords 29/30), and a first-event insert into a non-empty
// window is dropped without any callback anywhere while deletes drain the
// window and re-admit inserts once it is empty (ord 31).
//
// As in the keep-all/on-delete chain, the Java suite attaches an on-delete
// listener but never asserts its payload (only assertListenerNotInvoked on the
// consumer, which reads isInvoked() without reset), so the delete listener is
// excluded from the trace on both sides; deleted rows are observed through the
// window statement's old rows, the consumer old rows and the iterator.
const infraNWRVId = "infra-named-window-retention-views"

const infraNWRVDescription = "InfraNamedWindowViews retention-view slice: the legacy win:keepall() window over the SupportBean bean type, and the MySimpleKeyValueMap lastevent and firstevent windows with insert-into projections, on-delete triggers and irstream consumers, captured from listener callbacks and ordered window iterator snapshots (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWRVJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWRVSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// Byte-exact EPL pins (Java lines 166/169 for ord 0, 2645-2648 for ord 29,
// 2691/2695/2699 for ord 30 and 2748-2751 for ord 31).
const (
	infraNWRVCreateKeepAll = "@Name('create') @public create window MyWindow.win:keepall() as SupportBean"
	infraNWRVInsertKeepAll = "@Name('insert') insert into MyWindow select * from SupportBean"

	infraNWRVCreateLastEvent = "@name('create') create window MyWindowLE#lastevent as MySimpleKeyValueMap"
	infraNWRVInsertLastEvent = "insert into MyWindowLE select theString as key, longBoxed as value from SupportBean"
	infraNWRVSelectLastEvent = "@name('s0') select irstream key, value as value from MyWindowLE"
	infraNWRVDeleteLastEvent = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowLE as s1 where s0.symbol = s1.key"

	infraNWRVCreateLastEventTwo = "@name('create') @public create window MyWindow.std:lastevent() as select theString as key, intBoxed as value from SupportBean"
	infraNWRVInsertLastEventTwo = "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"
	infraNWRVDeleteLastEventTwo = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"

	infraNWRVCreateFirstEvent = "@name('create') create window MyWindowFE#firstevent as MySimpleKeyValueMap"
	infraNWRVInsertFirstEvent = "insert into MyWindowFE select theString as key, longBoxed as value from SupportBean"
	infraNWRVSelectFirstEvent = "@name('s0') select irstream key, value as value from MyWindowFE"
	infraNWRVDeleteFirstEvent = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowFE as s1 where s0.symbol = s1.key"
)

// infraNWRVSendKind selects the SupportBean payload shape and the projected row
// fields of a case.
type infraNWRVSendKind int

const (
	infraNWRVKeepAllSimple infraNWRVSendKind = iota
	infraNWRVLastEventLong
	infraNWRVLastEventInt
	infraNWRVFirstEventLong
)

type infraNWRVCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	windowName  string
	description string
	createEPL   string
	insertEPL   string
	selectEPL   string
	deleteEPL   string
	deploys     []string
	listened    map[string]bool
	snapshots   int
	sendKind    infraNWRVSendKind
	sendFields  []string
	rowFields   []string
}

var infraNWRVCaseSpecs = []infraNWRVCaseSpec{
	{
		name:        "keepall-simple",
		ordinal:     0,
		runtimeID:   "java-runtime-26c44410c8018a34d696",
		execution:   "InfraKeepAllSimple",
		windowName:  "MyWindow",
		description: "legacy win:keepall() window over SupportBean with a separate insert-into module, two window-statement inserts and the two-statement module teardown",
		createEPL:   infraNWRVCreateKeepAll,
		insertEPL:   infraNWRVInsertKeepAll,
		deploys:     []string{"create", "insert"},
		listened:    map[string]bool{"create": true},
		snapshots:   0,
		sendKind:    infraNWRVKeepAllSimple,
		sendFields:  []string{"theString"},
		rowFields:   []string{"theString"},
	},
	{
		name:        "lastevent",
		ordinal:     29,
		runtimeID:   "java-runtime-b85cc831b5c23a570cf7",
		execution:   "InfraLastEvent",
		windowName:  "MyWindowLE",
		description: "lastevent window over the key/value map schema: replacement delivers one invocation carrying the new row and the replaced row, three delete waves each followed by an empty iterator, and a no-match delete that stays silent",
		createEPL:   infraNWRVCreateLastEvent,
		insertEPL:   infraNWRVInsertLastEvent,
		selectEPL:   infraNWRVSelectLastEvent,
		deleteEPL:   infraNWRVDeleteLastEvent,
		deploys:     []string{"create", "insert", "s0", "delete"},
		listened:    map[string]bool{"create": true, "s0": true},
		snapshots:   6,
		sendKind:    infraNWRVLastEventLong,
		sendFields:  []string{"theString", "longBoxed"},
		rowFields:   []string{"key", "value"},
	},
	{
		name:        "lastevent-scene-two",
		ordinal:     30,
		runtimeID:   "java-runtime-59100403b3c6affd0729",
		execution:   "InfraLastEventSceneTwo",
		windowName:  "MyWindow",
		description: "projection lastevent window over theString/intBoxed with three module deployments: replacement IR pair, delete to empty, re-insert after becoming empty",
		createEPL:   infraNWRVCreateLastEventTwo,
		insertEPL:   infraNWRVInsertLastEventTwo,
		deleteEPL:   infraNWRVDeleteLastEventTwo,
		deploys:     []string{"create", "insert", "delete"},
		listened:    map[string]bool{"create": true},
		snapshots:   3,
		sendKind:    infraNWRVLastEventInt,
		sendFields:  []string{"theString", "intBoxed"},
		rowFields:   []string{"key", "value"},
	},
	{
		name:        "firstevent",
		ordinal:     31,
		runtimeID:   "java-runtime-c2f54d9eb104d061950c",
		execution:   "InfraFirstEvent",
		windowName:  "MyWindowFE",
		description: "firstevent window over the key/value map schema: a dropped insert emits nothing anywhere, deletes drain the window to empty, and re-inserts are admitted once empty",
		createEPL:   infraNWRVCreateFirstEvent,
		insertEPL:   infraNWRVInsertFirstEvent,
		selectEPL:   infraNWRVSelectFirstEvent,
		deleteEPL:   infraNWRVDeleteFirstEvent,
		deploys:     []string{"create", "insert", "s0", "delete"},
		listened:    map[string]bool{"create": true, "s0": true},
		snapshots:   6,
		sendKind:    infraNWRVFirstEventLong,
		sendFields:  []string{"theString", "longBoxed"},
		rowFields:   []string{"key", "value"},
	},
}

var (
	infraNWRVJavaSources = []string{
		infraNWRVSource,
	}
	infraNWRVJavaRuntimeIDs = []string{
		"java-runtime-26c44410c8018a34d696",
		"java-runtime-b85cc831b5c23a570cf7",
		"java-runtime-59100403b3c6affd0729",
		"java-runtime-c2f54d9eb104d061950c",
	}
	infraNWRVJavaExecutions = []string{
		"InfraKeepAllSimple",
		"InfraLastEvent",
		"InfraLastEventSceneTwo",
		"InfraFirstEvent",
	}
	infraNWRVCases = []string{
		"keepall-simple",
		"lastevent",
		"lastevent-scene-two",
		"firstevent",
	}
)

// infraNWRVBean mirrors SupportBean; each case validates the exact send fields
// it pins (the keep-all case sets only theString, so the boxed values stay at
// their Go zero value and are not projected into the trace).
type infraNWRVBean struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
	LongBoxed int64  `esper:"longBoxed"`
}

type infraNWRVMarket struct {
	Symbol string `esper:"symbol"`
}

type infraNWRVKVLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type infraNWRVKVInt struct {
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

func infraNWRVCaseSpecFor(name string) (infraNWRVCaseSpec, bool) {
	for _, spec := range infraNWRVCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWRVCaseSpec{}, false
}

func loadInfraNWRVScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWRVId)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWRVId, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRVId, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWRVId, err)
	}
	if err := requireInfraNWRVFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWRVId, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWRVId ||
		metadata.Description != infraNWRVDescription ||
		metadata.JavaCommit != infraNWRVJavaCommit || metadata.JavaSource != infraNWRVSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWRVId)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWRVId)
	}
	if err := infraNWRVRequireEqual(metadata.JavaRuntimes, infraNWRVJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWRVRequireEqual(metadata.JavaNames, infraNWRVJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWRVId, err)
	}
	if len(rawCases) != len(infraNWRVCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWRVId, len(rawCases), len(infraNWRVCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWRVFields(object,
			"case", "ordinal", "runtimeId", "executionName", "description", "createEpl",
			"insertEpl", "s0Epl", "deleteEpl", "deploys", "listened", "iteratorSnapshots"); err != nil {
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
			DeleteEPL         string   `json:"deleteEpl"`
			Deploys           []string `json:"deploys"`
			Listened          []string `json:"listened"`
			IteratorSnapshots int      `json:"iteratorSnapshots"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := infraNWRVCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
		}
		if definition.Description != spec.description {
			return compat.Scenario{}, fmt.Errorf("scenario case %q description does not match the pinned slice description", spec.name)
		}
		if definition.CreateEPL != spec.createEPL || definition.InsertEPL != spec.insertEPL ||
			definition.SelectEPL != spec.selectEPL || definition.DeleteEPL != spec.deleteEPL {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source", spec.name)
		}
		if err := infraNWRVRequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWRVRequireListened(definition.Listened, spec); err != nil {
			return compat.Scenario{}, err
		}
		if definition.IteratorSnapshots != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q iteratorSnapshots = %d, want %d",
				spec.name, definition.IteratorSnapshots, spec.snapshots)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWRVId, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWRVId)
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
			if err := requireInfraNWRVFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWRVFields(object, "op", "case", "statement", "epl"); err != nil {
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
			spec, ok := infraNWRVCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWRVEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWRVFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWRVFields(object, "op", "case", "statement", "mode"); err != nil {
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
			if step.Mode != "ordered" || step.Statement != "create" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot must target the create statement in ordered mode", index)
			}
			snapshotCounts[step.Case]++
		case "undeploy":
			if err := requireInfraNWRVFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWRVFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	for _, spec := range infraNWRVCaseSpecs {
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
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", infraNWRVId, err)
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWRVEPLForStep returns the byte-exact EPL of one deploy step.
func infraNWRVEPLForStep(spec infraNWRVCaseSpec, statement string) (string, bool) {
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

func infraNWRVRequireEqual(got, want []string, label string) error {
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

func infraNWRVRequireListened(got []string, spec infraNWRVCaseSpec) error {
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

func requireInfraNWRVFields(object map[string]json.RawMessage, names ...string) error {
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

func runInfraNWRVScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWRVId)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWRVCaseSpecs {
		caseTrace, err := runInfraNWRVCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWRVId, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWRVCase(ctx context.Context, scenario compat.Scenario, spec infraNWRVCaseSpec) ([]compat.TraceRecord, error) {
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
	if _, err := esper.RegisterStruct[infraNWRVBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWRVMarket](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	var windowSchema esper.Schema
	var err error
	switch spec.sendKind {
	case infraNWRVKeepAllSimple:
		windowSchema, err = esper.RegisterStruct[infraNWRVBean](env, spec.windowName)
	case infraNWRVLastEventInt:
		windowSchema, err = esper.RegisterStruct[infraNWRVKVInt](env, spec.windowName)
	default:
		windowSchema, err = esper.RegisterStruct[infraNWRVKVLong](env, spec.windowName)
	}
	if err != nil {
		return nil, err
	}
	var retention esper.WindowSpec
	switch spec.ordinal {
	case 0:
		retention = esper.KeepAll()
	case 29, 30:
		retention = esper.LastEvent()
	case 31:
		retention = esper.FirstEvent()
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
			plan, err := infraNWRVBuildPlan(env, spec, step.Statement)
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
			payload, err := infraNWRVDecodePayload(spec, step)
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
				New:       projectRecords(compat.NormalizeResults(result.Batch.New), spec.rowFields),
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

// infraNWRVBuildPlan maps one scenario deploy statement onto the typed chain.
func infraNWRVBuildPlan(env *esper.Environment, spec infraNWRVCaseSpec, statement string) (esper.Plan, error) {
	switch statement {
	case "create":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		source := esper.From[infraNWRVBean](env, "SupportBean")
		switch spec.sendKind {
		case infraNWRVKeepAllSimple:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(
				spec.windowName, esper.CopyMatchingFields(),
			).Query(esper.StatementName("insert")))
		case infraNWRVLastEventInt:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWRVBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWRVBean, int]("intBoxed")),
			).Query(esper.StatementName("insert")))
		default:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWRVBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWRVBean, int64]("longBoxed")),
			).Query(esper.StatementName("insert")))
		}
	case "s0":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Select(
			esper.Alias("key", esper.Field[any, string]("key")),
			esper.Alias("value", esper.Field[any, int64]("value")),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "delete":
		return env.Build(esper.OnEvent(esper.From[infraNWRVMarket](env, "SupportMarketDataBean")).
			DeleteFromNamedWindow(spec.windowName,
				esper.Equal[string](esper.NamedWindowField[string]("key"),
					esper.Field[infraNWRVMarket, string]("symbol"))).
			Query(esper.StatementName("delete")))
	default:
		return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
	}
}

func infraNWRVDecodePayload(spec infraNWRVCaseSpec, step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWRVFields(fields, spec.sendFields...); err != nil {
			return nil, fmt.Errorf("case %q SupportBean payload: %w", spec.name, err)
		}
		var bean infraNWRVBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWRVFields(fields, "symbol"); err != nil {
			return nil, fmt.Errorf("SupportMarketDataBean payload: %w", err)
		}
		var market infraNWRVMarket
		if err := json.Unmarshal(step.Payload, &market); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return market, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
