package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// epSupportBeanWithThis mirrors SupportBeanWithThis used by the ThisAsColumn
// execution. The target schemas attach the source schema to their event-valued
// fragment properties so dotted access remains typed.
type epSupportBeanWithThis struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func (event epSupportBeanWithThis) GetThis() epSupportBeanWithThis { return event }

func transposePatternSnapshot(t *testing.T, engine *Engine, name string) []Event {
	t.Helper()
	window, ok := engine.NamedWindow(name)
	if !ok {
		t.Fatalf("named window %q is missing", name)
	}
	events, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot %q: %v", name, err)
	}
	return events
}

func transposePatternRowsSubscriber(t *testing.T, deployment *Deployment) *[]Row {
	t.Helper()
	rows := make([]Row, 0)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &rows
}

type transposePatternResultSubscriber struct {
	results []Result
}

func (s *transposePatternResultSubscriber) subscribe(t *testing.T, deployment *Deployment, statementName string) {
	t.Helper()
	for _, statement := range deployment.Statements() {
		if statement.Name() != statementName {
			continue
		}
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			s.results = append(s.results, batch.New...)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("statement %q is missing", statementName)
}

func (s *transposePatternResultSubscriber) snapshot() []Result {
	return append([]Result(nil), s.results...)
}

func transposePatternResultProperty(result Result, name string) (any, bool) {
	if row, ok := result.Row(); ok {
		value := row.Get(name)
		return value.Any(), !value.IsMissing()
	}
	if event, ok := result.Event(); ok {
		value := event.Get(name)
		return value.Any(), !value.IsMissing()
	}
	return nil, false
}

// TestEPLInsertIntoThisAsColumnParity covers EPLInsertIntoThisAsColumn.
// PatternQuery.InsertInto targets the registered named windows directly; the
// snapshots mirror the Java iterator assertions after A, B, and C.
func TestEPLInsertIntoThisAsColumnParity(t *testing.T) {
	env := NewEnvironment()
	sourceSchema, err := RegisterStruct[epSupportBeanWithThis](env, "SupportBeanWithThis", WithAccessorStyle(AccessorJavaBean))
	if err != nil {
		t.Fatal(err)
	}

	oneWindowSchema, err := RegisterMap(env, "OneWindow", []FieldSpec{
		FieldDef("alertId", reflect.TypeOf("")),
		FieldDef("this", reflect.TypeOf(Event{})),
	}, WithNestedPropertySchema("this", sourceSchema))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "OneWindow", oneWindowSchema,
		NamedWindowRetention(TimeWindow(24*time.Hour))); err != nil {
		t.Fatal(err)
	}

	twoWindowSchema, err := RegisterMap(env, "TwoWindow", []FieldSpec{
		FieldDef("alertId", reflect.TypeOf("")),
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(0)),
		FieldDef("this", reflect.TypeOf(Event{})),
	}, WithNestedPropertySchema("this", sourceSchema))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "TwoWindow", twoWindowSchema,
		NamedWindowRetention(TimeWindow(24*time.Hour))); err != nil {
		t.Fatal(err)
	}

	source := From[epSupportBeanWithThis](env, "SupportBeanWithThis")
	pattern := func(value string) PatternStream {
		return PatternFrom(source, "quote", Equal[string](
			Field[epSupportBeanWithThis, string]("theString"), Literal(value),
		)).Every()
	}
	producerA, err := env.Build(pattern("A").Select(
		Alias("alertId", Literal("1")),
		Alias("this", PatternEvent("quote")),
	).InsertInto("OneWindow", StatementName("producer-A")))
	if err != nil {
		t.Fatal(err)
	}
	producerB, err := env.Build(pattern("B").Select(
		Alias("alertId", Literal("2")),
		Alias("this", PatternEvent("quote")),
	).InsertInto("OneWindow", StatementName("producer-B")))
	if err != nil {
		t.Fatal(err)
	}
	producerC, err := env.Build(pattern("C").Select(
		Alias("alertId", Literal("3")),
		Alias("theString", TagField[string]("quote", "theString")),
		Alias("intPrimitive", TagField[int]("quote", "intPrimitive")),
		Alias("this", PatternEvent("quote")),
	).InsertInto("TwoWindow", StatementName("producer-C")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env, WithRuntimeURI("java-runtime-b449bbd35d5ff2ee128d"))
	defer func() { _ = engine.Close(context.Background()) }()
	deployPlans(t, engine, producerA, producerB, producerC)

	if err := engine.SendEvent(context.Background(), epSupportBeanWithThis{TheString: "A", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	oneAfterA := transposePatternSnapshot(t, engine, "OneWindow")
	if len(oneAfterA) != 1 {
		t.Fatalf("OneWindow after A = %d events, want 1", len(oneAfterA))
	}
	if got := oneAfterA[0].Get("alertId").Any(); got != "1" {
		t.Fatalf("OneWindow after A alertId = %#v, want 1", got)
	}
	if got := oneAfterA[0].Get("this.intPrimitive").Any(); got != 10 {
		t.Fatalf("OneWindow after A this.intPrimitive = %#v, want 10", got)
	}

	if err := engine.SendEvent(context.Background(), epSupportBeanWithThis{TheString: "B", IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	oneAfterB := transposePatternSnapshot(t, engine, "OneWindow")
	if len(oneAfterB) != 2 {
		t.Fatalf("OneWindow after B = %d events, want 2", len(oneAfterB))
	}
	for index, want := range []struct {
		alertID string
		integer int
	}{{"1", 10}, {"2", 20}} {
		if got := oneAfterB[index].Get("alertId").Any(); got != want.alertID {
			t.Fatalf("OneWindow row %d alertId = %#v, want %q", index, got, want.alertID)
		}
		if got := oneAfterB[index].Get("this.intPrimitive").Any(); got != want.integer {
			t.Fatalf("OneWindow row %d this.intPrimitive = %#v, want %d", index, got, want.integer)
		}
	}

	if err := engine.SendEvent(context.Background(), epSupportBeanWithThis{TheString: "C", IntPrimitive: 30}); err != nil {
		t.Fatal(err)
	}
	twoAfterC := transposePatternSnapshot(t, engine, "TwoWindow")
	if len(twoAfterC) != 1 {
		t.Fatalf("TwoWindow after C = %d events, want 1", len(twoAfterC))
	}
	if got := twoAfterC[0].Get("alertId").Any(); got != "3" {
		t.Fatalf("TwoWindow after C alertId = %#v, want 3", got)
	}
	if got := twoAfterC[0].Get("intPrimitive").Any(); got != 30 {
		t.Fatalf("TwoWindow after C intPrimitive = %#v, want 30", got)
	}
}

// TestEPLInsertIntoTransposePOJOEventPatternParity covers
// EPLInsertIntoTransposePOJOEventPattern: a completed A -> B pattern routes
// the two source events into a bean-backed target, whose consumer reads the
// nested a.id and b.id properties.
func TestEPLInsertIntoTransposePOJOEventPatternParity(t *testing.T) {
	env := NewEnvironment()
	aSchema, err := RegisterStruct[epSupportBeanA](env, "SupportBean_A")
	if err != nil {
		t.Fatal(err)
	}
	bSchema, err := RegisterStruct[epSupportBeanB](env, "SupportBean_B")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MyStreamABBean", []FieldSpec{
		FieldDef("a", reflect.TypeOf(Event{})),
		FieldDef("b", reflect.TypeOf(Event{})),
	}, WithNestedPropertySchema("a", aSchema), WithNestedPropertySchema("b", bSchema)); err != nil {
		t.Fatal(err)
	}

	pattern := PatternFrom(
		From[epSupportBeanA](env, "SupportBean_A"), "a", Literal(true),
	).Then(PatternFrom(
		From[epSupportBeanB](env, "SupportBean_B"), "b", Literal(true),
	))
	producer, err := env.Build(pattern.Select(
		Alias("a", PatternEvent("a")),
		Alias("b", PatternEvent("b")),
	).InsertInto("MyStreamABBean", StatementName("producer")))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(FromAny(env, "MyStreamABBean").Select(
		Alias("a.id", NestedField[string](Field[Event, Event]("a"), "id")),
		Alias("b.id", NestedField[string](Field[Event, Event]("b"), "id")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env, WithRuntimeURI("java-runtime-42af77eea911eeaed74f"))
	defer func() { _ = engine.Close(context.Background()) }()
	deployments := deployPlans(t, engine, producer, consumer)
	rows := transposePatternRowsSubscriber(t, deployments[1])

	if err := engine.SendEvent(context.Background(), epSupportBeanA{ID: "A1"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 0 {
		t.Fatalf("POJO pattern rows after A = %d, want 0", len(*rows))
	}
	if err := engine.SendEvent(context.Background(), epSupportBeanB{ID: "B1"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("POJO pattern rows after B = %d, want 1", len(*rows))
	}
	if got := (*rows)[0].Get("a.id").Any(); got != "A1" {
		t.Fatalf("POJO pattern a.id = %#v, want A1", got)
	}
	if got := (*rows)[0].Get("b.id").Any(); got != "B1" {
		t.Fatalf("POJO pattern b.id = %#v, want B1", got)
	}
}

// TestEPLInsertIntoTransposeMapEventPatternParity covers
// EPLInsertIntoTransposeMapEventPattern. The producer listener verifies the
// two captured map payloads; the consumer verifies dotted ids and the target's
// declared root/fragment metadata.
func TestEPLInsertIntoTransposeMapEventPatternParity(t *testing.T) {
	env := NewEnvironment()
	aSchema, err := RegisterMap(env, "AEventMap", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	bSchema, err := RegisterMap(env, "BEventMap", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	targetSchema, err := RegisterMap(env, "MyStreamABMap", []FieldSpec{
		FieldDef("a", reflect.TypeOf(Event{})),
		FieldDef("b", reflect.TypeOf(Event{})),
	}, WithNestedPropertySchema("a", aSchema), WithNestedPropertySchema("b", bSchema))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if typ, ok := targetSchema.PropertyType(name); !ok || typ != reflect.TypeOf(Event{}) {
			t.Fatalf("MyStreamABMap %s type = %v/%t, want Event", name, typ, ok)
		}
	}
	for _, name := range []string{"a.id", "b.id"} {
		if typ, ok := targetSchema.PropertyType(name); !ok || typ != reflect.TypeOf("") {
			t.Fatalf("MyStreamABMap %s type = %v/%t, want string", name, typ, ok)
		}
	}

	pattern := PatternFromRecord(
		FromAny(env, "AEventMap"), "a", Literal(true),
	).Then(PatternFromRecord(
		FromAny(env, "BEventMap"), "b", Literal(true),
	))
	producer, err := env.Build(pattern.Select(
		Alias("a", PatternEvent("a")),
		Alias("b", PatternEvent("b")),
	).InsertInto("MyStreamABMap", StatementName("i1")))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(FromAny(env, "MyStreamABMap").Select(
		Alias("a.id", NestedField[string](Field[Event, Event]("a"), "id")),
		Alias("b.id", NestedField[string](Field[Event, Event]("b"), "id")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env, WithRuntimeURI("java-runtime-491c5cf7a48b7d584c16"))
	defer func() { _ = engine.Close(context.Background()) }()
	deployments := deployPlans(t, engine, producer, consumer)
	producerResults := &transposePatternResultSubscriber{}
	producerResults.subscribe(t, deployments[0], "i1")
	consumerRows := transposePatternRowsSubscriber(t, deployments[1])

	if err := engine.SendRecord(context.Background(), "AEventMap", map[string]any{"id": "A1"}); err != nil {
		t.Fatal(err)
	}
	if len(producerResults.snapshot()) != 0 || len(*consumerRows) != 0 {
		t.Fatalf("Map pattern rows after A = producer %d consumer %d, want 0/0", len(producerResults.snapshot()), len(*consumerRows))
	}
	if err := engine.SendRecord(context.Background(), "BEventMap", map[string]any{"id": "B1"}); err != nil {
		t.Fatal(err)
	}
	producerResult := producerResults.snapshot()
	if len(producerResult) != 1 || len(*consumerRows) != 1 {
		t.Fatalf("Map pattern rows after B = producer %d consumer %d, want 1/1", len(producerResult), len(*consumerRows))
	}
	for _, want := range []struct {
		name string
		id   string
	}{
		{name: "a", id: "A1"},
		{name: "b", id: "B1"},
	} {
		value, ok := transposePatternResultProperty(producerResult[0], want.name)
		if !ok {
			t.Fatalf("Map pattern %s result = %#v, want property", want.name, producerResult[0])
		}
		event, ok := value.(Event)
		if !ok {
			t.Fatalf("Map pattern %s payload = %#v, want Event fragment", want.name, value)
		}
		payload, ok := event.Underlying().(map[string]any)
		if !ok || payload["id"] != want.id {
			t.Fatalf("Map pattern %s payload = %#v, want map[id:%s]", want.name, event.Underlying(), want.id)
		}
	}
	if got := (*consumerRows)[0].Get("a.id").Any(); got != "A1" {
		t.Fatalf("Map pattern a.id = %#v, want A1", got)
	}
	if got := (*consumerRows)[0].Get("b.id").Any(); got != "B1" {
		t.Fatalf("Map pattern b.id = %#v, want B1", got)
	}
}
