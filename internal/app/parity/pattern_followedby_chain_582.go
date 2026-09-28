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

// Parity coverage for PatternOperatorFollowedBy ords 7 and 8: the pure
// followed-by chain pair — no timers, no guards, no clock ops. Ord 7 is
// an every-head fan-out whose two armed branches share one B/C/D tail;
// ord 8 (ESPER-411) is a correlated second-leg predicate replayed as
// TWO sequential deploy/undeploy-all cycles inside ONE Java execution
// (ONE runtimeId, unlike the per-leg case split of slice 577).
// env.milestone savepoints in the Java executions are harness splits
// restoring identical state and are unrepresented.
//
// Covered executions (both variant collection:executions(), flags [],
// deduped static id java-089b2086c9945dff918f):
//   - ord 7 PatternFollowedEveryMultiple java-runtime-25f948812c392841f1e5
//     (case every-multiple): `every a=SupportBean_A -> b=SupportBean_B ->
//     c=SupportBean_C -> d=SupportBean_D`. Sends A1, A2, B1, C1, D1: each
//     A arms a branch waiting on the same B1/C1/D1 tail, so the D1 send
//     completes BOTH branches in ONE listener invocation carrying TWO
//     rows — [A1,B1,C1,D1] first, then [A2,B1,C1,D1]
//     (assertPropsPerRowLastNew preserves the match order). select *
//     projects the a/b/c/d bean fragments.
//   - ord 8 PatternFilterGreaterThen java-runtime-4e9e6ad069be6a8e6074
//     (case filter-greater-then): ESPER-411 — ONE execution, TWO
//     sequential deploy/undeploy-all cycles under the same runtimeId.
//     Phase 1 `pattern[every a=SupportBean -> b=SupportBean(b.intPrimitive
//     <= a.intPrimitive)]` (NO space after `pattern`), phase 2 `pattern
//     [every a=SupportBean -> b=SupportBean(a.intPrimitive >=
//     b.intPrimitive)]` (WITH space). Each phase sends E1(10) then
//     E2(11) and fires ZERO times: the correlated second-leg predicate
//     evaluates against the captured first-leg event in either operand
//     order (11<=10 false; 10>=11 false). Zero records.
//
// Byte-exact EPL pins keep the Java source verbatim: ord 7 keeps the
// single space after `pattern`, ord 8 pins BOTH discriminants — the
// spaceless `pattern[` of phase 1 against the spaced `pattern [` of
// phase 2 — plus the operand-order flip (b.X <= a.X vs a.X >= b.X).
// select * on both executions projects every bound tag's bean fragment;
// ord 8's projected a/b fragments are unobservable because both phases
// are silent, matching the Java assertListenerNotInvoked pins.
//
// Siblings excluded from this slice (manifest dispositions are
// main-agent owned): ord 0 (W-harness), ord 2 (timer:within + where),
// ords 3-5 (RFID trio, covered by pattern-followedby-rfid-580) and ords
// 1/6/9 (timer+not trio, covered by pattern-followedby-timernot-581)
// remain tracked in the capability manifest by main.
const patternFollowedByChain582ID = "pattern-followedby-chain-582"

const patternFollowedByChain582Description = "PatternOperatorFollowedBy ords 7 and 8 — the pure followed-by chain pair (no timers, no guards, no clock ops): every-head fan-out over a shared tail plus a correlated second-leg predicate. every-multiple (PatternFollowedEveryMultiple, ord 7) deploys `every a=SupportBean_A -> b=SupportBean_B -> c=SupportBean_C -> d=SupportBean_D`: sends A1, A2, B1, C1, D1 deliver ONE listener invocation carrying TWO rows — [A1,B1,C1,D1] first then [A2,B1,C1,D1] — both A-branches pair with the shared B1/C1/D1 tail, select * projecting the a/b/c/d fragments. filter-greater-then (PatternFilterGreaterThen, ord 8, ESPER-411) runs TWO sequential deploy/undeploy-all cycles under ONE runtimeId in a single execution: phase 1 `pattern[every a=SupportBean -> b=SupportBean(b.intPrimitive <= a.intPrimitive)]` (no space after pattern), phase 2 `pattern [every a=SupportBean -> b=SupportBean(a.intPrimitive >= b.intPrimitive)]` (with space) — each phase sends E1(10), E2(11) and fires ZERO times because the correlated second-leg predicate evaluates against the captured a event in either operand order. All env.milestone savepoints are harness splits restoring identical state and are unrepresented."

const patternFollowedByChain582JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternFollowedByChain582JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java"

