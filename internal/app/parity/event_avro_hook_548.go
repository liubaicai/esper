package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Draft-4.548 runner for the four EventAvroHook executions replayed under
// the TestSuiteEventAvroWConfig session configuration. The hook MECHANISM
// (TypeRepresentationMapper LDT->string schema mapping plus the
// MyLDTTypeWidener/MySupportBeanWidener widen/widenCodegen hooks wired
// through ObjectValueTypeWidenerFactory, and ord-2/3's STATICHOOK
// MySupportBeanWidener.supportBeanSchema static-field wiring) has no Go
// counterpart and is intentionally-different: the runner predeclares the
// three Avro schemas and expresses every widened value as an explicit
// Func1 UDF, keeping the observable records byte-identical.
//
//   - property-coerce (ord 0, EventAvroHookSimpleWriteablePropertyCoerce):
//     the zdt probe is a build-error step (Go Build rejects the time.Time
//     -> string route with a different message kind; the record pins the
//     Java assertMessage prefix verbatim), then the valid ldt insert-into
//     projects Func1(isoDateTime, ldt) and the listener pins the exact
//     ISO_DATE_TIME string captured at scenario build.
//   - schema-from-class (ord 1, EventAvroHookSchemaFromClass):
//     @EventRepresentation('avro') insert into MyEventOut with a UDF that
//     calls now() per event; the trace pins only the asserted schema JSON
//     (value/avro-schema) plus a shape-only listener row whose isodate
//     renders the fixed "<isodate>" marker after the >10 length gate —
//     the wall-clock value is never recorded.
//   - populate (ord 2, EventAvroHookPopulate, STATICHOOK): Func1
//     (makeSupportBean) materializes the SupportBeanSchema record; the
//     listener row renders flat (required field) and value/avroToJson pins
//     the byte-exact record JSON.
//   - named-window-property-assignment (ord 3,
//     EventAvroHookNamedWindowPropertyAssignment, STATICHOOK): keepall
//     window over the union-field MyEventWSchema; an empty record (sb null)
//     seeds the window, the SupportBean trigger assigns sb through Func1
//     (widenInput), and the snapshot row keeps the Avro union encoding —
//     sb renders {"SupportBeanSchema":{...}} (never flattened) — while
//     value/avroToJson pins the byte-exact record JSON.
//
// Avro rows render in the avroToJson shape on both sides: declared fields
// marshal scalars, nested records flatten for required fields and keep the
// union branch name for union fields, and a null value is {state:null}.
const eventAvroHook548JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
const eventAvroHook548ID = "event-avro-hook-548"
const eventAvroHook548Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/avro/EventAvroHook.java"
const eventAvroHook548Description = "EventAvroHook hook slice (all 4 executions, ord 2-3 STATICHOOK): property-coerce pins the invalid zdt->isodate insert-into compile probe and the valid ldt insert-into asserting the captured ISO_DATE_TIME string; schema-from-class pins the MyEventOut avro schema JSON byte-exact plus a shape-only now() listener row; populate inserts a SupportBeanSchema record into MyEventPopulate(sb) pinning the flat avroToJson; named-window-property-assignment updates a keepall window's union sb through a SupportBean trigger pinning the double-nested SupportBeanSchema union encoding. The TypeRepresentationMapper/ObjectValueTypeWidenerFactory codegen-hook mechanism is intentionally-different (explicit Func1 UDFs + predeclared schemas)."

var eventAvroHook548JavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/avro/EventAvroHook.java",
}

var eventAvroHook548JavaRuntimeIDs = []string{
	"java-runtime-34ec4dc8abe4f2443930",
	"java-runtime-c80f13d8b1a53d4ff458",
	"java-runtime-20190c427227521c092b",
	"java-runtime-85c9c44d4baae883febe",
}

var eventAvroHook548JavaExecutions = []string{
	"EventAvroHookSimpleWriteablePropertyCoerce",
	"EventAvroHookSchemaFromClass",
	"EventAvroHookPopulate",
	"EventAvroHookNamedWindowPropertyAssignment",
}

var eventAvroHook548JavaStaticIDs = []string{
	"java-281ee8b5adc379b62b2c",
	"java-281ee8b5adc379b62b2c",
	"java-281ee8b5adc379b62b2c",
	"java-281ee8b5adc379b62b2c",
}

var eventAvroHook548JavaFlags = []string{"STATICHOOK"}

var eventAvroHook548Cases = []string{
	"property-coerce",
	"schema-from-class",
	"populate",
	"named-window-property-assignment",
}

var eventAvroHook548CaseOrdinals = []int{0, 1, 2, 3}

var eventAvroHook548CaseObservations = []string{
	"compile-error+deployed+listener",
	"deployed+value+listener",
	"deployed+listener+value",
	"deployed+snapshot+value",
}

var eventAvroHook548CaseFlags = [][]string{
	{},
	{},
	{"STATICHOOK"},
	{"STATICHOOK"},
}

// Byte-exact EPLs pinned from the Java regression source, including the
// missing space after @EventRepresentation('avro').
const eah548CoerceInvalidEPL = "insert into MyEvent(isodate) select zdt from SupportEventWithZonedDateTime"
const eah548CoerceEPL = "@name('s0') insert into MyEvent(isodate) select ldt from SupportEventWithLocalDateTime"
const eah548SchemaEPL = "@name('s0') @public @EventRepresentation('avro')insert into MyEventOut select com.espertech.esper.regressionlib.suite.event.avro.EventAvroHook.makeLocalDateTime() as isodate from SupportBean as e1"
const eah548PopulateEPL = "@name('s0') insert into MyEventPopulate(sb) select com.espertech.esper.regressionlib.suite.event.avro.EventAvroHook.makeSupportBean() from SupportBean_S0 as e1"
const eah548WindowCreateEPL = "@Name('NamedWindow') @public create window MyWindow#keepall as MyEventWSchema"
const eah548WindowInsertEPL = "insert into MyWindow select * from MyEventWSchema"
const eah548WindowUpdateEPL = "on SupportBean thebean update MyWindow set sb = thebean"

