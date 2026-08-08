package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

// fcmSupportBeanBoxed mirrors Java's SupportBean for the historical-join
// suites that send intPrimitive/intBoxed through an on-set variable trigger.
type fcmSupportBeanBoxed struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     int    `esper:"intBoxed"`
}

func newFCMBoxedEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[fcmSupportBeanBoxed](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func fcmMapSchema(t *testing.T, env *Environment, name string, fields ...FieldSpec) Schema {
	t.Helper()
	schema, err := NewMapSchema(name, fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(schema); err != nil {
		t.Fatal(err)
	}
	return schema
}

// fcmVariableInt reads one nullable int variable, mirroring Java's Integer
// variable boxing: a missing or null value reports ok == false.
func fcmVariableInt(request MethodRequest, name string) (int, bool) {
	value, ok := request.Variables[name]
	if !ok || value.Any() == nil {
		return 0, false
	}
	number, ok := value.Any().(int)
	return number, ok
}

// makeFCMFetchResult12Provider mirrors SupportStaticMethodLib.fetchResult12:
// constant argument, rows value = 1, 2.
func makeFCMFetchResult12Provider(schema Schema) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		return newEventsFrom(schema, []map[string]any{{"value": 1}, {"value": 2}}, request.Now)
	})
}

// makeFCMFetchResult23Provider mirrors SupportStaticMethodLib.fetchResult23: a
// null argument yields no rows, otherwise value = 2, 3. When depSource is set
// the argument is the dependency event's value field (subordinate variant).
func makeFCMFetchResult23Provider(schema Schema, depSource string) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		if depSource != "" {
			dep, ok := request.Dependency(depSource)
			if !ok {
				return nil, fmt.Errorf("missing dependency %s", depSource)
			}
			if dep.Get("value").Any() == nil {
				return nil, nil
			}
		}
		return newEventsFrom(schema, []map[string]any{{"value": 2}, {"value": 3}}, request.Now)
	})
}

// makeFCMFetchResult100Provider mirrors SupportStaticMethodLib.fetchResult100:
// the 100-row grid col1 = 0..9, col2 = 0..9.
func makeFCMFetchResult100Provider(schema Schema) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		rows := make([]map[string]any, 0, 100)
		for i := 0; i < 10; i++ {
			for j := 0; j < 10; j++ {
				rows = append(rows, map[string]any{"col1": i, "col2": j})
			}
		}
		return newEventsFrom(schema, rows, request.Now)
	})
}

// makeFCMFetchBetweenProvider mirrors SupportStaticMethodLib.fetchBetween: a
// null bound or upper < lower yields no rows, otherwise value = lower..upper.
func makeFCMFetchBetweenProvider(schema Schema, lowerVar, upperVar string) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		lower, ok := fcmVariableInt(request, lowerVar)
		if !ok {
			return nil, nil
		}
		upper, ok := fcmVariableInt(request, upperVar)
		if !ok || upper < lower {
			return nil, nil
		}
		rows := make([]map[string]any, 0, upper-lower+1)
		for i := lower; i <= upper; i++ {
			rows = append(rows, map[string]any{"value": i})
		}
		return newEventsFrom(schema, rows, request.Now)
	})
}

// makeFCMFetchBetweenStringProvider mirrors fetchBetweenString: same range as
// fetchBetween with the value rendered as a string.
func makeFCMFetchBetweenStringProvider(schema Schema, lowerVar, upperVar string) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		lower, ok := fcmVariableInt(request, lowerVar)
		if !ok {
			return nil, nil
		}
		upper, ok := fcmVariableInt(request, upperVar)
		if !ok || upper < lower {
			return nil, nil
		}
		rows := make([]map[string]any, 0, upper-lower+1)
		for i := lower; i <= upper; i++ {
			rows = append(rows, map[string]any{"value": fmt.Sprintf("%d", i)})
		}
		return newEventsFrom(schema, rows, request.Now)
	})
}

