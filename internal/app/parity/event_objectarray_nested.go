package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EventObjectArrayEventNested (ords 0-4) and
// EventObjectArrayEventNestedPojo (ord 0): object-array event nested
// property access — indexed/mapped/dynamic paths, named-map properties,
// nested OA types, deep mixed bean/map graphs and a POJO anyOf filter.
// Approved differences: Go has no EPL text; Field path strings carry the
// verbatim Java property paths (p0[0], outer.p1[0].n0, nodefmap.key3?.key4)
// resolved by Schema.get at runtime — except Java's mapped('k') accessor,
// which maps to Go's mapprop('k') path on the bean's map property;
// '?' optional access is wrapped in
// CoalesceOf so Java null and Go Missing both render {state:null}; the
// Java assertIterator on #lastevent maps to statement.Snapshot; bean
// underlyings render through the runner's deterministic renderer (Java
// oracle renders the same shapes).
var eventObjectArrayNestedJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/objectarray/EventObjectArrayEventNested.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/objectarray/EventObjectArrayEventNestedPojo.java",
}

const eventObjectArrayNestedID = "event-objectarray-nested"
const eventObjectArrayNestedJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eventObjectArrayNestedJavaRuntimeIDs = []string{
	"java-runtime-1704d5718125f4502592",
	"java-runtime-072c48af61cdd8b27856",
	"java-runtime-5a08e705396c651e2159",
	"java-runtime-7d6eb46f34c9d597b7d1",
	"java-runtime-957493fa76a9e36a6091",
	"java-runtime-57d2cf8ae645bedf06e6",
}

var eventObjectArrayNestedJavaExecutions = []string{
	"EventObjectArrayArrayProperty",
	"EventObjectArrayMappedProperty",
	"EventObjectArrayMapNamePropertyNested",
	"EventObjectArrayMapNameProperty",
	"EventObjectArrayObjectArrayNested",
	"EventObjectArrayEventNestedPojo",
}

// Bean mirrors. Field names carry the Java property names via esper tags;
// the renderer reads the tags so trace fields match the oracle's normalize().

type oaSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

type oaBeanA struct {
	ID string `esper:"id"`
}

type oaBeanB struct {
	ID string `esper:"id"`
}

type oaComplexNestedNested struct {
	NestedNestedValue string `esper:"nestedNestedValue"`
}

type oaComplexNested struct {
	NestedValue  string                `esper:"nestedValue"`
	NestedNested oaComplexNestedNested `esper:"nestedNested"`
}

// oaComplexProps mirrors SupportBeanComplexProps.makeDefaultBean().
type oaComplexProps struct {
	SimpleProperty string            `esper:"simpleProperty"`
	MapProperty    map[string]string `esper:"mapProperty"`
	Indexed        []int64           `esper:"indexed"`
	ArrayProperty  []int64           `esper:"arrayProperty"`
	Nested         oaComplexNested   `esper:"nested"`
}

func oaDefaultComplexProps() oaComplexProps {
	return oaComplexProps{
		SimpleProperty: "simple",
		MapProperty:    map[string]string{"xOne": "yOne", "xTwo": "yTwo"},
		Indexed:        []int64{1, 2},
		ArrayProperty:  []int64{10, 20, 30},
		Nested: oaComplexNested{
			NestedValue:  "nestedValue",
			NestedNested: oaComplexNestedNested{NestedNestedValue: "nestedNestedValue"},
		},
	}
}

type oaNestedLevTwo struct {
	Value string `esper:"value"`
}

type oaNestedLevOne struct {
	Mapprop       map[string]oaNestedLevTwo `esper:"mapprop"`
	NestLevOneVal string                    `esper:"nestLevOneVal"`
}

// oaCombinedProps mirrors SupportBeanCombinedProps.makeDefaultBean():
// getArray() returns the same slice as getIndexed(int).
type oaCombinedProps struct {
	Indexed []*oaNestedLevOne `esper:"indexed"`
	Array   []*oaNestedLevOne `esper:"array"`
}

