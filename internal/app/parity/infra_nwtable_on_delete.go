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
	infraNWTableOnDeleteID          = "infra-nwtable-on-delete"
	infraNWTableOnDeleteDescription = "InfraNWTableOnDelete on-trigger delete semantics over a keepall named window and over a primary-key table: where-clause deletes scoped to trigger-event versus infra-row properties (concat and range predicates), pattern-triggered unconditional delete-all, and event-triggered delete-all with deleted rows delivered as new data to the on-delete listener (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnDelete.java)."
	infraNWTableOnDeleteJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableOnDeleteSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnDelete.java"

	infraNWTableOnDeleteCreateNW       = "@name('CreateInfra') @public create window MyInfra#keepall as select theString as a, intPrimitive as b from SupportBean"
	infraNWTableOnDeleteCreateNWName   = "@Name('CreateInfra') @public create window MyInfra#keepall as select theString as a, intPrimitive as b from SupportBean"
	infraNWTableOnDeleteCreateTbl      = "@name('CreateInfra') @public create table MyInfra (a string primary key, b int)"
	infraNWTableOnDeleteCreateTblTight = "@name('CreateInfra') @public create table MyInfra(a string primary key, b int)"
	infraNWTableOnDeleteCreateTblName  = "@Name('CreateInfra') @public create table MyInfra (a string primary key, b int)"
	infraNWTableOnDeleteCondA          = "on SupportBean_A delete from MyInfra where 'X' || a || 'X' = id"
	infraNWTableOnDeleteCondB          = "on SupportBean_B delete from MyInfra where b < 5"
	infraNWTableOnDeletePattern        = "@name('OnDelete') on pattern [every ea=SupportBean_A or every eb=SupportBean_B] delete from MyInfra"
	infraNWTableOnDeleteAll            = "@Name('OnDelete') on SupportBean_A delete from MyInfra"
	infraNWTableOnDeleteInsert         = "insert into MyInfra select theString as a, intPrimitive as b from SupportBean"
	infraNWTableOnDeleteInsertName     = "@Name('Insert') insert into MyInfra select theString as a, intPrimitive as b from SupportBean"
	infraNWTableOnDeleteSelect         = "@Name('Select') select irstream MyInfra.a as a, b from MyInfra as s1"
	infraNWTableOnDeleteCount          = "select count(*) as c0 from MyInfra"
)

var (
	infraNWTableOnDeleteJavaSources = []string{
		infraNWTableOnDeleteSource,
	}
	infraNWTableOnDeleteJavaRuntimeIDs = []string{
		"java-runtime-6d190be4d9e8b76b11f3",
		"java-runtime-992fcd3db7f15b8254cd",
		"java-runtime-2ab09353c939b15ee52e",
		"java-runtime-e795679c4403d309f805",
		"java-runtime-11809b3f1d90b37d52b8",
		"java-runtime-b7a9b1b53c0009e7d347",
	}
	infraNWTableOnDeleteJavaExecutions = []string{
		"InfraDeleteCondition{namedWindow=true}",
		"InfraDeleteCondition{namedWindow=false}",
		"InfraDeletePattern{namedWindow=true}",
		"InfraDeletePattern{namedWindow=false}",
		"InfraDeleteAll{namedWindow=true}",
		"InfraDeleteAll{namedWindow=false}",
	}
	infraNWTableOnDeleteJavaStaticIDs = []string{
		"java-eb28fd3f17513a399ac9",
		"java-eb28fd3f17513a399ac9",
		"java-132a1f9e7f2c434a19e9",
		"java-132a1f9e7f2c434a19e9",
		"java-e3aeca986a0cfee60dc7",
		"java-e3aeca986a0cfee60dc7",
	}
	infraNWTableOnDeleteCases = []string{
		"cond-nw",
		"cond-table",
		"pattern-nw",
		"pattern-table",
		"deleteall-nw",
		"deleteall-table",
	}
	infraNWTableOnDeleteOrdinals = []int{0, 1, 2, 3, 4, 5}
)

type infraNWTableOnDeleteBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int64  `esper:"intPrimitive"`
}

type infraNWTableOnDeleteA struct {
	ID string `esper:"id"`
}

type infraNWTableOnDeleteB struct {
	ID string `esper:"id"`
}

func infraNWTableOnDeleteIsTable(caseName string) bool {
	return caseName == "cond-table" || caseName == "pattern-table" || caseName == "deleteall-table"
}

func infraNWTableOnDeleteIsPattern(caseName string) bool {
	return caseName == "pattern-nw" || caseName == "pattern-table"
}

// runInfraNWTableOnDeleteScenario replays the six InfraNWTableOnDelete
// executions: each case builds a fresh engine, deploys the pinned statements,
// sends the pinned events, and records listener batches, iterator snapshots
// and count reads exactly where the Java assertions observe them.
func runInfraNWTableOnDeleteScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableOnDeleteCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableOnDeleteCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableOnDeleteID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableOnDeleteID)
	}
	return trace, nil
}

func runInfraNWTableOnDeleteCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableOnDeleteBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnDeleteA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnDeleteB](env, "SupportBean_B"); err != nil {
		return compat.Trace{}, err
	}

	isTable := infraNWTableOnDeleteIsTable(caseName)
	if isTable {
		if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("a"),
			esper.TableColumnOf[int64]("b"),
		}); err != nil {
			return compat.Trace{}, err
		}
	} else {
		projection, err := esper.NewMapSchema("MyInfraSchema", []esper.FieldSpec{
			esper.FieldDef("a", reflect.TypeOf("")),
			esper.FieldDef("b", reflect.TypeOf(int64(0))),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterSchema(projection); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyInfra", projection, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableOnDeleteJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableOnDeleteID}
	record := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		rec := compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		}
		if len(rec.Old) == 0 {
			rec.Old = nil
		}
		trace.Records = append(trace.Records, rec)
	}

	var deployments []*esper.Deployment
	statements := map[string]*esper.Statement{}
	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			var plans []esper.Plan
			if step.Statement == "OnDelete" && infraNWTableOnDeleteIsPattern(caseName) {
				built, err := infraNWTableOnDeleteBuildPattern(env, isTable)
				if err != nil {
					return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
				}
				plans = built
			} else {
				plan, err := infraNWTableOnDeleteBuild(env, step.Statement, isTable)
				if err != nil {
					return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
				}
				plans = []esper.Plan{plan}
			}
			for _, plan := range plans {
				deployment, err := engine.Deploy(ctx, plan)
				if err != nil {
					return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
				}
				deployments = append(deployments, deployment)
				deploymentStatements := deployment.Statements()
				for _, statement := range deploymentStatements {
					if statement.Name() == step.Statement {
						statements[step.Statement] = statement
					}
				}
				if _, ok := statements[step.Statement]; !ok && len(deploymentStatements) == 1 {
					// Java deploys the where-clause deletes without @name:
					// bind the deployment's single anonymous statement.
					statements[step.Statement] = deploymentStatements[0]
				}
				for _, statement := range deploymentStatements {
					if infraNWTableOnDeleteListened(step.Statement) && statement.Name() == step.Statement {
						name := step.Statement
						if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
							record(name, batch)
							return nil
						}); err != nil {
							return compat.Trace{}, err
						}
					}
				}
			}
		case "deployed":
			statement, ok := statements[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("deployed marker for unknown statement %q", step.Statement)
			}
			_ = statement
			sequence[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequence[step.Statement+":deployed"],
				Time:      "1970-01-01T00:00:00Z",
			})
		case "send":
			payload, err := decodeInfraNWTableOnDeletePayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		case "snapshot":
			statement, ok := statements[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("snapshot statement %q was not deployed", step.Statement)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return compat.Trace{}, err
			}
			rows := compat.NormalizeResults(result.Batch.New)
			if step.Mode == "any" {
				sortRowsCanonical(rows)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: step.Statement,
				Sequence:  0,
				Time:      "1970-01-01T00:00:00Z",
				New:       rows,
			})
		case "faf":
			plan, err := infraNWTableOnDeleteBuildFAF(env, isTable)
			if err != nil {
				return compat.Trace{}, err
			}
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil {
				return compat.Trace{}, err
			}
			sequence["faf"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "faf",
				Statement: "count",
				Sequence:  sequence["faf"],
				Time:      "1970-01-01T00:00:00Z",
				New:       compat.NormalizeResults(result.Batch.New),
			})
		case "undeploy-all":
		default:
			return compat.Trace{}, fmt.Errorf("unexpected op %q", step.Op)
		}
	}
	for _, deployment := range deployments {
		if err := deployment.Undeploy(ctx); err != nil {
			return compat.Trace{}, err
		}
	}
	return trace, nil
}

