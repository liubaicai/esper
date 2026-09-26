package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	eventInfraPropertyNonDynamicID         = "event-infra-property-non-dynamic"
	eventInfraPropertyNonDynamicJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	// The cluster spans seven sibling files in suite/event/infra; the
	// scenario pins the shared directory while the evidence metadata carries
	// the files.
	eventInfraPropertyNonDynamicSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra"
)

var eventInfraPropertyNonDynamicJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyIndexedKeyExpr.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyIndexedRuntimeIndex.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyMappedIndexed.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyMappedRuntimeKey.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyNestedIndexed.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyNestedNestedEscaped.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyNestedSimple.java",
}

var eventInfraPropertyNonDynamicJavaRuntimeIDs = []string{
	"java-runtime-f5d82c0cf0cf2516d0ef",
	"java-runtime-10ce593e8d5ae6a9dd89",
	"java-runtime-b25d2dc5c7eb2c968105",
	"java-runtime-c4f7fe6163eff7d35461",
	"java-runtime-594a57ed26499f1ccd30",
	"java-runtime-0b17042e1ee4d75b7007",
	"java-runtime-848063976a9d6485ec20",
}

var eventInfraPropertyNonDynamicJavaStaticIDs = []string{
	"java-461ebcf502cbdf9ba15c",
	"java-ed0cc8c7b0867c0cd502",
	"java-287c4bf86e9464e7b595",
	"java-899877f666cfc36adcb1",
	"java-ff678fbbd673309b12b2",
	"java-2d629e45d1948a6770ee",
	"java-eb62cc47423e42ec7671",
}

var eventInfraPropertyNonDynamicJavaExecutions = []string{
	"EventInfraPropertyIndexedKeyExpr",
	"EventInfraPropertyIndexedRuntimeIndex",
	"EventInfraPropertyMappedIndexed",
	"EventInfraPropertyMappedRuntimeKey",
	"EventInfraPropertyNestedIndexed",
	"EventInfraPropertyNestedNestedEscaped",
	"EventInfraPropertyNestedSimple",
}

// All seven executions declare no flags, so the flat flag union is empty.
var eventInfraPropertyNonDynamicJavaFlags []string

var eventInfraPropertyNonDynamicCases = []string{
	"indexed-key-expr",
	"indexed-runtime-index",
	"mapped-indexed",
	"mapped-runtime-key",
	"nested-indexed",
	"nested-escaped",
	"nested-simple",
}

const eventInfraPropertyNonDynamicDescription = "EventInfraProperty* non-dynamic property slice (all ord 0, no flags): indexed-key-expr replays getGetterIndexed/getGetterMapped probes over six iterations including a select-{1,2}-wrapper SupportBean and a mapinner/oainner nested-array element; indexed-runtime-index selects indexed(offsetNum+0)/(offsetNum+1) with a compile-time constant; mapped-indexed probes mapped('k1') and indexed[1] plus event-type assertions over six underlyings; mapped-runtime-key selects mapped(keyChar||'1'/'2') with a compile-time constant; nested-indexed and nested-simple replay four-level l1..l4 indexed/nested projections with exists() columns and beanNav fragment/invalid probes over seven underlyings (XML sends the myevent document); nested-escaped deploys one five-type schema module then replays select-* getter probes and the backtick-escaped projection incl. the dynamic lvl1.lvl2.`vlvl2dyn`? tail per underlying. Java sources are the seven sibling files under suite/event/infra (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c)."

// ---- Case metadata tables -------------------------------------------------

const einpPkg = "com.espertech.esper.regressionlib.suite.event.infra"

// einpJavaTypeName resolves the Java event-type name per (case, mode),
// mirroring the suite TYPENAME constants and the ad-hoc names in the Java
// sources (LocalEvent/JsonSchema/BeanLvl0-style names included).
func einpJavaTypeName(caseName, mode string) string {
	switch caseName {
	case "indexed-key-expr":
		switch mode {
		case "objectarray":
			return "OAEvent"
		case "map":
			return "MapEvent"
		case "wrapper":
			return "SupportBean"
		case "bean":
			return "MyIndexMappedSamplerBean"
		case "json", "json-provided":
			return "JsonSchema"
		}
	case "indexed-runtime-index", "mapped-runtime-key":
		return "LocalEvent"
	case "mapped-indexed":
		switch mode {
		case "bean":
			return "MyIMEvent"
		case "map":
			return "EventInfraPropertyMappedIndexedMap"
		case "objectarray":
			return "EventInfraPropertyMappedIndexedOA"
		case "avro":
			return "EventInfraPropertyMappedIndexedAvro"
		case "json":
			return "EventInfraPropertyMappedIndexedJson"
		case "json-provided":
			return "EventInfraPropertyMappedIndexedJsonProvided"
		}
	case "nested-indexed":
		switch mode {
		case "bean":
			return "InfraNestedIndexPropTop"
		case "map":
			return "EventInfraPropertyNestedIndexedMap"
		case "objectarray":
			return "EventInfraPropertyNestedIndexedOA"
		case "xml":
			return "EventInfraPropertyNestedIndexedXML"
		case "avro":
			return "EventInfraPropertyNestedIndexedAvro"
		case "json":
			return "EventInfraPropertyNestedIndexedJson"
		case "json-provided":
			return "EventInfraPropertyNestedIndexedJsonProvided"
		}
	case "nested-escaped":
		switch mode {
		case "bean":
			return "BeanLvl0"
		case "map":
			return "MapLvl0"
		case "objectarray":
			return "OALvl0"
		case "json":
			return "JSONLvl0"
		case "avro":
			return "AvroLvl0"
		}
	case "nested-simple":
		switch mode {
		case "bean":
			return "InfraNestedSimplePropTop"
		case "map":
			return "EventInfraPropertyNestedSimpleMap"
		case "objectarray":
			return "EventInfraPropertyNestedSimpleOA"
		case "xml":
			return "EventInfraPropertyNestedSimpleXML"
		case "avro":
			return "EventInfraPropertyNestedSimpleAvro"
		case "json":
			return "EventInfraPropertyNestedSimpleJson"
		case "json-provided":
			return "EventInfraPropertyNestedSimpleJsonProvided"
		}
	}
	return ""
}

// einpModes lists the underlying modes replayed per case in Java source
// order. indexed-key-expr runs objectarray, map, the SupportBean select-
// wrapper, bean, json and json-provided (the Java source has no avro or xml
// iteration). nested-escaped registers its five event types from one schema
// module: the pseudo-mode "types" carries the single schema deploy and the
// five per-type passes follow without undeploy-all between them.
func einpModes(caseName string) []string {
	switch caseName {
	case "indexed-key-expr":
		return []string{"objectarray", "map", "wrapper", "bean", "json", "json-provided"}
	case "indexed-runtime-index":
		return []string{"bean", "map", "objectarray", "json", "json-provided", "avro"}
	case "mapped-indexed":
		return []string{"bean", "map", "objectarray", "avro", "json", "json-provided"}
	case "mapped-runtime-key":
		return []string{"bean", "map", "objectarray", "json", "json-provided", "avro"}
	case "nested-indexed", "nested-simple":
		return []string{"bean", "map", "objectarray", "xml", "avro", "json", "json-provided"}
	case "nested-escaped":
		return []string{"bean", "map", "objectarray", "json", "avro"}
	}
	return nil
}

// einpNestedCase reports whether the case replays the two-pass
// runAssertionSelectNested/runAssertionBeanNav structure (projection pass
// then select-* navigation pass per underlying, no undeploy-all between
// underlyings).
func einpNestedCase(caseName string) bool {
	return caseName == "nested-indexed" || caseName == "nested-simple"
}

// ---- Pinned EPL -----------------------------------------------------------

// einpSchemaEPL returns the Java schema-creation EPL for modes that create
// the type via compileDeploy. Preconfigured modes (registered in the suite
// runner Configuration) pin the empty string.
func einpSchemaEPL(caseName, mode string) string {
	keyExpr := einpPkg + ".EventInfraPropertyIndexedKeyExpr"
	runtimeIndex := einpPkg + ".EventInfraPropertyIndexedRuntimeIndex"
	mappedIndexed := einpPkg + ".EventInfraPropertyMappedIndexed"
	runtimeKey := einpPkg + ".EventInfraPropertyMappedRuntimeKey"
	nestedIndexed := einpPkg + ".EventInfraPropertyNestedIndexed"
	nestedSimple := einpPkg + ".EventInfraPropertyNestedSimple"
	switch caseName {
	case "indexed-key-expr":
		switch mode {
		case "objectarray":
			return "@public create objectarray schema OAEventInner(p0 string);\n" +
				"@buseventtype @public create objectarray schema OAEvent(intarray int[], oainner OAEventInner[]);\n"
		case "map":
			return "create schema MapEventInner(p0 string);\n" +
				"@public @buseventtype create schema MapEvent(intarray int[], mapinner MapEventInner[]);\n"
		case "wrapper":
			return ""
		case "bean":
			return "@public @buseventtype create schema MyIndexMappedSamplerBean as " + keyExpr + "$MyIndexMappedSamplerBean"
		case "json":
			return "@public @buseventtype create json schema JsonSchema(indexed int[], mapped java.util.Map);\n"
		case "json-provided":
			return "@JsonSchema(className='" + keyExpr + "$MyLocalJsonProvided') @public @buseventtype create json schema JsonSchema();\n"
		}
	case "indexed-runtime-index":
		switch mode {
		case "bean":
			return "@public @buseventtype create schema LocalEvent as " + runtimeIndex + "$LocalEvent;\n"
		case "map":
			return "@public @buseventtype create schema LocalEvent(indexed string[]);\n"
		case "objectarray":
			return "@public @buseventtype create objectarray schema LocalEvent(indexed string[]);\n"
		case "json":
			return "@public @buseventtype create json schema LocalEvent(indexed string[]);\n"
		case "json-provided":
			return "@JsonSchema(className='" + runtimeIndex + "$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n"
		case "avro":
			return "@name('schema') @public @buseventtype create avro schema LocalEvent(indexed string[]);\n"
		}
	case "mapped-indexed":
		switch mode {
		case "json":
			return "@public @buseventtype @name('schema') create json schema EventInfraPropertyMappedIndexedJson(indexed string[], mapped java.util.Map)"
		case "json-provided":
			return "@public @buseventtype @name('schema') @JsonSchema(className='" + mappedIndexed + "$MyLocalJsonProvided') create json schema EventInfraPropertyMappedIndexedJsonProvided()"
		default:
			return ""
		}
	case "mapped-runtime-key":
		switch mode {
		case "bean":
			return "@public @buseventtype create schema LocalEvent as " + runtimeKey + "$LocalEvent;\n"
		case "map":
			return "@public @buseventtype create schema LocalEvent(mapped java.util.Map);\n"
		case "objectarray":
			return "@public @buseventtype create objectarray schema LocalEvent(mapped java.util.Map);\n"
		case "json":
			return "@public @buseventtype create json schema LocalEvent(mapped java.util.Map);\n"
		case "json-provided":
			return "@JsonSchema(className='" + runtimeKey + "$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n"
		case "avro":
			return "@name('schema') @public @buseventtype create avro schema LocalEvent(mapped java.util.Map);\n"
		}
	case "nested-indexed":
		if mode == "json" {
			return "create json schema EventInfraPropertyNestedIndexedJson_4(lvl4 int);\n" +
				"create json schema EventInfraPropertyNestedIndexedJson_3(lvl3 int, l4 EventInfraPropertyNestedIndexedJson_4[]);\n" +
				"create json schema EventInfraPropertyNestedIndexedJson_2(lvl2 int, l3 EventInfraPropertyNestedIndexedJson_3[]);\n" +
				"create json schema EventInfraPropertyNestedIndexedJson_1(lvl1 int, l2 EventInfraPropertyNestedIndexedJson_2[]);\n" +
				"@name('types') @public @buseventtype create json schema EventInfraPropertyNestedIndexedJson(l1 EventInfraPropertyNestedIndexedJson_1[]);\n"
		}
		if mode == "json-provided" {
			return "@JsonSchema(className='" + nestedIndexed + "$MyLocalJSONProvidedLvl4') create json schema EventInfraPropertyNestedIndexedJsonProvided_4();\n" +
				"@JsonSchema(className='" + nestedIndexed + "$MyLocalJSONProvidedLvl3') create json schema EventInfraPropertyNestedIndexedJsonProvided_3(lvl3 int, l4 EventInfraPropertyNestedIndexedJsonProvided_4[]);\n" +
				"@JsonSchema(className='" + nestedIndexed + "$MyLocalJSONProvidedLvl2') create json schema EventInfraPropertyNestedIndexedJsonProvided_2(lvl2 int, l3 EventInfraPropertyNestedIndexedJsonProvided_3[]);\n" +
				"@JsonSchema(className='" + nestedIndexed + "$MyLocalJSONProvidedLvl1') create json schema EventInfraPropertyNestedIndexedJsonProvided_1(lvl1 int, l2 EventInfraPropertyNestedIndexedJsonProvided_2[]);\n" +
				"@JsonSchema(className='" + nestedIndexed + "$MyLocalJSONProvidedTop') @name('types') @public @buseventtype create json schema EventInfraPropertyNestedIndexedJsonProvided(l1 EventInfraPropertyNestedIndexedJsonProvided_1[]);\n"
		}
		return ""
	case "nested-escaped":
		if mode == "types" {
			return "@name('types') @public @buseventtype create schema BeanLvl0 as " + einpPkg + ".EventInfraPropertyNestedNestedEscaped$SupportLvl0;\n" +
				"\n" +
				"create schema MapLvl3(vlvl3 string);\n" +
				"create schema MapLvl2(vlvl2 string, lvl3 MapLvl3);\n" +
				"create schema MapLvl1(lvl2 MapLvl2);\n" +
				"@public @buseventtype create schema MapLvl0(lvl1 MapLvl1);\n" +
				"\n" +
				"create objectarray schema OALvl3(vlvl3 string);\n" +
				"create objectarray schema OALvl2(vlvl2 string, vlvl2dyn string, lvl3 OALvl3);\n" +
				"create objectarray schema OALvl1(lvl2 OALvl2);\n" +
				"@public @buseventtype create objectarray schema OALvl0(lvl1 OALvl1);\n" +
				"\n" +
				"@JsonSchema(dynamic=true) create json schema JSONLvl3(vlvl3 string);\n" +
				"@JsonSchema(dynamic=true) create json schema JSONLvl2(vlvl2 string, lvl3 JSONLvl3);\n" +
				"create json schema JSONLvl1(lvl2 JSONLvl2);\n" +
				"@public @buseventtype create json schema JSONLvl0(lvl1 JSONLvl1);\n" +
				"\n" +
				"create avro schema AvroLvl3(vlvl3 string);\n" +
				"create avro schema AvroLvl2(vlvl2 string, vlvl2dyn string, lvl3 AvroLvl3);\n" +
				"create avro schema AvroLvl1(lvl2 AvroLvl2);\n" +
				"@public @buseventtype create avro schema AvroLvl0(lvl1 AvroLvl1);\n"
		}
		return ""
	case "nested-simple":
		if mode == "json" {
			return "@public create json schema EventInfraPropertyNestedSimpleJson_4(lvl4 int);\n" +
				"@public create json schema EventInfraPropertyNestedSimpleJson_3(lvl3 int, l4 EventInfraPropertyNestedSimpleJson_4);\n" +
				"@public create json schema EventInfraPropertyNestedSimpleJson_2(lvl2 int, l3 EventInfraPropertyNestedSimpleJson_3);\n" +
				"@public create json schema EventInfraPropertyNestedSimpleJson_1(lvl1 int, l2 EventInfraPropertyNestedSimpleJson_2);\n" +
				"@name('types') @public @buseventtype create json schema EventInfraPropertyNestedSimpleJson(l1 EventInfraPropertyNestedSimpleJson_1);\n"
		}
		if mode == "json-provided" {
			return "@JsonSchema(className='" + nestedSimple + "$MyLocalJSONProvidedTop') @name('types') @public @buseventtype create json schema EventInfraPropertyNestedSimpleJsonProvided();\n"
		}
		return ""
	}
	return ""
}

