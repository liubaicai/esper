package esper

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"
)

// fcmEventWithManyArray mirrors SupportEventWithManyArray.
type fcmEventWithManyArray struct {
	ID        string    `esper:"id"`
	DoubleOne []float64 `esper:"doubleOne"`
	IntOne    []int     `esper:"intOne"`
	IntTwo    []int     `esper:"intTwo"`
	Value     int       `esper:"value"`
}

// fcmDoubleIntArrayRow mirrors SupportDoubleAndIntArray rows returned by
// SupportJoinResultIsArray.getArray.
type fcmDoubleIntArrayRow struct {
	ID          string    `esper:"id"`
	DoubleArray []float64 `esper:"doubleArray"`
	IntArray    []int     `esper:"intArray"`
	Value       int       `esper:"value"`
}

func newFCMMultikeyEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[fcmEventWithManyArray](env, "SupportEventWithManyArray"); err != nil {
		t.Fatal(err)
	}
	return env
}

// makeFCMGetArrayProvider mirrors SupportJoinResultIsArray.getArray: the
// constant two-row grid DA1/DA2.
func makeFCMGetArrayProvider(schema Schema) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		return newEventsFrom(schema, []fcmDoubleIntArrayRow{
			{ID: "DA1", DoubleArray: []float64{1, 2}, IntArray: []int{10, 20}, Value: 100},
			{ID: "DA2", DoubleArray: []float64{3, 4}, IntArray: []int{30, 40}, Value: 300},
		}, request.Now)
	})
}

func fcmDoubleSliceEqual(left, right []float64) bool { return slices.Equal(left, right) }

func fcmIntSliceEqual(left, right []int) bool { return slices.Equal(left, right) }

// fcmMultikeyStep asserts one send step of the Java runAssertion matrix: the
// exact last-new rows, or that the listener was not invoked at all when
// expected is nil (env.assertListenerNotInvoked).
func fcmMultikeyStep(t *testing.T, engine *Engine, listener *fcmOuterListener, fields []string, event any, expected [][]string, label string) {
	t.Helper()
	listener.lastNew = nil
	listener.invoked = false
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatalf("%s: send failed: %v", label, err)
	}
	if expected == nil {
		if listener.invoked {
			t.Fatalf("%s: listener invoked unexpectedly with %#v", label, listener.lastNew)
		}
		return
	}
	if !listener.invoked {
		t.Fatalf("%s: listener not invoked, want %#v", label, expected)
	}
	fcmOuterAssertRows(t, listener.lastNew, fields, expected, label+" lastNew")
}

// TestFromClauseMethodMultikeyJoinArrayParity mirrors
// EPLFromClauseMultikeyWArrayJoinArray: an array-valued equality between a
// method row field and the trigger event field.
func TestFromClauseMethodMultikeyJoinArrayParity(t *testing.T) {
	fields := []string{"eid", "sid"}
	env := newFCMMultikeyEnvironment(t)
	schema, err := StructSchema[fcmDoubleIntArrayRow]("FCMDoubleIntArrayRow")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(schema); err != nil {
		t.Fatal(err)
	}
	stream := From[fcmEventWithManyArray](env, "SupportEventWithManyArray").Window(KeepAll())
	method := FromMethod[fcmDoubleIntArrayRow](env, "s", schema, makeFCMGetArrayProvider(schema))
	query := Join(stream, method).
		Select(
			SelectLeft("eid", Field[fcmEventWithManyArray, string]("id")),
			SelectRight("sid", Field[fcmDoubleIntArrayRow, string]("id")),
		).
		Where(Func2("double-array-eq", fcmDoubleSliceEqual, JoinField[[]float64](1, "doubleArray"), JoinField[[]float64](0, "doubleOne"))).
		Query(StatementName("s0"))
	engine, _, listener := fcmOuterDeploy(t, env, query)

	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E1", DoubleOne: []float64{3, 4}, IntOne: []int{30, 40}, Value: 50}, [][]string{{"E1", "DA2"}}, "E1")
	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E2", DoubleOne: []float64{1, 2}, IntOne: []int{10, 20}, Value: 60}, [][]string{{"E2", "DA1"}}, "E2")
	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E3", DoubleOne: []float64{3, 4}, IntOne: []int{30, 40}, Value: 70}, [][]string{{"E3", "DA2"}}, "E3")
	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E4", DoubleOne: []float64{1}, IntOne: []int{30, 40}, Value: 80}, nil, "E4")
}

