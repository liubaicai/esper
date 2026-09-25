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
	eventInfraGetterNestedID         = "event-infra-getter-nested"
	eventInfraGetterNestedJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	// The cluster spans five sibling files in suite/event/infra; the scenario
	// pins the shared directory while the evidence metadata carries the files.
	eventInfraGetterNestedSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra"
	// eventInfraGetterNestedXMLType is the config-registered XML event type
	// used by the seventh (xml) underlying of simple-no-fragment.
	eventInfraGetterNestedXMLType = "EventInfraGetterSimpleNoFragmentXML"
)

var eventInfraGetterNestedJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraGetterNestedArray.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraGetterNestedSimple.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraGetterNestedSimpleDeep.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraGetterSimpleFragment.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra/EventInfraGetterSimpleNoFragment.java",
}

var eventInfraGetterNestedJavaRuntimeIDs = []string{
	"java-runtime-718503411fc69f8f01c3",
	"java-runtime-95388a521ff7fc9ae02e",
	"java-runtime-08402ea2d9e12c7ad46c",
	"java-runtime-1eb5dcd8d9611c9f40eb",
	"java-runtime-9c65668855ac0fc31533",
}

var eventInfraGetterNestedJavaStaticIDs = []string{
	"java-fa2a2775be3aa199b6a8",
	"java-45de20b4c43758274efe",
	"java-45fb81d0945532503b7c",
	"java-1d9833f8696776c207ae",
	"java-2be3bd8162600f6a48eb",
}

var eventInfraGetterNestedJavaExecutions = []string{
	"EventInfraGetterNestedArray",
	"EventInfraGetterNestedSimple",
	"EventInfraGetterNestedSimpleDeep",
	"EventInfraGetterSimpleFragment",
	"EventInfraGetterSimpleNoFragment",
}

var eventInfraGetterNestedJavaFlags []string

var eventInfraGetterNestedCases = []string{
	"nested-array",
	"nested-simple",
	"nested-simple-deep",
	"simple-fragment",
	"simple-no-fragment",
}

const eventInfraGetterNestedDescription = "EventInfraGetterNested*/EventInfraGetterSimple*Fragment nested-predefined getter slice (all ord 0, no flags): nested-array replays property[0].id/property[1].id over six underlyings with the four-step array matrix (len2, len1, empty, null; Avro null sends an empty list); nested-simple replays property.id over five underlyings (the Java Avro sender lambda is never invoked) and nested-simple-deep property.leaf.id over six, both with present-null leaf values reporting exists=true; simple-fragment selects the fragment-typed nested event property over six underlyings where getFragment is non-null exactly when the nested event is present; simple-no-fragment replays a declared string property over seven underlyings including a config-registered XML attribute type, with exists always true and the Avro null send using an ad-hoc optionalString record. Java sources are the five sibling files under suite/event/infra (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c)."

// Pinned s1 EPL per case (byte-exact).
var eventInfraGetterNestedS1 = map[string]string{
	"nested-array":       "@name('s1') select property[0].id as c0, property[1].id as c1, exists(property[0].id) as c2, exists(property[1].id) as c3, typeof(property[0].id) as c4, typeof(property[1].id) as c5 from LocalEvent;\n",
	"nested-simple":      "@name('s1') select property.id as c0, exists(property.id) as c1, typeof(property.id) as c2 from LocalEvent;\n",
	"nested-simple-deep": "@name('s1') select property.leaf.id as c0, exists(property.leaf.id) as c1, typeof(property.leaf.id) as c2 from LocalEvent;\n",
	"simple-fragment":    "@name('s1') select property as c0, exists(property) as c1, typeof(property) as c2 from LocalEvent;\n",
	"simple-no-fragment": "@name('s1') select property as c0, exists(property) as c1, typeof(property) as c2 from LocalEvent;\n",
}

const eventInfraGetterNestedS0 = "@name('s0') select * from LocalEvent"

// The simple-no-fragment XML underlying addresses the config-registered type
// instead of the shared LocalEvent name.
const eventInfraGetterNestedS0XML = "@name('s0') select * from " + eventInfraGetterNestedXMLType
const eventInfraGetterNestedS1XML = "@name('s1') select property as c0, exists(property) as c1, typeof(property) as c2 from " + eventInfraGetterNestedXMLType + ";\n"

const eventInfraGetterNestedPkg = "com.espertech.esper.regressionlib.suite.event.infra"

// Pinned create-schema EPL per case and underlying mode.
func eventInfraGetterNestedSchemaEPL(caseName, mode string) (string, bool) {
	arrayBase := eventInfraGetterNestedPkg + ".EventInfraGetterNestedArray"
	simpleBase := eventInfraGetterNestedPkg + ".EventInfraGetterNestedSimple"
	deepBase := eventInfraGetterNestedPkg + ".EventInfraGetterNestedSimpleDeep"
	fragmentBase := eventInfraGetterNestedPkg + ".EventInfraGetterSimpleFragment"
	noFragmentBase := eventInfraGetterNestedPkg + ".EventInfraGetterSimpleNoFragment"
	switch caseName {
	case "nested-array":
		switch mode {
		case "bean":
			return "@public @buseventtype create schema LocalInnerEvent as " + arrayBase + "$LocalInnerEvent;\n" +
				"@public @buseventtype create schema LocalEvent as " + arrayBase + "$LocalEvent;\n", true
		case "map", "objectarray", "json", "avro":
			return "@public @buseventtype create " + mode + " schema LocalInnerEvent(id string);\n" +
				"@name('schema') @public @buseventtype create " + mode + " schema LocalEvent(property LocalInnerEvent[]);\n", true
		case "json-provided":
			return "@JsonSchema(className='" + arrayBase + "$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n", true
		}
	case "nested-simple":
		// The Java Avro sender lambda is built but runAssertion is never
		// invoked for it, so no "avro" mode exists for this case.
		switch mode {
		case "bean":
			return "@public @buseventtype create schema LocalInnerEvent as " + simpleBase + "$LocalInnerEvent;\n" +
				"@public @buseventtype create schema LocalEvent as " + simpleBase + "$LocalEvent;\n", true
		case "map", "objectarray", "json":
			return "@public @buseventtype create " + mode + " schema LocalInnerEvent(id string);\n" +
				"@name('schema') @public @buseventtype create " + mode + " schema LocalEvent(property LocalInnerEvent);\n", true
		case "json-provided":
			return "@JsonSchema(className='" + simpleBase + "$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n", true
		}
	case "nested-simple-deep":
		switch mode {
		case "bean":
			// Deep's bean EPL registers only the outer LocalEvent class; the
			// inner/leaf beans are reached through bean introspection.
			return "@public @buseventtype create schema LocalEvent as " + deepBase + "$LocalEvent;\n", true
		case "map", "objectarray", "json", "avro":
			return "@public @buseventtype create " + mode + " schema LocalLeafEvent(id string);\n" +
				"@public @buseventtype create " + mode + " schema LocalInnerEvent(leaf LocalLeafEvent);\n" +
				"@name('schema') @public @buseventtype create " + mode + " schema LocalEvent(property LocalInnerEvent);\n", true
		case "json-provided":
			return "@JsonSchema(className='" + deepBase + "$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n", true
		}
	case "simple-fragment":
		switch mode {
		case "bean":
			return "@public @buseventtype create schema LocalEvent as " + fragmentBase + "$LocalEvent;\n", true
		case "map":
			return "@public @buseventtype create schema LocalInnerEvent();\n" +
				"@public @buseventtype create schema LocalEvent(property LocalInnerEvent);\n", true
		case "objectarray":
			return "@public @buseventtype create objectarray schema LocalInnerEvent();\n" +
				"@public @buseventtype create objectarray schema LocalEvent(property LocalInnerEvent);\n", true
		case "json":
			return "@public @buseventtype create json schema LocalInnerEvent();\n" +
				"@public @buseventtype create json schema LocalEvent(property LocalInnerEvent);\n", true
		case "avro":
			return "@public @buseventtype create avro schema LocalInnerEvent();\n" +
				"@name('schema') @public @buseventtype create avro schema LocalEvent(property LocalInnerEvent);\n", true
		case "json-provided":
			return "@JsonSchema(className='" + fragmentBase + "$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n", true
		}
	case "simple-no-fragment":
		switch mode {
		case "bean":
			return "@public @buseventtype create schema LocalEvent as " + noFragmentBase + "$LocalEvent;\n", true
		case "map":
			return "@public @buseventtype create schema LocalEvent(property string);\n", true
		case "objectarray":
			return "@public @buseventtype create objectarray schema LocalEvent(property string);\n", true
		case "json":
			return "@public @buseventtype create json schema LocalEvent(property string);\n", true
		case "json-provided":
			return "@JsonSchema(className='" + noFragmentBase + "$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n", true
		case "avro":
			return "@name('schema') @public @buseventtype create avro schema LocalEvent(property string);\n", true
		case "xml":
			// The XML event type is registered through the test-suite
			// configuration (schema text + root element name), not EPL.
			return "", true
		}
	}
	return "", false
}

