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

type contextDeclaredExpressionBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const (
	contextDeclaredExpressionID         = "context-declared-expression"
	contextDeclaredExpressionJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	contextDeclaredExpressionSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextWDeclaredExpression.java"
)

const contextDeclaredExpressionDescription = "ContextWDeclaredExpression ords 0/1/2: declared expressions resolving context.label in a category context (function-style call and alias-for forms) and an alias-for expression used as a pattern filter inside both the context-initiation pattern and a statement pattern."

var (
	contextDeclaredExpressionJavaRuntimeIDs = []string{
		"java-runtime-9acc9abebb2846f8b439",
		"java-runtime-77b90f33562c2c0f9548",
		"java-runtime-1bff58f3130b73dfc99f",
	}
	contextDeclaredExpressionJavaExecutions = []string{
		"ContextWDeclaredExpressionSimple",
		"ContextWDeclaredExpressionAlias",
		"ContextWDeclaredExpressionWFilter",
	}
	contextDeclaredExpressionJavaStaticIDs = []string{
		"java-999bc7e77f2d48538dc3",
		"java-33045a26365f5dd7f45b",
		"java-f4bff32df9aeaf56d49c",
	}
	contextDeclaredExpressionJavaFlags = []string{}
	contextDeclaredExpressionCases     = []string{"simple", "alias", "wfilter"}
	contextDeclaredExpressionOrdinals  = []int{0, 1, 2}
	contextDeclaredExpressionSources   = []string{contextDeclaredExpressionSource}
)

var contextDeclaredExpressionCaseObservations = []string{
	"listener; declared expressions resolving context.label deploy before the s0 module whose inline expression uses the function-call form; E1(-2) yields n/xnx/n and E2(1) yields p/xpx/p",
	"listener; declared expressions resolving context.label deploy before the s0 module whose inline expression uses the alias-for form; E1(-2) yields n/xnx/n and E2(1) yields p/xpx/p",
	"listener; alias-for expression THE_EXPRESSION filters both the context-initiation pattern (every SupportBean(theString='x')) and the statement pattern (e1=x -> e2=y); the initiating x event binds e1 in the new partition, yielding c0=1, c1=2 (explicit field selection replaces select * whose tagged-EventBean columns are not trace-comparable)",
}

var contextDeclaredExpressionCaseEPLs = []string{
	"@public create context MyCtx as group by intPrimitive < 0 as n, group by intPrimitive > 0 as p from SupportBean; @public create expression getLabelOne { context.label }; @public create expression getLabelTwo { 'x'||context.label||'x' }; @public @name('s0') expression getLabelThree { context.label } context MyCtx select getLabelOne() as c0, getLabelTwo() as c1, getLabelThree() as c2 from SupportBean",
	"@public create context MyCtx as group by intPrimitive < 0 as n, group by intPrimitive > 0 as p from SupportBean; @public create expression getLabelOne alias for { context.label }; @public create expression getLabelTwo alias for { 'x'||context.label||'x' }; @name('s0') expression getLabelThree alias for { context.label } context MyCtx select getLabelOne as c0, getLabelTwo as c1, getLabelThree as c2 from SupportBean",
	"@public create expression THE_EXPRESSION alias for {theString='x'}; @public create context context2 initiated @now and pattern[every(SupportBean(THE_EXPRESSION))] terminated after 10 minutes; @name('s0') context context2 select e1.intPrimitive as c0, e2.intPrimitive as c1 from pattern[e1=SupportBean(THE_EXPRESSION) -> e2=SupportBean(theString='y')]",
}

