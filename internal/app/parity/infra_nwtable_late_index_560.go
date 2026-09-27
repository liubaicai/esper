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

// infra_nwtable_late_index_560.go replays InfraNWTableCreateIndex ordinals
// 8-11 against the pinned Java oracle: the late-create-index executions
// where rows land in the named window/table before the `create index`
// statement deploys and a fire-and-forget select then reads them through
// the index.
//
//   - late-window/late-table (ords 8-9, InfraLateCreate{namedWindow=
//     true/false}): a SupportBean keepall window or
//     (theString, intPrimitive)-keyed table MyInfra fed by
//     `insert into MyInfra select theString, intPrimitive from SupportBean`;
//     three beans (A1,1), (B2,2), (B2,1) land before
//     `create index MyInfra_IDX on MyInfra(theString)` deploys, then
//     `select * from MyInfra where theString = 'B2' order by intPrimitive
//     asc` returns {B2,1},{B2,2} — the equality probe resolves through the
//     declared MyInfra_IDX hash index.
//   - scene-two-window/scene-two-table (ords 10-11, InfraLateCreateSceneTwo
//     {namedWindow=true/false}): a keepall window or fully-keyed table
//     MyInfraLC(f1 string, f2 int, f3 string, f4 string) fed by the concat
//     insert `insert into MyInfraLC(f1, f2, f3, f4) select theString,
//     intPrimitive, '>'||theString||'<', '?'||theString||'?' from
//     SupportBean`; three beans (E1,-4), (E1,-2), (E1,-3) land before
//     `create index MyInfraLCIndex on MyInfraLC(f2, f3, f1)` deploys, then
//     `select * from MyInfraLC where f3='>E1<' order by f2 asc` returns the
//     three E1 rows in f2 order {-4,-3,-2}.
//
// Index-resolution spike (verified against internal/esper/index_plan.go):
// the late-create probe `theString = 'B2'` is a full-key equality on the
// declared MyInfra_IDX hash index and resolves to IndexAccessEquality on
// both variants (the composite {theString,intPrimitive} primary key cannot
// satisfy a theString-only probe, so the secondary index wins). The
// scene-two probe `f3='>E1<'` leads with the non-leading column of the
// (f2,f3,f1) composite hash index: Go's composite hash matcher requires
// predicates on the leading columns in declaration order, so the probe
// resolves to a full scan — the same row set Java's hash index returns,
// since it likewise requires its full column set. The runner asserts the
// late-create probes resolve to the declared index and pins full-scan for
// the scene-two probes rather than faking index use.
//
// Approved differences (observably identical to the Java EPL):
//   - `create window`/`create table`/`create index` are catalog operations:
//     the deploy steps carry the byte-exact EPL while the Go side performs
//     env-level window/table registration; the secondary index is declared
//     at creation time because plan.indexPlan is frozen at env.Build — a
//     live CreateIndex would be invisible to FAF index selection (the
//     ordering divergence is observably identical: every row the late index
//     serves is equally visible to a creation-time index).
//   - `insert into ... select ... from SupportBean` maps to the OnEvent
//     InsertIntoNamedWindow/InsertIntoTable trigger with SetColumn
//     assignments; the `||` string concat maps to Concat of literal/field
//     operands.
//   - `env.compileExecuteFAF` rides `snapshot` steps that carry the pinned
//     query EPL in mode "ordered" — the Java `order by` clause fixes the
//     row order positionally, so the runner applies esper.OrderBy and
//     neither side canonicalizes.
//   - env.milestone(n) checkpoints are harness no-ops and carry no steps.
//   - `env.compileDeploy(stmtTextCreate, path).addListener("Create")` has no
//     observable output: create window/table statements never deliver
//     listener events, so neither side emits a listener record.
//   - sendEventBean(new SupportBean("A1", 1)) maps to a payload carrying
//     theString plus intPrimitive; unmentioned bean fields stay zero.
//   - The `as SupportBean` window schema maps to a two-field map schema:
//     the Java window rows carry the bean type, but the pinned FAF probe
//     reads only theString/intPrimitive, so the narrower schema is
//     observably identical.

// infraNWTableLateIndex560Bean mirrors the SupportBean properties the
// SupportBean(theString, intPrimitive) constructor populates.
type infraNWTableLateIndex560Bean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const (
	infraNWTableLateIndex560ID         = "infra-nwtable-late-index-560"
	infraNWTableLateIndex560JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableLateIndex560JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java"
)

