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

// Parity coverage for PatternOperatorEveryDistinct ords 0-2: the
// single-filter every-distinct slice — the distinct-key restart operator
// with and without per-key virtual-clock expiry — over SupportBean.
//
// Covered executions (all variant collection:executions(), flags []):
//   - ord 0 PatternEveryDistinctSimple java-runtime-593adaf9d26cd35ab4c9
//     (case every-distinct-simple): no clock; the first event per distinct
//     theString key fires c0, later duplicates are swallowed.
//   - ord 1 PatternEveryDistinctWTime java-runtime-666dd3af9524272914d9
//     (case every-distinct-w-time): 5-second per-key expiry measured from
//     each key's first sighting. The boundary is pinned: E1 first-seen at
//     t=15000 stays swallowed at t=19999 and fires again at t=20000 —
//     the key expires AT first-seen+5000, while E2 (first-seen t=18000)
//     stays swallowed past the boundary.
//   - ord 2 PatternExpireSeenBeforeKey java-runtime-904c2139b52a7eaeedfb
//     (case expire-seen-before-key): a theString like 'A%' filter with the
//     distinct key on intPrimitive and 1-second per-key expiry. select *
//     projects the captured tag; the replay projects `a` to the bean row
//     the Java assertions inspect through a.theString. A6@1000 is silent
//     (key 1 re-fires at 1000 exactly because it expired then); A7@1999
//     is swallowed and A7@2000 fires (key 2's 1000-ms window measured
//     from its own first sighting at t=0).
//
// Byte-exact EPL pins keep the asymmetry in the Java source verbatim:
// ords 0-1 use @Name('s0') (capital N) while ord 2 uses @name('s0')
// (lowercase); the annotation name is not normalized. Java milestone()
// savepoints restore identical state for these non-contextual executions
// and are omitted (573/575 precedent).
//
// Siblings excluded from this slice (manifest dispositions are
// main-agent owned): ord 3 PatternEveryDistinctOverFilter replays the
// same legs twice, the second with an eplToModelCompileDeploy tail
// (SODA/compile-text precedent); ord 14 PatternInvalid is
// compile-error-only; ord 15 PatternMonthScoped needs calendar-month
// expiry (EveryDistinctForCalendar — a different semantic); ords 4-13
// and 16 are compound-root/multi-key/special-type follow-on slices.
const patternEveryDistinct576ID = "pattern-everydistinct-576"

const patternEveryDistinct576Description = "PatternOperatorEveryDistinct ords 0-2 — the single-filter every-distinct slice over SupportBean. every-distinct-simple (PatternEveryDistinctSimple, ord 0) replays `@Name('s0') select a.theString as c0 from pattern [every-distinct(a.theString) a=SupportBean]` with no clock: E1 fires c0=E1, two E1 duplicates are swallowed, E2 fires c0=E2, the E1/E2 dup pairs stay silent and E3 fires c0=E3. every-distinct-w-time (PatternEveryDistinctWTime, ord 1) replays the same statement with `, 5 sec` expiry over the virtual clock: advanceTime(0) before deploy, then E1@15000 fires (dup swallowed); at t=18000 E1 stays swallowed and E2 fires; at t=19999 E1 still does not fire — the key expires AT first-seen+5000, so E1@20000 fires while E2 (first-seen 18000) stays swallowed, then the E1/E2 dups are silent and E3 fires c0=E3. expire-seen-before-key (PatternExpireSeenBeforeKey, ord 2) replays `@name('s0') select * from pattern [every-distinct(a.intPrimitive, 1 sec) a=SupportBean(theString like 'A%')]`: the key is intPrimitive, the filter gates on theString like 'A%' and select * delivers the captured bean; A1(1)/A3(2) fire at t=0, A4(1)/A5(2) are swallowed, then at t=1000 both keys expire so A4(1)/A5(2) fire, A6(1) is swallowed, A7(2)@1999 stays swallowed and A7(2)@2000 fires — per-key expiry measured from each key's first sighting. Statement names keep the Java asymmetry verbatim: @Name('s0') for ords 0-1 vs @name('s0') for ord 2; Java milestone() savepoints are omitted (they restore identical state for these non-contextual executions)."

