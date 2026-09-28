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

// Parity coverage for PatternOperatorEveryDistinct ords 4-7 and 16: the
// nested-operator quintet — every-distinct composed with the repeat ([2])
// and timer:within guards in both nesting orders, plus the int-array
// multi-key execution — over SupportBean and SupportEventWithIntArray.
//
// The chaining order is the semantic pin: applying MatchUntil/Within
// AFTER EveryDistinct nests the every-distinct inside the repeat/guard
// (Java's `[2] every-distinct(...) a` / `(...) where timer:within`),
// while applying them BEFORE nests the repeat/guard inside the
// every-distinct (Java's `every-distinct(...) [2] a` /
// `every-distinct(...) (a where timer:within)`).
//
// Covered executions (all variant collection:executions(), flags []):
//   - ord 4 PatternRepeatOverDistinct java-runtime-9ad19ee9b29674b08443
//     (cases repeat-over-distinct / repeat-over-distinct-expiry): the
//     repeat wraps every-distinct, so a match needs two distinct-key
//     events and delivers them as the a[] tag array. E1(1)+E2(1) are
//     silent (key 1 only counted once); E3(2) completes key 2 and fires
//     {a[0]=E1, a[1]=E3}; E4(3) starts a fresh match, E5(2) is a dup.
//     1 fire per leg.
//   - ord 5 PatternTimerWithinOverDistinct
//     java-runtime-56116a3e1351bcc4dc23 (cases timer-within-over-distinct /
//     timer-within-over-distinct-expiry): the 10-second guard wraps the
//     every-distinct, so the whole statement dies at deploy+10s. Clock
//     starts at 0 before deploy; E1(1) fires, E2(1) is a dup, E3(2)
//     fires; after sendTimer(11000) the guard is dead and E4(3)/E5(1)
//     stay silent — the guard-death discriminant. 2 fires per leg.
//   - ord 6 PatternEveryDistinctOverRepeat
//     java-runtime-d09d0786090e8a2e387e (cases everydistinct-over-repeat /
//     everydistinct-over-repeat-expiry): every-distinct wraps the repeat,
//     so the key a[0].intPrimitive evaluates on the COMPLETED match's
//     first element — the post-completion key discriminant. E1(1) alone
//     is silent; E2(1) completes {E1,E2} key 1 and fires. E3(1)+E4(2)
//     complete {E3,E4} but the would-be key 1 is a dup, and E5(2)+E6(1)
//     complete {E5,E6} key 2 and fire — 2 fires per leg. The expiry leg
//     carries the Java source's DOUBLED key expression
//     (a[0].intPrimitive listed twice before `1 hour`), pinned
//     byte-exact; the Go side passes the key expression twice, which
//     folds to the composite [v,v] — semantically identical.
//   - ord 7 PatternEveryDistinctOverTimerWithin
//     java-runtime-a0ae611c1bd46c4407d9 (cases
//     everydistinct-over-timerwithin / everydistinct-over-timerwithin-expiry):
//     every-distinct wraps the 10-second inner guard, so each branch
//     lives only while its own within-window is open. Clock starts at 0
//     before deploy; E1(1) fires, E2(1) is a dup; at t=5000 E3(2) fires.
//     Each dup match respawns a fresh child that REPLACES the prior one
//     (Java evaluateTrue): E4@10000 kills the E3-spawned branch before it
//     reaches its own deadline, E7 kills E6's, so earlier branches never
//     expire on their own timers. The keyset reset happens only when the
//     last surviving branch's own 10-second window closes: E4(1)/E5(1)/
//     E6(2)/E7(2)/E8(2)/E9(1) are swallowed while a branch is alive, but
//     E10(1) at t=50000 fires AGAIN on previously-seen key 1 and E12(2)
//     fires on key 2 — the within-quit keyset-reset discriminant
//     (EvalEveryDistinctStateNode respawns the attempt after the child
//     quits). E11(1)/E13(2) are dups. 4 fires per leg.
//   - ord 16 PatternEveryDistinctMultikeyWArray
//     java-runtime-1c2ea40ffffe4e58a323 (case multikey-w-array): the
//     distinct key is the a.array int[] content — content-equal arrays
//     deduplicate while null and empty are their own keys. E1{1,2}
//     fires, E2{1,2} is a content dup, E3{1} fires, E4{} fires, E5 null
//     fires; the mid-run milestone(0) savepoint is omitted (it restores
//     identical state, 573/575/576 precedent); E10{1,2}/E11{1}/E12{}/
//     E13 null are all swallowed. 4 fires.
//
// Byte-exact EPL pins keep the Java source verbatim: ord 16's
// `pattern[every-distinct` carries no space after `pattern`, and ord 6's
// expiry leg repeats the key expression literally. All legs use
// lowercase @name('s0') and select *.
//
// Siblings excluded from this slice (manifest dispositions are
// main-agent owned): ord 3 (SODA eplToModelCompileDeploy tail), ord 14
// (compile-error-only), ord 15 (calendar-month expiry), ord 13
// (dual-distinct multi-fire) remain follow-on slices; ords 0-2 and 8-12
// already live under pattern-everydistinct-576 /
// pattern-everydistinct-compound-577.
const patternEveryDistinctNested578ID = "pattern-everydistinct-nested-578"

