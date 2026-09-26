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

// expr_dt_format_551.go replays the two ExprDTFormat executions (Draft
// 4.551) against the pinned Java oracle. Each case runs on a fresh
// runtime: advance the engine clock to 2002-05-30T09:00:00.000 (Java's
// env.advanceTime before compileDeploy), deploy the byte-exact s0 select,
// pin the assertStmtTypesAllSame property-type surface through a types
// record, send a populated SupportDateTime bean built exactly like
// SupportDateTime.make under the system default (pinned UTC) timezone,
// send the all-null make(null) bean, verify each delivered row against
// the pinned expected strings like assertPropsNew, then undeployAll:
//
//   - format-simple (ord 0, ExprDTFormatSimple): no-arg format() renders
//     current_timestamp and the Date/long/Calendar representations
//     through a new SimpleDateFormat() — the en_US-pinned
//     "5/30/02, 9:00 AM" — localdate through ISO_DATE_TIME
//     ("2002-05-30T09:00:00") and zoneddate through ISO_ZONED_DATE_TIME
//     ("2002-05-30T09:00:00Z[UTC]"). The all-null send keeps
//     current_timestamp.format() non-null while val1..val5 go null.
//   - format-wstring (ord 1, ExprDTFormatWString): the pattern
//     "yyyy.MM.dd G 'at' HH:mm:ss" renders "2002.05.30 AD at 09:00:00"
//     across all five representations, the getDateInstance() object
//     overload renders "May 30, 2002", and BASIC_ISO_DATE renders
//     "20020530". The all-null send emits null in every column.
//
// Go maps val0..val3 of format-simple to DateTimeFormatDefault (the
// pinned en_US SimpleDateFormat default), val4 to DateTimeFormatISO and
// val5 to DateTimeFormatISOZoned; format-wstring maps the pattern
// literals to DateTimeFormatPattern (the 'G' era and 'at' quoting
// included), the getDateInstance() overload to
// DateTimeFormatDateInstance and BASIC_ISO_DATE to the equivalent
// "yyyyMMdd" pattern. DATE/LONGBOXED/CALENDAR/LOCALDATETIME/ZONEDDATETIME
// collapse to int64/time.Time reps.

const exprDTFormat551ID = "expr-dt-format-551"
const exprDTFormat551JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

// exprDTFormat551Source is the scenario javaSource pin: the single
// ExprDTFormat source file (the expr-dt-data-sources precedent).
const exprDTFormat551Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTFormat.java"

var exprDTFormat551JavaSources = []string{exprDTFormat551Source}

var exprDTFormat551JavaRuntimeIDs = []string{
	"java-runtime-2832950bf0c454ecf2df",
	"java-runtime-8a27995fa8e41684f6b8",
}

var exprDTFormat551JavaExecutions = []string{
	"ExprDTFormatSimple",
	"ExprDTFormatWString",
}

var exprDTFormat551Cases = []string{
	"format-simple",
	"format-wstring",
}

var exprDTFormat551CaseRuntimeIDs = map[string]string{
	"format-simple":  "java-runtime-2832950bf0c454ecf2df",
	"format-wstring": "java-runtime-8a27995fa8e41684f6b8",
}

var exprDTFormat551JavaStaticIDs = []string{
	"java-05592d8076a91dd5cbed",
	"java-05592d8076a91dd5cbed",
}

var exprDTFormat551JavaFlags = []string{}

var exprDTFormat551Ordinals = []int{0, 1}

const exprDTFormat551Description = "ExprDTFormat executions: format-simple replays " +
	"ExprDTFormatSimple (no-arg format() over current_timestamp and all five " +
	"SupportDateTime representations of 2002-05-30T09:00:00.000: the en_US " +
	"SimpleDateFormat default '5/30/02, 9:00 AM' for the legacy representations, " +
	"ISO_DATE_TIME '2002-05-30T09:00:00' and ISO_ZONED_DATE_TIME " +
	"'2002-05-30T09:00:00Z[UTC]' for the Java 8 representations; the all-null " +
	"send keeps val0 non-null), format-wstring replays ExprDTFormatWString " +
	"(the pattern \"yyyy.MM.dd G 'at' HH:mm:ss\" renders " +
	"'2002.05.30 AD at 09:00:00' across all five representations, " +
	"SimpleDateFormat.getDateInstance() renders 'May 30, 2002', " +
	"DateTimeFormatter.BASIC_ISO_DATE renders '20020530'; the all-null send " +
	"emits null in every column)."

