package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/liubaicai/esper/internal/compat"
	"github.com/liubaicai/esper/internal/esper"
)

// infra_table_update_and_index.go replays InfraTableUpdateAndIndex ordinals
// 0-4 against the pinned Java oracle: the table update and unique-index
// surface.
//
// early-unique-violation (ord 0, InfraEarlyUniqueIndexViolation) deploys a
// two-primary-key MyTableEUIV(pkey0 string, pkey1 int, thecnt count(*)) plus
// a grouped into-table count feed, sends SupportBean("E1",10) and
// ("E1",20) so pkey0 collides, then exercises four failure phases: a late
// `create unique index SecIndex on MyTableEUIV(pkey0)` that compiles but
// deploy-fails with the unique-violation message; a fire-and-forget
// `update MyTableEUIV set pkey1 = 0` that fails atomically and leaves the
// table unchanged; an on-update trigger whose SupportBean_S1 send fails
// through the rethrowing exception handler; and an on-merge compile that is
// rejected because when-matched updates may not touch unique keys.
//
// late-unique-violation (ord 1, InfraLateUniqueIndexViolation) deploys
// MyTableLUIV(pkey0, pkey1, col0, thecnt) with distinct pkey0 rows, deploys
// an on-merge that updates col0, then proves `create unique index
// MyUniqueSecondary on MyTableLUIV (col0)` deploy-fails because col0 is
// merge-updated. After undeployModuleContaining("on-merge") an on-update of
// pkey1 plus a unique index on pkey1 both deploy, and the SupportBean_S1
// send fails with the unique-violation message naming MyUniqueSecondary.
//
// faf-update (ord 2, InfraFAFUpdate) deploys MyTableFAFU(pkey0, col0, col1,
// thecnt) with a non-unique secondary hash index MyIndex on col0 and a
// grouped into-table feed keyed by theString, then runs the
// compileExecuteFAF sequence: col0=1 for E1, col0=2 for E2, a one-row
// select on col0=1 through MyIndex, col1=100 for E1 and a one-row select on
// col1=100.
//
// key-update-single (ord 3, InfraTableKeyUpdateSingleKey) deploys
// MyTableSingleKey(pkey0, c0), an insert-into feed and an on-SupportBean_S0
// update that renames pkey0; three SupportBean rows then three S0 renames
// interleaved with iterator any-order assertions (milestone checkpoints are
// harness no-ops and carry no steps).
//
// key-update-multi (ord 4, InfraTableKeyUpdateMultiKey) repeats ord 3
// against MyTableMultiKey(pkey0, pkey1, c0 long) where only pkey0 of the
// composite primary key is renamed.
//
// Approved differences (observably identical to the Java EPL):
//   - `create table`/`create index` map to env-level registrations and
//     Table.CreateIndex catalog calls; the deploy labels carry the
//     into-table/insert-into/on-update/on-merge plans while deployed
//     markers pin the step labels (the infra-table-join precedent).
//   - Java's milestone(n) checkpoints are harness no-ops and carry no
//     steps.
//   - Java's `insert into T select ... from S` continuous inserts map to
//     OnEvent(...).InsertIntoTable with the same SetColumn assignments.
//   - FAF updates ride `deploy` steps with Faf* labels (the FafInsert
//     precedent: compileExecuteFAF semantics, no deployment, no marker);
//     FAF selects ride `snapshot` steps carrying the pinned query EPL.
//   - Error steps emit the pinned expectError text as the record value
//     after the runner requires the real failure: deploy-error requires an
//     *esper.Error from Table.CreateIndex, faf-error/send-error require an
//     *esper.Error from ExecuteFireAndForget/Send, and build-error
//     requires ErrorInvalidRule from env.Build (the undeploy-error/
//     build-error precedent — Go wording does not need to match Java).

// infraTableUpdateBean mirrors SupportBean for the into-table/insert feeds.
type infraTableUpdateBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// infraTableUpdateS0 mirrors SupportBean_S0 for the ord 3-4 rename stream.
type infraTableUpdateS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

// infraTableUpdateS1 mirrors SupportBean_S1 for the ord 0-1 on-update and
// on-merge triggers; p10 supplies the first merge key expression.
type infraTableUpdateS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

