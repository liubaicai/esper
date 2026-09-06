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
	infraNWTableSubqAtEventBeanID          = "infra-nwtable-subq-at-eventbean"
	infraNWTableSubqAtEventBeanDescription = "InfraNWTableSubqueryAtEventBean uncorrelated select-star subquery (select * from MyInfra) @eventbean over a keepall named window and over a table, capturing the null detail fragment on an empty infra and the growing multi-row detail fragments after insertions from the s0 listener (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqueryAtEventBean.java)."
	infraNWTableSubqAtEventBeanJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableSubqAtEventBeanSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqueryAtEventBean.java"

	infraNWTableSubqAtEventBeanCreateNW  = "@public create window MyInfra#keepall as (c0 string, c1 int)"
	infraNWTableSubqAtEventBeanCreateTbl = "@public create table MyInfra(c0 string primary key, c1 int)"
	infraNWTableSubqAtEventBeanInsert    = "insert into MyInfra select theString as c0, intPrimitive as c1 from SupportBean"
	infraNWTableSubqAtEventBeanSubquery  = "@name('s0') select p00, (select * from MyInfra) @eventbean as detail from SupportBean_S0"
)

var (
	infraNWTableSubqAtEventBeanJavaSources = []string{
		infraNWTableSubqAtEventBeanSource,
	}
	infraNWTableSubqAtEventBeanJavaRuntimeIDs = []string{
		"java-runtime-59604d7dbcef79bacde1",
		"java-runtime-d8e8832ac46005459ffd",
	}
	infraNWTableSubqAtEventBeanJavaExecutions = []string{
		"InfraSubSelStar{namedWindow=true}",
		"InfraSubSelStar{namedWindow=false}",
	}
	infraNWTableSubqAtEventBeanJavaStaticIDs = []string{
		"java-0d99efae7b23e8d8b346",
		"java-0d99efae7b23e8d8b346",
	}
	infraNWTableSubqAtEventBeanCases = []string{
		"nw",
		"table",
	}
	infraNWTableSubqAtEventBeanOrdinals = []int{0, 1}
)

type infraNWTableSubqAtEventBeanS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
}

type infraNWTableSubqAtEventBeanSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func loadInfraNWTableSubqAtEventBeanScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableSubqAtEventBeanID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableSubqAtEventBeanID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableSubqAtEventBeanID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableSubqAtEventBeanID, err)
	}
	if err := requireInfraNWTableSubqAtEventBeanFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableSubqAtEventBeanID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableSubqAtEventBeanID ||
		metadata.Description != infraNWTableSubqAtEventBeanDescription ||
		metadata.JavaCommit != infraNWTableSubqAtEventBeanJavaCommit ||
		metadata.JavaSource != infraNWTableSubqAtEventBeanSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableSubqAtEventBeanID)
	}
	if err := validateInfraNWTableSubqAtEventBeanStringArray(root["javaRuntimes"], infraNWTableSubqAtEventBeanJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqAtEventBeanStringArray(root["javaNames"], infraNWTableSubqAtEventBeanJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqAtEventBeanStringArray(root["javaStaticIds"], infraNWTableSubqAtEventBeanJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqAtEventBeanStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableSubqAtEventBeanCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly two cases", infraNWTableSubqAtEventBeanID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableSubqAtEventBeanFields(object,
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
		if definition.Case != infraNWTableSubqAtEventBeanCases[index] ||
			definition.Ordinal != infraNWTableSubqAtEventBeanOrdinals[index] ||
			definition.RuntimeID != infraNWTableSubqAtEventBeanJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableSubqAtEventBeanJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != infraNWTableSubqAtEventBeanSubquery {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableSubqAtEventBeanID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableSubqAtEventBeanID)
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
			if err := requireInfraNWTableSubqAtEventBeanFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableSubqAtEventBeanFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableSubqAtEventBeanFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableSubqAtEventBeanPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableSubqAtEventBeanFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWTableSubqAtEventBeanScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateInfraNWTableSubqAtEventBeanScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNWTableSubqAtEventBeanID {
		return fmt.Errorf("%s scenario shape is not pinned", infraNWTableSubqAtEventBeanID)
	}
	offset := 0
	for caseIndex, caseName := range infraNWTableSubqAtEventBeanCases {
		steps := scenario.Steps[offset : offset+10]
		offset += 10
		sendCounters := map[string]int{}
		if len(steps) != 10 {
			return fmt.Errorf("%s case %q must contain exactly ten steps, got %d", infraNWTableSubqAtEventBeanID, caseName, len(steps))
		}
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", infraNWTableSubqAtEventBeanID, caseName)
		}
		if steps[len(steps)-1].Op != "undeploy-all" {
			return fmt.Errorf("%s case %q must end with undeploy-all", infraNWTableSubqAtEventBeanID, caseName)
		}
		createEPL := infraNWTableSubqAtEventBeanCreateNW
		if caseIndex == 1 {
			createEPL = infraNWTableSubqAtEventBeanCreateTbl
		}
		deployEPLs := map[string]string{
			"create": createEPL,
			"insert": infraNWTableSubqAtEventBeanInsert,
			"s0":     infraNWTableSubqAtEventBeanSubquery,
		}
		deployCount := 0
		for _, step := range steps[1 : len(steps)-1] {
			switch step.Op {
			case "deploy":
				deployCount++
				if deployCount > 3 {
					return fmt.Errorf("%s case %q has more than three deploys", infraNWTableSubqAtEventBeanID, caseName)
				}
				if step.Epl != deployEPLs[step.Statement] {
					return fmt.Errorf("%s case %q deploy %q is not pinned", infraNWTableSubqAtEventBeanID, caseName, step.Statement)
				}
			case "send":
				if err := validateInfraNWTableSubqAtEventBeanSendValues(caseName, step, sendCounters); err != nil {
					return fmt.Errorf("%s case %q: %w", infraNWTableSubqAtEventBeanID, caseName, err)
				}
			default:
				return fmt.Errorf("%s case %q has unsupported op %q", infraNWTableSubqAtEventBeanID, caseName, step.Op)
			}
		}
		if deployCount != 3 {
			return fmt.Errorf("%s case %q must contain exactly three deploys, got %d", infraNWTableSubqAtEventBeanID, caseName, deployCount)
		}
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableSubqAtEventBeanID)
	}
	return nil
}