const patternEveryDistinctNested578Description = "PatternOperatorEveryDistinct ords 4-7 and 16 — every-distinct nested with the repeat and timer:within guards in both orders plus the int-array multi-key execution, over SupportBean and SupportEventWithIntArray. repeat-over-distinct (PatternRepeatOverDistinct, ord 4) wraps every-distinct inside [2], so two distinct-key events make one match: E1(1)+E2(1) silent (key 1 counted once), E3(2) fires {a[0]=E1, a[1]=E3}, E4(3)+E5(2) silent — 1 fire. timer-within-over-distinct (PatternTimerWithinOverDistinct, ord 5) wraps every-distinct inside a 10-second guard starting at deploy (t=0): E1(1) fires, E2(1) dup, E3(2) fires, then sendTimer(11000) kills the guard so E4(3)/E5(1) stay silent — 2 fires. everydistinct-over-repeat (PatternEveryDistinctOverRepeat, ord 6) keys the COMPLETED two-event match on a[0].intPrimitive: E1(1) silent, E2(1) fires {E1,E2} key 1, E3(1)+E4(2) silent (would-be key 1 dup), E5(2)+E6(1) fires {E5,E6} key 2 — 2 fires; the expiry leg lists the key expression twice before `1 hour`, pinned byte-exact. everydistinct-over-timerwithin (PatternEveryDistinctOverTimerWithin, ord 7) guards each distinct branch with its own 10-second window: E1(1)/E3(2) fire, E4-E9 are swallowed while any guarded branch is alive, and after every branch has quit E10(1) fires again on the previously-seen key 1 (keyset reset on respawn), E12(2) fires, E11(1)/E13(2) dup — 4 fires. multikey-w-array (PatternEveryDistinctMultikeyWArray, ord 16) keys on int[] content: E1{1,2}/E3{1}/E4{}/E5null fire, E2{1,2} is a content dup and E10-E13 are all swallowed — 4 fires; the mid-run milestone(0) savepoint is omitted (it restores identical state). Each of ords 4-7 replays a no-expiry leg then a timed-expiry leg (1 hour; 2 days 2 minutes for ord 5 — both far beyond the send window) with undeployAll between; ord 16 is a single leg."

const patternEveryDistinctNested578JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternEveryDistinctNested578JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java"

// Byte-exact EPL pins (PatternOperatorEveryDistinct.java: the ord 4-7 run
// methods each deploy the no-expiry expression first, then the
// timed-expiry expression, undeploying between legs; ord 16 deploys a
// single expression). Ord 6's expiry leg carries the doubled key
// expression literally and ord 16 keeps `pattern[` without a space —
// both verbatim Java source.
const (
	patternEveryDistinctNested578RepeatOverDistinctEPL                 = "@name('s0') select * from pattern [[2] every-distinct(a.intPrimitive) a=SupportBean]"
	patternEveryDistinctNested578RepeatOverDistinctExpiryEPL           = "@name('s0') select * from pattern [[2] every-distinct(a.intPrimitive, 1 hour) a=SupportBean]"
	patternEveryDistinctNested578TimerWithinOverDistinctEPL            = "@name('s0') select * from pattern [(every-distinct(a.intPrimitive) a=SupportBean) where timer:within(10 sec)]"
	patternEveryDistinctNested578TimerWithinOverDistinctExpiryEPL      = "@name('s0') select * from pattern [(every-distinct(a.intPrimitive, 2 days 2 minutes) a=SupportBean) where timer:within(10 sec)]"
	patternEveryDistinctNested578EveryDistinctOverRepeatEPL            = "@name('s0') select * from pattern [every-distinct(a[0].intPrimitive) [2] a=SupportBean]"
	patternEveryDistinctNested578EveryDistinctOverRepeatExpiryEPL      = "@name('s0') select * from pattern [every-distinct(a[0].intPrimitive, a[0].intPrimitive, 1 hour) [2] a=SupportBean]"
	patternEveryDistinctNested578EveryDistinctOverTimerWithinEPL       = "@name('s0') select * from pattern [every-distinct(a.intPrimitive) (a=SupportBean where timer:within(10 sec))]"
	patternEveryDistinctNested578EveryDistinctOverTimerWithinExpiryEPL = "@name('s0') select * from pattern [every-distinct(a.intPrimitive, 1 hour) (a=SupportBean where timer:within(10 sec))]"
	patternEveryDistinctNested578MultikeyWArrayEPL                     = "@name('s0') select * from pattern[every-distinct(a.array) a=SupportEventWithIntArray]"

	patternEveryDistinctNested578SupportBeanEvent = "SupportBean"
	patternEveryDistinctNested578IntArrayEvent    = "SupportEventWithIntArray"
)

