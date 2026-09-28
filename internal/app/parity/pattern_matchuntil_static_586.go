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

// Parity coverage for PatternOperatorMatchUntil ords 7/8: the static
// remainder — the every-bounded and/not repeat plus the compile-error
// probes the fluent API can express. Two Java executions replayed as
// two cases with the direct compileDeploy -> sendEventBean ->
// assertListener harness for ord 7 and the tryInvalidCompile probe
// convention (build-error steps recording compile-error records) for
// ord 8: NO clock calls (milestone() savepoints restore identical state
// for these non-contextual executions and are documented no-ops, so
// they carry no steps).
//
// Covered executions (all variant collection:executions(), flags [],
// file java-2169accb9d43755e87e4):
//   - ord 7 PatternBoundRepeatWithNot java-runtime-f10e941aae4f3b003593
//     (case bound-repeat-with-not):
//     every [2] (e = SupportBean(theString='A') and not
//     SupportBean(theString='B')) — every pending repeat atom carries
//     its own not-guard: A(1)/A(2) complete the first pair and deliver
//     {e:[(A,1),(A,2)]}; A(3) arms a fresh atom; B(4) cancels ONLY that
//     pending partial (the completed first fire already delivered);
//     A(5) arms a new pair which A(6) completes as {e:[(A,5),(A,6)]}.
//     TWO listener fires total.
//   - ord 8 PatternInvalid java-runtime-50cec2b441fd878bf1f1 (case
//     invalid): thirteen tryInvalidPattern legs wrap each fragment as
//     `select * from pattern[<frag>]` and assert a message prefix. The
//     four Go-expressible legs become build-error steps — the inverted
//     [10:4] and negative [-1] bounds rejected at Build, the duplicate
//     tag 'c' declared by the until branch and redeclared by the
//     follow-on atom, and the tag 'a' reused across a nested until —
//     each verifying the Go rejection category before recording the
//     pinned Java message prefix. The remaining nine legs are
//     documented Java-only exclusions with no Go build boundary, so
//     they carry NO steps and NO records:
//   - [:0]/[0:0]/[0] zero-valued literals — Go MatchUntil(0,M) and
//     MatchUntil(N,0) are the legal [:M] and [N:] forms, so a zero
//     bound is an API shape, not a rejectable literal.
//   - [4:6] without until — Go's typed MatchUntil needs no
//     terminator; the "variable bounds repeat operator requires an
//     until-expression" check is EPL-text-only.
//   - [1] a=...(a[0].id='a') and a -> b(a[0].id='a') — indexed
//     tag-array references inside filter expressions are a
//     compile-text surface.
//   - [a.theString]/[:a.theString]/[a.theString:1] bounds —
//     non-numeric bound expressions resolve in the EPL compiler;
//     Go MatchUntilExpr bounds are typed int expressions.
//
// Byte-exact EPL pins keep the Java source verbatim: ord 7's pattern
// carries spaces around `e =` and `and not`; ord 8's build-error steps
// pin the WRAPPED EPL text `select * from pattern[<frag>]` exactly as
// tryInvalidCompile receives it, including the nested-until leg's outer
// parentheses, and the pinned expectError prefixes keep leg 9's
// SupportBean_B fully-qualified class name.
const patternMatchUntilStatic586ID = "pattern-matchuntil-static-586"

const patternMatchUntilStatic586Description = "PatternOperatorMatchUntil ords 7/8 static remainder — ord 7 PatternBoundRepeatWithNot `@name('s0') select * from pattern [every [2] (e = SupportBean(theString='A') and not SupportBean(theString='B'))]` (A(1)/A(2) fire e={1,2}, A(3) arms a fresh atom, B(4) cancels only that pending partial, A(5)/A(6) fire e={5,6} — each repeat atom carries its own not-guard and the completed first delivery survives); ord 8 PatternInvalid pins the four Go-expressible tryInvalidCompile legs (inverted [10:4], negative [-1], duplicate tag across until/follow-on, nested-until tag reuse) as compile-error records carrying the Java message prefixes — the other nine legs are documented Java-only exclusions (zero-valued literals are the legal Go [:M]/[N:] forms, [4:6]-without-until has no EPL-text surface, a[0] own/follow-on filter refs and non-numeric bound expressions are compile-text concerns) and carry no steps. No clock ops; milestone() savepoints carry no steps."