var eventAvroHook548CaseEPLs = map[string]string{
	"property-coerce":                  eah548CoerceEPL,
	"schema-from-class":                eah548SchemaEPL,
	"populate":                         eah548PopulateEPL,
	"named-window-property-assignment": eah548WindowCreateEPL + "\n" + eah548WindowInsertEPL + "\n" + eah548WindowUpdateEPL,
}

// eah548DeployEPLs maps each deploy-step label to its pinned EPL.
var eah548DeployEPLs = map[string]map[string]string{
	"property-coerce":   {"s0": eah548CoerceEPL},
	"schema-from-class": {"s0": eah548SchemaEPL},
	"populate":          {"s0": eah548PopulateEPL},
	"named-window-property-assignment": {
		"NamedWindow":   eah548WindowCreateEPL,
		"insert-window": eah548WindowInsertEPL,
		"update-window": eah548WindowUpdateEPL,
	},
}

// The pinned Java message the tryInvalidCompile probe asserts
// (assertMessage is a startsWith check); the Go side verifies only that
// its typed Build boundary also rejects the zdt->isodate assignment.
const eah548CoerceProbeMessage = "Invalid assignment of column 'isodate' of type 'java.time.ZonedDateTime' to event property 'isodate' typed as 'java.lang.CharSequence', column and parameter types mismatch"

// eah548SchemaJSON pins MyEventOut's runtime Avro schema text.
const eah548SchemaJSON = `{"type":"record","name":"MyEventOut","fields":[{"name":"isodate","type":"string"}]}`

// eah548PopulateJSON pins the required-field (flat) record encoding.
const eah548PopulateJSON = `{"sb":{"theString":"E1","intPrimitive":10}}`

// eah548WindowJSON pins the union-field (double-nested) record encoding;
// the "SupportBeanSchema" branch name MUST NOT be flattened.
const eah548WindowJSON = `{"sb":{"SupportBeanSchema":{"theString":"E1","intPrimitive":10}}}`

// eah548LDT is the LocalDateTime the scenario captures pre-send (ord 0
// freezes the wall-clock value Java computes at send time).
const eah548LDT = "2026-09-27T01:02:03.123456789"

// eah548IsodateMarker renders ord-1's wall-clock isodate: the Java
// makeLocalDateTime() UDF calls LocalDateTime.now() per event, so only the
// asserted shape (length > 10) is pinned and the value is never recorded.
const eah548IsodateMarker = "<isodate>"

// eah548UnionBranch names the union branch schema for the named-window
// case's sb field; populated fields look it up to preserve the Avro union
// encoding that avroToJson produces.
var eah548UnionBranch = map[string]string{
	"named-window-property-assignment": "SupportBeanSchema",
}

// Bean mirrors for the registered event types. SupportBean carries only
// the two properties the scenarios exercise; ZDT/LDT carry their single
// time field.
type eah548SupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type eah548SupportBeanS0 struct {
	ID int `esper:"id"`
}

type eah548LDTBean struct {
	Ldt time.Time `esper:"ldt"`
}

type eah548ZDTBean struct {
	Zdt time.Time `esper:"zdt"`
}

// eah548CaseState accumulates one case's records; sequences follow the
// parity convention (per-statement, per-operation counters starting at
// one).
type eah548CaseState struct {
	caseName string
	trace    *compat.Trace
	seq      map[string]uint64
	now      string
	// expectedIsodate pins ord-0's asserted isodate value for the listener
	// assertion; lastAvro captures the last delivered *AvroRecord for the
	// pinned avroToJson value steps.
	expectedIsodate string
	lastAvro        *esper.AvroRecord
}

func newEAH548CaseState(caseName string, trace *compat.Trace) *eah548CaseState {
	return &eah548CaseState{
		caseName: caseName,
		trace:    trace,
		seq:      map[string]uint64{},
		now:      compat.FormatTraceTime(time.Unix(0, 0).UTC()),
	}
}

func (s *eah548CaseState) emit(operation, statement string, newRows []compat.ResultRecord, name string, value any) {
	key := statement + ":" + operation
	s.seq[key]++
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: operation,
		Statement: statement,
		Sequence:  s.seq[key],
		Time:      s.now,
		New:       newRows,
		Name:      name,
		Value:     value,
	})
}

func (s *eah548CaseState) emitDeployed(statement string) {
	s.emit("deployed", statement, nil, "", nil)
}

func (s *eah548CaseState) emitValue(statement, name string, value any) {
	s.emit("value", statement, nil, name, value)
}

// emitCompileError records the pinned Java tryInvalidCompile text; like the
// EPLOtherInvalid/context-lifecycle precedent it carries no time field.
func (s *eah548CaseState) emitCompileError(statement, expected string) {
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: statement,
		Value:     expected,
	})
}

func (s *eah548CaseState) emitListener(statement string, newRows []compat.ResultRecord) {
	s.emit("listener", statement, newRows, "", nil)
}

// emitSnapshot records the window iteration; the row preserves the union
// field encoding (avroToJson shape).
func (s *eah548CaseState) emitSnapshot(statement string, newRows []compat.ResultRecord) {
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "snapshot",
		Statement: statement,
		Sequence:  0,
		Time:      s.now,
		New:       newRows,
	})
}

// eah548Env carries one case's engine, deployments and avro schemas.
type eah548Env struct {
	env           *esper.Environment
	engine        *esper.Engine
	supportSchema esper.Schema
	windowSchema  esper.Schema
	deployments   map[string]*esper.Deployment
	statements    map[string]*esper.Statement
}

