package esper

import (
	"context"
	"reflect"
	"testing"
)

// TestEPLInsertIntoIRStreamFuncParity covers
// EPLInsertIntoIRStreamFunc: the Go counterpart of Java's istream()
// built-in. The Java execution tests insert irstream with istream() in the
// projection, verifying that insert events have istream()=true and remove
// events have istream()=false.
//
// Approved difference: Java's SODA test
// ("select istream() from SupportBean") is not expressible in the Go
// chain API and is omitted.
func TestEPLInsertIntoIRStreamFuncParity(t *testing.T) {
	t.Run("lastevent-irstream", testEPLInsertIntoIRStreamFuncLastEvent)
	t.Run("join-irstream", testEPLInsertIntoIRStreamFuncJoin)
}

// testEPLInsertIntoIRStreamFuncLastEvent mirrors the Java
// EPLInsertIntoIRStreamFunc.istreamLastEvent test:
//
//	@public insert irstream into MyStream
//	  select irstream theString as c0, istream() as c1
//	  from SupportBean#lastevent
//
// Three events are sent. The consumer on MyStream receives:
//   - E1: 1 batch with {E1, true}
//   - E2: 2 batches: {E2, true} then {E1, false}
//   - E3: 2 batches: {E3, true} then {E2, false}
//
// Each insert or remove event is delivered as a separate batch; the
// istream() expression in the projection correctly distinguishes the two.
func testEPLInsertIntoIRStreamFuncLastEvent(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	registerInsertIntoTargetMap(env, "MyStream",
		FieldDef("c0", reflect.TypeOf("")),
		FieldDef("c1", reflect.TypeOf(false)),
	)

	producer, err := env.Build(
		Select(
			From[insertIntoSupportBean](env, "SupportBean").Window(LastEvent()),
			Alias("c0", Field[insertIntoSupportBean, string]("theString")),
			Alias("c1", IStream()),
		).InsertInto("MyStream", StatementName("s0"), WithOldStream()),
	)
	if err != nil {
		t.Fatal(err)
	}

	consumer, err := env.Build(FromAny(env, "MyStream").Query(StatementName("c0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployments := deployInsertIntoPlans(t, engine, []Plan{producer, consumer})
	_, batches := subscribeInsertIntoConsumer(t, deployments[1])

	// --- E1: insert, no remove yet ---
	if err := engine.SendEvent(context.Background(), insertIntoSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 1 {
		t.Fatalf("batches after E1 = %d, want 1", len(*batches))
	}
	assertBatchField(t, (*batches)[0], 0, "c0", "E1")
	assertBatchFieldBool(t, (*batches)[0], 0, "c1", true)

	// --- E2: insert E2, remove E1 (two batches) ---
	if err := engine.SendEvent(context.Background(), insertIntoSupportBean{TheString: "E2", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 3 {
		t.Fatalf("batches after E2 = %d, want 3", len(*batches))
	}
	// Batch 1: new insert
	assertBatchField(t, (*batches)[1], 0, "c0", "E2")
	assertBatchFieldBool(t, (*batches)[1], 0, "c1", true)
	// Batch 2: remove
	assertBatchField(t, (*batches)[2], 0, "c0", "E1")
	assertBatchFieldBool(t, (*batches)[2], 0, "c1", false)

	// --- E3: insert E3, remove E2 (two batches) ---
	if err := engine.SendEvent(context.Background(), insertIntoSupportBean{TheString: "E3", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 5 {
		t.Fatalf("batches after E3 = %d, want 5", len(*batches))
	}
	// Batch 3: new insert
	assertBatchField(t, (*batches)[3], 0, "c0", "E3")
	assertBatchFieldBool(t, (*batches)[3], 0, "c1", true)
	// Batch 4: remove
	assertBatchField(t, (*batches)[4], 0, "c0", "E2")
	assertBatchFieldBool(t, (*batches)[4], 0, "c1", false)
}

// testEPLInsertIntoIRStreamFuncJoin mirrors the Java
// EPLInsertIntoIRStreamFunc.istreamJoin test:
//
//	@public insert irstream into MyStream
//	  select irstream theString as c0, id as c1, istream() as c2
//	  from SupportBean#lastevent, SupportBean_S0#lastevent
//
// The join fires when both sides have a last event. The istream() flag
// correctly distinguishes insert from remove stream events in a join
// context.
func testEPLInsertIntoIRStreamFuncJoin(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertIntoSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[insertIntoSupportBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	registerInsertIntoTargetMap(env, "MyStream",
		FieldDef("c0", reflect.TypeOf("")),
		FieldDef("c1", reflect.TypeOf(0)),
		FieldDef("c2", reflect.TypeOf(false)),
	)

	producer, err := env.Build(
		Join(
			From[insertIntoSupportBean](env, "SupportBean").Window(LastEvent()),
			From[insertIntoSupportBeanS0](env, "SupportBean_S0").Window(LastEvent()),
		).Select(
			SelectFrom(0, "c0", JoinField[string](0, "theString")),
			SelectFrom(1, "c1", JoinField[int](1, "id")),
			SelectFrom(0, "c2", IStream()),
		).InsertInto("MyStream", StatementName("s0"), WithOldStream()),
	)
	if err != nil {
		t.Fatal(err)
	}

	consumer, err := env.Build(FromAny(env, "MyStream").Query(StatementName("c0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployments := deployInsertIntoPlans(t, engine, []Plan{producer, consumer})
	_, batches := subscribeInsertIntoConsumer(t, deployments[1])

	// --- Send SupportBean first: join not yet matched ---
	if err := engine.SendEvent(context.Background(), insertIntoSupportBean{TheString: "E1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 0 {
		t.Fatalf("batches after SB = %d, want 0", len(*batches))
	}

	// --- Send SupportBean_S0: join fires, insert row ---
	if err := engine.SendEvent(context.Background(), insertIntoSupportBeanS0{ID: 10}); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 1 {
		t.Fatalf("batches after S0 = %d, want 1", len(*batches))
	}
	assertBatchField(t, (*batches)[0], 0, "c0", "E1")
	assertBatchFieldInt(t, (*batches)[0], 0, "c1", 10)
	assertBatchFieldBool(t, (*batches)[0], 0, "c2", true)

	// --- Replace SupportBean: join removes old, inserts new (two batches) ---
	if err := engine.SendEvent(context.Background(), insertIntoSupportBean{TheString: "E2", IntPrimitive: 20}); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 3 {
		t.Fatalf("batches after SB2 = %d, want 3", len(*batches))
	}
	// Batch 1: new insert
	assertBatchField(t, (*batches)[1], 0, "c0", "E2")
	assertBatchFieldInt(t, (*batches)[1], 0, "c1", 10)
	assertBatchFieldBool(t, (*batches)[1], 0, "c2", true)
	// Batch 2: remove
	assertBatchField(t, (*batches)[2], 0, "c0", "E1")
	assertBatchFieldInt(t, (*batches)[2], 0, "c1", 10)
	assertBatchFieldBool(t, (*batches)[2], 0, "c2", false)
}

// assertBatchField checks the named field in the i-th result of a batch's New slice.
func assertBatchField(t *testing.T, batch ResultBatch, index int, name, want string) {
	t.Helper()
	if index >= len(batch.New) {
		t.Fatalf("batch.New index %d out of range (len=%d)", index, len(batch.New))
	}
	v, err := As[string](batch.New[index].Get(name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if v != want {
		t.Fatalf("%s = %q, want %q", name, v, want)
	}
}

func assertBatchFieldInt(t *testing.T, batch ResultBatch, index int, name string, want int) {
	t.Helper()
	if index >= len(batch.New) {
		t.Fatalf("batch.New index %d out of range (len=%d)", index, len(batch.New))
	}
	v, err := As[int](batch.New[index].Get(name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if v != want {
		t.Fatalf("%s = %d, want %d", name, v, want)
	}
}

func assertBatchFieldBool(t *testing.T, batch ResultBatch, index int, name string, want bool) {
	t.Helper()
	if index >= len(batch.New) {
		t.Fatalf("batch.New index %d out of range (len=%d)", index, len(batch.New))
	}
	v, err := As[bool](batch.New[index].Get(name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if v != want {
		t.Fatalf("%s = %v, want %v", name, v, want)
	}
}
