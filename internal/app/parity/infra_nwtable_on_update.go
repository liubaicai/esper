package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	infraNWTableOnUpdateID          = "infra-nwtable-on-update"
	infraNWTableOnUpdateDescription = "InfraNWTableOnUpdate on-trigger update semantics over a keepall named window and over a primary-key table: unqualified set/where names resolving infra-row versus trigger-event properties with insert-remove update delivery, a self-correlated subquery assignment reading the pre-update row (ESPER-507), and a group-by-int-array aggregated subquery assignment whose multi-row result becomes null (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnUpdate.java)."
	infraNWTableOnUpdateJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableOnUpdateSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnUpdate.java"

	infraNWTableOnUpdateCreateSceneOneNW  = "@name('create') @public create window MyInfra.win:keepall() as SupportBean"
	infraNWTableOnUpdateCreateSceneOneTbl = "@name('create') @public create table MyInfra(theString string, intPrimitive int primary key)"
	infraNWTableOnUpdateInsertSceneOne    = "@name('insert') insert into MyInfra select theString, intPrimitive from SupportBean"
	infraNWTableOnUpdateUpdateSceneOne    = "@name('update') on SupportBean_S0 update MyInfra set theString = p00 where intPrimitive = id"

	infraNWTableOnUpdateCreateSubqSelfNW  = "@name('create') @public create window MyInfraSS#keepall as SupportBean"
	infraNWTableOnUpdateCreateSubqSelfTbl = "@name('create') @public create table MyInfraSS(theString string primary key, intPrimitive int)"
	infraNWTableOnUpdateInsertSubqSelf    = "insert into MyInfraSS select theString, intPrimitive from SupportBean"
	infraNWTableOnUpdateUpdateSubqSelf    = "@Name(\"Self Update\")\non SupportBean_A c\nupdate MyInfraSS s\nset intPrimitive = (select intPrimitive from MyInfraSS t where t.theString = c.id) + 1\nwhere s.theString = c.id"

	infraNWTableOnUpdateCreateMultikeyNW  = "@name('create') @public create window MyInfra#keepall() as (value int)"
	infraNWTableOnUpdateCreateMultikeyTbl = "@name('create') @public create table MyInfra(value int)"
	infraNWTableOnUpdateFafInsertMultikey = "insert into MyInfra select 0 as value"
	infraNWTableOnUpdateUpdateMultikey    = "on SupportBean update MyInfra set value = (select sum(value) as c0 from SupportEventWithIntArray#keepall group by array)"
)

var (
	infraNWTableOnUpdateJavaSources = []string{
		infraNWTableOnUpdateSource,
	}
	infraNWTableOnUpdateJavaRuntimeIDs = []string{
		"java-runtime-f8090e148364d7b15116",
		"java-runtime-922adbe6628b17d5ec61",
		"java-runtime-046926326cad5f0172cd",
		"java-runtime-702d919aaadcbb188467",
		"java-runtime-5cc56f78a52d6a452948",
		"java-runtime-b23b2fbc3cc233237ac9",
	}
	infraNWTableOnUpdateJavaExecutions = []string{
		"InfraNWTableOnUpdateSceneOne{namedWindow=true}",
		"InfraNWTableOnUpdateSceneOne{namedWindow=false}",
		"InfraSubquerySelf{namedWindow=true}",
		"InfraSubquerySelf{namedWindow=false}",
		"InfraSubqueryMultikeyWArray{namedWindow=true}",
		"InfraSubqueryMultikeyWArray{namedWindow=false}",
	}
	infraNWTableOnUpdateJavaStaticIDs = []string{
		"java-bc54a78188b2249a0a9f",
		"java-bc54a78188b2249a0a9f",
		"java-c67318a7541b42eb7940",
		"java-c67318a7541b42eb7940",
		"java-9ab08590ab6533e21139",
		"java-9ab08590ab6533e21139",
	}
	infraNWTableOnUpdateCases = []string{
		"sceneone-nw",
		"sceneone-table",
		"subqself-nw",
		"subqself-table",
		"multikey-nw",
		"multikey-table",
	}
	infraNWTableOnUpdateOrdinals = []int{0, 1, 4, 5, 6, 7}
)

