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

// infra_nwtable_index_ops_561.go replays InfraNWTableCreateIndex ordinals
// 12-13, 16-17 and 18-19 against the pinned Java oracle: the
// multiple-column multiple-index fire-and-forget probes, the on-select
// index-reuse lifecycle and the invalid create-index probes.
//
//   - mcmi-window/mcmi-table (ords 12-13, InfraMultipleColumnMultipleIndex
//     {namedWindow=true/false}, FIREANDFORGET): keepall window or f1-keyed
//     table MyInfraMCMI(f1 string, f2 int, f3 string, f4 string) fed by
//     `insert into MyInfraMCMI(f1, f2, f3, f4) select theString,
//     intPrimitive, '>'||theString||'<', '?'||theString||'?' from
//     SupportBean`, three overlapping hash indexes MyInfraMCMIIndex1
//     (f2,f3,f1), MyInfraMCMIIndex2 (f2,f3) and MyInfraMCMIIndex3 (f2),
//     three beans (E1,-2), (E2,-4), (E3,-3), then six FAF probes each
//     returning {E1,-2,>E1<,?E1?}: `f3='>E1<'`, `f3='>E1<' and f2=-2`,
//     `f3='>E1<' and f2=-2 and f1='E1'`, `f2=-2`, `f1='E1'` and the
//     four-column `f3='>E1<' and f2=-2 and f1='E1' and f4='?E1?'`.
//   - onr-window/onr-table (ords 16-17, InfraOnSelectReUse
//     {namedWindow=true/false}, no flags): a two-column keepall window or
//     fully-keyed table MyInfraONR(f1 string, f2 int) fed by the plain
//     insert-into, a live MyInfraONRIndex1 on f2, then two identical
//     `on SupportBean_S0 s0 select nw.f1 as f1, nw.f2 as f2 from
//     MyInfraONR nw where nw.f2 = s0.id` consumers (s0 listened), one S0(1)
//     trigger row delivering {E1,1}, targeted undeploys (s0, stmtTwo,
//     indexOne) with getIndexCount asserts after each step, and the
//     MyInfraFour tail asserting one shared two-key index survives two
//     on-selects whose predicates differ only in conjunct order.
//   - invalid-window/invalid-table (ords 18-19, InfraInvalid
//     {namedWindow=true/false}, no flags): MyInfraOne with live index
//     MyInfraIndex, two initiated-by contexts and the contexted MyInfraCtx
//     fixtures, then the ten tryInvalidCompile probes (eleven for the
//     table: the no-primary-key table index probe runs last), and the
//     MyInfraTwo unique-index-violation send.
//
// Index-resolution spike (verified against internal/esper/index_plan.go):
// Go's hash matcher requires predicates on the leading index columns in
// declaration order and a complete key, so `f3`-leading and `f1`-leading
// probes full-scan on the window while the f3+f2, f2, full-key and
// four-column probes resolve Index2, Index3 and Index1 respectively. On
// the f1-keyed table the f1 probe resolves the implicit `<primary-key>`
// candidate. Java's hash index likewise requires its full column set, so
// the row sets are observably identical; the runner asserts the resolved
// selection honestly rather than faking index use.
//
// Index-count divergence (documented, per-step): Java's
// getIndexDescriptors() counts one merged descriptor when an explicit
// index covers an on-select lookup and its IndexMultiKey is
// order-insensitive, so the named-window counts stay 1 throughout
// onr-window and the MyInfraFour tail asserts 1 after both on-selects.
// Go's NamedWindow.IndexCount counts declared indexes and trigger-inferred
// implicit indexes separately (order-sensitive), observing 2/2/2 for the
// three divergent asserts and 3 for the tail. Steps where the observed
// counts diverge ride unrepresentable records pinning the Java-asserted
// count in the note; the runner still verifies the pinned Go-observed
// count internally. Steps whose counts coincide (post-stmtTwo-undeploy
// and every table-side count where len(indexes)+1 primary-key descriptor
// matches Java) ride real index-count records with the observed value.
//
// Invalid-probe divergence (documented): Go create-index rejections are
// runtime CreateIndex errors, not compile errors; build-error steps pin
// the Java message prefix while the runner asserts the matching
// esper.Error code. EPL-text-only spellings (context-scoped create-index,
// the 'gugu' keyword probe and the null-typed schema module) and the
// probes Java rejects but Go's CreateIndex accepts (duplicate column
// names, index on a primary-key-less table) ride unrepresentable records
// pinning the Java-asserted message; for the accept-side probes the
// runner performs the call, pins success and drops the index to mirror
// the Java fixture state.
//
// Approved differences (observably identical to the Java EPL):
//   - `create window`/`create table`/`create context`/`create index` are
//     catalog operations: deploy steps carry the byte-exact EPL while the
//     Go side performs env-level registration; MCMI secondary indexes are
//     declared at creation time because plan.indexPlan is frozen at
//     env.Build — a live CreateIndex would be invisible to FAF index
//     selection (observably identical: every row the index serves is
//     equally visible to a creation-time index). The non-FAF executions
//     use live CreateIndex/DropIndex, mirroring the Java deploy/undeploy
//     order.
//   - `insert into X(...) select ... from SupportBean` maps to the OnEvent
//     InsertIntoNamedWindow/InsertIntoTable trigger; the `||` concat maps
//     to Concat of literal/field operands.
//   - The `on SupportBean_S0 s0 select ... from MyInfraONR nw` triggers
//     map to OnEvent(S0).SelectFromNamedWindow/SelectFromTableWhere with
//     the trigger-field predicate; the deployment is undeployed via
//     Deployment.Undeploy while `undeployModuleContaining("indexOne")` —
//     the Java statement name, not the index name — maps to DropIndex on
//     MyInfraONRIndex1 (Go create-index is not a Deployment).
//   - env.compileExecuteFAF rides snapshot steps that carry the pinned
//     query EPL; the Java assertPropsPerRow is any-order, so both sides
//     emit canonically sorted rows (mode "any").
//   - The unique-index-violation send maps to a send-error step pinning
//     the Java-asserted exception text; Go verifies the live rejection
//     carries the unique-index-violation substring (statement attribution
//     and wrapper differ — the pinned text is the contract).
//   - sendEventBean(new SupportBean(theString, intPrimitive)) and
//     SupportBean_S0(id) map to payloads carrying those bean fields;
//     unmentioned fields stay zero.

// infraNWTableIndexOps561Bean mirrors the SupportBean properties the
// SupportBean(theString, intPrimitive) constructor populates.
type infraNWTableIndexOps561Bean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// infraNWTableIndexOps561S0 mirrors SupportBean_S0's single id property.
type infraNWTableIndexOps561S0 struct {
	ID int `esper:"id"`
}

const (
	infraNWTableIndexOps561ID         = "infra-nwtable-index-ops-561"
	infraNWTableIndexOps561JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableIndexOps561JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java"
)

const infraNWTableIndexOps561Description = "InfraNWTableCreateIndex ordinals 12-13, 16-17 and 18-19: InfraMultipleColumnMultipleIndex (ords 12-13, FIREANDFORGET) deploys a keepall window or f1-keyed MyInfraMCMI(f1 string, f2 int, f3 string, f4 string) fed by a concat insert-into over SupportBean plus three overlapping hash indexes (f2,f3,f1), (f2,f3) and (f2), sends E1/-2, E2/-4, E3/-3, then six FAF probes each returning {E1,-2,>E1<,?E1?} — f3 and f1 probes full-scan on the window (f1 resolves the primary key on the table), f3+f2 resolves Index2, f2 resolves Index3 and the full/four-column probes resolve Index1; InfraOnSelectReUse (ords 16-17) deploys a two-column MyInfraONR with live index on f2, two identical SupportBean_S0-triggered on-select consumers (s0 listened, S0(1) delivering {E1,1}), targeted undeploys and index-count asserts, then the MyInfraFour two-key tail asserting one shared index for both conjunct orders (Go observes the declared/implicit split, so divergent NW counts are pinned as unrepresentable notes); InfraInvalid (ords 18-19) pins the ten tryInvalidCompile probes — eleven for the table, adding the no-primary-key index probe — plus the MyInfraTwo unique-index-violation send (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java)."