// makeFCMFetchIdDelimitedProvider mirrors SupportStaticMethodLib.
// fetchIdDelimited: one row result = "|" + dependency value + "|".
func makeFCMFetchIdDelimitedProvider(schema Schema, depSource string) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		dep, ok := request.Dependency(depSource)
		if !ok {
			return nil, fmt.Errorf("missing dependency %s", depSource)
		}
		value := dep.Get("value").Any()
		if value == nil {
			return nil, nil
		}
		return newEventsFrom(schema, []map[string]any{{"result": fmt.Sprintf("|%v|", value)}}, request.Now)
	})
}

// TestFromClauseMethod2JoinHistoricalIndependentOuterParity mirrors
// EPLFromClauseMethod2JoinHistoricalIndependentOuter: two constant-argument
// method sources joined left/right/full outer; the iterator observes the
// seeded join state exactly as Java's start-time poll.
func TestFromClauseMethod2JoinHistoricalIndependentOuterParity(t *testing.T) {
	fields := []string{"valueOne", "valueTwo"}
	leftRight := [][]string{{"1", ""}, {"2", "2"}}
	full := [][]string{{"1", ""}, {"2", "2"}, {"", "3"}}

	build := func(t *testing.T, variant string) (*Environment, Query) {
		env := NewEnvironment()
		schema := fcmMapSchema(t, env, "FCMValueRow", FieldDef("value", reflect.TypeOf(0)))
		s0 := FromMethod[map[string]any](env, "s0", schema, makeFCMFetchResult12Provider(schema)).EvaluateOnce()
		s1 := FromMethod[map[string]any](env, "s1", schema, makeFCMFetchResult23Provider(schema, "")).EvaluateOnce()
		var chain ChainedJoinStream
		switch variant {
		case "left":
			chain = JoinChain(JoinSource(s0)).LeftOuterJoin(JoinSource(s1),
				OnSourcesEqual(0, Field[map[string]any, int]("value"), 1, Field[map[string]any, int]("value")))
		case "right":
			chain = JoinChain(JoinSource(s1)).RightOuterJoin(JoinSource(s0),
				OnSourcesEqual(0, Field[map[string]any, int]("value"), 1, Field[map[string]any, int]("value")))
		case "full":
			chain = JoinChain(JoinSource(s1)).FullOuterJoin(JoinSource(s0),
				OnSourcesEqual(0, Field[map[string]any, int]("value"), 1, Field[map[string]any, int]("value")))
		default: // full-reversed
			chain = JoinChain(JoinSource(s0)).FullOuterJoin(JoinSource(s1),
				OnSourcesEqual(0, Field[map[string]any, int]("value"), 1, Field[map[string]any, int]("value")))
		}
		// valueOne is always s0.value and valueTwo s1.value; the chain index of
		// each source depends on the textual order of the variant.
		s0Index, s1Index := 0, 1
		if variant == "right" || variant == "full" {
			s0Index, s1Index = 1, 0
		}
		query := chain.Select(
			SelectFrom(s0Index, "valueOne", Field[map[string]any, int]("value")),
			SelectFrom(s1Index, "valueTwo", Field[map[string]any, int]("value")),
		).Query(StatementName("fcm-2hist-ind-outer-" + variant))
		return env, query
	}

	for _, tc := range []struct {
		variant  string
		expected [][]string
	}{
		{"left", leftRight},
		{"right", leftRight},
		{"full", full},
		{"full-reversed", full},
	} {
		t.Run(tc.variant, func(t *testing.T) {
			env, query := build(t, tc.variant)
			engine, stmt, listener := fcmOuterDeploy(t, env, query)
			_ = engine
			if listener.invoked {
				t.Fatalf("listener invoked unexpectedly with %#v", listener.lastNew)
			}
			fcmOuterAssertRows(t, fcmOuterSnapshot(t, stmt), fields, tc.expected, "iterator")
		})
	}
}

