package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// makeFCMFetchObjectLogProvider mirrors
// SupportStaticMethodInvocations.fetchObjectLog(fetchId, passThroughNumber):
// every underlying invocation is counted and returns one row with
// id = passThroughNumber and p00 = "|" + fetchId + "|".
func makeFCMFetchObjectLogProvider(schema Schema, calls *int) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		*calls++
		fetchID, _ := request.Trigger.Get("theString").Any().(string)
		passThrough, _ := request.Trigger.Get("intPrimitive").Any().(int)
		return newEventsFrom(schema, []map[string]any{{"id": passThrough, "p00": "|" + fetchID + "|"}}, request.Now)
	})
}

// fcmFetchObjectLogCacheKey mirrors the Java method-reference cache key: the
// invocation argument array (theString, intPrimitive) read from the trigger.
func fcmFetchObjectLogCacheKey(request MethodRequest) []any {
	return []any{request.Trigger.Get("theString").Any(), request.Trigger.Get("intPrimitive").Any()}
}

func fcmCacheQuery(t *testing.T, env *Environment, schema Schema, provider MethodProvider, name string) Query {
	t.Helper()
	stream := From[fcmSupportBean](env, "SupportBean").Window(LengthWindow(100))
	method := FromMethod[map[string]any](env, "method", schema, provider)
	return Join(stream, method).Select(
		SelectRight("id", Field[map[string]any, int]("id")),
		SelectRight("p00", Field[map[string]any, string]("p00")),
		SelectLeft("theString", Field[fcmSupportBean, string]("theString")),
	).Query(StatementName(name))
}

func fcmCacheSend(t *testing.T, engine *Engine, listener *fcmOuterListener, calls *int, fields []string, theString string, intPrimitive int, expected [][]string, wantCalls int, label string) {
	t.Helper()
	listener.lastNew = nil
	listener.invoked = false
	if err := engine.SendEvent(context.Background(), fcmSupportBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
		t.Fatalf("%s: send failed: %v", label, err)
	}
	if !listener.invoked {
		t.Fatalf("%s: listener not invoked, want %#v", label, expected)
	}
	fcmOuterAssertRows(t, listener.lastNew, fields, expected, label+" lastNew")
	got := *calls
	*calls = 0
	if got != wantCalls {
		t.Fatalf("%s: underlying invocations = %d, want %d", label, got, wantCalls)
	}
}

// TestFromClauseMethodCacheLRUParity mirrors EPLFromClauseMethodCacheLRU run
// with an LRU(3) method-reference cache: identical invocation arguments hit
// the cache, a hit refreshes the entry, and the least recently used entry is
// evicted once the cache is full.
func TestFromClauseMethodCacheLRUParity(t *testing.T) {
	env := newFCMEnvironment(t)
	schema := fcmMapSchema(t, env, "FCMCacheLRURow",
		FieldDef("id", reflect.TypeOf(0)),
		FieldDef("p00", reflect.TypeOf("")))
	calls := 0
	cached, err := NewCachedMethodProvider(
		makeFCMFetchObjectLogProvider(schema, &calls),
		fcmFetchObjectLogCacheKey,
		MethodCacheConfig{LRUSize: 3},
	)
	if err != nil {
		t.Fatal(err)
	}
	engine, _, listener := fcmOuterDeploy(t, env, fcmCacheQuery(t, env, schema, cached, "s0"))
	fields := []string{"id", "p00", "theString"}

	fcmCacheSend(t, engine, listener, &calls, fields, "E1", 1, [][]string{{"1", "|E1|", "E1"}}, 1, "E1")
	fcmCacheSend(t, engine, listener, &calls, fields, "E2", 2, [][]string{{"2", "|E2|", "E2"}}, 1, "E2")
	fcmCacheSend(t, engine, listener, &calls, fields, "E3", 3, [][]string{{"3", "|E3|", "E3"}}, 1, "E3")
	fcmCacheSend(t, engine, listener, &calls, fields, "E3", 3, [][]string{{"3", "|E3|", "E3"}}, 0, "E3 cached")
	fcmCacheSend(t, engine, listener, &calls, fields, "E4", 4, [][]string{{"4", "|E4|", "E4"}}, 1, "E4 evicts E1")
	fcmCacheSend(t, engine, listener, &calls, fields, "E2", 2, [][]string{{"2", "|E2|", "E2"}}, 0, "E2 cached")
	fcmCacheSend(t, engine, listener, &calls, fields, "E1", 1, [][]string{{"1", "|E1|", "E1"}}, 1, "E1 evicted")
}

// TestFromClauseMethodCacheExpiryParity mirrors EPLFromClauseMethodCacheExpiry
// run with an expiry-time method-reference cache of max age 1 second: entries
// younger than the maximum age hit, older entries miss and are re-polled.
func TestFromClauseMethodCacheExpiryParity(t *testing.T) {
	env := newFCMEnvironment(t)
	schema := fcmMapSchema(t, env, "FCMCacheExpiryRow",
		FieldDef("id", reflect.TypeOf(0)),
		FieldDef("p00", reflect.TypeOf("")))
	calls := 0
	cached, err := NewCachedMethodProvider(
		makeFCMFetchObjectLogProvider(schema, &calls),
		fcmFetchObjectLogCacheKey,
		MethodCacheConfig{MaxAge: time.Second, PurgeInterval: 10 * time.Second},
	)
	if err != nil {
		t.Fatal(err)
	}
	engine, _, listener := fcmOuterDeploy(t, env, fcmCacheQuery(t, env, schema, cached, "s0"))
	fields := []string{"id", "p00", "theString"}
	epoch := time.Unix(0, 0).UTC()
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), epoch.Add(time.Duration(ms)*time.Millisecond)); err != nil {
			t.Fatalf("advance to %dms failed: %v", ms, err)
		}
	}

	advance(1000)
	fcmCacheSend(t, engine, listener, &calls, fields, "E1", 1, [][]string{{"1", "|E1|", "E1"}}, 1, "E1@1000")
	advance(1500)
	fcmCacheSend(t, engine, listener, &calls, fields, "E2", 2, [][]string{{"2", "|E2|", "E2"}}, 1, "E2@1500")
	advance(2000)
	fcmCacheSend(t, engine, listener, &calls, fields, "E3", 3, [][]string{{"3", "|E3|", "E3"}}, 1, "E3@2000")
	fcmCacheSend(t, engine, listener, &calls, fields, "E3", 3, [][]string{{"3", "|E3|", "E3"}}, 0, "E3 cached@2000")
	advance(2100)
	fcmCacheSend(t, engine, listener, &calls, fields, "E4", 4, [][]string{{"4", "|E4|", "E4"}}, 1, "E4@2100")
	fcmCacheSend(t, engine, listener, &calls, fields, "E2", 2, [][]string{{"2", "|E2|", "E2"}}, 0, "E2 cached@2100 (age 600ms)")
	fcmCacheSend(t, engine, listener, &calls, fields, "E1", 1, [][]string{{"1", "|E1|", "E1"}}, 1, "E1 expired@2100 (age 1100ms)")
}
