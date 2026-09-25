package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	eventInfraPropertyDynamicID         = "event-infra-property-dynamic"
	eventInfraPropertyDynamicJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	// The cluster spans six sibling files in suite/event/infra; the scenario
	// pins the shared directory while the evidence metadata carries the files.
	eventInfraPropertyDynamicSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra"
)

var eventInfraPropertyDynamicJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyDynamicSimple.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyDynamicNested.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyDynamicNestedDeep.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyDynamicNestedRootedNonSimple.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyDynamicNestedRootedSimple.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraPropertyDynamicNonSimple.java",
}

var eventInfraPropertyDynamicJavaRuntimeIDs = []string{
	"java-runtime-9f5b65c0125c70ea8760",
	"java-runtime-f686347226e7681f0325",
	"java-runtime-0285612baa5f5321fae2",
	"java-runtime-a1c916427bb1c85757f8",
	"java-runtime-4cd646a570fcd69e826a",
	"java-runtime-d6c08cf3d40f13126684",
}

var eventInfraPropertyDynamicJavaStaticIDs = []string{
	"java-1582f18f3ce77af9f4ff",
	"java-bba8d0912642e3573aeb",
	"java-90c1583c627d7e5f6219",
	"java-b13358549c04fc9cedfb",
	"java-48d6c20971992821958e",
	"java-a58ae9e754fe80b6b8da",
}

var eventInfraPropertyDynamicJavaExecutions = []string{
	"EventInfraPropertyDynamicSimple",
	"EventInfraPropertyDynamicNested",
	"EventInfraPropertyDynamicNestedDeep",
	"EventInfraPropertyDynamicNestedRootedNonSimple",
	"EventInfraPropertyDynamicNestedRootedSimple",
	"EventInfraPropertyDynamicNonSimple",
}

// javaFlags is the flat flag union; per-case flags live in eipdCaseFlags and
// pin the case-level "flags" array (EventInfraPropertyDynamicNested declares
// none — the only execution in the cluster without EXCLUDEWHENINSTRUMENTED).
var eventInfraPropertyDynamicJavaFlags = []string{"EXCLUDEWHENINSTRUMENTED"}

var eipdCaseFlags = map[string][]string{
	"dynamic-simple":   {"EXCLUDEWHENINSTRUMENTED"},
	"dynamic-nested":   {},
	"nested-deep":      {"EXCLUDEWHENINSTRUMENTED"},
	"rooted-nonsimple": {"EXCLUDEWHENINSTRUMENTED"},
	"rooted-simple":    {"EXCLUDEWHENINSTRUMENTED"},
	"nonsimple":        {"EXCLUDEWHENINSTRUMENTED"},
}

var eventInfraPropertyDynamicCases = []string{
	"dynamic-simple",
	"dynamic-nested",
	"nested-deep",
	"rooted-nonsimple",
	"rooted-simple",
	"nonsimple",
}

const eventInfraPropertyDynamicDescription = "EventInfraPropertyDynamic* dynamic-VALUE property slice (all ord 0): prop? resolves on the runtime value's type and exists(prop?) reports resolution, x?.y keeps a dynamic root with a static tail and leaf-? marks the leaf dynamic. dynamic-simple replays id? over bean/map/objectarray/xml/avro/json with exists/exists(null)/notExists tri-state; dynamic-nested replays item.id? plus a select-* statement under five Java output representations (objectarray/map/avro/default/json) probing the raw item.id? getter on the select-* event; nested-deep replays six ? placements n1..n6 then a select-* consistency pass; rooted-nonsimple replays item?.indexed[i]/mapped('k')/arrayProperty/mapProperty accessor mixes; rooted-simple replays simpleProperty?/nested?.nestedValue/nested?.nestedNested.nestedNestedValue; nonsimple replays leaf-? indexed[i]?/mapped('k')? accessor pairs over declared properties on the runtime value's type. Provided-class JSON instantiates fields as null (exists-on-null vs dynamic-schema notExists); XML sends pin the Java fragment and the Go myevent-wrapped document. Java sources are the six sibling files under suite/event/infra (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c)."

// ---- Case metadata tables -------------------------------------------------

// Java type names each underlying registers (config-side in the suite runner).
func eipdJavaTypeName(caseName, mode string) string {
	pkg := ""
	switch caseName {
	case "dynamic-simple":
		switch mode {
		case "bean":
			return "SupportMarkerInterface"
		case "map":
			pkg = "Map"
		case "objectarray":
			pkg = "OA"
		case "xml":
			pkg = "XML"
		case "avro":
			pkg = "Avro"
		case "json":
			pkg = "Json"
		}
		return "EventInfraPropertyDynamicSimple" + pkg
	case "dynamic-nested":
		switch mode {
		case "bean":
			return "SupportBeanDynRoot"
		case "map":
			pkg = "Map"
		case "objectarray":
			pkg = "OA"
		case "xml":
			pkg = "XML"
		case "avro":
			pkg = "Avro"
		case "json":
			pkg = "Json"
		case "json-provided":
			pkg = "JsonProvided"
		}
		return "EventInfraPropertyDynamicNested" + pkg
	case "nested-deep":
		switch mode {
		case "bean":
			return "SupportBeanDynRoot"
		case "map":
			pkg = "Map"
		case "objectarray":
			pkg = "OA"
		case "xml":
			pkg = "XML"
		case "avro":
			pkg = "Avro"
		case "json":
			pkg = "Json"
		case "json-provided":
			pkg = "JsonProvided"
		}
		return "EventInfraPropertyDynamicNestedDeep" + pkg
	case "rooted-nonsimple":
		switch mode {
		case "bean":
			return "SupportBeanDynRoot"
		case "map":
			pkg = "Map"
		case "objectarray":
			pkg = "OA"
		case "xml":
			pkg = "XML"
		case "avro":
			pkg = "Avro"
		case "json":
			pkg = "Json"
		case "json-provided":
			pkg = "JsonProvided"
		}
		return "EventInfraPropertyDynamicNestedRootedNonSimple" + pkg
	case "rooted-simple":
		switch mode {
		case "bean":
			return "SupportMarkerInterface"
		case "map":
			pkg = "Map"
		case "objectarray":
			pkg = "OA"
		case "xml":
			pkg = "XML"
		case "avro":
			pkg = "Avro"
		case "json":
			pkg = "Json"
		case "json-provided":
			pkg = "JsonProvided"
		}
		return "EventInfraPropertyDynamicNestedRootedSimple" + pkg
	case "nonsimple":
		switch mode {
		case "bean":
			return "SupportBeanComplexProps"
		case "map":
			pkg = "Map"
		case "objectarray":
			pkg = "OA"
		case "xml":
			pkg = "XML"
		case "avro":
			pkg = "Avro"
		case "json":
			pkg = "Json"
		case "json-provided":
			pkg = "JsonProvided"
		}
		return "EventInfraPropertyDynamicNonSimple" + pkg
	}
	return ""
}

// eipdModes lists the underlying modes replayed per case.
func eipdModes(caseName string) []string {
	if caseName == "dynamic-simple" {
		return []string{"bean", "map", "objectarray", "xml", "avro", "json"}
	}
	return []string{"bean", "map", "objectarray", "xml", "avro", "json", "json-provided"}
}

// eipdReps lists the output representations per iteration. Only dynamic-nested
// loops EventRepresentationChoice; the other cases have no rep dimension.
func eipdReps(caseName string) []string {
	if caseName == "dynamic-nested" {
		return []string{"objectarray", "map", "avro", "default", "json"}
	}
	return []string{""}
}

// nested XML is skipped when the output rep is avro or json (the Java source
// guards runAssertion on !outputEventRep.isAvroOrJsonEvent()).
func eipdIterationExists(caseName, rep, mode string) bool {
	if caseName == "dynamic-nested" && mode == "xml" && (rep == "avro" || rep == "json") {
		return false
	}
	return true
}

// ---- Pinned EPL -----------------------------------------------------------

// eipdS0EPL returns the byte-exact Java statement text for one iteration
// (typename already resolved).
func eipdS0EPL(caseName, mode, rep string) string {
	t := eipdJavaTypeName(caseName, mode)
	switch caseName {
	case "dynamic-simple":
		return "@name('s0') select id? as myid, exists(id?) as exists_myid from " + t
	case "dynamic-nested":
		annotation := ""
		switch rep {
		case "objectarray":
			annotation = "@EventRepresentation('objectarray')"
		case "map":
			annotation = "@EventRepresentation('map')"
		case "avro":
			annotation = "@EventRepresentation('avro')@AvroSchemaField(name='myid',schema='[\"int\",{\"type\":\"string\",\"avro.java.string\":\"String\"},\"null\"]')"
		case "json":
			annotation = "@EventRepresentation('json')"
		}
		return "@name('s0') " + annotation + " select " +
			"item.id? as myid, " +
			"exists(item.id?) as exists_myid " +
			"from " + t + ";\n" +
			"@name('s1') select * from " + t + ";\n"
	case "nested-deep":
		return "@name('s0') select " +
			" item.nested?.nestedValue as n1, " +
			" exists(item.nested?.nestedValue) as exists_n1, " +
			" item.nested?.nestedValue? as n2, " +
			" exists(item.nested?.nestedValue?) as exists_n2, " +
			" item.nested?.nestedNested.nestedNestedValue as n3, " +
			" exists(item.nested?.nestedNested.nestedNestedValue) as exists_n3, " +
			" item.nested?.nestedNested?.nestedNestedValue as n4, " +
			" exists(item.nested?.nestedNested?.nestedNestedValue) as exists_n4, " +
			" item.nested?.nestedNested.nestedNestedValue? as n5, " +
			" exists(item.nested?.nestedNested.nestedNestedValue?) as exists_n5, " +
			" item.nested?.nestedNested?.nestedNestedValue? as n6, " +
			" exists(item.nested?.nestedNested?.nestedNestedValue?) as exists_n6 " +
			" from " + t
	case "rooted-nonsimple":
		return "@name('s0') select " +
			"item?.indexed[0] as indexed1, " +
			"exists(item?.indexed[0]) as exists_indexed1, " +
			"item?.indexed[1]? as indexed2, " +
			"exists(item?.indexed[1]?) as exists_indexed2, " +
			"item?.arrayProperty[1]? as array, " +
			"exists(item?.arrayProperty[1]?) as exists_array, " +
			"item?.mapped('keyOne') as mapped1, " +
			"exists(item?.mapped('keyOne')) as exists_mapped1, " +
			"item?.mapped('keyTwo')? as mapped2,  " +
			"exists(item?.mapped('keyTwo')?) as exists_mapped2,  " +
			"item?.mapProperty('xOne')? as map, " +
			"exists(item?.mapProperty('xOne')?) as exists_map " +
			" from " + t
	case "rooted-simple":
		return "@name('s0') select " +
			"simpleProperty? as simple, " +
			"exists(simpleProperty?) as exists_simple, " +
			"nested?.nestedValue as nested, " +
			"exists(nested?.nestedValue) as exists_nested, " +
			"nested?.nestedNested.nestedNestedValue as nestedNested, " +
			"exists(nested?.nestedNested.nestedNestedValue) as exists_nestedNested " +
			"from " + t
	case "nonsimple":
		return "@name('s0') select " +
			"indexed[0]? as indexed1, " +
			"exists(indexed[0]?) as exists_indexed1, " +
			"indexed[1]? as indexed2, " +
			"exists(indexed[1]?) as exists_indexed2, " +
			"mapped('keyOne')? as mapped1, " +
			"exists(mapped('keyOne')?) as exists_mapped1, " +
			"mapped('keyTwo')? as mapped2,  " +
			"exists(mapped('keyTwo')?) as exists_mapped2  " +
			"from " + t
	}
	return ""
}

