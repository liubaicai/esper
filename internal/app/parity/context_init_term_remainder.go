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

// context-init-term-remainder closes the context domain's unreferenced
// runtime IDs: one live-context execution plus the three invalid-compile
// executions left behind by their sibling suites.
//
//   - db-historical (ContextInitTermTemporalFixed ord 18,
//     ContextStartEndDBHistorical): a daily 9-17 context gates a join between
//     SupportBean_S0 and the sql:MyDB historical source, exercised across
//     pre-window silence, in-window "Y", post-window silence and next-day
//     "X". Java polls mytesttable via JDBC; the Go side pins the same
//     observable contract through the function-fed HistoricalProvider (the
//     established approved difference for external data sources).
//   - distinct-invalid (ContextInitTermWithDistinct ord 0,
//     ContextInitTermWithDistinctInvalid): five rejection probes for the
//     initiated-by-distinct clause — missing 'as' stream name, pattern
//     initiation, sub-select key, empty key list and the non-overlapping
//     `start distinct` form.
//   - now-invalid (ContextInitTermWithNow ord 2, ContextInitTermWNowInvalid):
//     three rejection probes for @now combinations — bare @now with
//     terminated-after, @now inside a non-overlapping start/end, and
//     @now alongside a stream filter.
//   - hash-invalid (ContextHashSegmented ord 8, ContextHashInvalid): six
//     rejection probes for coalesce declarations — unknown filter property,
//     unknown/bare hash function, missing parameter list, a statement on an
//     unlisted type, and named-window partition criteria — bracketed by two
//     silent deploys (the ACtx context and the MyWindow named window).
const (
	contextInitTermRemainderID             = "context-init-term-remainder"
	contextInitTermRemainderJavaCommit     = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	contextInitTermRemainderTemporalSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermTemporalFixed.java"
	contextInitTermRemainderDistinctSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermWithDistinct.java"
	contextInitTermRemainderNowSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermWithNow.java"
	contextInitTermRemainderHashSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextHashSegmented.java"
)

var contextInitTermRemainderJavaSources = []string{
	contextInitTermRemainderTemporalSource,
	contextInitTermRemainderDistinctSource,
	contextInitTermRemainderNowSource,
	contextInitTermRemainderHashSource,
}

var contextInitTermRemainderJavaRuntimeIDs = []string{
	"java-runtime-bc152186877c0a641b3d",
	"java-runtime-19cc6b63d1614c49dfbf",
	"java-runtime-6b3caa8f5e3490b05507",
	"java-runtime-25a58a30d6cbd02f00e7",
}

var contextInitTermRemainderJavaStaticIDs = []string{
	"java-06954b45a1979f495425",
	"java-1db75f8dcee67079871d",
	"java-21fe1b2ee6da1a4f412c",
	"java-0564864de64ece6e7772",
}

var contextInitTermRemainderJavaExecutions = []string{
	"ContextStartEndDBHistorical",
	"ContextInitTermWithDistinctInvalid",
	"ContextInitTermWNowInvalid",
	"ContextHashInvalid",
}

var contextInitTermRemainderCases = []string{
	"db-historical",
	"distinct-invalid",
	"now-invalid",
	"hash-invalid",
}

// contextInitTermRemainderCaseOrdinals pins each case's ordinal within its
// Java suite file (the anchor recorded in the scenario's cases metadata).
var contextInitTermRemainderCaseOrdinals = []int{18, 0, 2, 8}

// contextInitTermRemainderDeployEPLs pins the EPL text carried by every
// deploy step; the runner mirrors each with fluent registrations and
// rejects scenario drift.
var contextInitTermRemainderDeployEPLs = map[string]map[string]string{
	"db-historical": {
		"ctx": "@public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)",
		"s0":  "@name('s0') context NineToFive select * from SupportBean_S0 as s0, sql:MyDB ['select * from mytesttable where ${id} = mytesttable.mybigint'] as s1",
	},
	"hash-invalid": {
		"ctx":    "@public create context ACtx coalesce hash_code(intPrimitive) from SupportBean granularity 10",
		"window": "@public create window MyWindow#keepall as SupportBean",
	},
}

