package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// expr_dt_interval_ops.go replays ExprDTIntervalOps ordinals 0, 17 and 1
// against the pinned Java oracle:
//
//   - calendar-ops (ord 0, ExprDTIntervalCalendarOps): the five where-clauses
//     run over all five A_<FT>/B_<FT> object-array field types. Java deploys
//     `select * from A_<FT>#lastevent as a, B_<FT>#lastevent as b where
//     <clause>` and asserts listener-invoked == expected per A send; the
//     parity deploy renders the same clause as `select <clause> as c0 ...`
//     (the select-clause form ExprDTIntervalBeforeInSelectClause proves
//     compiles) so every A send emits one row carrying the observed boolean
//     and the zero-diff compare is total. Go collapses the five Java field
//     types to two representations — MSEC binds int64 epoch-millis fields
//     while DATE/CAL/LDT/ZDT bind time.Time fields coerced through
//     UnixMillis — and still replays all five variants so the record stream
//     matches the oracle's 25 deployments.
//   - point-in-time-calendar (ord 17, ExprDTIntervalPointInTimeWCalendarOps):
//     one unidirectional SupportDateTime x SupportBean#lastevent deployment
//     emitting c0..c4; the SupportBean seed produces no row.
//   - invalid (ord 1, ExprDTIntervalInvalid): 23 tryInvalidCompile probes
//     record the pinned Java message prefixes. Only the operand-type
//     rejections are representable on the typed Go surface (DateTimeBefore
//     operand validation); the rest pin the prefix without claiming a Go
//     boundary, per the itcmsInvalidProbes precedent.

const exprDTIntervalOpsID = "expr-dt-interval-ops"
const exprDTIntervalOpsJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprDTIntervalOpsJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTIntervalOps.java",
}

var exprDTIntervalOpsJavaRuntimeIDs = []string{
	"java-runtime-e29e7c3671fa1488e788",
	"java-runtime-bcdd330f46ca3dd63da7",
	"java-runtime-eb0d233c3d7f90db1120",
}

var exprDTIntervalOpsJavaExecutions = []string{
	"ExprDTIntervalCalendarOps",
	"ExprDTIntervalPointInTimeWCalendarOps",
	"ExprDTIntervalInvalid",
}

var exprDTIntervalOpsCases = []string{
	"calendar-ops",
	"point-in-time-calendar",
	"invalid",
}

var exprDTIntervalOpsCaseRuntimeIDs = map[string]string{
	"calendar-ops":           "java-runtime-e29e7c3671fa1488e788",
	"point-in-time-calendar": "java-runtime-bcdd330f46ca3dd63da7",
	"invalid":                "java-runtime-eb0d233c3d7f90db1120",
}

const exprDTIntervalOpsDescription = "ExprDTIntervalOps ords 0/17/1: calendar-ops replays " +
	"ExprDTIntervalCalendarOps (five where-clauses over the five A_<FT>/B_<FT> object-array " +
	"field types, deployed in the select-clause form '<clause> as c0' so the observed boolean " +
	"is total per A send), point-in-time-calendar replays " +
	"ExprDTIntervalPointInTimeWCalendarOps (set('month',1).before over the five SupportDateTime " +
	"representations), and invalid replays ExprDTIntervalInvalid's 23 tryInvalidCompile probes " +
	"with pinned Java message prefixes."

var exprDTIntervalOpsJavaStaticIDs = []string{
	"java-7d6d5e191799aa72b645",
	"java-583eee3ee090b7f63076",
	"java-8b79d6633a735b6cc1ec",
}

var exprDTIntervalOpsJavaFlags = []string{}

var exprDTIntervalOpsOrdinals = []int{0, 17, 1}

var exprDTIntervalOpsCaseObservations = []string{
	"listener; 25 deployments (5 where-clauses x 5 field types) each send one " +
		"B_<FT> seed then A_<FT> rows; Java asserts listener invoked iff expected under " +
		"'select * ... where <clause>' — the parity deploy renders the same clause as " +
		"'<clause> as c0' so every A send emits one row carrying the observed boolean",
	"listener; one deployment emits c0..c4 per SupportDateTime send: the " +
		"SupportBean seed produces no row (unidirectional), 2002-05-30T09:00:00.000 yields " +
		"all-true and 2003-05-30T08:00:00.000 yields all-false",
	"compile-error; 23 tryInvalidCompile probes record the pinned Java " +
		"message prefixes (startsWith assertion); the Go runner verifies the nearest " +
		"expressible rejection for the representable subset and pins prefix-only for the " +
		"unrepresentable rest",
}

var exprDTIntervalOpsCaseEPLs = []string{
	"@name('s0') select a.withDate(2001, 1, 1).before(b) as c0 from A_MSEC#lastevent as a, B_MSEC#lastevent as b",
	exprDTIntervalOpsPointEPL,
	"select a.before('x') from SupportTimeStartEndA as a",
}

