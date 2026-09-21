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

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// infra_table_context.go replays InfraTableContext ordinals 0-2 against the
// pinned Java oracle: tables declared under a context (partitioned,
// non-overlapping initiated/terminated and context-visibility compile
// errors).
//
// context-partitioned (ord 0, InfraPartitioned) deploys the CtxPerString
// segmented context (theString from SupportBean, p00 from SupportBean_S0),
// the unkeyed context-bound MyTable(thesum sum(int)), an into-table sum feed
// and the listened s0 `select MyTable.thesum as c0 from SupportBean_S0`; the
// listener pins c0=110 for S0(0,"E1") and c0=20 for S0(0,"E2") across
// milestone(0).
//
// context-nonoverlapping (ord 1, InfraNonOverlapping) deploys the
// CtxNowTillS0 context (start @now end SupportBean_S0), the keyed
// context-bound MyTable(pkey primary key, thesum sum(int), col0 string), a
// grouped into-table sum feed keyed on theString and the listened s0
// `select pkey as c0, thesum as c1 from MyTable output snapshot when
// terminated`; each SupportBean_S0(-1) terminator emits the whole partition
// table as one new-data batch ({E1,110},{E2,20} then {E1,30},{E3,100}). A
// mid-run `create index MyIdx on MyTable(col0)` and a deploy-only
// `select * from MyTable, SupportBean_S1 where col0 = p11` join carry
// deployed markers only.
//
// context-invalid (ord 2, InfraTableContextInvalid) deploys the SimpleCtx
// scheduled context (start after 1 sec end after 1 sec) and the keyed
// context-bound MyTable, then runs three tryInvalidCompile probes whose
// pinned Java message prefixes record as compile-error records after the Go
// rejection is verified for code and wording.
//
// Approved differences (observably identical to the Java EPL):
//   - `create context`, `create table` and `create index` map to env-level
//     catalog calls (CreateKeyContextByStreams,
//     CreateInitiatedTerminatedContext, CreateScheduledTimePeriodContext,
//     CreateTable with TableContext, Table.CreateIndex); the deploy labels
//     carry the pinned EPL while deployed markers pin the step labels (the
//     infra-table-faf-execute-query precedent).
//   - `select MyTable.thesum as c0 from SupportBean_S0` maps to the
//     established OnEvent+SelectFromTableWhere trigger form (the
//     InfraUngroupedWContext precedent at
//     infra_table_access_parity_test.go:127).
//   - `select * from MyTable, SupportBean_S1 where col0 = p11` maps to a
//     JoinMany where-clause join; it deploys with no listener and no S1
//     send, exactly like the Java execution.
//   - Java milestone checkpoints are harness no-ops and carry no steps.
//   - Java's assertPropsPerRowLastNewAnyOrder batches record in canonical
//     sorted-field order on both traces (the
//     resultset-querytype-local-group-solution-pattern precedent).
//   - Java's tryInvalidCompile(path, epl, prefix) probes map to build-error
//     steps: the runner verifies the Go rejection carries ErrorInvalidRule
//     plus the context-visibility wording before recording the pinned Java
//     prefix (the context-key-segmented-invalid precedent).

// infraTableContextBean mirrors SupportBean's asserted fields (theString,
// intPrimitive).
type infraTableContextBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// infraTableContextS0 mirrors SupportBean_S0's asserted fields (id, p00).
type infraTableContextS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// infraTableContextS1 mirrors SupportBean_S1's asserted fields (id, p10,
// p11); the ord-1 join references p11 but the scenario never sends S1.
type infraTableContextS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
	P11 string `esper:"p11"`
}