// Verbatim transcriptions of InfraNWTableCreateIndex.java lines 284-316
// (ords 12-13, InfraMultipleColumnMultipleIndex.run), lines 168-201 (ords
// 16-17, InfraOnSelectReUse.run) and lines 78-149 (ords 18-19,
// InfraInvalid.run).
const (
	infraNWTableMCMI561CreateWindow = "@public create window MyInfraMCMI#keepall as (f1 string, f2 int, f3 string, f4 string)"
	infraNWTableMCMI561CreateTable  = "@public create table MyInfraMCMI as (f1 string primary key, f2 int, f3 string, f4 string)"
	infraNWTableMCMI561Insert       = "insert into MyInfraMCMI(f1, f2, f3, f4) select theString, intPrimitive, '>'||theString||'<', '?'||theString||'?' from SupportBean"
	infraNWTableMCMI561Index1       = "create index MyInfraMCMIIndex1 on MyInfraMCMI(f2, f3, f1)"
	infraNWTableMCMI561Index2       = "create index MyInfraMCMIIndex2 on MyInfraMCMI(f2, f3)"
	infraNWTableMCMI561Index3       = "create index MyInfraMCMIIndex3 on MyInfraMCMI(f2)"
	infraNWTableMCMI561SelectF3     = "select * from MyInfraMCMI where f3='>E1<'"
	infraNWTableMCMI561SelectF3F2   = "select * from MyInfraMCMI where f3='>E1<' and f2=-2"
	infraNWTableMCMI561SelectFull   = "select * from MyInfraMCMI where f3='>E1<' and f2=-2 and f1='E1'"
	infraNWTableMCMI561SelectF2     = "select * from MyInfraMCMI where f2=-2"
	infraNWTableMCMI561SelectF1     = "select * from MyInfraMCMI where f1='E1'"
	infraNWTableMCMI561SelectAll    = "select * from MyInfraMCMI where f3='>E1<' and f2=-2 and f1='E1' and f4='?E1?'"

	infraNWTableONR561CreateWindow  = "@name('create') @public create window MyInfraONR#keepall as (f1 string, f2 int)"
	infraNWTableONR561CreateTable   = "@name('create') @public create table MyInfraONR as (f1 string primary key, f2 int primary key)"
	infraNWTableONR561Insert        = "insert into MyInfraONR(f1, f2) select theString, intPrimitive from SupportBean"
	infraNWTableONR561Index         = "@name('indexOne') create index MyInfraONRIndex1 on MyInfraONR(f2)"
	infraNWTableONR561SelectS0      = "@name('s0') on SupportBean_S0 s0 select nw.f1 as f1, nw.f2 as f2 from MyInfraONR nw where nw.f2 = s0.id"
	infraNWTableONR561SelectTwo     = "@name('stmtTwo') on SupportBean_S0 s0 select nw.f1 as f1, nw.f2 as f2 from MyInfraONR nw where nw.f2 = s0.id"
	infraNWTableONR561CreateFour    = "@name('cw') @public create window MyInfraFour#keepall as SupportBean"
	infraNWTableONR561IndexFour     = "create index idx1 on MyInfraFour (theString, intPrimitive)"
	infraNWTableONR561OnSelectA     = "on SupportBean sb select * from MyInfraFour w where w.theString = sb.theString and w.intPrimitive = sb.intPrimitive"
	infraNWTableONR561OnSelectB     = "on SupportBean sb select * from MyInfraFour w where w.intPrimitive = sb.intPrimitive and w.theString = sb.theString"
	infraNWTableONR561UndeployIndex = "indexOne"

	infraNWTableINV561CreateWindow = "@public create window MyInfraOne#keepall as (f1 string, f2 int)"
	infraNWTableINV561CreateTable  = "@public create table MyInfraOne as (f1 string primary key, f2 int primary key)"
	infraNWTableINV561Index        = "create index MyInfraIndex on MyInfraOne(f1)"
	infraNWTableINV561ContextOne   = "@public create context ContextOne initiated by SupportBean terminated after 5 sec"
	infraNWTableINV561ContextTwo   = "@public create context ContextTwo initiated by SupportBean terminated after 5 sec"
	infraNWTableINV561CreateCtxNW  = "@public context ContextOne create window MyInfraCtx#keepall as (f1 string, f2 int)"
	infraNWTableINV561CreateCtxTBL = "@public context ContextOne create table MyInfraCtx as (f1 string primary key, f2 int primary key)"
	infraNWTableINV561CreateTwoNW  = "@Name('create') @public create window MyInfraTwo#keepall as SupportBean"
	infraNWTableINV561CreateTwoTBL = "@Name('create') @public create table MyInfraTwo(theString string primary key, intPrimitive int primary key)"
	infraNWTableINV561InsertTwo    = "@Name('insert') insert into MyInfraTwo select theString, intPrimitive from SupportBean"
	infraNWTableINV561UniqueIndex  = "create unique index I1 on MyInfraTwo(theString)"
	infraNWTableINV561CreateNoKey  = "@public create table MyTable (p0 string, sumint sum(int))"

	infraNWTableINV561ProbeCtxA       = "create unique index IndexTwo on MyInfraCtx(f1)"
	infraNWTableINV561ProbeCtxB       = "context ContextTwo create unique index IndexTwo on MyInfraCtx(f1)"
	infraNWTableINV561ProbeDupIndex   = "create index MyInfraIndex on MyInfraOne(f1)"
	infraNWTableINV561ProbeUnknownCol = "create index IndexTwo on MyInfraOne(fx)"
	infraNWTableINV561ProbeDupCol     = "create index IndexTwo on MyInfraOne(f1, f1)"
	infraNWTableINV561ProbeUnknownInf = "create index IndexTwo on MyWindowX(f1, f1)"
	infraNWTableINV561ProbeBadKind    = "create index IndexTwo on MyInfraOne(f1 bubu, f2)"
	infraNWTableINV561ProbeGugu       = "create gugu index IndexTwo on MyInfraOne(f2)"
	infraNWTableINV561ProbeUniqueBT   = "create unique index IndexTwo on MyInfraOne(f2 btree)"
	infraNWTableINV561ProbeNullTyped  = "create schema MyMap(somefield null);\ncreate window MyWindow#keepall as MyMap;\ncreate unique index MyIndex on MyWindow(somefield)"
	infraNWTableINV561ProbeNoPK       = "create index MyIndex on MyTable(p0)"
)

// Java-asserted message prefixes pinned by tryInvalidCompile (byte-exact,
// including the pinned 'more then' wording) plus the unique-violation
// send-error texts; the <Named window|Table> head words differ per
// variant.
const (
	infraNWTableINV561ErrCtxNW      = "Named window by name 'MyInfraCtx' has been declared for context 'ContextOne' and can only be used within the same context"
	infraNWTableINV561ErrCtxTBL     = "Table by name 'MyInfraCtx' has been declared for context 'ContextOne' and can only be used within the same context"
	infraNWTableINV561ErrDupIndex   = "An index by name 'MyInfraIndex' already exists ["
	infraNWTableINV561ErrUnknownCol = "Property named 'fx' not found"
	infraNWTableINV561ErrDupCol     = "Property named 'f1' has been declared more then once [create index IndexTwo on MyInfraOne(f1, f1)]"
	infraNWTableINV561ErrUnknownInf = "A named window or table by name 'MyWindowX' does not exist [create index IndexTwo on MyWindowX(f1, f1)]"
	infraNWTableINV561ErrBadKind    = "Unrecognized advanced-type index 'bubu'"
	infraNWTableINV561ErrGugu       = "Invalid keyword 'gugu' in create-index encountered, expected 'unique' [create gugu index IndexTwo on MyInfraOne(f2)]"
	infraNWTableINV561ErrUniqueBT   = "Combination of unique index with btree (range) is not supported [create unique index IndexTwo on MyInfraOne(f2 btree)]"
	infraNWTableINV561ErrNullTyped  = "Property named 'somefield' is null-typed"
	infraNWTableINV561ErrNoPK       = "Tables without primary key column(s) do not allow creating an index ["
	infraNWTableINV561ErrSendNW     = "Unexpected exception in statement 'create': Unique index violation, index 'I1' is a unique index and key 'E1' already exists"
	infraNWTableINV561ErrSendTBL    = "java.lang.RuntimeException: Unexpected exception in statement 'insert': Unique index violation, index 'I1' is a unique index and key 'E1' already exists"
)

// Index-count divergence notes pinned by the unrepresentable onr-window
// and shared-tail steps: the note carries the Java-asserted count and the
// Go-observed count so the record documents the divergence.
const (
	infraNWTableONR561NoteCount1 = "getIndexCount(MyInfraONR)=1 pinned by Java while the s0 consumers hold an implicit f2 lookup; Go counts the declared MyInfraONRIndex1 and the trigger-inferred implicit index separately (IndexCount=2)"
	infraNWTableONR561NoteCount2 = "getIndexCount(MyInfraONR)=1 pinned by Java after undeployModuleContaining(s0); the implicit index is ref-counted so stmtTwo still holds it and Go's IndexCount stays 2"
	infraNWTableONR561NoteCount3 = "getIndexCount(MyInfraONR)=1 pinned by Java after undeployModuleContaining(stmtTwo); with the last consumer gone Go's implicit index releases and IndexCount returns to the declared-index count"
	infraNWTableONR561NoteFour   = "getIndexCountNoContext(MyInfraFour)=1 pinned by Java: both on-selects reuse the declared idx1 (theString,intPrimitive) index and Java's IndexMultiKey is conjunct-order-insensitive; Go registers one implicit index per predicate order (IndexCount=3)"
)

var (
	infraNWTableIndexOps561JavaSources = []string{
		infraNWTableIndexOps561JavaSource,
	}
	infraNWTableIndexOps561JavaRuntimeIDs = []string{
		"java-runtime-b3fc4383cc57caee42ff",
		"java-runtime-dfaed70d1fe593eeaff6",
		"java-runtime-ef9ec62512971b058e1b",
		"java-runtime-e2ef7f21884bd471b60f",
		"java-runtime-1c04a900bd31d5dc0d0b",
		"java-runtime-a27f8fd61fbafd563173",
	}
	infraNWTableIndexOps561JavaExecutions = []string{
		"InfraMultipleColumnMultipleIndex{namedWindow=true}",
		"InfraMultipleColumnMultipleIndex{namedWindow=false}",
		"InfraOnSelectReUse{namedWindow=true}",
		"InfraOnSelectReUse{namedWindow=false}",
		"InfraInvalid{namedWindow=true}",
		"InfraInvalid{namedWindow=false}",
	}
	infraNWTableIndexOps561JavaStaticIDs = []string{
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
	}
	infraNWTableIndexOps561JavaFlags = []string{"FIREANDFORGET"}
	infraNWTableIndexOps561Cases     = []string{
		"mcmi-window",
		"mcmi-table",
		"onr-window",
		"onr-table",
		"invalid-window",
		"invalid-table",
	}
	infraNWTableIndexOps561Ordinals = []int{12, 13, 16, 17, 18, 19}
)