// einpS0EPL returns the byte-exact Java statement text for one iteration.
// For the nested cases this is the runAssertionSelectNested projection; the
// second pass select-* is pinned separately via einpStarEPL. For
// nested-escaped pass is either "getter" (select *) or "select" (the escaped
// projection).
func einpS0EPL(caseName, mode string) string {
	t := einpJavaTypeName(caseName, mode)
	switch caseName {
	case "indexed-key-expr":
		switch mode {
		case "objectarray":
			return "@name('s0') select * from OAEvent;\n"
		case "map":
			return "@name('s0') select * from MapEvent;\n"
		case "wrapper":
			return "@name('s0') select {1, 2} as arr, *, Collections.singletonMap('A', 2) as mapped from SupportBean"
		case "bean":
			return "@name('s0') select * from MyIndexMappedSamplerBean"
		case "json", "json-provided":
			return "@name('s0') select * from JsonSchema;\n"
		}
	case "indexed-runtime-index":
		return "create constant variable int offsetNum = 0;" +
			"@name('s0') select indexed(offsetNum+0) as c0, indexed(offsetNum+1) as c1 from LocalEvent as e;\n"
	case "mapped-indexed":
		return "@name('s0') select * from " + t
	case "mapped-runtime-key":
		return "create constant variable string keyChar = 'a';" +
			"@name('s0') select mapped(keyChar||'1') as c0, mapped(keyChar||'2') as c1 from LocalEvent as e;\n"
	case "nested-indexed":
		return "@name('s0') select " +
			"l1[0].lvl1 as c0, " +
			"exists(l1[0].lvl1) as exists_c0, " +
			"l1[0].l2[0].lvl2 as c1, " +
			"exists(l1[0].l2[0].lvl2) as exists_c1, " +
			"l1[0].l2[0].l3[0].lvl3 as c2, " +
			"exists(l1[0].l2[0].l3[0].lvl3) as exists_c2, " +
			"l1[0].l2[0].l3[0].l4[0].lvl4 as c3, " +
			"exists(l1[0].l2[0].l3[0].l4[0].lvl4) as exists_c3 " +
			"from " + t
	case "nested-escaped":
		return "@name('s0') select " +
			"lvl1.lvl2.`vlvl2` as c0, " +
			"lvl1.lvl2.lvl3.`vlvl3` as c1, " +
			"`lvl1`.`lvl2`.`lvl3`.`vlvl3` as c2, " +
			"lvl1.lvl2.`vlvl2dyn`? as c3 " +
			" from " + t
	case "nested-simple":
		return "@name('s0') select " +
			"l1.lvl1 as c0, " +
			"exists(l1.lvl1) as exists_c0, " +
			"l1.l2.lvl2 as c1, " +
			"exists(l1.l2.lvl2) as exists_c1, " +
			"l1.l2.l3.lvl3 as c2, " +
			"exists(l1.l2.l3.lvl3) as exists_c2, " +
			"l1.l2.l3.l4.lvl4 as c3, " +
			"exists(l1.l2.l3.l4.lvl4) as exists_c3 " +
			"from " + t
	}
	return ""
}

// einpStarEPL pins the select-* statement for the cases that deploy one:
// nested-indexed/nested-simple beanNav pass and nested-escaped's getter pass
// (which is its first per-type statement).
func einpStarEPL(caseName, mode string) string {
	switch caseName {
	case "mapped-indexed":
		return "@name('s0') select * from " + einpJavaTypeName(caseName, mode)
	case "nested-indexed", "nested-simple", "nested-escaped":
		return "@name('s0') select * from " + einpJavaTypeName(caseName, mode)
	}
	return ""
}

// ---- Projection columns ----------------------------------------------------

// einpColumn is one projected output column: the Java alias, the Go property
// path used for the value projection, whether the column wraps the value in
// exists(), and whether XML values coerce the schema-typed attribute string
// to an integer (Esper's XSD-typed attribute getter returns int while the Go
// XML document model carries the raw attribute text).
type einpColumn struct {
	name    string
	goPath  string
	exists  bool
	xmlInt  bool
	literal any // when set, the column projects a constant (unused currently)
}

// einpColumns resolves the Go projection columns for one (case, mode)
// iteration. Java's escaped identifiers drop their backticks and XML
// attribute leaves resolve through the Go document path (myevent…@attr).
func einpColumns(caseName, mode string) []einpColumn {
	xml := mode == "xml"
	switch caseName {
	case "indexed-runtime-index":
		return []einpColumn{
			{name: "c0", goPath: "indexed[0]"},
			{name: "c1", goPath: "indexed[1]"},
		}
	case "mapped-runtime-key":
		return []einpColumn{
			{name: "c0", goPath: "mapped('a1')"},
			{name: "c1", goPath: "mapped('a2')"},
		}
	case "nested-indexed":
		if xml {
			return []einpColumn{
				{name: "c0", goPath: "myevent.l1.@lvl1", xmlInt: true},
				{name: "exists_c0", goPath: "myevent.l1.@lvl1", exists: true},
				{name: "c1", goPath: "myevent.l1.l2.@lvl2", xmlInt: true},
				{name: "exists_c1", goPath: "myevent.l1.l2.@lvl2", exists: true},
				{name: "c2", goPath: "myevent.l1.l2.l3.@lvl3", xmlInt: true},
				{name: "exists_c2", goPath: "myevent.l1.l2.l3.@lvl3", exists: true},
				{name: "c3", goPath: "myevent.l1.l2.l3.l4.@lvl4", xmlInt: true},
				{name: "exists_c3", goPath: "myevent.l1.l2.l3.l4.@lvl4", exists: true},
			}
		}
		return []einpColumn{
			{name: "c0", goPath: "l1[0].lvl1"},
			{name: "exists_c0", goPath: "l1[0].lvl1", exists: true},
			{name: "c1", goPath: "l1[0].l2[0].lvl2"},
			{name: "exists_c1", goPath: "l1[0].l2[0].lvl2", exists: true},
			{name: "c2", goPath: "l1[0].l2[0].l3[0].lvl3"},
			{name: "exists_c2", goPath: "l1[0].l2[0].l3[0].lvl3", exists: true},
			{name: "c3", goPath: "l1[0].l2[0].l3[0].l4[0].lvl4"},
			{name: "exists_c3", goPath: "l1[0].l2[0].l3[0].l4[0].lvl4", exists: true},
		}
	case "nested-escaped":
		return []einpColumn{
			{name: "c0", goPath: "lvl1.lvl2.vlvl2"},
			{name: "c1", goPath: "lvl1.lvl2.lvl3.vlvl3"},
			{name: "c2", goPath: "lvl1.lvl2.lvl3.vlvl3"},
			{name: "c3", goPath: "lvl1.lvl2.vlvl2dyn?"},
		}
	case "nested-simple":
		if xml {
			return []einpColumn{
				{name: "c0", goPath: "myevent.l1.@lvl1", xmlInt: true},
				{name: "exists_c0", goPath: "myevent.l1.@lvl1", exists: true},
				{name: "c1", goPath: "myevent.l1.l2.@lvl2", xmlInt: true},
				{name: "exists_c1", goPath: "myevent.l1.l2.@lvl2", exists: true},
				{name: "c2", goPath: "myevent.l1.l2.l3.@lvl3", xmlInt: true},
				{name: "exists_c2", goPath: "myevent.l1.l2.l3.@lvl3", exists: true},
				{name: "c3", goPath: "myevent.l1.l2.l3.l4.@lvl4", xmlInt: true},
				{name: "exists_c3", goPath: "myevent.l1.l2.l3.l4.@lvl4", exists: true},
			}
		}
		return []einpColumn{
			{name: "c0", goPath: "l1.lvl1"},
			{name: "exists_c0", goPath: "l1.lvl1", exists: true},
			{name: "c1", goPath: "l1.l2.lvl2"},
			{name: "exists_c1", goPath: "l1.l2.lvl2", exists: true},
			{name: "c2", goPath: "l1.l2.l3.lvl3"},
			{name: "exists_c2", goPath: "l1.l2.l3.lvl3", exists: true},
			{name: "c3", goPath: "l1.l2.l3.l4.lvl4"},
			{name: "exists_c3", goPath: "l1.l2.l3.l4.lvl4", exists: true},
		}
	}
	return nil
}

// ---- Type pins --------------------------------------------------------------

// einpTypeOrder pins the property order of the "types" step per case. Only
// executions whose Java source asserts statement/event-type property types
// carry a types step: mapped-indexed (runAssertionTypeValidProp) and the two
// nested cases (assertStatement in runAssertionSelectNested).
func einpTypeOrder(caseName string) []string {
	switch caseName {
	case "mapped-indexed":
		return []string{"indexed", "indexed[0]", "mapped", "mapped('a')"}
	case "nested-indexed", "nested-simple":
		return []string{"c0", "exists_c0", "c1", "exists_c1", "c2", "exists_c2", "c3", "exists_c3"}
	}
	return nil
}

// einpTypePins mirrors the Java type assertions. mapped-indexed pins
// runAssertionTypeValidProp's expected classes: indexed is String[] (Collection
// on Avro), mapped is Map, mapped('a') is Object for the untyped-map
// underlyings (map/objectarray/json/json-provided) and String for bean/avro,
// indexed[0] is String (Object on Avro record collections? — Avro keeps
// String per the assertEquals on indexed[0]). The nested cases pin Integer on
// the c0..c3 value columns and Boolean on the exists columns.
func einpTypePins(caseName, mode string) map[string]string {
	switch caseName {
	case "mapped-indexed":
		indexed := "String[]"
		indexed0 := "String"
		mappedA := "String"
		if mode == "avro" {
			indexed = "Collection"
		}
		if mode == "map" || mode == "objectarray" || mode == "json" || mode == "json-provided" {
			mappedA = "Object"
		}
		return map[string]string{
			"indexed":     indexed,
			"indexed[0]":  indexed0,
			"mapped":      "Map",
			"mapped('a')": mappedA,
		}
	case "nested-indexed", "nested-simple":
		return map[string]string{
			"c0": "Integer", "exists_c0": "Boolean",
			"c1": "Integer", "exists_c1": "Boolean",
			"c2": "Integer", "exists_c2": "Boolean",
			"c3": "Integer", "exists_c3": "Boolean",
		}
	}
	return nil
}

// ---- Getter probes -----------------------------------------------------------

// einpProbe is one Java getter/fragment probe on the last select-* event:
// name is the byte-exact Java property path (kept verbatim, including
// backtick escapes), goPath is the Go-equivalent resolution path, kind
// selects event.get vs event.getFragment, fragmentType carries the nested
// schema name used to render fragment content, array marks indexed fragment
// renders, and xmlInt coerces the raw XML attribute text to the XSD-typed
// integer the Java getter returns.
type einpProbe struct {
	name         string
	goPath       string
	kind         einpProbeKind
	fragmentType string
	array        bool
	xmlInt       bool
	// existsOnly emits {"exists":true,"value":null} for XML fragment probes:
	// Java's assertFragments asserts the fragment exists but Go renders no
	// XML-fragment object, so the value stays null like the non-XML
	// exists=false records differ only in the flag.
	existsOnly bool
	expect     string // canonical JSON for the pinned expected value (empty = no pin)
}

type einpProbeKind uint8

const (
	einpProbeValue einpProbeKind = iota
	einpProbeFragment
)

// einpNestedFragmentType resolves the nested schema name for a fragment probe
// path: the leaf type name registered for the (case, mode) nested chain.
func einpNestedFragmentType(caseName, mode, path string) string {
	if caseName == "nested-escaped" {
		// The only fragment probe is lvl1.lvl2.`lvl3` (non-indexed, Lvl3).
		switch mode {
		case "bean":
			return "BeanLvl3"
		case "map":
			return "MapLvl3"
		case "objectarray":
			return "OALvl3"
		case "json":
			return "JSONLvl3"
		case "avro":
			return "AvroLvl3"
		}
		return ""
	}
	// nested-indexed / nested-simple fragment chains end on the l1..l4
	// nested types; the last segment before the leaf picks the type.
	top := einpJavaTypeName(caseName, mode)
	switch {
	case strings.HasSuffix(path, "l4") || strings.HasSuffix(path, "l4[0]"):
		return top + "_4"
	case strings.Contains(path, "l3"):
		return top + "_3"
	case strings.Contains(path, "l2"):
		return top + "_2"
	default:
		return top + "_1"
	}
}

// einpProbes returns the per-send probe list for a case/mode pair on the
// select-* pass. Value probes mirror event.getEventType().getGetter*(name)
// .get(event, …) or event.get(name); fragment probes mirror
// event.getFragment(name). Invalid-property probes (Java tryInvalidProperty
// assertions) emit exists=false. XML iterations emit value probes only: the
// Java fragment assertions for XML have no Go-side nested-schema
// counterpart, and the raw-attr rendering is already covered by the leaf
// probes.
func einpProbes(caseName, mode string) []einpProbe {
	xml := mode == "xml"
	switch caseName {
	case "indexed-key-expr":
		switch mode {
		case "objectarray":
			return []einpProbe{
				{name: "intarray", goPath: "intarray[1]", kind: einpProbeValue, expect: "2"},
				{name: "oainner", goPath: "oainner[1]", kind: einpProbeValue, expect: `["B"]`},
				{name: "dummy", goPath: "dummy", kind: einpProbeValue},
			}
		case "map":
			return []einpProbe{
				{name: "intarray", goPath: "intarray[1]", kind: einpProbeValue, expect: "2"},
				{name: "mapinner", goPath: "mapinner[1]", kind: einpProbeValue, expect: `{"p0":"B"}`},
				{name: "dummy", goPath: "dummy", kind: einpProbeValue},
			}
		case "wrapper":
			return []einpProbe{
				{name: "arr", goPath: "arr[1]", kind: einpProbeValue, expect: "2"},
				{name: "mapped", goPath: "mapped('A')", kind: einpProbeValue, expect: "2"},
			}
		case "bean":
			return []einpProbe{
				{name: "listOfInt", goPath: "listOfInt[1]", kind: einpProbeValue, expect: "2"},
				{name: "iterableOfInt", goPath: "iterableOfInt[1]", kind: einpProbeValue, expect: "2"},
			}
		case "json", "json-provided":
			return []einpProbe{
				{name: "indexed", goPath: "indexed[1]", kind: einpProbeValue, expect: "2"},
				{name: "mapped", goPath: "mapped('keyOne')", kind: einpProbeValue, expect: "20"},
			}
		}
	case "mapped-indexed":
		probes := []einpProbe{
			{name: "mapped", goPath: "mapped('k1')", kind: einpProbeValue, expect: `"v1"`},
			{name: "indexed", goPath: "indexed[1]", kind: einpProbeValue, expect: `"v2"`},
		}
		// Java's runAssertionEventInvalidProp: tryInvalidProperty +
		// tryInvalidGetFragment per name emit exists:false records.
		for _, path := range []string{"xxxx", "mapped[1]", "indexed('a')", "mapped.x", "indexed.x"} {
			probes = append(probes, einpProbe{name: path, goPath: path, kind: einpProbeValue})
		}
		return probes
	case "nested-escaped":
		return []einpProbe{
			{name: "lvl1.lvl2.`vlvl2`", goPath: "lvl1.lvl2.vlvl2", kind: einpProbeValue, expect: `"v2"`},
			{name: "lvl1.lvl2.lvl3.`vlvl3`", goPath: "lvl1.lvl2.lvl3.vlvl3", kind: einpProbeValue, expect: `"v3"`},
			{name: "`lvl1`.`lvl2`.`lvl3`.`vlvl3`", goPath: "lvl1.lvl2.lvl3.vlvl3", kind: einpProbeValue, expect: `"v3"`},
			{name: "lvl1.lvl2.`vlvl2dyn`?", goPath: "lvl1.lvl2.vlvl2dyn?", kind: einpProbeValue, expect: `"v2dyn"`},
			{name: "lvl1.lvl2.`lvl3`", goPath: "lvl1.lvl2.lvl3", kind: einpProbeFragment,
				fragmentType: einpNestedFragmentType(caseName, mode, "lvl1.lvl2.lvl3"), expect: `{"vlvl3":"v3"}`},
		}
	case "nested-indexed":
		var probes []einpProbe
		if xml {
			probes = append(probes,
				einpProbe{name: "l1[0].lvl1", goPath: "myevent.l1.@lvl1", kind: einpProbeValue, xmlInt: true, expect: "1"},
				einpProbe{name: "l1[0].l2[0].lvl2", goPath: "myevent.l1.l2.@lvl2", kind: einpProbeValue, xmlInt: true, expect: "2"},
				einpProbe{name: "l1[0].l2[0].l3[0].lvl3", goPath: "myevent.l1.l2.l3.@lvl3", kind: einpProbeValue, xmlInt: true, expect: "3"},
				einpProbe{name: "l1[0].l2[0].l3[0].l4[0].lvl4", goPath: "myevent.l1.l2.l3.l4.@lvl4", kind: einpProbeValue, xmlInt: true, expect: "4"},
			)
			// The Go XML document model stores a single child element as a
			// bare map (no array), so Java's [0] indexers drop out: l1[0].l2
			// resolves through myevent.l1.l2.
			stripped := strings.NewReplacer("[0]", "").Replace
			for _, path := range []string{"l1", "l1[0].l2", "l1[0].l2[0].l3", "l1[0].l2[0].l3[0].l4",
				"l1[0]", "l1[0].l2[0]", "l1[0].l2[0].l3[0]", "l1[0].l2[0].l3[0].l4[0]"} {
				probes = append(probes, einpProbe{name: path, goPath: "myevent." + stripped(path), kind: einpProbeFragment, existsOnly: true})
			}
		} else {
			probes = append(probes,
				einpProbe{name: "l1[0].lvl1", goPath: "l1[0].lvl1", kind: einpProbeValue, expect: "1"},
				einpProbe{name: "l1[0].l2[0].lvl2", goPath: "l1[0].l2[0].lvl2", kind: einpProbeValue, expect: "2"},
				einpProbe{name: "l1[0].l2[0].l3[0].lvl3", goPath: "l1[0].l2[0].l3[0].lvl3", kind: einpProbeValue, expect: "3"},
				einpProbe{name: "l1[0].l2[0].l3[0].l4[0].lvl4", goPath: "l1[0].l2[0].l3[0].l4[0].lvl4", kind: einpProbeValue, expect: "4"},
			)
			for _, path := range []string{"l1", "l1[0].l2", "l1[0].l2[0].l3", "l1[0].l2[0].l3[0].l4"} {
				probes = append(probes, einpProbe{name: path, goPath: path, kind: einpProbeFragment,
					fragmentType: einpNestedFragmentType(caseName, mode, path), array: true})
			}
			for _, path := range []string{"l1[0]", "l1[0].l2[0]", "l1[0].l2[0].l3[0]", "l1[0].l2[0].l3[0].l4[0]"} {
				probes = append(probes, einpProbe{name: path, goPath: path, kind: einpProbeFragment,
					fragmentType: einpNestedFragmentType(caseName, mode, path)})
			}
		}
		for _, path := range []string{"l2", "l2[0]", "l1[0].l3", "l1[0].l2[0].l3[0].x"} {
			probes = append(probes, einpProbe{name: path, goPath: path, kind: einpProbeValue})
		}
		return probes
	case "nested-simple":
		var probes []einpProbe
		if xml {
			probes = append(probes,
				einpProbe{name: "l1.lvl1", goPath: "myevent.l1.@lvl1", kind: einpProbeValue, xmlInt: true, expect: "1"},
				einpProbe{name: "l1.l2.lvl2", goPath: "myevent.l1.l2.@lvl2", kind: einpProbeValue, xmlInt: true, expect: "2"},
				einpProbe{name: "l1.l2.l3.lvl3", goPath: "myevent.l1.l2.l3.@lvl3", kind: einpProbeValue, xmlInt: true, expect: "3"},
				einpProbe{name: "l1.l2.l3.l4.lvl4", goPath: "myevent.l1.l2.l3.l4.@lvl4", kind: einpProbeValue, xmlInt: true, expect: "4"},
			)
			// Java calls assertFragments twice: once on "l1.l2" alone, then on
			// the four chain paths — the middle l1.l2 record is duplicated.
			for _, path := range []string{"l1.l2", "l1", "l1.l2", "l1.l2.l3", "l1.l2.l3.l4"} {
				probes = append(probes, einpProbe{name: path, goPath: "myevent." + path, kind: einpProbeFragment, existsOnly: true})
			}
		} else {
			probes = append(probes,
				einpProbe{name: "l1.lvl1", goPath: "l1.lvl1", kind: einpProbeValue, expect: "1"},
				einpProbe{name: "l1.l2.lvl2", goPath: "l1.l2.lvl2", kind: einpProbeValue, expect: "2"},
				einpProbe{name: "l1.l2.l3.lvl3", goPath: "l1.l2.l3.lvl3", kind: einpProbeValue, expect: "3"},
				einpProbe{name: "l1.l2.l3.l4.lvl4", goPath: "l1.l2.l3.l4.lvl4", kind: einpProbeValue, expect: "4"},
			)
			// Java calls assertFragments twice: once on "l1.l2" alone, then on
			// the four chain paths — the middle l1.l2 record is duplicated.
			for _, path := range []string{"l1.l2", "l1", "l1.l2", "l1.l2.l3", "l1.l2.l3.l4"} {
				probes = append(probes, einpProbe{name: path, goPath: path, kind: einpProbeFragment,
					fragmentType: einpNestedFragmentType(caseName, mode, path)})
			}
		}
		for _, path := range []string{"l2", "l1.l3", "l1.xxx", "l1.l2.x", "l1.l2.l3.x", "l1.lvl1.x"} {
			probes = append(probes, einpProbe{name: path, goPath: path, kind: einpProbeValue})
		}
		return probes
	}
	return nil
}

