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

// Parity coverage for PatternOperatorMatchUntil ord 1 PatternOp: ONE Java
// execution whose PatternTestHarness runs fifty-two match-until legs over
// the shared EventCollectionFactory.getEventSetOne(0, 1000) mixed event
// set. The harness deploys all fifty-two statements before replaying the
// clocked set; the scenario encodes that phase as ONE deploy step
// (statement "all", the leg texts pinned in case.epls) which the runner
// expands into ONE DeployPlans call carrying 52 plans in case order —
// statement names S0..S51 stand in for the harness's name--<atom> labels
// (StringEscapeUtils.escapeJava is an identity on these atoms) and the
// @Audit('pattern')/@Audit('pattern-instances') annotations carry no
// observable listener payload. Every send step performs the harness's
// advanceTime(currentTime) BEFORE sendEventBean — the `at` instant
// advances the clock first so timer legs whose expirations land inside
// an advance (S39 d until timer:interval(7 sec) at E1; S43 `a until
// every(timer:interval(6 sec) and not A)` at G1; S44 `A until
// every(timer:interval(7 sec) and not A)` at D3) attribute to the
// upcoming event's bucket exactly like the harness's checkResults-after
// -sendEventBean ordering. Per trigger event the harness compares each
// leg's expected EventDescriptors against the listener's last delivery
// as a MULTISET (compareLists), so per-statement bucketing and multiset
// semantics — not global cross-statement ordering — carry the contract.
// Leg S51 `SupportBean_B until not SupportBean_B` can fire on start;
// the harness ignores start fires by design (the start event carries
// no information) and the EPL statement surface does not deliver them
// to listeners, so S51 contributes zero records like the other silent
// legs S7/S12/S15/S16/S24/S26/S27/S28. The COMPILE_TO_MODEL,
// COMPILE_TO_EPL and consume+suppress replay styles, all env.milestone
// savepoints and the silent post-undeploy resend are harness machinery
// outside the replayed surface. Within one send Esper dispatches
// pattern completions to statement listeners in an unspecified
// internal order — empirically neither deployment order nor its
// reverse — while the Go engine dispatches in deployment order; the
// harness contract compares per (statement, event) as a multiset, so
// the trace carries Go's natural order and the -diff path sorts both
// traces into (case, time, statement) buckets before comparing.
//
// Covered execution (variant collection:executions(), flags [],
// static-manifest id java-d4cb4534ca508be87595):
//   - ord 1 PatternOp java-runtime-0383244f373c8a0ffc8b (case
//     "w-harness"), PatternOperatorMatchUntil.java lines 91-315. The
//     deployed statement text is
//     `@name("S<i>") select * from pattern [<atom>]`; select *
//     expands to the per-tag fragment columns — the repeated tag as
//     its collected bean array via TagEvents and each single tag via
//     PatternEvent; untagged atoms bind no column so S44's fire
//     carries an empty row.
const patternMatchUntilWHarness588ID = "pattern-matchuntil-wharness-588"

const patternMatchUntilWHarness588Description = "PatternOperatorMatchUntil ord 1 PatternOp — the shared 52-leg match-until W-harness over EventCollectionFactory.getEventSetOne(0,1000), replayed as ONE case: ONE deploy-all step (statement \"all\") expands to the harness's fifty-two per-leg deployments of `@name(\"S<i>\") select * from pattern [<atom>]` (S0..S51 stand in for the name--<atom> labels), then twelve advance-before-send steps each advance the external clock to the event's pinned instant (+1000ms per send) BEFORE sendEventBean — timer expirations inside an advance attribute to the upcoming event's bucket (S39 d until timer:interval(7 sec) -> E1, S43 a until every(timer:interval(6 sec) and not A) -> G1, S44 A until every(timer:interval(7 sec) and not A) -> D3) — and every send's fires are grouped per leg+trigger as multiset rows. Expected fires: S0 D1{a:A2}, S1 D1{a:A1,A2}, S2 A1{b:empty,a:A1}, S3 D3{b:B1,B2,B3}, S4 D3{a:A1,A2;b:B1,B2,B3;d:D3}, S5 D1{a:A1,A2;b:B1,B2;d:D1}, S6 A1{a:A1}, S7 silent, S8/S9 A2{a:A1,A2}, S10/S11 A1{a:A1}, S12 silent, S13 B3{b:B1,B2,B3}, S14 A2{a:A1,A2;b:B1,B2}, S15/S16 silent (until kills the sub-minimum repetition permanently), S17 B2{b:B1,B2} (tight [2:2] fires at the bound), S18/S19 G1{b:B1,B2,B3;g:G1}, S20 G1{b:B1,B2;g:G1}, S21 G1{b:B1;g:G1}, S22 A1{b:empty,a:A1}, S23 G1{b:B1,B2,B3;g:G1}, S24 silent (terminator below minimum), S25 A2{b:B1,B2;a:A2}, S26 silent, S27/S28 silent (until wins on a shared event), S29 A2{b:B1,B2;a:A2}, S30 G1{b:B1,B2,B3}, S31 G1{b:B1,B2}, S32 F1{b:B1,B2}, S33/S34 C1{b:B1}, S35 D3{c:C1;b:B2,B3;d:D3}, S36 B3{b:B1,B2,B3}, S37 D3{d:D1,D2,D3}, S38 D2{b:B1,B2;d:D1,D2}, S39 E1{d:D1}, S40 B1{b:B1}/B2{b:B2}/B3{b:B3;d:D1,D2}, S41/S42 B1{b:B1} (every binds tighter than until), S43 G1{a:A1,A2}, S44 D3{} (fire, no tags), S45 B1{a:A1;b:B1}, S46 A2{a:A1,A2}, S47 D1{a:A1,A2;d:D1} (ESPER-339 every precedence), S48 B2{a:A1;b:B1,B2}, S49 C1{a:A1;b:B1;c:C1}, S50 G1{a:A1,A2;b:B1,B2,B3;g:G1}, S51 silent (start-fire ignored by the harness). The COMPILE_TO_MODEL/COMPILE_TO_EPL/consume+suppress harness styles, env.milestone savepoints and the post-undeploy resend silence check are unrepresented harness machinery."

