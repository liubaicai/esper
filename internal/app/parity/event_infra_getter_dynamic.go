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
	eventInfraGetterDynamicID         = "event-infra-getter-dynamic"
	eventInfraGetterDynamicJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	// The cluster spans five sibling files in suite/event/infra; the scenario
	// pins the shared directory while the evidence metadata carries the files.
	eventInfraGetterDynamicSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra"
)

var eventInfraGetterDynamicJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraGetterDynamicSimple.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraGetterDynamicNested.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraGetterDynamicNestedDeep.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraGetterDynamicIndexexPropertyPredefined.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraGetterDynamicSimplePropertyPredefined.java",
}

var eventInfraGetterDynamicJavaRuntimeIDs = []string{
	"java-runtime-23791e63adf0caaff235",
	"java-runtime-c6db3a43dfabaf9c0f86",
	"java-runtime-5ef37219e1c801e38af7",
	"java-runtime-1a68e3e356c953b9c774",
	"java-runtime-0d912287d52a89b8725c",
}

var eventInfraGetterDynamicJavaStaticIDs = []string{
	"java-aab99c02485fd4c7259e",
	"java-681d397823021b2f5617",
	"java-6d027a1951a0f0c26280",
	"java-d89f3bc0bdc8ad41ac17",
	"java-c869682acfea77412912",
}

var eventInfraGetterDynamicJavaExecutions = []string{
	"EventInfraGetterDynamicSimple",
	"EventInfraGetterDynamicNested",
	"EventInfraGetterDynamicNestedDeep",
	"EventInfraGetterDynamicIndexexPropertyPredefined",
	"EventInfraGetterDynamicSimplePropertyPredefined",
}

var eventInfraGetterDynamicJavaFlags []string

var eventInfraGetterDynamicCases = []string{
	"dynamic-simple",
	"dynamic-nested",
	"dynamic-nested-deep",
	"indexed-predefined",
	"simple-predefined",
}

const eventInfraGetterDynamicDescription = "EventInfraGetterDynamic* dynamic-property getter slice (all ord 0, no flags): dynamic-simple replays property? over six underlyings with exists true/true/false and the beanBackedJsonOrAvro exists-on-absent quirk for json-provided and avro (Avro dynamic unrepresentable in Go); dynamic-nested replays property?.id and dynamic-nested-deep property?.leaf.id with null-safe tails (objectarray has no sender and asserts the getter is null); indexed-predefined replays array[0]?/array[1]? non-null/exists/typeof pairs with bean and json-provided fragments; simple-predefined replays declared property string access including Avro. Java sources are the five sibling files under suite/event/infra (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c)."

// Pinned s1 EPL per case (byte-exact, including the indexed case's missing
// space after "c1,").
var eventInfraGetterDynamicS1 = map[string]string{
	"dynamic-simple":      "@name('s1') select property? as c0, exists(property?) as c1, typeof(property?) as c2 from LocalEvent;\n",
	"dynamic-nested":      "@name('s1') select property?.id as c0, exists(property?.id) as c1, typeof(property?.id) as c2 from LocalEvent;\n",
	"dynamic-nested-deep": "@name('s1') select property?.leaf.id as c0, exists(property?.leaf.id) as c1, typeof(property?.leaf.id) as c2 from LocalEvent;\n",
	"indexed-predefined":  "@name('s1') select array[0]? as c0, array[1]? as c1,exists(array[0]?) as c2, exists(array[1]?) as c3, typeof(array[0]?) as c4, typeof(array[1]?) as c5 from LocalEvent;\n",
	"simple-predefined":   "@name('s1') select property? as c0, exists(property?) as c1, typeof(property?) as c2 from LocalEvent;\n",
}

const eventInfraGetterDynamicS0 = "@name('s0') select * from LocalEvent"

const eventInfraGetterDynamicPkg = "com.espertech.esper.regressionlib.suite.event.infra"

// Pinned create-schema EPL per case and underlying mode.
func eventInfraGetterDynamicSchemaEPL(caseName, mode string) (string, bool) {
	simpleBase := eventInfraGetterDynamicPkg + ".EventInfraGetterDynamicSimple"
	nestedBase := eventInfraGetterDynamicPkg + ".EventInfraGetterDynamicNested"
	deepBase := eventInfraGetterDynamicPkg + ".EventInfraGetterDynamicNestedDeep"
	indexedBase := eventInfraGetterDynamicPkg + ".EventInfraGetterDynamicIndexexPropertyPredefined"
	predefBase := eventInfraGetterDynamicPkg + ".EventInfraGetterDynamicSimplePropertyPredefined"
	switch caseName {
	case "dynamic-simple":
		switch mode {
		case "bean":
			return "@public @buseventtype create schema LocalEvent as " + simpleBase + "$LocalEvent;\n" +
				"@public @buseventtype create schema LocalEventSubA as " + simpleBase + "$LocalEventSubA;\n", true
		case "map":
			return "@public @buseventtype create schema LocalEvent();\n", true
		case "objectarray":
			return "@public @buseventtype create objectarray schema LocalEvent();\n" +
				"@public @buseventtype create objectarray schema LocalEventSubA (property string) inherits LocalEvent;\n", true
		case "json":
			return "@public @buseventtype @JsonSchema(dynamic=true) create json schema LocalEvent();\n", true
		case "json-provided":
			return "@JsonSchema(className='" + simpleBase + "$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n", true
		case "avro":
			return "@public @buseventtype create avro schema LocalEvent();\n", true
		}
	case "dynamic-nested", "dynamic-nested-deep":
		switch mode {
		case "bean":
			// NestedDeep's bean EPL pins EventInfraGetterDynamicNested's
			// LocalEvent/LocalEventSubA classes (cross-file reuse in Java).
			return "@public @buseventtype create schema LocalEvent as " + nestedBase + "$LocalEvent;\n" +
				"@public @buseventtype create schema LocalEventSubA as " + nestedBase + "$LocalEventSubA;\n", true
		case "map", "objectarray", "json", "avro":
			return "@public @buseventtype @JsonSchema(dynamic=true) create " + mode + " schema LocalEvent();\n", true
		case "json-provided":
			base := nestedBase
			if caseName == "dynamic-nested-deep" {
				base = deepBase
			}
			return "@JsonSchema(className='" + base + "$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n", true
		}
	case "indexed-predefined":
		switch mode {
		case "bean":
			return "@public @buseventtype create schema LocalInnerEvent as " + indexedBase + "$LocalInnerEvent;\n" +
				"@public @buseventtype create schema LocalEvent as " + indexedBase + "$LocalEvent;\n" +
				"@public @buseventtype create schema LocalEventSubA as " + indexedBase + "$LocalEventSubA;\n", true
		case "map":
			return "@public @buseventtype create schema LocalInnerEvent();\n" +
				"@public @buseventtype create schema LocalEvent(array LocalInnerEvent[]);\n", true
		case "objectarray":
			return "@public @buseventtype create objectarray schema LocalEvent();\n" +
				"@public @buseventtype create objectarray schema LocalEventSubA (array string[]) inherits LocalEvent;\n", true
		case "json":
			return "@public @buseventtype create json schema LocalInnerEvent();\n" +
				"@public @buseventtype create json schema LocalEvent(array LocalInnerEvent[]);\n", true
		case "json-provided":
			return "@JsonSchema(className='" + indexedBase + "$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n", true
		case "avro":
			return "@public @buseventtype create avro schema LocalInnerEvent();\n" +
				"@public @buseventtype create avro schema LocalEvent(array LocalInnerEvent[]);\n", true
		}
	case "simple-predefined":
		switch mode {
		case "bean":
			return "@public @buseventtype create schema LocalEvent as " + predefBase + "$LocalEvent;\n", true
		case "map", "objectarray", "json", "avro":
			return "@name('schema') @buseventtype @public create " + mode + " schema LocalEvent(property string);\n", true
		case "json-provided":
			return "@JsonSchema(className='" + predefBase + "$MyLocalJsonProvided') @buseventtype @public create json schema LocalEvent();\n", true
		}
	}
	return "", false
}

