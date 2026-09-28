package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for PatternOperatorMatchUntil ord 4 PatternRepeatUseTags:
// ONE Java execution replayed as THREE sequential deploy/undeploy-all legs
// inside a single case under one runtimeId, matching the single-run
// undeployAll semantics (the same ONE-case/multi-deploy convention as
// pattern-followedby-chain-582 ord 8). The legs exercise correlated
// repeats under every, a timed until/every chain under the external
// clock, and an indexed A[0] first-element read across a three-stream
// repeat. env.milestone() savepoints restore identical state and carry
// no steps; env.advanceTime calls become advance-time steps.
//
// Covered execution (variant collection:executions(), flags [],
// static id java-b2c644fc2f9603bd8568):
//   - ord 4 PatternRepeatUseTags java-runtime-eb238a96331acf6fc30b
//     (case repeat-use-tags), three legs:
//     leg 1 `every [2] (a=SupportBean_A() -> b=SupportBean_B(id=a.id))`:
//     sends A(id=A1), B(id=A1), A(id=A2), B(id=A2) and fires ONCE —
//     each [2]-repeat iteration opens on an `a` whose captured event
//     carries through to the correlated b filter `id=a.id`, so a
//     mismatched B can never cross-complete another iteration
//     (per-iteration correlation scope is the pinned discriminant).
//     The completed repeat delivers {a:[A1,A2], b:[B(A1),B(A2)]} via
//     select *.
//     leg 2 `every ([2:]e1=SupportBean(theString='2') until
//     timer:interval(5))->([2:]e2=SupportBean(theString='3') until
//     timer:interval(2))` under the external clock: advanceTime(0)
//     precedes the deploy, then sends 2,2 / advance 5000 / sends
//     3,3,3,3 / advance 10000 / sends 2,2 / advance 15000. In Esper
//     the until expression is a SUCCESSFUL terminator — the e1
//     until-timer at t=5000 completes its repeat delivering the two
//     collected '2's into the sequence and arms e2, whose own
//     until-timer expires at t=7000 completing with all four
//     collected '3's — the firing lands INSIDE advanceTime(10000):
//     Esper sets the clock to the target before processing due
//     schedules, so the delivery records ONE listener record at
//     t=10000. Java asserts nothing on this leg (the
//     assertListenerInvoked calls bookend only legs 1 and 3), so
//     the trace must reproduce the oracle delivery verbatim; any
//     extra or missing delivery is the parity discriminant.
//     -> [2] C=SupportBean(theString='3' and intPrimitive=A[0].
//     intPrimitive)]` (DOUBLE SPACE after `pattern [` from source
//     concatenation): sends (1,10), (1,20), (2,10)x2, (3,10)x2 and
//     fires ONCE at t=15000 — B and C read A[0].intPrimitive, the
//     FIRST element of the completed [2] A-repeat (10), deliberately
//     disambiguated from the nearest A value 20.
//
// Byte-exact EPL pins keep the Java source verbatim: leg 1 keeps the
// empty parens `SupportBean_A()` and the UNQUOTED correlation
// `id=a.id`; leg 2 keeps the `[2:]` open ranges and the parenthesized
// until-branches inside `every (...)->(...)`; leg 3 keeps the double
// space after `pattern [` plus the `A[0].intPrimitive` indexed reads.
//
// KNOWN SURFACE NUANCE (flagged for review): the Go fluent build
// materializes the `every` root on the FIRST pattern operand only, so
// leg 2 becomes `every(e1-until) -> (e2-until)` and leg 3 becomes
// `every([2]A) -> [2]B -> [2]C`, whereas the Java EPL scopes `every`
// over the whole sequence. For leg 2 the observable consequence is
// only WHEN the next e1 atom re-arms (on each e1-unit quiesce for Go
// vs on full-sequence completion for Java); the leg produces no
// further deliveries after the t=10000 firing either way under the
// pinned sends, so the nuance is unobservable in this lifecycle leg —
// it is documented, not "fixed". Leg 3's single completed repeat
// likewise cannot distinguish the scopes.
const patternMatchUntilRepeatTags587ID = "pattern-matchuntil-repeattags-587"