const patternMatchUntilWHarness588JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternMatchUntilWHarness588JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorMatchUntil.java"

// Byte-exact pattern atoms (PatternOperatorMatchUntil.java
// PatternOp case-list order, lines 91-315 verbatim): `until`, `->`,
// `[N]`, `[N:M]`, `[N:]` and `[:M]` bound spellings, parenthesization
// and `not`/`or`/`and`/`every` placement are pinned exactly as
// written — including S27/S28's same-event terminators
// `e=SupportBean_B(id='B2')`/`(id='B1')` and the timer legs'
// `timer:interval(7 sec)`/`(6 sec)` spellings.
const (
	patternMatchUntilWHarness588BeanAType = "SupportBean_A"
	patternMatchUntilWHarness588BeanBType = "SupportBean_B"
	patternMatchUntilWHarness588BeanCType = "SupportBean_C"
	patternMatchUntilWHarness588BeanDType = "SupportBean_D"
	patternMatchUntilWHarness588BeanEType = "SupportBean_E"
	patternMatchUntilWHarness588BeanFType = "SupportBean_F"
	patternMatchUntilWHarness588BeanGType = "SupportBean_G"
)

// patternMatchUntilWHarness588StatementText wraps a pinned atom in the
// deployed statement text: @name("S<i>") plus select * from pattern
// [<atom>] — the S0..S51 names replace the harness's name--<atom>
// labels while keeping the observable listener surface identical.
func patternMatchUntilWHarness588StatementText(leg int, atom string) string {
	return fmt.Sprintf("@name(\"S%d\") select * from pattern [%s]", leg, atom)
}

// patternMatchUntilWHarness588LegName is the statement name leg i
// deploys under and the label the listener records carry.
func patternMatchUntilWHarness588LegName(leg int) string {
	return fmt.Sprintf("S%d", leg)
}

// Per-case Java identity: the single w-harness case owns the ord 1
// runtimeId; legs index into the atom table below.
var (
	patternMatchUntilWHarness588JavaRuntimeIDs = []string{
		"java-runtime-0383244f373c8a0ffc8b",
	}
	patternMatchUntilWHarness588JavaSources = []string{
		patternMatchUntilWHarness588JavaSource,
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
	patternMatchUntilWHarness588JavaExecutions = []string{
		"PatternOp",
	}
	patternMatchUntilWHarness588JavaStaticIDs = []string{
		"java-d4cb4534ca508be87595",
	}
	patternMatchUntilWHarness588JavaFlags = []string{}
)

// patternMatchUntilWHarness588BeanA..G mirror SupportBean_A..G: each
// carries the single id property select * projects as a fragment.
type patternMatchUntilWHarness588BeanA struct {
	ID string `json:"id" esper:"id"`
}
type patternMatchUntilWHarness588BeanB struct {
	ID string `json:"id" esper:"id"`
}
type patternMatchUntilWHarness588BeanC struct {
	ID string `json:"id" esper:"id"`
}
type patternMatchUntilWHarness588BeanD struct {
	ID string `json:"id" esper:"id"`
}
type patternMatchUntilWHarness588BeanE struct {
	ID string `json:"id" esper:"id"`
}
type patternMatchUntilWHarness588BeanF struct {
	ID string `json:"id" esper:"id"`
}
type patternMatchUntilWHarness588BeanG struct {
	ID string `json:"id" esper:"id"`
}

// patternMatchUntilWHarness588LegAtoms are the 52 EventExpressionCase
// atoms in case-list order (statement S<i> deploys leg i).
var patternMatchUntilWHarness588LegAtoms = []string{
	"a=SupportBean_A(id='A2') until SupportBean_D",                                    // S0
	"a=SupportBean_A until SupportBean_D",                                             // S1
	"b=SupportBean_B until a=SupportBean_A",                                           // S2
	"b=SupportBean_B until SupportBean_D(id='D3')",                                    // S3
	"(a=SupportBean_A or b=SupportBean_B) until d=SupportBean_D(id='D3')",             // S4
	"(a=SupportBean_A or b=SupportBean_B) until (g=SupportBean_G or d=SupportBean_D)", // S5
	"(d=SupportBean_D) until a=SupportBean_A(id='A1')",                                // S6
	"a=SupportBean_A until SupportBean_G(id='GX')",                                    // S7
	"[2] a=SupportBean_A",                                                             // S8
	"[2:2] a=SupportBean_A",                                                           // S9
	"[1] a=SupportBean_A",                                                             // S10
	"[1:1] a=SupportBean_A",                                                           // S11
	"[3] a=SupportBean_A",                                                             // S12
	"[3] b=SupportBean_B",                                                             // S13
	"[4] (a=SupportBean_A or b=SupportBean_B)",                                        // S14
	"[2] b=SupportBean_B until a=SupportBean_A(id='A1')",                              // S15
	"[2] b=SupportBean_B until c=SupportBean_C",                                       // S16
	"[2:2] b=SupportBean_B until g=SupportBean_G(id='G1')",                            // S17
	"[:4] b=SupportBean_B until g=SupportBean_G(id='G1')",                             // S18
	"[:3] b=SupportBean_B until g=SupportBean_G(id='G1')",                             // S19
	"[:2] b=SupportBean_B until g=SupportBean_G(id='G1')",                             // S20
	"[:1] b=SupportBean_B until g=SupportBean_G(id='G1')",                             // S21
	"[:1] b=SupportBean_B until a=SupportBean_A(id='A1')",                             // S22
	"[1:] b=SupportBean_B until g=SupportBean_G(id='G1')",                             // S23
	"[1:] b=SupportBean_B until a=SupportBean_A",                                      // S24
	"[2:] b=SupportBean_B until a=SupportBean_A(id='A2')",                             // S25
	"[2:] b=SupportBean_B until c=SupportBean_C",                                      // S26
	"[2:] b=SupportBean_B until e=SupportBean_B(id='B2')",                             // S27
	"[1:] b=SupportBean_B until e=SupportBean_B(id='B1')",                             // S28
	"[1:2] b=SupportBean_B until a=SupportBean_A(id='A2')",                            // S29
	"[1:3] b=SupportBean_B until SupportBean_G",                                       // S30
	"[1:2] b=SupportBean_B until SupportBean_G",                                       // S31
	"[1:10] b=SupportBean_B until SupportBean_F",                                      // S32
	"[1:10] b=SupportBean_B until SupportBean_C",                                      // S33
	"[0:1] b=SupportBean_B until SupportBean_C",                                       // S34
	"c=SupportBean_C -> [2] b=SupportBean_B -> d=SupportBean_D",                       // S35
	"[3] d=SupportBean_D or [3] b=SupportBean_B",                                      // S36
	"[3] d=SupportBean_D or [4] b=SupportBean_B",                                      // S37
	"[2] d=SupportBean_D and [2] b=SupportBean_B",                                     // S38
	"d=SupportBean_D until timer:interval(7 sec)",                                     // S39
	"every (d=SupportBean_D until b=SupportBean_B)",                                   // S40
	"every d=SupportBean_D until b=SupportBean_B",                                     // S41
	"(every d=SupportBean_D) until b=SupportBean_B",                                   // S42
	"a=SupportBean_A until (every (timer:interval(6 sec) and not SupportBean_A))",     // S43
	"SupportBean_A until (every (timer:interval(7 sec) and not SupportBean_A))",       // S44
	"[2] (a=SupportBean_A or b=SupportBean_B)",                                        // S45
	"every [2] a=SupportBean_A",                                                       // S46
	"every [2] a=SupportBean_A until d=SupportBean_D",                                 // S47
	"[3] (a=SupportBean_A or b=SupportBean_B)",                                        // S48
	"(a=SupportBean_A until b=SupportBean_B) until c=SupportBean_C",                   // S49
	"(a=SupportBean_A until b=SupportBean_B) until g=SupportBean_G",                   // S50
	"SupportBean_B until not SupportBean_B",                                           // S51
}

// patternMatchUntilWHarness588CaseSpec pins the single case: Java
// execution identity (ordinal plus runtimeIndex), the observation text
// and the fifty-two byte-exact deployed statement texts pinned through
// case.epls.
type patternMatchUntilWHarness588CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epls         []string
}

