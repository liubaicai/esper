package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// context_init_term_prioritized.go replays ContextInitTermPrioritized
// (ords 0 and 1) against the pinned Java oracle:
//
//   - nonoverlapping-subquery (ord 0,
//     ContextInitTermPrioNonOverlappingSubqueryAndInvalid): the clock
//     advances to 10:00 inside the 9:00-17:00 RuleActivityTime cron
//     context, then the eight module statements deploy one per step
//     (the Java execution's single path-shared compileDeploy). The first
//     SupportProductIdEvent(A1) passes the not-exists guard on exactly
//     one of the four prioritized inserts, lands in EventsWindow, and
//     out emits one {productID=A1} row. The build-error probe then
//     verifies Go rejects the context-free insert with ErrorInvalidRule
//     ("has been declared for context") before recording the pinned
//     Java prefix.
//   - terminating-same-event (ord 1,
//     ContextInitTermPrioAtNowWithSelectedEventEnding): the
//     @Priority(1) C1 context starts @now and ends on the same
//     SupportBean event that @Priority(0) s0 selects, so E1 and E2 each
//     emit one row.
//
// Approved differences (observably identical to the Java EPL):
//   - `create context`/`create window`/`create variable` map to env-level
//     registrations (CreateCronTimeContext, CreateNamedWindow,
//     RegisterVariable); Go has no module path, so the oracle's
//     path-shared per-statement deploys have no Go-side counterpart
//     beyond ordering.
//   - @Priority annotations are unobservable ordering metadata; s0
//     carries StatementPriority(0) while the context and insert
//     statements need none.

const contextInitTermPrioritizedID = "context-init-term-prioritized"
const contextInitTermPrioritizedJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const contextInitTermPrioritizedDescription = "ContextInitTermPrioritized ords 0 and 1: a cron-scheduled (9-to-5) initiated/terminated context whose firstunique named window is fed by four insert statements guarded by a not-exists subquery — the first A1 event lands exactly once and out emits one row — followed by a tryInvalidCompile probe pinning the cross-context named-window subquery rejection; the second execution ends each @now-initiated partition with the same SupportBean event that s0 selects."

const contextInitTermPrioritizedSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermPrioritized.java"

var contextInitTermPrioritizedJavaSources = []string{
	contextInitTermPrioritizedSource,
}

var contextInitTermPrioritizedJavaRuntimeIDs = []string{
	"java-runtime-bb247dc87cf118eb8661", // ContextInitTermPrioNonOverlappingSubqueryAndInvalid
	"java-runtime-0c822c80cf402d017d61", // ContextInitTermPrioAtNowWithSelectedEventEnding
}

var contextInitTermPrioritizedJavaExecutions = []string{
	"ContextInitTermPrioNonOverlappingSubqueryAndInvalid",
	"ContextInitTermPrioAtNowWithSelectedEventEnding",
}

var contextInitTermPrioritizedJavaStaticIDs = []string{
	"java-41c13254dc50886c2dd2",
	"java-517175a60c987d2e2397",
}

var contextInitTermPrioritizedJavaFlags = []string{}

var contextInitTermPrioritizedCases = []string{
	"nonoverlapping-subquery",
	"terminating-same-event",
}

var contextInitTermPrioritizedOrdinals = []int{0, 1}

var contextInitTermPrioritizedCaseRuntimeIDs = map[string]string{
	"nonoverlapping-subquery": "java-runtime-bb247dc87cf118eb8661",
	"terminating-same-event":  "java-runtime-0c822c80cf402d017d61",
}

var contextInitTermPrioritizedCaseObservations = []string{
	"listener+compile-error; at 10:00 inside the 9:00-17:00 cron context the first SupportProductIdEvent(A1) passes the not-exists guard on exactly one of the four prioritized inserts, lands in EventsWindow and out emits one {productID=A1} row; the probe records the pinned cross-context named-window subquery rejection prefix",
	"listener; @Priority(1) context C1 starts @now and ends on the same SupportBean event that @Priority(0) s0 selects, so E1 and E2 each emit one row",
}