const infraNWTableLateIndex560Description = "InfraNWTableCreateIndex ordinals 8-11: InfraLateCreate (ords 8-9) deploys a SupportBean keepall window or (theString,intPrimitive)-keyed table MyInfra fed by `insert into MyInfra select theString, intPrimitive from SupportBean`, sends A1/1, B2/2, B2/1, then deploys `create index MyInfra_IDX on MyInfra(theString)` before the FAF probe `select * from MyInfra where theString = 'B2' order by intPrimitive asc` returns {B2,1},{B2,2} through the declared hash index; InfraLateCreateSceneTwo (ords 10-11) deploys a keepall window or fully-keyed table MyInfraLC(f1 string, f2 int, f3 string, f4 string) fed by the concat insert-into over SupportBean, sends E1/-4, E1/-2, E1/-3, then deploys `create index MyInfraLCIndex on MyInfraLC(f2, f3, f1)` before the FAF probe `select * from MyInfraLC where f3='>E1<' order by f2 asc` returns the three E1 rows in f2 order {-4,-3,-2}. The f3-leading probe resolves to a full scan on both engines (a composite hash index requires predicates on its leading columns); the late-create equality probe uses the declared MyInfra_IDX index. Java milestone checkpoints carry no steps (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java)."

// Verbatim transcriptions of InfraNWTableCreateIndex.java lines 342-364
// (ords 8-9, InfraLateCreate.run) and lines 389-410 (ords 10-11,
// InfraLateCreateSceneTwo.run).
const (
	infraNWTableLC560CreateWindow = "@Name('Create') @public create window MyInfra.win:keepall() as SupportBean"
	infraNWTableLC560CreateTable  = "@Name('Create') @public create table MyInfra(theString string primary key, intPrimitive int primary key)"
	infraNWTableLC560Insert       = "@Name('Insert') insert into MyInfra select theString, intPrimitive from SupportBean"
	infraNWTableLC560Index        = "@Name('Index') create index MyInfra_IDX on MyInfra(theString)"
	infraNWTableLC560Select       = "select * from MyInfra where theString = 'B2' order by intPrimitive asc"

	infraNWTableLC560TwoCreateWindow = "@public create window MyInfraLC#keepall as (f1 string, f2 int, f3 string, f4 string)"
	infraNWTableLC560TwoCreateTable  = "@public create table MyInfraLC as (f1 string primary key, f2 int primary key, f3 string primary key, f4 string primary key)"
	infraNWTableLC560TwoInsert       = "insert into MyInfraLC(f1, f2, f3, f4) select theString, intPrimitive, '>'||theString||'<', '?'||theString||'?' from SupportBean"
	infraNWTableLC560TwoIndex        = "create index MyInfraLCIndex on MyInfraLC(f2, f3, f1)"
	infraNWTableLC560TwoSelect       = "select * from MyInfraLC where f3='>E1<' order by f2 asc"
)

var (
	infraNWTableLateIndex560JavaSources = []string{
		infraNWTableLateIndex560JavaSource,
	}
	infraNWTableLateIndex560JavaRuntimeIDs = []string{
		"java-runtime-ae0b11741e8c17c55271",
		"java-runtime-31d2b97272df49845355",
		"java-runtime-ba5fc9c488e24b9bae78",
		"java-runtime-386fcd9642ae114b58af",
	}
	infraNWTableLateIndex560JavaExecutions = []string{
		"InfraLateCreate{namedWindow=true}",
		"InfraLateCreate{namedWindow=false}",
		"InfraLateCreateSceneTwo{namedWindow=true}",
		"InfraLateCreateSceneTwo{namedWindow=false}",
	}
	infraNWTableLateIndex560JavaStaticIDs = []string{
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
	}
	infraNWTableLateIndex560JavaFlags = []string{}
	infraNWTableLateIndex560Cases     = []string{
		"late-window",
		"late-table",
		"scene-two-window",
		"scene-two-table",
	}
	infraNWTableLateIndex560Ordinals = []int{8, 9, 10, 11}
)

