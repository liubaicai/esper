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

// event_objectarray_core.go replays EventObjectArrayCore ords 0, 1, 3 and 4
// against the pinned Java oracle (ord 2 EventObjectArrayQueryFields is already
// case.object-array-query-fields and stays out of scope):
//
//   - metadata (ord 0, EventObjectArrayMetadata): no EPL and no events — the
//     execution only introspects the preconfigured MyObjectArrayEvent type
//     (OBJECTARR application type, name, exactly three property descriptors:
//     myInt Integer, myString String, beanA SupportBeanComplexProps fragment;
//     order-insensitive). Both sides assert the same shape internally and the
//     case emits a single deployed marker, exactly like the map-core
//     metadata precedent.
//   - nested-objects (ord 1, EventObjectArrayNestedObjects): one
//     deploy/send/undeploy-all cycle navigating beanA.simpleProperty,
//     beanA.nested.nestedValue, beanA.indexed[1] and
//     beanA.nested.nestedNested.nestedNestedValue over
//     MyObjectArrayEvent#length(5).
//   - nested-eventbean-array (ord 3, EventObjectArrayNestedEventBeanArray):
//     a two-statement objectarray schema module (NBAL_1(val string),
//     NBAL_2 (lvl1s NBAL_1[])) deploys first; select * from NBAL_1 takes one
//     send, undeployModuleContaining("s0") retires only the select (the
//     schema module stays deployed), then select lvl1s[0] as c0 from NBAL_2
//     yields the raw Object[] carrier — not an event — rendered as a plain
//     JSON array on both sides.
//   - invalid (ord 4, EventObjectArrayInvalid): two of Java's three
//     compile-phase rejections replay as build-error probes — select XXX
//     (unknown property) and select myString * 2 (String arithmetic). The
//     third probe select String.trim(myInt) is unrepresentable in the Go
//     fluent API (no static-method-call expression surface) and is
//     documented in the case notes only, mirroring the
//     resultset-aggregate-invalid-closure precedent.
//
// Approved differences (observably identical to the Java EPL):
//   - The schema module's create-objectarray-schema statements map to
//     env-level RegisterObjectArray calls with BusEventType; Go has no
//     module path, so the deploy step registers the schemas without a
//     deployment and undeployModuleContaining("s0") retires only the s0
//     deployment.
//   - select * from NBAL_1 projects the single declared val column
//     explicitly; the row renders the same {kind:row,fields:{val:...}}
//     shape as Java's event row.
//   - Property path strings carry the verbatim Java property paths
//     (beanA.indexed[1], lvl1s[0]) resolved by Schema.get at runtime.

const (
	eventObjectArrayCoreID         = "event-object-array-core"
	eventObjectArrayCoreJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	eventObjectArrayCoreSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/objectarray/EventObjectArrayCore.java"
)

const eventObjectArrayCoreDescription = "EventObjectArrayCore object-array event core (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c): metadata introspects the preconfigured MyObjectArrayEvent (OBJECTARR application type, name, exactly three descriptors — myInt:Integer, myString:String, beanA:SupportBeanComplexProps fragment — order-insensitive) with no events, acknowledged by a deployed marker (ord 0, EventObjectArrayMetadata); nested-objects navigates beanA.simpleProperty, beanA.nested.nestedValue, beanA.indexed[1] and beanA.nested.nestedNested.nestedNestedValue over MyObjectArrayEvent#length(5) (ord 1, EventObjectArrayNestedObjects); nested-eventbean-array deploys a two-statement objectarray schema module (NBAL_1(val string), NBAL_2 (lvl1s NBAL_1[])), selects * from NBAL_1 for one send, undeploys the s0 module while the schema module stays, then selects lvl1s[0] as c0 from NBAL_2 yielding the raw Object[] carrier (ord 3, EventObjectArrayNestedEventBeanArray); invalid covers two of Java's three compile-phase rejections — unknown property XXX and String arithmetic myString * 2 — while the third probe select String.trim(myInt) is unrepresentable (Go has no static-method-call expression surface) (ord 4, EventObjectArrayInvalid). Ord 2 EventObjectArrayQueryFields is covered by case.object-array-query-fields and stays out of scope (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/objectarray/EventObjectArrayCore.java)."

