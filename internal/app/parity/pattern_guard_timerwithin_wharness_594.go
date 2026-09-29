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

// Parity coverage for PatternGuardTimerWithin ord 0 PatternOp
// (java-runtime-bb8113cb979826cff927, static java-0dec801a426fed297402,
// static-manifest java-811b1a1fcf834ee01db8, flags []): the 34-leg
// timer:within W-harness over EventCollectionFactory.getEventSetOne
// (0,1000), replayed as ONE case like the 592 timer:interval sibling.
//
// The semantics under test, pinned against the Java oracle:
//   - Exclusive deadline: a guard armed at T with period P quits at
//     T+P via the timer callback inside the advance, so an event at
//     exactly arm+P loses (S0/S5/S17/S20/S21/S24/S27/S31/S33 are the
//     ±1ms discriminators; S0/S5 hit the exact boundary).
//   - `every X where G` parses as every(X where G) — the guarded child
//     serially retries after each expiry or match (S9/S10/S15).
//   - `(every X) where G` guards the whole every — expiry kills the
//     loop (S7 dies at 2001 after B1; S14 dies at 4001 after B2).
//   - `every ((every X) where G)` accumulates: the inner every's
//     filter child never quits so the guarded instance stays live and
//     the outer every spawns a sibling per match — B1×1, B2×2, B3×4
//     (S11/S12).
//   - `and` dies when either side's guard expires (S20/S21/S33) but a
//     live and retains the completed side for later pairing — S18/S29
//     deliver {b:B3, d:D2} into the B3 bucket because D2 was retained
//     from its own send.
//   - `or` completes on the first match and kills the sibling (S30)
//     but survives the OTHER side's expiry to deliver a single-tag row
//     (S31's {d:D1}).
//   - Guard respawns arm at the advance TARGET instant — a child
//     spawned inside an expiry callback is live for the same-tick
//     event (S23's respawn-at-6000 sees D1).
//
// Statement text: `@name("S<i>") select * from pattern [<atom>]`; the
// S0..S33 names stand in for the harness's name--<atom> labels (the
// SODA leg S3 deploys via the model path whose toEPL text
// `b=SupportBean_B(id="B3") where timer:within(10.001d)` is the same
// 10001ms guard as S4).
const patternGuardTimerWithinWHarness594ID = "pattern-guard-timerwithin-wharness-594"

const patternGuardTimerWithinWHarness594Description = "PatternGuardTimerWithin ord 0 PatternOp — the shared 34-leg timer:within W-harness over EventCollectionFactory.getEventSetOne(0,1000), replayed as ONE case: ONE deploy-all step (statement \"all\") expands to the harness's thirty-four per-leg deployments of `@name(\"S<i>\") select * from pattern [<atom>]` (S0..S33 stand in for the name--<atom> labels, S3 carrying the SODA model's toEPL text `b=SupportBean_B(id=\"B3\") where timer:within(10.001d)`), then twelve advance-before-send steps each advance the external clock to the event's pinned instant (+1000ms per send) BEFORE sendEventBean — guard expiry callbacks inside an advance run at the advance instant BEFORE the event, so an event at exactly arm+period loses (the deadline is exclusive) and a child respawned inside an expiry callback is live for the same-tick event (S23's respawn-at-6000 sees D1). Expected: 64 listener records — buckets B1=11, B2=8, D1=12, D2=4, B3=18, D3=9; silent legs S0,S2,S5,S6,S17,S20,S21,S24,S27,S28,S33 produce zero rows. env.milestone savepoints, the three other replay styles and the ON_START advance-time carry no scenario op; per leg+trigger the fires compare as a multiset like compareLists."

const patternGuardTimerWithinWHarness594JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternGuardTimerWithinWHarness594JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternGuardTimerWithin.java"

// patternGuardTimerWithinWHarness594StatementText wraps a pinned atom
// in the deployed statement text, like the 592 convention.
func patternGuardTimerWithinWHarness594StatementText(leg int, atom string) string {
	return fmt.Sprintf("@name(\"S%d\") select * from pattern [%s]", leg, atom)
}

func patternGuardTimerWithinWHarness594LegName(leg int) string {
	return fmt.Sprintf("S%d", leg)
}

