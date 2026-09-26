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

// expr_dt_with_minmax_549.go replays the four ExprDTWithMax/ExprDTWithMin
// executions (Draft 4.549) against the pinned Java oracle. Each case runs on
// a fresh runtime: deploy the byte-exact s0 select, pin the assertStmtTypes
// property-type surface through a types record, send one SupportDateTime
// bean built exactly like SupportDateTime.make under the system default
// (pinned UTC) timezone, verify the delivered row against the pinned
// expected columns like assertPropsNew, then undeployAll:
//
//   - with-max-input (ord 0, ExprDTWithMaxInput): withMax('month') over all
//     five representations of 2002-05-30T09:00:00.000 emits
//     2002-12-30T09:00:00.000 in val0..val4. DATE/LONGBOXED/CALENDAR/
//     LOCALDATETIME/ZONEDDATETIME collapse to int64/time.Time reps.
//   - with-max-fields (ord 1, ExprDTWithMaxFields): withMax over
//     msec,sec,minutes,hour,day,month,year,week on utildate. 'week' clamps
//     WEEK_OF_YEAR keeping the day-of-week (Thu -> 2002-12-26); 'year'
//     reaches year 292278994 whose epoch-millis wraps int64 to
//     9223372030035600000 — the raw wrapped value is pinned, never
//     reinterpreted.
//   - with-min-input (ord 0, ExprDTWithMinInput): withMin('month') over the
//     same five representations emits 2002-01-30T09:00:00.000.
//   - with-min-fields (ord 1, ExprDTWithMinFields): withMin over the same
//     eight units at 2002-05-30T09:01:02.003; 'year' reaches year 1
//     (-62122863537997) and 'week' clamps to week 1 keeping Thursday
//     (2002-01-03).
//
// Instant-valued cells render as epoch-millis numbers on both sides, the
// ExprDTRound/ExprDTDataSources instant-token convention.

const exprDTWithMinMax549ID = "expr-dt-with-minmax-549"
const exprDTWithMinMax549JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

// exprDTWithMinMax549Source is the scenario javaSource pin: the suite
// directory shared by both source files (the event-infra-545 multi-source
// precedent); the file-level list lives in exprDTWithMinMax549JavaSources.
const exprDTWithMinMax549Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime"

var exprDTWithMinMax549JavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTWithMax.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTWithMin.java",
}

var exprDTWithMinMax549JavaRuntimeIDs = []string{
	"java-runtime-097a5db3742ae25c3b0b",
	"java-runtime-c6bba43af400a7375075",
	"java-runtime-3f79b4bf903cca74fe6a",
	"java-runtime-8ca773479bf038c94597",
}

var exprDTWithMinMax549JavaExecutions = []string{
	"ExprDTWithMaxInput",
	"ExprDTWithMaxFields",
	"ExprDTWithMinInput",
	"ExprDTWithMinFields",
}

var exprDTWithMinMax549Cases = []string{
	"with-max-input",
	"with-max-fields",
	"with-min-input",
	"with-min-fields",
}

var exprDTWithMinMax549CaseRuntimeIDs = map[string]string{
	"with-max-input":  "java-runtime-097a5db3742ae25c3b0b",
	"with-max-fields": "java-runtime-c6bba43af400a7375075",
	"with-min-input":  "java-runtime-3f79b4bf903cca74fe6a",
	"with-min-fields": "java-runtime-8ca773479bf038c94597",
}

var exprDTWithMinMax549JavaStaticIDs = []string{
	"java-d4afe3864ba41bfd7918",
	"java-d87f60ced41e2a52719f",
	"java-2224592eef4aca9b4e7b",
	"java-f3bf884f776301d91900",
}

var exprDTWithMinMax549JavaFlags = []string{}

var exprDTWithMinMax549Ordinals = []int{0, 1, 0, 1}

