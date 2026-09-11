package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the twelfth differential slice of InfraNamedWindowViews:
// the bean-backed, bean-contained, schema-backed and deep-supertype-insert
// named windows.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 2  InfraBeanBacked          java-runtime-f0a1da1fe931e132c21f
//   - ord 35 InfraBeanContained       java-runtime-fe6adc5803da60bf18f5
//   - ord 37 InfraBeanSchemaBacked    java-runtime-f195548d023dfbde1aed
//   - ord 38 InfraDeepSupertypeInsert java-runtime-baa0bd4ca41b9b2c5f94
//
// Java builds these window types from the create-window text: `as <bean class>`
// re-creates a BeanEventType named after the window and drops any requested
// representation, `as (<col> <type>)` builds a nestable type whose single
// `bean` column carries a whole POJO fragment, `create schema X as <class>`
// declares a distinct event type over the same class so that sends of the
// class never route to the schema type, and `as select * from <class>` builds
// a bean replication of the source class's property set. The delivered events
// therefore differ only in their event-type identity, which the trace carries
// as the row's `type` field for the two cases whose Java assertions are
// type-metadata only.
//
// The representation matrices run in full: each cycle deploys its annotated
// create statement, fills and exercises the window and undeploys everything,
// so every cycle starts from an empty window exactly like the Java suite's
// per-sub-run runtime. The annotation carrier is inert in the Go port (the
// window schema is chosen by the case shape, not by the annotation), and the
// suite's trailing AVRO compile-reject of the contained-bean window has no Go
// counterpart because Go Avro schemas are record-field declarations normalized
// at ingestion.
const infraNWBVId = "infra-named-window-bean-views"

const infraNWBVDescription = "InfraNamedWindowViews bean slice: the bean-backed window with its representation matrix and on-trigger update, the contained-bean window fed by a stream-wildcard insert, the schema alias read through a fire-and-forget query, and the deep-supertype insert whose window reads the most-derived override (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWBVJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWBVSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// Byte-exact EPL pins (Java lines 3594/3596/3598/3605 for ord 2,
// 3500/3503 for ord 35, 347-349/353/355 for ord 37 and 366-367 for ord 38).
// Each pin is the statement text without the trailing ";" and newline of the
// deployment that joins the statements. The audit representation carriers
// (@EventRepresentation('objectarray'|'map'|'avro')) keep the Java spelling;
// the DEFAULT variant drops the source's leading space, which carries no
// semantics ("" + " @name(...)").
const (
	infraNWBVCreateBeanBacked            = "@name('create') @public create window MyWindowBB#keepall as SupportBean"
	infraNWBVCreateBeanBackedObjectArray = "@EventRepresentation('objectarray') @name('create') @public create window MyWindowBB#keepall as SupportBean"
	infraNWBVCreateBeanBackedMap         = "@EventRepresentation('map') @name('create') @public create window MyWindowBB#keepall as SupportBean"
	infraNWBVCreateBeanBackedAvro        = "@EventRepresentation('avro') @name('create') @public create window MyWindowBB#keepall as SupportBean"
	infraNWBVInsertBeanBacked            = "@public insert into MyWindowBB select * from SupportBean"
	infraNWBVSelectBeanBacked            = "@name('s0') select * from MyWindowBB"
	infraNWBVUpdateBeanBacked            = "@name('update') on SupportBean_A update MyWindowBB set theString='s'"

	infraNWBVCreateBeanContained            = "@name('create') @public create window MyWindowBC#keepall as (bean SupportBean_S0)"
	infraNWBVCreateBeanContainedObjectArray = "@EventRepresentation('objectarray') @name('create') @public create window MyWindowBC#keepall as (bean SupportBean_S0)"
	infraNWBVCreateBeanContainedMap         = "@EventRepresentation('map') @name('create') @public create window MyWindowBC#keepall as (bean SupportBean_S0)"
	// The AVRO annotation of the contained-bean window is not part of the run
	// matrix (the loop skips Avro/Json representations); it appears only as the
	// suite's expected compile failure, which carries no @public.
	infraNWBVNegativeCompileBeanContained = "@EventRepresentation('avro') @name('create') create window MyWindowBC#keepall as (bean SupportBean_S0)"
	infraNWBVInsertBeanContained          = "insert into MyWindowBC select bean.* as bean from SupportBean_S0 as bean"

	infraNWBVSchemaBeanSchemaBacked = "@public create schema ABC as com.espertech.esper.common.internal.support.SupportBean"
	infraNWBVCreateBeanSchemaBacked = "@public create window MyWindowBSB#keepall as ABC"
	infraNWBVInsertBeanSchemaBacked = "insert into MyWindowBSB select * from SupportBean"
	infraNWBVSelectBeanSchemaBacked = "@name('s0') select * from ABC"
	infraNWBVFAFBeanSchemaBacked    = "select * from MyWindowBSB"

	infraNWBVCreateDeepSupertype = "@name('create') create window MyWindowDSI#keepall as select * from SupportOverrideBase"
	infraNWBVInsertDeepSupertype = "insert into MyWindowDSI select * from SupportOverrideOneA"
)

// infraNWBVShape selects the named window declaration of a case.
type infraNWBVShape int