// contextDeclaredExpressionCaseSteps pins the complete step sequence per case
// as op|statement|eventType triples so the runtime-ID mapping test can assert
// the scenario file matches the contract.
var contextDeclaredExpressionCaseSteps = map[string][]string{
	"simple": {
		"deploy|ctx|",
		"deployed|ctx|",
		"deploy|expr-1|",
		"deployed|expr-1|",
		"deploy|expr-2|",
		"deployed|expr-2|",
		"deploy|s0|",
		"deployed|s0|",
		"send||SupportBean",
		"send||SupportBean",
		"undeploy-all||",
	},
	"alias": {
		"deploy|ctx|",
		"deployed|ctx|",
		"deploy|expr-1|",
		"deployed|expr-1|",
		"deploy|expr-2|",
		"deployed|expr-2|",
		"deploy|s0|",
		"deployed|s0|",
		"send||SupportBean",
		"send||SupportBean",
		"undeploy-all||",
	},
	"wfilter": {
		"deploy|expr|",
		"deployed|expr|",
		"deploy|ctx|",
		"deployed|ctx|",
		"deploy|s0|",
		"deployed|s0|",
		"send||SupportBean",
		"send||SupportBean",
		"undeploy-all||",
	},
}

// contextDeclaredExpressionCaseState carries the per-case replay state: the
// environment/engine pair, label→deployment bookkeeping, and the deployed
// statement registry used by the deploy handler.
type contextDeclaredExpressionCaseState struct {
	env         *esper.Environment
	engine      *esper.Engine
	deployments map[string]*esper.Deployment
	statements  map[string]*esper.Statement
	caseName    string
}

func runContextDeclaredExpressionScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateContextDeclaredExpressionScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeContextDeclaredExpression(ctx, scenario, &trace)
}

func validateContextDeclaredExpressionScenario(scenario compat.Scenario) error {
	if scenario.Version != compat.ScenarioVersion || scenario.ID != contextDeclaredExpressionID {
		return fmt.Errorf("%s: scenario identity = %q/%q", contextDeclaredExpressionID, scenario.Version, scenario.ID)
	}
	return scenario.Validate()
}

