package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// infra_nwtable_context.go replays InfraNWTableContext ords 0 and 1 against
// the pinned Java oracle: the parameterized InfraContext execution
// (namedWindow=true/false) binds a contexted keepall named window or a
// contexted two-primary-key table under the non-overlapping ContextOne
// init-term context.
//
// named-window (ord 0, InfraContext{namedWindow=true}) and table (ord 1,
// InfraContext{namedWindow=false}) share one flow: deploy the
// `start SupportBean_S0 end SupportBean_S1` context, the @public contexted
// create-infra statement and the contexted insert-into feed; send
// SupportBean_S0(0) to open the partition, then SupportBean(E1,10,100) and
// SupportBean(E2,20,200); late-deploy the six `output snapshot when
// terminated` selects s1..s6 into the already-active partition; and send
// SupportBean_S1(0) so every statement fires exactly once — s1 both rows,
// s2 the single count 2, s3 one row per infra row carrying the total
// count, s4 the per-pkey0 counts, s5 the group-member pkey1 values and s6
// the rollup grouping sets with null super-aggregate columns.
//
// Approved differences (observably identical to the Java EPL):
//   - `create context`/`create window`/`create table` map to env-level
//     registrations (CreateInitiatedTerminatedContext, CreateNamedWindow,
//     CreateTable); Go has no module path, so the oracle's path-shared
//     per-statement compileDeploys have no Go-side counterpart beyond
//     ordering.
//   - `insert into MyInfra select ...` maps to Select+InsertInto for the
//     named window and OnRecord+InsertIntoTable for the table (the
//     context-key-segmented-infra-prioritized precedent).
//   - Java milestone(0) is a harness no-op and carries no step.
//   - Java's assertPropsPerRowLastNewAnyOrder/assertPropsNew batches record
//     in canonical sorted-field order on both traces (the
//     infra-table-context precedent).

const infraNWTableContextID = "infra-nwtable-context"
const infraNWTableContextJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWTableContextDescription = "InfraNWTableContext ords 0 and 1: the non-overlapping ContextOne init-term context (start SupportBean_S0 end SupportBean_S1) binds a keepall named window (ord 0) or a two-primary-key table (ord 1) fed by a contexted insert-into over SupportBean; six output-snapshot-when-terminated selects (wildcard, count(*), ungrouped per-row count, group-by pkey0, group-by pkey0 with member pkey1, and rollup(pkey0,pkey1)) deploy late into the active partition and each fires once when SupportBean_S1(0) ends it (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableContext.java)."

const infraNWTableContextSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableContext.java"

// Verbatim transcriptions of InfraNWTableContext lines 41, 43-45, 48 and
// 55-60: each compileDeploy is a separate module sharing one
// RegressionPath, so @public on the context and the infra is required for
// cross-module visibility. `@name('sN')` is concatenated directly onto
// `context` — no space.
const (
	intcCtx         = "@public create context ContextOne start SupportBean_S0 end SupportBean_S1"
	intcCreateNW    = "@public context ContextOne create window MyInfra#keepall as (pkey0 string, pkey1 int, c0 long)"
	intcCreateTable = "@public context ContextOne create table MyInfra as (pkey0 string primary key, pkey1 int primary key, c0 long)"
	intcInsert      = "context ContextOne insert into MyInfra select theString as pkey0, intPrimitive as pkey1, longPrimitive as c0 from SupportBean"
	intcS1          = "@name('s1')context ContextOne select * from MyInfra output snapshot when terminated"
	intcS2          = "@name('s2')context ContextOne select count(*) as thecnt from MyInfra output snapshot when terminated"
	intcS3          = "@name('s3')context ContextOne select pkey0, count(*) as thecnt from MyInfra output snapshot when terminated"
	intcS4          = "@name('s4')context ContextOne select pkey0, count(*) as thecnt from MyInfra group by pkey0 output snapshot when terminated"
	intcS5          = "@name('s5')context ContextOne select pkey0, pkey1, count(*) as thecnt from MyInfra group by pkey0 output snapshot when terminated"
	intcS6          = "@name('s6')context ContextOne select pkey0, pkey1, count(*) as thecnt from MyInfra group by rollup (pkey0, pkey1) output snapshot when terminated"
)