// Per-case Java identity: the single w-harness case owns the ord 0
// runtimeId; the whole file shares the static id.
var (
	patternGuardTimerWithinWHarness594JavaRuntimeIDs = []string{
		"java-runtime-bb8113cb979826cff927",
	}
	patternGuardTimerWithinWHarness594JavaSources = []string{
		patternGuardTimerWithinWHarness594JavaSource,
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
	patternGuardTimerWithinWHarness594JavaExecutions = []string{
		"PatternOp",
	}
	patternGuardTimerWithinWHarness594JavaStaticIDs = []string{
		"java-0dec801a426fed297402",
	}
	patternGuardTimerWithinWHarness594JavaFlags = []string{}
)

// patternGuardTimerWithinWHarness594LegAtoms are the 34
// EventExpressionCase atoms in case-list order (statement S<i>
// deploys leg i); S3's atom is the SODA model leg's pinned toEPL text
// (`10.001d` double-literal = 10001ms, twin of S4). The unit spellings
// (`sec`/`msec`/`milliseconds`, bare decimal seconds) and the spaced
// `timer:within (...)` forms are pinned byte-exact from
// PatternGuardTimerWithin.java lines 52-223.
var patternGuardTimerWithinWHarness594LegAtoms = []string{
	"b=SupportBean_B(id='B1') where timer:within(2 sec)",                                                                                   // S0 — SILENT (B1@2000 = deadline)
	"b=SupportBean_B(id='B1') where timer:within(2001 msec)",                                                                               // S1 — @B1
	"b=SupportBean_B(id='B1') where timer:within(1999 msec)",                                                                               // S2 — SILENT
	`b=SupportBean_B(id="B3") where timer:within(10.001d)`,                                                                                 // S3 (SODA model leg) — @B3
	"b=SupportBean_B(id='B3') where timer:within(10001 msec)",                                                                              // S4 — @B3
	"b=SupportBean_B(id='B3') where timer:within(10 sec)",                                                                                  // S5 — SILENT (B3@10000 = deadline)
	"b=SupportBean_B(id='B3') where timer:within(9.999)",                                                                                   // S6 — SILENT
	"(every b=SupportBean_B) where timer:within(2.001)",                                                                                    // S7 — @B1 (every dies at 2001)
	"(every b=SupportBean_B) where timer:within(4.001)",                                                                                    // S8 — @B1,@B2
	"every b=SupportBean_B where timer:within(2.001)",                                                                                      // S9 — every(guarded b): @B1,@B2,@B3
	"every (b=SupportBean_B where timer:within(2001 msec))",                                                                                // S10 — @B1,@B2,@B3
	"every ((every b=SupportBean_B) where timer:within(2.001))",                                                                            // S11 — accumulation @B1×1,@B2×2,@B3×4
	"every ((every b=SupportBean_B) where timer:within(6.001))",                                                                            // S12 — same multiset as S11
	"(every b=SupportBean_B) where timer:within(11.001)",                                                                                   // S13 — @B1,@B2,@B3
	"(every b=SupportBean_B) where timer:within(4001 milliseconds)",                                                                        // S14 — @B1,@B2 (dies at 4001)
	"every (b=SupportBean_B) where timer:within(6.001)",                                                                                    // S15 — every(guarded b): @B1,@B2,@B3
	"b=SupportBean_B -> d=SupportBean_D where timer:within(4001 milliseconds)",                                                             // S16 — @D1 {b:B1,d:D1}
	"b=SupportBean_B() -> d=SupportBean_D() where timer:within(4 sec)",                                                                     // S17 — SILENT (D1@6000 = dl)
	"every (b=SupportBean_B() where timer:within (4.001) and d=SupportBean_D() where timer:within(6.001))",                                 // S18 — @D1 {B1,D1}; @B3 {B3,D2}
	"b=SupportBean_B() where timer:within (2001 msec) and d=SupportBean_D() where timer:within(6001 msec)",                                 // S19 — @D1 {B1,D1}
	"b=SupportBean_B() where timer:within (2001 msec) and d=SupportBean_D() where timer:within(6000 msec)",                                 // S20 — SILENT (d dies at D1)
	"b=SupportBean_B() where timer:within (2000 msec) and d=SupportBean_D() where timer:within(6001 msec)",                                 // S21 — SILENT (b dies at B1)
	"every b=SupportBean_B -> d=SupportBean_D where timer:within(4000 msec)",                                                               // S22 — @D1 {B2,D1}; @D3 {B3,D3}
	"every b=SupportBean_B() -> every d=SupportBean_D where timer:within(4000 msec)",                                                       // S23 — 7 rows (respawn sees D1)
	"b=SupportBean_B() -> d=SupportBean_D() where timer:within(3999 msec)",                                                                 // S24 — SILENT (dl 5999)
	"every b=SupportBean_B() -> (every d=SupportBean_D) where timer:within(2001 msec)",                                                     // S25 — @D1 {B2,D1}; @D3 {B3,D3}
	"every (b=SupportBean_B() -> d=SupportBean_D()) where timer:within(6001 msec)",                                                         // S26 — @D1 {B1,D1}; @D3 {B3,D3}
	"b=SupportBean_B() where timer:within (2000 msec) or d=SupportBean_D() where timer:within(6000 msec)",                                  // S27 — SILENT
	"(b=SupportBean_B() where timer:within (2000 msec) or d=SupportBean_D() where timer:within(6000 msec)) where timer:within (1999 msec)", // S28 — SILENT
	"every (b=SupportBean_B() where timer:within (2001 msec) and d=SupportBean_D() where timer:within(6001 msec))",                         // S29 — @D1 {B1,D1}; @B3 {B3,D2}
	"b=SupportBean_B() where timer:within (2001 msec) or d=SupportBean_D() where timer:within(6001 msec)",                                  // S30 — @B1 {b:B1}
	"b=SupportBean_B() where timer:within (2000 msec) or d=SupportBean_D() where timer:within(6001 msec)",                                  // S31 — @D1 {d:D1}
	"every b=SupportBean_B() where timer:within (2001 msec) and every d=SupportBean_D() where timer:within(6001 msec)",                     // S32 — 9 rows (cross product)
	"(every b=SupportBean_B) where timer:within (2000 msec) and every d=SupportBean_D() where timer:within(6001 msec)",                     // S33 — SILENT (LHS dies at 2000)
}

// patternGuardTimerWithinWHarness594CaseSpec pins the single case: Java
// execution identity (ordinal plus runtimeIndex), the observation text
// and the thirty-four byte-exact deployed statement texts pinned
// through case.epls.
type patternGuardTimerWithinWHarness594CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epls         []string
}