// infraNWTableLateIndex560CaseEPLs pins the newline-joined EPL of every
// EPL-bearing step in the case, in step order — the value carried by the
// scenario cases[] metadata.
var infraNWTableLateIndex560CaseEPLs = []string{
	strings.Join([]string{
		infraNWTableLC560CreateWindow,
		infraNWTableLC560Insert,
		infraNWTableLC560Index,
		infraNWTableLC560Select,
	}, "\n"),
	strings.Join([]string{
		infraNWTableLC560CreateTable,
		infraNWTableLC560Insert,
		infraNWTableLC560Index,
		infraNWTableLC560Select,
	}, "\n"),
	strings.Join([]string{
		infraNWTableLC560TwoCreateWindow,
		infraNWTableLC560TwoInsert,
		infraNWTableLC560TwoIndex,
		infraNWTableLC560TwoSelect,
	}, "\n"),
	strings.Join([]string{
		infraNWTableLC560TwoCreateTable,
		infraNWTableLC560TwoInsert,
		infraNWTableLC560TwoIndex,
		infraNWTableLC560TwoSelect,
	}, "\n"),
}

var infraNWTableLateIndex560CaseObservations = []string{
	"deploy+send+snapshot; MyInfra.win:keepall() SupportBean window fed by insert-into, three beans pre-index then the late MyInfra_IDX(theString) hash index: the ordered FAF probe `theString = 'B2'` resolves to the declared index and returns {B2,1},{B2,2} in intPrimitive order; milestone(0..1) checkpoints carry no steps",
	"deploy+send+snapshot; (theString,intPrimitive)-keyed MyInfra table fed by insert-into, three beans pre-index then the late MyInfra_IDX(theString) hash index: the ordered FAF probe `theString = 'B2'` resolves to the declared index and returns {B2,1},{B2,2} in intPrimitive order; milestone(0..1) checkpoints carry no steps",
	"deploy+send+snapshot; MyInfraLC#keepall window fed by the concat insert-into, three E1 beans pre-index then the late MyInfraLCIndex(f2,f3,f1) composite hash index: the ordered FAF probe `f3='>E1<'` leads with the non-leading column so both engines full-scan and return the three E1 rows in f2 order {-4,-3,-2}; milestone(0..1) checkpoints carry no steps",
	"deploy+send+snapshot; fully-keyed MyInfraLC table fed by the concat insert-into, three E1 beans pre-index then the late MyInfraLCIndex(f2,f3,f1) composite hash index: the ordered FAF probe `f3='>E1<'` leads with the non-leading column so both engines full-scan and return the three E1 rows in f2 order {-4,-3,-2}; milestone(0..1) checkpoints carry no steps",
}

// infraNWTableLateIndex560CaseSpec carries the per-case fixture constants:
// whether the named-window or table variant runs, the infra and index
// names, the byte-exact create/insert/index/select EPLs, the ordered
// SupportBean sends and the index columns asserted catalog-visible.
type infraNWTableLateIndex560CaseSpec struct {
	namedWindow bool
	sceneTwo    bool
	infra       string
	indexName   string
	create      string
	insert      string
	index       string
	selectEPL   string
	fields      []string
	sends       []infraNWTableLateIndex560Bean
}

var infraNWTableLateIndex560CaseSpecs = map[string]infraNWTableLateIndex560CaseSpec{
	"late-window": {
		namedWindow: true,
		infra:       "MyInfra", indexName: "MyInfra_IDX",
		create: infraNWTableLC560CreateWindow, insert: infraNWTableLC560Insert,
		index: infraNWTableLC560Index, selectEPL: infraNWTableLC560Select,
		fields: []string{"theString", "intPrimitive"},
		sends: []infraNWTableLateIndex560Bean{
			{TheString: "A1", IntPrimitive: 1},
			{TheString: "B2", IntPrimitive: 2},
			{TheString: "B2", IntPrimitive: 1},
		},
	},
	"late-table": {
		namedWindow: false,
		infra:       "MyInfra", indexName: "MyInfra_IDX",
		create: infraNWTableLC560CreateTable, insert: infraNWTableLC560Insert,
		index: infraNWTableLC560Index, selectEPL: infraNWTableLC560Select,
		fields: []string{"theString", "intPrimitive"},
		sends: []infraNWTableLateIndex560Bean{
			{TheString: "A1", IntPrimitive: 1},
			{TheString: "B2", IntPrimitive: 2},
			{TheString: "B2", IntPrimitive: 1},
		},
	},
	"scene-two-window": {
		namedWindow: true, sceneTwo: true,
		infra: "MyInfraLC", indexName: "MyInfraLCIndex",
		create: infraNWTableLC560TwoCreateWindow, insert: infraNWTableLC560TwoInsert,
		index: infraNWTableLC560TwoIndex, selectEPL: infraNWTableLC560TwoSelect,
		fields: []string{"f1", "f2", "f3", "f4"},
		sends: []infraNWTableLateIndex560Bean{
			{TheString: "E1", IntPrimitive: -4},
			{TheString: "E1", IntPrimitive: -2},
			{TheString: "E1", IntPrimitive: -3},
		},
	},
	"scene-two-table": {
		namedWindow: false, sceneTwo: true,
		infra: "MyInfraLC", indexName: "MyInfraLCIndex",
		create: infraNWTableLC560TwoCreateTable, insert: infraNWTableLC560TwoInsert,
		index: infraNWTableLC560TwoIndex, selectEPL: infraNWTableLC560TwoSelect,
		fields: []string{"f1", "f2", "f3", "f4"},
		sends: []infraNWTableLateIndex560Bean{
			{TheString: "E1", IntPrimitive: -4},
			{TheString: "E1", IntPrimitive: -2},
			{TheString: "E1", IntPrimitive: -3},
		},
	},
}

