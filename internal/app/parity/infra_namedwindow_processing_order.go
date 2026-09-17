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

const (
	infraNWProcessingOrderID         = "infra-namedwindow-processing-order"
	infraNWProcessingOrderJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWProcessingOrderSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowProcessingOrder.java"
)

const infraNWProcessingOrderDescription = "InfraNamedWindowProcessingOrder dispatch-order semantics: six event-representation variants of the back-queue insert-into/on-update chain (ords 0-5, observably identical in Go) plus the ordered delete-then-select execution (ord 6). listener records capture the irstream select's atomic old/new update pair and the s0 insert-into stream deliveries (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowProcessingOrder.java)."

var infraNWProcessingOrderCaseObservations = []string{
	"listener; three declared-schema event types feed a back-queue insert-into chain: StartValueEvent inserts V1/O1 into a #unique(prop1) named window, TestInputEvent routes TestForwardEvent which fires an on-update trigger in the same dispatch, and the irstream select observes the update as one old/new pair (O1->U1)",
	"listener; three declared-schema event types feed a back-queue insert-into chain: StartValueEvent inserts V1/O1 into a #unique(prop1) named window, TestInputEvent routes TestForwardEvent which fires an on-update trigger in the same dispatch, and the irstream select observes the update as one old/new pair (O1->U1)",
	"listener; three declared-schema event types feed a back-queue insert-into chain: StartValueEvent inserts V1/O1 into a #unique(prop1) named window, TestInputEvent routes TestForwardEvent which fires an on-update trigger in the same dispatch, and the irstream select observes the update as one old/new pair (O1->U1)",
	"listener; three declared-schema event types feed a back-queue insert-into chain: StartValueEvent inserts V1/O1 into a #unique(prop1) named window, TestInputEvent routes TestForwardEvent which fires an on-update trigger in the same dispatch, and the irstream select observes the update as one old/new pair (O1->U1)",
	"listener; three declared-schema event types feed a back-queue insert-into chain: StartValueEvent inserts V1/O1 into a #unique(prop1) named window, TestInputEvent routes TestForwardEvent which fires an on-update trigger in the same dispatch, and the irstream select observes the update as one old/new pair (O1->U1)",
	"listener; three declared-schema event types feed a back-queue insert-into chain: StartValueEvent inserts V1/O1 into a #unique(prop1) named window, TestInputEvent routes TestForwardEvent which fires an on-update trigger in the same dispatch, and the irstream select observes the update as one old/new pair (O1->U1)",
	"listener; a #lastevent named window feeds three on-triggers in declaration order: two conditional deletes (intPrimitive 7 then 5) run before an on-insert select that performs a live window lookup per trigger event, so deleted rows never reach ResultStream",
}

var infraNWProcessingOrderCaseEPLs = []string{
	"@EventRepresentation('objectarray') @buseventtype @public create schema StartValueEvent as (dummy string);\n@EventRepresentation('objectarray') @buseventtype @public create schema TestForwardEvent as (prop1 string);\n@EventRepresentation('objectarray') @buseventtype @public create schema TestInputEvent as (dummy string);\ninsert into TestForwardEvent select'V1' as prop1 from TestInputEvent;\n@EventRepresentation('objectarray') @public create window NamedWin#unique(prop1) (prop1 string, prop2 string);\ninsert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent;\non TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1';\n@name('select') select irstream prop1, prop2 from NamedWin;",
	"@EventRepresentation('map') @buseventtype @public create schema StartValueEvent as (dummy string);\n@EventRepresentation('map') @buseventtype @public create schema TestForwardEvent as (prop1 string);\n@EventRepresentation('map') @buseventtype @public create schema TestInputEvent as (dummy string);\ninsert into TestForwardEvent select'V1' as prop1 from TestInputEvent;\n@EventRepresentation('map') @public create window NamedWin#unique(prop1) (prop1 string, prop2 string);\ninsert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent;\non TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1';\n@name('select') select irstream prop1, prop2 from NamedWin;",
	"@EventRepresentation('avro') @buseventtype @public create schema StartValueEvent as (dummy string);\n@EventRepresentation('avro') @buseventtype @public create schema TestForwardEvent as (prop1 string);\n@EventRepresentation('avro') @buseventtype @public create schema TestInputEvent as (dummy string);\ninsert into TestForwardEvent select'V1' as prop1 from TestInputEvent;\n@EventRepresentation('avro') @public create window NamedWin#unique(prop1) (prop1 string, prop2 string);\ninsert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent;\non TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1';\n@name('select') select irstream prop1, prop2 from NamedWin;",
	"@EventRepresentation('json') @buseventtype @public create schema StartValueEvent as (dummy string);\n@EventRepresentation('json') @buseventtype @public create schema TestForwardEvent as (prop1 string);\n@EventRepresentation('json') @buseventtype @public create schema TestInputEvent as (dummy string);\ninsert into TestForwardEvent select'V1' as prop1 from TestInputEvent;\n@EventRepresentation('json') @public create window NamedWin#unique(prop1) (prop1 string, prop2 string);\ninsert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent;\non TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1';\n@name('select') select irstream prop1, prop2 from NamedWin;",
	"@JsonSchema(className='com.espertech.esper.regressionlib.suite.infra.namedwindow.InfraNamedWindowProcessingOrder$MyLocalJsonProvidedStartValueEvent') @EventRepresentation('json') @buseventtype @public create schema StartValueEvent as (dummy string);\n@JsonSchema(className='com.espertech.esper.regressionlib.suite.infra.namedwindow.InfraNamedWindowProcessingOrder$MyLocalJsonProvidedTestForwardEvent') @EventRepresentation('json') @buseventtype @public create schema TestForwardEvent as (prop1 string);\n@JsonSchema(className='com.espertech.esper.regressionlib.suite.infra.namedwindow.InfraNamedWindowProcessingOrder$MyLocalJsonProvidedTestInputEvent') @EventRepresentation('json') @buseventtype @public create schema TestInputEvent as (dummy string);\ninsert into TestForwardEvent select'V1' as prop1 from TestInputEvent;\n@JsonSchema(className='com.espertech.esper.regressionlib.suite.infra.namedwindow.InfraNamedWindowProcessingOrder$MyLocalJsonProvidedNamedWin') @EventRepresentation('json') @public create window NamedWin#unique(prop1) (prop1 string, prop2 string);\ninsert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent;\non TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1';\n@name('select') select irstream prop1, prop2 from NamedWin;",
	" @buseventtype @public create schema StartValueEvent as (dummy string);\n @buseventtype @public create schema TestForwardEvent as (prop1 string);\n @buseventtype @public create schema TestInputEvent as (dummy string);\ninsert into TestForwardEvent select'V1' as prop1 from TestInputEvent;\n@public create window NamedWin#unique(prop1) (prop1 string, prop2 string);\ninsert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent;\non TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1';\n@name('select') select irstream prop1, prop2 from NamedWin;",
	"create window MyWindow#lastevent as select * from SupportBean;\ninsert into MyWindow select * from SupportBean;\non MyWindow e delete from MyWindow win where win.theString=e.theString and e.intPrimitive = 7;\non MyWindow e delete from MyWindow win where win.theString=e.theString and e.intPrimitive = 5;\non MyWindow e insert into ResultStream select e.* from MyWindow;\n@name('s0') select * from ResultStream;",
}

