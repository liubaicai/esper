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

// expr_dt_tail_553.go replays the five datetime-tail executions (Draft
// 4.553) against the pinned Java oracle: ExprToCalendarChain and
// ExprDTToDateCalMSecValue (ExprDTToDateCalMSec ords 0/1), ExprDTDocSamples,
// ExprDTIntervalOpsCreateSchema and ExprDTInvalid. Every case runs on a
// fresh runtime:
//
//   - tocalendar-chain (ExprToCalendarChain): current_timestamp.toCalendar()
//     .add(Calendar.DAY_OF_MONTH,1) is a plain Calendar method call, not a
//     date-time chain op, so Java's column evaluates to null. The fluent
//     surface has no Calendar.add(field,n) chain — Go projects NullLiteral
//     and both sides pin the null row (the documented intentionally-
//     different mapping).
//   - todatecalmsec-value (ExprDTToDateCalMSecValue): the eighteen-column
//     toDate/toCalendar/toMillisec matrix over the five SupportDateTime
//     representations plus current_timestamp. Java pins Date x6, Calendar
//     x6, Long x6 statement types; Go's two representations collapse the
//     Date/Calendar columns to time.Time and the toMillisec columns to
//     int64, so the types record pins the Java names while the Go schema is
//     verified as {time.Time x12, int64 x6}. The all-null send leaves only
//     the three current_timestamp cells.
//   - docsamples (ExprDTDocSamples): fifteen compile-only documentation
//     selects over the RFIDEvent map type (format, get('month'),
//     getMonthOfYear, minus/plus in duration-literal and arithmetic forms,
//     roundCeiling/roundFloor, set, withDate, withMax, toCalendar, toDate,
//     toMillisec) each emit a deployed marker, then four pattern after()
//     probes over the start-timestamp-declared A/B event types emit a
//     listener-invoked count per probe.
//   - intervalops-createschema (ExprDTIntervalOpsCreateSchema): the
//     create-schema startts/endts timestamp declarations plus
//     a.includes(b). Java iterates five representations x five field types;
//     the Avro/JSON/JSON-provided legs have no Go boundary, the DEFAULT
//     (annotationText="") rep legs are excluded as map-equivalent
//     duplicates of the map legs, and the four Java date-time classes
//     collapse to time.Time, so the pinned steps cover the map and
//     object-array representations x {long,
//     Calendar/Date/LocalDateTime/ZonedDateTime} — ten legs — plus the
//     SupportBeanXXX bean-timestamp tail reading a.get('month').
//   - invalid (ExprDTInvalid): eleven tryInvalidCompile probes pin the Java
//     message prefixes (startsWith). Three probes have nearest Go
//     rejections (the unknown set-field, string between-operands and
//     invalid format-pattern paths all flow through ErrorInvalidRule); the
//     remaining eight are unrepresentable on the typed surface and pin
//     prefix-only, per the ExprDTIntervalInvalid precedent.
//
// Instant-valued cells render as epoch-millis numbers and null cells as the
// tagged {"state":"null"} object on both sides (the ExprDTRound instant-
// token convention); boolean includes cells and the listener-invoked count
// records render directly.

const exprDTTail553ID = "expr-dt-tail-553"
const exprDTTail553JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

// exprDTTail553Source is the scenario javaSource pin: the suite directory
// shared by the four source files (the expr-dt-remainder-552 multi-source
// precedent); the file-level list lives in exprDTTail553JavaSources.
const exprDTTail553Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime"

var exprDTTail553JavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTToDateCalMSec.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTDocSamples.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTIntervalOpsCreateSchema.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTInvalid.java",
}

var exprDTTail553JavaRuntimeIDs = []string{
	"java-runtime-96fba8bd8db354b4058e",
	"java-runtime-9d3c9122a833c6235a1d",
	"java-runtime-a3cec43b565226d9cbac",
	"java-runtime-8c90cf3d4a6abbd4848f",
	"java-runtime-59302e42b09f7b2f6818",
}

var exprDTTail553JavaExecutions = []string{
	"ExprToCalendarChain",
	"ExprDTToDateCalMSecValue",
	"ExprDTDocSamples",
	"ExprDTIntervalOpsCreateSchema",
	"ExprDTInvalid",
}

var exprDTTail553Cases = []string{
	"tocalendar-chain",
	"todatecalmsec-value",
	"docsamples",
	"intervalops-createschema",
	"invalid",
}

var exprDTTail553CaseRuntimeIDs = map[string]string{
	"tocalendar-chain":         "java-runtime-96fba8bd8db354b4058e",
	"todatecalmsec-value":      "java-runtime-9d3c9122a833c6235a1d",
	"docsamples":               "java-runtime-a3cec43b565226d9cbac",
	"intervalops-createschema": "java-runtime-8c90cf3d4a6abbd4848f",
	"invalid":                  "java-runtime-59302e42b09f7b2f6818",
}

var exprDTTail553JavaStaticIDs = []string{
	"java-30f73c23a62ab5959453",
	"java-30f73c23a62ab5959453",
	"java-1788456b92fc17823173",
	"java-46150495284a50e52094",
	"java-0147f234d5dc15c47624",
}

var exprDTTail553JavaFlags = []string{}

var exprDTTail553Ordinals = []int{0, 1, 0, 0, 0}

const exprDTTail553Description = "ExprDTToDateCalMSec/ExprDTDocSamples/ExprDTIntervalOpsCreateSchema/" +
	"ExprDTInvalid executions: tocalendar-chain replays ExprToCalendarChain " +
	"(current_timestamp.toCalendar().add(DAY_OF_MONTH,1) is a Calendar method call, not a " +
	"date-time chain op, and Java observes null), todatecalmsec-value replays " +
	"ExprDTToDateCalMSecValue (the 18-column toDate/toCalendar/toMillisec matrix pins " +
	"Date x6, Calendar x6, Long x6 types and, for the null bean, only the three " +
	"current_timestamp cells), docsamples replays ExprDTDocSamples (15 compile-only " +
	"doc selects over RFIDEvent.timeTaken then four pattern after() probes pinned by " +
	"their listener-invoked flag), intervalops-createschema replays " +
	"ExprDTIntervalOpsCreateSchema (map + object-array create-schema TypeA/TypeB " +
	"startts/endts timestamps over five field types asserting a.includes(b) true, " +
	"then the SupportBeanXXX bean-timestamp tail asserting a.get('month')==4; the " +
	"Avro/JSON/JSON-provided legs are unrepresentable and DEFAULT legs are map-equivalent duplicates), invalid replays " +
	"ExprDTInvalid's 11 tryInvalidCompile probes with pinned Java message prefixes."

var exprDTTail553CaseObservations = []string{
	"listener; advanceTime(0), deploy s0 the toCalendar().add() chain select, " +
		"send SupportBean('E1',0) emits the null c column (Calendar.add is a " +
		"method call returning void, not a date-time chain op)",
	"deployed+types+listener x2; the unnamed warmup deploy of the same " +
		"toCalendar().add() chain emits a deployed marker, advanceTime(" +
		"2002-05-30T09:00:00.000), deploy s0 the 18-column toDate/toCalendar/" +
		"toMillisec select, types pins Date x6 + Calendar x6 + Long x6, send " +
		"make(t) emits the instant x18, send make(null) emits only the three " +
		"current_timestamp cells",
	"deployed x15+count x4; fifteen compile-only doc selects over " +
		"RFIDEvent.timeTaken each emit a deployed marker, then four pattern " +
		"[a=A -> b=B] after() probes send A then B and emit the " +
		"listener-invoked count (true, true, true, then false reversed)",
	"deployed x10+listener x11; ten map/object-array legs deploy the " +
		"create-schema TypeA/TypeB timestamp module and send A then B, the " +
		"second send emits a.includes(b)=true (the Avro/JSON legs are " +
		"excluded as unrepresentable and DEFAULT legs as map-equivalent), then the bean-tail deploy emits " +
		"a.get('month')==4 for the SupportBeanXXX longPrimitive timestamp",
	"compile-error; 11 tryInvalidCompile probes record the pinned Java " +
		"message prefixes (startsWith assertion); the Go runner verifies the " +
		"nearest expressible rejection for the three representable probes and " +
		"pins prefix-only for the unrepresentable rest",
}

// exprDTTail553ChainEPL is the byte-exact ExprToCalendarChain s0 statement:
// the chained Calendar.add call yields null in Java.
const exprDTTail553ChainEPL = "@name('s0') select current_timestamp.toCalendar().add(Calendar.DAY_OF_MONTH,1) as c from SupportBean"

// exprDTTail553WarmupEPL is the byte-exact ExprDTToDateCalMSecValue warmup
// deployment (no @name, no listener).
const exprDTTail553WarmupEPL = "select current_timestamp.toCalendar().add(Calendar.DAY_OF_MONTH,1) from SupportBean"

