package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for PatternGuardTimerWithinOrMax ord 0: ONE Java
// execution (the class implements RegressionExecution directly) whose
// PatternTestHarness runs seventeen timer:withinmax guard legs over
// the shared EventCollectionFactory.getEventSetOne(0, 1000) mixed
// event set. The harness deploys all seventeen statements before
// replaying the clocked set; the scenario encodes that phase as ONE
// deploy step (statement "all", the leg texts pinned in case.epls)
// which the runner expands into ONE DeployPlans call carrying 17
// plans in case order — statement names S0..S16 stand in for the
// harness's name--<atom> labels (StringEscapeUtils.escapeJava is an
// identity on these atoms), S3 being the harness's SODA-model leg
// whose pinned text is the model's toEPL output, and the
// @Audit('pattern')/@Audit('pattern-instances') annotations carry no
// observable listener payload. Every send step performs the harness's
// advanceTime(currentTime) BEFORE sendEventBean — the `at` instant
// advances the clock first so a withinmax deadline expiring inside an
// advance attributes to the upcoming event's bucket exactly like the
// harness's checkResults-after-sendEventBean ordering, and an expired
// guarded instance under `every` re-arms with a fresh timer at the
// advance instant (this is what lets S7 deliver B3 and lets S12..S16
// deliver the D1/D3 fires whose guard deadlines coincide exactly with
// the send instant). Per trigger event the harness compares each
// leg's expected EventDescriptors against the listener's last
// delivery as a MULTISET (compareLists), so per-statement bucketing
// and multiset semantics — not global cross-statement or within-batch
// ordering — carry the contract. Legs S0/S2/S4/S11 are pinned silence
// legs: the deadline expiring at the exact send instant kills the
// guarded filter before the event arrives (S0's 2 sec, S2's 1999
// msec) and the count cap 0 suppresses every completion of S4's
// `(every b)` child and S11's per-spawn guarded b. The
// COMPILE_TO_MODEL, COMPILE_TO_EPL and consume+suppress replay
// styles, all env.milestone savepoints and the silent post-undeploy
// resend are harness machinery outside the replayed surface. Within
// one send Esper dispatches pattern completions to statement
// listeners in an unspecified internal order while the Go engine
// dispatches in deployment order; the harness contract compares per
// (statement, event) as a multiset, so the trace carries Go's natural
// order and the -diff path sorts both traces into (case, time,
// statement) buckets before comparing.
//
// Covered execution (variant direct — the class implements
// RegressionExecution itself, flags [], static-manifest id
// java-0c023cc183e0d8221a0b):
//   - ord 0 PatternGuardTimerWithinOrMax java-runtime-78d9e9fcf78678c48ff9
//     (case "w-harness"), PatternGuardTimerWithinOrMax.java lines 22-129.
//     The deployed statement text is
//     `@name("S<i>") select * from pattern [<atom>]`; select * expands
//     to the per-tag fragment columns — each single tag via
//     PatternEvent (no leg uses a match-until collected tag).
const patternGuardWithinMax589ID = "pattern-guard-withinmax-589"

const patternGuardWithinMax589Description = "PatternGuardTimerWithinOrMax ord 0 — the single-execution 17-leg timer:withinmax W-harness over EventCollectionFactory.getEventSetOne(0,1000), replayed as ONE case: ONE deploy-all step (statement \"all\") expands to the harness's seventeen per-leg deployments of `@name(\"S<i>\") select * from pattern [<atom>]` (S0..S16 stand in for the name--<atom> labels, S3 carrying the SODA model's toEPL text), then twelve advance-before-send steps each advance the external clock to the event's pinned instant (+1000ms per send) BEFORE sendEventBean — a withinmax deadline expiring inside an advance attributes to the upcoming event's bucket (an expired guarded instance under every re-arms with a fresh timer at the advance instant, which is what lets S7 and S12..S16 deliver the D/D3 fires at the exact 4000ms expiry sends) — and every send's fires are grouped per leg+trigger as multiset rows. Expected fires: S0 silent (2 sec deadline expires at the B1 send before B1 is delivered), S1 B1{b:B1} (2001 msec bound covers B1@2000), S2 silent (1999 msec expires inside the B1 advance), S3 B3{b:B3} (10.001d SODA model leg, day bound never binds), S4 silent (count cap 0 suppresses every completion), S5 B1{b:B1}, S6 B1{b:B1}/B2{b:B2} (guard-over-every caps completions at 1/2; the 4.001 timer never binds), S7 B1{b:B1}/B2{b:B2}/B3{b:B3} (every spawns each guarded b instance, timer expiry re-arms a fresh instance inside the expiring advance), S8/S9/S10 B1{b:B1}/B2{b:B2}/B3{b:B3} (every (b where withinmax(2001 msec,N)) gives each spawn its own count cap), S11 silent (cap 0), S12 D1{b:B2,d:D1}/D3{b:B3,d:D3} (a single guarded d per b spawn; B1's instance expires at the exact D1 send), S13 D1{b:B1,d:D1}+D1{b:B2,d:D1}+D2{b:B1,d:D2}+D2{b:B2,d:D2}+D3{b:B1,d:D3}+D3{b:B2,d:D3}+D3{b:B3,d:D3} (guarded single d under every: cap 1 ends each instance after one d), S14 D1{b:B1,d:D1}+D1{b:B2,d:D1}+D2{b:B1,d:D2}+D2{b:B2,d:D2}+D3{b:B1,d:D3}+D3{b:B2,d:D3}+D3{b:B3,d:D3} (cap 3 on the parenthesized every-d never binds), S15 D1{b:B1,d:D1}+D1{b:B2,d:D1}+D2{b:B1,d:D2}+D2{b:B2,d:D2}+D3{b:B3,d:D3} (cap 2 drops the B1/B2 D3 fires), S16 D1{b:B1,d:D1}+D1{b:B2,d:D1}+D3{b:B3,d:D3} (cap 1 keeps only the first d per every-d instance). The COMPILE_TO_MODEL/COMPILE_TO_EPL/consume+suppress harness styles, env.milestone savepoints and the post-undeploy resend silence check are unrepresented harness machinery."

