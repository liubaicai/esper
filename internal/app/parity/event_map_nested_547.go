package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Draft-4.547 runner for the four unreferenced EventMapNested executions
// (all SERDEREQUIRED; the flag has no Go boundary):
//
//   - insert-into (ord 0, EventMapNestedInsertInto): the @public
//     "insert into MyStream select map.mapOne as val1 from NestedMap#length(5)"
//     feeds "select val1 as a from MyStream"; one full-payload send asserts
//     a == the level-two map fragment. Java's @public visibility annotation
//     has no Go counterpart: the MyStream type is pre-registered instead.
//   - event-type (ord 1, EventMapNestedEventType): metadata introspection of
//     the deployed "select * from NestedMap" statement — property names
//     {simple,object,nodefmap,map} in any order, types String/Map/Map/
//     SupportBean_A, and a null property type for the nested dotted path
//     map.mapOne.simpleOne — acknowledged by a deployed marker. Esper's
//     EventType.getPropertyType only resolves declared root names, so the
//     Go assertion mirrors it by checking the declared property set.
//   - nested-pojo (ord 2, EventMapNestedNestedPojo): the verbatim 22-column
//     projection over NestedMap#length(5) — '?' optional paths wrapped in
//     CoalesceOf so Java null and Go Missing both render {state:null},
//     indexed[1]/nested bean chains resolved by the fluent path resolver,
//     and Java's mapped('1ma') accessor expressed as the mapprop('1ma')
//     path on the bean's map property (the event-objectarray-nested
//     precedent) — against the full payload then the partial payload
//     (L1 drops nodefmapOne with null simpleOne/objectOne, L2 drops
//     simpleTwo, L3 drops objectThree); the 4000 long survives via the
//     {"_long":N} payload marker.
//   - is-exists (ord 3, EventMapNestedIsExists): seven exists() columns over
//     '?'-paths; explicit null still exists (b is always false because
//     mapOne?.simpleOne reads the mapOne sub-map, which never carries a
//     simpleOne key — only present-null vs missing distinguishes the
//     partial payload's c and g columns).
//
// Bean underlyings reuse the oa* mirrors from event_objectarray_nested.go
// and render through the shared oaResultRows/oaRenderValue helpers: maps
// become {kind:row,fields}, beans become their pinned field rows, null
// becomes {state:null}. The NestedMap type is pre-registered as a
// RegisterMap chain mirroring TestSuiteEventMap.configure (it is config,
// not a scenario deploy).
const eventMapNested547JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
const eventMapNested547ID = "event-map-nested-547"
const eventMapNested547Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/map/EventMapNested.java"
const eventMapNested547Description = "EventMapNested nested-map event slice (all 4 executions, SERDEREQUIRED): insert-into routes map.mapOne as val1 through the public MyStream into the s0 consumer over NestedMap#length(5); event-type introspects the deployed select-* NestedMap statement (declared property names, String/Map/Map/SupportBean_A types, null dotted-property type); nested-pojo projects the verbatim 22-column graph incl. '?' optional paths, indexed[1], nested bean chains and the mapped('1ma') accessor over the full then partial payload; is-exists checks seven exists() columns over '?'-paths for both payloads."

var eventMapNested547JavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/map/EventMapNested.java",
}

var eventMapNested547JavaRuntimeIDs = []string{
	"java-runtime-7eef27d1a8de16f61ff7",
	"java-runtime-17f60eacc6ec6d23d967",
	"java-runtime-cf3abd23bf728441a001",
	"java-runtime-68a5f852752eba589f5b",
}

var eventMapNested547JavaExecutions = []string{
	"EventMapNestedInsertInto",
	"EventMapNestedEventType",
	"EventMapNestedNestedPojo",
	"EventMapNestedIsExists",
}

var eventMapNested547JavaStaticIDs = []string{
	"java-26557b255ea0dcbd9d2a",
	"java-26557b255ea0dcbd9d2a",
	"java-26557b255ea0dcbd9d2a",
	"java-26557b255ea0dcbd9d2a",
}

var eventMapNested547JavaFlags = []string{"SERDEREQUIRED"}