const (
	// infraNWBVBeanBackedRows is the ord-2 window: keep-all over the
	// SupportBean bean schema, declared as a bare bean class.
	infraNWBVBeanBackedRows infraNWBVShape = iota
	// infraNWBVBeanContainedRows is the ord-35 window: keep-all over a
	// schema whose single fragment column carries a SupportBean_S0.
	infraNWBVBeanContainedRows
	// infraNWBVBeanSchemaAliasRows is the ord-37 window: keep-all over the
	// ABC schema alias of the SupportBean class.
	infraNWBVBeanSchemaAliasRows
	// infraNWBVDeepSupertypeRows is the ord-38 window: keep-all over the
	// SupportOverrideBase property set, fed by a SupportOverrideOneA insert.
	infraNWBVDeepSupertypeRows
)

type infraNWBVCaseSpec struct {
	name               string
	ordinal            int
	runtimeID          string
	execution          string
	windowName         string
	description        string
	createEPL          string
	createObjArrEPL    string
	createMapEPL       string
	createAvroEPL      string
	insertEPL          string
	s0EPL              string
	s2EPL              string
	s3EPL              string
	consumeEPL         string
	deleteEPL          string
	varEPL             string
	onSetEPL           string
	schemaEPL          string
	updateEPL          string
	fafEPL             string
	negativeCompileEPL string
	deploys            []string
	listened           map[string]bool
	sends              int
	advances           int
	// caseMode is the optional mode of the case step ("any" sorts that case's
	// listener rows); empty means the step carries no mode at all.
	caseMode string
	// undeployStatements is the pinned ordered list of module-scoped undeploy
	// targets (empty when the case tears down with undeploy-all).
	undeployStatements []string
	undeployAlls       int
	fafs               int
	fafFields          []string
	snapshots          int
	// snapModes, snapStatements and snapFields are the per-snapshot pins in
	// scenario order.
	snapModes      []string
	snapStatements []string
	snapFields     [][]string
	shape          infraNWBVShape
	// createVariants is the ordered create-window EPL per cycle; each cycle
	// deploys one of them before the cycle's other statements.
	createVariants []string
	// deploySequence is the ordered deploy-statement sequence of the whole
	// case (all cycles).
	deploySequence []string
	// rowFields projects the listener rows of each listened statement; an
	// empty list keeps the row as a count-only row (the case's Java
	// assertions read event metadata rather than property values).
	rowFields map[string][]string
	// typeTags tags every emitted row with the delivered event's type name,
	// the language-neutral half of Java's assertEvent metadata checks.
	typeTags bool
	// sendFields pins the exact payload fields per sent event type.
	sendFields map[string][]string
}

var infraNWBVCaseSpecs = []infraNWBVCaseSpec{
	{
		name:            "bean-backed",
		ordinal:         2,
		runtimeID:       "java-runtime-f0a1da1fe931e132c21f",
		execution:       "InfraBeanBacked",
		windowName:      "MyWindowBB",
		description:     "keepall window declared as SupportBean and replayed once per event-representation annotation: the annotation is ignored, so every sub-run delivers bean events of the window type named MyWindowBB through the create and s0 listeners and an on-trigger update that delivers the updated row as new and the pre-update row as old",
		createEPL:       infraNWBVCreateBeanBacked,
		createObjArrEPL: infraNWBVCreateBeanBackedObjectArray,
		createMapEPL:    infraNWBVCreateBeanBackedMap,
		createAvroEPL:   infraNWBVCreateBeanBackedAvro,
		insertEPL:       infraNWBVInsertBeanBacked,
		s0EPL:           infraNWBVSelectBeanBacked,
		updateEPL:       infraNWBVUpdateBeanBacked,
		deploys:         []string{"create", "insert", "s0", "update"},
		listened:        map[string]bool{"create": true, "s0": true, "update": true},
		sends:           8,
		undeployAlls:    4,
		shape:           infraNWBVBeanBackedRows,
		createVariants: []string{
			infraNWBVCreateBeanBackedObjectArray,
			infraNWBVCreateBeanBackedMap,
			infraNWBVCreateBeanBacked,
			infraNWBVCreateBeanBackedAvro,
		},
		deploySequence: []string{
			"create", "insert", "s0", "update",
			"create", "insert", "s0", "update",
			"create", "insert", "s0", "update",
			"create", "insert", "s0", "update",
		},
		rowFields: map[string][]string{"create": {}, "s0": {}, "update": {}},
		typeTags:  true,
		sendFields: map[string][]string{
			"SupportBean":   {},
			"SupportBean_A": {"id"},
		},
	},
	{
		name:               "bean-contained",
		ordinal:            35,
		runtimeID:          "java-runtime-fe6adc5803da60bf18f5",
		execution:          "InfraBeanContained",
		windowName:         "MyWindowBC",
		description:        "keepall window declared as (bean SupportBean_S0) with one sub-run per representation: the objectarray and map/default annotations shape the window underlying and the nested bean.p00 property is read through the stream-wildcard insert, while the avro variant must fail to compile",
		createEPL:          infraNWBVCreateBeanContained,
		createObjArrEPL:    infraNWBVCreateBeanContainedObjectArray,
		createMapEPL:       infraNWBVCreateBeanContainedMap,
		insertEPL:          infraNWBVInsertBeanContained,
		negativeCompileEPL: infraNWBVNegativeCompileBeanContained,
		deploys:            []string{"create", "insert"},
		listened:           map[string]bool{"create": true},
		sends:              3,
		undeployAlls:       3,
		shape:              infraNWBVBeanContainedRows,
		createVariants: []string{
			infraNWBVCreateBeanContainedObjectArray,
			infraNWBVCreateBeanContainedMap,
			infraNWBVCreateBeanContained,
		},
		deploySequence: []string{
			"create", "insert",
			"create", "insert",
			"create", "insert",
		},
		rowFields: map[string][]string{"create": {"bean.p00"}},
		sendFields: map[string][]string{
			"SupportBean_S0": {"id", "p00"},
		},
	},
	{
		name:           "bean-schema-backed",
		ordinal:        37,
		runtimeID:      "java-runtime-f195548d023dfbde1aed",
		execution:      "InfraBeanSchemaBacked",
		windowName:     "MyWindowBSB",
		description:    "keepall window declared over a schema alias of the SupportBean class: the schema type ABC is distinct from the configured SupportBean type, so the second bean send feeds only the window and the fire-and-forget query reads one row of the window type MyWindowBSB while the select over ABC stays uninvoked",
		schemaEPL:      infraNWBVSchemaBeanSchemaBacked,
		createEPL:      infraNWBVCreateBeanSchemaBacked,
		insertEPL:      infraNWBVInsertBeanSchemaBacked,
		s0EPL:          infraNWBVSelectBeanSchemaBacked,
		fafEPL:         infraNWBVFAFBeanSchemaBacked,
		deploys:        []string{"schema", "create", "insert", "s0"},
		listened:       map[string]bool{"s0": true},
		sends:          2,
		undeployAlls:   1,
		fafs:           1,
		fafFields:      []string{},
		shape:          infraNWBVBeanSchemaAliasRows,
		createVariants: []string{infraNWBVCreateBeanSchemaBacked},
		deploySequence: []string{"schema", "create", "insert", "s0"},
		rowFields:      map[string][]string{"s0": {}},
		typeTags:       true,
		sendFields: map[string][]string{
			"SupportBean": {},
		},
	},
	{
		name:           "deep-supertype-insert",
		ordinal:        38,
		runtimeID:      "java-runtime-baa0bd4ca41b9b2c5f94",
		execution:      "InfraDeepSupertypeInsert",
		windowName:     "MyWindowDSI",
		description:    "keepall window declared from as select * from SupportOverrideBase and fed by an insert from SupportOverrideOneA: the same underlying object is re-typed into the window and the val property reads the most-derived override",
		createEPL:      infraNWBVCreateDeepSupertype,
		insertEPL:      infraNWBVInsertDeepSupertype,
		deploys:        []string{"create", "insert"},
		listened:       map[string]bool{},
		sends:          1,
		undeployAlls:   1,
		snapshots:      1,
		snapModes:      []string{"ordered"},
		snapStatements: []string{"create"},
		snapFields:     [][]string{{"val"}},
		shape:          infraNWBVDeepSupertypeRows,
		createVariants: []string{infraNWBVCreateDeepSupertype},
		deploySequence: []string{"create", "insert"},
		sendFields: map[string][]string{
			"SupportOverrideOneA": {"valOneA", "valOne", "val"},
		},
	},
}