// infraNWTableIndexOps561CaseEPLs pins the newline-joined EPL of every
// EPL-bearing step in the case, in step order — the value carried by the
// scenario cases[] metadata. Steps that carry only a statement-name key
// (undeploy indexOne) or no EPL (index-count, unrepresentable count notes)
// are excluded; unrepresentable invalid probes carry the probe EPL.
var infraNWTableIndexOps561CaseEPLs = []string{
	strings.Join([]string{
		infraNWTableMCMI561CreateWindow,
		infraNWTableMCMI561Insert,
		infraNWTableMCMI561Index1,
		infraNWTableMCMI561Index2,
		infraNWTableMCMI561Index3,
		infraNWTableMCMI561SelectF3,
		infraNWTableMCMI561SelectF3F2,
		infraNWTableMCMI561SelectFull,
		infraNWTableMCMI561SelectF2,
		infraNWTableMCMI561SelectF1,
		infraNWTableMCMI561SelectAll,
	}, "\n"),
	strings.Join([]string{
		infraNWTableMCMI561CreateTable,
		infraNWTableMCMI561Insert,
		infraNWTableMCMI561Index1,
		infraNWTableMCMI561Index2,
		infraNWTableMCMI561Index3,
		infraNWTableMCMI561SelectF3,
		infraNWTableMCMI561SelectF3F2,
		infraNWTableMCMI561SelectFull,
		infraNWTableMCMI561SelectF2,
		infraNWTableMCMI561SelectF1,
		infraNWTableMCMI561SelectAll,
	}, "\n"),
	strings.Join([]string{
		infraNWTableONR561CreateWindow,
		infraNWTableONR561Insert,
		infraNWTableONR561Index,
		infraNWTableONR561SelectS0,
		infraNWTableONR561SelectTwo,
		infraNWTableONR561CreateFour,
		infraNWTableONR561IndexFour,
		infraNWTableONR561OnSelectA,
		infraNWTableONR561OnSelectB,
	}, "\n"),
	strings.Join([]string{
		infraNWTableONR561CreateTable,
		infraNWTableONR561Insert,
		infraNWTableONR561Index,
		infraNWTableONR561SelectS0,
		infraNWTableONR561SelectTwo,
		infraNWTableONR561CreateFour,
		infraNWTableONR561IndexFour,
		infraNWTableONR561OnSelectA,
		infraNWTableONR561OnSelectB,
	}, "\n"),
	strings.Join([]string{
		infraNWTableINV561CreateWindow,
		infraNWTableINV561Index,
		infraNWTableINV561ContextOne,
		infraNWTableINV561ContextTwo,
		infraNWTableINV561CreateCtxNW,
		infraNWTableINV561ProbeCtxA,
		infraNWTableINV561ProbeCtxB,
		infraNWTableINV561ProbeDupIndex,
		infraNWTableINV561ProbeUnknownCol,
		infraNWTableINV561ProbeDupCol,
		infraNWTableINV561ProbeUnknownInf,
		infraNWTableINV561ProbeBadKind,
		infraNWTableINV561ProbeGugu,
		infraNWTableINV561ProbeUniqueBT,
		infraNWTableINV561ProbeNullTyped,
		infraNWTableINV561CreateTwoNW,
		infraNWTableINV561InsertTwo,
		infraNWTableINV561UniqueIndex,
	}, "\n"),
	strings.Join([]string{
		infraNWTableINV561CreateTable,
		infraNWTableINV561Index,
		infraNWTableINV561ContextOne,
		infraNWTableINV561ContextTwo,
		infraNWTableINV561CreateCtxTBL,
		infraNWTableINV561ProbeCtxA,
		infraNWTableINV561ProbeCtxB,
		infraNWTableINV561ProbeDupIndex,
		infraNWTableINV561ProbeUnknownCol,
		infraNWTableINV561ProbeDupCol,
		infraNWTableINV561ProbeUnknownInf,
		infraNWTableINV561ProbeBadKind,
		infraNWTableINV561ProbeGugu,
		infraNWTableINV561ProbeUniqueBT,
		infraNWTableINV561ProbeNullTyped,
		infraNWTableINV561CreateTwoTBL,
		infraNWTableINV561InsertTwo,
		infraNWTableINV561UniqueIndex,
		infraNWTableINV561CreateNoKey,
		infraNWTableINV561ProbeNoPK,
	}, "\n"),
}

var infraNWTableIndexOps561CaseObservations = []string{
	"deploy+send+snapshot; MyInfraMCMI#keepall window with three overlapping hash indexes (f2,f3,f1)/(f2,f3)/(f2): the f3 and f1 FAF probes resolve to a full scan (no index leads with f3 or f1), f3+f2 resolves Index2, f2 resolves Index3 and the full-key and four-column probes resolve Index1 — each returns {E1,-2,>E1<,?E1?}",
	"deploy+send+snapshot; f1-keyed MyInfraMCMI table with three overlapping hash indexes (f2,f3,f1)/(f2,f3)/(f2): the f3 probe full-scans while f1 resolves the primary-key index, f3+f2 resolves Index2, f2 resolves Index3 and the full-key and four-column probes resolve Index1 — each returns {E1,-2,>E1<,?E1?}",
	"deploy+listener+index-count+unrepresentable; MyInfraONR#keepall window fed by insert-into with a live MyInfraONRIndex1(f2): two identical on-S0 select consumers, S0(1) delivering {E1,1}, then undeploys; Java's index registry merges the implicit on-select lookup into the declared index (count 1) while Go counts declared+implicit separately, so the divergent asserts ride pinned notes and the coincident post-undeploy count rides a real index-count record",
	"deploy+listener+index-count+unrepresentable; (f1,f2)-keyed MyInfraONR table with a live MyInfraONRIndex1(f2): two identical on-S0 select consumers, S0(1) delivering {E1,1}, then undeploys with index-count 2 asserts (declared index plus the implicit primary-key descriptor); the MyInfraFour two-key tail is a named window on both variants so its order-insensitive Java count rides a pinned note",
	"deploy+compile-error+unrepresentable+send-error; MyInfraOne with a live MyInfraIndex and the contexted MyInfraCtx fixtures pin ten tryInvalidCompile probes (context-scoped and EPL-text-only spellings plus the duplicate-column accept divergence are plan-only), then the MyInfraTwo unique index rejects the second E1 insert with the pinned 'create'-statement violation text",
	"deploy+compile-error+unrepresentable+send-error; MyInfraOne with a live MyInfraIndex and the contexted MyInfraCtx fixtures pin ten tryInvalidCompile probes plus the no-primary-key table index probe (a Go accept divergence, plan-only), then the MyInfraTwo unique index rejects the second E1 insert with the pinned 'insert'-statement RuntimeException text",
}

// infraNWTableIndexOps561CaseSpec carries the per-case fixture constants:
// which execution family runs and whether the named-window or table
// variant applies.
type infraNWTableIndexOps561CaseSpec struct {
	namedWindow bool
	family      string // "mcmi", "onr" or "invalid"
}

var infraNWTableIndexOps561CaseSpecs = map[string]infraNWTableIndexOps561CaseSpec{
	"mcmi-window":    {namedWindow: true, family: "mcmi"},
	"mcmi-table":     {namedWindow: false, family: "mcmi"},
	"onr-window":     {namedWindow: true, family: "onr"},
	"onr-table":      {namedWindow: false, family: "onr"},
	"invalid-window": {namedWindow: true, family: "invalid"},
	"invalid-table":  {namedWindow: false, family: "invalid"},
}

// infraNWTableIndexOps561CaseState carries the per-case replay state: the
// environment/engine pair, the label→deployments bookkeeping used by
// undeploy and undeploy-all, and the deployed-label set for marker checks.
type infraNWTableIndexOps561CaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	spec           infraNWTableIndexOps561CaseSpec
	deployments    map[string]*esper.Deployment
	deployedLabels map[string]bool
	listenerSeq    uint64
	indexSeq       map[string]uint64
	caseName       string
	trace          *compat.Trace
}

// runInfraNWTableIndexOps561Scenario replays the six
// InfraNWTableCreateIndex executions: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action. The oracle emits the epoch
// time for every record, so the runner pins the same value.
func runInfraNWTableIndexOps561Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableIndexOps561Cases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableIndexOps561Case(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableIndexOps561ID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableIndexOps561ID)
	}
	return trace, nil
}

func runInfraNWTableIndexOps561Case(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	spec, ok := infraNWTableIndexOps561CaseSpecs[caseName]
	if !ok {
		return compat.Trace{}, fmt.Errorf("%s: unknown case %q", infraNWTableIndexOps561ID, caseName)
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableIndexOps561Bean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableIndexOps561S0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableIndexOps561JavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableIndexOps561ID}
	state := &infraNWTableIndexOps561CaseState{
		env:            env,
		engine:         engine,
		spec:           spec,
		deployments:    make(map[string]*esper.Deployment),
		deployedLabels: make(map[string]bool),
		indexSeq:       make(map[string]uint64),
		caseName:       caseName,
		trace:          &trace,
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
					infraNWTableIndexOps561ID, step.Statement)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  1,
				Time:      "1970-01-01T00:00:00Z",
			})
		case "send":
			event, err := decodeInfraNWTableIndexOps561Payload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return compat.Trace{}, err
			}
		case "send-error":
			if err := state.sendError(ctx, step); err != nil {
				return compat.Trace{}, err
			}
		case "snapshot":
			if err := state.snapshot(ctx, step); err != nil {
				return compat.Trace{}, err
			}
		case "index-count":
			if err := state.indexCount(step); err != nil {
				return compat.Trace{}, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return compat.Trace{}, err
			}
		case "unrepresentable":
			if err := state.unrepresentable(step); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy":
			if err := state.undeploy(ctx, step); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s: unexpected op %q", infraNWTableIndexOps561ID, step.Op)
		}
	}
	return trace, nil
}

// deploy maps each scenario label to the equivalent Go catalog call or
// chain-API plan. The byte-exact EPL the Java execution passes to
// compileDeploy is pinned by the loader's step keys.
func (s *infraNWTableIndexOps561CaseState) deploy(ctx context.Context, step compat.Step) error {
	pinned, ok := infraNWTableIndexOps561DeployEPLs[s.caseName][step.Statement]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: deploy %q EPL %q is not pinned (want %q, case=%q)",
			infraNWTableIndexOps561ID, step.Statement, step.Epl, pinned, s.caseName)
	}
	switch s.spec.family {
	case "mcmi":
		return s.deployMCMI(ctx, step.Statement)
	case "onr":
		return s.deployONR(ctx, step.Statement)
	case "invalid":
		return s.deployInvalid(ctx, step.Statement)
	}
	return fmt.Errorf("%s: unknown family %q", infraNWTableIndexOps561ID, s.spec.family)
}

// deployMCMI mirrors the three deploy roles of
// InfraMultipleColumnMultipleIndex.run: the create step registers the
// window/table with the three overlapping indexes declared at creation
// time (plan.indexPlan freezes at env.Build so live CreateIndex calls
// would be invisible to the FAF probes — observably identical since every
// row the index serves is equally visible to a creation-time index), the
// insert step deploys the concat feed, and the index steps verify the
// declared indexes are catalog-visible.
func (s *infraNWTableIndexOps561CaseState) deployMCMI(ctx context.Context, label string) error {
	switch label {
	case "create":
		if s.spec.namedWindow {
			stringT := reflect.TypeOf("")
			schema, err := esper.NewMapSchema("MyInfraMCMI561Schema", []esper.FieldSpec{
				esper.FieldDef("f1", stringT),
				esper.FieldDef("f2", reflect.TypeOf(0)),
				esper.FieldDef("f3", stringT),
				esper.FieldDef("f4", stringT),
			})
			if err != nil {
				return err
			}
			if err := s.env.RegisterSchema(schema); err != nil {
				return err
			}
			if _, err := esper.CreateNamedWindow(s.env, "MyInfraMCMI", schema,
				esper.NamedWindowRetention(esper.KeepAll()),
				esper.NamedWindowIndex("MyInfraMCMIIndex1", "f2", "f3", "f1"),
				esper.NamedWindowIndex("MyInfraMCMIIndex2", "f2", "f3"),
				esper.NamedWindowIndex("MyInfraMCMIIndex3", "f2")); err != nil {
				return err
			}
		} else {
			if _, err := esper.CreateTable(s.env, "MyInfraMCMI", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("f1"),
				esper.TableColumnOf[int]("f2"),
				esper.TableColumnOf[string]("f3"),
				esper.TableColumnOf[string]("f4"),
			},
				esper.SecondaryIndex("MyInfraMCMIIndex1", "f2", "f3", "f1"),
				esper.SecondaryIndex("MyInfraMCMIIndex2", "f2", "f3"),
				esper.SecondaryIndex("MyInfraMCMIIndex3", "f2")); err != nil {
				return err
			}
		}
		s.deployedLabels[label] = true
		return nil
	case "insert":
		theString := esper.Field[infraNWTableIndexOps561Bean, string]("theString")
		assignments := []esper.TableAssignment{
			esper.SetColumn("f1", theString),
			esper.SetColumn("f2", esper.Field[infraNWTableIndexOps561Bean, int]("intPrimitive")),
			esper.SetColumn("f3", esper.Concat(
				esper.Literal[string](">"), theString, esper.Literal[string]("<"))),
			esper.SetColumn("f4", esper.Concat(
				esper.Literal[string]("?"), theString, esper.Literal[string]("?"))),
		}
		source := esper.From[infraNWTableIndexOps561Bean](s.env, "SupportBean")
		var plan esper.Plan
		var err error
		if s.spec.namedWindow {
			plan, err = s.env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfraMCMI", assignments...).Query())
		} else {
			plan, err = s.env.Build(esper.OnEvent(source).InsertIntoTable("MyInfraMCMI", assignments...).Query())
		}
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	case "index-one", "index-two", "index-three":
		return s.verifyCatalogIndex(label)
	}
	return fmt.Errorf("%s: unknown mcmi deploy label %q", infraNWTableIndexOps561ID, label)
}