// TestFromClauseMethod2JoinHistoricalSubordinateOuterParity mirrors
// EPLFromClauseMethod2JoinHistoricalSubordinateOuter: s1 is bound to s0
// (fetchResult23(s0.value)); dependency-bound rows never anchor placeholders,
// so all four outer variants iterate {1,null},{2,2}.
func TestFromClauseMethod2JoinHistoricalSubordinateOuterParity(t *testing.T) {
	fields := []string{"valueOne", "valueTwo"}
	expected := [][]string{{"1", ""}, {"2", "2"}}

	build := func(t *testing.T, variant string) (*Environment, Query) {
		env := NewEnvironment()
		schema := fcmMapSchema(t, env, "FCMValueRow", FieldDef("value", reflect.TypeOf(0)))
		s0 := FromMethod[map[string]any](env, "s0", schema, makeFCMFetchResult12Provider(schema)).EvaluateOnce()
		s1 := FromMethod[map[string]any](env, "s1", schema, makeFCMFetchResult23Provider(schema, "s0")).DependingOn("s0")
		var chain ChainedJoinStream
		switch variant {
		case "left":
			chain = JoinChain(JoinSource(s0)).LeftOuterJoin(JoinSource(s1),
				OnSourcesEqual(0, Field[map[string]any, int]("value"), 1, Field[map[string]any, int]("value")))
		case "right":
			chain = JoinChain(JoinSource(s1)).RightOuterJoin(JoinSource(s0),
				OnSourcesEqual(0, Field[map[string]any, int]("value"), 1, Field[map[string]any, int]("value")))
		case "full":
			chain = JoinChain(JoinSource(s1)).FullOuterJoin(JoinSource(s0),
				OnSourcesEqual(0, Field[map[string]any, int]("value"), 1, Field[map[string]any, int]("value")))
		default: // full-reversed
			chain = JoinChain(JoinSource(s0)).FullOuterJoin(JoinSource(s1),
				OnSourcesEqual(0, Field[map[string]any, int]("value"), 1, Field[map[string]any, int]("value")))
		}
		s0Index, s1Index := 0, 1
		if variant == "right" || variant == "full" {
			s0Index, s1Index = 1, 0
		}
		query := chain.Select(
			SelectFrom(s0Index, "valueOne", Field[map[string]any, int]("value")),
			SelectFrom(s1Index, "valueTwo", Field[map[string]any, int]("value")),
		).Query(StatementName("fcm-2hist-sub-outer-" + variant))
		return env, query
	}

	for _, variant := range []string{"left", "right", "full", "full-reversed"} {
		t.Run(variant, func(t *testing.T) {
			env, query := build(t, variant)
			_, stmt, listener := fcmOuterDeploy(t, env, query)
			if listener.invoked {
				t.Fatalf("listener invoked unexpectedly with %#v", listener.lastNew)
			}
			fcmOuterAssertRows(t, fcmOuterSnapshot(t, stmt), fields, expected, "iterator")
		})
	}
}

// TestFromClauseMethod2JoinHistoricalSubordinateOuterMultiFieldParity mirrors
// EPLFromClauseMethod2JoinHistoricalSubordinateOuterMultiField: a keep-all
// event stream left-outer-joined to the constant 100-row method grid on a
// two-field equi-condition.
func TestFromClauseMethod2JoinHistoricalSubordinateOuterMultiFieldParity(t *testing.T) {
	env := newFCMBoxedEnvironment(t)
	schema := fcmMapSchema(t, env, "FCMResult100Row",
		FieldDef("col1", reflect.TypeOf(0)),
		FieldDef("col2", reflect.TypeOf(0)))
	stream := From[fcmSupportBeanBoxed](env, "SupportBean").Window(KeepAll())
	method := FromMethod[map[string]any](env, "method100", schema, makeFCMFetchResult100Provider(schema)).EvaluateOnce()
	query := Join(stream, method,
		AllJoin(
			OnEqual(Field[fcmSupportBeanBoxed, int]("intPrimitive"), Field[map[string]any, int]("col1")),
			OnEqual(Field[fcmSupportBeanBoxed, int]("intBoxed"), Field[map[string]any, int]("col2")),
		),
	).LeftOuter().Select(
		SelectLeft("intPrimitive", Field[fcmSupportBeanBoxed, int]("intPrimitive")),
		SelectLeft("intBoxed", Field[fcmSupportBeanBoxed, int]("intBoxed")),
		SelectRight("col1", Field[map[string]any, int]("col1")),
		SelectRight("col2", Field[map[string]any, int]("col2")),
	).Query(StatementName("fcm-2hist-sub-outer-multifield"))

	fields := []string{"intPrimitive", "intBoxed", "col1", "col2"}
	engine, stmt, listener := fcmOuterDeploy(t, env, query)

	fcmOuterAssertRows(t, fcmOuterSnapshot(t, stmt), fields, nil, "deploy iterator")

	row := [][]string{{"2", "4", "2", "4"}}
	fcmOuterStep(t, engine, stmt, listener, fields,
		fcmSupportBeanBoxed{IntPrimitive: 2, IntBoxed: 4}, row, row, "bean(2,4)")
}

