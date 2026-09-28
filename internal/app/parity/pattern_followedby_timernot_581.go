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

// Parity coverage for PatternOperatorFollowedBy ords 1, 6 and 9: the
// timer+not trio — every-then followed-by whose second leg is
// `timer:interval(...) and not ...` — over SupportBean and the
// SupportBean_A/B/C id beans, all driven by advance-time steps against
// the virtual clock like the Java executions' external timer.
// env.milestone savepoints in the Java executions are harness splits
// restoring identical state and are unrepresented.
//
// Covered executions (all variant collection:executions(), flags [],
// deduped static id java-089b2086c9945dff918f):
//   - ord 1 PatternFollowedByWithNot java-runtime-df0604ce4c4e286b3e6e
//     (case with-not): `every a=SupportBean_A -> (timer:interval(10
//     seconds) and not (SupportBean_B(id=a.id) or
//     SupportBean_C(id=a.id)))`. Each A arms a per-branch 10-second
//     timer cancelled only by a same-id B or C: A1 fires at t=10000,
//     B(A2) at t=29999 cancels A2's branch (t=30000 silent), C(A3) at
//     t=30000 cancels A3's branch (t=40000 silent), and non-matching
//     B(B4)/C(A5) leave A4 firing at t=50000 — 2 listener records, both
//     assertEqualsNew(s0,a,...) deliveries projecting the SupportBean_A
//     fragment under select *.
//   - ord 6 PatternFollowedNotEvery java-runtime-8631344fa90a6c397c84
//     (case not-every): `every A=SupportBean -> (timer:interval(1
//     seconds) and not SupportBean_A)`. The terminator is UNCORRELATED —
//     any SupportBean_A would quiesce every armed branch, but none is
//     sent. Two SupportBean sends at t=0 arm two branches whose timers
//     expire together at t=1000: the Java assertion pins
//     getNewDataList().size()==1 AND get(0).length==2 — ONE listener
//     invocation carrying TWO rows, the batching discriminant of this
//     slice.
//   - ord 9 PatternFollowedOrPermFalse java-runtime-7682cbc4f27c52c33cce
//     (case or-perm-false): `every s=SupportBean(theString='E') ->
//     (timer:interval(10) and not SupportBean(theString='C1'))or(
//     SupportBean(theString='C2') and not timer:interval(10))` — the
//     literal `)or(` (no spaces) is pinned byte-exact. The bare numeric
//     timer:interval(10) is SECONDS under the pattern observer's time
//     abacus (deltaForSeconds; the external clock still advances in
//     millis): the `)or(` sits inside the followed-by's second leg, so
//     the right alternative's `not timer:interval(10)` arms at the E
//     match (t=1000) and dies permanently at t=11000 without a C2 — the
//     same instant the left branch's timer fires the E sent at t=1000 —
//     1 record.
//
// Byte-exact EPL pins keep the Java source verbatim: ord 1 keeps the
// leading space after `[` and trailing space after `]` plus the plural
// `10 seconds`; ord 6 keeps `1 seconds`; ord 9 keeps the bare `10` and
// the spaceless `)or(`. All three use select * — under Esper pattern
// semantics the delivered rows carry the bound tag's bean fragment (`a`,
// `A`, `s` respectively); unnamed not-branches project nothing.
//
// Siblings excluded from this slice (manifest dispositions are
// main-agent owned): ord 0 (W-harness), ord 2 (timer:within + where),
// ords 3-5 (RFID trio, covered by pattern-followedby-rfid-580) and
// ords 7/8 (no-timer chains) remain tracked in the capability manifest
// by main.
const patternFollowedByTimerNot581ID = "pattern-followedby-timernot-581"

const patternFollowedByTimerNot581Description = "PatternOperatorFollowedBy ords 1, 6 and 9 — the timer+not trio: every-then followed-by whose second leg is `timer:interval(...) and not ...`, all driven by the external clock (advance-time ops; engine timer threading off). with-not (PatternFollowedByWithNot, ord 1) arms a 10-second timer per SupportBean_A cancelled by a correlated B/C: A1 fires at t=10000, correlated B(A2)/C(A3) cancels keep A2/A3 silent, non-correlated B(B4)/C(A5) leave A4 firing at t=50000 — 2 listener records, select * projecting the SupportBean_A fragment. not-every (PatternFollowedNotEvery, ord 6) arms a 1-second timer per uncorrelated SupportBean_A terminator: two SupportBean sends at t=0 deliver ONE listener invocation carrying TWO rows at t=1000 — the batching pin. or-perm-false (PatternFollowedOrPermFalse, ord 9) ORs the timer leg against `(theString='C2' and not timer:interval(10))`: bare numeric timer parameters are seconds under the pattern observer's time abacus (external clock still advances in millis); the `)or(` sits inside the followed-by's second leg, so the right alternative's not-timer arms at the E match (t=1000) and dies permanently at t=11000 without C2 — the same instant the left branch's timer fires the E send at t=1000 — 1 record, the literal `)or(` kept byte-exact. All env.milestone savepoints are harness splits restoring identical state and are unrepresented."