var eventMapNested547Cases = []string{
	"insert-into",
	"event-type",
	"nested-pojo",
	"is-exists",
}

var eventMapNested547CaseOrdinals = []int{0, 1, 2, 3}

var eventMapNested547CaseObservations = []string{
	"listener",
	"deployed",
	"listener",
	"listener",
}

// Byte-exact EPLs pinned from the Java regression source.
const emn547InsertEPL = "@public insert into MyStream select map.mapOne as val1 from NestedMap#length(5)"
const emn547ConsumerEPL = "@name('s0') select val1 as a from MyStream"
const emn547EventTypeEPL = "@name('s0') select * from NestedMap"
const emn547NestedPojoEPL = "@name('s0') select simple, object, nodefmap, map, object.id as a1, nodefmap.key1? as a2, nodefmap.key2? as a3, nodefmap.key3?.key4 as a4, map.objectOne as b1, map.simpleOne as b2, map.nodefmapOne.key2? as b3, map.mapOne.simpleTwo? as b4, map.objectOne.indexed[1] as c1, map.objectOne.nested.nestedValue as c2,map.mapOne.simpleTwo as d1, map.mapOne.objectTwo as d2, map.mapOne.nodefmapTwo as d3, map.mapOne.mapTwo as e1, map.mapOne.mapTwo.simpleThree as e2, map.mapOne.mapTwo.objectThree as e3, map.mapOne.objectTwo.array[1].mapped('1ma').value as f1, map.mapOne.mapTwo.objectThree.id as f2 from NestedMap#length(5)"
const emn547IsExistsEPL = "@name('s0') select exists(map.mapOne?) as a,exists(map.mapOne?.simpleOne) as b,exists(map.mapOne?.simpleTwo) as c,exists(map.mapOne?.mapTwo) as d,exists(map.mapOne.mapTwo?) as e,exists(map.mapOne.mapTwo.simpleThree?) as f,exists(map.mapOne.mapTwo.objectThree?) as g  from NestedMap#length(5)"

// eventMapNested547CaseEPLs pins the case-level EPL text; insert-into joins
// the two deployed statements with a newline (same convention as the 546
// multi-statement case).
var eventMapNested547CaseEPLs = map[string]string{
	"insert-into": emn547InsertEPL + "\n" + emn547ConsumerEPL,
	"event-type":  emn547EventTypeEPL,
	"nested-pojo": emn547NestedPojoEPL,
	"is-exists":   emn547IsExistsEPL,
}

// emn547CaseState accumulates one case's records; sequences follow the
// parity convention (per-statement, per-operation counters starting at one).
type emn547CaseState struct {
	caseName string
	trace    *compat.Trace
	seq      map[string]uint64
	now      string
}

func newEMN547CaseState(caseName string, trace *compat.Trace) *emn547CaseState {
	return &emn547CaseState{
		caseName: caseName,
		trace:    trace,
		seq:      map[string]uint64{},
		now:      compat.FormatTraceTime(time.Unix(0, 0).UTC()),
	}
}

func (s *emn547CaseState) emit(operation, statement string, newRows, oldRows []compat.ResultRecord, name string, value any) {
	key := statement + ":" + operation
	s.seq[key]++
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: operation,
		Statement: statement,
		Sequence:  s.seq[key],
		Time:      s.now,
		New:       newRows,
		Old:       oldRows,
		Name:      name,
		Value:     value,
	})
}

func (s *emn547CaseState) emitDeployed(statement string) {
	s.emit("deployed", statement, nil, nil, "", nil)
}

func (s *emn547CaseState) emitListener(statement string, newRows, oldRows []compat.ResultRecord) {
	s.emit("listener", statement, newRows, oldRows, "", nil)
}

func (s *emn547CaseState) resetSequences() {
	// undeployAll mirrors Java's per-module listener counters restarting.
	s.seq = map[string]uint64{}
}

// emn547Env carries one case's engine and deployed statements.
type emn547Env struct {
	env         *esper.Environment
	engine      *esper.Engine
	deployments map[string]*esper.Deployment
}

