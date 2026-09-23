package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// context_key_segmented_named_window.go replays ContextKeySegmentedNamedWindow
// ordinals 0 and 3 against the pinned Java trace:
//
//   - keyed-named-window-basic (ord 0, ContextKeyedNamedWindowBasic): the
//     verbatim four-statement module — a two-key keyed context Ctx over
//     grp/subGrp, a contexted unique(type) named window EventData, the
//     contexted insert-into, and the contexted irstream consumer Test —
//     followed by one SupportGroupSubgroupEvent(G1,SG1,1,10.45) send that
//     delivers a single new-only row.
//   - keyed-named-window-faf (ord 3, ContextKeyedNamedWindowFAF): three
//     compileDeploy calls sharing the module path — the keyed
//     SegmentedByString context, the contexted keepall window MyWindow, and
//     the contexted insert-into — then a no-selector executeQuery after each
//     of two SupportBean sends, returning the G1 partition row and then both
//     partition rows in allocation order.
//
// The SupportBean payload decodes into the shared
// contextKeySegmentedInfraPrioritizedBean so the faf select * row carries all
// twenty Java bean properties.

type contextKeySegmentedNamedWindowGroup struct {
	Grp    string  `esper:"grp"`
	SubGrp string  `esper:"subGrp"`
	Type   int     `esper:"type"`
	Value  float64 `esper:"value"`
}

const contextKeySegmentedNamedWindowID = "context-key-segmented-named-window"

const contextKeySegmentedNamedWindowJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedNamedWindowJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmentedNamedWindow.java",
}

var contextKeySegmentedNamedWindowJavaRuntimeIDs = []string{
	"java-runtime-18f8400337cdcd1dffd3", // ContextKeyedNamedWindowBasic
	"java-runtime-73bbdb8596de168d6d94", // ContextKeyedNamedWindowFAF
}

var contextKeySegmentedNamedWindowJavaExecutions = []string{
	"ContextKeyedNamedWindowBasic",
	"ContextKeyedNamedWindowFAF",
}

// runContextKeySegmentedNamedWindowScenario replays both cases in scenario
// order, each on a fresh engine like the Java execution's undeployAll.
func runContextKeySegmentedNamedWindowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedNamedWindowCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-named-window case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedNamedWindowCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedInfraPrioritizedBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextKeySegmentedNamedWindowGroup](env, "SupportGroupSubgroupEvent"); err != nil {
		return compat.Trace{}, err
	}
	switch caseName {
	case "keyed-named-window-basic":
		return runContextKeySegmentedNamedWindowBasicCase(ctx, env, caseScenario)
	case "keyed-named-window-faf":
		return runContextKeySegmentedNamedWindowFAFCase(ctx, env, caseScenario)
	default:
		return compat.Trace{}, fmt.Errorf("%s: unknown case %q", contextKeySegmentedNamedWindowID, caseName)
	}
}

