package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/liubaicai/esper/internal/compat"
	"github.com/liubaicai/esper/internal/esper"
)

// infra_table_faf_execute_query.go replays InfraTableFAFExecuteQuery ordinals
// 0-3 against the pinned Java oracle: the fire-and-forget insert/delete/
// update/select surface over tables.
//
// faf-insert (ord 0, InfraFAFInsert) deploys the unkeyed MyTableINS(p0 string,
// p1 int) under statement name 'create', runs the compileExecuteFAF
// `insert into MyTableINS (p0, p1) select 'a', 1` (empty result array, event
// type identical to the create statement's row type) and pins the single
// {a,1} row through an ordered iterator assert.
//
// faf-delete (ord 1, InfraFAFDelete) deploys keyed MyTableDEL(p0 string
// primary key, thesum sum(int)) plus a grouped into-table sum feed, sends
// ten SupportBean("G0",0)..("G9",9) events, pins iterator count 10, runs the
// compileExecuteFAF `delete from MyTableDEL` delete-all and pins iterator
// count 0.
//
// faf-update (ord 2, InfraFAFUpdate) deploys MyTableUPD(p0 string primary
// key, p1 string, thesum sum(int)) under statement name 'TheTable' plus the
// same grouped feed, sends SupportBean("E1",1) and ("E2",2), runs the
// compileExecuteFAF `update MyTableUPD set p1 = 'ABC'` update-all and pins
// {E1,ABC},{E2,ABC} through an any-order iterator assert.
//
// faf-select (ord 3, InfraFAFSelect) deploys MyTableSEL(p0 string primary
// key, thesum sum(int)) under statement name 'TheTable' plus the same
// grouped feed, sends the same two events, runs the compileExecuteFAF
// `select * from MyTableSEL` and pins the result array projected to p0 as
// {E1},{E2} in any order.
//
// Approved differences (observably identical to the Java EPL):
//   - `create table` maps to an env-level registration; the deploy labels
//     carry the into-table plans while deployed markers pin the step labels
//     (the infra-table-join precedent). For ords 2-3 the Java statement name
//     'TheTable' differs from the deploy label 'create'; the snapshot plan
//     is registered under the statement name the iterator assert reads.
//   - FAF writes ride `deploy` steps with Faf* labels (the FafInsert
//     precedent: compileExecuteFAF semantics, no deployment, no marker);
//     the FAF select rides a `snapshot` step carrying the pinned query EPL.
//   - The unkeyed-table FAF insert maps to the positional InsertRows form:
//     `insert into T (p0, p1) select 'a', 1` supplies one value per column
//     in declaration order, the values-clause equivalent.
//   - Java's iteratorCount asserts ride snapshot steps carrying a count pin
//     alongside the row projection; the runner asserts the row count and
//     emits it on the record.
//   - Java's assertSame event-type identity check on the FAF insert result
//     has no observable Go counterpart; the runner asserts the empty result
//     array instead.

// infraTableFAFBean mirrors SupportBean for the grouped into-table feeds.
type infraTableFAFBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const (
	infraTableFAFID          = "infra-table-faf-execute-query"
	infraTableFAFJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraTableFAFJavaSource  = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableFAFExecuteQuery.java"
	infraTableFAFDescription = "InfraTableFAFExecuteQuery ordinals 0-3: InfraFAFInsert deploys the unkeyed MyTableINS and fire-and-forget inserts one ('a',1) row pinned by an ordered iterator assert; InfraFAFDelete feeds keyed MyTableDEL through a grouped into-table sum, pins the ten-row count, fire-and-forget deletes every row and pins the empty count; InfraFAFUpdate feeds MyTableUPD (statement name TheTable) two rows then fire-and-forget updates p1 to 'ABC' pinned by an any-order iterator assert; InfraFAFSelect feeds MyTableSEL two rows then fire-and-forget selects all rows projected to p0 (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableFAFExecuteQuery.java)."

	// Verbatim transcriptions of InfraTableFAFExecuteQuery lines 48, 50 (ord
	// 0); 66-67, 72 (ord 1); 87-88, 91 (ord 2); and 105-106, 109 (ord 3).
	fafqInsCreate = "@name('create') @public create table MyTableINS as (p0 string, p1 int)"
	fafqInsFaf    = "insert into MyTableINS (p0, p1) select 'a', 1"

	fafqDelCreate = "@name('create') @public create table MyTableDEL as (p0 string primary key, thesum sum(int))"
	fafqDelInto   = "into table MyTableDEL select theString, sum(intPrimitive) as thesum from SupportBean group by theString"
	fafqDelFaf    = "delete from MyTableDEL"

	fafqUpdCreate = "@Name('TheTable') @public create table MyTableUPD as (p0 string primary key, p1 string, thesum sum(int))"
	fafqUpdInto   = "into table MyTableUPD select theString, sum(intPrimitive) as thesum from SupportBean group by theString"
	fafqUpdFaf    = "update MyTableUPD set p1 = 'ABC'"

	fafqSelCreate = "@Name('TheTable') @public create table MyTableSEL as (p0 string primary key, thesum sum(int))"
	fafqSelInto   = "into table MyTableSEL select theString, sum(intPrimitive) as thesum from SupportBean group by theString"
	fafqSelFaf    = "select * from MyTableSEL"
)