// infraNWTableLateIndex560CaseState carries the per-case replay state: the
// environment/engine pair, the label→deployments bookkeeping used by
// undeploy-all and the deployed-label set for marker checks.
type infraNWTableLateIndex560CaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	spec           infraNWTableLateIndex560CaseSpec
	deployments    map[string][]*esper.Deployment
	deployedLabels map[string]bool
	caseName       string
}

// runInfraNWTableLateIndex560Scenario replays the four
// InfraNWTableCreateIndex late-create executions: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action. The oracle emits the epoch
// time for every record, so the runner pins the same value.
func runInfraNWTableLateIndex560Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableLateIndex560Cases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableLateIndex560Case(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableLateIndex560ID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableLateIndex560ID)
	}
	return trace, nil
}

func runInfraNWTableLateIndex560Case(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	spec, ok := infraNWTableLateIndex560CaseSpecs[caseName]
	if !ok {
		return compat.Trace{}, fmt.Errorf("%s: unknown case %q", infraNWTableLateIndex560ID, caseName)
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableLateIndex560Bean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableLateIndex560JavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	state := &infraNWTableLateIndex560CaseState{
		env:            env,
		engine:         engine,
		spec:           spec,
		deployments:    make(map[string][]*esper.Deployment),
		deployedLabels: make(map[string]bool),
		caseName:       caseName,
	}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableLateIndex560ID}
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
					infraNWTableLateIndex560ID, step.Statement)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  1,
				Time:      "1970-01-01T00:00:00Z",
			})
		case "send":
			event, err := decodeInfraNWTableLateIndex560Payload(step)
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
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s: unexpected op %q", infraNWTableLateIndex560ID, step.Op)
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
func (s *infraNWTableLateIndex560CaseState) deploy(ctx context.Context, step compat.Step) error {
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
		return fmt.Errorf("%s: unknown deploy label %q", infraNWTableLateIndex560ID, step.Statement)
	}
}

func (s *infraNWTableLateIndex560CaseState) deployDrift(step compat.Step) error {
	return fmt.Errorf("%s: deploy %q does not pin the expected EPL %q",
		infraNWTableLateIndex560ID, step.Statement, step.Epl)
}

// deployCreate mirrors a `create window`/`create table` deploy step: an
// env-level registration of the keepall window or the primary-keyed table.
// The late secondary index is declared here because plan.indexPlan is
// frozen at env.Build — a live CreateIndex would be invisible to FAF index
// selection, and the ordering divergence is observably identical (every
// row the late index serves is equally visible to a creation-time index).
func (s *infraNWTableLateIndex560CaseState) deployCreate(label string) error {
	if s.spec.sceneTwo {
		return s.deployCreateSceneTwo(label)
	}
	return s.deployCreateLate(label)
}