var eventObjectArrayCoreCaseObservations = []string{
	"deployed; no EPL and no events — the execution only introspects the preconfigured MyObjectArrayEvent type (OBJECTARR application type, name MyObjectArrayEvent, exactly three property descriptors: myInt Integer, myString String, beanA SupportBeanComplexProps fragment; order-insensitive) so the case emits a single deployed marker",
	"listener; one deploy/send/undeploy-all cycle over MyObjectArrayEvent{myInt:int,myString:string,beanA:SupportBeanComplexProps}: beanA.simpleProperty/beanA.nested.nestedValue/beanA.indexed[1]/beanA.nested.nestedNested.nestedNestedValue yields simple=simple,nested=nestedValue,indexed=2,nestednested=nestedNestedValue",
	"listener; schema module deploy then two select cycles: select * from NBAL_1 yields val=somevalue; undeployModuleContaining('s0') retires only the select (the schema module stays deployed); select lvl1s[0] as c0 from NBAL_2 yields c0=[somevalue] as the raw Object[] carrier, not an event",
	"compile-error; two build-error probes — select XXX (unknown property) and select myString * 2 (String arithmetic) — are rejected at build; the third Java probe select String.trim(myInt) is unrepresentable in the Go fluent API (no static-method-call expression) and is documented here only",
}

// Byte-exact EPL transcriptions of EventObjectArrayCore.java: the ord 1
// statement keeps the missing spaces after commas and the space before
// 'from' from the Java string concatenation; the ord 3 module keeps the
// NBAL_1(val string) / NBAL_2 (lvl1s NBAL_1[]) paren spacing and the
// inter-statement newline; the ord 4 probes carry no @name.
const (
	eventObjectArrayCoreEPLNestedObjects = "@name('s0') select beanA.simpleProperty as simple,beanA.nested.nestedValue as nested,beanA.indexed[1] as indexed,beanA.nested.nestedNested.nestedNestedValue as nestednested from MyObjectArrayEvent#length(5)"
	eventObjectArrayCoreEPLSchemas       = "@buseventtype @public create objectarray schema NBAL_1(val string);\n@buseventtype @public create objectarray schema NBAL_2 (lvl1s NBAL_1[]);\n"
	eventObjectArrayCoreEPLSelectNBAL1   = "@name('s0') select * from NBAL_1"
	eventObjectArrayCoreEPLSelectNBAL2   = "@name('s0') select lvl1s[0] as c0 from NBAL_2"
	eventObjectArrayCoreEPLProbeUnknown  = "select XXX from MyObjectArrayEvent#length(5)"
	eventObjectArrayCoreEPLProbeString   = "select myString * 2 from MyObjectArrayEvent#length(5)"
)

// eventObjectArrayCoreCaseEPLs pins the EPL metadata of each case: the
// metadata case deploys nothing (empty), multi-statement executions
// concatenate their statements with a trailing newline (the same convention
// as event-map-properties), and the invalid case concatenates its two
// covered probes.
var eventObjectArrayCoreCaseEPLs = []string{
	"",
	eventObjectArrayCoreEPLNestedObjects,
	eventObjectArrayCoreEPLSchemas + eventObjectArrayCoreEPLSelectNBAL1 + "\n" + eventObjectArrayCoreEPLSelectNBAL2 + "\n",
	eventObjectArrayCoreEPLProbeUnknown + "\n" + eventObjectArrayCoreEPLProbeString + "\n",
}

var (
	eventObjectArrayCoreJavaRuntimeIDs = []string{
		"java-runtime-17dd924d2ddc5c9577fe",
		"java-runtime-52d765c3469a8eddd95e",
		"java-runtime-8f3cd3e434650bc6364f",
		"java-runtime-4f63efae89e8c0bd4e66",
	}
	eventObjectArrayCoreJavaExecutions = []string{
		"EventObjectArrayMetadata",
		"EventObjectArrayNestedObjects",
		"EventObjectArrayNestedEventBeanArray",
		"EventObjectArrayInvalid",
	}
	eventObjectArrayCoreJavaStaticIDs = []string{
		"java-0556d6715c212191e007",
		"java-0556d6715c212191e007",
		"java-0556d6715c212191e007",
		"java-0556d6715c212191e007",
	}
	eventObjectArrayCoreJavaFlags = []string{}
	eventObjectArrayCoreCases     = []string{
		"metadata",
		"nested-objects",
		"nested-eventbean-array",
		"invalid",
	}
	eventObjectArrayCoreOrdinals = []int{0, 1, 3, 4}
	eventObjectArrayCoreSources  = []string{eventObjectArrayCoreSource}
)