const patternGuardWithinMax589JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternGuardWithinMax589JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternGuardTimerWithinOrMax.java"

// Byte-exact pattern atoms (PatternGuardTimerWithinOrMax.java case-list
// order, lines 27-126 verbatim): `where timer:withinmax(...)`, the
// `2 sec`/`2001 msec`/`1999 msec`/`10.001d`/`4.001`/`2.001`/`4000 msec`/
// `1 day` duration spellings, parentheses and `every` placement are
// pinned exactly as written — including the SODA leg's `10.001d` suffix
// and the guard-outside-every vs guard-inside-every distinction between
// legs 4-6 and 7-11.
const (
	patternGuardWithinMax589BeanAType = "SupportBean_A"
	patternGuardWithinMax589BeanBType = "SupportBean_B"
	patternGuardWithinMax589BeanCType = "SupportBean_C"
	patternGuardWithinMax589BeanDType = "SupportBean_D"
	patternGuardWithinMax589BeanEType = "SupportBean_E"
	patternGuardWithinMax589BeanFType = "SupportBean_F"
	patternGuardWithinMax589BeanGType = "SupportBean_G"
)

// patternGuardWithinMax589StatementText wraps a pinned atom in the
// deployed statement text: @name("S<i>") plus select * from pattern
// [<atom>] — the S0..S16 names replace the harness's name--<atom>
// labels while keeping the observable listener surface identical.
func patternGuardWithinMax589StatementText(leg int, atom string) string {
	return fmt.Sprintf("@name(\"S%d\") select * from pattern [%s]", leg, atom)
}

// patternGuardWithinMax589LegName is the statement name leg i deploys
// under and the label the listener records carry.
func patternGuardWithinMax589LegName(leg int) string {
	return fmt.Sprintf("S%d", leg)
}