var patternMatchUntilWHarness588CaseSpecs = []patternMatchUntilWHarness588CaseSpec{
	{
		name:         "w-harness",
		ordinal:      1,
		runtimeIndex: 0,
		observation: "listener; external clock +1000ms per send; all 52 legs deploy in ONE step " +
			"before any send (harness deploy-all), then twelve advance-before-send " +
			"replays of A1,B1,C1,B2,A2,D1,E1,F1,D2,B3,G1,D3 (advanceTime precedes " +
			"sendEventBean so timer expiries attribute to the upcoming bucket): S0 " +
			"D1{a:A2}, S1 D1{a:A1,A2}, S2 A1{b empty,a:A1}, S3 D3{b:B1,B2,B3}, S4 " +
			"D3{a:A1,A2;b:B1,B2,B3;d:D3}, S5 D1{a:A1,A2;b:B1,B2;d:D1}, S6 A1{a:A1}, " +
			"S7 silent, S8/S9 A2{a:A1,A2}, S10/S11 A1{a:A1}, S12 silent, S13 " +
			"B3{b:B1,B2,B3}, S14 A2{a:A1,A2;b:B1,B2}, S15/S16 silent, S17 B2{b:B1,B2}, " +
			"S18/S19 G1{b:B1,B2,B3;g:G1}, S20 G1{b:B1,B2;g:G1}, S21 G1{b:B1;g:G1}, " +
			"S22 A1{b empty,a:A1}, S23 G1{b:B1,B2,B3;g:G1}, S24 silent, S25 " +
			"A2{b:B1,B2;a:A2}, S26/S27/S28 silent, S29 A2{b:B1,B2;a:A2}, S30 " +
			"G1{b:B1,B2,B3}, S31 G1{b:B1,B2}, S32 F1{b:B1,B2}, S33/S34 C1{b:B1}, S35 " +
			"D3{c:C1;b:B2,B3;d:D3}, S36 B3{b:B1,B2,B3}, S37 D3{d:D1,D2,D3}, S38 " +
			"D2{b:B1,B2;d:D1,D2}, S39 E1{d:D1} (timer fires inside the advance to " +
			"7000), S40 B1/B2/B3 (d:D1,D2 collected on B3), S41/S42 B1 only (every " +
			"precedence), S43 G1{a:A1,A2} (6s timer+not-A terminator), S44 D3 fire " +
			"with no tags (7s every-timer), S45 B1{a:A1;b:B1}, S46 A2{a:A1,A2}, S47 " +
			"D1{a:A1,A2;d:D1} (ESPER-339), S48 B2{a:A1;b:B1,B2}, S49 C1{a:A1;b:B1;" +
			"c:C1}, S50 G1{a:A1,A2;b:B1,B2,B3;g:G1}, S51 silent (start-fire ignored); " +
			"per leg+trigger the fires compare as a multiset like compareLists",
		epls: func() []string {
			epls := make([]string, 0, len(patternMatchUntilWHarness588LegAtoms))
			for leg, atom := range patternMatchUntilWHarness588LegAtoms {
				epls = append(epls, patternMatchUntilWHarness588StatementText(leg, atom))
			}
			return epls
		}(),
	},
}

// patternMatchUntilWHarness588StepPin pins one scenario step's shape:
// the deploy-all step carries only the "all" statement label (the leg
// texts pin through case.epls), send steps carry the fused
// advance-before-send instant plus the event payload and undeploy-all
// closes the case.
type patternMatchUntilWHarness588StepPin struct {
	op        string
	statement string
	at        string
	eventType string
	payload   map[string]any
}

func patternMatchUntilWHarness588DeployAllPin() patternMatchUntilWHarness588StepPin {
	return patternMatchUntilWHarness588StepPin{op: "deploy", statement: "all"}
}

func patternMatchUntilWHarness588SendPin(at, eventType string, payload map[string]any) patternMatchUntilWHarness588StepPin {
	return patternMatchUntilWHarness588StepPin{op: "send", at: at, eventType: eventType, payload: payload}
}

