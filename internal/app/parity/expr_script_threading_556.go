package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// expr_script_threading_556.go replays two Java surfaces as the Draft 4.556
// differential chain (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c):
//
//   - script-probes bundles EPLScriptExpression ord 1 EPLScriptQuoteEscape
//     (java-runtime-950fed16f3830d3a9926) and ord 4
//     EPLScriptInvalidRegardlessDialect (java-runtime-416f111d2368882ea644) on
//     one scenario case: both are compile-only probes with no listener
//     surface, and the pinned per-step runtimeId keeps each probe on the
//     runtime the Java execution would own. Ord 1's two compileDeploy probes
//     (script bodies whose single-line and multi-line comments contain
//     I'am...) are recorded as compile-ok notes: DefineScript/RegisterScript
//     take provider callbacks rather than EPL body text, so the quote-escape
//     lexing has no Go boundary — the probe pins the Java acceptance note.
//     Ord 4's eight tryInvalidCompile probes are classified per Go boundary:
//     compile-error records pin the asserted Java prefix where Go rejects
//     (unknown-script via ErrorUnknownName at Build, arity-mismatch via the
//     ScriptArgumentTypes arity check, same-arity-overload and
//     param-name-overlap via the one-definition-per-name script registry);
//     unrepresentable records pin the Java prefix where the surface has no
//     Go counterpart (param-defined-twice — ScriptArgumentTypes carries
//     types, not names; unresolvable-return-type — Go return types are
//     reflect.Type values, never unresolvable names); intentionally-different
//     records pin the Java prefix where Go accepts the equivalent
//     registration (invalid-dialect — dialect is descriptive plan metadata
//     with no JSR-223 resolution; script-expression-overlap — the Go script
//     and declared-expression registries are separate name spaces).
//   - large-threading mirrors ExprFilterLargeThreading ord 0
//     (java-runtime-17b5c153a73414b75e24): @name('s0') select * from
//     pattern[a=SupportBean -> every event1=SupportTradeEvent(userId like
//     '123%')] deploys s0, a SupportBean() arm, TradeEvent(1,null,1001) does
//     not fire (null userId like-fails) and TradeEvent(2,'1234',1001) fires
//     once with event1.id=2.
//
// Approved differences (observably identical to the Java EPL):
//   - The Java oracle compiles/deploys module text; the Go runner builds the
//     equivalent fluent plan. The byte-exact EPL strings remain pinned on the
//     scenario steps and re-verified by both sides.
//   - Pattern tags surface as Maps under `select *`: the Go select projects
//     each tag through efoTagMap so the listener rows carry the same plain
//     property maps the Java oracle records (in-and-between precedent).
//   - The bare `a=SupportBean` leg maps to PatternFrom with Literal(true):
//     an unfiltered EPL event match accepts every SupportBean.
//   - The threading send payloads carry expectedFire so both hosts verify
//     the Java assertListenerNotInvoked/assertEqualsNew assertions
//     in-process; a contradicting fire is a replay error, not a trace row.

const exprScriptThreading556ID = "expr-script-threading-556"
const exprScriptThreading556JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
const exprScriptThreading556Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/script/EPLScriptExpression.java"
const exprScriptThreading556ThreadingSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/ExprFilterLargeThreading.java"

const exprScriptThreading556Description = "EPLScriptExpression compile probes + ExprFilterLargeThreading (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c): script-probes bundles ord 1 EPLScriptQuoteEscape (two compileDeploy quote-escape probes over single-line and multi-line I'am comments) and ord 4 EPLScriptInvalidRegardlessDialect (eight tryInvalidCompile probes — parameter defined twice, invalid dialect, unknown script, arity mismatch, same-arity overload, parameter-name overlap, script/expression overlap and unresolvable return type); large-threading (ord 0) deploys pattern[a=SupportBean -> every event1=SupportTradeEvent(userId like '123%')] and one SupportBean arm + TradeEvent(1,null,1001) no-fire + TradeEvent(2,'1234',1001) send emits event1.id=2"

var exprScriptThreading556JavaRuntimeIDs = []string{
	"java-runtime-950fed16f3830d3a9926",
	"java-runtime-416f111d2368882ea644",
	"java-runtime-17b5c153a73414b75e24",
}
var exprScriptThreading556JavaExecutions = []string{
	"EPLScriptQuoteEscape",
	"EPLScriptInvalidRegardlessDialect",
	"ExprFilterLargeThreading",
}
var exprScriptThreading556JavaStaticIDs = []string{
	"java-25975b631283eca66de6",
	"java-25975b631283eca66de6",
	"java-a977dd316de9d77c011c",
}
var exprScriptThreading556JavaSources = []string{
	exprScriptThreading556Source,
	exprScriptThreading556ThreadingSource,
}
var exprScriptThreading556Cases = []string{"script-probes", "large-threading"}