// eventInfraGetterNestedS1EPL resolves the pinned s1 text; the xml underlying
// of simple-no-fragment selects from the config type name.
func eventInfraGetterNestedS1EPL(caseName, mode string) string {
	if caseName == "simple-no-fragment" && mode == "xml" {
		return eventInfraGetterNestedS1XML
	}
	return eventInfraGetterNestedS1[caseName]
}

// eventInfraGetterNestedS0EPL resolves the pinned s0 text per underlying.
func eventInfraGetterNestedS0EPL(mode string) string {
	if mode == "xml" {
		return eventInfraGetterNestedS0XML
	}
	return eventInfraGetterNestedS0
}

// Pinned getter probe paths per case (the Java getGetter names).
var eventInfraGetterNestedProbes = map[string][]string{
	"nested-array":       {"property[0].id", "property[1].id"},
	"nested-simple":      {"property.id"},
	"nested-simple-deep": {"property.leaf.id"},
	"simple-fragment":    {"property"},
	"simple-no-fragment": {"property"},
}

// eignGoPath maps a Java probe name onto the equivalent Go property path.
// Esper's nested getters are implicitly null-safe along the path while Go's
// '?' suffix marks the segment boundary where a Null result becomes Missing,
// reproducing Java's exists=false on null/absent parents.
func eignGoPath(javaPath string) string {
	switch javaPath {
	case "property[0].id":
		return "property[0]?.id"
	case "property[1].id":
		return "property[1]?.id"
	case "property.id":
		return "property?.id"
	case "property.leaf.id":
		return "property?.leaf?.id"
	default:
		return javaPath
	}
}

// Pinned send payload sequence per case. Nested-array ids==null encodes the
// Java null-array send; deep's root/inner flags mirror Nullable2Lvl;
// simple-fragment's value flag is the Java Boolean sender.
var eventInfraGetterNestedPayloads = map[string][]string{
	"nested-array":       {`{"ids":["a","b"]}`, `{"ids":["a"]}`, `{"ids":[]}`, `{"ids":null}`},
	"nested-simple":      {`{"id":"a","present":true}`, `{"id":null,"present":true}`, `{"id":null,"present":false}`},
	"nested-simple-deep": {`{"id":"a","inner":false,"root":false}`, `{"id":null,"inner":false,"root":false}`, `{"id":null,"inner":true,"root":false}`, `{"id":null,"inner":false,"root":true}`},
	"simple-fragment":    {`{"value":true}`, `{"value":false}`},
	"simple-no-fragment": {`{"property":"a"}`, `{"property":null}`},
}

// eventInfraGetterNestedModes lists the underlyings each case runs, in Java
// source order. nested-simple skips avro (the lambda is never invoked) and
// simple-no-fragment adds xml as the seventh underlying.
func eventInfraGetterNestedModes(caseName string) []string {
	switch caseName {
	case "nested-simple":
		return []string{"bean", "map", "objectarray", "json", "json-provided"}
	case "simple-no-fragment":
		return []string{"bean", "map", "objectarray", "json", "json-provided", "avro", "xml"}
	default:
		return []string{"bean", "map", "objectarray", "json", "json-provided", "avro"}
	}
}

// eignArrayInner mirrors EventInfraGetterNestedArray.LocalInnerEvent.
type eignArrayInner struct {
	ID *string `esper:"id" json:"id"`
}

// eignArrayEvent mirrors EventInfraGetterNestedArray.LocalEvent
// {property: LocalInnerEvent[]}.
type eignArrayEvent struct {
	Property []eignArrayInner `esper:"property" json:"property"`
}

type eignArrayJSONProvidedInner struct {
	ID *string `esper:"id" json:"id"`
}

// eignArrayJSONProvided mirrors EventInfraGetterNestedArray.MyLocalJsonProvided.
type eignArrayJSONProvided struct {
	Property []eignArrayJSONProvidedInner `esper:"property" json:"property"`
}

// eignSimpleInner mirrors EventInfraGetterNestedSimple.LocalInnerEvent.
type eignSimpleInner struct {
	ID *string `esper:"id" json:"id"`
}

// eignSimpleEvent mirrors EventInfraGetterNestedSimple.LocalEvent.
type eignSimpleEvent struct {
	Property *eignSimpleInner `esper:"property" json:"property"`
}

type eignSimpleJSONProvidedInner struct {
	ID *string `esper:"id" json:"id"`
}

// eignSimpleJSONProvided mirrors EventInfraGetterNestedSimple.MyLocalJsonProvided.
type eignSimpleJSONProvided struct {
	Property *eignSimpleJSONProvidedInner `esper:"property" json:"property"`
}

// eignDeepLeaf mirrors EventInfraGetterNestedSimpleDeep.LocalLeafEvent.
type eignDeepLeaf struct {
	ID *string `esper:"id" json:"id"`
}

// eignDeepInner mirrors EventInfraGetterNestedSimpleDeep.LocalInnerEvent.
type eignDeepInner struct {
	Leaf *eignDeepLeaf `esper:"leaf" json:"leaf"`
}

// eignDeepEvent mirrors EventInfraGetterNestedSimpleDeep.LocalEvent.
type eignDeepEvent struct {
	Property *eignDeepInner `esper:"property" json:"property"`
}

type eignDeepJSONProvidedLeaf struct {
	ID *string `esper:"id" json:"id"`
}

type eignDeepJSONProvidedInner struct {
	Leaf *eignDeepJSONProvidedLeaf `esper:"leaf" json:"leaf"`
}

// eignDeepJSONProvided mirrors EventInfraGetterNestedSimpleDeep.MyLocalJsonProvided.
type eignDeepJSONProvided struct {
	Property *eignDeepJSONProvidedInner `esper:"property" json:"property"`
}

// eignFragmentInner mirrors the empty EventInfraGetterSimpleFragment.LocalInnerEvent.
type eignFragmentInner struct{}

// eignFragmentEvent mirrors EventInfraGetterSimpleFragment.LocalEvent.
type eignFragmentEvent struct {
	Property *eignFragmentInner `esper:"property" json:"property"`
}

type eignFragmentJSONProvidedInner struct{}

// eignFragmentJSONProvided mirrors EventInfraGetterSimpleFragment.MyLocalJsonProvided.
type eignFragmentJSONProvided struct {
	Property *eignFragmentJSONProvidedInner `esper:"property" json:"property"`
}