const (
	infraTableContextID          = "infra-table-context"
	infraTableContextJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraTableContextJavaSource  = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableContext.java"
	infraTableContextDescription = "InfraTableContext ordinals 0-2: InfraPartitioned deploys the CtxPerString partitioned context, an unkeyed context-bound MyTable, an into-table sum feed and an s0 reading MyTable.thesum per SupportBean_S0 partition key, pinning c0=110 then c0=20; InfraNonOverlapping deploys the CtxNowTillS0 start-@now/end-SupportBean_S0 context, a keyed context-bound MyTable fed by a grouped into-table sum and an s0 emitting the partition table as one output-snapshot-when-terminated batch per SupportBean_S0(-1) terminator ({E1,110},{E2,20} then {E1,30},{E3,100}), with a late create index on col0 and a deploy-only MyTable/SupportBean_S1 join; InfraTableContextInvalid deploys the SimpleCtx scheduled context and a context-bound keyed MyTable, then pins three tryInvalidCompile prefixes for a contextless select, a contextless subquery and a contextless insert-into (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableContext.java)."

	// Verbatim transcriptions of InfraTableContext lines 90-94 (ord 0);
	// 55-58 and 70-71 (ord 1); and 38-39 plus 41-46 (ord 2, the probe EPLs
	// and their assertMessage prefixes).
	itcPartCtx    = "@public create context CtxPerString partition by theString from SupportBean, p00 from SupportBean_S0"
	itcPartCreate = "@public context CtxPerString create table MyTable(thesum sum(int))"
	itcPartInto   = "context CtxPerString into table MyTable select sum(intPrimitive) as thesum from SupportBean"
	itcPartS0     = "@name('s0') context CtxPerString select MyTable.thesum as c0 from SupportBean_S0"

	itcNonOverlapCtx    = "@public create context CtxNowTillS0 start @now end SupportBean_S0"
	itcNonOverlapCreate = "@public context CtxNowTillS0 create table MyTable(pkey string primary key, thesum sum(int), col0 string)"
	itcNonOverlapInto   = "context CtxNowTillS0 into table MyTable select sum(intPrimitive) as thesum from SupportBean group by theString"
	itcNonOverlapS0     = "@name('s0') context CtxNowTillS0 select pkey as c0, thesum as c1 from MyTable output snapshot when terminated"
	itcNonOverlapIndex  = "context CtxNowTillS0 create index MyIdx on MyTable(col0)"
	itcNonOverlapJoin   = "context CtxNowTillS0 select * from MyTable, SupportBean_S1 where col0 = p11"

	itcInvalidCtx    = "@public create context SimpleCtx start after 1 sec end after 1 sec"
	itcInvalidCreate = "@public context SimpleCtx create table MyTable(pkey string primary key, thesum sum(int), col0 string)"

	itcProbeSelect   = "select * from MyTable"
	itcProbeSubquery = "select (select * from MyTable) from SupportBean"
	itcProbeInsert   = "insert into MyTable select theString as pkey from SupportBean"

	itcErrTableVisibility = "Table by name 'MyTable' has been declared for context 'SimpleCtx' and can only be used within the same context ["
	itcErrSubquery        = "Failed to plan subquery number 1 querying MyTable: Mismatch in context specification, the context for the table 'MyTable' is 'SimpleCtx' and the query specifies no context  [select (select * from MyTable) from SupportBean]"
)

var (
	infraTableContextJavaSources = []string{
		infraTableContextJavaSource,
	}
	infraTableContextJavaRuntimeIDs = []string{
		"java-runtime-8b5b2d92d108da7e8fb2",
		"java-runtime-5c625828c160a26df78b",
		"java-runtime-df03d93aca6b6b5ccd59",
	}
	infraTableContextJavaExecutions = []string{
		"InfraPartitioned",
		"InfraNonOverlapping",
		"InfraTableContextInvalid",
	}
	infraTableContextJavaStaticIDs = []string{
		"java-62ab3ac014745d5ab5c5",
		"java-969b28d4058f010a19af",
		"java-69b99206b5ca50456737",
	}
	infraTableContextCases = []string{
		"context-partitioned",
		"context-nonoverlapping",
		"context-invalid",
	}
	infraTableContextOrdinals         = []int{0, 1, 2}
	infraTableContextCaseObservations = []string{
		"listener; the partitioned context keys MyTable per theString/p00 so S0(0,E1) reads the E1-partition sum 110 and S0(0,E2) reads 20",
		"listener; s0 emits the whole partition table as one any-order new batch per SupportBean_S0(-1) terminator ({E1,110},{E2,20} then {E1,30},{E3,100}); the mid-run create index on col0 and the deploy-only MyTable/SupportBean_S1 join carry deployed markers only",
		"compile-error; three tryInvalidCompile probes pin the context-visibility prefixes for a contextless select, a contextless subquery and a contextless insert-into against SimpleCtx-bound MyTable",
	}
)

