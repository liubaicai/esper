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

// Parity coverage for PatternOperatorFollowedBy ord 0 PatternOpWHarness:
// ONE Java execution whose PatternTestHarness runs sixteen followed-by
// legs over the shared EventCollectionFactory.getEventSetOne(0, 1000)
// mixed event set. The harness compiles all sixteen statements first —
// USE_EPL style text `@name("name--<atom>") @Audit('pattern')
// @Audit('pattern-instances') select * from pattern [<atom>]` — then
// replays the external clock: advanceTime(ON_START=0) happens before
// the deployments and every event advances the clock to its pinned
// instant before sendEventBean. Per trigger event the harness compares
// each leg's expected EventDescriptors against the listener's last
// delivery as a MULTISET (compareLists), so the duplicated rows of legs
// 10/15/16 are normative multiplicity and must be preserved verbatim.
// The COMPILE_TO_MODEL/COMPILE_TO_EPL compile-text styles and the
// consume+suppress style are harness replay variants outside the Go
// fluent surface and are unrepresented; env.milestone savepoints are
// harness splits restoring identical state and are likewise
// unrepresented, as is the silent post-undeploy resend. Within one
// send Esper dispatches pattern completions to statement listeners in
// REVERSE deployment order (newest statement first); the Go engine
// dispatches in deployment order, so the runner buffers each send's
// deliveries and appends them newest-statement-first to match the
// Java observable contract.
//
// Covered execution (variant collection:executions(), flags [],
// static-manifest id java-971bf7dfac84756cdd0a — the per-execution
// discovery id; the deduplicated inventory id java-089b2086c9945dff918f
// shared by all ten PatternOperatorFollowedBy rows is an umbrella pin
// that the per-runtime javaStaticIds array does not carry when a
// per-execution id exists):
//   - ord 0 PatternOpWHarness java-runtime-d896ea164e3cebd5c27e
//     (case w-harness): sixteen legs — `b -> (d or not d)`, the
//     `-[1000]>` bounded twin, `b -> every d`, `b -> d`, `b -> not d`,
//     its bounded twin, `every b -> every d`, `every b -> d`, the
//     `-[10]>` bounded variant, `every (b -> every d)`, the
//     `a_1 -> b -> a_2` three-leg chain, the silent `c -> d -> a`,
//     parenthesized and `-|10|>`-bounded chain twins, `every ( every a
//     -> every b)` and `every (a -> every b)`.
//
// Byte-exact EPL pins keep the Java source verbatim: legs carry `()`
// after type names exactly as written (legs 11/13/14/16 fully, legs
// 12's lone `c=SupportBean_C()` and leg 11's lone `a_1=SupportBean_A()`),
// leg 15 keeps the literal space in `every ( every a=...`, and the
// `-[1000]>`/`-[10]>` FollowedByMax edges are pinned as written even
// though no max ever binds on this clock. The deployed statement text
// is the harness USE_EPL form: @name("name--<atom>") with the two
// @Audit annotations, so listener statements surface as
// `name--<atom>` like the harness's nameOfStatement.
const patternFollowedByWHarness583ID = "pattern-followedby-wharness-583"

