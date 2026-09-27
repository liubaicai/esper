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

// infra_nwtable_mrak_562.go replays InfraNWTableCreateIndex ordinals 0-1
// against the pinned Java oracle: the InfraMultiRangeAndKey multi-range-and-
// key fire-and-forget probes over a mixed-kind index.
//
//   - mrak-window/mrak-table (ords 0-1, InfraMultiRangeAndKey
//     {namedWindow=true/false}, FIREANDFORGET): keepall window or
//     (id,rangeStartLong,rangeEndLong)-keyed table MyInfraMRAK fed by
//     `insert into MyInfraMRAK select * from SupportBeanRange` (window) or
//     `on SupportBeanRange t0 merge MyInfraMRAK t1 where t0.id = t1.id when
//     not matched then insert select id, key, keyLong, rangeStartLong,
//     rangeEndLong` (table), the mixed-kind index idx1 (key hash, keyLong
//     hash, rangeStartLong btree, rangeEndLong btree), then four
//     compileExecuteFAF runs of query1 `rangeStartLong > 1 and
//     rangeEndLong > 2 and keyLong=1 and key='K1' order by id asc` around the
//     makeLong sends E1/K1/1/2/3, E2/K1/1/2/4, E3/K1/1/3/3 (milestone(0)
//     between E2 and E3 is a harness no-op carrying no step) plus one run of
//     query2 (query1 minus key='K1'), returning {}, {E1}, {E1,E2},
//     {E1,E2,E3} and {E1,E2,E3}. The tail asserts getIndexCount("create",
//     "MyInfraMRAK") == 1 (window) / 2 (table) before undeployAll.
//
// Index-resolution spike (verified against internal/esper/index_plan.go):
// Go's btree matcher consumes an equality prefix in declaration order and
// stops at the first range column, so the keyed query resolves idx1 as
// IndexAccessRange with MatchedColumns [key keyLong rangeStartLong] —
// rangeEndLong>2 falls through to the post-filter (there is no second range
// leg on a single-kind index). The key-less query cannot start the prefix at
// keyLong (the matcher requires the leading column), so both probes' row
// sets are observably identical to Java's even though the second full-scans.
// On the table variant the composite <primary-key> hash candidate cannot
// satisfy either probe (no id predicate), leaving idx1 / full scan.
//
// Index-kind divergence (documented): Java declares idx1 with per-column
// kinds — key hash, keyLong hash, rangeStartLong btree, rangeEndLong btree —
// while Go's NamedWindowIndexDefinition/TableIndexDefinition carry a single
// Kind per index (internal/esper/state.go). The replay declares idx1 as a
// creation-time btree composite (key,keyLong,rangeStartLong,rangeEndLong):
// hash legs would force a complete-key match and lose the range leg. Rows
// are identical; the kind split rides the plan-only unrepresentable
// "index-kind" record, the same record shape 4.561 used for the count
// divergences.
//
// Approved differences (observably identical to the Java EPL):
//   - `create window`/`create table`/`create index` are catalog operations:
//     the deploy steps carry the byte-exact EPL while the Go side performs
//     env-level window/table registration; idx1 is declared at creation time
//     because plan.indexPlan is frozen at env.Build — a live CreateIndex
//     would be invisible to FAF index selection (observably identical: every
//     row the index serves is equally visible to a creation-time index).
//   - `insert into MyInfraMRAK select *` maps to the plain stream InsertInto
//     (no projection: the bean's underlying value is preserved); the table's
//     merge-not-matched feed maps to MergeInsertIntoTable. Go's table merge
//     requires one key expression per primary-key column, so the key list
//     carries (id,rangeStartLong,rangeEndLong) where Java's where clause
//     matches on id alone — observably identical here because every trigger
//     bean is unmatched under either key shape.
//   - `env.compileExecuteFAF` rides `snapshot` steps that carry the pinned
//     query EPL in mode "ordered" — the `order by id asc` clause fixes the
//     row order positionally, so neither side canonicalizes.
//   - env.milestone(0) is a harness no-op and carries no step.
//   - getIndexCount(env, namedWindow, "create", "MyInfraMRAK") maps to
//     NamedWindow.IndexCount (window: declared idx1 = 1) or
//     len(Table.Definition().Indexes()) + the implicit primary-key
//     descriptor (table: 2), so the tail rides a real index-count record.
//   - sendEventBean(SupportBeanRange.makeLong(id,key,keyLong,rangeStartLong,
//     rangeEndLong)) maps to payloads carrying those five bean fields.

