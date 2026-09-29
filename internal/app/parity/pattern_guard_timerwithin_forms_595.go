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

// Parity coverage for PatternGuardTimerWithin ordinals 1-6 (static
// java-0dec801a426fed297402, static-manifest java-811b1a1fcf834ee01db8,
// flags []): the six remaining timer:within executions after the ord-0
// W-harness, each replayed as its own case over a fresh engine like the
// Java oracle's fresh per-execution runtime.
//
// The semantics under test, pinned against the Java oracle:
//   - ord 1/2/3 share `tryAssertion`: a `(every SupportBean) where
//     timer:within(P)` guard armed at deploy t=0 with period
//     93784005ms fires at t=0 and at t=period-1, then goes silent at
//     exactly t=period — the exclusive deadline the 594 W-harness
//     pinned (S0/S5/S17). ord 2 reads the same period out of five
//     suite-level variables D/H/M/S/MS=1..5; ord 3 binds the five
//     `?::int` substitution params 1..5 positionally at deploy.
//   - ord 4 arms the guard from an event-correlated expression
//     `within(a.intPrimitive seconds)`: E1(3) at t=0 arms 3000ms, the
//     (every b) under the guard emits {id:E2}@2000 and {id:E3}@2999,
//     and the advance to 3000 expires it before any further send.
//   - ord 5 wraps a followed-by in `every`: each SupportBean spawns a
//     branch whose `SMDB where timer:within(5 sec)` RHS dies at its own
//     deadline; advance(6000) kills both live branches and the outer
//     every respawns at the advance instant, so E4's fresh branch at
//     t=6000 pairs with E5 at t=6000 — exactly one emission.
//   - ord 6 is two rounds over one runtime: `timer:within(1 month)` then
//     `timer:withinmax(1 month, 10)`, each arming 2002-02-01 09:00,
//     firing E1 and E2 at 2002-03-01 09:00-1ms, silent at the exact
//     month boundary.
const patternGuardTimerWithinForms595ID = "pattern-guard-timerwithin-forms-595"

const patternGuardTimerWithinForms595Description = "PatternGuardTimerWithin ordinals 1-6 — the six remaining timer:within executions after the ord-0 W-harness, each replayed as its own case on a fresh engine: ord 1 `within(1 days 2 hours 3 minutes 4 seconds 5 milliseconds)` (93784005ms, tryAssertion fire@0/fire@period-1/silent@period), ord 2 the same period read from variables D/H/M/S/MS=1..5, ord 3 the same via five positional `?::int` substitution params (1..5), ord 4 `a=SupportBean -> (every b=SupportBean) where timer:within(a.intPrimitive seconds)` (E1(3)@0 arms 3000ms, {id:E2}@2000, {id:E3}@2999, expiry@3000 silent), ord 5 `every(SupportBean -> (SupportMarketDataBean where timer:within(5 sec)))` (branch expiry + every respawn at the 6000 advance, ONE emission for E4+E5), ord 6 two rounds `(every SupportBean) where timer:within(1 month)` and `timer:withinmax(1 month, 10)` (fire E1 + E2 at boundary-1ms, silent at boundary). Java milestone()/assertRuntime calls are oracle-internal and carry no observable listener payload."

const patternGuardTimerWithinForms595JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternGuardTimerWithinForms595JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternGuardTimerWithin.java"

func patternGuardTimerWithinForms595StatementText(epl string) string {
	return fmt.Sprintf("@name('s0') select * from pattern [%s]", epl)
}

// Per-case Java identity: ordinals 1-6 each own one runtimeId while the
// whole file shares the static id.
var (
	patternGuardTimerWithinForms595JavaRuntimeIDs = []string{
		"java-runtime-f0649272cfb528ce731b",
		"java-runtime-2fe5370a7c1599bfb424",
		"java-runtime-66d65309509fac58bbfc",
		"java-runtime-36ef6f15f14a0e86ab93",
		"java-runtime-6a5e8128ae184e8a7249",
		"java-runtime-34555c4a9823a346d710",
	}
	patternGuardTimerWithinForms595JavaSources = []string{
		patternGuardTimerWithinForms595JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportMarketDataBean.java",
	}
	patternGuardTimerWithinForms595JavaExecutions = []string{
		"PatternInterval10Min",
		"PatternInterval10MinVariable",
		"PatternIntervalPrepared",
		"PatternWithinFromExpression",
		"PatternPatternNotFollowedBy",
		"PatternWithinMayMaxMonthScoped",
	}
	patternGuardTimerWithinForms595JavaStaticIDs = []string{
		"java-0dec801a426fed297402",
	}
	patternGuardTimerWithinForms595JavaFlags = []string{}
)