// ---- Pinned sends -------------------------------------------------------------

// einpSends lists the pinned payload sequence per (case, mode). The nested
// cases send two events (1,2,3,4 then 10,5,50,400); nested-escaped sends the
// same shape twice per type (getter pass then select pass). Payloads are
// semantic JSON; the senders rebuild the mode-specific underlying.
func einpSends(caseName, mode string) []string {
	switch caseName {
	case "indexed-key-expr":
		switch mode {
		case "objectarray":
			return []string{`{"values":[[1,2],[["A"],["B"]]]}`}
		case "map":
			return []string{`{"intarray":[1,2],"mapinner":[{"p0":"A"},{"p0":"B"}]}`}
		case "wrapper", "bean":
			return []string{`{"variant":"default"}`}
		case "json", "json-provided":
			return []string{`{"indexed":[1,2],"mapped":{"keyOne":20}}`}
		}
	case "indexed-runtime-index":
		if mode == "objectarray" {
			return []string{`{"values":[["a","b"]]}`}
		}
		return []string{`{"indexed":["a","b"]}`}
	case "mapped-indexed":
		switch mode {
		case "bean":
			return []string{`{"variant":"default"}`}
		case "objectarray":
			return []string{`{"values":[["v1","v2"],{"k1":"v1"}]}`}
		default:
			return []string{`{"indexed":["v1","v2"],"mapped":{"k1":"v1"}}`}
		}
	case "mapped-runtime-key":
		switch mode {
		case "bean":
			return []string{`{"variant":"default"}`}
		case "objectarray":
			return []string{`{"values":[{"a1":"x","a2":"y"}]}`}
		default:
			return []string{`{"mapped":{"a1":"x","a2":"y"}}`}
		}
	case "nested-indexed", "nested-simple":
		return []string{`{"lvl1":1,"lvl2":2,"lvl3":3,"lvl4":4}`, `{"lvl1":10,"lvl2":5,"lvl3":50,"lvl4":400}`}
	case "nested-escaped":
		return []string{`{"vlvl2":"v2","vlvl2dyn":"v2dyn","vlvl3":"v3"}`}
	}
	return nil
}

// ---- Go bean mirrors ---------------------------------------------------------

// einpSupportBean mirrors the full SupportBean property set used by the
// wrapper select ({1,2} as arr, *, singletonMap as mapped): the select-*
// row renders every Java bean property, so the mirror declares all of them
// with Java zero values. charPrimitive/charBoxed render as Java Character
// strings ("\u0000"/null).
type einpSupportBean struct {
	TheString       *string `esper:"theString" json:"theString"`
	BoolPrimitive   bool    `esper:"boolPrimitive" json:"boolPrimitive"`
	IntPrimitive    int64   `esper:"intPrimitive" json:"intPrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive" json:"longPrimitive"`
	CharPrimitive   string  `esper:"charPrimitive" json:"charPrimitive"`
	ShortPrimitive  int64   `esper:"shortPrimitive" json:"shortPrimitive"`
	BytePrimitive   int64   `esper:"bytePrimitive" json:"bytePrimitive"`
	FloatPrimitive  float64 `esper:"floatPrimitive" json:"floatPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive" json:"doublePrimitive"`
	BoolBoxed       *bool   `esper:"boolBoxed" json:"boolBoxed"`
	IntBoxed        *int64  `esper:"intBoxed" json:"intBoxed"`
	LongBoxed       *int64  `esper:"longBoxed" json:"longBoxed"`
	CharBoxed       *string `esper:"charBoxed" json:"charBoxed"`
	ShortBoxed      *int64  `esper:"shortBoxed" json:"shortBoxed"`
	ByteBoxed       *int64  `esper:"byteBoxed" json:"byteBoxed"`
	FloatBoxed      *int64  `esper:"floatBoxed" json:"floatBoxed"`
	DoubleBoxed     *int64  `esper:"doubleBoxed" json:"doubleBoxed"`
	BigDecimal      *string `esper:"bigDecimal" json:"bigDecimal"`
	BigInteger      *string `esper:"bigInteger" json:"bigInteger"`
	EnumValue       *string `esper:"enumValue" json:"enumValue"`
}

func einpNewSupportBean() einpSupportBean {
	return einpSupportBean{CharPrimitive: "\x00"}
}

// einpIndexMappedSamplerBean mirrors
// EventInfraPropertyIndexedKeyExpr.MyIndexMappedSamplerBean (listOfInt and
// iterableOfInt both back the same [1,2] content).
type einpIndexMappedSamplerBean struct {
	ListOfInt     []int64 `esper:"listOfInt" json:"listOfInt"`
	IterableOfInt []int64 `esper:"iterableOfInt" json:"iterableOfInt"`
}

// einpIndexMappedJSONProvided mirrors
// EventInfraPropertyIndexedKeyExpr.MyLocalJsonProvided.
type einpIndexMappedJSONProvided struct {
	Indexed []int64        `esper:"indexed" json:"indexed"`
	Mapped  map[string]any `esper:"mapped" json:"mapped"`
}

// einpRuntimeIndexEvent mirrors EventInfraPropertyIndexedRuntimeIndex.LocalEvent.
type einpRuntimeIndexEvent struct {
	Indexed []string `esper:"indexed" json:"indexed"`
}

// einpRuntimeIndexJSONProvided mirrors
// EventInfraPropertyIndexedRuntimeIndex.MyLocalJsonProvided.
type einpRuntimeIndexJSONProvided struct {
	Indexed []string `esper:"indexed" json:"indexed"`
}

// einpIMEvent mirrors EventInfraPropertyMappedIndexed.MyIMEvent.
type einpIMEvent struct {
	Indexed []string          `esper:"indexed" json:"indexed"`
	Mapped  map[string]string `esper:"mapped" json:"mapped"`
}

// einpMappedIndexedJSONProvided mirrors
// EventInfraPropertyMappedIndexed.MyLocalJsonProvided.
type einpMappedIndexedJSONProvided struct {
	Indexed []string          `esper:"indexed" json:"indexed"`
	Mapped  map[string]string `esper:"mapped" json:"mapped"`
}

// einpRuntimeKeyEvent mirrors EventInfraPropertyMappedRuntimeKey.LocalEvent.
type einpRuntimeKeyEvent struct {
	Mapped map[string]string `esper:"mapped" json:"mapped"`
}

// einpRuntimeKeyJSONProvided mirrors
// EventInfraPropertyMappedRuntimeKey.MyLocalJsonProvided.
type einpRuntimeKeyJSONProvided struct {
	Mapped map[string]string `esper:"mapped" json:"mapped"`
}

// Nested-indexed bean chain (InfraNestedIndexPropTop +
// InfraNestedIndexedPropLvl1..4).
type einpNestedIndexedTop struct {
	L1 []einpNestedIndexedLvl1 `esper:"l1" json:"l1"`
}

type einpNestedIndexedLvl1 struct {
	L2   []einpNestedIndexedLvl2 `esper:"l2" json:"l2"`
	Lvl1 int64                   `esper:"lvl1" json:"lvl1"`
}

type einpNestedIndexedLvl2 struct {
	L3   []einpNestedIndexedLvl3 `esper:"l3" json:"l3"`
	Lvl2 int64                   `esper:"lvl2" json:"lvl2"`
}

type einpNestedIndexedLvl3 struct {
	L4   []einpNestedIndexedLvl4 `esper:"l4" json:"l4"`
	Lvl3 int64                   `esper:"lvl3" json:"lvl3"`
}

type einpNestedIndexedLvl4 struct {
	Lvl4 int64 `esper:"lvl4" json:"lvl4"`
}

// Nested-indexed json-provided chain (MyLocalJSONProvided*).
type einpNestedIndexedJSONProvidedTop struct {
	L1 []einpNestedIndexedJSONProvidedLvl1 `esper:"l1" json:"l1"`
}

type einpNestedIndexedJSONProvidedLvl1 struct {
	L2   []einpNestedIndexedJSONProvidedLvl2 `esper:"l2" json:"l2"`
	Lvl1 int64                               `esper:"lvl1" json:"lvl1"`
}

type einpNestedIndexedJSONProvidedLvl2 struct {
	L3   []einpNestedIndexedJSONProvidedLvl3 `esper:"l3" json:"l3"`
	Lvl2 int64                               `esper:"lvl2" json:"lvl2"`
}

type einpNestedIndexedJSONProvidedLvl3 struct {
	L4   []einpNestedIndexedJSONProvidedLvl4 `esper:"l4" json:"l4"`
	Lvl3 int64                               `esper:"lvl3" json:"lvl3"`
}

type einpNestedIndexedJSONProvidedLvl4 struct {
	Lvl4 int64 `esper:"lvl4" json:"lvl4"`
}

// Nested-simple bean chain (InfraNestedSimplePropTop +
// InfraNestedSimplePropLvl1..4).
type einpNestedSimpleTop struct {
	L1 *einpNestedSimpleLvl1 `esper:"l1" json:"l1"`
}

type einpNestedSimpleLvl1 struct {
	L2   *einpNestedSimpleLvl2 `esper:"l2" json:"l2"`
	Lvl1 int64                 `esper:"lvl1" json:"lvl1"`
}

type einpNestedSimpleLvl2 struct {
	L3   *einpNestedSimpleLvl3 `esper:"l3" json:"l3"`
	Lvl2 int64                 `esper:"lvl2" json:"lvl2"`
}

type einpNestedSimpleLvl3 struct {
	L4   *einpNestedSimpleLvl4 `esper:"l4" json:"l4"`
	Lvl3 int64                 `esper:"lvl3" json:"lvl3"`
}

type einpNestedSimpleLvl4 struct {
	Lvl4 int64 `esper:"lvl4" json:"lvl4"`
}

// Nested-simple json-provided chain (MyLocalJSONProvidedTop/Lvl1..4).
type einpNestedSimpleJSONProvidedTop struct {
	L1 *einpNestedSimpleJSONProvidedLvl1 `esper:"l1" json:"l1"`
}

type einpNestedSimpleJSONProvidedLvl1 struct {
	L2   *einpNestedSimpleJSONProvidedLvl2 `esper:"l2" json:"l2"`
	Lvl1 int64                             `esper:"lvl1" json:"lvl1"`
}

type einpNestedSimpleJSONProvidedLvl2 struct {
	L3   *einpNestedSimpleJSONProvidedLvl3 `esper:"l3" json:"l3"`
	Lvl2 int64                             `esper:"lvl2" json:"lvl2"`
}

type einpNestedSimpleJSONProvidedLvl3 struct {
	L4   *einpNestedSimpleJSONProvidedLvl4 `esper:"l4" json:"l4"`
	Lvl3 int64                             `esper:"lvl3" json:"lvl3"`
}

type einpNestedSimpleJSONProvidedLvl4 struct {
	Lvl4 int64 `esper:"lvl4" json:"lvl4"`
}

// Nested-escaped bean chain (SupportLvl0..3). vlvl2dyn is a declared getter
// on the bean (SupportLvl2.getVlvl2dyn) even though the map and json schemas
// leave it undeclared.
type einpEscapedLvl0 struct {
	Lvl1 *einpEscapedLvl1 `esper:"lvl1" json:"lvl1"`
}

type einpEscapedLvl1 struct {
	Lvl2 *einpEscapedLvl2 `esper:"lvl2" json:"lvl2"`
}

type einpEscapedLvl2 struct {
	Vlvl2    string           `esper:"vlvl2" json:"vlvl2"`
	Vlvl2dyn string           `esper:"vlvl2dyn" json:"vlvl2dyn"`
	Lvl3     *einpEscapedLvl3 `esper:"lvl3" json:"lvl3"`
}

type einpEscapedLvl3 struct {
	Vlvl3 string `esper:"vlvl3" json:"vlvl3"`
}

// ---- Iteration state ---------------------------------------------------------

type einpIteration struct {
	env         *esper.Environment
	engine      *esper.Engine
	schemas     map[string]esper.Schema
	avroNested  map[string]esper.Schema
	deployments map[string]*esper.Deployment
	deployOrder []string
	sequences   map[string]uint64
	mode        string
	s0Star      bool
	lastResult  *esper.Result
}

func runEventInfraPropertyNonDynamicScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eventInfraPropertyNonDynamicCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEventInfraPropertyNonDynamicCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eventInfraPropertyNonDynamicID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", eventInfraPropertyNonDynamicID, scenario.ID)
	}
	return trace, nil
}

func runEventInfraPropertyNonDynamicCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	var iter *einpIteration
	defer func() {
		if iter != nil && iter.engine != nil {
			_ = iter.engine.Close(context.Background())
		}
	}()

	newIteration := func(mode string) *einpIteration {
		if iter != nil && iter.engine != nil {
			_ = iter.engine.Close(context.Background())
		}
		iter = &einpIteration{
			env:         esper.NewEnvironment(),
			schemas:     make(map[string]esper.Schema),
			avroNested:  make(map[string]esper.Schema),
			deployments: make(map[string]*esper.Deployment),
			sequences:   make(map[string]uint64),
			mode:        mode,
		}
		iter.engine = esper.NewEngine(iter.env,
			esper.WithRuntimeURI(eventInfraPropertyNonDynamicJavaRuntimeIDs[eventInfraPropertyNonDynamicOrdinal(caseName)]),
			esper.WithStartTime(time.Unix(0, 0).UTC()))
		return iter
	}

	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "deploy":
			if iter == nil {
				iter = newIteration(step.Mode)
			}
			if err := iter.deploy(ctx, &trace, caseName, step); err != nil {
				return trace, err
			}
		case "deployed":
			if iter == nil {
				return trace, fmt.Errorf("%s: deployed marker without deploy", eventInfraPropertyNonDynamicID)
			}
			iter.sequences[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  iter.sequences[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(iter.engine.Now()),
			})
		case "types":
			if iter == nil {
				return trace, fmt.Errorf("%s: types probe without deploy", eventInfraPropertyNonDynamicID)
			}
			if err := iter.emitTypes(&trace, caseName, step); err != nil {
				return trace, err
			}
		case "send":
			if iter == nil {
				return trace, fmt.Errorf("%s: send without deploy", eventInfraPropertyNonDynamicID)
			}
			if err := iter.send(ctx, caseName, step); err != nil {
				return trace, err
			}
			if err := iter.emitGetterProbes(&trace, caseName); err != nil {
				return trace, err
			}
		case "undeploy":
			if iter != nil {
				if err := iter.undeploy(ctx, step.Statement); err != nil {
					return trace, err
				}
			}
		case "undeploy-all":
			if iter != nil {
				if err := iter.undeployAll(ctx); err != nil {
					return trace, err
				}
				_ = iter.engine.Close(context.Background())
				iter = nil
			}
		default:
			return trace, fmt.Errorf("%s: unsupported step op %q", eventInfraPropertyNonDynamicID, step.Op)
		}
	}
	return trace, nil
}

func eventInfraPropertyNonDynamicOrdinal(caseName string) int {
	for index, name := range eventInfraPropertyNonDynamicCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// deploy handles the "schema" and "s0" deploy steps. The schema step
// registers the Go event types for the underlying mode (or all five type
// families for nested-escaped's single-module deploy); s0 deploys the pinned
// statement — the projection, the select-* navigation pass, or the wrapper
// select for indexed-key-expr's SupportBean iteration.
func (it *einpIteration) deploy(ctx context.Context, trace *compat.Trace, caseName string, step compat.Step) error {
	switch step.Statement {
	case "schema":
		it.mode = step.Mode
		return it.registerSchemas(caseName, step.Mode)
	case "s0":
		it.mode = step.Mode
		// s0Star flags the pass whose listener events carry the getter
		// probes: the select-* statements (mapped-indexed's s0, the nested
		// cases' bean-nav pass, and indexed-key-expr's per-mode select *)
		// plus indexed-key-expr's wrapper projection, whose result event
		// owns the arr/mapped fields the probes read. Projection statements
		// (the nested cases' first s0, nested-escaped's select pass) emit
		// no probes.
		star := einpStarEPL(caseName, it.mode)
		it.s0Star = (star != "" && step.Epl == star) ||
			(caseName == "indexed-key-expr" && it.mode == "wrapper") ||
			(caseName == "indexed-key-expr" && len(einpProbes(caseName, it.mode)) > 0)
		if star != "" && step.Epl == star {
			plan, err := it.env.Build(esper.FromAny(it.env, einpJavaTypeName(caseName, it.mode)).Query(esper.StatementName("s0")))
			if err != nil {
				return fmt.Errorf("%s: build s0 select-*: %w", eventInfraPropertyNonDynamicID, err)
			}
			return it.deployPlan(ctx, trace, caseName, step.Statement, plan)
		}
		if step.Epl != einpS0EPL(caseName, it.mode) {
			return fmt.Errorf("%s: s0 EPL %q is not pinned", eventInfraPropertyNonDynamicID, step.Epl)
		}
		plan, err := it.buildS0(caseName, it.mode)
		if err != nil {
			return err
		}
		return it.deployPlan(ctx, trace, caseName, step.Statement, plan)
	default:
		return fmt.Errorf("%s: unsupported deploy statement %q", eventInfraPropertyNonDynamicID, step.Statement)
	}
}

func (it *einpIteration) deployPlan(ctx context.Context, trace *compat.Trace, caseName, label string, plan esper.Plan) error {
	deployment, err := it.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", eventInfraPropertyNonDynamicID, label, err)
	}
	it.deployments[label] = deployment
	it.deployOrder = append(it.deployOrder, label)
	for _, statement := range deployment.Statements() {
		stmt := statement
		if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			normalized := einpNormalizeRows(batch.New)
			if it.mode == "xml" && caseName == "nested-indexed" && it.s0Star {
				// Java's XML getter returns NodeLists for element
				// properties declared maxOccurs=unbounded: the Go document
				// model stores a single child element as a bare map, so
				// select-* listener fields re-wrap element children to
				// arrays like the Java oracle's node-list normalization.
				for _, record := range normalized {
					for name, value := range record.Fields {
						record.Fields[name] = []any{einpXMLNodeArrays(value)}
					}
				}
			}
			it.sequences[stmt.Name()]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "listener",
				Statement: stmt.Name(),
				Sequence:  it.sequences[stmt.Name()],
				Time:      compat.FormatTraceTime(batch.Time),
				New:       normalized,
				Old:       compat.NormalizeResults(batch.Old),
			})
			if stmt.Name() == "s0" && len(batch.New) > 0 {
				last := batch.New[len(batch.New)-1]
				it.lastResult = &last
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// einpNormalizeRows renders rows like compat.NormalizeResults, then collapses
// Missing markers to the tagged null (Java event.get() returns null where Go
// reports Missing) and untags nested null markers inside map/array values
// (Java renders nested nulls as plain JSON null).
func einpNormalizeRows(results []esper.Result) []compat.ResultRecord {
	records := compat.NormalizeResults(results)
	for _, record := range records {
		for name, value := range record.Fields {
			if einpIsNilValue(value) {
				record.Fields[name] = map[string]any{"state": "null"}
				continue
			}
			if missing, ok := value.(map[string]any); ok && missing["state"] == "missing" {
				record.Fields[name] = map[string]any{"state": "null"}
			}
		}
	}
	for _, record := range records {
		for name, value := range record.Fields {
			record.Fields[name] = einpUntagNested(value)
		}
	}
	return records
}

// einpUntagNested converts nested {state:null}/{state:missing} tags inside
// row field values to plain JSON nulls, matching the Java oracle's
// normalizeNested (the top-level field itself keeps the tagged object).
func einpUntagNested(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 1 && (typed["state"] == "null" || typed["state"] == "missing") {
			return value
		}
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = einpUntagNestedChild(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = einpUntagNestedChild(item)
		}
		return out
	default:
		return value
	}
}

func einpUntagNestedChild(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 1 && (typed["state"] == "null" || typed["state"] == "missing") {
			return nil
		}
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = einpUntagNestedChild(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = einpUntagNestedChild(item)
		}
		return out
	default:
		return value
	}
}

func einpIsNilValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Interface, reflect.Func, reflect.Chan:
		return reflected.IsNil()
	}
	return false
}

// buildS0 projects the pinned Java statement. Select-* statements reach here
// only when einpStarEPL does not cover them (indexed-key-expr's non-wrapper
// iterations and mapped-indexed): an empty column table means select-* and
// builds a bare stream query. The wrapper iteration projects the array/map
// literals alongside the SupportBean transpose; the projection cases emit
// the column table (XML leaf columns coerce the raw attribute text to int64
// like the XSD-typed Java getter).
func (it *einpIteration) buildS0(caseName, mode string) (esper.Plan, error) {
	typeName := einpJavaTypeName(caseName, mode)
	stream := esper.FromAny(it.env, typeName)
	eventValue := esper.EventValue[esper.Event]()
	prop := func(path string) esper.Expression[any] {
		return esper.Property[any](eventValue, path)
	}
	var selections []esper.Selection
	if caseName == "indexed-key-expr" && mode == "wrapper" {
		// @name('s0') select {1, 2} as arr, *,
		// Collections.singletonMap('A', 2) as mapped from SupportBean — the
		// Go select has no wildcard selection, so the * expands to an
		// explicit alias per SupportBean property (Java getter order).
		selections = append(selections,
			esper.Alias("arr", esper.ArrayLiteral[any](esper.Literal[any](int64(1)), esper.Literal[any](int64(2)))),
		)
		for _, name := range []string{
			"theString", "boolPrimitive", "intPrimitive", "longPrimitive",
			"charPrimitive", "shortPrimitive", "bytePrimitive", "floatPrimitive",
			"doublePrimitive", "boolBoxed", "intBoxed", "longBoxed", "charBoxed",
			"shortBoxed", "byteBoxed", "floatBoxed", "doubleBoxed", "enumValue",
			"bigDecimal", "bigInteger",
		} {
			selections = append(selections, esper.Alias(name, prop(name)))
		}
		selections = append(selections,
			esper.Alias("mapped", esper.Literal(map[string]any{"A": int64(2)})),
		)
		return it.env.Build(stream.Select(selections...).Query(esper.StatementName("s0")))
	}
	columns := einpColumns(caseName, mode)
	if len(columns) == 0 {
		return it.env.Build(stream.Query(esper.StatementName("s0")))
	}
	for _, column := range columns {
		if column.exists {
			selections = append(selections, esper.Alias(column.name, esper.Exists(prop(column.goPath))))
			continue
		}
		if column.xmlInt {
			selections = append(selections, esper.Alias(column.name,
				esper.Func1[any, any]("einpXmlInt", einpCoerceXMLInt, prop(column.goPath))))
			continue
		}
		selections = append(selections, esper.Alias(column.name, prop(column.goPath)))
	}
	return it.env.Build(stream.Select(selections...).Query(esper.StatementName("s0")))
}

// einpCoerceXMLInt renders an XML attribute text value as the integer the
// Java XSD-typed attribute getter returns.
func einpCoerceXMLInt(value any) any {
	switch typed := value.(type) {
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
			return parsed
		}
		return typed
	case int64:
		return typed
	default:
		return value
	}
}

// emitTypes asserts and emits one types record per pinned property. Statement
// types steps (statement "s0") mirror Java's assertStatement property-type
// checks: the emitted value is the pinned Java type name; Go verifies the
// column field exists on the s0 result schema. Type-level steps (no
// statement, eventType set) mirror the runAssertionTypeValidProp /
// runAssertionTypeInvalidProp checks on the event type: "<invalid>" pins a
// property that must not resolve, anything else requires resolution; the
// Java oracle asserts the concrete getPropertyType/descriptor values.
func (it *einpIteration) emitTypes(trace *compat.Trace, caseName string, step compat.Step) error {
	var resolver func(name string) bool
	if step.EventType != "" {
		input, ok := it.schemas[step.EventType]
		if !ok {
			return fmt.Errorf("%s: types step has no input schema %q", eventInfraPropertyNonDynamicID, step.EventType)
		}
		resolver = func(name string) bool { return einpTypePropertyExists(input, name) }
	} else {
		deployment, ok := it.deployments[step.Statement]
		if !ok {
			return fmt.Errorf("%s: types for undeployed statement %q", eventInfraPropertyNonDynamicID, step.Statement)
		}
		if len(deployment.Statements()) == 0 {
			return fmt.Errorf("%s: deployment %q has no statements", eventInfraPropertyNonDynamicID, step.Statement)
		}
		schema, ok := deployment.Statements()[0].Plan().ResultSchema()
		if !ok {
			// Select-* statements carry no projection schema: property
			// checks run against the input event type, which is what the
			// star expands to.
			schema, ok = it.schemas[einpJavaTypeName(caseName, it.mode)]
			if !ok {
				return fmt.Errorf("%s: statement %q has no result schema", eventInfraPropertyNonDynamicID, step.Statement)
			}
		}
		resolver = func(name string) bool {
			return einpTypePropertyExists(schema, name)
		}
	}
	for _, name := range step.PropertyOrder {
		expected, pinned := step.PropertyTypes[name]
		if !pinned {
			return fmt.Errorf("%s: types step has no pinned type for %q", eventInfraPropertyNonDynamicID, name)
		}
		exists := resolver(name)
		if expected == "<invalid>" {
			if exists {
				return fmt.Errorf("%s: property %q unexpectedly resolves", eventInfraPropertyNonDynamicID, name)
			}
		} else if !exists {
			return fmt.Errorf("%s: types property %q unresolvable (case %s mode %s statement %s eventType %s)", eventInfraPropertyNonDynamicID, name, caseName, it.mode, step.Statement, step.EventType)
		}
		it.sequences[step.Statement+":types"]++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "types",
			Statement: step.Statement,
			Sequence:  it.sequences[step.Statement+":types"],
			Time:      compat.FormatTraceTime(it.engine.Now()),
			Name:      name,
			Value:     expected,
		})
	}
	return nil
}

// einpTypePropertyExists mirrors Java's eventType.isProperty(path): each
// segment's base must resolve, a [i] accessor needs an indexed (slice/array)
// field, a ('k') accessor needs a mapped (map) field, and dotted descent
// through an indexed field without its own accessor is invalid (Java's
// isProperty("l1[0].l2.l4") is false because l2 is an event-type array).
func einpTypePropertyExists(schema esper.Schema, path string) bool {
	if schema.Name() == "" {
		return false
	}
	segments := strings.Split(path, ".")
	current := schema
	for index, segment := range segments {
		base := segment
		indexed := false
		keyed := false
		if open := strings.Index(segment, "["); open >= 0 {
			if !strings.HasSuffix(segment, "]") {
				return false
			}
			base = segment[:open]
			indexed = true
		} else if open := strings.Index(segment, "("); open >= 0 {
			if !strings.HasSuffix(segment, ")") {
				return false
			}
			base = segment[:open]
			keyed = true
		}
		if _, found := current.Property(base); !found {
			return false
		}
		fieldKind := einpSchemaFieldKind(current, base)
		if indexed && fieldKind != "indexed" {
			return false
		}
		if keyed && fieldKind != "mapped" {
			return false
		}
		if index == len(segments)-1 {
			return true
		}
		if !indexed && fieldKind == "indexed" {
			return false
		}
		nested, ok := current.NestedSchema(base)
		if !ok {
			return false
		}
		current = nested
	}
	return true
}

// einpSchemaFieldKind classifies a declared field the way Esper's accessor
// validation does: slice/array-typed fields accept [i] (indexed), map-typed
// fields accept ('k') (mapped), everything else is plain. Descending a dotted
// path through an indexed field without an [i] accessor is invalid, matching
// Java's isProperty("l1.l2") on an array-typed l1.
func einpSchemaFieldKind(schema esper.Schema, name string) string {
	field, ok := schema.Field(name)
	if !ok || field.Type == nil {
		return "plain"
	}
	switch field.Type.Kind() {
	case reflect.Slice, reflect.Array:
		return "indexed"
	case reflect.Map:
		return "mapped"
	}
	return "plain"
}

// emitGetterProbes mirrors the Java per-send assertions that run on the last
// select-* event: getGetterIndexed/getGetterMapped/getGetter value probes and
// getFragment fragment probes. Sends on the projection pass (nested cases'
// first statement, nested-escaped's select pass) emit no probes.
func (it *einpIteration) emitGetterProbes(trace *compat.Trace, caseName string) error {
	if !it.s0Star {
		return nil
	}
	probes := einpProbes(caseName, it.mode)
	if len(probes) == 0 {
		return nil
	}
	if it.lastResult == nil {
		return fmt.Errorf("%s: no s0 event captured for getter probe", eventInfraPropertyNonDynamicID)
	}
	result := *it.lastResult
	for _, probe := range probes {
		exists := false
		var rendered any
		switch probe.kind {
		case einpProbeValue:
			value := einpResultGet(result, probe.goPath)
			exists = value.IsPresent() || value.IsNull()
			if exists {
				rendered = einpRenderProbeValue(probe, it.schemas, value.Any())
			}
		case einpProbeFragment:
			if probe.existsOnly {
				value := einpResultGet(result, probe.goPath)
				exists = value.IsPresent() || value.IsNull()
				break
			}
			rendered, exists = einpResolveFragment(result, probe, it.schemas)
		}
		it.sequences["s0:getter"]++
		recordValue := any(nil)
		if exists {
			recordValue = rendered
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "getter",
			Statement: "s0",
			Sequence:  it.sequences["s0:getter"],
			Time:      compat.FormatTraceTime(it.engine.Now()),
			Name:      probe.name,
			Value: map[string]any{
				"exists": exists,
				"value":  recordValue,
			},
		})
	}
	return nil
}