// Pinned getter probe paths per case (the Java getGetter names).
var eventInfraGetterDynamicProbes = map[string][]string{
	"dynamic-simple":      {"property?"},
	"dynamic-nested":      {"property?.id"},
	"dynamic-nested-deep": {"property?.leaf.id"},
	"indexed-predefined":  {"array[0]?", "array[1]?"},
	"simple-predefined":   {"property?"},
}

// Pinned send payload sequence per case.
var eventInfraGetterDynamicPayloads = map[string][]string{
	"dynamic-simple":      {`{"property":"a"}`, `{"property":null}`, `{}`},
	"dynamic-nested":      {`{"property":{"id":"a"}}`, `{"property":{"id":null}}`, `{}`},
	"dynamic-nested-deep": {`{"property":{"leaf":{"id":"a"}}}`, `{"property":{"leaf":{"id":null}}}`, `{"property":{"leaf":null}}`, `{}`},
	"indexed-predefined":  {`{"array":[{},{}]}`, `{"array":[{}]}`, `{"array":[]}`, `{"array":null}`, `{}`},
	"simple-predefined":   {`{"property":"a"}`, `{"property":null}`},
}

var eventInfraGetterDynamicModes = []string{"bean", "map", "objectarray", "json", "json-provided", "avro"}

// avroDynamicNotes pin the unrepresentable marker text per case.
var eventInfraGetterDynamicAvroNotes = map[string]string{
	"dynamic-simple":      "Java replays the Avro dynamic-property iteration: create avro schema LocalEvent() deploys, s0 and s1 deploy, three sends (property 'a', null, absent) assert c0/c1/c2 and the property? getter with beanBackedJsonOrAvro exists-on-absent; Go Avro schemas reject AllowDynamicFields and undeclared record fields, so the iteration is unrepresentable",
	"dynamic-nested":      "Java replays the Avro dynamic-property iteration: create avro schema LocalEvent() deploys, s0 and s1 deploy, three sends assert c0/c1/c2 and the property?.id getter; Go Avro schemas reject AllowDynamicFields and undeclared record fields, so the iteration is unrepresentable",
	"dynamic-nested-deep": "Java replays the Avro dynamic-property iteration: create avro schema LocalEvent() deploys, s0 and s1 deploy, four sends assert c0/c1/c2 and the property?.leaf.id getter; Go Avro schemas reject AllowDynamicFields and undeclared record fields, so the iteration is unrepresentable",
}

// eigdSimpleBase mirrors EventInfraGetterDynamicSimple.LocalEvent (empty bean).
type eigdSimpleBase struct{}

// eigdSimpleSubA mirrors LocalEventSubA: a nullable string property.
type eigdSimpleSubA struct {
	Property *string `esper:"property" json:"property"`
}

// eigdSimpleJSONProvided mirrors MyLocalJsonProvided (public field bean).
type eigdSimpleJSONProvided struct {
	Property *string `esper:"property" json:"property"`
}

// eigdNestedBase mirrors EventInfraGetterDynamicNested.LocalEvent.
type eigdNestedBase struct{}

// eigdNestedInner mirrors LocalInnerEvent {id}.
type eigdNestedInner struct {
	ID *string `esper:"id" json:"id"`
}

// eigdNestedSubA mirrors LocalEventSubA {property: LocalInnerEvent}.
type eigdNestedSubA struct {
	Property *eigdNestedInner `esper:"property" json:"property"`
}

type eigdNestedJSONProvidedInner struct {
	ID *string `esper:"id" json:"id"`
}

type eigdNestedJSONProvided struct {
	Property *eigdNestedJSONProvidedInner `esper:"property" json:"property"`
}

// eigdDeepSubA mirrors EventInfraGetterDynamicNestedDeep.LocalEventSubA. The
// Java execution registers the bean event type for the EMPTY base class
// (Nested.LocalEvent) yet sends NestedDeep.LocalEventSubA instances under the
// "LocalEvent" name, so the registered schema carries no declared fields and
// the '?' getter resolves on the underlying bean at runtime. Go reproduces
// that shape by hiding the field from schema inference (esper:"-") and
// exposing it through the JavaBean-style accessor the dynamic fallback finds.
type eigdDeepSubA struct {
	Property *eigdDeepInner `esper:"-" json:"-"`
}

// GetProperty is the JavaBean accessor the dynamic property fallback invokes.
func (e eigdDeepSubA) GetProperty() *eigdDeepInner { return e.Property }

type eigdDeepInner struct {
	Leaf *eigdDeepLeaf `esper:"leaf" json:"leaf"`
}