const patternMatchUntilRepeatTags587Description = "PatternOperatorMatchUntil ord 4 PatternRepeatUseTags — ONE execution replayed as THREE sequential deploy/undeploy-all legs under one runtimeId. Leg 1 `every [2] (a=SupportBean_A() -> b=SupportBean_B(id=a.id))` sends A1/B(A1)/A2/B(A2) and fires ONCE: the correlated b filter reads the a that opened THAT repeat iteration (per-iteration scope is the discriminant — a non-matching B must not cross-complete another iteration). Leg 2 `every ([2:]e1=SupportBean(theString='2') until timer:interval(5))->([2:]e2=SupportBean(theString='3') until timer:interval(2))` runs under the external clock — advanceTime(0) before the deploy, sends 2,2 / advance 5000 / sends 3,3,3,3 / advance 10000 / sends 2,2 / advance 15000: Esper's until is a SUCCESSFUL terminator, so the e1 repeat completes at t=5000 delivering its two '2's into the sequence and the e2 repeat completes when its until-timer expires at t=7000 with the four collected '3's — the firing lands inside advanceTime(10000), which sets the clock to the target before processing due schedules, so ONE listener record at t=10000 (Java asserts nothing for this leg, so any delivery must match the oracle verbatim). Leg 3 `pattern [ every [2] A=SupportBean(theString='1') -> [2] B=SupportBean(theString='2' and intPrimitive=A[0].intPrimitive)-> [2] C=SupportBean(theString='3' and intPrimitive=A[0].intPrimitive)]` (double space after `pattern [` from source concatenation) sends (1,10)/(1,20)/(2,10)x2/(3,10)x2 and fires ONCE: B and C read A[0].intPrimitive — the FIRST repeat element 10, not the nearest A value 20. milestone() savepoints carry no steps."

const patternMatchUntilRepeatTags587JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternMatchUntilRepeatTags587JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorMatchUntil.java"

// Byte-exact EPL pins (PatternOperatorMatchUntil.java verbatim, leg
// order): leg 1 keeps `SupportBean_A()` empty parens and the unquoted
// `id=a.id` reference; leg 2 keeps `[2:]`, `until timer:interval(...)`
// and the parenthesized branches inside `every (...)->(...)`; leg 3
// keeps the double space after `pattern [` produced by the source
// concatenation "pattern [ every [2] A=..." plus the A[0].intPrimitive
// first-element reads.
const (
	patternMatchUntilRepeatTags587Leg1EPL = "@name('s0') select * from pattern [every [2] (a=SupportBean_A() -> b=SupportBean_B(id=a.id))]"
	patternMatchUntilRepeatTags587Leg2EPL = "@name('s0') select * from pattern [every ([2:]e1=SupportBean(theString='2') until timer:interval(5))->([2:]e2=SupportBean(theString='3') until timer:interval(2))]"
	patternMatchUntilRepeatTags587Leg3EPL = "@name('s0') select * from pattern [ every [2] A=SupportBean(theString='1') -> [2] B=SupportBean(theString='2' and intPrimitive=A[0].intPrimitive)-> [2] C=SupportBean(theString='3' and intPrimitive=A[0].intPrimitive)]"

	patternMatchUntilRepeatTags587BeanType = "SupportBean"
	patternMatchUntilRepeatTags587AType    = "SupportBean_A"
	patternMatchUntilRepeatTags587BType    = "SupportBean_B"
)

// Per-case Java identity: the single case owns ord 4's runtimeId.
var (
	patternMatchUntilRepeatTags587JavaRuntimeIDs = []string{
		"java-runtime-eb238a96331acf6fc30b",
	}
	patternMatchUntilRepeatTags587JavaSources = []string{
		patternMatchUntilRepeatTags587JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
	}
	patternMatchUntilRepeatTags587JavaExecutions = []string{
		"PatternRepeatUseTags",
	}
	patternMatchUntilRepeatTags587JavaStaticIDs = []string{
		"java-b2c644fc2f9603bd8568",
	}
	patternMatchUntilRepeatTags587JavaFlags = []string{}
)

// patternMatchUntilRepeatTags587Bean mirrors SupportBean's asserted
// properties: theString and intPrimitive (legs 2/3 pin both fields).
type patternMatchUntilRepeatTags587Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// patternMatchUntilRepeatTags587A/B mirror the SupportBean_A/B letter
// beans used by leg 1: a single id property each.
type patternMatchUntilRepeatTags587A struct {
	ID string `json:"id" esper:"id"`
}

type patternMatchUntilRepeatTags587B struct {
	ID string `json:"id" esper:"id"`
}