var patternGuardTimerWithinWHarness594CaseSpecs = []patternGuardTimerWithinWHarness594CaseSpec{
	{
		name:         "w-harness",
		ordinal:      0,
		runtimeIndex: 0,
		observation: "listener; external clock +1000ms per send; all 34 legs deploy in ONE step " +
			"before any send (harness deploy-all), then twelve advance-before-send " +
			"replays of A1,B1,C1,B2,A2,D1,E1,F1,D2,B3,G1,D3 (advanceTime precedes " +
			"sendEventBean so guard-expiry callbacks run at the advance instant — " +
			"the exclusive deadline kills a leg whose event lands exactly at " +
			"arm+period, and a child respawned inside an expiry callback is live " +
			"for the same-tick event): S0/S2/S5/S6 silent boundary pairs, S1 B1, " +
			"S3/S4 B3 (SODA 10.001d == 10001 msec), S7 B1 only (guarded every dies " +
			"at 2001), S8 B1+B2 (dies at 4001), S9/S10/S15 B1+B2+B3 (serial " +
			"every(guarded b) retry), S11/S12 accumulation @B1×1,@B2×2,@B3×4 " +
			"(nested every over guarded every), S13 B1+B2+B3, S14 B1+B2 (dies at " +
			"4001), S16 D1{b:B1,d:D1} (d arm at B1+0), S17/S20/S21/S24/S27/S28/S33 " +
			"silent, S18/S29 @D1 {B1,D1} + @B3 {B3,D2} (and-side expiry-respawn " +
			"retains D2 for B3's pairing), S19 @D1 {B1,D1} once, S22 @D1 {B2,D1} + " +
			"@D3 {B3,D3} (per-B serial every LHS), S23 seven rows (per-branch every " +
			"d retry; respawn at 6000 sees D1), S25 @D1 {B2,D1} + @D3 {B3,D3} " +
			"(guarded inner every dies permanently at expiry), S26 @D1 {B1,D1} + " +
			"@D3 {B3,D3} (every over guarded sequence), S30 @B1 {b:B1} (or wins on " +
			"first match), S31 @D1 {d:D1} (or survives the b side's expiry with b " +
			"unbound), S32 nine rows (every×every and cross product); per leg+trigger " +
			"the fires compare as a multiset like compareLists",
		epls: func() []string {
			epls := make([]string, 0, len(patternGuardTimerWithinWHarness594LegAtoms))
			for leg, atom := range patternGuardTimerWithinWHarness594LegAtoms {
				epls = append(epls, patternGuardTimerWithinWHarness594StatementText(leg, atom))
			}
			return epls
		}(),
	},
}