// eventObjectArrayCoreCaseSteps pins the complete step sequence per case as
// op|case|statement|eventType|epl|payload|expectError keys so the loader
// asserts the scenario file matches the contract. The schema-module deploy
// carries the byte-exact two-statement EPL under the "schemas" label; the
// build-error probes carry their verbatim EPL with an empty expectError
// (Java's tryInvalidCompile "skip" pins no message text).
var eventObjectArrayCoreCaseSteps = map[string][]string{
	"metadata": {
		"deployed|metadata|metadata||||",
	},
	"nested-objects": {
		"deploy|nested-objects|s0||@name('s0') select beanA.simpleProperty as simple,beanA.nested.nestedValue as nested,beanA.indexed[1] as indexed,beanA.nested.nestedNested.nestedNestedValue as nestednested from MyObjectArrayEvent#length(5)||",
		`send|nested-objects||MyObjectArrayEvent||[3,"some string",{"_bean":"SupportBeanComplexProps"}]|`,
		"undeploy-all|nested-objects|||||",
	},
	"nested-eventbean-array": {
		"deploy|nested-eventbean-array|schemas||@buseventtype @public create objectarray schema NBAL_1(val string);\n@buseventtype @public create objectarray schema NBAL_2 (lvl1s NBAL_1[]);\n||",
		"deploy|nested-eventbean-array|s0||@name('s0') select * from NBAL_1||",
		`send|nested-eventbean-array||NBAL_1||["somevalue"]|`,
		"undeploy|nested-eventbean-array|s0||||",
		"deploy|nested-eventbean-array|s0||@name('s0') select lvl1s[0] as c0 from NBAL_2||",
		`send|nested-eventbean-array||NBAL_2||[[["somevalue"]]]|`,
		"undeploy-all|nested-eventbean-array|||||",
	},
	"invalid": {
		"build-error|invalid|unknown-property||select XXX from MyObjectArrayEvent#length(5)||",
		"build-error|invalid|string-arithmetic||select myString * 2 from MyObjectArrayEvent#length(5)||",
	},
}

func runEventObjectArrayCoreScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eventObjectArrayCoreCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEventObjectArrayCoreCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eventObjectArrayCoreID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", eventObjectArrayCoreID, scenario.ID)
	}
	return trace, nil
}

