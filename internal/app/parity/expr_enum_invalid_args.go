package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// expr-enum-invalid-args closes the last unreferenced enum-method
// executions: five tryInvalidCompile-only executions across five sibling
// enum method suites. Each execution replays its pinned EPL probes against
// the fluent Go boundary (or pins them as unrepresentable where Java's
// grammar accepts a form Go generics reject outright).
//
//   - min-invalid (ExprEnumMinMax ord 4, ExprEnumInvalid): 0-parameter
//     footprint over an event collection (unrepresentable — Go's
//     EnumMin[T EnumOrdered] generics reject []SupportBean_ST0) and a
//     null-typed min selector (Go boundary: Null-type is not allowed).
//   - minby-invalid (ExprEnumMinMaxBy ord 2): null-typed minBy selector.
//   - orderby-invalid (ExprEnumOrderBy ord 4): 0-parameter orderBy over an
//     event collection (unrepresentable) and a null-typed selector.
//   - take-invalid (ExprEnumTakeAndTakeLast ord 2): take(null) — the
//     expression-valued count rejects a null-typed parameter.
//   - takewhile-invalid (ExprEnumTakeWhileAndWhileLast ord 2):
//     takeWhile(x => null) — the predicate parameter rejects a null-typed
//     expression.
const (
	exprEnumInvalidArgsID         = "expr-enum-invalid-args"
	exprEnumInvalidArgsJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	exprEnumInvalidArgsMinMaxSrc  = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumMinMax.java"
	exprEnumInvalidArgsMinMaxBy   = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumMinMaxBy.java"
	exprEnumInvalidArgsOrderBy    = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumOrderBy.java"
	exprEnumInvalidArgsTake       = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumTakeAndTakeLast.java"
	exprEnumInvalidArgsTakeWhile  = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumTakeWhileAndWhileLast.java"
)

var exprEnumInvalidArgsJavaSources = []string{
	exprEnumInvalidArgsMinMaxSrc,
	exprEnumInvalidArgsMinMaxBy,
	exprEnumInvalidArgsOrderBy,
	exprEnumInvalidArgsTake,
	exprEnumInvalidArgsTakeWhile,
}

var exprEnumInvalidArgsJavaRuntimeIDs = []string{
	"java-runtime-67deab0a6d57e2bc57fa",
	"java-runtime-5556be14d552b0228acc",
	"java-runtime-f721caa1c77ad596eca4",
	"java-runtime-3c4f6374416fe5a2ee04",
	"java-runtime-50b5bc269985fbfd9ed2",
}

var exprEnumInvalidArgsJavaStaticIDs = []string{
	"java-167415d6e7af87a7a111",
	"java-4083b006f50db9c59884",
	"java-0277b2cbf963ec07d2c0",
	"java-3f7b6e1e84fe78b4a816",
	"java-0fec7ea37236a9b16857",
}

var exprEnumInvalidArgsJavaExecutions = []string{
	"ExprEnumInvalid",
	"ExprEnumMinMaxByInvalid",
	"ExprEnumOrderByInvalid",
	"ExprEnumTakeInvalid",
	"ExprEnumTakeWhileInvalid",
}

var exprEnumInvalidArgsCases = []string{
	"min-invalid",
	"minby-invalid",
	"orderby-invalid",
	"take-invalid",
	"takewhile-invalid",
}

var exprEnumInvalidArgsCaseOrdinals = []int{4, 2, 4, 2, 2}

// exprEnumInvalidArgsCaseEPLs pins the cases[] metadata EPL (the suite's
// first probe text per execution), separate from the per-step probe map.
var exprEnumInvalidArgsCaseEPLs = []string{
	"select contained.min() from SupportBean_ST0_Container",
	"select contained.minBy(x => null) from SupportBean_ST0_Container",
	"select contained.orderBy() from SupportBean_ST0_Container",
	"select strvals.take(null) from SupportCollection",
	"select strvals.takeWhile(x => null) from SupportCollection",
}

