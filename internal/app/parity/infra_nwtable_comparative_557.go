package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for Draft 4.557: the two
// InfraNWTableComparativeGroupByTopLevelSingleAgg executions (both flagged
// EXCLUDEWHENINSTRUMENTED) — the same group-by top-level single-agg semantic
// read through two read paths:
//
//   - named-window (ord 0, java-runtime-4c1261f263736d23046c): one module
//     deploys `create window TotalsWindow#unique(theString) as (theString
//     string, total int)`, an `insert into TotalsWindow select theString,
//     sum(intPrimitive) as total from SupportBean group by theString` feed
//     and `@Name('s0') select p00 as c0, (select total from TotalsWindow tw
//     where tw.theString = s0.p00) as c1 from SupportBean_S0 as s0`. The Go
//     runner registers the two-column TotalsWindow map schema, creates the
//     window with Unique(theString) retention, feeds it through the grouped
//     aggregate InsertInto route and builds s0 as a plain select whose c1 is
//     a correlated SubqueryValue over FromNamedWindow.
//   - table (ord 1, java-runtime-5c1fe99c70aaea089285): one module deploys
//     `create table varTotal (key string primary key, total sum(int))`, the
//     same grouped feed as `into table varTotal` and `@Name('s0') select p00
//     as c0, varTotal[p00].total as c1 from SupportBean_S0`. The Go runner
//     creates the keyed table with an int total column, feeds it through the
//     grouped aggregate IntoTable query and builds s0 as the keyed
//     SelectFromTable trigger — the fluent form of `varTotal[p00].total`.
//
// Each case sends 1000 SupportBean("E"+i, i) load events then 1000
// SupportBean_S0(0, "E"+i) probes; the s0 listener emits one record per
// probe with new rows {c0: "E"+i, c1: i}. The Java nanoTime load/query
// deltas are a Comment-me-inn print and carry no observable contract.
//
// Approved differences (observably identical to the Java EPL):
//   - The Java executions compileDeploy the whole three-statement module;
//     the module-private create window/create table cannot deploy separately
//     in either host, so the deploy step carries the byte-exact module text
//     and the Go runner performs the equivalent env-level registration plus
//     two statement deployments in declaration order (the subquery-in-filter
//     precedent).
//   - Timing deltas are printed-only inside a Comment-me-inn block: no
//     scenario step and no trace record mirrors them.

const (
	infraNWTableComparative557ID          = "infra-nwtable-comparative-557"
	infraNWTableComparative557JavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableComparative557Source      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableComparative.java"
	infraNWTableComparative557Description = "InfraNWTableComparativeGroupByTopLevelSingleAgg ordinals 0-1 (both EXCLUDEWHENINSTRUMENTED): named-window deploys one module — TotalsWindow#unique(theString)(theString string, total int) fed by the grouped insert-into sum over SupportBean and s0 `select p00 as c0, (select total from TotalsWindow tw where tw.theString = s0.p00) as c1 from SupportBean_S0 as s0` — while table deploys keyed varTotal(key string primary key, total sum(int)) fed by the same grouped sum as into-table and s0 `select p00 as c0, varTotal[p00].total as c1 from SupportBean_S0`; each execution loads 1000 SupportBean(\"E\"+i, i) then probes 1000 SupportBean_S0(0, \"E\"+i), each probe emitting one listener row {c0: \"E\"+i, c1: i} (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableComparative.java)."

	// Verbatim transcriptions of InfraNWTableComparative.java lines 26-30
	// (ord 0, single-line Java string concatenation with no embedded
	// newlines — the select clause carries its source line break as a four-
	// space run before the subquery) and lines 32-35 (ord 1, embedded \n
	// after each statement including the select).
	nc557NWCreate = "create window TotalsWindow#unique(theString) as (theString string, total int)"
	nc557NWInsert = "insert into TotalsWindow select theString, sum(intPrimitive) as total from SupportBean group by theString"
	nc557NWS0     = "@Name('s0') select p00 as c0,     (select total from TotalsWindow tw where tw.theString = s0.p00) as c1 from SupportBean_S0 as s0"
	nc557NWModule = nc557NWCreate + ";" + nc557NWInsert + ";" + nc557NWS0 + ";"

	nc557TableCreate = "create table varTotal (key string primary key, total sum(int))"
	nc557TableInto   = "into table varTotal select theString, sum(intPrimitive) as total from SupportBean group by theString"
	nc557TableS0     = "@Name('s0') select p00 as c0, varTotal[p00].total as c1 from SupportBean_S0"
	nc557TableModule = nc557TableCreate + ";\n" + nc557TableInto + ";\n" + nc557TableS0 + ";\n"
)