var exprScriptThreading556CaseOrdinals = map[string][]int{
	"script-probes":   {1, 4},
	"large-threading": {0},
}
var exprScriptThreading556CaseRuntimeIDs = map[string][]string{
	"script-probes":   {exprScriptThreading556JavaRuntimeIDs[0], exprScriptThreading556JavaRuntimeIDs[1]},
	"large-threading": {exprScriptThreading556JavaRuntimeIDs[2]},
}
var exprScriptThreading556CaseExecutionNames = map[string][]string{
	"script-probes":   {"EPLScriptQuoteEscape", "EPLScriptInvalidRegardlessDialect"},
	"large-threading": {"ExprFilterLargeThreading"},
}
var exprScriptThreading556CaseStaticIDs = map[string][]string{
	"script-probes":   {"java-25975b631283eca66de6", "java-25975b631283eca66de6"},
	"large-threading": {"java-a977dd316de9d77c011c"},
}
var exprScriptThreading556CaseSources = map[string]string{
	"script-probes":   exprScriptThreading556Source,
	"large-threading": exprScriptThreading556ThreadingSource,
}
var exprScriptThreading556CaseObservations = map[string]string{
	"script-probes":   "compile-ok+compile-error+unrepresentable+intentionally-different; the two quote-escape compileDeploy probes (ord 1) and the eight tryInvalidCompile probes (ord 4) pin the asserted Java messages — Go rejects the unregistered/arity/duplicate-name probes at Build, the named-parameter and named-return-type probes have no Go surface, and Go accepts the descriptive 'dummy' dialect plus the separate script/expression name spaces",
	"large-threading": "deployed+types+listener; pattern[a=SupportBean -> every event1=SupportTradeEvent(userId like '123%')] arms on SupportBean(), ignores TradeEvent(1,null,1001) (null userId like-fails) and fires once on TradeEvent(2,'1234',1001) with event1.id=2",
}

// Byte-exact EPL transcription: EPLScriptExpression.java lines 139-150
// (ord 1) and 229-261 (ord 4), ExprFilterLargeThreading.java line 28.
const exprScriptThreading556SLComment = "create expression f(params)[\n" +
	"  // I'am...\n" +
	"];"
const exprScriptThreading556MLComment = "create expression g(params)[\n" +
	"  /* I'am... */" +
	"];"
const exprScriptThreading556ThreadingEPL = "@name('s0') select * from pattern[a=SupportBean -> every event1=SupportTradeEvent(userId like '123%')]"

// exprScriptThreading556Probe pins one ord-4 tryInvalidCompile probe: the
// byte-exact EPL, the asserted Java message prefix and the emitted record
// operation (the Go-boundary classification).
type exprScriptThreading556Probe struct {
	Label   string
	Epl     string
	Message string
	Record  string
}

var exprScriptThreading556Probes = []exprScriptThreading556Probe{
	{"param-defined-twice",
		"expression js:abc(p1, p1) [/* text */] select * from SupportBean",
		"Invalid script parameters for script 'abc', parameter 'p1' is defined more then once [expression js:abc(p1, p1) [/* text */] select * from SupportBean]",
		"unrepresentable"},
	{"invalid-dialect",
		"expression dummy:abc() [10] select * from SupportBean",
		"Failed to obtain script runtime for dialect 'dummy' for script 'abc' [expression dummy:abc() [10] select * from SupportBean]",
		"intentionally-different"},
	{"unknown-script",
		"select abc() from SupportBean",
		"Failed to validate select-clause expression 'abc()': Unknown single-row function, expression declaration, script or aggregation function named 'abc' could not be resolved [select abc() from SupportBean]",
		"compile-error"},
	{"arity-mismatch",
		"expression js:abc() [10] select abc(1) from SupportBean",
		"Failed to validate select-clause expression 'abc(1)': Invalid number of parameters for script 'abc', expected 0 parameters but received 1 parameters [expression js:abc() [10] select abc(1) from SupportBean]",
		"compile-error"},
	{"same-arity-overload",
		"expression js:abc() [10] expression js:abc() [10] select abc() from SupportBean",
		"Script name 'abc' has already been defined with the same number of parameters [expression js:abc() [10] expression js:abc() [10] select abc() from SupportBean]",
		"compile-error"},
	{"param-name-overlap",
		"expression js:abc(p1) [10] expression js:abc(p2) [10] select abc() from SupportBean",
		"Script name 'abc' has already been defined with the same number of parameters [expression js:abc(p1) [10] expression js:abc(p2) [10] select abc() from SupportBean]",
		"compile-error"},
	{"script-expression-overlap",
		"expression js:abc() [10] expression abc {10} select abc() from SupportBean",
		"Script name 'abc' overlaps with another expression of the same name [expression js:abc() [10] expression abc {10} select abc() from SupportBean]",
		"intentionally-different"},
	{"unresolvable-return-type",
		"expression dummy js:abc() [10] select abc() from SupportBean",
		"Failed to validate select-clause expression 'abc()': Failed to resolve return type 'dummy' specified for script 'abc' [expression dummy js:abc() [10] select abc() from SupportBean]",
		"unrepresentable"},
}

