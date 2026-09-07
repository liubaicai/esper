package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	infraNamedWindowInsertFromID          = "infra-named-window-insert-from"
	infraNamedWindowInsertFromDescription = "InfraNamedWindowInsertFrom insert-from-window semantics: type-by-window creation after an existing named window with shared insert delivery, seeded keep-all and filtered and unique-index create-window-insert with deploy-time row copying that never reaches create-statement listeners, filtered routing inserts into each window, and lenient partial-column inserts over map and object-array schemas, captured from window listeners and ordered or canonical mode-any snapshots projected to the Java-asserted fields (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowInsertFrom.java)."
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
)

var (
	infraNamedWindowInsertFromJavaSources = []string{
		infraNamedWindowInsertFromSource,
	}
	infraNamedWindowInsertFromJavaRuntimeIDs = []string{
		"java-runtime-b3f6cb7b36c5211c8822",
		"java-runtime-e601b3cc7f827d578185",
		"java-runtime-46011542d6e9d34a87f5",
		"java-runtime-8138dd777290d00417d1",
	}
	infraNamedWindowInsertFromJavaExecutions = []string{
		"InfraCreateNamedAfterNamed",
		"InfraInsertWhereTypeAndFilter",
		"InfraNamedWindowInsertLenientPropCount{rep=MAP}",
		"InfraNamedWindowInsertLenientPropCount{rep=OBJECTARRAY}",
	}
	infraNamedWindowInsertFromJavaStaticIDs = []string{
		"java-b0917219c462cba9d770",
		"java-cff4a3193da2b063a351",
		"java-282d7b64878866ea4709",
		"java-282d7b64878866ea4709",
	}
	infraNamedWindowInsertFromCases = []string{
		"create-after-named",
		"insert-where-type-filter",
		"lenient-map",
		"lenient-objectarray",
	}
	// Ordinals are the Java executions() ordinals; ordinals 2/3/4 (OM
	// toEPL round-trip matrix, invalid compiles, variant streams) are
	// deferred with rationale in the capability manifest.
	infraNamedWindowInsertFromOrdinals  = []int{0, 1, 5, 6}
	infraNamedWindowInsertFromIterSnaps = []int{0, 3, 1, 1}
)

type infraNamedWindowInsertFromBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type infraNamedWindowInsertFromTwoCol struct {
	C0 string `esper:"c0"`
	C1 *int   `esper:"c1"`
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
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", infraNamedWindowInsertFromID)
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
			definition.Observation != "listener" ||
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
// the source; ord 0's windowOne does not).
func infraNamedWindowInsertFromObservedEPL(caseName string) string {
	switch caseName {
	case "create-after-named":
		return "@name('windowOne') create window MyWindow#keepall as SupportBean"
	case "insert-where-type-filter":
		return infraNamedWindowInsertFromIWTCreate
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
	spans := []int{7, 23, 10, 10}
	for caseIndex, caseName := range infraNamedWindowInsertFromCases {
		caseSteps := scenario.Steps[offset : offset+spans[caseIndex]]
		offset += spans[caseIndex]
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
	// ordered snapshots keep engine iteration order.
	snapshotProjection := map[string][]string{
		"insert-where-type-filter": {"theString"},
		"lenient-map":              {"c0", "c1"},
		"lenient-objectarray":      {"c0", "c1"},
	}
	modeAny := map[string]bool{
		"windowFour": true,
	}
	recordSnapshot := func(statement string, result esper.QueryResult) {
		projected := compat.NormalizeResults(result.Batch.New)
		if projection, ok := snapshotProjection[caseName]; ok {
			projected = projectRecords(projected, projection)
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
	// routing inserts demonstrate.
	seedFrom := func(target string, sourceStatement string, filterLike string) error {
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
			if filterLike != "" {
				bean, ok := underlying.(infraNamedWindowInsertFromBean)
				if !ok {
					return fmt.Errorf("seed source row has unexpected underlying type %T", underlying)
				}
				if len(bean.TheString) == 0 || bean.TheString[:1] != filterLike {
					continue
				}
			}
			if err := engine.InsertNamedWindow(ctx, target, underlying); err != nil {
				return err
			}
		}
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
				if err = seedFrom("MyWindowTwo", "window", ""); err != nil {
					return compat.Trace{}, fmt.Errorf("seed MyWindowTwo: %w", err)
				}
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindowTwo").
					Query(esper.StatementName("windowTwo")))
			case caseName == "insert-where-type-filter" && step.Statement == "windowThree":
				if err = seedFrom("MyWindowThree", "window", "A"); err != nil {
					return compat.Trace{}, fmt.Errorf("seed MyWindowThree: %w", err)
				}
				plan, err = env.Build(esper.FromNamedWindow(env, "MyWindowThree").
					Query(esper.StatementName("windowThree")))
			case caseName == "insert-where-type-filter" && step.Statement == "windowFour":
				if err = seedFrom("MyWindowFour", "window", ""); err != nil {
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
				"create-after-named":       {"windowOne": true, "selectOne": true},
				"insert-where-type-filter": {"window": true, "windowTwo": true, "windowThree": true, "windowFour": true},
				"lenient-map":              {},
				"lenient-objectarray":      {},
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
