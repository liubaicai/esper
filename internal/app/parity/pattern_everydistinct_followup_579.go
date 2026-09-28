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

// Parity coverage for PatternOperatorEveryDistinct ords 3, 13 and 15: the
// follow-up triplet — every-distinct over a bare filter with an
// unqualified key, dual every-distinct on both followed-by sides with
// per-branch right keysets, and calendar-month scoped expiry — all over
// SupportBean.
//
// Covered executions (all variant collection:executions(), flags []):
//   - ord 3 PatternEveryDistinctOverFilter java-runtime-7d52c9e689e3eeca65d7
//     (cases over-filter / over-filter-expiry): the distinct key is the
//     UNQUALIFIED intPrimitive (no a. prefix — the Go side maps it to
//     Field[bean,int]("intPrimitive") like the existing OverFilter parity
//     test, not TagField). E1(1) fires, E2(1) is a dup, E3(2)/E4(3) fire,
//     E5(2)/E6(3)/E7(1) stay silent, E8(0) fires — 4 fires per leg. The
//     `2 minutes` expiry leg carries no clock ops so the identical
//     sequence replays. The trailing eplToModelCompileDeploy(expression)
//     per leg is a SODA/object-model front-end round-trip and is excluded
//     (compile-text precedent, cf. complex-property-access-575 ords 3/4).
//   - ord 13 PatternFollowedByWithDistinct
//     java-runtime-0fe853af533719599512 (cases followedby-with-distinct /
//     followedby-with-distinct-expiry): both followed-by sides carry
//     their own every-distinct over a='A%' -> b='B%'. Each retained left
//     branch owns an INDEPENDENT right-side keyset: B1(0) and B2(1) both
//     fire on the A1 branch, B3(0) is a right-key dup on that branch,
//     A2(1) is a swallowed LEFT dup (spawns no branch), B4(2) fires
//     {A1,B4}, A3(2) arms a second branch and B5(1) fires {A3,B5}, B6(1)
//     is a dup on the A3 branch, and B7(3) completes BOTH retained
//     branches delivering TWO rows {A1,B7},{A3,B7} — the fan-out
//     discriminant — 5 listener calls / 6 rows per leg. The `1 day` expiry
//     leg applies to the LEFT every-distinct ONLY (the right side stays
//     bare in the Java source — pinned asymmetry; the pre-existing Go
//     parity test wrongly expires both sides, this asset pins the
//     byte-exact left-only mapping). No clock, so both legs replay the
//     identical sequence.
//   - ord 15 PatternMonthScoped java-runtime-d211795c3ecad71c297a (case
//     month-scoped): `every-distinct(theString, 1 month)` is a CALENDAR
//     expiry — a key lives until the first-seen wall time shifted by one
//     month. Clock starts at 2002-02-01T09:00:00 before deploy (Java
//     sendCurrentTime; Go WithStartTime). E1(1) fires {E1,1}, E1(2) is a
//     dup; at 2002-03-01T09:00:00 MINUS 1ms the key is still alive so
//     E1(3) stays silent (boundary exclusive); the mid-run milestone(0)
//     savepoint is omitted (restores identical state, 573/575/576
//     precedent); at exactly 2002-03-01T09:00:00 E1(4) fires {E1,4} —
//     2 fires.
//
// Byte-exact EPL pins keep the Java source verbatim: ord 3's expiry leg
// carries `every-distinct(intPrimitive,2 minutes)` with no space after
// the comma, ord 13 keeps both `theString like 'A%'/'B%'` filter
// expressions, and ord 15 keeps `1 month`. All legs use lowercase
// @name('s0') and select *.
//
// Siblings excluded from this slice (manifest dispositions are
// main-agent owned): ord 14 (compile-error-only tryInvalid diagnostics)
// remains the file's last open execution; ords 0-2, 4-12 and 16 already
// live under pattern-everydistinct-576 / pattern-everydistinct-compound-577
// / pattern-everydistinct-nested-578.
const patternEveryDistinctFollowup579ID = "pattern-everydistinct-followup-579"

