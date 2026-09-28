package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for PatternGuardWhile ords 0-3: four executions
// (static java-2be6b5626499eeff834e, flags []) replayed as four cases,
// each on a fresh engine like the Java oracle's fresh per-execution
// runtime.
//
//   - "simple" (ord 0, PatternGuardWhileSimple): deploys
//     `@Name('s0') select a.theString as c0 from pattern [(every
//     a=SupportBean) while (a.theString like 'E%')]` — the fluent
//     `(every a) while (...)` maps to PatternFrom().Every().WhileGuard,
//     which materializes the pending definition-level every inside the
//     guarded child exactly like Esper's ExpressionGuard. E1/E2 fire
//     new {c0}; the first X match falsifies the guard permanently
//     (guardQuit + evaluateFalse), so E3 and the trailing X stay
//     silent. env.milestone(0/1/2) savepoints and undeployAll carry no
//     records.
//   - "pattern-op" (ord 1, PatternOp): the five-leg PatternTestHarness
//     over EventCollectionFactory.getEventSetOne(0,1000). ONE deploy
//     step (statement "all", leg texts pinned through case.epls)
//     expands to ONE DeployPlans call carrying the five legs as
//     `@name("S<i>") select * from pattern [<atom>]` (S0..S4 stand in
//     for the harness's name--<atom> labels; S3 is the SODA model leg
//     whose pinned toEPL text keeps `b.id!="B3"` double-quoted and
//     deploys through the module path on the Java side), then twelve
//     fused advance-before-send steps (advanceTime precedes
//     sendEventBean) and undeploy-all plus the kill-resend silence
//     check. Per (statement, trigger-event) the harness compares the
//     listener's last delivery as a multiset (compareLists), so the
//     -diff path sorts both traces into (case, time, statement)
//     buckets. S0 fires B1{a,b} then quits at B2; S1 fires B1,B2 then
//     quits at B3; S2/S3 fire B1,B2 then quit at B3; S4 is the
//     zero-fire discriminant — the very first B1 match fails
//     `b.id != 'B1'` and quits the every permanently.
//   - "pattern-variable" (ord 2, PatternVariable): deploys the
//     create-variable EPL as an env.RegisterVariable fixture (the
//     deployment-scoped variable has no Go boundary) then the s0
//     pattern statement; B1 delivers ONE listener batch of two rows —
//     one per live every-a branch — while myVariable is true;
//     runtimeSetVariable('var','myVariable',false) maps to
//     engine.SetVariable and falsifies every live branch so A3/A4/B2
//     stay silent.
//   - "pattern-invalid" (ord 3, PatternInvalid): two tryInvalidCompile
//     probes as build-error steps. `while ('abc')` is a non-boolean
//     literal — a Go compile-time Expression[bool] type error, not
//     expressible, so the probe records the pinned Java message without
//     claiming a Go rejection boundary; `while (abc)` maps to an
//     unbound field guard that env.Build rejects, the Go-side
//     counterpart of 'Property named 'abc' is not valid in any stream'.
//
// Within one send Esper dispatches pattern completions to statement
// listeners in an unspecified internal order while the Go engine
// dispatches in deployment order; the harness contract compares per
// (statement, event) as a multiset, so the trace carries Go's natural
// order and the -diff path sorts both traces into (case, time,
// statement) buckets before comparing.
const patternGuardWhile590ID = "pattern-guard-while-590"

const patternGuardWhile590Description = "PatternGuardWhile ords 0-3 — the four expression-guard `while` executions replayed as four cases: (simple) PatternGuardWhileSimple deploys `@Name('s0') select a.theString as c0 from pattern [(every a=SupportBean) while (a.theString like 'E%')]` and sends E1,E2,X,E3,X — E1 and E2 fire new {c0:E1}/{c0:E2} while the first X match falsifies the while-guard permanently (ExpressionGuard.inspect returns FALSE -> guardQuit + evaluateFalse) so E3 and the trailing X stay silent; (pattern-op) PatternOp's five-leg PatternTestHarness over EventCollectionFactory.getEventSetOne(0,1000) replays as ONE deploy-all step (statement \"all\" expands to the five per-leg deployments of `@name(\"S<i>\") select * from pattern [<atom>]`, S3 carrying the SODA model's `select * from pattern [(every b=SupportBean_B) while (b.id!=\"B3\")]` toEPL text with double-quoted literal) then twelve advance-before-send steps (+1000ms each) and undeploy-all — S0 fires B1{a:A1,b:B1} then quits at B2 (b.id!='B2'), S1 fires B1{a:A1,b:B1}/B2{a:A1,b:B2} then quits at B3, S2 and S3 fire B1{b:B1}/B2{b:B2} then quit at B3, S4 stays silent (the very first B1 match fails b.id!='B1'); (pattern-variable) PatternVariable deploys `@name('var') @public create variable boolean myVariable = true` plus `@name('s0') select * from pattern [every a=SupportBean(theString like 'A%') -> (every b=SupportBean(theString like 'B%')) while (myVariable)]` — B1 delivers ONE listener batch of two rows (a:A1,b:B1)/(a:A2,b:B1), one per live every-a branch — then runtimeSetVariable('var','myVariable',false) falsifies every live branch so A3/A4/B2 stay silent; (pattern-invalid) PatternInvalid's two tryInvalidCompile probes record the pinned compile-error messages for the non-boolean literal guard `while ('abc')` and the unresolvable property guard `while (abc)`, each compiled without the runtime path. env.milestone savepoints, the COMPILE_TO_MODEL/COMPILE_TO_EPL/consume+suppress replay styles, the @Audit('pattern')/@Audit('pattern-instances') annotations and the post-undeploy statement lookup are unrepresented harness machinery."

