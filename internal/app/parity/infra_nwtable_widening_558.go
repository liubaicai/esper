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

// infra_nwtable_widening_558.go replays InfraNWTableCreateIndex ordinals 2-5
// against the pinned Java oracle: the numeric-widening index surface where a
// fire-and-forget `f1=10`/`f1>9`/`f1=2`/`f1>=2` int literal must match a
// long or short indexed column.
//
//   - hashbtree-window/hashbtree-table (ords 2-3, InfraHashBTreeWidening
//     {namedWindow=true/false}): keepall window or (f1,f2)-keyed table
//     MyInfraHBTW(f1 long) fed by `insert into MyInfraHBTW(f1, f2) select
//     longPrimitive, theString from SupportBean`, a late btree index
//     MyInfraHBTWIndex1 on (f1), then FAF `select * from MyInfraHBTW where
//     f1>9`; two SODA eplToModelCompileDeploy probes (`create index IX1 on
//     MyInfraHBTW(f1, f2 btree)` and `create unique index IX2 on
//     MyInfraHBTW(f1)`) deploy the same index surface through the
//     statement-object-model path — plan-only for Go, pinned as
//     unrepresentable records; the second leg repeats on MyInfraHBTWTwo
//     (f1 short) with `f1>=2`.
//   - widening-window/widening-table (ords 4-5, InfraWidening
//     {namedWindow=true/false}): the same two-leg fixture over MyInfraW /
//     MyInfraWTwo with plain hash indexes and the equality FAF reads
//     `f1=10` / `f1=2`.
//
// Coercion is the observable: Java widens the int literals into the indexed
// column type before the hash/btree probe. The Go runner builds the FAF
// filters with `EqualOf`/`GreaterOf`/`GreaterOrEqualOf` against int literals
// so the engine's index probe normalization must coerce int -> int64/int16
// in the lookup path, not just in the full-scan filter.
//
// Approved differences (observably identical to the Java EPL):
//   - `create window`/`create table`/`create index` are catalog operations:
//     the deploy steps carry the byte-exact EPL while the Go side performs
//     env-level window/table registration; the secondary index is declared
//     at creation time because plan.indexPlan is frozen at env.Build — a
//     live CreateIndex would be invisible to FAF index selection (the
//     ordering divergence is observably identical: no rows exist before
//     the index step).
//   - `insert into X(f1, f2) select a, b from SupportBean` maps to the
//     OnEvent InsertIntoNamedWindow/InsertIntoTable trigger with SetColumn
//     assignments (the infra-nwtable-on-delete precedent).
//   - `env.compileExecuteFAF` rides `snapshot` steps that carry the pinned
//     query EPL; the runner builds the typed FAF plan and executes it
//     through ExecuteFireAndForget (no FIREANDFORGET flag on these
//     executions).
//   - The SODA eplToModelCompileDeploy probes have no Go statement-object
//     boundary; the oracle replays parse -> toEPL -> deploy and the runner
//     emits the pinned unrepresentable record.
//   - The sendEventLong/sendEventShort helpers send a SupportBean with only
//     theString and one numeric field set; the payload keys mirror the
//     populated setters and all other bean fields stay zero.

// infraNWTableWidening558Bean mirrors the SupportBean properties the
// sendEventLong/sendEventShort helpers populate.
type infraNWTableWidening558Bean struct {
	TheString      string `esper:"theString"`
	LongPrimitive  int64  `esper:"longPrimitive"`
	ShortPrimitive int16  `esper:"shortPrimitive"`
}

const (
	infraNWTableWidening558ID         = "infra-nwtable-widening-558"
	infraNWTableWidening558JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableWidening558JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java"
)

const infraNWTableWidening558Description = "InfraNWTableCreateIndex ordinals 2-5: InfraHashBTreeWidening (ords 2-3) deploys keepall window or (f1,f2)-keyed MyInfraHBTW(f1 long) fed by insert-into over SupportBean.longPrimitive, a late btree index MyInfraHBTWIndex1 on (f1), and the FAF range read `select * from MyInfraHBTW where f1>9` — the int literal 9 must widen to the long column — then deploys two SODA probes (`create index IX1 on MyInfraHBTW(f1, f2 btree)`, `create unique index IX2 on MyInfraHBTW(f1)`) and repeats the fixture on short-column MyInfraHBTWTwo with `f1>=2`; InfraWidening (ords 4-5) runs the same two-leg fixture over MyInfraW/MyInfraWTwo with plain hash indexes and the equality reads `f1=10`/`f1=2`. SODA probes are plan-only: pinned unrepresentable records. Expected rows {10,E1} and {2,E1} (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java)."

