package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/liubaicai/esper/internal/compat"
	"github.com/liubaicai/esper/internal/esper"
)

// infraTableJoinBean mirrors SupportBean for the table-join executions.
type infraTableJoinBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type infraTableJoinS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

type infraTableJoinS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

const (
	infraTableJoinID          = "infra-table-join"
	infraTableJoinJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraTableJoinJavaSource  = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableJoin.java"
	infraTableJoinDescription = "InfraTableJoin ords 0/3/4/5: from-clause comma join of stream to keyed table, unkeyed table surviving module undeploy, unidirectional left outer join with null fill, and inner join with on-clause plus select * fragment access."
)

var infraTableJoinJavaSources = []string{infraTableJoinJavaSource}

// Case order fixes the runtime-ID index mapping below.
var infraTableJoinCases = []string{
	"from-clause",
	"unkeyed",
	"outer-join",
	"inner-join-on",
}

var infraTableJoinJavaRuntimeIDs = []string{
	"java-runtime-c7ba930cd8efcf4a0daa",
	"java-runtime-bd56535c23535c3e90ce",
	"java-runtime-375040b0fa5a7b77c5aa",
	"java-runtime-7e0cba3e4899478393ef",
}

var infraTableJoinJavaExecutions = []string{
	"InfraFromClause",
	"InfraUnkeyedTable",
	"InfraOuterJoin",
	"InfraInnerJoinWithOnClause",
}

var infraTableJoinJavaStaticIDs = []string{
	"java-810b53325c6857c0d476",
	"java-353b57ef718eb50ea62e",
	"java-46bca199882ab8e60624",
	"java-a70a422119e7d37641a0",
}

var infraTableJoinJavaFlags = []string{}

var infraTableJoinCaseObservations = []string{
	"listener; comma join of SupportBean_S0 to keyed table varaggFC via where va.key=s0.p00; S0(0,'G1') yields value=100 after SupportBean(G1,100), S0(0,'G2') silent until SupportBean(G2,200) populates the key",
	"listener; unkeyed table MyTable(sumint sum(int)) fed by into-table aggregation survives undeployModuleContaining('into'); join select sumint from MyTable, SupportBean yields 201; SecondTable(a,b) via positional FAF insert values('a1',10) yields a1/10",
	"listener; unidirectional left outer join of SupportBean to MyTable on theString=p0; FAF insert 'a'/10 then SupportBean('a',0) yields theString=a,p1=10 and SupportBean('b',0) yields theString=b,p1=null",
	"listener; inner join MyTable mt to SupportBean_S1 s1 on p10=prop; S0(0,'K1','A') populates the table, S1(10,'X') silent, S1(11,'A') yields mt.key1=K1,mt.prop=A via select * fragment access",
}

var infraTableJoinCaseEPLs = []string{
	"@public create table varaggFC as (key string primary key, total sum(int)); into table varaggFC select sum(intPrimitive) as total from SupportBean group by theString; @name('s0') select total as value from SupportBean_S0 as s0, varaggFC as va where va.key = s0.p00",
	"@public create table MyTable (sumint sum(int)); @name('into') into table MyTable select sum(intPrimitive) as sumint from SupportBean; @name('join') select sumint from MyTable, SupportBean; @public create table SecondTable (a string, b int); @name('s0')select a, b from SecondTable, SupportBean",
	"@public create table MyTable as (p0 string primary key, p1 int); @name('s0') select theString, p1 from SupportBean unidirectional left outer join MyTable on theString = p0",
	"create table MyTable(key1 string primary key, prop string);insert into MyTable select p00 as key1, p01 as prop from SupportBean_S0;@name('s0') select * from MyTable as mt inner join SupportBean_S1 as s1 on p10 = prop;",
}