func oaDefaultCombinedProps() oaCombinedProps {
	indexed := []*oaNestedLevOne{
		{Mapprop: map[string]oaNestedLevTwo{"0ma": {Value: "0ma0"}, "0mb": {Value: "0ma1"}}, NestLevOneVal: "abc"},
		{Mapprop: map[string]oaNestedLevTwo{"1ma": {Value: "1ma0"}, "1mb": {Value: "1ma1"}}, NestLevOneVal: "abc"},
		{Mapprop: map[string]oaNestedLevTwo{"2ma": {Value: "valueOne"}, "2mb": {Value: "2ma1"}}, NestLevOneVal: "abc"},
		nil,
	}
	return oaCombinedProps{Indexed: indexed, Array: indexed}
}

type oaMyInside struct {
	ID string `esper:"id"`
}

type oaMyNested struct {
	Insides []oaMyInside `esper:"insides"`
}

func runEventObjectArrayNestedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range []string{
		"array-property", "mapped-property", "map-name-nested",
		"map-name", "oa-nested", "pojo",
	} {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		caseTrace, err := runEventObjectArrayNestedCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("scenario has no supported cases")
	}
	return trace, nil
}

func runEventObjectArrayNestedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if err := oaRegisterSchemas(env); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seq := uint64(0)
	var deployment *esper.Deployment
	deploy := func(epl string) error {
		query, err := oaQueryForEPL(env, caseName, epl)
		if err != nil {
			return err
		}
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		deployment, err = engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		for _, st := range deployment.Statements() {
			st := st
			if caseName == "oa-nested" {
				// Java asserts the iterator only; no listener is attached.
				continue
			}
			if _, subErr := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				seq++
				record := compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: st.Name(),
					Sequence:  seq,
					Time:      "1970-01-01T00:00:00Z",
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
			}); subErr != nil {
				return subErr
			}
		}
		return nil
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
		case "deploy":
			if step.Epl == "" {
				return trace, fmt.Errorf("%s deploy step without pinned epl", eventObjectArrayNestedID)
			}
			if err := deploy(step.Epl); err != nil {
				return trace, fmt.Errorf("%s deploy %q: %w", eventObjectArrayNestedID, step.Epl, err)
			}
		case "undeploy-all":
			if deployment != nil {
				if err := deployment.Undeploy(ctx); err != nil {
					return trace, fmt.Errorf("undeploy %q: %w", caseName, err)
				}
				deployment = nil
			}
		case "send":
			if err := oaSend(ctx, engine, step); err != nil {
				return trace, err
			}
		case "snapshot":
			if deployment == nil {
				return trace, fmt.Errorf("%s snapshot without deployment", eventObjectArrayNestedID)
			}
			result, err := deployment.Statements()[0].Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			var nested map[string]esper.Schema
			if caseName == "oa-nested" {
				// The projection row does not carry the registered nested
				// schema; render p0 positionally against TypeP0.
				if schema, ok := env.Schema("TypeLev0"); ok {
					nested = map[string]esper.Schema{"p0": schema}
				}
			}
			rows, err := oaResultRows(result.Batch.New, nested)
			if err != nil {
				return trace, err
			}
			if rows == nil {
				rows = []compat.ResultRecord{}
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: "s0",
				Sequence:  0,
				Time:      engine.Now().UTC().Format(time.RFC3339Nano),
				New:       rows,
			})
		default:
			return trace, fmt.Errorf("%s unsupported step op %q", eventObjectArrayNestedID, step.Op)
		}
	}
	return trace, nil
}

