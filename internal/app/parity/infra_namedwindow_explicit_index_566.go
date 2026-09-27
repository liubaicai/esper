package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the named-window explicit-index bundle: two executions
// sharing one semantic — a late `create index` DDL against a populated named
// window and the late consumers that reuse the declared index.
//
// Covered executions (Java ordinals of their suite's executions()):
// - InfraNamedWindowIndex ord 0 java-runtime-9c952fff6ef8c649c2e4
// - InfraNamedWindowLateStartIndex ord 0 java-runtime-3bf753f4ff71d21df968
//
// named-window-index replays the single-module compileDeploy: a
// #unique(theString) MyWindowOne fed by insert-into, then a unique hash
// index I1 on theString; the five SupportBean sends dedupe last-wins so the
// window iterator returns {(E0,5),(E1,4),(E2,3)} any-order. Java's idx
// statement-type assertions (STATEMENTTYPE=CREATE_INDEX,
// CREATEOBJECTNAME=I1) have no typed-Go counterpart and ride the pinned
// "unrepresentable" idx-props record.
//
// late-start-index replays two phases over the @public #keepall AWindow —
// preload through the insert-into (100 E-rows plus (-1,'x')), `create index
// I1 on AWindow(p00)`, then the late unidirectional s0 join and s1 correlated
// subquery; undeploy-all restarts the path and phase B replays under the
// enable_window_subquery_indexshare hint deploying the identical subquery as
// s2. Esper's getIndexDescriptors() counts the declared I1 only (the
// subquery's implicit/indexshared access paths are internal), so index-count
// pins 1 at every probe. The SupportCountAccessEvent getter-call counters
// (101 after preload, 2 after each late deploy) are JVM instrumentation and
// ride pinned "unrepresentable" getter records; Java keeps each asserted
// window deploy-only, so sends and snapshots sit between an assert and the
// next reset.
const infraNWIdx566ID = "infra-namedwindow-explicit-index-566"

const infraNWIdx566Description = "InfraNamedWindowIndex ord 0 and InfraNamedWindowLateStartIndex ord 0: one named-window explicit-index bundle. named-window-index replays the single-module compileDeploy — a SupportBean #unique(theString) window fed by insert-into and a late `create unique index I1 on MyWindowOne(theString)` — sends E0/1,E2/2,E2/3,E1/4,E0/5, pins the idx statement's CREATE_INDEX/CREATEOBJECTNAME properties as an unrepresentable record (no Go statement-type surface) and pins the window iterator any-order {(E0,5),(E1,4),(E2,3)} (unique last-wins dedup). late-start-index replays two phases over a preloaded @public keepall AWindow (100 SupportCountAccessEvent E-rows plus (-1,'x')) with `create index I1 on AWindow(p00)`: phase A deploys the late unidirectional join s0 (p00='x' filtered window, aw.id=s0.id) and the correlated subquery s1; phase B redeploys the same infra under @Hint('enable_window_subquery_indexshare') and deploys the identical subquery as s2. Each SupportBean_S0(-1,'x') triggers one delivery; index-count records pin getIndexDescriptors().length=1 and snapshots pin the 101-row window contents; the SupportCountAccessEvent getter-call counters (101,2,2,2) are JVM instrumentation and ride unrepresentable records (EXCLUDEWHENINSTRUMENTED+PERFORMANCE)."

const infraNWIdx566JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWIdx566Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite"

// Byte-exact EPL pins (InfraNamedWindowIndex.java lines 26-28 for the
// three-statement module; InfraNamedWindowLateStartIndex.java lines 38,
// 46-47, 57-58 and 70-77 for the late-start-index statements).
const (
	infraNWIdx566E1Window = "@name('window') create window MyWindowOne#unique(theString) as SupportBean"
	infraNWIdx566E1Insert = "insert into MyWindowOne select * from SupportBean"
	infraNWIdx566E1Idx    = "@name('idx') create unique index I1 on MyWindowOne(theString)"

	infraNWIdx566E2Create       = "@public create window AWindow#keepall as SupportCountAccessEvent"
	infraNWIdx566E2CreateShared = "@Hint('enable_window_subquery_indexshare') @public create window AWindow#keepall as SupportCountAccessEvent"
	infraNWIdx566E2Insert       = "insert into AWindow select * from SupportCountAccessEvent"
	infraNWIdx566E2Index        = "create index I1 on AWindow(p00)"
	infraNWIdx566E2S0           = "@name('s0') select * from SupportBean_S0 as s0 unidirectional, AWindow(p00='x') as aw where aw.id = s0.id"
	infraNWIdx566E2S1           = "@name('s1') select (select id from AWindow(p00='x') as aw where aw.id = s0.id) from SupportBean_S0 as s0 unidirectional"
	infraNWIdx566E2S2           = "@name('s2') select (select id from AWindow(p00='x') as aw where aw.id = s0.id) from SupportBean_S0 as s0 unidirectional"
)

