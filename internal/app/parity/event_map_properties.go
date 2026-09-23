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
	eventMapPropertiesID         = "event-map-properties"
	eventMapPropertiesJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	eventMapPropertiesSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/map/EventMapProperties.java"
)

const eventMapPropertiesDescription = "EventMapProperties map-event property access (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c): indexed access over int[] and bean[] map properties (ord 0, EventMapArrayProperty), mapped ('key') access over untyped Map properties and a bean map getter (ord 1, EventMapMappedProperty), named-map nested navigation including the '?' dynamic-property variants (ord 2, EventMapMapNamePropertyNested) and named-map references with MyNamedMap[] elements (ord 3, EventMapMapNameProperty). All eight map types are Configuration-registered by TestSuiteEventMap (no create-schema EPL); every statement deploys as its own module, sends one event, then undeploys — undeployModuleContaining('s0') and undeployAll are the same boundary for a single-statement module. Java EventType assertions (Integer/Object/String/Map/Map[]/int[]/bean) are compile-time types in the Go fluent API and carry no trace records (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/map/EventMapProperties.java)."

var eventMapPropertiesCaseObservations = []string{
	"listener; two deploy/send/undeploy cycles: p0[0]/p0[1]/p1[0].intPrimitive/p1[1]/p0 over MyArrayMap{p0:int[],p1:SupportBean[]} yields a=1,b=2,c=5,d=bean(e2,6),e=[1,2,3], then the same columns prefixed outer. over MyArrayMapOuter{outer:<inline MyArrayMap def>} yields the same row (e selected, not value-asserted in Java)",
	"listener; three deploy/send/undeploy cycles: p0('k1') over MyMappedPropertyMap{p0:Map} yields a=v1, outer.p0('k1') over MyMappedPropertyMapOuter yields a=v1, outerTwo.mapProperty('xOne') over MyMappedPropertyMapOuterTwo{outerTwo:SupportBeanComplexProps} yields a=yOne from the makeDefaultBean payload",
	"listener; two deploy/send/undeploy cycles over MyArrayMapTwo{outer:<inline MyMapWithAMap def>}: outer.p0.n0/outer.p1[0].n0/outer.p1[1].n0/outer.p0/outer.p1 yields a=1,b=2,c=3,d={n0:1},e=[{n0:2},{n0:3}], then the '?' variants outer.p0.n0?/outer.p1[0].n0?/outer.p1[1]?.n0/outer.p0?/outer.p1? yield the identical row",
	"listener; one deploy/send/undeploy cycle over MyMapWithAMap{p0:MyNamedMap,p1:MyNamedMap[]}: p0.n0/p1[0].n0/p1[1].n0/p0/p1 yields a=1,b=2,c=3,d={n0:1},e=[{n0:2},{n0:3}]",
}

// Byte-exact EPL transcriptions of EventMapProperties.java: stmt1 line 40,
// stmt2 line 59 (ord 0); lines 82, 94, 105 (ord 1); lines 120, 142 (ord 2);
// line 158 (ord 3).
const (
	eventMapPropertiesEPLArray           = "@name('s0') select p0[0] as a, p0[1] as b, p1[0].intPrimitive as c, p1[1] as d, p0 as e from MyArrayMap"
	eventMapPropertiesEPLArrayOuter      = "@name('s0') select outer.p0[0] as a, outer.p0[1] as b, outer.p1[0].intPrimitive as c, outer.p1[1] as d, outer.p0 as e from MyArrayMapOuter"
	eventMapPropertiesEPLMapped          = "@name('s0') select p0('k1') as a from MyMappedPropertyMap"
	eventMapPropertiesEPLMappedOuter     = "@name('s0') select outer.p0('k1') as a from MyMappedPropertyMapOuter"
	eventMapPropertiesEPLMappedOuterTwo  = "@name('s0') select outerTwo.mapProperty('xOne') as a from MyMappedPropertyMapOuterTwo"
	eventMapPropertiesEPLNameNested      = "@name('s0') select outer.p0.n0 as a, outer.p1[0].n0 as b, outer.p1[1].n0 as c, outer.p0 as d, outer.p1 as e from MyArrayMapTwo"
	eventMapPropertiesEPLNameNestedOpt   = "@name('s0') select outer.p0.n0? as a, outer.p1[0].n0? as b, outer.p1[1]?.n0 as c, outer.p0? as d, outer.p1? as e from MyArrayMapTwo"
	eventMapPropertiesEPLMapNameProperty = "@name('s0') select p0.n0 as a, p1[0].n0 as b, p1[1].n0 as c, p0 as d, p1 as e from MyMapWithAMap"
)