// infraNWTableMRAK562Bean mirrors the SupportBeanRange properties the
// five-arg makeLong factory populates.
type infraNWTableMRAK562Bean struct {
	ID             string `esper:"id"`
	Key            string `esper:"key"`
	KeyLong        int64  `esper:"keyLong"`
	RangeStartLong int64  `esper:"rangeStartLong"`
	RangeEndLong   int64  `esper:"rangeEndLong"`
}

const (
	infraNWTableMRAK562ID         = "infra-nwtable-mrak-562"
	infraNWTableMRAK562JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableMRAK562JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java"
)

const infraNWTableMRAK562Description = "InfraNWTableCreateIndex ordinals 0-1: InfraMultiRangeAndKey {namedWindow=true/false} deploys a SupportBeanRange keepall window or the (id,rangeStartLong,rangeEndLong)-keyed table MyInfraMRAK fed by the select-* insert-into (window) or the merge-not-matched insert (table) plus the mixed-kind index idx1 (key hash, keyLong hash, rangeStartLong btree, rangeEndLong btree), then runs query1 (rangeStartLong>1, rangeEndLong>2, keyLong=1, key='K1', order by id asc) four times around the E1/E2/E3 sends with milestone(0) between E2 and E3 (a harness no-op carrying no step) and query2 (query1 minus key='K1') once, returning {}, {E1}, {E1,E2}, {E1,E2,E3} and {E1,E2,E3}; the mixed index kind has no Go single-Kind form and rides a plan-only record, and the tail asserts getIndexCount == 1 (window) / 2 (table). (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java)."

// Verbatim transcriptions of InfraNWTableCreateIndex.java lines 586-620
// (ords 0-1, InfraMultiRangeAndKey.run).
const (
	infraNWTableMRAK562CreateWindow = "@name('create') @public create window MyInfraMRAK#keepall as SupportBeanRange"
	infraNWTableMRAK562CreateTable  = "@name('create') @public create table MyInfraMRAK(id string primary key, key string, keyLong long, rangeStartLong long primary key, rangeEndLong long primary key)"
	infraNWTableMRAK562InsertWindow = "insert into MyInfraMRAK select * from SupportBeanRange"
	infraNWTableMRAK562InsertTable  = "on SupportBeanRange t0 merge MyInfraMRAK t1 where t0.id = t1.id when not matched then insert select id, key, keyLong, rangeStartLong, rangeEndLong"
	infraNWTableMRAK562Index        = "create index idx1 on MyInfraMRAK(key hash, keyLong hash, rangeStartLong btree, rangeEndLong btree)"
	infraNWTableMRAK562Query1       = "select * from MyInfraMRAK where rangeStartLong > 1 and rangeEndLong > 2 and keyLong=1 and key='K1' order by id asc"
	infraNWTableMRAK562Query2       = "select * from MyInfraMRAK where rangeStartLong > 1 and rangeEndLong > 2 and keyLong=1 order by id asc"
)

// infraNWTableMRAK562IndexKindNote is the plan-divergence note pinned by the
// unrepresentable index-kind step: Java's per-column kinds (two hash plus two
// btree legs) have no Go single-Kind form, so the replay declares idx1 as a
// creation-time btree composite; identical rows keep the record plan-only.
const infraNWTableMRAK562IndexKindNote = "create index idx1 on MyInfraMRAK(key hash, keyLong hash, rangeStartLong btree, rangeEndLong btree): Java declares per-column kinds (two hash plus two btree legs) while Go indexes carry a single Kind; the replay declares idx1 as a creation-time btree composite (key,keyLong,rangeStartLong,rangeEndLong) - identical rows (Go consumes the key+keyLong equality prefix and the rangeStartLong range leg; rangeEndLong>2 post-filters) so the kind split is plan-only"

