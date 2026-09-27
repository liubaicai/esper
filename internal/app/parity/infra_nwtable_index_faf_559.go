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

// infra_nwtable_index_faf_559.go replays InfraNWTableCreateIndex ordinals
// 6-7 and 20-21 against the pinned Java oracle: the composite-index
// fire-and-forget probes over a three-column hash index (f2, f3, f1) whose
// where-clauses lead with the non-leading column f3.
//
//   - composite-window/composite-table (ords 6-7, InfraCompositeIndex
//     {namedWindow=true/false}): keepall window or f1-keyed table
//     MyInfraCI(f1 string, f2 int, f3 string, f4 string) fed by `insert into
//     MyInfraCI(f1, f2, f3, f4) select theString, intPrimitive,
//     '>'||theString||'<', '?'||theString||'?' from SupportBean`, a late
//     index MyInfraCIIndex on (f2, f3, f1), then three FAF probes —
//     `f3='>E1<'`, `f3='>E1<' and f2=-2`, `f3='>E1<' and f2=-2 and f1='E1'` —
//     each returning {E1,-2,>E1<,?E1?}; the SODA tail
//     (undeployModuleContaining("indexOne") plus the eplToModelCompileDeploy
//     of `create index MyInfraCIIndexTwo on MyInfraCI(f2, f3, f1)`) is
//     plan-only for Go, pinned as unrepresentable records.
//   - multikey-window/multikey-table (ords 20-21, InfraMultikeyIndexFAF
//     {isNamedWindow=true/false}): the same fixture over MyInfra (the window
//     uses the `.win:keepall()` form) with index MyInfraIndex on
//     (f2, f3, f1) and milestone(0..2) checkpoints between the three FAF
//     probes.
//
// Index-resolution spike (verified against internal/esper/index_plan.go):
// Go's composite hash matcher requires predicates on the leading index
// columns in declaration order, so the f3-leading probe and the f3+f2 probe
// cannot consume the (f2,f3,f1) key — both resolve to a full scan. Only the
// complete {f2,f3,f1} predicate set (in any order) matches, yielding
// IndexAccessEquality on the declared index. Java's hash index likewise
// requires its full column set for equality probes, so the row sets are
// observably identical; the runner asserts the full probe resolves to the
// declared index (not a scan) and pins full-scan for the prefix probes
// rather than faking index use.
//
// Approved differences (observably identical to the Java EPL):
//   - `create window`/`create table`/`create index` are catalog operations:
//     the deploy steps carry the byte-exact EPL while the Go side performs
//     env-level window/table registration; the secondary index is declared
//     at creation time because plan.indexPlan is frozen at env.Build — a
//     live CreateIndex would be invisible to FAF index selection (the
//     ordering divergence is observably identical: no rows exist before
//     the index step).
//   - `insert into X(f1, f2, f3, f4) select ...` maps to the OnEvent
//     InsertIntoNamedWindow/InsertIntoTable trigger with SetColumn
//     assignments; the `||` string concat maps to Concat of literal/field
//     operands.
//   - `env.compileExecuteFAF` rides `snapshot` steps that carry the pinned
//     query EPL; the runner builds the typed FAF plan and executes it
//     through ExecuteFireAndForget.
//   - env.milestone(n) checkpoints are harness no-ops and carry no steps.
//   - The SODA tail has no Go boundary: undeployModuleContaining has no
//     deployment to retire (the index step is a catalog check) and the
//     eplToModelCompileDeploy statement-object path does not exist; both
//     are pinned unrepresentable records.
//   - sendEventBean(new SupportBean("E1", -2)) maps to a payload carrying
//     theString plus intPrimitive; unmentioned bean fields stay zero.

// infraNWTableIndexFAF559Bean mirrors the SupportBean properties the
// SupportBean("E1", -2) constructor populates.
type infraNWTableIndexFAF559Bean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const (
	infraNWTableIndexFAF559ID         = "infra-nwtable-index-faf-559"
	infraNWTableIndexFAF559JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableIndexFAF559JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java"
)