// exprDTIntervalOpsSupportBean mirrors the SupportBean properties the
// scenarios touch: longPrimitive drives the point-in-time threshold and
// theString is the wrong-typed receiver of probe 5.
type exprDTIntervalOpsSupportBean struct {
	TheString     string `esper:"theString"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// exprDTIntervalOpsFieldTypes are the five SupportDateTimeFieldType variants
// the Java execution iterates; Go replays all five so the record stream
// matches even though DATE/CAL/LDT/ZDT share one time.Time representation.
var exprDTIntervalOpsFieldTypes = []string{"MSEC", "DATE", "CAL", "LDT", "ZDT"}

// exprDTIntervalOpsVariant pins one assertExpression call: the byte-exact
// where-clause text, the B seed duration and the A rows with their expected
// booleans (the expected flag steers the oracle's per-send verification).
type exprDTIntervalOpsVariant struct {
	name         string
	clause       string
	seedDuration int64
	rows         []exprDTIntervalOpsRow
}

type exprDTIntervalOpsRow struct {
	start    string
	duration int64
	expected bool
}

var exprDTIntervalOpsVariants = []exprDTIntervalOpsVariant{
	{name: "v1", clause: "a.withDate(2001, 1, 1).before(b)", seedDuration: 0, rows: []exprDTIntervalOpsRow{
		{start: "2999-01-01T09:00:00.001Z", duration: 0, expected: true},
	}},
	{name: "v2", clause: "a.withDate(2001, 1, 1).before(b.withDate(2001, 1, 1))", seedDuration: 0, rows: []exprDTIntervalOpsRow{
		{start: "2999-01-01T10:00:00.001Z", duration: 0, expected: false},
		{start: "2999-01-01T08:00:00.001Z", duration: 0, expected: true},
	}},
	{name: "v3", clause: "a.before(b)", seedDuration: 0, rows: []exprDTIntervalOpsRow{
		{start: "2002-05-30T08:59:59.000Z", duration: 2000, expected: false},
	}},
	{name: "v4", clause: "a.withTime(8, 59, 59, 0).before(b)", seedDuration: 0, rows: []exprDTIntervalOpsRow{
		{start: "2002-05-30T08:59:59.000Z", duration: 2000, expected: false},
	}},
	{name: "v5", clause: "a.after(b)", seedDuration: 1000, rows: []exprDTIntervalOpsRow{
		{start: "2002-05-30T09:00:01.000Z", duration: 0, expected: false},
		{start: "2002-05-30T09:00:01.001Z", duration: 0, expected: true},
	}},
}

// exprDTIntervalOpsPointEPL is the byte-exact ord-17 statement (the Java
// source concatenates the select items without a space after the c1/c2/c3
// commas).
const exprDTIntervalOpsPointEPL = "@name('s0') select " +
	"longdate.set('month', 1).before(longPrimitive) as c0, " +
	"utildate.set('month', 1).before(longPrimitive) as c1," +
	"caldate.set('month', 1).before(longPrimitive) as c2," +
	"localdate.set('month', 1).before(longPrimitive) as c3," +
	"zoneddate.set('month', 1).before(longPrimitive) as c4 " +
	"from SupportDateTime unidirectional, SupportBean#lastevent"

// exprDTIntervalOpsDeployEPLs maps every deploy-step statement label to its
// pinned EPL: the 25 calendar-ops deployments (variant-major, field-type
// minor, mirroring the assertExpression loop nesting) plus the single
// point-in-time deployment.
var exprDTIntervalOpsDeployEPLs = func() map[string]string {
	epls := map[string]string{"s0": exprDTIntervalOpsPointEPL}
	for _, variant := range exprDTIntervalOpsVariants {
		for _, fieldType := range exprDTIntervalOpsFieldTypes {
			epls[variant.name+"-"+strings.ToLower(fieldType)] = fmt.Sprintf(
				"@name('s0') select %s as c0 from A_%s#lastevent as a, B_%s#lastevent as b",
				variant.clause, fieldType, fieldType)
		}
	}
	return epls
}()

type exprDTIntervalOpsCaseState struct {
	caseName   string
	env        *esper.Environment
	engine     *esper.Engine
	trace      *compat.Trace
	sequence   uint64
	deployment *esper.Deployment
}

func runExprDTIntervalOpsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range exprDTIntervalOpsCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runExprDTIntervalOpsCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprDTIntervalOpsID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprDTIntervalOpsID, scenario.ID)
	}
	return trace, nil
}

func runExprDTIntervalOpsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if err := registerExprDTIntervalOpsTypes(env, caseName); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(exprDTIntervalOpsCaseRuntimeIDs[caseName]))
	defer func() { _ = engine.Close(context.Background()) }()

	state := &exprDTIntervalOpsCaseState{
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
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *state.trace, err
			}
		case "send":
			if err := state.send(ctx, step); err != nil {
				return *state.trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *state.trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *state.trace, err
			}
		default:
			return *state.trace, fmt.Errorf("%s: unsupported step op %q", exprDTIntervalOpsID, step.Op)
		}
	}
	return *state.trace, nil
}

// registerExprDTIntervalOpsTypes mirrors the TestSuiteExprDateTime
// registrations each case compiles against: the ten A_<FT>/B_<FT>
// object-array types for calendar-ops (startTS/endTS timestamp properties),
// SupportDateTime + SupportBean for point-in-time-calendar, and the
// SupportTimeStartEndA/B beans plus SupportBean for the invalid probes.
func registerExprDTIntervalOpsTypes(env *esper.Environment, caseName string) error {
	switch caseName {
	case "calendar-ops":
		for _, fieldType := range exprDTIntervalOpsFieldTypes {
			for _, prefix := range []string{"A_", "B_"} {
				if _, err := esper.RegisterMap(env, prefix+fieldType, exprDTIntervalOpsObjectArrayFields(fieldType)); err != nil {
					return err
				}
			}
		}
		return nil
	case "point-in-time-calendar":
		if _, err := esper.RegisterMap(env, "SupportDateTime", exprDTIntervalOpsDateTimeFields()); err != nil {
			return err
		}
		_, err := esper.RegisterStruct[exprDTIntervalOpsSupportBean](env, "SupportBean")
		return err
	case "invalid":
		for _, name := range []string{"SupportTimeStartEndA", "SupportTimeStartEndB"} {
			if _, err := esper.RegisterMap(env, name, exprDTIntervalOpsStartEndFields()); err != nil {
				return err
			}
		}
		_, err := esper.RegisterStruct[exprDTIntervalOpsSupportBean](env, "SupportBean")
		return err
	default:
		return fmt.Errorf("%s: unsupported case %q", exprDTIntervalOpsID, caseName)
	}
}

// exprDTIntervalOpsObjectArrayFields mirrors the object-array type
// declaration: startTS/endTS typed long for MSEC and a date-time class for
// the other four field types, which Go collapses to time.Time.
func exprDTIntervalOpsObjectArrayFields(fieldType string) []esper.FieldSpec {
	valueType := reflect.TypeOf(time.Time{})
	if fieldType == "MSEC" {
		valueType = reflect.TypeOf(int64(0))
	}
	return []esper.FieldSpec{
		esper.FieldDef("startTS", valueType),
		esper.FieldDef("endTS", valueType),
	}
}

// exprDTIntervalOpsStartEndFields mirrors the SupportTimeStartEndA/B bean
// properties the invalid probes reference (longdateStart/longdateEnd carry
// the declared start/end timestamps).
func exprDTIntervalOpsStartEndFields() []esper.FieldSpec {
	return []esper.FieldSpec{
		esper.FieldDef("longdateStart", reflect.TypeOf(int64(0))),
		esper.FieldDef("longdateEnd", reflect.TypeOf(int64(0))),
	}
}

// exprDTIntervalOpsDateTimeFields mirrors SupportDateTime: longdate is the
// epoch-millis Long and the other four representations collapse to
// time.Time.
func exprDTIntervalOpsDateTimeFields() []esper.FieldSpec {
	return []esper.FieldSpec{
		esper.FieldDef("longdate", reflect.TypeOf(int64(0))),
		esper.FieldDef("utildate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("caldate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("localdate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("zoneddate", reflect.TypeOf(time.Time{})),
	}
}

// deploy mirrors one compileDeploy + addListener('s0') cycle. Java's
// assertExpression undeploys between where-clauses; the deploy step folds
// that undeploy in so consecutive deploys replace the live statement.
func (s *exprDTIntervalOpsCaseState) deploy(ctx context.Context, step compat.Step) error {
	pinned, ok := exprDTIntervalOpsDeployEPLs[step.Statement]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q", exprDTIntervalOpsID, step.Statement, step.Epl)
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
			if len(newRows) == 0 {
				return nil
			}
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

// buildDeploy renders the fluent equivalent of the pinned EPL: the
// point-in-time select for "s0" or the calendar-ops join for the
// variant-fieldType labels.
func (s *exprDTIntervalOpsCaseState) buildDeploy(label string) (esper.Query, error) {
	if s.caseName == "point-in-time-calendar" {
		if label != "s0" {
			return esper.Query{}, fmt.Errorf("%s: case %q deploys no statement %q", exprDTIntervalOpsID, s.caseName, label)
		}
		return buildExprDTIntervalOpsPointQuery(s.env), nil
	}
	if s.caseName != "calendar-ops" {
		return esper.Query{}, fmt.Errorf("%s: case %q deploys no statements", exprDTIntervalOpsID, s.caseName)
	}
	variantName, fieldType, err := parseExprDTIntervalOpsDeployLabel(label)
	if err != nil {
		return esper.Query{}, err
	}
	variant, ok := exprDTIntervalOpsVariantByName(variantName)
	if !ok {
		return esper.Query{}, fmt.Errorf("%s: unknown calendar-ops variant %q", exprDTIntervalOpsID, variantName)
	}
	left, right := exprDTIntervalOpsBounds(s.env, fieldType)
	var predicate esper.Expression[bool]
	switch variant.name {
	case "v1":
		// a.withDate(2001, 1, 1).before(b): the interval target preserves
		// duration through the calendar transform.
		predicate = esper.Interval(esper.Before, esper.WithDateBounds(left, 2001, 1, 1), right)
	case "v2":
		// a.withDate(2001, 1, 1).before(b.withDate(2001, 1, 1)): the
		// parameter-side calendar op collapses to a point (documented Java
		// limitation), so the right bounds become (start, start).
		predicate = esper.Interval(esper.Before,
			esper.WithDateBounds(left, 2001, 1, 1),
			esper.PointBounds(esper.WithDateBounds(right, 2001, 1, 1)))
	case "v3":
		predicate = esper.Interval(esper.Before, left, right)
	case "v4":
		// a.withTime(8, 59, 59, 0).before(b): duration preserved.
		predicate = esper.Interval(esper.Before, esper.WithTimeBounds(left, 8, 59, 59, 0), right)
	case "v5":
		predicate = esper.Interval(esper.After, left, right)
	default:
		return esper.Query{}, fmt.Errorf("%s: unsupported calendar-ops variant %q", exprDTIntervalOpsID, variant.name)
	}
	return esper.JoinMany(
		esper.JoinSource(esper.From[map[string]any](s.env, "A_"+fieldType)).Window(esper.LengthWindow(1)),
		esper.JoinSource(esper.From[map[string]any](s.env, "B_"+fieldType)).Window(esper.LengthWindow(1)),
	).Select(
		esper.SelectFrom(0, "c0", predicate),
	).Query(esper.StatementName("s0")), nil
}

// exprDTIntervalOpsBounds builds the (startTS, endTS) interval bounds for
// one field type: int64 epoch-millis fields for MSEC, time.Time fields
// coerced through UnixMillis for the collapsed DATE/CAL/LDT/ZDT reps.
func exprDTIntervalOpsBounds(env *esper.Environment, fieldType string) (esper.IntervalBounds, esper.IntervalBounds) {
	if fieldType == "MSEC" {
		return esper.IntervalBounds{
				Start: esper.JoinField[int64](0, "startTS"),
				End:   esper.JoinField[int64](0, "endTS"),
			}, esper.IntervalBounds{
				Start: esper.JoinField[int64](1, "startTS"),
				End:   esper.JoinField[int64](1, "endTS"),
			}
	}
	return esper.IntervalBounds{
			Start: esper.UnixMillis(esper.JoinField[time.Time](0, "startTS")),
			End:   esper.UnixMillis(esper.JoinField[time.Time](0, "endTS")),
		}, esper.IntervalBounds{
			Start: esper.UnixMillis(esper.JoinField[time.Time](1, "startTS")),
			End:   esper.UnixMillis(esper.JoinField[time.Time](1, "endTS")),
		}
}

// buildExprDTIntervalOpsPointQuery renders the ord-17 unidirectional join:
// SupportDateTime drives, SupportBean#lastevent supplies the longPrimitive
// threshold, and each column applies set('month',1) before the strict
// point-in-time before.
func buildExprDTIntervalOpsPointQuery(env *esper.Environment) esper.Query {
	dateInput := esper.From[map[string]any](env, "SupportDateTime")
	beanInput := esper.From[exprDTIntervalOpsSupportBean](env, "SupportBean").Window(esper.LastEvent())
	primitive := esper.JoinField[int64](1, "longPrimitive")
	return esper.Join(dateInput, beanInput).Unidirectional(esper.JoinLeft).Select(
		esper.SelectFrom(0, "c0", esper.DateTimeBefore(
			esper.DateTimeSet[int64](esper.JoinField[int64](0, "longdate"), "month", 1), primitive)),
		esper.SelectFrom(0, "c1", esper.DateTimeBefore(
			esper.DateTimeSet[time.Time](esper.JoinField[time.Time](0, "utildate"), "month", 1), primitive)),
		esper.SelectFrom(0, "c2", esper.DateTimeBefore(
			esper.DateTimeSet[time.Time](esper.JoinField[time.Time](0, "caldate"), "month", 1), primitive)),
		esper.SelectFrom(0, "c3", esper.DateTimeBefore(
			esper.DateTimeSet[time.Time](esper.JoinField[time.Time](0, "localdate"), "month", 1), primitive)),
		esper.SelectFrom(0, "c4", esper.DateTimeBefore(
			esper.DateTimeSet[time.Time](esper.JoinField[time.Time](0, "zoneddate"), "month", 1), primitive)),
	).Query(esper.StatementName("s0"))
}

func parseExprDTIntervalOpsDeployLabel(label string) (string, string, error) {
	variantName, fieldType, found := strings.Cut(label, "-")
	if !found || fieldType == "" {
		return "", "", fmt.Errorf("%s: malformed deploy label %q", exprDTIntervalOpsID, label)
	}
	for _, candidate := range exprDTIntervalOpsFieldTypes {
		if strings.EqualFold(candidate, fieldType) {
			return variantName, candidate, nil
		}
	}
	return "", "", fmt.Errorf("%s: deploy label %q names an unknown field type", exprDTIntervalOpsID, label)
}

func exprDTIntervalOpsVariantByName(name string) (exprDTIntervalOpsVariant, bool) {
	for _, variant := range exprDTIntervalOpsVariants {
		if variant.name == name {
			return variant, true
		}
	}
	return exprDTIntervalOpsVariant{}, false
}

// send decodes the payload and delivers the event. A_<FT>/B_<FT> payloads
// carry the ISO start plus a millisecond duration (the Java object array is
// {makeStart(time), makeEnd(time, duration)}); SupportDateTime fans one
// instant out to the five representations; SupportBean sends the typed bean.
func (s *exprDTIntervalOpsCaseState) send(ctx context.Context, step compat.Step) error {
	event, err := decodeExprDTIntervalOpsPayload(step)
	if err != nil {
		return err
	}
	if bean, ok := event.(exprDTIntervalOpsSupportBean); ok {
		return s.engine.Send(ctx, step.EventType, bean)
	}
	return s.engine.SendRecord(ctx, step.EventType, event.(map[string]any))
}

func decodeExprDTIntervalOpsPayload(step compat.Step) (any, error) {
	switch {
	case strings.HasPrefix(step.EventType, "A_") || strings.HasPrefix(step.EventType, "B_"):
		var payload struct {
			Start    string `json:"start"`
			Duration int64  `json:"duration"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode %s: %w", step.EventType, err)
		}
		start, err := time.Parse(time.RFC3339Nano, payload.Start)
		if err != nil {
			return nil, fmt.Errorf("parse %s start: %w", step.EventType, err)
		}
		end := start.Add(time.Duration(payload.Duration) * time.Millisecond)
		if strings.HasSuffix(step.EventType, "_MSEC") {
			return map[string]any{"startTS": start.UnixMilli(), "endTS": end.UnixMilli()}, nil
		}
		return map[string]any{"startTS": start, "endTS": end}, nil
	case step.EventType == "SupportDateTime":
		var payload struct {
			Date string `json:"date"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportDateTime: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, payload.Date)
		if err != nil {
			return nil, fmt.Errorf("parse SupportDateTime date: %w", err)
		}
		// SupportDateTime.make derives every representation from one
		// instant; the calendar rep forces MILLISECOND to zero.
		return map[string]any{
			"longdate":  parsed.UnixMilli(),
			"utildate":  parsed,
			"caldate":   parsed.Add(-time.Duration(parsed.Nanosecond())),
			"localdate": parsed,
			"zoneddate": parsed,
		}, nil
	case step.EventType == "SupportBean":
		var payload struct {
			LongPrimitive int64 `json:"longPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return exprDTIntervalOpsSupportBean{LongPrimitive: payload.LongPrimitive}, nil
	default:
		return nil, fmt.Errorf("%s: unsupported event type %q", exprDTIntervalOpsID, step.EventType)
	}
}

