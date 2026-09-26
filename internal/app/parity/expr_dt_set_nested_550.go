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

// expr_dt_set_nested_550.go replays the three ExprDTSet/ExprDTNested
// executions (Draft 4.550) against the pinned Java oracle. Each case runs on
// a fresh runtime: deploy the byte-exact s0 select, pin the assertStmtTypes
// property-type surface through a types record, send one SupportDateTime
// bean built exactly like SupportDateTime.make under the system default
// (pinned UTC) timezone, verify the delivered row against the pinned
// expected columns like assertPropsNew, then undeployAll:
//
//   - set-input (ord 0, ExprDTSetInput): set('month',0) on the
//     Calendar/long reps and set('month',1) on the LDT reps all land on
//     January — 2002-01-30T09:00:00.000 in val0..val4. Go calls the 1-based
//     DateTimeSet(v,"month",1) uniformly. DATE/LONGBOXED/CALENDAR/
//     LOCALDATETIME/ZONEDDATETIME collapse to int64/time.Time reps.
//   - set-fields (ord 1, ExprDTSetFields): set over
//     msec,sec,minutes,hour,day,month,year,week on utildate. 'month' 6 is
//     Java's 0-based Calendar month (July; Go passes 7), 'year' 7 resolves
//     pre-cutover on the Julian calendar (0007-05-30 epoch-millis
//     -61933561200000), 'week' sets WEEK_OF_YEAR 8 keeping the day-of-week
//     (Thu -> 2002-02-21).
//   - nested (ord 0, ExprDTNested, milestone indices 0 and 1): every representation
//     chains set('hour',1).set('minute',2).set('second',3) emitting
//     2002-05-30T01:02:03.000 with representation-preserving types, then
//     undeploys and redeploys the same chains suffixed .toCalendar() so all
//     five columns collapse to Calendar emitting the same instant.
//
// Instant-valued cells render as epoch-millis numbers on both sides, the
// ExprDTRound/ExprDTDataSources instant-token convention.

const exprDTSetNested550ID = "expr-dt-set-nested-550"
const exprDTSetNested550JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

// exprDTSetNested550Source is the scenario javaSource pin: the suite
// directory shared by both source files (the event-infra-545 multi-source
// precedent); the file-level list lives in exprDTSetNested550JavaSources.
const exprDTSetNested550Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime"

var exprDTSetNested550JavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTSet.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTNested.java",
}

var exprDTSetNested550JavaRuntimeIDs = []string{
	"java-runtime-94b9e2a9baff63289a84",
	"java-runtime-8eeb0fc84eee8e2f3676",
	"java-runtime-05ab4e5671dc2fc9e434",
}

var exprDTSetNested550JavaExecutions = []string{
	"ExprDTSetInput",
	"ExprDTSetFields",
	"ExprDTNested",
}

var exprDTSetNested550Cases = []string{
	"set-input",
	"set-fields",
	"nested",
}

var exprDTSetNested550CaseRuntimeIDs = map[string]string{
	"set-input":  "java-runtime-94b9e2a9baff63289a84",
	"set-fields": "java-runtime-8eeb0fc84eee8e2f3676",
	"nested":     "java-runtime-05ab4e5671dc2fc9e434",
}

var exprDTSetNested550JavaStaticIDs = []string{
	"java-295d51f5c010279653b5",
	"java-295d51f5c010279653b5",
	"java-cf7184473782469f6840",
}

var exprDTSetNested550JavaFlags = []string{}

var exprDTSetNested550Ordinals = []int{0, 1, 0}