const infraNWTableIndexFAF559Description = "InfraNWTableCreateIndex ordinals 6-7 and 20-21: InfraCompositeIndex (ords 6-7) deploys a keepall window or f1-keyed MyInfraCI(f1 string, f2 int, f3 string, f4 string) fed by a concat insert-into over SupportBean, a composite hash index MyInfraCIIndex on (f2, f3, f1) and three f3-leading FAF probes each returning {E1,-2,>E1<,?E1?}, then the plan-only SODA tail (undeployModuleContaining indexOne + eplToModelCompileDeploy IX2); InfraMultikeyIndexFAF (ords 20-21) runs the same fixture over MyInfra (.win:keepall() window form) with MyInfraIndex and milestone(0..2) between probes (harness no-ops carrying no steps). The f3-leading and f3+f2 probes resolve to a full scan on both engines (a composite hash index requires its full column set); only the complete {f2,f3,f1} predicate uses the declared index. Expected row {E1,-2,>E1<,?E1?} (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java)."

// Verbatim transcriptions of InfraNWTableCreateIndex.java lines 430-457
// (ords 6-7, InfraCompositeIndex.run) and lines 645-672 (ords 20-21,
// InfraMultikeyIndexFAF.run).
const (
	infraNWTableCI559CreateWindow = "@public create window MyInfraCI#keepall as (f1 string, f2 int, f3 string, f4 string)"
	infraNWTableCI559CreateTable  = "@public create table MyInfraCI as (f1 string primary key, f2 int, f3 string, f4 string)"
	infraNWTableCI559Insert       = "insert into MyInfraCI(f1, f2, f3, f4) select theString, intPrimitive, '>'||theString||'<', '?'||theString||'?' from SupportBean"
	infraNWTableCI559Index        = "@name('indexOne') create index MyInfraCIIndex on MyInfraCI(f2, f3, f1)"
	infraNWTableCI559SelectF3     = "select * from MyInfraCI where f3='>E1<'"
	infraNWTableCI559SelectF3F2   = "select * from MyInfraCI where f3='>E1<' and f2=-2"
	infraNWTableCI559SelectFull   = "select * from MyInfraCI where f3='>E1<' and f2=-2 and f1='E1'"
	infraNWTableCI559SodaIndexTwo = "create index MyInfraCIIndexTwo on MyInfraCI(f2, f3, f1)"

	infraNWTableMK559CreateWindow = "@public create window MyInfra.win:keepall() as (f1 string, f2 int, f3 string, f4 string)"
	infraNWTableMK559CreateTable  = "@public create table MyInfra as (f1 string primary key, f2 int, f3 string, f4 string)"
	infraNWTableMK559Insert       = "insert into MyInfra(f1, f2, f3, f4) select theString, intPrimitive, '>'||theString||'<', '?'||theString||'?' from SupportBean"
	infraNWTableMK559Index        = "create index MyInfraIndex on MyInfra(f2, f3, f1)"
	infraNWTableMK559SelectF3     = "select * from MyInfra where f3='>E1<'"
	infraNWTableMK559SelectF3F2   = "select * from MyInfra where f3='>E1<' and f2=-2"
	infraNWTableMK559SelectFull   = "select * from MyInfra where f3='>E1<' and f2=-2 and f1='E1'"
)

// infraNWTableCI559UndeployKey is the statement-name argument the Java
// composite tail passes to undeployModuleContaining (not EPL).
const infraNWTableCI559UndeployKey = "indexOne"

// SODA-tail notes pinned by the unrepresentable steps: the Java composite
// execution retires the index module and replays the IX2 index through the
// statement-object-model deploy path; neither has a Go boundary.
const (
	infraNWTableCI559UndeployIndexNote = "undeployModuleContaining(indexOne): the Java execution retires the @name('indexOne') index module before the SODA probe; the Go secondary index is declared at creation time and the deploy step is a catalog check, so no deployment exists to undeploy and the record is plan-only"
	infraNWTableCI559SodaIndexTwoNote  = "SODA create index MyInfraCIIndexTwo on MyInfraCI(f2, f3, f1): the Java execution replays eplToModelCompileDeploy (parse -> toEPL -> module compile -> deploy) then undeployAll; EPL-object-model deploy has no Go boundary so the record is plan-only"
)