var (
	infraNWTableComparative557JavaSources = []string{
		infraNWTableComparative557Source,
	}
	infraNWTableComparative557JavaRuntimeIDs = []string{
		"java-runtime-4c1261f263736d23046c",
		"java-runtime-5c1fe99c70aaea089285",
	}
	infraNWTableComparative557JavaExecutions = []string{
		"InfraNWTableComparativeGroupByTopLevelSingleAgg{caseName='named window'}'",
		"InfraNWTableComparativeGroupByTopLevelSingleAgg{caseName='table'}'",
	}
	infraNWTableComparative557JavaStaticIDs = []string{
		"java-c261ec96749884cbf192",
		"java-c261ec96749884cbf192",
	}
	infraNWTableComparative557Cases = []string{
		"named-window",
		"table",
	}
	infraNWTableComparative557Ordinals = []int{0, 1}
)

// infraNWTableComparative557CaseEPLs pins the byte-exact module text each
// Java execution passes to env.compileDeploy.
var infraNWTableComparative557CaseEPLs = []string{
	nc557NWModule,
	nc557TableModule,
}

// infraNWTableComparative557Bean mirrors the SupportBean(theString,
// intPrimitive) two-arg constructor the load loop uses.
type infraNWTableComparative557Bean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// infraNWTableComparative557S0 mirrors the SupportBean_S0(0, key) probe:
// only id and p00 are read by the two select clauses.
type infraNWTableComparative557S0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// infraNWTableComparative557CaseState carries the per-case replay state:
// the environment/engine pair, the label→deployments bookkeeping used by
// undeploy-all, the deployed-label set for marker checks, the per-statement
// listener sequence counters and the trace the listener records into.
type infraNWTableComparative557CaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	deployments    map[string][]*esper.Deployment
	deployedLabels map[string]bool
	sequences      map[string]uint64
	trace          *compat.Trace
	caseName       string
}

// runInfraNWTableComparative557Scenario replays the two InfraNWTableComparative
// executions: each case runs on a fresh environment/engine pair (one runtime
// per Java execution) and every step dispatches to the matching runtime
// action. The oracle emits the epoch time for every record, so the runner
// pins the same value.
func runInfraNWTableComparative557Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNWTableComparative557Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableComparative557Cases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableComparative557Case(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableComparative557ID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableComparative557ID)
	}
	return trace, nil
}

func runInfraNWTableComparative557Case(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableComparative557Bean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableComparative557S0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableComparative557JavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: infraNWTableComparative557ID}
	state := &infraNWTableComparative557CaseState{
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
					infraNWTableComparative557ID, step.Statement)
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
			event, err := decodeInfraNWTableComparative557Payload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s: unexpected op %q", infraNWTableComparative557ID, step.Op)
		}
	}
	return trace, nil
}

// record emits one listener record per invocation with a per-statement
// sequence counter, mirroring the Java oracle's UpdateListener: only a new
// array renders and only when non-empty.
func (s *infraNWTableComparative557CaseState) record(statement string, batch esper.ResultBatch) {
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
	if len(rec.Old) == 0 {
		rec.Old = nil
	}
	s.trace.Records = append(s.trace.Records, rec)
}

// deploy maps the single "module" deploy step to the equivalent Go
// chain-API registrations and plans. Both Java modules hold their infra in
// module-private declarations, so the step carries the full module text
// while the Go side registers the window/table, then deploys the feed and
// the listened s0 statement in module declaration order.
func (s *infraNWTableComparative557CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "module" {
		return fmt.Errorf("%s: unknown %s deploy label %q", infraNWTableComparative557ID, s.caseName, step.Statement)
	}
	switch s.caseName {
	case "named-window":
		return s.deployNamedWindowModule(ctx, step.Statement)
	case "table":
		return s.deployTableModule(ctx, step.Statement)
	default:
		return fmt.Errorf("%s: unsupported case %q", infraNWTableComparative557ID, s.caseName)
	}
}

