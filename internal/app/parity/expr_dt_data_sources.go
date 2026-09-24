package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// expr_dt_data_sources.go replays ExprDTDataSources' four executions against
// the pinned Java oracle:
//
//   - minmax (ord 3, ExprDTDataSourcesMinMax): windowed min/max aggregates
//     and the per-row min(a,b) row function serve as interval endpoints for
//     before(b, 1 second) — delta in [1000, MAX] inclusive — over
//     SupportBean#length(2). Sends (20000,20000) and (19000,20000) emit
//     {false,false} and {true,true}.
//   - all-combinations (ord 2, ExprDTDataSourcesAllCombinations): five
//     deploys, one per SupportDateTime field, each selecting the eleven
//     calendar getters with space-separated aliases. Go collapses the five
//     Java representations to int64/time.Time; the zoneddate/localdate
//     deploys compose the java.time semantics Java observes (1-based
//     getMonthValue and the DayOfWeek enum name) so the emitted rows match
//     the oracle byte-for-byte.
//   - field-w-value (ord 1, ExprDTDataSourcesFieldWValue): advanceTime pins
//     engine time to the event instant; one 16-column select reads the
//     eleven getters off current_timestamp plus gethourOfDay off all five
//     fields. A types step pins the all-Integer property-type assertion.
//   - start-end-ts (ord 0, ExprDTDataSourcesStartEndTS): compile-only.
//     Map and object-array schema inheritance deploys assert the declared
//     startTS/endTS timestamp property names through a types record; the
//     POJO and XML sub-tests are unrepresentable on the typed Go surface
//     and pin the note; three tryInvalidCompile probes pin the Java
//     message prefixes (the schema-level timestamp-name-conflict and
//     timestamp-type rejections have no Go boundary, so the probes are
//     prefix-only).

const exprDTDataSourcesID = "expr-dt-data-sources"
const exprDTDataSourcesJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprDTDataSourcesJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTDataSources.java",
}

var exprDTDataSourcesJavaRuntimeIDs = []string{
	"java-runtime-fb5317848fcedbabf016",
	"java-runtime-e1d8bce93d3743e9137f",
	"java-runtime-0ee5536b7a3d3d75f2eb",
	"java-runtime-ab467628bb158bf8ce6c",
}

var exprDTDataSourcesJavaExecutions = []string{
	"ExprDTDataSourcesMinMax",
	"ExprDTDataSourcesAllCombinations",
	"ExprDTDataSourcesFieldWValue",
	"ExprDTDataSourcesStartEndTS",
}

var exprDTDataSourcesCases = []string{
	"minmax",
	"all-combinations",
	"field-w-value",
	"start-end-ts",
}

var exprDTDataSourcesCaseRuntimeIDs = map[string]string{
	"minmax":           "java-runtime-fb5317848fcedbabf016",
	"all-combinations": "java-runtime-e1d8bce93d3743e9137f",
	"field-w-value":    "java-runtime-0ee5536b7a3d3d75f2eb",
	"start-end-ts":     "java-runtime-ab467628bb158bf8ce6c",
}

const exprDTDataSourcesDescription = "ExprDTDataSources executions: minmax replays " +
	"ExprDTDataSourcesMinMax (windowed min/max and per-row min(a,b) as before(b,1 second) " +
	"interval endpoints over SupportBean#length(2); sends (20000,20000)->{false,false} and " +
	"(19000,20000)->{true,true}), all-combinations replays ExprDTDataSourcesAllCombinations " +
	"(five per-field deploys x eleven getters; zoneddate/localdate pin the java.time " +
	"asymmetries month=5 and DayOfWeek=THURSDAY), field-w-value replays " +
	"ExprDTDataSourcesFieldWValue (advanceTime(2002-05-30T09:01:02.003); 16 Integer-typed " +
	"columns over current_timestamp and the five fields), and start-end-ts replays " +
	"ExprDTDataSourcesStartEndTS compile-only (map/object-array schema inheritance asserts " +
	"startTS/endTS timestamp property names; POJO/XML sub-tests unrepresentable; three " +
	"tryInvalidCompile probes pin the Java prefixes)."

var exprDTDataSourcesJavaStaticIDs = []string{
	"java-4f45a77a1aa86f1e7bbb",
	"java-69706afe80bb8912e34a",
	"java-f0520b6dd47cc7a13a1b",
	"java-b828fb8eddc4facd6116",
}

var exprDTDataSourcesJavaFlags = []string{}

var exprDTDataSourcesOrdinals = []int{3, 2, 1, 0}

var exprDTDataSourcesCaseObservations = []string{
	"listener; deploy s0 over SupportBean#length(2): send (20000,20000) emits " +
		"{c0:false,c1:false} (delta 0 < 1000), send (19000,20000) emits {c0:true,c1:true} " +
		"(delta 1000 inside [1000, MAX] inclusive)",
	"listener; five deploys (utildate,longdate,caldate,zoneddate,localdate) each send " +
		"SupportDateTime.make(2002-05-30T09:01:02.003) with caldate ms reset to 3 and emit " +
		"c0..c10 = {1,4|5,30,5|THURSDAY,150,1,9,3,2,22,2002} — the java.time fields read " +
		"1-based month and the DayOfWeek enum name",
	"listener+types; advanceTime(2002-05-30T09:01:02.003), the types step pins all 16 " +
		"columns Integer-typed, then one SupportDateTime send emits " +
		"{1,4,30,5,150,1,9,3,2,22,2002,9,9,9,9,9}",
	"types+unrepresentable+compile-error; map and object-array schema inheritance deploys " +
		"assert startTS/endTS timestamp property names, the POJO/XML sub-tests pin " +
		"unrepresentable notes, and three tryInvalidCompile probes record the pinned " +
		"Java prefixes",
}

// exprDTDataSourcesMinMaxEPL is the byte-exact ord-3 statement.
const exprDTDataSourcesMinMaxEPL = "@name('s0') select " +
	"min(longPrimitive).before(max(longBoxed), 1 second) as c0," +
	"min(longPrimitive, longBoxed).before(20000L, 1 second) as c1" +
	" from SupportBean#length(2)"

