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

const (
	viewIntersectID          = "view-intersect"
	viewIntersectJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	viewIntersectSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewIntersect.java"
	viewIntersectDescription = "ViewIntersect ordinals 6/14/15/16/17/18: intersect composition with pattern, grouped time+unique+sort, subselect membership, firstunique+firstlength on-delete, and named-window time+unique retention with delete and expiry."

	viewIntersectPatternRuntimeID               = "java-runtime-edb2cfcf01622829e41e"
	viewIntersectGroupTimeUniqueRuntimeID       = "java-runtime-bb2c317ddb0be1894e9b"
	viewIntersectSubselectRuntimeID             = "java-runtime-2c83921c540e86f9eee2"
	viewIntersectFirstUniqueOnDeleteRuntimeID   = "java-runtime-f8abfbe10b8b71648894"
	viewIntersectTimeWinNamedWindowRuntimeID    = "java-runtime-dde215412ea6cd2bb327"
	viewIntersectTimeWinNamedWindowDelRuntimeID = "java-runtime-57b98063fdef9954a959"

	viewIntersectPatternStaticID               = "java-1445f7258ae3c32e2b2a"
	viewIntersectGroupTimeUniqueStaticID       = "java-f8a8a4a254e62d6d6b33"
	viewIntersectSubselectStaticID             = "java-36f61ab5bb30038999eb"
	viewIntersectFirstUniqueOnDeleteStaticID   = "java-2ea9d1646a01c330cc1c"
	viewIntersectTimeWinNamedWindowStaticID    = "java-b113051087b49ee063aa"
	viewIntersectTimeWinNamedWindowDelStaticID = "java-3f0b421dec5e7c91ca42"

	// The pinned EPL texts are the Java source statements verbatim, including
	// the newlines that separate the deployed statements and the stray "\n;"
	// inside ViewIntersectTimeWinNamedWindowDelete.
	viewIntersectPatternEPL               = "@name('s0') select irstream a.p00||b.p10 as theString from pattern [every a=SupportBean_S0 -> b=SupportBean_S1]#unique(a.id)#unique(b.id) retain-intersection"
	viewIntersectGroupTimeUniqueEPL       = "@name('s0') SELECT irstream * FROM SupportSensorEvent#groupwin(type)#time(1 hour)#unique(device)#sort(1, measurement desc) as high order by measurement asc"
	viewIntersectSubselectEPL             = "@name('s0') select * from SupportBean_S0 where p00 in (select theString from SupportBean#length(2)#unique(intPrimitive) retain-intersection)"
	viewIntersectFirstUniqueOnDeleteEPL   = "create window MyWindowOne#firstunique(theString)#firstlength(3) as SupportBean;\ninsert into MyWindowOne select * from SupportBean;\non SupportBean_S0 delete from MyWindowOne where theString = p00;\n@name('s0') select irstream * from MyWindowOne"
	viewIntersectTimeWinNamedWindowEPL    = "@name('s0') create window MyWindowTwo#time(10 sec)#unique(intPrimitive) retain-intersection as select * from SupportBean;\ninsert into MyWindowTwo select * from SupportBean;\non SupportBean_S0 delete from MyWindowTwo where intBoxed = id;\n"
	viewIntersectTimeWinNamedWindowDelEPL = "@name('s0') create window MyWindowThree#time(10 sec)#unique(intPrimitive) retain-intersection as select * from SupportBean;\ninsert into MyWindowThree select * from SupportBean\n;on SupportBean_S0 delete from MyWindowThree where intBoxed = id;\n"
)

type viewIntersectCaseSpec struct {
	name      string
	ordinal   int
	runtimeID string
	staticID  string
	execution string
	epl       string
}