// infraNWTableOnUpdateBean mirrors the full SupportBean schema: the Java
// sceneone/subqself windows keep every property, so listener and iterator
// rows carry all twenty fields with Java's default values.
type infraNWTableOnUpdateBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int64    `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int64   `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *float64 `esper:"bigDecimal"`
	BigInteger      *int64   `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

type infraNWTableOnUpdateS0 struct {
	ID  int64  `esper:"id"`
	P00 string `esper:"p00"`
}

type infraNWTableOnUpdateA struct {
	ID string `esper:"id"`
}

type infraNWTableOnUpdateIntArray struct {
	ID    string  `esper:"id"`
	Array []int64 `esper:"array"`
	Value int64   `esper:"value"`
}

func infraNWTableOnUpdateIsTable(caseName string) bool {
	return caseName == "sceneone-table" || caseName == "subqself-table" || caseName == "multikey-table"
}

func infraNWTableOnUpdateInfraName(caseName string) string {
	if caseName == "subqself-nw" || caseName == "subqself-table" {
		return "MyInfraSS"
	}
	return "MyInfra"
}

// runInfraNWTableOnUpdateScenario replays the six InfraNWTableOnUpdate
// executions: each case deploys the pinned statements, sends the pinned
// events, and records listener batches, iterator snapshots and undeploy
// markers in Java's observable order.
func runInfraNWTableOnUpdateScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableOnUpdateCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableOnUpdateCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableOnUpdateID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableOnUpdateID)
	}
	return trace, nil
}

func runInfraNWTableOnUpdateCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableOnUpdateBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnUpdateS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnUpdateA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnUpdateIntArray](env, "SupportEventWithIntArray"); err != nil {
		return compat.Trace{}, err
	}

	isTable := infraNWTableOnUpdateIsTable(caseName)
	infraName := infraNWTableOnUpdateInfraName(caseName)
	if err := infraNWTableOnUpdateCreateInfra(env, caseName, infraName, isTable); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableOnUpdateJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableOnUpdateID}
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
	pinned := infraNWTableOnUpdateCaseSteps[caseName]
	for stepIndex, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			if step.Statement == "FafInsert" {
				// Java executes the seed insert via compileExecuteFAFNoResult:
				// a fire-and-forget insert-into with no result rows.
				plan, err := infraNWTableOnUpdateBuildFAFInsert(env, infraName, isTable)
				if err != nil {
					return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
				}
				if _, err := engine.ExecuteFireAndForget(ctx, plan); err != nil {
					return compat.Trace{}, fmt.Errorf("faf insert %q: %w", step.Statement, err)
				}
				continue
			}
			plan, err := infraNWTableOnUpdateBuild(env, caseName, step.Statement, infraName, isTable)
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
				// Java deploys the insert and on-update triggers without
				// @name: bind the deployment's single anonymous statement.
				statements[step.Statement] = deploymentStatements[0]
			}
			for _, statement := range deploymentStatements {
				if infraNWTableOnUpdateListened(caseName, step.Statement) && statement.Name() == step.Statement {
					name := step.Statement
					if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record(name, batch)
						return nil
					}); err != nil {
						return compat.Trace{}, err
					}
				}
			}
		case "deployed":
			if _, ok := statements[step.Statement]; !ok {
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
			payload, err := decodeInfraNWTableOnUpdatePayload(step)
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
			rows = projectInfraNWTableOnUpdateRows(rows, infraNWTableOnUpdateSnapshotFields(pinned, stepIndex))
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

// infraNWTableOnUpdateCreateInfra pre-creates the named window or table that
// the Java @public create statement establishes, so the 'create' deploy step
// can attach the consumer query the Java listener observes.
func infraNWTableOnUpdateCreateInfra(env *esper.Environment, caseName string, infraName string, isTable bool) error {
	switch caseName {
	case "sceneone-nw", "subqself-nw":
		schema, ok := env.Schema("SupportBean")
		if !ok {
			return fmt.Errorf("SupportBean schema is missing")
		}
		return createKeepAllNamedWindow(env, infraName, schema)
	case "multikey-nw":
		projection, err := esper.NewMapSchema("MyInfraValueSchema", []esper.FieldSpec{
			esper.FieldDef("value", reflect.TypeOf(int64(0))),
		})
		if err != nil {
			return err
		}
		if err := env.RegisterSchema(projection); err != nil {
			return err
		}
		return createKeepAllNamedWindow(env, infraName, projection)
	case "sceneone-table":
		_, err := esper.CreateTable(env, infraName, []esper.TableColumn{
			esper.TableColumnOf[string]("theString"),
			esper.PrimaryKeyColumn[int64]("intPrimitive"),
		})
		return err
	case "subqself-table":
		_, err := esper.CreateTable(env, infraName, []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("theString"),
			esper.TableColumnOf[int64]("intPrimitive"),
		})
		return err
	case "multikey-table":
		_, err := esper.CreateTable(env, infraName, []esper.TableColumn{
			esper.TableColumnOf[int64]("value"),
		})
		return err
	}
	return fmt.Errorf("unexpected case %q", caseName)
}

func createKeepAllNamedWindow(env *esper.Environment, name string, schema esper.Schema) error {
	_, err := esper.CreateNamedWindow(env, name, schema, esper.NamedWindowRetention(esper.KeepAll()))
	return err
}

// infraNWTableOnUpdateListened reports whether the Java execution attaches a
// listener to the named statement. SceneOne listens 'create' and 'update';
// SubquerySelf listens nothing; MultikeyWArray listens 'create'. Table
// 'create' listeners never fire in Java (tables do not stream to listeners),
// so Go attaches them too for fidelity where the API allows.
func infraNWTableOnUpdateListened(caseName string, statement string) bool {
	switch caseName {
	case "sceneone-nw", "sceneone-table":
		return statement == "create" || statement == "update"
	case "multikey-nw", "multikey-table":
		return statement == "create"
	}
	return false
}

func infraNWTableOnUpdateBuild(env *esper.Environment, caseName string, statement string, infraName string, isTable bool) (esper.Plan, error) {
	switch caseName {
	case "sceneone-nw", "sceneone-table":
		return infraNWTableOnUpdateBuildSceneOne(env, statement, infraName, isTable)
	case "subqself-nw", "subqself-table":
		return infraNWTableOnUpdateBuildSubqSelf(env, statement, infraName, isTable)
	case "multikey-nw", "multikey-table":
		return infraNWTableOnUpdateBuildMultikey(env, statement, infraName, isTable)
	}
	return esper.Plan{}, fmt.Errorf("unexpected case %q", caseName)
}

func infraNWTableOnUpdateBuildSceneOne(env *esper.Environment, statement string, infraName string, isTable bool) (esper.Plan, error) {
	sourceS0 := esper.From[infraNWTableOnUpdateS0](env, "SupportBean_S0")
	sourceBean := esper.From[infraNWTableOnUpdateBean](env, "SupportBean")
	switch statement {
	case "create":
		if isTable {
			return env.Build(esper.FromTable(env, infraName).Query(esper.StatementName("create"), esper.WithOldStream()))
		}
		return env.Build(esper.FromNamedWindow(env, infraName).Query(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		if isTable {
			return env.Build(esper.OnEvent(sourceBean).InsertIntoTable(infraName,
				esper.SetColumn("theString", esper.Field[infraNWTableOnUpdateBean, string]("theString")),
				esper.SetColumn("intPrimitive", esper.Field[infraNWTableOnUpdateBean, int64]("intPrimitive"))).Query())
		}
		return env.Build(esper.OnEvent(sourceBean).InsertIntoNamedWindow(infraName,
			infraNWTableOnUpdateInsertAssignments()...).Query())
	case "update":
		assignment := esper.SetColumn("theString", esper.Field[infraNWTableOnUpdateS0, string]("p00"))
		if isTable {
			return env.Build(esper.OnEvent(sourceS0).UpdateTableWhere(infraName,
				esper.Equal[int64](esper.TableField[int64]("intPrimitive"), esper.Field[infraNWTableOnUpdateS0, int64]("id")),
				assignment).Query(esper.StatementName("update"), esper.WithOldStream()))
		}
		return env.Build(esper.OnEvent(sourceS0).UpdateNamedWindow(infraName,
			esper.Equal[int64](esper.NamedWindowField[int64]("intPrimitive"), esper.Field[infraNWTableOnUpdateS0, int64]("id")),
			assignment).Query(esper.StatementName("update"), esper.WithOldStream()))
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q", statement)
}

func infraNWTableOnUpdateBuildSubqSelf(env *esper.Environment, statement string, infraName string, isTable bool) (esper.Plan, error) {
	sourceBean := esper.From[infraNWTableOnUpdateBean](env, "SupportBean")
	sourceA := esper.From[infraNWTableOnUpdateA](env, "SupportBean_A")
	switch statement {
	case "create":
		if isTable {
			return env.Build(esper.FromTable(env, infraName).Query(esper.StatementName("create"), esper.WithOldStream()))
		}
		return env.Build(esper.FromNamedWindow(env, infraName).Query(esper.StatementName("create"), esper.WithOldStream()))
	case "Insert":
		if isTable {
			return env.Build(esper.OnEvent(sourceBean).InsertIntoTable(infraName,
				esper.SetColumn("theString", esper.Field[infraNWTableOnUpdateBean, string]("theString")),
				esper.SetColumn("intPrimitive", esper.Field[infraNWTableOnUpdateBean, int64]("intPrimitive"))).Query())
		}
		return env.Build(esper.OnEvent(sourceBean).InsertIntoNamedWindow(infraName,
			infraNWTableOnUpdateInsertAssignments()...).Query())
	case "Self Update":
		// Java: set intPrimitive = (select intPrimitive from MyInfraSS t
		// where t.theString = c.id) + 1 where s.theString = c.id — the
		// subquery is correlated to the trigger event (c.id) and reads the
		// pre-update value of the same infra row.
		var inner esper.RecordStream
		if isTable {
			inner = esper.FromTable(env, infraName)
		} else {
			inner = esper.FromNamedWindow(env, infraName)
		}
		subquery := esper.SubqueryValueWithOptions[int64](inner,
			esper.Field[any, int64]("intPrimitive"),
			esper.SubqueryWhere(esper.Equal[string](
				esper.Field[any, string]("theString"), esper.OuterField[string]("id"))),
			esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))
		assignment := esper.SetColumn("intPrimitive", esper.Add[int64](subquery, esper.Literal(int64(1))))
		if isTable {
			return env.Build(esper.OnEvent(sourceA).UpdateTableWhere(infraName,
				esper.Equal[string](esper.TableField[string]("theString"), esper.Field[infraNWTableOnUpdateA, string]("id")),
				assignment).Query())
		}
		return env.Build(esper.OnEvent(sourceA).UpdateNamedWindow(infraName,
			esper.Equal[string](esper.NamedWindowField[string]("theString"), esper.Field[infraNWTableOnUpdateA, string]("id")),
			assignment).Query())
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q", statement)
}

func infraNWTableOnUpdateBuildMultikey(env *esper.Environment, statement string, infraName string, isTable bool) (esper.Plan, error) {
	sourceBean := esper.From[infraNWTableOnUpdateBean](env, "SupportBean")
	switch statement {
	case "create":
		if isTable {
			return env.Build(esper.FromTable(env, infraName).Query(esper.StatementName("create"), esper.WithOldStream()))
		}
		return env.Build(esper.FromNamedWindow(env, infraName).Query(esper.StatementName("create"), esper.WithOldStream()))
	case "Update":
		// Java: set value = (select sum(value) as c0 from
		// SupportEventWithIntArray#keepall group by array) — a group-by-array
		// multikey aggregated subquery; multiple groups assign null.
		inner := esper.From[infraNWTableOnUpdateIntArray](env, "SupportEventWithIntArray").
			Window(esper.KeepAll()).AsRecord()
		subquery := esper.SubqueryValueWithOptions[int64](inner,
			esper.Sum[int64](esper.Field[any, int64]("value")),
			esper.SubqueryGroupKey(esper.Field[any, []int64]("array")),
			esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))
		assignment := esper.SetColumn("value", subquery)
		if isTable {
			return env.Build(esper.OnEvent(sourceBean).UpdateTableWhere(infraName,
				esper.Literal(true), assignment).Query())
		}
		return env.Build(esper.OnEvent(sourceBean).UpdateNamedWindow(infraName,
			esper.Literal(true), assignment).Query())
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q", statement)
}

func infraNWTableOnUpdateBuildFAFInsert(env *esper.Environment, infraName string, isTable bool) (esper.Plan, error) {
	row := esper.InsertValues(esper.Literal(int64(0)))
	if isTable {
		return env.Build(esper.FromTable(env, infraName).OnDemand().InsertRows(row))
	}
	return env.Build(esper.FromNamedWindow(env, infraName).OnDemand().InsertRows(row))
}

func decodeInfraNWTableOnUpdatePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString    *string `json:"theString"`
			IntPrimitive int64   `json:"intPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean payload: %w", err)
		}
		event := infraNWTableOnUpdateBean{
			IntPrimitive:  payload.IntPrimitive,
			CharPrimitive: "\u0000",
		}
		if payload.TheString != nil {
			event.TheString = *payload.TheString
		}
		return event, nil
	case "SupportBean_S0":
		var payload struct {
			ID  int64  `json:"id"`
			P00 string `json:"p00"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0 payload: %w", err)
		}
		return infraNWTableOnUpdateS0{ID: payload.ID, P00: payload.P00}, nil
	case "SupportBean_A":
		var payload struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A payload: %w", err)
		}
		return infraNWTableOnUpdateA{ID: payload.ID}, nil
	case "SupportEventWithIntArray":
		var payload struct {
			ID    string  `json:"id"`
			Array []int64 `json:"array"`
			Value int64   `json:"value"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportEventWithIntArray payload: %w", err)
		}
		return infraNWTableOnUpdateIntArray{ID: payload.ID, Array: payload.Array, Value: payload.Value}, nil
	}
	return nil, fmt.Errorf("unexpected event type %q", step.EventType)
}

