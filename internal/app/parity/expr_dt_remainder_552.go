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

// expr_dt_remainder_552.go replays the six datetime-remainder executions
// (Draft 4.552) against the pinned Java oracle: ExprDTGet ord 0/1,
// ExprDTPlusMinus ord 0/1, ExprDTWithDate and ExprDTWithTime. Every case
// runs on a fresh runtime; the deploy -> types -> send/set-variable cycle
// mirrors the executions' compileDeploy -> assertStmtTypes -> sendEventBean
// -> assertPropsNew -> runtimeSetVariable -> undeployAll flow:
//
//   - get-fields (ExprDTGetFields): utildate.get over msec/sec/minutes/
//     hour/day/month/year/week on 2002-05-30T09:01:02.003 emits
//     {3,2,1,9,30,4,2002,22} — month is Calendar 0-based and week is the
//     ISO WEEK_OF_YEAR 22.
//   - get-input (ExprDTGetInput): milestone 0 reads get('month') across all
//     five SupportDateTime representations of 2002-05-30T09:00:00.000,
//     emitting {4,4,4,5,5} (Calendar-backed reps are 0-based, LDT/ZDT are
//     1-based); milestone 1 reads abc.get('month') on a SupportTimeStartEndA
//     event — Esper's event-datetime .get resolves the configured
//     start-timestamp property (longdateStart) — emitting {4}. Milestone 2
//     of the Java execution (e.get()/e.get('abc') on SupportEventWithJustGet,
//     which pins bean-method overload preference over the datetime .get) is
//     not expressible through the datetime DSL and is intentionally excluded.
//   - plusminus-simple (ExprDTPlusMinusSimple): create variable long
//     varmsec (0) deploys first; the engine clock advances to
//     2002-05-30T09:00:00.000 and twelve plus/minus(varmsec) columns run
//     over the null bean (only the current_timestamp columns emit), then
//     varmsec 0/1000/172800000 shift all representations by 0/+1s/+2d.
//   - plusminus-timeperiod (ExprDTPlusMinusTimePeriod): the same twelve
//     columns over the literal period "1 hour 10 sec 20 msec" — 3610020ms —
//     emit 10:00:10.020/07:59:49.980; the null-bean send leaves only the
//     two current_timestamp columns shifted.
//   - withdate (ExprDTWithDate): three int variables drive
//     withDate(varyear,varmonth,varday); a null field keeps the input's
//     field value. Java spells varmonth 0-based on the Calendar-backed reps
//     and varmonth+1 on LDT/ZDT; Go's 1-based builder takes varmonth+1 on
//     every rep, collapsing the EPL asymmetry to one call.
//   - withtime (ExprDTWithTime): four int variables drive
//     withTime(varhour,varmin,varsec,varmsec) with the same null-keeps-field
//     semantics, covering the all-null no-op send and the partial
//     (0,null,null,6) zero-hour send.
//
// Go maps get() columns to DateTimeGet (0-based month, with Add(+1) on the
// LDT/ZDT reps of get-input), variable-arg plus/minus to
// DateTimePlusExpr/DateTimeMinusExpr over VariableRef[int64], the literal
// time period to DateTimePlus/DateTimeMinus at 3610020ms, and
// withDate/withTime to DateTimeWithDateExpr/DateTimeWithTimeExpr over
// VariableRef + Add(+1) for the month argument. Instant-valued cells render
// as epoch-millis numbers and null cells as the tagged {"state":"null"}
// object on both sides (the ExprDTRound instant-token convention);
// get() columns render Integer cells directly.

const exprDTRemainder552ID = "expr-dt-remainder-552"
const exprDTRemainder552JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

// exprDTRemainder552Source is the scenario javaSource pin: the suite
// directory shared by the four source files (the expr-dt-set-nested-550
// multi-source precedent); the file-level list lives in
// exprDTRemainder552JavaSources.
const exprDTRemainder552Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime"

var exprDTRemainder552JavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTGet.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTPlusMinus.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTWithDate.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTWithTime.java",
}

var exprDTRemainder552JavaRuntimeIDs = []string{
	"java-runtime-5080e24710592989a761",
	"java-runtime-bf73801b376a885fb2fa",
	"java-runtime-684e3de645e51f1896c9",
	"java-runtime-fb1954df531a2bf64b7d",
	"java-runtime-e14ad77bdd6257964633",
	"java-runtime-a3c430889e0a5976707a",
}

var exprDTRemainder552JavaExecutions = []string{
	"ExprDTGetFields",
	"ExprDTGetInput",
	"ExprDTPlusMinusSimple",
	"ExprDTPlusMinusTimePeriod",
	"ExprDTWithDate",
	"ExprDTWithTime",
}

var exprDTRemainder552Cases = []string{
	"get-fields",
	"get-input",
	"plusminus-simple",
	"plusminus-timeperiod",
	"withdate",
	"withtime",
}

var exprDTRemainder552CaseRuntimeIDs = map[string]string{
	"get-fields":           "java-runtime-5080e24710592989a761",
	"get-input":            "java-runtime-bf73801b376a885fb2fa",
	"plusminus-simple":     "java-runtime-684e3de645e51f1896c9",
	"plusminus-timeperiod": "java-runtime-fb1954df531a2bf64b7d",
	"withdate":             "java-runtime-e14ad77bdd6257964633",
	"withtime":             "java-runtime-a3c430889e0a5976707a",
}

var exprDTRemainder552JavaStaticIDs = []string{
	"java-be89d7aef06f908e34ed",
	"java-be89d7aef06f908e34ed",
	"java-038b5ae418fa9a980078",
	"java-038b5ae418fa9a980078",
	"java-709b9139cb524eef97dd",
	"java-209fc0139b57c99890bb",
}

var exprDTRemainder552JavaFlags = []string{}

var exprDTRemainder552Ordinals = []int{0, 1, 0, 1, 0, 0}

const exprDTRemainder552Description = "ExprDTGet/ExprDTPlusMinus/ExprDTWithDate/ExprDTWithTime " +
	"executions: get-fields replays ExprDTGetFields (utildate.get over " +
	"msec,sec,minutes,hour,day,month,year,week on 2002-05-30T09:01:02.003 emits " +
	"{3,2,1,9,30,4,2002,22} — 0-based month, ISO week 22), get-input replays " +
	"ExprDTGetInput (get('month') over all five representations of " +
	"2002-05-30T09:00:00.000 emits {4,4,4,5,5} — Calendar-backed reps 0-based " +
	"vs LDT/ZDT 1-based, then the SupportTimeStartEndA event milestone emits " +
	"{4}; the SupportEventWithJustGet bean-method milestone is excluded as " +
	"non-datetime DSL), plusminus-simple replays ExprDTPlusMinusSimple " +
	"(varmsec 0/1000/172800000 shifts all representations of " +
	"2002-05-30T09:00:00.000 by 0/+1s/+2d; the null-bean send emits only " +
	"current_timestamp), plusminus-timeperiod replays " +
	"ExprDTPlusMinusTimePeriod (.plus/.minus(1 hour 10 sec 20 msec) = " +
	"+/-3610020ms emits 10:00:10.020/07:59:49.980), withdate replays " +
	"ExprDTWithDate (variable varyear/varmonth/varday with null-keeps-field: " +
	"(2004,8,3) emits 2004-09-03T09:00, (null,8,null) emits 2002-09-30T09:00), " +
	"withtime replays ExprDTWithTime (variable varhour/varmin/varsec/varmsec " +
	"with null-keeps-field: all-null emits the unchanged instant, (1,2,3,4) " +
	"emits 01:02:03.004, (0,null,null,6) emits 00:00:00.006)."