const patternMatchUntilStatic586JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternMatchUntilStatic586JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorMatchUntil.java"

// Byte-exact EPL pins (PatternOperatorMatchUntil.java verbatim): ord 7
// keeps `e = SupportBean(theString='A') and not SupportBean(theString='B')`
// inside `every [2] (...)`; ord 8 pins the wrapped `select * from
// pattern[...]` text that env.tryInvalidCompile compiles (tryInvalidPattern
// concatenates the fragment verbatim).
const (
	patternMatchUntilStatic586BoundRepeatEPL = "@name('s0') select * from pattern [every [2] (e = SupportBean(theString='A') and not SupportBean(theString='B'))]"

	patternMatchUntilStatic586InvertedBoundsEPL = "select * from pattern[[10:4] SupportBean_A]"
	patternMatchUntilStatic586NegativeBoundsEPL = "select * from pattern[[-1] SupportBean_A]"
	patternMatchUntilStatic586TagAcrossUntilEPL = "select * from pattern[(a=SupportBean_A until c=SupportBean_B) -> c=SupportBean_C]"
	patternMatchUntilStatic586TagNestedUntilEPL = "select * from pattern[((a=SupportBean_A until b=SupportBean_B) until a=SupportBean_A)]"

	patternMatchUntilStatic586InvertedBoundsError = "Incorrect range specification, lower bounds value '10' is higher then higher bounds '4'"
	patternMatchUntilStatic586NegativeBoundsError = "Incorrect range specification, a bounds value of zero or negative value is not allowed"
	patternMatchUntilStatic586TagAcrossUntilError = "Tag 'c' for event 'SupportBean_C' has already been declared for events of type com.espertech.esper.regressionlib.support.bean.SupportBean_B"
	patternMatchUntilStatic586TagNestedUntilError = "Tag 'a' for event 'SupportBean_A' used in the repeat-until operator cannot also appear in other filter expressions"

	patternMatchUntilStatic586BeanType = "SupportBean"
	patternMatchUntilStatic586AType    = "SupportBean_A"
	patternMatchUntilStatic586BType    = "SupportBean_B"
	patternMatchUntilStatic586CType    = "SupportBean_C"
)

// Per-case Java identity: each case owns its ordinal's runtimeId; the
// static ids are the per-execution discovery static-candidate rows.
var (
	patternMatchUntilStatic586JavaRuntimeIDs = []string{
		"java-runtime-f10e941aae4f3b003593",
		"java-runtime-50cec2b441fd878bf1f1",
	}
	patternMatchUntilStatic586JavaSources = []string{
		patternMatchUntilStatic586JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_C.java",
	}
	patternMatchUntilStatic586JavaExecutions = []string{
		"PatternBoundRepeatWithNot",
		"PatternInvalid",
	}
	patternMatchUntilStatic586JavaStaticIDs = []string{
		"java-ada27e5f94fecc9ffce2",
		"java-256196856cfafb3c17b7",
	}
	patternMatchUntilStatic586JavaFlags = []string{}
)

// patternMatchUntilStatic586Bean mirrors SupportBean's asserted
// properties: theString and intPrimitive (the only fields the Java
// assertions pin via e[0].intPrimitive/e[1].intPrimitive and the event
// payloads).
type patternMatchUntilStatic586Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// patternMatchUntilStatic586A/B/C mirror the SupportBean_A/B/C letter
// beans: a single id property each. The ord 8 probes reference the
// letter beans so the invalid case registers them.
type patternMatchUntilStatic586A struct {
	ID string `json:"id" esper:"id"`
}

type patternMatchUntilStatic586B struct {
	ID string `json:"id" esper:"id"`
}

type patternMatchUntilStatic586C struct {
	ID string `json:"id" esper:"id"`
}