// Per-case Java identity: the single w-harness case owns the ord 0
// runtimeId; legs index into the atom table below.
var (
	patternGuardWithinMax589JavaRuntimeIDs = []string{
		"java-runtime-78d9e9fcf78678c48ff9",
	}
	patternGuardWithinMax589JavaSources = []string{
		patternGuardWithinMax589JavaSource,
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
	patternGuardWithinMax589JavaExecutions = []string{
		"PatternGuardTimerWithinOrMax",
	}
	patternGuardWithinMax589JavaStaticIDs = []string{
		"java-0c023cc183e0d8221a0b",
	}
	patternGuardWithinMax589JavaFlags = []string{}
)

// patternGuardWithinMax589BeanA..G mirror SupportBean_A..G: each
// carries the single id property select * projects as a fragment.
type patternGuardWithinMax589BeanA struct {
	ID string `json:"id" esper:"id"`
}
type patternGuardWithinMax589BeanB struct {
	ID string `json:"id" esper:"id"`
}
type patternGuardWithinMax589BeanC struct {
	ID string `json:"id" esper:"id"`
}
type patternGuardWithinMax589BeanD struct {
	ID string `json:"id" esper:"id"`
}
type patternGuardWithinMax589BeanE struct {
	ID string `json:"id" esper:"id"`
}
type patternGuardWithinMax589BeanF struct {
	ID string `json:"id" esper:"id"`
}
type patternGuardWithinMax589BeanG struct {
	ID string `json:"id" esper:"id"`
}

// patternGuardWithinMax589LegAtoms are the 17 EventExpressionCase
// atoms in case-list order (statement S<i> deploys leg i).
var patternGuardWithinMax589LegAtoms = []string{
	"b=SupportBean_B(id='B1') where timer:withinmax(2 sec,100)",                            // S0
	"b=SupportBean_B(id='B1') where timer:withinmax(2001 msec,1)",                          // S1
	"b=SupportBean_B(id='B1') where timer:withinmax(1999 msec,10)",                         // S2
	"b=SupportBean_B(id='B3') where timer:withinmax(10.001d,1)",                            // S3 (SODA model leg)
	"(every b=SupportBean_B) where timer:withinmax(4.001, 0)",                              // S4
	"(every b=SupportBean_B) where timer:withinmax(4.001, 1)",                              // S5
	"(every b=SupportBean_B) where timer:withinmax(4.001, 2)",                              // S6
	"every b=SupportBean_B where timer:withinmax(2.001, 4)",                                // S7
	"every (b=SupportBean_B where timer:withinmax(2001 msec, 2))",                          // S8
	"every (b=SupportBean_B where timer:withinmax(2001 msec, 3))",                          // S9
	"every (b=SupportBean_B where timer:withinmax(2001 msec, 1))",                          // S10
	"every (b=SupportBean_B where timer:withinmax(2001 msec, 0))",                          // S11
	"every b=SupportBean_B -> d=SupportBean_D where timer:withinmax(4000 msec, 1)",         // S12
	"every b=SupportBean_B() -> every d=SupportBean_D where timer:withinmax(4000 msec, 1)", // S13
	"every b=SupportBean_B() -> (every d=SupportBean_D) where timer:withinmax(1 day, 3)",   // S14
	"every b=SupportBean_B() -> (every d=SupportBean_D) where timer:withinmax(1 day, 2)",   // S15
	"every b=SupportBean_B() -> (every d=SupportBean_D) where timer:withinmax(1 day, 1)",   // S16
}

// patternGuardWithinMax589CaseSpec pins the single case: Java
// execution identity (ordinal plus runtimeIndex), the observation text
// and the seventeen byte-exact deployed statement texts pinned through
// case.epls.
type patternGuardWithinMax589CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epls         []string
}

var patternGuardWithinMax589CaseSpecs = []patternGuardWithinMax589CaseSpec{
	{
		name:         "w-harness",
		ordinal:      0,
		runtimeIndex: 0,
		observation: "listener; external clock +1000ms per send; all 17 legs deploy in ONE step " +
			"before any send (harness deploy-all), then twelve advance-before-send " +
			"replays of A1,B1,C1,B2,A2,D1,E1,F1,D2,B3,G1,D3 (advanceTime precedes " +
			"sendEventBean so deadline expiries attribute to the upcoming bucket and " +
			"every re-arms expired guarded instances there): S0 silent (2 sec expires " +
			"at B1's send), S1 B1{b:B1}, S2 silent (1999 msec), S3 B3{b:B3} (SODA " +
			"10.001d model), S4 silent (cap 0 over every), S5 B1{b:B1}, S6 B1,B2 (cap " +
			"1/2 over every), S7 B1,B2,B3 (per-spawn guard instances), S8/S9/S10 B1,B2,B3 " +
			"(per-instance caps 2/3/1 under every), S11 silent (cap 0), S12 (b:B2,d:D1) " +
			"and (b:B3,d:D3) only (single-d guard expires at exact deadline sends), S13 " +
			"seven rows B1/B2 x D1-D3 fan-out (every over guarded d), S14 seven rows " +
			"(cap 3 on parenthesized every-d), S15 five rows (cap 2 drops B1/B2 D3), " +
			"S16 three rows (cap 1 keeps first d); per leg+trigger the fires compare as " +
			"a multiset like compareLists",
		epls: func() []string {
			epls := make([]string, 0, len(patternGuardWithinMax589LegAtoms))
			for leg, atom := range patternGuardWithinMax589LegAtoms {
				epls = append(epls, patternGuardWithinMax589StatementText(leg, atom))
			}
			return epls
		}(),
	},
}

// patternGuardWithinMax589StepPin pins one scenario step's shape: the
// deploy-all step carries only the "all" statement label (the leg texts
// pin through case.epls), send steps carry the fused
// advance-before-send instant plus the event payload and undeploy-all
// closes the case.
type patternGuardWithinMax589StepPin struct {
	op        string
	statement string
	at        string
	eventType string
	payload   map[string]any
}

