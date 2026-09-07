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
	infraNWTableSubqFilteredCorrelID          = "infra-nwtable-subq-filtered-correl"
	infraNWTableSubqFilteredCorrelDescription = "InfraNWTableSubqFilteredCorrel filtered correlated scalar subquery (select intPrimitive from MyInfra(intPrimitive<0) sw where s0.p00=sw.theString) over a keepall named window and over a table, covering named-window subquery index sharing through enable_window_subquery_indexshare create hints and disable_window_subquery_indexshare consumer hints plus explicit create index on the correlated column, captured from the consume listener (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqFilteredCorrel.java)."
	infraNWTableSubqFilteredCorrelJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableSubqFilteredCorrelSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqFilteredCorrel.java"

	infraNWTableSubqFilteredCorrelCreateNW    = "@public create window MyInfra#keepall as select * from SupportBean"
	infraNWTableSubqFilteredCorrelCreateNWES  = "@Hint('enable_window_subquery_indexshare') " + infraNWTableSubqFilteredCorrelCreateNW
	infraNWTableSubqFilteredCorrelCreateTbl   = "@public create table MyInfra (theString string primary key, intPrimitive int primary key)"
	infraNWTableSubqFilteredCorrelInsert      = "insert into MyInfra select theString, intPrimitive from SupportBean"
	infraNWTableSubqFilteredCorrelIndex       = "@name('index') create index MyIndex on MyInfra(theString)"
	infraNWTableSubqFilteredCorrelConsume     = "@name('consume') select (select intPrimitive from MyInfra(intPrimitive<0) sw where s0.p00=sw.theString) as val from S0 s0"
	infraNWTableSubqFilteredCorrelConsumeHint = "@Hint('disable_window_subquery_indexshare') " + infraNWTableSubqFilteredCorrelConsume
)

var (
	infraNWTableSubqFilteredCorrelJavaSources = []string{
		infraNWTableSubqFilteredCorrelSource,
	}
	infraNWTableSubqFilteredCorrelJavaRuntimeIDs = []string{
		"java-runtime-6823e53d322aa502295b",
		"java-runtime-9ff8e85038e7fa442f0b",
		"java-runtime-11e662916b4e4bebf3ed",
		"java-runtime-fdb6dcb460167c720624",
		"java-runtime-d8db6be7fe12e953f8f2",
		"java-runtime-df8a638d47fce3bbe9e7",
		"java-runtime-1e359a7b2b7cee11d210",
	}
	infraNWTableSubqFilteredCorrelJavaExecutions = []string{
		"InfraNWTableSubqFilteredCorrelAssertion",
		"InfraNWTableSubqFilteredCorrelAssertion",
		"InfraNWTableSubqFilteredCorrelAssertion",
		"InfraNWTableSubqFilteredCorrelAssertion",
		"InfraNWTableSubqFilteredCorrelAssertion",
		"InfraNWTableSubqFilteredCorrelAssertion",
		"InfraNWTableSubqFilteredCorrelAssertion",
	}
	infraNWTableSubqFilteredCorrelJavaStaticIDs = []string{
		"java-cef495b8aa35bb19a5ac",
		"java-cef495b8aa35bb19a5ac",
		"java-cef495b8aa35bb19a5ac",
		"java-cef495b8aa35bb19a5ac",
		"java-cef495b8aa35bb19a5ac",
		"java-cef495b8aa35bb19a5ac",
		"java-cef495b8aa35bb19a5ac",
	}
	infraNWTableSubqFilteredCorrelCases = []string{
		"nw-no-share",
		"nw-no-share-index",
		"nw-share",
		"nw-share-disable",
		"nw-share-disable-index",
		"table-no-share",
		"table-no-share-index",
	}
	infraNWTableSubqFilteredCorrelOrdinals = []int{0, 1, 2, 3, 4, 5, 6}
)

type infraNWTableSubqFilteredCorrelBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type infraNWTableSubqFilteredCorrelS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

