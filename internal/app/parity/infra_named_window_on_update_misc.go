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
	infraNamedWindowOnUpdateMiscID          = "infra-named-window-on-update-misc"
	infraNamedWindowOnUpdateMiscDescription = "InfraNamedWindowOnUpdate on-update variants: method-call set clauses ported as approved-difference literal assignments with the update statement's own old/new delivery, subclass-typed named-window updates through a collapsed Go struct, copy-method bean updates under the field-wise-copy approved difference, and wrapper-select windows with an extra projected property, captured from update/create listeners and window snapshots projected to the Java-asserted fields (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnUpdate.java)."
	infraNamedWindowOnUpdateMiscJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNamedWindowOnUpdateMiscSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnUpdate.java"

	// The method-call set clauses (mywin.setIntPrimitive(10) and the plugin
	// single-row function setBeanLongPrimitive999) are pinned verbatim in the
	// scenario; the Go runner ports them as the approved-difference literal
	// assignments (both calls are constant-valued in Java).
	infraNamedWindowOnUpdateMiscNonPropertySetCreate = "@public create window MyWindowUNP#keepall as SupportBean"
	infraNamedWindowOnUpdateMiscNonPropertySetInsert = "insert into MyWindowUNP select * from SupportBean"
	infraNamedWindowOnUpdateMiscNonPropertySetUpdate = "@name('update') on SupportBean_S0 as sb update MyWindowUNP as mywin set mywin.setIntPrimitive(10),     setBeanLongPrimitive999(mywin)"
	infraNamedWindowOnUpdateMiscSubclassObserved     = "@name('create') create window MyWindowSC#keepall as select * from SupportBeanAbstractSub"
	infraNamedWindowOnUpdateMiscSubclassCreate       = "@name('create') @public create window MyWindowSC#keepall as select * from SupportBeanAbstractSub"
	infraNamedWindowOnUpdateMiscSubclassInsert       = "insert into MyWindowSC select * from SupportBeanAbstractSub"
	infraNamedWindowOnUpdateMiscSubclassUpdate       = "on SupportBean update MyWindowSC set v1=theString, v2=theString"
	infraNamedWindowOnUpdateMiscCopyMethodObserved   = "@name('window') create window MyWindowBeanCopyMethod#keepall as SupportBeanCopyMethod"
	infraNamedWindowOnUpdateMiscCopyMethodCreate     = "@name('window') @public create window MyWindowBeanCopyMethod#keepall as SupportBeanCopyMethod"
	infraNamedWindowOnUpdateMiscCopyMethodInsert     = "insert into MyWindowBeanCopyMethod select * from SupportBeanCopyMethod"
	infraNamedWindowOnUpdateMiscCopyMethodUpdate     = "on SupportBean update MyWindowBeanCopyMethod set valOne = 'x'"
	infraNamedWindowOnUpdateMiscWrapperObserved      = "@name('window') create window MyWindow#keepall as select *, 1 as p0 from SupportBean"
	infraNamedWindowOnUpdateMiscWrapperCreate        = "@name('window') @public create window MyWindow#keepall as select *, 1 as p0 from SupportBean"
	infraNamedWindowOnUpdateMiscWrapperInsert        = "insert into MyWindow select *, 2 as p0 from SupportBean"
	// The Java engine cannot replay the wrapper window with a separate
	// insert module (a create-window-as-select with wildcard + extra property
	// never registers its type among public types, proven by probes on the
	// fixed tree), so the scenario merges create+insert into one deploy step
	// — exactly how the Java suite compiles them.
	infraNamedWindowOnUpdateMiscWrapperCreateInsert = infraNamedWindowOnUpdateMiscWrapperCreate + ";\n" + infraNamedWindowOnUpdateMiscWrapperInsert
	infraNamedWindowOnUpdateMiscWrapperUpdate       = "on SupportBean_S0 update MyWindow set theString = 'x', p0 = 2"
)