var infraNWTableContextJavaSources = []string{
	infraNWTableContextSource,
}

var infraNWTableContextJavaRuntimeIDs = []string{
	"java-runtime-dfaacac6bb82c21d6d47", // InfraContext{namedWindow=true}
	"java-runtime-913c09693fb262b75d75", // InfraContext{namedWindow=false}
}

var infraNWTableContextJavaExecutions = []string{
	"InfraContext{namedWindow=true}",
	"InfraContext{namedWindow=false}",
}

// Both ordinals share the parameterized InfraContext execution class, so
// the static manifest records one id repeated per ordinal.
var infraNWTableContextJavaStaticIDs = []string{
	"java-504b139a33ff22d20d65",
	"java-504b139a33ff22d20d65",
}

var infraNWTableContextJavaFlags = []string{}

var infraNWTableContextCases = []string{
	"named-window",
	"table",
}

var infraNWTableContextOrdinals = []int{0, 1}

var infraNWTableContextCaseRuntimeIDs = map[string]string{
	"named-window": "java-runtime-dfaacac6bb82c21d6d47",
	"table":        "java-runtime-913c09693fb262b75d75",
}

var infraNWTableContextCaseObservations = []string{
	"listener; six late-deployed output-snapshot-when-terminated selects over the contexted keepall window each fire once when SupportBean_S1(0) terminates the partition: s1 {E1,10,100},{E2,20,200}; s2 {thecnt=2}; s3 {E1,2},{E2,2}; s4 {E1,1},{E2,1}; s5 {E1,10,1},{E2,20,1}; s6 adds rollup super-aggregate rows {E1,null,1},{E2,null,1},{null,null,2}",
	"listener; six late-deployed output-snapshot-when-terminated selects over the contexted two-primary-key table each fire once when SupportBean_S1(0) terminates the partition: s1 {E1,10,100},{E2,20,200}; s2 {thecnt=2}; s3 {E1,2},{E2,2}; s4 {E1,1},{E2,1}; s5 {E1,10,1},{E2,20,1}; s6 adds rollup super-aggregate rows {E1,null,1},{E2,null,1},{null,null,2}",
}

// infraNWTableContextCaseEPLs pins the newline-joined EPL of every deploy
// step in the case, in step order — the value carried by the scenario
// cases[] metadata.
var infraNWTableContextCaseEPLs = []string{
	strings.Join([]string{intcCtx, intcCreateNW, intcInsert,
		intcS1, intcS2, intcS3, intcS4, intcS5, intcS6}, "\n"),
	strings.Join([]string{intcCtx, intcCreateTable, intcInsert,
		intcS1, intcS2, intcS3, intcS4, intcS5, intcS6}, "\n"),
}

// infraNWTableContextDeployEPLs pins the byte-exact EPL each deploy step
// carries, keyed by case then statement label.
var infraNWTableContextDeployEPLs = map[string]map[string]string{
	"named-window": {
		"ctx":    intcCtx,
		"create": intcCreateNW,
		"insert": intcInsert,
		"s1":     intcS1,
		"s2":     intcS2,
		"s3":     intcS3,
		"s4":     intcS4,
		"s5":     intcS5,
		"s6":     intcS6,
	},
	"table": {
		"ctx":    intcCtx,
		"create": intcCreateTable,
		"insert": intcInsert,
		"s1":     intcS1,
		"s2":     intcS2,
		"s3":     intcS3,
		"s4":     intcS4,
		"s5":     intcS5,
		"s6":     intcS6,
	},
}

// infraNWTableContextCaseSteps pins the complete step sequence per case as
// op|statement|eventType|at keys so the loader asserts the scenario file
// matches the contract: deploy ctx/create/insert, the S0 partition opener,
// the two SupportBean inserts, the six late s1..s6 deploys, the S1
// terminator and undeploy-all. Java milestone(0) carries no step.
var infraNWTableContextCaseSteps = map[string][]string{
	"named-window": {
		"deploy|ctx||",
		"deploy|create||",
		"deploy|insert||",
		"send||SupportBean_S0|",
		"send||SupportBean|",
		"send||SupportBean|",
		"deploy|s1||",
		"deploy|s2||",
		"deploy|s3||",
		"deploy|s4||",
		"deploy|s5||",
		"deploy|s6||",
		"send||SupportBean_S1|",
		"undeploy-all|||",
	},
	"table": {
		"deploy|ctx||",
		"deploy|create||",
		"deploy|insert||",
		"send||SupportBean_S0|",
		"send||SupportBean|",
		"send||SupportBean|",
		"deploy|s1||",
		"deploy|s2||",
		"deploy|s3||",
		"deploy|s4||",
		"deploy|s5||",
		"deploy|s6||",
		"send||SupportBean_S1|",
		"undeploy-all|||",
	},
}