func requireInfraNWTableOnUpdateFields(object map[string]json.RawMessage, names ...string) error {
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

// loadInfraNWTableOnUpdateScenario enforces the strict scenario contract
// shared by the differential runners: no duplicate or unknown JSON fields,
// pinned metadata, pinned per-case runtime/execution/EPL, and a per-op step
// field whitelist followed by a full step-shape pin.
func loadInfraNWTableOnUpdateScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableOnUpdateID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableOnUpdateID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnUpdateID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnUpdateID, err)
	}
	if err := requireInfraNWTableOnUpdateFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableOnUpdateID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableOnUpdateID ||
		metadata.Description != infraNWTableOnUpdateDescription ||
		metadata.JavaCommit != infraNWTableOnUpdateJavaCommit ||
		metadata.JavaSource != infraNWTableOnUpdateSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableOnUpdateID)
	}
	if err := validateInfraNWTableOnUpdateStringArray(root["javaRuntimes"], infraNWTableOnUpdateJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnUpdateStringArray(root["javaNames"], infraNWTableOnUpdateJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnUpdateStringArray(root["javaStaticIds"], infraNWTableOnUpdateJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnUpdateStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableOnUpdateCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableOnUpdateID, len(infraNWTableOnUpdateCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableOnUpdateFields(object,
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
		if definition.Case != infraNWTableOnUpdateCases[index] ||
			definition.Ordinal != infraNWTableOnUpdateOrdinals[index] ||
			definition.RuntimeID != infraNWTableOnUpdateJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableOnUpdateJavaExecutions[index] ||
			definition.Observation != infraNWTableOnUpdateCaseObservations[index] ||
			definition.EPL != infraNWTableOnUpdateCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableOnUpdateID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableOnUpdateID)
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
			if err := requireInfraNWTableOnUpdateFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableOnUpdateFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableOnUpdateFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableOnUpdateFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableOnUpdatePayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableOnUpdateFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableOnUpdateFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWTableOnUpdateRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateInfraNWTableOnUpdateStringArray(raw json.RawMessage, expected []string, name string) error {
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

// infraNWTableOnUpdateCaseEPLs pins the first deploy EPL of each case, the
// value carried by the scenario cases[] metadata.
var infraNWTableOnUpdateCaseEPLs = []string{
	infraNWTableOnUpdateCreateSceneOneNW,
	infraNWTableOnUpdateCreateSceneOneTbl,
	infraNWTableOnUpdateCreateSubqSelfNW,
	infraNWTableOnUpdateCreateSubqSelfTbl,
	infraNWTableOnUpdateCreateMultikeyNW,
	infraNWTableOnUpdateCreateMultikeyTbl,
}

var infraNWTableOnUpdateCaseObservations = []string{
	"listener+iterator; on-update IR pairs over the keepall named window with the updated row reinserted at the tail of iteration order",
	"listener+iterator; same on-update over the primary-key table with no create-statement deliveries",
	"iterator; self-correlated subquery assignment reading the pre-update row of the same named window (ESPER-507)",
	"iterator; self-correlated subquery assignment reading the pre-update row of the same primary-key table (ESPER-507)",
	"listener+iterator; group-by-int-array aggregated subquery assignment over the named window, multi-row subquery result becoming null",
	"iterator; same group-by-int-array subquery assignment over the table with no listener deliveries",
}

// validateInfraNWTableOnUpdateRawSteps pins the complete step sequence per
// case against the raw JSON objects: deploy statements with byte-exact EPL,
// deployed markers, send event types with canonical payloads, snapshot reads,
// and undeploy-all terminators.
func validateInfraNWTableOnUpdateRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableOnUpdateCases {
		want, ok := infraNWTableOnUpdateCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableOnUpdateID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableOnUpdateID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnUpdateID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnUpdateID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableOnUpdateStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableOnUpdateID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned", infraNWTableOnUpdateID, caseName, index+1)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableOnUpdateID)
	}
	return nil
}

// infraNWTableOnUpdateStepKey renders a raw step object into its pinned
// string form. Fields are read from the raw JSON because compat.Step does
// not carry the fields array.
func infraNWTableOnUpdateStepKey(object map[string]json.RawMessage, operation string) (string, error) {
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
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// infraNWTableOnUpdateCaseSteps pins the exact op sequence per case:
// deploy statements with byte-exact EPL, deployed markers, send event types
// with canonical payloads, snapshot reads, and undeploy-all terminators.
var infraNWTableOnUpdateCaseSteps = map[string][]string{
	"sceneone-nw": {
		"deploy:create:@name('create') @public create window MyInfra.win:keepall() as SupportBean",
		"deployed:create",
		"deploy:insert:@name('insert') insert into MyInfra select theString, intPrimitive from SupportBean",
		"deployed:insert",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"A1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"B2\"}",
		"deploy:update:@name('update') on SupportBean_S0 update MyInfra set theString = p00 where intPrimitive = id",
		"deployed:update",
		"send:SupportBean_S0:{\"id\":1,\"p00\":\"X1\"}",
		"snapshot:create:ordered:theString,intPrimitive",
		"send:SupportBean_S0:{\"id\":2,\"p00\":\"X2\"}",
		"snapshot:create:any:theString,intPrimitive",
		"snapshot:create:any:theString,intPrimitive",
		"undeploy-all",
	},
	"sceneone-table": {
		"deploy:create:@name('create') @public create table MyInfra(theString string, intPrimitive int primary key)",
		"deployed:create",
		"deploy:insert:@name('insert') insert into MyInfra select theString, intPrimitive from SupportBean",
		"deployed:insert",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"A1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"B2\"}",
		"deploy:update:@name('update') on SupportBean_S0 update MyInfra set theString = p00 where intPrimitive = id",
		"deployed:update",
		"send:SupportBean_S0:{\"id\":1,\"p00\":\"X1\"}",
		"snapshot:create:any:theString,intPrimitive",
		"send:SupportBean_S0:{\"id\":2,\"p00\":\"X2\"}",
		"snapshot:create:any:theString,intPrimitive",
		"snapshot:create:any:theString,intPrimitive",
		"undeploy-all",
	},
	"subqself-nw": {
		"deploy:create:@name('create') @public create window MyInfraSS#keepall as SupportBean",
		"deployed:create",
		"deploy:Insert:insert into MyInfraSS select theString, intPrimitive from SupportBean",
		"deployed:Insert",
		"deploy:Self Update:@Name(\"Self Update\")\non SupportBean_A c\nupdate MyInfraSS s\nset intPrimitive = (select intPrimitive from MyInfraSS t where t.theString = c.id) + 1\nwhere s.theString = c.id",
		"deployed:Self Update",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":6,\"theString\":\"E2\"}",
		"send:SupportBean_A:{\"id\":\"E1\"}",
		"send:SupportBean_A:{\"id\":\"E1\"}",
		"send:SupportBean_A:{\"id\":\"E2\"}",
		"snapshot:create:any:theString,intPrimitive",
		"undeploy-all",
	},
	"subqself-table": {
		"deploy:create:@name('create') @public create table MyInfraSS(theString string primary key, intPrimitive int)",
		"deployed:create",
		"deploy:Insert:insert into MyInfraSS select theString, intPrimitive from SupportBean",
		"deployed:Insert",
		"deploy:Self Update:@Name(\"Self Update\")\non SupportBean_A c\nupdate MyInfraSS s\nset intPrimitive = (select intPrimitive from MyInfraSS t where t.theString = c.id) + 1\nwhere s.theString = c.id",
		"deployed:Self Update",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":6,\"theString\":\"E2\"}",
		"send:SupportBean_A:{\"id\":\"E1\"}",
		"send:SupportBean_A:{\"id\":\"E1\"}",
		"send:SupportBean_A:{\"id\":\"E2\"}",
		"snapshot:create:any:theString,intPrimitive",
		"undeploy-all",
	},
	"multikey-nw": {
		"deploy:create:@name('create') @public create window MyInfra#keepall() as (value int)",
		"deployed:create",
		"deploy:FafInsert:insert into MyInfra select 0 as value",
		"deploy:Update:on SupportBean update MyInfra set value = (select sum(value) as c0 from SupportEventWithIntArray#keepall group by array)",
		"deployed:Update",
		"send:SupportEventWithIntArray:{\"array\":[1,2],\"id\":\"E1\",\"value\":10}",
		"send:SupportEventWithIntArray:{\"array\":[1,2],\"id\":\"E2\",\"value\":11}",
		"send:SupportBean:{\"intPrimitive\":0,\"theString\":null}",
		"snapshot:create:any:value",
		"send:SupportEventWithIntArray:{\"array\":[1,2],\"id\":\"E3\",\"value\":12}",
		"send:SupportBean:{\"intPrimitive\":0,\"theString\":null}",
		"snapshot:create:any:value",
		"send:SupportEventWithIntArray:{\"array\":[1],\"id\":\"E4\",\"value\":13}",
		"send:SupportBean:{\"intPrimitive\":0,\"theString\":null}",
		"snapshot:create:any:value",
		"undeploy-all",
	},
	"multikey-table": {
		"deploy:create:@name('create') @public create table MyInfra(value int)",
		"deployed:create",
		"deploy:FafInsert:insert into MyInfra select 0 as value",
		"deploy:Update:on SupportBean update MyInfra set value = (select sum(value) as c0 from SupportEventWithIntArray#keepall group by array)",
		"deployed:Update",
		"send:SupportEventWithIntArray:{\"array\":[1,2],\"id\":\"E1\",\"value\":10}",
		"send:SupportEventWithIntArray:{\"array\":[1,2],\"id\":\"E2\",\"value\":11}",
		"send:SupportBean:{\"intPrimitive\":0,\"theString\":null}",
		"snapshot:create:any:value",
		"send:SupportEventWithIntArray:{\"array\":[1,2],\"id\":\"E3\",\"value\":12}",
		"send:SupportBean:{\"intPrimitive\":0,\"theString\":null}",
		"snapshot:create:any:value",
		"send:SupportEventWithIntArray:{\"array\":[1],\"id\":\"E4\",\"value\":13}",
		"send:SupportBean:{\"intPrimitive\":0,\"theString\":null}",
		"snapshot:create:any:value",
		"undeploy-all",
	},
}

// infraNWTableOnUpdateSnapshotFields extracts the pinned projection list
// for the snapshot at steps[stepIndex] from the pinned step key
// ("snapshot:<statement>:<mode>:<f1,f2,...>"). The case marker occupies
// steps[0], so the pinned index is stepIndex-1.
func infraNWTableOnUpdateSnapshotFields(pinned []string, stepIndex int) []string {
	if stepIndex < 1 || stepIndex-1 >= len(pinned) {
		return nil
	}
	key := pinned[stepIndex-1]
	if !strings.HasPrefix(key, "snapshot:") {
		return nil
	}
	parts := strings.SplitN(key, ":", 4)
	if len(parts) != 4 || parts[3] == "" {
		return nil
	}
	return strings.Split(parts[3], ",")
}

// projectInfraNWTableOnUpdateRows reduces each row to the pinned assertion
// fields: the Java iterator assertions read only these properties even when
// the infra row carries the full SupportBean schema.
func projectInfraNWTableOnUpdateRows(rows []compat.ResultRecord, fields []string) []compat.ResultRecord {
	if len(fields) == 0 {
		return rows
	}
	projected := make([]compat.ResultRecord, len(rows))
	for index, row := range rows {
		narrowed := row
		if row.Fields != nil {
			narrowed.Fields = make(map[string]any, len(fields))
			for _, name := range fields {
				narrowed.Fields[name] = row.Fields[name]
			}
		}
		projected[index] = narrowed
	}
	return projected
}

// infraNWTableOnUpdateInsertAssignments covers every SupportBean property:
// the Java insert-into selects only theString and intPrimitive, but the
// window row materializes the remaining properties with Java defaults, which
// the trigger event carries identically.
func infraNWTableOnUpdateInsertAssignments() []esper.TableAssignment {
	return []esper.TableAssignment{
		esper.SetColumn("theString", esper.Field[infraNWTableOnUpdateBean, string]("theString")),
		esper.SetColumn("boolPrimitive", esper.Field[infraNWTableOnUpdateBean, bool]("boolPrimitive")),
		esper.SetColumn("intPrimitive", esper.Field[infraNWTableOnUpdateBean, int64]("intPrimitive")),
		esper.SetColumn("longPrimitive", esper.Field[infraNWTableOnUpdateBean, int64]("longPrimitive")),
		esper.SetColumn("charPrimitive", esper.Field[infraNWTableOnUpdateBean, string]("charPrimitive")),
		esper.SetColumn("shortPrimitive", esper.Field[infraNWTableOnUpdateBean, int16]("shortPrimitive")),
		esper.SetColumn("bytePrimitive", esper.Field[infraNWTableOnUpdateBean, int8]("bytePrimitive")),
		esper.SetColumn("floatPrimitive", esper.Field[infraNWTableOnUpdateBean, float32]("floatPrimitive")),
		esper.SetColumn("doublePrimitive", esper.Field[infraNWTableOnUpdateBean, float64]("doublePrimitive")),
		esper.SetColumn("boolBoxed", esper.Field[infraNWTableOnUpdateBean, *bool]("boolBoxed")),
		esper.SetColumn("intBoxed", esper.Field[infraNWTableOnUpdateBean, *int64]("intBoxed")),
		esper.SetColumn("longBoxed", esper.Field[infraNWTableOnUpdateBean, *int64]("longBoxed")),
		esper.SetColumn("charBoxed", esper.Field[infraNWTableOnUpdateBean, *string]("charBoxed")),
		esper.SetColumn("shortBoxed", esper.Field[infraNWTableOnUpdateBean, *int16]("shortBoxed")),
		esper.SetColumn("byteBoxed", esper.Field[infraNWTableOnUpdateBean, *int8]("byteBoxed")),
		esper.SetColumn("floatBoxed", esper.Field[infraNWTableOnUpdateBean, *float32]("floatBoxed")),
		esper.SetColumn("doubleBoxed", esper.Field[infraNWTableOnUpdateBean, *float64]("doubleBoxed")),
		esper.SetColumn("bigDecimal", esper.Field[infraNWTableOnUpdateBean, *float64]("bigDecimal")),
		esper.SetColumn("bigInteger", esper.Field[infraNWTableOnUpdateBean, *int64]("bigInteger")),
		esper.SetColumn("enumValue", esper.Field[infraNWTableOnUpdateBean, *string]("enumValue")),
	}
}