// patternGuardTimerWithinForms595EPLs are the deployed statement texts
// per case (ord 6's two rounds share one case and one EPL list in
// deploy order). Byte-exact from PatternGuardTimerWithin.java lines
// 231-367.
func patternGuardTimerWithinForms595EPLs(caseName string) []string {
	switch caseName {
	case "interval-10-min":
		return []string{"@name('s0') select * from pattern [(every SupportBean) where timer:within(1 days 2 hours 3 minutes 4 seconds 5 milliseconds)]"}
	case "interval-10-min-variable":
		return []string{"@name('s0') select * from pattern [(every SupportBean) where timer:within(D days H hours M minutes S seconds MS milliseconds)]"}
	case "interval-prepared":
		return []string{"@name('s0') select * from pattern [(every SupportBean) where timer:within(?::int days ?::int hours ?::int minutes ?::int seconds ?::int milliseconds)]"}
	case "within-from-expression":
		return []string{"@name('s0') select b.theString as id from pattern[a=SupportBean -> (every b=SupportBean) where timer:within(a.intPrimitive seconds)]"}
	case "pattern-not-followed-by":
		return []string{"@name('s0') select * from pattern [ every(SupportBean -> (SupportMarketDataBean where timer:within(5 sec))) ]"}
	case "may-max-month":
		return []string{
			"@name('s0') select * from pattern [(every SupportBean) where timer:within(1 month)]",
			"@name('s0') select * from pattern [(every SupportBean) where timer:withinmax(1 month, 10)]",
		}
	}
	return nil
}

// patternGuardTimerWithinForms595CaseSpec pins one execution: Java
// identity, the observation text and the fluent deployment builder(s).
// Ord 6's single case carries two deploy phases (the Java execution
// runs tryAssertionWithinMayMaxMonthScoped twice under one runtime).
type patternGuardTimerWithinForms595CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
}

// patternGuardTimerWithinForms595Bean mirrors SupportBean's two send
// payload fields used by these executions (theString/intPrimitive);
// Java's default-ctor sends carry theString=null/intPrimitive=0.
type patternGuardTimerWithinForms595Bean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// patternGuardTimerWithinForms595Market mirrors
// SupportMarketDataBean(symbol,id,price)'s three-arg ctor payload.
type patternGuardTimerWithinForms595Market struct {
	Symbol string  `esper:"symbol"`
	ID     string  `esper:"id"`
	Price  float64 `esper:"price"`
}