// exprDTIntervalOpsProbe pins one tryInvalidCompile probe: the byte-exact
// EPL, the startsWith prefix Java asserts, and the Go fluent equivalent. A
// nil go_ means the probe is unrepresentable on the typed surface and only
// the pinned prefix is recorded.
type exprDTIntervalOpsProbe struct {
	label  string
	epl    string
	expect string
	go_    func(env *esper.Environment) error
	goSub  string
}

// exprDTIntervalOpsProbes transcribes ExprDTIntervalInvalid's 23 probes
// verbatim, including the trailing spaces several expected prefixes carry.
// Representable probes verify the nearest Go rejection (DateTimeBefore
// operand validation); the rest are unrepresentable — Go has no event-level
// timestamp-property declaration to violate, no get() facade, no zero- or
// four-parameter footprints, and no threshold constructors for the
// coincides/during/finishes/meets/overlaps/starts families.
var exprDTIntervalOpsProbes = []exprDTIntervalOpsProbe{
	{
		label:  "before-string-param",
		epl:    "select a.before('x') from SupportTimeStartEndA as a",
		expect: "Failed to validate select-clause expression 'a.before('x')': Failed to resolve enumeration method, date-time method or mapped property 'a.before('x')': For date-time method 'before' the first parameter expression returns 'String', however requires a Date, Calendar, Long-type return value or event (with timestamp)",
		go_: func(env *esper.Environment) error {
			_, err := env.Build(esper.Select(esper.From[map[string]any](env, "SupportTimeStartEndA"),
				esper.Alias("c0", esper.DateTimeBefore(
					esper.Field[map[string]any, int64]("longdateStart"), esper.Literal("x"))),
			).Query(esper.StatementName("s0")))
			return err
		},
		goSub: "epoch milliseconds or time.Time",
	},
	{
		label:  "before-untimestamped-event",
		epl:    "select a.before(b) from SupportTimeStartEndA#lastevent as a, SupportBean#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.before(b)': For date-time method 'before' the first parameter is event type 'SupportBean', however no timestamp property has been defined for this event type",
		// Unrepresentable: Go interval bounds are plain field expressions;
		// there is no event-level timestamp-property declaration for
		// SupportBean to lack.
	},
	{
		label:  "before-boolean-param",
		epl:    "select a.before(true) from SupportTimeStartEndA#lastevent as a, SupportBean#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.before(true)': For date-time method 'before' the first parameter expression returns 'boolean', however requires a Date, Calendar, Long-type return value or event (with timestamp)",
		go_: func(env *esper.Environment) error {
			_, err := env.Build(esper.Select(esper.From[map[string]any](env, "SupportTimeStartEndA"),
				esper.Alias("c0", esper.DateTimeBefore(
					esper.Field[map[string]any, int64]("longdateStart"), esper.Literal(true))),
			).Query(esper.StatementName("s0")))
			return err
		},
		goSub: "epoch milliseconds or time.Time",
	},
	{
		label:  "before-no-params",
		epl:    "select a.before() from SupportTimeStartEndA#lastevent as a, SupportBean#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.before()': Parameters mismatch for date-time method 'before', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing interval start value, or an expression providing timestamp or timestamped-event and an expression providing interval start value and an expression providing interval finishes value, but receives no parameters",
		// Unrepresentable: DateTimeBefore requires both operands and
		// Interval requires two bounds; a zero-parameter form does not
		// type-check.
	},
	{
		label:  "before-string-target",
		epl:    "select theString.before(a) from SupportTimeStartEndA#lastevent as a, SupportBean#lastevent as b",
		expect: "Failed to validate select-clause expression 'theString.before(a)': Date-time enumeration method 'before' requires either a Calendar, Date, long, LocalDateTime or ZonedDateTime value as input or events of an event type that declares a timestamp property but received String",
		go_: func(env *esper.Environment) error {
			_, err := env.Build(esper.JoinMany(
				esper.JoinSource(esper.From[map[string]any](env, "SupportTimeStartEndA")).Window(esper.LengthWindow(1)),
				esper.JoinSource(esper.From[exprDTIntervalOpsSupportBean](env, "SupportBean")).Window(esper.LengthWindow(1)),
			).Select(
				esper.SelectFrom(0, "c0", esper.DateTimeBefore(
					esper.JoinField[string](1, "theString"), esper.JoinField[int64](0, "longdateStart"))),
			).Query(esper.StatementName("s0")))
			return err
		},
		goSub: "epoch milliseconds or time.Time",
	},
	{
		label:  "before-untimestamped-target",
		epl:    "select b.before(a) from SupportTimeStartEndA#lastevent as a, SupportBean#lastevent as b",
		expect: "Failed to validate select-clause expression 'b.before(a)': Date-time enumeration method 'before' requires either a Calendar, Date, long, LocalDateTime or ZonedDateTime value as input or events of an event type that declares a timestamp property",
		// Unrepresentable: same missing timestamp-property declaration as
		// before-untimestamped-event, on the receiver side.
	},
	{
		label:  "before-get-target",
		epl:    "select a.get('month').before(a) from SupportTimeStartEndA#lastevent as a, SupportBean#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.get(\"month\").before(a)': Failed to resolve method 'get': Could not find enumeration method, date-time method, instance method or property named 'get'",
		// Unrepresentable: the fluent surface has no get() facade.
	},
	{
		label:  "before-string-threshold",
		epl:    "select a.before(b, 'abc') from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.before(b,\"abc\")': Failed to validate date-time method 'before', expected a time-period expression or a numeric-type result for expression parameter 1 but received String ",
		// Unrepresentable: BeforeThreshold takes int64 deltas; a string
		// threshold does not type-check.
	},
	{
		label:  "before-string-threshold-2",
		epl:    "select a.before(b, 1, 'def') from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.before(b,1,\"def\")': Failed to validate date-time method 'before', expected a time-period expression or a numeric-type result for expression parameter 2 but received String ",
	},
	{
		label:  "before-four-params",
		epl:    "select a.before(b, 1, 2, 3) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.before(b,1,2,3)': Parameters mismatch for date-time method 'before', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing interval start value, or an expression providing timestamp or timestamped-event and an expression providing interval start value and an expression providing interval finishes value, but receives 4 expressions ",
		// Unrepresentable: BeforeThreshold accepts at most two deltas.
	},
	{
		label:  "coincides-four-params",
		epl:    "select a.coincides(b, 1, 2, 3) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.coincides(b,1,2,3)': Parameters mismatch for date-time method 'coincides', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing threshold for start and end value, or an expression providing timestamp or timestamped-event and an expression providing threshold for start value and an expression providing threshold for end value, but receives 4 expressions ",
		// Unrepresentable: no threshold form of Coincides exists.
	},
	{
		label:  "coincides-negative",
		epl:    "select a.coincides(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.coincides(b,-1)': The coincides date-time method does not allow negative start and end values ",
	},
	{
		label:  "during-four-params",
		epl:    "select a.during(b, 1, 2, 3) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.during(b,1,2,3)': Parameters mismatch for date-time method 'during', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing maximum distance interval both start and end, or an expression providing timestamp or timestamped-event and an expression providing minimum distance interval both start and end and an expression providing maximum distance interval both start and end, or an expression providing timestamp or timestamped-event and an expression providing minimum distance start and an expression providing maximum distance start and an expression providing minimum distance end and an expression providing maximum distance end, but receives 4 expressions ",
	},
	{
		label:  "finishes-three-params",
		epl:    "select a.finishes(b, 1, 2) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.finishes(b,1,2)': Parameters mismatch for date-time method 'finishes', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing maximum distance between end timestamps, but receives 3 expressions ",
	},
	{
		label:  "finishes-negative",
		epl:    "select a.finishes(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.finishes(b,-1)': The finishes date-time method does not allow negative threshold value ",
	},
	{
		label:  "finishedby-negative",
		epl:    "select a.finishedby(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.finishedby(b,-1)': The finishedby date-time method does not allow negative threshold value ",
	},
	{
		label:  "meets-three-params",
		epl:    "select a.meets(b, 1, 2) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.meets(b,1,2)': Parameters mismatch for date-time method 'meets', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing maximum distance between start and end timestamps, but receives 3 expressions ",
	},
	{
		label:  "meets-negative",
		epl:    "select a.meets(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.meets(b,-1)': The meets date-time method does not allow negative threshold value ",
	},
	{
		label:  "metby-negative",
		epl:    "select a.metBy(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.metBy(b,-1)': The metBy date-time method does not allow negative threshold value ",
	},
	{
		label:  "overlaps-four-params",
		epl:    "select a.overlaps(b, 1, 2, 3) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.overlaps(b,1,2,3)': Parameters mismatch for date-time method 'overlaps', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing maximum distance interval both start and end, or an expression providing timestamp or timestamped-event and an expression providing minimum distance interval both start and end and an expression providing maximum distance interval both start and end, but receives 4 expressions ",
	},
	{
		label:  "starts-four-params",
		epl:    "select a.starts(b, 1, 2, 3) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.starts(b,1,2,3)': Parameters mismatch for date-time method 'starts', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing maximum distance between start timestamps, but receives 4 expressions ",
	},
	{
		label:  "starts-negative",
		epl:    "select a.starts(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.starts(b,-1)': The starts date-time method does not allow negative threshold value ",
	},
	{
		label:  "startedby-negative",
		epl:    "select a.startedBy(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b",
		expect: "Failed to validate select-clause expression 'a.startedBy(b,-1)': The startedBy date-time method does not allow negative threshold value ",
	},
}