const patternEveryDistinctFollowup579Description = "PatternOperatorEveryDistinct ords 3, 13 and 15 — the follow-up triplet: every-distinct over a bare filter with an unqualified key, dual every-distinct on both followed-by sides with per-branch right keysets, and calendar-month scoped expiry over SupportBean. over-filter (PatternEveryDistinctOverFilter, ord 3) keys on the unqualified intPrimitive: E1(1) fires, E2(1) dup, E3(2)/E4(3) fire, E5(2)/E6(3)/E7(1) silent, E8(0) fires — 4 fires; the `2 minutes` expiry leg replays the identical sequence with no clock and the trailing eplToModelCompileDeploy SODA round-trip is excluded (compile-text precedent). followedby-with-distinct (PatternFollowedByWithDistinct, ord 13) keys each followed-by side on its own tag's intPrimitive over a='A%' -> b='B%': B1(0)/B2(1) both fire on the A1 branch (per-branch right keyset), B3(0) is a right-key dup, A2(1) is a swallowed left dup spawning no branch, B4(2) fires {A1,B4}, A3(2) arms a second branch and B5(1) fires {A3,B5}, B6(1) is a dup on the A3 branch, and B7(3) fans out TWO rows {A1,B7},{A3,B7} inside one listener update — 5 listener calls, 6 rows; the `1 day` expiry leg applies to the LEFT distinct only (right side bare — pinned asymmetry) and replays identically. month-scoped (PatternMonthScoped, ord 15) keys on theString with a 1-month calendar expiry starting at 2002-02-01T09:00:00: E1(1) fires {E1,1}, E1(2) is a dup, E1(3) at one millisecond before the 2002-03-01 mark stays silent (the milestone(0) savepoint is omitted — restores identical state), and E1(4) at exactly the month mark fires {E1,4} — 2 fires."

const patternEveryDistinctFollowup579JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternEveryDistinctFollowup579JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java"

// Byte-exact EPL pins (PatternOperatorEveryDistinct.java: the ord 3 and
// ord 13 run methods each deploy the no-expiry expression first, then the
// timed-expiry expression, undeploying between legs; ord 15 deploys a
// single expression after sendCurrentTime). Ord 3's expiry leg keeps
// `intPrimitive,2 minutes` with no space and ord 13's expiry leg expires
// the LEFT distinct only — both verbatim Java source.
const (
	patternEveryDistinctFollowup579OverFilterEPL                  = "@name('s0') select * from pattern [every-distinct(intPrimitive) a=SupportBean]"
	patternEveryDistinctFollowup579OverFilterExpiryEPL            = "@name('s0') select * from pattern [every-distinct(intPrimitive,2 minutes) a=SupportBean]"
	patternEveryDistinctFollowup579FollowedByWithDistinctEPL      = "@name('s0') select * from pattern [every-distinct(a.intPrimitive) a=SupportBean(theString like 'A%') -> every-distinct(b.intPrimitive) b=SupportBean(theString like 'B%')]"
	patternEveryDistinctFollowup579FollowedByWithDistinctExpirEPL = "@name('s0') select * from pattern [every-distinct(a.intPrimitive, 1 day) a=SupportBean(theString like 'A%') -> every-distinct(b.intPrimitive) b=SupportBean(theString like 'B%')]"
	patternEveryDistinctFollowup579MonthScopedEPL                 = "@name('s0') select * from pattern [every-distinct(theString, 1 month) a=SupportBean]"

	patternEveryDistinctFollowup579SupportBeanEvent = "SupportBean"
)

var (
	// One javaRuntimes row per Java execution (legs share their ord's
	// runtimeId — each leg is the same execution replayed through
	// runX(env, expression, milestone), 574/577/578 runtimeIndex
	// precedent).
	patternEveryDistinctFollowup579JavaRuntimeIDs = []string{
		"java-runtime-7d52c9e689e3eeca65d7",
		"java-runtime-0fe853af533719599512",
		"java-runtime-d211795c3ecad71c297a",
	}
	patternEveryDistinctFollowup579JavaSources = []string{
		patternEveryDistinctFollowup579JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
	}
	patternEveryDistinctFollowup579JavaExecutions = []string{
		"PatternEveryDistinctOverFilter",
		"PatternFollowedByWithDistinct",
		"PatternMonthScoped",
	}
	// Deduplicated inventory id: the three runtime rows share static id
	// java-073091a0b42ca8dd974f with the other PatternOperatorEveryDistinct
	// executions, pinned once per runtimeId row.
	patternEveryDistinctFollowup579JavaStaticIDs = []string{
		"java-073091a0b42ca8dd974f",
		"java-073091a0b42ca8dd974f",
		"java-073091a0b42ca8dd974f",
	}
	patternEveryDistinctFollowup579JavaFlags = []string{}
)

