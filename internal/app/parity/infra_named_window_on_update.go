package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	infraNamedWindowOnUpdateID          = "infra-named-window-on-update"
	infraNamedWindowOnUpdateDescription = "InfraNamedWindowOnUpdate on-update statements over named windows: intersect and union composite windows with update old/new listener delivery and iterator state, plus multikey-with-array update matching through single-field and two-field int[] deep-equality where clauses, captured from create-statement listeners and window snapshots projected to the Java-asserted fields (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnUpdate.java)."
	infraNamedWindowOnUpdateJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNamedWindowOnUpdateSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnUpdate.java"

	infraNamedWindowOnUpdateIntersectCreate = "@name('create') @public create window MyWindowMDW#unique(theString)#length(2) as select * from SupportBean"
	infraNamedWindowOnUpdateIntersectInsert = "insert into MyWindowMDW select * from SupportBean"
	infraNamedWindowOnUpdateIntersectUpdate = "on SupportBean_A update MyWindowMDW set intPrimitive=intPrimitive*100 where theString=id"
	infraNamedWindowOnUpdateUnionCreate     = "@name('create') @public create window MyWindowMU#unique(theString)#length(2) retain-union as select * from SupportBean"
	infraNamedWindowOnUpdateUnionInsert     = "insert into MyWindowMU select * from SupportBean"
	infraNamedWindowOnUpdateUnionUpdate     = "on SupportBean_A update MyWindowMU mw set mw.intPrimitive=intPrimitive*100 where theString=id"
	infraNamedWindowOnUpdateMultikeyCreate  = "@name('create') @public create window MyWindow#keepall as SupportEventWithManyArray"
	infraNamedWindowOnUpdateMultikeyInsert  = "insert into MyWindow select * from SupportEventWithManyArray"
	// The two multikey update statements share create/insert; only the where
	// clause differs (single array equality vs id-and-array conjunction).
	infraNamedWindowOnUpdateMultikeyArrayUpdate = "on SupportEventWithIntArray as sewia update MyWindow as mw set value = sewia.value where mw.intOne = sewia.array"
	infraNamedWindowOnUpdateMultikeyTwoUpdate   = "on SupportEventWithIntArray as sewia update MyWindow as mw set value = sewia.value where mw.id = sewia.id and mw.intOne = sewia.array"
)

var (
	infraNamedWindowOnUpdateJavaSources = []string{
		infraNamedWindowOnUpdateSource,
	}
	infraNamedWindowOnUpdateJavaRuntimeIDs = []string{
		"java-runtime-f921cf2543cbb10b4150",
		"java-runtime-0947ec873ea298b60209",
		"java-runtime-9870ee394d3099ef85e8",
		"java-runtime-a25a18f2754aeae08015",
	}
	infraNamedWindowOnUpdateJavaExecutions = []string{
		"InfraMultipleDataWindowIntersect",
		"InfraMultipleDataWindowUnion",
		"InfraUpdateMultikeyWArrayPrimitiveArray",
		"InfraUpdateMultikeyWArrayTwoFields",
	}
	infraNamedWindowOnUpdateJavaStaticIDs = []string{
		"java-b0afef60c0bc90fc51d8",
		"java-a5f1d32f26cb39f39266",
		"java-0b7af7284d24833e1753",
		"java-cdea340e2ff2cabfd0f3",
	}
	infraNamedWindowOnUpdateCases = []string{
		"intersect",
		"union",
		"multikey-array",
		"multikey-two-fields",
	}
	// Ordinals are the Java executions() ordinals; ordinals 0/3/4/5 of the
	// source file are deferred (method-call set clauses, subclass typing,
	// copy-method config, wrapper select).
	infraNamedWindowOnUpdateOrdinals = []int{1, 2, 6, 7}
)

type infraNamedWindowOnUpdateBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type infraNamedWindowOnUpdateA struct {
	ID string `esper:"id"`
}

type infraNamedWindowOnUpdateManyArray struct {
	ID     string `esper:"id"`
	IntOne []int  `esper:"intOne"`
	Value  int    `esper:"value"`
}

type infraNamedWindowOnUpdateIntArray struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

func loadInfraNamedWindowOnUpdateScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNamedWindowOnUpdateID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNamedWindowOnUpdateID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNamedWindowOnUpdateID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNamedWindowOnUpdateID, err)
	}
	if err := requireInfraNamedWindowOnUpdateFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNamedWindowOnUpdateID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNamedWindowOnUpdateID ||
		metadata.Description != infraNamedWindowOnUpdateDescription ||
		metadata.JavaCommit != infraNamedWindowOnUpdateJavaCommit ||
		metadata.JavaSource != infraNamedWindowOnUpdateSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNamedWindowOnUpdateID)
	}
	if err := validateInfraNamedWindowOnUpdateStringArray(root["javaRuntimes"], infraNamedWindowOnUpdateJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNamedWindowOnUpdateStringArray(root["javaNames"], infraNamedWindowOnUpdateJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNamedWindowOnUpdateStringArray(root["javaStaticIds"], infraNamedWindowOnUpdateJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNamedWindowOnUpdateStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNamedWindowOnUpdateCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", infraNamedWindowOnUpdateID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNamedWindowOnUpdateFields(object,
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
		if definition.Case != infraNamedWindowOnUpdateCases[index] ||
			definition.Ordinal != infraNamedWindowOnUpdateOrdinals[index] ||
			definition.RuntimeID != infraNamedWindowOnUpdateJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNamedWindowOnUpdateJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 1 ||
			definition.EPL != infraNamedWindowOnUpdateCreateEPL(definition.Case) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNamedWindowOnUpdateID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNamedWindowOnUpdateID)
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
			if err := requireInfraNamedWindowOnUpdateFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNamedWindowOnUpdateFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNamedWindowOnUpdateFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNamedWindowOnUpdatePayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			// The ordered union snapshot omits "mode"; mode-any snapshots
			// carry "mode":"any".
			if len(object) == 3 {
				if err := requireInfraNamedWindowOnUpdateFields(object, "op", "case", "statement"); err != nil {
					return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
				}
			} else {
				if err := requireInfraNamedWindowOnUpdateFields(object, "op", "case", "statement", "mode"); err != nil {
					return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
				}
			}
		case "undeploy-all":
			if err := requireInfraNamedWindowOnUpdateFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNamedWindowOnUpdateScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// infraNamedWindowOnUpdateCreateEPL pins the create statement per case; it is
// also the cases[].epl pin (the observed statement). The @public annotation is
// load-bearing for the Java harness (separate module deploys need cross-module
// window visibility) and is pinned verbatim even though the Go runner applies
// the create as an environment catalog operation.
func infraNamedWindowOnUpdateCreateEPL(caseName string) string {
	switch caseName {
	case "intersect":
		return infraNamedWindowOnUpdateIntersectCreate
	case "union":
		return infraNamedWindowOnUpdateUnionCreate
	default:
		return infraNamedWindowOnUpdateMultikeyCreate
	}
}

func validateInfraNamedWindowOnUpdateScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNamedWindowOnUpdateID {
		return fmt.Errorf("%s scenario shape is not pinned", infraNamedWindowOnUpdateID)
	}
	// Per case the exact step interleaving is pinned. Snapshot mode is "any"
	// except union, whose Java assertion sorts the iterator rows by theString
	// and therefore pins an ordered snapshot.
	type expectedStep struct {
		op        string
		statement string
		epl       string
		eventType string
		sendIndex int
		mode      string
	}
	beanSends := []struct {
		theString    string
		intPrimitive int
	}{{"E1", 2}, {"E2", 3}}
	manySends := map[string][]struct {
		id     string
		intOne []int
	}{
		"multikey-array":      {{"E1", []int{1, 2}}, {"E2", []int{3, 4}}, {"E3", []int{1}}, {"E4", []int{}}},
		"multikey-two-fields": {{"ID1", []int{1, 2}}, {"ID2", []int{3, 4}}, {"ID3", []int{1}}},
	}
	intSends := map[string][]struct {
		id    string
		array []int
		value int
	}{
		"multikey-array":      {{"U1", []int{3, 4}, 10}, {"U2", []int{1}, 11}, {"U3", []int{}, 12}, {"U4", []int{1, 2}, 13}},
		"multikey-two-fields": {{"ID2", []int{3, 4}, 10}, {"ID3", []int{1}, 11}, {"ID1", []int{1, 2}, 12}, {"IDX", []int{1}, 14}, {"ID1", []int{1, 2, 3}, 15}},
	}
	offset := 0
	for _, caseName := range infraNamedWindowOnUpdateCases {
		createEPL := infraNamedWindowOnUpdateCreateEPL(caseName)
		expected := []expectedStep{
			{op: "deploy", statement: "create", epl: createEPL},
			{op: "deploy", statement: "insert"},
			{op: "deploy", statement: "update"},
		}
		snapshotMode := "any"
		switch caseName {
		case "intersect":
			expected[1].epl = infraNamedWindowOnUpdateIntersectInsert
			expected[2].epl = infraNamedWindowOnUpdateIntersectUpdate
			for i := range beanSends {
				expected = append(expected, expectedStep{op: "send", eventType: "SupportBean", sendIndex: i})
			}
			expected = append(expected, expectedStep{op: "send", eventType: "SupportBean_A", sendIndex: 0})
		case "union":
			expected[1].epl = infraNamedWindowOnUpdateUnionInsert
			expected[2].epl = infraNamedWindowOnUpdateUnionUpdate
			snapshotMode = ""
			for i := range beanSends {
				expected = append(expected, expectedStep{op: "send", eventType: "SupportBean", sendIndex: i})
			}
			expected = append(expected, expectedStep{op: "send", eventType: "SupportBean_A", sendIndex: 0})
		case "multikey-array":
			expected[1].epl = infraNamedWindowOnUpdateMultikeyInsert
			expected[2].epl = infraNamedWindowOnUpdateMultikeyArrayUpdate
			for i := range manySends[caseName] {
				expected = append(expected, expectedStep{op: "send", eventType: "SupportEventWithManyArray", sendIndex: i})
			}
			for i := range intSends[caseName] {
				expected = append(expected, expectedStep{op: "send", eventType: "SupportEventWithIntArray", sendIndex: i})
			}
		default:
			expected[1].epl = infraNamedWindowOnUpdateMultikeyInsert
			expected[2].epl = infraNamedWindowOnUpdateMultikeyTwoUpdate
			for i := range manySends[caseName] {
				expected = append(expected, expectedStep{op: "send", eventType: "SupportEventWithManyArray", sendIndex: i})
			}
			for i := range intSends[caseName] {
				expected = append(expected, expectedStep{op: "send", eventType: "SupportEventWithIntArray", sendIndex: i})
			}
		}
		expected = append(expected,
			expectedStep{op: "snapshot", statement: "create", mode: snapshotMode},
			expectedStep{op: "undeploy-all"},
		)
		steps := scenario.Steps[offset : offset+1+len(expected)]
		offset += 1 + len(expected)
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", infraNamedWindowOnUpdateID, caseName)
		}
		beanIndex, manyIndex, intIndex := 0, 0, 0
		for index, want := range expected {
			step := steps[index+1]
			if step.Op != want.op || step.Case != caseName {
				return fmt.Errorf("%s case %q step %d must be op %q in order", infraNamedWindowOnUpdateID, caseName, index+1, want.op)
			}
			switch want.op {
			case "deploy":
				if step.Statement != want.statement || step.Epl != want.epl {
					return fmt.Errorf("%s case %q step %d deploy %q is not pinned", infraNamedWindowOnUpdateID, caseName, index+1, want.statement)
				}
			case "send":
				if step.EventType != want.eventType {
					return fmt.Errorf("%s case %q step %d must send %q", infraNamedWindowOnUpdateID, caseName, index+1, want.eventType)
				}
				payload, err := decodeInfraNamedWindowOnUpdatePayload(step)
				if err != nil {
					return fmt.Errorf("%s case %q step %d: %w", infraNamedWindowOnUpdateID, caseName, index+1, err)
				}
				switch typed := payload.(type) {
				case infraNamedWindowOnUpdateBean:
					wantBean := beanSends[beanIndex]
					if typed.TheString != wantBean.theString || typed.IntPrimitive != wantBean.intPrimitive {
						return fmt.Errorf("%s case %q step %d SupportBean payload is not pinned", infraNamedWindowOnUpdateID, caseName, index+1)
					}
					beanIndex++
				case infraNamedWindowOnUpdateA:
					if typed.ID != "E2" {
						return fmt.Errorf("%s case %q step %d SupportBean_A payload is not pinned", infraNamedWindowOnUpdateID, caseName, index+1)
					}
				case infraNamedWindowOnUpdateManyArray:
					wantMany := manySends[caseName][manyIndex]
					if typed.ID != wantMany.id || !equalInts(typed.IntOne, wantMany.intOne) || typed.Value != 0 {
						return fmt.Errorf("%s case %q step %d SupportEventWithManyArray payload is not pinned", infraNamedWindowOnUpdateID, caseName, index+1)
					}
					manyIndex++
				case infraNamedWindowOnUpdateIntArray:
					wantInt := intSends[caseName][intIndex]
					if typed.ID != wantInt.id || !equalInts(typed.Array, wantInt.array) || typed.Value != wantInt.value {
						return fmt.Errorf("%s case %q step %d SupportEventWithIntArray payload is not pinned", infraNamedWindowOnUpdateID, caseName, index+1)
					}
					intIndex++
				}
			case "snapshot":
				if step.Statement != want.statement || step.Mode != want.mode {
					return fmt.Errorf("%s case %q step %d snapshot is not pinned", infraNamedWindowOnUpdateID, caseName, index+1)
				}
			case "undeploy-all":
			}
		}
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNamedWindowOnUpdateID)
	}
	return nil
}