var (
	infraNamedWindowOnUpdateMiscJavaSources = []string{
		infraNamedWindowOnUpdateMiscSource,
	}
	infraNamedWindowOnUpdateMiscJavaRuntimeIDs = []string{
		"java-runtime-5d041a3958410a90fa9a",
		"java-runtime-9ea51cb84d163be79693",
		"java-runtime-43feff597147c7867ca7",
		"java-runtime-09f56f49bab53388bb2d",
	}
	infraNamedWindowOnUpdateMiscJavaExecutions = []string{
		"InfraUpdateNonPropertySet",
		"InfraSubclass",
		"InfraUpdateCopyMethodBean",
		"InfraUpdateWrapper",
	}
	infraNamedWindowOnUpdateMiscJavaStaticIDs = []string{
		"java-26306de901985e70904f",
		"java-d920e63f2081f1d72d2f",
		"java-6eb1ea7dc80c3078f47f",
		"java-6b566143dbbb00ea202b",
	}
	infraNamedWindowOnUpdateMiscCases = []string{
		"non-property-set",
		"subclass",
		"copy-method",
		"wrapper",
	}
	infraNamedWindowOnUpdateMiscOrdinals = []int{0, 3, 4, 5}
)

type infraNamedWindowOnUpdateMiscBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type infraNamedWindowOnUpdateMiscS0 struct {
	ID int `esper:"id"`
}

type infraNamedWindowOnUpdateMiscSub struct {
	V1 *string `esper:"v1"`
	V2 *string `esper:"v2"`
}

type infraNamedWindowOnUpdateMiscCopyMethodBean struct {
	ValOne string `esper:"valOne"`
	ValTwo string `esper:"valTwo"`
}

type infraNamedWindowOnUpdateMiscWrapperWindow struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	P0           int    `esper:"p0"`
}

