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

// Parity coverage for PatternObserverTimerInterval ord 0 PatternOp: ONE
// Java execution whose PatternTestHarness runs thirty-one
// timer:interval legs over the shared
// EventCollectionFactory.getEventSetOne(0, 1000) mixed event set. The
// harness deploys all thirty-one statements before replaying the
// clocked set; the scenario encodes that phase as ONE deploy step
// (statement "all", the leg texts pinned in case.epls) which the
// runner expands into ONE DeployPlans call carrying 31 plans in case
// order — statement names S0..S30 stand in for the harness's
// name--<atom> labels (StringEscapeUtils.escapeJava is an identity on
// these atoms), S1 being the harness's SODA-model leg whose pinned
// text is the model's toEPL output `timer:interval(1.999d)` (the `d`
// suffix is the double literal marker, so the interval is 1.999
// seconds), and the @Audit('pattern')/@Audit('pattern-instances')
// annotations carry no observable listener payload. Every send step
// performs the harness's advanceTime(currentTime) BEFORE
// sendEventBean — the `at` instant advances the clock first so timer
// expirations inside an advance attribute to the upcoming event's
// bucket exactly like the harness's checkResults-after-sendEventBean
// ordering, and an every-interval observer whose expiry lands inside
// an advance re-arms its next callback from the advance instant (this
// is what lets S7's 3.001-second every reach the F1/D3 buckets and
// S8's 5000 msec every reach B3). Per trigger event the harness
// compares each leg's expected EventDescriptors against the listener's
// last delivery as a MULTISET (compareLists), so per-statement
// bucketing and multiset semantics — not global cross-statement
// ordering — carry the contract. Legs S27/S29/S30 are pinned silence
// legs: the 9999999 msec timer never fires, and both
// `where timer:within` guards quit before (S29, 2.000 < 3.000) or at
// (S30, the spaced `timer:within (3.000)` spelling, deadline equal to
// the interval) the timer's own completion. The COMPILE_TO_MODEL,
// COMPILE_TO_EPL and consume+suppress replay styles, all
// env.milestone savepoints and the silent post-undeploy resend are
// harness machinery outside the replayed surface. Within one send
// Esper dispatches pattern completions to statement listeners in an
// unspecified internal order while the Go engine dispatches in
// deployment order; the harness contract compares per (statement,
// event) as a multiset, so the trace carries Go's natural order and
// the -diff path sorts both traces into (case, time, statement)
// buckets before comparing.
//
// Covered execution (variant collection:executions(), flags [],
// static-manifest id java-1422565b568236b2bfea):
//   - ord 0 PatternOp java-runtime-9b41fb5951301c0de979 (case
//     "w-harness"), PatternObserverTimerInterval.java lines 46-192.
//     The deployed statement text is
//     `@name("S<i>") select * from pattern [<atom>]`; select *
//     expands to the per-tag fragment columns — each single tag via
//     PatternEvent, an unbound tag as the null marker (the S20/S22
//     or-timer wins leave `b` vacant) and the untagged legs no column
//     at all.
const patternTimerIntervalWHarness592ID = "pattern-timerinterval-wharness-592"

const patternTimerIntervalWHarness592Description = "PatternObserverTimerInterval ord 0 PatternOp — the shared 31-leg timer:interval W-harness over EventCollectionFactory.getEventSetOne(0,1000), replayed as ONE case: ONE deploy-all step (statement \"all\") expands to the harness's thirty-one per-leg deployments of `@name(\"S<i>\") select * from pattern [<atom>]` (S0..S30 stand in for the name--<atom> labels, S1 carrying the SODA model's toEPL text `timer:interval(1.999d)`), then twelve advance-before-send steps each advance the external clock to the event's pinned instant (+1000ms per send) BEFORE sendEventBean — timer expirations inside an advance attribute to the upcoming event's bucket (an every-interval observer whose expiry lands inside an advance re-arms from the advance instant, which is what lets S7 reach F1/D3 and S8 reach B3) — and every send's fires are grouped per leg+trigger as multiset rows. Expected fires: S0/S1/S2 B1{} (1999 msec, the SODA 1.999d double-literal leg and the boundary-inclusive 2 sec), S3/S4/S5 C1{} (2001/2999/3000 msec expiries inside the C1 advance), S6 B2{} (3.001 seconds), S7 B2{}/F1{}/D3{} (every 3.001 sec re-arms at each advance instant: 3001/7001/11001), S8 A2{}/B3{} (every 5000 msec: 5000/10000), S9 B2{b:B2} (3.999 second expiry arms b inside the B2 advance), S10 B2{b:B2} (4 sec expiry at the exact B2 advance instant), S11 B3{b:B3} (4.001 sec expiry arms b inside the B3 advance), S12 B1{b:B1} (interval 0 pre-arms b at deploy), S13 C1{b:B1} (B1 arms a 0.001 timer that expires inside the C1 advance), S14 B1{b:B1} (B1's zero-interval timer fires inside the same send), S15 C1{b:B1} (B1's 1 sec timer expires inside the C1 advance), S16 B2{b:B1} (B1's 1.001 timer expires inside the B2 advance), S17 D2{b:B1,d:D2} (B1's 6.000 timer arms d inside the E1 advance), S18 D1{b:B1,d:D1} (serial every respawn: B2's 2.001 timer expires inside the E1 advance so D1 pairs only with B1, and B3's timer expires inside the D3 advance), S19 D1{b:B1,d:D1}+D3{b:B3,d:D3} (the 2.000 timers expire at the exact D1/D3 send instants), S20 B1{b unbound} (the 1.001 or-timer expires inside the B1 advance before B1 arrives), S21 B1{b:B1} (B1 wins the or before the 2.001 timer), S22 D2{b unbound} (the B3-filtered b never matches before the 8.500 or-timer expires inside the D2 advance), S23 F1{} (the 7.500 timer wins inside the F1 advance), S24 G1{g:G1} (the 999999 msec timer never binds), S25 B2{b:B1} (B1 plus the 4000 msec timer expiring at the exact B2 advance), S26 A2{b:B1} (B1 plus the 4001 msec timer inside the A2 advance), S27 silent (the 9999999 msec timer never fires), S28 B2{b:B2} (the 1 msec timer expires inside the A1 advance; the B2-filtered b completes the and at B2), S29 silent (timer:within(2.000) quits the 3.000 timer inside the B1 advance), S30 silent (timer:within (3.000) — spaced spelling — quits at the exact 3000 deadline inside the C1 advance). The COMPILE_TO_MODEL/COMPILE_TO_EPL/consume+suppress harness styles, env.milestone savepoints and the post-undeploy resend silence check are unrepresented harness machinery."

