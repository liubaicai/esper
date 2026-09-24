package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for InfraNamedWindowOnSelect (3 executions): the named-window
// on-trigger select family — an on-S0 delete observed through the keepall
// window statement's new/old rows, an on-A insert-into that routes the ordered
// window rows into a consumer stream, and an on-pattern select that emits the
// retained rows when the pattern fires.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 0 InfraNamedWindowOnSelectSimple   java-runtime-8a348801e82d24a6c755
//   - ord 1 InfraNamedWindowOnSelectSceneTwo java-runtime-1bbd8705a7bbea1db0cd
//   - ord 2 InfraNamedWindowOnSelectWPattern java-runtime-caeff76ee56490d832ea
//
// The simple case is an on-delete in disguise: the window statement's listener
// sees the E1 insert as new data and the S0(1)-triggered delete as old data.
// Scene-two pins the on-insert select: each SupportBean_A trigger routes the
// keepall window's rows (ordered by theString asc) into MyStream, where the
// select-* consumer observes them as separate events alongside the I% direct
// inserts; the second module's on-B delete-all empties the window so the last
// trigger fires nothing. WPattern pins the on-pattern select: the A event arms
// every e = SupportBean(theString = 'A'), the B event completes the followed-by
// through intPrimitive = e.intPrimitive, and s0 emits one join-shaped row
// (stream_0 the retained Z bean, stream_1 the pattern match).
//
// The Java suite deploys simple and wpattern as three single-statement modules
// and scene-two as one five-statement module plus the delete module; the Go
// runner deploys one plan per deploy step in the same environment — module
// boundaries are a Java packaging detail with no observable effect. Java
// auto-creates the MyStream insert-into type; Go requires it registered
// env-level with the projected SupportBean shape before the routing statements
// build.
const infraNWOSID = "infra-named-window-on-select"

const infraNWOSDescription = "InfraNamedWindowOnSelect named-window on-trigger select slice (ords 0-2): an on-S0 delete observed through the keepall window statement's new/old rows (simple, ord 0), an on-A insert-into that routes the ordered window rows into MyStream for a select-* consumer plus an I% direct insert and an on-B delete-all (scene-two, ord 1), and an on-pattern select that emits a join-shaped row (stream_0 the retained Z bean, stream_1 the pattern match) when the every-A -> B pattern fires (wpattern, ord 2). Deployed markers pin the module fan-out; listener records carry the full SupportBean row and ordered iterator snapshots pin the window contents (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnSelect.java)."

const infraNWOSJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWOSSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnSelect.java"

// Byte-exact EPL pins (InfraNamedWindowOnSelect.java lines 50/53/56 for
// ordinal 0, 81-85 plus 134 for ordinal 1, and 149/150/153 for ordinal 2).
const (
	infraNWOSSimpleCreate = "@Name('create') @public create window MyWindow.win:keepall() as SupportBean"
	infraNWOSSimpleInsert = "@Name('insert') insert into MyWindow select * from SupportBean"
	infraNWOSSimpleDelete = "@Name('delete') on SupportBean_S0 delete from MyWindow where intPrimitive = id"

	infraNWOSSceneCreate   = "@name('create') @public create window MyWindow#keepall as select * from SupportBean"
	infraNWOSSceneInsert   = "insert into MyWindow select * from SupportBean(theString like 'E%')"
	infraNWOSSceneSelect   = "@name('select') on SupportBean_A insert into MyStream select mywin.* from MyWindow as mywin order by theString asc"
	infraNWOSSceneConsumer = "@name('consumer') select * from MyStream"
	infraNWOSSceneInsertI  = "insert into MyStream select * from SupportBean(theString like 'I%')"
	infraNWOSSceneDelete   = "@name('delete') on SupportBean_B delete from MyWindow"

	infraNWOSPatternCreate = "@public create window MyWindow.win:keepall() as SupportBean"
	infraNWOSPatternInsert = "insert into MyWindow select * from SupportBean(theString = 'Z')"
	infraNWOSPatternS0     = "@Name('s0') on pattern[every e = SupportBean(theString = 'A') -> SupportBean(intPrimitive = e.intPrimitive)] select * from MyWindow"
)

// infraNWOSCaseSpec pins one Java execution: identity, deployment order whose
// fan-out the deployed markers pin, the listened statements and the ordered
// iterator snapshot count.
type infraNWOSCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	deploys     []string
	listened    map[string]bool
	snapshots   int
	observation string
	epl         string
}