const exprDTSetNested550Description = "ExprDTSet/ExprDTNested executions: set-input replays " +
	"ExprDTSetInput (set('month') over all five SupportDateTime representations of " +
	"2002-05-30T09:00:00.000 emits January 2002-01-30T09:00:00.000 in val0..val4), set-fields " +
	"replays ExprDTSetFields (set over msec,sec,minutes,hour,day,month,year,week on utildate: " +
	"'month' 6 -> July 2002-07-30, 'year' 7 -> pre-cutover Julian 0007-05-30 epoch-millis " +
	"-61933561200000, 'week' 8 keeps the day-of-week -> 2002-02-21), nested replays ExprDTNested " +
	"(set('hour',1).set('minute',2).set('second',3) chains emit 2002-05-30T01:02:03.000 across " +
	"all five representations, then the .toCalendar() milestone emits the same instant as " +
	"all-Calendar)."

var exprDTSetNested550CaseObservations = []string{
	"listener+types; deploy s0 set('month') over all five representations, types pins " +
		"{Date,Long,Calendar,LocalDateTime,ZonedDateTime}, send make(2002-05-30T09:00:00.000) " +
		"emits 2002-01-30T09:00:00.000 epoch-millis in val0..val4",
	"listener+types; deploy s0 set over msec,sec,minutes,hour,day,month,year,week on " +
		"utildate, types pins all-Date, send make(2002-05-30T09:00:00.000) emits " +
		"{...00.001, ...00:02.000, ...03:00.000, 13:00:00.000, 2002-05-05, 2002-07-30, " +
		"Julian year-7 -61933561200000, week-8 2002-02-21}",
	"listener+types x2; deploy s0 set('hour',1).set('minute',2).set('second',3) chains, " +
		"types pins {Date,Long,Calendar,LocalDateTime,ZonedDateTime}, send " +
		"make(2002-05-30T09:00:00.000) emits 2002-05-30T01:02:03.000, undeploy, redeploy " +
		"the .toCalendar() milestone, types pins all-Calendar, send emits the same instant",
}

// exprDTSetNested550SetInputEPL is the byte-exact ExprDTSetInput ord-0
// statement: Java's month argument is 0-based for the Calendar-backed
// representations but 1-based for the LDT/ZDT reps; both spell January.
const exprDTSetNested550SetInputEPL = "@name('s0') select " +
	"utildate.set('month', 0) as val0," +
	"longdate.set('month', 0) as val1," +
	"caldate.set('month', 0) as val2," +
	"localdate.set('month', 1) as val3," +
	"zoneddate.set('month', 1) as val4" +
	" from SupportDateTime"

// exprDTSetNested550SetFieldsEPL is the byte-exact ExprDTSetFields ord-1
// statement: eight utildate set() calls including the 'minutes'/'msec'/'sec'
// aliases and the WEEK_OF_YEAR 'week' field.
const exprDTSetNested550SetFieldsEPL = "@name('s0') select " +
	"utildate.set('msec', 1) as val0," +
	"utildate.set('sec', 2) as val1," +
	"utildate.set('minutes', 3) as val2," +
	"utildate.set('hour', 13) as val3," +
	"utildate.set('day', 5) as val4," +
	"utildate.set('month', 6) as val5," +
	"utildate.set('year', 7) as val6," +
	"utildate.set('week', 8) as val7" +
	" from SupportDateTime"

// exprDTSetNested550NestedEPL is the byte-exact ExprDTNested milestone-0
// statement: chained set('hour',1).set('minute',2).set('second',3) on all
// five representations.
const exprDTSetNested550NestedEPL = "@name('s0') select " +
	"utildate.set('hour', 1).set('minute', 2).set('second', 3) as val0," +
	"longdate.set('hour', 1).set('minute', 2).set('second', 3) as val1," +
	"caldate.set('hour', 1).set('minute', 2).set('second', 3) as val2," +
	"localdate.set('hour', 1).set('minute', 2).set('second', 3) as val3," +
	"zoneddate.set('hour', 1).set('minute', 2).set('second', 3) as val4" +
	" from SupportDateTime"