const exprDTWithMinMax549Description = "ExprDTWithMax/ExprDTWithMin executions: with-max-input replays " +
	"ExprDTWithMaxInput (withMax('month') over all five SupportDateTime representations of " +
	"2002-05-30T09:00:00.000 emits 2002-12-30T09:00:00.000 in val0..val4), with-max-fields replays " +
	"ExprDTWithMaxFields (withMax over msec,sec,minutes,hour,day,month,year,week on utildate: " +
	"'week' keeps the day-of-week -> 2002-12-26, 'year' wraps epoch-millis to " +
	"9223372030035600000), with-min-input replays ExprDTWithMinInput (withMin('month') emits " +
	"2002-01-30T09:00:00.000), with-min-fields replays ExprDTWithMinFields (withMin at " +
	"2002-05-30T09:01:02.003: 'year' -> 0001-05-30 epoch-millis -62122863537997, 'week' -> " +
	"2002-01-03)."

var exprDTWithMinMax549CaseObservations = []string{
	"listener+types; deploy s0 withMax('month') over all five representations, types pins " +
		"{Date,Long,Calendar,LocalDateTime,ZonedDateTime}, send make(2002-05-30T09:00:00.000) " +
		"emits 2002-12-30T09:00:00.000 epoch-millis in val0..val4",
	"listener+types; deploy s0 withMax over msec,sec,minutes,hour,day,month,year,week on " +
		"utildate, types pins all-Date, send make(2002-05-30T09:00:00.000) emits " +
		"{...999, ...59.000, ...59:00, ...23:00, 2002-05-31, 2002-12-30, wrapped-year " +
		"9223372030035600000, 2002-12-26}",
	"listener+types; deploy s0 withMin('month') over all five representations, types pins " +
		"{Date,Long,Calendar,LocalDateTime,ZonedDateTime}, send make(2002-05-30T09:00:00.000) " +
		"emits 2002-01-30T09:00:00.000 epoch-millis in val0..val4",
	"listener+types; deploy s0 withMin over msec,sec,minutes,hour,day,month,year,week on " +
		"utildate, types pins all-Date, send make(2002-05-30T09:01:02.003) emits " +
		"{...02.000, ...00.003, ...00:02.003, 00:01:02.003, 2002-05-01, 2002-01-30, " +
		"year-1 -62122863537997, 2002-01-03}",
}

// exprDTWithMinMax549InputEPL is the byte-exact withMax ord-0 statement (the
// withMin variant substitutes the method name).
const exprDTWithMinMax549MaxInputEPL = "@name('s0') select " +
	"utildate.withMax('month') as val0," +
	"longdate.withMax('month') as val1," +
	"caldate.withMax('month') as val2," +
	"localdate.withMax('month') as val3," +
	"zoneddate.withMax('month') as val4" +
	" from SupportDateTime"
const exprDTWithMinMax549MinInputEPL = "@name('s0') select " +
	"utildate.withMin('month') as val0," +
	"longdate.withMin('month') as val1," +
	"caldate.withMin('month') as val2," +
	"localdate.withMin('month') as val3," +
	"zoneddate.withMin('month') as val4" +
	" from SupportDateTime"

// exprDTWithMinMax549FieldsEPL is the byte-exact withMax ord-1 statement:
// eight utildate clamps including the 'minutes'/'msec'/'sec' aliases and the
// WEEK_OF_YEAR 'week' field.
const exprDTWithMinMax549MaxFieldsEPL = "@name('s0') select " +
	"utildate.withMax('msec') as val0," +
	"utildate.withMax('sec') as val1," +
	"utildate.withMax('minutes') as val2," +
	"utildate.withMax('hour') as val3," +
	"utildate.withMax('day') as val4," +
	"utildate.withMax('month') as val5," +
	"utildate.withMax('year') as val6," +
	"utildate.withMax('week') as val7" +
	" from SupportDateTime"
const exprDTWithMinMax549MinFieldsEPL = "@name('s0') select " +
	"utildate.withMin('msec') as val0," +
	"utildate.withMin('sec') as val1," +
	"utildate.withMin('minutes') as val2," +
	"utildate.withMin('hour') as val3," +
	"utildate.withMin('day') as val4," +
	"utildate.withMin('month') as val5," +
	"utildate.withMin('year') as val6," +
	"utildate.withMin('week') as val7" +
	" from SupportDateTime"

