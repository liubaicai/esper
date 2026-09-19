package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// context_key_segmented_subselect_prev_prior.go replays
// ContextKeySegmentedSubselectPrevPrior (ord 8) against the pinned Java
// oracle: a single-type segmented context partitions SupportBean by
// theString while statement s0 selects theString plus a scalar subquery
// over the unlisted SupportBean_S0#keepall. S0 events are not in the
// partition spec, so they broadcast into every existing partition's
// subquery window without allocating partitions; new partitions start
// empty and a multi-row subquery yields null. undeployModuleContaining
// plus a prior(0, id) redeploy resets partition state and replays the
// identical send sequence.
//
// Approved differences (observably identical to the Java EPL):
//   - `create context` maps to the env-level CreateKeyContextByStreams
//     registration; the single-stream form records streamKeys so the
//     unlisted-type statement validation stays enforceable.
//   - The scalar subselect maps to SubqueryValueWithOptions with
//     Prev(0, id)/Prior(0, id) over the KeepAll AsRecord source and
//     SubqueryNullOnMultiple cardinality.

const contextKeySegmentedSubselectPrevPriorID = "context-key-segmented-subselect-prev-prior"
const contextKeySegmentedSubselectPrevPriorJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedSubselectPrevPriorJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedSubselectPrevPriorJavaRuntimeIDs = []string{
	"java-runtime-2afbd86618b752a6dd5b",
}

var contextKeySegmentedSubselectPrevPriorJavaExecutions = []string{
	"ContextKeySegmentedSubselectPrevPrior",
}

var contextKeySegmentedSubselectPrevPriorCases = []string{
	"subselect-prev-prior",
}

var contextKeySegmentedSubselectPrevPriorCaseRuntimeIDs = map[string]string{
	"subselect-prev-prior": "java-runtime-2afbd86618b752a6dd5b",
}

// The byte-exact EPLs the deploy steps pin; the s0 projection switches on
// which of the two pinned forms the step carries.
const contextKeySegmentedSubselectPrevPriorEPLContext = "@Name('context') @public create context SegmentedByString partition by theString from SupportBean"
const contextKeySegmentedSubselectPrevPriorEPLPrev = "@Name('s0') context SegmentedByString select theString, (select prev(0, id) from SupportBean_S0#keepall) as col1 from SupportBean"
const contextKeySegmentedSubselectPrevPriorEPLPrior = "@Name('s0') context SegmentedByString select theString, (select prior(0, id) from SupportBean_S0#keepall) as col1 from SupportBean"

// cksppBean mirrors SupportBean's asserted fields.
type cksppBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// cksppS0 mirrors SupportBean_S0's asserted fields.
type cksppS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type cksppCaseState struct {
	caseName    string
	env         *esper.Environment
	engine      *esper.Engine
	trace       *compat.Trace
	sequences   map[string]uint64
	statements  map[string]*esper.Statement
	deployments map[string]string
	plans       map[string]esper.Plan
}

func runContextKeySegmentedSubselectPrevPriorScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range contextKeySegmentedSubselectPrevPriorCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runContextKeySegmentedSubselectPrevPriorCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-subselect-prev-prior case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("context-key-segmented-subselect-prev-prior scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runContextKeySegmentedSubselectPrevPriorCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[cksppBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[cksppS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(contextKeySegmentedSubselectPrevPriorCaseRuntimeIDs[caseName]),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &cksppCaseState{
		caseName:    caseName,
		env:         env,
		engine:      engine,
		trace:       &compat.Trace{Version: scenario.Version, ID: scenario.ID},
		sequences:   make(map[string]uint64),
		statements:  make(map[string]*esper.Statement),
		deployments: make(map[string]string),
		plans:       make(map[string]esper.Plan),
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
			if err := state.send(ctx, step); err != nil {
				return *state.trace, err
			}
		case "types":
			if err := state.types(step); err != nil {
				return *state.trace, err
			}
		case "undeploy":
			if err := state.undeploy(ctx, step.Statement); err != nil {
				return *state.trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *state.trace, err
			}
		default:
			return *state.trace, fmt.Errorf("context-key-segmented-subselect-prev-prior: unsupported step op %q", step.Op)
		}
	}
	return *state.trace, nil
}