// exprDTTail553ValueEPL is the byte-exact ExprDTToDateCalMSecValue s0
// statement: the eighteen-column conversion matrix.
const exprDTTail553ValueEPL = "@name('s0') select " +
	"current_timestamp.toDate() as val0," +
	"utildate.toDate() as val1," +
	"longdate.toDate() as val2," +
	"caldate.toDate() as val3," +
	"localdate.toDate() as val4," +
	"zoneddate.toDate() as val5," +
	"current_timestamp.toCalendar() as val6," +
	"utildate.toCalendar() as val7," +
	"longdate.toCalendar() as val8," +
	"caldate.toCalendar() as val9," +
	"localdate.toCalendar() as val10," +
	"zoneddate.toCalendar() as val11," +
	"current_timestamp.toMillisec() as val12," +
	"utildate.toMillisec() as val13," +
	"longdate.toMillisec() as val14," +
	"caldate.toMillisec() as val15," +
	"localdate.toMillisec() as val16," +
	"zoneddate.toMillisec() as val17" +
	" from SupportDateTime"

// exprDTTail553DocSelect pins one ExprDTDocSamples compileDeploy statement:
// the label, the byte-exact EPL (no @name, no listener, never sent events).
type exprDTTail553DocSelect struct {
	Label string
	EPL   string
}

var exprDTTail553DocSelects = []exprDTTail553DocSelect{
	{"doc-format", "select timeTaken.format() as timeTakenStr from RFIDEvent"},
	{"doc-get-month", "select timeTaken.get('month') as timeTakenMonth from RFIDEvent"},
	{"doc-get-month-of-year", "select timeTaken.getMonthOfYear() as timeTakenMonth from RFIDEvent"},
	{"doc-minus-minutes", "select timeTaken.minus(2 minutes) as timeTakenMinus2Min from RFIDEvent"},
	{"doc-minus-millis", "select timeTaken.minus(2*60*1000) as timeTakenMinus2Min from RFIDEvent"},
	{"doc-plus-minutes", "select timeTaken.plus(2 minutes) as timeTakenMinus2Min from RFIDEvent"},
	{"doc-plus-millis", "select timeTaken.plus(2*60*1000) as timeTakenMinus2Min from RFIDEvent"},
	{"doc-round-ceiling", "select timeTaken.roundCeiling('min') as timeTakenRounded from RFIDEvent"},
	{"doc-round-floor", "select timeTaken.roundFloor('min') as timeTakenRounded from RFIDEvent"},
	{"doc-set-month", "select timeTaken.set('month', 3) as timeTakenMonth from RFIDEvent"},
	{"doc-with-date", "select timeTaken.withDate(2002, 4, 30) as timeTakenDated from RFIDEvent"},
	{"doc-with-max", "select timeTaken.withMax('sec') as timeTakenMaxSec from RFIDEvent"},
	{"doc-to-calendar", "select timeTaken.toCalendar() as timeTakenCal from RFIDEvent"},
	{"doc-to-date", "select timeTaken.toDate() as timeTakenDate from RFIDEvent"},
	{"doc-to-millisec", "select timeTaken.toMillisec() as timeTakenLong from RFIDEvent"},
}

// exprDTTail553PatternProbe pins one ExprDTDocSamples tryRun probe: the
// where-clause condition, the A and B instants, and the asserted
// listener-invoked flag. The A/B event types declare only a start
// timestamp, so every after() form collapses to the point comparison
// longdateStart(A) > longdateStart(B).
type exprDTTail553PatternProbe struct {
	Condition string
	StartA    string
	StartB    string
	Invoked   bool
}

var exprDTTail553PatternProbes = []exprDTTail553PatternProbe{
	{"a.longdateStart.after(b)", "2002-05-30T09:00:00.000Z", "2002-05-30T08:59:59.999Z", true},
	{"a.after(b.longdateStart)", "2002-05-30T09:00:00.000Z", "2002-05-30T08:59:59.999Z", true},
	{"a.after(b)", "2002-05-30T09:00:00.000Z", "2002-05-30T08:59:59.999Z", true},
	{"a.after(b)", "2002-05-30T08:59:59.999Z", "2002-05-30T09:00:00.000Z", false},
}

// exprDTTail553PatternEPL renders the byte-exact tryRun statement for one
// condition.
func exprDTTail553PatternEPL(condition string) string {
	return "@name('s0') select * from pattern [a=A -> b=B] as abc where " + condition
}

// exprDTTail553LegReps are the two event representations the pinned legs
// cover; the Avro/JSON/JSON-provided legs are unrepresentable on the Go
// surface and excluded (the declared intentionally-different set).
var exprDTTail553LegReps = []string{"map", "objectarray"}

// exprDTTail553LegTypes are the five startts/endts property types: the
// epoch-millis long plus the four Java date-time classes, which collapse to
// time.Time on the Go side.
var exprDTTail553LegTypes = []string{"msec", "calendar", "date", "localdatetime", "zoneddatetime"}

// exprDTTail553LegJavaType renders the create-schema property type for one
// leg type name.
func exprDTTail553LegJavaType(legType string) string {
	switch legType {
	case "msec":
		return "long"
	case "calendar":
		return "java.util.Calendar"
	case "date":
		return "java.util.Date"
	case "localdatetime":
		return "java.time.LocalDateTime"
	case "zoneddatetime":
		return "java.time.ZonedDateTime"
	}
	return ""
}

// exprDTTail553LegEPL renders the byte-exact per-leg module EPL: the
// representation annotation, the two create-schema declarations with
// startts/endts timestamps and the s0 includes select, newlines preserved.
func exprDTTail553LegEPL(rep, legType string) string {
	annotation := "@EventRepresentation('objectarray') "
	if rep == "map" {
		annotation = "@EventRepresentation('map') "
	}
	javaType := exprDTTail553LegJavaType(legType)
	return annotation + "@buseventtype @public create schema TypeA as (startts " + javaType + ", endts " + javaType + ") starttimestamp startts endtimestamp endts;\n" +
		annotation + "@buseventtype @public create schema TypeB as (startts " + javaType + ", endts " + javaType + ") starttimestamp startts endtimestamp endts;\n" +
		"@name('s0') select a.includes(b) as val0 from TypeA#lastevent as a, TypeB#lastevent as b;\n"
}

// exprDTTail553BeanTailEPL is the byte-exact bean-timestamp module: the
// SupportBean-inheriting schema declares longPrimitive/longBoxed as its
// timestamps and s0 reads the event-datetime get('month') — May 2002 is
// month 4 on the Calendar 0-based scale.
const exprDTTail553BeanTailEPL = "@public @buseventtype create schema SupportBeanXXX as " +
	"com.espertech.esper.common.internal.support.SupportBean starttimestamp longPrimitive endtimestamp longBoxed;\n" +
	"@name('s0') select a.get('month') as val0 from SupportBeanXXX a;\n"

// exprDTTail553Deploy pins one deploy step: the statement label, the
// byte-exact EPL, whether the deployment's only observable is the compile
// itself (the "deployed" record), and whether the select's deliveries are
// recorded as listener rows or counted as the pattern probe's invoked flag.
type exprDTTail553Deploy struct {
	Statement string
	EPL       string
	Deployed  bool
	Listener  string // "none", "rows" or "pattern"
}

// exprDTTail553Deploys pins each case's deployments in step order.
var exprDTTail553Deploys = func() map[string][]exprDTTail553Deploy {
	deploys := map[string][]exprDTTail553Deploy{
		"tocalendar-chain": {
			{Statement: "s0", EPL: exprDTTail553ChainEPL, Listener: "rows"},
		},
		"todatecalmsec-value": {
			{Statement: "warmup", EPL: exprDTTail553WarmupEPL, Deployed: true},
			{Statement: "s0", EPL: exprDTTail553ValueEPL, Listener: "rows"},
		},
		"docsamples":               {},
		"intervalops-createschema": {},
	}
	for _, select_ := range exprDTTail553DocSelects {
		deploys["docsamples"] = append(deploys["docsamples"],
			exprDTTail553Deploy{Statement: select_.Label, EPL: select_.EPL, Deployed: true})
	}
	for _, probe := range exprDTTail553PatternProbes {
		deploys["docsamples"] = append(deploys["docsamples"],
			exprDTTail553Deploy{Statement: "s0", EPL: exprDTTail553PatternEPL(probe.Condition), Listener: "pattern"})
	}
	for _, rep := range exprDTTail553LegReps {
		for _, legType := range exprDTTail553LegTypes {
			deploys["intervalops-createschema"] = append(deploys["intervalops-createschema"],
				exprDTTail553Deploy{
					Statement: "schema-" + rep + "-" + legType,
					EPL:       exprDTTail553LegEPL(rep, legType),
					Deployed:  true,
					Listener:  "rows",
				})
		}
	}
	deploys["intervalops-createschema"] = append(deploys["intervalops-createschema"],
		exprDTTail553Deploy{Statement: "bean-tail", EPL: exprDTTail553BeanTailEPL, Deployed: true, Listener: "rows"})
	return deploys
}()