const patternEveryDistinct576JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternEveryDistinct576JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java"

// Byte-exact EPL pins (PatternOperatorEveryDistinct.java lines 83, 124 and
// 176). The @Name/@name case asymmetry is verbatim Java source and must
// not be normalized.
const (
	patternEveryDistinct576SimpleEPL = "@Name('s0') select a.theString as c0 from pattern [every-distinct(a.theString) a=SupportBean]"
	patternEveryDistinct576WTimeEPL  = "@Name('s0') select a.theString as c0 from pattern [every-distinct(a.theString, 5 sec) a=SupportBean]"
	patternEveryDistinct576ExpiryEPL = "@name('s0') select * from pattern [every-distinct(a.intPrimitive, 1 sec) a=SupportBean(theString like 'A%')]"

	patternEveryDistinct576SupportBeanEvent = "SupportBean"

	patternEveryDistinct576TimeEpoch = "1970-01-01T00:00:00.000Z"
	patternEveryDistinct576Time15000 = "1970-01-01T00:00:15.000Z"
	patternEveryDistinct576Time18000 = "1970-01-01T00:00:18.000Z"
	patternEveryDistinct576Time19999 = "1970-01-01T00:00:19.999Z"
	patternEveryDistinct576Time20000 = "1970-01-01T00:00:20.000Z"
	patternEveryDistinct576Time1000  = "1970-01-01T00:00:01.000Z"
	patternEveryDistinct576Time1999  = "1970-01-01T00:00:01.999Z"
	patternEveryDistinct576Time2000  = "1970-01-01T00:00:02.000Z"
)

var (
	patternEveryDistinct576JavaRuntimeIDs = []string{
		"java-runtime-593adaf9d26cd35ab4c9",
		"java-runtime-666dd3af9524272914d9",
		"java-runtime-904c2139b52a7eaeedfb",
	}
	patternEveryDistinct576JavaSources = []string{
		patternEveryDistinct576JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
	}
	patternEveryDistinct576JavaExecutions = []string{
		"PatternEveryDistinctSimple",
		"PatternEveryDistinctWTime",
		"PatternExpireSeenBeforeKey",
	}
	// Deduplicated inventory id: the three runtime rows share static id
	// java-073091a0b42ca8dd974f with the other PatternOperatorEveryDistinct
	// executions, pinned once per runtimeId row.
	patternEveryDistinct576JavaStaticIDs = []string{
		"java-073091a0b42ca8dd974f",
		"java-073091a0b42ca8dd974f",
		"java-073091a0b42ca8dd974f",
	}
	patternEveryDistinct576JavaFlags = []string{}
)

// patternEveryDistinct576Bean mirrors the SupportBean properties the
// executions use: theString (the c0/property-path projection and the
// ord 0/ord 1 distinct key) and intPrimitive (the ord 2 distinct key).
type patternEveryDistinct576Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// patternEveryDistinct576CaseSpec pins one case: Java execution identity,
// the observation text, the byte-exact EPL script and the projections the
// listener records carry.
type patternEveryDistinct576CaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
}

