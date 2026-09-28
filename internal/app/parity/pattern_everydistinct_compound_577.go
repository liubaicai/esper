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

// Parity coverage for PatternOperatorEveryDistinct ords 8-12: the
// compound-operator quintet — every-distinct as the outer operator over
// and/or/and-not/followed-by roots (ords 8-11) plus a left-nested
// every-distinct inside followed-by (ord 12) — over SupportBean.
//
// Each execution runs the identical send sequence twice: first with the
// no-expiry every-distinct form, then with the timed-expiry form
// (1 hour for ords 8-11, 2 hours 1 minute for ord 12), with undeployAll
// between the legs. No clock ops exist anywhere in these executions, so
// the timed key never expires and both legs share the identical expected
// sequence (ord 3 OverFilter precedent).
//
// Covered executions (all variant collection:executions(), flags []):
//   - ord 8 PatternEveryDistinctOverAnd java-runtime-fa2df3aeba01a5eda1d2
//     (cases over-and / over-and-expiry): the composite key is the
//     (a.intPrimitive, b.intPrimitive) pair over a='A%' and b='B%'.
//     A1(1) is silent; B1(10) completes key 1+10 and fires {A1,B1};
//     A2(1)+B2(10)+A3(2) stay silent (dup 1+10); B3(10) fires {A3,B3};
//     A4(1)+B4(20) fires {A4,B4}; A5(2)+B5(10) silent (dup 2+10);
//     A6(2)+B6(20) fires {A6,B6}; A7(2)+B7(20) silent (dup 2+20). 4 fires.
//   - ord 9 PatternEveryDistinctOverOr java-runtime-6aab27f9a29d3ad7188f
//     (cases over-or / over-or-expiry): the single expression key
//     coalesce(a.intPrimitive,0)+coalesce(b.intPrimitive,0) sees both
//     branches. A1(1) fires {A1,null} key 1; B1(2) fires {null,B1} key 2;
//     B2(1)/A2(2)/A3(2)/B3(1) are all swallowed (keys 1 and 2 seen);
//     B4(3) and B5(4) fire; B6(3)/A4(3)/A5(4) stay silent. 4 fires.
//   - ord 10 PatternEveryDistinctOverNot java-runtime-c23dddb4d0d001809ffd
//     (cases over-not / over-not-expiry): key a.intPrimitive over
//     a='A%' and not 'B%'. A1(1)/A3(2) fire; A2(1) is swallowed; B1(1)
//     falsifies the attempt and EvalEveryDistinctStateNode.evaluateFalse
//     respawns WITHOUT copying the key set, so A4(1) fires again on the
//     previously seen key 1 — the falsification-respawn discriminant;
//     A5(1) stays silent. 3 fires.
//   - ord 11 PatternEveryDistinctOverFollowedBy
//     java-runtime-c28f82ea566795672292 (cases over-followed-by /
//     over-followed-by-expiry): the key sums a.intPrimitive+b.intPrimitive.
//     A1(1)->B1(1) fires key 2; A2(1)->B2(1) and A3(10)->B3(-8) are
//     swallowed (both sum to 2); A4(2) waits alone, B4(1) completes
//     {A4,B4} key 3; A5(3)->B5(0) silent (dup 3). 2 fires.
//   - ord 12 PatternEveryDistinctWithinFollowedBy
//     java-runtime-58fcea53e80425896c2d (cases within-followed-by /
//     within-followed-by-expiry): every-distinct nests on the left of
//     followed-by, so each fresh a.intPrimitive key spawns ONE waiting
//     branch that correlates b on intPrimitive=a.intPrimitive — the
//     per-key-branch discriminant. B1(0) matches nothing; B2(1) fires
//     {A1,B2}; A2(2)/A3(3) spawn branches while A4(1) is dup-swallowed;
//     B3(3) fires {A3,B3}; B4(1) is silent (the A1 branch already
//     completed); B5(2) fires {A2,B5}; A5(2)+B6(2) silent (dup key /
//     consumed branch); A6(4)+B7(4) fires {A6,B7}. 4 fires.
//
// Byte-exact EPL pins keep the Java source verbatim: all ten legs use
// lowercase @name('s0') and select *; ord 12's expiry leg carries the
// `2 hours 1 minute` duration literal. Java milestone() savepoints
// restore identical state for these non-contextual executions and are
// omitted (573/575/576 precedent).
//
// Siblings excluded from this slice (manifest dispositions are
// main-agent owned): ord 3 (SODA eplToModelCompileDeploy tail), ord 14
// (compile-error-only), ord 15 (calendar-month expiry), ords 4/6
// (repeat pair with tag-array projections), ord 13 (dual-distinct
// multi-fire), ord 16 (int-array bean) are follow-on slices; ords 5/7
// already live under case.pattern-every.
const patternEveryDistinctCompound577ID = "pattern-everydistinct-compound-577"