var patternGuardTimerWithinForms595CaseSpecs = []patternGuardTimerWithinForms595CaseSpec{
	{
		name:         "interval-10-min",
		ordinal:      1,
		runtimeIndex: 0,
		observation: "listener; external clock; guard arms at deploy t=0 with period 93784005ms " +
			"(1d2h3m4s5ms): tryAssertion sends the default-ctor SupportBean at t=0 (fire 1), " +
			"advances to 93784004 and sends again (fire 2), then advances to 93784005 and sends " +
			"a third SupportBean which the exclusive deadline kills — two empty-payload rows " +
			"(untagged atoms project nothing).",
	},
	{
		name:         "interval-10-min-variable",
		ordinal:      2,
		runtimeIndex: 1,
		observation: "listener; identical tryAssertion schedule; the guard period is the same " +
			"93784005ms read once at arm time from variables D/H/M/S/MS=1/2/3/4/5 (suite-level " +
			"declarations like TestSuitePattern.configure's addVariable doubles). Java also " +
			"asserts stmtText == eplToModel(stmtText).toEPL() — a compile-path invariant with " +
			"no Go counterpart (approved-difference precedent), not an observable payload.",
	},
	{
		name:         "interval-prepared",
		ordinal:      3,
		runtimeIndex: 2,
		observation: "listener; identical tryAssertion schedule; the guard period is the same " +
			"93784005ms composed at deploy from five positional `?::int` substitution params " +
			"(1,2,3,4,5) — Go deploys via DeployWithPositionalParameters(1..5) " +
			"instead of DeploymentOptions.setStatementSubstitutionParameter.",
	},
	{
		name:         "within-from-expression",
		ordinal:      4,
		runtimeIndex: 3,
		observation: "listener; event-correlated guard: `a=SupportBean -> (every b=SupportBean) " +
			"where timer:within(a.intPrimitive seconds)` — SupportBean(\"E1\",3)@t=0 completes a " +
			"and arms the every-b guard to t=3000; SupportBean(\"E2\",-1)@2000 emits {id:E2}, " +
			"SupportBean(\"E3\",-1)@2999 emits {id:E3}, advance to 3000 expires the guard so " +
			"the post-deadline SupportBean(\"E4\",-1) emits nothing.",
	},
	{
		name:         "pattern-not-followed-by",
		ordinal:      5,
		runtimeIndex: 4,
		observation: "listener; `every(SupportBean -> (SupportMarketDataBean where " +
			"timer:within(5 sec)))` — E1@t=0 and E2@t=0 each spawn a branch whose guarded RHS " +
			"expires at its own +5000ms deadline; advance to 6000 kills both branches and the " +
			"outer every respawns at the advance instant (594 respawn semantics), so " +
			"SupportBean(\"E4\",1)@6000 starts a fresh branch that SupportMarketDataBean " +
			"(\"E5\",\"M1\",1)@6000 completes — exactly one empty-payload emission.",
	},
	{
		name:         "may-max-month",
		ordinal:      6,
		runtimeIndex: 5,
		observation: "listener; two sequential rounds under one execution (deploy/undeploy " +
			"cycle per round): `(every SupportBean) where timer:within(1 month)` then " +
			"`timer:withinmax(1 month, 10)` — each arms 2002-02-01T09:00:00Z, fires E1 at arm " +
			"and E2 at 2002-03-01T09:00:00Z-1ms (calendar AddDate keeps the month boundary), " +
			"and stays silent for E3 at the exact boundary; two empty-payload rows per round.",
	},
}

// patternGuardTimerWithinForms595StepPin mirrors the 594 step shape:
// deploy carries the statement label, send steps carry the fused
// advance-before-send instant plus the event payload, and undeploy-all
// closes each case. Ord 6's case runs the deploy→sends→undeploy cycle
// twice (the two rounds).
type patternGuardTimerWithinForms595StepPin struct {
	op        string
	statement string
	at        string
	eventType string
	payload   map[string]any
}

func patternGuardTimerWithinForms595SendPin(at, eventType string, payload map[string]any) patternGuardTimerWithinForms595StepPin {
	return patternGuardTimerWithinForms595StepPin{op: "send", at: at, eventType: eventType, payload: payload}
}

// patternGuardTimerWithinForms595Epoch formats an epoch-millisecond
// instant like the 592 convention.
func patternGuardTimerWithinForms595Epoch(ms int64) string {
	return time.Unix(0, 0).UTC().Add(time.Duration(ms) * time.Millisecond).Format(time.RFC3339Nano)
}