// patternGuardTimerWithinWHarness594StepPin mirrors the 592 step shape:
// deploy-all carries only the "all" statement label (leg texts pin
// through case.epls), send steps carry the fused advance-before-send
// instant plus the event payload and undeploy-all closes the case.
type patternGuardTimerWithinWHarness594StepPin struct {
	op        string
	statement string
	at        string
	eventType string
	payload   map[string]any
}

func patternGuardTimerWithinWHarness594DeployAllPin() patternGuardTimerWithinWHarness594StepPin {
	return patternGuardTimerWithinWHarness594StepPin{op: "deploy", statement: "all"}
}

func patternGuardTimerWithinWHarness594UndeployAllPin() patternGuardTimerWithinWHarness594StepPin {
	return patternGuardTimerWithinWHarness594StepPin{op: "undeploy-all"}
}

// patternGuardTimerWithinWHarness594CaseSteps pins the complete step
// sequence in harness order over the SAME mixed event set as the 592
// harness (A1@1000 .. D3@12000): ONE deploy-all step, then each
// event's fused advance-before-send step, then undeploy-all whose
// post-undeploy resend proves silence.
var patternGuardTimerWithinWHarness594CaseSteps = func() map[string][]patternGuardTimerWithinWHarness594StepPin {
	steps := make(map[string][]patternGuardTimerWithinWHarness594StepPin)
	for _, spec := range patternGuardTimerWithinWHarness594CaseSpecs {
		pins := []patternGuardTimerWithinWHarness594StepPin{
			patternGuardTimerWithinWHarness594DeployAllPin(),
		}
		for _, event := range patternTimerIntervalWHarness592EventSet {
			pins = append(pins, patternGuardTimerWithinWHarness594StepPin{
				op:        "send",
				at:        patternTimerIntervalWHarness592EventAt(event.at),
				eventType: event.eventType,
				payload:   map[string]any{"id": event.id},
			})
		}
		pins = append(pins, patternGuardTimerWithinWHarness594UndeployAllPin())
		steps[spec.name] = pins
	}
	return steps
}()

func loadPatternGuardTimerWithinWHarness594Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternGuardTimerWithinWHarness594ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternGuardTimerWithinWHarness594ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternGuardTimerWithinWHarness594ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternGuardTimerWithinWHarness594ID, err)
	}
	if err := requirePatternGuardTimerWithinWHarness594Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternGuardTimerWithinWHarness594ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternGuardTimerWithinWHarness594ID ||
		metadata.Description != patternGuardTimerWithinWHarness594Description ||
		metadata.JavaCommit != patternGuardTimerWithinWHarness594JavaCommit ||
		metadata.JavaSource != patternGuardTimerWithinWHarness594JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternGuardTimerWithinWHarness594ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternGuardTimerWithinWHarness594JavaSources},
		{metadata.JavaRuntimes, patternGuardTimerWithinWHarness594JavaRuntimeIDs},
		{metadata.JavaNames, patternGuardTimerWithinWHarness594JavaExecutions},
		{metadata.JavaStaticIDs, patternGuardTimerWithinWHarness594JavaStaticIDs},
		{metadata.JavaFlags, patternGuardTimerWithinWHarness594JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternGuardTimerWithinWHarness594ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternGuardTimerWithinWHarness594CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternGuardTimerWithinWHarness594ID, len(patternGuardTimerWithinWHarness594CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternGuardTimerWithinWHarness594Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternGuardTimerWithinWHarness594CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternGuardTimerWithinWHarness594JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternGuardTimerWithinWHarness594JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || !reflect.DeepEqual(definition.EPLs, spec.epls) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternGuardTimerWithinWHarness594ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternGuardTimerWithinWHarness594ID)
	}
	if err := validatePatternGuardTimerWithinWHarness594RawSteps(rawSteps); err != nil {
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

// validatePatternGuardTimerWithinWHarness594RawSteps pins the complete
// step sequence: op field whitelists per step kind plus positional
// comparison — the ONE deploy-all step (statement "all"), the twelve
// fused advance-before-send steps with pinned instants/payloads and
// the closing undeploy-all.
func validatePatternGuardTimerWithinWHarness594RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternGuardTimerWithinWHarness594CaseSpecs))
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
			if err := requirePatternGuardTimerWithinWHarness594Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternGuardTimerWithinWHarness594CaseSteps[marker.Case]; !ok {
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
		pins := patternGuardTimerWithinWHarness594CaseSteps[currentCase]
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
			if err := requirePatternGuardTimerWithinWHarness594Fields(object, "op", "case", "statement"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternGuardTimerWithinWHarness594Fields(object, "op", "case", "at", "eventType", "payload"); err != nil {
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
			if err := requirePatternGuardTimerWithinWHarness594Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternGuardTimerWithinWHarness594CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternGuardTimerWithinWHarness594CaseSpecs))
	}
	for index, spec := range patternGuardTimerWithinWHarness594CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternGuardTimerWithinWHarness594CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternGuardTimerWithinWHarness594CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternGuardTimerWithinWHarness594Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternGuardTimerWithinWHarness594CaseState carries the replay state