// Verbatim transcriptions of InfraNWTableCreateIndex.java lines 481-509
// (ords 4-5, InfraWidening.run) and lines 527-571 (ords 2-3,
// InfraHashBTreeWidening.run).
const (
	infraNWTableW558CreateOneWindow = "@public create window MyInfraW#keepall as (f1 long, f2 string)"
	infraNWTableW558CreateOneTable  = "@public create table MyInfraW as (f1 long primary key, f2 string primary key)"
	infraNWTableW558InsertOne       = "insert into MyInfraW(f1, f2) select longPrimitive, theString from SupportBean"
	infraNWTableW558IndexOne        = "create index MyInfraWIndex1 on MyInfraW(f1)"
	infraNWTableW558SelectOne       = "select * from MyInfraW where f1=10"
	infraNWTableW558CreateTwoWindow = "@public create window MyInfraWTwo#keepall as (f1 short, f2 string)"
	infraNWTableW558CreateTwoTable  = "@public create table MyInfraWTwo as (f1 short primary key, f2 string primary key)"
	infraNWTableW558InsertTwo       = "insert into MyInfraWTwo(f1, f2) select shortPrimitive, theString from SupportBean"
	infraNWTableW558IndexTwo        = "create index MyInfraWTwoIndex1 on MyInfraWTwo(f1)"
	infraNWTableW558SelectTwo       = "select * from MyInfraWTwo where f1=2"

	infraNWTableHBT558CreateOneWindow = "@public create window MyInfraHBTW#keepall as (f1 long, f2 string)"
	infraNWTableHBT558CreateOneTable  = "@public create table MyInfraHBTW as (f1 long primary key, f2 string primary key)"
	infraNWTableHBT558InsertOne       = "insert into MyInfraHBTW(f1, f2) select longPrimitive, theString from SupportBean"
	infraNWTableHBT558IndexOne        = "create index MyInfraHBTWIndex1 on MyInfraHBTW(f1 btree)"
	infraNWTableHBT558SelectOne       = "select * from MyInfraHBTW where f1>9"
	infraNWTableHBT558SodaIX1         = "create index IX1 on MyInfraHBTW(f1, f2 btree)"
	infraNWTableHBT558SodaIX2         = "create unique index IX2 on MyInfraHBTW(f1)"
	infraNWTableHBT558CreateTwoWindow = "@public create window MyInfraHBTWTwo#keepall as (f1 short, f2 string)"
	infraNWTableHBT558CreateTwoTable  = "@public create table MyInfraHBTWTwo as (f1 short primary key, f2 string primary key)"
	infraNWTableHBT558InsertTwo       = "insert into MyInfraHBTWTwo(f1, f2) select shortPrimitive, theString from SupportBean"
	infraNWTableHBT558IndexTwo        = "create index MyInfraHBTWTwoIndex1 on MyInfraHBTWTwo(f1 btree)"
	infraNWTableHBT558SelectTwo       = "select * from MyInfraHBTWTwo where f1>=2"
)

// SODA probe notes pinned by the unrepresentable steps; the Java oracle
// replays eplToModelCompileDeploy (parse -> toEPL round-trip -> deploy) and
// the Go side has no statement-object boundary.
const (
	infraNWTableW558SodaIX1Note = "SODA create index IX1 on MyInfraHBTW(f1, f2 btree): the Java execution deploys the statement-object-model index over the long leg; EPL-object-model deploy has no Go boundary so the record is plan-only"
	infraNWTableW558SodaIX2Note = "SODA create unique index IX2 on MyInfraHBTW(f1): the Java execution deploys the statement-object-model unique index over the long leg; EPL-object-model deploy has no Go boundary so the record is plan-only"
)

var (
	infraNWTableWidening558JavaSources = []string{
		infraNWTableWidening558JavaSource,
	}
	infraNWTableWidening558JavaRuntimeIDs = []string{
		"java-runtime-ea8f73c9bd9fc31f1312",
		"java-runtime-c29b248899a0388e0768",
		"java-runtime-f4b0d37d40e38914df36",
		"java-runtime-2bffd7c0683d9195c556",
	}
	infraNWTableWidening558JavaExecutions = []string{
		"InfraHashBTreeWidening{namedWindow=true}",
		"InfraHashBTreeWidening{namedWindow=false}",
		"InfraWidening{namedWindow=true}",
		"InfraWidening{namedWindow=false}",
	}
	infraNWTableWidening558JavaStaticIDs = []string{
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
		"java-06a59928c63cc7360d2b",
	}
	infraNWTableWidening558Cases = []string{
		"hashbtree-window",
		"hashbtree-table",
		"widening-window",
		"widening-table",
	}
	infraNWTableWidening558Ordinals = []int{2, 3, 4, 5}
)