// exprDTWithMinMax549DeployEPLs pins each case's single s0 deployment.
var exprDTWithMinMax549DeployEPLs = map[string]string{
	"with-max-input":  exprDTWithMinMax549MaxInputEPL,
	"with-max-fields": exprDTWithMinMax549MaxFieldsEPL,
	"with-min-input":  exprDTWithMinMax549MinInputEPL,
	"with-min-fields": exprDTWithMinMax549MinFieldsEPL,
}

// exprDTWithMinMax549CaseEPLs pins each case's representative EPL for the
// case metadata.
var exprDTWithMinMax549CaseEPLs = []string{
	exprDTWithMinMax549MaxInputEPL,
	exprDTWithMinMax549MaxFieldsEPL,
	exprDTWithMinMax549MinInputEPL,
	exprDTWithMinMax549MinFieldsEPL,
}

// exprDTWithMinMax549T00 is DateTime.parseDefaultMSec("2002-05-30T09:00:00.000"),
// the input-cases' and with-max-fields' send instant.
const exprDTWithMinMax549T00 = "2002-05-30T09:00:00.000Z"

// exprDTWithMinMax549T03 is DateTime.parseDefaultMSec("2002-05-30T09:01:02.003"),
// the with-min-fields send instant.
const exprDTWithMinMax549T03 = "2002-05-30T09:01:02.003Z"

// exprDTWithMinMax549Units is the eight-field clamp order the *Fields
// executions iterate.
var exprDTWithMinMax549Units = []string{"msec", "sec", "minutes", "hour", "day", "month", "year", "week"}

// exprDTWithMinMax549ExpectedMillis pins every send's assertPropsNew values
// as epoch millis (the oracle's instant-token rendering):
//
//   - with-max-input: all five reps 2002-12-30T09:00:00.000.
//   - with-max-fields: 09:00:00.999, 09:00:59.000, 09:59:00.000,
//     23:00:00.000, 2002-05-31, 2002-12-30, the int64-wrapped year-max
//     9223372030035600000 (pinned raw, never normalized), 2002-12-26.
//   - with-min-input: all five reps 2002-01-30T09:00:00.000.
//   - with-min-fields: 09:01:02.000, 09:01:00.003, 09:00:02.003,
//     00:01:02.003, 2002-05-01, 2002-01-30, year-1 -62122863537997,
//     2002-01-03.
var exprDTWithMinMax549ExpectedMillis = map[string][]int64{
	"with-max-input": {1041238800000, 1041238800000, 1041238800000, 1041238800000, 1041238800000},
	"with-max-fields": {1022749200999, 1022749259000, 1022752740000, 1022799600000,
		1022835600000, 1041238800000, 9223372030035600000, 1040893200000},
	"with-min-input": {1012381200000, 1012381200000, 1012381200000, 1012381200000, 1012381200000},
	"with-min-fields": {1022749262000, 1022749260003, 1022749202003, 1022716862003,
		1020243662003, 1012381262003, -62122863537997, 1010048462003},
}

// exprDTWithMinMax549SendDates pins each case's single send instant.
var exprDTWithMinMax549SendDates = map[string]string{
	"with-max-input":  exprDTWithMinMax549T00,
	"with-max-fields": exprDTWithMinMax549T00,
	"with-min-input":  exprDTWithMinMax549T00,
	"with-min-fields": exprDTWithMinMax549T03,
}