// eah548Register mirrors TestSuiteEventAvroWConfig.configure: the four bean
// types plus the three preconfigured Avro types. The hook classes and their
// static-field schema wiring have no Go counterpart; the equivalent Avro
// schemas are declared explicitly (intentionally-different mechanism, same
// observable types). MyEventWSchema.sb is the union field: OptionalFieldDef
// stands in for unionOf(null, SupportBeanSchema) and the nested schema link
// names the record branch.
func eah548Register(env *esper.Environment, caseName string) (esper.Schema, esper.Schema, error) {
	if _, err := esper.RegisterStruct[eah548SupportBean](env, "SupportBean"); err != nil {
		return esper.Schema{}, esper.Schema{}, err
	}
	if _, err := esper.RegisterStruct[eah548SupportBeanS0](env, "SupportBean_S0"); err != nil {
		return esper.Schema{}, esper.Schema{}, err
	}
	if _, err := esper.RegisterStruct[eah548LDTBean](env, "SupportEventWithLocalDateTime"); err != nil {
		return esper.Schema{}, esper.Schema{}, err
	}
	if _, err := esper.RegisterStruct[eah548ZDTBean](env, "SupportEventWithZonedDateTime"); err != nil {
		return esper.Schema{}, esper.Schema{}, err
	}
	supportSchema, err := esper.NewAvroSchema("SupportBeanSchema", []esper.FieldSpec{
		esper.FieldDef("theString", reflect.TypeOf("")),
		esper.FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
	})
	if err != nil {
		return esper.Schema{}, esper.Schema{}, err
	}
	if _, err := esper.RegisterAvro(env, "MyEvent", []esper.FieldSpec{
		esper.FieldDef("isodate", reflect.TypeOf("")),
	}); err != nil {
		return esper.Schema{}, esper.Schema{}, err
	}
	if _, err := esper.RegisterAvro(env, "MyEventOut", []esper.FieldSpec{
		esper.FieldDef("isodate", reflect.TypeOf("")),
	}); err != nil {
		return esper.Schema{}, esper.Schema{}, err
	}
	if _, err := esper.RegisterAvro(env, "MyEventPopulate", []esper.FieldSpec{
		esper.FieldDef("sb", reflect.TypeOf((*esper.AvroRecord)(nil))),
	}, esper.WithNestedPropertySchema("sb", supportSchema)); err != nil {
		return esper.Schema{}, esper.Schema{}, err
	}
	windowSchema, err := esper.RegisterAvro(env, "MyEventWSchema", []esper.FieldSpec{
		esper.OptionalFieldDef("sb", reflect.TypeOf((*esper.AvroRecord)(nil))),
	}, esper.WithNestedPropertySchema("sb", supportSchema))
	if err != nil {
		return esper.Schema{}, esper.Schema{}, err
	}
	if caseName == "named-window-property-assignment" {
		if _, err := esper.CreateNamedWindow(env, "MyWindow", windowSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return esper.Schema{}, esper.Schema{}, err
		}
	}
	return supportSchema, windowSchema, nil
}

func newEAH548Env(caseName string) (*eah548Env, error) {
	env := esper.NewEnvironment()
	supportSchema, windowSchema, err := eah548Register(env, caseName)
	if err != nil {
		return nil, err
	}
	return &eah548Env{
		env:           env,
		engine:        esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC())),
		supportSchema: supportSchema,
		windowSchema:  windowSchema,
		deployments:   map[string]*esper.Deployment{},
		statements:    map[string]*esper.Statement{},
	}, nil
}

// eah548FormatISODateTime mirrors Java's DateTimeFormatter.ISO_DATE_TIME
// over a LocalDateTime: fraction-of-second renders in nano groups of three
// with trailing zero groups omitted (9 digits for nsec%1e6!=0, 6 digits for
// nsec%1e3!=0, 3 digits otherwise).
func eah548FormatISODateTime(value time.Time) string {
	text := value.Format("2006-01-02T15:04:05")
	nsec := value.Nanosecond()
	switch {
	case nsec == 0:
	case nsec%1000000 != 0:
		text += fmt.Sprintf(".%09d", nsec)
	case nsec%1000 != 0:
		text += fmt.Sprintf(".%06d", nsec/1000)
	default:
		text += fmt.Sprintf(".%03d", nsec/1000000)
	}
	return text
}

// eah548MakeSupportBean mirrors EventAvroHook.makeSupportBean() widened by
// MySupportBeanWidener.widenInput: a SupportBeanSchema record holding
// theString/intPrimitive (E1/10 like the Java factory).
func eah548MakeSupportBean(schema esper.Schema) (*esper.AvroRecord, error) {
	return esper.NewAvroRecordFromMap(schema, map[string]any{
		"theString":    "E1",
		"intPrimitive": int64(10),
	})
}