// buildError runs one expected-invalid probe: the pinned EPL is verified,
// the nearest expressible Go rejection is checked when the probe is
// representable, and the pinned Java prefix is recorded either way.
func (s *exprDTIntervalOpsCaseState) buildError(step compat.Step) error {
	probe, ok := exprDTIntervalOpsProbeByLabel(step.Statement)
	if !ok || step.Epl != probe.epl || step.ExpectError != probe.expect {
		return fmt.Errorf("%s: build-error probe %q is not pinned", exprDTIntervalOpsID, step.Statement)
	}
	if probe.go_ != nil {
		buildErr := probe.go_(s.env)
		if buildErr == nil {
			return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", exprDTIntervalOpsID, step.Statement)
		}
		if !errors.Is(buildErr, esper.ErrorInvalidRule) ||
			(probe.goSub != "" && !strings.Contains(buildErr.Error(), probe.goSub)) {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", exprDTIntervalOpsID, step.Statement, buildErr)
		}
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

func exprDTIntervalOpsProbeByLabel(label string) (exprDTIntervalOpsProbe, bool) {
	for _, probe := range exprDTIntervalOpsProbes {
		if probe.label == label {
			return probe, true
		}
	}
	return exprDTIntervalOpsProbe{}, false
}

func (s *exprDTIntervalOpsCaseState) undeployAll(ctx context.Context) error {
	if s.deployment == nil {
		return nil
	}
	if err := s.deployment.Undeploy(ctx); err != nil {
		return err
	}
	s.deployment = nil
	return nil
}

// exprDTIntervalOpsCaseSteps pins the complete step sequence per case as
// op|case|statement|eventType|epl|payload|expectError keys so the loader
// asserts the scenario file matches the contract byte-for-byte.
var exprDTIntervalOpsCaseSteps = func() map[string][]string {
	steps := map[string][]string{}
	calendar := []string{}
	for _, variant := range exprDTIntervalOpsVariants {
		for _, fieldType := range exprDTIntervalOpsFieldTypes {
			label := variant.name + "-" + strings.ToLower(fieldType)
			calendar = append(calendar,
				"deploy|calendar-ops|"+label+"||"+exprDTIntervalOpsDeployEPLs[label]+"||",
				fmt.Sprintf("send|calendar-ops||B_%s||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":%d}|",
					fieldType, variant.seedDuration))
			for _, row := range variant.rows {
				calendar = append(calendar,
					fmt.Sprintf("send|calendar-ops||A_%s||{\"start\":\"%s\",\"duration\":%d,\"expected\":%t}|",
						fieldType, row.start, row.duration, row.expected))
			}
		}
	}
	calendar = append(calendar, "undeploy-all|calendar-ops|||||")
	steps["calendar-ops"] = calendar
	steps["point-in-time-calendar"] = []string{
		"deploy|point-in-time-calendar|s0||" + exprDTIntervalOpsPointEPL + "||",
		"send|point-in-time-calendar||SupportBean||{\"longPrimitive\":1022749200000}|",
		"send|point-in-time-calendar||SupportDateTime||{\"date\":\"2002-05-30T09:00:00.000Z\",\"expected\":true}|",
		"send|point-in-time-calendar||SupportDateTime||{\"date\":\"2003-05-30T08:00:00.000Z\",\"expected\":false}|",
		"undeploy-all|point-in-time-calendar|||||",
	}
	invalid := []string{}
	for _, probe := range exprDTIntervalOpsProbes {
		invalid = append(invalid, "build-error|invalid|"+probe.label+"||"+probe.epl+"||"+probe.expect)
	}
	invalid = append(invalid, "undeploy-all|invalid|||||")
	steps["invalid"] = invalid
	return steps
}()

// loadExprDTIntervalOpsScenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned metadata and case metadata, and the complete
// pinned step sequence so unknown, duplicated or drifted content fails the
// replay.
func loadExprDTIntervalOpsScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprDTIntervalOpsID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprDTIntervalOpsID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTIntervalOpsID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTIntervalOpsID, err)
	}
	if err := requireExprDTIntervalOpsFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprDTIntervalOpsID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprDTIntervalOpsID ||
		metadata.Description != exprDTIntervalOpsDescription ||
		metadata.JavaCommit != exprDTIntervalOpsJavaCommit ||
		metadata.JavaSource != exprDTIntervalOpsJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprDTIntervalOpsID)
	}
	if err := validateExprDTIntervalOpsStringArray(root["javaRuntimes"], exprDTIntervalOpsJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTIntervalOpsStringArray(root["javaNames"], exprDTIntervalOpsJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTIntervalOpsStringArray(root["javaStaticIds"], exprDTIntervalOpsJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTIntervalOpsStringArray(root["javaFlags"], exprDTIntervalOpsJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprDTIntervalOpsCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", exprDTIntervalOpsID, len(exprDTIntervalOpsCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprDTIntervalOpsFields(object,
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
		if definition.Case != exprDTIntervalOpsCases[index] ||
			definition.Ordinal != exprDTIntervalOpsOrdinals[index] ||
			definition.RuntimeID != exprDTIntervalOpsJavaRuntimeIDs[index] ||
			definition.ExecutionName != exprDTIntervalOpsJavaExecutions[index] ||
			definition.Observation != exprDTIntervalOpsCaseObservations[index] ||
			definition.EPL != exprDTIntervalOpsCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprDTIntervalOpsID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", exprDTIntervalOpsID, err)
	}
	offset := 0
	for _, caseName := range exprDTIntervalOpsCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", exprDTIntervalOpsID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTIntervalOpsID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", exprDTIntervalOpsID, offset, caseName)
		}
		if _, err := exprDTIntervalOpsStepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTIntervalOpsID, offset, err)
		}
		offset++
		want, ok := exprDTIntervalOpsCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", exprDTIntervalOpsID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", exprDTIntervalOpsID, caseName)
		}
		for _, pinned := range want {
			key, err := exprDTIntervalOpsStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTIntervalOpsID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", exprDTIntervalOpsID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", exprDTIntervalOpsID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTIntervalOpsID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// exprDTIntervalOpsStepKey renders one raw step as its pinned key:
// op|case|statement|eventType|epl|payload|expectError with the payload
// compacted. Unknown fields on the step object are rejected per op.
func exprDTIntervalOpsStepKey(raw json.RawMessage) (string, error) {
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
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"send":         {"op", "case", "eventType", "payload"},
		"build-error":  {"op", "case", "statement", "epl", "expectError"},
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
		"|" + step.Epl + "|" + payloadText + "|" + step.ExpectError, nil
}

func requireExprDTIntervalOpsFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", exprDTIntervalOpsID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", exprDTIntervalOpsID, name)
		}
	}
	return nil
}

func validateExprDTIntervalOpsStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