// fcmDeployLowerUpperSetter registers lower/upper as null-initialized
// variables (Java create variable int) and deploys the on-set trigger
// mirroring "on SupportBean set lower=intPrimitive, upper=intBoxed".
func fcmDeployLowerUpperSetter(t *testing.T, env *Environment, engine *Engine) {
	t.Helper()
	for _, name := range []string{"lower", "upper"} {
		if err := env.RegisterVariable(name, nil); err != nil {
			t.Fatal(err)
		}
	}
	setterPlan, err := env.Build(OnEvent(From[fcmSupportBeanBoxed](env, "SupportBean")).SetVariables(
		SetVariableExpr("lower", Field[fcmSupportBeanBoxed, int]("intPrimitive")),
		SetVariableExpr("upper", Field[fcmSupportBeanBoxed, int]("intBoxed")),
	).Query(StatementName("fcm-set-lower-upper")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), setterPlan); err != nil {
		t.Fatal(err)
	}
}

// TestFromClauseMethod2JoinHistoricalOnlyDependentParity mirrors
// EPLFromClauseMethod2JoinHistoricalOnlyDependent: fetchIdDelimited(value) is
// subordinate to fetchBetween(lower,upper); both textual orders re-poll on
// variable change and the listener never fires.
func TestFromClauseMethod2JoinHistoricalOnlyDependentParity(t *testing.T) {
	fields := []string{"value", "result"}

	build := func(t *testing.T, reversed bool) (*Environment, *Engine, Query) {
		env := newFCMBoxedEnvironment(t)
		valueSchema := fcmMapSchema(t, env, "FCMBetweenRow", FieldDef("value", reflect.TypeOf(0)))
		resultSchema := fcmMapSchema(t, env, "FCMDelimitedRow", FieldDef("result", reflect.TypeOf("")))
		between := FromMethod[map[string]any](env, "between", valueSchema, makeFCMFetchBetweenProvider(valueSchema, "lower", "upper"))
		delimited := FromMethod[map[string]any](env, "delimited", resultSchema, makeFCMFetchIdDelimitedProvider(resultSchema, "between")).DependingOn("between")
		var query Query
		if reversed {
			query = JoinMany(JoinSource(delimited), JoinSource(between)).Select(
				SelectFrom(1, "value", Field[map[string]any, int]("value")),
				SelectFrom(0, "result", Field[map[string]any, string]("result")),
			).Query(StatementName("fcm-2hist-only-dep-rev"))
		} else {
			query = JoinMany(JoinSource(between), JoinSource(delimited)).Select(
				SelectFrom(0, "value", Field[map[string]any, int]("value")),
				SelectFrom(1, "result", Field[map[string]any, string]("result")),
			).Query(StatementName("fcm-2hist-only-dep"))
		}
		return env, NewEngine(env), query
	}

	for _, reversed := range []bool{false, true} {
		name := "declared"
		if reversed {
			name = "reversed"
		}
		t.Run(name, func(t *testing.T) {
			env, engine, query := build(t, reversed)
			t.Cleanup(func() { _ = engine.Close(context.Background()) })
			fcmDeployLowerUpperSetter(t, env, engine)

			plan, err := env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			stmt := deployment.Statements()[0]
			listener := fcmOuterSubscribe(t, stmt)

			send := func(intPrimitive, intBoxed int) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), fcmSupportBeanBoxed{IntPrimitive: intPrimitive, IntBoxed: intBoxed}); err != nil {
					t.Fatal(err)
				}
			}
			assertIterator := func(expected [][]string, label string) {
				t.Helper()
				fcmOuterAssertRows(t, fcmOuterSnapshot(t, stmt), fields, expected, label+" iterator")
				if listener.invoked {
					t.Fatalf("%s: listener invoked unexpectedly with %#v", label, listener.lastNew)
				}
			}

			assertIterator(nil, "initial")

			send(5, 5)
			assertIterator([][]string{{"5", "|5|"}}, "bean(5,5)")

			send(1, 2)
			assertIterator([][]string{{"1", "|1|"}, {"2", "|2|"}}, "bean(1,2)")

			send(0, -1)
			assertIterator(nil, "bean(0,-1)")

			send(4, 6)
			assertIterator([][]string{{"4", "|4|"}, {"5", "|5|"}, {"6", "|6|"}}, "bean(4,6)")

			if err := deployment.Undeploy(context.Background()); err != nil {
				t.Fatal(err)
			}
			send(0, -1)
			if listener.invoked {
				t.Fatalf("post-undeploy: listener invoked unexpectedly with %#v", listener.lastNew)
			}
		})
	}
}