var viewIntersectCaseSpecs = []viewIntersectCaseSpec{
	{name: "pattern", ordinal: 6, runtimeID: viewIntersectPatternRuntimeID, staticID: viewIntersectPatternStaticID, execution: "ViewIntersectPattern", epl: viewIntersectPatternEPL},
	{name: "group-time-unique", ordinal: 14, runtimeID: viewIntersectGroupTimeUniqueRuntimeID, staticID: viewIntersectGroupTimeUniqueStaticID, execution: "ViewIntersectGroupTimeUnique", epl: viewIntersectGroupTimeUniqueEPL},
	{name: "subselect", ordinal: 15, runtimeID: viewIntersectSubselectRuntimeID, staticID: viewIntersectSubselectStaticID, execution: "ViewIntersectSubselect", epl: viewIntersectSubselectEPL},
	{name: "firstunique-length-ondelete", ordinal: 16, runtimeID: viewIntersectFirstUniqueOnDeleteRuntimeID, staticID: viewIntersectFirstUniqueOnDeleteStaticID, execution: "ViewIntersectFirstUniqueAndLengthOnDelete", epl: viewIntersectFirstUniqueOnDeleteEPL},
	{name: "timewin-namedwindow", ordinal: 17, runtimeID: viewIntersectTimeWinNamedWindowRuntimeID, staticID: viewIntersectTimeWinNamedWindowStaticID, execution: "ViewIntersectTimeWinNamedWindow", epl: viewIntersectTimeWinNamedWindowEPL},
	{name: "timewin-namedwindow-delete", ordinal: 18, runtimeID: viewIntersectTimeWinNamedWindowDelRuntimeID, staticID: viewIntersectTimeWinNamedWindowDelStaticID, execution: "ViewIntersectTimeWinNamedWindowDelete", epl: viewIntersectTimeWinNamedWindowDelEPL},
}

var (
	viewIntersectJavaSources   = []string{viewIntersectSource}
	viewIntersectJavaStaticIDs = []string{
		viewIntersectPatternStaticID,
		viewIntersectGroupTimeUniqueStaticID,
		viewIntersectSubselectStaticID,
		viewIntersectFirstUniqueOnDeleteStaticID,
		viewIntersectTimeWinNamedWindowStaticID,
		viewIntersectTimeWinNamedWindowDelStaticID,
	}
)

func viewIntersectJavaRuntimeIDs() []string {
	ids := make([]string, 0, len(viewIntersectCaseSpecs))
	for _, spec := range viewIntersectCaseSpecs {
		ids = append(ids, spec.runtimeID)
	}
	return ids
}

func viewIntersectJavaExecutions() []string {
	names := make([]string, 0, len(viewIntersectCaseSpecs))
	for _, spec := range viewIntersectCaseSpecs {
		names = append(names, spec.execution)
	}
	return names
}

// viewIntersectBean mirrors the pinned SupportBean members used by the
// intersect executions: theString/intPrimitive for the plain cases plus the
// nullable intBoxed that the named-window delete predicate compares against
// SupportBean_S0.id.
type viewIntersectBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int   `esper:"intBoxed"`
}

// viewIntersectS0 mirrors SupportBean_S0 (id, p00); p00 stays a plain string
// because the subselect predicate compares it against theString values.
type viewIntersectS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// viewIntersectS1 mirrors SupportBean_S1 (id, p10).
type viewIntersectS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

// viewIntersectSensor mirrors SupportSensorEvent (id, type, device,
// measurement, confidence) used by the grouped time+unique+sort case.
type viewIntersectSensor struct {
	ID          int     `esper:"id"`
	Type        string  `esper:"type"`
	Device      string  `esper:"device"`
	Measurement float64 `esper:"measurement"`
	Confidence  float64 `esper:"confidence"`
}

// viewIntersectStepSpec pins every scenario step in order; the loader rejects
// any drift before the runtime is touched.
type viewIntersectStepSpec struct {
	op        string
	caseName  string
	kind      string
	statement string
	at        string
	mode      string
	payload   map[string]any
}