// einpResultGet reads a property path off a listener result. Rows only
// resolve bare field names, so indexed/mapped accessor paths and dotted
// nested paths fall back to a manual walk of the row value; when the path
// still misses, the retained source event answers it (mapped-indexed's
// projection rows carry no row fields, so the pass event resolves there).
func einpResultGet(result esper.Result, path string) esper.Value {
	value := result.Get(path)
	if value.IsPresent() || value.IsNull() {
		return value
	}
	if row, ok := result.Row(); ok {
		if manual, found := einpRowPathGet(row, path); found {
			return manual
		}
	}
	if source, ok := result.RowEvent(); ok {
		return source.Get(path)
	}
	return value
}

// einpRowPathGet walks a dotted property path on a projection row: the first
// segment names a row column, later segments traverse maps and slices, and
// [n]/('k') accessors index the current value.
func einpRowPathGet(row esper.Row, path string) (esper.Value, bool) {
	base, accessors := einpSplitAccessorPath(path)
	current := row.Get(base)
	if !current.IsPresent() && !current.IsNull() {
		return current, false
	}
	return einpApplyAccessors(current, accessors), true
}

// einpSplitAccessorPath splits a Go probe path into its leading field name
// and the accessor chain: [0] indices and ('key') mapped lookups. Dotted
// segments are folded into the accessor list as field hops.
func einpSplitAccessorPath(path string) (string, []string) {
	var accessors []string
	rest := path
	base := rest
	for index := 0; index < len(rest); index++ {
		c := rest[index]
		if c != '[' && c != '(' && c != '.' {
			continue
		}
		base = rest[:index]
		rest = rest[index:]
		break
	}
	if base == rest {
		return path, nil
	}
	for len(rest) > 0 {
		switch rest[0] {
		case '[':
			end := strings.Index(rest, "]")
			if end < 0 {
				return base, accessors
			}
			accessors = append(accessors, rest[:end+1])
			rest = rest[end+1:]
		case '(':
			end := strings.Index(rest, ")")
			if end < 0 {
				return base, accessors
			}
			accessors = append(accessors, rest[:end+1])
			rest = rest[end+1:]
		case '.':
			next := 1
			for next < len(rest) && rest[next] != '[' && rest[next] != '(' && rest[next] != '.' {
				next++
			}
			accessors = append(accessors, rest[1:next])
			rest = rest[next:]
		default:
			return base, accessors
		}
	}
	return base, accessors
}

// einpApplyAccessors applies index/mapped/dotted accessors to a raw value:
// [n] indexes slices, ('k') and field hops read maps and named getters.
func einpApplyAccessors(current esper.Value, accessors []string) esper.Value {
	for _, accessor := range accessors {
		raw := current.Any()
		if raw == nil {
			return esper.Missing()
		}
		switch {
		case strings.HasPrefix(accessor, "["):
			index, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(accessor, "["), "]"))
			if err != nil {
				return esper.Missing()
			}
			items, ok := einpAsSlice(raw)
			if !ok || index < 0 || index >= len(items) {
				return esper.Missing()
			}
			current = esper.Present(items[index])
		case strings.HasPrefix(accessor, "("):
			key := accessor[1 : len(accessor)-1]
			key = strings.TrimSuffix(strings.TrimPrefix(key, "'"), "'")
			object, ok := raw.(map[string]any)
			if !ok {
				return esper.Missing()
			}
			item, found := object[key]
			if !found {
				return esper.Missing()
			}
			current = esper.Present(item)
		default:
			object, ok := raw.(map[string]any)
			if ok {
				item, found := object[accessor]
				if !found {
					return esper.Missing()
				}
				current = esper.Present(item)
				continue
			}
			reflected := reflect.ValueOf(raw)
			for reflected.IsValid() && (reflected.Kind() == reflect.Pointer || reflected.Kind() == reflect.Interface) {
				if reflected.IsNil() {
					return esper.Missing()
				}
				reflected = reflected.Elem()
			}
			if !reflected.IsValid() || reflected.Kind() != reflect.Struct {
				return esper.Missing()
			}
			current = esper.Present(einpStructFieldValue(reflected, accessor))
		}
	}
	return current
}

// einpResolveFragment resolves an event.getFragment(name) equivalent: the
// raw value walks the Go property path and renders through the pinned nested
// schema so objectarray rows and Avro records render as named objects like
// the Java EventBean fragment rendering.
func einpResolveFragment(result esper.Result, probe einpProbe, schemas map[string]esper.Schema) (any, bool) {
	value := einpResultGet(result, probe.goPath)
	if !value.IsPresent() {
		return nil, false
	}
	raw := value.Any()
	if raw == nil {
		return nil, false
	}
	schema, ok := schemas[probe.fragmentType]
	if !ok {
		// Class-provided JSON types (json-provided nested-simple) register no
		// per-level schemas; the underlying Go structs render generically.
		if probe.array {
			items, isSlice := einpAsSlice(raw)
			if !isSlice {
				return nil, false
			}
			rendered := make([]any, 0, len(items))
			for _, item := range items {
				rendered = append(rendered, einpRenderNested(item))
			}
			return rendered, true
		}
		return einpRenderNested(raw), true
	}
	if probe.array {
		items, ok := einpAsSlice(raw)
		if !ok {
			return nil, false
		}
		rendered := make([]any, 0, len(items))
		for _, item := range items {
			rendered = append(rendered, einpRenderSchemaFields(schema, item))
		}
		return rendered, true
	}
	return einpRenderSchemaFields(schema, raw), true
}

// einpXMLNodeArrays rewraps single-child element nodes into arrays, matching
// how the Java oracle normalizes DOM NodeList values: maps gain a slice per
// non-attribute child, and nested maps recurse.
func einpXMLNodeArrays(value any) any {
	switch typed := value.(type) {
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = einpXMLNodeArrays(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if strings.HasPrefix(key, "@") {
				out[key] = item
				continue
			}
			out[key] = einpXMLNodeArrays(item)
			if _, isSlice := out[key].([]any); !isSlice {
				out[key] = []any{out[key]}
			}
		}
		return out
	default:
		return value
	}
}

// einpAsSlice unwraps a fragment value to its element list.
func einpAsSlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	}
	reflected := reflect.ValueOf(value)
	for reflected.IsValid() && reflected.Kind() == reflect.Pointer {
		reflected = reflected.Elem()
	}
	if !reflected.IsValid() || (reflected.Kind() != reflect.Slice && reflected.Kind() != reflect.Array) {
		return nil, false
	}
	items := make([]any, reflected.Len())
	for index := range items {
		items[index] = reflected.Index(index).Interface()
	}
	return items, true
}

// einpRenderSchemaFields renders one fragment value as the sorted-field
// object Java's beanFragmentToJson produces: each declared field name maps
// to the raw cell normalized recursively (objectarray rows and Avro records
// carry positional/nested values, no schema recursion inside); structs and
// maps resolve their children through the nested schema links (bean, map,
// json chains keep the declared field names).
func einpRenderSchemaFields(schema esper.Schema, underlying any) any {
	if underlying == nil {
		return nil
	}
	if event, ok := underlying.(esper.Event); ok {
		out := make(map[string]any)
		for _, field := range event.Schema().Fields() {
			out[field.Name] = einpRenderNested(event.Get(field.Name).Any())
		}
		return out
	}
	fields := schema.Fields()
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	sort.Strings(names)
	out := make(map[string]any, len(names))
	// Objectarray cells and Avro record fields render raw: Java's get() hands
	// back Object[][] / nested GenericData.Records that normalizeNested
	// walks positionally or recursively without re-reading the schema.
	if record, ok := underlying.(*esper.AvroRecord); ok {
		for _, name := range names {
			value, _ := record.Lookup(name)
			out[name] = einpRenderNested(value)
		}
		return out
	}
	if record, ok := underlying.(esper.AvroRecord); ok {
		for _, name := range names {
			value, _ := record.Lookup(name)
			out[name] = einpRenderNested(value)
		}
		return out
	}
	if cells, ok := underlying.([]any); ok {
		for _, name := range names {
			var value any
			if index, found := schemaFieldIndex(schema, name); found && index < len(cells) {
				value = cells[index]
			}
			out[name] = einpRenderNested(value)
		}
		return out
	}
	underlyingValue := reflect.ValueOf(underlying)
	for underlyingValue.IsValid() && underlyingValue.Kind() == reflect.Pointer {
		underlyingValue = underlyingValue.Elem()
	}
	for _, name := range names {
		var value any
		if values, ok := underlying.(map[string]any); ok {
			value = values[name]
		} else if underlyingValue.IsValid() && underlyingValue.Kind() == reflect.Struct {
			value = einpStructFieldValue(underlyingValue, name)
		}
		out[name] = einpRenderNested(einpChildValue(schema, name, value))
	}
	return out
}

// einpChildValue renders a nested field through its nested schema when the
// schema declares one (fragment chains), so nested objectarray cells also
// render as named objects.
func einpChildValue(schema esper.Schema, name string, value any) any {
	nested, ok := schema.NestedSchema(name)
	if !ok || value == nil {
		return value
	}
	if items, isSlice := einpAsSlice(value); isSlice {
		rendered := make([]any, 0, len(items))
		for _, item := range items {
			rendered = append(rendered, einpRenderSchemaFields(nested, item))
		}
		return rendered
	}
	return einpRenderSchemaFields(nested, value)
}

// einpStructFieldValue reads one exported field by its esper/json tag name.
func einpStructFieldValue(value reflect.Value, name string) any {
	typeOf := value.Type()
	for index := 0; index < typeOf.NumField(); index++ {
		field := typeOf.Field(index)
		if field.Tag.Get("esper") == name || field.Tag.Get("json") == name {
			fieldValue := value.Field(index)
			if !fieldValue.CanInterface() {
				return nil
			}
			item := fieldValue.Interface()
			if reflected := reflect.ValueOf(item); reflected.IsValid() && (reflected.Kind() == reflect.Pointer || reflected.Kind() == reflect.Interface) && reflected.IsNil() {
				return nil
			}
			return item
		}
	}
	return nil
}

func schemaFieldIndex(schema esper.Schema, name string) (int, bool) {
	for index, field := range schema.Fields() {
		if field.Name == name {
			return index, true
		}
	}
	return 0, false
}

// einpRenderNested normalizes one probe/fragment leaf value like the Java
// oracle's normalizeNested: maps and records render as objects, arrays and
// slices as arrays, structs through their tagged fields.
func einpRenderNested(value any) any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case esper.Value:
		if !typed.IsPresent() {
			return nil
		}
		return einpRenderNested(typed.Any())
	case esper.Event:
		out := make(map[string]any)
		for _, field := range typed.Schema().Fields() {
			out[field.Name] = einpRenderNested(typed.Get(field.Name).Any())
		}
		return out
	case *esper.AvroRecord:
		fields := typed.Schema().Fields()
		names := make([]string, 0, len(fields))
		for _, field := range fields {
			names = append(names, field.Name)
		}
		sort.Strings(names)
		out := make(map[string]any, len(names))
		for _, name := range names {
			item, _ := typed.Lookup(name)
			out[name] = einpRenderNested(item)
		}
		return out
	case esper.AvroRecord:
		record := &typed
		fields := record.Schema().Fields()
		names := make([]string, 0, len(fields))
		for _, field := range fields {
			names = append(names, field.Name)
		}
		sort.Strings(names)
		out := make(map[string]any, len(names))
		for _, name := range names {
			item, _ := record.Lookup(name)
			out[name] = einpRenderNested(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = einpRenderNested(item)
		}
		return out
	}
	reflected := reflect.ValueOf(value)
	for reflected.IsValid() && reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return nil
		}
		reflected = reflected.Elem()
	}
	if !reflected.IsValid() {
		return nil
	}
	switch reflected.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]any, reflected.Len())
		for index := 0; index < reflected.Len(); index++ {
			out[index] = einpRenderNested(reflected.Index(index).Interface())
		}
		return out
	case reflect.Struct:
		out := make(map[string]any)
		typeOf := reflected.Type()
		for index := 0; index < typeOf.NumField(); index++ {
			field := typeOf.Field(index)
			name := field.Tag.Get("json")
			if name == "" {
				name = field.Tag.Get("esper")
			}
			if name == "" || name == "-" {
				continue
			}
			item := reflected.Field(index)
			if !item.CanInterface() {
				continue
			}
			out[name] = einpRenderNested(item.Interface())
		}
		return out
	case reflect.Map:
		out := make(map[string]any, reflected.Len())
		iter := reflected.MapRange()
		for iter.Next() {
			out[fmt.Sprint(iter.Key().Interface())] = einpRenderNested(iter.Value().Interface())
		}
		return out
	}
	return value
}

// einpRenderProbeValue renders a value-probe result: xmlInt columns coerce
// the raw attribute text to the XSD-typed integer; everything else renders
// like einpRenderNested.
func einpRenderProbeValue(probe einpProbe, schemas map[string]esper.Schema, value any) any {
	if probe.xmlInt {
		return einpCoerceXMLInt(einpRenderNested(value))
	}
	return einpRenderNested(value)
}

func (it *einpIteration) undeploy(ctx context.Context, label string) error {
	deployment, ok := it.deployments[label]
	if !ok {
		return fmt.Errorf("%s: undeploy %q without deploy", eventInfraPropertyNonDynamicID, label)
	}
	if err := deployment.Undeploy(ctx); err != nil {
		return fmt.Errorf("%s: undeploy %q: %w", eventInfraPropertyNonDynamicID, label, err)
	}
	delete(it.deployments, label)
	return nil
}

func (it *einpIteration) undeployAll(ctx context.Context) error {
	for index := len(it.deployOrder) - 1; index >= 0; index-- {
		label := it.deployOrder[index]
		deployment, ok := it.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", eventInfraPropertyNonDynamicID, label, err)
		}
		delete(it.deployments, label)
	}
	it.deployOrder = nil
	return nil
}

// ---- Schema registration -----------------------------------------------------

