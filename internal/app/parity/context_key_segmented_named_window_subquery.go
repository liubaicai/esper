package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// context_key_segmented_named_window_subquery.go replays
// ContextKeySegmentedNamedWindow ordinals 4 and 5 against the pinned Java
// trace:
//
//   - keyed-subquery-nw-index-unshared (ord 4,
//     ContextKeyedSubqueryNamedWindowIndexUnShared): one module deploys the
//     keyed SegmentedByString context, the context-FREE keepall window
//     MyWindowThree over SupportBean_S0, the context-free insert-into, and
//     the contexted s0 statement whose scalar subquery correlates
//     sb.intPrimitive = s0.id against the global window.
//   - keyed-subquery-nw-index-shared (ord 5,
//     ContextKeyedSubqueryNamedWindowIndexShared): four path-shared deploys
//     with @public context/window and the enable_window_subquery_indexshare
//     hint on the create-window (NamedWindowSubqueryIndexSharing); the
//     observable output is identical to the unshared variant.
//
// The correlation is not partition-scoped: every context partition reads the
// same global window snapshot, so G2 sees s1 and G1 sees s2 after it was
// inserted while only G3 events had flowed; val0 is null when no row matches.
// Java's milestone calls are harness no-ops and carry no steps.

const contextKeySegmentedNamedWindowSubqueryID = "context-key-segmented-named-window-subquery"

const contextKeySegmentedNamedWindowSubqueryJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedNamedWindowSubqueryJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmentedNamedWindow.java",
}

var contextKeySegmentedNamedWindowSubqueryJavaRuntimeIDs = []string{
	"java-runtime-7347c7d16d52e5ea0d30", // ContextKeyedSubqueryNamedWindowIndexUnShared
	"java-runtime-af7bcf071474f57227fb", // ContextKeyedSubqueryNamedWindowIndexShared
}

var contextKeySegmentedNamedWindowSubqueryJavaExecutions = []string{
	"ContextKeyedSubqueryNamedWindowIndexUnShared",
	"ContextKeyedSubqueryNamedWindowIndexShared",
}

// runContextKeySegmentedNamedWindowSubqueryScenario replays both cases in
// scenario order, each on a fresh engine like the Java execution's
// undeployAll.
func runContextKeySegmentedNamedWindowSubqueryScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedNamedWindowSubqueryCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-named-window-subquery case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedNamedWindowSubqueryCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedInfraPrioritizedBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	s0Schema, err := esper.RegisterStruct[contextKeySegmentedInfraPrioritizedS0](env, "SupportBean_S0")
	if err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedInfraPrioritizedBean](env, "SupportBean")
	s0Source := esper.From[contextKeySegmentedInfraPrioritizedS0](env, "SupportBean_S0")
	theString := esper.Field[contextKeySegmentedInfraPrioritizedBean, string]("theString")
	intPrimitive := esper.Field[contextKeySegmentedInfraPrioritizedBean, int]("intPrimitive")

	var windowName string
	var windowOptions []esper.NamedWindowOption
	switch caseName {
	case "keyed-subquery-nw-index-unshared":
		windowName = "MyWindowThree"
	case "keyed-subquery-nw-index-shared":
		// @Hint('enable_window_subquery_indexshare') on the create-window:
		// all context partitions share one subquery index on s0.id.
		windowName = "MyWindowTwo"
		windowOptions = append(windowOptions, esper.NamedWindowSubqueryIndexSharing())
	default:
		return compat.Trace{}, fmt.Errorf("%s: unknown case %q", contextKeySegmentedNamedWindowSubqueryID, caseName)
	}

	// create context SegmentedByString partition by theString from SupportBean
	if _, err := esper.CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		return compat.Trace{}, err
	}
	// create window MyWindow{Two,Three}#keepall as SupportBean_S0 — no
	// NamedWindowContext: the window is context-free (one global partition).
	windowOptions = append(windowOptions, esper.NamedWindowRetention(esper.KeepAll()))
	if _, err := esper.CreateNamedWindow(env, windowName, s0Schema, windowOptions...); err != nil {
		return compat.Trace{}, err
	}
	// insert into MyWindow{Two,Three} select * from SupportBean_S0
	insertPlan, err := env.Build(s0Source.InsertInto(windowName, esper.StatementName("insert")))
	if err != nil {
		return compat.Trace{}, err
	}
	// @Name('s0') context SegmentedByString select theString, intPrimitive,
	// (select p00 from W as s0 where sb.intPrimitive = s0.id) as val0 from
	// SupportBean as sb — the inner window is context-free, so every
	// partition probes the same global snapshot.
	s0Plan, err := env.Build(esper.Select(beanSource,
		esper.Alias("theString", theString),
		esper.Alias("intPrimitive", intPrimitive),
		esper.Alias("val0", esper.SubqueryValue[string](
			esper.FromNamedWindow(env, windowName),
			esper.Field[contextKeySegmentedInfraPrioritizedS0, string]("p00"),
			esper.Equal[int](esper.Field[contextKeySegmentedInfraPrioritizedS0, int]("id"), esper.OuterField[int]("intPrimitive")),
		)),
	).Query(esper.StatementName("s0"), esper.WithContext("SegmentedByString")))
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(ctx, insertPlan); err != nil {
		return compat.Trace{}, err
	}
	deployment, err := engine.Deploy(ctx, s0Plan)
	if err != nil {
		return compat.Trace{}, err
	}
	if len(deployment.Statements()) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
	}
	statement := deployment.Statements()[0]
	return compat.ReplayWithStatementsAndHandlers(ctx, engine, statement, caseScenario,
		decodeContextKeySegmentedNamedWindowSubqueryPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-key-segmented-named-window-subquery statement %q", name)
			}
			return statement, nil
		}, contextKeySegmentedNamedWindowSubqueryNoopHandlers())
}

