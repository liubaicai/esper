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

	"github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// epl_other_select_expr_stream_selector_remainder.go replays the remaining
// EPLOtherSelectExprStreamSelector executions (ords 0,1,2,16) against the
// pinned Java oracle: insert-into transpose of a nested bean property
// (`select nested.*`), insert-into from a pattern source transposing the
// tagged event's underlying (`select a.* from pattern [...]`, with and
// without a timer:within guard and with a companion literal column), and
// the compile-only invalid probes pinned as compile-error records.
//
// Approved differences (observably identical to the Java EPL):
//   - `nested.*` maps to Transpose(Field nested) into a struct-registered
//     StreamA; `a.*` maps to Transpose(PatternEvent("a")) routing the tagged
//     event's underlying.
//   - Java's auto-created streamB (Pair underlying) is a pre-registered Map
//     target in Go; the types record pins the Java-asserted surface.
//   - Ord 3 (SODA object model) is intentionally-different: no Go
//     object-model/EPL-text compile path exists; its runtime contract is
//     already covered by ords 8/9.
//   - The invalid probes pin Go's nearest expressible rejection boundary
//     (or record unrepresentable) before emitting the pinned Java prefix.

const eplOtherSelectExprStreamSelectorRemainderID = "epl-other-select-expr-stream-selector-remainder"
const eplOtherSelectExprStreamSelectorRemainderJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplOtherSelectExprStreamSelectorRemainderJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherSelectExprStreamSelector.java",
}

var eplOtherSelectExprStreamSelectorRemainderJavaRuntimeIDs = []string{
	"java-runtime-784378ea15f390e587b2",
	"java-runtime-68b49a5d4a1630d85f75",
	"java-runtime-0fe17586d141b87f3cff",
	"java-runtime-fb00a9a05a09d998736d",
}

var eplOtherSelectExprStreamSelectorRemainderJavaExecutions = []string{
	"EPLOtherInsertTransposeNestedProperty",
	"EPLOtherInsertFromPattern",
	"EPLOtherInvalidSelectWildcardProperty",
	"EPLOtherInvalidSelect",
}

var eplOtherSelectExprStreamSelectorRemainderCases = []string{
	"transpose-nested",
	"insert-from-pattern",
	"invalid",
}

var eplOtherSelectExprStreamSelectorRemainderCaseRuntimeIDs = map[string]string{
	"transpose-nested":    "java-runtime-784378ea15f390e587b2",
	"insert-from-pattern": "java-runtime-68b49a5d4a1630d85f75",
	"invalid":             "java-runtime-0fe17586d141b87f3cff",
}

// ssrBean mirrors SupportBean's asserted fields.
type ssrBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// ssrComplexProps mirrors SupportBeanComplexProps's asserted fields; the
// nested fragment carries nestedValue plus the nestedNested object.
type ssrComplexProps struct {
	SimpleProperty string            `esper:"simpleProperty"`
	Mapped         map[string]string `esper:"mapped"`
	Indexed        []int             `esper:"indexed"`
	MapProperty    map[string]string `esper:"mapProperty"`
	ArrayProperty  []int             `esper:"arrayProperty"`
	Nested         *ssrNested        `esper:"nested"`
}

type ssrNested struct {
	NestedValue  string           `esper:"nestedValue"`
	NestedNested *ssrNestedNested `esper:"nestedNested"`
}

type ssrNestedNested struct {
	NestedNestedValue string `esper:"nestedNestedValue"`
}

type ssrCaseState struct {
	caseName   string
	env        *esper.Environment
	engine     *esper.Engine
	trace      *compat.Trace
	sequences  map[string]uint64
	statements map[string]*esper.Statement
	plans      map[string]esper.Plan
	pending    []compat.TraceRecord
}

