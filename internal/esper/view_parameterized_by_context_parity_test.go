package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type zzCtxInit struct {
	ID      string `esper:"id"`
	IntSize int32  `esper:"intSize"`
}
type zzBean struct {
	TheString    *string `esper:"theString"`
	IntPrimitive int32   `esper:"intPrimitive"`
}

// TestViewParameterizedByContextLengthWindowParity pins the Java execution
// `initiated by SupportContextInitEventWLength as miewl terminated after 1 year`
// with statement
// `select context.miewl.id as id, count(*) as cnt
//
//	from SupportBean(theString=context.miewl.id)#length(context.miewl.intSize)`.
//
// Every initiating event allocates a NEW overlapping partition (P1 size 2,
// P2 size 4, P3 size 3 coexist). Each value event joins only the partition
// whose initiating id matches theString, and each partition's count is capped
// by its own parameterized length window.
//
// Construction notes (the probe originally failed with a lone {P2:0} row):
// the statement MUST be built through Stream.Aggregate(...) so the query is a
// real aggregate statement. The top-level Select(stream, Alias("cnt",
// CountAll())) combinator creates an unaggregated RecordStream whose
// count(*) evaluates per event over an empty scope (always 0), which is what
// produced the misleading single zero-count row.
// TestViewParameterizedByContextLengthWindowParity covers the
// ViewParameterizedByContext length-window executions' engine core: a
// context-parameterized #length(context.<initiator>.<prop>) window under an
// overlapping initiated context — per-partition sizes P1=2, P2=4, P3=3 cap
// the count(*) aggregate per partition, matching Java's iterator vector.
func TestViewParameterizedByContextLengthWindowParity(t *testing.T) {
	env := NewEnvironment()
	RegisterStruct[zzCtxInit](env, "SupportContextInitEventWLength")
	RegisterStruct[zzBean](env, "SupportBean")
	isInit := Equal[string](TypeName(EventValue[Event]()), Literal("SupportContextInitEventWLength"))
	endTimer := TimerIntervalCalendar(From[zzCtxInit](env, "SupportContextInitEventWLength"), 1, 0, 0)
	if _, err := CreateOverlappingPatternTerminatedContext(env, "CtxInitToTerm", Literal("global"), isInit, endTimer); err != nil {
		t.Fatal(err)
	}
	initID := func() Expr { return Property[*string](ContextInitiatingEvent(), "id") }
	plan, err := env.Build(
		From[zzBean](env, "SupportBean").
			Filter(EqualOf(Field[zzBean, *string]("theString"), initID())).
			Window(LengthWindowExpr(Property[int](ContextInitiatingEvent(), "intSize"))).
			Aggregate(
				Alias("id", initID()),
				Alias("cnt", CountAll()),
			).
			Query(StatementName("s0"), WithContext("CtxInitToTerm")),
	)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	defer engine.Close(context.Background())
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	st := dep.Statements()[0]
	send := func(id string, ip int32) {
		e := zzBean{TheString: &id, IntPrimitive: ip}
		engine.Send(context.Background(), "SupportBean", e)
	}
	sendInit := func(id string, size int32) {
		engine.Send(context.Background(), "SupportContextInitEventWLength", zzCtxInit{ID: id, IntSize: size})
	}
	assertRows := func(want map[string]int64) {
		t.Helper()
		snap, err := st.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]int64{}
		for _, row := range snap.Results() {
			id := *(row.Get("id").Any().(*string))
			got[id] = row.Get("cnt").Any().(int64)
		}
		if len(got) != len(want) {
			t.Fatalf("rows=%v want %v", got, want)
		}
		for id, c := range want {
			if got[id] != c {
				t.Fatalf("rows=%v want %v", got, want)
			}
		}
	}
	sendInit("P1", 2)
	sendInit("P2", 4)
	sendInit("P3", 3)
	send("P2", -1)
	assertRows(map[string]int64{"P1": 0, "P2": 1, "P3": 0})
	for i := 0; i < 10; i++ {
		send("P1", -1)
		send("P2", -1)
		send("P3", -1)
	}
	// P1 received 10 events into a length(2) window, P2 11 into length(4),
	// P3 10 into length(3): counts cap at each partition's parameterized size.
	assertRows(map[string]int64{"P1": 2, "P2": 4, "P3": 3})
	// One more round stays capped by the same per-partition sizes.
	send("P1", -1)
	send("P2", -1)
	send("P3", -1)
	assertRows(map[string]int64{"P1": 2, "P2": 4, "P3": 3})
}

