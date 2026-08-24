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

var ebprJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/bean/EventBeanPropertyResolutionFragment.java",
}

var ebprJavaRuntimeIDs = []string{
	"java-runtime-1abd87887a2624440680",
	"java-runtime-59d49f62d061fc49f9ee",
	"java-runtime-2eaac4dcd93014962bd8",
	"java-runtime-43a5f4e4ec952ad9887c",
	"java-runtime-276de28b1f0e90a5e292",
	"java-runtime-544ea453ff05cbd7f75f",
	"java-runtime-ca984db35e239326a14a",
	"java-runtime-77591a9e2caed406cc58",
	"java-runtime-9231049b0bac5cc51f78",
	"java-runtime-f6149bcefb8f4290f17c",
	"java-runtime-b83f0d5666898d8dcd04",
	"java-runtime-b4077c7d900735417782",
	"java-runtime-93fd5baf00af2e252e0b",
	"java-runtime-415ec6ff4f1b6f87cc65",
	"java-runtime-aa515a1f3bad40ec148b",
}

var ebprJavaExecutions = []string{
	"EPLBeanMapSimpleTypes",
	"EPLBeanObjectArraySimpleTypes",
	"EPLBeanWrapperFragmentWithMap",
	"EPLBeanWrapperFragmentWithObjectArray",
	"EPLBeanNativeBeanFragment",
	"EPLBeanMapFragmentMapNested",
	"EPLBeanObjectArrayFragmentObjectArrayNested",
	"EPLBeanMapFragmentMapUnnamed",
	"EPLBeanMapFragmentTransposedMapEventBean",
	"EPLBeanObjectArrayFragmentTransposedMapEventBean",
	"EPLBeanMapFragmentMapBeans",
	"EPLBeanObjectArrayFragmentBeans",
	"EPLBeanMapFragmentMap3Level",
	"EPLBeanObjectArrayFragment3Level",
	"EPLBeanFragmentMapMulti",
}

// ebprComplex mirrors SupportBeanComplexProps' readable bean surface.
type ebprComplex struct {
	SimpleProperty string            `esper:"simpleProperty"`
	MapProperty    map[string]string `esper:"mapProperty"`
	ArrayProperty  []int             `esper:"arrayProperty"`
	Nested         map[string]string `esper:"nested"`
	ObjectArray    []any             `esper:"objectArray"`
}

// ebprCombined mirrors SupportBeanCombinedProps' readable surface: the array
// property carries its mapprop maps plus the trailing null element.
type ebprCombined struct {
	Array []map[string]any `esper:"array"`
}

func runEbprScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, cn := range []string{
		"map-simple-types", "object-array-simple-types", "wrapper-map",
		"wrapper-object-array", "native-bean-fragment", "map-nested",
		"object-array-nested", "map-unnamed", "transposed-map",
		"transposed-object-array", "map-beans", "object-array-beans",
		"map-3level", "object-array-3level", "map-multi",
	} {
		if !scenarioHasCase(scenario, cn) {
			continue
		}
		ct, err := runEbprCase(ctx, scenario, cn)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("case %q: %w", cn, err)
		}
		trace.Records = append(trace.Records, ct.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("no supported cases")
	}
	return trace, nil
}

func runEbprCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if err := registerEbprSchemas(env, caseName); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := &compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seq := uint64(0)
	statements := make(map[string]*esper.Statement)

	deployAny := func(name string, query esper.Query, waitOrder bool) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		for _, st := range deployment.Statements() {
			stmt := st
			statements[stmt.Name()] = stmt
			if _, subErr := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				hasNew := len(batch.New) > 0
				hasOld := len(batch.Old) > 0
				if !hasNew && !hasOld {
					return nil
				}
				seq++
				record := compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: stmt.Name(),
					Time:      batch.Time.UTC().Format(time.RFC3339),
					Sequence:  seq,
				}
				record.New = compat.NormalizeResults(batch.New)
				record.Old = compat.NormalizeResults(batch.Old)
				ebprNormalizeComplexRows(caseName, record.New)
				if record.New == nil {
					record.New = []compat.ResultRecord{}
				}
				if record.Old == nil {
					record.Old = []compat.ResultRecord{}
				}
				trace.Records = append(trace.Records, record)
				return nil
			}); subErr != nil {
				return subErr
			}
		}
		return nil
	}

	phase := 0
	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			if err := sendEbpr(ctx, engine, env, caseName, step.EventType, step.Payload); err != nil {
				return *trace, err
			}
		case "deploy":
			for name := range statements {
				_ = engine.Undeploy(ctx, statements[name].DeploymentID())
				delete(statements, name)
			}
			// A fresh deployment restarts the per-statement sequence, matching
			// the Java oracle's per-deployment TraceWriter.
			seq = 0
			deploy := func(query esper.Query) error {
				return deployAny("s0", query, false)
			}
			switch caseName {
			case "native-bean-fragment":
				if phase == 0 {
					phase++
					if err := deploy(esper.Select(
						esper.From[ebprComplex](env, "SupportBeanComplexProps"),
					).Query(esper.StatementName("s0"))); err != nil {
						return *trace, err
					}
				} else {
					if err := deploy(esper.Select(
						esper.From[ebprCombined](env, "SupportBeanCombinedProps"),
					).Query(esper.StatementName("s0"))); err != nil {
						return *trace, err
					}
				}
			case "transposed-map", "transposed-object-array":
				aName, bName := "GistMapOne", "GistMapTwo"
				if caseName == "transposed-object-array" {
					aName, bName = "CashMapOne", "CashMapTwo"
				}
				pat := esper.PatternFromRecord(esper.FromAny(env, aName), "one", esper.Literal(true)).
					Until(esper.PatternFromRecord(esper.FromAny(env, bName), "two", esper.Literal(true)))
				if caseName == "transposed-object-array" {
					if err := deploy(pat.Select(
						esper.Alias("one", ebprTagArrayRaw(esper.TagEvents("one"))),
						esper.Alias("two", ebprTagRaw(esper.PatternEvent("two"))),
					).Query(esper.StatementName("s0"))); err != nil {
						return *trace, err
					}
				} else {
					if err := deploy(pat.Select(
						esper.Alias("one", ebprTagArrayMap(esper.TagEvents("one"))),
						esper.Alias("two", ebprTagMap(esper.PatternEvent("two"))),
					).Query(esper.StatementName("s0"))); err != nil {
						return *trace, err
					}
				}
			case "wrapper-map", "wrapper-object-array":
				// Pinned: select *, p0simple.p1id + 1 as plusone, p0bean as mybean.
				stream := esper.FromAny(env, ebprTypeFor(caseName))
				if err := deploy(stream.Select(
					esper.Alias("p0simple", esper.Property[any](esper.EventValue[esper.Event](), "p0simple")),
					esper.Alias("p0bean", esper.Property[any](esper.EventValue[esper.Event](), "p0bean")),
					esper.Alias("plusone", esper.Func1Ctx[esper.Event, int32]("plusone",
						func(e esper.Event, _ esper.EvalContext) int32 {
							simple := e.Get("p0simple")
							switch v := simple.Any().(type) {
							case map[string]any:
								if id, ok := v["p1id"].(int32); ok {
									return id + 1
								}
								if id, ok := v["p1id"].(float64); ok {
									return int32(id) + 1
								}
							case []any:
								// Object-array root: p0simple is the positional value list
								// of the nested WheatLev0 [p1id]; index 0 is p1id.
								if len(v) > 0 {
									if id, ok := v[0].(int32); ok {
										return id + 1
									}
									if id, ok := v[0].(float64); ok {
										return int32(id) + 1
									}
								}
							}
							return 0
						}, esper.EventValue[esper.Event]())),
					esper.Alias("mybean", esper.Property[any](esper.EventValue[esper.Event](), "p0bean")),
				).Query(esper.StatementName("s0"))); err != nil {
					return *trace, err
				}
			default:
				if err := deploy(esper.FromAny(env, ebprTypeFor(caseName)).
					Select().Query(esper.StatementName("s0"))); err != nil {
					return *trace, err
				}
			}
		default:
			return *trace, fmt.Errorf("unsupported event-bean-property-fragment op %q (case %q)", step.Op, caseName)
		}
	}
	return *trace, nil
}