var (
	// One javaRuntimes row per Java execution (legs share their ord's
	// runtimeId — each leg is the same execution replayed through
	// runX(env, expression, milestone), 574/577 runtimeIndex precedent).
	patternEveryDistinctNested578JavaRuntimeIDs = []string{
		"java-runtime-9ad19ee9b29674b08443",
		"java-runtime-56116a3e1351bcc4dc23",
		"java-runtime-d09d0786090e8a2e387e",
		"java-runtime-a0ae611c1bd46c4407d9",
		"java-runtime-1c2ea40ffffe4e58a323",
	}
	patternEveryDistinctNested578JavaSources = []string{
		patternEveryDistinctNested578JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportEventWithIntArray.java",
	}
	patternEveryDistinctNested578JavaExecutions = []string{
		"PatternRepeatOverDistinct",
		"PatternTimerWithinOverDistinct",
		"PatternEveryDistinctOverRepeat",
		"PatternEveryDistinctOverTimerWithin",
		"PatternEveryDistinctMultikeyWArray",
	}
	// Deduplicated inventory id: the five runtime rows share static id
	// java-073091a0b42ca8dd974f with the other PatternOperatorEveryDistinct
	// executions, pinned once per runtimeId row.
	patternEveryDistinctNested578JavaStaticIDs = []string{
		"java-073091a0b42ca8dd974f",
		"java-073091a0b42ca8dd974f",
		"java-073091a0b42ca8dd974f",
		"java-073091a0b42ca8dd974f",
		"java-073091a0b42ca8dd974f",
	}
	patternEveryDistinctNested578JavaFlags = []string{}
)

// patternEveryDistinctNested578Bean mirrors the SupportBean properties
// the executions use: theString (the a.theString assertions pin) and
// intPrimitive (every distinct key expression).
type patternEveryDistinctNested578Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// patternEveryDistinctNested578IntArrayBean mirrors the
// SupportEventWithIntArray properties the ord 16 execution uses: id (the
// a.id discriminator the assertions read through the delivered bean) and
// array (the int[] content key — a nil slice pins the Java null-array key,
// distinct from an empty slice).
type patternEveryDistinctNested578IntArrayBean struct {
	ID    string `json:"id" esper:"id"`
	Array []int  `json:"array" esper:"array"`
}

// patternEveryDistinctNested578CaseSpec pins one case: Java execution
// identity (ordinal plus runtimeIndex into the shared per-ord arrays),
// the observation text and the byte-exact EPL script. select * projects
// every bound tag's bean row — the tag-array a for the repeat legs and
// the bean for the others.
type patternEveryDistinctNested578CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epl          string
}