func loadInfraNamedWindowOnUpdateMiscScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNamedWindowOnUpdateMiscID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNamedWindowOnUpdateMiscID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNamedWindowOnUpdateMiscID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNamedWindowOnUpdateMiscID, err)
	}
	if err := requireInfraNamedWindowOnUpdateMiscFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNamedWindowOnUpdateMiscID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNamedWindowOnUpdateMiscID ||
		metadata.Description != infraNamedWindowOnUpdateMiscDescription ||
		metadata.JavaCommit != infraNamedWindowOnUpdateMiscJavaCommit ||
		metadata.JavaSource != infraNamedWindowOnUpdateMiscSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNamedWindowOnUpdateMiscID)
	}
	if err := validateInfraNamedWindowOnUpdateMiscStringArray(root["javaRuntimes"], infraNamedWindowOnUpdateMiscJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNamedWindowOnUpdateMiscStringArray(root["javaNames"], infraNamedWindowOnUpdateMiscJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNamedWindowOnUpdateMiscStringArray(root["javaStaticIds"], infraNamedWindowOnUpdateMiscJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNamedWindowOnUpdateMiscStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNamedWindowOnUpdateMiscCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", infraNamedWindowOnUpdateMiscID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNamedWindowOnUpdateMiscFields(object,
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
		if definition.Case != infraNamedWindowOnUpdateMiscCases[index] ||
			definition.Ordinal != infraNamedWindowOnUpdateMiscOrdinals[index] ||
			definition.RuntimeID != infraNamedWindowOnUpdateMiscJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNamedWindowOnUpdateMiscJavaExecutions[index] ||
			definition.Observation != "listener" ||
			definition.IteratorSnapshots != infraNamedWindowOnUpdateMiscIterSnaps[index] ||
			definition.EPL != infraNamedWindowOnUpdateMiscObservedEPL(definition.Case) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNamedWindowOnUpdateMiscID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNamedWindowOnUpdateMiscID)
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
			if err := requireInfraNamedWindowOnUpdateMiscFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNamedWindowOnUpdateMiscFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNamedWindowOnUpdateMiscFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNamedWindowOnUpdateMiscPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNamedWindowOnUpdateMiscFields(object, "op", "case", "statement", "mode"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNamedWindowOnUpdateMiscFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNamedWindowOnUpdateMiscScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

var infraNamedWindowOnUpdateMiscIterSnaps = []int{0, 0, 1, 1}

// infraNamedWindowOnUpdateMiscObservedEPL pins the cases[].epl metadata (the
// Java-observed statement, without the harness @public annotation).
func infraNamedWindowOnUpdateMiscObservedEPL(caseName string) string {
	switch caseName {
	case "non-property-set":
		return infraNamedWindowOnUpdateMiscNonPropertySetUpdate
	case "subclass":
		return infraNamedWindowOnUpdateMiscSubclassObserved
	case "copy-method":
		return infraNamedWindowOnUpdateMiscCopyMethodObserved
	default:
		return infraNamedWindowOnUpdateMiscWrapperObserved
	}
}

func validateInfraNamedWindowOnUpdateMiscScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNamedWindowOnUpdateMiscID {
		return fmt.Errorf("%s scenario shape is not pinned", infraNamedWindowOnUpdateMiscID)
	}
	// Per case the exact step interleaving is pinned. The create deploy
	// carries the harness @public annotation; the update/observed EPLs are
	// verbatim Java source text.
	type expectedStep struct {
		op        string
		statement string
		epl       string
		eventType string
		mode      string
	}
	deployEPLs := map[string]map[string]string{
		"non-property-set": {
			"create": infraNamedWindowOnUpdateMiscNonPropertySetCreate,
			"insert": infraNamedWindowOnUpdateMiscNonPropertySetInsert,
			"update": infraNamedWindowOnUpdateMiscNonPropertySetUpdate,
		},
		"subclass": {
			"create": infraNamedWindowOnUpdateMiscSubclassCreate,
			"insert": infraNamedWindowOnUpdateMiscSubclassInsert,
			"update": infraNamedWindowOnUpdateMiscSubclassUpdate,
		},
		"copy-method": {
			"create": infraNamedWindowOnUpdateMiscCopyMethodCreate,
			"insert": infraNamedWindowOnUpdateMiscCopyMethodInsert,
			"update": infraNamedWindowOnUpdateMiscCopyMethodUpdate,
		},
		"wrapper": {
			"create": infraNamedWindowOnUpdateMiscWrapperCreateInsert,
			"insert": infraNamedWindowOnUpdateMiscWrapperInsert,
			"update": infraNamedWindowOnUpdateMiscWrapperUpdate,
		},
	}
	snapshotStatement := map[string]string{
		"non-property-set": "",
		"subclass":         "",
		"copy-method":      "window",
		"wrapper":          "window",
	}
	offset := 0
	for _, caseName := range infraNamedWindowOnUpdateMiscCases {
		expected := []expectedStep{
			{op: "deploy", statement: "create", epl: deployEPLs[caseName]["create"]},
			{op: "deploy", statement: "insert", epl: deployEPLs[caseName]["insert"]},
			{op: "deploy", statement: "update", epl: deployEPLs[caseName]["update"]},
		}
		switch caseName {
		case "non-property-set":
			expected = append(expected,
				expectedStep{op: "send", eventType: "SupportBean"},
				expectedStep{op: "send", eventType: "SupportBean_S0"},
			)
		case "subclass":
			expected = append(expected,
				expectedStep{op: "send", eventType: "SupportBeanAbstractSub"},
				expectedStep{op: "send", eventType: "SupportBean"},
			)
		case "wrapper":
			// The wrapper create+insert is one merged deploy; no separate
			// insert step exists.
			expected = expected[:1]
			expected[0].epl = deployEPLs[caseName]["create"]
			expected = append(expected,
				expectedStep{op: "deploy", statement: "update", epl: deployEPLs[caseName]["update"]},
				expectedStep{op: "send", eventType: "SupportBean"},
				expectedStep{op: "send", eventType: "SupportBean_S0"},
			)
		default:
			expected = append(expected,
				expectedStep{op: "send", eventType: "SupportBeanCopyMethod"},
				expectedStep{op: "send", eventType: "SupportBean"},
			)
		}
		if statement := snapshotStatement[caseName]; statement != "" {
			expected = append(expected, expectedStep{op: "snapshot", statement: statement, mode: "any"})
		}
		expected = append(expected, expectedStep{op: "undeploy-all"})
		steps := scenario.Steps[offset : offset+1+len(expected)]
		offset += 1 + len(expected)
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", infraNamedWindowOnUpdateMiscID, caseName)
		}
		for index, want := range expected {
			step := steps[index+1]
			if step.Op != want.op || step.Case != caseName {
				return fmt.Errorf("%s case %q step %d must be op %q in order", infraNamedWindowOnUpdateMiscID, caseName, index+1, want.op)
			}
			switch want.op {
			case "deploy":
				if step.Statement != want.statement || step.Epl != want.epl {
					return fmt.Errorf("%s case %q step %d deploy %q is not pinned", infraNamedWindowOnUpdateMiscID, caseName, index+1, want.statement)
				}
			case "send":
				if step.EventType != want.eventType {
					return fmt.Errorf("%s case %q step %d must send %q", infraNamedWindowOnUpdateMiscID, caseName, index+1, want.eventType)
				}
				payload, err := decodeInfraNamedWindowOnUpdateMiscPayload(step)
				if err != nil {
					return fmt.Errorf("%s case %q step %d: %w", infraNamedWindowOnUpdateMiscID, caseName, index+1, err)
				}
				if err := validateInfraNamedWindowOnUpdateMiscSendValues(caseName, index+1, step.EventType, payload); err != nil {
					return err
				}
			case "snapshot":
				if step.Statement != want.statement || step.Mode != want.mode {
					return fmt.Errorf("%s case %q step %d snapshot is not pinned", infraNamedWindowOnUpdateMiscID, caseName, index+1)
				}
			case "undeploy-all":
			}
		}
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNamedWindowOnUpdateMiscID)
	}
	return nil
}