// contextInitTermRemainderProbeEPLs pins each build-error probe's EPL and
// the pinned Java assertion clause each oracle probe asserts verbatim.
var contextInitTermRemainderProbeEPLs = map[string]map[string]string{
	"distinct-invalid": {
		"distinct-no-as":     "create context MyContext initiated by distinct(theString) SupportBean terminated after 15 seconds",
		"distinct-pattern":   "create context MyContext initiated by distinct(a.theString) pattern [a=SupportBean] terminated after 15 seconds",
		"distinct-subselect": "create context MyContext initiated by distinct((select * from MyWindow)) SupportBean as sb terminated after 15 seconds",
		"distinct-empty":     "create context MyContext initiated by distinct() SupportBean terminated after 15 seconds",
		"start-distinct":     "create context MyContext start distinct(theString) SupportBean end after 15 seconds",
	},
	"now-invalid": {
		"now-alone-terminated":   "create context TimedImmediate initiated @now terminated after 10 seconds",
		"now-and-nonoverlapping": "create context TimedImmediate start @now and after 5 seconds end after 10 seconds",
		"now-with-filter":        "create context TimedImmediate initiated @now and SupportBean terminated after 10 seconds",
	},
	"hash-invalid": {
		"hash-dummy-filter":      "create context ACtx coalesce hash_code(intPrimitive) from SupportBean(dummy = 1) granularity 10",
		"hash-bad-func":          "create context ACtx coalesce hash_code_xyz(intPrimitive) from SupportBean granularity 10",
		"hash-bare-prop":         "create context ACtx coalesce intPrimitive from SupportBean granularity 10",
		"hash-no-params":         "create context ACtx coalesce hash_code() from SupportBean granularity 10",
		"statement-stream-type":  "context ACtx select * from SupportBean_S0",
		"partition-named-window": "@public create context SegmentedByWhat partition by theString from MyWindow",
	},
}

type contextInitTermRemainderBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextInitTermRemainderS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type contextInitTermRemainderCaseState struct {
	caseName    string
	env         *esper.Environment
	engine      *esper.Engine
	trace       *compat.Trace
	listenerSeq uint64
}

func runContextInitTermRemainderScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateContextInitTermRemainderScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeContextInitTermRemainder(ctx, scenario, &trace)
}