var (
	infraTableFAFJavaSources = []string{
		infraTableFAFJavaSource,
	}
	infraTableFAFJavaRuntimeIDs = []string{
		"java-runtime-a79e19dc5f135bb8e628",
		"java-runtime-a68109b2bb91de4ce1cd",
		"java-runtime-196ff792f0f739c8d97c",
		"java-runtime-b995c40f3c052bcc277e",
	}
	infraTableFAFJavaExecutions = []string{
		"InfraFAFInsert",
		"InfraFAFDelete",
		"InfraFAFUpdate",
		"InfraFAFSelect",
	}
	infraTableFAFJavaStaticIDs = []string{
		"java-1899404b366f6f3c1a31",
		"java-11e9c7fcb6e1617929a9",
		"java-db32f6072d9b36a84471",
		"java-c87f698c077e7dd7ada7",
	}
	infraTableFAFCases = []string{
		"faf-insert",
		"faf-delete",
		"faf-update",
		"faf-select",
	}
	infraTableFAFOrdinals = []int{0, 1, 2, 3}
)

// infraTableFAFCaseEPLs pins the newline-joined EPL of every EPL-bearing
// step in the case, in step order — the value carried by the scenario
// cases[] metadata.
var infraTableFAFCaseEPLs = []string{
	strings.Join([]string{fafqInsCreate, fafqInsFaf}, "\n"),
	strings.Join([]string{fafqDelCreate, fafqDelInto, fafqDelFaf}, "\n"),
	strings.Join([]string{fafqUpdCreate, fafqUpdInto, fafqUpdFaf}, "\n"),
	strings.Join([]string{fafqSelCreate, fafqSelInto, fafqSelFaf}, "\n"),
}

var infraTableFAFCaseObservations = []string{
	"deploy+snapshot; unkeyed MyTableINS: FAF insert into (p0,p1) select 'a',1 returns an empty array with the create statement's row type and the ordered iterator assert pins the single {a,1} row",
	"deploy+snapshot; keyed MyTableDEL fed by a grouped into-table sum: ten SupportBean sends pin iterator count 10, the FAF delete-all empties the table and the count-0 snapshot pins the empty iterator",
	"deploy+snapshot; MyTableUPD under statement name TheTable fed by the same grouped sum: the FAF update-all sets p1='ABC' and the any-order iterator assert pins {E1,ABC},{E2,ABC}",
	"deploy+snapshot; MyTableSEL fed by the same grouped sum: the FAF select * returns both rows and the any-order assert pins p0 {E1},{E2}",
}

// infraTableFAFCaseState carries the per-case replay state: the
// environment/engine pair, the label→deployments bookkeeping used by
// undeploy-all, the deployed-label set for marker checks, and the snapshot
// plans registered by the create-table deploys.
type infraTableFAFCaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	deployments    map[string][]*esper.Deployment
	deployedLabels map[string]bool
	snapshots      map[string]esper.Plan
	caseName       string
}