var (
	infraNWTableIndexFAF559JavaSources = []string{
		infraNWTableIndexFAF559JavaSource,
	}
	infraNWTableIndexFAF559JavaRuntimeIDs = []string{
		"java-runtime-78145a646789e9674a38",
		"java-runtime-694aacd1c09bee51ffda",
		"java-runtime-4c0e49ebf2ae523ef825",
		"java-runtime-dbe715ce8e9fbede1285",
	}
	infraNWTableIndexFAF559JavaExecutions = []string{
		"InfraCompositeIndex{namedWindow=true}",
		"InfraCompositeIndex{namedWindow=false}",
		"InfraMultikeyIndexFAF{isNamedWindow=true}",
		"InfraMultikeyIndexFAF{isNamedWindow=false}",
	}
	infraNWTableIndexFAF559JavaStaticIDs = []string{
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
	}
	infraNWTableIndexFAF559JavaFlags = []string{"FIREANDFORGET"}
	infraNWTableIndexFAF559Cases     = []string{
		"composite-window",
		"composite-table",
		"multikey-window",
		"multikey-table",
	}
	infraNWTableIndexFAF559Ordinals = []int{6, 7, 20, 21}
)

// infraNWTableIndexFAF559CaseEPLs pins the newline-joined EPL of every
// EPL-bearing step in the case, in step order — the value carried by the
// scenario cases[] metadata. The undeploy-index-one step carries a
// statement-name key rather than EPL and is excluded.
var infraNWTableIndexFAF559CaseEPLs = []string{
	strings.Join([]string{
		infraNWTableCI559CreateWindow,
		infraNWTableCI559Insert,
		infraNWTableCI559Index,
		infraNWTableCI559SelectF3,
		infraNWTableCI559SelectF3F2,
		infraNWTableCI559SelectFull,
		infraNWTableCI559SodaIndexTwo,
	}, "\n"),
	strings.Join([]string{
		infraNWTableCI559CreateTable,
		infraNWTableCI559Insert,
		infraNWTableCI559Index,
		infraNWTableCI559SelectF3,
		infraNWTableCI559SelectF3F2,
		infraNWTableCI559SelectFull,
		infraNWTableCI559SodaIndexTwo,
	}, "\n"),
	strings.Join([]string{
		infraNWTableMK559CreateWindow,
		infraNWTableMK559Insert,
		infraNWTableMK559Index,
		infraNWTableMK559SelectF3,
		infraNWTableMK559SelectF3F2,
		infraNWTableMK559SelectFull,
	}, "\n"),
	strings.Join([]string{
		infraNWTableMK559CreateTable,
		infraNWTableMK559Insert,
		infraNWTableMK559Index,
		infraNWTableMK559SelectF3,
		infraNWTableMK559SelectF3F2,
		infraNWTableMK559SelectFull,
	}, "\n"),
}

var infraNWTableIndexFAF559CaseObservations = []string{
	"deploy+snapshot+unrepresentable; MyInfraCI#keepall window with composite hash index (f2,f3,f1): the f3-leading and f3+f2 FAF probes resolve to a full scan (Java's hash index requires its full column set) while the complete {f2,f3,f1} predicate uses the declared index; each returns {E1,-2,>E1<,?E1?}; undeployModuleContaining(indexOne) and the SODA IX2 deploy are plan-only",
	"deploy+snapshot+unrepresentable; f1-keyed MyInfraCI table with composite hash index (f2,f3,f1): the f3-leading and f3+f2 FAF probes resolve to a full scan (Java's hash index requires its full column set) while the complete {f2,f3,f1} predicate uses the declared index; each returns {E1,-2,>E1<,?E1?}; undeployModuleContaining(indexOne) and the SODA IX2 deploy are plan-only",
	"deploy+snapshot; MyInfra.win:keepall() window with composite hash index MyInfraIndex (f2,f3,f1): the f3-leading and f3+f2 FAF probes resolve to a full scan while the complete {f2,f3,f1} predicate uses the declared index; each returns {E1,-2,>E1<,?E1?}; milestone(0..2) checkpoints carry no steps",
	"deploy+snapshot; f1-keyed MyInfra table with composite hash index MyInfraIndex (f2,f3,f1): the f3-leading and f3+f2 FAF probes resolve to a full scan while the complete {f2,f3,f1} predicate uses the declared index; each returns {E1,-2,>E1<,?E1?}; milestone(0..2) checkpoints carry no steps",
}

// infraNWTableIndexFAF559CaseSpec carries the per-case fixture constants:
// whether the named-window or table variant runs, whether the composite
// SODA tail follows the probes, the infra and index names, and the
// byte-exact create/insert/index/select EPLs.
type infraNWTableIndexFAF559CaseSpec struct {
	namedWindow bool
	composite   bool
	infra       string
	indexName   string
	create      string
	insert      string
	index       string
	selects     []string
	soda        string
}