// infraNWTableWidening558CaseEPLs pins the newline-joined EPL of every
// EPL-bearing step in the case, in step order — the value carried by the
// scenario cases[] metadata.
var infraNWTableWidening558CaseEPLs = []string{
	strings.Join([]string{
		infraNWTableHBT558CreateOneWindow,
		infraNWTableHBT558InsertOne,
		infraNWTableHBT558IndexOne,
		infraNWTableHBT558SelectOne,
		infraNWTableHBT558SodaIX1,
		infraNWTableHBT558SodaIX2,
		infraNWTableHBT558CreateTwoWindow,
		infraNWTableHBT558InsertTwo,
		infraNWTableHBT558IndexTwo,
		infraNWTableHBT558SelectTwo,
	}, "\n"),
	strings.Join([]string{
		infraNWTableHBT558CreateOneTable,
		infraNWTableHBT558InsertOne,
		infraNWTableHBT558IndexOne,
		infraNWTableHBT558SelectOne,
		infraNWTableHBT558SodaIX1,
		infraNWTableHBT558SodaIX2,
		infraNWTableHBT558CreateTwoTable,
		infraNWTableHBT558InsertTwo,
		infraNWTableHBT558IndexTwo,
		infraNWTableHBT558SelectTwo,
	}, "\n"),
	strings.Join([]string{
		infraNWTableW558CreateOneWindow,
		infraNWTableW558InsertOne,
		infraNWTableW558IndexOne,
		infraNWTableW558SelectOne,
		infraNWTableW558CreateTwoWindow,
		infraNWTableW558InsertTwo,
		infraNWTableW558IndexTwo,
		infraNWTableW558SelectTwo,
	}, "\n"),
	strings.Join([]string{
		infraNWTableW558CreateOneTable,
		infraNWTableW558InsertOne,
		infraNWTableW558IndexOne,
		infraNWTableW558SelectOne,
		infraNWTableW558CreateTwoTable,
		infraNWTableW558InsertTwo,
		infraNWTableW558IndexTwo,
		infraNWTableW558SelectTwo,
	}, "\n"),
}

var infraNWTableWidening558CaseObservations = []string{
	"deploy+snapshot; MyInfraHBTW#keepall window with late btree index on long f1: FAF `where f1>9` widens int 9 to long and returns {10,E1}; SODA IX1/IX2 probes are plan-only; MyInfraHBTWTwo short leg answers `f1>=2` with {2,E1}",
	"deploy+snapshot; (f1,f2)-keyed MyInfraHBTW table with late btree index on long f1: FAF `where f1>9` widens int 9 to long and returns {10,E1}; SODA IX1/IX2 probes are plan-only; MyInfraHBTWTwo short leg answers `f1>=2` with {2,E1}",
	"deploy+snapshot; MyInfraW#keepall window with late hash index on long f1: FAF `where f1=10` widens int 10 to long and returns {10,E1}; MyInfraWTwo short leg answers `f1=2` with {2,E1}",
	"deploy+snapshot; (f1,f2)-keyed MyInfraW table with late hash index on long f1: FAF `where f1=10` widens int 10 to long and returns {10,E1}; MyInfraWTwo short leg answers `f1=2` with {2,E1}",
}

// infraNWTableWidening558CaseSpec carries the per-case fixture constants:
// whether the named-window or table variant runs, the two infra names, the
// secondary index kind and the four byte-exact create/insert/index EPLs.
type infraNWTableWidening558CaseSpec struct {
	namedWindow  bool
	hashBTree    bool
	infraOne     string
	infraTwo     string
	createOne    string
	createTwo    string
	insertOne    string
	insertTwo    string
	indexOne     string
	indexTwo     string
	indexNameOne string
	indexNameTwo string
	indexKind    esper.IndexKind
	selectOne    string
	selectTwo    string
}