// patternGuardTimerWithinForms595CaseSteps pins the complete step
// sequence per case in Java source order.
var patternGuardTimerWithinForms595CaseSteps = func() map[string][]patternGuardTimerWithinForms595StepPin {
	defaultBean := map[string]any{"theString": nil, "intPrimitive": 0}
	bean := func(s string, i int) map[string]any { return map[string]any{"theString": s, "intPrimitive": i} }
	return map[string][]patternGuardTimerWithinForms595StepPin{
		"interval-10-min": {
			{op: "deploy", statement: "s0"},
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(0), "SupportBean", defaultBean),
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(93784004), "SupportBean", defaultBean),
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(93784005), "SupportBean", defaultBean),
			{op: "undeploy-all"},
		},
		"interval-10-min-variable": {
			{op: "deploy", statement: "s0"},
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(0), "SupportBean", defaultBean),
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(93784004), "SupportBean", defaultBean),
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(93784005), "SupportBean", defaultBean),
			{op: "undeploy-all"},
		},
		"interval-prepared": {
			{op: "deploy", statement: "s0"},
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(0), "SupportBean", defaultBean),
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(93784004), "SupportBean", defaultBean),
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(93784005), "SupportBean", defaultBean),
			{op: "undeploy-all"},
		},
		"within-from-expression": {
			{op: "deploy", statement: "s0"},
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(0), "SupportBean", bean("E1", 3)),
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(2000), "SupportBean", bean("E2", -1)),
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(2999), "SupportBean", bean("E3", -1)),
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(3000), "SupportBean", bean("E4", -1)),
			{op: "undeploy-all"},
		},
		"pattern-not-followed-by": {
			{op: "deploy", statement: "s0"},
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(0), "SupportBean", bean("E1", 1)),
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(0), "SupportBean", bean("E2", 2)),
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(6000), "SupportBean", bean("E4", 1)),
			patternGuardTimerWithinForms595SendPin(patternGuardTimerWithinForms595Epoch(6000), "SupportMarketDataBean", map[string]any{"symbol": "E5", "id": "M1", "price": 1.0}),
			{op: "undeploy-all"},
		},
		"may-max-month": {
			// Round 1: timer:within(1 month). The Java oracle advances
			// to 2002-02-01 BEFORE compileDeploy, so deploy carries `at`
			// and the guard arms on the already-advanced clock.
			{op: "deploy", statement: "s0", at: "2002-02-01T09:00:00Z"},
			patternGuardTimerWithinForms595SendPin("2002-02-01T09:00:00Z", "SupportBean", bean("E1", 0)),
			patternGuardTimerWithinForms595SendPin("2002-03-01T08:59:59.999Z", "SupportBean", bean("E2", 0)),
			patternGuardTimerWithinForms595SendPin("2002-03-01T09:00:00Z", "SupportBean", bean("E3", 0)),
			{op: "undeploy-all"},
			// Round 2: timer:withinmax(1 month, 10) — same schedule.
			{op: "deploy", statement: "s0", at: "2002-02-01T09:00:00Z"},
			patternGuardTimerWithinForms595SendPin("2002-02-01T09:00:00Z", "SupportBean", bean("E1", 0)),
			patternGuardTimerWithinForms595SendPin("2002-03-01T08:59:59.999Z", "SupportBean", bean("E2", 0)),
			patternGuardTimerWithinForms595SendPin("2002-03-01T09:00:00Z", "SupportBean", bean("E3", 0)),
			{op: "undeploy-all"},
		},
	}
}()