var exprDTRemainder552CaseObservations = []string{
	"listener+types; deploy s0 utildate.get over msec,sec,minutes,hour,day," +
		"month,year,week, types pins all-Integer, send " +
		"make(2002-05-30T09:01:02.003) emits {3,2,1,9,30,4,2002,22}",
	"listener x2+types; deploy s0 get('month') over all five representations, " +
		"types pins all-Integer, send make(2002-05-30T09:00:00.000) emits " +
		"{4,4,4,5,5}, undeploy, redeploy s0 abc.get('month') on " +
		"SupportTimeStartEndA, send make(A0,2002-05-30T09:00:00.000,0) emits " +
		"{4} (SupportEventWithJustGet milestone excluded: bean-method get() " +
		"preference is not datetime DSL)",
	"listener x4+types; deploy 'var' create variable long varmsec, " +
		"advanceTime(2002-05-30T09:00:00.000), deploy s0 twelve " +
		"plus/minus(varmsec) columns, types pins " +
		"{Long,Date,Long,Calendar,LocalDateTime,ZonedDateTime}x2, send " +
		"make(null) emits only current_timestamp x2, send make(startTime) at " +
		"varmsec=0 emits the instant x12, set varmsec=1000 emits +/-1s, set " +
		"varmsec=172800000 emits +/-2d",
	"listener x2+types; advanceTime(2002-05-30T09:00:00.000), deploy s0 " +
		"twelve plus/minus(1 hour 10 sec 20 msec) columns, types pins " +
		"{Long,Date,Long,Calendar,LocalDateTime,ZonedDateTime}x2, send " +
		"make(startTime) emits 10:00:10.020 x6 + 07:59:49.980 x6, send " +
		"make(null) emits only the two current_timestamp columns",
	"listener x3+types; advanceTime(2002-05-30T09:00:00.000), deploy s0 " +
		"create-variable module + withDate(varyear,varmonth,varday), types " +
		"pins {Long,Date,Long,Calendar,LocalDateTime,ZonedDateTime}, send " +
		"make(null) emits only current_timestamp, set (2004,8,3) emits " +
		"2004-09-03T09:00 x6, set (null,8,null) emits 2002-09-30T09:00 x6",
	"listener x4+types; deploy 'variables' create variable module, " +
		"advanceTime(2002-05-30T09:00:00.000), deploy s0 " +
		"withTime(varhour,varmin,varsec,varmsec), types pins " +
		"{Long,Date,Long,Calendar,LocalDateTime,ZonedDateTime}, send " +
		"make(null) emits only current_timestamp, send at varhour=null emits " +
		"the unchanged instant x6, set (1,2,3,4) emits 01:02:03.004 x6, set " +
		"(0,null,null,6) emits 00:00:00.006 x6",
}

// exprDTRemainder552GetFieldsEPL is the byte-exact ExprDTGetFields ord-0
// statement: eight utildate.get() field reads.
const exprDTRemainder552GetFieldsEPL = "@name('s0') select " +
	"utildate.get('msec') as val0," +
	"utildate.get('sec') as val1," +
	"utildate.get('minutes') as val2," +
	"utildate.get('hour') as val3," +
	"utildate.get('day') as val4," +
	"utildate.get('month') as val5," +
	"utildate.get('year') as val6," +
	"utildate.get('week') as val7" +
	" from SupportDateTime"

// exprDTRemainder552GetInputEPL is the byte-exact ExprDTGetInput milestone-0
// statement — note the Java source's literal spaces after 'val2,' and
// 'val4 ' are preserved.
const exprDTRemainder552GetInputEPL = "@name('s0') select " +
	"utildate.get('month') as val0," +
	"longdate.get('month') as val1," +
	"caldate.get('month') as val2, " +
	"localdate.get('month') as val3, " +
	"zoneddate.get('month') as val4 " +
	" from SupportDateTime"

// exprDTRemainder552GetInputMile1EPL is the byte-exact ExprDTGetInput
// milestone-1 statement: abc.get('month') over SupportTimeStartEndA, where
// Esper's event-datetime .get resolves the configured start-timestamp
// property (longdateStart).
const exprDTRemainder552GetInputMile1EPL = "@name('s0') select abc.get('month') as val0 from SupportTimeStartEndA as abc"

// exprDTRemainder552PlusSimpleVarsEPL is the byte-exact ExprDTPlusMinusSimple
// variable deployment: a single long varmsec (initial 0).
const exprDTRemainder552PlusSimpleVarsEPL = "@name('var') @public create variable long varmsec"

// exprDTRemainder552PlusSimpleEPL is the byte-exact ExprDTPlusMinusSimple
// s0 statement: twelve expression-argument plus/minus(varmsec) columns.
const exprDTRemainder552PlusSimpleEPL = "@name('s0') select " +
	"current_timestamp.plus(varmsec) as val0," +
	"utildate.plus(varmsec) as val1," +
	"longdate.plus(varmsec) as val2," +
	"caldate.plus(varmsec) as val3," +
	"localdate.plus(varmsec) as val4," +
	"zoneddate.plus(varmsec) as val5," +
	"current_timestamp.minus(varmsec) as val6," +
	"utildate.minus(varmsec) as val7," +
	"longdate.minus(varmsec) as val8," +
	"caldate.minus(varmsec) as val9," +
	"localdate.minus(varmsec) as val10," +
	"zoneddate.minus(varmsec) as val11" +
	" from SupportDateTime"

// exprDTRemainder552TimePeriodEPL is the byte-exact ExprDTPlusMinusTimePeriod
// s0 statement: the literal "1 hour 10 sec 20 msec" period is 3610020ms —
// the Go side constant-folds it through DateTimePlus/DateTimeMinus.
const exprDTRemainder552TimePeriodEPL = "@name('s0') select " +
	"current_timestamp.plus(1 hour 10 sec 20 msec) as val0," +
	"utildate.plus(1 hour 10 sec 20 msec) as val1," +
	"longdate.plus(1 hour 10 sec 20 msec) as val2," +
	"caldate.plus(1 hour 10 sec 20 msec) as val3," +
	"localdate.plus(1 hour 10 sec 20 msec) as val4," +
	"zoneddate.plus(1 hour 10 sec 20 msec) as val5," +
	"current_timestamp.minus(1 hour 10 sec 20 msec) as val6," +
	"utildate.minus(1 hour 10 sec 20 msec) as val7," +
	"longdate.minus(1 hour 10 sec 20 msec) as val8," +
	"caldate.minus(1 hour 10 sec 20 msec) as val9," +
	"localdate.minus(1 hour 10 sec 20 msec) as val10," +
	"zoneddate.minus(1 hour 10 sec 20 msec) as val11" +
	" from SupportDateTime"