// eipdSchemaEPL returns the Java schema-creation EPL for json/json-provided
// modes (compileDeploy'd in the Java source). Other modes configure the type
// in the runner Configuration, so the pinned string is empty and no schema
// module deploys.
func eipdSchemaEPL(caseName, mode string) string {
	nestedJSONProvidedItem := "com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyDynamicNested$MyLocalJsonProvidedItem"
	nestedJSONProvided := "com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyDynamicNested$MyLocalJsonProvided"
	deepJSONProvided := "com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyDynamicNestedDeep$MyLocalJsonProvided"
	nonSimpleJSONProvided := "com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyDynamicNestedRootedNonSimple$MyLocalJsonProvided"
	leafNonSimpleJSONProvided := "com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyDynamicNonSimple$MyLocalJsonProvided"
	rootedSimpleJSONProvided := "com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyDynamicNestedRootedSimple$MyLocalJsonProvided"
	switch caseName {
	case "dynamic-simple":
		if mode == "json" {
			return "@JsonSchema(dynamic=true) @public @buseventtype create json schema EventInfraPropertyDynamicSimpleJson()"
		}
	case "dynamic-nested":
		if mode == "json" {
			return "@JsonSchema(dynamic=true) create json schema Undefined();\n" +
				"@public @buseventtype @name('schema') create json schema EventInfraPropertyDynamicNestedJson(item Undefined)"
		}
		if mode == "json-provided" {
			return "@JsonSchema(className='" + nestedJSONProvidedItem + "') @public @buseventtype @name('schema') create json schema Item();\n" +
				"@JsonSchema(className='" + nestedJSONProvided + "') @public @buseventtype @name('schema') create json schema EventInfraPropertyDynamicNestedJsonProvided(item Item)"
		}
	case "nested-deep":
		if mode == "json" {
			return "@JsonSchema(dynamic=true) create json schema Item();\n" +
				"@public @buseventtype @name('schema') create json schema EventInfraPropertyDynamicNestedDeepJson(item Item)"
		}
		if mode == "json-provided" {
			return "@JsonSchema(className='" + deepJSONProvided + "') @public @buseventtype @name('schema') create json schema EventInfraPropertyDynamicNestedDeepJsonProvided()"
		}
	case "rooted-nonsimple":
		if mode == "json" {
			return "@public @buseventtype @name('schema') @JsonSchema(dynamic=true) create json schema EventInfraPropertyDynamicNestedRootedNonSimpleJson()"
		}
		if mode == "json-provided" {
			return "@JsonSchema(className='" + nonSimpleJSONProvided + "') @public @buseventtype @name('schema') @JsonSchema(dynamic=true) create json schema EventInfraPropertyDynamicNestedRootedNonSimpleJsonProvided()"
		}
	case "rooted-simple":
		if mode == "json" {
			return "@JsonSchema(dynamic=true) @public @buseventtype @name('schema') create json schema EventInfraPropertyDynamicNestedRootedSimpleJson()"
		}
		if mode == "json-provided" {
			return "@JsonSchema(className='" + rootedSimpleJSONProvided + "') @public @buseventtype @name('schema') create json schema EventInfraPropertyDynamicNestedRootedSimpleJsonProvided()"
		}
	case "nonsimple":
		if mode == "json" {
			return "@JsonSchema(dynamic=true) @public @buseventtype create json schema EventInfraPropertyDynamicNonSimpleJson()"
		}
		if mode == "json-provided" {
			return "@JsonSchema(className='" + leafNonSimpleJSONProvided + "') @public @buseventtype create json schema EventInfraPropertyDynamicNonSimpleJsonProvided()"
		}
	}
	return ""
}

// ---- Projection columns ----------------------------------------------------

// eipdColumn is one output column: the Java expression text, its alias, the Go
// property path used for the value column and the (possibly different) Go path
// used for exists. javaExpr pins the source expression text.
type eipdColumn struct {
	name     string // output alias (n1, exists_n1, myid, ...)
	javaExpr string // pinned Java expression text
	goValue  string // Go property path for the value projection
	goExists string // Go property path for exists(); empty = reuse goValue
	exists   bool   // column is exists(expr)
}