const patternTimerIntervalWHarness592JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternTimerIntervalWHarness592JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternObserverTimerInterval.java"

// Byte-exact pattern atoms (PatternObserverTimerInterval.java PatternOp
// case-list order, lines 46-192 verbatim): the `msec`/`sec`/`second`/
// `seconds`/`milliseconds`/`1 msec` unit spellings, the bare decimal
// seconds (`2.001`, `3.001`, `1.001`, `6.000`, `2.000`, `8.500`,
// `7.500`, `3.000`), the `()` empty-filter markers, `every` placement,
// `or`/`and`/`->` punctuation and the S30 `timer:within (3.000)`
// spacing are pinned exactly as written — including the SODA leg's
// `1.999d` double-literal suffix.
const (
	patternTimerIntervalWHarness592BeanAType = "SupportBean_A"
	patternTimerIntervalWHarness592BeanBType = "SupportBean_B"
	patternTimerIntervalWHarness592BeanCType = "SupportBean_C"
	patternTimerIntervalWHarness592BeanDType = "SupportBean_D"
	patternTimerIntervalWHarness592BeanEType = "SupportBean_E"
	patternTimerIntervalWHarness592BeanFType = "SupportBean_F"
	patternTimerIntervalWHarness592BeanGType = "SupportBean_G"
)

// patternTimerIntervalWHarness592StatementText wraps a pinned atom in
// the deployed statement text: @name("S<i>") plus select * from
// pattern [<atom>] — the S0..S30 names replace the harness's
// name--<atom> labels while keeping the observable listener surface
// identical.
func patternTimerIntervalWHarness592StatementText(leg int, atom string) string {
	return fmt.Sprintf("@name(\"S%d\") select * from pattern [%s]", leg, atom)
}

// patternTimerIntervalWHarness592LegName is the statement name leg i
// deploys under and the label the listener records carry.
func patternTimerIntervalWHarness592LegName(leg int) string {
	return fmt.Sprintf("S%d", leg)
}