var exprDTFormat551CaseObservations = []string{
	"listener x2+types; advanceTime(2002-05-30T09:00:00.000), deploy s0 no-arg " +
		"format() over current_timestamp and all five representations, types pins " +
		"all-String, send make(2002-05-30T09:00:00.000) emits {5/30/02, 9:00 AM " +
		"x4, 2002-05-30T09:00:00, 2002-05-30T09:00:00Z[UTC]}, send make(null) " +
		"emits {5/30/02, 9:00 AM, null x5}",
	"listener x2+types; advanceTime(2002-05-30T09:00:00.000), deploy s0 " +
		"pattern/formatter-object format() over all five representations, types " +
		"pins all-String, send make(2002-05-30T09:00:00.000) emits " +
		"{2002.05.30 AD at 09:00:00 x5, May 30, 2002, 20020530}, send " +
		"make(null) emits {null x7}",
}

// exprDTFormat551SimpleEPL is the byte-exact ExprDTFormatSimple ord-0
// statement: no-arg format() over current_timestamp and all five
// SupportDateTime representations.
const exprDTFormat551SimpleEPL = "@name('s0') select " +
	"current_timestamp.format() as val0," +
	"utildate.format() as val1," +
	"longdate.format() as val2," +
	"caldate.format() as val3," +
	"localdate.format() as val4," +
	"zoneddate.format() as val5" +
	" from SupportDateTime"

// exprDTFormat551WStringEPL is the byte-exact ExprDTFormatWString ord-1
// statement: the double-quoted "yyyy.MM.dd G 'at' HH:mm:ss" pattern over
// all five representations plus the two formatter-object overloads
// (SimpleDateFormat.getDateInstance() and BASIC_ISO_DATE).
const exprDTFormat551WStringEPL = "@name('s0') select " +
	"longdate.format(\"yyyy.MM.dd G 'at' HH:mm:ss\") as val0," +
	"utildate.format(\"yyyy.MM.dd G 'at' HH:mm:ss\") as val1," +
	"caldate.format(\"yyyy.MM.dd G 'at' HH:mm:ss\") as val2," +
	"localdate.format(\"yyyy.MM.dd G 'at' HH:mm:ss\") as val3," +
	"zoneddate.format(\"yyyy.MM.dd G 'at' HH:mm:ss\") as val4," +
	"utildate.format(SimpleDateFormat.getDateInstance()) as val5," +
	"localdate.format(java.time.format.DateTimeFormatter.BASIC_ISO_DATE) as val6" +
	" from SupportDateTime"

// exprDTFormat551DeployEPLs pins each case's single s0 deploy.
var exprDTFormat551DeployEPLs = map[string]string{
	"format-simple":  exprDTFormat551SimpleEPL,
	"format-wstring": exprDTFormat551WStringEPL,
}

// exprDTFormat551CaseEPLs pins each case's representative EPL for the
// case metadata.
var exprDTFormat551CaseEPLs = []string{
	exprDTFormat551SimpleEPL,
	exprDTFormat551WStringEPL,
}

// exprDTFormat551T00 is DateTime.parseDefaultMSec("2002-05-30T09:00:00.000"),
// every case's advance-time and populated-send instant.
const exprDTFormat551T00 = "2002-05-30T09:00:00.000Z"

// exprDTFormat551WStringPattern is the constant Java
// SimpleDateFormat/DateTimeFormatter pattern every format-wstring
// pattern column renders through.
const exprDTFormat551WStringPattern = "yyyy.MM.dd G 'at' HH:mm:ss"

