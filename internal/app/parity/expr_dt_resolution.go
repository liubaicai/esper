package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// expr_dt_resolution.go replays ExprDTResolution's two executions under both
// engine time resolutions against the pinned Java oracle:
//
//   - event-time-ms / event-time-us (ord 0, ExprDTResolutionEventTime): the
//     object-array MyEvent(id,sts,ets) type declares sts/ets as the start/end
//     timestamp properties, so a.withDate(2002, 4, 30) rewrites the event's
//     interval bounds (start -> 2002-05-30 preserving time-of-day, end shifted
//     by the same delta) and before(b) is the strict leftEnd < rightStart.
//     B@t seeds #lastevent without evaluating (unidirectional), A@flip-1
//     fires, A@flip does not — the boundary is exactly one engine time unit
//     (1 ms vs 1 µs), which is what proves the resolution. Every send emits
//     one trace record: a listener row on fire, a listener-not-invoked count
//     otherwise.
//   - long-property-ms / long-property-us (ord 1, ExprDTLongProperty): one
//     SupportDateTime send emits c0..c7. Long-returning calendar ops preserve
//     the sub-millisecond µs remainder (c0 keeps +123), toCalendar()/toDate()
//     truncate to ms (c1/c5/c6 are byte-identical across resolutions), and
//     minus(1) is one MILLISECOND regardless of resolution (c7 = t-1 ms /
//     t*1000-1000 µs).

const exprDTResolutionID = "expr-dt-resolution"
const exprDTResolutionJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprDTResolutionJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTResolution.java",
}

var exprDTResolutionJavaRuntimeIDs = []string{
	"java-runtime-28a96cc89bb103f3750a",
	"java-runtime-deee6eb6aef5c6bfa2a7",
	"java-runtime-35ef58e0d7badc88b6a8",
	"java-runtime-81d2223362062b76b0a5",
}

var exprDTResolutionJavaExecutions = []string{
	"ExprDTResolutionEventTime",
	"ExprDTResolutionEventTime",
	"ExprDTLongProperty",
	"ExprDTLongProperty",
}

var exprDTResolutionCases = []string{
	"event-time-ms",
	"event-time-us",
	"long-property-ms",
	"long-property-us",
}

var exprDTResolutionCaseRuntimeIDs = map[string]string{
	"event-time-ms":    "java-runtime-28a96cc89bb103f3750a",
	"event-time-us":    "java-runtime-deee6eb6aef5c6bfa2a7",
	"long-property-ms": "java-runtime-35ef58e0d7badc88b6a8",
	"long-property-us": "java-runtime-81d2223362062b76b0a5",
}

const exprDTResolutionDescription = "ExprDTResolution executions(isMicrosecond) under both engine time " +
	"resolutions: event-time-ms/us replay ExprDTResolutionEventTime (object-array MyEvent(id,sts,ets) " +
	"with sts/ets timestamp properties; a.withDate(2002,4,30).before(b) fires iff the shifted end is " +
	"strictly before b's start — the boundary is one engine unit, 1 ms vs 1 µs), long-property-ms/us " +
	"replay ExprDTLongProperty (c0..c7 over longdate and current_timestamp: µs long ops preserve the " +
	"sub-ms remainder, toCalendar/toDate truncate to ms, minus(1) is one millisecond in both modes)."

var exprDTResolutionJavaStaticIDs = []string{
	"java-f30ce9b974beba2c58e8",
	"java-f30ce9b974beba2c58e8",
	"java-55fcf32b326f5e318713",
	"java-55fcf32b326f5e318713",
}

var exprDTResolutionJavaFlags = []string{}

var exprDTResolutionOrdinals = []int{0, 0, 1, 1}