func patternGuardWithinMax589DeployAllPin() patternGuardWithinMax589StepPin {
	return patternGuardWithinMax589StepPin{op: "deploy", statement: "all"}
}

func patternGuardWithinMax589SendPin(at, eventType string, payload map[string]any) patternGuardWithinMax589StepPin {
	return patternGuardWithinMax589StepPin{op: "send", at: at, eventType: eventType, payload: payload}
}

func patternGuardWithinMax589IDSendPin(at int, eventType, id string) patternGuardWithinMax589StepPin {
	return patternGuardWithinMax589SendPin(patternGuardWithinMax589EventAt(at), eventType, map[string]any{"id": id})
}

func patternGuardWithinMax589UndeployAllPin() patternGuardWithinMax589StepPin {
	return patternGuardWithinMax589StepPin{op: "undeploy-all"}
}

// patternGuardWithinMax589EventAt renders one event-time instant: the
// harness's sendEventCollection.getTime(eventId) advance — A1=1000ms ..
// D3=12000ms on the external clock.
func patternGuardWithinMax589EventAt(offsetMS int) string {
	return time.Unix(0, 0).UTC().Add(time.Duration(offsetMS) * time.Millisecond).Format(time.RFC3339Nano)
}

var patternGuardWithinMax589EventSet = []struct {
	at        int
	eventType string
	id        string
}{
	{1000, patternGuardWithinMax589BeanAType, "A1"},
	{2000, patternGuardWithinMax589BeanBType, "B1"},
	{3000, patternGuardWithinMax589BeanCType, "C1"},
	{4000, patternGuardWithinMax589BeanBType, "B2"},
	{5000, patternGuardWithinMax589BeanAType, "A2"},
	{6000, patternGuardWithinMax589BeanDType, "D1"},
	{7000, patternGuardWithinMax589BeanEType, "E1"},
	{8000, patternGuardWithinMax589BeanFType, "F1"},
	{9000, patternGuardWithinMax589BeanDType, "D2"},
	{10000, patternGuardWithinMax589BeanBType, "B3"},
	{11000, patternGuardWithinMax589BeanGType, "G1"},
	{12000, patternGuardWithinMax589BeanDType, "D3"},
}

// patternGuardWithinMax589CaseSteps pins the complete step sequence in
// harness order: ONE deploy-all step expands to the seventeen leg
// deployments (the USE_EPL style compiles all statements before the
// event loop), then each event's fused advance-before-send step, then
// undeploy-all whose post-undeploy resend proves silence.
// env.milestone savepoints, the three other replay styles and the
// ON_START advance-time carry no scenario op.
var patternGuardWithinMax589CaseSteps = func() map[string][]patternGuardWithinMax589StepPin {
	steps := make(map[string][]patternGuardWithinMax589StepPin)
	for _, spec := range patternGuardWithinMax589CaseSpecs {
		pins := []patternGuardWithinMax589StepPin{
			patternGuardWithinMax589DeployAllPin(),
		}
		for _, event := range patternGuardWithinMax589EventSet {
			pins = append(pins,
				patternGuardWithinMax589IDSendPin(event.at, event.eventType, event.id))
		}
		pins = append(pins, patternGuardWithinMax589UndeployAllPin())
		steps[spec.name] = pins
	}
	return steps
}()