func (e eigdDeepInner) GetLeaf() *eigdDeepLeaf { return e.Leaf }

type eigdDeepLeaf struct {
	ID *string `esper:"id" json:"id"`
}

func (e eigdDeepLeaf) GetID() *string { return e.ID }

type eigdDeepJSONProvidedLeaf struct {
	ID *string `esper:"id" json:"id"`
}

type eigdDeepJSONProvidedInner struct {
	Leaf *eigdDeepJSONProvidedLeaf `esper:"leaf" json:"leaf"`
}

type eigdDeepJSONProvided struct {
	Property *eigdDeepJSONProvidedInner `esper:"property" json:"property"`
}

// eigdIndexedInner mirrors the empty LocalInnerEvent bean.
type eigdIndexedInner struct{}

// eigdIndexedBase mirrors the empty LocalEvent bean.
type eigdIndexedBase struct{}

// eigdIndexedSubA mirrors LocalEventSubA {array: LocalInnerEvent[]}.
type eigdIndexedSubA struct {
	Array []eigdIndexedInner `esper:"array" json:"array"`
}

type eigdIndexedJSONProvidedInner struct{}

type eigdIndexedJSONProvided struct {
	Array []eigdIndexedJSONProvidedInner `esper:"array" json:"array"`
}

// eigdPredefBean mirrors EventInfraGetterDynamicSimplePropertyPredefined.LocalEvent.
type eigdPredefBean struct {
	Property *string `esper:"property" json:"property"`
}

type eigdPredefJSONProvided struct {
	Property *string `esper:"property" json:"property"`
}

// eigdIteration is the per-(case, underlying) replay state. Java reuses one
// runtime per execution and undeploys between underlyings; Go event-type
// registrations are environment-scoped, so each iteration gets a fresh
// environment/engine pair (the undeploy-all boundary is the engine close).
type eigdIteration struct {
	env         *esper.Environment
	engine      *esper.Engine
	schemas     map[string]esper.Schema
	deployments map[string]*esper.Deployment
	deployOrder []string
	sequences   map[string]uint64
	lastS0      *esper.Event
	avroDynamic bool
}

func runEventInfraGetterDynamicScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eventInfraGetterDynamicCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEventInfraGetterDynamicCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eventInfraGetterDynamicID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", eventInfraGetterDynamicID, scenario.ID)
	}
	return trace, nil
}

func runEventInfraGetterDynamicCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	var iter *eigdIteration
	defer func() {
		if iter != nil && iter.engine != nil {
			_ = iter.engine.Close(context.Background())
		}
	}()

	newIteration := func() *eigdIteration {
		if iter != nil && iter.engine != nil {
			_ = iter.engine.Close(context.Background())
		}
		iter = &eigdIteration{
			env:         esper.NewEnvironment(),
			schemas:     make(map[string]esper.Schema),
			deployments: make(map[string]*esper.Deployment),
			sequences:   make(map[string]uint64),
		}
		iter.engine = esper.NewEngine(iter.env,
			esper.WithRuntimeURI(eventInfraGetterDynamicJavaRuntimeIDs[eventInfraGetterDynamicOrdinal(caseName)]),
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
				return trace, fmt.Errorf("%s: deployed marker without deploy", eventInfraGetterDynamicID)
			}
			iter.sequences[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  iter.sequences[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(iter.engine.Now()),
			})
		case "send":
			if iter == nil {
				return trace, fmt.Errorf("%s: send without deploy", eventInfraGetterDynamicID)
			}
			if err := iter.send(ctx, caseName, step); err != nil {
				return trace, err
			}
			if err := iter.emitGetterProbes(&trace, caseName); err != nil {
				return trace, err
			}
		case "types":
			if iter == nil {
				return trace, fmt.Errorf("%s: types probe without deploy", eventInfraGetterDynamicID)
			}
			if err := iter.emitNullGetterProbe(&trace, caseName, step); err != nil {
				return trace, err
			}
		case "unrepresentable":
			if iter == nil {
				return trace, fmt.Errorf("%s: unrepresentable without deploy", eventInfraGetterDynamicID)
			}
			if err := iter.emitUnrepresentable(&trace, caseName, step); err != nil {
				return trace, err
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
			return trace, fmt.Errorf("%s: unsupported step op %q", eventInfraGetterDynamicID, step.Op)
		}
	}
	return trace, nil
}