// oaRegisterSchemas declares every event type the six cases touch. Nested
// object-array and named-map properties attach their schemas via
// WithNestedPropertySchema; AllowDynamicFields lets Field path strings
// (p0[0], outer.p1[0].n0, nodefmap.key3?.key4) pass build-time validation —
// Schema.get resolves them at runtime.
func oaRegisterSchemas(env *esper.Environment) error {
	anyType := reflect.TypeOf((*any)(nil)).Elem()
	mapType := reflect.TypeOf(map[string]any(nil))

	regOA := func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) error {
		_, err := esper.RegisterObjectArray(env, name, fields, opts...)
		return err
	}
	regMap := func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) error {
		_, err := esper.RegisterMap(env, name, fields, opts...)
		return err
	}

	// array-property
	myArrayOA, err := esper.NewObjectArraySchema("MyArrayOA", []esper.FieldSpec{
		esper.FieldDef("p0", anyType),
		esper.FieldDef("p1", anyType),
	}, esper.AllowDynamicFields())
	if err != nil {
		return err
	}
	if err := env.RegisterSchema(myArrayOA); err != nil {
		return err
	}
	outerOA, err := esper.NewObjectArraySchema("MyArrayOAMapOuter", []esper.FieldSpec{
		esper.FieldDef("outer", anyType),
	}, esper.WithNestedPropertySchema("outer", myArrayOA), esper.AllowDynamicFields())
	if err != nil {
		return err
	}
	if err := env.RegisterSchema(outerOA); err != nil {
		return err
	}

	// mapped-property (map event types declared from Map defs)
	if err := regMap("MyMappedPropertyMap", []esper.FieldSpec{
		esper.FieldDef("p0", mapType),
	}, esper.AllowDynamicFields()); err != nil {
		return err
	}
	mappedInner, err := esper.NewMapSchema("MyMappedPropertyMapInner", []esper.FieldSpec{
		esper.FieldDef("p0", mapType),
	}, esper.AllowDynamicFields())
	if err != nil {
		return err
	}
	if err := regMap("MyMappedPropertyMapOuter", []esper.FieldSpec{
		esper.FieldDef("outer", mapType),
	}, esper.WithNestedPropertySchema("outer", mappedInner), esper.AllowDynamicFields()); err != nil {
		return err
	}
	if err := regMap("MyMappedPropertyMapOuterTwo", []esper.FieldSpec{
		esper.FieldDef("outerTwo", reflect.TypeOf(oaComplexProps{})),
	}, esper.AllowDynamicFields()); err != nil {
		return err
	}

	// map-name-nested / map-name
	if err := regMap("MyNamedMap", []esper.FieldSpec{
		esper.FieldDef("n0", reflect.TypeOf(int64(0))),
	}, esper.AllowDynamicFields()); err != nil {
		return err
	}
	namedMap, _ := env.Schema("MyNamedMap")
	mapOuterInner, err := esper.NewMapSchema("MyObjectArrayMapOuterInner", []esper.FieldSpec{
		esper.FieldDef("p0", anyType),
		esper.FieldDef("p1", anyType),
	},
		esper.WithNestedPropertySchema("p0", namedMap),
		esper.WithNestedPropertySchema("p1", namedMap),
		esper.AllowDynamicFields())
	if err != nil {
		return err
	}
	if err := regOA("MyObjectArrayMapOuter", []esper.FieldSpec{
		esper.FieldDef("outer", mapType),
	}, esper.WithNestedPropertySchema("outer", mapOuterInner), esper.AllowDynamicFields()); err != nil {
		return err
	}
	if err := regOA("MyOAWithAMap", []esper.FieldSpec{
		esper.FieldDef("p0", anyType),
		esper.FieldDef("p1", anyType),
	},
		esper.WithNestedPropertySchema("p0", namedMap),
		esper.WithNestedPropertySchema("p1", namedMap),
		esper.AllowDynamicFields()); err != nil {
		return err
	}

	// oa-nested
	lev1, err := esper.NewObjectArraySchema("TypeLev1", []esper.FieldSpec{
		esper.FieldDef("p1id", reflect.TypeOf(int64(0))),
	}, esper.AllowDynamicFields())
	if err != nil {
		return err
	}
	if err := env.RegisterSchema(lev1); err != nil {
		return err
	}
	lev0, err := esper.NewObjectArraySchema("TypeLev0", []esper.FieldSpec{
		esper.FieldDef("p0id", reflect.TypeOf(int64(0))),
		esper.FieldDef("p1", anyType),
	}, esper.WithNestedPropertySchema("p1", lev1), esper.AllowDynamicFields())
	if err != nil {
		return err
	}
	if err := env.RegisterSchema(lev0); err != nil {
		return err
	}
	if err := regOA("TypeRoot", []esper.FieldSpec{
		esper.FieldDef("rootId", reflect.TypeOf(int64(0))),
		esper.FieldDef("p0", anyType),
	}, esper.WithNestedPropertySchema("p0", lev0), esper.AllowDynamicFields()); err != nil {
		return err
	}

	// pojo: NestedObjectArr with a three-level anonymous nested map def.
	levelThree, err := esper.NewMapSchema("NestedObjectArrLevelThree", []esper.FieldSpec{
		esper.FieldDef("simpleThree", reflect.TypeOf(int64(0))),
		esper.FieldDef("objectThree", reflect.TypeOf(oaBeanB{})),
	}, esper.AllowDynamicFields())
	if err != nil {
		return err
	}
	levelTwo, err := esper.NewMapSchema("NestedObjectArrLevelTwo", []esper.FieldSpec{
		esper.FieldDef("simpleTwo", reflect.TypeOf(int64(0))),
		esper.FieldDef("objectTwo", reflect.TypeOf(oaCombinedProps{})),
		esper.FieldDef("nodefmapTwo", mapType),
		esper.FieldDef("mapTwo", mapType),
	}, esper.WithNestedPropertySchema("mapTwo", levelThree), esper.AllowDynamicFields())
	if err != nil {
		return err
	}
	levelOne, err := esper.NewMapSchema("NestedObjectArrLevelOne", []esper.FieldSpec{
		esper.FieldDef("simpleOne", reflect.TypeOf(int64(0))),
		esper.FieldDef("objectOne", reflect.TypeOf(oaComplexProps{})),
		esper.FieldDef("nodefmapOne", mapType),
		esper.FieldDef("mapOne", mapType),
	}, esper.WithNestedPropertySchema("mapOne", levelTwo), esper.AllowDynamicFields())
	if err != nil {
		return err
	}
	if err := regOA("NestedObjectArr", []esper.FieldSpec{
		esper.FieldDef("simple", reflect.TypeOf("")),
		esper.FieldDef("object", reflect.TypeOf(oaBeanA{})),
		esper.FieldDef("nodefmap", mapType),
		esper.FieldDef("map", mapType),
	}, esper.WithNestedPropertySchema("map", levelOne), esper.AllowDynamicFields()); err != nil {
		return err
	}
	if err := regOA("MyNested", []esper.FieldSpec{
		esper.FieldDef("bean", reflect.TypeOf(oaMyNested{})),
	}, esper.AllowDynamicFields()); err != nil {
		return err
	}
	return nil
}