var (
	infraNWIdx566JavaRuntimeIDs = []string{
		"java-runtime-9c952fff6ef8c649c2e4",
		"java-runtime-3bf753f4ff71d21df968",
	}
	infraNWIdx566JavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowIndex.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowLateStartIndex.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_S0.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportCountAccessEvent.java",
	}
	infraNWIdx566JavaExecutions = []string{
		"InfraNamedWindowIndex",
		"InfraNamedWindowLateStartIndex",
	}
	infraNWIdx566JavaStaticIDs = []string{
		"java-152a3c2771c531a8841c",
		"java-a81367bdfb722afec857",
	}
	infraNWIdx566JavaFlags = []string{"EXCLUDEWHENINSTRUMENTED", "PERFORMANCE"}
)

// Pinned notes the unrepresentable records carry (byte-equal to the oracle's
// constants and the scenario's expectError texts).
const (
	infraNWIdx566NoteIdxProps = "Java asserts the idx statement properties STATEMENTTYPE=CREATE_INDEX and CREATEOBJECTNAME=I1 through EPStatement.getProperty; the typed Go API has no statement-type metadata surface"
	infraNWIdx566NoteReset    = "SupportCountAccessEvent.getAndResetCountGetterCalled() resets the JVM getter-call counter at the assert boundary; no Go instrumentation counterpart exists"
	infraNWIdx566NotePreload  = "Java asserts getAndResetCountGetterCalled()=101 after preloading 100 E-rows plus (-1,'x') through the insert-into; Go has no getter-call instrumentation"
	infraNWIdx566NoteGetterS0 = "Java asserts getAndResetCountGetterCalled()=2 after deploying the s0 unidirectional join (the parens p00='x' filter is planned against the declared I1 index); Go has no getter-call instrumentation"
	infraNWIdx566NoteGetterS1 = "Java asserts getAndResetCountGetterCalled()=2 after deploying the s1 correlated subquery without index sharing; Go has no getter-call instrumentation"
	infraNWIdx566NoteGetterS2 = "Java asserts getAndResetCountGetterCalled()=2 after deploying the s2 correlated subquery with enable_window_subquery_indexshare; Go has no getter-call instrumentation"
)

var infraNWIdx566Unrepresentable = map[string]string{
	"idx-props":      infraNWIdx566NoteIdxProps,
	"getter-reset":   infraNWIdx566NoteReset,
	"getter-preload": infraNWIdx566NotePreload,
	"getter-s0":      infraNWIdx566NoteGetterS0,
	"getter-s1":      infraNWIdx566NoteGetterS1,
	"getter-s2":      infraNWIdx566NoteGetterS2,
}

// infraNWIdx566Bean mirrors the SupportBean properties the executions pin:
// theString (unique key + iterator field) and intPrimitive (iterator field).
type infraNWIdx566Bean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// infraNWIdx566S0 mirrors SupportBean_S0's id/p00 projection the join and
// subquery correlate on; the Java bean's p01-p03 are unasserted.
type infraNWIdx566S0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// infraNWIdx566Access mirrors SupportCountAccessEvent's id/p00 surface; the
// JVM getter-call counter inside the Java bean has no Go counterpart.
type infraNWIdx566Access struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// infraNWIdx566CaseSpec pins one Java execution: case identity, observation
// text, newline-joined EPL and the deploy label sequence.
type infraNWIdx566CaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
	deploys     []string
}

var infraNWIdx566CaseSpecs = []infraNWIdx566CaseSpec{
	{
		name:      "named-window-index",
		ordinal:   0,
		runtimeID: "java-runtime-9c952fff6ef8c649c2e4",
		execution: "InfraNamedWindowIndex",
		observation: "deployed+unrepresentable+snapshot; one three-statement module (unique-retention" +
			" MyWindowOne, wildcard insert, unique index I1 on theString): five SupportBean sends" +
			" dedupe last-wins on theString, the idx statement-type/object-name assertion rides a" +
			" pinned record, and the window iterator returns {(E0,5),(E1,4),(E2,3)} any-order",
		epl:     infraNWIdx566E1Window + ";\n" + infraNWIdx566E1Insert + ";\n" + infraNWIdx566E1Idx + ";\n",
		deploys: []string{"window", "insert", "idx"},
	},
	{
		name:      "late-start-index",
		ordinal:   0,
		runtimeID: "java-runtime-3bf753f4ff71d21df968",
		execution: "InfraNamedWindowLateStartIndex",
		observation: "deployed+listener+index-count+snapshot+unrepresentable; two phases on separate" +
			" paths: preload create+insert+index over @public keepall AWindow (100 E-rows + (-1,'x')," +
			" index count 1, 101-row snapshot), then late s0 unidirectional join delivers one" +
			" {s0:(-1,x),aw:(-1,x)} pair and late s1 subquery delivers its scalar, undeploy-all" +
			" boundary, indexshare-hinted phase B deploys the identical subquery as s2 delivering the" +
			" same scalar; getter-count asserts (101,2,2,2) ride unrepresentable records",
		epl: infraNWIdx566E2Create + "\n" + infraNWIdx566E2Insert + "\n" + infraNWIdx566E2Index + "\n" +
			infraNWIdx566E2S0 + "\n" + infraNWIdx566E2S1 + "\n" + infraNWIdx566E2CreateShared + "\n" +
			infraNWIdx566E2Insert + "\n" + infraNWIdx566E2Index + "\n" + infraNWIdx566E2S2,
		deploys: []string{"create", "insert", "index", "s0", "s1", "create", "insert", "index", "s2"},
	},
}

