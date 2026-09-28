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

// Parity coverage for PatternObserverTimerInterval ord 7
// (PatternIntervalSpecExpressionWithPropertyArray,
// java-runtime-6155a2b0181e2169f2a4, static java-1422565b568236b2bfea,
// flags []): the interval spec reads the repeated-tag "property array"
// `a` produced by `[2] a=SupportBean` — a[0].intPrimitive +
// a[1].intPrimitive seconds evaluated once when the repeat completes
// (TimerIntervalObserverFactory.computeDelta(beginState)), not at
// deploy and not per-tick. The deadline is strictly-after arming:
// E1+E2 at t=10000ms arms a 5000ms observer, t=14999 is silent and
// t=15000 fires exactly one row projecting the two tag ids.
const patternTimerIntervalPropertyArray593ID = "pattern-timer-interval-property-array-593"

const patternTimerIntervalPropertyArray593Description = "PatternObserverTimerInterval ord 7 (PatternIntervalSpecExpressionWithPropertyArray) — timer:interval with a duration expression over the repeated-tag array `[2] a=SupportBean`: advanceTime(0), deploy `@name('s0') select a[0].theString as a0id, a[1].theString as a1id from pattern [ [2] a=SupportBean -> timer:interval(a[0].intPrimitive+a[1].intPrimitive seconds)]`, advanceTime(10000), send SupportBean E1 (intPrimitive=3 — first element captured, observer NOT armed), send SupportBean E2 (intPrimitive=2 — repeat completes, arms 3+2=5s once at the completion instant), advanceTime(14999) silent, milestone(0) savepoint (no step), advanceTime(15000) fires one row {a0id:E1,a1id:E2}, undeployAll. One-shot: no `every`, so the followed-by match completes and the pattern terminates."

const patternTimerIntervalPropertyArray593JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternTimerIntervalPropertyArray593JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternObserverTimerInterval.java"

// Byte-exact EPL (PatternObserverTimerInterval.java:319 verbatim —
// note the space after `pattern [`).
const patternTimerIntervalPropertyArray593EPL = "@name('s0') select a[0].theString as a0id, a[1].theString as a1id from pattern [ [2] a=SupportBean -> timer:interval(a[0].intPrimitive+a[1].intPrimitive seconds)]"

// Pinned instants: arm at the epoch, send both beans at t=10000ms, the
// 14999ms silent probe and the 15000ms firing advance.
const (
	patternTimerIntervalPropertyArray593Arm    = "1970-01-01T00:00:00Z"
	patternTimerIntervalPropertyArray593Send   = "1970-01-01T00:00:10Z"
	patternTimerIntervalPropertyArray593Silent = "1970-01-01T00:00:14.999Z"
	patternTimerIntervalPropertyArray593Fire   = "1970-01-01T00:00:15Z"
)

const patternTimerIntervalPropertyArray593Case = "property-array"

var (
	patternTimerIntervalPropertyArray593JavaRuntimeIDs = []string{
		"java-runtime-6155a2b0181e2169f2a4",
	}
	patternTimerIntervalPropertyArray593JavaExecutions = []string{
		"PatternIntervalSpecExpressionWithPropertyArray",
	}
	patternTimerIntervalPropertyArray593JavaStaticIDs = []string{
		"java-1422565b568236b2bfea",
	}
	patternTimerIntervalPropertyArray593JavaSources = []string{
		patternTimerIntervalPropertyArray593JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
	}
	patternTimerIntervalPropertyArray593JavaFlags = []string{}
)

// patternTimerIntervalPropertyArray593Bean is the SupportBean shape —
// theString + intPrimitive, constructed E1(3) then E2(2).
type patternTimerIntervalPropertyArray593Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

const patternTimerIntervalPropertyArray593BeanType = "SupportBean"

// patternTimerIntervalPropertyArray593StepPin pins one scenario step:
// deploy steps carry the byte-exact EPL, advance-time steps the pinned
// instant, send steps the instant (advance-before-send, like the W
// harness) plus the bean payload.
type patternTimerIntervalPropertyArray593StepPin struct {
	op        string
	statement string
	epl       string
	at        string
	payload   map[string]any
}

// patternTimerIntervalPropertyArray593Steps pins the full ordered
// sequence: arm, deploy s0, E1 then E2 at t=10000, the silent probe,
// the firing advance and undeploy-all.
var patternTimerIntervalPropertyArray593Steps = []patternTimerIntervalPropertyArray593StepPin{
	{op: "advance-time", at: patternTimerIntervalPropertyArray593Arm},
	{op: "deploy", statement: "s0", epl: patternTimerIntervalPropertyArray593EPL},
	{op: "send", at: patternTimerIntervalPropertyArray593Send, payload: map[string]any{"theString": "E1", "intPrimitive": 3}},
	{op: "send", at: patternTimerIntervalPropertyArray593Send, payload: map[string]any{"theString": "E2", "intPrimitive": 2}},
	{op: "advance-time", at: patternTimerIntervalPropertyArray593Silent},
	{op: "advance-time", at: patternTimerIntervalPropertyArray593Fire},
	{op: "undeploy-all"},
}

