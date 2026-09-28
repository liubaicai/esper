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

// Parity coverage for PatternOperatorFollowedBy ord 2
// PatternFollowedByTimer: ONE Java execution whose every-A followed-by
// fans persistent branches into a per-branch every-B leg carrying TWO
// correlated filter predicates — dest equality plus the closed range
// `startTime in [A.startTime:A.endTime]` — wrapped by a `where
// timer:within (7200000)` guard, and a statement-level `where B.source
// != A.source` post-filter outside the pattern brackets. The execution
// performs NO clock calls: no sendTimer/advanceTime/milestone, so the
// runtime clock stays at epoch and the ~83-day within guard never
// binds (bare numerics are SECONDS under the pattern observer's time
// abacus; the longest window is ~79s of payload event fields, which are
// bean properties, not engine time). The oracle runs with the internal
// timer disabled like the regression suite's deterministic runtime.
//
// Covered execution (variant collection:executions(), flags [],
// static-manifest id java-b96c718a6895cd0d80de — the per-execution
// discovery id; the deduplicated inventory id java-089b2086c9945dff918f
// shared by all ten PatternOperatorFollowedBy rows is an umbrella pin
// that the per-runtime javaStaticIds array does not carry when a
// per-execution id exists):
//   - ord 2 PatternFollowedByTimer java-runtime-4759bc801b8c0be6c10a
//     (case timer): three SupportCallEvent sends — e1 arms an every-A
//     branch (no fire), e2 completes it ({A:e1,B:e2}), e3 completes TWO
//     branches and is delivered as ONE listener invocation carrying TWO
//     rows [{A:e1,B:e3},{A:e2,B:e3}] — the Java assertion pins
//     getNewDataList().size()==1 AND getLastNewData().length==2.
//
// Byte-exact EPL pins keep the Java source verbatim: there is NO space
// between the closing `]` and the statement-level `where` ("]where"
// is one concatenated fragment), while `timer:within (7200000)` keeps
// its literal space inside the pattern guard, and
// `startTime in [A.startTime:A.endTime]` keeps its range spacing. The
// two `where` keywords live at different levels — the first is the
// B-leg pattern guard, the second the statement post-filter. The
// SupportCallEvent payload pins RELATIVE millisecond offsets because
// the Java source's dateToLong helper resolves the pinned datetimes via
// SimpleDateFormat in the default timezone — e1 0/41200, e2
// 24100/65400, e3 38100/78900 — which only matters relative to each
// other (long-millis closed-range comparisons and field rendering).
//
// select * projects BOTH bound tags as bean fragments (A and B); the
// Java asserts assertSame identity on the underlying beans, which the
// normalized trace renders as the full five-property SupportCallEvent
// row shape (callId/source/dest/startTime/endTime).
//
// Siblings excluded from this slice (manifest dispositions are
// main-agent owned): ord 0 (W-harness), ords 1/6/9 (timer+not trio,
// covered by pattern-followedby-timernot-581), ords 3/4/5 (RFID trio,
// covered by pattern-followedby-rfid-580) and ords 7/8 (no-timer
// chains, covered by pattern-followedby-chain-582) remain tracked in
// the capability manifest by main.
const patternFollowedByTimer584ID = "pattern-followedby-timer-584"

const patternFollowedByTimer584Description = "PatternOperatorFollowedBy ord 2 PatternFollowedByTimer — `every A=SupportCallEvent -> every B=SupportCallEvent(dest=A.dest, startTime in [A.startTime:A.endTime]) where timer:within (7200000)` plus the statement-level `where B.source != A.source` post-filter (byte-exact: `)]where` has NO space, `timer:within (7200000)` keeps its space). Three SupportCallEvent sends carry RELATIVE millisecond offsets (Java's dateToLong is timezone-dependent): e1(callId 2000002601, source 18, 0/41200) arms an every-A branch with no fire, e2(2000002607, source 20, 24100/65400) completes it — one record {A:e1,B:e2} — and e3(2000002610, source 22, 38100/78900) falls inside BOTH armed windows ([0:41200] and [24100:65400]), delivering ONE listener invocation carrying TWO rows [{A:e1,B:e3},{A:e2,B:e3}] (Java pins getNewDataList().size()==1 and getLastNewData().length==2). No clock ops: the timer:within guard never binds. select * projects both the A and B bean fragments (assertSame identity in Java; the five-property SupportCallEvent row shape in the trace)."