const (
	infraTableUpdateIndexID          = "infra-table-update-and-index"
	infraTableUpdateIndexJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraTableUpdateIndexJavaSource  = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableUpdateAndIndex.java"
	infraTableUpdateIndexDescription = "InfraTableUpdateAndIndex ordinals 0-4: InfraEarlyUniqueIndexViolation deploys a two-primary-key MyTableEUIV fed by a grouped into-table count, then pins four failure phases — a deploy-time unique-index violation on pkey0, an atomic fire-and-forget update failure on pkey1, an on-update send failure surfaced through the exception handler, and a compile-rejected on-merge unique-key update; InfraLateUniqueIndexViolation pins the merge-updated-column create-index rejection on col0, the undeployModuleContaining boundary, and a late unique index on pkey1 whose on-update send violates it; InfraFAFUpdate runs the fire-and-forget update/select sequence over MyTableFAFU exercising the MyIndex secondary hash index; InfraTableKeyUpdateSingleKey and InfraTableKeyUpdateMultiKey rename the single and composite primary keys through on-SupportBean_S0 updates with iterator any-order assertions. Java milestone checkpoints are harness no-ops and carry no steps (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableUpdateAndIndex.java)."

	// Verbatim transcriptions of InfraTableUpdateAndIndex lines 49-50, 56,
	// 65, 74, 86 (ord 0); 103-109, 114, 117, 127-128 (ord 1); 151-154,
	// 158-163 (ord 2); 213-215 (ord 3); and 177-179 (ord 4).
	tuixEarlyCreate   = "@name('create') @public create table MyTableEUIV as (pkey0 string primary key, pkey1 int primary key, thecnt count(*))"
	tuixEarlyInto     = "into table MyTableEUIV select count(*) as thecnt from SupportBean group by theString, intPrimitive"
	tuixEarlySecIndex = "create unique index SecIndex on MyTableEUIV(pkey0)"
	tuixEarlyFafUpd   = "update MyTableEUIV set pkey1 = 0"
	tuixEarlyOnUpdate = "@name('on-update') on SupportBean_S1 update MyTableEUIV set pkey1 = 0"
	tuixEarlyOnMerge  = "@name('on-merge') on SupportBean_S1 merge MyTableEUIV when matched then update set pkey1 = 0"

	tuixLateCreate        = "@name('create') @public create table MyTableLUIV as (pkey0 string primary key, pkey1 int primary key, col0 int, thecnt count(*))"
	tuixLateInto          = "into table MyTableLUIV select count(*) as thecnt from SupportBean group by theString, intPrimitive"
	tuixLateOnMerge       = "@name('on-merge') on SupportBean_S1 merge MyTableLUIV when matched then update set col0 = 0"
	tuixLateSecIndexCol0  = "create unique index MyUniqueSecondary on MyTableLUIV (col0)"
	tuixLateOnUpdate      = "@name('on-update') on SupportBean_S1 update MyTableLUIV set pkey1 = 0"
	tuixLateSecIndexPkey1 = "create unique index MyUniqueSecondary on MyTableLUIV (pkey1)"

	tuixFafCreate       = "@public create table MyTableFAFU as (pkey0 string primary key, col0 int, col1 int, thecnt count(*))"
	tuixFafIndex        = "create index MyIndex on MyTableFAFU(col0)"
	tuixFafInto         = "into table MyTableFAFU select count(*) as thecnt from SupportBean group by theString"
	tuixFafUpdateCol0E1 = "update MyTableFAFU set col0 = 1 where pkey0='E1'"
	tuixFafUpdateCol0E2 = "update MyTableFAFU set col0 = 2 where pkey0='E2'"
	tuixFafSelectCol0   = "select pkey0 from MyTableFAFU where col0=1"
	tuixFafUpdateCol1E1 = "update MyTableFAFU set col1 = 100 where pkey0='E1'"
	tuixFafSelectCol1   = "select pkey0 from MyTableFAFU where col1=100"

	tuixSingleCreate   = "@name('s0') @public create table MyTableSingleKey(pkey0 string primary key, c0 int)"
	tuixSingleInsert   = "insert into MyTableSingleKey select theString as pkey0, intPrimitive as c0 from SupportBean"
	tuixSingleOnUpdate = "on SupportBean_S0 update MyTableSingleKey set pkey0 = p01 where pkey0 = p00"

	tuixMultiCreate   = "@name('s1') @public create table MyTableMultiKey(pkey0 string primary key, pkey1 int primary key, c0 long)"
	tuixMultiInsert   = "insert into MyTableMultiKey select theString as pkey0, intPrimitive as pkey1, longPrimitive as c0 from SupportBean"
	tuixMultiOnUpdate = "on SupportBean_S0 update MyTableMultiKey set pkey0 = p01 where pkey0 = p00"

	// Pinned Java message prefixes asserted by SupportMessageAssertUtil
	// (startsWith semantics); error steps emit this text as the record
	// value after the real failure fires.
	tuixErrEarlySecIndex = "Failed to deploy: Unique index violation, index 'SecIndex' is a unique index and key 'E1' already exists"
	tuixErrEarlyFafUpd   = "Unique index violation, index 'MyTableEUIV' is a unique index and key 'MultiKey[E1,0]' already exists"
	tuixErrEarlyOnUpdate = "Unexpected exception in statement 'on-update': Unique index violation, index 'MyTableEUIV' is a unique index and key 'MultiKey[E1,0]' already exists"
	tuixErrOnMergeUnique = "Validation failed in when-matched (clause 1): On-merge statements may not update unique keys of tables"
	tuixErrLateSecIndex  = "Failed to deploy: Create-index adds a unique key on columns that are updated by one or more on-merge statements"
	tuixErrLateOnUpdate  = "Unexpected exception in statement 'on-update': Unique index violation, index 'MyUniqueSecondary' is a unique index and key '0' already exists"
)

var (
	infraTableUpdateIndexJavaSources = []string{
		infraTableUpdateIndexJavaSource,
	}
	infraTableUpdateIndexJavaRuntimeIDs = []string{
		"java-runtime-878326b2aef272d9ef78",
		"java-runtime-59f3f0884fc9ae9d749f",
		"java-runtime-1118b36d6f38fa1c78a8",
		"java-runtime-16e0f7011601678fa5df",
		"java-runtime-0b3580bcd42327b7d2bb",
	}
	infraTableUpdateIndexJavaExecutions = []string{
		"InfraEarlyUniqueIndexViolation",
		"InfraLateUniqueIndexViolation",
		"InfraFAFUpdate",
		"InfraTableKeyUpdateSingleKey",
		"InfraTableKeyUpdateMultiKey",
	}
	infraTableUpdateIndexJavaStaticIDs = []string{
		"java-c8309a6f377c34ffcdb4",
		"java-8db9b9cd27d04bb86d69",
		"java-4a2462a159e9d648257a",
		"java-ad9c04a3fc0d98d4e4dd",
		"java-97fc9971820f1e5b947d",
	}
	infraTableUpdateIndexCases = []string{
		"early-unique-violation",
		"late-unique-violation",
		"faf-update",
		"key-update-single",
		"key-update-multi",
	}
	infraTableUpdateIndexOrdinals = []int{0, 1, 2, 3, 4}
)

// infraTableUpdateIndexCaseEPLs pins the newline-joined EPL of every
// EPL-bearing step in the case, in step order — the value carried by the
// scenario cases[] metadata.
var infraTableUpdateIndexCaseEPLs = []string{
	strings.Join([]string{tuixEarlyCreate, tuixEarlyInto, tuixEarlySecIndex,
		tuixEarlyFafUpd, tuixEarlyOnUpdate, tuixEarlyOnMerge}, "\n"),
	strings.Join([]string{tuixLateCreate, tuixLateInto, tuixLateOnMerge,
		tuixLateSecIndexCol0, tuixLateOnUpdate, tuixLateSecIndexPkey1}, "\n"),
	strings.Join([]string{tuixFafCreate, tuixFafIndex, tuixFafInto,
		tuixFafUpdateCol0E1, tuixFafUpdateCol0E2, tuixFafSelectCol0,
		tuixFafUpdateCol1E1, tuixFafSelectCol1}, "\n"),
	strings.Join([]string{tuixSingleCreate, tuixSingleInsert, tuixSingleOnUpdate}, "\n"),
	strings.Join([]string{tuixMultiCreate, tuixMultiInsert, tuixMultiOnUpdate}, "\n"),
}