// oaQueryForEPL maps each pinned deploy EPL to its fluent equivalent. The
// scenario carries the verbatim Java EPL; drift is a contract violation.
func oaQueryForEPL(env *esper.Environment, caseName, epl string) (esper.Query, error) {
	field := func(path string) esper.Expression[any] {
		return esper.Field[any, any](path)
	}
	optional := func(path string) esper.Expression[any] {
		return esper.CoalesceOf[any](field(path), esper.NullLiteral[any]())
	}
	switch caseName {
	case "array-property":
		switch epl {
		case "@name('s0') select p0[0] as a, p0[1] as b, p1[0].intPrimitive as c, p1[1] as d, p0 as e from MyArrayOA":
			return esper.FromAny(env, "MyArrayOA").Select(
				esper.Alias("a", field("p0[0]")),
				esper.Alias("b", field("p0[1]")),
				esper.Alias("c", field("p1[0].intPrimitive")),
				esper.Alias("d", field("p1[1]")),
				esper.Alias("e", field("p0")),
			).Query(esper.StatementName("s0")), nil
		case "@name('s0') select outer.p0[0] as a, outer.p0[1] as b, outer.p1[0].intPrimitive as c, outer.p1[1] as d, outer.p0 as e from MyArrayOAMapOuter":
			return esper.FromAny(env, "MyArrayOAMapOuter").Select(
				esper.Alias("a", field("outer.p0[0]")),
				esper.Alias("b", field("outer.p0[1]")),
				esper.Alias("c", field("outer.p1[0].intPrimitive")),
				esper.Alias("d", field("outer.p1[1]")),
				esper.Alias("e", field("outer.p0")),
			).Query(esper.StatementName("s0")), nil
		}
	case "mapped-property":
		switch epl {
		case "@name('s0') select p0('k1') as a from MyMappedPropertyMap":
			return esper.FromAny(env, "MyMappedPropertyMap").Select(
				esper.Alias("a", field("p0('k1')")),
			).Query(esper.StatementName("s0")), nil
		case "@name('s0') select outer.p0('k1') as a from MyMappedPropertyMapOuter":
			return esper.FromAny(env, "MyMappedPropertyMapOuter").Select(
				esper.Alias("a", field("outer.p0('k1')")),
			).Query(esper.StatementName("s0")), nil
		case "@name('s0') select outerTwo.mapProperty('xOne') as a from MyMappedPropertyMapOuterTwo":
			return esper.FromAny(env, "MyMappedPropertyMapOuterTwo").Select(
				esper.Alias("a", field("outerTwo.mapProperty('xOne')")),
			).Query(esper.StatementName("s0")), nil
		}
	case "map-name-nested":
		switch epl {
		case "@name('s0') select outer.p0.n0 as a, outer.p1[0].n0 as b, outer.p1[1].n0 as c, outer.p0 as d, outer.p1 as e from MyObjectArrayMapOuter":
			return esper.FromAny(env, "MyObjectArrayMapOuter").Select(
				esper.Alias("a", field("outer.p0.n0")),
				esper.Alias("b", field("outer.p1[0].n0")),
				esper.Alias("c", field("outer.p1[1].n0")),
				esper.Alias("d", field("outer.p0")),
				esper.Alias("e", field("outer.p1")),
			).Query(esper.StatementName("s0")), nil
		case "@name('s0') select outer.p0.n0? as a, outer.p1[0].n0? as b, outer.p1[1]?.n0 as c, outer.p0? as d, outer.p1? as e from MyObjectArrayMapOuter":
			return esper.FromAny(env, "MyObjectArrayMapOuter").Select(
				esper.Alias("a", optional("outer.p0.n0?")),
				esper.Alias("b", optional("outer.p1[0].n0?")),
				esper.Alias("c", optional("outer.p1[1]?.n0")),
				esper.Alias("d", optional("outer.p0?")),
				esper.Alias("e", optional("outer.p1?")),
			).Query(esper.StatementName("s0")), nil
		}
	case "map-name":
		if epl == "@name('s0') select p0.n0 as a, p1[0].n0 as b, p1[1].n0 as c, p0 as d, p1 as e from MyOAWithAMap" {
			return esper.FromAny(env, "MyOAWithAMap").Select(
				esper.Alias("a", field("p0.n0")),
				esper.Alias("b", field("p1[0].n0")),
				esper.Alias("c", field("p1[1].n0")),
				esper.Alias("d", field("p0")),
				esper.Alias("e", field("p1")),
			).Query(esper.StatementName("s0")), nil
		}
	case "oa-nested":
		if epl == "@name('s0') select * from TypeRoot#lastevent" {
			return esper.FromAny(env, "TypeRoot").Window(esper.LastEvent()).Select(
				esper.Alias("rootId", field("rootId")),
				esper.Alias("p0", field("p0")),
			).Query(esper.StatementName("s0")), nil
		}
	case "pojo":
		switch epl {
		case "@name('s0') select simple, object, nodefmap, map, object.id as a1, nodefmap.key1? as a2, nodefmap.key2? as a3, nodefmap.key3?.key4 as a4, map.objectOne as b1, map.simpleOne as b2, map.nodefmapOne.key2? as b3, map.mapOne.simpleTwo? as b4, map.objectOne.indexed[1] as c1, map.objectOne.nested.nestedValue as c2,map.mapOne.simpleTwo as d1, map.mapOne.objectTwo as d2, map.mapOne.nodefmapTwo as d3, map.mapOne.mapTwo as e1, map.mapOne.mapTwo.simpleThree as e2, map.mapOne.mapTwo.objectThree as e3, map.mapOne.objectTwo.array[1].mapped('1ma').value as f1, map.mapOne.mapTwo.objectThree.id as f2 from NestedObjectArr":
			return esper.FromAny(env, "NestedObjectArr").Select(
				esper.Alias("simple", field("simple")),
				esper.Alias("object", field("object")),
				esper.Alias("nodefmap", field("nodefmap")),
				esper.Alias("map", field("map")),
				esper.Alias("a1", field("object.id")),
				esper.Alias("a2", optional("nodefmap.key1?")),
				esper.Alias("a3", optional("nodefmap.key2?")),
				esper.Alias("a4", optional("nodefmap.key3?.key4")),
				esper.Alias("b1", field("map.objectOne")),
				esper.Alias("b2", field("map.simpleOne")),
				esper.Alias("b3", optional("map.nodefmapOne.key2?")),
				esper.Alias("b4", optional("map.mapOne.simpleTwo?")),
				esper.Alias("c1", field("map.objectOne.indexed[1]")),
				esper.Alias("c2", field("map.objectOne.nested.nestedValue")),
				esper.Alias("d1", field("map.mapOne.simpleTwo")),
				esper.Alias("d2", field("map.mapOne.objectTwo")),
				esper.Alias("d3", field("map.mapOne.nodefmapTwo")),
				esper.Alias("e1", field("map.mapOne.mapTwo")),
				esper.Alias("e2", field("map.mapOne.mapTwo.simpleThree")),
				esper.Alias("e3", field("map.mapOne.mapTwo.objectThree")),
				esper.Alias("f1", field("map.mapOne.objectTwo.array[1].mapprop('1ma').value")),
				esper.Alias("f2", field("map.mapOne.mapTwo.objectThree.id")),
			).Query(esper.StatementName("s0")), nil
		case "@name('s0') select * from NestedObjectArr":
			return esper.FromAny(env, "NestedObjectArr").Select(
				esper.Alias("simple", field("simple")),
				esper.Alias("object", field("object")),
				esper.Alias("nodefmap", field("nodefmap")),
				esper.Alias("map", field("map")),
			).Query(esper.StatementName("s0")), nil
		case "@name('s0') select * from MyNested(bean.insides.anyOf(i=>id = 'A'))":
			return esper.FromAny(env, "MyNested").
				Filter(esper.EnumAnyOf[oaMyInside](
					field("bean.insides"),
					esper.Equal[string](esper.EnumField[oaMyInside, string]("id"), esper.Literal("A")))).
				Query(esper.StatementName("s0")), nil
		}
	}
	return esper.Query{}, fmt.Errorf("%s case %q: unpinned deploy epl %q", eventObjectArrayNestedID, caseName, epl)
}