var infraNWOSCaseSpecs = []infraNWOSCaseSpec{
	{
		name:      "simple",
		ordinal:   0,
		runtimeID: "java-runtime-8a348801e82d24a6c755",
		execution: "InfraNamedWindowOnSelectSimple",
		deploys:   []string{"create", "insert", "delete"},
		listened:  map[string]bool{"create": true},
		observation: "listener+deployed; three separately deployed statements (public keepall window," +
			" wildcard insert, on-S0 delete where intPrimitive = id): the E1 insert arrives as the" +
			" window statement's new row and the S0(1) trigger removes it as the window statement's" +
			" old row",
		epl: "@Name('create') @public create window MyWindow.win:keepall() as SupportBean;\n" +
			"@Name('insert') insert into MyWindow select * from SupportBean;\n" +
			"@Name('delete') on SupportBean_S0 delete from MyWindow where intPrimitive = id;\n",
	},
	{
		name:      "scene-two",
		ordinal:   1,
		runtimeID: "java-runtime-1bbd8705a7bbea1db0cd",
		execution: "InfraNamedWindowOnSelectSceneTwo",
		deploys:   []string{"create", "insert", "select", "consumer", "insert-i", "delete"},
		listened:  map[string]bool{"select": true, "consumer": true},
		snapshots: 3,
		observation: "listener+snapshot+deployed; one five-statement module (keepall window, E% insert," +
			" on-A insert-into MyStream selecting mywin.* ordered by theString asc, select-* consumer," +
			" I% insert into MyStream) plus a second module's on-B delete-all: A1 fires one select row" +
			" and one consumer row for E1, I2 reaches the consumer only, A2 fires the two-row ordered" +
			" batch [E1,E3] on select and two separate consumer events, and B1 empties the window so" +
			" A3 fires nothing",
		epl: "@name('create') @public create window MyWindow#keepall as select * from SupportBean;\n" +
			"insert into MyWindow select * from SupportBean(theString like 'E%');\n" +
			"@name('select') on SupportBean_A insert into MyStream select mywin.* from MyWindow as mywin order by theString asc;\n" +
			"@name('consumer') select * from MyStream;\n" +
			"insert into MyStream select * from SupportBean(theString like 'I%');\n" +
			"@name('delete') on SupportBean_B delete from MyWindow;\n",
	},
	{
		name:      "wpattern",
		ordinal:   2,
		runtimeID: "java-runtime-caeff76ee56490d832ea",
		execution: "InfraNamedWindowOnSelectWPattern",
		deploys:   []string{"create", "insert", "s0"},
		listened:  map[string]bool{"s0": true},
		observation: "listener+deployed; a keepall window pre-populated by the Z-filtered insert feeds" +
			" an on-pattern select: the A(1) event arms every e = SupportBean(theString = 'A'), the" +
			" B(1) event matches intPrimitive = e.intPrimitive, and s0 emits one join-shaped row" +
			" whose stream_0 column is the retained Z bean and stream_1 the pattern match",
		epl: "@public create window MyWindow.win:keepall() as SupportBean;\n" +
			"insert into MyWindow select * from SupportBean(theString = 'Z');\n" +
			"@Name('s0') on pattern[every e = SupportBean(theString = 'A') -> SupportBean(intPrimitive = e.intPrimitive)] select * from MyWindow;\n",
	},
}

var (
	infraNWOSJavaSources = []string{
		infraNWOSSource,
	}
	infraNWOSJavaRuntimeIDs = []string{
		"java-runtime-8a348801e82d24a6c755",
		"java-runtime-1bbd8705a7bbea1db0cd",
		"java-runtime-caeff76ee56490d832ea",
	}
	infraNWOSJavaExecutions = []string{
		"InfraNamedWindowOnSelectSimple",
		"InfraNamedWindowOnSelectSceneTwo",
		"InfraNamedWindowOnSelectWPattern",
	}
	infraNWOSJavaStaticIDs = []string{
		"java-763e7086c0fdad5f4c49",
		"java-0f1cbcf0f43473325b2b",
		"java-4201486cef18d7d66843",
	}
	infraNWOSCases = []string{
		"simple",
		"scene-two",
		"wpattern",
	}
)