var infraNWTableIndexFAF559CaseSpecs = map[string]infraNWTableIndexFAF559CaseSpec{
	"composite-window": {
		namedWindow: true, composite: true,
		infra: "MyInfraCI", indexName: "MyInfraCIIndex",
		create: infraNWTableCI559CreateWindow, insert: infraNWTableCI559Insert,
		index: infraNWTableCI559Index, soda: infraNWTableCI559SodaIndexTwo,
		selects: []string{infraNWTableCI559SelectF3, infraNWTableCI559SelectF3F2, infraNWTableCI559SelectFull},
	},
	"composite-table": {
		namedWindow: false, composite: true,
		infra: "MyInfraCI", indexName: "MyInfraCIIndex",
		create: infraNWTableCI559CreateTable, insert: infraNWTableCI559Insert,
		index: infraNWTableCI559Index, soda: infraNWTableCI559SodaIndexTwo,
		selects: []string{infraNWTableCI559SelectF3, infraNWTableCI559SelectF3F2, infraNWTableCI559SelectFull},
	},
	"multikey-window": {
		namedWindow: true, composite: false,
		infra: "MyInfra", indexName: "MyInfraIndex",
		create: infraNWTableMK559CreateWindow, insert: infraNWTableMK559Insert,
		index:   infraNWTableMK559Index,
		selects: []string{infraNWTableMK559SelectF3, infraNWTableMK559SelectF3F2, infraNWTableMK559SelectFull},
	},
	"multikey-table": {
		namedWindow: false, composite: false,
		infra: "MyInfra", indexName: "MyInfraIndex",
		create: infraNWTableMK559CreateTable, insert: infraNWTableMK559Insert,
		index:   infraNWTableMK559Index,
		selects: []string{infraNWTableMK559SelectF3, infraNWTableMK559SelectF3F2, infraNWTableMK559SelectFull},
	},
}

// infraNWTableIndexFAF559CaseState carries the per-case replay state: the
// environment/engine pair, the label→deployments bookkeeping used by
// undeploy-all and the deployed-label set for marker checks.
type infraNWTableIndexFAF559CaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	spec           infraNWTableIndexFAF559CaseSpec
	deployments    map[string][]*esper.Deployment
	deployedLabels map[string]bool
	caseName       string
}

// runInfraNWTableIndexFAF559Scenario replays the four
// InfraNWTableCreateIndex composite/multikey executions: each case runs on
// a fresh environment/engine pair (one runtime per Java execution) and
// every step dispatches to the matching runtime action. The oracle emits
// the epoch time for every record, so the runner pins the same value.
func runInfraNWTableIndexFAF559Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableIndexFAF559Cases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableIndexFAF559Case(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableIndexFAF559ID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableIndexFAF559ID)
	}
	return trace, nil
}

func runInfraNWTableIndexFAF559Case(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	spec, ok := infraNWTableIndexFAF559CaseSpecs[caseName]
	if !ok {
		return compat.Trace{}, fmt.Errorf("%s: unknown case %q", infraNWTableIndexFAF559ID, caseName)
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableIndexFAF559Bean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableIndexFAF559JavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &infraNWTableIndexFAF559CaseState{
		env:            env,
		engine:         engine,
		spec:           spec,
		deployments:    make(map[string][]*esper.Deployment),
		deployedLabels: make(map[string]bool),
		caseName:       caseName,
	}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableIndexFAF559ID}
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
					infraNWTableIndexFAF559ID, step.Statement)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  1,
				Time:      "1970-01-01T00:00:00Z",
			})
		case "send":
			event, err := decodeInfraNWTableIndexFAF559Payload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return compat.Trace{}, err
			}
		case "snapshot":
			if err := state.snapshot(ctx, step, &trace); err != nil {
				return compat.Trace{}, err
			}
		case "unrepresentable":
			if err := state.unrepresentable(step, &trace); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s: unexpected op %q", infraNWTableIndexFAF559ID, step.Op)
		}
	}
	return trace, nil
}