// patternEveryDistinctFollowup579Bean mirrors the SupportBean properties
// the executions use: theString (the 'A%'/'B%' filter expressions, the
// a.theString/b.theString assertions and the ord 15 calendar key) and
// intPrimitive (the ord 3/13 distinct key expressions).
type patternEveryDistinctFollowup579Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// patternEveryDistinctFollowup579CaseSpec pins one case: Java execution
// identity (ordinal plus runtimeIndex into the shared per-ord arrays),
// the observation text and the byte-exact EPL script. select * projects
// every bound tag's bean row — `a` alone for ords 3/15, `a` and `b` for
// ord 13's followed-by.
type patternEveryDistinctFollowup579CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epl          string
}

var patternEveryDistinctFollowup579CaseSpecs = []patternEveryDistinctFollowup579CaseSpec{
	{
		name:         "over-filter",
		ordinal:      3,
		runtimeIndex: 0,
		observation:  "listener; no clock; unqualified `every-distinct(intPrimitive)` key over a bare a=SupportBean: E1(1) fires, E2(1) dup, E3(2)/E4(3) fire, E5(2)/E6(3)/E7(1) silent, E8(0) fires — 4 fires; trailing eplToModelCompileDeploy SODA round-trip excluded (compile-text precedent)",
		epl:          patternEveryDistinctFollowup579OverFilterEPL,
	},
	{
		name:         "over-filter-expiry",
		ordinal:      3,
		runtimeIndex: 0,
		observation:  "listener; no clock; `every-distinct(intPrimitive,2 minutes)` per-key expiry leg (no space after the comma — pinned byte-exact) — the clock never advances so the identical sequence replays: E1(1)/E3(2)/E4(3)/E8(0) fire, E2(1)/E5(2)/E6(3)/E7(1) silent; SODA tail excluded",
		epl:          patternEveryDistinctFollowup579OverFilterExpiryEPL,
	},
	{
		name:         "followedby-with-distinct",
		ordinal:      13,
		runtimeIndex: 1,
		observation:  "listener; no clock; dual every-distinct over a='A%' -> b='B%' with per-branch right keysets: A1(1) arms the left, B1(0)/B2(1) fire on the A1 branch, B3(0) right-dup silent, A2(1) left-dup swallowed, B4(2) fires {A1,B4}, A3(2)+B5(1) fire {A3,B5}, B6(1) A3-branch dup, B7(3) TWO rows {A1,B7},{A3,B7} any-order — 5 fires, 6 rows",
		epl:          patternEveryDistinctFollowup579FollowedByWithDistinctEPL,
	},
	{
		name:         "followedby-with-distinct-expiry",
		ordinal:      13,
		runtimeIndex: 1,
		observation:  "listener; no clock; `1 day` expiry applies to the LEFT every-distinct only (the right side stays bare — pinned asymmetry); the key never expires in-run so the identical sequence replays including the two-row B7 fan-out {A1,B7},{A3,B7} — 5 fires, 6 rows",
		epl:          patternEveryDistinctFollowup579FollowedByWithDistinctExpirEPL,
	},
	{
		name:         "month-scoped",
		ordinal:      15,
		runtimeIndex: 2,
		observation:  "listener; virtual clock 2002-02-01T09:00:00; `every-distinct(theString, 1 month)` calendar expiry: E1(1) fires {E1,1}, E1(2) dup, advance to 2002-03-01T09:00:00 minus 1ms keeps E1(3) silent (boundary exclusive — key still alive), milestone(0) savepoint omitted, advance to the exact month mark lets E1(4) fire {E1,4} — 2 fires",
		epl:          patternEveryDistinctFollowup579MonthScopedEPL,
	},
}

// patternEveryDistinctFollowup579StepPin pins one scenario step's shape.
type patternEveryDistinctFollowup579StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	at        string
}

func patternEveryDistinctFollowup579DeployPin(statement, epl string) patternEveryDistinctFollowup579StepPin {
	return patternEveryDistinctFollowup579StepPin{op: "deploy", statement: statement, epl: epl}
}

func patternEveryDistinctFollowup579SendPin(theString string, intPrimitive int) patternEveryDistinctFollowup579StepPin {
	return patternEveryDistinctFollowup579StepPin{
		op:        "send",
		eventType: patternEveryDistinctFollowup579SupportBeanEvent,
		payload:   map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)},
	}
}

func patternEveryDistinctFollowup579AdvancePin(at string) patternEveryDistinctFollowup579StepPin {
	return patternEveryDistinctFollowup579StepPin{op: "advance-time", at: at}
}

