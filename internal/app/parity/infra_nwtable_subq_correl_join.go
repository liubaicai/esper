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
	infraNWTableSubqCorrelJoinID          = "infra-nwtable-subq-correl-join"
	infraNWTableSubqCorrelJoinDescription = "InfraNWTableSubqCorrelJoin correlated scalar subquery (select intPrimitive from MyInfra where theString = s1.p10) evaluated in a two-stream SupportBean_S0 and SupportBean_S1 lastevent join over a #unique named window and over a table, covering named-window subquery index sharing through the enable_window_subquery_indexshare create hint and join-driven subquery re-evaluation from both join streams captured from the s0 listener (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqCorrelJoin.java)."
	infraNWTableSubqCorrelJoinJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableSubqCorrelJoinSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqCorrelJoin.java"

	infraNWTableSubqCorrelJoinCreateNW   = "@public create window MyInfra#unique(theString) as select * from SupportBean"
	infraNWTableSubqCorrelJoinCreateNWES = "@Hint('enable_window_subquery_indexshare') @public create window MyInfra#unique(theString) as select * from SupportBean"
	infraNWTableSubqCorrelJoinCreateTbl  = "@public create table MyInfra(theString string primary key, intPrimitive int primary key)"
	infraNWTableSubqCorrelJoinInsert     = "insert into MyInfra select theString, intPrimitive from SupportBean"
	infraNWTableSubqCorrelJoinConsume    = "@name('s0') select (select intPrimitive from MyInfra where theString = s1.p10) as val from SupportBean_S0#lastevent as s0, SupportBean_S1#lastevent as s1"
)

var (
	infraNWTableSubqCorrelJoinJavaSources = []string{
		infraNWTableSubqCorrelJoinSource,
	}
	infraNWTableSubqCorrelJoinJavaRuntimeIDs = []string{
		"java-runtime-a1f99ce62c1931e994d8",
		"java-runtime-00bf2cd9669115eb2950",
		"java-runtime-ffb6fe55a1ecde12a134",
	}
	infraNWTableSubqCorrelJoinJavaExecutions = []string{
		"InfraNWTableSubqCorrelJoinAssertion{namedWindow=true, enableIndexShareCreate=false}",
		"InfraNWTableSubqCorrelJoinAssertion{namedWindow=true, enableIndexShareCreate=true}",
		"InfraNWTableSubqCorrelJoinAssertion{namedWindow=false, enableIndexShareCreate=false}",
	}
	infraNWTableSubqCorrelJoinJavaStaticIDs = []string{
		"java-0ce6cd8b5317076e9a5a",
		"java-0ce6cd8b5317076e9a5a",
		"java-0ce6cd8b5317076e9a5a",
	}
	infraNWTableSubqCorrelJoinCases = []string{
		"nw-no-share",
		"nw-index-share",
		"table",
	}
	infraNWTableSubqCorrelJoinOrdinals = []int{0, 1, 2}
)

type infraNWTableSubqCorrelJoinBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type infraNWTableSubqCorrelJoinS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type infraNWTableSubqCorrelJoinS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

func loadInfraNWTableSubqCorrelJoinScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableSubqCorrelJoinID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableSubqCorrelJoinID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableSubqCorrelJoinID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableSubqCorrelJoinID, err)
	}
	if err := requireInfraNWTableSubqCorrelJoinFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableSubqCorrelJoinID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableSubqCorrelJoinID ||
		metadata.Description != infraNWTableSubqCorrelJoinDescription ||
		metadata.JavaCommit != infraNWTableSubqCorrelJoinJavaCommit ||
		metadata.JavaSource != infraNWTableSubqCorrelJoinSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableSubqCorrelJoinID)
	}
	if err := validateInfraNWTableSubqCorrelJoinStringArray(root["javaRuntimes"], infraNWTableSubqCorrelJoinJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqCorrelJoinStringArray(root["javaNames"], infraNWTableSubqCorrelJoinJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqCorrelJoinStringArray(root["javaStaticIds"], infraNWTableSubqCorrelJoinJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqCorrelJoinStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableSubqCorrelJoinCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly three cases", infraNWTableSubqCorrelJoinID)
	}
	consumeEPL := func(caseIndex int) string {
		return infraNWTableSubqCorrelJoinConsume
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableSubqCorrelJoinFields(object,
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
		if definition.Case != infraNWTableSubqCorrelJoinCases[index] ||
			definition.Ordinal != infraNWTableSubqCorrelJoinOrdinals[index] ||
			definition.RuntimeID != infraNWTableSubqCorrelJoinJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableSubqCorrelJoinJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != consumeEPL(index) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableSubqCorrelJoinID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableSubqCorrelJoinID)
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
			if err := requireInfraNWTableSubqCorrelJoinFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableSubqCorrelJoinFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableSubqCorrelJoinFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableSubqCorrelJoinPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableSubqCorrelJoinFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWTableSubqCorrelJoinScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// infraNWTableSubqCorrelJoinCreateEPL pins the create statement per case.
func infraNWTableSubqCorrelJoinCreateEPL(caseName string) string {
	switch caseName {
	case "nw-no-share":
		return infraNWTableSubqCorrelJoinCreateNW
	case "nw-index-share":
		return infraNWTableSubqCorrelJoinCreateNWES
	default:
		return infraNWTableSubqCorrelJoinCreateTbl
	}
}

func validateInfraNWTableSubqCorrelJoinScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNWTableSubqCorrelJoinID {
		return fmt.Errorf("%s scenario shape is not pinned", infraNWTableSubqCorrelJoinID)
	}
	offset := 0
	for _, caseName := range infraNWTableSubqCorrelJoinCases {
		steps := scenario.Steps[offset : offset+13]
		offset += 13
		if len(steps) != 13 {
			return fmt.Errorf("%s case %q must contain exactly thirteen steps, got %d", infraNWTableSubqCorrelJoinID, caseName, len(steps))
		}
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", infraNWTableSubqCorrelJoinID, caseName)
		}
		if steps[len(steps)-1].Op != "undeploy-all" {
			return fmt.Errorf("%s case %q must end with undeploy-all", infraNWTableSubqCorrelJoinID, caseName)
		}
		createEPL := infraNWTableSubqCorrelJoinCreateEPL(caseName)
		deployEPLs := map[string]string{
			"create": createEPL,
			"insert": infraNWTableSubqCorrelJoinInsert,
			"s0":     infraNWTableSubqCorrelJoinConsume,
		}
		deployCount := 0
		seenDeployStatements := map[string]bool{}
		beanSendValues := []struct {
			theString string
			primitive int
		}{{"E1", 10}, {"E2", 20}, {"E3", 30}}
		beanIndex := 0
		for _, step := range steps[1 : len(steps)-1] {
			switch step.Op {
			case "deploy":
				deployCount++
				if seenDeployStatements[step.Statement] {
					return fmt.Errorf("%s case %q deploys %q twice", infraNWTableSubqCorrelJoinID, caseName, step.Statement)
				}
				seenDeployStatements[step.Statement] = true
				if step.Epl != deployEPLs[step.Statement] {
					return fmt.Errorf("%s case %q deploy %q is not pinned", infraNWTableSubqCorrelJoinID, caseName, step.Statement)
				}
			case "send":
				payload, err := decodeInfraNWTableSubqCorrelJoinPayload(step)
				if err != nil {
					return fmt.Errorf("%s case %q: %w", infraNWTableSubqCorrelJoinID, caseName, err)
				}
				if bean, ok := payload.(infraNWTableSubqCorrelJoinBean); ok {
					if beanIndex >= len(beanSendValues) {
						return fmt.Errorf("%s case %q has an extra SupportBean send", infraNWTableSubqCorrelJoinID, caseName)
					}
					want := beanSendValues[beanIndex]
					if bean.TheString != want.theString || bean.IntPrimitive != want.primitive {
						return fmt.Errorf("%s case %q SupportBean payload %d is not pinned", infraNWTableSubqCorrelJoinID, caseName, beanIndex)
					}
					beanIndex++
				}
			case "undeploy-all":
			default:
				return fmt.Errorf("%s case %q has unsupported op %q", infraNWTableSubqCorrelJoinID, caseName, step.Op)
			}
		}
		if deployCount != 3 {
			return fmt.Errorf("%s case %q must contain exactly three deploys, got %d", infraNWTableSubqCorrelJoinID, caseName, deployCount)
		}
		if beanIndex != len(beanSendValues) {
			return fmt.Errorf("%s case %q must contain exactly %d SupportBean sends, got %d", infraNWTableSubqCorrelJoinID, caseName, len(beanSendValues), beanIndex)
		}
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableSubqCorrelJoinID)
	}
	return nil
}

func runInfraNWTableSubqCorrelJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNWTableSubqCorrelJoinScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableSubqCorrelJoinCases {
		caseSteps := scenario.Steps[offset : offset+13]
		offset += 13
		caseTrace, err := runInfraNWTableSubqCorrelJoinCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableSubqCorrelJoinID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runInfraNWTableSubqCorrelJoinCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableSubqCorrelJoinBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableSubqCorrelJoinS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableSubqCorrelJoinS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}

	isTable := caseName == "table"
	// Infra must exist before the engine snapshots environment named windows.
	if isTable {
		if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("theString"),
			esper.PrimaryKeyColumn[int]("intPrimitive"),
		}); err != nil {
			return compat.Trace{}, err
		}
	} else {
		schema, ok := env.Schema("SupportBean")
		if !ok {
			return compat.Trace{}, fmt.Errorf("SupportBean schema is missing")
		}
		if _, err := esper.CreateNamedWindow(env, "MyInfra", schema,
			esper.NamedWindowRetention(esper.Unique(esper.Field[infraNWTableSubqCorrelJoinBean, string]("theString")))); err != nil {
			return compat.Trace{}, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableSubqCorrelJoinJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableSubqCorrelJoinID}
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
				// The create statement is unnamed in the Java source and its
				// engine-generated name is not pinned; the catalog operation is
				// already applied at env level before the engine started, so
				// only the consume and insert statements are deployed.
				continue
			case "insert":
				source := esper.From[infraNWTableSubqCorrelJoinBean](env, "SupportBean")
				assignments := []esper.TableAssignment{
					esper.SetColumn("theString", esper.Field[infraNWTableSubqCorrelJoinBean, string]("theString")),
					esper.SetColumn("intPrimitive", esper.Field[infraNWTableSubqCorrelJoinBean, int]("intPrimitive")),
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
				plan, err = env.Build(esper.JoinMany(
					esper.JoinSource(esper.From[infraNWTableSubqCorrelJoinS0](env, "SupportBean_S0").Window(esper.LastEvent())),
					esper.JoinSource(esper.From[infraNWTableSubqCorrelJoinS1](env, "SupportBean_S1").Window(esper.LastEvent())),
				).Select(
					esper.SelectLeft("val", esper.SubqueryValueWithOptions[int](inner,
						esper.Field[any, int]("intPrimitive"),
						esper.SubqueryWhere(esper.Equal[string](
							esper.Field[any, string]("theString"),
							esper.JoinField[string](1, "p10"))),
						esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))),
				).Query(esper.StatementName("s0")))
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
			payload, err := decodeInfraNWTableSubqCorrelJoinPayload(step)
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

func decodeInfraNWTableSubqCorrelJoinPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWTableSubqCorrelJoinFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean infraNWTableSubqCorrelJoinBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		if err := requireInfraNWTableSubqCorrelJoinFields(fields, "id", "p00"); err != nil {
			return nil, err
		}
		var s0 infraNWTableSubqCorrelJoinS0
		if err := json.Unmarshal(step.Payload, &s0); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return s0, nil
	case "SupportBean_S1":
		if err := requireInfraNWTableSubqCorrelJoinFields(fields, "id", "p10"); err != nil {
			return nil, err
		}
		var s1 infraNWTableSubqCorrelJoinS1
		if err := json.Unmarshal(step.Payload, &s1); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return s1, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWTableSubqCorrelJoinID, step.EventType)
	}
}

func requireInfraNWTableSubqCorrelJoinFields(object map[string]json.RawMessage, names ...string) error {
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

func validateInfraNWTableSubqCorrelJoinStringArray(raw json.RawMessage, expected []string, name string) error {
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