func loadPatternGuardWithinMax589Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternGuardWithinMax589ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternGuardWithinMax589ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternGuardWithinMax589ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternGuardWithinMax589ID, err)
	}
	if err := requirePatternGuardWithinMax589Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternGuardWithinMax589ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternGuardWithinMax589ID ||
		metadata.Description != patternGuardWithinMax589Description ||
		metadata.JavaCommit != patternGuardWithinMax589JavaCommit ||
		metadata.JavaSource != patternGuardWithinMax589JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternGuardWithinMax589ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternGuardWithinMax589JavaSources},
		{metadata.JavaRuntimes, patternGuardWithinMax589JavaRuntimeIDs},
		{metadata.JavaNames, patternGuardWithinMax589JavaExecutions},
		{metadata.JavaStaticIDs, patternGuardWithinMax589JavaStaticIDs},
		{metadata.JavaFlags, patternGuardWithinMax589JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternGuardWithinMax589ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternGuardWithinMax589CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternGuardWithinMax589ID, len(patternGuardWithinMax589CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternGuardWithinMax589Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternGuardWithinMax589CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternGuardWithinMax589JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternGuardWithinMax589JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || !reflect.DeepEqual(definition.EPLs, spec.epls) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternGuardWithinMax589ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternGuardWithinMax589ID)
	}
	if err := validatePatternGuardWithinMax589RawSteps(rawSteps); err != nil {
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

// validatePatternGuardWithinMax589RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned sequence — the ONE deploy-all step (statement
// "all"), the twelve fused advance-before-send steps with pinned
// instants/payloads and the closing undeploy-all.
func validatePatternGuardWithinMax589RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternGuardWithinMax589CaseSpecs))
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
			At        string          `json:"at"`
			EventType string          `json:"eventType"`
			Payload   json.RawMessage `json:"payload"`
		}
		if operation == "case" {
			if err := requirePatternGuardWithinMax589Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternGuardWithinMax589CaseSteps[marker.Case]; !ok {
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
		pins := patternGuardWithinMax589CaseSteps[currentCase]
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
			if err := requirePatternGuardWithinMax589Fields(object, "op", "case", "statement"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternGuardWithinMax589Fields(object, "op", "case", "at", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.At != pin.at || step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternGuardWithinMax589Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternGuardWithinMax589CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternGuardWithinMax589CaseSpecs))
	}
	for index, spec := range patternGuardWithinMax589CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternGuardWithinMax589CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternGuardWithinMax589CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternGuardWithinMax589Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternGuardWithinMax589CaseState carries the replay state: the
// single DeployPlans deployment of the 17 legs, the deployed per-leg
// statements keyed by their S0..S16 labels, per-statement listener
// sequence counters and the delivery records.
type patternGuardWithinMax589CaseState struct {
	spec        patternGuardWithinMax589CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	deployment  *esper.Deployment
	statements  map[string]*esper.Statement
	sequence    map[string]uint64
	records     []compat.TraceRecord
	deployedAll bool
}

// runPatternGuardWithinMax589Scenario replays the execution against a
// fresh engine like the Java oracle's fresh per-execution runtime —
// the seventeen legs share ONE engine and ONE DeployPlans deployment
// because the harness deploys all statements before replaying the
// event set.
func runPatternGuardWithinMax589Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternGuardWithinMax589Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternGuardWithinMax589CaseSpecs {
		records, err := runPatternGuardWithinMax589Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternGuardWithinMax589ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternGuardWithinMax589Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternGuardWithinMax589ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternGuardWithinMax589ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternGuardWithinMax589RawSteps(rawSteps)
}

func runPatternGuardWithinMax589Case(ctx context.Context, scenario compat.Scenario, spec patternGuardWithinMax589CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternGuardWithinMax589BeanA](env, patternGuardWithinMax589BeanAType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternGuardWithinMax589BeanB](env, patternGuardWithinMax589BeanBType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternGuardWithinMax589BeanC](env, patternGuardWithinMax589BeanCType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternGuardWithinMax589BeanD](env, patternGuardWithinMax589BeanDType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternGuardWithinMax589BeanE](env, patternGuardWithinMax589BeanEType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternGuardWithinMax589BeanF](env, patternGuardWithinMax589BeanFType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternGuardWithinMax589BeanG](env, patternGuardWithinMax589BeanGType); err != nil {
		return nil, err
	}
	state := &patternGuardWithinMax589CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternGuardWithinMax589JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			if err := state.deployAll(ctx, step); err != nil {
				return nil, err
			}
		case "send":
			payload, err := patternGuardWithinMax589DecodePayload(step)
			if err != nil {
				return nil, err
			}
			// The fused step performs the harness's advanceTime
			// BEFORE sendEventBean: a withinmax deadline expiring
			// inside the advance attributes to the upcoming event's
			// bucket and an expired guarded instance under `every`
			// re-arms at the advance instant (S7's respawn for B3,
			// S12..S16's deadline-exact D fires) exactly like the
			// harness's checkResults-after-sendEventBean ordering.
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse send at %q: %w", patternGuardWithinMax589ID, step.At, err)
			}
			if err := state.engine.AdvanceTime(ctx, at); err != nil {
				return nil, err
			}
			// The harness compares per (statement, event) as a
			// multiset — Java's cross-statement delivery order inside
			// one send is an unspecified internal dispatch order, so
			// the runner emits Go's natural deployment-order delivery
			// and the -diff path normalizes both traces into
			// (case, time, statement) buckets before comparing.
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "undeploy-all":
			if state.deployment != nil {
				if err := state.deployment.Undeploy(ctx); err != nil {
					return nil, err
				}
			}
			state.deployment = nil
			state.statements = make(map[string]*esper.Statement)
			state.sequence = make(map[string]uint64)
			state.deployedAll = false
			// The harness resends the whole event set after
			// undeployAll to prove every listener is gone; the sends
			// must produce zero records.
			for _, event := range patternGuardWithinMax589EventSet {
				bean, err := patternGuardWithinMax589TypedEvent(event.eventType, event.id)
				if err != nil {
					return nil, err
				}
				if err := state.engine.Send(ctx, event.eventType, bean); err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternGuardWithinMax589ID, step.Op)
		}
	}
	return state.records, nil
}