// eah548Query maps each pinned deploy EPL to its fluent equivalent; the
// Java compile-time hook wideners are expressed as explicit Func1 UDFs.
func (e *eah548Env) query(caseName, statement string) (esper.Query, error) {
	epl, ok := eah548DeployEPLs[caseName][statement]
	if !ok {
		return esper.Query{}, fmt.Errorf("%s: unpinned deploy statement %q for case %q", eventAvroHook548ID, statement, caseName)
	}
	switch caseName {
	case "property-coerce":
		return esper.Select(esper.From[eah548LDTBean](e.env, "SupportEventWithLocalDateTime"),
			esper.Alias("isodate", esper.Func1[time.Time, string]("isoDateTime", eah548FormatISODateTime,
				esper.Field[eah548LDTBean, time.Time]("ldt"))),
		).InsertInto("MyEvent", esper.StatementName("s0")), nil
	case "schema-from-class":
		return esper.Select(esper.From[eah548SupportBean](e.env, "SupportBean"),
			esper.Alias("isodate", esper.Func1[eah548SupportBean, string]("makeLocalDateTime",
				func(eah548SupportBean) string { return eah548FormatISODateTime(time.Now()) },
				esper.EventValue[eah548SupportBean]())),
		).InsertInto("MyEventOut", esper.StatementName("s0")), nil
	case "populate":
		schema := e.supportSchema
		return esper.Select(esper.From[eah548SupportBeanS0](e.env, "SupportBean_S0"),
			esper.Alias("sb", esper.Func1[eah548SupportBeanS0, *esper.AvroRecord]("makeSupportBean",
				func(eah548SupportBeanS0) *esper.AvroRecord {
					record, err := eah548MakeSupportBean(schema)
					if err != nil {
						return nil
					}
					return record
				},
				esper.EventValue[eah548SupportBeanS0]())),
		).InsertInto("MyEventPopulate", esper.StatementName("s0")), nil
	case "named-window-property-assignment":
		switch statement {
		case "NamedWindow":
			return esper.FromNamedWindow(e.env, "MyWindow").Query(esper.StatementName("NamedWindow")), nil
		case "insert-window":
			return esper.FromAny(e.env, "MyEventWSchema").InsertInto("MyWindow", esper.StatementName("insert-window")), nil
		case "update-window":
			schema := e.supportSchema
			return esper.OnEvent(esper.From[eah548SupportBean](e.env, "SupportBean")).
				UpdateNamedWindow("MyWindow", esper.Literal(true),
					esper.SetColumn("sb", esper.Func1[eah548SupportBean, *esper.AvroRecord]("widenInput",
						func(bean eah548SupportBean) *esper.AvroRecord {
							record, err := esper.NewAvroRecordFromMap(schema, map[string]any{
								"theString":    bean.TheString,
								"intPrimitive": int64(bean.IntPrimitive),
							})
							if err != nil {
								return nil
							}
							return record
						},
						esper.EventValue[eah548SupportBean]()))).
				Query(esper.StatementName("update-window")), nil
		}
	}
	return esper.Query{}, fmt.Errorf("%s: unpinned deploy epl %q", eventAvroHook548ID, epl)
}

// deploy builds and deploys the statement for one deploy step; s0
// statements receive the recording listener (Java's addListener("s0")).
func (e *eah548Env) deploy(ctx context.Context, state *eah548CaseState, caseName, statement string) error {
	query, err := e.query(caseName, statement)
	if err != nil {
		return err
	}
	plan, err := e.env.Build(query)
	if err != nil {
		return fmt.Errorf("%s case %q: build %q: %w", eventAvroHook548ID, caseName, statement, err)
	}
	deployment, err := e.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s case %q: deploy %q: %w", eventAvroHook548ID, caseName, statement, err)
	}
	e.deployments[statement] = deployment
	for _, deployed := range deployment.Statements() {
		e.statements[deployed.Name()] = deployed
		if deployed.Name() != "s0" {
			continue
		}
		captured := deployed
		if _, err := captured.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			newRows, err := state.listenerRows(caseName, batch.New)
			if err != nil {
				return err
			}
			state.emitListener(captured.Name(), newRows)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// listenerRows normalizes delivered results, applying the case-level
// assertion the Java listener asserts in-process: ord-0 isodate equality,
// ord-1's shape-only wall-clock marker, ord-2's capture for avroToJson.
func (s *eah548CaseState) listenerRows(caseName string, results []esper.Result) ([]compat.ResultRecord, error) {
	rows := compat.NormalizeResults(results)
	for _, result := range results {
		if event, ok := result.Event(); ok {
			if record, ok := event.Underlying().(*esper.AvroRecord); ok {
				s.lastAvro = record
			}
			continue
		}
		if row, ok := result.Row(); ok {
			if record, ok := row.Get("sb").Any().(*esper.AvroRecord); ok {
				s.lastAvro = record
			}
		}
	}
	for index := range rows {
		isodate, ok := rows[index].Fields["isodate"].(string)
		switch caseName {
		case "property-coerce":
			if !ok || isodate != s.expectedIsodate {
				return nil, fmt.Errorf("%s case %q: isodate %v does not equal the pinned send value %q",
					eventAvroHook548ID, caseName, rows[index].Fields["isodate"], s.expectedIsodate)
			}
		case "schema-from-class":
			if !ok || len(isodate) <= 10 {
				return nil, fmt.Errorf("%s case %q: isodate %v fails the length > 10 shape assertion",
					eventAvroHook548ID, caseName, rows[index].Fields["isodate"])
			}
			rows[index].Fields["isodate"] = eah548IsodateMarker
		}
	}
	return rows, nil
}

// buildError runs the ord-0 invalid probe: Java's tryInvalidCompile asserts
// the widener-free zdt->isodate assignment fails; Go's typed Build boundary
// rejects the same assignment with a different message kind, so the record
// pins the Java text (intentionally-different wording, verified rejection).
func (e *eah548Env) buildError(state *eah548CaseState, step compat.Step) error {
	if step.Epl != eah548CoerceInvalidEPL || step.ExpectError != eah548CoerceProbeMessage {
		return fmt.Errorf("%s: build-error probe carries unpinned fields", eventAvroHook548ID)
	}
	_, buildErr := e.env.Build(esper.Select(esper.From[eah548ZDTBean](e.env, "SupportEventWithZonedDateTime"),
		esper.Alias("isodate", esper.Field[eah548ZDTBean, time.Time]("zdt")),
	).InsertInto("MyEvent"))
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", eventAvroHook548ID, step.Statement)
	}
	if !strings.Contains(buildErr.Error(), "isodate") || !strings.Contains(buildErr.Error(), "target expects string") {
		return fmt.Errorf("%s: build-error probe %q drift: got %v", eventAvroHook548ID, step.Statement, buildErr)
	}
	state.emitCompileError(step.Statement, step.ExpectError)
	return nil
}