// exprDTWithMinMax549TypeProperties pins the Java-asserted property types:
// the input cases assert DATE/LONGBOXED/CALENDAR/LOCALDATETIME/ZONEDDATETIME
// per column; the fields cases assert all-Date. The recorded simple names
// mirror assertStmtTypes; Go verifies the collapsed int64/time.Time schema.
var exprDTWithMinMax549TypeProperties = map[string]map[string]string{
	"with-max-input": {
		"val0": "Date", "val1": "Long", "val2": "Calendar",
		"val3": "LocalDateTime", "val4": "ZonedDateTime",
	},
	"with-max-fields": {
		"val0": "Date", "val1": "Date", "val2": "Date", "val3": "Date",
		"val4": "Date", "val5": "Date", "val6": "Date", "val7": "Date",
	},
	"with-min-input": {
		"val0": "Date", "val1": "Long", "val2": "Calendar",
		"val3": "LocalDateTime", "val4": "ZonedDateTime",
	},
	"with-min-fields": {
		"val0": "Date", "val1": "Date", "val2": "Date", "val3": "Date",
		"val4": "Date", "val5": "Date", "val6": "Date", "val7": "Date",
	},
}

type exprDTWithMinMax549CaseState struct {
	caseName   string
	env        *esper.Environment
	engine     *esper.Engine
	trace      *compat.Trace
	sequence   uint64
	deployment *esper.Deployment
	plan       esper.Plan
	fired      bool
	row        compat.ResultRecord
}

func runExprDTWithMinMax549Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range exprDTWithMinMax549Cases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runExprDTWithMinMax549Case(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprDTWithMinMax549ID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprDTWithMinMax549ID, scenario.ID)
	}
	return trace, nil
}

func runExprDTWithMinMax549Case(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterMap(env, "SupportDateTime", exprDTWithMinMax549FieldSpecs()); err != nil {
		return compat.Trace{}, fmt.Errorf("%s register SupportDateTime: %w", exprDTWithMinMax549ID, err)
	}
	state := &exprDTWithMinMax549CaseState{
		caseName: caseName,
		env:      env,
		engine: esper.NewEngine(env,
			esper.WithStartTime(time.Unix(0, 0).UTC()),
			esper.WithRuntimeURI(exprDTWithMinMax549CaseRuntimeIDs[caseName])),
		trace: &compat.Trace{Version: scenario.Version, ID: scenario.ID},
	}
	defer func() { _ = state.engine.Close(context.Background()) }()

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
		case "types":
			if err := state.types(step); err != nil {
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
			return *state.trace, fmt.Errorf("%s: unsupported step op %q", exprDTWithMinMax549ID, step.Op)
		}
	}
	return *state.trace, nil
}