func eventInfraGetterDynamicOrdinal(caseName string) int {
	for index, name := range eventInfraGetterDynamicCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// deploy handles the "schema", "s0" and "s1" deploy steps. The schema step
// registers the pinned Go event types for the underlying mode (the Java
// compileDeploy of the create-schema EPL); s0 deploys select-* and s1 the
// pinned projection statement, each with a trace listener.
func (it *eigdIteration) deploy(ctx context.Context, trace *compat.Trace, caseName string, step compat.Step) error {
	switch step.Statement {
	case "schema":
		return it.registerSchemas(caseName, step.Mode)
	case "s0":
		if step.Epl != eventInfraGetterDynamicS0 {
			return fmt.Errorf("%s: s0 EPL %q is not pinned", eventInfraGetterDynamicID, step.Epl)
		}
		plan, err := it.env.Build(esper.FromAny(it.env, "LocalEvent").Query(esper.StatementName("s0")))
		if err != nil {
			return fmt.Errorf("%s: build s0: %w", eventInfraGetterDynamicID, err)
		}
		return it.deployPlan(ctx, trace, caseName, step.Statement, plan)
	case "s1":
		if step.Epl != eventInfraGetterDynamicS1[caseName] {
			return fmt.Errorf("%s: s1 EPL %q is not pinned", eventInfraGetterDynamicID, step.Epl)
		}
		plan, err := it.buildS1(caseName)
		if err != nil {
			return err
		}
		return it.deployPlan(ctx, trace, caseName, step.Statement, plan)
	default:
		return fmt.Errorf("%s: unsupported deploy statement %q", eventInfraGetterDynamicID, step.Statement)
	}
}

func (it *eigdIteration) deployPlan(ctx context.Context, trace *compat.Trace, caseName, label string, plan esper.Plan) error {
	deployment, err := it.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", eventInfraGetterDynamicID, label, err)
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
				New:       it.normalizeRows(stmt.Name(), batch.New),
				Old:       compat.NormalizeResults(batch.Old),
			})
			if stmt.Name() == "s0" && len(batch.New) > 0 {
				if event, ok := batch.New[len(batch.New)-1].Event(); ok {
					it.lastS0 = &event
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// normalizeRows renders s1 rows like compat.NormalizeResults, then applies the
// two pinned Java projections the Go expression surface cannot carry verbatim:
// c0/c1 raw values render null for absent (Java get() returns null where Go
// reports missing), and the indexed case's c0/c1/c4/c5 are Java non-null
// assertions (get()!=null / typeof()!=null) rendered as booleans.
func (it *eigdIteration) normalizeRows(statement string, results []esper.Result) []compat.ResultRecord {
	records := compat.NormalizeResults(results)
	for _, record := range records {
		for name, value := range record.Fields {
			// Java's row rendering tags every null event.get() result; Go
			// distinguishes a present-nil field (plain null, including typed
			// nils like a nil slice column) and a Missing marker, so both
			// collapse to the tagged null here.
			if eigdIsNilValue(value) {
				record.Fields[name] = map[string]any{"state": "null"}
				continue
			}
			if missing, ok := value.(map[string]any); ok && missing["state"] == "missing" {
				record.Fields[name] = map[string]any{"state": "null"}
			}
		}
	}
	if statement != "s1" {
		return records
	}
	for _, record := range records {
		// Only the indexed case's s1 carries six columns; its c0/c1/c4/c5
		// are Java non-null assertions rendered as booleans.
		if _, six := record.Fields["c5"]; !six {
			continue
		}
		for _, name := range []string{"c0", "c1", "c4", "c5"} {
			value, ok := record.Fields[name]
			if !ok {
				continue
			}
			if _, isBool := value.(bool); isBool {
				continue
			}
			if marker, isMarker := value.(map[string]any); isMarker {
				if marker["state"] == "null" || marker["state"] == "missing" {
					record.Fields[name] = false
					continue
				}
			}
			record.Fields[name] = true
		}
	}
	return records
}

// eigdIsNilValue reports whether a normalized field value is nil or a typed
// nil (nil pointer, slice, or map), all of which Java's row renderer tags.
func eigdIsNilValue(value any) bool {
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

// buildS1 projects the pinned s1 EPL. Java's exists(property?) reports true on
// a present-null value, so c0/c1 use the plain path (Go's '?' maps a terminal
// null to Missing); c2 keeps the '?' path since typeof() is null either way.
// Nested/deep paths carry '?' on every nullable segment because Java's
// property?.leaf.id makes the whole tail null-safe. The indexed case projects
// the raw values and the runner renders the Java non-null assertions.
func (it *eigdIteration) buildS1(caseName string) (esper.Plan, error) {
	eventValue := esper.EventValue[esper.Event]()
	prop := func(path string) esper.Expression[any] {
		return esper.Property[any](eventValue, path)
	}
	javaTypeName := func(path string) esper.Expression[string] {
		return esper.Func1[any, string]("eigdTypeName", func(value any) string {
			name := reflect.TypeOf(value).String()
			name = strings.TrimPrefix(name, "*")
			name = strings.TrimPrefix(name, "[]")
			if name == "string" {
				return "String"
			}
			return name
		}, prop(path))
	}
	stream := esper.FromAny(it.env, "LocalEvent")
	var query esper.Query
	switch caseName {
	case "dynamic-simple", "simple-predefined":
		query = stream.Select(
			esper.Alias("c0", prop("property")),
			esper.Alias("c1", esper.Exists(prop("property"))),
			esper.Alias("c2", javaTypeName("property?")),
		).Query(esper.StatementName("s1"))
	case "dynamic-nested":
		query = stream.Select(
			esper.Alias("c0", prop("property?.id")),
			esper.Alias("c1", esper.Exists(prop("property?.id"))),
			esper.Alias("c2", javaTypeName("property?.id")),
		).Query(esper.StatementName("s1"))
	case "dynamic-nested-deep":
		query = stream.Select(
			esper.Alias("c0", prop("property?.leaf?.id")),
			esper.Alias("c1", esper.Exists(prop("property?.leaf?.id"))),
			esper.Alias("c2", javaTypeName("property?.leaf?.id")),
		).Query(esper.StatementName("s1"))
	case "indexed-predefined":
		query = stream.Select(
			esper.Alias("c0", prop("array[0]?")),
			esper.Alias("c1", prop("array[1]?")),
			esper.Alias("c2", esper.Exists(prop("array[0]?"))),
			esper.Alias("c3", esper.Exists(prop("array[1]?"))),
			esper.Alias("c4", javaTypeName("array[0]?")),
			esper.Alias("c5", javaTypeName("array[1]?")),
		).Query(esper.StatementName("s1"))
	default:
		return esper.Plan{}, fmt.Errorf("%s: no s1 plan for case %q", eventInfraGetterDynamicID, caseName)
	}
	return it.env.Build(query)
}

// registerSchemas mirrors the pinned create-schema EPL for one underlying.
// Avro dynamic schemas are the unrepresentable boundary: Go rejects
// AllowDynamicFields on Avro, verified here before the marker step.
func (it *eigdIteration) registerSchemas(caseName, mode string) error {
	env := it.env
	stringT := reflect.TypeOf("")
	innerSliceT := reflect.TypeOf([]map[string]any{})
	reg := func(schema esper.Schema, err error) error {
		if err != nil {
			return fmt.Errorf("%s: register schema: %w", eventInfraGetterDynamicID, err)
		}
		it.schemas[schema.Name()] = schema
		return nil
	}
	switch caseName {
	case "dynamic-simple":
		switch mode {
		case "bean":
			base, err := esper.RegisterStruct[eigdSimpleBase](env, "LocalEvent")
			if err != nil {
				return err
			}
			it.schemas["LocalEvent"] = base
			sub, err := esper.RegisterStruct[eigdSimpleSubA](env, "LocalEventSubA", esper.WithSchemaParent(base))
			if err != nil {
				return err
			}
			it.schemas["LocalEventSubA"] = sub
			return nil
		case "map":
			return reg(esper.RegisterMap(env, "LocalEvent", nil, esper.AllowDynamicFields()))
		case "objectarray":
			base, err := esper.RegisterObjectArray(env, "LocalEvent", nil)
			if err != nil {
				return err
			}
			it.schemas["LocalEvent"] = base
			sub, err := esper.RegisterObjectArray(env, "LocalEventSubA",
				[]esper.FieldSpec{esper.OptionalFieldDef("property", stringT)}, esper.WithSchemaParent(base))
			if err != nil {
				return err
			}
			it.schemas["LocalEventSubA"] = sub
			return nil
		case "json":
			return reg(esper.RegisterJSON(env, "LocalEvent", nil, esper.AllowDynamicFields()))
		case "json-provided":
			return reg(esper.RegisterJSONFor[eigdSimpleJSONProvided](env, "LocalEvent", nil))
		case "avro":
			return it.verifyAvroDynamicBoundary(caseName)
		}
	case "dynamic-nested":
		switch mode {
		case "bean":
			base, err := esper.RegisterStruct[eigdNestedBase](env, "LocalEvent")
			if err != nil {
				return err
			}
			it.schemas["LocalEvent"] = base
			sub, err := esper.RegisterStruct[eigdNestedSubA](env, "LocalEventSubA", esper.WithSchemaParent(base))
			if err != nil {
				return err
			}
			it.schemas["LocalEventSubA"] = sub
			return nil
		case "map":
			return reg(esper.RegisterMap(env, "LocalEvent", nil, esper.AllowDynamicFields()))
		case "objectarray":
			return reg(esper.RegisterObjectArray(env, "LocalEvent", nil))
		case "json":
			return reg(esper.RegisterJSON(env, "LocalEvent", nil, esper.AllowDynamicFields()))
		case "json-provided":
			return reg(esper.RegisterJSONFor[eigdNestedJSONProvided](env, "LocalEvent", nil))
		case "avro":
			return it.verifyAvroDynamicBoundary(caseName)
		}
	case "dynamic-nested-deep":
		switch mode {
		case "bean":
			// The Java schema registers the EMPTY base class; the sent bean
			// carries the property only through the dynamic getter, so the Go
			// schema is the field-free eigdDeepSubA shape.
			return reg(esper.RegisterStruct[eigdDeepSubA](env, "LocalEvent"))
		case "map":
			return reg(esper.RegisterMap(env, "LocalEvent", nil, esper.AllowDynamicFields()))
		case "objectarray":
			return reg(esper.RegisterObjectArray(env, "LocalEvent", nil))
		case "json":
			return reg(esper.RegisterJSON(env, "LocalEvent", nil, esper.AllowDynamicFields()))
		case "json-provided":
			return reg(esper.RegisterJSONFor[eigdDeepJSONProvided](env, "LocalEvent", nil))
		case "avro":
			return it.verifyAvroDynamicBoundary(caseName)
		}
	case "indexed-predefined":
		switch mode {
		case "bean":
			inner, err := esper.RegisterStruct[eigdIndexedInner](env, "LocalInnerEvent")
			if err != nil {
				return err
			}
			it.schemas["LocalInnerEvent"] = inner
			base, err := esper.RegisterStruct[eigdIndexedBase](env, "LocalEvent")
			if err != nil {
				return err
			}
			it.schemas["LocalEvent"] = base
			sub, err := esper.RegisterStruct[eigdIndexedSubA](env, "LocalEventSubA",
				esper.WithSchemaParent(base), esper.WithNestedPropertySchema("array", inner))
			if err != nil {
				return err
			}
			it.schemas["LocalEventSubA"] = sub
			return nil
		case "map":
			if _, err := esper.RegisterMap(env, "LocalInnerEvent", nil); err != nil {
				return err
			}
			return reg(esper.RegisterMap(env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("array", innerSliceT)}))
		case "objectarray":
			return reg(esper.RegisterObjectArray(env, "LocalEvent", nil))
		case "json":
			if _, err := esper.RegisterJSON(env, "LocalInnerEvent", nil); err != nil {
				return err
			}
			return reg(esper.RegisterJSON(env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("array", innerSliceT)}))
		case "json-provided":
			return reg(esper.RegisterJSONFor[eigdIndexedJSONProvided](env, "LocalEvent", nil))
		case "avro":
			if _, err := esper.RegisterAvro(env, "LocalInnerEvent", nil); err != nil {
				return err
			}
			return reg(esper.RegisterAvro(env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("array", innerSliceT)}))
		}
	case "simple-predefined":
		switch mode {
		case "bean":
			return reg(esper.RegisterStruct[eigdPredefBean](env, "LocalEvent"))
		case "map":
			return reg(esper.RegisterMap(env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("property", stringT)}))
		case "objectarray":
			return reg(esper.RegisterObjectArray(env, "LocalEvent",
				[]esper.FieldSpec{esper.OptionalFieldDef("property", stringT)}))
		case "json":
			return reg(esper.RegisterJSON(env, "LocalEvent",
				[]esper.FieldSpec{esper.FieldDef("property", stringT)}))
		case "json-provided":
			return reg(esper.RegisterJSONFor[eigdPredefJSONProvided](env, "LocalEvent", nil))
		case "avro":
			return reg(esper.RegisterAvro(env, "LocalEvent",
				[]esper.FieldSpec{esper.OptionalFieldDef("property", stringT)}))
		}
	}
	return fmt.Errorf("%s: no schema registration for %q/%q", eventInfraGetterDynamicID, caseName, mode)
}