// exprEnumInvalidArgsCaseObservations pins the cases[] observation
// strings the Java oracle also requires (CASE_OBSERVATIONS).
var exprEnumInvalidArgsCaseObservations = []string{
	"compile-error; two tryInvalidCompile probes over SupportBean_ST0_Container pin the 0-parameter-footprint collection-of-events rejection for contained.min() and the Null-type selector rejection for contained.min(x => null)",
	"compile-error; one tryInvalidCompile probe over SupportBean_ST0_Container pins the Null-type selector rejection for contained.minBy(x => null)",
	"compile-error; two tryInvalidCompile probes pin the 0-parameter-footprint collection-of-events rejection for contained.orderBy() over SupportBean_ST0_Container and the Null-type selector rejection for strvals.orderBy(v => null) over SupportCollection",
	"compile-error; one tryInvalidCompile probe over SupportCollection pins the non-null-expression-parameter rejection for strvals.take(null)",
	"compile-error; one tryInvalidCompile probe over SupportCollection pins the non-null-expression-parameter rejection for strvals.takeWhile(x => null)",
}

// exprEnumInvalidArgsProbeEPLs pins each build-error probe's EPL text and
// the Java assertion clause the oracle verifies (byte-exact against the
// suite sources; see .omp/contract-598.md).
var exprEnumInvalidArgsProbeEPLs = map[string]map[string]string{
	"min-invalid": {
		"min-event-input":   "select contained.min() from SupportBean_ST0_Container",
		"min-null-selector": "select contained.min(x => null) from SupportBean_ST0_Container",
	},
	"minby-invalid": {
		"minby-null-selector": "select contained.minBy(x => null) from SupportBean_ST0_Container",
	},
	"orderby-invalid": {
		"orderby-event-input":   "select contained.orderBy() from SupportBean_ST0_Container",
		"orderby-null-selector": "select strvals.orderBy(v => null) from SupportCollection",
	},
	"take-invalid": {
		"take-null-count": "select strvals.take(null) from SupportCollection",
	},
	"takewhile-invalid": {
		"takewhile-null-predicate": "select strvals.takeWhile(x => null) from SupportCollection",
	},
}

type exprEnumInvalidArgsST0 struct {
	ID string `esper:"id"`
}

type exprEnumInvalidArgsContainer struct {
	Contained []exprEnumInvalidArgsST0 `esper:"contained"`
}

type exprEnumInvalidArgsCollection struct {
	Strvals []string `esper:"strvals"`
}

type exprEnumInvalidArgsCaseState struct {
	caseName string
	env      *esper.Environment
	engine   *esper.Engine
	trace    *compat.Trace
}

func runExprEnumInvalidArgsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateExprEnumInvalidArgsScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeExprEnumInvalidArgs(ctx, scenario, &trace)
}