// infraTableContextCaseEPLs pins the newline-joined EPL of every
// EPL-bearing step in the case, in step order (deploys and build-error
// probes) — the value carried by the scenario cases[] metadata.
var infraTableContextCaseEPLs = []string{
	strings.Join([]string{itcPartCtx, itcPartCreate, itcPartInto, itcPartS0}, "\n"),
	strings.Join([]string{itcNonOverlapCtx, itcNonOverlapCreate, itcNonOverlapInto,
		itcNonOverlapS0, itcNonOverlapIndex, itcNonOverlapJoin}, "\n"),
	strings.Join([]string{itcInvalidCtx, itcInvalidCreate, itcProbeSelect,
		itcProbeSubquery, itcProbeInsert}, "\n"),
}

// infraTableContextCaseState carries the per-case replay state: the
// environment/engine pair, the label→deployments bookkeeping used by
// undeploy-all, the deployed-label set for marker checks, the per-statement
// listener sequence counters, and the trace the listener records into.
type infraTableContextCaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	deployments    map[string][]*esper.Deployment
	deployedLabels map[string]bool
	sequences      map[string]uint64
	trace          *compat.Trace
	caseName       string
}

// runInfraTableContextScenario replays the three InfraTableContext
// executions: each case runs on a fresh environment/engine pair (one runtime
// per Java execution) and every step dispatches to the matching runtime
// action. The oracle emits the epoch time for every record, so the runner
// pins the same value.
func runInfraTableContextScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraTableContextScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraTableContextCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraTableContextCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraTableContextID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraTableContextID)
	}
	return trace, nil
}

func runInfraTableContextCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraTableContextBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraTableContextS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraTableContextS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraTableContextJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: infraTableContextID}
	state := &infraTableContextCaseState{
		env:            env,
		engine:         engine,
		deployments:    make(map[string][]*esper.Deployment),
		deployedLabels: make(map[string]bool),
		sequences:      make(map[string]uint64),
		trace:          &trace,
		caseName:       caseName,
	}
	for _, step := range steps {
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
					infraTableContextID, step.Statement)
			}
			state.sequences[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  state.sequences[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(engine.Now()),
			})
		case "send":
			event, err := decodeInfraTableContextPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return compat.Trace{}, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s: unexpected op %q", infraTableContextID, step.Op)
		}
	}
	return trace, nil
}

// record emits one listener record per invocation with a per-statement
// sequence counter, mirroring the Java oracle's UpdateListener: only a new
// array renders and only when non-empty. Rows sort by their compact field
// rendering because the ord-1 batches are asserted with
// assertPropsPerRowLastNewAnyOrder.
func (s *infraTableContextCaseState) record(statement string, batch esper.ResultBatch) {
	s.sequences[statement]++
	rec := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "listener",
		Statement: statement,
		Sequence:  s.sequences[statement],
		Time:      compat.FormatTraceTime(batch.Time),
		New:       compat.NormalizeResults(batch.New),
		Old:       compat.NormalizeResults(batch.Old),
	}
	sortRowsCanonical(rec.New)
	sortRowsCanonical(rec.Old)
	if len(rec.Old) == 0 {
		rec.Old = nil
	}
	s.trace.Records = append(s.trace.Records, rec)
}