// verifyAvroDynamicBoundary proves the Go surface cannot express the Java
// Avro dynamic iteration: Avro schemas reject AllowDynamicFields, and a
// non-dynamic Avro record rejects the undeclared "property" field the Java
// sender writes. The iteration then continues as the unrepresentable marker.
func (it *eigdIteration) verifyAvroDynamicBoundary(caseName string) error {
	if _, err := esper.NewAvroSchema("LocalEvent", nil, esper.AllowDynamicFields()); err == nil {
		return fmt.Errorf("%s: Avro schema unexpectedly accepted AllowDynamicFields", eventInfraGetterDynamicID)
	}
	plain, err := esper.NewAvroSchema("LocalEvent", nil)
	if err != nil {
		return fmt.Errorf("%s: Avro schema without dynamic fields failed: %w", eventInfraGetterDynamicID, err)
	}
	if _, err := esper.NewAvroRecordFromMap(plain, map[string]any{"property": "a"}); err == nil {
		return fmt.Errorf("%s: Avro record unexpectedly accepted undeclared field", eventInfraGetterDynamicID)
	}
	it.avroDynamic = true
	return nil
}

// send decodes one pinned payload and dispatches it through the sender the
// Java iteration uses for the underlying mode.
func (it *eigdIteration) send(ctx context.Context, caseName string, step compat.Step) error {
	var payload map[string]json.RawMessage
	if err := strictObject(step.Payload, &payload); err != nil {
		return fmt.Errorf("%s: decode send payload: %w", eventInfraGetterDynamicID, err)
	}
	switch step.Mode {
	case "bean":
		return it.sendBean(ctx, caseName, step.EventType, payload)
	case "map":
		event, err := eigdDecodeMap(payload)
		if err != nil {
			return err
		}
		return it.engine.Send(ctx, step.EventType, event)
	case "objectarray":
		return it.sendObjectArray(ctx, caseName, step.EventType, payload)
	case "json", "json-provided":
		raw, err := json.Marshal(eigdRawToAny(payload))
		if err != nil {
			return err
		}
		return it.engine.SendJSON(ctx, step.EventType, raw)
	case "avro":
		return it.sendAvro(ctx, caseName, step.EventType, payload)
	default:
		return fmt.Errorf("%s: unsupported send mode %q", eventInfraGetterDynamicID, step.Mode)
	}
}