// Byte-exact EPL pins (PatternOperatorFollowedBy.java, verbatim): ord 7
// keeps the single space after `pattern`; ord 8 pins the spaceless
// `pattern[` of phase 1 against the spaced `pattern [` of phase 2 plus
// the operand-order flip — the two EPLs differ only in bracket spacing
// and the correlated comparison's operand order.
const (
	patternFollowedByChain582EveryMultipleEPL = "@name('s0') select * from pattern [every a=SupportBean_A -> b=SupportBean_B -> c=SupportBean_C -> d=SupportBean_D]"
	patternFollowedByChain582LessOrEqualEPL   = "@name('s0') select * from pattern[every a=SupportBean -> b=SupportBean(b.intPrimitive <= a.intPrimitive)]"
	patternFollowedByChain582GreaterOrEqEPL   = "@name('s0') select * from pattern [every a=SupportBean -> b=SupportBean(a.intPrimitive >= b.intPrimitive)]"

	patternFollowedByChain582SupportBeanType  = "SupportBean"
	patternFollowedByChain582SupportBeanAType = "SupportBean_A"
	patternFollowedByChain582SupportBeanBType = "SupportBean_B"
	patternFollowedByChain582SupportBeanCType = "SupportBean_C"
	patternFollowedByChain582SupportBeanDType = "SupportBean_D"
)

// Per-case Java identities: each case pins its own ordinal plus a
// runtimeIndex into the arrays below. Ord 8's two deploy phases stay in
// ONE case so both share the execution's runtimeId — matching the
// single-run Java semantics (undeployAll between, same runtime).
var (
	patternFollowedByChain582JavaRuntimeIDs = []string{
		"java-runtime-25f948812c392841f1e5",
		"java-runtime-4e9e6ad069be6a8e6074",
	}
	patternFollowedByChain582JavaSources = []string{
		patternFollowedByChain582JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_C.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_D.java",
	}
	patternFollowedByChain582JavaExecutions = []string{
		"PatternFollowedEveryMultiple",
		"PatternFilterGreaterThen",
	}
	// Deduplicated inventory id: all ten PatternOperatorFollowedBy
	// inventory rows share static id java-089b2086c9945dff918f (NOT the
	// every-distinct file's java-073091a0b42ca8dd974f), pinned once per
	// runtimeId row.
	patternFollowedByChain582JavaStaticIDs = []string{
		"java-089b2086c9945dff918f",
		"java-089b2086c9945dff918f",
	}
	patternFollowedByChain582JavaFlags = []string{}
)

// patternFollowedByChain582SupportBean mirrors SupportBean's asserted
// fields (theString, intPrimitive). theString is a *string so a null
// would render the {"state":"null"} marker exactly like the Java bean;
// ord 8 only ever sends E1/E2, so the field is always populated.
type patternFollowedByChain582SupportBean struct {
	TheString    *string `json:"theString" esper:"theString"`
	IntPrimitive int     `json:"intPrimitive" esper:"intPrimitive"`
}

// patternFollowedByChain582BeanA/B/C/D mirror SupportBean_A/B/C/D: each
// carries the single id property the chain projects under select *.
type patternFollowedByChain582BeanA struct {
	ID string `json:"id" esper:"id"`
}
type patternFollowedByChain582BeanB struct {
	ID string `json:"id" esper:"id"`
}
type patternFollowedByChain582BeanC struct {
	ID string `json:"id" esper:"id"`
}
type patternFollowedByChain582BeanD struct {
	ID string `json:"id" esper:"id"`
}

// patternFollowedByChain582CaseSpec pins one case: Java execution
// identity (ordinal plus runtimeIndex into the shared per-ord arrays),
// the observation text and the byte-exact EPL scripts — one per deploy
// step. Ord 7 deploys once; ord 8 deploys twice in sequence under the
// same runtimeId, so epls carries both phases in Java source order.
type patternFollowedByChain582CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epls         []string
}

var patternFollowedByChain582CaseSpecs = []patternFollowedByChain582CaseSpec{
	{
		name:         "every-multiple",
		ordinal:      7,
		runtimeIndex: 0,
		observation:  "listener; no clock; `every a=SupportBean_A -> b=SupportBean_B -> c=SupportBean_C -> d=SupportBean_D`: A1+A2 arm two branches that share the B1/C1/D1 tail, so D1 completes both — ONE listener invocation carrying TWO rows, [A1,B1,C1,D1] first then [A2,B1,C1,D1]; select * projects the a/b/c/d fragments",
		epls:         []string{patternFollowedByChain582EveryMultipleEPL},
	},
	{
		name:         "filter-greater-then",
		ordinal:      8,
		runtimeIndex: 1,
		observation:  "listener; no clock; ESPER-411 — TWO sequential deploy/undeploy-all cycles under ONE runtimeId: `pattern[every a=SupportBean -> b=SupportBean(b.intPrimitive <= a.intPrimitive)]` (no space after pattern) then `pattern [every a=SupportBean -> b=SupportBean(a.intPrimitive >= b.intPrimitive)]` (with space); each phase sends E1(10), E2(11) and fires ZERO times in both operand orders — 0 records",
		epls:         []string{patternFollowedByChain582LessOrEqualEPL, patternFollowedByChain582GreaterOrEqEPL},
	},
}