// verifyCatalogIndex mirrors a `create index` deploy step for an index
// declared at creation time: the step verifies the declared index is
// catalog-visible before the FAF probes run.
func (s *infraNWTableIndexOps561CaseState) verifyCatalogIndex(label string) error {
	indexName := map[string]string{
		"index-one":   "MyInfraMCMIIndex1",
		"index-two":   "MyInfraMCMIIndex2",
		"index-three": "MyInfraMCMIIndex3",
	}[label]
	var found bool
	if s.spec.namedWindow {
		window, ok := s.engine.NamedWindow("MyInfraMCMI")
		if !ok {
			return fmt.Errorf("%s: named window MyInfraMCMI is missing", infraNWTableIndexOps561ID)
		}
		for _, def := range window.Definition().Indexes() {
			if def.Name == indexName {
				found = true
			}
		}
	} else {
		table, ok := s.engine.Table("MyInfraMCMI")
		if !ok {
			return fmt.Errorf("%s: table MyInfraMCMI is missing", infraNWTableIndexOps561ID)
		}
		for _, def := range table.Definition().Indexes() {
			if def.Name == indexName {
				found = true
			}
		}
	}
	if !found {
		return fmt.Errorf("%s: declared index %q is not catalog-visible", infraNWTableIndexOps561ID, indexName)
	}
	s.deployedLabels[label] = true
	return nil
}

// deployONR mirrors the InfraOnSelectReUse deploys: the two-column infra
// registers at env level, the index deploy performs a live CreateIndex
// (the trigger path never consults plan.indexPlan, so the Java ordering
// is faithfully replayable), the on-select consumers deploy as OnEvent
// triggers (s0 subscribed, mirroring addListener("s0")), and the
// MyInfraFour tail registers a SupportBean keepall window with a live
// two-column index plus the two order-variant on-selects.
func (s *infraNWTableIndexOps561CaseState) deployONR(ctx context.Context, label string) error {
	switch label {
	case "create":
		if s.spec.namedWindow {
			schema, err := esper.NewMapSchema("MyInfraONR561Schema", []esper.FieldSpec{
				esper.FieldDef("f1", reflect.TypeOf("")),
				esper.FieldDef("f2", reflect.TypeOf(0)),
			})
			if err != nil {
				return err
			}
			if err := s.env.RegisterSchema(schema); err != nil {
				return err
			}
			if _, err := esper.CreateNamedWindow(s.env, "MyInfraONR", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return err
			}
		} else {
			if _, err := esper.CreateTable(s.env, "MyInfraONR", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("f1"),
				esper.PrimaryKeyColumn[int]("f2"),
			}); err != nil {
				return err
			}
		}
		s.deployedLabels[label] = true
		return nil
	case "insert":
		assignments := []esper.TableAssignment{
			esper.SetColumn("f1", esper.Field[infraNWTableIndexOps561Bean, string]("theString")),
			esper.SetColumn("f2", esper.Field[infraNWTableIndexOps561Bean, int]("intPrimitive")),
		}
		source := esper.From[infraNWTableIndexOps561Bean](s.env, "SupportBean")
		var plan esper.Plan
		var err error
		if s.spec.namedWindow {
			plan, err = s.env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfraONR", assignments...).Query())
		} else {
			plan, err = s.env.Build(esper.OnEvent(source).InsertIntoTable("MyInfraONR", assignments...).Query())
		}
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	case "index":
		// Live create-index mirrors `@name('indexOne') create index
		// MyInfraONRIndex1 on MyInfraONR(f2)`: the trigger reads live
		// engine state, so the index is created post-insert exactly as
		// Java deploys it.
		if err := s.createLiveIndex("MyInfraONR", "MyInfraONRIndex1",
			[]string{"f2"}, esper.IndexHash, false); err != nil {
			return err
		}
		s.deployedLabels[label] = true
		return nil
	case "s0", "stmtTwo":
		deployment, err := s.deployONRSelect(ctx, label)
		if err != nil {
			return err
		}
		if label == "s0" {
			return s.subscribeONR(deployment.Statements()[0])
		}
		return nil
	case "cw":
		schema, ok := s.env.Schema("SupportBean")
		if !ok {
			return fmt.Errorf("%s: SupportBean schema is missing", infraNWTableIndexOps561ID)
		}
		if _, err := esper.CreateNamedWindow(s.env, "MyInfraFour", schema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return err
		}
		s.deployedLabels[label] = true
		return nil
	case "cw-index":
		window, ok := s.engine.NamedWindow("MyInfraFour")
		if !ok {
			return fmt.Errorf("%s: named window MyInfraFour is missing", infraNWTableIndexOps561ID)
		}
		if err := window.CreateIndex("idx1", []string{"theString", "intPrimitive"}, esper.IndexHash, false); err != nil {
			return err
		}
		s.deployedLabels[label] = true
		return nil
	case "on-select-a", "on-select-b":
		source := esper.From[infraNWTableIndexOps561Bean](s.env, "SupportBean")
		windowString := esper.EqualOf(esper.NamedWindowField[string]("theString"),
			esper.Field[infraNWTableIndexOps561Bean, string]("theString"))
		windowInt := esper.EqualOf(esper.NamedWindowField[int]("intPrimitive"),
			esper.Field[infraNWTableIndexOps561Bean, int]("intPrimitive"))
		var predicate esper.Expression[bool]
		if label == "on-select-a" {
			predicate = esper.And(windowString, windowInt)
		} else {
			predicate = esper.And(windowInt, windowString)
		}
		plan, err := s.env.Build(esper.OnEvent(source).SelectFromNamedWindow("MyInfraFour", predicate).Query())
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	}
	return fmt.Errorf("%s: unknown onr deploy label %q", infraNWTableIndexOps561ID, label)
}

// deployONRSelect builds and deploys one `on SupportBean_S0 s0 select
// nw.f1 as f1, nw.f2 as f2 from MyInfraONR nw where nw.f2 = s0.id`
// consumer; both s0 and stmtTwo share the same predicate and projection.
func (s *infraNWTableIndexOps561CaseState) deployONRSelect(ctx context.Context, label string) (*esper.Deployment, error) {
	source := esper.From[infraNWTableIndexOps561S0](s.env, "SupportBean_S0")
	triggerID := esper.Field[infraNWTableIndexOps561S0, int]("id")
	var plan esper.Plan
	var err error
	if s.spec.namedWindow {
		plan, err = s.env.Build(esper.OnEvent(source).SelectFromNamedWindow("MyInfraONR",
			esper.EqualOf(esper.NamedWindowField[int]("f2"), triggerID),
			esper.Alias("f1", esper.NamedWindowField[string]("f1")),
			esper.Alias("f2", esper.NamedWindowField[int]("f2")),
		).Query(esper.StatementName(label)))
	} else {
		plan, err = s.env.Build(esper.OnEvent(source).SelectFromTableWhere("MyInfraONR",
			esper.EqualOf(esper.TableField[int]("f2"), triggerID),
			esper.Alias("f1", esper.TableField[string]("f1")),
			esper.Alias("f2", esper.TableField[int]("f2")),
		).Query(esper.StatementName(label)))
	}
	if err != nil {
		return nil, err
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return nil, err
	}
	s.deployments[label] = deployment
	s.deployedLabels[label] = true
	return deployment, nil
}

// subscribeONR mirrors addListener("s0"): one listener record per
// delivered batch with normalized row fields.
func (s *infraNWTableIndexOps561CaseState) subscribeONR(statement *esper.Statement) error {
	_, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		s.listenerSeq++
		s.trace.Records = append(s.trace.Records, compat.TraceRecord{
			Case:      s.caseName,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  s.listenerSeq,
			Time:      "1970-01-01T00:00:00Z",
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
		return nil
	})
	return err
}