func (it *eigdIteration) sendBean(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	str := func(raw json.RawMessage) *string {
		if raw == nil {
			return nil
		}
		var value *string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil
		}
		return value
	}
	switch caseName {
	case "dynamic-simple":
		if eventType == "LocalEvent" {
			return it.engine.Send(ctx, eventType, eigdSimpleBase{})
		}
		return it.engine.Send(ctx, eventType, eigdSimpleSubA{Property: str(payload["property"])})
	case "dynamic-nested":
		if eventType == "LocalEvent" {
			return it.engine.Send(ctx, eventType, eigdNestedBase{})
		}
		var inner *eigdNestedInner
		if raw, ok := payload["property"]; ok {
			var nested map[string]json.RawMessage
			if err := json.Unmarshal(raw, &nested); err == nil && nested != nil {
				inner = &eigdNestedInner{ID: str(nested["id"])}
			}
		}
		return it.engine.Send(ctx, eventType, eigdNestedSubA{Property: inner})
	case "dynamic-nested-deep":
		// Java sends every deep bean under the "LocalEvent" name.
		var property *eigdDeepInner
		if raw, ok := payload["property"]; ok {
			var inner map[string]json.RawMessage
			if err := json.Unmarshal(raw, &inner); err == nil && inner != nil {
				property = &eigdDeepInner{}
				if leafRaw, ok := inner["leaf"]; ok {
					var leaf map[string]json.RawMessage
					if err := json.Unmarshal(leafRaw, &leaf); err == nil && leaf != nil {
						property.Leaf = &eigdDeepLeaf{ID: str(leaf["id"])}
					}
				}
			}
		}
		return it.engine.Send(ctx, eventType, eigdDeepSubA{Property: property})
	case "indexed-predefined":
		if eventType == "LocalEvent" {
			return it.engine.Send(ctx, eventType, eigdIndexedBase{})
		}
		// Populated payloads send LocalEventSubA (the declared child type).
		var array []eigdIndexedInner
		if raw, ok := payload["array"]; ok {
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err == nil && items != nil {
				array = make([]eigdIndexedInner, len(items))
			}
		}
		return it.engine.Send(ctx, eventType, eigdIndexedSubA{Array: array})
	case "simple-predefined":
		return it.engine.Send(ctx, eventType, eigdPredefBean{Property: str(payload["property"])})
	default:
		return fmt.Errorf("%s: no bean sender for case %q", eventInfraGetterDynamicID, caseName)
	}
}
func (it *eigdIteration) sendObjectArray(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	str := func(raw json.RawMessage) any {
		var decoded *string
		if err := json.Unmarshal(raw, &decoded); err != nil || decoded == nil {
			return nil
		}
		return *decoded
	}
	switch caseName {
	case "dynamic-simple":
		if eventType == "LocalEvent" {
			return it.engine.SendObjectArray(ctx, eventType, []any{})
		}
		return it.engine.SendObjectArray(ctx, eventType, []any{str(payload["property"])})
	case "simple-predefined":
		return it.engine.SendObjectArray(ctx, eventType, []any{str(payload["property"])})
	default:
		return fmt.Errorf("%s: no object-array sender for case %q", eventInfraGetterDynamicID, caseName)
	}
}

func (it *eigdIteration) sendAvro(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	schema, ok := it.schemas[eventType]
	if !ok {
		return fmt.Errorf("%s: no Avro schema %q", eventInfraGetterDynamicID, eventType)
	}
	switch caseName {
	case "indexed-predefined":
		array := []map[string]any{}
		if raw, exists := payload["array"]; exists {
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err == nil && items != nil {
				array = make([]map[string]any, len(items))
				for index := range items {
					array[index] = map[string]any{}
				}
			}
		}
		record, err := esper.NewAvroRecordFromMap(schema, map[string]any{"array": array})
		if err != nil {
			return err
		}
		return it.engine.SendAvro(ctx, eventType, record)
	case "simple-predefined":
		var value *string
		if raw, exists := payload["property"]; exists {
			if err := json.Unmarshal(raw, &value); err != nil {
				return err
			}
		}
		var stored any
		if value != nil {
			stored = *value
		}
		record, err := esper.NewAvroRecordFromMap(schema, map[string]any{"property": stored})
		if err != nil {
			return err
		}
		return it.engine.SendAvro(ctx, eventType, record)
	default:
		return fmt.Errorf("%s: no Avro sender for case %q", eventInfraGetterDynamicID, caseName)
	}
}