var patternEveryDistinctNested578CaseSpecs = []patternEveryDistinctNested578CaseSpec{
	{
		name:         "repeat-over-distinct",
		ordinal:      4,
		runtimeIndex: 0,
		observation:  "listener; no clock; `[2] every-distinct` nests the distinct inside the repeat: E1(1)+E2(1) silent (key 1 counted once so the match never completes), E3(2) fires {a[0]=E1, a[1]=E3}, E4(3) starts a fresh match and E5(2) is a dup — 1 fire",
		epl:          patternEveryDistinctNested578RepeatOverDistinctEPL,
	},
	{
		name:         "repeat-over-distinct-expiry",
		ordinal:      4,
		runtimeIndex: 0,
		observation:  "listener; no clock; `, 1 hour` per-key expiry leg — the clock never advances so the key never expires and the identical sequence replays: E1(1)+E2(1) silent, E3(2) fires {a[0]=E1, a[1]=E3}, E4(3)+E5(2) silent",
		epl:          patternEveryDistinctNested578RepeatOverDistinctExpiryEPL,
	},
	{
		name:         "timer-within-over-distinct",
		ordinal:      5,
		runtimeIndex: 1,
		observation:  "listener; virtual clock; the 10-second guard wraps the whole every-distinct starting at deploy t=0: E1(1) fires, E2(1) dup, E3(2) fires; sendTimer(11000) kills the guard so E4(3)/E5(1) stay silent — guard-death discriminant, 2 fires",
		epl:          patternEveryDistinctNested578TimerWithinOverDistinctEPL,
	},
	{
		name:         "timer-within-over-distinct-expiry",
		ordinal:      5,
		runtimeIndex: 1,
		observation:  "listener; virtual clock; `, 2 days 2 minutes` per-key expiry leg — the expiry far outlives the 10-second guard so the identical sequence replays: E1(1)/E3(2) fire, E4(3)/E5(1) silent after sendTimer(11000)",
		epl:          patternEveryDistinctNested578TimerWithinOverDistinctExpiryEPL,
	},
	{
		name:         "everydistinct-over-repeat",
		ordinal:      6,
		runtimeIndex: 2,
		observation:  "listener; no clock; `every-distinct [2]` keys the COMPLETED match on a[0].intPrimitive: E1(1) silent, E2(1) fires {E1,E2} key 1, E3(1)+E4(2) silent (would-be key 1 dup), E5(2)+E6(1) fires {E5,E6} key 2 — post-completion key discriminant, 2 fires",
		epl:          patternEveryDistinctNested578EveryDistinctOverRepeatEPL,
	},
	{
		name:         "everydistinct-over-repeat-expiry",
		ordinal:      6,
		runtimeIndex: 2,
		observation:  "listener; no clock; `a[0].intPrimitive, a[0].intPrimitive, 1 hour` expiry leg — the Java source lists the key expression twice (pinned byte-exact) and the clock never advances so the identical sequence replays: E2 fires {E1,E2} key 1, E6 fires {E5,E6} key 2",
		epl:          patternEveryDistinctNested578EveryDistinctOverRepeatExpiryEPL,
	},
	{
		name:         "everydistinct-over-timerwithin",
		ordinal:      7,
		runtimeIndex: 3,
		observation:  "listener; virtual clock; `every-distinct (a where timer:within)` gives each branch its own 10-second window: E1(1)/E3(2) fire, E4-E9 swallowed while a branch lives, then E10(1)@50000 fires again on previously-seen key 1 (keyset reset after the last guarded branch quits) and E12(2) fires, E11(1)/E13(2) dup — within-quit keyset-reset discriminant, 4 fires",
		epl:          patternEveryDistinctNested578EveryDistinctOverTimerWithinEPL,
	},
	{
		name:         "everydistinct-over-timerwithin-expiry",
		ordinal:      7,
		runtimeIndex: 3,
		observation:  "listener; virtual clock; `, 1 hour` per-key expiry leg — the key never expires inside the 50-second replay so the identical sequence replays: E1(1)/E3(2)/E10(1)/E12(2) fire, E4-E9 and E11(1)/E13(2) silent",
		epl:          patternEveryDistinctNested578EveryDistinctOverTimerWithinExpiryEPL,
	},
	{
		name:         "multikey-w-array",
		ordinal:      16,
		runtimeIndex: 4,
		observation:  "listener; no clock; int[] content key over SupportEventWithIntArray: E1{1,2} fires, E2{1,2} content-dup silent, E3{1} fires, E4{} fires (empty key), E5null fires (null key — its own partition); milestone(0) savepoint omitted; E10{1,2}/E11{1}/E12{}/E13null all swallowed — 4 fires",
		epl:          patternEveryDistinctNested578MultikeyWArrayEPL,
	},
}

// patternEveryDistinctNested578StepPin pins one scenario step's shape.
type patternEveryDistinctNested578StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	at        string
}

func patternEveryDistinctNested578DeployPin(statement, epl string) patternEveryDistinctNested578StepPin {
	return patternEveryDistinctNested578StepPin{op: "deploy", statement: statement, epl: epl}
}

func patternEveryDistinctNested578SendPin(theString string, intPrimitive int) patternEveryDistinctNested578StepPin {
	return patternEveryDistinctNested578StepPin{
		op:        "send",
		eventType: patternEveryDistinctNested578SupportBeanEvent,
		payload:   map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)},
	}
}