var (
	infraNWTableMRAK562JavaSources = []string{
		infraNWTableMRAK562JavaSource,
	}
	infraNWTableMRAK562JavaRuntimeIDs = []string{
		"java-runtime-7242c335a588f1d6fa15",
		"java-runtime-4d509bfd0e311be23afe",
	}
	infraNWTableMRAK562JavaExecutions = []string{
		"InfraMultiRangeAndKey{namedWindow=true}",
		"InfraMultiRangeAndKey{namedWindow=false}",
	}
	infraNWTableMRAK562JavaStaticIDs = []string{
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
	}
	infraNWTableMRAK562JavaFlags = []string{"FIREANDFORGET"}
	infraNWTableMRAK562Cases     = []string{
		"mrak-window",
		"mrak-table",
	}
	infraNWTableMRAK562Ordinals = []int{0, 1}
)

// infraNWTableMRAK562CaseEPLs pins the newline-joined EPL of every
// EPL-bearing step in the case, in step order — the value carried by the
// scenario cases[] metadata. The index deploy and the index-kind
// unrepresentable step both carry the create-index text, so idx1's EPL
// appears twice.
var infraNWTableMRAK562CaseEPLs = []string{
	strings.Join([]string{
		infraNWTableMRAK562CreateWindow,
		infraNWTableMRAK562InsertWindow,
		infraNWTableMRAK562Index,
		infraNWTableMRAK562Index,
		infraNWTableMRAK562Query1,
		infraNWTableMRAK562Query1,
		infraNWTableMRAK562Query1,
		infraNWTableMRAK562Query1,
		infraNWTableMRAK562Query2,
	}, "\n"),
	strings.Join([]string{
		infraNWTableMRAK562CreateTable,
		infraNWTableMRAK562InsertTable,
		infraNWTableMRAK562Index,
		infraNWTableMRAK562Index,
		infraNWTableMRAK562Query1,
		infraNWTableMRAK562Query1,
		infraNWTableMRAK562Query1,
		infraNWTableMRAK562Query1,
		infraNWTableMRAK562Query2,
	}, "\n"),
}

var infraNWTableMRAK562CaseObservations = []string{
	"deploy+send+snapshot+index-count+unrepresentable; SupportBeanRange MyInfraMRAK#keepall window fed by the select-* insert-into with mixed-kind index idx1 (key hash, keyLong hash, rangeStartLong btree, rangeEndLong btree) replayed as a single btree composite (plan-only record): the keyed FAF probe resolves idx1's equality prefix plus the rangeStartLong range leg (rangeEndLong post-filters) while the key-less probe full-scans; both return {E1},{E1,E2},{E1,E2,E3}; index-count 1",
	"deploy+send+snapshot+index-count+unrepresentable; (id,rangeStartLong,rangeEndLong)-keyed MyInfraMRAK table fed by the merge-not-matched insert with mixed-kind index idx1 replayed as a single btree composite (plan-only record): the keyed FAF probe resolves idx1's equality prefix plus the rangeStartLong range leg while the key-less probe full-scans; both return {E1},{E1,E2},{E1,E2,E3}; index-count 2 (idx1 plus the implicit primary-key descriptor)",
}

// infraNWTableMRAK562CaseSpec carries the per-case fixture constants:
// whether the named-window or table variant runs plus the byte-exact
// create/insert EPLs (the index and query texts are shared).
type infraNWTableMRAK562CaseSpec struct {
	namedWindow bool
	create      string
	insert      string
}

var infraNWTableMRAK562CaseSpecs = map[string]infraNWTableMRAK562CaseSpec{
	"mrak-window": {
		namedWindow: true,
		create:      infraNWTableMRAK562CreateWindow, insert: infraNWTableMRAK562InsertWindow,
	},
	"mrak-table": {
		namedWindow: false,
		create:      infraNWTableMRAK562CreateTable, insert: infraNWTableMRAK562InsertTable,
	},
}

// infraNWTableMRAK562IndexColumns is the single-kind replay of the mixed
// (key hash, keyLong hash, rangeStartLong btree, rangeEndLong btree) index.
var infraNWTableMRAK562IndexColumns = []string{"key", "keyLong", "rangeStartLong", "rangeEndLong"}