// deploy maps each scenario label to the equivalent Go catalog call or
// chain-API plan: the create step registers the window/table at env level,
// the insert step deploys the OnEvent feed, and the index step verifies the
// creation-time secondary index is catalog-visible. The byte-exact EPL the
// Java execution passes to compileDeploy is pinned by the loader's step
// keys.
func (s *infraNWTableIndexFAF559CaseState) deploy(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "create":
		if step.Epl != s.spec.create {
			return s.deployDrift(step)
		}
		return s.deployCreate(step.Statement)
	case "insert":
		if step.Epl != s.spec.insert {
			return s.deployDrift(step)
		}
		return s.deployInsert(ctx, step.Statement)
	case "index":
		if step.Epl != s.spec.index {
			return s.deployDrift(step)
		}
		return s.deployIndex(step.Statement)
	default:
		return fmt.Errorf("%s: unknown deploy label %q", infraNWTableIndexFAF559ID, step.Statement)
	}
}

func (s *infraNWTableIndexFAF559CaseState) deployDrift(step compat.Step) error {
	return fmt.Errorf("%s: deploy %q does not pin the expected EPL %q",
		infraNWTableIndexFAF559ID, step.Statement, step.Epl)
}

// deployCreate mirrors a `create window`/`create table` deploy step: an
// env-level registration of the keepall window or the f1-keyed table. The
// composite (f2, f3, f1) hash index is declared here because plan.indexPlan
// is frozen at env.Build — a live CreateIndex would be invisible to FAF
// index selection, and the ordering divergence is observably identical (no
// rows exist before the index step).
func (s *infraNWTableIndexFAF559CaseState) deployCreate(label string) error {
	indexCols := []string{"f2", "f3", "f1"}
	if s.spec.namedWindow {
		stringT := reflect.TypeOf("")
		schema, err := esper.NewMapSchema(s.spec.infra+"559Schema", []esper.FieldSpec{
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
		if _, err := esper.CreateNamedWindow(s.env, s.spec.infra, schema,
			esper.NamedWindowRetention(esper.KeepAll()),
			esper.NamedWindowIndex(s.spec.indexName, indexCols...)); err != nil {
			return err
		}
	} else {
		if _, err := esper.CreateTable(s.env, s.spec.infra, []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("f1"),
			esper.TableColumnOf[int]("f2"),
			esper.TableColumnOf[string]("f3"),
			esper.TableColumnOf[string]("f4"),
		}, esper.SecondaryIndex(s.spec.indexName, indexCols...)); err != nil {
			return err
		}
	}
	s.deployedLabels[label] = true
	return nil
}

// deployInsert mirrors `insert into X(f1, f2, f3, f4) select theString,
// intPrimitive, '>'||theString||'<', '?'||theString||'?' from SupportBean`:
// the OnEvent trigger with SetColumn assignments into the window or table;
// the `||` concat operands map to Concat.
func (s *infraNWTableIndexFAF559CaseState) deployInsert(ctx context.Context, label string) error {
	theString := esper.Field[infraNWTableIndexFAF559Bean, string]("theString")
	assignments := []esper.TableAssignment{
		esper.SetColumn("f1", theString),
		esper.SetColumn("f2", esper.Field[infraNWTableIndexFAF559Bean, int]("intPrimitive")),
		esper.SetColumn("f3", esper.Concat(
			esper.Literal[string](">"), theString, esper.Literal[string]("<"))),
		esper.SetColumn("f4", esper.Concat(
			esper.Literal[string]("?"), theString, esper.Literal[string]("?"))),
	}
	source := esper.From[infraNWTableIndexFAF559Bean](s.env, "SupportBean")
	var plan esper.Plan
	var err error
	if s.spec.namedWindow {
		plan, err = s.env.Build(esper.OnEvent(source).InsertIntoNamedWindow(s.spec.infra, assignments...).Query())
	} else {
		plan, err = s.env.Build(esper.OnEvent(source).InsertIntoTable(s.spec.infra, assignments...).Query())
	}
	if err != nil {
		return err
	}
	return s.deployPlan(ctx, label, plan)
}

// deployIndex mirrors the late `create index X on infra(f2, f3, f1)` deploy
// step. The index is declared at creation time (see deployCreate): the step
// verifies the declared index is catalog-visible before the FAF probes run.
func (s *infraNWTableIndexFAF559CaseState) deployIndex(label string) error {
	if s.spec.namedWindow {
		window, ok := s.engine.NamedWindow(s.spec.infra)
		if !ok {
			return fmt.Errorf("%s named window is missing", s.spec.infra)
		}
		found := false
		for _, def := range window.Definition().Indexes() {
			if def.Name == s.spec.indexName {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s: declared index %q is not catalog-visible on window %s",
				infraNWTableIndexFAF559ID, s.spec.indexName, s.spec.infra)
		}
	} else {
		table, ok := s.engine.Table(s.spec.infra)
		if !ok {
			return fmt.Errorf("%s table is missing", s.spec.infra)
		}
		found := false
		for _, def := range table.Definition().Indexes() {
			if def.Name == s.spec.indexName {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s: declared index %q is not catalog-visible on table %s",
				infraNWTableIndexFAF559ID, s.spec.indexName, s.spec.infra)
		}
	}
	s.deployedLabels[label] = true
	return nil
}

func (s *infraNWTableIndexFAF559CaseState) deployPlan(ctx context.Context, label string,
	plan esper.Plan) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[label] = append(s.deployments[label], deployment)
	s.deployedLabels[label] = true
	return nil
}