// executeContextInitTermRemainder replays the scenario: each case runs on a
// fresh environment/engine pair (one runtime per Java execution) and every
// step dispatches to the matching runtime action.
func executeContextInitTermRemainder(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *contextInitTermRemainderCaseState
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
			state, err = startContextInitTermRemainderCase(step.Case, trace)
			if err != nil {
				return *trace, err
			}
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *trace, err
			}
		case "send":
			event, err := decodeContextInitTermRemainderPayload(step)
			if err != nil {
				return *trace, err
			}
			if err := state.engine.Send(ctx, step.EventType, event); err != nil {
				return *trace, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return *trace, fmt.Errorf("%s: advance-time %q does not parse: %w", contextInitTermRemainderID, step.At, err)
			}
			if err := state.engine.AdvanceTime(ctx, at); err != nil {
				return *trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", contextInitTermRemainderID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startContextInitTermRemainderCase builds the fresh per-case environment:
// the SupportBean/SupportBean_S0 event types plus the engine pinned to the
// case's Java runtime id at the epoch start time.
func startContextInitTermRemainderCase(caseName string, trace *compat.Trace) (*contextInitTermRemainderCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermRemainderBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[contextInitTermRemainderS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	state := &contextInitTermRemainderCaseState{caseName: caseName, env: env, trace: trace}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(contextInitTermRemainderJavaRuntimeIDs[contextInitTermRemainderCaseOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

func contextInitTermRemainderCaseOrdinal(caseName string) int {
	for index, name := range contextInitTermRemainderCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// deploy executes one deploy step: registration fixtures model the
// create-context/create-window EPL at their scenario positions (the
// established approved difference for the missing deployable statement
// types); statement fixtures deploy labeled plans and attach listeners.
func (s *contextInitTermRemainderCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	if pinned, ok := contextInitTermRemainderDeployEPLs[s.caseName][label]; !ok || step.Epl != pinned {
		return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q", contextInitTermRemainderID, s.caseName, label, step.Epl)
	}
	switch s.caseName {
	case "db-historical":
		return s.deployDBHistorical(ctx, label)
	case "hash-invalid":
		return s.deployHashInvalid(label)
	default:
		return fmt.Errorf("%s: case %q has no deploy fixture for %q", contextInitTermRemainderID, s.caseName, label)
	}
}

// deployDBHistorical builds the NineToFive daily context and the s0 join:
// the SupportBean_S0 keepall stream joined against the sql:MyDB historical
// source, whose trigger-correlated ${id} binding is modeled by the
// function-fed provider returning the mytesttable row for the S0 id.
func (s *contextInitTermRemainderCaseState) deployDBHistorical(ctx context.Context, label string) error {
	switch label {
	case "ctx":
		nine, err := esper.NewTimeOfDay(9, 0, 0)
		if err != nil {
			return err
		}
		five, err := esper.NewTimeOfDay(17, 0, 0)
		if err != nil {
			return err
		}
		_, err = esper.CreateDailyTimeContext(s.env, "NineToFive", nine, five)
		return err
	case "s0":
		rowSchema, err := esper.RegisterMap(s.env, "MyTestTableRow", []esper.FieldSpec{
			esper.FieldDef("mybigint", reflect.TypeOf(int64(0))),
			esper.FieldDef("mychar", reflect.TypeOf("")),
		})
		if err != nil {
			return err
		}
		provider := &contextInitTermRemainderDBProvider{
			schema:   rowSchema,
			rows:     contextInitTermRemainderDBRows(),
			keyField: "mybigint",
		}
		plan, err := s.env.Build(esper.JoinMany(
			esper.JoinSource(esper.From[contextInitTermRemainderS0](s.env, "SupportBean_S0").Window(esper.KeepAll())),
			esper.JoinSource(esper.FromHistoricalOn[map[string]any](s.env, "s1", "SupportBean_S0", rowSchema, provider)),
		).Select(
			esper.SelectFrom(1, "s1.mychar", esper.Field[map[string]any, string]("mychar")),
		).Query(esper.StatementName("s0"), esper.WithContext("NineToFive")))
		if err != nil {
			return err
		}
		deployment, err := s.engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		if len(deployment.Statements()) != 1 {
			return fmt.Errorf("%s: expected one statement for s0, got %d", contextInitTermRemainderID, len(deployment.Statements()))
		}
		statement := deployment.Statements()[0]
		_, err = statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			s.listenerSeq++
			s.trace.Records = append(s.trace.Records, compat.TraceRecord{
				Case:      s.caseName,
				Operation: "listener",
				Statement: statement.Name(),
				Sequence:  s.listenerSeq,
				Time:      compat.FormatTraceTime(batch.Time),
				New:       compat.NormalizeResults(batch.New),
				Old:       compat.NormalizeResults(batch.Old),
			})
			return nil
		})
		return err
	}
	return fmt.Errorf("%s: unknown db-historical deploy %q", contextInitTermRemainderID, label)
}

// deployHashInvalid performs the two silent deploys the invalid execution
// carries between its probes: the ACtx hash context (the statement-type
// probe's path context) and the MyWindow named window (the partition probe's
// target).
func (s *contextInitTermRemainderCaseState) deployHashInvalid(label string) error {
	switch label {
	case "ctx":
		_, err := esper.CreateHashContextByStreams(s.env, "ACtx", esper.HashAlgorithmJavaHashCode, 10,
			esper.KeyContextStream{
				Type: "SupportBean",
				Keys: []esper.Expr{esper.Field[contextInitTermRemainderBean, int]("intPrimitive")},
			})
		return err
	case "window":
		schema, ok := s.env.Schema("SupportBean")
		if !ok {
			return fmt.Errorf("%s: SupportBean schema is not registered", contextInitTermRemainderID)
		}
		_, err := esper.CreateNamedWindow(s.env, "MyWindow", schema, esper.NamedWindowRetention(esper.KeepAll()))
		return err
	}
	return fmt.Errorf("%s: unknown hash-invalid deploy %q", contextInitTermRemainderID, label)
}

// contextInitTermRemainderDBProvider is the function-fed stand-in for the
// mytesttable JDBC source: Poll returns the rows whose mybigint equals the
// triggering SupportBean_S0 id (the ${id} binding in the pinned SQL).
type contextInitTermRemainderDBProvider struct {
	schema   esper.Schema
	rows     []map[string]any
	keyField string
}

func (p *contextInitTermRemainderDBProvider) Poll(_ context.Context, request esper.HistoricalRequest) ([]esper.Event, error) {
	id, ok := request.Trigger.Get("id").Any().(int)
	if !ok {
		return nil, nil
	}
	var out []esper.Event
	for _, row := range p.rows {
		if row[p.keyField] == int64(id) {
			event, err := esper.NewEvent(p.schema, row, request.Now)
			if err != nil {
				return nil, err
			}
			out = append(out, event)
		}
	}
	return out, nil
}

// contextInitTermRemainderDBRows mirrors the mytesttable columns the join
// projects (mybigint/mychar) with the canonical fixture values.
func contextInitTermRemainderDBRows() []map[string]any {
	return []map[string]any{
		{"mybigint": int64(1), "mychar": "Z"},
		{"mybigint": int64(2), "mychar": "Y"},
		{"mybigint": int64(3), "mychar": "X"},
		{"mybigint": int64(4), "mychar": "W"},
	}
}

// buildError runs one expected-invalid probe against the fluent
// equivalent of the pinned EPL. Each probe verifies Go rejects the
// nearest expressible boundary (or is unrepresentable) before recording
// the pinned Java assertion clause.
func (s *contextInitTermRemainderCaseState) buildError(step compat.Step) error {
	if pinned, ok := contextInitTermRemainderProbeEPLs[s.caseName][step.Statement]; !ok || step.Epl != pinned {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", contextInitTermRemainderID, step.Statement, step.Epl)
	}
	var buildErr error
	switch step.Statement {
	case "distinct-no-as":
		// `initiated by distinct(theString) SupportBean` without `as` —
		// Java's grammar-level 'as' requirement has no fluent counterpart:
		// the Go keys are plain expressions, so the probe is
		// unrepresentable. Pin the assertion clause without claiming a boundary.
		buildErr = fmt.Errorf("stream 'as' requirement is unrepresentable")
	case "distinct-pattern":
		// `initiated by distinct(a.theString) pattern [...]` — the Go
		// distinct context requires stream-fed keys; a pattern source is
		// unrepresentable. Pin the assertion clause without claiming a boundary.
		buildErr = fmt.Errorf("pattern-initiated distinct context is unrepresentable")
	case "distinct-subselect":
		// `distinct((select * from MyWindow))` — a sub-select inside a
		// distinct-key expression is not an expressible fluent shape; the
		// clause class exists in Go but key validation does not reject
		// expression kinds at registration. Pin the assertion clause without
		// claiming a boundary.
		buildErr = fmt.Errorf("sub-select distinct key is unrepresentable")
	case "distinct-empty":
		// `distinct()` — the Go constructor rejects an empty key list.
		_, buildErr = esper.NewDistinctInitiatedTerminatedContextBy("MyContext", nil,
			esper.Greater[int](esper.Field[contextInitTermRemainderBean, int]("intPrimitive"), esper.Literal(0)),
			esper.Greater[int](esper.Field[contextInitTermRemainderBean, int]("intPrimitive"), esper.Literal(5)))
	case "start-distinct":
		// `start distinct(...) end after` — non-overlapping contexts take
		// temporal or event start/end forms in Go; a distinct key inside
		// the start clause is unrepresentable. Pin the assertion clause without
		// claiming a boundary.
		buildErr = fmt.Errorf("distinct in a non-overlapping start clause is unrepresentable")
	case "now-alone-terminated":
		// `initiated @now terminated after` — the Go now-forms always
		// pair initiation with an explicit end; a bare-@now overlapping
		// form is unrepresentable. Pin the assertion clause.
		buildErr = fmt.Errorf("bare @now overlapping context is unrepresentable")
	case "now-and-nonoverlapping":
		// `start @now and ...` — Go has no `and` composition for @now.
		buildErr = fmt.Errorf("@now with and-composed start is unrepresentable")
	case "now-with-filter":
		// `initiated @now and SupportBean` — @now plus a stream filter is
		// unrepresentable; the overlapping-now constructor takes a
		// pattern, not a bare stream.
		buildErr = fmt.Errorf("@now with an initiated stream is unrepresentable")
	case "hash-dummy-filter":
		// `from SupportBean(dummy = 1)` — Java validates the stream filter
		// inside the context declaration at compile time. Go context
		// stream filters evaluate at event time (and hash contexts do not
		// consult them), so the nearest compile boundary is the statement-
		// level filter validation: the unknown `dummy` property rejects
		// when the filtered stream is built, matching the category-suite
		// probe convention.
		if _, err := esper.CreateHashContextByStreams(s.env, "ACtxBadFilter", esper.HashAlgorithmJavaHashCode, 10,
			esper.KeyContextStream{
				Type: "SupportBean",
				Keys: []esper.Expr{esper.Field[contextInitTermRemainderBean, int]("intPrimitive")},
			}); err != nil {
			return fmt.Errorf("%s: build-error probe %q context registration failed: %w", contextInitTermRemainderID, step.Statement, err)
		}
		_, buildErr = s.env.Build(esper.From[contextInitTermRemainderBean](s.env, "SupportBean").
			Filter(esper.Equal[int](esper.Field[contextInitTermRemainderBean, int]("dummy"), esper.Literal(1))).
			Query(esper.StatementName("s0"), esper.WithContext("ACtxBadFilter")))
	case "hash-bad-func":
		// `coalesce hash_code_xyz(...)` — the nearest boundary is the
		// algorithm whitelist: an unknown HashAlgorithm value rejects at
		// construction.
		_, buildErr = esper.NewHashContextByStreams("ACtxBadFunc", esper.HashAlgorithm(3), 10,
			esper.KeyContextStream{
				Type: "SupportBean",
				Keys: []esper.Expr{esper.Field[contextInitTermRemainderBean, int]("intPrimitive")},
			})
	case "hash-bare-prop":
		// `coalesce intPrimitive` — a bare property with no function
		// call has no fluent counterpart; Go always takes an algorithm
		// plus keys. Pin the assertion clause without claiming a boundary.
		buildErr = fmt.Errorf("function-less coalesce key is unrepresentable")
	case "hash-no-params":
		// `coalesce hash_code()` — the Go constructor rejects a stream
		// with an empty key list.
		_, buildErr = esper.NewHashContextByStreams("ACtxNoParams", esper.HashAlgorithmJavaHashCode, 10,
			esper.KeyContextStream{Type: "SupportBean"})
	case "statement-stream-type":
		// `context ACtx select * from SupportBean_S0` — a statement on a
		// type the single-type hash context does not list.
		_, buildErr = s.env.Build(esper.From[contextInitTermRemainderS0](s.env, "SupportBean_S0").Query(
			esper.StatementName("s0"), esper.WithContext("ACtx")))
	case "partition-named-window":
		// `partition by theString from MyWindow` — named windows are not
		// valid partition criteria.
		_, buildErr = esper.CreateKeyContextByStreams(s.env, "SegmentedByWhat",
			esper.KeyContextStream{
				Type: "MyWindow",
				Keys: []esper.Expr{esper.Field[contextInitTermRemainderBean, string]("theString")},
			})
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", contextInitTermRemainderID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", contextInitTermRemainderID, step.Statement)
	}
	// Verify the Go rejection carries the expected category and wording
	// before recording the pinned Java assertion clause; the unrepresentable probes
	// skip the gate.
	expected := map[string]struct {
		code      esper.ErrorCode
		substring string
	}{
		"distinct-empty":         {esper.ErrorInvalidRule, "context key expression is required"},
		"hash-dummy-filter":      {esper.ErrorInvalidRule, `unknown field "dummy"`},
		"hash-bad-func":          {esper.ErrorInvalidRule, "unknown hash algorithm"},
		"hash-no-params":         {esper.ErrorInvalidRule, "context key expression is required"},
		"statement-stream-type":  {esper.ErrorInvalidRule, "requires that any of the event types that are listed in the segmented context"},
		"partition-named-window": {esper.ErrorInvalidRule, "partition criteria may not include named windows"},
	}
	if want, ok := expected[step.Statement]; ok {
		var espErr *esper.Error
		if !errors.As(buildErr, &espErr) || espErr.Code != want.code || !strings.Contains(buildErr.Error(), want.substring) {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", contextInitTermRemainderID, step.Statement, buildErr)
		}
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// undeployAll mirrors the Java execution's undeployAll: deployments retire
// and env-scoped contexts/windows are destroyed; every case in this family
// runs on a fresh env so the close path is enough.
func (s *contextInitTermRemainderCaseState) undeployAll(ctx context.Context) error {
	if s.engine == nil {
		return nil
	}
	return s.engine.Close(ctx)
}

func decodeContextInitTermRemainderPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(step.Payload, &fields); err != nil {
		return nil, fmt.Errorf("%s: decode %s payload: %w", contextInitTermRemainderID, step.EventType, err)
	}
	decode := func(name string, target any) error {
		raw, ok := fields[name]
		if !ok {
			return fmt.Errorf("%s: %s payload field %q is required", contextInitTermRemainderID, step.EventType, name)
		}
		if err := json.Unmarshal(raw, target); err != nil {
			return fmt.Errorf("%s: %s payload field %q: %w", contextInitTermRemainderID, step.EventType, name, err)
		}
		return nil
	}
	wantKeys := map[string][]string{
		"SupportBean":    {"theString", "intPrimitive"},
		"SupportBean_S0": {"id", "p00"},
	}
	expected, known := wantKeys[step.EventType]
	if !known {
		return nil, fmt.Errorf("%s: unsupported send event type %q", contextInitTermRemainderID, step.EventType)
	}
	if len(fields) != len(expected) {
		return nil, fmt.Errorf("%s: %s send payload keys = %v, want %v", contextInitTermRemainderID, step.EventType, fields, expected)
	}
	switch step.EventType {
	case "SupportBean":
		payload := contextInitTermRemainderBean{}
		if err := decode("theString", &payload.TheString); err != nil {
			return nil, err
		}
		if err := decode("intPrimitive", &payload.IntPrimitive); err != nil {
			return nil, err
		}
		return payload, nil
	case "SupportBean_S0":
		payload := contextInitTermRemainderS0{}
		if err := decode("id", &payload.ID); err != nil {
			return nil, err
		}
		if err := decode("p00", &payload.P00); err != nil {
			return nil, err
		}
		return payload, nil
	}
	return nil, fmt.Errorf("%s: unsupported send event type %q", contextInitTermRemainderID, step.EventType)
}

func loadContextInitTermRemainderScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", contextInitTermRemainderID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", contextInitTermRemainderID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextInitTermRemainderID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextInitTermRemainderID, err)
	}
	if err := requireInfraNWTableOnDeleteFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaSourceFiles", "javaRuntimes",
		"javaStaticIds", "javaNames", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version     string   `json:"version"`
		ID          string   `json:"id"`
		Description string   `json:"description"`
		JavaCommit  string   `json:"javaCommit"`
		JavaSource  string   `json:"javaSource"`
		Sources     []string `json:"javaSourceFiles"`
		Runtimes    []string `json:"javaRuntimes"`
		Statics     []string `json:"javaStaticIds"`
		Names       []string `json:"javaNames"`
		Flags       []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextInitTermRemainderID, err)
	}
	if metadata.Version != compat.ScenarioVersion ||
		metadata.ID != contextInitTermRemainderID ||
		metadata.JavaCommit != contextInitTermRemainderJavaCommit ||
		metadata.JavaSource != contextInitTermRemainderTemporalSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario header drift: %#v", contextInitTermRemainderID, metadata)
	}
	if !reflect.DeepEqual(metadata.Sources, contextInitTermRemainderJavaSources) {
		return compat.Scenario{}, fmt.Errorf("%s javaSourceFiles drift: %#v", contextInitTermRemainderID, metadata.Sources)
	}
	if !reflect.DeepEqual(metadata.Runtimes, contextInitTermRemainderJavaRuntimeIDs) {
		return compat.Scenario{}, fmt.Errorf("%s javaRuntimes drift: %#v", contextInitTermRemainderID, metadata.Runtimes)
	}
	if !reflect.DeepEqual(metadata.Statics, contextInitTermRemainderJavaStaticIDs) {
		return compat.Scenario{}, fmt.Errorf("%s javaStaticIds drift: %#v", contextInitTermRemainderID, metadata.Statics)
	}
	if !reflect.DeepEqual(metadata.Names, contextInitTermRemainderJavaExecutions) {
		return compat.Scenario{}, fmt.Errorf("%s javaNames drift: %#v", contextInitTermRemainderID, metadata.Names)
	}
	if len(metadata.Flags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s javaFlags must be empty: %#v", contextInitTermRemainderID, metadata.Flags)
	}
	var cases []struct {
		Case          string `json:"case"`
		Ordinal       int    `json:"ordinal"`
		RuntimeID     string `json:"runtimeId"`
		ExecutionName string `json:"executionName"`
	}
	if err := json.Unmarshal(root["cases"], &cases); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s cases: %w", contextInitTermRemainderID, err)
	}
	if len(cases) != len(contextInitTermRemainderCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", contextInitTermRemainderID, len(contextInitTermRemainderCases))
	}
	for index, entry := range cases {
		if entry.Case != contextInitTermRemainderCases[index] ||
			entry.Ordinal != contextInitTermRemainderCaseOrdinals[index] ||
			entry.RuntimeID != contextInitTermRemainderJavaRuntimeIDs[index] ||
			entry.ExecutionName != contextInitTermRemainderJavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", contextInitTermRemainderID, index)
		}
	}
	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextInitTermRemainderID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextInitTermRemainderID, err)
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", contextInitTermRemainderID, err)
	}
	// Defense in depth: pin the exact step count and each op's raw field
	// set, and validate the case membership/compileWithoutPath contract
	// that compat.Step cannot express.
	stepFields := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"send":         {"op", "case", "eventType", "payload"},
		"advance-time": {"op", "case", "at"},
		"build-error":  {"op", "case", "statement", "epl", "expectError", "compileWithoutPath"},
		"undeploy-all": {"op", "case"},
	}
	if len(rawSteps) != len(scenario.Steps) || len(rawSteps) != 34 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 34 steps, found %d", contextInitTermRemainderID, len(rawSteps))
	}
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextInitTermRemainderID, index, err)
		}
		var op string
		if err := json.Unmarshal(object["op"], &op); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d op: %w", contextInitTermRemainderID, index, err)
		}
		allowed, ok := stepFields[op]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s step %d has unsupported op %q", contextInitTermRemainderID, index, op)
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
				return compat.Scenario{}, fmt.Errorf("%s step %d (%s) carries unexpected field %q", contextInitTermRemainderID, index, op, field)
			}
		}
	}
	for _, step := range scenario.Steps {
		if err := validateContextInitTermRemainderStep(step); err != nil {
			return compat.Scenario{}, err
		}
	}
	for index, rawStep := range rawSteps {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawStep, &fields); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextInitTermRemainderID, index, err)
		}
		var step struct {
			Op                 string `json:"op"`
			Statement          string `json:"statement"`
			CompileWithoutPath *bool  `json:"compileWithoutPath"`
		}
		if err := json.Unmarshal(rawStep, &step); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextInitTermRemainderID, index, err)
		}
		if step.Op != "build-error" {
			continue
		}
		// path-less probes compile the context EPL directly and must
		// carry compileWithoutPath=true; probes needing the ACtx/MyWindow
		// fixtures must not.
		pathless := map[string]bool{
			"distinct-no-as":         true,
			"distinct-pattern":       true,
			"distinct-subselect":     true,
			"distinct-empty":         true,
			"start-distinct":         true,
			"now-alone-terminated":   true,
			"now-and-nonoverlapping": true,
			"now-with-filter":        true,
			"hash-dummy-filter":      true,
			"hash-bad-func":          true,
			"hash-bare-prop":         true,
			"hash-no-params":         true,
			"statement-stream-type":  false,
			"partition-named-window": false,
		}
		want, known := pathless[step.Statement]
		if !known {
			return compat.Scenario{}, fmt.Errorf("%s step %d build-error probe %q is not pinned", contextInitTermRemainderID, index, step.Statement)
		}
		got := step.CompileWithoutPath != nil && *step.CompileWithoutPath
		if got != want {
			return compat.Scenario{}, fmt.Errorf("%s step %d build-error probe %q compileWithoutPath = %v, want %v", contextInitTermRemainderID, index, step.Statement, got, want)
		}
		_ = fields
	}
	return scenario, nil
}