// exprDTFormat551Expected pins every send's assertPropsNew values as
// rendered strings; a null cell is the tagged {"state":"null"} object the
// trace protocol renders on both sides:
//
//   - format-simple send 1: the en_US SimpleDateFormat default for
//     current_timestamp and the three legacy reps, ISO_DATE_TIME for
//     localdate, ISO_ZONED_DATE_TIME for zoneddate; send 2 (all-null
//     bean) keeps current_timestamp.format() non-null.
//   - format-wstring send 1: the 'G'-era pattern across all five reps,
//     getDateInstance()'s medium date, BASIC_ISO_DATE; send 2 is
//     all-null.
var exprDTFormat551Expected = map[string][][]any{
	"format-simple": {
		{"5/30/02, 9:00 AM", "5/30/02, 9:00 AM", "5/30/02, 9:00 AM",
			"5/30/02, 9:00 AM", "2002-05-30T09:00:00", "2002-05-30T09:00:00Z[UTC]"},
		{"5/30/02, 9:00 AM",
			map[string]any{"state": "null"}, map[string]any{"state": "null"},
			map[string]any{"state": "null"}, map[string]any{"state": "null"},
			map[string]any{"state": "null"}},
	},
	"format-wstring": {
		{"2002.05.30 AD at 09:00:00", "2002.05.30 AD at 09:00:00",
			"2002.05.30 AD at 09:00:00", "2002.05.30 AD at 09:00:00",
			"2002.05.30 AD at 09:00:00", "May 30, 2002", "20020530"},
		{map[string]any{"state": "null"}, map[string]any{"state": "null"},
			map[string]any{"state": "null"}, map[string]any{"state": "null"},
			map[string]any{"state": "null"}, map[string]any{"state": "null"},
			map[string]any{"state": "null"}},
	},
}

// exprDTFormat551SendDates pins each case's populated-send instant; the
// all-null send carries JSON null.
var exprDTFormat551SendDates = map[string]string{
	"format-simple":  exprDTFormat551T00,
	"format-wstring": exprDTFormat551T00,
}

// exprDTFormat551TypeProperties pins the Java-asserted property types
// per case: assertStmtTypesAllSame asserts every column is String in
// both executions.
var exprDTFormat551TypeProperties = map[string]map[string]string{
	"format-simple": {
		"val0": "String", "val1": "String", "val2": "String",
		"val3": "String", "val4": "String", "val5": "String",
	},
	"format-wstring": {
		"val0": "String", "val1": "String", "val2": "String",
		"val3": "String", "val4": "String", "val5": "String",
		"val6": "String",
	},
}

type exprDTFormat551CaseState struct {
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

func runExprDTFormat551Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range exprDTFormat551Cases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runExprDTFormat551Case(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprDTFormat551ID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprDTFormat551ID, scenario.ID)
	}
	return trace, nil
}

func runExprDTFormat551Case(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterMap(env, "SupportDateTime", exprDTFormat551FieldSpecs()); err != nil {
		return compat.Trace{}, fmt.Errorf("%s register SupportDateTime: %w", exprDTFormat551ID, err)
	}
	state := &exprDTFormat551CaseState{
		caseName: caseName,
		env:      env,
		engine: esper.NewEngine(env,
			esper.WithStartTime(time.Unix(0, 0).UTC()),
			esper.WithRuntimeURI(exprDTFormat551CaseRuntimeIDs[caseName])),
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
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return *state.trace, fmt.Errorf("%s: parse advance-time %q: %w", exprDTFormat551ID, step.At, err)
			}
			if step.At != exprDTFormat551T00 {
				return *state.trace, fmt.Errorf("%s: case %q advance-time %q is not pinned",
					exprDTFormat551ID, caseName, step.At)
			}
			if err := state.engine.AdvanceTime(ctx, at); err != nil {
				return *state.trace, err
			}
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
			return *state.trace, fmt.Errorf("%s: unsupported step op %q", exprDTFormat551ID, step.Op)
		}
	}
	return *state.trace, nil
}