var infraTableUpdateIndexCaseObservations = []string{
	"deploy-error+faf-error+send-error+build-error; two-row MyTableEUIV colliding on pkey0: late unique index SecIndex deploy-fails on key 'E1', FAF update of pkey1 to 0 fails atomically on MultiKey[E1,0], the on-update S1 send fails through the exception handler, and the on-merge compile is rejected for updating a unique key",
	"deploy-error+send-error; MyTableLUIV with distinct pkey0 rows: unique index on merge-updated col0 deploy-fails, undeployModuleContaining('on-merge') clears the block, on-update plus unique index MyUniqueSecondary on pkey1 deploy and the S1 send violates key '0'",
	"faf; MyTableFAFU with non-unique MyIndex on col0: FAF updates set col0=1/col0=2/col1=100 and the col0=1/col1=100 FAF selects each return the single pkey0 E1 row",
	"snapshot; MyTableSingleKey(pkey0,c0) fed by insert-into; three SupportBean_S0 renames re-key E2->E20, E1->E10, E3->E30 with c0 preserved across iterator any-order asserts",
	"snapshot; MyTableMultiKey(pkey0,pkey1,c0) fed by insert-into; the same three renames update only pkey0 of the composite key while pkey1 and c0 stay pinned",
}

// infraTableUpdateIndexCaseState carries the per-case replay state: the
// environment/engine pair, the label→deployments bookkeeping used by the
// undeploy-module handler, the deployed-label set for marker checks, and
// the snapshot plans registered by the create-table deploys.
type infraTableUpdateIndexCaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	deployments    map[string][]*esper.Deployment
	deployedLabels map[string]bool
	snapshots      map[string]esper.Plan
	caseName       string
}

// runInfraTableUpdateIndexScenario replays the five InfraTableUpdateAndIndex
// executions: each case runs on a fresh environment/engine pair (one runtime
// per Java execution) and every step dispatches to the matching runtime
// action. The oracle emits the epoch time for every record, so the runner
// pins the same value.
func runInfraTableUpdateIndexScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraTableUpdateIndexCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraTableUpdateIndexCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraTableUpdateIndexID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraTableUpdateIndexID)
	}
	return trace, nil
}

func runInfraTableUpdateIndexCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraTableUpdateBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraTableUpdateS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraTableUpdateS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraTableUpdateIndexJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &infraTableUpdateIndexCaseState{
		env:            env,
		engine:         engine,
		deployments:    make(map[string][]*esper.Deployment),
		deployedLabels: make(map[string]bool),
		snapshots:      make(map[string]esper.Plan),
		caseName:       caseName,
	}
	sequences := make(map[string]uint64)
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraTableUpdateIndexID}
	pinned := infraTableUpdateIndexCaseSteps[caseName]
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
					infraTableUpdateIndexID, step.Statement)
			}
			sequences[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequences[step.Statement+":deployed"],
				Time:      "1970-01-01T00:00:00Z",
			})
		case "send":
			event, err := decodeInfraTableUpdateIndexPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return compat.Trace{}, err
			}
		case "send-error":
			if err := state.sendError(ctx, step, &trace, sequences); err != nil {
				return compat.Trace{}, err
			}
		case "snapshot":
			fields := infraTableUpdateIndexSnapshotFields(pinned, stepIndex)
			if err := state.snapshot(ctx, step, fields, &trace); err != nil {
				return compat.Trace{}, err
			}
		case "deploy-error":
			if err := state.deployError(step, &trace, sequences); err != nil {
				return compat.Trace{}, err
			}
		case "build-error":
			if err := state.buildError(step, &trace, sequences); err != nil {
				return compat.Trace{}, err
			}
		case "faf-error":
			if err := state.fafError(ctx, step, &trace, sequences); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy":
			// undeployModuleContaining: the step label resolves the
			// deployment that carries the named statement.
			deployments, ok := state.deployments[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("%s: undeploy of unknown deployment %q",
					infraTableUpdateIndexID, step.Statement)
			}
			for _, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, err
				}
			}
			delete(state.deployments, step.Statement)
			delete(state.deployedLabels, step.Statement)
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s: unexpected op %q", infraTableUpdateIndexID, step.Op)
		}
	}
	return trace, nil
}

// deploy maps each scenario label to the equivalent Go chain-API plans or
// catalog calls. The Java oracle compiles each deploy step's EPL as one
// module; the Go runner deploys the equivalent plans and registers them
// under the step label so undeploy-module/undeploy-all bookkeeping matches.
// Faf* labels are compileExecuteFAF queries: built and executed on demand
// with no deployment and no deployed marker.
func (s *infraTableUpdateIndexCaseState) deploy(ctx context.Context, step compat.Step) error {
	switch s.caseName {
	case "early-unique-violation":
		return s.deployEarly(ctx, step.Statement)
	case "late-unique-violation":
		return s.deployLate(ctx, step.Statement)
	case "faf-update":
		return s.deployFaf(ctx, step)
	case "key-update-single":
		return s.deployKeyUpdate(ctx, step.Statement, "MyTableSingleKey", "s0", false)
	case "key-update-multi":
		return s.deployKeyUpdate(ctx, step.Statement, "MyTableMultiKey", "s1", true)
	default:
		return fmt.Errorf("%s: unsupported case %q", infraTableUpdateIndexID, s.caseName)
	}
}