const patternFollowedByTimerNot581JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternFollowedByTimerNot581JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java"

// Byte-exact EPL pins (PatternOperatorFollowedBy.java, single-space
// concatenation verbatim): ord 1 keeps the leading space after `[` and
// trailing space after `]` (the " every ... " fragment plus "] "), ord 6
// keeps `1 seconds`, and ord 9 keeps the literal `)or(` and the bare
// numeric `timer:interval(10)` — seconds under the pattern observer's
// time abacus, not milliseconds.
const (
	patternFollowedByTimerNot581WithNotEPL  = "@name('s0') select * from pattern [ every a=SupportBean_A -> (timer:interval(10 seconds) and not (SupportBean_B(id=a.id) or SupportBean_C(id=a.id)))] "
	patternFollowedByTimerNot581NotEveryEPL = "@name('s0') select * from pattern [every A=SupportBean -> (timer:interval(1 seconds) and not SupportBean_A)]"
	patternFollowedByTimerNot581OrPermEPL   = "@name('s0') select * from pattern [every s=SupportBean(theString='E') -> (timer:interval(10) and not SupportBean(theString='C1'))or(SupportBean(theString='C2') and not timer:interval(10))]"

	patternFollowedByTimerNot581SupportBeanType  = "SupportBean"
	patternFollowedByTimerNot581SupportBeanAType = "SupportBean_A"
	patternFollowedByTimerNot581SupportBeanBType = "SupportBean_B"
	patternFollowedByTimerNot581SupportBeanCType = "SupportBean_C"
)

// Per-case Java identities: each case pins its own ordinal plus a
// runtimeIndex into the arrays below (no multi-leg runtimeIndex sharing
// in this slice).
var (
	patternFollowedByTimerNot581JavaRuntimeIDs = []string{
		"java-runtime-df0604ce4c4e286b3e6e",
		"java-runtime-8631344fa90a6c397c84",
		"java-runtime-7682cbc4f27c52c33cce",
	}
	patternFollowedByTimerNot581JavaSources = []string{
		patternFollowedByTimerNot581JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_C.java",
	}
	patternFollowedByTimerNot581JavaExecutions = []string{
		"PatternFollowedByWithNot",
		"PatternFollowedNotEvery",
		"PatternFollowedOrPermFalse",
	}
	// Deduplicated inventory id: all ten PatternOperatorFollowedBy
	// inventory rows share static id java-089b2086c9945dff918f (NOT the
	// every-distinct file's java-073091a0b42ca8dd974f), pinned once per
	// runtimeId row.
	patternFollowedByTimerNot581JavaStaticIDs = []string{
		"java-089b2086c9945dff918f",
		"java-089b2086c9945dff918f",
		"java-089b2086c9945dff918f",
	}
	patternFollowedByTimerNot581JavaFlags = []string{}
)

// patternFollowedByTimerNot581SupportBean mirrors SupportBean's asserted
// fields (theString, intPrimitive). theString is a *string so the default
// SupportBean() the ord 6 execution sends carries null and renders the
// {"state":"null"} marker exactly like the Java bean; only these two
// pinned properties are observable under select * normalization.
type patternFollowedByTimerNot581SupportBean struct {
	TheString    *string `json:"theString" esper:"theString"`
	IntPrimitive int     `json:"intPrimitive" esper:"intPrimitive"`
}

// patternFollowedByTimerNot581BeanA/B/C mirror SupportBean_A/B/C: each
// carries the single id property the executions correlate on.
type patternFollowedByTimerNot581BeanA struct {
	ID string `json:"id" esper:"id"`
}
type patternFollowedByTimerNot581BeanB struct {
	ID string `json:"id" esper:"id"`
}
type patternFollowedByTimerNot581BeanC struct {
	ID string `json:"id" esper:"id"`
}