func patternMatchUntilWHarness588IDSendPin(at int, eventType, id string) patternMatchUntilWHarness588StepPin {
	return patternMatchUntilWHarness588SendPin(patternMatchUntilWHarness588EventAt(at), eventType, map[string]any{"id": id})
}

func patternMatchUntilWHarness588UndeployAllPin() patternMatchUntilWHarness588StepPin {
	return patternMatchUntilWHarness588StepPin{op: "undeploy-all"}
}

// patternMatchUntilWHarness588EventAt renders one event-time instant:
// the harness's sendEventCollection.getTime(eventId) advance —
// A1=1000ms .. D3=12000ms on the external clock.
func patternMatchUntilWHarness588EventAt(offsetMS int) string {
	return time.Unix(0, 0).UTC().Add(time.Duration(offsetMS) * time.Millisecond).Format(time.RFC3339Nano)
}

var patternMatchUntilWHarness588EventSet = []struct {
	at        int
	eventType string
	id        string
}{
	{1000, patternMatchUntilWHarness588BeanAType, "A1"},
	{2000, patternMatchUntilWHarness588BeanBType, "B1"},
	{3000, patternMatchUntilWHarness588BeanCType, "C1"},
	{4000, patternMatchUntilWHarness588BeanBType, "B2"},
	{5000, patternMatchUntilWHarness588BeanAType, "A2"},
	{6000, patternMatchUntilWHarness588BeanDType, "D1"},
	{7000, patternMatchUntilWHarness588BeanEType, "E1"},
	{8000, patternMatchUntilWHarness588BeanFType, "F1"},
	{9000, patternMatchUntilWHarness588BeanDType, "D2"},
	{10000, patternMatchUntilWHarness588BeanBType, "B3"},
	{11000, patternMatchUntilWHarness588BeanGType, "G1"},
	{12000, patternMatchUntilWHarness588BeanDType, "D3"},
}

// patternMatchUntilWHarness588CaseSteps pins the complete step sequence
// in harness order: ONE deploy-all step expands to the fifty-two leg
// deployments (the USE_EPL style compiles all statements before the
// event loop), then each event's fused advance-before-send step, then
// undeploy-all whose post-undeploy resend proves silence. env.milestone
// savepoints, the three other replay styles and the ON_START
// advance-time carry no scenario op.
var patternMatchUntilWHarness588CaseSteps = func() map[string][]patternMatchUntilWHarness588StepPin {
	steps := make(map[string][]patternMatchUntilWHarness588StepPin)
	for _, spec := range patternMatchUntilWHarness588CaseSpecs {
		pins := []patternMatchUntilWHarness588StepPin{
			patternMatchUntilWHarness588DeployAllPin(),
		}
		for _, event := range patternMatchUntilWHarness588EventSet {
			pins = append(pins,
				patternMatchUntilWHarness588IDSendPin(event.at, event.eventType, event.id))
		}
		pins = append(pins, patternMatchUntilWHarness588UndeployAllPin())
		steps[spec.name] = pins
	}
	return steps
}()

func loadPatternMatchUntilWHarness588Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternMatchUntilWHarness588ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternMatchUntilWHarness588ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternMatchUntilWHarness588ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternMatchUntilWHarness588ID, err)
	}
	if err := requirePatternMatchUntilWHarness588Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternMatchUntilWHarness588ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternMatchUntilWHarness588ID ||
		metadata.Description != patternMatchUntilWHarness588Description ||
		metadata.JavaCommit != patternMatchUntilWHarness588JavaCommit ||
		metadata.JavaSource != patternMatchUntilWHarness588JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternMatchUntilWHarness588ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternMatchUntilWHarness588JavaSources},
		{metadata.JavaRuntimes, patternMatchUntilWHarness588JavaRuntimeIDs},
		{metadata.JavaNames, patternMatchUntilWHarness588JavaExecutions},
		{metadata.JavaStaticIDs, patternMatchUntilWHarness588JavaStaticIDs},
		{metadata.JavaFlags, patternMatchUntilWHarness588JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternMatchUntilWHarness588ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternMatchUntilWHarness588CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternMatchUntilWHarness588ID, len(patternMatchUntilWHarness588CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternMatchUntilWHarness588Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternMatchUntilWHarness588CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternMatchUntilWHarness588JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternMatchUntilWHarness588JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || !reflect.DeepEqual(definition.EPLs, spec.epls) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternMatchUntilWHarness588ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternMatchUntilWHarness588ID)
	}
	if err := validatePatternMatchUntilWHarness588RawSteps(rawSteps); err != nil {
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

// validatePatternMatchUntilWHarness588RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned sequence — the ONE deploy-all step (statement
// "all"), the twelve fused advance-before-send steps with pinned
// instants/payloads and the closing undeploy-all.
func validatePatternMatchUntilWHarness588RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternMatchUntilWHarness588CaseSpecs))
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
			if err := requirePatternMatchUntilWHarness588Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternMatchUntilWHarness588CaseSteps[marker.Case]; !ok {
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
		pins := patternMatchUntilWHarness588CaseSteps[currentCase]
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
			if err := requirePatternMatchUntilWHarness588Fields(object, "op", "case", "statement"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternMatchUntilWHarness588Fields(object, "op", "case", "at", "eventType", "payload"); err != nil {
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
			if err := requirePatternMatchUntilWHarness588Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternMatchUntilWHarness588CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternMatchUntilWHarness588CaseSpecs))
	}
	for index, spec := range patternMatchUntilWHarness588CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternMatchUntilWHarness588CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternMatchUntilWHarness588CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternMatchUntilWHarness588Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternMatchUntilWHarness588CaseState carries the replay state: the
// single DeployPlans deployment of the 52 legs, the deployed
// per-leg statements keyed by their S0..S51 labels, per-statement
// listener sequence counters and the delivery records.
type patternMatchUntilWHarness588CaseState struct {
	spec        patternMatchUntilWHarness588CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	deployment  *esper.Deployment
	statements  map[string]*esper.Statement
	sequence    map[string]uint64
	records     []compat.TraceRecord
	deployedAll bool
}