const patternFollowedByWHarness583Description = "PatternOperatorFollowedBy ord 0 PatternOpWHarness — the shared sixteen-leg followed-by W-harness over EventCollectionFactory.getEventSetOne(0,1000), replayed as ONE case: all sixteen legs deploy up front as `@name(\"name--<atom>\") @Audit('pattern') @Audit('pattern-instances') select * from pattern [<atom>]`, the external clock advances to ON_START=0 before the deployments and to each event's pinned instant (+1000ms per send) before sendEventBean, and every send's fires are grouped per leg+trigger as multiset rows — including the DUPLICATED rows of legs 10/15/16 (`every (b -> every d)` re-fires {B3,D3} twice at D3; `every ( every a -> every b)` re-fires {A2,B3} three times; `every (a -> every b)` twice). Legs 1/2/5/6 fire AT B1 with a vacant d tag rendered {\"state\":\"null\"} (spawn-satisfied or-not/not right side, not a timeout); leg 12 `c=SupportBean_C() -> d=SupportBean_D -> a=SupportBean_A` is a normative 0-fire leg; the `-[1000]>`/`-[10]>` FollowedByMax edges never bind on this clock. Expected rows (fire event -> tag rows): leg1 B1{b,B1;d,null}+D1{b,B1;d,D1}, leg2 same, leg3 D1/D2/D3 {b,B1;d,Dn}, leg4 D1{b,B1;d,D1}, leg5/6 B1{b,B1;d,null}, leg7 D1/D2 2rows+D3 3rows, leg8 D1 2rows+D3 {b,B3}, leg9 same, leg10 D1/D2 {b,B1}+D3 {b,B1;d,D3},{b,B3;d,D3},{b,B3;d,D3}, leg11 A2{a_1,A1;b,B1;a_2,A2}, leg12 none, leg13/14 same as 11, leg15 B1/B2 {a,A1}+B3 {a,A1;b,B3},{a,A2;b,B3}x3, leg16 same with x2. The COMPILE_TO_MODEL/COMPILE_TO_EPL and consume+suppress harness styles plus all env.milestone savepoints are unrepresented harness machinery."

const patternFollowedByWHarness583JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternFollowedByWHarness583JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java"

// Byte-exact pattern atoms (PatternOperatorFollowedBy.java,
// EventExpressionCase constructor strings, verbatim): `()` after type
// names and `-[N]>` edges are pinned exactly as written — including leg
// 11's lone `a_1=SupportBean_A()` (b and a_2 are bare), leg 12's lone
// `c=SupportBean_C()`, leg 15's `every ( every a=...` with the space
// after the opening parenthesis, and legs 13/14/16 carrying `()` on
// every atom.
const (
	patternFollowedByWHarness583Leg01Atom = "b=SupportBean_B -> (d=SupportBean_D or not d=SupportBean_D)"
	patternFollowedByWHarness583Leg02Atom = "b=SupportBean_B -[1000]> (d=SupportBean_D or not d=SupportBean_D)"
	patternFollowedByWHarness583Leg03Atom = "b=SupportBean_B -> every d=SupportBean_D"
	patternFollowedByWHarness583Leg04Atom = "b=SupportBean_B -> d=SupportBean_D"
	patternFollowedByWHarness583Leg05Atom = "b=SupportBean_B -> not d=SupportBean_D"
	patternFollowedByWHarness583Leg06Atom = "b=SupportBean_B -[1000]> not d=SupportBean_D"
	patternFollowedByWHarness583Leg07Atom = "every b=SupportBean_B -> every d=SupportBean_D"
	patternFollowedByWHarness583Leg08Atom = "every b=SupportBean_B -> d=SupportBean_D"
	patternFollowedByWHarness583Leg09Atom = "every b=SupportBean_B -[10]> d=SupportBean_D"
	patternFollowedByWHarness583Leg10Atom = "every (b=SupportBean_B -> every d=SupportBean_D)"
	patternFollowedByWHarness583Leg11Atom = "every (a_1=SupportBean_A() -> b=SupportBean_B -> a_2=SupportBean_A)"
	patternFollowedByWHarness583Leg12Atom = "c=SupportBean_C() -> d=SupportBean_D -> a=SupportBean_A"
	patternFollowedByWHarness583Leg13Atom = "every (a_1=SupportBean_A() -> b=SupportBean_B() -> a_2=SupportBean_A())"
	patternFollowedByWHarness583Leg14Atom = "every (a_1=SupportBean_A() -[10]> b=SupportBean_B() -[10]> a_2=SupportBean_A())"
	patternFollowedByWHarness583Leg15Atom = "every ( every a=SupportBean_A -> every b=SupportBean_B)"
	patternFollowedByWHarness583Leg16Atom = "every (a=SupportBean_A() -> every b=SupportBean_B())"

	patternFollowedByWHarness583BeanAType = "SupportBean_A"
	patternFollowedByWHarness583BeanBType = "SupportBean_B"
	patternFollowedByWHarness583BeanCType = "SupportBean_C"
	patternFollowedByWHarness583BeanDType = "SupportBean_D"
	patternFollowedByWHarness583BeanEType = "SupportBean_E"
	patternFollowedByWHarness583BeanFType = "SupportBean_F"
	patternFollowedByWHarness583BeanGType = "SupportBean_G"
)

