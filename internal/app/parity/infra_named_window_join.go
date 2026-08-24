package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type infraNamedWindowJoinSimpleOne struct {
	S1 string  `esper:"s1"`
	I1 int     `esper:"i1"`
	D1 float64 `esper:"d1"`
	L1 int64   `esper:"l1"`
}

type infraNamedWindowJoinSimpleTwo struct {
	S2 string  `esper:"s2"`
	I2 int     `esper:"i2"`
	D2 float64 `esper:"d2"`
	L2 int64   `esper:"l2"`
}

type infraNamedWindowJoinQueueLeave struct {
	ID        int    `esper:"id"`
	Location  string `esper:"location"`
	TimeLeave int64  `esper:"timeLeave"`
}

type infraNamedWindowJoinQueueEnter struct {
	ID        int    `esper:"id"`
	Location  string `esper:"location"`
	SKU       string `esper:"sku"`
	TimeEnter int64  `esper:"timeEnter"`
}

type infraNamedWindowJoinBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
}

type infraNamedWindowJoinMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   string  `esper:"feed"`
}

const infraNamedWindowJoinJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var infraNamedWindowJoinJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowJoin.java",
}

var infraNamedWindowJoinJavaRuntimeIDs = []string{
	"java-runtime-e138d2fc24010a22dbb1",
	"java-runtime-26e5704237191a42acf4",
	"java-runtime-38e260a335688bf62391",
}

var infraNamedWindowJoinJavaExecutions = []string{
	"InfraJoinIndexChoice",
	"InfraRightOuterJoinLateStart",
	"InfraFullOuterJoinNamedAggregationLateStart",
}

var infraNamedWindowJoinCases = []string{
	"index-choice",
	"right-outer-late-start",
	"full-outer-named-agg-late-start",
}

// runInfraNamedWindowJoinScenario replays three named-window join executions
// with independently isolated epoch-zero engine state. Index-choice recreates
// its same-named window between five combinations, so each combination uses a
// fresh Environment as the typed catalog deliberately has no unregister API.
func runInfraNamedWindowJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range infraNamedWindowJoinCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runInfraNamedWindowJoinCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("infra named-window join case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("infra named-window join scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runInfraNamedWindowJoinCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	if caseName != "index-choice" {
		return runInfraNamedWindowJoinReplayCase(ctx, scenario, caseName, 0)
	}
	segments, err := splitInfraNamedWindowJoinIndexChoiceScenario(scenario)
	if err != nil {
		return compat.Trace{}, err
	}
	if len(segments) != len(infraNamedWindowJoinIndexChoices) {
		return compat.Trace{}, fmt.Errorf("index-choice replay has %d combinations, want %d", len(segments), len(infraNamedWindowJoinIndexChoices))
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	var listenerSequence uint64
	for combination, segment := range segments {
		segmentTrace, err := runInfraNamedWindowJoinReplayCase(ctx, segment, caseName, combination)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("index-choice combination %d: %w", combination, err)
		}
		for index := range segmentTrace.Records {
			record := &segmentTrace.Records[index]
			if record.Operation != "listener" {
				continue
			}
			record.Sequence += listenerSequence
		}
		for _, record := range segmentTrace.Records {
			if record.Operation == "listener" && record.Sequence > listenerSequence {
				listenerSequence = record.Sequence
			}
		}
		trace.Records = append(trace.Records, segmentTrace.Records...)
	}
	return trace, nil
}

func splitInfraNamedWindowJoinIndexChoiceScenario(scenario compat.Scenario) ([]compat.Scenario, error) {
	result := make([]compat.Scenario, 0, len(infraNamedWindowJoinIndexChoices))
	steps := make([]compat.Step, 0)
	for _, step := range scenario.Steps {
		steps = append(steps, step)
		if step.Op != "undeploy-all" {
			continue
		}
		segment := compat.Scenario{Version: scenario.Version, ID: scenario.ID, Steps: steps}
		if err := segment.Validate(); err != nil {
			return nil, err
		}
		result = append(result, segment)
		steps = []compat.Step{{Op: "case", Case: "index-choice"}}
	}
	if len(steps) != 1 {
		return nil, fmt.Errorf("index-choice scenario ends before undeploy-all")
	}
	return result, nil
}