// TestFromClauseMethodMultikeyJoinTwoFieldParity mirrors
// EPLFromClauseMultikeyWArrayJoinTwoField: equality over two array fields.
func TestFromClauseMethodMultikeyJoinTwoFieldParity(t *testing.T) {
	fields := []string{"eid", "sid"}
	env := newFCMMultikeyEnvironment(t)
	schema, err := StructSchema[fcmDoubleIntArrayRow]("FCMDoubleIntArrayRow")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(schema); err != nil {
		t.Fatal(err)
	}
	stream := From[fcmEventWithManyArray](env, "SupportEventWithManyArray").Window(KeepAll())
	method := FromMethod[fcmDoubleIntArrayRow](env, "s", schema, makeFCMGetArrayProvider(schema))
	query := Join(stream, method).
		Select(
			SelectLeft("eid", Field[fcmEventWithManyArray, string]("id")),
			SelectRight("sid", Field[fcmDoubleIntArrayRow, string]("id")),
		).
		Where(And(
			Func2("double-array-eq", fcmDoubleSliceEqual, JoinField[[]float64](1, "doubleArray"), JoinField[[]float64](0, "doubleOne")),
			Func2("int-array-eq", fcmIntSliceEqual, JoinField[[]int](1, "intArray"), JoinField[[]int](0, "intOne")),
		)).
		Query(StatementName("s0"))
	engine, _, listener := fcmOuterDeploy(t, env, query)

	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E1", DoubleOne: []float64{3, 4}, IntOne: []int{30, 40}, Value: 50}, [][]string{{"E1", "DA2"}}, "E1")
	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E2", DoubleOne: []float64{1, 2}, IntOne: []int{10, 20}, Value: 60}, [][]string{{"E2", "DA1"}}, "E2")
	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E3", DoubleOne: []float64{3, 4}, IntOne: []int{30, 40}, Value: 70}, [][]string{{"E3", "DA2"}}, "E3")
	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E4", DoubleOne: []float64{1}, IntOne: []int{30, 40}, Value: 80}, nil, "E4")
	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E3", DoubleOne: []float64{3, 4}, IntOne: []int{30, 41}, Value: 0}, nil, "E3-int-mismatch")
}

// TestFromClauseMethodMultikeyJoinCompositeParity mirrors
// EPLFromClauseMultikeyWArrayJoinComposite: two array equalities plus a
// scalar range comparison.
func TestFromClauseMethodMultikeyJoinCompositeParity(t *testing.T) {
	fields := []string{"eid", "sid"}
	env := newFCMMultikeyEnvironment(t)
	schema, err := StructSchema[fcmDoubleIntArrayRow]("FCMDoubleIntArrayRow")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(schema); err != nil {
		t.Fatal(err)
	}
	stream := From[fcmEventWithManyArray](env, "SupportEventWithManyArray").Window(KeepAll())
	method := FromMethod[fcmDoubleIntArrayRow](env, "s", schema, makeFCMGetArrayProvider(schema))
	query := Join(stream, method).
		Select(
			SelectLeft("eid", Field[fcmEventWithManyArray, string]("id")),
			SelectRight("sid", Field[fcmDoubleIntArrayRow, string]("id")),
		).
		Where(And(
			Func2("double-array-eq", fcmDoubleSliceEqual, JoinField[[]float64](1, "doubleArray"), JoinField[[]float64](0, "doubleOne")),
			And(
				Func2("int-array-eq", fcmIntSliceEqual, JoinField[[]int](1, "intArray"), JoinField[[]int](0, "intOne")),
				Greater[int](JoinField[int](1, "value"), JoinField[int](0, "value")),
			),
		)).
		Query(StatementName("s0"))
	engine, _, listener := fcmOuterDeploy(t, env, query)

	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E1", DoubleOne: []float64{3, 4}, IntOne: []int{30, 40}, Value: 50}, [][]string{{"E1", "DA2"}}, "E1")
	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E2", DoubleOne: []float64{1, 2}, IntOne: []int{10, 20}, Value: 60}, [][]string{{"E2", "DA1"}}, "E2")
	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E3", DoubleOne: []float64{3, 4}, IntOne: []int{30, 40}, Value: 70}, [][]string{{"E3", "DA2"}}, "E3")
	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E4", DoubleOne: []float64{1}, IntOne: []int{30, 40}, Value: 80}, nil, "E4")
	fcmMultikeyStep(t, engine, listener, fields, fcmEventWithManyArray{ID: "E3", DoubleOne: []float64{3, 4}, IntOne: []int{30, 40}, Value: 1000}, nil, "E3-value-too-large")
}

// makeFCMUUIDResultProvider mirrors SupportJoinResultIsArray.getResultIntArray
// and getResultTwoField: every underlying invocation returns one row holding a
// freshly generated id. Deterministic increasing ids stand in for Java UUIDs:
// the assertions only compare equality across cache hits.
func makeFCMUUIDResultProvider(schema Schema, sequence *int) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		*sequence++
		return newEventsFrom(schema, []map[string]any{{"uuid": fmt.Sprintf("uuid-%d", *sequence)}}, request.Now)
	})
}

func fcmMultikeyCachedUUIDProvider(t *testing.T, schema Schema, sequence *int, key MethodCacheKey) MethodProvider {
	t.Helper()
	cached, err := NewCachedMethodProvider(
		makeFCMUUIDResultProvider(schema, sequence),
		key,
		// Mirrors the expiry-time cache (max age 1s, purge 10s) registered
		// for SupportJoinResultIsArray in TestSuiteEPLFromClauseMethodWConfig.
		MethodCacheConfig{MaxAge: time.Second, PurgeInterval: 10 * time.Second},
	)
	if err != nil {
		t.Fatal(err)
	}
	return cached
}