// patternMatchUntilStatic586CaseSpec pins one case: Java execution
// identity (ordinal plus runtimeIndex), the observation text and the
// byte-exact EPL scripts in step order (case.epl in the scenario pins
// the first script as the representative — ord 8's first modeled probe).
type patternMatchUntilStatic586CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epls         []string
}

var patternMatchUntilStatic586CaseSpecs = []patternMatchUntilStatic586CaseSpec{
	{
		name:         "bound-repeat-with-not",
		ordinal:      7,
		runtimeIndex: 0,
		observation: "listener x2; no clock ops (milestone 0-3 savepoints carry no steps); " +
			"every [2] (e=SupportBean(theString='A') and not SupportBean(theString='B')): " +
			"A(1)/A(2) complete the first pair and fire {e:[(A,1),(A,2)]}; A(3) arms a fresh " +
			"repeat atom; B(4) cancels ONLY that pending partial — the completed first " +
			"delivery already left the engine; A(5) arms a new pair which A(6) completes, " +
			"firing {e:[(A,5),(A,6)]}; each pending repeat atom carries its own not-guard",
		epls: []string{patternMatchUntilStatic586BoundRepeatEPL},
	},
	{
		name:         "invalid",
		ordinal:      8,
		runtimeIndex: 1,
		observation: "compile-error; four tryInvalidCompile probes pin the Go-expressible legs — " +
			"inverted [10:4] (lower bound higher than upper), negative [-1] bound, duplicate " +
			"tag 'c' redeclared across until and follow-on, tag 'a' reused inside a nested " +
			"until — recording the pinned Java message prefixes; the remaining nine legs are " +
			"documented Java-only exclusions (zero-valued [:0]/[0:0]/[0] literals are legal Go " +
			"[:M]/[N:] forms, [4:6]-without-until is EPL-text-only, a[0].id own/follow-on " +
			"filter refs and the non-numeric bound expressions are compile-text surfaces) " +
			"with no steps and no records",
		epls: []string{
			patternMatchUntilStatic586InvertedBoundsEPL,
			patternMatchUntilStatic586NegativeBoundsEPL,
			patternMatchUntilStatic586TagAcrossUntilEPL,
			patternMatchUntilStatic586TagNestedUntilEPL,
		},
	},
}

// patternMatchUntilStatic586StepPin pins one scenario step's shape:
// deploys carry the byte-exact EPL, sends pin the full payload and
// build-error probes pin the wrapped EPL plus the expected Java message
// prefix (all ord 8 probes mirror tryInvalidCompile's path-less
// compileWCheckedEx, so no path marker is needed). There are NO
// advance-time steps and NO milestone steps — the Java executions
// perform no clock calls.
type patternMatchUntilStatic586StepPin struct {
	op          string
	statement   string
	epl         string
	eventType   string
	payload     map[string]any
	expectError string
}

func patternMatchUntilStatic586DeployPin(statement, epl string) patternMatchUntilStatic586StepPin {
	return patternMatchUntilStatic586StepPin{op: "deploy", statement: statement, epl: epl}
}

func patternMatchUntilStatic586SendPin(eventType string, payload map[string]any) patternMatchUntilStatic586StepPin {
	return patternMatchUntilStatic586StepPin{op: "send", eventType: eventType, payload: payload}
}

// patternMatchUntilStatic586BeanSendPin pins one SupportBean send:
// theString plus intPrimitive.
func patternMatchUntilStatic586BeanSendPin(theString string, intPrimitive int64) patternMatchUntilStatic586StepPin {
	return patternMatchUntilStatic586SendPin(patternMatchUntilStatic586BeanType, map[string]any{
		"theString":    theString,
		"intPrimitive": float64(intPrimitive),
	})
}

// patternMatchUntilStatic586BuildErrorPin pins one tryInvalidCompile
// probe: the wrapped EPL and the asserted Java message prefix.
func patternMatchUntilStatic586BuildErrorPin(statement, epl, expectError string) patternMatchUntilStatic586StepPin {
	return patternMatchUntilStatic586StepPin{
		op: "build-error", statement: statement, epl: epl,
		expectError: expectError,
	}
}

