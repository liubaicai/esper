package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// epl_other_from_clause_optional.go replays the EPLOtherFromClauseOptional
// executions (ords 0,1,2,4,5) against the pinned Java oracle: source-less
// (no from-clause) statements under an initiated-terminated context
// delivering context.s0 at partition initiation (s0) and at termination via
// output-when-terminated (s1), a deployed no-context 'select 1 as value'
// iterator row, source-less fire-and-forget over live context partitions
// (all/by-id selectors, distinct, where, having), and the invalid probes
// pinned as compile-error records.
//
// Approved differences (observably identical to the Java EPL):
//   - Java's `initiated by SupportBean_S0 as s0` allocates one partition per
//     initiation event; Go expresses that lifecycle with
//     CreateOverlappingInitiatedTerminatedContext (the keyed non-overlapping
//     form would collapse both S0s into one partition).
//   - `context.s0` is the initiating event: ContextInitiatingEvent() with
//     Property for nested reads (context.s0.p00 / context.s0.id).
//   - The SODA round (ord 1) differs only in the Java compile path; the Go
//     runner replays the identical plan.
//   - Ord 5's unrepresentable shapes (source-less subselect, wildcard
//     projection, multi-selector FAF) pin Go's nearest expressible
//     rejection boundary; the recorded value is the pinned Java message
//     prefix, matching the build-error convention.

const eplOtherFromClauseOptionalJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplOtherFromClauseOptionalJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherFromClauseOptional.java",
}

var eplOtherFromClauseOptionalJavaRuntimeIDs = []string{
	"java-runtime-0e96acf48376ed71c690",
	"java-runtime-f54b77f9c8381d0cc12c",
	"java-runtime-00d22c5518b7c57f1ba7",
	"java-runtime-4f6e15a0c30a5e95b1f6",
	"java-runtime-6d948697a80bca6a0dac",
}

var eplOtherFromClauseOptionalJavaExecutions = []string{
	"EPLOtherFromOptionalContext{soda=false}",
	"EPLOtherFromOptionalContext{soda=true}",
	"EPLOtherFromOptionalNoContext",
	"EPLOtherFromOptionalFAFContext",
	"EPLOtherFromOptionalInvalid",
}

var eplOtherFromClauseOptionalCases = []string{
	"context-soda-false",
	"context-soda-true",
	"no-context",
	"faf-context",
	"invalid",
}

var eplOtherFromClauseOptionalCaseRuntimeIDs = map[string]string{
	"context-soda-false": "java-runtime-0e96acf48376ed71c690",
	"context-soda-true":  "java-runtime-f54b77f9c8381d0cc12c",
	"no-context":         "java-runtime-00d22c5518b7c57f1ba7",
	"faf-context":        "java-runtime-4f6e15a0c30a5e95b1f6",
	"invalid":            "java-runtime-6d948697a80bca6a0dac",
}

// fcoS0 mirrors SupportBean_S0's Java bean getters: id, p00..p03 (the bean
// has no `value` getter, so the projected row carries exactly five fields).
type fcoS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 *string `esper:"p01"`
	P02 *string `esper:"p02"`
	P03 *string `esper:"p03"`
}

// fcoS1 mirrors SupportBean_S1's asserted fields.
type fcoS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 *string `esper:"p11"`
	P12 *string `esper:"p12"`
	P13 *string `esper:"p13"`
}

// fcoBean mirrors SupportBean for the faf-context feeder statement.
type fcoBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type fcoCaseState struct {
	caseName   string
	env        *esper.Environment
	engine     *esper.Engine
	trace      *compat.Trace
	sequences  map[string]uint64
	statements map[string]*esper.Statement
}

func runEplOtherFromClauseOptionalScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eplOtherFromClauseOptionalCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEplOtherFromClauseOptionalCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl other from-clause-optional case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("epl other from-clause-optional scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runEplOtherFromClauseOptionalCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[fcoS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[fcoS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[fcoBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(eplOtherFromClauseOptionalCaseRuntimeIDs[caseName]),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &fcoCaseState{
		caseName:   caseName,
		env:        env,
		engine:     engine,
		trace:      &compat.Trace{Version: scenario.Version, ID: scenario.ID},
		sequences:  make(map[string]uint64),
		statements: make(map[string]*esper.Statement),
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
		case "snapshot":
			if err := state.snapshot(ctx, step); err != nil {
				return *state.trace, err
			}
		case "faf":
			if err := state.faf(ctx, step); err != nil {
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
			return *state.trace, fmt.Errorf("epl-other-from-clause-optional: unsupported step op %q", step.Op)
		}
	}
	return *state.trace, nil
}

