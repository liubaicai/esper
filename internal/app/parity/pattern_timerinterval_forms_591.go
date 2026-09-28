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

// Parity coverage for PatternObserverTimerInterval ords 1/2/3/5/6: the
// five static-spec-resolution timer:interval forms sharing one
// external-clock harness (static java-1422565b568236b2bfea, flags []),
// each replayed as one case on a fresh engine like the Java oracle's
// fresh per-execution runtime.
//
//   - "interval-spec" (ord 1, PatternIntervalSpec): advanceTime(0) arms
//     the clock, then the literal form `timer:interval(1 minute 2
//     seconds)` deploys and arms at t=0 — the fluent leg pins the same
//     62-second deadline as TimerInterval(62s). advanceTime(61999) is
//     silent, milestone(0) is a savepoint carrying no step,
//     advanceTime(62000) fires exactly one listener batch, undeployAll.
//   - "interval-spec-variables" (ord 2, PatternIntervalSpecVariables):
//     the two `@public create variable double` deployments map to
//     env.RegisterVariable fixtures (Go has no deployment-scoped
//     variable boundary; the RegressionPath-scoped public variables are
//     read once at arming) then s0 deploys `timer:interval(M_isv minute
//     S_isv seconds)` as TimerIntervalExpr(DurationSum(DurationMinutes(
//     VariableRef("M_isv")), DurationSeconds(VariableRef("S_isv")))).
//   - "interval-spec-expression" (ord 3, PatternIntervalSpecExpression):
//     MOne/SOne register as double variables and s0 deploys
//     `timer:interval(MOne*60+SOne seconds)` as
//     TimerIntervalExpr(DurationSeconds(Add(Multiply(VariableRef("MOne"),
//     60), VariableRef("SOne")))) — the same 62000ms deadline via
//     double arithmetic.
//   - "interval-spec-prepared-stmt" (ord 5, PatternIntervalSpecPreparedStmt):
//     `timer:interval(?::int minute ?::int seconds)` with the deploy
//     step's payload [1, 2] pinning SupportPortableDeploySubstitution
//     Params().add(1,1).add(2,2) — the fluent leg binds
//     ParameterAt[int32](1)/(2) through DeployWithPositionalParameters,
//     a deploy-time binding, not a variable.
//   - "month-scoped" (ord 6, PatternMonthScoped): sendCurrentTime
//     (2002-02-01T09:00:00.000) arms then `timer:interval(1 month)`
//     deploys as TimerIntervalCalendar(0, 1, 0) — a calendar
//     recurrence, so the boundary is 2002-03-01T09:00:00.000 (Feb 1 +
//     1 month), not a fixed day span; advancing to boundary-1ms stays
//     silent and the exact boundary fires once.
//
// The deadline contract is strictly-after arming: the one-millisecond-
// early advance must not fire. select * over a tagless timer match
// projects one empty row per fire in both traces. env.milestone(0)
// savepoints carry no scenario steps (harness machinery only).
const patternTimerIntervalForms591ID = "pattern-timer-interval-forms-591"

const patternTimerIntervalForms591Description = "PatternObserverTimerInterval ords 1/2/3/5/6 — the five static-spec-resolution forms sharing one external-clock harness, replayed as five cases on fresh runtimes: advanceTime(0) (sendCurrentTime(2002-02-01T09:00:00.000) for month-scoped) before deploy, then deploy the pinned s0 pattern statement and listener, advanceTime to one millisecond before the deadline (silent), milestone(0) (a savepoint carrying no step), advanceTime to the exact deadline (ONE fire) and undeployAll. (interval-spec) PatternIntervalSpec deploys `@name('s0') select * from pattern [timer:interval(1 minute 2 seconds)]` — a fixed 62000ms deadline. (interval-spec-variables) PatternIntervalSpecVariables deploys `@public create variable double M_isv=1` and `@public create variable double S_isv=2` (RegressionPath-scoped) then `@name('s0') select * from pattern [timer:interval(M_isv minute S_isv seconds)]` — the double variables are read once at arming. (interval-spec-expression) PatternIntervalSpecExpression deploys create-variable doubles MOne=1/SOne=2 then `@name('s0') select * from pattern [timer:interval(MOne*60+SOne seconds)]` — the same 62000ms deadline via double arithmetic. (interval-spec-prepared-stmt) PatternIntervalSpecPreparedStmt compiles `@name('s0') select * from pattern [timer:interval(?::int minute ?::int seconds)]` and deploys it with statement substitution parameters add(1,1).add(2,2) — deploy-time (not variable) binding. (month-scoped) PatternMonthScoped deploys `@name('s0') select * from pattern [timer:interval(1 month)]` at sendCurrentTime(2002-02-01T09:00:00.000): `1 month` is a calendar recurrence so the boundary is 2002-03-01T09:00:00.000 (Feb 1 + 1 month), not a fixed day span. The deadline contract is strictly-after arming: advancing to deadline minus one millisecond must not fire. select * over a tagless timer match projects one empty row per fire. env.milestone(0) savepoints carry no steps."