// contextKeySegmentedNamedWindowSubqueryNoopHandlers swallows the pinned
// deploy/undeploy-all steps: the runner pre-deploys every statement before
// replay, so the oracle's deploy steps are byte-exact EPL evidence only.
func contextKeySegmentedNamedWindowSubqueryNoopHandlers() map[string]compat.StepHandler {
	noop := func(compat.Step, func(*esper.Statement) error) ([]compat.TraceRecord, error) { return nil, nil }
	return map[string]compat.StepHandler{"deploy": noop, "undeploy-all": noop}
}

func decodeContextKeySegmentedNamedWindowSubqueryPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextKeySegmentedInfraPrioritizedBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		// charPrimitive mirrors Java's char default: an absent payload
		// field decodes to the NUL character the Java bean carries.
		if value.CharPrimitive == "" {
			value.CharPrimitive = "\x00"
		}
		return value, nil
	case "SupportBean_S0":
		var value contextKeySegmentedInfraPrioritizedS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-key-segmented-named-window-subquery event type %q", step.EventType)
	}
}

// loadContextKeySegmentedNamedWindowSubqueryScenario decodes the scenario
// with the strict-shape checks the raw-mutation tests pin: no duplicate JSON
// keys, the exact top-level field set, pinned case metadata, and per-step
// field whitelists so unknown or duplicated step fields fail the replay.
func loadContextKeySegmentedNamedWindowSubqueryScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read context-key-segmented-named-window-subquery scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-named-window-subquery scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-named-window-subquery scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery scenario is missing field %q", name)
		}
	}
	var cases []struct {
		Case          string `json:"case"`
		Ordinal       int    `json:"ordinal"`
		RuntimeID     string `json:"runtimeId"`
		ExecutionName string `json:"executionName"`
	}
	if err := json.Unmarshal(root["cases"], &cases); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-named-window-subquery cases: %w", err)
	}
	wantCases := []struct {
		name      string
		ordinal   int
		runtimeID string
		execution string
	}{
		{"keyed-subquery-nw-index-unshared", 4, "java-runtime-7347c7d16d52e5ea0d30", "ContextKeyedSubqueryNamedWindowIndexUnShared"},
		{"keyed-subquery-nw-index-shared", 5, "java-runtime-af7bcf071474f57227fb", "ContextKeyedSubqueryNamedWindowIndexShared"},
	}
	if len(cases) != len(wantCases) {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery scenario must contain exactly %d cases", len(wantCases))
	}
	for index, entry := range cases {
		want := wantCases[index]
		if entry.Case != want.name || entry.Ordinal != want.ordinal ||
			entry.RuntimeID != want.runtimeID || entry.ExecutionName != want.execution {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery scenario case %d metadata is not pinned", index)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-named-window-subquery steps: %w", err)
	}
	stepFields := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"send":         {"op", "case", "eventType", "payload"},
		"undeploy-all": {"op", "case"},
	}
	if len(rawSteps) != 23 {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery scenario must contain exactly 23 steps, found %d", len(rawSteps))
	}
	knownCases := map[string]bool{}
	for _, want := range wantCases {
		knownCases[want.name] = true
	}
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery step %d: %w", index, err)
		}
		var op string
		if err := json.Unmarshal(object["op"], &op); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery step %d op: %w", index, err)
		}
		allowed, ok := stepFields[op]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery step %d has unsupported op %q", index, op)
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
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery step %d has unexpected field %q", index, field)
			}
		}
		var stepCase string
		if err := json.Unmarshal(object["case"], &stepCase); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery step %d case: %w", index, err)
		}
		if !knownCases[stepCase] {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery step %d has unknown case %q", index, stepCase)
		}
		if op == "send" {
			var eventType string
			if err := json.Unmarshal(object["eventType"], &eventType); err != nil {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery step %d eventType: %w", index, err)
			}
			payloadFields := map[string][]string{
				"SupportBean":    {"theString", "intPrimitive"},
				"SupportBean_S0": {"id", "p00"},
			}
			allowedPayload, ok := payloadFields[eventType]
			if !ok {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery step %d has unknown event type %q", index, eventType)
			}
			var payload map[string]json.RawMessage
			if err := strictObject(object["payload"], &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery step %d payload: %w", index, err)
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
					return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window-subquery step %d payload has unexpected field %q", index, field)
				}
			}
		}
	}
	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-named-window-subquery scenario: %w", err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}