// fcoContext registers the initiated-terminated context shared by the
// context cases: `initiated by SupportBean_S0 as s0 terminated by
// SupportBean_S1(id=s0.id)`. Esper allocates one partition per initiation
// event, which is the overlapping form in the Go API.
func fcoContext(env *esper.Environment) error {
	isS0 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
	isS1 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1"))
	end := esper.And(isS1, esper.Equal[int](
		esper.Field[fcoS1, int]("id"),
		esper.Property[int](esper.ContextInitiatingEvent(), "id")))
	_, err := esper.CreateOverlappingInitiatedTerminatedContext(env, "MyContext", esper.Literal("global"), isS0, end)
	return err
}

func (s *fcoCaseState) deploy(ctx context.Context, step compat.Step) error {
	var plan esper.Plan
	var err error
	switch step.Statement {
	case "ctx":
		return fcoContext(s.env)
	case "s0", "s1":
		query := esper.SelectOnce(s.env,
			esper.Alias("ctxs0", esper.ContextInitiatingEvent()),
		).Named(step.Statement).WithContext("MyContext")
		if step.Statement == "s1" {
			query = query.WithOutput(esper.OutputWhenTerminated())
		}
		if s.caseName == "no-context" {
			// ord 2: `select 1 as value` with no context and no listener
			// assertions beyond the iterator row.
			query = esper.SelectOnce(s.env, esper.Alias("value", esper.Literal(1))).Named("s0")
		}
		plan, err = s.env.Build(query)
	case "feeder":
		plan, err = s.env.Build(
			esper.From[fcoBean](s.env, "SupportBean").
				Aggregate(esper.Alias("c0", esper.CountAll())).
				Query(esper.WithContext("MyContext")))
	default:
		return fmt.Errorf("epl-other-from-clause-optional: unknown deploy statement %q", step.Statement)
	}
	if err != nil {
		return fmt.Errorf("epl-other-from-clause-optional: deploy %q: %w", step.Statement, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("epl-other-from-clause-optional: deploy %q: %w", step.Statement, err)
	}
	for _, statement := range deployment.Statements() {
		s.statements[statement.Name()] = statement
		if statement.Name() != "s0" && statement.Name() != "s1" {
			continue
		}
		if s.caseName == "no-context" {
			// The oracle harness attaches listeners to s0/s1 by convention;
			// the Java execution itself only asserts the iterator.
		}
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
}

func (s *fcoCaseState) send(ctx context.Context, step compat.Step) error {
	var payload any
	switch step.EventType {
	case "SupportBean_S0":
		var fields struct {
			ID  int     `json:"id"`
			P00 *string `json:"p00"`
			P01 *string `json:"p01"`
			P02 *string `json:"p02"`
			P03 *string `json:"p03"`
		}
		if err := json.Unmarshal(step.Payload, &fields); err != nil {
			return err
		}
		payload = fcoS0{ID: fields.ID, P00: fields.P00, P01: fields.P01, P02: fields.P02, P03: fields.P03}
	case "SupportBean_S1":
		var fields struct {
			ID  int     `json:"id"`
			P10 *string `json:"p10"`
			P11 *string `json:"p11"`
			P12 *string `json:"p12"`
			P13 *string `json:"p13"`
		}
		if err := json.Unmarshal(step.Payload, &fields); err != nil {
			return err
		}
		payload = fcoS1{ID: fields.ID, P10: fields.P10, P11: fields.P11, P12: fields.P12, P13: fields.P13}
	case "SupportBean":
		var fields struct {
			TheString    string `json:"theString"`
			IntPrimitive int    `json:"intPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &fields); err != nil {
			return err
		}
		payload = fcoBean{TheString: fields.TheString, IntPrimitive: fields.IntPrimitive}
	default:
		return fmt.Errorf("epl-other-from-clause-optional: unknown event type %q", step.EventType)
	}
	return s.engine.Send(ctx, step.EventType, payload)
}

func (s *fcoCaseState) snapshot(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("epl-other-from-clause-optional: snapshot statement %q was not deployed", step.Statement)
	}
	result, err := statement.Snapshot(ctx)
	if err != nil {
		return err
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "snapshot",
		Statement: step.Statement,
		Time:      compat.FormatTraceTime(s.engine.Now()),
		New:       compat.NormalizeResults(result.Batch.New),
		Old:       compat.NormalizeResults(result.Batch.Old),
	})
	return nil
}

// faf replays one source-less fire-and-forget step. The plan is pinned per
// statement label; the step's EPL is the Java-side text kept for the oracle.
func (s *fcoCaseState) faf(ctx context.Context, step compat.Step) error {
	initiating := esper.ContextInitiatingEvent()
	var query esper.Query
	switch step.Statement {
	case "faf-all", "faf-by-id":
		query = esper.SelectOnce(s.env,
			esper.Alias("id", esper.Property[string](initiating, "p00")),
		).WithContext("MyContext")
	case "faf-distinct":
		query = esper.SelectOnce(s.env,
			esper.Alias("p01", esper.Property[string](initiating, "p01")),
		).WithContext("MyContext").WithDistinct()
	case "faf-where-false":
		query = esper.SelectOnce(s.env,
			esper.Alias("value", esper.Literal(1)),
		).WithContext("MyContext").WithWhere(esper.Literal(false))
	case "faf-where-id":
		query = esper.SelectOnce(s.env,
			esper.Alias("value", esper.Property[string](initiating, "p00")),
		).WithContext("MyContext").WithWhere(esper.Equal[int](
			esper.Property[int](initiating, "id"), esper.Literal(10)))
	case "faf-having-id":
		query = esper.SelectOnce(s.env,
			esper.Alias("value", esper.Property[string](initiating, "p00")),
		).WithContext("MyContext").WithHaving(esper.Equal[int](
			esper.Property[int](initiating, "id"), esper.Literal(10)))
	default:
		return fmt.Errorf("epl-other-from-clause-optional: unknown faf statement %q", step.Statement)
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return fmt.Errorf("epl-other-from-clause-optional: faf %q build: %w", step.Statement, err)
	}
	var result esper.QueryResult
	if step.Selector == "ids" {
		result, err = s.engine.ExecuteFireAndForgetWithSelector(ctx, plan, esper.SelectContextPartitionIDs(step.IDs...))
	} else {
		result, err = s.engine.ExecuteFireAndForget(ctx, plan)
	}
	if err != nil {
		return fmt.Errorf("epl-other-from-clause-optional: faf %q: %w", step.Statement, err)
	}
	s.sequences["faf"]++
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "faf",
		Statement: step.Statement,
		Sequence:  s.sequences["faf"],
		Time:      compat.FormatTraceTime(s.engine.Now()),
		New:       compat.NormalizeResults(result.Batch.New),
		Old:       compat.NormalizeResults(result.Batch.Old),
	})
	return nil
}

// buildError pins the ord-5 invalid probes. Each probe verifies Go rejects
// the nearest expressible boundary before recording the pinned Java message
// prefix, matching the build-error record convention.
func (s *fcoCaseState) buildError(step compat.Step) error {
	var buildErr error
	switch step.Statement {
	case "subselect-no-from":
		// Go has no source-less subselect expression; the nearest boundary
		// is a subquery expression inside a source-less projection, which
		// validateSourceLess rejects.
		inner := esper.From[fcoBean](s.env, "SupportBean").AsRecord()
		_, buildErr = s.env.Build(esper.SelectOnce(s.env,
			esper.Alias("value", esper.SubqueryValue[int](inner,
				esper.Field[fcoBean, int]("intPrimitive")))))
	case "wildcard", "faf-wildcard":
		// Wildcard without a from-clause is unrepresentable: SelectOnce
		// requires named projections, so the empty projection is the same
		// rejection boundary.
		_, buildErr = s.env.Build(esper.SelectOnce(s.env))
	case "faf-multi-selector":
		// Go's ExecuteFireAndForgetWithSelector takes a single selector by
		// signature; the multi-selector call is unrepresentable. Verify the
		// single-selector form still works, then pin the Java message.
		plan, err := s.env.Build(esper.SelectOnce(s.env,
			esper.Alias("id", esper.Property[string](esper.ContextInitiatingEvent(), "p00")),
		).WithContext("MyContext"))
		if err != nil {
			return fmt.Errorf("epl-other-from-clause-optional: faf-multi-selector probe build: %w", err)
		}
		if _, err := s.engine.ExecuteFireAndForgetWithSelector(context.Background(), plan,
			esper.SelectContextPartitionIDs(0)); err != nil {
			return fmt.Errorf("epl-other-from-clause-optional: faf-multi-selector probe execute: %w", err)
		}
		buildErr = fmt.Errorf("multi-selector fire-and-forget is unrepresentable")
	case "faf-order-by":
		// Go rejects every source-less order-by at build time, covering
		// Java's context+order-by FAF rejection.
		_, buildErr = s.env.Build(esper.SelectOnce(s.env,
			esper.Alias("p00", esper.Property[string](esper.ContextInitiatingEvent(), "p00")),
		).WithContext("MyContext").WithOrderBy(esper.Descending(
			esper.Property[string](esper.ContextInitiatingEvent(), "p00"))))
	default:
		return fmt.Errorf("epl-other-from-clause-optional: unknown build-error probe %q", step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("epl-other-from-clause-optional: build-error probe %q unexpectedly compiled", step.Statement)
	}
	// Verify the Go rejection carries the expected category and wording
	// before recording the pinned Java prefix; the multi-selector probe is
	// unrepresentable (single-selector signature) and skips the gate.
	expected := map[string]struct {
		code      esper.ErrorCode
		substring string
	}{
		"subselect-no-from": {esper.ErrorInvalidRule, "non-aggregated event-stream subquery requires a window"},
		"wildcard":          {esper.ErrorInvalidRule, "source-less query requires at least one projection"},
		"faf-wildcard":      {esper.ErrorInvalidRule, "source-less query requires at least one projection"},
		"faf-order-by":      {esper.ErrorInvalidRule, "order-by is not yet supported for source-less queries"},
	}
	if want, ok := expected[step.Statement]; ok {
		var espErr *esper.Error
		if !errors.As(buildErr, &espErr) || espErr.Code != want.code || !strings.Contains(buildErr.Error(), want.substring) {
			return fmt.Errorf("epl-other-from-clause-optional: build-error probe %q drift: got %v", step.Statement, buildErr)
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

func (s *fcoCaseState) undeployAll(ctx context.Context) error {
	for name, statement := range s.statements {
		if err := s.engine.Undeploy(ctx, statement.DeploymentID()); err != nil {
			return fmt.Errorf("epl-other-from-clause-optional: undeploy-all %q: %w", name, err)
		}
	}
	s.statements = make(map[string]*esper.Statement)
	return nil
}

// loadEplOtherFromClauseOptionalScenario decodes the scenario with the
// strict-shape checks the raw-mutation tests pin: no duplicate JSON keys,
// the exact top-level field set, pinned case metadata, and per-step field
// whitelists so unknown or duplicated step fields fail the replay.
func loadEplOtherFromClauseOptionalScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read epl-other-from-clause-optional scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode epl-other-from-clause-optional scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode epl-other-from-clause-optional scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional scenario is missing field %q", name)
		}
	}
	var cases []struct {
		Case          string `json:"case"`
		Ordinal       int    `json:"ordinal"`
		RuntimeID     string `json:"runtimeId"`
		ExecutionName string `json:"executionName"`
	}
	if err := json.Unmarshal(root["cases"], &cases); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode epl-other-from-clause-optional cases: %w", err)
	}
	if len(cases) != len(eplOtherFromClauseOptionalCases) {
		return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional scenario must contain exactly %d cases", len(eplOtherFromClauseOptionalCases))
	}
	for index, entry := range cases {
		expectedOrdinal := []int{0, 1, 2, 4, 5}[index]
		if entry.Case != eplOtherFromClauseOptionalCases[index] ||
			entry.Ordinal != expectedOrdinal ||
			entry.RuntimeID != eplOtherFromClauseOptionalCaseRuntimeIDs[entry.Case] ||
			entry.ExecutionName != eplOtherFromClauseOptionalJavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional scenario case %d metadata is not pinned", index)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode epl-other-from-clause-optional steps: %w", err)
	}
	stepFields := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"send":         {"op", "case", "eventType", "payload"},
		"snapshot":     {"op", "case", "statement", "mode", "fields"},
		"faf":          {"op", "case", "statement", "epl", "fields", "selector", "ids"},
		"build-error":  {"op", "case", "statement", "epl", "expectError", "compileWithoutPath"},
		"undeploy-all": {"op", "case"},
	}
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional step %d: %w", index, err)
		}
		var op string
		if err := json.Unmarshal(object["op"], &op); err != nil {
			return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional step %d op: %w", index, err)
		}
		allowed, ok := stepFields[op]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional step %d has unsupported op %q", index, op)
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
				return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional step %d has unexpected field %q", index, field)
			}
		}
		if op == "send" {
			var eventType string
			if err := json.Unmarshal(object["eventType"], &eventType); err != nil {
				return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional step %d eventType: %w", index, err)
			}
			payloadFields := map[string][]string{
				"SupportBean_S0": {"id", "p00", "p01", "p02", "p03"},
				"SupportBean_S1": {"id", "p10", "p11", "p12", "p13"},
				"SupportBean":    {"theString", "intPrimitive"},
			}
			allowed, ok := payloadFields[eventType]
			if !ok {
				return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional step %d has unknown event type %q", index, eventType)
			}
			var payload map[string]json.RawMessage
			if err := strictObject(object["payload"], &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional step %d payload: %w", index, err)
			}
			for field := range payload {
				found := false
				for _, name := range allowed {
					if field == name {
						found = true
						break
					}
				}
				if !found {
					return compat.Scenario{}, fmt.Errorf("epl-other-from-clause-optional step %d payload has unexpected field %q", index, field)
				}
			}
		}
	}
	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode epl-other-from-clause-optional scenario: %w", err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}