const patternTimerIntervalForms591JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternTimerIntervalForms591JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternObserverTimerInterval.java"

// Byte-exact EPL texts (PatternObserverTimerInterval.java verbatim):
// the five s0 statements plus the four RegressionPath-scoped
// `@public create variable double` deployments; the prepared-stmt EPL
// keeps its `?::int` casts.
const patternTimerIntervalForms591IntervalSpecEPL = "@name('s0') select * from pattern [timer:interval(1 minute 2 seconds)]"
const patternTimerIntervalForms591VarMIsvEPL = "@public create variable double M_isv=1"
const patternTimerIntervalForms591VarSIsvEPL = "@public create variable double S_isv=2"
const patternTimerIntervalForms591IntervalVarsEPL = "@name('s0') select * from pattern [timer:interval(M_isv minute S_isv seconds)]"
const patternTimerIntervalForms591VarMOneEPL = "@public create variable double MOne=1"
const patternTimerIntervalForms591VarSOneEPL = "@public create variable double SOne=2"
const patternTimerIntervalForms591IntervalExprEPL = "@name('s0') select * from pattern [timer:interval(MOne*60+SOne seconds)]"
const patternTimerIntervalForms591IntervalPreparedEPL = "@name('s0') select * from pattern [timer:interval(?::int minute ?::int seconds)]"
const patternTimerIntervalForms591IntervalMonthEPL = "@name('s0') select * from pattern [timer:interval(1 month)]"

// Pinned instants: sendTimer(0/61999/62000) for the four fixed forms and
// sendCurrentTime for month-scoped, expressed as UTC RFC3339 instants
// (the Java harness runs under -Duser.timezone=UTC, so
// DateTime.parseDefaultMSec("2002-02-01T09:00:00.000") equals
// 2002-02-01T09:00:00Z).
const (
	patternTimerIntervalForms591ArmFixed = "1970-01-01T00:00:00Z"
	patternTimerIntervalForms591Silent   = "1970-01-01T00:01:01.999Z"
	patternTimerIntervalForms591Fire     = "1970-01-01T00:01:02Z"
	patternTimerIntervalForms591ArmMonth = "2002-02-01T09:00:00Z"
	patternTimerIntervalForms591SilentM  = "2002-03-01T08:59:59.999Z"
	patternTimerIntervalForms591FireM    = "2002-03-01T09:00:00Z"
)

// Per-case Java identity: each case owns its ordinal's runtimeId; the
// five executions share the collection's single static id.
var (
	patternTimerIntervalForms591JavaRuntimeIDs = []string{
		"java-runtime-d5ad6ad9d226628f8383",
		"java-runtime-bbb75d6bcc54f29c7652",
		"java-runtime-97c5e6b5e4c46cc0adb4",
		"java-runtime-ea394f5795b88ddd71c9",
		"java-runtime-28fc7f508485cbc765a2",
	}
	patternTimerIntervalForms591JavaExecutions = []string{
		"PatternIntervalSpec",
		"PatternIntervalSpecVariables",
		"PatternIntervalSpecExpression",
		"PatternIntervalSpecPreparedStmt",
		"PatternMonthScoped",
	}
	patternTimerIntervalForms591JavaStaticIDs = []string{
		"java-1422565b568236b2bfea",
		"java-1422565b568236b2bfea",
		"java-1422565b568236b2bfea",
		"java-1422565b568236b2bfea",
		"java-1422565b568236b2bfea",
	}
	patternTimerIntervalForms591JavaSources = []string{
		patternTimerIntervalForms591JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
	}
	patternTimerIntervalForms591JavaFlags = []string{}
)