const patternGuardWhile590JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternGuardWhile590JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternGuardWhile.java"

const (
	patternGuardWhile590BeanType  = "SupportBean"
	patternGuardWhile590BeanAType = "SupportBean_A"
	patternGuardWhile590BeanBType = "SupportBean_B"
	patternGuardWhile590BeanCType = "SupportBean_C"
	patternGuardWhile590BeanDType = "SupportBean_D"
	patternGuardWhile590BeanEType = "SupportBean_E"
	patternGuardWhile590BeanFType = "SupportBean_F"
	patternGuardWhile590BeanGType = "SupportBean_G"
)

// Byte-exact EPL texts (PatternGuardWhile.java verbatim): ord 0's
// single statement, the five ord 1 pattern atoms in EventExpressionCase
// order (leg 3 is the SODA model leg — its pinned text is the model's
// toEPL output `(every b=SupportBean_B) while (b.id!="B3")` with the
// double-quoted literal, wrapped like the other legs), ord 2's
// create-variable plus select pair, and ord 3's two rejected probes.
const patternGuardWhile590SimpleEPL = "@Name('s0') select a.theString as c0 from pattern " +
	"[(every a=SupportBean) while (a.theString like 'E%')]"

const (
	patternGuardWhile590VariableCreateEPL = "@name('var') @public create variable boolean myVariable = true"
	patternGuardWhile590VariableSelectEPL = "@name('s0') select * from pattern [every a=SupportBean(theString like 'A%') -> " +
		"(every b=SupportBean(theString like 'B%')) while (myVariable)]"
)

const (
	patternGuardWhile590InvalidLiteralEPL  = "select * from pattern [every SupportBean while ('abc')]"
	patternGuardWhile590InvalidPropertyEPL = "select * from pattern [every SupportBean while (abc)]"
)

// Pinned Java compile-error messages (PatternInvalid's tryInvalidCompile
// expected strings — assertMessage is a startsWith assertion; the pinned
// texts carry the full message including the bracketed statement source).
const (
	patternGuardWhile590InvalidLiteralError = "Invalid parameter for pattern guard 'SupportBean while (\"abc\")': Expression " +
		"pattern guard requires a single expression as a parameter returning a true or false (boolean) value " +
		"[select * from pattern [every SupportBean while ('abc')]]"
	patternGuardWhile590InvalidPropertyError = "Failed to validate pattern guard expression 'abc': Property named 'abc' is not " +
		"valid in any stream [select * from pattern [every SupportBean while (abc)]]"
)

// patternGuardWhile590StatementText wraps a pinned ord-1 atom in the
// deployed statement text: @name("S<i>") plus select * from pattern
// [<atom>] — the S0..S4 names replace the harness's name--<atom>
// labels while keeping the observable listener surface identical.
func patternGuardWhile590StatementText(leg int, atom string) string {
	return fmt.Sprintf("@name(\"S%d\") select * from pattern [%s]", leg, atom)
}

// patternGuardWhile590LegName is the statement name leg i deploys
// under and the label the listener records carry.
func patternGuardWhile590LegName(leg int) string {
	return fmt.Sprintf("S%d", leg)
}

// Per-case Java identity: each case owns its ordinal's runtimeId; the
// four executions share the collection's single static id.
var (
	patternGuardWhile590JavaRuntimeIDs = []string{
		"java-runtime-aa6c31e987ec865a8cf2",
		"java-runtime-bf4b4c7d66f189e40a73",
		"java-runtime-8808f29a3bfdd475589f",
		"java-runtime-4416cc3367e38623077a",
	}
	patternGuardWhile590JavaSources = []string{
		patternGuardWhile590JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/patternassert/EventCollectionFactory.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/patternassert/EventExpressionCase.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/patternassert/PatternTestHarness.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_C.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_D.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_E.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_F.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_G.java",
	}
	patternGuardWhile590JavaExecutions = []string{
		"PatternGuardWhileSimple",
		"PatternOp",
		"PatternVariable",
		"PatternInvalid",
	}
	patternGuardWhile590JavaStaticIDs = []string{
		"java-2be6b5626499eeff834e",
		"java-2be6b5626499eeff834e",
		"java-2be6b5626499eeff834e",
		"java-2be6b5626499eeff834e",
	}
	patternGuardWhile590JavaFlags = []string{}
)

// patternGuardWhile590Bean mirrors the SupportBean properties the
// executions pin: theString (the like 'E%'/'A%'/'B%' filters and the c0
// projection) and intPrimitive (the payload's numeric field).
type patternGuardWhile590Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// patternGuardWhile590BeanA..G mirror SupportBean_A..G: each carries
// the single id property select * projects as a fragment.
type patternGuardWhile590BeanA struct {
	ID string `json:"id" esper:"id"`
}
type patternGuardWhile590BeanB struct {
	ID string `json:"id" esper:"id"`
}
type patternGuardWhile590BeanC struct {
	ID string `json:"id" esper:"id"`
}
type patternGuardWhile590BeanD struct {
	ID string `json:"id" esper:"id"`
}
type patternGuardWhile590BeanE struct {
	ID string `json:"id" esper:"id"`
}
type patternGuardWhile590BeanF struct {
	ID string `json:"id" esper:"id"`
}
type patternGuardWhile590BeanG struct {
	ID string `json:"id" esper:"id"`
}