func (s *cksppCaseState) deploy(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "ctx":
		if step.Epl != contextKeySegmentedSubselectPrevPriorEPLContext {
			return fmt.Errorf("context-key-segmented-subselect-prev-prior: deploy %q carries an unpinned EPL %q", step.Statement, step.Epl)
		}
		// `@Name('context') @public create context SegmentedByString
		// partition by theString from SupportBean` — the single-stream form
		// records streamKeys so statements on unlisted types stay
		// rejectable while unlisted-type events still fan out to existing
		// partitions.
		_, err := esper.CreateKeyContextByStreams(s.env, "SegmentedByString",
			esper.KeyContextStream{
				Type: "SupportBean",
				Keys: []esper.Expr{esper.Field[cksppBean, string]("theString")},
			})
		return err
	case "s0":
		inner := esper.From[cksppS0](s.env, "SupportBean_S0").Window(esper.KeepAll()).AsRecord()
		var projection esper.Expression[int]
		switch step.Epl {
		case contextKeySegmentedSubselectPrevPriorEPLPrev:
			projection = esper.Prev[int](0, esper.Field[any, int]("id"))
		case contextKeySegmentedSubselectPrevPriorEPLPrior:
			// Esper prior(0, id) inside a subquery refers to the current
			// subquery row; Go Prior(0, x) matches Esper prior(1, x) by
			// convention, so the current row is the plain field.
			projection = esper.Field[any, int]("id")
		default:
			return fmt.Errorf("context-key-segmented-subselect-prev-prior: deploy %q carries an unpinned EPL %q", step.Statement, step.Epl)
		}
		plan, err := s.env.Build(esper.Select(
			esper.From[cksppBean](s.env, "SupportBean"),
			esper.Alias("theString", esper.Field[cksppBean, string]("theString")),
			esper.Alias("col1", esper.SubqueryValueWithOptions[int](inner, projection,
				esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))),
		).Query(esper.StatementName("s0"), esper.WithContext("SegmentedByString")))
		if err != nil {
			return fmt.Errorf("context-key-segmented-subselect-prev-prior: deploy %q: %w", step.Statement, err)
		}
		deployment, err := s.engine.Deploy(ctx, plan)
		if err != nil {
			return fmt.Errorf("context-key-segmented-subselect-prev-prior: deploy %q: %w", step.Statement, err)
		}
		for _, statement := range deployment.Statements() {
			s.statements[step.Statement] = statement
			s.deployments[step.Statement] = statement.DeploymentID()
			s.plans[step.Statement] = plan
			stmt := statement
			if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				s.sequences["listener:"+stmt.Name()]++
				s.trace.Records = append(s.trace.Records, compat.TraceRecord{
					Case:      s.caseName,
					Operation: "listener",
					Statement: stmt.Name(),
					Sequence:  s.sequences["listener:"+stmt.Name()],
					Time:      compat.FormatTraceTime(s.engine.Now()),
					New:       compat.NormalizeResults(batch.New),
					Old:       compat.NormalizeResults(batch.Old),
				})
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("context-key-segmented-subselect-prev-prior: unknown deploy %q in case %q", step.Statement, s.caseName)
	}
}

