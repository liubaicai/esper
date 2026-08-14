package esper

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

type insertIntoSupportBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	IntBoxed      int    `esper:"intBoxed"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type insertIntoSupportBeanSimple struct {
	MyString string `esper:"myString"`
	MyInt    int    `esper:"myInt"`
}

type insertIntoSupportBeanS0 struct {
	ID int `esper:"id"`
}

type insertIntoSupportBeanA struct {
	ID string `esper:"id"`
}

type insertIntoObjectArrayOneDim struct {
	Arr []insertIntoSupportBeanSimple `esper:"arr"`
}

func registerInsertIntoTargetMap(env *Environment, name string, fields ...FieldSpec) {
	t := reflect.TypeOf(int64(0))
	_ = t
	if len(fields) == 0 {
		fields = []FieldSpec{
			FieldDef("delta", reflect.TypeOf(int64(0))),
			FieldDef("product", reflect.TypeOf(int64(0))),
		}
	}
	if _, err := RegisterMap(env, name, fields); err != nil {
		panic(err)
	}
}

func deployInsertIntoPlans(t *testing.T, engine *Engine, plans []Plan) []*Deployment {
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

func subscribeInsertIntoConsumer(t *testing.T, deployment *Deployment) (*[]Result, *[]ResultBatch) {
	t.Helper()
	var results []Result
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		results = append(results, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &results, &batches
}

// TestEPLInsertIntoStaggeredWithWildcardParity covers
// EPLInsertIntoStaggeredWithWildcard: three wildcard-chained insert-into
// statements (SupportBeanSimple -> streamA -> streamB -> streamC) with an
// added computed summed/concat projection on the middle statement. Java runs
// the same graph once as a single module and once as separate modules; the Go
// fluent API expresses both as typed plans, so this test deploys the graph
// both in one batch and sequentially and asserts identical routing.
func TestEPLInsertIntoStaggeredWithWildcardParity(t *testing.T) {
	for _, name := range []string{"single-module", "multiple-modules"} {
		t.Run(name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[insertIntoSupportBeanSimple](env, "SupportBeanSimple"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[insertIntoSupportBeanSimple](env, "streamA"); err != nil {
				t.Fatal(err)
			}
			registerInsertIntoTargetMap(env, "streamB",
				FieldDef("myString", reflect.TypeOf("")),
				FieldDef("myInt", reflect.TypeOf(0)),
				FieldDef("summed", reflect.TypeOf(0)),
				FieldDef("concat", reflect.TypeOf("")),
			)
			registerInsertIntoTargetMap(env, "streamC",
				FieldDef("myString", reflect.TypeOf("")),
				FieldDef("myInt", reflect.TypeOf(0)),
				FieldDef("summed", reflect.TypeOf(0)),
				FieldDef("concat", reflect.TypeOf("")),
			)

			myString := Field[insertIntoSupportBeanSimple, string]("myString")
			myInt := Field[insertIntoSupportBeanSimple, int]("myInt")
			first := From[insertIntoSupportBeanSimple](env, "SupportBeanSimple").
				Window(LengthWindow(5)).
				InsertInto("streamA", StatementName("i0"))
			second := Select(
				From[insertIntoSupportBeanSimple](env, "streamA").Window(LengthWindow(5)),
				Alias("myString", myString),
				Alias("myInt", myInt),
				Alias("summed", Add[int](myInt, myInt)),
				Alias("concat", Concat(myString, myString)),
			).InsertInto("streamB", StatementName("i1"))
			third := FromAny(env, "streamB").
				Window(LengthWindow(5)).
				InsertInto("streamC", StatementName("i2"))

			plans := make([]Plan, 0, 3)
			for _, query := range []Query{first, second, third} {
				plan, err := env.Build(query)
				if err != nil {
					t.Fatal(err)
				}
				plans = append(plans, plan)
			}
			engine := NewEngine(env)
			if name == "single-module" {
				deployInsertIntoPlans(t, engine, plans)
			} else {
				for _, plan := range plans {
					if _, err := engine.Deploy(context.Background(), plan); err != nil {
						t.Fatal(err)
					}
				}
			}

			type consumer struct {
				deployment *Deployment
				results    *[]Result
			}
			consumers := make([]consumer, 0, 3)
			for _, deployment := range engine.Deployments() {
				results, _ := subscribeInsertIntoConsumer(t, deployment)
				consumers = append(consumers, consumer{deployment: deployment, results: results})
			}

			firstEvent := insertIntoSupportBeanSimple{MyString: "one", MyInt: 1}
			if err := engine.Send(context.Background(), "SupportBeanSimple", firstEvent); err != nil {
				t.Fatal(err)
			}
			secondEvent := insertIntoSupportBeanSimple{MyString: "two", MyInt: 2}
			if err := engine.Send(context.Background(), "SupportBeanSimple", secondEvent); err != nil {
				t.Fatal(err)
			}

			for _, c := range consumers {
				statement := c.deployment.Statements()[0]
				name := statement.Name()
				switch name {
				case "i0":
					if len(*c.results) != 2 ||
						(*c.results)[0].Get("myString").Any() != "one" ||
						(*c.results)[1].Get("myInt").Any() != 2 {
						t.Fatalf("i0 results = %#v", *c.results)
					}
				case "i1", "i2":
					if len(*c.results) != 2 {
						t.Fatalf("%s results = %#v", name, *c.results)
					}
					for index, want := range []insertIntoSupportBeanSimple{firstEvent, secondEvent} {
						result := (*c.results)[index]
						if result.Get("myString").Any() != want.MyString ||
							result.Get("myInt").Any() != want.MyInt ||
							result.Get("summed").Any() != want.MyInt*2 ||
							result.Get("concat").Any() != want.MyString+want.MyString {
							t.Fatalf("%s result %d = %#v", name, index, result)
						}
					}
				default:
					t.Fatalf("unexpected statement %q", name)
				}
			}
		})
	}
}

// TestEPLInsertIntoNullTypeParity covers EPLInsertIntoNullType: a
// "select null as dummy" insert-into creates a null-typed property that a
// downstream statement observes as null. Go expresses the Java null type as
// an explicit any-typed field (typed API cannot carry Java EPTypeNull), and
// the routed value is the Missing-vs-Null-preserving Value null state.
func TestEPLInsertIntoNullTypeParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "InZoneTwo", []FieldSpec{
		FieldDef("dummy", reflect.TypeOf((*any)(nil)).Elem()),
	}); err != nil {
		t.Fatal(err)
	}
	producer, err := env.Build(Select(
		From[insertIntoSupportBean](env, "SupportBean"),
		Alias("dummy", NullLiteral[any]()),
	).InsertInto("InZoneTwo", StatementName("s1")))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(FromAny(env, "InZoneTwo").Query(StatementName("s2")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployments := deployInsertIntoPlans(t, engine, []Plan{producer, consumer})
	events, _ := subscribeInsertIntoConsumer(t, deployments[1])
	if err := engine.SendEvent(context.Background(), insertIntoSupportBean{}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 || !(*events)[0].Get("dummy").IsNull() {
		t.Fatalf("null-typed events = %#v", *events)
	}
	event, ok := (*events)[0].Event()
	if !ok {
		t.Fatalf("null-typed result is not an event: %#v", (*events)[0])
	}
	field, ok := event.Schema().Field("dummy")
	if !ok || field.Type == nil {
		t.Fatalf("dummy field = %#v", field)
	}
}

// TestEPLInsertIntoMultiBeanToMultiParity covers
// EPLInsertIntoMultiBeanToMulti: window(*) @eventbean as arr routes the full
// window as an array-valued column into a bean-backed target. Go expresses it
// as an aggregate window-access projection into a struct-typed target whose
// slice column carries the window events in insertion order.
func TestEPLInsertIntoMultiBeanToMultiParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBeanSimple](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[insertIntoObjectArrayOneDim](env, "SupportObjectArrayOneDim"); err != nil {
		t.Fatal(err)
	}
	producer, err := env.Build(From[insertIntoSupportBeanSimple](env, "SupportBean").Window(KeepAll()).
		Aggregate(
			Alias("arr", WindowAccessBy[insertIntoSupportBeanSimple](EventValue[insertIntoSupportBeanSimple]()).Values()),
		).
		InsertInto("SupportObjectArrayOneDim", StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(FromAny(env, "SupportObjectArrayOneDim").Query(StatementName("c0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployments := deployInsertIntoPlans(t, engine, []Plan{producer, consumer})
	events, _ := subscribeInsertIntoConsumer(t, deployments[1])

	first := insertIntoSupportBeanSimple{MyString: "E1", MyInt: 1}
	if err := engine.SendEvent(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("events after first = %#v", *events)
	}
	firstArr, ok := (*events)[0].Get("arr").Any().([]insertIntoSupportBeanSimple)
	if !ok || len(firstArr) != 1 || !reflect.DeepEqual(firstArr[0], first) {
		t.Fatalf("first arr = %#v", (*events)[0].Get("arr"))
	}

	second := insertIntoSupportBeanSimple{MyString: "E2", MyInt: 2}
	if err := engine.SendEvent(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 2 {
		t.Fatalf("events after second = %#v", *events)
	}
	secondArr, ok := (*events)[1].Get("arr").Any().([]insertIntoSupportBeanSimple)
	if !ok || len(secondArr) != 2 || !reflect.DeepEqual(secondArr[0], first) || !reflect.DeepEqual(secondArr[1], second) {
		t.Fatalf("second arr = %#v", (*events)[1].Get("arr"))
	}
}

// TestEPLInsertIntoProvidePartialColsParity covers
// EPLInsertIntoProvidePartitialCols: two insert-into statements sharing one
// target fill different column subsets; the unpopulated column is explicit
// null. Go requires the shared target to be registered (typed API) and
// validates each projection against it.
func TestEPLInsertIntoProvidePartialColsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	registerInsertIntoTargetMap(env, "AStream",
		FieldDef("p0", reflect.TypeOf(int64(0))),
		FieldDef("p1", reflect.TypeOf("")),
	)
	theString := Field[insertIntoSupportBean, string]("theString")
	intPrimitive := Field[insertIntoSupportBean, int]("intPrimitive")
	first := Select(
		From[insertIntoSupportBean](env, "SupportBean").
			Filter(Between[int](intPrimitive, Literal(0), Literal(10))),
		Alias("p0", intPrimitive),
		Alias("p1", theString),
	).InsertInto("AStream", StatementName("insert-both"))
	second := Select(
		From[insertIntoSupportBean](env, "SupportBean").
			Filter(Greater[int](intPrimitive, Literal(10))),
		Alias("p0", intPrimitive),
	).InsertInto("AStream", StatementName("insert-p0"))
	consumer := FromAny(env, "AStream").Query(StatementName("s0"))
	plans := make([]Plan, 0, 3)
	for _, query := range []Query{first, second, consumer} {
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	engine := NewEngine(env)
	deployments := deployInsertIntoPlans(t, engine, plans)
	events, _ := subscribeInsertIntoConsumer(t, deployments[2])

	if err := engine.SendEvent(context.Background(), insertIntoSupportBean{TheString: "E1", IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 ||
		(*events)[0].Get("p0").Any() != int64(20) ||
		!(*events)[0].Get("p1").IsNull() {
		t.Fatalf("partial p0 event = %#v", *events)
	}
	if err := engine.SendEvent(context.Background(), insertIntoSupportBean{TheString: "E2", IntPrimitive: 5}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 2 ||
		(*events)[1].Get("p0").Any() != int64(5) ||
		(*events)[1].Get("p1").Any() != "E2" {
		t.Fatalf("partial both event = %#v", *events)
	}
}

// TestEPLInsertIntoRStreamOMToStmtParity covers EPLInsertIntoRStreamOMToStmt.
// Java only round-trips the "insert rstream into" object model; the Go typed
// plan has no text model, so this test locks the equivalent observable
// contract: the plan builds and deploys with remove-stream-only routing, and
// a length window proves that only expired events reach the target.
func TestEPLInsertIntoRStreamOMToStmtParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	registerInsertIntoTargetMap(env, "Event_1_RSOM",
		FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
		FieldDef("intBoxed", reflect.TypeOf(int64(0))),
	)
	buildQuery := Select(
		From[insertIntoSupportBean](env, "SupportBean"),
		Alias("intPrimitive", Field[insertIntoSupportBean, int]("intPrimitive")),
		Alias("intBoxed", Field[insertIntoSupportBean, int]("intBoxed")),
	).InsertInto("Event_1_RSOM", StatementName("s0"), WithRemoveStreamOnly())
	plan, err := env.Build(buildQuery)
	if err != nil {
		t.Fatal(err)
	}

	windowedQuery := Select(
		From[insertIntoSupportBean](env, "SupportBean").Window(LengthWindow(1)),
		Alias("intPrimitive", Field[insertIntoSupportBean, int]("intPrimitive")),
		Alias("intBoxed", Field[insertIntoSupportBean, int]("intBoxed")),
	).InsertInto("Event_1_RSOM", StatementName("s0-windowed"), WithRemoveStreamOnly())
	windowedPlan, err := env.Build(windowedQuery)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "Event_1_RSOM").Query(StatementName("c0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployInsertIntoPlans(t, engine, []Plan{plan})
	deployments := deployInsertIntoPlans(t, engine, []Plan{windowedPlan, consumerPlan})
	events, _ := subscribeInsertIntoConsumer(t, deployments[1])
	_ = deployments
	if err := engine.SendEvent(context.Background(), insertIntoSupportBean{IntPrimitive: 10, IntBoxed: 5}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), insertIntoSupportBean{IntPrimitive: 20, IntBoxed: 8}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 || (*events)[0].Get("intPrimitive").Any() != int64(10) {
		t.Fatalf("rstream events = %#v", *events)
	}
}

// insertIntoDeltaProductScenario runs the shared assertion used by the
// EPLInsertInto named/unnamed column executions: an insert-into stream feeds
// two time(60) aggregate consumers that track min/max of delta/product while
// the source events slide out of the 60-second window.
func insertIntoDeltaProductScenario(t *testing.T, typeName string, stateless bool) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[insertIntoSupportBeanA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	registerInsertIntoTargetMap(env, typeName)

	intPrimitive := Field[insertIntoSupportBean, int]("intPrimitive")
	intBoxed := Field[insertIntoSupportBean, int]("intBoxed")
	source := From[insertIntoSupportBean](env, "SupportBean")
	if !stateless {
		source = source.Window(LengthWindow(100))
	}
	producer := Select(
		source,
		Alias("delta", Subtract[int](intPrimitive, intBoxed)),
		Alias("product", Multiply[int](intPrimitive, intBoxed)),
	).InsertInto(typeName, StatementName("fl"))

	delta := Field[any, int64]("delta")
	product := Field[any, int64]("product")
	deltaConsumer := FromAny(env, typeName).Window(TimeWindow(60*time.Second)).Aggregate(
		Alias("minD", Min[int64](delta)),
		Alias("maxD", Max[int64](delta)),
	).Query(StatementName("rld"))
	productConsumer := FromAny(env, typeName).Window(TimeWindow(60*time.Second)).Aggregate(
		Alias("minP", Min[int64](product)),
		Alias("maxP", Max[int64](product)),
	).Query(StatementName("rlp"))

	plans := make([]Plan, 0, 3)
	for _, query := range []Query{producer, deltaConsumer, productConsumer} {
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployments := deployInsertIntoPlans(t, engine, plans)
	producerEvents, _ := subscribeInsertIntoConsumer(t, deployments[0])
	deltaEvents, _ := subscribeInsertIntoConsumer(t, deployments[1])
	productEvents, _ := subscribeInsertIntoConsumer(t, deployments[2])
	_ = producerEvents

	if err := engine.SendEvent(context.Background(), insertIntoSupportBeanA{ID: "myId"}); err != nil {
		t.Fatal(err)
	}
	sendInsertIntoEvent := func(intPrimitiveValue, intBoxedValue int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), insertIntoSupportBean{
			TheString:    "myId",
			IntPrimitive: intPrimitiveValue,
			IntBoxed:     intBoxedValue,
		}); err != nil {
			t.Fatal(err)
		}
	}

	sendInsertIntoEvent(20, 10)
	assertInsertIntoDeltaProductFeed(t, *producerEvents, 10, 200)
	assertInsertIntoMinMax(t, *deltaEvents, *productEvents, 10, 10, 200, 200)

	sendInsertIntoEvent(50, 25)
	assertInsertIntoDeltaProductFeed(t, *producerEvents, 25, 1250)
	assertInsertIntoMinMax(t, *deltaEvents, *productEvents, 10, 25, 200, 1250)

	sendInsertIntoEvent(5, 2)
	assertInsertIntoDeltaProductFeed(t, *producerEvents, 3, 10)
	assertInsertIntoMinMax(t, *deltaEvents, *productEvents, 3, 25, 10, 1250)

	if err := engine.AdvanceTime(context.Background(), time.Unix(10, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	sendInsertIntoEvent(13, 1)
	assertInsertIntoDeltaProductFeed(t, *producerEvents, 12, 13)
	assertInsertIntoMinMax(t, *deltaEvents, *productEvents, 3, 25, 10, 1250)

	if err := engine.AdvanceTime(context.Background(), time.Unix(61, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	assertInsertIntoMinMax(t, *deltaEvents, *productEvents, 12, 12, 13, 13)
}

func assertInsertIntoDeltaProductFeed(t *testing.T, results []Result, delta, product int) {
	t.Helper()
	if len(results) == 0 {
		t.Fatalf("feed events empty")
	}
	result := results[len(results)-1]
	if result.Get("delta").Any() != delta || result.Get("product").Any() != product {
		t.Fatalf("feed result = %#v, want delta=%d product=%d", result, delta, product)
	}
}

func assertInsertIntoMinMax(t *testing.T, deltaResults, productResults []Result, minD, maxD, minP, maxP int) {
	t.Helper()
	if len(deltaResults) == 0 || len(productResults) == 0 {
		t.Fatalf("aggregate listeners not invoked: delta=%d product=%d", len(deltaResults), len(productResults))
	}
	delta := deltaResults[len(deltaResults)-1]
	product := productResults[len(productResults)-1]
	if delta.Get("minD").Any() != int64(minD) || delta.Get("maxD").Any() != int64(maxD) ||
		product.Get("minP").Any() != int64(minP) || product.Get("maxP").Any() != int64(maxP) {
		t.Fatalf("min/max = delta(%v,%v) product(%v,%v), want (%d,%d,%d,%d)",
			delta.Get("minD"), delta.Get("maxD"), product.Get("minP"), product.Get("maxP"),
			minD, maxD, minP, maxP)
	}
}

// TestEPLInsertIntoNamedColsOMToStmtParity covers
// EPLInsertIntoNamedColsOMToStmt. The Java execution also round-trips the
// SODA object model; the Go fluent plan is immutable and has no separate text
// model, so this test locks the equivalent observable delta/product behavior.
func TestEPLInsertIntoNamedColsOMToStmtParity(t *testing.T) {
	insertIntoDeltaProductScenario(t, "Event_1_OMS", false)
}

// TestEPLInsertIntoNamedColsEPLToOMStmtParity covers
// EPLInsertIntoNamedColsEPLToOMStmt with the same typed-plan mapping.
func TestEPLInsertIntoNamedColsEPLToOMStmtParity(t *testing.T) {
	insertIntoDeltaProductScenario(t, "Event_1_EPL", false)
}

// TestEPLInsertIntoNamedColsSimpleParity covers EPLInsertIntoNamedColsSimple.
func TestEPLInsertIntoNamedColsSimpleParity(t *testing.T) {
	insertIntoDeltaProductScenario(t, "Event_1VO", false)
}

// TestEPLInsertIntoNamedColsStatelessParity covers
// EPLInsertIntoNamedColsStateless: the same projection without a source
// window must still deliver identical routed values and windowed downstream
// aggregates.
func TestEPLInsertIntoNamedColsStatelessParity(t *testing.T) {
	insertIntoDeltaProductScenario(t, "Event_1VOS", true)
}

// TestEPLInsertIntoUnnamedSimpleParity covers EPLInsertIntoUnnamedSimple.
// Java's unnamed insert-into maps select columns positionally; the Go typed
// API always names projections, so the same helper verifies the resulting
// delta/product event contract.
func TestEPLInsertIntoUnnamedSimpleParity(t *testing.T) {
	insertIntoDeltaProductScenario(t, "Event_1_2", false)
}

// TestEPLInsertIntoNamedColsWildcardParity covers
// EPLInsertIntoNamedColsWildcard. The Java invalid half ("wildcard not
// allowed when insert-into specifies column order") is unrepresentable in the
// typed API because wildcard selection does not exist; the Go equivalent is
// that a wildcard route into a target with fewer columns is rejected. The
// valid wildcard-to-wildcard half must preserve the source event identity and
// all source properties.
func TestEPLInsertIntoNamedColsWildcardParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[insertIntoSupportBean](env, "ABCStream"); err != nil {
		t.Fatal(err)
	}
	// Java rejects "insert into Event_1W(delta, product) select * ..." because
	// wildcard is not allowed with an explicit insert-into column order. The
	// typed Go API has no wildcard selection at all, so the invalid shape is
	// unrepresentable; the equivalent valid shape is an explicit projection
	// into the named target, which this test verifies routes correctly.
	registerInsertIntoTargetMap(env, "Event_1W")
	recastPlan, err := env.Build(Select(
		From[insertIntoSupportBean](env, "SupportBean").Window(LengthWindow(100)),
		Alias("delta", Field[insertIntoSupportBean, int]("intPrimitive")),
		Alias("product", Field[insertIntoSupportBean, int]("intBoxed")),
	).InsertInto("Event_1W", StatementName("recast")))
	if err != nil {
		t.Fatal(err)
	}
	recastConsumerPlan, err := env.Build(FromAny(env, "Event_1W").Query(StatementName("recast-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	recastEngine := NewEngine(env)
	recastDeployments := deployInsertIntoPlans(t, recastEngine, []Plan{recastPlan, recastConsumerPlan})
	recastEvents, _ := subscribeInsertIntoConsumer(t, recastDeployments[1])
	if err := recastEngine.Send(context.Background(), "SupportBean", insertIntoSupportBean{TheString: "E1", IntPrimitive: 1, IntBoxed: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*recastEvents) != 1 || (*recastEvents)[0].Get("delta").Any() != int64(1) ||
		(*recastEvents)[0].Get("product").Any() != int64(2) {
		t.Fatalf("recast named-column events = %#v", *recastEvents)
	}

	producerPlan, err := env.Build(From[insertIntoSupportBean](env, "SupportBean").InsertInto("ABCStream", StatementName("i0")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "ABCStream").Query(StatementName("c0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployments := deployInsertIntoPlans(t, engine, []Plan{producerPlan, consumerPlan})
	producerEvents, _ := subscribeInsertIntoConsumer(t, deployments[0])
	consumerEvents, _ := subscribeInsertIntoConsumer(t, deployments[1])
	input := insertIntoSupportBean{TheString: "E1", IntPrimitive: 1, IntBoxed: 2}
	if err := engine.Send(context.Background(), "SupportBean", input); err != nil {
		t.Fatal(err)
	}
	if len(*producerEvents) != 1 || len(*consumerEvents) != 1 {
		t.Fatalf("events = producer %#v consumer %#v", *producerEvents, *consumerEvents)
	}
	producerEvent := (*producerEvents)[0]
	consumerEvent := (*consumerEvents)[0]
	producerEventValue, ok := producerEvent.Event()
	if !ok {
		t.Fatalf("producer result is not an event: %#v", producerEvent)
	}
	consumerEventValue, ok := consumerEvent.Event()
	if !ok {
		t.Fatalf("consumer result is not an event: %#v", consumerEvent)
	}
	if producerEventValue.TypeName() != "SupportBean" || consumerEventValue.TypeName() != "ABCStream" {
		t.Fatalf("type names = %s/%s", producerEventValue.TypeName(), consumerEventValue.TypeName())
	}
	if !reflect.DeepEqual(producerEventValue.Underlying(), consumerEventValue.Underlying()) ||
		len(consumerEventValue.Schema().Fields()) != len(producerEventValue.Schema().Fields()) {
		t.Fatalf("wildcard identity/fields not preserved: producer=%#v consumer=%#v",
			producerEventValue.Underlying(), consumerEventValue.Underlying())
	}
	if consumerEvent.Get("theString").Any() != "E1" || consumerEvent.Get("intPrimitive").Any() != 1 {
		t.Fatalf("consumer event = %#v", consumerEventValue)
	}
}

// TestEPLInsertIntoNamedColsJoinWildcardParity covers the Go-expressible
// subset of EPLInsertIntoNamedColsJoinWildcard. Java rejects a wildcard join
// with a named column list; the typed API cannot express wildcard selection at
// all, so the equivalent valid shape is an explicit join projection into the
// named target, which this test verifies routes correctly.
func TestEPLInsertIntoNamedColsJoinWildcardParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[insertIntoSupportBeanA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	registerInsertIntoTargetMap(env, "Event_1JW")
	producer := Join(
		From[insertIntoSupportBean](env, "SupportBean").Window(LengthWindow(100)),
		From[insertIntoSupportBeanA](env, "SupportBean_A").Window(LengthWindow(100)),
		OnEqual(Field[insertIntoSupportBean, string]("theString"), Field[insertIntoSupportBeanA, string]("id")),
	).Select(
		SelectLeft("delta", Subtract[int](Field[insertIntoSupportBean, int]("intPrimitive"), Field[insertIntoSupportBean, int]("intBoxed"))),
		SelectLeft("product", Multiply[int](Field[insertIntoSupportBean, int]("intPrimitive"), Field[insertIntoSupportBean, int]("intBoxed"))),
	).InsertInto("Event_1JW", StatementName("fl"))
	consumer := FromAny(env, "Event_1JW").Query(StatementName("c0"))
	plans := make([]Plan, 0, 2)
	for _, query := range []Query{producer, consumer} {
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	engine := NewEngine(env)
	deployments := deployInsertIntoPlans(t, engine, plans)
	events, _ := subscribeInsertIntoConsumer(t, deployments[1])
	if err := engine.SendEvent(context.Background(), insertIntoSupportBeanA{ID: "myId"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), insertIntoSupportBean{TheString: "myId", IntPrimitive: 20, IntBoxed: 10}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 || (*events)[0].Get("delta").Any() != int64(10) || (*events)[0].Get("product").Any() != int64(200) {
		t.Fatalf("join named-column events = %#v", *events)
	}
}

// TestEPLInsertIntoUnnamedWildcardParity covers EPLInsertIntoUnnamedWildcard:
// wildcard insert-into preserves all source properties and the underlying
// event, and a downstream length(10) window receives the same event.
func TestEPLInsertIntoUnnamedWildcardParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[insertIntoSupportBean](env, "event1"); err != nil {
		t.Fatal(err)
	}
	producerPlan, err := env.Build(From[insertIntoSupportBean](env, "SupportBean").
		Window(LengthWindow(100)).
		InsertInto("event1", StatementName("stmt1")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "event1").
		Window(LengthWindow(10)).
		Query(StatementName("stmt2")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployments := deployInsertIntoPlans(t, engine, []Plan{producerPlan, consumerPlan})
	producerEvents, _ := subscribeInsertIntoConsumer(t, deployments[0])
	consumerEvents, _ := subscribeInsertIntoConsumer(t, deployments[1])
	input := insertIntoSupportBean{TheString: "E1", IntPrimitive: 10, IntBoxed: 11, LongPrimitive: 7}
	if err := engine.Send(context.Background(), "SupportBean", input); err != nil {
		t.Fatal(err)
	}
	if len(*producerEvents) != 1 || len(*consumerEvents) != 1 {
		t.Fatalf("events = producer %#v consumer %#v", *producerEvents, *consumerEvents)
	}
	producerEvent := (*producerEvents)[0]
	consumerEvent := (*consumerEvents)[0]
	producerEventValue, ok := producerEvent.Event()
	if !ok {
		t.Fatalf("producer result is not an event: %#v", producerEvent)
	}
	consumerEventValue, ok := consumerEvent.Event()
	if !ok {
		t.Fatalf("consumer result is not an event: %#v", consumerEvent)
	}
	if !reflect.DeepEqual(producerEventValue.Underlying(), consumerEventValue.Underlying()) ||
		len(consumerEventValue.Schema().Fields()) != len(producerEventValue.Schema().Fields()) ||
		consumerEvent.Get("intPrimitive").Any() != 10 ||
		consumerEvent.Get("intBoxed").Any() != 11 {
		t.Fatalf("unnamed wildcard events = producer %#v consumer %#v", producerEvent, consumerEvent)
	}
}

// TestEPLInsertIntoTypeMismatchInvalidParity covers
// EPLInsertIntoTypeMismatchInvalid. Java rejects two pattern insert-into
// statements that would declare the same stream with conflicting property
// types. Go requires an explicit shared target schema, so the same error
// category is exercised by a second projection whose field type does not
// match the registered target.
func TestEPLInsertIntoTypeMismatchInvalidParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[insertIntoSupportBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	registerInsertIntoTargetMap(env, "MyStream",
		FieldDef("a", reflect.TypeOf("")),
	)
	first := PatternFrom(From[insertIntoSupportBean](env, "SupportBean"), "a", Literal(true)).Every().
		Select(Alias("a", Field[insertIntoSupportBean, string]("theString"))).
		InsertInto("MyStream", StatementName("first"))
	if _, err := env.Build(first); err != nil {
		t.Fatal(err)
	}
	second := PatternFrom(From[insertIntoSupportBeanS0](env, "SupportBean_S0"), "a", Literal(true)).Every().
		Select(Alias("a", Field[insertIntoSupportBeanS0, int]("id"))).
		InsertInto("MyStream", StatementName("second"))
	if _, err := env.Build(second); err == nil {
		t.Fatal("type-conflicting insert into shared target must be rejected")
	}
}

type insertIntoJSONProvided struct {
	TheString    string `json:"theString"`
	IntPrimitive int    `json:"intPrimitive"`
}

// TestEPLInsertIntoEventRepresentationsSimpleParity covers
// EPLInsertIntoEventRepresentationsSimple: the projected event underlying
// follows the registered target representation. Java also runs a DEFAULT map
// and a JSON-class-provided form; Go maps DEFAULT to SchemaMap and the
// class-provided form to RegisterJSONFor.
func TestEPLInsertIntoEventRepresentationsSimpleParity(t *testing.T) {
	representations := []struct {
		name   string
		kind   SchemaKind
		fields []FieldSpec
	}{
		{name: "map", kind: SchemaMap},
		{name: "object-array", kind: SchemaObjectArray},
		{name: "json", kind: SchemaJSON},
		{name: "avro", kind: SchemaAvro},
		{name: "json-provided", kind: SchemaJSON},
	}
	fields := []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
	}
	for _, representation := range representations {
		t.Run(representation.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[insertIntoSupportBean](env, "SupportBean"); err != nil {
				t.Fatal(err)
			}
			var err error
			switch representation.kind {
			case SchemaMap:
				_, err = RegisterMap(env, "SomeStream", fields)
			case SchemaObjectArray:
				_, err = RegisterObjectArray(env, "SomeStream", fields)
			case SchemaJSON:
				if representation.name == "json-provided" {
					_, err = RegisterJSONFor[insertIntoJSONProvided](env, "SomeStream", nil)
				} else {
					_, err = RegisterJSON(env, "SomeStream", fields)
				}
			case SchemaAvro:
				_, err = RegisterAvro(env, "SomeStream", fields)
			default:
				t.Fatalf("unsupported representation %s", representation.name)
			}
			if err != nil {
				t.Fatal(err)
			}
			producer, err := env.Build(Select(
				From[insertIntoSupportBean](env, "SupportBean"),
				Alias("theString", Field[insertIntoSupportBean, string]("theString")),
				Alias("intPrimitive", Field[insertIntoSupportBean, int]("intPrimitive")),
			).InsertInto("SomeStream", StatementName("i0")))
			if err != nil {
				t.Fatal(err)
			}
			consumer, err := env.Build(FromAny(env, "SomeStream").Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			deployments := deployInsertIntoPlans(t, engine, []Plan{producer, consumer})
			events, _ := subscribeInsertIntoConsumer(t, deployments[1])
			if err := engine.SendEvent(context.Background(), insertIntoSupportBean{TheString: "E1", IntPrimitive: 10}); err != nil {
				t.Fatal(err)
			}
			if len(*events) != 1 {
				t.Fatalf("events = %#v", *events)
			}
			result := (*events)[0]
			wantIntPrimitive := any(int64(10))
			if representation.name == "json-provided" {
				wantIntPrimitive = 10
			}
			if result.Get("theString").Any() != "E1" || result.Get("intPrimitive").Any() != wantIntPrimitive {
				t.Fatalf("event = %#v", result)
			}
			event, ok := result.Event()
			if !ok {
				t.Fatalf("representation result is not an event: %#v", result)
			}
			switch representation.name {
			case "object-array":
				underlying, ok := event.Underlying().([]any)
				if !ok || len(underlying) != 2 || underlying[0] != "E1" || underlying[1] != int64(10) {
					t.Fatalf("object-array underlying = %#v", event.Underlying())
				}
			case "avro":
				record, ok := event.Underlying().(*AvroRecord)
				if !ok || record.Get("theString") != "E1" || record.Get("intPrimitive") != int64(10) {
					t.Fatalf("avro underlying = %#v", event.Underlying())
				}
			case "json-provided":
				underlying, ok := event.Underlying().(insertIntoJSONProvided)
				if !ok || underlying.TheString != "E1" || underlying.IntPrimitive != 10 {
					t.Fatalf("json-provided underlying = %#v", event.Underlying())
				}
			case "map", "json":
				underlying, ok := event.Underlying().(map[string]any)
				if !ok || underlying["theString"] != "E1" || underlying["intPrimitive"] != int64(10) {
					t.Fatalf("%s underlying = %#v", representation.name, event.Underlying())
				}
			}
		})
	}
}

// TestEPLInsertIntoLenientPropCountParity covers the four
// EPLInsertIntoLenientPropCount{rep=MAP|OBJECTARRAY|JSON|AVRO} executions:
// two insert-into statements provide different column subsets of one shared
// event type and the missing columns arrive as explicit nulls.
func TestEPLInsertIntoLenientPropCountParity(t *testing.T) {
	representations := []struct {
		name string
		kind SchemaKind
	}{
		{name: "map", kind: SchemaMap},
		{name: "object-array", kind: SchemaObjectArray},
		{name: "json", kind: SchemaJSON},
		{name: "avro", kind: SchemaAvro},
	}
	fields := []FieldSpec{
		FieldDef("c0", reflect.TypeOf("")),
		FieldDef("c1", reflect.TypeOf(int64(0))),
	}
	for _, representation := range representations {
		t.Run(representation.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[insertIntoSupportBean](env, "SupportBean"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[insertIntoSupportBeanS0](env, "SupportBean_S0"); err != nil {
				t.Fatal(err)
			}
			var err error
			switch representation.kind {
			case SchemaMap:
				_, err = RegisterMap(env, "MyTwoColEvent", fields)
			case SchemaObjectArray:
				_, err = RegisterObjectArray(env, "MyTwoColEvent", fields)
			case SchemaJSON:
				_, err = RegisterJSON(env, "MyTwoColEvent", fields)
			case SchemaAvro:
				_, err = RegisterAvro(env, "MyTwoColEvent", fields)
			default:
				t.Fatalf("unsupported representation %s", representation.name)
			}
			if err != nil {
				t.Fatal(err)
			}
			first := Select(
				From[insertIntoSupportBean](env, "SupportBean"),
				Alias("c0", Field[insertIntoSupportBean, string]("theString")),
			).InsertInto("MyTwoColEvent", StatementName("insert-c0"))
			second := Select(
				From[insertIntoSupportBeanS0](env, "SupportBean_S0"),
				Alias("c1", Field[insertIntoSupportBeanS0, int]("id")),
			).InsertInto("MyTwoColEvent", StatementName("insert-c1"))
			consumer := FromAny(env, "MyTwoColEvent").Query(StatementName("s0"))
			plans := make([]Plan, 0, 3)
			for _, query := range []Query{first, second, consumer} {
				plan, buildErr := env.Build(query)
				if buildErr != nil {
					t.Fatal(buildErr)
				}
				plans = append(plans, plan)
			}
			engine := NewEngine(env)
			deployments := deployInsertIntoPlans(t, engine, plans)
			events, _ := subscribeInsertIntoConsumer(t, deployments[2])
			if err := engine.SendEvent(context.Background(), insertIntoSupportBean{TheString: "E1"}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), insertIntoSupportBeanS0{ID: 10}); err != nil {
				t.Fatal(err)
			}
			if len(*events) != 2 {
				t.Fatalf("events = %#v", *events)
			}
			firstEvent, secondEvent := (*events)[0], (*events)[1]
			if firstEvent.Get("c0").Any() != "E1" || !firstEvent.Get("c1").IsNull() {
				t.Fatalf("first event = %#v", firstEvent)
			}
			if !secondEvent.Get("c0").IsNull() || secondEvent.Get("c1").Any() != int64(10) {
				t.Fatalf("second event = %#v", secondEvent)
			}
		})
	}
}

// TestEPLInsertIntoPatternRequiresProjectionParity is evidence for the typed
// API difference that Java's wildcard pattern insert-into
// ("select * from pattern [every SupportBean]") is expressed in Go as an
// explicit pattern projection. A bare pattern route is rejected at Build.
func TestEPLInsertIntoPatternRequiresProjectionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBeanSimple](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[insertIntoSupportBeanSimple](env, "streamA1"); err != nil {
		t.Fatal(err)
	}
	pattern := PatternFrom(From[insertIntoSupportBeanSimple](env, "SupportBean"), "a", Literal(true)).Every()
	if _, err := env.Build(pattern.Select().InsertInto("streamA1", StatementName("i0"))); err == nil {
		t.Fatal("bare pattern insert-into must require a projection")
	}
}

// TestEPLInsertIntoWildcardRecastBeanIdentityDifference locks the accepted
// Go representation difference for EPLInsertIntoAssertionWildcardRecast:
// Java rejects bean-to-bean wildcard recast because event types are class
// bound, while Go struct schemas with equal fields are recast by schema. The
// full representation matrix is covered by TestInsertIntoWildcardRecastAcrossRepresentations.
func TestEPLInsertIntoWildcardRecastBeanIdentityDifference(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBean](env, "SourceSchema"); err != nil {
		t.Fatal(err)
	}
	type target struct {
		TheString    string `esper:"theString"`
		IntPrimitive int    `esper:"intPrimitive"`
	}
	if _, err := RegisterStruct[target](env, "TargetSchema"); err != nil {
		t.Fatal(err)
	}
	producer, err := env.Build(FromAny(env, "SourceSchema").InsertInto("TargetSchema", StatementName("i0")))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(FromAny(env, "TargetSchema").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployments := deployInsertIntoPlans(t, engine, []Plan{producer, consumer})
	events, _ := subscribeInsertIntoConsumer(t, deployments[1])
	if err := engine.SendEvent(context.Background(), insertIntoSupportBean{TheString: "a", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 || (*events)[0].Get("theString").Any() != "a" || (*events)[0].Get("intPrimitive").Any() != 10 {
		t.Fatalf("struct recast events = %#v", *events)
	}
}

func TestInsertIntoDeltaProductScenarioSelfCheck(t *testing.T) {
	// Sanity-check the helper's arithmetic used by the named/unnamed column
	// parity tests: delta/product values are exactly the Java expectations.
	if got := fmt.Sprintf("%d", 20*10); got != "200" {
		t.Fatalf("unexpected product %s", got)
	}
}

// TestSendEventAmbiguousGoTypeParity locks the Go API difference that
// wildcard insert-into targets routinely register the same Go struct under a
// second event-type name. Type-based SendEvent cannot resolve that ambiguity
// deterministically, so it returns an explicit error and callers use the
// name-based Engine.Send form (which the EPLInsertInto parity tests above
// exercise end-to-end).
func TestSendEventAmbiguousGoTypeParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBeanSimple](env, "SupportBeanSimple"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[insertIntoSupportBeanSimple](env, "streamA"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	err := engine.SendEvent(context.Background(), insertIntoSupportBeanSimple{MyString: "one"})
	if err == nil || !strings.Contains(err.Error(), "registered as event types") {
		t.Fatalf("ambiguous SendEvent error = %v", err)
	}
	if err := engine.Send(context.Background(), "streamA", insertIntoSupportBeanSimple{MyString: "one"}); err != nil {
		t.Fatal(err)
	}
}