var infraNWTableWidening558CaseSpecs = map[string]infraNWTableWidening558CaseSpec{
	"hashbtree-window": {
		namedWindow: true, hashBTree: true,
		infraOne: "MyInfraHBTW", infraTwo: "MyInfraHBTWTwo",
		createOne: infraNWTableHBT558CreateOneWindow, createTwo: infraNWTableHBT558CreateTwoWindow,
		insertOne: infraNWTableHBT558InsertOne, insertTwo: infraNWTableHBT558InsertTwo,
		indexOne: infraNWTableHBT558IndexOne, indexTwo: infraNWTableHBT558IndexTwo,
		indexNameOne: "MyInfraHBTWIndex1", indexNameTwo: "MyInfraHBTWTwoIndex1",
		indexKind: esper.IndexBTree,
		selectOne: infraNWTableHBT558SelectOne, selectTwo: infraNWTableHBT558SelectTwo,
	},
	"hashbtree-table": {
		namedWindow: false, hashBTree: true,
		infraOne: "MyInfraHBTW", infraTwo: "MyInfraHBTWTwo",
		createOne: infraNWTableHBT558CreateOneTable, createTwo: infraNWTableHBT558CreateTwoTable,
		insertOne: infraNWTableHBT558InsertOne, insertTwo: infraNWTableHBT558InsertTwo,
		indexOne: infraNWTableHBT558IndexOne, indexTwo: infraNWTableHBT558IndexTwo,
		indexNameOne: "MyInfraHBTWIndex1", indexNameTwo: "MyInfraHBTWTwoIndex1",
		indexKind: esper.IndexBTree,
		selectOne: infraNWTableHBT558SelectOne, selectTwo: infraNWTableHBT558SelectTwo,
	},
	"widening-window": {
		namedWindow: true, hashBTree: false,
		infraOne: "MyInfraW", infraTwo: "MyInfraWTwo",
		createOne: infraNWTableW558CreateOneWindow, createTwo: infraNWTableW558CreateTwoWindow,
		insertOne: infraNWTableW558InsertOne, insertTwo: infraNWTableW558InsertTwo,
		indexOne: infraNWTableW558IndexOne, indexTwo: infraNWTableW558IndexTwo,
		indexNameOne: "MyInfraWIndex1", indexNameTwo: "MyInfraWTwoIndex1",
		indexKind: esper.IndexHash,
		selectOne: infraNWTableW558SelectOne, selectTwo: infraNWTableW558SelectTwo,
	},
	"widening-table": {
		namedWindow: false, hashBTree: false,
		infraOne: "MyInfraW", infraTwo: "MyInfraWTwo",
		createOne: infraNWTableW558CreateOneTable, createTwo: infraNWTableW558CreateTwoTable,
		insertOne: infraNWTableW558InsertOne, insertTwo: infraNWTableW558InsertTwo,
		indexOne: infraNWTableW558IndexOne, indexTwo: infraNWTableW558IndexTwo,
		indexNameOne: "MyInfraWIndex1", indexNameTwo: "MyInfraWTwoIndex1",
		indexKind: esper.IndexHash,
		selectOne: infraNWTableW558SelectOne, selectTwo: infraNWTableW558SelectTwo,
	},
}

// infraNWTableWidening558CaseState carries the per-case replay state: the
// environment/engine pair, the label→deployments bookkeeping used by
// undeploy-all and the deployed-label set for marker checks.
type infraNWTableWidening558CaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	spec           infraNWTableWidening558CaseSpec
	deployments    map[string][]*esper.Deployment
	deployedLabels map[string]bool
	caseName       string
}

// runInfraNWTableWidening558Scenario replays the four
// InfraNWTableCreateIndex widening executions: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action. The oracle emits the epoch
// time for every record, so the runner pins the same value.
func runInfraNWTableWidening558Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableWidening558Cases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableWidening558Case(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableWidening558ID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableWidening558ID)
	}
	return trace, nil
}

func runInfraNWTableWidening558Case(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	spec, ok := infraNWTableWidening558CaseSpecs[caseName]
	if !ok {
		return compat.Trace{}, fmt.Errorf("%s: unknown case %q", infraNWTableWidening558ID, caseName)
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableWidening558Bean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableWidening558JavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &infraNWTableWidening558CaseState{
		env:            env,
		engine:         engine,
		spec:           spec,
		deployments:    make(map[string][]*esper.Deployment),
		deployedLabels: make(map[string]bool),
		caseName:       caseName,
	}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableWidening558ID}
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
					infraNWTableWidening558ID, step.Statement)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  1,
				Time:      "1970-01-01T00:00:00Z",
			})
		case "send":
			event, err := decodeInfraNWTableWidening558Payload(step)
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
			return compat.Trace{}, fmt.Errorf("%s: unexpected op %q", infraNWTableWidening558ID, step.Op)
		}
	}
	return trace, nil
}