// patternFollowedByChain582StepPin pins one scenario step's shape:
// deploys carry the byte-exact EPL and sends the pinned payload. No
// advance-time or other clock ops exist in this slice — the Java
// executions never touch the timer service.
type patternFollowedByChain582StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
}

func patternFollowedByChain582DeployPin(statement, epl string) patternFollowedByChain582StepPin {
	return patternFollowedByChain582StepPin{op: "deploy", statement: statement, epl: epl}
}

func patternFollowedByChain582SendPin(eventType string, payload map[string]any) patternFollowedByChain582StepPin {
	return patternFollowedByChain582StepPin{op: "send", eventType: eventType, payload: payload}
}

func patternFollowedByChain582IDSendPin(eventType, id string) patternFollowedByChain582StepPin {
	return patternFollowedByChain582SendPin(eventType, map[string]any{"id": id})
}

func patternFollowedByChain582BeanSendPin(theString any, intPrimitive int) patternFollowedByChain582StepPin {
	return patternFollowedByChain582SendPin(patternFollowedByChain582SupportBeanType,
		map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)})
}

func patternFollowedByChain582UndeployAllPin() patternFollowedByChain582StepPin {
	return patternFollowedByChain582StepPin{op: "undeploy-all"}
}

// patternFollowedByChain582CaseSteps pins the complete step sequence
// per case in Java source order (env.milestone savepoints omitted —
// they restore identical state: ord 7 milestones 0/1 between the A1/A2
// and B1/C1 sends, ord 8 milestones 0/1 between each phase's sends).
// Ord 8's single case carries both deploy legs with the undeploy-all
// between them, exactly like the Java undeployAll inside one run.
var patternFollowedByChain582CaseSteps = func() map[string][]patternFollowedByChain582StepPin {
	steps := make(map[string][]patternFollowedByChain582StepPin)
	for _, spec := range patternFollowedByChain582CaseSpecs {
		switch spec.name {
		case "every-multiple":
			// ord 7: A1, milestone, A2, B1, milestone, C1 (not invoked),
			// D1 — both A-branches complete on the shared tail.
			steps[spec.name] = []patternFollowedByChain582StepPin{
				patternFollowedByChain582DeployPin("s0", spec.epls[0]),
				patternFollowedByChain582IDSendPin(patternFollowedByChain582SupportBeanAType, "A1"),
				patternFollowedByChain582IDSendPin(patternFollowedByChain582SupportBeanAType, "A2"),
				patternFollowedByChain582IDSendPin(patternFollowedByChain582SupportBeanBType, "B1"),
				patternFollowedByChain582IDSendPin(patternFollowedByChain582SupportBeanCType, "C1"),
				patternFollowedByChain582IDSendPin(patternFollowedByChain582SupportBeanDType, "D1"),
				patternFollowedByChain582UndeployAllPin(),
			}
		case "filter-greater-then":
			// ord 8: phase 1 deploys the spaceless pattern[ ... leg,
			// sends E1(10),E2(11) (silent), undeploys all; phase 2
			// redeploys s0 with the spaced pattern [ ... leg and the
			// flipped operand order, same sends, silent again.
			steps[spec.name] = []patternFollowedByChain582StepPin{
				patternFollowedByChain582DeployPin("s0", spec.epls[0]),
				patternFollowedByChain582BeanSendPin("E1", 10),
				patternFollowedByChain582BeanSendPin("E2", 11),
				patternFollowedByChain582UndeployAllPin(),
				patternFollowedByChain582DeployPin("s0", spec.epls[1]),
				patternFollowedByChain582BeanSendPin("E1", 10),
				patternFollowedByChain582BeanSendPin("E2", 11),
				patternFollowedByChain582UndeployAllPin(),
			}
		}
	}
	return steps
}()