func eipdColumns(caseName, mode string) []eipdColumn {
	xml := mode == "xml"
	switch caseName {
	case "dynamic-simple":
		val, ex := "id?", "id"
		if xml {
			val, ex = "myevent.id", "myevent.id"
		}
		return []eipdColumn{
			{name: "myid", javaExpr: "id?", goValue: val},
			{name: "exists_myid", javaExpr: "exists(id?)", goValue: val, goExists: ex, exists: true},
		}
	case "dynamic-nested":
		val := "item?.id"
		if xml {
			val = "myevent.item?.@id"
		}
		return []eipdColumn{
			{name: "myid", javaExpr: "item.id?", goValue: val},
			{name: "exists_myid", javaExpr: "exists(item.id?)", goValue: val, exists: true},
		}
	case "nested-deep":
		if xml {
			return []eipdColumn{
				{name: "n1", javaExpr: "item.nested?.nestedValue", goValue: "myevent.item.nested?.@nestedValue"},
				{name: "exists_n1", javaExpr: "exists(item.nested?.nestedValue)", goValue: "myevent.item.nested?.@nestedValue", exists: true},
				{name: "n2", javaExpr: "item.nested?.nestedValue?", goValue: "myevent.item.nested?.@nestedValue?"},
				{name: "exists_n2", javaExpr: "exists(item.nested?.nestedValue?)", goValue: "myevent.item.nested?.@nestedValue", exists: true},
				{name: "n3", javaExpr: "item.nested?.nestedNested.nestedNestedValue", goValue: "myevent.item.nested?.nestedNested?.@nestedNestedValue"},
				{name: "exists_n3", javaExpr: "exists(item.nested?.nestedNested.nestedNestedValue)", goValue: "myevent.item.nested?.nestedNested?.@nestedNestedValue", exists: true},
				{name: "n4", javaExpr: "item.nested?.nestedNested?.nestedNestedValue", goValue: "myevent.item.nested?.nestedNested?.@nestedNestedValue"},
				{name: "exists_n4", javaExpr: "exists(item.nested?.nestedNested?.nestedNestedValue)", goValue: "myevent.item.nested?.nestedNested?.@nestedNestedValue", exists: true},
				{name: "n5", javaExpr: "item.nested?.nestedNested.nestedNestedValue?", goValue: "myevent.item.nested?.nestedNested?.@nestedNestedValue?"},
				{name: "exists_n5", javaExpr: "exists(item.nested?.nestedNested.nestedNestedValue?)", goValue: "myevent.item.nested?.nestedNested?.@nestedNestedValue", exists: true},
				{name: "n6", javaExpr: "item.nested?.nestedNested?.nestedNestedValue?", goValue: "myevent.item.nested?.nestedNested?.@nestedNestedValue?"},
				{name: "exists_n6", javaExpr: "exists(item.nested?.nestedNested?.nestedNestedValue?)", goValue: "myevent.item.nested?.nestedNested?.@nestedNestedValue", exists: true},
			}
		}
		return []eipdColumn{
			{name: "n1", javaExpr: "item.nested?.nestedValue", goValue: "item.nested?.nestedValue"},
			{name: "exists_n1", javaExpr: "exists(item.nested?.nestedValue)", goValue: "item.nested?.nestedValue", exists: true},
			{name: "n2", javaExpr: "item.nested?.nestedValue?", goValue: "item.nested?.nestedValue?"},
			{name: "exists_n2", javaExpr: "exists(item.nested?.nestedValue?)", goValue: "item.nested?.nestedValue", exists: true},
			{name: "n3", javaExpr: "item.nested?.nestedNested.nestedNestedValue", goValue: "item.nested?.nestedNested?.nestedNestedValue"},
			{name: "exists_n3", javaExpr: "exists(item.nested?.nestedNested.nestedNestedValue)", goValue: "item.nested?.nestedNested?.nestedNestedValue", exists: true},
			{name: "n4", javaExpr: "item.nested?.nestedNested?.nestedNestedValue", goValue: "item.nested?.nestedNested?.nestedNestedValue"},
			{name: "exists_n4", javaExpr: "exists(item.nested?.nestedNested?.nestedNestedValue)", goValue: "item.nested?.nestedNested?.nestedNestedValue", exists: true},
			{name: "n5", javaExpr: "item.nested?.nestedNested.nestedNestedValue?", goValue: "item.nested?.nestedNested?.nestedNestedValue?"},
			{name: "exists_n5", javaExpr: "exists(item.nested?.nestedNested.nestedNestedValue?)", goValue: "item.nested?.nestedNested?.nestedNestedValue", exists: true},
			{name: "n6", javaExpr: "item.nested?.nestedNested?.nestedNestedValue?", goValue: "item.nested?.nestedNested?.nestedNestedValue?"},
			{name: "exists_n6", javaExpr: "exists(item.nested?.nestedNested?.nestedNestedValue?)", goValue: "item.nested?.nestedNested?.nestedNestedValue", exists: true},
		}
	case "rooted-nonsimple":
		if xml {
			return []eipdColumn{
				{name: "indexed1", javaExpr: "item?.indexed[0]", goValue: "myevent.item?.indexed[0]"},
				// Caveat: XML mapped('keyOne') is emulated positionally as myevent.mapped[0] and
				// exists(item?.indexed[0]) as Exists(item?.indexed); the pinned sends make positional
				// and keyed access coincide and never declare an empty indexed array.
				{name: "exists_indexed1", javaExpr: "exists(item?.indexed[0])", goValue: "myevent.item?.indexed[0]", goExists: "myevent.item?.indexed", exists: true},
				{name: "indexed2", javaExpr: "item?.indexed[1]?", goValue: "myevent.item?.indexed[1]?"},
				{name: "exists_indexed2", javaExpr: "exists(item?.indexed[1]?)", goValue: "myevent.item?.indexed[1]?", exists: true},
				{name: "array", javaExpr: "item?.arrayProperty[1]?", goValue: "myevent.item?.arrayProperty[1]?"},
				{name: "exists_array", javaExpr: "exists(item?.arrayProperty[1]?)", goValue: "myevent.item?.arrayProperty[1]?", exists: true},
				{name: "mapped1", javaExpr: "item?.mapped('keyOne')", goValue: "myevent.item?.mapped[0]"},
				{name: "exists_mapped1", javaExpr: "exists(item?.mapped('keyOne'))", goValue: "myevent.item?.mapped[0]", goExists: "myevent.item?.mapped", exists: true},
				{name: "mapped2", javaExpr: "item?.mapped('keyTwo')?", goValue: "myevent.item?.mapped[1]?"},
				{name: "exists_mapped2", javaExpr: "exists(item?.mapped('keyTwo')?)", goValue: "myevent.item?.mapped[1]?", exists: true},
				{name: "map", javaExpr: "item?.mapProperty('xOne')?", goValue: "myevent.item?.mapProperty('xOne')?"},
				{name: "exists_map", javaExpr: "exists(item?.mapProperty('xOne')?)", goValue: "myevent.item?.mapProperty('xOne')?", exists: true},
			}

		}
		return []eipdColumn{
			{name: "indexed1", javaExpr: "item?.indexed[0]", goValue: "item?.indexed[0]"},
			{name: "exists_indexed1", javaExpr: "exists(item?.indexed[0])", goValue: "item?.indexed[0]", goExists: "item?.indexed", exists: true},
			{name: "indexed2", javaExpr: "item?.indexed[1]?", goValue: "item?.indexed[1]?"},
			{name: "exists_indexed2", javaExpr: "exists(item?.indexed[1]?)", goValue: "item?.indexed[1]?", exists: true},
			{name: "array", javaExpr: "item?.arrayProperty[1]?", goValue: "item?.arrayProperty[1]?"},
			{name: "exists_array", javaExpr: "exists(item?.arrayProperty[1]?)", goValue: "item?.arrayProperty[1]?", exists: true},
			{name: "mapped1", javaExpr: "item?.mapped('keyOne')", goValue: "item?.mapped('keyOne')"},
			{name: "exists_mapped1", javaExpr: "exists(item?.mapped('keyOne'))", goValue: "item?.mapped('keyOne')", goExists: "item?.mapped", exists: true},
			{name: "mapped2", javaExpr: "item?.mapped('keyTwo')?", goValue: "item?.mapped('keyTwo')?"},
			{name: "exists_mapped2", javaExpr: "exists(item?.mapped('keyTwo')?)", goValue: "item?.mapped('keyTwo')?", exists: true},
			{name: "map", javaExpr: "item?.mapProperty('xOne')?", goValue: "item?.mapProperty('xOne')?"},
			{name: "exists_map", javaExpr: "exists(item?.mapProperty('xOne')?)", goValue: "item?.mapProperty('xOne')?", exists: true},
		}
	case "rooted-simple":
		if xml {
			return []eipdColumn{
				{name: "simple", javaExpr: "simpleProperty?", goValue: "myevent.simpleProperty"},
				{name: "exists_simple", javaExpr: "exists(simpleProperty?)", goValue: "myevent.simpleProperty", exists: true},
				{name: "nested", javaExpr: "nested?.nestedValue", goValue: "myevent.nested?.@nestedValue"},
				{name: "exists_nested", javaExpr: "exists(nested?.nestedValue)", goValue: "myevent.nested?.@nestedValue", exists: true},
				{name: "nestedNested", javaExpr: "nested?.nestedNested.nestedNestedValue", goValue: "myevent.nested?.nestedNested?.@nestedNestedValue"},
				{name: "exists_nestedNested", javaExpr: "exists(nested?.nestedNested.nestedNestedValue)", goValue: "myevent.nested?.nestedNested?.@nestedNestedValue", exists: true},
			}
		}
		return []eipdColumn{
			{name: "simple", javaExpr: "simpleProperty?", goValue: "simpleProperty?"},
			{name: "exists_simple", javaExpr: "exists(simpleProperty?)", goValue: "simpleProperty", exists: true},
			{name: "nested", javaExpr: "nested?.nestedValue", goValue: "nested?.nestedValue"},
			{name: "exists_nested", javaExpr: "exists(nested?.nestedValue)", goValue: "nested?.nestedValue", exists: true},
			{name: "nestedNested", javaExpr: "nested?.nestedNested.nestedNestedValue", goValue: "nested?.nestedNested?.nestedNestedValue"},
			{name: "exists_nestedNested", javaExpr: "exists(nested?.nestedNested.nestedNestedValue)", goValue: "nested?.nestedNested?.nestedNestedValue", exists: true},
		}
	case "nonsimple":
		if xml {
			return []eipdColumn{
				{name: "indexed1", javaExpr: "indexed[0]?", goValue: "myevent.indexed[0]?"},
				{name: "exists_indexed1", javaExpr: "exists(indexed[0]?)", goValue: "myevent.indexed[0]?", exists: true},
				{name: "indexed2", javaExpr: "indexed[1]?", goValue: "myevent.indexed[1]?"},
				{name: "exists_indexed2", javaExpr: "exists(indexed[1]?)", goValue: "myevent.indexed[1]?", exists: true},
				{name: "mapped1", javaExpr: "mapped('keyOne')?", goValue: "myevent.mapped[0]?"},
				{name: "exists_mapped1", javaExpr: "exists(mapped('keyOne')?)", goValue: "myevent.mapped[0]?", exists: true},
				{name: "mapped2", javaExpr: "mapped('keyTwo')?", goValue: "myevent.mapped[1]?"},
				{name: "exists_mapped2", javaExpr: "exists(mapped('keyTwo')?)", goValue: "myevent.mapped[1]?", exists: true},
			}
		}
		return []eipdColumn{
			{name: "indexed1", javaExpr: "indexed[0]?", goValue: "indexed[0]?"},
			{name: "exists_indexed1", javaExpr: "exists(indexed[0]?)", goValue: "indexed[0]?", exists: true},
			{name: "indexed2", javaExpr: "indexed[1]?", goValue: "indexed[1]?"},
			{name: "exists_indexed2", javaExpr: "exists(indexed[1]?)", goValue: "indexed[1]?", exists: true},
			{name: "mapped1", javaExpr: "mapped('keyOne')?", goValue: "mapped('keyOne')?"},
			{name: "exists_mapped1", javaExpr: "exists(mapped('keyOne')?)", goValue: "mapped('keyOne')?", exists: true},
			{name: "mapped2", javaExpr: "mapped('keyTwo')?", goValue: "mapped('keyTwo')?"},
			{name: "exists_mapped2", javaExpr: "exists(mapped('keyTwo')?)", goValue: "mapped('keyTwo')?", exists: true},
		}
	}
	return nil
}

// eipdValuePropNames are the value-bearing output names used by the Java
// property-type assertion and the per-send flag array.
func eipdValuePropNames(caseName string) []string {
	switch caseName {
	case "dynamic-simple", "dynamic-nested":
		return []string{"myid"}
	case "nested-deep":
		return []string{"n1", "n2", "n3", "n4", "n5", "n6"}
	case "rooted-nonsimple":
		return []string{"indexed1", "indexed2", "array", "mapped1", "mapped2", "map"}
	case "rooted-simple":
		return []string{"simple", "nested", "nestedNested"}
	case "nonsimple":
		return []string{"indexed1", "indexed2", "mapped1", "mapped2"}
	}
	return nil
}

// eipdTypePins are the pinned canonical output types per (mode family, name):
// value columns are Object (Node for XML), exists columns Boolean.
func eipdTypePins(caseName, mode string) map[string]string {
	valueType := "Object"
	if mode == "xml" {
		valueType = "Node"
	}
	pins := map[string]string{}
	for _, name := range eipdValuePropNames(caseName) {
		pins[name] = valueType
		pins["exists_"+name] = "Boolean"
	}
	return pins
}