// executeContextDeclaredExpression replays the scenario: each case runs on a
// fresh environment/engine pair (one runtime per Java execution) and every
// step dispatches to the matching runtime action.
func executeContextDeclaredExpression(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *contextDeclaredExpressionCaseState
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
			state, err = startContextDeclaredExpressionCase(step.Case)
			if err != nil {
				return *trace, err
			}
			// Per-statement sequence counters are scoped to the case, matching
			// the Java oracle's per-case sequence map.
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
				Time:      compat.FormatTraceTime(state.engine.Now()),
			})
		case "send":
			event, err := decodeContextDeclaredExpressionPayload(step)
			if err != nil {
				return *trace, err
			}
			if err := state.engine.Send(ctx, step.EventType, event); err != nil {
				return *trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", contextDeclaredExpressionID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

func startContextDeclaredExpressionCase(caseName string) (*contextDeclaredExpressionCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextDeclaredExpressionBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	return &contextDeclaredExpressionCaseState{
		env:         env,
		engine:      esper.NewEngine(env),
		deployments: make(map[string]*esper.Deployment),
		statements:  make(map[string]*esper.Statement),
		caseName:    caseName,
	}, nil
}

// deploy maps each scenario label to the equivalent Go chain-API plan. The
// declared-expression modules become DefineExpression calls; the context and
// statement modules become env registrations and Build+Deploy.
func (s *contextDeclaredExpressionCaseState) deploy(ctx context.Context, step compat.Step,
	trace *compat.Trace, sequences map[string]uint64) error {
	switch s.caseName {
	case "simple", "alias":
		return s.deployCategoryCase(ctx, step, trace, sequences)
	case "wfilter":
		return s.deployWFilterCase(ctx, step, trace, sequences)
	default:
		return fmt.Errorf("%s: unsupported case %q", contextDeclaredExpressionID, s.caseName)
	}
}

func (s *contextDeclaredExpressionCaseState) deployCategoryCase(ctx context.Context, step compat.Step,
	trace *compat.Trace, sequences map[string]uint64) error {
	switch step.Statement {
	case "ctx":
		if _, err := esper.CreateCategoryContext(s.env, "MyCtx",
			esper.Category("n", esper.Less[int](esper.Field[contextDeclaredExpressionBean, int]("intPrimitive"), esper.Literal(0))),
			esper.Category("p", esper.Greater[int](esper.Field[contextDeclaredExpressionBean, int]("intPrimitive"), esper.Literal(0))),
		); err != nil {
			return err
		}
		return nil
	case "expr-1":
		return esper.DefineExpression[string](s.env, "getLabelOne", esper.ContextLabel())
	case "expr-2":
		return esper.DefineExpression[string](s.env, "getLabelTwo",
			esper.Concat(esper.Literal("x"), esper.ContextLabel(), esper.Literal("x")))
	case "s0":
		if err := esper.DefineExpression[string](s.env, "getLabelThree", esper.ContextLabel()); err != nil {
			return err
		}
		plan, err := s.env.Build(esper.Select(
			esper.From[contextDeclaredExpressionBean](s.env, "SupportBean"),
			esper.Alias("c0", esper.ExpressionRef[string](s.env, "getLabelOne")),
			esper.Alias("c1", esper.ExpressionRef[string](s.env, "getLabelTwo")),
			esper.Alias("c2", esper.ExpressionRef[string](s.env, "getLabelThree")),
		).Query(esper.StatementName("s0"), esper.WithContext("MyCtx")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, step.Statement, plan, trace, sequences)
	default:
		return fmt.Errorf("%s: unknown deploy label %q", contextDeclaredExpressionID, step.Statement)
	}
}

func (s *contextDeclaredExpressionCaseState) deployWFilterCase(ctx context.Context, step compat.Step,
	trace *compat.Trace, sequences map[string]uint64) error {
	base := esper.From[contextDeclaredExpressionBean](s.env, "SupportBean")
	switch step.Statement {
	case "expr":
		return esper.DefineExpression[bool](s.env, "THE_EXPRESSION",
			esper.Equal[string](esper.Field[contextDeclaredExpressionBean, string]("theString"), esper.Literal("x")))
	case "ctx":
		// Java `initiated @now and pattern[every(...)]` is overlapping and
		// inclusive: the initiating event is analyzed by the new partition's
		// statements. @now anchors at engine initialization (epoch 0 under
		// the parity runtime's fixed clock).
		// Java's every(SupportBean(THE_EXPRESSION)) carries no tag; Go's
		// pattern events require one, so the initiation leg uses a dummy tag
		// that no statement references.
		start := esper.TimerAt(base, time.Unix(0, 0).UTC()).
			And(esper.PatternFrom(base, "init",
				esper.ExpressionRef[bool](s.env, "THE_EXPRESSION")).Every())
		end := esper.TimerInterval(base, 10*time.Minute)
		_, err := esper.CreateOverlappingPatternInitiatedTerminatedContextInclusive(
			s.env, "context2", start, end)
		return err
	case "s0":
		pattern := esper.PatternFrom(base, "e1",
			esper.ExpressionRef[bool](s.env, "THE_EXPRESSION")).
			FollowedBy("e2", esper.Equal[string](
				esper.Field[contextDeclaredExpressionBean, string]("theString"), esper.Literal("y")))
		plan, err := s.env.Build(pattern.Select(
			esper.Alias("c0", esper.TagField[int]("e1", "intPrimitive")),
			esper.Alias("c1", esper.TagField[int]("e2", "intPrimitive")),
		).Query(esper.StatementName("s0"), esper.WithContext("context2")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, step.Statement, plan, trace, sequences)
	default:
		return fmt.Errorf("%s: unknown deploy label %q", contextDeclaredExpressionID, step.Statement)
	}
}

func (s *contextDeclaredExpressionCaseState) deployPlan(ctx context.Context, label string,
	plan esper.Plan, trace *compat.Trace, sequences map[string]uint64) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[label] = deployment
	for _, statement := range deployment.Statements() {
		s.statements[statement.Name()] = statement
		if statement.Name() == "s0" {
			stmt := statement
			if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				sequences[stmt.Name()+":listener"]++
				record := compat.TraceRecord{
					Case:      s.caseName,
					Operation: "listener",
					Statement: stmt.Name(),
					Sequence:  sequences[stmt.Name()+":listener"],
					Time:      compat.FormatTraceTime(batch.Time),
					New:       compat.NormalizeResults(batch.New),
					Old:       compat.NormalizeResults(batch.Old),
				}
				trace.Records = append(trace.Records, record)
				return nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *contextDeclaredExpressionCaseState) undeployAll(ctx context.Context) error {
	for _, deployment := range s.deployments {
		if err := deployment.Undeploy(ctx); err != nil {
			return err
		}
	}
	s.deployments = make(map[string]*esper.Deployment)
	s.statements = make(map[string]*esper.Statement)
	return nil
}

func decodeContextDeclaredExpressionPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextDeclaredExpressionBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context declared-expression event type %q", step.EventType)
	}
}

// loadContextDeclaredExpressionScenario decodes the pinned scenario with the
// strict shape checks used by the other context runners: duplicate keys and
// unknown fields are rejected, and every case/step must match the frozen
// contract exactly.
func loadContextDeclaredExpressionScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", contextDeclaredExpressionID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", contextDeclaredExpressionID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextDeclaredExpressionID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextDeclaredExpressionID, err)
	}
	if err := requireContextDeclaredExpressionFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", contextDeclaredExpressionID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != contextDeclaredExpressionID ||
		metadata.Description != contextDeclaredExpressionDescription ||
		metadata.JavaCommit != contextDeclaredExpressionJavaCommit ||
		metadata.JavaSource != contextDeclaredExpressionSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", contextDeclaredExpressionID)
	}
	if err := validateContextDeclaredExpressionStringArray(root["javaRuntimes"], contextDeclaredExpressionJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextDeclaredExpressionStringArray(root["javaNames"], contextDeclaredExpressionJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextDeclaredExpressionStringArray(root["javaStaticIds"], contextDeclaredExpressionJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextDeclaredExpressionStringArray(root["javaFlags"], contextDeclaredExpressionJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(contextDeclaredExpressionCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", contextDeclaredExpressionID, len(contextDeclaredExpressionCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireContextDeclaredExpressionFields(object,
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
		if definition.Case != contextDeclaredExpressionCases[index] ||
			definition.Ordinal != contextDeclaredExpressionOrdinals[index] ||
			definition.RuntimeID != contextDeclaredExpressionJavaRuntimeIDs[index] ||
			definition.ExecutionName != contextDeclaredExpressionJavaExecutions[index] ||
			definition.Observation != contextDeclaredExpressionCaseObservations[index] ||
			definition.EPL != contextDeclaredExpressionCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", contextDeclaredExpressionID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", contextDeclaredExpressionID, err)
	}
	offset := 0
	for _, caseName := range contextDeclaredExpressionCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", contextDeclaredExpressionID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextDeclaredExpressionID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", contextDeclaredExpressionID, offset, caseName)
		}
		offset++
		want, ok := contextDeclaredExpressionCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", contextDeclaredExpressionID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", contextDeclaredExpressionID, caseName)
		}
		for _, pinned := range want {
			key, err := contextDeclaredExpressionStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextDeclaredExpressionID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", contextDeclaredExpressionID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", contextDeclaredExpressionID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextDeclaredExpressionID, err)
	}
	if len(scenario.Steps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", contextDeclaredExpressionID)
	}
	return scenario, nil
}

// contextDeclaredExpressionStepKey renders one raw step as its pinned key:
// op|statement|eventType. Unknown fields on the step object are rejected.
func contextDeclaredExpressionStepKey(raw json.RawMessage) (string, error) {
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
	allowed := map[string]bool{
		"op": true, "case": true, "statement": true, "eventType": true,
		"epl": true, "payload": true,
	}
	for field := range object {
		if !allowed[field] {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	return step.Op + "|" + step.Statement + "|" + step.EventType, nil
}

func requireContextDeclaredExpressionFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", contextDeclaredExpressionID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", contextDeclaredExpressionID, name)
		}
	}
	return nil
}

func validateContextDeclaredExpressionStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