var patternEveryDistinct576CaseSpecs = []patternEveryDistinct576CaseSpec{
	{
		name:        "every-distinct-simple",
		ordinal:     0,
		runtimeID:   "java-runtime-593adaf9d26cd35ab4c9",
		execution:   "PatternEveryDistinctSimple",
		observation: "listener; no clock; the first event per distinct theString key fires c0 and later duplicates are swallowed: E1 fires, E1 x2 silent, E2 fires, E1+E2 and E1+E2 silent, E3 fires",
		epl:         patternEveryDistinct576SimpleEPL,
	},
	{
		name:        "every-distinct-w-time",
		ordinal:     1,
		runtimeID:   "java-runtime-666dd3af9524272914d9",
		execution:   "PatternEveryDistinctWTime",
		observation: "listener; 5-second per-key expiry from first sighting over the virtual clock: E1@15000 fires (dup swallowed), E1@18000 swallowed and E2 fires, E1@19999 still swallowed (the key expires AT first-seen+5000=20000), E1@20000 fires while E2 (first-seen 18000) stays swallowed, E1/E2 dups silent, E3 fires c0=E3",
		epl:         patternEveryDistinct576WTimeEPL,
	},
	{
		name:        "expire-seen-before-key",
		ordinal:     2,
		runtimeID:   "java-runtime-904c2139b52a7eaeedfb",
		execution:   "PatternExpireSeenBeforeKey",
		observation: "listener; theString like 'A%' filter with the distinct key on intPrimitive and 1-second per-key expiry from each key's first sighting; select * projects the captured tag's bean (the a.theString the Java assertions pin): A1(1)/A3(2) fire at t=0, A4(1)/A5(2) swallowed, at t=1000 both keys expired so A4(1)/A5(2) fire, A6(1) silent, A7(2)@1999 swallowed, A7(2)@2000 fires",
		epl:         patternEveryDistinct576ExpiryEPL,
	},
}

// patternEveryDistinct576StepPin pins one scenario step's shape.
type patternEveryDistinct576StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	at        string
}

func patternEveryDistinct576AdvancePin(at string) patternEveryDistinct576StepPin {
	return patternEveryDistinct576StepPin{op: "advance-time", at: at}
}

func patternEveryDistinct576DeployPin(statement, epl string) patternEveryDistinct576StepPin {
	return patternEveryDistinct576StepPin{op: "deploy", statement: statement, epl: epl}
}

func patternEveryDistinct576SendPin(theString string, intPrimitive int) patternEveryDistinct576StepPin {
	return patternEveryDistinct576StepPin{
		op:        "send",
		eventType: patternEveryDistinct576SupportBeanEvent,
		payload:   map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)},
	}
}

func patternEveryDistinct576UndeployAllPin() patternEveryDistinct576StepPin {
	return patternEveryDistinct576StepPin{op: "undeploy-all"}
}

// patternEveryDistinct576CaseSteps pins the complete step sequence per
// case in Java source order (milestones omitted — regression-harness
// savepoints restoring identical state). Ord 0 carries no clock ops;
// ords 1-2 advance to t=0 before deploy exactly like the Java executions'
// env.advanceTime(0). The boundary advances (19999/20000 for the 5-second
// key, 1999/2000 for the 1-second key) pin expiry-at-first-seen+expiry.
var patternEveryDistinct576CaseSteps = map[string][]patternEveryDistinct576StepPin{
	"every-distinct-simple": {
		patternEveryDistinct576DeployPin("s0", patternEveryDistinct576SimpleEPL),
		patternEveryDistinct576SendPin("E1", 0),
		patternEveryDistinct576SendPin("E1", 0),
		patternEveryDistinct576SendPin("E1", 0),
		patternEveryDistinct576SendPin("E2", 0),
		patternEveryDistinct576SendPin("E1", 0),
		patternEveryDistinct576SendPin("E2", 0),
		patternEveryDistinct576SendPin("E1", 0),
		patternEveryDistinct576SendPin("E2", 0),
		patternEveryDistinct576SendPin("E3", 0),
		patternEveryDistinct576UndeployAllPin(),
	},
	"every-distinct-w-time": {
		patternEveryDistinct576AdvancePin(patternEveryDistinct576TimeEpoch),
		patternEveryDistinct576DeployPin("s0", patternEveryDistinct576WTimeEPL),
		patternEveryDistinct576AdvancePin(patternEveryDistinct576Time15000),
		patternEveryDistinct576SendPin("E1", 0),
		patternEveryDistinct576SendPin("E1", 0),
		patternEveryDistinct576AdvancePin(patternEveryDistinct576Time18000),
		patternEveryDistinct576SendPin("E1", 0),
		patternEveryDistinct576SendPin("E2", 0),
		patternEveryDistinct576AdvancePin(patternEveryDistinct576Time19999),
		patternEveryDistinct576SendPin("E1", 0),
		patternEveryDistinct576AdvancePin(patternEveryDistinct576Time20000),
		patternEveryDistinct576SendPin("E1", 0),
		patternEveryDistinct576SendPin("E2", 0),
		patternEveryDistinct576SendPin("E1", 0),
		patternEveryDistinct576SendPin("E2", 0),
		patternEveryDistinct576SendPin("E3", 0),
		patternEveryDistinct576UndeployAllPin(),
	},
	"expire-seen-before-key": {
		patternEveryDistinct576AdvancePin(patternEveryDistinct576TimeEpoch),
		patternEveryDistinct576DeployPin("s0", patternEveryDistinct576ExpiryEPL),
		patternEveryDistinct576SendPin("A1", 1),
		patternEveryDistinct576SendPin("A2", 1),
		patternEveryDistinct576SendPin("A3", 2),
		patternEveryDistinct576SendPin("A4", 1),
		patternEveryDistinct576SendPin("A5", 2),
		patternEveryDistinct576AdvancePin(patternEveryDistinct576Time1000),
		patternEveryDistinct576SendPin("A4", 1),
		patternEveryDistinct576SendPin("A5", 2),
		patternEveryDistinct576SendPin("A6", 1),
		patternEveryDistinct576AdvancePin(patternEveryDistinct576Time1999),
		patternEveryDistinct576SendPin("A7", 2),
		patternEveryDistinct576AdvancePin(patternEveryDistinct576Time2000),
		patternEveryDistinct576SendPin("A7", 2),
		patternEveryDistinct576UndeployAllPin(),
	},
}