// oaSend decodes a send step. Object-array payloads are positional JSON
// arrays; map payloads are JSON objects; bean payloads carry a "_bean" tag
// the decoder turns into the typed mirror.
func oaSend(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	var raw any
	if err := json.Unmarshal(step.Payload, &raw); err != nil {
		return err
	}
	switch step.EventType {
	case "MyMappedPropertyMap", "MyMappedPropertyMapOuter", "MyMappedPropertyMapOuterTwo":
		payload, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: %s payload must be an object", eventObjectArrayNestedID, step.EventType)
		}
		decoded, err := oaDecodeValue(payload)
		if err != nil {
			return err
		}
		return engine.SendRecord(ctx, step.EventType, decoded.(map[string]any))
	default:
		payload, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("%s: %s payload must be a positional array", eventObjectArrayNestedID, step.EventType)
		}
		decoded, err := oaDecodeValue(payload)
		if err != nil {
			return err
		}
		return engine.SendObjectArray(ctx, step.EventType, decoded.([]any))
	}
}

// oaDecodeValue converts JSON payload values into Go values: objects tagged
// "_bean" become typed mirrors, plain objects become map[string]any, arrays
// recurse.
func oaDecodeValue(value any) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		if bean, ok := v["_bean"].(string); ok {
			return oaDecodeBean(bean, v)
		}
		if long, ok := v["_long"]; ok {
			// {"_long":N} pins a Java Long value in the payload.
			if n, ok := long.(float64); ok {
				return int64(n), nil
			}
			return nil, fmt.Errorf("%s: invalid _long payload %v", eventObjectArrayNestedID, long)
		}
		out := make(map[string]any, len(v))
		for key, item := range v {
			decoded, err := oaDecodeValue(item)
			if err != nil {
				return nil, err
			}
			out[key] = decoded
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			decoded, err := oaDecodeValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = decoded
		}
		return out, nil
	default:
		return value, nil
	}
}