// patternFollowedByTimerNot581CaseSpec pins one case: Java execution
// identity (ordinal plus runtimeIndex into the shared per-ord arrays),
// the observation text and the byte-exact EPL script. select * projects
// the bound tag's bean fragment — `a` (ord 1), `A` (ord 6) and `s`
// (ord 9); the not-branches are unnamed and project nothing.
type patternFollowedByTimerNot581CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epl          string
}

var patternFollowedByTimerNot581CaseSpecs = []patternFollowedByTimerNot581CaseSpec{
	{
		name:         "with-not",
		ordinal:      1,
		runtimeIndex: 0,
		observation:  "listener; advance-time drives the 10-second timers; `every a=SupportBean_A -> (timer:interval(10 seconds) and not (SupportBean_B(id=a.id) or SupportBean_C(id=a.id)))`: A1 fires at t=10000, B(A2)/C(A3) correlated cancels stay silent, B(B4)/C(A5) non-matching cancels let A4 fire at t=50000 — 2 records, select * projects the a fragment",
		epl:          patternFollowedByTimerNot581WithNotEPL,
	},
	{
		name:         "not-every",
		ordinal:      6,
		runtimeIndex: 1,
		observation:  "listener; `every A=SupportBean -> (timer:interval(1 seconds) and not SupportBean_A)` — the terminator is UNCORRELATED: two SupportBean sends at t=0 arm both branches, advance to t=1000 delivers ONE listener invocation with TWO rows (Java asserts getNewDataList().size()==1 and get(0).length==2)",
		epl:          patternFollowedByTimerNot581NotEveryEPL,
	},
	{
		name:         "or-perm-false",
		ordinal:      9,
		runtimeIndex: 2,
		observation:  "listener; `every s=SupportBean(theString='E') -> (timer:interval(10) and not SupportBean(theString='C1'))or(SupportBean(theString='C2') and not timer:interval(10))` — literal )or( no spaces; bare timer:interval(10) is SECONDS: right alternative's not-timer arms at the E match and dies permanently at t=11000, E sent at t=1000 stays silent at t=10999 and fires at t=11000 — 1 record",
		epl:          patternFollowedByTimerNot581OrPermEPL,
	},
}

// patternFollowedByTimerNot581StepPin pins one scenario step's shape:
// deploys carry the byte-exact EPL, sends the pinned payload, and
// advance-time steps the pinned instant (RFC3339 UTC like every other
// virtual-clock scenario).
type patternFollowedByTimerNot581StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	at        string
}

func patternFollowedByTimerNot581DeployPin(statement, epl string) patternFollowedByTimerNot581StepPin {
	return patternFollowedByTimerNot581StepPin{op: "deploy", statement: statement, epl: epl}
}

func patternFollowedByTimerNot581AdvancePin(at string) patternFollowedByTimerNot581StepPin {
	return patternFollowedByTimerNot581StepPin{op: "advance-time", at: at}
}

func patternFollowedByTimerNot581SendPin(eventType string, payload map[string]any) patternFollowedByTimerNot581StepPin {
	return patternFollowedByTimerNot581StepPin{op: "send", eventType: eventType, payload: payload}
}

func patternFollowedByTimerNot581IDSendPin(eventType, id string) patternFollowedByTimerNot581StepPin {
	return patternFollowedByTimerNot581SendPin(eventType, map[string]any{"id": id})
}

func patternFollowedByTimerNot581BeanSendPin(theString any, intPrimitive int) patternFollowedByTimerNot581StepPin {
	return patternFollowedByTimerNot581SendPin(patternFollowedByTimerNot581SupportBeanType,
		map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)})
}

func patternFollowedByTimerNot581UndeployAllPin() patternFollowedByTimerNot581StepPin {
	return patternFollowedByTimerNot581StepPin{op: "undeploy-all"}
}