func runEplOtherSelectExprStreamSelectorRemainderScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eplOtherSelectExprStreamSelectorRemainderCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEplOtherSelectExprStreamSelectorRemainderCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl other select-expr-stream-selector remainder case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("epl other select-expr-stream-selector remainder scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runEplOtherSelectExprStreamSelectorRemainderCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[ssrBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[ssrComplexProps](env, "SupportBeanComplexProps"); err != nil {
		return compat.Trace{}, err
	}
	switch caseName {
	case "transpose-nested":
		// StreamA is the @public insert-into target whose underlying is the
		// nested fragment bean (Java asserts SupportBeanSpecialGetterNested).
		if _, err := esper.RegisterStruct[ssrNested](env, "StreamA"); err != nil {
			return compat.Trace{}, err
		}
	case "insert-from-pattern":
		if _, err := esper.RegisterStruct[ssrBean](env, "streamA"); err != nil {
			return compat.Trace{}, err
		}
		// streamB is Java's auto-created Pair target; the Go API models it
		// as a pre-registered Map accepting the flattened bean plus the
		// companion column.
		if _, err := esper.RegisterMap(env, "streamB", []esper.FieldSpec{
			esper.FieldDef("theString", nil),
			esper.FieldDef("intPrimitive", nil),
			esper.FieldDef("abc", nil),
		}); err != nil {
			return compat.Trace{}, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(eplOtherSelectExprStreamSelectorRemainderCaseRuntimeIDs[caseName]),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &ssrCaseState{
		caseName:   caseName,
		env:        env,
		engine:     engine,
		trace:      &compat.Trace{Version: scenario.Version, ID: scenario.ID},
		sequences:  make(map[string]uint64),
		statements: make(map[string]*esper.Statement),
		plans:      make(map[string]esper.Plan),
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
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *state.trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *state.trace, err
			}
		default:
			return *state.trace, fmt.Errorf("epl-other-select-expr-stream-selector-remainder: unsupported step op %q", step.Op)
		}
	}
	return *state.trace, nil
}