// infraNWTableMRAK562CaseState carries the per-case replay state: the
// environment/engine pair, the label→deployments bookkeeping used by
// undeploy-all, the deployed-label set for marker checks and the index-count
// sequencing map.
type infraNWTableMRAK562CaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	spec           infraNWTableMRAK562CaseSpec
	deployments    map[string][]*esper.Deployment
	deployedLabels map[string]bool
	indexSeq       map[string]uint64
	caseName       string
}

// runInfraNWTableMRAK562Scenario replays the two InfraMultiRangeAndKey
// executions: each case runs on a fresh environment/engine pair (one runtime
// per Java execution) and every step dispatches to the matching runtime
// action. The oracle emits the epoch time for every timed record, so the
// runner pins the same value.
func runInfraNWTableMRAK562Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableMRAK562Cases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableMRAK562Case(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableMRAK562ID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableMRAK562ID)
	}
	return trace, nil
}

func runInfraNWTableMRAK562Case(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	spec, ok := infraNWTableMRAK562CaseSpecs[caseName]
	if !ok {
		return compat.Trace{}, fmt.Errorf("%s: unknown case %q", infraNWTableMRAK562ID, caseName)
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableMRAK562Bean](env, "SupportBeanRange"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableMRAK562JavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &infraNWTableMRAK562CaseState{
		env:            env,
		engine:         engine,
		spec:           spec,
		deployments:    make(map[string][]*esper.Deployment),
		deployedLabels: make(map[string]bool),
		indexSeq:       make(map[string]uint64),
		caseName:       caseName,
	}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableMRAK562ID}
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
					infraNWTableMRAK562ID, step.Statement)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  1,
				Time:      "1970-01-01T00:00:00Z",
			})
		case "send":
			event, err := decodeInfraNWTableMRAK562Payload(step)
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
		case "index-count":
			if err := state.indexCount(step, &trace); err != nil {
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
			return compat.Trace{}, fmt.Errorf("%s: unexpected op %q", infraNWTableMRAK562ID, step.Op)
		}
	}
	return trace, nil
}

// deploy maps each scenario label to the equivalent Go catalog call or
// chain-API plan: the create step registers the window/table at env level,
// the insert step deploys the feed, and the index step verifies the
// creation-time secondary index is catalog-visible. The byte-exact EPL the
// Java execution passes to compileDeploy is pinned by the loader's step
// keys.
func (s *infraNWTableMRAK562CaseState) deploy(ctx context.Context, step compat.Step) error {
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
		if step.Epl != infraNWTableMRAK562Index {
			return s.deployDrift(step)
		}
		return s.deployIndex(step.Statement)
	default:
		return fmt.Errorf("%s: unknown deploy label %q", infraNWTableMRAK562ID, step.Statement)
	}
}

func (s *infraNWTableMRAK562CaseState) deployDrift(step compat.Step) error {
	return fmt.Errorf("%s: deploy %q does not pin the expected EPL %q",
		infraNWTableMRAK562ID, step.Statement, step.Epl)
}

// deployCreate mirrors a `create window`/`create table` deploy step: an
// env-level registration of the bean-schema keepall window or the
// (id,rangeStartLong,rangeEndLong)-keyed table. The idx1 index is declared
// here because plan.indexPlan is frozen at env.Build — a live CreateIndex
// would be invisible to FAF index selection, and the ordering divergence is
// observably identical (no rows exist before the index step).
func (s *infraNWTableMRAK562CaseState) deployCreate(label string) error {
	if s.spec.namedWindow {
		schema, ok := s.env.Schema("SupportBeanRange")
		if !ok {
			return fmt.Errorf("%s: SupportBeanRange schema is missing", infraNWTableMRAK562ID)
		}
		if _, err := esper.CreateNamedWindow(s.env, "MyInfraMRAK", schema,
			esper.NamedWindowRetention(esper.KeepAll()),
			esper.NamedWindowBTreeIndex("idx1", infraNWTableMRAK562IndexColumns...)); err != nil {
			return err
		}
	} else {
		if _, err := esper.CreateTable(s.env, "MyInfraMRAK", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("id"),
			esper.TableColumnOf[string]("key"),
			esper.TableColumnOf[int64]("keyLong"),
			esper.PrimaryKeyColumn[int64]("rangeStartLong"),
			esper.PrimaryKeyColumn[int64]("rangeEndLong"),
		}, esper.SecondaryBTreeIndex("idx1", infraNWTableMRAK562IndexColumns...)); err != nil {
			return err
		}
	}
	s.deployedLabels[label] = true
	return nil
}