// validateInfraNamedWindowOnUpdateMiscSendValues pins every send payload so a
// drifted scenario cannot silently replay different values.
func validateInfraNamedWindowOnUpdateMiscSendValues(caseName string, stepNumber int, eventType string, payload any) error {
	switch eventType {
	case "SupportBean":
		bean := payload.(infraNamedWindowOnUpdateMiscBean)
		var theString string
		var primitive int
		switch caseName {
		case "non-property-set", "subclass":
			theString, primitive = "E1", 1
		case "copy-method":
			theString, primitive = "", 0
		case "wrapper":
			theString, primitive = "E1", 100
		}
		if bean.TheString != theString || bean.IntPrimitive != primitive {
			return fmt.Errorf("%s case %q step %d SupportBean payload is not pinned", infraNamedWindowOnUpdateMiscID, caseName, stepNumber)
		}
	case "SupportBean_S0":
		s0 := payload.(infraNamedWindowOnUpdateMiscS0)
		var id int
		switch caseName {
		case "non-property-set":
			id = 1
		case "wrapper":
			id = -1
		}
		if s0.ID != id {
			return fmt.Errorf("%s case %q step %d SupportBean_S0 payload is not pinned", infraNamedWindowOnUpdateMiscID, caseName, stepNumber)
		}
	case "SupportBeanAbstractSub":
		sub := payload.(infraNamedWindowOnUpdateMiscSub)
		if sub.V1 != nil || sub.V2 == nil || *sub.V2 != "value2" {
			return fmt.Errorf("%s case %q step %d SupportBeanAbstractSub payload is not pinned", infraNamedWindowOnUpdateMiscID, caseName, stepNumber)
		}
	case "SupportBeanCopyMethod":
		bean := payload.(infraNamedWindowOnUpdateMiscCopyMethodBean)
		if bean.ValOne != "a" || bean.ValTwo != "b" {
			return fmt.Errorf("%s case %q step %d SupportBeanCopyMethod payload is not pinned", infraNamedWindowOnUpdateMiscID, caseName, stepNumber)
		}
	}
	return nil
}

func runInfraNamedWindowOnUpdateMiscScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNamedWindowOnUpdateMiscScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNamedWindowOnUpdateMiscCases {
		span := 8
		if caseName != "copy-method" {
			span = 7
		}
		caseSteps := scenario.Steps[offset : offset+span]
		offset += span
		caseTrace, err := runInfraNamedWindowOnUpdateMiscCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNamedWindowOnUpdateMiscID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runInfraNamedWindowOnUpdateMiscCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNamedWindowOnUpdateMiscBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNamedWindowOnUpdateMiscS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNamedWindowOnUpdateMiscSub](env, "SupportBeanAbstractSub"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNamedWindowOnUpdateMiscCopyMethodBean](env, "SupportBeanCopyMethod"); err != nil {
		return compat.Trace{}, err
	}

	// Infra must exist before the engine snapshots environment named windows.
	var windowName string
	switch caseName {
	case "non-property-set":
		windowName = "MyWindowUNP"
		schema, ok := env.Schema("SupportBean")
		if !ok {
			return compat.Trace{}, fmt.Errorf("SupportBean schema is missing")
		}
		if _, err := esper.CreateNamedWindow(env, windowName, schema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	case "subclass":
		windowName = "MyWindowSC"
		schema, ok := env.Schema("SupportBeanAbstractSub")
		if !ok {
			return compat.Trace{}, fmt.Errorf("SupportBeanAbstractSub schema is missing")
		}
		if _, err := esper.CreateNamedWindow(env, windowName, schema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	case "copy-method":
		windowName = "MyWindowBeanCopyMethod"
		schema, ok := env.Schema("SupportBeanCopyMethod")
		if !ok {
			return compat.Trace{}, fmt.Errorf("SupportBeanCopyMethod schema is missing")
		}
		if _, err := esper.CreateNamedWindow(env, windowName, schema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	default:
		windowName = "MyWindow"
		schema, err := esper.NewMapSchema("InfraNamedWindowOnUpdateMiscWrapper", []esper.FieldSpec{
			esper.FieldDef("theString", reflect.TypeOf("")),
			esper.FieldDef("intPrimitive", reflect.TypeOf(int(0))),
			esper.FieldDef("p0", reflect.TypeOf(int(0))),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterSchema(schema); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, windowName, schema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	}

	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNamedWindowOnUpdateMiscJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNamedWindowOnUpdateMiscID}
	sequence := map[string]uint64{}
	// Listener rows are projected to the fields the Java assertions read:
	// intPrimitive/longPrimitive for the update statement's own delivery and
	// v1/v2 for the subclass window consumer.
	projection := []string{"intPrimitive", "longPrimitive"}
	subscribedStatement := "update"
	if caseName == "subclass" {
		projection = []string{"v1", "v2"}
		subscribedStatement = "create"
	} else if caseName != "non-property-set" {
		projection = nil
		subscribedStatement = ""
	}
	record := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		rows := compat.NormalizeResults(batch.New)
		oldRows := compat.NormalizeResults(batch.Old)
		if projection != nil {
			rows = projectRecords(rows, projection)
			oldRows = projectRecords(oldRows, projection)
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       rows,
			Old:       oldRows,
		})
	}

	// Snapshot rows are projected to the fields the Java assertions read and
	// emitted in the differential canonical row order (mode any).
	snapshotProjection := map[string][]string{
		"copy-method": {"valOne"},
		"wrapper":     {"theString", "p0"},
	}
	recordSnapshot := func(statement string, result esper.QueryResult) {
		projected := projectRecords(compat.NormalizeResults(result.Batch.New), snapshotProjection[caseName])
		sort.SliceStable(projected, func(i, j int) bool {
			leftJSON, _ := json.Marshal(projected[i].Fields)
			rightJSON, _ := json.Marshal(projected[j].Fields)
			return string(leftJSON) < string(rightJSON)
		})
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
	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			var plan esper.Plan
			var err error
			switch step.Statement {
			case "create":
				// Java statements default to irstream delivery, so the
				// window consumer receives the update's old pre-image too.
				plan, err = env.Build(esper.FromNamedWindow(env, windowName).
					Query(esper.StatementName("create"), esper.WithOldStream()))
				if err == nil && caseName == "wrapper" {
					// The scenario's merged wrapper deploy covers create and
					// insert; the create is a catalog operation already
					// applied, so this step deploys the insert statement.
					var insertPlan esper.Plan
					insertPlan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateMiscBean](env, "SupportBean")).
						InsertIntoNamedWindow(windowName,
							esper.CopyMatchingFields(),
							esper.SetColumn("p0", esper.Literal(2))).Query(esper.StatementName("insert")))
					if err == nil {
						if _, err = engine.Deploy(ctx, insertPlan); err != nil {
							err = fmt.Errorf("deploy insert: %w", err)
						}
					}
				}
			case "insert":
				switch caseName {
				case "non-property-set":
					plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateMiscBean](env, "SupportBean")).
						InsertIntoNamedWindow(windowName, esper.CopyMatchingFields()).Query(esper.StatementName("insert")))
				case "subclass":
					plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateMiscSub](env, "SupportBeanAbstractSub")).
						InsertIntoNamedWindow(windowName, esper.CopyMatchingFields()).Query(esper.StatementName("insert")))
				case "copy-method":
					plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateMiscCopyMethodBean](env, "SupportBeanCopyMethod")).
						InsertIntoNamedWindow(windowName, esper.CopyMatchingFields()).Query(esper.StatementName("insert")))
				default:
					// insert into MyWindow select *, 2 as p0: wildcard copy
					// plus the projected p0 assignment.
					plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateMiscBean](env, "SupportBean")).
						InsertIntoNamedWindow(windowName,
							esper.CopyMatchingFields(),
							esper.SetColumn("p0", esper.Literal(2))).Query(esper.StatementName("insert")))
				}
			case "update":
				switch caseName {
				case "non-property-set":
					// Approved difference: the Java set clauses are method
					// calls with constant values (setIntPrimitive(10) and the
					// plugin function writing 999), ported as literal
					// assignments.
					plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateMiscS0](env, "SupportBean_S0")).
						UpdateNamedWindow(windowName,
							esper.Literal(true),
							esper.SetColumn("intPrimitive", esper.Literal(10)),
							esper.SetColumn("longPrimitive", esper.Literal(int64(999)))).
						Query(esper.StatementName("update")))
				case "subclass":
					plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateMiscBean](env, "SupportBean")).
						UpdateNamedWindow(windowName,
							esper.Literal(true),
							esper.SetColumn("v1", esper.Field[infraNamedWindowOnUpdateMiscBean, string]("theString")),
							esper.SetColumn("v2", esper.Field[infraNamedWindowOnUpdateMiscBean, string]("theString"))).
						Query(esper.StatementName("update")))
				case "copy-method":
					plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateMiscBean](env, "SupportBean")).
						UpdateNamedWindow(windowName,
							esper.Literal(true),
							esper.SetColumn("valOne", esper.Literal("x"))).
						Query(esper.StatementName("update")))
				default:
					plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateMiscS0](env, "SupportBean_S0")).
						UpdateNamedWindow(windowName,
							esper.Literal(true),
							esper.SetColumn("theString", esper.Literal("x")),
							esper.SetColumn("p0", esper.Literal(2))).
						Query(esper.StatementName("update")))
				}
			default:
				return compat.Trace{}, fmt.Errorf("unexpected deploy statement %q", step.Statement)
			}
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			for _, statement := range deployment.Statements() {
				if statement.Name() == step.Statement {
					statements[step.Statement] = statement
				}
			}
			// The copy-method and wrapper create statements are named
			// 'window' in the Java source; the scenario's snapshot steps
			// address them by that name.
			if caseName == "copy-method" || caseName == "wrapper" {
				if statement, ok := statements["create"]; ok {
					statements["window"] = statement
				}
			}
			if subscribedStatement != "" && step.Statement == subscribedStatement {
				if statement, ok := statements[subscribedStatement]; ok {
					name := subscribedStatement
					if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record(name, batch)
						return nil
					}); err != nil {
						return compat.Trace{}, err
					}
				}
			}
		case "send":
			payload, err := decodeInfraNamedWindowOnUpdateMiscPayload(step)
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