// eventMapPropertiesCaseEPLs pins the EPL metadata of each case: multi-statement
// executions concatenate their statements with a trailing newline (the same
// convention as expr-filter-in-and-between); the single-statement case carries
// its one EPL verbatim.
var eventMapPropertiesCaseEPLs = []string{
	eventMapPropertiesEPLArray + "\n" + eventMapPropertiesEPLArrayOuter + "\n",
	eventMapPropertiesEPLMapped + "\n" + eventMapPropertiesEPLMappedOuter + "\n" + eventMapPropertiesEPLMappedOuterTwo + "\n",
	eventMapPropertiesEPLNameNested + "\n" + eventMapPropertiesEPLNameNestedOpt + "\n",
	eventMapPropertiesEPLMapNameProperty,
}

var (
	eventMapPropertiesJavaRuntimeIDs = []string{
		"java-runtime-cb09bcf76eee12b3c551",
		"java-runtime-088ce1203ecce0c8be3a",
		"java-runtime-b8c72b0931684bdc57dc",
		"java-runtime-14c459a77aca4647964b",
	}
	eventMapPropertiesJavaExecutions = []string{
		"EventMapArrayProperty",
		"EventMapMappedProperty",
		"EventMapMapNamePropertyNested",
		"EventMapMapNameProperty",
	}
	eventMapPropertiesJavaStaticIDs = []string{
		"java-368891e43058141edd4d",
		"java-0c40f754142dc89a2adf",
		"java-a138650435fe869f14c5",
		"java-1e1838567edaeb0850df",
	}
	eventMapPropertiesJavaFlags = []string{}
	eventMapPropertiesCases     = []string{
		"array-property",
		"mapped-property",
		"map-name-nested",
		"map-name",
	}
	eventMapPropertiesOrdinals = []int{0, 1, 2, 3}
	eventMapPropertiesSources  = []string{eventMapPropertiesSource}
)