// patternFollowedByWHarness583StatementText wraps a pinned atom in the
// PatternTestHarness USE_EPL statement text: the @name annotation is
// `name--` + the atom (StringEscapeUtils.escapeJava is an identity on
// these atoms — no quotes, backslashes or control characters), followed
// by the two pattern-audit annotations.
func patternFollowedByWHarness583StatementText(atom string) string {
	return "@name(\"name--" + atom + "\") @Audit('pattern') @Audit('pattern-instances') select * from pattern [" + atom + "]"
}

// patternFollowedByWHarness583LegNames are the statement names the Java
// listeners register under: nameOfStatement is `name--` plus the raw
// atom text.
func patternFollowedByWHarness583LegName(atom string) string {
	return "name--" + atom
}

// Per-case Java identity: the single w-harness case owns the ord 0
// runtimeId; legs index into the atom table below.
var (
	patternFollowedByWHarness583JavaRuntimeIDs = []string{
		"java-runtime-d896ea164e3cebd5c27e",
	}
	patternFollowedByWHarness583JavaSources = []string{
		patternFollowedByWHarness583JavaSource,
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/patternassert/EventCollectionFactory.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/patternassert/PatternTestHarness.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_C.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_D.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_E.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_F.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_G.java",
	}
	patternFollowedByWHarness583JavaExecutions = []string{
		"PatternOpWHarness",
	}
	// Static-manifest id for the ord 0 execution (discovery
	// static-candidate). The deduplicated inventory id
	// java-089b2086c9945dff918f — shared by all ten
	// PatternOperatorFollowedBy rows — is NOT the static id here:
	// where a per-execution static id exists the per-runtime row
	// carries it (infra-namedwindow-explicit-index-566 precedent).
	patternFollowedByWHarness583JavaStaticIDs = []string{
		"java-971bf7dfac84756cdd0a",
	}
	patternFollowedByWHarness583JavaFlags = []string{}
)

// patternFollowedByWHarness583BeanA..G mirror SupportBean_A..G: each
// carries the single id property select * projects as a fragment.
type patternFollowedByWHarness583BeanA struct {
	ID string `json:"id" esper:"id"`
}
type patternFollowedByWHarness583BeanB struct {
	ID string `json:"id" esper:"id"`
}
type patternFollowedByWHarness583BeanC struct {
	ID string `json:"id" esper:"id"`
}
type patternFollowedByWHarness583BeanD struct {
	ID string `json:"id" esper:"id"`
}
type patternFollowedByWHarness583BeanE struct {
	ID string `json:"id" esper:"id"`
}
type patternFollowedByWHarness583BeanF struct {
	ID string `json:"id" esper:"id"`
}
type patternFollowedByWHarness583BeanG struct {
	ID string `json:"id" esper:"id"`
}

// patternFollowedByWHarness583CaseSpec pins the single case: Java
// execution identity (ordinal plus runtimeIndex), the observation text
// and the sixteen byte-exact deployed statement texts — one per deploy
// step in Java source order.
type patternFollowedByWHarness583CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epls         []string
}

var patternFollowedByWHarness583LegAtoms = []string{
	patternFollowedByWHarness583Leg01Atom,
	patternFollowedByWHarness583Leg02Atom,
	patternFollowedByWHarness583Leg03Atom,
	patternFollowedByWHarness583Leg04Atom,
	patternFollowedByWHarness583Leg05Atom,
	patternFollowedByWHarness583Leg06Atom,
	patternFollowedByWHarness583Leg07Atom,
	patternFollowedByWHarness583Leg08Atom,
	patternFollowedByWHarness583Leg09Atom,
	patternFollowedByWHarness583Leg10Atom,
	patternFollowedByWHarness583Leg11Atom,
	patternFollowedByWHarness583Leg12Atom,
	patternFollowedByWHarness583Leg13Atom,
	patternFollowedByWHarness583Leg14Atom,
	patternFollowedByWHarness583Leg15Atom,
	patternFollowedByWHarness583Leg16Atom,
}