// patternGuardWhile590LegAtoms are the five EventExpressionCase atoms
// in case-list order (statement S<i> deploys leg i). Leg 3's atom is
// the SODA model's toEPL pattern text — `while (b.id!="B3")` with the
// double-quoted literal, distinct from the EPL legs' `while(b.id !=
// 'B3')` spelling.
var patternGuardWhile590LegAtoms = []string{
	"a=SupportBean_A -> (every b=SupportBean_B) while(b.id != 'B2')", // S0
	"a=SupportBean_A -> (every b=SupportBean_B) while(b.id != 'B3')", // S1
	"(every b=SupportBean_B) while(b.id != 'B3')",                    // S2
	"(every b=SupportBean_B) while (b.id!=\"B3\")",                   // S3 (SODA model leg)
	"(every b=SupportBean_B) while(b.id != 'B1')",                    // S4
}

// patternGuardWhile590CaseSpec pins one case: Java execution identity
// (ordinal plus runtimeIndex into the shared per-ord arrays), the
// observation text and the byte-exact EPL texts pinned through
// case.epls.
type patternGuardWhile590CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epls         []string
}

var patternGuardWhile590CaseSpecs = []patternGuardWhile590CaseSpec{
	{
		name:         "simple",
		ordinal:      0,
		runtimeIndex: 0,
		observation: "listener; deploy s0 then sends E1,E2,X,E3,X at the engine start instant (no " +
			"external clock): E1 fires new {c0:E1}, E2 fires {c0:E2}, the first X " +
			"sends' falsifying match quits the guarded every permanently so E3 and " +
			"the second X deliver nothing; milestone(0/1/2) savepoints and " +
			"undeployAll carry no records",
		epls: []string{patternGuardWhile590SimpleEPL},
	},
	{
		name:         "pattern-op",
		ordinal:      1,
		runtimeIndex: 1,
		observation: "listener; external clock +1000ms per send; all five legs deploy in ONE step " +
			"before any send (harness deploy-all), then twelve advance-before-send " +
			"replays of A1,B1,C1,B2,A2,D1,E1,F1,D2,B3,G1,D3 (advanceTime precedes " +
			"sendEventBean): S0 B1{a:A1,b:B1} only (`b.id != 'B2'` quits at B2), " +
			"S1 B1{a:A1,b:B1}+B2{a:A1,b:B2} (`!= 'B3'` quits at B3), S2 B1{b:B1}+" +
			"B2{b:B2}, S3 the SODA model leg `b.id!=\"B3\"` fires B1{b:B1}+B2{b:B2}, " +
			"S4 silent (B1 falsifies `!= 'B1'` on the first match); per leg+trigger " +
			"the fires compare as a multiset like compareLists; undeployAll resend " +
			"silence",
		epls: func() []string {
			epls := make([]string, 0, len(patternGuardWhile590LegAtoms))
			for leg, atom := range patternGuardWhile590LegAtoms {
				epls = append(epls, patternGuardWhile590StatementText(leg, atom))
			}
			return epls
		}(),
	},
	{
		name:         "pattern-variable",
		ordinal:      2,
		runtimeIndex: 2,
		observation: "listener; deploy var (`@public create variable boolean myVariable = true`, " +
			"RegressionPath-scoped) then deploy s0: sends A1,A2 arm two every-a " +
			"branches, B1 delivers ONE new batch of two rows (a:A1,b:B1)/(a:A2,b:B1) " +
			"with the guard true; runtimeSetVariable('var','myVariable',false) " +
			"falsifies every live branch permanently (guardQuit+evaluateFalse) so " +
			"A3/A4/B2 deliver nothing; milestone(0/1) savepoints and undeployAll " +
			"carry no records",
		epls: []string{patternGuardWhile590VariableCreateEPL, patternGuardWhile590VariableSelectEPL},
	},
	{
		name:         "pattern-invalid",
		ordinal:      3,
		runtimeIndex: 3,
		observation: "compile-error; two tryInvalidCompile probes compiled without the runtime path " +
			"record the pinned Java messages: `while ('abc')` fails with 'Invalid " +
			"parameter for pattern guard \"SupportBean while (\\\"abc\\\")\": " +
			"Expression pattern guard requires a single expression as a parameter " +
			"returning a true or false (boolean) value' and `while (abc)` fails " +
			"with 'Failed to validate pattern guard expression \"abc\": Property " +
			"named 'abc' is not valid in any stream'",
		epls: []string{patternGuardWhile590InvalidLiteralEPL, patternGuardWhile590InvalidPropertyEPL},
	},
}

// patternGuardWhile590StepPin pins one scenario step's shape: deploy
// steps carry the statement label plus the byte-exact EPL (except the
// W-harness "all" deploy which pins through case.epls), send steps
// carry the optional fused advance-before-send instant plus the event
// payload, the set-variable step carries statement/name/payload and
// build-error steps carry statement/epl/expectError (the Java side is
// pinned to tryInvalidCompile's path-less compile).
type patternGuardWhile590StepPin struct {
	op          string
	statement   string
	epl         string
	at          string
	eventType   string
	payload     map[string]any
	setName     string
	setValue    any
	expectError string
}

func patternGuardWhile590DeployPin(statement, epl string) patternGuardWhile590StepPin {
	return patternGuardWhile590StepPin{op: "deploy", statement: statement, epl: epl}
}

func patternGuardWhile590DeployAllPin() patternGuardWhile590StepPin {
	return patternGuardWhile590StepPin{op: "deploy", statement: "all"}
}

func patternGuardWhile590SendPin(at, eventType string, payload map[string]any) patternGuardWhile590StepPin {
	return patternGuardWhile590StepPin{op: "send", at: at, eventType: eventType, payload: payload}
}