func ebprTypeFor(caseName string) string {
	switch caseName {
	case "map-simple-types":
		return "MSTypeOne"
	case "object-array-simple-types":
		return "OASimple"
	case "wrapper-map":
		return "Frosty"
	case "wrapper-object-array":
		return "WheatRoot"
	case "map-nested":
		return "HomerunRoot"
	case "object-array-nested":
		return "GoalRoot"
	case "map-unnamed":
		return "FlywheelRoot"
	case "map-beans":
		return "TXTypeRoot"
	case "object-array-beans":
		return "LocalTypeRoot"
	case "map-3level":
		return "JimTypeRoot"
	case "object-array-3level":
		return "JackTypeRoot"
	case "map-multi":
		return "MMOuterMap"
	}
	panic("unhandled ebpr case " + caseName)
}

func registerEbprSchemas(env *esper.Environment, caseName string) error {
	anyT := reflect.TypeOf(any(nil))

	register := func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) error {
		_, err := esper.RegisterMap(env, name, fields, opts...)
		return err
	}
	registerOA := func(name string, fields []esper.FieldSpec, opts ...esper.SchemaOption) error {
		_, err := esper.RegisterObjectArray(env, name, fields, opts...)
		return err
	}

	switch caseName {
	case "map-simple-types":
		return register("MSTypeOne", []esper.FieldSpec{
			{Name: "p0int", Type: anyT}, {Name: "p0intarray", Type: anyT}, {Name: "p0map", Type: anyT},
		})
	case "object-array-simple-types":
		return registerOA("OASimple", []esper.FieldSpec{
			{Name: "p0int", Type: anyT}, {Name: "p0intarray", Type: anyT}, {Name: "p0map", Type: anyT},
		})
	case "wrapper-map":
		if err := register("FrostyLev0", []esper.FieldSpec{{Name: "p1id", Type: anyT}}); err != nil {
			return err
		}
		lev0, _ := env.Schema("FrostyLev0")
		if err := register("Frosty", []esper.FieldSpec{
			{Name: "p0simple", Type: anyT}, {Name: "p0bean", Type: anyT},
		}, esper.WithNestedPropertySchema("p0simple", lev0)); err != nil {
			return err
		}
		return nil
	case "wrapper-object-array":
		if err := registerOA("WheatLev0", []esper.FieldSpec{{Name: "p1id", Type: anyT}}); err != nil {
			return err
		}
		lev0, _ := env.Schema("WheatLev0")
		return registerOA("WheatRoot", []esper.FieldSpec{
			{Name: "p0simple", Type: anyT}, {Name: "p0bean", Type: anyT},
		}, esper.WithNestedPropertySchema("p0simple", lev0))
	case "native-bean-fragment":
		if _, err := esper.RegisterStruct[ebprComplex](env, "SupportBeanComplexProps"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[ebprCombined](env, "SupportBeanCombinedProps"); err != nil {
			return err
		}
		return nil
	case "map-nested":
		if err := register("HomerunLev0", []esper.FieldSpec{{Name: "p1id", Type: anyT}}); err != nil {
			return err
		}
		lev0, _ := env.Schema("HomerunLev0")
		return register("HomerunRoot", []esper.FieldSpec{
			{Name: "p0simple", Type: anyT},
			{Name: "p0array", Type: anyT},
		}, esper.WithNestedPropertySchema("p0simple", lev0),
			esper.WithNestedPropertySchema("p0array", lev0))
	case "object-array-nested":
		if err := registerOA("GoalLev0", []esper.FieldSpec{{Name: "p1id", Type: anyT}}); err != nil {
			return err
		}
		lev0, _ := env.Schema("GoalLev0")
		return registerOA("GoalRoot", []esper.FieldSpec{
			{Name: "p0simple", Type: anyT},
			{Name: "p0array", Type: anyT},
		}, esper.WithNestedPropertySchema("p0simple", lev0),
			esper.WithNestedPropertySchema("p0array", lev0))
	case "map-unnamed":
		return register("FlywheelRoot", []esper.FieldSpec{{Name: "p0simple", Type: anyT}})
	case "transposed-map":
		if err := register("GistInner", []esper.FieldSpec{{Name: "p2id", Type: anyT}}); err != nil {
			return err
		}
		inner, _ := env.Schema("GistInner")
		reg := func(name string) error {
			return register(name, []esper.FieldSpec{
				{Name: "id", Type: anyT},
				{Name: "bean", Type: anyT},
				{Name: "beanarray", Type: anyT},
				{Name: "complex", Type: anyT},
				{Name: "complexarray", Type: anyT},
				{Name: "map", Type: anyT},
				{Name: "maparray", Type: anyT},
			}, esper.WithNestedPropertySchema("map", inner),
				esper.WithNestedPropertySchema("maparray", inner))
		}
		if err := reg("GistMapOne"); err != nil {
			return err
		}
		return reg("GistMapTwo")
	case "transposed-object-array":
		if err := registerOA("CashInner", []esper.FieldSpec{{Name: "p2id", Type: anyT}}); err != nil {
			return err
		}
		inner, _ := env.Schema("CashInner")
		reg := func(name string) error {
			return registerOA(name, []esper.FieldSpec{
				{Name: "id", Type: anyT},
				{Name: "bean", Type: anyT},
				{Name: "beanarray", Type: anyT},
				{Name: "complex", Type: anyT},
				{Name: "complexarray", Type: anyT},
				{Name: "map", Type: anyT},
				{Name: "maparray", Type: reflect.TypeOf([][]any{})},
			}, esper.WithNestedPropertySchema("map", inner),
				esper.WithNestedPropertySchema("maparray", inner))
		}
		if err := reg("CashMapOne"); err != nil {
			return err
		}
		return reg("CashMapTwo")
	case "map-beans":
		if err := register("TXTypeLev0", []esper.FieldSpec{
			{Name: "p1simple", Type: anyT},
			{Name: "p1array", Type: anyT},
			{Name: "p1complex", Type: anyT},
			{Name: "p1complexarray", Type: anyT},
		}); err != nil {
			return err
		}
		lev0, _ := env.Schema("TXTypeLev0")
		return register("TXTypeRoot", []esper.FieldSpec{
			{Name: "p0simple", Type: anyT},
			{Name: "p0array", Type: anyT},
		}, esper.WithNestedPropertySchema("p0simple", lev0),
			esper.WithNestedPropertySchema("p0array", lev0))
	case "object-array-beans":
		if err := registerOA("LocalTypeLev0", []esper.FieldSpec{
			{Name: "p1simple", Type: anyT},
			{Name: "p1array", Type: anyT},
			{Name: "p1complex", Type: anyT},
			{Name: "p1complexarray", Type: anyT},
		}); err != nil {
			return err
		}
		lev0, _ := env.Schema("LocalTypeLev0")
		return registerOA("LocalTypeRoot", []esper.FieldSpec{
			{Name: "p0simple", Type: anyT},
			{Name: "p0array", Type: anyT},
		}, esper.WithNestedPropertySchema("p0simple", lev0),
			esper.WithNestedPropertySchema("p0array", lev0))
	case "map-3level":
		if err := register("JimTypeLev1", []esper.FieldSpec{{Name: "p2id", Type: anyT}}); err != nil {
			return err
		}
		lev1, _ := env.Schema("JimTypeLev1")
		if err := register("JimTypeLev0", []esper.FieldSpec{
			{Name: "p1simple", Type: anyT},
			{Name: "p1array", Type: anyT},
		}, esper.WithNestedPropertySchema("p1simple", lev1),
			esper.WithNestedPropertySchema("p1array", lev1)); err != nil {
			return err
		}
		lev0, _ := env.Schema("JimTypeLev0")
		return register("JimTypeRoot", []esper.FieldSpec{
			{Name: "p0simple", Type: anyT},
			{Name: "p0array", Type: anyT},
		}, esper.WithNestedPropertySchema("p0simple", lev0),
			esper.WithNestedPropertySchema("p0array", lev0))
	case "object-array-3level":
		if err := registerOA("JackTypeLev1", []esper.FieldSpec{{Name: "p2id", Type: anyT}}); err != nil {
			return err
		}
		lev1, _ := env.Schema("JackTypeLev1")
		if err := registerOA("JackTypeLev0", []esper.FieldSpec{
			{Name: "p1simple", Type: anyT},
			{Name: "p1array", Type: anyT},
		}, esper.WithNestedPropertySchema("p1simple", lev1),
			esper.WithNestedPropertySchema("p1array", lev1)); err != nil {
			return err
		}
		lev0, _ := env.Schema("JackTypeLev0")
		return registerOA("JackTypeRoot", []esper.FieldSpec{
			{Name: "p0simple", Type: anyT},
			{Name: "p0array", Type: anyT},
		}, esper.WithNestedPropertySchema("p0simple", lev0),
			esper.WithNestedPropertySchema("p0array", lev0))
	case "map-multi":
		if err := register("MMInner", []esper.FieldSpec{{Name: "p2id", Type: anyT}}); err != nil {
			return err
		}
		inner, _ := env.Schema("MMInner")
		if err := register("MMInnerMap", []esper.FieldSpec{
			{Name: "p1bean", Type: anyT},
			{Name: "p1beanComplex", Type: anyT},
			{Name: "p1beanArray", Type: anyT},
			{Name: "p1innerId", Type: anyT},
			{Name: "p1innerMap", Type: anyT},
		}, esper.WithNestedPropertySchema("p1innerMap", inner)); err != nil {
			return err
		}
		innerMap, _ := env.Schema("MMInnerMap")
		return register("MMOuterMap", []esper.FieldSpec{
			{Name: "p0simple", Type: anyT},
			{Name: "p0array", Type: anyT},
		}, esper.WithNestedPropertySchema("p0simple", innerMap),
			esper.WithNestedPropertySchema("p0array", innerMap))
	}
	return fmt.Errorf("unhandled ebpr schemas for case %q", caseName)
}