// deployInsert mirrors the feed statement: the window's `insert into
// MyInfraMRAK select *` maps to a plain stream InsertInto (the bean's
// underlying value is preserved), while the table's merge-not-matched insert
// maps to MergeInsertIntoTable. Go's table merge requires one key expression
// per primary-key column, so the key list carries (id,rangeStartLong,
// rangeEndLong) where Java's where clause matches on t0.id = t1.id alone —
// observably identical because every trigger bean is unmatched under either
// key shape.
func (s *infraNWTableMRAK562CaseState) deployInsert(ctx context.Context, label string) error {
	source := esper.From[infraNWTableMRAK562Bean](s.env, "SupportBeanRange")
	var plan esper.Plan
	var err error
	if s.spec.namedWindow {
		plan, err = s.env.Build(source.InsertInto("MyInfraMRAK"))
	} else {
		plan, err = s.env.Build(esper.OnEvent(source).MergeInsertIntoTable("MyInfraMRAK",
			[]esper.Expr{
				esper.Field[infraNWTableMRAK562Bean, string]("id"),
				esper.Field[infraNWTableMRAK562Bean, int64]("rangeStartLong"),
				esper.Field[infraNWTableMRAK562Bean, int64]("rangeEndLong"),
			},
			esper.SetColumn("id", esper.Field[infraNWTableMRAK562Bean, string]("id")),
			esper.SetColumn("key", esper.Field[infraNWTableMRAK562Bean, string]("key")),
			esper.SetColumn("keyLong", esper.Field[infraNWTableMRAK562Bean, int64]("keyLong")),
			esper.SetColumn("rangeStartLong", esper.Field[infraNWTableMRAK562Bean, int64]("rangeStartLong")),
			esper.SetColumn("rangeEndLong", esper.Field[infraNWTableMRAK562Bean, int64]("rangeEndLong")),
		).Query())
	}
	if err != nil {
		return err
	}
	return s.deployPlan(ctx, label, plan)
}

// deployIndex mirrors the `create index idx1` deploy step. The index is
// declared at creation time (see deployCreate): the step verifies the
// declared btree composite is catalog-visible before the FAF probes run.
func (s *infraNWTableMRAK562CaseState) deployIndex(label string) error {
	if s.spec.namedWindow {
		window, ok := s.engine.NamedWindow("MyInfraMRAK")
		if !ok {
			return fmt.Errorf("%s named window is missing", "MyInfraMRAK")
		}
		found := false
		for _, def := range window.Definition().Indexes() {
			if def.Name == "idx1" && def.Kind == esper.IndexBTree &&
				reflect.DeepEqual(def.Columns, infraNWTableMRAK562IndexColumns) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s: declared index %q is not catalog-visible on window %s",
				infraNWTableMRAK562ID, "idx1", "MyInfraMRAK")
		}
	} else {
		table, ok := s.engine.Table("MyInfraMRAK")
		if !ok {
			return fmt.Errorf("%s table is missing", "MyInfraMRAK")
		}
		found := false
		for _, def := range table.Definition().Indexes() {
			if def.Name == "idx1" && def.Kind == esper.IndexBTree &&
				reflect.DeepEqual(def.Columns, infraNWTableMRAK562IndexColumns) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s: declared index %q is not catalog-visible on table %s",
				infraNWTableMRAK562ID, "idx1", "MyInfraMRAK")
		}
	}
	s.deployedLabels[label] = true
	return nil
}