const patternFollowedByTimer584JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternFollowedByTimer584JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java"

// Byte-exact EPL pin (PatternOperatorFollowedBy.java, three-fragment
// concatenation verbatim): NO space joins "]" to the statement-level
// `where`, the pattern guard keeps `timer:within (7200000)` with its
// literal space, and the B filter keeps `dest=A.dest` plus
// `startTime in [A.startTime:A.endTime]` exactly as written.
const (
	patternFollowedByTimer584EPL = "@name('s0') select * from pattern " +
		"[every A=SupportCallEvent -> every B=SupportCallEvent(dest=A.dest, startTime in [A.startTime:A.endTime]) where timer:within (7200000)]" +
		"where B.source != A.source"

	patternFollowedByTimer584CallEventType = "SupportCallEvent"
)

// Per-case Java identity: the single timer case owns the ord 2
// runtimeId.
var (
	patternFollowedByTimer584JavaRuntimeIDs = []string{
		"java-runtime-4759bc801b8c0be6c10a",
	}
	patternFollowedByTimer584JavaSources = []string{
		patternFollowedByTimer584JavaSource,
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportCallEvent.java",
	}
	patternFollowedByTimer584JavaExecutions = []string{
		"PatternFollowedByTimer",
	}
	// Static-manifest id for the ord 2 execution (discovery
	// static-candidate). The deduplicated inventory id
	// java-089b2086c9945dff918f — shared by all ten
	// PatternOperatorFollowedBy rows — is NOT the static id here:
	// where a per-execution static id exists the per-runtime row
	// carries it (pattern-followedby-wharness-583 precedent).
	patternFollowedByTimer584JavaStaticIDs = []string{
		"java-b96c718a6895cd0d80de",
	}
	patternFollowedByTimer584JavaFlags = []string{}
)

// patternFollowedByTimer584CallEvent mirrors SupportCallEvent: a call
// with a callId, source/destination numbers and start/end timestamps in
// milliseconds. startTime/endTime carry the pinned RELATIVE offsets —
// Java's dateToLong resolves the source datetimes in the default
// timezone, so only the offsets are TZ-stable.
type patternFollowedByTimer584CallEvent struct {
	CallID    int64  `json:"callId" esper:"callId"`
	Source    string `json:"source" esper:"source"`
	Dest      string `json:"dest" esper:"dest"`
	StartTime int64  `json:"startTime" esper:"startTime"`
	EndTime   int64  `json:"endTime" esper:"endTime"`
}

// patternFollowedByTimer584CaseSpec pins the single case: Java
// execution identity (ordinal plus runtimeIndex), the observation text
// and the byte-exact EPL script. select * projects both bound tags —
// A and B — as bean fragments.
type patternFollowedByTimer584CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epl          string
}

var patternFollowedByTimer584CaseSpecs = []patternFollowedByTimer584CaseSpec{
	{
		name:         "timer",
		ordinal:      2,
		runtimeIndex: 0,
		observation: "listener; no clock ops (timer:within(7200000) never binds over the ~79s " +
			"payload span); `every A -> every B(dest=A.dest, startTime in [A.startTime:A.endTime]) " +
			"where timer:within (7200000)` + statement `where B.source != A.source`: e1 arms a branch, " +
			"e2 delivers ONE record {A:e1,B:e2}, e3 falls inside both windows and delivers ONE " +
			"invocation carrying TWO rows [{A:e1,B:e3},{A:e2,B:e3}] (Java pins getNewDataList().size()==1 " +
			"and getLastNewData().length==2); select * projects both A and B fragments",
		epl: patternFollowedByTimer584EPL,
	},
}

// patternFollowedByTimer584StepPin pins one scenario step's shape:
// deploys carry the byte-exact EPL and sends pin the full five-field
// SupportCallEvent payload. There are NO advance-time steps — the Java
// execution performs no clock calls.
type patternFollowedByTimer584StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
}

func patternFollowedByTimer584DeployPin(statement, epl string) patternFollowedByTimer584StepPin {
	return patternFollowedByTimer584StepPin{op: "deploy", statement: statement, epl: epl}
}

func patternFollowedByTimer584SendPin(eventType string, payload map[string]any) patternFollowedByTimer584StepPin {
	return patternFollowedByTimer584StepPin{op: "send", eventType: eventType, payload: payload}
}