// exprDTDataSourcesFieldWValueEPL is the byte-exact ord-1 statement; the Java
// source carries a double space before 'as valmos'.
const exprDTDataSourcesFieldWValueEPL = "@name('s0') select " +
	"current_timestamp.getMinuteOfHour() as valmoh," +
	"current_timestamp.getMonthOfYear() as valmoy," +
	"current_timestamp.getDayOfMonth() as valdom," +
	"current_timestamp.getDayOfWeek() as valdow," +
	"current_timestamp.getDayOfYear() as valdoy," +
	"current_timestamp.getEra() as valera," +
	"current_timestamp.gethourOfDay() as valhod," +
	"current_timestamp.getmillisOfSecond()  as valmos," +
	"current_timestamp.getsecondOfMinute() as valsom," +
	"current_timestamp.getweekyear() as valwye," +
	"current_timestamp.getyear() as valyea," +
	"utildate.gethourOfDay() as val1," +
	"longdate.gethourOfDay() as val2," +
	"caldate.gethourOfDay() as val3," +
	"zoneddate.gethourOfDay() as val4," +
	"localdate.gethourOfDay() as val5" +
	" from SupportDateTime"

// exprDTDataSourcesSelectEPL is the byte-exact dt.before select every
// start-end-ts sub-test compiles against its event type.
const exprDTDataSourcesSelectEPL = "@name('s0') select * from ChildType dt where dt.before(current_timestamp())"

// exprDTDataSourcesSelectPOJOEPL / exprDTDataSourcesSelectXMLEPL are the
// byte-exact s2 statements for the POJO and XML sub-tests.
const exprDTDataSourcesSelectPOJOEPL = "@name('s2') select * from DerivedType dt where dt.before(current_timestamp())"
const exprDTDataSourcesSelectXMLEPL = "@name('s2') select * from MyXMLEvent dt where dt.before(current_timestamp())"

// exprDTDataSourcesSchemaEPLs are the byte-exact create-schema modules the
// four start-end-ts sub-tests deploy (the Java source concatenates the two
// create statements; the POJO module ends without a trailing semicolon).
const exprDTDataSourcesSchemaMapEPL = "@buseventtype @public create schema ParentType as (startTS long, endTS long) starttimestamp startTS endtimestamp endTS;\n" +
	"@buseventtype @public create schema ChildType as (foo string) inherits ParentType;\n"
const exprDTDataSourcesSchemaObjectArrayEPL = "@buseventtype @public create objectarray schema ParentType as (startTS long, endTS long) starttimestamp startTS endtimestamp endTS;\n" +
	"@buseventtype @public create objectarray schema ChildType as (foo string) inherits ParentType;\n"
const exprDTDataSourcesSchemaPOJOEPL = "@public @buseventtype create schema InterfaceType as com.espertech.esper.regressionlib.support.bean.SupportStartTSEndTSInterface starttimestamp startTS endtimestamp endTS;\n" +
	"@public @buseventtype create schema DerivedType as com.espertech.esper.regressionlib.support.bean.SupportStartTSEndTSImpl inherits InterfaceType"
const exprDTDataSourcesSchemaXMLEPL = "@XMLSchema(rootElementName='root', schemaText='') " +
	"@XMLSchemaField(name='startTS', xpath='/abc', type='string', castToType='long')" +
	"@XMLSchemaField(name='endTS', xpath='/def', type='string', castToType='long')" +
	"@public @buseventtype create xml schema MyXMLEvent() starttimestamp startTS endtimestamp endTS;\n"
const exprDTDataSourcesSchemaIncompatibleEPL = "@public @buseventtype create schema T1 as (startTS long, endTS long) starttimestamp startTS endtimestamp endTS;\n" +
	"@public @buseventtype create schema T2 as (startTSOne long, endTSOne long) starttimestamp startTSOne endtimestamp endTSOne;\n"

// exprDTDataSourcesGetterMethods are the eleven date-time getters the
// all-combinations execution iterates, in select order.
var exprDTDataSourcesGetterMethods = []string{
	"getMinuteOfHour", "getMonthOfYear", "getDayOfMonth", "getDayOfWeek",
	"getDayOfYear", "getEra", "gethourOfDay", "getmillisOfSecond",
	"getsecondOfMinute", "getweekyear", "getyear",
}

// exprDTDataSourcesGetterFields maps each Java getter to the DateTimeGet
// field name the Go surface resolves.
var exprDTDataSourcesGetterFields = []string{
	"minute_of_hour", "month_of_year", "day_of_month", "day_of_week",
	"day_of_year", "era", "hour_of_day", "millis_of_second",
	"second_of_minute", "weekyear", "year",
}

// exprDTDataSourcesDateTimeFields are the five SupportDateTime properties
// the all-combinations execution loops over.
var exprDTDataSourcesDateTimeFields = []string{"utildate", "longdate", "caldate", "zoneddate", "localdate"}

// exprDTDataSourcesAllCombinationsEPL renders the byte-exact per-field
// statement: space-separated c0..c10 aliases without the 'as' keyword.
func exprDTDataSourcesAllCombinationsEPL(field string) string {
	var epl strings.Builder
	epl.WriteString("@name('s0') select ")
	for index, method := range exprDTDataSourcesGetterMethods {
		if index > 0 {
			epl.WriteString(",")
		}
		fmt.Fprintf(&epl, "%s.%s() c%d", field, method, index)
	}
	epl.WriteString(" from SupportDateTime")
	return epl.String()
}

// exprDTDataSourcesDeployEPLs maps every deploy-step statement label to its
// pinned EPL.
var exprDTDataSourcesDeployEPLs = func() map[string]string {
	epls := map[string]string{
		"s0":                  exprDTDataSourcesMinMaxEPL,
		"field-w-value":       exprDTDataSourcesFieldWValueEPL,
		"schema-map":          exprDTDataSourcesSchemaMapEPL,
		"schema-objectarray":  exprDTDataSourcesSchemaObjectArrayEPL,
		"schema-pojo":         exprDTDataSourcesSchemaPOJOEPL,
		"schema-xml":          exprDTDataSourcesSchemaXMLEPL,
		"schema-incompatible": exprDTDataSourcesSchemaIncompatibleEPL,
		"select-map":          exprDTDataSourcesSelectEPL,
		"select-objectarray":  exprDTDataSourcesSelectEPL,
		"select-pojo":         exprDTDataSourcesSelectPOJOEPL,
		"select-xml":          exprDTDataSourcesSelectXMLEPL,
	}
	for _, field := range exprDTDataSourcesDateTimeFields {
		epls[field] = exprDTDataSourcesAllCombinationsEPL(field)
	}
	return epls
}()