var patternFollowedByWHarness583CaseSpecs = []patternFollowedByWHarness583CaseSpec{
	{
		name:         "w-harness",
		ordinal:      0,
		runtimeIndex: 0,
		observation: "listener; external clock +1000ms per send; sixteen legs deploy before any send " +
			"(harness deploys all statements, then replays the event set): leg1/2 B1{b,B1;d,null}+D1{b,B1;d,D1}, " +
			"leg3 D1/D2/D3 {b,B1;d,Dn}, leg4 D1{b,B1;d,D1}, leg5/6 B1{b,B1;d,null}, leg7 D1 2rows/D2 2rows/D3 3rows, " +
			"leg8 D1 2rows/D3 {b,B3;d,D3}, leg9 same, leg10 D3 fires {b,B1;d,D3},{b,B3;d,D3},{b,B3;d,D3} (dup tail), " +
			"leg11 A2{a_1,A1;b,B1;a_2,A2}, leg12 silent, leg13/14 same as 11, " +
			"leg15 B3 fires {a,A1;b,B3},{a,A2;b,B3}x3 (dup tail), leg16 x2; " +
			"each fire event delivers its rows in ONE listener invocation per leg (Java getLastNewData multiset pin)",
		epls: func() []string {
			epls := make([]string, 0, len(patternFollowedByWHarness583LegAtoms))
			for _, atom := range patternFollowedByWHarness583LegAtoms {
				epls = append(epls, patternFollowedByWHarness583StatementText(atom))
			}
			return epls
		}(),
	},
}

// patternFollowedByWHarness583StepPin pins one scenario step's shape:
// deploys carry the byte-exact USE_EPL statement text plus the pinned
// name--<atom> statement label, advance-time steps pin the event-time
// instant and sends pin the event payload.
type patternFollowedByWHarness583StepPin struct {
	op        string
	statement string
	epl       string
	at        string
	eventType string
	payload   map[string]any
}

func patternFollowedByWHarness583DeployPin(atom string) patternFollowedByWHarness583StepPin {
	return patternFollowedByWHarness583StepPin{
		op:        "deploy",
		statement: patternFollowedByWHarness583LegName(atom),
		epl:       patternFollowedByWHarness583StatementText(atom),
	}
}

func patternFollowedByWHarness583AdvancePin(at string) patternFollowedByWHarness583StepPin {
	return patternFollowedByWHarness583StepPin{op: "advance-time", at: at}
}

func patternFollowedByWHarness583SendPin(eventType string, payload map[string]any) patternFollowedByWHarness583StepPin {
	return patternFollowedByWHarness583StepPin{op: "send", eventType: eventType, payload: payload}
}

func patternFollowedByWHarness583IDSendPin(eventType, id string) patternFollowedByWHarness583StepPin {
	return patternFollowedByWHarness583SendPin(eventType, map[string]any{"id": id})
}

func patternFollowedByWHarness583UndeployAllPin() patternFollowedByWHarness583StepPin {
	return patternFollowedByWHarness583StepPin{op: "undeploy-all"}
}

// patternFollowedByWHarness583EventSet pins the getEventSetOne(0,1000)
// replay: makeExternalClockTimes stamps ON_START at base 0 and each
// event at +1000ms in makeMixedSet insertion order — A1=1000, B1=2000,
// C1=3000, B2=4000, A2=5000, D1=6000, E1=7000, F1=8000, D2=9000,
// B3=10000, G1=11000, D3=12000. The harness advances the clock BEFORE
// each send, so every (advance-time, send) pair is pinned.
func patternFollowedByWHarness583EventAt(offsetMS int) string {
	return time.Unix(0, 0).UTC().Add(time.Duration(offsetMS) * time.Millisecond).Format(time.RFC3339Nano)
}