// validateInfraNWTableSubqAtEventBeanSendValues pins every send payload so a
// drifted scenario cannot silently replay different values.
func validateInfraNWTableSubqAtEventBeanSendValues(caseName string, step compat.Step, counters map[string]int) error {
	kind := step.EventType
	defer func() { counters[kind]++ }()
	switch kind {
	case "SupportBean_S0":
		var payload struct {
			ID int
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		if payload.ID != 0 {
			return fmt.Errorf("S0 payload %d is not pinned", counters[kind])
		}
	case "SupportBean":
		want := []struct {
			theString string
			primitive int
		}{{"E1", 1}, {"E2", 2}}
		if counters[kind] >= len(want) {
			return fmt.Errorf("unexpected extra SupportBean send %d", counters[kind])
		}
		var payload struct {
			TheString    string
			IntPrimitive int
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		if payload.TheString != want[counters[kind]].theString || payload.IntPrimitive != want[counters[kind]].primitive {
			return fmt.Errorf("SupportBean payload %d is not pinned", counters[kind])
		}
	}
	return nil
}
func runInfraNWTableSubqAtEventBeanScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNWTableSubqAtEventBeanScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableSubqAtEventBeanCases {
		caseSteps := scenario.Steps[offset : offset+10]
		offset += 10
		caseTrace, err := runInfraNWTableSubqAtEventBeanCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableSubqAtEventBeanID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runInfraNWTableSubqAtEventBeanCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableSubqAtEventBeanS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableSubqAtEventBeanSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	isTable := caseName == "table"
	// Infra must exist before the engine snapshots environment named windows.
	projection, err := esper.NewMapSchema("MyInfraSchema", []esper.FieldSpec{
		esper.FieldDef("c0", reflect.TypeOf("")),
		esper.FieldDef("c1", reflect.TypeOf(0)),
	})
	if err2 := env.RegisterSchema(projection); err2 != nil {
		return compat.Trace{}, err
	}
	if isTable {
		if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("c0"),
			esper.TableColumnOf[int]("c1"),
		}); err != nil {
			return compat.Trace{}, err
		}
	} else {
		if _, err := esper.CreateNamedWindow(env, "MyInfra", projection, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableSubqAtEventBeanJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableSubqAtEventBeanID}
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

	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			var plan esper.Plan
			var err error
			switch step.Statement {
			case "create":
				// The create/insert statements are unnamed in the Java source
				// and unlistened; the create is already applied as a catalog
				// operation, so only a query-shaped statement is deployed for
				// the insert.
			case "insert":
				source := esper.From[infraNWTableSubqAtEventBeanSupportBean](env, "SupportBean")
				assignments := []esper.TableAssignment{
					esper.SetColumn("c0", esper.Field[infraNWTableSubqAtEventBeanSupportBean, string]("theString")),
					esper.SetColumn("c1", esper.Field[infraNWTableSubqAtEventBeanSupportBean, int]("intPrimitive")),
				}
				if isTable {
					plan, err = env.Build(esper.OnEvent(source).InsertIntoTable("MyInfra", assignments...).Query(esper.StatementName("insert")))
				} else {
					plan, err = env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfra", assignments...).Query(esper.StatementName("insert")))
				}
			case "s0":
				var inner esper.RecordStream
				if isTable {
					inner = esper.FromTable(env, "MyInfra")
				} else {
					inner = esper.FromNamedWindow(env, "MyInfra")
				}
				plan, err = env.Build(esper.Select(
					esper.From[infraNWTableSubqAtEventBeanS0](env, "SupportBean_S0"),
					esper.Alias("p00", esper.Field[infraNWTableSubqAtEventBeanS0, string]("p00")),
					esper.Alias("detail", esper.SubqueryValueWithOptions[[]esper.Event](inner, esper.WindowEvents())),
				).Query(esper.StatementName("s0")))
			default:
				return compat.Trace{}, fmt.Errorf("unexpected deploy statement %q", step.Statement)
			}
			if step.Statement == "create" {
				continue
			}
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			if step.Statement == "s0" {
				statements := deployment.Statements()
				if len(statements) != 1 || statements[0].Name() != "s0" {
					return compat.Trace{}, fmt.Errorf("expected one s0 statement")
				}
				if _, err := statements[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					record(batch)
					return nil
				}); err != nil {
					return compat.Trace{}, err
				}
			}
		case "send":
			payload, err := decodeInfraNWTableSubqAtEventBeanPayload(step)
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

func decodeInfraNWTableSubqAtEventBeanPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean_S0":
		if err := requireInfraNWTableSubqAtEventBeanFields(fields, "id"); err != nil {
			return nil, err
		}
		var s0 infraNWTableSubqAtEventBeanS0
		if err := json.Unmarshal(step.Payload, &s0); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return s0, nil
	case "SupportBean":
		if err := requireInfraNWTableSubqAtEventBeanFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean infraNWTableSubqAtEventBeanSupportBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWTableSubqAtEventBeanID, step.EventType)
	}
}

func requireInfraNWTableSubqAtEventBeanFields(object map[string]json.RawMessage, names ...string) error {
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

func validateInfraNWTableSubqAtEventBeanStringArray(raw json.RawMessage, expected []string, name string) error {
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