// exprDTDataSourcesFieldWValueT is DateTime.parseDefaultMSec("2002-05-30T09:01:02.003").
const exprDTDataSourcesFieldWValueT = "2002-05-30T09:01:02.003Z"

// exprDTDataSourcesSupportBean mirrors the SupportBean properties the
// minmax case sends.
type exprDTDataSourcesSupportBean struct {
	LongPrimitive int64 `esper:"longPrimitive"`
	LongBoxed     int64 `esper:"longBoxed"`
}

// exprDTDataSourcesProbe pins one start-end-ts tryInvalidCompile probe: the
// byte-exact EPL and the startsWith prefix Java asserts. All three probes
// are unrepresentable on the typed Go surface — schema-level timestamp
// property-name conflicts and timestamp-property type checks have no Go
// boundary — so only the pinned prefix is recorded.
type exprDTDataSourcesProbe struct {
	label  string
	epl    string
	expect string
}

var exprDTDataSourcesProbes = []exprDTDataSourcesProbe{
	{
		label:  "inherits-conflict-start",
		epl:    "create schema T12 as () inherits T1,T2",
		expect: "Event type declares start timestamp as property 'startTS' however inherited event type 'T2' declares start timestamp as property 'startTSOne'",
	},
	{
		label:  "inherits-conflict-end",
		epl:    "create schema T12 as (startTSOne long, endTSXXX long) inherits T2 starttimestamp startTSOne endtimestamp endTSXXX",
		expect: "Event type declares end timestamp as property 'endTSXXX' however inherited event type 'T2' declares end timestamp as property 'endTSOne'",
	},
	{
		label:  "null-start-ts-type",
		epl:    "create schema T12 as (startTSOne null, endTSXXX long) starttimestamp startTSOne endtimestamp endTSXXX",
		expect: "Declared start timestamp property 'startTSOne' is expected to return a Date, Calendar or long-typed value but returns 'null'",
	},
}

// exprDTDataSourcesUnrepresentableNotes pin the Java sub-tests with no Go
// boundary: the POJO class-reference create-schema and the XML schema
// variants of the start-end-ts execution.
var exprDTDataSourcesUnrepresentableNotes = map[string]string{
	"pojo-inheritance": "POJO create-schema inheritance: create schema InterfaceType as <interface> starttimestamp startTS endtimestamp endTS with DerivedType inherits InterfaceType compiles and declares startTS/endTS; the typed Go surface has no class-reference schema boundary",
	"xml-inheritance":  "XML create-schema: @XMLSchemaField string xpath properties with castToType='long' accepted as starttimestamp/endtimestamp on create xml schema; the Go XML schema surface has no xpath-cast timestamp boundary",
}

type exprDTDataSourcesCaseState struct {
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

func runExprDTDataSourcesScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range exprDTDataSourcesCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runExprDTDataSourcesCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprDTDataSourcesID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprDTDataSourcesID, scenario.ID)
	}
	return trace, nil
}

func runExprDTDataSourcesCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	state := &exprDTDataSourcesCaseState{
		caseName: caseName,
		trace:    &compat.Trace{Version: scenario.Version, ID: scenario.ID},
	}
	if err := state.resetEnvironment(); err != nil {
		return compat.Trace{}, err
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
				return *state.trace, fmt.Errorf("%s: parse advance-time %q: %w", exprDTDataSourcesID, step.At, err)
			}
			if err := state.engine.AdvanceTime(ctx, at); err != nil {
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
		case "types":
			if err := state.types(step); err != nil {
				return *state.trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *state.trace, err
			}
		case "unrepresentable":
			if err := state.unrepresentable(step); err != nil {
				return *state.trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *state.trace, err
			}
		default:
			return *state.trace, fmt.Errorf("%s: unsupported step op %q", exprDTDataSourcesID, step.Op)
		}
	}
	return *state.trace, nil
}

// resetEnvironment builds a fresh environment and engine, mirroring the
// Java execution's undeployAll + RegressionPath.clear() cycle: Go schema
// registration is permanent per environment, so each start-end-ts sub-test
// (and every other case) starts on a clean type namespace. The engine is
// pinned to the case's Java runtime id at the epoch start time.
func (s *exprDTDataSourcesCaseState) resetEnvironment() error {
	if s.engine != nil {
		_ = s.engine.Close(context.Background())
	}
	env := esper.NewEnvironment()
	if err := registerExprDTDataSourcesTypes(env, s.caseName); err != nil {
		return err
	}
	s.env = env
	s.engine = esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(exprDTDataSourcesCaseRuntimeIDs[s.caseName]))
	s.deployment = nil
	s.plan = esper.Plan{}
	return nil
}

// registerExprDTDataSourcesTypes mirrors the session registrations each
// case compiles against: SupportBean for minmax, the SupportDateTime map
// type for all-combinations and field-w-value. start-end-ts registers its
// schema types through the deploy steps themselves.
func registerExprDTDataSourcesTypes(env *esper.Environment, caseName string) error {
	switch caseName {
	case "minmax":
		_, err := esper.RegisterStruct[exprDTDataSourcesSupportBean](env, "SupportBean")
		return err
	case "all-combinations", "field-w-value":
		_, err := esper.RegisterMap(env, "SupportDateTime", exprDTDataSourcesDateTimeFieldSpecs())
		return err
	case "start-end-ts":
		return nil
	default:
		return fmt.Errorf("%s: unsupported case %q", exprDTDataSourcesID, caseName)
	}
}