// patternEveryDistinctNested578SendIntArrayPin pins a
// SupportEventWithIntArray send: the array renders as the JSON element
// list, nil pinning Java's null int[] argument.
func patternEveryDistinctNested578SendIntArrayPin(id string, array []int) patternEveryDistinctNested578StepPin {
	var elements any
	if array != nil {
		list := make([]any, 0, len(array))
		for _, item := range array {
			list = append(list, float64(item))
		}
		elements = list
	}
	return patternEveryDistinctNested578StepPin{
		op:        "send",
		eventType: patternEveryDistinctNested578IntArrayEvent,
		payload:   map[string]any{"id": id, "array": elements},
	}
}

func patternEveryDistinctNested578AdvancePin(at string) patternEveryDistinctNested578StepPin {
	return patternEveryDistinctNested578StepPin{op: "advance-time", at: at}
}

func patternEveryDistinctNested578UndeployAllPin() patternEveryDistinctNested578StepPin {
	return patternEveryDistinctNested578StepPin{op: "undeploy-all"}
}

// patternEveryDistinctNested578CaseSteps pins the complete step sequence
// per case in Java source order (milestones omitted — regression-harness
// savepoints restoring identical state; ord 16's mid-run milestone(0)
// included). Ords 5/7 carry the Java executions' sendTimer calls as
// absolute advance-time pins at t=0/5s/10s/11s/15s/20s/25s/50s; the
// timed-expiry legs replay the identical send sequence.
var patternEveryDistinctNested578CaseSteps = func() map[string][]patternEveryDistinctNested578StepPin {
	steps := make(map[string][]patternEveryDistinctNested578StepPin)
	repeatLeg := func(epl string) []patternEveryDistinctNested578StepPin {
		// ord 4: E1(1), E2(1), E3(2), E4(3), E5(2).
		return []patternEveryDistinctNested578StepPin{
			patternEveryDistinctNested578DeployPin("s0", epl),
			patternEveryDistinctNested578SendPin("E1", 1),
			patternEveryDistinctNested578SendPin("E2", 1),
			patternEveryDistinctNested578SendPin("E3", 2),
			patternEveryDistinctNested578SendPin("E4", 3),
			patternEveryDistinctNested578SendPin("E5", 2),
			patternEveryDistinctNested578UndeployAllPin(),
		}
	}
	withinLeg := func(epl string) []patternEveryDistinctNested578StepPin {
		// ord 5: sendTimer(0), deploy, E1(1), E2(1), E3(2),
		// sendTimer(11000), E4(3), E5(1).
		return []patternEveryDistinctNested578StepPin{
			patternEveryDistinctNested578AdvancePin("1970-01-01T00:00:00.000Z"),
			patternEveryDistinctNested578DeployPin("s0", epl),
			patternEveryDistinctNested578SendPin("E1", 1),
			patternEveryDistinctNested578SendPin("E2", 1),
			patternEveryDistinctNested578SendPin("E3", 2),
			patternEveryDistinctNested578AdvancePin("1970-01-01T00:00:11.000Z"),
			patternEveryDistinctNested578SendPin("E4", 3),
			patternEveryDistinctNested578SendPin("E5", 1),
			patternEveryDistinctNested578UndeployAllPin(),
		}
	}
	overRepeatLeg := func(epl string) []patternEveryDistinctNested578StepPin {
		// ord 6: E1(1), E2(1), E3(1), E4(2), E5(2), E6(1).
		return []patternEveryDistinctNested578StepPin{
			patternEveryDistinctNested578DeployPin("s0", epl),
			patternEveryDistinctNested578SendPin("E1", 1),
			patternEveryDistinctNested578SendPin("E2", 1),
			patternEveryDistinctNested578SendPin("E3", 1),
			patternEveryDistinctNested578SendPin("E4", 2),
			patternEveryDistinctNested578SendPin("E5", 2),
			patternEveryDistinctNested578SendPin("E6", 1),
			patternEveryDistinctNested578UndeployAllPin(),
		}
	}
	overWithinLeg := func(epl string) []patternEveryDistinctNested578StepPin {
		// ord 7: sendTimer(0), deploy, then the long timer sequence —
		// E1(1), E2(1); @5s E3(2); @10s E4(1), E5(1), E6(2); @15s E7(2);
		// @20s E8(2); @25s E9(1); @50s E10(1), E11(1), E12(2), E13(2).
		return []patternEveryDistinctNested578StepPin{
			patternEveryDistinctNested578AdvancePin("1970-01-01T00:00:00.000Z"),
			patternEveryDistinctNested578DeployPin("s0", epl),
			patternEveryDistinctNested578SendPin("E1", 1),
			patternEveryDistinctNested578SendPin("E2", 1),
			patternEveryDistinctNested578AdvancePin("1970-01-01T00:00:05.000Z"),
			patternEveryDistinctNested578SendPin("E3", 2),
			patternEveryDistinctNested578AdvancePin("1970-01-01T00:00:10.000Z"),
			patternEveryDistinctNested578SendPin("E4", 1),
			patternEveryDistinctNested578SendPin("E5", 1),
			patternEveryDistinctNested578SendPin("E6", 2),
			patternEveryDistinctNested578AdvancePin("1970-01-01T00:00:15.000Z"),
			patternEveryDistinctNested578SendPin("E7", 2),
			patternEveryDistinctNested578AdvancePin("1970-01-01T00:00:20.000Z"),
			patternEveryDistinctNested578SendPin("E8", 2),
			patternEveryDistinctNested578AdvancePin("1970-01-01T00:00:25.000Z"),
			patternEveryDistinctNested578SendPin("E9", 1),
			patternEveryDistinctNested578AdvancePin("1970-01-01T00:00:50.000Z"),
			patternEveryDistinctNested578SendPin("E10", 1),
			patternEveryDistinctNested578SendPin("E11", 1),
			patternEveryDistinctNested578SendPin("E12", 2),
			patternEveryDistinctNested578SendPin("E13", 2),
			patternEveryDistinctNested578UndeployAllPin(),
		}
	}
	multikeyLeg := func(epl string) []patternEveryDistinctNested578StepPin {
		// ord 16: E1{1,2}, E2{1,2}, E3{1}, E4{}, E5null, then (after the
		// omitted milestone(0) savepoint) E10{1,2}, E11{1}, E12{}, E13null.
		return []patternEveryDistinctNested578StepPin{
			patternEveryDistinctNested578DeployPin("s0", epl),
			patternEveryDistinctNested578SendIntArrayPin("E1", []int{1, 2}),
			patternEveryDistinctNested578SendIntArrayPin("E2", []int{1, 2}),
			patternEveryDistinctNested578SendIntArrayPin("E3", []int{1}),
			patternEveryDistinctNested578SendIntArrayPin("E4", []int{}),
			patternEveryDistinctNested578SendIntArrayPin("E5", nil),
			patternEveryDistinctNested578SendIntArrayPin("E10", []int{1, 2}),
			patternEveryDistinctNested578SendIntArrayPin("E11", []int{1}),
			patternEveryDistinctNested578SendIntArrayPin("E12", []int{}),
			patternEveryDistinctNested578SendIntArrayPin("E13", nil),
			patternEveryDistinctNested578UndeployAllPin(),
		}
	}
	for _, spec := range patternEveryDistinctNested578CaseSpecs {
		switch spec.name {
		case "repeat-over-distinct", "repeat-over-distinct-expiry":
			steps[spec.name] = repeatLeg(spec.epl)
		case "timer-within-over-distinct", "timer-within-over-distinct-expiry":
			steps[spec.name] = withinLeg(spec.epl)
		case "everydistinct-over-repeat", "everydistinct-over-repeat-expiry":
			steps[spec.name] = overRepeatLeg(spec.epl)
		case "everydistinct-over-timerwithin", "everydistinct-over-timerwithin-expiry":
			steps[spec.name] = overWithinLeg(spec.epl)
		case "multikey-w-array":
			steps[spec.name] = multikeyLeg(spec.epl)
		}
	}
	return steps
}()