// deployCreateLate registers the InfraLateCreate infra: a two-field window
// (the Java `as SupportBean` schema projects only theString/intPrimitive
// observably) or the (theString, intPrimitive)-keyed table, plus the
// MyInfra_IDX hash index on theString.
func (s *infraNWTableLateIndex560CaseState) deployCreateLate(label string) error {
	if s.spec.namedWindow {
		schema, err := esper.NewMapSchema(s.spec.infra+"560Schema", []esper.FieldSpec{
			esper.FieldDef("theString", reflect.TypeOf("")),
			esper.FieldDef("intPrimitive", reflect.TypeOf(0)),
		})
		if err != nil {
			return err
		}
		if err := s.env.RegisterSchema(schema); err != nil {
			return err
		}
		if _, err := esper.CreateNamedWindow(s.env, s.spec.infra, schema,
			esper.NamedWindowRetention(esper.KeepAll()),
			esper.NamedWindowIndex(s.spec.indexName, "theString")); err != nil {
			return err
		}
	} else {
		if _, err := esper.CreateTable(s.env, s.spec.infra, []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("theString"),
			esper.PrimaryKeyColumn[int]("intPrimitive"),
		}, esper.SecondaryIndex(s.spec.indexName, "theString")); err != nil {
			return err
		}
	}
	s.deployedLabels[label] = true
	return nil
}

// deployCreateSceneTwo registers the InfraLateCreateSceneTwo infra: the
// f1..f4 keepall window or the fully-keyed table, plus the composite
// (f2, f3, f1) hash index.
func (s *infraNWTableLateIndex560CaseState) deployCreateSceneTwo(label string) error {
	indexCols := []string{"f2", "f3", "f1"}
	if s.spec.namedWindow {
		stringT := reflect.TypeOf("")
		schema, err := esper.NewMapSchema(s.spec.infra+"560Schema", []esper.FieldSpec{
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
			esper.PrimaryKeyColumn[int]("f2"),
			esper.PrimaryKeyColumn[string]("f3"),
			esper.PrimaryKeyColumn[string]("f4"),
		}, esper.SecondaryIndex(s.spec.indexName, indexCols...)); err != nil {
			return err
		}
	}
	s.deployedLabels[label] = true
	return nil
}

// deployInsert mirrors the `insert into ... select ... from SupportBean`
// feed: the OnEvent trigger with SetColumn assignments into the window or
// table; the `||` concat operands map to Concat.
func (s *infraNWTableLateIndex560CaseState) deployInsert(ctx context.Context, label string) error {
	theString := esper.Field[infraNWTableLateIndex560Bean, string]("theString")
	assignments := []esper.TableAssignment{
		esper.SetColumn("theString", theString),
		esper.SetColumn("intPrimitive", esper.Field[infraNWTableLateIndex560Bean, int]("intPrimitive")),
	}
	if s.spec.sceneTwo {
		assignments = []esper.TableAssignment{
			esper.SetColumn("f1", theString),
			esper.SetColumn("f2", esper.Field[infraNWTableLateIndex560Bean, int]("intPrimitive")),
			esper.SetColumn("f3", esper.Concat(
				esper.Literal[string](">"), theString, esper.Literal[string]("<"))),
			esper.SetColumn("f4", esper.Concat(
				esper.Literal[string]("?"), theString, esper.Literal[string]("?"))),
		}
	}
	source := esper.From[infraNWTableLateIndex560Bean](s.env, "SupportBean")
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

// deployIndex mirrors the late `create index` deploy step. The index is
// declared at creation time (see deployCreate): the step verifies the
// declared index is catalog-visible before the FAF probe runs.
func (s *infraNWTableLateIndex560CaseState) deployIndex(label string) error {
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
				infraNWTableLateIndex560ID, s.spec.indexName, s.spec.infra)
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
				infraNWTableLateIndex560ID, s.spec.indexName, s.spec.infra)
		}
	}
	s.deployedLabels[label] = true
	return nil
}