// send decodes a send step: beans become the typed mirrors, MyEventWSchema
// sends a schema-bound *AvroRecord through SendAvro (the empty payload is
// Java's new GenericData.Record(schema) with the union sb left null).
func (e *eah548Env) send(ctx context.Context, state *eah548CaseState, caseName string, step compat.Step) error {
	var raw any
	if err := json.Unmarshal(step.Payload, &raw); err != nil {
		return fmt.Errorf("%s case %q: decode send payload: %w", eventAvroHook548ID, caseName, err)
	}
	object, _ := raw.(map[string]any)
	switch step.EventType {
	case "SupportEventWithLocalDateTime":
		text, ok := object["ldt"].(string)
		if !ok || text != eah548LDT {
			return fmt.Errorf("%s case %q: ldt payload %v is not the pinned %q", eventAvroHook548ID, caseName, object["ldt"], eah548LDT)
		}
		ldt, err := time.ParseInLocation("2006-01-02T15:04:05.999999999", text, time.UTC)
		if err != nil {
			return fmt.Errorf("%s case %q: parse ldt: %w", eventAvroHook548ID, caseName, err)
		}
		state.expectedIsodate = text
		return e.engine.Send(ctx, step.EventType, eah548LDTBean{Ldt: ldt})
	case "SupportBean":
		bean := eah548SupportBean{}
		if value, ok := object["theString"].(string); ok {
			bean.TheString = value
		}
		if value, ok := object["intPrimitive"].(float64); ok {
			bean.IntPrimitive = int(value)
		}
		return e.engine.Send(ctx, step.EventType, bean)
	case "SupportBean_S0":
		bean := eah548SupportBeanS0{}
		if value, ok := object["id"].(float64); ok {
			bean.ID = int(value)
		}
		return e.engine.Send(ctx, step.EventType, bean)
	case "MyEventWSchema":
		if len(object) != 0 {
			return fmt.Errorf("%s case %q: MyEventWSchema payload must be the pinned empty record", eventAvroHook548ID, caseName)
		}
		record, err := esper.NewAvroRecord(e.windowSchema)
		if err != nil {
			return fmt.Errorf("%s case %q: build MyEventWSchema record: %w", eventAvroHook548ID, caseName, err)
		}
		return e.engine.SendAvro(ctx, step.EventType, record)
	}
	return fmt.Errorf("%s case %q: unsupported send event type %q", eventAvroHook548ID, caseName, step.EventType)
}

// avroToJsonValue renders one Avro value in the avroToJson JSON shape:
// required record fields flatten; a union record keeps its branch name.
// unionBranch is the record schema name used when the field was declared as
// unionOf(null, <branch>) on the Java side ("" for required fields).
func eah548AvroToJsonValue(value any, unionBranch string) any {
	record, ok := value.(*esper.AvroRecord)
	if !ok || record == nil {
		return value
	}
	fields := make(map[string]any, len(record.Schema().Fields()))
	for _, field := range record.Schema().Fields() {
		fieldValue := record.Get(field.Name)
		if nested, ok := fieldValue.(*esper.AvroRecord); ok && nested != nil {
			fields[field.Name] = eah548AvroRecordMap(nested)
			continue
		}
		fields[field.Name] = fieldValue
	}
	if unionBranch != "" {
		return map[string]any{unionBranch: fields}
	}
	return fields
}

// eah548AvroRecordMap flattens a nested (required) record to its field map.
func eah548AvroRecordMap(record *esper.AvroRecord) map[string]any {
	fields := make(map[string]any, len(record.Schema().Fields()))
	for _, field := range record.Schema().Fields() {
		fields[field.Name] = record.Get(field.Name)
	}
	return fields
}