func oaDecodeBean(name string, payload map[string]any) (any, error) {
	str := func(key string) string {
		if s, ok := payload[key].(string); ok {
			return s
		}
		return ""
	}
	num := func(key string) int64 {
		if n, ok := payload[key].(float64); ok {
			return int64(n)
		}
		return 0
	}
	switch name {
	case "SupportBean":
		return oaSupportBean{TheString: str("theString"), IntPrimitive: int32(num("intPrimitive"))}, nil
	case "SupportBean_A":
		return oaBeanA{ID: str("id")}, nil
	case "SupportBean_B":
		return oaBeanB{ID: str("id")}, nil
	case "SupportBeanComplexProps":
		return oaDefaultComplexProps(), nil
	case "SupportBeanCombinedProps":
		return oaDefaultCombinedProps(), nil
	case "MyNested":
		var insides []oaMyInside
		if list, ok := payload["insides"].([]any); ok {
			for _, item := range list {
				if entry, ok := item.(map[string]any); ok {
					id, _ := entry["id"].(string)
					insides = append(insides, oaMyInside{ID: id})
				}
			}
		}
		return oaMyNested{Insides: insides}, nil
	}
	return nil, fmt.Errorf("%s: unknown bean tag %q", eventObjectArrayNestedID, name)
}