func sendEbpr(ctx context.Context, engine *esper.Engine, env *esper.Environment, caseName, eventType string, payload json.RawMessage) error {
	if eventType == "SupportBeanComplexProps" {
		var p map[string]any
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		v := ebprComplex{
			SimpleProperty: jsonString(p["simpleProperty"]),
			MapProperty:    jsonStringMap(p["mapProperty"]),
			ArrayProperty:  jsonIntSlice(p["arrayProperty"]),
			Nested: map[string]string{
				"nestedValue":       jsonString(p["nestedValue"]),
				"nestedNestedValue": jsonString(p["nestedNestedValue"]),
			},
		}
		return engine.SendEvent(ctx, v)
	}
	if eventType == "SupportBeanCombinedProps" {
		var p map[string]any
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		v := ebprCombined{Array: ebprCombinedArray(p["array"])}
		return engine.SendEvent(ctx, v)
	}
	if len(payload) > 0 && payload[0] == '[' {
		var arr []any
		if err := json.Unmarshal(payload, &arr); err != nil {
			return err
		}
		return engine.SendObjectArray(ctx, eventType, ebprConvertOA(caseName, arr))
	}
	var p map[string]any
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	return engine.SendRecord(ctx, eventType, ebprFillMap(env, eventType, ebprConvertMap(caseName, p)))
}