func infraNWIdx566CaseSpecFor(name string) (infraNWIdx566CaseSpec, bool) {
	for _, spec := range infraNWIdx566CaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWIdx566CaseSpec{}, false
}

// infraNWIdx566StepPin pins one scenario step's shape.
type infraNWIdx566StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	mode      string
	create    string
	of        string
	count     int64
	note      string
}

func infraNWIdx566DeployPin(statement, epl string) infraNWIdx566StepPin {
	return infraNWIdx566StepPin{op: "deploy", statement: statement, epl: epl}
}

func infraNWIdx566DeployedPin(statement string) infraNWIdx566StepPin {
	return infraNWIdx566StepPin{op: "deployed", statement: statement}
}

func infraNWIdx566BeanSendPin(theString string, intPrimitive int64) infraNWIdx566StepPin {
	return infraNWIdx566StepPin{op: "send", eventType: "SupportBean",
		payload: map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)}}
}

func infraNWIdx566AccessSendPin(id int64, p00 string) infraNWIdx566StepPin {
	return infraNWIdx566StepPin{op: "send", eventType: "SupportCountAccessEvent",
		payload: map[string]any{"id": float64(id), "p00": p00}}
}

func infraNWIdx566S0SendPin() infraNWIdx566StepPin {
	return infraNWIdx566StepPin{op: "send", eventType: "SupportBean_S0",
		payload: map[string]any{"id": float64(-1), "p00": "x"}}
}

func infraNWIdx566SnapshotPin(statement string) infraNWIdx566StepPin {
	return infraNWIdx566StepPin{op: "snapshot", statement: statement, mode: "any"}
}

func infraNWIdx566IndexCountPin(statement, create string) infraNWIdx566StepPin {
	return infraNWIdx566StepPin{op: "index-count", statement: statement, create: create,
		of: "indexes", count: 1}
}

func infraNWIdx566UnrepresentablePin(statement, note string) infraNWIdx566StepPin {
	return infraNWIdx566StepPin{op: "unrepresentable", statement: statement, note: note}
}

// infraNWIdx566LatePreloadPins builds the shared phase prelude: window,
// insert and index deploys each with a deployed marker, the getter reset,
// the 101 preload sends, the getter-preload assert and the index-count pin.
func infraNWIdx566LatePreloadPins(createEPL string) []infraNWIdx566StepPin {
	steps := []infraNWIdx566StepPin{
		infraNWIdx566DeployPin("create", createEPL),
		infraNWIdx566DeployedPin("create"),
		infraNWIdx566DeployPin("insert", infraNWIdx566E2Insert),
		infraNWIdx566DeployedPin("insert"),
		infraNWIdx566DeployPin("index", infraNWIdx566E2Index),
		infraNWIdx566DeployedPin("index"),
		infraNWIdx566UnrepresentablePin("getter-reset", infraNWIdx566NoteReset),
	}
	for i := int64(0); i < 100; i++ {
		steps = append(steps, infraNWIdx566AccessSendPin(i, fmt.Sprintf("E%d", i)))
	}
	steps = append(steps,
		infraNWIdx566AccessSendPin(-1, "x"),
		infraNWIdx566UnrepresentablePin("getter-preload", infraNWIdx566NotePreload),
		infraNWIdx566IndexCountPin("AWindow", "create"),
	)
	return steps
}