// Per-case Java identity: the single w-harness case owns the ord 0
// runtimeId; legs index into the atom table below.
var (
	patternTimerIntervalWHarness592JavaRuntimeIDs = []string{
		"java-runtime-9b41fb5951301c0de979",
	}
	patternTimerIntervalWHarness592JavaSources = []string{
		patternTimerIntervalWHarness592JavaSource,
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
	patternTimerIntervalWHarness592JavaExecutions = []string{
		"PatternOp",
	}
	patternTimerIntervalWHarness592JavaStaticIDs = []string{
		"java-1422565b568236b2bfea",
	}
	patternTimerIntervalWHarness592JavaFlags = []string{}
)

// patternTimerIntervalWHarness592BeanA..G mirror SupportBean_A..G:
// each carries the single id property select * projects as a
// fragment.
type patternTimerIntervalWHarness592BeanA struct {
	ID string `json:"id" esper:"id"`
}
type patternTimerIntervalWHarness592BeanB struct {
	ID string `json:"id" esper:"id"`
}
type patternTimerIntervalWHarness592BeanC struct {
	ID string `json:"id" esper:"id"`
}
type patternTimerIntervalWHarness592BeanD struct {
	ID string `json:"id" esper:"id"`
}
type patternTimerIntervalWHarness592BeanE struct {
	ID string `json:"id" esper:"id"`
}
type patternTimerIntervalWHarness592BeanF struct {
	ID string `json:"id" esper:"id"`
}
type patternTimerIntervalWHarness592BeanG struct {
	ID string `json:"id" esper:"id"`
}

// patternTimerIntervalWHarness592LegAtoms are the 31
// EventExpressionCase atoms in case-list order (statement S<i>
// deploys leg i); S1's atom is the SODA model leg's pinned toEPL
// text.
var patternTimerIntervalWHarness592LegAtoms = []string{
	"timer:interval(1999 msec)",                                               // S0
	"timer:interval(1.999d)",                                                  // S1 (SODA model leg)
	"timer:interval(2 sec)",                                                   // S2
	"timer:interval(2.001)",                                                   // S3
	"timer:interval(2999 milliseconds)",                                       // S4
	"timer:interval(3 seconds)",                                               // S5
	"timer:interval(3.001 seconds)",                                           // S6
	"every timer:interval(3.001 sec)",                                         // S7
	"every timer:interval(5000 msec)",                                         // S8
	"timer:interval(3.999 second) -> b=SupportBean_B",                         // S9
	"timer:interval(4 sec) -> b=SupportBean_B",                                // S10
	"timer:interval(4.001 sec) -> b=SupportBean_B",                            // S11
	"timer:interval(0) -> b=SupportBean_B",                                    // S12
	"b=SupportBean_B -> timer:interval(0.001)",                                // S13
	"b=SupportBean_B -> timer:interval(0)",                                    // S14
	"b=SupportBean_B -> timer:interval(1 sec)",                                // S15
	"b=SupportBean_B -> timer:interval(1.001)",                                // S16
	"b=SupportBean_B() -> timer:interval(6.000) -> d=SupportBean_D",           // S17
	"every (b=SupportBean_B() -> timer:interval(2.001) -> d=SupportBean_D())", // S18
	"every (b=SupportBean_B() -> timer:interval(2.000) -> d=SupportBean_D())", // S19
	"b=SupportBean_B() or timer:interval(1.001)",                              // S20
	"b=SupportBean_B() or timer:interval(2.001)",                              // S21
	"b=SupportBean_B(id='B3') or timer:interval(8.500)",                       // S22
	"timer:interval(8.500) or timer:interval(7.500)",                          // S23
	"timer:interval(999999 msec) or g=SupportBean_G",                          // S24
	"b=SupportBean_B() and timer:interval(4000 msec)",                         // S25
	"b=SupportBean_B() and timer:interval(4001 msec)",                         // S26
	"timer:interval(9999999 msec) and b=SupportBean_B",                        // S27
	"timer:interval(1 msec) and b=SupportBean_B(id='B2')",                     // S28
	"timer:interval(3.000) where timer:within(2.000)",                         // S29
	"timer:interval(3.000) where timer:within (3.000)",                        // S30
}

// patternTimerIntervalWHarness592CaseSpec pins the single case: Java
// execution identity (ordinal plus runtimeIndex), the observation
// text and the thirty-one byte-exact deployed statement texts pinned
// through case.epls.
type patternTimerIntervalWHarness592CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epls         []string
}

var patternTimerIntervalWHarness592CaseSpecs = []patternTimerIntervalWHarness592CaseSpec{
	{
		name:         "w-harness",
		ordinal:      0,
		runtimeIndex: 0,
		observation: "listener; external clock +1000ms per send; all 31 legs deploy in ONE step " +
			"before any send (harness deploy-all), then twelve advance-before-send " +
			"replays of A1,B1,C1,B2,A2,D1,E1,F1,D2,B3,G1,D3 (advanceTime precedes " +
			"sendEventBean so timer expiries attribute to the upcoming bucket and " +
			"every-interval observers re-arm at the advance instant): S0/S1/S2 B1, " +
			"S3/S4/S5 C1, S6 B2, S7 B2/F1/D3 (every 3.001 sec re-arm 3001/7001/11001), " +
			"S8 A2/B3 (every 5000 msec), S9/S10 B2{b:B2}, S11 B3{b:B3}, S12 B1{b:B1} " +
			"(zero pre-armed), S13 C1{b:B1}, S14 B1{b:B1} (zero same-send), S15 " +
			"C1{b:B1}, S16 B2{b:B1}, S17 D2{b:B1,d:D2}, S18 D1{b:B1,d:D1} only " +
			"(serial every), S19 D1+D3, S20 B1 with b unbound (1.001 or-timer wins), " +
			"S21 B1{b:B1}, S22 D2 with b unbound (8.500 or-timer wins), S23 F1 " +
			"(7.500 wins), S24 G1{g:G1}, S25 B2{b:B1}, S26 A2{b:B1}, S27 silent, " +
			"S28 B2{b:B2}, S29/S30 silent (within guards quit at/inside the 3.000 " +
			"deadline); per leg+trigger the fires compare as a multiset like " +
			"compareLists",
		epls: func() []string {
			epls := make([]string, 0, len(patternTimerIntervalWHarness592LegAtoms))
			for leg, atom := range patternTimerIntervalWHarness592LegAtoms {
				epls = append(epls, patternTimerIntervalWHarness592StatementText(leg, atom))
			}
			return epls
		}(),
	},
}