func runInfraNamedWindowJoinReplayCase(ctx context.Context, scenario compat.Scenario, caseName string, combo int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if err := registerInfraNamedWindowJoinAnchor(env); err != nil {
		return compat.Trace{}, err
	}
	if err := registerInfraNamedWindowJoinCase(env, caseName); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(infraNamedWindowJoinRuntimeID(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := infraNamedWindowJoinReplayState{
		ctx:        ctx,
		env:        env,
		engine:     engine,
		caseName:   caseName,
		statements: make(map[string]*esper.Statement),
		combo:      combo,
	}
	anchor, err := state.deployAnchor()
	if err != nil {
		return compat.Trace{}, err
	}
	return compat.ReplayWithStatementsAndHandlers(ctx, engine, anchor, scenario,
		decodeInfraNamedWindowJoinPayload,
		state.resolve,
		map[string]compat.StepHandler{
			"deploy":       state.deploy,
			"undeploy":     state.undeploy,
			"undeploy-all": state.undeployAll,
		},
	)
}

func registerInfraNamedWindowJoinAnchor(env *esper.Environment) error {
	_, err := esper.RegisterMap(env, infraNamedWindowJoinAnchorEvent, []esper.FieldSpec{
		esper.FieldDef("unused", reflect.TypeOf("")),
	})
	return err
}

func registerInfraNamedWindowJoinCase(env *esper.Environment, caseName string) error {
	switch caseName {
	case "index-choice":
		if _, err := esper.RegisterStruct[infraNamedWindowJoinSimpleOne](env, "SupportSimpleBeanOne"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[infraNamedWindowJoinSimpleTwo](env, "SupportSimpleBeanTwo"); err != nil {
			return err
		}
	case "right-outer-late-start":
		if _, err := esper.RegisterStruct[infraNamedWindowJoinQueueLeave](env, "SupportQueueLeave"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[infraNamedWindowJoinQueueEnter](env, "SupportQueueEnter"); err != nil {
			return err
		}
	case "full-outer-named-agg-late-start":
		if _, err := esper.RegisterStruct[infraNamedWindowJoinBean](env, "SupportBean"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[infraNamedWindowJoinMarket](env, "SupportMarketDataBean"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported infra named-window join case %q", caseName)
	}
	return nil
}

type infraNamedWindowJoinReplayState struct {
	ctx         context.Context
	env         *esper.Environment
	engine      *esper.Engine
	caseName    string
	statements  map[string]*esper.Statement
	deployments []*esper.Deployment
	combo       int
	index       int
	where       int
}

func (s *infraNamedWindowJoinReplayState) deployAnchor() (*esper.Statement, error) {
	plan, err := s.env.Build(esper.FromAny(s.env, infraNamedWindowJoinAnchorEvent).Query(esper.StatementName(infraNamedWindowJoinAnchorStatement)))
	if err != nil {
		return nil, err
	}
	deployment, err := s.engine.Deploy(s.ctx, plan)
	if err != nil {
		return nil, err
	}
	statement, ok := deployment.Statement(infraNamedWindowJoinAnchorStatement)
	if !ok {
		return nil, fmt.Errorf("infra named-window join anchor statement is missing")
	}
	s.statements[statement.Name()] = statement
	return statement, nil
}

const (
	infraNamedWindowJoinAnchorEvent     = "infra-named-window-join-anchor"
	infraNamedWindowJoinAnchorStatement = "infra-named-window-join-anchor"
)

func (s *infraNamedWindowJoinReplayState) deploy(step compat.Step, attach func(*esper.Statement) error) ([]compat.TraceRecord, error) {
	plans, err := s.plansFor(step.Statement)
	if err != nil {
		return nil, err
	}
	deployment, err := s.engine.DeployPlans(s.ctx, plans)
	if err != nil {
		return nil, err
	}
	s.deployments = append(s.deployments, deployment)
	for _, statement := range deployment.Statements() {
		s.statements[statement.Name()] = statement
	}
	if step.Statement == "s0" {
		statement, ok := deployment.Statement("s0")
		if !ok {
			return nil, fmt.Errorf("index-choice s0 statement is missing")
		}
		if err := attach(statement); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (s *infraNamedWindowJoinReplayState) undeploy(_ compat.Step, _ func(*esper.Statement) error) ([]compat.TraceRecord, error) {
	if len(s.deployments) == 0 {
		return nil, fmt.Errorf("infra named-window join undeploy without a deployment")
	}
	index := len(s.deployments) - 1
	deployment := s.deployments[index]
	if err := deployment.Undeploy(s.ctx); err != nil {
		return nil, err
	}
	for _, statement := range deployment.Statements() {
		delete(s.statements, statement.Name())
	}
	s.deployments = s.deployments[:index]
	return nil, nil
}

func (s *infraNamedWindowJoinReplayState) undeployAll(_ compat.Step, _ func(*esper.Statement) error) ([]compat.TraceRecord, error) {
	for index := len(s.deployments) - 1; index >= 0; index-- {
		deployment := s.deployments[index]
		if err := deployment.Undeploy(s.ctx); err != nil {
			return nil, err
		}
		for _, statement := range deployment.Statements() {
			delete(s.statements, statement.Name())
		}
	}
	s.deployments = nil
	return nil, nil
}

func (s *infraNamedWindowJoinReplayState) resolve(name string) (*esper.Statement, error) {
	statement, ok := s.statements[name]
	if !ok {
		return nil, fmt.Errorf("unknown infra named-window join statement %q", name)
	}
	return statement, nil
}

func (s *infraNamedWindowJoinReplayState) plansFor(statement string) ([]esper.Plan, error) {
	switch s.caseName {
	case "index-choice":
		return s.indexChoicePlans(statement)
	case "right-outer-late-start":
		return s.rightOuterPlans(statement)
	case "full-outer-named-agg-late-start":
		return s.fullOuterPlans(statement)
	default:
		return nil, fmt.Errorf("unsupported infra named-window join case %q", s.caseName)
	}
}

func (s *infraNamedWindowJoinReplayState) indexChoicePlans(statement string) ([]esper.Plan, error) {
	switch statement {
	case "window":
		if s.combo >= len(infraNamedWindowJoinIndexChoices) {
			return nil, fmt.Errorf("index-choice has more window deployments than combinations")
		}
		choice := infraNamedWindowJoinIndexChoices[s.combo]
		options := []esper.NamedWindowOption{esper.NamedWindowRetention(choice.retention)}
		options = append(options, choice.indexes...)
		if _, err := s.env.RegisterNamedWindow("MyWindow", infraNamedWindowJoinSimpleOneSchema(), options...); err != nil {
			return nil, err
		}
		if _, ok := s.engine.NamedWindow("MyWindow"); !ok {
			return nil, fmt.Errorf("index-choice named window was not materialized")
		}
		s.index = 0
		s.where = 0
		s.combo++
		return s.emptyDeploymentPlan("window")
	case "insert":
		if s.combo == 0 {
			return nil, fmt.Errorf("index-choice insert deployed before its window")
		}
		plan, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinSimpleOne](s.env, "SupportSimpleBeanOne")).
			InsertIntoNamedWindow("MyWindow", esper.CopyMatchingFields()).
			Query(esper.StatementName("insert")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	case "index":
		choice, err := s.indexChoice()
		if err != nil {
			return nil, err
		}
		if s.index >= len(choice.indexes) {
			return nil, fmt.Errorf("index-choice combination %d has too many index deployments", s.combo-1)
		}
		s.index++
		return s.emptyDeploymentPlan(fmt.Sprintf("index-%d", s.index))
	case "s0":
		choice, err := s.indexChoice()
		if err != nil {
			return nil, err
		}
		if s.where >= len(choice.predicates) {
			return nil, fmt.Errorf("index-choice combination %d has too many s0 deployments", s.combo-1)
		}
		predicate := choice.predicates[s.where]
		s.where++
		left := esper.From[infraNamedWindowJoinSimpleTwo](s.env, "SupportSimpleBeanTwo")
		right := esper.FromNamedWindowAs[infraNamedWindowJoinSimpleOne](s.env, "MyWindow")
		plan, err := s.env.Build(esper.Join(left, right, predicate).
			Unidirectional(esper.JoinLeft).
			Select(
				esper.SelectLeft("s2", esper.JoinField[string](0, "s2")),
				esper.SelectRight("s1", esper.JoinField[string](1, "s1")),
				esper.SelectRight("i1", esper.JoinField[int](1, "i1")),
			).
			Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	default:
		return nil, fmt.Errorf("unknown index-choice deployment %q", statement)
	}
}

type infraNamedWindowJoinIndexChoice struct {
	retention  esper.WindowSpec
	indexes    []esper.NamedWindowOption
	predicates []esper.JoinCondition
}

var infraNamedWindowJoinIndexChoices = []infraNamedWindowJoinIndexChoice{
	{
		retention: esper.Unique(esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
		predicates: []esper.JoinCondition{
			esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
			esper.AllJoin(
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, int64]("l2"), esper.Field[infraNamedWindowJoinSimpleOne, int64]("l1")),
			),
		},
	},
	{
		retention: esper.Unique(esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
		indexes:   []esper.NamedWindowOption{esper.NamedWindowUniqueIndex("One", "s1")},
		predicates: []esper.JoinCondition{
			esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
			esper.AllJoin(
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, int64]("l2"), esper.Field[infraNamedWindowJoinSimpleOne, int64]("l1")),
			),
		},
	},
	{
		retention: esper.Unique(esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
		indexes:   []esper.NamedWindowOption{esper.NamedWindowUniqueIndex("One", "s1", "l1")},
		predicates: []esper.JoinCondition{
			esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
			esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, float64]("d2"), esper.Field[infraNamedWindowJoinSimpleOne, float64]("d1")),
			esper.AllJoin(
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, int64]("l2"), esper.Field[infraNamedWindowJoinSimpleOne, int64]("l1")),
			),
		},
	},
	{
		retention: esper.Unique(esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
		indexes: []esper.NamedWindowOption{
			esper.NamedWindowIndex("One", "s1"),
			esper.NamedWindowUniqueIndex("Two", "s1", "d1"),
		},
		predicates: []esper.JoinCondition{
			esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, float64]("d2"), esper.Field[infraNamedWindowJoinSimpleOne, float64]("d1")),
			esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
			esper.AllJoin(
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, int64]("l2"), esper.Field[infraNamedWindowJoinSimpleOne, int64]("l1")),
			),
			esper.AllJoin(
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, float64]("d2"), esper.Field[infraNamedWindowJoinSimpleOne, float64]("d1")),
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, int64]("l2"), esper.Field[infraNamedWindowJoinSimpleOne, int64]("l1")),
			),
		},
	},
	{
		retention: esper.KeepAll(),
		indexes: []esper.NamedWindowOption{
			esper.NamedWindowIndex("One", "s1"),
			esper.NamedWindowUniqueIndex("Two", "s1", "d1"),
		},
		predicates: []esper.JoinCondition{
			esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, float64]("d2"), esper.Field[infraNamedWindowJoinSimpleOne, float64]("d1")),
			esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
			esper.AllJoin(
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, int64]("l2"), esper.Field[infraNamedWindowJoinSimpleOne, int64]("l1")),
			),
			esper.AllJoin(
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, float64]("d2"), esper.Field[infraNamedWindowJoinSimpleOne, float64]("d1")),
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, int64]("l2"), esper.Field[infraNamedWindowJoinSimpleOne, int64]("l1")),
			),
			esper.AllJoin(
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, float64]("d2"), esper.Field[infraNamedWindowJoinSimpleOne, float64]("d1")),
				esper.OnEqual(esper.Field[infraNamedWindowJoinSimpleTwo, string]("s2"), esper.Field[infraNamedWindowJoinSimpleOne, string]("s1")),
			),
		},
	},
}