// executeExprEnumInvalidArgs replays the scenario: each case runs on a
// fresh environment/engine pair (one runtime per Java execution) and every
// step dispatches to the matching runtime action.
func executeExprEnumInvalidArgs(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *exprEnumInvalidArgsCaseState
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
			state, err = startExprEnumInvalidArgsCase(step.Case, trace)
			if err != nil {
				return *trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *trace, err
			}
		case "undeploy-all":
			if state.engine != nil {
				if err := state.engine.Close(ctx); err != nil {
					return *trace, err
				}
				state.engine = nil
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", exprEnumInvalidArgsID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startExprEnumInvalidArgsCase builds the fresh per-case environment:
// SupportBean_ST0_Container/SupportBean_ST0/SupportCollection event types
// plus the engine pinned to the case's Java runtime id.
func startExprEnumInvalidArgsCase(caseName string, trace *compat.Trace) (*exprEnumInvalidArgsCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprEnumInvalidArgsST0](env, "SupportBean_ST0"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[exprEnumInvalidArgsContainer](env, "SupportBean_ST0_Container"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[exprEnumInvalidArgsCollection](env, "SupportCollection"); err != nil {
		return nil, err
	}
	state := &exprEnumInvalidArgsCaseState{caseName: caseName, env: env, trace: trace}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(exprEnumInvalidArgsJavaRuntimeIDs[exprEnumInvalidArgsCaseOrdinal(caseName)]))
	return state, nil
}

func exprEnumInvalidArgsCaseOrdinal(caseName string) int {
	for index, name := range exprEnumInvalidArgsCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// buildError runs one expected-invalid probe against the fluent
// equivalent of the pinned EPL. Each probe verifies Go rejects the
// nearest expressible boundary (or is unrepresentable) before recording
// the pinned Java assertion clause.
func (s *exprEnumInvalidArgsCaseState) buildError(step compat.Step) error {
	if pinned, ok := exprEnumInvalidArgsProbeEPLs[s.caseName][step.Statement]; !ok || step.Epl != pinned {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", exprEnumInvalidArgsID, step.Statement, step.Epl)
	}
	contained := esper.Field[exprEnumInvalidArgsContainer, []exprEnumInvalidArgsST0]("contained")
	strvals := esper.Field[exprEnumInvalidArgsCollection, []string]("strvals")
	var buildErr error
	switch step.Statement {
	case "min-event-input":
		// `contained.min()` — Java's 0-parameter footprint accepts an
		// event collection and fails inside validation. Go generics
		// require EnumOrdered elements, so the fluent form cannot be
		// expressed: unrepresentable, pin the clause only.
		buildErr = fmt.Errorf("0-parameter footprint over an event collection is unrepresentable")
	case "min-null-selector":
		// `contained.min(x => null)` — null-typed selector lambda.
		_, buildErr = s.env.Build(esper.Select(
			esper.From[exprEnumInvalidArgsContainer](s.env, "SupportBean_ST0_Container"),
			esper.Alias("c0", esper.EnumMinOf[exprEnumInvalidArgsST0, int64](contained, esper.NullLiteral[int64]())),
		).Query())
	case "minby-null-selector":
		// `contained.minBy(x => null)` — null-typed selector lambda.
		_, buildErr = s.env.Build(esper.Select(
			esper.From[exprEnumInvalidArgsContainer](s.env, "SupportBean_ST0_Container"),
			esper.Alias("c0", esper.EnumMinBy[exprEnumInvalidArgsST0, int64](contained, esper.NullLiteral[int64]())),
		).Query())
	case "orderby-event-input":
		// `contained.orderBy()` — same unrepresentable 0-parameter
		// footprint over an event collection.
		buildErr = fmt.Errorf("0-parameter footprint over an event collection is unrepresentable")
	case "orderby-null-selector":
		// `strvals.orderBy(v => null)` — null-typed selector lambda.
		_, buildErr = s.env.Build(esper.Select(
			esper.From[exprEnumInvalidArgsCollection](s.env, "SupportCollection"),
			esper.Alias("c0", esper.EnumOrderBy[string, int64](strvals, esper.NullLiteral[int64](), false)),
		).Query())
	case "take-null-count":
		// `strvals.take(null)` — the expression-valued count parameter
		// must reject a null-typed expression.
		_, buildErr = s.env.Build(esper.Select(
			esper.From[exprEnumInvalidArgsCollection](s.env, "SupportCollection"),
			esper.Alias("c0", esper.EnumTakeExpr[string](strvals, esper.NullLiteral[int64]())),
		).Query())
	case "takewhile-null-predicate":
		// `strvals.takeWhile(x => null)` — the predicate parameter must
		// reject a null-typed expression.
		_, buildErr = s.env.Build(esper.Select(
			esper.From[exprEnumInvalidArgsCollection](s.env, "SupportCollection"),
			esper.Alias("c0", esper.EnumTakeWhile[string](strvals, esper.NullLiteral[bool]())),
		).Query())
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", exprEnumInvalidArgsID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", exprEnumInvalidArgsID, step.Statement)
	}
	// Verify the Go rejection carries the expected category and wording
	// before recording the pinned Java assertion clause; the
	// unrepresentable probes skip the gate.
	expected := map[string]struct {
		code      esper.ErrorCode
		substring string
	}{
		"min-null-selector":        {esper.ErrorInvalidRule, `enumeration method "min": Null-type is not allowed`},
		"minby-null-selector":      {esper.ErrorInvalidRule, `enumeration method "minBy": Null-type is not allowed`},
		"orderby-null-selector":    {esper.ErrorInvalidRule, `enumeration method "orderBy": Null-type is not allowed`},
		"take-null-count":          {esper.ErrorInvalidRule, `enumeration method "take" expected a non-null result for expression parameter 0 but received a null-typed expression`},
		"takewhile-null-predicate": {esper.ErrorInvalidRule, `enumeration method "takeWhile" expected a non-null result for expression parameter 0 but received a null-typed expression`},
	}
	if want, ok := expected[step.Statement]; ok {
		var espErr *esper.Error
		if !errors.As(buildErr, &espErr) || espErr.Code != want.code || !strings.Contains(buildErr.Error(), want.substring) {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", exprEnumInvalidArgsID, step.Statement, buildErr)
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

func loadExprEnumInvalidArgsScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprEnumInvalidArgsID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprEnumInvalidArgsID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprEnumInvalidArgsID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprEnumInvalidArgsID, err)
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
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprEnumInvalidArgsID, err)
	}
	if metadata.Version != compat.ScenarioVersion ||
		metadata.ID != exprEnumInvalidArgsID ||
		metadata.JavaCommit != exprEnumInvalidArgsJavaCommit ||
		metadata.JavaSource != exprEnumInvalidArgsMinMaxSrc {
		return compat.Scenario{}, fmt.Errorf("%s scenario header drift: %#v", exprEnumInvalidArgsID, metadata)
	}
	if !reflect.DeepEqual(metadata.Sources, exprEnumInvalidArgsJavaSources) {
		return compat.Scenario{}, fmt.Errorf("%s javaSourceFiles drift: %#v", exprEnumInvalidArgsID, metadata.Sources)
	}
	if !reflect.DeepEqual(metadata.Runtimes, exprEnumInvalidArgsJavaRuntimeIDs) {
		return compat.Scenario{}, fmt.Errorf("%s javaRuntimes drift: %#v", exprEnumInvalidArgsID, metadata.Runtimes)
	}
	if !reflect.DeepEqual(metadata.Statics, exprEnumInvalidArgsJavaStaticIDs) {
		return compat.Scenario{}, fmt.Errorf("%s javaStaticIds drift: %#v", exprEnumInvalidArgsID, metadata.Statics)
	}
	if !reflect.DeepEqual(metadata.Names, exprEnumInvalidArgsJavaExecutions) {
		return compat.Scenario{}, fmt.Errorf("%s javaNames drift: %#v", exprEnumInvalidArgsID, metadata.Names)
	}
	if len(metadata.Flags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s javaFlags must be empty: %#v", exprEnumInvalidArgsID, metadata.Flags)
	}
	var cases []struct {
		Case          string `json:"case"`
		Ordinal       int    `json:"ordinal"`
		RuntimeID     string `json:"runtimeId"`
		ExecutionName string `json:"executionName"`
		Epl           string `json:"epl"`
		Observation   string `json:"observation"`
	}
	if err := json.Unmarshal(root["cases"], &cases); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s cases: %w", exprEnumInvalidArgsID, err)
	}
	if len(cases) != len(exprEnumInvalidArgsCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", exprEnumInvalidArgsID, len(exprEnumInvalidArgsCases))
	}
	for index, entry := range cases {
		if entry.Case != exprEnumInvalidArgsCases[index] ||
			entry.Ordinal != exprEnumInvalidArgsCaseOrdinals[index] ||
			entry.RuntimeID != exprEnumInvalidArgsJavaRuntimeIDs[index] ||
			entry.ExecutionName != exprEnumInvalidArgsJavaExecutions[index] ||
			entry.Epl != exprEnumInvalidArgsCaseEPLs[index] ||
			entry.Observation != exprEnumInvalidArgsCaseObservations[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprEnumInvalidArgsID, index)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", exprEnumInvalidArgsID, err)
	}
	// Defense in depth: pin the exact step count (5 case + 7
	// build-error + 5 undeploy-all = 17) and each op's raw field set;
	// every build-error/undeploy-all step's case field must equal the
	// enclosing case marker, matching the Java oracle's stepKey
	// correlation.
	stepFields := map[string][]string{
		"case":         {"op", "case"},
		"build-error":  {"op", "case", "statement", "epl", "expectError", "compileWithoutPath"},
		"undeploy-all": {"op", "case"},
	}
	if len(rawSteps) != 17 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 17 steps, found %d", exprEnumInvalidArgsID, len(rawSteps))
	}
	currentCase := ""
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprEnumInvalidArgsID, index, err)
		}
		var op string
		if err := json.Unmarshal(object["op"], &op); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d op: %w", exprEnumInvalidArgsID, index, err)
		}
		allowed, ok := stepFields[op]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s step %d has unsupported op %q", exprEnumInvalidArgsID, index, op)
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
				return compat.Scenario{}, fmt.Errorf("%s step %d (%s) carries unexpected field %q", exprEnumInvalidArgsID, index, op, field)
			}
		}
		var stepCase string
		if err := json.Unmarshal(object["case"], &stepCase); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d case: %w", exprEnumInvalidArgsID, index, err)
		}
		switch op {
		case "case":
			markerIndex := indexOfExprEnumInvalidArgsMarker(index, rawSteps)
			if markerIndex >= len(exprEnumInvalidArgsCaseOrder()) {
				return compat.Scenario{}, fmt.Errorf("%s step %d case marker %q exceeds the %d pinned cases", exprEnumInvalidArgsID, index, stepCase, len(exprEnumInvalidArgsCaseOrder()))
			}
			if stepCase != exprEnumInvalidArgsCaseOrder()[markerIndex] {
				return compat.Scenario{}, fmt.Errorf("%s step %d case marker %q does not follow the pinned case order", exprEnumInvalidArgsID, index, stepCase)
			}
			currentCase = stepCase
		default:
			if stepCase == "" || stepCase != currentCase {
				return compat.Scenario{}, fmt.Errorf("%s step %d (%s) case %q does not match enclosing case marker %q", exprEnumInvalidArgsID, index, op, stepCase, currentCase)
			}
		}
	}
	for index, rawStep := range rawSteps {
		var step struct {
			Op                 string `json:"op"`
			Statement          string `json:"statement"`
			CompileWithoutPath *bool  `json:"compileWithoutPath"`
		}
		if err := json.Unmarshal(rawStep, &step); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprEnumInvalidArgsID, index, err)
		}
		if step.Op != "build-error" {
			continue
		}
		// Every probe compiles a bare select without a context path, so
		// compileWithoutPath=true is required on all 7.
		if step.CompileWithoutPath == nil || !*step.CompileWithoutPath {
			return compat.Scenario{}, fmt.Errorf("%s step %d build-error probe %q requires compileWithoutPath=true", exprEnumInvalidArgsID, index, step.Statement)
		}
	}
	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprEnumInvalidArgsID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprEnumInvalidArgsID, err)
	}
	for _, step := range scenario.Steps {
		if err := validateExprEnumInvalidArgsStep(step); err != nil {
			return compat.Scenario{}, err
		}
	}
	return scenario, nil
}