// patternTimerIntervalWHarness592StepPin pins one scenario step's
// shape: the deploy-all step carries only the "all" statement label
// (the leg texts pin through case.epls), send steps carry the fused
// advance-before-send instant plus the event payload and
// undeploy-all closes the case.
type patternTimerIntervalWHarness592StepPin struct {
	op        string
	statement string
	at        string
	eventType string
	payload   map[string]any
}

func patternTimerIntervalWHarness592DeployAllPin() patternTimerIntervalWHarness592StepPin {
	return patternTimerIntervalWHarness592StepPin{op: "deploy", statement: "all"}
}

func patternTimerIntervalWHarness592SendPin(at, eventType string, payload map[string]any) patternTimerIntervalWHarness592StepPin {
	return patternTimerIntervalWHarness592StepPin{op: "send", at: at, eventType: eventType, payload: payload}
}

func patternTimerIntervalWHarness592IDSendPin(at int, eventType, id string) patternTimerIntervalWHarness592StepPin {
	return patternTimerIntervalWHarness592SendPin(patternTimerIntervalWHarness592EventAt(at), eventType, map[string]any{"id": id})
}

func patternTimerIntervalWHarness592UndeployAllPin() patternTimerIntervalWHarness592StepPin {
	return patternTimerIntervalWHarness592StepPin{op: "undeploy-all"}
}

// patternTimerIntervalWHarness592EventAt renders one event-time
// instant: the harness's sendEventCollection.getTime(eventId)
// advance — A1=1000ms .. D3=12000ms on the external clock.
func patternTimerIntervalWHarness592EventAt(offsetMS int) string {
	return time.Unix(0, 0).UTC().Add(time.Duration(offsetMS) * time.Millisecond).Format(time.RFC3339Nano)
}

var patternTimerIntervalWHarness592EventSet = []struct {
	at        int
	eventType string
	id        string
}{
	{1000, patternTimerIntervalWHarness592BeanAType, "A1"},
	{2000, patternTimerIntervalWHarness592BeanBType, "B1"},
	{3000, patternTimerIntervalWHarness592BeanCType, "C1"},
	{4000, patternTimerIntervalWHarness592BeanBType, "B2"},
	{5000, patternTimerIntervalWHarness592BeanAType, "A2"},
	{6000, patternTimerIntervalWHarness592BeanDType, "D1"},
	{7000, patternTimerIntervalWHarness592BeanEType, "E1"},
	{8000, patternTimerIntervalWHarness592BeanFType, "F1"},
	{9000, patternTimerIntervalWHarness592BeanDType, "D2"},
	{10000, patternTimerIntervalWHarness592BeanBType, "B3"},
	{11000, patternTimerIntervalWHarness592BeanGType, "G1"},
	{12000, patternTimerIntervalWHarness592BeanDType, "D3"},
}

// patternTimerIntervalWHarness592CaseSteps pins the complete step
// sequence in harness order: ONE deploy-all step expands to the
// thirty-one leg deployments (the USE_EPL style compiles all
// statements before the event loop), then each event's fused
// advance-before-send step, then undeploy-all whose post-undeploy
// resend proves silence. env.milestone savepoints, the three other
// replay styles and the ON_START advance-time carry no scenario op.
var patternTimerIntervalWHarness592CaseSteps = func() map[string][]patternTimerIntervalWHarness592StepPin {
	steps := make(map[string][]patternTimerIntervalWHarness592StepPin)
	for _, spec := range patternTimerIntervalWHarness592CaseSpecs {
		pins := []patternTimerIntervalWHarness592StepPin{
			patternTimerIntervalWHarness592DeployAllPin(),
		}
		for _, event := range patternTimerIntervalWHarness592EventSet {
			pins = append(pins,
				patternTimerIntervalWHarness592IDSendPin(event.at, event.eventType, event.id))
		}
		pins = append(pins, patternTimerIntervalWHarness592UndeployAllPin())
		steps[spec.name] = pins
	}
	return steps
}()