// patternMatchUntilRepeatTags587CaseSpec pins the single case: Java
// execution identity (ordinal plus runtimeIndex), the observation text
// and the byte-exact EPL scripts in deploy order — one per leg.
type patternMatchUntilRepeatTags587CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epls         []string
}

var patternMatchUntilRepeatTags587CaseSpecs = []patternMatchUntilRepeatTags587CaseSpec{
	{
		name:         "repeat-use-tags",
		ordinal:      4,
		runtimeIndex: 0,
		observation: "listener x3; ONE execution, THREE sequential deploy/undeploy-all legs under one " +
			"runtimeId: leg 1 `every [2] (a=SupportBean_A() -> b=SupportBean_B(id=a.id))` fires ONCE " +
			"at t=0 — sends A(id=A1), B(id=A1), A(id=A2), B(id=A2) deliver {a:[A1,A2], " +
			"b:[B(A1),B(A2)]}; the b predicate `id=a.id` reads the a that opened that same repeat " +
			"iteration (per-iteration correlation scope); leg 2 `every ([2:]e1='2' until " +
			"timer:interval(5))->([2:]e2='3' until timer:interval(2))` under advance-time " +
			"0/5000/10000/15000 — the e1 until-timer completes its repeat at t=5000 (two collected " +
			"'2's), arming e2 whose own until-timer expires at t=7000 with all four collected " +
			"'3's — the firing lands inside advanceTime(10000) so ONE record at t=10000 (Java has " +
			"no listener assert on this leg); leg 3 `every " +
			"[2] A=SupportBean('1') -> [2] B=SupportBean('2' and intPrimitive=A[0].intPrimitive)" +
			"-> [2] C=SupportBean('3' and intPrimitive=A[0].intPrimitive)` fires ONCE at t=15000 — B " +
			"and C correlate on A[0].intPrimitive, the first repeat element (10), deliberately " +
			"distinguishable from A[1]=20; milestone 0-3 savepoints carry no steps",
		epls: []string{
			patternMatchUntilRepeatTags587Leg1EPL,
			patternMatchUntilRepeatTags587Leg2EPL,
			patternMatchUntilRepeatTags587Leg3EPL,
		},
	},
}

// patternMatchUntilRepeatTags587StepPin pins one scenario step's shape:
// deploys carry the byte-exact EPL, advance-time steps pin the event
// instant (RFC3339 UTC like every other parity scenario) and sends pin
// the full payload. milestone() savepoints carry no steps.
type patternMatchUntilRepeatTags587StepPin struct {
	op        string
	statement string
	epl       string
	at        string
	eventType string
	payload   map[string]any
}

func patternMatchUntilRepeatTags587DeployPin(statement, epl string) patternMatchUntilRepeatTags587StepPin {
	return patternMatchUntilRepeatTags587StepPin{op: "deploy", statement: statement, epl: epl}
}

func patternMatchUntilRepeatTags587AdvancePin(at string) patternMatchUntilRepeatTags587StepPin {
	return patternMatchUntilRepeatTags587StepPin{op: "advance-time", at: at}
}

func patternMatchUntilRepeatTags587SendPin(eventType string, payload map[string]any) patternMatchUntilRepeatTags587StepPin {
	return patternMatchUntilRepeatTags587StepPin{op: "send", eventType: eventType, payload: payload}
}

func patternMatchUntilRepeatTags587IDSendPin(eventType, id string) patternMatchUntilRepeatTags587StepPin {
	return patternMatchUntilRepeatTags587SendPin(eventType, map[string]any{"id": id})
}

// patternMatchUntilRepeatTags587BeanSendPin pins one SupportBean send:
// theString plus intPrimitive.
func patternMatchUntilRepeatTags587BeanSendPin(theString string, intPrimitive int64) patternMatchUntilRepeatTags587StepPin {
	return patternMatchUntilRepeatTags587SendPin(patternMatchUntilRepeatTags587BeanType, map[string]any{
		"theString":    theString,
		"intPrimitive": float64(intPrimitive),
	})
}

func patternMatchUntilRepeatTags587UndeployAllPin() patternMatchUntilRepeatTags587StepPin {
	return patternMatchUntilRepeatTags587StepPin{op: "undeploy-all"}
}