// runPatternMatchUntilWHarness588Scenario replays the execution against
// a fresh engine like the Java oracle's fresh per-execution runtime —
// the fifty-two legs share ONE engine and ONE DeployPlans deployment
// because the harness deploys all statements before replaying the
// event set.
func runPatternMatchUntilWHarness588Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternMatchUntilWHarness588Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternMatchUntilWHarness588CaseSpecs {
		records, err := runPatternMatchUntilWHarness588Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternMatchUntilWHarness588ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternMatchUntilWHarness588Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternMatchUntilWHarness588ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternMatchUntilWHarness588ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternMatchUntilWHarness588RawSteps(rawSteps)
}

func runPatternMatchUntilWHarness588Case(ctx context.Context, scenario compat.Scenario, spec patternMatchUntilWHarness588CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternMatchUntilWHarness588BeanA](env, patternMatchUntilWHarness588BeanAType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilWHarness588BeanB](env, patternMatchUntilWHarness588BeanBType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilWHarness588BeanC](env, patternMatchUntilWHarness588BeanCType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilWHarness588BeanD](env, patternMatchUntilWHarness588BeanDType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilWHarness588BeanE](env, patternMatchUntilWHarness588BeanEType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilWHarness588BeanF](env, patternMatchUntilWHarness588BeanFType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilWHarness588BeanG](env, patternMatchUntilWHarness588BeanGType); err != nil {
		return nil, err
	}
	state := &patternMatchUntilWHarness588CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternMatchUntilWHarness588JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			payload, err := patternMatchUntilWHarness588DecodePayload(step)
			if err != nil {
				return nil, err
			}
			// The fused step performs the harness's advanceTime
			// BEFORE sendEventBean: timer legs whose expirations land
			// inside the advance (S39/S43/S44) attribute to the
			// upcoming event's bucket exactly like the harness's
			// checkResults-after-sendEventBean ordering.
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse send at %q: %w", patternMatchUntilWHarness588ID, step.At, err)
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
			for _, event := range patternMatchUntilWHarness588EventSet {
				bean, err := patternMatchUntilWHarness588TypedEvent(event.eventType, event.id)
				if err != nil {
					return nil, err
				}
				if err := state.engine.Send(ctx, event.eventType, bean); err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternMatchUntilWHarness588ID, step.Op)
		}
	}
	return state.records, nil
}

// patternMatchUntilWHarness588Tag pins one projected select-* column:
// repeated marks a match-until (or every-collected) tag whose fragment
// is the bean array (TagEvents), otherwise the tag is a single bean
// fragment (PatternEvent). Untagged atoms (S0/S1/S3/S7 terminators and
// the S44 repeated atom, S51's not leg) project no column, matching
// Esper's select * over nameless pattern atoms.
type patternMatchUntilWHarness588Tag struct {
	name     string
	repeated bool
}

// patternMatchUntilWHarness588LegSpec pins one leg's fluent build plus
// its select-* fragment columns.
type patternMatchUntilWHarness588LegSpec struct {
	build func(patternMatchUntilWHarness588Streams) esper.PatternStream
	tags  []patternMatchUntilWHarness588Tag
}

// patternMatchUntilWHarness588Streams bundles the typed streams of the
// mixed event set plus the leg predicate helpers shared by all 52 legs
// of the DeployPlans deployment.
type patternMatchUntilWHarness588Streams struct {
	a esper.Stream[patternMatchUntilWHarness588BeanA]
	b esper.Stream[patternMatchUntilWHarness588BeanB]
	c esper.Stream[patternMatchUntilWHarness588BeanC]
	d esper.Stream[patternMatchUntilWHarness588BeanD]
	e esper.Stream[patternMatchUntilWHarness588BeanE]
	f esper.Stream[patternMatchUntilWHarness588BeanF]
	g esper.Stream[patternMatchUntilWHarness588BeanG]

	always esper.Expression[bool]
	idA    func(id string) esper.Expression[bool]
	idB    func(id string) esper.Expression[bool]
	idD    func(id string) esper.Expression[bool]
	idG    func(id string) esper.Expression[bool]
}

func newPatternMatchUntilWHarness588Streams(env *esper.Environment) patternMatchUntilWHarness588Streams {
	s := patternMatchUntilWHarness588Streams{
		a:      esper.From[patternMatchUntilWHarness588BeanA](env, patternMatchUntilWHarness588BeanAType),
		b:      esper.From[patternMatchUntilWHarness588BeanB](env, patternMatchUntilWHarness588BeanBType),
		c:      esper.From[patternMatchUntilWHarness588BeanC](env, patternMatchUntilWHarness588BeanCType),
		d:      esper.From[patternMatchUntilWHarness588BeanD](env, patternMatchUntilWHarness588BeanDType),
		e:      esper.From[patternMatchUntilWHarness588BeanE](env, patternMatchUntilWHarness588BeanEType),
		f:      esper.From[patternMatchUntilWHarness588BeanF](env, patternMatchUntilWHarness588BeanFType),
		g:      esper.From[patternMatchUntilWHarness588BeanG](env, patternMatchUntilWHarness588BeanGType),
		always: esper.Literal[bool](true),
	}
	s.idA = func(id string) esper.Expression[bool] {
		return esper.Equal[string](esper.Field[patternMatchUntilWHarness588BeanA, string]("id"), esper.Literal(id))
	}
	s.idB = func(id string) esper.Expression[bool] {
		return esper.Equal[string](esper.Field[patternMatchUntilWHarness588BeanB, string]("id"), esper.Literal(id))
	}
	s.idD = func(id string) esper.Expression[bool] {
		return esper.Equal[string](esper.Field[patternMatchUntilWHarness588BeanD, string]("id"), esper.Literal(id))
	}
	s.idG = func(id string) esper.Expression[bool] {
		return esper.Equal[string](esper.Field[patternMatchUntilWHarness588BeanG, string]("id"), esper.Literal(id))
	}
	return s
}

