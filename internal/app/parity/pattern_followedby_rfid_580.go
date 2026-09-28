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

// Parity coverage for PatternOperatorFollowedBy ords 3, 4 and 5: the RFID
// trio — every-then-followed-by with a per-branch correlated `and not`
// terminator — all over SupportRFIDEvent (mac, zoneID, locationReportId).
// No clock ops and no timer expiry inside the send windows; env.milestone
// savepoints in the Java executions are harness splits restoring identical
// state and are unrepresented.
//
// Covered executions (all variant collection:executions(), flags [],
// deduped static id java-089b2086c9945dff918f):
//   - ord 3 PatternMemoryRFIDEvent java-runtime-a477964502f64fe1b368
//     (case memory-rfid): `every tagMayBeBroken=SupportRFIDEvent ->
//     (timer:interval(10 sec) and not
//     SupportRFIDEvent(mac=tagMayBeBroken.mac))`. Twenty sends — ten
//     identical ("a","111") pairs — each repeat cancels the pending
//     same-mac branch before the 10-second timer can fire, so the
//     listener is NEVER invoked: the case emits ZERO listener records
//     (the Java assertion is the absence of the missing-heartbeat alert;
//     the RegressionEnvironment run does not even assert a fire count,
//     silence IS the contract).
//   - ord 4 PatternRFIDZoneExit java-runtime-f8ac45e337f93e276ac3 (case
//     zone-exit): `every a=SupportRFIDEvent(zoneID='1') -> (b=SupportRFIDEvent(
//     mac=a.mac,zoneID!='1') and not SupportRFIDEvent(mac=a.mac,zoneID='1'))`.
//     A zone-1 report arms an exit watch; a different-zone report completes
//     it unless a same-mac zone-1 report cancelled the branch first.
//     (a,1) arms silently, (a,2) fires, (b,1) arms, the duplicate (b,1)
//     cancels AND re-arms its own mac branch only, (b,2) fires — 2 rows.
//   - ord 5 PatternRFIDZoneEnter java-runtime-6a045f5ae813471b32e8 (case
//     zone-enter): `every a=SupportRFIDEvent(zoneID!='1') -> (b=SupportRFIDEvent(
//     mac=a.mac,zoneID='1') and not SupportRFIDEvent(mac=a.mac,zoneID=
//     a.zoneID))`. The not-conjunct zoneID is TagField(a,zoneID) — the
//     captured tag value — NOT a constant: the repeated (b,2) report
//     cancels the (b,2)-armed entry watch, so (b,1) fires only once after
//     re-arming — 2 rows.
//
// Byte-exact EPL pins keep the Java source verbatim: ord 3 uses the
// singular `timer:interval(10 sec)` and the alert/mac/zoneID projection;
// ords 4/5 use select * — which under Esper pattern semantics projects
// every bound tag as a fragment property, so the delivered rows carry
// both the `a` and `b` SupportRFIDEvent beans (the Java assertions pin
// the b-tagged row's identity).
//
// Siblings excluded from this slice (manifest dispositions are
// main-agent owned): ords 0 (W-harness), 1/2 (timer/guard variants),
// and 6-9 (followed-by chains/quiesce) remain open; ords are tracked in
// the capability manifest by main.
const patternFollowedByRFID580ID = "pattern-followedby-rfid-580"

const patternFollowedByRFID580Description = "PatternOperatorFollowedBy ords 3, 4 and 5 — the RFID trio: every-then followed-by with a per-branch correlated `and not` terminator over SupportRFIDEvent(mac,zoneID). memory-rfid (PatternMemoryRFIDEvent, ord 3) sends ten identical (a,111) pairs: each repeat cancels the pending same-mac branch before timer:interval(10 sec) can elapse, so the missing-heartbeat alert NEVER fires — zero listener records. zone-exit (PatternRFIDZoneExit, ord 4) arms an exit watch on zone-1 reports: (a,1) arms silently, (a,2) fires, (b,1) arms, the duplicate (b,1) cancels and re-arms only its own mac branch, (b,2) fires — 2 rows with per-mac branch isolation. zone-enter (PatternRFIDZoneEnter, ord 5) mirrors it with the not-conjunct keyed on TagField(a,zoneID) rather than a constant: (a,2) arms, (a,1) fires, (b,2) arms, the repeat (b,2) cancels the (b,2)-armed watch, (b,1) fires — 2 rows. select * on ords 4/5 projects every bound tag — both the a and b SupportRFIDEvent beans per Java fragment semantics. All milestone() savepoints are harness splits restoring identical state and are unrepresented; no clock ops exist (the 10-second timer never elapses)."