// patternGuardWithinMax589Tag pins one projected select-* column: a
// single bean fragment (PatternEvent). No leg uses a collected tag —
// every tags here bind exactly one bean per fire (the `every` and
// guarded forms deliver one event per completion), matching Esper's
// select * over single-match pattern atoms.
type patternGuardWithinMax589Tag struct {
	name string
}

// patternGuardWithinMax589LegSpec pins one leg's fluent build plus its
// select-* fragment columns.
type patternGuardWithinMax589LegSpec struct {
	build func(patternGuardWithinMax589Streams) esper.PatternStream
	tags  []patternGuardWithinMax589Tag
}

// patternGuardWithinMax589Streams bundles the typed streams of the
// mixed event set plus the leg predicate helpers shared by all 17 legs
// of the DeployPlans deployment.
type patternGuardWithinMax589Streams struct {
	a esper.Stream[patternGuardWithinMax589BeanA]
	b esper.Stream[patternGuardWithinMax589BeanB]
	c esper.Stream[patternGuardWithinMax589BeanC]
	d esper.Stream[patternGuardWithinMax589BeanD]
	e esper.Stream[patternGuardWithinMax589BeanE]
	f esper.Stream[patternGuardWithinMax589BeanF]
	g esper.Stream[patternGuardWithinMax589BeanG]

	always esper.Expression[bool]
	idB    func(id string) esper.Expression[bool]
}

func newPatternGuardWithinMax589Streams(env *esper.Environment) patternGuardWithinMax589Streams {
	s := patternGuardWithinMax589Streams{
		a:      esper.From[patternGuardWithinMax589BeanA](env, patternGuardWithinMax589BeanAType),
		b:      esper.From[patternGuardWithinMax589BeanB](env, patternGuardWithinMax589BeanBType),
		c:      esper.From[patternGuardWithinMax589BeanC](env, patternGuardWithinMax589BeanCType),
		d:      esper.From[patternGuardWithinMax589BeanD](env, patternGuardWithinMax589BeanDType),
		e:      esper.From[patternGuardWithinMax589BeanE](env, patternGuardWithinMax589BeanEType),
		f:      esper.From[patternGuardWithinMax589BeanF](env, patternGuardWithinMax589BeanFType),
		g:      esper.From[patternGuardWithinMax589BeanG](env, patternGuardWithinMax589BeanGType),
		always: esper.Literal[bool](true),
	}
	s.idB = func(id string) esper.Expression[bool] {
		return esper.Equal[string](esper.Field[patternGuardWithinMax589BeanB, string]("id"), esper.Literal(id))
	}
	return s
}