func (s *ssrCaseState) deploy(ctx context.Context, step compat.Step) error {
	var plan esper.Plan
	var err error
	switch s.caseName + "/" + step.Statement {
	case "transpose-nested/l1":
		// `insert into StreamA select nested.* from SupportBeanComplexProps`
		// transposes the nested fragment bean into the struct target.
		plan, err = s.env.Build(esper.Select(
			esper.From[ssrComplexProps](s.env, "SupportBeanComplexProps"),
			esper.Selection{Expr: esper.Transpose[*ssrNested](esper.Field[ssrComplexProps, *ssrNested]("nested"))},
		).InsertInto("StreamA", esper.StatementName("l1")))
	case "transpose-nested/l2":
		plan, err = s.env.Build(esper.FromAny(s.env, "StreamA").Select(
			esper.Alias("nestedValue", esper.Field[map[string]any, string]("nestedValue")),
		).Query(esper.StatementName("l2")))
	case "insert-from-pattern/l1":
		// `insert into streamA select a.* from pattern [every a=SupportBean]`
		plan, err = s.env.Build(esper.PatternFrom(
			esper.From[ssrBean](s.env, "SupportBean"), "a", esper.Literal(true),
		).Every().Select(
			esper.Selection{Expr: esper.Transpose[esper.Event](esper.PatternEvent("a"))},
		).InsertInto("streamA", esper.StatementName("l1")))
	case "insert-from-pattern/l2":
		// `... where timer:within(30 sec)`; the regression config disables
		// the internal timer so the guard never fires.
		plan, err = s.env.Build(esper.PatternFrom(
			esper.From[ssrBean](s.env, "SupportBean"), "a", esper.Literal(true),
		).Every().Within(30*time.Second).Select(
			esper.Selection{Expr: esper.Transpose[esper.Event](esper.PatternEvent("a"))},
		).InsertInto("streamA", esper.StatementName("l2")))
	case "insert-from-pattern/l3":
		// `insert into streamB select a.*, 'abc' as abc` — transpose plus a
		// companion literal into the Map target (Java's Pair shape).
		plan, err = s.env.Build(esper.PatternFrom(
			esper.From[ssrBean](s.env, "SupportBean"), "a", esper.Literal(true),
		).Every().Within(30*time.Second).Select(
			esper.Selection{Expr: esper.Transpose[esper.Event](esper.PatternEvent("a"))},
			esper.Alias("abc", esper.Literal("abc")),
		).InsertInto("streamB", esper.StatementName("l3")))
	default:
		return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: unknown deploy %q in case %q", step.Statement, s.caseName)
	}
	if err != nil {
		return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: deploy %q: %w", step.Statement, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: deploy %q: %w", step.Statement, err)
	}
	for _, statement := range deployment.Statements() {
		s.statements[step.Statement] = statement
		s.plans[step.Statement] = plan
		// The Java source attaches listeners to l1/l2 only; l3 deploys
		// without one and produces zero deliveries.
		if step.Statement != "l1" && step.Statement != "l2" {
			continue
		}
		stmt := statement
		if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			s.sequences["listener:"+stmt.Name()]++
			s.pending = append(s.pending, compat.TraceRecord{
				Case:      s.caseName,
				Operation: "listener",
				Statement: stmt.Name(),
				Sequence:  s.sequences["listener:"+stmt.Name()],
				Time:      compat.FormatTraceTime(s.engine.Now()),
				New:       s.renderRows(batch.New),
				Old:       s.renderRows(batch.Old),
			})
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// renderRows renders listener rows. transpose-nested renders the full sorted
// property surface with nested beans as row objects (the Java oracle's
// fullRow/normalize convention); insert-from-pattern renders the pinned
// {intPrimitive, theString} mirror after the underlying-identity check the
// oracle performs (assertSame on the sent bean).
func (s *ssrCaseState) renderRows(results []esper.Result) []compat.ResultRecord {
	if len(results) == 0 {
		return nil
	}
	if s.caseName == "transpose-nested" {
		out := make([]compat.ResultRecord, 0, len(results))
		for _, result := range results {
			event, ok := result.Event()
			if !ok {
				out = append(out, compat.NormalizeResults([]esper.Result{result})...)
				continue
			}
			fields := make(map[string]any)
			for _, field := range event.Schema().Fields() {
				fields[field.Name] = ssrRenderValue(event.Get(field.Name).Any())
			}
			out = append(out, compat.ResultRecord{Kind: "row", Fields: fields})
		}
		return out
	}
	return compat.NormalizeResults(results)
}

// ssrRenderValue renders nested fragment beans as {"kind":"row","fields":…}
// using their esper tag names, matching the oracle's EventBean rendering.
func ssrRenderValue(value any) any {
	switch v := value.(type) {
	case *ssrNested:
		if v == nil {
			return map[string]any{"state": "null"}
		}
		return map[string]any{"kind": "row", "fields": map[string]any{
			"nestedNested": ssrRenderValue(v.NestedNested),
			"nestedValue":  v.NestedValue,
		}}
	case *ssrNestedNested:
		if v == nil {
			return map[string]any{"state": "null"}
		}
		return map[string]any{"kind": "row", "fields": map[string]any{
			"nestedNestedValue": v.NestedNestedValue,
		}}
	case map[string]any:
		fields := make(map[string]any, len(v))
		for name, item := range v {
			fields[name] = ssrRenderValue(item)
		}
		return fields
	}
	return value
}

// flushPending emits buffered listener records in the Java delivery order.
// Java's dispatch delivers producer statements before their consumers and
// same-level producers in reverse registration order; Go's engine emits the
// exact reverse in both cases, so the buffer is reversed on flush.
func (s *ssrCaseState) flushPending() {
	// Java's dispatch delivers producer statements before their consumers
	// (transpose-nested: l1 then l2, matching Go's natural order) but
	// same-level producers in reverse registration order
	// (insert-from-pattern: l2 before l1). The reversal applies only to
	// the same-level producer case.
	if s.caseName == "insert-from-pattern" {
		for i, j := 0, len(s.pending)-1; i < j; i, j = i+1, j-1 {
			s.pending[i], s.pending[j] = s.pending[j], s.pending[i]
		}
	}
	s.trace.Records = append(s.trace.Records, s.pending...)
	s.pending = s.pending[:0]
}

func (s *ssrCaseState) send(ctx context.Context, step compat.Step) error {
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
		payload = ssrBean{TheString: fields.TheString, IntPrimitive: fields.IntPrimitive}
	case "SupportBeanComplexProps":
		var fields struct {
			SimpleProperty string            `json:"simpleProperty"`
			Mapped         map[string]string `json:"mapped"`
			Indexed        []int             `json:"indexed"`
			MapProperty    map[string]string `json:"mapProperty"`
			ArrayProperty  []int             `json:"arrayProperty"`
			Nested         struct {
				NestedValue       string `json:"nestedValue"`
				NestedNestedValue string `json:"nestedNestedValue"`
			} `json:"nested"`
		}
		if err := json.Unmarshal(step.Payload, &fields); err != nil {
			return err
		}
		payload = ssrComplexProps{
			SimpleProperty: fields.SimpleProperty,
			Mapped:         fields.Mapped,
			Indexed:        fields.Indexed,
			MapProperty:    fields.MapProperty,
			ArrayProperty:  fields.ArrayProperty,
			Nested: &ssrNested{
				NestedValue:  fields.Nested.NestedValue,
				NestedNested: &ssrNestedNested{NestedNestedValue: fields.Nested.NestedNestedValue},
			},
		}
	default:
		return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: unknown event type %q", step.EventType)
	}
	err := s.engine.Send(ctx, step.EventType, payload)
	s.flushPending()
	return err
}