// exprScriptThreading556CompileOKNotes pins the note each quote-escape
// compile-ok record carries; identical wording on both sides of the diff.
var exprScriptThreading556CompileOKNotes = map[string]string{
	"sl-comment": "compile-ok: Java compileDeploy accepts the script body whose single-line comment text contains an apostrophe (// I'am...); the Go script surface registers provider callbacks rather than EPL bodies, so the lexing detail has no Go boundary",
	"ml-comment": "compile-ok: Java compileDeploy accepts the script body whose multi-line comment text contains an apostrophe (/* I'am... */); the Go script surface registers provider callbacks rather than EPL bodies, so the lexing detail has no Go boundary",
}

var exprScriptThreading556QuoteEscapeEPLs = map[string]string{
	"sl-comment": exprScriptThreading556SLComment,
	"ml-comment": exprScriptThreading556MLComment,
}

// exprScriptThreading556ProbeByLabel indexes the pinned ord-4 probe table.
func exprScriptThreading556ProbeByLabel(label string) (exprScriptThreading556Probe, bool) {
	for _, probe := range exprScriptThreading556Probes {
		if probe.Label == label {
			return probe, true
		}
	}
	return exprScriptThreading556Probe{}, false
}

// exprScriptThreading556ProbeStepOps pins the step op each probe carries:
// unrepresentable records ride unrepresentable steps; compile-error and
// intentionally-different records ride build-error steps (the step op names
// the Java-side expectation — the compile must fail — while the record op
// carries the Go-boundary classification).
func exprScriptThreading556ProbeStepOps() map[string]string {
	ops := make(map[string]string, len(exprScriptThreading556Probes))
	for _, probe := range exprScriptThreading556Probes {
		if probe.Record == "compile-error" {
			ops[probe.Label] = "build-error"
		} else {
			ops[probe.Label] = "unrepresentable"
		}
	}
	return ops
}

// exprScriptThreading556CaseEPLs pins the per-case module text: the
// script-probes case concatenates the two ord-1 modules with the eight
// ord-4 probe modules in source order.
var exprScriptThreading556CaseEPLs = map[string]string{
	"script-probes": exprScriptThreading556SLComment + "\n" + exprScriptThreading556MLComment + "\n" +
		exprScriptThreading556Probes[0].Epl + "\n" +
		exprScriptThreading556Probes[1].Epl + "\n" +
		exprScriptThreading556Probes[2].Epl + "\n" +
		exprScriptThreading556Probes[3].Epl + "\n" +
		exprScriptThreading556Probes[4].Epl + "\n" +
		exprScriptThreading556Probes[5].Epl + "\n" +
		exprScriptThreading556Probes[6].Epl + "\n" +
		exprScriptThreading556Probes[7].Epl,
	"large-threading": exprScriptThreading556ThreadingEPL,
}

// exprScriptThreading556CaseState carries the per-case replay state.
type exprScriptThreading556CaseState struct {
	caseName       string
	env            *esper.Environment
	engine         *esper.Engine
	trace          *compat.Trace
	deployments    map[string]*esper.Deployment
	deployOrder    []string
	statements     map[string]*esper.Statement
	plans          map[string]esper.Plan
	listenerSeq    uint64
	listenerFired  bool
	deployedLabels map[string]bool
}

// exprScriptThreading556Provider is the script provider the probe
// registrations share; its body is never invoked (every probe is
// compile-only).
func exprScriptThreading556Provider(_ esper.ScriptContext) (esper.Value, error) {
	return esper.Present(int64(10)), nil
}

var exprScriptThreading556IntType = reflect.TypeOf(int64(0))

func runExprScriptThreading556Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateExprScriptThreading556Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeExprScriptThreading556(ctx, scenario, &trace)
}