func patternGuardWhile590BeanSendPin(theString string, intPrimitive int) patternGuardWhile590StepPin {
	return patternGuardWhile590SendPin("", patternGuardWhile590BeanType,
		map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)})
}

func patternGuardWhile590IDSendPin(at int, eventType, id string) patternGuardWhile590StepPin {
	return patternGuardWhile590SendPin(patternGuardWhile590EventAt(at), eventType,
		map[string]any{"id": id})
}

func patternGuardWhile590SetVariablePin(statement, name string, value bool) patternGuardWhile590StepPin {
	return patternGuardWhile590StepPin{op: "set-variable", statement: statement,
		setName: name, setValue: value}
}

func patternGuardWhile590BuildErrorPin(statement, epl, expectError string) patternGuardWhile590StepPin {
	return patternGuardWhile590StepPin{op: "build-error", statement: statement, epl: epl,
		expectError: expectError}
}

func patternGuardWhile590UndeployAllPin() patternGuardWhile590StepPin {
	return patternGuardWhile590StepPin{op: "undeploy-all"}
}

// patternGuardWhile590EventAt renders one event-time instant: the
// pattern-op harness's sendEventCollection.getTime(eventId) advance —
// A1=1000ms .. D3=12000ms on the external clock.
func patternGuardWhile590EventAt(offsetMS int) string {
	return time.Unix(0, 0).UTC().Add(time.Duration(offsetMS) * time.Millisecond).Format(time.RFC3339Nano)
}

var patternGuardWhile590EventSet = []struct {
	at        int
	eventType string
	id        string
}{
	{1000, patternGuardWhile590BeanAType, "A1"},
	{2000, patternGuardWhile590BeanBType, "B1"},
	{3000, patternGuardWhile590BeanCType, "C1"},
	{4000, patternGuardWhile590BeanBType, "B2"},
	{5000, patternGuardWhile590BeanAType, "A2"},
	{6000, patternGuardWhile590BeanDType, "D1"},
	{7000, patternGuardWhile590BeanEType, "E1"},
	{8000, patternGuardWhile590BeanFType, "F1"},
	{9000, patternGuardWhile590BeanDType, "D2"},
	{10000, patternGuardWhile590BeanBType, "B3"},
	{11000, patternGuardWhile590BeanGType, "G1"},
	{12000, patternGuardWhile590BeanDType, "D3"},
}

// patternGuardWhile590CaseSteps pins the complete step sequence in
// execution order per case. env.milestone savepoints, the three other
// pattern-op replay styles and the ON_START advance-time carry no
// scenario op; the pattern-op undeploy-all's post-undeploy resend is
// the kill-resend silence check.
var patternGuardWhile590CaseSteps = map[string][]patternGuardWhile590StepPin{
	"simple": {
		patternGuardWhile590DeployPin("s0", patternGuardWhile590SimpleEPL),
		patternGuardWhile590BeanSendPin("E1", 0),
		patternGuardWhile590BeanSendPin("E2", 0),
		patternGuardWhile590BeanSendPin("X", 0),
		patternGuardWhile590BeanSendPin("E3", 0),
		patternGuardWhile590BeanSendPin("X", 0),
		patternGuardWhile590UndeployAllPin(),
	},
	"pattern-op": func() []patternGuardWhile590StepPin {
		pins := []patternGuardWhile590StepPin{patternGuardWhile590DeployAllPin()}
		for _, event := range patternGuardWhile590EventSet {
			pins = append(pins,
				patternGuardWhile590IDSendPin(event.at, event.eventType, event.id))
		}
		pins = append(pins, patternGuardWhile590UndeployAllPin())
		return pins
	}(),
	"pattern-variable": {
		patternGuardWhile590DeployPin("var", patternGuardWhile590VariableCreateEPL),
		patternGuardWhile590DeployPin("s0", patternGuardWhile590VariableSelectEPL),
		patternGuardWhile590BeanSendPin("A1", 1),
		patternGuardWhile590BeanSendPin("A2", 2),
		patternGuardWhile590BeanSendPin("B1", 100),
		patternGuardWhile590SetVariablePin("var", "myVariable", false),
		patternGuardWhile590BeanSendPin("A3", 3),
		patternGuardWhile590BeanSendPin("A4", 4),
		patternGuardWhile590BeanSendPin("B2", 200),
		patternGuardWhile590UndeployAllPin(),
	},
	"pattern-invalid": {
		patternGuardWhile590BuildErrorPin("non-boolean-literal-guard",
			patternGuardWhile590InvalidLiteralEPL, patternGuardWhile590InvalidLiteralError),
		patternGuardWhile590BuildErrorPin("unresolvable-property-guard",
			patternGuardWhile590InvalidPropertyEPL, patternGuardWhile590InvalidPropertyError),
	},
}

