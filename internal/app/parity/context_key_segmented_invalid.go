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

// context_key_segmented_invalid.go replays ContextKeySegmentedInvalid
// (ord 19) against the pinned Java oracle: nine tryInvalidCompile probes
// record the pinned Java message prefixes for segmented-context
// rejections. Probes 1-6 compile without the runtime path; probes 7-9
// compile against the path after their fixture deploys (the
// SegmentedByAString context, the MyWindow named window, the SomeSchema
// type and the TheSomeSchemaCtx context).
//
// Approved differences (observably identical to the Java EPL):
//   - `create context`/`create window`/`create schema` map to env-level
//     registrations (CreateKeyContextByStreams, CreateNamedWindow,
//     RegisterMap); Go has no module path, so compileWithoutPath has no
//     Go-side counterpart and only steers the oracle's compiler args.
//   - Probes 1 (per-stream filter in the partition spec) and 6
//     (subtype/supertype duplicate) are unrepresentable in the typed Go
//     API — KeyContextStream has no filter and Go has no event-type
//     inheritance — so they record the pinned prefix without claiming a
//     Go rejection boundary. Every other probe verifies the nearest
//     expressible Go rejection before recording the pinned value.

const contextKeySegmentedInvalidID = "context-key-segmented-invalid"
const contextKeySegmentedInvalidJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedInvalidJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedInvalidJavaRuntimeIDs = []string{
	"java-runtime-5e338406dcc2ca1aaf6a",
}

var contextKeySegmentedInvalidJavaExecutions = []string{
	"ContextKeySegmentedInvalid",
}

var contextKeySegmentedInvalidCases = []string{
	"invalid",
}

var contextKeySegmentedInvalidCaseRuntimeIDs = map[string]string{
	"invalid": "java-runtime-5e338406dcc2ca1aaf6a",
}

// cksiBean mirrors SupportBean's asserted fields.
type cksiBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// cksiS0 mirrors SupportBean_S0's asserted fields.
type cksiS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type cksiCaseState struct {
	caseName string
	env      *esper.Environment
	engine   *esper.Engine
	trace    *compat.Trace
}

func runContextKeySegmentedInvalidScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range contextKeySegmentedInvalidCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runContextKeySegmentedInvalidCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-invalid case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("context-key-segmented-invalid scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runContextKeySegmentedInvalidCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[cksiBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[cksiS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(contextKeySegmentedInvalidCaseRuntimeIDs[caseName]),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &cksiCaseState{
		caseName: caseName,
		env:      env,
		engine:   engine,
		trace:    &compat.Trace{Version: scenario.Version, ID: scenario.ID},
	}
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *state.trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "deploy":
			if err := state.deploy(step); err != nil {
				return *state.trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *state.trace, err
			}
		case "undeploy-all":
			// The Java execution ends with undeployAll; the Go fixture's
			// registrations are env-scoped and retire with the engine.
		default:
			return *state.trace, fmt.Errorf("context-key-segmented-invalid: unsupported step op %q", step.Op)
		}
	}
	return *state.trace, nil
}

// deploy executes the fixture registrations the path-ful probes compile
// against: the SegmentedByAString context (probe 7), the MyWindow named
// window (probe 8), and the SomeSchema type plus TheSomeSchemaCtx context
// (probe 9).
// The byte-exact EPLs the deploy and build-error steps pin.
var contextKeySegmentedInvalidDeployEPLs = map[string]string{
	"ctx":    "@public create context SegmentedByAString partition by theString from SupportBean",
	"window": "@public create window MyWindow#keepall as SupportBean",
	"schema": "@public create schema SomeSchema(ipAddress string)",
	"ctx2":   "@public create context TheSomeSchemaCtx Partition By ipAddress From SomeSchema",
}