// exprDTFormat551FieldSpecs mirrors SupportDateTime: longdate is the
// epoch-millis Long and the other four representations collapse to
// time.Time.
func exprDTFormat551FieldSpecs() []esper.FieldSpec {
	return []esper.FieldSpec{
		esper.FieldDef("longdate", reflect.TypeOf(int64(0))),
		esper.FieldDef("utildate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("caldate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("localdate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("zoneddate", reflect.TypeOf(time.Time{})),
	}
}

// deploy mirrors the compileDeploy + addListener('s0') cycle; the pinned
// EPL is verified before the fluent equivalent is built.
func (s *exprDTFormat551CaseState) deploy(ctx context.Context, step compat.Step) error {
	pinned, ok := exprDTFormat551DeployEPLs[s.caseName]
	if !ok || step.Statement != "s0" || step.Epl != pinned {
		return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q",
			exprDTFormat551ID, s.caseName, step.Statement, step.Epl)
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
// select. utildate/caldate collapse to the time.Time rep Java's legacy
// family shares with longdate (int64); format-simple val0..val3 use the
// pinned en_US SimpleDateFormat default and val4/val5 use the Java 8
// ISO formatters. format-wstring val0..val4 use the constant 'G'-era
// pattern, val5 models the getDateInstance() object overload, and val6
// models BASIC_ISO_DATE through the equivalent "yyyyMMdd" pattern.
func (s *exprDTFormat551CaseState) buildSelect() (esper.Query, error) {
	longdate := esper.Field[map[string]any, int64]("longdate")
	utildate := esper.Field[map[string]any, time.Time]("utildate")
	caldate := esper.Field[map[string]any, time.Time]("caldate")
	localdate := esper.Field[map[string]any, time.Time]("localdate")
	zoneddate := esper.Field[map[string]any, time.Time]("zoneddate")
	switch s.caseName {
	case "format-simple":
		return esper.FromAny(s.env, "SupportDateTime").Select(
			esper.Alias("val0", esper.DateTimeFormatDefault[int64](esper.CurrentTimestamp())),
			esper.Alias("val1", esper.DateTimeFormatDefault[time.Time](utildate)),
			esper.Alias("val2", esper.DateTimeFormatDefault[int64](longdate)),
			esper.Alias("val3", esper.DateTimeFormatDefault[time.Time](caldate)),
			esper.Alias("val4", esper.DateTimeFormatISO[time.Time](localdate)),
			esper.Alias("val5", esper.DateTimeFormatISOZoned[time.Time](zoneddate)),
		).Query(esper.StatementName("s0")), nil
	case "format-wstring":
		return esper.FromAny(s.env, "SupportDateTime").Select(
			esper.Alias("val0", esper.DateTimeFormatPattern[int64](longdate, exprDTFormat551WStringPattern)),
			esper.Alias("val1", esper.DateTimeFormatPattern[time.Time](utildate, exprDTFormat551WStringPattern)),
			esper.Alias("val2", esper.DateTimeFormatPattern[time.Time](caldate, exprDTFormat551WStringPattern)),
			esper.Alias("val3", esper.DateTimeFormatPattern[time.Time](localdate, exprDTFormat551WStringPattern)),
			esper.Alias("val4", esper.DateTimeFormatPattern[time.Time](zoneddate, exprDTFormat551WStringPattern)),
			esper.Alias("val5", esper.DateTimeFormatDateInstance[time.Time](utildate)),
			esper.Alias("val6", esper.DateTimeFormatPattern[time.Time](localdate, "yyyyMMdd")),
		).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("%s: unsupported case %q", exprDTFormat551ID, s.caseName)
	}
}

// types emits the pinned property-type record after verifying the
// deployed statement's Go schema reports every column as string-typed,
// matching the Java assertStmtTypesAllSame(STRING) assertion.
func (s *exprDTFormat551CaseState) types(step compat.Step) error {
	properties, ok := exprDTFormat551TypeProperties[s.caseName]
	if !ok || step.Statement != "s0" {
		return fmt.Errorf("%s: case %q has no types assertion for %q",
			exprDTFormat551ID, s.caseName, step.Statement)
	}
	schema, ok := s.plan.ResultSchema()
	if !ok {
		return fmt.Errorf("%s: statement %q has no result schema", exprDTFormat551ID, step.Statement)
	}
	columns, err := exprDTFormat551Columns(s.caseName)
	if err != nil {
		return err
	}
	value := map[string]any{}
	for _, column := range columns {
		field, exists := schema.Field(column)
		if !exists {
			return fmt.Errorf("%s: s0 is missing column %q", exprDTFormat551ID, column)
		}
		if field.Type != reflect.TypeOf("") {
			return fmt.Errorf("%s: s0 %s type drift: %v", exprDTFormat551ID, column, field.Type)
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
// MILLISECOND to zero) — or the all-null bean when the payload date is
// null — delivers the map event, and verifies the delivered row against
// the pinned expected rendered strings: the assertPropsNew equivalent.
func (s *exprDTFormat551CaseState) send(ctx context.Context, step compat.Step) error {
	if step.EventType != "SupportDateTime" {
		return fmt.Errorf("%s: unsupported event type %q", exprDTFormat551ID, step.EventType)
	}
	var payload struct {
		Date     *string `json:"date"`
		Expected []any   `json:"expected"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return fmt.Errorf("%s: decode SupportDateTime: %w", exprDTFormat551ID, err)
	}
	event := map[string]any{
		"longdate":  nil,
		"utildate":  nil,
		"caldate":   nil,
		"localdate": nil,
		"zoneddate": nil,
	}
	if payload.Date != nil {
		if *payload.Date != exprDTFormat551SendDates[s.caseName] {
			return fmt.Errorf("%s: case %q send date %q is not pinned",
				exprDTFormat551ID, s.caseName, *payload.Date)
		}
		parsed, err := time.Parse(time.RFC3339Nano, *payload.Date)
		if err != nil {
			return fmt.Errorf("%s: parse SupportDateTime date: %w", exprDTFormat551ID, err)
		}
		event["longdate"] = parsed.UnixMilli()
		event["utildate"] = parsed
		event["caldate"] = parsed.Add(-time.Duration(parsed.Nanosecond()))
		event["localdate"] = parsed
		event["zoneddate"] = parsed
	}
	s.fired = false
	s.row = compat.ResultRecord{}
	if err := s.engine.SendRecord(ctx, "SupportDateTime", event); err != nil {
		return err
	}
	if !s.fired {
		return fmt.Errorf("%s: SupportDateTime send produced no listener row", exprDTFormat551ID)
	}
	return s.verifyExpected(payload.Expected)
}

// verifyExpected compares the delivered row's fields against the pinned
// expected rendered strings in select order, mirroring assertPropsNew;
// null cells compare through the tagged {"state":"null"} token.
func (s *exprDTFormat551CaseState) verifyExpected(expected []any) error {
	columns, err := exprDTFormat551Columns(s.caseName)
	if err != nil {
		return err
	}
	if len(expected) != len(columns) {
		return fmt.Errorf("%s: %s send pins %d expected values, want %d",
			exprDTFormat551ID, s.caseName, len(expected), len(columns))
	}
	for index, column := range columns {
		actual, ok := s.row.Fields[column]
		if !ok {
			return fmt.Errorf("%s: listener row is missing column %q", exprDTFormat551ID, column)
		}
		if !exprDTFormat551ValueEqual(expected[index], actual) {
			return fmt.Errorf("%s: column %q = %v, want %v", exprDTFormat551ID, column, actual, expected[index])
		}
	}
	return nil
}

// exprDTFormat551ValueEqual compares a JSON-decoded expected cell with
// the normalized row value: strings compare directly and the
// {"state":"null"} token matches the tagged null cell both sides render.
func exprDTFormat551ValueEqual(expected, actual any) bool {
	if want, ok := expected.(map[string]any); ok {
		if want["state"] == "null" {
			got, isMap := actual.(map[string]any)
			return isMap && got["state"] == "null"
		}
		return false
	}
	want, ok := expected.(string)
	if !ok {
		return false
	}
	got, ok := actual.(string)
	return ok && want == got
}

// exprDTFormat551Columns returns the select-order column names a send's
// expected values pin: val0..val5 for format-simple, val0..val6 for
// format-wstring.
func exprDTFormat551Columns(caseName string) ([]string, error) {
	count := 0
	switch caseName {
	case "format-simple":
		count = 6
	case "format-wstring":
		count = 7
	default:
		return nil, fmt.Errorf("%s: unsupported case %q", exprDTFormat551ID, caseName)
	}
	columns := make([]string, 0, count)
	for index := range count {
		columns = append(columns, fmt.Sprintf("val%d", index))
	}
	return columns, nil
}

// undeployAll tears down the live deployment, mirroring the execution's
// env.undeployAll() at case end.
func (s *exprDTFormat551CaseState) undeployAll(ctx context.Context) error {
	if s.deployment != nil {
		if err := s.deployment.Undeploy(ctx); err != nil {
			return err
		}
		s.deployment = nil
	}
	return nil
}

// exprDTFormat551CaseSteps pins the complete step sequence per case as
// op|case|statement|eventType|epl|payload|expectError|at keys so the
// loader asserts the scenario file matches the contract byte-for-byte.
// Each case advances to the pinned instant, deploys, pins the types
// surface, then sends the populated and all-null beans.
var exprDTFormat551CaseSteps = func() map[string][]string {
	steps := map[string][]string{}
	for _, caseName := range exprDTFormat551Cases {
		rows := exprDTFormat551Expected[caseName]
		pinned := []string{
			"advance-time|" + caseName + "||||||" + exprDTFormat551T00,
			"deploy|" + caseName + "|s0||" + exprDTFormat551DeployEPLs[caseName] + "|||",
			"types|" + caseName + "|s0|||||",
		}
		for index, row := range rows {
			expected, err := json.Marshal(row)
			if err != nil {
				panic(err)
			}
			nullSend := index == len(rows)-1
			payload := fmt.Sprintf(`{"date":%s,"expected":%s}`,
				map[bool]string{false: fmt.Sprintf("%q", exprDTFormat551SendDates[caseName]), true: "null"}[nullSend],
				expected)
			pinned = append(pinned,
				"send|"+caseName+"||SupportDateTime||"+payload+"||")
		}
		pinned = append(pinned, "undeploy-all|"+caseName+"||||||")
		steps[caseName] = pinned
	}
	return steps
}()

// loadExprDTFormat551Scenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned metadata and case metadata, and the
// complete pinned step sequence so unknown, duplicated or drifted
// content fails the replay.
func loadExprDTFormat551Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprDTFormat551ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprDTFormat551ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTFormat551ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTFormat551ID, err)
	}
	if err := requireExprDTFormat551Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprDTFormat551ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprDTFormat551ID ||
		metadata.Description != exprDTFormat551Description ||
		metadata.JavaCommit != exprDTFormat551JavaCommit ||
		metadata.JavaSource != exprDTFormat551Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprDTFormat551ID)
	}
	if err := validateExprDTFormat551StringArray(root["javaRuntimes"], exprDTFormat551JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTFormat551StringArray(root["javaNames"], exprDTFormat551JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTFormat551StringArray(root["javaStaticIds"], exprDTFormat551JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTFormat551StringArray(root["javaFlags"], exprDTFormat551JavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprDTFormat551Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", exprDTFormat551ID, len(exprDTFormat551Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprDTFormat551Fields(object,
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
		if definition.Case != exprDTFormat551Cases[index] ||
			definition.Ordinal != exprDTFormat551Ordinals[index] ||
			definition.RuntimeID != exprDTFormat551JavaRuntimeIDs[index] ||
			definition.ExecutionName != exprDTFormat551JavaExecutions[index] ||
			definition.Observation != exprDTFormat551CaseObservations[index] ||
			definition.EPL != exprDTFormat551CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprDTFormat551ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", exprDTFormat551ID, err)
	}
	offset := 0
	for _, caseName := range exprDTFormat551Cases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", exprDTFormat551ID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTFormat551ID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", exprDTFormat551ID, offset, caseName)
		}
		if _, err := exprDTFormat551StepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTFormat551ID, offset, err)
		}
		offset++
		want, ok := exprDTFormat551CaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", exprDTFormat551ID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", exprDTFormat551ID, caseName)
		}
		for _, pinned := range want {
			key, err := exprDTFormat551StepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTFormat551ID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", exprDTFormat551ID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", exprDTFormat551ID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTFormat551ID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// exprDTFormat551StepKey renders one raw step as its pinned key:
// op|case|statement|eventType|epl|payload|expectError|at with the payload
// compacted. Unknown fields on the step object are rejected per op.
func exprDTFormat551StepKey(raw json.RawMessage) (string, error) {
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

func requireExprDTFormat551Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", exprDTFormat551ID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", exprDTFormat551ID, name)
		}
	}
	return nil
}

func validateExprDTFormat551StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