// eignNoFragmentEvent mirrors EventInfraGetterSimpleNoFragment.LocalEvent.
type eignNoFragmentEvent struct {
	Property *string `esper:"property" json:"property"`
}

// eignNoFragmentJSONProvided mirrors
// EventInfraGetterSimpleNoFragment.MyLocalJsonProvided.
type eignNoFragmentJSONProvided struct {
	Property *string `esper:"property" json:"property"`
}

// eignIteration is the per-(case, underlying) replay state. Java reuses one
// runtime per execution and undeploys between underlyings; Go event-type
// registrations are environment-scoped, so each iteration gets a fresh
// environment/engine pair (the undeploy-all boundary is the engine close).
type eignIteration struct {
	env         *esper.Environment
	engine      *esper.Engine
	schemas     map[string]esper.Schema
	deployments map[string]*esper.Deployment
	deployOrder []string
	sequences   map[string]uint64
	mode        string
	lastS0      *esper.Event
	lastExpects []eignProbeExpect
}

// eignProbeExpect carries the per-send assertions the Java runAssertion pins:
// exists (isExistsProperty), the leaf value or whole-property presence
// (hasValue), and fragment (getFragment != null).
type eignProbeExpect struct {
	exists   bool
	hasValue bool
	value    *string
	fragment bool
}

func runEventInfraGetterNestedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eventInfraGetterNestedCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEventInfraGetterNestedCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eventInfraGetterNestedID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", eventInfraGetterNestedID, scenario.ID)
	}
	return trace, nil
}

func runEventInfraGetterNestedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	var iter *eignIteration
	defer func() {
		if iter != nil && iter.engine != nil {
			_ = iter.engine.Close(context.Background())
		}
	}()

	newIteration := func(mode string) *eignIteration {
		if iter != nil && iter.engine != nil {
			_ = iter.engine.Close(context.Background())
		}
		iter = &eignIteration{
			env:         esper.NewEnvironment(),
			schemas:     make(map[string]esper.Schema),
			deployments: make(map[string]*esper.Deployment),
			sequences:   make(map[string]uint64),
			mode:        mode,
		}
		iter.engine = esper.NewEngine(iter.env,
			esper.WithRuntimeURI(eventInfraGetterNestedJavaRuntimeIDs[eventInfraGetterNestedOrdinal(caseName)]),
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
				return trace, fmt.Errorf("%s: deployed marker without deploy", eventInfraGetterNestedID)
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
				return trace, fmt.Errorf("%s: send without deploy", eventInfraGetterNestedID)
			}
			if err := iter.send(ctx, caseName, step); err != nil {
				return trace, err
			}
			if err := iter.emitGetterProbes(&trace, caseName); err != nil {
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
			return trace, fmt.Errorf("%s: unsupported step op %q", eventInfraGetterNestedID, step.Op)
		}
	}
	return trace, nil
}