func patternMatchUntilStatic586UndeployAllPin() patternMatchUntilStatic586StepPin {
	return patternMatchUntilStatic586StepPin{op: "undeploy-all"}
}

// patternMatchUntilStatic586CaseSteps pins the complete step sequence
// per case in Java source order: ord 7 deploys s0, sends the six
// SupportBean events and undeploys; ord 8 is compile-only with one
// build-error step per Go-expressible probe (Java source order of the
// modeled legs: [10:4], [-1], until/follow-on tag, nested-until tag).
var patternMatchUntilStatic586CaseSteps = map[string][]patternMatchUntilStatic586StepPin{
	"bound-repeat-with-not": {
		patternMatchUntilStatic586DeployPin("s0", patternMatchUntilStatic586BoundRepeatEPL),
		patternMatchUntilStatic586BeanSendPin("A", 1),
		patternMatchUntilStatic586BeanSendPin("A", 2),
		patternMatchUntilStatic586BeanSendPin("A", 3),
		patternMatchUntilStatic586BeanSendPin("B", 4),
		patternMatchUntilStatic586BeanSendPin("A", 5),
		patternMatchUntilStatic586BeanSendPin("A", 6),
		patternMatchUntilStatic586UndeployAllPin(),
	},
	"invalid": {
		patternMatchUntilStatic586BuildErrorPin("inverted-bounds",
			patternMatchUntilStatic586InvertedBoundsEPL, patternMatchUntilStatic586InvertedBoundsError),
		patternMatchUntilStatic586BuildErrorPin("negative-bounds",
			patternMatchUntilStatic586NegativeBoundsEPL, patternMatchUntilStatic586NegativeBoundsError),
		patternMatchUntilStatic586BuildErrorPin("tag-redeclared-across-until",
			patternMatchUntilStatic586TagAcrossUntilEPL, patternMatchUntilStatic586TagAcrossUntilError),
		patternMatchUntilStatic586BuildErrorPin("tag-reused-inside-nested-until",
			patternMatchUntilStatic586TagNestedUntilEPL, patternMatchUntilStatic586TagNestedUntilError),
	},
}