func patternEveryDistinctFollowup579UndeployAllPin() patternEveryDistinctFollowup579StepPin {
	return patternEveryDistinctFollowup579StepPin{op: "undeploy-all"}
}

// patternEveryDistinctFollowup579CaseSteps pins the complete step sequence
// per case in Java source order (milestones omitted — regression-harness
// savepoints restoring identical state; ord 15's mid-run milestone(0)
// included). Ord 15 carries the Java execution's sendCurrentTime calls as
// absolute advance-time pins at 2002-02-01T09:00:00.000Z,
// 2002-03-01T08:59:59.999Z and 2002-03-01T09:00:00.000Z; the
// timed-expiry legs replay the identical send sequence.
var patternEveryDistinctFollowup579CaseSteps = func() map[string][]patternEveryDistinctFollowup579StepPin {
	steps := make(map[string][]patternEveryDistinctFollowup579StepPin)
	overFilterLeg := func(epl string) []patternEveryDistinctFollowup579StepPin {
		// ord 3: E1(1), E2(1), E3(2), E4(3), E5(2), E6(3), E7(1), E8(0).
		return []patternEveryDistinctFollowup579StepPin{
			patternEveryDistinctFollowup579DeployPin("s0", epl),
			patternEveryDistinctFollowup579SendPin("E1", 1),
			patternEveryDistinctFollowup579SendPin("E2", 1),
			patternEveryDistinctFollowup579SendPin("E3", 2),
			patternEveryDistinctFollowup579SendPin("E4", 3),
			patternEveryDistinctFollowup579SendPin("E5", 2),
			patternEveryDistinctFollowup579SendPin("E6", 3),
			patternEveryDistinctFollowup579SendPin("E7", 1),
			patternEveryDistinctFollowup579SendPin("E8", 0),
			patternEveryDistinctFollowup579UndeployAllPin(),
		}
	}
	followedByLeg := func(epl string) []patternEveryDistinctFollowup579StepPin {
		// ord 13: A1(1), B1(0), B2(1), B3(0), A2(1), B4(2), A3(2),
		// B5(1), B6(1), B7(3).
		return []patternEveryDistinctFollowup579StepPin{
			patternEveryDistinctFollowup579DeployPin("s0", epl),
			patternEveryDistinctFollowup579SendPin("A1", 1),
			patternEveryDistinctFollowup579SendPin("B1", 0),
			patternEveryDistinctFollowup579SendPin("B2", 1),
			patternEveryDistinctFollowup579SendPin("B3", 0),
			patternEveryDistinctFollowup579SendPin("A2", 1),
			patternEveryDistinctFollowup579SendPin("B4", 2),
			patternEveryDistinctFollowup579SendPin("A3", 2),
			patternEveryDistinctFollowup579SendPin("B5", 1),
			patternEveryDistinctFollowup579SendPin("B6", 1),
			patternEveryDistinctFollowup579SendPin("B7", 3),
			patternEveryDistinctFollowup579UndeployAllPin(),
		}
	}
	monthScopedLeg := func(epl string) []patternEveryDistinctFollowup579StepPin {
		// ord 15: sendCurrentTime(2002-02-01T09:00:00.000), deploy,
		// E1(1), E1(2), sendCurrentTimeWithMinus(2002-03-01T09:00:00.000,
		// 1), E1(3), milestone(0) omitted, sendCurrentTime
		// (2002-03-01T09:00:00.000), E1(4).
		return []patternEveryDistinctFollowup579StepPin{
			patternEveryDistinctFollowup579AdvancePin("2002-02-01T09:00:00.000Z"),
			patternEveryDistinctFollowup579DeployPin("s0", epl),
			patternEveryDistinctFollowup579SendPin("E1", 1),
			patternEveryDistinctFollowup579SendPin("E1", 2),
			patternEveryDistinctFollowup579AdvancePin("2002-03-01T08:59:59.999Z"),
			patternEveryDistinctFollowup579SendPin("E1", 3),
			patternEveryDistinctFollowup579AdvancePin("2002-03-01T09:00:00.000Z"),
			patternEveryDistinctFollowup579SendPin("E1", 4),
			patternEveryDistinctFollowup579UndeployAllPin(),
		}
	}
	for _, spec := range patternEveryDistinctFollowup579CaseSpecs {
		switch spec.name {
		case "over-filter", "over-filter-expiry":
			steps[spec.name] = overFilterLeg(spec.epl)
		case "followedby-with-distinct", "followedby-with-distinct-expiry":
			steps[spec.name] = followedByLeg(spec.epl)
		case "month-scoped":
			steps[spec.name] = monthScopedLeg(spec.epl)
		}
	}
	return steps
}()