// runInfraTableFAFScenario replays the four InfraTableFAFExecuteQuery
// executions: each case runs on a fresh environment/engine pair (one runtime
// per Java execution) and every step dispatches to the matching runtime
// action. The oracle emits the epoch time for every record, so the runner
// pins the same value.
func runInfraTableFAFScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraTableFAFCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraTableFAFCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraTableFAFID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraTableFAFID)
	}
	return trace, nil
}

func runInfraTableFAFCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraTableFAFBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraTableFAFJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &infraTableFAFCaseState{
		env:            env,
		engine:         engine,
		deployments:    make(map[string][]*esper.Deployment),
		deployedLabels: make(map[string]bool),
		snapshots:      make(map[string]esper.Plan),
		caseName:       caseName,
	}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraTableFAFID}
	pinned := infraTableFAFCaseSteps[caseName]
	for stepIndex, step := range steps {
		if err := ctx.Err(); err != nil {
			return compat.Trace{}, err
		}
		switch step.Op {
		case "case":
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return compat.Trace{}, err
			}
		case "deployed":
			if !state.deployedLabels[step.Statement] {
				return compat.Trace{}, fmt.Errorf("%s: deployed marker for unknown statement %q",
					infraTableFAFID, step.Statement)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  1,
				Time:      "1970-01-01T00:00:00Z",
			})
		case "send":
			event, err := decodeInfraTableFAFPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return compat.Trace{}, err
			}
		case "snapshot":
			fields := infraTableFAFSnapshotFields(pinned, stepIndex)
			if err := state.snapshot(ctx, step, fields, &trace); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s: unexpected op %q", infraTableFAFID, step.Op)
		}
	}
	return trace, nil
}

// deploy maps each scenario label to the equivalent Go chain-API plans or
// catalog calls. The Java oracle compiles each deploy step's EPL as one
// module; the Go runner deploys the equivalent plans and registers them
// under the step label so undeploy-all bookkeeping matches. Faf* labels are
// compileExecuteFAF queries: built and executed on demand with no
// deployment and no deployed marker.
func (s *infraTableFAFCaseState) deploy(ctx context.Context, step compat.Step) error {
	switch s.caseName {
	case "faf-insert":
		return s.deployInsert(ctx, step)
	case "faf-delete":
		return s.deployDelete(ctx, step)
	case "faf-update":
		return s.deployUpdate(ctx, step)
	case "faf-select":
		return s.deploySelect(ctx, step)
	default:
		return fmt.Errorf("%s: unsupported case %q", infraTableFAFID, s.caseName)
	}
}

// deployInsert builds the ord-0 fixture: the unkeyed MyTableINS and the
// compileExecuteFAF insert of one ('a',1) row. The insert maps to the
// positional InsertRows form (values in column declaration order) and its
// empty result array is asserted the way assertFAFInsertResult does.
func (s *infraTableFAFCaseState) deployInsert(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "create":
		return s.deployCreate(step.Statement, "create", "MyTableINS", []esper.TableColumn{
			esper.OptionalTableColumnOf[string]("p0"),
			esper.OptionalTableColumnOf[int]("p1"),
		})
	case "FafInsert":
		plan, err := s.env.Build(esper.FromTable(s.env, "MyTableINS").OnDemand().InsertRows(
			esper.InsertValues(esper.Literal("a"), esper.Literal(1)),
		))
		if err != nil {
			return fmt.Errorf("%s: build faf insert %q: %w", infraTableFAFID, step.Statement, err)
		}
		result, err := s.engine.ExecuteFireAndForget(ctx, plan)
		if err != nil {
			return fmt.Errorf("%s: faf insert %q: %w", infraTableFAFID, step.Statement, err)
		}
		if len(result.Results()) != 0 {
			return fmt.Errorf("%s: faf insert %q returned %d rows, want 0",
				infraTableFAFID, step.Statement, len(result.Results()))
		}
		return nil
	default:
		return fmt.Errorf("%s: unknown faf-insert deploy label %q", infraTableFAFID, step.Statement)
	}
}