// exprDTSetNested550NestedCalendarEPL is the byte-exact ExprDTNested
// milestone-1 statement: the same chains suffixed .toCalendar().
const exprDTSetNested550NestedCalendarEPL = "@name('s0') select " +
	"utildate.set('hour', 1).set('minute', 2).set('second', 3).toCalendar() as val0," +
	"longdate.set('hour', 1).set('minute', 2).set('second', 3).toCalendar() as val1," +
	"caldate.set('hour', 1).set('minute', 2).set('second', 3).toCalendar() as val2," +
	"localdate.set('hour', 1).set('minute', 2).set('second', 3).toCalendar() as val3," +
	"zoneddate.set('hour', 1).set('minute', 2).set('second', 3).toCalendar() as val4" +
	" from SupportDateTime"

// exprDTSetNested550DeployEPLs pins each case's s0 deployments in order;
// the nested case deploys twice (plain chains, then the .toCalendar()
// milestone).
var exprDTSetNested550DeployEPLs = map[string][]string{
	"set-input":  {exprDTSetNested550SetInputEPL},
	"set-fields": {exprDTSetNested550SetFieldsEPL},
	"nested":     {exprDTSetNested550NestedEPL, exprDTSetNested550NestedCalendarEPL},
}

// exprDTSetNested550CaseEPLs pins each case's representative EPL for the
// case metadata; the nested case's representative is its milestone-0
// statement.
var exprDTSetNested550CaseEPLs = []string{
	exprDTSetNested550SetInputEPL,
	exprDTSetNested550SetFieldsEPL,
	exprDTSetNested550NestedEPL,
}

// exprDTSetNested550T00 is DateTime.parseDefaultMSec("2002-05-30T09:00:00.000"),
// every case's send instant.
const exprDTSetNested550T00 = "2002-05-30T09:00:00.000Z"

// exprDTSetNested550Units is the eight-field set order ExprDTSetFields
// iterates; exprDTSetNested550SetFieldArgs pins the matching 1-based Go
// arguments (Java's 'month' literal 6 is 0-based Calendar-speak for July,
// which the 1-based Go DateTimeSet spells 7).
var exprDTSetNested550Units = []string{"msec", "sec", "minutes", "hour", "day", "month", "year", "week"}
var exprDTSetNested550SetFieldArgs = []int{1, 2, 3, 13, 5, 7, 7, 8}

// exprDTSetNested550ExpectedMillis pins every send's assertPropsNew values
// as epoch millis (the oracle's instant-token rendering):
//
//   - set-input: all five reps 2002-01-30T09:00:00.000 (January).
//   - set-fields: 09:00:00.001, 09:00:02.000, 09:03:00.000, 13:00:00.000,
//     2002-05-05, 2002-07-30, the pre-cutover Julian year-7 0007-05-30
//     -61933561200000 (pinned raw, never normalized), week-8 2002-02-21.
//   - nested: all five reps 2002-05-30T01:02:03.000, identical across the
//     plain-chain and .toCalendar() sends.
var exprDTSetNested550ExpectedMillis = map[string][]int64{
	"set-input": {1012381200000, 1012381200000, 1012381200000, 1012381200000, 1012381200000},
	"set-fields": {1022749200001, 1022749202000, 1022749380000, 1022763600000,
		1020589200000, 1028019600000, -61933561200000, 1014282000000},
	"nested": {1022720523000, 1022720523000, 1022720523000, 1022720523000, 1022720523000},
}

// exprDTSetNested550SendDates pins each case's send instant.
var exprDTSetNested550SendDates = map[string]string{
	"set-input":  exprDTSetNested550T00,
	"set-fields": exprDTSetNested550T00,
	"nested":     exprDTSetNested550T00,
}

