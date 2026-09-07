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

const (
	infraNWTableSubqCorrelCoerceID          = "infra-nwtable-subq-correl-coerce"
	infraNWTableSubqCorrelCoerceDescription = "InfraNWTableSubqCorrelCoerce correlated scalar subqueries with int-to-long coercion: the col2 = es.e2 string equality and col1 = es.e1 coerced equality probe a keepall named window or a primary-key table through implicit subquery indexes, shared indexes via the enable_window_subquery_indexshare create hint, disable_window_subquery_indexshare consumer hints, and explicit two-key create index MyIndex (col2, col1), including a mid-case statement undeploy and redeploy proving the subquery index repopulates from existing infra contents (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqCorrelCoerce.java)."
	infraNWTableSubqCorrelCoerceJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableSubqCorrelCoerceSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqCorrelCoerce.java"

	infraNWTableSubqCorrelCoerceSchemaEvent  = "@public @buseventtype create schema EventSchema(e0 string, e1 int, e2 string)"
	infraNWTableSubqCorrelCoerceSchemaWindow = "@public @buseventtype create schema WindowSchema(col0 string, col1 long, col2 string)"
	infraNWTableSubqCorrelCoerceCreateNW     = "@public create window MyInfra#keepall as WindowSchema"
	infraNWTableSubqCorrelCoerceCreateNWES   = "@Hint('enable_window_subquery_indexshare') " + infraNWTableSubqCorrelCoerceCreateNW
	infraNWTableSubqCorrelCoerceCreateTbl    = "@public create table MyInfra (col0 string primary key, col1 long, col2 string)"
	infraNWTableSubqCorrelCoerceInsert       = "insert into MyInfra select * from WindowSchema"
	infraNWTableSubqCorrelCoerceIndex        = "@name('index') create index MyIndex on MyInfra (col2, col1)"
	infraNWTableSubqCorrelCoerceConsume      = "@name('s0') select e0, (select col0 from MyInfra where col2 = es.e2 and col1 = es.e1) as val from EventSchema es"
	infraNWTableSubqCorrelCoerceConsumeHint  = "@Hint('disable_window_subquery_indexshare') " + infraNWTableSubqCorrelCoerceConsume
)

var (
	infraNWTableSubqCorrelCoerceJavaSources = []string{
		infraNWTableSubqCorrelCoerceSource,
	}
	infraNWTableSubqCorrelCoerceJavaRuntimeIDs = []string{
		"java-runtime-0f3833a87dcd9b209935",
		"java-runtime-f1f621deec2314404f2a",
		"java-runtime-4f850a69b870e08ebfd5",
		"java-runtime-1616d20fc73fe4be679c",
		"java-runtime-0422895f8568f52e0956",
		"java-runtime-9161da818506b8194079",
		"java-runtime-3a94283dcac4e9ae4dea",
		"java-runtime-ce6e53f117adccdf9f59",
	}
	infraNWTableSubqCorrelCoerceJavaExecutions = []string{
		"InfraNWTableSubqCorrelCoerceSimple{namedWindow=true, enableIndexShareCreate=false, disableIndexShareConsumer=false, createExplicitIndex=false}",
		"InfraNWTableSubqCorrelCoerceSimple{namedWindow=true, enableIndexShareCreate=false, disableIndexShareConsumer=false, createExplicitIndex=true}",
		"InfraNWTableSubqCorrelCoerceSimple{namedWindow=true, enableIndexShareCreate=true, disableIndexShareConsumer=false, createExplicitIndex=false}",
		"InfraNWTableSubqCorrelCoerceSimple{namedWindow=true, enableIndexShareCreate=true, disableIndexShareConsumer=false, createExplicitIndex=true}",
		"InfraNWTableSubqCorrelCoerceSimple{namedWindow=true, enableIndexShareCreate=true, disableIndexShareConsumer=true, createExplicitIndex=false}",
		"InfraNWTableSubqCorrelCoerceSimple{namedWindow=true, enableIndexShareCreate=true, disableIndexShareConsumer=true, createExplicitIndex=true}",
		"InfraNWTableSubqCorrelCoerceSimple{namedWindow=false, enableIndexShareCreate=false, disableIndexShareConsumer=false, createExplicitIndex=false}",
		"InfraNWTableSubqCorrelCoerceSimple{namedWindow=false, enableIndexShareCreate=false, disableIndexShareConsumer=false, createExplicitIndex=true}",
	}
	infraNWTableSubqCorrelCoerceJavaStaticIDs = []string{
		"java-5a8fbdfa647a5bef3660",
		"java-5a8fbdfa647a5bef3660",
		"java-5a8fbdfa647a5bef3660",
		"java-5a8fbdfa647a5bef3660",
		"java-5a8fbdfa647a5bef3660",
		"java-5a8fbdfa647a5bef3660",
		"java-5a8fbdfa647a5bef3660",
		"java-5a8fbdfa647a5bef3660",
	}
	infraNWTableSubqCorrelCoerceCases = []string{
		"nw-no-share",
		"nw-no-share-index",
		"nw-share",
		"nw-share-index",
		"nw-share-disable",
		"nw-share-disable-index",
		"table-no-share",
		"table-no-share-index",
	}
	infraNWTableSubqCorrelCoerceOrdinals = []int{0, 1, 2, 3, 4, 5, 6, 7}
)