// deployDelete builds the ord-1 fixture: keyed MyTableDEL, the grouped
// into-table sum feed and the compileExecuteFAF delete-all.
func (s *infraTableFAFCaseState) deployDelete(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "create":
		return s.deployCreate(step.Statement, "create", "MyTableDEL", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("p0"),
			esper.TableColumnOf[int]("thesum"),
		})
	case "into":
		return s.deployInto(ctx, step.Statement, "MyTableDEL")
	case "FafDelete":
		plan, err := s.env.Build(esper.FromTable(s.env, "MyTableDEL").OnDemand().DeleteAll())
		if err != nil {
			return fmt.Errorf("%s: build faf delete %q: %w", infraTableFAFID, step.Statement, err)
		}
		if _, err := s.engine.ExecuteFireAndForget(ctx, plan); err != nil {
			return fmt.Errorf("%s: faf delete %q: %w", infraTableFAFID, step.Statement, err)
		}
		return nil
	default:
		return fmt.Errorf("%s: unknown faf-delete deploy label %q", infraTableFAFID, step.Statement)
	}
}

// deployUpdate builds the ord-2 fixture: keyed MyTableUPD (snapshot label
// TheTable, the Java statement name), the grouped feed and the
// compileExecuteFAF update-all that sets p1='ABC'.
func (s *infraTableFAFCaseState) deployUpdate(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "create":
		return s.deployCreate(step.Statement, "TheTable", "MyTableUPD", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("p0"),
			esper.OptionalTableColumnOf[string]("p1"),
			esper.TableColumnOf[int]("thesum"),
		})
	case "into":
		return s.deployInto(ctx, step.Statement, "MyTableUPD")
	case "FafUpdate":
		plan, err := s.env.Build(esper.FromTable(s.env, "MyTableUPD").OnDemand().UpdateAll(
			esper.SetColumn("p1", esper.Literal("ABC")),
		))
		if err != nil {
			return fmt.Errorf("%s: build faf update %q: %w", infraTableFAFID, step.Statement, err)
		}
		if _, err := s.engine.ExecuteFireAndForget(ctx, plan); err != nil {
			return fmt.Errorf("%s: faf update %q: %w", infraTableFAFID, step.Statement, err)
		}
		return nil
	default:
		return fmt.Errorf("%s: unknown faf-update deploy label %q", infraTableFAFID, step.Statement)
	}
}

// deploySelect builds the ord-3 fixture: keyed MyTableSEL (snapshot label
// TheTable) plus the grouped feed; the FAF select rides a snapshot step.
func (s *infraTableFAFCaseState) deploySelect(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "create":
		return s.deployCreate(step.Statement, "TheTable", "MyTableSEL", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("p0"),
			esper.TableColumnOf[int]("thesum"),
		})
	case "into":
		return s.deployInto(ctx, step.Statement, "MyTableSEL")
	default:
		return fmt.Errorf("%s: unknown faf-select deploy label %q", infraTableFAFID, step.Statement)
	}
}

// deployCreate mirrors a `create table` deploy step: an env-level
// registration. The snapshot plan is registered under snapshotLabel — the
// Java statement name the iterator assert reads ('create' for ords 0-1,
// 'TheTable' for ords 2-3) — while the deploy label itself is marked
// deployed for the marker check.
func (s *infraTableFAFCaseState) deployCreate(stepLabel, snapshotLabel, table string,
	columns []esper.TableColumn) error {
	if _, err := esper.CreateTable(s.env, table, columns); err != nil {
		return err
	}
	if err := s.registerSnapshot(snapshotLabel, table); err != nil {
		return err
	}
	s.deployedLabels[stepLabel] = true
	return nil
}

// deployInto mirrors the grouped into-table sum feed shared by ords 1-3:
// `into table T select theString, sum(intPrimitive) as thesum from
// SupportBean group by theString` — the group key maps positionally to
// primary-key column p0.
func (s *infraTableFAFCaseState) deployInto(ctx context.Context, label, table string) error {
	theString := esper.Field[infraTableFAFBean, string]("theString")
	intPrimitive := esper.Field[infraTableFAFBean, int]("intPrimitive")
	plan, err := s.env.Build(esper.From[infraTableFAFBean](s.env, "SupportBean").
		GroupBy(theString).
		Select(
			esper.Alias("p0", theString),
			esper.Alias("thesum", esper.Sum[int](intPrimitive)),
		).IntoTable(table))
	if err != nil {
		return err
	}
	return s.deployPlan(ctx, label, plan)
}