// infraTableJoinCaseSteps pins the complete step sequence per case as
// op|statement|eventType triples so the runtime-ID mapping test can assert
// the scenario file matches the contract.
var infraTableJoinCaseSteps = map[string][]string{
	"from-clause": {
		"deploy|table|",
		"deployed|table|",
		"deploy|into|",
		"deployed|into|",
		"deploy|s0|",
		"deployed|s0|",
		"send||SupportBean",
		"send||SupportBean_S0",
		"send||SupportBean_S0",
		"send||SupportBean",
		"send||SupportBean_S0",
		"send||SupportBean_S0",
		"undeploy-all||",
	},
	"unkeyed": {
		"deploy|table|",
		"deployed|table|",
		"deploy|into|",
		"deployed|into|",
		"send||SupportBean",
		"send||SupportBean",
		"undeploy|into|",
		"deploy|join|",
		"deployed|join|",
		"send||SupportBean",
		"undeploy|join|",
		"deploy|table2|",
		"deployed|table2|",
		"faf|faf|",
		"deploy|s0|",
		"deployed|s0|",
		"send||SupportBean",
		"undeploy-all||",
	},
	"outer-join": {
		"deploy|table|",
		"deployed|table|",
		"faf|faf|",
		"deploy|s0|",
		"deployed|s0|",
		"send||SupportBean",
		"send||SupportBean",
		"undeploy-all||",
	},
	"inner-join-on": {
		"deploy|module|",
		"deployed|module|",
		"deployed|s0|",
		"send||SupportBean_S0",
		"send||SupportBean_S1",
		"send||SupportBean_S1",
		"undeploy-all||",
	},
}

// infraTableJoinDeployEPLs pins the byte-exact EPL attached to each deploy
// step so a scenario that drifts from the Java source fails to load.
var infraTableJoinDeployEPLs = map[string]map[string]string{
	"from-clause": {
		"table": "@public create table varaggFC as (key string primary key, total sum(int))",
		"into":  "into table varaggFC select sum(intPrimitive) as total from SupportBean group by theString",
		"s0":    "@name('s0') select total as value from SupportBean_S0 as s0, varaggFC as va where va.key = s0.p00",
	},
	"unkeyed": {
		"table":  "@public create table MyTable (sumint sum(int))",
		"into":   "@name('into') into table MyTable select sum(intPrimitive) as sumint from SupportBean",
		"join":   "@name('join') select sumint from MyTable, SupportBean",
		"table2": "@public create table SecondTable (a string, b int)",
		"s0":     "@name('s0')select a, b from SecondTable, SupportBean",
	},
	"outer-join": {
		"table": "@public create table MyTable as (p0 string primary key, p1 int)",
		"s0":    "@name('s0') select theString, p1 from SupportBean unidirectional left outer join MyTable on theString = p0",
	},
	"inner-join-on": {
		"module": "create table MyTable(key1 string primary key, prop string);insert into MyTable select p00 as key1, p01 as prop from SupportBean_S0;@name('s0') select * from MyTable as mt inner join SupportBean_S1 as s1 on p10 = prop;",
	},
}

// infraTableJoinFafEPLs pins the fire-and-forget insert statements.
var infraTableJoinFafEPLs = map[string]map[string]string{
	"unkeyed":    {"faf": "insert into SecondTable values ('a1', 10)"},
	"outer-join": {"faf": "insert into MyTable select 'a' as p0, 10 as p1"},
}

// infraTableJoinCaseState carries the per-case replay state: the
// environment/engine pair plus label→deployments bookkeeping used by the
// deploy and undeploy-module handlers.
type infraTableJoinCaseState struct {
	env         *esper.Environment
	engine      *esper.Engine
	deployments map[string][]*esper.Deployment
	caseName    string
}

func runInfraTableJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraTableJoinScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeInfraTableJoin(ctx, scenario, &trace)
}

func validateInfraTableJoinScenario(scenario compat.Scenario) error {
	if scenario.Version != compat.ScenarioVersion || scenario.ID != infraTableJoinID {
		return fmt.Errorf("%s: scenario identity = %q/%q", infraTableJoinID, scenario.Version, scenario.ID)
	}
	return scenario.Validate()
}

// executeInfraTableJoin replays the scenario: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action. The oracle emits the literal
// time "0" for every record, so the runner pins the same value.
func executeInfraTableJoin(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *infraTableJoinCaseState
	sequences := make(map[string]uint64)
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *trace, err
		}
		switch step.Op {
		case "case":
			if state != nil && state.engine != nil {
				_ = state.engine.Close(context.Background())
			}
			var err error
			state, err = startInfraTableJoinCase(step.Case)
			if err != nil {
				return *trace, err
			}
			sequences = make(map[string]uint64)
		case "deploy":
			if err := state.deploy(ctx, step, trace, sequences); err != nil {
				return *trace, err
			}
		case "deployed":
			sequences[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      state.caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequences[step.Statement+":deployed"],
				Time:      "0",
			})
		case "send":
			event, err := decodeInfraTableJoinPayload(step)
			if err != nil {
				return *trace, err
			}
			if err := state.engine.Send(ctx, step.EventType, event); err != nil {
				return *trace, err
			}
		case "faf":
			if err := state.fafInsert(ctx, step); err != nil {
				return *trace, err
			}
		case "undeploy":
			for _, deployment := range state.deployments[step.Statement] {
				if err := deployment.Undeploy(ctx); err != nil {
					return *trace, err
				}
			}
			delete(state.deployments, step.Statement)
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", infraTableJoinID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