// infraNWTableOnDeleteListened reports whether the Java execution attaches a
// listener to the named statement: CreateInfra/OnDelete/Select are listened
// for named windows; for tables only OnDelete is listened (the Java execution
// still attaches CreateInfra/Select listeners but they never fire, so the
// observable record stream is identical either way — Go attaches them too for
// fidelity where the API allows).
func infraNWTableOnDeleteListened(statement string) bool {
	switch statement {
	case "CreateInfra", "OnDelete", "Select":
		return true
	}
	return false
}

func infraNWTableOnDeleteBuild(env *esper.Environment, statement string, isTable bool) (esper.Plan, error) {
	source := esper.From[infraNWTableOnDeleteBean](env, "SupportBean")
	sourceA := esper.From[infraNWTableOnDeleteA](env, "SupportBean_A")
	sourceB := esper.From[infraNWTableOnDeleteB](env, "SupportBean_B")
	switch statement {
	case "CreateInfra":
		if isTable {
			return env.Build(esper.FromTable(env, "MyInfra").Query(esper.StatementName("CreateInfra"), esper.WithOldStream()))
		}
		return env.Build(esper.FromNamedWindow(env, "MyInfra").Query(esper.StatementName("CreateInfra"), esper.WithOldStream()))
	case "DeleteCondA":
		predicateNW := esper.Equal[string](
			esper.Concat(esper.Literal("X"), esper.NamedWindowField[string]("a"), esper.Literal("X")),
			esper.Field[infraNWTableOnDeleteA, string]("id"))
		predicateTbl := esper.Equal[string](
			esper.Concat(esper.Literal("X"), esper.TableField[string]("a"), esper.Literal("X")),
			esper.Field[infraNWTableOnDeleteA, string]("id"))
		if isTable {
			return env.Build(esper.OnEvent(sourceA).DeleteFromTableWhere("MyInfra", predicateTbl).Query())
		}
		return env.Build(esper.OnEvent(sourceA).DeleteFromNamedWindow("MyInfra", predicateNW).Query())
	case "DeleteCondB":
		if isTable {
			return env.Build(esper.OnEvent(sourceB).DeleteFromTableWhere("MyInfra",
				esper.Less[int64](esper.TableField[int64]("b"), esper.Literal(int64(5)))).Query())
		}
		return env.Build(esper.OnEvent(sourceB).DeleteFromNamedWindow("MyInfra",
			esper.Less[int64](esper.NamedWindowField[int64]("b"), esper.Literal(int64(5)))).Query())
	case "OnDelete":
		if isTable {
			return env.Build(esper.OnEvent(sourceA).DeleteAllFromTable("MyInfra").Query(esper.StatementName("OnDelete")))
		}
		return env.Build(esper.OnEvent(sourceA).DeleteAllFromNamedWindow("MyInfra").Query(esper.StatementName("OnDelete")))
	case "Insert":
		assignments := []esper.TableAssignment{
			esper.SetColumn("a", esper.Field[infraNWTableOnDeleteBean, string]("theString")),
			esper.SetColumn("b", esper.Field[infraNWTableOnDeleteBean, int64]("intPrimitive")),
		}
		if isTable {
			return env.Build(esper.OnEvent(source).InsertIntoTable("MyInfra", assignments...).Query(esper.StatementName("Insert")))
		}
		return env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfra", assignments...).Query(esper.StatementName("Insert")))
	case "Select":
		var inner esper.RecordStream
		if isTable {
			inner = esper.FromTable(env, "MyInfra")
		} else {
			inner = esper.FromNamedWindow(env, "MyInfra")
		}
		return env.Build(inner.Query(esper.StatementName("Select"), esper.WithOldStream()))
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q", statement)
}

// infraNWTableOnDeleteBuildPattern materializes the Java
// `on pattern [every ea=A or every eb=B] delete from MyInfra` statement as a
// two-statement Go deployment: a pattern select routed into a map stream,
// then an OnRecord trigger that deletes all rows. The Java pattern fires
// once per A or B event; the Go equivalent preserves that cardinality.
func infraNWTableOnDeleteBuildPattern(env *esper.Environment, isTable bool) ([]esper.Plan, error) {
	if _, err := esper.RegisterMap(env, "PatternTrigger", []esper.FieldSpec{
		esper.FieldDef("fired", reflect.TypeOf("")),
	}); err != nil {
		return nil, err
	}
	pattern := esper.PatternFromRecord(esper.FromAny(env, "SupportBean_A"), "ea", esper.Literal(true)).Every().
		Or(esper.PatternFromRecord(esper.FromAny(env, "SupportBean_B"), "eb", esper.Literal(true)).Every())
	patternPlan, err := env.Build(pattern.Select(
		esper.Alias("fired", esper.Literal("x")),
	).InsertInto("PatternTrigger"))
	if err != nil {
		return nil, err
	}
	trigger := esper.OnRecord(esper.FromAny(env, "PatternTrigger"))
	var triggerPlan esper.Plan
	if isTable {
		triggerPlan, err = env.Build(trigger.DeleteAllFromTable("MyInfra").Query(esper.StatementName("OnDelete")))
	} else {
		triggerPlan, err = env.Build(trigger.DeleteAllFromNamedWindow("MyInfra").Query(esper.StatementName("OnDelete")))
	}
	if err != nil {
		return nil, err
	}
	return []esper.Plan{patternPlan, triggerPlan}, nil
}

func infraNWTableOnDeleteBuildFAF(env *esper.Environment, isTable bool) (esper.Plan, error) {
	var inner esper.RecordStream
	if isTable {
		inner = esper.FromTable(env, "MyInfra")
	} else {
		inner = esper.FromNamedWindow(env, "MyInfra")
	}
	return env.Build(inner.Aggregate(esper.Alias("c0", esper.CountAll())).Query())
}

func decodeInfraNWTableOnDeletePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWTableOnDeleteFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean infraNWTableOnDeleteBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_A":
		if err := requireInfraNWTableOnDeleteFields(fields, "id"); err != nil {
			return nil, err
		}
		var a infraNWTableOnDeleteA
		if err := json.Unmarshal(step.Payload, &a); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return a, nil
	case "SupportBean_B":
		if err := requireInfraNWTableOnDeleteFields(fields, "id"); err != nil {
			return nil, err
		}
		var b infraNWTableOnDeleteB
		if err := json.Unmarshal(step.Payload, &b); err != nil {
			return nil, fmt.Errorf("decode SupportBean_B: %w", err)
		}
		return b, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWTableOnDeleteID, step.EventType)
	}
}