// exprDTDataSourcesDateTimeFieldSpecs mirrors SupportDateTime: longdate is
// the epoch-millis Long and the other four representations collapse to
// time.Time.
func exprDTDataSourcesDateTimeFieldSpecs() []esper.FieldSpec {
	return []esper.FieldSpec{
		esper.FieldDef("longdate", reflect.TypeOf(int64(0))),
		esper.FieldDef("utildate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("caldate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("localdate", reflect.TypeOf(time.Time{})),
		esper.FieldDef("zoneddate", reflect.TypeOf(time.Time{})),
	}
}

// deploy mirrors one compileDeploy cycle; the pinned EPL is verified before
// the fluent equivalent is built. start-end-ts schema deploys register the
// declared event types (the create-schema module's Go counterpart); the
// POJO/XML schema and select deploys are unrepresentable and no-op so the
// pinned unrepresentable record carries the contract.
func (s *exprDTDataSourcesCaseState) deploy(ctx context.Context, step compat.Step) error {
	pinned, ok := exprDTDataSourcesDeployEPLs[step.Statement]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q", exprDTDataSourcesID, step.Statement, step.Epl)
	}
	switch s.caseName {
	case "minmax", "all-combinations", "field-w-value":
		if err := s.undeployAll(ctx); err != nil {
			return err
		}
		query, err := s.buildSelect(step.Statement)
		if err != nil {
			return err
		}
		return s.deployListened(ctx, query)
	case "start-end-ts":
		return s.deployStartEndTS(ctx, step.Statement)
	default:
		return fmt.Errorf("%s: unsupported case %q", exprDTDataSourcesID, s.caseName)
	}
}