var patternFollowedByWHarness583EventSet = []struct {
	at        int
	eventType string
	id        string
}{
	{1000, patternFollowedByWHarness583BeanAType, "A1"},
	{2000, patternFollowedByWHarness583BeanBType, "B1"},
	{3000, patternFollowedByWHarness583BeanCType, "C1"},
	{4000, patternFollowedByWHarness583BeanBType, "B2"},
	{5000, patternFollowedByWHarness583BeanAType, "A2"},
	{6000, patternFollowedByWHarness583BeanDType, "D1"},
	{7000, patternFollowedByWHarness583BeanEType, "E1"},
	{8000, patternFollowedByWHarness583BeanFType, "F1"},
	{9000, patternFollowedByWHarness583BeanDType, "D2"},
	{10000, patternFollowedByWHarness583BeanBType, "B3"},
	{11000, patternFollowedByWHarness583BeanGType, "G1"},
	{12000, patternFollowedByWHarness583BeanDType, "D3"},
}

// patternFollowedByWHarness583CaseSteps pins the complete step sequence
// in harness order: advanceTime(ON_START=0) precedes the sixteen
// deployments (the USE_EPL style compiles all statements before the
// event loop), then each event's advance-time+send pair, then
// undeploy-all. env.milestone savepoints, the three other replay styles
// and the post-undeploy resend carry no scenario op.
var patternFollowedByWHarness583CaseSteps = func() map[string][]patternFollowedByWHarness583StepPin {
	steps := make(map[string][]patternFollowedByWHarness583StepPin)
	for _, spec := range patternFollowedByWHarness583CaseSpecs {
		pins := []patternFollowedByWHarness583StepPin{
			patternFollowedByWHarness583AdvancePin(patternFollowedByWHarness583EventAt(0)),
		}
		for _, atom := range patternFollowedByWHarness583LegAtoms {
			pins = append(pins, patternFollowedByWHarness583DeployPin(atom))
		}
		for _, event := range patternFollowedByWHarness583EventSet {
			pins = append(pins,
				patternFollowedByWHarness583AdvancePin(patternFollowedByWHarness583EventAt(event.at)),
				patternFollowedByWHarness583IDSendPin(event.eventType, event.id))
		}
		pins = append(pins, patternFollowedByWHarness583UndeployAllPin())
		steps[spec.name] = pins
	}
	return steps
}()

func loadPatternFollowedByWHarness583Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternFollowedByWHarness583ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternFollowedByWHarness583ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternFollowedByWHarness583ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternFollowedByWHarness583ID, err)
	}
	if err := requirePatternFollowedByWHarness583Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternFollowedByWHarness583ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternFollowedByWHarness583ID ||
		metadata.Description != patternFollowedByWHarness583Description ||
		metadata.JavaCommit != patternFollowedByWHarness583JavaCommit ||
		metadata.JavaSource != patternFollowedByWHarness583JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternFollowedByWHarness583ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternFollowedByWHarness583JavaSources},
		{metadata.JavaRuntimes, patternFollowedByWHarness583JavaRuntimeIDs},
		{metadata.JavaNames, patternFollowedByWHarness583JavaExecutions},
		{metadata.JavaStaticIDs, patternFollowedByWHarness583JavaStaticIDs},
		{metadata.JavaFlags, patternFollowedByWHarness583JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternFollowedByWHarness583ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternFollowedByWHarness583CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternFollowedByWHarness583ID, len(patternFollowedByWHarness583CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternFollowedByWHarness583Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternFollowedByWHarness583CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternFollowedByWHarness583JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternFollowedByWHarness583JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || !reflect.DeepEqual(definition.EPLs, spec.epls) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternFollowedByWHarness583ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternFollowedByWHarness583ID)
	}
	if err := validatePatternFollowedByWHarness583RawSteps(rawSteps); err != nil {
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

// validatePatternFollowedByWHarness583RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned sequence — the leading ON_START advance-time, the
// sixteen name--<atom> deploys with verbatim USE_EPL texts, the twelve
// advance-time+send pairs and the closing undeploy-all.
func validatePatternFollowedByWHarness583RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternFollowedByWHarness583CaseSpecs))
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
			if err := requirePatternFollowedByWHarness583Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternFollowedByWHarness583CaseSteps[marker.Case]; !ok {
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
		pins := patternFollowedByWHarness583CaseSteps[currentCase]
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
			if err := requirePatternFollowedByWHarness583Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternFollowedByWHarness583Fields(object, "op", "case", "eventType", "payload"); err != nil {
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
			if err := requirePatternFollowedByWHarness583Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time %q is not the pinned %q for case %q",
					index, step.At, pin.at, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternFollowedByWHarness583Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternFollowedByWHarness583CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternFollowedByWHarness583CaseSpecs))
	}
	for index, spec := range patternFollowedByWHarness583CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternFollowedByWHarness583CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternFollowedByWHarness583CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternFollowedByWHarness583Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternFollowedByWHarness583CaseState carries the replay state: the