func validateExprEnumInvalidArgsStep(step compat.Step) error {
	switch step.Op {
	case "case":
		if !exprEnumInvalidArgsCaseSet()[step.Case] {
			return fmt.Errorf("%s step case %q is not one of %v", exprEnumInvalidArgsID, step.Case, exprEnumInvalidArgsCases)
		}
		if len(step.Payload) != 0 {
			return fmt.Errorf("%s case step %q may not carry a payload", exprEnumInvalidArgsID, step.Case)
		}
	case "build-error", "undeploy-all":
		if !exprEnumInvalidArgsCaseSet()[step.Case] {
			return fmt.Errorf("%s %s step case %q is not one of %v", exprEnumInvalidArgsID, step.Op, step.Case, exprEnumInvalidArgsCases)
		}
		if step.Op == "build-error" {
			pinned, ok := exprEnumInvalidArgsProbeEPLs[step.Case][step.Statement]
			if !ok || step.Epl != pinned {
				return fmt.Errorf("%s build-error step %q carries an unpinned EPL %q", exprEnumInvalidArgsID, step.Statement, step.Epl)
			}
			if strings.TrimSpace(step.ExpectError) == "" {
				return fmt.Errorf("%s build-error step %q requires expectError", exprEnumInvalidArgsID, step.Statement)
			}
		}
	default:
		return fmt.Errorf("%s unsupported step op %q", exprEnumInvalidArgsID, step.Op)
	}
	return nil
}