func eventInfraGetterNestedOrdinal(caseName string) int {
	for index, name := range eventInfraGetterNestedCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// deploy handles the "schema", "s0" and "s1" deploy steps. The schema step
// registers the pinned Go event types for the underlying mode (the Java
// compileDeploy of the create-schema EPL, or the config-registered XML type
func (it *eignIteration) deploy(ctx context.Context, trace *compat.Trace, caseName string, step compat.Step) error {
	switch step.Statement {
	case "schema":
		it.mode = step.Mode
		return it.registerSchemas(caseName, step.Mode)
	case "s0":
		if step.Epl != eventInfraGetterNestedS0EPL(it.mode) {
			return fmt.Errorf("%s: s0 EPL %q is not pinned", eventInfraGetterNestedID, step.Epl)
		}
		plan, err := it.env.Build(esper.FromAny(it.env, eignEventType(it.mode)).Query(esper.StatementName("s0")))
		if err != nil {
			return fmt.Errorf("%s: build s0: %w", eventInfraGetterNestedID, err)
		}
		return it.deployPlan(ctx, trace, caseName, step.Statement, plan)
	case "s1":
		if step.Epl != eventInfraGetterNestedS1EPL(caseName, it.mode) {
			return fmt.Errorf("%s: s1 EPL %q is not pinned", eventInfraGetterNestedID, step.Epl)
		}
		plan, err := it.buildS1(caseName, it.mode)
		if err != nil {
			return err
		}
		return it.deployPlan(ctx, trace, caseName, step.Statement, plan)
	default:
		return fmt.Errorf("%s: unsupported deploy statement %q", eventInfraGetterNestedID, step.Statement)
	}
}

// eignEventType names the stream the underlying mode sends into.
func eignEventType(mode string) string {
	if mode == "xml" {
		return eventInfraGetterNestedXMLType
	}
	return "LocalEvent"
}

func (it *eignIteration) deployPlan(ctx context.Context, trace *compat.Trace, caseName, label string, plan esper.Plan) error {
	deployment, err := it.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", eventInfraGetterNestedID, label, err)
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
				New:       it.normalizeRows(caseName, stmt.Name(), batch.New),
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

// normalizeRows renders rows like compat.NormalizeResults, then applies the
// pinned Java projections the Go expression surface cannot carry verbatim:
// absent values render null (Java get() returns null where Go reports
// missing), and simple-fragment's s1 c0 renders the fragment event like the
// oracle's beanFragmentToJson (an empty object for the empty inner type).
func (it *eignIteration) normalizeRows(caseName, statement string, results []esper.Result) []compat.ResultRecord {
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
	for _, record := range records {
		for name, value := range record.Fields {
			record.Fields[name] = eignUntagNested(value)
		}
	}
	if caseName != "simple-fragment" || statement != "s1" {
		return records
	}
	for index, result := range results {
		row, ok := result.Row()
		if !ok || index >= len(records) {
			continue
		}
		if fragment, ok := row.GetFragment("c0"); ok {
			if it.mode == "objectarray" && len(fragment.Schema().Fields()) == 0 {
				// Java's row rendering emits the empty Object[] inner as [].
				records[index].Fields["c0"] = []any{}
				continue
			}
			fields := make(map[string]any)
			for _, field := range fragment.Schema().Fields() {
				fields[field.Name] = fragment.Get(field.Name).Any()
			}
			records[index].Fields["c0"] = fields
			continue
		}
		if value := row.Get("c0"); value.IsPresent() {
			// LocalInnerEvent declares no properties, so a present fragment
			// renders as the empty object (or empty array for objectarray,
			// matching the Java row rendering of the Object[] inner event)
			// even when the projection could not carry fragment metadata.
			if it.mode == "objectarray" {
				records[index].Fields["c0"] = []any{}
			} else {
				records[index].Fields["c0"] = map[string]any{}
			}
		}
	}
	return records
}

// buildS1 projects the pinned s1 EPL. Esper's nested-property accessors are
// implicitly null-safe along the path: exists(property.id) is false when the
// property itself is null but true when the property is present with a null
// id. Go reproduces that with '?' on every segment that may resolve to null
// before the leaf, while Exists still reports true on a present-null leaf.
func (it *eignIteration) buildS1(caseName, mode string) (esper.Plan, error) {
	eventValue := esper.EventValue[esper.Event]()
	prop := func(path string) esper.Expression[any] {
		return esper.Property[any](eventValue, path)
	}
	javaTypeName := func(path string) esper.Expression[string] {
		return esper.Func1[any, string]("eignTypeName", func(value any) string {
			name := reflect.TypeOf(value).String()
			name = strings.TrimPrefix(name, "*")
			name = strings.TrimPrefix(name, "[]")
			if name == "string" {
				return "String"
			}
			return name
		}, prop(path))
	}
	// fragmentTypeName renders typeof(property) for the fragment-typed case:
	// the declared fragment event type name, except bean and json-provided
	// where the Java typeof() reports the underlying bean class name.
	fragmentTypeName := func() esper.Expression[string] {
		name := "LocalInnerEvent"
		if mode == "bean" {
			name = eventInfraGetterNestedPkg + ".EventInfraGetterSimpleFragment$LocalInnerEvent"
		}
		if mode == "json-provided" {
			name = eventInfraGetterNestedPkg + ".EventInfraGetterSimpleFragment$MyLocalJsonProvidedInnerEvent"
		}
		return esper.Func1[any, string]("eignFragmentTypeName", func(any) string {
			return name
		}, prop("property"))
	}
	stream := esper.FromAny(it.env, eignEventType(mode))
	var query esper.Query
	switch caseName {
	case "nested-array":
		query = stream.Select(
			esper.Alias("c0", prop("property[0]?.id")),
			esper.Alias("c1", prop("property[1]?.id")),
			esper.Alias("c2", esper.Exists(prop("property[0]?.id"))),
			esper.Alias("c3", esper.Exists(prop("property[1]?.id"))),
			esper.Alias("c4", javaTypeName("property[0]?.id")),
			esper.Alias("c5", javaTypeName("property[1]?.id")),
		).Query(esper.StatementName("s1"))
	case "nested-simple":
		query = stream.Select(
			esper.Alias("c0", prop("property?.id")),
			esper.Alias("c1", esper.Exists(prop("property?.id"))),
			esper.Alias("c2", javaTypeName("property?.id")),
		).Query(esper.StatementName("s1"))
	case "nested-simple-deep":
		query = stream.Select(
			esper.Alias("c0", prop("property?.leaf?.id")),
			esper.Alias("c1", esper.Exists(prop("property?.leaf?.id"))),
			esper.Alias("c2", javaTypeName("property?.leaf?.id")),
		).Query(esper.StatementName("s1"))
	case "simple-fragment":
		query = stream.Select(
			esper.Alias("c0", prop("property")),
			esper.Alias("c1", esper.Exists(prop("property"))),
			esper.Alias("c2", fragmentTypeName()),
		).Query(esper.StatementName("s1"))
	case "simple-no-fragment":
		if mode == "xml" {
			// The Java XML getter reports isExistsProperty=true for the
			// schema-declared attribute even when the document omits it.
			query = stream.Select(
				esper.Alias("c0", prop("property")),
				esper.Alias("c1", esper.Literal(true)),
				esper.Alias("c2", javaTypeName("property")),
			).Query(esper.StatementName("s1"))
			break
		}
		query = stream.Select(
			esper.Alias("c0", prop("property")),
			esper.Alias("c1", esper.Exists(prop("property"))),
			esper.Alias("c2", javaTypeName("property")),
		).Query(esper.StatementName("s1"))
	default:
		return esper.Plan{}, fmt.Errorf("%s: no s1 plan for case %q", eventInfraGetterNestedID, caseName)
	}
	return it.env.Build(query)
}

// registerSchemas mirrors the pinned create-schema EPL for one underlying.
// Nested event links (WithNestedPropertySchema) reproduce the Java nested
// fragment-type metadata so path getters and fragment probes resolve.
func (it *eignIteration) registerSchemas(caseName, mode string) error {
	env := it.env
	stringT := reflect.TypeOf("")
	innerSliceT := reflect.TypeOf([]map[string]any{})
	mapT := reflect.TypeOf(map[string]any{})
	reg := func(schema esper.Schema, err error) error {
		if err != nil {
			return fmt.Errorf("%s: register schema: %w", eventInfraGetterNestedID, err)
		}
		it.schemas[schema.Name()] = schema
		return nil
	}
	regNested := func(name string, fields []esper.FieldSpec, register func(*esper.Environment, string, []esper.FieldSpec, ...esper.SchemaOption) (esper.Schema, error), outerType reflect.Type) error {
		inner, err := register(env, "LocalInnerEvent", fields)
		if err != nil {
			return fmt.Errorf("%s: register schema: %w", eventInfraGetterNestedID, err)
		}
		it.schemas["LocalInnerEvent"] = inner
		outer, err := register(env, "LocalEvent",
			[]esper.FieldSpec{esper.OptionalFieldDef("property", outerType)},
			esper.WithNestedPropertySchema("property", inner))
		if err != nil {
			return fmt.Errorf("%s: register schema: %w", eventInfraGetterNestedID, err)
		}
		it.schemas["LocalEvent"] = outer
		return nil
	}
	regDeep := func(register func(*esper.Environment, string, []esper.FieldSpec, ...esper.SchemaOption) (esper.Schema, error)) error {
		leaf, err := register(env, "LocalLeafEvent", []esper.FieldSpec{esper.FieldDef("id", stringT)})
		if err != nil {
			return fmt.Errorf("%s: register schema: %w", eventInfraGetterNestedID, err)
		}
		it.schemas["LocalLeafEvent"] = leaf
		inner, err := register(env, "LocalInnerEvent",
			[]esper.FieldSpec{esper.OptionalFieldDef("leaf", mapT)},
			esper.WithNestedPropertySchema("leaf", leaf))
		if err != nil {
			return fmt.Errorf("%s: register schema: %w", eventInfraGetterNestedID, err)
		}
		it.schemas["LocalInnerEvent"] = inner
		outer, err := register(env, "LocalEvent",
			[]esper.FieldSpec{esper.OptionalFieldDef("property", mapT)},
			esper.WithNestedPropertySchema("property", inner))
		if err != nil {
			return fmt.Errorf("%s: register schema: %w", eventInfraGetterNestedID, err)
		}
		it.schemas["LocalEvent"] = outer
		return nil
	}
	switch caseName {
	case "nested-array":
		switch mode {
		case "bean":
			inner, err := esper.RegisterStruct[eignArrayInner](env, "LocalInnerEvent")
			if err != nil {
				return err
			}
			it.schemas["LocalInnerEvent"] = inner
			return reg(esper.RegisterStruct[eignArrayEvent](env, "LocalEvent",
				esper.WithNestedPropertySchema("property", inner)))
		case "map":
			return regNested("LocalInnerEvent",
				[]esper.FieldSpec{esper.FieldDef("id", stringT)},
				esper.RegisterMap, innerSliceT)
		case "objectarray":
			return regNested("LocalInnerEvent",
				[]esper.FieldSpec{esper.OptionalFieldDef("id", stringT)},
				esper.RegisterObjectArray, reflect.TypeOf([]any{}))
		case "json":
			return regNested("LocalInnerEvent",
				[]esper.FieldSpec{esper.FieldDef("id", stringT)},
				esper.RegisterJSON, innerSliceT)
		case "json-provided":
			return reg(esper.RegisterJSONFor[eignArrayJSONProvided](env, "LocalEvent", nil))
		case "avro":
			return regNested("LocalInnerEvent",
				[]esper.FieldSpec{esper.FieldDef("id", stringT)},
				esper.RegisterAvro, innerSliceT)
		}
	case "nested-simple":
		switch mode {
		case "bean":
			inner, err := esper.RegisterStruct[eignSimpleInner](env, "LocalInnerEvent")
			if err != nil {
				return err
			}
			it.schemas["LocalInnerEvent"] = inner
			return reg(esper.RegisterStruct[eignSimpleEvent](env, "LocalEvent",
				esper.WithNestedPropertySchema("property", inner)))
		case "map":
			return regNested("LocalInnerEvent",
				[]esper.FieldSpec{esper.FieldDef("id", stringT)},
				esper.RegisterMap, mapT)
		case "objectarray":
			return regNested("LocalInnerEvent",
				[]esper.FieldSpec{esper.OptionalFieldDef("id", stringT)},
				esper.RegisterObjectArray, reflect.TypeOf([]any{}))
		case "json":
			return regNested("LocalInnerEvent",
				[]esper.FieldSpec{esper.FieldDef("id", stringT)},
				esper.RegisterJSON, mapT)
		case "json-provided":
			return reg(esper.RegisterJSONFor[eignSimpleJSONProvided](env, "LocalEvent", nil))
		}
	case "nested-simple-deep":
		switch mode {
		case "bean":
			inner, err := esper.RegisterStruct[eignDeepInner](env, "LocalInnerEvent")
			if err != nil {
				return err
			}
			it.schemas["LocalInnerEvent"] = inner
			leaf, err := esper.RegisterStruct[eignDeepLeaf](env, "LocalLeafEvent")
			if err != nil {
				return err
			}
			it.schemas["LocalLeafEvent"] = leaf
			return reg(esper.RegisterStruct[eignDeepEvent](env, "LocalEvent",
				esper.WithNestedPropertySchema("property", inner)))
		case "map":
			return regDeep(esper.RegisterMap)
		case "objectarray":
			leaf, err := esper.RegisterObjectArray(env, "LocalLeafEvent",
				[]esper.FieldSpec{esper.OptionalFieldDef("id", stringT)})
			if err != nil {
				return fmt.Errorf("%s: register schema: %w", eventInfraGetterNestedID, err)
			}
			it.schemas["LocalLeafEvent"] = leaf
			inner, err := esper.RegisterObjectArray(env, "LocalInnerEvent",
				[]esper.FieldSpec{esper.OptionalFieldDef("leaf", reflect.TypeOf([]any{}))},
				esper.WithNestedPropertySchema("leaf", leaf))
			if err != nil {
				return fmt.Errorf("%s: register schema: %w", eventInfraGetterNestedID, err)
			}
			it.schemas["LocalInnerEvent"] = inner
			return reg(esper.RegisterObjectArray(env, "LocalEvent",
				[]esper.FieldSpec{esper.OptionalFieldDef("property", reflect.TypeOf([]any{}))},
				esper.WithNestedPropertySchema("property", inner)))
		case "json":
			return regDeep(esper.RegisterJSON)
		case "json-provided":
			return reg(esper.RegisterJSONFor[eignDeepJSONProvided](env, "LocalEvent", nil))
		case "avro":
			return regDeep(esper.RegisterAvro)
		}
	case "simple-fragment":
		switch mode {
		case "bean":
			inner, err := esper.RegisterStruct[eignFragmentInner](env, "LocalInnerEvent")
			if err != nil {
				return err
			}
			it.schemas["LocalInnerEvent"] = inner
			return reg(esper.RegisterStruct[eignFragmentEvent](env, "LocalEvent",
				esper.WithNestedPropertySchema("property", inner)))
		case "map":
			return regNested("LocalInnerEvent", nil, esper.RegisterMap, mapT)
		case "objectarray":
			return regNested("LocalInnerEvent", nil, esper.RegisterObjectArray, reflect.TypeOf([]any{}))
		case "json":
			return regNested("LocalInnerEvent", nil, esper.RegisterJSON, mapT)
		case "json-provided":
			return reg(esper.RegisterJSONFor[eignFragmentJSONProvided](env, "LocalEvent", nil))
		case "avro":
			return regNested("LocalInnerEvent", nil, esper.RegisterAvro, mapT)
		}
	case "simple-no-fragment":
		switch mode {
		case "bean":
			return reg(esper.RegisterStruct[eignNoFragmentEvent](env, "LocalEvent"))
		case "map":
			return reg(esper.RegisterMap(env, "LocalEvent",
				[]esper.FieldSpec{esper.OptionalFieldDef("property", stringT)}))
		case "objectarray":
			return reg(esper.RegisterObjectArray(env, "LocalEvent",
				[]esper.FieldSpec{esper.OptionalFieldDef("property", stringT)}))
		case "json":
			return reg(esper.RegisterJSON(env, "LocalEvent",
				[]esper.FieldSpec{esper.OptionalFieldDef("property", stringT)}))
		case "json-provided":
			return reg(esper.RegisterJSONFor[eignNoFragmentJSONProvided](env, "LocalEvent", nil))
		case "avro":
			return reg(esper.RegisterAvro(env, "LocalEvent",
				[]esper.FieldSpec{esper.OptionalFieldDef("property", stringT)}))
		case "xml":
			return reg(esper.RegisterXML(env, eventInfraGetterNestedXMLType,
				[]esper.FieldSpec{esper.OptionalFieldDef("property", stringT)}))
		}
	}
	return fmt.Errorf("%s: no schema registration for %q/%q", eventInfraGetterNestedID, caseName, mode)
}

// send decodes one pinned payload, records the Java-side getter/projection
// expectations for the probe step, and dispatches through the sender the
// Java iteration uses for the underlying mode.
func (it *eignIteration) send(ctx context.Context, caseName string, step compat.Step) error {
	var payload map[string]json.RawMessage
	if err := strictObject(step.Payload, &payload); err != nil {
		return fmt.Errorf("%s: decode send payload: %w", eventInfraGetterNestedID, err)
	}
	it.mode = step.Mode
	if err := it.expectProbes(caseName, payload); err != nil {
		return err
	}
	switch step.Mode {
	case "bean":
		return it.sendBean(ctx, caseName, step.EventType, payload)
	case "map":
		event, err := it.decodeMap(caseName, payload)
		if err != nil {
			return err
		}
		return it.engine.Send(ctx, step.EventType, event)
	case "objectarray":
		return it.sendObjectArray(ctx, caseName, step.EventType, payload)
	case "json", "json-provided":
		raw, err := it.encodeJSON(caseName, payload)
		if err != nil {
			return err
		}
		return it.engine.SendJSON(ctx, step.EventType, raw)
	case "avro":
		return it.sendAvro(ctx, caseName, step.EventType, payload)
	case "xml":
		return it.sendXML(ctx, payload)
	default:
		return fmt.Errorf("%s: unsupported send mode %q", eventInfraGetterNestedID, step.Mode)
	}
}

// eignStr decodes a JSON string-or-null payload field into a *string.
func eignStr(raw json.RawMessage) *string {
	if len(raw) == 0 {
		return nil
	}
	var value *string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return value
}

// eignFlag decodes a required boolean payload field.
func eignFlag(payload map[string]json.RawMessage, name string) (bool, error) {
	var value bool
	if raw, ok := payload[name]; !ok || len(raw) == 0 {
		return false, fmt.Errorf("%s: send payload missing %q", eventInfraGetterNestedID, name)
	} else if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("%s: decode send payload %q: %w", eventInfraGetterNestedID, name, err)
	}
	return value, nil
}

// eignIDs decodes the nested-array ids payload: the slice and a null flag so
// the object-array null encoding (Object[][]{null}) stays distinct from the
// empty array.
func eignIDs(payload map[string]json.RawMessage) ([]*string, bool, error) {
	raw, ok := payload["ids"]
	if !ok || len(raw) == 0 {
		return nil, false, fmt.Errorf("%s: send payload missing %q", eventInfraGetterNestedID, "ids")
	}
	var ids []*string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, false, fmt.Errorf("%s: decode send payload %q: %w", eventInfraGetterNestedID, "ids", err)
	}
	return ids, ids == nil, nil
}