func (s *infraNWTableMRAK562CaseState) deployPlan(ctx context.Context, label string,
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
// text. The planned index selection is asserted honestly per probe; rows are
// projected to the pinned id field in result order — the `order by id asc`
// clause fixes ordering positionally (mode "ordered"), so neither side
// canonicalizes.
func (s *infraNWTableMRAK562CaseState) snapshot(ctx context.Context, step compat.Step,
	trace *compat.Trace) error {
	plan, err := s.buildFafSelect(step)
	if err != nil {
		return err
	}
	selection, ok := plan.IndexPlan().ForSource(0)
	if step.Statement == "select-q2" {
		// Spike resolution: dropping key='K1' leaves no predicate on the
		// leading idx1 column, so the btree composite cannot start its
		// equality prefix and the probe resolves to a full scan — the same
		// row set the Java assertion pins. The assertion pins the fallback
		// rather than faking index use.
		if ok && selection.Access != esper.IndexAccessFullScan {
			return fmt.Errorf("%s: faf %q unexpectedly resolved to index %q (access %v) instead of a full scan",
				infraNWTableMRAK562ID, step.Statement, selection.IndexName, selection.Access)
		}
	} else {
		// The keyed probe MUST resolve through idx1: Go's btree matcher
		// consumes the (key,keyLong) equality prefix and stops at the first
		// range column, so rangeStartLong>1 is the range leg and
		// rangeEndLong>2 post-filters. A full scan here would hide a matcher
		// regression.
		if !ok || selection.IndexName != "idx1" ||
			selection.Access != esper.IndexAccessRange ||
			selection.Backing != esper.IndexBackingBTree ||
			!reflect.DeepEqual(selection.MatchedColumns, []string{"key", "keyLong", "rangeStartLong"}) {
			return fmt.Errorf("%s: faf %q resolved to %#v instead of the idx1 btree equality-prefix+range selection",
				infraNWTableMRAK562ID, step.Statement, selection)
		}
	}
	result, err := s.engine.ExecuteFireAndForget(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: faf %q: %w", infraNWTableMRAK562ID, step.Statement, err)
	}
	rows := infraTableJoinNormalizeResults(result.Results())
	rows = projectInfraNWTableOnMergeRows(rows, []string{"id"})
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

// buildFafSelect maps the pinned FAF select EPL to the typed plan; the probe
// conjuncts follow the Java where-clause order exactly and the `order by id
// asc` tail maps to the ascending id sort key.
func (s *infraNWTableMRAK562CaseState) buildFafSelect(step compat.Step) (esper.Plan, error) {
	var source esper.RecordStream
	if s.spec.namedWindow {
		source = esper.FromNamedWindow(s.env, "MyInfraMRAK")
	} else {
		source = esper.FromTable(s.env, "MyInfraMRAK")
	}
	rangeStart := esper.Greater[int64](esper.Field[any, int64]("rangeStartLong"), esper.Literal[int64](1))
	rangeEnd := esper.Greater[int64](esper.Field[any, int64]("rangeEndLong"), esper.Literal[int64](2))
	keyLong := esper.EqualOf(esper.Field[any, int64]("keyLong"), esper.Literal[int64](1))
	key := esper.EqualOf(esper.Field[any, string]("key"), esper.Literal[string]("K1"))
	var filter esper.Expression[bool]
	switch step.Statement {
	case "select-q1-empty", "select-q1-e1", "select-q1-e2", "select-q1-e3":
		if step.Epl != infraNWTableMRAK562Query1 {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = esper.And(esper.And(esper.And(rangeStart, rangeEnd), keyLong), key)
	case "select-q2":
		if step.Epl != infraNWTableMRAK562Query2 {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = esper.And(esper.And(rangeStart, rangeEnd), keyLong)
	default:
		return esper.Plan{}, fmt.Errorf("%s: unknown faf select %q", infraNWTableMRAK562ID, step.Statement)
	}
	return s.env.Build(source.Filter(filter).Query(
		esper.OrderBy(esper.Ascending(esper.Field[any, string]("id")))))
}

func (s *infraNWTableMRAK562CaseState) fafDrift(step compat.Step) error {
	return fmt.Errorf("%s: faf select %q does not pin the expected EPL %q",
		infraNWTableMRAK562ID, step.Statement, step.Epl)
}

// indexCount mirrors the getIndexCount(env, namedWindow, "create",
// "MyInfraMRAK") tail assert: the window reports IndexCount (declared idx1,
// no trigger-inferred lookups) while the table reports its declared
// secondary index plus the implicit primary-key descriptor Java counts. Both
// counts coincide with the Java-asserted values, so the record is real.
func (s *infraNWTableMRAK562CaseState) indexCount(step compat.Step, trace *compat.Trace) error {
	if step.Statement != "MyInfraMRAK" || step.Create != "create" || step.Of != "indexes" {
		return fmt.Errorf("%s: index-count step %#v is not pinned", infraNWTableMRAK562ID, step.Statement)
	}
	if step.Count == nil {
		return fmt.Errorf("%s: index-count %q has no count", infraNWTableMRAK562ID, step.Statement)
	}
	count, err := s.observedIndexCount("MyInfraMRAK")
	if err != nil {
		return err
	}
	if count != *step.Count {
		return fmt.Errorf("%s: index-count mismatch for %s (indexes): expected %d, got %d",
			infraNWTableMRAK562ID, step.Statement, *step.Count, count)
	}
	// The oracle sequences index-count records per infra name.
	s.indexSeq[step.Statement]++
	trace.Records = append(trace.Records, compat.TraceRecord{
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
func (s *infraNWTableMRAK562CaseState) observedIndexCount(infra string) (int64, error) {
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
	return 0, fmt.Errorf("%s: index-count targets unknown infra %q", infraNWTableMRAK562ID, infra)
}

// unrepresentable emits the pinned plan-only record for the mixed-kind index
// declaration: Java's per-column kinds have no Go single-Kind form, so the
// replay's creation-time btree composite is documented rather than deployed
// as a mixed index.
func (s *infraNWTableMRAK562CaseState) unrepresentable(step compat.Step,
	trace *compat.Trace) error {
	if step.Statement != "index-kind" || step.Epl != infraNWTableMRAK562Index ||
		step.ExpectError != infraNWTableMRAK562IndexKindNote {
		return fmt.Errorf("%s: unrepresentable step %q is not pinned",
			infraNWTableMRAK562ID, step.Statement)
	}
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

func (s *infraNWTableMRAK562CaseState) undeployAll(ctx context.Context) error {
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

// decodeInfraNWTableMRAK562Payload mirrors the sendEventBean(
// SupportBeanRange.makeLong(id,key,keyLong,rangeStartLong,rangeEndLong))
// calls: the payload carries those five bean fields; Java's other
// SupportBeanRange properties stay unset on both sides.
func decodeInfraNWTableMRAK562Payload(step compat.Step) (any, error) {
	if step.EventType != "SupportBeanRange" {
		return nil, fmt.Errorf("unsupported infra nwtable mrak event type %q", step.EventType)
	}
	var value infraNWTableMRAK562Bean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBeanRange: %w", err)
	}
	return value, nil
}

// loadInfraNWTableMRAK562Scenario enforces the strict scenario contract
// shared by the differential runners: no duplicate or unknown JSON fields,
// pinned metadata, pinned per-case runtime/execution/EPL, and a per-op step
// field whitelist followed by a full step-shape pin.
func loadInfraNWTableMRAK562Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableMRAK562ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableMRAK562ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableMRAK562ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableMRAK562ID, err)
	}
	if err := requireInfraNWTableMRAK562Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableMRAK562ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableMRAK562ID ||
		metadata.Description != infraNWTableMRAK562Description ||
		metadata.JavaCommit != infraNWTableMRAK562JavaCommit ||
		metadata.JavaSource != infraNWTableMRAK562JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableMRAK562ID)
	}
	if err := validateInfraNWTableMRAK562StringArray(root["javaRuntimes"], infraNWTableMRAK562JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableMRAK562StringArray(root["javaNames"], infraNWTableMRAK562JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableMRAK562StringArray(root["javaStaticIds"], infraNWTableMRAK562JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableMRAK562StringArray(root["javaFlags"], infraNWTableMRAK562JavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableMRAK562Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableMRAK562ID, len(infraNWTableMRAK562Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableMRAK562Fields(object,
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
		if definition.Case != infraNWTableMRAK562Cases[index] ||
			definition.Ordinal != infraNWTableMRAK562Ordinals[index] ||
			definition.RuntimeID != infraNWTableMRAK562JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableMRAK562JavaExecutions[index] ||
			definition.Observation != infraNWTableMRAK562CaseObservations[index] ||
			definition.EPL != infraNWTableMRAK562CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableMRAK562ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableMRAK562ID)
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
			if err := requireInfraNWTableMRAK562Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableMRAK562Fields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableMRAK562Fields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableMRAK562Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableMRAK562Payload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableMRAK562Fields(object, "op", "case", "statement", "mode", "fields", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "index-count":
			if err := requireInfraNWTableMRAK562Fields(object, "op", "case", "statement", "create", "of", "count"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "unrepresentable":
			if err := requireInfraNWTableMRAK562Fields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableMRAK562Fields(object, "op", "case"); err != nil {
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
		for _, name := range infraNWTableMRAK562Cases {
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
	if err := validateInfraNWTableMRAK562RawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraNWTableMRAK562RawSteps pins the complete step sequence per
// case against the raw JSON objects: deploy steps with byte-exact EPL,
// deployed markers, the SupportBeanRange makeLong sends, the ordered FAF
// snapshot reads, the index-kind plan-divergence record, the index-count
// assert and the undeploy-all terminators. Java milestone(0) carries no
// step.
func validateInfraNWTableMRAK562RawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableMRAK562Cases {
		want, ok := infraNWTableMRAK562CaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableMRAK562ID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableMRAK562ID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableMRAK562ID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableMRAK562ID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableMRAK562StepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableMRAK562ID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q",
					infraNWTableMRAK562ID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableMRAK562ID)
	}
	return nil
}

// infraNWTableMRAK562StepKey renders a raw step object into its pinned
// string form. Fields are read from the raw JSON because compat.Step does
// not carry the fields array.
func infraNWTableMRAK562StepKey(object map[string]json.RawMessage, operation string) (string, error) {
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

// mrak562CaseSteps renders the pinned step sequence of
// InfraMultiRangeAndKey.run (InfraNWTableCreateIndex.java lines 586-626):
// the create/insert/index deploys, the plan-only index-kind record, the
// four query1 probes around the three makeLong sends (milestone(0) carries
// no step), the query2 probe, the getIndexCount tail assert and undeployAll.
func mrak562CaseSteps(spec infraNWTableMRAK562CaseSpec, count int64) []string {
	return []string{
		"deploy:create:" + spec.create,
		"deployed:create",
		"deploy:insert:" + spec.insert,
		"deployed:insert",
		"deploy:index:" + infraNWTableMRAK562Index,
		"deployed:index",
		"unrepresentable:index-kind:" + infraNWTableMRAK562Index + ":" + infraNWTableMRAK562IndexKindNote,
		"snapshot:select-q1-empty:ordered:id:" + infraNWTableMRAK562Query1,
		`send:SupportBeanRange:{"id":"E1","key":"K1","keyLong":1,"rangeEndLong":3,"rangeStartLong":2}`,
		"snapshot:select-q1-e1:ordered:id:" + infraNWTableMRAK562Query1,
		`send:SupportBeanRange:{"id":"E2","key":"K1","keyLong":1,"rangeEndLong":4,"rangeStartLong":2}`,
		"snapshot:select-q1-e2:ordered:id:" + infraNWTableMRAK562Query1,
		`send:SupportBeanRange:{"id":"E3","key":"K1","keyLong":1,"rangeEndLong":3,"rangeStartLong":3}`,
		"snapshot:select-q1-e3:ordered:id:" + infraNWTableMRAK562Query1,
		"snapshot:select-q2:ordered:id:" + infraNWTableMRAK562Query2,
		"index-count:MyInfraMRAK:create:indexes:" + fmt.Sprintf("%d", count),
		"undeploy-all",
	}
}

// infraNWTableMRAK562CaseSteps pins the exact op sequence per case.
var infraNWTableMRAK562CaseSteps = map[string][]string{
	"mrak-window": mrak562CaseSteps(infraNWTableMRAK562CaseSpecs["mrak-window"], 1),
	"mrak-table":  mrak562CaseSteps(infraNWTableMRAK562CaseSpecs["mrak-table"], 2),
}

func requireInfraNWTableMRAK562Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraNWTableMRAK562ID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraNWTableMRAK562ID, name)
		}
	}
	return nil
}

func validateInfraNWTableMRAK562StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