// emn547Register declares the preconfigured types mirroring
// TestSuiteEventMap.configure: the three-level NestedMap chain
// (map -> {simpleOne,objectOne,nodefmapOne,mapOne} -> {simpleTwo,objectTwo,
// nodefmapTwo,mapTwo} -> {simpleThree,objectThree}) and the MyStream
// insert-into target. Java bean types ride as the oa* struct mirrors;
// AllowDynamicFields lets the '?' optional paths pass build-time
// validation, exactly like the event-objectarray-nested registrations.
func emn547Register(env *esper.Environment) error {
	intT := reflect.TypeOf(int64(0))
	stringT := reflect.TypeOf("")
	mapT := reflect.TypeOf(map[string]any(nil))

	levelThree, err := esper.NewMapSchema("NestedMapLevelThree", []esper.FieldSpec{
		esper.FieldDef("simpleThree", intT),
		esper.FieldDef("objectThree", reflect.TypeOf(oaBeanB{})),
	}, esper.AllowDynamicFields())
	if err != nil {
		return fmt.Errorf("%s: level-three schema: %w", eventMapNested547ID, err)
	}
	levelTwo, err := esper.NewMapSchema("NestedMapLevelTwo", []esper.FieldSpec{
		esper.FieldDef("simpleTwo", intT),
		esper.FieldDef("objectTwo", reflect.TypeOf(oaCombinedProps{})),
		esper.FieldDef("nodefmapTwo", mapT),
		esper.FieldDef("mapTwo", mapT),
	}, esper.WithNestedPropertySchema("mapTwo", levelThree), esper.AllowDynamicFields())
	if err != nil {
		return fmt.Errorf("%s: level-two schema: %w", eventMapNested547ID, err)
	}
	levelOne, err := esper.NewMapSchema("NestedMapLevelOne", []esper.FieldSpec{
		esper.FieldDef("simpleOne", intT),
		esper.FieldDef("objectOne", reflect.TypeOf(oaComplexProps{})),
		esper.FieldDef("nodefmapOne", mapT),
		esper.FieldDef("mapOne", mapT),
	}, esper.WithNestedPropertySchema("mapOne", levelTwo), esper.AllowDynamicFields())
	if err != nil {
		return fmt.Errorf("%s: level-one schema: %w", eventMapNested547ID, err)
	}
	if _, err := esper.RegisterMap(env, "NestedMap", []esper.FieldSpec{
		esper.FieldDef("simple", stringT),
		esper.FieldDef("object", reflect.TypeOf(oaBeanA{})),
		esper.FieldDef("nodefmap", mapT),
		esper.FieldDef("map", mapT),
	}, esper.WithNestedPropertySchema("map", levelOne), esper.AllowDynamicFields()); err != nil {
		return fmt.Errorf("%s: register NestedMap: %w", eventMapNested547ID, err)
	}
	if _, err := esper.RegisterMap(env, "MyStream", []esper.FieldSpec{
		esper.FieldDef("val1", mapT),
	}); err != nil {
		return fmt.Errorf("%s: register MyStream: %w", eventMapNested547ID, err)
	}
	return nil
}

func newEMN547Env() (*emn547Env, error) {
	env := esper.NewEnvironment()
	if err := emn547Register(env); err != nil {
		return nil, err
	}
	return &emn547Env{
		env:         env,
		engine:      esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC())),
		deployments: map[string]*esper.Deployment{},
	}, nil
}