func loadPatternEveryDistinct576Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternEveryDistinct576ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternEveryDistinct576ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternEveryDistinct576ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternEveryDistinct576ID, err)
	}
	if err := requirePatternEveryDistinct576Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternEveryDistinct576ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternEveryDistinct576ID ||
		metadata.Description != patternEveryDistinct576Description ||
		metadata.JavaCommit != patternEveryDistinct576JavaCommit ||
		metadata.JavaSource != patternEveryDistinct576JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternEveryDistinct576ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternEveryDistinct576JavaSources},
		{metadata.JavaRuntimes, patternEveryDistinct576JavaRuntimeIDs},
		{metadata.JavaNames, patternEveryDistinct576JavaExecutions},
		{metadata.JavaStaticIDs, patternEveryDistinct576JavaStaticIDs},
		{metadata.JavaFlags, patternEveryDistinct576JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternEveryDistinct576ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternEveryDistinct576CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternEveryDistinct576ID, len(patternEveryDistinct576CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternEveryDistinct576Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternEveryDistinct576CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID ||
			definition.ExecutionName != spec.execution ||
			definition.Observation != spec.observation || definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternEveryDistinct576ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternEveryDistinct576ID)
	}
	if err := validatePatternEveryDistinct576RawSteps(rawSteps); err != nil {
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

// validatePatternEveryDistinct576RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned per-case sequences.
func validatePatternEveryDistinct576RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternEveryDistinct576CaseSpecs))
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
			if err := requirePatternEveryDistinct576Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternEveryDistinct576CaseSteps[marker.Case]; !ok {
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
		pins := patternEveryDistinct576CaseSteps[currentCase]
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
			if err := requirePatternEveryDistinct576Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternEveryDistinct576Fields(object, "op", "case", "eventType", "payload"); err != nil {
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
			if err := requirePatternEveryDistinct576Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time is not pinned for case %q",
					index, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternEveryDistinct576Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternEveryDistinct576CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternEveryDistinct576CaseSpecs))
	}
	for index, spec := range patternEveryDistinct576CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternEveryDistinct576CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternEveryDistinct576CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternEveryDistinct576Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternEveryDistinct576CaseState carries per-case replay state: the