// infraNWTableContextBean mirrors SupportBean's asserted fields (theString,
// intPrimitive, longPrimitive).
type infraNWTableContextBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// infraNWTableContextS0 mirrors SupportBean_S0's asserted field (id).
type infraNWTableContextS0 struct {
	ID int `esper:"id"`
}

// infraNWTableContextS1 mirrors SupportBean_S1's asserted field (id).
type infraNWTableContextS1 struct {
	ID int `esper:"id"`
}

// infraNWTableContextCaseState carries the per-case replay state: the
// environment/engine pair, deployment bookkeeping for undeploy-all, and the
// trace/sequence counters the listener records draw from.
type infraNWTableContextCaseState struct {
	caseName    string
	env         *esper.Environment
	engine      *esper.Engine
	trace       *compat.Trace
	sequences   map[string]uint64
	deployments map[string]*esper.Deployment
	deployOrder []string
}

func runInfraNWTableContextScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range infraNWTableContextCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runInfraNWTableContextCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableContextID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", infraNWTableContextID, scenario.ID)
	}
	return trace, nil
}

func runInfraNWTableContextCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableContextBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableContextS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableContextS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(infraNWTableContextCaseRuntimeIDs[caseName]),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &infraNWTableContextCaseState{
		caseName:    caseName,
		env:         env,
		engine:      engine,
		trace:       &compat.Trace{Version: scenario.Version, ID: scenario.ID},
		sequences:   make(map[string]uint64),
		deployments: make(map[string]*esper.Deployment),
	}
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *state.trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *state.trace, err
			}
		case "send":
			event, err := decodeInfraNWTableContextPayload(step)
			if err != nil {
				return *state.trace, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return *state.trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *state.trace, err
			}
		default:
			return *state.trace, fmt.Errorf("%s: unsupported step op %q", infraNWTableContextID, step.Op)
		}
	}
	return *state.trace, nil
}