// runEventObjectArrayCoreCase replays one execution on a fresh
// environment/engine pair, mirroring the Java execution's fresh runtime:
// MyObjectArrayEvent registers up front (the TestSuiteEventObjectArray
// preconfigured type), the metadata case asserts the schema shape before
// its deployed marker, deploy steps build the pinned plan or register the
// NBAL schemas, sends deliver positional object-array payloads, and the
// undeploy/undeploy-all steps mirror the Java module boundaries.
func runEventObjectArrayCoreCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if err := registerEventObjectArrayCoreTypes(env); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(eventObjectArrayCoreJavaRuntimeIDs[eventObjectArrayCoreOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	sequences := make(map[string]uint64)
	deployments := map[string]*esper.Deployment{}
	var deployOrder []string

	// Pinned pre-execution assertions, mirroring the oracle's assertThat:
	// the metadata case introspects the preconfigured type before any step.
	if caseName == "metadata" {
		if err := assertEventObjectArrayCoreMetadata(env); err != nil {
			return trace, err
		}
	}

	subscribe := func(statement *esper.Statement) error {
		stmt := statement
		_, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			sequences[stmt.Name()]++
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "listener",
				Statement: stmt.Name(),
				Sequence:  sequences[stmt.Name()],
				Time:      compat.FormatTraceTime(batch.Time),
			}
			var renderErr error
			record.New, renderErr = oaResultRows(batch.New, nil)
			if renderErr != nil {
				return renderErr
			}
			record.Old, renderErr = oaResultRows(batch.Old, nil)
			if renderErr != nil {
				return renderErr
			}
			trace.Records = append(trace.Records, record)
			return nil
		})
		return err
	}

	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "deploy":
			if step.Epl == eventObjectArrayCoreEPLSchemas {
				// The Java module's create-objectarray-schema statements map
				// to env-level registrations (Go has no module path); the
				// deployment carries no s0 statement and emits no record.
				if err := registerEventObjectArrayCoreNBAL(env); err != nil {
					return trace, fmt.Errorf("deploy %q/%q: %w", caseName, step.Statement, err)
				}
				continue
			}
			query, err := buildEventObjectArrayCore(env, step.Epl)
			if err != nil {
				return trace, fmt.Errorf("build %q/%q: %w", caseName, step.Statement, err)
			}
			plan, err := env.Build(query)
			if err != nil {
				return trace, fmt.Errorf("build %q/%q: %w", caseName, step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return trace, fmt.Errorf("deploy %q/%q: %w", caseName, step.Statement, err)
			}
			deployments[step.Statement] = deployment
			deployOrder = append(deployOrder, step.Statement)
			for _, statement := range deployment.Statements() {
				if err := subscribe(statement); err != nil {
					return trace, err
				}
			}
		case "deployed":
			// Only the metadata case (which deploys nothing) emits the
			// marker, exactly like the oracle's deployedMarker.
			if caseName != "metadata" {
				return trace, fmt.Errorf("%s: unexpected deployed op for case %q", eventObjectArrayCoreID, caseName)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  0,
			})
		case "send":
			if err := sendEventObjectArrayCore(ctx, engine, step); err != nil {
				return trace, err
			}
		case "undeploy":
			// undeployModuleContaining: every select deploys as its own
			// module, so the statement key selects the whole deployment;
			// the env-registered NBAL schemas stay.
			deployment, ok := deployments[step.Statement]
			if !ok {
				return trace, fmt.Errorf("undeploy %q/%q: no deployment", caseName, step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return trace, fmt.Errorf("undeploy %q/%q: %w", caseName, step.Statement, err)
			}
			delete(deployments, step.Statement)
		case "undeploy-all":
			for index := len(deployOrder) - 1; index >= 0; index-- {
				label := deployOrder[index]
				deployment, ok := deployments[label]
				if !ok {
					continue
				}
				if err := deployment.Undeploy(ctx); err != nil {
					return trace, fmt.Errorf("undeploy-all %q/%q: %w", caseName, label, err)
				}
				delete(deployments, label)
			}
			deployOrder = nil
		case "build-error":
			if err := buildEventObjectArrayCoreError(env, step, &trace, caseName); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("%s: unsupported step op %q", eventObjectArrayCoreID, step.Op)
		}
	}
	return trace, nil
}

