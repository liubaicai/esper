package esper

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
)

// Parity coverage for EPLInsertIntoTransposeStream (10 executions).
//
// Java source:
// regression-lib/src/main/java/com/espertech/esper/regressionlib/
// suite/epl/insertinto/EPLInsertIntoTransposeStream.java
//
// The Go typed fluent API expresses transpose() as Transpose[T](expr), whose
// evaluated payload becomes the routed event's underlying object instead of
// named columns. Runtime enforcement is split exactly as the runbook freezes
// it: type-coercion errors are rejected at Build (plan.go validateRoute +
// validateTransposeExpression), while the projection path materializes the
// verified payload into the registered route target.
//
// Executions 0-8 (observable) are covered here as parity tests. Executions
// 9-10 are INVALIDITY executions (EPLInsertIntoTransposeSingleColumnInsertInvalid,
// EPLInsertIntoInvalidTranspose) whose Java protocol has no trace step; their
// Go equivalents are covered by Go-unit build-error tests in
// epl_insert_into_transpose_stream_invalid_test.go and registered as
// implemented (not differential-verified).

// epSupportBeanTwo mirrors SupportBeanTwo (exec 0 transpose target).
type epSupportBeanTwo struct {
	StringTwo       string `esper:"stringTwo"`
	IntPrimitiveTwo int    `esper:"intPrimitiveTwo"`
}

// epSupportBeanA/B mirror SupportBean_A/SupportBean_B (exec 7 join POJO).
type epSupportBeanA struct {
	ID string `esper:"id"`
}

type epSupportBeanB struct {
	ID string `esper:"id"`
}

// goComplexProps/goComplexNested mirror SupportBeanComplexProps.Nested
// (exec 8 POJO property stream) so the nested inneritem.nestedValue path is
// asserted through a dotted property name.
type goComplexProps struct {
	Nested    *goComplexNested  `esper:"nested"`
	NestedMap map[string]string `esper:"nestedMap"`
}

type goComplexNested struct {
	NestedValue string `esper:"nestedValue"`
}

// transposeRoutesCollector accumulates routed Event envelopes delivered to one
// subscribed statement, preserving deterministic order.
type transposeRoutesCollector struct {
	mu     sync.Mutex
	events []Event
}

func (c *transposeRoutesCollector) subscribe(runtime *Engine, deployment *Deployment, stmtName string) {
	for _, statement := range deployment.Statements() {
		if statement.Name() != stmtName {
			continue
		}
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			c.mu.Lock()
			defer c.mu.Unlock()
			for index := range batch.New {
				if event, ok := batch.New[index].Event(); ok {
					c.events = append(c.events, event)
				}
			}
			return nil
		}); err != nil {
			panic(err)
		}
	}
}

func (c *transposeRoutesCollector) snapshot() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Event(nil), c.events...)
}