func (s *cksppCaseState) send(ctx context.Context, step compat.Step) error {
	var payload any
	switch step.EventType {
	case "SupportBean":
		var fields struct {
			TheString    string `json:"theString"`
			IntPrimitive int    `json:"intPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &fields); err != nil {
			return err
		}
		payload = cksppBean{TheString: fields.TheString, IntPrimitive: fields.IntPrimitive}
	case "SupportBean_S0":
		var fields struct {
			ID  int    `json:"id"`
			P00 string `json:"p00"`
		}
		if err := json.Unmarshal(step.Payload, &fields); err != nil {
			return err
		}
		payload = cksppS0{ID: fields.ID, P00: fields.P00}
	default:
		return fmt.Errorf("context-key-segmented-subselect-prev-prior: unknown event type %q", step.EventType)
	}
	return s.engine.Send(ctx, step.EventType, payload)
}

// types pins the asserted select-clause property surface: theString String
// and col1 Integer (the boxed Java type of the scalar subquery). The Go
// schema is verified against the equivalent shape before the pinned Java
// names are recorded.
func (s *cksppCaseState) types(step compat.Step) error {
	plan, ok := s.plans[step.Statement]
	if !ok {
		return fmt.Errorf("context-key-segmented-subselect-prev-prior: types statement %q was not deployed", step.Statement)
	}
	schema, ok := plan.ResultSchema()
	if !ok {
		return fmt.Errorf("context-key-segmented-subselect-prev-prior: statement %q has no result schema", step.Statement)
	}
	if step.Statement != "s0" {
		return fmt.Errorf("context-key-segmented-subselect-prev-prior: unknown types statement %q", step.Statement)
	}
	theString, exists := schema.Field("theString")
	if !exists || theString.Type != reflect.TypeOf("") {
		return fmt.Errorf("context-key-segmented-subselect-prev-prior: s0 theString drift: %v", theString.Type)
	}
	col1, exists := schema.Field("col1")
	if !exists || col1.Type != reflect.TypeOf(0) {
		return fmt.Errorf("context-key-segmented-subselect-prev-prior: s0 col1 drift: %v", col1.Type)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "types",
		Statement: step.Statement,
		Sequence:  0,
		Time:      compat.FormatTraceTime(s.engine.Now()),
		Value: map[string]any{
			"properties": map[string]any{"col1": "Integer", "theString": "String"},
		},
	})
	return nil
}

// undeploy removes the deployment registered under the label, mirroring
// undeployModuleContaining("s0"): the redeployed statement starts with
// fresh per-partition subquery state.
func (s *cksppCaseState) undeploy(ctx context.Context, label string) error {
	deploymentID, ok := s.deployments[label]
	if !ok {
		return fmt.Errorf("context-key-segmented-subselect-prev-prior: unknown undeploy label %q", label)
	}
	if err := s.engine.Undeploy(ctx, deploymentID); err != nil {
		return fmt.Errorf("context-key-segmented-subselect-prev-prior: undeploy %q: %w", label, err)
	}
	delete(s.deployments, label)
	delete(s.statements, label)
	delete(s.plans, label)
	return nil
}

func (s *cksppCaseState) undeployAll(ctx context.Context) error {
	for name, deploymentID := range s.deployments {
		if err := s.engine.Undeploy(ctx, deploymentID); err != nil {
			return fmt.Errorf("context-key-segmented-subselect-prev-prior: undeploy-all %q: %w", name, err)
		}
	}
	s.statements = make(map[string]*esper.Statement)
	s.deployments = make(map[string]string)
	s.plans = make(map[string]esper.Plan)
	return nil
}

// loadContextKeySegmentedSubselectPrevPriorScenario decodes the scenario
// with the strict-shape checks the raw-mutation tests pin: no duplicate
// JSON keys, the exact top-level field set, pinned case metadata, and
// per-step field whitelists so unknown or duplicated step fields fail the
// replay.
func loadContextKeySegmentedSubselectPrevPriorScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read context-key-segmented-subselect-prev-prior scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-subselect-prev-prior scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-subselect-prev-prior scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior scenario is missing field %q", name)
		}
	}
	var cases []struct {
		Case          string `json:"case"`
		Ordinal       int    `json:"ordinal"`
		RuntimeID     string `json:"runtimeId"`
		ExecutionName string `json:"executionName"`
	}
	if err := json.Unmarshal(root["cases"], &cases); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-subselect-prev-prior cases: %w", err)
	}
	if len(cases) != len(contextKeySegmentedSubselectPrevPriorCases) {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior scenario must contain exactly %d cases", len(contextKeySegmentedSubselectPrevPriorCases))
	}
	for index, entry := range cases {
		if entry.Case != contextKeySegmentedSubselectPrevPriorCases[index] ||
			entry.Ordinal != 8 ||
			entry.RuntimeID != contextKeySegmentedSubselectPrevPriorCaseRuntimeIDs[entry.Case] ||
			entry.ExecutionName != contextKeySegmentedSubselectPrevPriorJavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior scenario case %d metadata is not pinned", index)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-subselect-prev-prior steps: %w", err)
	}
	stepFields := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"send":         {"op", "case", "eventType", "payload"},
		"types":        {"op", "case", "statement"},
		"undeploy":     {"op", "case", "statement"},
		"undeploy-all": {"op", "case"},
	}
	if len(rawSteps) != 22 {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior scenario must contain exactly 22 steps, found %d", len(rawSteps))
	}
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior step %d: %w", index, err)
		}
		var op string
		if err := json.Unmarshal(object["op"], &op); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior step %d op: %w", index, err)
		}
		allowed, ok := stepFields[op]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior step %d has unsupported op %q", index, op)
		}
		for field := range object {
			found := false
			for _, name := range allowed {
				if field == name {
					found = true
					break
				}
			}
			if !found {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior step %d has unexpected field %q", index, field)
			}
		}
		var stepCase string
		if err := json.Unmarshal(object["case"], &stepCase); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior step %d case: %w", index, err)
		}
		caseKnown := false
		for _, name := range contextKeySegmentedSubselectPrevPriorCases {
			if stepCase == name {
				caseKnown = true
				break
			}
		}
		if !caseKnown {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior step %d has unknown case %q", index, stepCase)
		}
		if op == "send" {
			var eventType string
			if err := json.Unmarshal(object["eventType"], &eventType); err != nil {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior step %d eventType: %w", index, err)
			}
			payloadFields := map[string][]string{
				"SupportBean":    {"theString", "intPrimitive"},
				"SupportBean_S0": {"id", "p00"},
			}
			allowedPayload, ok := payloadFields[eventType]
			if !ok {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior step %d has unknown event type %q", index, eventType)
			}
			var payload map[string]json.RawMessage
			if err := strictObject(object["payload"], &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior step %d payload: %w", index, err)
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
					return compat.Scenario{}, fmt.Errorf("context-key-segmented-subselect-prev-prior step %d payload has unexpected field %q", index, field)
				}
			}
		}
	}
	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-subselect-prev-prior scenario: %w", err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}