func loadPatternGuardTimerWithinForms595Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternGuardTimerWithinForms595ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternGuardTimerWithinForms595ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternGuardTimerWithinForms595ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternGuardTimerWithinForms595ID, err)
	}
	if err := requirePatternGuardTimerWithinForms595Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternGuardTimerWithinForms595ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternGuardTimerWithinForms595ID ||
		metadata.Description != patternGuardTimerWithinForms595Description ||
		metadata.JavaCommit != patternGuardTimerWithinForms595JavaCommit ||
		metadata.JavaSource != patternGuardTimerWithinForms595JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternGuardTimerWithinForms595ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternGuardTimerWithinForms595JavaSources},
		{metadata.JavaRuntimes, patternGuardTimerWithinForms595JavaRuntimeIDs},
		{metadata.JavaNames, patternGuardTimerWithinForms595JavaExecutions},
		{metadata.JavaStaticIDs, patternGuardTimerWithinForms595JavaStaticIDs},
		{metadata.JavaFlags, patternGuardTimerWithinForms595JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternGuardTimerWithinForms595ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternGuardTimerWithinForms595CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternGuardTimerWithinForms595ID, len(patternGuardTimerWithinForms595CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternGuardTimerWithinForms595Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternGuardTimerWithinForms595CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternGuardTimerWithinForms595JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternGuardTimerWithinForms595JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation ||
			!reflect.DeepEqual(definition.EPLs, patternGuardTimerWithinForms595EPLs(spec.name)) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternGuardTimerWithinForms595ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternGuardTimerWithinForms595ID)
	}
	if err := validatePatternGuardTimerWithinForms595RawSteps(rawSteps); err != nil {
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

// validatePatternGuardTimerWithinForms595RawSteps pins the complete
// step sequence per case: op whitelists plus positional comparison
// against patternGuardTimerWithinForms595CaseSteps (deploy/send/
// undeploy-all; ord 6's case contains two cycles).
func validatePatternGuardTimerWithinForms595RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternGuardTimerWithinForms595CaseSpecs))
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
			if err := requirePatternGuardTimerWithinForms595Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternGuardTimerWithinForms595CaseSteps[marker.Case]; !ok {
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
		pins := patternGuardTimerWithinForms595CaseSteps[currentCase]
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
			// Deploys normally pin {op,case,statement}; ord 6's deploys
			// additionally pin `at` (the pre-deploy clock advance).
			fields := []string{"op", "case", "statement"}
			if pin.at != "" {
				fields = append(fields, "at")
			}
			if err := requirePatternGuardTimerWithinForms595Fields(object, fields...); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.At != pin.at {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternGuardTimerWithinForms595Fields(object, "op", "case", "at", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			// Pins hold Go literals; the JSON round-trip yields float64
			// numerics, so normalize the pin payload the same way
			// before comparing.
			pinRaw, _ := json.Marshal(pin.payload)
			var pinPayload map[string]any
			if err := json.Unmarshal(pinRaw, &pinPayload); err != nil {
				return fmt.Errorf("scenario step %d pin payload: %w", index, err)
			}
			if step.At != pin.at || step.EventType != pin.eventType || !reflect.DeepEqual(payload, pinPayload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternGuardTimerWithinForms595Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternGuardTimerWithinForms595CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternGuardTimerWithinForms595CaseSpecs))
	}
	for index, spec := range patternGuardTimerWithinForms595CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternGuardTimerWithinForms595CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternGuardTimerWithinForms595CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternGuardTimerWithinForms595Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternGuardTimerWithinForms595CaseState carries the replay state:
// the per-case engine, the deployment, listener sequence counters and
// delivery records.
type patternGuardTimerWithinForms595CaseState struct {
	spec       patternGuardTimerWithinForms595CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	deployment *esper.Deployment
	statement  *esper.Statement
	sequence   uint64
	deployIdx  int
	records    []compat.TraceRecord
}

// runPatternGuardTimerWithinForms595Scenario replays every case against
// a fresh engine like the Java oracle's fresh per-execution runtime.
func runPatternGuardTimerWithinForms595Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternGuardTimerWithinForms595Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternGuardTimerWithinForms595CaseSpecs {
		records, err := runPatternGuardTimerWithinForms595Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternGuardTimerWithinForms595ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternGuardTimerWithinForms595Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternGuardTimerWithinForms595ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternGuardTimerWithinForms595ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternGuardTimerWithinForms595RawSteps(rawSteps)
}

func runPatternGuardTimerWithinForms595Case(ctx context.Context, scenario compat.Scenario, spec patternGuardTimerWithinForms595CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternGuardTimerWithinForms595Bean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternGuardTimerWithinForms595Market](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	// Ord 2's suite-level variables D/H/M/S/MS (TestSuitePattern.configure
	// addVariable 1..5 as double.class) resolve the variable-duration
	// guard at arm time — registered as float64 to match the declared
	// Java types; duration math coerces identically either way.
	for name, value := range map[string]float64{"D": 1, "H": 2, "M": 3, "S": 4, "MS": 5} {
		if err := env.RegisterVariable(name, value); err != nil {
			return nil, err
		}
	}
	state := &patternGuardTimerWithinForms595CaseState{
		spec:   spec,
		env:    env,
		engine: esper.NewEngine(env, esper.WithRuntimeURI(patternGuardTimerWithinForms595JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			if step.At != "" {
				// Ord 6 deploys on the already-advanced clock (Java:
				// sendCurrentTime before compileDeploy).
				at, err := time.Parse(time.RFC3339Nano, step.At)
				if err != nil {
					return nil, fmt.Errorf("%s: parse deploy at %q: %w", patternGuardTimerWithinForms595ID, step.At, err)
				}
				if err := state.engine.AdvanceTime(ctx, at); err != nil {
					return nil, err
				}
			}
			if err := state.deploy(ctx, step); err != nil {
				return nil, err
			}
		case "send":
			payload, err := patternGuardTimerWithinForms595DecodePayload(step)
			if err != nil {
				return nil, err
			}
			// The fused step performs advanceTime BEFORE sendEventBean:
			// guard-expiry callbacks inside the advance run at the
			// advance instant (the exclusive deadline kills an event
			// landing exactly at arm+period) and a branch respawned
			// inside an expiry callback is live for the same-tick
			// event (ord 5's E4 branch at t=6000 sees E5).
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse send at %q: %w", patternGuardTimerWithinForms595ID, step.At, err)
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
			state.statement = nil
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternGuardTimerWithinForms595ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy builds this case's query for the next undeployed phase and
// attaches the s0 listener. The within-from-expression case projects
// `b.theString as id`; every other case is `select *` over untagged
// atoms, so Java emits empty-payload rows which the runner asserts
// through identical normalization on both sides.
func (s *patternGuardTimerWithinForms595CaseState) deploy(ctx context.Context, step compat.Step) error {
	if s.deployment != nil || step.Statement != "s0" {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternGuardTimerWithinForms595ID, s.spec.name)
	}
	if s.deployIdx >= len(patternGuardTimerWithinForms595Builders[s.spec.name]) {
		return fmt.Errorf("%s: case %q has no build for deploy phase %d",
			patternGuardTimerWithinForms595ID, s.spec.name, s.deployIdx)
	}
	build := patternGuardTimerWithinForms595Builders[s.spec.name][s.deployIdx]
	s.deployIdx++
	query, params, positional, err := build(s.env)
	if err != nil {
		return fmt.Errorf("%s: case %q build: %w", patternGuardTimerWithinForms595ID, s.spec.name, err)
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return fmt.Errorf("%s: case %q plan: %w", patternGuardTimerWithinForms595ID, s.spec.name, err)
	}
	var deployment *esper.Deployment
	switch {
	case len(positional) > 0:
		deployment, err = s.engine.DeployWithPositionalParameters(ctx, plan, positional...)
	case params != nil:
		deployment, err = s.engine.DeployWithParameters(ctx, plan, params)
	default:
		deployment, err = s.engine.Deploy(ctx, plan)
	}
	if err != nil {
		return fmt.Errorf("%s: case %q deploy: %w", patternGuardTimerWithinForms595ID, s.spec.name, err)
	}
	statements := deployment.Statements()
	if len(statements) != 1 || statements[0].Name() != "s0" {
		return fmt.Errorf("%s: case %q produced %d statements, want s0",
			patternGuardTimerWithinForms595ID, s.spec.name, len(statements))
	}
	s.deployment = deployment
	s.statement = statements[0]
	// The Java oracle attaches a fresh listener per deploy, so its
	// sequence resets each round.
	s.sequence = 0
	if _, err := s.statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		s.sequence++
		s.records = append(s.records, compat.TraceRecord{
			Case:      s.spec.name,
			Operation: "listener",
			Statement: "s0",
			Sequence:  s.sequence,
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

func patternGuardTimerWithinForms595DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requirePatternGuardTimerWithinForms595Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean patternGuardTimerWithinForms595Bean
		var theString *string
		if err := json.Unmarshal(fields["theString"], &theString); err != nil {
			return nil, fmt.Errorf("decode theString: %w", err)
		}
		if theString != nil {
			bean.TheString = *theString
		}
		if err := json.Unmarshal(fields["intPrimitive"], &bean.IntPrimitive); err != nil {
			return nil, fmt.Errorf("decode intPrimitive: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requirePatternGuardTimerWithinForms595Fields(fields, "symbol", "id", "price"); err != nil {
			return nil, err
		}
		var bean patternGuardTimerWithinForms595Market
		if err := json.Unmarshal(fields["symbol"], &bean.Symbol); err != nil {
			return nil, fmt.Errorf("decode symbol: %w", err)
		}
		if err := json.Unmarshal(fields["id"], &bean.ID); err != nil {
			return nil, fmt.Errorf("decode id: %w", err)
		}
		if err := json.Unmarshal(fields["price"], &bean.Price); err != nil {
			return nil, fmt.Errorf("decode price: %w", err)
		}
		return bean, nil
	}
	return nil, fmt.Errorf("%s: unsupported event type %q", patternGuardTimerWithinForms595ID, step.EventType)
}

// patternGuardTimerWithinForms595Builders maps each case name to its
// per-phase query builders (ord 6 has two: the within round and the
// withinmax round). Each builder returns the query plus any named/
// positional deploy bindings.
var patternGuardTimerWithinForms595Builders = func() map[string][]func(env *esper.Environment) (esper.Query, esper.ParameterValues, []any, error) {
	// every-sb = `(every SupportBean)` under the guard, untagged atoms.
	everySB := func(env *esper.Environment) esper.PatternStream {
		return esper.PatternFrom(esper.From[patternGuardTimerWithinForms595Bean](env, "SupportBean"), "", esper.Literal(true)).Every()
	}
	star := func(p esper.PatternStream) esper.Query {
		return p.Query(esper.StatementName("s0"))
	}
	return map[string][]func(env *esper.Environment) (esper.Query, esper.ParameterValues, []any, error){
		"interval-10-min": {
			func(env *esper.Environment) (esper.Query, esper.ParameterValues, []any, error) {
				// `within(1 days 2 hours 3 minutes 4 seconds 5 milliseconds)`
				// = 93784005ms fixed.
				return star(everySB(env).Within(93784005 * time.Millisecond)), nil, nil, nil
			},
		},
		"interval-10-min-variable": {
			func(env *esper.Environment) (esper.Query, esper.ParameterValues, []any, error) {
				// `within(D days H hours M minutes S seconds MS
				// milliseconds)` over the registered variables — the
				// same 93784005ms read at arm time.
				return star(everySB(env).WithinExpr(esper.DurationSum(
					esper.DurationDays[int](esper.VariableRef[int]("D")),
					esper.DurationHours[int](esper.VariableRef[int]("H")),
					esper.DurationMinutes[int](esper.VariableRef[int]("M")),
					esper.DurationSeconds[int](esper.VariableRef[int]("S")),
					esper.DurationMilliseconds[int](esper.VariableRef[int]("MS")),
				))), nil, nil, nil
			},
		},
		"interval-prepared": {
			func(env *esper.Environment) (esper.Query, esper.ParameterValues, []any, error) {
				// Five `?::int` positional params 1..5 in declaration
				// order → ParameterAt(1..5) bound positionally.
				return star(everySB(env).WithinExpr(esper.DurationSum(
					esper.DurationDays[int](esper.ParameterAt[int](1)),
					esper.DurationHours[int](esper.ParameterAt[int](2)),
					esper.DurationMinutes[int](esper.ParameterAt[int](3)),
					esper.DurationSeconds[int](esper.ParameterAt[int](4)),
					esper.DurationMilliseconds[int](esper.ParameterAt[int](5)),
				))), nil, []any{1, 2, 3, 4, 5}, nil
			},
		},
		"within-from-expression": {
			func(env *esper.Environment) (esper.Query, esper.ParameterValues, []any, error) {
				// `a=SupportBean -> (every b=SupportBean) where
				// timer:within(a.intPrimitive seconds)` — the guard
				// period resolves from the completed a event's
				// intPrimitive at arm time. Both atoms read the same
				// stream: two From() calls would duplicate the
				// SupportBean feed and double-fire b's matches.
				sb := esper.From[patternGuardTimerWithinForms595Bean](env, "SupportBean")
				a := esper.PatternFrom(sb, "a", esper.Literal(true))
				b := esper.PatternFrom(sb, "b", esper.Literal(true)).Every()
				return a.Then(b.WithinExpr(esper.DurationSeconds[int](esper.TagField[int]("a", "intPrimitive")))).Select(
					esper.Alias("id", esper.TagField[string]("b", "theString")),
				).Query(esper.StatementName("s0")), nil, nil, nil
			},
		},
		"pattern-not-followed-by": {
			func(env *esper.Environment) (esper.Query, esper.ParameterValues, []any, error) {
				// `every(SupportBean -> (SupportMarketDataBean where
				// timer:within(5 sec)))` — untagged followed-by under
				// the outer every; each left match spawns a branch
				// whose guarded RHS dies at +5s.
				sb := esper.PatternFrom(esper.From[patternGuardTimerWithinForms595Bean](env, "SupportBean"), "", esper.Literal(true))
				mdb := esper.PatternFrom(esper.From[patternGuardTimerWithinForms595Market](env, "SupportMarketDataBean"), "", esper.Literal(true)).Within(5 * time.Second)
				return star(sb.Then(mdb).Every()), nil, nil, nil
			},
		},
		"may-max-month": {
			func(env *esper.Environment) (esper.Query, esper.ParameterValues, []any, error) {
				// `within(1 month)` — calendar AddDate.
				return star(everySB(env).WithinCalendar(0, 1, 0)), nil, nil, nil
			},
			func(env *esper.Environment) (esper.Query, esper.ParameterValues, []any, error) {
				// `withinmax(1 month, 10)` — same calendar deadline
				// plus a 10-completion cap.
				return star(everySB(env).WithinOrMaxCalendar(0, 1, 0, 10)), nil, nil, nil
			},
		},
	}
}()