func loadInfraNWTableSubqFilteredCorrelScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableSubqFilteredCorrelID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableSubqFilteredCorrelID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableSubqFilteredCorrelID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableSubqFilteredCorrelID, err)
	}
	if err := requireInfraNWTableSubqFilteredCorrelFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableSubqFilteredCorrelID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableSubqFilteredCorrelID ||
		metadata.Description != infraNWTableSubqFilteredCorrelDescription ||
		metadata.JavaCommit != infraNWTableSubqFilteredCorrelJavaCommit ||
		metadata.JavaSource != infraNWTableSubqFilteredCorrelSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableSubqFilteredCorrelID)
	}
	if err := validateInfraNWTableSubqFilteredCorrelStringArray(root["javaRuntimes"], infraNWTableSubqFilteredCorrelJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqFilteredCorrelStringArray(root["javaNames"], infraNWTableSubqFilteredCorrelJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqFilteredCorrelStringArray(root["javaStaticIds"], infraNWTableSubqFilteredCorrelJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqFilteredCorrelStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableSubqFilteredCorrelCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly seven cases", infraNWTableSubqFilteredCorrelID)
	}
	consumeEPL := func(caseIndex int) string {
		if caseIndex == 3 || caseIndex == 4 {
			return infraNWTableSubqFilteredCorrelConsumeHint
		}
		return infraNWTableSubqFilteredCorrelConsume
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableSubqFilteredCorrelFields(object,
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
		if definition.Case != infraNWTableSubqFilteredCorrelCases[index] ||
			definition.Ordinal != infraNWTableSubqFilteredCorrelOrdinals[index] ||
			definition.RuntimeID != infraNWTableSubqFilteredCorrelJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableSubqFilteredCorrelJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != consumeEPL(index) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableSubqFilteredCorrelID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableSubqFilteredCorrelID)
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
			if err := requireInfraNWTableSubqFilteredCorrelFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableSubqFilteredCorrelFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableSubqFilteredCorrelFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableSubqFilteredCorrelPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableSubqFilteredCorrelFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWTableSubqFilteredCorrelScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// infraNWTableSubqFilteredCorrelCreateEPL pins the create statement per case.
func infraNWTableSubqFilteredCorrelCreateEPL(caseName string) string {
	switch caseName {
	case "nw-no-share", "nw-no-share-index":
		return infraNWTableSubqFilteredCorrelCreateNW
	case "nw-share", "nw-share-disable", "nw-share-disable-index":
		return infraNWTableSubqFilteredCorrelCreateNWES
	default:
		return infraNWTableSubqFilteredCorrelCreateTbl
	}
}

func infraNWTableSubqFilteredCorrelIsTable(caseName string) bool {
	return caseName == "table-no-share" || caseName == "table-no-share-index"
}

func validateInfraNWTableSubqFilteredCorrelScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNWTableSubqFilteredCorrelID {
		return fmt.Errorf("%s scenario shape is not pinned", infraNWTableSubqFilteredCorrelID)
	}
	// Per case the exact step interleaving is pinned: case marker, create,
	// insert, [index], E1/E2 sends, consume, four S0 sends interleaved with
	// the E3/E4 sends in Java order, undeploy-all. Index cases are
	// nw-no-share-index, nw-share-disable-index, and table-no-share-index.
	type expectedStep struct {
		op        string
		statement string
		eventType string
		sendIndex int
	}
	beanSends := []struct {
		theString string
		primitive int
	}{{"E1", 1}, {"E2", -2}, {"E3", -3}, {"E4", 4}}
	s0Sends := []struct {
		id  int
		p00 string
	}{{10, "E1"}, {20, "E2"}, {-3, "E3"}, {20, "E4"}}
	offset := 0
	for caseIndex, caseName := range infraNWTableSubqFilteredCorrelCases {
		hasIndex := caseName == "nw-no-share-index" || caseName == "nw-share-disable-index" || caseName == "table-no-share-index"
		consumeEPL := infraNWTableSubqFilteredCorrelConsume
		if caseIndex == 3 || caseIndex == 4 {
			consumeEPL = infraNWTableSubqFilteredCorrelConsumeHint
		}
		deployEPLs := map[string]string{
			"create":  infraNWTableSubqFilteredCorrelCreateEPL(caseName),
			"insert":  infraNWTableSubqFilteredCorrelInsert,
			"index":   infraNWTableSubqFilteredCorrelIndex,
			"consume": consumeEPL,
		}
		expected := []expectedStep{
			{op: "deploy", statement: "create"},
			{op: "deploy", statement: "insert"},
		}
		if hasIndex {
			expected = append(expected, expectedStep{op: "deploy", statement: "index"})
		}
		expected = append(expected,
			expectedStep{op: "send", eventType: "SupportBean", sendIndex: 0},
			expectedStep{op: "send", eventType: "SupportBean", sendIndex: 1},
			expectedStep{op: "deploy", statement: "consume"},
			expectedStep{op: "send", eventType: "SupportBean_S0", sendIndex: 0},
			expectedStep{op: "send", eventType: "SupportBean_S0", sendIndex: 1},
			expectedStep{op: "send", eventType: "SupportBean", sendIndex: 2},
			expectedStep{op: "send", eventType: "SupportBean", sendIndex: 3},
			expectedStep{op: "send", eventType: "SupportBean_S0", sendIndex: 2},
			expectedStep{op: "send", eventType: "SupportBean_S0", sendIndex: 3},
			expectedStep{op: "undeploy-all"},
		)
		steps := scenario.Steps[offset : offset+1+len(expected)]
		offset += 1 + len(expected)
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", infraNWTableSubqFilteredCorrelID, caseName)
		}
		for index, want := range expected {
			step := steps[index+1]
			if step.Op != want.op || step.Case != caseName {
				return fmt.Errorf("%s case %q step %d must be op %q in order", infraNWTableSubqFilteredCorrelID, caseName, index+1, want.op)
			}
			switch want.op {
			case "deploy":
				if step.Statement != want.statement || step.Epl != deployEPLs[want.statement] {
					return fmt.Errorf("%s case %q step %d deploy %q is not pinned", infraNWTableSubqFilteredCorrelID, caseName, index+1, want.statement)
				}
			case "send":
				if step.EventType != want.eventType {
					return fmt.Errorf("%s case %q step %d must send %q", infraNWTableSubqFilteredCorrelID, caseName, index+1, want.eventType)
				}
				payload, err := decodeInfraNWTableSubqFilteredCorrelPayload(step)
				if err != nil {
					return fmt.Errorf("%s case %q step %d: %w", infraNWTableSubqFilteredCorrelID, caseName, index+1, err)
				}
				if bean, ok := payload.(infraNWTableSubqFilteredCorrelBean); ok {
					wanted := beanSends[want.sendIndex]
					if bean.TheString != wanted.theString || bean.IntPrimitive != wanted.primitive {
						return fmt.Errorf("%s case %q step %d SupportBean payload is not pinned", infraNWTableSubqFilteredCorrelID, caseName, index+1)
					}
				} else if s0, ok := payload.(infraNWTableSubqFilteredCorrelS0); ok {
					wanted := s0Sends[want.sendIndex]
					if s0.ID != wanted.id || s0.P00 != wanted.p00 {
						return fmt.Errorf("%s case %q step %d SupportBean_S0 payload is not pinned", infraNWTableSubqFilteredCorrelID, caseName, index+1)
					}
				}
			case "undeploy-all":
			}
		}
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableSubqFilteredCorrelID)
	}
	return nil
}

func runInfraNWTableSubqFilteredCorrelScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNWTableSubqFilteredCorrelScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableSubqFilteredCorrelCases {
		hasIndex := caseName == "nw-no-share-index" || caseName == "nw-share-disable-index" || caseName == "table-no-share-index"
		span := 13
		if hasIndex {
			span = 14
		}
		caseSteps := scenario.Steps[offset : offset+span]
		offset += span
		caseTrace, err := runInfraNWTableSubqFilteredCorrelCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableSubqFilteredCorrelID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runInfraNWTableSubqFilteredCorrelCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableSubqFilteredCorrelBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableSubqFilteredCorrelS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}

	isTable := infraNWTableSubqFilteredCorrelIsTable(caseName)
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
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableSubqFilteredCorrelJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableSubqFilteredCorrelID}
	sequence := uint64(0)
	record := func(batch esper.ResultBatch) {
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: "consume",
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
				// only the insert, index, and consume statements act here.
				continue
			case "insert":
				source := esper.From[infraNWTableSubqFilteredCorrelBean](env, "SupportBean")
				assignments := []esper.TableAssignment{
					esper.SetColumn("theString", esper.Field[infraNWTableSubqFilteredCorrelBean, string]("theString")),
					esper.SetColumn("intPrimitive", esper.Field[infraNWTableSubqFilteredCorrelBean, int]("intPrimitive")),
				}
				if isTable {
					plan, err = env.Build(esper.OnEvent(source).InsertIntoTable("MyInfra", assignments...).Query(esper.StatementName("insert")))
				} else {
					plan, err = env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfra", assignments...).Query(esper.StatementName("insert")))
				}
			case "index":
				// The explicit create index is a late catalog operation on the
				// live infra; the scenario still pins the exact EPL text.
				var indexErr error
				if isTable {
					table, ok := engine.Table("MyInfra")
					if !ok {
						indexErr = fmt.Errorf("MyInfra table is missing")
					} else {
						indexErr = table.CreateIndex("MyIndex", []string{"theString"}, esper.IndexHash, false)
					}
				} else {
					window, ok := engine.NamedWindow("MyInfra")
					if !ok {
						indexErr = fmt.Errorf("MyInfra named window is missing")
					} else {
						indexErr = window.CreateIndex("MyIndex", []string{"theString"}, esper.IndexHash, false)
					}
				}
				if indexErr != nil {
					return compat.Trace{}, fmt.Errorf("create index: %w", indexErr)
				}
				continue
			case "consume":
				// The Java statement filters the infra stream with
				// MyInfra(intPrimitive<0) and correlates on s0.p00=sw.theString;
				// table inner streams carry no typed Filter, so the filter is
				// folded into the subquery predicate (result-equivalent: rows
				// failing either conjunct are excluded).
				var inner esper.RecordStream
				if isTable {
					inner = esper.FromTable(env, "MyInfra")
				} else {
					inner = esper.FromNamedWindow(env, "MyInfra")
				}
				plan, err = env.Build(esper.Select(
					esper.From[infraNWTableSubqFilteredCorrelS0](env, "SupportBean_S0"),
					esper.Alias("val", esper.SubqueryValueWithOptions[int](inner,
						esper.Field[any, int]("intPrimitive"),
						esper.SubqueryWhere(esper.And(
							esper.Less[int](esper.Field[any, int]("intPrimitive"), esper.Literal(0)),
							esper.Equal[string](esper.Field[any, string]("theString"), esper.OuterField[string]("p00")))),
						esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))),
				).Query(esper.StatementName("consume")))
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
			if step.Statement == "consume" {
				statements := deployment.Statements()
				if len(statements) != 1 || statements[0].Name() != "consume" {
					return compat.Trace{}, fmt.Errorf("expected one consume statement")
				}
				if _, err := statements[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					record(batch)
					return nil
				}); err != nil {
					return compat.Trace{}, err
				}
			}
		case "send":
			payload, err := decodeInfraNWTableSubqFilteredCorrelPayload(step)
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

func decodeInfraNWTableSubqFilteredCorrelPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWTableSubqFilteredCorrelFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean infraNWTableSubqFilteredCorrelBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		if err := requireInfraNWTableSubqFilteredCorrelFields(fields, "id", "p00"); err != nil {
			return nil, err
		}
		var s0 infraNWTableSubqFilteredCorrelS0
		if err := json.Unmarshal(step.Payload, &s0); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return s0, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWTableSubqFilteredCorrelID, step.EventType)
	}
}

func requireInfraNWTableSubqFilteredCorrelFields(object map[string]json.RawMessage, names ...string) error {
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

func validateInfraNWTableSubqFilteredCorrelStringArray(raw json.RawMessage, expected []string, name string) error {
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