// patternGuardWithinMax589LegSpecs returns the 17 leg builders in
// case-list order; each builder closes over the shared streams bundle
// passed by deployAll so all 17 plans deploy on the same input streams.
// Leg i of this slice is statement S<i>. The builds pin the EPL
// nesting distinction that carries the semantics: `(every b) where
// withinmax` is a guard over the every (one deadline and one count cap
// over all completions — Every().WithinOrMax), while `every b where
// withinmax` and `every (b where withinmax)` put the guard inside the
// every so each spawned instance owns a fresh timer and count
// (WithinOrMax().Every()); S12 guards the single d of each followed-by
// instance, S13 guards a single d under its own every, and S14..S16
// guard the parenthesized every-d so the cap applies to that every's
// completions.
func patternGuardWithinMax589LegSpecs() []patternGuardWithinMax589LegSpec {
	tag := func(name string) patternGuardWithinMax589Tag {
		return patternGuardWithinMax589Tag{name: name}
	}
	b := func(s patternGuardWithinMax589Streams) esper.PatternStream {
		return esper.PatternFrom(s.b, "b", s.always)
	}
	d := func(s patternGuardWithinMax589Streams) esper.PatternStream {
		return esper.PatternFrom(s.d, "d", s.always)
	}
	day := 24 * time.Hour
	return []patternGuardWithinMax589LegSpec{
		// S0 `b=B(id='B1') where withinmax(2 sec,100)` — deadline 2000
		// expires inside the B1 advance; silent.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.idB("B1")).WithinOrMax(2*time.Second, 100)
		}, []patternGuardWithinMax589Tag{tag("b")}},
		// S1 `b=B(id='B1') where withinmax(2001 msec,1)` — fires at B1.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.idB("B1")).WithinOrMax(2001*time.Millisecond, 1)
		}, []patternGuardWithinMax589Tag{tag("b")}},
		// S2 `b=B(id='B1') where withinmax(1999 msec,10)` — silent.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.idB("B1")).WithinOrMax(1999*time.Millisecond, 10)
		}, []patternGuardWithinMax589Tag{tag("b")}},
		// S3 SODA model leg `b=B(id='B3') where withinmax(10.001d,1)` —
		// the day-scaled bound (864086400 ms) never binds; fires at B3.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.idB("B3")).WithinOrMax(10*day+86400*time.Millisecond, 1)
		}, []patternGuardWithinMax589Tag{tag("b")}},
		// S4 `(every b) where withinmax(4.001,0)` — cap 0 suppresses
		// every completion; silent.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).Every().WithinOrMax(4001*time.Millisecond, 0)
		}, []patternGuardWithinMax589Tag{tag("b")}},
		// S5 `(every b) where withinmax(4.001,1)` — one completion.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).Every().WithinOrMax(4001*time.Millisecond, 1)
		}, []patternGuardWithinMax589Tag{tag("b")}},
		// S6 `(every b) where withinmax(4.001,2)` — two completions.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).Every().WithinOrMax(4001*time.Millisecond, 2)
		}, []patternGuardWithinMax589Tag{tag("b")}},
		// S7 `every b where withinmax(2.001,4)` — the guard binds
		// inside the every, so each spawned instance owns a fresh
		// timer; expiry inside an advance re-arms before the send.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).WithinOrMax(2001*time.Millisecond, 4).Every()
		}, []patternGuardWithinMax589Tag{tag("b")}},
		// S8 `every (b where withinmax(2001 msec,2))` — per-instance cap.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).WithinOrMax(2001*time.Millisecond, 2).Every()
		}, []patternGuardWithinMax589Tag{tag("b")}},
		// S9 `every (b where withinmax(2001 msec,3))`.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).WithinOrMax(2001*time.Millisecond, 3).Every()
		}, []patternGuardWithinMax589Tag{tag("b")}},
		// S10 `every (b where withinmax(2001 msec,1))`.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).WithinOrMax(2001*time.Millisecond, 1).Every()
		}, []patternGuardWithinMax589Tag{tag("b")}},
		// S11 `every (b where withinmax(2001 msec,0))` — cap 0
		// suppresses each spawn; silent.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).WithinOrMax(2001*time.Millisecond, 0).Every()
		}, []patternGuardWithinMax589Tag{tag("b")}},
		// S12 `every b -> d where withinmax(4000 msec,1)` — each b
		// spawn's single guarded d lives 4s: B1's instance expires at
		// the exact D1 send; only (B2,D1) and (B3,D3) fire.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).Every().Then(d(s).WithinOrMax(4000*time.Millisecond, 1))
		}, []patternGuardWithinMax589Tag{tag("b"), tag("d")}},
		// S13 `every b -> every d where withinmax(4000 msec,1)` — the
		// guard sits inside the every-d, so each per-b every instance
		// respawns a fresh guarded d after each fire (cap 1 ends the
		// instance attempt); yields the 2+2+3 fan-out.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).Every().Then(d(s).WithinOrMax(4000*time.Millisecond, 1).Every())
		}, []patternGuardWithinMax589Tag{tag("b"), tag("d")}},
		// S14 `every b -> (every d) where withinmax(1 day,3)` — the
		// guard wraps the whole every-d per b spawn; cap 3 and the
		// day window never bind; yields the same seven rows.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).Every().Then(d(s).Every().WithinOrMax(day, 3))
		}, []patternGuardWithinMax589Tag{tag("b"), tag("d")}},
		// S15 `... (every d) where withinmax(1 day,2)` — cap 2 drops
		// the B1/B2 D3 fires.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).Every().Then(d(s).Every().WithinOrMax(day, 2))
		}, []patternGuardWithinMax589Tag{tag("b"), tag("d")}},
		// S16 `... (every d) where withinmax(1 day,1)` — cap 1 keeps
		// only each every-d instance's first fire.
		{func(s patternGuardWithinMax589Streams) esper.PatternStream {
			return b(s).Every().Then(d(s).Every().WithinOrMax(day, 1))
		}, []patternGuardWithinMax589Tag{tag("b"), tag("d")}},
	}
}