func decodeInfraNamedWindowOnUpdateMiscPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNamedWindowOnUpdateMiscFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean infraNamedWindowOnUpdateMiscBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		if err := requireInfraNamedWindowOnUpdateMiscFields(fields, "id"); err != nil {
			return nil, err
		}
		var s0 infraNamedWindowOnUpdateMiscS0
		if err := json.Unmarshal(step.Payload, &s0); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return s0, nil
	case "SupportBeanAbstractSub":
		if err := requireInfraNamedWindowOnUpdateMiscFields(fields, "v1", "v2"); err != nil {
			return nil, err
		}
		var sub infraNamedWindowOnUpdateMiscSub
		if err := json.Unmarshal(step.Payload, &sub); err != nil {
			return nil, fmt.Errorf("decode SupportBeanAbstractSub: %w", err)
		}
		return sub, nil
	case "SupportBeanCopyMethod":
		if err := requireInfraNamedWindowOnUpdateMiscFields(fields, "valOne", "valTwo"); err != nil {
			return nil, err
		}
		var copyMethod infraNamedWindowOnUpdateMiscCopyMethodBean
		if err := json.Unmarshal(step.Payload, &copyMethod); err != nil {
			return nil, fmt.Errorf("decode SupportBeanCopyMethod: %w", err)
		}
		return copyMethod, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNamedWindowOnUpdateMiscID, step.EventType)
	}
}

func requireInfraNamedWindowOnUpdateMiscFields(object map[string]json.RawMessage, names ...string) error {
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

func validateInfraNamedWindowOnUpdateMiscStringArray(raw json.RawMessage, expected []string, name string) error {
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