// registerSnapshot prepares the fire-and-forget table query behind the
// create-table deploy label so snapshot steps read the table rows the way
// the Java iterator on the named create statement does.
func (s *infraTableFAFCaseState) registerSnapshot(label, table string) error {
	plan, err := s.env.Build(esper.FromTable(s.env, table).Query(
		esper.StatementName("snapshot-" + table)))
	if err != nil {
		return err
	}
	s.snapshots[label] = plan
	s.deployedLabels[label] = true
	return nil
}

func (s *infraTableFAFCaseState) deployPlan(ctx context.Context, label string,
	plan esper.Plan) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[label] = append(s.deployments[label], deployment)
	s.deployedLabels[label] = true
	return nil
}

// snapshot executes the pinned read: a Faf* label builds and runs the
// fire-and-forget select carried by the step's epl field (compileExecuteFAF
// semantics), while a create-statement label runs the registered table
// query. Rows are projected to the pinned fields and sorted for mode "any";
// a count pin asserts the row count the way Java's iteratorCount does.
func (s *infraTableFAFCaseState) snapshot(ctx context.Context, step compat.Step,
	fields []string, trace *compat.Trace) error {
	var result esper.QueryResult
	if step.Epl != "" {
		plan, err := s.buildFafSelect(step)
		if err != nil {
			return err
		}
		executed, err := s.engine.ExecuteFireAndForget(ctx, plan)
		if err != nil {
			return err
		}
		result = executed
	} else {
		plan, ok := s.snapshots[step.Statement]
		if !ok {
			return fmt.Errorf("%s: unknown snapshot %q", infraTableFAFID, step.Statement)
		}
		executed, err := s.engine.ExecuteFireAndForget(ctx, plan)
		if err != nil {
			return err
		}
		result = executed
	}
	rows := infraTableJoinNormalizeResults(result.Results())
	rows = projectInfraNWTableOnMergeRows(rows, fields)
	if step.Mode == "any" {
		sortRowsCanonical(rows)
	}
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "snapshot",
		Statement: step.Statement,
		Sequence:  0,
		Time:      "1970-01-01T00:00:00Z",
	}
	if len(rows) > 0 {
		record.New = rows
	}
	if step.Count != nil {
		if int64(len(rows)) != *step.Count {
			return fmt.Errorf("%s: snapshot %q count drift: got %d want %d",
				infraTableFAFID, step.Statement, len(rows), *step.Count)
		}
		pinned := *step.Count
		record.Count = &pinned
	}
	trace.Records = append(trace.Records, record)
	return nil
}

// buildFafSelect maps the pinned FAF select EPL to the typed plan:
// `select * from MyTableSEL` is the unfiltered all-column table query; the
// snapshot projection narrows the rows to the pinned p0 field.
func (s *infraTableFAFCaseState) buildFafSelect(step compat.Step) (esper.Plan, error) {
	if step.Statement != "FafSelect" || step.Epl != fafqSelFaf {
		return esper.Plan{}, fmt.Errorf("%s: unknown faf select %q", infraTableFAFID, step.Statement)
	}
	return s.env.Build(esper.FromTable(s.env, "MyTableSEL").Query())
}

func (s *infraTableFAFCaseState) undeployAll(ctx context.Context) error {
	for _, deployments := range s.deployments {
		for _, deployment := range deployments {
			if err := deployment.Undeploy(ctx); err != nil {
				return err
			}
		}
	}
	s.deployments = make(map[string][]*esper.Deployment)
	s.deployedLabels = make(map[string]bool)
	s.snapshots = make(map[string]esper.Plan)
	return nil
}

func decodeInfraTableFAFPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value infraTableFAFBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported infra table faf-execute-query event type %q", step.EventType)
	}
}