// patternFollowedByTimerNot581CaseSteps pins the complete step sequence
// per case in Java source order (env.milestone savepoints omitted —
// they restore identical state: ord 1 milestones 0/1/2, ord 6
// milestone 1, ord 9 milestone 0). Advance-time steps pin every
// sendTimer/advanceTime call verbatim, including ord 1's repeated
// t=30000 advances around the A3/C(A3) pair.
var patternFollowedByTimerNot581CaseSteps = func() map[string][]patternFollowedByTimerNot581StepPin {
	steps := make(map[string][]patternFollowedByTimerNot581StepPin)
	withNotLeg := func(epl string) []patternFollowedByTimerNot581StepPin {
		// ord 1: A1 fires at 10000; B(A2) cancels (30000 silent); C(A3)
		// cancels (40000 silent); B(B4)/C(A5) non-correlated, A4 fires at
		// 50000.
		return []patternFollowedByTimerNot581StepPin{
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:00Z"),
			patternFollowedByTimerNot581DeployPin("s0", epl),
			patternFollowedByTimerNot581IDSendPin(patternFollowedByTimerNot581SupportBeanAType, "A1"),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:09.999Z"),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:10Z"),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:20Z"),
			patternFollowedByTimerNot581IDSendPin(patternFollowedByTimerNot581SupportBeanAType, "A2"),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:29.999Z"),
			patternFollowedByTimerNot581IDSendPin(patternFollowedByTimerNot581SupportBeanBType, "A2"),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:30Z"),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:30Z"),
			patternFollowedByTimerNot581IDSendPin(patternFollowedByTimerNot581SupportBeanAType, "A3"),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:30Z"),
			patternFollowedByTimerNot581IDSendPin(patternFollowedByTimerNot581SupportBeanCType, "A3"),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:40Z"),
			patternFollowedByTimerNot581IDSendPin(patternFollowedByTimerNot581SupportBeanAType, "A4"),
			patternFollowedByTimerNot581IDSendPin(patternFollowedByTimerNot581SupportBeanBType, "B4"),
			patternFollowedByTimerNot581IDSendPin(patternFollowedByTimerNot581SupportBeanCType, "A5"),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:50Z"),
			patternFollowedByTimerNot581UndeployAllPin(),
		}
	}
	notEveryLeg := func(epl string) []patternFollowedByTimerNot581StepPin {
		// ord 6: two default SupportBean sends at t=0 (theString null
		// like new SupportBean()), advance to t=1000 delivers ONE
		// invocation carrying TWO rows.
		return []patternFollowedByTimerNot581StepPin{
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:00Z"),
			patternFollowedByTimerNot581DeployPin("s0", epl),
			patternFollowedByTimerNot581BeanSendPin(nil, 0),
			patternFollowedByTimerNot581BeanSendPin(nil, 0),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:01Z"),
			patternFollowedByTimerNot581UndeployAllPin(),
		}
	}
	orPermFalseLeg := func(epl string) []patternFollowedByTimerNot581StepPin {
		// ord 9: deploy at t=0, advance to t=1000, send E, silent at
		// t=10999, fires at t=11000.
		return []patternFollowedByTimerNot581StepPin{
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:00Z"),
			patternFollowedByTimerNot581DeployPin("s0", epl),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:01Z"),
			patternFollowedByTimerNot581BeanSendPin("E", 0),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:10.999Z"),
			patternFollowedByTimerNot581AdvancePin("1970-01-01T00:00:11Z"),
			patternFollowedByTimerNot581UndeployAllPin(),
		}
	}
	for _, spec := range patternFollowedByTimerNot581CaseSpecs {
		switch spec.name {
		case "with-not":
			steps[spec.name] = withNotLeg(spec.epl)
		case "not-every":
			steps[spec.name] = notEveryLeg(spec.epl)
		case "or-perm-false":
			steps[spec.name] = orPermFalseLeg(spec.epl)
		}
	}
	return steps
}()