// infraNWIdx566CaseSteps pins the complete step sequence per case in Java
// source order.
var infraNWIdx566CaseSteps = map[string][]infraNWIdx566StepPin{
	"named-window-index": {
		infraNWIdx566DeployPin("window", infraNWIdx566E1Window),
		infraNWIdx566DeployPin("insert", infraNWIdx566E1Insert),
		infraNWIdx566DeployPin("idx", infraNWIdx566E1Idx),
		infraNWIdx566DeployedPin("window"),
		infraNWIdx566DeployedPin("insert"),
		infraNWIdx566DeployedPin("idx"),
		infraNWIdx566UnrepresentablePin("idx-props", infraNWIdx566NoteIdxProps),
		infraNWIdx566BeanSendPin("E0", 1),
		infraNWIdx566BeanSendPin("E2", 2),
		infraNWIdx566BeanSendPin("E2", 3),
		infraNWIdx566BeanSendPin("E1", 4),
		infraNWIdx566BeanSendPin("E0", 5),
		infraNWIdx566SnapshotPin("window"),
		infraNWIdx566StepPin{op: "undeploy-all"},
	},
	"late-start-index": func() []infraNWIdx566StepPin {
		steps := infraNWIdx566LatePreloadPins(infraNWIdx566E2Create)
		steps = append(steps,
			infraNWIdx566DeployPin("s0", infraNWIdx566E2S0),
			infraNWIdx566DeployedPin("s0"),
			infraNWIdx566UnrepresentablePin("getter-s0", infraNWIdx566NoteGetterS0),
			infraNWIdx566IndexCountPin("AWindow", "create"),
			infraNWIdx566S0SendPin(),
			// The send's listener rendering reads aw.p00 (uncounted in Go)
			// while the Java p00 getter is JVM-counted; reset before s1 so
			// the Java-asserted window stays deploy-only.
			infraNWIdx566UnrepresentablePin("getter-reset", infraNWIdx566NoteReset),
			infraNWIdx566DeployPin("s1", infraNWIdx566E2S1),
			infraNWIdx566DeployedPin("s1"),
			infraNWIdx566UnrepresentablePin("getter-s1", infraNWIdx566NoteGetterS1),
			infraNWIdx566IndexCountPin("AWindow", "create"),
			infraNWIdx566S0SendPin(),
			infraNWIdx566SnapshotPin("create"),
			infraNWIdx566StepPin{op: "undeploy-all"},
		)
		steps = append(steps, infraNWIdx566LatePreloadPins(infraNWIdx566E2CreateShared)...)
		steps = append(steps,
			infraNWIdx566DeployPin("s2", infraNWIdx566E2S2),
			infraNWIdx566DeployedPin("s2"),
			infraNWIdx566UnrepresentablePin("getter-s2", infraNWIdx566NoteGetterS2),
			infraNWIdx566IndexCountPin("AWindow", "create"),
			infraNWIdx566S0SendPin(),
			infraNWIdx566SnapshotPin("create"),
			infraNWIdx566StepPin{op: "undeploy-all"},
		)
		return steps
	}(),
}