// deploy maps each scenario label to the equivalent Go chain-API plans or
// catalog calls. The Java oracle compiles each deploy step's EPL as one
// module; the Go runner deploys the equivalent plans and registers them
// under the step label so undeploy-all bookkeeping matches.
func (s *infraTableContextCaseState) deploy(ctx context.Context, step compat.Step) error {
	switch s.caseName {
	case "context-partitioned":
		return s.deployPartitioned(ctx, step)
	case "context-nonoverlapping":
		return s.deployNonOverlapping(ctx, step)
	case "context-invalid":
		return s.deployInvalid(ctx, step)
	default:
		return fmt.Errorf("%s: unsupported case %q", infraTableContextID, s.caseName)
	}
}

// deployPartitioned builds the ord-0 fixture: the CtxPerString segmented
// context, the unkeyed context-bound MyTable, the into-table sum feed and
// the listened s0 reading MyTable.thesum per SupportBean_S0 partition key.
func (s *infraTableContextCaseState) deployPartitioned(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "ctx":
		// `@public create context CtxPerString partition by theString from
		// SupportBean, p00 from SupportBean_S0`.
		if _, err := esper.CreateKeyContextByStreams(s.env, "CtxPerString",
			esper.KeyContextStream{
				Type: "SupportBean",
				Keys: []esper.Expr{esper.Field[infraTableContextBean, string]("theString")},
			},
			esper.KeyContextStream{
				Type: "SupportBean_S0",
				Keys: []esper.Expr{esper.Field[infraTableContextS0, string]("p00")},
			}); err != nil {
			return err
		}
		s.deployedLabels[step.Statement] = true
		return nil
	case "create":
		// `@public context CtxPerString create table MyTable(thesum
		// sum(int))` — an unkeyed table bound to the context.
		if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
			esper.TableColumnOf[int]("thesum"),
		}, esper.TableContext("CtxPerString")); err != nil {
			return err
		}
		s.deployedLabels[step.Statement] = true
		return nil
	case "into":
		// `context CtxPerString into table MyTable select sum(intPrimitive)
		// as thesum from SupportBean` — an ungrouped aggregate feed into
		// the unkeyed per-partition row.
		intPrimitive := esper.Field[infraTableContextBean, int]("intPrimitive")
		plan, err := s.env.Build(esper.From[infraTableContextBean](s.env, "SupportBean").
			Aggregate(esper.Alias("thesum", esper.Sum[int](intPrimitive))).
			IntoTable("MyTable", esper.WithContext("CtxPerString")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, step.Statement, plan)
	case "s0":
		// `select MyTable.thesum as c0 from SupportBean_S0` — the dotted
		// table-column read maps to the established OnEvent +
		// SelectFromTableWhere trigger form (the InfraUngroupedWContext
		// precedent).
		plan, err := s.env.Build(esper.OnEvent(esper.From[infraTableContextS0](s.env, "SupportBean_S0")).
			SelectFromTableWhere("MyTable", esper.Literal(true),
				esper.Alias("c0", esper.TableField[int]("thesum")),
			).
			Query(esper.StatementName("s0"), esper.WithContext("CtxPerString")))
		if err != nil {
			return err
		}
		return s.deployListened(ctx, step.Statement, plan)
	default:
		return fmt.Errorf("%s: unknown context-partitioned deploy label %q", infraTableContextID, step.Statement)
	}
}

