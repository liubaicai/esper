package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	infraNamedWindowInsertShapeID          = "infra-named-window-insert-shape"
	infraNamedWindowInsertShapeDescription = "InfraNamedWindowViews insert-shape slice: duplicate insert delivery into one keep-all window, intersecting length/unique retention with old-stream replacement, the object-array stream-star insert whose undeclared column is an approved typed-builder difference, and preemptive on-trigger cascade across two named windows (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."
	infraNamedWindowInsertShapeJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNamedWindowInsertShapeSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

	infraNWSCreateDouble = "@name('create') create window MyWindowDISM#keepall as MySimpleKeyValueMap"
	infraNWSInsertOne    = "insert into MyWindowDISM select theString as key, longBoxed+1 as value from SupportBean"
	infraNWSInsertTwo    = "insert into MyWindowDISM select theString as key, longBoxed+2 as value from SupportBean"
	infraNWSS0Double     = "@name('s0') select key, value as value from MyWindowDISM"

	infraNWSCreateIntersection = "create window MyWindowINT#length(2)#unique(intPrimitive) as SupportBean"
	infraNWSInsertIntersection = "insert into MyWindowINT select * from SupportBean"
	infraNWSS0Intersection     = "@name('s0') select irstream * from MyWindowINT"

	infraNWSCreateObjectArray = "@EventRepresentation('objectarray') create window MyNWWindowObjectArray#keepall (p0 int)"
	infraNWSInsertObjectArray = "insert into MyNWWindowObjectArray select intPrimitive as p0, sb.* as c0 from SupportBean as sb"

	infraNWSSchemaPreemptive = "@public @buseventtype create schema TypeOne(col1 int);\n@public @buseventtype create schema TypeTwo(col2 int);\n@public @buseventtype create schema TypeTrigger(trigger int)"
	infraNWSCreatePreemptive = "create window WinOne#keepall as TypeOne;\ncreate window WinTwo#keepall as TypeTwo"
	infraNWSInsertPreemptive = "@name('insert-window-one') insert into WinOne(col1) select intPrimitive from SupportBean"
	infraNWSS2Preemptive     = "@name('insert-otherstream') on TypeTrigger insert into OtherStream select col1 from WinOne"
	infraNWSS3Preemptive     = "@name('insert-window-two') on TypeTrigger insert into WinTwo(col2) select col1 from WinOne"
	infraNWSS0Preemptive     = "@name('s0') on OtherStream select col2 from WinTwo"

	infraNWSDescriptionDouble = "keepall named window receives two independent inserts from each SupportBean and its create/s0 listeners flatten the two rows in insert order"
	infraNWSDescriptionInt    = "length(2) and unique(intPrimitive) intersect so E3/intPrimitive=2 emits E3 and expires E1/E2 as old rows"
	infraNWSDescriptionObject = "object-array keepall deployment accepts the stream-star c0 source in Java but the typed Go builder rejects undeclared c0 while p0-only remains expressible"
	infraNWSDescriptionPre    = "TypeTrigger fires both on-trigger inserts preemptively so OtherStream's s0 observes WinTwo col2=9"
)

var (
	infraNamedWindowInsertShapeJavaSources = []string{
		infraNamedWindowInsertShapeSource,
	}
	infraNamedWindowInsertShapeJavaRuntimeIDs = []string{
		"java-runtime-1b9f5b6ccf6b49cdba09",
		"java-runtime-f739aa91028d27aa572f",
		"java-runtime-9740441818d84a3ee94d",
		"java-runtime-71402af94b27b4255ab8",
	}
	infraNamedWindowInsertShapeJavaExecutions = []string{
		"InfraDoubleInsertSameWindow",
		"InfraIntersection",
		"InfraSelectStreamDotStarInsert",
		"InfraOnInsertPremptiveTwoWindow",
	}
	infraNamedWindowInsertShapeJavaStaticIDs = []string{
		"java-bed54e012c8a814aa1b2",
		"java-1aec04798b31edb42da8",
		"java-e9459e4192187d4bf9a9",
		"java-657282705cf5d824b721",
	}
)

type infraNWSExpectedSend struct {
	eventType  string
	fields     []string
	stringVal  string
	intVal     int
	longVal    int64
	trigger    int
	hasInt     bool
	hasLong    bool
	hasTrigger bool
}

type infraNWSCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	description string
	observation string
	epl         string

	createEPL            string
	createObjectArrayEPL string
	createMapEPL         string
	createAvroEPL        string
	insertEPL            string
	s0EPL                string
	s2EPL                string
	s3EPL                string
	consumeEPL           string
	deleteEPL            string
	varEPL               string
	onSetEPL             string
	schemaEPL            string
	updateEPL            string
	fafEPL               string
	negativeCompileEPL   string

	deploys           []string
	listened          []string
	iteratorSnapshots int
	caseMode          string
	rowFields         map[string][]string
	sends             []infraNWSExpectedSend
}

var infraNWSCaseSpecs = []infraNWSCaseSpec{
	{
		name:        "double-insert-same-window",
		ordinal:     28,
		runtimeID:   "java-runtime-1b9f5b6ccf6b49cdba09",
		execution:   "InfraDoubleInsertSameWindow",
		description: infraNWSDescriptionDouble,
		observation: "listener",
		epl:         infraNWSCreateDouble,
		createEPL:   infraNWSCreateDouble,
		insertEPL:   infraNWSInsertOne,
		s0EPL:       infraNWSS0Double,
		s2EPL:       infraNWSInsertTwo,
		deploys:     []string{"create", "insert-one", "insert-two", "s0"},
		listened:    []string{"create", "s0"},
		rowFields: map[string][]string{
			"create": {"key", "value"},
			"s0":     {"key", "value"},
		},
		sends: []infraNWSExpectedSend{
			{eventType: "SupportBean", fields: []string{"theString", "longBoxed"}, stringVal: "E1", longVal: 10, hasLong: true},
		},
	},
	{
		name:        "intersection",
		ordinal:     36,
		runtimeID:   "java-runtime-f739aa91028d27aa572f",
		execution:   "InfraIntersection",
		description: infraNWSDescriptionInt,
		observation: "listener",
		epl:         infraNWSCreateIntersection,
		createEPL:   infraNWSCreateIntersection,
		insertEPL:   infraNWSInsertIntersection,
		s0EPL:       infraNWSS0Intersection,
		deploys:     []string{"create", "insert", "s0"},
		listened:    []string{"s0"},
		caseMode:    "any",
		rowFields: map[string][]string{
			"s0": {"theString", "intPrimitive"},
		},
		sends: []infraNWSExpectedSend{
			{eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, stringVal: "E1", intVal: 1, hasInt: true},
			{eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, stringVal: "E2", intVal: 2, hasInt: true},
			{eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, stringVal: "E3", intVal: 2, hasInt: true},
		},
	},
	{
		name:        "select-stream-dot-star-insert",
		ordinal:     54,
		runtimeID:   "java-runtime-9740441818d84a3ee94d",
		execution:   "InfraSelectStreamDotStarInsert",
		description: infraNWSDescriptionObject,
		observation: "listener",
		epl:         infraNWSCreateObjectArray,
		createEPL:   infraNWSCreateObjectArray,
		insertEPL:   infraNWSInsertObjectArray,
		deploys:     []string{"create", "insert"},
		listened:    []string{},
		rowFields:   map[string][]string{},
		sends:       []infraNWSExpectedSend{},
	},
	{
		name:        "on-insert-preemptive-two-window",
		ordinal:     56,
		runtimeID:   "java-runtime-71402af94b27b4255ab8",
		execution:   "InfraOnInsertPremptiveTwoWindow",
		description: infraNWSDescriptionPre,
		observation: "listener",
		epl:         infraNWSSchemaPreemptive + "\n" + infraNWSCreatePreemptive,
		createEPL:   infraNWSCreatePreemptive,
		insertEPL:   infraNWSInsertPreemptive,
		s0EPL:       infraNWSS0Preemptive,
		s2EPL:       infraNWSS2Preemptive,
		s3EPL:       infraNWSS3Preemptive,
		schemaEPL:   infraNWSSchemaPreemptive,
		deploys:     []string{"schema", "create", "insert", "s2", "s3", "s0"},
		listened:    []string{"s0"},
		rowFields: map[string][]string{
			"s0": {"col2"},
		},
		sends: []infraNWSExpectedSend{
			{eventType: "SupportBean", fields: []string{"theString", "intPrimitive"}, stringVal: "E1", intVal: 9, hasInt: true},
			{eventType: "TypeTrigger", fields: []string{"trigger"}, trigger: 0, hasTrigger: true},
		},
	},
}