// deploy maps each scenario label to the equivalent Go registration or
// chain-API plan. Context and create-infra statements are env-level
// registrations (Go has no module path); the insert and s1..s6 statements
// build and deploy plans, and each sN deploy attaches the listener that
// emits the normalized per-delivery records, mirroring the Java
// register(env, path, num, epl) helper's addListener.
func (s *infraNWTableContextCaseState) deploy(ctx context.Context, step compat.Step) error {
	pinned, ok := infraNWTableContextDeployEPLs[s.caseName][step.Statement]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: deploy %q in case %q carries an unpinned EPL %q",
			infraNWTableContextID, step.Statement, s.caseName, step.Epl)
	}
	switch step.Statement {
	case "ctx":
		// `@public create context ContextOne start SupportBean_S0 end
		// SupportBean_S1` — a non-overlapping init-term context whose
		// partitions open on S0 and close on S1.
		isS0 := esper.Equal[string](
			esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
		isS1 := esper.Equal[string](
			esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1"))
		_, err := esper.CreateInitiatedTerminatedContext(s.env, "ContextOne",
			esper.Literal("global"), isS0, isS1)
		return err
	case "create":
		if s.caseName == "named-window" {
			// `@public context ContextOne create window MyInfra#keepall as
			// (pkey0 string, pkey1 int, c0 long)` — an inline map schema
			// bound to the context with keep-all retention.
			schema, err := esper.NewMapSchema("MyInfraSchema", []esper.FieldSpec{
				esper.FieldDef("pkey0", reflect.TypeOf("")),
				esper.FieldDef("pkey1", reflect.TypeOf(int(0))),
				esper.FieldDef("c0", reflect.TypeOf(int64(0))),
			})
			if err != nil {
				return err
			}
			if err := s.env.RegisterSchema(schema); err != nil {
				return err
			}
			_, err = esper.CreateNamedWindow(s.env, "MyInfra", schema,
				esper.NamedWindowContext("ContextOne"),
				esper.NamedWindowRetention(esper.KeepAll()))
			return err
		}
		// `@public context ContextOne create table MyInfra as (pkey0 string
		// primary key, pkey1 int primary key, c0 long)` — a keyed table
		// bound to the context.
		_, err := esper.CreateTable(s.env, "MyInfra", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("pkey0"),
			esper.PrimaryKeyColumn[int]("pkey1"),
			esper.TableColumnOf[int64]("c0"),
		}, esper.TableContext("ContextOne"))
		return err
	case "insert":
		// `context ContextOne insert into MyInfra select theString as
		// pkey0, intPrimitive as pkey1, longPrimitive as c0 from
		// SupportBean` — the contexted feed; the named-window form routes
		// the projected row, the table form assigns columns.
		beanSource := esper.From[infraNWTableContextBean](s.env, "SupportBean")
		var plan esper.Plan
		var err error
		if s.caseName == "named-window" {
			plan, err = s.env.Build(esper.Select(beanSource,
				esper.Alias("pkey0", esper.Field[infraNWTableContextBean, string]("theString")),
				esper.Alias("pkey1", esper.Field[infraNWTableContextBean, int]("intPrimitive")),
				esper.Alias("c0", esper.Field[infraNWTableContextBean, int64]("longPrimitive")),
			).InsertInto("MyInfra",
				esper.StatementName("insert"), esper.WithContext("ContextOne")))
		} else {
			plan, err = s.env.Build(esper.OnRecord(beanSource.AsRecord()).InsertIntoTable("MyInfra",
				esper.SetColumn("pkey0", esper.Field[any, string]("theString")),
				esper.SetColumn("pkey1", esper.Field[any, int]("intPrimitive")),
				esper.SetColumn("c0", esper.Field[any, int64]("longPrimitive")),
			).Query(esper.StatementName("insert"), esper.WithContext("ContextOne")))
		}
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, step.Statement, plan, false)
	case "s1", "s2", "s3", "s4", "s5", "s6":
		return s.deploySelect(ctx, step.Statement)
	default:
		return fmt.Errorf("%s: unknown deploy %q in case %q", infraNWTableContextID, step.Statement, s.caseName)
	}
}

// deploySelect builds and deploys one late s1..s6 select over the
// context-bound infra and attaches the traced listener. Each statement
// carries `output snapshot when terminated`, so it fires once when the
// SupportBean_S1 terminator ends the partition.
func (s *infraNWTableContextCaseState) deploySelect(ctx context.Context, label string) error {
	infra := esper.FromNamedWindow(s.env, "MyInfra")
	if s.caseName == "table" {
		infra = esper.FromTable(s.env, "MyInfra")
	}
	pkey0 := esper.Field[any, string]("pkey0")
	pkey1 := esper.Field[any, int]("pkey1")
	options := []esper.QueryOption{
		esper.StatementName(label),
		esper.WithContext("ContextOne"),
		esper.WithOutput(esper.OutputSnapshotWhenTerminated()),
	}
	var query esper.Query
	switch label {
	case "s1":
		// `select * from MyInfra output snapshot when terminated`.
		query = infra.Query(options...)
	case "s2":
		// `select count(*) as thecnt from MyInfra output snapshot when
		// terminated` — one row carrying the partition row count.
		query = infra.Aggregate(
			esper.Alias("thecnt", esper.CountAll()),
		).Query(options...)
	case "s3":
		// `select pkey0, count(*) as thecnt from MyInfra output snapshot
		// when terminated` — ungrouped aggregate emits one row per infra
		// row, each carrying the total count.
		query = infra.Aggregate(
			esper.Alias("pkey0", pkey0),
			esper.Alias("thecnt", esper.CountAll()),
		).Query(options...)
	case "s4":
		// `select pkey0, count(*) as thecnt from MyInfra group by pkey0
		// output snapshot when terminated`.
		query = infra.GroupBy(pkey0).Select(
			esper.Alias("pkey0", pkey0),
			esper.Alias("thecnt", esper.CountAll()),
		).Query(options...)
	case "s5":
		// `select pkey0, pkey1, count(*) as thecnt from MyInfra group by
		// pkey0 output snapshot when terminated` — pkey1 is not a group
		// key; Esper returns the group member's value.
		query = infra.GroupBy(pkey0).Select(
			esper.Alias("pkey0", pkey0),
			esper.Alias("pkey1", pkey1),
			esper.Alias("thecnt", esper.CountAll()),
		).Query(options...)
	case "s6":
		// `select pkey0, pkey1, count(*) as thecnt from MyInfra group by
		// rollup (pkey0, pkey1) output snapshot when terminated` — the
		// (pkey0,pkey1), (pkey0) and () grouping sets render grouped-away
		// columns as null.
		query = infra.GroupByRollup(pkey0, pkey1).Select(
			esper.Alias("pkey0", pkey0),
			esper.Alias("pkey1", pkey1),
			esper.Alias("thecnt", esper.CountAll()),
		).Query(options...)
	default:
		return fmt.Errorf("%s: unknown select %q in case %q", infraNWTableContextID, label, s.caseName)
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return err
	}
	return s.deployPlan(ctx, label, plan, true)
}