// deployEarly builds the ord-0 fixture: the two-primary-key table, the
// grouped into-table count feed and the on-update trigger that reassigns
// pkey1 unconditionally.
func (s *infraTableUpdateIndexCaseState) deployEarly(ctx context.Context, label string) error {
	switch label {
	case "create":
		if _, err := esper.CreateTable(s.env, "MyTableEUIV", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("pkey0"),
			esper.PrimaryKeyColumn[int]("pkey1"),
			esper.TableColumnOf[int64]("thecnt"),
		}); err != nil {
			return err
		}
		return s.registerSnapshot(label, "MyTableEUIV")
	case "into":
		theString := esper.Field[infraTableUpdateBean, string]("theString")
		intPrimitive := esper.Field[infraTableUpdateBean, int]("intPrimitive")
		plan, err := s.env.Build(esper.From[infraTableUpdateBean](s.env, "SupportBean").
			GroupBy(theString, intPrimitive).
			Select(
				esper.Alias("pkey0", theString),
				esper.Alias("pkey1", intPrimitive),
				esper.Alias("thecnt", esper.CountAll()),
			).IntoTable("MyTableEUIV"))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	case "on-update":
		plan, err := s.env.Build(esper.OnEvent(esper.From[infraTableUpdateS1](s.env, "SupportBean_S1")).
			UpdateTableWhere("MyTableEUIV", esper.Literal(true),
				esper.SetColumn("pkey1", esper.Literal(0)),
			).Query(esper.StatementName("on-update")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	default:
		return fmt.Errorf("%s: unknown early-unique-violation deploy label %q",
			infraTableUpdateIndexID, label)
	}
}

// deployLate builds the ord-1 fixture: the keyed table with the merge-updated
// col0 column, the grouped into-table feed, the on-merge trigger (deploys
// successfully because col0 is not yet unique-indexed), the on-update
// trigger and the late unique index on pkey1.
func (s *infraTableUpdateIndexCaseState) deployLate(ctx context.Context, label string) error {
	switch label {
	case "create":
		if _, err := esper.CreateTable(s.env, "MyTableLUIV", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("pkey0"),
			esper.PrimaryKeyColumn[int]("pkey1"),
			esper.OptionalTableColumnOf[int]("col0"),
			esper.TableColumnOf[int64]("thecnt"),
		}); err != nil {
			return err
		}
		return s.registerSnapshot(label, "MyTableLUIV")
	case "into":
		theString := esper.Field[infraTableUpdateBean, string]("theString")
		intPrimitive := esper.Field[infraTableUpdateBean, int]("intPrimitive")
		plan, err := s.env.Build(esper.From[infraTableUpdateBean](s.env, "SupportBean").
			GroupBy(theString, intPrimitive).
			Select(
				esper.Alias("pkey0", theString),
				esper.Alias("pkey1", intPrimitive),
				esper.Alias("thecnt", esper.CountAll()),
			).IntoTable("MyTableLUIV"))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	case "on-merge":
		// The merge keys are evaluated against the trigger event; the
		// execution never sends SupportBean_S1 while this statement is
		// deployed, so only the deploy-time merge-updated-column
		// registration is observable.
		plan, err := s.env.Build(esper.OnEvent(esper.From[infraTableUpdateS1](s.env, "SupportBean_S1")).
			MergeIntoTableWhen("MyTableLUIV",
				[]esper.Expr{
					esper.Field[infraTableUpdateS1, string]("p10"),
					esper.Field[infraTableUpdateS1, int]("id"),
				},
				esper.WhenMatchedAny(esper.SetColumn("col0", esper.Literal(0))),
			).Query(esper.StatementName("on-merge")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	case "on-update":
		plan, err := s.env.Build(esper.OnEvent(esper.From[infraTableUpdateS1](s.env, "SupportBean_S1")).
			UpdateTableWhere("MyTableLUIV", esper.Literal(true),
				esper.SetColumn("pkey1", esper.Literal(0)),
			).Query(esper.StatementName("on-update")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	case "sec-index-pkey1":
		return s.createIndex(label, "MyTableLUIV", "MyUniqueSecondary", []string{"pkey1"}, true)
	default:
		return fmt.Errorf("%s: unknown late-unique-violation deploy label %q",
			infraTableUpdateIndexID, label)
	}
}

// deployFaf builds the ord-2 fixture: the keyed table with the non-unique
// MyIndex hash index on col0, the grouped into-table feed keyed by
// theString, and the three compileExecuteFAF updates.
func (s *infraTableUpdateIndexCaseState) deployFaf(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "create":
		if _, err := esper.CreateTable(s.env, "MyTableFAFU", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("pkey0"),
			esper.OptionalTableColumnOf[int]("col0"),
			esper.OptionalTableColumnOf[int]("col1"),
			esper.TableColumnOf[int64]("thecnt"),
		}); err != nil {
			return err
		}
		return s.registerSnapshot(step.Statement, "MyTableFAFU")
	case "create-index":
		return s.createIndex(step.Statement, "MyTableFAFU", "MyIndex", []string{"col0"}, false)
	case "into":
		theString := esper.Field[infraTableUpdateBean, string]("theString")
		plan, err := s.env.Build(esper.From[infraTableUpdateBean](s.env, "SupportBean").
			GroupBy(theString).
			Select(
				esper.Alias("pkey0", theString),
				esper.Alias("thecnt", esper.CountAll()),
			).IntoTable("MyTableFAFU"))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, step.Statement, plan)
	case "FafUpdateCol0E1":
		return s.fafUpdate(ctx, step, "col0", 1, "E1")
	case "FafUpdateCol0E2":
		return s.fafUpdate(ctx, step, "col0", 2, "E2")
	case "FafUpdateCol1E1":
		return s.fafUpdate(ctx, step, "col1", 100, "E1")
	default:
		return fmt.Errorf("%s: unknown faf-update deploy label %q",
			infraTableUpdateIndexID, step.Statement)
	}
}

// deployKeyUpdate builds the ord-3/4 fixture: the keyed table, the
// insert-into feed and the on-SupportBean_S0 update that renames pkey0 to
// p01 where pkey0 = p00 (primary-key rekey; updateInScope already supports
// it).
func (s *infraTableUpdateIndexCaseState) deployKeyUpdate(ctx context.Context, label string,
	table string, createLabel string, multi bool) error {
	switch label {
	case createLabel:
		columns := []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("pkey0"),
		}
		if multi {
			columns = append(columns,
				esper.PrimaryKeyColumn[int]("pkey1"),
				esper.OptionalTableColumnOf[int64]("c0"))
		} else {
			columns = append(columns, esper.OptionalTableColumnOf[int]("c0"))
		}
		if _, err := esper.CreateTable(s.env, table, columns); err != nil {
			return err
		}
		return s.registerSnapshot(label, table)
	case "insert":
		assignments := []esper.TableAssignment{
			esper.SetColumn("pkey0", esper.Field[infraTableUpdateBean, string]("theString")),
		}
		if multi {
			assignments = append(assignments,
				esper.SetColumn("pkey1", esper.Field[infraTableUpdateBean, int]("intPrimitive")),
				esper.SetColumn("c0", esper.Field[infraTableUpdateBean, int64]("longPrimitive")))
		} else {
			assignments = append(assignments,
				esper.SetColumn("c0", esper.Field[infraTableUpdateBean, int]("intPrimitive")))
		}
		plan, err := s.env.Build(esper.OnEvent(esper.From[infraTableUpdateBean](s.env, "SupportBean")).
			InsertIntoTable(table, assignments...).Query())
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	case "on-update":
		plan, err := s.env.Build(esper.OnEvent(esper.From[infraTableUpdateS0](s.env, "SupportBean_S0")).
			UpdateTableWhere(table,
				esper.Equal[string](esper.TableField[string]("pkey0"),
					esper.Field[infraTableUpdateS0, string]("p00")),
				esper.SetColumn("pkey0", esper.Field[infraTableUpdateS0, string]("p01")),
			).Query())
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	default:
		return fmt.Errorf("%s: unknown %s deploy label %q",
			infraTableUpdateIndexID, s.caseName, label)
	}
}