const patternEveryDistinctCompound577Description = "PatternOperatorEveryDistinct ords 8-12 — every-distinct over compound pattern operators (and/or/and-not/followed-by roots plus left-nested distinct inside followed-by) over SupportBean, each replayed twice: no-expiry leg and timed-expiry leg (1 hour; 2 hours 1 minute for within-followed-by) with undeployAll between and no clock, so each leg pair shares the identical expected sequence (ord 3 OverFilter precedent). over-and (PatternEveryDistinctOverAnd, ord 8) keys on the (a.intPrimitive, b.intPrimitive) pair: A1(1) silent, B1(10) fires {A1,B1}, A2(1)+B2(10)+A3(2) silent (dup key 1+10), B3(10) fires {A3,B3}, A4(1)+B4(20) fires {A4,B4}, A5(2)+B5(10) silent (dup 2+10), A6(2)+B6(20) fires {A6,B6}, A7(2)+B7(20) silent — 4 fires. over-or (PatternEveryDistinctOverOr, ord 9) keys on coalesce(a.intPrimitive,0)+coalesce(b.intPrimitive,0): A1(1) fires {A1,null} key 1, B1(2) fires {null,B1} key 2, B2(1)+A2(2)+A3(2)+B3(1) silent (keys 1 and 2 seen), B4(3) and B5(4) fire, B6(3)+A4(3)+A5(4) silent — 4 fires. over-not (PatternEveryDistinctOverNot, ord 10) keys on a.intPrimitive: A1(1)/A3(2) fire, A2(1) swallowed, B1(1) falsifies the attempt and the respawned attempt carries an EMPTY key set (EvalEveryDistinctStateNode evaluateFalse) so A4(1) fires again on the previously seen key, A5(1) silent — 3 fires. over-followed-by (PatternEveryDistinctOverFollowedBy, ord 11) keys on a.intPrimitive+b.intPrimitive: A1(1)->B1(1) fires key 2, A2(1)->B2(1) and A3(10)->B3(-8) silent (both sum to 2), A4(2)->B4(1) fires key 3, A5(3)->B5(0) silent (dup 3) — 2 fires. within-followed-by (PatternEveryDistinctWithinFollowedBy, ord 12) nests every-distinct on the left of followed-by so each fresh a.intPrimitive key spawns one waiting branch correlating b on intPrimitive=a.intPrimitive: B1(0) matches nothing, B2(1) fires {A1,B2}, A4(1) dup-swallowed, B3(3) fires {A3,B3}, B4(1) silent (A1 branch consumed), B5(2) fires {A2,B5}, A5(2)+B6(2) silent, A6(4)+B7(4) fires — 4 fires. Java milestone() savepoints are omitted (they restore identical state for these non-contextual executions)."

const patternEveryDistinctCompound577JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternEveryDistinctCompound577JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java"