func loadPatternGuardWhile590Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternGuardWhile590ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternGuardWhile590ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternGuardWhile590ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternGuardWhile590ID, err)
	}
	if err := requirePatternGuardWhile590Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternGuardWhile590ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternGuardWhile590ID ||
		metadata.Description != patternGuardWhile590Description ||
		metadata.JavaCommit != patternGuardWhile590JavaCommit ||
		metadata.JavaSource != patternGuardWhile590JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternGuardWhile590ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternGuardWhile590JavaSources},
		{metadata.JavaRuntimes, patternGuardWhile590JavaRuntimeIDs},
		{metadata.JavaNames, patternGuardWhile590JavaExecutions},
		{metadata.JavaStaticIDs, patternGuardWhile590JavaStaticIDs},
		{metadata.JavaFlags, patternGuardWhile590JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternGuardWhile590ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternGuardWhile590CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternGuardWhile590ID, len(patternGuardWhile590CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternGuardWhile590Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternGuardWhile590CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternGuardWhile590JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternGuardWhile590JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || !reflect.DeepEqual(definition.EPLs, spec.epls) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternGuardWhile590ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternGuardWhile590ID)
	}
	if err := validatePatternGuardWhile590RawSteps(rawSteps); err != nil {
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

// validatePatternGuardWhile590RawSteps pins the complete step sequence:
// op field whitelists per step kind plus positional comparison against
// the pinned per-case sequences — the deploy/send/undeploy-all run for
// "simple", the deploy-all + twelve fused advance-before-send +
// undeploy-all W-harness replay for "pattern-op", the two deploy +
// sends + set-variable + sends + undeploy-all sequence for
// "pattern-variable" and the two build-error probes for
// "pattern-invalid".
func validatePatternGuardWhile590RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternGuardWhile590CaseSpecs))
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
			At          string          `json:"at"`
			EventType   string          `json:"eventType"`
			Payload     json.RawMessage `json:"payload"`
			Name        string          `json:"name"`
			ExpectError string          `json:"expectError"`
		}
		if operation == "case" {
			if err := requirePatternGuardWhile590Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternGuardWhile590CaseSteps[marker.Case]; !ok {
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
		pins := patternGuardWhile590CaseSteps[currentCase]
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
			if pin.statement == "all" {
				if err := requirePatternGuardWhile590Fields(object, "op", "case", "statement"); err != nil {
					return fmt.Errorf("scenario step %d: %w", index, err)
				}
			} else {
				if err := requirePatternGuardWhile590Fields(object, "op", "case", "statement", "epl"); err != nil {
					return fmt.Errorf("scenario step %d: %w", index, err)
				}
				if step.EPL != pin.epl {
					return fmt.Errorf("scenario step %d deploy epl is not pinned for case %q", index, currentCase)
				}
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if pin.at == "" {
				if err := requirePatternGuardWhile590Fields(object, "op", "case", "eventType", "payload"); err != nil {
					return fmt.Errorf("scenario step %d: %w", index, err)
				}
			} else {
				if err := requirePatternGuardWhile590Fields(object, "op", "case", "at", "eventType", "payload"); err != nil {
					return fmt.Errorf("scenario step %d: %w", index, err)
				}
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.At != pin.at || step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "set-variable":
			if err := requirePatternGuardWhile590Fields(object, "op", "case", "statement", "name", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var value any
			if err := json.Unmarshal(step.Payload, &value); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.Statement != pin.statement || step.Name != pin.setName ||
				!reflect.DeepEqual(value, pin.setValue) {
				return fmt.Errorf("scenario step %d set-variable is not pinned for case %q", index, currentCase)
			}
		case "build-error":
			if err := requirePatternGuardWhile590Fields(object, "op", "case", "statement",
				"epl", "expectError"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl ||
				step.ExpectError != pin.expectError {
				return fmt.Errorf("scenario step %d build-error is not pinned for case %q", index, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternGuardWhile590Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternGuardWhile590CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternGuardWhile590CaseSpecs))
	}
	for index, spec := range patternGuardWhile590CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternGuardWhile590CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternGuardWhile590CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternGuardWhile590Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternGuardWhile590CaseState carries the replay state: the deployed
// statements keyed by label (s0 or the S0..S4 leg names), per-statement
// listener sequence counters, the deployment handles and the delivery
// records.
type patternGuardWhile590CaseState struct {
	spec        patternGuardWhile590CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	deployments []*esper.Deployment
	statements  map[string]*esper.Statement
	sequence    map[string]uint64
	records     []compat.TraceRecord
	deployedAll bool
}

// runPatternGuardWhile590Scenario replays each execution against a
// fresh engine like the Java oracle's fresh per-execution runtime.
func runPatternGuardWhile590Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternGuardWhile590Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternGuardWhile590CaseSpecs {
		records, err := runPatternGuardWhile590Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternGuardWhile590ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternGuardWhile590Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternGuardWhile590ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternGuardWhile590ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternGuardWhile590RawSteps(rawSteps)
}

func runPatternGuardWhile590Case(ctx context.Context, scenario compat.Scenario, spec patternGuardWhile590CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if spec.name == "pattern-op" {
		if _, err := esper.RegisterStruct[patternGuardWhile590BeanA](env, patternGuardWhile590BeanAType); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[patternGuardWhile590BeanB](env, patternGuardWhile590BeanBType); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[patternGuardWhile590BeanC](env, patternGuardWhile590BeanCType); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[patternGuardWhile590BeanD](env, patternGuardWhile590BeanDType); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[patternGuardWhile590BeanE](env, patternGuardWhile590BeanEType); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[patternGuardWhile590BeanF](env, patternGuardWhile590BeanFType); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[patternGuardWhile590BeanG](env, patternGuardWhile590BeanGType); err != nil {
			return nil, err
		}
	} else {
		if _, err := esper.RegisterStruct[patternGuardWhile590Bean](env, patternGuardWhile590BeanType); err != nil {
			return nil, err
		}
	}
	state := &patternGuardWhile590CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternGuardWhile590JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			if step.Statement == "all" {
				if err := state.deployAll(ctx, step); err != nil {
					return nil, err
				}
				continue
			}
			if err := state.deployStep(ctx, step); err != nil {
				return nil, err
			}
		case "send":
			payload, err := patternGuardWhile590DecodePayload(step)
			if err != nil {
				return nil, err
			}
			// The pattern-op fused step performs the harness's
			// advanceTime BEFORE sendEventBean; the other cases carry
			// no advance (records time at the engine start instant).
			if step.At != "" {
				at, err := time.Parse(time.RFC3339Nano, step.At)
				if err != nil {
					return nil, fmt.Errorf("%s: parse send at %q: %w", patternGuardWhile590ID, step.At, err)
				}
				if err := state.engine.AdvanceTime(ctx, at); err != nil {
					return nil, err
				}
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "set-variable":
			// runtimeSetVariable("var","myVariable",false) — the
			// deployment-scoped write maps to the engine-level variable
			// set; the guard's next inspect reads the flipped value and
			// quits every live branch.
			var value bool
			if err := json.Unmarshal(step.Payload, &value); err != nil {
				return nil, fmt.Errorf("%s: decode set-variable payload: %w", patternGuardWhile590ID, err)
			}
			if err := state.engine.SetVariable(ctx, step.Name, value); err != nil {
				return nil, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
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
			state.deployedAll = false
			// The pattern-op harness resends the whole event set after
			// undeployAll to prove every listener is gone; the sends
			// must produce zero records.
			if spec.name == "pattern-op" {
				for _, event := range patternGuardWhile590EventSet {
					bean, err := patternGuardWhile590TypedEvent(event.eventType, event.id)
					if err != nil {
						return nil, err
					}
					if err := state.engine.Send(ctx, event.eventType, bean); err != nil {
						return nil, err
					}
				}
			}
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternGuardWhile590ID, step.Op)
		}
	}
	return state.records, nil
}

// deployStep executes one labeled deployment: "var" is the
// create-variable EPL whose deployment-scoped public variable maps to
// env.RegisterVariable (Go has no deployment-scoped variable boundary);
// "s0" builds and deploys the case's select statement and attaches the
// trace listener (env.addListener("s0")).
func (s *patternGuardWhile590CaseState) deployStep(ctx context.Context, step compat.Step) error {
	var plan esper.Plan
	var err error
	switch {
	case s.spec.name == "simple" && step.Statement == "s0":
		if step.Epl != patternGuardWhile590SimpleEPL {
			return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q", patternGuardWhile590ID, step.Statement, step.Epl)
		}
		base := esper.From[patternGuardWhile590Bean](s.env, patternGuardWhile590BeanType)
		// `(every a=SupportBean) while (a.theString like 'E%')` — the
		// pending every materializes inside the while-guard child.
		pattern := esper.PatternFrom(base, "a", esper.Literal(true)).Every().
			WhileGuard(esper.LikeOf(esper.TagField[string]("a", "theString"), esper.Literal("E%")))
		plan, err = s.env.Build(pattern.Select(
			esper.Alias("c0", esper.TagField[string]("a", "theString")),
		).Query(esper.StatementName("s0")))
	case s.spec.name == "pattern-variable" && step.Statement == "var":
		if step.Epl != patternGuardWhile590VariableCreateEPL {
			return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q", patternGuardWhile590ID, step.Statement, step.Epl)
		}
		// `@name('var') @public create variable boolean myVariable =
		// true` — the deployment-scoped variable has no Go boundary; the
		// env-level registration is the observable equivalent.
		return s.env.RegisterVariable("myVariable", true)
	case s.spec.name == "pattern-variable" && step.Statement == "s0":
		if step.Epl != patternGuardWhile590VariableSelectEPL {
			return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q", patternGuardWhile590ID, step.Statement, step.Epl)
		}
		base := esper.From[patternGuardWhile590Bean](s.env, patternGuardWhile590BeanType)
		pattern := esper.PatternFrom(base, "a",
			esper.LikeOf(esper.Field[patternGuardWhile590Bean, string]("theString"), esper.Literal("A%"))).Every().
			Then(esper.PatternFrom(base, "b",
				esper.LikeOf(esper.Field[patternGuardWhile590Bean, string]("theString"), esper.Literal("B%"))).Every().
				WhileGuard(esper.VariableRef[bool]("myVariable")))
		plan, err = s.env.Build(pattern.Select(
			esper.Alias("a", esper.PatternEvent("a")),
			esper.Alias("b", esper.PatternEvent("b")),
		).Query(esper.StatementName("s0")))
	default:
		return fmt.Errorf("%s: unknown deploy %q in case %q", patternGuardWhile590ID, step.Statement, s.spec.name)
	}
	if err != nil {
		return fmt.Errorf("%s: build %q: %w", patternGuardWhile590ID, step.Statement, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", patternGuardWhile590ID, step.Statement, err)
	}
	s.deployments = append(s.deployments, deployment)
	statement, ok := deployment.Statement(step.Statement)
	if !ok {
		return fmt.Errorf("%s: statement %q was not deployed", patternGuardWhile590ID, step.Statement)
	}
	return s.attach(step.Statement, statement)
}

// attach registers the trace listener on one statement: records carry
// the case, the listener operation, the statement name, the per-
// statement sequence and the batch time — the Java TraceWriter shape.
func (s *patternGuardWhile590CaseState) attach(name string, statement *esper.Statement) error {
	s.statements[name] = statement
	_, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		s.sequence[name]++
		s.records = append(s.records, compat.TraceRecord{
			Case:      s.spec.name,
			Operation: "listener",
			Statement: name,
			Sequence:  s.sequence[name],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
		return nil
	})
	return err
}

// patternGuardWhile590LegSpec pins one pattern-op leg's fluent build
// plus its select-* fragment columns (a tag name per projected
// PatternEvent bean).
type patternGuardWhile590LegSpec struct {
	build func(patternGuardWhile590Streams) esper.PatternStream
	tags  []string
}

// patternGuardWhile590Streams bundles only the streams the five legs
// actually read — A and B plus the shared literal/neq helpers; the
// C..G event types are registered on the environment for the harness
// send set but carry no leg references.

type patternGuardWhile590Streams struct {
	a esper.Stream[patternGuardWhile590BeanA]
	b esper.Stream[patternGuardWhile590BeanB]

	always esper.Expression[bool]
	neqB   func(id string) esper.Expression[bool]
}

func newPatternGuardWhile590Streams(env *esper.Environment) patternGuardWhile590Streams {
	s := patternGuardWhile590Streams{
		a:      esper.From[patternGuardWhile590BeanA](env, patternGuardWhile590BeanAType),
		b:      esper.From[patternGuardWhile590BeanB](env, patternGuardWhile590BeanBType),
		always: esper.Literal[bool](true),
	}
	s.neqB = func(id string) esper.Expression[bool] {
		return esper.NotEqual[string](esper.TagField[string]("b", "id"), esper.Literal(id))
	}
	return s
}

// patternGuardWhile590LegSpecs returns the five leg builders in
// case-list order; each builder closes over the shared streams bundle
// so all legs deploy on the same input streams. `(every b) while
// (...)` maps to Every().WhileGuard — the pending every materializes
// inside the guarded child; S3 is the SODA leg and builds identically
// to S2 (the model path produces the same WhileGuard node).
func patternGuardWhile590LegSpecs() []patternGuardWhile590LegSpec {
	b := func(s patternGuardWhile590Streams) esper.PatternStream {
		return esper.PatternFrom(s.b, "b", s.always)
	}
	return []patternGuardWhile590LegSpec{
		// S0 `a=A -> (every b=B) while(b.id != 'B2')` — the guard quits
		// the followed-by instance at the first B2 match.
		{func(s patternGuardWhile590Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).
				Then(b(s).Every().WhileGuard(s.neqB("B2")))
		}, []string{"a", "b"}},
		// S1 `a=A -> (every b=B) while(b.id != 'B3')` — fires B1,B2 then
		// quits at B3.
		{func(s patternGuardWhile590Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).
				Then(b(s).Every().WhileGuard(s.neqB("B3")))
		}, []string{"a", "b"}},
		// S2 `(every b=B) while(b.id != 'B3')`.
		{func(s patternGuardWhile590Streams) esper.PatternStream {
			return b(s).Every().WhileGuard(s.neqB("B3"))
		}, []string{"b"}},
		// S3 SODA model leg `(every b=B) while (b.id!="B3")` — same
		// WhileGuard node as S2.
		{func(s patternGuardWhile590Streams) esper.PatternStream {
			return b(s).Every().WhileGuard(s.neqB("B3"))
		}, []string{"b"}},
		// S4 `(every b=B) while(b.id != 'B1')` — the first B1 match
		// falsifies the guard immediately: zero fires.
		{func(s patternGuardWhile590Streams) esper.PatternStream {
			return b(s).Every().WhileGuard(s.neqB("B1"))
		}, []string{"b"}},
	}
}

// deployAll expands the ONE deploy-all step into one DeployPlans call
// carrying the five legs in case order — statement S<i> deploys leg i —
// then attaches one listener per statement. select * expands to the
// per-tag fragment columns: each single tag as the bean fragment
// (PatternEvent), matching the Java wildcard fragments.
func (s *patternGuardWhile590CaseState) deployAll(ctx context.Context, step compat.Step) error {
	if s.deployedAll || step.Statement != "all" || s.spec.name != "pattern-op" {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternGuardWhile590ID, s.spec.name)
	}
	legs := patternGuardWhile590LegSpecs()
	streams := newPatternGuardWhile590Streams(s.env)
	if len(legs) != len(patternGuardWhile590LegAtoms) {
		return fmt.Errorf("%s: %d leg builders, want %d", patternGuardWhile590ID,
			len(legs), len(patternGuardWhile590LegAtoms))
	}
	plans := make([]esper.Plan, 0, len(legs))
	for leg, spec := range legs {
		selections := make([]esper.Selection, 0, len(spec.tags))
		for _, tag := range spec.tags {
			selections = append(selections, esper.Alias(tag, esper.PatternEvent(tag)))
		}
		plan, err := s.env.Build(spec.build(streams).Select(selections...).Query(
			esper.StatementName(patternGuardWhile590LegName(leg))))
		if err != nil {
			return fmt.Errorf("%s: build leg S%d %q: %w", patternGuardWhile590ID, leg,
				patternGuardWhile590LegAtoms[leg], err)
		}
		plans = append(plans, plan)
	}
	deployment, err := s.engine.DeployPlans(ctx, plans)
	if err != nil {
		return fmt.Errorf("%s: deploy-all: %w", patternGuardWhile590ID, err)
	}
	statements := deployment.Statements()
	if len(statements) != len(legs) {
		return fmt.Errorf("%s: deploy-all produced %d statements, want %d",
			patternGuardWhile590ID, len(statements), len(legs))
	}
	s.deployments = append(s.deployments, deployment)
	s.deployedAll = true
	for leg, statement := range statements {
		name := patternGuardWhile590LegName(leg)
		if statement.Name() != name {
			return fmt.Errorf("%s: statement %d is %q, want %q",
				patternGuardWhile590ID, leg, statement.Name(), name)
		}
		if err := s.attach(name, statement); err != nil {
			return err
		}
	}
	return nil
}

// buildError replays one expected-invalid probe: the non-boolean
// literal guard is unrepresentable in the typed API (Expression[bool]
// is a Go compile-time constraint) so the probe records the pinned
// Java message without claiming a Go rejection boundary, while the
// unresolvable-property guard builds the nearest expressible form —
// Field[bool]("abc") — and requires env.Build to reject it.
func (s *patternGuardWhile590CaseState) buildError(step compat.Step) error {
	pinnedEPL := map[string]string{
		"non-boolean-literal-guard":   patternGuardWhile590InvalidLiteralEPL,
		"unresolvable-property-guard": patternGuardWhile590InvalidPropertyEPL,
	}
	pinned, ok := pinnedEPL[step.Statement]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q",
			patternGuardWhile590ID, step.Statement, step.Epl)
	}
	switch step.Statement {
	case "non-boolean-literal-guard":
		// `every SupportBean while ('abc')` — a non-boolean literal is a
		// Go compile-time Expression[bool] type error and therefore not
		// expressible; record the pinned Java message verbatim.
	case "unresolvable-property-guard":
		// `every SupportBean while (abc)` — the guard expression's 'abc'
		// property resolves against the SupportBean event type and the
		// build must reject it (Property named 'abc' is not valid in any
		// stream).
		base := esper.From[patternGuardWhile590Bean](s.env, patternGuardWhile590BeanType)
		_, err := s.env.Build(esper.PatternFrom(base, "a", esper.Literal(true)).Every().
			WhileGuard(esper.Field[patternGuardWhile590Bean, bool]("abc")).
			Select(esper.Alias("c0", esper.TagField[string]("a", "theString"))).
			Query(esper.StatementName("s0")))
		var espErr *esper.Error
		if err == nil || !errors.As(err, &espErr) ||
			espErr.Code != esper.ErrorInvalidRule || !strings.Contains(err.Error(), "abc") {
			return fmt.Errorf("%s: build-error probe %q drift: got %v",
				patternGuardWhile590ID, step.Statement, err)
		}
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", patternGuardWhile590ID, step.Statement)
	}
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.spec.name,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// patternGuardWhile590DecodePayload converts a scenario send payload
// into the typed host object: SupportBean carries theString +
// intPrimitive; the mixed-set SupportBean_? types carry the single
// pinned id property.
func patternGuardWhile590DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	if step.EventType == patternGuardWhile590BeanType {
		if err := requirePatternGuardWhile590Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var event patternGuardWhile590Bean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean payload: %w", err)
		}
		return event, nil
	}
	if err := requirePatternGuardWhile590Fields(fields, "id"); err != nil {
		return nil, err
	}
	var id string
	if err := json.Unmarshal(fields["id"], &id); err != nil {
		return nil, fmt.Errorf("decode send id: %w", err)
	}
	return patternGuardWhile590TypedEvent(step.EventType, id)
}