var exprDTResolutionCaseObservations = []string{
	"listener+count; advanceTime(0), B@t seeds #lastevent without evaluating (unidirectional), " +
		"A@flip-1 fires the join row (a,b fragments), A@flip does not — the strict before boundary " +
		"is 1 ms",
	"listener+count; identical shape under the microsecond time source with all timestamps x1000: " +
		"A@flip-1µs fires, A@flip does not — the strict before boundary is 1 µs",
	"listener; advanceTime(t) then one SupportDateTime(longdate=t) send emits c0..c7 = " +
		"[calModMillis, calMod, 4, 4, 5, calTime, calTime, t-1]",
	"listener; advanceTime(t*1000) then SupportDateTime(longdate=t*1000+123) emits " +
		"c0=calModMillis*1000+123 (µs remainder preserved), c1/c5/c6 truncated to the same ms values " +
		"as the ms run, c7=t*1000-1000 (minus(1) is one millisecond)",
}

// exprDTResolutionEventTimeEPL is the byte-exact ord-0 statement.
const exprDTResolutionEventTimeEPL = "@name('s0') select * from MyEvent(id='A') as a unidirectional, " +
	"MyEvent(id='B')#lastevent as b where a.withDate(2002, 4, 30).before(b)"

// exprDTResolutionLongPropertyEPL is the byte-exact ord-1 statement: the Java
// source concatenates the select items without a space after the commas.
const exprDTResolutionLongPropertyEPL = "@name('s0') select " +
	"longdate.withTime(1, 2, 3, 4) as c0," +
	"longdate.set('hour', 1).set('minute', 2).set('second', 3).set('millisecond', 4).toCalendar() as c1," +
	"longdate.get('month') as c2," +
	"current_timestamp.get('month') as c3," +
	"current_timestamp.getMinuteOfHour() as c4," +
	"current_timestamp.toDate() as c5," +
	"current_timestamp.toCalendar() as c6," +
	"current_timestamp.minus(1) as c7 " +
	"from SupportDateTime"

var exprDTResolutionCaseEPLs = []string{
	exprDTResolutionEventTimeEPL,
	exprDTResolutionEventTimeEPL,
	exprDTResolutionLongPropertyEPL,
	exprDTResolutionLongPropertyEPL,
}

// exprDTResolutionEventTimeT is DateTime.parseDefaultMSec("2002-05-30T09:00:00.000").
const exprDTResolutionEventTimeT = int64(1022749200000)

// exprDTResolutionLongPropertyT is DateTime.parseDefaultMSec("2002-05-30T09:05:06.007").
const exprDTResolutionLongPropertyT = int64(1022749506007)

// exprDTResolutionSupportDateTime mirrors the SupportDateTime bean the
// long-property cases send: only longdate is non-null in Java; the Go struct
// carries the same five fields with the four date-time representations
// collapsed to time.Time.
type exprDTResolutionSupportDateTime struct {
	Longdate  int64     `esper:"longdate"`
	Utildate  time.Time `esper:"utildate"`
	Caldate   time.Time `esper:"caldate"`
	Localdate time.Time `esper:"localdate"`
	Zoneddate time.Time `esper:"zoneddate"`
}

type exprDTResolutionCaseState struct {
	caseName   string
	env        *esper.Environment
	engine     *esper.Engine
	trace      *compat.Trace
	sequence   uint64
	deployment *esper.Deployment
	fired      bool
}

func runExprDTResolutionScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range exprDTResolutionCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runExprDTResolutionCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprDTResolutionID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprDTResolutionID, scenario.ID)
	}
	return trace, nil
}

func runExprDTResolutionCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if err := registerExprDTResolutionTypes(env, caseName); err != nil {
		return compat.Trace{}, err
	}
	options := []esper.EngineOption{
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(exprDTResolutionCaseRuntimeIDs[caseName]),
	}
	if exprDTResolutionMicrosecond(caseName) {
		options = append(options, esper.WithTimeUnit(esper.Microseconds))
	}
	engine := esper.NewEngine(env, options...)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &exprDTResolutionCaseState{
		caseName: caseName,
		env:      env,
		engine:   engine,
		trace:    &compat.Trace{Version: scenario.Version, ID: scenario.ID},
	}
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *state.trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return *state.trace, fmt.Errorf("%s: parse advance-time %q: %w", exprDTResolutionID, step.At, err)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return *state.trace, err
			}
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *state.trace, err
			}
		case "send":
			if err := state.send(ctx, step); err != nil {
				return *state.trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *state.trace, err
			}
		default:
			return *state.trace, fmt.Errorf("%s: unsupported step op %q", exprDTResolutionID, step.Op)
		}
	}
	return *state.trace, nil
}