func loadPatternFollowedByChain582Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternFollowedByChain582ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternFollowedByChain582ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternFollowedByChain582ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternFollowedByChain582ID, err)
	}
	if err := requirePatternFollowedByChain582Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternFollowedByChain582ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternFollowedByChain582ID ||
		metadata.Description != patternFollowedByChain582Description ||
		metadata.JavaCommit != patternFollowedByChain582JavaCommit ||
		metadata.JavaSource != patternFollowedByChain582JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternFollowedByChain582ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternFollowedByChain582JavaSources},
		{metadata.JavaRuntimes, patternFollowedByChain582JavaRuntimeIDs},
		{metadata.JavaNames, patternFollowedByChain582JavaExecutions},
		{metadata.JavaStaticIDs, patternFollowedByChain582JavaStaticIDs},
		{metadata.JavaFlags, patternFollowedByChain582JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternFollowedByChain582ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternFollowedByChain582CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternFollowedByChain582ID, len(patternFollowedByChain582CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternFollowedByChain582Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternFollowedByChain582CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternFollowedByChain582JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternFollowedByChain582JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || !reflect.DeepEqual(definition.EPLs, spec.epls) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternFollowedByChain582ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternFollowedByChain582ID)
	}
	if err := validatePatternFollowedByChain582RawSteps(rawSteps); err != nil {
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

// validatePatternFollowedByChain582RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned per-case sequences (deploy EPLs and send payloads
// all pinned, including ord 8's second deploy inside the same case).
func validatePatternFollowedByChain582RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternFollowedByChain582CaseSpecs))
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
		}
		if operation == "case" {
			if err := requirePatternFollowedByChain582Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternFollowedByChain582CaseSteps[marker.Case]; !ok {
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
		pins := patternFollowedByChain582CaseSteps[currentCase]
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
			if err := requirePatternFollowedByChain582Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternFollowedByChain582Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternFollowedByChain582Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternFollowedByChain582CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternFollowedByChain582CaseSpecs))
	}
	for index, spec := range patternFollowedByChain582CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternFollowedByChain582CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternFollowedByChain582CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternFollowedByChain582Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternFollowedByChain582CaseState carries per-case replay state:
// the deployed statement, the per-deploy listener sequence counter and
// the delivery records. deployIndex tracks which of the case's pinned
// EPLs the next deploy step must carry (ord 8 deploys twice).
type patternFollowedByChain582CaseState struct {
	spec        patternFollowedByChain582CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	statements  map[string]*esper.Statement
	deployment  *esper.Deployment
	sequence    map[string]uint64
	records     []compat.TraceRecord
	deployIndex int
}

// runPatternFollowedByChain582Scenario replays the executions against a
// fresh engine per case like the Java oracle's fresh per-execution
// runtime — ord 8's two deploy phases stay inside that one engine,
// matching the single-run undeployAll semantics.
func runPatternFollowedByChain582Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternFollowedByChain582Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternFollowedByChain582CaseSpecs {
		records, err := runPatternFollowedByChain582Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternFollowedByChain582ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternFollowedByChain582Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternFollowedByChain582ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternFollowedByChain582ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternFollowedByChain582RawSteps(rawSteps)
}