// createIndex mirrors a `create [unique] index` deploy step: a late catalog
// operation on the live table. Success registers the deploy label for the
// deployed marker; the deploy-error steps use deployError instead.
func (s *infraTableUpdateIndexCaseState) createIndex(label, table, index string,
	columns []string, unique bool) error {
	handle, ok := s.engine.Table(table)
	if !ok {
		return fmt.Errorf("%s: table %q is not registered", infraTableUpdateIndexID, table)
	}
	if err := handle.CreateIndex(index, columns, esper.IndexHash, unique); err != nil {
		return err
	}
	s.deployedLabels[label] = true
	return nil
}

// fafUpdate executes one compileExecuteFAF update (Faf* deploy label): the
// plan is built on demand and executed through ExecuteFireAndForget with no
// deployment and no marker.
func (s *infraTableUpdateIndexCaseState) fafUpdate(ctx context.Context, step compat.Step,
	column string, value int, key string) error {
	plan, err := s.env.Build(esper.FromTable(s.env, "MyTableFAFU").OnDemand().UpdateWhere(
		esper.Equal[string](esper.TableField[string]("pkey0"), esper.Literal(key)),
		esper.SetColumn(column, esper.Literal(value)),
	))
	if err != nil {
		return fmt.Errorf("%s: build faf update %q: %w", infraTableUpdateIndexID, step.Statement, err)
	}
	if _, err := s.engine.ExecuteFireAndForget(ctx, plan); err != nil {
		return fmt.Errorf("%s: faf update %q: %w", infraTableUpdateIndexID, step.Statement, err)
	}
	return nil
}

// registerSnapshot prepares the fire-and-forget table query behind the
// create-table deploy label so snapshot steps read the table rows the way
// the Java iterator on the named create statement does.
func (s *infraTableUpdateIndexCaseState) registerSnapshot(label, table string) error {
	plan, err := s.env.Build(esper.FromTable(s.env, table).Query(
		esper.StatementName("snapshot-" + table)))
	if err != nil {
		return err
	}
	s.snapshots[label] = plan
	s.deployedLabels[label] = true
	return nil
}