// patternTimerIntervalForms591Bean is the timer observer's source
// stream — the Java executions register SupportBean on the runtime's
// configuration even though no events are sent; the Go legs anchor the
// observer on the same registered type.
type patternTimerIntervalForms591Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

const patternTimerIntervalForms591BeanType = "SupportBean"

// patternTimerIntervalForms591CaseSpec pins one case: Java execution
// identity (ordinal plus runtimeIndex into the shared per-ord arrays),
// the observation text and the byte-exact EPL texts pinned through
// case.epls.
type patternTimerIntervalForms591CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epls         []string
}

var patternTimerIntervalForms591CaseSpecs = []patternTimerIntervalForms591CaseSpec{
	{
		name:         "interval-spec",
		ordinal:      1,
		runtimeIndex: 0,
		observation: "listener x1; advanceTime(0) arms, deploy s0 `timer:interval(1 minute 2 " +
			"seconds)`, advanceTime(61999) silent, milestone(0) no-step, " +
			"advanceTime(62000) fires ONE row (select * over a tagless timer " +
			"match yields the empty row), undeployAll",
		epls: []string{patternTimerIntervalForms591IntervalSpecEPL},
	},
	{
		name:         "interval-spec-variables",
		ordinal:      2,
		runtimeIndex: 1,
		observation: "listener x1; advanceTime(0) arms, deploy `@public create variable double " +
			"M_isv=1` + `@public create variable double S_isv=2` on the shared " +
			"path then s0 `timer:interval(M_isv minute S_isv seconds)` — " +
			"variables read once at arming give the same 62000ms deadline; " +
			"advanceTime(61999) silent, milestone(0) no-step, " +
			"advanceTime(62000) fires ONE row, undeployAll",
		epls: []string{
			patternTimerIntervalForms591VarMIsvEPL,
			patternTimerIntervalForms591VarSIsvEPL,
			patternTimerIntervalForms591IntervalVarsEPL,
		},
	},
	{
		name:         "interval-spec-expression",
		ordinal:      3,
		runtimeIndex: 2,
		observation: "listener x1; advanceTime(0) arms, deploy `@public create variable double " +
			"MOne=1` + `@public create variable double SOne=2` then s0 " +
			"`timer:interval(MOne*60+SOne seconds)` — double arithmetic yields " +
			"the same 62000ms deadline; advanceTime(61999) silent, milestone(0) " +
			"no-step, advanceTime(62000) fires ONE row, undeployAll",
		epls: []string{
			patternTimerIntervalForms591VarMOneEPL,
			patternTimerIntervalForms591VarSOneEPL,
			patternTimerIntervalForms591IntervalExprEPL,
		},
	},
	{
		name:         "interval-spec-prepared-stmt",
		ordinal:      5,
		runtimeIndex: 3,
		observation: "listener x1; advanceTime(0) arms, compile `@name('s0') select * from pattern " +
			"[timer:interval(?::int minute ?::int seconds)]` and deploy with " +
			"statement substitution parameters add(1,1).add(2,2) (deploy-time " +
			"binding, not variables); advanceTime(61999) silent, milestone(0) " +
			"no-step, advanceTime(62000) fires ONE row, undeployAll",
		epls: []string{patternTimerIntervalForms591IntervalPreparedEPL},
	},
	{
		name:         "month-scoped",
		ordinal:      6,
		runtimeIndex: 4,
		observation: "listener x1; sendCurrentTime(2002-02-01T09:00:00.000) arms, deploy s0 " +
			"`timer:interval(1 month)` — calendar recurrence: the boundary is " +
			"2002-03-01T09:00:00.000 (Feb 1 + 1 calendar month, not a fixed day " +
			"span); advanceTime(boundary-1ms) silent, milestone(0) no-step, " +
			"advanceTime(boundary) fires ONE row, undeployAll",
		epls: []string{patternTimerIntervalForms591IntervalMonthEPL},
	},
}