// snapshot executes the pinned compileExecuteFAF read: the step's statement
// label selects the typed FAF plan and the epl field pins the Java query
// text. Rows are projected to the pinned f1..f4 fields and sorted — the
// Java assertPropsPerRow is any-order.
func (s *infraNWTableIndexFAF559CaseState) snapshot(ctx context.Context, step compat.Step,
	trace *compat.Trace) error {
	plan, err := s.buildFafSelect(step)
	if err != nil {
		return err
	}
	selection, ok := plan.IndexPlan().ForSource(0)
	if step.Statement == "select-full" {
		// The complete {f2,f3,f1} predicate set MUST resolve through the
		// declared composite index — the index-probe observable the Java
		// executions pin; a full scan here would hide a matcher regression.
		if !ok || selection.Access != esper.IndexAccessEquality ||
			selection.IndexName != s.spec.indexName ||
			!reflect.DeepEqual(selection.MatchedColumns, []string{"f2", "f3", "f1"}) {
			return fmt.Errorf("%s: faf %q resolved to %#v instead of the %s composite index",
				infraNWTableIndexFAF559ID, step.Statement, selection, s.spec.indexName)
		}
	} else if ok && selection.Access != esper.IndexAccessFullScan {
		// Spike resolution: Go's composite hash matcher requires predicates
		// on the leading index columns, so the f3-leading prefix probes
		// cannot consume the (f2,f3,f1) key and resolve to a full scan —
		// the same row set the Java assertions pin (Java's hash index
		// likewise requires its full column set). The assertion pins the
		// fallback rather than faking index use.
		return fmt.Errorf("%s: faf %q unexpectedly resolved to index %q (access %v) instead of a full scan",
			infraNWTableIndexFAF559ID, step.Statement, selection.IndexName, selection.Access)
	}
	result, err := s.engine.ExecuteFireAndForget(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: faf %q: %w", infraNWTableIndexFAF559ID, step.Statement, err)
	}
	rows := infraTableJoinNormalizeResults(result.Results())
	rows = projectInfraNWTableOnMergeRows(rows, []string{"f1", "f2", "f3", "f4"})
	sortRowsCanonical(rows)
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "snapshot",
		Statement: step.Statement,
		Sequence:  0,
		Time:      "1970-01-01T00:00:00Z",
		New:       rows,
	})
	return nil
}