// runContextKeySegmentedNamedWindowBasicCase builds the two-key context, the
// contexted unique(type) window, the contexted wildcard insert-into, and the
// contexted irstream consumer; the single send drives the insert and the
// consumer in one partition.
func runContextKeySegmentedNamedWindowBasicCase(ctx context.Context, env *esper.Environment, caseScenario compat.Scenario) (compat.Trace, error) {
	beanSource := esper.From[contextKeySegmentedNamedWindowGroup](env, "SupportGroupSubgroupEvent")
	grp := esper.Field[contextKeySegmentedNamedWindowGroup, string]("grp")
	subGrp := esper.Field[contextKeySegmentedNamedWindowGroup, string]("subGrp")
	typeField := esper.Field[contextKeySegmentedNamedWindowGroup, int]("type")
	// create context Ctx partition by grp, subGrp from SupportGroupSubgroupEvent
	if _, err := esper.CreateKeyContext(env, "Ctx", grp, subGrp); err != nil {
		return compat.Trace{}, err
	}
	schema, ok := env.Schema("SupportGroupSubgroupEvent")
	if !ok {
		return compat.Trace{}, fmt.Errorf("%s: SupportGroupSubgroupEvent schema is not registered", contextKeySegmentedNamedWindowID)
	}
	// context Ctx create window EventData#unique(type) as SupportGroupSubgroupEvent
	if _, err := esper.CreateNamedWindow(env, "EventData", schema,
		esper.NamedWindowContext("Ctx"), esper.NamedWindowRetention(esper.Unique(typeField))); err != nil {
		return compat.Trace{}, err
	}
	// context Ctx insert into EventData select * from SupportGroupSubgroupEvent
	insertPlan, err := env.Build(beanSource.InsertInto("EventData",
		esper.StatementName("Insert"), esper.WithContext("Ctx")))
	if err != nil {
		return compat.Trace{}, err
	}
	// context Ctx select irstream * from EventData — WithOldStream selects
	// irstream output (new and old rows).
	testPlan, err := env.Build(esper.FromNamedWindow(env, "EventData").Query(
		esper.StatementName("Test"), esper.WithContext("Ctx"), esper.WithOldStream()))
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(ctx, insertPlan); err != nil {
		return compat.Trace{}, err
	}
	deployment, err := engine.Deploy(ctx, testPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	if len(deployment.Statements()) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
	}
	statement := deployment.Statements()[0]
	return compat.ReplayWithStatementsAndHandlers(ctx, engine, statement, caseScenario,
		decodeContextKeySegmentedNamedWindowPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-key-segmented-named-window statement %q", name)
			}
			return statement, nil
		}, contextKeySegmentedNamedWindowNoopHandlers())
}

// runContextKeySegmentedNamedWindowFAFCase builds the keyed context, the
// contexted keepall window, and the contexted wildcard insert-into; the faf
// steps execute the pinned select * without a selector, which resolves to
// every live partition in allocation order.
func runContextKeySegmentedNamedWindowFAFCase(ctx context.Context, env *esper.Environment, caseScenario compat.Scenario) (compat.Trace, error) {
	beanSource := esper.From[contextKeySegmentedInfraPrioritizedBean](env, "SupportBean")
	theString := esper.Field[contextKeySegmentedInfraPrioritizedBean, string]("theString")
	// @public create context SegmentedByString partition by theString from SupportBean
	if _, err := esper.CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		return compat.Trace{}, err
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		return compat.Trace{}, fmt.Errorf("%s: SupportBean schema is not registered", contextKeySegmentedNamedWindowID)
	}
	// @public context SegmentedByString create window MyWindow#keepall as SupportBean
	if _, err := esper.CreateNamedWindow(env, "MyWindow", schema,
		esper.NamedWindowContext("SegmentedByString"), esper.NamedWindowRetention(esper.KeepAll())); err != nil {
		return compat.Trace{}, err
	}
	// context SegmentedByString insert into MyWindow select * from SupportBean
	insertPlan, err := env.Build(beanSource.InsertInto("MyWindow",
		esper.StatementName("insert"), esper.WithContext("SegmentedByString")))
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(ctx, insertPlan); err != nil {
		return compat.Trace{}, err
	}
	fafPlan, err := env.Build(esper.FromNamedWindow(env, "MyWindow").Query())
	if err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case", "deploy", "undeploy-all":
			// The runner pre-deploys every statement; the oracle's deploy
			// steps are byte-exact EPL evidence only.
		case "send":
			event, err := decodeContextKeySegmentedNamedWindowPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return trace, err
			}
		case "faf":
			if step.Epl != "select * from MyWindow" {
				return trace, fmt.Errorf("%s: faf %q carries an unpinned EPL %q", contextKeySegmentedNamedWindowID, step.Statement, step.Epl)
			}
			if step.Selector != "none" {
				return trace, fmt.Errorf("%s: faf %q carries an unsupported selector %q", contextKeySegmentedNamedWindowID, step.Statement, step.Selector)
			}
			result, err := engine.ExecuteFireAndForget(ctx, fafPlan)
			if err != nil {
				return trace, fmt.Errorf("%s: faf %q: %w", contextKeySegmentedNamedWindowID, step.Statement, err)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      step.Case,
				Operation: "faf",
				Statement: step.Statement,
				Time:      compat.FormatTraceTime(engine.Now()),
				New:       compat.NormalizeResults(result.Results()),
			})
		default:
			return trace, fmt.Errorf("%s: unsupported step op %q", contextKeySegmentedNamedWindowID, step.Op)
		}
	}
	return trace, nil
}