// patternMatchUntilWHarness588LegSpecs returns the 52 leg builders in
// case-list order; each builder closes over the shared streams bundle
// passed by deployAll so all 52 plans deploy on the same input streams.
// Leg i of this slice is statement S<i>; the builds mirror
// internal/esper/pattern_matchuntil_parity_test.go's
// TestPatternMatchUntilOp{Basic,Bounds,Composition}MatchesEsper legs
// one-to-one, except untagged atoms now carry "" instead of a
// synthetic name so select * binds no phantom column, and S27/S28's
// `e=SupportBean_B(id=...)` terminators read the B stream (the
// corpus's patternNotE typo only ever produced the silent result the
// until-wins semantics require).
func patternMatchUntilWHarness588LegSpecs() []patternMatchUntilWHarness588LegSpec {
	tag := func(name string, repeated bool) patternMatchUntilWHarness588Tag {
		return patternMatchUntilWHarness588Tag{name: name, repeated: repeated}
	}
	return []patternMatchUntilWHarness588LegSpec{
		// S0 `a=A(id='A2') until D` — unbounded until collects a into
		// an array; the untagged D terminator binds no column.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.idA("A2")).Until(esper.PatternFrom(s.d, "", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("a", true)}},
		// S1 `a=A until D`.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).Until(esper.PatternFrom(s.d, "", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("a", true)}},
		// S2 `b=B until a=A` — fires AT A1 with b's array empty.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).Until(esper.PatternFrom(s.a, "a", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("a", false)}},
		// S3 `b=B until D(id='D3')`.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).Until(esper.PatternFrom(s.d, "", s.idD("D3")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true)}},
		// S4 `(a or b) until d=D(id='D3')` — both or-arms collect.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).
				Or(esper.PatternFrom(s.b, "b", s.always)).
				Until(esper.PatternFrom(s.d, "d", s.idD("D3")))
		}, []patternMatchUntilWHarness588Tag{tag("a", true), tag("b", true), tag("d", false)}},
		// S5 `(a or b) until (g or d)` — terminates at D1.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			terminator := esper.PatternFrom(s.g, "g", s.always).Or(esper.PatternFrom(s.d, "d", s.always))
			return esper.PatternFrom(s.a, "a", s.always).
				Or(esper.PatternFrom(s.b, "b", s.always)).Until(terminator)
		}, []patternMatchUntilWHarness588Tag{tag("a", true), tag("b", true), tag("g", false), tag("d", false)}},
		// S6 `(d=D) until a=A(id='A1')` — terminates immediately.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.d, "d", s.always).Until(esper.PatternFrom(s.a, "a", s.idA("A1")))
		}, []patternMatchUntilWHarness588Tag{tag("d", true), tag("a", false)}},
		// S7 `a=A until G(id='GX')` — silent (no GX arrives).
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).Until(esper.PatternFrom(s.g, "", s.idG("GX")))
		}, []patternMatchUntilWHarness588Tag{tag("a", true)}},
		// S8 `[2] a` — tightly-bound [2:2] repeat, no terminator.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).MatchUntil(2, 2)
		}, []patternMatchUntilWHarness588Tag{tag("a", true)}},
		// S9 `[2:2] a` — identical bound spelling.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).MatchUntil(2, 2)
		}, []patternMatchUntilWHarness588Tag{tag("a", true)}},
		// S10 `[1] a`.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).MatchUntil(1, 1)
		}, []patternMatchUntilWHarness588Tag{tag("a", true)}},
		// S11 `[1:1] a`.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).MatchUntil(1, 1)
		}, []patternMatchUntilWHarness588Tag{tag("a", true)}},
		// S12 `[3] a` — silent (only two As arrive).
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).MatchUntil(3, 3)
		}, []patternMatchUntilWHarness588Tag{tag("a", true)}},
		// S13 `[3] b`.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(3, 3)
		}, []patternMatchUntilWHarness588Tag{tag("b", true)}},
		// S14 `[4] (a or b)` — or-repeat interleaves by arrival.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).
				Or(esper.PatternFrom(s.b, "b", s.always)).MatchUntil(4, 4)
		}, []patternMatchUntilWHarness588Tag{tag("a", true), tag("b", true)}},
		// S15 `[2] b until a=A(id='A1')` — the until kills the
		// sub-minimum repetition permanently; silent.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(2, 2).
				Until(esper.PatternFrom(s.a, "a", s.idA("A1")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("a", false)}},
		// S16 `[2] b until c=C` — silent.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(2, 2).
				Until(esper.PatternFrom(s.c, "c", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("c", false)}},
		// S17 `[2:2] b until g=G(id='G1')` — the tight bound fires at
		// B2 without waiting for the terminator.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(2, 2).
				Until(esper.PatternFrom(s.g, "g", s.idG("G1")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("g", false)}},
		// S18 `[:4] b until g=G(id='G1')`.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(0, 4).
				Until(esper.PatternFrom(s.g, "g", s.idG("G1")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("g", false)}},
		// S19 `[:3] b until g=G(id='G1')`.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(0, 3).
				Until(esper.PatternFrom(s.g, "g", s.idG("G1")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("g", false)}},
		// S20 `[:2] b until g=G(id='G1')` — max caps the collection.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(0, 2).
				Until(esper.PatternFrom(s.g, "g", s.idG("G1")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("g", false)}},
		// S21 `[:1] b until g=G(id='G1')`.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(0, 1).
				Until(esper.PatternFrom(s.g, "g", s.idG("G1")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("g", false)}},
		// S22 `[:1] b until a=A(id='A1')` — fires AT A1, b empty.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(0, 1).
				Until(esper.PatternFrom(s.a, "a", s.idA("A1")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("a", false)}},
		// S23 `[1:] b until g=G(id='G1')` — unbounded above min 1.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(1, 0).
				Until(esper.PatternFrom(s.g, "g", s.idG("G1")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("g", false)}},
		// S24 `[1:] b until a=A` — terminator below minimum: silent.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(1, 0).
				Until(esper.PatternFrom(s.a, "a", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("a", false)}},
		// S25 `[2:] b until a=A(id='A2')`.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(2, 0).
				Until(esper.PatternFrom(s.a, "a", s.idA("A2")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("a", false)}},
		// S26 `[2:] b until c=C` — terminator below minimum: silent.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(2, 0).
				Until(esper.PatternFrom(s.c, "c", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("c", false)}},
		// S27 `[2:] b until e=B(id='B2')` — the until branch evaluates
		// first so the shared B2 event resolves to the terminator;
		// silent. e tags a B-stream atom.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(2, 0).
				Until(esper.PatternFrom(s.b, "e", s.idB("B2")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("e", false)}},
		// S28 `[1:] b until e=B(id='B1')` — same until-wins on B1.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(1, 0).
				Until(esper.PatternFrom(s.b, "e", s.idB("B1")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("e", false)}},
		// S29 `[1:2] b until a=A(id='A2')` — max truncates before the
		// terminator arrives.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(1, 2).
				Until(esper.PatternFrom(s.a, "a", s.idA("A2")))
		}, []patternMatchUntilWHarness588Tag{tag("b", true), tag("a", false)}},
		// S30 `[1:3] b until G` — the terminating G atom's tag is
		// dropped from the wildcard row (Java emits only b at G1).
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(1, 3).
				Until(esper.PatternFrom(s.g, "g", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("b", true)}},
		// S31 `[1:2] b until G` — same terminator-tag drop.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(1, 2).
				Until(esper.PatternFrom(s.g, "g", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("b", true)}},
		// S32 `[1:10] b until F` — terminates at F1, f tag dropped.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(1, 10).
				Until(esper.PatternFrom(s.f, "f", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("b", true)}},
		// S33 `[1:10] b until C` — terminates at C1, c tag dropped.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(1, 10).
				Until(esper.PatternFrom(s.c, "c", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("b", true)}},
		// S34 `[0:1] b until C` — same terminator-tag drop.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "b", s.always).MatchUntil(0, 1).
				Until(esper.PatternFrom(s.c, "c", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("b", true)}},
		// S35 `c -> [2] b -> d` — the chain's B pair collects B2/B3.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.c, "c", s.always).
				Then(esper.PatternFrom(s.b, "b", s.always).MatchUntil(2, 2)).
				Then(esper.PatternFrom(s.d, "d", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("c", false), tag("b", true), tag("d", false)}},
		// S36 `[3] d or [3] b` — the b arm fires at B3.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.d, "d", s.always).MatchUntil(3, 3).
				Or(esper.PatternFrom(s.b, "b", s.always).MatchUntil(3, 3))
		}, []patternMatchUntilWHarness588Tag{tag("d", true), tag("b", true)}},
		// S37 `[3] d or [4] b` — the d arm fires at D3.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.d, "d", s.always).MatchUntil(3, 3).
				Or(esper.PatternFrom(s.b, "b", s.always).MatchUntil(4, 4))
		}, []patternMatchUntilWHarness588Tag{tag("d", true), tag("b", true)}},
		// S38 `[2] d and [2] b` — both arrays complete by D2.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.d, "d", s.always).MatchUntil(2, 2).
				And(esper.PatternFrom(s.b, "b", s.always).MatchUntil(2, 2))
		}, []patternMatchUntilWHarness588Tag{tag("d", true), tag("b", true)}},
		// S39 `d until timer:interval(7 sec)` — the 7000ms expiry
		// inside the advance to E1 attributes to the E1 bucket.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.d, "d", s.always).
				Until(esper.TimerInterval(s.d, 7*time.Second))
		}, []patternMatchUntilWHarness588Tag{tag("d", true)}},
		// S40 `every (d until b)` — the until collects d's between
		// every re-arms; fires at B1, B2 and B3 (three records).
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.d, "d", s.always).
				Until(esper.PatternFrom(s.b, "b", s.always)).Every()
		}, []patternMatchUntilWHarness588Tag{tag("d", true), tag("b", false)}},
		// S41 `every d until b` — every binds tighter than until, so
		// only the first fire (B1) lands.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.d, "d", s.always).Every().
				Until(esper.PatternFrom(s.b, "b", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("d", true), tag("b", false)}},
		// S42 `(every d) until b` — same precedence, same single fire.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.d, "d", s.always).Every().
				Until(esper.PatternFrom(s.b, "b", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("d", true), tag("b", false)}},
		// S43 `a until (every (timer:interval(6 sec) and not A))` —
		// the first uncancelled 6s timer completes the terminator at
		// the G1 advance (every re-arms after A1/A2 cancel attempts).
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			terminator := esper.TimerInterval(s.a, 6*time.Second).
				And(esper.PatternFrom(s.a, "", s.always).Not()).Every()
			return esper.PatternFrom(s.a, "a", s.always).Until(terminator)
		}, []patternMatchUntilWHarness588Tag{tag("a", true)}},
		// S44 `A until (every (timer:interval(7 sec) and not A))` —
		// fires at the D3 advance; the repeated atom is UNTAGGED so the
		// fire row carries no columns.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			terminator := esper.TimerInterval(s.a, 7*time.Second).
				And(esper.PatternFrom(s.a, "", s.always).Not()).Every()
			return esper.PatternFrom(s.a, "", s.always).Until(terminator)
		}, nil},
		// S45 `[2] (a or b)` — tight bound fires at B1 (second match).
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).
				Or(esper.PatternFrom(s.b, "b", s.always)).MatchUntil(2, 2)
		}, []patternMatchUntilWHarness588Tag{tag("a", true), tag("b", true)}},
		// S46 `every [2] a` — every re-arms the tight pair; only A1+A2
		// complete once.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).MatchUntil(2, 2).Every()
		}, []patternMatchUntilWHarness588Tag{tag("a", true)}},
		// S47 `every [2] a until d` — ESPER-339 precedence: every
		// binds the [2]a inside the until's left side; fires at D1.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).MatchUntil(2, 2).Every().
				Until(esper.PatternFrom(s.d, "d", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("a", true), tag("d", false)}},
		// S48 `[3] (a or b)` — fires at B2 (third match).
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.a, "a", s.always).
				Or(esper.PatternFrom(s.b, "b", s.always)).MatchUntil(3, 3)
		}, []patternMatchUntilWHarness588Tag{tag("a", true), tag("b", true)}},
		// S49 `(a until b) until c` — nested until collects inner a/b
		// arrays; terminates at C1.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			inner := esper.PatternFrom(s.a, "a", s.always).Until(esper.PatternFrom(s.b, "b", s.always))
			return inner.Until(esper.PatternFrom(s.c, "c", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("a", true), tag("b", true), tag("c", false)}},
		// S50 `(a until b) until g` — same nesting, outer waits for G1.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			inner := esper.PatternFrom(s.a, "a", s.always).Until(esper.PatternFrom(s.b, "b", s.always))
			return inner.Until(esper.PatternFrom(s.g, "g", s.always))
		}, []patternMatchUntilWHarness588Tag{tag("a", true), tag("b", true), tag("g", false)}},
		// S51 `B until not B` — start-firing leg the harness ignores;
		// both atoms are untagged so a fired row would carry no
		// columns.
		{func(s patternMatchUntilWHarness588Streams) esper.PatternStream {
			return esper.PatternFrom(s.b, "", s.always).
				Until(esper.PatternFrom(s.b, "", s.always).Not())
		}, nil},
	}
}