func eventObjectArrayCoreOrdinal(caseName string) int {
	for index, name := range eventObjectArrayCoreCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// registerEventObjectArrayCoreTypes mirrors the TestSuiteEventObjectArray
// registration this suite uses: MyObjectArrayEvent{myInt:int,
// myString:string, beanA:SupportBeanComplexProps} with beanA carrying the
// struct mirror as its nested fragment schema (Java marks the bean-typed
// property a fragment).
func registerEventObjectArrayCoreTypes(env *esper.Environment) error {
	beanASchema, err := esper.StructSchema[oaComplexProps]("SupportBeanComplexProps")
	if err != nil {
		return fmt.Errorf("schema SupportBeanComplexProps: %w", err)
	}
	if _, err := esper.RegisterObjectArray(env, "MyObjectArrayEvent", []esper.FieldSpec{
		esper.FieldDef("myInt", reflect.TypeOf(int64(0))),
		esper.FieldDef("myString", reflect.TypeOf("")),
		esper.FieldDef("beanA", reflect.TypeOf(oaComplexProps{})),
	}, esper.WithNestedPropertySchema("beanA", beanASchema)); err != nil {
		return fmt.Errorf("register MyObjectArrayEvent: %w", err)
	}
	return nil
}

// registerEventObjectArrayCoreNBAL mirrors the ord 3 schema module:
// @buseventtype @public create objectarray schema NBAL_1(val string) and
// NBAL_2 (lvl1s NBAL_1[]) — env-level bus-visible registrations whose
// lifetime is the case runtime, matching the Java schema module that stays
// deployed across the s0 undeploy boundary. lvl1s stays a plain []any
// field (no nested schema): Java's lvl1s[0] returns the raw Object[]
// carrier rather than an event, and a nested schema would render the
// element as a row instead of the pinned plain JSON array.
func registerEventObjectArrayCoreNBAL(env *esper.Environment) error {
	if _, err := esper.RegisterObjectArray(env, "NBAL_1", []esper.FieldSpec{
		esper.FieldDef("val", reflect.TypeOf("")),
	}, esper.BusEventType()); err != nil {
		return fmt.Errorf("register NBAL_1: %w", err)
	}
	if _, err := esper.RegisterObjectArray(env, "NBAL_2", []esper.FieldSpec{
		esper.FieldDef("lvl1s", reflect.TypeOf([]any{})),
	}, esper.BusEventType()); err != nil {
		return fmt.Errorf("register NBAL_2: %w", err)
	}
	return nil
}

// assertEventObjectArrayCoreMetadata mirrors the oracle's assertMetadata:
// OBJECTARR application type, the MyObjectArrayEvent name and exactly three
// property descriptors — myInt/myString simple and beanA a fragment —
// compared order-insensitively like SupportEventPropUtil.assertPropsEquals.
func assertEventObjectArrayCoreMetadata(env *esper.Environment) error {
	schema, ok := env.Schema("MyObjectArrayEvent")
	if !ok {
		return fmt.Errorf("preconfigured type MyObjectArrayEvent not found")
	}
	if schema.Kind() != esper.SchemaObjectArray {
		return fmt.Errorf("expected OBJECTARR application type, got %v", schema.Kind())
	}
	if schema.Name() != "MyObjectArrayEvent" {
		return fmt.Errorf("expected name MyObjectArrayEvent, got %q", schema.Name())
	}
	specs := schema.Fields()
	if len(specs) != 3 {
		return fmt.Errorf("expected three descriptors, got %d", len(specs))
	}
	byName := make(map[string]esper.FieldSpec, len(specs))
	for _, spec := range specs {
		if _, dup := byName[spec.Name]; dup {
			return fmt.Errorf("duplicate descriptor %q", spec.Name)
		}
		byName[spec.Name] = spec
	}
	for _, name := range []string{"myInt", "myString", "beanA"} {
		if _, ok := byName[name]; !ok {
			return fmt.Errorf("missing descriptor %q", name)
		}
	}
	if byName["myInt"].Type != reflect.TypeOf(int64(0)) {
		return fmt.Errorf("myInt type %v", byName["myInt"].Type)
	}
	if byName["myString"].Type != reflect.TypeOf("") {
		return fmt.Errorf("myString type %v", byName["myString"].Type)
	}
	if byName["beanA"].Type != reflect.TypeOf(oaComplexProps{}) {
		return fmt.Errorf("beanA type %v", byName["beanA"].Type)
	}
	if _, ok := schema.NestedSchema("beanA"); !ok {
		return fmt.Errorf("beanA must be a fragment")
	}
	return nil
}

// buildEventObjectArrayCore returns the Go query equivalent of one pinned
// deploy EPL. Property path expressions carry the verbatim Java property
// paths (beanA.indexed[1], lvl1s[0]) resolved by Event.Get at runtime —
// the same convention map-core uses for its non-dynamic schemas; select *
// from NBAL_1 projects its single declared val column.
func buildEventObjectArrayCore(env *esper.Environment, epl string) (esper.Query, error) {
	eventValue := esper.EventValue[esper.Event]()
	prop := func(path string) esper.Expression[any] {
		return esper.Property[any](eventValue, path)
	}
	switch epl {
	case eventObjectArrayCoreEPLNestedObjects:
		return esper.FromAny(env, "MyObjectArrayEvent").Window(esper.LengthWindow(5)).Select(
			esper.Alias("simple", prop("beanA.simpleProperty")),
			esper.Alias("nested", prop("beanA.nested.nestedValue")),
			esper.Alias("indexed", prop("beanA.indexed[1]")),
			esper.Alias("nestednested", prop("beanA.nested.nestedNested.nestedNestedValue")),
		).Query(esper.StatementName("s0")), nil
	case eventObjectArrayCoreEPLSelectNBAL1:
		return esper.FromAny(env, "NBAL_1").Select(
			esper.Alias("val", prop("val")),
		).Query(esper.StatementName("s0")), nil
	case eventObjectArrayCoreEPLSelectNBAL2:
		return esper.FromAny(env, "NBAL_2").Select(
			esper.Alias("c0", prop("lvl1s[0]")),
		).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("unsupported %s deploy EPL %q", eventObjectArrayCoreID, epl)
	}
}

// buildEventObjectArrayCoreError runs one expected-invalid probe: the Go
// build must reject the equivalent fluent plan before the compile-error
// record is emitted. The contract leaves the message text unpinned (Java's
// tryInvalidCompile "skip"), so the probe only requires a build rejection
// and the record carries a value only when the scenario pins expectError.
func buildEventObjectArrayCoreError(env *esper.Environment, step compat.Step, trace *compat.Trace, caseName string) error {
	if step.Epl != eventObjectArrayCoreProbeEPL(step.Statement) {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", eventObjectArrayCoreID, step.Statement, step.Epl)
	}
	var buildErr error
	switch step.Statement {
	case "unknown-property":
		_, buildErr = env.Build(esper.FromAny(env, "MyObjectArrayEvent").Window(esper.LengthWindow(5)).Select(
			esper.Alias("c0", esper.Field[any, any]("XXX")),
		).Query(esper.StatementName("s0")))
	case "string-arithmetic":
		_, buildErr = env.Build(esper.FromAny(env, "MyObjectArrayEvent").Window(esper.LengthWindow(5)).Select(
			esper.Alias("c0", esper.MultiplyOf[int64](esper.Field[any, string]("myString"), esper.Literal(int64(2)))),
		).Query(esper.StatementName("s0")))
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", eventObjectArrayCoreID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", eventObjectArrayCoreID, step.Statement)
	}
	record := compat.TraceRecord{
		Case:      caseName,
		Operation: "compile-error",
		Statement: step.Statement,
	}
	// Java omits the value field for "skip"-pinned probes; an empty string
	// would serialize as a present-but-empty value and diff against absent.
	if step.ExpectError != "" {
		record.Value = step.ExpectError
	}
	trace.Records = append(trace.Records, record)
	return nil
}