const patternFollowedByRFID580JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternFollowedByRFID580JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java"

// Byte-exact EPL pins (PatternOperatorFollowedBy.java, single-space
// concatenation verbatim): ord 3 keeps the singular `10 sec` and the
// alert/mac/zoneID projection; ord 4's not-branch carries the constant
// zoneID='1' while ord 5's carries the correlated zoneID=a.zoneID — the
// discriminant between the two executions.
const (
	patternFollowedByRFID580MemoryEPL    = "@name('s0') select 'Tag May Be Broken' as alert, tagMayBeBroken.mac, tagMayBeBroken.zoneID from pattern [every tagMayBeBroken=SupportRFIDEvent -> (timer:interval(10 sec) and not SupportRFIDEvent(mac=tagMayBeBroken.mac))]"
	patternFollowedByRFID580ZoneExitEPL  = "@name('s0') select * from pattern [every a=SupportRFIDEvent(zoneID='1') -> (b=SupportRFIDEvent(mac=a.mac,zoneID!='1') and not SupportRFIDEvent(mac=a.mac,zoneID='1'))]"
	patternFollowedByRFID580ZoneEnterEPL = "@name('s0') select * from pattern [every a=SupportRFIDEvent(zoneID!='1') -> (b=SupportRFIDEvent(mac=a.mac,zoneID='1') and not SupportRFIDEvent(mac=a.mac,zoneID=a.zoneID))]"

	patternFollowedByRFID580EventType = "SupportRFIDEvent"
)

var (
	// One javaRuntimes row per Java execution (one case per execution;
	// no multi-leg runtimeIndex sharing in this slice).
	patternFollowedByRFID580JavaRuntimeIDs = []string{
		"java-runtime-a477964502f64fe1b368",
		"java-runtime-f8ac45e337f93e276ac3",
		"java-runtime-6a045f5ae813471b32e8",
	}
	patternFollowedByRFID580JavaSources = []string{
		patternFollowedByRFID580JavaSource,
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportRFIDEvent.java",
	}
	patternFollowedByRFID580JavaExecutions = []string{
		"PatternMemoryRFIDEvent",
		"PatternRFIDZoneExit",
		"PatternRFIDZoneEnter",
	}
	// Deduplicated inventory id: all ten PatternOperatorFollowedBy
	// inventory rows share static id java-089b2086c9945dff918f (NOT the
	// every-distinct file's java-073091a0b42ca8dd974f), pinned once per
	// runtimeId row.
	patternFollowedByRFID580JavaStaticIDs = []string{
		"java-089b2086c9945dff918f",
		"java-089b2086c9945dff918f",
		"java-089b2086c9945dff918f",
	}
	patternFollowedByRFID580JavaFlags = []string{}
)

// patternFollowedByRFID580Event mirrors SupportRFIDEvent: the Java
// two-argument constructor sends (mac, zoneID) with locationReportId
// null, so the Go bean carries it as a nil *string — the Java bean's
// null property and the Go nil pointer both normalize to the
// {"state":"null"} marker under select *.
type patternFollowedByRFID580Event struct {
	LocationReportID *string `json:"locationReportId" esper:"locationReportId"`
	Mac              string  `json:"mac" esper:"mac"`
	ZoneID           string  `json:"zoneID" esper:"zoneID"`
}

// patternFollowedByRFID580CaseSpec pins one case: Java execution
// identity (ordinal plus runtimeIndex into the shared per-ord arrays),
// the observation text and the byte-exact EPL script. select * on
// ords 4/5 projects every bound tag's bean row — `a` and `b` — as
// fragment properties; ord 3's alert projection never materializes
// because the listener is never invoked.
type patternFollowedByRFID580CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epl          string
}