// emn547QueryForEPL maps each pinned deploy EPL to its fluent equivalent.
// The scenario carries the verbatim Java EPL; drift is a contract violation.
// Java's mapped('k') bean accessor maps to the mapprop('k') path; the '?'
// optional accesses are wrapped in CoalesceOf so Java null and Go Missing
// both render {state:null}.
func emn547QueryForEPL(env *esper.Environment, epl string) (esper.Query, error) {
	eventValue := esper.EventValue[esper.Event]()
	field := func(path string) esper.Expression[any] {
		return esper.Field[any, any](path)
	}
	prop := func(path string) esper.Expression[any] {
		return esper.Property[any](eventValue, path)
	}
	optional := func(path string) esper.Expression[any] {
		return esper.CoalesceOf[any](field(path), esper.NullLiteral[any]())
	}
	switch epl {
	case emn547InsertEPL:
		return esper.FromAny(env, "NestedMap").Window(esper.LengthWindow(5)).Select(
			esper.Alias("val1", field("map.mapOne")),
		).InsertInto("MyStream", esper.StatementName("insert-into")), nil
	case emn547ConsumerEPL:
		return esper.FromAny(env, "MyStream").Select(
			esper.Alias("a", field("val1")),
		).Query(esper.StatementName("s0")), nil
	case emn547EventTypeEPL:
		return esper.FromAny(env, "NestedMap").Query(esper.StatementName("s0")), nil
	case emn547NestedPojoEPL:
		return esper.FromAny(env, "NestedMap").Window(esper.LengthWindow(5)).Select(
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
	case emn547IsExistsEPL:
		return esper.FromAny(env, "NestedMap").Window(esper.LengthWindow(5)).Select(
			esper.Alias("a", esper.Exists(prop("map.mapOne?"))),
			esper.Alias("b", esper.Exists(prop("map.mapOne?.simpleOne"))),
			esper.Alias("c", esper.Exists(prop("map.mapOne?.simpleTwo"))),
			esper.Alias("d", esper.Exists(prop("map.mapOne?.mapTwo"))),
			esper.Alias("e", esper.Exists(prop("map.mapOne.mapTwo?"))),
			esper.Alias("f", esper.Exists(prop("map.mapOne.mapTwo.simpleThree?"))),
			esper.Alias("g", esper.Exists(prop("map.mapOne.mapTwo.objectThree?"))),
		).Query(esper.StatementName("s0")), nil
	}
	return esper.Query{}, fmt.Errorf("%s: unpinned deploy epl %q", eventMapNested547ID, epl)
}

// emn547AssertEventType mirrors EventMapNestedEventType's assertStatement
// against the deployed select-* statement's result schema: the four declared
// property names in any order, the pinned root property types and a null
// nested dotted lookup. Esper's EventType.getPropertyType resolves only
// declared root property names, so the Go check compares against the
// declared-property set.
func emn547AssertEventType(schema esper.Schema) error {
	if schema.Kind() != esper.SchemaMap {
		return fmt.Errorf("%s: expected MAP schema kind, got %v", eventMapNested547ID, schema.Kind())
	}
	names := schema.PropertyNames()
	if len(names) != 4 {
		return fmt.Errorf("%s: expected four property names, got %v", eventMapNested547ID, names)
	}
	for _, want := range []string{"simple", "object", "nodefmap", "map"} {
		if !slices.Contains(names, want) {
			return fmt.Errorf("%s: missing declared property %q in %v", eventMapNested547ID, want, names)
		}
	}
	checkType := func(name string, want reflect.Type) error {
		got, ok := schema.PropertyType(name)
		if !ok || got != want {
			return fmt.Errorf("%s: property %q type = %v (ok=%v), want %v", eventMapNested547ID, name, got, ok, want)
		}
		return nil
	}
	if err := checkType("simple", reflect.TypeOf("")); err != nil {
		return err
	}
	if err := checkType("map", reflect.TypeOf(map[string]any(nil))); err != nil {
		return err
	}
	if err := checkType("nodefmap", reflect.TypeOf(map[string]any(nil))); err != nil {
		return err
	}
	if err := checkType("object", reflect.TypeOf(oaBeanA{})); err != nil {
		return err
	}
	if slices.Contains(names, "map.mapOne.simpleOne") {
		return fmt.Errorf("%s: dotted property map.mapOne.simpleOne unexpectedly declared", eventMapNested547ID)
	}
	return nil
}

// deploy builds and deploys the statement for one deploy step; only named
// s0 statements of listener cases receive the recording listener (Java's
// addListener("s0")), and event-type runs the pinned metadata assertion
// against the pre-registered NestedMap schema (the source schema select-*
// projects verbatim).
func (e *emn547Env) deploy(ctx context.Context, state *emn547CaseState, caseName string, step compat.Step) error {
	if step.Epl == "" {
		return fmt.Errorf("%s case %q: deploy step without pinned epl", eventMapNested547ID, caseName)
	}
	query, err := emn547QueryForEPL(e.env, step.Epl)
	if err != nil {
		return err
	}
	plan, err := e.env.Build(query)
	if err != nil {
		return fmt.Errorf("%s case %q: build %q: %w", eventMapNested547ID, caseName, step.Epl, err)
	}
	deployment, err := e.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s case %q: deploy %q: %w", eventMapNested547ID, caseName, step.Epl, err)
	}
	e.deployments[step.Statement] = deployment
	for _, statement := range deployment.Statements() {
		if caseName != "event-type" && statement.Name() == "s0" {
			captured := statement
			if _, err := captured.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				newRows, err := oaResultRows(batch.New, nil)
				if err != nil {
					return err
				}
				oldRows, err := oaResultRows(batch.Old, nil)
				if err != nil {
					return err
				}
				state.emitListener(captured.Name(), newRows, oldRows)
				return nil
			}); err != nil {
				return err
			}
		}
	}
	if caseName == "event-type" {
		// select * projects the source schema verbatim; the registered
		// NestedMap schema carries the same observable the Java
		// assertStatement reads off statement.getEventType().
		resultSchema, ok := e.env.Schema("NestedMap")
		if !ok {
			return fmt.Errorf("%s case %q: preconfigured type NestedMap not found", eventMapNested547ID, caseName)
		}
		if err := emn547AssertEventType(resultSchema); err != nil {
			return err
		}
	}
	return nil
}