// like the 592 sibling: the single DeployPlans deployment of the 34
// legs, per-statement listener sequence counters and the delivery
// records.
type patternGuardTimerWithinWHarness594CaseState struct {
	spec        patternGuardTimerWithinWHarness594CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	deployment  *esper.Deployment
	statements  map[string]*esper.Statement
	sequence    map[string]uint64
	records     []compat.TraceRecord
	deployedAll bool
}

// runPatternGuardTimerWithinWHarness594Scenario replays the execution
// against a fresh engine like the Java oracle's fresh per-execution
// runtime — the thirty-four legs share ONE engine and ONE DeployPlans
// deployment because the harness deploys all statements before
// replaying the event set.
func runPatternGuardTimerWithinWHarness594Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternGuardTimerWithinWHarness594Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternGuardTimerWithinWHarness594CaseSpecs {
		records, err := runPatternGuardTimerWithinWHarness594Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternGuardTimerWithinWHarness594ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternGuardTimerWithinWHarness594Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternGuardTimerWithinWHarness594ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternGuardTimerWithinWHarness594ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternGuardTimerWithinWHarness594RawSteps(rawSteps)
}

func runPatternGuardTimerWithinWHarness594Case(ctx context.Context, scenario compat.Scenario, spec patternGuardTimerWithinWHarness594CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	for _, registration := range []struct {
		register func() error
	}{
		{func() error {
			_, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanA](env, patternTimerIntervalWHarness592BeanAType)
			return err
		}},
		{func() error {
			_, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanB](env, patternTimerIntervalWHarness592BeanBType)
			return err
		}},
		{func() error {
			_, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanC](env, patternTimerIntervalWHarness592BeanCType)
			return err
		}},
		{func() error {
			_, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanD](env, patternTimerIntervalWHarness592BeanDType)
			return err
		}},
		{func() error {
			_, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanE](env, patternTimerIntervalWHarness592BeanEType)
			return err
		}},
		{func() error {
			_, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanF](env, patternTimerIntervalWHarness592BeanFType)
			return err
		}},
		{func() error {
			_, err := esper.RegisterStruct[patternTimerIntervalWHarness592BeanG](env, patternTimerIntervalWHarness592BeanGType)
			return err
		}},
	} {
		if err := registration.register(); err != nil {
			return nil, err
		}
	}
	state := &patternGuardTimerWithinWHarness594CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternGuardTimerWithinWHarness594JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			// The fused step performs the harness's advanceTime BEFORE
			// sendEventBean: guard-expiry callbacks inside the advance
			// run at the advance instant (the exclusive deadline kills
			// a leg whose event lands exactly at arm+period) and a
			// child respawned inside an expiry callback is live for
			// the same-tick event (S23's respawn-at-6000 sees D1).
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse send at %q: %w", patternGuardTimerWithinWHarness594ID, step.At, err)
			}
			if err := state.engine.AdvanceTime(ctx, at); err != nil {
				return nil, err
			}
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
			return nil, fmt.Errorf("%s: unsupported step op %q", patternGuardTimerWithinWHarness594ID, step.Op)
		}
	}
	return state.records, nil
}