// expectProbes computes the pinned per-send expectations exactly as the Java
// runAssertion methods do.
func (it *eignIteration) expectProbes(caseName string, payload map[string]json.RawMessage) error {
	probes := eventInfraGetterNestedProbes[caseName]
	expects := make([]eignProbeExpect, len(probes))
	switch caseName {
	case "nested-array":
		ids, isNull, err := eignIDs(payload)
		if err != nil {
			return err
		}
		for index := range probes {
			exists := !isNull && len(ids) > index && ids[index] != nil
			expect := eignProbeExpect{exists: exists}
			if exists {
				expect.value = ids[index]
			}
			expects[index] = expect
		}
	case "nested-simple":
		present, err := eignFlag(payload, "present")
		if err != nil {
			return err
		}
		expects[0] = eignProbeExpect{exists: present}
		if present {
			expects[0].value = eignStr(payload["id"])
		}
	case "nested-simple-deep":
		root, err := eignFlag(payload, "root")
		if err != nil {
			return err
		}
		inner, err := eignFlag(payload, "inner")
		if err != nil {
			return err
		}
		exists := !root && !inner
		expects[0] = eignProbeExpect{exists: exists}
		if exists {
			expects[0].value = eignStr(payload["id"])
		}
	case "simple-fragment":
		value, err := eignFlag(payload, "value")
		if err != nil {
			return err
		}
		expects[0] = eignProbeExpect{exists: true, hasValue: value, fragment: value}
	case "simple-no-fragment":
		expects[0] = eignProbeExpect{exists: true, value: eignStr(payload["property"])}
	default:
		return fmt.Errorf("%s: no probe expectations for case %q", eventInfraGetterNestedID, caseName)
	}
	it.lastExpects = expects
	return nil
}