// emitGetterProbes mirrors the Java assertGetter/assertGetters block that runs
// on the last s0 event after each send: one record per probe path carrying
// exists (isExistsProperty), value (get) and fragment (getFragment != null).
// The simple cases probe the plain path because Java's property? reports
// exists on a present-null value (and on absent for bean-backed JSON/Avro).
func (it *eigdIteration) emitGetterProbes(trace *compat.Trace, caseName string) error {
	if it.lastS0 == nil {
		return fmt.Errorf("%s: no s0 event captured for getter probe", eventInfraGetterDynamicID)
	}
	event := *it.lastS0
	for _, javaPath := range eventInfraGetterDynamicProbes[caseName] {
		goPath := javaPath
		if caseName == "dynamic-simple" || caseName == "simple-predefined" {
			goPath = "property"
		}
		if caseName == "dynamic-nested-deep" {
			goPath = "property?.leaf?.id"
		}
		value := event.Get(goPath)
		exists := !value.IsMissing()
		recordValue := any(nil)
		if exists {
			recordValue = eigdNormalizeProbeValue(value)
		}
		fragment := false
		if caseName == "indexed-predefined" && exists {
			// Java getFragment is non-null only for bean-backed (bean or
			// provided-class JSON) indexed elements; Go exposes the fragment
			// list on the whole array property.
			fragments, ok := event.GetFragments("array")
			index := 0
			if javaPath == "array[1]?" {
				index = 1
			}
			fragment = ok && index < len(fragments)
		}
		it.sequences["s0:getter"]++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "getter",
			Statement: "s0",
			Sequence:  it.sequences["s0:getter"],
			Time:      compat.FormatTraceTime(it.engine.Now()),
			Name:      javaPath,
			Value: map[string]any{
				"exists":   exists,
				"value":    recordValue,
				"fragment": fragment,
			},
		})
	}
	return nil
}

// emitNullGetterProbe mirrors the Java sender==null branch: the event type's
// dynamic getter is null, verified against the Go schema surface before the
// record is emitted.
func (it *eigdIteration) emitNullGetterProbe(trace *compat.Trace, caseName string, step compat.Step) error {
	schema, ok := it.schemas["LocalEvent"]
	if !ok {
		return fmt.Errorf("%s: no LocalEvent schema for getter probe", eventInfraGetterDynamicID)
	}
	if _, found := schema.Getter(step.Name); found {
		return fmt.Errorf("%s: getter %q unexpectedly resolvable on %s", eventInfraGetterDynamicID, step.Name, schema.Name())
	}
	it.sequences["s0:getter"]++
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      caseName,
		Operation: "getter",
		Statement: "s0",
		Sequence:  it.sequences["s0:getter"],
		Time:      compat.FormatTraceTime(it.engine.Now()),
		Name:      step.Name,
		Value:     map[string]any{"state": "null"},
	})
	return nil
}

// emitUnrepresentable emits the pinned Avro-dynamic boundary marker after the
// deploy step verified the Go rejection.
func (it *eigdIteration) emitUnrepresentable(trace *compat.Trace, caseName string, step compat.Step) error {
	if !it.avroDynamic {
		return fmt.Errorf("%s: unrepresentable %q without the verified Avro boundary", eventInfraGetterDynamicID, step.Statement)
	}
	if step.ExpectError != eventInfraGetterDynamicAvroNotes[caseName] {
		return fmt.Errorf("%s: unrepresentable note is not pinned", eventInfraGetterDynamicID)
	}
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Sequence:  0,
		Value:     step.ExpectError,
	})
	return nil
}

func (it *eigdIteration) undeployAll(ctx context.Context) error {
	for index := len(it.deployOrder) - 1; index >= 0; index-- {
		label := it.deployOrder[index]
		deployment, ok := it.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", eventInfraGetterDynamicID, label, err)
		}
		delete(it.deployments, label)
	}
	it.deployOrder = nil
	return nil
}

// eigdNormalizeProbeValue renders a getter value like the Java oracle's
// normalize: null stays the tagged object, maps become plain objects and
// everything else passes through to the JSON encoder.
func eigdNormalizeProbeValue(value esper.Value) any {
	if value.IsNull() {
		return map[string]any{"state": "null"}
	}
	if !value.IsPresent() {
		return map[string]any{"state": "missing"}
	}
	return value.Any()
}

func eigdRawToAny(payload map[string]json.RawMessage) map[string]any {
	result := make(map[string]any, len(payload))
	for key, raw := range payload {
		var value any
		if err := json.Unmarshal(raw, &value); err == nil {
			result[key] = value
		}
	}
	return result
}

func eigdDecodeMap(payload map[string]json.RawMessage) (map[string]any, error) {
	result := make(map[string]any, len(payload))
	for key, raw := range payload {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("decode payload field %q: %w", key, err)
		}
		result[key] = value
	}
	return result, nil
}

// loadEventInfraGetterDynamicScenario decodes the scenario JSON with strict
// field checking and pins the metadata, case identities and step sequence.
func loadEventInfraGetterDynamicScenario(reader io.Reader) (compat.Scenario, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario must be a JSON object: %w", eventInfraGetterDynamicID, err)
	}
	if err := requireEventInfraGetterDynamicFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eventInfraGetterDynamicID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eventInfraGetterDynamicID ||
		metadata.Description != eventInfraGetterDynamicDescription ||
		metadata.JavaCommit != eventInfraGetterDynamicJavaCommit || metadata.JavaSource != eventInfraGetterDynamicSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", eventInfraGetterDynamicID)
	}
	if err := eventInfraGetterDynamicRequireEqual(metadata.JavaFlags, eventInfraGetterDynamicJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eventInfraGetterDynamicRequireEqual(metadata.JavaRuntimes, eventInfraGetterDynamicJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eventInfraGetterDynamicRequireEqual(metadata.JavaNames, eventInfraGetterDynamicJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eventInfraGetterDynamicRequireEqual(metadata.JavaStaticID, eventInfraGetterDynamicJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", eventInfraGetterDynamicID, err)
	}
	if len(rawCases) != len(eventInfraGetterDynamicCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", eventInfraGetterDynamicID, len(rawCases), len(eventInfraGetterDynamicCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEventInfraGetterDynamicFields(object,
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
		caseName := eventInfraGetterDynamicCases[index]
		if definition.Case != caseName || definition.Ordinal != 0 ||
			definition.RuntimeID != eventInfraGetterDynamicJavaRuntimeIDs[index] ||
			definition.ExecutionName != eventInfraGetterDynamicJavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d identity = %+v, want case %q ordinal 0 runtime %s execution %s",
				eventInfraGetterDynamicID, index, definition, caseName,
				eventInfraGetterDynamicJavaRuntimeIDs[index], eventInfraGetterDynamicJavaExecutions[index])
		}
		if definition.EPL != eventInfraGetterDynamicS1[caseName] {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source", caseName)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", eventInfraGetterDynamicID, err)
	}
	pinned := eventInfraGetterDynamicPinnedSteps()
	if len(rawSteps) != len(pinned) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d steps, want %d", eventInfraGetterDynamicID, len(rawSteps), len(pinned))
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
		if err := eventInfraGetterDynamicValidateStep(index, object, step); err != nil {
			return compat.Scenario{}, err
		}
		if key := eventInfraGetterDynamicStepKey(step); key != pinned[index] {
			return compat.Scenario{}, fmt.Errorf("scenario step %d = %q, want pinned %q", index, key, pinned[index])
		}
		steps[index] = step
	}
	return compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}, nil
}