// exprDTRemainder552WithDateEPL is the byte-exact ExprDTWithDate module: the
// three int variable declarations deploy with the s0 select. Java's
// Calendar-backed reps take the 0-based varmonth while the LDT/ZDT reps
// take varmonth+1; Go's 1-based builder takes varmonth+1 uniformly, so both
// columns collapse to the same September result.
const exprDTRemainder552WithDateEPL = "create variable int varyear;\n" +
	"create variable int varmonth;\n" +
	"create variable int varday;\n" +
	"@name('s0') select " +
	"current_timestamp.withDate(varyear, varmonth, varday) as val0," +
	"utildate.withDate(varyear, varmonth, varday) as val1," +
	"longdate.withDate(varyear, varmonth, varday) as val2," +
	"caldate.withDate(varyear, varmonth, varday) as val3," +
	"localdate.withDate(varyear, varmonth+1, varday) as val4," +
	"zoneddate.withDate(varyear, varmonth+1, varday) as val5" +
	" from SupportDateTime"

// exprDTRemainder552WithTimeVarsEPL is the byte-exact ExprDTWithTime
// variable deployment: four int variables in their own module.
const exprDTRemainder552WithTimeVarsEPL = "@name('variables') @public create variable int varhour;\n" +
	"@public create variable int varmin;\n" +
	"@public create variable int varsec;\n" +
	"@public create variable int varmsec;\n"

// exprDTRemainder552WithTimeEPL is the byte-exact ExprDTWithTime s0
// statement: four expression-argument withTime columns.
const exprDTRemainder552WithTimeEPL = "@name('s0') select " +
	"current_timestamp.withTime(varhour, varmin, varsec, varmsec) as val0," +
	"utildate.withTime(varhour, varmin, varsec, varmsec) as val1," +
	"longdate.withTime(varhour, varmin, varsec, varmsec) as val2," +
	"caldate.withTime(varhour, varmin, varsec, varmsec) as val3," +
	"localdate.withTime(varhour, varmin, varsec, varmsec) as val4," +
	"zoneddate.withTime(varhour, varmin, varsec, varmsec) as val5" +
	" from SupportDateTime"

// exprDTRemainder552Deploy pins one deploy step: the statement label, the
// byte-exact EPL, and the environment variables the Go side registers for
// create-variable deployments (statement labels other than "s0" carry no
// select; the withdate s0 module registers its variables before the select
// builds).
type exprDTRemainder552Deploy struct {
	Statement string
	EPL       string
	Variables []string
}

// exprDTRemainder552Deploys pins each case's deployments in order.
var exprDTRemainder552Deploys = map[string][]exprDTRemainder552Deploy{
	"get-fields": {
		{Statement: "s0", EPL: exprDTRemainder552GetFieldsEPL},
	},
	"get-input": {
		{Statement: "s0", EPL: exprDTRemainder552GetInputEPL},
		{Statement: "s0", EPL: exprDTRemainder552GetInputMile1EPL},
	},
	"plusminus-simple": {
		{Statement: "var", EPL: exprDTRemainder552PlusSimpleVarsEPL, Variables: []string{"varmsec"}},
		{Statement: "s0", EPL: exprDTRemainder552PlusSimpleEPL},
	},
	"plusminus-timeperiod": {
		{Statement: "s0", EPL: exprDTRemainder552TimePeriodEPL},
	},
	"withdate": {
		{Statement: "s0", EPL: exprDTRemainder552WithDateEPL,
			Variables: []string{"varyear", "varmonth", "varday"}},
	},
	"withtime": {
		{Statement: "variables", EPL: exprDTRemainder552WithTimeVarsEPL,
			Variables: []string{"varhour", "varmin", "varsec", "varmsec"}},
		{Statement: "s0", EPL: exprDTRemainder552WithTimeEPL},
	},
}

// exprDTRemainder552VariableInitials pins each variable's declared initial
// value: Java's long varmsec starts 0, the int variables start null.
var exprDTRemainder552VariableInitials = map[string]map[string]any{
	"plusminus-simple": {"varmsec": int64(0)},
	"withdate":         {"varyear": nil, "varmonth": nil, "varday": nil},
	"withtime":         {"varhour": nil, "varmin": nil, "varsec": nil, "varmsec": nil},
}

// exprDTRemainder552VarSet pins one runtimeSetVariable step: the deployment
// statement name Java addresses, the variable, and the assigned value (null
// keeps the field in withDate/withTime).
type exprDTRemainder552VarSet struct {
	Statement string
	Name      string
	Value     any
}

// exprDTRemainder552VarSets pins each case's set-variable sequence in step
// order; the send steps interleave exactly as the executions' sendEventBean
// calls do (the pinned step tables pair each set-variable with its send).
var exprDTRemainder552VarSets = map[string][]exprDTRemainder552VarSet{
	"plusminus-simple": {
		{Statement: "var", Name: "varmsec", Value: int64(1000)},
		{Statement: "var", Name: "varmsec", Value: int64(172800000)},
	},
	"withdate": {
		{Statement: "s0", Name: "varyear", Value: int64(2004)},
		{Statement: "s0", Name: "varmonth", Value: int64(8)},
		{Statement: "s0", Name: "varday", Value: int64(3)},
		{Statement: "s0", Name: "varyear", Value: nil},
		{Statement: "s0", Name: "varmonth", Value: int64(8)},
		{Statement: "s0", Name: "varday", Value: nil},
	},
	"withtime": {
		{Statement: "variables", Name: "varhour", Value: nil},
		{Statement: "variables", Name: "varhour", Value: int64(1)},
		{Statement: "variables", Name: "varmin", Value: int64(2)},
		{Statement: "variables", Name: "varsec", Value: int64(3)},
		{Statement: "variables", Name: "varmsec", Value: int64(4)},
		{Statement: "variables", Name: "varhour", Value: int64(0)},
		{Statement: "variables", Name: "varmin", Value: nil},
		{Statement: "variables", Name: "varsec", Value: nil},
		{Statement: "variables", Name: "varmsec", Value: int64(6)},
	},
}

// exprDTRemainder552CaseEPLs pins each case's representative EPL for the
// case metadata (the milestone-0 s0 statement; get-input's representative
// is the five-representation select).
var exprDTRemainder552CaseEPLs = []string{
	exprDTRemainder552GetFieldsEPL,
	exprDTRemainder552GetInputEPL,
	exprDTRemainder552PlusSimpleEPL,
	exprDTRemainder552TimePeriodEPL,
	exprDTRemainder552WithDateEPL,
	exprDTRemainder552WithTimeEPL,
}

// exprDTRemainder552T00 is DateTime.parseDefaultMSec("2002-05-30T09:00:00.000"),
// the advance-time and populated-send instant for every case except
// get-fields (which pins 2002-05-30T09:01:02.003).
const exprDTRemainder552T00 = "2002-05-30T09:00:00.000Z"
const exprDTRemainder552T00Millis = int64(1022749200000)

