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
	infraNWTableStartStopID          = "infra-nwtable-start-stop"
	infraNWTableStartStopDescription = "InfraNWTableStartStop consumer and inserter start/stop lifecycles over a keepall named window and over a primary-key table: named-window istream listeners with silent table listeners, mid-case undeploy and redeploy of the insert and select statements resolved by statement name, iterator snapshots of the infra contents after each start/stop transition, and the redeployed consumer immediately observing current window contents (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableStartStop.java)."
	infraNWTableStartStopJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableStartStopSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableStartStop.java"

	infraNWTableStartStopCreateNW       = "@name('create') @public create window MyInfra#keepall as select theString as a, intPrimitive as b from SupportBean"
	infraNWTableStartStopSelectConsumer = "@Name('select') select a, b from MyInfra as s1"
	infraNWTableStartStopSelectInserter = "@name('select') select a, b from MyInfra as s1"
	infraNWTableStartStopCreateTbl      = "@name('create') @public create table MyInfra(a string primary key, b int primary key)"
	infraNWTableStartStopInsert         = "@name('insert') insert into MyInfra select theString as a, intPrimitive as b from SupportBean"
	infraNWTableStartStopInsertU        = "insert into MyInfra select theString as a, intPrimitive as b from SupportBean"
)

var (
	infraNWTableStartStopJavaSources = []string{
		infraNWTableStartStopSource,
	}
	infraNWTableStartStopJavaRuntimeIDs = []string{
		"java-runtime-e14796f7892a57096988",
		"java-runtime-7ef0dbf9f8ea7e311720",
		"java-runtime-6c0691c9d5549b937cfc",
		"java-runtime-683b282c41f311ed8429",
	}
	infraNWTableStartStopJavaExecutions = []string{
		"InfraStartStopConsumer{namedWindow=true}",
		"InfraStartStopConsumer{namedWindow=false}",
		"InfraStartStopInserter{namedWindow=true}",
		"InfraStartStopInserter{namedWindow=false}",
	}
	infraNWTableStartStopJavaStaticIDs = []string{
		"java-2fe58125cb5f265d5fd1",
		"java-2fe58125cb5f265d5fd1",
		"java-c4a5793bec169880d651",
		"java-c4a5793bec169880d651",
	}
	infraNWTableStartStopCases = []string{
		"consumer-nw",
		"consumer-table",
		"inserter-nw",
		"inserter-table",
	}
	infraNWTableStartStopOrdinals  = []int{0, 1, 2, 3}
	infraNWTableStartStopIterSnaps = []int{5, 4, 3, 2}

	// infraNWTableStartStopBeanSends pins the identical E1-E4 send timeline
	// every execution runs, including the sends that land while the insert or
	// select statement is stopped.
	infraNWTableStartStopBeanSends = []struct {
		theString    string
		intPrimitive int
	}{
		{"E1", 1},
		{"E2", 2},
		{"E3", 3},
		{"E4", 4},
	}
)

type infraNWTableStartStopSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func loadInfraNWTableStartStopScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableStartStopID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableStartStopID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableStartStopID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableStartStopID, err)
	}
	if err := requireInfraNWTableStartStopFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableStartStopID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableStartStopID ||
		metadata.Description != infraNWTableStartStopDescription ||
		metadata.JavaCommit != infraNWTableStartStopJavaCommit ||
		metadata.JavaSource != infraNWTableStartStopSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableStartStopID)
	}
	if err := validateInfraNWTableStartStopStringArray(root["javaRuntimes"], infraNWTableStartStopJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableStartStopStringArray(root["javaNames"], infraNWTableStartStopJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableStartStopStringArray(root["javaStaticIds"], infraNWTableStartStopJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableStartStopStringArray(root["javaFlags"], []string{"OBSERVEROPS"}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableStartStopCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", infraNWTableStartStopID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableStartStopFields(object,
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
		if definition.Case != infraNWTableStartStopCases[index] ||
			definition.Ordinal != infraNWTableStartStopOrdinals[index] ||
			definition.RuntimeID != infraNWTableStartStopJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableStartStopJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != infraNWTableStartStopIterSnaps[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableStartStopID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableStartStopID)
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
			if err := requireInfraNWTableStartStopFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableStartStopFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy":
			if err := requireInfraNWTableStartStopFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if object["mode"] != nil {
				if err := requireInfraNWTableStartStopFields(object, "op", "case", "statement", "mode"); err != nil {
					return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
				}
				var modeStep compat.Step
				if err := json.Unmarshal(rawStep, &modeStep); err != nil {
					return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
				}
				if modeStep.Mode != "any" {
					return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot mode %q is not pinned", index, modeStep.Mode)
				}
			} else {
				if err := requireInfraNWTableStartStopFields(object, "op", "case", "statement"); err != nil {
					return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
				}
			}
		case "send":
			if err := requireInfraNWTableStartStopFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableStartStopPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableStartStopFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWTableStartStopScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateInfraNWTableStartStopScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNWTableStartStopID {
		return fmt.Errorf("%s scenario shape is not pinned", infraNWTableStartStopID)
	}
	offset := 0
	for _, caseName := range infraNWTableStartStopCases {
		steps := scenario.Steps[offset:]
		if len(steps) == 0 || steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q marker is not pinned", infraNWTableStartStopID, caseName)
		}
		end := len(steps)
		for i := 1; i < len(steps); i++ {
			if steps[i].Op == "case" {
				end = i
				break
			}
		}
		span := steps[0:end]
		isTable := caseName == "consumer-table" || caseName == "inserter-table"
		isConsumer := caseName == "consumer-nw" || caseName == "consumer-table"
		createEPL := infraNWTableStartStopCreateNW
		if isTable {
			createEPL = infraNWTableStartStopCreateTbl
		}
		insertEPL := infraNWTableStartStopInsert
		if isConsumer {
			insertEPL = infraNWTableStartStopInsertU
		}
		selectEPL := infraNWTableStartStopSelectInserter
		if isConsumer {
			selectEPL = infraNWTableStartStopSelectConsumer
		}
		deployCount := 0
		snapCount := 0
		undeployCount := 0
		beanIndex := 0
		for _, step := range span {
			switch step.Op {
			case "deploy":
				deployCount++
				switch step.Statement {
				case "create":
					if step.Epl != createEPL {
						return fmt.Errorf("%s case %q create EPL is not pinned", infraNWTableStartStopID, caseName)
					}
				case "insert":
					if step.Epl != insertEPL {
						return fmt.Errorf("%s case %q insert EPL is not pinned", infraNWTableStartStopID, caseName)
					}
				case "select":
					if step.Epl != selectEPL {
						return fmt.Errorf("%s case %q select EPL is not pinned", infraNWTableStartStopID, caseName)
					}
				default:
					return fmt.Errorf("%s case %q unexpected deploy statement %q", infraNWTableStartStopID, caseName, step.Statement)
				}
			case "send":
				payload, err := decodeInfraNWTableStartStopPayload(step)
				if err != nil {
					return fmt.Errorf("%s case %q: %w", infraNWTableStartStopID, caseName, err)
				}
				bean, ok := payload.(infraNWTableStartStopSupportBean)
				if !ok {
					return fmt.Errorf("%s case %q send %d is not a SupportBean", infraNWTableStartStopID, caseName, beanIndex)
				}
				if beanIndex >= len(infraNWTableStartStopBeanSends) {
					return fmt.Errorf("%s case %q has an extra SupportBean send", infraNWTableStartStopID, caseName)
				}
				want := infraNWTableStartStopBeanSends[beanIndex]
				if bean.TheString != want.theString || bean.IntPrimitive != want.intPrimitive {
					return fmt.Errorf("%s case %q SupportBean payload %d is not pinned", infraNWTableStartStopID, caseName, beanIndex)
				}
				beanIndex++
			case "snapshot":
				snapCount++
			case "case":
				// Marker included in the span; nothing to validate.
			case "undeploy":
				undeployCount++
			case "undeploy-all":
			default:
				return fmt.Errorf("%s case %q has unsupported op %q", infraNWTableStartStopID, caseName, step.Op)
			}
		}
		if beanIndex != len(infraNWTableStartStopBeanSends) {
			return fmt.Errorf("%s case %q has %d SupportBean sends, want %d", infraNWTableStartStopID, caseName, beanIndex, len(infraNWTableStartStopBeanSends))
		}
		wantIterSnaps := infraNWTableStartStopIterSnaps[0]
		for index, name := range infraNWTableStartStopCases {
			if name == caseName {
				wantIterSnaps = infraNWTableStartStopIterSnaps[index]
			}
		}
		if snapCount != wantIterSnaps {
			return fmt.Errorf("%s case %q has %d snapshots, want %d", infraNWTableStartStopID, caseName, snapCount, wantIterSnaps)
		}
		if deployCount+undeployCount == 0 {
			return fmt.Errorf("%s case %q has no lifecycle transitions", infraNWTableStartStopID, caseName)
		}
		offset += end
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableStartStopID)
	}
	return nil
}

func runInfraNWTableStartStopScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNWTableStartStopScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableStartStopCases {
		end := len(scenario.Steps)
		for i := offset + 1; i < len(scenario.Steps); i++ {
			if scenario.Steps[i].Op == "case" {
				end = i
				break
			}
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableStartStopCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableStartStopID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runInfraNWTableStartStopCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableStartStopSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	isTable := caseName == "consumer-table" || caseName == "inserter-table"
	projection, newSchemaErr := esper.NewMapSchema("MyInfraStartStopSchema", []esper.FieldSpec{
		esper.FieldDef("a", reflect.TypeOf("")),
		esper.FieldDef("b", reflect.TypeOf(0)),
	})
	if newSchemaErr != nil {
		return compat.Trace{}, newSchemaErr
	}
	if err := env.RegisterSchema(projection); err != nil {
		return compat.Trace{}, err
	}
	if isTable {
		if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("a"),
			esper.PrimaryKeyColumn[int]("b"),
		}); err != nil {
			return compat.Trace{}, err
		}
	} else {
		if _, err := esper.CreateNamedWindow(env, "MyInfra", projection, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableStartStopJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableStartStopID}
	sequence := map[string]uint64{"create": 0, "select": 0}
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
	recordSnapshot := func(statement string, result esper.QueryResult) {
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "snapshot",
			Statement: statement,
			Sequence:  0,
			Time:      compat.FormatTraceTime(time.Unix(0, 0).UTC()),
			New:       compat.NormalizeResults(result.Batch.New),
		})
	}

	// Deployed statements tracked by scenario label; insert is unlistened.
	deployments := map[string]*esper.Deployment{}
	statements := map[string]*esper.Statement{}

	buildAndDeploy := func(label string) error {
		var plan esper.Plan
		var err error
		switch label {
		case "create":
			if isTable {
				plan, err = env.Build(esper.FromTable(env, "MyInfra").Query(esper.StatementName("create")))
			} else {
				plan, err = env.Build(esper.FromNamedWindow(env, "MyInfra").
					Query(esper.StatementName("create"), esper.WithOldStream()))
			}
		case "insert":
			source := esper.From[infraNWTableStartStopSupportBean](env, "SupportBean")
			assignments := []esper.TableAssignment{
				esper.SetColumn("a", esper.Field[infraNWTableStartStopSupportBean, string]("theString")),
				esper.SetColumn("b", esper.Field[infraNWTableStartStopSupportBean, int]("intPrimitive")),
			}
			if isTable {
				plan, err = env.Build(esper.OnEvent(source).InsertIntoTable("MyInfra", assignments...).Query(esper.StatementName("insert")))
			} else {
				plan, err = env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfra", assignments...).Query(esper.StatementName("insert")))
			}
		case "select":
			if isTable {
				plan, err = env.Build(esper.FromTable(env, "MyInfra").Query(esper.StatementName("select")))
			} else {
				plan, err = env.Build(esper.FromNamedWindow(env, "MyInfra").Query(esper.StatementName("select")))
			}
		}
		if err != nil {
			return err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		deployments[label] = deployment
		for _, statement := range deployment.Statements() {
			if statement.Name() == label && label != "insert" {
				name := label
				statements[name] = statement
				listened := statement
				if _, err := listened.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					record(name, batch)
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
		delete(statements, label)
		return nil
	}

	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			if err := buildAndDeploy(step.Statement); err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
		case "undeploy":
			if err := undeployLabel(step.Statement); err != nil {
				return compat.Trace{}, fmt.Errorf("undeploy %q: %w", step.Statement, err)
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
		case "send":
			payload, err := decodeInfraNWTableStartStopPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
		}
	}
	return trace, nil
}

func decodeInfraNWTableStartStopPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWTableStartStopFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean infraNWTableStartStopSupportBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWTableStartStopID, step.EventType)
	}
}

func requireInfraNWTableStartStopFields(object map[string]json.RawMessage, names ...string) error {
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

func validateInfraNWTableStartStopStringArray(raw json.RawMessage, expected []string, name string) error {
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