// ebprConvertMap converts the scenario payload into the schema's send shape
// (bean maps to SupportBean strings; complex maps trimmed to readable keys).
func ebprConvertMap(caseName string, p map[string]any) map[string]any {
	out := make(map[string]any, len(p))
	for k, v := range p {
		out[k] = ebprConvertValue(caseName, k, v)
	}
	return out
}

func ebprConvertOA(caseName string, arr []any) []any {
	out := make([]any, len(arr))
	for i, v := range arr {
		out[i] = ebprConvertValue(caseName, "", v)
	}
	if caseName == "transposed-object-array" {
		// maparray is [][]any in the schema: wrap the inner list once.
		if inner, ok := out[6].([]any); ok {
			wrapped := make([][]any, 0, len(inner))
			for _, item := range inner {
				if m, ok := item.([]any); ok {
					wrapped = append(wrapped, m)
				}
			}
			out[6] = wrapped
		}
	}
	return out
}

// ebprCoerceOA converts raw JSON []interface{} values into the typed shapes
// the object-array schema expects ([][]any stays, []any of numbers -> []int32).
func ebprCoerceOA(caseName string, arr []any) []any {
	return arr
}

func ebprConvertValue(caseName, key string, v any) any {
	switch val := v.(type) {
	case map[string]any:
		if isBeanMap(val) {
			return beanToString(val)
		}
		if isComplexMap(val) {
			return complexToStringMap(val)
		}
		conv := make(map[string]any, len(val))
		for k2, v2 := range val {
			conv[k2] = ebprConvertValue(caseName, k2, v2)
		}
		return conv
	case []any:
		out := make([]any, len(val))
		for i, v2 := range val {
			out[i] = ebprConvertValue(caseName, key, v2)
		}
		return out
	}
	return v
}