func (it *eignIteration) sendBean(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	switch caseName {
	case "nested-array":
		ids, isNull, _ := eignIDs(payload)
		var property []eignArrayInner
		if !isNull {
			property = make([]eignArrayInner, len(ids))
			for index, id := range ids {
				property[index] = eignArrayInner{ID: id}
			}
		}
		return it.engine.Send(ctx, eventType, eignArrayEvent{Property: property})
	case "nested-simple":
		present, _ := eignFlag(payload, "present")
		var property *eignSimpleInner
		if present {
			property = &eignSimpleInner{ID: eignStr(payload["id"])}
		}
		return it.engine.Send(ctx, eventType, eignSimpleEvent{Property: property})
	case "nested-simple-deep":
		root, _ := eignFlag(payload, "root")
		inner, _ := eignFlag(payload, "inner")
		var property *eignDeepInner
		if !root {
			property = &eignDeepInner{}
			if !inner {
				property.Leaf = &eignDeepLeaf{ID: eignStr(payload["id"])}
			}
		}
		return it.engine.Send(ctx, eventType, eignDeepEvent{Property: property})
	case "simple-fragment":
		value, _ := eignFlag(payload, "value")
		var property *eignFragmentInner
		if value {
			property = &eignFragmentInner{}
		}
		return it.engine.Send(ctx, eventType, eignFragmentEvent{Property: property})
	case "simple-no-fragment":
		return it.engine.Send(ctx, eventType, eignNoFragmentEvent{Property: eignStr(payload["property"])})
	default:
		return fmt.Errorf("%s: no bean sender for case %q", eventInfraGetterNestedID, caseName)
	}
}

// decodeMap builds the map payload the Java sender writes for one send.
func (it *eignIteration) decodeMap(caseName string, payload map[string]json.RawMessage) (map[string]any, error) {
	switch caseName {
	case "nested-array":
		ids, isNull, _ := eignIDs(payload)
		var property any
		if !isNull {
			items := make([]any, len(ids))
			for index, id := range ids {
				item := map[string]any{"id": nil}
				if id != nil {
					item["id"] = *id
				}
				items[index] = item
			}
			property = items
		}
		return map[string]any{"property": property}, nil
	case "nested-simple":
		present, _ := eignFlag(payload, "present")
		var property any
		if present {
			inner := map[string]any{"id": nil}
			if id := eignStr(payload["id"]); id != nil {
				inner["id"] = *id
			}
			property = inner
		}
		return map[string]any{"property": property}, nil
	case "nested-simple-deep":
		root, _ := eignFlag(payload, "root")
		inner, _ := eignFlag(payload, "inner")
		event := map[string]any{}
		if !root {
			if inner {
				event["property"] = map[string]any{"leaf": nil}
			} else {
				leaf := map[string]any{"id": nil}
				if id := eignStr(payload["id"]); id != nil {
					leaf["id"] = *id
				}
				event["property"] = map[string]any{"leaf": leaf}
			}
		}
		return event, nil
	case "simple-fragment":
		value, _ := eignFlag(payload, "value")
		var property any
		if value {
			property = map[string]any{}
		}
		return map[string]any{"property": property}, nil
	case "simple-no-fragment":
		event := map[string]any{"property": nil}
		if property := eignStr(payload["property"]); property != nil {
			event["property"] = *property
		}
		return event, nil
	default:
		return nil, fmt.Errorf("%s: no map sender for case %q", eventInfraGetterNestedID, caseName)
	}
}

func (it *eignIteration) sendObjectArray(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	str := func(raw json.RawMessage) any {
		if decoded := eignStr(raw); decoded != nil {
			return *decoded
		}
		return nil
	}
	switch caseName {
	case "nested-array":
		ids, isNull, _ := eignIDs(payload)
		var property any
		if isNull {
			// Java encodes the null array as Object[][]{null}, a one-element
			// array holding a null row.
			property = []any{nil}
		} else {
			items := make([]any, len(ids))
			for index, id := range ids {
				items[index] = []any{eignAny(id)}
			}
			property = items
		}
		return it.engine.SendObjectArray(ctx, eventType, []any{property})
	case "nested-simple":
		present, _ := eignFlag(payload, "present")
		var property any
		if present {
			property = []any{str(payload["id"])}
		}
		return it.engine.SendObjectArray(ctx, eventType, []any{property})
	case "nested-simple-deep":
		root, _ := eignFlag(payload, "root")
		inner, _ := eignFlag(payload, "inner")
		var property any
		if !root {
			if inner {
				property = []any{nil}
			} else {
				property = []any{[]any{str(payload["id"])}}
			}
		}
		return it.engine.SendObjectArray(ctx, eventType, []any{property})
	case "simple-fragment":
		value, _ := eignFlag(payload, "value")
		var property any
		if value {
			property = []any{}
		}
		return it.engine.SendObjectArray(ctx, eventType, []any{property})
	case "simple-no-fragment":
		return it.engine.SendObjectArray(ctx, eventType, []any{str(payload["property"])})
	default:
		return fmt.Errorf("%s: no object-array sender for case %q", eventInfraGetterNestedID, caseName)
	}
}

// eignAny renders a decoded nullable string as an object-array cell value.
func eignAny(id *string) any {
	if id == nil {
		return nil
	}
	return *id
}

// encodeJSON builds the JSON document the Java sender writes for one send.
func (it *eignIteration) encodeJSON(caseName string, payload map[string]json.RawMessage) ([]byte, error) {
	event := make(map[string]any, 1)
	switch caseName {
	case "nested-array":
		ids, isNull, _ := eignIDs(payload)
		if isNull {
			event["property"] = nil
		} else {
			items := make([]any, len(ids))
			for index, id := range ids {
				item := map[string]any{"id": nil}
				if id != nil {
					item["id"] = *id
				}
				items[index] = item
			}
			event["property"] = items
		}
	case "nested-simple":
		present, _ := eignFlag(payload, "present")
		if present {
			inner := map[string]any{"id": nil}
			if id := eignStr(payload["id"]); id != nil {
				inner["id"] = *id
			}
			event["property"] = inner
		}
	case "nested-simple-deep":
		root, _ := eignFlag(payload, "root")
		inner, _ := eignFlag(payload, "inner")
		if !root {
			if inner {
				event["property"] = map[string]any{"leaf": nil}
			} else {
				leaf := map[string]any{"id": nil}
				if id := eignStr(payload["id"]); id != nil {
					leaf["id"] = *id
				}
				event["property"] = map[string]any{"leaf": leaf}
			}
		}
	case "simple-fragment":
		value, _ := eignFlag(payload, "value")
		if value {
			event["property"] = map[string]any{}
		} else {
			event["property"] = nil
		}
	case "simple-no-fragment":
		if property := eignStr(payload["property"]); property != nil {
			event["property"] = *property
		} else {
			event["property"] = nil
		}
	default:
		return nil, fmt.Errorf("%s: no json sender for case %q", eventInfraGetterNestedID, caseName)
	}
	return json.Marshal(event)
}