// exprDTTail553Send pins one send step: the Java event type (the
// create-schema legs address TypeA/TypeB by their declared names; the Go
// side maps them to the leg's registered names), the event fields the
// payload carries, and the assertions the send resolves — an expected
// output row, or the pattern probe's listener-invoked flag.
type exprDTTail553Send struct {
	EventType     string
	TheString     string
	IntPrimitive  int64
	LongPrimitive int64
	Date          string // ISO instant; "" is the make(null) bean
	Duration      int64  // interval legs: endts = startts + duration millis
	Expected      []any  // pinned output row cells; nil means no row expected
	Invoked       *bool  // pattern probes: the asserted listener-invoked flag
}

// exprDTTail553T00 is DateTime.parseDefaultMSec("2002-05-30T09:00:00.000"),
// the populated instant every case shares.
const exprDTTail553T00 = "2002-05-30T09:00:00.000Z"
const exprDTTail553T00Millis = int64(1022749200000)

// exprDTTail553Epoch is the epoch instant ExprToCalendarChain's
// advanceTime(0) pins.
const exprDTTail553Epoch = "1970-01-01T00:00:00Z"

// exprDTTail553Sends pins each case's send sequence in step order.
var exprDTTail553Sends = func() map[string][]exprDTTail553Send {
	t00Row := make([]any, 18)
	for index := range t00Row {
		t00Row[index] = exprDTTail553T00Millis
	}
	nullRow := make([]any, 18)
	nullRow[0], nullRow[6], nullRow[12] = exprDTTail553T00Millis, exprDTTail553T00Millis, exprDTTail553T00Millis
	sends := map[string][]exprDTTail553Send{
		"tocalendar-chain": {
			{EventType: "SupportBean", TheString: "E1", Expected: []any{nil}},
		},
		"todatecalmsec-value": {
			{EventType: "SupportDateTime", Date: exprDTTail553T00, Expected: t00Row},
			{EventType: "SupportDateTime", Date: "", Expected: nullRow},
		},
		"docsamples": {},
	}
	for _, probe := range exprDTTail553PatternProbes {
		invoked := probe.Invoked
		sends["docsamples"] = append(sends["docsamples"],
			exprDTTail553Send{EventType: "A", Date: probe.StartA},
			exprDTTail553Send{EventType: "B", Date: probe.StartB, Invoked: &invoked})
	}
	interval := []exprDTTail553Send{}
	for range exprDTTail553LegReps {
		for range exprDTTail553LegTypes {
			interval = append(interval,
				exprDTTail553Send{EventType: "TypeA", Date: "2002-05-30T09:00:00.000Z", Duration: 1000},
				exprDTTail553Send{EventType: "TypeB", Date: "2002-05-30T09:00:00.500Z", Duration: 200,
					Expected: []any{true}})
		}
	}
	interval = append(interval, exprDTTail553Send{
		EventType: "SupportBeanXXX", LongPrimitive: exprDTTail553T00Millis, Expected: []any{int64(4)}})
	sends["intervalops-createschema"] = interval
	return sends
}()

// exprDTTail553TypeProperties pins the Java-asserted property types for
// todatecalmsec-value's single types step (Date x6, Calendar x6, Long x6);
// Go verifies the collapsed {time.Time x12, int64 x6} schema.
var exprDTTail553TypeProperties = map[string]string{
	"val0": "Date", "val1": "Date", "val2": "Date", "val3": "Date", "val4": "Date", "val5": "Date",
	"val6": "Calendar", "val7": "Calendar", "val8": "Calendar", "val9": "Calendar", "val10": "Calendar", "val11": "Calendar",
	"val12": "Long", "val13": "Long", "val14": "Long", "val15": "Long", "val16": "Long", "val17": "Long",
}

// exprDTTail553Probe pins one ExprDTInvalid tryInvalidCompile probe: the
// byte-exact EPL, the startsWith prefix Java asserts, and the nearest Go
// rejection. A nil go_ means the probe is unrepresentable on the typed
// surface and only the pinned prefix is recorded.
type exprDTTail553Probe struct {
	label  string
	epl    string
	expect string
	go_    func(env *esper.Environment) error
	goSub  string
}

// exprDTTail553Probes transcribes ExprDTInvalid's eleven probes verbatim.
// The contained/window(*) collection receivers, the set() arity/lambda/
// numeric footprints, the formatter-object mismatches and the boolean
// endpoint flag are all unspellable on the typed Go surface; the
// representable probes verify the nearest ErrorInvalidRule rejection.
var exprDTTail553Probes = []exprDTTail553Probe{
	{
		label: "set-contained-collection",
		epl:   "select contained.set('hour', 1) from SupportBean_ST0_Container",
		expect: "Failed to validate select-clause expression 'contained.set(\"hour\",1)': " +
			"Date-time enumeration method 'set' requires either a Calendar, Date, long, " +
			"LocalDateTime or ZonedDateTime value as input or events of an event type that " +
			"declares a timestamp property but received collection of events of type " +
			"'com.espertech.esper.regressionlib.support.bean.SupportBean_ST0'",
		// Unrepresentable: no collection-of-events contained receiver exists.
	},
	{
		label: "set-window-collection",
		epl:   "select window(*).set('hour', 1) from SupportBean#keepall",
		expect: "Failed to validate select-clause expression 'window(*).set(\"hour\",1)': " +
			"Date-time enumeration method 'set' requires either a Calendar, Date, long, " +
			"LocalDateTime or ZonedDateTime value as input or events of an event type that " +
			"declares a timestamp property but received collection of events of type 'SupportBean'",
		// Unrepresentable: no window(*) collection receiver exists.
	},
	{
		label: "set-invalid-field",
		epl:   "select utildate.set('invalid') from SupportDateTime",
		expect: "Failed to validate select-clause expression 'utildate.set('invalid')': " +
			"Failed to resolve enumeration method, date-time method or mapped property " +
			"'utildate.set('invalid')': Parameters mismatch for date-time method 'set', " +
			"the method requires an expression providing a string-type calendar field name " +
			"and an expression providing an integer-type value",
		go_: func(env *esper.Environment) error {
			_, err := env.Build(esper.FromAny(env, "SupportDateTime").Select(
				esper.Alias("c0", esper.DateTimeSet[time.Time](
					esper.Field[map[string]any, time.Time]("utildate"), "invalid", 1)),
			).Query(esper.StatementName("s0")))
			return err
		},
		goSub: "unknown date-time calendar field",
	},
	{
		label: "set-lambda-param",
		epl:   "select utildate.set(x => true) from SupportDateTime",
		expect: "Failed to validate select-clause expression 'utildate.set()': " +
			"Parameters mismatch for date-time method 'set', the method requires an " +
			"expression providing a string-type calendar field name and an expression " +
			"providing an integer-type value",
		// Unrepresentable: DateTimeSet takes (value, field, n); a lambda
		// argument does not type-check.
	},
	{
		label: "set-no-params",
		epl:   "select utildate.set() from SupportDateTime",
		expect: "Failed to validate select-clause expression 'utildate.set()': " +
			"Parameters mismatch for date-time method 'set', the method requires an " +
			"expression providing a string-type calendar field name and an expression " +
			"providing an integer-type value",
		// Unrepresentable: the zero-parameter footprint does not type-check.
	},
	{
		label: "set-numeric-param",
		epl:   "select utildate.set(1) from SupportDateTime",
		expect: "Failed to validate select-clause expression 'utildate.set(1)': " +
			"Parameters mismatch for date-time method 'set', the method requires an " +
			"expression providing a string-type calendar field name and an expression " +
			"providing an integer-type value",
		// Unrepresentable: the numeric-only footprint does not type-check.
	},
	{
		label: "between-string-bounds",
		epl:   "select utildate.between('a', 'b') from SupportDateTime",
		expect: "Failed to validate select-clause expression 'utildate.between(\"a\",\"b\")': " +
			"Failed to validate date-time method 'between', expected a long-typed, " +
			"Date-typed or Calendar-typed result for expression parameter 0 but received String",
		go_: func(env *esper.Environment) error {
			_, err := env.Build(esper.FromAny(env, "SupportDateTime").Select(
				esper.Alias("c0", esper.DateTimeBetween(
					esper.Field[map[string]any, time.Time]("utildate"),
					esper.Literal("a"), esper.Literal("b"))),
			).Query(esper.StatementName("s0")))
			return err
		},
		goSub: "epoch milliseconds or time.Time",
	},
	{
		label: "between-int-endpoint",
		epl:   "select utildate.between(utildate, utildate, 1, true) from SupportDateTime",
		expect: "Failed to validate select-clause expression 'utildate.between(utildate,utildate,...(42 chars)': " +
			"Failed to validate date-time method 'between', expected a boolean-type result " +
			"for expression parameter 2 but received int",
		// Unrepresentable: the endpoint flags are typed Expression[bool]; an
		// int flag does not type-check.
	},
	{
		label: "format-formatter-param",
		epl:   "select utildate.format(java.time.format.DateTimeFormatter.ISO_ORDINAL_DATE) from SupportDateTime",
		expect: "Failed to validate select-clause expression 'utildate.format(ParseCaseSensitive(...(114 chars)': " +
			"Date-time enumeration method 'format' invalid format, expected string-format " +
			"or DateFormat but received java.time.format.DateTimeFormatter",
		// Unrepresentable: no DateTimeFormatter-object format overload exists.
	},
	{
		label: "format-dateformat-param",
		epl:   "select zoneddate.format(SimpleDateFormat.getInstance()) from SupportDateTime",
		expect: "Failed to validate select-clause expression 'zoneddate.format(SimpleDateFormat.g...(48 chars)': " +
			"Date-time enumeration method 'format' invalid format, expected string-format " +
			"or DateTimeFormatter but received java.text.DateFormat",
		// Unrepresentable: no DateFormat-object format overload exists.
	},
	{
		label: "format-null-param",
		epl:   "select utildate.format(null) from SupportDateTime",
		expect: "Failed to validate select-clause expression 'utildate.format(null)': " +
			"Failed to validate date-time method 'format', expected a non-null result " +
			"for expression parameter 0 but received a null-typed expression",
		go_: func(env *esper.Environment) error {
			_, err := env.Build(esper.FromAny(env, "SupportDateTime").Select(
				esper.Alias("c0", esper.DateTimeFormatPattern[time.Time](
					esper.Field[map[string]any, time.Time]("utildate"), "'")),
			).Query(esper.StatementName("s0")))
			return err
		},
		goSub: "invalid date-time format pattern",
	},
}