type infraNWSCaseDefinition struct {
	Case                 string   `json:"case"`
	Ordinal              int      `json:"ordinal"`
	RuntimeID            string   `json:"runtimeId"`
	ExecutionName        string   `json:"executionName"`
	Description          string   `json:"description"`
	Observation          string   `json:"observation"`
	IteratorSnapshots    int      `json:"iteratorSnapshots"`
	EPL                  string   `json:"epl"`
	CreateEPL            string   `json:"createEpl"`
	CreateObjectArrayEPL string   `json:"createEplObjectArray"`
	CreateMapEPL         string   `json:"createEplMap"`
	CreateAvroEPL        string   `json:"createEplAvro"`
	InsertEPL            string   `json:"insertEpl"`
	S0EPL                string   `json:"s0Epl"`
	S2EPL                string   `json:"s2Epl"`
	S3EPL                string   `json:"s3Epl"`
	ConsumeEPL           string   `json:"consumeEpl"`
	DeleteEPL            string   `json:"deleteEpl"`
	VarEPL               string   `json:"varEpl"`
	OnSetEPL             string   `json:"onSetEpl"`
	SchemaEPL            string   `json:"schemaEpl"`
	UpdateEPL            string   `json:"updateEpl"`
	FafEPL               string   `json:"fafEpl"`
	NegativeCompileEPL   string   `json:"negativeCompileEpl"`
	Deploys              []string `json:"deploys"`
	Listened             []string `json:"listened"`
}

type infraNWSFieldSlot struct {
	name string
	got  string
	want string
}

func infraNWSCaseSpecFor(name string) (infraNWSCaseSpec, bool) {
	for _, spec := range infraNWSCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWSCaseSpec{}, false
}

func loadInfraNamedWindowInsertShapeScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNamedWindowInsertShapeID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNamedWindowInsertShapeID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNamedWindowInsertShapeID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNamedWindowInsertShapeID, err)
	}
	if err := requireInfraNWSFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version       string   `json:"version"`
		ID            string   `json:"id"`
		Description   string   `json:"description"`
		JavaCommit    string   `json:"javaCommit"`
		JavaSource    string   `json:"javaSource"`
		JavaRuntimes  []string `json:"javaRuntimes"`
		JavaNames     []string `json:"javaNames"`
		JavaStaticIDs []string `json:"javaStaticIds"`
		JavaFlags     []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNamedWindowInsertShapeID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNamedWindowInsertShapeID ||
		metadata.Description != infraNamedWindowInsertShapeDescription ||
		metadata.JavaCommit != infraNamedWindowInsertShapeJavaCommit || metadata.JavaSource != infraNamedWindowInsertShapeSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNamedWindowInsertShapeID)
	}
	if err := infraNWSRequireEqual(metadata.JavaRuntimes, infraNamedWindowInsertShapeJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWSRequireEqual(metadata.JavaNames, infraNamedWindowInsertShapeJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWSRequireEqual(metadata.JavaStaticIDs, infraNamedWindowInsertShapeJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWSRequireEqual(metadata.JavaFlags, []string{"EVENTSENDER"}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWSCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNamedWindowInsertShapeID, len(infraNWSCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWSFields(object,
			"case", "ordinal", "runtimeId", "executionName", "description", "observation", "iteratorSnapshots", "epl",
			"createEpl", "createEplObjectArray", "createEplMap", "createEplAvro", "insertEpl", "s0Epl", "s2Epl", "s3Epl",
			"consumeEpl", "deleteEpl", "varEpl", "onSetEpl", "schemaEpl", "updateEpl", "fafEpl", "negativeCompileEpl",
			"deploys", "listened"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition infraNWSCaseDefinition
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := infraNWSCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal || definition.RuntimeID != spec.runtimeID ||
			definition.ExecutionName != spec.execution || definition.Description != spec.description ||
			definition.Observation != spec.observation || definition.IteratorSnapshots != spec.iteratorSnapshots || definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("scenario case %d metadata is not pinned", index)
		}
		slots := []infraNWSFieldSlot{
			{"createEpl", definition.CreateEPL, spec.createEPL},
			{"createEplObjectArray", definition.CreateObjectArrayEPL, spec.createObjectArrayEPL},
			{"createEplMap", definition.CreateMapEPL, spec.createMapEPL},
			{"createEplAvro", definition.CreateAvroEPL, spec.createAvroEPL},
			{"insertEpl", definition.InsertEPL, spec.insertEPL},
			{"s0Epl", definition.S0EPL, spec.s0EPL},
			{"s2Epl", definition.S2EPL, spec.s2EPL},
			{"s3Epl", definition.S3EPL, spec.s3EPL},
			{"consumeEpl", definition.ConsumeEPL, spec.consumeEPL},
			{"deleteEpl", definition.DeleteEPL, spec.deleteEPL},
			{"varEpl", definition.VarEPL, spec.varEPL},
			{"onSetEpl", definition.OnSetEPL, spec.onSetEPL},
			{"schemaEpl", definition.SchemaEPL, spec.schemaEPL},
			{"updateEpl", definition.UpdateEPL, spec.updateEPL},
			{"fafEpl", definition.FafEPL, spec.fafEPL},
			{"negativeCompileEpl", definition.NegativeCompileEPL, spec.negativeCompileEPL},
		}
		for _, slot := range slots {
			if slot.got != slot.want {
				return compat.Scenario{}, fmt.Errorf("scenario case %q %s is not pinned", spec.name, slot.name)
			}
		}
		if err := infraNWSRequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWSRequireEqual(definition.Listened, spec.listened, spec.name+" listened"); err != nil {
			return compat.Scenario{}, err
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNamedWindowInsertShapeID)
	}
	if err := validateInfraNWSRawSteps(rawSteps); err != nil {
		return compat.Scenario{}, err
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateInfraNamedWindowInsertShapeScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateInfraNWSRawSteps(rawSteps []json.RawMessage) error {
	deployCounts := make(map[string]int)
	sendCounts := make(map[string]int)
	undeployCounts := make(map[string]int)
	caseOrder := make([]string, 0, len(infraNWSCaseSpecs))
	currentCase := ""
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return fmt.Errorf("scenario step %d op must be a string", index)
		}
		switch operation {
		case "case":
			var step struct {
				Case string `json:"case"`
				Mode string `json:"mode"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWSCaseSpecFor(step.Case)
			if !ok {
				return fmt.Errorf("scenario step %d selects unknown case %q", index, step.Case)
			}
			allowed := []string{"op", "case"}
			if spec.caseMode != "" {
				allowed = append(allowed, "mode")
			}
			if err := requireInfraNWSFields(object, allowed...); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Mode != spec.caseMode {
				return fmt.Errorf("scenario step %d case %q mode %q must be %q", index, step.Case, step.Mode, spec.caseMode)
			}
			currentCase = step.Case
			caseOrder = append(caseOrder, step.Case)
		case "deploy":
			if err := requireInfraNWSFields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
				EPL       string `json:"epl"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWSCaseSpecFor(step.Case)
			if !ok || step.Case != currentCase {
				return fmt.Errorf("scenario step %d deploy is outside its case block", index)
			}
			position := deployCounts[step.Case]
			if position >= len(spec.deploys) || step.Statement != spec.deploys[position] {
				return fmt.Errorf("scenario step %d deploy statement %q is not pinned for case %q", index, step.Statement, step.Case)
			}
			if want := infraNWSDeployEPL(spec, step.Statement); step.EPL != want {
				return fmt.Errorf("scenario step %d deploy EPL is not pinned for statement %q", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWSFields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step compat.Step
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWSCaseSpecFor(step.Case)
			if !ok || step.Case != currentCase {
				return fmt.Errorf("scenario step %d send is outside its case block", index)
			}
			position := sendCounts[step.Case]
			if position >= len(spec.sends) {
				return fmt.Errorf("scenario case %q has too many sends", step.Case)
			}
			if err := infraNWSValidateSend(spec, position, step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			sendCounts[step.Case]++
		case "undeploy-all":
			if err := requireInfraNWSFields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := infraNWSCaseSpecFor(step.Case); !ok || step.Case != currentCase {
				return fmt.Errorf("scenario step %d undeploy-all is outside its case block", index)
			}
			undeployCounts[step.Case]++
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	if len(caseOrder) != len(infraNWSCaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want four cases", caseOrder)
	}
	for index, spec := range infraNWSCaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if deployCounts[spec.name] != len(spec.deploys) || sendCounts[spec.name] != len(spec.sends) || undeployCounts[spec.name] != 1 {
			return fmt.Errorf("scenario case %q has unpinned step counts", spec.name)
		}
	}
	return nil
}

func validateInfraNamedWindowInsertShapeScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNamedWindowInsertShapeID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, infraNamedWindowInsertShapeID)
	}
	if err := validateInfraNWSRawStepsFromScenario(scenario); err != nil {
		return err
	}
	return nil
}

func validateInfraNWSRawStepsFromScenario(scenario compat.Scenario) error {
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validateInfraNWSRawSteps(rawSteps)
}

func infraNWSDeployEPL(spec infraNWSCaseSpec, statement string) string {
	switch statement {
	case "create":
		return spec.createEPL
	case "insert", "insert-one":
		return spec.insertEPL
	case "insert-two", "s2":
		return spec.s2EPL
	case "s0":
		return spec.s0EPL
	case "s3":
		return spec.s3EPL
	case "schema":
		return spec.schemaEPL
	default:
		return ""
	}
}

func infraNWSValidateSend(spec infraNWSCaseSpec, index int, step compat.Step) error {
	want := spec.sends[index]
	if step.EventType != want.eventType {
		return fmt.Errorf("case %q send %d event type %q, want %q", spec.name, index, step.EventType, want.eventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return err
	}
	if err := requireInfraNWSFields(fields, want.fields...); err != nil {
		return err
	}
	switch want.eventType {
	case "SupportBean":
		var payload infraNWSSupportBean
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("decode SupportBean: %w", err)
		}
		if payload.TheString != want.stringVal || (want.hasInt && payload.IntPrimitive != want.intVal) || (want.hasLong && payload.LongBoxed != want.longVal) {
			return fmt.Errorf("case %q SupportBean payload %d is not pinned", spec.name, index)
		}
	case "TypeTrigger":
		var payload struct {
			Trigger int `json:"trigger"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("decode TypeTrigger: %w", err)
		}
		if !want.hasTrigger || payload.Trigger != want.trigger {
			return fmt.Errorf("case %q TypeTrigger payload is not pinned", spec.name)
		}
	default:
		return fmt.Errorf("unsupported event type %q", want.eventType)
	}
	return nil
}

func infraNWSRequireEqual(got, want []string, label string) error {
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

func requireInfraNWSFields(object map[string]json.RawMessage, names ...string) error {
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

type infraNWSSupportBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
	IntBoxed      int    `esper:"intBoxed"`
	LongBoxed     int64  `esper:"longBoxed"`
}

type infraNWSKeyValueLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

func runInfraNamedWindowInsertShapeScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNamedWindowInsertShapeScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWSCaseSpecs {
		records, err := runInfraNWSCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNamedWindowInsertShapeID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func infraNWSStartEnvironment(spec infraNWSCaseSpec) (*esper.Environment, error) {
	env := esper.NewEnvironment()
	switch spec.name {
	case "double-insert-same-window":
		if _, err := esper.RegisterStruct[infraNWSSupportBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[infraNWSKeyValueLong](env, "MyWindowDISM"); err != nil {
			return nil, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowDISM", mustInfraNWSSchema(env, "MyWindowDISM"), esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
	case "intersection":
		schema, err := esper.RegisterStruct[infraNWSSupportBean](env, "SupportBean")
		if err != nil {
			return nil, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowINT", schema, esper.NamedWindowRetention(esper.IntersectWindows(
			esper.LengthWindow(2),
			esper.Unique(esper.Field[infraNWSSupportBean, int]("intPrimitive")),
		))); err != nil {
			return nil, err
		}
	case "select-stream-dot-star-insert":
		if _, err := esper.RegisterStruct[infraNWSSupportBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterObjectArray(env, "MyNWWindowObjectArray", []esper.FieldSpec{
			esper.FieldDef("p0", reflect.TypeOf(0)),
		}); err != nil {
			return nil, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyNWWindowObjectArray", mustInfraNWSSchema(env, "MyNWWindowObjectArray"), esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
	case "on-insert-preemptive-two-window":
		if _, err := esper.RegisterStruct[infraNWSSupportBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		intType := reflect.TypeOf(0)
		typeOne, err := esper.RegisterMap(env, "TypeOne", []esper.FieldSpec{esper.FieldDef("col1", intType)})
		if err != nil {
			return nil, err
		}
		typeTwo, err := esper.RegisterMap(env, "TypeTwo", []esper.FieldSpec{esper.FieldDef("col2", intType)})
		if err != nil {
			return nil, err
		}
		if _, err := esper.RegisterMap(env, "TypeTrigger", []esper.FieldSpec{esper.FieldDef("trigger", intType)}); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterMap(env, "OtherStream", []esper.FieldSpec{esper.FieldDef("col1", intType)}); err != nil {
			return nil, err
		}
		if _, err := esper.CreateNamedWindow(env, "WinOne", typeOne, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
		if _, err := esper.CreateNamedWindow(env, "WinTwo", typeTwo, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown case %q", spec.name)
	}
	return env, nil
}

func mustInfraNWSSchema(env *esper.Environment, name string) esper.Schema {
	schema, ok := env.Schema(name)
	if !ok {
		return esper.Schema{}
	}
	return schema
}

func runInfraNWSCase(ctx context.Context, scenario compat.Scenario, spec infraNWSCaseSpec) ([]compat.TraceRecord, error) {
	env, err := infraNWSStartEnvironment(spec)
	if err != nil {
		return nil, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(spec.runtimeID), esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := make(map[string]uint64)
	statements := make(map[string]*esper.Statement)
	var records []compat.TraceRecord
	recordListener := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		newRows := projectRecords(compat.NormalizeResults(batch.New), spec.rowFields[statement])
		oldRows := projectRecords(compat.NormalizeResults(batch.Old), spec.rowFields[statement])
		if spec.caseMode == "any" {
			sortRowsCanonical(newRows)
			sortRowsCanonical(oldRows)
		}
		records = append(records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(time.Unix(0, 0).UTC()),
			New:       newRows,
			Old:       oldRows,
		})
	}

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
			if spec.name == "on-insert-preemptive-two-window" && (step.Statement == "schema" || step.Statement == "create") {
				continue
			}
			if spec.name == "select-stream-dot-star-insert" && step.Statement == "insert" {
				if err := infraNWSAssertObjectArrayBoundary(env); err != nil {
					return nil, err
				}
				plan, err := infraNWSBuildObjectArrayP0Plan(env)
				if err != nil {
					return nil, err
				}
				if _, err := engine.Deploy(ctx, plan); err != nil {
					return nil, err
				}
				continue
			}
			plan, err := infraNWSBuildPlan(env, spec, step.Statement)
			if err != nil {
				return nil, fmt.Errorf("build %q: %w", step.Statement, err)
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
			if infraNWSContains(spec.listened, step.Statement) {
				statement, ok := statements[step.Statement]
				if !ok {
					return nil, fmt.Errorf("deploy %q did not register its statement", step.Statement)
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
			payload, err := infraNWSDecodePayload(spec, step)
			if err != nil {
				return nil, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "undeploy-all":
		default:
			return nil, fmt.Errorf("unsupported step op %q", step.Op)
		}
	}
	return records, nil
}

func infraNWSBuildPlan(env *esper.Environment, spec infraNWSCaseSpec, statement string) (esper.Plan, error) {
	switch spec.name {
	case "double-insert-same-window":
		source := esper.From[infraNWSSupportBean](env, "SupportBean")
		switch statement {
		case "create":
			return env.Build(esper.FromNamedWindow(env, "MyWindowDISM").CreateNamedWindowQuery(esper.StatementName("create")))
		case "insert-one":
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyWindowDISM",
				esper.SetColumn("key", esper.Field[infraNWSSupportBean, string]("theString")),
				esper.SetColumn("value", esper.Add[int64](esper.Field[infraNWSSupportBean, int64]("longBoxed"), esper.Literal[int64](1))),
			).Query(esper.StatementName("insert-one")))
		case "insert-two":
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyWindowDISM",
				esper.SetColumn("key", esper.Field[infraNWSSupportBean, string]("theString")),
				esper.SetColumn("value", esper.Add[int64](esper.Field[infraNWSSupportBean, int64]("longBoxed"), esper.Literal[int64](2))),
			).Query(esper.StatementName("insert-two")))
		case "s0":
			return env.Build(esper.FromNamedWindow(env, "MyWindowDISM").Query(esper.StatementName("s0")))
		}
	case "intersection":
		source := esper.From[infraNWSSupportBean](env, "SupportBean")
		switch statement {
		case "create":
			return env.Build(esper.FromNamedWindow(env, "MyWindowINT").CreateNamedWindowQuery(esper.StatementName("create")))
		case "insert":
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyWindowINT",
				esper.SetColumn("theString", esper.Field[infraNWSSupportBean, string]("theString")),
				esper.SetColumn("intPrimitive", esper.Field[infraNWSSupportBean, int]("intPrimitive")),
				esper.SetColumn("longPrimitive", esper.Field[infraNWSSupportBean, int64]("longPrimitive")),
				esper.SetColumn("boolPrimitive", esper.Field[infraNWSSupportBean, bool]("boolPrimitive")),
				esper.SetColumn("intBoxed", esper.Field[infraNWSSupportBean, int]("intBoxed")),
				esper.SetColumn("longBoxed", esper.Field[infraNWSSupportBean, int64]("longBoxed")),
			).Query(esper.StatementName("insert")))
		case "s0":
			return env.Build(esper.FromNamedWindow(env, "MyWindowINT").Query(esper.StatementName("s0"), esper.WithOldStream()))
		}
	case "select-stream-dot-star-insert":
		switch statement {
		case "create":
			return env.Build(esper.FromNamedWindow(env, "MyNWWindowObjectArray").CreateNamedWindowQuery(esper.StatementName("create")))
		}
	case "on-insert-preemptive-two-window":
		switch statement {
		case "insert":
			return env.Build(esper.OnEvent(esper.From[infraNWSSupportBean](env, "SupportBean")).InsertIntoNamedWindow(
				"WinOne", esper.SetColumn("col1", esper.Field[infraNWSSupportBean, int]("intPrimitive")),
			).Query(esper.StatementName("insert")))
		case "s2":
			return env.Build(esper.OnRecord(esper.FromAny(env, "TypeTrigger")).SelectFromNamedWindow(
				"WinOne", nil, esper.Alias("col1", esper.NamedWindowField[int]("col1")),
			).Query(esper.RouteTo("OtherStream"), esper.StatementName("s2")))
		case "s3":
			return env.Build(esper.OnRecord(esper.FromAny(env, "TypeTrigger")).InsertIntoNamedWindowFrom(
				"WinTwo", "WinOne", esper.SetColumn("col2", esper.NamedWindowField[int]("col1")),
			).Query(esper.StatementName("s3")))
		case "s0":
			return env.Build(esper.OnRecord(esper.FromAny(env, "OtherStream")).SelectFromNamedWindow(
				"WinTwo", nil, esper.Alias("col2", esper.NamedWindowField[int]("col2")),
			).Query(esper.StatementName("s0")))
		}
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
}

func infraNWSAssertObjectArrayBoundary(env *esper.Environment) error {
	source := esper.From[infraNWSSupportBean](env, "SupportBean")
	_, err := env.Build(esper.OnEvent(source).InsertIntoNamedWindow(
		"MyNWWindowObjectArray",
		esper.SetColumn("p0", esper.Field[infraNWSSupportBean, int]("intPrimitive")),
		esper.SetColumn("c0", esper.Field[infraNWSSupportBean, string]("theString")),
	).Query(esper.StatementName("insert-c0")))
	if err == nil || !errors.Is(err, esper.ErrorUnknownName) {
		return fmt.Errorf("object-array stream-star build error = %v, want ErrorUnknownName", err)
	}
	return nil
}

func infraNWSBuildObjectArrayP0Plan(env *esper.Environment) (esper.Plan, error) {
	source := esper.From[infraNWSSupportBean](env, "SupportBean")
	return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(
		"MyNWWindowObjectArray",
		esper.SetColumn("p0", esper.Field[infraNWSSupportBean, int]("intPrimitive")),
	).Query(esper.StatementName("insert")))
}

func infraNWSDecodePayload(spec infraNWSCaseSpec, step compat.Step) (any, error) {
	index := 0
	for _, prior := range spec.sends {
		_ = prior
		break
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	if step.EventType == "SupportBean" {
		var payload infraNWSSupportBean
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return payload, nil
	}
	if step.EventType == "TypeTrigger" {
		var payload struct {
			Trigger int `json:"trigger"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode TypeTrigger: %w", err)
		}
		return map[string]any{"trigger": payload.Trigger}, nil
	}
	_ = index
	return nil, fmt.Errorf("unsupported %s event type %q", infraNamedWindowInsertShapeID, step.EventType)
}

func infraNWSContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