func loadPatternTimerIntervalPropertyArray593Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternTimerIntervalPropertyArray593ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternTimerIntervalPropertyArray593ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternTimerIntervalPropertyArray593ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternTimerIntervalPropertyArray593ID, err)
	}
	if err := requirePatternTimerIntervalPropertyArray593Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternTimerIntervalPropertyArray593ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternTimerIntervalPropertyArray593ID ||
		metadata.Description != patternTimerIntervalPropertyArray593Description ||
		metadata.JavaCommit != patternTimerIntervalPropertyArray593JavaCommit ||
		metadata.JavaSource != patternTimerIntervalPropertyArray593JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternTimerIntervalPropertyArray593ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternTimerIntervalPropertyArray593JavaSources},
		{metadata.JavaRuntimes, patternTimerIntervalPropertyArray593JavaRuntimeIDs},
		{metadata.JavaNames, patternTimerIntervalPropertyArray593JavaExecutions},
		{metadata.JavaStaticIDs, patternTimerIntervalPropertyArray593JavaStaticIDs},
		{metadata.JavaFlags, patternTimerIntervalPropertyArray593JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternTimerIntervalPropertyArray593ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != 1 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly one case", patternTimerIntervalPropertyArray593ID)
	}
	var caseObject map[string]json.RawMessage
	if err := strictObject(rawCases[0], &caseObject); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	if err := requirePatternTimerIntervalPropertyArray593Fields(caseObject, "case", "ordinal", "runtimeId",
		"executionName", "observation", "epls"); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	var definition struct {
		Case          string   `json:"case"`
		Ordinal       int      `json:"ordinal"`
		RuntimeID     string   `json:"runtimeId"`
		ExecutionName string   `json:"executionName"`
		Observation   string   `json:"observation"`
		EPLs          []string `json:"epls"`
	}
	if err := json.Unmarshal(rawCases[0], &definition); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	if definition.Case != patternTimerIntervalPropertyArray593Case || definition.Ordinal != 7 ||
		definition.RuntimeID != patternTimerIntervalPropertyArray593JavaRuntimeIDs[0] ||
		definition.ExecutionName != patternTimerIntervalPropertyArray593JavaExecutions[0] ||
		definition.Observation != "dynamic-interval" ||
		!reflect.DeepEqual(definition.EPLs, []string{patternTimerIntervalPropertyArray593EPL}) {
		return compat.Scenario{}, fmt.Errorf("%s scenario case metadata is not pinned", patternTimerIntervalPropertyArray593ID)
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternTimerIntervalPropertyArray593ID)
	}
	if err := validatePatternTimerIntervalPropertyArray593RawSteps(rawSteps); err != nil {
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

// validatePatternTimerIntervalPropertyArray593RawSteps pins the ordered
// step sequence positionally: the arm advance, the s0 deploy carrying
// the byte-exact EPL, the two send steps at t=10000 with their pinned
// SupportBean payloads, the 14999 silent probe, the 15000 firing
// advance and undeploy-all. The milestone(0) savepoint carries no step.
func validatePatternTimerIntervalPropertyArray593RawSteps(rawSteps []json.RawMessage) error {
	position := 0
	seenCase := false
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
			EventType string          `json:"eventType"`
			Payload   json.RawMessage `json:"payload"`
		}
		if operation == "case" {
			if err := requirePatternTimerIntervalPropertyArray593Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if marker.Case != patternTimerIntervalPropertyArray593Case || seenCase {
				return fmt.Errorf("scenario step %d selects unexpected case %q", index, marker.Case)
			}
			seenCase = true
			continue
		}
		if !seenCase {
			return fmt.Errorf("scenario step %d is outside the case block", index)
		}
		if err := json.Unmarshal(rawStep, &step); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		if step.Case != patternTimerIntervalPropertyArray593Case {
			return fmt.Errorf("scenario step %d declares case %q", index, step.Case)
		}
		if position >= len(patternTimerIntervalPropertyArray593Steps) {
			return fmt.Errorf("scenario step %d exceeds the pinned step sequence", index)
		}
		pin := patternTimerIntervalPropertyArray593Steps[position]
		if operation != pin.op {
			return fmt.Errorf("scenario step %d op %q is not the pinned %q at position %d",
				index, operation, pin.op, position)
		}
		switch operation {
		case "deploy":
			if err := requirePatternTimerIntervalPropertyArray593Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned", index)
			}
		case "advance-time":
			if err := requirePatternTimerIntervalPropertyArray593Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time %q is not pinned", index, step.At)
			}
		case "send":
			if err := requirePatternTimerIntervalPropertyArray593Fields(object, "op", "case", "at", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at || step.EventType != patternTimerIntervalPropertyArray593BeanType {
				return fmt.Errorf("scenario step %d send is not pinned", index)
			}
			var payload patternTimerIntervalPropertyArray593Bean
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d send payload: %w", index, err)
			}
			if payload.TheString != pin.payload["theString"] || payload.IntPrimitive != pin.payload["intPrimitive"] {
				return fmt.Errorf("scenario step %d send payload is not pinned", index)
			}
		case "undeploy-all":
			if err := requirePatternTimerIntervalPropertyArray593Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		position++
	}
	if !seenCase || position != len(patternTimerIntervalPropertyArray593Steps) {
		return fmt.Errorf("scenario has %d pinned steps, want %d", position, len(patternTimerIntervalPropertyArray593Steps))
	}
	return nil
}