// patternTimerIntervalForms591StepPin pins one scenario step's shape:
// deploy steps carry the statement label, the byte-exact EPL and — only
// for the prepared-stmt s0 deploy — the positional substitution
// parameter payload; advance-time steps carry the pinned instant.
type patternTimerIntervalForms591StepPin struct {
	op        string
	statement string
	epl       string
	at        string
	params    []int
}

func patternTimerIntervalForms591DeployPin(statement, epl string) patternTimerIntervalForms591StepPin {
	return patternTimerIntervalForms591StepPin{op: "deploy", statement: statement, epl: epl}
}

// patternTimerIntervalForms591PreparedDeployPin pins the prepared-stmt
// deploy: the positional substitution parameters [1, 2] ride the deploy
// step's payload (SupportPortableDeploySubstitutionParams().add(1,1)
// .add(2,2) on the Java side).
func patternTimerIntervalForms591PreparedDeployPin() patternTimerIntervalForms591StepPin {
	return patternTimerIntervalForms591StepPin{
		op:        "deploy",
		statement: "s0",
		epl:       patternTimerIntervalForms591IntervalPreparedEPL,
		params:    []int{1, 2},
	}
}

func patternTimerIntervalForms591AdvancePin(at string) patternTimerIntervalForms591StepPin {
	return patternTimerIntervalForms591StepPin{op: "advance-time", at: at}
}

func patternTimerIntervalForms591UndeployAllPin() patternTimerIntervalForms591StepPin {
	return patternTimerIntervalForms591StepPin{op: "undeploy-all"}
}

// patternTimerIntervalForms591FixedSteps is the shared step sequence for
// the four fixed-deadline cases: arm at t=0, deploy s0 (plus the
// create-variable deployments for the variable/expression cases), the
// 61999 silent probe, the 62000 firing advance and undeploy-all. The
// milestone(0) savepoint between the probes carries no step.
func patternTimerIntervalForms591FixedSteps(deploys ...patternTimerIntervalForms591StepPin) []patternTimerIntervalForms591StepPin {
	steps := []patternTimerIntervalForms591StepPin{
		patternTimerIntervalForms591AdvancePin(patternTimerIntervalForms591ArmFixed),
	}
	steps = append(steps, deploys...)
	return append(steps,
		patternTimerIntervalForms591AdvancePin(patternTimerIntervalForms591Silent),
		patternTimerIntervalForms591AdvancePin(patternTimerIntervalForms591Fire),
		patternTimerIntervalForms591UndeployAllPin())
}

// patternTimerIntervalForms591CaseSteps pins the complete step sequence
// in execution order per case; the month-scoped case arms at the 2002
// Feb-1 wall time and fires at the Mar-1 calendar boundary.
var patternTimerIntervalForms591CaseSteps = map[string][]patternTimerIntervalForms591StepPin{
	"interval-spec": patternTimerIntervalForms591FixedSteps(
		patternTimerIntervalForms591DeployPin("s0", patternTimerIntervalForms591IntervalSpecEPL)),
	"interval-spec-variables": patternTimerIntervalForms591FixedSteps(
		patternTimerIntervalForms591DeployPin("create-m_isv", patternTimerIntervalForms591VarMIsvEPL),
		patternTimerIntervalForms591DeployPin("create-s_isv", patternTimerIntervalForms591VarSIsvEPL),
		patternTimerIntervalForms591DeployPin("s0", patternTimerIntervalForms591IntervalVarsEPL)),
	"interval-spec-expression": patternTimerIntervalForms591FixedSteps(
		patternTimerIntervalForms591DeployPin("create-mone", patternTimerIntervalForms591VarMOneEPL),
		patternTimerIntervalForms591DeployPin("create-sone", patternTimerIntervalForms591VarSOneEPL),
		patternTimerIntervalForms591DeployPin("s0", patternTimerIntervalForms591IntervalExprEPL)),
	"interval-spec-prepared-stmt": patternTimerIntervalForms591FixedSteps(
		patternTimerIntervalForms591PreparedDeployPin()),
	"month-scoped": {
		patternTimerIntervalForms591AdvancePin(patternTimerIntervalForms591ArmMonth),
		patternTimerIntervalForms591DeployPin("s0", patternTimerIntervalForms591IntervalMonthEPL),
		patternTimerIntervalForms591AdvancePin(patternTimerIntervalForms591SilentM),
		patternTimerIntervalForms591AdvancePin(patternTimerIntervalForms591FireM),
		patternTimerIntervalForms591UndeployAllPin(),
	},
}