func loadPatternEveryDistinctNested578Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternEveryDistinctNested578ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternEveryDistinctNested578ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternEveryDistinctNested578ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternEveryDistinctNested578ID, err)
	}
	if err := requirePatternEveryDistinctNested578Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternEveryDistinctNested578ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternEveryDistinctNested578ID ||
		metadata.Description != patternEveryDistinctNested578Description ||
		metadata.JavaCommit != patternEveryDistinctNested578JavaCommit ||
		metadata.JavaSource != patternEveryDistinctNested578JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternEveryDistinctNested578ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternEveryDistinctNested578JavaSources},
		{metadata.JavaRuntimes, patternEveryDistinctNested578JavaRuntimeIDs},
		{metadata.JavaNames, patternEveryDistinctNested578JavaExecutions},
		{metadata.JavaStaticIDs, patternEveryDistinctNested578JavaStaticIDs},
		{metadata.JavaFlags, patternEveryDistinctNested578JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternEveryDistinctNested578ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternEveryDistinctNested578CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternEveryDistinctNested578ID, len(patternEveryDistinctNested578CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternEveryDistinctNested578Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternEveryDistinctNested578CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternEveryDistinctNested578JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternEveryDistinctNested578JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternEveryDistinctNested578ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternEveryDistinctNested578ID)
	}
	if err := validatePatternEveryDistinctNested578RawSteps(rawSteps); err != nil {
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

// validatePatternEveryDistinctNested578RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned per-case sequences.
func validatePatternEveryDistinctNested578RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternEveryDistinctNested578CaseSpecs))
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
			if err := requirePatternEveryDistinctNested578Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternEveryDistinctNested578CaseSteps[marker.Case]; !ok {
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
		pins := patternEveryDistinctNested578CaseSteps[currentCase]
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
			if err := requirePatternEveryDistinctNested578Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternEveryDistinctNested578Fields(object, "op", "case", "eventType", "payload"); err != nil {
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
			if err := requirePatternEveryDistinctNested578Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time is not pinned for case %q",
					index, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternEveryDistinctNested578Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternEveryDistinctNested578CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternEveryDistinctNested578CaseSpecs))
	}
	for index, spec := range patternEveryDistinctNested578CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternEveryDistinctNested578CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternEveryDistinctNested578CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternEveryDistinctNested578Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternEveryDistinctNested578CaseState carries per-case replay state:
// the deployed statement, the listener sequence counter and the delivery
// records.
type patternEveryDistinctNested578CaseState struct {
	spec       patternEveryDistinctNested578CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	statements map[string]*esper.Statement
	deployment *esper.Deployment
	sequence   map[string]uint64
	records    []compat.TraceRecord
}

// runPatternEveryDistinctNested578Scenario replays the executions
// against a fresh engine per case like the Java oracle's per-leg deploy.
func runPatternEveryDistinctNested578Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternEveryDistinctNested578Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternEveryDistinctNested578CaseSpecs {
		records, err := runPatternEveryDistinctNested578Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternEveryDistinctNested578ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternEveryDistinctNested578Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternEveryDistinctNested578ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternEveryDistinctNested578ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternEveryDistinctNested578RawSteps(rawSteps)
}

func runPatternEveryDistinctNested578Case(ctx context.Context, scenario compat.Scenario, spec patternEveryDistinctNested578CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternEveryDistinctNested578Bean](env, patternEveryDistinctNested578SupportBeanEvent); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternEveryDistinctNested578IntArrayBean](env, patternEveryDistinctNested578IntArrayEvent); err != nil {
		return nil, err
	}
	state := &patternEveryDistinctNested578CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternEveryDistinctNested578JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			payload, err := patternEveryDistinctNested578DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse advance-time %q: %w", patternEveryDistinctNested578ID, step.At, err)
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
			return nil, fmt.Errorf("%s: unsupported step op %q", patternEveryDistinctNested578ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned EPL onto the fluent Go equivalent. The chaining
// order is the semantic pin: MatchUntil/Within AFTER EveryDistinct nests
// the distinct inside the repeat/guard; BEFORE nests the repeat/guard
// inside the distinct:
//   - ord 4: PatternFrom(a, true).EveryDistinct(Field intPrimitive)
//     .MatchUntil(2, 2) — repeat-over-distinct
//   - ord 5: PatternFrom(a, true).EveryDistinct(Field intPrimitive)
//     .Within(10s) — guard-over-distinct
//   - ord 6: PatternFrom(a, true).MatchUntil(2, 2).EveryDistinct(
//     TagFieldAt a[0].intPrimitive) — distinct-over-repeat; the expiry
//     leg passes the key expression twice, mirroring the Java source's
//     doubled literal which folds to the composite [v,v]
//   - ord 7: PatternFrom(a, true).Within(10s).EveryDistinct(
//     Field intPrimitive) — distinct-over-guard
//   - ord 16: PatternFrom(a, true).EveryDistinct(Field array) — int[]
//     content key
//
// select * expands to Alias(a, PatternEvent a): the repeat legs deliver
// the tag array (a[0]/a[1]) and the others the captured bean.
func (s *patternEveryDistinctNested578CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" || step.Epl != s.spec.epl {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternEveryDistinctNested578ID, s.spec.name)
	}
	if _, ok := s.statements[step.Statement]; ok {
		return fmt.Errorf("%s: label %q is already deployed", patternEveryDistinctNested578ID, step.Statement)
	}
	base := esper.From[patternEveryDistinctNested578Bean](s.env, patternEveryDistinctNested578SupportBeanEvent)
	intArrayBase := esper.From[patternEveryDistinctNested578IntArrayBean](s.env, patternEveryDistinctNested578IntArrayEvent)
	selectA := []esper.Selection{esper.Alias("a", esper.PatternEvent("a"))}
	selectTagArray := []esper.Selection{esper.Alias("a", esper.TagEvents("a"))}
	keyInt := func() esper.Expression[int] {
		return esper.Field[patternEveryDistinctNested578Bean, int]("intPrimitive")
	}
	keyAt := func() esper.Expression[int] {
		return esper.TagFieldAt[int]("a", 0, "intPrimitive")
	}
	var query esper.Query
	switch s.spec.name {
	case "repeat-over-distinct":
		query = esper.PatternFrom(base, "a", esper.Literal(true)).
			EveryDistinct(keyInt()).MatchUntil(2, 2).
			Select(selectTagArray...).Query(esper.StatementName("s0"))
	case "repeat-over-distinct-expiry":
		query = esper.PatternFrom(base, "a", esper.Literal(true)).
			EveryDistinctFor(time.Hour, keyInt()).MatchUntil(2, 2).
			Select(selectTagArray...).Query(esper.StatementName("s0"))
	case "timer-within-over-distinct":
		query = esper.PatternFrom(base, "a", esper.Literal(true)).
			EveryDistinct(keyInt()).Within(10 * time.Second).
			Select(selectA...).Query(esper.StatementName("s0"))
	case "timer-within-over-distinct-expiry":
		query = esper.PatternFrom(base, "a", esper.Literal(true)).
			EveryDistinctFor(2*24*time.Hour+2*time.Minute, keyInt()).Within(10 * time.Second).
			Select(selectA...).Query(esper.StatementName("s0"))
	case "everydistinct-over-repeat":
		query = esper.PatternFrom(base, "a", esper.Literal(true)).
			MatchUntil(2, 2).EveryDistinct(keyAt()).
			Select(selectTagArray...).Query(esper.StatementName("s0"))
	case "everydistinct-over-repeat-expiry":
		query = esper.PatternFrom(base, "a", esper.Literal(true)).
			MatchUntil(2, 2).EveryDistinctFor(time.Hour, keyAt(), keyAt()).
			Select(selectTagArray...).Query(esper.StatementName("s0"))
	case "everydistinct-over-timerwithin":
		query = esper.PatternFrom(base, "a", esper.Literal(true)).
			Within(10 * time.Second).EveryDistinct(keyInt()).
			Select(selectA...).Query(esper.StatementName("s0"))
	case "everydistinct-over-timerwithin-expiry":
		query = esper.PatternFrom(base, "a", esper.Literal(true)).
			Within(10*time.Second).EveryDistinctFor(time.Hour, keyInt()).
			Select(selectA...).Query(esper.StatementName("s0"))
	case "multikey-w-array":
		query = esper.PatternFrom(intArrayBase, "a", esper.Literal(true)).
			EveryDistinct(esper.Field[patternEveryDistinctNested578IntArrayBean, []int]("array")).
			Select(selectA...).Query(esper.StatementName("s0"))
	default:
		return fmt.Errorf("%s: unsupported case %q", patternEveryDistinctNested578ID, s.spec.name)
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

// patternEveryDistinctNested578DecodePayload converts a scenario send
// payload into the typed host object: a SupportBean struct with the
// pinned theString and intPrimitive fields, or a
// SupportEventWithIntArray struct carrying the id and the int[] content
// (a JSON null array pins the Java null int[] — the Go nil slice stays
// distinct from the empty slice both as a distinct key and in the trace
// rendering).
func patternEveryDistinctNested578DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case patternEveryDistinctNested578SupportBeanEvent:
		if err := requirePatternEveryDistinctNested578Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean patternEveryDistinctNested578Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case patternEveryDistinctNested578IntArrayEvent:
		if err := requirePatternEveryDistinctNested578Fields(fields, "id", "array"); err != nil {
			return nil, err
		}
		var bean patternEveryDistinctNested578IntArrayBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportEventWithIntArray: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternEveryDistinctNested578ID, step.EventType)
	}
}