func (s *infraNWTableLateIndex560CaseState) deployPlan(ctx context.Context, label string,
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
// text. Rows are projected to the pinned fields and kept in result order —
// the Java `order by` clause fixes the ordering positionally (mode
// "ordered"), so neither side canonicalizes.
func (s *infraNWTableLateIndex560CaseState) snapshot(ctx context.Context, step compat.Step,
	trace *compat.Trace) error {
	plan, err := s.buildFafSelect(step)
	if err != nil {
		return err
	}
	selection, ok := plan.IndexPlan().ForSource(0)
	if !s.spec.sceneTwo {
		// The theString equality probe MUST resolve through the declared
		// MyInfra_IDX hash index — the index-probe observable the Java
		// executions pin; a full scan here would hide a matcher regression.
		if !ok || selection.Access != esper.IndexAccessEquality ||
			selection.IndexName != s.spec.indexName ||
			!reflect.DeepEqual(selection.MatchedColumns, []string{"theString"}) {
			return fmt.Errorf("%s: faf %q resolved to %#v instead of the %s hash index",
				infraNWTableLateIndex560ID, step.Statement, selection, s.spec.indexName)
		}
	} else if !ok || selection.Access != esper.IndexAccessFullScan {
		// Spike resolution: the f3-leading probe cannot consume the
		// (f2,f3,f1) composite hash key (Go's matcher requires predicates on
		// the leading columns) and the fully-keyed primary index cannot
		// satisfy an f3-only probe either, so both engines return the same
		// row set through a full scan. The assertion pins the fallback
		// rather than faking index use.
		return fmt.Errorf("%s: faf %q unexpectedly resolved to index %q (access %v) instead of a full scan",
			infraNWTableLateIndex560ID, step.Statement, selection.IndexName, selection.Access)
	}
	result, err := s.engine.ExecuteFireAndForget(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: faf %q: %w", infraNWTableLateIndex560ID, step.Statement, err)
	}
	rows := infraTableJoinNormalizeResults(result.Results())
	rows = projectInfraNWTableOnMergeRows(rows, s.spec.fields)
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

// buildFafSelect maps the pinned FAF select EPL to the typed plan,
// including the `order by` key that fixes the row order positionally.
func (s *infraNWTableLateIndex560CaseState) buildFafSelect(step compat.Step) (esper.Plan, error) {
	var source esper.RecordStream
	if s.spec.namedWindow {
		source = esper.FromNamedWindow(s.env, s.spec.infra)
	} else {
		source = esper.FromTable(s.env, s.spec.infra)
	}
	if step.Epl != s.spec.selectEPL {
		return esper.Plan{}, s.fafDrift(step)
	}
	var filter esper.Expression[bool]
	var order esper.SortKey
	if s.spec.sceneTwo {
		filter = esper.EqualOf(esper.Field[any, string]("f3"), esper.Literal[string](">E1<"))
		order = esper.Ascending(esper.Field[any, int]("f2"))
	} else {
		filter = esper.EqualOf(esper.Field[any, string]("theString"), esper.Literal[string]("B2"))
		order = esper.Ascending(esper.Field[any, int]("intPrimitive"))
	}
	return s.env.Build(source.Filter(filter).Query(esper.OrderBy(order)))
}

func (s *infraNWTableLateIndex560CaseState) fafDrift(step compat.Step) error {
	return fmt.Errorf("%s: faf select %q does not pin the expected EPL %q",
		infraNWTableLateIndex560ID, step.Statement, step.Epl)
}

func (s *infraNWTableLateIndex560CaseState) undeployAll(ctx context.Context) error {
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

// decodeInfraNWTableLateIndex560Payload mirrors the sendEventBean(
// SupportBean(theString, intPrimitive)) calls: the payload carries
// theString plus intPrimitive; unmentioned fields stay zero.
func decodeInfraNWTableLateIndex560Payload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported infra nwtable late-index event type %q", step.EventType)
	}
	var value infraNWTableLateIndex560Bean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}

// loadInfraNWTableLateIndex560Scenario enforces the strict scenario
// contract shared by the differential runners: no duplicate or unknown
// JSON fields, pinned metadata, pinned per-case runtime/execution/EPL, and
// a per-op step field whitelist followed by a full step-shape pin.
func loadInfraNWTableLateIndex560Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableLateIndex560ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableLateIndex560ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableLateIndex560ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableLateIndex560ID, err)
	}
	if err := requireInfraNWTableLateIndex560Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableLateIndex560ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableLateIndex560ID ||
		metadata.Description != infraNWTableLateIndex560Description ||
		metadata.JavaCommit != infraNWTableLateIndex560JavaCommit ||
		metadata.JavaSource != infraNWTableLateIndex560JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableLateIndex560ID)
	}
	if err := validateInfraNWTableLateIndex560StringArray(root["javaRuntimes"], infraNWTableLateIndex560JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableLateIndex560StringArray(root["javaNames"], infraNWTableLateIndex560JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableLateIndex560StringArray(root["javaStaticIds"], infraNWTableLateIndex560JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableLateIndex560StringArray(root["javaFlags"], infraNWTableLateIndex560JavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableLateIndex560Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableLateIndex560ID, len(infraNWTableLateIndex560Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableLateIndex560Fields(object,
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
		if definition.Case != infraNWTableLateIndex560Cases[index] ||
			definition.Ordinal != infraNWTableLateIndex560Ordinals[index] ||
			definition.RuntimeID != infraNWTableLateIndex560JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableLateIndex560JavaExecutions[index] ||
			definition.Observation != infraNWTableLateIndex560CaseObservations[index] ||
			definition.EPL != infraNWTableLateIndex560CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableLateIndex560ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableLateIndex560ID)
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
			if err := requireInfraNWTableLateIndex560Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableLateIndex560Fields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableLateIndex560Fields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableLateIndex560Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableLateIndex560Payload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableLateIndex560Fields(object, "op", "case", "statement", "mode", "fields", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableLateIndex560Fields(object, "op", "case"); err != nil {
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
		for _, name := range infraNWTableLateIndex560Cases {
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
	if err := validateInfraNWTableLateIndex560RawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraNWTableLateIndex560RawSteps pins the complete step sequence
// per case against the raw JSON objects: deploy steps with byte-exact EPL,
// deployed markers, the SupportBean sends, the ordered snapshot read
// carrying the pinned FAF text and the undeploy-all terminators. Java
// milestone(0..1) checkpoints carry no steps.
func validateInfraNWTableLateIndex560RawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableLateIndex560Cases {
		want, ok := infraNWTableLateIndex560CaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableLateIndex560ID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableLateIndex560ID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableLateIndex560ID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableLateIndex560ID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableLateIndex560StepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableLateIndex560ID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q",
					infraNWTableLateIndex560ID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableLateIndex560ID)
	}
	return nil
}

// infraNWTableLateIndex560StepKey renders a raw step object into its
// pinned string form. Fields are read from the raw JSON because compat.Step
// does not carry the fields array.
func infraNWTableLateIndex560StepKey(object map[string]json.RawMessage, operation string) (string, error) {
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
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// infraNWTableLateIndex560PinnedCaseSteps renders the pinned step sequence
// of InfraLateCreate.run (InfraNWTableCreateIndex.java lines 337-370) and
// InfraLateCreateSceneTwo.run (lines 387-413): the create/insert deploys,
// the pre-index SupportBean sends, the late create-index deploy, the
// ordered FAF probe (milestone(0..1) checkpoints carry no steps) and
// undeployAll.
func infraNWTableLateIndex560PinnedCaseSteps(spec infraNWTableLateIndex560CaseSpec) []string {
	steps := []string{
		"deploy:create:" + spec.create,
		"deployed:create",
		"deploy:insert:" + spec.insert,
		"deployed:insert",
	}
	for _, send := range spec.sends {
		payload, _ := json.Marshal(map[string]any{
			"theString":    send.TheString,
			"intPrimitive": send.IntPrimitive,
		})
		steps = append(steps, "send:SupportBean:"+string(payload))
	}
	return append(steps,
		"deploy:index:"+spec.index,
		"deployed:index",
		"snapshot:select:ordered:"+joinStrings(spec.fields, ",")+":"+spec.selectEPL,
		"undeploy-all",
	)
}

// infraNWTableLateIndex560CaseSteps pins the exact op sequence per case.
var infraNWTableLateIndex560CaseSteps = map[string][]string{
	"late-window":      infraNWTableLateIndex560PinnedCaseSteps(infraNWTableLateIndex560CaseSpecs["late-window"]),
	"late-table":       infraNWTableLateIndex560PinnedCaseSteps(infraNWTableLateIndex560CaseSpecs["late-table"]),
	"scene-two-window": infraNWTableLateIndex560PinnedCaseSteps(infraNWTableLateIndex560CaseSpecs["scene-two-window"]),
	"scene-two-table":  infraNWTableLateIndex560PinnedCaseSteps(infraNWTableLateIndex560CaseSpecs["scene-two-table"]),
}

func requireInfraNWTableLateIndex560Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraNWTableLateIndex560ID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraNWTableLateIndex560ID, name)
		}
	}
	return nil
}

func validateInfraNWTableLateIndex560StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