// executeExprScriptThreading556 replays the scenario: each case runs on a
// fresh environment/engine pair and every step dispatches to the matching
// runtime action. Script probes never touch the engine — they run against
// per-probe environments so each probe sees a clean registry, matching the
// per-module script scope of the Java executions.
func executeExprScriptThreading556(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *exprScriptThreading556CaseState
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *trace, err
		}
		if step.Op != "case" && state == nil {
			return *trace, fmt.Errorf("%s: step %q arrives before any case marker", exprScriptThreading556ID, step.Op)
		}
		switch step.Op {
		case "case":
			if state != nil && state.engine != nil {
				_ = state.engine.Close(context.Background())
			}
			var err error
			state, err = startExprScriptThreading556Case(step.Case, trace)
			if err != nil {
				return *trace, err
			}
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *trace, err
			}
		case "deployed":
			if err := state.deployed(step); err != nil {
				return *trace, err
			}
		case "types":
			if err := state.types(step); err != nil {
				return *trace, err
			}
		case "send":
			if err := state.send(ctx, step); err != nil {
				return *trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *trace, err
			}
		case "unrepresentable":
			if err := state.unrepresentable(step); err != nil {
				return *trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", exprScriptThreading556ID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startExprScriptThreading556Case builds the fresh per-case environment:
// the SupportBean/SupportTradeEvent types the threading deploy needs plus
// the engine pinned to the case's Java runtime id at the epoch start time.
func startExprScriptThreading556Case(caseName string, trace *compat.Trace) (*exprScriptThreading556CaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[efabBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[efeTradeBean](env, "SupportTradeEvent"); err != nil {
		return nil, err
	}
	state := &exprScriptThreading556CaseState{
		caseName:       caseName,
		env:            env,
		deployments:    map[string]*esper.Deployment{},
		statements:     map[string]*esper.Statement{},
		plans:          map[string]esper.Plan{},
		deployedLabels: map[string]bool{},
		trace:          trace,
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(exprScriptThreading556CaseRuntimeIDs[caseName][len(exprScriptThreading556CaseRuntimeIDs[caseName])-1]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

// deploy handles the two deploy shapes: a script-probes deploy step is an
// ord-1 quote-escape compileDeploy probe recorded as compile-ok (there is
// no Go EPL-text boundary, so the pinned note is the whole record); the
// threading deploy step builds the pattern query and attaches the trace
// listener like env.compileDeploy(epl).addListener("s0").
func (s *exprScriptThreading556CaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	if s.caseName == "script-probes" {
		epl, ok := exprScriptThreading556QuoteEscapeEPLs[label]
		if !ok || step.Epl != epl {
			return fmt.Errorf("%s: compile probe %q carries an unpinned EPL %q", exprScriptThreading556ID, label, step.Epl)
		}
		s.trace.Records = append(s.trace.Records, compat.TraceRecord{
			Case:      s.caseName,
			Operation: "compile-ok",
			Statement: label,
			Sequence:  0,
			Value:     exprScriptThreading556CompileOKNotes[label],
		})
		return nil
	}
	if s.caseName != "large-threading" || label != "s0" || step.Epl != exprScriptThreading556ThreadingEPL {
		return fmt.Errorf("%s: case %q deploy %q is not pinned", exprScriptThreading556ID, s.caseName, label)
	}
	bean := esper.From[efabBean](s.env, "SupportBean")
	trade := esper.From[efeTradeBean](s.env, "SupportTradeEvent")
	userID := esper.Field[efeTradeBean, *string]("userId")
	pattern := esper.PatternFrom(bean, "a", esper.Literal(true)).
		Then(esper.PatternFrom(trade, "event1",
			esper.LikeOf(userID, esper.Literal("123%"))).Every())
	plan, err := s.env.Build(pattern.Select(
		esper.Alias("a", efoTagMap(esper.PatternEvent("a"))),
		esper.Alias("event1", efoTagMap(esper.PatternEvent("event1"))),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return fmt.Errorf("%s: deploy s0: %w", exprScriptThreading556ID, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy s0: %w", exprScriptThreading556ID, err)
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return fmt.Errorf("%s: deploy s0 produced %d statements, want 1", exprScriptThreading556ID, len(statements))
	}
	s.deployedLabels[label] = true
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	s.statements[label] = statements[0]
	s.plans[label] = plan
	statement := statements[0]
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		s.listenerFired = true
		s.listenerSeq++
		s.trace.Records = append(s.trace.Records, compat.TraceRecord{
			Case:      s.caseName,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  s.listenerSeq,
			Time:      compat.FormatTraceTime(s.engine.Now()),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// deployed emits the deployed marker for the threading s0 statement,
// mirroring the oracle's per-statement marker.
func (s *exprScriptThreading556CaseState) deployed(step compat.Step) error {
	if s.caseName != "large-threading" || step.Statement != "s0" || !s.deployedLabels[step.Statement] {
		return fmt.Errorf("%s: deployed marker for unknown statement %q", exprScriptThreading556ID, step.Statement)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "deployed",
		Statement: step.Statement,
		Sequence:  1,
		Time:      compat.FormatTraceTime(s.engine.Now()),
	})
	return nil
}

// types pins the asserted select-clause surface of the threading s0
// statement: both pattern tags surface as Map-typed properties under
// select *. The Go result schema projects each tag through efoTagMap,
// so the schema check verifies the equivalent map[string]any shape.
func (s *exprScriptThreading556CaseState) types(step compat.Step) error {
	if s.caseName != "large-threading" || step.Statement != "s0" {
		return fmt.Errorf("%s: unknown types statement %q", exprScriptThreading556ID, step.Statement)
	}
	plan, ok := s.plans[step.Statement]
	if !ok {
		return fmt.Errorf("%s: types statement %q was not deployed", exprScriptThreading556ID, step.Statement)
	}
	schema, ok := plan.ResultSchema()
	if !ok {
		return fmt.Errorf("%s: statement %q has no result schema", exprScriptThreading556ID, step.Statement)
	}
	for _, name := range []string{"a", "event1"} {
		field, exists := schema.Field(name)
		if !exists || field.Type != reflect.TypeOf(map[string]any{}) {
			return fmt.Errorf("%s: s0 %s drift: %v", exprScriptThreading556ID, name, field.Type)
		}
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "types",
		Statement: step.Statement,
		Sequence:  0,
		Time:      compat.FormatTraceTime(s.engine.Now()),
		Value: map[string]any{
			"properties": map[string]any{"a": "Map", "event1": "Map"},
		},
	})
	return nil
}

// send mirrors the threading event stream: SupportBean() arms the pattern,
// TradeEvent(1,null,1001) must not fire and TradeEvent(2,'1234',1001) must
// fire once. The payload's expectedFire flag is verified against the
// listener's invocation in-process, mirroring the Java oracle's
// assertListenerNotInvoked/assertEqualsNew assertions.
func (s *exprScriptThreading556CaseState) send(ctx context.Context, step compat.Step) error {
	if s.caseName != "large-threading" || step.Statement != "s0" {
		return fmt.Errorf("%s: case %q send %q is not pinned", exprScriptThreading556ID, s.caseName, step.Statement)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return fmt.Errorf("%s: decode %s payload: %w", exprScriptThreading556ID, step.EventType, err)
	}
	expectedFire, err := exprScriptThreading556ExpectedFire(fields)
	if err != nil {
		return err
	}
	var event any
	switch step.EventType {
	case "SupportBean":
		// Java's `new SupportBean()` leaves charPrimitive at '\u0000' and
		// every boxed/string column null; no other payload keys are pinned.
		if len(fields) != 1 {
			return fmt.Errorf("%s: SupportBean payload has unexpected fields", exprScriptThreading556ID)
		}
		event = efabBean{CharPrimitive: "\u0000"}
	case "SupportTradeEvent":
		if len(fields) != 4 {
			return fmt.Errorf("%s: SupportTradeEvent payload has unexpected fields", exprScriptThreading556ID)
		}
		var id, amount int32
		for name, target := range map[string]*int32{"id": &id, "amount": &amount} {
			raw, ok := fields[name]
			if !ok {
				return fmt.Errorf("%s: SupportTradeEvent payload is missing %s", exprScriptThreading556ID, name)
			}
			if err := json.Unmarshal(raw, target); err != nil {
				return fmt.Errorf("%s: decode %s.%s: %w", exprScriptThreading556ID, step.EventType, name, err)
			}
		}
		var userID *string
		raw, ok := fields["userId"]
		if !ok {
			return fmt.Errorf("%s: SupportTradeEvent payload is missing userId", exprScriptThreading556ID)
		}
		if err := json.Unmarshal(raw, &userID); err != nil {
			return fmt.Errorf("%s: decode SupportTradeEvent.userId: %w", exprScriptThreading556ID, err)
		}
		event = efeTradeBean{ID: id, UserID: userID, Amount: amount}
	default:
		return fmt.Errorf("%s: unknown event type %q", exprScriptThreading556ID, step.EventType)
	}
	s.listenerFired = false
	if err := s.engine.Send(ctx, step.EventType, event); err != nil {
		return fmt.Errorf("%s: send: %w", exprScriptThreading556ID, err)
	}
	if s.listenerFired != expectedFire {
		return fmt.Errorf("%s: send %s fired=%t contradicts expectedFire=%t", exprScriptThreading556ID, step.EventType, s.listenerFired, expectedFire)
	}
	return nil
}

// exprScriptThreading556ExpectedFire extracts the pinned expectedFire flag
// every threading send payload carries.
func exprScriptThreading556ExpectedFire(fields map[string]json.RawMessage) (bool, error) {
	raw, ok := fields["expectedFire"]
	if !ok {
		return false, fmt.Errorf("%s: send payload is missing expectedFire", exprScriptThreading556ID)
	}
	var expected bool
	if err := json.Unmarshal(raw, &expected); err != nil {
		return false, fmt.Errorf("%s: decode expectedFire: %w", exprScriptThreading556ID, err)
	}
	return expected, nil
}

// buildError runs one ord-4 compile-error probe against the typed
// equivalent of the pinned EPL: the Go boundary must reject before the
// pinned Java prefix is recorded.
func (s *exprScriptThreading556CaseState) buildError(step compat.Step) error {
	if s.caseName != "script-probes" {
		return fmt.Errorf("%s: case %q has no build-error steps", exprScriptThreading556ID, s.caseName)
	}
	probe, err := exprScriptThreading556PinnedProbe(step, "compile-error")
	if err != nil {
		return err
	}
	buildErr, want := exprScriptThreading556ProbeBoundary(step.Statement)
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", exprScriptThreading556ID, step.Statement)
	}
	if want != "" && !strings.Contains(buildErr.Error(), want) {
		return fmt.Errorf("%s: build-error probe %q drift: got %v", exprScriptThreading556ID, step.Statement, buildErr)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: probe.Record,
		Statement: step.Statement,
		Sequence:  0,
		Value:     probe.Message,
	})
	return nil
}

// unrepresentable handles the ord-4 probes the typed surface cannot express
// or accepts where Java rejects: the named-parameter and named-return-type
// probes record unrepresentable; the descriptive-dialect and
// script/expression-overlap probes verify the Go build accepts the
// equivalent before recording intentionally-different. Both pin the
// asserted Java prefix.
func (s *exprScriptThreading556CaseState) unrepresentable(step compat.Step) error {
	if s.caseName != "script-probes" {
		return fmt.Errorf("%s: case %q has no unrepresentable steps", exprScriptThreading556ID, s.caseName)
	}
	probe, err := exprScriptThreading556PinnedProbe(step, "")
	if err != nil {
		return err
	}
	if probe.Record == "intentionally-different" {
		if acceptErr := exprScriptThreading556ProbeAccepted(step.Statement); acceptErr != nil {
			return fmt.Errorf("%s: intentionally-different probe %q build drift: %w", exprScriptThreading556ID, step.Statement, acceptErr)
		}
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: probe.Record,
		Statement: step.Statement,
		Sequence:  0,
		Value:     probe.Message,
	})
	return nil
}

// exprScriptThreading556PinnedProbe verifies a script-probes step against
// the pinned ord-4 table and returns its probe row.
func exprScriptThreading556PinnedProbe(step compat.Step, record string) (exprScriptThreading556Probe, error) {
	probe, ok := exprScriptThreading556ProbeByLabel(step.Statement)
	if !ok {
		return probe, fmt.Errorf("%s: unknown invalid probe %q", exprScriptThreading556ID, step.Statement)
	}
	if record != "" && probe.Record != record {
		return probe, fmt.Errorf("%s: probe %q rides the wrong step op", exprScriptThreading556ID, step.Statement)
	}
	if step.Epl != probe.Epl || step.ExpectError != probe.Message {
		return probe, fmt.Errorf("%s: invalid probe %q carries unpinned fields", exprScriptThreading556ID, step.Statement)
	}
	return probe, nil
}

// exprScriptThreading556ProbeEnv returns a clean probe environment: each
// ord-4 probe sees an empty registry exactly like the Java module scope.
func exprScriptThreading556ProbeEnv() (*esper.Environment, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[efabBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	return env, nil
}

// exprScriptThreading556WildSelect builds `select <exprs> from SupportBean`
// against a probe environment, the fluent form of the probes' trailing
// select clause.
func exprScriptThreading556WildSelect(env *esper.Environment, selections ...esper.Selection) error {
	_, err := env.Build(esper.Select(esper.From[efabBean](env, "SupportBean"), selections...).
		Query(esper.StatementName("s0")))
	return err
}

// exprScriptThreading556ProbeBoundary runs the Go boundary for the probes
// whose step op is build-error, returning the construction error and the
// wording the error must carry.
func exprScriptThreading556ProbeBoundary(label string) (error, string) {
	env, err := exprScriptThreading556ProbeEnv()
	if err != nil {
		return err, ""
	}
	switch label {
	case "unknown-script":
		// `select abc() from SupportBean` — the unregistered script call
		// fails name resolution during Build.
		return exprScriptThreading556WildSelect(env,
			esper.Alias("c0", esper.ScriptCall[int64](env, "abc"))), "script \"abc\" is not registered"
	case "arity-mismatch":
		// `expression js:abc() [10] select abc(1)` — ScriptArgumentTypes()
		// declares arity 0 so the one-argument call fails during Build.
		if err := esper.RegisterValueScript(env, "abc", "js", nil,
			exprScriptThreading556Provider, esper.ScriptArgumentTypes()); err != nil {
			return err, ""
		}
		return exprScriptThreading556WildSelect(env,
				esper.Alias("c0", esper.ScriptCall[int64](env, "abc", esper.Literal(1)))),
			"expects 0 arguments, received 1"
	case "same-arity-overload":
		// `expression js:abc() [10] expression js:abc() [10]` — the
		// one-definition-per-name registry rejects the second registration
		// at the same arity.
		if err := esper.RegisterValueScript(env, "abc", "js", nil,
			exprScriptThreading556Provider, esper.ScriptArgumentTypes()); err != nil {
			return err, ""
		}
		return esper.RegisterValueScript(env, "abc", "js", nil,
				exprScriptThreading556Provider, esper.ScriptArgumentTypes()),
			"script by name 'abc (0 parameters)' has already been created"
	case "param-name-overlap":
		// `expression js:abc(p1) [10] expression js:abc(p2) [10]` — same
		// duplicate-name boundary at arity 1; Go script parameters carry
		// types, not names.
		if err := esper.RegisterValueScript(env, "abc", "js", nil,
			exprScriptThreading556Provider, esper.ScriptArgumentTypes(exprScriptThreading556IntType)); err != nil {
			return err, ""
		}
		return esper.RegisterValueScript(env, "abc", "js", nil,
				exprScriptThreading556Provider, esper.ScriptArgumentTypes(exprScriptThreading556IntType)),
			"script by name 'abc (1 parameters)' has already been created"
	default:
		return fmt.Errorf("%s: probe %q has no build boundary", exprScriptThreading556ID, label), ""
	}
}

// exprScriptThreading556ProbeAccepted runs the Go construction the
// intentionally-different probes pin: Java rejects while the equivalent Go
// registration succeeds, so a nil result proves the divergence.
func exprScriptThreading556ProbeAccepted(label string) error {
	env, err := exprScriptThreading556ProbeEnv()
	if err != nil {
		return err
	}
	switch label {
	case "invalid-dialect":
		// `expression dummy:abc() [10]` — the Go dialect string is
		// descriptive plan metadata; no JSR-223 resolution exists.
		if err := esper.RegisterValueScript(env, "abc", "dummy", nil,
			exprScriptThreading556Provider, esper.ScriptArgumentTypes()); err != nil {
			return err
		}
		return exprScriptThreading556WildSelect(env)
	case "script-expression-overlap":
		// `expression js:abc() [10] expression abc {10}` — the Go script
		// and declared-expression registries are separate name spaces.
		if err := esper.RegisterValueScript(env, "abc", "js", nil,
			exprScriptThreading556Provider, esper.ScriptArgumentTypes()); err != nil {
			return err
		}
		if err := env.DefineExpression("abc", esper.Literal(int64(10))); err != nil {
			return err
		}
		if err := exprScriptThreading556WildSelect(env,
			esper.Alias("c0", esper.ScriptCall[int64](env, "abc"))); err != nil {
			return err
		}
		return exprScriptThreading556WildSelect(env,
			esper.Alias("c1", esper.ExpressionRef[int64](env, "abc")))
	default:
		return fmt.Errorf("%s: probe %q has no intentionally-different boundary", exprScriptThreading556ID, label)
	}
}

// undeployAll mirrors env.undeployAll(): every deployment registered by the
// case's deploy steps is undeployed in deploy order.
func (s *exprScriptThreading556CaseState) undeployAll(ctx context.Context) error {
	for _, label := range s.deployOrder {
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", exprScriptThreading556ID, label, err)
		}
		delete(s.deployments, label)
	}
	s.deployOrder = nil
	s.deployedLabels = map[string]bool{}
	s.statements = map[string]*esper.Statement{}
	s.plans = map[string]esper.Plan{}
	return nil
}

// loadExprScriptThreading556Scenario decodes the scenario with the strict
// contract shared by the differential runners: no duplicate or unknown JSON
// fields, pinned metadata, pinned case identity and EPL, the pinned step
// schedule and a per-op step field whitelist.
func loadExprScriptThreading556Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprScriptThreading556ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprScriptThreading556ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprScriptThreading556ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprScriptThreading556ID, err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("%s scenario contains unexpected or missing fields", exprScriptThreading556ID)
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing field %q", exprScriptThreading556ID, name)
		}
	}
	for name, want := range map[string]string{
		"version":     compat.ScenarioVersion,
		"id":          exprScriptThreading556ID,
		"description": exprScriptThreading556Description,
		"javaCommit":  exprScriptThreading556JavaCommit,
		"javaSource":  exprScriptThreading556Source,
	} {
		var got string
		if err := json.Unmarshal(root[name], &got); err != nil || got != want {
			return compat.Scenario{}, fmt.Errorf("%s scenario field %q is not pinned", exprScriptThreading556ID, name)
		}
	}
	for name, want := range map[string][]string{
		"javaRuntimes":  exprScriptThreading556JavaRuntimeIDs,
		"javaNames":     exprScriptThreading556JavaExecutions,
		"javaStaticIds": exprScriptThreading556JavaStaticIDs,
		"javaFlags":     {},
	} {
		var got []string
		if err := json.Unmarshal(root[name], &got); err != nil || !equalStrings(got, want) {
			return compat.Scenario{}, fmt.Errorf("%s scenario field %q is not pinned", exprScriptThreading556ID, name)
		}
	}
	var cases []struct {
		Case           string   `json:"case"`
		Ordinals       []int    `json:"ordinals"`
		RuntimeIDs     []string `json:"runtimeIds"`
		ExecutionNames []string `json:"executionNames"`
		StaticIDs      []string `json:"staticIds"`
		Source         string   `json:"source"`
		Observation    string   `json:"observation"`
		Epl            string   `json:"epl"`
	}
	if err := json.Unmarshal(root["cases"], &cases); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s cases: %w", exprScriptThreading556ID, err)
	}
	if len(cases) != len(exprScriptThreading556Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", exprScriptThreading556ID, len(exprScriptThreading556Cases))
	}
	for index, entry := range cases {
		if entry.Case != exprScriptThreading556Cases[index] ||
			!reflect.DeepEqual(entry.Ordinals, exprScriptThreading556CaseOrdinals[entry.Case]) ||
			!reflect.DeepEqual(entry.RuntimeIDs, exprScriptThreading556CaseRuntimeIDs[entry.Case]) ||
			!reflect.DeepEqual(entry.ExecutionNames, exprScriptThreading556CaseExecutionNames[entry.Case]) ||
			!reflect.DeepEqual(entry.StaticIDs, exprScriptThreading556CaseStaticIDs[entry.Case]) ||
			entry.Source != exprScriptThreading556CaseSources[entry.Case] ||
			entry.Observation != exprScriptThreading556CaseObservations[entry.Case] ||
			entry.Epl != exprScriptThreading556CaseEPLs[entry.Case] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprScriptThreading556ID, index)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", exprScriptThreading556ID, err)
	}
	expected := exprScriptThreading556StepKeys()
	if len(rawSteps) != len(expected) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps, found %d", exprScriptThreading556ID, len(expected), len(rawSteps))
	}
	for index, rawStep := range rawSteps {
		key, err := exprScriptThreading556StepKey(rawStep)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprScriptThreading556ID, index, err)
		}
		if key != expected[index] {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not pinned", exprScriptThreading556ID, index)
		}
	}
	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprScriptThreading556ID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// exprScriptThreading556StepFields pins the field whitelist per step op.