var patternFollowedByRFID580CaseSpecs = []patternFollowedByRFID580CaseSpec{
	{
		name:         "memory-rfid",
		ordinal:      3,
		runtimeIndex: 0,
		observation:  "listener never invoked; no clock ops; `every tagMayBeBroken -> (timer:interval(10 sec) and not SupportRFIDEvent(mac=tagMayBeBroken.mac))`: ten identical (a,111) pairs — each repeat cancels the pending same-mac branch before the timer elapses — ZERO listener records",
		epl:          patternFollowedByRFID580MemoryEPL,
	},
	{
		name:         "zone-exit",
		ordinal:      4,
		runtimeIndex: 1,
		observation:  "listener; no clock; `every a=(zoneID='1') -> (b=(mac=a.mac,zoneID!='1') and not (mac=a.mac,zoneID='1'))`: (a,1) arms silently, (a,2) fires, (b,1) arms, duplicate (b,1) cancels and re-arms only its own mac branch, (b,2) fires — 2 rows, select * delivers both a and b RFID fragments",
		epl:          patternFollowedByRFID580ZoneExitEPL,
	},
	{
		name:         "zone-enter",
		ordinal:      5,
		runtimeIndex: 2,
		observation:  "listener; no clock; `every a=(zoneID!='1') -> (b=(mac=a.mac,zoneID='1') and not (mac=a.mac,zoneID=a.zoneID))` — the not-conjunct is TagField(a,zoneID), not a constant: (a,2) arms, (a,1) fires, (b,2) arms, repeat (b,2) cancels the armed watch, (b,1) fires — 2 rows, select * delivers both a and b RFID fragments",
		epl:          patternFollowedByRFID580ZoneEnterEPL,
	},
}

// patternFollowedByRFID580StepPin pins one scenario step's shape.
type patternFollowedByRFID580StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
}

func patternFollowedByRFID580DeployPin(statement, epl string) patternFollowedByRFID580StepPin {
	return patternFollowedByRFID580StepPin{op: "deploy", statement: statement, epl: epl}
}

func patternFollowedByRFID580SendPin(mac, zoneID string) patternFollowedByRFID580StepPin {
	return patternFollowedByRFID580StepPin{
		op:        "send",
		eventType: patternFollowedByRFID580EventType,
		payload:   map[string]any{"mac": mac, "zoneID": zoneID},
	}
}

func patternFollowedByRFID580UndeployAllPin() patternFollowedByRFID580StepPin {
	return patternFollowedByRFID580StepPin{op: "undeploy-all"}
}

// patternFollowedByRFID580CaseSteps pins the complete step sequence per
// case in Java source order (env.milestone savepoints omitted — they
// restore identical state; ord 4 after sends 1/3, ord 5 after sends
// 1/4). Ord 3's Java loop sends ten identical ("a","111") pairs —
// pinned as twenty literal send steps; no advance-time ops exist (the
// 10-second timer is never given time to elapse).
var patternFollowedByRFID580CaseSteps = func() map[string][]patternFollowedByRFID580StepPin {
	steps := make(map[string][]patternFollowedByRFID580StepPin)
	memoryLeg := func(epl string) []patternFollowedByRFID580StepPin {
		// ord 3: ten identical ("a","111") pairs — 20 sends, 0 fires.
		pins := []patternFollowedByRFID580StepPin{patternFollowedByRFID580DeployPin("s0", epl)}
		for range 10 {
			pins = append(pins,
				patternFollowedByRFID580SendPin("a", "111"),
				patternFollowedByRFID580SendPin("a", "111"))
		}
		return append(pins, patternFollowedByRFID580UndeployAllPin())
	}
	zoneExitLeg := func(epl string) []patternFollowedByRFID580StepPin {
		// ord 4: (a,1) silent, (a,2) fires, (b,1) silent, milestone(0)
		// omitted, (b,1) silent dup-cancel, milestone(1) omitted,
		// (b,2) fires.
		return []patternFollowedByRFID580StepPin{
			patternFollowedByRFID580DeployPin("s0", epl),
			patternFollowedByRFID580SendPin("a", "1"),
			patternFollowedByRFID580SendPin("a", "2"),
			patternFollowedByRFID580SendPin("b", "1"),
			patternFollowedByRFID580SendPin("b", "1"),
			patternFollowedByRFID580SendPin("b", "2"),
			patternFollowedByRFID580UndeployAllPin(),
		}
	}
	zoneEnterLeg := func(epl string) []patternFollowedByRFID580StepPin {
		// ord 5: (a,2) silent, milestone(0) omitted, (a,1) fires, (b,2)
		// silent, (b,2) silent repeat-cancel (the discriminant),
		// milestone(1) omitted, (b,1) fires.
		return []patternFollowedByRFID580StepPin{
			patternFollowedByRFID580DeployPin("s0", epl),
			patternFollowedByRFID580SendPin("a", "2"),
			patternFollowedByRFID580SendPin("a", "1"),
			patternFollowedByRFID580SendPin("b", "2"),
			patternFollowedByRFID580SendPin("b", "2"),
			patternFollowedByRFID580SendPin("b", "1"),
			patternFollowedByRFID580UndeployAllPin(),
		}
	}
	for _, spec := range patternFollowedByRFID580CaseSpecs {
		switch spec.name {
		case "memory-rfid":
			steps[spec.name] = memoryLeg(spec.epl)
		case "zone-exit":
			steps[spec.name] = zoneExitLeg(spec.epl)
		case "zone-enter":
			steps[spec.name] = zoneEnterLeg(spec.epl)
		}
	}
	return steps
}()