func startInfraTableJoinCase(caseName string) (*infraTableJoinCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraTableJoinBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraTableJoinS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraTableJoinS1](env, "SupportBean_S1"); err != nil {
		return nil, err
	}
	return &infraTableJoinCaseState{
		env:         env,
		engine:      esper.NewEngine(env),
		deployments: make(map[string][]*esper.Deployment),
		caseName:    caseName,
	}, nil
}

// deploy maps each scenario label to the equivalent Go chain-API plans. The
// Java oracle compiles each deploy step's EPL as one module; the Go runner
// deploys the equivalent plans and registers them under the step label so
// undeploy-module/undeploy-all bookkeeping matches.
func (s *infraTableJoinCaseState) deploy(ctx context.Context, step compat.Step,
	trace *compat.Trace, sequences map[string]uint64) error {
	var plans []esper.Plan
	var err error
	switch s.caseName {
	case "from-clause":
		plans, err = s.deployFromClause(step.Statement)
	case "unkeyed":
		plans, err = s.deployUnkeyed(step.Statement)
	case "outer-join":
		plans, err = s.deployOuterJoin(step.Statement)
	case "inner-join-on":
		plans, err = s.deployInnerJoinOn(step.Statement)
	default:
		return fmt.Errorf("%s: unsupported case %q", infraTableJoinID, s.caseName)
	}
	if err != nil {
		return err
	}
	for _, plan := range plans {
		if err := s.deployPlan(ctx, step.Statement, plan, trace, sequences); err != nil {
			return err
		}
	}
	return nil
}