func viewIntersectStepSpecs() []viewIntersectStepSpec {
	send := func(kind string, payload map[string]any) viewIntersectStepSpec {
		return viewIntersectStepSpec{op: "send", kind: kind, payload: payload}
	}
	adv := func(at string) viewIntersectStepSpec {
		return viewIntersectStepSpec{op: "advance-time", at: at}
	}
	snap := func() viewIntersectStepSpec {
		return viewIntersectStepSpec{op: "snapshot", statement: "s0"}
	}
	snapAny := func() viewIntersectStepSpec {
		return viewIntersectStepSpec{op: "snapshot", statement: "s0", mode: "any"}
	}
	bean := func(theString string, intPrimitive int) map[string]any {
		return map[string]any{"theString": theString, "intPrimitive": intPrimitive}
	}
	beanBoxed := func(theString string, intPrimitive, intBoxed int) map[string]any {
		return map[string]any{"theString": theString, "intPrimitive": intPrimitive, "intBoxed": intBoxed}
	}
	s0 := func(id int, p00 string) map[string]any { return map[string]any{"id": id, "p00": p00} }
	s0id := func(id int) map[string]any { return map[string]any{"id": id} }
	s1 := func(id int, p10 string) map[string]any { return map[string]any{"id": id, "p10": p10} }
	sensor := func(id int, device string, measurement, confidence float64) map[string]any {
		return map[string]any{"id": id, "type": "Temperature", "device": device, "measurement": measurement, "confidence": confidence}
	}

	steps := []viewIntersectStepSpec{{op: "case", caseName: "pattern"}}
	steps = append(steps,
		send("SupportBean_S0", s0(1, "E1")),
		send("SupportBean_S1", s1(2, "E2")),
		snapAny(),
		send("SupportBean_S0", s0(10, "E3")),
		send("SupportBean_S1", s1(20, "E4")),
		snapAny(),
		send("SupportBean_S0", s0(1, "E5")),
		send("SupportBean_S1", s1(2, "E6")),
		snapAny(),
	)

	steps = append(steps, viewIntersectStepSpec{op: "case", caseName: "group-time-unique"})
	steps = append(steps,
		send("SupportSensorEvent", sensor(1, "Device1", 5.0, 96.5)),
		send("SupportSensorEvent", sensor(2, "Device2", 7.0, 98.5)),
		send("SupportSensorEvent", sensor(3, "Device2", 4.0, 99.5)),
		snap(),
	)

	steps = append(steps, viewIntersectStepSpec{op: "case", caseName: "subselect"})
	steps = append(steps,
		send("SupportBean", bean("E1", 1)),
		send("SupportBean", bean("E2", 2)),
		send("SupportBean", bean("E3", 3)),
		send("SupportBean", bean("E4", 2)),
		send("SupportBean", bean("E5", 1)),
		send("SupportBean_S0", s0(1, "E1")),
		send("SupportBean_S0", s0(1, "E2")),
		send("SupportBean_S0", s0(1, "E3")),
		send("SupportBean_S0", s0(1, "E4")),
		send("SupportBean_S0", s0(1, "E5")),
	)

	steps = append(steps, viewIntersectStepSpec{op: "case", caseName: "firstunique-length-ondelete"})
	steps = append(steps,
		send("SupportBean", bean("E1", 1)),
		snapAny(),
		send("SupportBean", bean("E1", 99)),
		snapAny(),
		send("SupportBean", bean("E2", 2)),
		snapAny(),
		send("SupportBean_S0", s0(1, "E1")),
		snapAny(),
		send("SupportBean", bean("E1", 3)),
		snapAny(),
		send("SupportBean", bean("E1", 99)),
		snapAny(),
		send("SupportBean", bean("E3", 3)),
		snapAny(),
		send("SupportBean", bean("E3", 98)),
		snapAny(),
	)

	steps = append(steps, viewIntersectStepSpec{op: "case", caseName: "timewin-namedwindow"})
	steps = append(steps,
		adv("1970-01-01T00:00:00Z"),
		adv("1970-01-01T00:00:01Z"),
		send("SupportBean", bean("E1", 1)),
		snapAny(),
		adv("1970-01-01T00:00:02Z"),
		send("SupportBean", bean("E2", 2)),
		snapAny(),
		adv("1970-01-01T00:00:03Z"),
		send("SupportBean", bean("E3", 1)),
		snapAny(),
		adv("1970-01-01T00:00:04Z"),
		send("SupportBean", bean("E4", 3)),
		send("SupportBean", bean("E5", 3)),
		snapAny(),
		adv("1970-01-01T00:00:11.999Z"),
		adv("1970-01-01T00:00:12Z"),
		snapAny(),
		adv("1970-01-01T00:00:12.999Z"),
		adv("1970-01-01T00:00:13Z"),
		snapAny(),
		adv("1970-01-01T00:00:13.999Z"),
		adv("1970-01-01T00:00:14Z"),
		snapAny(),
	)

	steps = append(steps, viewIntersectStepSpec{op: "case", caseName: "timewin-namedwindow-delete"})
	steps = append(steps,
		adv("1970-01-01T00:00:00Z"),
		adv("1970-01-01T00:00:01Z"),
		send("SupportBean", beanBoxed("E1", 1, 10)),
		snapAny(),
		adv("1970-01-01T00:00:02Z"),
		send("SupportBean", beanBoxed("E2", 2, 20)),
		send("SupportBean_S0", s0id(20)),
		snapAny(),
		adv("1970-01-01T00:00:03Z"),
		send("SupportBean", beanBoxed("E3", 3, 30)),
		send("SupportBean", beanBoxed("E4", 3, 40)),
		snapAny(),
		adv("1970-01-01T00:00:04Z"),
		send("SupportBean", beanBoxed("E5", 4, 50)),
		send("SupportBean", beanBoxed("E6", 4, 50)),
		snapAny(),
		send("SupportBean_S0", s0id(20)),
		send("SupportBean_S0", s0id(50)),
		snapAny(),
		adv("1970-01-01T00:00:10.999Z"),
		adv("1970-01-01T00:00:11Z"),
		snapAny(),
		adv("1970-01-01T00:00:12.999Z"),
		adv("1970-01-01T00:00:13Z"),
		snapAny(),
		adv("1970-01-01T02:46:40Z"),
	)
	return steps
}

func loadViewIntersectScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", viewIntersectID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", viewIntersectID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewIntersectID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewIntersectID, err)
	}
	if err := requireResultSetQueryTypeLocalGroupByFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}

	version, err := decodeResultSetQueryTypeLocalGroupByString(root["version"], "version")
	if err != nil {
		return compat.Scenario{}, err
	}
	id, err := decodeResultSetQueryTypeLocalGroupByString(root["id"], "id")
	if err != nil {
		return compat.Scenario{}, err
	}
	description, err := decodeResultSetQueryTypeLocalGroupByString(root["description"], "description")
	if err != nil {
		return compat.Scenario{}, err
	}
	javaCommit, err := decodeResultSetQueryTypeLocalGroupByString(root["javaCommit"], "javaCommit")
	if err != nil {
		return compat.Scenario{}, err
	}
	javaSource, err := decodeResultSetQueryTypeLocalGroupByString(root["javaSource"], "javaSource")
	if err != nil {
		return compat.Scenario{}, err
	}
	if version != compat.ScenarioVersion || id != viewIntersectID ||
		description != viewIntersectDescription ||
		javaCommit != viewIntersectJavaCommit ||
		javaSource != viewIntersectSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", viewIntersectID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], viewIntersectJavaRuntimeIDs(), "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], viewIntersectJavaExecutions(), "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], viewIntersectJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(viewIntersectCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", viewIntersectID, len(viewIntersectCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var caseObject map[string]json.RawMessage
		if err := strictObject(rawCase, &caseObject); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultSetQueryTypeLocalGroupByFields(caseObject,
			"case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		ordinal, err := decodeResultSetQueryTypeLocalGroupByInteger(caseObject["ordinal"], "ordinal")
		if err != nil {
			return compat.Scenario{}, err
		}
		var caseMeta struct {
			Case          string `json:"case"`
			Ordinal       int    `json:"ordinal"`
			RuntimeID     string `json:"runtimeId"`
			ExecutionName string `json:"executionName"`
			Observation   string `json:"observation"`
			EPL           string `json:"epl"`
		}
		spec := viewIntersectCaseSpecs[index]
		if err := json.Unmarshal(rawCase, &caseMeta); err != nil ||
			caseMeta.Case != spec.name || ordinal != spec.ordinal || caseMeta.Ordinal != ordinal ||
			caseMeta.RuntimeID != spec.runtimeID || caseMeta.ExecutionName != spec.execution ||
			caseMeta.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", viewIntersectID, index)
		}
	}

	specs := viewIntersectStepSpecs()
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(specs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", viewIntersectID, len(specs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := specs[index]
		var expected []string
		switch spec.op {
		case "case":
			expected = []string{"op", "case"}
		case "send":
			expected = []string{"op", "eventType", "payload"}
		case "advance-time":
			expected = []string{"op", "at"}
		case "snapshot":
			expected = []string{"op", "statement"}
		}
		if spec.mode != "" {
			expected = append(expected, "mode")
		}
		if err := requireResultSetQueryTypeLocalGroupByFields(object, expected...); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateViewIntersectScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateViewIntersectScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	specs := viewIntersectStepSpecs()
	if scenario.ID != viewIntersectID || len(scenario.Steps) != len(specs) {
		return fmt.Errorf("%s scenario steps are not pinned", viewIntersectID)
	}
	for index, spec := range specs {
		step := scenario.Steps[index]
		if step.Op != spec.op || step.Mode != spec.mode {
			return fmt.Errorf("%s scenario step %d must be %q with mode %q", viewIntersectID, index, spec.op, spec.mode)
		}
		switch spec.op {
		case "case":
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", viewIntersectID, index, spec.caseName)
			}
		case "send":
			if step.EventType != spec.kind || step.Case != "" {
				return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", viewIntersectID, index, spec.kind)
			}
			if err := validateViewIntersectPayload(step, spec.payload); err != nil {
				return fmt.Errorf("%s scenario step %d: %w", viewIntersectID, index, err)
			}
		case "advance-time":
			if step.At != spec.at || step.Case != "" {
				return fmt.Errorf("%s scenario step %d must advance time to %s", viewIntersectID, index, spec.at)
			}
		case "snapshot":
			if step.Statement != spec.statement || step.Case != "" {
				return fmt.Errorf("%s scenario step %d must snapshot %q", viewIntersectID, index, spec.statement)
			}
		}
	}
	return nil
}