// exprDTRemainder552TGet is get-fields' send instant
// DateTime.parseDefaultMSec("2002-05-30T09:01:02.003").
const exprDTRemainder552TGet = "2002-05-30T09:01:02.003Z"

// exprDTRemainder552TimePeriodMillis is the folded "1 hour 10 sec 20 msec"
// period: 3600000 + 10000 + 20.
const exprDTRemainder552TimePeriodMillis = int64(3610020)

// exprDTRemainder552Expected pins every send's assertPropsNew cells in
// select order: Integer/int cells and epoch-millis instants are JSON
// numbers, a nil cell renders the tagged {"state":"null"} token both sides
// emit:
//
//   - get-fields: the eight get() reads {3,2,1,9,30,4,2002,22}.
//   - get-input: {4,4,4,5,5} for milestone 0, {4} for milestone 1.
//   - plusminus-simple: null-bean row (only the two current_timestamp
//     columns emit T00), varmsec=0 identity, +/-1000ms, +/-172800000ms.
//   - plusminus-timeperiod: +/-3610020ms rows, then the null-bean row.
//   - withdate: null-bean row, (2004,8,3)->2004-09-03T09:00, the
//     null-keeps-field (null,8,null)->2002-09-30T09:00.
//   - withtime: null-bean row, all-null no-op, (1,2,3,4)->01:02:03.004,
//     (0,null,null,6)->00:00:00.006.
var exprDTRemainder552Expected = func() map[string][][]any {
	null := any(nil)
	repeat := func(value any, count int) []any {
		row := make([]any, count)
		for index := range row {
			row[index] = value
		}
		return row
	}
	nullBean12 := func(lead, tail int64) []any {
		row := repeat(null, 12)
		row[0], row[6] = lead, tail
		return row
	}
	nullBean6 := func(lead int64) []any {
		row := repeat(null, 6)
		row[0] = lead
		return row
	}
	return map[string][][]any{
		"get-fields": {
			{int64(3), int64(2), int64(1), int64(9), int64(30), int64(4), int64(2002), int64(22)},
		},
		"get-input": {
			{int64(4), int64(4), int64(4), int64(5), int64(5)},
			{int64(4)},
		},
		"plusminus-simple": {
			nullBean12(exprDTRemainder552T00Millis, exprDTRemainder552T00Millis),
			repeat(int64(1022749200000), 12),
			append(repeat(int64(1022749201000), 6), repeat(int64(1022749199000), 6)...),
			append(repeat(int64(1022922000000), 6), repeat(int64(1022576400000), 6)...),
		},
		"plusminus-timeperiod": {
			append(repeat(int64(1022752810020), 6), repeat(int64(1022745589980), 6)...),
			nullBean12(1022752810020, 1022745589980),
		},
		"withdate": {
			nullBean6(exprDTRemainder552T00Millis),
			repeat(int64(1094202000000), 6),
			repeat(int64(1033376400000), 6),
		},
		"withtime": {
			nullBean6(exprDTRemainder552T00Millis),
			repeat(exprDTRemainder552T00Millis, 6),
			repeat(int64(1022720523004), 6),
			repeat(int64(1022716800006), 6),
		},
	}
}()

// exprDTRemainder552SendDates pins each send's date in step order: a null
// entry is the make(null) all-null bean. get-input's second entry feeds the
// SupportTimeStartEndA milestone.
var exprDTRemainder552SendDates = map[string][]string{
	"get-fields":           {exprDTRemainder552TGet},
	"get-input":            {exprDTRemainder552T00, exprDTRemainder552T00},
	"plusminus-simple":     {"", exprDTRemainder552T00, exprDTRemainder552T00, exprDTRemainder552T00},
	"plusminus-timeperiod": {exprDTRemainder552T00, ""},
	"withdate":             {"", exprDTRemainder552T00, exprDTRemainder552T00},
	"withtime":             {"", exprDTRemainder552T00, exprDTRemainder552T00, exprDTRemainder552T00},
}

// exprDTRemainder552SendEventTypes pins each send's event type in step
// order: get-input's milestone-1 send carries SupportTimeStartEndA.
var exprDTRemainder552SendEventTypes = map[string][]string{
	"get-fields":           {"SupportDateTime"},
	"get-input":            {"SupportDateTime", "SupportTimeStartEndA"},
	"plusminus-simple":     {"SupportDateTime", "SupportDateTime", "SupportDateTime", "SupportDateTime"},
	"plusminus-timeperiod": {"SupportDateTime", "SupportDateTime"},
	"withdate":             {"SupportDateTime", "SupportDateTime", "SupportDateTime"},
	"withtime":             {"SupportDateTime", "SupportDateTime", "SupportDateTime", "SupportDateTime"},
}

// exprDTRemainder552TypeProperties pins the Java-asserted property types
// per types step per case (assertStmtTypes/assertStmtTypesAllSame): the
// get() columns are Integer and the rep-preserving columns pin
// LONGBOXED/DATE/CALENDAR/LOCALDATETIME/ZONEDDATETIME — Go verifies the
// collapsed int64/time.Time schema.
var exprDTRemainder552TypeProperties = map[string][]map[string]string{
	"get-fields": {
		{"val0": "Integer", "val1": "Integer", "val2": "Integer", "val3": "Integer",
			"val4": "Integer", "val5": "Integer", "val6": "Integer", "val7": "Integer"},
	},
	"get-input": {
		{"val0": "Integer", "val1": "Integer", "val2": "Integer",
			"val3": "Integer", "val4": "Integer"},
	},
	"plusminus-simple": {
		exprDTRemainder552RepTypes(12),
	},
	"plusminus-timeperiod": {
		exprDTRemainder552RepTypes(12),
	},
	"withdate": {
		exprDTRemainder552RepTypes(6),
	},
	"withtime": {
		exprDTRemainder552RepTypes(6),
	},
}

// exprDTRemainder552RepTypes renders the LONGBOXED/DATE/LONGBOXED/CALENDAR/
// LOCALDATETIME/ZONEDDATETIME cycle assertStmtTypes pins for the
// rep-preserving selects.
func exprDTRemainder552RepTypes(count int) map[string]string {
	names := []string{"Long", "Date", "Long", "Calendar", "LocalDateTime", "ZonedDateTime"}
	properties := make(map[string]string, count)
	for index := 0; index < count; index++ {
		properties[fmt.Sprintf("val%d", index)] = names[index%len(names)]
	}
	return properties
}

type exprDTRemainder552CaseState struct {
	caseName    string
	env         *esper.Environment
	engine      *esper.Engine
	trace       *compat.Trace
	sequence    uint64
	deployIndex int
	s0Deploys   int
	typesIndex  int
	sendIndex   int
	setIndex    int
	deployment  *esper.Deployment
	plan        esper.Plan
	fired       bool
	row         compat.ResultRecord
}

func runExprDTRemainder552Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range exprDTRemainder552Cases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runExprDTRemainder552Case(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprDTRemainder552ID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprDTRemainder552ID, scenario.ID)
	}
	return trace, nil
}