// deployAll expands the ONE deploy-all step into one DeployPlans call
// carrying all 17 plans in case order — statement S<i> deploys leg i —
// then attaches one listener per statement. select * expands to the
// per-tag fragment columns: each single tag as the bean fragment
// (PatternEvent), matching the Java wildcard fragments.
func (s *patternGuardWithinMax589CaseState) deployAll(ctx context.Context, step compat.Step) error {
	if s.deployedAll || step.Statement != "all" {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternGuardWithinMax589ID, s.spec.name)
	}
	legs := patternGuardWithinMax589LegSpecs()
	streams := newPatternGuardWithinMax589Streams(s.env)
	if len(legs) != len(patternGuardWithinMax589LegAtoms) {
		return fmt.Errorf("%s: %d leg builders, want %d", patternGuardWithinMax589ID,
			len(legs), len(patternGuardWithinMax589LegAtoms))
	}
	plans := make([]esper.Plan, 0, len(legs))
	for leg, spec := range legs {
		selections := make([]esper.Selection, 0, len(spec.tags))
		for _, tag := range spec.tags {
			selections = append(selections, esper.Alias(tag.name, esper.PatternEvent(tag.name)))
		}
		plan, err := s.env.Build(spec.build(streams).Select(selections...).Query(
			esper.StatementName(patternGuardWithinMax589LegName(leg))))
		if err != nil {
			return fmt.Errorf("%s: build leg S%d %q: %w", patternGuardWithinMax589ID, leg,
				patternGuardWithinMax589LegAtoms[leg], err)
		}
		plans = append(plans, plan)
	}
	deployment, err := s.engine.DeployPlans(ctx, plans)
	if err != nil {
		return fmt.Errorf("%s: deploy-all: %w", patternGuardWithinMax589ID, err)
	}
	statements := deployment.Statements()
	if len(statements) != len(legs) {
		return fmt.Errorf("%s: deploy-all produced %d statements, want %d",
			patternGuardWithinMax589ID, len(statements), len(legs))
	}
	s.deployment = deployment
	s.deployedAll = true
	for leg, statement := range statements {
		name := patternGuardWithinMax589LegName(leg)
		if statement.Name() != name {
			return fmt.Errorf("%s: statement %d is %q, want %q",
				patternGuardWithinMax589ID, leg, statement.Name(), name)
		}
		s.statements[name] = statement
		if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
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
		}); err != nil {
			return err
		}
	}
	return nil
}

// patternGuardWithinMax589DecodePayload converts a scenario send
// payload into the typed host object: every SupportBean_? type carries
// the single pinned id property.
func patternGuardWithinMax589DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	if err := requirePatternGuardWithinMax589Fields(fields, "id"); err != nil {
		return nil, err
	}
	var id string
	if err := json.Unmarshal(fields["id"], &id); err != nil {
		return nil, fmt.Errorf("decode send id: %w", err)
	}
	return patternGuardWithinMax589TypedEvent(step.EventType, id)
}

// patternGuardWithinMax589TypedEvent rebuilds one mixed-set bean for
// the fused send steps and the post-undeploy resend silence check.
func patternGuardWithinMax589TypedEvent(eventType, id string) (any, error) {
	switch eventType {
	case patternGuardWithinMax589BeanAType:
		return patternGuardWithinMax589BeanA{ID: id}, nil
	case patternGuardWithinMax589BeanBType:
		return patternGuardWithinMax589BeanB{ID: id}, nil
	case patternGuardWithinMax589BeanCType:
		return patternGuardWithinMax589BeanC{ID: id}, nil
	case patternGuardWithinMax589BeanDType:
		return patternGuardWithinMax589BeanD{ID: id}, nil
	case patternGuardWithinMax589BeanEType:
		return patternGuardWithinMax589BeanE{ID: id}, nil
	case patternGuardWithinMax589BeanFType:
		return patternGuardWithinMax589BeanF{ID: id}, nil
	case patternGuardWithinMax589BeanGType:
		return patternGuardWithinMax589BeanG{ID: id}, nil
	}
	return nil, fmt.Errorf("unsupported %s event type %q", patternGuardWithinMax589ID, eventType)
}

// sortPatternGuardWithinMax589Records is the -diff normalizer for both
// traces: the harness compares each (statement, trigger-event) bucket
// as a multiset and Java's dispatch order inside one send — both the
// cross-statement order and the row order inside a multi-row listener
// batch — is unspecified, so each record's rows are serialized-sorted,
// the records are sorted by (case, time, statement) with the rows as
// the within-bucket tiebreak, and per-(case,statement) sequence numbers
// are re-assigned in bucket order — a pure multiset canonicalization
// that keeps every record.
func sortPatternGuardWithinMax589Records(trace compat.Trace) compat.Trace {
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