// validateViewIntersectPayload compares the decoded payload against the pinned
// field set and values; JSON numbers decode as float64 through map[string]any.
func validateViewIntersectPayload(step compat.Step, want map[string]any) error {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return err
	}
	var got map[string]any
	if err := json.Unmarshal(step.Payload, &got); err != nil {
		return fmt.Errorf("%s payload must be an object", step.EventType)
	}
	if len(got) != len(want) {
		return fmt.Errorf("%s payload fields are not pinned", step.EventType)
	}
	for key, wantValue := range want {
		gotValue, ok := got[key]
		if !ok {
			return fmt.Errorf("%s payload is missing field %q", step.EventType, key)
		}
		if !viewIntersectPayloadEqual(gotValue, wantValue) {
			return fmt.Errorf("%s payload field %q is not pinned", step.EventType, key)
		}
	}
	return nil
}

func viewIntersectPayloadEqual(got any, want any) bool {
	switch typed := want.(type) {
	case int:
		number, ok := got.(float64)
		return ok && number == float64(typed)
	case float64:
		number, ok := got.(float64)
		return ok && number == typed
	case string:
		text, ok := got.(string)
		return ok && text == typed
	}
	return reflect.DeepEqual(got, want)
}

func runViewIntersectScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateViewIntersectScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, spec := range viewIntersectCaseSpecs {
		caseTrace, err := runViewIntersectCase(ctx, scenario, spec)
		if err != nil {
			return trace, err
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runViewIntersectCase(ctx context.Context, scenario compat.Scenario, spec viewIntersectCaseSpec) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, spec.name)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewIntersectBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[viewIntersectS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[viewIntersectS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[viewIntersectSensor](env, "SupportSensorEvent"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterMap(env, "PatternMatch", []esper.FieldSpec{
		esper.FieldDef("theString", reflect.TypeOf("")),
		esper.FieldDef("a_id", reflect.TypeOf(0)),
		esper.FieldDef("b_id", reflect.TypeOf(0)),
	}); err != nil {
		return compat.Trace{}, err
	}

	plans, err := viewIntersectPlans(env, spec.name)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.DeployPlans(ctx, plans)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := map[string]*esper.Statement{}
	for _, statement := range deployment.Statements() {
		statements[statement.Name()] = statement
	}
	s0 := statements["s0"]
	if s0 == nil {
		return compat.Trace{}, fmt.Errorf("%s case %s did not deploy statement s0", viewIntersectID, spec.name)
	}

	trace := compat.Trace{}
	var sequence uint64
	if _, err := s0.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		sequence++
		record := compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
		}
		record.New = compat.NormalizeResults(batch.New)
		record.Old = compat.NormalizeResults(batch.Old)
		if record.New == nil {
			record.New = []compat.ResultRecord{}
		}
		if record.Old == nil {
			record.Old = []compat.ResultRecord{}
		}
		trace.Records = append(trace.Records, record)
		return nil
	}); err != nil {
		return compat.Trace{}, err
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			if err := viewIntersectSend(ctx, engine, step); err != nil {
				return trace, err
			}
		case "advance-time":
			at, parseErr := time.Parse(time.RFC3339Nano, step.At)
			if parseErr != nil {
				return trace, fmt.Errorf("view-intersect advance time %q: %w", step.At, parseErr)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return trace, err
			}
		case "snapshot":
			result, snapErr := s0.Snapshot(ctx)
			if snapErr != nil {
				return trace, snapErr
			}
			record := compat.TraceRecord{
				Case:      spec.name,
				Operation: "snapshot",
				Statement: step.Statement,
			}
			record.New = compat.NormalizeResults(result.Batch.New)
			if record.New == nil {
				record.New = []compat.ResultRecord{}
			}
			trace.Records = append(trace.Records, record)
		default:
			return trace, fmt.Errorf("unsupported view-intersect step op %q", step.Op)
		}
	}
	return trace, nil
}