// exprDTTail553ProbeByLabel resolves one build-error label to its pin.
func exprDTTail553ProbeByLabel(label string) (exprDTTail553Probe, bool) {
	for _, probe := range exprDTTail553Probes {
		if probe.label == label {
			return probe, true
		}
	}
	return exprDTTail553Probe{}, false
}

// exprDTTail553CaseEPLs pins each case's representative EPL for the case
// metadata.
var exprDTTail553CaseEPLs = []string{
	exprDTTail553ChainEPL,
	exprDTTail553ValueEPL,
	exprDTTail553PatternEPL(exprDTTail553PatternProbes[0].Condition),
	exprDTTail553LegEPL("map", "msec"),
	exprDTTail553Probes[0].epl,
}

// exprDTTail553SupportBean mirrors the SupportBean properties the
// tocalendar-chain send carries.
type exprDTTail553SupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int64  `esper:"intPrimitive"`
}

// exprDTTail553SupportBeanXXX mirrors the SupportBeanXXX create-schema
// bean: longPrimitive/longBoxed are the declared start/end timestamp
// properties the event-datetime get('month') resolves.
type exprDTTail553SupportBeanXXX struct {
	LongPrimitive int64 `esper:"longPrimitive,start"`
	LongBoxed     int64 `esper:"longBoxed,end"`
}

type exprDTTail553CaseState struct {
	caseName    string
	env         *esper.Environment
	engine      *esper.Engine
	trace       *compat.Trace
	sequence    uint64
	deployIndex int
	sendIndex   int
	typesIndex  int
	deployments []*esper.Deployment
	plan        esper.Plan
	legRep      string
	legType     string
	deliveries  int
	row         compat.ResultRecord
}

func runExprDTTail553Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range exprDTTail553Cases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runExprDTTail553Case(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprDTTail553ID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprDTTail553ID, scenario.ID)
	}
	return trace, nil
}

func runExprDTTail553Case(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if err := registerExprDTTail553Types(env, caseName); err != nil {
		return compat.Trace{}, err
	}
	state := &exprDTTail553CaseState{
		caseName: caseName,
		env:      env,
		engine: esper.NewEngine(env,
			esper.WithStartTime(time.Unix(0, 0).UTC()),
			esper.WithRuntimeURI(exprDTTail553CaseRuntimeIDs[caseName])),
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
			if err := state.advanceTime(ctx, step); err != nil {
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
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *state.trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *state.trace, err
			}
		default:
			return *state.trace, fmt.Errorf("%s: unsupported step op %q", exprDTTail553ID, step.Op)
		}
	}
	return *state.trace, nil
}

// registerExprDTTail553Types mirrors the TestSuiteExprDateTime
// registrations each case compiles against: SupportBean for
// tocalendar-chain, SupportBean + SupportDateTime for todatecalmsec-value,
// the RFIDEvent map type and the start-timestamp A/B types for docsamples,
// the per-leg TypeA/TypeB create-schema types plus the SupportBeanXXX bean
// for intervalops-createschema, and SupportDateTime + SupportBean for the
// invalid probes.
func registerExprDTTail553Types(env *esper.Environment, caseName string) error {
	switch caseName {
	case "tocalendar-chain":
		_, err := esper.RegisterStruct[exprDTTail553SupportBean](env, "SupportBean")
		return err
	case "todatecalmsec-value":
		if _, err := esper.RegisterStruct[exprDTTail553SupportBean](env, "SupportBean"); err != nil {
			return err
		}
		_, err := esper.RegisterMap(env, "SupportDateTime", exprDTTail553DateTimeFieldSpecs())
		return err
	case "docsamples":
		if _, err := esper.RegisterMap(env, "RFIDEvent", []esper.FieldSpec{
			esper.FieldDef("timeTaken", reflect.TypeOf(time.Time{})),
		}); err != nil {
			return err
		}
		for _, name := range []string{"A", "B"} {
			if _, err := esper.RegisterMap(env, name, []esper.FieldSpec{
				{Name: "longdateStart", Type: reflect.TypeOf(int64(0)), StartTimestamp: true},
				esper.FieldDef("longdateEnd", reflect.TypeOf(int64(0))),
			}); err != nil {
				return err
			}
		}
		return nil
	case "intervalops-createschema":
		for _, rep := range exprDTTail553LegReps {
			for _, legType := range exprDTTail553LegTypes {
				fields := exprDTTail553LegFieldSpecs(legType)
				for _, prefix := range []string{"TypeA", "TypeB"} {
					name := exprDTTail553LegTypeName(prefix, rep, legType)
					var err error
					if rep == "objectarray" {
						_, err = esper.RegisterObjectArray(env, name, fields, esper.BusEventType())
					} else {
						_, err = esper.RegisterMap(env, name, fields, esper.BusEventType())
					}
					if err != nil {
						return err
					}
				}
			}
		}
		_, err := esper.RegisterStruct[exprDTTail553SupportBeanXXX](env, "SupportBeanXXX", esper.BusEventType())
		return err
	case "invalid":
		if _, err := esper.RegisterMap(env, "SupportDateTime", exprDTTail553DateTimeFieldSpecs()); err != nil {
			return err
		}
		_, err := esper.RegisterStruct[exprDTTail553SupportBean](env, "SupportBean")
		return err
	default:
		return fmt.Errorf("%s: unsupported case %q", exprDTTail553ID, caseName)
	}
}

// exprDTTail553LegTypeName renders the Go-registered event type name for
// one create-schema leg; the pinned steps address the schema-declared
// TypeA/TypeB names and the runner maps them through the current leg.
func exprDTTail553LegTypeName(prefix, rep, legType string) string {
	return prefix + "_" + rep + "_" + legType
}

// exprDTTail553LegFieldSpecs mirrors one leg's create-schema declaration:
// startts/endts declared with the leg's property type (int64 for msec,
// time.Time for the collapsed date-time classes) and flagged as the
// start/end timestamps.
func exprDTTail553LegFieldSpecs(legType string) []esper.FieldSpec {
	valueType := reflect.TypeOf(time.Time{})
	if legType == "msec" {
		valueType = reflect.TypeOf(int64(0))
	}
	return []esper.FieldSpec{
		{Name: "startts", Type: valueType, StartTimestamp: true},
		{Name: "endts", Type: valueType, EndTimestamp: true},
	}
}