// eventMapPropertiesCaseSteps pins the complete step sequence per case as
// op|case|statement|eventType|epl|payload keys so the loader asserts the
// scenario file matches the contract. Each Java statement is one
// compileDeploy/addListener/send/undeploy cycle; the deploy step carries the
// byte-exact EPL the Java oracle compiles and the undeploy step carries the
// statement whose module the boundary removes.
var eventMapPropertiesCaseSteps = map[string][]string{
	"array-property": {
		"deploy|array-property|s0||@name('s0') select p0[0] as a, p0[1] as b, p1[0].intPrimitive as c, p1[1] as d, p0 as e from MyArrayMap|",
		`send|array-property||MyArrayMap||{"p0":[1,2,3],"p1":[{"theString":"e1","intPrimitive":5},{"theString":"e2","intPrimitive":6}]}`,
		"undeploy|array-property|s0|||",
		"deploy|array-property|s0||@name('s0') select outer.p0[0] as a, outer.p0[1] as b, outer.p1[0].intPrimitive as c, outer.p1[1] as d, outer.p0 as e from MyArrayMapOuter|",
		`send|array-property||MyArrayMapOuter||{"outer":{"p0":[1,2,3],"p1":[{"theString":"e1","intPrimitive":5},{"theString":"e2","intPrimitive":6}]}}`,
		"undeploy|array-property|s0|||",
	},
	"mapped-property": {
		"deploy|mapped-property|s0||@name('s0') select p0('k1') as a from MyMappedPropertyMap|",
		`send|mapped-property||MyMappedPropertyMap||{"p0":{"k1":"v1"}}`,
		"undeploy|mapped-property|s0|||",
		"deploy|mapped-property|s0||@name('s0') select outer.p0('k1') as a from MyMappedPropertyMapOuter|",
		`send|mapped-property||MyMappedPropertyMapOuter||{"outer":{"p0":{"k1":"v1"}}}`,
		"undeploy|mapped-property|s0|||",
		"deploy|mapped-property|s0||@name('s0') select outerTwo.mapProperty('xOne') as a from MyMappedPropertyMapOuterTwo|",
		`send|mapped-property||MyMappedPropertyMapOuterTwo||{"outerTwo":{"simpleProperty":"simple","mapped":{"keyOne":"valueOne","keyTwo":"valueTwo"},"indexed":[1,2],"mapProperty":{"xOne":"yOne","xTwo":"yTwo"},"arrayProperty":[10,20,30],"nested":{"nestedValue":"nestedValue","nestedNested":{"nestedNestedValue":"nestedNestedValue"}}}}`,
		"undeploy|mapped-property|s0|||",
	},
	"map-name-nested": {
		"deploy|map-name-nested|s0||@name('s0') select outer.p0.n0 as a, outer.p1[0].n0 as b, outer.p1[1].n0 as c, outer.p0 as d, outer.p1 as e from MyArrayMapTwo|",
		`send|map-name-nested||MyArrayMapTwo||{"outer":{"p0":{"n0":1},"p1":[{"n0":2},{"n0":3}]}}`,
		"undeploy|map-name-nested|s0|||",
		"deploy|map-name-nested|s0||@name('s0') select outer.p0.n0? as a, outer.p1[0].n0? as b, outer.p1[1]?.n0 as c, outer.p0? as d, outer.p1? as e from MyArrayMapTwo|",
		`send|map-name-nested||MyArrayMapTwo||{"outer":{"p0":{"n0":1},"p1":[{"n0":2},{"n0":3}]}}`,
		"undeploy|map-name-nested|s0|||",
	},
	"map-name": {
		"deploy|map-name|s0||@name('s0') select p0.n0 as a, p1[0].n0 as b, p1[1].n0 as c, p0 as d, p1 as e from MyMapWithAMap|",
		`send|map-name||MyMapWithAMap||{"p0":{"n0":1},"p1":[{"n0":2},{"n0":3}]}`,
		"undeploy|map-name|s0|||",
	},
}

// empSupportBean mirrors SupportBean's asserted fields (theString,
// intPrimitive). theString is a *string so a null renders as {state:null}
// exactly like the Java bean; the json tags pin the bean column's trace
// rendering to the Java property names.
type empSupportBean struct {
	TheString    *string `esper:"theString" json:"theString"`
	IntPrimitive int     `esper:"intPrimitive" json:"intPrimitive"`
}

// empComplexNestedNested mirrors SupportBeanSpecialGetterNestedNested.
type empComplexNestedNested struct {
	NestedNestedValue string `esper:"nestedNestedValue" json:"nestedNestedValue"`
}

// empComplexNested mirrors SupportBeanSpecialGetterNested.
type empComplexNested struct {
	NestedValue  string                 `esper:"nestedValue" json:"nestedValue"`
	NestedNested empComplexNestedNested `esper:"nestedNested" json:"nestedNested"`
}

// empComplexProps mirrors SupportBeanComplexProps.makeDefaultBean(): the six
// payload-pinned readable properties (objectArray stays null and is never
// sent). Only mapProperty is navigated (ord 1 stmt 3); the full field set is
// carried so the sent bean matches the Java makeDefaultBean shape.
type empComplexProps struct {
	SimpleProperty string            `esper:"simpleProperty" json:"simpleProperty"`
	Mapped         map[string]string `esper:"mapped" json:"mapped"`
	Indexed        []int             `esper:"indexed" json:"indexed"`
	MapProperty    map[string]string `esper:"mapProperty" json:"mapProperty"`
	ArrayProperty  []int             `esper:"arrayProperty" json:"arrayProperty"`
	Nested         empComplexNested  `esper:"nested" json:"nested"`
}

func runEventMapPropertiesScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eventMapPropertiesCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEventMapPropertiesCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eventMapPropertiesID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", eventMapPropertiesID, scenario.ID)
	}
	return trace, nil
}

// runEventMapPropertiesCase replays one execution on a fresh
// environment/engine pair, mirroring the Java execution's fresh runtime: the
// eight TestSuiteEventMap map types register up front, then each deploy step
// builds the pinned plan and attaches the s0 listener (compileDeploy +
// addListener("s0")), each send delivers one map event, and each undeploy
// step removes the statement's module (undeployAll/undeployModuleContaining
// are the same boundary for a single-statement module).
func runEventMapPropertiesCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if err := registerEventMapPropertiesTypes(env); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(eventMapPropertiesJavaRuntimeIDs[eventMapPropertiesOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	sequences := make(map[string]uint64)
	deployments := map[string]*esper.Deployment{}

	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "deploy":
			plan, err := buildEventMapProperties(env, step.Epl)
			if err != nil {
				return trace, fmt.Errorf("build %q/%q: %w", caseName, step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return trace, fmt.Errorf("deploy %q/%q: %w", caseName, step.Statement, err)
			}
			deployments[step.Statement] = deployment
			for _, statement := range deployment.Statements() {
				stmt := statement
				if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					if len(batch.New) == 0 && len(batch.Old) == 0 {
						return nil
					}
					sequences[stmt.Name()]++
					trace.Records = append(trace.Records, compat.TraceRecord{
						Case:      caseName,
						Operation: "listener",
						Statement: stmt.Name(),
						Sequence:  sequences[stmt.Name()],
						Time:      compat.FormatTraceTime(batch.Time),
						New:       compat.NormalizeResults(batch.New),
						Old:       compat.NormalizeResults(batch.Old),
					})
					return nil
				}); err != nil {
					return trace, err
				}
			}
		case "send":
			record, err := decodeEventMapPropertiesPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.SendRecord(ctx, step.EventType, record); err != nil {
				return trace, fmt.Errorf("send %s: %w", step.EventType, err)
			}
		case "undeploy":
			// undeployModuleContaining/undeployAll: every statement deploys as
			// its own module, so the statement key selects the whole deployment.
			deployment, ok := deployments[step.Statement]
			if !ok {
				return trace, fmt.Errorf("undeploy %q/%q: no deployment", caseName, step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return trace, fmt.Errorf("undeploy %q/%q: %w", caseName, step.Statement, err)
			}
			delete(deployments, step.Statement)
		default:
			return trace, fmt.Errorf("%s: unsupported step op %q", eventMapPropertiesID, step.Op)
		}
	}
	return trace, nil
}