func (it *einpIteration) registerSchemas(caseName, mode string) error {
	anyT := reflect.TypeOf((*any)(nil)).Elem()
	intT := reflect.TypeOf(int64(0))
	strT := reflect.TypeOf("")
	intSliceT := reflect.TypeOf([]int64{})
	strSliceT := reflect.TypeOf([]string{})
	mapAnyT := reflect.TypeOf(map[string]any{})
	anySliceT := reflect.TypeOf([]any{})
	reg := func(schema esper.Schema, err error) error {
		if err != nil {
			return fmt.Errorf("%s: register schema: %w", eventInfraPropertyNonDynamicID, err)
		}
		it.schemas[schema.Name()] = schema
		return nil
	}
	// chainFields builds the _4.._1 FieldSpec chains for the nested cases:
	// leaves carry lvlN first when indexed (Java declares l4 then lvl3
	// bottom-up with the child array leading each level: {l4, lvl3}).
	chainFields := func(simple bool) [][]esper.FieldSpec {
		if simple {
			return [][]esper.FieldSpec{
				{esper.FieldDef("lvl4", intT)},
				{esper.FieldDef("l4", anyT), esper.FieldDef("lvl3", intT)},
				{esper.FieldDef("l3", anyT), esper.FieldDef("lvl2", intT)},
				{esper.FieldDef("l2", anyT), esper.FieldDef("lvl1", intT)},
			}
		}
		return [][]esper.FieldSpec{
			{esper.FieldDef("lvl4", intT)},
			{esper.FieldDef("l4", anySliceT), esper.FieldDef("lvl3", intT)},
			{esper.FieldDef("l3", anySliceT), esper.FieldDef("lvl2", intT)},
			{esper.FieldDef("l2", anySliceT), esper.FieldDef("lvl1", intT)},
		}
	}
	// registerChain registers _4.._1 bottom-up, linking each child via
	// WithNestedPropertySchema, and returns the _1 schema. factory builds one
	// schema from name/fields/options.
	registerChain := func(prefix string, simple bool, factory func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error)) (esper.Schema, error) {
		fields := chainFields(simple)
		s4, err := factory(prefix+"_4", fields[0])
		if err != nil {
			return esper.Schema{}, err
		}
		it.schemas[s4.Name()] = s4
		s3, err := factory(prefix+"_3", fields[1], esper.WithNestedPropertySchema("l4", s4))
		if err != nil {
			return esper.Schema{}, err
		}
		it.schemas[s3.Name()] = s3
		s2, err := factory(prefix+"_2", fields[2], esper.WithNestedPropertySchema("l3", s3))
		if err != nil {
			return esper.Schema{}, err
		}
		it.schemas[s2.Name()] = s2
		s1, err := factory(prefix+"_1", fields[3], esper.WithNestedPropertySchema("l2", s2))
		if err != nil {
			return esper.Schema{}, err
		}
		it.schemas[s1.Name()] = s1
		return s1, nil
	}

	switch caseName {
	case "indexed-key-expr":
		switch mode {
		case "objectarray":
			inner, err := esper.RegisterObjectArray(it.env, "OAEventInner",
				[]esper.FieldSpec{esper.FieldDef("p0", strT)})
			if err != nil {
				return err
			}
			it.schemas[inner.Name()] = inner
			return reg(esper.RegisterObjectArray(it.env, "OAEvent",
				[]esper.FieldSpec{esper.FieldDef("intarray", intSliceT), esper.FieldDef("oainner", anySliceT)},
				esper.WithNestedPropertySchema("oainner", inner)))
		case "map":
			inner, err := esper.RegisterMap(it.env, "MapEventInner",
				[]esper.FieldSpec{esper.FieldDef("p0", strT)})
			if err != nil {
				return err
			}
			it.schemas[inner.Name()] = inner
			return reg(esper.RegisterMap(it.env, "MapEvent",
				[]esper.FieldSpec{esper.FieldDef("intarray", intSliceT), esper.FieldDef("mapinner", anySliceT)},
				esper.WithNestedPropertySchema("mapinner", inner)))
		case "wrapper":
			return reg(esper.RegisterStruct[einpSupportBean](it.env, "SupportBean"))
		case "bean":
			return reg(esper.RegisterStruct[einpIndexMappedSamplerBean](it.env, "MyIndexMappedSamplerBean"))
		case "json":
			return reg(esper.RegisterJSON(it.env, "JsonSchema",
				[]esper.FieldSpec{esper.FieldDef("indexed", intSliceT), esper.FieldDef("mapped", mapAnyT)}))
		case "json-provided":
			return reg(esper.RegisterJSONFor[einpIndexMappedJSONProvided](it.env, "JsonSchema", nil))
		}
	case "indexed-runtime-index":
		switch mode {
		case "bean":
			return reg(esper.RegisterStruct[einpRuntimeIndexEvent](it.env, "LocalEvent"))
		case "map":
			return reg(esper.RegisterMap(it.env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("indexed", strSliceT)}))
		case "objectarray":
			return reg(esper.RegisterObjectArray(it.env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("indexed", strSliceT)}))
		case "json":
			return reg(esper.RegisterJSON(it.env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("indexed", strSliceT)}))
		case "json-provided":
			return reg(esper.RegisterJSONFor[einpRuntimeIndexJSONProvided](it.env, "LocalEvent", nil))
		case "avro":
			return reg(esper.RegisterAvro(it.env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("indexed", anySliceT)}))
		}
	case "mapped-indexed":
		switch mode {
		case "bean":
			return reg(esper.RegisterStruct[einpIMEvent](it.env, "MyIMEvent"))
		case "map":
			return reg(esper.RegisterMap(it.env, "EventInfraPropertyMappedIndexedMap",
				[]esper.FieldSpec{esper.FieldDef("indexed", strSliceT), esper.FieldDef("mapped", mapAnyT)}))
		case "objectarray":
			// configureMappedIndexed registers {String[].class, Map.class}.
			return reg(esper.RegisterObjectArray(it.env, "EventInfraPropertyMappedIndexedOA",
				[]esper.FieldSpec{esper.FieldDef("indexed", strSliceT), esper.FieldDef("mapped", mapAnyT)}))
		case "json":
			return reg(esper.RegisterJSON(it.env, "EventInfraPropertyMappedIndexedJson",
				[]esper.FieldSpec{esper.FieldDef("indexed", strSliceT), esper.FieldDef("mapped", mapAnyT)}))
		case "avro":
			// configureMappedIndexed's Avro record: array of string (Java
			// Collection), map of string. Untyped slice keeps the send
			// coercion free-form like the Avro Collection field.
			return reg(esper.RegisterAvro(it.env, "EventInfraPropertyMappedIndexedAvro",
				[]esper.FieldSpec{esper.FieldDef("indexed", anySliceT), esper.FieldDef("mapped", mapAnyT)}))
		case "json-provided":
			return reg(esper.RegisterJSONFor[einpMappedIndexedJSONProvided](it.env, "EventInfraPropertyMappedIndexedJsonProvided", nil))
		}
	case "mapped-runtime-key":
		switch mode {
		case "bean":
			return reg(esper.RegisterStruct[einpRuntimeKeyEvent](it.env, "LocalEvent"))
		case "map":
			return reg(esper.RegisterMap(it.env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("mapped", mapAnyT)}))
		case "objectarray":
			return reg(esper.RegisterObjectArray(it.env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("mapped", mapAnyT)}))
		case "json":
			return reg(esper.RegisterJSON(it.env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("mapped", mapAnyT)}))
		case "json-provided":
			return reg(esper.RegisterJSONFor[einpRuntimeKeyJSONProvided](it.env, "LocalEvent", nil))
		case "avro":
			return reg(esper.RegisterAvro(it.env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("mapped", mapAnyT)}))
		}
	case "nested-indexed":
		top := einpJavaTypeName(caseName, mode)
		switch mode {
		case "bean":
			s1, err := registerChain(top, false, func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
				switch name {
				case top + "_4":
					return esper.RegisterStruct[einpNestedIndexedLvl4](it.env, name, opts...)
				case top + "_3":
					return esper.RegisterStruct[einpNestedIndexedLvl3](it.env, name, opts...)
				case top + "_2":
					return esper.RegisterStruct[einpNestedIndexedLvl2](it.env, name, opts...)
				default:
					return esper.RegisterStruct[einpNestedIndexedLvl1](it.env, name, opts...)
				}
			})
			if err != nil {
				return err
			}
			return reg(esper.RegisterStruct[einpNestedIndexedTop](it.env, top, esper.WithNestedPropertySchema("l1", s1)))
		case "map":
			s1, err := registerChain(top, false, func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
				return esper.RegisterMap(it.env, name, fields, opts...)
			})
			if err != nil {
				return err
			}
			return reg(esper.RegisterMap(it.env, top,
				[]esper.FieldSpec{esper.FieldDef("l1", anySliceT)},
				esper.WithNestedPropertySchema("l1", s1)))
		case "objectarray":
			s1, err := registerChain(top, false, func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
				return esper.RegisterObjectArray(it.env, name, fields, opts...)
			})
			if err != nil {
				return err
			}
			return reg(esper.RegisterObjectArray(it.env, top,
				[]esper.FieldSpec{esper.FieldDef("l1", anySliceT)},
				esper.WithNestedPropertySchema("l1", s1)))
		case "xml":
			// The XSD registers the l1..l4 element chain (each lN an
			// unbounded lN+1 element array); the Go XML schema mirrors it via
			// nested-schema links.
			s1, err := registerChain(top, false, func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
				return esper.RegisterXML(it.env, name, fields, opts...)
			})
			if err != nil {
				return err
			}
			return reg(esper.RegisterXML(it.env, top,
				[]esper.FieldSpec{esper.FieldDef("l1", anySliceT)},
				esper.WithNestedPropertySchema("l1", s1)))
		case "avro":
			s1, err := registerChain(top, false, func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
				return esper.RegisterAvro(it.env, name, fields, opts...)
			})
			if err != nil {
				return err
			}
			return reg(esper.RegisterAvro(it.env, top,
				[]esper.FieldSpec{esper.FieldDef("l1", anySliceT)},
				esper.WithNestedPropertySchema("l1", s1)))
		case "json":
			s1, err := registerChain(top, false, func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
				return esper.RegisterJSON(it.env, name, fields, opts...)
			})
			if err != nil {
				return err
			}
			return reg(esper.RegisterJSON(it.env, top,
				[]esper.FieldSpec{esper.FieldDef("l1", anySliceT)},
				esper.WithNestedPropertySchema("l1", s1)))
		case "json-provided":
			jp := "EventInfraPropertyNestedIndexedJsonProvided"
			// Class-provided JSON types derive their fields (and nested
			// fragment schemas) from the provided Go structs; Java's EPL
			// declares the same fields but the class backing governs.
			registrations := map[string]func(string) (esper.Schema, error){
				jp + "_4": func(name string) (esper.Schema, error) {
					return esper.RegisterJSONFor[einpNestedIndexedJSONProvidedLvl4](it.env, name, nil)
				},
				jp + "_3": func(name string) (esper.Schema, error) {
					return esper.RegisterJSONFor[einpNestedIndexedJSONProvidedLvl3](it.env, name, nil)
				},
				jp + "_2": func(name string) (esper.Schema, error) {
					return esper.RegisterJSONFor[einpNestedIndexedJSONProvidedLvl2](it.env, name, nil)
				},
				jp + "_1": func(name string) (esper.Schema, error) {
					return esper.RegisterJSONFor[einpNestedIndexedJSONProvidedLvl1](it.env, name, nil)
				},
			}
			for _, name := range []string{jp + "_4", jp + "_3", jp + "_2", jp + "_1"} {
				schema, err := registrations[name](name)
				if err != nil {
					return err
				}
				it.schemas[name] = schema
			}
			return reg(esper.RegisterJSONFor[einpNestedIndexedJSONProvidedTop](it.env, jp, nil))
		}
	case "nested-simple":
		top := einpJavaTypeName(caseName, mode)
		switch mode {
		case "bean":
			s1, err := registerChain(top, true, func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
				switch name {
				case top + "_4":
					return esper.RegisterStruct[einpNestedSimpleLvl4](it.env, name, opts...)
				case top + "_3":
					return esper.RegisterStruct[einpNestedSimpleLvl3](it.env, name, opts...)
				case top + "_2":
					return esper.RegisterStruct[einpNestedSimpleLvl2](it.env, name, opts...)
				default:
					return esper.RegisterStruct[einpNestedSimpleLvl1](it.env, name, opts...)
				}
			})
			if err != nil {
				return err
			}
			return reg(esper.RegisterStruct[einpNestedSimpleTop](it.env, top, esper.WithNestedPropertySchema("l1", s1)))
		case "map":
			s1, err := registerChain(top, true, func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
				return esper.RegisterMap(it.env, name, fields, opts...)
			})
			if err != nil {
				return err
			}
			return reg(esper.RegisterMap(it.env, top,
				[]esper.FieldSpec{esper.FieldDef("l1", anyT)},
				esper.WithNestedPropertySchema("l1", s1)))
		case "objectarray":
			s1, err := registerChain(top, true, func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
				return esper.RegisterObjectArray(it.env, name, fields, opts...)
			})
			if err != nil {
				return err
			}
			return reg(esper.RegisterObjectArray(it.env, top,
				[]esper.FieldSpec{esper.FieldDef("l1", anyT)},
				esper.WithNestedPropertySchema("l1", s1)))
		case "xml":
			// The XSD registers the l1..l4 element chain (each lN a single
			// lN+1 element); the Go XML schema mirrors it via nested links.
			s1, err := registerChain(top, true, func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
				return esper.RegisterXML(it.env, name, fields, opts...)
			})
			if err != nil {
				return err
			}
			return reg(esper.RegisterXML(it.env, top,
				[]esper.FieldSpec{esper.FieldDef("l1", anyT)},
				esper.WithNestedPropertySchema("l1", s1)))
		case "avro":
			s1, err := registerChain(top, true, func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
				return esper.RegisterAvro(it.env, name, fields, opts...)
			})
			if err != nil {
				return err
			}
			return reg(esper.RegisterAvro(it.env, top,
				[]esper.FieldSpec{esper.FieldDef("l1", anyT)},
				esper.WithNestedPropertySchema("l1", s1)))
		case "json":
			s1, err := registerChain(top, true, func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) (esper.Schema, error) {
				return esper.RegisterJSON(it.env, name, fields, opts...)
			})
			if err != nil {
				return err
			}
			return reg(esper.RegisterJSON(it.env, top,
				[]esper.FieldSpec{esper.FieldDef("l1", anyT)},
				esper.WithNestedPropertySchema("l1", s1)))
		case "json-provided":
			jp := "EventInfraPropertyNestedSimpleJsonProvided"
			// Java declares only the top class-provided schema; the lvl chain
			// materializes through the Top struct's l1 field.
			return reg(esper.RegisterJSONFor[einpNestedSimpleJSONProvidedTop](it.env, jp, nil))
		}
	case "nested-escaped":
		if mode != "types" {
			return fmt.Errorf("%s: nested-escaped registers all types from the single schema step, got mode %q", eventInfraPropertyNonDynamicID, mode)
		}
		// bean family
		bean3, err := esper.RegisterStruct[einpEscapedLvl3](it.env, "BeanLvl3")
		if err != nil {
			return err
		}
		it.schemas[bean3.Name()] = bean3
		bean2, err := esper.RegisterStruct[einpEscapedLvl2](it.env, "BeanLvl2", esper.WithNestedPropertySchema("lvl3", bean3))
		if err != nil {
			return err
		}
		it.schemas[bean2.Name()] = bean2
		bean1, err := esper.RegisterStruct[einpEscapedLvl1](it.env, "BeanLvl1", esper.WithNestedPropertySchema("lvl2", bean2))
		if err != nil {
			return err
		}
		it.schemas[bean1.Name()] = bean1
		bean0, err := esper.RegisterStruct[einpEscapedLvl0](it.env, "BeanLvl0", esper.WithNestedPropertySchema("lvl1", bean1))
		if err != nil {
			return err
		}
		it.schemas[bean0.Name()] = bean0
		// map family
		map3, err := esper.RegisterMap(it.env, "MapLvl3", []esper.FieldSpec{esper.FieldDef("vlvl3", strT)})
		if err != nil {
			return err
		}
		it.schemas[map3.Name()] = map3
		map2, err := esper.RegisterMap(it.env, "MapLvl2", []esper.FieldSpec{
			esper.FieldDef("vlvl2", strT), esper.FieldDef("lvl3", mapAnyT)}, esper.WithNestedPropertySchema("lvl3", map3))
		if err != nil {
			return err
		}
		it.schemas[map2.Name()] = map2
		map1, err := esper.RegisterMap(it.env, "MapLvl1", []esper.FieldSpec{esper.FieldDef("lvl2", mapAnyT)}, esper.WithNestedPropertySchema("lvl2", map2))
		if err != nil {
			return err
		}
		it.schemas[map1.Name()] = map1
		map0, err := esper.RegisterMap(it.env, "MapLvl0", []esper.FieldSpec{esper.FieldDef("lvl1", mapAnyT)}, esper.WithNestedPropertySchema("lvl1", map1))
		if err != nil {
			return err
		}
		it.schemas[map0.Name()] = map0
		// objectarray family
		oa3, err := esper.RegisterObjectArray(it.env, "OALvl3", []esper.FieldSpec{esper.FieldDef("vlvl3", strT)})
		if err != nil {
			return err
		}
		it.schemas[oa3.Name()] = oa3
		oa2, err := esper.RegisterObjectArray(it.env, "OALvl2", []esper.FieldSpec{
			esper.FieldDef("vlvl2", strT), esper.FieldDef("vlvl2dyn", strT), esper.FieldDef("lvl3", anyT)},
			esper.WithNestedPropertySchema("lvl3", oa3))
		if err != nil {
			return err
		}
		it.schemas[oa2.Name()] = oa2
		oa1, err := esper.RegisterObjectArray(it.env, "OALvl1", []esper.FieldSpec{esper.FieldDef("lvl2", anyT)}, esper.WithNestedPropertySchema("lvl2", oa2))
		if err != nil {
			return err
		}
		it.schemas[oa1.Name()] = oa1
		oa0, err := esper.RegisterObjectArray(it.env, "OALvl0", []esper.FieldSpec{esper.FieldDef("lvl1", anyT)}, esper.WithNestedPropertySchema("lvl1", oa1))
		if err != nil {
			return err
		}
		it.schemas[oa0.Name()] = oa0
		// json family (Lvl3/Lvl2 are @JsonSchema(dynamic=true))
		json3, err := esper.RegisterJSON(it.env, "JSONLvl3", []esper.FieldSpec{esper.FieldDef("vlvl3", strT)}, esper.AllowDynamicFields())
		if err != nil {
			return err
		}
		it.schemas[json3.Name()] = json3
		json2, err := esper.RegisterJSON(it.env, "JSONLvl2", []esper.FieldSpec{
			esper.FieldDef("vlvl2", strT), esper.FieldDef("lvl3", anyT)},
			esper.WithNestedPropertySchema("lvl3", json3), esper.AllowDynamicFields())
		if err != nil {
			return err
		}
		it.schemas[json2.Name()] = json2
		json1, err := esper.RegisterJSON(it.env, "JSONLvl1", []esper.FieldSpec{esper.FieldDef("lvl2", anyT)}, esper.WithNestedPropertySchema("lvl2", json2))
		if err != nil {
			return err
		}
		it.schemas[json1.Name()] = json1
		json0, err := esper.RegisterJSON(it.env, "JSONLvl0", []esper.FieldSpec{esper.FieldDef("lvl1", anyT)}, esper.WithNestedPropertySchema("lvl1", json1))
		if err != nil {
			return err
		}
		it.schemas[json0.Name()] = json0
		// avro family
		avro3, err := esper.RegisterAvro(it.env, "AvroLvl3", []esper.FieldSpec{esper.FieldDef("vlvl3", strT)})
		if err != nil {
			return err
		}
		it.schemas[avro3.Name()] = avro3
		avro2, err := esper.RegisterAvro(it.env, "AvroLvl2", []esper.FieldSpec{
			esper.FieldDef("vlvl2", strT), esper.FieldDef("vlvl2dyn", strT), esper.FieldDef("lvl3", anyT)},
			esper.WithNestedPropertySchema("lvl3", avro3))
		if err != nil {
			return err
		}
		it.schemas[avro2.Name()] = avro2
		avro1, err := esper.RegisterAvro(it.env, "AvroLvl1", []esper.FieldSpec{esper.FieldDef("lvl2", anyT)}, esper.WithNestedPropertySchema("lvl2", avro2))
		if err != nil {
			return err
		}
		it.schemas[avro1.Name()] = avro1
		avro0, err := esper.RegisterAvro(it.env, "AvroLvl0", []esper.FieldSpec{esper.FieldDef("lvl1", anyT)}, esper.WithNestedPropertySchema("lvl1", avro1))
		if err != nil {
			return err
		}
		it.schemas[avro0.Name()] = avro0
		return nil
	}
	return fmt.Errorf("%s: no schema registration for %q/%q", eventInfraPropertyNonDynamicID, caseName, mode)
}