// eventInfraGetterDynamicValidateStep enforces the per-op field set.
func eventInfraGetterDynamicValidateStep(index int, object map[string]json.RawMessage, step compat.Step) error {
	require := func(names ...string) error {
		if err := requireEventInfraGetterDynamicFields(object, names...); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		return nil
	}
	switch step.Op {
	case "case":
		return require("op", "case")
	case "deploy":
		if step.Statement == "schema" {
			if err := require("op", "case", "statement", "epl", "mode"); err != nil {
				return err
			}
			pinned, ok := eventInfraGetterDynamicSchemaEPL(step.Case, step.Mode)
			if !ok || step.Epl != pinned {
				return fmt.Errorf("scenario step %d: schema EPL is not pinned", index)
			}
			return nil
		}
		if err := require("op", "case", "statement", "epl"); err != nil {
			return err
		}
		if step.Statement == "s0" && step.Epl != eventInfraGetterDynamicS0 {
			return fmt.Errorf("scenario step %d: s0 EPL is not pinned", index)
		}
		if step.Statement == "s1" && step.Epl != eventInfraGetterDynamicS1[step.Case] {
			return fmt.Errorf("scenario step %d: s1 EPL is not pinned", index)
		}
		return nil
	case "deployed":
		return require("op", "case", "statement")
	case "send":
		if err := require("op", "case", "eventType", "mode", "payload"); err != nil {
			return err
		}
		return nil
	case "types":
		return require("op", "case", "statement", "name")
	case "unrepresentable":
		return require("op", "case", "statement", "expectError")
	case "undeploy-all":
		return require("op", "case")
	default:
		return fmt.Errorf("scenario step %d: unsupported op %q", index, step.Op)
	}
}

// eventInfraGetterDynamicStepKey renders one step in canonical form for the
// pinned sequence comparison.
func eventInfraGetterDynamicStepKey(step compat.Step) string {
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

// eventInfraGetterDynamicPinnedSteps rebuilds the pinned step sequence from
// the same tables the scenario JSON was generated from.
func eventInfraGetterDynamicPinnedSteps() []string {
	var keys []string
	for _, caseName := range eventInfraGetterDynamicCases {
		keys = append(keys, "case|"+caseName+"|||||||")
		for _, mode := range eventInfraGetterDynamicModes {
			epl, _ := eventInfraGetterDynamicSchemaEPL(caseName, mode)
			keys = append(keys, strings.Join([]string{"deploy", caseName, "schema", "", mode, "", epl, "", ""}, "|"))
			keys = append(keys, "deployed|"+caseName+"|schema||||||")
			if mode == "avro" {
				if note, ok := eventInfraGetterDynamicAvroNotes[caseName]; ok {
					keys = append(keys, strings.Join([]string{"unrepresentable", caseName, "avro-dynamic", "", "", "", "", note, ""}, "|"))
					keys = append(keys, "undeploy-all|"+caseName+"|||||||")
					continue
				}
			}
			keys = append(keys, strings.Join([]string{"deploy", caseName, "s0", "", "", "", eventInfraGetterDynamicS0, "", ""}, "|"))
			keys = append(keys, "deployed|"+caseName+"|s0||||||")
			if !eventInfraGetterDynamicHasSender(caseName, mode) {
				for _, path := range eventInfraGetterDynamicProbes[caseName] {
					keys = append(keys, strings.Join([]string{"types", caseName, "s0", "", "", path, "", "", ""}, "|"))
				}
				keys = append(keys, "undeploy-all|"+caseName+"|||||||")
				continue
			}
			keys = append(keys, strings.Join([]string{"deploy", caseName, "s1", "", "", "", eventInfraGetterDynamicS1[caseName], "", ""}, "|"))
			keys = append(keys, "deployed|"+caseName+"|s1||||||")
			for _, payload := range eventInfraGetterDynamicPayloads[caseName] {
				eventType := eventInfraGetterDynamicSendType(caseName, mode, payload)
				keys = append(keys, strings.Join([]string{"send", caseName, "", eventType, mode, "", "", "", payload}, "|"))
			}
			keys = append(keys, "undeploy-all|"+caseName+"|||||||")
		}
	}
	return keys
}

func eventInfraGetterDynamicHasSender(caseName, mode string) bool {
	if mode == "objectarray" && (caseName == "dynamic-nested" || caseName == "dynamic-nested-deep" || caseName == "indexed-predefined") {
		return false
	}
	if mode == "avro" && (caseName == "dynamic-simple" || caseName == "dynamic-nested" || caseName == "dynamic-nested-deep") {
		return false
	}
	return true
}

func eventInfraGetterDynamicSendType(caseName, mode, payload string) string {
	if mode == "bean" {
		if caseName == "dynamic-nested-deep" {
			return "LocalEvent"
		}
		if caseName == "dynamic-simple" || caseName == "dynamic-nested" || caseName == "indexed-predefined" {
			if payload == `{}` {
				return "LocalEvent"
			}
			return "LocalEventSubA"
		}
		return "LocalEvent"
	}
	if mode == "objectarray" && caseName == "dynamic-simple" {
		if payload == `{}` {
			return "LocalEvent"
		}
		return "LocalEventSubA"
	}
	return "LocalEvent"
}

func requireEventInfraGetterDynamicFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("expected fields %v, got %v", names, keysOfEigd(object))
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("missing field %q", name)
		}
	}
	return nil
}

func keysOfEigd(object map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	return keys
}

func eventInfraGetterDynamicRequireEqual(actual, expected []string, name string) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("%s %s length = %d, want %d", eventInfraGetterDynamicID, name, len(actual), len(expected))
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf("%s %s[%d] = %q, want %q", eventInfraGetterDynamicID, name, index, actual[index], expected[index])
		}
	}
	return nil
}