func (s *infraNamedWindowJoinReplayState) indexChoice() (infraNamedWindowJoinIndexChoice, error) {
	if s.combo == 0 || s.combo > len(infraNamedWindowJoinIndexChoices) {
		return infraNamedWindowJoinIndexChoice{}, fmt.Errorf("index-choice deployment outside a combination")
	}
	return infraNamedWindowJoinIndexChoices[s.combo-1], nil
}

func infraNamedWindowJoinSimpleOneSchema() esper.Schema {
	schema, err := esper.StructSchema[infraNamedWindowJoinSimpleOne]("MyWindow")
	if err != nil {
		panic(err)
	}
	return schema
}

func (s *infraNamedWindowJoinReplayState) rightOuterPlans(statement string) ([]esper.Plan, error) {
	switch statement {
	case "window-leave":
		schema, err := esper.NewMapSchema("WindowLeave", []esper.FieldSpec{
			esper.FieldDef("timeLeave", reflect.TypeOf(int64(0))),
			esper.FieldDef("id", reflect.TypeOf(int(0))),
			esper.FieldDef("location", reflect.TypeOf("")),
		})
		if err != nil {
			return nil, err
		}
		if _, err := s.env.RegisterNamedWindow("WindowLeave", schema, esper.NamedWindowRetention(esper.TimeWindow(6*time.Second))); err != nil {
			return nil, err
		}
		if _, ok := s.engine.NamedWindow("WindowLeave"); !ok {
			return nil, fmt.Errorf("right-outer leave window was not materialized")
		}
		plan, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinQueueLeave](s.env, "SupportQueueLeave")).
			InsertIntoNamedWindow("WindowLeave",
				esper.SetColumn("timeLeave", esper.Field[infraNamedWindowJoinQueueLeave, int64]("timeLeave")),
				esper.SetColumn("id", esper.Field[infraNamedWindowJoinQueueLeave, int]("id")),
				esper.SetColumn("location", esper.Field[infraNamedWindowJoinQueueLeave, string]("location")),
			).Query(esper.StatementName("insert-leave")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	case "window-enter":
		schema, err := esper.NewMapSchema("WindowEnter", []esper.FieldSpec{
			esper.FieldDef("location", reflect.TypeOf("")),
			esper.FieldDef("sku", reflect.TypeOf("")),
			esper.FieldDef("timeEnter", reflect.TypeOf(int64(0))),
			esper.FieldDef("id", reflect.TypeOf(int(0))),
		})
		if err != nil {
			return nil, err
		}
		if _, err := s.env.RegisterNamedWindow("WindowEnter", schema, esper.NamedWindowRetention(esper.TimeWindow(6*time.Second))); err != nil {
			return nil, err
		}
		if _, ok := s.engine.NamedWindow("WindowEnter"); !ok {
			return nil, fmt.Errorf("right-outer enter window was not materialized")
		}
		plan, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinQueueEnter](s.env, "SupportQueueEnter")).
			InsertIntoNamedWindow("WindowEnter",
				esper.SetColumn("location", esper.Field[infraNamedWindowJoinQueueEnter, string]("location")),
				esper.SetColumn("sku", esper.Field[infraNamedWindowJoinQueueEnter, string]("sku")),
				esper.SetColumn("timeEnter", esper.Field[infraNamedWindowJoinQueueEnter, int64]("timeEnter")),
				esper.SetColumn("id", esper.Field[infraNamedWindowJoinQueueEnter, int]("id")),
			).Query(esper.StatementName("insert-enter")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	case "s1":
		return s.rightOuterAggregatePlan("s1", true)
	case "s2":
		return s.rightOuterAggregatePlan("s2", false)
	default:
		return nil, fmt.Errorf("unknown right-outer deployment %q", statement)
	}
}