// oaResultRows renders result rows with the runner's deterministic renderer:
// NormalizeResults cannot render Go structs/slices as the oracle's nested
// {kind:row,fields} shape, so every column goes through oaRenderValue.
func oaResultRows(results []esper.Result, nested map[string]esper.Schema) ([]compat.ResultRecord, error) {
	if len(results) == 0 {
		return nil, nil
	}
	records := make([]compat.ResultRecord, 0, len(results))
	for _, result := range results {
		record := compat.ResultRecord{Kind: "row", Fields: make(map[string]any)}
		if event, ok := result.Event(); ok {
			for _, field := range event.Schema().Fields() {
				record.Fields[field.Name] = oaRenderValue(event.Get(field.Name).Any())
			}
			records = append(records, record)
			continue
		}
		if row, ok := result.Row(); ok {
			for _, field := range row.Schema().Fields() {
				if fragment, ok := row.GetFragment(field.Name); ok {
					record.Fields[field.Name] = oaRenderValue(fragment)
					continue
				}
				if fragments, ok := row.GetFragments(field.Name); ok {
					out := make([]any, len(fragments))
					for i, fragment := range fragments {
						out[i] = oaRenderValue(fragment)
					}
					record.Fields[field.Name] = out
					continue
				}
				if schema, ok := nested[field.Name]; ok {
					record.Fields[field.Name] = oaRenderNested(row.Get(field.Name).Any(), schema)
					continue
				}
				if schema, ok := row.Schema().NestedSchema(field.Name); ok {
					record.Fields[field.Name] = oaRenderNested(row.Get(field.Name).Any(), schema)
					continue
				}
				record.Fields[field.Name] = oaRenderValue(row.Get(field.Name).Any())
			}
			records = append(records, record)
		}
	}
	return records, nil
}