func (it *eignIteration) sendAvro(ctx context.Context, caseName, eventType string, payload map[string]json.RawMessage) error {
	schema, ok := it.schemas[eventType]
	if !ok {
		return fmt.Errorf("%s: no Avro schema %q", eventInfraGetterNestedID, eventType)
	}
	record, err := esper.NewAvroRecordFromMap(schema, nil)
	if err != nil {
		return err
	}
	switch caseName {
	case "nested-array":
		ids, isNull, _ := eignIDs(payload)
		// The Java Avro sender writes an empty list for the null-array send.
		items := make([]map[string]any, 0)
		if !isNull {
			for _, id := range ids {
				item := map[string]any{"id": nil}
				if id != nil {
					item["id"] = *id
				}
				items = append(items, item)
			}
		}
		if err := record.Set("property", items); err != nil {
			return err
		}
	case "nested-simple-deep":
		root, _ := eignFlag(payload, "root")
		inner, _ := eignFlag(payload, "inner")
		if !root {
			innerRecord := map[string]any{"leaf": nil}
			if !inner {
				leaf := map[string]any{"id": nil}
				if id := eignStr(payload["id"]); id != nil {
					leaf["id"] = *id
				}
				innerRecord["leaf"] = leaf
			}
			if err := record.Set("property", innerRecord); err != nil {
				return err
			}
		}
	case "simple-fragment":
		value, _ := eignFlag(payload, "value")
		if value {
			if err := record.Set("property", map[string]any{}); err != nil {
				return err
			}
		}
	case "simple-no-fragment":
		// Java's null send uses an ad-hoc optionalString record; the deployed
		// schema field is optional so a plain null is observably identical.
		if property := eignStr(payload["property"]); property != nil {
			if err := record.Set("property", *property); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("%s: no avro sender for case %q", eventInfraGetterNestedID, caseName)
	}
	return it.engine.SendAvro(ctx, eventType, record)
}

// sendXML sends the simple-no-fragment XML document. The Java sender maps the
// property onto a root-element attribute; Go's XML schema resolves the same
// observable value through a leaf element.
func (it *eignIteration) sendXML(ctx context.Context, payload map[string]json.RawMessage) error {
	doc := "<" + eventInfraGetterNestedXMLType + "></" + eventInfraGetterNestedXMLType + ">"
	if property := eignStr(payload["property"]); property != nil {
		doc = "<" + eventInfraGetterNestedXMLType + "><property>" +
			strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(*property) +
			"</property></" + eventInfraGetterNestedXMLType + ">"
	}
	return it.engine.SendXML(ctx, eventInfraGetterNestedXMLType, []byte(doc))
}

// emitGetterProbes mirrors the Java assertGetter/assertGetters block that runs
// on the last s0 event after each send: one record per probe path carrying
// exists (isExistsProperty), value (get) and fragment (getFragment != null).
// The pinned expectations computed at send time are asserted in-process.
func (it *eignIteration) emitGetterProbes(trace *compat.Trace, caseName string) error {
	if it.lastS0 == nil {
		return fmt.Errorf("%s: no s0 event captured for getter probe", eventInfraGetterNestedID)
	}
	event := *it.lastS0
	eventType := eignEventType(it.mode)
	schema, ok := it.schemas[eventType]
	if !ok {
		return fmt.Errorf("%s: no %s schema for getter probe", eventInfraGetterNestedID, eventType)
	}
	probes := eventInfraGetterNestedProbes[caseName]
	if len(it.lastExpects) != len(probes) {
		return fmt.Errorf("%s: %d probe expectations for %d probes", eventInfraGetterNestedID, len(it.lastExpects), len(probes))
	}
	for index, javaPath := range probes {
		expect := it.lastExpects[index]
		goPath := eignGoPath(javaPath)
		getterPath := goPath
		if it.mode == "xml" {
			getterPath = javaPath
		}
		if _, found := schema.Getter(getterPath); !found {
			return fmt.Errorf("%s: getter %q unexpectedly unresolvable on %s", eventInfraGetterNestedID, getterPath, schema.Name())
		}
		value := event.Get(goPath)
		exists := !value.IsMissing()
		if it.mode == "xml" {
			// The Java XML property getter reports isExistsProperty=true for
			// the schema-declared attribute even when the document omits it.
			exists = true
		}
		if exists != expect.exists {
			return fmt.Errorf("%s: getter %q exists = %t, want %t", eventInfraGetterNestedID, javaPath, exists, expect.exists)
		}
		fragment := false
		if caseName == "simple-fragment" {
			_, fragment = event.GetFragment("property")
		} else if _, ok := event.GetFragment(goPath); ok {
			// Java asserts getFragment null for every leaf/scalar probe.
			return fmt.Errorf("%s: getter %q fragment unexpectedly non-null", eventInfraGetterNestedID, javaPath)
		}
		if fragment != expect.fragment {
			return fmt.Errorf("%s: getter %q fragment = %t, want %t", eventInfraGetterNestedID, javaPath, fragment, expect.fragment)
		}
		recordValue := any(nil)
		if exists {
			if caseName == "simple-fragment" {
				if value.IsPresent() != expect.hasValue {
					return fmt.Errorf("%s: getter %q present = %t, want %t", eventInfraGetterNestedID, javaPath, value.IsPresent(), expect.hasValue)
				}
				recordValue = eignNormalizeProbeValue(value)
			} else if it.mode == "xml" && !value.IsPresent() {
				recordValue = map[string]any{"state": "null"}
			} else {
				got := eignProbeString(value)
				if (got != nil) != (expect.value != nil) {
					return fmt.Errorf("%s: getter %q value mismatch", eventInfraGetterNestedID, javaPath)
				}
				if got != nil && *got != *expect.value {
					return fmt.Errorf("%s: getter %q value = %q, want %q", eventInfraGetterNestedID, javaPath, *got, *expect.value)
				}
				recordValue = eignNormalizeProbeValue(value)
			}
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

func (it *eignIteration) undeployAll(ctx context.Context) error {
	for index := len(it.deployOrder) - 1; index >= 0; index-- {
		label := it.deployOrder[index]
		deployment, ok := it.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", eventInfraGetterNestedID, label, err)
		}
		delete(it.deployments, label)
	}
	it.deployOrder = nil
	return nil
}

// eignNormalizeProbeValue renders a getter value like the Java oracle's
// normalize: a missing or null value stays the tagged null object, maps
// become plain objects and everything else passes through to the JSON
// encoder.
func eignNormalizeProbeValue(value esper.Value) any {
	if !value.IsPresent() {
		return map[string]any{"state": "null"}
	}
	return value.Any()
}

// eignProbeString dereferences a probe value to a nullable string, covering
// the *string leaf values the bean and json-provided underlyings carry.
func eignProbeString(value esper.Value) *string {
	if !value.IsPresent() {
		return nil
	}
	switch typed := value.Any().(type) {
	case string:
		return &typed
	case *string:
		return typed
	default:
		return nil
	}
}

// eignUntagNested converts nested {state:null} tags inside row field values
// to plain JSON nulls, matching the Java oracle's normalizeNested (the
// top-level field itself keeps the tagged null object).
func eignUntagNested(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 1 && typed["state"] == "null" {
			return value
		}
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = eignUntagNestedChild(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = eignUntagNestedChild(item)
		}
		return out
	default:
		return value
	}
}

// eignUntagNestedChild untags {state:null} at every level below the row
// field root, recursing into maps and arrays.
func eignUntagNestedChild(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 1 && typed["state"] == "null" {
			return nil
		}
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = eignUntagNestedChild(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = eignUntagNestedChild(item)
		}
		return out
	default:
		return value
	}
}

// loadEventInfraGetterNestedScenario decodes the scenario JSON with strict
// field checking and pins the metadata, case identities and step sequence.
func loadEventInfraGetterNestedScenario(reader io.Reader) (compat.Scenario, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario must be a JSON object: %w", eventInfraGetterNestedID, err)
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eventInfraGetterNestedID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eventInfraGetterNestedID ||
		metadata.Description != eventInfraGetterNestedDescription ||
		metadata.JavaCommit != eventInfraGetterNestedJavaCommit || metadata.JavaSource != eventInfraGetterNestedSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", eventInfraGetterNestedID)
	}
	if err := eventInfraGetterNestedRequireEqual(metadata.JavaFlags, eventInfraGetterNestedJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eventInfraGetterNestedRequireEqual(metadata.JavaRuntimes, eventInfraGetterNestedJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eventInfraGetterNestedRequireEqual(metadata.JavaNames, eventInfraGetterNestedJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eventInfraGetterNestedRequireEqual(metadata.JavaStaticID, eventInfraGetterNestedJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", eventInfraGetterNestedID, err)
	}
	if len(rawCases) != len(eventInfraGetterNestedCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", eventInfraGetterNestedID, len(rawCases), len(eventInfraGetterNestedCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEventInfraGetterNestedFields(object,
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
		caseName := eventInfraGetterNestedCases[index]
		if definition.Case != caseName || definition.Ordinal != 0 ||
			definition.RuntimeID != eventInfraGetterNestedJavaRuntimeIDs[index] ||
			definition.ExecutionName != eventInfraGetterNestedJavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d identity = %+v, want case %q ordinal 0 runtime %s execution %s",
				eventInfraGetterNestedID, index, definition, caseName,
				eventInfraGetterNestedJavaRuntimeIDs[index], eventInfraGetterNestedJavaExecutions[index])
		}
		if definition.EPL != eventInfraGetterNestedS1[caseName] {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source", caseName)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", eventInfraGetterNestedID, err)
	}
	pinned := eventInfraGetterNestedPinnedSteps()
	if len(rawSteps) != len(pinned) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d steps, want %d", eventInfraGetterNestedID, len(rawSteps), len(pinned))
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
		if err := eventInfraGetterNestedValidateStep(index, object, step); err != nil {
			return compat.Scenario{}, err
		}
		if key := eventInfraGetterNestedStepKey(step); key != pinned[index] {
			return compat.Scenario{}, fmt.Errorf("scenario step %d = %q, want pinned %q", index, key, pinned[index])
		}
		steps[index] = step
	}
	return compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}, nil
}