// normalizeInfraNWTableContextTrace sorts each case's six
// terminated-snapshot listener records into canonical statement order
// (s1..s6) on both traces before comparison. Java's context-partition
// teardown delivers `output snapshot when terminated` in reverse
// deployment order (s6..s1) on the SupportBean_S1 terminator while the Go
// engine delivers in forward deployment order (s1..s6); the Java
// execution itself asserts each statement's batch independently, so the
// cross-statement dispatch order is engine-internal. The sort is pinned
// to the exact run — one case, listener op, statements s1..s6 each once,
// sequence 1, one shared delivery time — so per-statement row content,
// sequences, streams and every other ordering stay compared exactly, and
// the canonicalization is stable under either engine's dispatch order.
func normalizeInfraNWTableContextTrace(trace compat.Trace) compat.Trace {
	order := map[string]int{"s1": 0, "s2": 1, "s3": 2, "s4": 3, "s5": 4, "s6": 5}
	for start := 0; start+len(order) <= len(trace.Records); start++ {
		run := trace.Records[start : start+len(order)]
		if !isInfraNWTableContextTerminationRun(run, order) {
			continue
		}
		sort.SliceStable(run, func(left, right int) bool {
			return order[run[left].Statement] < order[run[right].Statement]
		})
		start += len(order) - 1
	}
	return trace
}

func isInfraNWTableContextTerminationRun(run []compat.TraceRecord, order map[string]int) bool {
	caseName := run[0].Case
	at := run[0].Time
	seen := make(map[string]bool, len(order))
	for _, record := range run {
		if record.Case != caseName || record.Operation != "listener" ||
			record.Sequence != 1 || record.Time != at {
			return false
		}
		if _, ok := order[record.Statement]; !ok || seen[record.Statement] {
			return false
		}
		seen[record.Statement] = true
	}
	return len(seen) == len(order)
}

// deployPlan deploys one plan and, when traced, subscribes the listener
// that emits the normalized per-delivery records.
func (s *infraNWTableContextCaseState) deployPlan(ctx context.Context, label string,
	plan esper.Plan, traced bool) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	for _, statement := range deployment.Statements() {
		if !traced {
			continue
		}
		stmt := statement
		if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			s.record(stmt.Name(), batch)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// record emits one listener record per invocation with a per-statement
// sequence counter, mirroring the Java oracle's UpdateListener. Rows sort
// by their compact field rendering because the Java execution asserts the
// batches with assertPropsPerRowLastNewAnyOrder.
func (s *infraNWTableContextCaseState) record(statement string, batch esper.ResultBatch) {
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
	if len(rec.New) == 0 {
		rec.New = nil
	}
	if len(rec.Old) == 0 {
		rec.Old = nil
	}
	s.trace.Records = append(s.trace.Records, rec)
}

// undeployAll mirrors the Java execution's undeployAll: deployments retire
// in reverse deployment order; the env-scoped context and infra
// registrations retire with the engine.
func (s *infraNWTableContextCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", infraNWTableContextID, label, err)
		}
	}
	s.deployments = make(map[string]*esper.Deployment)
	s.deployOrder = nil
	return nil
}

func decodeInfraNWTableContextPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value infraNWTableContextBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value infraNWTableContextS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value infraNWTableContextS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWTableContextID, step.EventType)
	}
}

// loadInfraNWTableContextScenario decodes the scenario with the
// strict-shape checks the raw-mutation tests pin: no duplicate JSON keys,
// the exact top-level field set, pinned case metadata, and per-step field
// whitelists so unknown or duplicated step fields fail the replay.
func loadInfraNWTableContextScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableContextID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableContextID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableContextID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableContextID, err)
	}
	if err := requireInfraNWTableContextFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableContextID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableContextID ||
		metadata.Description != infraNWTableContextDescription ||
		metadata.JavaCommit != infraNWTableContextJavaCommit ||
		metadata.JavaSource != infraNWTableContextSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableContextID)
	}
	if err := validateInfraNWTableContextStringArray(root["javaRuntimes"], infraNWTableContextJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableContextStringArray(root["javaNames"], infraNWTableContextJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableContextStringArray(root["javaStaticIds"], infraNWTableContextJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableContextStringArray(root["javaFlags"], infraNWTableContextJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableContextCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableContextID, len(infraNWTableContextCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableContextFields(object,
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
		if definition.Case != infraNWTableContextCases[index] ||
			definition.Ordinal != infraNWTableContextOrdinals[index] ||
			definition.RuntimeID != infraNWTableContextJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableContextJavaExecutions[index] ||
			definition.Observation != infraNWTableContextCaseObservations[index] ||
			definition.EPL != infraNWTableContextCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableContextID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", infraNWTableContextID, err)
	}
	offset := 0
	for _, caseName := range infraNWTableContextCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", infraNWTableContextID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraNWTableContextID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", infraNWTableContextID, offset, caseName)
		}
		// The case marker is a step too: run it through the field whitelist
		// so an unexpected field on the marker is rejected like any other
		// step's extra field.
		if _, err := infraNWTableContextStepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraNWTableContextID, offset, err)
		}
		offset++
		want, ok := infraNWTableContextCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", infraNWTableContextID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", infraNWTableContextID, caseName)
		}
		for _, pinned := range want {
			key, err := infraNWTableContextStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraNWTableContextID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", infraNWTableContextID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", infraNWTableContextID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableContextID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// infraNWTableContextStepKey renders one raw step as its pinned key:
// op|statement|eventType|at. Unknown fields on the step object are
// rejected; send payloads are restricted to the registered event type's
// asserted fields.
func infraNWTableContextStepKey(raw json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return "", err
	}
	var step struct {
		Op        string          `json:"op"`
		Case      string          `json:"case"`
		Statement string          `json:"statement"`
		EventType string          `json:"eventType"`
		Epl       string          `json:"epl"`
		At        string          `json:"at"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"send":         {"op", "case", "eventType", "payload"},
		"undeploy-all": {"op", "case"},
	}
	fields, ok := allowed[step.Op]
	if !ok {
		return "", fmt.Errorf("step has unsupported op %q", step.Op)
	}
	for field := range object {
		found := false
		for _, name := range fields {
			if field == name {
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	if step.Op == "send" {
		payloadFields := map[string][]string{
			"SupportBean":    {"theString", "intPrimitive", "longPrimitive"},
			"SupportBean_S0": {"id"},
			"SupportBean_S1": {"id"},
		}
		allowedPayload, ok := payloadFields[step.EventType]
		if !ok {
			return "", fmt.Errorf("step has unknown event type %q", step.EventType)
		}
		var payload map[string]json.RawMessage
		if err := strictObject(step.Payload, &payload); err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		for field := range payload {
			found := false
			for _, name := range allowedPayload {
				if field == name {
					found = true
					break
				}
			}
			if !found {
				return "", fmt.Errorf("send payload has unexpected field %q", field)
			}
		}
	}
	return step.Op + "|" + step.Statement + "|" + step.EventType + "|" + step.At, nil
}

func requireInfraNWTableContextFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraNWTableContextID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraNWTableContextID, name)
		}
	}
	return nil
}

func validateInfraNWTableContextStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