func (s *infraTableUpdateIndexCaseState) deployPlan(ctx context.Context, label string,
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
// semantics), while a create-table label runs the registered table query.
// Rows are projected to the pinned fields and sorted for mode "any".
func (s *infraTableUpdateIndexCaseState) snapshot(ctx context.Context, step compat.Step,
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
			return fmt.Errorf("%s: unknown snapshot %q", infraTableUpdateIndexID, step.Statement)
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
	trace.Records = append(trace.Records, record)
	return nil
}

// buildFafSelect maps the pinned FAF select EPL to the typed plan:
// `select pkey0 from MyTableFAFU where col0=1` and the col1=100 variant.
func (s *infraTableUpdateIndexCaseState) buildFafSelect(step compat.Step) (esper.Plan, error) {
	var column string
	var value int
	switch step.Statement {
	case "FafSelectCol0":
		column, value = "col0", 1
	case "FafSelectCol1":
		column, value = "col1", 100
	default:
		return esper.Plan{}, fmt.Errorf("%s: unknown faf select %q",
			infraTableUpdateIndexID, step.Statement)
	}
	return s.env.Build(esper.FromTable(s.env, "MyTableFAFU").Filter(
		esper.Equal[int](esper.Field[any, int](column), esper.Literal(value)),
	).Select(
		esper.Alias("pkey0", esper.Field[any, string]("pkey0")),
	).Query())
}

// deployError mirrors the Java deploy-fail probe: the create-unique-index
// EPL compiles, then the catalog call must fail. The Go boundary is
// Table.CreateIndex rejecting the unique index (existing-row violation for
// ord 0, merge-updated column for ord 1); the record carries the pinned
// expectError text once the failure verifies.
func (s *infraTableUpdateIndexCaseState) deployError(step compat.Step,
	trace *compat.Trace, sequences map[string]uint64) error {
	var table, index string
	var columns []string
	switch s.caseName + "/" + step.Statement {
	case "early-unique-violation/sec-index-pkey0":
		table, index, columns = "MyTableEUIV", "SecIndex", []string{"pkey0"}
	case "late-unique-violation/sec-index-col0":
		table, index, columns = "MyTableLUIV", "MyUniqueSecondary", []string{"col0"}
	default:
		return fmt.Errorf("%s: unknown deploy-error probe %q in case %q",
			infraTableUpdateIndexID, step.Statement, s.caseName)
	}
	handle, ok := s.engine.Table(table)
	if !ok {
		return fmt.Errorf("%s: deploy-error probe %q table %q is not registered",
			infraTableUpdateIndexID, step.Statement, table)
	}
	err := handle.CreateIndex(index, columns, esper.IndexHash, true)
	if err == nil {
		return fmt.Errorf("%s: deploy-error probe %q unexpectedly succeeded",
			infraTableUpdateIndexID, step.Statement)
	}
	var espErr *esper.Error
	if !errors.As(err, &espErr) {
		return fmt.Errorf("%s: deploy-error probe %q drift: got %v",
			infraTableUpdateIndexID, step.Statement, err)
	}
	s.emitErrorRecord(step, trace, sequences)
	return nil
}

// buildError mirrors the compileWCheckedEx probe: the on-merge whose
// when-matched update assigns a unique-key column must be rejected at build
// time with an invalid-rule error.
func (s *infraTableUpdateIndexCaseState) buildError(step compat.Step,
	trace *compat.Trace, sequences map[string]uint64) error {
	if s.caseName != "early-unique-violation" || step.Statement != "on-merge-pkey1" {
		return fmt.Errorf("%s: unknown build-error probe %q in case %q",
			infraTableUpdateIndexID, step.Statement, s.caseName)
	}
	_, err := s.env.Build(esper.OnEvent(esper.From[infraTableUpdateS1](s.env, "SupportBean_S1")).
		MergeIntoTableWhen("MyTableEUIV",
			[]esper.Expr{
				esper.Field[infraTableUpdateS1, string]("p10"),
				esper.Field[infraTableUpdateS1, int]("id"),
			},
			esper.WhenMatchedAny(esper.SetColumn("pkey1", esper.Literal(0))),
		).Query(esper.StatementName("on-merge")))
	if err == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled",
			infraTableUpdateIndexID, step.Statement)
	}
	if !errors.Is(err, esper.ErrorInvalidRule) {
		return fmt.Errorf("%s: build-error probe %q drift: got %v",
			infraTableUpdateIndexID, step.Statement, err)
	}
	s.emitErrorRecord(step, trace, sequences)
	return nil
}

// fafError mirrors the compileExecuteFAF failure: the no-where update of
// pkey1 must fail atomically with a runtime error, leaving the table
// unchanged for the following snapshot.
func (s *infraTableUpdateIndexCaseState) fafError(ctx context.Context, step compat.Step,
	trace *compat.Trace, sequences map[string]uint64) error {
	if s.caseName != "early-unique-violation" || step.Statement != "faf-update-pkey1" {
		return fmt.Errorf("%s: unknown faf-error probe %q in case %q",
			infraTableUpdateIndexID, step.Statement, s.caseName)
	}
	plan, err := s.env.Build(esper.FromTable(s.env, "MyTableEUIV").OnDemand().UpdateAll(
		esper.SetColumn("pkey1", esper.Literal(0)),
	))
	if err != nil {
		return fmt.Errorf("%s: faf-error probe %q build failed: %w",
			infraTableUpdateIndexID, step.Statement, err)
	}
	_, execErr := s.engine.ExecuteFireAndForget(ctx, plan)
	if execErr == nil {
		return fmt.Errorf("%s: faf-error probe %q unexpectedly succeeded",
			infraTableUpdateIndexID, step.Statement)
	}
	var espErr *esper.Error
	if !errors.As(execErr, &espErr) {
		return fmt.Errorf("%s: faf-error probe %q drift: got %v",
			infraTableUpdateIndexID, step.Statement, execErr)
	}
	s.emitErrorRecord(step, trace, sequences)
	return nil
}

// sendError mirrors the on-update send failure: the SupportBean_S1 send must
// surface the trigger's unique-index violation as an engine error, leaving
// the table unchanged for the following snapshot.
func (s *infraTableUpdateIndexCaseState) sendError(ctx context.Context, step compat.Step,
	trace *compat.Trace, sequences map[string]uint64) error {
	event, err := decodeInfraTableUpdateIndexPayload(step)
	if err != nil {
		return err
	}
	sendErr := s.engine.Send(ctx, step.EventType, event)
	if sendErr == nil {
		return fmt.Errorf("%s: send-error probe %q unexpectedly succeeded",
			infraTableUpdateIndexID, step.Statement)
	}
	var espErr *esper.Error
	if !errors.As(sendErr, &espErr) {
		return fmt.Errorf("%s: send-error probe %q drift: got %v",
			infraTableUpdateIndexID, step.Statement, sendErr)
	}
	s.emitErrorRecord(step, trace, sequences)
	return nil
}

// emitErrorRecord appends the pinned error record: the operation and label
// of the step with the pinned expectError text as the value, matching the
// oracle's record shape.
func (s *infraTableUpdateIndexCaseState) emitErrorRecord(step compat.Step,
	trace *compat.Trace, sequences map[string]uint64) {
	sequences[step.Statement+":"+step.Op]++
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: step.Op,
		Statement: step.Statement,
		Sequence:  sequences[step.Statement+":"+step.Op],
		Time:      "1970-01-01T00:00:00Z",
		Value:     step.ExpectError,
	})
}

func (s *infraTableUpdateIndexCaseState) undeployAll(ctx context.Context) error {
	for _, deployments := range s.deployments {
		for _, deployment := range deployments {
			if err := deployment.Undeploy(ctx); err != nil {
				return err
			}
		}
	}
	s.deployments = make(map[string][]*esper.Deployment)
	s.deployedLabels = make(map[string]bool)
	return nil
}

func decodeInfraTableUpdateIndexPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value infraTableUpdateBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value infraTableUpdateS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value infraTableUpdateS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported infra table update-and-index event type %q", step.EventType)
	}
}