// exprDTSetNested550TypeProperties pins the Java-asserted property types
// per types step: set-input and the nested milestone-0 types assert
// DATE/LONGBOXED/CALENDAR/LOCALDATETIME/ZONEDDATETIME per column,
// set-fields asserts all-Date, and the nested milestone-1 types asserts
// all-Calendar. The recorded simple names mirror assertStmtTypes /
// assertStmtTypesAllSame; Go verifies the collapsed int64/time.Time schema.
var exprDTSetNested550TypeProperties = map[string][]map[string]string{
	"set-input": {
		{"val0": "Date", "val1": "Long", "val2": "Calendar",
			"val3": "LocalDateTime", "val4": "ZonedDateTime"},
	},
	"set-fields": {
		{"val0": "Date", "val1": "Date", "val2": "Date", "val3": "Date",
			"val4": "Date", "val5": "Date", "val6": "Date", "val7": "Date"},
	},
	"nested": {
		{"val0": "Date", "val1": "Long", "val2": "Calendar",
			"val3": "LocalDateTime", "val4": "ZonedDateTime"},
		{"val0": "Calendar", "val1": "Calendar", "val2": "Calendar",
			"val3": "Calendar", "val4": "Calendar"},
	},
}

type exprDTSetNested550CaseState struct {
	caseName    string
	env         *esper.Environment
	engine      *esper.Engine
	trace       *compat.Trace
	sequence    uint64
	deployIndex int
	typesIndex  int
	deployment  *esper.Deployment
	plan        esper.Plan
	fired       bool
	row         compat.ResultRecord
}

func runExprDTSetNested550Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range exprDTSetNested550Cases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runExprDTSetNested550Case(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprDTSetNested550ID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprDTSetNested550ID, scenario.ID)
	}
	return trace, nil
}

func runExprDTSetNested550Case(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterMap(env, "SupportDateTime", exprDTSetNested550FieldSpecs()); err != nil {
		return compat.Trace{}, fmt.Errorf("%s register SupportDateTime: %w", exprDTSetNested550ID, err)
	}
	state := &exprDTSetNested550CaseState{
		caseName: caseName,
		env:      env,
		engine: esper.NewEngine(env,
			esper.WithStartTime(time.Unix(0, 0).UTC()),
			esper.WithRuntimeURI(exprDTSetNested550CaseRuntimeIDs[caseName])),
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
			return *state.trace, fmt.Errorf("%s: unsupported step op %q", exprDTSetNested550ID, step.Op)
		}
	}
	return *state.trace, nil
}