func loadPatternTimerIntervalWHarness592Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternTimerIntervalWHarness592ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternTimerIntervalWHarness592ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternTimerIntervalWHarness592ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternTimerIntervalWHarness592ID, err)
	}
	if err := requirePatternTimerIntervalWHarness592Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternTimerIntervalWHarness592ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternTimerIntervalWHarness592ID ||
		metadata.Description != patternTimerIntervalWHarness592Description ||
		metadata.JavaCommit != patternTimerIntervalWHarness592JavaCommit ||
		metadata.JavaSource != patternTimerIntervalWHarness592JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternTimerIntervalWHarness592ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternTimerIntervalWHarness592JavaSources},
		{metadata.JavaRuntimes, patternTimerIntervalWHarness592JavaRuntimeIDs},
		{metadata.JavaNames, patternTimerIntervalWHarness592JavaExecutions},
		{metadata.JavaStaticIDs, patternTimerIntervalWHarness592JavaStaticIDs},
		{metadata.JavaFlags, patternTimerIntervalWHarness592JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternTimerIntervalWHarness592ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternTimerIntervalWHarness592CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternTimerIntervalWHarness592ID, len(patternTimerIntervalWHarness592CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternTimerIntervalWHarness592Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternTimerIntervalWHarness592CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternTimerIntervalWHarness592JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternTimerIntervalWHarness592JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || !reflect.DeepEqual(definition.EPLs, spec.epls) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternTimerIntervalWHarness592ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternTimerIntervalWHarness592ID)
	}
	if err := validatePatternTimerIntervalWHarness592RawSteps(rawSteps); err != nil {
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

// validatePatternTimerIntervalWHarness592RawSteps pins the complete
// step sequence: op field whitelists per step kind plus positional
// comparison against the pinned sequence — the ONE deploy-all step
// (statement "all"), the twelve fused advance-before-send steps with
// pinned instants/payloads and the closing undeploy-all.
func validatePatternTimerIntervalWHarness592RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternTimerIntervalWHarness592CaseSpecs))
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
			if err := requirePatternTimerIntervalWHarness592Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternTimerIntervalWHarness592CaseSteps[marker.Case]; !ok {
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
		pins := patternTimerIntervalWHarness592CaseSteps[currentCase]
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
			if err := requirePatternTimerIntervalWHarness592Fields(object, "op", "case", "statement"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternTimerIntervalWHarness592Fields(object, "op", "case", "at", "eventType", "payload"); err != nil {
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
			if err := requirePatternTimerIntervalWHarness592Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternTimerIntervalWHarness592CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternTimerIntervalWHarness592CaseSpecs))
	}
	for index, spec := range patternTimerIntervalWHarness592CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternTimerIntervalWHarness592CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternTimerIntervalWHarness592CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternTimerIntervalWHarness592Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternTimerIntervalWHarness592CaseState carries the replay state:
// the single DeployPlans deployment of the 31 legs, the deployed
// per-leg statements keyed by their S0..S30 labels, per-statement
// listener sequence counters and the delivery records.
type patternTimerIntervalWHarness592CaseState struct {
	spec        patternTimerIntervalWHarness592CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	deployment  *esper.Deployment
	statements  map[string]*esper.Statement
	sequence    map[string]uint64
	records     []compat.TraceRecord
	deployedAll bool
}

// runPatternTimerIntervalWHarness592Scenario replays the execution
// against a fresh engine like the Java oracle's fresh per-execution
// runtime — the thirty-one legs share ONE engine and ONE DeployPlans
// deployment because the harness deploys all statements before
// replaying the event set.
func runPatternTimerIntervalWHarness592Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternTimerIntervalWHarness592Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternTimerIntervalWHarness592CaseSpecs {
		records, err := runPatternTimerIntervalWHarness592Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternTimerIntervalWHarness592ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternTimerIntervalWHarness592Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternTimerIntervalWHarness592ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternTimerIntervalWHarness592ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternTimerIntervalWHarness592RawSteps(rawSteps)
}

func runPatternTimerIntervalWHarness592Case(ctx context.Context, scenario compat.Scenario, spec patternTimerIntervalWHarness592CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanA](env, patternTimerIntervalWHarness592BeanAType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanB](env, patternTimerIntervalWHarness592BeanBType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanC](env, patternTimerIntervalWHarness592BeanCType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanD](env, patternTimerIntervalWHarness592BeanDType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanE](env, patternTimerIntervalWHarness592BeanEType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanF](env, patternTimerIntervalWHarness592BeanFType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanG](env, patternTimerIntervalWHarness592BeanGType); err != nil {
		return nil, err
	}
	state := &patternTimerIntervalWHarness592CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternTimerIntervalWHarness592JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			payload, err := patternTimerIntervalWHarness592DecodePayload(step)
			if err != nil {
				return nil, err
			}
			// The fused step performs the harness's advanceTime
			// BEFORE sendEventBean: timer expirations inside the
			// advance attribute to the upcoming event's bucket and
			// every-interval observers re-arm their next callback at
			// the advance instant (S7's 3001/7001/11001 fires, S8's
			// 5000/10000 fires) exactly like the harness's
			// checkResults-after-sendEventBean ordering.
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse send at %q: %w", patternTimerIntervalWHarness592ID, step.At, err)
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
			for _, event := range patternTimerIntervalWHarness592EventSet {
				bean, err := patternTimerIntervalWHarness592TypedEvent(event.eventType, event.id)
				if err != nil {
					return nil, err
				}
				if err := state.engine.Send(ctx, event.eventType, bean); err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternTimerIntervalWHarness592ID, step.Op)
		}
	}
	return state.records, nil
}

// patternTimerIntervalWHarness592Tag pins one projected select-*
// column: a single bean fragment (PatternEvent). No leg uses a
// collected tag — the `every` forms deliver one event per
// completion — matching Esper's select * over single-match pattern
// atoms; a tag left unbound by a losing or-branch projects the null
// marker.
type patternTimerIntervalWHarness592Tag struct {
	name string
}