func loadInfraNWIdx566Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWIdx566ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWIdx566ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWIdx566ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWIdx566ID, err)
	}
	if err := requireInfraNWIdx566Fields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaSourceFiles",
		"javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version        string   `json:"version"`
		ID             string   `json:"id"`
		Description    string   `json:"description"`
		JavaCommit     string   `json:"javaCommit"`
		JavaSource     string   `json:"javaSource"`
		JavaSourceFile []string `json:"javaSourceFiles"`
		JavaRuntimes   []string `json:"javaRuntimes"`
		JavaNames      []string `json:"javaNames"`
		JavaStaticIDs  []string `json:"javaStaticIds"`
		JavaFlags      []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWIdx566ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWIdx566ID ||
		metadata.Description != infraNWIdx566Description ||
		metadata.JavaCommit != infraNWIdx566JavaCommit ||
		metadata.JavaSource != infraNWIdx566Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWIdx566ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, infraNWIdx566JavaSources},
		{metadata.JavaRuntimes, infraNWIdx566JavaRuntimeIDs},
		{metadata.JavaNames, infraNWIdx566JavaExecutions},
		{metadata.JavaStaticIDs, infraNWIdx566JavaStaticIDs},
		{metadata.JavaFlags, infraNWIdx566JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", infraNWIdx566ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWIdx566CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			infraNWIdx566ID, len(infraNWIdx566CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWIdx566Fields(object, "case", "ordinal", "runtimeId",
			"executionName", "observation", "epl", "deploys"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string   `json:"case"`
			Ordinal       int      `json:"ordinal"`
			RuntimeID     string   `json:"runtimeId"`
			ExecutionName string   `json:"executionName"`
			Observation   string   `json:"observation"`
			EPL           string   `json:"epl"`
			Deploys       []string `json:"deploys"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := infraNWIdx566CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID ||
			definition.ExecutionName != spec.execution ||
			definition.Observation != spec.observation || definition.EPL != spec.epl ||
			!reflect.DeepEqual(definition.Deploys, spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %d metadata is not pinned", index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWIdx566ID)
	}
	if err := validateInfraNWIdx566RawSteps(rawSteps); err != nil {
		return compat.Scenario{}, err
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraNWIdx566RawSteps pins the complete step sequence per case:
// op field whitelists per step kind plus positional comparison against the
// pinned sequence.
func validateInfraNWIdx566RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(infraNWIdx566CaseSpecs))
	positions := make(map[string]int)
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return fmt.Errorf("scenario step %d op must be a string: %w", index, err)
		}
		var step struct {
			Case      string          `json:"case"`
			Statement string          `json:"statement"`
			EPL       string          `json:"epl"`
			EventType string          `json:"eventType"`
			Payload   json.RawMessage `json:"payload"`
			Mode      string          `json:"mode"`
			Create    string          `json:"create"`
			Of        string          `json:"of"`
			Count     *int64          `json:"count"`
			ExpectErr string          `json:"expectError"`
		}
		if operation == "case" {
			if err := requireInfraNWIdx566Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := infraNWIdx566CaseSpecFor(marker.Case); !ok {
				return fmt.Errorf("scenario step %d selects unknown case %q", index, marker.Case)
			}
			currentCase = marker.Case
			caseOrder = append(caseOrder, marker.Case)
			continue
		}
		if currentCase == "" {
			return fmt.Errorf("scenario step %d is outside any case block", index)
		}
		if err := json.Unmarshal(rawStep, &step); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		if step.Case != currentCase {
			return fmt.Errorf("scenario step %d declares case %q inside the %q block",
				index, step.Case, currentCase)
		}
		pins := infraNWIdx566CaseSteps[currentCase]
		position := positions[currentCase]
		if position >= len(pins) {
			return fmt.Errorf("scenario step %d exceeds the pinned %s step sequence",
				index, currentCase)
		}
		pin := pins[position]
		if operation != pin.op {
			return fmt.Errorf("scenario step %d op %q is not the pinned %q for case %q position %d",
				index, operation, pin.op, currentCase, position)
		}
		switch operation {
		case "deploy":
			if err := requireInfraNWIdx566Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "deployed":
			if err := requireInfraNWIdx566Fields(object, "op", "case", "statement"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deployed marker is not pinned for case %q",
					index, currentCase)
			}
		case "send":
			if err := requireInfraNWIdx566Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "snapshot":
			if err := requireInfraNWIdx566Fields(object, "op", "case", "statement", "mode"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.Mode != pin.mode {
				return fmt.Errorf("scenario step %d snapshot is not pinned for case %q",
					index, currentCase)
			}
		case "index-count":
			if err := requireInfraNWIdx566Fields(object, "op", "case", "statement", "create", "of", "count"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Count == nil || step.Statement != pin.statement ||
				step.Create != pin.create || step.Of != pin.of || *step.Count != pin.count {
				return fmt.Errorf("scenario step %d index-count is not pinned for case %q",
					index, currentCase)
			}
		case "unrepresentable":
			if err := requireInfraNWIdx566Fields(object, "op", "case", "statement", "expectError"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.ExpectErr != pin.note {
				return fmt.Errorf("scenario step %d unrepresentable is not pinned for case %q",
					index, currentCase)
			}
		case "undeploy-all":
			if err := requireInfraNWIdx566Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(infraNWIdx566CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(infraNWIdx566CaseSpecs))
	}
	for index, spec := range infraNWIdx566CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(infraNWIdx566CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(infraNWIdx566CaseSteps[spec.name]))
		}
	}
	return nil
}

func requireInfraNWIdx566Fields(object map[string]json.RawMessage, names ...string) error {
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

// infraNWIdx566CaseState carries per-case replay state: environment and
// engine (rebuilt at undeploy-all so the named window's data drops like
// Esper's deployment teardown), per-label deployment tracking and listener
// sequences that persist across phases.
type infraNWIdx566CaseState struct {
	spec           infraNWIdx566CaseSpec
	env            *esper.Environment
	engine         *esper.Engine
	now            time.Time
	statements     map[string]*esper.Statement
	deployedLabels map[string]bool
	indexSeq       map[string]uint64
	sequence       map[string]uint64
	deployedSeq    map[string]uint64
	records        []compat.TraceRecord
}

func infraNWIdx566StartEnvironment(spec infraNWIdx566CaseSpec) (*esper.Environment, error) {
	env := esper.NewEnvironment()
	switch spec.name {
	case "named-window-index":
		if _, err := esper.RegisterStruct[infraNWIdx566Bean](env, "SupportBean"); err != nil {
			return nil, err
		}
	case "late-start-index":
		if _, err := esper.RegisterStruct[infraNWIdx566S0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[infraNWIdx566Access](env, "SupportCountAccessEvent"); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown case %q", spec.name)
	}
	return env, nil
}

// runInfraNWIdx566Scenario replays both executions, one fresh engine per
// case like the Java oracle's per-execution runtime.
func runInfraNWIdx566Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNWIdx566Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWIdx566CaseSpecs {
		records, err := runInfraNWIdx566Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWIdx566ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validateInfraNWIdx566Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNWIdx566ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, infraNWIdx566ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validateInfraNWIdx566RawSteps(rawSteps)
}

func runInfraNWIdx566Case(ctx context.Context, scenario compat.Scenario, spec infraNWIdx566CaseSpec) ([]compat.TraceRecord, error) {
	env, err := infraNWIdx566StartEnvironment(spec)
	if err != nil {
		return nil, err
	}
	now := time.Unix(0, 0).UTC()
	state := &infraNWIdx566CaseState{
		spec:           spec,
		env:            env,
		engine:         esper.NewEngine(env, esper.WithRuntimeURI(spec.runtimeID), esper.WithStartTime(now)),
		now:            now,
		statements:     make(map[string]*esper.Statement),
		deployedLabels: make(map[string]bool),
		indexSeq:       make(map[string]uint64),
		sequence:       make(map[string]uint64),
		deployedSeq:    make(map[string]uint64),
	}
	defer func() { _ = state.engine.Close(context.Background()) }()

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
			if err := state.deploy(ctx, step); err != nil {
				return nil, err
			}
		case "deployed":
			if !state.deployedLabels[step.Statement] {
				return nil, fmt.Errorf("%s: deployed marker for unknown statement %q",
					infraNWIdx566ID, step.Statement)
			}
			state.deployedSeq[step.Statement+":deployed"]++
			state.records = append(state.records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  state.deployedSeq[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(state.now),
			})
		case "send":
			payload, err := infraNWIdx566DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "snapshot":
			if err := state.snapshot(ctx, step); err != nil {
				return nil, err
			}
		case "index-count":
			if err := state.indexCount(step); err != nil {
				return nil, err
			}
		case "unrepresentable":
			state.unrepresentable(step)
		case "undeploy-all":
			// Java undeployAll destroys every deployment including the named
			// window's contents (a fresh path for the next phase), so the Go
			// runner rebuilds environment and engine.
			if err := state.rebuild(ctx); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", infraNWIdx566ID, step.Op)
		}
	}
	return state.records, nil
}

func (s *infraNWIdx566CaseState) rebuild(ctx context.Context) error {
	if err := s.engine.Close(ctx); err != nil {
		return err
	}
	env, err := infraNWIdx566StartEnvironment(s.spec)
	if err != nil {
		return err
	}
	s.env = env
	s.engine = esper.NewEngine(env, esper.WithRuntimeURI(s.spec.runtimeID), esper.WithStartTime(s.now))
	s.statements = make(map[string]*esper.Statement)
	s.deployedLabels = make(map[string]bool)
	return nil
}

// deploy maps one scenario label onto the typed Go call or plan equivalent
// of the Java statement. The create/index DDL labels map to catalog calls on
// the named window (RegisterNamedWindow + materializing query statement for
// "create"/"window", NamedWindow.CreateIndex for "idx"/"index") while the
// consumers build fluent plans; the byte-exact EPL text of the Java
// compileDeploy call is pinned by the loader.
func (s *infraNWIdx566CaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	if s.spec.name == "named-window-index" {
		switch label {
		case "window":
			return s.deployIndexWindow(ctx)
		case "insert":
			return s.deployStatement(ctx, label, esper.OnEvent(esper.From[infraNWIdx566Bean](s.env, "SupportBean")).
				InsertIntoNamedWindow("MyWindowOne", esper.CopyMatchingFields()).
				Query(esper.StatementName("insert")))
		case "idx":
			window, ok := s.engine.NamedWindow("MyWindowOne")
			if !ok {
				return fmt.Errorf("named window MyWindowOne is not materialized")
			}
			// Java declares I1 unique; Esper's #unique(theString) retention
			// replace precedes index validation, so the dedup reinsert of an
			// existing unique key is accepted. Go's insertIntoState applies
			// the same post-retention validation contract (the displaced row
			// is excluded from the unique-index check), so unique=true is
			// safe here.
			if err := window.CreateIndex("I1", []string{"theString"}, esper.IndexHash, true); err != nil {
				return fmt.Errorf("create index I1 on MyWindowOne(theString): %w", err)
			}
			s.deployedLabels[label] = true
			return nil
		}
		return fmt.Errorf("unknown deploy label %q for case %q", label, s.spec.name)
	}
	switch label {
	case "create":
		return s.deployLateCreate(ctx, step.Epl)
	case "insert":
		return s.deployStatement(ctx, label, esper.OnEvent(esper.From[infraNWIdx566Access](s.env, "SupportCountAccessEvent")).
			InsertIntoNamedWindow("AWindow", esper.CopyMatchingFields()).
			Query(esper.StatementName("insert")))
	case "index":
		window, ok := s.engine.NamedWindow("AWindow")
		if !ok {
			return fmt.Errorf("named window AWindow is not materialized")
		}
		if err := window.CreateIndex("I1", []string{"p00"}, esper.IndexHash, false); err != nil {
			return fmt.Errorf("create index I1 on AWindow(p00): %w", err)
		}
		s.deployedLabels[label] = true
		return nil
	case "s0":
		return s.deployS0Join(ctx)
	case "s1", "s2":
		return s.deployLateSubquery(ctx, label)
	}
	return fmt.Errorf("unknown deploy label %q for case %q", label, s.spec.name)
}

// deployIndexWindow registers the #unique(theString) MyWindowOne over the
// SupportBean schema and materializes it through the create-window query
// statement, matching the Java module's `create window ... as SupportBean`.
func (s *infraNWIdx566CaseState) deployIndexWindow(ctx context.Context) error {
	schema, ok := s.env.Schema("SupportBean")
	if !ok {
		return fmt.Errorf("SupportBean schema is not registered")
	}
	if _, err := esper.CreateNamedWindow(s.env, "MyWindowOne", schema,
		esper.NamedWindowRetention(esper.Unique(esper.Field[infraNWIdx566Bean, string]("theString")))); err != nil {
		return err
	}
	if _, ok := s.engine.NamedWindow("MyWindowOne"); !ok {
		return fmt.Errorf("named-window-index window was not materialized")
	}
	return s.deployStatement(ctx, "window",
		esper.FromNamedWindow(s.env, "MyWindowOne").CreateNamedWindowQuery(esper.StatementName("window")))
}

// deployLateCreate registers the @public #keepall AWindow over the
// SupportCountAccessEvent schema. Phase B's hinted EPL carries
// enable_window_subquery_indexshare, mapped to
// NamedWindowSubqueryIndexSharing; the unhinted phase A window omits it.
func (s *infraNWIdx566CaseState) deployLateCreate(ctx context.Context, epl string) error {
	schema, ok := s.env.Schema("SupportCountAccessEvent")
	if !ok {
		return fmt.Errorf("SupportCountAccessEvent schema is not registered")
	}
	options := []esper.NamedWindowOption{esper.NamedWindowRetention(esper.KeepAll())}
	if epl == infraNWIdx566E2CreateShared {
		options = append(options, esper.NamedWindowSubqueryIndexSharing())
	} else if epl != infraNWIdx566E2Create {
		return fmt.Errorf("create EPL %q is not pinned", epl)
	}
	if _, err := esper.CreateNamedWindow(s.env, "AWindow", schema, options...); err != nil {
		return err
	}
	if _, ok := s.engine.NamedWindow("AWindow"); !ok {
		return fmt.Errorf("late-start-index window was not materialized")
	}
	return s.deployStatement(ctx, "create",
		esper.FromNamedWindow(s.env, "AWindow").CreateNamedWindowQuery(esper.StatementName("create")))
}

// deployS0Join mirrors `@name('s0') select * from SupportBean_S0 as s0
// unidirectional, AWindow(p00='x') as aw where aw.id = s0.id`: the
// unidirectional driver probes the p00-filtered window rows on the id key
// resolved through the declared I1 hash index.
func (s *infraNWIdx566CaseState) deployS0Join(ctx context.Context) error {
	s0 := esper.JoinSource(esper.From[infraNWIdx566S0](s.env, "SupportBean_S0")).Unidirectional()
	aw := esper.JoinRecordSource(esper.FromNamedWindow(s.env, "AWindow").
		Filter(esper.Equal[string](esper.Field[any, string]("p00"), esper.Literal("x"))))
	plan, err := s.env.Build(esper.JoinMany(s0, aw).On(
		esper.OnSourcesEqual(0, esper.JoinField[int](0, "id"), 1, esper.JoinField[int](1, "id")),
	).Select(
		esper.SelectSourceEvent(0, "s0"),
		esper.SelectSourceEvent(1, "aw"),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return fmt.Errorf("build s0 join: %w", err)
	}
	return s.deployPlan(ctx, "s0", plan, true)
}

// deployLateSubquery mirrors the correlated `(select id from AWindow(p00='x')
// as aw where aw.id = s0.id) from SupportBean_S0 as s0 unidirectional` that
// Java deploys as s1 (phase A) and s2 (indexshare phase B): the scalar
// subquery projects aw.id over the p00-filtered window correlated on the id
// key against the declared I1 index.
func (s *infraNWIdx566CaseState) deployLateSubquery(ctx context.Context, label string) error {
	inner := esper.FromNamedWindow(s.env, "AWindow").
		Filter(esper.Equal[string](esper.Field[any, string]("p00"), esper.Literal("x")))
	plan, err := s.env.Build(esper.Select(
		esper.From[infraNWIdx566S0](s.env, "SupportBean_S0"),
		esper.Alias("id", esper.SubqueryValue[int](inner,
			esper.Field[any, int]("id"),
			esper.Equal[int](esper.Field[any, int]("id"), esper.OuterField[int]("id")))),
	).Query(esper.StatementName(label)))
	if err != nil {
		return fmt.Errorf("build %s subquery: %w", label, err)
	}
	return s.deployPlan(ctx, label, plan, true)
}

// deployStatement builds and deploys one query, registering its statement
// under the scenario label.
func (s *infraNWIdx566CaseState) deployStatement(ctx context.Context, label string, query esper.Query) error {
	plan, err := s.env.Build(query)
	if err != nil {
		return fmt.Errorf("build %q: %w", label, err)
	}
	return s.deployPlan(ctx, label, plan, false)
}

func (s *infraNWIdx566CaseState) deployPlan(ctx context.Context, label string, plan esper.Plan, listen bool) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("deploy %q: %w", label, err)
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return fmt.Errorf("deploy %q produced %d statements", label, len(statements))
	}
	statement := statements[0]
	s.statements[label] = statement
	s.deployedLabels[label] = true
	if listen {
		if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			s.recordListener(label, batch)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// recordListener appends the normalized new/old rows of one listener
// delivery under the label; s0 rows render the s0/aw join fragments as the
// pinned (id,p00) nested rows while s1/s2 project the scalar `id` column.
func (s *infraNWIdx566CaseState) recordListener(label string, batch esper.ResultBatch) {
	var newRows, oldRows []compat.ResultRecord
	if label == "s0" {
		newRows = infraNWIdx566JoinRows(batch.New)
		oldRows = infraNWIdx566JoinRows(batch.Old)
	} else {
		newRows = projectRecords(compat.NormalizeResults(batch.New), []string{"id"})
		oldRows = projectRecords(compat.NormalizeResults(batch.Old), []string{"id"})
	}
	if len(newRows) == 0 && len(oldRows) == 0 {
		return
	}
	s.sequence[label]++
	record := compat.TraceRecord{
		Case:      s.spec.name,
		Operation: "listener",
		Statement: label,
		Sequence:  s.sequence[label],
		Time:      compat.FormatTraceTime(s.now),
		New:       newRows,
	}
	if len(oldRows) > 0 {
		record.Old = oldRows
	}
	s.records = append(s.records, record)
}

// infraNWIdx566JoinRows renders join results as {s0:{id,p00},aw:{id,p00}}
// rows, matching the oracle's fragment projection over select *.
func infraNWIdx566JoinRows(results []esper.Result) []compat.ResultRecord {
	if len(results) == 0 {
		return nil
	}
	rows := make([]compat.ResultRecord, 0, len(results))
	for _, result := range results {
		fields := make(map[string]any, 2)
		for _, alias := range []string{"s0", "aw"} {
			fields[alias] = infraNWIdx566FragmentRow(result, alias)
		}
		rows = append(rows, compat.ResultRecord{Kind: "row", Fields: fields})
	}
	return rows
}

// infraNWIdx566FragmentRow renders one join-fragment event column as the
// pinned (id,p00) row object the oracle emits for select * over a
// two-stream join.
func infraNWIdx566FragmentRow(result esper.Result, name string) map[string]any {
	value := result.Get(name)
	fields := make(map[string]any, 2)
	if event, ok := value.Any().(esper.Event); ok {
		fields["id"] = event.Get("id").Any()
		fields["p00"] = event.Get("p00").Any()
	} else {
		fields["id"] = value.Any()
		fields["p00"] = map[string]any{"state": "missing"}
	}
	return map[string]any{"kind": "row", "fields": fields}
}

// snapshot emits one iterator snapshot record for the statement, projecting
// the pinned fields and sorting canonically (the Java assertion is
// any-order in both cases).
func (s *infraNWIdx566CaseState) snapshot(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot statement %q was not deployed",
			infraNWIdx566ID, step.Statement)
	}
	result, err := statement.Snapshot(ctx)
	if err != nil {
		return err
	}
	fields := []string{"theString", "intPrimitive"}
	if s.spec.name == "late-start-index" {
		fields = []string{"id", "p00"}
	}
	rows := projectRecords(compat.NormalizeResults(result.Batch.New), fields)
	if step.Mode == "any" {
		sortRowsCanonical(rows)
	}
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.spec.name,
		Operation: "snapshot",
		Statement: step.Statement,
		Sequence:  0,
		Time:      compat.FormatTraceTime(s.now),
		New:       rows,
	})
	return nil
}