func loadPatternFollowedByRFID580Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternFollowedByRFID580ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternFollowedByRFID580ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternFollowedByRFID580ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternFollowedByRFID580ID, err)
	}
	if err := requirePatternFollowedByRFID580Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternFollowedByRFID580ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternFollowedByRFID580ID ||
		metadata.Description != patternFollowedByRFID580Description ||
		metadata.JavaCommit != patternFollowedByRFID580JavaCommit ||
		metadata.JavaSource != patternFollowedByRFID580JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternFollowedByRFID580ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternFollowedByRFID580JavaSources},
		{metadata.JavaRuntimes, patternFollowedByRFID580JavaRuntimeIDs},
		{metadata.JavaNames, patternFollowedByRFID580JavaExecutions},
		{metadata.JavaStaticIDs, patternFollowedByRFID580JavaStaticIDs},
		{metadata.JavaFlags, patternFollowedByRFID580JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternFollowedByRFID580ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternFollowedByRFID580CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternFollowedByRFID580ID, len(patternFollowedByRFID580CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternFollowedByRFID580Fields(object, "case", "ordinal", "runtimeId",
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
		spec := patternFollowedByRFID580CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternFollowedByRFID580JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternFollowedByRFID580JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternFollowedByRFID580ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternFollowedByRFID580ID)
	}
	if err := validatePatternFollowedByRFID580RawSteps(rawSteps); err != nil {
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

// validatePatternFollowedByRFID580RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned per-case sequences.
func validatePatternFollowedByRFID580RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternFollowedByRFID580CaseSpecs))
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
			if err := requirePatternFollowedByRFID580Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternFollowedByRFID580CaseSteps[marker.Case]; !ok {
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
		pins := patternFollowedByRFID580CaseSteps[currentCase]
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
			if err := requirePatternFollowedByRFID580Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternFollowedByRFID580Fields(object, "op", "case", "eventType", "payload"); err != nil {
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
			if err := requirePatternFollowedByRFID580Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternFollowedByRFID580CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternFollowedByRFID580CaseSpecs))
	}
	for index, spec := range patternFollowedByRFID580CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternFollowedByRFID580CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternFollowedByRFID580CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternFollowedByRFID580Fields(object map[string]json.RawMessage, names ...string) error {
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

// patternFollowedByRFID580CaseState carries per-case replay state: the
// deployed statement, the listener sequence counter and the delivery
// records.
type patternFollowedByRFID580CaseState struct {
	spec       patternFollowedByRFID580CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	statements map[string]*esper.Statement
	deployment *esper.Deployment
	sequence   map[string]uint64
	records    []compat.TraceRecord
}

// runPatternFollowedByRFID580Scenario replays the executions against a
// fresh engine per case like the Java oracle's fresh per-execution
// runtime.
func runPatternFollowedByRFID580Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternFollowedByRFID580Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternFollowedByRFID580CaseSpecs {
		records, err := runPatternFollowedByRFID580Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternFollowedByRFID580ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternFollowedByRFID580Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternFollowedByRFID580ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternFollowedByRFID580ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternFollowedByRFID580RawSteps(rawSteps)
}

func runPatternFollowedByRFID580Case(ctx context.Context, scenario compat.Scenario, spec patternFollowedByRFID580CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternFollowedByRFID580Event](env, patternFollowedByRFID580EventType); err != nil {
		return nil, err
	}
	state := &patternFollowedByRFID580CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(patternFollowedByRFID580JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
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
			payload, err := patternFollowedByRFID580DecodePayload(step)
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
			return nil, fmt.Errorf("%s: unsupported step op %q", patternFollowedByRFID580ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned EPL onto the fluent Go equivalent, mirroring
// internal/esper/pattern_followedby_parity_test.go:
//   - memory-rfid (ord 3): PatternFrom(rfid,"tagMayBeBroken",true).Every()
//     .Then(TimerInterval(rfid,10s).And(PatternFrom(rfid,"n",
//     mac==TagField(tagMayBeBroken,mac)).Not())) — the not-branch carries
//     no tag (the EPL leaves it unnamed) — with the byte-exact
//     alert/tagMayBeBroken.mac/tagMayBeBroken.zoneID projection
//   - zone-exit (ord 4): a.Every().Then(b.And(n.Not())) where b's filter
//     is mac==a.mac AND zoneID!='1' and n's is mac==a.mac AND
//     zoneID=='1' — both constant '1' comparisons
//   - zone-enter (ord 5): same shape with a=zoneID!='1', b=zoneID=='1',
//     and n=zoneID==TagField(a,"zoneID") — the captured-tag correlation,
//     NOT a constant
//
// select * on ords 4/5 expands to Alias(tag, PatternEvent tag) per bound
// tag — `a` and `b` — matching Esper's fragment projection of every
// bound tag.
func (s *patternFollowedByRFID580CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" || step.Epl != s.spec.epl {
		return fmt.Errorf("%s: case %q deploy is not pinned", patternFollowedByRFID580ID, s.spec.name)
	}
	if _, ok := s.statements[step.Statement]; ok {
		return fmt.Errorf("%s: label %q is already deployed", patternFollowedByRFID580ID, step.Statement)
	}
	rfid := esper.From[patternFollowedByRFID580Event](s.env, patternFollowedByRFID580EventType)
	macField := esper.Field[patternFollowedByRFID580Event, string]("mac")
	zoneField := esper.Field[patternFollowedByRFID580Event, string]("zoneID")
	var query esper.Query
	switch s.spec.name {
	case "memory-rfid":
		query = esper.PatternFrom(rfid, "tagMayBeBroken", esper.Literal(true)).Every().Then(
			esper.TimerInterval(rfid, 10*time.Second).And(
				esper.PatternFrom(rfid, "n", esper.Equal[string](macField, esper.TagField[string]("tagMayBeBroken", "mac"))).Not()),
		).Select(
			esper.Alias("alert", esper.Literal("Tag May Be Broken")),
			esper.Alias("tagMayBeBroken.mac", esper.TagField[string]("tagMayBeBroken", "mac")),
			esper.Alias("tagMayBeBroken.zoneID", esper.TagField[string]("tagMayBeBroken", "zoneID")),
		).Query(esper.StatementName("s0"))
	case "zone-exit":
		aTag := esper.PatternFrom(rfid, "a", esper.Equal[string](zoneField, esper.Literal("1")))
		bTag := esper.PatternFrom(rfid, "b", esper.And(
			esper.Equal[string](macField, esper.TagField[string]("a", "mac")),
			esper.NotEqual[string](zoneField, esper.Literal("1")),
		))
		nTag := esper.PatternFrom(rfid, "n", esper.And(
			esper.Equal[string](macField, esper.TagField[string]("a", "mac")),
			esper.Equal[string](zoneField, esper.Literal("1")),
		))
		query = aTag.Every().Then(bTag.And(nTag.Not())).Select(
			esper.Alias("a", esper.PatternEvent("a")),
			esper.Alias("b", esper.PatternEvent("b")),
		).Query(esper.StatementName("s0"))
	case "zone-enter":
		aTag := esper.PatternFrom(rfid, "a", esper.NotEqual[string](zoneField, esper.Literal("1")))
		bTag := esper.PatternFrom(rfid, "b", esper.And(
			esper.Equal[string](macField, esper.TagField[string]("a", "mac")),
			esper.Equal[string](zoneField, esper.Literal("1")),
		))
		nTag := esper.PatternFrom(rfid, "n", esper.And(
			esper.Equal[string](macField, esper.TagField[string]("a", "mac")),
			esper.Equal[string](zoneField, esper.TagField[string]("a", "zoneID")),
		))
		query = aTag.Every().Then(bTag.And(nTag.Not())).Select(
			esper.Alias("a", esper.PatternEvent("a")),
			esper.Alias("b", esper.PatternEvent("b")),
		).Query(esper.StatementName("s0"))
	default:
		return fmt.Errorf("%s: unsupported case %q", patternFollowedByRFID580ID, s.spec.name)
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

// patternFollowedByRFID580DecodePayload converts a scenario send payload
// into the typed host object: a SupportRFIDEvent struct with the pinned
// mac and zoneID fields (locationReportId stays nil like the Java
// two-argument constructor's null).
func patternFollowedByRFID580DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case patternFollowedByRFID580EventType:
		if err := requirePatternFollowedByRFID580Fields(fields, "mac", "zoneID"); err != nil {
			return nil, err
		}
		var event patternFollowedByRFID580Event
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportRFIDEvent: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternFollowedByRFID580ID, step.EventType)
	}
}