func equalInts(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func runInfraNamedWindowOnUpdateScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNamedWindowOnUpdateScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNamedWindowOnUpdateCases {
		span := infraNamedWindowOnUpdateCaseSpan(caseName)
		caseSteps := scenario.Steps[offset : offset+span]
		offset += span
		caseTrace, err := runInfraNamedWindowOnUpdateCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNamedWindowOnUpdateID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func infraNamedWindowOnUpdateCaseSpan(caseName string) int {
	switch caseName {
	case "intersect", "union":
		return 9
	default:
		// Both multikey cases carry 3+4 or 3+5 sends around the update.
		return 14
	}
}

func runInfraNamedWindowOnUpdateCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNamedWindowOnUpdateBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNamedWindowOnUpdateA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNamedWindowOnUpdateManyArray](env, "SupportEventWithManyArray"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNamedWindowOnUpdateIntArray](env, "SupportEventWithIntArray"); err != nil {
		return compat.Trace{}, err
	}

	// Infra must exist before the engine snapshots environment named windows.
	isMultikey := caseName == "multikey-array" || caseName == "multikey-two-fields"
	var windowName string
	if isMultikey {
		windowName = "MyWindow"
		manySchema, ok := env.Schema("SupportEventWithManyArray")
		if !ok {
			return compat.Trace{}, fmt.Errorf("SupportEventWithManyArray schema is missing")
		}
		if _, err := esper.CreateNamedWindow(env, windowName, manySchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	} else {
		windowName = "MyWindowMDW"
		if caseName == "union" {
			windowName = "MyWindowMU"
		}
		schema, ok := env.Schema("SupportBean")
		if !ok {
			return compat.Trace{}, fmt.Errorf("SupportBean schema is missing")
		}
		retention := esper.IntersectWindows(
			esper.Unique(esper.Field[infraNamedWindowOnUpdateBean, string]("theString")),
			esper.LengthWindow(2))
		if caseName == "union" {
			retention = esper.UnionWindows(
				esper.Unique(esper.Field[infraNamedWindowOnUpdateBean, string]("theString")),
				esper.LengthWindow(2))
		}
		if _, err := esper.CreateNamedWindow(env, windowName, schema,
			esper.NamedWindowRetention(retention)); err != nil {
			return compat.Trace{}, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNamedWindowOnUpdateJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNamedWindowOnUpdateID}
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

	// Snapshot rows are projected to the fields the Java assertions pin:
	// theString/intPrimitive for the composite-window cases and id/value for
	// the multikey cases. The Java harness sorts union rows by theString, so
	// the union snapshot is emitted in that order (ordered pin); the other
	// snapshots replay in engine order under mode any.
	projection := []string{"theString", "intPrimitive"}
	sortSnapshotByKey := caseName == "union"
	if isMultikey {
		projection = []string{"id", "value"}
	}
	recordSnapshot := func(statement string, result esper.QueryResult) {
		projected := projectRecords(compat.NormalizeResults(result.Batch.New), projection)
		if sortSnapshotByKey {
			// Union pins an ordered snapshot (the Java assertion sorts rows
			// by theString before comparing).
			sort.SliceStable(projected, func(i, j int) bool {
				return fmt.Sprint(projected[i].Fields[projection[0]]) < fmt.Sprint(projected[j].Fields[projection[0]])
			})
		} else {
			// Mode-any snapshots are emitted in the differential protocol's
			// canonical row order (ascending marshaled fields), matching the
			// Java oracle's mode-any emission and keeping the checked-in
			// traces stable against engine iteration order.
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
	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			var plan esper.Plan
			var err error
			switch step.Statement {
			case "create":
				createQuery := esper.FromNamedWindow(env, windowName)
				if !isMultikey {
					plan, err = env.Build(createQuery.Query(esper.StatementName("create"), esper.WithOldStream()))
				} else {
					plan, err = env.Build(createQuery.Query(esper.StatementName("create")))
				}
			case "insert":
				if isMultikey {
					plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateManyArray](env, "SupportEventWithManyArray")).
						InsertIntoNamedWindow(windowName, esper.CopyMatchingFields()).Query(esper.StatementName("insert")))
				} else {
					plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateBean](env, "SupportBean")).
						InsertIntoNamedWindow(windowName, esper.CopyMatchingFields()).Query(esper.StatementName("insert")))
				}
			case "update":
				if isMultikey {
					var predicate esper.Expression[bool]
					if caseName == "multikey-array" {
						predicate = esper.EqualOf(
							esper.NamedWindowField[[]int]("intOne"),
							esper.Field[infraNamedWindowOnUpdateIntArray, []int]("array"))
					} else {
						predicate = esper.And(
							esper.Equal[string](
								esper.NamedWindowField[string]("id"),
								esper.Field[infraNamedWindowOnUpdateIntArray, string]("id")),
							esper.EqualOf(
								esper.NamedWindowField[[]int]("intOne"),
								esper.Field[infraNamedWindowOnUpdateIntArray, []int]("array")))
					}
					plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateIntArray](env, "SupportEventWithIntArray")).
						UpdateNamedWindow(windowName, predicate,
							esper.SetColumn("value", esper.Field[infraNamedWindowOnUpdateIntArray, int]("value"))).
						Query(esper.StatementName("update")))
				} else {
					plan, err = env.Build(esper.OnEvent(esper.From[infraNamedWindowOnUpdateA](env, "SupportBean_A")).
						UpdateNamedWindow(windowName,
							esper.Equal[string](
								esper.NamedWindowField[string]("theString"),
								esper.Field[infraNamedWindowOnUpdateA, string]("id")),
							esper.SetColumn("intPrimitive",
								esper.Multiply[int](esper.NamedWindowField[int]("intPrimitive"), esper.Literal(100)))).
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
			if step.Statement == "create" && !isMultikey {
				if statement, ok := statements["create"]; ok {
					name := "create"
					if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record(name, batch)
						return nil
					}); err != nil {
						return compat.Trace{}, err
					}
				}
			}
		case "send":
			payload, err := decodeInfraNamedWindowOnUpdatePayload(step)
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