func eventMapPropertiesOrdinal(caseName string) int {
	for index, name := range eventMapPropertiesCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// registerEventMapPropertiesTypes mirrors the TestSuiteEventMap.configure
// registrations this suite uses: MyNamedMap{n0:int}; MyMapWithAMap with the
// named-map references p0:"MyNamedMap" and p1:"MyNamedMap[]" (any-typed
// properties carrying the named schema as their nested fragment);
// MyArrayMap{p0:int[],p1:SupportBean[]}; MyArrayMapOuter and MyArrayMapTwo
// whose outer property is an inline anonymous map definition (Go reuses the
// structurally identical registered schema; Java shares the arrayDef/mappedDef
// map objects while MyArrayMapTwo's inline def is a structurally identical
// fresh map); MyMappedPropertyMap{p0:Map} and its Outer
// inline wrapper; MyMappedPropertyMapOuterTwo{outerTwo:SupportBeanComplexProps}.
// Java registers MyNamedMap/MyMapWithAMap twice identically; the
// re-registration is idempotent and registers once here.
func registerEventMapPropertiesTypes(env *esper.Environment) error {
	anyT := reflect.TypeOf(any(nil))
	intT := reflect.TypeOf(int(0))
	intSliceT := reflect.TypeOf([]int{})
	beanSliceT := reflect.TypeOf([]empSupportBean{})
	mapT := reflect.TypeOf(map[string]any{})
	complexT := reflect.TypeOf(empComplexProps{})

	namedMap, err := esper.RegisterMap(env, "MyNamedMap", []esper.FieldSpec{
		{Name: "n0", Type: intT},
	})
	if err != nil {
		return fmt.Errorf("register MyNamedMap: %w", err)
	}
	mapWithAMap, err := esper.RegisterMap(env, "MyMapWithAMap", []esper.FieldSpec{
		{Name: "p0", Type: anyT},
		{Name: "p1", Type: anyT},
	},
		esper.WithNestedPropertySchema("p0", namedMap),
		esper.WithNestedPropertySchema("p1", namedMap))
	if err != nil {
		return fmt.Errorf("register MyMapWithAMap: %w", err)
	}
	arrayMap, err := esper.RegisterMap(env, "MyArrayMap", []esper.FieldSpec{
		{Name: "p0", Type: intSliceT},
		{Name: "p1", Type: beanSliceT},
	})
	if err != nil {
		return fmt.Errorf("register MyArrayMap: %w", err)
	}
	if _, err := esper.RegisterMap(env, "MyArrayMapOuter", []esper.FieldSpec{
		{Name: "outer", Type: anyT},
	}, esper.WithNestedPropertySchema("outer", arrayMap)); err != nil {
		return fmt.Errorf("register MyArrayMapOuter: %w", err)
	}
	mappedMap, err := esper.RegisterMap(env, "MyMappedPropertyMap", []esper.FieldSpec{
		{Name: "p0", Type: mapT},
	})
	if err != nil {
		return fmt.Errorf("register MyMappedPropertyMap: %w", err)
	}
	if _, err := esper.RegisterMap(env, "MyMappedPropertyMapOuter", []esper.FieldSpec{
		{Name: "outer", Type: anyT},
	}, esper.WithNestedPropertySchema("outer", mappedMap)); err != nil {
		return fmt.Errorf("register MyMappedPropertyMapOuter: %w", err)
	}
	if _, err := esper.RegisterMap(env, "MyMappedPropertyMapOuterTwo", []esper.FieldSpec{
		{Name: "outerTwo", Type: complexT},
	}); err != nil {
		return fmt.Errorf("register MyMappedPropertyMapOuterTwo: %w", err)
	}
	if _, err := esper.RegisterMap(env, "MyArrayMapTwo", []esper.FieldSpec{
		{Name: "outer", Type: anyT},
	}, esper.WithNestedPropertySchema("outer", mapWithAMap)); err != nil {
		return fmt.Errorf("register MyArrayMapTwo: %w", err)
	}
	return nil
}

// buildEventMapProperties returns the Go plan equivalent of one pinned deploy
// EPL. Every select projects Property path expressions over the event value:
// [N] indexed access, ('key') mapped access, dotted navigation and the '?'
// optional-segment marker all parse through the property-path engine.
func buildEventMapProperties(env *esper.Environment, epl string) (esper.Plan, error) {
	eventValue := esper.EventValue[esper.Event]()
	prop := func(path string) esper.Expression[any] {
		return esper.Property[any](eventValue, path)
	}
	switch epl {
	case eventMapPropertiesEPLArray:
		return env.Build(esper.FromAny(env, "MyArrayMap").Select(
			esper.Alias("a", prop("p0[0]")),
			esper.Alias("b", prop("p0[1]")),
			esper.Alias("c", prop("p1[0].intPrimitive")),
			esper.Alias("d", prop("p1[1]")),
			esper.Alias("e", prop("p0")),
		).Query(esper.StatementName("s0")))
	case eventMapPropertiesEPLArrayOuter:
		return env.Build(esper.FromAny(env, "MyArrayMapOuter").Select(
			esper.Alias("a", prop("outer.p0[0]")),
			esper.Alias("b", prop("outer.p0[1]")),
			esper.Alias("c", prop("outer.p1[0].intPrimitive")),
			esper.Alias("d", prop("outer.p1[1]")),
			esper.Alias("e", prop("outer.p0")),
		).Query(esper.StatementName("s0")))
	case eventMapPropertiesEPLMapped:
		return env.Build(esper.FromAny(env, "MyMappedPropertyMap").Select(
			esper.Alias("a", prop("p0('k1')")),
		).Query(esper.StatementName("s0")))
	case eventMapPropertiesEPLMappedOuter:
		return env.Build(esper.FromAny(env, "MyMappedPropertyMapOuter").Select(
			esper.Alias("a", prop("outer.p0('k1')")),
		).Query(esper.StatementName("s0")))
	case eventMapPropertiesEPLMappedOuterTwo:
		return env.Build(esper.FromAny(env, "MyMappedPropertyMapOuterTwo").Select(
			esper.Alias("a", prop("outerTwo.mapProperty('xOne')")),
		).Query(esper.StatementName("s0")))
	case eventMapPropertiesEPLNameNested:
		return env.Build(esper.FromAny(env, "MyArrayMapTwo").Select(
			esper.Alias("a", prop("outer.p0.n0")),
			esper.Alias("b", prop("outer.p1[0].n0")),
			esper.Alias("c", prop("outer.p1[1].n0")),
			esper.Alias("d", prop("outer.p0")),
			esper.Alias("e", prop("outer.p1")),
		).Query(esper.StatementName("s0")))
	case eventMapPropertiesEPLNameNestedOpt:
		return env.Build(esper.FromAny(env, "MyArrayMapTwo").Select(
			esper.Alias("a", prop("outer.p0.n0?")),
			esper.Alias("b", prop("outer.p1[0].n0?")),
			esper.Alias("c", prop("outer.p1[1]?.n0")),
			esper.Alias("d", prop("outer.p0?")),
			esper.Alias("e", prop("outer.p1?")),
		).Query(esper.StatementName("s0")))
	case eventMapPropertiesEPLMapNameProperty:
		return env.Build(esper.FromAny(env, "MyMapWithAMap").Select(
			esper.Alias("a", prop("p0.n0")),
			esper.Alias("b", prop("p1[0].n0")),
			esper.Alias("c", prop("p1[1].n0")),
			esper.Alias("d", prop("p0")),
			esper.Alias("e", prop("p1")),
		).Query(esper.StatementName("s0")))
	default:
		return esper.Plan{}, fmt.Errorf("unsupported %s deploy EPL %q", eventMapPropertiesID, epl)
	}
}

// decodeEventMapPropertiesPayload rebuilds one pinned send payload with the
// Java value shapes: int[] -> []int, SupportBean[] -> []empSupportBean,
// Map -> map[string]any, Map[] -> []map[string]any and
// SupportBeanComplexProps -> empComplexProps. All schema fields are any-typed
// or already-typed so SendRecord passes the decoded values through.
func decodeEventMapPropertiesPayload(step compat.Step) (map[string]any, error) {
	var raw map[string]json.RawMessage
	if err := strictObject(step.Payload, &raw); err != nil {
		return nil, fmt.Errorf("decode %s payload: %w", step.EventType, err)
	}
	switch step.EventType {
	case "MyArrayMap":
		return decodeEventMapPropertiesArrayMap(raw)
	case "MyArrayMapOuter":
		return decodeEventMapPropertiesOuter(raw, "outer", decodeEventMapPropertiesArrayMap)
	case "MyMappedPropertyMap":
		return decodeEventMapPropertiesMapped(raw)
	case "MyMappedPropertyMapOuter":
		return decodeEventMapPropertiesOuter(raw, "outer", decodeEventMapPropertiesMapped)
	case "MyMappedPropertyMapOuterTwo":
		if len(raw) != 1 || raw["outerTwo"] == nil {
			return nil, fmt.Errorf("%s payload must carry only outerTwo", step.EventType)
		}
		var bean empComplexProps
		if err := json.Unmarshal(raw["outerTwo"], &bean); err != nil {
			return nil, fmt.Errorf("decode outerTwo: %w", err)
		}
		return map[string]any{"outerTwo": bean}, nil
	case "MyArrayMapTwo":
		return decodeEventMapPropertiesOuter(raw, "outer", decodeEventMapPropertiesMapWithAMap)
	case "MyMapWithAMap":
		return decodeEventMapPropertiesMapWithAMap(raw)
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", eventMapPropertiesID, step.EventType)
	}
}

// decodeEventMapPropertiesOuter unwraps the single-key outer envelope and
// decodes the nested map definition with the inner decoder.
func decodeEventMapPropertiesOuter(raw map[string]json.RawMessage, key string,
	inner func(map[string]json.RawMessage) (map[string]any, error)) (map[string]any, error) {
	if len(raw) != 1 || raw[key] == nil {
		return nil, fmt.Errorf("outer payload must carry only %s", key)
	}
	var nested map[string]json.RawMessage
	if err := strictObject(raw[key], &nested); err != nil {
		return nil, fmt.Errorf("decode %s: %w", key, err)
	}
	value, err := inner(nested)
	if err != nil {
		return nil, err
	}
	return map[string]any{key: value}, nil
}

// decodeEventMapPropertiesArrayMap rebuilds {p0:int[], p1:SupportBean[]}.
func decodeEventMapPropertiesArrayMap(raw map[string]json.RawMessage) (map[string]any, error) {
	record := make(map[string]any, len(raw))
	for key, value := range raw {
		switch key {
		case "p0":
			var items []int
			if err := json.Unmarshal(value, &items); err != nil {
				return nil, fmt.Errorf("decode p0: %w", err)
			}
			record[key] = items
		case "p1":
			var items []empSupportBean
			if err := json.Unmarshal(value, &items); err != nil {
				return nil, fmt.Errorf("decode p1: %w", err)
			}
			record[key] = items
		default:
			return nil, fmt.Errorf("unexpected MyArrayMap field %q", key)
		}
	}
	return record, nil
}

// decodeEventMapPropertiesMapped rebuilds {p0:Map} with the untyped map value.
func decodeEventMapPropertiesMapped(raw map[string]json.RawMessage) (map[string]any, error) {
	record := make(map[string]any, len(raw))
	for key, value := range raw {
		if key != "p0" {
			return nil, fmt.Errorf("unexpected MyMappedPropertyMap field %q", key)
		}
		var nested map[string]any
		if err := json.Unmarshal(value, &nested); err != nil {
			return nil, fmt.Errorf("decode p0: %w", err)
		}
		record[key] = nested
	}
	return record, nil
}

// decodeEventMapPropertiesMapWithAMap rebuilds {p0:MyNamedMap, p1:MyNamedMap[]}
// with plain map values, mirroring Java's Map/Map[] underlying objects.
func decodeEventMapPropertiesMapWithAMap(raw map[string]json.RawMessage) (map[string]any, error) {
	record := make(map[string]any, len(raw))
	for key, value := range raw {
		switch key {
		case "p0":
			named, err := decodeEventMapPropertiesNamedMap(value)
			if err != nil {
				return nil, fmt.Errorf("decode p0: %w", err)
			}
			record[key] = named
		case "p1":
			var items []json.RawMessage
			if err := json.Unmarshal(value, &items); err != nil {
				return nil, fmt.Errorf("decode p1: %w", err)
			}
			named := make([]map[string]any, 0, len(items))
			for index, item := range items {
				element, err := decodeEventMapPropertiesNamedMap(item)
				if err != nil {
					return nil, fmt.Errorf("decode p1[%d]: %w", index, err)
				}
				named = append(named, element)
			}
			record[key] = named
		default:
			return nil, fmt.Errorf("unexpected MyMapWithAMap field %q", key)
		}
	}
	return record, nil
}

// decodeEventMapPropertiesNamedMap rebuilds one MyNamedMap{n0:int} value.
func decodeEventMapPropertiesNamedMap(raw json.RawMessage) (map[string]any, error) {
	var fields map[string]int
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	named := make(map[string]any, len(fields))
	for key, value := range fields {
		named[key] = value
	}
	return named, nil
}

// loadEventMapPropertiesScenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned case metadata, and per-step field whitelists so
// unknown or duplicated step fields fail the replay.
func loadEventMapPropertiesScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eventMapPropertiesID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eventMapPropertiesID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eventMapPropertiesID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eventMapPropertiesID, err)
	}
	if err := requireEventMapPropertiesFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eventMapPropertiesID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eventMapPropertiesID ||
		metadata.Description != eventMapPropertiesDescription ||
		metadata.JavaCommit != eventMapPropertiesJavaCommit ||
		metadata.JavaSource != eventMapPropertiesSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eventMapPropertiesID)
	}
	if err := validateEventMapPropertiesStringArray(root["javaRuntimes"], eventMapPropertiesJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEventMapPropertiesStringArray(root["javaNames"], eventMapPropertiesJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEventMapPropertiesStringArray(root["javaStaticIds"], eventMapPropertiesJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEventMapPropertiesStringArray(root["javaFlags"], eventMapPropertiesJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eventMapPropertiesCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", eventMapPropertiesID, len(eventMapPropertiesCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEventMapPropertiesFields(object,
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
		if definition.Case != eventMapPropertiesCases[index] ||
			definition.Ordinal != eventMapPropertiesOrdinals[index] ||
			definition.RuntimeID != eventMapPropertiesJavaRuntimeIDs[index] ||
			definition.ExecutionName != eventMapPropertiesJavaExecutions[index] ||
			definition.Observation != eventMapPropertiesCaseObservations[index] ||
			definition.EPL != eventMapPropertiesCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eventMapPropertiesID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", eventMapPropertiesID, err)
	}
	offset := 0
	for _, caseName := range eventMapPropertiesCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", eventMapPropertiesID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eventMapPropertiesID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", eventMapPropertiesID, offset, caseName)
		}
		if _, err := eventMapPropertiesStepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eventMapPropertiesID, offset, err)
		}
		offset++
		want, ok := eventMapPropertiesCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", eventMapPropertiesID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", eventMapPropertiesID, caseName)
		}
		for _, pinned := range want {
			key, err := eventMapPropertiesStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eventMapPropertiesID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", eventMapPropertiesID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", eventMapPropertiesID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eventMapPropertiesID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// eventMapPropertiesStepKey renders one raw step as its pinned key:
// op|case|statement|eventType|epl|payload with the payload compacted. Unknown
// fields on the step object are rejected per op.
func eventMapPropertiesStepKey(raw json.RawMessage) (string, error) {
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
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string][]string{
		"case":     {"op", "case"},
		"deploy":   {"op", "case", "statement", "epl"},
		"send":     {"op", "case", "eventType", "payload"},
		"undeploy": {"op", "case", "statement"},
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
		"|" + step.Epl + "|" + payloadText, nil
}

func requireEventMapPropertiesFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", eventMapPropertiesID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", eventMapPropertiesID, name)
		}
	}
	return nil
}

func validateEventMapPropertiesStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}

// normalizeEventMapPropertiesTrace is the identity normalizer: every deploy
// attaches a single s0 statement and listener delivery is synchronous on the
// sending thread, so the dispatch order is already the canonical record order
// on both traces.
func normalizeEventMapPropertiesTrace(trace compat.Trace) compat.Trace {
	return trace
}