// deployFromClause builds the from-clause case: a keyed table fed by a
// grouped into-table aggregate plus the comma join of SupportBean_S0 to the
// table with the where-equality on the table primary key.
func (s *infraTableJoinCaseState) deployFromClause(label string) ([]esper.Plan, error) {
	switch label {
	case "table":
		if _, err := esper.CreateTable(s.env, "varaggFC", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("key"),
			esper.TableColumnOf[int]("total"),
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "into":
		theString := esper.Field[infraTableJoinBean, string]("theString")
		plan, err := s.env.Build(esper.From[infraTableJoinBean](s.env, "SupportBean").
			GroupBy(theString).
			Select(
				esper.Alias("key", theString),
				esper.Alias("total", esper.Sum[int](esper.Field[infraTableJoinBean, int]("intPrimitive"))),
			).IntoTable("varaggFC"))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	case "s0":
		s0 := esper.From[infraTableJoinS0](s.env, "SupportBean_S0")
		plan, err := s.env.Build(esper.JoinMany(
			esper.JoinSource(s0),
			esper.JoinRecordSource(esper.FromTable(s.env, "varaggFC")),
		).On(
			esper.OnSourcesEqual(1, esper.JoinField[string](1, "key"), 0, esper.JoinField[string](0, "p00")),
		).Select(
			esper.SelectFrom(1, "value", esper.JoinField[int](1, "total")),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	default:
		return nil, fmt.Errorf("%s: unknown from-clause deploy label %q", infraTableJoinID, label)
	}
}

// deployUnkeyed builds the unkeyed-table case: an unkeyed aggregate table
// that survives undeployModuleContaining, a Cartesian join of the table to
// SupportBean, and a second table populated by a positional FAF insert.
func (s *infraTableJoinCaseState) deployUnkeyed(label string) ([]esper.Plan, error) {
	switch label {
	case "table":
		if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
			esper.TableColumnOf[int]("sumint"),
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "into":
		plan, err := s.env.Build(esper.From[infraTableJoinBean](s.env, "SupportBean").
			Aggregate(
				esper.Alias("sumint", esper.Sum[int](esper.Field[infraTableJoinBean, int]("intPrimitive"))),
			).IntoTable("MyTable"))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	case "join":
		plan, err := s.env.Build(esper.JoinMany(
			esper.JoinRecordSource(esper.FromTable(s.env, "MyTable")),
			esper.JoinSource(esper.From[infraTableJoinBean](s.env, "SupportBean")),
		).Select(
			esper.SelectFrom(0, "sumint", esper.JoinField[int](0, "sumint")),
		).Query(esper.StatementName("join")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	case "table2":
		if _, err := esper.CreateTable(s.env, "SecondTable", []esper.TableColumn{
			esper.TableColumnOf[string]("a"),
			esper.TableColumnOf[int]("b"),
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "s0":
		plan, err := s.env.Build(esper.JoinMany(
			esper.JoinRecordSource(esper.FromTable(s.env, "SecondTable")),
			esper.JoinSource(esper.From[infraTableJoinBean](s.env, "SupportBean")),
		).Select(
			esper.SelectFrom(0, "a", esper.JoinField[string](0, "a")),
			esper.SelectFrom(0, "b", esper.JoinField[int](0, "b")),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	default:
		return nil, fmt.Errorf("%s: unknown unkeyed deploy label %q", infraTableJoinID, label)
	}
}

// deployOuterJoin builds the unidirectional left outer join of SupportBean
// to a keyed table; a miss materializes the null side so p1 reads null.
func (s *infraTableJoinCaseState) deployOuterJoin(label string) ([]esper.Plan, error) {
	switch label {
	case "table":
		if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("p0"),
			esper.TableColumnOf[int]("p1"),
		}); err != nil {
			return nil, err
		}
		return nil, nil
	case "s0":
		sb := esper.From[infraTableJoinBean](s.env, "SupportBean")
		plan, err := s.env.Build(esper.JoinChain(esper.JoinSource(sb).Unidirectional()).
			LeftOuterJoin(esper.JoinRecordSource(esper.FromTable(s.env, "MyTable")),
				esper.OnSourcesEqual(0, esper.JoinField[string](0, "theString"), 1, esper.JoinField[string](1, "p0")),
			).Select(
			esper.SelectFrom(0, "theString", esper.JoinField[string](0, "theString")),
			esper.SelectFrom(1, "p1", esper.JoinField[int](1, "p1")),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	default:
		return nil, fmt.Errorf("%s: unknown outer-join deploy label %q", infraTableJoinID, label)
	}
}

// deployInnerJoinOn builds the inner-join-on module: a keyed table populated
// by an on-event insert trigger plus the inner join of the table to
// SupportBean_S1 on a non-key column with select-* fragment access.
func (s *infraTableJoinCaseState) deployInnerJoinOn(label string) ([]esper.Plan, error) {
	switch label {
	case "module":
		if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("key1"),
			esper.TableColumnOf[string]("prop"),
		}); err != nil {
			return nil, err
		}
		insertPlan, err := s.env.Build(esper.OnEvent(esper.From[infraTableJoinS0](s.env, "SupportBean_S0")).
			InsertIntoTable("MyTable",
				esper.SetColumn("key1", esper.Field[infraTableJoinS0, string]("p00")),
				esper.SetColumn("prop", esper.Field[infraTableJoinS0, string]("p01")),
			).Query())
		if err != nil {
			return nil, err
		}
		joinPlan, err := s.env.Build(esper.JoinChain(esper.JoinRecordSource(esper.FromTable(s.env, "MyTable"))).
			InnerJoin(esper.JoinSource(esper.From[infraTableJoinS1](s.env, "SupportBean_S1")),
				esper.OnSourcesEqual(1, esper.JoinField[string](1, "p10"), 0, esper.JoinField[string](0, "prop")),
			).Select(
			esper.SelectSourceEvent(0, "mt"),
			esper.SelectSourceEvent(1, "s1"),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insertPlan, joinPlan}, nil
	default:
		return nil, fmt.Errorf("%s: unknown inner-join-on deploy label %q", infraTableJoinID, label)
	}
}

func (s *infraTableJoinCaseState) deployPlan(ctx context.Context, label string,
	plan esper.Plan, trace *compat.Trace, sequences map[string]uint64) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[label] = append(s.deployments[label], deployment)
	for _, statement := range deployment.Statements() {
		name := statement.Name()
		if name != "s0" && name != "join" {
			continue
		}
		stmt := statement
		if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			sequences[stmt.Name()+":listener"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      s.caseName,
				Operation: "listener",
				Statement: stmt.Name(),
				Sequence:  sequences[stmt.Name()+":listener"],
				Time:      "0",
				New:       infraTableJoinNormalizeResults(batch.New),
				Old:       infraTableJoinNormalizeResults(batch.Old),
			})
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// fafInsert executes the pinned fire-and-forget insert (positional or
// select-form per the pinned EPL), mirroring compileExecuteFAFNoResult.
func (s *infraTableJoinCaseState) fafInsert(ctx context.Context, step compat.Step) error {
	var plan esper.Plan
	var err error
	switch s.caseName {
	case "unkeyed":
		plan, err = s.env.Build(esper.FromTable(s.env, "SecondTable").OnDemand().InsertRows(
			esper.InsertValues(esper.Literal("a1"), esper.Literal(10)),
		))
	case "outer-join":
		plan, err = s.env.Build(esper.FromTable(s.env, "MyTable").OnDemand().InsertRows(
			esper.InsertValues(esper.Literal("a"), esper.Literal(10)),
		))
	default:
		return fmt.Errorf("%s: case %q has no faf steps", infraTableJoinID, s.caseName)
	}
	if err != nil {
		return err
	}
	_, err = s.engine.ExecuteFireAndForget(ctx, plan)
	return err
}

func (s *infraTableJoinCaseState) undeployAll(ctx context.Context) error {
	for _, deployments := range s.deployments {
		for _, deployment := range deployments {
			if err := deployment.Undeploy(ctx); err != nil {
				return err
			}
		}
	}
	s.deployments = make(map[string][]*esper.Deployment)
	return nil
}

func decodeInfraTableJoinPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value infraTableJoinBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value infraTableJoinS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value infraTableJoinS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported infra table join event type %q", step.EventType)
	}
}

// infraTableJoinNormalizeResults renders listener batches the way the pinned
// oracle serializes them: null/missing fields become JSON null, table-row
// fragments become positional arrays (Java table rows are Object[]), and
// bean fragments become plain objects.
func infraTableJoinNormalizeResults(results []esper.Result) []compat.ResultRecord {
	if len(results) == 0 {
		return nil
	}
	normalized := make([]compat.ResultRecord, 0, len(results))
	for _, result := range results {
		if event, ok := result.Event(); ok {
			record := compat.ResultRecord{Kind: "row", Fields: make(map[string]any)}
			for _, field := range event.Schema().Fields() {
				record.Fields[field.Name] = infraTableJoinNormalizeValue(event.Get(field.Name))
			}
			normalized = append(normalized, record)
			continue
		}
		if row, ok := result.Row(); ok {
			record := compat.ResultRecord{Kind: "row", Fields: make(map[string]any)}
			for _, field := range row.Schema().Fields() {
				record.Fields[field.Name] = infraTableJoinNormalizeValue(row.Get(field.Name))
			}
			normalized = append(normalized, record)
		}
	}
	return normalized
}

func infraTableJoinNormalizeValue(value esper.Value) any {
	if value.IsMissing() || value.IsNull() {
		return nil
	}
	return infraTableJoinNormalizeUnderlying(value.Any())
}

func infraTableJoinNormalizeUnderlying(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case esper.Event:
		return infraTableJoinNormalizeEvent(typed)
	case []esper.Event:
		items := make([]any, 0, len(typed))
		for _, event := range typed {
			items = append(items, infraTableJoinNormalizeEvent(event))
		}
		return items
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, infraTableJoinNormalizeUnderlying(item))
		}
		return items
	case map[string]any:
		fields := make(map[string]any, len(typed))
		for name, field := range typed {
			fields[name] = infraTableJoinNormalizeUnderlying(field)
		}
		return fields
	default:
		return typed
	}
}

// infraTableJoinNormalizeEvent renders a fragment event like the oracle's
// normalizeValue: a table-row event (map underlying) becomes the positional
// Object[] array, while a bean event becomes the plain object the oracle
// builds for SupportBean_S0/SupportBean_S1.
func infraTableJoinNormalizeEvent(event esper.Event) any {
	underlying := event.Underlying()
	if _, isMap := underlying.(map[string]any); isMap {
		items := make([]any, 0, len(event.Schema().Fields()))
		for _, field := range event.Schema().Fields() {
			items = append(items, infraTableJoinNormalizeValue(event.Get(field.Name)))
		}
		return items
	}
	fields := make(map[string]any)
	for _, field := range event.Schema().Fields() {
		fields[field.Name] = infraTableJoinNormalizeValue(event.Get(field.Name))
	}
	return fields
}

// loadInfraTableJoinScenario decodes the pinned scenario with the strict
// shape checks used by the other runners: duplicate keys and unknown fields
// are rejected, and every case/step must match the frozen contract exactly.
func loadInfraTableJoinScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraTableJoinID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraTableJoinID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableJoinID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableJoinID, err)
	}
	if err := requireInfraTableJoinFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraTableJoinID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraTableJoinID ||
		metadata.Description != infraTableJoinDescription ||
		metadata.JavaCommit != infraTableJoinJavaCommit ||
		metadata.JavaSource != infraTableJoinJavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraTableJoinID)
	}
	if err := validateInfraTableJoinStringArray(root["javaRuntimes"], infraTableJoinJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableJoinStringArray(root["javaNames"], infraTableJoinJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableJoinStringArray(root["javaStaticIds"], infraTableJoinJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableJoinStringArray(root["javaFlags"], infraTableJoinJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraTableJoinCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraTableJoinID, len(infraTableJoinCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraTableJoinFields(object,
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
		if definition.Case != infraTableJoinCases[index] ||
			definition.RuntimeID != infraTableJoinJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraTableJoinJavaExecutions[index] ||
			definition.Observation != infraTableJoinCaseObservations[index] ||
			definition.EPL != infraTableJoinCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraTableJoinID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", infraTableJoinID, err)
	}
	offset := 0
	for _, caseName := range infraTableJoinCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", infraTableJoinID, caseName)
		}
		key, keyErr := infraTableJoinStepKey(rawSteps[offset])
		if keyErr != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraTableJoinID, offset, keyErr)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraTableJoinID, offset, err)
		}
		if key != "case||" || marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", infraTableJoinID, offset, caseName)
		}
		offset++
		want, ok := infraTableJoinCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", infraTableJoinID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", infraTableJoinID, caseName)
		}
		for _, pinned := range want {
			key, err := infraTableJoinStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraTableJoinID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", infraTableJoinID, caseName, offset, key, pinned)
			}
			if err := infraTableJoinCheckStepEPL(caseName, rawSteps[offset]); err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraTableJoinID, offset, err)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", infraTableJoinID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableJoinID, err)
	}
	if len(scenario.Steps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraTableJoinID)
	}
	return scenario, nil
}