// exprDTWithMinMax549FieldSpecs mirrors SupportDateTime: longdate is the
// epoch-millis Long and the other four representations collapse to
// time.Time.
func exprDTWithMinMax549FieldSpecs() []esper.FieldSpec {
	return []esper.FieldSpec{
		esper.FieldDef("longdate", reflect.TypeOf(int64(0))),
		esper.FieldDef("utildate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("caldate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("localdate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("zoneddate", reflect.TypeOf(time.Time{})),
	}
}

// deploy mirrors one compileDeploy + addListener('s0') cycle; the pinned EPL
// is verified before the fluent equivalent is built.
func (s *exprDTWithMinMax549CaseState) deploy(ctx context.Context, step compat.Step) error {
	pinned, ok := exprDTWithMinMax549DeployEPLs[s.caseName]
	if !ok || step.Statement != "s0" || step.Epl != pinned {
		return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q",
			exprDTWithMinMax549ID, s.caseName, step.Statement, step.Epl)
	}
	if err := s.undeployAll(ctx); err != nil {
		return err
	}
	query, err := s.buildSelect()
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
	s.plan = plan
	for _, statement := range deployment.Statements() {
		captured := statement
		if _, err := captured.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			newRows := compat.NormalizeResults(batch.New)
			exprDTWithMinMax549RenderMillis(newRows)
			if len(newRows) == 0 {
				return nil
			}
			s.fired = true
			s.row = newRows[0]
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

// buildSelect renders the fluent equivalent of the case's pinned s0 select.
// utildate/longdate/caldate/localdate/zoneddate collapse to the int64 and
// time.Time reps; withMax/withMin preserve the input representation.
func (s *exprDTWithMinMax549CaseState) buildSelect() (esper.Query, error) {
	longdate := esper.Field[map[string]any, int64]("longdate")
	utildate := esper.Field[map[string]any, time.Time]("utildate")
	caldate := esper.Field[map[string]any, time.Time]("caldate")
	localdate := esper.Field[map[string]any, time.Time]("localdate")
	zoneddate := esper.Field[map[string]any, time.Time]("zoneddate")
	switch s.caseName {
	case "with-max-input":
		return esper.FromAny(s.env, "SupportDateTime").Select(
			esper.Alias("val0", esper.DateTimeWithMax[time.Time](utildate, "month")),
			esper.Alias("val1", esper.DateTimeWithMax[int64](longdate, "month")),
			esper.Alias("val2", esper.DateTimeWithMax[time.Time](caldate, "month")),
			esper.Alias("val3", esper.DateTimeWithMax[time.Time](localdate, "month")),
			esper.Alias("val4", esper.DateTimeWithMax[time.Time](zoneddate, "month")),
		).Query(esper.StatementName("s0")), nil
	case "with-min-input":
		return esper.FromAny(s.env, "SupportDateTime").Select(
			esper.Alias("val0", esper.DateTimeWithMin[time.Time](utildate, "month")),
			esper.Alias("val1", esper.DateTimeWithMin[int64](longdate, "month")),
			esper.Alias("val2", esper.DateTimeWithMin[time.Time](caldate, "month")),
			esper.Alias("val3", esper.DateTimeWithMin[time.Time](localdate, "month")),
			esper.Alias("val4", esper.DateTimeWithMin[time.Time](zoneddate, "month")),
		).Query(esper.StatementName("s0")), nil
	case "with-max-fields", "with-min-fields":
		selections := make([]esper.Selection, 0, len(exprDTWithMinMax549Units))
		for index, unit := range exprDTWithMinMax549Units {
			var expression esper.Expression[time.Time]
			if s.caseName == "with-max-fields" {
				expression = esper.DateTimeWithMax[time.Time](utildate, unit)
			} else {
				expression = esper.DateTimeWithMin[time.Time](utildate, unit)
			}
			selections = append(selections, esper.Alias(fmt.Sprintf("val%d", index), expression))
		}
		return esper.FromAny(s.env, "SupportDateTime").Select(selections...).
			Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("%s: unsupported case %q", exprDTWithMinMax549ID, s.caseName)
	}
}

// types emits the pinned property-type record after verifying the deployed
// statement's collapsed Go schema: the input cases' val1 (longdate rep) is
// int64-typed and every other column is time.Time-typed, matching the
// Java-asserted LONGBOXED vs date-time types.
func (s *exprDTWithMinMax549CaseState) types(step compat.Step) error {
	properties, ok := exprDTWithMinMax549TypeProperties[s.caseName]
	if !ok || step.Statement != "s0" {
		return fmt.Errorf("%s: case %q has no types assertion for %q",
			exprDTWithMinMax549ID, s.caseName, step.Statement)
	}
	schema, ok := s.plan.ResultSchema()
	if !ok {
		return fmt.Errorf("%s: statement %q has no result schema", exprDTWithMinMax549ID, step.Statement)
	}
	columns, err := exprDTWithMinMax549Columns(s.caseName)
	if err != nil {
		return err
	}
	value := map[string]any{}
	for _, column := range columns {
		field, exists := schema.Field(column)
		if !exists {
			return fmt.Errorf("%s: s0 is missing column %q", exprDTWithMinMax549ID, column)
		}
		want := reflect.TypeOf(time.Time{})
		if properties[column] == "Long" {
			want = reflect.TypeOf(int64(0))
		}
		if field.Type != want {
			return fmt.Errorf("%s: s0 %s type drift: %v", exprDTWithMinMax549ID, column, field.Type)
		}
		value[column] = properties[column]
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "types",
		Statement: step.Statement,
		Time:      compat.FormatTraceTime(s.engine.Now()),
		Value:     map[string]any{"properties": value},
	})
	return nil
}

// send decodes the payload, rebuilds SupportDateTime.make's five
// representations from the one ISO instant (the Calendar rep forces
// MILLISECOND to zero), delivers the map event, and verifies the delivered
// row against the pinned expected epoch-millis values — the assertPropsNew
// equivalent.
func (s *exprDTWithMinMax549CaseState) send(ctx context.Context, step compat.Step) error {
	if step.EventType != "SupportDateTime" {
		return fmt.Errorf("%s: unsupported event type %q", exprDTWithMinMax549ID, step.EventType)
	}
	var payload struct {
		Date     string  `json:"date"`
		Expected []int64 `json:"expected"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return fmt.Errorf("%s: decode SupportDateTime: %w", exprDTWithMinMax549ID, err)
	}
	if payload.Date != exprDTWithMinMax549SendDates[s.caseName] {
		return fmt.Errorf("%s: case %q send date %q is not pinned",
			exprDTWithMinMax549ID, s.caseName, payload.Date)
	}
	parsed, err := time.Parse(time.RFC3339Nano, payload.Date)
	if err != nil {
		return fmt.Errorf("%s: parse SupportDateTime date: %w", exprDTWithMinMax549ID, err)
	}
	s.fired = false
	s.row = compat.ResultRecord{}
	if err := s.engine.SendRecord(ctx, "SupportDateTime", map[string]any{
		"longdate":  parsed.UnixMilli(),
		"utildate":  parsed,
		"caldate":   parsed.Add(-time.Duration(parsed.Nanosecond())),
		"localdate": parsed,
		"zoneddate": parsed,
	}); err != nil {
		return err
	}
	if !s.fired {
		return fmt.Errorf("%s: SupportDateTime send produced no listener row", exprDTWithMinMax549ID)
	}
	return s.verifyExpected(payload.Expected)
}

// verifyExpected compares the delivered row's fields against the pinned
// expected epoch-millis values in select order, mirroring assertPropsNew.
func (s *exprDTWithMinMax549CaseState) verifyExpected(expected []int64) error {
	columns, err := exprDTWithMinMax549Columns(s.caseName)
	if err != nil {
		return err
	}
	if len(expected) != len(columns) {
		return fmt.Errorf("%s: %s send pins %d expected values, want %d",
			exprDTWithMinMax549ID, s.caseName, len(expected), len(columns))
	}
	for index, column := range columns {
		actual, ok := s.row.Fields[column]
		if !ok {
			return fmt.Errorf("%s: listener row is missing column %q", exprDTWithMinMax549ID, column)
		}
		got, ok := actual.(int64)
		if !ok || got != expected[index] {
			return fmt.Errorf("%s: column %q = %v, want %v", exprDTWithMinMax549ID, column, actual, expected[index])
		}
	}
	return nil
}

// exprDTWithMinMax549Columns returns the select-order column names a send's
// expected values pin: val0..val4 for the input cases, val0..val7 for the
// fields cases.
func exprDTWithMinMax549Columns(caseName string) ([]string, error) {
	count := 0
	switch caseName {
	case "with-max-input", "with-min-input":
		count = 5
	case "with-max-fields", "with-min-fields":
		count = 8
	default:
		return nil, fmt.Errorf("%s: unsupported case %q", exprDTWithMinMax549ID, caseName)
	}
	columns := make([]string, 0, count)
	for index := range count {
		columns = append(columns, fmt.Sprintf("val%d", index))
	}
	return columns, nil
}

// exprDTWithMinMax549RenderMillis renders clamped time.Time cells as epoch
// millis, the oracle's instant-token convention for every SupportDateTime
// representation.
func exprDTWithMinMax549RenderMillis(rows []compat.ResultRecord) {
	for _, row := range rows {
		for name, value := range row.Fields {
			if located, ok := value.(time.Time); ok {
				row.Fields[name] = located.UnixMilli()
			}
		}
	}
}

// undeployAll tears down the live deployment, mirroring the execution's
// env.undeployAll().
func (s *exprDTWithMinMax549CaseState) undeployAll(ctx context.Context) error {
	if s.deployment != nil {
		if err := s.deployment.Undeploy(ctx); err != nil {
			return err
		}
		s.deployment = nil
	}
	return nil
}

// exprDTWithMinMax549CaseSteps pins the complete step sequence per case as
// op|case|statement|eventType|epl|payload|expectError|at keys so the loader
// asserts the scenario file matches the contract byte-for-byte.
var exprDTWithMinMax549CaseSteps = func() map[string][]string {
	steps := map[string][]string{}
	for _, caseName := range exprDTWithMinMax549Cases {
		expected, err := json.Marshal(exprDTWithMinMax549ExpectedMillis[caseName])
		if err != nil {
			panic(err)
		}
		payload := fmt.Sprintf(`{"date":%q,"expected":%s}`, exprDTWithMinMax549SendDates[caseName], expected)
		steps[caseName] = []string{
			"deploy|" + caseName + "|s0||" + exprDTWithMinMax549DeployEPLs[caseName] + "|||",
			"types|" + caseName + "|s0|||||",
			"send|" + caseName + "||SupportDateTime||" + payload + "||",
			"undeploy-all|" + caseName + "||||||",
		}
	}
	return steps
}()

// loadExprDTWithMinMax549Scenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned metadata and case metadata, and the complete
// pinned step sequence so unknown, duplicated or drifted content fails the
// replay.
func loadExprDTWithMinMax549Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprDTWithMinMax549ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprDTWithMinMax549ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTWithMinMax549ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTWithMinMax549ID, err)
	}
	if err := requireExprDTWithMinMax549Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprDTWithMinMax549ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprDTWithMinMax549ID ||
		metadata.Description != exprDTWithMinMax549Description ||
		metadata.JavaCommit != exprDTWithMinMax549JavaCommit ||
		metadata.JavaSource != exprDTWithMinMax549Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprDTWithMinMax549ID)
	}
	if err := validateExprDTWithMinMax549StringArray(root["javaRuntimes"], exprDTWithMinMax549JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTWithMinMax549StringArray(root["javaNames"], exprDTWithMinMax549JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTWithMinMax549StringArray(root["javaStaticIds"], exprDTWithMinMax549JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTWithMinMax549StringArray(root["javaFlags"], exprDTWithMinMax549JavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprDTWithMinMax549Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", exprDTWithMinMax549ID, len(exprDTWithMinMax549Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprDTWithMinMax549Fields(object,
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
		if definition.Case != exprDTWithMinMax549Cases[index] ||
			definition.Ordinal != exprDTWithMinMax549Ordinals[index] ||
			definition.RuntimeID != exprDTWithMinMax549JavaRuntimeIDs[index] ||
			definition.ExecutionName != exprDTWithMinMax549JavaExecutions[index] ||
			definition.Observation != exprDTWithMinMax549CaseObservations[index] ||
			definition.EPL != exprDTWithMinMax549CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprDTWithMinMax549ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", exprDTWithMinMax549ID, err)
	}
	offset := 0
	for _, caseName := range exprDTWithMinMax549Cases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", exprDTWithMinMax549ID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTWithMinMax549ID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", exprDTWithMinMax549ID, offset, caseName)
		}
		if _, err := exprDTWithMinMax549StepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTWithMinMax549ID, offset, err)
		}
		offset++
		want, ok := exprDTWithMinMax549CaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", exprDTWithMinMax549ID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", exprDTWithMinMax549ID, caseName)
		}
		for _, pinned := range want {
			key, err := exprDTWithMinMax549StepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTWithMinMax549ID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", exprDTWithMinMax549ID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", exprDTWithMinMax549ID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTWithMinMax549ID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// exprDTWithMinMax549StepKey renders one raw step as its pinned key:
// op|case|statement|eventType|epl|payload|expectError|at with the payload
// compacted. Unknown fields on the step object are rejected per op.
func exprDTWithMinMax549StepKey(raw json.RawMessage) (string, error) {
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
		"deploy":       {"op", "case", "statement", "epl"},
		"types":        {"op", "case", "statement"},
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

func requireExprDTWithMinMax549Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", exprDTWithMinMax549ID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", exprDTWithMinMax549ID, name)
		}
	}
	return nil
}

func validateExprDTWithMinMax549StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
