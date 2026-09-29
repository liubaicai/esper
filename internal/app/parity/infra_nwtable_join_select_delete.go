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
	infraNWTableJoinSelectDeleteID          = "infra-nwtable-join-select-delete"
	infraNWTableJoinSelectDeleteDescription = "InfraNWTableJoin and InfraNWTableOnSelectWDelete sibling slices: a continuous join of a keepall named window or primary-key table against a SupportBean#keepall stream on cid/theString equality, then an on-trigger select-and-delete whose ungrouped aggregate fold projects window(win.*) over the matched store rows and removes exactly that match set, with iterator probes on the create statement and a SODA re-deploy of the same select-delete text (Java sources regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableJoin.java and InfraNWTableOnSelectWDelete.java)."
	infraNWTableJoinSelectDeleteJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableJoinSelectDeleteSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableJoin.java"

	infraNWTableJoinSelectDeleteSchema   = "@public @buseventtype create schema MyEvent(cid string);"
	infraNWTableJoinSelectDeleteCreateNW = "@public create window MyInfra.win:keepall() as MyEvent"
	infraNWTableJoinSelectDeleteCreateTB = "@public create table MyInfra(cid string primary key)"
	infraNWTableJoinSelectDeleteInsert   = "insert into MyInfra select * from MyEvent"
	infraNWTableJoinSelectDeleteJoin     = "@name('s0') select ce.cid as c0, sb.intPrimitive as c1 from MyInfra as ce, SupportBean#keepall() as sb where sb.theString = ce.cid"

	infraNWTableJoinSelectDeleteStoreNW  = "@name('create') @public create window MyInfra#keepall as SupportBean"
	infraNWTableJoinSelectDeleteStoreTB  = "@name('create') @public create table MyInfra (theString string primary key, intPrimitive int primary key)"
	infraNWTableJoinSelectDeleteInsertSB = "insert into MyInfra select theString, intPrimitive from SupportBean"
	infraNWTableJoinSelectDeleteSelDel   = "@name('s0') on SupportBean_S0 as s0 select and delete window(win.*).aggregate(0,(result,value) => result+value.intPrimitive) as c0 from MyInfra as win where s0.p00=win.theString"
)

var (
	infraNWTableJoinSelectDeleteJavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableJoin.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnSelectWDelete.java",
	}
	infraNWTableJoinSelectDeleteJavaRuntimeIDs = []string{
		"java-runtime-9aad0c9a0b81e251f6d4",
		"java-runtime-333f1a440da03d4b266a",
		"java-runtime-27d8980edc91c4593d34",
		"java-runtime-60c74e5e717dc6d9330e",
	}
	infraNWTableJoinSelectDeleteJavaExecutions = []string{
		"InfraNWTableJoinSimple{namedWindow=true}",
		"InfraNWTableJoinSimple{namedWindow=false}",
		"InfraNWTableOnSelectWDeleteAssertion{namedWindow=true}",
		"InfraNWTableOnSelectWDeleteAssertion{namedWindow=false}",
	}
	infraNWTableJoinSelectDeleteJavaStaticIDs = []string{
		"java-675ca69dbc976b4c4448",
		"java-675ca69dbc976b4c4448",
		"java-5a29ff903cb7fd7a100d",
		"java-5a29ff903cb7fd7a100d",
	}
	infraNWTableJoinSelectDeleteCases = []string{
		"join-nw",
		"join-table",
		"seldel-nw",
		"seldel-table",
	}
	infraNWTableJoinSelectDeleteOrdinals = []int{0, 1, 0, 1}
)
var (
	infraNWTableJoinSelectDeleteCaseEPLs = []string{
		infraNWTableJoinSelectDeleteSchema + "\n" + infraNWTableJoinSelectDeleteCreateNW + ";\n" + infraNWTableJoinSelectDeleteInsert + ";\n" + infraNWTableJoinSelectDeleteJoin + ";\n",
		infraNWTableJoinSelectDeleteSchema + "\n" + infraNWTableJoinSelectDeleteCreateTB + ";\n" + infraNWTableJoinSelectDeleteInsert + ";\n" + infraNWTableJoinSelectDeleteJoin + ";\n",
		infraNWTableJoinSelectDeleteStoreNW + ";\n" + infraNWTableJoinSelectDeleteInsertSB + ";\n" + infraNWTableJoinSelectDeleteSelDel + ";\n",
		infraNWTableJoinSelectDeleteStoreTB + ";\n" + infraNWTableJoinSelectDeleteInsertSB + ";\n" + infraNWTableJoinSelectDeleteSelDel + ";\n",
	}
	infraNWTableJoinSelectDeleteCaseObservations = []string{
		"deployed+listener; keepall window MyInfra fed by the MyEvent insert and declared in one module with the @buseventtype MyEvent schema; the join select ce.cid,sb.intPrimitive emits {c0='C2',c1=1} on SupportBean(C2,1) and {c0='C1',c1=4} on SupportBean(C1,4) once C1/C2/C3 sit in the store",
		"deployed+listener; cid-PK table MyInfra fed by the MyEvent insert and declared in one module with the @buseventtype MyEvent schema; the join select ce.cid,sb.intPrimitive emits {c0='C2',c1=1} on SupportBean(C2,1) and {c0='C1',c1=4} on SupportBean(C1,4) once C1/C2/C3 sit in the store",
		"deployed+listener+snapshot; keepall window MyInfra over SupportBean fed by the theString/intPrimitive insert: the create iterator probes {E1,E2} then {E2} then {E2,E2,E2} then empty while s0 emits c0=1 then c0=9",
		"deployed+listener+snapshot; composite-PK table MyInfra(theString, intPrimitive) fed by the same insert: the create iterator probes {E1,E2} then {E2} then {E2,E2,E2} then empty while s0 emits c0=1 then c0=9",
	}
)

type infraNWTableJoinSelectDeleteBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

type infraNWTableJoinSelectDeleteS0 struct {
	ID  int32  `esper:"id"`
	P00 string `esper:"p00"`
}

func infraNWTableJoinSelectDeleteIsJoin(caseName string) bool {
	return caseName == "join-nw" || caseName == "join-table"
}

func infraNWTableJoinSelectDeleteIsTable(caseName string) bool {
	return caseName == "join-table" || caseName == "seldel-table"
}

// runInfraNWTableJoinSelectDeleteScenario replays the four executions: each
// case builds a fresh environment, deploys the pinned statements, sends the
// pinned events, and records listener batches plus store-iterator snapshots
// exactly where the Java assertions observe them.
func runInfraNWTableJoinSelectDeleteScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableJoinSelectDeleteCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableJoinSelectDeleteCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableJoinSelectDeleteID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableJoinSelectDeleteID)
	}
	return trace, nil
}

func runInfraNWTableJoinSelectDeleteCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	isJoin := infraNWTableJoinSelectDeleteIsJoin(caseName)
	isTable := infraNWTableJoinSelectDeleteIsTable(caseName)
	if _, err := esper.RegisterStruct[infraNWTableJoinSelectDeleteBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if !isJoin {
		if _, err := esper.RegisterStruct[infraNWTableJoinSelectDeleteS0](env, "SupportBean_S0"); err != nil {
			return compat.Trace{}, err
		}
	}
	if isJoin {
		// create schema MyEvent(cid string) — Java registers a map event
		// type; the MyInfra store copies its layout.
		myEvent, err := esper.NewMapSchema("MyEvent", []esper.FieldSpec{
			esper.FieldDef("cid", reflect.TypeOf("")),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterSchema(myEvent); err != nil {
			return compat.Trace{}, err
		}
		if isTable {
			if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("cid"),
			}); err != nil {
				return compat.Trace{}, err
			}
		} else {
			if _, err := esper.CreateNamedWindow(env, "MyInfra", myEvent, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return compat.Trace{}, err
			}
		}
	} else {
		if isTable {
			if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("theString"),
				esper.PrimaryKeyColumn[int32]("intPrimitive"),
			}); err != nil {
				return compat.Trace{}, err
			}
		} else {
			schema, ok := env.Schema("SupportBean")
			if !ok {
				return compat.Trace{}, fmt.Errorf("SupportBean schema missing")
			}
			if _, err := esper.CreateNamedWindow(env, "MyInfra", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return compat.Trace{}, err
			}
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableJoinSelectDeleteJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableJoinSelectDeleteID}
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
	defer func() {
		for _, deployment := range deployments {
			_ = deployment.Undeploy(context.Background())
		}
	}()
	statements := map[string]*esper.Statement{}
	deployed := map[string]bool{}
	s0Subscribed := false
	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			// Store declarations are environment-level in Go: the schema+
			// window/table module deploy of the join cases only registers
			// the marker label, matching the Java compileDeploy boundary.
			if isJoin && step.Statement == "create" {
				deployed[step.Statement] = true
				continue
			}
			plan, err := infraNWTableJoinSelectDeleteBuild(env, caseName, step.Statement)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
			}
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
				statements[step.Statement] = deploymentStatements[0]
			}
			for _, statement := range deploymentStatements {
				// Java attaches the s0 listener at the first deploy only; the
				// SODA re-deploy of the same text stays unlistened.
				if step.Statement == "s0" && statement.Name() == "s0" && !s0Subscribed {
					s0Subscribed = true
					name := step.Statement
					if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record(name, batch)
						return nil
					}); err != nil {
						return compat.Trace{}, err
					}
				}
			}
			deployed[step.Statement] = true
		case "deployed":
			if !deployed[step.Statement] {
				return compat.Trace{}, fmt.Errorf("deployed marker for unknown statement %q", step.Statement)
			}
			sequence[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequence[step.Statement+":deployed"],
				Time:      "1970-01-01T00:00:00Z",
			})
		case "send":
			payload, err := decodeInfraNWTableJoinSelectDeletePayload(step)
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
		case "undeploy-all":
		default:
			return compat.Trace{}, fmt.Errorf("unsupported step op %q", step.Op)
		}
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("case %q produced no records", caseName)
	}
	return trace, nil
}