// patternMatchUntilRepeatTags587EventAt renders one pinned advance-time
// instant as RFC3339 UTC (millis after epoch, matching the Java
// sendTimeSpan instants).
func patternMatchUntilRepeatTags587EventAt(offsetMS int64) string {
	return time.Unix(0, 0).UTC().Add(time.Duration(offsetMS) * time.Millisecond).Format(time.RFC3339Nano)
}

// patternMatchUntilRepeatTags587CaseSteps pins the complete step
// sequence per case in Java source order: leg 1 deploys s0, sends the
// four letter-bean events and undeploys; leg 2 advances to 0 BEFORE the
// s0 deploy, interleaves the eight SupportBean sends with the 5000/
// 10000/15000 advances and undeploys; leg 3 redeploys s0, sends the six
// SupportBean events and undeploys. env.milestone savepoints omitted —
// they restore identical state.
var patternMatchUntilRepeatTags587CaseSteps = map[string][]patternMatchUntilRepeatTags587StepPin{
	"repeat-use-tags": {
		// leg 1: every [2] (a=SupportBean_A() -> b=SupportBean_B(id=a.id))
		patternMatchUntilRepeatTags587DeployPin("s0", patternMatchUntilRepeatTags587Leg1EPL),
		patternMatchUntilRepeatTags587IDSendPin(patternMatchUntilRepeatTags587AType, "A1"),
		patternMatchUntilRepeatTags587IDSendPin(patternMatchUntilRepeatTags587BType, "A1"),
		patternMatchUntilRepeatTags587IDSendPin(patternMatchUntilRepeatTags587AType, "A2"),
		patternMatchUntilRepeatTags587IDSendPin(patternMatchUntilRepeatTags587BType, "A2"),
		patternMatchUntilRepeatTags587UndeployAllPin(),
		// leg 2: advanceTime(0) precedes the deploy; the until timers
		// complete e1 at t=5000 and e2 when its timer expires at
		// t=7000 — the delivery records at the t=10000 advance target.
		patternMatchUntilRepeatTags587AdvancePin(patternMatchUntilRepeatTags587EventAt(0)),
		patternMatchUntilRepeatTags587DeployPin("s0", patternMatchUntilRepeatTags587Leg2EPL),
		patternMatchUntilRepeatTags587BeanSendPin("2", 0),
		patternMatchUntilRepeatTags587BeanSendPin("2", 0),
		patternMatchUntilRepeatTags587AdvancePin(patternMatchUntilRepeatTags587EventAt(5000)),
		patternMatchUntilRepeatTags587BeanSendPin("3", 0),
		patternMatchUntilRepeatTags587BeanSendPin("3", 0),
		patternMatchUntilRepeatTags587BeanSendPin("3", 0),
		patternMatchUntilRepeatTags587BeanSendPin("3", 0),
		patternMatchUntilRepeatTags587AdvancePin(patternMatchUntilRepeatTags587EventAt(10000)),
		patternMatchUntilRepeatTags587BeanSendPin("2", 0),
		patternMatchUntilRepeatTags587BeanSendPin("2", 0),
		patternMatchUntilRepeatTags587AdvancePin(patternMatchUntilRepeatTags587EventAt(15000)),
		patternMatchUntilRepeatTags587UndeployAllPin(),
		// leg 3: every [2] A -> [2] B -> [2] C with A[0].intPrimitive
		// correlation; note the pinned double space inside `pattern [`.
		patternMatchUntilRepeatTags587DeployPin("s0", patternMatchUntilRepeatTags587Leg3EPL),
		patternMatchUntilRepeatTags587BeanSendPin("1", 10),
		patternMatchUntilRepeatTags587BeanSendPin("1", 20),
		patternMatchUntilRepeatTags587BeanSendPin("2", 10),
		patternMatchUntilRepeatTags587BeanSendPin("2", 10),
		patternMatchUntilRepeatTags587BeanSendPin("3", 10),
		patternMatchUntilRepeatTags587BeanSendPin("3", 10),
		patternMatchUntilRepeatTags587UndeployAllPin(),
	},
}