func (s *infraNamedWindowJoinReplayState) rightOuterAggregatePlan(name string, rightOuter bool) ([]esper.Plan, error) {
	leave := esper.FromNamedWindow(s.env, "WindowLeave")
	enter := esper.FromNamedWindow(s.env, "WindowEnter")
	conditions := []esper.JoinCondition{
		esper.OnSourcesEqual(0, esper.JoinField[int](0, "id"), 1, esper.JoinField[int](1, "id")),
		esper.OnSourcesEqual(0, esper.JoinField[string](0, "location"), 1, esper.JoinField[string](1, "location")),
	}
	join := esper.JoinMany(esper.JoinRecordSource(leave), esper.JoinRecordSource(enter)).On(conditions...)
	if rightOuter {
		join = join.RightOuter()
	} else {
		join = esper.JoinMany(esper.JoinRecordSource(enter), esper.JoinRecordSource(leave)).On(
			esper.OnSourcesEqual(0, esper.JoinField[int](0, "id"), 1, esper.JoinField[int](1, "id")),
			esper.OnSourcesEqual(0, esper.JoinField[string](0, "location"), 1, esper.JoinField[string](1, "location")),
		).LeftOuter()
	}
	enterSource := 1
	leaveSource := 0
	if !rightOuter {
		enterSource = 0
		leaveSource = 1
	}
	location := esper.JoinField[string](enterSource, "location")
	sku := esper.JoinField[string](enterSource, "sku")
	timeEnter := esper.JoinField[int64](enterSource, "timeEnter")
	timeLeave := esper.JoinField[int64](leaveSource, "timeLeave")
	latency := esper.Subtract[int64](esper.Coalesce[int64](timeLeave, esper.Literal(int64(250))), timeEnter)
	countEnter := esper.Count[int64](timeEnter)
	countLeave := esper.Count[int64](timeLeave)
	plan, err := s.env.Build(join.GroupBy(location, sku).Select(
		esper.Alias("loc", location),
		esper.Alias("sku", sku),
		esper.Alias("avgTime", esper.Avg[int64](latency)),
		esper.Alias("cntEnter", countEnter),
		esper.Alias("cntLeave", countLeave),
		esper.Alias("diff", esper.Subtract[int64](countEnter, countLeave)),
	).Query(
		esper.StatementName(name),
		esper.WithOutput(esper.OutputEveryTime(time.Second)),
		esper.OrderBy(esper.Ascending(esper.ResultField[string]("loc")), esper.Ascending(esper.ResultField[string]("sku"))),
	))
	if err != nil {
		return nil, err
	}
	return []esper.Plan{plan}, nil
}