func loadPatternMatchUntilStatic586Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternMatchUntilStatic586ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternMatchUntilStatic586ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternMatchUntilStatic586ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternMatchUntilStatic586ID, err)
	}
	if err := requirePatternMatchUntilStatic586Fields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaSourceFiles",
		"javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version        string   `json:"version"`
		ID             string   `json:"id"`
		Description    string   `json:"description"`
		JavaCommit     string   `json:"javaCommit"`
		JavaSource     string   `json:"javaSource"`
		JavaSourceFile []string `json:"javaSourceFiles"`
		JavaRuntimes   []string `json:"javaRuntimes"`
		JavaNames      []string `json:"javaNames"`
		JavaStaticIDs  []string `json:"javaStaticIds"`
		JavaFlags      []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternMatchUntilStatic586ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternMatchUntilStatic586ID ||
		metadata.Description != patternMatchUntilStatic586Description ||
		metadata.JavaCommit != patternMatchUntilStatic586JavaCommit ||
		metadata.JavaSource != patternMatchUntilStatic586JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternMatchUntilStatic586ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternMatchUntilStatic586JavaSources},
		{metadata.JavaRuntimes, patternMatchUntilStatic586JavaRuntimeIDs},
		{metadata.JavaNames, patternMatchUntilStatic586JavaExecutions},
		{metadata.JavaStaticIDs, patternMatchUntilStatic586JavaStaticIDs},
		{metadata.JavaFlags, patternMatchUntilStatic586JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternMatchUntilStatic586ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternMatchUntilStatic586CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternMatchUntilStatic586ID, len(patternMatchUntilStatic586CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternMatchUntilStatic586Fields(object, "case", "ordinal", "runtimeId",
			"executionName", "observation", "epl"); err != nil {
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
		spec := patternMatchUntilStatic586CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternMatchUntilStatic586JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternMatchUntilStatic586JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || definition.EPL != spec.epls[0] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternMatchUntilStatic586ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternMatchUntilStatic586ID)
	}
	if err := validatePatternMatchUntilStatic586RawSteps(rawSteps); err != nil {
		return compat.Scenario{}, err
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validatePatternMatchUntilStatic586RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned per-case sequences — ord 7's s0 deploy with the
// verbatim EPL, the six pinned sends and undeploy-all; ord 8's four
// build-error probes with the wrapped EPL and the Java message prefix.
// There are no advance-time or milestone steps because the Java
// executions perform no clock calls.
func validatePatternMatchUntilStatic586RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternMatchUntilStatic586CaseSpecs))
	positions := make(map[string]int)
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return fmt.Errorf("scenario step %d op must be a string: %w", index, err)
		}
		var step struct {
			Case        string          `json:"case"`
			Statement   string          `json:"statement"`
			EPL         string          `json:"epl"`
			ExpectError string          `json:"expectError"`
			EventType   string          `json:"eventType"`
			Payload     json.RawMessage `json:"payload"`
		}
		if operation == "case" {
			if err := requirePatternMatchUntilStatic586Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternMatchUntilStatic586CaseSteps[marker.Case]; !ok {
				return fmt.Errorf("scenario step %d selects unknown case %q", index, marker.Case)
			}
			currentCase = marker.Case
			caseOrder = append(caseOrder, marker.Case)
			continue
		}
		if currentCase == "" {
			return fmt.Errorf("scenario step %d is outside any case block", index)
		}
		if err := json.Unmarshal(rawStep, &step); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		if step.Case != currentCase {
			return fmt.Errorf("scenario step %d declares case %q inside the %q block",
				index, step.Case, currentCase)
		}
		pins := patternMatchUntilStatic586CaseSteps[currentCase]
		position := positions[currentCase]
		if position >= len(pins) {
			return fmt.Errorf("scenario step %d exceeds the pinned %s step sequence",
				index, currentCase)
		}
		pin := pins[position]
		if operation != pin.op {
			return fmt.Errorf("scenario step %d op %q is not the pinned %q for case %q position %d",
				index, operation, pin.op, currentCase, position)
		}
		switch operation {
		case "deploy":
			if err := requirePatternMatchUntilStatic586Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternMatchUntilStatic586Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "build-error":
			if err := requirePatternMatchUntilStatic586Fields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl ||
				step.ExpectError != pin.expectError {
				return fmt.Errorf("scenario step %d build-error is not pinned for case %q", index, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternMatchUntilStatic586Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternMatchUntilStatic586CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternMatchUntilStatic586CaseSpecs))
	}
	for index, spec := range patternMatchUntilStatic586CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternMatchUntilStatic586CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternMatchUntilStatic586CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternMatchUntilStatic586Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has %d fields, want %d", len(object), len(names))
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

// patternMatchUntilStatic586CaseState carries per-case replay state:
// the deployed statement and the delivery records.
type patternMatchUntilStatic586CaseState struct {
	spec       patternMatchUntilStatic586CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	deployment *esper.Deployment
	records    []compat.TraceRecord
}

// runPatternMatchUntilStatic586Scenario replays the executions against
// a fresh engine per case like the Java oracle's fresh per-execution
// runtime. The engine clock stays at epoch — the scenario carries no
// advance-time steps and milestone() savepoints are documented no-ops.
func runPatternMatchUntilStatic586Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternMatchUntilStatic586Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternMatchUntilStatic586CaseSpecs {
		records, err := runPatternMatchUntilStatic586Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternMatchUntilStatic586ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternMatchUntilStatic586Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternMatchUntilStatic586ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternMatchUntilStatic586ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternMatchUntilStatic586RawSteps(rawSteps)
}