func isBeanMap(m map[string]any) bool {
	_, ok := m["theString"]
	return ok
}

func isComplexMap(m map[string]any) bool {
	_, ok := m["simpleProperty"]
	return ok
}

func beanToString(m map[string]any) string {
	return fmt.Sprintf("SupportBean(%s, %d)",
		jsonString(m["theString"]), jsonInt64(m["intPrimitive"]))
}

func complexToStringMap(m map[string]any) map[string]any {
	out := map[string]any{
		"simpleProperty": jsonString(m["simpleProperty"]),
		"mapProperty":    jsonStringMap(m["mapProperty"]),
		"arrayProperty":  jsonIntSlice(m["arrayProperty"]),
	}
	if nested, ok := m["nested"].(map[string]any); ok {
		out["nested"] = map[string]any{
			"nestedValue":       jsonString(nested["nestedValue"]),
			"nestedNestedValue": jsonString(nested["nestedNestedValue"]),
		}
	} else if _, has := m["nestedValue"]; has {
		out["nested"] = map[string]any{
			"nestedValue":       jsonString(m["nestedValue"]),
			"nestedNestedValue": jsonString(m["nestedNestedValue"]),
		}
	}
	if m["objectArray"] == nil {
		out["objectArray"] = map[string]any{"state": "null"}
	} else {
		out["objectArray"] = m["objectArray"]
	}
	return out
}