// exprDTResolutionMicrosecond reports whether the case runs under the
// microsecond time source (the -us suffixed cases replay
// executions(isMicrosecond=true)).
func exprDTResolutionMicrosecond(caseName string) bool {
	return caseName == "event-time-us" || caseName == "long-property-us"
}

// registerExprDTResolutionTypes mirrors the suite registrations each case
// compiles against: the object-array MyEvent(id,sts,ets) type with sts/ets
// declared as the start/end timestamp properties for the event-time cases,
// and the SupportDateTime bean for the long-property cases.
func registerExprDTResolutionTypes(env *esper.Environment, caseName string) error {
	switch caseName {
	case "event-time-ms", "event-time-us":
		if _, err := esper.RegisterObjectArray(env, "MyEvent", []esper.FieldSpec{
			{Name: "id", Type: reflect.TypeOf("")},
			{Name: "sts", Type: reflect.TypeOf(int64(0)), StartTimestamp: true},
			{Name: "ets", Type: reflect.TypeOf(int64(0)), EndTimestamp: true},
		}); err != nil {
			return err
		}
		return nil
	case "long-property-ms", "long-property-us":
		_, err := esper.RegisterStruct[exprDTResolutionSupportDateTime](env, "SupportDateTime")
		return err
	default:
		return fmt.Errorf("%s: unsupported case %q", exprDTResolutionID, caseName)
	}
}