func runPatternMatchUntilStatic586Case(ctx context.Context, scenario compat.Scenario, spec patternMatchUntilStatic586CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternMatchUntilStatic586Bean](env, patternMatchUntilStatic586BeanType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilStatic586A](env, patternMatchUntilStatic586AType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilStatic586B](env, patternMatchUntilStatic586BType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilStatic586C](env, patternMatchUntilStatic586CType); err != nil {
		return nil, err
	}
	state := &patternMatchUntilStatic586CaseState{
		spec:   spec,
		env:    env,
		engine: esper.NewEngine(env, esper.WithRuntimeURI(patternMatchUntilStatic586JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
	}
	defer func() { _ = state.engine.Close(context.Background()) }()

	inCase := false
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			inCase = step.Case == spec.name
			continue
		}
		if !inCase {
			continue
		}
		switch step.Op {
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return nil, err
			}
		case "send":
			payload, err := patternMatchUntilStatic586DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return nil, err
			}
		case "undeploy-all":
			if state.deployment != nil {
				if err := state.deployment.Undeploy(ctx); err != nil {
					return nil, err
				}
				state.deployment = nil
			}
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternMatchUntilStatic586ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned ord 7 EPL onto the fluent Go equivalent:
// every [2] (e=SupportBean(theString='A') and not
// SupportBean(theString='B')) is
// PatternFrom(e, theString='A').And(PatternFrom(theString='B').Not())
// .MatchUntil(2,2).Every() — the not-branch atom is UNTAGGED like the
// Java EPL (it binds no tag and participates only as the and/not
// guard). select * expands to the collected array column e via
// TagEvents, matching the Java wildcard fragments (e[0].intPrimitive /
// e[1].intPrimitive assertions ride the same bean array). The listener
// sequence counter starts at the deploy, mirroring the Java oracle's
// per-deployment TraceWriter.
func (s *patternMatchUntilStatic586CaseState) deploy(ctx context.Context, step compat.Step) error {
	if s.spec.name != "bound-repeat-with-not" || step.Statement != "s0" ||
		step.Epl != patternMatchUntilStatic586BoundRepeatEPL {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternMatchUntilStatic586ID, s.spec.name)
	}
	bean := esper.From[patternMatchUntilStatic586Bean](s.env, patternMatchUntilStatic586BeanType)
	theString := esper.Field[patternMatchUntilStatic586Bean, string]("theString")
	pattern := esper.PatternFrom(bean, "e",
		esper.Equal[string](theString, esper.Literal("A"))).
		And(esper.PatternFrom(bean, "",
			esper.Equal[string](theString, esper.Literal("B"))).Not()).
		MatchUntil(2, 2).
		Every()
	plan, err := s.env.Build(pattern.Select(
		esper.Alias("e", esper.TagEvents("e")),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return err
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	statements := deployment.Statements()
	if len(statements) != 1 || statements[0].Name() != "s0" {
		return fmt.Errorf("%s: expected one s0 statement", patternMatchUntilStatic586ID)
	}
	statement := statements[0]
	s.deployment = deployment
	var sequence uint64
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		sequence++
		s.records = append(s.records, compat.TraceRecord{
			Case:      s.spec.name,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// buildError runs one expected-invalid ord 8 probe against the fluent
// equivalent of the pinned EPL, mirroring
// internal/esper/pattern_matchuntil_parity_test.go's
// TestPatternMatchUntilInvalidMatchesEsper legs:
//
//   - inverted-bounds: [10:4] -> MatchUntil(10,4) rejected because the
//     maximum is below the minimum.
//   - negative-bounds: [-1] -> MatchUntil(-1,-1) rejected because the
//     minimum is negative.
//   - tag-redeclared-across-until: (a until c) -> c -> the follow-on
//     atom redeclares the terminator's tag.
//   - tag-reused-inside-nested-until: ((a until b) until a) -> the
//     nested until's terminator reuses the repeated tag.
//
// Each probe verifies the Go rejection carries the expected category and
// wording before recording the pinned Java message prefix, mirroring the
// context-key-segmented-invalid gate.
func (s *patternMatchUntilStatic586CaseState) buildError(step compat.Step) error {
	if s.spec.name != "invalid" {
		return fmt.Errorf("%s: case %q has no build-error steps", patternMatchUntilStatic586ID, s.spec.name)
	}
	trueExpr := esper.Literal(true)
	streamA := esper.From[patternMatchUntilStatic586A](s.env, patternMatchUntilStatic586AType)
	streamB := esper.From[patternMatchUntilStatic586B](s.env, patternMatchUntilStatic586BType)
	streamC := esper.From[patternMatchUntilStatic586C](s.env, patternMatchUntilStatic586CType)
	var buildErr error
	switch step.Statement {
	case "inverted-bounds":
		if step.Epl != patternMatchUntilStatic586InvertedBoundsEPL ||
			step.ExpectError != patternMatchUntilStatic586InvertedBoundsError {
			return fmt.Errorf("%s: build-error probe %q is not pinned", patternMatchUntilStatic586ID, step.Statement)
		}
		_, buildErr = s.env.Build(esper.PatternFrom(streamA, "a", trueExpr).
			MatchUntil(10, 4).
			Select(esper.Alias("n", esper.TagCount("a"))).Query())
	case "negative-bounds":
		if step.Epl != patternMatchUntilStatic586NegativeBoundsEPL ||
			step.ExpectError != patternMatchUntilStatic586NegativeBoundsError {
			return fmt.Errorf("%s: build-error probe %q is not pinned", patternMatchUntilStatic586ID, step.Statement)
		}
		_, buildErr = s.env.Build(esper.PatternFrom(streamA, "a", trueExpr).
			MatchUntil(-1, -1).
			Select(esper.Alias("n", esper.TagCount("a"))).Query())
	case "tag-redeclared-across-until":
		if step.Epl != patternMatchUntilStatic586TagAcrossUntilEPL ||
			step.ExpectError != patternMatchUntilStatic586TagAcrossUntilError {
			return fmt.Errorf("%s: build-error probe %q is not pinned", patternMatchUntilStatic586ID, step.Statement)
		}
		repeated := esper.PatternFrom(streamA, "a", trueExpr).
			Until(esper.PatternFrom(streamB, "c", trueExpr))
		_, buildErr = s.env.Build(repeated.
			Then(esper.PatternFrom(streamC, "c", trueExpr)).
			Select(esper.Alias("n", esper.TagCount("a"))).Query())
	case "tag-reused-inside-nested-until":
		if step.Epl != patternMatchUntilStatic586TagNestedUntilEPL ||
			step.ExpectError != patternMatchUntilStatic586TagNestedUntilError {
			return fmt.Errorf("%s: build-error probe %q is not pinned", patternMatchUntilStatic586ID, step.Statement)
		}
		inner := esper.PatternFrom(streamA, "a", trueExpr).
			Until(esper.PatternFrom(streamB, "b", trueExpr))
		_, buildErr = s.env.Build(inner.
			Until(esper.PatternFrom(streamA, "a", trueExpr)).
			Select(esper.Alias("n", esper.TagCount("a"))).Query())
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", patternMatchUntilStatic586ID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", patternMatchUntilStatic586ID, step.Statement)
	}
	expected := map[string]string{
		"inverted-bounds":                "match-until maximum must be at least minimum",
		"negative-bounds":                "match-until minimum must not be negative",
		"tag-redeclared-across-until":    `pattern duplicates tag "c"`,
		"tag-reused-inside-nested-until": `pattern duplicates tag "a"`,
	}
	var espErr *esper.Error
	if !errors.As(buildErr, &espErr) || espErr.Code != esper.ErrorInvalidRule ||
		!strings.Contains(buildErr.Error(), expected[step.Statement]) {
		return fmt.Errorf("%s: build-error probe %q drift: got %v", patternMatchUntilStatic586ID, step.Statement, buildErr)
	}
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.spec.name,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// patternMatchUntilStatic586DecodePayload converts a scenario send
// payload into the typed host object for the event type. Only
// SupportBean sends exist (ord 7); the letter beans are registered for
// the ord 8 probes.
func patternMatchUntilStatic586DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case patternMatchUntilStatic586BeanType:
		if err := requirePatternMatchUntilStatic586Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var event patternMatchUntilStatic586Bean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternMatchUntilStatic586ID, step.EventType)
	}
}