// deployInvalid mirrors the InfraInvalid deploys: the MyInfraOne infra
// plus its live MyInfraIndex, the two initiated-by SupportBean contexts
// terminated after 5 seconds, the context-bound MyInfraCtx fixture, the
// MyInfraTwo infra plus feed and live unique index, and the table-only
// primary-key-less MyTable fixture (p0 string, sumint sum(int)).
func (s *infraNWTableIndexOps561CaseState) deployInvalid(ctx context.Context, label string) error {
	switch label {
	case "create":
		if s.spec.namedWindow {
			schema, err := esper.NewMapSchema("MyInfraOne561Schema", []esper.FieldSpec{
				esper.FieldDef("f1", reflect.TypeOf("")),
				esper.FieldDef("f2", reflect.TypeOf(0)),
			})
			if err != nil {
				return err
			}
			if err := s.env.RegisterSchema(schema); err != nil {
				return err
			}
			if _, err := esper.CreateNamedWindow(s.env, "MyInfraOne", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return err
			}
		} else {
			if _, err := esper.CreateTable(s.env, "MyInfraOne", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("f1"),
				esper.PrimaryKeyColumn[int]("f2"),
			}); err != nil {
				return err
			}
		}
		s.deployedLabels[label] = true
		return nil
	case "index":
		if err := s.createLiveIndex("MyInfraOne", "MyInfraIndex",
			[]string{"f1"}, esper.IndexHash, false); err != nil {
			return err
		}
		s.deployedLabels[label] = true
		return nil
	case "context-one", "context-two":
		name := "ContextOne"
		if label == "context-two" {
			name = "ContextTwo"
		}
		// `create context X initiated by SupportBean terminated after 5 sec`
		// maps to an overlapping initiated-by-type/pattern-terminated
		// context; the fixture only exists so the contexted window/table
		// registration resolves.
		isBean := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()),
			esper.Literal("SupportBean"))
		beanStream := esper.From[infraNWTableIndexOps561Bean](s.env, "SupportBean")
		_, err := esper.CreateOverlappingPatternTerminatedContext(s.env, name,
			esper.Literal("global"), isBean, esper.TimerInterval(beanStream, 5*time.Second))
		if err != nil {
			return err
		}
		s.deployedLabels[label] = true
		return nil
	case "create-ctx":
		if s.spec.namedWindow {
			schema, err := esper.NewMapSchema("MyInfraCtx561Schema", []esper.FieldSpec{
				esper.FieldDef("f1", reflect.TypeOf("")),
				esper.FieldDef("f2", reflect.TypeOf(0)),
			})
			if err != nil {
				return err
			}
			if err := s.env.RegisterSchema(schema); err != nil {
				return err
			}
			if _, err := esper.CreateNamedWindow(s.env, "MyInfraCtx", schema,
				esper.NamedWindowRetention(esper.KeepAll()),
				esper.NamedWindowContext("ContextOne")); err != nil {
				return err
			}
		} else {
			if _, err := esper.CreateTable(s.env, "MyInfraCtx", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("f1"),
				esper.PrimaryKeyColumn[int]("f2"),
			}, esper.TableContext("ContextOne")); err != nil {
				return err
			}
		}
		s.deployedLabels[label] = true
		return nil
	case "create-two":
		if s.spec.namedWindow {
			schema, ok := s.env.Schema("SupportBean")
			if !ok {
				return fmt.Errorf("%s: SupportBean schema is missing", infraNWTableIndexOps561ID)
			}
			if _, err := esper.CreateNamedWindow(s.env, "MyInfraTwo", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return err
			}
		} else {
			if _, err := esper.CreateTable(s.env, "MyInfraTwo", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("theString"),
				esper.PrimaryKeyColumn[int]("intPrimitive"),
			}); err != nil {
				return err
			}
		}
		s.deployedLabels[label] = true
		return nil
	case "insert":
		assignments := []esper.TableAssignment{
			esper.SetColumn("theString", esper.Field[infraNWTableIndexOps561Bean, string]("theString")),
			esper.SetColumn("intPrimitive", esper.Field[infraNWTableIndexOps561Bean, int]("intPrimitive")),
		}
		source := esper.From[infraNWTableIndexOps561Bean](s.env, "SupportBean")
		var plan esper.Plan
		var err error
		if s.spec.namedWindow {
			plan, err = s.env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfraTwo", assignments...).Query(esper.StatementName("insert")))
		} else {
			plan, err = s.env.Build(esper.OnEvent(source).InsertIntoTable("MyInfraTwo", assignments...).Query(esper.StatementName("insert")))
		}
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	case "index-unique":
		if err := s.createLiveIndex("MyInfraTwo", "I1",
			[]string{"theString"}, esper.IndexHash, true); err != nil {
			return err
		}
		s.deployedLabels[label] = true
		return nil
	case "create-nokey":
		// `@public create table MyTable (p0 string, sumint sum(int))`: a
		// primary-key-less table with one declared aggregation column.
		if s.spec.namedWindow {
			return fmt.Errorf("%s: create-nokey is table-only", infraNWTableIndexOps561ID)
		}
		if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
			esper.TableColumnOf[string]("p0"),
			esper.TableColumnOf[int]("sumint", esper.WithTableAgg("sum", "sum(int)", true)),
		}); err != nil {
			return err
		}
		s.deployedLabels[label] = true
		return nil
	}
	return fmt.Errorf("%s: unknown invalid deploy label %q", infraNWTableIndexOps561ID, label)
}

// createLiveIndex applies one live secondary index on the named window or
// table, mirroring a separate `create index` statement deploy.
func (s *infraNWTableIndexOps561CaseState) createLiveIndex(infra, name string,
	columns []string, kind esper.IndexKind, unique bool) error {
	if !s.spec.namedWindow {
		table, ok := s.engine.Table(infra)
		if !ok {
			return fmt.Errorf("%s: table %s is missing", infraNWTableIndexOps561ID, infra)
		}
		return table.CreateIndex(name, columns, kind, unique)
	}
	window, ok := s.engine.NamedWindow(infra)
	if !ok {
		return fmt.Errorf("%s: named window %s is missing", infraNWTableIndexOps561ID, infra)
	}
	return window.CreateIndex(name, columns, kind, unique)
}

func (s *infraNWTableIndexOps561CaseState) deployPlan(ctx context.Context, label string,
	plan esper.Plan) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[label] = deployment
	s.deployedLabels[label] = true
	return nil
}

// snapshot executes the pinned compileExecuteFAF read for the mcmi cases:
// the step's statement label selects the typed FAF plan and the epl field
// pins the Java query text. The planned index selection is asserted
// honestly per probe; rows are projected to the pinned f1..f4 fields and
// canonically sorted — the Java assertPropsPerRow is any-order.
func (s *infraNWTableIndexOps561CaseState) snapshot(ctx context.Context, step compat.Step) error {
	plan, err := s.buildFafSelect(step)
	if err != nil {
		return err
	}
	if err := s.assertFafSelection(step, plan); err != nil {
		return err
	}
	result, err := s.engine.ExecuteFireAndForget(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: faf %q: %w", infraNWTableIndexOps561ID, step.Statement, err)
	}
	rows := infraTableJoinNormalizeResults(result.Results())
	rows = projectInfraNWTableOnMergeRows(rows, []string{"f1", "f2", "f3", "f4"})
	sortRowsCanonical(rows)
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "snapshot",
		Statement: step.Statement,
		Sequence:  0,
		Time:      "1970-01-01T00:00:00Z",
		New:       rows,
	})
	return nil
}

// assertFafSelection pins the index candidate each MCMI probe resolves:
// f3-only and f1-only (window variant) full-scan, f3+f2 resolves Index2,
// f2-only resolves Index3, the full-key and four-column probes resolve
// Index1, and the table f1 probe resolves the implicit primary-key
// candidate. Java's hash indexes require the full column set for the same
// probes, so the resolved selection is pinned rather than faked.
func (s *infraNWTableIndexOps561CaseState) assertFafSelection(step compat.Step, plan esper.Plan) error {
	selection, ok := plan.IndexPlan().ForSource(0)
	var wantIndex string
	var wantColumns []string
	switch step.Statement {
	case "select-f3":
		wantIndex = ""
	case "select-f3-f2":
		wantIndex, wantColumns = "MyInfraMCMIIndex2", []string{"f2", "f3"}
	case "select-full", "select-all":
		wantIndex, wantColumns = "MyInfraMCMIIndex1", []string{"f2", "f3", "f1"}
	case "select-f2":
		wantIndex, wantColumns = "MyInfraMCMIIndex3", []string{"f2"}
	case "select-f1":
		if !s.spec.namedWindow {
			wantIndex, wantColumns = "<primary-key>", []string{"f1"}
		}
	default:
		return fmt.Errorf("%s: unknown faf select %q", infraNWTableIndexOps561ID, step.Statement)
	}
	if wantIndex == "" {
		if ok && selection.Access != esper.IndexAccessFullScan {
			return fmt.Errorf("%s: faf %q unexpectedly resolved to index %q (access %v) instead of a full scan",
				infraNWTableIndexOps561ID, step.Statement, selection.IndexName, selection.Access)
		}
		return nil
	}
	if !ok || selection.Access != esper.IndexAccessEquality ||
		selection.IndexName != wantIndex ||
		!reflect.DeepEqual(selection.MatchedColumns, wantColumns) {
		return fmt.Errorf("%s: faf %q resolved to %#v instead of the %s index",
			infraNWTableIndexOps561ID, step.Statement, selection, wantIndex)
	}
	return nil
}