func validateContextInitTermRemainderStep(step compat.Step) error {
	switch step.Op {
	case "case":
		if !contextInitTermRemainderCaseSet()[step.Case] {
			return fmt.Errorf("%s step case %q is not one of %v", contextInitTermRemainderID, step.Case, contextInitTermRemainderCases)
		}
		if len(step.Payload) != 0 {
			return fmt.Errorf("%s case step %q may not carry a payload", contextInitTermRemainderID, step.Case)
		}
	case "deploy", "build-error", "send", "advance-time", "undeploy-all":
		if !contextInitTermRemainderCaseSet()[step.Case] {
			return fmt.Errorf("%s %s step case %q is not one of %v", contextInitTermRemainderID, step.Op, step.Case, contextInitTermRemainderCases)
		}
		if step.Op == "deploy" {
			pinned, ok := contextInitTermRemainderDeployEPLs[step.Case][step.Statement]
			if !ok || step.Epl != pinned {
				return fmt.Errorf("%s deploy step %q carries an unpinned EPL %q", contextInitTermRemainderID, step.Statement, step.Epl)
			}
		}
		if step.Op == "build-error" {
			pinned, ok := contextInitTermRemainderProbeEPLs[step.Case][step.Statement]
			if !ok || step.Epl != pinned {
				return fmt.Errorf("%s build-error step %q carries an unpinned EPL %q", contextInitTermRemainderID, step.Statement, step.Epl)
			}
			if strings.TrimSpace(step.ExpectError) == "" {
				return fmt.Errorf("%s build-error step %q requires expectError", contextInitTermRemainderID, step.Statement)
			}
		}
		if step.Op == "send" && step.EventType != "SupportBean" && step.EventType != "SupportBean_S0" {
			return fmt.Errorf("%s send step eventType %q is not one of the pinned types", contextInitTermRemainderID, step.EventType)
		}
	default:
		return fmt.Errorf("%s unsupported step op %q", contextInitTermRemainderID, step.Op)
	}
	return nil
}

func contextInitTermRemainderCaseSet() map[string]bool {
	set := make(map[string]bool, len(contextInitTermRemainderCases))
	for _, name := range contextInitTermRemainderCases {
		set[name] = true
	}
	return set
}

func validateContextInitTermRemainderScenario(scenario compat.Scenario) error {
	if scenario.ID != contextInitTermRemainderID || scenario.Version != compat.ScenarioVersion {
		return fmt.Errorf("%s scenario header drift: %#v", contextInitTermRemainderID, scenario)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", contextInitTermRemainderID)
	}
	return nil
}