// patternFollowedByTimer584CallSendPin pins one SupportCallEvent send:
// callId, source, dest and the startTime/endTime offsets in order.
func patternFollowedByTimer584CallSendPin(callID int64, source, dest string, startTime, endTime int64) patternFollowedByTimer584StepPin {
	return patternFollowedByTimer584SendPin(patternFollowedByTimer584CallEventType, map[string]any{
		"callId":    float64(callID),
		"source":    source,
		"dest":      dest,
		"startTime": float64(startTime),
		"endTime":   float64(endTime),
	})
}

func patternFollowedByTimer584UndeployAllPin() patternFollowedByTimer584StepPin {
	return patternFollowedByTimer584StepPin{op: "undeploy-all"}
}

// patternFollowedByTimer584CaseSteps pins the complete step sequence in
// Java source order: one deploy followed by the three SupportCallEvent
// sends (e1 arms, e2 fires once, e3 fires a two-row batch), then
// undeploy-all. Payload values are RELATIVE millisecond offsets — e1
// 0/41200, e2 24100/65400, e3 38100/78900 — mirroring the frozen
// dateToLong differences; dest is the shared "123456789014795" and the
// sources are the distinct "18"/"20"/"22".
var patternFollowedByTimer584CaseSteps = func() map[string][]patternFollowedByTimer584StepPin {
	steps := make(map[string][]patternFollowedByTimer584StepPin)
	for _, spec := range patternFollowedByTimer584CaseSpecs {
		steps[spec.name] = []patternFollowedByTimer584StepPin{
			patternFollowedByTimer584DeployPin("s0", spec.epl),
			patternFollowedByTimer584CallSendPin(2000002601, "18", "123456789014795", 0, 41200),
			patternFollowedByTimer584CallSendPin(2000002607, "20", "123456789014795", 24100, 65400),
			patternFollowedByTimer584CallSendPin(2000002610, "22", "123456789014795", 38100, 78900),
			patternFollowedByTimer584UndeployAllPin(),
		}
	}
	return steps
}()

func loadPatternFollowedByTimer584Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternFollowedByTimer584ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternFollowedByTimer584ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternFollowedByTimer584ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternFollowedByTimer584ID, err)
	}
	if err := requirePatternFollowedByTimer584Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternFollowedByTimer584ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternFollowedByTimer584ID ||
		metadata.Description != patternFollowedByTimer584Description ||
		metadata.JavaCommit != patternFollowedByTimer584JavaCommit ||
		metadata.JavaSource != patternFollowedByTimer584JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternFollowedByTimer584ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternFollowedByTimer584JavaSources},
		{metadata.JavaRuntimes, patternFollowedByTimer584JavaRuntimeIDs},
		{metadata.JavaNames, patternFollowedByTimer584JavaExecutions},
		{metadata.JavaStaticIDs, patternFollowedByTimer584JavaStaticIDs},
		{metadata.JavaFlags, patternFollowedByTimer584JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternFollowedByTimer584ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternFollowedByTimer584CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternFollowedByTimer584ID, len(patternFollowedByTimer584CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternFollowedByTimer584Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternFollowedByTimer584CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternFollowedByTimer584JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternFollowedByTimer584JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternFollowedByTimer584ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternFollowedByTimer584ID)
	}
	if err := validatePatternFollowedByTimer584RawSteps(rawSteps); err != nil {
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

// validatePatternFollowedByTimer584RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned per-case sequences — one s0 deploy with the
// verbatim EPL ("]where" no space, "timer:within (7200000)" spaced),
// three five-field SupportCallEvent sends and undeploy-all. There are
// no advance-time pins because the Java execution performs no clock
// calls.
func validatePatternFollowedByTimer584RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternFollowedByTimer584CaseSpecs))
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
			if err := requirePatternFollowedByTimer584Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternFollowedByTimer584CaseSteps[marker.Case]; !ok {
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
		pins := patternFollowedByTimer584CaseSteps[currentCase]
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
			if err := requirePatternFollowedByTimer584Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternFollowedByTimer584Fields(object, "op", "case", "eventType", "payload"); err != nil {
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
			if err := requirePatternFollowedByTimer584Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternFollowedByTimer584CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternFollowedByTimer584CaseSpecs))
	}
	for index, spec := range patternFollowedByTimer584CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternFollowedByTimer584CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternFollowedByTimer584CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternFollowedByTimer584Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternFollowedByTimer584CaseState carries per-case replay state: the