// send decodes a send step and delivers it through SendRecord; {"_bean":...}
// payload objects become the typed mirrors and {"_long":N} pins a Java Long.
func (e *emn547Env) send(ctx context.Context, caseName string, step compat.Step) error {
	if step.EventType != "NestedMap" {
		return fmt.Errorf("%s case %q: unsupported send event type %q", eventMapNested547ID, caseName, step.EventType)
	}
	var raw any
	if err := json.Unmarshal(step.Payload, &raw); err != nil {
		return fmt.Errorf("%s case %q: decode send payload: %w", eventMapNested547ID, caseName, err)
	}
	decoded, err := oaDecodeValue(raw)
	if err != nil {
		return err
	}
	payload, ok := decoded.(map[string]any)
	if !ok {
		return fmt.Errorf("%s case %q: NestedMap payload must decode to a map", eventMapNested547ID, caseName)
	}
	return e.engine.SendRecord(ctx, step.EventType, payload)
}

func runEventMapNested547Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	type caseBlock struct {
		name  string
		steps []compat.Step
	}
	var blocks []caseBlock
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			blocks = append(blocks, caseBlock{name: step.Case})
			continue
		}
		if len(blocks) == 0 {
			return trace, fmt.Errorf("%s: step %q precedes the first case marker", eventMapNested547ID, step.Op)
		}
		blocks[len(blocks)-1].steps = append(blocks[len(blocks)-1].steps, step)
	}
	for _, block := range blocks {
		if err := runEMN547Case(ctx, block.name, block.steps, &trace); err != nil {
			return trace, fmt.Errorf("%s case %q: %w", eventMapNested547ID, block.name, err)
		}
	}
	return trace, nil
}

func runEMN547Case(ctx context.Context, caseName string, steps []compat.Step, trace *compat.Trace) error {
	if !slices.Contains(eventMapNested547Cases, caseName) {
		return fmt.Errorf("unsupported case %q", caseName)
	}
	state := newEMN547CaseState(caseName, trace)
	ee, err := newEMN547Env()
	if err != nil {
		return err
	}
	engineOpen := true
	closeEngine := func() error {
		if !engineOpen {
			return nil
		}
		engineOpen = false
		return ee.engine.Close(ctx)
	}
	defer func() { _ = closeEngine() }()
	for _, step := range steps {
		switch step.Op {
		case "deploy":
			if err := ee.deploy(ctx, state, caseName, step); err != nil {
				return err
			}
		case "deployed":
			state.emitDeployed(step.Statement)
		case "send":
			if err := ee.send(ctx, caseName, step); err != nil {
				return err
			}
		case "undeploy-all":
			state.resetSequences()
			for name, deployment := range ee.deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return fmt.Errorf("undeploy %q: %w", name, err)
				}
			}
			ee.deployments = map[string]*esper.Deployment{}
			if err := closeEngine(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported op %q in %q", step.Op, caseName)
		}
	}
	return nil
}