// transposeScenarioEnv builds an environment with SupportBean and the given
// target event types registered, mirroring the Java compileDeploy path.
func transposeScenarioEnv(runtimeID string, registers ...SchemaRegistrar) (*Environment, error) {
	env := NewEnvironment()
	if _, err := RegisterStruct[epSupportBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	for _, register := range registers {
		if err := register(env); err != nil {
			return nil, err
		}
	}
	return env, nil
}

// SchemaRegistrar registers one named event type into the environment.
type SchemaRegistrar func(env *Environment) error

func transposeRegisterStruct[T any](name string) SchemaRegistrar {
	return func(env *Environment) error {
		_, err := RegisterStruct[T](env, name)
		return err
	}
}

func transposeRegisterMap(name string, fields []FieldSpec) SchemaRegistrar {
	return func(env *Environment) error {
		_, err := RegisterMap(env, name, fields)
		return err
	}
}

func transposeRegisterObjectArray(name string, fields []FieldSpec) SchemaRegistrar {
	return func(env *Environment) error {
		_, err := RegisterObjectArray(env, name, fields)
		return err
	}
}

func transposeRegisterAvro(name string, fields []FieldSpec) SchemaRegistrar {
	return func(env *Environment) error {
		_, err := RegisterAvro(env, name, fields)
		return err
	}
}

func transposeRegisterJSON(name string, fields []FieldSpec) SchemaRegistrar {
	return func(env *Environment) error {
		_, err := RegisterJSON(env, name, fields)
		return err
	}
}

func transposeRegisterJSONFor[T any](name string, fields []FieldSpec) SchemaRegistrar {
	return func(env *Environment) error {
		_, err := RegisterJSONFor[T](env, name, fields)
		return err
	}
}

func deployPlans(t *testing.T, engine *Engine, plans ...Plan) []*Deployment {
	t.Helper()
	deployments := make([]*Deployment, 0, len(plans))
	for _, plan := range plans {
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		deployments = append(deployments, deployment)
	}
	return deployments
}

// TestEPLInsertIntoTransposeCreateSchemaPOJOParity covers
// EPLInsertIntoTransposeCreateSchemaPOJO: an on-trigger stream select routes
// transpose(makeSB2Event(event)) into two event streams (astream/bstream)
// registered as SupportBeanTwo; both consumer statements observe the routed
// target event with the source properties carried through.
func TestEPLInsertIntoTransposeCreateSchemaPOJOParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[epSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[epSupportBeanTwo](env, "SupportBeanTwo"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[epSupportBeanTwo](env, "astream"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[epSupportBeanTwo](env, "bstream"); err != nil {
		t.Fatal(err)
	}

	produceA, err := env.Build(Select(
		From[epSupportBean](env, "SupportBean"),
		Selection{Name: "col", Expr: Transpose[*epSupportBeanTwo](
			Func1[epSupportBean, *epSupportBeanTwo]("makeSB2Event", func(sb epSupportBean) *epSupportBeanTwo {
				return &epSupportBeanTwo{StringTwo: sb.TheString, IntPrimitiveTwo: sb.IntPrimitive}
			}, EventValue[epSupportBean]()),
		)},
	).InsertInto("astream", StatementName("produce-a")))
	if err != nil {
		t.Fatal(err)
	}
	produceB, err := env.Build(Select(
		From[epSupportBean](env, "SupportBean"),
		Selection{Name: "col", Expr: Transpose[*epSupportBeanTwo](
			Func1[epSupportBean, *epSupportBeanTwo]("makeSB2Event", func(sb epSupportBean) *epSupportBeanTwo {
				return &epSupportBeanTwo{StringTwo: sb.TheString, IntPrimitiveTwo: sb.IntPrimitive}
			}, EventValue[epSupportBean]()),
		)},
	).InsertInto("bstream", StatementName("produce-b")))
	if err != nil {
		t.Fatal(err)
	}
	consumerA, err := env.Build(FromAny(env, "astream").Query(StatementName("a")))
	if err != nil {
		t.Fatal(err)
	}
	consumerB, err := env.Build(FromAny(env, "bstream").Query(StatementName("b")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env, WithRuntimeURI("java-runtime-02d711374dfb7f329757"))
	collectA := &transposeRoutesCollector{}
	collectB := &transposeRoutesCollector{}
	deployments := deployPlans(t, engine, produceA, produceB, consumerA, consumerB)
	collectA.subscribe(engine, deployments[2], "a")
	collectB.subscribe(engine, deployments[3], "b")
	defer func() { _ = engine.Close(context.Background()) }()

	if err := engine.SendEvent(context.Background(), epSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	eventsA := collectA.snapshot()
	eventsB := collectB.snapshot()
	if len(eventsA) != 1 || len(eventsB) != 1 {
		t.Fatalf("astream=%d bstream=%d routed events", len(eventsA), len(eventsB))
	}
	for _, events := range [][]Event{eventsA, eventsB} {
		for _, event := range events {
			if event.Get("stringTwo").Any() != "E1" || event.Get("intPrimitiveTwo").Any() != 1 {
				t.Fatalf("transpose routed event = %v/%v", event.Get("stringTwo").Any(), event.Get("intPrimitiveTwo").Any())
			}
		}
	}
}

// transposeRepresentationPair runs one exec 1 representation subtest.
// Java selects transpose(generateX(theString, intPrimitive)) from SupportBean
// into MySchema and asserts the target event carries p0/p1. The Go equivalent
// uses a typed Transpose[T](Func2[...,T](...)) whose payload matches the
// registered target representation.
func transposeRepresentationPair(t *testing.T, name string, register SchemaRegistrar, transpose Expr) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[epSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if err := register(env); err != nil {
		t.Fatal(err)
	}
	target := targetFor(name)

	buildProducer := func(stmtName string) Plan {
		plan, err := env.Build(Select(
			From[epSupportBean](env, "SupportBean"),
			Selection{Name: "col", Expr: transpose},
		).InsertInto(target, StatementName(stmtName)))
		if err != nil {
			t.Fatalf("%s: build %s: %v", name, stmtName, err)
		}
		return plan
	}

	engine := NewEngine(env, WithRuntimeURI("java-runtime-c667bb8d3dabac4238f8"))
	defer func() { _ = engine.Close(context.Background()) }()

	producerS0 := buildProducer("s0")
	consumer, err := env.Build(FromAny(env, target).Query(StatementName("s0-consumer")))
	if err != nil {
		t.Fatalf("%s: build consumer: %v", name, err)
	}
	deployS0 := deployPlans(t, engine, producerS0, consumer)
	collect := &transposeRoutesCollector{}
	// Capture routed events on the consumer, which sees every delivered
	// target event regardless of which producer produced it.
	collect.subscribe(engine, deployS0[1], "s0-consumer")

	if err := engine.SendEvent(context.Background(), epSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), epSupportBean{TheString: "E2", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Undeploy(context.Background(), deployS0[0].ID()); err != nil {
		t.Fatal(err)
	}
	deployS1 := deployPlans(t, engine, buildProducer("s1"))
	_ = deployS1
	if err := engine.SendEvent(context.Background(), epSupportBean{TheString: "E3", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	events := collect.snapshot()
	if len(events) != 3 {
		t.Fatalf("%s: routed events = %d, want 3", name, len(events))
	}
	if got := events[0].Get("p0").Any(); got != "E1" {
		t.Fatalf("%s: first p0 = %v", name, got)
	}
	if got := events[0].Get("p1").Any(); got != 1 {
		t.Fatalf("%s: first p1 = %v", name, got)
	}
	if got := events[1].Get("p0").Any(); got != "E2" {
		t.Fatalf("%s: second p0 = %v", name, got)
	}
	if got := events[1].Get("p1").Any(); got != 2 {
		t.Fatalf("%s: second p1 = %v", name, got)
	}
	if got := events[2].Get("p0").Any(); got != "E3" {
		t.Fatalf("%s: third p0 = %v", name, got)
	}
	if got := events[2].Get("p1").Any(); got != 3 {
		t.Fatalf("%s: third p1 = %v", name, got)
	}
}

func targetFor(rep string) string {
	switch rep {
	case "map":
		return "MySchemaMap"
	case "oa":
		return "MySchemaOA"
	case "avro":
		return "MySchemaAvro"
	case "json":
		return "MySchemaJSON"
	case "jsonprovided":
		return "MySchemaJsonProvided"
	default:
		return "MySchema"
	}
}

func transposeSchemaFields() []FieldSpec {
	return []FieldSpec{
		FieldDef("p0", reflect.TypeOf("")),
		FieldDef("p1", reflect.TypeOf(0)),
	}
}

// TestEPLInsertIntoTransposeMapAndObjectArrayAndOthersParity covers
// EPLInsertIntoTransposeMapAndObjectArrayAndOthers across the five event
// representations supported by transpose (Map, ObjectArray, Avro, JSON,
// JSON class-provided). Each subtest routes two events and asserts the mapped
// p0/p1 values on the produce subject listener.
func TestEPLInsertIntoTransposeMapAndObjectArrayAndOthersParity(t *testing.T) {
	// Each representation subtest builds one environment, then sends two
	// SupportBean events. The producer statement's own listener observes the
	// routed target event.
	cases := []struct {
		name       string
		register   SchemaRegistrar
		transposed Expr // transpose child expression carrying a typed payload
	}{
		{
			name:     "map",
			register: transposeRegisterMap("MySchemaMap", transposeSchemaFields()),
			transposed: Transpose[map[string]any](
				Func2[string, int, map[string]any]("custom", func(s string, i int) map[string]any {
					return map[string]any{"p0": s, "p1": i}
				}, Field[epSupportBean, string]("theString"), Field[epSupportBean, int]("intPrimitive")),
			),
		},
		{
			name:     "oa",
			register: transposeRegisterObjectArray("MySchemaOA", transposeSchemaFields()),
			transposed: Transpose[[]any](
				Func2[string, int, []any]("custom", func(s string, i int) []any {
					values := make([]any, 2)
					values[0] = s
					values[1] = i
					return values
				}, Field[epSupportBean, string]("theString"), Field[epSupportBean, int]("intPrimitive")),
			),
		},
		{
			name:     "avro",
			register: transposeRegisterAvro("MySchemaAvro", transposeSchemaFields()),
			transposed: Transpose[map[string]any](
				Func2[string, int, map[string]any]("custom", func(s string, i int) map[string]any {
					return map[string]any{"p0": s, "p1": i}
				}, Field[epSupportBean, string]("theString"), Field[epSupportBean, int]("intPrimitive")),
			),
		},
		{
			name:     "json",
			register: transposeRegisterJSON("MySchemaJSON", transposeSchemaFields()),
			transposed: Transpose[string](
				Func2[string, int, string]("custom", func(s string, i int) string {
					encoded, _ := json.Marshal(map[string]any{"p0": s, "p1": i})
					return string(encoded)
				}, Field[epSupportBean, string]("theString"), Field[epSupportBean, int]("intPrimitive")),
			),
		},
		{
			name:     "jsonprovided",
			register: transposeRegisterJSONFor[transposeJsonProvided]("MySchemaJsonProvided", transposeSchemaFields()),
			transposed: Transpose[string](
				Func2[string, int, string]("custom", func(s string, i int) string {
					encoded, _ := json.Marshal(map[string]any{"p0": s, "p1": i})
					return string(encoded)
				}, Field[epSupportBean, string]("theString"), Field[epSupportBean, int]("intPrimitive")),
			),
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			transposeRepresentationPair(t, testCase.name, testCase.register, testCase.transposed)
		})
	}
}

// transposeJsonProvided mirrors MyLocalJsonProvidedMySchema, the class-backed
// JSON schema the Java exec 1 jsonprovided case transposes into.
type transposeJsonProvided struct {
	P0 string `esper:"p0"`
	P1 int    `esper:"p1"`
}

// TestEPLInsertIntoTransposeFunctionToStreamWithPropsParity covers
// EPLInsertIntoTransposeFunctionToStreamWithProps: one named dummy column
// coexists with the transpose payload, routed into a Map target MyStream.
// The Java assertion checks the Pair underlying exposes the payload bean and
// the merged dummy column; Go models this as a pre-registered Map target
// whose properties include the companion column, so the observable contract
// is dummy=1, theString=OI1, intPrimitive=10.
func TestEPLInsertIntoTransposeFunctionToStreamWithPropsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[epSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MyStream", []FieldSpec{
		FieldDef("dummy", reflect.TypeOf(0)),
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(0)),
	}); err != nil {
		t.Fatal(err)
	}

	producer, err := env.Build(Select(
		From[epSupportBean](env, "SupportBean").
			Filter(Like(Field[epSupportBean, string]("theString"), Literal("I%"))),
		Alias("dummy", Literal(1)),
		Selection{Name: "transposed", Expr: Transpose[*epSupportBean](
			Func2[string, int, *epSupportBean]("custom", func(s string, i int) *epSupportBean {
				return &epSupportBean{TheString: s, IntPrimitive: i}
			}, Concat(Literal("O"), Field[epSupportBean, string]("theString")), Literal(10)),
		)},
	).InsertInto("MyStream", StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(FromAny(env, "MyStream").Query(StatementName("s0-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithRuntimeURI("java-runtime-d3220f91b49d423f15d6"))
	collect := &transposeRoutesCollector{}
	deployments := deployPlans(t, engine, producer, consumer)
	collect.subscribe(engine, deployments[1], "s0-consumer")
	defer func() { _ = engine.Close(context.Background()) }()

	if err := engine.SendEvent(context.Background(), epSupportBean{TheString: "I1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	events := collect.snapshot()
	if len(events) != 1 {
		t.Fatalf("routed events = %d", len(events))
	}
	if events[0].Get("dummy").Any() != 1 {
		t.Fatalf("dummy = %v", events[0].Get("dummy").Any())
	}
	if events[0].Get("theString").Any() != "OI1" {
		t.Fatalf("theString = %v", events[0].Get("theString").Any())
	}
	if events[0].Get("intPrimitive").Any() != 10 {
		t.Fatalf("intPrimitive = %v", events[0].Get("intPrimitive").Any())
	}
}

// TestEPLInsertIntoTransposeFunctionToStreamParity covers
// EPLInsertIntoTransposeFunctionToStream: the transpose payload bean itself
// becomes the routed OtherStream event; a filtered consumer reads theString
// and intPrimitive directly from the routed event, and a second producer
// reverts to the same target after the consumer is undeployed.
func TestEPLInsertIntoTransposeFunctionToStreamParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[epSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[epSupportBean](env, "OtherStream"); err != nil {
		t.Fatal(err)
	}

	buildProducer := func(stmtName string) Plan {
		plan, err := env.Build(Select(
			From[epSupportBean](env, "SupportBean").
				Filter(Like(Field[epSupportBean, string]("theString"), Literal("I%"))),
			Selection{Name: "col", Expr: Transpose[*epSupportBean](
				Func2[string, int, *epSupportBean]("custom", func(s string, i int) *epSupportBean {
					return &epSupportBean{TheString: s, IntPrimitive: i}
				}, Concat(Literal("O"), Field[epSupportBean, string]("theString")), Literal(10)),
			)},
		).InsertInto("OtherStream", StatementName(stmtName)))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}

	producerFirst, err := env.Build(Select(
		From[epSupportBean](env, "SupportBean").
			Filter(Like(Field[epSupportBean, string]("theString"), Literal("I%"))),
		Selection{Name: "col", Expr: Transpose[*epSupportBean](
			Func2[string, int, *epSupportBean]("custom", func(s string, i int) *epSupportBean {
				return &epSupportBean{TheString: s, IntPrimitive: i}
			}, Concat(Literal("O"), Field[epSupportBean, string]("theString")), Literal(10)),
		)},
	).InsertInto("OtherStream", StatementName("first")))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(FromAny(env, "OtherStream").
		Filter(Like(Field[epSupportBean, string]("theString"), Literal("O%"))).
		Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithRuntimeURI("java-runtime-8ab48d77b45d6d38fd42"))
	collect := &transposeRoutesCollector{}
	firstDeployments := deployPlans(t, engine, producerFirst, consumer)
	collect.subscribe(engine, firstDeployments[1], "s0")
	defer func() { _ = engine.Close(context.Background()) }()
	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{TheString: "I1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	firstEvents := collect.snapshot()
	if len(firstEvents) != 1 {
		t.Fatalf("first routed events = %d", len(firstEvents))
	}
	if firstEvents[0].Get("theString").Any() != "OI1" {
		t.Fatalf("first theString = %v", firstEvents[0].Get("theString").Any())
	}
	if firstEvents[0].Get("intPrimitive").Any() != 10 {
		t.Fatalf("first intPrimitive = %v", firstEvents[0].Get("intPrimitive").Any())
	}

	// The Java suite deploys a second producer "second" for the
	// already-existing OtherStream, then sends I2, and its own listener
	// observes the routed event carrying theString/intPrimitive (OI2, 10).
	// Mirror that phase order exactly: subscribe "second" before the send so
	// the observed event matches the Java assertion.
	if err := engine.Undeploy(context.Background(), firstDeployments[1].ID()); err != nil {
		t.Fatal(err)
	}
	secondDeployments := deployPlans(t, engine, buildProducer("second"))
	secondEvents := &transposeRoutesCollector{}
	secondEvents.subscribe(engine, secondDeployments[0], "second")
	if err := engine.Send(context.Background(), "SupportBean", epSupportBean{TheString: "I2", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	secondSnapshot := secondEvents.snapshot()
	if len(secondSnapshot) != 1 {
		t.Fatalf("second routed events = %d", len(secondSnapshot))
	}
	if secondSnapshot[0].Get("theString").Any() != "OI2" || secondSnapshot[0].Get("intPrimitive").Any() != 10 {
		t.Fatalf("second routed = %v/%v", secondSnapshot[0].Get("theString").Any(), secondSnapshot[0].Get("intPrimitive").Any())
	}
}

// TestEPLInsertIntoTransposeSingleColumnInsertParity covers
// EPLInsertIntoTransposeSingleColumnInsert: transpose with the same input and
// output type (SupportBean -> SupportBean) and with an explicit alias column
// ignored by the transpose route (SupportBean -> SupportBeanNumeric).
func TestEPLInsertIntoTransposeSingleColumnInsertParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[epSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[filterBeanNumeric](env, "SupportBeanNumeric"); err != nil {
		t.Fatal(err)
	}

	// insert into SupportBean select transpose(customOne('O'||theString, 10))
	// from SupportBean(theString like 'I%'); self-insertion is bounded by the
	// 'I%' filter because the routed theString "OI1" does not match it.
	sameProducer, err := env.Build(Select(
		From[epSupportBean](env, "SupportBean").
			Filter(Like(Field[epSupportBean, string]("theString"), Literal("I%"))),
		Selection{Name: "col", Expr: Transpose[*epSupportBean](
			Func2[string, int, *epSupportBean]("customOne", func(s string, i int) *epSupportBean {
				return &epSupportBean{TheString: s, IntPrimitive: i}
			}, Concat(Literal("O"), Field[epSupportBean, string]("theString")), Literal(10)),
		)},
	).InsertInto("SupportBean", StatementName("same-s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithRuntimeURI("java-runtime-aef71ab1bc6a0d48f2df"))
	sameEvents := &transposeRoutesCollector{}
	sameDeployments := deployPlans(t, engine, sameProducer)
	sameEvents.subscribe(engine, sameDeployments[0], "same-s0")
	defer func() { _ = engine.Close(context.Background()) }()

	if err := engine.SendEvent(context.Background(), epSupportBean{TheString: "I1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	got := sameEvents.snapshot()
	if len(got) != 1 {
		t.Fatalf("same-type routed events = %d", len(got))
	}
	if got[0].Get("theString").Any() != "OI1" || got[0].Get("intPrimitive").Any() != 10 {
		t.Fatalf("same-type routed = %v/%v", got[0].Get("theString").Any(), got[0].Get("intPrimitive").Any())
	}
	if err := engine.Undeploy(context.Background(), sameDeployments[0].ID()); err != nil {
		t.Fatal(err)
	}

	// insert into SupportBeanNumeric select transpose(customTwo(intPrimitive,
	// intPrimitive+1)) as col1 from SupportBean(theString like 'I%'); the name
	// "col1" is ignored by the transpose route.
	numericProducer, err := env.Build(Select(
		From[epSupportBean](env, "SupportBean").
			Filter(Like(Field[epSupportBean, string]("theString"), Literal("I%"))),
		Selection{Name: "col1", Expr: Transpose[*filterBeanNumeric](
			Func2[int, int, *filterBeanNumeric]("customTwo", func(one, two int) *filterBeanNumeric {
				return &filterBeanNumeric{IntOne: one, IntTwo: two}
			}, Field[epSupportBean, int]("intPrimitive"), Add[int](Field[epSupportBean, int]("intPrimitive"), Literal(1))),
		)},
	).InsertInto("SupportBeanNumeric", StatementName("numeric-s0")))
	if err != nil {
		t.Fatal(err)
	}
	numericEvents := &transposeRoutesCollector{}
	numericDeployments := deployPlans(t, engine, numericProducer)
	numericEvents.subscribe(engine, numericDeployments[0], "numeric-s0")

	if err := engine.SendEvent(context.Background(), epSupportBean{TheString: "I2", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	gotNumeric := numericEvents.snapshot()
	if len(gotNumeric) != 1 {
		t.Fatalf("numeric routed events = %d", len(gotNumeric))
	}
	if gotNumeric[0].Get("intOne").Any() != 10 || gotNumeric[0].Get("intTwo").Any() != 11 {
		t.Fatalf("numeric routed = %v/%v", gotNumeric[0].Get("intOne").Any(), gotNumeric[0].Get("intTwo").Any())
	}
}

// TestEPLInsertIntoTransposeEventJoinMapParity covers
// EPLInsertIntoTransposeEventJoinMap: a keepall join of two Map streams
// projects a.id and b.id into MyStreamTE, and the consumer reads the dotted
// nested properties a.id/b.id from the routed event.
func TestEPLInsertIntoTransposeEventJoinMapParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "AEventTE", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "BEventTE", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MyStreamTE", []FieldSpec{
		FieldDef("a", reflect.TypeOf(Event{})),
		FieldDef("b", reflect.TypeOf(Event{})),
	}); err != nil {
		t.Fatal(err)
	}

	producer, err := env.Build(JoinMany(
		JoinRecordSource(FromAny(env, "AEventTE")),
		JoinRecordSource(FromAny(env, "BEventTE")),
	).On().Select(
		SelectSourceEvent(0, "a"),
		SelectSourceEvent(1, "b"),
	).InsertInto("MyStreamTE", StatementName("join-producer")))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(FromAny(env, "MyStreamTE").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithRuntimeURI("java-runtime-fee5f7e9a61131ac2e3a"))
	collect := &transposeRoutesCollector{}
	deployments := deployPlans(t, engine, producer, consumer)
	collect.subscribe(engine, deployments[1], "s0")
	defer func() { _ = engine.Close(context.Background()) }()

	if err := engine.Send(context.Background(), "AEventTE", map[string]any{"id": "A1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "BEventTE", map[string]any{"id": "B1"}); err != nil {
		t.Fatal(err)
	}
	events := collect.snapshot()
	if len(events) != 1 {
		t.Fatalf("join-map routed events = %d", len(events))
	}
	if got := events[0].Get("a.id").Any(); got != "A1" {
		t.Fatalf("a.id = %v", got)
	}
	if got := events[0].Get("b.id").Any(); got != "B1" {
		t.Fatalf("b.id = %v", got)
	}
}

// TestEPLInsertIntoTransposeEventJoinPOJOParity covers
// EPLInsertIntoTransposeEventJoinPOJO: the same join over struct streams
// projects the complete source events under aliases a and b.
func TestEPLInsertIntoTransposeEventJoinPOJOParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[epSupportBeanA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[epSupportBeanB](env, "SupportBean_B"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[epSupportBeanA](env, "SupportBean_A2"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[epSupportBeanB](env, "SupportBean_B2"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinStreamAB](env, "MyStream2Bean"); err != nil {
		t.Fatal(err)
	}

	producer, err := env.Build(JoinMany(
		JoinSource(FromAs[epSupportBeanA](env, "SupportBean_A").As("a")),
		JoinSource(FromAs[epSupportBeanB](env, "SupportBean_B").As("b")),
	).On().Select(
		SelectSourceEvent(0, "a"),
		SelectSourceEvent(1, "b"),
	).InsertInto("MyStream2Bean", StatementName("join-producer")))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(FromAny(env, "MyStream2Bean").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithRuntimeURI("java-runtime-bbabb76978292c0ec5b4"))
	collect := &transposeRoutesCollector{}
	deployments := deployPlans(t, engine, producer, consumer)
	collect.subscribe(engine, deployments[1], "s0")
	defer func() { _ = engine.Close(context.Background()) }()

	if err := engine.Send(context.Background(), "SupportBean_A", epSupportBeanA{ID: "A1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean_B", epSupportBeanB{ID: "B1"}); err != nil {
		t.Fatal(err)
	}
	events := collect.snapshot()
	if len(events) != 1 {
		t.Fatalf("join-pojo routed events = %d", len(events))
	}
	if got := events[0].Get("a.id").Any(); got != "A1" {
		t.Fatalf("a.id = %v", got)
	}
	if got := events[0].Get("b.id").Any(); got != "B1" {
		t.Fatalf("b.id = %v", got)
	}
}

// joinStreamAB is the struct carrying the two joined source events as
// Event-typed nested properties (the Go model of MyStream2Bean).
type joinStreamAB struct {
	A Event `esper:"a"`
	B Event `esper:"b"`
}

// TestEPLInsertIntoTransposePOJOPropertyStreamParity covers
// EPLInsertIntoTransposePOJOPropertyStream: a nested bean property is
// projected as an inneritem column and the consumer reads the dotted nested
// property inneritem.nestedValue from the routed event.
func TestEPLInsertIntoTransposePOJOPropertyStreamParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[goComplexProps](env, "SupportBeanComplexProps"); err != nil {
		t.Fatal(err)
	}
	innerSchema, err := StructSchema[goComplexNested]("MyStreamComplexInner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MyStreamComplex", []FieldSpec{
		FieldDef("inneritem", reflect.TypeOf(goComplexNested{})),
	}, WithNestedPropertySchema("inneritem", innerSchema)); err != nil {
		t.Fatal(err)
	}

	producer, err := env.Build(Select(
		From[goComplexProps](env, "SupportBeanComplexProps"),
		Alias("inneritem", Field[goComplexProps, *goComplexNested]("nested")),
	).InsertInto("MyStreamComplex", StatementName("property-producer")))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(FromAny(env, "MyStreamComplex").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithRuntimeURI("java-runtime-97bbfc3e2d43eec0e9fd"))
	collect := &transposeRoutesCollector{}
	deployments := deployPlans(t, engine, producer, consumer)
	collect.subscribe(engine, deployments[1], "s0")
	defer func() { _ = engine.Close(context.Background()) }()

	if err := engine.Send(context.Background(), "SupportBeanComplexProps",
		goComplexProps{Nested: &goComplexNested{NestedValue: "nestedValue"}}); err != nil {
		t.Fatal(err)
	}
	events := collect.snapshot()
	if len(events) != 1 {
		t.Fatalf("property routed events = %d", len(events))
	}
	if got := events[0].Get("inneritem.nestedValue").Any(); got != "nestedValue" {
		t.Fatalf("inneritem.nestedValue = %v", got)
	}
}