type infraNWTableSubqCorrelCoerceEvent struct {
	E0 string `esper:"e0"`
	E1 int    `esper:"e1"`
	E2 string `esper:"e2"`
}

type infraNWTableSubqCorrelCoerceWindow struct {
	Col0 string `esper:"col0"`
	Col1 int64  `esper:"col1"`
	Col2 string `esper:"col2"`
}

func loadInfraNWTableSubqCorrelCoerceScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableSubqCorrelCoerceID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableSubqCorrelCoerceID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableSubqCorrelCoerceID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableSubqCorrelCoerceID, err)
	}
	if err := requireInfraNWTableSubqCorrelCoerceFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableSubqCorrelCoerceID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableSubqCorrelCoerceID ||
		metadata.Description != infraNWTableSubqCorrelCoerceDescription ||
		metadata.JavaCommit != infraNWTableSubqCorrelCoerceJavaCommit ||
		metadata.JavaSource != infraNWTableSubqCorrelCoerceSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableSubqCorrelCoerceID)
	}
	if err := validateInfraNWTableSubqCorrelCoerceStringArray(root["javaRuntimes"], infraNWTableSubqCorrelCoerceJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqCorrelCoerceStringArray(root["javaNames"], infraNWTableSubqCorrelCoerceJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqCorrelCoerceStringArray(root["javaStaticIds"], infraNWTableSubqCorrelCoerceJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqCorrelCoerceStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableSubqCorrelCoerceCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly eight cases", infraNWTableSubqCorrelCoerceID)
	}
	consumeEPL := func(caseIndex int) string {
		if caseIndex == 4 || caseIndex == 5 {
			return infraNWTableSubqCorrelCoerceConsumeHint
		}
		return infraNWTableSubqCorrelCoerceConsume
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableSubqCorrelCoerceFields(object,
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
		if definition.Case != infraNWTableSubqCorrelCoerceCases[index] ||
			definition.Ordinal != infraNWTableSubqCorrelCoerceOrdinals[index] ||
			definition.RuntimeID != infraNWTableSubqCorrelCoerceJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableSubqCorrelCoerceJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != consumeEPL(index) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableSubqCorrelCoerceID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableSubqCorrelCoerceID)
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
			if err := requireInfraNWTableSubqCorrelCoerceFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableSubqCorrelCoerceFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableSubqCorrelCoerceFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableSubqCorrelCoercePayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy":
			if err := requireInfraNWTableSubqCorrelCoerceFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableSubqCorrelCoerceFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWTableSubqCorrelCoerceScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// infraNWTableSubqCorrelCoerceCreateEPL pins the create statement per case.
func infraNWTableSubqCorrelCoerceCreateEPL(caseName string) string {
	switch caseName {
	case "nw-no-share", "nw-no-share-index":
		return infraNWTableSubqCorrelCoerceCreateNW
	case "nw-share", "nw-share-index", "nw-share-disable", "nw-share-disable-index":
		return infraNWTableSubqCorrelCoerceCreateNWES
	default:
		return infraNWTableSubqCorrelCoerceCreateTbl
	}
}

func infraNWTableSubqCorrelCoerceIsTable(caseName string) bool {
	return caseName == "table-no-share" || caseName == "table-no-share-index"
}

func validateInfraNWTableSubqCorrelCoerceScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNWTableSubqCorrelCoerceID {
		return fmt.Errorf("%s scenario shape is not pinned", infraNWTableSubqCorrelCoerceID)
	}
	type expectedStep struct {
		op        string
		statement string
		epl       string
		send      compat.Step
	}
	windowSends := []compat.Step{
		{EventType: "WindowSchema", Payload: json.RawMessage(`{"col0": "W1", "col1": 10, "col2": "c31"}`)},
		{EventType: "WindowSchema", Payload: json.RawMessage(`{"col0": "W2", "col1": 11, "col2": "c32"}`)},
		{EventType: "WindowSchema", Payload: json.RawMessage(`{"col0": "W3", "col1": 11, "col2": "c31"}`)},
		{EventType: "WindowSchema", Payload: json.RawMessage(`{"col0": "W4", "col1": 10, "col2": "c32"}`)},
	}
	eventSends := []compat.Step{
		{EventType: "EventSchema", Payload: json.RawMessage(`{"e0": "E1", "e1": 10, "e2": "c31"}`)},
		{EventType: "EventSchema", Payload: json.RawMessage(`{"e0": "E2", "e1": 11, "e2": "c32"}`)},
		{EventType: "EventSchema", Payload: json.RawMessage(`{"e0": "E3", "e1": 11, "e2": "c32"}`)},
		{EventType: "EventSchema", Payload: json.RawMessage(`{"e0": "E4", "e1": 11, "e2": "c31"}`)},
		{EventType: "EventSchema", Payload: json.RawMessage(`{"e0": "E5", "e1": 10, "e2": "c31"}`)},
		{EventType: "EventSchema", Payload: json.RawMessage(`{"e0": "E6", "e1": 10, "e2": "c32"}`)},
	}
	// Send order: W1, E1, E2, W2, E3, W3, W4, E4, E5, E6, then the
	// redeployed-consumer E6 replay (pinned separately below).
	sendOrder := []compat.Step{
		windowSends[0], eventSends[0], eventSends[1], windowSends[1], eventSends[2],
		windowSends[2], windowSends[3], eventSends[3], eventSends[4], eventSends[5],
	}
	offset := 0
	for caseIndex, caseName := range infraNWTableSubqCorrelCoerceCases {
		hasIndex := caseName == "nw-no-share-index" || caseName == "nw-share-index" ||
			caseName == "nw-share-disable-index" || caseName == "table-no-share-index"
		consumeEPL := infraNWTableSubqCorrelCoerceConsume
		if caseIndex == 4 || caseIndex == 5 {
			consumeEPL = infraNWTableSubqCorrelCoerceConsumeHint
		}
		deployEPLs := map[string]string{
			"c1":      infraNWTableSubqCorrelCoerceSchemaEvent,
			"c2":      infraNWTableSubqCorrelCoerceSchemaWindow,
			"create":  infraNWTableSubqCorrelCoerceCreateEPL(caseName),
			"insert":  infraNWTableSubqCorrelCoerceInsert,
			"index":   infraNWTableSubqCorrelCoerceIndex,
			"consume": consumeEPL,
		}
		expected := []expectedStep{
			{op: "deploy", statement: "c1", epl: deployEPLs["c1"]},
			{op: "deploy", statement: "c2", epl: deployEPLs["c2"]},
			{op: "deploy", statement: "create", epl: deployEPLs["create"]},
			{op: "deploy", statement: "insert", epl: deployEPLs["insert"]},
		}
		if hasIndex {
			expected = append(expected, expectedStep{op: "deploy", statement: "index", epl: deployEPLs["index"]})
		}
		// The consume statement deploys before any rows exist, unlike the
		// filtered_correl sibling.
		expected = append(expected, expectedStep{op: "deploy", statement: "consume", epl: consumeEPL})
		for _, send := range sendOrder {
			expected = append(expected, expectedStep{op: "send", send: send})
		}
		expected = append(expected,
			expectedStep{op: "undeploy", statement: "s0"},
			expectedStep{op: "deploy", statement: "consume", epl: consumeEPL},
			expectedStep{op: "send", send: eventSends[5]},
			expectedStep{op: "undeploy", statement: "s0"},
		)
		if hasIndex {
			expected = append(expected, expectedStep{op: "undeploy", statement: "index"})
		}
		expected = append(expected, expectedStep{op: "undeploy-all"})
		steps := scenario.Steps[offset : offset+1+len(expected)]
		offset += 1 + len(expected)
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", infraNWTableSubqCorrelCoerceID, caseName)
		}
		for index, want := range expected {
			step := steps[index+1]
			if step.Op != want.op || step.Case != caseName {
				return fmt.Errorf("%s case %q step %d must be op %q in order", infraNWTableSubqCorrelCoerceID, caseName, index, want.op)
			}
			switch want.op {
			case "deploy":
				if step.Statement != want.statement || step.Epl != want.epl {
					return fmt.Errorf("%s case %q step %d deploy %q is not pinned", infraNWTableSubqCorrelCoerceID, caseName, index, want.statement)
				}
			case "send":
				if step.EventType != want.send.EventType {
					return fmt.Errorf("%s case %q step %d must send %q", infraNWTableSubqCorrelCoerceID, caseName, index, want.send.EventType)
				}
				payload, err := decodeInfraNWTableSubqCorrelCoercePayload(step)
				if err != nil {
					return fmt.Errorf("%s case %q step %d: %w", infraNWTableSubqCorrelCoerceID, caseName, index, err)
				}
				var wantPayload struct {
					Col0 string `json:"col0"`
					Col1 int    `json:"col1"`
					Col2 string `json:"col2"`
					E0   string `json:"e0"`
					E1   int    `json:"e1"`
					E2   string `json:"e2"`
				}
				if err := json.Unmarshal(want.send.Payload, &wantPayload); err != nil {
					return err
				}
				if bean, ok := payload.(infraNWTableSubqCorrelCoerceWindow); ok {
					if bean.Col0 != wantPayload.Col0 || bean.Col1 != int64(wantPayload.Col1) || bean.Col2 != wantPayload.Col2 {
						return fmt.Errorf("%s case %q step %d WindowSchema payload is not pinned", infraNWTableSubqCorrelCoerceID, caseName, index)
					}
				} else if event, ok := payload.(infraNWTableSubqCorrelCoerceEvent); ok {
					if event.E0 != wantPayload.E0 || event.E1 != wantPayload.E1 || event.E2 != wantPayload.E2 {
						return fmt.Errorf("%s case %q step %d EventSchema payload is not pinned", infraNWTableSubqCorrelCoerceID, caseName, index)
					}
				}
			case "undeploy":
				if step.Statement != want.statement {
					return fmt.Errorf("%s case %q step %d undeploy %q is not pinned", infraNWTableSubqCorrelCoerceID, caseName, index, want.statement)
				}
			case "undeploy-all":
			}
		}
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableSubqCorrelCoerceID)
	}
	return nil
}

func runInfraNWTableSubqCorrelCoerceScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNWTableSubqCorrelCoerceScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableSubqCorrelCoerceCases {
		hasIndex := caseName == "nw-no-share-index" || caseName == "nw-share-index" ||
			caseName == "nw-share-disable-index" || caseName == "table-no-share-index"
		span := 21
		if hasIndex {
			span = 23
		}
		caseSteps := scenario.Steps[offset : offset+span]
		offset += span
		caseTrace, err := runInfraNWTableSubqCorrelCoerceCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableSubqCorrelCoerceID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runInfraNWTableSubqCorrelCoerceCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableSubqCorrelCoerceEvent](env, "EventSchema"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableSubqCorrelCoerceWindow](env, "WindowSchema"); err != nil {
		return compat.Trace{}, err
	}

	isTable := infraNWTableSubqCorrelCoerceIsTable(caseName)
	// Infra must exist before the engine snapshots environment named windows.
	if isTable {
		if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("col0"),
			esper.TableColumnOf[int64]("col1"),
			esper.TableColumnOf[string]("col2"),
		}); err != nil {
			return compat.Trace{}, err
		}
	} else {
		schema, ok := env.Schema("WindowSchema")
		if !ok {
			return compat.Trace{}, fmt.Errorf("WindowSchema schema is missing")
		}
		if _, err := esper.CreateNamedWindow(env, "MyInfra", schema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableSubqCorrelCoerceJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableSubqCorrelCoerceID}
	sequence := uint64(0)
	record := func(batch esper.ResultBatch) {
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
	}

	deployments := map[string]*esper.Deployment{}
	buildAndDeploy := func(label string) error {
		var plan esper.Plan
		var err error
		switch label {
		case "c1", "c2", "create":
			// The schema and create statements are catalog operations,
			// already applied at env level before the engine started.
			return nil
		case "insert":
			assignments := []esper.TableAssignment{
				esper.SetColumn("col0", esper.Field[infraNWTableSubqCorrelCoerceWindow, string]("col0")),
				esper.SetColumn("col1", esper.Field[infraNWTableSubqCorrelCoerceWindow, int64]("col1")),
				esper.SetColumn("col2", esper.Field[infraNWTableSubqCorrelCoerceWindow, string]("col2")),
			}
			if isTable {
				plan, err = env.Build(esper.OnEvent(esper.From[infraNWTableSubqCorrelCoerceWindow](env, "WindowSchema")).
					InsertIntoTable("MyInfra", assignments...).Query(esper.StatementName("insert")))
			} else {
				plan, err = env.Build(esper.OnEvent(esper.From[infraNWTableSubqCorrelCoerceWindow](env, "WindowSchema")).
					InsertIntoNamedWindow("MyInfra", assignments...).Query(esper.StatementName("insert")))
			}
		case "index":
			// The explicit two-key create index is a late catalog operation
			// on the live infra (sibling precedent
			// infra_nwtable_subq_filtered_correl.go); the scenario still
			// pins the exact EPL text. No statement deploys, but the
			// undeploy steps address the index by name, so record an empty
			// deployment marker.
			var indexErr error
			if isTable {
				table, ok := engine.Table("MyInfra")
				if !ok {
					indexErr = fmt.Errorf("MyInfra table is missing")
				} else {
					indexErr = table.CreateIndex("MyIndex", []string{"col2", "col1"}, esper.IndexHash, false)
				}
			} else {
				window, ok := engine.NamedWindow("MyInfra")
				if !ok {
					indexErr = fmt.Errorf("MyInfra named window is missing")
				} else {
					indexErr = window.CreateIndex("MyIndex", []string{"col2", "col1"}, esper.IndexHash, false)
				}
			}
			if indexErr != nil {
				return indexErr
			}
			deployments["index"] = &esper.Deployment{}
			return nil
		case "consume":
			var inner esper.RecordStream
			if infraNWTableSubqCorrelCoerceIsTable(caseName) {
				inner = esper.FromTable(env, "MyInfra")
			} else {
				inner = esper.FromNamedWindow(env, "MyInfra")
			}
			// The col1 = es.e1 coercion (outer int side against the indexed
			// long column) goes through EqualOf's big.Rat comparison; the
			// engine's subquery index probe collector recognizes equal-of
			// and coerces probe values into the indexed column type.
			plan, err = env.Build(esper.Select(
				esper.From[infraNWTableSubqCorrelCoerceEvent](env, "EventSchema"),
				esper.Alias("e0", esper.Field[infraNWTableSubqCorrelCoerceEvent, string]("e0")),
				esper.Alias("val", esper.SubqueryValueWithOptions[string](inner,
					esper.Field[any, string]("col0"),
					esper.SubqueryWhere(esper.And(
						esper.Equal[string](esper.Field[any, string]("col2"), esper.OuterField[string]("e2")),
						esper.EqualOf(esper.Field[any, int64]("col1"), esper.OuterField[int]("e1")))),
					esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))),
			).Query(esper.StatementName("s0")))
		default:
			return fmt.Errorf("unexpected deploy statement %q", label)
		}
		if err != nil {
			return fmt.Errorf("build %q: %w", label, err)
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return fmt.Errorf("deploy %q: %w", label, err)
		}
		deployments[label] = deployment
		if label == "consume" {
			// The consume statement is named 's0' in the Java source; the
			// scenario's undeploy steps address it by that name.
			deployments["s0"] = deployment
		}
		for _, statement := range deployment.Statements() {
			if statement.Name() == "s0" {
				// The Go statement name is s0 (matching the Java @name and
				// the undeploy steps); the deployment label is consume.
				if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					record(batch)
					return nil
				}); err != nil {
					return err
				}
			}
		}
		return nil
	}
	undeployLabel := func(label string) error {
		deployment, ok := deployments[label]
		if !ok {
			return fmt.Errorf("no deployment for statement %q", label)
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return err
		}
		delete(deployments, label)
		return nil
	}

	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			if err := buildAndDeploy(step.Statement); err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
		case "send":
			payload, err := decodeInfraNWTableSubqCorrelCoercePayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy":
			if err := undeployLabel(step.Statement); err != nil {
				return compat.Trace{}, fmt.Errorf("undeploy %q: %w", step.Statement, err)
			}
		case "undeploy-all":
		}
	}
	return trace, nil
}

func decodeInfraNWTableSubqCorrelCoercePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "WindowSchema":
		if err := requireInfraNWTableSubqCorrelCoerceFields(fields, "col0", "col1", "col2"); err != nil {
			return nil, err
		}
		var window infraNWTableSubqCorrelCoerceWindow
		if err := json.Unmarshal(step.Payload, &window); err != nil {
			return nil, fmt.Errorf("decode WindowSchema: %w", err)
		}
		return window, nil
	case "EventSchema":
		if err := requireInfraNWTableSubqCorrelCoerceFields(fields, "e0", "e1", "e2"); err != nil {
			return nil, err
		}
		var event infraNWTableSubqCorrelCoerceEvent
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode EventSchema: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWTableSubqCorrelCoerceID, step.EventType)
	}
}

func requireInfraNWTableSubqCorrelCoerceFields(object map[string]json.RawMessage, names ...string) error {
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

func validateInfraNWTableSubqCorrelCoerceStringArray(raw json.RawMessage, expected []string, name string) error {
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