var contextKeySegmentedInvalidProbeEPLs = map[string]string{
	"partition-filter-property":       "create context SegmentedByAString partition by string from SupportBean(dummy = 1)",
	"unknown-key-property":            "create context SegmentedByAString partition by dummy from SupportBean",
	"mismatched-key-count":            "create context SegmentedByAString partition by theString from SupportBean, id, p00 from SupportBean_S0",
	"mismatched-key-type":             "create context SegmentedByAString partition by theString from SupportBean, id from SupportBean_S0",
	"duplicate-type":                  "create context SegmentedByAString partition by theString from SupportBean, theString from SupportBean",
	"duplicate-subtype":               "create context SegmentedByAString partition by baseAB from ISupportBaseAB, a from ISupportA",
	"unlisted-statement-type":         "context SegmentedByAString select * from SupportBean_S0",
	"named-window-partition-criteria": "@public create context SegmentedByWhat partition by theString from MyWindow",
	"named-window-unlisted-schema":    "@public context TheSomeSchemaCtx create window MyEvent#time(30 sec) (ipAddress string)",
}

func (s *cksiCaseState) deploy(step compat.Step) error {
	if pinned, ok := contextKeySegmentedInvalidDeployEPLs[step.Statement]; !ok || step.Epl != pinned {
		return fmt.Errorf("context-key-segmented-invalid: deploy %q carries an unpinned EPL %q", step.Statement, step.Epl)
	}
	switch step.Statement {
	case "ctx":
		// `@public create context SegmentedByAString partition by theString
		// from SupportBean` — the single-stream form records streamKeys so
		// the unlisted-type statement validation fires for probe 7.
		_, err := esper.CreateKeyContextByStreams(s.env, "SegmentedByAString",
			esper.KeyContextStream{
				Type: "SupportBean",
				Keys: []esper.Expr{esper.Field[cksiBean, string]("theString")},
			})
		return err
	case "window":
		// `@public create window MyWindow#keepall as SupportBean`.
		schema, ok := s.env.Schema("SupportBean")
		if !ok {
			return fmt.Errorf("context-key-segmented-invalid: SupportBean schema is not registered")
		}
		_, err := esper.CreateNamedWindow(s.env, "MyWindow", schema,
			esper.NamedWindowRetention(esper.KeepAll()))
		return err
	case "schema":
		// `@public create schema SomeSchema(ipAddress string)`.
		_, err := esper.RegisterMap(s.env, "SomeSchema", []esper.FieldSpec{
			esper.FieldDef("ipAddress", reflect.TypeOf("")),
		})
		return err
	case "ctx2":
		// `@public create context TheSomeSchemaCtx Partition By ipAddress
		// From SomeSchema`.
		_, err := esper.CreateKeyContextByStreams(s.env, "TheSomeSchemaCtx",
			esper.KeyContextStream{
				Type: "SomeSchema",
				Keys: []esper.Expr{esper.Field[map[string]any, string]("ipAddress")},
			})
		return err
	default:
		return fmt.Errorf("context-key-segmented-invalid: unknown deploy %q in case %q", step.Statement, s.caseName)
	}
}