// deployed statement, the listener sequence counter and the delivery
// records.
type patternFollowedByTimer584CaseState struct {
	spec       patternFollowedByTimer584CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	statements map[string]*esper.Statement
	deployment *esper.Deployment
	sequence   map[string]uint64
	records    []compat.TraceRecord
}

// runPatternFollowedByTimer584Scenario replays the execution against a
// fresh engine per case like the Java oracle's fresh per-execution
// runtime. The engine clock stays at epoch — the scenario carries no
// advance-time steps and the ~83-day timer:within guard never binds.
func runPatternFollowedByTimer584Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternFollowedByTimer584Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternFollowedByTimer584CaseSpecs {
		records, err := runPatternFollowedByTimer584Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternFollowedByTimer584ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternFollowedByTimer584Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternFollowedByTimer584ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternFollowedByTimer584ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternFollowedByTimer584RawSteps(rawSteps)
}

func runPatternFollowedByTimer584Case(ctx context.Context, scenario compat.Scenario, spec patternFollowedByTimer584CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternFollowedByTimer584CallEvent](env, patternFollowedByTimer584CallEventType); err != nil {
		return nil, err
	}
	state := &patternFollowedByTimer584CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternFollowedByTimer584JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			payload, err := patternFollowedByTimer584DecodePayload(step)
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
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternFollowedByTimer584ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned EPL onto the fluent Go equivalent, mirroring
// internal/esper/pattern_followedby_parity_test.go's
// TestPatternFollowedByTimerMatchesEsper:
//
//	every A=SupportCallEvent ->
//	  every B=SupportCallEvent(dest=A.dest,
//	    startTime in [A.startTime:A.endTime]) where timer:within (7200000)
//
// maps to PatternFrom(call,"A",Literal(true)).Every().Then(
// PatternFrom(call,"B",And(Equal(dest, TagField A.dest),
// Between(startTime, TagField A.startTime, TagField A.endTime)))
// .Every().Within(7200000s)) — the `.Every().Within()` order wraps the
// timer:within` (the bare numeric is SECONDS under the pattern
// guard's time abacus). The `]where B.source != A.source` statement
// post-filter maps to PatternQuery.Where(NotEqual(B.source, A.source)),
// evaluated after match completion without touching pattern state.
//
// select * expands to Alias("A", PatternEvent "A") plus
// Alias("B", PatternEvent "B") — Esper projects every bound tag as a
// fragment with assertSame bean identity, which normalizes to the full
// five-property SupportCallEvent row shape.
func (s *patternFollowedByTimer584CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" || step.Epl != s.spec.epl {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternFollowedByTimer584ID, s.spec.name)
	}
	if _, ok := s.statements[step.Statement]; ok {
		return fmt.Errorf("%s: label %q is already deployed", patternFollowedByTimer584ID, step.Statement)
	}
	call := esper.From[patternFollowedByTimer584CallEvent](s.env, patternFollowedByTimer584CallEventType)
	pattern := esper.PatternFrom(call, "A", esper.Literal(true)).Every().Then(
		esper.PatternFrom(call, "B", esper.And(
			esper.Equal[string](
				esper.Field[patternFollowedByTimer584CallEvent, string]("dest"),
				esper.TagField[string]("A", "dest")),
			esper.Between[int64](
				esper.Field[patternFollowedByTimer584CallEvent, int64]("startTime"),
				esper.TagField[int64]("A", "startTime"),
				esper.TagField[int64]("A", "endTime")),
		)).Every().Within(7200000 * time.Second),
	)
	query := pattern.Select(
		esper.Alias("A", esper.PatternEvent("A")),
		esper.Alias("B", esper.PatternEvent("B")),
	).Where(
		esper.NotEqual[string](
			esper.TagField[string]("B", "source"),
			esper.TagField[string]("A", "source")),
	).Query(esper.StatementName("s0"))
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

// patternFollowedByTimer584DecodePayload converts a scenario send
// payload into the typed host object: SupportCallEvent carries the
// pinned callId/source/dest/startTime/endTime fields.
func patternFollowedByTimer584DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case patternFollowedByTimer584CallEventType:
		if err := requirePatternFollowedByTimer584Fields(fields,
			"callId", "source", "dest", "startTime", "endTime"); err != nil {
			return nil, err
		}
		var event patternFollowedByTimer584CallEvent
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportCallEvent: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternFollowedByTimer584ID, step.EventType)
	}
}