func eipdSelectAllEPL(caseName, mode string) string {
	if caseName != "nested-deep" {
		return ""
	}
	return "@name('s0') select * from " + eipdJavaTypeName(caseName, mode)
}
func eipdSends(caseName, mode string) []string {
	switch caseName {
	case "dynamic-simple":
		switch mode {
		case "bean":
			return []string{`{"variant":"impl-a"}`, `{"variant":"impl-b"}`, `{"variant":"impl-c"}`}
		case "map":
			return []string{`{"somekey":"10"}`, `{"id":"abc"}`, `{"id":10}`}
		case "objectarray":
			return []string{`{"values":[1,null]}`, `{"values":[2,"abc"]}`, `{"values":[3,10]}`}
		case "xml":
			return []string{
				`{"xml":"","goXml":"<myevent/>"}`,
				`{"xml":"<id>10</id>","goXml":"<myevent><id>10</id></myevent>"}`,
				`{"xml":"<id>abc</id>","goXml":"<myevent><id>abc</id></myevent>"}`,
			}
		case "avro":
			return []string{`{"id":null}`, `{"id":101}`, `{"id":null}`}
		case "json":
			return []string{`{}`, `{"id":10}`, `{"id":"abc"}`}
		}
	case "dynamic-nested":
		switch mode {
		case "bean":
			return []string{
				`{"variant":"s0-101"}`, `{"variant":"str-abc"}`, `{"variant":"a-e1"}`,
				`{"variant":"b-e2"}`, `{"variant":"s1-102"}`,
			}
		case "map":
			return []string{`{}`, `{"item":{"id":101}}`, `{"item":{}}`}
		case "objectarray":
			return []string{`{"values":[null]}`, `{"variant":"oa-s0-101"}`, `{"values":["abc"]}`}
		case "xml":
			return []string{
				`{"xml":"<item id=\"101\"/>","goXml":"<myevent><item id=\"101\"/></myevent>"}`,
				`{"xml":"<item/>","goXml":"<myevent><item/></myevent>"}`,
			}
		case "avro":
			return []string{`{"item":{"id":null}}`, `{"item":{"id":101}}`, `{"item":{"id":"abc"}}`}
		case "json", "json-provided":
			return []string{`{}`, `{"item":{"id":101}}`, `{"item":{"id":"abc"}}`}
		}
	case "nested-deep":
		switch mode {
		case "bean":
			return []string{`{"variant":"complex-default"}`, `{"variant":"complex-mutated"}`, `{"variant":"str-abc"}`}
		case "map":
			return []string{
				`{"item":{"nested":{"nestedValue":100,"nestedNested":{"nestedNestedValue":101}}}}`,
				`{}`,
			}
		case "objectarray":
			return []string{`{"values":[[[[101],100]]]}`, `{"values":[null]}`}
		case "xml":
			return []string{
				`{"xml":"<item>\n\t<nested nestedValue=\"100\">\n\t\t<nestedNested nestedNestedValue=\"101\">\n\t\t</nestedNested>\n\t</nested>\n</item>\n","goXml":"<myevent><item><nested nestedValue=\"100\"><nestedNested nestedNestedValue=\"101\"/></nested></item></myevent>"}`,
				`{"xml":"<item/>","goXml":"<myevent><item/></myevent>"}`,
			}
		case "avro":
			// send1 wraps nestedDatum inside item; send2 is an empty outer record.
			return []string{
				`{"item":{"nested":{"nestedValue":100,"nestedNested":{"nestedNestedValue":101}}}}`,
				`{"item":null}`,
			}
		case "json":
			return []string{
				`{"item":{"nested":{"nestedValue":100,"nestedNested":{"nestedNestedValue":101}}}}`,
				`{"item":{"nested":{}}}`,
				`{"item":{}}`,
				`{}`,
			}
		case "json-provided":
			return []string{
				`{"item":{"nested":{"nestedValue":100,"nestedNested":{"nestedNestedValue":101}}}}`,
				`{"item":{"nested":{}}}`,
				`{"item":{}}`,
				`{}`,
			}
		}
	case "rooted-nonsimple":
		switch mode {
		case "bean":
			return []string{`{"variant":"str-xxx"}`, `{"variant":"complex-default"}`}
		case "map":
			return []string{
				`{}`,
				`{"item":{"indexed":[1,2],"arrayProperty":null,"mapped":{"keyOne":100,"keyTwo":200},"mapProperty":null}}`,
			}
		case "objectarray":
			return []string{
				`{"values":[null,null]}`,
				`{"values":[null,{"values":[[1,2],{"keyOne":100,"keyTwo":200},[1000,2000],{"xOne":"abc"}]}]}`,
			}
		case "xml":
			return []string{
				`{"xml":"","goXml":"<myevent></myevent>"}`,
				`{"xml":"<item><indexed>1</indexed><indexed>2</indexed><mapped id=\"keyOne\">3</mapped><mapped id=\"keyTwo\">4</mapped></item>","goXml":"<myevent><item><indexed>1</indexed><indexed>2</indexed><mapped id=\"keyOne\">3</mapped><mapped id=\"keyTwo\">4</mapped></item></myevent>"}`,
			}
		case "avro":
			// datumNull and the empty record are both records with all fields null.
			return []string{
				`{"item":null}`,
				`{"item":null}`,
				`{"item":{"indexed":[1,2],"mapped":{"keyOne":3,"keyTwo":4}}}`,
			}
		case "json":
			return []string{
				`{}`,
				`{"item":{}}`,
				`{"item":{"indexed":[1,2],"mapped":{"keyOne":3,"keyTwo":4}}}`,
			}
		case "json-provided":
			return []string{
				`{}`,
				`{"item":{}}`,
				`{"item":{"indexed":[1,2],"mapped":{"keyOne":3,"keyTwo":4}}}`,
			}
		}
	case "nonsimple":
		switch mode {
		case "bean":
			return []string{`{"variant":"complex-default"}`}
		case "map":
			return []string{
				`{"somekey":"10"}`,
				`{"indexed":[1,2],"mapped":{"keyOne":3,"keyTwo":4}}`,
			}
		case "objectarray":
			return []string{
				`{"values":[null,null]}`,
				`{"values":[[1,2],{"keyOne":3,"keyTwo":4}]}`,
			}
		case "xml":
			return []string{
				`{"xml":"","goXml":"<myevent></myevent>"}`,
				`{"xml":"<indexed>1</indexed><indexed>2</indexed><mapped id=\"keyOne\">3</mapped><mapped id=\"keyTwo\">4</mapped>","goXml":"<myevent><indexed>1</indexed><indexed>2</indexed><mapped id=\"keyOne\">3</mapped><mapped id=\"keyTwo\">4</mapped></myevent>"}`,
			}
		case "avro":
			return []string{
				`{}`,
				`{"indexed":[1,2],"mapped":{"keyOne":3,"keyTwo":4}}`,
			}
		case "json", "json-provided":
			return []string{
				`{}`,
				`{"mapped":{"keyOne":"3","keyTwo":"4"},"indexed":["1","2"]}`,
			}
		}
	case "rooted-simple":
		switch mode {
		case "bean":
			return []string{`{"variant":"complex-default"}`, `{"variant":"impl-a-x"}`}
		case "map":
			return []string{
				`{"simpleProperty":"a"}`,
				`{"simpleProperty":5,"nested":{"nestedNested":{"nestedNestedValue":101},"nestedValue":"abc"}}`,
			}
		case "objectarray":
			return []string{
				`{"values":["a",null]}`,
				`{"values":[5,{"values":["abc",{"values":[101]}]}]}`,
			}
		case "xml":
			return []string{
				`{"xml":"<simpleProperty>abc</simpleProperty><nested nestedValue=\"100\">\n\t<nestedNested nestedNestedValue=\"101\">\n\t</nestedNested>\n</nested>\n","goXml":"<myevent><simpleProperty>abc</simpleProperty><nested nestedValue=\"100\"><nestedNested nestedNestedValue=\"101\"/></nested></myevent>"}`,
				`{"xml":"<nested/>","goXml":"<myevent><nested/></myevent>"}`,
			}
		case "avro":
			return []string{
				`{"simpleProperty":null,"nested":null}`,
				`{"simpleProperty":null,"nested":null}`,
				`{"simpleProperty":"abc","nested":{"nestedValue":100,"nestedNested":{"nestedNestedValue":101}}}`,
			}
		case "json":
			return []string{
				`{}`,
				`{"simpleProperty":1}`,
				`{"simpleProperty":"abc","nested":{"nestedValue":100,"nestedNested":{"nestedNestedValue":101}}}`,
			}
		case "json-provided":
			return []string{
				`{}`,
				`{"simpleProperty":1}`,
				`{"simpleProperty":"abc","nested":{"nestedValue":100,"nestedNested":{"nestedNestedValue":101}}}`,
			}
		}
	}
	return nil
}

// ---- Go bean mirrors ---------------------------------------------------------

// eipdMarkerImplA mirrors SupportMarkerImplA {String id}.
type eipdMarkerImplA struct {
	ID string `esper:"id" json:"id"`
}

// eipdMarkerImplB mirrors SupportMarkerImplB {int id}.
type eipdMarkerImplB struct {
	ID int64 `esper:"id" json:"id"`
}

// eipdMarkerImplC mirrors SupportMarkerImplC (no properties).
type eipdMarkerImplC struct{}

// eipdDynRoot mirrors SupportBeanDynRoot {Object item}.
type eipdDynRoot struct {
	Item any `esper:"item" json:"item"`
}

// eipdBeanS0 mirrors SupportBean_S0 (getId + p00..p03 accessors).
type eipdBeanS0 struct {
	ID  int64   `esper:"id" json:"id"`
	P00 *string `esper:"p00" json:"p00"`
	P01 *string `esper:"p01" json:"p01"`
	P02 *string `esper:"p02" json:"p02"`
	P03 *string `esper:"p03" json:"p03"`
}

// eipdBeanS1 mirrors SupportBean_S1 (getId + p10..p13).
type eipdBeanS1 struct {
	ID  int64   `esper:"id" json:"id"`
	P10 *string `esper:"p10" json:"p10"`
	P11 *string `esper:"p11" json:"p11"`
	P12 *string `esper:"p12" json:"p12"`
	P13 *string `esper:"p13" json:"p13"`
}

// eipdBeanA mirrors SupportBean_A (getId via SupportBeanAtoFBase).
type eipdBeanA struct {
	ID string `esper:"id" json:"id"`
}

// eipdBeanB mirrors SupportBean_B.
type eipdBeanB struct {
	ID string `esper:"id" json:"id"`
}

// eipdComplexProps mirrors SupportBeanComplexProps. Mapped/indexed carry
// esper tags for the dynamic accessor paths but are hidden from the JSON row
// rendering exactly like Java's parameter-taking getMapped/getIndexed are
// skipped by beanToJson.
type eipdComplexProps struct {
	SimpleProperty string             `esper:"simpleProperty" json:"simpleProperty"`
	Mapped         map[string]string  `esper:"mapped" json:"-"`
	Indexed        []int64            `esper:"indexed" json:"-"`
	MapProperty    map[string]string  `esper:"mapProperty" json:"mapProperty"`
	ArrayProperty  []int64            `esper:"arrayProperty" json:"arrayProperty"`
	Nested         *eipdComplexNested `esper:"nested" json:"nested"`
	ObjectArray    []any              `esper:"objectArray" json:"objectArray"`
}

type eipdComplexNested struct {
	NestedValue  string                   `esper:"nestedValue" json:"nestedValue"`
	NestedNested *eipdComplexNestedNested `esper:"nestedNested" json:"nestedNested"`
}