// deployNonOverlapping builds the ord-1 fixture: the CtxNowTillS0
// initiated-terminated context, the keyed context-bound MyTable, the
// grouped into-table sum feed, the listened s0 emitting the partition table
// per terminator, the mid-run secondary index on col0 and the deploy-only
// MyTable/SupportBean_S1 join.
func (s *infraTableContextCaseState) deployNonOverlapping(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "ctx":
		// `@public create context CtxNowTillS0 start @now end
		// SupportBean_S0` — Literal(true) start mirrors @now; the end
		// condition fires on every SupportBean_S0 event.
		end := esper.Equal[string](
			esper.TypeName(esper.EventValue[esper.Event]()),
			esper.Literal("SupportBean_S0"))
		if _, err := esper.CreateInitiatedTerminatedContext(s.env, "CtxNowTillS0",
			esper.Literal("global"), esper.Literal(true), end); err != nil {
			return err
		}
		s.deployedLabels[step.Statement] = true
		return nil
	case "create":
		// `@public context CtxNowTillS0 create table MyTable(pkey string
		// primary key, thesum sum(int), col0 string)` — a keyed table bound
		// to the context; col0 is never populated.
		if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("pkey"),
			esper.TableColumnOf[int]("thesum"),
			esper.OptionalTableColumnOf[string]("col0"),
		}, esper.TableContext("CtxNowTillS0")); err != nil {
			return err
		}
		s.deployedLabels[step.Statement] = true
		return nil
	case "into":
		// `context CtxNowTillS0 into table MyTable select sum(intPrimitive)
		// as thesum from SupportBean group by theString` — the group key
		// maps positionally to primary-key column pkey.
		theString := esper.Field[infraTableContextBean, string]("theString")
		intPrimitive := esper.Field[infraTableContextBean, int]("intPrimitive")
		plan, err := s.env.Build(esper.From[infraTableContextBean](s.env, "SupportBean").
			GroupBy(theString).
			Select(
				esper.Alias("pkey", theString),
				esper.Alias("thesum", esper.Sum[int](intPrimitive)),
			).IntoTable("MyTable", esper.WithContext("CtxNowTillS0")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, step.Statement, plan)
	case "s0":
		// `select pkey as c0, thesum as c1 from MyTable output snapshot when
		// terminated` — emits the whole partition table as one new-data
		// batch per partition end.
		plan, err := s.env.Build(esper.FromTable(s.env, "MyTable").Select(
			esper.Alias("c0", esper.Field[any, string]("pkey")),
			esper.Alias("c1", esper.Field[any, int]("thesum")),
		).Query(esper.StatementName("s0"), esper.WithContext("CtxNowTillS0"),
			esper.WithOutput(esper.OutputSnapshotWhenTerminated())))
		if err != nil {
			return err
		}
		return s.deployListened(ctx, step.Statement, plan)
	case "index":
		// `context CtxNowTillS0 create index MyIdx on MyTable(col0)` — a
		// late catalog operation on the live context-bound table.
		table, ok := s.engine.Table("MyTable")
		if !ok {
			return fmt.Errorf("%s: MyTable is missing", infraTableContextID)
		}
		if err := table.CreateIndex("MyIdx", []string{"col0"}, esper.IndexHash, false); err != nil {
			return fmt.Errorf("%s: create index: %w", infraTableContextID, err)
		}
		s.deployedLabels[step.Statement] = true
		return nil
	case "join":
		// `context CtxNowTillS0 select * from MyTable, SupportBean_S1 where
		// col0 = p11` — a deploy-only where-clause join over the table and
		// the S1 stream; no listener attaches and no S1 event is sent.
		plan, err := s.env.Build(esper.JoinMany(
			esper.JoinRecordSource(esper.FromTable(s.env, "MyTable")),
			esper.JoinSource(esper.From[infraTableContextS1](s.env, "SupportBean_S1")).Unidirectional(),
		).Select(
			esper.SelectSourceEvent(0, "MyTable"),
			esper.SelectSourceEvent(1, "SupportBean_S1"),
		).Where(esper.Equal[string](
			esper.JoinField[string](0, "col0"),
			esper.JoinField[string](1, "p11"),
		)).Query(esper.WithContext("CtxNowTillS0")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, step.Statement, plan)
	default:
		return fmt.Errorf("%s: unknown context-nonoverlapping deploy label %q", infraTableContextID, step.Statement)
	}
}