var exprScriptThreading556StepFields = map[string][]string{
	"case":            {"op", "case"},
	"deploy":          {"op", "case", "statement", "runtimeId", "epl"},
	"deployed":        {"op", "case", "statement", "runtimeId"},
	"types":           {"op", "case", "statement", "runtimeId"},
	"send":            {"op", "case", "statement", "runtimeId", "eventType", "payload"},
	"build-error":     {"op", "case", "statement", "runtimeId", "epl", "expectError"},
	"unrepresentable": {"op", "case", "statement", "runtimeId", "epl", "expectError"},
	"undeploy-all":    {"op", "case", "runtimeId"},
}

// exprScriptThreading556StepKey renders one raw step as its pinned key:
// op|case|statement|runtimeId|epl|expectError|eventType|payload with the
// payload compacted. Unknown fields on the step object are rejected.
func exprScriptThreading556StepKey(raw json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return "", err
	}
	var op string
	if err := json.Unmarshal(object["op"], &op); err != nil {
		return "", fmt.Errorf("step op: %w", err)
	}
	allowed, ok := exprScriptThreading556StepFields[op]
	if !ok {
		return "", fmt.Errorf("unsupported op %q", op)
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
			return "", fmt.Errorf("unexpected field %q", field)
		}
	}
	values := make([]string, 0, 8)
	values = append(values, op)
	for _, name := range []string{"case", "statement", "runtimeId", "epl", "expectError", "eventType"} {
		value := ""
		if raw, ok := object[name]; ok {
			if err := json.Unmarshal(raw, &value); err != nil {
				return "", fmt.Errorf("step %s: %w", name, err)
			}
		}
		values = append(values, value)
	}
	payload := ""
	if raw, ok := object["payload"]; ok {
		var buffer bytes.Buffer
		if err := json.Compact(&buffer, raw); err != nil {
			return "", fmt.Errorf("step payload: %w", err)
		}
		payload = buffer.String()
	}
	values = append(values, payload)
	return strings.Join(values, "|"), nil
}