func viewIntersectSend(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	var payload map[string]any
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return fmt.Errorf("view-intersect send payload: %w", err)
	}
	switch step.EventType {
	case "SupportBean":
		event := viewIntersectBean{}
		if v, ok := payload["theString"].(string); ok {
			event.TheString = v
		}
		if v, ok := payload["intPrimitive"].(float64); ok {
			event.IntPrimitive = int(v)
		}
		if v, ok := payload["intBoxed"].(float64); ok {
			boxed := int(v)
			event.IntBoxed = &boxed
		}
		return engine.Send(ctx, "SupportBean", event)
	case "SupportBean_S0":
		event := viewIntersectS0{}
		if v, ok := payload["id"].(float64); ok {
			event.ID = int(v)
		}
		if v, ok := payload["p00"].(string); ok {
			event.P00 = v
		}
		return engine.Send(ctx, "SupportBean_S0", event)
	case "SupportBean_S1":
		event := viewIntersectS1{}
		if v, ok := payload["id"].(float64); ok {
			event.ID = int(v)
		}
		if v, ok := payload["p10"].(string); ok {
			event.P10 = v
		}
		return engine.Send(ctx, "SupportBean_S1", event)
	case "SupportSensorEvent":
		event := viewIntersectSensor{}
		if v, ok := payload["id"].(float64); ok {
			event.ID = int(v)
		}
		if v, ok := payload["type"].(string); ok {
			event.Type = v
		}
		if v, ok := payload["device"].(string); ok {
			event.Device = v
		}
		if v, ok := payload["measurement"].(float64); ok {
			event.Measurement = v
		}
		if v, ok := payload["confidence"].(float64); ok {
			event.Confidence = v
		}
		return engine.Send(ctx, "SupportSensorEvent", event)
	default:
		return fmt.Errorf("unknown view-intersect event type %q", step.EventType)
	}
}