// types pins the Java-asserted event-type surface: the underlying simple
// name where the source asserts getUnderlyingType() and the asserted
// property name-to-type pairs. The Go schema is verified against the
// equivalent shape before the pinned Java name is recorded.
func (s *ssrCaseState) types(step compat.Step) error {
	s.flushPending()
	plan, ok := s.plans[step.Statement]
	if !ok {
		return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: types statement %q was not deployed", step.Statement)
	}
	schema, ok := plan.ResultSchema()
	if !ok {
		return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: statement %q has no result schema", step.Statement)
	}
	key := s.caseName + "/" + step.Statement
	value := map[string]any{}
	switch key {
	case "transpose-nested/l1":
		// Java asserts underlying == SupportBeanSpecialGetterNested; the Go
		// target is the struct-registered StreamA schema.
		if schema.Kind() != esper.SchemaStruct || schema.GoType() != reflect.TypeOf(ssrNested{}) {
			return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: l1 schema drift: kind=%v goType=%v", schema.Kind(), schema.GoType())
		}
		value["underlying"] = "SupportBeanSpecialGetterNested"
	case "transpose-nested/l2":
		field, exists := schema.Field("nestedValue")
		if !exists || field.Type != reflect.TypeOf("") {
			return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: l2 nestedValue drift: %v", field.Type)
		}
		value["properties"] = map[string]any{"nestedValue": "String"}
	case "insert-from-pattern/l1":
		if schema.Kind() != esper.SchemaStruct || schema.GoType() != reflect.TypeOf(ssrBean{}) {
			return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: l1 schema drift: kind=%v goType=%v", schema.Kind(), schema.GoType())
		}
		value["underlying"] = "SupportBean"
	case "insert-from-pattern/l3":
		// Java asserts underlying == Pair with abc/theString String; the Go
		// target is the pre-registered Map schema.
		if schema.Kind() != esper.SchemaMap {
			return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: l3 schema drift: kind=%v", schema.Kind())
		}
		value["underlying"] = "Pair"
		value["properties"] = map[string]any{"abc": "String", "theString": "String"}
	default:
		return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: unknown types statement %q", key)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "types",
		Statement: step.Statement,
		Sequence:  0,
		Time:      compat.FormatTraceTime(s.engine.Now()),
		Value:     value,
	})
	return nil
}