// deployInvalid builds the ord-2 fixture: the SimpleCtx scheduled context
// and the keyed context-bound MyTable the three probes compile against.
func (s *infraTableContextCaseState) deployInvalid(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "ctx":
		// `@public create context SimpleCtx start after 1 sec end after
		// 1 sec` — the scheduled-start time-period form.
		if _, err := esper.CreateScheduledTimePeriodContext(s.env, "SimpleCtx",
			time.Second, time.Second); err != nil {
			return err
		}
		s.deployedLabels[step.Statement] = true
		return nil
	case "create":
		// `@public context SimpleCtx create table MyTable(pkey string
		// primary key, thesum sum(int), col0 string)`.
		if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("pkey"),
			esper.TableColumnOf[int]("thesum"),
			esper.OptionalTableColumnOf[string]("col0"),
		}, esper.TableContext("SimpleCtx")); err != nil {
			return err
		}
		s.deployedLabels[step.Statement] = true
		return nil
	default:
		return fmt.Errorf("%s: unknown context-invalid deploy label %q", infraTableContextID, step.Statement)
	}
}

// infraTableContextProbe pins one ord-2 tryInvalidCompile probe: the
// byte-exact EPL, the pinned Java message prefix and the Go rejection
// wording verified before the prefix is recorded.
type infraTableContextProbe struct {
	epl         string
	expectError string
	substring   string
}

var infraTableContextProbes = map[string]infraTableContextProbe{
	"select-table": {
		epl:         itcProbeSelect,
		expectError: itcErrTableVisibility,
		substring:   "has been declared for context",
	},
	"subquery-table": {
		epl:         itcProbeSubquery,
		expectError: itcErrSubquery,
		substring:   "mismatch in context specification",
	},
	"insert-table": {
		epl:         itcProbeInsert,
		expectError: itcErrTableVisibility,
		substring:   "has been declared for context",
	},
}

// buildError runs one ord-2 tryInvalidCompile probe against the fluent
// equivalent of the pinned EPL. Each probe verifies Go rejects the
// statement with ErrorInvalidRule plus the context-visibility wording
// before recording the pinned Java message prefix.
func (s *infraTableContextCaseState) buildError(step compat.Step) error {
	probe, ok := infraTableContextProbes[step.Statement]
	if !ok || step.Epl != probe.epl || step.ExpectError != probe.expectError {
		return fmt.Errorf("%s: build-error probe %q is not pinned", infraTableContextID, step.Statement)
	}
	var buildErr error
	switch step.Statement {
	case "select-table":
		// `select * from MyTable` — a contextless table source.
		_, buildErr = s.env.Build(esper.FromTable(s.env, "MyTable").Query())
	case "subquery-table":
		// `select (select * from MyTable) from SupportBean` — a contextless
		// statement whose scalar subquery reads the context-bound table.
		_, buildErr = s.env.Build(esper.Select(
			esper.From[infraTableContextBean](s.env, "SupportBean"),
			esper.Alias("c0", esper.SubqueryValueWithOptions[esper.Event](
				esper.FromTable(s.env, "MyTable"),
				esper.EventValue[esper.Event](),
				esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))),
		).Query())
	case "insert-table":
		// `insert into MyTable select theString as pkey from SupportBean` —
		// a contextless insert-into targeting the context-bound table.
		_, buildErr = s.env.Build(esper.OnEvent(
			esper.From[infraTableContextBean](s.env, "SupportBean")).
			InsertIntoTable("MyTable",
				esper.SetColumn("pkey", esper.Field[infraTableContextBean, string]("theString")),
			).Query())
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", infraTableContextID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", infraTableContextID, step.Statement)
	}
	var espErr *esper.Error
	if !errors.As(buildErr, &espErr) || espErr.Code != esper.ErrorInvalidRule ||
		!strings.Contains(buildErr.Error(), probe.substring) {
		return fmt.Errorf("%s: build-error probe %q drift: got %v", infraTableContextID, step.Statement, buildErr)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

func (s *infraTableContextCaseState) deployPlan(ctx context.Context, label string,
	plan esper.Plan) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[label] = append(s.deployments[label], deployment)
	s.deployedLabels[label] = true
	return nil
}