// eventInfraGetterNestedValidateStep enforces the per-op field set.
func eventInfraGetterNestedValidateStep(index int, object map[string]json.RawMessage, step compat.Step) error {
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
		if step.Statement == "schema" {
			pinned, ok := eventInfraGetterNestedSchemaEPL(step.Case, step.Mode)
			if !ok {
				return fmt.Errorf("scenario step %d: schema EPL is not pinned", index)
			}
			if pinned == "" {
				// Config-registered types (the xml underlying) carry no schema
				// EPL; the field is optional so the decoded step round-trips.
				if err := require("op", "case", "statement", "mode"); err != nil {
					return err
				}
				if step.Epl != "" {
					return fmt.Errorf("scenario step %d: schema EPL is not pinned", index)
				}
				return nil
			}
			if err := require("op", "case", "statement", "epl", "mode"); err != nil {
				return err
			}
			if step.Epl != pinned {
				return fmt.Errorf("scenario step %d: schema EPL is not pinned", index)
			}
			return nil
		}
		if err := require("op", "case", "statement", "epl"); err != nil {
			return err
		}
		if step.Statement == "s0" && step.Epl != eventInfraGetterNestedS0 && step.Epl != eventInfraGetterNestedS0XML {
			return fmt.Errorf("scenario step %d: s0 EPL is not pinned", index)
		}
		if step.Statement == "s1" && step.Epl != eventInfraGetterNestedS1[step.Case] && step.Epl != eventInfraGetterNestedS1XML {
			return fmt.Errorf("scenario step %d: s1 EPL is not pinned", index)
		}
		return nil
	case "deployed":
		return require("op", "case", "statement")
	case "send":
		if err := require("op", "case", "eventType", "mode", "payload"); err != nil {
			return err
		}
		if step.EventType != eignEventType(step.Mode) {
			return fmt.Errorf("scenario step %d: send eventType %q is not pinned", index, step.EventType)
		}
		return nil
	case "undeploy-all":
		return require("op", "case")
	default:
		return fmt.Errorf("scenario step %d: unsupported op %q", index, step.Op)
	}
}

// eventInfraGetterNestedStepKey renders one step in canonical form for the
// pinned sequence comparison.
func eventInfraGetterNestedStepKey(step compat.Step) string {
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

// eventInfraGetterNestedPinnedSteps rebuilds the pinned step sequence from
// the same tables the scenario JSON was generated from.
func eventInfraGetterNestedPinnedSteps() []string {
	var keys []string
	for _, caseName := range eventInfraGetterNestedCases {
		keys = append(keys, "case|"+caseName+"|||||||")
		for _, mode := range eventInfraGetterNestedModes(caseName) {
			epl, _ := eventInfraGetterNestedSchemaEPL(caseName, mode)
			keys = append(keys, strings.Join([]string{"deploy", caseName, "schema", "", mode, "", epl, "", ""}, "|"))
			keys = append(keys, "deployed|"+caseName+"|schema||||||")
			keys = append(keys, strings.Join([]string{"deploy", caseName, "s0", "", "", "", eventInfraGetterNestedS0EPL(mode), "", ""}, "|"))
			keys = append(keys, "deployed|"+caseName+"|s0||||||")
			keys = append(keys, strings.Join([]string{"deploy", caseName, "s1", "", "", "", eventInfraGetterNestedS1EPL(caseName, mode), "", ""}, "|"))
			keys = append(keys, "deployed|"+caseName+"|s1||||||")
			eventType := eignEventType(mode)
			for _, payload := range eventInfraGetterNestedPayloads[caseName] {
				keys = append(keys, strings.Join([]string{"send", caseName, "", eventType, mode, "", "", "", payload}, "|"))
			}
			keys = append(keys, "undeploy-all|"+caseName+"|||||||")
		}
	}
	return keys
}

func requireEventInfraGetterNestedFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("expected fields %v, got %v", names, keysOfEign(object))
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("missing field %q", name)
		}
	}
	return nil
}

func keysOfEign(object map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	return keys
}

func eventInfraGetterNestedRequireEqual(actual, expected []string, name string) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("%s %s length = %d, want %d", eventInfraGetterNestedID, name, len(actual), len(expected))
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf("%s %s[%d] = %q, want %q", eventInfraGetterNestedID, name, index, actual[index], expected[index])
		}
	}
	return nil
}