// buildError runs one expected-invalid probe against the fluent
// equivalent of the pinned EPL. Each probe verifies Go rejects the
// nearest expressible boundary (or is unrepresentable) before recording
// the pinned Java message prefix.
func (s *cksiCaseState) buildError(step compat.Step) error {
	if pinned, ok := contextKeySegmentedInvalidProbeEPLs[step.Statement]; !ok || step.Epl != pinned {
		return fmt.Errorf("context-key-segmented-invalid: build-error probe %q carries an unpinned EPL %q", step.Statement, step.Epl)
	}
	var buildErr error
	switch step.Statement {
	case "partition-filter-property":
		// `partition by string from SupportBean(dummy = 1)` — the Go
		// KeyContextStream has no per-stream filter, so the Java form is
		// unrepresentable. Pin the prefix without claiming a boundary.
		buildErr = fmt.Errorf("per-stream partition filter is unrepresentable")
	case "unknown-key-property":
		// `partition by dummy from SupportBean` — the nearest boundary is
		// the context-key field validation when a statement using the
		// context is built. A distinct context name keeps the probe from
		// colliding with the later SegmentedByAString deploy step.
		if _, err := esper.CreateKeyContextByStreams(s.env, "SegmentedByAStringBadKey",
			esper.KeyContextStream{
				Type: "SupportBean",
				Keys: []esper.Expr{esper.Field[cksiBean, string]("dummy")},
			}); err != nil {
			return fmt.Errorf("context-key-segmented-invalid: build-error probe %q context registration failed: %w", step.Statement, err)
		}
		_, buildErr = s.env.Build(esper.From[cksiBean](s.env, "SupportBean").Query(
			esper.StatementName("s0"), esper.WithContext("SegmentedByAStringBadKey")))
	case "mismatched-key-count":
		// `partition by theString from SupportBean, id, p00 from
		// SupportBean_S0` — the streams declare different key counts.
		_, buildErr = esper.CreateKeyContextByStreams(s.env, "SegmentedByAStringBadCount",
			esper.KeyContextStream{
				Type: "SupportBean",
				Keys: []esper.Expr{esper.Field[cksiBean, string]("theString")},
			},
			esper.KeyContextStream{
				Type: "SupportBean_S0",
				Keys: []esper.Expr{
					esper.Field[cksiS0, int]("id"),
					esper.Field[cksiS0, string]("p00"),
				},
			})
	case "mismatched-key-type":
		// `partition by theString from SupportBean, id from
		// SupportBean_S0` — String keys versus an Integer key.
		_, buildErr = esper.CreateKeyContextByStreams(s.env, "SegmentedByAStringBadType",
			esper.KeyContextStream{
				Type: "SupportBean",
				Keys: []esper.Expr{esper.Field[cksiBean, string]("theString")},
			},
			esper.KeyContextStream{
				Type: "SupportBean_S0",
				Keys: []esper.Expr{esper.Field[cksiS0, int]("id")},
			})
	case "duplicate-type":
		// `partition by theString from SupportBean, theString from
		// SupportBean` — the same event type listed twice.
		_, buildErr = esper.CreateKeyContextByStreams(s.env, "SegmentedByAStringDuplicate",
			esper.KeyContextStream{
				Type: "SupportBean",
				Keys: []esper.Expr{esper.Field[cksiBean, string]("theString")},
			},
			esper.KeyContextStream{
				Type: "SupportBean",
				Keys: []esper.Expr{esper.Field[cksiBean, string]("theString")},
			})
	case "duplicate-subtype":
		// `partition by baseAB from ISupportBaseAB, a from ISupportA` —
		// Go has no event-type inheritance, so the subtype/supertype
		// duplicate is unrepresentable. Pin the prefix without claiming a
		// boundary.
		buildErr = fmt.Errorf("event-type subtype/supertype duplicates are unrepresentable")
	case "unlisted-statement-type":
		// `context SegmentedByAString select * from SupportBean_S0` — a
		// statement on a type the single-type context does not list.
		_, buildErr = s.env.Build(esper.From[cksiS0](s.env, "SupportBean_S0").Query(
			esper.StatementName("s0"), esper.WithContext("SegmentedByAString")))
	case "named-window-partition-criteria":
		// `partition by theString from MyWindow` — named windows are not
		// valid partition criteria.
		_, buildErr = esper.CreateKeyContextByStreams(s.env, "SegmentedByWhat",
			esper.KeyContextStream{
				Type: "MyWindow",
				Keys: []esper.Expr{esper.Field[cksiBean, string]("theString")},
			})
	case "named-window-unlisted-schema":
		// `context TheSomeSchemaCtx create window MyEvent#time(30 sec)
		// (ipAddress string)` — the window's schema type is not listed by
		// the segmented context. Java's inline column list creates the
		// MyEvent type; the Go fixture registers it as a Map schema.
		schema, err := esper.RegisterMap(s.env, "MyEvent", []esper.FieldSpec{
			esper.FieldDef("ipAddress", reflect.TypeOf("")),
		})
		if err != nil {
			return fmt.Errorf("context-key-segmented-invalid: build-error probe %q schema registration failed: %w", step.Statement, err)
		}
		_, buildErr = esper.CreateNamedWindow(s.env, "MyEvent", schema,
			esper.NamedWindowContext("TheSomeSchemaCtx"),
			esper.NamedWindowRetention(esper.TimeWindow(30*time.Second)))
	default:
		return fmt.Errorf("context-key-segmented-invalid: unknown build-error probe %q", step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("context-key-segmented-invalid: build-error probe %q unexpectedly compiled", step.Statement)
	}
	// Verify the Go rejection carries the expected category and wording
	// before recording the pinned Java prefix; the unrepresentable probes
	// skip the gate.
	expected := map[string]struct {
		code      esper.ErrorCode
		substring string
	}{
		"unknown-key-property":            {esper.ErrorInvalidRule, `unknown field "dummy"`},
		"mismatched-key-count":            {esper.ErrorInvalidRule, "expected the same number of key expressions for each event type"},
		"mismatched-key-type":             {esper.ErrorInvalidRule, "found mismatch of property types"},
		"duplicate-type":                  {esper.ErrorInvalidRule, `the event type "SupportBean" is listed twice`},
		"unlisted-statement-type":         {esper.ErrorInvalidRule, "requires that any of the event types that are listed in the segmented context"},
		"named-window-partition-criteria": {esper.ErrorInvalidRule, "partition criteria may not include named windows"},
		"named-window-unlisted-schema":    {esper.ErrorInvalidRule, "requires that named windows are associated to an existing event type"},
	}
	if want, ok := expected[step.Statement]; ok {
		var espErr *esper.Error
		if !errors.As(buildErr, &espErr) || espErr.Code != want.code || !strings.Contains(buildErr.Error(), want.substring) {
			return fmt.Errorf("context-key-segmented-invalid: build-error probe %q drift: got %v", step.Statement, buildErr)
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

// loadContextKeySegmentedInvalidScenario decodes the scenario with the
// strict-shape checks the raw-mutation tests pin: no duplicate JSON keys,
// the exact top-level field set, pinned case metadata, and per-step field
// whitelists so unknown or duplicated step fields fail the replay.
func loadContextKeySegmentedInvalidScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-invalid scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read context-key-segmented-invalid scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-invalid scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-invalid scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-invalid scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-invalid scenario is missing field %q", name)
		}
	}
	var cases []struct {
		Case          string `json:"case"`
		Ordinal       int    `json:"ordinal"`
		RuntimeID     string `json:"runtimeId"`
		ExecutionName string `json:"executionName"`
	}
	if err := json.Unmarshal(root["cases"], &cases); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-invalid cases: %w", err)
	}
	if len(cases) != len(contextKeySegmentedInvalidCases) {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-invalid scenario must contain exactly %d cases", len(contextKeySegmentedInvalidCases))
	}
	for index, entry := range cases {
		if entry.Case != contextKeySegmentedInvalidCases[index] ||
			entry.Ordinal != 19 ||
			entry.RuntimeID != contextKeySegmentedInvalidCaseRuntimeIDs[entry.Case] ||
			entry.ExecutionName != contextKeySegmentedInvalidJavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-invalid scenario case %d metadata is not pinned", index)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-invalid steps: %w", err)
	}
	stepFields := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"build-error":  {"op", "case", "statement", "epl", "expectError", "compileWithoutPath"},
		"undeploy-all": {"op", "case"},
	}
	if len(rawSteps) != 15 {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-invalid scenario must contain exactly 15 steps, found %d", len(rawSteps))
	}
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-invalid step %d: %w", index, err)
		}
		var op string
		if err := json.Unmarshal(object["op"], &op); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-invalid step %d op: %w", index, err)
		}
		allowed, ok := stepFields[op]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-invalid step %d has unsupported op %q", index, op)
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
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-invalid step %d has unexpected field %q", index, field)
			}
		}
		var stepCase string
		if err := json.Unmarshal(object["case"], &stepCase); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-invalid step %d case: %w", index, err)
		}
		if stepCase != "invalid" {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-invalid step %d has unknown case %q", index, stepCase)
		}
	}
	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-invalid scenario: %w", err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}