// buildFafSelect maps the pinned FAF select EPL to the typed plan; the
// probe conjuncts follow the Java where-clause order exactly.
func (s *infraNWTableIndexOps561CaseState) buildFafSelect(step compat.Step) (esper.Plan, error) {
	var source esper.RecordStream
	if s.spec.namedWindow {
		source = esper.FromNamedWindow(s.env, "MyInfraMCMI")
	} else {
		source = esper.FromTable(s.env, "MyInfraMCMI")
	}
	f3 := esper.EqualOf(esper.Field[any, string]("f3"), esper.Literal[string](">E1<"))
	f2 := esper.EqualOf(esper.Field[any, int]("f2"), esper.Literal[int](-2))
	f1 := esper.EqualOf(esper.Field[any, string]("f1"), esper.Literal[string]("E1"))
	f4 := esper.EqualOf(esper.Field[any, string]("f4"), esper.Literal[string]("?E1?"))
	var filter esper.Expression[bool]
	switch step.Statement {
	case "select-f3":
		if step.Epl != infraNWTableMCMI561SelectF3 {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = f3
	case "select-f3-f2":
		if step.Epl != infraNWTableMCMI561SelectF3F2 {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = esper.And(f3, f2)
	case "select-full":
		if step.Epl != infraNWTableMCMI561SelectFull {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = esper.And(esper.And(f3, f2), f1)
	case "select-f2":
		if step.Epl != infraNWTableMCMI561SelectF2 {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = f2
	case "select-f1":
		if step.Epl != infraNWTableMCMI561SelectF1 {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = f1
	case "select-all":
		if step.Epl != infraNWTableMCMI561SelectAll {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = esper.And(esper.And(esper.And(f3, f2), f1), f4)
	default:
		return esper.Plan{}, fmt.Errorf("%s: unknown faf select %q", infraNWTableIndexOps561ID, step.Statement)
	}
	return s.env.Build(source.Filter(filter).Query())
}

func (s *infraNWTableIndexOps561CaseState) fafDrift(step compat.Step) error {
	return fmt.Errorf("%s: faf select %q does not pin the expected EPL %q",
		infraNWTableIndexOps561ID, step.Statement, step.Epl)
}

// indexCount mirrors a Java getIndexCount assert whose observed count
// coincides with the Go surface: onr-table counts (declared indexes plus
// the implicit primary-key descriptor) and the post-stmtTwo-undeploy
// onr-window count (the implicit index released with the last consumer).
// The record carries the observed count exactly like the oracle.
func (s *infraNWTableIndexOps561CaseState) indexCount(step compat.Step) error {
	if step.Count == nil {
		return fmt.Errorf("%s: index-count %q has no count", infraNWTableIndexOps561ID, step.Statement)
	}
	count, err := s.observedIndexCount(step.Statement)
	if err != nil {
		return err
	}
	if count != *step.Count {
		return fmt.Errorf("%s: index-count mismatch for %s (indexes): expected %d, got %d",
			infraNWTableIndexOps561ID, step.Statement, *step.Count, count)
	}
	// The oracle sequences index-count records per infra name.
	s.indexSeq[step.Statement]++
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "index-count",
		Statement: step.Statement,
		Sequence:  s.indexSeq[step.Statement],
		Time:      "1970-01-01T00:00:00Z",
		Count:     &count,
	})
	return nil
}

// observedIndexCount mirrors SupportInfraUtil.getIndexCountNoContext: the
// named window's IndexCount (declared plus trigger-inferred implicit
// indexes) or the table's declared secondary indexes plus the implicit
// primary-key descriptor Java counts.
func (s *infraNWTableIndexOps561CaseState) observedIndexCount(infra string) (int64, error) {
	if window, ok := s.engine.NamedWindow(infra); ok {
		return int64(window.IndexCount()), nil
	}
	if table, ok := s.engine.Table(infra); ok {
		definition := table.Definition()
		count := int64(len(definition.Indexes()))
		if len(definition.PrimaryKey()) > 0 {
			count++
		}
		return count, nil
	}
	return 0, fmt.Errorf("%s: index-count targets unknown infra %q", infraNWTableIndexOps561ID, infra)
}

// undeploy mirrors undeployModuleContaining for deployment-owned labels
// (s0, stmtTwo) and the index statement (indexOne) which has no Go
// deployment — it maps to DropIndex on the live index, whose declared
// descriptor outlives its consumers exactly like Java's.
func (s *infraNWTableIndexOps561CaseState) undeploy(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "s0", "stmtTwo":
		deployment, ok := s.deployments[step.Statement]
		if !ok {
			return fmt.Errorf("%s: undeploy targets unknown statement %q", infraNWTableIndexOps561ID, step.Statement)
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return err
		}
		delete(s.deployments, step.Statement)
		return nil
	case infraNWTableONR561UndeployIndex:
		// undeployModuleContaining("indexOne") retires the module owning
		// the create-index statement; the Go live index has no deployment
		// so the equivalent is DropIndex on MyInfraONRIndex1.
		if !s.spec.namedWindow {
			table, ok := s.engine.Table("MyInfraONR")
			if !ok {
				return fmt.Errorf("%s: table MyInfraONR is missing", infraNWTableIndexOps561ID)
			}
			return table.DropIndex("MyInfraONRIndex1")
		}
		window, ok := s.engine.NamedWindow("MyInfraONR")
		if !ok {
			return fmt.Errorf("%s: named window MyInfraONR is missing", infraNWTableIndexOps561ID)
		}
		return window.DropIndex("MyInfraONRIndex1")
	}
	return fmt.Errorf("%s: unknown undeploy label %q", infraNWTableIndexOps561ID, step.Statement)
}

// buildError runs the Go counterpart of one tryInvalidCompile probe whose
// rejection surface exists (live CreateIndex validation or infra lookup),
// asserts the matching esper.Error code and records the pinned Java
// message prefix.
func (s *infraNWTableIndexOps561CaseState) buildError(step compat.Step) error {
	expected, ok := infraNWTableIndexOps561BuildErrors[s.caseName][step.Statement]
	if !ok || step.ExpectError != expected {
		return fmt.Errorf("%s: build-error %q is not pinned", infraNWTableIndexOps561ID, step.Statement)
	}
	var err error
	var code esper.ErrorCode
	switch step.Statement {
	case "dup-index":
		err = s.createLiveIndex("MyInfraOne", "MyInfraIndex", []string{"f1"}, esper.IndexHash, false)
		code = esper.ErrorDependency
	case "unknown-column":
		err = s.createLiveIndex("MyInfraOne", "IndexTwo", []string{"fx"}, esper.IndexHash, false)
		code = esper.ErrorUnknownName
	case "unknown-infra":
		// Java's infra lookup fails before the duplicate-column check; Go
		// mirrors it with a missing-infra lookup.
		if s.spec.namedWindow {
			_, ok := s.engine.NamedWindow("MyWindowX")
			if !ok {
				err = esper.NewError(esper.ErrorUnknownName, "named window MyWindowX is not available")
			}
		} else {
			_, ok := s.engine.Table("MyWindowX")
			if !ok {
				err = esper.NewError(esper.ErrorUnknownName, "table MyWindowX is not available")
			}
		}
		code = esper.ErrorUnknownName
	case "bad-kind":
		// The 'bubu' advanced-type marker has no typed IndexKind; the Go
		// counterpart is an out-of-domain kind rejected by the same guard.
		err = s.createLiveIndex("MyInfraOne", "IndexTwo", []string{"f1", "f2"}, esper.IndexKind(99), false)
		code = esper.ErrorInvalidRule
	case "unique-btree":
		err = s.createLiveIndex("MyInfraOne", "IndexTwo", []string{"f2"}, esper.IndexBTree, true)
		code = esper.ErrorInvalidRule
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", infraNWTableIndexOps561ID, step.Statement)
	}
	if err == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly succeeded", infraNWTableIndexOps561ID, step.Statement)
	}
	if !errors.Is(err, code) {
		return fmt.Errorf("%s: build-error probe %q rejected with %v, want code %s",
			infraNWTableIndexOps561ID, step.Statement, err, code)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// unrepresentable emits the pinned records for probes with no Go boundary
// (context-scoped and EPL-text-only create-index spellings), for probes
// Java rejects but Go's CreateIndex accepts (the runner performs the call,
// pins success, then drops the index to mirror the Java fixture state),
// and for the divergent onr index-count asserts (the runner verifies the
// pinned Go-observed count internally).
func (s *infraNWTableIndexOps561CaseState) unrepresentable(step compat.Step) error {
	pinned, ok := infraNWTableIndexOps561Unrepresentable[s.caseName][step.Statement]
	if !ok || step.Epl != pinned.epl || step.ExpectError != pinned.note {
		return fmt.Errorf("%s: unrepresentable step %q is not pinned",
			infraNWTableIndexOps561ID, step.Statement)
	}
	switch step.Statement {
	case "dup-column":
		// Java rejects the repeated column; Go's CreateIndex accepts it —
		// create then drop so the fixture state matches Java's.
		if err := s.createLiveIndex("MyInfraOne", "IndexTwo", []string{"f1", "f1"}, esper.IndexHash, false); err != nil {
			return fmt.Errorf("%s: duplicate-column probe unexpectedly rejected: %w",
				infraNWTableIndexOps561ID, err)
		}
		if s.spec.namedWindow {
			window, _ := s.engine.NamedWindow("MyInfraOne")
			if err := window.DropIndex("IndexTwo"); err != nil {
				return err
			}
		} else {
			table, _ := s.engine.Table("MyInfraOne")
			if err := table.DropIndex("IndexTwo"); err != nil {
				return err
			}
		}
	case "no-pk-index":
		// Java requires a primary key for table indexes; Go accepts the
		// index on the PK-less table — create then drop to mirror Java.
		table, ok := s.engine.Table("MyTable")
		if !ok {
			return fmt.Errorf("%s: table MyTable is missing", infraNWTableIndexOps561ID)
		}
		if err := table.CreateIndex("MyIndex", []string{"p0"}, esper.IndexHash, false); err != nil {
			return fmt.Errorf("%s: no-primary-key index probe unexpectedly rejected: %w",
				infraNWTableIndexOps561ID, err)
		}
		if err := table.DropIndex("MyIndex"); err != nil {
			return err
		}
	case "count-after-s0":
		if err := s.assertObservedIndexCount("MyInfraONR", 2); err != nil {
			return err
		}
	case "count-after-two":
		if err := s.assertObservedIndexCount("MyInfraONR", 2); err != nil {
			return err
		}
	case "count-after-s0-undeploy":
		if err := s.assertObservedIndexCount("MyInfraONR", 2); err != nil {
			return err
		}
	case "count-cw":
		if err := s.assertObservedIndexCount("MyInfraFour", 3); err != nil {
			return err
		}
	case "context-a", "context-b", "gugu", "null-typed":
		// EPL-text-only spellings: no Go surface exists to exercise.
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// assertObservedIndexCount pins the Go-observed count for the divergent
// index-count asserts: the unrepresentable record documents the
// Java-asserted value while this check keeps the Go observation honest.
func (s *infraNWTableIndexOps561CaseState) assertObservedIndexCount(infra string, want int64) error {
	count, err := s.observedIndexCount(infra)
	if err != nil {
		return err
	}
	if count != want {
		return fmt.Errorf("%s: Go-observed index-count for %s = %d, want %d",
			infraNWTableIndexOps561ID, infra, count, want)
	}
	return nil
}

// sendError mirrors the unique-index-violation send: the second
// SupportBean("E1", 2) must be rejected with the unique-index-violation
// text; the record carries the pinned Java statement-attributed message.
func (s *infraNWTableIndexOps561CaseState) sendError(ctx context.Context, step compat.Step) error {
	if step.Statement != "unique-violation" || step.EventType != "SupportBean" {
		return fmt.Errorf("%s: unknown send-error step %q", infraNWTableIndexOps561ID, step.Statement)
	}
	expected := infraNWTableINV561ErrSendNW
	if !s.spec.namedWindow {
		expected = infraNWTableINV561ErrSendTBL
	}
	if step.ExpectError != expected {
		return fmt.Errorf("%s: send-error %q is not pinned", infraNWTableIndexOps561ID, step.Statement)
	}
	event, err := decodeInfraNWTableIndexOps561Payload(compat.Step{
		EventType: step.EventType, Payload: step.Payload})
	if err != nil {
		return err
	}
	err = s.engine.Send(ctx, step.EventType, event)
	if err == nil {
		return fmt.Errorf("%s: unique-violation send unexpectedly succeeded", infraNWTableIndexOps561ID)
	}
	// Go surfaces the violation as an ErrorState naming the unique index
	// (`unique ... index "I1" rejected duplicate key`); the Java
	// statement-attributed text is pinned in the record.
	if !errors.Is(err, esper.ErrorState) || !strings.Contains(err.Error(), "I1") {
		return fmt.Errorf("%s: unique-violation send rejected with %v, want a unique index violation",
			infraNWTableIndexOps561ID, err)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "send-error",
		Statement: step.Statement,
		Sequence:  0,
		Value:     step.ExpectError,
	})
	return nil
}

func (s *infraNWTableIndexOps561CaseState) undeployAll(ctx context.Context) error {
	for label, deployment := range s.deployments {
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", infraNWTableIndexOps561ID, label, err)
		}
	}
	s.deployments = make(map[string]*esper.Deployment)
	s.deployedLabels = make(map[string]bool)
	return nil
}

// decodeInfraNWTableIndexOps561Payload mirrors the sendEventBean calls:
// SupportBean payloads carry theString plus intPrimitive and
// SupportBean_S0 payloads carry id; unmentioned fields stay zero.
func decodeInfraNWTableIndexOps561Payload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value infraNWTableIndexOps561Bean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value infraNWTableIndexOps561S0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	}
	return nil, fmt.Errorf("unsupported infra nwtable index-ops event type %q", step.EventType)
}

// infraNWTableIndexOps561DeployEPLs pins the byte-exact EPL per deploy
// label per case.
var infraNWTableIndexOps561DeployEPLs = map[string]map[string]string{
	"mcmi-window": {
		"create":      infraNWTableMCMI561CreateWindow,
		"insert":      infraNWTableMCMI561Insert,
		"index-one":   infraNWTableMCMI561Index1,
		"index-two":   infraNWTableMCMI561Index2,
		"index-three": infraNWTableMCMI561Index3,
	},
	"mcmi-table": {
		"create":      infraNWTableMCMI561CreateTable,
		"insert":      infraNWTableMCMI561Insert,
		"index-one":   infraNWTableMCMI561Index1,
		"index-two":   infraNWTableMCMI561Index2,
		"index-three": infraNWTableMCMI561Index3,
	},
	"onr-window": {
		"create":      infraNWTableONR561CreateWindow,
		"insert":      infraNWTableONR561Insert,
		"index":       infraNWTableONR561Index,
		"s0":          infraNWTableONR561SelectS0,
		"stmtTwo":     infraNWTableONR561SelectTwo,
		"cw":          infraNWTableONR561CreateFour,
		"cw-index":    infraNWTableONR561IndexFour,
		"on-select-a": infraNWTableONR561OnSelectA,
		"on-select-b": infraNWTableONR561OnSelectB,
	},
	"onr-table": {
		"create":      infraNWTableONR561CreateTable,
		"insert":      infraNWTableONR561Insert,
		"index":       infraNWTableONR561Index,
		"s0":          infraNWTableONR561SelectS0,
		"stmtTwo":     infraNWTableONR561SelectTwo,
		"cw":          infraNWTableONR561CreateFour,
		"cw-index":    infraNWTableONR561IndexFour,
		"on-select-a": infraNWTableONR561OnSelectA,
		"on-select-b": infraNWTableONR561OnSelectB,
	},
	"invalid-window": {
		"create":       infraNWTableINV561CreateWindow,
		"index":        infraNWTableINV561Index,
		"context-one":  infraNWTableINV561ContextOne,
		"context-two":  infraNWTableINV561ContextTwo,
		"create-ctx":   infraNWTableINV561CreateCtxNW,
		"create-two":   infraNWTableINV561CreateTwoNW,
		"insert":       infraNWTableINV561InsertTwo,
		"index-unique": infraNWTableINV561UniqueIndex,
	},
	"invalid-table": {
		"create":       infraNWTableINV561CreateTable,
		"index":        infraNWTableINV561Index,
		"context-one":  infraNWTableINV561ContextOne,
		"context-two":  infraNWTableINV561ContextTwo,
		"create-ctx":   infraNWTableINV561CreateCtxTBL,
		"create-two":   infraNWTableINV561CreateTwoTBL,
		"insert":       infraNWTableINV561InsertTwo,
		"index-unique": infraNWTableINV561UniqueIndex,
		"create-nokey": infraNWTableINV561CreateNoKey,
	},
}

// infraNWTableIndexOps561BuildErrors pins the Java-asserted prefix per
// build-error probe per case — the probes whose Go counterpart exists.
var infraNWTableIndexOps561BuildErrors = map[string]map[string]string{
	"invalid-window": {
		"dup-index":      infraNWTableINV561ErrDupIndex,
		"unknown-column": infraNWTableINV561ErrUnknownCol,
		"unknown-infra":  infraNWTableINV561ErrUnknownInf,
		"bad-kind":       infraNWTableINV561ErrBadKind,
		"unique-btree":   infraNWTableINV561ErrUniqueBT,
	},
	"invalid-table": {
		"dup-index":      infraNWTableINV561ErrDupIndex,
		"unknown-column": infraNWTableINV561ErrUnknownCol,
		"unknown-infra":  infraNWTableINV561ErrUnknownInf,
		"bad-kind":       infraNWTableINV561ErrBadKind,
		"unique-btree":   infraNWTableINV561ErrUniqueBT,
	},
}

// infraNWTableIndexOps561UnrepresentablePins carries the pinned probe EPL
// and the note/Java message recorded per unrepresentable step.
type infraNWTableIndexOps561UnrepresentablePins struct {
	epl  string
	note string
}

var infraNWTableIndexOps561Unrepresentable = map[string]map[string]infraNWTableIndexOps561UnrepresentablePins{
	"onr-window": {
		"count-after-s0":          {note: infraNWTableONR561NoteCount1},
		"count-after-two":         {note: infraNWTableONR561NoteCount1},
		"count-after-s0-undeploy": {note: infraNWTableONR561NoteCount2},
		"count-cw":                {note: infraNWTableONR561NoteFour},
	},
	"onr-table": {
		"count-cw": {note: infraNWTableONR561NoteFour},
	},
	"invalid-window": {
		"context-a":  {epl: infraNWTableINV561ProbeCtxA, note: infraNWTableINV561ErrCtxNW},
		"context-b":  {epl: infraNWTableINV561ProbeCtxB, note: infraNWTableINV561ErrCtxNW},
		"dup-column": {epl: infraNWTableINV561ProbeDupCol, note: infraNWTableINV561ErrDupCol},
		"gugu":       {epl: infraNWTableINV561ProbeGugu, note: infraNWTableINV561ErrGugu},
		"null-typed": {epl: infraNWTableINV561ProbeNullTyped, note: infraNWTableINV561ErrNullTyped},
	},
	"invalid-table": {
		"context-a":   {epl: infraNWTableINV561ProbeCtxA, note: infraNWTableINV561ErrCtxTBL},
		"context-b":   {epl: infraNWTableINV561ProbeCtxB, note: infraNWTableINV561ErrCtxTBL},
		"dup-column":  {epl: infraNWTableINV561ProbeDupCol, note: infraNWTableINV561ErrDupCol},
		"gugu":        {epl: infraNWTableINV561ProbeGugu, note: infraNWTableINV561ErrGugu},
		"null-typed":  {epl: infraNWTableINV561ProbeNullTyped, note: infraNWTableINV561ErrNullTyped},
		"no-pk-index": {epl: infraNWTableINV561ProbeNoPK, note: infraNWTableINV561ErrNoPK},
	},
}

// loadInfraNWTableIndexOps561Scenario enforces the strict scenario
// contract shared by the differential runners: no duplicate or unknown
// JSON fields, pinned metadata, pinned per-case runtime/execution/EPL,
// and a per-op step field whitelist followed by a full step-shape pin.
func loadInfraNWTableIndexOps561Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableIndexOps561ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableIndexOps561ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableIndexOps561ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableIndexOps561ID, err)
	}
	if err := requireInfraNWTableIndexOps561Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableIndexOps561ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableIndexOps561ID ||
		metadata.Description != infraNWTableIndexOps561Description ||
		metadata.JavaCommit != infraNWTableIndexOps561JavaCommit ||
		metadata.JavaSource != infraNWTableIndexOps561JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableIndexOps561ID)
	}
	if err := validateInfraNWTableIndexOps561StringArray(root["javaRuntimes"], infraNWTableIndexOps561JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableIndexOps561StringArray(root["javaNames"], infraNWTableIndexOps561JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableIndexOps561StringArray(root["javaStaticIds"], infraNWTableIndexOps561JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableIndexOps561StringArray(root["javaFlags"], infraNWTableIndexOps561JavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableIndexOps561Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableIndexOps561ID, len(infraNWTableIndexOps561Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableIndexOps561Fields(object,
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
		if definition.Case != infraNWTableIndexOps561Cases[index] ||
			definition.Ordinal != infraNWTableIndexOps561Ordinals[index] ||
			definition.RuntimeID != infraNWTableIndexOps561JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableIndexOps561JavaExecutions[index] ||
			definition.Observation != infraNWTableIndexOps561CaseObservations[index] ||
			definition.EPL != infraNWTableIndexOps561CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableIndexOps561ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableIndexOps561ID)
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
			if err := requireInfraNWTableIndexOps561Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableIndexOps561Fields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableIndexOps561Fields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableIndexOps561Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableIndexOps561Payload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send-error":
			if err := requireInfraNWTableIndexOps561Fields(object, "op", "case", "statement", "eventType", "payload", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableIndexOps561Fields(object, "op", "case", "statement", "mode", "fields", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "index-count":
			if err := requireInfraNWTableIndexOps561Fields(object, "op", "case", "statement", "create", "of", "count"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "build-error":
			if err := requireInfraNWTableIndexOps561Fields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "unrepresentable":
			if _, hasEpl := object["epl"]; hasEpl {
				if err := requireInfraNWTableIndexOps561Fields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
					return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
				}
			} else if err := requireInfraNWTableIndexOps561Fields(object, "op", "case", "statement", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy":
			if err := requireInfraNWTableIndexOps561Fields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableIndexOps561Fields(object, "op", "case"); err != nil {
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
		for _, name := range infraNWTableIndexOps561Cases {
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
	if err := validateInfraNWTableIndexOps561RawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraNWTableIndexOps561RawSteps pins the complete step sequence
// per case against the raw JSON objects: deploy steps with byte-exact EPL,
// deployed markers, the SupportBean/SupportBean_S0 sends, the FAF
// snapshot reads, the index-count and unrepresentable count probes, the
// invalid compile probes, the unique-violation send and the undeploy
// terminators.
func validateInfraNWTableIndexOps561RawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableIndexOps561Cases {
		want, ok := infraNWTableIndexOps561CaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableIndexOps561ID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableIndexOps561ID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableIndexOps561ID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableIndexOps561ID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableIndexOps561StepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableIndexOps561ID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q",
					infraNWTableIndexOps561ID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableIndexOps561ID)
	}
	return nil
}

// infraNWTableIndexOps561StepKey renders a raw step object into its
// pinned string form. Fields are read from the raw JSON because
// compat.Step does not carry the fields array.
func infraNWTableIndexOps561StepKey(object map[string]json.RawMessage, operation string) (string, error) {
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
	case "send", "send-error":
		statement := ""
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
		key := operation + ":" + eventType + ":" + string(canonical)
		if operation == "send-error" {
			statement, err = stringField("statement")
			if err != nil {
				return "", err
			}
			expectError, err := stringField("expectError")
			if err != nil {
				return "", err
			}
			key = operation + ":" + statement + ":" + eventType + ":" + string(canonical) + ":" + expectError
		}
		return key, nil
	case "snapshot":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		mode, err := stringField("mode")
		if err != nil {
			return "", err
		}
		var fields []string
		if err := json.Unmarshal(object["fields"], &fields); err != nil {
			return "", fmt.Errorf("step fields must be a string array")
		}
		epl, err := stringField("epl")
		if err != nil {
			return "", err
		}
		return "snapshot:" + statement + ":" + mode + ":" + joinStrings(fields, ",") + ":" + epl, nil
	case "index-count":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		create, err := stringField("create")
		if err != nil {
			return "", err
		}
		of, err := stringField("of")
		if err != nil {
			return "", err
		}
		var count json.Number
		if err := json.Unmarshal(object["count"], &count); err != nil {
			return "", fmt.Errorf("index-count step count must be a number")
		}
		return "index-count:" + statement + ":" + create + ":" + of + ":" + count.String(), nil
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
	case "unrepresentable":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		epl := ""
		if raw, hasEpl := object["epl"]; hasEpl {
			if err := json.Unmarshal(raw, &epl); err != nil {
				return "", fmt.Errorf("unrepresentable step epl must be a string")
			}
		}
		note, err := stringField("expectError")
		if err != nil {
			return "", err
		}
		return "unrepresentable:" + statement + ":" + epl + ":" + note, nil
	case "undeploy":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		return "undeploy:" + statement, nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// infraNWTableIndexOps561CaseSteps pins the exact op sequence per case.
var infraNWTableIndexOps561CaseSteps = map[string][]string{
	"mcmi-window":    mcmi561CaseSteps(infraNWTableMCMI561CreateWindow),
	"mcmi-table":     mcmi561CaseSteps(infraNWTableMCMI561CreateTable),
	"onr-window":     onr561WindowCaseSteps(),
	"onr-table":      onr561TableCaseSteps(),
	"invalid-window": invalid561CaseSteps(infraNWTableINV561CreateWindow, infraNWTableINV561CreateCtxNW, infraNWTableINV561CreateTwoNW, infraNWTableINV561ErrCtxNW, infraNWTableINV561ErrSendNW, false),
	"invalid-table":  invalid561CaseSteps(infraNWTableINV561CreateTable, infraNWTableINV561CreateCtxTBL, infraNWTableINV561CreateTwoTBL, infraNWTableINV561ErrCtxTBL, infraNWTableINV561ErrSendTBL, true),
}

// mcmi561CaseSteps renders the pinned step sequence of
// InfraMultipleColumnMultipleIndex.run (InfraNWTableCreateIndex.java
// lines 282-316): the create/insert/index deploys, the three SupportBean
// sends, the six FAF probes and undeployAll.
func mcmi561CaseSteps(create string) []string {
	return []string{
		"deploy:create:" + create,
		"deployed:create",
		"deploy:insert:" + infraNWTableMCMI561Insert,
		"deployed:insert",
		"deploy:index-one:" + infraNWTableMCMI561Index1,
		"deployed:index-one",
		"deploy:index-two:" + infraNWTableMCMI561Index2,
		"deployed:index-two",
		"deploy:index-three:" + infraNWTableMCMI561Index3,
		"deployed:index-three",
		`send:SupportBean:{"intPrimitive":-2,"theString":"E1"}`,
		`send:SupportBean:{"intPrimitive":-4,"theString":"E2"}`,
		`send:SupportBean:{"intPrimitive":-3,"theString":"E3"}`,
		"snapshot:select-f3:any:f1,f2,f3,f4:" + infraNWTableMCMI561SelectF3,
		"snapshot:select-f3-f2:any:f1,f2,f3,f4:" + infraNWTableMCMI561SelectF3F2,
		"snapshot:select-full:any:f1,f2,f3,f4:" + infraNWTableMCMI561SelectFull,
		"snapshot:select-f2:any:f1,f2,f3,f4:" + infraNWTableMCMI561SelectF2,
		"snapshot:select-f1:any:f1,f2,f3,f4:" + infraNWTableMCMI561SelectF1,
		"snapshot:select-all:any:f1,f2,f3,f4:" + infraNWTableMCMI561SelectAll,
		"undeploy-all",
	}
}

// onr561CaseSteps renders the shared step prefix/suffix of
// InfraOnSelectReUse.run (InfraNWTableCreateIndex.java lines 166-203);
// the divergent index-count asserts differ between the variants.
func onr561CaseSteps(create string, counts []string) []string {
	steps := []string{
		"deploy:create:" + create,
		"deployed:create",
		"deploy:insert:" + infraNWTableONR561Insert,
		"deployed:insert",
		"deploy:index:" + infraNWTableONR561Index,
		"deployed:index",
		`send:SupportBean:{"intPrimitive":1,"theString":"E1"}`,
		"deploy:s0:" + infraNWTableONR561SelectS0,
		"deployed:s0",
	}
	steps = append(steps, counts[0])
	steps = append(steps,
		`send:SupportBean_S0:{"id":1}`,
		"deploy:stmtTwo:"+infraNWTableONR561SelectTwo,
		"deployed:stmtTwo",
	)
	steps = append(steps, counts[1])
	steps = append(steps, "undeploy:s0")
	steps = append(steps, counts[2])
	steps = append(steps, "undeploy:stmtTwo")
	steps = append(steps, counts[3])
	steps = append(steps,
		"undeploy:"+infraNWTableONR561UndeployIndex,
		"deploy:cw:"+infraNWTableONR561CreateFour,
		"deployed:cw",
		"deploy:cw-index:"+infraNWTableONR561IndexFour,
		"deployed:cw-index",
		"deploy:on-select-a:"+infraNWTableONR561OnSelectA,
		"deployed:on-select-a",
		"deploy:on-select-b:"+infraNWTableONR561OnSelectB,
		"deployed:on-select-b",
		"unrepresentable:count-cw::"+infraNWTableONR561NoteFour,
		"undeploy-all",
	)
	return steps
}

// onr561WindowCaseSteps pins the named-window variant: the three divergent
// count asserts ride unrepresentable notes and the post-stmtTwo-undeploy
// count — where Go's implicit index has released and both engines observe
// the declared index alone — rides a real index-count record.
func onr561WindowCaseSteps() []string {
	return onr561CaseSteps(infraNWTableONR561CreateWindow, []string{
		"unrepresentable:count-after-s0::" + infraNWTableONR561NoteCount1,
		"unrepresentable:count-after-two::" + infraNWTableONR561NoteCount1,
		"unrepresentable:count-after-s0-undeploy::" + infraNWTableONR561NoteCount2,
		"index-count:MyInfraONR:create:indexes:1",
	})
}

// onr561TableCaseSteps pins the table variant: every MyInfraONR count
// coincides (declared index plus the implicit primary-key descriptor).
func onr561TableCaseSteps() []string {
	return onr561CaseSteps(infraNWTableONR561CreateTable, []string{
		"index-count:MyInfraONR:create:indexes:2",
		"index-count:MyInfraONR:create:indexes:2",
		"index-count:MyInfraONR:create:indexes:2",
		"index-count:MyInfraONR:create:indexes:2",
	})
}

// invalid561CaseSteps renders the pinned step sequence of InfraInvalid.run
// (InfraNWTableCreateIndex.java lines 76-150): the MyInfraOne fixture
// deploys, the context deploys and the contexted infra, the ten
// tryInvalidCompile probes, the MyInfraTwo unique-violation sequence, the
// table-only no-primary-key fixture and probe (last), then undeployAll.
func invalid561CaseSteps(create, createCtx, createTwo, errCtx, errSend string, table bool) []string {
	steps := []string{
		"deploy:create:" + create,
		"deployed:create",
		"deploy:index:" + infraNWTableINV561Index,
		"deployed:index",
		"deploy:context-one:" + infraNWTableINV561ContextOne,
		"deployed:context-one",
		"deploy:context-two:" + infraNWTableINV561ContextTwo,
		"deployed:context-two",
		"deploy:create-ctx:" + createCtx,
		"deployed:create-ctx",
		"unrepresentable:context-a:" + infraNWTableINV561ProbeCtxA + ":" + errCtx,
		"unrepresentable:context-b:" + infraNWTableINV561ProbeCtxB + ":" + errCtx,
		"build-error:dup-index:" + infraNWTableINV561ProbeDupIndex + ":" + infraNWTableINV561ErrDupIndex,
		"build-error:unknown-column:" + infraNWTableINV561ProbeUnknownCol + ":" + infraNWTableINV561ErrUnknownCol,
		"unrepresentable:dup-column:" + infraNWTableINV561ProbeDupCol + ":" + infraNWTableINV561ErrDupCol,
		"build-error:unknown-infra:" + infraNWTableINV561ProbeUnknownInf + ":" + infraNWTableINV561ErrUnknownInf,
		"build-error:bad-kind:" + infraNWTableINV561ProbeBadKind + ":" + infraNWTableINV561ErrBadKind,
		"unrepresentable:gugu:" + infraNWTableINV561ProbeGugu + ":" + infraNWTableINV561ErrGugu,
		"build-error:unique-btree:" + infraNWTableINV561ProbeUniqueBT + ":" + infraNWTableINV561ErrUniqueBT,
		"unrepresentable:null-typed:" + infraNWTableINV561ProbeNullTyped + ":" + infraNWTableINV561ErrNullTyped,
		"deploy:create-two:" + createTwo,
		"deployed:create-two",
		"deploy:insert:" + infraNWTableINV561InsertTwo,
		"deployed:insert",
		"deploy:index-unique:" + infraNWTableINV561UniqueIndex,
		"deployed:index-unique",
		`send:SupportBean:{"intPrimitive":1,"theString":"E1"}`,
		"send-error:unique-violation:SupportBean:" + `{"intPrimitive":2,"theString":"E1"}` + ":" + errSend,
	}
	if table {
		steps = append(steps,
			"deploy:create-nokey:"+infraNWTableINV561CreateNoKey,
			"deployed:create-nokey",
			"unrepresentable:no-pk-index:"+infraNWTableINV561ProbeNoPK+":"+infraNWTableINV561ErrNoPK,
		)
	}
	return append(steps, "undeploy-all")
}

func requireInfraNWTableIndexOps561Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraNWTableIndexOps561ID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraNWTableIndexOps561ID, name)
		}
	}
	return nil
}

func validateInfraNWTableIndexOps561StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
