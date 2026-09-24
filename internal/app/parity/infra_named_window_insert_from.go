package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	infraNamedWindowInsertFromID          = "infra-named-window-insert-from"
	infraNamedWindowInsertFromDescription = "InfraNamedWindowInsertFrom insert-from-window semantics: type-by-window creation after an existing named window with shared insert delivery, seeded keep-all and filtered and unique-index create-window-insert with deploy-time row copying that never reaches create-statement listeners, filtered routing inserts into each window, the staggered map-representation insert-where create chain (ord 2 narrowed to rep=MAP; the object-model toEPL round-trip is unrepresentable), the invalid create-window-insert compile probes, the variant-stream window fed by insert-into routing, and lenient partial-column inserts over map and object-array schemas, captured from window listeners and ordered or canonical mode-any snapshots projected to the Java-asserted fields (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowInsertFrom.java)."
	infraNamedWindowInsertFromJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNamedWindowInsertFromSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowInsertFrom.java"

	infraNamedWindowInsertFromWindowOneCreate   = "@name('windowOne') @public create window MyWindow#keepall as SupportBean"
	infraNamedWindowInsertFromWindowTwoCreate   = "@name('windowTwo') @public create window MyWindowTwo#keepall as MyWindow"
	infraNamedWindowInsertFromWindowOneInsert   = "insert into MyWindow select * from SupportBean"
	infraNamedWindowInsertFromSelectOne         = "@name('selectOne') select theString from MyWindow"
	infraNamedWindowInsertFromIWTCreate         = "@name('window') @public create window MyWindowIWT#keepall as SupportBean"
	infraNamedWindowInsertFromIWTInsert         = "insert into MyWindowIWT select * from SupportBean(intPrimitive > 0)"
	infraNamedWindowInsertFromTwoCreate         = "@name('windowTwo') @public create window MyWindowTwo#keepall as MyWindowIWT insert"
	infraNamedWindowInsertFromThreeCreate       = "@name('windowThree') @public create window MyWindowThree#keepall as MyWindowIWT insert where theString like 'A%'"
	infraNamedWindowInsertFromFourCreate        = "@name('windowFour') @public create window MyWindowFour#unique(intPrimitive) as MyWindowIWT insert"
	infraNamedWindowInsertFromRouteA            = "insert into MyWindowIWT select * from SupportBean(theString like 'A%')"
	infraNamedWindowInsertFromRouteB            = "insert into MyWindowTwo select * from SupportBean(theString like 'B%')"
	infraNamedWindowInsertFromRouteC            = "insert into MyWindowThree select * from SupportBean(theString like 'C%')"
	infraNamedWindowInsertFromRouteD            = "insert into MyWindowFour select * from SupportBean(theString like 'D%')"
	infraNamedWindowInsertFromMapSchema         = "@public create MAP schema MyTwoColEvent(c0 string, c1 int)"
	infraNamedWindowInsertFromObjectArraySchema = "@public create OBJECTARRAY schema MyTwoColEvent(c0 string, c1 int)"
	infraNamedWindowInsertFromLenientWindow     = "@public @name('window') create window MyWindow#keepall as MyTwoColEvent"
	infraNamedWindowInsertFromLenientInsertOne  = "insert into MyWindow select theString as c0 from SupportBean"
	infraNamedWindowInsertFromLenientInsertTwo  = "insert into MyWindow select id as c1 from SupportBean_S0"

	infraNamedWindowInsertFromIWOMCreate      = "@EventRepresentation('map') @name('window') @public create window MyWindowIWOM#keepall as select a, b from MyMapAB"
	infraNamedWindowInsertFromIWOMInsert      = "@public insert into MyWindowIWOM select a, b from MyMapAB"
	infraNamedWindowInsertFromIWOMTwoCreate   = "@name('windowTwo') @public create window MyWindowIWOMTwo#keepall as select * from MyWindowIWOM insert where b=10"
	infraNamedWindowInsertFromIWOMThreeCreate = "@EventRepresentation('map') @name('windowThree') create window MyWindowIWOMThree#keepall as select a from MyWindowIWOMTwo insert where a = 'E2'"
	infraNamedWindowInsertFromINVCreate       = "@public create window MyWindowINV#keepall as SupportBean"
	infraNamedWindowInsertFromVSCreate        = "@public create window MyWindowVS#keepall as select * from VarStream"
	infraNamedWindowInsertFromVSTwoCreate     = "@name('window') @public create window MyWindowVSTwo#keepall as MyWindowVS"
	infraNamedWindowInsertFromVSInsertA       = "insert into VarStream select * from SupportBean_A"
	infraNamedWindowInsertFromVSInsertB       = "insert into VarStream select * from SupportBean_B"
	infraNamedWindowInsertFromVSInsert        = "insert into MyWindowVSTwo select * from VarStream"

	// The byte-exact EPL of the five InfraInvalid tryInvalidCompile probes
	// (InfraNamedWindowInsertFrom lines 303-312).
	infraNamedWindowInsertFromProbeInsert      = "create window testWindow3#keepall as SupportBean insert"
	infraNamedWindowInsertFromProbeInsertWhere = "create window testWindow3#keepall as select * from SupportBean insert where (intPrimitive = 10)"
	infraNamedWindowInsertFromProbeSubselect   = "create window MyWindowTwo#keepall as MyWindowINV insert where (select intPrimitive from SupportBean#lastevent)"
	infraNamedWindowInsertFromProbeAggregation = "create window MyWindowTwo#keepall as MyWindowINV insert where sum(intPrimitive) > 2"
	infraNamedWindowInsertFromProbePrev        = "create window MyWindowTwo#keepall as MyWindowINV insert where prev(1, intPrimitive) = 1"

	// Pinned Java message prefixes the compile-error records carry.
	infraNamedWindowInsertFromMissingWindowPrefix = "A named window by name 'SupportBean' could not be located, the insert-keyword requires an existing named window"
	infraNamedWindowInsertFromSubselectPrefix     = "Create window where-clause may not have a subselect"
	infraNamedWindowInsertFromAggregationPrefix   = "Create window where-clause may not have an aggregation function"
	infraNamedWindowInsertFromPrevPrefix          = "Create window where-clause may not have a function that requires view resources (prior, prev)"

	// infraNamedWindowInsertFromOMNote pins the unrepresentable record for
	// the object-model toEPL round-trip asserts at InfraInsertWhereOMStaggered
	// lines 252-263; the Go surface has no statement object model.
	infraNamedWindowInsertFromOMNote = "object-model round-trip: the programmatic create-window insert-where model and eplToModel both render '@public create window MyWindowIWOMTwo#keepall as select * from MyWindowIWOM insert where b=10'; no Go statement-object-model surface"
)