// ---------- pinned step keys and scenario loader ----------

func emn547CaseKey(caseName string) string {
	return strings.Join([]string{"case", caseName, "", "", "", "", "", "", ""}, "|")
}
func emn547DeployKey(caseName, statement, epl string) string {
	return strings.Join([]string{"deploy", caseName, statement, "", "", "", epl, "", ""}, "|")
}
func emn547DeployedKey(caseName, statement string) string {
	return strings.Join([]string{"deployed", caseName, statement, "", "", "", "", "", ""}, "|")
}
func emn547SendKey(caseName, eventType, payload string) string {
	return strings.Join([]string{"send", caseName, "", eventType, "", "", "", "", payload}, "|")
}
func emn547UndeployAllKey(caseName string) string {
	return strings.Join([]string{"undeploy-all", caseName, "", "", "", "", "", "", ""}, "|")
}

// emn547CanonicalPayload renders a payload object as the canonical JSON the
// pinned-step comparison uses (sorted keys, compact form).
func emn547CanonicalPayload(payload any) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Sprintf("%s: marshal pinned payload: %v", eventMapNested547ID, err))
	}
	return string(raw)
}

// emn547Payload builds the NestedMap payload variants mirroring
// EventMapNested.getTestData()/getTestDataThree().
func emn547Payload(partial bool) string {
	levelThree := map[string]any{"simpleThree": map[string]any{"_long": 4000}}
	if !partial {
		levelThree["objectThree"] = map[string]any{"_bean": "SupportBean_B", "id": "B1"}
	}
	levelTwo := map[string]any{
		"objectTwo":   map[string]any{"_bean": "SupportBeanCombinedProps"},
		"nodefmapTwo": map[string]any{"key3": "val3"},
		"mapTwo":      levelThree,
	}
	if !partial {
		levelTwo["simpleTwo"] = 300
	}
	levelOne := map[string]any{"mapOne": levelTwo}
	if partial {
		levelOne["simpleOne"] = nil
		levelOne["objectOne"] = nil
	} else {
		levelOne["simpleOne"] = 10
		levelOne["objectOne"] = map[string]any{"_bean": "SupportBeanComplexProps"}
		levelOne["nodefmapOne"] = map[string]any{"key2": "val2"}
	}
	return emn547CanonicalPayload(map[string]any{
		"simple":   "abc",
		"object":   map[string]any{"_bean": "SupportBean_A", "id": "A1"},
		"nodefmap": map[string]any{"key1": "val1"},
		"map":      levelOne,
	})
}

// eventMapNested547PinnedSteps rebuilds the pinned step-key sequence; the
// loader compares every decoded step against it.
func eventMapNested547PinnedSteps() []string {
	var keys []string
	// insert-into (ord 0): @public insert + @name('s0') consumer, one send.
	keys = append(keys,
		emn547CaseKey("insert-into"),
		emn547DeployKey("insert-into", "insert-into", emn547InsertEPL),
		emn547DeployKey("insert-into", "s0", emn547ConsumerEPL),
		emn547SendKey("insert-into", "NestedMap", emn547Payload(false)),
		emn547UndeployAllKey("insert-into"),
	)
	// event-type (ord 1): select-* deploy, deployed marker, no sends.
	keys = append(keys,
		emn547CaseKey("event-type"),
		emn547DeployKey("event-type", "s0", emn547EventTypeEPL),
		emn547DeployedKey("event-type", "s0"),
		emn547UndeployAllKey("event-type"),
	)
	// nested-pojo (ord 2): 22-column projection, full then partial payload.
	keys = append(keys,
		emn547CaseKey("nested-pojo"),
		emn547DeployKey("nested-pojo", "s0", emn547NestedPojoEPL),
		emn547SendKey("nested-pojo", "NestedMap", emn547Payload(false)),
		emn547SendKey("nested-pojo", "NestedMap", emn547Payload(true)),
		emn547UndeployAllKey("nested-pojo"),
	)
	// is-exists (ord 3): seven exists() columns, full then partial payload.
	keys = append(keys,
		emn547CaseKey("is-exists"),
		emn547DeployKey("is-exists", "s0", emn547IsExistsEPL),
		emn547SendKey("is-exists", "NestedMap", emn547Payload(false)),
		emn547SendKey("is-exists", "NestedMap", emn547Payload(true)),
		emn547UndeployAllKey("is-exists"),
	)
	return keys
}