// TestFromClauseMethod2JoinHistoricalOnlyIndependentParity mirrors
// EPLFromClauseMethod2JoinHistoricalOnlyIndependent: two independent
// variable-driven method sources in a Cartesian join over both textual orders.
func TestFromClauseMethod2JoinHistoricalOnlyIndependentParity(t *testing.T) {
	fields := []string{"valueOne", "valueTwo"}

	build := func(t *testing.T, reversed bool) (*Environment, *Engine, Query) {
		env := newFCMBoxedEnvironment(t)
		intSchema := fcmMapSchema(t, env, "FCMBetweenRow", FieldDef("value", reflect.TypeOf(0)))
		stringSchema := fcmMapSchema(t, env, "FCMBetweenStringRow", FieldDef("value", reflect.TypeOf("")))
		between := FromMethod[map[string]any](env, "between", intSchema, makeFCMFetchBetweenProvider(intSchema, "lower", "upper"))
		betweenString := FromMethod[map[string]any](env, "betweenString", stringSchema, makeFCMFetchBetweenStringProvider(stringSchema, "lower", "upper"))
		var query Query
		if reversed {
			query = JoinMany(JoinSource(betweenString), JoinSource(between)).Select(
				SelectFrom(1, "valueOne", Field[map[string]any, int]("value")),
				SelectFrom(0, "valueTwo", Field[map[string]any, string]("value")),
			).Query(StatementName("fcm-2hist-only-ind-rev"))
		} else {
			query = JoinMany(JoinSource(between), JoinSource(betweenString)).Select(
				SelectFrom(0, "valueOne", Field[map[string]any, int]("value")),
				SelectFrom(1, "valueTwo", Field[map[string]any, string]("value")),
			).Query(StatementName("fcm-2hist-only-ind"))
		}
		return env, NewEngine(env), query
	}

	for _, reversed := range []bool{false, true} {
		name := "declared"
		if reversed {
			name = "reversed"
		}
		t.Run(name, func(t *testing.T) {
			env, engine, query := build(t, reversed)
			t.Cleanup(func() { _ = engine.Close(context.Background()) })
			fcmDeployLowerUpperSetter(t, env, engine)

			plan, err := env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			stmt := deployment.Statements()[0]
			listener := fcmOuterSubscribe(t, stmt)

			send := func(intPrimitive, intBoxed int) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), fcmSupportBeanBoxed{IntPrimitive: intPrimitive, IntBoxed: intBoxed}); err != nil {
					t.Fatal(err)
				}
			}
			assertIterator := func(expected [][]string, label string) {
				t.Helper()
				fcmOuterAssertRows(t, fcmOuterSnapshot(t, stmt), fields, expected, label+" iterator")
				if listener.invoked {
					t.Fatalf("%s: listener invoked unexpectedly with %#v", label, listener.lastNew)
				}
			}

			assertIterator(nil, "initial")

			send(5, 5)
			assertIterator([][]string{{"5", "5"}}, "bean(5,5)")

			send(1, 2)
			assertIterator([][]string{{"1", "1"}, {"1", "2"}, {"2", "1"}, {"2", "2"}}, "bean(1,2)")

			send(0, -1)
			assertIterator(nil, "bean(0,-1)")
		})
	}
}