// patternGuardWhile590TypedEvent rebuilds one mixed-set bean for the
// fused send steps and the post-undeploy resend silence check.
func patternGuardWhile590TypedEvent(eventType, id string) (any, error) {
	switch eventType {
	case patternGuardWhile590BeanAType:
		return patternGuardWhile590BeanA{ID: id}, nil
	case patternGuardWhile590BeanBType:
		return patternGuardWhile590BeanB{ID: id}, nil
	case patternGuardWhile590BeanCType:
		return patternGuardWhile590BeanC{ID: id}, nil
	case patternGuardWhile590BeanDType:
		return patternGuardWhile590BeanD{ID: id}, nil
	case patternGuardWhile590BeanEType:
		return patternGuardWhile590BeanE{ID: id}, nil
	case patternGuardWhile590BeanFType:
		return patternGuardWhile590BeanF{ID: id}, nil
	case patternGuardWhile590BeanGType:
		return patternGuardWhile590BeanG{ID: id}, nil
	}
	return nil, fmt.Errorf("unsupported %s event type %q", patternGuardWhile590ID, eventType)
}

// sortPatternGuardWhile590Records is the -diff normalizer for both
// traces: the harness compares each (statement, trigger-event) bucket
// as a multiset and Java's dispatch order inside one send — both the
// cross-statement order and the row order inside a multi-row listener
// batch — is unspecified, so each record's rows are serialized-sorted,
// the records are sorted by (case, time, statement) with the rows as
// the within-bucket tiebreak, and per-(case,statement) sequence numbers
// are re-assigned in bucket order — a pure multiset canonicalization
// that keeps every record.
func sortPatternGuardWhile590Records(trace compat.Trace) compat.Trace {
	rowKey := func(row compat.ResultRecord) string {
		raw, _ := json.Marshal(row)
		return string(raw)
	}
	sortRows := func(rows []compat.ResultRecord) {
		sort.SliceStable(rows, func(left, right int) bool {
			return rowKey(rows[left]) < rowKey(rows[right])
		})
	}
	key := func(record compat.TraceRecord) string {
		newRows, _ := json.Marshal(record.New)
		oldRows, _ := json.Marshal(record.Old)
		return string(newRows) + "|" + string(oldRows)
	}
	for index := range trace.Records {
		sortRows(trace.Records[index].New)
		sortRows(trace.Records[index].Old)
	}
	sort.SliceStable(trace.Records, func(left, right int) bool {
		a, b := trace.Records[left], trace.Records[right]
		if a.Case != b.Case {
			return a.Case < b.Case
		}
		if a.Time != b.Time {
			return a.Time < b.Time
		}
		if a.Statement != b.Statement {
			return a.Statement < b.Statement
		}
		return key(a) < key(b)
	})
	sequences := make(map[string]uint64)
	for index := range trace.Records {
		record := &trace.Records[index]
		bucket := record.Case + "\x00" + record.Statement
		sequences[bucket]++
		record.Sequence = sequences[bucket]
	}
	return trace
}