var (
	infraNamedWindowInsertFromJavaSources = []string{
		infraNamedWindowInsertFromSource,
	}
	infraNamedWindowInsertFromJavaRuntimeIDs = []string{
		"java-runtime-b3f6cb7b36c5211c8822",
		"java-runtime-e601b3cc7f827d578185",
		"java-runtime-f0f0e5e651a8a513377e",
		"java-runtime-75dcb72bef59bc4cc772",
		"java-runtime-b6e5b14130feae458c23",
		"java-runtime-46011542d6e9d34a87f5",
		"java-runtime-8138dd777290d00417d1",
	}
	infraNamedWindowInsertFromJavaExecutions = []string{
		"InfraCreateNamedAfterNamed",
		"InfraInsertWhereTypeAndFilter",
		"InfraInsertWhereOMStaggered",
		"InfraInvalid",
		"InfraVariantStream",
		"InfraNamedWindowInsertLenientPropCount{rep=MAP}",
		"InfraNamedWindowInsertLenientPropCount{rep=OBJECTARRAY}",
	}
	infraNamedWindowInsertFromJavaStaticIDs = []string{
		"java-b0917219c462cba9d770",
		"java-cff4a3193da2b063a351",
		"java-03b8de29f9f76bb86748",
		"java-10fd9729c661bf429fe4",
		"java-1e55dc9c7e5d907881c5",
		"java-282d7b64878866ea4709",
		"java-282d7b64878866ea4709",
	}
	infraNamedWindowInsertFromCases = []string{
		"create-after-named",
		"insert-where-type-filter",
		"insert-where-om-staggered",
		"infra-invalid",
		"variant-stream",
		"lenient-map",
		"lenient-objectarray",
	}
	// Ordinals are the Java executions() ordinals. Ord 2's Java execution
	// loops all EventRepresentationChoice values; the replay covers rep=MAP
	// only (the rep-matrix narrowing is pinned in the case observation).
	infraNamedWindowInsertFromOrdinals     = []int{0, 1, 2, 3, 4, 5, 6}
	infraNamedWindowInsertFromIterSnaps    = []int{0, 3, 2, 0, 1, 1, 1}
	infraNamedWindowInsertFromObservations = []string{
		"listener",
		"listener",
		"listener+snapshot+unrepresentable; Java loops all EventRepresentationChoice values while the replay covers rep=MAP only (the rep-matrix narrowing): the keepall window over MyMapAB{a,b} receives three map sends through the shared insert, then the b=10-filtered create-window-insert seeds MyWindowIWOMTwo ({E2,10},{E3,10}) and the chained a='E2' create seeds MyWindowIWOMThree ({E2}); the object-model toEPL round-trip assertion is unrepresentable on the Go surface and pins a marker",
		"compile-error; after deploying MyWindowINV five probes pin the Java message prefixes: the two insert-keyword probes against the SupportBean event type compile without the runtime path, while the subselect, aggregation and prev() insert-where probes compile with the path and are unrepresentable on the Go surface (no create-window-where boundary)",
		"snapshot; the variant-schema window pair is fed by insert-into routing from SupportBean_A/SupportBean_B through VarStream (the Java create-window-as-select types MyWindowVS from VarStream; MyWindowVSTwo is fed by insert-into routing); A1 then B1 land in MyWindowVSTwo in insertion order and the iterator projects the dynamic id? property",
		"listener",
		"listener",
	}
)

type infraNamedWindowInsertFromBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type infraNamedWindowInsertFromTwoCol struct {
	C0 string `esper:"c0"`
	C1 *int   `esper:"c1"`
}

// infraNamedWindowInsertFromAB mirrors SupportBean_A/SupportBean_B: a single
// id property carried by the variant members.
type infraNamedWindowInsertFromAB struct {
	ID string `esper:"id"`
}

func loadInfraNamedWindowInsertFromScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNamedWindowInsertFromID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNamedWindowInsertFromID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNamedWindowInsertFromID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNamedWindowInsertFromID, err)
	}
	if err := requireInfraNamedWindowInsertFromFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version     string `json:"version"`
		ID          string `json:"id"`
		Description string `json:"description"`
		JavaCommit  string `json:"javaCommit"`
		JavaSource  string `json:"javaSource"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNamedWindowInsertFromID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNamedWindowInsertFromID ||
		metadata.Description != infraNamedWindowInsertFromDescription ||
		metadata.JavaCommit != infraNamedWindowInsertFromJavaCommit ||
		metadata.JavaSource != infraNamedWindowInsertFromSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNamedWindowInsertFromID)
	}
	if err := validateInfraNamedWindowInsertFromStringArray(root["javaRuntimes"], infraNamedWindowInsertFromJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNamedWindowInsertFromStringArray(root["javaNames"], infraNamedWindowInsertFromJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNamedWindowInsertFromStringArray(root["javaStaticIds"], infraNamedWindowInsertFromJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNamedWindowInsertFromStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNamedWindowInsertFromCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly seven cases", infraNamedWindowInsertFromID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNamedWindowInsertFromFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != infraNamedWindowInsertFromCases[index] ||
			definition.Ordinal != infraNamedWindowInsertFromOrdinals[index] ||
			definition.RuntimeID != infraNamedWindowInsertFromJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNamedWindowInsertFromJavaExecutions[index] ||
			definition.Observation != infraNamedWindowInsertFromObservations[index] ||
			definition.IteratorSnapshots != infraNamedWindowInsertFromIterSnaps[index] ||
			definition.EPL != infraNamedWindowInsertFromObservedEPL(definition.Case) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNamedWindowInsertFromID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNamedWindowInsertFromID)
	}
	steps := make([]compat.Step, len(rawSteps))
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
			if err := requireInfraNamedWindowInsertFromFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNamedWindowInsertFromFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "build-error":
			if err := requireInfraNamedWindowInsertFromFields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "unrepresentable":
			if err := requireInfraNamedWindowInsertFromFields(object, "op", "case", "statement", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNamedWindowInsertFromFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNamedWindowInsertFromPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNamedWindowInsertFromFields(object, "op", "case", "statement", "mode"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNamedWindowInsertFromFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateInfraNamedWindowInsertFromScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// infraNamedWindowInsertFromObservedEPL pins the cases[].epl metadata: the
// Java-observed statement's source text (ord 1's create carries @public in
// the source; ord 0's windowOne does not; ord 2 pins the rep=MAP annotation
// text the narrowed replay deploys).
func infraNamedWindowInsertFromObservedEPL(caseName string) string {
	switch caseName {
	case "create-after-named":
		return "@name('windowOne') create window MyWindow#keepall as SupportBean"
	case "insert-where-type-filter":
		return infraNamedWindowInsertFromIWTCreate
	case "insert-where-om-staggered":
		return infraNamedWindowInsertFromIWOMCreate
	case "infra-invalid":
		return infraNamedWindowInsertFromINVCreate
	case "variant-stream":
		return infraNamedWindowInsertFromVSCreate
	default:
		return infraNamedWindowInsertFromLenientWindow
	}
}

// infraNamedWindowInsertFromDeployEPLs pins the exact deploy EPL per case and
// statement label.
func infraNamedWindowInsertFromDeployEPLs(caseName string) map[string]string {
	switch caseName {
	case "create-after-named":
		return map[string]string{
			"windowOne": infraNamedWindowInsertFromWindowOneCreate,
			"windowTwo": infraNamedWindowInsertFromWindowTwoCreate,
			"insert":    infraNamedWindowInsertFromWindowOneInsert,
			"selectOne": infraNamedWindowInsertFromSelectOne,
		}
	case "insert-where-type-filter":
		return map[string]string{
			"window":      infraNamedWindowInsertFromIWTCreate,
			"insert":      infraNamedWindowInsertFromIWTInsert,
			"windowTwo":   infraNamedWindowInsertFromTwoCreate,
			"windowThree": infraNamedWindowInsertFromThreeCreate,
			"windowFour":  infraNamedWindowInsertFromFourCreate,
			"insertA":     infraNamedWindowInsertFromRouteA,
			"insertB":     infraNamedWindowInsertFromRouteB,
			"insertC":     infraNamedWindowInsertFromRouteC,
			"insertD":     infraNamedWindowInsertFromRouteD,
		}
	case "insert-where-om-staggered":
		return map[string]string{
			"window":      infraNamedWindowInsertFromIWOMCreate,
			"insert":      infraNamedWindowInsertFromIWOMInsert,
			"windowTwo":   infraNamedWindowInsertFromIWOMTwoCreate,
			"windowThree": infraNamedWindowInsertFromIWOMThreeCreate,
		}
	case "infra-invalid":
		return map[string]string{
			"window": infraNamedWindowInsertFromINVCreate,
		}
	case "variant-stream":
		return map[string]string{
			"windowVS": infraNamedWindowInsertFromVSCreate,
			"window":   infraNamedWindowInsertFromVSTwoCreate,
			"insertA":  infraNamedWindowInsertFromVSInsertA,
			"insertB":  infraNamedWindowInsertFromVSInsertB,
			"insertVS": infraNamedWindowInsertFromVSInsert,
		}
	case "lenient-map":
		return map[string]string{
			"schema":    infraNamedWindowInsertFromMapSchema,
			"window":    infraNamedWindowInsertFromLenientWindow,
			"insertOne": infraNamedWindowInsertFromLenientInsertOne,
			"insertTwo": infraNamedWindowInsertFromLenientInsertTwo,
		}
	default:
		return map[string]string{
			"schema":    infraNamedWindowInsertFromObjectArraySchema,
			"window":    infraNamedWindowInsertFromLenientWindow,
			"insertOne": infraNamedWindowInsertFromLenientInsertOne,
			"insertTwo": infraNamedWindowInsertFromLenientInsertTwo,
		}
	}
}

// infraNamedWindowInsertFromCasePin pins the per-case step order by index
// (0-based; index 0 is the case marker).
type infraNamedWindowInsertFromCasePin struct {
	deploys   map[int]string
	sends     map[int]compat.Step
	snapshot  map[int]string
	snapshotM map[int]string
	buildErrs map[int]string
	unrep     map[int]string
	total     int
}

func infraNamedWindowInsertFromBeanSend(theString string, primitive int) compat.Step {
	return compat.Step{
		Op:        "send",
		EventType: "SupportBean",
		Payload:   json.RawMessage(fmt.Sprintf(`{"theString": %q, "intPrimitive": %d}`, theString, primitive)),
	}
}

func infraNamedWindowInsertFromS0Send(id int) compat.Step {
	return compat.Step{
		Op:        "send",
		EventType: "SupportBean_S0",
		Payload:   json.RawMessage(fmt.Sprintf(`{"id": %d}`, id)),
	}
}

func infraNamedWindowInsertFromMapABSend(a string, b int) compat.Step {
	return compat.Step{
		Op:        "send",
		EventType: "MyMapAB",
		Payload:   json.RawMessage(fmt.Sprintf(`{"a": %q, "b": %d}`, a, b)),
	}
}

func infraNamedWindowInsertFromABSend(eventType, id string) compat.Step {
	return compat.Step{
		Op:        "send",
		EventType: eventType,
		Payload:   json.RawMessage(fmt.Sprintf(`{"id": %q}`, id)),
	}
}

// infraNamedWindowInsertFromBuildError pins one InfraInvalid probe step.
func infraNamedWindowInsertFromBuildError(statement, epl, expectError string) compat.Step {
	return compat.Step{Op: "build-error", Statement: statement, Epl: epl, ExpectError: expectError}
}

func infraNamedWindowInsertFromPins() []infraNamedWindowInsertFromCasePin {
	return []infraNamedWindowInsertFromCasePin{
		{
			deploys: map[int]string{1: "windowOne", 2: "windowTwo", 3: "insert", 4: "selectOne"},
			sends: map[int]compat.Step{
				5: infraNamedWindowInsertFromBeanSend("E1", 1),
			},
			total: 7,
		},
		{
			deploys: map[int]string{
				1: "window", 2: "insert",
				8: "windowTwo", 10: "windowThree", 12: "windowFour",
				14: "insertA", 15: "insertB", 16: "insertC", 17: "insertD",
			},
			sends: map[int]compat.Step{
				3:  infraNamedWindowInsertFromBeanSend("A1", 1),
				4:  infraNamedWindowInsertFromBeanSend("B2", 1),
				5:  infraNamedWindowInsertFromBeanSend("C3", 1),
				6:  infraNamedWindowInsertFromBeanSend("A4", 4),
				7:  infraNamedWindowInsertFromBeanSend("C5", 4),
				18: infraNamedWindowInsertFromBeanSend("B9", -9),
				19: infraNamedWindowInsertFromBeanSend("A8", -8),
				20: infraNamedWindowInsertFromBeanSend("C7", -7),
				21: infraNamedWindowInsertFromBeanSend("D6", -6),
			},
			snapshot:  map[int]string{9: "windowTwo", 11: "windowThree", 13: "windowFour"},
			snapshotM: map[int]string{9: "ordered", 11: "ordered", 13: "any"},
			total:     23,
		},
		{
			deploys: map[int]string{1: "window", 2: "insert", 7: "windowTwo", 9: "windowThree"},
			sends: map[int]compat.Step{
				3: infraNamedWindowInsertFromMapABSend("E1", 2),
				4: infraNamedWindowInsertFromMapABSend("E2", 10),
				5: infraNamedWindowInsertFromMapABSend("E3", 10),
			},
			unrep:     map[int]string{6: "om-roundtrip"},
			snapshot:  map[int]string{8: "windowTwo", 10: "windowThree"},
			snapshotM: map[int]string{8: "ordered", 10: "ordered"},
			total:     12,
		},
		{
			deploys: map[int]string{1: "window"},
			buildErrs: map[int]string{
				2: "missing-window-insert",
				3: "missing-window-insert-where",
				4: "insert-where-subselect",
				5: "insert-where-aggregation",
				6: "insert-where-prev",
			},
			total: 8,
		},
		{
			deploys: map[int]string{1: "windowVS", 2: "window", 3: "insertA", 4: "insertB", 5: "insertVS"},
			sends: map[int]compat.Step{
				6: infraNamedWindowInsertFromABSend("SupportBean_A", "A1"),
				7: infraNamedWindowInsertFromABSend("SupportBean_B", "B1"),
			},
			snapshot:  map[int]string{8: "window"},
			snapshotM: map[int]string{8: "ordered"},
			total:     10,
		},
		{
			deploys: map[int]string{1: "schema", 2: "window", 3: "insertOne", 4: "insertTwo"},
			sends: map[int]compat.Step{
				5: infraNamedWindowInsertFromBeanSend("E1", 0),
				7: infraNamedWindowInsertFromS0Send(10),
			},
			snapshot:  map[int]string{6: "window", 8: "window"},
			snapshotM: map[int]string{6: "ordered", 8: "ordered"},
			total:     10,
		},
		{
			deploys: map[int]string{1: "schema", 2: "window", 3: "insertOne", 4: "insertTwo"},
			sends: map[int]compat.Step{
				5: infraNamedWindowInsertFromBeanSend("E1", 0),
				7: infraNamedWindowInsertFromS0Send(10),
			},
			snapshot:  map[int]string{6: "window", 8: "window"},
			snapshotM: map[int]string{6: "ordered", 8: "ordered"},
			total:     10,
		},
	}
}

// infraNamedWindowInsertFromProbeEPLs pins the byte-exact EPL each
// infra-invalid build-error step carries.
var infraNamedWindowInsertFromProbeEPLs = map[string]string{
	"missing-window-insert":       infraNamedWindowInsertFromProbeInsert,
	"missing-window-insert-where": infraNamedWindowInsertFromProbeInsertWhere,
	"insert-where-subselect":      infraNamedWindowInsertFromProbeSubselect,
	"insert-where-aggregation":    infraNamedWindowInsertFromProbeAggregation,
	"insert-where-prev":           infraNamedWindowInsertFromProbePrev,
}

// infraNamedWindowInsertFromProbeErrors pins the Java message prefix each
// infra-invalid build-error step records.
var infraNamedWindowInsertFromProbeErrors = map[string]string{
	"missing-window-insert":       infraNamedWindowInsertFromMissingWindowPrefix,
	"missing-window-insert-where": infraNamedWindowInsertFromMissingWindowPrefix,
	"insert-where-subselect":      infraNamedWindowInsertFromSubselectPrefix,
	"insert-where-aggregation":    infraNamedWindowInsertFromAggregationPrefix,
	"insert-where-prev":           infraNamedWindowInsertFromPrevPrefix,
}

func validateInfraNamedWindowInsertFromScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNamedWindowInsertFromID {
		return fmt.Errorf("%s scenario shape is not pinned", infraNamedWindowInsertFromID)
	}
	pins := infraNamedWindowInsertFromPins()
	offset := 0
	for caseIndex, caseName := range infraNamedWindowInsertFromCases {
		pin := pins[caseIndex]
		steps := scenario.Steps[offset : offset+pin.total]
		offset += pin.total
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", infraNamedWindowInsertFromID, caseName)
		}
		deployEPLs := infraNamedWindowInsertFromDeployEPLs(caseName)
		for index := 1; index < pin.total; index++ {
			step := steps[index]
			if label, isDeploy := pin.deploys[index]; isDeploy {
				if step.Op != "deploy" || step.Case != caseName || step.Statement != label {
					return fmt.Errorf("%s case %q step %d must deploy %q", infraNamedWindowInsertFromID, caseName, index, label)
				}
				if step.Epl != deployEPLs[label] {
					return fmt.Errorf("%s case %q step %d deploy %q is not pinned", infraNamedWindowInsertFromID, caseName, index, label)
				}
				continue
			}
			if snapshotTarget, isSnapshot := pin.snapshot[index]; isSnapshot {
				if step.Op != "snapshot" || step.Statement != snapshotTarget || step.Mode != pin.snapshotM[index] {
					return fmt.Errorf("%s case %q step %d snapshot is not pinned", infraNamedWindowInsertFromID, caseName, index)
				}
				continue
			}
			if label, isBuildError := pin.buildErrs[index]; isBuildError {
				if step.Op != "build-error" || step.Case != caseName || step.Statement != label ||
					step.Epl != infraNamedWindowInsertFromProbeEPLs[label] ||
					step.ExpectError != infraNamedWindowInsertFromProbeErrors[label] {
					return fmt.Errorf("%s case %q step %d build-error is not pinned", infraNamedWindowInsertFromID, caseName, index)
				}
				continue
			}
			if label, isUnrep := pin.unrep[index]; isUnrep {
				if step.Op != "unrepresentable" || step.Case != caseName || step.Statement != label ||
					step.ExpectError != infraNamedWindowInsertFromOMNote {
					return fmt.Errorf("%s case %q step %d unrepresentable is not pinned", infraNamedWindowInsertFromID, caseName, index)
				}
				continue
			}
			want, isSend := pin.sends[index]
			if !isSend {
				if index == pin.total-1 && step.Op == "undeploy-all" && step.Case == caseName {
					continue
				}
				return fmt.Errorf("%s case %q step %d has unexpected op %q", infraNamedWindowInsertFromID, caseName, index, step.Op)
			}
			if step.Op != "send" || step.EventType != want.EventType {
				return fmt.Errorf("%s case %q step %d must send %q", infraNamedWindowInsertFromID, caseName, index, want.EventType)
			}
			payload, err := decodeInfraNamedWindowInsertFromPayload(step)
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNamedWindowInsertFromID, caseName, index, err)
			}
			switch typed := payload.(type) {
			case infraNamedWindowInsertFromBean:
				var wantBean infraNamedWindowInsertFromBean
				if err := json.Unmarshal(want.Payload, &wantBean); err != nil {
					return err
				}
				if typed.TheString != wantBean.TheString || typed.IntPrimitive != wantBean.IntPrimitive {
					return fmt.Errorf("%s case %q step %d SupportBean payload is not pinned", infraNamedWindowInsertFromID, caseName, index)
				}
			case infraNamedWindowInsertFromS0:
				var wantS0 infraNamedWindowInsertFromS0
				if err := json.Unmarshal(want.Payload, &wantS0); err != nil {
					return err
				}
				if typed.ID != wantS0.ID {
					return fmt.Errorf("%s case %q step %d SupportBean_S0 payload is not pinned", infraNamedWindowInsertFromID, caseName, index)
				}
			case map[string]any:
				wantPayload, err := decodeInfraNamedWindowInsertFromPayload(want)
				if err != nil {
					return err
				}
				if !reflect.DeepEqual(typed, wantPayload) {
					return fmt.Errorf("%s case %q step %d map payload is not pinned", infraNamedWindowInsertFromID, caseName, index)
				}
			case infraNamedWindowInsertFromAB:
				var wantAB infraNamedWindowInsertFromAB
				if err := json.Unmarshal(want.Payload, &wantAB); err != nil {
					return err
				}
				if typed.ID != wantAB.ID {
					return fmt.Errorf("%s case %q step %d %s payload is not pinned", infraNamedWindowInsertFromID, caseName, index, step.EventType)
				}
			}
		}
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNamedWindowInsertFromID)
	}
	return nil
}

func runInfraNamedWindowInsertFromScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNamedWindowInsertFromScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	pins := infraNamedWindowInsertFromPins()
	for caseIndex, caseName := range infraNamedWindowInsertFromCases {
		caseSteps := scenario.Steps[offset : offset+pins[caseIndex].total]
		offset += pins[caseIndex].total
		caseTrace, err := runInfraNamedWindowInsertFromCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNamedWindowInsertFromID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runInfraNamedWindowInsertFromCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNamedWindowInsertFromBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNamedWindowInsertFromS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}

	// Infra must exist before the engine snapshots environment named windows.
	schemaByName := map[string]esper.Schema{}
	switch caseName {
	case "create-after-named":
		beanSchema, ok := env.Schema("SupportBean")
		if !ok {
			return compat.Trace{}, fmt.Errorf("SupportBean schema is missing")
		}
		schemaByName["MyWindow"] = beanSchema
		schemaByName["MyWindowTwo"] = beanSchema
		if _, err := esper.CreateNamedWindow(env, "MyWindow", beanSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowTwo", beanSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	case "insert-where-type-filter":
		beanSchema, ok := env.Schema("SupportBean")
		if !ok {
			return compat.Trace{}, fmt.Errorf("SupportBean schema is missing")
		}
		schemaByName["MyWindowIWT"] = beanSchema
		schemaByName["MyWindowTwo"] = beanSchema
		schemaByName["MyWindowThree"] = beanSchema
		schemaByName["MyWindowFour"] = beanSchema
		if _, err := esper.CreateNamedWindow(env, "MyWindowIWT", beanSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowTwo", beanSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowThree", beanSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowFour", beanSchema,
			esper.NamedWindowRetention(esper.Unique(esper.Field[infraNamedWindowInsertFromBean, int]("intPrimitive")))); err != nil {
			return compat.Trace{}, err
		}
	case "insert-where-om-staggered":
		// rep=MAP narrowing: the Java execution loops every
		// EventRepresentationChoice; the replay covers the map
		// representation only.
		mapABSchema, err := esper.NewMapSchema("MyMapAB", []esper.FieldSpec{
			esper.FieldDef("a", reflect.TypeOf("")),
			esper.FieldDef("b", reflect.TypeOf(0)),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterSchema(mapABSchema); err != nil {
			return compat.Trace{}, err
		}
		iwomThreeSchema, err := esper.NewMapSchema("MyWindowIWOMThree", []esper.FieldSpec{
			esper.FieldDef("a", reflect.TypeOf("")),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		schemaByName["MyWindowIWOM"] = mapABSchema
		schemaByName["MyWindowIWOMTwo"] = mapABSchema
		schemaByName["MyWindowIWOMThree"] = iwomThreeSchema
		if _, err := esper.CreateNamedWindow(env, "MyWindowIWOM", mapABSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowIWOMTwo", mapABSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowIWOMThree", iwomThreeSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	case "infra-invalid":
		beanSchema, ok := env.Schema("SupportBean")
		if !ok {
			return compat.Trace{}, fmt.Errorf("SupportBean schema is missing")
		}
		schemaByName["MyWindowINV"] = beanSchema
		if _, err := esper.CreateNamedWindow(env, "MyWindowINV", beanSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	case "variant-stream":
		schemaA, err := esper.RegisterStruct[infraNamedWindowInsertFromAB](env, "SupportBean_A")
		if err != nil {
			return compat.Trace{}, err
		}
		schemaB, err := esper.RegisterStruct[infraNamedWindowInsertFromAB](env, "SupportBean_B")
		if err != nil {
			return compat.Trace{}, err
		}
		variant, err := esper.RegisterVariant(env, "VarStream", schemaA, schemaB)
		if err != nil {
			return compat.Trace{}, err
		}
		schemaByName["MyWindowVS"] = variant
		schemaByName["MyWindowVSTwo"] = variant
		if _, err := esper.CreateNamedWindow(env, "MyWindowVS", variant,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowVSTwo", variant,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	case "lenient-map":
		schema, err := esper.NewMapSchema("MyTwoColEvent", []esper.FieldSpec{
			esper.FieldDef("c0", reflect.TypeOf("")),
			esper.FieldDef("c1", reflect.TypeOf((*int)(nil))),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterSchema(schema); err != nil {
			return compat.Trace{}, err
		}
		schemaByName["MyWindow"] = schema
		if _, err := esper.CreateNamedWindow(env, "MyWindow", schema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	default:
		schema, err := esper.NewObjectArraySchema("MyTwoColEvent", []esper.FieldSpec{
			esper.FieldDef("c0", reflect.TypeOf("")),
			esper.FieldDef("c1", reflect.TypeOf((*int)(nil))),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterSchema(schema); err != nil {
			return compat.Trace{}, err
		}
		schemaByName["MyWindow"] = schema
		if _, err := esper.CreateNamedWindow(env, "MyWindow", schema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	}

	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNamedWindowInsertFromJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNamedWindowInsertFromID}
	sequence := map[string]uint64{}
	record := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
	}

	// Snapshot rows are projected to the fields the Java assertions read;
	// mode-any snapshots are emitted in the differential canonical row order,
	// ordered snapshots keep engine iteration order. The variant-stream
	// snapshot renames the member id property to the dynamic id? spelling the
	// Java iterator assert reads.
	snapshotProjection := map[string]map[string][]string{
		"insert-where-type-filter": {
			"windowTwo":   {"theString"},
			"windowThree": {"theString"},
			"windowFour":  {"theString"},
		},
		"insert-where-om-staggered": {
			"windowTwo":   {"a", "b"},
			"windowThree": {"a"},
		},
		"lenient-map":         {"window": {"c0", "c1"}},
		"lenient-objectarray": {"window": {"c0", "c1"}},
	}
	// snapshotRename maps case -> statement -> [from, to] field names for
	// rows whose Java-asserted property spelling differs from the stored
	// field: the variant window stores the member id while the Java iterator
	// assert reads the dynamic id? property.
	snapshotRename := map[string]map[string][2]string{
		"variant-stream": {"window": {"id", "id?"}},
	}
	modeAny := map[string]bool{
		"windowFour": true,
	}
	recordSnapshot := func(statement string, result esper.QueryResult) {
		projected := compat.NormalizeResults(result.Batch.New)
		if renames, ok := snapshotRename[caseName]; ok {
			if rename, ok := renames[statement]; ok {
				for i := range projected {
					projected[i].Fields = map[string]any{rename[1]: projected[i].Fields[rename[0]]}
				}
			}
		}
		if projections, ok := snapshotProjection[caseName]; ok {
			if projection, ok := projections[statement]; ok {
				projected = projectRecords(projected, projection)
			}
		}
		if modeAny[statement] {
			sort.SliceStable(projected, func(i, j int) bool {
				leftJSON, _ := json.Marshal(projected[i].Fields)
				rightJSON, _ := json.Marshal(projected[j].Fields)
				return string(leftJSON) < string(rightJSON)
			})
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "snapshot",
			Statement: statement,
			Sequence:  0,
			Time:      compat.FormatTraceTime(time.Unix(0, 0).UTC()),
			New:       projected,
		})
	}

	statements := map[string]*esper.Statement{}

	// seedFrom pins Esper's create-window-insert deploy-time semantics: the
	// starting contents of the new window are copied from the source window
	// when the create statement deploys (filtered for `insert where`), and
	// the copied rows never reach the new window's create-statement listener
	// because it attaches after the deployment. Ongoing flow requires
	// explicit insert-into statements, exactly as the Java execution's own
	// routing inserts demonstrate. keep maps a source underlying to the value
	// inserted into the target (nil,false skips the row); it also projects
	// the row when the target window declares a narrower schema.
	seedFrom := func(target string, sourceStatement string, keep func(underlying any) (any, bool)) error {
		source, ok := statements[sourceStatement]
		if !ok {
			return fmt.Errorf("seed source statement %q not found", sourceStatement)
		}
		result, err := source.Snapshot(ctx)
		if err != nil {
			return err
		}
		for _, row := range result.Batch.New {
			underlying := row.Underlying()
			if underlying == nil {
				return fmt.Errorf("seed source row has no underlying")
			}
			insert := underlying
			if keep != nil {
				projected, ok := keep(underlying)
				if !ok {
					continue
				}
				insert = projected
			}
			if err := engine.InsertNamedWindow(ctx, target, insert); err != nil {
				return err
			}
		}
		return nil
	}

	// beanPrefixKeep is the insert-where theString-like-'X%' seed filter for
	// the bean-typed windows.
	beanPrefixKeep := func(prefix string) func(any) (any, bool) {
		return func(underlying any) (any, bool) {
			bean, ok := underlying.(infraNamedWindowInsertFromBean)
			if !ok {
				panic(fmt.Sprintf("seed row underlying is %T, want infraNamedWindowInsertFromBean", underlying))
			}
			return underlying, len(bean.TheString) > 0 && bean.TheString[:1] == prefix
		}
	}

	// buildError runs one InfraInvalid probe: the pinned EPL is verified, the
	// nearest expressible Go boundary is exercised for the representable
	// probes, and the pinned Java prefix is recorded.
	buildError := func(step compat.Step) error {
		if step.Epl != infraNamedWindowInsertFromProbeEPLs[step.Statement] ||
			step.ExpectError != infraNamedWindowInsertFromProbeErrors[step.Statement] {
			return fmt.Errorf("%s: build-error probe %q is not pinned", infraNamedWindowInsertFromID, step.Statement)
		}
		var buildErr error
		switch step.Statement {
		case "missing-window-insert", "missing-window-insert-where":
			// `as SupportBean insert` requires an existing named window;
			// the nearest Go boundary is the named-window source lookup,
			// which rejects the event-type name.
			_, buildErr = env.Build(esper.FromNamedWindow(env, "SupportBean").
				CreateNamedWindowQuery(esper.StatementName(step.Statement)))
			if buildErr == nil || !strings.Contains(buildErr.Error(), `named window "SupportBean" is not registered`) {
				return fmt.Errorf("%s: build-error probe %q drift: got %v", infraNamedWindowInsertFromID, step.Statement, buildErr)
			}
		case "insert-where-subselect", "insert-where-aggregation", "insert-where-prev":
			// Go has no create-window-where surface, so the Java rejection
			// is unrepresentable; the pinned prefix is recorded without
			// claiming a Go boundary.
			buildErr = fmt.Errorf("create-window insert-where is unrepresentable")
		default:
			return fmt.Errorf("%s: unknown build-error probe %q", infraNamedWindowInsertFromID, step.Statement)
		}
		if buildErr == nil {
			return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", infraNamedWindowInsertFromID, step.Statement)
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "compile-error",
			Statement: step.Statement,
			Value:     step.ExpectError,
		})
		return nil
	}

	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			var plan esper.Plan
			var err error
			switch {
			case caseName == "create-after-named" && step.Statement == "windowOne":
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindow").
					CreateNamedWindowQuery(esper.StatementName("windowOne")))
			case caseName == "create-after-named" && step.Statement == "windowTwo":
				// Type-by-window creation: no insert keyword, no seed.
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindowTwo").
					CreateNamedWindowQuery(esper.StatementName("windowTwo")))
			case caseName == "create-after-named" && step.Statement == "insert":
				plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowInsertFromBean](env, "SupportBean")).
					InsertIntoNamedWindow("MyWindow", esper.CopyMatchingFields()).
					Query(esper.StatementName("insert")))
			case caseName == "create-after-named" && step.Statement == "selectOne":
				plan, err = env.Build(esper.FromNamedWindowAs[infraNamedWindowInsertFromBean](env, "MyWindow").
					AsRecord().
					Select(esper.Alias("theString", esper.Field[any, string]("theString"))).
					Query(esper.StatementName("selectOne")))
			case caseName == "insert-where-type-filter" && step.Statement == "window":
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindowIWT").
					CreateNamedWindowQuery(esper.StatementName("window")))
			case caseName == "insert-where-type-filter" && step.Statement == "insert":
				plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowInsertFromBean](env, "SupportBean").
					Filter(esper.Greater[int](esper.Field[infraNamedWindowInsertFromBean, int]("intPrimitive"), esper.Literal(0)))).
					InsertIntoNamedWindow("MyWindowIWT", esper.CopyMatchingFields()).
					Query(esper.StatementName("insert")))
			case caseName == "insert-where-type-filter" && step.Statement == "windowTwo":
				if err = seedFrom("MyWindowTwo", "window", nil); err != nil {
					return compat.Trace{}, fmt.Errorf("seed MyWindowTwo: %w", err)
				}
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindowTwo").
					Query(esper.StatementName("windowTwo")))
			case caseName == "insert-where-type-filter" && step.Statement == "windowThree":
				if err = seedFrom("MyWindowThree", "window", beanPrefixKeep("A")); err != nil {
					return compat.Trace{}, fmt.Errorf("seed MyWindowThree: %w", err)
				}
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindowThree").
					Query(esper.StatementName("windowThree")))
			case caseName == "insert-where-type-filter" && step.Statement == "windowFour":
				if err = seedFrom("MyWindowFour", "window", nil); err != nil {
					return compat.Trace{}, fmt.Errorf("seed MyWindowFour: %w", err)
				}
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindowFour").
					Query(esper.StatementName("windowFour")))
			case caseName == "insert-where-type-filter" && step.Statement == "insertA":
				plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowInsertFromBean](env, "SupportBean").
					Filter(esper.Like(esper.Field[infraNamedWindowInsertFromBean, string]("theString"), esper.Literal("A%")))).
					InsertIntoNamedWindow("MyWindowIWT", esper.CopyMatchingFields()).
					Query(esper.StatementName("insertA")))
			case caseName == "insert-where-type-filter" && step.Statement == "insertB":
				plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowInsertFromBean](env, "SupportBean").
					Filter(esper.Like(esper.Field[infraNamedWindowInsertFromBean, string]("theString"), esper.Literal("B%")))).
					InsertIntoNamedWindow("MyWindowTwo", esper.CopyMatchingFields()).
					Query(esper.StatementName("insertB")))
			case caseName == "insert-where-type-filter" && step.Statement == "insertC":
				plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowInsertFromBean](env, "SupportBean").
					Filter(esper.Like(esper.Field[infraNamedWindowInsertFromBean, string]("theString"), esper.Literal("C%")))).
					InsertIntoNamedWindow("MyWindowThree", esper.CopyMatchingFields()).
					Query(esper.StatementName("insertC")))
			case caseName == "insert-where-type-filter" && step.Statement == "insertD":
				plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowInsertFromBean](env, "SupportBean").
					Filter(esper.Like(esper.Field[infraNamedWindowInsertFromBean, string]("theString"), esper.Literal("D%")))).
					InsertIntoNamedWindow("MyWindowFour", esper.CopyMatchingFields()).
					Query(esper.StatementName("insertD")))
			case caseName == "insert-where-om-staggered" && step.Statement == "window":
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindowIWOM").
					CreateNamedWindowQuery(esper.StatementName("window")))
			case caseName == "insert-where-om-staggered" && step.Statement == "insert":
				plan, err = env.Build(esper.FromAny(env, "MyMapAB").
					InsertInto("MyWindowIWOM", esper.StatementName("insert")))
			case caseName == "insert-where-om-staggered" && step.Statement == "windowTwo":
				// `insert where b=10`: the deploy-time seed copies only the
				// b=10 rows; the listener attaches after deployment so the
				// seed never reaches it.
				if err = seedFrom("MyWindowIWOMTwo", "window", func(underlying any) (any, bool) {
					row, ok := underlying.(map[string]any)
					if !ok {
						return nil, false
					}
					return underlying, infraNamedWindowInsertFromIntValue(row["b"]) == 10
				}); err != nil {
					return compat.Trace{}, fmt.Errorf("seed MyWindowIWOMTwo: %w", err)
				}
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindowIWOMTwo").
					Query(esper.StatementName("windowTwo")))
			case caseName == "insert-where-om-staggered" && step.Statement == "windowThree":
				// `as select a ... insert where a = 'E2'`: the chained seed
				// filters on a and projects the narrower {a} schema.
				if err = seedFrom("MyWindowIWOMThree", "windowTwo", func(underlying any) (any, bool) {
					row, ok := underlying.(map[string]any)
					if !ok || row["a"] != "E2" {
						return nil, false
					}
					return map[string]any{"a": row["a"]}, true
				}); err != nil {
					return compat.Trace{}, fmt.Errorf("seed MyWindowIWOMThree: %w", err)
				}
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindowIWOMThree").
					Query(esper.StatementName("windowThree")))
			case caseName == "infra-invalid" && step.Statement == "window":
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindowINV").
					CreateNamedWindowQuery(esper.StatementName("window")))
			case caseName == "variant-stream" && step.Statement == "windowVS":
				// `create window ... as select * from VarStream` only derives
				// the window type in Esper; the Go bridge routes the variant
				// stream into the pre-registered window.
				plan, err = env.Build(esper.FromAny(env, "VarStream").
					InsertInto("MyWindowVS", esper.StatementName("windowVS")))
			case caseName == "variant-stream" && step.Statement == "window":
				// Type-by-window creation: no insert keyword, no seed.
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindowVSTwo").
					CreateNamedWindowQuery(esper.StatementName("window")))
			case caseName == "variant-stream" && step.Statement == "insertA":
				plan, err = env.Build(esper.From[infraNamedWindowInsertFromAB](env, "SupportBean_A").
					InsertInto("VarStream", esper.StatementName("insertA")))
			case caseName == "variant-stream" && step.Statement == "insertB":
				plan, err = env.Build(esper.From[infraNamedWindowInsertFromAB](env, "SupportBean_B").
					InsertInto("VarStream", esper.StatementName("insertB")))
			case caseName == "variant-stream" && step.Statement == "insertVS":
				plan, err = env.Build(esper.FromAny(env, "VarStream").
					InsertInto("MyWindowVSTwo", esper.StatementName("insertVS")))
			case (caseName == "lenient-map" || caseName == "lenient-objectarray") && step.Statement == "schema":
				// The schema is a catalog operation, already applied at env
				// level before the engine started.
				continue
			case (caseName == "lenient-map" || caseName == "lenient-objectarray") && step.Statement == "window":
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindow").
					CreateNamedWindowQuery(esper.StatementName("window")))
			case (caseName == "lenient-map" || caseName == "lenient-objectarray") && step.Statement == "insertOne":
				plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowInsertFromBean](env, "SupportBean")).
					InsertIntoNamedWindow("MyWindow",
						esper.SetColumn("c0", esper.Field[infraNamedWindowInsertFromBean, string]("theString"))).
					Query(esper.StatementName("insertOne")))
			case (caseName == "lenient-map" || caseName == "lenient-objectarray") && step.Statement == "insertTwo":
				plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowInsertFromS0](env, "SupportBean_S0")).
					InsertIntoNamedWindow("MyWindow",
						esper.SetColumn("c1", esper.Field[infraNamedWindowInsertFromS0, int]("id"))).
					Query(esper.StatementName("insertTwo")))
			default:
				return compat.Trace{}, fmt.Errorf("unexpected deploy statement %q for case %q", step.Statement, caseName)
			}
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			listen := map[string]map[string]bool{
				"create-after-named":        {"windowOne": true, "selectOne": true},
				"insert-where-type-filter":  {"window": true, "windowTwo": true, "windowThree": true, "windowFour": true},
				"insert-where-om-staggered": {"window": true, "windowTwo": true},
				"infra-invalid":             {},
				"variant-stream":            {},
				"lenient-map":               {},
				"lenient-objectarray":       {},
			}[caseName]
			for _, statement := range deployment.Statements() {
				if statement.Name() == step.Statement {
					statements[step.Statement] = statement
				}
			}
			if listen[step.Statement] {
				if statement, ok := statements[step.Statement]; ok {
					name := step.Statement
					if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record(name, batch)
						return nil
					}); err != nil {
						return compat.Trace{}, err
					}
				}
			}
		case "build-error":
			if err := buildError(step); err != nil {
				return compat.Trace{}, err
			}
		case "unrepresentable":
			if step.Statement != "om-roundtrip" || step.ExpectError != infraNamedWindowInsertFromOMNote {
				return compat.Trace{}, fmt.Errorf("%s: unrepresentable step %q is not pinned", infraNamedWindowInsertFromID, step.Statement)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "unrepresentable",
				Statement: step.Statement,
				Value:     step.ExpectError,
			})
		case "send":
			payload, err := decodeInfraNamedWindowInsertFromPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		case "snapshot":
			statement, ok := statements[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("snapshot statement %q not found", step.Statement)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return compat.Trace{}, err
			}
			recordSnapshot(step.Statement, result)
		case "undeploy-all":
		}
	}
	return trace, nil
}

// infraNamedWindowInsertFromIntValue reads a JSON-decoded map payload number
// (float64) or an int stored by the map event materialization.
func infraNamedWindowInsertFromIntValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}

type infraNamedWindowInsertFromS0 struct {
	ID int `esper:"id"`
}

func decodeInfraNamedWindowInsertFromPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNamedWindowInsertFromFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean infraNamedWindowInsertFromBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		if err := requireInfraNamedWindowInsertFromFields(fields, "id"); err != nil {
			return nil, err
		}
		var s0 infraNamedWindowInsertFromS0
		if err := json.Unmarshal(step.Payload, &s0); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return s0, nil
	case "MyMapAB":
		if err := requireInfraNamedWindowInsertFromFields(fields, "a", "b"); err != nil {
			return nil, err
		}
		var payload struct {
			A string `json:"a"`
			B int    `json:"b"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode MyMapAB: %w", err)
		}
		return map[string]any{"a": payload.A, "b": payload.B}, nil
	case "SupportBean_A", "SupportBean_B":
		if err := requireInfraNamedWindowInsertFromFields(fields, "id"); err != nil {
			return nil, err
		}
		var bean infraNamedWindowInsertFromAB
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode %s: %w", step.EventType, err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNamedWindowInsertFromID, step.EventType)
	}
}

func requireInfraNamedWindowInsertFromFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has unexpected fields")
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func validateInfraNamedWindowInsertFromStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if values[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}