// exprDTTail553DateTimeFieldSpecs mirrors SupportDateTime: longdate is the
// epoch-millis Long and the other four representations collapse to
// time.Time.
func exprDTTail553DateTimeFieldSpecs() []esper.FieldSpec {
	return []esper.FieldSpec{
		esper.FieldDef("longdate", reflect.TypeOf(int64(0))),
		esper.FieldDef("utildate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("caldate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("localdate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("zoneddate", reflect.TypeOf(time.Time{})),
	}
}

// advanceTime mirrors env.advanceTime: tocalendar-chain pins the epoch and
// todatecalmsec-value pins T00.
func (s *exprDTTail553CaseState) advanceTime(ctx context.Context, step compat.Step) error {
	pinned := map[string]string{
		"tocalendar-chain":    exprDTTail553Epoch,
		"todatecalmsec-value": exprDTTail553T00,
	}[s.caseName]
	if step.At != pinned {
		return fmt.Errorf("%s: case %q advance-time %q is not pinned", exprDTTail553ID, s.caseName, step.At)
	}
	at, err := time.Parse(time.RFC3339Nano, step.At)
	if err != nil {
		return fmt.Errorf("%s: parse advance-time %q: %w", exprDTTail553ID, step.At, err)
	}
	return s.engine.AdvanceTime(ctx, at)
}

// deploy mirrors one compileDeploy (+ addListener for s0/pattern selects)
// cycle; the pinned EPL for this deploy index is verified before the fluent
// equivalent is built. Compile-only deployments (the warmup and the fifteen
// doc selects) emit the "deployed" record; create-schema legs emit it for
// the schema declarations alongside the s0 deployment.
func (s *exprDTTail553CaseState) deploy(ctx context.Context, step compat.Step) error {
	pinned, ok := exprDTTail553Deploys[s.caseName]
	if !ok || s.deployIndex >= len(pinned) {
		return fmt.Errorf("%s: case %q has no pinned deploy %d", exprDTTail553ID, s.caseName, s.deployIndex)
	}
	pin := pinned[s.deployIndex]
	s.deployIndex++
	if step.Statement != pin.Statement || step.Epl != pin.EPL {
		return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q",
			exprDTTail553ID, s.caseName, step.Statement, step.Epl)
	}
	if s.caseName == "intervalops-createschema" {
		if err := s.trackLeg(pin.Statement); err != nil {
			return err
		}
	}
	query, err := s.buildSelect(pin)
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
	s.deployments = append(s.deployments, deployment)
	if pin.Listener == "rows" {
		s.plan = plan
	}
	s.deliveries = 0
	if pin.Listener != "rows" && pin.Listener != "pattern" {
		if pin.Deployed {
			s.emitDeployed(step)
		}
		return nil
	}
	for _, statement := range deployment.Statements() {
		captured := statement
		listener := pin.Listener
		if _, err := captured.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			newRows := compat.NormalizeResults(batch.New)
			exprDTTail553RenderMillis(newRows)
			if len(newRows) == 0 {
				return nil
			}
			s.deliveries++
			if listener == "pattern" {
				return nil
			}
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
	if pin.Deployed {
		s.emitDeployed(step)
	}
	return nil
}

// emitDeployed appends the "deployed" marker record for a compile-observed
// deployment.
func (s *exprDTTail553CaseState) emitDeployed(step compat.Step) {
	s.sequence++
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "deployed",
		Statement: step.Statement,
		Sequence:  s.sequence,
		Time:      compat.FormatTraceTime(s.engine.Now()),
	})
}

// trackLeg parses the create-schema leg label so the leg's sends resolve
// the Go-registered TypeA/TypeB names and property types.
func (s *exprDTTail553CaseState) trackLeg(statement string) error {
	if statement == "bean-tail" {
		s.legRep, s.legType = "", ""
		return nil
	}
	var rep, legType string
	for _, candidateRep := range exprDTTail553LegReps {
		for _, candidateType := range exprDTTail553LegTypes {
			if statement == "schema-"+candidateRep+"-"+candidateType {
				rep, legType = candidateRep, candidateType
			}
		}
	}
	if rep == "" || legType == "" {
		return fmt.Errorf("%s: malformed leg label %q", exprDTTail553ID, statement)
	}
	s.legRep, s.legType = rep, legType
	return nil
}

// buildSelect renders the fluent equivalent of the pinned deploy:
//
//   - tocalendar-chain: the Calendar.add chain is a Java method call with
//     no date-time chain op; the column is null in Java, so Go projects
//     NullLiteral (the intentionally-different pinned mapping).
//   - todatecalmsec-value: DateTimeToTime covers the twelve toDate/
//     toCalendar columns (Date and Calendar collapse to time.Time) and
//     DateTimeToMillis covers the six toMillisec columns.
//   - docsamples: the fifteen doc selects map to DateTimeFormatDefault,
//     DateTimeGet, DateTimeGetMonthOfYear, DateTimeMinus/PlusDuration over
//     DurationMinutes, DateTimeMinus/PlusExpr over the folded 2*60*1000
//     product, DateTimeRoundCeiling/Floor, DateTimeSet, DateTimeWithDate
//     (Java's 0-based month 4 is May, Go's 1-based argument is 5),
//     DateTimeWithMax, DateTimeToTime and DateTimeToMillis; the pattern
//     probes build PatternFrom(A) -> PatternFrom(B) with the after()
//     where-clause — the A/B types declare only a start timestamp, so all
//     three spellings collapse to the point comparison.
//   - intervalops-createschema: each leg joins the Go-registered TypeA/TypeB
//     over #lastevent and selects Interval(Includes); the bean tail reads
//     DateTimeGet('month') over the longPrimitive timestamp.
func (s *exprDTTail553CaseState) buildSelect(pin exprDTTail553Deploy) (esper.Query, error) {
	utildate := esper.Field[map[string]any, time.Time]("utildate")
	longdate := esper.Field[map[string]any, int64]("longdate")
	caldate := esper.Field[map[string]any, time.Time]("caldate")
	localdate := esper.Field[map[string]any, time.Time]("localdate")
	zoneddate := esper.Field[map[string]any, time.Time]("zoneddate")
	switch s.caseName {
	case "tocalendar-chain":
		return esper.Select(esper.From[exprDTTail553SupportBean](s.env, "SupportBean"),
			esper.Alias("c", esper.NullLiteral[time.Time]())).Query(esper.StatementName("s0")), nil
	case "todatecalmsec-value":
		if pin.Statement != "s0" {
			return esper.Select(esper.From[exprDTTail553SupportBean](s.env, "SupportBean"),
				esper.Alias("c", esper.NullLiteral[time.Time]())).Query(esper.StatementName("warmup")), nil
		}
		return esper.FromAny(s.env, "SupportDateTime").Select(
			esper.Alias("val0", esper.DateTimeToTime[int64](esper.CurrentTimestamp())),
			esper.Alias("val1", esper.DateTimeToTime[time.Time](utildate)),
			esper.Alias("val2", esper.DateTimeToTime[int64](longdate)),
			esper.Alias("val3", esper.DateTimeToTime[time.Time](caldate)),
			esper.Alias("val4", esper.DateTimeToTime[time.Time](localdate)),
			esper.Alias("val5", esper.DateTimeToTime[time.Time](zoneddate)),
			esper.Alias("val6", esper.DateTimeToTime[int64](esper.CurrentTimestamp())),
			esper.Alias("val7", esper.DateTimeToTime[time.Time](utildate)),
			esper.Alias("val8", esper.DateTimeToTime[int64](longdate)),
			esper.Alias("val9", esper.DateTimeToTime[time.Time](caldate)),
			esper.Alias("val10", esper.DateTimeToTime[time.Time](localdate)),
			esper.Alias("val11", esper.DateTimeToTime[time.Time](zoneddate)),
			esper.Alias("val12", esper.DateTimeToMillis[int64](esper.CurrentTimestamp())),
			esper.Alias("val13", esper.DateTimeToMillis[time.Time](utildate)),
			esper.Alias("val14", esper.DateTimeToMillis[int64](longdate)),
			esper.Alias("val15", esper.DateTimeToMillis[time.Time](caldate)),
			esper.Alias("val16", esper.DateTimeToMillis[time.Time](localdate)),
			esper.Alias("val17", esper.DateTimeToMillis[time.Time](zoneddate)),
		).Query(esper.StatementName("s0")), nil
	case "docsamples":
		return s.buildDocSample(pin)
	case "intervalops-createschema":
		return s.buildIntervalOps(pin)
	default:
		return esper.Query{}, fmt.Errorf("%s: case %q deploys no statements", exprDTTail553ID, s.caseName)
	}
}