func exprEnumInvalidArgsCaseSet() map[string]bool {
	set := make(map[string]bool, len(exprEnumInvalidArgsCases))
	for _, name := range exprEnumInvalidArgsCases {
		set[name] = true
	}
	return set
}

func validateExprEnumInvalidArgsScenario(scenario compat.Scenario) error {
	if scenario.ID != exprEnumInvalidArgsID || scenario.Version != compat.ScenarioVersion {
		return fmt.Errorf("%s scenario header drift: %#v", exprEnumInvalidArgsID, scenario)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", exprEnumInvalidArgsID)
	}
	return nil
}

// exprEnumInvalidArgsCaseOrder returns the pinned per-case marker order.
func exprEnumInvalidArgsCaseOrder() []string {
	return exprEnumInvalidArgsCases
}

// indexOfExprEnumInvalidArgsMarker returns how many "case" marker steps
// precede the given step index; each case step must carry
// exprEnumInvalidArgsCases[markerIndex].
func indexOfExprEnumInvalidArgsMarker(stepIndex int, rawSteps []json.RawMessage) int {
	markers := 0
	for cursor := 0; cursor < stepIndex; cursor++ {
		var probe struct {
			Op string `json:"op"`
		}
		if err := json.Unmarshal(rawSteps[cursor], &probe); err == nil && probe.Op == "case" {
			markers++
		}
	}
	return markers
}
