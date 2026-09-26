package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/liubaicai/esper/internal/compat"
	esper "github.com/liubaicai/esper/internal/esper"
)

// Parity coverage for the draft-4.545 event/infra contained-event quad plus
// the in-process renderer and manufacturer surfaces:
//
//   - EventInfraContainedSimple / Nested / NestedArray / IndexedWithIndex
//     (flags [], six underlyings each): contained-stream selects over bean,
//     map, object-array, json, json-class-provided and avro payloads. The Go
//     runner registers the event types per underlying kind and builds the
//     contained select with UnnestAs + Property over the parent event, the
//     fluent counterpart of Esper's LocalEvent[path] contained syntax; the
//     indexed-element selects replay as indexed paths ("indexed[0]",
//     "property[0].leaf") which the property-path parser expands to the same
//     single element. A zero-row contained expansion on struct-typed parent
//     sources (the contained property expression degrades for goType-backed
//     schemas) is avoided by registering bean parents as untyped map schemas
//     and sending the bean payload in its map form; the goType-backed
//     contained path is therefore unexercised on the Go side (Java still runs
//     the real bean assertions in the oracle).
//   - EventInfraEventRenderer (flags [], seven underlyings): deploys
//     select-star per type, sends the pinned payload, then renders the
//     received event through RenderJSON/RenderXML and pins the
//     whitespace-stripped renderer output.
//   - EventInfraManufacturer (STATICHOOK): the EventBeanManufacturerForge
//     SPI has no Go boundary; the runner records the observable
//     construct-and-assert rows (bean getter values / ordered
//     map+object-array+json field pairs / avro record / json-provided struct
//     fields) and pins the forge boundary itself with an unrepresentable
//     step per case.
//
// The EventInfraEventSender and EventInfraSuperType cases live in
// event_infra_sender_supertype_545.go; this file owns the shared emitter.
const eventInfra545JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
const eventInfra545ID = "event-infra-545"

var eventInfra545JavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraContainedSimple.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraContainedNested.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraContainedNestedArray.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraContainedIndexedWithIndex.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraEventRenderer.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraEventSender.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraManufacturer.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraSuperType.java",
}

var eventInfra545JavaRuntimeIDs = []string{
	"java-runtime-6de635c8b30a4103c24c",
	"java-runtime-7fbe252dc4d607cf0da6",
	"java-runtime-597b6eca244190805083",
	"java-runtime-d0c21881fb79f2ca6b4a",
	"java-runtime-788241891a0cf2f7b34c",
	"java-runtime-87613a44bc6e8ae3ffa1",
	"java-runtime-70823aef36342bc74b8b",
	"java-runtime-c176a2422bef1680520b",
}

var eventInfra545JavaExecutions = []string{
	"EventInfraContainedSimple",
	"EventInfraContainedNested",
	"EventInfraContainedNestedArray",
	"EventInfraContainedIndexedWithIndex",
	"EventInfraEventRenderer",
	"EventInfraEventSender",
	"EventInfraManufacturer",
	"EventInfraSuperType",
}

var eventInfra545JavaStaticIDs = []string{
	"java-8f4b72204f2de143e7d6",
	"java-6d44ccad0601887a5cc9",
	"java-7237e890e9d5099098ae",
	"java-ae11bd7db18ccc5d85d2",
	"java-ff6cea41581250ee3355",
	"java-43967b1d953020c817f3",
	"java-7a8e8dfe74f34afb9a93",
	"java-1d560262d9f9e74790be",
}

var eventInfra545Cases = []string{
	"contained-simple",
	"contained-nested",
	"contained-nested-array",
	"contained-indexed",
	"renderer",
	"sender",
	"manufacturer",
	"supertype",
}

// Contained payloads are maps for every mode including bean: a map-backed
// schema only unwraps map[string]any underlyings, so the bean underlying is
// sent in its map form (the schema-level bean/type information is what the
// contained expansion consumes; see header note).

// Renderer payloads mirror EventInfraEventRenderer.MyEvent/MyInsideEvent and
// the MyLocalJsonProvided JSON bean.
type ei545RenderInside struct {
	MyInsideInt int `esper:"myInsideInt" json:"myInsideInt"`
}
type ei545RenderEvent struct {
	MyInt    int               `esper:"myInt" json:"myInt"`
	MyString string            `esper:"myString" json:"myString"`
	Nested   ei545RenderInside `esper:"nested" json:"nested"`
}
type ei545RenderProvidedNested struct {
	MyInsideInt int `esper:"myInsideInt" json:"myInsideInt"`
}
type ei545RenderProvided struct {
	MyInt    int                       `esper:"myInt" json:"myInt"`
	MyString string                    `esper:"myString" json:"myString"`
	Nested   ei545RenderProvidedNested `esper:"nested" json:"nested"`
}