// loadInfraTableFAFScenario enforces the strict scenario contract shared by
// the differential runners: no duplicate or unknown JSON fields, pinned
// metadata, pinned per-case runtime/execution/EPL, and a per-op step field
// whitelist followed by a full step-shape pin.
func loadInfraTableFAFScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraTableFAFID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraTableFAFID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableFAFID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableFAFID, err)
	}
	if err := requireInfraTableFAFFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraTableFAFID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraTableFAFID ||
		metadata.Description != infraTableFAFDescription ||
		metadata.JavaCommit != infraTableFAFJavaCommit ||
		metadata.JavaSource != infraTableFAFJavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraTableFAFID)
	}
	if err := validateInfraTableFAFStringArray(root["javaRuntimes"], infraTableFAFJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableFAFStringArray(root["javaNames"], infraTableFAFJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableFAFStringArray(root["javaStaticIds"], infraTableFAFJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableFAFStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraTableFAFCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraTableFAFID, len(infraTableFAFCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraTableFAFFields(object,
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
		if definition.Case != infraTableFAFCases[index] ||
			definition.Ordinal != infraTableFAFOrdinals[index] ||
			definition.RuntimeID != infraTableFAFJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraTableFAFJavaExecutions[index] ||
			definition.Observation != infraTableFAFCaseObservations[index] ||
			definition.EPL != infraTableFAFCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraTableFAFID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraTableFAFID)
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
			if err := requireInfraTableFAFFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraTableFAFFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraTableFAFFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraTableFAFFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraTableFAFPayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			// Statement snapshots carry op/case/statement/mode/fields; FAF
			// selects additionally carry the pinned query epl and iterator
			// counts the count pin.
			names := []string{"op", "case", "statement", "mode", "fields"}
			if _, hasEpl := object["epl"]; hasEpl {
				names = append(names, "epl")
			}
			if _, hasCount := object["count"]; hasCount {
				names = append(names, "count")
			}
			if err := requireInfraTableFAFFields(object, names...); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraTableFAFFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		var stepCase string
		if err := json.Unmarshal(object["case"], &stepCase); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d case: %w", index, err)
		}
		found := false
		for _, name := range infraTableFAFCases {
			if stepCase == name {
				found = true
				break
			}
		}
		if !found {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unknown case %q", index, stepCase)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateInfraTableFAFRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraTableFAFRawSteps pins the complete step sequence per case
// against the raw JSON objects: deploy steps with byte-exact EPL, deployed
// markers, send event types with canonical payloads, snapshot reads with
// optional epl/count pins, and the undeploy-all terminators.
func validateInfraTableFAFRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraTableFAFCases {
		want, ok := infraTableFAFCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraTableFAFID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraTableFAFID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraTableFAFID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraTableFAFID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraTableFAFStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraTableFAFID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q",
					infraTableFAFID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraTableFAFID)
	}
	return nil
}

// infraTableFAFStepKey renders a raw step object into its pinned string
// form. Fields are read from the raw JSON because compat.Step does not
// carry the fields array.
func infraTableFAFStepKey(object map[string]json.RawMessage, operation string) (string, error) {
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
		return operation + ":" + statement, nil
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
		key := "snapshot:" + statement + ":" + mode + ":" + fields
		if countValue, hasCount := object["count"]; hasCount {
			var count int64
			if err := json.Unmarshal(countValue, &count); err != nil {
				return "", fmt.Errorf("step field %q must be an integer", "count")
			}
			key += fmt.Sprintf(":count=%d", count)
		}
		if eplValue, hasEpl := object["epl"]; hasEpl {
			var epl string
			if err := json.Unmarshal(eplValue, &epl); err != nil {
				return "", fmt.Errorf("step field %q must be a string", "epl")
			}
			key += ":" + epl
		}
		return key, nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// fafqInsertCaseSteps renders the pinned step sequence of
// InfraFAFInsert.run (lines 45-56): the create-table deploy, the
// compileExecuteFAF insert, the ordered iterator assert and undeployAll.
func fafqInsertCaseSteps() []string {
	return []string{
		"deploy:create:" + fafqInsCreate,
		"deployed:create",
		"deploy:FafInsert:" + fafqInsFaf,
		"snapshot:create:ordered:p0,p1",
		"undeploy-all",
	}
}

// fafqDeleteCaseSteps renders the pinned step sequence of
// InfraFAFDelete.run (lines 65-75): the create-table and into-table deploys,
// the ten SupportBean sends, the count-10 iterator pin, the
// compileExecuteFAF delete-all, the count-0 iterator pin and undeployAll.
func fafqDeleteCaseSteps() []string {
	steps := []string{
		"deploy:create:" + fafqDelCreate,
		"deployed:create",
		"deploy:into:" + fafqDelInto,
		"deployed:into",
	}
	for index := range 10 {
		steps = append(steps, fmt.Sprintf(
			`send:SupportBean:{"intPrimitive":%d,"theString":"G%d"}`, index, index))
	}
	return append(steps,
		"snapshot:create:any:p0,thesum:count=10",
		"deploy:FafDelete:"+fafqDelFaf,
		"snapshot:create:any:p0,thesum:count=0",
		"undeploy-all")
}

// fafqUpdateCaseSteps renders the pinned step sequence of
// InfraFAFUpdate.run (lines 85-93): the create-table deploy under statement
// name TheTable, the into-table deploy, the two SupportBean sends, the
// compileExecuteFAF update-all and the any-order iterator assert on
// TheTable.
func fafqUpdateCaseSteps() []string {
	return []string{
		"deploy:create:" + fafqUpdCreate,
		"deployed:create",
		"deploy:into:" + fafqUpdInto,
		"deployed:into",
		`send:SupportBean:{"intPrimitive":1,"theString":"E1"}`,
		`send:SupportBean:{"intPrimitive":2,"theString":"E2"}`,
		"deploy:FafUpdate:" + fafqUpdFaf,
		"snapshot:TheTable:any:p0,p1",
		"undeploy-all",
	}
}

// fafqSelectCaseSteps renders the pinned step sequence of
// InfraFAFSelect.run (lines 103-111): the create-table deploy under
// statement name TheTable, the into-table deploy, the two SupportBean sends
// and the compileExecuteFAF select-all carried as a snapshot step with the
// pinned query EPL.
func fafqSelectCaseSteps() []string {
	return []string{
		"deploy:create:" + fafqSelCreate,
		"deployed:create",
		"deploy:into:" + fafqSelInto,
		"deployed:into",
		`send:SupportBean:{"intPrimitive":1,"theString":"E1"}`,
		`send:SupportBean:{"intPrimitive":2,"theString":"E2"}`,
		"snapshot:FafSelect:any:p0:" + fafqSelFaf,
		"undeploy-all",
	}
}

// infraTableFAFCaseSteps pins the exact op sequence per case.
var infraTableFAFCaseSteps = map[string][]string{
	"faf-insert": fafqInsertCaseSteps(),
	"faf-delete": fafqDeleteCaseSteps(),
	"faf-update": fafqUpdateCaseSteps(),
	"faf-select": fafqSelectCaseSteps(),
}

// infraTableFAFSnapshotFields extracts the pinned projection list for the
// snapshot at steps[stepIndex] from the pinned step key
// ("snapshot:<statement>:<mode>:<f1,f2,...>" with an optional :count=n or
// :epl tail). The case marker occupies steps[0], so the pinned index is
// stepIndex-1.
func infraTableFAFSnapshotFields(pinned []string, stepIndex int) []string {
	if stepIndex < 1 || stepIndex-1 >= len(pinned) {
		return nil
	}
	key := pinned[stepIndex-1]
	if !strings.HasPrefix(key, "snapshot:") {
		return nil
	}
	parts := strings.SplitN(key, ":", 5)
	if len(parts) < 4 || parts[3] == "" {
		return nil
	}
	return strings.Split(parts[3], ",")
}

func requireInfraTableFAFFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraTableFAFID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraTableFAFID, name)
		}
	}
	return nil
}

func validateInfraTableFAFStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}

// infraTableFAFRuntimeID maps each scenario case to the inventory runtime
// ID of the Java execution it replays.
func infraTableFAFRuntimeID(caseName string) string {
	for index, name := range infraTableFAFCases {
		if name == caseName {
			return infraTableFAFJavaRuntimeIDs[index]
		}
	}
	return ""
}