func loadPatternFollowedByTimerNot581Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternFollowedByTimerNot581ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternFollowedByTimerNot581ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternFollowedByTimerNot581ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternFollowedByTimerNot581ID, err)
	}
	if err := requirePatternFollowedByTimerNot581Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternFollowedByTimerNot581ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternFollowedByTimerNot581ID ||
		metadata.Description != patternFollowedByTimerNot581Description ||
		metadata.JavaCommit != patternFollowedByTimerNot581JavaCommit ||
		metadata.JavaSource != patternFollowedByTimerNot581JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternFollowedByTimerNot581ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternFollowedByTimerNot581JavaSources},
		{metadata.JavaRuntimes, patternFollowedByTimerNot581JavaRuntimeIDs},
		{metadata.JavaNames, patternFollowedByTimerNot581JavaExecutions},
		{metadata.JavaStaticIDs, patternFollowedByTimerNot581JavaStaticIDs},
		{metadata.JavaFlags, patternFollowedByTimerNot581JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternFollowedByTimerNot581ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternFollowedByTimerNot581CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternFollowedByTimerNot581ID, len(patternFollowedByTimerNot581CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternFollowedByTimerNot581Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternFollowedByTimerNot581CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternFollowedByTimerNot581JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternFollowedByTimerNot581JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternFollowedByTimerNot581ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternFollowedByTimerNot581ID)
	}
	if err := validatePatternFollowedByTimerNot581RawSteps(rawSteps); err != nil {
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

// validatePatternFollowedByTimerNot581RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned per-case sequences (advance-time instants, deploy
// EPLs and send payloads all pinned).
func validatePatternFollowedByTimerNot581RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternFollowedByTimerNot581CaseSpecs))
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
			EventType string          `json:"eventType"`
			Payload   json.RawMessage `json:"payload"`
			At        string          `json:"at"`
		}
		if operation == "case" {
			if err := requirePatternFollowedByTimerNot581Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternFollowedByTimerNot581CaseSteps[marker.Case]; !ok {
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
		pins := patternFollowedByTimerNot581CaseSteps[currentCase]
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
			if err := requirePatternFollowedByTimerNot581Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternFollowedByTimerNot581Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "advance-time":
			if err := requirePatternFollowedByTimerNot581Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time %q is not the pinned %q for case %q",
					index, step.At, pin.at, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternFollowedByTimerNot581Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternFollowedByTimerNot581CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternFollowedByTimerNot581CaseSpecs))
	}
	for index, spec := range patternFollowedByTimerNot581CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternFollowedByTimerNot581CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternFollowedByTimerNot581CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternFollowedByTimerNot581Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternFollowedByTimerNot581CaseState carries per-case replay state:
// the deployed statement, the listener sequence counter and the
// delivery records.
type patternFollowedByTimerNot581CaseState struct {
	spec       patternFollowedByTimerNot581CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	statements map[string]*esper.Statement
	deployment *esper.Deployment
	sequence   map[string]uint64
	records    []compat.TraceRecord
}

// runPatternFollowedByTimerNot581Scenario replays the executions against
// a fresh engine per case like the Java oracle's fresh per-execution
// runtime.
func runPatternFollowedByTimerNot581Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternFollowedByTimerNot581Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternFollowedByTimerNot581CaseSpecs {
		records, err := runPatternFollowedByTimerNot581Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternFollowedByTimerNot581ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternFollowedByTimerNot581Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternFollowedByTimerNot581ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternFollowedByTimerNot581ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternFollowedByTimerNot581RawSteps(rawSteps)
}