// patternTimerIntervalWHarness592LegSpec pins one leg's fluent build
// plus its select-* fragment columns.
type patternTimerIntervalWHarness592LegSpec struct {
	build func(patternTimerIntervalWHarness592Streams) esper.PatternStream
	tags  []patternTimerIntervalWHarness592Tag
}

// patternTimerIntervalWHarness592Streams bundles the typed streams of
// the mixed event set plus the leg predicate helpers shared by all 31
// legs of the DeployPlans deployment.
type patternTimerIntervalWHarness592Streams struct {
	a esper.Stream[patternTimerIntervalWHarness592BeanA]
	b esper.Stream[patternTimerIntervalWHarness592BeanB]
	c esper.Stream[patternTimerIntervalWHarness592BeanC]
	d esper.Stream[patternTimerIntervalWHarness592BeanD]
	e esper.Stream[patternTimerIntervalWHarness592BeanE]
	f esper.Stream[patternTimerIntervalWHarness592BeanF]
	g esper.Stream[patternTimerIntervalWHarness592BeanG]

	always esper.Expression[bool]
	idB    func(id string) esper.Expression[bool]
}

func newPatternTimerIntervalWHarness592Streams(env *esper.Environment) patternTimerIntervalWHarness592Streams {
	s := patternTimerIntervalWHarness592Streams{
		a:      esper.From[patternTimerIntervalWHarness592BeanA](env, patternTimerIntervalWHarness592BeanAType),
		b:      esper.From[patternTimerIntervalWHarness592BeanB](env, patternTimerIntervalWHarness592BeanBType),
		c:      esper.From[patternTimerIntervalWHarness592BeanC](env, patternTimerIntervalWHarness592BeanCType),
		d:      esper.From[patternTimerIntervalWHarness592BeanD](env, patternTimerIntervalWHarness592BeanDType),
		e:      esper.From[patternTimerIntervalWHarness592BeanE](env, patternTimerIntervalWHarness592BeanEType),
		f:      esper.From[patternTimerIntervalWHarness592BeanF](env, patternTimerIntervalWHarness592BeanFType),
		g:      esper.From[patternTimerIntervalWHarness592BeanG](env, patternTimerIntervalWHarness592BeanGType),
		always: esper.Literal[bool](true),
	}
	s.idB = func(id string) esper.Expression[bool] {
		return esper.Equal[string](esper.Field[patternTimerIntervalWHarness592BeanB, string]("id"), esper.Literal(id))
	}
	return s
}