func eventObjectArrayCoreProbeEPL(statement string) string {
	switch statement {
	case "unknown-property":
		return eventObjectArrayCoreEPLProbeUnknown
	case "string-arithmetic":
		return eventObjectArrayCoreEPLProbeString
	default:
		return ""
	}
}

// sendEventObjectArrayCore decodes one positional send payload and delivers
// it through SendObjectArray. The {"_bean":"SupportBeanComplexProps"} tag
// reconstructs the makeDefaultBean mirror; nested arrays stay raw carriers
// (NBAL_2's lvl1s element is a plain Object[] on both sides).
func sendEventObjectArrayCore(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	var raw any
	if err := json.Unmarshal(step.Payload, &raw); err != nil {
		return fmt.Errorf("decode %s payload: %w", step.EventType, err)
	}
	payload, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("%s: %s payload must be a positional array", eventObjectArrayCoreID, step.EventType)
	}
	decoded, err := oaDecodeValue(payload)
	if err != nil {
		return err
	}
	values, ok := decoded.([]any)
	if !ok {
		return fmt.Errorf("%s: %s decoded payload is not positional", eventObjectArrayCoreID, step.EventType)
	}
	switch step.EventType {
	case "MyObjectArrayEvent", "NBAL_1", "NBAL_2":
		return engine.SendObjectArray(ctx, step.EventType, values)
	default:
		return fmt.Errorf("%s: unsupported event type %q", eventObjectArrayCoreID, step.EventType)
	}
}