// contextKeySegmentedNamedWindowNoopHandlers swallows the pinned
// deploy/undeploy-all steps: the runner pre-deploys every statement before
// replay, so the oracle's deploy steps are byte-exact EPL evidence only.
func contextKeySegmentedNamedWindowNoopHandlers() map[string]compat.StepHandler {
	noop := func(compat.Step, func(*esper.Statement) error) ([]compat.TraceRecord, error) { return nil, nil }
	return map[string]compat.StepHandler{"deploy": noop, "undeploy-all": noop}
}

func decodeContextKeySegmentedNamedWindowPayload(step compat.Step) (any, error) {
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
	case "SupportGroupSubgroupEvent":
		var value contextKeySegmentedNamedWindowGroup
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportGroupSubgroupEvent: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-key-segmented-named-window event type %q", step.EventType)
	}
}

// loadContextKeySegmentedNamedWindowScenario decodes the scenario with the
// strict-shape checks the raw-mutation tests pin: no duplicate JSON keys,
// the exact top-level field set, pinned case metadata, and per-step field
// whitelists so unknown or duplicated step fields fail the replay.
func loadContextKeySegmentedNamedWindowScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read context-key-segmented-named-window scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-named-window scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-named-window scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window scenario is missing field %q", name)
		}
	}
	var cases []struct {
		Case          string `json:"case"`
		Ordinal       int    `json:"ordinal"`
		RuntimeID     string `json:"runtimeId"`
		ExecutionName string `json:"executionName"`
	}
	if err := json.Unmarshal(root["cases"], &cases); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-named-window cases: %w", err)
	}
	wantCases := []struct {
		name      string
		ordinal   int
		runtimeID string
		execution string
	}{
		{"keyed-named-window-basic", 0, "java-runtime-18f8400337cdcd1dffd3", "ContextKeyedNamedWindowBasic"},
		{"keyed-named-window-faf", 3, "java-runtime-73bbdb8596de168d6d94", "ContextKeyedNamedWindowFAF"},
	}
	if len(cases) != len(wantCases) {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window scenario must contain exactly %d cases", len(wantCases))
	}
	for index, entry := range cases {
		want := wantCases[index]
		if entry.Case != want.name || entry.Ordinal != want.ordinal ||
			entry.RuntimeID != want.runtimeID || entry.ExecutionName != want.execution {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window scenario case %d metadata is not pinned", index)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-named-window steps: %w", err)
	}
	stepFields := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"send":         {"op", "case", "eventType", "payload"},
		"faf":          {"op", "case", "statement", "epl", "selector"},
		"undeploy-all": {"op", "case"},
	}
	if len(rawSteps) != 13 {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window scenario must contain exactly 13 steps, found %d", len(rawSteps))
	}
	knownCases := map[string]bool{}
	for _, want := range wantCases {
		knownCases[want.name] = true
	}
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window step %d: %w", index, err)
		}
		var op string
		if err := json.Unmarshal(object["op"], &op); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window step %d op: %w", index, err)
		}
		allowed, ok := stepFields[op]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window step %d has unsupported op %q", index, op)
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
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window step %d has unexpected field %q", index, field)
			}
		}
		var stepCase string
		if err := json.Unmarshal(object["case"], &stepCase); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window step %d case: %w", index, err)
		}
		if !knownCases[stepCase] {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window step %d has unknown case %q", index, stepCase)
		}
		if op == "send" {
			var eventType string
			if err := json.Unmarshal(object["eventType"], &eventType); err != nil {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window step %d eventType: %w", index, err)
			}
			payloadFields := map[string][]string{
				"SupportBean":               {"theString", "intPrimitive"},
				"SupportGroupSubgroupEvent": {"grp", "subGrp", "type", "value"},
			}
			allowedPayload, ok := payloadFields[eventType]
			if !ok {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window step %d has unknown event type %q", index, eventType)
			}
			var payload map[string]json.RawMessage
			if err := strictObject(object["payload"], &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window step %d payload: %w", index, err)
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
					return compat.Scenario{}, fmt.Errorf("context-key-segmented-named-window step %d payload has unexpected field %q", index, field)
				}
			}
		}
	}
	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-named-window scenario: %w", err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}