var (
	infraNWProcessingOrderJavaRuntimeIDs = []string{
		"java-runtime-d103aeca629813a82acf",
		"java-runtime-0c77245232ebd6281a47",
		"java-runtime-cef00c40d92d73111b6e",
		"java-runtime-21dd5ef783d585d85dfd",
		"java-runtime-59a4da55e98ca67b95c2",
		"java-runtime-99f94152e6a94e3e60bc",
		"java-runtime-c56598034a18ee372891",
	}
	infraNWProcessingOrderJavaExecutions = []string{
		"InfraDispatchBackQueue{OBJECTARRAY}",
		"InfraDispatchBackQueue{MAP}",
		"InfraDispatchBackQueue{AVRO}",
		"InfraDispatchBackQueue{JSON}",
		"InfraDispatchBackQueue{JSONCLASSPROVIDED}",
		"InfraDispatchBackQueue{DEFAULT}",
		"InfraOrderedDeleteAndSelect",
	}
	infraNWProcessingOrderJavaStaticIDs = []string{
		"java-78aff9650bba15e78056",
		"java-78aff9650bba15e78056",
		"java-78aff9650bba15e78056",
		"java-78aff9650bba15e78056",
		"java-78aff9650bba15e78056",
		"java-78aff9650bba15e78056",
		"java-78aff9650bba15e78056",
	}
	infraNWProcessingOrderJavaFlags = []string{"EXCLUDEWHENINSTRUMENTED"}
	infraNWProcessingOrderCases     = []string{
		"dispatch-objectarray",
		"dispatch-map",
		"dispatch-avro",
		"dispatch-json",
		"dispatch-json-provided",
		"dispatch-default",
		"ordered-delete-select",
	}
	infraNWProcessingOrderOrdinals = []int{0, 1, 2, 3, 4, 5, 6}
	infraNWProcessingOrderSources  = []string{infraNWProcessingOrderSource}
)