// infraNWTableJoinSelectDeleteBuild maps each deployed statement label to its
// fluent plan. The store declarations are environment-level, so "create" is
// the named consumer statement the iterator probes read.
func infraNWTableJoinSelectDeleteBuild(env *esper.Environment, caseName, statement string) (esper.Plan, error) {
	isJoin := infraNWTableJoinSelectDeleteIsJoin(caseName)
	isTable := infraNWTableJoinSelectDeleteIsTable(caseName)
	sourceBean := esper.From[infraNWTableJoinSelectDeleteBean](env, "SupportBean")
	sourceS0 := esper.From[infraNWTableJoinSelectDeleteS0](env, "SupportBean_S0")
	sourceEvent := esper.From[map[string]any](env, "MyEvent")
	switch statement {
	case "create":
		if isTable {
			return env.Build(esper.FromTable(env, "MyInfra").Query(esper.StatementName("create")))
		}
		return env.Build(esper.FromNamedWindow(env, "MyInfra").Query(esper.StatementName("create")))
	case "insert":
		if isJoin {
			// insert into MyInfra select * from MyEvent — the whole map event
			// routes into the store; copy-matching covers both targets.
			if isTable {
				return env.Build(esper.OnEvent(sourceEvent).InsertIntoTable("MyInfra",
					esper.SetColumn("cid", esper.Field[map[string]any, string]("cid"))).Query(esper.StatementName("insert")))
			}
			return env.Build(esper.OnEvent(sourceEvent).InsertIntoNamedWindow("MyInfra",
				esper.CopyMatchingFields()).Query(esper.StatementName("insert")))
		}
		if isTable {
			return env.Build(esper.OnEvent(sourceBean).InsertIntoTable("MyInfra",
				esper.SetColumn("theString", esper.Field[infraNWTableJoinSelectDeleteBean, string]("theString")),
				esper.SetColumn("intPrimitive", esper.Field[infraNWTableJoinSelectDeleteBean, int32]("intPrimitive"))).Query(esper.StatementName("insert")))
		}
		return env.Build(esper.OnEvent(sourceBean).InsertIntoNamedWindow("MyInfra",
			esper.CopyMatchingFields()).Query(esper.StatementName("insert")))
	case "s0":
		if isJoin {
			// select ce.cid as c0, sb.intPrimitive as c1 from MyInfra as ce,
			// SupportBean#keepall() as sb where sb.theString = ce.cid — the
			// infra store is the record side, the keepall bean window the
			// other; the equi-condition becomes the join key.
			var store esper.RecordStream
			if isTable {
				store = esper.FromTable(env, "MyInfra")
			} else {
				store = esper.FromNamedWindow(env, "MyInfra")
			}
			return env.Build(esper.JoinMany(
				esper.JoinRecordSource(store),
				esper.JoinRecordSource(sourceBean.Window(esper.KeepAll()).AsRecord()),
			).On(
				esper.OnSourcesEqual(0, esper.Field[any, string]("cid"), 1, esper.Field[any, string]("theString")),
			).Select(
				esper.SelectFrom(0, "c0", esper.Field[any, string]("cid")),
				esper.SelectFrom(1, "c1", esper.Field[any, int32]("intPrimitive")),
			).Query(esper.StatementName("s0")))
		}
		// on SupportBean_S0 select and delete
		//   window(win.*).aggregate(0,(result,value) => result+value.intPrimitive) as c0
		//   from MyInfra as win where s0.p00=win.theString — the ungrouped sum
		//   folds the matched set into one c0 row, then the same match set is
		//   deleted while the trigger event is still being processed.
		if isTable {
			return env.Build(esper.OnEvent(sourceS0).SelectDeleteFromTable("MyInfra",
				esper.Equal[string](esper.Field[infraNWTableJoinSelectDeleteS0, string]("p00"), esper.TableField[string]("theString")),
				esper.Alias("c0", esper.Sum[int32](esper.TableField[int32]("intPrimitive")))).Query(esper.StatementName("s0")))
		}
		return env.Build(esper.OnEvent(sourceS0).SelectDeleteFromNamedWindow("MyInfra",
			esper.Equal[string](esper.Field[infraNWTableJoinSelectDeleteS0, string]("p00"), esper.NamedWindowField[string]("theString")),
			esper.Alias("c0", esper.Sum[int32](esper.NamedWindowField[int32]("intPrimitive")))).Query(esper.StatementName("s0")))
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q", statement)
}

// decodeInfraNWTableJoinSelectDeletePayload decodes the send payloads: MyEvent
// map events carry cid; SupportBean carries theString+intPrimitive;
// SupportBean_S0 carries id+p00.
func decodeInfraNWTableJoinSelectDeletePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(step.Payload, &fields); err != nil {
		return nil, fmt.Errorf("%s send payload must be an object: %w", infraNWTableJoinSelectDeleteID, err)
	}
	decode := func(name string, target any) error {
		raw, ok := fields[name]
		if !ok {
			return fmt.Errorf("%s send payload missing %q", infraNWTableJoinSelectDeleteID, name)
		}
		if err := json.Unmarshal(raw, target); err != nil {
			return fmt.Errorf("%s send payload %q: %w", infraNWTableJoinSelectDeleteID, name, err)
		}
		return nil
	}
	wantKeys := map[string][]string{
		"MyEvent":        {"cid"},
		"SupportBean":    {"theString", "intPrimitive"},
		"SupportBean_S0": {"id", "p00"},
	}
	expected, known := wantKeys[step.EventType]
	if !known {
		return nil, fmt.Errorf("%s unsupported send event type %q", infraNWTableJoinSelectDeleteID, step.EventType)
	}
	if len(fields) != len(expected) {
		return nil, fmt.Errorf("%s %s send payload keys = %v, want %v", infraNWTableJoinSelectDeleteID, step.EventType, fields, expected)
	}
	switch step.EventType {
	case "MyEvent":
		payload := map[string]any{}
		var cid string
		if err := decode("cid", &cid); err != nil {
			return nil, err
		}
		payload["cid"] = cid
		return payload, nil
	case "SupportBean":
		payload := infraNWTableJoinSelectDeleteBean{}
		if err := decode("theString", &payload.TheString); err != nil {
			return nil, err
		}
		if err := decode("intPrimitive", &payload.IntPrimitive); err != nil {
			return nil, err
		}
		return payload, nil
	case "SupportBean_S0":
		payload := infraNWTableJoinSelectDeleteS0{}
		if err := decode("id", &payload.ID); err != nil {
			return nil, err
		}
		if err := decode("p00", &payload.P00); err != nil {
			return nil, err
		}
		return payload, nil
	}
	return nil, fmt.Errorf("%s unsupported send event type %q", infraNWTableJoinSelectDeleteID, step.EventType)
}

func loadInfraNWTableJoinSelectDeleteScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableJoinSelectDeleteID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableJoinSelectDeleteID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableJoinSelectDeleteID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableJoinSelectDeleteID, err)
	}
	if err := requireInfraNWTableOnDeleteFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaSourceFiles", "javaRuntimes",
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableJoinSelectDeleteID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableJoinSelectDeleteID ||
		metadata.Description != infraNWTableJoinSelectDeleteDescription ||
		metadata.JavaCommit != infraNWTableJoinSelectDeleteJavaCommit ||
		metadata.JavaSource != infraNWTableJoinSelectDeleteSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableJoinSelectDeleteID)
	}
	if err := validateInfraNWTableOnDeleteStringArray(root["javaRuntimes"], infraNWTableJoinSelectDeleteJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnDeleteStringArray(root["javaNames"], infraNWTableJoinSelectDeleteJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnDeleteStringArray(root["javaStaticIds"], infraNWTableJoinSelectDeleteJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnDeleteStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnDeleteStringArray(root["javaSourceFiles"], infraNWTableJoinSelectDeleteJavaSources, "javaSourceFiles"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableJoinSelectDeleteCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableJoinSelectDeleteID, len(infraNWTableJoinSelectDeleteCases))
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
		if definition.Case != infraNWTableJoinSelectDeleteCases[index] ||
			definition.Ordinal != infraNWTableJoinSelectDeleteOrdinals[index] ||
			definition.RuntimeID != infraNWTableJoinSelectDeleteJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableJoinSelectDeleteJavaExecutions[index] ||
			definition.Observation != infraNWTableJoinSelectDeleteCaseObservations[index] ||
			definition.EPL != infraNWTableJoinSelectDeleteCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableJoinSelectDeleteID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableJoinSelectDeleteID)
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
			if _, err := decodeInfraNWTableJoinSelectDeletePayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableOnDeleteFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
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
	return scenario, nil
}