// infraTableJoinStepKey renders one raw step as its pinned key:
// op|statement|eventType. Unknown fields on the step object are rejected.
func infraTableJoinStepKey(raw json.RawMessage) (string, error) {
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
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string]map[string]bool{
		"case":         {"op": true, "case": true},
		"deploy":       {"op": true, "statement": true, "epl": true},
		"deployed":     {"op": true, "statement": true},
		"send":         {"op": true, "eventType": true, "payload": true},
		"faf":          {"op": true, "statement": true, "epl": true},
		"undeploy":     {"op": true, "statement": true},
		"undeploy-all": {"op": true},
	}
	fields, known := allowed[step.Op]
	if !known {
		return "", fmt.Errorf("step has unsupported op %q", step.Op)
	}
	for field := range object {
		if !fields[field] {
			return "", fmt.Errorf("step %q has unexpected field %q", step.Op, field)
		}
	}
	return step.Op + "|" + step.Statement + "|" + step.EventType, nil
}

// infraTableJoinCheckStepEPL verifies the byte-exact EPL carried by deploy
// and faf-insert steps against the pinned Java source text.
func infraTableJoinCheckStepEPL(caseName string, raw json.RawMessage) error {
	var step struct {
		Op        string `json:"op"`
		Statement string `json:"statement"`
		Epl       string `json:"epl"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return err
	}
	switch step.Op {
	case "deploy":
		want, ok := infraTableJoinDeployEPLs[caseName][step.Statement]
		if !ok || step.Epl != want {
			return fmt.Errorf("deploy %q EPL is not pinned", step.Statement)
		}
	case "faf":
		want, ok := infraTableJoinFafEPLs[caseName][step.Statement]
		if !ok || step.Epl != want {
			return fmt.Errorf("faf %q EPL is not pinned", step.Statement)
		}
	}
	return nil
}

func requireInfraTableJoinFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraTableJoinID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraTableJoinID, name)
		}
	}
	return nil
}

func validateInfraTableJoinStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}

// infraTableJoinRuntimeID maps each scenario case to the inventory runtime
// ID of the Java execution it replays.
func infraTableJoinRuntimeID(caseName string) string {
	for index, name := range infraTableJoinCases {
		if name == caseName {
			return infraTableJoinJavaRuntimeIDs[index]
		}
	}
	return ""
}