func runExprDTRemainder552Case(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterMap(env, "SupportDateTime", exprDTRemainder552DateTimeFieldSpecs()); err != nil {
		return compat.Trace{}, fmt.Errorf("%s register SupportDateTime: %w", exprDTRemainder552ID, err)
	}
	if _, err := esper.RegisterMap(env, "SupportTimeStartEndA", exprDTRemainder552StartEndFieldSpecs()); err != nil {
		return compat.Trace{}, fmt.Errorf("%s register SupportTimeStartEndA: %w", exprDTRemainder552ID, err)
	}
	state := &exprDTRemainder552CaseState{
		caseName: caseName,
		env:      env,
		engine: esper.NewEngine(env,
			esper.WithStartTime(time.Unix(0, 0).UTC()),
			esper.WithRuntimeURI(exprDTRemainder552CaseRuntimeIDs[caseName])),
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
				return *state.trace, fmt.Errorf("%s: parse advance-time %q: %w", exprDTRemainder552ID, step.At, err)
			}
			if step.At != exprDTRemainder552T00 {
				return *state.trace, fmt.Errorf("%s: case %q advance-time %q is not pinned",
					exprDTRemainder552ID, caseName, step.At)
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
		case "set-variable":
			if err := state.setVariable(ctx, step); err != nil {
				return *state.trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *state.trace, err
			}
		default:
			return *state.trace, fmt.Errorf("%s: unsupported step op %q", exprDTRemainder552ID, step.Op)
		}
	}
	return *state.trace, nil
}