// deployAll expands the ONE deploy-all step into one DeployPlans call
// carrying all 52 plans in case order — statement S<i> deploys leg i —
// then attaches one listener per statement. select * expands to the
// per-tag fragment columns: the repeated tag as its collected bean
// array (TagEvents) and each single tag as the bean fragment
// (PatternEvent), matching the Java wildcard fragments.
func (s *patternMatchUntilWHarness588CaseState) deployAll(ctx context.Context, step compat.Step) error {
	if s.deployedAll || step.Statement != "all" {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternMatchUntilWHarness588ID, s.spec.name)
	}
	legs := patternMatchUntilWHarness588LegSpecs()
	streams := newPatternMatchUntilWHarness588Streams(s.env)
	if len(legs) != len(patternMatchUntilWHarness588LegAtoms) {
		return fmt.Errorf("%s: %d leg builders, want %d", patternMatchUntilWHarness588ID,
			len(legs), len(patternMatchUntilWHarness588LegAtoms))
	}
	plans := make([]esper.Plan, 0, len(legs))
	for leg, spec := range legs {
		selections := make([]esper.Selection, 0, len(spec.tags))
		for _, tag := range spec.tags {
			if tag.repeated {
				selections = append(selections, esper.Alias(tag.name, esper.TagEvents(tag.name)))
			} else {
				selections = append(selections, esper.Alias(tag.name, esper.PatternEvent(tag.name)))
			}
		}
		plan, err := s.env.Build(spec.build(streams).Select(selections...).Query(
			esper.StatementName(patternMatchUntilWHarness588LegName(leg))))
		if err != nil {
			return fmt.Errorf("%s: build leg S%d %q: %w", patternMatchUntilWHarness588ID, leg,
				patternMatchUntilWHarness588LegAtoms[leg], err)
		}
		plans = append(plans, plan)
	}
	deployment, err := s.engine.DeployPlans(ctx, plans)
	if err != nil {
		return fmt.Errorf("%s: deploy-all: %w", patternMatchUntilWHarness588ID, err)
	}
	statements := deployment.Statements()
	if len(statements) != len(legs) {
		return fmt.Errorf("%s: deploy-all produced %d statements, want %d",
			patternMatchUntilWHarness588ID, len(statements), len(legs))
	}
	s.deployment = deployment
	s.deployedAll = true
	for leg, statement := range statements {
		name := patternMatchUntilWHarness588LegName(leg)
		if statement.Name() != name {
			return fmt.Errorf("%s: statement %d is %q, want %q",
				patternMatchUntilWHarness588ID, leg, statement.Name(), name)
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

// patternMatchUntilWHarness588DecodePayload converts a scenario send
// payload into the typed host object: every SupportBean_? type carries
// the single pinned id property.
func patternMatchUntilWHarness588DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	if err := requirePatternMatchUntilWHarness588Fields(fields, "id"); err != nil {
		return nil, err
	}
	var id string
	if err := json.Unmarshal(fields["id"], &id); err != nil {
		return nil, fmt.Errorf("decode send id: %w", err)
	}
	return patternMatchUntilWHarness588TypedEvent(step.EventType, id)
}

// patternMatchUntilWHarness588TypedEvent rebuilds one mixed-set bean
// for the fused send steps and the post-undeploy resend silence check.
func patternMatchUntilWHarness588TypedEvent(eventType, id string) (any, error) {
	switch eventType {
	case patternMatchUntilWHarness588BeanAType:
		return patternMatchUntilWHarness588BeanA{ID: id}, nil
	case patternMatchUntilWHarness588BeanBType:
		return patternMatchUntilWHarness588BeanB{ID: id}, nil
	case patternMatchUntilWHarness588BeanCType:
		return patternMatchUntilWHarness588BeanC{ID: id}, nil
	case patternMatchUntilWHarness588BeanDType:
		return patternMatchUntilWHarness588BeanD{ID: id}, nil
	case patternMatchUntilWHarness588BeanEType:
		return patternMatchUntilWHarness588BeanE{ID: id}, nil
	case patternMatchUntilWHarness588BeanFType:
		return patternMatchUntilWHarness588BeanF{ID: id}, nil
	case patternMatchUntilWHarness588BeanGType:
		return patternMatchUntilWHarness588BeanG{ID: id}, nil
	}
	return nil, fmt.Errorf("unsupported %s event type %q", patternMatchUntilWHarness588ID, eventType)
}

// sortPatternMatchUntilWHarness588Records is the -diff normalizer for
// both traces: the harness compares each (statement, trigger-event)
// bucket as a multiset and Java's cross-statement dispatch order inside
// one send is unspecified, so records are sorted by (case, time,
// statement) with the serialized rows as the within-bucket tiebreak —
// a pure multiset canonicalization that keeps every record.
func sortPatternMatchUntilWHarness588Records(trace compat.Trace) compat.Trace {
	key := func(record compat.TraceRecord) string {
		newRows, _ := json.Marshal(record.New)
		oldRows, _ := json.Marshal(record.Old)
		return string(newRows) + "|" + string(oldRows)
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
	return trace
}