func loadPatternEveryDistinctFollowup579Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternEveryDistinctFollowup579ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternEveryDistinctFollowup579ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternEveryDistinctFollowup579ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternEveryDistinctFollowup579ID, err)
	}
	if err := requirePatternEveryDistinctFollowup579Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternEveryDistinctFollowup579ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternEveryDistinctFollowup579ID ||
		metadata.Description != patternEveryDistinctFollowup579Description ||
		metadata.JavaCommit != patternEveryDistinctFollowup579JavaCommit ||
		metadata.JavaSource != patternEveryDistinctFollowup579JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternEveryDistinctFollowup579ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternEveryDistinctFollowup579JavaSources},
		{metadata.JavaRuntimes, patternEveryDistinctFollowup579JavaRuntimeIDs},
		{metadata.JavaNames, patternEveryDistinctFollowup579JavaExecutions},
		{metadata.JavaStaticIDs, patternEveryDistinctFollowup579JavaStaticIDs},
		{metadata.JavaFlags, patternEveryDistinctFollowup579JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternEveryDistinctFollowup579ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternEveryDistinctFollowup579CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternEveryDistinctFollowup579ID, len(patternEveryDistinctFollowup579CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternEveryDistinctFollowup579Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternEveryDistinctFollowup579CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternEveryDistinctFollowup579JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternEveryDistinctFollowup579JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternEveryDistinctFollowup579ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternEveryDistinctFollowup579ID)
	}
	if err := validatePatternEveryDistinctFollowup579RawSteps(rawSteps); err != nil {
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

// validatePatternEveryDistinctFollowup579RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned per-case sequences.
func validatePatternEveryDistinctFollowup579RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternEveryDistinctFollowup579CaseSpecs))
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
			if err := requirePatternEveryDistinctFollowup579Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternEveryDistinctFollowup579CaseSteps[marker.Case]; !ok {
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
		pins := patternEveryDistinctFollowup579CaseSteps[currentCase]
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
			if err := requirePatternEveryDistinctFollowup579Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternEveryDistinctFollowup579Fields(object, "op", "case", "eventType", "payload"); err != nil {
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
			if err := requirePatternEveryDistinctFollowup579Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time is not pinned for case %q",
					index, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternEveryDistinctFollowup579Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternEveryDistinctFollowup579CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternEveryDistinctFollowup579CaseSpecs))
	}
	for index, spec := range patternEveryDistinctFollowup579CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternEveryDistinctFollowup579CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternEveryDistinctFollowup579CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternEveryDistinctFollowup579Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternEveryDistinctFollowup579CaseState carries per-case replay state:
// the deployed statement, the listener sequence counter and the delivery
// records.
type patternEveryDistinctFollowup579CaseState struct {
	spec       patternEveryDistinctFollowup579CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	statements map[string]*esper.Statement
	deployment *esper.Deployment
	sequence   map[string]uint64
	records    []compat.TraceRecord
}

// runPatternEveryDistinctFollowup579Scenario replays the executions
// against a fresh engine per case like the Java oracle's per-leg deploy.
func runPatternEveryDistinctFollowup579Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternEveryDistinctFollowup579Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternEveryDistinctFollowup579CaseSpecs {
		records, err := runPatternEveryDistinctFollowup579Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternEveryDistinctFollowup579ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternEveryDistinctFollowup579Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternEveryDistinctFollowup579ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternEveryDistinctFollowup579ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternEveryDistinctFollowup579RawSteps(rawSteps)
}