func emn547StepKey(step compat.Step) string {
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

func emn547ValidateStep(index int, object map[string]json.RawMessage, step compat.Step) error {
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
		return require("op", "case", "statement", "epl")
	case "deployed":
		return require("op", "case", "statement")
	case "send":
		return require("op", "case", "eventType", "payload")
	case "undeploy-all":
		return require("op", "case")
	}
	return fmt.Errorf("scenario step %d has unsupported op %q", index, step.Op)
}

func loadEventMapNested547Scenario(reader io.Reader) (compat.Scenario, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario must be a JSON object: %w", eventMapNested547ID, err)
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eventMapNested547ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eventMapNested547ID ||
		metadata.Description != eventMapNested547Description ||
		metadata.JavaCommit != eventMapNested547JavaCommit || metadata.JavaSource != eventMapNested547Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", eventMapNested547ID)
	}
	if !reflect.DeepEqual(metadata.JavaFlags, eventMapNested547JavaFlags) {
		return compat.Scenario{}, fmt.Errorf("%s javaFlags = %#v", eventMapNested547ID, metadata.JavaFlags)
	}
	if !reflect.DeepEqual(metadata.JavaRuntimes, eventMapNested547JavaRuntimeIDs) {
		return compat.Scenario{}, fmt.Errorf("%s javaRuntimes = %#v", eventMapNested547ID, metadata.JavaRuntimes)
	}
	if !reflect.DeepEqual(metadata.JavaNames, eventMapNested547JavaExecutions) {
		return compat.Scenario{}, fmt.Errorf("%s javaNames = %#v", eventMapNested547ID, metadata.JavaNames)
	}
	if !reflect.DeepEqual(metadata.JavaStaticID, eventMapNested547JavaStaticIDs) {
		return compat.Scenario{}, fmt.Errorf("%s javaStaticIds = %#v", eventMapNested547ID, metadata.JavaStaticID)
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", eventMapNested547ID, err)
	}
	if len(rawCases) != len(eventMapNested547Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", eventMapNested547ID, len(rawCases), len(eventMapNested547Cases))
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
		if definition.Case != eventMapNested547Cases[index] || definition.Ordinal != eventMapNested547CaseOrdinals[index] ||
			definition.RuntimeID != eventMapNested547JavaRuntimeIDs[index] ||
			definition.ExecutionName != eventMapNested547JavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d identity mismatch", eventMapNested547ID, index)
		}
		if definition.Observation != eventMapNested547CaseObservations[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %q observation is not pinned", eventMapNested547ID, definition.Case)
		}
		if definition.EPL != eventMapNested547CaseEPLs[definition.Case] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %q EPL is not pinned", eventMapNested547ID, definition.Case)
		}
		if !reflect.DeepEqual(definition.Flags, []string{"SERDEREQUIRED"}) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %q flags = %#v, want [SERDEREQUIRED]", eventMapNested547ID, definition.Case, definition.Flags)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", eventMapNested547ID, err)
	}
	pinned := eventMapNested547PinnedSteps()
	if len(rawSteps) != len(pinned) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d steps, want %d", eventMapNested547ID, len(rawSteps), len(pinned))
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
		if err := emn547ValidateStep(index, object, step); err != nil {
			return compat.Scenario{}, err
		}
		if key := emn547StepKey(step); key != pinned[index] {
			return compat.Scenario{}, fmt.Errorf("scenario step %d = %q, want pinned %q", index, key, pinned[index])
		}
		steps[index] = step
	}
	return compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}, nil
}