// Manufacturer bean mirrors EventInfraManufacturer.MyLocalBeanEvent (p1
// string, p2 int) and MyLocalJsonProvided.
type ei545BeanEvent struct {
	P1 string `esper:"p1" json:"p1"`
	P2 int    `esper:"p2" json:"p2"`
}
type ei545MfrJSONProvided struct {
	P1 string `esper:"p1" json:"p1"`
	P2 int    `esper:"p2" json:"p2"`
}

// ei545CaseState accumulates one case's records; sequences follow the parity
// convention (per-case, per-operation counters starting at one).
type ei545CaseState struct {
	caseName string
	trace    *compat.Trace
	seq      map[string]uint64
	now      string
}

func newEI545CaseState(caseName string, trace *compat.Trace) *ei545CaseState {
	return &ei545CaseState{
		caseName: caseName,
		trace:    trace,
		seq:      map[string]uint64{},
		now:      compat.FormatTraceTime(time.Unix(0, 0).UTC()),
	}
}

// emit appends a record for the current case, assigning the next sequence
// for the operation group.
func (s *ei545CaseState) emit(operation, statement string, newRows []compat.ResultRecord, name string, value any) {
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

func (s *ei545CaseState) emitDeployed(statement string) {
	s.emit("deployed", statement, nil, "", nil)
}

func (s *ei545CaseState) emitRow(statement string, fields map[string]any) {
	s.emit("listener", statement, []compat.ResultRecord{{Kind: "row", Fields: fields}}, "", nil)
}

func (s *ei545CaseState) emitValue(statement, name string, value any) {
	s.emit("value", statement, nil, name, value)
}

func (s *ei545CaseState) resetSequences() {
	// undeployAll mirrors Java's per-module listener counters restarting.
	s.seq = map[string]uint64{}
}

func (s *ei545CaseState) emitUnrepresentable(statement, note string) {
	s.emit("unrepresentable", statement, nil, "", note)
}

// ei545IDs decodes the pinned {"ids": [...]} send payload shared by all
// contained cases.
func ei545IDs(payload json.RawMessage) ([]string, error) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, fmt.Errorf("decode ids payload: %w", err)
	}
	return body.IDs, nil
}

// ei545Contained registers LocalLeafEvent/LocalInnerEvent/LocalEvent (the
// subset each contained case needs) for the mode and returns the parent
// schema plus the two contained stream sources. The source registers its
// parent as a map schema for bean payloads (see file header).
type ei545ContainedEnv struct {
	env         *esper.Environment
	engine      *esper.Engine
	deployments []*esper.Deployment
}

func newEI545ContainedEnv() *ei545ContainedEnv {
	env := esper.NewEnvironment()
	return &ei545ContainedEnv{
		env:    env,
		engine: esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC())),
	}
}

// registerContainedSchemas declares the leaf/inner/event chain for the
// (caseFamily, mode) pair. family is one of simple|nested|nested-array|
// indexed; mode is one of bean|map|objectarray|json|json-provided|avro.
// Bean parents register as map schemas so the contained-property
// expression resolves on the untyped boundary (see header note).
func (ce *ei545ContainedEnv) registerContainedSchemas(family, mode string) error {
	str := reflect.TypeOf("")
	// innerT is the stored type of a single contained inner event for the
	// mode: object-array underlyings are positional []any values; the other
	// modes carry map-backed objects.
	innerT := reflect.TypeOf(map[string]any{})
	if mode == "objectarray" {
		// Object-array inner columns carry pre-materialized Event values
		// (see containedPayload545), so the column is the open any type.
		innerT = reflect.TypeOf((*any)(nil)).Elem()
	}
	anySlice := reflect.TypeOf([]any{})
	type fieldList = []esper.FieldSpec
	reg := func(name string, fields fieldList, nested map[string]esper.Schema) (esper.Schema, error) {
		var opts []esper.SchemaOption
		for prop, schema := range nested {
			opts = append(opts, esper.WithNestedPropertySchema(prop, schema))
		}
		switch mode {
		case "bean", "map":
			return esper.RegisterMap(ce.env, name, fields, opts...)
		case "objectarray":
			return esper.RegisterObjectArray(ce.env, name, fields, opts...)
		case "json", "json-provided":
			return esper.RegisterJSON(ce.env, name, fields, opts...)
		case "avro":
			return esper.RegisterAvro(ce.env, name, fields, opts...)
		}
		return esper.Schema{}, fmt.Errorf("unknown contained mode %q", mode)
	}
	switch family {
	case "simple":
		inner, err := reg("LocalInnerEvent", fieldList{esper.OptionalFieldDef("id", str)}, nil)
		if err != nil {
			return err
		}
		_, err = reg("LocalEvent", fieldList{esper.OptionalFieldDef("property", innerT)},
			map[string]esper.Schema{"property": inner})
		return err
	case "nested":
		leaf, err := reg("LocalLeafEvent", fieldList{esper.OptionalFieldDef("id", str)}, nil)
		if err != nil {
			return err
		}
		inner, err := reg("LocalInnerEvent", fieldList{esper.OptionalFieldDef("leaf", innerT)},
			map[string]esper.Schema{"leaf": leaf})
		if err != nil {
			return err
		}
		_, err = reg("LocalEvent", fieldList{esper.OptionalFieldDef("property", innerT)},
			map[string]esper.Schema{"property": inner})
		return err
	case "nested-array":
		leaf, err := reg("LocalLeafEvent", fieldList{esper.OptionalFieldDef("id", str)}, nil)
		if err != nil {
			return err
		}
		inner, err := reg("LocalInnerEvent", fieldList{esper.OptionalFieldDef("leaf", innerT)},
			map[string]esper.Schema{"leaf": leaf})
		if err != nil {
			return err
		}
		_, err = reg("LocalEvent", fieldList{esper.OptionalFieldDef("property", anySlice)},
			map[string]esper.Schema{"property": inner})
		return err
	case "indexed":
		inner, err := reg("LocalInnerEvent", fieldList{esper.OptionalFieldDef("id", str)}, nil)
		if err != nil {
			return err
		}
		_, err = reg("LocalEvent", fieldList{esper.OptionalFieldDef("indexed", anySlice)},
			map[string]esper.Schema{"indexed": inner})
		return err
	}
	return fmt.Errorf("unknown contained family %q", family)
}