type eipdComplexNestedNested struct {
	NestedNestedValue string `esper:"nestedNestedValue" json:"nestedNestedValue"`
}

func eipdDefaultComplexBean() eipdComplexProps {
	return eipdComplexProps{
		SimpleProperty: "simple",
		Mapped:         map[string]string{"keyOne": "valueOne", "keyTwo": "valueTwo"},
		Indexed:        []int64{1, 2},
		MapProperty:    map[string]string{"xOne": "yOne", "xTwo": "yTwo"},
		ArrayProperty:  []int64{10, 20, 30},
		Nested: &eipdComplexNested{
			NestedValue:  "nestedValue",
			NestedNested: &eipdComplexNestedNested{NestedNestedValue: "nestedNestedValue"},
		},
	}
}

func eipdMutatedComplexBean() eipdComplexProps {
	bean := eipdDefaultComplexBean()
	bean.Nested.NestedValue = "nested1"
	bean.Nested.NestedNested.NestedNestedValue = "nested2"
	return bean
}

// JSON-provided mirrors: Java class-provided schemas instantiate declared
// fields as null, matching the Go typed-struct materialization.
type eipdNestedJSONProvidedItem struct {
	ID any `esper:"id" json:"id"`
}

// eipdLeafNonSimpleJSONProvided mirrors
// EventInfraPropertyDynamicNonSimple.MyLocalJsonProvided {int[] indexed,
// Map mapped}; the JSON sender coerces the pinned numeric strings via the
// declared field types.
type eipdLeafNonSimpleJSONProvided struct {
	Indexed []int64        `esper:"indexed" json:"indexed"`
	Mapped  map[string]any `esper:"mapped" json:"mapped"`
}

type eipdNestedJSONProvided struct {
	Item *eipdNestedJSONProvidedItem `esper:"item" json:"item"`
}

type eipdDeepJSONProvidedNestedNested struct {
	NestedNestedValue *int64 `esper:"nestedNestedValue" json:"nestedNestedValue"`
}

type eipdDeepJSONProvidedNested struct {
	NestedValue  *int64                            `esper:"nestedValue" json:"nestedValue"`
	NestedNested *eipdDeepJSONProvidedNestedNested `esper:"nestedNested" json:"nestedNested"`
}

type eipdDeepJSONProvidedItem struct {
	Nested *eipdDeepJSONProvidedNested `esper:"nested" json:"nested"`
}

type eipdDeepJSONProvided struct {
	Item *eipdDeepJSONProvidedItem `esper:"item" json:"item"`
}

type eipdNonSimpleJSONProvidedItem struct {
	Indexed []any          `esper:"indexed" json:"indexed"`
	Mapped  map[string]any `esper:"mapped" json:"mapped"`
}

type eipdNonSimpleJSONProvided struct {
	Item *eipdNonSimpleJSONProvidedItem `esper:"item" json:"item"`
}

type eipdRootedSimpleJSONProvidedNestedNested struct {
	NestedNestedValue any `esper:"nestedNestedValue" json:"nestedNestedValue"`
}

type eipdRootedSimpleJSONProvidedNested struct {
	NestedValue  any                                       `esper:"nestedValue" json:"nestedValue"`
	NestedNested *eipdRootedSimpleJSONProvidedNestedNested `esper:"nestedNested" json:"nestedNested"`
}

type eipdRootedSimpleJSONProvided struct {
	SimpleProperty any                                 `esper:"simpleProperty" json:"simpleProperty"`
	Nested         *eipdRootedSimpleJSONProvidedNested `esper:"nested" json:"nested"`
}

// ---- Iteration state ---------------------------------------------------------

type eipdIteration struct {
	env         *esper.Environment
	engine      *esper.Engine
	schemas     map[string]esper.Schema
	avroNested  map[string]esper.Schema
	deployments map[string]*esper.Deployment
	deployOrder []string
	sequences   map[string]uint64
	mode        string
	rep         string
	lastS1      *esper.Event
}

func runEventInfraPropertyDynamicScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eventInfraPropertyDynamicCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEventInfraPropertyDynamicCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eventInfraPropertyDynamicID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", eventInfraPropertyDynamicID, scenario.ID)
	}
	return trace, nil
}

func runEventInfraPropertyDynamicCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	var iter *eipdIteration
	defer func() {
		if iter != nil && iter.engine != nil {
			_ = iter.engine.Close(context.Background())
		}
	}()

	newIteration := func() *eipdIteration {
		if iter != nil && iter.engine != nil {
			_ = iter.engine.Close(context.Background())
		}
		iter = &eipdIteration{
			env:         esper.NewEnvironment(),
			schemas:     make(map[string]esper.Schema),
			avroNested:  make(map[string]esper.Schema),
			deployments: make(map[string]*esper.Deployment),
			sequences:   make(map[string]uint64),
		}
		iter.engine = esper.NewEngine(iter.env,
			esper.WithRuntimeURI(eventInfraPropertyDynamicJavaRuntimeIDs[eventInfraPropertyDynamicOrdinal(caseName)]),
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
				iter = newIteration()
			}
			if err := iter.deploy(ctx, &trace, caseName, step); err != nil {
				return trace, err
			}
		case "deployed":
			if iter == nil {
				return trace, fmt.Errorf("%s: deployed marker without deploy", eventInfraPropertyDynamicID)
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
				return trace, fmt.Errorf("%s: types probe without deploy", eventInfraPropertyDynamicID)
			}
			if err := iter.emitTypes(&trace, caseName, step); err != nil {
				return trace, err
			}
		case "send":
			if iter == nil {
				return trace, fmt.Errorf("%s: send without deploy", eventInfraPropertyDynamicID)
			}
			if err := iter.send(ctx, caseName, step); err != nil {
				return trace, err
			}
			if err := iter.emitGetterProbe(&trace, caseName); err != nil {
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
			return trace, fmt.Errorf("%s: unsupported step op %q", eventInfraPropertyDynamicID, step.Op)
		}
	}
	return trace, nil
}

func eventInfraPropertyDynamicOrdinal(caseName string) int {
	for index, name := range eventInfraPropertyDynamicCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// deploy handles the "schema" and "s0" deploy steps. The schema step carries
// the output rep in Name (nested case only) and registers the Go event types
// for the underlying mode; s0 deploys the projection plan — for dynamic-nested
// the pinned module EPL holds both s0 and s1 and both deploy from one plan.
func (it *eipdIteration) deploy(ctx context.Context, trace *compat.Trace, caseName string, step compat.Step) error {
	switch step.Statement {
	case "schema":
		it.mode = step.Mode
		it.rep = step.Name
		return it.registerSchemas(caseName, step.Mode)
	case "s0":
		it.mode = step.Mode
		it.rep = step.Name
		if selectAll := eipdSelectAllEPL(caseName, it.mode); selectAll != "" && step.Epl == selectAll {
			// nested-deep's second pass deploys select * on the same
			// statement name after undeployModuleContaining("s0").
			plan, err := it.env.Build(esper.FromAny(it.env, eipdGoTypeName(caseName, it.mode)).Query(esper.StatementName("s0")))
			if err != nil {
				return fmt.Errorf("%s: build s0 select-*: %w", eventInfraPropertyDynamicID, err)
			}
			return it.deployPlan(ctx, trace, caseName, step.Statement, plan)
		}
		pinned := eipdS0EPL(caseName, it.mode, it.rep)
		if step.Epl != pinned {
			return fmt.Errorf("%s: s0 EPL %q is not pinned %q", eventInfraPropertyDynamicID, step.Epl, pinned)
		}
		var plans []esper.Plan
		var labels []string
		projection, err := it.buildS0(caseName, it.mode)
		if err != nil {
			return err
		}
		plans = append(plans, projection)
		labels = append(labels, "s0")
		if caseName == "dynamic-nested" {
			selectAll, err := it.env.Build(esper.FromAny(it.env, eipdGoTypeName(caseName, it.mode)).Query(esper.StatementName("s1")))
			if err != nil {
				return fmt.Errorf("%s: build s1: %w", eventInfraPropertyDynamicID, err)
			}
			plans = append(plans, selectAll)
			labels = append(labels, "s1")
		}
		for index, plan := range plans {
			if err := it.deployPlan(ctx, trace, caseName, labels[index], plan); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("%s: unsupported deploy statement %q", eventInfraPropertyDynamicID, step.Statement)
	}
}

// eipdGoTypeName is the registered Go event-type name for one (case, mode).
// The Go runner registers the Java type name verbatim so the pinned EPL from
// name carries the same text.
func eipdGoTypeName(caseName, mode string) string {
	return eipdJavaTypeName(caseName, mode)
}

func (it *eipdIteration) deployPlan(ctx context.Context, trace *compat.Trace, caseName, label string, plan esper.Plan) error {
	deployment, err := it.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", eventInfraPropertyDynamicID, label, err)
	}
	it.deployments[label] = deployment
	it.deployOrder = append(it.deployOrder, label)
	for _, statement := range deployment.Statements() {
		stmt := statement
		if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			it.sequences[stmt.Name()]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "listener",
				Statement: stmt.Name(),
				Sequence:  it.sequences[stmt.Name()],
				Time:      compat.FormatTraceTime(batch.Time),
				New:       eipdNormalizeRows(batch.New),
				Old:       compat.NormalizeResults(batch.Old),
			})
			if stmt.Name() == "s1" && len(batch.New) > 0 {
				if event, ok := batch.New[len(batch.New)-1].Event(); ok {
					it.lastS1 = &event
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// eipdNormalizeRows renders rows like compat.NormalizeResults, then collapses
// Missing markers to the tagged null: Java's event.get() returns null for
// absent dynamic properties while Go reports Missing.
func eipdNormalizeRows(results []esper.Result) []compat.ResultRecord {
	records := compat.NormalizeResults(results)
	for _, record := range records {
		for name, value := range record.Fields {
			if eipdIsNilValue(value) {
				record.Fields[name] = map[string]any{"state": "null"}
				continue
			}
			if marker, ok := value.(map[string]any); ok && marker["state"] == "missing" {
				record.Fields[name] = map[string]any{"state": "null"}
			}
		}
	}
	return records
}

func eipdIsNilValue(value any) bool {
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

// buildS0 projects the pinned Java s0 statement. Java's leaf `?` resolves the
// property on the parent value's runtime type and reports exists() even when
// the resolved value is null; the Go path table therefore moves `?` onto the
// parent segments (null-safe navigation) while leaf named properties stay
// plain so a resolved null reports exists()=true like Java.
func (it *eipdIteration) buildS0(caseName, mode string) (esper.Plan, error) {
	eventValue := esper.EventValue[esper.Event]()
	prop := func(path string) esper.Expression[any] {
		return esper.Property[any](eventValue, path)
	}
	var selections []esper.Selection
	for _, column := range eipdColumns(caseName, mode) {
		if column.exists {
			path := column.goExists
			if path == "" {
				path = column.goValue
			}
			selections = append(selections, esper.Alias(column.name, esper.Exists(prop(path))))
		} else {
			selections = append(selections, esper.Alias(column.name, prop(column.goValue)))
		}
	}
	return it.env.Build(esper.FromAny(it.env, eipdGoTypeName(caseName, mode)).
		Select(selections...).
		Query(esper.StatementName("s0")))
}

// emitTypes asserts and emits one types record per output property, mirroring
// the Java assertStatement property-type checks (Object/Node on values,
// Boolean on exists columns).
func (it *eipdIteration) emitTypes(trace *compat.Trace, caseName string, step compat.Step) error {
	deployment, ok := it.deployments[step.Statement]
	if !ok {
		return fmt.Errorf("%s: types for undeployed statement %q", eventInfraPropertyDynamicID, step.Statement)
	}
	if len(deployment.Statements()) == 0 {
		return fmt.Errorf("%s: deployment %q has no statements", eventInfraPropertyDynamicID, step.Statement)
	}
	schema, ok := deployment.Statements()[0].Plan().ResultSchema()
	if !ok {
		return fmt.Errorf("%s: statement %q has no result schema", eventInfraPropertyDynamicID, step.Statement)
	}
	names := make([]string, 0, len(step.PropertyOrder))
	names = append(names, step.PropertyOrder...)
	for _, name := range names {
		expected, pinned := step.PropertyTypes[name]
		if !pinned {
			return fmt.Errorf("%s: types step has no pinned type for %q", eventInfraPropertyDynamicID, name)
		}
		field, found := schema.Field(name)
		if !found {
			return fmt.Errorf("%s: output schema has no property %q", eventInfraPropertyDynamicID, name)
		}
		canonical := "Object"
		if field.Type == reflect.TypeOf(true) {
			canonical = "Boolean"
		} else if it.mode == "xml" {
			canonical = "Node"
		}
		if canonical != expected {
			return fmt.Errorf("%s: property %q type %s, want %s", eventInfraPropertyDynamicID, name, canonical, expected)
		}
		it.sequences[step.Statement+":types"]++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "types",
			Statement: step.Statement,
			Sequence:  it.sequences[step.Statement+":types"],
			Time:      compat.FormatTraceTime(it.engine.Now()),
			Name:      name,
			Value:     canonical,
		})
	}
	return nil
}

// emitGetterProbe emits the dynamic-nested getter record: Java probes the raw
// item.id? getter on the last s1 (select-*) event after each send.
func (it *eipdIteration) emitGetterProbe(trace *compat.Trace, caseName string) error {
	if caseName != "dynamic-nested" {
		return nil
	}
	if it.lastS1 == nil {
		return fmt.Errorf("%s: no s1 event captured for getter probe", eventInfraPropertyDynamicID)
	}
	event := *it.lastS1
	goPath := "item?.id"
	if it.mode == "xml" {
		goPath = "myevent.item?.@id"
	}
	value := event.Get(goPath)
	exists := !value.IsMissing()
	record := map[string]any{"exists": exists}
	if exists {
		if value.IsNull() {
			record["value"] = map[string]any{"state": "null"}
		} else {
			record["value"] = value.Any()
		}
	}
	it.sequences["s1:getter"]++
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      caseName,
		Operation: "getter",
		Statement: "s1",
		Sequence:  it.sequences["s1:getter"],
		Time:      compat.FormatTraceTime(it.engine.Now()),
		Name:      "item.id?",
		Value:     record,
	})
	return nil
}