// exprScriptThreading556StepKeys pins the complete step sequence: the
// script-probes case marker, the two ord-1 deploy probes, the eight ord-4
// probes in source order and undeploy-all; then the threading case marker,
// deploy/deployed/types, the three sends and undeploy-all.
func exprScriptThreading556StepKeys() []string {
	rtQuote := exprScriptThreading556JavaRuntimeIDs[0]
	rtInvalid := exprScriptThreading556JavaRuntimeIDs[1]
	rtThreading := exprScriptThreading556JavaRuntimeIDs[2]
	keys := []string{
		"case|script-probes||||||",
		"deploy|script-probes|sl-comment|" + rtQuote + "|" + exprScriptThreading556SLComment + "|||",
		"deploy|script-probes|ml-comment|" + rtQuote + "|" + exprScriptThreading556MLComment + "|||",
	}
	stepOps := exprScriptThreading556ProbeStepOps()
	for _, probe := range exprScriptThreading556Probes {
		keys = append(keys, stepOps[probe.Label]+"|script-probes|"+probe.Label+"|"+
			rtInvalid+"|"+probe.Epl+"|"+probe.Message+"||")
	}
	keys = append(keys,
		"undeploy-all|script-probes||"+rtQuote+"||||",
		"case|large-threading||||||",
		"deploy|large-threading|s0|"+rtThreading+"|"+exprScriptThreading556ThreadingEPL+"|||",
		"deployed|large-threading|s0|"+rtThreading+"||||",
		"types|large-threading|s0|"+rtThreading+"||||",
		"send|large-threading|s0|"+rtThreading+"|||SupportBean|{\"expectedFire\":false}",
		"send|large-threading|s0|"+rtThreading+"|||SupportTradeEvent|{\"id\":1,\"userId\":null,\"amount\":1001,\"expectedFire\":false}",
		"send|large-threading|s0|"+rtThreading+"|||SupportTradeEvent|{\"id\":2,\"userId\":\"1234\",\"amount\":1001,\"expectedFire\":true}",
		"undeploy-all|large-threading||"+rtThreading+"||||",
	)
	return keys
}

// validateExprScriptThreading556Scenario re-checks a decoded scenario (used
// when the runner receives a scenario decoded by the generic loader path).
func validateExprScriptThreading556Scenario(scenario compat.Scenario) error {
	if scenario.ID != exprScriptThreading556ID {
		return fmt.Errorf("%s: unexpected scenario id %q", exprScriptThreading556ID, scenario.ID)
	}
	if len(scenario.Steps) != len(exprScriptThreading556StepKeys()) {
		return fmt.Errorf("%s: expected %d steps, got %d", exprScriptThreading556ID, len(exprScriptThreading556StepKeys()), len(scenario.Steps))
	}
	return nil
}