// indexCount mirrors the Java assertIndexCount probe: the named window's
// IndexCount (declared indexes plus consumer implicit indexes on the Go
// surface) is asserted against the pinned count, then the record carries the
// observed value exactly like the oracle.
func (s *infraNWIdx566CaseState) indexCount(step compat.Step) error {
	if step.Count == nil {
		return fmt.Errorf("%s: index-count %q has no count", infraNWIdx566ID, step.Statement)
	}
	window, ok := s.engine.NamedWindow(step.Statement)
	if !ok {
		return fmt.Errorf("%s: index-count targets unknown named window %q",
			infraNWIdx566ID, step.Statement)
	}
	count := int64(window.IndexCount())
	if count != *step.Count {
		return fmt.Errorf("%s: index-count mismatch for %s (indexes): expected %d, got %d",
			infraNWIdx566ID, step.Statement, *step.Count, count)
	}
	s.indexSeq[step.Statement+":index-count"]++
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.spec.name,
		Operation: "index-count",
		Statement: step.Statement,
		Sequence:  s.indexSeq[step.Statement+":index-count"],
		Time:      compat.FormatTraceTime(s.now),
		Count:     &count,
	})
	return nil
}

// unrepresentable emits the pinned record for a Java assertion with no Go
// surface: the idx statement-type/object-name properties and the
// SupportCountAccessEvent getter-call counters. The oracle verifies the Java
// side in-process before emitting the same record; here the record documents
// the Java-asserted value verbatim.
func (s *infraNWIdx566CaseState) unrepresentable(step compat.Step) {
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.spec.name,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Sequence:  0,
		Value:     step.ExpectError,
	})
}

// infraNWIdx566DecodePayload converts a scenario send payload into the typed
// host object: SupportBean struct, SupportBean_S0 struct or
// SupportCountAccessEvent struct.
func infraNWIdx566DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWIdx566Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean infraNWIdx566Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		if err := requireInfraNWIdx566Fields(fields, "id", "p00"); err != nil {
			return nil, err
		}
		var bean infraNWIdx566S0
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return bean, nil
	case "SupportCountAccessEvent":
		if err := requireInfraNWIdx566Fields(fields, "id", "p00"); err != nil {
			return nil, err
		}
		var event infraNWIdx566Access
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportCountAccessEvent: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWIdx566ID, step.EventType)
	}
}