// TestFromClauseMethodMultikeyParameterizedByArrayParity mirrors
// EPLFromClauseMultikeyWArrayParameterizedByArray: the invocation cache keys
// on the int[] argument contents, so repeated arrays observe the same
// generated row.
func TestFromClauseMethodMultikeyParameterizedByArrayParity(t *testing.T) {
	env := newFCMMultikeyEnvironment(t)
	schema := fcmMapSchema(t, env, "FCMUUIDRow", FieldDef("uuid", reflect.TypeOf("")))
	sequence := 0
	provider := fcmMultikeyCachedUUIDProvider(t, schema, &sequence, func(request MethodRequest) []any {
		return []any{request.Trigger.Get("intOne").Any()}
	})
	stream := From[fcmEventWithManyArray](env, "SupportEventWithManyArray").Window(KeepAll())
	method := FromMethod[map[string]any](env, "s", schema, provider)
	query := Join(stream, method).
		Select(SelectRight("uuid", Field[map[string]any, string]("uuid"))).
		Query(StatementName("s0"))
	engine, _, listener := fcmOuterDeploy(t, env, query)

	send := func(id string, ints []int) string {
		t.Helper()
		listener.lastNew = nil
		listener.invoked = false
		if err := engine.SendEvent(context.Background(), fcmEventWithManyArray{ID: id, IntOne: ints}); err != nil {
			t.Fatal(err)
		}
		if !listener.invoked || len(listener.lastNew) != 1 {
			t.Fatalf("%s: expected exactly one new row, got %#v", id, listener.lastNew)
		}
		uuid, _ := listener.lastNew[0].Get("uuid").Any().(string)
		if uuid == "" {
			t.Fatalf("%s: missing uuid in %#v", id, listener.lastNew)
		}
		return uuid
	}

	sb12 := send("E1", []int{1, 2})
	if got := send("E2", []int{1, 2}); got != sb12 {
		t.Fatalf("E2 uuid = %s, want cached %s", got, sb12)
	}
	sb3 := send("E3", []int{3})
	if got := send("E4", []int{3}); got != sb3 {
		t.Fatalf("E4 uuid = %s, want cached %s", got, sb3)
	}
	if got := send("E5", []int{1, 2}); got != sb12 {
		t.Fatalf("E5 uuid = %s, want cached %s", got, sb12)
	}
}

// TestFromClauseMethodMultikeyParameterizedByTwoFieldParity mirrors
// EPLFromClauseMultikeyWArrayParameterizedByTwoField: the invocation cache
// keys on the (id, int[]) pair, including empty and null arrays.
func TestFromClauseMethodMultikeyParameterizedByTwoFieldParity(t *testing.T) {
	env := newFCMMultikeyEnvironment(t)
	schema := fcmMapSchema(t, env, "FCMUUIDRow", FieldDef("uuid", reflect.TypeOf("")))
	sequence := 0
	provider := fcmMultikeyCachedUUIDProvider(t, schema, &sequence, func(request MethodRequest) []any {
		return []any{request.Trigger.Get("id").Any(), request.Trigger.Get("intOne").Any()}
	})
	stream := From[fcmEventWithManyArray](env, "SupportEventWithManyArray").Window(KeepAll())
	method := FromMethod[map[string]any](env, "s", schema, provider)
	query := Join(stream, method).
		Select(SelectRight("uuid", Field[map[string]any, string]("uuid"))).
		Query(StatementName("s0"))
	engine, _, listener := fcmOuterDeploy(t, env, query)

	send := func(id string, ints []int) string {
		t.Helper()
		listener.lastNew = nil
		listener.invoked = false
		if err := engine.SendEvent(context.Background(), fcmEventWithManyArray{ID: id, IntOne: ints}); err != nil {
			t.Fatal(err)
		}
		if !listener.invoked || len(listener.lastNew) != 1 {
			t.Fatalf("%s: expected exactly one new row, got %#v", id, listener.lastNew)
		}
		uuid, _ := listener.lastNew[0].Get("uuid").Any().(string)
		if uuid == "" {
			t.Fatalf("%s: missing uuid in %#v", id, listener.lastNew)
		}
		return uuid
	}

	sb1 := send("MA1", []int{1, 2})
	sb2 := send("MA2", []int{1})
	sb3 := send("MA3", []int{})
	sb4 := send("MA4", nil)

	if got := send("MA3", []int{}); got != sb3 {
		t.Fatalf("MA3 repeat uuid = %s, want cached %s", got, sb3)
	}
	if got := send("MA1", []int{1, 2}); got != sb1 {
		t.Fatalf("MA1 repeat uuid = %s, want cached %s", got, sb1)
	}
	if got := send("MA4", nil); got != sb4 {
		t.Fatalf("MA4 repeat uuid = %s, want cached %s", got, sb4)
	}
	if got := send("MA2", []int{1}); got != sb2 {
		t.Fatalf("MA2 repeat uuid = %s, want cached %s", got, sb2)
	}
	if got := send("MA1", []int{1, 3}); got == sb1 {
		t.Fatalf("MA1[1,3] uuid = %s, want a fresh id distinct from %s", got, sb1)
	}
}