// deploy mirrors one compileDeploy + addListener('s0') cycle; the pinned EPL
// is verified before the fluent equivalent is built.
func (s *exprDTResolutionCaseState) deploy(ctx context.Context, step compat.Step) error {
	pinned, err := exprDTResolutionCaseEPL(s.caseName)
	if err != nil {
		return err
	}
	if step.Epl != pinned {
		return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q", exprDTResolutionID, step.Statement, step.Epl)
	}
	if err := s.undeployAll(ctx); err != nil {
		return err
	}
	query, err := s.buildDeploy(step.Statement)
	if err != nil {
		return err
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return err
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployment = deployment
	for _, statement := range deployment.Statements() {
		captured := statement
		if _, err := captured.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			newRows := compat.NormalizeResults(batch.New)
			exprDTResolutionRenderMillis(newRows)
			if len(newRows) == 0 {
				return nil
			}
			s.fired = true
			s.sequence++
			s.trace.Records = append(s.trace.Records, compat.TraceRecord{
				Case:      s.caseName,
				Operation: "listener",
				Statement: captured.Name(),
				Sequence:  s.sequence,
				Time:      compat.FormatTraceTime(batch.Time),
				New:       newRows,
			})
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// exprDTResolutionCaseEPL returns the case's pinned deploy EPL: the
// event-time join for the event-time cases, the c0..c7 select for the
// long-property cases.
func exprDTResolutionCaseEPL(caseName string) (string, error) {
	switch caseName {
	case "event-time-ms", "event-time-us":
		return exprDTResolutionEventTimeEPL, nil
	case "long-property-ms", "long-property-us":
		return exprDTResolutionLongPropertyEPL, nil
	default:
		return "", fmt.Errorf("%s: unsupported case %q", exprDTResolutionID, caseName)
	}
}

// buildDeploy renders the fluent equivalent of the case's pinned EPL.
func (s *exprDTResolutionCaseState) buildDeploy(label string) (esper.Query, error) {
	if label != "s0" {
		return esper.Query{}, fmt.Errorf("%s: case %q deploys no statement %q", exprDTResolutionID, s.caseName, label)
	}
	switch s.caseName {
	case "event-time-ms", "event-time-us":
		return buildExprDTResolutionEventTimeQuery(s.env), nil
	case "long-property-ms", "long-property-us":
		return buildExprDTResolutionLongPropertyQuery(s.env), nil
	default:
		return esper.Query{}, fmt.Errorf("%s: case %q deploys no statements", exprDTResolutionID, s.caseName)
	}
}

// buildExprDTResolutionEventTimeQuery renders the ord-0 unidirectional join:
// the filtered A stream drives, the filtered B stream is retained by
// #lastevent, and the where clause is the strict interval before over the
// withDate-shifted event bounds. select * projects both stream fragments.
// Java's withDate(2002, 4, 30) takes the 0-based Calendar.MONTH (4 = May);
// the Go calendar ops pin the 1-based LDT convention, so the equivalent
// call passes month 5 — both land on 2002-05-30.
func buildExprDTResolutionEventTimeQuery(env *esper.Environment) esper.Query {
	eventValue := esper.EventValue[esper.Event]()
	id := esper.Property[string](eventValue, "id")
	aInput := esper.JoinRecordSource(
		esper.FromAny(env, "MyEvent").Filter(esper.Equal[string](id, esper.Literal("A")))).Unidirectional()
	bInput := esper.JoinRecordSource(
		esper.FromAny(env, "MyEvent").Filter(esper.Equal[string](id, esper.Literal("B"))).Window(esper.LastEvent()))
	return esper.JoinMany(aInput, bInput).Select(
		esper.SelectSourceEvent(0, "a"),
		esper.SelectSourceEvent(1, "b"),
	).Where(esper.IntervalBefore(
		esper.WithDateBounds(esper.EventIntervalBounds(0), 2002, 5, 30),
		esper.EventIntervalBounds(1),
	)).Query(esper.StatementName("s0"))
}

// buildExprDTResolutionLongPropertyQuery renders the ord-1 select: c0 keeps
// the engine-unit remainder through withTime, c1 truncates to ms through
// toCalendar, c2/c3 read the 0-based month, c4 reads minute-of-hour, c5/c6
// truncate current_timestamp to ms, and c7 subtracts one millisecond.
func buildExprDTResolutionLongPropertyQuery(env *esper.Environment) esper.Query {
	longdate := esper.Field[exprDTResolutionSupportDateTime, int64]("longdate")
	now := esper.CurrentTimestamp()
	return esper.Select(esper.From[exprDTResolutionSupportDateTime](env, "SupportDateTime"),
		esper.Alias("c0", esper.DateTimeWithTime[int64](longdate, 1, 2, 3, 4)),
		esper.Alias("c1", esper.DateTimeToTime[int64](
			esper.DateTimeSet[int64](
				esper.DateTimeSet[int64](
					esper.DateTimeSet[int64](
						esper.DateTimeSet[int64](longdate, "hour", 1), "minute", 2), "second", 3), "millisecond", 4))),
		esper.Alias("c2", esper.DateTimeGet[int64](longdate, "month")),
		esper.Alias("c3", esper.DateTimeGet[int64](now, "month")),
		esper.Alias("c4", esper.DateTimeGet[int64](now, "minute_of_hour")),
		esper.Alias("c5", esper.DateTimeToTime[int64](now)),
		esper.Alias("c6", esper.DateTimeToTime[int64](now)),
		esper.Alias("c7", esper.DateTimeMinus[int64](now, 1)),
	).Query(esper.StatementName("s0"))
}

// send decodes the payload, delivers the event, and emits exactly one record:
// the listener row when the send fires (the callback appends it) or a
// listener-not-invoked count when it does not — mirroring the oracle's
// assertListenerInvoked/assertListenerNotInvoked checks. An expected row that
// fails to arrive, or an unexpected delivery, is a replay error.
func (s *exprDTResolutionCaseState) send(ctx context.Context, step compat.Step) error {
	expected, err := decodeExprDTResolutionExpected(step)
	if err != nil {
		return err
	}
	s.fired = false
	switch step.EventType {
	case "MyEvent":
		var payload struct {
			ID  string `json:"id"`
			Sts int64  `json:"sts"`
			Ets int64  `json:"ets"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("%s: decode MyEvent: %w", exprDTResolutionID, err)
		}
		if err := s.engine.SendObjectArray(ctx, "MyEvent", []any{payload.ID, payload.Sts, payload.Ets}); err != nil {
			return err
		}
	case "SupportDateTime":
		var payload struct {
			Longdate int64 `json:"longdate"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("%s: decode SupportDateTime: %w", exprDTResolutionID, err)
		}
		if err := s.engine.Send(ctx, "SupportDateTime", exprDTResolutionSupportDateTime{Longdate: payload.Longdate}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s: unsupported event type %q", exprDTResolutionID, step.EventType)
	}
	if s.fired != expected {
		return fmt.Errorf("%s: %s send fired=%t, want %t", exprDTResolutionID, step.EventType, s.fired, expected)
	}
	if !s.fired {
		s.sequence++
		zero := int64(0)
		s.trace.Records = append(s.trace.Records, compat.TraceRecord{
			Case:      s.caseName,
			Operation: "count",
			Statement: "s0",
			Sequence:  s.sequence,
			Time:      compat.FormatTraceTime(s.engine.Now()),
			Name:      "listener-not-invoked",
			Count:     &zero,
		})
	}
	return nil
}

// decodeExprDTResolutionExpected reads the pinned expected flag every send
// payload carries: true when the send must produce a listener row, false when
// it must not.
func decodeExprDTResolutionExpected(step compat.Step) (bool, error) {
	var payload struct {
		Expected *bool `json:"expected"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return false, fmt.Errorf("%s: decode expected flag: %w", exprDTResolutionID, err)
	}
	if payload.Expected == nil {
		return false, fmt.Errorf("%s: %s send is missing the expected flag", exprDTResolutionID, step.EventType)
	}
	return *payload.Expected, nil
}

// exprDTResolutionRenderMillis renders time.Time cells as epoch millis, the
// oracle's instant-token convention for toCalendar()/toDate() columns.
func exprDTResolutionRenderMillis(rows []compat.ResultRecord) {
	for _, row := range rows {
		for name, value := range row.Fields {
			if located, ok := value.(time.Time); ok {
				row.Fields[name] = located.UnixMilli()
			}
		}
	}
}

func (s *exprDTResolutionCaseState) undeployAll(ctx context.Context) error {
	if s.deployment == nil {
		return nil
	}
	if err := s.deployment.Undeploy(ctx); err != nil {
		return err
	}
	s.deployment = nil
	return nil
}

// exprDTResolutionCaseSteps pins the complete step sequence per case as
// op|case|statement|eventType|epl|payload|expectError|at keys so the loader
// asserts the scenario file matches the contract byte-for-byte.
var exprDTResolutionCaseSteps = func() map[string][]string {
	steps := map[string][]string{}
	for _, suffix := range []string{"ms", "us"} {
		caseName := "event-time-" + suffix
		unit := exprDTResolutionEventTimeT
		if suffix == "us" {
			unit = exprDTResolutionEventTimeT * 1000
		}
		steps[caseName] = []string{
			"advance-time|" + caseName + "||||||1970-01-01T00:00:00Z",
			"deploy|" + caseName + "|s0||" + exprDTResolutionEventTimeEPL + "|||",
			fmt.Sprintf("send|%s||MyEvent||{\"id\":\"B\",\"sts\":%d,\"ets\":%d,\"expected\":false}||", caseName, unit, unit),
			fmt.Sprintf("send|%s||MyEvent||{\"id\":\"A\",\"sts\":%d,\"ets\":%d,\"expected\":true}||", caseName, unit-1, unit-1),
			fmt.Sprintf("send|%s||MyEvent||{\"id\":\"A\",\"sts\":%d,\"ets\":%d,\"expected\":false}||", caseName, unit, unit),
			"undeploy-all|" + caseName + "||||||",
		}
		caseName = "long-property-" + suffix
		advance := "2002-05-30T09:05:06.007Z"
		longdate := exprDTResolutionLongPropertyT
		if suffix == "us" {
			longdate = exprDTResolutionLongPropertyT*1000 + 123
		}
		steps[caseName] = []string{
			"advance-time|" + caseName + "||||||" + advance,
			"deploy|" + caseName + "|s0||" + exprDTResolutionLongPropertyEPL + "|||",
			fmt.Sprintf("send|%s||SupportDateTime||{\"longdate\":%d,\"expected\":true}||", caseName, longdate),
			"undeploy-all|" + caseName + "||||||",
		}
	}
	return steps
}()

// loadExprDTResolutionScenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned metadata and case metadata, and the complete
// pinned step sequence so unknown, duplicated or drifted content fails the
// replay.
func loadExprDTResolutionScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprDTResolutionID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprDTResolutionID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTResolutionID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTResolutionID, err)
	}
	if err := requireExprDTResolutionFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version     string `json:"version"`
		ID          string `json:"id"`
		Description string `json:"description"`
		JavaCommit  string `json:"javaCommit"`
		JavaSource  string `json:"javaSource"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprDTResolutionID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprDTResolutionID ||
		metadata.Description != exprDTResolutionDescription ||
		metadata.JavaCommit != exprDTResolutionJavaCommit ||
		metadata.JavaSource != exprDTResolutionJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprDTResolutionID)
	}
	if err := validateExprDTResolutionStringArray(root["javaRuntimes"], exprDTResolutionJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTResolutionStringArray(root["javaNames"], exprDTResolutionJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTResolutionStringArray(root["javaStaticIds"], exprDTResolutionJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTResolutionStringArray(root["javaFlags"], exprDTResolutionJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprDTResolutionCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", exprDTResolutionID, len(exprDTResolutionCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprDTResolutionFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
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
		if definition.Case != exprDTResolutionCases[index] ||
			definition.Ordinal != exprDTResolutionOrdinals[index] ||
			definition.RuntimeID != exprDTResolutionJavaRuntimeIDs[index] ||
			definition.ExecutionName != exprDTResolutionJavaExecutions[index] ||
			definition.Observation != exprDTResolutionCaseObservations[index] ||
			definition.EPL != exprDTResolutionCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprDTResolutionID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", exprDTResolutionID, err)
	}
	offset := 0
	for _, caseName := range exprDTResolutionCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", exprDTResolutionID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTResolutionID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", exprDTResolutionID, offset, caseName)
		}
		if _, err := exprDTResolutionStepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTResolutionID, offset, err)
		}
		offset++
		want, ok := exprDTResolutionCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", exprDTResolutionID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", exprDTResolutionID, caseName)
		}
		for _, pinned := range want {
			key, err := exprDTResolutionStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTResolutionID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", exprDTResolutionID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", exprDTResolutionID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTResolutionID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// exprDTResolutionStepKey renders one raw step as its pinned key:
// op|case|statement|eventType|epl|payload|expectError|at with the payload
// compacted. Unknown fields on the step object are rejected per op.
func exprDTResolutionStepKey(raw json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return "", err
	}
	var step struct {
		Op          string          `json:"op"`
		Case        string          `json:"case"`
		Statement   string          `json:"statement"`
		EventType   string          `json:"eventType"`
		Epl         string          `json:"epl"`
		Payload     json.RawMessage `json:"payload"`
		ExpectError string          `json:"expectError"`
		At          string          `json:"at"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string][]string{
		"case":         {"op", "case"},
		"advance-time": {"op", "case", "at"},
		"deploy":       {"op", "case", "statement", "epl"},
		"send":         {"op", "case", "eventType", "payload"},
		"undeploy-all": {"op", "case"},
	}
	fields, ok := allowed[step.Op]
	if !ok {
		return "", fmt.Errorf("step has unsupported op %q", step.Op)
	}
	for field := range object {
		found := false
		for _, name := range fields {
			if field == name {
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	payloadText := ""
	if step.Op == "send" {
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, step.Payload); err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		payloadText = compacted.String()
	}
	return step.Op + "|" + step.Case + "|" + step.Statement + "|" + step.EventType +
		"|" + step.Epl + "|" + payloadText + "|" + step.ExpectError + "|" + step.At, nil
}

func requireExprDTResolutionFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", exprDTResolutionID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", exprDTResolutionID, name)
		}
	}
	return nil
}

func validateExprDTResolutionStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