func runPatternFollowedByChain582Case(ctx context.Context, scenario compat.Scenario, spec patternFollowedByChain582CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternFollowedByChain582SupportBean](env, patternFollowedByChain582SupportBeanType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByChain582BeanA](env, patternFollowedByChain582SupportBeanAType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByChain582BeanB](env, patternFollowedByChain582SupportBeanBType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByChain582BeanC](env, patternFollowedByChain582SupportBeanCType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternFollowedByChain582BeanD](env, patternFollowedByChain582SupportBeanDType); err != nil {
		return nil, err
	}
	state := &patternFollowedByChain582CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternFollowedByChain582JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			payload, err := patternFollowedByChain582DecodePayload(step)
			if err != nil {
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
				state.deployment = nil
			}
			state.statements = make(map[string]*esper.Statement)
			// The Java oracle attaches a fresh listener per deploy, so
			// the per-statement sequence restarts with each phase.
			state.sequence = make(map[string]uint64)
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternFollowedByChain582ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned EPL onto the fluent Go equivalent, mirroring
// internal/esper/pattern_followedby_parity_test.go:
//   - every-multiple (ord 7): PatternFrom(a,"a",true).Every().Then(b)
//     .Then(c).Then(d) — head-leg Every keeps one waiting branch per A,
//     so B1/C1/D1 complete both armed branches in one delivery
//   - filter-greater-then (ord 8): PatternFrom(sb,"a",true).Every()
//     .Then(PatternFrom(sb,"b",pred)) where pred correlates the second
//     leg against the captured a event — phase 1
//     LessOrEqual(Field intPrimitive, TagField a.intPrimitive), phase 2
//     GreaterOrEqual(TagField a.intPrimitive, Field intPrimitive); the
//     deploy position selects the operand order
//
// select * expands to Alias(tag, PatternEvent tag) for every bound tag —
// a/b/c/d (ord 7) and a/b (ord 8) — matching Esper's fragment
// projection.
func (s *patternFollowedByChain582CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" || s.deployIndex >= len(s.spec.epls) ||
		step.Epl != s.spec.epls[s.deployIndex] {
		return fmt.Errorf("%s: case %q deploy %d is not pinned", patternFollowedByChain582ID, s.spec.name, s.deployIndex)
	}
	if _, ok := s.statements[step.Statement]; ok {
		return fmt.Errorf("%s: label %q is already deployed", patternFollowedByChain582ID, step.Statement)
	}
	deployIndex := s.deployIndex
	var query esper.Query
	switch s.spec.name {
	case "every-multiple":
		a := esper.From[patternFollowedByChain582BeanA](s.env, patternFollowedByChain582SupportBeanAType)
		b := esper.From[patternFollowedByChain582BeanB](s.env, patternFollowedByChain582SupportBeanBType)
		c := esper.From[patternFollowedByChain582BeanC](s.env, patternFollowedByChain582SupportBeanCType)
		d := esper.From[patternFollowedByChain582BeanD](s.env, patternFollowedByChain582SupportBeanDType)
		query = esper.PatternFrom(a, "a", esper.Literal(true)).Every().
			Then(esper.PatternFrom(b, "b", esper.Literal(true))).
			Then(esper.PatternFrom(c, "c", esper.Literal(true))).
			Then(esper.PatternFrom(d, "d", esper.Literal(true))).
			Select(
				esper.Alias("a", esper.PatternEvent("a")),
				esper.Alias("b", esper.PatternEvent("b")),
				esper.Alias("c", esper.PatternEvent("c")),
				esper.Alias("d", esper.PatternEvent("d")),
			).Query(esper.StatementName("s0"))
	case "filter-greater-then":
		sb := esper.From[patternFollowedByChain582SupportBean](s.env, patternFollowedByChain582SupportBeanType)
		// The correlated second leg evaluates b's intPrimitive against
		// the captured a event's intPrimitive; the deploy index pins the
		// operand order (phase 1 b <= a, phase 2 a >= b).
		var predicate esper.Expression[bool]
		if deployIndex == 0 {
			predicate = esper.LessOrEqual[int](
				esper.Field[patternFollowedByChain582SupportBean, int]("intPrimitive"),
				esper.TagField[int]("a", "intPrimitive"))
		} else {
			predicate = esper.GreaterOrEqual[int](
				esper.TagField[int]("a", "intPrimitive"),
				esper.Field[patternFollowedByChain582SupportBean, int]("intPrimitive"))
		}
		query = esper.PatternFrom(sb, "a", esper.Literal(true)).Every().
			Then(esper.PatternFrom(sb, "b", predicate)).
			Select(
				esper.Alias("a", esper.PatternEvent("a")),
				esper.Alias("b", esper.PatternEvent("b")),
			).Query(esper.StatementName("s0"))
	default:
		return fmt.Errorf("%s: unsupported case %q", patternFollowedByChain582ID, s.spec.name)
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
	s.deployIndex++
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

// patternFollowedByChain582DecodePayload converts a scenario send
// payload into the typed host object: SupportBean carries the pinned
// theString/intPrimitive pair and SupportBean_A/B/C/D carry the single
// id.
func patternFollowedByChain582DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case patternFollowedByChain582SupportBeanType:
		if err := requirePatternFollowedByChain582Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var event patternFollowedByChain582SupportBean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return event, nil
	case patternFollowedByChain582SupportBeanAType:
		if err := requirePatternFollowedByChain582Fields(fields, "id"); err != nil {
			return nil, err
		}
		var event patternFollowedByChain582BeanA
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return event, nil
	case patternFollowedByChain582SupportBeanBType:
		if err := requirePatternFollowedByChain582Fields(fields, "id"); err != nil {
			return nil, err
		}
		var event patternFollowedByChain582BeanB
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_B: %w", err)
		}
		return event, nil
	case patternFollowedByChain582SupportBeanCType:
		if err := requirePatternFollowedByChain582Fields(fields, "id"); err != nil {
			return nil, err
		}
		var event patternFollowedByChain582BeanC
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_C: %w", err)
		}
		return event, nil
	case patternFollowedByChain582SupportBeanDType:
		if err := requirePatternFollowedByChain582Fields(fields, "id"); err != nil {
			return nil, err
		}
		var event patternFollowedByChain582BeanD
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_D: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternFollowedByChain582ID, step.EventType)
	}
}