func (it *eipdIteration) undeploy(ctx context.Context, label string) error {
	deployment, ok := it.deployments[label]
	if !ok {
		return fmt.Errorf("%s: undeploy %q without deploy", eventInfraPropertyDynamicID, label)
	}
	if err := deployment.Undeploy(ctx); err != nil {
		return fmt.Errorf("%s: undeploy %q: %w", eventInfraPropertyDynamicID, label, err)
	}
	delete(it.deployments, label)
	return nil
}

func (it *eipdIteration) undeployAll(ctx context.Context) error {
	for index := len(it.deployOrder) - 1; index >= 0; index-- {
		label := it.deployOrder[index]
		deployment, ok := it.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", eventInfraPropertyDynamicID, label, err)
		}
		delete(it.deployments, label)
	}
	it.deployOrder = nil
	return nil
}

// ---- Schema registration -----------------------------------------------------

func (it *eipdIteration) registerSchemas(caseName, mode string) error {
	anyT := reflect.TypeOf((*any)(nil)).Elem()
	intSliceT := reflect.TypeOf([]int64{})
	mapAnyT := reflect.TypeOf(map[string]any{})
	reg := func(schema esper.Schema, err error) error {
		if err != nil {
			return fmt.Errorf("%s: register schema: %w", eventInfraPropertyDynamicID, err)
		}
		it.schemas[schema.Name()] = schema
		return nil
	}
	typeName := eipdGoTypeName(caseName, mode)
	switch caseName {
	case "dynamic-simple":
		switch mode {
		case "bean":
			// SupportMarkerInterface exposes no declared properties; every
			// property resolves on the runtime value's type.
			return reg(esper.RegisterMap(it.env, typeName, nil, esper.AllowDynamicFields()))
		case "map":
			return reg(esper.RegisterMap(it.env, typeName, nil, esper.AllowDynamicFields()))
		case "objectarray":
			return reg(esper.RegisterObjectArray(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("somefield", anyT), esper.FieldDef("id", anyT)}))
		case "xml":
			return reg(esper.RegisterXML(it.env, typeName, nil))
		case "avro":
			return reg(esper.RegisterAvro(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("id", anyT)}))
		case "json":
			return reg(esper.RegisterJSON(it.env, typeName, nil, esper.AllowDynamicFields()))
		}
	case "dynamic-nested":
		switch mode {
		case "bean":
			return reg(esper.RegisterStruct[eipdDynRoot](it.env, typeName))
		case "map":
			return reg(esper.RegisterMap(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", anyT)}, esper.AllowDynamicFields()))
		case "objectarray":
			return reg(esper.RegisterObjectArray(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", anyT)}))
		case "xml":
			return reg(esper.RegisterXML(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", anyT)}))
		case "avro":
			itemSchema, err := esper.RegisterAvro(it.env, typeName+"Item",
				[]esper.FieldSpec{esper.FieldDef("id", anyT)})
			if err != nil {
				return err
			}
			it.avroNested["item"] = itemSchema
			return reg(esper.RegisterAvro(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", anyT)}))
		case "json":
			return reg(esper.RegisterJSON(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", anyT)}, esper.AllowDynamicFields()))
		case "json-provided":
			return reg(esper.RegisterJSONFor[eipdNestedJSONProvided](it.env, typeName, nil))
		}
	case "nested-deep":
		switch mode {
		case "bean":
			return reg(esper.RegisterStruct[eipdDynRoot](it.env, typeName))
		case "map":
			return reg(esper.RegisterMap(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", mapAnyT)}, esper.AllowDynamicFields()))
		case "objectarray":
			type3, err := esper.RegisterObjectArray(it.env, typeName+"_3",
				[]esper.FieldSpec{esper.FieldDef("nestedNestedValue", anyT)})
			if err != nil {
				return err
			}
			type2, err := esper.RegisterObjectArray(it.env, typeName+"_2",
				[]esper.FieldSpec{esper.FieldDef("nestedNested", anyT), esper.FieldDef("nestedValue", anyT)},
				esper.WithNestedPropertySchema("nestedNested", type3))
			if err != nil {
				return err
			}
			type1, err := esper.RegisterObjectArray(it.env, typeName+"_1",
				[]esper.FieldSpec{esper.FieldDef("nested", anyT)},
				esper.WithNestedPropertySchema("nested", type2))
			if err != nil {
				return err
			}
			return reg(esper.RegisterObjectArray(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", anyT)},
				esper.WithNestedPropertySchema("item", type1)))
		case "xml":
			return reg(esper.RegisterXML(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", anyT)}))
		case "avro":
			nn, err := esper.RegisterAvro(it.env, typeName+"NN",
				[]esper.FieldSpec{esper.FieldDef("nestedNestedValue", anyT)})
			if err != nil {
				return err
			}
			it.avroNested["nestedNested"] = nn
			n, err := esper.RegisterAvro(it.env, typeName+"N",
				[]esper.FieldSpec{esper.FieldDef("nestedValue", anyT), esper.FieldDef("nestedNested", anyT)})
			if err != nil {
				return err
			}
			it.avroNested["nested"] = n
			itemSchema, err := esper.RegisterAvro(it.env, typeName+"Item",
				[]esper.FieldSpec{esper.FieldDef("nested", anyT)})
			if err != nil {
				return err
			}
			it.avroNested["item"] = itemSchema
			return reg(esper.RegisterAvro(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", anyT)}))
		case "json":
			return reg(esper.RegisterJSON(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", anyT)}, esper.AllowDynamicFields()))
		case "json-provided":
			return reg(esper.RegisterJSONFor[eipdDeepJSONProvided](it.env, typeName, nil))
		}
	case "rooted-nonsimple":
		switch mode {
		case "bean":
			return reg(esper.RegisterStruct[eipdDynRoot](it.env, typeName))
		case "map":
			return reg(esper.RegisterMap(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", mapAnyT)}, esper.AllowDynamicFields()))
		case "objectarray":
			nested, err := esper.RegisterObjectArray(it.env, typeName+"_1",
				[]esper.FieldSpec{
					esper.FieldDef("indexed", intSliceT),
					esper.FieldDef("mapped", mapAnyT),
					esper.FieldDef("arrayProperty", intSliceT),
					esper.FieldDef("mapProperty", mapAnyT),
				})
			if err != nil {
				return err
			}
			return reg(esper.RegisterObjectArray(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("someprop", anyT), esper.FieldDef("item", anyT)},
				esper.WithNestedPropertySchema("item", nested)))
		case "xml":
			return reg(esper.RegisterXML(it.env, typeName, nil))
		case "avro":
			itemSchema, err := esper.RegisterAvro(it.env, typeName+"Item",
				[]esper.FieldSpec{esper.FieldDef("indexed", intSliceT), esper.FieldDef("mapped", mapAnyT)})
			if err != nil {
				return err
			}
			it.avroNested["item"] = itemSchema
			return reg(esper.RegisterAvro(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", anyT)}))
		case "json":
			return reg(esper.RegisterJSON(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("item", anyT)}, esper.AllowDynamicFields()))
		case "json-provided":
			return reg(esper.RegisterJSONFor[eipdNonSimpleJSONProvided](it.env, typeName, nil))
		}
	case "rooted-simple":
		switch mode {
		case "bean":
			return reg(esper.RegisterMap(it.env, typeName, nil, esper.AllowDynamicFields()))
		case "map":
			return reg(esper.RegisterMap(it.env, typeName, nil, esper.AllowDynamicFields()))
		case "objectarray":
			type2, err := esper.RegisterObjectArray(it.env, typeName+"_2",
				[]esper.FieldSpec{esper.FieldDef("nestedNestedValue", anyT)})
			if err != nil {
				return err
			}
			type1, err := esper.RegisterObjectArray(it.env, typeName+"_1",
				[]esper.FieldSpec{esper.FieldDef("nestedValue", anyT), esper.FieldDef("nestedNested", anyT)},
				esper.WithNestedPropertySchema("nestedNested", type2))
			if err != nil {
				return err
			}
			return reg(esper.RegisterObjectArray(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("simpleProperty", anyT), esper.FieldDef("nested", anyT)},
				esper.WithNestedPropertySchema("nested", type1)))
		case "xml":
			return reg(esper.RegisterXML(it.env, typeName, nil))
		case "avro":
			nn, err := esper.RegisterAvro(it.env, typeName+"NN",
				[]esper.FieldSpec{esper.FieldDef("nestedNestedValue", anyT)})
			if err != nil {
				return err
			}
			it.avroNested["nestedNested"] = nn
			n, err := esper.RegisterAvro(it.env, typeName+"N",
				[]esper.FieldSpec{esper.FieldDef("nestedValue", anyT), esper.FieldDef("nestedNested", anyT)})
			if err != nil {
				return err
			}
			it.avroNested["nested"] = n
			return reg(esper.RegisterAvro(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("simpleProperty", anyT), esper.FieldDef("nested", anyT)}))
		case "json":
			return reg(esper.RegisterJSON(it.env, typeName, nil, esper.AllowDynamicFields()))
		case "json-provided":
			return reg(esper.RegisterJSONFor[eipdRootedSimpleJSONProvided](it.env, typeName, nil))
		}
	case "nonsimple":
		switch mode {
		case "bean":
			return reg(esper.RegisterStruct[eipdComplexProps](it.env, typeName))
		case "map":
			return reg(esper.RegisterMap(it.env, typeName, nil, esper.AllowDynamicFields()))
		case "objectarray":
			return reg(esper.RegisterObjectArray(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("indexed", anyT), esper.FieldDef("mapped", anyT)}))
		case "xml":
			return reg(esper.RegisterXML(it.env, typeName, nil))
		case "avro":
			return reg(esper.RegisterAvro(it.env, typeName,
				[]esper.FieldSpec{esper.FieldDef("indexed", anyT), esper.FieldDef("mapped", anyT)}))
		case "json":
			return reg(esper.RegisterJSON(it.env, typeName, nil, esper.AllowDynamicFields()))
		case "json-provided":
			return reg(esper.RegisterJSONFor[eipdLeafNonSimpleJSONProvided](it.env, typeName, nil))
		}
	}
	return fmt.Errorf("%s: no schema registration for %q/%q", eventInfraPropertyDynamicID, caseName, mode)
}

// ---- Send dispatch -----------------------------------------------------------

func (it *eipdIteration) send(ctx context.Context, caseName string, step compat.Step) error {
	var payload map[string]json.RawMessage
	if err := strictObject(step.Payload, &payload); err != nil {
		return fmt.Errorf("%s: decode send payload: %w", eventInfraPropertyDynamicID, err)
	}
	switch step.Mode {
	case "bean":
		return it.sendBean(ctx, caseName, step.EventType, payload)
	case "map":
		event, err := eipdDecodeValueMap(payload)
		if err != nil {
			return err
		}
		return it.engine.Send(ctx, step.EventType, event)
	case "objectarray":
		return it.sendObjectArray(ctx, caseName, step.EventType, payload)
	case "xml":
		return it.sendXML(ctx, step.EventType, payload)
	case "avro":
		return it.sendAvro(ctx, caseName, step.EventType, payload)
	case "json", "json-provided":
		return it.engine.SendJSON(ctx, step.EventType, step.Payload)
	default:
		return fmt.Errorf("%s: unsupported send mode %q", eventInfraPropertyDynamicID, step.Mode)
	}
}

func (it *eipdIteration) sendBean(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	variant := eipdVariant(payload)
	switch caseName {
	case "dynamic-simple":
		switch variant {
		case "impl-a":
			return it.engine.Send(ctx, eventType, eipdMarkerImplA{ID: "e1"})
		case "impl-b":
			return it.engine.Send(ctx, eventType, eipdMarkerImplB{ID: 1})
		case "impl-c":
			return it.engine.Send(ctx, eventType, eipdMarkerImplC{})
		}
	case "dynamic-nested":
		switch variant {
		case "s0-101":
			return it.engine.Send(ctx, eventType, eipdDynRoot{Item: eipdBeanS0{ID: 101}})
		case "str-abc":
			return it.engine.Send(ctx, eventType, eipdDynRoot{Item: "abc"})
		case "a-e1":
			return it.engine.Send(ctx, eventType, eipdDynRoot{Item: eipdBeanA{ID: "e1"}})
		case "b-e2":
			return it.engine.Send(ctx, eventType, eipdDynRoot{Item: eipdBeanB{ID: "e2"}})
		case "s1-102":
			return it.engine.Send(ctx, eventType, eipdDynRoot{Item: eipdBeanS1{ID: 102}})
		}
	case "nested-deep":
		switch variant {
		case "complex-default":
			return it.engine.Send(ctx, eventType, eipdDynRoot{Item: eipdDefaultComplexBean()})
		case "complex-mutated":
			return it.engine.Send(ctx, eventType, eipdDynRoot{Item: eipdMutatedComplexBean()})
		case "str-abc":
			return it.engine.Send(ctx, eventType, eipdDynRoot{Item: "abc"})
		}
	case "rooted-nonsimple":
		switch variant {
		case "str-xxx":
			return it.engine.Send(ctx, eventType, eipdDynRoot{Item: "xxx"})
		case "complex-default":
			return it.engine.Send(ctx, eventType, eipdDynRoot{Item: eipdDefaultComplexBean()})
		}
	case "rooted-simple":
		switch variant {
		case "complex-default":
			return it.engine.Send(ctx, eventType, eipdDefaultComplexBean())
		case "impl-a-x":
			return it.engine.Send(ctx, eventType, eipdMarkerImplA{ID: "x"})
		}
	case "nonsimple":
		switch variant {
		case "complex-default":
			return it.engine.Send(ctx, eventType, eipdDefaultComplexBean())
		}
	}
	return fmt.Errorf("%s: no bean sender for case %q variant %q", eventInfraPropertyDynamicID, caseName, variant)
}

func (it *eipdIteration) sendObjectArray(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	if caseName == "dynamic-nested" && eipdVariant(payload) == "oa-s0-101" {
		return it.engine.SendObjectArray(ctx, eventType, []any{eipdBeanS0{ID: 101}})
	}
	values, err := eipdDecodeAnyArray(payload["values"])
	if err != nil {
		return fmt.Errorf("%s: objectarray payload: %w", eventInfraPropertyDynamicID, err)
	}
	for index := range values {
		values[index] = eipdUnwrapOACell(values[index])
	}
	return it.engine.SendObjectArray(ctx, eventType, values)
}

// eipdUnwrapOACell converts a JSON {"values":[…]} cell into a plain []any for
// nested objectarray registration (the payload encodes nested OA rows as
func eipdUnwrapOACell(cell any) any {
	object, ok := cell.(map[string]any)
	if !ok {
		return cell
	}
	inner, ok := object["values"].([]any)
	if !ok {
		return object
	}
	for index := range inner {
		inner[index] = eipdUnwrapOACell(inner[index])
	}
	return inner
}

func (it *eipdIteration) sendXML(ctx context.Context, eventType string, payload map[string]json.RawMessage) error {
	var document string
	if err := json.Unmarshal(payload["goXml"], &document); err != nil {
		return fmt.Errorf("%s: xml payload goXml: %w", eventInfraPropertyDynamicID, err)
	}
	if document == "" {
		return fmt.Errorf("%s: xml payload has no goXml", eventInfraPropertyDynamicID)
	}
	return it.engine.SendXML(ctx, eventType, []byte(document))
}

func (it *eipdIteration) sendAvro(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	fields, err := eipdDecodeValueMap(payload)
	if err != nil {
		return err
	}
	schema, ok := it.schemas[eventType]
	if !ok {
		return fmt.Errorf("%s: no Avro schema %q", eventInfraPropertyDynamicID, eventType)
	}
	converted := make(map[string]any, len(fields))
	for name, value := range fields {
		converted[name] = it.convertAvroField(name, value)
	}
	record, err := esper.NewAvroRecordFromMap(schema, converted)
	if err != nil {
		return err
	}
	return it.engine.SendAvro(ctx, eventType, record)
}

// convertAvroField materializes nested Avro records from decoded payload maps;
// it.avroNested maps the enclosing field name to its nested record schema.
func (it *eipdIteration) convertAvroField(name string, value any) any {
	object, ok := value.(map[string]any)
	if !ok {
		return value
	}
	nested := make(map[string]any, len(object))
	for key, item := range object {
		nested[key] = it.convertAvroField(key, item)
	}
	schema, ok := it.avroNested[name]
	if !ok {
		return nested
	}
	record, err := esper.NewAvroRecordFromMap(schema, nested)
	if err != nil {
		return nested
	}
	return record
}

// ---- Payload decode helpers ---------------------------------------------------

func eipdVariant(payload map[string]json.RawMessage) string {
	var variant string
	if raw, ok := payload["variant"]; ok {
		_ = json.Unmarshal(raw, &variant)
	}
	return variant
}

// eipdDecodeValueMap decodes a payload object preserving integral numbers as
// int64 (Java sends Integer values, not doubles).
func eipdDecodeValueMap(payload map[string]json.RawMessage) (map[string]any, error) {
	result := make(map[string]any, len(payload))
	for key, raw := range payload {
		var value any
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode payload field %q: %w", key, err)
		}
		result[key] = eipdNormalizeJSONNumbers(value)
	}
	return result, nil
}

func eipdDecodeAnyArray(raw json.RawMessage) ([]any, error) {
	var values []any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&values); err != nil {
		return nil, err
	}
	for index := range values {
		values[index] = eipdNormalizeJSONNumbers(values[index])
	}
	return values, nil
}

func eipdNormalizeJSONNumbers(value any) any {
	switch current := value.(type) {
	case json.Number:
		if integer, err := current.Int64(); err == nil {
			return integer
		}
		if floating, err := current.Float64(); err == nil {
			return floating
		}
		return current.String()
	case map[string]any:
		for key, nested := range current {
			current[key] = eipdNormalizeJSONNumbers(nested)
		}
		return current
	case []any:
		for index := range current {
			current[index] = eipdNormalizeJSONNumbers(current[index])
		}
		return current
	}
	return value
}

// ---- Scenario loader ---------------------------------------------------------

func loadEventInfraPropertyDynamicScenario(reader io.Reader) (compat.Scenario, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario must be a JSON object: %w", eventInfraPropertyDynamicID, err)
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eventInfraPropertyDynamicID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eventInfraPropertyDynamicID ||
		metadata.Description != eventInfraPropertyDynamicDescription ||
		metadata.JavaCommit != eventInfraPropertyDynamicJavaCommit || metadata.JavaSource != eventInfraPropertyDynamicSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", eventInfraPropertyDynamicID)
	}
	if err := eipdRequireEqual(metadata.JavaFlags, eventInfraPropertyDynamicJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eipdRequireEqual(metadata.JavaRuntimes, eventInfraPropertyDynamicJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eipdRequireEqual(metadata.JavaNames, eventInfraPropertyDynamicJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eipdRequireEqual(metadata.JavaStaticID, eventInfraPropertyDynamicJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", eventInfraPropertyDynamicID, err)
	}
	if len(rawCases) != len(eventInfraPropertyDynamicCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", eventInfraPropertyDynamicID, len(rawCases), len(eventInfraPropertyDynamicCases))
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
		caseName := eventInfraPropertyDynamicCases[index]
		if definition.Case != caseName || definition.Ordinal != 0 ||
			definition.RuntimeID != eventInfraPropertyDynamicJavaRuntimeIDs[index] ||
			definition.ExecutionName != eventInfraPropertyDynamicJavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d identity = %+v, want case %q ordinal 0 runtime %s execution %s",
				eventInfraPropertyDynamicID, index, definition, caseName,
				eventInfraPropertyDynamicJavaRuntimeIDs[index], eventInfraPropertyDynamicJavaExecutions[index])
		}
		if definition.EPL != eipdCasePinnedEPL(caseName) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source", caseName)
		}
		if err := eipdRequireEqual(definition.Flags, eipdCaseFlags[caseName], "case "+caseName+" flags"); err != nil {
			return compat.Scenario{}, err
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", eventInfraPropertyDynamicID, err)
	}
	pinned := eventInfraPropertyDynamicPinnedSteps()
	if len(rawSteps) != len(pinned) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d steps, want %d", eventInfraPropertyDynamicID, len(rawSteps), len(pinned))
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
		if err := eipdValidateStep(index, object, step); err != nil {
			return compat.Scenario{}, err
		}
		if key := eipdStepKey(step); key != pinned[index] {
			return compat.Scenario{}, fmt.Errorf("scenario step %d = %q, want pinned %q", index, key, pinned[index])
		}
		steps[index] = step
	}
	return compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}, nil
}

// eipdCasePinnedEPL pins one representative Java statement per case (the bean
// iteration's s0 text; nested drops the s1 tail).
func eipdCasePinnedEPL(caseName string) string {
	switch caseName {
	case "dynamic-nested":
		return "@name('s0')  select item.id? as myid, exists(item.id?) as exists_myid from SupportBeanDynRoot;\n@name('s1') select * from SupportBeanDynRoot;\n"
	}
	return eipdS0EPL(caseName, "bean", "")
}

func eipdValidateStep(index int, object map[string]json.RawMessage, step compat.Step) error {
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
			required := []string{"op", "case", "statement", "eventType", "mode", "epl"}
			if step.Case == "dynamic-nested" {
				required = append(required, "name")
			}
			if err := require(required...); err != nil {
				return err
			}
			if step.Epl != eipdSchemaEPL(step.Case, step.Mode) {
				return fmt.Errorf("scenario step %d: schema EPL is not pinned", index)
			}
			if step.EventType != eipdJavaTypeName(step.Case, step.Mode) {
				return fmt.Errorf("scenario step %d: schema eventType is not pinned", index)
			}
			return nil
		}
		required := []string{"op", "case", "statement", "mode", "epl"}
		if step.Case == "dynamic-nested" {
			required = append(required, "name")
		}
		if err := require(required...); err != nil {
			return err
		}
		if step.Statement == "s0" {
			if selectAll := eipdSelectAllEPL(step.Case, step.Mode); selectAll != "" && step.Epl == selectAll {
				return nil
			}
			if step.Epl != eipdS0EPL(step.Case, step.Mode, step.Name) {
				return fmt.Errorf("scenario step %d: s0 EPL is not pinned", index)
			}
		}
		return nil
	case "deployed":
		return require("op", "case", "statement")
	case "types":
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

func eipdStepKey(step compat.Step) string {
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

// eventInfraPropertyDynamicPinnedSteps rebuilds the pinned step sequence from
// the same tables the scenario JSON was generated from.
func eventInfraPropertyDynamicPinnedSteps() []string {
	var keys []string
	for _, caseName := range eventInfraPropertyDynamicCases {
		keys = append(keys, eipdKey("case", caseName))
		for _, rep := range eipdReps(caseName) {
			for _, mode := range eipdModes(caseName) {
				if !eipdIterationExists(caseName, rep, mode) {
					continue
				}
				typeName := eipdJavaTypeName(caseName, mode)
				schemaEPL := eipdSchemaEPL(caseName, mode)
				keys = append(keys, eipdKeyFields("deploy", caseName, "schema", typeName, mode, rep, schemaEPL, "", "", "", ""))
				if schemaEPL != "" {
					keys = append(keys, eipdKey("deployed", caseName, "schema"))
				}
				s0EPL := eipdS0EPL(caseName, mode, rep)
				deployRep := ""
				if caseName == "dynamic-nested" {
					deployRep = rep
				}
				keys = append(keys, eipdKeyFields("deploy", caseName, "s0", "", mode, deployRep, s0EPL, "", "", "", ""))
				keys = append(keys, eipdKey("deployed", caseName, "s0"))
				if caseName == "dynamic-nested" {
					keys = append(keys, eipdKey("deployed", caseName, "s1"))
				}
				order := eipdTypeOrder(caseName)
				typesJSON := eipdTypesJSON(caseName, mode)
				keys = append(keys, eipdKeyFields("types", caseName, "s0", "", "", "", "", "", strings.Join(order, ","), typesJSON, ""))
				for _, payload := range eipdSends(caseName, mode) {
					keys = append(keys, eipdKeyFields("send", caseName, "", typeName, mode, "", "", "", "", "", eipdCanonicalPayload(payload)))
				}
				if caseName == "nested-deep" {
					keys = append(keys, eipdKey("undeploy", caseName, "s0"))
					selectAll := "@name('s0') select * from " + typeName
					keys = append(keys, eipdKeyFields("deploy", caseName, "s0", "", mode, "", selectAll, "", "", "", ""))
					keys = append(keys, eipdKey("deployed", caseName, "s0"))
					keys = append(keys, eipdKeyFields("send", caseName, "", typeName, mode, "", "", "", "", "", eipdCanonicalPayload(eipdSends(caseName, mode)[0])))
				}
				keys = append(keys, eipdKey("undeploy-all", caseName))
			}
		}
	}
	return keys
}

// eipdCanonicalPayload normalizes a pinned payload the same way eipdStepKey
// canonicalizes the loaded step payload (decode + re-encode).
func eipdCanonicalPayload(payload string) string {
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

func eipdKey(op, caseName string, rest ...string) string {
	parts := append([]string{op, caseName}, rest...)
	for len(parts) < 11 {
		parts = append(parts, "")
	}
	return strings.Join(parts, "|")
}

func eipdKeyFields(op, caseName, statement, eventType, mode, name, epl, expectError, propertyOrder, types, payload string) string {
	return strings.Join([]string{op, caseName, statement, eventType, mode, name, epl, expectError, propertyOrder, types, payload}, "|")
}

func eipdTypeOrder(caseName string) []string {
	var order []string
	for _, name := range eipdValuePropNames(caseName) {
		order = append(order, name, "exists_"+name)
	}
	return order
}

func eipdTypesJSON(caseName, mode string) string {
	encoded, err := json.Marshal(eipdTypePins(caseName, mode))
	if err != nil {
		return ""
	}
	return string(encoded)
}

// ---- small shared helpers -------------------------------------------------

func eipdRequireFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s: expected fields %v, got %v", eventInfraPropertyDynamicID, names, eipdKeysOf(object))
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s: missing field %q", eventInfraPropertyDynamicID, name)
		}
	}
	return nil
}

func eipdKeysOf(object map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	return keys
}

func eipdRequireEqual(actual, expected []string, name string) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("%s %s length = %d, want %d", eventInfraPropertyDynamicID, name, len(actual), len(expected))
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf("%s %s[%d] = %q, want %q", eventInfraPropertyDynamicID, name, index, actual[index], expected[index])
		}
	}
	return nil
}