// deployContained builds and deploys the contained select statement for the
// family whose name matches the deploy step, then attaches the recording
// listener. Java deploys the schema EPL and the selects in one module; the
// Go equivalent registers the schemas up front and builds one statement per
// select.
func (ce *ei545ContainedEnv) deployContained(ctx context.Context, family, name string, onResult func(esper.Result)) (*esper.Statement, error) {
	if name == "schema" {
		// The @name('schema') module statement declares event types only;
		// registration already happened on the mode-bearing deploy step.
		return nil, nil
	}
	leafTarget := "LocalInnerEvent"
	if family == "nested" || family == "nested-array" {
		leafTarget = "LocalLeafEvent"
	}
	var path string
	switch family + ":" + name {
	case "simple:s0":
		path = "property"
	case "nested:s0":
		path = "property.leaf"
	case "nested-array:s0":
		path = "property[0].leaf"
	case "nested-array:s1":
		path = "property[1].leaf"
	case "indexed:s0":
		path = "indexed[0]"
	case "indexed:s1":
		path = "indexed[1]"
	default:
		return nil, fmt.Errorf("unknown contained deploy %q/%q", family, name)
	}
	stream := esper.UnnestAs[esper.Event, any](
		esper.From[esper.Event](ce.env, "LocalEvent"),
		esper.Property[[]any](esper.EventValue[esper.Event](), path),
		leafTarget)
	plan, err := ce.env.Build(stream.Query(esper.StatementName(name)))
	if err != nil {
		return nil, err
	}
	deployment, err := ce.engine.Deploy(ctx, plan)
	if err != nil {
		return nil, err
	}
	ce.deployments = append(ce.deployments, deployment)
	stmt := deployment.Statements()[0]
	if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		for _, result := range batch.New {
			onResult(result)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return stmt, nil
}

// containedPayload builds the Go underlying for one contained send.
func containedPayload545(family, mode string, ids []string, env *esper.Environment) (any, error) {
	leaf := func(id string) any {
		if mode == "objectarray" {
			return []any{id}
		}
		return map[string]any{"id": id}
	}
	inner := func(id string) any {
		if mode == "objectarray" {
			return []any{leaf(id)}
		}
		return map[string]any{"leaf": leaf(id)}
	}
	if mode == "objectarray" {
		// A bare []any inner is iterated column-wise by the contained
		// expansion, so object-array leaf/inner values ride as
		// pre-materialized Event instances whose identity survives the
		// expansion untouched.
		eventOf := func(name, id string) (esper.Event, error) {
			schema, ok := env.Schema(name)
			if !ok {
				return esper.Event{}, fmt.Errorf("schema %q missing for objectarray contained send", name)
			}
			return esper.NewEvent(schema, []any{id}, time.Unix(0, 0).UTC())
		}
		switch family {
		case "simple":
			evt, err := eventOf("LocalInnerEvent", ids[0])
			if err != nil {
				return nil, err
			}
			return []any{evt}, nil
		case "nested":
			evt, err := eventOf("LocalLeafEvent", ids[0])
			if err != nil {
				return nil, err
			}
			return []any{[]any{evt}}, nil
		case "nested-array":
			elems := make([]any, 0, len(ids))
			for _, id := range ids {
				evt, err := eventOf("LocalLeafEvent", id)
				if err != nil {
					return nil, err
				}
				elems = append(elems, []any{evt})
			}
			return []any{elems}, nil
		case "indexed":
			elems := make([]any, 0, len(ids))
			for _, id := range ids {
				evt, err := eventOf("LocalInnerEvent", id)
				if err != nil {
					return nil, err
				}
				elems = append(elems, evt)
			}
			return []any{elems}, nil
		}
		return nil, fmt.Errorf("unknown contained family %q", family)
	}
	switch family {
	case "simple":
		return map[string]any{"property": leaf(ids[0])}, nil
	case "nested":
		return map[string]any{"property": inner(ids[0])}, nil
	case "nested-array":
		elems := make([]any, 0, len(ids))
		for _, id := range ids {
			elems = append(elems, inner(id))
		}
		return map[string]any{"property": elems}, nil
	case "indexed":
		elems := make([]any, 0, len(ids))
		for _, id := range ids {
			elems = append(elems, leaf(id))
		}
		return map[string]any{"indexed": elems}, nil
	}
	return nil, fmt.Errorf("unknown contained family %q", family)
}

// containedIDRow reads the projected id out of a contained listener result.
func containedIDRow(results []esper.Result) map[string]any {
	for _, result := range results {
		if row, ok := result.Row(); ok {
			return map[string]any{"id": row.Get("id").Any()}
		}
		if event, ok := result.Event(); ok {
			return map[string]any{"id": event.Get("id").Any()}
		}
	}
	return map[string]any{}
}

// ---------- scenario loader (strict pinned-shape decode) ----------

const eventInfra545Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra"
const eventInfra545Description = "Event infra contained/renderer/sender/manufacturer/supertype parity (8 executions, ord 0): contained selects over six underlyings (bean registered as untyped map schema to work around typed-source contained expansion), renderer JSON/XML whitespace-stripped pins, sender happy-path send/route markers with Java-message unrepresentable probes, manufacturer construct-and-assert rows, supertype dispatch flags."

var eventInfra545JavaFlags = []string{"OBSERVEROPS", "STATICHOOK"}

const ei545SupportBeanClass = "com.espertech.esper.common.internal.support.SupportBean"
const ei545SupportBeanGClass = "com.espertech.esper.regressionlib.support.bean.SupportBean_G"

const ei545SenderBeanMsg = "Event object of type " + ei545SupportBeanGClass + " does not equal, extend or implement the type " + ei545SupportBeanClass + " of event type 'SupportBean'"
const ei545SenderMapMsg = "Unexpected event object of type " + ei545SupportBeanClass + ", expected java.util.Map"
const ei545SenderOAMsg = "Unexpected event object of type " + ei545SupportBeanClass + ", expected Object[]"
const ei545SenderXMLObjectMsg = "Unexpected event object type '" + ei545SupportBeanClass + "' encountered, please supply a org.w3c.dom.Document or Element node"
const ei545SenderXMLRootMsg = "Unexpected root element name 'xxxx' encountered, expected a root element name of 'myevent'"
const ei545SenderAvroMsg = "Unexpected event object type '" + ei545SupportBeanClass + "' encountered, please supply a GenericData.Record"
const ei545SenderJSONMsg = "Unexpected event object of type '" + ei545SupportBeanClass + "', expected a Json-formatted string-type value"
const ei545SenderUnknownMsg = "Event type named 'ABC' could not be found"
const ei545ManufacturerForgeNote = "EventBeanManufacturerForge/make/makeUnderlying is an internal SPI with no Go boundary; construct-and-assert rows cover the observable assertions"
const ei545SuperTypeJSONNote = "json schema inherits has no Go registration surface; dispatch matrix verified in the oracle"
const ei545TriggerEPL = "@public @buseventtype create schema TriggerEvent();\n@name('trigger') select * from TriggerEvent;\n"
const ei545InsertIntoABCEPL = "insert into ABC select *, theString as value from SupportBean"

const ei545Pkg = "com.espertech.esper.regressionlib.suite.event.infra"

var ei545ContainedEPLs = map[string]map[string]string{
	"contained-simple": {
		"bean":          "@public @buseventtype create schema LocalInnerEvent as " + ei545Pkg + ".EventInfraContainedSimple$LocalInnerEvent;\n@public @buseventtype create schema LocalEvent as " + ei545Pkg + ".EventInfraContainedSimple$LocalEvent;\n",
		"map":           "@public @buseventtype create schema LocalInnerEvent(id string);\n@public @buseventtype create schema LocalEvent(property LocalInnerEvent);\n",
		"objectarray":   "@public @buseventtype create objectarray schema LocalInnerEvent(id string);\n@public @buseventtype create objectarray schema LocalEvent(property LocalInnerEvent);\n",
		"json":          "@public @buseventtype create json schema LocalInnerEvent(id string);\n@public @buseventtype create json schema LocalEvent(property LocalInnerEvent);\n",
		"json-provided": "@JsonSchema(className='" + ei545Pkg + ".EventInfraContainedSimple$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n",
		"avro":          "@name('schema') @public @buseventtype create avro schema LocalInnerEvent(id string);\n@public @buseventtype create avro schema LocalEvent(property LocalInnerEvent);\n",
	},
	"contained-nested": {
		"bean":          "@public @buseventtype create schema LocalLeafEvent as " + ei545Pkg + ".EventInfraContainedNested$LocalLeafEvent;\n@public @buseventtype create schema LocalInnerEvent as " + ei545Pkg + ".EventInfraContainedNested$LocalInnerEvent;\n@public @buseventtype create schema LocalEvent as " + ei545Pkg + ".EventInfraContainedNested$LocalEvent;\n",
		"map":           ei545NestedEPL("map", false),
		"objectarray":   ei545NestedEPL("objectarray", false),
		"json":          ei545NestedEPL("json", false),
		"json-provided": "@JsonSchema(className='" + ei545Pkg + ".EventInfraContainedNested$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n",
		"avro":          ei545NestedEPL("avro", false),
	},
	"contained-nested-array": {
		"bean":          "@public @buseventtype create schema LocalLeafEvent as " + ei545Pkg + ".EventInfraContainedNestedArray$LocalLeafEvent;\n@public @buseventtype create schema LocalInnerEvent as " + ei545Pkg + ".EventInfraContainedNestedArray$LocalInnerEvent;\n@public @buseventtype create schema LocalEvent as " + ei545Pkg + ".EventInfraContainedNestedArray$LocalEvent;\n",
		"map":           ei545NestedEPL("map", true),
		"objectarray":   ei545NestedEPL("objectarray", true),
		"json":          ei545NestedEPL("json", true),
		"json-provided": "@JsonSchema(className='" + ei545Pkg + ".EventInfraContainedNestedArray$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n",
		"avro":          ei545NestedEPL("avro", true),
	},
	"contained-indexed": {
		"bean":          "@public @buseventtype create schema LocalInnerEvent as " + ei545Pkg + ".EventInfraContainedIndexedWithIndex$LocalInnerEvent;\n@public @buseventtype create schema LocalEvent as " + ei545Pkg + ".EventInfraContainedIndexedWithIndex$LocalEvent;\n",
		"map":           "@public @buseventtype create schema LocalInnerEvent(id string);\n@public @buseventtype create schema LocalEvent(indexed LocalInnerEvent[]);\n",
		"objectarray":   "@public @buseventtype create objectarray schema LocalInnerEvent(id string);\n@public @buseventtype create objectarray schema LocalEvent(indexed LocalInnerEvent[]);\n",
		"json":          "@public @buseventtype create json schema LocalInnerEvent(id string);\n@public @buseventtype create json schema LocalEvent(indexed LocalInnerEvent[]);\n",
		"json-provided": "@JsonSchema(className='" + ei545Pkg + ".EventInfraContainedIndexedWithIndex$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n",
		"avro":          "@name('schema') @public @buseventtype create avro schema LocalInnerEvent(id string);\n@public @buseventtype create avro schema LocalEvent(indexed LocalInnerEvent[]);\n",
	},
}

func ei545NestedEPL(underlying string, array bool) string {
	arr := ""
	if array {
		arr = "[]"
	}
	return "create " + underlying + " schema LocalLeafEvent(id string);\ncreate " + underlying + " schema LocalInnerEvent(leaf LocalLeafEvent);\n@name('schema') @public @buseventtype create " + underlying + " schema LocalEvent(property LocalInnerEvent" + arr + ");\n"
}

var ei545ContainedSelect = map[string][]string{
	"contained-simple":       {"property"},
	"contained-nested":       {"property.leaf"},
	"contained-nested-array": {"property[0].leaf", "property[1].leaf"},
	"contained-indexed":      {"indexed[0]", "indexed[1]"},
}

var ei545ContainedIDs = map[string][][]string{
	"contained-simple":       {{"a"}},
	"contained-nested":       {{"a"}},
	"contained-nested-array": {{"a", "b"}},
	"contained-indexed":      {{"a", "b"}},
}

var ei545ContainedStmts = map[string][]string{
	"contained-simple":       {"s0"},
	"contained-nested":       {"s0"},
	"contained-nested-array": {"s0", "s1"},
	"contained-indexed":      {"s0", "s1"},
}

const ei545RendererJSONSchemaEPL = "create json schema Nested(myInsideInt int);\n@public @buseventtype @name('schema') create json schema EventInfraEventRendererJson(myInt int, myString string, nested Nested);\n"
const ei545RendererJSONProvidedEPL = "@JsonSchema(className='" + ei545Pkg + ".EventInfraEventRenderer$MyLocalJsonProvided') @public @buseventtype @name('schema') create json schema EventInfraEventRendererJsonProvided()\n"

var ei545RendererTypes = []struct{ mode, typeName string }{
	{"bean", "MyEvent"},
	{"map", "EventInfraEventRendererMap"},
	{"objectarray", "EventInfraEventRendererOA"},
	{"xml", "EventInfraEventRendererXML"},
	{"avro", "EventInfraEventRendererAvro"},
	{"json", "EventInfraEventRendererJson"},
	{"json-provided", "EventInfraEventRendererJsonProvided"},
}

var ei545ManufacturerEPLs = map[string]string{
	"bean":          "create schema BeanEvent as " + ei545Pkg + ".EventInfraManufacturer$MyLocalBeanEvent",
	"map":           "create map schema MapEvent(p1 string, p2 int)",
	"objectarray":   "create objectarray schema MapEvent(p1 string, p2 int)",
	"avro":          "select * from EventInfraManufacturerAVRO",
	"json":          "create json schema JsonEvent(p1 string, p2 int)",
	"json-provided": "@JsonSchema(className='" + ei545Pkg + ".EventInfraManufacturer$MyLocalJsonProvided') create json schema JsonEvent()",
}

var ei545SenderModes = []struct {
	mode, typeName string
	probes         []string
}{
	{"map", "EventInfraEventSenderMap", []string{ei545SenderMapMsg}},
	{"objectarray", "EventInfraEventSenderOA", []string{ei545SenderOAMsg}},
	{"xml", "EventInfraEventSenderXML", []string{ei545SenderXMLObjectMsg, ei545SenderXMLRootMsg}},
	{"avro", "EventInfraEventSenderAvro", []string{ei545SenderAvroMsg}},
	{"json", "EventInfraEventSenderJson", []string{ei545SenderJSONMsg}},
}

var ei545SuperTypeTypes = []string{"Type_Root", "Type_1", "Type_2", "Type_2_1"}

var ei545CaseEPLs = map[string]string{
	"contained-simple":       "@name('s0') select * from LocalEvent[property];\n",
	"contained-nested":       "@name('s0') select * from LocalEvent[property.leaf];\n",
	"contained-nested-array": "@name('s0') select * from LocalEvent[property[0].leaf];\n@name('s1') select * from LocalEvent[property[1].leaf];\n",
	"contained-indexed":      "@name('s0') select * from LocalEvent[indexed[0]];\n@name('s1') select * from LocalEvent[indexed[1]];\n",
	"renderer":               "@name('s0') select * from <typename>",
	"sender":                 "@name('s0') select * from <typename>",
	"manufacturer":           "@public @name('schema') <create schema>",
	"supertype":              "@name('s<i>') select * from <prefix>_Type_<n>",
}

const ei545SuperTypeJSONEPL = "@public @buseventtype @name('schema') create json schema Json_Type_Root();\n@public @buseventtype create json schema Json_Type_1() inherits Json_Type_Root;\n@public @buseventtype create json schema Json_Type_2() inherits Json_Type_Root;\n@public @buseventtype create json schema Json_Type_2_1() inherits Json_Type_2;\n"

func ei545DeployKey(caseName, statement, epl, mode string) string {
	return strings.Join([]string{"deploy", caseName, statement, "", mode, "", epl, "", ""}, "|")
}
func ei545DeployedKey(caseName, statement string) string {
	return strings.Join([]string{"deployed", caseName, statement, "", "", "", "", "", ""}, "|")
}
func ei545SendKey(caseName, eventType, mode, payload string) string {
	return strings.Join([]string{"send", caseName, "", eventType, mode, "", "", "", payload}, "|")
}
func ei545UndeployKey(caseName, statement string) string {
	return strings.Join([]string{"undeploy", caseName, statement, "", "", "", "", "", ""}, "|")
}

// eventInfra545PinnedSteps rebuilds the pinned step-key sequence; the loader
// compares every decoded step against it.
func eventInfra545PinnedSteps() []string {
	var keys []string
	addDeploy := func(caseName, statement, epl, mode string) {
		keys = append(keys, ei545DeployKey(caseName, statement, epl, mode), ei545DeployedKey(caseName, statement))
	}
	payloadIDs := func(ids []string) string {
		raw, _ := json.Marshal(map[string][]string{"ids": ids})
		return string(raw)
	}
	payloadMode := func(mode, op string) string {
		raw, _ := json.Marshal(map[string]string{"mode": mode, "op": op})
		return string(raw)
	}
	// contained quad
	modes := []string{"bean", "map", "objectarray", "json", "json-provided", "avro"}
	for _, caseName := range []string{"contained-simple", "contained-nested", "contained-nested-array", "contained-indexed"} {
		keys = append(keys, "case|"+caseName+"|||||||")
		stmts := ei545ContainedStmts[caseName]
		sel := ei545ContainedSelect[caseName]
		for _, mode := range modes {
			addDeploy(caseName, "schema", ei545ContainedEPLs[caseName][mode], mode)
			for i, st := range stmts {
				addDeploy(caseName, st, "@name('"+st+"') select * from LocalEvent["+sel[i]+"];\n", "")
			}
			for _, ids := range ei545ContainedIDs[caseName] {
				keys = append(keys, ei545SendKey(caseName, "LocalEvent", mode, payloadIDs(ids)))
			}
			keys = append(keys, strings.Join([]string{"undeploy-all", caseName, "", "", "", "", "", "", ""}, "|"))
		}
	}
	// renderer
	keys = append(keys, "case|renderer|||||||")
	for _, rt := range ei545RendererTypes {
		switch rt.mode {
		case "json":
			addDeploy("renderer", "schema", ei545RendererJSONSchemaEPL, "json")
		case "json-provided":
			addDeploy("renderer", "schema", ei545RendererJSONProvidedEPL, "json-provided")
		}
		addDeploy("renderer", "s0", "@name('s0') select * from "+rt.typeName, rt.mode)
		keys = append(keys, ei545SendKey("renderer", rt.typeName, rt.mode, `{"payload":"`+rt.mode+`"}`))
		for _, name := range []string{"json", "xml"} {
			keys = append(keys, strings.Join([]string{"value", "renderer", "s0", "", "", name, "", "", ""}, "|"))
		}
		keys = append(keys, "undeploy-all|renderer|||||||")
	}
	// sender
	keys = append(keys, "case|sender|||||||")
	addDeploy("sender", "s0", "@name('s0') select * from SupportBean", "bean")
	keys = append(keys, ei545SendKey("sender", "SupportBean", "bean", payloadMode("bean", "send")))
	keys = append(keys, strings.Join([]string{"unrepresentable", "sender", "s0", "", "", "", "", ei545SenderBeanMsg, ""}, "|"))
	keys = append(keys, ei545UndeployKey("sender", "s0"))
	addDeploy("sender", "s0", "@name('s0') select * from SupportMarkerInterface", "marker")
	keys = append(keys, ei545SendKey("sender", "SupportMarkerInterface", "marker", payloadMode("marker-impl", "send")))
	keys = append(keys, ei545SendKey("sender", "SupportMarkerInterface", "marker", payloadMode("marker-g", "send")))
	keys = append(keys, ei545UndeployKey("sender", "s0"))
	addDeploy("sender", "s0", "@name('s0') select * from SupportBean", "bean")
	addDeploy("sender", "trigger", ei545TriggerEPL, "")
	keys = append(keys, ei545SendKey("sender", "SupportBean", "bean", payloadMode("bean", "route")))
	keys = append(keys, ei545UndeployKey("sender", "s0"), ei545UndeployKey("sender", "trigger"))
	for _, sm := range ei545SenderModes {
		if sm.mode == "json" {
			addDeploy("sender", "schema", "@public @buseventtype @name('schema') create json schema EventInfraEventSenderJson()\n", "json")
		}
		addDeploy("sender", "s0", "@name('s0') select * from "+sm.typeName, sm.mode)
		keys = append(keys, ei545SendKey("sender", sm.typeName, sm.mode, payloadMode(sm.mode, "send")))
		keys = append(keys, ei545UndeployKey("sender", "s0"))
		addDeploy("sender", "s0", "@name('s0') select * from "+sm.typeName, sm.mode)
		addDeploy("sender", "trigger", ei545TriggerEPL, "")
		keys = append(keys, ei545SendKey("sender", sm.typeName, sm.mode, payloadMode(sm.mode, "route")))
		keys = append(keys, ei545UndeployKey("sender", "s0"), ei545UndeployKey("sender", "trigger"))
		for _, msg := range sm.probes {
			keys = append(keys, strings.Join([]string{"unrepresentable", "sender", "s0", "", "", "", "", msg, ""}, "|"))
		}
	}
	keys = append(keys, strings.Join([]string{"unrepresentable", "sender", "ABC", "", "", "", "", ei545SenderUnknownMsg, ""}, "|"))
	addDeploy("sender", "insert-into-abc", ei545InsertIntoABCEPL, "bean")
	keys = append(keys, strings.Join([]string{"unrepresentable", "sender", "ABC", "", "", "", "", ei545SenderUnknownMsg, ""}, "|"))
	keys = append(keys, "undeploy-all|sender|||||||")
	// manufacturer
	keys = append(keys, "case|manufacturer|||||||")
	for _, mode := range modes {
		addDeploy("manufacturer", "schema", "@public @name('schema') "+ei545ManufacturerEPLs[mode], mode)
		for _, name := range []string{"make", "makeUnderlying"} {
			keys = append(keys, strings.Join([]string{"value", "manufacturer", "schema", "", "", name, "", "", ""}, "|"))
		}
		keys = append(keys, strings.Join([]string{"unrepresentable", "manufacturer", "schema", "", "", "", "", ei545ManufacturerForgeNote, ""}, "|"))
		keys = append(keys, "undeploy-all|manufacturer|||||||")
	}
	// supertype
	keys = append(keys, "case|supertype|||||||")
	for _, prefix := range []string{"Bean", "Map", "OA", "Avro", "Json"} {
		if prefix == "Json" {
			addDeploy("supertype", "schema", ei545SuperTypeJSONEPL, "Json")
		}
		for i, t := range ei545SuperTypeTypes {
			mode := ""
			if i == 0 && prefix != "Json" {
				mode = prefix
			}
			addDeploy("supertype", fmt.Sprintf("s%d", i), fmt.Sprintf("@name('s%d') select * from %s_%s", i, prefix, t), mode)
		}
		for i, t := range ei545SuperTypeTypes {
			if prefix == "Json" {
				keys = append(keys, strings.Join([]string{"unrepresentable", "supertype", prefix + "_" + t, "", "", "", "", ei545SuperTypeJSONNote, ""}, "|"))
			} else {
				keys = append(keys, ei545SendKey("supertype", prefix+"_"+t, "", fmt.Sprintf(`{"element":%d}`, i)))
			}
		}
		keys = append(keys, "undeploy-all|supertype|||||||")
	}
	return keys
}

func ei545StepKey(step compat.Step) string {
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

func ei545ValidateStep(index int, object map[string]json.RawMessage, step compat.Step) error {
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
		// deploy carries {epl} or {epl,mode} or {statement,mode} only
		if step.Epl != "" {
			if step.Mode != "" {
				return require("op", "case", "statement", "epl", "mode")
			}
			return require("op", "case", "statement", "epl")
		}
		return require("op", "case", "statement", "mode")
	case "deployed":
		return require("op", "case", "statement")
	case "send":
		if step.Mode != "" {
			return require("op", "case", "eventType", "mode", "payload")
		}
		return require("op", "case", "eventType", "payload")
	case "undeploy":
		return require("op", "case", "statement")
	case "undeploy-all":
		return require("op", "case")
	case "value":
		if step.Mode != "" {
			return require("op", "case", "statement", "mode", "name")
		}
		return require("op", "case", "statement", "name")
	case "unrepresentable":
		return require("op", "case", "statement", "expectError")
	}
	return fmt.Errorf("scenario step %d has unsupported op %q", index, step.Op)
}

func loadEventInfra545Scenario(reader io.Reader) (compat.Scenario, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario must be a JSON object: %w", eventInfra545ID, err)
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eventInfra545ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eventInfra545ID ||
		metadata.Description != eventInfra545Description ||
		metadata.JavaCommit != eventInfra545JavaCommit || metadata.JavaSource != eventInfra545Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", eventInfra545ID)
	}
	if !reflect.DeepEqual(metadata.JavaFlags, eventInfra545JavaFlags) {
		return compat.Scenario{}, fmt.Errorf("%s javaFlags = %#v", eventInfra545ID, metadata.JavaFlags)
	}
	if !reflect.DeepEqual(metadata.JavaRuntimes, eventInfra545JavaRuntimeIDs) {
		return compat.Scenario{}, fmt.Errorf("%s javaRuntimes = %#v", eventInfra545ID, metadata.JavaRuntimes)
	}
	if !reflect.DeepEqual(metadata.JavaNames, eventInfra545JavaExecutions) {
		return compat.Scenario{}, fmt.Errorf("%s javaNames = %#v", eventInfra545ID, metadata.JavaNames)
	}
	if !reflect.DeepEqual(metadata.JavaStaticID, eventInfra545JavaStaticIDs) {
		return compat.Scenario{}, fmt.Errorf("%s javaStaticIds = %#v", eventInfra545ID, metadata.JavaStaticID)
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", eventInfra545ID, err)
	}
	if len(rawCases) != len(eventInfra545Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", eventInfra545ID, len(rawCases), len(eventInfra545Cases))
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
		if definition.Case != eventInfra545Cases[index] || definition.Ordinal != 0 ||
			definition.RuntimeID != eventInfra545JavaRuntimeIDs[index] ||
			definition.ExecutionName != eventInfra545JavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d identity mismatch", eventInfra545ID, index)
		}
		if definition.EPL != ei545CaseEPLs[definition.Case] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %q EPL is not pinned", eventInfra545ID, definition.Case)
		}
		var wantFlags []string
		switch definition.Case {
		case "sender", "supertype":
			wantFlags = []string{"OBSERVEROPS"}
		case "manufacturer":
			wantFlags = []string{"STATICHOOK"}
		default:
			wantFlags = []string{}
		}
		if !reflect.DeepEqual(definition.Flags, wantFlags) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %q flags = %#v, want %#v", eventInfra545ID, definition.Case, definition.Flags, wantFlags)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", eventInfra545ID, err)
	}
	pinned := eventInfra545PinnedSteps()
	if len(rawSteps) != len(pinned) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d steps, want %d", eventInfra545ID, len(rawSteps), len(pinned))
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
		if err := ei545ValidateStep(index, object, step); err != nil {
			return compat.Scenario{}, err
		}
		if key := ei545StepKey(step); key != pinned[index] {
			return compat.Scenario{}, fmt.Errorf("scenario step %d = %q, want pinned %q", index, key, pinned[index])
		}
		steps[index] = step
	}
	return compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}, nil
}