// exprDTSetNested550FieldSpecs mirrors SupportDateTime: longdate is the
// epoch-millis Long and the other four representations collapse to
// time.Time.
func exprDTSetNested550FieldSpecs() []esper.FieldSpec {
	return []esper.FieldSpec{
		esper.FieldDef("longdate", reflect.TypeOf(int64(0))),
		esper.FieldDef("utildate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("caldate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("localdate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("zoneddate", reflect.TypeOf(time.Time{})),
	}
}

// deploy mirrors one compileDeploy + addListener('s0') cycle; the pinned
// EPL for this deploy index is verified before the fluent equivalent is
// built. The nested case deploys twice: the plain chains, then (mirroring
// compileDeployAddListenerMile) the .toCalendar() milestone.
func (s *exprDTSetNested550CaseState) deploy(ctx context.Context, step compat.Step) error {
	pinned, ok := exprDTSetNested550DeployEPLs[s.caseName]
	if !ok || s.deployIndex >= len(pinned) || step.Statement != "s0" || step.Epl != pinned[s.deployIndex] {
		return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q",
			exprDTSetNested550ID, s.caseName, step.Statement, step.Epl)
	}
	if err := s.undeployAll(ctx); err != nil {
		return err
	}
	query, err := s.buildSelect()
	if err != nil {
		return err
	}
	s.deployIndex++
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
			exprDTSetNested550RenderMillis(newRows)
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

// buildSelect renders the fluent equivalent of the case's pinned s0
// select. utildate/longdate/caldate/localdate/zoneddate collapse to the
// int64 and time.Time reps; set() preserves the input representation and
// DateTimeToTime (the .toCalendar() milestone) collapses to time.Time.
// Go's DateTimeSet month is 1-based, so set-input passes 1 uniformly and
// set-fields passes 7 where Java passes the 0-based literal 6.
func (s *exprDTSetNested550CaseState) buildSelect() (esper.Query, error) {
	longdate := esper.Field[map[string]any, int64]("longdate")
	utildate := esper.Field[map[string]any, time.Time]("utildate")
	caldate := esper.Field[map[string]any, time.Time]("caldate")
	localdate := esper.Field[map[string]any, time.Time]("localdate")
	zoneddate := esper.Field[map[string]any, time.Time]("zoneddate")
	switch s.caseName {
	case "set-input":
		return esper.FromAny(s.env, "SupportDateTime").Select(
			esper.Alias("val0", esper.DateTimeSet[time.Time](utildate, "month", 1)),
			esper.Alias("val1", esper.DateTimeSet[int64](longdate, "month", 1)),
			esper.Alias("val2", esper.DateTimeSet[time.Time](caldate, "month", 1)),
			esper.Alias("val3", esper.DateTimeSet[time.Time](localdate, "month", 1)),
			esper.Alias("val4", esper.DateTimeSet[time.Time](zoneddate, "month", 1)),
		).Query(esper.StatementName("s0")), nil
	case "set-fields":
		selections := make([]esper.Selection, 0, len(exprDTSetNested550Units))
		for index, unit := range exprDTSetNested550Units {
			selections = append(selections, esper.Alias(fmt.Sprintf("val%d", index),
				esper.DateTimeSet[time.Time](utildate, unit, exprDTSetNested550SetFieldArgs[index])))
		}
		return esper.FromAny(s.env, "SupportDateTime").Select(selections...).
			Query(esper.StatementName("s0")), nil
	case "nested":
		chain := func(value esper.Expression[time.Time]) esper.Expression[time.Time] {
			return esper.DateTimeSet[time.Time](
				esper.DateTimeSet[time.Time](
					esper.DateTimeSet[time.Time](value, "hour", 1), "minute", 2), "second", 3)
		}
		chainLong := func(value esper.Expression[int64]) esper.Expression[int64] {
			return esper.DateTimeSet[int64](
				esper.DateTimeSet[int64](
					esper.DateTimeSet[int64](value, "hour", 1), "minute", 2), "second", 3)
		}
		if s.deployIndex == 0 {
			return esper.FromAny(s.env, "SupportDateTime").Select(
				esper.Alias("val0", chain(utildate)),
				esper.Alias("val1", chainLong(longdate)),
				esper.Alias("val2", chain(caldate)),
				esper.Alias("val3", chain(localdate)),
				esper.Alias("val4", chain(zoneddate)),
			).Query(esper.StatementName("s0")), nil
		}
		return esper.FromAny(s.env, "SupportDateTime").Select(
			esper.Alias("val0", esper.DateTimeToTime[time.Time](chain(utildate))),
			esper.Alias("val1", esper.DateTimeToTime[int64](chainLong(longdate))),
			esper.Alias("val2", esper.DateTimeToTime[time.Time](chain(caldate))),
			esper.Alias("val3", esper.DateTimeToTime[time.Time](chain(localdate))),
			esper.Alias("val4", esper.DateTimeToTime[time.Time](chain(zoneddate))),
		).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("%s: unsupported case %q", exprDTSetNested550ID, s.caseName)
	}
}

// types emits the pinned property-type record for this types index after
// verifying the deployed statement's collapsed Go schema: Long columns are
// int64-typed and every other column is time.Time-typed, matching the
// Java-asserted LONGBOXED vs date-time types (Date and Calendar both
// collapse to time.Time).
func (s *exprDTSetNested550CaseState) types(step compat.Step) error {
	pins, ok := exprDTSetNested550TypeProperties[s.caseName]
	if !ok || s.typesIndex >= len(pins) || step.Statement != "s0" {
		return fmt.Errorf("%s: case %q has no types assertion for %q",
			exprDTSetNested550ID, s.caseName, step.Statement)
	}
	properties := pins[s.typesIndex]
	s.typesIndex++
	schema, ok := s.plan.ResultSchema()
	if !ok {
		return fmt.Errorf("%s: statement %q has no result schema", exprDTSetNested550ID, step.Statement)
	}
	columns, err := exprDTSetNested550Columns(s.caseName)
	if err != nil {
		return err
	}
	value := map[string]any{}
	for _, column := range columns {
		field, exists := schema.Field(column)
		if !exists {
			return fmt.Errorf("%s: s0 is missing column %q", exprDTSetNested550ID, column)
		}
		want := reflect.TypeOf(time.Time{})
		if properties[column] == "Long" {
			want = reflect.TypeOf(int64(0))
		}
		if field.Type != want {
			return fmt.Errorf("%s: s0 %s type drift: %v", exprDTSetNested550ID, column, field.Type)
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
func (s *exprDTSetNested550CaseState) send(ctx context.Context, step compat.Step) error {
	if step.EventType != "SupportDateTime" {
		return fmt.Errorf("%s: unsupported event type %q", exprDTSetNested550ID, step.EventType)
	}
	var payload struct {
		Date     string  `json:"date"`
		Expected []int64 `json:"expected"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return fmt.Errorf("%s: decode SupportDateTime: %w", exprDTSetNested550ID, err)
	}
	if payload.Date != exprDTSetNested550SendDates[s.caseName] {
		return fmt.Errorf("%s: case %q send date %q is not pinned",
			exprDTSetNested550ID, s.caseName, payload.Date)
	}
	parsed, err := time.Parse(time.RFC3339Nano, payload.Date)
	if err != nil {
		return fmt.Errorf("%s: parse SupportDateTime date: %w", exprDTSetNested550ID, err)
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
		return fmt.Errorf("%s: SupportDateTime send produced no listener row", exprDTSetNested550ID)
	}
	return s.verifyExpected(payload.Expected)
}

// verifyExpected compares the delivered row's fields against the pinned
// expected epoch-millis values in select order, mirroring assertPropsNew.
func (s *exprDTSetNested550CaseState) verifyExpected(expected []int64) error {
	columns, err := exprDTSetNested550Columns(s.caseName)
	if err != nil {
		return err
	}
	if len(expected) != len(columns) {
		return fmt.Errorf("%s: %s send pins %d expected values, want %d",
			exprDTSetNested550ID, s.caseName, len(expected), len(columns))
	}
	for index, column := range columns {
		actual, ok := s.row.Fields[column]
		if !ok {
			return fmt.Errorf("%s: listener row is missing column %q", exprDTSetNested550ID, column)
		}
		got, ok := actual.(int64)
		if !ok || got != expected[index] {
			return fmt.Errorf("%s: column %q = %v, want %v", exprDTSetNested550ID, column, actual, expected[index])
		}
	}
	return nil
}

// exprDTSetNested550Columns returns the select-order column names a send's
// expected values pin: val0..val7 for set-fields, val0..val4 for the
// input/nested cases.
func exprDTSetNested550Columns(caseName string) ([]string, error) {
	count := 0
	switch caseName {
	case "set-input", "nested":
		count = 5
	case "set-fields":
		count = 8
	default:
		return nil, fmt.Errorf("%s: unsupported case %q", exprDTSetNested550ID, caseName)
	}
	columns := make([]string, 0, count)
	for index := range count {
		columns = append(columns, fmt.Sprintf("val%d", index))
	}
	return columns, nil
}

// exprDTSetNested550RenderMillis renders transformed time.Time cells as
// epoch millis, the oracle's instant-token convention for every
// SupportDateTime representation (Date, Calendar, LDT and ZDT all collapse
// to the same instant token).
func exprDTSetNested550RenderMillis(rows []compat.ResultRecord) {
	for _, row := range rows {
		for name, value := range row.Fields {
			if located, ok := value.(time.Time); ok {
				row.Fields[name] = located.UnixMilli()
			}
		}
	}
}

// undeployAll tears down the live deployment, mirroring the execution's
// env.undeployAll() between milestones and at case end.
func (s *exprDTSetNested550CaseState) undeployAll(ctx context.Context) error {
	if s.deployment != nil {
		if err := s.deployment.Undeploy(ctx); err != nil {
			return err
		}
		s.deployment = nil
	}
	return nil
}

// exprDTSetNested550CaseSteps pins the complete step sequence per case as
// op|case|statement|eventType|epl|payload|expectError|at keys so the loader
// asserts the scenario file matches the contract byte-for-byte. The nested
// case runs the deploy->types->send->undeploy-all cycle twice.
var exprDTSetNested550CaseSteps = func() map[string][]string {
	steps := map[string][]string{}
	for _, caseName := range exprDTSetNested550Cases {
		expected, err := json.Marshal(exprDTSetNested550ExpectedMillis[caseName])
		if err != nil {
			panic(err)
		}
		payload := fmt.Sprintf(`{"date":%q,"expected":%s}`, exprDTSetNested550SendDates[caseName], expected)
		pinned := []string{}
		for _, epl := range exprDTSetNested550DeployEPLs[caseName] {
			pinned = append(pinned,
				"deploy|"+caseName+"|s0||"+epl+"|||",
				"types|"+caseName+"|s0|||||",
				"send|"+caseName+"||SupportDateTime||"+payload+"||",
				"undeploy-all|"+caseName+"||||||")
		}
		steps[caseName] = pinned
	}
	return steps
}()

// loadExprDTSetNested550Scenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned metadata and case metadata, and the complete
// pinned step sequence so unknown, duplicated or drifted content fails the
// replay.
func loadExprDTSetNested550Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprDTSetNested550ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprDTSetNested550ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTSetNested550ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTSetNested550ID, err)
	}
	if err := requireExprDTSetNested550Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprDTSetNested550ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprDTSetNested550ID ||
		metadata.Description != exprDTSetNested550Description ||
		metadata.JavaCommit != exprDTSetNested550JavaCommit ||
		metadata.JavaSource != exprDTSetNested550Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprDTSetNested550ID)
	}
	if err := validateExprDTSetNested550StringArray(root["javaRuntimes"], exprDTSetNested550JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTSetNested550StringArray(root["javaNames"], exprDTSetNested550JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTSetNested550StringArray(root["javaStaticIds"], exprDTSetNested550JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTSetNested550StringArray(root["javaFlags"], exprDTSetNested550JavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprDTSetNested550Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", exprDTSetNested550ID, len(exprDTSetNested550Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprDTSetNested550Fields(object,
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
		if definition.Case != exprDTSetNested550Cases[index] ||
			definition.Ordinal != exprDTSetNested550Ordinals[index] ||
			definition.RuntimeID != exprDTSetNested550JavaRuntimeIDs[index] ||
			definition.ExecutionName != exprDTSetNested550JavaExecutions[index] ||
			definition.Observation != exprDTSetNested550CaseObservations[index] ||
			definition.EPL != exprDTSetNested550CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprDTSetNested550ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", exprDTSetNested550ID, err)
	}
	offset := 0
	for _, caseName := range exprDTSetNested550Cases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", exprDTSetNested550ID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTSetNested550ID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", exprDTSetNested550ID, offset, caseName)
		}
		if _, err := exprDTSetNested550StepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTSetNested550ID, offset, err)
		}
		offset++
		want, ok := exprDTSetNested550CaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", exprDTSetNested550ID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", exprDTSetNested550ID, caseName)
		}
		for _, pinned := range want {
			key, err := exprDTSetNested550StepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTSetNested550ID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", exprDTSetNested550ID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", exprDTSetNested550ID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTSetNested550ID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// exprDTSetNested550StepKey renders one raw step as its pinned key:
// op|case|statement|eventType|epl|payload|expectError|at with the payload
// compacted. Unknown fields on the step object are rejected per op.
func exprDTSetNested550StepKey(raw json.RawMessage) (string, error) {
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

func requireExprDTSetNested550Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", exprDTSetNested550ID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", exprDTSetNested550ID, name)
		}
	}
	return nil
}

func validateExprDTSetNested550StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