func (s *infraNamedWindowJoinReplayState) fullOuterPlans(statement string) ([]esper.Plan, error) {
	switch statement {
	case "create":
		schema, err := esper.NewMapSchema("MyWindowFO", []esper.FieldSpec{
			esper.FieldDef("theString", reflect.TypeOf("")),
			esper.FieldDef("intPrimitive", reflect.TypeOf(int(0))),
			esper.FieldDef("boolPrimitive", reflect.TypeOf(true)),
		})
		if err != nil {
			return nil, err
		}
		if _, err := s.env.RegisterNamedWindow("MyWindowFO", schema,
			esper.NamedWindowRetention(esper.GroupWindowKeys([]esper.Expr{
				esper.Field[infraNamedWindowJoinBean, string]("theString"),
				esper.Field[infraNamedWindowJoinBean, int]("intPrimitive"),
			}, esper.LengthWindow(3)))); err != nil {
			return nil, err
		}
		if _, ok := s.engine.NamedWindow("MyWindowFO"); !ok {
			return nil, fmt.Errorf("full-outer named window was not materialized")
		}
		createPlan, err := s.env.Build(esper.FromNamedWindow(s.env, "MyWindowFO").CreateNamedWindowQuery(esper.StatementName("create")))
		if err != nil {
			return nil, err
		}
		insertPlan, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinBean](s.env, "SupportBean")).
			InsertIntoNamedWindow("MyWindowFO", esper.CopyMatchingFields()).
			Query(esper.StatementName("insert")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{createPlan, insertPlan}, nil
	case "select":
		window := esper.FromNamedWindowAs[infraNamedWindowJoinBean](s.env, "MyWindowFO")
		market := esper.From[infraNamedWindowJoinMarket](s.env, "SupportMarketDataBean").Window(esper.KeepAll())
		plan, err := s.env.Build(esper.Join(window, market,
			esper.OnEqual(
				esper.Field[infraNamedWindowJoinBean, string]("theString"),
				esper.Field[infraNamedWindowJoinMarket, string]("symbol"),
			),
		).FullOuter().GroupBy(
			esper.JoinField[string](0, "theString"),
			esper.JoinField[int](0, "intPrimitive"),
			esper.JoinField[string](1, "symbol"),
		).Select(
			esper.Alias("theString", esper.JoinField[string](0, "theString")),
			esper.Alias("intPrimitive", esper.JoinField[int](0, "intPrimitive")),
			esper.Alias("cntBool", esper.Count[bool](esper.JoinField[bool](0, "boolPrimitive"))),
			esper.Alias("symbol", esper.JoinField[string](1, "symbol")),
		).Query(
			esper.StatementName("select"),
			esper.OrderBy(
				esper.Ascending(esper.ResultField[string]("theString")),
				esper.Ascending(esper.ResultField[int]("intPrimitive")),
				esper.Ascending(esper.ResultField[string]("symbol")),
			),
		))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	default:
		return nil, fmt.Errorf("unknown full-outer deployment %q", statement)
	}
}

