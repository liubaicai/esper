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
	eplSubselectWithinFilterHavingID          = "epl-subselect-within-filter-having"
	eplSubselectWithinFilterHavingDescription = "EPLSubselectWithinFilter subqueries within stream filters with an inlined-class UDF (exists and scalar-row correlation over SupportBean_S1#keepall) plus EPLSubselectWithinHaving grouped having gated by a correlated subquery over MyInfra, replayed once as a named window and once as a table (second oracle source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectWithinHaving.java)."
	eplSubselectWithinFilterHavingJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	eplSubselectWithinFilterHavingSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectWithinFilter.java"

	eplSubselectWithinFilterHavingExistsEPL    = "@name('s0') inlined_class \"\"\"\n  public class MyUtil { public static boolean compareIt(String one, String two) { return one.equals(two); } }\n\"\"\" \nselect * from SupportBean_S0(exists (select * from SupportBean_S1#keepall where MyUtil.compareIt(s0.p00,p10))) as s0;\n"
	eplSubselectWithinFilterHavingRowEPL       = "@name('s0') inlined_class \"\"\"\n  public class MyUtil { public static boolean compareIt(String one, String two) { return one.equals(two); } }\n\"\"\" \nselect * from SupportBean_S0('abc' = (select p11 from SupportBean_S1#keepall where MyUtil.compareIt(s0.p00,p10))) as s0;\n"
	eplSubselectWithinFilterHavingSelectEPL    = "@name('s0') select theString as c0, sum(intPrimitive) as c1 from SupportBean#groupwin(theString)#length(2) as sb group by theString having sum(intPrimitive) > (select maxAmount from MyInfra as mw where sb.theString = mw.key)"
	eplSubselectWithinFilterHavingCreateWINEPL = "@name('create') @public create window MyInfra#unique(key) as SupportMaxAmountEvent"
	eplSubselectWithinFilterHavingCreateTABLE  = "@name('create') @public create table MyInfra(key string primary key, maxAmount double)"
	eplSubselectWithinFilterHavingInsertEPL    = "@name('insert') insert into MyInfra select * from SupportMaxAmountEvent"
)

var (
	eplSubselectWithinFilterHavingJavaSources = []string{
		eplSubselectWithinFilterHavingSource,
	}
	eplSubselectWithinFilterHavingJavaRuntimeIDs = []string{
		"java-runtime-844ff5e51e235151dccc",
		"java-runtime-1405e55b4e78331c4ec7",
		"java-runtime-5249f7c19ee8d26d8f30",
		"java-runtime-4b4ca324ee2a41b8ccd6",
	}
	eplSubselectWithinFilterHavingJavaExecutions = []string{
		"EPLSubselectWithinFilterExistsWhereAndUDF",
		"EPLSubselectWithinFilterRowWhereAndUDF",
		"EPLSubselectHavingSubselectWithGroupBy{namedWindow=true}",
		"EPLSubselectHavingSubselectWithGroupBy{namedWindow=false}",
	}
	eplSubselectWithinFilterHavingJavaStaticIDs = []string{
		"java-e0811e7acc03cb5daab2",
		"java-09e87f283cdd09c8434f",
		"java-d69f172401e51113c81d",
		"java-d69f172401e51113c81d",
	}
	eplSubselectWithinFilterHavingCases = []string{
		"within-filter-exists",
		"within-filter-row",
		"within-having-named-window",
		"within-having-table",
	}
	eplSubselectWithinFilterHavingOrdinals = []int{0, 1, 0, 1}
)

type eplSubselectWithinFilterHavingS0 struct {
	ID  int     `esper:"id"`
	P00 string  `esper:"p00"`
	P01 *string `esper:"p01"`
	P02 *string `esper:"p02"`
	P03 *string `esper:"p03"`
}

type eplSubselectWithinFilterHavingS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
	P11 string `esper:"p11"`
}

type eplSubselectWithinFilterHavingBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type eplSubselectWithinFilterHavingMaxAmount struct {
	Key       string  `esper:"key"`
	MaxAmount float64 `esper:"maxAmount"`
}

func loadEplSubselectWithinFilterHavingScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplSubselectWithinFilterHavingID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplSubselectWithinFilterHavingID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplSubselectWithinFilterHavingID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplSubselectWithinFilterHavingID, err)
	}
	if err := requireEplSubselectWithinFilterHavingFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplSubselectWithinFilterHavingID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplSubselectWithinFilterHavingID ||
		metadata.Description != eplSubselectWithinFilterHavingDescription ||
		metadata.JavaCommit != eplSubselectWithinFilterHavingJavaCommit ||
		metadata.JavaSource != eplSubselectWithinFilterHavingSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplSubselectWithinFilterHavingID)
	}
	if err := validateEplSubselectWithinFilterHavingStringArray(root["javaRuntimes"], eplSubselectWithinFilterHavingJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplSubselectWithinFilterHavingStringArray(root["javaNames"], eplSubselectWithinFilterHavingJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplSubselectWithinFilterHavingStringArray(root["javaStaticIds"], eplSubselectWithinFilterHavingJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplSubselectWithinFilterHavingStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplSubselectWithinFilterHavingCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", eplSubselectWithinFilterHavingID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEplSubselectWithinFilterHavingFields(object,
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
		expectedEPL := eplSubselectWithinFilterHavingExistsEPL
		if index == 1 {
			expectedEPL = eplSubselectWithinFilterHavingRowEPL
		} else if index >= 2 {
			expectedEPL = eplSubselectWithinFilterHavingSelectEPL
		}
		if definition.Case != eplSubselectWithinFilterHavingCases[index] ||
			definition.Ordinal != eplSubselectWithinFilterHavingOrdinals[index] ||
			definition.RuntimeID != eplSubselectWithinFilterHavingJavaRuntimeIDs[index] ||
			definition.ExecutionName != eplSubselectWithinFilterHavingJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != expectedEPL {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplSubselectWithinFilterHavingID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", eplSubselectWithinFilterHavingID)
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
			if err := requireEplSubselectWithinFilterHavingFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireEplSubselectWithinFilterHavingFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireEplSubselectWithinFilterHavingFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeEplSubselectWithinFilterHavingPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireEplSubselectWithinFilterHavingFields(object, "op", "case"); err != nil {
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
	if err := validateEplSubselectWithinFilterHavingScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// eplSubselectWithinFilterHavingCaseEPL pins the s0 EPL per case.
func eplSubselectWithinFilterHavingCaseEPL(caseName string) string {
	switch caseName {
	case "within-filter-exists":
		return eplSubselectWithinFilterHavingExistsEPL
	case "within-filter-row":
		return eplSubselectWithinFilterHavingRowEPL
	default:
		return eplSubselectWithinFilterHavingSelectEPL
	}
}

// eplSubselectWithinFilterHavingExpectedSteps pins the per-case step
// sequences: op, deploy statement, event type, and the send payload kind.
type eplSubselectWithinFilterHavingExpectedStep struct {
	op        string
	statement string
	event     string
}

func eplSubselectWithinFilterHavingCaseSteps(caseName string) []eplSubselectWithinFilterHavingExpectedStep {
	s0 := func() eplSubselectWithinFilterHavingExpectedStep {
		return eplSubselectWithinFilterHavingExpectedStep{op: "send", event: "SupportBean_S0"}
	}
	s1 := func() eplSubselectWithinFilterHavingExpectedStep {
		return eplSubselectWithinFilterHavingExpectedStep{op: "send", event: "SupportBean_S1"}
	}
	maxAmount := func() eplSubselectWithinFilterHavingExpectedStep {
		return eplSubselectWithinFilterHavingExpectedStep{op: "send", event: "SupportMaxAmountEvent"}
	}
	bean := func() eplSubselectWithinFilterHavingExpectedStep {
		return eplSubselectWithinFilterHavingExpectedStep{op: "send", event: "SupportBean"}
	}
	switch caseName {
	case "within-filter-exists":
		return []eplSubselectWithinFilterHavingExpectedStep{
			{op: "deploy", statement: "s0"},
			s0(), s1(), s0(), s1(), s0(), s0(),
			{op: "undeploy-all"},
		}
	case "within-filter-row":
		return []eplSubselectWithinFilterHavingExpectedStep{
			{op: "deploy", statement: "s0"},
			s0(), s1(), s0(), s1(), s0(), s0(), s0(),
			{op: "undeploy-all"},
		}
	default:
		createStatement := "create"
		return []eplSubselectWithinFilterHavingExpectedStep{
			{op: "deploy", statement: createStatement},
			{op: "deploy", statement: "insert"},
			{op: "deploy", statement: "s0"},
			maxAmount(), maxAmount(), maxAmount(),
			bean(), bean(), bean(), bean(), bean(), bean(), bean(), bean(), bean(), bean(), bean(), bean(), bean(), bean(),
			{op: "undeploy-all"},
		}
	}
}

func validateEplSubselectWithinFilterHavingScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != eplSubselectWithinFilterHavingID {
		return fmt.Errorf("%s scenario shape is not pinned", eplSubselectWithinFilterHavingID)
	}
	offset := 0
	for _, caseName := range eplSubselectWithinFilterHavingCases {
		expected := eplSubselectWithinFilterHavingCaseSteps(caseName)
		steps := scenario.Steps[offset:]
		if len(steps) < len(expected)+1 {
			return fmt.Errorf("%s case %q steps are truncated", eplSubselectWithinFilterHavingID, caseName)
		}
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", eplSubselectWithinFilterHavingID, caseName)
		}
		for index, want := range expected {
			step := steps[index+1]
			if step.Op != want.op || step.Case != caseName {
				return fmt.Errorf("%s case %q step %d must be op %q", eplSubselectWithinFilterHavingID, caseName, index, want.op)
			}
			switch want.op {
			case "deploy":
				expectedEPL := eplSubselectWithinFilterHavingSelectEPL
				switch want.statement {
				case "create":
					expectedEPL = eplSubselectWithinFilterHavingCreateWINEPL
					if caseName == "within-having-table" {
						expectedEPL = eplSubselectWithinFilterHavingCreateTABLE
					}
				case "insert":
					expectedEPL = eplSubselectWithinFilterHavingInsertEPL
				case "s0":
					expectedEPL = eplSubselectWithinFilterHavingCaseEPL(caseName)
				}
				if step.Statement != want.statement || step.Epl != expectedEPL {
					return fmt.Errorf("%s case %q step %d must deploy %s with the pinned EPL", eplSubselectWithinFilterHavingID, caseName, index, want.statement)
				}
			case "send":
				if step.EventType != want.event {
					return fmt.Errorf("%s case %q step %d must send %s", eplSubselectWithinFilterHavingID, caseName, index, want.event)
				}
				if _, err := decodeEplSubselectWithinFilterHavingPayload(step); err != nil {
					return fmt.Errorf("%s case %q step %d: %w", eplSubselectWithinFilterHavingID, caseName, index, err)
				}
			case "undeploy-all":
			}
		}
		offset += len(expected) + 1
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", eplSubselectWithinFilterHavingID)
	}
	return nil
}

func runEplSubselectWithinFilterHavingScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateEplSubselectWithinFilterHavingScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range eplSubselectWithinFilterHavingCases {
		expected := eplSubselectWithinFilterHavingCaseSteps(caseName)
		caseSteps := scenario.Steps[offset : offset+len(expected)+1]
		offset += len(expected) + 1
		caseTrace, err := runEplSubselectWithinFilterHavingCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eplSubselectWithinFilterHavingID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runEplSubselectWithinFilterHavingCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplSubselectWithinFilterHavingS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplSubselectWithinFilterHavingS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplSubselectWithinFilterHavingBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplSubselectWithinFilterHavingMaxAmount](env, "SupportMaxAmountEvent"); err != nil {
		return compat.Trace{}, err
	}
	// The having cases create MyInfra before the engine exists: the engine
	// snapshots environment named windows at construction and the trigger
	// actions read that snapshot directly.
	for _, step := range steps {
		if step.Op == "deploy" && step.Statement == "create" {
			schema, ok := env.Schema("SupportMaxAmountEvent")
			if !ok {
				return compat.Trace{}, fmt.Errorf("SupportMaxAmountEvent schema is missing")
			}
			if caseName == "within-having-table" {
				if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
					esper.PrimaryKeyColumn[string]("key"),
					esper.TableColumnOf[float64]("maxAmount"),
				}); err != nil {
					return compat.Trace{}, err
				}
				continue
			}
			retention := esper.NamedWindowRetention(esper.Unique(esper.Field[eplSubselectWithinFilterHavingMaxAmount, string]("key")))
			if _, err := esper.CreateNamedWindow(env, "MyInfra", schema, retention); err != nil {
				return compat.Trace{}, err
			}
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(eplSubselectWithinFilterHavingJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	// s0Statement is subscribed when the s0 deploy step runs; the having
	// cases deploy create/insert infra first.
	sequence := uint64(0)
	trace := compat.Trace{Version: "esper-parity/v1", ID: eplSubselectWithinFilterHavingID}
	record := func(caseName string, batch esper.ResultBatch) {
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
	deployPlan := func(step compat.Step) error {
		var plan esper.Plan
		var err error
		switch step.Statement {
		case "create":
			// Infra was created before the engine snapshot; nothing to deploy.
			return nil
		case "insert":
			source := esper.From[eplSubselectWithinFilterHavingMaxAmount](env, "SupportMaxAmountEvent")
			assignments := []esper.TableAssignment{
				esper.SetColumn("key", esper.Field[eplSubselectWithinFilterHavingMaxAmount, string]("key")),
				esper.SetColumn("maxAmount", esper.Field[eplSubselectWithinFilterHavingMaxAmount, float64]("maxAmount")),
			}
			if caseName == "within-having-table" {
				plan, err = env.Build(esper.OnEvent(source).InsertIntoTable("MyInfra", assignments...).Query(esper.StatementName("insert")))
			} else {
				plan, err = env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfra", assignments...).Query(esper.StatementName("insert")))
			}
		case "s0":
			var query esper.Query
			query, err = eplSubselectWithinFilterHavingQuery(env, caseName)
			if err != nil {
				return err
			}
			plan, err = env.Build(query)
		default:
			return fmt.Errorf("unexpected deploy statement %q", step.Statement)
		}
		if err != nil {
			return err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		if step.Statement == "s0" {
			statements := deployment.Statements()
			if len(statements) != 1 || statements[0].Name() != "s0" {
				return fmt.Errorf("expected one s0 statement")
			}
			if _, err := statements[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				record(caseName, batch)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}

	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			if err := deployPlan(step); err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
		case "send":
			payload, err := decodeEplSubselectWithinFilterHavingPayload(step)
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

// eplSubselectWithinFilterHavingQuery builds the s0 statement for the case.
// The filter cases inline the subquery into the stream filter (Java's
// inlined-class UDF compareIt is string equality, so the Go typed form uses
// Equal directly); the having cases correlate the subquery against MyInfra.
func eplSubselectWithinFilterHavingQuery(env *esper.Environment, caseName string) (esper.Query, error) {
	s0P00 := esper.OuterField[string]("p00")
	switch caseName {
	case "within-filter-exists":
		inner := esper.From[eplSubselectWithinFilterHavingS1](env, "SupportBean_S1").Window(esper.KeepAll()).AsRecord()
		return esper.From[eplSubselectWithinFilterHavingS0](env, "SupportBean_S0").Filter(
			esper.SubqueryExists(inner,
				esper.Equal[string](esper.Field[any, string]("p10"), s0P00)),
		).Query(esper.StatementName("s0")), nil
	case "within-filter-row":
		inner := esper.From[eplSubselectWithinFilterHavingS1](env, "SupportBean_S1").Window(esper.KeepAll()).AsRecord()
		return esper.From[eplSubselectWithinFilterHavingS0](env, "SupportBean_S0").Filter(
			esper.Equal[string](esper.Literal("abc"), esper.SubqueryValue[string](inner,
				esper.Field[any, string]("p11"),
				esper.Equal[string](esper.Field[any, string]("p10"), s0P00))),
		).Query(esper.StatementName("s0")), nil
	case "within-having-named-window":
		return eplSubselectWithinFilterHavingSelect(env, esper.FromNamedWindow(env, "MyInfra")), nil
	case "within-having-table":
		return eplSubselectWithinFilterHavingSelect(env, esper.FromTable(env, "MyInfra")), nil
	}
	return esper.Query{}, fmt.Errorf("unknown %s case %q", eplSubselectWithinFilterHavingID, caseName)
}

func eplSubselectWithinFilterHavingSelect(env *esper.Environment, inner esper.RecordStream) esper.Query {
	theString := esper.Field[eplSubselectWithinFilterHavingBean, string]("theString")
	sum := esper.Sum[float64](esper.Cast[int, float64](esper.Field[eplSubselectWithinFilterHavingBean, int]("intPrimitive")))
	maxAmount := esper.SubqueryValue[float64](inner,
		esper.Field[any, float64]("maxAmount"),
		esper.Equal[string](esper.Field[any, string]("key"), esper.OuterField[string]("theString")))
	grouped := esper.From[eplSubselectWithinFilterHavingBean](env, "SupportBean").
		Window(esper.GroupWindow(theString, esper.LengthWindow(2))).
		GroupBy(theString)
	return grouped.Having(esper.Greater[float64](sum, maxAmount)).Select(
		esper.Alias("c0", theString),
		esper.Alias("c1", sum),
	).Query(esper.StatementName("s0"))
}

func decodeEplSubselectWithinFilterHavingPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean_S0":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireEplSubselectWithinFilterHavingFields(fields, "id", "p00"); err != nil {
			return nil, err
		}
		var value eplSubselectWithinFilterHavingS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if len(fields) == 2 {
			if err := requireEplSubselectWithinFilterHavingFields(fields, "id", "p10"); err != nil {
				return nil, err
			}
		} else {
			if err := requireEplSubselectWithinFilterHavingFields(fields, "id", "p10", "p11"); err != nil {
				return nil, err
			}
		}
		var value eplSubselectWithinFilterHavingS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	case "SupportBean":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireEplSubselectWithinFilterHavingFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var value eplSubselectWithinFilterHavingBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportMaxAmountEvent":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireEplSubselectWithinFilterHavingFields(fields, "key", "maxAmount"); err != nil {
			return nil, err
		}
		var value eplSubselectWithinFilterHavingMaxAmount
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMaxAmountEvent: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", eplSubselectWithinFilterHavingID, step.EventType)
	}
}

func requireEplSubselectWithinFilterHavingFields(object map[string]json.RawMessage, names ...string) error {
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

func validateEplSubselectWithinFilterHavingStringArray(raw json.RawMessage, expected []string, name string) error {
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