// viewIntersectPlans builds the deployed statements for one case. Every case
// deploys the same statement set as the Java execution: the pattern case adds
// a producer insert-into plus the consumer; the named-window cases register
// the window, its insert trigger and its on-delete trigger before the
// create-statement consumer.
func viewIntersectPlans(env *esper.Environment, caseName string) ([]esper.Plan, error) {
	s0 := esper.From[viewIntersectS0](env, "SupportBean_S0")
	s1 := esper.From[viewIntersectS1](env, "SupportBean_S1")
	bean := esper.From[viewIntersectBean](env, "SupportBean")
	sensor := esper.From[viewIntersectSensor](env, "SupportSensorEvent")

	switch caseName {
	case "pattern":
		producer, err := env.Build(esper.PatternFrom(s0, "a", esper.Literal(true)).
			Every().
			Then(esper.PatternFrom(s1, "b", esper.Literal(true))).
			Select(
				esper.Alias("theString", esper.Concat(esper.TagField[string]("a", "p00"), esper.TagField[string]("b", "p10"))),
				esper.Alias("a_id", esper.TagField[int]("a", "id")),
				esper.Alias("b_id", esper.TagField[int]("b", "id")),
			).
			InsertInto("PatternMatch", esper.StatementName("producer")))
		if err != nil {
			return nil, err
		}
		consumer, err := env.Build(esper.FromAny(env, "PatternMatch").
			Window(esper.IntersectWindows(
				esper.Unique(esper.Field[any, int]("a_id")),
				esper.Unique(esper.Field[any, int]("b_id")),
			)).
			Select(esper.Alias("theString", esper.Field[any, string]("theString"))).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{producer, consumer}, nil

	case "group-time-unique":
		plan, err := env.Build(sensor.
			Window(esper.GroupWindow(esper.Field[viewIntersectSensor, string]("type"),
				esper.IntersectWindows(
					esper.TimeWindow(time.Hour),
					esper.Unique(esper.Field[viewIntersectSensor, string]("device")),
					esper.SortWindow(1, esper.Descending(esper.Field[viewIntersectSensor, float64]("measurement"))),
				))).
			AsRecord().
			Query(esper.StatementName("s0"), esper.WithOldStream(),
				esper.OrderBy(esper.Ascending(esper.Field[any, float64]("measurement")))))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil

	case "subselect":
		inner := bean.Window(esper.IntersectWindows(
			esper.LengthWindow(2),
			esper.Unique(esper.Field[viewIntersectBean, int]("intPrimitive")),
		)).AsRecord()
		plan, err := env.Build(s0.
			Filter(esper.SubqueryIn[string](
				esper.Field[viewIntersectS0, string]("p00"),
				inner,
				esper.Field[viewIntersectBean, string]("theString"),
			)).
			AsRecord().
			Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil

	case "firstunique-length-ondelete":
		schema, err := esper.StructSchema[viewIntersectBean]("SupportBean")
		if err != nil {
			return nil, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowOne", schema,
			esper.NamedWindowRetention(esper.IntersectWindows(
				esper.FirstUnique(esper.Field[viewIntersectBean, string]("theString")),
				esper.FirstLength(3),
			))); err != nil {
			return nil, err
		}
		insert, err := env.Build(esper.OnEvent(bean).InsertIntoNamedWindow("MyWindowOne",
			esper.SetColumn("theString", esper.Field[viewIntersectBean, string]("theString")),
			esper.SetColumn("intPrimitive", esper.Field[viewIntersectBean, int]("intPrimitive")),
			esper.SetColumn("intBoxed", esper.Field[viewIntersectBean, *int]("intBoxed")),
		).Query(esper.StatementName("insert")))
		if err != nil {
			return nil, err
		}
		onDelete, err := env.Build(esper.OnEvent(s0).DeleteFromNamedWindow("MyWindowOne",
			esper.Equal[string](esper.NamedWindowField[string]("theString"), esper.Field[viewIntersectS0, string]("p00")),
		).Query(esper.StatementName("on-delete")))
		if err != nil {
			return nil, err
		}
		consumer, err := env.Build(esper.FromNamedWindow(env, "MyWindowOne").
			CreateNamedWindowQuery(esper.StatementName("s0"), esper.WithOldStream()))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insert, onDelete, consumer}, nil

	case "timewin-namedwindow", "timewin-namedwindow-delete":
		windowName := "MyWindowTwo"
		if caseName == "timewin-namedwindow-delete" {
			windowName = "MyWindowThree"
		}
		schema, err := esper.StructSchema[viewIntersectBean]("SupportBean")
		if err != nil {
			return nil, err
		}
		if _, err := esper.CreateNamedWindow(env, windowName, schema,
			esper.NamedWindowRetention(esper.IntersectWindows(
				esper.TimeWindow(10*time.Second),
				esper.Unique(esper.Field[viewIntersectBean, int]("intPrimitive")),
			))); err != nil {
			return nil, err
		}
		insert, err := env.Build(esper.OnEvent(bean).InsertIntoNamedWindow(windowName,
			esper.SetColumn("theString", esper.Field[viewIntersectBean, string]("theString")),
			esper.SetColumn("intPrimitive", esper.Field[viewIntersectBean, int]("intPrimitive")),
			esper.SetColumn("intBoxed", esper.Field[viewIntersectBean, *int]("intBoxed")),
		).Query(esper.StatementName("insert")))
		if err != nil {
			return nil, err
		}
		onDelete, err := env.Build(esper.OnEvent(s0).DeleteFromNamedWindow(windowName,
			esper.Equal[int](esper.NamedWindowField[int]("intBoxed"), esper.Field[viewIntersectS0, int]("id")),
		).Query(esper.StatementName("on-delete")))
		if err != nil {
			return nil, err
		}
		consumer, err := env.Build(esper.FromNamedWindow(env, windowName).
			CreateNamedWindowQuery(esper.StatementName("s0"), esper.WithOldStream()))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insert, onDelete, consumer}, nil
	}
	return nil, fmt.Errorf("unknown view-intersect case %q", caseName)
}