// patternTimerIntervalWHarness592LegSpecs returns the 31 leg builders
// in case-list order; each builder closes over the shared streams
// bundle passed by deployAll so all 31 plans deploy on the same
// input streams. Leg i of this slice is statement S<i>. The builds
// mirror internal/esper/pattern_test.go's
// TestPatternTimerObserverComposesWithEvents /
// TestPatternEventThenTimerObserverCompletesOnClock compositions:
// bare timer:interval roots become TimerInterval (the stream argument
// only supplies the environment — timer observers consume no source
// events), `every` wraps the whole atom, `->` chains become Then,
// inclusive `or`/`and` become Or/And, the `where timer:within`
// guards become Within on the timer root, and S1's SODA `1.999d`
// double-literal interval is the same 1999ms observer as S0.
func patternTimerIntervalWHarness592LegSpecs() []patternTimerIntervalWHarness592LegSpec {
	tag := func(name string) patternTimerIntervalWHarness592Tag {
		return patternTimerIntervalWHarness592Tag{name: name}
	}
	b := func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
		return esper.PatternFrom(s.b, "b", s.always)
	}
	d := func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
		return esper.PatternFrom(s.d, "d", s.always)
	}
	g := func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
		return esper.PatternFrom(s.g, "g", s.always)
	}
	timer := func(s patternTimerIntervalWHarness592Streams, interval time.Duration) esper.PatternStream {
		return esper.TimerInterval(s.b, interval)
	}
	return []patternTimerIntervalWHarness592LegSpec{
		// S0 `timer:interval(1999 msec)` — expires inside the B1
		// advance; untagged.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 1999*time.Millisecond)
		}, nil},
		// S1 SODA model leg `timer:interval(1.999d)` — the `d` suffix
		// marks a double literal so the interval is 1.999 seconds,
		// the same 1999ms observer as S0; fires at B1.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 1999*time.Millisecond)
		}, nil},
		// S2 `timer:interval(2 sec)` — boundary-inclusive at the B1
		// advance instant.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 2*time.Second)
		}, nil},
		// S3 `timer:interval(2.001)` — bare decimal seconds; expires
		// inside the C1 advance.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 2001*time.Millisecond)
		}, nil},
		// S4 `timer:interval(2999 milliseconds)` — C1 advance.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 2999*time.Millisecond)
		}, nil},
		// S5 `timer:interval(3 seconds)` — expires at the exact C1
		// advance instant.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 3*time.Second)
		}, nil},
		// S6 `timer:interval(3.001 seconds)` — B2 advance.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 3001*time.Millisecond)
		}, nil},
		// S7 `every timer:interval(3.001 sec)` — fires inside the
		// B2/F1/D3 advances (3001, then re-armed at each advance
		// instant: 4000+3001=7001 inside F1, 8000+3001=11001 inside
		// D3).
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 3001*time.Millisecond).Every()
		}, nil},
		// S8 `every timer:interval(5000 msec)` — fires inside the A2
		// advance (5000) and re-arms at 5000 to fire inside the B3
		// advance (10000).
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 5*time.Second).Every()
		}, nil},
		// S9 `timer:interval(3.999 second) -> b=B` — the timer
		// expires inside the B2 advance and arms b just in time for
		// the B2 send.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 3999*time.Millisecond).Then(b(s))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S10 `timer:interval(4 sec) -> b=B` — expiry at the exact
		// B2 advance instant still precedes the send.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 4*time.Second).Then(b(s))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S11 `timer:interval(4.001 sec) -> b=B` — expires inside
		// the A2 advance, so b binds B3 at the next B send.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 4001*time.Millisecond).Then(b(s))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S12 `timer:interval(0) -> b=B` — the zero timer expires at
		// deploy so b is armed before B1.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 0).Then(b(s))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S13 `b=B -> timer:interval(0.001)` — B1 arms a 1ms timer
		// that expires inside the C1 advance.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Then(timer(s, time.Millisecond))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S14 `b=B -> timer:interval(0)` — the zero timer fires
		// inside the same B1 send that armed it.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Then(timer(s, 0))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S15 `b=B -> timer:interval(1 sec)` — B1's timer expires
		// inside the C1 advance.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Then(timer(s, time.Second))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S16 `b=B -> timer:interval(1.001)` — B1's timer expires
		// inside the B2 advance.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Then(timer(s, 1001*time.Millisecond))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S17 `b=B() -> timer:interval(6.000) -> d=D` — B1's 6s timer
		// expires inside the E1 advance, so d binds D2.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Then(timer(s, 6*time.Second)).Then(d(s))
		}, []patternTimerIntervalWHarness592Tag{tag("b"), tag("d")}},
		// S18 `every (b=B() -> timer:interval(2.001) -> d=D())` —
		// the every respawns serially at each completion: only B1's
		// instance reaches d (B2's 2.001 timer expires inside the
		// E1 advance, too late for D1; B3's expires inside the D3
		// advance).
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Then(timer(s, 2001*time.Millisecond)).Then(d(s)).Every()
		}, []patternTimerIntervalWHarness592Tag{tag("b"), tag("d")}},
		// S19 `every (b=B() -> timer:interval(2.000) -> d=D())` —
		// B1's timer expires at the exact D1 advance instant and
		// B3's respawned instance does the same at D3.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Then(timer(s, 2*time.Second)).Then(d(s)).Every()
		}, []patternTimerIntervalWHarness592Tag{tag("b"), tag("d")}},
		// S20 `b=B() or timer:interval(1.001)` — the or-timer
		// expires inside the B1 advance before B1 arrives, so the
		// single fire leaves b unbound.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Or(timer(s, 1001*time.Millisecond))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S21 `b=B() or timer:interval(2.001)` — B1 wins the or
		// before the timer expires inside the C1 advance.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Or(timer(s, 2001*time.Millisecond))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S22 `b=B(id='B3') or timer:interval(8.500)` — the filtered
		// b never sees B3 before the timer expires inside the D2
		// advance, so the fire leaves b unbound.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.idB("B3")).Or(timer(s, 8500*time.Millisecond))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S23 `timer:interval(8.500) or timer:interval(7.500)` — the
		// earlier 7.500 timer expires inside the F1 advance.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 8500*time.Millisecond).Or(timer(s, 7500*time.Millisecond))
		}, nil},
		// S24 `timer:interval(999999 msec) or g=G` — the g atom
		// wins; the day-scale timer never binds.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 999999*time.Millisecond).Or(g(s))
		}, []patternTimerIntervalWHarness592Tag{tag("g")}},
		// S25 `b=B() and timer:interval(4000 msec)` — B1 is held and
		// the timer expires at the exact B2 advance instant.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).And(timer(s, 4*time.Second))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S26 `b=B() and timer:interval(4001 msec)` — the timer
		// expires inside the A2 advance.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).And(timer(s, 4001*time.Millisecond))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S27 `timer:interval(9999999 msec) and b=B` — the timer
		// never fires inside the replayed window; silent.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 9999999*time.Millisecond).And(b(s))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S28 `timer:interval(1 msec) and b=B(id='B2')` — the timer
		// expires inside the A1 advance and the B2-filtered b
		// completes the and at B2.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, time.Millisecond).And(esper.PatternFrom(s.b, "b", s.idB("B2")))
		}, []patternTimerIntervalWHarness592Tag{tag("b")}},
		// S29 `timer:interval(3.000) where timer:within(2.000)` —
		// the guard deadline quits the timer inside the B1 advance;
		// silent.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 3*time.Second).Within(2 * time.Second)
		}, nil},
		// S30 `timer:interval(3.000) where timer:within (3.000)` —
		// the exact-boundary deadline quits at the same 3000
		// instant the timer would complete inside the C1 advance;
		// silent.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return timer(s, 3*time.Second).Within(3 * time.Second)
		}, nil},
	}
}