// Byte-exact EPL pins (PatternOperatorEveryDistinct.java: the ord 8-12
// run methods each deploy the no-expiry expression first, then the
// timed-expiry expression, undeploying between legs).
const (
	patternEveryDistinctCompound577OverAndEPL                = "@name('s0') select * from pattern [every-distinct(a.intPrimitive, b.intPrimitive) (a=SupportBean(theString like 'A%') and b=SupportBean(theString like 'B%'))]"
	patternEveryDistinctCompound577OverAndExpiryEPL          = "@name('s0') select * from pattern [every-distinct(a.intPrimitive, b.intPrimitive, 1 hour) (a=SupportBean(theString like 'A%') and b=SupportBean(theString like 'B%'))]"
	patternEveryDistinctCompound577OverOrEPL                 = "@name('s0') select * from pattern [every-distinct(coalesce(a.intPrimitive, 0) + coalesce(b.intPrimitive, 0)) (a=SupportBean(theString like 'A%') or b=SupportBean(theString like 'B%'))]"
	patternEveryDistinctCompound577OverOrExpiryEPL           = "@name('s0') select * from pattern [every-distinct(coalesce(a.intPrimitive, 0) + coalesce(b.intPrimitive, 0), 1 hour) (a=SupportBean(theString like 'A%') or b=SupportBean(theString like 'B%'))]"
	patternEveryDistinctCompound577OverNotEPL                = "@name('s0') select * from pattern [every-distinct(a.intPrimitive) (a=SupportBean(theString like 'A%') and not SupportBean(theString like 'B%'))]"
	patternEveryDistinctCompound577OverNotExpiryEPL          = "@name('s0') select * from pattern [every-distinct(a.intPrimitive, 1 hour) (a=SupportBean(theString like 'A%') and not SupportBean(theString like 'B%'))]"
	patternEveryDistinctCompound577OverFollowedByEPL         = "@name('s0') select * from pattern [every-distinct(a.intPrimitive + b.intPrimitive) (a=SupportBean(theString like 'A%') -> b=SupportBean(theString like 'B%'))]"
	patternEveryDistinctCompound577OverFollowedByExpiryEPL   = "@name('s0') select * from pattern [every-distinct(a.intPrimitive + b.intPrimitive, 1 hour) (a=SupportBean(theString like 'A%') -> b=SupportBean(theString like 'B%'))]"
	patternEveryDistinctCompound577WithinFollowedByEPL       = "@name('s0') select * from pattern [(every-distinct(a.intPrimitive) a=SupportBean(theString like 'A%')) -> b=SupportBean(intPrimitive=a.intPrimitive)]"
	patternEveryDistinctCompound577WithinFollowedByExpiryEPL = "@name('s0') select * from pattern [(every-distinct(a.intPrimitive, 2 hours 1 minute) a=SupportBean(theString like 'A%')) -> b=SupportBean(intPrimitive=a.intPrimitive)]"

	patternEveryDistinctCompound577SupportBeanEvent = "SupportBean"
)

var (
	// One javaRuntimes row per Java execution (legs share their ord's
	// runtimeId — each leg is the same execution replayed through
	// runX(env, expression, milestone), 574 runtimeIndex precedent).
	patternEveryDistinctCompound577JavaRuntimeIDs = []string{
		"java-runtime-fa2df3aeba01a5eda1d2",
		"java-runtime-6aab27f9a29d3ad7188f",
		"java-runtime-c23dddb4d0d001809ffd",
		"java-runtime-c28f82ea566795672292",
		"java-runtime-58fcea53e80425896c2d",
	}
	patternEveryDistinctCompound577JavaSources = []string{
		patternEveryDistinctCompound577JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
	}
	patternEveryDistinctCompound577JavaExecutions = []string{
		"PatternEveryDistinctOverAnd",
		"PatternEveryDistinctOverOr",
		"PatternEveryDistinctOverNot",
		"PatternEveryDistinctOverFollowedBy",
		"PatternEveryDistinctWithinFollowedBy",
	}
	// Deduplicated inventory id: the five runtime rows share static id
	// java-073091a0b42ca8dd974f with the other PatternOperatorEveryDistinct
	// executions, pinned once per runtimeId row.
	patternEveryDistinctCompound577JavaStaticIDs = []string{
		"java-073091a0b42ca8dd974f",
		"java-073091a0b42ca8dd974f",
		"java-073091a0b42ca8dd974f",
		"java-073091a0b42ca8dd974f",
		"java-073091a0b42ca8dd974f",
	}
	patternEveryDistinctCompound577JavaFlags = []string{}
)