// exprDTRemainder552DateTimeFieldSpecs mirrors SupportDateTime: longdate is
// the epoch-millis Long and the other four representations collapse to
// time.Time.
func exprDTRemainder552DateTimeFieldSpecs() []esper.FieldSpec {
	return []esper.FieldSpec{
		esper.FieldDef("longdate", reflect.TypeOf(int64(0))),
		esper.FieldDef("utildate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("caldate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("localdate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("zoneddate", reflect.TypeOf(time.Time{})),
	}
}

// exprDTRemainder552StartEndFieldSpecs mirrors the SupportTimeStartEndA
// properties the get-input milestone reads: Esper's event-datetime
// abc.get('month') resolves the configured start-timestamp property, the
// Long-typed longdateStart, which collapses to int64.
func exprDTRemainder552StartEndFieldSpecs() []esper.FieldSpec {
	return []esper.FieldSpec{
		esper.FieldDef("longdateStart", reflect.TypeOf(int64(0))),
	}
}

// deploy mirrors one compileDeploy (+ addListener for s0) cycle; the pinned
// EPL for this deploy index is verified before the fluent equivalent is
// built. Deployments carrying create-variable EPL register the pinned
// environment variables (Java's deployment-scoped variables collapse to
// the environment surface); s0 deployments then build and deploy the
// select.
func (s *exprDTRemainder552CaseState) deploy(ctx context.Context, step compat.Step) error {
	pinned, ok := exprDTRemainder552Deploys[s.caseName]
	if !ok || s.deployIndex >= len(pinned) {
		return fmt.Errorf("%s: case %q has no pinned deploy %d", exprDTRemainder552ID, s.caseName, s.deployIndex)
	}
	pin := pinned[s.deployIndex]
	s.deployIndex++
	if step.Statement != pin.Statement || step.Epl != pin.EPL {
		return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q",
			exprDTRemainder552ID, s.caseName, step.Statement, step.Epl)
	}
	for _, name := range pin.Variables {
		if err := s.env.RegisterVariable(name, exprDTRemainder552VariableInitials[s.caseName][name],
			esper.VariableType(reflect.TypeOf(int64(0)))); err != nil {
			return fmt.Errorf("%s: register variable %q: %w", exprDTRemainder552ID, name, err)
		}
	}
	if pin.Statement != "s0" {
		return nil
	}
	if err := s.undeployAll(ctx); err != nil {
		return err
	}
	query, err := s.buildSelect(s.s0Deploys)
	if err != nil {
		return err
	}
	s.s0Deploys++
	s.typesIndex = 0
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
			exprDTRemainder552RenderMillis(newRows)
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
// time.Time reps:
//
//   - get-fields: eight DateTimeGet reads on utildate.
//   - get-input milestone 0: DateTimeGet('month') is 0-based everywhere, so
//     the LDT/ZDT reps add 1 to reach Java's 1-based ChronoField read;
//     milestone 1 reads the SupportTimeStartEndA longdateStart property the
//     event-datetime .get resolves (the configured start-timestamp field).
//   - plusminus-simple: DateTimePlusExpr/DateTimeMinusExpr over the varmsec
//     VariableRef (Java's expression ms argument).
//   - plusminus-timeperiod: the folded 3610020ms period through
//     DateTimePlus/DateTimeMinus.
//   - withdate: DateTimeWithDateExpr over VariableRef args with the month
//     arg as varmonth+1 — Go's builder is 1-based, so Java's split
//     varmonth/varmonth+1 EPL collapses to one call per rep.
//   - withtime: DateTimeWithTimeExpr over the four VariableRef args.
func (s *exprDTRemainder552CaseState) buildSelect(s0Index int) (esper.Query, error) {
	longdate := esper.Field[map[string]any, int64]("longdate")
	utildate := esper.Field[map[string]any, time.Time]("utildate")
	caldate := esper.Field[map[string]any, time.Time]("caldate")
	localdate := esper.Field[map[string]any, time.Time]("localdate")
	zoneddate := esper.Field[map[string]any, time.Time]("zoneddate")
	switch s.caseName {
	case "get-fields":
		units := []string{"msec", "sec", "minutes", "hour", "day", "month", "year", "week"}
		selections := make([]esper.Selection, 0, len(units))
		for index, unit := range units {
			selections = append(selections, esper.Alias(fmt.Sprintf("val%d", index),
				esper.DateTimeGet[time.Time](utildate, unit)))
		}
		return esper.FromAny(s.env, "SupportDateTime").Select(selections...).
			Query(esper.StatementName("s0")), nil
	case "get-input":
		if s0Index == 0 {
			plusOne := esper.Literal[int64](1)
			return esper.FromAny(s.env, "SupportDateTime").Select(
				esper.Alias("val0", esper.DateTimeGet[time.Time](utildate, "month")),
				esper.Alias("val1", esper.DateTimeGet[int64](longdate, "month")),
				esper.Alias("val2", esper.DateTimeGet[time.Time](caldate, "month")),
				esper.Alias("val3", esper.Add[int64](
					esper.DateTimeGet[time.Time](localdate, "month"), plusOne)),
				esper.Alias("val4", esper.Add[int64](
					esper.DateTimeGet[time.Time](zoneddate, "month"), plusOne)),
			).Query(esper.StatementName("s0")), nil
		}
		longdateStart := esper.Field[map[string]any, int64]("longdateStart")
		return esper.FromAny(s.env, "SupportTimeStartEndA").Select(
			esper.Alias("val0", esper.DateTimeGet[int64](longdateStart, "month")),
		).Query(esper.StatementName("s0")), nil
	case "plusminus-simple":
		varmsec := esper.VariableRef[int64]("varmsec")
		return esper.FromAny(s.env, "SupportDateTime").Select(
			esper.Alias("val0", esper.DateTimePlusExpr[int64](esper.CurrentTimestamp(), varmsec)),
			esper.Alias("val1", esper.DateTimePlusExpr[time.Time](utildate, varmsec)),
			esper.Alias("val2", esper.DateTimePlusExpr[int64](longdate, varmsec)),
			esper.Alias("val3", esper.DateTimePlusExpr[time.Time](caldate, varmsec)),
			esper.Alias("val4", esper.DateTimePlusExpr[time.Time](localdate, varmsec)),
			esper.Alias("val5", esper.DateTimePlusExpr[time.Time](zoneddate, varmsec)),
			esper.Alias("val6", esper.DateTimeMinusExpr[int64](esper.CurrentTimestamp(), varmsec)),
			esper.Alias("val7", esper.DateTimeMinusExpr[time.Time](utildate, varmsec)),
			esper.Alias("val8", esper.DateTimeMinusExpr[int64](longdate, varmsec)),
			esper.Alias("val9", esper.DateTimeMinusExpr[time.Time](caldate, varmsec)),
			esper.Alias("val10", esper.DateTimeMinusExpr[time.Time](localdate, varmsec)),
			esper.Alias("val11", esper.DateTimeMinusExpr[time.Time](zoneddate, varmsec)),
		).Query(esper.StatementName("s0")), nil
	case "plusminus-timeperiod":
		period := exprDTRemainder552TimePeriodMillis
		return esper.FromAny(s.env, "SupportDateTime").Select(
			esper.Alias("val0", esper.DateTimePlus[int64](esper.CurrentTimestamp(), period)),
			esper.Alias("val1", esper.DateTimePlus[time.Time](utildate, period)),
			esper.Alias("val2", esper.DateTimePlus[int64](longdate, period)),
			esper.Alias("val3", esper.DateTimePlus[time.Time](caldate, period)),
			esper.Alias("val4", esper.DateTimePlus[time.Time](localdate, period)),
			esper.Alias("val5", esper.DateTimePlus[time.Time](zoneddate, period)),
			esper.Alias("val6", esper.DateTimeMinus[int64](esper.CurrentTimestamp(), period)),
			esper.Alias("val7", esper.DateTimeMinus[time.Time](utildate, period)),
			esper.Alias("val8", esper.DateTimeMinus[int64](longdate, period)),
			esper.Alias("val9", esper.DateTimeMinus[time.Time](caldate, period)),
			esper.Alias("val10", esper.DateTimeMinus[time.Time](localdate, period)),
			esper.Alias("val11", esper.DateTimeMinus[time.Time](zoneddate, period)),
		).Query(esper.StatementName("s0")), nil
	case "withdate":
		varyear := esper.VariableRef[int64]("varyear")
		varmonth := esper.Add[int64](esper.VariableRef[int64]("varmonth"), esper.Literal[int64](1))
		varday := esper.VariableRef[int64]("varday")
		return esper.FromAny(s.env, "SupportDateTime").Select(
			esper.Alias("val0", esper.DateTimeWithDateExpr[int64](esper.CurrentTimestamp(), varyear, varmonth, varday)),
			esper.Alias("val1", esper.DateTimeWithDateExpr[time.Time](utildate, varyear, varmonth, varday)),
			esper.Alias("val2", esper.DateTimeWithDateExpr[int64](longdate, varyear, varmonth, varday)),
			esper.Alias("val3", esper.DateTimeWithDateExpr[time.Time](caldate, varyear, varmonth, varday)),
			esper.Alias("val4", esper.DateTimeWithDateExpr[time.Time](localdate, varyear, varmonth, varday)),
			esper.Alias("val5", esper.DateTimeWithDateExpr[time.Time](zoneddate, varyear, varmonth, varday)),
		).Query(esper.StatementName("s0")), nil
	case "withtime":
		varhour := esper.VariableRef[int64]("varhour")
		varmin := esper.VariableRef[int64]("varmin")
		varsec := esper.VariableRef[int64]("varsec")
		varmsec := esper.VariableRef[int64]("varmsec")
		return esper.FromAny(s.env, "SupportDateTime").Select(
			esper.Alias("val0", esper.DateTimeWithTimeExpr[int64](esper.CurrentTimestamp(), varhour, varmin, varsec, varmsec)),
			esper.Alias("val1", esper.DateTimeWithTimeExpr[time.Time](utildate, varhour, varmin, varsec, varmsec)),
			esper.Alias("val2", esper.DateTimeWithTimeExpr[int64](longdate, varhour, varmin, varsec, varmsec)),
			esper.Alias("val3", esper.DateTimeWithTimeExpr[time.Time](caldate, varhour, varmin, varsec, varmsec)),
			esper.Alias("val4", esper.DateTimeWithTimeExpr[time.Time](localdate, varhour, varmin, varsec, varmsec)),
			esper.Alias("val5", esper.DateTimeWithTimeExpr[time.Time](zoneddate, varhour, varmin, varsec, varmsec)),
		).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("%s: unsupported case %q", exprDTRemainder552ID, s.caseName)
	}
}

// types emits the pinned property-type record for this types index after
// verifying the deployed statement's collapsed Go schema: Integer and Long
// columns are int64-typed and every date-time column is time.Time-typed
// (Date and Calendar both collapse to time.Time).
func (s *exprDTRemainder552CaseState) types(step compat.Step) error {
	pins, ok := exprDTRemainder552TypeProperties[s.caseName]
	if !ok || s.typesIndex >= len(pins) || step.Statement != "s0" {
		return fmt.Errorf("%s: case %q has no types assertion for %q",
			exprDTRemainder552ID, s.caseName, step.Statement)
	}
	properties := pins[s.typesIndex]
	s.typesIndex++
	schema, ok := s.plan.ResultSchema()
	if !ok {
		return fmt.Errorf("%s: statement %q has no result schema", exprDTRemainder552ID, step.Statement)
	}
	value := map[string]any{}
	for index := 0; index < len(properties); index++ {
		column := fmt.Sprintf("val%d", index)
		field, exists := schema.Field(column)
		if !exists {
			return fmt.Errorf("%s: s0 is missing column %q", exprDTRemainder552ID, column)
		}
		want := reflect.TypeOf(time.Time{})
		if properties[column] == "Integer" || properties[column] == "Long" {
			want = reflect.TypeOf(int64(0))
		}
		if field.Type != want {
			return fmt.Errorf("%s: s0 %s type drift: %v", exprDTRemainder552ID, column, field.Type)
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
// null; the SupportTimeStartEndA send carries the longdateStart instant the
// event-datetime get resolves — delivers the map event, and verifies the
// delivered row against the pinned expected cells: the assertPropsNew
// equivalent.
func (s *exprDTRemainder552CaseState) send(ctx context.Context, step compat.Step) error {
	eventTypes, ok := exprDTRemainder552SendEventTypes[s.caseName]
	if !ok || s.sendIndex >= len(eventTypes) || step.EventType != eventTypes[s.sendIndex] {
		return fmt.Errorf("%s: case %q send %d carries unpinned event type %q",
			exprDTRemainder552ID, s.caseName, s.sendIndex, step.EventType)
	}
	var payload struct {
		Date     *string `json:"date"`
		Expected []any   `json:"expected"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return fmt.Errorf("%s: decode %s payload: %w", exprDTRemainder552ID, step.EventType, err)
	}
	pinnedDate := exprDTRemainder552SendDates[s.caseName][s.sendIndex]
	pinnedRow := exprDTRemainder552Expected[s.caseName][s.sendIndex]
	s.sendIndex++
	if payload.Date == nil {
		if pinnedDate != "" {
			return fmt.Errorf("%s: case %q null send is not pinned", exprDTRemainder552ID, s.caseName)
		}
	} else if *payload.Date != pinnedDate {
		return fmt.Errorf("%s: case %q send date %q is not pinned",
			exprDTRemainder552ID, s.caseName, *payload.Date)
	}
	if len(payload.Expected) != len(pinnedRow) {
		return fmt.Errorf("%s: case %q send pins %d expected values, want %d",
			exprDTRemainder552ID, s.caseName, len(payload.Expected), len(pinnedRow))
	}
	for index, cell := range payload.Expected {
		if !exprDTRemainder552CellEqual(cell, pinnedRow[index]) {
			return fmt.Errorf("%s: case %q send expected[%d] = %v is not pinned",
				exprDTRemainder552ID, s.caseName, index, cell)
		}
	}
	var event map[string]any
	if payload.Date == nil {
		event = map[string]any{}
	} else {
		parsed, err := time.Parse(time.RFC3339Nano, *payload.Date)
		if err != nil {
			return fmt.Errorf("%s: parse %s date: %w", exprDTRemainder552ID, step.EventType, err)
		}
		switch step.EventType {
		case "SupportDateTime":
			event = map[string]any{
				"longdate":  parsed.UnixMilli(),
				"utildate":  parsed,
				"caldate":   parsed.Add(-time.Duration(parsed.Nanosecond())),
				"localdate": parsed,
				"zoneddate": parsed,
			}
		case "SupportTimeStartEndA":
			event = map[string]any{"longdateStart": parsed.UnixMilli()}
		default:
			return fmt.Errorf("%s: unsupported event type %q", exprDTRemainder552ID, step.EventType)
		}
	}
	s.fired = false
	s.row = compat.ResultRecord{}
	if err := s.engine.SendRecord(ctx, step.EventType, event); err != nil {
		return err
	}
	if !s.fired {
		return fmt.Errorf("%s: %s send produced no listener row", exprDTRemainder552ID, step.EventType)
	}
	return s.verifyExpected(payload.Expected)
}

// setVariable executes one runtimeSetVariable, mirroring
// env.runtimeSetVariable(deploymentName, name, value); the Go engine's
// variable surface is environment-scoped, so the pinned deployment
// statement name is verified but not addressed. A null payload assigns the
// null value Java's nullable int variables carry.
func (s *exprDTRemainder552CaseState) setVariable(ctx context.Context, step compat.Step) error {
	sets, ok := exprDTRemainder552VarSets[s.caseName]
	if !ok || s.setIndex >= len(sets) {
		return fmt.Errorf("%s: case %q has no pinned set-variable %d",
			exprDTRemainder552ID, s.caseName, s.setIndex)
	}
	pin := sets[s.setIndex]
	s.setIndex++
	if step.Statement != pin.Statement || step.Name != pin.Name {
		return fmt.Errorf("%s: case %q set-variable %s.%s is not pinned",
			exprDTRemainder552ID, s.caseName, step.Statement, step.Name)
	}
	var value any
	if len(step.Payload) > 0 && string(step.Payload) != "null" {
		var number int64
		if err := json.Unmarshal(step.Payload, &number); err != nil {
			return fmt.Errorf("%s: decode set-variable payload: %w", exprDTRemainder552ID, err)
		}
		value = number
	}
	if !reflect.DeepEqual(value, pin.Value) {
		return fmt.Errorf("%s: case %q set-variable %s value %v is not pinned",
			exprDTRemainder552ID, s.caseName, step.Name, value)
	}
	return s.engine.SetVariable(ctx, step.Name, value)
}

// verifyExpected compares the delivered row's fields against the payload's
// pinned expected cells in select order, mirroring assertPropsNew; null
// cells compare through the tagged {"state":"null"} token.
func (s *exprDTRemainder552CaseState) verifyExpected(expected []any) error {
	for index, cell := range expected {
		column := fmt.Sprintf("val%d", index)
		actual, ok := s.row.Fields[column]
		if !ok {
			return fmt.Errorf("%s: listener row is missing column %q", exprDTRemainder552ID, column)
		}
		if !exprDTRemainder552CellEqual(cell, actual) {
			return fmt.Errorf("%s: column %q = %v, want %v", exprDTRemainder552ID, column, actual, cell)
		}
	}
	return nil
}

// exprDTRemainder552CellEqual compares a JSON-decoded expected cell with a
// normalized row value: JSON numbers compare against int64 cells and the
// {"state":"null"} token matches the tagged null cell both sides render.
func exprDTRemainder552CellEqual(expected, actual any) bool {
	if want, ok := expected.(map[string]any); ok {
		if want["state"] == "null" {
			got, isMap := actual.(map[string]any)
			return (isMap && got["state"] == "null") || actual == nil
		}
		return false
	}
	if expected == nil {
		return actual == nil
	}
	want, ok := expected.(float64)
	if !ok {
		return false
	}
	got, ok := actual.(int64)
	return ok && got == int64(want)
}

// exprDTRemainder552RenderMillis renders transformed time.Time cells as
// epoch millis, the oracle's instant-token convention for every
// SupportDateTime representation (Date, Calendar, LDT and ZDT all collapse
// to the same instant token); get() Integer columns pass through.
func exprDTRemainder552RenderMillis(rows []compat.ResultRecord) {
	for _, row := range rows {
		for name, value := range row.Fields {
			if located, ok := value.(time.Time); ok {
				row.Fields[name] = located.UnixMilli()
			}
		}
	}
}

// undeployAll tears down the live s0 deployment, mirroring the execution's
// env.undeployAll() between milestones and at case end; create-variable
// deployments carry no select deployment on the Go side (the variables live
// on the environment for the life of the case).
func (s *exprDTRemainder552CaseState) undeployAll(ctx context.Context) error {
	if s.deployment != nil {
		if err := s.deployment.Undeploy(ctx); err != nil {
			return err
		}
		s.deployment = nil
	}
	return nil
}

// exprDTRemainder552CaseSteps pins the complete step sequence per case as
// op|case|statement|name|eventType|epl|payload|expectError|at keys so the
// loader asserts the scenario file matches the contract byte-for-byte.
var exprDTRemainder552CaseSteps = func() map[string][]string {
	steps := map[string][]string{}
	send := func(caseName string, index int) string {
		row := exprDTRemainder552Expected[caseName][index]
		cells := make([]any, len(row))
		for cellIndex, cell := range row {
			if cell == nil {
				cells[cellIndex] = map[string]any{"state": "null"}
			} else {
				cells[cellIndex] = cell
			}
		}
		encoded, err := json.Marshal(cells)
		if err != nil {
			panic(err)
		}
		date := "null"
		if pinned := exprDTRemainder552SendDates[caseName][index]; pinned != "" {
			date = fmt.Sprintf("%q", pinned)
		}
		payload := fmt.Sprintf(`{"date":%s,"expected":%s}`, date, encoded)
		return "send|" + caseName + "|||" + exprDTRemainder552SendEventTypes[caseName][index] +
			"||" + payload + "||"
	}
	deploy := func(caseName string, pin exprDTRemainder552Deploy) string {
		return "deploy|" + caseName + "|" + pin.Statement + "|||" + pin.EPL + "|||"
	}
	types := func(caseName string) string {
		return "types|" + caseName + "|s0||||||"
	}
	setVariable := func(caseName string, index int) string {
		pin := exprDTRemainder552VarSets[caseName][index]
		payload := "null"
		if pin.Value != nil {
			payload = fmt.Sprintf("%d", pin.Value)
		}
		return "set-variable|" + caseName + "|" + pin.Statement + "|" + pin.Name + "|||" + payload + "||"
	}
	advance := func(caseName string) string {
		return "advance-time|" + caseName + "|||||||" + exprDTRemainder552T00
	}
	undeploy := func(caseName string) string {
		return "undeploy-all|" + caseName + "|||||||"
	}
	for _, caseName := range exprDTRemainder552Cases {
		pinned := []string{}
		switch caseName {
		case "get-fields":
			pinned = append(pinned, deploy(caseName, exprDTRemainder552Deploys[caseName][0]),
				types(caseName), send(caseName, 0), undeploy(caseName))
		case "get-input":
			pinned = append(pinned, deploy(caseName, exprDTRemainder552Deploys[caseName][0]),
				types(caseName), send(caseName, 0), undeploy(caseName),
				deploy(caseName, exprDTRemainder552Deploys[caseName][1]),
				send(caseName, 1), undeploy(caseName))
		case "plusminus-simple":
			pinned = append(pinned, deploy(caseName, exprDTRemainder552Deploys[caseName][0]),
				advance(caseName),
				deploy(caseName, exprDTRemainder552Deploys[caseName][1]),
				types(caseName), send(caseName, 0), send(caseName, 1),
				setVariable(caseName, 0), send(caseName, 2),
				setVariable(caseName, 1), send(caseName, 3), undeploy(caseName))
		case "plusminus-timeperiod":
			pinned = append(pinned, advance(caseName),
				deploy(caseName, exprDTRemainder552Deploys[caseName][0]),
				types(caseName), send(caseName, 0), send(caseName, 1), undeploy(caseName))
		case "withdate":
			pinned = append(pinned, advance(caseName),
				deploy(caseName, exprDTRemainder552Deploys[caseName][0]),
				types(caseName), send(caseName, 0),
				setVariable(caseName, 0), setVariable(caseName, 1), setVariable(caseName, 2),
				send(caseName, 1),
				setVariable(caseName, 3), setVariable(caseName, 4), setVariable(caseName, 5),
				send(caseName, 2), undeploy(caseName))
		case "withtime":
			pinned = append(pinned, deploy(caseName, exprDTRemainder552Deploys[caseName][0]),
				advance(caseName),
				deploy(caseName, exprDTRemainder552Deploys[caseName][1]),
				types(caseName), send(caseName, 0),
				setVariable(caseName, 0), send(caseName, 1),
				setVariable(caseName, 1), setVariable(caseName, 2),
				setVariable(caseName, 3), setVariable(caseName, 4),
				send(caseName, 2),
				setVariable(caseName, 5), setVariable(caseName, 6),
				setVariable(caseName, 7), setVariable(caseName, 8),
				send(caseName, 3), undeploy(caseName))
		}
		steps[caseName] = pinned
	}
	return steps
}()

// loadExprDTRemainder552Scenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned metadata and case metadata, and the complete
// pinned step sequence so unknown, duplicated or drifted content fails the
// replay.
func loadExprDTRemainder552Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprDTRemainder552ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprDTRemainder552ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTRemainder552ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTRemainder552ID, err)
	}
	if err := requireExprDTRemainder552Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprDTRemainder552ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprDTRemainder552ID ||
		metadata.Description != exprDTRemainder552Description ||
		metadata.JavaCommit != exprDTRemainder552JavaCommit ||
		metadata.JavaSource != exprDTRemainder552Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprDTRemainder552ID)
	}
	if err := validateExprDTRemainder552StringArray(root["javaRuntimes"], exprDTRemainder552JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTRemainder552StringArray(root["javaNames"], exprDTRemainder552JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTRemainder552StringArray(root["javaStaticIds"], exprDTRemainder552JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTRemainder552StringArray(root["javaFlags"], exprDTRemainder552JavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprDTRemainder552Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", exprDTRemainder552ID, len(exprDTRemainder552Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprDTRemainder552Fields(object,
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
		if definition.Case != exprDTRemainder552Cases[index] ||
			definition.Ordinal != exprDTRemainder552Ordinals[index] ||
			definition.RuntimeID != exprDTRemainder552JavaRuntimeIDs[index] ||
			definition.ExecutionName != exprDTRemainder552JavaExecutions[index] ||
			definition.Observation != exprDTRemainder552CaseObservations[index] ||
			definition.EPL != exprDTRemainder552CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprDTRemainder552ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", exprDTRemainder552ID, err)
	}
	offset := 0
	for _, caseName := range exprDTRemainder552Cases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", exprDTRemainder552ID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTRemainder552ID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", exprDTRemainder552ID, offset, caseName)
		}
		if _, err := exprDTRemainder552StepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTRemainder552ID, offset, err)
		}
		offset++
		want, ok := exprDTRemainder552CaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", exprDTRemainder552ID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", exprDTRemainder552ID, caseName)
		}
		for _, pinned := range want {
			key, err := exprDTRemainder552StepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTRemainder552ID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", exprDTRemainder552ID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", exprDTRemainder552ID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTRemainder552ID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// exprDTRemainder552StepKey renders one raw step as its pinned key:
// op|case|statement|name|eventType|epl|payload|expectError|at with the
// payload compacted. Unknown fields on the step object are rejected per op.
func exprDTRemainder552StepKey(raw json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return "", err
	}
	var step struct {
		Op          string          `json:"op"`
		Case        string          `json:"case"`
		Statement   string          `json:"statement"`
		Name        string          `json:"name"`
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
		"set-variable": {"op", "case", "statement", "name", "payload"},
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
	if step.Op == "send" || step.Op == "set-variable" {
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, step.Payload); err != nil {
			return "", fmt.Errorf("%s payload: %w", step.Op, err)
		}
		payloadText = compacted.String()
	}
	return step.Op + "|" + step.Case + "|" + step.Statement + "|" + step.Name +
		"|" + step.EventType + "|" + step.Epl + "|" + payloadText +
		"|" + step.ExpectError + "|" + step.At, nil
}

func requireExprDTRemainder552Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", exprDTRemainder552ID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", exprDTRemainder552ID, name)
		}
	}
	return nil
}

func validateExprDTRemainder552StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