// patternGuardTimerWithinWHarness594Tag pins one projected select-*
// column: a single bean fragment (PatternEvent); a tag left unbound by
// a losing or-side projects the null marker (S31's {d:D1}).
type patternGuardTimerWithinWHarness594Tag struct {
	name string
}

// patternGuardTimerWithinWHarness594LegSpec pins one leg's fluent
// build plus its select-* fragment columns.
type patternGuardTimerWithinWHarness594LegSpec struct {
	build func(patternTimerIntervalWHarness592Streams) esper.PatternStream
	tags  []patternGuardTimerWithinWHarness594Tag
}

// patternGuardTimerWithinWHarness594LegSpecs returns the 34 leg
// builders in case-list order over the same shared streams bundle the
// 592 harness uses. The fluent mapping follows the Java grammar
// (guardPostFix binds tightest):
//
//	X.Within(d)                  = `X where timer:within(d)`
//	X.Every().Within(d)          = `(every X) where timer:within(d)`
//	X.Within(d).Every()          = `every (X where timer:within(d))`
//	X.Every().Within(d).Every()  = `every ((every X) where
//	                               timer:within(d))`
//	b.Then(d.Within(d))          = `b -> d where timer:within(d)` —
//	                               the guard scopes the RHS only.
//
// S3's SODA `10.001d` double-literal is the same 10001ms guard as S4.
func patternGuardTimerWithinWHarness594LegSpecs() []patternGuardTimerWithinWHarness594LegSpec {
	tag := func(name string) patternGuardTimerWithinWHarness594Tag {
		return patternGuardTimerWithinWHarness594Tag{name: name}
	}
	b := func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
		return esper.PatternFrom(s.b, "b", s.always)
	}
	bB1 := func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
		return esper.PatternFrom(s.b, "b", s.idB("B1"))
	}
	bB3 := func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
		return esper.PatternFrom(s.b, "b", s.idB("B3"))
	}
	d := func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
		return esper.PatternFrom(s.d, "d", s.always)
	}
	return []patternGuardTimerWithinWHarness594LegSpec{
		// S0 `b=B(id='B1') where timer:within(2 sec)` — deadline
		// 2000 lands exactly at the B1 instant: the quit callback
		// inside the advance runs first; silent.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return bB1(s).Within(2 * time.Second)
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S1 `... within(2001 msec)` — survives the B1 send.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return bB1(s).Within(2001 * time.Millisecond)
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S2 `... within(1999 msec)` — expires inside the B1 advance.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return bB1(s).Within(1999 * time.Millisecond)
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S3 SODA leg `b=B(id="B3") where timer:within(10.001d)` —
		// double literal 10.001 seconds = 10001ms.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return bB3(s).Within(10001 * time.Millisecond)
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S4 `... within(10001 msec)` — twin of S3.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return bB3(s).Within(10001 * time.Millisecond)
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S5 `... within(10 sec)` — deadline lands exactly at B3.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return bB3(s).Within(10 * time.Second)
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S6 `... within(9.999)` — expires inside the B3 advance.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return bB3(s).Within(9999 * time.Millisecond)
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S7 `(every b=B) where within(2.001)` — the guard wraps the
		// every: B1 fires once, the whole pattern dies at 2001.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Every().Within(2001 * time.Millisecond)
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S8 `(every b=B) where within(4.001)` — B1 and B2 fire,
		// the every dies at 4001 before B3.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Every().Within(4001 * time.Millisecond)
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S9 `every b=B where within(2.001)` — every's operand
		// includes the guard: serial retry, all three Bs fire.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(2001 * time.Millisecond).Every()
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S10 `every (b=B where within(2001 msec))` — same serial
		// retry shape as S9.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(2001 * time.Millisecond).Every()
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S11 `every ((every b=B) where within(2.001))` — the inner
		// every's filter child never quits so the guarded instance
		// stays live until its deadline while the outer every
		// spawns a sibling per match: B1×1, B2×2, B3×4.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Every().Within(2001 * time.Millisecond).Every()
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S12 `every ((every b=B) where within(6.001))` — wider
		// window, same accumulation multiset.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Every().Within(6001 * time.Millisecond).Every()
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S13 `(every b=B) where within(11.001)` — the guard
		// outlives B3; all three fire.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Every().Within(11001 * time.Millisecond)
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S14 `(every b=B) where within(4001 milliseconds)` — B1
		// and B2 fire; the every dies at 4001 before B3.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Every().Within(4001 * time.Millisecond)
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S15 `every (b=B) where within(6.001)` — the guard binds
		// inside every's operand (every(guarded b), NOT
		// guarded(every b)); serial retry, all three Bs fire.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(6001 * time.Millisecond).Every()
		}, []patternGuardTimerWithinWHarness594Tag{tag("b")}},
		// S16 `b=B -> d=D where within(4001 milliseconds)` — the
		// guard scopes the d RHS only: armed when b completes at
		// 2000, deadline 6001 — D1 fires {b:B1,d:D1}.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Then(d(s).Within(4001 * time.Millisecond))
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S17 `b=B() -> d=D() where within(4 sec)` — deadline 6000
		// lands exactly at the D1 instant; the guard wins; silent.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Then(d(s).Within(4 * time.Second))
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S18 `every (b where within(4.001) and d where
		// within(6.001))` — serial every over the guarded and:
		// instance 1 fires @D1 {B1,D1}; instance 2's b-side expires
		// inside the E1 advance (deadline 8001, no B until B3);
		// instance 3 armed at 9000 retains d:D2 and pairs it with
		// B3: @B3 {B3,D2}.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(4001 * time.Millisecond).And(d(s).Within(6001 * time.Millisecond)).Every()
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S19 `b where within(2001) and d where within(6001)` —
		// B1 holds the left side, D1 completes the and.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(2001 * time.Millisecond).And(d(s).Within(6001 * time.Millisecond))
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S20 — d deadline 6000 lands exactly at the D1 instant;
		// the and dies; silent.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(2001 * time.Millisecond).And(d(s).Within(6000 * time.Millisecond))
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S21 — b deadline 2000 lands exactly at the B1 instant;
		// the and dies at once; silent.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(2000 * time.Millisecond).And(d(s).Within(6001 * time.Millisecond))
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S22 `every b=B -> d=D where within(4000 msec)` —
		// per-B serial instances: B1's instance expires at the
		// exact D1 instant (deadline 6000), B2's pairs D1, B3's
		// pairs D3: @D1 {B2,D1}, @D3 {B3,D3}.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Every().Then(d(s).Within(4000 * time.Millisecond))
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S23 `every b=B() -> every d=D where within(4000 msec)` —
		// each B's branch runs a perpetual every(guarded d): the
		// first d instance expires at the exact D1 instant then
		// respawns INSIDE the advance at 6000 and sees D1 — I1 and
		// I2 both match D1/D2/D3, I3 (armed at 10000) matches D3:
		// seven rows.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Every().Then(d(s).Within(4000 * time.Millisecond).Every())
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S24 `b -> d where within(3999)` — deadline 5999 expires
		// inside the D1 advance before D1; silent.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Then(d(s).Within(3999 * time.Millisecond))
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S25 `every b=B() -> (every d=D) where within(2001)` —
		// the guard wraps the inner every, which cannot retry:
		// B1's every-d dies at 4001 with no D yet, B2's sees D1
		// then dies at 6001 (D2 misses), B3's sees D3.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Every().Then(d(s).Every().Within(2001 * time.Millisecond))
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S26 `every (b=B() -> d=D()) where within(6001 msec)` —
		// every over the guarded sequence: instance 1 fires @D1
		// {B1,D1}; the respawn armed at 6000 waits for b — B2 at
		// 4000 already passed, B3 completes at 10000 and D3 pairs
		// at 12000 (6000+6001 > 12001? deadline 12001 survives):
		// @D3 {B3,D3}.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Then(d(s)).Within(6001 * time.Millisecond).Every()
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S27 `b within(2000) or d within(6000)` — both deadlines
		// land exactly at the B1/D1 instants; both sides die;
		// silent.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(2000 * time.Millisecond).Or(d(s).Within(6000 * time.Millisecond))
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S28 `(b within(2000) or d within(6000)) within(1999)` —
		// the outer guard expires inside the A1 advance before
		// either side can match; silent.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(2000 * time.Millisecond).Or(d(s).Within(6000 * time.Millisecond)).Within(1999 * time.Millisecond)
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S29 `every (b within(2001) and d within(6001))` — same
		// shape as S18 with tighter windows: @D1 {B1,D1}, then the
		// respawned instance retains D2 for B3: @B3 {B3,D2}.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(2001 * time.Millisecond).And(d(s).Within(6001 * time.Millisecond)).Every()
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S30 `b within(2001) or d within(6001)` — B1 completes
		// the or on the b side at 2000 and kills d: @B1 {b:B1}.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(2001 * time.Millisecond).Or(d(s).Within(6001 * time.Millisecond))
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S31 `b within(2000) or d within(6001)` — the b side
		// expires at the exact B1 instant but the or survives on
		// d: D1 completes with b unbound — the row carries only
		// the d tag.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(2000 * time.Millisecond).Or(d(s).Within(6001 * time.Millisecond))
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S32 `every b within(2001) and every d within(6001)` —
		// and of two perpetual guarded-every retries: each and
		// completion keeps the other side's accumulated match, so
		// every later arrival pairs with all retained B/D
		// matches — nine rows.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Within(2001 * time.Millisecond).Every().And(
				d(s).Within(6001 * time.Millisecond).Every())
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
		// S33 `(every b) within(2000) and every d within(6001)` —
		// the left guarded-every expires at the exact B1 instant
		// and kills the whole and; silent.
		{func(s patternTimerIntervalWHarness592Streams) esper.PatternStream {
			return b(s).Every().Within(2000 * time.Millisecond).And(
				d(s).Within(6001 * time.Millisecond).Every())
		}, []patternGuardTimerWithinWHarness594Tag{tag("b"), tag("d")}},
	}
}