// contextInitTermPrioritizedCaseEPLs pins the Java module text per case:
// the eight newline-prefixed statements of the path-shared compileDeploy
// (ord 0) and the two-statement module (ord 1).
var contextInitTermPrioritizedCaseEPLs = []string{
	"\n @Name('ctx') @public create context RuleActivityTime as start (0, 9, *, *, *) end (0, 17, *, *, *);" +
		"\n @Name('window') @public context RuleActivityTime create window EventsWindow#firstunique(productID) as SupportProductIdEvent;" +
		"\n @Name('variable') create variable boolean IsOutputTriggered_2 = false;" +
		"\n @Name('A') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow));" +
		"\n @Name('B') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow));" +
		"\n @Name('C') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow));" +
		"\n @Name('D') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow));" +
		"\n @Name('out') context RuleActivityTime select * from EventsWindow",
	"@Priority(1) create context C1 start @now end SupportBean;\n" +
		"@name('s0') @Priority(0) context C1 select * from SupportBean;\n",
}

// contextInitTermPrioritizedDeployEPLs pins the byte-exact EPL each
// deploy step carries, keyed by case then statement label.
var contextInitTermPrioritizedDeployEPLs = map[string]map[string]string{
	"nonoverlapping-subquery": {
		"ctx":      "@Name('ctx') @public create context RuleActivityTime as start (0, 9, *, *, *) end (0, 17, *, *, *)",
		"window":   "@Name('window') @public context RuleActivityTime create window EventsWindow#firstunique(productID) as SupportProductIdEvent",
		"variable": "@Name('variable') create variable boolean IsOutputTriggered_2 = false",
		"A":        "@Name('A') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow))",
		"B":        "@Name('B') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow))",
		"C":        "@Name('C') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow))",
		"D":        "@Name('D') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow))",
		"out":      "@Name('out') context RuleActivityTime select * from EventsWindow",
	},
	"terminating-same-event": {
		// Java compiles the context and s0 as ONE module (non-@public C1
		// is invisible across modules), so the scenario carries a single
		// deploy step keyed by the traced statement s0.
		"s0": "@Priority(1) create context C1 start @now end SupportBean;\n" +
			"@name('s0') @Priority(0) context C1 select * from SupportBean",
	},
}

var contextInitTermPrioritizedProbeEPLs = map[string]string{
	"subquery-context-mismatch": "insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow))",
}

// contextInitTermPrioritizedCaseSteps pins the complete step sequence per
// case as op|statement|eventType|at keys so the loader asserts the
// scenario file matches the contract.
var contextInitTermPrioritizedCaseSteps = map[string][]string{
	"nonoverlapping-subquery": {
		"advance-time|||2002-05-01T10:00:00.000Z",
		"deploy|ctx||",
		"deploy|window||",
		"deploy|variable||",
		"deploy|A||",
		"deploy|B||",
		"deploy|C||",
		"deploy|D||",
		"deploy|out||",
		"send||SupportProductIdEvent|",
		"build-error|subquery-context-mismatch||",
		"undeploy-all|||",
	},
	"terminating-same-event": {
		"deploy|s0||",
		"send||SupportBean|",
		"send||SupportBean|",
		"undeploy-all|||",
	},
}

// contextInitTermPrioritizedBean mirrors SupportBean's asserted fields.
type contextInitTermPrioritizedBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// contextInitTermPrioritizedProduct mirrors SupportProductIdEvent's
// asserted field.
type contextInitTermPrioritizedProduct struct {
	ProductID string `esper:"productID"`
}

// contextInitTermPrioritizedCaseState carries the per-case replay state:
// the environment/engine pair, deployment bookkeeping for undeploy-all,
// and the trace/sequence counters the listener records draw from.
type contextInitTermPrioritizedCaseState struct {
	caseName    string
	env         *esper.Environment
	engine      *esper.Engine
	trace       *compat.Trace
	sequences   map[string]uint64
	deployments map[string]*esper.Deployment
	deployOrder []string
}

func runContextInitTermPrioritizedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range contextInitTermPrioritizedCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runContextInitTermPrioritizedCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", contextInitTermPrioritizedID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", contextInitTermPrioritizedID, scenario.ID)
	}
	return trace, nil
}

func runContextInitTermPrioritizedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermPrioritizedBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermPrioritizedProduct](env, "SupportProductIdEvent"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(contextInitTermPrioritizedCaseRuntimeIDs[caseName]),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &contextInitTermPrioritizedCaseState{
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
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return *state.trace, err
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return *state.trace, err
			}
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *state.trace, err
			}
		case "send":
			event, err := decodeContextInitTermPrioritizedPayload(step)
			if err != nil {
				return *state.trace, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return *state.trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *state.trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *state.trace, err
			}
		default:
			return *state.trace, fmt.Errorf("%s: unsupported step op %q", contextInitTermPrioritizedID, step.Op)
		}
	}
	return *state.trace, nil
}

// deploy maps each scenario label to the equivalent Go registration or
// chain-API plan. Context/window/variable statements are env-level
// registrations (Go has no module path); the insert and select
// statements build and deploy plans, and the listener attaches to the
// case's traced statement (out or s0), mirroring env.addListener.
func (s *contextInitTermPrioritizedCaseState) deploy(ctx context.Context, step compat.Step) error {
	pinned, ok := contextInitTermPrioritizedDeployEPLs[s.caseName][step.Statement]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: deploy %q in case %q carries an unpinned EPL %q",
			contextInitTermPrioritizedID, step.Statement, s.caseName, step.Epl)
	}
	switch s.caseName {
	case "nonoverlapping-subquery":
		return s.deploySubqueryStep(ctx, step.Statement)
	case "terminating-same-event":
		return s.deploySameEventStep(ctx, step.Statement)
	default:
		return fmt.Errorf("%s: unknown case %q", contextInitTermPrioritizedID, s.caseName)
	}
}