// infraNWOSBean mirrors the full SupportBean schema: the Java assertions read
// only theString/intPrimitive but the trace records the complete row, so every
// declared property is carried with Java defaults. The same struct registers
// MyStream for scene-two (the insert-into stream carries the SupportBean
// shape, matching Java's auto-created type).
type infraNWOSBean struct {
	TheString       string   `esper:"theString"`
	IntPrimitive    int64    `esper:"intPrimitive"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntBoxed        *int64   `esper:"intBoxed"`
	CharPrimitive   string   `esper:"charPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *string  `esper:"bigDecimal"`
	BigInteger      *string  `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

// infraNWOSS0 mirrors the SupportBean_S0 trigger surface the simple case's
// on-delete reads (id only).
type infraNWOSS0 struct {
	ID int64 `esper:"id"`
}

// infraNWOSA and infraNWOSB mirror the regression-lib SupportBean_A/_B trigger
// beans (id only).
type infraNWOSA struct {
	ID string `esper:"id"`
}

type infraNWOSB struct {
	ID string `esper:"id"`
}

func infraNWOSCaseSpecFor(name string) (infraNWOSCaseSpec, bool) {
	for _, spec := range infraNWOSCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWOSCaseSpec{}, false
}

func loadInfraNWOSScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWOSID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWOSID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWOSID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWOSID, err)
	}
	if err := requireInfraNWOSFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
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
		JavaStaticID []string `json:"javaStaticIds"`
		JavaFlags    []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWOSID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWOSID ||
		metadata.Description != infraNWOSDescription ||
		metadata.JavaCommit != infraNWOSJavaCommit || metadata.JavaSource != infraNWOSSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWOSID)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWOSID)
	}
	if err := infraNWOSRequireEqual(metadata.JavaRuntimes, infraNWOSJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWOSRequireEqual(metadata.JavaNames, infraNWOSJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWOSRequireEqual(metadata.JavaStaticID, infraNWOSJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWOSID, err)
	}
	if len(rawCases) != len(infraNWOSCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWOSID, len(rawCases), len(infraNWOSCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWOSFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string `json:"case"`
			Ordinal       int    `json:"ordinal"`
			RuntimeID     string `json:"runtimeId"`
			ExecutionName string `json:"executionName"`
			Observation   string `json:"observation"`
			EPL           string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := infraNWOSCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
		}
		if definition.Observation != spec.observation {
			return compat.Scenario{}, fmt.Errorf("scenario case %q observation does not match the pinned slice description", spec.name)
		}
		if definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source", spec.name)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWOSID, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWOSID)
	}
	steps := make([]compat.Step, len(rawSteps))
	operations := make([]string, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		operations[index] = operation
		switch operation {
		case "case":
			if err := requireInfraNWOSFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWOSFields(object, "op", "case", "statement", "epl"); err != nil {
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
			spec, ok := infraNWOSCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWOSEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
		case "deployed":
			if err := requireInfraNWOSFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWOSFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := infraNWOSDecodePayload(compat.Step{EventType: step.EventType, Payload: step.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWOSFields(object, "op", "case", "statement", "mode"); err != nil {
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
			if step.Statement != "create" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshots unexpected statement %q", index, step.Statement)
			}
		case "undeploy-all":
			if err := requireInfraNWOSFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	if err := validateInfraNWOSRawSteps(rawSteps, operations); err != nil {
		return compat.Scenario{}, err
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWOSEPLForStep returns the byte-exact EPL of one deploy step.
func infraNWOSEPLForStep(spec infraNWOSCaseSpec, statement string) (string, bool) {
	switch spec.name {
	case "simple":
		switch statement {
		case "create":
			return infraNWOSSimpleCreate, true
		case "insert":
			return infraNWOSSimpleInsert, true
		case "delete":
			return infraNWOSSimpleDelete, true
		}
	case "scene-two":
		switch statement {
		case "create":
			return infraNWOSSceneCreate, true
		case "insert":
			return infraNWOSSceneInsert, true
		case "select":
			return infraNWOSSceneSelect, true
		case "consumer":
			return infraNWOSSceneConsumer, true
		case "insert-i":
			return infraNWOSSceneInsertI, true
		case "delete":
			return infraNWOSSceneDelete, true
		}
	case "wpattern":
		switch statement {
		case "create":
			return infraNWOSPatternCreate, true
		case "insert":
			return infraNWOSPatternInsert, true
		case "s0":
			return infraNWOSPatternS0, true
		}
	}
	return "", false
}

// infraNWOSCaseSteps pins the complete step sequence per case: case marker,
// deploy/deployed pairs in module order (scene-two's five-statement module
// carries its markers after the five deploys), sends, ordered snapshots and
// undeploy-all.
var infraNWOSCaseSteps = map[string][]string{
	"simple": {
		"deploy:create:" + infraNWOSSimpleCreate,
		"deployed:create",
		"deploy:insert:" + infraNWOSSimpleInsert,
		"deployed:insert",
		"deploy:delete:" + infraNWOSSimpleDelete,
		"deployed:delete",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean_S0:{\"id\":1}",
		"undeploy-all",
	},
	"scene-two": {
		"deploy:create:" + infraNWOSSceneCreate,
		"deploy:insert:" + infraNWOSSceneInsert,
		"deploy:select:" + infraNWOSSceneSelect,
		"deploy:consumer:" + infraNWOSSceneConsumer,
		"deploy:insert-i:" + infraNWOSSceneInsertI,
		"deployed:create",
		"deployed:insert",
		"deployed:select",
		"deployed:consumer",
		"deployed:insert-i",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"snapshot:create",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"I2\"}",
		"snapshot:create",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"E3\"}",
		"snapshot:create",
		"send:SupportBean_A:{\"id\":\"A2\"}",
		"deploy:delete:" + infraNWOSSceneDelete,
		"deployed:delete",
		"send:SupportBean_B:{\"id\":\"B1\"}",
		"send:SupportBean_A:{\"id\":\"A3\"}",
		"undeploy-all",
	},
	"wpattern": {
		"deploy:create:" + infraNWOSPatternCreate,
		"deployed:create",
		"deploy:insert:" + infraNWOSPatternInsert,
		"deployed:insert",
		"send:SupportBean:{\"intPrimitive\":0,\"theString\":\"Z\"}",
		"deploy:s0:" + infraNWOSPatternS0,
		"deployed:s0",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"A\"}",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"B\"}",
		"undeploy-all",
	},
}

func validateInfraNWOSRawSteps(rawSteps []json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWOSCases {
		want, ok := infraNWOSCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWOSID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWOSID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s case %q does not start with a case marker", infraNWOSID, caseName)
		}
		offset++
		for _, pinned := range want {
			var step struct {
				Op        string          `json:"op"`
				Case      string          `json:"case"`
				Statement string          `json:"statement"`
				EPL       string          `json:"epl"`
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
				Mode      string          `json:"mode"`
			}
			if err := json.Unmarshal(rawSteps[offset], &step); err != nil {
				return fmt.Errorf("%s step %d: %w", infraNWOSID, offset, err)
			}
			if step.Case != caseName {
				return fmt.Errorf("%s step %d is not pinned for case %q", infraNWOSID, offset, caseName)
			}
			var key string
			switch step.Op {
			case "deploy":
				key = "deploy:" + step.Statement + ":" + step.EPL
			case "deployed":
				key = "deployed:" + step.Statement
			case "send":
				var payload map[string]any
				if err := json.Unmarshal(step.Payload, &payload); err != nil {
					return fmt.Errorf("%s step %d send payload: %w", infraNWOSID, offset, err)
				}
				canonical, err := json.Marshal(payload)
				if err != nil {
					return fmt.Errorf("%s step %d send payload: %w", infraNWOSID, offset, err)
				}
				key = "send:" + step.EventType + ":" + string(canonical)
			case "snapshot":
				key = "snapshot:" + step.Statement
			case "undeploy-all":
				key = "undeploy-all"
			default:
				return fmt.Errorf("%s step %d has unsupported op %q", infraNWOSID, offset, step.Op)
			}
			if key != pinned {
				return fmt.Errorf("%s step %d is not pinned: got %q want %q", infraNWOSID, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario steps contain an unexpected suffix", infraNWOSID)
	}
	return nil
}

func infraNWOSRequireEqual(got, want []string, label string) error {
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

func requireInfraNWOSFields(object map[string]json.RawMessage, names ...string) error {
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

// runInfraNWOSScenario replays the three pinned executions, one fresh
// environment, window and engine per case.
func runInfraNWOSScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWOSID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWOSCaseSpecs {
		caseTrace, err := runInfraNWOSCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWOSID, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWOSCase(ctx context.Context, scenario compat.Scenario, spec infraNWOSCaseSpec) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := map[string]uint64{}
	var records []compat.TraceRecord
	recordListener := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		newRows := compat.NormalizeResults(batch.New)
		oldRows := compat.NormalizeResults(batch.Old)
		if spec.name == "wpattern" && statement == "s0" {
			// select * over the pattern+window join yields a join-shaped row
			// whose columns are the stream underlyings; the Java oracle prints
			// them with String.valueOf (bean toString / match-map toString),
			// so the Go record renders the same pinned strings from the actual
			// result values.
			newRows = infraNWOSPatternRows(batch.New)
			oldRows = infraNWOSPatternRows(batch.Old)
		}
		records = append(records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       newRows,
			Old:       oldRows,
		})
	}

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWOSBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	switch spec.name {
	case "simple":
		if _, err := esper.RegisterStruct[infraNWOSS0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
	case "scene-two":
		if _, err := esper.RegisterStruct[infraNWOSA](env, "SupportBean_A"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[infraNWOSB](env, "SupportBean_B"); err != nil {
			return nil, err
		}
		// Java's insert-into creates the MyStream type implicitly; Go requires
		// the route target registered with the projected field set (the full
		// SupportBean shape of mywin.* / select *).
		if _, err := esper.RegisterStruct[infraNWOSBean](env, "MyStream"); err != nil {
			return nil, err
		}
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
			plan, err := infraNWOSBuildPlan(env, spec, step.Statement)
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
		case "deployed":
			if _, ok := statements[step.Statement]; !ok {
				return nil, fmt.Errorf("deployed marker for unknown statement %q", step.Statement)
			}
			sequence[step.Statement+":deployed"]++
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequence[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(engine.Now()),
			})
		case "send":
			payload, err := infraNWOSDecodePayload(step)
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
			// The Java suite tears the module(s) down at this point; teardown
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

// infraNWOSBuildPlan maps one scenario deploy statement onto the typed chain.
// The create statement registers the named window env-level (the Java window
// is @public so later modules resolve it) and deploys a direct-child select to
// keep the statement slot.
func infraNWOSBuildPlan(env *esper.Environment, spec infraNWOSCaseSpec, statement string) (esper.Plan, error) {
	switch spec.name {
	case "simple":
		switch statement {
		case "create":
			schema, ok := env.Schema("SupportBean")
			if !ok {
				return esper.Plan{}, fmt.Errorf("SupportBean schema is missing")
			}
			if _, err := esper.CreateNamedWindow(env, "MyWindow", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
		case "insert":
			return env.Build(esper.OnEvent(esper.From[infraNWOSBean](env, "SupportBean")).
				InsertIntoNamedWindow("MyWindow", esper.CopyMatchingFields()).
				Query(esper.StatementName("insert")))
		case "delete":
			return env.Build(esper.OnEvent(esper.From[infraNWOSS0](env, "SupportBean_S0")).
				DeleteFromNamedWindow("MyWindow",
					esper.Equal[int64](esper.NamedWindowField[int64]("intPrimitive"),
						esper.Field[infraNWOSS0, int64]("id"))).
				Query(esper.StatementName("delete")))
		}
	case "scene-two":
		switch statement {
		case "create":
			schema, ok := env.Schema("SupportBean")
			if !ok {
				return esper.Plan{}, fmt.Errorf("SupportBean schema is missing")
			}
			if _, err := esper.CreateNamedWindow(env, "MyWindow", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
		case "insert":
			return env.Build(esper.OnEvent(esper.From[infraNWOSBean](env, "SupportBean").
				Filter(esper.Like(esper.Field[infraNWOSBean, string]("theString"), esper.Literal("E%")))).
				InsertIntoNamedWindow("MyWindow", esper.CopyMatchingFields()).
				Query(esper.StatementName("insert")))
		case "select":
			// select mywin.* projects the original window events (no
			// selections), preserving the SupportBean underlying for the
			// routed MyStream events.
			return env.Build(esper.OnEvent(esper.From[infraNWOSA](env, "SupportBean_A")).
				SelectFromNamedWindow("MyWindow", nil).
				Query(esper.RouteTo("MyStream"),
					esper.OrderBy(esper.Ascending(esper.NamedWindowField[string]("theString"))),
					esper.StatementName("select")))
		case "consumer":
			return env.Build(esper.FromAny(env, "MyStream").Query(esper.StatementName("consumer")))
		case "insert-i":
			return env.Build(esper.From[infraNWOSBean](env, "SupportBean").
				Filter(esper.Like(esper.Field[infraNWOSBean, string]("theString"), esper.Literal("I%"))).
				InsertInto("MyStream", esper.StatementName("insert-i")))
		case "delete":
			return env.Build(esper.OnEvent(esper.From[infraNWOSB](env, "SupportBean_B")).
				DeleteAllFromNamedWindow("MyWindow").
				Query(esper.StatementName("delete")))
		}
	case "wpattern":
		switch statement {
		case "create":
			schema, ok := env.Schema("SupportBean")
			if !ok {
				return esper.Plan{}, fmt.Errorf("SupportBean schema is missing")
			}
			if _, err := esper.CreateNamedWindow(env, "MyWindow", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
		case "insert":
			return env.Build(esper.OnEvent(esper.From[infraNWOSBean](env, "SupportBean").
				Filter(esper.Equal[string](esper.Field[infraNWOSBean, string]("theString"), esper.Literal("Z")))).
				InsertIntoNamedWindow("MyWindow", esper.CopyMatchingFields()).
				Query(esper.StatementName("insert")))
		case "s0":
			// every e = SupportBean(theString = 'A') -> SupportBean(intPrimitive
			// = e.intPrimitive): the second atom is untagged in the EPL, so the
			// Go chain leaves the FollowedBy tag empty; Field reads the
			// candidate event and TagField reads the captured e.
			bean := esper.From[infraNWOSBean](env, "SupportBean")
			pattern := esper.PatternFrom(bean, "e",
				esper.Equal[string](esper.Field[infraNWOSBean, string]("theString"), esper.Literal("A"))).
				Every().
				FollowedBy("", esper.Equal[int64](
					esper.Field[infraNWOSBean, int64]("intPrimitive"),
					esper.TagField[int64]("e", "intPrimitive")))
			return env.Build(esper.OnPattern(pattern).
				SelectFromNamedWindow("MyWindow", nil).
				Query(esper.StatementName("s0")))
		}
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
}

func infraNWOSDecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWOSFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, fmt.Errorf("SupportBean payload: %w", err)
		}
		var bean infraNWOSBean
		bean.CharPrimitive = "\u0000"
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		if err := requireInfraNWOSFields(fields, "id"); err != nil {
			return nil, fmt.Errorf("SupportBean_S0 payload: %w", err)
		}
		var event infraNWOSS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return event, nil
	case "SupportBean_A":
		if err := requireInfraNWOSFields(fields, "id"); err != nil {
			return nil, fmt.Errorf("SupportBean_A payload: %w", err)
		}
		var event infraNWOSA
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return event, nil
	case "SupportBean_B":
		if err := requireInfraNWOSFields(fields, "id"); err != nil {
			return nil, fmt.Errorf("SupportBean_B payload: %w", err)
		}
		var event infraNWOSB
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_B: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

// infraNWOSPatternRows renders the wpattern s0 listener rows in the Java
// join shape: select * over the every-A -> B pattern and the MyWindow stream
// produces one row per match whose columns are the stream underlyings, which
// the Java oracle prints with String.valueOf (the SupportBean toString for
// the window event and the {tag=BeanEventBean ...} map toString for the
// pattern match). The strings are rendered from the actual Go result values
// so a wrong row still fails the comparison.
func infraNWOSPatternRows(results []esper.Result) []compat.ResultRecord {
	if len(results) == 0 {
		return nil
	}
	rows := make([]compat.ResultRecord, 0, len(results))
	for _, result := range results {
		if row, ok := result.Row(); ok {
			fields := make(map[string]any)
			for _, field := range row.Schema().Fields() {
				fields[field.Name] = infraNWOSJavaRender(row.Get(field.Name).Any())
			}
			rows = append(rows, compat.ResultRecord{Kind: "row", Fields: fields})
			continue
		}
		if event, ok := result.Event(); ok {
			fields := make(map[string]any)
			for _, field := range event.Schema().Fields() {
				fields[field.Name] = infraNWOSJavaRender(event.Get(field.Name).Any())
			}
			rows = append(rows, compat.ResultRecord{Kind: "row", Fields: fields})
			continue
		}
		rows = append(rows, compat.NormalizeResults([]esper.Result{result})...)
	}
	return rows
}

// infraNWOSJavaRender renders one join column the way Java's
// String.valueOf(event.get(name)) prints it: a SupportBean underlying as
// "SupportBean(<theString>, <intPrimitive>)", a pattern-match map as the
// "{tag=BeanEventBean eventType=BeanEventType name=SupportBean
// clazz=com.espertech.esper.common.internal.support.SupportBean
// bean=SupportBean(<theString>, <intPrimitive>)}" toString, and scalars
// through fmt.
func infraNWOSJavaRender(value any) any {
	switch v := value.(type) {
	case nil:
		return map[string]any{"state": "null"}
	case esper.Event:
		return infraNWOSJavaRender(v.Underlying())
	case *esper.Event:
		if v == nil {
			return map[string]any{"state": "null"}
		}
		return infraNWOSJavaRender(v.Underlying())
	case infraNWOSBean:
		return fmt.Sprintf("SupportBean(%s, %d)", v.TheString, v.IntPrimitive)
	case *infraNWOSBean:
		if v == nil {
			return map[string]any{"state": "null"}
		}
		return fmt.Sprintf("SupportBean(%s, %d)", v.TheString, v.IntPrimitive)
	case esper.Row:
		if theString, intPrimitive, ok := infraNWOSBeanFields(v.AsMap()); ok {
			return fmt.Sprintf("SupportBean(%s, %d)", theString, intPrimitive)
		}
		return infraNWOSJavaRender(v.AsMap())
	case map[string]any:
		// Pattern-match map: Java prints {tag=BeanEventBean ...} in map order;
		// the pinned pattern carries a single tag so sorted keys are stable.
		names := make([]string, 0, len(v))
		for name := range v {
			names = append(names, name)
		}
		sort.Strings(names)
		var rendered strings.Builder
		rendered.WriteByte('{')
		for index, name := range names {
			if index > 0 {
				rendered.WriteString(", ")
			}
			rendered.WriteString(name)
			rendered.WriteByte('=')
			rendered.WriteString(infraNWOSJavaRenderTagged(v[name]))
		}
		rendered.WriteByte('}')
		return rendered.String()
	}
	return fmt.Sprintf("%v", value)
}

// infraNWOSJavaRenderTagged renders one pattern-match tag value the way Java
// prints the tagged BeanEventBean inside the match map.
func infraNWOSJavaRenderTagged(value any) string {
	theString, intPrimitive, ok := infraNWOSBeanFields(value)
	if !ok {
		return fmt.Sprintf("%v", value)
	}
	return fmt.Sprintf("BeanEventBean eventType=BeanEventType name=SupportBean "+
		"clazz=com.espertech.esper.common.internal.support.SupportBean "+
		"bean=SupportBean(%s, %d)", theString, intPrimitive)
}

// infraNWOSBeanFields extracts theString/intPrimitive from whichever value
// shape the engine surfaces for a SupportBean event (typed struct, event
// envelope or property map).
func infraNWOSBeanFields(value any) (string, int64, bool) {
	switch v := value.(type) {
	case infraNWOSBean:
		return v.TheString, v.IntPrimitive, true
	case *infraNWOSBean:
		if v == nil {
			return "", 0, false
		}
		return v.TheString, v.IntPrimitive, true
	case esper.Event:
		return infraNWOSBeanFields(v.Underlying())
	case *esper.Event:
		if v == nil {
			return "", 0, false
		}
		return infraNWOSBeanFields(v.Underlying())
	case esper.Row:
		return infraNWOSBeanFields(v.AsMap())
	case map[string]any:
		theString, _ := v["theString"].(string)
		var intPrimitive int64
		switch n := v["intPrimitive"].(type) {
		case int64:
			intPrimitive = n
		case int:
			intPrimitive = int64(n)
		case float64:
			intPrimitive = int64(n)
		case json.Number:
			parsed, err := n.Int64()
			if err != nil {
				return "", 0, false
			}
			intPrimitive = parsed
		default:
			return "", 0, false
		}
		return theString, intPrimitive, true
	}
	return "", 0, false
}