func runPatternFollowedByTimerNot581Case(ctx context.Context, scenario compat.Scenario, spec patternFollowedByTimerNot581CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternFollowedByTimerNot581SupportBean](env, patternFollowedByTimerNot581SupportBeanType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByTimerNot581BeanA](env, patternFollowedByTimerNot581SupportBeanAType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByTimerNot581BeanB](env, patternFollowedByTimerNot581SupportBeanBType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByTimerNot581BeanC](env, patternFollowedByTimerNot581SupportBeanCType); err != nil {
		return nil, err
	}
	state := &patternFollowedByTimerNot581CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternFollowedByTimerNot581JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			payload, err := patternFollowedByTimerNot581DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse advance-time %q: %w", patternFollowedByTimerNot581ID, step.At, err)
			}
			// The clock moves first: timer fires produced by the advance
			// carry the boundary time, matching the Java trace.
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
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternFollowedByTimerNot581ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned EPL onto the fluent Go equivalent, mirroring
// internal/esper/pattern_followedby_parity_test.go:
//   - with-not (ord 1): PatternFrom(a,"a",true).Every().Then(
//     TimerInterval(a,10s).And(PatternFrom(b,"b",id==a.id).Or(
//     PatternFrom(c,"c",id==a.id)).Not())) — the B/C cancel alternatives
//     correlate per-branch on a.id so B(B4)/C(A5) cannot cancel A4
//   - not-every (ord 6): PatternFrom(sb,"A",true).Every().Then(
//     TimerInterval(sb,1s).And(PatternFrom(a,"na",true).Not())) — the
//     terminator carries no correlation (any SupportBean_A would quiesce)
//   - or-perm-false (ord 9): the `)or(` binds inside Then's argument —
//     left=(timer and not C1), right=(C2 and not timer), plain Or not
//     OrExclusive; the right alternative's not-timer arms at the E match
//     and dies permanently 10s after it (t=11000)
//
// select * expands to Alias(tag, PatternEvent tag) for the bound tag —
// `a`, `A` and `s` respectively — matching Esper's fragment projection;
// the unnamed not-branches project nothing.
func (s *patternFollowedByTimerNot581CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" || step.Epl != s.spec.epl {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternFollowedByTimerNot581ID, s.spec.name)
	}
	if _, ok := s.statements[step.Statement]; ok {
		return fmt.Errorf("%s: label %q is already deployed", patternFollowedByTimerNot581ID, step.Statement)
	}
	sb := esper.From[patternFollowedByTimerNot581SupportBean](s.env, patternFollowedByTimerNot581SupportBeanType)
	a := esper.From[patternFollowedByTimerNot581BeanA](s.env, patternFollowedByTimerNot581SupportBeanAType)
	b := esper.From[patternFollowedByTimerNot581BeanB](s.env, patternFollowedByTimerNot581SupportBeanBType)
	c := esper.From[patternFollowedByTimerNot581BeanC](s.env, patternFollowedByTimerNot581SupportBeanCType)
	theString := esper.Field[patternFollowedByTimerNot581SupportBean, *string]("theString")
	literalE := "E"
	literalC1 := "C1"
	literalC2 := "C2"
	var query esper.Query
	switch s.spec.name {
	case "with-not":
		query = esper.PatternFrom(a, "a", esper.Literal(true)).Every().Then(
			esper.TimerInterval(a, 10*time.Second).And(
				esper.PatternFrom(b, "b", esper.Equal[string](
					esper.Field[patternFollowedByTimerNot581BeanB, string]("id"),
					esper.TagField[string]("a", "id"))).
					Or(esper.PatternFrom(c, "c", esper.Equal[string](
						esper.Field[patternFollowedByTimerNot581BeanC, string]("id"),
						esper.TagField[string]("a", "id")))).Not()),
		).Select(
			esper.Alias("a", esper.PatternEvent("a")),
		).Query(esper.StatementName("s0"))
	case "not-every":
		query = esper.PatternFrom(sb, "A", esper.Literal(true)).Every().Then(
			esper.TimerInterval(sb, time.Second).And(
				esper.PatternFrom(a, "na", esper.Literal(true)).Not()),
		).Select(
			esper.Alias("A", esper.PatternEvent("A")),
		).Query(esper.StatementName("s0"))
	case "or-perm-false":
		// EPL `->` binds loosest: the second leg is a full
		// or-expression — every s=E -> ((timer and not C1) or
		// (C2 and not timer)). The Or is INSIDE Then's argument;
		// a top-level left.Or(right) would arm the right branch at
		// deploy time and fire on C2 before any E — Java only arms
		// it at the E match (its not-timer dies at t=11000).
		query = esper.PatternFrom(sb, "s", esper.Equal[*string](theString, esper.Literal[*string](&literalE))).Every().Then(
			esper.TimerInterval(sb, 10*time.Second).And(
				esper.PatternFrom(sb, "c1", esper.Equal[*string](theString, esper.Literal[*string](&literalC1))).Not()).Or(
				esper.PatternFrom(sb, "c2", esper.Equal[*string](theString, esper.Literal[*string](&literalC2))).
					And(esper.TimerInterval(sb, 10*time.Second).Not())),
		).Select(
			esper.Alias("s", esper.PatternEvent("s")),
		).Query(esper.StatementName("s0"))
	default:
		return fmt.Errorf("%s: unsupported case %q", patternFollowedByTimerNot581ID, s.spec.name)
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

// patternFollowedByTimerNot581DecodePayload converts a scenario send
// payload into the typed host object: SupportBean carries the pinned
// theString/intPrimitive pair (theString may be null like the Java
// default constructor) and SupportBean_A/B/C carry the single id.
func patternFollowedByTimerNot581DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case patternFollowedByTimerNot581SupportBeanType:
		if err := requirePatternFollowedByTimerNot581Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var event patternFollowedByTimerNot581SupportBean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return event, nil
	case patternFollowedByTimerNot581SupportBeanAType:
		if err := requirePatternFollowedByTimerNot581Fields(fields, "id"); err != nil {
			return nil, err
		}
		var event patternFollowedByTimerNot581BeanA
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return event, nil
	case patternFollowedByTimerNot581SupportBeanBType:
		if err := requirePatternFollowedByTimerNot581Fields(fields, "id"); err != nil {
			return nil, err
		}
		var event patternFollowedByTimerNot581BeanB
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_B: %w", err)
		}
		return event, nil
	case patternFollowedByTimerNot581SupportBeanCType:
		if err := requirePatternFollowedByTimerNot581Fields(fields, "id"); err != nil {
			return nil, err
		}
		var event patternFollowedByTimerNot581BeanC
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_C: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternFollowedByTimerNot581ID, step.EventType)
	}
}