// oaRenderValue mirrors the oracle's normalize(): events and maps become
// {kind:row,fields}, slices become JSON arrays, structs render via esper
// tags, nil becomes {state:null}.
func oaRenderValue(value any) any {
	switch v := value.(type) {
	case nil:
		return map[string]any{"state": "null"}
	case esper.Event:
		fields := make(map[string]any)
		for _, f := range v.Schema().Fields() {
			fields[f.Name] = oaRenderValue(v.Get(f.Name).Any())
		}
		return map[string]any{"kind": "row", "fields": fields}
	case map[string]any:
		fields := make(map[string]any, len(v))
		for key, item := range v {
			fields[key] = oaRenderValue(item)
		}
		return map[string]any{"kind": "row", "fields": fields}
	case map[string]string:
		fields := make(map[string]any, len(v))
		for key, item := range v {
			fields[key] = item
		}
		return map[string]any{"kind": "row", "fields": fields}
	case map[string]oaNestedLevTwo:
		fields := make(map[string]any, len(v))
		for key, item := range v {
			fields[key] = oaRenderValue(item)
		}
		return map[string]any{"kind": "row", "fields": fields}
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = oaRenderValue(item)
		}
		return out
	case []oaSupportBean:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = oaRenderStruct(item)
		}
		return out
	case []*oaNestedLevOne:
		out := make([]any, len(v))
		for i, item := range v {
			if item == nil {
				out[i] = map[string]any{"state": "null"}
			} else {
				out[i] = oaRenderStruct(*item)
			}
		}
		return out
	case []oaMyInside:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = oaRenderStruct(item)
		}
		return out
	case []int64:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = item
		}
		return out
	case []int:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = int64(item)
		}
		return out
	case []map[string]any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = oaRenderValue(item)
		}
		return out
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case oaSupportBean, oaBeanA, oaBeanB, oaComplexProps, oaCombinedProps,
		oaNestedLevOne, oaNestedLevTwo, oaComplexNested, oaComplexNestedNested,
		oaMyNested, oaMyInside:
		return oaRenderStruct(v)
	default:
		rv := reflect.ValueOf(value)
		if rv.IsValid() && rv.Kind() == reflect.Struct {
			return oaRenderStruct(value)
		}
		return value
	}
}

// oaRenderStruct renders a Go struct as {kind:row,fields} using esper tags.
func oaRenderStruct(value any) map[string]any {
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return map[string]any{"kind": "row", "fields": map[string]any{"state": "null"}}
		}
		rv = rv.Elem()
	}
	rt := rv.Type()
	fields := make(map[string]any, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		name := rt.Field(i).Tag.Get("esper")
		if name == "" {
			name = rt.Field(i).Name
		}
		fields[name] = oaRenderValue(rv.Field(i).Interface())
	}
	return map[string]any{"kind": "row", "fields": fields}
}

// oaRenderNested renders a value against a nested schema: positional arrays
// become rows, arrays of arrays become row arrays, maps become rows.
func oaRenderNested(value any, schema esper.Schema) any {
	if value == nil {
		return map[string]any{"state": "null"}
	}
	fields := schema.Fields()
	switch v := value.(type) {
	case []any:
		// An array of objects is an array of nested rows, never a positional
		// row — even when its length happens to equal the field count.
		allMaps := len(v) > 0
		for _, item := range v {
			if _, ok := item.(map[string]any); !ok {
				allMaps = false
				break
			}
		}
		if !allMaps && len(v) == len(fields) {
			row := make(map[string]any, len(fields))
			for i, f := range fields {
				row[f.Name] = oaRenderNestedValue(v[i], schema, f.Name)
			}
			return map[string]any{"kind": "row", "fields": row}
		}
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = oaRenderNested(item, schema)
		}
		return out
	case map[string]any:
		row := make(map[string]any, len(v))
		for key, item := range v {
			row[key] = oaRenderNestedValue(item, schema, key)
		}
		return map[string]any{"kind": "row", "fields": row}
	default:
		return oaRenderValue(value)
	}
}

// oaRenderNestedValue renders one nested field, recursing into the field's
// own nested schema when present.
func oaRenderNestedValue(value any, schema esper.Schema, name string) any {
	if nested, ok := schema.NestedSchema(name); ok {
		return oaRenderNested(value, nested)
	}
	return oaRenderValue(value)
}