// patternEveryDistinctCompound577Bean mirrors the SupportBean properties
// the executions use: theString (the A%/B% leg filter and the a.theString/
// b.theString the Java assertions pin) and intPrimitive (every distinct
// key expression plus ord 12's correlated b filter).
type patternEveryDistinctCompound577Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// patternEveryDistinctCompound577CaseSpec pins one case: Java execution
// identity (ordinal plus runtimeIndex into the shared per-ord arrays),
// the observation text and the byte-exact EPL script. select * projects
// every bound tag's bean row.
type patternEveryDistinctCompound577CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epl          string
}

var patternEveryDistinctCompound577CaseSpecs = []patternEveryDistinctCompound577CaseSpec{
	{
		name:         "over-and",
		ordinal:      8,
		runtimeIndex: 0,
		observation:  "listener; no clock; composite distinct key (a.intPrimitive, b.intPrimitive) over a='A%' and b='B%': A1(1) silent, B1(10) fires {A1,B1} key 1+10, A2(1)+B2(10)+A3(2) silent (dup 1+10), B3(10) fires {A3,B3} key 2+10, A4(1)+B4(20) fires {A4,B4} key 1+20, A5(2)+B5(10) silent (dup 2+10), A6(2)+B6(20) fires {A6,B6} key 2+20, A7(2)+B7(20) silent (dup 2+20)",
		epl:          patternEveryDistinctCompound577OverAndEPL,
	},
	{
		name:         "over-and-expiry",
		ordinal:      8,
		runtimeIndex: 0,
		observation:  "listener; no clock; `, 1 hour` per-key expiry leg — the clock never advances so the key never expires and the identical sequence replays: A1(1) silent, B1(10) fires {A1,B1}, A2(1)+B2(10)+A3(2) silent, B3(10) fires {A3,B3}, A4(1)+B4(20) fires {A4,B4}, A5(2)+B5(10) silent, A6(2)+B6(20) fires {A6,B6}, A7(2)+B7(20) silent",
		epl:          patternEveryDistinctCompound577OverAndExpiryEPL,
	},
	{
		name:         "over-or",
		ordinal:      9,
		runtimeIndex: 1,
		observation:  "listener; no clock; single expression key coalesce(a.intPrimitive,0)+coalesce(b.intPrimitive,0) over a='A%' or b='B%': A1(1) fires {A1,null} key 1, B1(2) fires {null,B1} key 2, B2(1)+A2(2)+A3(2)+B3(1) silent (keys 1 and 2 seen), B4(3) fires {null,B4} key 3, B5(4) fires {null,B5} key 4, B6(3)+A4(3)+A5(4) silent",
		epl:          patternEveryDistinctCompound577OverOrEPL,
	},
	{
		name:         "over-or-expiry",
		ordinal:      9,
		runtimeIndex: 1,
		observation:  "listener; no clock; `, 1 hour` per-key expiry leg — the clock never advances so the key never expires and the identical sequence replays: A1(1) fires {A1,null}, B1(2) fires {null,B1}, B2(1)+A2(2)+A3(2)+B3(1) silent, B4(3) fires {null,B4}, B5(4) fires {null,B5}, B6(3)+A4(3)+A5(4) silent",
		epl:          patternEveryDistinctCompound577OverOrExpiryEPL,
	},
	{
		name:         "over-not",
		ordinal:      10,
		runtimeIndex: 2,
		observation:  "listener; no clock; key a.intPrimitive over a='A%' and not 'B%': A1(1) fires, A2(1) swallowed (key 1 seen), A3(2) fires, B1(1) falsifies the attempt and the respawned attempt carries an EMPTY key set so A4(1) fires again on previously-seen key 1, A5(1) silent",
		epl:          patternEveryDistinctCompound577OverNotEPL,
	},
	{
		name:         "over-not-expiry",
		ordinal:      10,
		runtimeIndex: 2,
		observation:  "listener; no clock; `, 1 hour` per-key expiry leg — the clock never advances so the key never expires and the identical sequence replays: A1(1) fires, A2(1) swallowed, A3(2) fires, B1(1) falsifies and resets the key set so A4(1) fires again, A5(1) silent",
		epl:          patternEveryDistinctCompound577OverNotExpiryEPL,
	},
	{
		name:         "over-followed-by",
		ordinal:      11,
		runtimeIndex: 3,
		observation:  "listener; no clock; expression key a.intPrimitive+b.intPrimitive over a='A%' -> b='B%': A1(1)->B1(1) fires {A1,B1} key 2, A2(1)->B2(1) silent (dup 2), A3(10)->B3(-8) silent (sum 2 again), A4(2) waits alone then B4(1) fires {A4,B4} key 3, A5(3)->B5(0) silent (dup 3)",
		epl:          patternEveryDistinctCompound577OverFollowedByEPL,
	},
	{
		name:         "over-followed-by-expiry",
		ordinal:      11,
		runtimeIndex: 3,
		observation:  "listener; no clock; `, 1 hour` per-key expiry leg — the clock never advances so the key never expires and the identical sequence replays: A1(1)->B1(1) fires {A1,B1}, A2(1)->B2(1) and A3(10)->B3(-8) silent (dup key 2), A4(2)+B4(1) fires {A4,B4}, A5(3)->B5(0) silent",
		epl:          patternEveryDistinctCompound577OverFollowedByExpiryEPL,
	},
	{
		name:         "within-followed-by",
		ordinal:      12,
		runtimeIndex: 4,
		observation:  "listener; no clock; left-nested every-distinct spawns one waiting branch per fresh a.intPrimitive key, each correlating b on intPrimitive=a.intPrimitive: A1(1) then B1(0) matches nothing, B2(1) fires {A1,B2}, A2(2)/A3(3) spawn branches and A4(1) is dup-swallowed, B3(3) fires {A3,B3}, B4(1) silent (A1 branch consumed), B5(2) fires {A2,B5}, A5(2)+B6(2) silent, A6(4)+B7(4) fires {A6,B7}",
		epl:          patternEveryDistinctCompound577WithinFollowedByEPL,
	},
	{
		name:         "within-followed-by-expiry",
		ordinal:      12,
		runtimeIndex: 4,
		observation:  "listener; no clock; `, 2 hours 1 minute` per-key expiry leg — the clock never advances so the key never expires and the identical sequence replays: B1(0) matches nothing, B2(1) fires {A1,B2}, A4(1) dup-swallowed, B3(3) fires {A3,B3}, B4(1) silent, B5(2) fires {A2,B5}, A5(2)+B6(2) silent, A6(4)+B7(4) fires {A6,B7}",
		epl:          patternEveryDistinctCompound577WithinFollowedByExpiryEPL,
	},
}