func loadPatternMatchUntilRepeatTags587Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternMatchUntilRepeatTags587ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternMatchUntilRepeatTags587ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternMatchUntilRepeatTags587ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternMatchUntilRepeatTags587ID, err)
	}
	if err := requirePatternMatchUntilRepeatTags587Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternMatchUntilRepeatTags587ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternMatchUntilRepeatTags587ID ||
		metadata.Description != patternMatchUntilRepeatTags587Description ||
		metadata.JavaCommit != patternMatchUntilRepeatTags587JavaCommit ||
		metadata.JavaSource != patternMatchUntilRepeatTags587JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternMatchUntilRepeatTags587ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternMatchUntilRepeatTags587JavaSources},
		{metadata.JavaRuntimes, patternMatchUntilRepeatTags587JavaRuntimeIDs},
		{metadata.JavaNames, patternMatchUntilRepeatTags587JavaExecutions},
		{metadata.JavaStaticIDs, patternMatchUntilRepeatTags587JavaStaticIDs},
		{metadata.JavaFlags, patternMatchUntilRepeatTags587JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternMatchUntilRepeatTags587ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternMatchUntilRepeatTags587CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternMatchUntilRepeatTags587ID, len(patternMatchUntilRepeatTags587CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternMatchUntilRepeatTags587Fields(object, "case", "ordinal", "runtimeId",
			"executionName", "observation", "epls"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string   `json:"case"`
			Ordinal       int      `json:"ordinal"`
			RuntimeID     string   `json:"runtimeId"`
			ExecutionName string   `json:"executionName"`
			Observation   string   `json:"observation"`
			EPLs          []string `json:"epls"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := patternMatchUntilRepeatTags587CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternMatchUntilRepeatTags587JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternMatchUntilRepeatTags587JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || !reflect.DeepEqual(definition.EPLs, spec.epls) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternMatchUntilRepeatTags587ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternMatchUntilRepeatTags587ID)
	}
	if err := validatePatternMatchUntilRepeatTags587RawSteps(rawSteps); err != nil {
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

// validatePatternMatchUntilRepeatTags587RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned per-case sequences — each leg's s0 deploy with the
// verbatim EPL, the pinned sends and advance-time instants, and the
// undeploy-all separators. milestone() savepoints carry no steps.
func validatePatternMatchUntilRepeatTags587RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternMatchUntilRepeatTags587CaseSpecs))
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
			Case      string          `json:"case"`
			Statement string          `json:"statement"`
			EPL       string          `json:"epl"`
			At        string          `json:"at"`
			EventType string          `json:"eventType"`
			Payload   json.RawMessage `json:"payload"`
		}
		if operation == "case" {
			if err := requirePatternMatchUntilRepeatTags587Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternMatchUntilRepeatTags587CaseSteps[marker.Case]; !ok {
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
		pins := patternMatchUntilRepeatTags587CaseSteps[currentCase]
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
			if err := requirePatternMatchUntilRepeatTags587Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "advance-time":
			if err := requirePatternMatchUntilRepeatTags587Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time %q is not the pinned %q for case %q",
					index, step.At, pin.at, currentCase)
			}
		case "send":
			if err := requirePatternMatchUntilRepeatTags587Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternMatchUntilRepeatTags587Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternMatchUntilRepeatTags587CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternMatchUntilRepeatTags587CaseSpecs))
	}
	for index, spec := range patternMatchUntilRepeatTags587CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternMatchUntilRepeatTags587CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternMatchUntilRepeatTags587CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternMatchUntilRepeatTags587Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternMatchUntilRepeatTags587CaseState carries per-case replay state:
// the deployed statement, the per-deploy listener sequence counter and
// the delivery records. deployIndex tracks which of the case's pinned
// EPLs the next deploy step must carry (three sequential legs).
type patternMatchUntilRepeatTags587CaseState struct {
	spec        patternMatchUntilRepeatTags587CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	statements  map[string]*esper.Statement
	deployment  *esper.Deployment
	sequence    map[string]uint64
	records     []compat.TraceRecord
	deployIndex int
}

// runPatternMatchUntilRepeatTags587Scenario replays the execution
// against a fresh engine like the Java oracle's per-execution runtime;
// all three legs share that one engine because the Java execution runs
// its undeployAll calls inside a single runtime. The engine starts at
// the epoch instant; leg 2's advance-time steps move the external clock
// to the pinned instants.
func runPatternMatchUntilRepeatTags587Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternMatchUntilRepeatTags587Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternMatchUntilRepeatTags587CaseSpecs {
		records, err := runPatternMatchUntilRepeatTags587Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternMatchUntilRepeatTags587ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternMatchUntilRepeatTags587Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternMatchUntilRepeatTags587ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternMatchUntilRepeatTags587ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternMatchUntilRepeatTags587RawSteps(rawSteps)
}

func runPatternMatchUntilRepeatTags587Case(ctx context.Context, scenario compat.Scenario, spec patternMatchUntilRepeatTags587CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternMatchUntilRepeatTags587Bean](env, patternMatchUntilRepeatTags587BeanType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilRepeatTags587A](env, patternMatchUntilRepeatTags587AType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilRepeatTags587B](env, patternMatchUntilRepeatTags587BType); err != nil {
		return nil, err
	}
	state := &patternMatchUntilRepeatTags587CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternMatchUntilRepeatTags587JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
		statements: make(map[string]*esper.Statement),
		sequence:   make(map[string]uint64),
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
			payload, err := patternMatchUntilRepeatTags587DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse advance-time %q: %w", patternMatchUntilRepeatTags587ID, step.At, err)
			}
			if err := state.engine.AdvanceTime(ctx, at); err != nil {
				return nil, err
			}
		case "undeploy-all":
			if state.deployment != nil {
				if err := state.deployment.Undeploy(ctx); err != nil {
					return nil, err
				}
				state.deployment = nil
			}
			state.statements = make(map[string]*esper.Statement)
			// The Java oracle attaches a fresh listener per deploy, so
			// the per-statement sequence restarts with each leg.
			state.sequence = make(map[string]uint64)
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternMatchUntilRepeatTags587ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned leg EPL onto the fluent Go equivalent,
// mirroring internal/esper/pattern_matchuntil_parity_test.go's
// TestPatternMatchUntilRepeatUseTagsMatchesEsper subtests:
//
//   - leg 1 `every [2] (a=SupportBean_A() -> b=SupportBean_B(id=a.id))`:
//     PatternFrom(a,true).Then(PatternFrom(b, id=a.id)).MatchUntil(2,2)
//     .Every() — each repeat iteration opens on the a that captured it
//     and the correlated b filter evaluates against THAT iteration's
//     tag scope. select * projects the collected tag arrays a/b via
//     TagEvents.
//   - leg 2 `every ([2:]e1='2' until timer:interval(5))->([2:]e2='3'
//     until timer:interval(2))`: MatchUntil(2,0) pins the [2:] open
//     range; Until(TimerInterval(5s/2s)) attaches the until-timer. NOTE
//     the documented surface nuance: Go's Every applies to the first
//     operand (every(e1-until) -> e2-until) while the Java every scopes
//     the whole sequence — unobservable for this leg's pinned sends.
//     select * projects e1/e2 via TagEvents.
//   - leg 3 `every [2] A -> [2] B -> [2] C` with B/C correlated on
//     A[0].intPrimitive: TagFieldAt("A",0,"intPrimitive") reads the
//     FIRST element of the completed A-repeat (10), which the pinned
//     data distinguishes from A[1]=20. MatchUntil(2,2) pins each [2]
//     exact repeat; select * projects A/B/C via TagEvents.
//
// The listener sequence counter starts at each deploy, mirroring the
// Java oracle's per-deployment TraceWriter.
func (s *patternMatchUntilRepeatTags587CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" || s.deployIndex >= len(s.spec.epls) ||
		step.Epl != s.spec.epls[s.deployIndex] {
		return fmt.Errorf("%s: case %q deploy %d is not pinned", patternMatchUntilRepeatTags587ID, s.spec.name, s.deployIndex)
	}
	if _, ok := s.statements[step.Statement]; ok {
		return fmt.Errorf("%s: label %q is already deployed", patternMatchUntilRepeatTags587ID, step.Statement)
	}
	deployIndex := s.deployIndex
	var query esper.Query
	switch deployIndex {
	case 0:
		// leg 1: every [2] (a=SupportBean_A() -> b=SupportBean_B(id=a.id))
		a := esper.From[patternMatchUntilRepeatTags587A](s.env, patternMatchUntilRepeatTags587AType)
		b := esper.From[patternMatchUntilRepeatTags587B](s.env, patternMatchUntilRepeatTags587BType)
		sequence := esper.PatternFrom(a, "a", esper.Literal(true)).Then(
			esper.PatternFrom(b, "b", esper.Equal[string](
				esper.Field[patternMatchUntilRepeatTags587B, string]("id"),
				esper.TagField[string]("a", "id"))))
		query = sequence.MatchUntil(2, 2).Every().Select(
			esper.Alias("a", esper.TagEvents("a")),
			esper.Alias("b", esper.TagEvents("b")),
		).Query(esper.StatementName("s0"))
	case 1:
		// leg 2: every ([2:]e1='2' until timer:interval(5)) ->
		// ([2:]e2='3' until timer:interval(2)) — Go materializes
		// every(e1-until) -> (e2-until); see the header's every-scope
		// nuance note (unobservable for this leg's pinned sends).
		sb := esper.From[patternMatchUntilRepeatTags587Bean](s.env, patternMatchUntilRepeatTags587BeanType)
		strIs := func(value string) esper.Expression[bool] {
			return esper.Equal[string](
				esper.Field[patternMatchUntilRepeatTags587Bean, string]("theString"),
				esper.Literal(value))
		}
		first := esper.PatternFrom(sb, "e1", strIs("2")).
			MatchUntil(2, 0).
			Until(esper.TimerInterval(sb, 5*time.Second)).
			Every()
		second := esper.PatternFrom(sb, "e2", strIs("3")).
			MatchUntil(2, 0).
			Until(esper.TimerInterval(sb, 2*time.Second))
		query = first.Then(second).Select(
			esper.Alias("e1", esper.TagEvents("e1")),
			esper.Alias("e2", esper.TagEvents("e2")),
		).Query(esper.StatementName("s0"))
	case 2:
		// leg 3: every [2] A='1' -> [2] B('2' and intPrimitive=A[0]) ->
		// [2] C('3' and intPrimitive=A[0]); A[0].intPrimitive reads the
		// FIRST element of the completed A-repeat.
		sb := esper.From[patternMatchUntilRepeatTags587Bean](s.env, patternMatchUntilRepeatTags587BeanType)
		strIs := func(value string) esper.Expression[bool] {
			return esper.Equal[string](
				esper.Field[patternMatchUntilRepeatTags587Bean, string]("theString"),
				esper.Literal(value))
		}
		intIsA0 := esper.Equal[int](
			esper.Field[patternMatchUntilRepeatTags587Bean, int]("intPrimitive"),
			esper.TagFieldAt[int]("A", 0, "intPrimitive"))
		first := esper.PatternFrom(sb, "A", strIs("1")).MatchUntil(2, 2).Every()
		second := esper.PatternFrom(sb, "B", esper.And(strIs("2"), intIsA0)).MatchUntil(2, 2)
		third := esper.PatternFrom(sb, "C", esper.And(strIs("3"), intIsA0)).MatchUntil(2, 2)
		query = first.Then(second).Then(third).Select(
			esper.Alias("A", esper.TagEvents("A")),
			esper.Alias("B", esper.TagEvents("B")),
			esper.Alias("C", esper.TagEvents("C")),
		).Query(esper.StatementName("s0"))
	default:
		return fmt.Errorf("%s: unsupported deploy index %d", patternMatchUntilRepeatTags587ID, deployIndex)
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return err
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	statement := deployment.Statements()[0]
	s.deployment = deployment
	s.deployIndex++
	s.statements[step.Statement] = statement
	label := step.Statement
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		s.sequence[label]++
		s.records = append(s.records, compat.TraceRecord{
			Case:      s.spec.name,
			Operation: "listener",
			Statement: label,
			Sequence:  s.sequence[label],
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

// patternMatchUntilRepeatTags587DecodePayload converts a scenario send
// payload into the typed host object for the event type: SupportBean
// carries theString/intPrimitive (legs 2/3) and the letter beans carry
// the single id (leg 1).
func patternMatchUntilRepeatTags587DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case patternMatchUntilRepeatTags587BeanType:
		if err := requirePatternMatchUntilRepeatTags587Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var event patternMatchUntilRepeatTags587Bean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return event, nil
	case patternMatchUntilRepeatTags587AType:
		if err := requirePatternMatchUntilRepeatTags587Fields(fields, "id"); err != nil {
			return nil, err
		}
		var event patternMatchUntilRepeatTags587A
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return event, nil
	case patternMatchUntilRepeatTags587BType:
		if err := requirePatternMatchUntilRepeatTags587Fields(fields, "id"); err != nil {
			return nil, err
		}
		var event patternMatchUntilRepeatTags587B
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_B: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternMatchUntilRepeatTags587ID, step.EventType)
	}
}