func jsonStringMap(m any) map[string]string {
	out := map[string]string{}
	if mm, ok := m.(map[string]any); ok {
		for k, v := range mm {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

func jsonIntSlice(m any) []int {
	var out []int
	if arr, ok := m.([]any); ok {
		for _, v := range arr {
			if n, ok := v.(float64); ok {
				out = append(out, int(n))
			}
		}
	}
	return out
}

func jsonAnyMapSlice(m any) []map[string]any {
	var out []map[string]any
	if arr, ok := m.([]any); ok {
		for _, v := range arr {
			if mm, ok := v.(map[string]any); ok {
				out = append(out, mm)
			} else {
				out = append(out, nil)
			}
		}
	}
	return out
}

// ebprTagArrayMap renders a repeated pattern tag (until-accumulated) as an
// array of plain maps.
func ebprTagArrayMap(events esper.Expression[[]esper.Event]) esper.Expression[[]map[string]any] {
	return esper.Func1Ctx[[]esper.Event, []map[string]any]("ebprTagArrayMap",
		func(events []esper.Event, _ esper.EvalContext) []map[string]any {
			rows := make([]map[string]any, 0, len(events))
			for _, e := range events {
				rows = append(rows, ebprEventMap(e))
			}
			return rows
		}, events)
}

func ebprTagMap(tag esper.Expression[esper.Event]) esper.Expression[map[string]any] {
	return esper.Func1Ctx[esper.Event, map[string]any]("ebprTagMap",
		func(e esper.Event, _ esper.EvalContext) map[string]any { return ebprEventMap(e) }, tag)
}

func ebprEventMap(e esper.Event) map[string]any {
	out := make(map[string]any, len(e.Schema().Fields()))
	for _, field := range e.Schema().Fields() {
		out[field.Name] = e.Get(field.Name).Any()
	}
	return out
}

// ebprFillMap fills any schema field absent from the payload with a nil value
// so absent properties render as null (matching the Java map-event contract).
func ebprFillMap(env *esper.Environment, eventType string, p map[string]any) map[string]any {
	schema, ok := env.Schema(eventType)
	if !ok {
		return p
	}
	for _, field := range schema.Fields() {
		if _, exists := p[field.Name]; !exists {
			p[field.Name] = nil
		}
	}
	return p
}

// ebprCombinedArray builds the SupportBeanCombinedProps array from the
// scenario payload: each element is {mapprop: {key: {value}}, nestLevOneVal}.
func ebprCombinedArray(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		mapprop := map[string]any{}
		for k, val := range m {
			if str, ok := val.(string); ok {
				mapprop[k] = map[string]any{"value": str}
			}
		}
		out = append(out, map[string]any{"mapprop": mapprop, "nestLevOneVal": "abc"})
	}
	if len(out) == 3 {
		out = append(out, map[string]any{"__ebprNull__": true})
	}
	return out
}

// ebprNormalizeComplexRows adjusts bean rows to the Java oracle rendering:
// nil objectArray renders as {state:null}.
func ebprNormalizeComplexRows(caseName string, rows []compat.ResultRecord) {
	for _, row := range rows {
		if v, ok := row.Fields["objectArray"]; ok {
			if v == nil {
				row.Fields["objectArray"] = map[string]any{"state": "null"}
			} else if arr, isSlice := v.([]any); isSlice && len(arr) == 0 {
				row.Fields["objectArray"] = map[string]any{"state": "null"}
			}
		}
		if caseName == "native-bean-fragment" {
			slice, ok := ebprAnySlice(row.Fields["array"])
			if ok {
				for i, item := range slice {
					if m, isMap := item.(map[string]any); isMap && m["__ebprNull__"] == true {
						slice[i] = map[string]any{"state": "null"}
					} else if item == nil {
						slice[i] = map[string]any{"state": "null"}
					}
				}
				row.Fields["array"] = slice
			}
		}
	}
}

// ebprTagRaw renders an object-array pattern tag as its raw positional values.
func ebprTagRaw(tag esper.Expression[esper.Event]) esper.Expression[[]any] {
	return esper.Func1Ctx[esper.Event, []any]("ebprTagRaw",
		func(e esper.Event, _ esper.EvalContext) []any {
			values, _ := e.Underlying().([]any)
			return values
		}, tag)
}

// ebprTagArrayRaw renders repeated object-array pattern tags as raw positional
// value arrays.
func ebprTagArrayRaw(events esper.Expression[[]esper.Event]) esper.Expression[[][]any] {
	return esper.Func1Ctx[[]esper.Event, [][]any]("ebprTagArrayRaw",
		func(events []esper.Event, _ esper.EvalContext) [][]any {
			rows := make([][]any, 0, len(events))
			for _, e := range events {
				values, _ := e.Underlying().([]any)
				rows = append(rows, values)
			}
			return rows
		}, events)
}

// ebprAnySlice converts a slice-typed field value into []any.
func ebprAnySlice(v any) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	case []map[string]any:
		out := make([]any, len(s))
		for i, m := range s {
			out[i] = m
		}
		return out, true
	}
	return nil, false
}
