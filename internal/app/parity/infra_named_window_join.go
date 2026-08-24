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

type infraNamedWindowJoinBeanA struct {
	ID string `esper:"id"`
}

// infraNamedWindowJoinSupportBean mirrors the full pinned
// com.espertech.esper.common.internal.support.SupportBean property surface.
// Executions 7-8 project whole window rows (w.* and window(win.*)), so the
// trace carries every property: primitives default, boxed pointers stay nil
// for null, and charPrimitive mirrors the Java char default "\u0000".
type infraNamedWindowJoinSupportBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int      `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int     `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *float64 `esper:"bigDecimal"`
	BigInteger      *int64   `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

type infraNamedWindowJoinS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
	P02 string `esper:"p02"`
	P03 string `esper:"p03"`
}

type infraNamedWindowJoinS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
	P11 string `esper:"p11"`
	P12 string `esper:"p12"`
	P13 string `esper:"p13"`
}

type infraNamedWindowJoinProduct struct {
	Product string `esper:"product"`
	Size    int    `esper:"size"`
}

type infraNamedWindowJoinPortfolio struct {
	Portfolio string `esper:"portfolio"`
	Product   string `esper:"product"`
}

const infraNamedWindowJoinJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var infraNamedWindowJoinJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowJoin.java",
}

var infraNamedWindowJoinJavaRuntimeIDs = []string{
	"java-runtime-e138d2fc24010a22dbb1",
	"java-runtime-26e5704237191a42acf4",
	"java-runtime-38e260a335688bf62391",
	"java-runtime-479eabd476ce403c4ab8",
	"java-runtime-342d46139a3313b382b7",
	"java-runtime-8f4b0394ee32a5524464",
	"java-runtime-e7320476230a3903e2ec",
	"java-runtime-98c12887d1e25c26c101",
	"java-runtime-09ca3e1b6ac4f52b0b7e",
	"java-runtime-2a245ed9721b6b840746",
}

var infraNamedWindowJoinJavaExecutions = []string{
	"InfraJoinIndexChoice",
	"InfraRightOuterJoinLateStart",
	"InfraFullOuterJoinNamedAggregationLateStart",
	"InfraJoinNamedAndStream",
	"InfraJoinBetweenNamed",
	"InfraJoinBetweenSameNamed",
	"InfraJoinSingleInsertOneWindow",
	"InfraUnidirectional",
	"InfraWindowUnidirectionalJoin",
	"InfraInnerJoinLateStart",
}

var infraNamedWindowJoinCases = []string{
	"index-choice",
	"right-outer-late-start",
	"full-outer-named-agg-late-start",
	"named-and-stream",
	"between-named",
	"between-same-named",
	"single-insert-one-window",
	"unidirectional",
	"window-unidirectional-join",
	"inner-join-late-start-objectarray",
	"inner-join-late-start-map",
	"inner-join-late-start-json",
	"inner-join-late-start-jsonprovided",
	"inner-join-late-start-default",
}

// infraNamedWindowJoinRepresentation returns the EventRepresentationChoice
// variant a case replays; empty for the non-representation executions.
func infraNamedWindowJoinRepresentation(caseName string) string {
	switch caseName {
	case "inner-join-late-start-objectarray":
		return "objectarray"
	case "inner-join-late-start-map":
		return "map"
	case "inner-join-late-start-json":
		return "json"
	case "inner-join-late-start-jsonprovided":
		return "jsonprovided"
	case "inner-join-late-start-default":
		return "default"
	default:
		return ""
	}
}

// runInfraNamedWindowJoinScenario replays ten named-window join executions
// (fourteen cases; the representation execution expands to five cases) with
// independently isolated epoch-zero engine state. Index-choice recreates
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
		state.decodePayload,
		state.resolve,
		map[string]compat.StepHandler{
			"deploy":       state.deploy,
			"undeploy":     state.undeploy,
			"undeploy-all": state.undeployAll,
		},
	)
}

// decodePayload decodes scenario payloads for the case's event surface.
// Representation cases deliver Product/Portfolio events in the case's
// representation; the unidirectional cases mirror the full SupportBean
// surface whose charPrimitive defaults to the Java char zero.
func (s *infraNamedWindowJoinReplayState) decodePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		if s.caseName == "unidirectional" || s.caseName == "window-unidirectional-join" {
			var value infraNamedWindowJoinSupportBean
			if err := json.Unmarshal(step.Payload, &value); err != nil {
				return nil, fmt.Errorf("decode SupportBean: %w", err)
			}
			value.CharPrimitive = "\u0000"
			return value, nil
		}
		var value infraNamedWindowJoinBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value infraNamedWindowJoinS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value infraNamedWindowJoinS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	case "Product", "Portfolio":
		rep := infraNamedWindowJoinRepresentation(s.caseName)
		if rep == "objectarray" {
			// The scenario carries positional values in schema declaration
			// order, mirroring the pinned sendEventObjectArray helper.
			var ordered []any
			if err := json.Unmarshal(step.Payload, &ordered); err != nil {
				return nil, fmt.Errorf("decode %s: %w", step.EventType, err)
			}
			for index, value := range ordered {
				if number, ok := value.(float64); ok {
					ordered[index] = int(number)
				}
			}
			return ordered, nil
		}
		var fields map[string]any
		if err := json.Unmarshal(step.Payload, &fields); err != nil {
			return nil, fmt.Errorf("decode %s: %w", step.EventType, err)
		}
		typed := make(map[string]any, len(fields))
		for name, value := range fields {
			if number, ok := value.(float64); ok {
				typed[name] = int(number)
				continue
			}
			typed[name] = value
		}
		return typed, nil
	default:
		return decodeInfraNamedWindowJoinPayload(step)
	}
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
	case "full-outer-named-agg-late-start", "named-and-stream":
		if _, err := esper.RegisterStruct[infraNamedWindowJoinBean](env, "SupportBean"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[infraNamedWindowJoinMarket](env, "SupportMarketDataBean"); err != nil {
			return err
		}
		if caseName == "named-and-stream" {
			if _, err := esper.RegisterStruct[infraNamedWindowJoinBeanA](env, "SupportBean_A"); err != nil {
				return err
			}
		}
	case "between-named", "between-same-named", "single-insert-one-window":
		if _, err := esper.RegisterStruct[infraNamedWindowJoinBean](env, "SupportBean"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[infraNamedWindowJoinMarket](env, "SupportMarketDataBean"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[infraNamedWindowJoinBeanA](env, "SupportBean_A"); err != nil {
			return err
		}
	case "unidirectional", "window-unidirectional-join":
		if _, err := esper.RegisterStruct[infraNamedWindowJoinSupportBean](env, "SupportBean"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[infraNamedWindowJoinBeanA](env, "SupportBean_A"); err != nil {
			return err
		}
		if caseName == "window-unidirectional-join" {
			if _, err := esper.RegisterStruct[infraNamedWindowJoinS0](env, "SupportBean_S0"); err != nil {
				return err
			}
			if _, err := esper.RegisterStruct[infraNamedWindowJoinS1](env, "SupportBean_S1"); err != nil {
				return err
			}
		}
	case "inner-join-late-start-objectarray", "inner-join-late-start-map", "inner-join-late-start-json", "inner-join-late-start-jsonprovided", "inner-join-late-start-default":
		if err := registerInfraNamedWindowJoinRepresentationSchemas(env, infraNamedWindowJoinRepresentation(caseName)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported infra named-window join case %q", caseName)
	}
	return nil
}

// registerInfraNamedWindowJoinRepresentationSchemas registers Product and
// Portfolio under the case's EventRepresentationChoice variant, mirroring the
// pinned suite's representation matrix (AVRO is an approved infrastructure
// difference for this lane).
func registerInfraNamedWindowJoinRepresentationSchemas(env *esper.Environment, rep string) error {
	productFields := []esper.FieldSpec{
		esper.FieldDef("product", reflect.TypeOf("")),
		esper.FieldDef("size", reflect.TypeOf(int(0))),
	}
	portfolioFields := []esper.FieldSpec{
		esper.FieldDef("portfolio", reflect.TypeOf("")),
		esper.FieldDef("product", reflect.TypeOf("")),
	}
	switch rep {
	case "objectarray":
		if _, err := esper.RegisterObjectArray(env, "Product", productFields); err != nil {
			return err
		}
		_, err := esper.RegisterObjectArray(env, "Portfolio", portfolioFields)
		return err
	case "json", "jsonprovided":
		if _, err := esper.RegisterJSON(env, "Product", productFields); err != nil {
			return err
		}
		_, err := esper.RegisterJSON(env, "Portfolio", portfolioFields)
		return err
	default:
		if _, err := esper.RegisterMap(env, "Product", productFields); err != nil {
			return err
		}
		_, err := esper.RegisterMap(env, "Portfolio", portfolioFields)
		return err
	}
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
	if len(plans) == 0 {
		// Compile-only steps (the window(win.*) trio) and representation
		// schema steps validate their Build during planning; the Java oracle
		// deploys them but they emit zero records either way.
		return nil, nil
	}
	deployment, err := s.engine.DeployPlans(s.ctx, plans)
	if err != nil {
		return nil, err
	}
	s.deployments = append(s.deployments, deployment)
	for _, statement := range deployment.Statements() {
		s.statements[statement.Name()] = statement
	}
	attachName := s.attachStatement(step.Statement)
	if attachName != "" {
		statement, ok := deployment.Statement(attachName)
		if !ok {
			return nil, fmt.Errorf("infra named-window join consumer %q statement is missing", attachName)
		}
		if err := attach(statement); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// attachStatement resolves which statement of a deployment carries the
// case's pinned listener. Executions 7-8 compile their consumer inside the
// one-module setup deployment; every other consumer deploys as its own step.
func (s *infraNamedWindowJoinReplayState) attachStatement(deployed string) string {
	consumer := infraNamedWindowJoinConsumerStatement(s.caseName)
	if consumer == "" {
		return ""
	}
	if deployed == consumer || (consumer == "Query2" && deployed == "query2") {
		return consumer
	}
	if deployed == "setup" && (s.caseName == "unidirectional" || s.caseName == "window-unidirectional-join") {
		return consumer
	}
	return ""
}

// infraNamedWindowJoinConsumerStatement names the statement carrying the
// case's pinned listener; empty for listener-less cases (full-outer select).
func infraNamedWindowJoinConsumerStatement(caseName string) string {
	switch caseName {
	case "index-choice", "named-and-stream", "between-named", "between-same-named", "window-unidirectional-join":
		return "s0"
	case "unidirectional", "single-insert-one-window":
		return "select"
	case "inner-join-late-start-objectarray", "inner-join-late-start-map", "inner-join-late-start-json", "inner-join-late-start-jsonprovided", "inner-join-late-start-default":
		// The pinned EPL names the consumer @Name("Query2"); the scenario
		// deploy step key stays lowercase "query2".
		return "Query2"
	default:
		return ""
	}
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
	case "named-and-stream":
		return s.namedAndStreamPlans(statement)
	case "between-named":
		return s.betweenNamedPlans("MyWindowOne", "MyWindowTwo", "a1", "b1", "a2", "b2", "s0", statement)
	case "between-same-named":
		return s.betweenSameNamedPlans(statement)
	case "single-insert-one-window":
		return s.betweenNamedPlans("MyWindowJSIOne", "MyWindowJSITwo", "a1", "b1", "a2", "b2", "select", statement)
	case "unidirectional":
		return s.unidirectionalPlans(statement)
	case "window-unidirectional-join":
		return s.windowUnidirectionalJoinPlans(statement)
	case "inner-join-late-start-objectarray", "inner-join-late-start-map", "inner-join-late-start-json", "inner-join-late-start-jsonprovided", "inner-join-late-start-default":
		return s.innerJoinLateStartPlans(statement)
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

// unidirectionalPlans mirrors InfraUnidirectional: a keepall window over the
// full SupportBean surface fed by insert, joined unidirectionally (window
// side drives) with SupportBean_A#lastevent on id = theString; the consumer
// projects the whole w.* row surface as named columns.
func (s *infraNamedWindowJoinReplayState) unidirectionalPlans(statement string) ([]esper.Plan, error) {
	switch statement {
	case "setup":
		if _, err := s.env.RegisterNamedWindow("MyWindowU", infraNamedWindowJoinSupportBeanSchema("MyWindowU"), esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
		if _, ok := s.engine.NamedWindow("MyWindowU"); !ok {
			return nil, fmt.Errorf("unidirectional window was not materialized")
		}
		createPlan, err := s.env.Build(esper.FromNamedWindow(s.env, "MyWindowU").CreateNamedWindowQuery(esper.StatementName("create-u")))
		if err != nil {
			return nil, err
		}
		insertPlan, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinSupportBean](s.env, "SupportBean")).
			InsertIntoNamedWindow("MyWindowU", esper.CopyMatchingFields()).
			Query(esper.StatementName("insert-u")))
		if err != nil {
			return nil, err
		}
		selectPlan, err := s.unidirectionalSelectPlan()
		if err != nil {
			return nil, err
		}
		// The pinned suite compiles create, insert and the consumer as ONE
		// module; the scenario deploys them as the single setup step.
		return []esper.Plan{createPlan, insertPlan, selectPlan}, nil
	default:
		return nil, fmt.Errorf("unknown unidirectional deployment %q", statement)
	}
}

// unidirectionalSelectPlan builds the w.* consumer: the whole window-row
// surface projected as named columns over the unidirectional window-side
// driver joined to SupportBean_A#lastevent.
func (s *infraNamedWindowJoinReplayState) unidirectionalSelectPlan() (esper.Plan, error) {
	window := esper.FromNamedWindowAs[infraNamedWindowJoinSupportBean](s.env, "MyWindowU")
	lastA := esper.From[infraNamedWindowJoinBeanA](s.env, "SupportBean_A").Window(esper.LastEvent())
	return s.env.Build(esper.Join(window, lastA,
		esper.OnEqual(
			esper.Field[infraNamedWindowJoinSupportBean, string]("theString"),
			esper.Field[infraNamedWindowJoinBeanA, string]("id"),
		),
	).Unidirectional(esper.JoinLeft).Select(
		esper.SelectFrom(0, "theString", esper.JoinField[string](0, "theString")),
		esper.SelectFrom(0, "boolPrimitive", esper.JoinField[bool](0, "boolPrimitive")),
		esper.SelectFrom(0, "intPrimitive", esper.JoinField[int](0, "intPrimitive")),
		esper.SelectFrom(0, "longPrimitive", esper.JoinField[int64](0, "longPrimitive")),
		esper.SelectFrom(0, "charPrimitive", esper.JoinField[string](0, "charPrimitive")),
		esper.SelectFrom(0, "shortPrimitive", esper.JoinField[int16](0, "shortPrimitive")),
		esper.SelectFrom(0, "bytePrimitive", esper.JoinField[int8](0, "bytePrimitive")),
		esper.SelectFrom(0, "floatPrimitive", esper.JoinField[float32](0, "floatPrimitive")),
		esper.SelectFrom(0, "doublePrimitive", esper.JoinField[float64](0, "doublePrimitive")),
		esper.SelectFrom(0, "boolBoxed", esper.JoinField[*bool](0, "boolBoxed")),
		esper.SelectFrom(0, "intBoxed", esper.JoinField[*int](0, "intBoxed")),
		esper.SelectFrom(0, "longBoxed", esper.JoinField[*int64](0, "longBoxed")),
		esper.SelectFrom(0, "charBoxed", esper.JoinField[*string](0, "charBoxed")),
		esper.SelectFrom(0, "shortBoxed", esper.JoinField[*int16](0, "shortBoxed")),
		esper.SelectFrom(0, "byteBoxed", esper.JoinField[*int8](0, "byteBoxed")),
		esper.SelectFrom(0, "floatBoxed", esper.JoinField[*float32](0, "floatBoxed")),
		esper.SelectFrom(0, "doubleBoxed", esper.JoinField[*float64](0, "doubleBoxed")),
		esper.SelectFrom(0, "bigDecimal", esper.JoinField[*float64](0, "bigDecimal")),
		esper.SelectFrom(0, "bigInteger", esper.JoinField[*int64](0, "bigInteger")),
		esper.SelectFrom(0, "enumValue", esper.JoinField[*string](0, "enumValue")),
	).Query(esper.StatementName("select")))
}

// windowUnidirectionalJoinPlans mirrors InfraWindowUnidirectionalJoin: a
// keepall window over the full SupportBean surface with an on-S1 delete, an
// unconditioned unidirectional join whose aggregate projects window(win.*),
// its filtered and toMap enumeration forms, and three compile-only
// window(win.*) statements (plain, join, subquery) that emit no records.
func (s *infraNamedWindowJoinReplayState) windowUnidirectionalJoinPlans(statement string) ([]esper.Plan, error) {
	switch statement {
	case "setup":
		if _, err := s.env.RegisterNamedWindow("MyWindowWUJ", infraNamedWindowJoinSupportBeanSchema("MyWindowWUJ"), esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
		if _, ok := s.engine.NamedWindow("MyWindowWUJ"); !ok {
			return nil, fmt.Errorf("window-unidirectional-join window was not materialized")
		}
		createPlan, err := s.env.Build(esper.FromNamedWindow(s.env, "MyWindowWUJ").CreateNamedWindowQuery(esper.StatementName("create-wuj")))
		if err != nil {
			return nil, err
		}
		insertPlan, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinSupportBean](s.env, "SupportBean")).
			InsertIntoNamedWindow("MyWindowWUJ", esper.CopyMatchingFields()).
			Query(esper.StatementName("insert-wuj")))
		if err != nil {
			return nil, err
		}
		deletePlan, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinS1](s.env, "SupportBean_S1")).
			DeleteFromNamedWindow("MyWindowWUJ",
				esper.Equal[string](esper.Field[infraNamedWindowJoinS1, string]("p10"), esper.NamedWindowField[string]("theString")),
			).Query(esper.StatementName("delete-wuj")))
		if err != nil {
			return nil, err
		}
		s0Plan, err := s.windowUnidirectionalJoinS0Plan()
		if err != nil {
			return nil, err
		}
		// The pinned suite compiles create, insert, delete and the consumer
		// as ONE module; the scenario deploys them as the single setup step.
		return []esper.Plan{createPlan, insertPlan, deletePlan, s0Plan}, nil
	case "compile-a", "compile-b", "compile-c":
		var query esper.Query
		switch statement {
		case "compile-a":
			// select window(win.*) from MyWindowWUJ as win
			query = esper.FromNamedWindow(s.env, "MyWindowWUJ").Aggregate(
				esper.Alias("window", esper.WindowEvents()),
			).Query()
		case "compile-b":
			// select window(win.*) as c0 from SupportBean_S0#lastevent as s0, MyWindowWUJ as win
			query = esper.Join(
				esper.From[infraNamedWindowJoinS0](s.env, "SupportBean_S0").Window(esper.LastEvent()),
				esper.FromNamedWindowAs[infraNamedWindowJoinSupportBean](s.env, "MyWindowWUJ"),
			).Aggregate(
				esper.Alias("c0", esper.WindowValues[esper.Event](esper.JoinEventValue[esper.Event](1))),
			).Query()
		default:
			// select (select window(win.*) from MyWindowWUJ as win) from SupportBean_S0
			query = esper.Select(
				esper.From[infraNamedWindowJoinS0](s.env, "SupportBean_S0"),
				esper.Alias("window", esper.SubqueryValue[[]esper.Event](
					esper.FromNamedWindow(s.env, "MyWindowWUJ"),
					esper.WindowEvents(),
				)),
			).Query()
		}
		if _, err := s.env.Build(query); err != nil {
			return nil, err
		}
		// Compile-only proof: the plan is built but never deployed, so the
		// deployment stack and the trace stay unaffected.
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown window-unidirectional-join deployment %q", statement)
	}
}

// over the unconditioned unidirectional join.
func (s *infraNamedWindowJoinReplayState) windowUnidirectionalJoinS0Plan() (esper.Plan, error) {
	win := esper.WindowValues[esper.Event](esper.JoinEventValue[esper.Event](1))
	return s.env.Build(esper.Join(
		esper.From[infraNamedWindowJoinS0](s.env, "SupportBean_S0"),
		esper.FromNamedWindowAs[infraNamedWindowJoinSupportBean](s.env, "MyWindowWUJ"),
	).Unidirectional(esper.JoinLeft).Aggregate(
		esper.Alias("c0", win),
		esper.Alias("c1", esper.EnumWhere[esper.Event](win, esper.Less[int](
			esper.EnumField[esper.Event, int]("intPrimitive"),
			esper.Literal(2),
		))),
		esper.Alias("c2", esper.EnumToMap[esper.Event, string, int](
			win,
			esper.EnumField[esper.Event, string]("theString"),
			esper.EnumField[esper.Event, int]("intPrimitive"),
		)),
	).Query(esper.StatementName("s0")))
}

// innerJoinLateStartPlans mirrors InfraInnerJoinLateStart per representation:
// Product/Portfolio keepall windows are populated before the unidirectional
// PortfolioWin-driven join deploys late, so pre-existing rows still match.
func (s *infraNamedWindowJoinReplayState) innerJoinLateStartPlans(statement string) ([]esper.Plan, error) {
	switch statement {
	case "schema":
		// Schemas register at case setup; the deploy step mirrors the Java
		// schema deployment and emits no records.
		return nil, nil
	case "window", "portfolio-window":
		windowName := "ProductWin"
		schemaName := "ProductWinSchema"
		if statement == "portfolio-window" {
			windowName = "PortfolioWin"
			schemaName = "PortfolioWinSchema"
		}
		eventType := "Product"
		if statement == "portfolio-window" {
			eventType = "Portfolio"
		}
		schema, ok := s.env.Schema(eventType)
		if !ok {
			return nil, fmt.Errorf("inner-join-late-start event type %q is not registered", eventType)
		}
		if _, err := s.env.RegisterNamedWindow(windowName, schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
		if _, ok := s.engine.NamedWindow(windowName); !ok {
			return nil, fmt.Errorf("inner-join-late-start window %q was not materialized", windowName)
		}
		return s.emptyDeploymentPlan(schemaName)
	case "insert-product", "insert-portfolio":
		eventType := "Product"
		windowName := "ProductWin"
		if statement == "insert-portfolio" {
			eventType = "Portfolio"
			windowName = "PortfolioWin"
		}
		plan, err := s.env.Build(esper.OnRecord(esper.FromAny(s.env, eventType)).
			InsertIntoNamedWindow(windowName, esper.CopyMatchingFields()).
			Query(esper.StatementName(statement)))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	case "query2":
		portfolio := esper.JoinRecordSource(esper.FromNamedWindow(s.env, "PortfolioWin")).Unidirectional()
		product := esper.JoinRecordSource(esper.FromNamedWindow(s.env, "ProductWin"))
		plan, err := s.env.Build(esper.JoinMany(portfolio, product).On(
			esper.OnSourcesEqual(0, esper.JoinField[string](0, "product"), 1, esper.JoinField[string](1, "product")),
		).Select(
			esper.SelectFrom(0, "portfolio", esper.JoinField[string](0, "portfolio")),
			esper.SelectFrom(1, "ProductWin.product", esper.JoinField[string](1, "product")),
			esper.SelectFrom(1, "size", esper.JoinField[int](1, "size")),
		).Query(esper.StatementName("Query2")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	default:
		return nil, fmt.Errorf("unknown inner-join-late-start deployment %q", statement)
	}
}

// infraNamedWindowJoinSupportBeanSchema builds a named struct schema over the
// full SupportBean mirror for whole-row named windows.
func infraNamedWindowJoinSupportBeanSchema(name string) esper.Schema {
	schema, err := esper.StructSchema[infraNamedWindowJoinSupportBean](name)
	if err != nil {
		panic(err)
	}
	return schema
}

// namedAndStreamPlans mirrors InfraJoinNamedAndStream: a keepall window
// projecting (a, b), an on-SupportBean_A delete keyed by id = a, and an
// irstream join of SupportMarketDataBean#length(10) with the window.
func (s *infraNamedWindowJoinReplayState) namedAndStreamPlans(statement string) ([]esper.Plan, error) {
	switch statement {
	case "setup":
		schema, err := esper.NewMapSchema("MyWindowJNS", []esper.FieldSpec{
			esper.FieldDef("a", reflect.TypeOf("")),
			esper.FieldDef("b", reflect.TypeOf(int(0))),
		})
		if err != nil {
			return nil, err
		}
		if _, err := s.env.RegisterNamedWindow("MyWindowJNS", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
		if _, ok := s.engine.NamedWindow("MyWindowJNS"); !ok {
			return nil, fmt.Errorf("named-and-stream window was not materialized")
		}
		createPlan, err := s.env.Build(esper.FromNamedWindow(s.env, "MyWindowJNS").CreateNamedWindowQuery(esper.StatementName("create-jns")))
		if err != nil {
			return nil, err
		}
		insertPlan, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinBean](s.env, "SupportBean")).
			InsertIntoNamedWindow("MyWindowJNS",
				esper.SetColumn("a", esper.Field[infraNamedWindowJoinBean, string]("theString")),
				esper.SetColumn("b", esper.Field[infraNamedWindowJoinBean, int]("intPrimitive")),
			).Query(esper.StatementName("insert-jns")))
		if err != nil {
			return nil, err
		}
		deletePlan, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinBeanA](s.env, "SupportBean_A")).
			DeleteFromNamedWindow("MyWindowJNS",
				esper.Equal[string](esper.NamedWindowField[string]("a"), esper.Field[infraNamedWindowJoinBeanA, string]("id")),
			).Query(esper.StatementName("delete-jns")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{createPlan, insertPlan, deletePlan}, nil
	case "s0":
		market := esper.From[infraNamedWindowJoinMarket](s.env, "SupportMarketDataBean").Window(esper.LengthWindow(10))
		window := esper.FromNamedWindowAs[infraNamedWindowJoinBean](s.env, "MyWindowJNS")
		plan, err := s.env.Build(esper.Join(market, window,
			esper.OnEqual(
				esper.Field[infraNamedWindowJoinMarket, string]("symbol"),
				esper.Field[infraNamedWindowJoinBean, string]("a"),
			),
		).Select(
			esper.SelectLeft("symbol", esper.JoinField[string](0, "symbol")),
			esper.SelectRight("a", esper.JoinField[string](1, "a")),
			esper.SelectRight("b", esper.JoinField[int](1, "b")),
		).Query(
			esper.StatementName("s0"),
			esper.WithOldStream(),
		))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	default:
		return nil, fmt.Errorf("unknown named-and-stream deployment %q", statement)
	}
}

// betweenNamedPlans mirrors InfraJoinBetweenNamed and its single-insert twin:
// two keepall windows with boolPrimitive-routed inserts and volume-keyed
// on-deletes joined on the string key with an irstream consumer.
func (s *infraNamedWindowJoinReplayState) betweenNamedPlans(windowOne, windowTwo, keyOne, countOne, keyTwo, countTwo, consumer, statement string) ([]esper.Plan, error) {
	switch statement {
	case "setup":
		schemaOne, err := esper.NewMapSchema(windowOne, []esper.FieldSpec{
			esper.FieldDef(keyOne, reflect.TypeOf("")),
			esper.FieldDef(countOne, reflect.TypeOf(int(0))),
		})
		if err != nil {
			return nil, err
		}
		if _, err := s.env.RegisterNamedWindow(windowOne, schemaOne, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
		schemaTwo, err := esper.NewMapSchema(windowTwo, []esper.FieldSpec{
			esper.FieldDef(keyTwo, reflect.TypeOf("")),
			esper.FieldDef(countTwo, reflect.TypeOf(int(0))),
		})
		if err != nil {
			return nil, err
		}
		if _, err := s.env.RegisterNamedWindow(windowTwo, schemaTwo, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
		if _, ok := s.engine.NamedWindow(windowOne); !ok {
			return nil, fmt.Errorf("between-named window %q was not materialized", windowOne)
		}
		if _, ok := s.engine.NamedWindow(windowTwo); !ok {
			return nil, fmt.Errorf("between-named window %q was not materialized", windowTwo)
		}
		createOne, err := s.env.Build(esper.FromNamedWindow(s.env, windowOne).CreateNamedWindowQuery(esper.StatementName("create-one")))
		if err != nil {
			return nil, err
		}
		createTwo, err := s.env.Build(esper.FromNamedWindow(s.env, windowTwo).CreateNamedWindowQuery(esper.StatementName("create-two")))
		if err != nil {
			return nil, err
		}
		insertOne, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinBean](s.env, "SupportBean").
			Filter(esper.Equal[bool](esper.Field[infraNamedWindowJoinBean, bool]("boolPrimitive"), esper.Literal(true)))).
			InsertIntoNamedWindow(windowOne,
				esper.SetColumn(keyOne, esper.Field[infraNamedWindowJoinBean, string]("theString")),
				esper.SetColumn(countOne, esper.Field[infraNamedWindowJoinBean, int]("intPrimitive")),
			).Query(esper.StatementName("insert-one")))
		if err != nil {
			return nil, err
		}
		insertTwo, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinBean](s.env, "SupportBean").
			Filter(esper.Equal[bool](esper.Field[infraNamedWindowJoinBean, bool]("boolPrimitive"), esper.Literal(false)))).
			InsertIntoNamedWindow(windowTwo,
				esper.SetColumn(keyTwo, esper.Field[infraNamedWindowJoinBean, string]("theString")),
				esper.SetColumn(countTwo, esper.Field[infraNamedWindowJoinBean, int]("intPrimitive")),
			).Query(esper.StatementName("insert-two")))
		if err != nil {
			return nil, err
		}
		deleteOne, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinMarket](s.env, "SupportMarketDataBean").
			Filter(esper.Equal[int64](esper.Field[infraNamedWindowJoinMarket, int64]("volume"), esper.Literal(int64(1))))).
			DeleteFromNamedWindow(windowOne,
				esper.Equal[string](esper.NamedWindowField[string](keyOne), esper.Field[infraNamedWindowJoinMarket, string]("symbol")),
			).Query(esper.StatementName("delete-one")))
		if err != nil {
			return nil, err
		}
		deleteTwo, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinMarket](s.env, "SupportMarketDataBean").
			Filter(esper.Equal[int64](esper.Field[infraNamedWindowJoinMarket, int64]("volume"), esper.Literal(int64(0))))).
			DeleteFromNamedWindow(windowTwo,
				esper.Equal[string](esper.NamedWindowField[string](keyTwo), esper.Field[infraNamedWindowJoinMarket, string]("symbol")),
			).Query(esper.StatementName("delete-two")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{createOne, createTwo, insertOne, insertTwo, deleteOne, deleteTwo}, nil
	case consumer:
		one := esper.JoinRecordSource(esper.FromNamedWindow(s.env, windowOne))
		two := esper.JoinRecordSource(esper.FromNamedWindow(s.env, windowTwo))
		plan, err := s.env.Build(esper.JoinMany(one, two).On(
			esper.OnSourcesEqual(0, esper.JoinField[string](0, keyOne), 1, esper.JoinField[string](1, keyTwo)),
		).Select(
			esper.SelectFrom(0, keyOne, esper.JoinField[string](0, keyOne)),
			esper.SelectFrom(0, countOne, esper.JoinField[int](0, countOne)),
			esper.SelectFrom(1, keyTwo, esper.JoinField[string](1, keyTwo)),
			esper.SelectFrom(1, countTwo, esper.JoinField[int](1, countTwo)),
		).Query(
			esper.StatementName(consumer),
			esper.WithOldStream(),
		))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	default:
		return nil, fmt.Errorf("unknown between-named deployment %q", statement)
	}
}

// betweenSameNamedPlans mirrors InfraJoinBetweenSameNamed: one keepall window
// self-joined under two aliases with an unfiltered on-delete; a matched
// delete emits exactly one old row despite the two alias positions.
func (s *infraNamedWindowJoinReplayState) betweenSameNamedPlans(statement string) ([]esper.Plan, error) {
	switch statement {
	case "setup":
		schema, err := esper.NewMapSchema("MyWindowJSN", []esper.FieldSpec{
			esper.FieldDef("a", reflect.TypeOf("")),
			esper.FieldDef("b", reflect.TypeOf(int(0))),
		})
		if err != nil {
			return nil, err
		}
		if _, err := s.env.RegisterNamedWindow("MyWindowJSN", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
		if _, ok := s.engine.NamedWindow("MyWindowJSN"); !ok {
			return nil, fmt.Errorf("between-same-named window was not materialized")
		}
		createPlan, err := s.env.Build(esper.FromNamedWindow(s.env, "MyWindowJSN").CreateNamedWindowQuery(esper.StatementName("create-jsn")))
		if err != nil {
			return nil, err
		}
		insertPlan, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinBean](s.env, "SupportBean")).
			InsertIntoNamedWindow("MyWindowJSN",
				esper.SetColumn("a", esper.Field[infraNamedWindowJoinBean, string]("theString")),
				esper.SetColumn("b", esper.Field[infraNamedWindowJoinBean, int]("intPrimitive")),
			).Query(esper.StatementName("insert-jsn")))
		if err != nil {
			return nil, err
		}
		deletePlan, err := s.env.Build(esper.OnEvent(esper.From[infraNamedWindowJoinMarket](s.env, "SupportMarketDataBean")).
			DeleteFromNamedWindow("MyWindowJSN",
				esper.Equal[string](esper.NamedWindowField[string]("a"), esper.Field[infraNamedWindowJoinMarket, string]("symbol")),
			).Query(esper.StatementName("delete-jsn")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{createPlan, insertPlan, deletePlan}, nil
	case "s0":
		left := esper.JoinRecordSource(esper.FromNamedWindow(s.env, "MyWindowJSN"))
		right := esper.JoinRecordSource(esper.FromNamedWindow(s.env, "MyWindowJSN"))
		plan, err := s.env.Build(esper.JoinMany(left, right).On(
			esper.OnSourcesEqual(0, esper.JoinField[string](0, "a"), 1, esper.JoinField[string](1, "a")),
		).Select(
			esper.SelectFrom(0, "a0", esper.JoinField[string](0, "a")),
			esper.SelectFrom(0, "b0", esper.JoinField[int](0, "b")),
			esper.SelectFrom(1, "a1", esper.JoinField[string](1, "a")),
			esper.SelectFrom(1, "b1", esper.JoinField[int](1, "b")),
		).Query(
			esper.StatementName("s0"),
			esper.WithOldStream(),
		))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	default:
		return nil, fmt.Errorf("unknown between-same-named deployment %q", statement)
	}
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
	case "SupportBean_A":
		var value infraNamedWindowJoinBeanA
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
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
	case "named-and-stream":
		return infraNamedWindowJoinJavaRuntimeIDs[3]
	case "between-named":
		return infraNamedWindowJoinJavaRuntimeIDs[4]
	case "between-same-named":
		return infraNamedWindowJoinJavaRuntimeIDs[5]
	case "single-insert-one-window":
		return infraNamedWindowJoinJavaRuntimeIDs[6]
	default:
		return "infra-named-window-join-unknown"
	}
}

// normalizeInfraNamedWindowJoinTrace adapts map-valued columns to the
// InfraNamedWindowJoin oracle's Map rendering: a raw Go map (for example
// window(win.*).toMap results) becomes the row-shaped sorted-key object the
// oracle emits for java.util.Map values. Values that already carry the row
// shape are left untouched, so the adapter is idempotent over both traces.
// The shared compat normalizer stays untouched; 104 checked-in evidences pin
// its plain-object rendering of nested maps.
func normalizeInfraNamedWindowJoinTrace(trace compat.Trace) compat.Trace {
	for index := range trace.Records {
		record := &trace.Records[index]
		normalizeInfraNamedWindowJoinRows(record.New)
		normalizeInfraNamedWindowJoinRows(record.Old)
	}
	return trace
}

func normalizeInfraNamedWindowJoinRows(rows []compat.ResultRecord) {
	for _, row := range rows {
		normalizeInfraNamedWindowJoinFields(row.Fields)
	}
}

func normalizeInfraNamedWindowJoinFields(fields map[string]any) {
	for name, value := range fields {
		normalized, changed := normalizeInfraNamedWindowJoinValue(value)
		if changed {
			fields[name] = normalized
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			if kind, hasKind := typed["kind"]; hasKind && kind == "row" {
				continue
			}
			normalizeInfraNamedWindowJoinFields(typed)
		case []any:
			for _, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					if kind, hasKind := nested["kind"]; hasKind && kind == "row" {
						// Producer-emitted rows are opaque; descending into
						// them would re-envelope their fields member.
						continue
					}
					normalizeInfraNamedWindowJoinFields(nested)
				}
			}
		}
	}
}

func normalizeInfraNamedWindowJoinValue(value any) (any, bool) {
	if sentinel, ok := value.(map[string]any); ok {
		if _, hasState := sentinel["state"]; hasState && len(sentinel) == 1 {
			// Null/missing sentinels are protocol atoms, never maps to wrap.
			return nil, false
		}
	}
	nested, ok := value.(map[string]any)
	if !ok {
		if typed, isTyped := value.(map[string]int); isTyped {
			converted := make(map[string]any, len(typed))
			for key, item := range typed {
				converted[key] = item
			}
			nested, ok = converted, true
		}
	}
	if !ok {
		return nil, false
	}
	if kind, hasKind := nested["kind"]; hasKind && kind == "row" {
		return nil, false
	}
	return map[string]any{"kind": "row", "fields": nested}, true
}