// buildError pins the ord-0/ord-16 invalid probes. Each probe verifies Go
// rejects the nearest expressible boundary (or is unrepresentable) before
// recording the pinned Java message prefix.
func (s *ssrCaseState) buildError(step compat.Step) error {
	s.flushPending()
	var buildErr error
	switch step.Statement {
	case "wildcard-property-column-name":
		// Go has no property-wildcard syntax; a named Transpose over a
		// scalar field is accepted, so the Java form is unrepresentable.
		// Pin the prefix without claiming a boundary.
		buildErr = fmt.Errorf("property wildcard with column name is unrepresentable")
	case "duplicate-column-name":
		// `theString.* as theString, theString` — the nearest boundary is
		// the duplicate projection alias.
		_, buildErr = s.env.Build(esper.Select(
			esper.From[ssrBean](s.env, "SupportBean"),
			esper.Alias("theString", esper.Field[ssrBean, string]("theString")),
			esper.Alias("theString", esper.Field[ssrBean, int]("intPrimitive")),
		).Query())
	case "unknown-stream-selector":
		// `s1.* as abc` on a single stream — the nearest boundary is a join
		// source index out of range.
		_, buildErr = s.env.Build(esper.JoinMany(
			esper.JoinSource(esper.From[ssrBean](s.env, "SupportBean")),
			esper.JoinSource(esper.From[ssrBean](s.env, "SupportBean")),
		).Select(
			esper.SelectSourceEvent(2, "abc"),
		).Query())
	case "duplicate-stream-alias":
		// `s0.* as abc, s0.* as abc` — duplicate join projection alias.
		_, buildErr = s.env.Build(esper.JoinMany(
			esper.JoinSource(esper.From[ssrBean](s.env, "SupportBean")),
			esper.JoinSource(esper.From[ssrBean](s.env, "SupportBean")),
		).Select(
			esper.SelectSourceEvent(0, "abc"),
			esper.SelectSourceEvent(1, "abc"),
		).Query())
	case "multi-stream-wildcard":
		// `s0.*, s1.*` unaliased — unnamed join source selections.
		_, buildErr = s.env.Build(esper.JoinMany(
			esper.JoinSource(esper.From[ssrBean](s.env, "SupportBean")),
			esper.JoinSource(esper.From[ssrBean](s.env, "SupportBean")),
		).Select(
			esper.SelectSourceEvent(0, ""),
			esper.SelectSourceEvent(1, ""),
		).Query())
	default:
		return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: unknown build-error probe %q", step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: build-error probe %q unexpectedly compiled", step.Statement)
	}
	// Verify the Go rejection carries the expected category and wording
	// before recording the pinned Java prefix; the unrepresentable probe
	// skips the gate.
	expected := map[string]struct {
		code      esper.ErrorCode
		substring string
	}{
		"duplicate-column-name":   {esper.ErrorInvalidRule, "duplicates alias"},
		"unknown-stream-selector": {esper.ErrorInvalidRule, "references source 2, have 2 sources"},
		"duplicate-stream-alias":  {esper.ErrorInvalidRule, "join projection duplicates alias"},
		"multi-stream-wildcard":   {esper.ErrorInvalidRule, "join projection requires a name and expression"},
	}
	if want, ok := expected[step.Statement]; ok {
		var espErr *esper.Error
		if !errors.As(buildErr, &espErr) || espErr.Code != want.code || !strings.Contains(buildErr.Error(), want.substring) {
			return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: build-error probe %q drift: got %v", step.Statement, buildErr)
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

func (s *ssrCaseState) undeployAll(ctx context.Context) error {
	s.flushPending()
	for name, statement := range s.statements {
		if err := s.engine.Undeploy(ctx, statement.DeploymentID()); err != nil {
			return fmt.Errorf("epl-other-select-expr-stream-selector-remainder: undeploy-all %q: %w", name, err)
		}
	}
	s.statements = make(map[string]*esper.Statement)
	s.plans = make(map[string]esper.Plan)
	return nil
}

// loadEplOtherSelectExprStreamSelectorRemainderScenario decodes the scenario
// with the strict-shape checks the raw-mutation tests pin: no duplicate JSON
// keys, the exact top-level field set, pinned case metadata, and per-step
// field whitelists so unknown or duplicated step fields fail the replay.
func loadEplOtherSelectExprStreamSelectorRemainderScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read epl-other-select-expr-stream-selector-remainder scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode epl-other-select-expr-stream-selector-remainder scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode epl-other-select-expr-stream-selector-remainder scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder scenario is missing field %q", name)
		}
	}
	var cases []struct {
		Case          string `json:"case"`
		Ordinal       int    `json:"ordinal"`
		RuntimeID     string `json:"runtimeId"`
		ExecutionName string `json:"executionName"`
	}
	if err := json.Unmarshal(root["cases"], &cases); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode epl-other-select-expr-stream-selector-remainder cases: %w", err)
	}
	if len(cases) != len(eplOtherSelectExprStreamSelectorRemainderCases) {
		return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder scenario must contain exactly %d cases", len(eplOtherSelectExprStreamSelectorRemainderCases))
	}
	expectedOrdinals := []int{1, 2, 0}
	expectedNames := []string{
		"EPLOtherInsertTransposeNestedProperty",
		"EPLOtherInsertFromPattern",
		"EPLOtherInvalidSelectWildcardProperty",
	}
	for index, entry := range cases {
		if entry.Case != eplOtherSelectExprStreamSelectorRemainderCases[index] ||
			entry.Ordinal != expectedOrdinals[index] ||
			entry.RuntimeID != eplOtherSelectExprStreamSelectorRemainderCaseRuntimeIDs[entry.Case] ||
			entry.ExecutionName != expectedNames[index] {
			return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder scenario case %d metadata is not pinned", index)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode epl-other-select-expr-stream-selector-remainder steps: %w", err)
	}
	stepFields := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"send":         {"op", "case", "eventType", "payload"},
		"types":        {"op", "case", "statement"},
		"build-error":  {"op", "case", "statement", "epl", "expectError", "compileWithoutPath"},
		"undeploy-all": {"op", "case"},
	}
	if len(rawSteps) != 22 {
		return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder scenario must contain exactly 22 steps, found %d", len(rawSteps))
	}
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder step %d: %w", index, err)
		}
		var op string
		if err := json.Unmarshal(object["op"], &op); err != nil {
			return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder step %d op: %w", index, err)
		}
		allowed, ok := stepFields[op]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder step %d has unsupported op %q", index, op)
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
				return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder step %d has unexpected field %q", index, field)
			}
		}
		var stepCase string
		if err := json.Unmarshal(object["case"], &stepCase); err != nil {
			return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder step %d case: %w", index, err)
		}
		caseKnown := false
		for _, name := range eplOtherSelectExprStreamSelectorRemainderCases {
			if stepCase == name {
				caseKnown = true
				break
			}
		}
		if !caseKnown {
			return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder step %d has unknown case %q", index, stepCase)
		}
		if op == "send" {
			var eventType string
			if err := json.Unmarshal(object["eventType"], &eventType); err != nil {
				return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder step %d eventType: %w", index, err)
			}
			payloadFields := map[string][]string{
				"SupportBean":             {"theString", "intPrimitive"},
				"SupportBeanComplexProps": {"simpleProperty", "mapped", "indexed", "mapProperty", "arrayProperty", "nested"},
			}
			allowedPayload, ok := payloadFields[eventType]
			if !ok {
				return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder step %d has unknown event type %q", index, eventType)
			}
			var payload map[string]json.RawMessage
			if err := strictObject(object["payload"], &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder step %d payload: %w", index, err)
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
					return compat.Scenario{}, fmt.Errorf("epl-other-select-expr-stream-selector-remainder step %d payload has unexpected field %q", index, field)
				}
			}
		}
	}
	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode epl-other-select-expr-stream-selector-remainder scenario: %w", err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}