func runPatternEveryDistinctFollowup579Case(ctx context.Context, scenario compat.Scenario, spec patternEveryDistinctFollowup579CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternEveryDistinctFollowup579Bean](env, patternEveryDistinctFollowup579SupportBeanEvent); err != nil {
		return nil, err
	}
	// Ord 15 pins the Java sendCurrentTime("2002-02-01T09:00:00.000")
	// before deploy as the engine start clock (WithStartTime); the
	// advance-time step at the same instant replays it verbatim. The
	// no-clock legs keep the epoch start like the Java initialize(0).
	start := time.Unix(0, 0).UTC()
	if spec.name == "month-scoped" {
		start = time.Date(2002, time.February, 1, 9, 0, 0, 0, time.UTC)
	}
	state := &patternEveryDistinctFollowup579CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternEveryDistinctFollowup579JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(start)),
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
			payload, err := patternEveryDistinctFollowup579DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse advance-time %q: %w", patternEveryDistinctFollowup579ID, step.At, err)
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
			return nil, fmt.Errorf("%s: unsupported step op %q", patternEveryDistinctFollowup579ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned EPL onto the fluent Go equivalent, mirroring
// internal/esper/pattern_everydistinct_parity_test.go:
//   - ord 3: PatternFrom(a, true).EveryDistinct(Field intPrimitive) /
//     EveryDistinctFor(2min, ...) — the Java key is UNQUALIFIED so the Go
//     key is the plain stream Field, not TagField
//   - ord 13: a.EveryDistinct(TagField a.intPrimitive).Then(
//     b.EveryDistinct(TagField b.intPrimitive)) over 'A%'/'B%' filters;
//     the expiry leg applies EveryDistinctFor(24h) to the LEFT side only
//     (the right side stays a bare EveryDistinct — the Java source
//     expires only the left distinct)
//   - ord 15: PatternFrom(a, true).EveryDistinctForCalendar(0, 1, 0,
//     TagField a.theString) — calendar-month key expiry
//
// select * expands to Alias(tag, PatternEvent tag) per bound tag: `a`
// alone for ords 3/15 and `a`,`b` for ord 13's followed-by.
func (s *patternEveryDistinctFollowup579CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" || step.Epl != s.spec.epl {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternEveryDistinctFollowup579ID, s.spec.name)
	}
	if _, ok := s.statements[step.Statement]; ok {
		return fmt.Errorf("%s: label %q is already deployed", patternEveryDistinctFollowup579ID, step.Statement)
	}
	base := esper.From[patternEveryDistinctFollowup579Bean](s.env, patternEveryDistinctFollowup579SupportBeanEvent)
	aBare := esper.PatternFrom(base, "a", esper.Literal(true))
	aLike := esper.PatternFrom(base, "a", esper.LikeOf(
		esper.Field[patternEveryDistinctFollowup579Bean, string]("theString"), esper.Literal("A%")))
	bLike := esper.PatternFrom(base, "b", esper.LikeOf(
		esper.Field[patternEveryDistinctFollowup579Bean, string]("theString"), esper.Literal("B%")))
	selectA := []esper.Selection{esper.Alias("a", esper.PatternEvent("a"))}
	selectAB := []esper.Selection{
		esper.Alias("a", esper.PatternEvent("a")),
		esper.Alias("b", esper.PatternEvent("b")),
	}
	keyField := func() esper.Expression[int] {
		return esper.Field[patternEveryDistinctFollowup579Bean, int]("intPrimitive")
	}
	keyA := func() esper.Expression[int] { return esper.TagField[int]("a", "intPrimitive") }
	keyB := func() esper.Expression[int] { return esper.TagField[int]("b", "intPrimitive") }
	var query esper.Query
	switch s.spec.name {
	case "over-filter":
		query = aBare.EveryDistinct(keyField()).
			Select(selectA...).Query(esper.StatementName("s0"))
	case "over-filter-expiry":
		query = aBare.EveryDistinctFor(2*time.Minute, keyField()).
			Select(selectA...).Query(esper.StatementName("s0"))
	case "followedby-with-distinct":
		query = aLike.EveryDistinct(keyA()).Then(bLike.EveryDistinct(keyB())).
			Select(selectAB...).Query(esper.StatementName("s0"))
	case "followedby-with-distinct-expiry":
		query = aLike.EveryDistinctFor(24*time.Hour, keyA()).Then(bLike.EveryDistinct(keyB())).
			Select(selectAB...).Query(esper.StatementName("s0"))
	case "month-scoped":
		query = aBare.EveryDistinctForCalendar(0, 1, 0, esper.TagField[string]("a", "theString")).
			Select(selectA...).Query(esper.StatementName("s0"))
	default:
		return fmt.Errorf("%s: unsupported case %q", patternEveryDistinctFollowup579ID, s.spec.name)
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

// patternEveryDistinctFollowup579DecodePayload converts a scenario send
// payload into the typed host object: a SupportBean struct with the
// pinned theString and intPrimitive fields.
func patternEveryDistinctFollowup579DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case patternEveryDistinctFollowup579SupportBeanEvent:
		if err := requirePatternEveryDistinctFollowup579Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean patternEveryDistinctFollowup579Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternEveryDistinctFollowup579ID, step.EventType)
	}
}