// deployListened deploys a plan and subscribes the listener to its s0
// statement, mirroring the Java oracle's addListener("s0").
func (s *infraTableContextCaseState) deployListened(ctx context.Context, label string,
	plan esper.Plan) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[label] = append(s.deployments[label], deployment)
	s.deployedLabels[label] = true
	for _, statement := range deployment.Statements() {
		name := statement.Name()
		if name != "s0" {
			continue
		}
		if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			s.record(name, batch)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *infraTableContextCaseState) undeployAll(ctx context.Context) error {
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

func decodeInfraTableContextPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value infraTableContextBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value infraTableContextS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value infraTableContextS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraTableContextID, step.EventType)
	}
}

// loadInfraTableContextScenario enforces the strict scenario contract
// shared by the differential runners: no duplicate or unknown JSON fields,
// pinned metadata, pinned per-case runtime/execution/EPL, and a per-op step
// field whitelist followed by a full step-shape pin.
func loadInfraTableContextScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraTableContextID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraTableContextID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableContextID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableContextID, err)
	}
	if err := requireInfraTableContextFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraTableContextID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraTableContextID ||
		metadata.Description != infraTableContextDescription ||
		metadata.JavaCommit != infraTableContextJavaCommit ||
		metadata.JavaSource != infraTableContextJavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraTableContextID)
	}
	if err := validateInfraTableContextStringArray(root["javaRuntimes"], infraTableContextJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableContextStringArray(root["javaNames"], infraTableContextJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableContextStringArray(root["javaStaticIds"], infraTableContextJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableContextStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraTableContextCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraTableContextID, len(infraTableContextCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraTableContextFields(object,
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
		if definition.Case != infraTableContextCases[index] ||
			definition.Ordinal != infraTableContextOrdinals[index] ||
			definition.RuntimeID != infraTableContextJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraTableContextJavaExecutions[index] ||
			definition.Observation != infraTableContextCaseObservations[index] ||
			definition.IteratorSnapshots != 0 ||
			definition.EPL != infraTableContextCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraTableContextID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraTableContextID)
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
			if err := requireInfraTableContextFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraTableContextFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraTableContextFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraTableContextFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraTableContextPayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "build-error":
			if err := requireInfraTableContextFields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraTableContextFields(object, "op", "case"); err != nil {
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
		for _, name := range infraTableContextCases {
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
	if err := validateInfraTableContextRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraTableContextScenario re-pins the loaded scenario shape
// before replay (the runner entry point validates independently of the
// loader).
func validateInfraTableContextScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraTableContextID {
		return fmt.Errorf("%s scenario shape is not pinned", infraTableContextID)
	}
	return nil
}

// validateInfraTableContextRawSteps pins the complete step sequence per
// case against the raw JSON objects: deploy steps with byte-exact EPL,
// deployed markers, send event types with canonical payloads, build-error
// probes with their pinned EPL and expectError prefix, and the
// undeploy-all terminators.
func validateInfraTableContextRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraTableContextCases {
		want, ok := infraTableContextCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraTableContextID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraTableContextID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraTableContextID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraTableContextID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraTableContextStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraTableContextID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q",
					infraTableContextID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraTableContextID)
	}
	return nil
}

// infraTableContextStepKey renders a raw step object into its pinned
// string form.
func infraTableContextStepKey(object map[string]json.RawMessage, operation string) (string, error) {
	stringField := func(name string) (string, error) {
		var value string
		if err := json.Unmarshal(object[name], &value); err != nil {
			return "", fmt.Errorf("step field %q must be a string", name)
		}
		return value, nil
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
	case "build-error":
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
		return "build-error:" + statement + ":" + epl + ":" + expectError, nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// itcPartitionedCaseSteps renders the pinned step sequence of
// InfraPartitioned.run (lines 89-107): the context, create-table,
// into-table and s0 deploys, SupportBean("E1",50)/("E2",20)/("E1",60), the
// S0(0,"E1")/S0(0,"E2") reads, milestone(0) (no step) and undeployAll.
func itcPartitionedCaseSteps() []string {
	return []string{
		"deploy:ctx:" + itcPartCtx,
		"deployed:ctx",
		"deploy:create:" + itcPartCreate,
		"deployed:create",
		"deploy:into:" + itcPartInto,
		"deployed:into",
		"deploy:s0:" + itcPartS0,
		"deployed:s0",
		`send:SupportBean:{"intPrimitive":50,"theString":"E1"}`,
		`send:SupportBean:{"intPrimitive":20,"theString":"E2"}`,
		`send:SupportBean:{"intPrimitive":60,"theString":"E1"}`,
		`send:SupportBean_S0:{"id":0,"p00":"E1"}`,
		`send:SupportBean_S0:{"id":0,"p00":"E2"}`,
		"undeploy-all",
	}
}

// itcNonOverlappingCaseSteps renders the pinned step sequence of
// InfraNonOverlapping.run (lines 54-83): the context, create-table,
// into-table and s0 deploys, the first SupportBean batch, the S0(-1)
// terminator, the mid-run index and deploy-only join, the second
// SupportBean batch, the second S0(-1) terminator and undeployAll.
func itcNonOverlappingCaseSteps() []string {
	return []string{
		"deploy:ctx:" + itcNonOverlapCtx,
		"deployed:ctx",
		"deploy:create:" + itcNonOverlapCreate,
		"deployed:create",
		"deploy:into:" + itcNonOverlapInto,
		"deployed:into",
		"deploy:s0:" + itcNonOverlapS0,
		"deployed:s0",
		`send:SupportBean:{"intPrimitive":50,"theString":"E1"}`,
		`send:SupportBean:{"intPrimitive":20,"theString":"E2"}`,
		`send:SupportBean:{"intPrimitive":60,"theString":"E1"}`,
		`send:SupportBean_S0:{"id":-1}`,
		"deploy:index:" + itcNonOverlapIndex,
		"deployed:index",
		"deploy:join:" + itcNonOverlapJoin,
		"deployed:join",
		`send:SupportBean:{"intPrimitive":90,"theString":"E3"}`,
		`send:SupportBean:{"intPrimitive":30,"theString":"E1"}`,
		`send:SupportBean:{"intPrimitive":10,"theString":"E3"}`,
		`send:SupportBean_S0:{"id":-1}`,
		"undeploy-all",
	}
}

// itcInvalidCaseSteps renders the pinned step sequence of
// InfraTableContextInvalid.run (lines 37-48): the SimpleCtx and create-table
// deploys, the three tryInvalidCompile probes with their pinned Java
// prefixes, and undeployAll.
func itcInvalidCaseSteps() []string {
	return []string{
		"deploy:ctx:" + itcInvalidCtx,
		"deployed:ctx",
		"deploy:create:" + itcInvalidCreate,
		"deployed:create",
		"build-error:select-table:" + itcProbeSelect + ":" + itcErrTableVisibility,
		"build-error:subquery-table:" + itcProbeSubquery + ":" + itcErrSubquery,
		"build-error:insert-table:" + itcProbeInsert + ":" + itcErrTableVisibility,
		"undeploy-all",
	}
}

// infraTableContextCaseSteps pins the exact op sequence per case.
var infraTableContextCaseSteps = map[string][]string{
	"context-partitioned":    itcPartitionedCaseSteps(),
	"context-nonoverlapping": itcNonOverlappingCaseSteps(),
	"context-invalid":        itcInvalidCaseSteps(),
}

func requireInfraTableContextFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraTableContextID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraTableContextID, name)
		}
	}
	return nil
}

func validateInfraTableContextStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