// projectRecords narrows normalized rows to the pinned field set.
func projectRecords(records []compat.ResultRecord, fields []string) []compat.ResultRecord {
	projected := make([]compat.ResultRecord, len(records))
	for i, record := range records {
		fieldsMap := make(map[string]any, len(fields))
		for _, name := range fields {
			fieldsMap[name] = record.Fields[name]
		}
		projected[i] = compat.ResultRecord{Kind: record.Kind, Fields: fieldsMap}
	}
	return projected
}

func decodeInfraNamedWindowOnUpdatePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNamedWindowOnUpdateFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean infraNamedWindowOnUpdateBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_A":
		if err := requireInfraNamedWindowOnUpdateFields(fields, "id"); err != nil {
			return nil, err
		}
		var a infraNamedWindowOnUpdateA
		if err := json.Unmarshal(step.Payload, &a); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return a, nil
	case "SupportEventWithManyArray":
		if err := requireInfraNamedWindowOnUpdateFields(fields, "id", "intOne"); err != nil {
			return nil, err
		}
		var many infraNamedWindowOnUpdateManyArray
		if err := json.Unmarshal(step.Payload, &many); err != nil {
			return nil, fmt.Errorf("decode SupportEventWithManyArray: %w", err)
		}
		return many, nil
	case "SupportEventWithIntArray":
		if err := requireInfraNamedWindowOnUpdateFields(fields, "id", "array", "value"); err != nil {
			return nil, err
		}
		var single infraNamedWindowOnUpdateIntArray
		if err := json.Unmarshal(step.Payload, &single); err != nil {
			return nil, fmt.Errorf("decode SupportEventWithIntArray: %w", err)
		}
		return single, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNamedWindowOnUpdateID, step.EventType)
	}
}

func requireInfraNamedWindowOnUpdateFields(object map[string]json.RawMessage, names ...string) error {
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

func validateInfraNamedWindowOnUpdateStringArray(raw json.RawMessage, expected []string, name string) error {
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