// patternEveryDistinctCompound577StepPin pins one scenario step's shape.
type patternEveryDistinctCompound577StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	at        string
}

func patternEveryDistinctCompound577DeployPin(statement, epl string) patternEveryDistinctCompound577StepPin {
	return patternEveryDistinctCompound577StepPin{op: "deploy", statement: statement, epl: epl}
}

func patternEveryDistinctCompound577SendPin(theString string, intPrimitive int) patternEveryDistinctCompound577StepPin {
	return patternEveryDistinctCompound577StepPin{
		op:        "send",
		eventType: patternEveryDistinctCompound577SupportBeanEvent,
		payload:   map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)},
	}
}

func patternEveryDistinctCompound577UndeployAllPin() patternEveryDistinctCompound577StepPin {
	return patternEveryDistinctCompound577StepPin{op: "undeploy-all"}
}

// patternEveryDistinctCompound577OrdSends pins the send payload order per
// ord; both expiry legs replay the identical send sequence.
var patternEveryDistinctCompound577OrdSends = map[string][][2]any{
	// ord 8: A1(1), B1(10), A2(1), B2(10), A3(2), B3(10), A4(1), B4(20),
	// A5(2), B5(10), A6(2), B6(20), A7(2), B7(20).
	"over-and": {
		{"A1", 1}, {"B1", 10}, {"A2", 1}, {"B2", 10}, {"A3", 2},
		{"B3", 10}, {"A4", 1}, {"B4", 20}, {"A5", 2}, {"B5", 10},
		{"A6", 2}, {"B6", 20}, {"A7", 2}, {"B7", 20},
	},
	// ord 9: A1(1), B1(2), B2(1), A2(2), A3(2), B3(1), B4(3), B5(4),
	// B6(3), A4(3), A5(4).
	"over-or": {
		{"A1", 1}, {"B1", 2}, {"B2", 1}, {"A2", 2}, {"A3", 2},
		{"B3", 1}, {"B4", 3}, {"B5", 4}, {"B6", 3}, {"A4", 3}, {"A5", 4},
	},
	// ord 10: A1(1), A2(1), A3(2), B1(1), A4(1), A5(1).
	"over-not": {
		{"A1", 1}, {"A2", 1}, {"A3", 2}, {"B1", 1}, {"A4", 1}, {"A5", 1},
	},
	// ord 11: A1(1), B1(1), A2(1), B2(1), A3(10), B3(-8), A4(2), B4(1),
	// A5(3), B5(0).
	"over-followed-by": {
		{"A1", 1}, {"B1", 1}, {"A2", 1}, {"B2", 1}, {"A3", 10},
		{"B3", -8}, {"A4", 2}, {"B4", 1}, {"A5", 3}, {"B5", 0},
	},
	// ord 12: A1(1), B1(0), B2(1), A2(2), A3(3), A4(1), B3(3), B4(1),
	// B5(2), A5(2), B6(2), A6(4), B7(4).
	"within-followed-by": {
		{"A1", 1}, {"B1", 0}, {"B2", 1}, {"A2", 2}, {"A3", 3},
		{"A4", 1}, {"B3", 3}, {"B4", 1}, {"B5", 2}, {"A5", 2},
		{"B6", 2}, {"A6", 4}, {"B7", 4},
	},
}