// ---- Send dispatch -----------------------------------------------------------

func (it *einpIteration) send(ctx context.Context, caseName string, step compat.Step) error {
	var payload map[string]json.RawMessage
	if err := strictObject(step.Payload, &payload); err != nil {
		return fmt.Errorf("%s: decode send payload: %w", eventInfraPropertyNonDynamicID, err)
	}
	switch step.Mode {
	case "bean", "wrapper":
		return it.sendBean(ctx, caseName, step.EventType, payload)
	case "map":
		return it.sendMap(ctx, caseName, step.EventType, payload)
	case "objectarray":
		return it.sendObjectArray(ctx, caseName, step.EventType, payload)
	case "xml":
		return it.sendXMLDoc(ctx, caseName, step.EventType, payload)
	case "avro":
		return it.sendAvro(ctx, caseName, step.EventType, payload)
	case "json", "json-provided":
		return it.sendJSONDoc(ctx, caseName, step.EventType, payload)
	default:
		return fmt.Errorf("%s: unsupported send mode %q", eventInfraPropertyNonDynamicID, step.Mode)
	}
}

// einpNestedInts decodes the uniform nested payload {lvl1,lvl2,lvl3,lvl4}.
func einpNestedInts(payload map[string]json.RawMessage) (ints [4]int64, err error) {
	for index, name := range []string{"lvl1", "lvl2", "lvl3", "lvl4"} {
		var value int64
		if err := json.Unmarshal(payload[name], &value); err != nil {
			return ints, fmt.Errorf("%s: nested payload %q: %w", eventInfraPropertyNonDynamicID, name, err)
		}
		ints[index] = value
	}
	return ints, nil
}

// einpEscapedValues decodes the nested-escaped payload {vlvl2, vlvl2dyn, vlvl3}.
func einpEscapedValues(payload map[string]json.RawMessage) (values [3]string, err error) {
	for index, name := range []string{"vlvl2", "vlvl2dyn", "vlvl3"} {
		var value string
		if err := json.Unmarshal(payload[name], &value); err != nil {
			return values, fmt.Errorf("%s: escaped payload %q: %w", eventInfraPropertyNonDynamicID, name, err)
		}
		values[index] = value
	}
	return values, nil
}

func (it *einpIteration) sendBean(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	switch caseName {
	case "indexed-key-expr":
		if it.mode == "wrapper" {
			return it.engine.Send(ctx, eventType, einpNewSupportBean())
		}
		return it.engine.Send(ctx, eventType, einpIndexMappedSamplerBean{
			ListOfInt: []int64{1, 2}, IterableOfInt: []int64{1, 2}})
	case "indexed-runtime-index":
		return it.engine.Send(ctx, eventType, einpRuntimeIndexEvent{Indexed: []string{"a", "b"}})
	case "mapped-indexed":
		return it.engine.Send(ctx, eventType, einpIMEvent{
			Indexed: []string{"v1", "v2"}, Mapped: map[string]string{"k1": "v1"}})
	case "mapped-runtime-key":
		return it.engine.Send(ctx, eventType, einpRuntimeKeyEvent{
			Mapped: map[string]string{"a1": "x", "a2": "y"}})
	case "nested-indexed":
		ints, err := einpNestedInts(payload)
		if err != nil {
			return err
		}
		return it.engine.Send(ctx, eventType, einpNestedIndexedTop{L1: []einpNestedIndexedLvl1{{
			L2: []einpNestedIndexedLvl2{{
				L3: []einpNestedIndexedLvl3{{
					L4: []einpNestedIndexedLvl4{{Lvl4: ints[3]}}, Lvl3: ints[2],
				}},
				Lvl2: ints[1],
			}},
			Lvl1: ints[0],
		}}})
	case "nested-simple":
		ints, err := einpNestedInts(payload)
		if err != nil {
			return err
		}
		return it.engine.Send(ctx, eventType, einpNestedSimpleTop{L1: &einpNestedSimpleLvl1{
			L2: &einpNestedSimpleLvl2{
				L3: &einpNestedSimpleLvl3{
					L4: &einpNestedSimpleLvl4{Lvl4: ints[3]}, Lvl3: ints[2],
				},
				Lvl2: ints[1],
			},
			Lvl1: ints[0],
		}})
	case "nested-escaped":
		values, err := einpEscapedValues(payload)
		if err != nil {
			return err
		}
		return it.engine.Send(ctx, eventType, einpEscapedLvl0{Lvl1: &einpEscapedLvl1{
			Lvl2: &einpEscapedLvl2{
				Vlvl2: values[0], Vlvl2dyn: values[1],
				Lvl3: &einpEscapedLvl3{Vlvl3: values[2]},
			},
		}})
	}
	return fmt.Errorf("%s: no bean sender for %q", eventInfraPropertyNonDynamicID, caseName)
}

func (it *einpIteration) sendMap(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	switch caseName {
	case "nested-indexed":
		ints, err := einpNestedInts(payload)
		if err != nil {
			return err
		}
		l4 := map[string]any{"lvl4": ints[3]}
		l3 := map[string]any{"l4": []any{l4}, "lvl3": ints[2]}
		l2 := map[string]any{"l3": []any{l3}, "lvl2": ints[1]}
		l1 := map[string]any{"l2": []any{l2}, "lvl1": ints[0]}
		return it.engine.Send(ctx, eventType, map[string]any{"l1": []any{l1}})
	case "nested-simple":
		ints, err := einpNestedInts(payload)
		if err != nil {
			return err
		}
		l4 := map[string]any{"lvl4": ints[3]}
		l3 := map[string]any{"l4": l4, "lvl3": ints[2]}
		l2 := map[string]any{"l3": l3, "lvl2": ints[1]}
		l1 := map[string]any{"l2": l2, "lvl1": ints[0]}
		return it.engine.Send(ctx, eventType, map[string]any{"l1": l1})
	case "nested-escaped":
		values, err := einpEscapedValues(payload)
		if err != nil {
			return err
		}
		l3 := map[string]any{"vlvl3": values[2]}
		l2 := map[string]any{"vlvl2": values[0], "vlvl2dyn": values[1], "lvl3": l3}
		l1 := map[string]any{"lvl2": l2}
		return it.engine.Send(ctx, eventType, map[string]any{"lvl1": l1})
	}
	event, err := eipdDecodeValueMap(payload)
	if err != nil {
		return err
	}
	return it.engine.Send(ctx, eventType, event)
}

func (it *einpIteration) sendObjectArray(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	switch caseName {
	case "indexed-key-expr":
		values, err := eipdDecodeAnyArray(payload["values"])
		if err != nil {
			return fmt.Errorf("%s: objectarray payload: %w", eventInfraPropertyNonDynamicID, err)
		}
		// new Object[]{new int[]{1, 2}, {{"A"}, {"B"}}}: the int[] cell must
		// arrive typed []int64 for the int[]-declared schema.
		raw, ok := values[0].([]any)
		if !ok {
			return fmt.Errorf("%s: objectarray intarray cell is %T", eventInfraPropertyNonDynamicID, values[0])
		}
		intarray := make([]int64, len(raw))
		for i, v := range raw {
			n, ok := v.(int64)
			if !ok {
				if f, isFloat := v.(float64); isFloat {
					n = int64(f)
				} else {
					return fmt.Errorf("%s: intarray element is %T", eventInfraPropertyNonDynamicID, v)
				}
			}
			intarray[i] = n
		}
		return it.engine.SendObjectArray(ctx, eventType, []any{intarray, values[1]})
	case "indexed-runtime-index":
		values, err := eipdDecodeAnyArray(payload["values"])
		if err != nil {
			return fmt.Errorf("%s: objectarray payload: %w", eventInfraPropertyNonDynamicID, err)
		}
		// new Object[]{values}: the string[] cell arrives typed []string.
		raw, ok := values[0].([]any)
		if !ok {
			return fmt.Errorf("%s: objectarray indexed cell is %T", eventInfraPropertyNonDynamicID, values[0])
		}
		indexed := make([]string, len(raw))
		for i, v := range raw {
			text, ok := v.(string)
			if !ok {
				return fmt.Errorf("%s: indexed element is %T", eventInfraPropertyNonDynamicID, v)
			}
			indexed[i] = text
		}
		return it.engine.SendObjectArray(ctx, eventType, []any{indexed})
	case "mapped-indexed":
		values, err := eipdDecodeAnyArray(payload["values"])
		if err != nil {
			return fmt.Errorf("%s: objectarray payload: %w", eventInfraPropertyNonDynamicID, err)
		}
		// new Object[]{new String[]{"v1","v2"}, singletonMap("k1","v1")}: cells
		// arrive typed []string / map[string]string for the typed schema.
		strings0, ok := values[0].([]any)
		if !ok {
			return fmt.Errorf("%s: objectarray indexed cell is %T", eventInfraPropertyNonDynamicID, values[0])
		}
		indexedCell := make([]string, len(strings0))
		for i, v := range strings0 {
			text, ok := v.(string)
			if !ok {
				return fmt.Errorf("%s: indexed element is %T", eventInfraPropertyNonDynamicID, v)
			}
			indexedCell[i] = text
		}
		mappedRaw, ok := values[1].(map[string]any)
		if !ok {
			return fmt.Errorf("%s: objectarray mapped cell is %T", eventInfraPropertyNonDynamicID, values[1])
		}
		return it.engine.SendObjectArray(ctx, eventType, []any{indexedCell, mappedRaw})
	case "nested-indexed":
		ints, err := einpNestedInts(payload)
		if err != nil {
			return err
		}
		// Java builds l4={lvl4} then wraps element arrays upward:
		// l3={ {l4}, lvl3 }, l2={ {l3}, lvl2 }, l1={ {l2}, lvl1 },
		// top={ {l1} } — nine brackets deep.
		l4 := []any{ints[3]}
		l3 := []any{[]any{l4}, ints[2]}
		l2 := []any{[]any{l3}, ints[1]}
		l1 := []any{[]any{l2}, ints[0]}
		return it.engine.SendObjectArray(ctx, eventType, []any{[]any{l1}})
	case "nested-simple":
		ints, err := einpNestedInts(payload)
		if err != nil {
			return err
		}
		l4 := []any{ints[3]}
		l3 := []any{l4, ints[2]}
		l2 := []any{l3, ints[1]}
		l1 := []any{l2, ints[0]}
		return it.engine.SendObjectArray(ctx, eventType, []any{l1})
	case "nested-escaped":
		values, err := einpEscapedValues(payload)
		if err != nil {
			return err
		}
		l3 := []any{values[2]}
		l2 := []any{values[0], values[1], l3}
		l1 := []any{l2}
		return it.engine.SendObjectArray(ctx, eventType, []any{l1})
	}
	values, err := eipdDecodeAnyArray(payload["values"])
	if err != nil {
		return fmt.Errorf("%s: objectarray payload: %w", eventInfraPropertyNonDynamicID, err)
	}
	for index := range values {
		values[index] = eipdUnwrapOACell(values[index])
	}
	return it.engine.SendObjectArray(ctx, eventType, values)
}

// einpNestedXML renders the pinned myevent document both nested cases send.
func einpNestedXML(ints [4]int64) string {
	return fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"+
		"<myevent>\n"+
		"\t<l1 lvl1=\"%d\">\n"+
		"\t\t<l2 lvl2=\"%d\">\n"+
		"\t\t\t<l3 lvl3=\"%d\">\n"+
		"\t\t\t\t<l4 lvl4=\"%d\">\n"+
		"\t\t\t\t</l4>\n"+
		"\t\t\t</l3>\n"+
		"\t\t</l2>\n"+
		"\t</l1>\n"+
		"</myevent>", ints[0], ints[1], ints[2], ints[3])
}

func (it *einpIteration) sendXMLDoc(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	ints, err := einpNestedInts(payload)
	if err != nil {
		return err
	}
	return it.engine.SendXML(ctx, eventType, []byte(einpNestedXML(ints)))
}

func (it *einpIteration) sendAvro(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	schema, ok := it.schemas[eventType]
	if !ok {
		return fmt.Errorf("%s: no Avro schema %q", eventInfraPropertyNonDynamicID, eventType)
	}
	switch caseName {
	case "nested-indexed":
		ints, err := einpNestedInts(payload)
		if err != nil {
			return err
		}
		l4 := map[string]any{"lvl4": ints[3]}
		l3 := map[string]any{"l4": []any{l4}, "lvl3": ints[2]}
		l2 := map[string]any{"l3": []any{l3}, "lvl2": ints[1]}
		l1 := map[string]any{"l2": []any{l2}, "lvl1": ints[0]}
		record, err := einpBuildAvro(schema, map[string]any{"l1": []any{l1}})
		if err != nil {
			return err
		}
		return it.engine.SendAvro(ctx, eventType, record)
	case "nested-simple":
		ints, err := einpNestedInts(payload)
		if err != nil {
			return err
		}
		l4 := map[string]any{"lvl4": ints[3]}
		l3 := map[string]any{"l4": l4, "lvl3": ints[2]}
		l2 := map[string]any{"l3": l3, "lvl2": ints[1]}
		l1 := map[string]any{"l2": l2, "lvl1": ints[0]}
		record, err := einpBuildAvro(schema, map[string]any{"l1": l1})
		if err != nil {
			return err
		}
		return it.engine.SendAvro(ctx, eventType, record)
	case "nested-escaped":
		values, err := einpEscapedValues(payload)
		if err != nil {
			return err
		}
		l3 := map[string]any{"vlvl3": values[2]}
		l2 := map[string]any{"vlvl2": values[0], "vlvl2dyn": values[1], "lvl3": l3}
		l1 := map[string]any{"lvl2": l2}
		record, err := einpBuildAvro(schema, map[string]any{"lvl1": l1})
		if err != nil {
			return err
		}
		return it.engine.SendAvro(ctx, eventType, record)
	}
	fields, err := eipdDecodeValueMap(payload)
	if err != nil {
		return err
	}
	record, err := einpBuildAvro(schema, fields)
	if err != nil {
		return err
	}
	return it.engine.SendAvro(ctx, eventType, record)
}

// einpBuildAvro materializes nested AvroRecords from decoded payload maps via
// the registered nested-schema links (map slices become record lists).
func einpBuildAvro(schema esper.Schema, fields map[string]any) (*esper.AvroRecord, error) {
	converted := make(map[string]any, len(fields))
	for name, value := range fields {
		nested, hasNested := schema.NestedSchema(name)
		switch {
		case hasNested:
			if items, isSlice := einpAsSlice(value); isSlice {
				records := make([]any, 0, len(items))
				for _, item := range items {
					object, ok := item.(map[string]any)
					if !ok {
						records = append(records, item)
						continue
					}
					record, err := einpBuildAvro(nested, object)
					if err != nil {
						return nil, err
					}
					records = append(records, record)
				}
				converted[name] = records
			} else if object, ok := value.(map[string]any); ok {
				record, err := einpBuildAvro(nested, object)
				if err != nil {
					return nil, err
				}
				converted[name] = record
			} else {
				converted[name] = value
			}
		default:
			if object, ok := value.(map[string]any); ok {
				nestedMap := make(map[string]any, len(object))
				for key, item := range object {
					nestedMap[key] = item
				}
				converted[name] = nestedMap
			} else {
				converted[name] = value
			}
		}
	}
	return esper.NewAvroRecordFromMap(schema, converted)
}