func requireInfraNWTableOnDeleteFields(object map[string]json.RawMessage, names ...string) error {
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

// validateInfraNWTableOnDeleteRawSteps pins the complete step sequence per
// case against the raw JSON objects: deploy statements with byte-exact EPL,
// deployed markers, send event types with canonical payloads, snapshot/faf
// reads, and undeploy-all terminators.
func validateInfraNWTableOnDeleteRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableOnDeleteCases {
		want, ok := infraNWTableOnDeleteCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableOnDeleteID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableOnDeleteID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnDeleteID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnDeleteID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableOnDeleteStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableOnDeleteID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned", infraNWTableOnDeleteID, caseName, index+1)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableOnDeleteID)
	}
	return nil
}

// loadInfraNWTableOnDeleteScenario decodes the scenario with the strict
// contract shared by the differential runners: no duplicate or unknown JSON
// fields, pinned metadata, pinned per-case runtime/execution/EPL, and a
// per-op step field whitelist.
func loadInfraNWTableOnDeleteScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableOnDeleteID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableOnDeleteID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnDeleteID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnDeleteID, err)
	}
	if err := requireInfraNWTableOnDeleteFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableOnDeleteID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableOnDeleteID ||
		metadata.Description != infraNWTableOnDeleteDescription ||
		metadata.JavaCommit != infraNWTableOnDeleteJavaCommit ||
		metadata.JavaSource != infraNWTableOnDeleteSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableOnDeleteID)
	}
	if err := validateInfraNWTableOnDeleteStringArray(root["javaRuntimes"], infraNWTableOnDeleteJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnDeleteStringArray(root["javaNames"], infraNWTableOnDeleteJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnDeleteStringArray(root["javaStaticIds"], infraNWTableOnDeleteJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnDeleteStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableOnDeleteCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableOnDeleteID, len(infraNWTableOnDeleteCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableOnDeleteFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string `json:"case"`
			Ordinal       int    `json:"ordinal"`
			RuntimeID     string `json:"runtimeId"`
			ExecutionName string `json:"executionName"`
			Observation   string `json:"observation"`
			EPL           string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != infraNWTableOnDeleteCases[index] ||
			definition.Ordinal != infraNWTableOnDeleteOrdinals[index] ||
			definition.RuntimeID != infraNWTableOnDeleteJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableOnDeleteJavaExecutions[index] ||
			definition.Observation != infraNWTableOnDeleteCaseObservations[index] ||
			definition.EPL != infraNWTableOnDeleteCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableOnDeleteID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableOnDeleteID)
	}
	steps := make([]compat.Step, len(rawSteps))
	objects := make([]map[string]json.RawMessage, len(rawSteps))
	operations := make([]string, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		objects[index] = object
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		operations[index] = operation
		switch operation {
		case "case":
			if err := requireInfraNWTableOnDeleteFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableOnDeleteFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableOnDeleteFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableOnDeleteFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableOnDeletePayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableOnDeleteFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "faf":
			if err := requireInfraNWTableOnDeleteFields(object, "op", "case", "statement", "epl", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableOnDeleteFields(object, "op", "case"); err != nil {
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
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnDeleteRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateInfraNWTableOnDeleteStringArray(raw json.RawMessage, expected []string, name string) error {
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

// infraNWTableOnDeleteCaseEPLs pins the first deploy EPL of each case, the
// value carried by the scenario cases[] metadata.
var infraNWTableOnDeleteCaseEPLs = []string{
	infraNWTableOnDeleteCreateNW,
	infraNWTableOnDeleteCreateTbl,
	infraNWTableOnDeleteCreateNW,
	infraNWTableOnDeleteCreateTblTight,
	infraNWTableOnDeleteCreateNWName,
	infraNWTableOnDeleteCreateTblName,
}

var infraNWTableOnDeleteCaseObservations = []string{
	"listener; where-clause deletes scoped to trigger id vs row a/b over the keepall named window",
	"iterator+count; same where-clause deletes over the primary-key table with no listener deliveries",
	"listener; pattern-triggered unconditional delete-all over the keepall named window",
	"listener; pattern-triggered unconditional delete-all over the primary-key table",
	"listener; event-triggered delete-all delivering deleted rows as new data over the named window",
	"listener; event-triggered delete-all delivering deleted rows as new data over the table",
}

// infraNWTableOnDeleteCaseSteps pins the exact op sequence per case:
// deploy statements with byte-exact EPL, deployed markers, send event types
// with canonical payloads, snapshot/faf reads, and undeploy-all terminators.
var infraNWTableOnDeleteCaseSteps = map[string][]string{
	"cond-nw": {
		"deploy:CreateInfra:@name('CreateInfra') @public create window MyInfra#keepall as select theString as a, intPrimitive as b from SupportBean",
		"deployed:CreateInfra",
		"deploy:DeleteCondA:on SupportBean_A delete from MyInfra where 'X' || a || 'X' = id",
		"deployed:DeleteCondA",
		"deploy:DeleteCondB:on SupportBean_B delete from MyInfra where b < 5",
		"deployed:DeleteCondB",
		"deploy:Insert:insert into MyInfra select theString as a, intPrimitive as b from SupportBean",
		"deployed:Insert",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"E3\"}",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"snapshot:CreateInfra:any:a,b",
		"send:SupportBean_A:{\"id\":\"XE2X\"}",
		"snapshot:CreateInfra:any:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean:{\"intPrimitive\":7,\"theString\":\"E7\"}",
		"snapshot:CreateInfra:any:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean_B:{\"id\":\"B1\"}",
		"snapshot:CreateInfra:any:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"undeploy-all",
	},
	"cond-table": {
		"deploy:CreateInfra:@name('CreateInfra') @public create table MyInfra (a string primary key, b int)",
		"deployed:CreateInfra",
		"deploy:DeleteCondA:on SupportBean_A delete from MyInfra where 'X' || a || 'X' = id",
		"deployed:DeleteCondA",
		"deploy:DeleteCondB:on SupportBean_B delete from MyInfra where b < 5",
		"deployed:DeleteCondB",
		"deploy:Insert:insert into MyInfra select theString as a, intPrimitive as b from SupportBean",
		"deployed:Insert",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"E3\"}",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"snapshot:CreateInfra:any:a,b",
		"send:SupportBean_A:{\"id\":\"XE2X\"}",
		"snapshot:CreateInfra:any:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean:{\"intPrimitive\":7,\"theString\":\"E7\"}",
		"snapshot:CreateInfra:any:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean_B:{\"id\":\"B1\"}",
		"snapshot:CreateInfra:any:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"undeploy-all",
	},
	"pattern-nw": {
		"deploy:CreateInfra:@name('CreateInfra') @public create window MyInfra#keepall as select theString as a, intPrimitive as b from SupportBean",
		"deployed:CreateInfra",
		"deploy:OnDelete:@name('OnDelete') on pattern [every ea=SupportBean_A or every eb=SupportBean_B] delete from MyInfra",
		"deployed:OnDelete",
		"deploy:Insert:insert into MyInfra select theString as a, intPrimitive as b from SupportBean",
		"deployed:Insert",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"snapshot:OnDelete:ordered:a,b",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"snapshot:OnDelete:ordered:a,b",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean_B:{\"id\":\"B1\"}",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"undeploy-all",
	},
	"pattern-table": {
		"deploy:CreateInfra:@name('CreateInfra') @public create table MyInfra(a string primary key, b int)",
		"deployed:CreateInfra",
		"deploy:OnDelete:@name('OnDelete') on pattern [every ea=SupportBean_A or every eb=SupportBean_B] delete from MyInfra",
		"deployed:OnDelete",
		"deploy:Insert:insert into MyInfra select theString as a, intPrimitive as b from SupportBean",
		"deployed:Insert",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean_B:{\"id\":\"B1\"}",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"undeploy-all",
	},
	"deleteall-nw": {
		"deploy:CreateInfra:@Name('CreateInfra') @public create window MyInfra#keepall as select theString as a, intPrimitive as b from SupportBean",
		"deployed:CreateInfra",
		"deploy:OnDelete:@Name('OnDelete') on SupportBean_A delete from MyInfra",
		"deployed:OnDelete",
		"deploy:Insert:@Name('Insert') insert into MyInfra select theString as a, intPrimitive as b from SupportBean",
		"deployed:Insert",
		"deploy:Select:@Name('Select') select irstream MyInfra.a as a, b from MyInfra as s1",
		"deployed:Select",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"snapshot:CreateInfra:ordered:a,b",
		"snapshot:OnDelete:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean_A:{\"id\":\"A2\"}",
		"snapshot:OnDelete:ordered:a,b",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"E3\"}",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean_A:{\"id\":\"A2\"}",
		"snapshot:OnDelete:ordered:a,b",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"undeploy-all",
	},
	"deleteall-table": {
		"deploy:CreateInfra:@Name('CreateInfra') @public create table MyInfra (a string primary key, b int)",
		"deployed:CreateInfra",
		"deploy:OnDelete:@Name('OnDelete') on SupportBean_A delete from MyInfra",
		"deployed:OnDelete",
		"deploy:Insert:@Name('Insert') insert into MyInfra select theString as a, intPrimitive as b from SupportBean",
		"deployed:Insert",
		"deploy:Select:@Name('Select') select irstream MyInfra.a as a, b from MyInfra as s1",
		"deployed:Select",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"snapshot:CreateInfra:ordered:a,b",
		"snapshot:OnDelete:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean_A:{\"id\":\"A2\"}",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"E3\"}",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"send:SupportBean_A:{\"id\":\"A2\"}",
		"snapshot:CreateInfra:ordered:a,b",
		"faf:count:select count(*) as c0 from MyInfra:c0",
		"undeploy-all",
	},
}

// infraNWTableOnDeleteStepKey renders a raw step object into its pinned
// string form. Fields are read from the raw JSON because compat.Step does
// not carry the fields array.
func infraNWTableOnDeleteStepKey(object map[string]json.RawMessage, operation string) (string, error) {
	stringField := func(name string) (string, error) {
		var value string
		if err := json.Unmarshal(object[name], &value); err != nil {
			return "", fmt.Errorf("step field %q must be a string", name)
		}
		return value, nil
	}
	fieldsList := func() (string, error) {
		var values []string
		if err := json.Unmarshal(object["fields"], &values); err != nil {
			return "", fmt.Errorf("step fields must be a string array")
		}
		return joinStrings(values, ","), nil
	}
	switch operation {
	case "deploy":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		epl, err := stringField("epl")
		if err != nil {
			return "", err
		}
		return "deploy:" + statement + ":" + epl, nil
	case "deployed":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		return "deployed:" + statement, nil
	case "send":
		eventType, err := stringField("eventType")
		if err != nil {
			return "", err
		}
		var payload map[string]any
		if err := json.Unmarshal(object["payload"], &payload); err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		canonical, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		return "send:" + eventType + ":" + string(canonical), nil
	case "snapshot":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		mode, err := stringField("mode")
		if err != nil {
			return "", err
		}
		fields, err := fieldsList()
		if err != nil {
			return "", err
		}
		return "snapshot:" + statement + ":" + mode + ":" + fields, nil
	case "faf":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		epl, err := stringField("epl")
		if err != nil {
			return "", err
		}
		fields, err := fieldsList()
		if err != nil {
			return "", err
		}
		return "faf:" + statement + ":" + epl + ":" + fields, nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

func joinStrings(values []string, sep string) string {
	result := ""
	for index, value := range values {
		if index > 0 {
			result += sep
		}
		result += value
	}
	return result
}