var (
	infraNWBVJavaSources = []string{
		infraNWBVSource,
	}
	infraNWBVJavaRuntimeIDs = []string{
		"java-runtime-f0a1da1fe931e132c21f",
		"java-runtime-fe6adc5803da60bf18f5",
		"java-runtime-f195548d023dfbde1aed",
		"java-runtime-baa0bd4ca41b9b2c5f94",
	}
	infraNWBVJavaExecutions = []string{
		"InfraBeanBacked",
		"InfraBeanContained",
		"InfraBeanSchemaBacked",
		"InfraDeepSupertypeInsert",
	}
)

// infraNWBVBean mirrors SupportBean for the bean-backed and schema-backed
// windows; each case validates the exact send fields it pins.
type infraNWBVBean struct {
	TheString     string `esper:"theString"`
	IntBoxed      int    `esper:"intBoxed"`
	LongBoxed     int64  `esper:"longBoxed"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// infraNWBVBeanA mirrors SupportBean_A.
type infraNWBVBeanA struct {
	ID string `esper:"id"`
}

// infraNWBVBeanS0 mirrors SupportBean_S0.
type infraNWBVBeanS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// infraNWBVBeanAlias mirrors the ABC schema alias of SupportBean. The Go type
// to event-type index points one Go type at one event type, so the alias needs
// its own struct: reusing infraNWBVBean would re-point the index and route
// SupportBean sends to ABC.
type infraNWBVBeanAlias struct {
	TheString string `esper:"theString"`
	IntBoxed  int    `esper:"intBoxed"`
	LongBoxed int64  `esper:"longBoxed"`
}

// infraNWBVBeanContained mirrors the ord-35 window schema: one fragment column
// carrying a whole SupportBean_S0.
type infraNWBVBeanContained struct {
	Bean infraNWBVBeanS0 `esper:"bean"`
}

// infraNWBVOverrideBase mirrors the Esper-visible surface of
// SupportOverrideBase; infraNWBVOverrideOneA mirrors the subtype whose
// overridden getter supplies that property's value.
type infraNWBVOverrideBase struct {
	Val string `esper:"val"`
}

type infraNWBVOverrideOneA struct {
	Val string `esper:"val"`
}

func infraNWBVCaseSpecFor(name string) (infraNWBVCaseSpec, bool) {
	for _, spec := range infraNWBVCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWBVCaseSpec{}, false
}

// infraNWBVCaseNames returns the pinned case order used by the loader's
// ordering check.
func infraNWBVCaseNames() []string {
	names := make([]string, 0, len(infraNWBVCaseSpecs))
	for _, spec := range infraNWBVCaseSpecs {
		names = append(names, spec.name)
	}
	return names
}

// infraNWBVCaseDefinition is the decoded case-object of the scenario, shared by
// the loader's validation and its EPL-slot drift diagnostic.
type infraNWBVCaseDefinition struct {
	Case                 string   `json:"case"`
	Ordinal              int      `json:"ordinal"`
	RuntimeID            string   `json:"runtimeId"`
	ExecutionName        string   `json:"executionName"`
	Description          string   `json:"description"`
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
	IteratorSnapshots    int      `json:"iteratorSnapshots"`
}

func loadInfraNWBVScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWBVId)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWBVId, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWBVId, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWBVId, err)
	}
	if err := requireInfraNWBVFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWBVId, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWBVId ||
		metadata.Description != infraNWBVDescription ||
		metadata.JavaCommit != infraNWBVJavaCommit || metadata.JavaSource != infraNWBVSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWBVId)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWBVId)
	}
	if err := infraNWBVRequireEqual(metadata.JavaRuntimes, infraNWBVJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWBVRequireEqual(metadata.JavaNames, infraNWBVJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWBVId, err)
	}
	if len(rawCases) != len(infraNWBVCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWBVId, len(rawCases), len(infraNWBVCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWBVFields(object,
			"case", "ordinal", "runtimeId", "executionName", "description", "createEpl",
			"createEplObjectArray", "createEplMap", "createEplAvro", "insertEpl",
			"s0Epl", "s2Epl", "s3Epl", "consumeEpl", "deleteEpl", "varEpl", "onSetEpl",
			"schemaEpl", "updateEpl", "fafEpl", "negativeCompileEpl",
			"deploys", "listened", "iteratorSnapshots"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition infraNWBVCaseDefinition
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := infraNWBVCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
		}
		if definition.Description != spec.description {
			return compat.Scenario{}, fmt.Errorf("scenario case %q description does not match the pinned slice description", spec.name)
		}
		if drifted := infraNWBVEPLDrift(spec, definition); len(drifted) != 0 {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source (slots %s)",
				spec.name, strings.Join(drifted, ", "))
		}
		if err := infraNWBVRequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWBVRequireListened(definition.Listened, spec); err != nil {
			return compat.Scenario{}, err
		}
		if definition.IteratorSnapshots != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q iteratorSnapshots = %d, want %d",
				spec.name, definition.IteratorSnapshots, spec.snapshots)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWBVId, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWBVId)
	}
	deployCounts := map[string]int{}
	sendCounts := map[string]int{}
	advanceCounts := map[string]int{}
	fafCounts := map[string]int{}
	undeployAllCounts := map[string]int{}
	undeployStatements := map[string][]string{}
	caseOrder := make([]string, 0, len(infraNWBVCaseSpecs))
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
			var step struct {
				Case string `json:"case"`
				Mode string `json:"mode"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWBVCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d selects unknown case %q", index, step.Case)
			}
			allowed := []string{"op", "case"}
			if spec.caseMode != "" {
				allowed = append(allowed, "mode")
			}
			if err := requireInfraNWBVFields(object, allowed...); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Mode != spec.caseMode {
				return compat.Scenario{}, fmt.Errorf("scenario step %d case %q mode %q must be %q",
					index, step.Case, step.Mode, spec.caseMode)
			}
			currentCase = step.Case
			caseOrder = append(caseOrder, step.Case)
		case "deploy":
			if err := requireInfraNWBVFields(object, "op", "case", "statement", "epl"); err != nil {
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
			spec, ok := infraNWBVCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			if err := infraNWBVRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
			if deployCounts[step.Case] >= len(spec.deploySequence) {
				return compat.Scenario{}, fmt.Errorf("scenario case %q has more deploy steps than the pinned sequence", spec.name)
			}
			if want := spec.deploySequence[deployCounts[step.Case]]; step.Statement != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys statement %q at position %d of case %q, want %q",
					index, step.Statement, deployCounts[step.Case], step.Case, want)
			}
			want, ok := infraNWBVEPLForStep(spec, step.Statement, deployCounts[step.Case])
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWBVFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := infraNWBVCaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d sends for unknown case %q", index, step.Case)
			}
			if err := infraNWBVRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
			sendCounts[step.Case]++
		case "snapshot":
			if err := requireInfraNWBVFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
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
			spec, ok := infraNWBVCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshots unknown case %q", index, step.Case)
			}
			if err := infraNWBVRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
			if snapshotCounts[step.Case] >= len(spec.snapStatements) {
				return compat.Scenario{}, fmt.Errorf("scenario case %q has more snapshot steps than the pinned sites", spec.name)
			}
			if step.Statement != spec.snapStatements[snapshotCounts[step.Case]] {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot statement %q must be %q for case %q",
					index, step.Statement, spec.snapStatements[snapshotCounts[step.Case]], step.Case)
			}
			if step.Mode != spec.snapModes[snapshotCounts[step.Case]] {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot mode %q must be %q for case %q",
					index, step.Mode, spec.snapModes[snapshotCounts[step.Case]], step.Case)
			}
			if err := infraNWBVRequireEqual(step.Fields, spec.snapFields[snapshotCounts[step.Case]], step.Case+" snapshot fields"); err != nil {
				return compat.Scenario{}, err
			}
			snapshotModes[step.Case] = append(snapshotModes[step.Case], step.Mode)
			snapshotStatements[step.Case] = append(snapshotStatements[step.Case], step.Statement)
			snapshotFields[step.Case] = append(snapshotFields[step.Case], step.Fields)
			snapshotCounts[step.Case]++
		case "faf":
			if err := requireInfraNWBVFields(object, "op", "case", "statement", "epl", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string   `json:"case"`
				Statement string   `json:"statement"`
				EPL       string   `json:"epl"`
				Fields    []string `json:"fields"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWBVCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d runs a faf query for unknown case %q", index, step.Case)
			}
			if err := infraNWBVRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
			if spec.fafEPL == "" {
				return compat.Scenario{}, fmt.Errorf("scenario case %q has no pinned faf query", spec.name)
			}
			if step.Statement != "faf" || step.EPL != spec.fafEPL {
				return compat.Scenario{}, fmt.Errorf("scenario step %d faf query does not match the pinned text for case %q", index, step.Case)
			}
			if err := infraNWBVRequireEqual(step.Fields, spec.fafFields, step.Case+" faf fields"); err != nil {
				return compat.Scenario{}, err
			}
			fafCounts[step.Case]++
		case "undeploy":
			if err := requireInfraNWBVFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWBVCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d undeploys unknown case %q", index, step.Case)
			}
			if err := infraNWBVRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
			if !infraNWBVContains(spec.deploys, step.Statement) {
				return compat.Scenario{}, fmt.Errorf("scenario step %d undeploys statement %q which case %q never deploys",
					index, step.Statement, step.Case)
			}
			undeployStatements[step.Case] = append(undeployStatements[step.Case], step.Statement)
		case "advance-time":
			if err := requireInfraNWBVFields(object, "op", "case", "at"); err != nil {
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
			if _, ok := infraNWBVCaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advances unknown case %q", index, step.Case)
			}
			if err := infraNWBVRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
			advanceCounts[step.Case]++
		case "undeploy-all":
			if err := requireInfraNWBVFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := infraNWBVCaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d undeploys all for unknown case %q", index, step.Case)
			}
			if err := infraNWBVRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
			undeployAllCounts[step.Case]++
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	for index, spec := range infraNWBVCaseSpecs {
		if index >= len(caseOrder) || caseOrder[index] != spec.name {
			return compat.Scenario{}, fmt.Errorf("scenario case order = %v, want %v", caseOrder, infraNWBVCaseNames())
		}
		if sendCounts[spec.name] != spec.sends {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d send steps, want %d",
				spec.name, sendCounts[spec.name], spec.sends)
		}
		if advanceCounts[spec.name] != spec.advances {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d advance-time steps, want %d",
				spec.name, advanceCounts[spec.name], spec.advances)
		}
		if deployCounts[spec.name] != len(spec.deploySequence) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d deploy steps, want %d",
				spec.name, deployCounts[spec.name], len(spec.deploySequence))
		}
		if undeployAllCounts[spec.name] != spec.undeployAlls {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d undeploy-all steps, want %d",
				spec.name, undeployAllCounts[spec.name], spec.undeployAlls)
		}
		if fafCounts[spec.name] != spec.fafs {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d faf steps, want %d",
				spec.name, fafCounts[spec.name], spec.fafs)
		}
		if snapshotCounts[spec.name] != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d snapshot steps, want %d",
				spec.name, snapshotCounts[spec.name], spec.snapshots)
		}
		if err := infraNWBVRequireEqual(undeployStatements[spec.name], spec.undeployStatements, spec.name+" undeploy statements"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWBVRequireEqual(snapshotModes[spec.name], spec.snapModes, spec.name+" snapshot modes"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWBVRequireEqual(snapshotStatements[spec.name], spec.snapStatements, spec.name+" snapshot statements"); err != nil {
			return compat.Scenario{}, err
		}
		for index := range spec.snapFields {
			if err := infraNWBVRequireEqual(snapshotFields[spec.name][index], spec.snapFields[index],
				fmt.Sprintf("%s snapshot %d fields", spec.name, index)); err != nil {
				return compat.Scenario{}, err
			}
		}
	}
	var steps []compat.Step
	if err := json.Unmarshal(root["steps"], &steps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", infraNWBVId, err)
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWBVEPLForStep returns the byte-exact EPL of one deploy step at its
// position in the case's deploy sequence: a create step carries the cycle's
// representation variant, every other statement its pinned slot text.
func infraNWBVEPLForStep(spec infraNWBVCaseSpec, statement string, position int) (string, bool) {
	if statement == "create" {
		cycle := 0
		for index := 0; index < position && index < len(spec.deploySequence); index++ {
			if spec.deploySequence[index] == "create" {
				cycle++
			}
		}
		if cycle >= len(spec.createVariants) {
			return "", false
		}
		return spec.createVariants[cycle], true
	}
	switch statement {
	case "insert":
		return spec.insertEPL, spec.insertEPL != ""
	case "s0":
		return spec.s0EPL, spec.s0EPL != ""
	case "s2":
		return spec.s2EPL, spec.s2EPL != ""
	case "s3":
		return spec.s3EPL, spec.s3EPL != ""
	case "delete":
		return spec.deleteEPL, spec.deleteEPL != ""
	case "var":
		return spec.varEPL, spec.varEPL != ""
	case "on-set":
		return spec.onSetEPL, spec.onSetEPL != ""
	case "schema":
		return spec.schemaEPL, spec.schemaEPL != ""
	case "update":
		return spec.updateEPL, spec.updateEPL != ""
	default:
		return "", false
	}
}

// infraNWBVEPLDrift names the case-definition EPL slots that differ from the
// pinned Java statement text.
func infraNWBVEPLDrift(spec infraNWBVCaseSpec, definition infraNWBVCaseDefinition) []string {
	var drifted []string
	slots := []struct {
		name string
		got  string
		want string
	}{
		{"createEpl", definition.CreateEPL, spec.createEPL},
		{"createEplObjectArray", definition.CreateObjectArrayEPL, spec.createObjArrEPL},
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
			drifted = append(drifted, slot.name)
		}
	}
	return drifted
}

func infraNWBVRequireEqual(got, want []string, label string) error {
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

func infraNWBVRequireListened(got []string, spec infraNWBVCaseSpec) error {
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

// infraNWBVRequireInBlock rejects a case-scoped step that appears outside its
// own case block.
func infraNWBVRequireInBlock(index int, stepCase, currentCase string) error {
	if stepCase != currentCase {
		return fmt.Errorf("scenario step %d targets case %q outside its block (current case %q)",
			index, stepCase, currentCase)
	}
	return nil
}

func infraNWBVContains(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

func requireInfraNWBVFields(object map[string]json.RawMessage, names ...string) error {
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

func runInfraNWBVScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWBVId)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWBVCaseSpecs {
		caseTrace, err := runInfraNWBVCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWBVId, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

// infraNWBVCaseState holds the per-cycle environment: the Java suite's
// representation matrix runs the same deploy/fill/undeploy cycle several times
// against a fresh runtime, so each cycle gets a fresh environment, schema
// registration, window and engine.
type infraNWBVCaseState struct {
	env         *esper.Environment
	engine      *esper.Engine
	deployments map[string]*esper.Deployment
	statements  map[string]*esper.Statement
}

func (s *infraNWBVCaseState) close() {
	if s == nil || s.engine == nil {
		return
	}
	_ = s.engine.Close(context.Background())
	s.engine = nil
}

// infraNWBVStartCycle builds the environment of one cycle.
func infraNWBVStartCycle(ctx context.Context, spec infraNWBVCaseSpec, now time.Time) (*infraNWBVCaseState, error) {
	env := esper.NewEnvironment()
	var windowSchema esper.Schema
	var err error
	switch spec.shape {
	case infraNWBVBeanBackedRows:
		if _, err := esper.RegisterStruct[infraNWBVBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[infraNWBVBeanA](env, "SupportBean_A"); err != nil {
			return nil, err
		}
		// The window is declared over the SupportBean bean class, so it
		// exposes the SupportBean property set rather than a new type.
		found := false
		if windowSchema, found = env.Schema("SupportBean"); !found {
			return nil, fmt.Errorf("case %q: SupportBean schema is missing", spec.name)
		}
	case infraNWBVBeanContainedRows:
		if _, err := esper.RegisterStruct[infraNWBVBeanS0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
		if windowSchema, err = esper.RegisterStruct[infraNWBVBeanContained](env, spec.windowName); err != nil {
			return nil, err
		}
	case infraNWBVBeanSchemaAliasRows:
		if _, err := esper.RegisterStruct[infraNWBVBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		// The create-schema deployment declares the ABC alias type; the Go
		// port registers the distinct alias schema at environment level so
		// the window can be declared over it.
		if windowSchema, err = esper.RegisterStruct[infraNWBVBeanAlias](env, "ABC"); err != nil {
			return nil, err
		}
	case infraNWBVDeepSupertypeRows:
		if _, err := esper.RegisterStruct[infraNWBVOverrideBase](env, "SupportOverrideBase"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[infraNWBVOverrideOneA](env, "SupportOverrideOneA"); err != nil {
			return nil, err
		}
		if windowSchema, err = esper.RegisterStruct[infraNWBVOverrideBase](env, spec.windowName); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unexpected window shape %d for case %q", spec.shape, spec.name)
	}
	if err != nil {
		return nil, err
	}
	if _, err := esper.CreateNamedWindow(env, spec.windowName, windowSchema,
		esper.NamedWindowRetention(esper.KeepAll())); err != nil {
		return nil, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(spec.runtimeID),
		esper.WithStartTime(now),
	)
	return &infraNWBVCaseState{
		env:         env,
		engine:      engine,
		deployments: map[string]*esper.Deployment{},
		statements:  map[string]*esper.Statement{},
	}, nil
}

func runInfraNWBVCase(ctx context.Context, scenario compat.Scenario, spec infraNWBVCaseSpec) ([]compat.TraceRecord, error) {
	// Virtual clock: starts at the epoch and moves only on advance-time steps,
	// so every record carries the clock value at delivery.
	now := time.Unix(0, 0).UTC()
	sequence := map[string]uint64{}
	snapshotCount := 0
	var records []compat.TraceRecord
	var state *infraNWBVCaseState
	defer func() { state.close() }()
	recordListener := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		records = append(records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(now),
			New:       infraNWBVProject(batch.New, spec.rowFields[statement], spec.typeTags),
			Old:       infraNWBVProject(batch.Old, spec.rowFields[statement], spec.typeTags),
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
			if state == nil {
				started, err := infraNWBVStartCycle(ctx, spec, now)
				if err != nil {
					return nil, err
				}
				state = started
			}
			if step.Statement == "schema" {
				// "create schema" carries no statement in the Go chain: the
				// alias type is the environment-level schema registration.
				if spec.schemaEPL == "" {
					return nil, fmt.Errorf("case %q does not declare a schema", spec.name)
				}
				continue
			}
			plan, err := infraNWBVBuildPlan(state.env, spec, step.Statement)
			if err != nil {
				return nil, err
			}
			deployment, err := state.engine.Deploy(ctx, plan)
			if err != nil {
				return nil, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			for _, statement := range deployment.Statements() {
				if statement.Name() == step.Statement {
					state.statements[step.Statement] = statement
					state.deployments[step.Statement] = deployment
				}
			}
			if spec.listened[step.Statement] {
				statement, ok := state.statements[step.Statement]
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
			if state == nil {
				return nil, fmt.Errorf("case %q sends before its first deploy", spec.name)
			}
			payload, err := infraNWBVDecodePayload(spec, step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "snapshot":
			if state == nil {
				return nil, fmt.Errorf("case %q snapshots before its first deploy", spec.name)
			}
			statement, ok := state.statements[step.Statement]
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
			rows := infraNWBVProject(result.Batch.New, spec.snapFields[snapshotCount], spec.typeTags)
			snapshotCount++
			// A count-only iterator assertion pins no fields; both sides emit
			// empty rows and mark the step "any" so the comparison is a count
			// comparison.
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
		case "faf":
			if state == nil {
				return nil, fmt.Errorf("case %q runs a faf query before its first deploy", spec.name)
			}
			plan, err := infraNWBVBuildFAF(state.env, spec)
			if err != nil {
				return nil, err
			}
			result, err := state.engine.ExecuteFireAndForget(ctx, plan)
			if err != nil {
				return nil, err
			}
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "faf",
				Statement: "faf",
				Sequence:  0,
				Time:      compat.FormatTraceTime(now),
				New:       infraNWBVProject(result.Batch.New, spec.fafFields, spec.typeTags),
			})
		case "undeploy":
			if state == nil {
				return nil, fmt.Errorf("case %q undeploys before its first deploy", spec.name)
			}
			deployment, ok := state.deployments[step.Statement]
			if !ok {
				return nil, fmt.Errorf("no deployment for statement %q", step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return nil, err
			}
			delete(state.deployments, step.Statement)
			delete(state.statements, step.Statement)
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("advance-time %q: %w", step.At, err)
			}
			// The clock moves first: expiry deliveries produced by the advance
			// carry the boundary time, matching the Java trace.
			now = at.UTC()
			if state != nil {
				if err := state.engine.AdvanceTime(ctx, now); err != nil {
					return nil, err
				}
			}
		case "undeploy-all":
			// The Java suite tears the deployment down at this point: the
			// window and its contents are dropped, and the next cycle starts
			// from a fresh environment. Teardown emits no trace records.
			state.close()
			state = nil
		default:
			return nil, fmt.Errorf("unsupported step op %q", step.Op)
		}
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("case %q produced no records", spec.name)
	}
	return records, nil
}

// normalizeInfraNWBVTrace reorders one engine-internal publication pair per
// bean-backed cycle. Java's view topology dispatches a window update as window
// statement, trigger statement, then the window's consumer view, while the Go
// engine publishes the on-trigger statement's own result after the consumer's
// delta of the same wave. The swap is pinned to the exact pair — a bean-backed
// consumer record whose sequence is twice the following trigger record's
// sequence — so row content, sequences, streams, counts and every other
// ordering difference stay compared strictly.
func normalizeInfraNWBVTrace(trace compat.Trace) compat.Trace {
	for index := 1; index < len(trace.Records); index++ {
		previous := trace.Records[index-1]
		current := trace.Records[index]
		if previous.Case != "bean-backed" || current.Case != "bean-backed" ||
			previous.Operation != "listener" || current.Operation != "listener" ||
			previous.Statement != "s0" || current.Statement != "update" ||
			previous.Sequence != 2*current.Sequence {
			continue
		}
		trace.Records[index-1], trace.Records[index] = current, previous
	}
	return trace
}

// infraNWBVProject renders a result batch as protocol rows, projecting the
// pinned fields and optionally tagging each row with the delivered event's
// type name (the language-neutral half of Java's assertEvent metadata checks).
func infraNWBVProject(results []esper.Result, fields []string, tagType bool) []compat.ResultRecord {
	records := compat.NormalizeResults(results)
	if tagType {
		for index := range records {
			if index >= len(results) {
				break
			}
			if event, ok := results[index].Event(); ok {
				records[index].Type = event.TypeName()
			}
		}
	}
	projected := make([]compat.ResultRecord, len(records))
	for index, record := range records {
		fieldsMap := make(map[string]any, len(fields))
		for _, name := range fields {
			fieldsMap[name] = infraNWBVResolveField(record, name)
		}
		projected[index] = compat.ResultRecord{Kind: record.Kind, Type: record.Type, Fields: fieldsMap}
	}
	return projected
}

// infraNWBVResolveField resolves a (possibly dotted) property path against a
// normalized row, unwrapping nested fragment rows on the way. A nested POJO
// column normalizes as the bare Go struct, so the path segment is matched
// against the struct's esper tags.
func infraNWBVResolveField(record compat.ResultRecord, path string) any {
	var current any = record.Fields
	for _, part := range strings.Split(path, ".") {
		mapping, ok := current.(map[string]any)
		if !ok {
			resolved, found := infraNWBVStructField(current, part)
			if !found {
				return nil
			}
			current = resolved
			continue
		}
		next, ok := mapping[part]
		if !ok {
			resolved, found := infraNWBVStructField(current, part)
			if !found {
				return nil
			}
			current = resolved
			continue
		}
		current = next
		if wrapper, ok := current.(map[string]any); ok {
			if kind, _ := wrapper["kind"].(string); kind == "row" {
				if nested, ok := wrapper["fields"].(map[string]any); ok {
					current = nested
				}
			}
		}
	}
	return current
}

// infraNWBVStructField reads one property of a nested POJO value by its
// Esper name.
func infraNWBVStructField(value any, name string) (any, bool) {
	typed := reflect.ValueOf(value)
	if typed.Kind() != reflect.Struct {
		return nil, false
	}
	structType := typed.Type()
	for index := range structType.NumField() {
		field := structType.Field(index)
		tag := field.Tag.Get("esper")
		if tag == "" {
			tag = field.Name
		}
		if tag == name {
			return typed.Field(index).Interface(), true
		}
	}
	return nil, false
}

// infraNWBVBuildPlan maps one scenario deploy statement onto the typed chain.
func infraNWBVBuildPlan(env *esper.Environment, spec infraNWBVCaseSpec, statement string) (esper.Plan, error) {
	switch statement {
	case "create":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		return infraNWBVBuildInsert(env, spec)
	case "s0":
		return infraNWBVBuildConsumer(env, spec)
	case "update":
		return env.Build(esper.OnEvent(esper.From[infraNWBVBeanA](env, "SupportBean_A")).
			UpdateNamedWindow(spec.windowName,
				esper.Literal(true),
				esper.SetColumn("theString", esper.Literal("s"))).
			Query(esper.StatementName("update")))
	default:
		return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
	}
}

// infraNWBVBuildInsert maps the case's insert-into statement onto the typed
// chain; a cross-type insert writes the target window's declared columns.
func infraNWBVBuildInsert(env *esper.Environment, spec infraNWBVCaseSpec) (esper.Plan, error) {
	switch spec.shape {
	case infraNWBVBeanBackedRows, infraNWBVBeanSchemaAliasRows:
		return env.Build(esper.OnEvent(esper.From[infraNWBVBean](env, "SupportBean")).
			InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("theString", esper.Field[infraNWBVBean, string]("theString")),
				esper.SetColumn("intBoxed", esper.Field[infraNWBVBean, int]("intBoxed")),
				esper.SetColumn("longBoxed", esper.Field[infraNWBVBean, int64]("longBoxed"))).
			Query(esper.StatementName("insert")))
	case infraNWBVBeanContainedRows:
		return env.Build(esper.OnEvent(esper.From[infraNWBVBeanS0](env, "SupportBean_S0")).
			InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("bean", esper.EventValue[infraNWBVBeanS0]())).
			Query(esper.StatementName("insert")))
	case infraNWBVDeepSupertypeRows:
		return env.Build(esper.OnEvent(esper.From[infraNWBVOverrideOneA](env, "SupportOverrideOneA")).
			InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("val", esper.Field[infraNWBVOverrideOneA, string]("val"))).
			Query(esper.StatementName("insert")))
	default:
		return esper.Plan{}, fmt.Errorf("case %q: unexpected insert for window shape %d", spec.name, spec.shape)
	}
}

// infraNWBVBuildConsumer maps the case's consumer statement onto the typed
// chain: a plain read of the window, or a plain read of the alias type.
func infraNWBVBuildConsumer(env *esper.Environment, spec infraNWBVCaseSpec) (esper.Plan, error) {
	switch spec.shape {
	case infraNWBVBeanBackedRows:
		return env.Build(esper.FromNamedWindow(env, spec.windowName).Query(esper.StatementName("s0")))
	case infraNWBVBeanSchemaAliasRows:
		return env.Build(esper.From[infraNWBVBeanAlias](env, "ABC").Query(esper.StatementName("s0")))
	default:
		return esper.Plan{}, fmt.Errorf("case %q: unexpected consumer for window shape %d", spec.name, spec.shape)
	}
}

// infraNWBVBuildFAF maps the case's fire-and-forget query onto the typed chain.
func infraNWBVBuildFAF(env *esper.Environment, spec infraNWBVCaseSpec) (esper.Plan, error) {
	if spec.fafEPL == "" {
		return esper.Plan{}, fmt.Errorf("case %q has no pinned faf query", spec.name)
	}
	return env.Build(esper.FromNamedWindow(env, spec.windowName).Query(esper.StatementName("faf")))
}

func infraNWBVDecodePayload(spec infraNWBVCaseSpec, step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	pinned, ok := spec.sendFields[step.EventType]
	if !ok {
		return nil, fmt.Errorf("case %q does not send %q events", spec.name, step.EventType)
	}
	if err := requireInfraNWBVFields(fields, pinned...); err != nil {
		return nil, fmt.Errorf("case %q %s payload: %w", spec.name, step.EventType, err)
	}
	switch step.EventType {
	case "SupportBean":
		var bean infraNWBVBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_A":
		var bean infraNWBVBeanA
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		var bean infraNWBVBeanS0
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return bean, nil
	case "SupportOverrideOneA":
		// The scenario keeps the Java constructor's three hierarchy values
		// (SupportOverrideOneA("1a","1","base")). Esper exposes only the
		// overridden getter getVal(), so the window's `val` column carries the
		// most-derived value; the base and middle values are validated but
		// deliberately not exposed, which keeps a port that read the base
		// field instead of the override visibly wrong in the trace.
		var sent struct {
			ValOneA string `json:"valOneA"`
			ValOne  string `json:"valOne"`
			ValBase string `json:"val"`
		}
		if err := json.Unmarshal(step.Payload, &sent); err != nil {
			return nil, fmt.Errorf("decode SupportOverrideOneA: %w", err)
		}
		return infraNWBVOverrideOneA{Val: sent.ValOneA}, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