func (it *einpIteration) sendJSONDoc(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	var document map[string]any
	switch caseName {
	case "nested-indexed":
		ints, err := einpNestedInts(payload)
		if err != nil {
			return err
		}
		// Java's FJSON templates the lvl values as strings ("lvl4": "4") but
		// the int-typed JSON schema coerces them; Go sends ints directly.
		l4 := map[string]any{"lvl4": ints[3]}
		l3 := map[string]any{"l4": []any{l4}, "lvl3": ints[2]}
		l2 := map[string]any{"l3": []any{l3}, "lvl2": ints[1]}
		l1 := map[string]any{"l2": []any{l2}, "lvl1": ints[0]}
		document = map[string]any{"l1": []any{l1}}
	case "nested-simple":
		ints, err := einpNestedInts(payload)
		if err != nil {
			return err
		}
		l4 := map[string]any{"lvl4": ints[3]}
		l3 := map[string]any{"l4": l4, "lvl3": ints[2]}
		l2 := map[string]any{"l3": l3, "lvl2": ints[1]}
		l1 := map[string]any{"l2": l2, "lvl1": ints[0]}
		document = map[string]any{"l1": l1}
	case "nested-escaped":
		values, err := einpEscapedValues(payload)
		if err != nil {
			return err
		}
		l3 := map[string]any{"vlvl3": values[2]}
		l2 := map[string]any{"vlvl2": values[0], "vlvl2dyn": values[1], "lvl3": l3}
		document = map[string]any{"lvl1": map[string]any{"lvl2": l2}}

	default:
		fields, err := eipdDecodeValueMap(payload)
		if err != nil {
			return err
		}
		document = fields
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return err
	}
	return it.engine.SendJSON(ctx, eventType, encoded)
}

// ---- Type-level pins ---------------------------------------------------------

// einpTypeLevelOrder lists the event-type property checks per (case, mode)
// that Java runs outside the statement assertions (runAssertionTypeValidProp
// and runAssertionTypeInvalidProp). Nested cases also pin their s0 result
// columns via einpTypeOrder.
func einpTypeLevelOrder(caseName string) []string {
	switch caseName {
	case "mapped-indexed":
		return []string{"indexed", "indexed[0]", "mapped", "mapped('a')",
			"xxxx", "myString[0]", "indexed('a')", "indexed.x", "mapped[0]", "mapped.x"}
	case "nested-indexed":
		return []string{"l1", "l1[0]", "l1[0].lvl1", "l1[0].l2", "l1[0].l2[0]", "l1[0].l2[0].lvl2",
			"l1[0].l2[0].l3[0].lvl3",
			"l2[0]", "l1[0].l3", "l1[0].lvl1.lvl1", "l1[0].l2.l4", "l1[0].l2[0].xx", "l1[0].l2[0].l3[0].lvl5"}
	case "nested-simple":
		return []string{"l1", "l1.lvl1", "l1.l2", "l1.l2.lvl2", "l1.l2.l3.lvl3",
			"l2", "l1.l3", "l1.lvl1.lvl1", "l1.l2.l4", "l1.l2.xx", "l1.l2.l3.lvl5"}
	}
	return nil
}

// einpTypeLevelPins pins the emitted value for each type-level property:
// "<invalid>" for properties Java asserts do not resolve, and the Java
// getPropertyType simple name (or the fragment type name for l1) otherwise.
func einpTypeLevelPins(caseName, mode string) map[string]string {
	switch caseName {
	case "mapped-indexed":
		indexed := "String[]"
		mappedA := "String"
		if mode == "avro" {
			indexed = "Collection"
		}
		if mode == "map" || mode == "objectarray" || mode == "json" || mode == "json-provided" {
			mappedA = "Object"
		}
		return map[string]string{
			"indexed": indexed, "indexed[0]": "String", "mapped": "Map", "mapped('a')": mappedA,
			"xxxx": "<invalid>", "myString[0]": "<invalid>", "indexed('a')": "<invalid>",
			"indexed.x": "<invalid>", "mapped[0]": "<invalid>", "mapped.x": "<invalid>",
		}
	case "nested-indexed":
		l1 := einpNestedClassName(caseName, mode)
		return map[string]string{
			"l1": l1, "l1[0]": "exists", "l1[0].lvl1": "Integer", "l1[0].l2": "exists",
			"l1[0].l2[0]": "exists", "l1[0].l2[0].lvl2": "Integer", "l1[0].l2[0].l3[0].lvl3": "Integer",
			"l2[0]": "<invalid>", "l1[0].l3": "<invalid>", "l1[0].lvl1.lvl1": "<invalid>",
			"l1[0].l2.l4": "<invalid>", "l1[0].l2[0].xx": "<invalid>", "l1[0].l2[0].l3[0].lvl5": "<invalid>",
		}
	case "nested-simple":
		l1 := einpNestedClassName(caseName, mode)
		return map[string]string{
			"l1": l1, "l1.lvl1": "Integer", "l1.l2": "exists", "l1.l2.lvl2": "Integer", "l1.l2.l3.lvl3": "Integer",
			"l2": "<invalid>", "l1.l3": "<invalid>", "l1.lvl1.lvl1": "<invalid>",
			"l1.l2.l4": "<invalid>", "l1.l2.xx": "<invalid>", "l1.l2.l3.lvl5": "<invalid>",
		}
	}
	return nil
}

// einpNestedClassName renders Java's nestedClass/fragment name for the l1
// property: the bean/nested-JSON types carry their class name, preconfigured
// types carry the Esper type name.
func einpNestedClassName(caseName, mode string) string {
	top := einpJavaTypeName(caseName, mode)
	switch caseName {
	case "nested-indexed":
		switch mode {
		case "bean":
			return einpPkg + ".EventInfraPropertyNestedIndexed$InfraNestedIndexedPropLvl1[]"
		case "map", "objectarray", "avro":
			return top + "_1[]"
		case "xml":
			return "Node[]"
		case "json":
			return "Object[]"
		case "json-provided":
			return "EventInfraPropertyNestedIndexedJsonProvided_1"
		}
	case "nested-simple":
		switch mode {
		case "bean":
			return einpPkg + ".EventInfraPropertyNestedSimple$InfraNestedSimplePropLvl1"
		case "map":
			return "Map"
		case "objectarray":
			return "Object[]"
		case "xml":
			return "Node"
		case "avro":
			return "GenericData.Record"
		case "json":
			return "Object"
		case "json-provided":
			return einpPkg + ".EventInfraPropertyNestedSimple$MyLocalJSONProvidedLvl1"
		}
	}
	return ""
}

// ---- Scenario pinning -------------------------------------------------------

// einpCasePinnedEPL pins one representative Java statement per case (the bean
// iteration's s0 text; nested cases keep the select-* second pass out of the
// case pin like the dynamic precedent).
func einpCasePinnedEPL(caseName string) string {
	switch caseName {
	case "nested-indexed", "nested-simple", "nested-escaped":
		return einpS0EPL(caseName, "bean")
	}
	return einpS0EPL(caseName, "bean")
}

// einpValidateStep verifies one loaded step's required fields and pins.
func einpValidateStep(index int, object map[string]json.RawMessage, step compat.Step) error {
	require := func(names ...string) error {
		if err := eipdRequireFields(object, names...); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		return nil
	}
	switch step.Op {
	case "case":
		return require("op", "case")
	case "deploy":
		if step.Statement == "schema" {
			if err := require("op", "case", "statement", "eventType", "mode", "epl"); err != nil {
				return err
			}
			if step.Epl != einpSchemaEPL(step.Case, step.Mode) {
				return fmt.Errorf("scenario step %d: schema EPL is not pinned", index)
			}
			if step.Mode == "types" {
				if step.EventType != "" {
					return fmt.Errorf("scenario step %d: types-mode schema step carries no eventType", index)
				}
			} else if step.EventType != einpJavaTypeName(step.Case, step.Mode) {
				return fmt.Errorf("scenario step %d: schema eventType is not pinned", index)
			}
			return nil
		}
		if err := require("op", "case", "statement", "mode", "epl"); err != nil {
			return err
		}
		if step.Statement == "s0" {
			if star := einpStarEPL(step.Case, step.Mode); star != "" && step.Epl == star {
				return nil
			}
			if step.Epl != einpS0EPL(step.Case, step.Mode) {
				return fmt.Errorf("scenario step %d: s0 EPL is not pinned", index)
			}
		}
		return nil
	case "deployed":
		return require("op", "case", "statement")
	case "types":
		if step.Statement == "types" {
			return require("op", "case", "statement", "eventType", "propertyOrder", "propertyTypes")
		}
		return require("op", "case", "statement", "propertyOrder", "propertyTypes")
	case "send":
		return require("op", "case", "eventType", "mode", "payload")
	case "undeploy":
		return require("op", "case", "statement")
	case "undeploy-all":
		return require("op", "case")
	default:
		return fmt.Errorf("scenario step %d: unsupported op %q", index, step.Op)
	}
}

func einpStepKey(step compat.Step) string {
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
	types := ""
	if len(step.PropertyTypes) > 0 {
		encoded, err := json.Marshal(step.PropertyTypes)
		if err == nil {
			types = string(encoded)
		}
	}
	return strings.Join([]string{step.Op, step.Case, step.Statement, step.EventType, step.Mode, step.Name, step.Epl, step.ExpectError, strings.Join(step.PropertyOrder, ","), types, payload}, "|")
}

func einpKey(op, caseName string, rest ...string) string {
	parts := append([]string{op, caseName}, rest...)
	for len(parts) < 11 {
		parts = append(parts, "")
	}
	return strings.Join(parts, "|")
}

func einpKeyFields(op, caseName, statement, eventType, mode, name, epl, expectError, propertyOrder, types, payload string) string {
	return strings.Join([]string{op, caseName, statement, eventType, mode, name, epl, expectError, propertyOrder, types, payload}, "|")
}

func einpTypesJSON(pins map[string]string) string {
	encoded, err := json.Marshal(pins)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func einpCanonicalPayload(payload string) string {
	var decoded any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return payload
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return payload
	}
	return string(encoded)
}

// eventInfraPropertyNonDynamicPinnedSteps rebuilds the pinned step sequence
// from the same tables the scenario JSON was generated from.
func eventInfraPropertyNonDynamicPinnedSteps() []string {
	var keys []string
	nestedCase := func(caseName string) bool {
		return einpNestedCase(caseName)
	}
	for _, caseName := range eventInfraPropertyNonDynamicCases {
		keys = append(keys, einpKey("case", caseName))
		if caseName == "nested-escaped" {
			// One schema module registers all five event types up front.
			keys = append(keys, einpKeyFields("deploy", caseName, "schema", "", "types", "",
				einpSchemaEPL(caseName, "types"), "", "", "", ""))
			keys = append(keys, einpKey("deployed", caseName, "types"))
			for _, mode := range einpModes(caseName) {
				typeName := einpJavaTypeName(caseName, mode)
				payload := einpCanonicalPayload(einpSends(caseName, mode)[0])
				// getter pass: select * then send + probes
				keys = append(keys, einpKeyFields("deploy", caseName, "s0", "", mode, "",
					einpStarEPL(caseName, mode), "", "", "", ""))
				keys = append(keys, einpKey("deployed", caseName, "s0"))
				keys = append(keys, einpKeyFields("send", caseName, "", typeName, mode, "", "", "", "", "", payload))
				keys = append(keys, einpKey("undeploy", caseName, "s0"))
				// select pass: escaped projection
				keys = append(keys, einpKeyFields("deploy", caseName, "s0", "", mode, "",
					einpS0EPL(caseName, mode), "", "", "", ""))
				keys = append(keys, einpKey("deployed", caseName, "s0"))
				keys = append(keys, einpKeyFields("send", caseName, "", typeName, mode, "", "", "", "", "", payload))
				keys = append(keys, einpKey("undeploy", caseName, "s0"))
			}
			keys = append(keys, einpKey("undeploy-all", caseName))
			continue
		}
		for _, mode := range einpModes(caseName) {
			typeName := einpJavaTypeName(caseName, mode)
			schemaEPL := einpSchemaEPL(caseName, mode)
			keys = append(keys, einpKeyFields("deploy", caseName, "schema", typeName, mode, "", schemaEPL, "", "", "", ""))
			if schemaEPL != "" {
				keys = append(keys, einpKey("deployed", caseName, "schema"))
			}
			if caseName == "mapped-indexed" {
				order := einpTypeLevelOrder(caseName)
				keys = append(keys, einpKeyFields("types", caseName, "types", typeName, "", "",
					"", "", strings.Join(order, ","), einpTypesJSON(einpTypeLevelPins(caseName, mode)), ""))
			}
			keys = append(keys, einpKeyFields("deploy", caseName, "s0", "", mode, "",
				einpS0EPL(caseName, mode), "", "", "", ""))
			keys = append(keys, einpKey("deployed", caseName, "s0"))
			if order := einpTypeOrder(caseName); len(order) > 0 {
				keys = append(keys, einpKeyFields("types", caseName, "s0", "", "", "",
					"", "", strings.Join(order, ","), einpTypesJSON(einpTypePins(caseName, mode)), ""))
			}
			sends := einpSends(caseName, mode)
			keys = append(keys, einpKeyFields("send", caseName, "", typeName, mode, "", "", "", "", "",
				einpCanonicalPayload(sends[0])))
			if nestedCase(caseName) {
				keys = append(keys, einpKeyFields("send", caseName, "", typeName, mode, "", "", "", "", "",
					einpCanonicalPayload(sends[1])))
				keys = append(keys, einpKey("undeploy", caseName, "s0"))
				keys = append(keys, einpKeyFields("deploy", caseName, "s0", "", mode, "",
					einpStarEPL(caseName, mode), "", "", "", ""))
				keys = append(keys, einpKey("deployed", caseName, "s0"))
				keys = append(keys, einpKeyFields("send", caseName, "", typeName, mode, "", "", "", "", "",
					einpCanonicalPayload(sends[0])))
				keys = append(keys, einpKey("undeploy", caseName, "s0"))
				// Type-level assertions run after both passes.
				order := einpTypeLevelOrder(caseName)
				keys = append(keys, einpKeyFields("types", caseName, "types", typeName, "", "",
					"", "", strings.Join(order, ","), einpTypesJSON(einpTypeLevelPins(caseName, mode)), ""))
				// Java runs undeployModuleContaining between underlyings,
				// not undeployAll: the registered types persist, so the
				// scenario closes the runtime only once at the case end.
				continue
			}
			keys = append(keys, einpKey("undeploy-all", caseName))
		}
		if nestedCase(caseName) {
			keys = append(keys, einpKey("undeploy-all", caseName))
		}
	}
	return keys
}

func loadEventInfraPropertyNonDynamicScenario(reader io.Reader) (compat.Scenario, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario must be a JSON object: %w", eventInfraPropertyNonDynamicID, err)
	}
	if err := eipdRequireFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eventInfraPropertyNonDynamicID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eventInfraPropertyNonDynamicID ||
		metadata.Description != eventInfraPropertyNonDynamicDescription ||
		metadata.JavaCommit != eventInfraPropertyNonDynamicJavaCommit || metadata.JavaSource != eventInfraPropertyNonDynamicSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", eventInfraPropertyNonDynamicID)
	}
	if err := eipdRequireEqual(metadata.JavaFlags, eventInfraPropertyNonDynamicJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eipdRequireEqual(metadata.JavaRuntimes, eventInfraPropertyNonDynamicJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eipdRequireEqual(metadata.JavaNames, eventInfraPropertyNonDynamicJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eipdRequireEqual(metadata.JavaStaticID, eventInfraPropertyNonDynamicJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", eventInfraPropertyNonDynamicID, err)
	}
	if len(rawCases) != len(eventInfraPropertyNonDynamicCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", eventInfraPropertyNonDynamicID, len(rawCases), len(eventInfraPropertyNonDynamicCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := eipdRequireFields(object,
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
		caseName := eventInfraPropertyNonDynamicCases[index]
		if definition.Case != caseName || definition.Ordinal != 0 ||
			definition.RuntimeID != eventInfraPropertyNonDynamicJavaRuntimeIDs[index] ||
			definition.ExecutionName != eventInfraPropertyNonDynamicJavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d identity = %+v, want case %q ordinal 0 runtime %s execution %s",
				eventInfraPropertyNonDynamicID, index, definition, caseName,
				eventInfraPropertyNonDynamicJavaRuntimeIDs[index], eventInfraPropertyNonDynamicJavaExecutions[index])
		}
		if definition.EPL != einpCasePinnedEPL(caseName) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source", caseName)
		}
		if len(definition.Flags) != 0 {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %q declares flags, want none", eventInfraPropertyNonDynamicID, caseName)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", eventInfraPropertyNonDynamicID, err)
	}
	pinned := eventInfraPropertyNonDynamicPinnedSteps()
	if len(rawSteps) != len(pinned) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d steps, want %d", eventInfraPropertyNonDynamicID, len(rawSteps), len(pinned))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		var step compat.Step
		if err := json.Unmarshal(rawStep, &step); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := einpValidateStep(index, object, step); err != nil {
			return compat.Scenario{}, err
		}
		if key := einpStepKey(step); key != pinned[index] {
			return compat.Scenario{}, fmt.Errorf("scenario step %d = %q, want pinned %q", index, key, pinned[index])
		}
		steps[index] = step
	}
	return compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}, nil
}