// buildFafSelect maps the pinned FAF select EPL to the typed plan. The
// probes lead with the non-leading index column f3 exactly as the Java
// where-clauses order them.
func (s *infraNWTableIndexFAF559CaseState) buildFafSelect(step compat.Step) (esper.Plan, error) {
	var source esper.RecordStream
	if s.spec.namedWindow {
		source = esper.FromNamedWindow(s.env, s.spec.infra)
	} else {
		source = esper.FromTable(s.env, s.spec.infra)
	}
	f3 := esper.EqualOf(esper.Field[any, string]("f3"), esper.Literal[string](">E1<"))
	f2 := esper.EqualOf(esper.Field[any, int]("f2"), esper.Literal[int](-2))
	f1 := esper.EqualOf(esper.Field[any, string]("f1"), esper.Literal[string]("E1"))
	var filter esper.Expression[bool]
	switch step.Statement {
	case "select-f3":
		if step.Epl != s.spec.selects[0] {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = f3
	case "select-f3-f2":
		if step.Epl != s.spec.selects[1] {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = esper.And(f3, f2)
	case "select-full":
		if step.Epl != s.spec.selects[2] {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = esper.And(esper.And(f3, f2), f1)
	default:
		return esper.Plan{}, fmt.Errorf("%s: unknown faf select %q", infraNWTableIndexFAF559ID, step.Statement)
	}
	return s.env.Build(source.Filter(filter).Query())
}

func (s *infraNWTableIndexFAF559CaseState) fafDrift(step compat.Step) error {
	return fmt.Errorf("%s: faf select %q does not pin the expected EPL %q",
		infraNWTableIndexFAF559ID, step.Statement, step.Epl)
}

// unrepresentable emits the pinned records for the composite SODA tail:
// undeployModuleContaining("indexOne") has no Go deployment to retire and
// the eplToModelCompileDeploy IX2 probe has no statement-object boundary,
// so both records are plan-only.
func (s *infraNWTableIndexFAF559CaseState) unrepresentable(step compat.Step,
	trace *compat.Trace) error {
	var key, note string
	switch step.Statement {
	case "undeploy-index-one":
		key, note = infraNWTableCI559UndeployKey, infraNWTableCI559UndeployIndexNote
	case "soda-index-two":
		key, note = s.spec.soda, infraNWTableCI559SodaIndexTwoNote
	default:
		return fmt.Errorf("%s: unknown unrepresentable label %q", infraNWTableIndexFAF559ID, step.Statement)
	}
	if !s.spec.composite || step.Epl != key || step.ExpectError != note {
		return fmt.Errorf("%s: unrepresentable step %q is not pinned",
			infraNWTableIndexFAF559ID, step.Statement)
	}
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

func (s *infraNWTableIndexFAF559CaseState) undeployAll(ctx context.Context) error {
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

// decodeInfraNWTableIndexFAF559Payload mirrors the sendEventBean(
// SupportBean("E1", -2)) call: the payload carries theString plus
// intPrimitive; unmentioned fields stay zero.
func decodeInfraNWTableIndexFAF559Payload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported infra nwtable index-faf event type %q", step.EventType)
	}
	var value infraNWTableIndexFAF559Bean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}

// loadInfraNWTableIndexFAF559Scenario enforces the strict scenario contract
// shared by the differential runners: no duplicate or unknown JSON fields,
// pinned metadata, pinned per-case runtime/execution/EPL, and a per-op step
// field whitelist followed by a full step-shape pin.
func loadInfraNWTableIndexFAF559Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableIndexFAF559ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableIndexFAF559ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableIndexFAF559ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableIndexFAF559ID, err)
	}
	if err := requireInfraNWTableIndexFAF559Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableIndexFAF559ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableIndexFAF559ID ||
		metadata.Description != infraNWTableIndexFAF559Description ||
		metadata.JavaCommit != infraNWTableIndexFAF559JavaCommit ||
		metadata.JavaSource != infraNWTableIndexFAF559JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableIndexFAF559ID)
	}
	if err := validateInfraNWTableIndexFAF559StringArray(root["javaRuntimes"], infraNWTableIndexFAF559JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableIndexFAF559StringArray(root["javaNames"], infraNWTableIndexFAF559JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableIndexFAF559StringArray(root["javaStaticIds"], infraNWTableIndexFAF559JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableIndexFAF559StringArray(root["javaFlags"], infraNWTableIndexFAF559JavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableIndexFAF559Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableIndexFAF559ID, len(infraNWTableIndexFAF559Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableIndexFAF559Fields(object,
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
		if definition.Case != infraNWTableIndexFAF559Cases[index] ||
			definition.Ordinal != infraNWTableIndexFAF559Ordinals[index] ||
			definition.RuntimeID != infraNWTableIndexFAF559JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableIndexFAF559JavaExecutions[index] ||
			definition.Observation != infraNWTableIndexFAF559CaseObservations[index] ||
			definition.EPL != infraNWTableIndexFAF559CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableIndexFAF559ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableIndexFAF559ID)
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
			if err := requireInfraNWTableIndexFAF559Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableIndexFAF559Fields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableIndexFAF559Fields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableIndexFAF559Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableIndexFAF559Payload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableIndexFAF559Fields(object, "op", "case", "statement", "mode", "fields", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "unrepresentable":
			if err := requireInfraNWTableIndexFAF559Fields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableIndexFAF559Fields(object, "op", "case"); err != nil {
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
		for _, name := range infraNWTableIndexFAF559Cases {
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
	if err := validateInfraNWTableIndexFAF559RawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraNWTableIndexFAF559RawSteps pins the complete step sequence
// per case against the raw JSON objects: deploy steps with byte-exact EPL,
// deployed markers, the SupportBean send, snapshot reads carrying the
// pinned FAF text, the SODA unrepresentable records and the undeploy-all
// terminators. Java milestone(0..2) checkpoints carry no steps.
func validateInfraNWTableIndexFAF559RawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableIndexFAF559Cases {
		want, ok := infraNWTableIndexFAF559CaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableIndexFAF559ID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableIndexFAF559ID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableIndexFAF559ID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableIndexFAF559ID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableIndexFAF559StepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableIndexFAF559ID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q",
					infraNWTableIndexFAF559ID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableIndexFAF559ID)
	}
	return nil
}

// infraNWTableIndexFAF559StepKey renders a raw step object into its pinned
// string form. Fields are read from the raw JSON because compat.Step does
// not carry the fields array.
func infraNWTableIndexFAF559StepKey(object map[string]json.RawMessage, operation string) (string, error) {
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
	case "unrepresentable":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		epl, err := stringField("epl")
		if err != nil {
			return "", err
		}
		note, err := stringField("expectError")
		if err != nil {
			return "", err
		}
		return "unrepresentable:" + statement + ":" + epl + ":" + note, nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// composite559CaseSteps renders the pinned step sequence of
// InfraCompositeIndex.run (InfraNWTableCreateIndex.java lines 430-457): the
// create/insert/index deploys, the SupportBean("E1",-2) send, the three
// f3-leading FAF probes, the plan-only undeployModuleContaining("indexOne")
// and SODA IX2 records, then the undeployAll terminator.
func composite559CaseSteps(spec infraNWTableIndexFAF559CaseSpec) []string {
	return []string{
		"deploy:create:" + spec.create,
		"deployed:create",
		"deploy:insert:" + spec.insert,
		"deployed:insert",
		"deploy:index:" + spec.index,
		"deployed:index",
		`send:SupportBean:{"intPrimitive":-2,"theString":"E1"}`,
		"snapshot:select-f3:any:f1,f2,f3,f4:" + spec.selects[0],
		"snapshot:select-f3-f2:any:f1,f2,f3,f4:" + spec.selects[1],
		"snapshot:select-full:any:f1,f2,f3,f4:" + spec.selects[2],
		"unrepresentable:undeploy-index-one:" + infraNWTableCI559UndeployKey + ":" + infraNWTableCI559UndeployIndexNote,
		"unrepresentable:soda-index-two:" + spec.soda + ":" + infraNWTableCI559SodaIndexTwoNote,
		"undeploy-all",
	}
}

// multikey559CaseSteps renders the pinned step sequence of
// InfraMultikeyIndexFAF.run (InfraNWTableCreateIndex.java lines 645-672):
// the create/insert/index deploys, the SupportBean("E1",-2) send, the three
// f3-leading FAF probes (milestone(0..2) checkpoints carry no steps) and
// undeployAll.
func multikey559CaseSteps(spec infraNWTableIndexFAF559CaseSpec) []string {
	return []string{
		"deploy:create:" + spec.create,
		"deployed:create",
		"deploy:insert:" + spec.insert,
		"deployed:insert",
		"deploy:index:" + spec.index,
		"deployed:index",
		`send:SupportBean:{"intPrimitive":-2,"theString":"E1"}`,
		"snapshot:select-f3:any:f1,f2,f3,f4:" + spec.selects[0],
		"snapshot:select-f3-f2:any:f1,f2,f3,f4:" + spec.selects[1],
		"snapshot:select-full:any:f1,f2,f3,f4:" + spec.selects[2],
		"undeploy-all",
	}
}

// infraNWTableIndexFAF559CaseSteps pins the exact op sequence per case.
var infraNWTableIndexFAF559CaseSteps = map[string][]string{
	"composite-window": composite559CaseSteps(infraNWTableIndexFAF559CaseSpecs["composite-window"]),
	"composite-table":  composite559CaseSteps(infraNWTableIndexFAF559CaseSpecs["composite-table"]),
	"multikey-window":  multikey559CaseSteps(infraNWTableIndexFAF559CaseSpecs["multikey-window"]),
	"multikey-table":   multikey559CaseSteps(infraNWTableIndexFAF559CaseSpecs["multikey-table"]),
}

func requireInfraNWTableIndexFAF559Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraNWTableIndexFAF559ID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraNWTableIndexFAF559ID, name)
		}
	}
	return nil
}

func validateInfraNWTableIndexFAF559StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