// patternEveryDistinctCompound577CaseSteps pins the complete step
// sequence per case in Java source order (milestones omitted —
// regression-harness savepoints restoring identical state). No case
// carries clock ops: ords 8-12 never call sendTimeEvent/advanceTime, so
// each timed-expiry leg replays the identical send sequence as its
// no-expiry sibling.
var patternEveryDistinctCompound577CaseSteps = func() map[string][]patternEveryDistinctCompound577StepPin {
	steps := make(map[string][]patternEveryDistinctCompound577StepPin)
	for _, spec := range patternEveryDistinctCompound577CaseSpecs {
		ord := spec.name
		if len(ord) > len("-expiry") && ord[len(ord)-len("-expiry"):] == "-expiry" {
			ord = ord[:len(ord)-len("-expiry")]
		}
		pins := []patternEveryDistinctCompound577StepPin{
			patternEveryDistinctCompound577DeployPin("s0", spec.epl),
		}
		for _, send := range patternEveryDistinctCompound577OrdSends[ord] {
			pins = append(pins, patternEveryDistinctCompound577SendPin(
				send[0].(string), send[1].(int)))
		}
		pins = append(pins, patternEveryDistinctCompound577UndeployAllPin())
		steps[spec.name] = pins
	}
	return steps
}()

func loadPatternEveryDistinctCompound577Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternEveryDistinctCompound577ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternEveryDistinctCompound577ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternEveryDistinctCompound577ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternEveryDistinctCompound577ID, err)
	}
	if err := requirePatternEveryDistinctCompound577Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternEveryDistinctCompound577ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternEveryDistinctCompound577ID ||
		metadata.Description != patternEveryDistinctCompound577Description ||
		metadata.JavaCommit != patternEveryDistinctCompound577JavaCommit ||
		metadata.JavaSource != patternEveryDistinctCompound577JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternEveryDistinctCompound577ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternEveryDistinctCompound577JavaSources},
		{metadata.JavaRuntimes, patternEveryDistinctCompound577JavaRuntimeIDs},
		{metadata.JavaNames, patternEveryDistinctCompound577JavaExecutions},
		{metadata.JavaStaticIDs, patternEveryDistinctCompound577JavaStaticIDs},
		{metadata.JavaFlags, patternEveryDistinctCompound577JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternEveryDistinctCompound577ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternEveryDistinctCompound577CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternEveryDistinctCompound577ID, len(patternEveryDistinctCompound577CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternEveryDistinctCompound577Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternEveryDistinctCompound577CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternEveryDistinctCompound577JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternEveryDistinctCompound577JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternEveryDistinctCompound577ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternEveryDistinctCompound577ID)
	}
	if err := validatePatternEveryDistinctCompound577RawSteps(rawSteps); err != nil {
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

// validatePatternEveryDistinctCompound577RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned per-case sequences.
func validatePatternEveryDistinctCompound577RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternEveryDistinctCompound577CaseSpecs))
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
			if err := requirePatternEveryDistinctCompound577Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternEveryDistinctCompound577CaseSteps[marker.Case]; !ok {
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
		pins := patternEveryDistinctCompound577CaseSteps[currentCase]
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
			if err := requirePatternEveryDistinctCompound577Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternEveryDistinctCompound577Fields(object, "op", "case", "eventType", "payload"); err != nil {
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
			if err := requirePatternEveryDistinctCompound577Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time is not pinned for case %q",
					index, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternEveryDistinctCompound577Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternEveryDistinctCompound577CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternEveryDistinctCompound577CaseSpecs))
	}
	for index, spec := range patternEveryDistinctCompound577CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternEveryDistinctCompound577CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternEveryDistinctCompound577CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternEveryDistinctCompound577Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternEveryDistinctCompound577CaseState carries per-case replay state:
// the deployed statement, the listener sequence counter and the delivery
// records.
type patternEveryDistinctCompound577CaseState struct {
	spec       patternEveryDistinctCompound577CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	statements map[string]*esper.Statement
	deployment *esper.Deployment
	sequence   map[string]uint64
	records    []compat.TraceRecord
}

// runPatternEveryDistinctCompound577Scenario replays the executions
// against a fresh engine per case like the Java oracle's per-leg deploy.
func runPatternEveryDistinctCompound577Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternEveryDistinctCompound577Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternEveryDistinctCompound577CaseSpecs {
		records, err := runPatternEveryDistinctCompound577Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternEveryDistinctCompound577ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternEveryDistinctCompound577Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternEveryDistinctCompound577ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternEveryDistinctCompound577ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternEveryDistinctCompound577RawSteps(rawSteps)
}

func runPatternEveryDistinctCompound577Case(ctx context.Context, scenario compat.Scenario, spec patternEveryDistinctCompound577CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternEveryDistinctCompound577Bean](env, patternEveryDistinctCompound577SupportBeanEvent); err != nil {
		return nil, err
	}
	state := &patternEveryDistinctCompound577CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternEveryDistinctCompound577JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			payload, err := patternEveryDistinctCompound577DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse advance-time %q: %w", patternEveryDistinctCompound577ID, step.At, err)
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
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternEveryDistinctCompound577ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned EPL onto the fluent Go equivalent, mirroring
// internal/esper/pattern_everydistinct_parity_test.go:
//   - ord 8: a.And(b).EveryDistinct(TagField a.intPrimitive,
//     TagField b.intPrimitive) / EveryDistinctFor(1h, ...)
//   - ord 9: a.Or(b).EveryDistinct(Add(Coalesce(a.intPrimitive,0),
//     Coalesce(b.intPrimitive,0))) / EveryDistinctFor(1h, ...)
//   - ord 10: a.And(b.Not()).EveryDistinct(TagField a.intPrimitive) /
//     EveryDistinctFor(1h, ...)
//   - ord 11: a.Then(b).EveryDistinct(Add(a.intPrimitive,
//     b.intPrimitive)) / EveryDistinctFor(1h, ...)
//   - ord 12: a.EveryDistinct(TagField a.intPrimitive).Then(b correlated
//     on intPrimitive=a.intPrimitive) / EveryDistinctFor(2h1m, ...).Then(b)
//
// select * expands to Alias(tag, PatternEvent tag) per bound tag: ords
// 8/9/11/12 project `a` and `b` (ord 9 delivers the non-firing side as
// the null marker) while ord 10's `and not` carries only tag `a`.
func (s *patternEveryDistinctCompound577CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" || step.Epl != s.spec.epl {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternEveryDistinctCompound577ID, s.spec.name)
	}
	if _, ok := s.statements[step.Statement]; ok {
		return fmt.Errorf("%s: label %q is already deployed", patternEveryDistinctCompound577ID, step.Statement)
	}
	base := esper.From[patternEveryDistinctCompound577Bean](s.env, patternEveryDistinctCompound577SupportBeanEvent)
	a := esper.PatternFrom(base, "a", esper.LikeOf(
		esper.Field[patternEveryDistinctCompound577Bean, string]("theString"), esper.Literal("A%")))
	bLike := esper.PatternFrom(base, "b", esper.LikeOf(
		esper.Field[patternEveryDistinctCompound577Bean, string]("theString"), esper.Literal("B%")))
	selectAB := []esper.Selection{
		esper.Alias("a", esper.PatternEvent("a")),
		esper.Alias("b", esper.PatternEvent("b")),
	}
	keyA := func() esper.Expression[int] { return esper.TagField[int]("a", "intPrimitive") }
	keyB := func() esper.Expression[int] { return esper.TagField[int]("b", "intPrimitive") }
	keyOrSum := func() esper.Expression[int] {
		return esper.Add[int](
			esper.Coalesce[int](keyA(), esper.Literal(0)),
			esper.Coalesce[int](keyB(), esper.Literal(0)))
	}
	keySum := func() esper.Expression[int] { return esper.Add[int](keyA(), keyB()) }
	var query esper.Query
	switch s.spec.name {
	case "over-and":
		query = a.And(bLike).EveryDistinct(keyA(), keyB()).
			Select(selectAB...).Query(esper.StatementName("s0"))
	case "over-and-expiry":
		query = a.And(bLike).EveryDistinctFor(time.Hour, keyA(), keyB()).
			Select(selectAB...).Query(esper.StatementName("s0"))
	case "over-or":
		query = a.Or(bLike).EveryDistinct(keyOrSum()).
			Select(selectAB...).Query(esper.StatementName("s0"))
	case "over-or-expiry":
		query = a.Or(bLike).EveryDistinctFor(time.Hour, keyOrSum()).
			Select(selectAB...).Query(esper.StatementName("s0"))
	case "over-not":
		query = a.And(bLike.Not()).EveryDistinct(keyA()).
			Select(esper.Alias("a", esper.PatternEvent("a"))).Query(esper.StatementName("s0"))
	case "over-not-expiry":
		query = a.And(bLike.Not()).EveryDistinctFor(time.Hour, keyA()).
			Select(esper.Alias("a", esper.PatternEvent("a"))).Query(esper.StatementName("s0"))
	case "over-followed-by":
		query = a.Then(bLike).EveryDistinct(keySum()).
			Select(selectAB...).Query(esper.StatementName("s0"))
	case "over-followed-by-expiry":
		query = a.Then(bLike).EveryDistinctFor(time.Hour, keySum()).
			Select(selectAB...).Query(esper.StatementName("s0"))
	case "within-followed-by":
		bCorr := esper.PatternFrom(base, "b", esper.Equal[int](
			esper.Field[patternEveryDistinctCompound577Bean, int]("intPrimitive"), keyA()))
		query = a.EveryDistinct(keyA()).Then(bCorr).
			Select(selectAB...).Query(esper.StatementName("s0"))
	case "within-followed-by-expiry":
		bCorr := esper.PatternFrom(base, "b", esper.Equal[int](
			esper.Field[patternEveryDistinctCompound577Bean, int]("intPrimitive"), keyA()))
		query = a.EveryDistinctFor(2*time.Hour+time.Minute, keyA()).Then(bCorr).
			Select(selectAB...).Query(esper.StatementName("s0"))
	default:
		return fmt.Errorf("%s: unsupported case %q", patternEveryDistinctCompound577ID, s.spec.name)
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

// patternEveryDistinctCompound577DecodePayload converts a scenario send
// payload into the typed host object: a SupportBean struct with the
// pinned theString and intPrimitive fields.
func patternEveryDistinctCompound577DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case patternEveryDistinctCompound577SupportBeanEvent:
		if err := requirePatternEveryDistinctCompound577Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean patternEveryDistinctCompound577Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternEveryDistinctCompound577ID, step.EventType)
	}
}