func requirePatternTimerIntervalPropertyArray593Fields(object map[string]json.RawMessage, names ...string) error {
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

// runPatternTimerIntervalPropertyArray593Scenario replays the execution
// on a fresh engine like the Java oracle's fresh per-execution runtime:
// send steps advance the external clock to their pinned instant before
// delivering the bean (advance-before-send, the W-harness convention),
// and the listener records deliveries.
func runPatternTimerIntervalPropertyArray593Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternTimerIntervalPropertyArray593Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternTimerIntervalPropertyArray593Bean](env, patternTimerIntervalPropertyArray593BeanType); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(patternTimerIntervalPropertyArray593JavaRuntimeIDs[0]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	var deployment *esper.Deployment
	var records []compat.TraceRecord
	var sequence uint64
	for _, step := range scenario.Steps {
		switch step.Op {
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("%s: parse advance-time %q: %w", patternTimerIntervalPropertyArray593ID, step.At, err)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return compat.Trace{}, err
			}
		case "deploy":
			query, err := patternTimerIntervalPropertyArray593Query(env)
			if err != nil {
				return compat.Trace{}, err
			}
			plan, err := env.Build(query)
			if err != nil {
				return compat.Trace{}, err
			}
			deployment, err = engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, err
			}
			statements := deployment.Statements()
			if len(statements) != 1 || statements[0].Name() != "s0" {
				return compat.Trace{}, fmt.Errorf("%s: expected one s0 statement", patternTimerIntervalPropertyArray593ID)
			}
			statement := statements[0]
			if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				sequence++
				records = append(records, compat.TraceRecord{
					Case:      patternTimerIntervalPropertyArray593Case,
					Operation: "listener",
					Statement: statement.Name(),
					Sequence:  sequence,
					Time:      compat.FormatTraceTime(batch.Time),
					New:       compat.NormalizeResults(batch.New),
					Old:       compat.NormalizeResults(batch.Old),
				})
				return nil
			}); err != nil {
				return compat.Trace{}, err
			}
		case "send":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("%s: parse send at %q: %w", patternTimerIntervalPropertyArray593ID, step.At, err)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return compat.Trace{}, err
			}
			var bean patternTimerIntervalPropertyArray593Bean
			if err := json.Unmarshal(step.Payload, &bean); err != nil {
				return compat.Trace{}, fmt.Errorf("%s: decode SupportBean: %w", patternTimerIntervalPropertyArray593ID, err)
			}
			if err := engine.SendEvent(ctx, bean); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			if deployment != nil {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, err
				}
				deployment = nil
			}
		case "case":
		default:
			return compat.Trace{}, fmt.Errorf("%s: unsupported step op %q", patternTimerIntervalPropertyArray593ID, step.Op)
		}
	}
	return compat.Trace{Version: scenario.Version, ID: scenario.ID, Records: records}, nil
}

func validatePatternTimerIntervalPropertyArray593Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternTimerIntervalPropertyArray593ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternTimerIntervalPropertyArray593ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternTimerIntervalPropertyArray593RawSteps(rawSteps)
}

// patternTimerIntervalPropertyArray593Query builds the fluent plan:
// `[2] a=SupportBean` is MatchUntil(2,2) over the tagged stream and
// `a[0].intPrimitive+a[1].intPrimitive seconds` is DurationSeconds over
// the Add of the two repeated-tag fields, evaluated once when the
// repeat completes.
func patternTimerIntervalPropertyArray593Query(env *esper.Environment) (esper.Query, error) {
	base := esper.From[patternTimerIntervalPropertyArray593Bean](env, patternTimerIntervalPropertyArray593BeanType)
	repeated := esper.PatternFrom(base, "a", esper.Literal[bool](true)).MatchUntil(2, 2)
	pattern := repeated.Then(esper.TimerIntervalExpr(base, esper.DurationSeconds[int](
		esper.Add[int](
			esper.TagFieldAt[int]("a", 0, "intPrimitive"),
			esper.TagFieldAt[int]("a", 1, "intPrimitive"),
		),
	)))
	return pattern.Select(
		esper.Alias("a0id", esper.TagFieldAt[string]("a", 0, "theString")),
		esper.Alias("a1id", esper.TagFieldAt[string]("a", 1, "theString")),
	).Query(esper.StatementName("s0")), nil
}