// eah548AvroFieldJSON renders one field value as compact JSON text in the
// Avro JsonEncoder shape: records serialize fields in declaration order
// (NOT the encoding/json alphabetical order), strings quote, null is
// literal null, and a union record wraps under its branch name.
func eah548AvroFieldJSON(value any, unionBranch string) (string, error) {
	switch current := value.(type) {
	case nil:
		return "null", nil
	case *esper.AvroRecord:
		if current == nil {
			return "null", nil
		}
		text, err := eah548AvroRecordJSON(current)
		if err != nil {
			return "", err
		}
		if unionBranch != "" {
			branch, err := json.Marshal(unionBranch)
			if err != nil {
				return "", err
			}
			return "{" + string(branch) + ":" + text + "}", nil
		}
		return text, nil
	case map[string]any:
		raw, err := json.Marshal(current)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	default:
		raw, err := json.Marshal(current)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
}

// eah548AvroRecordJSON renders a record in declaration field order with
// no union awareness (nested records flatten), matching the Avro
// JsonEncoder output for a required record field.
func eah548AvroRecordJSON(record *esper.AvroRecord) (string, error) {
	var text strings.Builder
	text.WriteByte('{')
	for index, field := range record.Schema().Fields() {
		if index > 0 {
			text.WriteByte(',')
		}
		name, err := json.Marshal(field.Name)
		if err != nil {
			return "", err
		}
		text.Write(name)
		text.WriteByte(':')
		value, err := eah548AvroFieldJSON(record.Get(field.Name), "")
		if err != nil {
			return "", err
		}
		text.WriteString(value)
	}
	text.WriteByte('}')
	return text.String(), nil
}

// eah548AvroToJson renders the whole record as the compact JSON text the
// Java SupportAvroUtil.avroToJson produces (field order follows schema
// declaration; the named union field wraps its record under the branch
// name and a null union renders literal null).
func eah548AvroToJson(record *esper.AvroRecord, unionField, unionBranch string) (string, error) {
	var text strings.Builder
	text.WriteByte('{')
	for index, field := range record.Schema().Fields() {
		if index > 0 {
			text.WriteByte(',')
		}
		name, err := json.Marshal(field.Name)
		if err != nil {
			return "", err
		}
		text.Write(name)
		text.WriteByte(':')
		branch := ""
		if field.Name == unionField {
			branch = unionBranch
		}
		value, err := eah548AvroFieldJSON(record.Get(field.Name), branch)
		if err != nil {
			return "", err
		}
		text.WriteString(value)
	}
	text.WriteByte('}')
	return text.String(), nil
}

// avroRow renders a window row event in the avroToJson shape for snapshot
// records (union branch kept; a null union stays {state:null} via nil ->
// the trace's null marker replacement below).
func (e *eah548Env) avroRows(result esper.QueryResult, unionField, unionBranch string) ([]compat.ResultRecord, error) {
	rows := compat.NormalizeResults(result.Batch.New)
	for index, resultItem := range result.Batch.New {
		event, ok := resultItem.Event()
		if !ok {
			continue
		}
		record, ok := event.Underlying().(*esper.AvroRecord)
		if !ok || record == nil {
			continue
		}
		row := map[string]any{}
		for _, field := range record.Schema().Fields() {
			fieldValue := record.Get(field.Name)
			if fieldValue == nil {
				row[field.Name] = map[string]any{"state": "null"}
				continue
			}
			branch := ""
			if field.Name == unionField {
				branch = unionBranch
			}
			row[field.Name] = eah548AvroToJsonValue(fieldValue, branch)
		}
		if index < len(rows) {
			rows[index].Fields = row
		}
	}
	return rows, nil
}

// value runs the pinned in-process assertion for one value step and emits
// the record: avro-schema pins MyEventOut's schema text (Java asserts
// schema.toString() byte-exact; Go verifies the declared field set carries
// the same observable shape), avroToJson pins the asserted record encoding
// (flat for populate, union-nested for the window).
func (e *eah548Env) value(ctx context.Context, state *eah548CaseState, caseName string, step compat.Step) error {
	switch caseName + "/" + step.Name {
	case "schema-from-class/avro-schema":
		schema, ok := e.env.Schema("MyEventOut")
		if !ok {
			return fmt.Errorf("%s case %q: MyEventOut schema is missing", eventAvroHook548ID, caseName)
		}
		fields := schema.Fields()
		if schema.Name() != "MyEventOut" || len(fields) != 1 || fields[0].Name != "isodate" || fields[0].Type != reflect.TypeOf("") {
			return fmt.Errorf("%s case %q: MyEventOut schema = %q %#v, want record MyEventOut with one string isodate field",
				eventAvroHook548ID, caseName, schema.Name(), fields)
		}
		state.emitValue(step.Statement, step.Name, eah548SchemaJSON)
		return nil
	case "populate/avroToJson":
		record := state.lastAvro
		if record == nil {
			return fmt.Errorf("%s case %q: avroToJson step has no captured event", eventAvroHook548ID, caseName)
		}
		// The captured record is the delivered event's sb member; the
		// avroToJson text wraps it in the MyEventPopulate(sb) field (a
		// required field, so the record flattens — no union branch name)
		// with declaration field order, not encoding/json key order.
		inner, err := eah548AvroRecordJSON(record)
		if err != nil {
			return err
		}
		text := `{"sb":` + inner + `}`
		if text != eah548PopulateJSON {
			return fmt.Errorf("%s case %q: avroToJson = %q, want pinned %q", eventAvroHook548ID, caseName, text, eah548PopulateJSON)
		}
		state.emitValue(step.Statement, step.Name, text)
		return nil
	case "named-window-property-assignment/avroToJson":
		statement, ok := e.statements[step.Statement]
		if !ok {
			return fmt.Errorf("%s case %q: statement %q not deployed", eventAvroHook548ID, caseName, step.Statement)
		}
		result, err := statement.Snapshot(ctx)
		if err != nil {
			return err
		}
		var first *esper.AvroRecord
		for _, item := range result.Batch.New {
			if event, ok := item.Event(); ok {
				if record, ok := event.Underlying().(*esper.AvroRecord); ok {
					first = record
					break
				}
			}
		}
		if first == nil {
			return fmt.Errorf("%s case %q: window holds no avro row", eventAvroHook548ID, caseName)
		}
		text, err := eah548AvroToJson(first, "sb", eah548UnionBranch[caseName])
		if err != nil {
			return err
		}
		if text != eah548WindowJSON {
			return fmt.Errorf("%s case %q: avroToJson = %q, want pinned %q", eventAvroHook548ID, caseName, text, eah548WindowJSON)
		}
		state.emitValue(step.Statement, step.Name, text)
		return nil
	}
	return fmt.Errorf("%s case %q: unsupported value step %q", eventAvroHook548ID, caseName, step.Name)
}

func runEventAvroHook548Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	type caseBlock struct {
		name  string
		steps []compat.Step
	}
	var blocks []caseBlock
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			blocks = append(blocks, caseBlock{name: step.Case})
			continue
		}
		if len(blocks) == 0 {
			return trace, fmt.Errorf("%s: step %q precedes the first case marker", eventAvroHook548ID, step.Op)
		}
		blocks[len(blocks)-1].steps = append(blocks[len(blocks)-1].steps, step)
	}
	for _, block := range blocks {
		if err := runEAH548Case(ctx, block.name, block.steps, &trace); err != nil {
			return trace, fmt.Errorf("%s case %q: %w", eventAvroHook548ID, block.name, err)
		}
	}
	return trace, nil
}