// buildDocSample renders one doc-select or pattern-probe deployment.
func (s *exprDTTail553CaseState) buildDocSample(pin exprDTTail553Deploy) (esper.Query, error) {
	timeTaken := esper.Field[map[string]any, time.Time]("timeTaken")
	twoMinutes := esper.DurationMinutes[int64](esper.Literal[int64](2))
	twoMinutesMillis := esper.Multiply[int64](
		esper.Multiply[int64](esper.Literal[int64](2), esper.Literal[int64](60)),
		esper.Literal[int64](1000))
	stream := esper.FromAny(s.env, "RFIDEvent")
	select_ := func(name string, expr esper.Expr) esper.Query {
		return stream.Select(esper.Alias(name, expr)).Query(esper.StatementName(pin.Statement))
	}
	switch pin.Statement {
	case "doc-format":
		return select_("timeTakenStr", esper.DateTimeFormatDefault[time.Time](timeTaken)), nil
	case "doc-get-month":
		return select_("timeTakenMonth", esper.DateTimeGet[time.Time](timeTaken, "month")), nil
	case "doc-get-month-of-year":
		return select_("timeTakenMonth", esper.DateTimeGetMonthOfYear[time.Time](timeTaken)), nil
	case "doc-minus-minutes":
		return select_("timeTakenMinus2Min", esper.DateTimeMinusDuration[time.Time](timeTaken, twoMinutes)), nil
	case "doc-minus-millis":
		return select_("timeTakenMinus2Min", esper.DateTimeMinusExpr[time.Time](timeTaken, twoMinutesMillis)), nil
	case "doc-plus-minutes":
		return select_("timeTakenMinus2Min", esper.DateTimePlusDuration[time.Time](timeTaken, twoMinutes)), nil
	case "doc-plus-millis":
		return select_("timeTakenMinus2Min", esper.DateTimePlusExpr[time.Time](timeTaken, twoMinutesMillis)), nil
	case "doc-round-ceiling":
		return select_("timeTakenRounded", esper.DateTimeRoundCeiling[time.Time](timeTaken, "min")), nil
	case "doc-round-floor":
		return select_("timeTakenRounded", esper.DateTimeRoundFloor[time.Time](timeTaken, "min")), nil
	case "doc-set-month":
		// Java set('month',3) is Calendar.MONTH 3 = April; Go's month
		// argument is 1-based, so April is 4.
		return select_("timeTakenMonth", esper.DateTimeSet[time.Time](timeTaken, "month", 4)), nil
	case "doc-with-date":
		return select_("timeTakenDated", esper.DateTimeWithDate[time.Time](timeTaken, 2002, 5, 30)), nil
	case "doc-with-max":
		return select_("timeTakenMaxSec", esper.DateTimeWithMax[time.Time](timeTaken, "sec")), nil
	case "doc-to-calendar":
		return select_("timeTakenCal", esper.DateTimeToTime[time.Time](timeTaken)), nil
	case "doc-to-date":
		return select_("timeTakenDate", esper.DateTimeToTime[time.Time](timeTaken)), nil
	case "doc-to-millisec":
		return select_("timeTakenLong", esper.DateTimeToMillis[time.Time](timeTaken)), nil
	}
	if pin.Statement != "s0" || pin.Listener != "pattern" {
		return esper.Query{}, fmt.Errorf("%s: docsamples has no pinned statement %q",
			exprDTTail553ID, pin.Statement)
	}
	probe := exprDTTail553PatternProbes[s.patternProbeOrdinal()]
	aStart := esper.TagField[int64]("a", "longdateStart")
	bStart := esper.TagField[int64]("b", "longdateStart")
	var condition esper.Expression[bool]
	switch probe.Condition {
	case "a.longdateStart.after(b)", "a.after(b.longdateStart)":
		// Point vs timestamped-event and timestamped-event vs point: the
		// A/B types declare a start timestamp only, so both spellings
		// evaluate startA > startB.
		condition = esper.DateTimeAfter(aStart, bStart)
	case "a.after(b)":
		// Event vs event: both sides are start-timestamp points.
		boundsA := esper.IntervalBounds{Start: aStart, End: aStart}
		boundsB := esper.IntervalBounds{Start: bStart, End: bStart}
		condition = esper.Interval(esper.After, boundsA, boundsB)
	default:
		return esper.Query{}, fmt.Errorf("%s: unsupported pattern condition %q",
			exprDTTail553ID, probe.Condition)
	}
	pattern := esper.PatternFromRecord(esper.FromAny(s.env, "A"), "a", esper.Literal(true)).
		Then(esper.PatternFromRecord(esper.FromAny(s.env, "B"), "b", esper.Literal(true)))
	return pattern.Select(
		esper.Alias("a", esper.PatternEvent("a")),
		esper.Alias("b", esper.PatternEvent("b")),
	).Where(condition).Query(esper.StatementName("s0")), nil
}

// patternProbeOrdinal resolves which tryRun probe an s0 pattern deploy is:
// the count of pattern deployments already built for this case.
func (s *exprDTTail553CaseState) patternProbeOrdinal() int {
	ordinal := 0
	for index := range s.deployIndex - 1 {
		if exprDTTail553Deploys[s.caseName][index].Listener == "pattern" {
			ordinal++
		}
	}
	return ordinal
}

// buildIntervalOps renders one leg's includes join or the bean-tail
// get('month') select.
func (s *exprDTTail553CaseState) buildIntervalOps(pin exprDTTail553Deploy) (esper.Query, error) {
	if pin.Statement == "bean-tail" {
		return esper.FromAny(s.env, "SupportBeanXXX").Select(
			esper.Alias("val0", esper.DateTimeGet[int64](
				esper.Field[any, int64]("longPrimitive"), "month")),
		).Query(esper.StatementName("s0")), nil
	}
	var left, right esper.IntervalBounds
	if s.legType == "msec" {
		left = esper.IntervalBounds{
			Start: esper.JoinField[int64](0, "startts"), End: esper.JoinField[int64](0, "endts")}
		right = esper.IntervalBounds{
			Start: esper.JoinField[int64](1, "startts"), End: esper.JoinField[int64](1, "endts")}
	} else {
		left = esper.IntervalBounds{
			Start: esper.UnixMillis(esper.JoinField[time.Time](0, "startts")),
			End:   esper.UnixMillis(esper.JoinField[time.Time](0, "endts"))}
		right = esper.IntervalBounds{
			Start: esper.UnixMillis(esper.JoinField[time.Time](1, "startts")),
			End:   esper.UnixMillis(esper.JoinField[time.Time](1, "endts"))}
	}
	return esper.JoinMany(
		esper.JoinRecordSource(esper.FromAny(s.env, exprDTTail553LegTypeName("TypeA", s.legRep, s.legType))).Window(esper.LengthWindow(1)),
		esper.JoinRecordSource(esper.FromAny(s.env, exprDTTail553LegTypeName("TypeB", s.legRep, s.legType))).Window(esper.LengthWindow(1)),
	).Select(
		esper.SelectFrom(0, "val0", esper.Interval(esper.Includes, left, right)),
	).Query(esper.StatementName("s0")), nil
}