// infraNWProcessingOrderBean mirrors the full SupportBean schema for the
// ordered-delete-select case (the on-insert trigger projects e.*).
type infraNWProcessingOrderBean struct {
	TheString       string   `esper:"theString"`
	IntPrimitive    int64    `esper:"intPrimitive"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntBoxed        *int64   `esper:"intBoxed"`
	CharPrimitive   string   `esper:"charPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *string  `esper:"bigDecimal"`
	BigInteger      *string  `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

// infraNWProcessingOrderJSONProvided* stand in for Java's
// MyLocalJsonProvided<T> wrappers used by the JSONCLASSPROVIDED variant:
// one provided class per declared event type.
type infraNWProcessingOrderJSONProvidedStart struct {
	Dummy string `esper:"dummy"`
}

type infraNWProcessingOrderJSONProvidedForward struct {
	Prop1 string `esper:"prop1"`
}

type infraNWProcessingOrderJSONProvidedInput struct {
	Dummy string `esper:"dummy"`
}

type infraNWProcessingOrderSendSpec struct {
	repr   string
	fields map[string]json.RawMessage
}

func decodeInfraNWProcessingOrderPayload(step compat.Step) (infraNWProcessingOrderSendSpec, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return infraNWProcessingOrderSendSpec{}, fmt.Errorf("decode %s payload: %w", step.EventType, err)
	}
	if step.EventType == "SupportBean" {
		for key := range fields {
			switch key {
			case "theString", "intPrimitive", "boolPrimitive", "intBoxed", "charPrimitive",
				"longPrimitive", "shortPrimitive", "bytePrimitive", "floatPrimitive",
				"doublePrimitive", "boolBoxed", "charBoxed", "longBoxed", "shortBoxed",
				"byteBoxed", "floatBoxed", "doubleBoxed", "bigDecimal", "bigInteger", "enumValue":
			default:
				return infraNWProcessingOrderSendSpec{}, fmt.Errorf("unknown %s field %q", step.EventType, key)
			}
		}
		return infraNWProcessingOrderSendSpec{fields: fields}, nil
	}
	switch step.EventType {
	case "StartValueEvent", "TestInputEvent":
		if len(fields) != 2 {
			return infraNWProcessingOrderSendSpec{}, fmt.Errorf("%s payload must carry repr and fields", step.EventType)
		}
		var repr string
		if err := json.Unmarshal(fields["repr"], &repr); err != nil {
			return infraNWProcessingOrderSendSpec{}, fmt.Errorf("decode %s repr: %w", step.EventType, err)
		}
		var inner map[string]json.RawMessage
		if err := strictObject(fields["fields"], &inner); err != nil {
			return infraNWProcessingOrderSendSpec{}, fmt.Errorf("decode %s fields: %w", step.EventType, err)
		}
		for key := range inner {
			if key != "dummy" {
				return infraNWProcessingOrderSendSpec{}, fmt.Errorf("unknown %s field %q", step.EventType, key)
			}
		}
		return infraNWProcessingOrderSendSpec{repr: repr, fields: inner}, nil
	default:
		return infraNWProcessingOrderSendSpec{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

// infraNWProcessingOrderBuild builds the plan for one deploy step. Schema
// statements register the event type on the environment and return a nil
// plan (Java deploys them as statements; Go registers them env-level).
// The create-window statement registers the window env-level and deploys a
// direct-child select to keep the statement slot.
func infraNWProcessingOrderBuild(env *esper.Environment, caseName, statement, epl string) (esper.Plan, error) {
	mapSchema := func(name string, fields ...esper.FieldSpec) (esper.Schema, error) {
		schema, err := esper.NewMapSchema(name, fields)
		if err != nil {
			return esper.Schema{}, err
		}
		if err := env.RegisterSchema(schema); err != nil {
			return esper.Schema{}, err
		}
		return schema, nil
	}
	registerDispatchSchema := func(name string, fields ...esper.FieldSpec) error {
		repr := infraNWProcessingOrderRepr(caseName)
		var err error
		switch repr {
		case "objectarray":
			_, err = esper.RegisterObjectArray(env, name, fields, esper.BusEventType())
		case "map", "default":
			_, err = esper.RegisterMap(env, name, fields, esper.BusEventType())
		case "avro":
			_, err = esper.RegisterAvro(env, name, fields, esper.BusEventType())
		case "json":
			_, err = esper.RegisterJSON(env, name, fields, esper.BusEventType())
		case "json-provided":
			switch name {
			case "StartValueEvent":
				_, err = esper.RegisterJSONFor[infraNWProcessingOrderJSONProvidedStart](env, name, fields, esper.BusEventType())
			case "TestForwardEvent":
				_, err = esper.RegisterJSONFor[infraNWProcessingOrderJSONProvidedForward](env, name, fields, esper.BusEventType())
			case "TestInputEvent":
				_, err = esper.RegisterJSONFor[infraNWProcessingOrderJSONProvidedInput](env, name, fields, esper.BusEventType())
			default:
				err = fmt.Errorf("unknown json-provided event type %q", name)
			}
		default:
			err = fmt.Errorf("unknown representation %q", repr)
		}
		return err
	}
	dummyField := []esper.FieldSpec{esper.FieldDef("dummy", reflect.TypeOf(""))}
	prop1Field := []esper.FieldSpec{esper.FieldDef("prop1", reflect.TypeOf(""))}

	switch statement {
	case "schema-start":
		return esper.Plan{}, registerDispatchSchema("StartValueEvent", dummyField...)
	case "schema-forward":
		return esper.Plan{}, registerDispatchSchema("TestForwardEvent", prop1Field...)
	case "schema-input":
		return esper.Plan{}, registerDispatchSchema("TestInputEvent", dummyField...)
	}

	if caseName == "ordered-delete-select" {
		bean := esper.From[infraNWProcessingOrderBean](env, "SupportBean")
		beanField := func(name string) esper.Expression[string] {
			return esper.Field[infraNWProcessingOrderBean, string](name)
		}
		beanInt := func(name string) esper.Expression[int64] {
			return esper.Field[infraNWProcessingOrderBean, int64](name)
		}
		nwStr := func(name string) esper.Expression[string] {
			return esper.NamedWindowField[string](name)
		}
		switch statement {
		case "create":
			schema, ok := env.Schema("SupportBean")
			if !ok {
				return esper.Plan{}, fmt.Errorf("SupportBean schema not registered")
			}
			if _, err := esper.CreateNamedWindow(env, "MyWindow", schema,
				esper.NamedWindowRetention(esper.LastEvent())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
		case "insert":
			return env.Build(bean.InsertInto("MyWindow", esper.StatementName("insert")))
		case "delete7":
			return env.Build(esper.OnRecord(esper.FromNamedWindow(env, "MyWindow")).
				DeleteFromNamedWindow("MyWindow", esper.And(
					esper.EqualOf(nwStr("theString"), beanField("theString")),
					esper.Equal[int64](beanInt("intPrimitive"), esper.Literal(int64(7))))).
				Query(esper.StatementName("delete7")))
		case "delete5":
			return env.Build(esper.OnRecord(esper.FromNamedWindow(env, "MyWindow")).
				DeleteFromNamedWindow("MyWindow", esper.And(
					esper.EqualOf(nwStr("theString"), beanField("theString")),
					esper.Equal[int64](beanInt("intPrimitive"), esper.Literal(int64(5))))).
				Query(esper.StatementName("delete5")))
		case "oninsert":
			// Java's insert-into creates the ResultStream type implicitly;
			// Go requires the route target registered with the projected
			// field set (the full SupportBean shape of e.*).
			if _, err := mapSchema("ResultStream", infraNWProcessingOrderBeanFieldSpecs()...); err != nil {
				return esper.Plan{}, err
			}
			// select e.* projects the trigger event once per window row;
			// enumerate the SupportBean fields explicitly (no wildcard
			// selection exists for trigger projections).
			return env.Build(esper.OnRecord(esper.FromNamedWindow(env, "MyWindow")).
				SelectFromNamedWindow("MyWindow", nil, infraNWProcessingOrderBeanSelections()...).
				Query(esper.RouteTo("ResultStream"), esper.StatementName("oninsert")))
		case "s0":
			return env.Build(esper.FromAny(env, "ResultStream").Query(esper.StatementName("s0")))
		}
		return esper.Plan{}, fmt.Errorf("%s case %q statement %q has no plan", infraNWProcessingOrderID, caseName, statement)
	}

	// dispatch-* cases
	switch statement {
	case "insert-forward":
		return env.Build(esper.FromAny(env, "TestInputEvent").
			Select(esper.Alias("prop1", esper.Literal("V1"))).
			InsertInto("TestForwardEvent", esper.StatementName("insert-forward")))
	case "create":
		schema, err := mapSchema("NamedWinSchema",
			esper.FieldDef("prop1", reflect.TypeOf("")),
			esper.FieldDef("prop2", reflect.TypeOf("")))
		if err != nil {
			return esper.Plan{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "NamedWin", schema,
			esper.NamedWindowRetention(esper.Unique(esper.Field[any, string]("prop1")))); err != nil {
			return esper.Plan{}, err
		}
		return env.Build(esper.FromNamedWindow(env, "NamedWin").
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert-window":
		return env.Build(esper.FromAny(env, "StartValueEvent").
			Select(
				esper.Alias("prop1", esper.Literal("V1")),
				esper.Alias("prop2", esper.Literal("O1"))).
			InsertInto("NamedWin", esper.StatementName("insert-window")))
	case "update":
		return env.Build(esper.OnRecord(esper.FromAny(env, "TestForwardEvent")).
			UpdateNamedWindow("NamedWin",
				esper.EqualOf(esper.NamedWindowField[string]("prop1"), esper.Literal("V1")),
				esper.SetColumn("prop2", esper.Literal("U1"))).
			Query(esper.StatementName("update")))
	case "select":
		return env.Build(esper.FromNamedWindow(env, "NamedWin").
			Select(
				esper.Alias("prop1", esper.Field[any, string]("prop1")),
				esper.Alias("prop2", esper.Field[any, string]("prop2"))).
			Query(esper.StatementName("select"), esper.WithOldStream()))
	}
	return esper.Plan{}, fmt.Errorf("%s case %q statement %q has no plan", infraNWProcessingOrderID, caseName, statement)
}

func infraNWProcessingOrderRepr(caseName string) string {
	switch caseName {
	case "dispatch-objectarray":
		return "objectarray"
	case "dispatch-map":
		return "map"
	case "dispatch-avro":
		return "avro"
	case "dispatch-json":
		return "json"
	case "dispatch-json-provided":
		return "json-provided"
	case "dispatch-default":
		return "default"
	}
	return ""
}

func infraNWProcessingOrderBeanFieldSpecs() []esper.FieldSpec {
	str := reflect.TypeOf("")
	i64 := reflect.TypeOf(int64(0))
	b := reflect.TypeOf(false)
	i16 := reflect.TypeOf(int16(0))
	i8 := reflect.TypeOf(int8(0))
	f32 := reflect.TypeOf(float32(0))
	f64 := reflect.TypeOf(float64(0))
	strP := reflect.TypeOf((*string)(nil))
	i64P := reflect.TypeOf((*int64)(nil))
	bP := reflect.TypeOf((*bool)(nil))
	i16P := reflect.TypeOf((*int16)(nil))
	i8P := reflect.TypeOf((*int8)(nil))
	f32P := reflect.TypeOf((*float32)(nil))
	f64P := reflect.TypeOf((*float64)(nil))
	return []esper.FieldSpec{
		esper.FieldDef("theString", str),
		esper.FieldDef("intPrimitive", i64),
		esper.FieldDef("boolPrimitive", b),
		esper.FieldDef("intBoxed", i64P),
		esper.FieldDef("charPrimitive", str),
		esper.FieldDef("longPrimitive", i64),
		esper.FieldDef("shortPrimitive", i16),
		esper.FieldDef("bytePrimitive", i8),
		esper.FieldDef("floatPrimitive", f32),
		esper.FieldDef("doublePrimitive", f64),
		esper.FieldDef("boolBoxed", bP),
		esper.FieldDef("charBoxed", strP),
		esper.FieldDef("longBoxed", i64P),
		esper.FieldDef("shortBoxed", i16P),
		esper.FieldDef("byteBoxed", i8P),
		esper.FieldDef("floatBoxed", f32P),
		esper.FieldDef("doubleBoxed", f64P),
		esper.FieldDef("bigDecimal", strP),
		esper.FieldDef("bigInteger", strP),
		esper.FieldDef("enumValue", strP),
	}
}

// infraNWProcessingOrderBeanSelections enumerates every SupportBean field as
// an Alias(name, Field) selection, mirroring Java's `select e.*` projection
// of the trigger event.
func infraNWProcessingOrderBeanSelections() []esper.Selection {
	field := func(name string) esper.Expr {
		return esper.Field[infraNWProcessingOrderBean, any](name)
	}
	return []esper.Selection{
		esper.Alias("theString", field("theString")),
		esper.Alias("intPrimitive", field("intPrimitive")),
		esper.Alias("boolPrimitive", field("boolPrimitive")),
		esper.Alias("intBoxed", field("intBoxed")),
		esper.Alias("charPrimitive", field("charPrimitive")),
		esper.Alias("longPrimitive", field("longPrimitive")),
		esper.Alias("shortPrimitive", field("shortPrimitive")),
		esper.Alias("bytePrimitive", field("bytePrimitive")),
		esper.Alias("floatPrimitive", field("floatPrimitive")),
		esper.Alias("doublePrimitive", field("doublePrimitive")),
		esper.Alias("boolBoxed", field("boolBoxed")),
		esper.Alias("charBoxed", field("charBoxed")),
		esper.Alias("longBoxed", field("longBoxed")),
		esper.Alias("shortBoxed", field("shortBoxed")),
		esper.Alias("byteBoxed", field("byteBoxed")),
		esper.Alias("floatBoxed", field("floatBoxed")),
		esper.Alias("doubleBoxed", field("doubleBoxed")),
		esper.Alias("bigDecimal", field("bigDecimal")),
		esper.Alias("bigInteger", field("bigInteger")),
		esper.Alias("enumValue", field("enumValue")),
	}
}

// infraNWProcessingOrderListened mirrors the Java oracle's LISTENED set:
// the irstream select in the dispatch cases and s0 in ordered-delete-select.
func infraNWProcessingOrderListened(statement string) bool {
	switch statement {
	case "select", "s0":
		return true
	}
	return false
}

// infraNWProcessingOrderCaseSteps pins the complete step sequence per
// case: deploys with byte-exact EPL, deployed markers, sends with
// representation-tagged payloads, and undeploy-all cleanup.
var infraNWProcessingOrderCaseSteps = map[string][]string{
	"dispatch-objectarray": {
		"deploy:schema-start:@EventRepresentation('objectarray') @buseventtype @public create schema StartValueEvent as (dummy string)",
		"deployed:schema-start",
		"deploy:schema-forward:@EventRepresentation('objectarray') @buseventtype @public create schema TestForwardEvent as (prop1 string)",
		"deployed:schema-forward",
		"deploy:schema-input:@EventRepresentation('objectarray') @buseventtype @public create schema TestInputEvent as (dummy string)",
		"deployed:schema-input",
		"deploy:insert-forward:insert into TestForwardEvent select'V1' as prop1 from TestInputEvent",
		"deployed:insert-forward",
		"deploy:create:@EventRepresentation('objectarray') @public create window NamedWin#unique(prop1) (prop1 string, prop2 string)",
		"deployed:create",
		"deploy:insert-window:insert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent",
		"deployed:insert-window",
		"deploy:update:on TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1'",
		"deployed:update",
		"deploy:select:@name('select') select irstream prop1, prop2 from NamedWin",
		"deployed:select",
		"send:StartValueEvent:{\"repr\":\"objectarray\",\"fields\":{\"dummy\":\"dummyValue\"}}",
		"send:TestInputEvent:{\"repr\":\"objectarray\",\"fields\":{\"dummy\":\"dummyValue\"}}",
		"undeploy-all:",
	},
	"dispatch-map": {
		"deploy:schema-start:@EventRepresentation('map') @buseventtype @public create schema StartValueEvent as (dummy string)",
		"deployed:schema-start",
		"deploy:schema-forward:@EventRepresentation('map') @buseventtype @public create schema TestForwardEvent as (prop1 string)",
		"deployed:schema-forward",
		"deploy:schema-input:@EventRepresentation('map') @buseventtype @public create schema TestInputEvent as (dummy string)",
		"deployed:schema-input",
		"deploy:insert-forward:insert into TestForwardEvent select'V1' as prop1 from TestInputEvent",
		"deployed:insert-forward",
		"deploy:create:@EventRepresentation('map') @public create window NamedWin#unique(prop1) (prop1 string, prop2 string)",
		"deployed:create",
		"deploy:insert-window:insert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent",
		"deployed:insert-window",
		"deploy:update:on TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1'",
		"deployed:update",
		"deploy:select:@name('select') select irstream prop1, prop2 from NamedWin",
		"deployed:select",
		"send:StartValueEvent:{\"repr\":\"map\",\"fields\":{\"dummy\":\"dummyValue\"}}",
		"send:TestInputEvent:{\"repr\":\"map\",\"fields\":{\"dummy\":\"dummyValue\"}}",
		"undeploy-all:",
	},
	"dispatch-avro": {
		"deploy:schema-start:@EventRepresentation('avro') @buseventtype @public create schema StartValueEvent as (dummy string)",
		"deployed:schema-start",
		"deploy:schema-forward:@EventRepresentation('avro') @buseventtype @public create schema TestForwardEvent as (prop1 string)",
		"deployed:schema-forward",
		"deploy:schema-input:@EventRepresentation('avro') @buseventtype @public create schema TestInputEvent as (dummy string)",
		"deployed:schema-input",
		"deploy:insert-forward:insert into TestForwardEvent select'V1' as prop1 from TestInputEvent",
		"deployed:insert-forward",
		"deploy:create:@EventRepresentation('avro') @public create window NamedWin#unique(prop1) (prop1 string, prop2 string)",
		"deployed:create",
		"deploy:insert-window:insert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent",
		"deployed:insert-window",
		"deploy:update:on TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1'",
		"deployed:update",
		"deploy:select:@name('select') select irstream prop1, prop2 from NamedWin",
		"deployed:select",
		"send:StartValueEvent:{\"repr\":\"avro\",\"fields\":{\"dummy\":\"dummyValue\"}}",
		"send:TestInputEvent:{\"repr\":\"avro\",\"fields\":{\"dummy\":\"dummyValue\"}}",
		"undeploy-all:",
	},
	"dispatch-json": {
		"deploy:schema-start:@EventRepresentation('json') @buseventtype @public create schema StartValueEvent as (dummy string)",
		"deployed:schema-start",
		"deploy:schema-forward:@EventRepresentation('json') @buseventtype @public create schema TestForwardEvent as (prop1 string)",
		"deployed:schema-forward",
		"deploy:schema-input:@EventRepresentation('json') @buseventtype @public create schema TestInputEvent as (dummy string)",
		"deployed:schema-input",
		"deploy:insert-forward:insert into TestForwardEvent select'V1' as prop1 from TestInputEvent",
		"deployed:insert-forward",
		"deploy:create:@EventRepresentation('json') @public create window NamedWin#unique(prop1) (prop1 string, prop2 string)",
		"deployed:create",
		"deploy:insert-window:insert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent",
		"deployed:insert-window",
		"deploy:update:on TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1'",
		"deployed:update",
		"deploy:select:@name('select') select irstream prop1, prop2 from NamedWin",
		"deployed:select",
		"send:StartValueEvent:{\"repr\":\"json\",\"fields\":{\"dummy\":\"dummyValue\"}}",
		"send:TestInputEvent:{\"repr\":\"json\",\"fields\":{\"dummy\":\"dummyValue\"}}",
		"undeploy-all:",
	},
	"dispatch-json-provided": {
		"deploy:schema-start:@JsonSchema(className='com.espertech.esper.regressionlib.suite.infra.namedwindow.InfraNamedWindowProcessingOrder$MyLocalJsonProvidedStartValueEvent') @EventRepresentation('json') @buseventtype @public create schema StartValueEvent as (dummy string)",
		"deployed:schema-start",
		"deploy:schema-forward:@JsonSchema(className='com.espertech.esper.regressionlib.suite.infra.namedwindow.InfraNamedWindowProcessingOrder$MyLocalJsonProvidedTestForwardEvent') @EventRepresentation('json') @buseventtype @public create schema TestForwardEvent as (prop1 string)",
		"deployed:schema-forward",
		"deploy:schema-input:@JsonSchema(className='com.espertech.esper.regressionlib.suite.infra.namedwindow.InfraNamedWindowProcessingOrder$MyLocalJsonProvidedTestInputEvent') @EventRepresentation('json') @buseventtype @public create schema TestInputEvent as (dummy string)",
		"deployed:schema-input",
		"deploy:insert-forward:insert into TestForwardEvent select'V1' as prop1 from TestInputEvent",
		"deployed:insert-forward",
		"deploy:create:@JsonSchema(className='com.espertech.esper.regressionlib.suite.infra.namedwindow.InfraNamedWindowProcessingOrder$MyLocalJsonProvidedNamedWin') @EventRepresentation('json') @public create window NamedWin#unique(prop1) (prop1 string, prop2 string)",
		"deployed:create",
		"deploy:insert-window:insert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent",
		"deployed:insert-window",
		"deploy:update:on TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1'",
		"deployed:update",
		"deploy:select:@name('select') select irstream prop1, prop2 from NamedWin",
		"deployed:select",
		"send:StartValueEvent:{\"repr\":\"json-provided\",\"fields\":{\"dummy\":\"dummyValue\"}}",
		"send:TestInputEvent:{\"repr\":\"json-provided\",\"fields\":{\"dummy\":\"dummyValue\"}}",
		"undeploy-all:",
	},
	"dispatch-default": {
		"deploy:schema-start: @buseventtype @public create schema StartValueEvent as (dummy string)",
		"deployed:schema-start",
		"deploy:schema-forward: @buseventtype @public create schema TestForwardEvent as (prop1 string)",
		"deployed:schema-forward",
		"deploy:schema-input: @buseventtype @public create schema TestInputEvent as (dummy string)",
		"deployed:schema-input",
		"deploy:insert-forward:insert into TestForwardEvent select'V1' as prop1 from TestInputEvent",
		"deployed:insert-forward",
		"deploy:create: @public create window NamedWin#unique(prop1) (prop1 string, prop2 string)",
		"deployed:create",
		"deploy:insert-window:insert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent",
		"deployed:insert-window",
		"deploy:update:on TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1'",
		"deployed:update",
		"deploy:select:@name('select') select irstream prop1, prop2 from NamedWin",
		"deployed:select",
		"send:StartValueEvent:{\"repr\":\"default\",\"fields\":{\"dummy\":\"dummyValue\"}}",
		"send:TestInputEvent:{\"repr\":\"default\",\"fields\":{\"dummy\":\"dummyValue\"}}",
		"undeploy-all:",
	},
	"ordered-delete-select": {
		"deploy:create:create window MyWindow#lastevent as select * from SupportBean",
		"deployed:create",
		"deploy:insert:insert into MyWindow select * from SupportBean",
		"deployed:insert",
		"deploy:delete7:on MyWindow e delete from MyWindow win where win.theString=e.theString and e.intPrimitive = 7",
		"deployed:delete7",
		"deploy:delete5:on MyWindow e delete from MyWindow win where win.theString=e.theString and e.intPrimitive = 5",
		"deployed:delete5",
		"deploy:oninsert:on MyWindow e insert into ResultStream select e.* from MyWindow",
		"deployed:oninsert",
		"deploy:s0:@name('s0') select * from ResultStream",
		"deployed:s0",
		"send:SupportBean:{\"theString\":\"E1\",\"intPrimitive\":7}",
		"send:SupportBean:{\"theString\":\"E2\",\"intPrimitive\":8}",
		"send:SupportBean:{\"theString\":\"E3\",\"intPrimitive\":5}",
		"send:SupportBean:{\"theString\":\"E4\",\"intPrimitive\":6}",
		"undeploy-all:",
	},
}

func executeInfraNWProcessingOrder(ctx context.Context, steps []compat.Step) (compat.Trace, error) {
	caseName := ""
	caseIndex := -1
	var env *esper.Environment
	var engine *esper.Engine
	deployments := map[string]*esper.Deployment{}
	statements := map[string]*esper.Statement{}
	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWProcessingOrderID}
	record := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		rec := compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		}
		if len(rec.Old) == 0 {
			rec.Old = nil
		}
		trace.Records = append(trace.Records, rec)
	}
	startCase := func() error {
		env = esper.NewEnvironment()
		if _, err := esper.RegisterStruct[infraNWProcessingOrderBean](env, "SupportBean"); err != nil {
			return err
		}
		engine = esper.NewEngine(env,
			esper.WithRuntimeURI(infraNWProcessingOrderJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(time.Unix(0, 0).UTC()))
		return nil
	}
	defer func() {
		if engine != nil {
			_ = engine.Close(context.Background())
		}
	}()

	for _, step := range steps {
		switch step.Op {
		case "case":
			caseName = step.Case
			caseIndex++
			sequence = map[string]uint64{}
			deployments = map[string]*esper.Deployment{}
			statements = map[string]*esper.Statement{}
			if err := startCase(); err != nil {
				return compat.Trace{}, err
			}
		case "deploy":
			plan, err := infraNWProcessingOrderBuild(env, caseName, step.Statement, step.Epl)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q/%q: %w", caseName, step.Statement, err)
			}
			// Schema statements register env-level and produce no plan; Java
			// deploys them as statements but they carry no runtime behavior.
			if plan.Hash() == "" {
				continue
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q/%q: %w", caseName, step.Statement, err)
			}
			deployments[step.Statement] = deployment
			for _, statement := range deployment.Statements() {
				statements[step.Statement] = statement
				if infraNWProcessingOrderListened(statement.Name()) {
					name := statement.Name()
					if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record(name, batch)
						return nil
					}); err != nil {
						return compat.Trace{}, err
					}
				}
			}
		case "deployed":
			sequence[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequence[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(engine.Now()),
			})
		case "send":
			send, err := decodeInfraNWProcessingOrderPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := infraNWProcessingOrderSend(ctx, env, engine, step.EventType, send); err != nil {
				return compat.Trace{}, fmt.Errorf("send %s: %w", step.EventType, err)
			}
		case "undeploy":
			deployment, ok := deployments[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("undeploy targets unknown statement %q", step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return compat.Trace{}, err
			}
			delete(deployments, step.Statement)
			delete(statements, step.Statement)
		case "undeploy-all":
			for name, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, fmt.Errorf("undeploy-all %q: %w", name, err)
				}
			}
			deployments = map[string]*esper.Deployment{}
			statements = map[string]*esper.Statement{}
		default:
			return compat.Trace{}, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}

func infraNWProcessingOrderSend(ctx context.Context, env *esper.Environment, engine *esper.Engine, eventType string, send infraNWProcessingOrderSendSpec) error {
	if eventType == "SupportBean" {
		// Java's `new SupportBean()` leaves charPrimitive at the char default
		// '\u0000', which the trace serializer emits as "\u0000".
		bean := infraNWProcessingOrderBean{CharPrimitive: "\u0000"}
		for key, raw := range send.fields {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode %s.%s: %w", eventType, key, err)
			}
			switch key {
			case "theString":
				bean.TheString = stringField(value)
			case "intPrimitive":
				bean.IntPrimitive = int64Field(value)
			}
		}
		return engine.Send(ctx, eventType, bean)
	}
	dummy := ""
	if raw, ok := send.fields["dummy"]; ok {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("decode %s.dummy: %w", eventType, err)
		}
		dummy = stringField(value)
	}
	switch send.repr {
	case "objectarray":
		return engine.SendObjectArray(ctx, eventType, []any{dummy})
	case "map", "default":
		return engine.Send(ctx, eventType, map[string]any{})
	case "avro":
		schema, ok := env.Schema(eventType)
		if !ok {
			return fmt.Errorf("%s schema not registered", eventType)
		}
		record, err := esper.NewAvroRecordFromMap(schema, nil)
		if err != nil {
			return err
		}
		return engine.SendAvro(ctx, eventType, record)
	case "json", "json-provided":
		return engine.SendJSON(ctx, eventType, []byte("{}"))
	default:
		return fmt.Errorf("unknown representation %q", send.repr)
	}
}

func runInfraNWProcessingOrderScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	return executeInfraNWProcessingOrder(ctx, scenario.Steps)
}

func loadInfraNWProcessingOrderScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWProcessingOrderID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWProcessingOrderID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWProcessingOrderID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWProcessingOrderID, err)
	}
	if err := requireInfraNWProcessingOrderFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWProcessingOrderID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWProcessingOrderID ||
		metadata.Description != infraNWProcessingOrderDescription ||
		metadata.JavaCommit != infraNWProcessingOrderJavaCommit ||
		metadata.JavaSource != infraNWProcessingOrderSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWProcessingOrderID)
	}
	if err := validateInfraNWProcessingOrderStringArray(root["javaRuntimes"], infraNWProcessingOrderJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWProcessingOrderStringArray(root["javaNames"], infraNWProcessingOrderJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWProcessingOrderStringArray(root["javaStaticIds"], infraNWProcessingOrderJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWProcessingOrderStringArray(root["javaFlags"], infraNWProcessingOrderJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWProcessingOrderCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWProcessingOrderID, len(infraNWProcessingOrderCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWProcessingOrderFields(object,
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
		if definition.Case != infraNWProcessingOrderCases[index] ||
			definition.Ordinal != infraNWProcessingOrderOrdinals[index] ||
			definition.RuntimeID != infraNWProcessingOrderJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWProcessingOrderJavaExecutions[index] ||
			definition.Observation != infraNWProcessingOrderCaseObservations[index] ||
			definition.EPL != infraNWProcessingOrderCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWProcessingOrderID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWProcessingOrderID)
	}
	steps := make([]compat.Step, len(rawSteps))
	operations := make([]string, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		operations[index] = operation
		switch operation {
		case "case":
			if err := requireInfraNWProcessingOrderFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWProcessingOrderFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWProcessingOrderFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWProcessingOrderFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWProcessingOrderPayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy":
			if err := requireInfraNWProcessingOrderFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWProcessingOrderFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWProcessingOrderRawSteps(rawSteps, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func requireInfraNWProcessingOrderFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has unexpected fields")
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func validateInfraNWProcessingOrderStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if values[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

func validateInfraNWProcessingOrderRawSteps(rawSteps []json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWProcessingOrderCases {
		want, ok := infraNWProcessingOrderCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWProcessingOrderID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWProcessingOrderID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s case %q does not start with a case marker", infraNWProcessingOrderID, caseName)
		}
		offset++
		for _, pinned := range want {
			var step struct {
				Op        string          `json:"op"`
				Case      string          `json:"case"`
				Statement string          `json:"statement"`
				EventType string          `json:"eventType"`
				Epl       string          `json:"epl"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawSteps[offset], &step); err != nil {
				return fmt.Errorf("%s step %d: %w", infraNWProcessingOrderID, offset, err)
			}
			key := step.Op + ":" + step.Statement + step.EventType
			if step.Op == "send" {
				var compacted bytes.Buffer
				if err := json.Compact(&compacted, step.Payload); err != nil {
					return fmt.Errorf("%s step %d payload: %w", infraNWProcessingOrderID, offset, err)
				}
				key += ":" + compacted.String()
			}
			if step.Op == "deploy" {
				key += ":" + step.Epl
			}
			if key != pinned {
				return fmt.Errorf("%s case %q step %d = %q, want %q", infraNWProcessingOrderID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s has %d trailing steps", infraNWProcessingOrderID, len(rawSteps)-offset)
	}
	return nil
}