func (s *contextInitTermPrioritizedCaseState) deploySubqueryStep(ctx context.Context, label string) error {
	switch label {
	case "ctx":
		// `@public create context RuleActivityTime as start (0, 9, *, *, *)
		// end (0, 17, *, *, *)` — Esper cron field order is
		// (minute, hour, dom, month, dow): 09:00 to 17:00 daily.
		_, err := esper.CreateCronTimeContext(s.env, "RuleActivityTime",
			esper.NewCronSchedule(esper.CronValues(0), esper.CronValues(9),
				esper.CronWildcard(), esper.CronWildcard(), esper.CronWildcard()),
			esper.NewCronSchedule(esper.CronValues(0), esper.CronValues(17),
				esper.CronWildcard(), esper.CronWildcard(), esper.CronWildcard()))
		return err
	case "window":
		// `@public context RuleActivityTime create window
		// EventsWindow#firstunique(productID) as SupportProductIdEvent`.
		schema, ok := s.env.Schema("SupportProductIdEvent")
		if !ok {
			return fmt.Errorf("%s: SupportProductIdEvent schema is not registered", contextInitTermPrioritizedID)
		}
		_, err := esper.CreateNamedWindow(s.env, "EventsWindow", schema,
			esper.NamedWindowContext("RuleActivityTime"),
			esper.NamedWindowRetention(esper.FirstUnique(
				esper.Field[contextInitTermPrioritizedProduct, string]("productID"))))
		return err
	case "variable":
		// `create variable boolean IsOutputTriggered_2 = false` — nothing
		// references the variable; the registration mirrors the module
		// statement.
		return s.env.RegisterVariable("IsOutputTriggered_2", false)
	case "A", "B", "C", "D":
		// `context RuleActivityTime insert into EventsWindow select * from
		// SupportProductIdEvent(not exists (select * from EventsWindow))`.
		plan, err := s.env.Build(
			esper.From[contextInitTermPrioritizedProduct](s.env, "SupportProductIdEvent").
				Filter(esper.Not(esper.SubqueryExists(
					esper.FromNamedWindow(s.env, "EventsWindow"), nil))).
				InsertInto("EventsWindow",
					esper.StatementName(label), esper.WithContext("RuleActivityTime")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan, false)
	case "out":
		// `context RuleActivityTime select * from EventsWindow` — the
		// traced statement.
		plan, err := s.env.Build(
			esper.FromNamedWindow(s.env, "EventsWindow").
				Query(esper.StatementName("out"), esper.WithContext("RuleActivityTime")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan, true)
	default:
		return fmt.Errorf("%s: unknown deploy %q in case %q", contextInitTermPrioritizedID, label, s.caseName)
	}
}

func (s *contextInitTermPrioritizedCaseState) deploySameEventStep(ctx context.Context, label string) error {
	switch label {
	case "s0":
		// `@Priority(1) create context C1 start @now end SupportBean` +
		// `@name('s0') @Priority(0) context C1 select * from SupportBean`
		// compile as ONE Java module (non-@public C1 is invisible across
		// modules), so the single deploy step registers the context then
		// builds and deploys s0. @now initiates immediately; the
		// terminating SupportBean event is the same event s0 selects.
		isBean := esper.Equal[string](
			esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean"))
		if _, err := esper.CreateInitiatedTerminatedContext(s.env, "C1",
			esper.Literal("global"), esper.Literal(true), isBean); err != nil {
			return err
		}
		plan, err := s.env.Build(
			esper.From[contextInitTermPrioritizedBean](s.env, "SupportBean").
				Query(esper.StatementName("s0"), esper.WithContext("C1"),
					esper.StatementPriority(0)))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan, true)
	default:
		return fmt.Errorf("%s: unknown deploy %q in case %q", contextInitTermPrioritizedID, label, s.caseName)
	}
}

// deployPlan deploys one plan and, when traced, subscribes the listener
// that emits the normalized per-delivery records.
func (s *contextInitTermPrioritizedCaseState) deployPlan(ctx context.Context, label string,
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
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				// Force-dispatched empty pairs are not recorded and do not
				// consume a sequence number (Java oracle convention).
				return nil
			}
			s.sequences[stmt.Name()]++
			s.trace.Records = append(s.trace.Records, compat.TraceRecord{
				Case:      s.caseName,
				Operation: "listener",
				Statement: stmt.Name(),
				Sequence:  s.sequences[stmt.Name()],
				Time:      compat.FormatTraceTime(batch.Time),
				New:       compat.NormalizeResults(batch.New),
				Old:       compat.NormalizeResults(batch.Old),
			})
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// buildError runs the expected-invalid probe: the same insert without
// WithContext, which Go rejects with ErrorInvalidRule because
// EventsWindow is declared for RuleActivityTime. The pinned Java prefix
// is recorded after the Go rejection is verified.
func (s *contextInitTermPrioritizedCaseState) buildError(step compat.Step) error {
	pinned, ok := contextInitTermPrioritizedProbeEPLs[step.Statement]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q",
			contextInitTermPrioritizedID, step.Statement, step.Epl)
	}
	var buildErr error
	switch step.Statement {
	case "subquery-context-mismatch":
		// `insert into EventsWindow select * from SupportProductIdEvent
		// (not exists (select * from EventsWindow))` — the context-free
		// form reads a named window declared for RuleActivityTime.
		_, buildErr = s.env.Build(
			esper.From[contextInitTermPrioritizedProduct](s.env, "SupportProductIdEvent").
				Filter(esper.Not(esper.SubqueryExists(
					esper.FromNamedWindow(s.env, "EventsWindow"), nil))).
				InsertInto("EventsWindow", esper.StatementName(step.Statement)))
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", contextInitTermPrioritizedID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", contextInitTermPrioritizedID, step.Statement)
	}
	var espErr *esper.Error
	if !errors.As(buildErr, &espErr) || espErr.Code != esper.ErrorInvalidRule ||
		!strings.Contains(buildErr.Error(), "has been declared for context") {
		return fmt.Errorf("%s: build-error probe %q drift: got %v", contextInitTermPrioritizedID, step.Statement, buildErr)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// undeployAll mirrors the Java execution's undeployAll: deployments
// retire in reverse deployment order; the env-scoped context, window and
// variable registrations retire with the engine.
func (s *contextInitTermPrioritizedCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", contextInitTermPrioritizedID, label, err)
		}
	}
	s.deployments = make(map[string]*esper.Deployment)
	s.deployOrder = nil
	return nil
}

func decodeContextInitTermPrioritizedPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextInitTermPrioritizedBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportProductIdEvent":
		var value contextInitTermPrioritizedProduct
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportProductIdEvent: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", contextInitTermPrioritizedID, step.EventType)
	}
}

// loadContextInitTermPrioritizedScenario decodes the scenario with the
// strict-shape checks the raw-mutation tests pin: no duplicate JSON keys,
// the exact top-level field set, pinned case metadata, and per-step field
// whitelists so unknown or duplicated step fields fail the replay.
func loadContextInitTermPrioritizedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", contextInitTermPrioritizedID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", contextInitTermPrioritizedID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextInitTermPrioritizedID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextInitTermPrioritizedID, err)
	}
	if err := requireContextInitTermPrioritizedFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", contextInitTermPrioritizedID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != contextInitTermPrioritizedID ||
		metadata.Description != contextInitTermPrioritizedDescription ||
		metadata.JavaCommit != contextInitTermPrioritizedJavaCommit ||
		metadata.JavaSource != contextInitTermPrioritizedSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", contextInitTermPrioritizedID)
	}
	if err := validateContextInitTermPrioritizedStringArray(root["javaRuntimes"], contextInitTermPrioritizedJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextInitTermPrioritizedStringArray(root["javaNames"], contextInitTermPrioritizedJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextInitTermPrioritizedStringArray(root["javaStaticIds"], contextInitTermPrioritizedJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextInitTermPrioritizedStringArray(root["javaFlags"], contextInitTermPrioritizedJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(contextInitTermPrioritizedCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", contextInitTermPrioritizedID, len(contextInitTermPrioritizedCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireContextInitTermPrioritizedFields(object,
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
		if definition.Case != contextInitTermPrioritizedCases[index] ||
			definition.Ordinal != contextInitTermPrioritizedOrdinals[index] ||
			definition.RuntimeID != contextInitTermPrioritizedJavaRuntimeIDs[index] ||
			definition.ExecutionName != contextInitTermPrioritizedJavaExecutions[index] ||
			definition.Observation != contextInitTermPrioritizedCaseObservations[index] ||
			definition.EPL != contextInitTermPrioritizedCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", contextInitTermPrioritizedID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", contextInitTermPrioritizedID, err)
	}
	offset := 0
	for _, caseName := range contextInitTermPrioritizedCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", contextInitTermPrioritizedID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextInitTermPrioritizedID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", contextInitTermPrioritizedID, offset, caseName)
		}
		// The case marker is a step too: run it through the field whitelist
		// so an unexpected field on the marker is rejected like any other
		// step's extra field.
		if _, err := contextInitTermPrioritizedStepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextInitTermPrioritizedID, offset, err)
		}
		offset++
		want, ok := contextInitTermPrioritizedCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", contextInitTermPrioritizedID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", contextInitTermPrioritizedID, caseName)
		}
		for _, pinned := range want {
			key, err := contextInitTermPrioritizedStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextInitTermPrioritizedID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", contextInitTermPrioritizedID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", contextInitTermPrioritizedID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextInitTermPrioritizedID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// contextInitTermPrioritizedStepKey renders one raw step as its pinned
// key: op|statement|eventType|at. Unknown fields on the step object are
// rejected; send payloads are restricted to the registered event type's
// asserted fields.
func contextInitTermPrioritizedStepKey(raw json.RawMessage) (string, error) {
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
		ExpectErr string          `json:"expectError"`
		At        string          `json:"at"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string][]string{
		"case":         {"op", "case"},
		"advance-time": {"op", "case", "at"},
		"deploy":       {"op", "case", "statement", "epl"},
		"send":         {"op", "case", "eventType", "payload"},
		"build-error":  {"op", "case", "statement", "epl", "expectError"},
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
			"SupportBean":           {"theString", "intPrimitive"},
			"SupportProductIdEvent": {"productID"},
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

func requireContextInitTermPrioritizedFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", contextInitTermPrioritizedID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", contextInitTermPrioritizedID, name)
		}
	}
	return nil
}

func validateContextInitTermPrioritizedStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