func runEAH548Case(ctx context.Context, caseName string, steps []compat.Step, trace *compat.Trace) error {
	if !slices.Contains(eventAvroHook548Cases, caseName) {
		return fmt.Errorf("unsupported case %q", caseName)
	}
	state := newEAH548CaseState(caseName, trace)
	ee, err := newEAH548Env(caseName)
	if err != nil {
		return err
	}
	defer func() { _ = ee.engine.Close(ctx) }()
	for _, step := range steps {
		switch step.Op {
		case "deploy":
			if err := ee.deploy(ctx, state, caseName, step.Statement); err != nil {
				return err
			}
		case "deployed":
			if _, ok := ee.statements[step.Statement]; !ok {
				return fmt.Errorf("deployed marker for unknown statement %q", step.Statement)
			}
			state.emitDeployed(step.Statement)
		case "build-error":
			if err := ee.buildError(state, step); err != nil {
				return err
			}
		case "send":
			if err := ee.send(ctx, state, caseName, step); err != nil {
				return err
			}
		case "value":
			if err := ee.value(ctx, state, caseName, step); err != nil {
				return err
			}
		case "snapshot":
			statement, ok := ee.statements[step.Statement]
			if !ok {
				return fmt.Errorf("snapshot statement %q not found", step.Statement)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return err
			}
			rows, err := ee.avroRows(result, "sb", eah548UnionBranch[caseName])
			if err != nil {
				return err
			}
			state.emitSnapshot(step.Statement, rows)
		case "undeploy-all":
			for name, deployment := range ee.deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return fmt.Errorf("undeploy %q: %w", name, err)
				}
			}
			ee.deployments = map[string]*esper.Deployment{}
			ee.statements = map[string]*esper.Statement{}
		default:
			return fmt.Errorf("unsupported op %q in %q", step.Op, caseName)
		}
	}
	return nil
}

// ---------- pinned step keys and scenario loader ----------

func eah548CaseKey(caseName string) string {
	return strings.Join([]string{"case", caseName, "", "", "", "", "", "", ""}, "|")
}
func eah548DeployKey(caseName, statement, epl string) string {
	return strings.Join([]string{"deploy", caseName, statement, "", "", "", epl, "", ""}, "|")
}
func eah548DeployedKey(caseName, statement string) string {
	return strings.Join([]string{"deployed", caseName, statement, "", "", "", "", "", ""}, "|")
}
func eah548BuildErrorKey(caseName, statement, epl, expectError string) string {
	return strings.Join([]string{"build-error", caseName, statement, "", "", "", epl, expectError, ""}, "|")
}
func eah548SendKey(caseName, eventType, payload string) string {
	return strings.Join([]string{"send", caseName, "", eventType, "", "", "", "", payload}, "|")
}
func eah548ValueKey(caseName, statement, name string) string {
	return strings.Join([]string{"value", caseName, statement, "", "", name, "", "", ""}, "|")
}
func eah548SnapshotKey(caseName, statement string) string {
	return strings.Join([]string{"snapshot", caseName, statement, "", "", "", "", "", ""}, "|")
}
func eah548UndeployAllKey(caseName string) string {
	return strings.Join([]string{"undeploy-all", caseName, "", "", "", "", "", "", ""}, "|")
}

// eventAvroHook548PinnedSteps rebuilds the pinned step-key sequence; the
// loader compares every decoded step against it.
func eventAvroHook548PinnedSteps() []string {
	var keys []string
	// property-coerce (ord 0): invalid zdt probe, valid ldt deploy+send.
	keys = append(keys,
		eah548CaseKey("property-coerce"),
		eah548BuildErrorKey("property-coerce", "probe", eah548CoerceInvalidEPL, eah548CoerceProbeMessage),
		eah548DeployKey("property-coerce", "s0", eah548CoerceEPL),
		eah548DeployedKey("property-coerce", "s0"),
		eah548SendKey("property-coerce", "SupportEventWithLocalDateTime", `{"ldt":"`+eah548LDT+`"}`),
		eah548UndeployAllKey("property-coerce"),
	)
	// schema-from-class (ord 1): deploy, schema assertion, one send.
	keys = append(keys,
		eah548CaseKey("schema-from-class"),
		eah548DeployKey("schema-from-class", "s0", eah548SchemaEPL),
		eah548DeployedKey("schema-from-class", "s0"),
		eah548ValueKey("schema-from-class", "s0", "avro-schema"),
		eah548SendKey("schema-from-class", "SupportBean", `{"intPrimitive":10,"theString":"E1"}`),
		eah548UndeployAllKey("schema-from-class"),
	)
	// populate (ord 2): deploy, one SupportBean_S0 send, record-JSON assert.
	keys = append(keys,
		eah548CaseKey("populate"),
		eah548DeployKey("populate", "s0", eah548PopulateEPL),
		eah548DeployedKey("populate", "s0"),
		eah548SendKey("populate", "SupportBean_S0", `{"id":10}`),
		eah548ValueKey("populate", "s0", "avroToJson"),
		eah548UndeployAllKey("populate"),
	)
	// named-window-property-assignment (ord 3): window create + insert +
	// update, empty-record seed, SupportBean trigger, iterator assert.
	keys = append(keys,
		eah548CaseKey("named-window-property-assignment"),
		eah548DeployKey("named-window-property-assignment", "NamedWindow", eah548WindowCreateEPL),
		eah548DeployedKey("named-window-property-assignment", "NamedWindow"),
		eah548DeployKey("named-window-property-assignment", "insert-window", eah548WindowInsertEPL),
		eah548DeployedKey("named-window-property-assignment", "insert-window"),
		eah548DeployKey("named-window-property-assignment", "update-window", eah548WindowUpdateEPL),
		eah548DeployedKey("named-window-property-assignment", "update-window"),
		eah548SendKey("named-window-property-assignment", "MyEventWSchema", `{}`),
		eah548SendKey("named-window-property-assignment", "SupportBean", `{"intPrimitive":10,"theString":"E1"}`),
		eah548SnapshotKey("named-window-property-assignment", "NamedWindow"),
		eah548ValueKey("named-window-property-assignment", "NamedWindow", "avroToJson"),
		eah548UndeployAllKey("named-window-property-assignment"),
	)
	return keys
}