// types emits the pinned property-type record after verifying the deployed
// statement's collapsed Go schema: the twelve toDate/toCalendar columns are
// time.Time-typed (Java's Date and Calendar collapse) and the six
// toMillisec columns are int64-typed (Java's Long).
func (s *exprDTTail553CaseState) types(step compat.Step) error {
	if s.caseName != "todatecalmsec-value" || step.Statement != "s0" || s.typesIndex != 0 {
		return fmt.Errorf("%s: case %q has no types assertion for %q",
			exprDTTail553ID, s.caseName, step.Statement)
	}
	s.typesIndex++
	schema, ok := s.plan.ResultSchema()
	if !ok {
		return fmt.Errorf("%s: statement %q has no result schema", exprDTTail553ID, step.Statement)
	}
	value := map[string]any{}
	for index := 0; index < len(exprDTTail553TypeProperties); index++ {
		column := fmt.Sprintf("val%d", index)
		field, exists := schema.Field(column)
		if !exists {
			return fmt.Errorf("%s: s0 is missing column %q", exprDTTail553ID, column)
		}
		want := reflect.TypeOf(time.Time{})
		if exprDTTail553TypeProperties[column] == "Long" {
			want = reflect.TypeOf(int64(0))
		}
		if field.Type != want {
			return fmt.Errorf("%s: s0 %s type drift: %v", exprDTTail553ID, column, field.Type)
		}
		value[column] = exprDTTail553TypeProperties[column]
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

// send decodes the payload, delivers the event the pinned Java send mirrors
// and verifies the resulting observation: the pinned output row for
// select-driven cases, or the listener-invoked count for pattern probes.
func (s *exprDTTail553CaseState) send(ctx context.Context, step compat.Step) error {
	pinned, ok := exprDTTail553Sends[s.caseName]
	if !ok || s.sendIndex >= len(pinned) {
		return fmt.Errorf("%s: case %q has no pinned send %d", exprDTTail553ID, s.caseName, s.sendIndex)
	}
	pin := pinned[s.sendIndex]
	s.sendIndex++
	if step.EventType != pin.EventType {
		return fmt.Errorf("%s: case %q send %d carries unpinned event type %q",
			exprDTTail553ID, s.caseName, s.sendIndex-1, step.EventType)
	}
	var payload struct {
		TheString     string  `json:"theString"`
		IntPrimitive  int64   `json:"intPrimitive"`
		LongPrimitive int64   `json:"longPrimitive"`
		Date          *string `json:"date"`
		Start         string  `json:"start"`
		Duration      int64   `json:"duration"`
		Expected      []any   `json:"expected"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return fmt.Errorf("%s: decode %s payload: %w", exprDTTail553ID, step.EventType, err)
	}
	switch pin.EventType {
	case "SupportBean":
		if payload.TheString != pin.TheString || payload.IntPrimitive != pin.IntPrimitive {
			return fmt.Errorf("%s: SupportBean payload is not pinned", exprDTTail553ID)
		}
		s.deliveries = 0
		if err := s.engine.Send(ctx, "SupportBean",
			exprDTTail553SupportBean{TheString: pin.TheString, IntPrimitive: pin.IntPrimitive}); err != nil {
			return err
		}
	case "SupportDateTime":
		date := ""
		if payload.Date != nil {
			date = *payload.Date
		}
		if date != pin.Date {
			return fmt.Errorf("%s: SupportDateTime date %q is not pinned", exprDTTail553ID, date)
		}
		event := map[string]any{}
		if date != "" {
			parsed, err := time.Parse(time.RFC3339Nano, date)
			if err != nil {
				return fmt.Errorf("%s: parse %s date: %w", exprDTTail553ID, step.EventType, err)
			}
			event = map[string]any{
				"longdate":  parsed.UnixMilli(),
				"utildate":  parsed,
				"caldate":   parsed.Add(-time.Duration(parsed.Nanosecond())),
				"localdate": parsed,
				"zoneddate": parsed,
			}
		}
		s.deliveries = 0
		if err := s.engine.SendRecord(ctx, "SupportDateTime", event); err != nil {
			return err
		}
	case "A", "B":
		date := ""
		if payload.Date != nil {
			date = *payload.Date
		}
		if date != pin.Date {
			return fmt.Errorf("%s: %s date %q is not pinned", exprDTTail553ID, pin.EventType, date)
		}
		parsed, err := time.Parse(time.RFC3339Nano, date)
		if err != nil {
			return fmt.Errorf("%s: parse %s date: %w", exprDTTail553ID, pin.EventType, err)
		}
		millis := parsed.UnixMilli()
		s.deliveries = 0
		if err := s.engine.SendRecord(ctx, pin.EventType,
			map[string]any{"longdateStart": millis, "longdateEnd": millis}); err != nil {
			return err
		}
		if pin.EventType == "B" {
			return s.verifyInvoked(pin, payload.Expected)
		}
		if s.deliveries != 0 {
			return fmt.Errorf("%s: A send unexpectedly fired the pattern", exprDTTail553ID)
		}
		return nil
	case "TypeA", "TypeB":
		if s.legRep == "" {
			return fmt.Errorf("%s: interval send outside a create-schema leg", exprDTTail553ID)
		}
		if payload.Start != pin.Date || payload.Duration != pin.Duration {
			return fmt.Errorf("%s: %s leg payload is not pinned", exprDTTail553ID, pin.EventType)
		}
		start, err := time.Parse(time.RFC3339Nano, pin.Date)
		if err != nil {
			return fmt.Errorf("%s: parse %s start: %w", exprDTTail553ID, pin.EventType, err)
		}
		end := start.Add(time.Duration(pin.Duration) * time.Millisecond)
		eventType := exprDTTail553LegTypeName(pin.EventType, s.legRep, s.legType)
		s.deliveries = 0
		if s.legRep == "objectarray" {
			var values []any
			if s.legType == "msec" {
				values = []any{start.UnixMilli(), end.UnixMilli()}
			} else {
				values = []any{start, end}
			}
			if err := s.engine.SendObjectArray(ctx, eventType, values); err != nil {
				return err
			}
		} else {
			var record map[string]any
			if s.legType == "msec" {
				record = map[string]any{"startts": start.UnixMilli(), "endts": end.UnixMilli()}
			} else {
				record = map[string]any{"startts": start, "endts": end}
			}
			if err := s.engine.SendRecord(ctx, eventType, record); err != nil {
				return err
			}
		}
	case "SupportBeanXXX":
		if payload.LongPrimitive != pin.LongPrimitive {
			return fmt.Errorf("%s: SupportBeanXXX payload is not pinned", exprDTTail553ID)
		}
		s.deliveries = 0
		if err := s.engine.Send(ctx, "SupportBeanXXX",
			exprDTTail553SupportBeanXXX{LongPrimitive: pin.LongPrimitive}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s: unsupported event type %q", exprDTTail553ID, pin.EventType)
	}
	return s.verifyExpected(pin, payload.Expected)
}

// verifyExpected mirrors the send's assertPropsNew/assertEqualsNew: a nil
// pin means no delivery is expected; otherwise exactly one row must arrive
// with the pinned cells.
func (s *exprDTTail553CaseState) verifyExpected(pin exprDTTail553Send, payloadExpected []any) error {
	if pin.Expected == nil {
		if len(payloadExpected) != 0 {
			return fmt.Errorf("%s: send %s carries an unpinned expected row",
				exprDTTail553ID, pin.EventType)
		}
		if s.deliveries != 0 {
			return fmt.Errorf("%s: %s send unexpectedly delivered a row",
				exprDTTail553ID, pin.EventType)
		}
		return nil
	}
	if len(payloadExpected) != len(pin.Expected) {
		return fmt.Errorf("%s: %s send expected %d cells, want %d",
			exprDTTail553ID, pin.EventType, len(payloadExpected), len(pin.Expected))
	}
	for index, cell := range payloadExpected {
		if !exprDTTail553CellEqual(cell, pin.Expected[index]) {
			return fmt.Errorf("%s: %s send expected[%d] = %v is not pinned",
				exprDTTail553ID, pin.EventType, index, cell)
		}
	}
	if s.deliveries != 1 {
		return fmt.Errorf("%s: %s send produced %d listener rows, want 1",
			exprDTTail553ID, pin.EventType, s.deliveries)
	}
	columns := exprDTTail553RowColumns(pin.EventType, s.caseName)
	for index, cell := range pin.Expected {
		actual, ok := s.row.Fields[columns[index]]
		if !ok {
			return fmt.Errorf("%s: listener row is missing column %q", exprDTTail553ID, columns[index])
		}
		if !exprDTTail553CellEqual(cell, actual) {
			return fmt.Errorf("%s: column %q = %v, want %v", exprDTTail553ID, columns[index], actual, cell)
		}
	}
	return nil
}

// exprDTTail553RowColumns renders the asserted column names for one send:
// tocalendar-chain's "c", the bean-tail/leg "val0" and the eighteen-column
// conversion matrix.
func exprDTTail553RowColumns(eventType, caseName string) []string {
	if eventType == "SupportBean" && caseName == "tocalendar-chain" {
		return []string{"c"}
	}
	if eventType == "TypeB" || eventType == "SupportBeanXXX" {
		return []string{"val0"}
	}
	columns := make([]string, 18)
	for index := range columns {
		columns[index] = fmt.Sprintf("val%d", index)
	}
	return columns
}

// verifyInvoked emits the listener-invoked count record for one pattern
// probe B send after verifying the flag, mirroring
// assertListenerInvokedFlag.
func (s *exprDTTail553CaseState) verifyInvoked(pin exprDTTail553Send, expected []any) error {
	if pin.Invoked == nil || len(expected) != 1 {
		return fmt.Errorf("%s: B send is missing the invoked flag", exprDTTail553ID)
	}
	flag, ok := expected[0].(bool)
	if !ok || flag != *pin.Invoked {
		return fmt.Errorf("%s: B send invoked flag %v is not pinned", exprDTTail553ID, expected[0])
	}
	invoked := s.deliveries > 0
	if invoked != *pin.Invoked {
		return fmt.Errorf("%s: pattern invoked = %t, want %t", exprDTTail553ID, invoked, *pin.Invoked)
	}
	count := int64(0)
	if invoked {
		count = 1
	}
	s.sequence++
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "count",
		Statement: "s0",
		Sequence:  s.sequence,
		Time:      compat.FormatTraceTime(s.engine.Now()),
		Name:      "listener-invoked",
		Count:     &count,
	})
	return nil
}

// buildError runs one expected-invalid probe: the pinned EPL and prefix
// are verified, the nearest expressible Go rejection is checked when the
// probe is representable, and the pinned Java prefix is recorded either
// way.
func (s *exprDTTail553CaseState) buildError(step compat.Step) error {
	probe, ok := exprDTTail553ProbeByLabel(step.Statement)
	if !ok || step.Epl != probe.epl || step.ExpectError != probe.expect {
		return fmt.Errorf("%s: build-error probe %q is not pinned", exprDTTail553ID, step.Statement)
	}
	if probe.go_ != nil {
		buildErr := probe.go_(s.env)
		if buildErr == nil {
			return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", exprDTTail553ID, step.Statement)
		}
		if !errors.Is(buildErr, esper.ErrorInvalidRule) ||
			(probe.goSub != "" && !strings.Contains(buildErr.Error(), probe.goSub)) {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", exprDTTail553ID, step.Statement, buildErr)
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

// exprDTTail553CellEqual compares a JSON-decoded expected cell with a
// normalized row value: JSON numbers compare against int64 cells, booleans
// compare directly, and the {"state":"null"} token matches the tagged null
// cell both sides render.
func exprDTTail553CellEqual(expected, actual any) bool {
	if want, ok := expected.(map[string]any); ok {
		if want["state"] == "null" {
			got, isMap := actual.(map[string]any)
			return (isMap && got["state"] == "null") || actual == nil
		}
		return false
	}
	if expected == nil {
		if actual == nil {
			return true
		}
		got, isMap := actual.(map[string]any)
		return isMap && got["state"] == "null"
	}
	if want, ok := expected.(bool); ok {
		got, ok := actual.(bool)
		return ok && got == want
	}
	wantNumber, wantIsNumber := exprDTTail553Number(expected)
	gotNumber, gotIsNumber := exprDTTail553Number(actual)
	return wantIsNumber && gotIsNumber && wantNumber == gotNumber
}

// exprDTTail553Number normalizes a JSON float64 or Go int64 cell for
// numeric equality.
func exprDTTail553Number(value any) (int64, bool) {
	switch number := value.(type) {
	case float64:
		return int64(number), true
	case int64:
		return number, true
	case int:
		return int64(number), true
	default:
		return 0, false
	}
}

// exprDTTail553RenderMillis renders transformed time.Time cells as epoch
// millis, the oracle's instant-token convention for every date-time
// representation.
func exprDTTail553RenderMillis(rows []compat.ResultRecord) {
	for _, row := range rows {
		for name, value := range row.Fields {
			if located, ok := value.(time.Time); ok {
				row.Fields[name] = located.UnixMilli()
			}
		}
	}
}

// undeployAll tears down every live deployment, mirroring the execution's
// env.undeployAll() between probes/legs and at case end.
func (s *exprDTTail553CaseState) undeployAll(ctx context.Context) error {
	for _, deployment := range s.deployments {
		if err := deployment.Undeploy(ctx); err != nil {
			return err
		}
	}
	s.deployments = nil
	s.legRep, s.legType = "", ""
	s.deliveries = 0
	s.row = compat.ResultRecord{}
	return nil
}

// exprDTTail553CaseSteps pins the complete step sequence per case as
// op|case|statement|name|eventType|epl|payload|expectError|at keys so the
// loader asserts the scenario file matches the contract byte-for-byte.
var exprDTTail553CaseSteps = func() map[string][]string {
	steps := map[string][]string{}
	deploy := func(caseName string, pin exprDTTail553Deploy) string {
		return "deploy|" + caseName + "|" + pin.Statement + "|||" + pin.EPL + "|||"
	}
	send := func(caseName string, pin exprDTTail553Send) string {
		payload := exprDTTail553SendPayload(pin)
		return "send|" + caseName + "|||" + pin.EventType + "||" + payload + "||"
	}
	undeploy := func(caseName string) string {
		return "undeploy-all|" + caseName + "|||||||"
	}

	chain := []string{
		"advance-time|tocalendar-chain|||||||" + exprDTTail553Epoch,
		deploy("tocalendar-chain", exprDTTail553Deploys["tocalendar-chain"][0]),
		send("tocalendar-chain", exprDTTail553Sends["tocalendar-chain"][0]),
		undeploy("tocalendar-chain"),
	}
	steps["tocalendar-chain"] = chain

	value := []string{
		deploy("todatecalmsec-value", exprDTTail553Deploys["todatecalmsec-value"][0]),
		"advance-time|todatecalmsec-value|||||||" + exprDTTail553T00,
		deploy("todatecalmsec-value", exprDTTail553Deploys["todatecalmsec-value"][1]),
		"types|todatecalmsec-value|s0||||||",
		send("todatecalmsec-value", exprDTTail553Sends["todatecalmsec-value"][0]),
		send("todatecalmsec-value", exprDTTail553Sends["todatecalmsec-value"][1]),
		undeploy("todatecalmsec-value"),
	}
	steps["todatecalmsec-value"] = value

	doc := []string{}
	docDeploys := exprDTTail553Deploys["docsamples"]
	docSends := exprDTTail553Sends["docsamples"]
	for index := 0; index < 15; index++ {
		doc = append(doc, deploy("docsamples", docDeploys[index]))
	}
	for probe := 0; probe < len(exprDTTail553PatternProbes); probe++ {
		doc = append(doc,
			deploy("docsamples", docDeploys[15+probe]),
			send("docsamples", docSends[2*probe]),
			send("docsamples", docSends[2*probe+1]),
			undeploy("docsamples"))
	}
	steps["docsamples"] = doc

	interval := []string{}
	intervalDeploys := exprDTTail553Deploys["intervalops-createschema"]
	intervalSends := exprDTTail553Sends["intervalops-createschema"]
	legIndex := 0
	for index := 0; index < 10; index++ {
		interval = append(interval,
			deploy("intervalops-createschema", intervalDeploys[index]),
			send("intervalops-createschema", intervalSends[legIndex]),
			send("intervalops-createschema", intervalSends[legIndex+1]),
			undeploy("intervalops-createschema"))
		legIndex += 2
	}
	interval = append(interval,
		deploy("intervalops-createschema", intervalDeploys[10]),
		send("intervalops-createschema", intervalSends[20]),
		undeploy("intervalops-createschema"))
	steps["intervalops-createschema"] = interval

	invalid := []string{}
	for _, probe := range exprDTTail553Probes {
		invalid = append(invalid, "build-error|invalid|"+probe.label+"|||"+probe.epl+"||"+probe.expect+"|")
	}
	invalid = append(invalid, undeploy("invalid"))
	steps["invalid"] = invalid
	return steps
}()

// exprDTTail553SendPayload renders one send's compacted payload: the bean
// fields plus the pinned expected cells (null cells as the tagged
// {"state":"null"} token, booleans and numbers directly) or the pattern
// probe's invoked flag.
func exprDTTail553SendPayload(pin exprDTTail553Send) string {
	fields := ""
	add := func(fragment string) {
		if fields != "" {
			fields += ","
		}
		fields += fragment
	}
	switch pin.EventType {
	case "SupportBean":
		add(fmt.Sprintf("\"theString\":%q,\"intPrimitive\":%d", pin.TheString, pin.IntPrimitive))
	case "SupportDateTime", "A", "B":
		date := "null"
		if pin.Date != "" {
			date = fmt.Sprintf("%q", pin.Date)
		}
		add("\"date\":" + date)
	case "TypeA", "TypeB":
		add(fmt.Sprintf("\"start\":%q,\"duration\":%d", pin.Date, pin.Duration))
	case "SupportBeanXXX":
		add(fmt.Sprintf("\"longPrimitive\":%d", pin.LongPrimitive))
	}
	if pin.Invoked != nil {
		add(fmt.Sprintf("\"expected\":[%t]", *pin.Invoked))
	} else if pin.Expected != nil {
		cells := make([]any, len(pin.Expected))
		for index, cell := range pin.Expected {
			if cell == nil {
				cells[index] = map[string]any{"state": "null"}
			} else {
				cells[index] = cell
			}
		}
		encoded, err := json.Marshal(cells)
		if err != nil {
			panic(err)
		}
		add("\"expected\":" + string(encoded))
	}
	return "{" + fields + "}"
}

// loadExprDTTail553Scenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned metadata and case metadata, and the complete
// pinned step sequence so unknown, duplicated or drifted content fails the
// replay.
func loadExprDTTail553Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprDTTail553ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprDTTail553ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTTail553ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTTail553ID, err)
	}
	if err := requireExprDTTail553Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprDTTail553ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprDTTail553ID ||
		metadata.Description != exprDTTail553Description ||
		metadata.JavaCommit != exprDTTail553JavaCommit ||
		metadata.JavaSource != exprDTTail553Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprDTTail553ID)
	}
	if err := validateExprDTTail553StringArray(root["javaRuntimes"], exprDTTail553JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTTail553StringArray(root["javaNames"], exprDTTail553JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTTail553StringArray(root["javaStaticIds"], exprDTTail553JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTTail553StringArray(root["javaFlags"], exprDTTail553JavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprDTTail553Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", exprDTTail553ID, len(exprDTTail553Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprDTTail553Fields(object,
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
		if definition.Case != exprDTTail553Cases[index] ||
			definition.Ordinal != exprDTTail553Ordinals[index] ||
			definition.RuntimeID != exprDTTail553JavaRuntimeIDs[index] ||
			definition.ExecutionName != exprDTTail553JavaExecutions[index] ||
			definition.Observation != exprDTTail553CaseObservations[index] ||
			definition.EPL != exprDTTail553CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprDTTail553ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", exprDTTail553ID, err)
	}
	offset := 0
	for _, caseName := range exprDTTail553Cases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", exprDTTail553ID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTTail553ID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", exprDTTail553ID, offset, caseName)
		}
		if _, err := exprDTTail553StepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTTail553ID, offset, err)
		}
		offset++
		want, ok := exprDTTail553CaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", exprDTTail553ID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", exprDTTail553ID, caseName)
		}
		for _, pinned := range want {
			key, err := exprDTTail553StepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTTail553ID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", exprDTTail553ID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", exprDTTail553ID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTTail553ID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// exprDTTail553StepKey renders one raw step as its pinned key:
// op|case|statement|name|eventType|epl|payload|expectError|at with the
// payload compacted. Unknown fields on the step object are rejected per op.
func exprDTTail553StepKey(raw json.RawMessage) (string, error) {
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

func requireExprDTTail553Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", exprDTTail553ID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", exprDTTail553ID, name)
		}
	}
	return nil
}

func validateExprDTTail553StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