// deployAll expands the ONE deploy-all step into one DeployPlans call
// carrying all 34 plans in case order — statement S<i> deploys leg i —
// then attaches one listener per statement, exactly like the 592
// sibling.
func (s *patternGuardTimerWithinWHarness594CaseState) deployAll(ctx context.Context, step compat.Step) error {
	if s.deployedAll || step.Statement != "all" {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternGuardTimerWithinWHarness594ID, s.spec.name)
	}
	legs := patternGuardTimerWithinWHarness594LegSpecs()
	streams := newPatternTimerIntervalWHarness592Streams(s.env)
	if len(legs) != len(patternGuardTimerWithinWHarness594LegAtoms) {
		return fmt.Errorf("%s: %d leg builders, want %d", patternGuardTimerWithinWHarness594ID,
			len(legs), len(patternGuardTimerWithinWHarness594LegAtoms))
	}
	plans := make([]esper.Plan, 0, len(legs))
	for leg, spec := range legs {
		selections := make([]esper.Selection, 0, len(spec.tags))
		for _, tag := range spec.tags {
			selections = append(selections, esper.Alias(tag.name, esper.PatternEvent(tag.name)))
		}
		plan, err := s.env.Build(spec.build(streams).Select(selections...).Query(
			esper.StatementName(patternGuardTimerWithinWHarness594LegName(leg))))
		if err != nil {
			return fmt.Errorf("%s: build leg S%d %q: %w", patternGuardTimerWithinWHarness594ID, leg,
				patternGuardTimerWithinWHarness594LegAtoms[leg], err)
		}
		plans = append(plans, plan)
	}
	deployment, err := s.engine.DeployPlans(ctx, plans)
	if err != nil {
		return fmt.Errorf("%s: deploy-all: %w", patternGuardTimerWithinWHarness594ID, err)
	}
	statements := deployment.Statements()
	if len(statements) != len(legs) {
		return fmt.Errorf("%s: deploy-all produced %d statements, want %d",
			patternGuardTimerWithinWHarness594ID, len(statements), len(legs))
	}
	s.deployment = deployment
	s.deployedAll = true
	for leg, statement := range statements {
		name := patternGuardTimerWithinWHarness594LegName(leg)
		if statement.Name() != name {
			return fmt.Errorf("%s: statement %d is %q, want %q",
				patternGuardTimerWithinWHarness594ID, leg, statement.Name(), name)
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