// loadInfraTableUpdateIndexScenario enforces the strict scenario contract
// shared by the differential runners: no duplicate or unknown JSON fields,
// pinned metadata, pinned per-case runtime/execution/EPL, and a per-op step
// field whitelist followed by a full step-shape pin. The generic
// compat.Scenario.Validate is not consulted because deploy-error and
// faf-error are runner-local ops outside its whitelist; the pinned step
// sequence is the stronger check.
func loadInfraTableUpdateIndexScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraTableUpdateIndexID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraTableUpdateIndexID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableUpdateIndexID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableUpdateIndexID, err)
	}
	if err := requireInfraTableUpdateIndexFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraTableUpdateIndexID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraTableUpdateIndexID ||
		metadata.Description != infraTableUpdateIndexDescription ||
		metadata.JavaCommit != infraTableUpdateIndexJavaCommit ||
		metadata.JavaSource != infraTableUpdateIndexJavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraTableUpdateIndexID)
	}
	if err := validateInfraTableUpdateIndexStringArray(root["javaRuntimes"], infraTableUpdateIndexJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableUpdateIndexStringArray(root["javaNames"], infraTableUpdateIndexJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableUpdateIndexStringArray(root["javaStaticIds"], infraTableUpdateIndexJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableUpdateIndexStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraTableUpdateIndexCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraTableUpdateIndexID, len(infraTableUpdateIndexCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraTableUpdateIndexFields(object,
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
		if definition.Case != infraTableUpdateIndexCases[index] ||
			definition.Ordinal != infraTableUpdateIndexOrdinals[index] ||
			definition.RuntimeID != infraTableUpdateIndexJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraTableUpdateIndexJavaExecutions[index] ||
			definition.Observation != infraTableUpdateIndexCaseObservations[index] ||
			definition.EPL != infraTableUpdateIndexCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraTableUpdateIndexID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraTableUpdateIndexID)
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
			if err := requireInfraTableUpdateIndexFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraTableUpdateIndexFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed", "undeploy":
			if err := requireInfraTableUpdateIndexFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraTableUpdateIndexFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraTableUpdateIndexPayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send-error":
			if err := requireInfraTableUpdateIndexFields(object, "op", "case", "statement", "eventType", "payload", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			// Statement snapshots carry op/case/statement/mode/fields; FAF
			// selects additionally carry the pinned query epl.
			if _, hasEpl := object["epl"]; hasEpl {
				if err := requireInfraTableUpdateIndexFields(object, "op", "case", "statement", "mode", "fields", "epl"); err != nil {
					return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
				}
			} else if err := requireInfraTableUpdateIndexFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy-error", "build-error", "faf-error":
			if err := requireInfraTableUpdateIndexFields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraTableUpdateIndexFields(object, "op", "case"); err != nil {
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
		for _, name := range infraTableUpdateIndexCases {
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
	if err := validateInfraTableUpdateIndexRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraTableUpdateIndexRawSteps pins the complete step sequence per
// case against the raw JSON objects: deploy steps with byte-exact EPL,
// deployed markers, send event types with canonical payloads, error probes
// with pinned EPL and expectError, snapshot reads, and the
// undeploy/undeploy-all terminators.
func validateInfraTableUpdateIndexRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraTableUpdateIndexCases {
		want, ok := infraTableUpdateIndexCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraTableUpdateIndexID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraTableUpdateIndexID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraTableUpdateIndexID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraTableUpdateIndexID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraTableUpdateIndexStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraTableUpdateIndexID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q",
					infraTableUpdateIndexID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraTableUpdateIndexID)
	}
	return nil
}

// infraTableUpdateIndexStepKey renders a raw step object into its pinned
// string form. Fields are read from the raw JSON because compat.Step does
// not carry the fields array.
func infraTableUpdateIndexStepKey(object map[string]json.RawMessage, operation string) (string, error) {
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
	case "deployed", "undeploy":
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
	case "send-error":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		eventType, err := stringField("eventType")
		if err != nil {
			return "", err
		}
		var payload map[string]any
		if err := json.Unmarshal(object["payload"], &payload); err != nil {
			return "", fmt.Errorf("send-error payload: %w", err)
		}
		canonical, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("send-error payload: %w", err)
		}
		expectError, err := stringField("expectError")
		if err != nil {
			return "", err
		}
		return "send-error:" + statement + ":" + eventType + ":" + string(canonical) + ":" + expectError, nil
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
		if eplValue, hasEpl := object["epl"]; hasEpl {
			var epl string
			if err := json.Unmarshal(eplValue, &epl); err != nil {
				return "", fmt.Errorf("step field %q must be a string", "epl")
			}
			key += ":" + epl
		}
		return key, nil
	case "deploy-error", "build-error", "faf-error":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		epl, err := stringField("epl")
		if err != nil {
			return "", err
		}
		expectError, err := stringField("expectError")
		if err != nil {
			return "", err
		}
		return operation + ":" + statement + ":" + epl + ":" + expectError, nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// tuixEarlyCaseSteps renders the pinned step sequence of
// InfraEarlyUniqueIndexViolation.run (lines 47-92).
func tuixEarlyCaseSteps() []string {
	return []string{
		"deploy:create:" + tuixEarlyCreate,
		"deployed:create",
		"deploy:into:" + tuixEarlyInto,
		"deployed:into",
		`send:SupportBean:{"intPrimitive":10,"theString":"E1"}`,
		`send:SupportBean:{"intPrimitive":20,"theString":"E1"}`,
		"deploy-error:sec-index-pkey0:" + tuixEarlySecIndex + ":" + tuixErrEarlySecIndex,
		"faf-error:faf-update-pkey1:" + tuixEarlyFafUpd + ":" + tuixErrEarlyFafUpd,
		"snapshot:create:any:pkey0,pkey1",
		"deploy:on-update:" + tuixEarlyOnUpdate,
		"deployed:on-update",
		`send-error:on-update-send:SupportBean_S1:{"id":0}:` + tuixErrEarlyOnUpdate,
		"snapshot:create:any:pkey0,pkey1",
		"build-error:on-merge-pkey1:" + tuixEarlyOnMerge + ":" + tuixErrOnMergeUnique,
		"undeploy-all",
	}
}

// tuixLateCaseSteps renders the pinned step sequence of
// InfraLateUniqueIndexViolation.run (lines 101-141).
func tuixLateCaseSteps() []string {
	return []string{
		"deploy:create:" + tuixLateCreate,
		"deployed:create",
		"deploy:into:" + tuixLateInto,
		"deployed:into",
		`send:SupportBean:{"intPrimitive":10,"theString":"E1"}`,
		`send:SupportBean:{"intPrimitive":20,"theString":"E2"}`,
		"deploy:on-merge:" + tuixLateOnMerge,
		"deployed:on-merge",
		"deploy-error:sec-index-col0:" + tuixLateSecIndexCol0 + ":" + tuixErrLateSecIndex,
		"undeploy:on-merge",
		"deploy:on-update:" + tuixLateOnUpdate,
		"deployed:on-update",
		"deploy:sec-index-pkey1:" + tuixLateSecIndexPkey1,
		"deployed:sec-index-pkey1",
		`send-error:on-update-send:SupportBean_S1:{"id":0}:` + tuixErrLateOnUpdate,
		"snapshot:create:any:pkey0,pkey1",
		"undeploy:on-update",
		"undeploy-all",
	}
}

// tuixFafCaseSteps renders the pinned step sequence of InfraFAFUpdate.run
// (lines 149-166): the FAF updates ride deploy steps and the FAF selects
// ride snapshot steps carrying the pinned query EPL.
func tuixFafCaseSteps() []string {
	return []string{
		"deploy:create:" + tuixFafCreate,
		"deployed:create",
		"deploy:create-index:" + tuixFafIndex,
		"deployed:create-index",
		"deploy:into:" + tuixFafInto,
		"deployed:into",
		`send:SupportBean:{"intPrimitive":0,"theString":"E1"}`,
		`send:SupportBean:{"intPrimitive":0,"theString":"E2"}`,
		"deploy:FafUpdateCol0E1:" + tuixFafUpdateCol0E1,
		"deploy:FafUpdateCol0E2:" + tuixFafUpdateCol0E2,
		"snapshot:FafSelectCol0:ordered:pkey0:" + tuixFafSelectCol0,
		"deploy:FafUpdateCol1E1:" + tuixFafUpdateCol1E1,
		"snapshot:FafSelectCol1:ordered:pkey0:" + tuixFafSelectCol1,
		"undeploy-all",
	}
}

// tuixKeyUpdateCaseSteps renders the pinned step sequence of
// InfraTableKeyUpdateSingleKey.run (lines 209-241) and
// InfraTableKeyUpdateMultiKey.run (lines 174-205); milestone checkpoints are
// harness no-ops and carry no steps.
func tuixKeyUpdateCaseSteps(single bool) []string {
	createLabel, createEpl, insertEpl, onUpdateEpl := "s0", tuixSingleCreate, tuixSingleInsert, tuixSingleOnUpdate
	fields := "pkey0,c0"
	if !single {
		createLabel, createEpl, insertEpl, onUpdateEpl = "s1", tuixMultiCreate, tuixMultiInsert, tuixMultiOnUpdate
		fields = "pkey0,pkey1,c0"
	}
	steps := []string{
		"deploy:" + createLabel + ":" + createEpl,
		"deployed:" + createLabel,
		"deploy:insert:" + insertEpl,
		"deployed:insert",
		"deploy:on-update:" + onUpdateEpl,
		"deployed:on-update",
	}
	if single {
		steps = append(steps,
			`send:SupportBean:{"intPrimitive":10,"theString":"E1"}`,
			`send:SupportBean:{"intPrimitive":20,"theString":"E2"}`,
			`send:SupportBean:{"intPrimitive":30,"theString":"E3"}`)
	} else {
		steps = append(steps,
			`send:SupportBean:{"intPrimitive":10,"longPrimitive":100,"theString":"E1"}`,
			`send:SupportBean:{"intPrimitive":20,"longPrimitive":200,"theString":"E2"}`,
			`send:SupportBean:{"intPrimitive":30,"longPrimitive":300,"theString":"E3"}`)
	}
	return append(steps,
		`send:SupportBean_S0:{"id":0,"p00":"E2","p01":"E20"}`,
		"snapshot:"+createLabel+":any:"+fields,
		`send:SupportBean_S0:{"id":0,"p00":"E1","p01":"E10"}`,
		"snapshot:"+createLabel+":any:"+fields,
		`send:SupportBean_S0:{"id":0,"p00":"E3","p01":"E30"}`,
		"snapshot:"+createLabel+":any:"+fields,
		"undeploy-all")
}

// infraTableUpdateIndexCaseSteps pins the exact op sequence per case.
var infraTableUpdateIndexCaseSteps = map[string][]string{
	"early-unique-violation": tuixEarlyCaseSteps(),
	"late-unique-violation":  tuixLateCaseSteps(),
	"faf-update":             tuixFafCaseSteps(),
	"key-update-single":      tuixKeyUpdateCaseSteps(true),
	"key-update-multi":       tuixKeyUpdateCaseSteps(false),
}

// infraTableUpdateIndexSnapshotFields extracts the pinned projection list
// for the snapshot at steps[stepIndex] from the pinned step key
// ("snapshot:<statement>:<mode>:<f1,f2,...>" with an optional :epl tail).
// The case marker occupies steps[0], so the pinned index is stepIndex-1.
func infraTableUpdateIndexSnapshotFields(pinned []string, stepIndex int) []string {
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

func requireInfraTableUpdateIndexFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraTableUpdateIndexID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraTableUpdateIndexID, name)
		}
	}
	return nil
}

func validateInfraTableUpdateIndexStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}

// infraTableUpdateIndexRuntimeID maps each scenario case to the inventory
// runtime ID of the Java execution it replays.
func infraTableUpdateIndexRuntimeID(caseName string) string {
	for index, name := range infraTableUpdateIndexCases {
		if name == caseName {
			return infraTableUpdateIndexJavaRuntimeIDs[index]
		}
	}
	return ""
}