// deployed per-leg statements keyed by their name--<atom> labels, the
// deployments (one per leg, matching the harness's per-descriptor
// deploy), per-statement listener sequence counters and the delivery
// records.
type patternFollowedByWHarness583CaseState struct {
	spec        patternFollowedByWHarness583CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	statements  map[string]*esper.Statement
	deployments []*esper.Deployment
	sequence    map[string]uint64
	records     []compat.TraceRecord
	deployIndex int
}

// runPatternFollowedByWHarness583Scenario replays the execution against
// a fresh engine like the Java oracle's fresh per-execution runtime —
// the sixteen legs share ONE engine because the harness deploys all
// statements before replaying the event set.
func runPatternFollowedByWHarness583Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternFollowedByWHarness583Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternFollowedByWHarness583CaseSpecs {
		records, err := runPatternFollowedByWHarness583Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternFollowedByWHarness583ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternFollowedByWHarness583Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternFollowedByWHarness583ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternFollowedByWHarness583ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternFollowedByWHarness583RawSteps(rawSteps)
}

func runPatternFollowedByWHarness583Case(ctx context.Context, scenario compat.Scenario, spec patternFollowedByWHarness583CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternFollowedByWHarness583BeanA](env, patternFollowedByWHarness583BeanAType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByWHarness583BeanB](env, patternFollowedByWHarness583BeanBType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByWHarness583BeanC](env, patternFollowedByWHarness583BeanCType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByWHarness583BeanD](env, patternFollowedByWHarness583BeanDType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByWHarness583BeanE](env, patternFollowedByWHarness583BeanEType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByWHarness583BeanF](env, patternFollowedByWHarness583BeanFType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByWHarness583BeanG](env, patternFollowedByWHarness583BeanGType); err != nil {
		return nil, err
	}
	state := &patternFollowedByWHarness583CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternFollowedByWHarness583JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			payload, err := patternFollowedByWHarness583DecodePayload(step)
			if err != nil {
				return nil, err
			}
			// Esper delivers one event's pattern completions to statement
			// listeners in REVERSE deployment order (the most recently
			// deployed statement receives its callback first — observed
			// across every multi-leg send in the oracle trace), while the
			// Go engine dispatches in deployment order. Buffer the
			// deliveries produced by this send and append them newest-
			// statement-first so the record stream matches the Java
			// observable contract.
			buffered := len(state.records)
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
			for left, right := buffered, len(state.records)-1; left < right; left, right = left+1, right-1 {
				state.records[left], state.records[right] = state.records[right], state.records[left]
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse advance-time %q: %w", patternFollowedByWHarness583ID, step.At, err)
			}
			if err := state.engine.AdvanceTime(ctx, at); err != nil {
				return nil, err
			}
		case "undeploy-all":
			for _, deployment := range state.deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return nil, err
				}
			}
			state.deployments = nil
			state.statements = make(map[string]*esper.Statement)
			state.sequence = make(map[string]uint64)
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternFollowedByWHarness583ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps each pinned USE_EPL statement onto its fluent Go
// equivalent, mirroring
// internal/esper/pattern_followedby_parity_test.go's
// TestPatternOperatorFollowedByWHarnessMatchesEsper table — the deploy
// position selects the leg:
//   - leg 1 `b -> (d or not d)`: Then(b, Or(d, Not(d))) — the or-not
//     right side is spawn-satisfied so the leg fires AT B1 with d vacant
//   - leg 2: same with ThenMax(1000) — the bound never binds on this
//     clock (gaps are 1000ms but or-not completes at spawn)
//   - leg 3 `b -> every d`: Then(b, Every(d))
//   - leg 4 `b -> d`: Then(b, d)
//   - leg 5 `b -> not d`: Then(b, Not(d)) — spawn-satisfied, fires at B1
//   - leg 6: same with ThenMax(1000)
//   - leg 7 `every b -> every d`: Every(b).Then(Every(d))
//   - leg 8 `every b -> d`: Every(b).Then(d)
//   - leg 9 `every b -[10]> d`: Every(b).ThenMax(10, d)
//   - leg 10 `every (b -> every d)`: the outer Every wraps the completed
//     sequence — D3 re-fires {b,B3;d,D3} a second time (dup row is
//     normative Java multiplicity)
//   - leg 11 `every (a_1 -> b -> a_2)`: Every over the 3-deep Then chain
//   - leg 12 `c -> d -> a`: normative zero-fire — no A follows any D
//   - leg 13/14: chain variants (parenthesized, -[10]> edges) — same Go
//     AST modulo ThenMax
//   - leg 15 `every ( every a -> every b)`: nested everys — B3 re-fires
//     {a,A2;b,B3} three times
//   - leg 16 `every (a -> every b)`: outer-every re-arm — {a,A2;b,B3}
//     twice at B3
//
// select * expands to Alias(tag, PatternEvent tag) for every named tag —
// legs carry b/d, a_1/b/a_2, c/d/a or a/b — matching Esper's fragment
// projection; vacant tags (legs 1/2/5/6's d at B1) evaluate to Null and
// render {"state":"null"} exactly like the Java row.
func (s *patternFollowedByWHarness583CaseState) deploy(ctx context.Context, step compat.Step) error {
	if s.deployIndex >= len(s.spec.epls) || step.Epl != s.spec.epls[s.deployIndex] ||
		step.Statement != patternFollowedByWHarness583LegName(patternFollowedByWHarness583LegAtoms[s.deployIndex]) {
		return fmt.Errorf("%s: case %q deploy %d is not pinned", patternFollowedByWHarness583ID, s.spec.name, s.deployIndex)
	}
	if _, ok := s.statements[step.Statement]; ok {
		return fmt.Errorf("%s: label %q is already deployed", patternFollowedByWHarness583ID, step.Statement)
	}
	leg := s.deployIndex + 1
	always := esper.Literal[bool](true)
	a := esper.From[patternFollowedByWHarness583BeanA](s.env, patternFollowedByWHarness583BeanAType)
	b := esper.From[patternFollowedByWHarness583BeanB](s.env, patternFollowedByWHarness583BeanBType)
	c := esper.From[patternFollowedByWHarness583BeanC](s.env, patternFollowedByWHarness583BeanCType)
	d := esper.From[patternFollowedByWHarness583BeanD](s.env, patternFollowedByWHarness583BeanDType)
	var pattern esper.PatternStream
	var tags []string
	switch leg {
	case 1:
		pattern = esper.PatternFrom(b, "b", always).
			Then(esper.PatternFrom(d, "d", always).Or(esper.PatternFrom(d, "d", always).Not()))
		tags = []string{"b", "d"}
	case 2:
		pattern = esper.PatternFrom(b, "b", always).
			ThenMax(1000, esper.PatternFrom(d, "d", always).Or(esper.PatternFrom(d, "d", always).Not()))
		tags = []string{"b", "d"}
	case 3:
		pattern = esper.PatternFrom(b, "b", always).
			Then(esper.PatternFrom(d, "d", always).Every())
		tags = []string{"b", "d"}
	case 4:
		pattern = esper.PatternFrom(b, "b", always).
			Then(esper.PatternFrom(d, "d", always))
		tags = []string{"b", "d"}
	case 5:
		pattern = esper.PatternFrom(b, "b", always).
			Then(esper.PatternFrom(d, "d", always).Not())
		tags = []string{"b", "d"}
	case 6:
		pattern = esper.PatternFrom(b, "b", always).
			ThenMax(1000, esper.PatternFrom(d, "d", always).Not())
		tags = []string{"b", "d"}
	case 7:
		pattern = esper.PatternFrom(b, "b", always).Every().
			Then(esper.PatternFrom(d, "d", always).Every())
		tags = []string{"b", "d"}
	case 8:
		pattern = esper.PatternFrom(b, "b", always).Every().
			Then(esper.PatternFrom(d, "d", always))
		tags = []string{"b", "d"}
	case 9:
		pattern = esper.PatternFrom(b, "b", always).Every().
			ThenMax(10, esper.PatternFrom(d, "d", always))
		tags = []string{"b", "d"}
	case 10:
		pattern = esper.PatternFrom(b, "b", always).
			Then(esper.PatternFrom(d, "d", always).Every()).Every()
		tags = []string{"b", "d"}
	case 11:
		pattern = esper.PatternFrom(a, "a_1", always).
			Then(esper.PatternFrom(b, "b", always)).
			Then(esper.PatternFrom(a, "a_2", always)).Every()
		tags = []string{"a_1", "b", "a_2"}
	case 12:
		pattern = esper.PatternFrom(c, "c", always).
			Then(esper.PatternFrom(d, "d", always)).
			Then(esper.PatternFrom(a, "a", always))
		tags = []string{"c", "d", "a"}
	case 13:
		pattern = esper.PatternFrom(a, "a_1", always).
			Then(esper.PatternFrom(b, "b", always)).
			Then(esper.PatternFrom(a, "a_2", always)).Every()
		tags = []string{"a_1", "b", "a_2"}
	case 14:
		pattern = esper.PatternFrom(a, "a_1", always).
			ThenMax(10, esper.PatternFrom(b, "b", always)).
			ThenMax(10, esper.PatternFrom(a, "a_2", always)).Every()
		tags = []string{"a_1", "b", "a_2"}
	case 15:
		pattern = esper.PatternFrom(a, "a", always).Every().
			Then(esper.PatternFrom(b, "b", always).Every()).Every()
		tags = []string{"a", "b"}
	case 16:
		pattern = esper.PatternFrom(a, "a", always).
			Then(esper.PatternFrom(b, "b", always).Every()).Every()
		tags = []string{"a", "b"}
	default:
		return fmt.Errorf("%s: unsupported leg %d", patternFollowedByWHarness583ID, leg)
	}
	selections := make([]esper.Selection, 0, len(tags))
	for _, tag := range tags {
		selections = append(selections, esper.Alias(tag, esper.PatternEvent(tag)))
	}
	query := pattern.Select(selections...).Query(esper.StatementName(step.Statement))
	plan, err := s.env.Build(query)
	if err != nil {
		return err
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	statement := deployment.Statements()[0]
	s.deployments = append(s.deployments, deployment)
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

// patternFollowedByWHarness583DecodePayload converts a scenario send
// payload into the typed host object: every SupportBean_? type carries
// the single pinned id property.
func patternFollowedByWHarness583DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	if err := requirePatternFollowedByWHarness583Fields(fields, "id"); err != nil {
		return nil, err
	}
	switch step.EventType {
	case patternFollowedByWHarness583BeanAType:
		var event patternFollowedByWHarness583BeanA
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return event, nil
	case patternFollowedByWHarness583BeanBType:
		var event patternFollowedByWHarness583BeanB
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_B: %w", err)
		}
		return event, nil
	case patternFollowedByWHarness583BeanCType:
		var event patternFollowedByWHarness583BeanC
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_C: %w", err)
		}
		return event, nil
	case patternFollowedByWHarness583BeanDType:
		var event patternFollowedByWHarness583BeanD
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_D: %w", err)
		}
		return event, nil
	case patternFollowedByWHarness583BeanEType:
		var event patternFollowedByWHarness583BeanE
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_E: %w", err)
		}
		return event, nil
	case patternFollowedByWHarness583BeanFType:
		var event patternFollowedByWHarness583BeanF
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_F: %w", err)
		}
		return event, nil
	case patternFollowedByWHarness583BeanGType:
		var event patternFollowedByWHarness583BeanG
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_G: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternFollowedByWHarness583ID, step.EventType)
	}
}