// deploy maps each scenario label to the equivalent Go chain-API plan or
// catalog call: the create steps register the window/table at env level,
// the insert steps deploy the OnEvent feed, and the index steps verify the
// creation-time secondary index is catalog-visible. The byte-exact EPL the
// Java execution
// passes to compileDeploy is pinned by the loader's step keys.
func (s *infraNWTableWidening558CaseState) deploy(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "create-one":
		return s.deployCreate(step.Statement, s.spec.infraOne, true)
	case "create-two":
		return s.deployCreate(step.Statement, s.spec.infraTwo, false)
	case "insert-one":
		return s.deployInsert(ctx, step.Statement, s.spec.infraOne, true)
	case "insert-two":
		return s.deployInsert(ctx, step.Statement, s.spec.infraTwo, false)
	case "index-one":
		return s.deployIndex(step.Statement, s.spec.infraOne, s.spec.indexNameOne)
	case "index-two":
		return s.deployIndex(step.Statement, s.spec.infraTwo, s.spec.indexNameTwo)
	default:
		return fmt.Errorf("%s: unknown deploy label %q", infraNWTableWidening558ID, step.Statement)
	}
}

// deployCreate mirrors a `create window`/`create table` deploy step: an
// env-level registration of the keepall window or the (f1,f2)-keyed table.
func (s *infraNWTableWidening558CaseState) deployCreate(label, infra string, longLeg bool) error {
	indexName := s.spec.indexNameOne
	if infra == s.spec.infraTwo {
		indexName = s.spec.indexNameTwo
	}
	if s.spec.namedWindow {
		fieldType := reflect.TypeOf(int64(0))
		schemaName := infra + "558LongSchema"
		if !longLeg {
			fieldType = reflect.TypeOf(int16(0))
			schemaName = infra + "558ShortSchema"
		}
		schema, err := esper.NewMapSchema(schemaName, []esper.FieldSpec{
			esper.FieldDef("f1", fieldType),
			esper.FieldDef("f2", reflect.TypeOf("")),
		})
		if err != nil {
			return err
		}
		if err := s.env.RegisterSchema(schema); err != nil {
			return err
		}
		var index esper.NamedWindowOption
		if s.spec.indexKind == esper.IndexBTree {
			index = esper.NamedWindowBTreeIndex(indexName, "f1")
		} else {
			index = esper.NamedWindowIndex(indexName, "f1")
		}
		if _, err := esper.CreateNamedWindow(s.env, infra, schema,
			esper.NamedWindowRetention(esper.KeepAll()), index); err != nil {
			return err
		}
	} else {
		var f1 esper.TableColumn
		if longLeg {
			f1 = esper.PrimaryKeyColumn[int64]("f1")
		} else {
			f1 = esper.PrimaryKeyColumn[int16]("f1")
		}
		var index esper.TableOption
		if s.spec.indexKind == esper.IndexBTree {
			index = esper.SecondaryBTreeIndex(indexName, "f1")
		} else {
			index = esper.SecondaryIndex(indexName, "f1")
		}
		if _, err := esper.CreateTable(s.env, infra, []esper.TableColumn{
			f1, esper.PrimaryKeyColumn[string]("f2"),
		}, index); err != nil {
			return err
		}
	}
	s.deployedLabels[label] = true
	return nil
}

// deployInsert mirrors `insert into X(f1, f2) select a, b from SupportBean`:
// the OnEvent trigger with SetColumn assignments into the window or table.
func (s *infraNWTableWidening558CaseState) deployInsert(ctx context.Context, label, infra string, longLeg bool) error {
	var f1 esper.Expr
	if longLeg {
		f1 = esper.Field[infraNWTableWidening558Bean, int64]("longPrimitive")
	} else {
		f1 = esper.Field[infraNWTableWidening558Bean, int16]("shortPrimitive")
	}
	assignments := []esper.TableAssignment{
		esper.SetColumn("f1", f1),
		esper.SetColumn("f2", esper.Field[infraNWTableWidening558Bean, string]("theString")),
	}
	source := esper.From[infraNWTableWidening558Bean](s.env, "SupportBean")
	var plan esper.Plan
	var err error
	if s.spec.namedWindow {
		plan, err = s.env.Build(esper.OnEvent(source).InsertIntoNamedWindow(infra, assignments...).Query())
	} else {
		plan, err = s.env.Build(esper.OnEvent(source).InsertIntoTable(infra, assignments...).Query())
	}
	if err != nil {
		return err
	}
	return s.deployPlan(ctx, label, plan)
}