func loadPatternTimerIntervalForms591Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternTimerIntervalForms591ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternTimerIntervalForms591ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternTimerIntervalForms591ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternTimerIntervalForms591ID, err)
	}
	if err := requirePatternTimerIntervalForms591Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternTimerIntervalForms591ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternTimerIntervalForms591ID ||
		metadata.Description != patternTimerIntervalForms591Description ||
		metadata.JavaCommit != patternTimerIntervalForms591JavaCommit ||
		metadata.JavaSource != patternTimerIntervalForms591JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternTimerIntervalForms591ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternTimerIntervalForms591JavaSources},
		{metadata.JavaRuntimes, patternTimerIntervalForms591JavaRuntimeIDs},
		{metadata.JavaNames, patternTimerIntervalForms591JavaExecutions},
		{metadata.JavaStaticIDs, patternTimerIntervalForms591JavaStaticIDs},
		{metadata.JavaFlags, patternTimerIntervalForms591JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternTimerIntervalForms591ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternTimerIntervalForms591CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternTimerIntervalForms591ID, len(patternTimerIntervalForms591CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternTimerIntervalForms591Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternTimerIntervalForms591CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternTimerIntervalForms591JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternTimerIntervalForms591JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || !reflect.DeepEqual(definition.EPLs, spec.epls) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternTimerIntervalForms591ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternTimerIntervalForms591ID)
	}
	if err := validatePatternTimerIntervalForms591RawSteps(rawSteps); err != nil {
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

// validatePatternTimerIntervalForms591RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned per-case sequences — the arm advance-time, the
// deploys (byte-exact EPL per step; the prepared-stmt s0 deploy carries
// the pinned positional payload [1, 2]), the silent and firing
// advance-time probes and undeploy-all. The milestone(0) savepoints
// between the probes carry no steps.
func validatePatternTimerIntervalForms591RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternTimerIntervalForms591CaseSpecs))
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
			Payload   json.RawMessage `json:"payload"`
		}
		if operation == "case" {
			if err := requirePatternTimerIntervalForms591Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternTimerIntervalForms591CaseSteps[marker.Case]; !ok {
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
		pins := patternTimerIntervalForms591CaseSteps[currentCase]
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
			if len(pin.params) > 0 {
				if err := requirePatternTimerIntervalForms591Fields(object, "op", "case", "statement", "epl", "payload"); err != nil {
					return fmt.Errorf("scenario step %d: %w", index, err)
				}
				var params []int
				if err := json.Unmarshal(step.Payload, &params); err != nil {
					return fmt.Errorf("scenario step %d deploy payload: %w", index, err)
				}
				if !reflect.DeepEqual(params, pin.params) {
					return fmt.Errorf("scenario step %d deploy payload is not pinned for case %q", index, currentCase)
				}
			} else {
				if err := requirePatternTimerIntervalForms591Fields(object, "op", "case", "statement", "epl"); err != nil {
					return fmt.Errorf("scenario step %d: %w", index, err)
				}
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "advance-time":
			if err := requirePatternTimerIntervalForms591Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time is not pinned for case %q", index, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternTimerIntervalForms591Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternTimerIntervalForms591CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternTimerIntervalForms591CaseSpecs))
	}
	for index, spec := range patternTimerIntervalForms591CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternTimerIntervalForms591CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternTimerIntervalForms591CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternTimerIntervalForms591Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternTimerIntervalForms591CaseState carries the per-case replay
// state: the deployment handle, the listener sequence counter and the
// delivery records.
type patternTimerIntervalForms591CaseState struct {
	spec       patternTimerIntervalForms591CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	deployment *esper.Deployment
	records    []compat.TraceRecord
}

// runPatternTimerIntervalForms591Scenario replays each execution against
// a fresh engine like the Java oracle's fresh per-execution runtime; the
// engine starts at the epoch and the scenario's advance-time steps drive
// the external clock.
func runPatternTimerIntervalForms591Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternTimerIntervalForms591Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternTimerIntervalForms591CaseSpecs {
		records, err := runPatternTimerIntervalForms591Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternTimerIntervalForms591ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternTimerIntervalForms591Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternTimerIntervalForms591ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternTimerIntervalForms591ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternTimerIntervalForms591RawSteps(rawSteps)
}

func runPatternTimerIntervalForms591Case(ctx context.Context, scenario compat.Scenario, spec patternTimerIntervalForms591CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternTimerIntervalForms591Bean](env, patternTimerIntervalForms591BeanType); err != nil {
		return nil, err
	}
	state := &patternTimerIntervalForms591CaseState{
		spec:   spec,
		env:    env,
		engine: esper.NewEngine(env, esper.WithRuntimeURI(patternTimerIntervalForms591JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse advance-time %q: %w", patternTimerIntervalForms591ID, step.At, err)
			}
			if err := state.engine.AdvanceTime(ctx, at); err != nil {
				return nil, err
			}
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return nil, err
			}
		case "undeploy-all":
			if state.deployment != nil {
				if err := state.deployment.Undeploy(ctx); err != nil {
					return nil, err
				}
				state.deployment = nil
			}
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternTimerIntervalForms591ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps each pinned EPL onto the observable-equivalent Go action:
// the create-* labels are `@public create variable` deployments — the
// deployment-scoped public variable has no Go boundary, so they map to
// env.RegisterVariable fixtures whose values the s0 statement's duration
// expression reads at arming; the s0 deploy builds the fluent plan,
// deploys it (with positional parameters for the prepared-stmt case) and
// attaches the trace listener (env.addListener("s0")).
func (s *patternTimerIntervalForms591CaseState) deploy(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "create-m_isv":
		return s.registerVariable(step, patternTimerIntervalForms591VarMIsvEPL, "M_isv", 1.0)
	case "create-s_isv":
		return s.registerVariable(step, patternTimerIntervalForms591VarSIsvEPL, "S_isv", 2.0)
	case "create-mone":
		return s.registerVariable(step, patternTimerIntervalForms591VarMOneEPL, "MOne", 1.0)
	case "create-sone":
		return s.registerVariable(step, patternTimerIntervalForms591VarSOneEPL, "SOne", 2.0)
	case "s0":
		// falls through to the statement deployment below
	default:
		return fmt.Errorf("%s: unknown deploy %q in case %q", patternTimerIntervalForms591ID, step.Statement, s.spec.name)
	}
	query, err := s.query()
	if err != nil {
		return err
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return err
	}
	var deployment *esper.Deployment
	if s.spec.name == "interval-spec-prepared-stmt" {
		params, err := patternTimerIntervalForms591DeployParams(step)
		if err != nil {
			return err
		}
		deployment, err = s.engine.DeployWithPositionalParameters(ctx, plan, params...)
	} else {
		deployment, err = s.engine.Deploy(ctx, plan)
	}
	if err != nil {
		return err
	}
	statements := deployment.Statements()
	if len(statements) != 1 || statements[0].Name() != "s0" {
		return fmt.Errorf("%s: expected one s0 statement", patternTimerIntervalForms591ID)
	}
	statement := statements[0]
	s.deployment = deployment
	var sequence uint64
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		sequence++
		s.records = append(s.records, compat.TraceRecord{
			Case:      s.spec.name,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  sequence,
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

// registerVariable replays one `@public create variable double` deploy:
// the deployment-scoped public variable has no Go boundary so the
// env-level registration is the observable equivalent — the s0
// statement's duration expression resolves the value at arming.
func (s *patternTimerIntervalForms591CaseState) registerVariable(step compat.Step, pinnedEPL, name string, initial float64) error {
	if step.Epl != pinnedEPL {
		return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q", patternTimerIntervalForms591ID, step.Statement, step.Epl)
	}
	return s.env.RegisterVariable(name, initial)
}

// patternTimerIntervalForms591DeployParams decodes the prepared-stmt
// deploy payload — the pinned positional substitution parameters [1, 2]
// of SupportPortableDeploySubstitutionParams().add(1,1).add(2,2) — into
// the int32 values DeployWithPositionalParameters binds to
// ParameterAt[int32](1)/(2) (Java `?::int`).
func patternTimerIntervalForms591DeployParams(step compat.Step) ([]any, error) {
	var params []int
	if err := json.Unmarshal(step.Payload, &params); err != nil {
		return nil, fmt.Errorf("%s: decode deploy payload: %w", patternTimerIntervalForms591ID, err)
	}
	if !reflect.DeepEqual(params, []int{1, 2}) {
		return nil, fmt.Errorf("%s: deploy payload %v is not pinned", patternTimerIntervalForms591ID, params)
	}
	values := make([]any, len(params))
	for index, value := range params {
		values[index] = int32(value)
	}
	return values, nil
}

// query builds the fluent plan for the case's s0 statement. select *
// over the tagless timer match projects one empty row per fire — the
// Java wildcard over a pattern with no tags produces the same
// zero-column row.
func (s *patternTimerIntervalForms591CaseState) query() (esper.Query, error) {
	base := esper.From[patternTimerIntervalForms591Bean](s.env, patternTimerIntervalForms591BeanType)
	switch s.spec.name {
	case "interval-spec":
		// timer:interval(1 minute 2 seconds) — the fixed literal form.
		return esper.TimerInterval(base, time.Minute+2*time.Second).
			Select().Query(esper.StatementName("s0")), nil
	case "interval-spec-variables":
		// timer:interval(M_isv minute S_isv seconds) — component-wise
		// variable references, read once at arming.
		return esper.TimerIntervalExpr(base, esper.DurationSum(
			esper.DurationMinutes[float64](esper.VariableRef[float64]("M_isv")),
			esper.DurationSeconds[float64](esper.VariableRef[float64]("S_isv")),
		)).Select().Query(esper.StatementName("s0")), nil
	case "interval-spec-expression":
		// timer:interval(MOne*60+SOne seconds) — one expression in
		// seconds over the double variables.
		return esper.TimerIntervalExpr(base, esper.DurationSeconds[float64](
			esper.Add[float64](
				esper.Multiply[float64](esper.VariableRef[float64]("MOne"), esper.Literal[float64](60)),
				esper.VariableRef[float64]("SOne")),
		)).Select().Query(esper.StatementName("s0")), nil
	case "interval-spec-prepared-stmt":
		// timer:interval(?::int minute ?::int seconds) — deploy-time
		// positional substitution parameters bound at deploy, not
		// variables.
		return esper.TimerIntervalExpr(base, esper.DurationSum(
			esper.DurationMinutes[int32](esper.ParameterAt[int32](1)),
			esper.DurationSeconds[int32](esper.ParameterAt[int32](2)),
		)).Select().Query(esper.StatementName("s0")), nil
	case "month-scoped":
		// timer:interval(1 month) — calendar recurrence (Feb 1 ->
		// Mar 1 boundary), not a fixed day span.
		return esper.TimerIntervalCalendar(base, 0, 1, 0).
			Select().Query(esper.StatementName("s0")), nil
	}
	return esper.Query{}, fmt.Errorf("%s: case %q has no s0 leg", patternTimerIntervalForms591ID, s.spec.name)
}