func eah548StepKey(step compat.Step) string {
	payload := ""
	if len(step.Payload) > 0 {
		var decoded any
		if err := json.Unmarshal(step.Payload, &decoded); err == nil {
			if compacted, err := json.Marshal(decoded); err == nil {
				payload = string(compacted)
			}
		}
		if payload == "" {
			payload = string(step.Payload)
		}
	}
	return strings.Join([]string{step.Op, step.Case, step.Statement, step.EventType, step.Mode, step.Name, step.Epl, step.ExpectError, payload}, "|")
}

func eah548ValidateStep(index int, object map[string]json.RawMessage, step compat.Step) error {
	require := func(names ...string) error {
		if err := requireEventInfraGetterNestedFields(object, names...); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		return nil
	}
	switch step.Op {
	case "case":
		return require("op", "case")
	case "deploy":
		return require("op", "case", "statement", "epl")
	case "deployed":
		return require("op", "case", "statement")
	case "build-error":
		return require("op", "case", "statement", "epl", "expectError")
	case "send":
		return require("op", "case", "eventType", "payload")
	case "value":
		return require("op", "case", "statement", "name")
	case "snapshot":
		return require("op", "case", "statement")
	case "undeploy-all":
		return require("op", "case")
	}
	return fmt.Errorf("scenario step %d has unsupported op %q", index, step.Op)
}

func loadEventAvroHook548Scenario(reader io.Reader) (compat.Scenario, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario must be a JSON object: %w", eventAvroHook548ID, err)
	}
	if err := requireEventInfraGetterNestedFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version      string   `json:"version"`
		ID           string   `json:"id"`
		Description  string   `json:"description"`
		JavaCommit   string   `json:"javaCommit"`
		JavaSource   string   `json:"javaSource"`
		JavaRuntimes []string `json:"javaRuntimes"`
		JavaNames    []string `json:"javaNames"`
		JavaStaticID []string `json:"javaStaticIds"`
		JavaFlags    []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eventAvroHook548ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eventAvroHook548ID ||
		metadata.Description != eventAvroHook548Description ||
		metadata.JavaCommit != eventAvroHook548JavaCommit || metadata.JavaSource != eventAvroHook548Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", eventAvroHook548ID)
	}
	if !reflect.DeepEqual(metadata.JavaFlags, eventAvroHook548JavaFlags) {
		return compat.Scenario{}, fmt.Errorf("%s javaFlags = %#v", eventAvroHook548ID, metadata.JavaFlags)
	}
	if !reflect.DeepEqual(metadata.JavaRuntimes, eventAvroHook548JavaRuntimeIDs) {
		return compat.Scenario{}, fmt.Errorf("%s javaRuntimes = %#v", eventAvroHook548ID, metadata.JavaRuntimes)
	}
	if !reflect.DeepEqual(metadata.JavaNames, eventAvroHook548JavaExecutions) {
		return compat.Scenario{}, fmt.Errorf("%s javaNames = %#v", eventAvroHook548ID, metadata.JavaNames)
	}
	if !reflect.DeepEqual(metadata.JavaStaticID, eventAvroHook548JavaStaticIDs) {
		return compat.Scenario{}, fmt.Errorf("%s javaStaticIds = %#v", eventAvroHook548ID, metadata.JavaStaticID)
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", eventAvroHook548ID, err)
	}
	if len(rawCases) != len(eventAvroHook548Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", eventAvroHook548ID, len(rawCases), len(eventAvroHook548Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEventInfraGetterNestedFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "epl", "flags"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string   `json:"case"`
			Ordinal       int      `json:"ordinal"`
			RuntimeID     string   `json:"runtimeId"`
			ExecutionName string   `json:"executionName"`
			Observation   string   `json:"observation"`
			EPL           string   `json:"epl"`
			Flags         []string `json:"flags"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != eventAvroHook548Cases[index] || definition.Ordinal != eventAvroHook548CaseOrdinals[index] ||
			definition.RuntimeID != eventAvroHook548JavaRuntimeIDs[index] ||
			definition.ExecutionName != eventAvroHook548JavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d identity mismatch", eventAvroHook548ID, index)
		}
		if definition.Observation != eventAvroHook548CaseObservations[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %q observation is not pinned", eventAvroHook548ID, definition.Case)
		}
		if definition.EPL != eventAvroHook548CaseEPLs[definition.Case] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %q EPL is not pinned", eventAvroHook548ID, definition.Case)
		}
		if !reflect.DeepEqual(definition.Flags, eventAvroHook548CaseFlags[index]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %q flags = %#v, want %#v",
				eventAvroHook548ID, definition.Case, definition.Flags, eventAvroHook548CaseFlags[index])
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", eventAvroHook548ID, err)
	}
	pinned := eventAvroHook548PinnedSteps()
	if len(rawSteps) != len(pinned) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d steps, want %d", eventAvroHook548ID, len(rawSteps), len(pinned))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var step compat.Step
		if err := json.Unmarshal(rawStep, &step); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := eah548ValidateStep(index, object, step); err != nil {
			return compat.Scenario{}, err
		}
		if key := eah548StepKey(step); key != pinned[index] {
			return compat.Scenario{}, fmt.Errorf("scenario step %d = %q, want pinned %q", index, key, pinned[index])
		}
		steps[index] = step
	}
	return compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}, nil
}