// deployListened builds and deploys one select statement, subscribing the
// trace listener to every deployed statement.
func (s *exprDTDataSourcesCaseState) deployListened(ctx context.Context, query esper.Query) error {
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

// buildSelect renders the fluent equivalent of one select deploy for the
// event-producing cases.
func (s *exprDTDataSourcesCaseState) buildSelect(label string) (esper.Query, error) {
	switch s.caseName {
	case "minmax":
		if label != "s0" {
			return esper.Query{}, fmt.Errorf("%s: case %q deploys no statement %q", exprDTDataSourcesID, s.caseName, label)
		}
		return buildExprDTDataSourcesMinMaxQuery(s.env), nil
	case "all-combinations":
		return buildExprDTDataSourcesAllCombinationsQuery(s.env, label)
	case "field-w-value":
		if label != "field-w-value" {
			return esper.Query{}, fmt.Errorf("%s: case %q deploys no statement %q", exprDTDataSourcesID, s.caseName, label)
		}
		return buildExprDTDataSourcesFieldWValueQuery(s.env), nil
	default:
		return esper.Query{}, fmt.Errorf("%s: case %q deploys no statements", exprDTDataSourcesID, s.caseName)
	}
}

// buildExprDTDataSourcesMinMaxQuery renders the ord-3 select: c0 compares
// the windowed min(longPrimitive) interval point against the windowed
// max(longBoxed) point with the inclusive [1000, MAX] before threshold;
// c1 does the same for the per-row min(longPrimitive, longBoxed) against
// the 20000L point.
func buildExprDTDataSourcesMinMaxQuery(env *esper.Environment) esper.Query {
	longPrimitive := esper.Field[exprDTDataSourcesSupportBean, int64]("longPrimitive")
	longBoxed := esper.Field[exprDTDataSourcesSupportBean, int64]("longBoxed")
	minPrimitive := esper.Min[int64](longPrimitive)
	maxBoxed := esper.Max[int64](longBoxed)
	rowMin := esper.MinOf[int64](longPrimitive, longBoxed)
	threshold := esper.BeforeThreshold(1000, math.MaxInt64)
	return esper.From[exprDTDataSourcesSupportBean](env, "SupportBean").Window(esper.LengthWindow(2)).Aggregate(
		esper.Alias("c0", esper.Interval(threshold,
			esper.IntervalBounds{Start: minPrimitive, End: minPrimitive},
			esper.IntervalBounds{Start: maxBoxed, End: maxBoxed})),
		esper.Alias("c1", esper.Interval(threshold,
			esper.IntervalBounds{Start: rowMin, End: rowMin},
			esper.IntervalBounds{Start: esper.Literal(int64(20000)), End: esper.Literal(int64(20000))})),
	).Query(esper.StatementName("s0"))
}

// buildExprDTDataSourcesAllCombinationsQuery renders one per-field deploy:
// the eleven getters over the named SupportDateTime property. The
// zoneddate/localdate deploys compose the java.time semantics Java
// observes — getMonthValue (1-based month) and the DayOfWeek enum name —
// because Go collapses LocalDateTime/ZonedDateTime to time.Time.
func buildExprDTDataSourcesAllCombinationsQuery(env *esper.Environment, field string) (esper.Query, error) {
	java8 := field == "zoneddate" || field == "localdate"
	if field == "longdate" {
		return buildExprDTDataSourcesAllCombinationsTyped[int64](env, field,
			esper.Field[map[string]any, int64](field), java8), nil
	}
	for _, candidate := range exprDTDataSourcesDateTimeFields {
		if candidate == field {
			return buildExprDTDataSourcesAllCombinationsTyped[time.Time](env, field,
				esper.Field[map[string]any, time.Time](field), java8), nil
		}
	}
	return esper.Query{}, fmt.Errorf("%s: case %q deploys no statement %q", exprDTDataSourcesID, "all-combinations", field)
}

// buildExprDTDataSourcesAllCombinationsTyped renders the eleven-column
// select for one field representation. c1/c3 switch on java8: the
// Date/long/Calendar representations read the 0-based Calendar.MONTH and
// the Calendar.DAY_OF_WEEK int, while the java.time representations read
// getMonthValue and the DayOfWeek enum name.
func buildExprDTDataSourcesAllCombinationsTyped[V int64 | time.Time](
	env *esper.Environment, field string, source esper.Expression[V], java8 bool) esper.Query {
	selections := make([]esper.Selection, 0, len(exprDTDataSourcesGetterFields))
	for index, name := range exprDTDataSourcesGetterFields {
		var column esper.Expr
		switch {
		case java8 && index == 1:
			// LocalDateTime/ZonedDateTime.getMonthOfYear() is the 1-based
			// getMonthValue: the collapsed time.Time reads Calendar.MONTH
			// (0-based), so the java.time value adds one.
			column = esper.Func1[V, int64]("getMonthValue", func(value V) int64 {
				return int64(exprDTDataSourcesMonthOfYear(value))
			}, source)
		case java8 && index == 3:
			// LocalDateTime/ZonedDateTime.getDayOfWeek() returns the
			// java.time.DayOfWeek enum, which the trace renders as its
			// name (THURSDAY).
			column = esper.Func1[V, string]("getDayOfWeek", func(value V) string {
				return exprDTDataSourcesDayOfWeekName(value)
			}, source)
		default:
			column = esper.DateTimeGet[V](source, name)
		}
		selections = append(selections, esper.Alias(fmt.Sprintf("c%d", index), column))
	}
	return esper.Select(esper.From[map[string]any](env, "SupportDateTime"), selections...).
		Query(esper.StatementName("s0"))
}

// exprDTDataSourcesMonthOfYear reads the 1-based month a java.time
// representation reports for the collapsed value.
func exprDTDataSourcesMonthOfYear[V int64 | time.Time](value V) int {
	switch typed := any(value).(type) {
	case int64:
		return int(time.UnixMilli(typed).UTC().Month())
	case time.Time:
		return int(typed.Month())
	default:
		return 0
	}
}

// exprDTDataSourcesDayOfWeekName renders the java.time.DayOfWeek enum name
// for the collapsed value (Go's Weekday names match the English enum
// constants once upper-cased).
func exprDTDataSourcesDayOfWeekName[V int64 | time.Time](value V) string {
	var weekday time.Weekday
	switch typed := any(value).(type) {
	case int64:
		weekday = time.UnixMilli(typed).UTC().Weekday()
	case time.Time:
		weekday = typed.Weekday()
	default:
		return ""
	}
	return strings.ToUpper(weekday.String())
}

// buildExprDTDataSourcesFieldWValueQuery renders the ord-1 select: the
// eleven getters over current_timestamp plus gethourOfDay over all five
// SupportDateTime representations.
func buildExprDTDataSourcesFieldWValueQuery(env *esper.Environment) esper.Query {
	now := esper.CurrentTimestamp()
	selections := []esper.Selection{
		esper.Alias("valmoh", esper.DateTimeGet[int64](now, "minute_of_hour")),
		esper.Alias("valmoy", esper.DateTimeGet[int64](now, "month_of_year")),
		esper.Alias("valdom", esper.DateTimeGet[int64](now, "day_of_month")),
		esper.Alias("valdow", esper.DateTimeGet[int64](now, "day_of_week")),
		esper.Alias("valdoy", esper.DateTimeGet[int64](now, "day_of_year")),
		esper.Alias("valera", esper.DateTimeGet[int64](now, "era")),
		esper.Alias("valhod", esper.DateTimeGet[int64](now, "hour_of_day")),
		esper.Alias("valmos", esper.DateTimeGet[int64](now, "millis_of_second")),
		esper.Alias("valsom", esper.DateTimeGet[int64](now, "second_of_minute")),
		esper.Alias("valwye", esper.DateTimeGet[int64](now, "weekyear")),
		esper.Alias("valyea", esper.DateTimeGet[int64](now, "year")),
		esper.Alias("val1", esper.DateTimeGet[time.Time](esper.Field[map[string]any, time.Time]("utildate"), "hour_of_day")),
		esper.Alias("val2", esper.DateTimeGet[int64](esper.Field[map[string]any, int64]("longdate"), "hour_of_day")),
		esper.Alias("val3", esper.DateTimeGet[time.Time](esper.Field[map[string]any, time.Time]("caldate"), "hour_of_day")),
		esper.Alias("val4", esper.DateTimeGet[time.Time](esper.Field[map[string]any, time.Time]("zoneddate"), "hour_of_day")),
		esper.Alias("val5", esper.DateTimeGet[time.Time](esper.Field[map[string]any, time.Time]("localdate"), "hour_of_day")),
	}
	return esper.Select(esper.From[map[string]any](env, "SupportDateTime"), selections...).
		Query(esper.StatementName("s0"))
}

// deployStartEndTS runs one start-end-ts deploy step: schema deploys
// register the declared event types (the create-schema module's Go
// counterpart), select deploys build and deploy the dt.before select, and
// the unrepresentable POJO/XML deploys no-op after the pinned EPL check.
func (s *exprDTDataSourcesCaseState) deployStartEndTS(ctx context.Context, label string) error {
	switch label {
	case "schema-map":
		parent, err := esper.RegisterMap(s.env, "ParentType", exprDTDataSourcesStartEndFields())
		if err != nil {
			return err
		}
		_, err = esper.RegisterMap(s.env, "ChildType",
			[]esper.FieldSpec{esper.FieldDef("foo", reflect.TypeOf(""))},
			esper.WithSchemaParent(parent))
		return err
	case "schema-objectarray":
		parent, err := esper.RegisterObjectArray(s.env, "ParentType", exprDTDataSourcesStartEndFields())
		if err != nil {
			return err
		}
		_, err = esper.RegisterObjectArray(s.env, "ChildType",
			[]esper.FieldSpec{esper.FieldDef("foo", reflect.TypeOf(""))},
			esper.WithSchemaParent(parent))
		return err
	case "schema-pojo", "schema-xml", "select-pojo", "select-xml":
		// Unrepresentable: the typed Go surface has no class-reference or
		// xpath-cast schema boundary; the pinned unrepresentable record
		// carries the contract.
		return nil
	case "schema-incompatible":
		if _, err := esper.RegisterMap(s.env, "T1", exprDTDataSourcesStartEndFields()); err != nil {
			return err
		}
		_, err := esper.RegisterMap(s.env, "T2", []esper.FieldSpec{
			{Name: "startTSOne", Type: reflect.TypeOf(int64(0)), StartTimestamp: true},
			{Name: "endTSOne", Type: reflect.TypeOf(int64(0)), EndTimestamp: true},
		})
		return err
	case "select-map", "select-objectarray":
		// The schema deploy registered ChildType into this environment; the
		// select deploys against it without undeploying (Java's path keeps
		// the create-schema module live).
		now := esper.CurrentTimestamp()
		query := esper.FromAny(s.env, "ChildType").
			Filter(esper.IntervalBefore(
				esper.EventIntervalBounds(0),
				esper.IntervalBounds{Start: now, End: now})).
			Query(esper.StatementName("s0"))
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
		return nil
	default:
		return fmt.Errorf("%s: case %q deploys no statement %q", exprDTDataSourcesID, s.caseName, label)
	}
}

// exprDTDataSourcesStartEndFields declares the startTS/endTS long fields
// flagged as the event type's start/end timestamp properties.
func exprDTDataSourcesStartEndFields() []esper.FieldSpec {
	return []esper.FieldSpec{
		{Name: "startTS", Type: reflect.TypeOf(int64(0)), StartTimestamp: true},
		{Name: "endTS", Type: reflect.TypeOf(int64(0)), EndTimestamp: true},
	}
}

// send decodes the payload, delivers the event, and verifies the delivery
// against the pinned expected values — the assertPropsNew equivalent. Every
// send must produce exactly one listener row whose fields match the pinned
// column values in order.
func (s *exprDTDataSourcesCaseState) send(ctx context.Context, step compat.Step) error {
	expected, err := decodeExprDTDataSourcesExpected(step)
	if err != nil {
		return err
	}
	s.fired = false
	s.row = compat.ResultRecord{}
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			LongPrimitive int64 `json:"longPrimitive"`
			LongBoxed     int64 `json:"longBoxed"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("%s: decode SupportBean: %w", exprDTDataSourcesID, err)
		}
		if err := s.engine.Send(ctx, "SupportBean", exprDTDataSourcesSupportBean{
			LongPrimitive: payload.LongPrimitive,
			LongBoxed:     payload.LongBoxed,
		}); err != nil {
			return err
		}
	case "SupportDateTime":
		var payload struct {
			Date string `json:"date"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("%s: decode SupportDateTime: %w", exprDTDataSourcesID, err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, payload.Date)
		if err != nil {
			return fmt.Errorf("%s: parse SupportDateTime date: %w", exprDTDataSourcesID, err)
		}
		// SupportDateTime.make derives every representation from one
		// instant and zeroes the Calendar millisecond; the
		// all-combinations execution then re-sets cal ms to 3.
		caldate := parsed.Add(-time.Duration(parsed.Nanosecond()))
		if s.caseName == "all-combinations" {
			caldate = parsed
		}
		if err := s.engine.SendRecord(ctx, "SupportDateTime", map[string]any{
			"longdate":  parsed.UnixMilli(),
			"utildate":  parsed,
			"caldate":   caldate,
			"localdate": parsed,
			"zoneddate": parsed,
		}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s: unsupported event type %q", exprDTDataSourcesID, step.EventType)
	}
	if !s.fired {
		return fmt.Errorf("%s: %s send produced no listener row", exprDTDataSourcesID, step.EventType)
	}
	return s.verifyExpected(step, expected)
}

// decodeExprDTDataSourcesExpected reads the pinned expected column values
// every send payload carries, in select order.
func decodeExprDTDataSourcesExpected(step compat.Step) ([]any, error) {
	var payload struct {
		Expected []any `json:"expected"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return nil, fmt.Errorf("%s: decode expected values: %w", exprDTDataSourcesID, err)
	}
	if payload.Expected == nil {
		return nil, fmt.Errorf("%s: %s send is missing the expected values", exprDTDataSourcesID, step.EventType)
	}
	return payload.Expected, nil
}

// verifyExpected compares the delivered row's fields against the pinned
// expected values in select order, mirroring assertPropsNew.
func (s *exprDTDataSourcesCaseState) verifyExpected(step compat.Step, expected []any) error {
	columns, err := exprDTDataSourcesExpectedColumns(s.caseName, step)
	if err != nil {
		return err
	}
	if len(expected) != len(columns) {
		return fmt.Errorf("%s: %s send pins %d expected values, want %d",
			exprDTDataSourcesID, step.EventType, len(expected), len(columns))
	}
	for index, column := range columns {
		actual, ok := s.row.Fields[column]
		if !ok {
			return fmt.Errorf("%s: listener row is missing column %q", exprDTDataSourcesID, column)
		}
		if !exprDTDataSourcesValueEqual(expected[index], actual) {
			return fmt.Errorf("%s: column %q = %v, want %v", exprDTDataSourcesID, column, actual, expected[index])
		}
	}
	return nil
}

// exprDTDataSourcesExpectedColumns returns the select-order column names a
// send's expected values pin.
func exprDTDataSourcesExpectedColumns(caseName string, step compat.Step) ([]string, error) {
	switch caseName {
	case "minmax":
		return []string{"c0", "c1"}, nil
	case "all-combinations":
		columns := make([]string, 0, len(exprDTDataSourcesGetterFields))
		for index := range exprDTDataSourcesGetterFields {
			columns = append(columns, fmt.Sprintf("c%d", index))
		}
		return columns, nil
	case "field-w-value":
		return []string{
			"valmoh", "valmoy", "valdom", "valdow", "valdoy", "valera",
			"valhod", "valmos", "valsom", "valwye", "valyea",
			"val1", "val2", "val3", "val4", "val5",
		}, nil
	default:
		return nil, fmt.Errorf("%s: case %q sends no events", exprDTDataSourcesID, caseName)
	}
}

// exprDTDataSourcesValueEqual compares a JSON-decoded expected cell with
// the normalized row value: numbers compare numerically, strings and
// booleans compare directly, and the {state:null} token matches a nil
// field.
func exprDTDataSourcesValueEqual(expected, actual any) bool {
	switch want := expected.(type) {
	case float64:
		switch got := actual.(type) {
		case int64:
			return want == float64(got)
		case int:
			return want == float64(got)
		case float64:
			return want == got
		}
		return false
	case bool:
		got, ok := actual.(bool)
		return ok && want == got
	case string:
		got, ok := actual.(string)
		return ok && want == got
	case map[string]any:
		if state, ok := want["state"]; ok && state == "null" {
			return actual == nil
		}
		return false
	default:
		return reflect.DeepEqual(expected, actual)
	}
}

// types emits the pinned metadata record after verifying the asserted
// surface: field-w-value checks all 16 result columns are int64-typed (the
// Java Integer assertion), and start-end-ts checks the deployed select's
// event type declares startTS/endTS as the timestamp properties.
func (s *exprDTDataSourcesCaseState) types(step compat.Step) error {
	var value map[string]any
	switch s.caseName {
	case "field-w-value":
		if step.Statement != "s0" {
			return fmt.Errorf("%s: unknown types statement %q", exprDTDataSourcesID, step.Statement)
		}
		schema, ok := s.plan.ResultSchema()
		if !ok {
			return fmt.Errorf("%s: statement %q has no result schema", exprDTDataSourcesID, step.Statement)
		}
		columns, err := exprDTDataSourcesExpectedColumns(s.caseName, step)
		if err != nil {
			return err
		}
		properties := map[string]any{}
		for _, column := range columns {
			field, exists := schema.Field(column)
			if !exists || field.Type != reflect.TypeOf(int64(0)) {
				return fmt.Errorf("%s: s0 %s type drift: %v", exprDTDataSourcesID, column, field.Type)
			}
			properties[column] = "Integer"
		}
		value = map[string]any{"properties": properties}
	case "start-end-ts":
		typeName, ok := map[string]string{
			"select-map":         "ChildType",
			"select-objectarray": "ChildType",
		}[step.Statement]
		if !ok {
			return fmt.Errorf("%s: unknown types statement %q", exprDTDataSourcesID, step.Statement)
		}
		schema, ok := s.env.Schema(typeName)
		if !ok {
			return fmt.Errorf("%s: event type %q is not registered", exprDTDataSourcesID, typeName)
		}
		start, end := "", ""
		for _, field := range schema.Fields() {
			if field.StartTimestamp {
				start = field.Name
			}
			if field.EndTimestamp {
				end = field.Name
			}
		}
		if start != "startTS" || end != "endTS" {
			return fmt.Errorf("%s: %s timestamp properties drift: start=%q end=%q",
				exprDTDataSourcesID, typeName, start, end)
		}
		value = map[string]any{"startTimestamp": start, "endTimestamp": end}
	default:
		return fmt.Errorf("%s: case %q has no types assertions", exprDTDataSourcesID, s.caseName)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "types",
		Statement: step.Statement,
		Time:      compat.FormatTraceTime(s.engine.Now()),
		Value:     value,
	})
	return nil
}

// buildError runs one expected-invalid probe: the pinned EPL and prefix are
// verified, then the pinned Java prefix is recorded. All three start-end-ts
// probes are unrepresentable on the typed Go surface — schema-level
// timestamp property-name conflicts and timestamp-property type checks
// have no Go boundary — so no Go rejection is claimed.
func (s *exprDTDataSourcesCaseState) buildError(step compat.Step) error {
	probe, ok := exprDTDataSourcesProbeByLabel(step.Statement)
	if !ok || step.Epl != probe.epl || step.ExpectError != probe.expect {
		return fmt.Errorf("%s: build-error probe %q is not pinned", exprDTDataSourcesID, step.Statement)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

func exprDTDataSourcesProbeByLabel(label string) (exprDTDataSourcesProbe, bool) {
	for _, probe := range exprDTDataSourcesProbes {
		if probe.label == label {
			return probe, true
		}
	}
	return exprDTDataSourcesProbe{}, false
}

// unrepresentable emits the pinned record for a Java sub-test with no Go
// boundary. The oracle verifies the asserted metadata before emitting the
// same record; the Go side only pins the note.
func (s *exprDTDataSourcesCaseState) unrepresentable(step compat.Step) error {
	note, ok := exprDTDataSourcesUnrepresentableNotes[step.Statement]
	if !ok || step.ExpectError != note {
		return fmt.Errorf("%s: unrepresentable step %q carries an unpinned note %q",
			exprDTDataSourcesID, step.Statement, step.ExpectError)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// undeployAll tears down the live deployment. For start-end-ts it also
// resets the environment and engine, mirroring the Java execution's
// undeployAll + RegressionPath.clear() cycle so each sub-test starts on a
// clean type namespace.
func (s *exprDTDataSourcesCaseState) undeployAll(ctx context.Context) error {
	if s.deployment != nil {
		if err := s.deployment.Undeploy(ctx); err != nil {
			return err
		}
		s.deployment = nil
	}
	if s.caseName == "start-end-ts" {
		return s.resetEnvironment()
	}
	return nil
}

// exprDTDataSourcesCaseSteps pins the complete step sequence per case as
// op|case|statement|eventType|epl|payload|expectError|at keys so the loader
// asserts the scenario file matches the contract byte-for-byte.
var exprDTDataSourcesCaseSteps = func() map[string][]string {
	steps := map[string][]string{}
	steps["minmax"] = []string{
		"deploy|minmax|s0||" + exprDTDataSourcesMinMaxEPL + "|||",
		`send|minmax||SupportBean||{"longPrimitive":20000,"longBoxed":20000,"expected":[false,false]}||`,
		`send|minmax||SupportBean||{"longPrimitive":19000,"longBoxed":20000,"expected":[true,true]}||`,
		"undeploy-all|minmax||||||",
	}
	allCombinations := []string{}
	for _, field := range exprDTDataSourcesDateTimeFields {
		java8 := field == "zoneddate" || field == "localdate"
		month, dow := "4", "5"
		if java8 {
			month, dow = "5", `"THURSDAY"`
		}
		allCombinations = append(allCombinations,
			"deploy|all-combinations|"+field+"||"+exprDTDataSourcesAllCombinationsEPL(field)+"|||",
			fmt.Sprintf(`send|all-combinations||SupportDateTime||{"date":%q,"expected":[1,%s,30,%s,150,1,9,3,2,22,2002]}||`,
				exprDTDataSourcesFieldWValueT, month, dow),
			"undeploy-all|all-combinations||||||",
		)
	}
	steps["all-combinations"] = allCombinations
	steps["field-w-value"] = []string{
		"advance-time|field-w-value||||||" + exprDTDataSourcesFieldWValueT,
		"deploy|field-w-value|field-w-value||" + exprDTDataSourcesFieldWValueEPL + "|||",
		"types|field-w-value|s0|||||",
		fmt.Sprintf(`send|field-w-value||SupportDateTime||{"date":%q,"expected":[1,4,30,5,150,1,9,3,2,22,2002,9,9,9,9,9]}||`,
			exprDTDataSourcesFieldWValueT),
		"undeploy-all|field-w-value||||||",
	}
	steps["start-end-ts"] = []string{
		"deploy|start-end-ts|schema-map||" + exprDTDataSourcesSchemaMapEPL + "|||",
		"deploy|start-end-ts|select-map||" + exprDTDataSourcesSelectEPL + "|||",
		"types|start-end-ts|select-map|||||",
		"undeploy-all|start-end-ts||||||",
		"deploy|start-end-ts|schema-objectarray||" + exprDTDataSourcesSchemaObjectArrayEPL + "|||",
		"deploy|start-end-ts|select-objectarray||" + exprDTDataSourcesSelectEPL + "|||",
		"types|start-end-ts|select-objectarray|||||",
		"undeploy-all|start-end-ts||||||",
		"deploy|start-end-ts|schema-pojo||" + exprDTDataSourcesSchemaPOJOEPL + "|||",
		"deploy|start-end-ts|select-pojo||" + exprDTDataSourcesSelectPOJOEPL + "|||",
		"unrepresentable|start-end-ts|pojo-inheritance||||" + exprDTDataSourcesUnrepresentableNotes["pojo-inheritance"] + "|",
		"undeploy-all|start-end-ts||||||",
		"deploy|start-end-ts|schema-xml||" + exprDTDataSourcesSchemaXMLEPL + "|||",
		"deploy|start-end-ts|select-xml||" + exprDTDataSourcesSelectXMLEPL + "|||",
		"unrepresentable|start-end-ts|xml-inheritance||||" + exprDTDataSourcesUnrepresentableNotes["xml-inheritance"] + "|",
		"undeploy-all|start-end-ts||||||",
		"deploy|start-end-ts|schema-incompatible||" + exprDTDataSourcesSchemaIncompatibleEPL + "|||",
		"build-error|start-end-ts|inherits-conflict-start||" + exprDTDataSourcesProbes[0].epl + "||" + exprDTDataSourcesProbes[0].expect + "|",
		"build-error|start-end-ts|inherits-conflict-end||" + exprDTDataSourcesProbes[1].epl + "||" + exprDTDataSourcesProbes[1].expect + "|",
		"build-error|start-end-ts|null-start-ts-type||" + exprDTDataSourcesProbes[2].epl + "||" + exprDTDataSourcesProbes[2].expect + "|",
		"undeploy-all|start-end-ts||||||",
	}
	return steps
}()

// loadExprDTDataSourcesScenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned metadata and case metadata, and the complete
// pinned step sequence so unknown, duplicated or drifted content fails the
// replay.
func loadExprDTDataSourcesScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprDTDataSourcesID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprDTDataSourcesID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTDataSourcesID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTDataSourcesID, err)
	}
	if err := requireExprDTDataSourcesFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprDTDataSourcesID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprDTDataSourcesID ||
		metadata.Description != exprDTDataSourcesDescription ||
		metadata.JavaCommit != exprDTDataSourcesJavaCommit ||
		metadata.JavaSource != exprDTDataSourcesJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprDTDataSourcesID)
	}
	if err := validateExprDTDataSourcesStringArray(root["javaRuntimes"], exprDTDataSourcesJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTDataSourcesStringArray(root["javaNames"], exprDTDataSourcesJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTDataSourcesStringArray(root["javaStaticIds"], exprDTDataSourcesJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDTDataSourcesStringArray(root["javaFlags"], exprDTDataSourcesJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprDTDataSourcesCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", exprDTDataSourcesID, len(exprDTDataSourcesCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprDTDataSourcesFields(object,
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
		if definition.Case != exprDTDataSourcesCases[index] ||
			definition.Ordinal != exprDTDataSourcesOrdinals[index] ||
			definition.RuntimeID != exprDTDataSourcesJavaRuntimeIDs[index] ||
			definition.ExecutionName != exprDTDataSourcesJavaExecutions[index] ||
			definition.Observation != exprDTDataSourcesCaseObservations[index] ||
			definition.EPL != exprDTDataSourcesCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprDTDataSourcesID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", exprDTDataSourcesID, err)
	}
	offset := 0
	for _, caseName := range exprDTDataSourcesCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", exprDTDataSourcesID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTDataSourcesID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", exprDTDataSourcesID, offset, caseName)
		}
		if _, err := exprDTDataSourcesStepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTDataSourcesID, offset, err)
		}
		offset++
		want, ok := exprDTDataSourcesCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", exprDTDataSourcesID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", exprDTDataSourcesID, caseName)
		}
		for _, pinned := range want {
			key, err := exprDTDataSourcesStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", exprDTDataSourcesID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", exprDTDataSourcesID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", exprDTDataSourcesID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDTDataSourcesID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// exprDTDataSourcesCaseEPLs pins one representative EPL per case for the
// case metadata: the minmax statement, the utildate all-combinations
// statement (first of the five field deploys), the field-w-value
// statement, and the dt.before select shared by the start-end-ts
// sub-tests.
var exprDTDataSourcesCaseEPLs = []string{
	exprDTDataSourcesMinMaxEPL,
	exprDTDataSourcesAllCombinationsEPL("utildate"),
	exprDTDataSourcesFieldWValueEPL,
	exprDTDataSourcesSelectEPL,
}

// exprDTDataSourcesStepKey renders one raw step as its pinned key:
// op|case|statement|eventType|epl|payload|expectError|at with the payload
// compacted. Unknown fields on the step object are rejected per op.
func exprDTDataSourcesStepKey(raw json.RawMessage) (string, error) {
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
		"case":            {"op", "case"},
		"advance-time":    {"op", "case", "at"},
		"deploy":          {"op", "case", "statement", "epl"},
		"send":            {"op", "case", "eventType", "payload"},
		"types":           {"op", "case", "statement"},
		"build-error":     {"op", "case", "statement", "epl", "expectError"},
		"unrepresentable": {"op", "case", "statement", "expectError"},
		"undeploy-all":    {"op", "case"},
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

func requireExprDTDataSourcesFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", exprDTDataSourcesID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", exprDTDataSourcesID, name)
		}
	}
	return nil
}

func validateExprDTDataSourcesStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