// deployed statement, the listener sequence counter, the current clock
// and the delivery records.
type patternEveryDistinct576CaseState struct {
	spec       patternEveryDistinct576CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	statements map[string]*esper.Statement
	deployment *esper.Deployment
	sequence   map[string]uint64
	records    []compat.TraceRecord
}

// runPatternEveryDistinct576Scenario replays the executions against a
// fresh engine per case like the Java oracle's per-execution runtime.
func runPatternEveryDistinct576Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternEveryDistinct576Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternEveryDistinct576CaseSpecs {
		records, err := runPatternEveryDistinct576Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternEveryDistinct576ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternEveryDistinct576Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternEveryDistinct576ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternEveryDistinct576ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternEveryDistinct576RawSteps(rawSteps)
}

func runPatternEveryDistinct576Case(ctx context.Context, scenario compat.Scenario, spec patternEveryDistinct576CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternEveryDistinct576Bean](env, patternEveryDistinct576SupportBeanEvent); err != nil {
		return nil, err
	}
	state := &patternEveryDistinct576CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(spec.runtimeID), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			payload, err := patternEveryDistinct576DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse advance-time %q: %w", patternEveryDistinct576ID, step.At, err)
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
			return nil, fmt.Errorf("%s: unsupported step op %q", patternEveryDistinct576ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned EPL onto the fluent Go equivalent, mirroring
// internal/esper/pattern_everydistinct_parity_test.go:
//   - ord 0: PatternFrom(a, true).EveryDistinct(Field theString) with
//     Alias(c0, a.theString)
//   - ord 1: EveryDistinctFor(5s, TagField a.theString) with the same c0
//     projection
//   - ord 2: PatternFrom(a, theString like 'A%').EveryDistinctFor(1s,
//     TagField a.intPrimitive) with select * expanding to the captured
//     tag's bean row Alias(a, PatternEvent a)
func (s *patternEveryDistinct576CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" || step.Epl != s.spec.epl {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternEveryDistinct576ID, s.spec.name)
	}
	if _, ok := s.statements[step.Statement]; ok {
		return fmt.Errorf("%s: label %q is already deployed", patternEveryDistinct576ID, step.Statement)
	}
	base := esper.From[patternEveryDistinct576Bean](s.env, patternEveryDistinct576SupportBeanEvent)
	var query esper.Query
	switch s.spec.name {
	case "every-distinct-simple":
		query = esper.PatternFrom(base, "a", esper.Literal(true)).
			EveryDistinct(esper.Field[patternEveryDistinct576Bean, string]("theString")).
			Select(esper.Alias("c0", esper.TagField[string]("a", "theString"))).
			Query(esper.StatementName("s0"))
	case "every-distinct-w-time":
		query = esper.PatternFrom(base, "a", esper.Literal(true)).
			EveryDistinctFor(5*time.Second, esper.TagField[string]("a", "theString")).
			Select(esper.Alias("c0", esper.TagField[string]("a", "theString"))).
			Query(esper.StatementName("s0"))
	case "expire-seen-before-key":
		predicate := esper.LikeOf(
			esper.Field[patternEveryDistinct576Bean, string]("theString"), esper.Literal("A%"))
		query = esper.PatternFrom(base, "a", predicate).
			EveryDistinctFor(time.Second, esper.TagField[int]("a", "intPrimitive")).
			Select(esper.Alias("a", esper.PatternEvent("a"))).
			Query(esper.StatementName("s0"))
	default:
		return fmt.Errorf("%s: unsupported case %q", patternEveryDistinct576ID, s.spec.name)
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

// patternEveryDistinct576DecodePayload converts a scenario send payload
// into the typed host object: a SupportBean struct with the pinned
// theString and intPrimitive fields.
func patternEveryDistinct576DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case patternEveryDistinct576SupportBeanEvent:
		if err := requirePatternEveryDistinct576Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean patternEveryDistinct576Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternEveryDistinct576ID, step.EventType)
	}
}