func (s *infraNamedWindowJoinReplayState) emptyDeploymentPlan(name string) ([]esper.Plan, error) {
	plan, err := s.env.Build(esper.FromAny(s.env, infraNamedWindowJoinAnchorEvent).Query(esper.StatementName(name)))
	if err != nil {
		return nil, err
	}
	return []esper.Plan{plan}, nil
}

func decodeInfraNamedWindowJoinPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportSimpleBeanOne":
		var value infraNamedWindowJoinSimpleOne
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportSimpleBeanOne: %w", err)
		}
		return value, nil
	case "SupportSimpleBeanTwo":
		var value infraNamedWindowJoinSimpleTwo
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportSimpleBeanTwo: %w", err)
		}
		return value, nil
	case "SupportQueueLeave":
		var value infraNamedWindowJoinQueueLeave
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportQueueLeave: %w", err)
		}
		return value, nil
	case "SupportQueueEnter":
		var value infraNamedWindowJoinQueueEnter
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportQueueEnter: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value infraNamedWindowJoinBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportMarketDataBean":
		var value infraNamedWindowJoinMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported infra named-window join event type %q", step.EventType)
	}
}

func infraNamedWindowJoinRuntimeID(caseName string) string {
	switch caseName {
	case "index-choice":
		return infraNamedWindowJoinJavaRuntimeIDs[0]
	case "right-outer-late-start":
		return infraNamedWindowJoinJavaRuntimeIDs[1]
	case "full-outer-named-agg-late-start":
		return infraNamedWindowJoinJavaRuntimeIDs[2]
	default:
		return "infra-named-window-join-unknown"
	}
}