// deployIndex mirrors the late `create index X on infra(f1 [btree])`
// deploy step. The index is declared at creation time (see deployCreate):
// plan.indexPlan is frozen at env.Build from the env catalog, so a live
// NamedWindow/Table CreateIndex would mutate only the runtime definition
// and the FAF probes would silently full-scan instead of exercising the
// indexProbeKeys/indexProbeRangeSpec widening path — the observable the
// Java executions pin. The step verifies the declared index is
// catalog-visible before the FAF probes run.
func (s *infraNWTableWidening558CaseState) deployIndex(label, infra, indexName string) error {
	if s.spec.namedWindow {
		window, ok := s.engine.NamedWindow(infra)
		if !ok {
			return fmt.Errorf("%s named window is missing", infra)
		}
		found := false
		for _, def := range window.Definition().Indexes() {
			if def.Name == indexName {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s: declared index %q is not catalog-visible on window %s", infraNWTableWidening558ID, indexName, infra)
		}
	} else {
		table, ok := s.engine.Table(infra)
		if !ok {
			return fmt.Errorf("%s table is missing", infra)
		}
		found := false
		for _, def := range table.Definition().Indexes() {
			if def.Name == indexName {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s: declared index %q is not catalog-visible on table %s", infraNWTableWidening558ID, indexName, infra)
		}
	}
	s.deployedLabels[label] = true
	return nil
}

func (s *infraNWTableWidening558CaseState) deployPlan(ctx context.Context, label string,
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
// text. Rows are projected to the pinned f1,f2 fields and sorted — the Java
// assertPropsPerRow is any-order.
func (s *infraNWTableWidening558CaseState) snapshot(ctx context.Context, step compat.Step,
	trace *compat.Trace) error {
	plan, err := s.buildFafSelect(step)
	if err != nil {
		return err
	}
	// The probe MUST resolve through the secondary index (the observable the
	// Java executions pin is numeric widening inside the index lookup, not a
	// filtered scan). plan.indexPlan is frozen at env.Build from the env
	// catalog, so a live CreateIndex would be invisible here — the index is
	// declared at creation time (see deployCreate) and this check fails hard
	// if the selection ever falls back to a full scan.
	selection, ok := plan.IndexPlan().ForSource(0)
	if !ok || selection.Access == esper.IndexAccessFullScan {
		return fmt.Errorf("%s: faf %q resolved to full scan instead of the %s index", infraNWTableWidening558ID, step.Statement, s.spec.indexKind)
	}
	result, err := s.engine.ExecuteFireAndForget(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: faf %q: %w", infraNWTableWidening558ID, step.Statement, err)
	}
	rows := infraTableJoinNormalizeResults(result.Results())
	rows = projectInfraNWTableOnMergeRows(rows, []string{"f1", "f2"})
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
// mixed-type comparison forms keep the int literal visible so the index
// probe normalization must widen int -> long/short, mirroring the EPL
// numeric coercion.
func (s *infraNWTableWidening558CaseState) buildFafSelect(step compat.Step) (esper.Plan, error) {
	var source esper.RecordStream
	if s.spec.namedWindow {
		source = esper.FromNamedWindow(s.env, s.infraForSelect(step.Statement))
	} else {
		source = esper.FromTable(s.env, s.infraForSelect(step.Statement))
	}
	var filter esper.Expression[bool]
	switch step.Statement {
	case "select-gt-long":
		if step.Epl != s.spec.selectOne || !s.spec.hashBTree {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = esper.GreaterOf(esper.Field[any, int64]("f1"), esper.Literal[int](9))
	case "select-gte-short":
		if step.Epl != s.spec.selectTwo || !s.spec.hashBTree {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = esper.GreaterOrEqualOf(esper.Field[any, int16]("f1"), esper.Literal[int](2))
	case "select-eq-long":
		if step.Epl != s.spec.selectOne || s.spec.hashBTree {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = esper.EqualOf(esper.Field[any, int64]("f1"), esper.Literal[int](10))
	case "select-eq-short":
		if step.Epl != s.spec.selectTwo || s.spec.hashBTree {
			return esper.Plan{}, s.fafDrift(step)
		}
		filter = esper.EqualOf(esper.Field[any, int16]("f1"), esper.Literal[int](2))
	default:
		return esper.Plan{}, fmt.Errorf("%s: unknown faf select %q", infraNWTableWidening558ID, step.Statement)
	}
	return s.env.Build(source.Filter(filter).Query())
}

func (s *infraNWTableWidening558CaseState) fafDrift(step compat.Step) error {
	return fmt.Errorf("%s: faf select %q does not pin the expected EPL %q",
		infraNWTableWidening558ID, step.Statement, step.Epl)
}

// infraForSelect resolves the infra name for the leg the snapshot label
// reads: the *-long labels read infraOne (f1 long), the *-short labels
// infraTwo (f1 short).
func (s *infraNWTableWidening558CaseState) infraForSelect(statement string) string {
	if strings.HasSuffix(statement, "-short") {
		return s.spec.infraTwo
	}
	return s.spec.infraOne
}

// unrepresentable emits the pinned record for the two SODA index deploys.
// The Java oracle replays eplToModelCompileDeploy (parse -> toEPL -> module
// compile -> deploy); the Go side has no statement-object boundary so the
// record is plan-only.
func (s *infraNWTableWidening558CaseState) unrepresentable(step compat.Step,
	trace *compat.Trace) error {
	var epl, note string
	switch step.Statement {
	case "soda-ix1":
		epl, note = infraNWTableHBT558SodaIX1, infraNWTableW558SodaIX1Note
	case "soda-ix2":
		epl, note = infraNWTableHBT558SodaIX2, infraNWTableW558SodaIX2Note
	default:
		return fmt.Errorf("%s: unknown unrepresentable label %q", infraNWTableWidening558ID, step.Statement)
	}
	if !s.spec.hashBTree || step.Epl != epl || step.ExpectError != note {
		return fmt.Errorf("%s: unrepresentable step %q is not pinned",
			infraNWTableWidening558ID, step.Statement)
	}
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

func (s *infraNWTableWidening558CaseState) undeployAll(ctx context.Context) error {
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

// decodeInfraNWTableWidening558Payload mirrors the sendEventLong/
// sendEventShort helpers: the payload carries theString plus exactly the
// numeric setter the Java helper populates; unmentioned fields stay zero.
func decodeInfraNWTableWidening558Payload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported infra nwtable widening event type %q", step.EventType)
	}
	var value infraNWTableWidening558Bean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}

// loadInfraNWTableWidening558Scenario enforces the strict scenario contract
// shared by the differential runners: no duplicate or unknown JSON fields,
// pinned metadata, pinned per-case runtime/execution/EPL, and a per-op step
// field whitelist followed by a full step-shape pin.
func loadInfraNWTableWidening558Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableWidening558ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableWidening558ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableWidening558ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableWidening558ID, err)
	}
	if err := requireInfraNWTableWidening558Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableWidening558ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableWidening558ID ||
		metadata.Description != infraNWTableWidening558Description ||
		metadata.JavaCommit != infraNWTableWidening558JavaCommit ||
		metadata.JavaSource != infraNWTableWidening558JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableWidening558ID)
	}
	if err := validateInfraNWTableWidening558StringArray(root["javaRuntimes"], infraNWTableWidening558JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableWidening558StringArray(root["javaNames"], infraNWTableWidening558JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableWidening558StringArray(root["javaStaticIds"], infraNWTableWidening558JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableWidening558StringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableWidening558Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableWidening558ID, len(infraNWTableWidening558Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableWidening558Fields(object,
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
		if definition.Case != infraNWTableWidening558Cases[index] ||
			definition.Ordinal != infraNWTableWidening558Ordinals[index] ||
			definition.RuntimeID != infraNWTableWidening558JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableWidening558JavaExecutions[index] ||
			definition.Observation != infraNWTableWidening558CaseObservations[index] ||
			definition.EPL != infraNWTableWidening558CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableWidening558ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableWidening558ID)
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
			if err := requireInfraNWTableWidening558Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableWidening558Fields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableWidening558Fields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableWidening558Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableWidening558Payload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableWidening558Fields(object, "op", "case", "statement", "mode", "fields", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "unrepresentable":
			if err := requireInfraNWTableWidening558Fields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableWidening558Fields(object, "op", "case"); err != nil {
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
		for _, name := range infraNWTableWidening558Cases {
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
	if err := validateInfraNWTableWidening558RawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraNWTableWidening558RawSteps pins the complete step sequence
// per case against the raw JSON objects: deploy steps with byte-exact EPL,
// deployed markers, the two SupportBean sends, snapshot reads carrying the
// pinned FAF text, the SODA unrepresentable probes and the undeploy-all
// terminators.
func validateInfraNWTableWidening558RawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableWidening558Cases {
		want, ok := infraNWTableWidening558CaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableWidening558ID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableWidening558ID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableWidening558ID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableWidening558ID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableWidening558StepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableWidening558ID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q",
					infraNWTableWidening558ID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableWidening558ID)
	}
	return nil
}

// infraNWTableWidening558StepKey renders a raw step object into its pinned
// string form. Fields are read from the raw JSON because compat.Step does
// not carry the fields array.
func infraNWTableWidening558StepKey(object map[string]json.RawMessage, operation string) (string, error) {
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

// hashBTree558CaseSteps renders the pinned step sequence of
// InfraHashBTreeWidening.run (InfraNWTableCreateIndex.java lines 525-571):
// the long-leg create/insert/btree-index deploys, the load send and the
// `f1>9` FAF read, the two SODA index probes, then the short-leg
// create/insert/btree-index deploys, the load send, the `f1>=2` FAF read
// and undeployAll.
func hashBTree558CaseSteps(spec infraNWTableWidening558CaseSpec) []string {
	return []string{
		"deploy:create-one:" + spec.createOne,
		"deployed:create-one",
		"deploy:insert-one:" + spec.insertOne,
		"deployed:insert-one",
		"deploy:index-one:" + spec.indexOne,
		"deployed:index-one",
		`send:SupportBean:{"longPrimitive":10,"theString":"E1"}`,
		"snapshot:select-gt-long:any:f1,f2:" + spec.selectOne,
		"unrepresentable:soda-ix1:" + infraNWTableHBT558SodaIX1 + ":" + infraNWTableW558SodaIX1Note,
		"unrepresentable:soda-ix2:" + infraNWTableHBT558SodaIX2 + ":" + infraNWTableW558SodaIX2Note,
		"deploy:create-two:" + spec.createTwo,
		"deployed:create-two",
		"deploy:insert-two:" + spec.insertTwo,
		"deployed:insert-two",
		"deploy:index-two:" + spec.indexTwo,
		"deployed:index-two",
		`send:SupportBean:{"shortPrimitive":2,"theString":"E1"}`,
		"snapshot:select-gte-short:any:f1,f2:" + spec.selectTwo,
		"undeploy-all",
	}
}

// widening558CaseSteps renders the pinned step sequence of
// InfraWidening.run (InfraNWTableCreateIndex.java lines 478-510): the
// long-leg create/insert/hash-index deploys, the load send and the `f1=10`
// FAF read, then the short-leg create/insert/hash-index deploys, the load
// send, the `f1=2` FAF read and undeployAll.
func widening558CaseSteps(spec infraNWTableWidening558CaseSpec) []string {
	return []string{
		"deploy:create-one:" + spec.createOne,
		"deployed:create-one",
		"deploy:insert-one:" + spec.insertOne,
		"deployed:insert-one",
		"deploy:index-one:" + spec.indexOne,
		"deployed:index-one",
		`send:SupportBean:{"longPrimitive":10,"theString":"E1"}`,
		"snapshot:select-eq-long:any:f1,f2:" + spec.selectOne,
		"deploy:create-two:" + spec.createTwo,
		"deployed:create-two",
		"deploy:insert-two:" + spec.insertTwo,
		"deployed:insert-two",
		"deploy:index-two:" + spec.indexTwo,
		"deployed:index-two",
		`send:SupportBean:{"shortPrimitive":2,"theString":"E1"}`,
		"snapshot:select-eq-short:any:f1,f2:" + spec.selectTwo,
		"undeploy-all",
	}
}

// infraNWTableWidening558CaseSteps pins the exact op sequence per case.
var infraNWTableWidening558CaseSteps = map[string][]string{
	"hashbtree-window": hashBTree558CaseSteps(infraNWTableWidening558CaseSpecs["hashbtree-window"]),
	"hashbtree-table":  hashBTree558CaseSteps(infraNWTableWidening558CaseSpecs["hashbtree-table"]),
	"widening-window":  widening558CaseSteps(infraNWTableWidening558CaseSpecs["widening-window"]),
	"widening-table":   widening558CaseSteps(infraNWTableWidening558CaseSpecs["widening-table"]),
}

func requireInfraNWTableWidening558Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraNWTableWidening558ID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraNWTableWidening558ID, name)
		}
	}
	return nil
}

func validateInfraNWTableWidening558StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