// loadEventObjectArrayCoreScenario decodes the scenario with the
// strict-shape checks the raw-mutation tests pin: no duplicate JSON keys,
// the exact top-level field set, pinned case metadata, and per-step field
// whitelists so unknown or duplicated step fields fail the replay.
func loadEventObjectArrayCoreScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eventObjectArrayCoreID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eventObjectArrayCoreID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eventObjectArrayCoreID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eventObjectArrayCoreID, err)
	}
	if err := requireEventObjectArrayCoreFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eventObjectArrayCoreID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eventObjectArrayCoreID ||
		metadata.Description != eventObjectArrayCoreDescription ||
		metadata.JavaCommit != eventObjectArrayCoreJavaCommit ||
		metadata.JavaSource != eventObjectArrayCoreSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eventObjectArrayCoreID)
	}
	if err := validateEventObjectArrayCoreStringArray(root["javaRuntimes"], eventObjectArrayCoreJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEventObjectArrayCoreStringArray(root["javaNames"], eventObjectArrayCoreJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEventObjectArrayCoreStringArray(root["javaStaticIds"], eventObjectArrayCoreJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEventObjectArrayCoreStringArray(root["javaFlags"], eventObjectArrayCoreJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eventObjectArrayCoreCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", eventObjectArrayCoreID, len(eventObjectArrayCoreCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEventObjectArrayCoreFields(object,
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
		if definition.Case != eventObjectArrayCoreCases[index] ||
			definition.Ordinal != eventObjectArrayCoreOrdinals[index] ||
			definition.RuntimeID != eventObjectArrayCoreJavaRuntimeIDs[index] ||
			definition.ExecutionName != eventObjectArrayCoreJavaExecutions[index] ||
			definition.Observation != eventObjectArrayCoreCaseObservations[index] ||
			definition.EPL != eventObjectArrayCoreCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eventObjectArrayCoreID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", eventObjectArrayCoreID, err)
	}
	offset := 0
	for _, caseName := range eventObjectArrayCoreCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", eventObjectArrayCoreID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eventObjectArrayCoreID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", eventObjectArrayCoreID, offset, caseName)
		}
		// The case marker is a step too: run it through the field whitelist
		// so an unexpected field on the marker is rejected like any other
		// step's extra field.
		if _, err := eventObjectArrayCoreStepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eventObjectArrayCoreID, offset, err)
		}
		offset++
		want, ok := eventObjectArrayCoreCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", eventObjectArrayCoreID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", eventObjectArrayCoreID, caseName)
		}
		for _, pinned := range want {
			key, err := eventObjectArrayCoreStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eventObjectArrayCoreID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", eventObjectArrayCoreID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", eventObjectArrayCoreID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eventObjectArrayCoreID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// eventObjectArrayCoreStepKey renders one raw step as its pinned key:
// op|case|statement|eventType|epl|payload|expectError with the payload
// compacted. Unknown fields on the step object are rejected per op.
func eventObjectArrayCoreStepKey(raw json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return "", err
	}
	var step struct {
		Op        string          `json:"op"`
		Case      string          `json:"case"`
		Statement string          `json:"statement"`
		EventType string          `json:"eventType"`
		Epl       string          `json:"epl"`
		ExpectErr string          `json:"expectError"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"deployed":     {"op", "case", "statement"},
		"send":         {"op", "case", "eventType", "payload"},
		"undeploy":     {"op", "case", "statement"},
		"undeploy-all": {"op", "case"},
		"build-error":  {"op", "case", "statement", "epl", "expectError"},
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
		"|" + step.Epl + "|" + payloadText + "|" + step.ExpectErr, nil
}

func requireEventObjectArrayCoreFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", eventObjectArrayCoreID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", eventObjectArrayCoreID, name)
		}
	}
	return nil
}

func validateEventObjectArrayCoreStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}

// normalizeEventObjectArrayCoreTrace is the identity normalizer: every
// deploy attaches a single s0 statement and listener delivery is
// synchronous on the sending thread, so the dispatch order is already the
// canonical record order on both traces.
func normalizeEventObjectArrayCoreTrace(trace compat.Trace) compat.Trace {
	return trace
}