// deployAll expands the ONE deploy-all step into one DeployPlans
// call carrying all 31 plans in case order — statement S<i> deploys
// leg i — then attaches one listener per statement. select *
// expands to the per-tag fragment columns: each single tag as the
// bean fragment (PatternEvent), matching the Java wildcard
// fragments; untagged legs carry no selection so their fires render
// an empty row.
func (s *patternTimerIntervalWHarness592CaseState) deployAll(ctx context.Context, step compat.Step) error {
	if s.deployedAll || step.Statement != "all" {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternTimerIntervalWHarness592ID, s.spec.name)
	}
	legs := patternTimerIntervalWHarness592LegSpecs()
	streams := newPatternTimerIntervalWHarness592Streams(s.env)
	if len(legs) != len(patternTimerIntervalWHarness592LegAtoms) {
		return fmt.Errorf("%s: %d leg builders, want %d", patternTimerIntervalWHarness592ID,
			len(legs), len(patternTimerIntervalWHarness592LegAtoms))
	}
	plans := make([]esper.Plan, 0, len(legs))
	for leg, spec := range legs {
		selections := make([]esper.Selection, 0, len(spec.tags))
		for _, tag := range spec.tags {
			selections = append(selections, esper.Alias(tag.name, esper.PatternEvent(tag.name)))
		}
		plan, err := s.env.Build(spec.build(streams).Select(selections...).Query(
			esper.StatementName(patternTimerIntervalWHarness592LegName(leg))))
		if err != nil {
			return fmt.Errorf("%s: build leg S%d %q: %w", patternTimerIntervalWHarness592ID, leg,
				patternTimerIntervalWHarness592LegAtoms[leg], err)
		}
		plans = append(plans, plan)
	}
	deployment, err := s.engine.DeployPlans(ctx, plans)
	if err != nil {
		return fmt.Errorf("%s: deploy-all: %w", patternTimerIntervalWHarness592ID, err)
	}
	statements := deployment.Statements()
	if len(statements) != len(legs) {
		return fmt.Errorf("%s: deploy-all produced %d statements, want %d",
			patternTimerIntervalWHarness592ID, len(statements), len(legs))
	}
	s.deployment = deployment
	s.deployedAll = true
	for leg, statement := range statements {
		name := patternTimerIntervalWHarness592LegName(leg)
		if statement.Name() != name {
			return fmt.Errorf("%s: statement %d is %q, want %q",
				patternTimerIntervalWHarness592ID, leg, statement.Name(), name)
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

// patternTimerIntervalWHarness592DecodePayload converts a scenario
// send payload into the typed host object: every SupportBean_? type
// carries the single pinned id property.
func patternTimerIntervalWHarness592DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	if err := requirePatternTimerIntervalWHarness592Fields(fields, "id"); err != nil {
		return nil, err
	}
	var id string
	if err := json.Unmarshal(fields["id"], &id); err != nil {
		return nil, fmt.Errorf("decode send id: %w", err)
	}
	return patternTimerIntervalWHarness592TypedEvent(step.EventType, id)
}

// patternTimerIntervalWHarness592TypedEvent rebuilds one mixed-set
// bean for the fused send steps and the post-undeploy resend silence
// check.
func patternTimerIntervalWHarness592TypedEvent(eventType, id string) (any, error) {
	switch eventType {
	case patternTimerIntervalWHarness592BeanAType:
		return patternTimerIntervalWHarness592BeanA{ID: id}, nil
	case patternTimerIntervalWHarness592BeanBType:
		return patternTimerIntervalWHarness592BeanB{ID: id}, nil
	case patternTimerIntervalWHarness592BeanCType:
		return patternTimerIntervalWHarness592BeanC{ID: id}, nil
	case patternTimerIntervalWHarness592BeanDType:
		return patternTimerIntervalWHarness592BeanD{ID: id}, nil
	case patternTimerIntervalWHarness592BeanEType:
		return patternTimerIntervalWHarness592BeanE{ID: id}, nil
	case patternTimerIntervalWHarness592BeanFType:
		return patternTimerIntervalWHarness592BeanF{ID: id}, nil
	case patternTimerIntervalWHarness592BeanGType:
		return patternTimerIntervalWHarness592BeanG{ID: id}, nil
	}
	return nil, fmt.Errorf("unsupported %s event type %q", patternTimerIntervalWHarness592ID, eventType)
}

// sortPatternTimerIntervalWHarness592Records is the -diff normalizer
// for both traces: the harness compares each (statement,
// trigger-event) bucket as a multiset and Java's dispatch order
// inside one send — both the cross-statement order and the row order
// inside a multi-row listener batch — is unspecified, so each
// record's rows are serialized-sorted, the records are sorted by
// (case, time, statement) with the rows as the within-bucket
// tiebreak, and per-(case,statement) sequence numbers are
// re-assigned in bucket order — a pure multiset canonicalization
// that keeps every record.
func sortPatternTimerIntervalWHarness592Records(trace compat.Trace) compat.Trace {
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