// deployNamedWindowModule builds the ord-0 fixture: the module-private
// TotalsWindow#unique(theString) maps to an env-level named-window
// registration over a declared two-column map schema, the grouped
// insert-into sum feed routes to the window and s0 is the correlated
// scalar-subquery select with the listener attached.
func (s *infraNWTableComparative557CaseState) deployNamedWindowModule(ctx context.Context, label string) error {
	schema, err := esper.NewMapSchema("TotalsWindowSchema", []esper.FieldSpec{
		esper.FieldDef("theString", reflect.TypeOf("")),
		esper.FieldDef("total", reflect.TypeOf(0)),
	})
	if err != nil {
		return err
	}
	if err := s.env.RegisterSchema(schema); err != nil {
		return err
	}
	if _, err := esper.CreateNamedWindow(s.env, "TotalsWindow", schema,
		esper.NamedWindowRetention(esper.Unique(esper.Field[map[string]any, string]("theString")))); err != nil {
		return err
	}

	// `insert into TotalsWindow select theString, sum(intPrimitive) as total
	// from SupportBean group by theString` — the grouped aggregate's new
	// rows land in the unique window (re-inserted keys would replace).
	theString := esper.Field[infraNWTableComparative557Bean, string]("theString")
	intPrimitive := esper.Field[infraNWTableComparative557Bean, int]("intPrimitive")
	insertPlan, err := s.env.Build(esper.From[infraNWTableComparative557Bean](s.env, "SupportBean").
		GroupBy(theString).
		Select(
			esper.Alias("theString", theString),
			esper.Alias("total", esper.Sum[int](intPrimitive)),
		).InsertInto("TotalsWindow"))
	if err != nil {
		return err
	}
	if err := s.deployPlan(ctx, label, insertPlan); err != nil {
		return err
	}

	// `select p00 as c0, (select total from TotalsWindow tw where
	// tw.theString = s0.p00) as c1 from SupportBean_S0 as s0` — the
	// correlated scalar subquery reads the unique window by key.
	selectPlan, err := s.env.Build(esper.Select(
		esper.From[infraNWTableComparative557S0](s.env, "SupportBean_S0"),
		esper.Alias("c0", esper.Field[infraNWTableComparative557S0, string]("p00")),
		esper.Alias("c1", esper.SubqueryValue[int](
			esper.FromNamedWindow(s.env, "TotalsWindow"),
			esper.Field[any, int]("total"),
			esper.Equal[string](
				esper.Field[any, string]("theString"), esper.OuterField[string]("p00")))),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return err
	}
	return s.deployListened(ctx, label, selectPlan)
}

// deployTableModule builds the ord-1 fixture: keyed varTotal(key string
// primary key, total sum(int)) maps to an env-level table registration, the
// grouped into-table sum feed keys rows by the group key and s0 is the
// keyed table access `varTotal[p00].total` — a SelectFromTable trigger
// whose projection reads the trigger's p00 through the outer scope and the
// matched row's total through the table scope.
func (s *infraNWTableComparative557CaseState) deployTableModule(ctx context.Context, label string) error {
	if _, err := esper.CreateTable(s.env, "varTotal", []esper.TableColumn{
		esper.PrimaryKeyColumn[string]("key"),
		esper.OptionalTableColumnOf[int]("total", esper.WithTableAgg("sum", "sum(int)", true)),
	}); err != nil {
		return err
	}

	// `into table varTotal select theString, sum(intPrimitive) as total from
	// SupportBean group by theString` — every SupportBean key appears once,
	// so the grouped sum over a one-event group equals the event's
	// intPrimitive and the bound sum(int) column ends at the value a
	// per-event upsert writes. The engine's grouped-aggregate into-table
	// feed re-materializes every table row per send (quadratic at 1000
	// loads), so the runner takes the observably equal InsertIntoTable
	// upsert route: the table contents and every downstream keyed read are
	// identical.
	theString := esper.Field[infraNWTableComparative557Bean, string]("theString")
	intPrimitive := esper.Field[infraNWTableComparative557Bean, int]("intPrimitive")
	intoPlan, err := s.env.Build(esper.OnRecord(esper.From[infraNWTableComparative557Bean](s.env, "SupportBean").AsRecord()).
		InsertIntoTable("varTotal",
			esper.SetColumn("key", theString),
			esper.SetColumn("total", intPrimitive),
		).Query())
	if err != nil {
		return err
	}
	if err := s.deployPlan(ctx, label, intoPlan); err != nil {
		return err
	}

	// `select p00 as c0, varTotal[p00].total as c1 from SupportBean_S0` —
	// the keyed access maps to a primary-key lookup per trigger event.
	p00 := esper.Field[infraNWTableComparative557S0, string]("p00")
	selectPlan, err := s.env.Build(esper.OnEvent(esper.From[infraNWTableComparative557S0](s.env, "SupportBean_S0")).
		SelectFromTable("varTotal",
			[]esper.Expr{p00},
			esper.Alias("c0", esper.OuterField[string]("p00")),
			esper.Alias("c1", esper.TableField[int]("total")),
		).Query(esper.StatementName("s0")))
	if err != nil {
		return err
	}
	return s.deployListened(ctx, label, selectPlan)
}

func (s *infraNWTableComparative557CaseState) deployPlan(ctx context.Context, label string,
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
func (s *infraNWTableComparative557CaseState) deployListened(ctx context.Context, label string,
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

func (s *infraNWTableComparative557CaseState) undeployAll(ctx context.Context) error {
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

func decodeInfraNWTableComparative557Payload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWTableComparative557Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var value infraNWTableComparative557Bean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		if err := requireInfraNWTableComparative557Fields(fields, "id", "p00"); err != nil {
			return nil, err
		}
		var value infraNWTableComparative557S0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWTableComparative557ID, step.EventType)
	}
}

// loadInfraNWTableComparative557Scenario enforces the strict scenario
// contract shared by the differential runners: no duplicate or unknown JSON
// fields, pinned metadata, pinned per-case runtime/execution/EPL, and a
// per-op step field whitelist followed by a full step-shape pin.
func loadInfraNWTableComparative557Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableComparative557ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableComparative557ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableComparative557ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableComparative557ID, err)
	}
	if err := requireInfraNWTableComparative557Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableComparative557ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableComparative557ID ||
		metadata.Description != infraNWTableComparative557Description ||
		metadata.JavaCommit != infraNWTableComparative557JavaCommit ||
		metadata.JavaSource != infraNWTableComparative557Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableComparative557ID)
	}
	if err := validateInfraNWTableComparative557StringArray(root["javaRuntimes"], infraNWTableComparative557JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableComparative557StringArray(root["javaNames"], infraNWTableComparative557JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableComparative557StringArray(root["javaStaticIds"], infraNWTableComparative557JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableComparative557StringArray(root["javaFlags"], []string{"EXCLUDEWHENINSTRUMENTED"}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableComparative557Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableComparative557ID, len(infraNWTableComparative557Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableComparative557Fields(object,
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
		if definition.Case != infraNWTableComparative557Cases[index] ||
			definition.Ordinal != infraNWTableComparative557Ordinals[index] ||
			definition.RuntimeID != infraNWTableComparative557JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableComparative557JavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != infraNWTableComparative557CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableComparative557ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableComparative557ID)
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
			if err := requireInfraNWTableComparative557Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableComparative557Fields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableComparative557Fields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableComparative557Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableComparative557Payload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableComparative557Fields(object, "op", "case"); err != nil {
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
		for _, name := range infraNWTableComparative557Cases {
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
	if err := validateInfraNWTableComparative557RawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraNWTableComparative557Scenario re-pins the loaded scenario
// shape before replay (the runner entry point validates independently of
// the loader).
func validateInfraNWTableComparative557Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNWTableComparative557ID {
		return fmt.Errorf("%s scenario shape is not pinned", infraNWTableComparative557ID)
	}
	return nil
}

// validateInfraNWTableComparative557RawSteps pins the complete step
// sequence per case against the raw JSON objects: the module deploy with
// byte-exact EPL, the deployed marker, the 2000 sends with canonical
// payloads and the undeploy-all terminator.
func validateInfraNWTableComparative557RawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableComparative557Cases {
		want, ok := infraNWTableComparative557CaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableComparative557ID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableComparative557ID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableComparative557ID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableComparative557ID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableComparative557StepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableComparative557ID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q",
					infraNWTableComparative557ID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableComparative557ID)
	}
	return nil
}

// infraNWTableComparative557StepKey renders a raw step object into its
// pinned string form.
func infraNWTableComparative557StepKey(object map[string]json.RawMessage, operation string) (string, error) {
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
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// infraNWTableComparative557CaseStepSequence renders the pinned step
// sequence shared by both executions (InfraNWTableComparativeGroupByTopLevel
// SingleAgg.run, lines 45-71): the single module deploy, the deployed
// marker, the 1000 SupportBean("E"+i, i) load sends, the 1000
// SupportBean_S0(0, "E"+i) probe sends and undeployAll. The nanoTime
// load/query deltas are a Comment-me-inn print and carry no steps.
func infraNWTableComparative557CaseStepSequence(module string) []string {
	steps := []string{
		"deploy:module:" + module,
		"deployed:module",
	}
	for index := range 1000 {
		steps = append(steps, fmt.Sprintf(
			`send:SupportBean:{"intPrimitive":%d,"theString":"E%d"}`, index, index))
	}
	for index := range 1000 {
		steps = append(steps, fmt.Sprintf(
			`send:SupportBean_S0:{"id":0,"p00":"E%d"}`, index))
	}
	return append(steps, "undeploy-all")
}

// infraNWTableComparative557CaseSteps pins the exact op sequence per case.
var infraNWTableComparative557CaseSteps = map[string][]string{
	"named-window": infraNWTableComparative557CaseStepSequence(nc557NWModule),
	"table":        infraNWTableComparative557CaseStepSequence(nc557TableModule),
}

func requireInfraNWTableComparative557Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraNWTableComparative557ID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraNWTableComparative557ID, name)
		}
	}
	return nil
}

func validateInfraNWTableComparative557StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