// TestContextParameterizedExprWindowExpiry guards the expire paths of the
// context-parameterized window variants: time_accum and firsttime must use
// the partition-resolved period (not a zero default) on the time-advance
// path, and the static TimeToLive expiry must be unaffected by the
// resolution plumbing.
func TestContextParameterizedExprWindowExpiry(t *testing.T) {
	env := NewEnvironment()
	RegisterStruct[zzCtxInit](env, "SupportContextInitEventWLength")
	RegisterStruct[zzBean](env, "SupportBean")
	isInit := Equal[string](TypeName(EventValue[Event]()), Literal("SupportContextInitEventWLength"))
	endTimer := TimerIntervalCalendar(From[zzCtxInit](env, "SupportContextInitEventWLength"), 1, 0, 0)
	if _, err := CreateOverlappingPatternTerminatedContext(env, "Ctx", Literal("global"), isInit, endTimer); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	build := func(window WindowSpec, withContext bool) *Statement {
		t.Helper()
		source := From[zzBean](env, "SupportBean")
		if withContext {
			source = source.Filter(EqualOf(Field[zzBean, *string]("theString"), Property[*string](ContextInitiatingEvent(), "id")))
		}
		options := []QueryOption{StatementName("s0"), WithOldStream()}
		if withContext {
			options = append(options, WithContext("Ctx"))
		}
		plan, err := env.Build(source.Window(window).Query(options...))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		return deployment.Statements()[0]
	}
	snapshotIDs := func(st *Statement) []string {
		t.Helper()
		snap, err := st.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, row := range snap.Results() {
			if id := row.Get("theString").Any(); id != nil {
				ids = append(ids, *(id.(*string)))
			}
		}
		return ids
	}
	advance := func(at int64) {
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(at).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	sendCtx := func(id string, size int32) {
		if err := engine.Send(context.Background(), "SupportContextInitEventWLength", zzCtxInit{ID: id, IntSize: size}); err != nil {
			t.Fatal(err)
		}
	}
	sendBean := func(id string) {
		if err := engine.Send(context.Background(), "SupportBean", zzBean{TheString: &id}); err != nil {
			t.Fatal(err)
		}
	}

	// time_accum(<ctx>.intSize = 5): the value event lands at t=0 and must
	// survive the 2 ms tick, expiring only once the resolved 5 ms period
	// passes (at t=5).
	accum := build(TimeAccumExpr(Property[int](ContextInitiatingEvent(), "intSize")), true)
	sendCtx("A", 5)
	sendBean("A")
	advance(2)
	if got := snapshotIDs(accum); !reflect.DeepEqual(got, []string{"A"}) {
		t.Fatalf("time-accum before expiry = %v", got)
	}
	advance(5)
	if got := snapshotIDs(accum); len(got) != 0 {
		t.Fatalf("time-accum after expiry = %v", got)
	}

	// firsttime(<ctx>.intSize = 10): the partition starts at the current
	// clock (5 ms), admits everything before start+10 ms (= 15) and closes
	// exactly there.
	first := build(FirstTimeExpr(Property[int](ContextInitiatingEvent(), "intSize")), true)
	sendCtx("B", 10)
	sendBean("B")
	advance(14)
	sendBean("B")
	advance(15)
	sendBean("B")
	if got := snapshotIDs(first); !reflect.DeepEqual(got, []string{"B", "B"}) {
		t.Fatalf("firsttime rows after close = %v", got)
	}

	// Static TTL regression: expiry at receivedAt+duration still fires.
	ttl := build(TimeToLive(5*time.Millisecond), false)
	advance(2000)
	sendBean("T1")
	advance(2003)
	if got := snapshotIDs(ttl); !reflect.DeepEqual(got, []string{"T1"}) {
		t.Fatalf("ttl before expiry = %v", got)
	}
	advance(2005)
	if got := snapshotIDs(ttl); len(got) != 0 {
		t.Fatalf("ttl after expiry = %v", got)
	}
}
