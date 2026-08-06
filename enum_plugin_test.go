package esper

import (
	"sort"
	"strings"
	"testing"
	"time"
)

type enumPluginMedianState struct {
	values []float64
}

func (state *enumPluginMedianState) SetParameter(int, Value) {}

func (state *enumPluginMedianState) Add(item Value, lambdaValues []Value) {
	value := item
	if len(lambdaValues) > 0 {
		value = lambdaValues[0]
	}
	number, ok := enumRatFromValue(value)
	if !ok {
		return
	}
	converted, _ := number.Float64()
	state.values = append(state.values, converted)
}

func (state *enumPluginMedianState) Completed() bool { return false }

func (state *enumPluginMedianState) Result() (float64, bool) {
	if len(state.values) < 2 {
		return 0, false
	}
	values := append([]float64(nil), state.values...)
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 0 {
		return (values[middle-1] + values[middle]) / 2, true
	}
	return values[middle], true
}

func enumPluginMedianFootprints() []EnumMethodFootprint {
	return []EnumMethodFootprint{
		{Input: EnumInputScalarNumeric},
		{Input: EnumInputScalarAny, Parameters: []EnumMethodParameter{enumParameter(1, "value-selector", EnumParameterNumeric)}},
		{Input: EnumInputEventCollection, Parameters: []EnumMethodParameter{enumParameter(1, "value-selector", EnumParameterNumeric)}},
		{Input: EnumInputScalarAny, Parameters: []EnumMethodParameter{enumParameter(2, "(value-selector, index)", EnumParameterNumeric)}},
		{Input: EnumInputEventCollection, Parameters: []EnumMethodParameter{enumParameter(2, "(value-selector, index)", EnumParameterNumeric)}},
		{Input: EnumInputScalarAny, Parameters: []EnumMethodParameter{enumParameter(3, "(value-selector, index, size)", EnumParameterNumeric)}},
		{Input: EnumInputEventCollection, Parameters: []EnumMethodParameter{enumParameter(3, "(value-selector, index, size)", EnumParameterNumeric)}},
	}
}

func buildEnumPluginExpression(t *testing.T, env *Environment, expression Expr) {
	t.Helper()
	if _, err := env.Build(SelectOnce(env, Alias("value", expression))); err != nil {
		t.Fatal(err)
	}
}

func TestEnumPluginScalarMedianFootprintsAndLambdaArity(t *testing.T) {
	env := NewEnvironment()
	if err := RegisterEnumPlugin[float64](env, "median", enumPluginMedianFootprints(), func(context EnumPluginContext) EnumPluginState[float64] {
		if context.Name != "median" || context.ElementType == nil || context.ResultType == nil {
			t.Fatalf("median plugin context = %#v", context)
		}
		return &enumPluginMedianState{}
	}); err != nil {
		t.Fatal(err)
	}
	values := Literal([]int64{1, 2, 2, 4})
	plain := EnumPluginRef[int64, float64](env, "median", values)
	metadataBeforeBuild, ok := EnumerationMetadata(plain)
	if !ok || len(metadataBeforeBuild.Footprints) != 7 || metadataBeforeBuild.ElementType != typeOf[int64]() {
		t.Fatalf("registered plugin metadata before build = %#v", metadataBeforeBuild)
	}
	buildEnumPluginExpression(t, env, plain)
	if got := plain.eval(EvalContext{}); !got.Equal(Present(2.0)) {
		t.Fatalf("scalar median = %v", got)
	}

	selector := EnumLambda1(Func1[int64, int64]("identity", func(value int64) int64 { return value }, EnumElement[int64]()))
	withSelector := EnumPluginRef[int64, float64](env, "median", values, selector)
	buildEnumPluginExpression(t, env, withSelector)
	if got := withSelector.eval(EvalContext{}); !got.Equal(Present(2.0)) {
		t.Fatalf("one-argument median = %v", got)
	}

	withIndex := EnumPluginRef[int64, float64](env, "median", values, EnumLambda2(Add[int64](EnumElement[int64](), EnumIndex())))
	buildEnumPluginExpression(t, env, withIndex)
	if got := withIndex.eval(EvalContext{}); !got.Equal(Present(3.5)) {
		t.Fatalf("two-argument median = %v", got)
	}

	withSize := EnumPluginRef[int64, float64](env, "median", values, EnumLambda3(Add[int64](Add[int64](EnumElement[int64](), EnumIndex()), EnumSize())))
	plan, err := env.Build(SelectOnce(env, Alias("value", withSize)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plan.Canonical()), "enum-plugin(median:") {
		t.Fatalf("plan canonical omits enum plugin registration: %s", plan.Canonical())
	}
	if got := withSize.eval(EvalContext{}); !got.Equal(Present(7.5)) {
		t.Fatalf("three-argument median = %v", got)
	}

	metadata, ok := EnumerationMetadata(withSize)
	if !ok || metadata.Method != "median" || len(metadata.Footprints) != 7 || metadata.ElementType != typeOf[int64]() || metadata.ResultType != typeOf[float64]() {
		t.Fatalf("median metadata = %#v", metadata)
	}
	if again := withSize.eval(EvalContext{}); !again.Equal(Present(7.5)) {
		t.Fatalf("plugin state leaked across evaluations: %v", again)
	}
}

type enumPluginEvent struct {
	ID    string `esper:"id"`
	Value int64  `esper:"value"`
}

func TestEnumPluginEventLambdaAndChainableEventResults(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumPluginEvent](env, "EnumPluginEvent"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("EnumPluginEvent")
	if !ok {
		t.Fatal("event schema was not registered")
	}
	makeEvent := func(id string, value int64) Event {
		event, err := NewEvent(schema, enumPluginEvent{ID: id, Value: value}, timeNowForEnumPlugin())
		if err != nil {
			t.Fatal(err)
		}
		return event
	}
	if err := RegisterEnumPlugin[[]Event](env, "positive-events", []EnumMethodFootprint{{
		Input:      EnumInputEventCollection,
		Parameters: []EnumMethodParameter{enumParameter(1, "predicate", EnumParameterBoolean)},
	}}, func(EnumPluginContext) EnumPluginState[[]Event] {
		return &enumPluginEventsState{}
	}); err != nil {
		t.Fatal(err)
	}
	if err := RegisterEnumPlugin[float64](env, "event-median", enumPluginMedianFootprints(), func(context EnumPluginContext) EnumPluginState[float64] {
		if context.ElementType != typeOf[Event]() {
			t.Fatalf("event median element type = %v", context.ElementType)
		}
		return &enumPluginMedianState{}
	}); err != nil {
		t.Fatal(err)
	}
	values := Literal([]Event{makeEvent("E1", -1), makeEvent("E2", 10), makeEvent("E3", 20), makeEvent("E4", 3)})
	predicate := Greater[int64](Property[int64](EnumElement[Event](), "value"), Literal(int64(0)))
	median := EnumPluginRef[Event, float64](env, "event-median", values, EnumLambda1(Property[int64](EnumElement[Event](), "value")))
	buildEnumPluginExpression(t, env, median)
	if got := median.eval(EvalContext{}); !got.Equal(Present(6.5)) {
		t.Fatalf("event median = %v", got)
	}
	filtered := EnumPluginRef[Event, []Event](env, "positive-events", values, EnumLambda1(predicate))
	buildEnumPluginExpression(t, env, filtered)
	last := EnumLastOf[Event](filtered)
	buildEnumPluginExpression(t, env, last)
	lastID := Property[string](last, "id")
	buildEnumPluginExpression(t, env, lastID)
	if got := lastID.eval(EvalContext{}); !got.Equal(Present("E4")) {
		t.Fatalf("last positive event = %v", got)
	}
}

type enumPluginSingleEventState struct {
	event Event
	set   bool
}

func (state *enumPluginSingleEventState) SetParameter(int, Value) {}

func (state *enumPluginSingleEventState) Add(item Value, lambdaValues []Value) {
	if len(lambdaValues) == 0 {
		return
	}
	pass, ok := boolValue(lambdaValues[0])
	if !ok || !pass {
		return
	}
	event, err := As[Event](item)
	if err == nil {
		state.event = event
		state.set = true
	}
}

func (state *enumPluginSingleEventState) Completed() bool { return state.set }
func (state *enumPluginSingleEventState) Result() (Event, bool) {
	return state.event, state.set
}

type enumPluginCountState struct{ count int64 }

func (state *enumPluginCountState) SetParameter(int, Value) {}
func (state *enumPluginCountState) Add(_ Value, lambdaValues []Value) {
	if len(lambdaValues) == 0 {
		return
	}
	if pass, ok := boolValue(lambdaValues[0]); ok && pass {
		state.count++
	}
}
func (state *enumPluginCountState) Completed() bool       { return false }
func (state *enumPluginCountState) Result() (int64, bool) { return state.count, true }

func TestEnumPluginSingleEventTwoLambdaAndValueIndexParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumPluginEvent](env, "EnumPluginEvent"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("EnumPluginEvent")
	if !ok {
		t.Fatal("event schema was not registered")
	}
	makeEvent := func(id string, value int64) Event {
		event, err := NewEvent(schema, enumPluginEvent{ID: id, Value: value}, timeNowForEnumPlugin())
		if err != nil {
			t.Fatal(err)
		}
		return event
	}
	events := Literal([]Event{
		makeEvent("E1", -1),
		makeEvent("E2", 2),
		makeEvent("E3", 4),
		makeEvent("E4", 0),
	})
	predicateFootprints := []EnumMethodFootprint{
		{Input: EnumInputScalarNumeric, Parameters: []EnumMethodParameter{enumParameter(2, "predicate", EnumParameterBoolean)}},
		{Input: EnumInputEventCollection, Parameters: []EnumMethodParameter{enumParameter(2, "predicate", EnumParameterBoolean)}},
	}
	if err := RegisterEnumPlugin[int64](env, "predicate-index", predicateFootprints, func(EnumPluginContext) EnumPluginState[int64] {
		return &enumPluginCountState{}
	}); err != nil {
		t.Fatal(err)
	}
	if err := RegisterEnumPlugin[Event](env, "first-positive-event", []EnumMethodFootprint{{
		Input:      EnumInputEventCollection,
		Parameters: []EnumMethodParameter{enumParameter(1, "predicate", EnumParameterBoolean)},
	}}, func(EnumPluginContext) EnumPluginState[Event] {
		return &enumPluginSingleEventState{}
	}); err != nil {
		t.Fatal(err)
	}
	if err := RegisterEnumPlugin[int64](env, "two-lambda-valid", []EnumMethodFootprint{{
		Input: EnumInputEventCollection,
		Parameters: []EnumMethodParameter{
			enumParameter(1, "v1", EnumParameterAny),
			enumParameter(1, "v2", EnumParameterAny),
		},
	}}, func(EnumPluginContext) EnumPluginState[int64] {
		return &enumPluginTwoLambdaState{}
	}); err != nil {
		t.Fatal(err)
	}

	eventPredicate := And(
		Greater[int64](Property[int64](EnumElement[Event](), "value"), Literal(int64(0))),
		Less[int64](EnumIndex(), Literal(int64(3))),
	)
	eventCount := EnumPluginRef[Event, int64](env, "predicate-index", events, EnumLambda2(eventPredicate))
	buildEnumPluginExpression(t, env, eventCount)
	if got := eventCount.eval(EvalContext{}); !got.Equal(Present(int64(2))) {
		t.Fatalf("event value/index predicate count = %v", got)
	}

	scalarPredicate := And(
		Greater[int64](EnumElement[int64](), Literal(int64(0))),
		Less[int64](EnumIndex(), Literal(int64(3))),
	)
	scalarCount := EnumPluginRef[int64, int64](env, "predicate-index", Literal([]int64{-1, 2, 2, 2}), EnumLambda2(scalarPredicate))
	buildEnumPluginExpression(t, env, scalarCount)
	if got := scalarCount.eval(EvalContext{}); !got.Equal(Present(int64(2))) {
		t.Fatalf("scalar value/index predicate count = %v", got)
	}

	firstPositive := EnumPluginRef[Event, Event](env, "first-positive-event", events, EnumLambda1(Greater[int64](Property[int64](EnumElement[Event](), "value"), Literal(int64(0)))))
	firstID := Property[string](firstPositive, "id")
	buildEnumPluginExpression(t, env, firstID)
	if got := firstID.eval(EvalContext{}); !got.Equal(Present("E2")) {
		t.Fatalf("single event result = %v", got)
	}

	valueOne := Multiply[int64](Property[int64](EnumElement[Event](), "value"), Literal(int64(2)))
	valueTwo := Multiply[int64](Property[int64](EnumElement[Event](), "value"), Literal(int64(3)))
	twoLambda := EnumPluginRef[Event, int64](env, "two-lambda-valid", events, EnumLambda1(valueOne), EnumLambda1(valueTwo))
	buildEnumPluginExpression(t, env, twoLambda)
	if got := twoLambda.eval(EvalContext{}); !got.Equal(Present(int64(25))) {
		t.Fatalf("two-lambda result = %v", got)
	}
}

type enumPluginEventsState struct {
	values []Event
}

func (state *enumPluginEventsState) SetParameter(int, Value) {}

func (state *enumPluginEventsState) Add(item Value, lambdaValues []Value) {
	if len(lambdaValues) == 0 {
		return
	}
	pass, ok := boolValue(lambdaValues[0])
	if !ok || !pass {
		return
	}
	event, err := As[Event](item)
	if err == nil {
		state.values = append(state.values, event)
	}
}

func (state *enumPluginEventsState) Completed() bool { return false }
func (state *enumPluginEventsState) Result() ([]Event, bool) {
	if len(state.values) == 0 {
		return nil, false
	}
	return append([]Event(nil), state.values...), true
}

func TestEnumPluginParametersEarlyExitAndTwoLambdaState(t *testing.T) {
	env := NewEnvironment()
	if err := RegisterEnumPlugin[int64](env, "range-sum", []EnumMethodFootprint{{
		Input: EnumInputScalarNumeric,
		Parameters: []EnumMethodParameter{
			enumParameter(0, "from", EnumParameterNumeric),
			enumParameter(0, "to", EnumParameterNumeric),
		},
	}}, func(EnumPluginContext) EnumPluginState[int64] { return &enumPluginRangeState{} }); err != nil {
		t.Fatal(err)
	}
	if err := RegisterEnumPlugin[int64](env, "early-sum", []EnumMethodFootprint{{Input: EnumInputScalarNumeric}}, func(EnumPluginContext) EnumPluginState[int64] {
		return &enumPluginEarlySumState{}
	}); err != nil {
		t.Fatal(err)
	}
	if err := RegisterEnumPlugin[int64](env, "two-lambda", []EnumMethodFootprint{{
		Input: EnumInputEventCollection,
		Parameters: []EnumMethodParameter{
			enumParameter(1, "v1", EnumParameterNumeric),
			enumParameter(1, "v2", EnumParameterNumeric),
		},
	}}, func(EnumPluginContext) EnumPluginState[int64] { return &enumPluginTwoLambdaState{} }); err != nil {
		t.Fatal(err)
	}

	rangeExpression := EnumPluginRef[int64, int64](env, "range-sum", Literal([]int64{1, 2, 11, 3}), EnumArgument(Literal(int64(10))), EnumArgument(Literal(int64(20))))
	buildEnumPluginExpression(t, env, rangeExpression)
	if got := rangeExpression.eval(EvalContext{}); !got.Equal(Present(int64(11))) {
		t.Fatalf("range plugin = %v", got)
	}
	earlyExpression := EnumPluginRef[int64, int64](env, "early-sum", Literal([]int64{5, 5, 5}))
	buildEnumPluginExpression(t, env, earlyExpression)
	if got := earlyExpression.eval(EvalContext{}); !got.Equal(Present(int64(10))) {
		t.Fatalf("early-exit plugin = %v", got)
	}

	// A two-lambda plugin is also valid for scalar values when its footprint
	// declares ANY; this event footprint intentionally rejects that mismatch.
	if _, err := env.Build(SelectOnce(env, Alias("bad", EnumPluginRef[int64, int64](env, "two-lambda", Literal([]int64{1}), EnumLambda1(Literal(int64(1))), EnumLambda1(Literal(int64(2))))))); err == nil {
		t.Fatal("event-only two-lambda plugin unexpectedly accepted scalar input")
	}
}

func TestEnumPluginScalarAnyAndStateParameterSlots(t *testing.T) {
	env := NewEnvironment()
	if err := RegisterEnumPlugin[int64](env, "parameter-slots", []EnumMethodFootprint{{
		Input: EnumInputScalarAny,
		Parameters: []EnumMethodParameter{
			enumParameter(1, "predicate", EnumParameterBoolean),
			enumParameter(0, "initial", EnumParameterNumeric),
		},
	}}, func(EnumPluginContext) EnumPluginState[int64] {
		return &enumPluginParameterSlotState{}
	}); err != nil {
		t.Fatal(err)
	}

	expression := EnumPluginRef[string, int64](env, "parameter-slots", Literal([]string{"a", "b"}),
		EnumLambda1(Literal(true)), EnumArgument(Literal(int64(7))))
	buildEnumPluginExpression(t, env, expression)
	if got := expression.eval(EvalContext{}); !got.Equal(Present(int64(7))) {
		t.Fatalf("non-lambda state parameter slot = %v", got)
	}
}

type enumPluginParameterSlotState struct {
	initial int64
}

func (state *enumPluginParameterSlotState) SetParameter(parameterNumber int, value Value) {
	if parameterNumber != 0 {
		return
	}
	if initial, err := As[int64](value); err == nil {
		state.initial = initial
	}
}
func (state *enumPluginParameterSlotState) Add(Value, []Value) {}
func (state *enumPluginParameterSlotState) Completed() bool    { return false }
func (state *enumPluginParameterSlotState) Result() (int64, bool) {
	return state.initial, true
}

type enumPluginRangeState struct{ from, to, sum int64 }

func (state *enumPluginRangeState) SetParameter(parameterNumber int, value Value) {
	valueInt, err := As[int64](value)
	if err != nil {
		return
	}
	if parameterNumber == 0 {
		state.from = valueInt
	} else if parameterNumber == 1 {
		state.to = valueInt
	}
}
func (state *enumPluginRangeState) Add(item Value, _ []Value) {
	value, err := As[int64](item)
	if err == nil && value >= state.from && value <= state.to {
		state.sum += value
	}
}
func (state *enumPluginRangeState) Completed() bool       { return false }
func (state *enumPluginRangeState) Result() (int64, bool) { return state.sum, true }

type enumPluginEarlySumState struct{ sum int64 }

func (state *enumPluginEarlySumState) SetParameter(int, Value) {}
func (state *enumPluginEarlySumState) Add(item Value, _ []Value) {
	value, err := As[int64](item)
	if err == nil {
		state.sum += value
	}
}
func (state *enumPluginEarlySumState) Completed() bool       { return state.sum >= 10 }
func (state *enumPluginEarlySumState) Result() (int64, bool) { return state.sum, true }

type enumPluginTwoLambdaState struct{ sum int64 }

func (state *enumPluginTwoLambdaState) SetParameter(int, Value) {}
func (state *enumPluginTwoLambdaState) Add(_ Value, lambdaValues []Value) {
	for _, value := range lambdaValues {
		if number, err := As[int64](value); err == nil {
			state.sum += number
		}
	}
}
func (state *enumPluginTwoLambdaState) Completed() bool       { return false }
func (state *enumPluginTwoLambdaState) Result() (int64, bool) { return state.sum, true }

func TestEnumPluginStateGetterDirectRegistrationAndInvalidRules(t *testing.T) {
	env := NewEnvironment()
	footprints := []EnumMethodFootprint{{
		Input: EnumInputAny,
		Parameters: []EnumMethodParameter{
			enumParameter(0, "initialvalue", EnumParameterAny),
			enumParameter(2, "(result, next)", EnumParameterAny),
		},
	}}
	stateGetter := EnumPlugin[string, string]("state-and-value", Literal([]string{"a", "b"}), footprints, func(EnumPluginContext) EnumPluginState[string] {
		return &enumPluginStringState{}
	}, EnumArgument(Literal("X")), EnumLambda2(Concat(EnumPluginStateValue[string](), EnumElement[string]())))
	buildEnumPluginExpression(t, env, stateGetter)
	if got := stateGetter.eval(EvalContext{}); !got.Equal(Present("Xab")) {
		t.Fatalf("state getter plugin = %v", got)
	}

	if err := RegisterEnumPlugin[int64](env, "bad", nil, func(EnumPluginContext) EnumPluginState[int64] { return &enumPluginEarlySumState{} }); err == nil {
		t.Fatal("empty plugin footprint registration unexpectedly succeeded")
	}
	if err := RegisterEnumPlugin[int64](env, "bad-arity", []EnumMethodFootprint{{Input: EnumInputAny, Parameters: []EnumMethodParameter{enumParameter(-1, "bad", EnumParameterAny)}}}, func(EnumPluginContext) EnumPluginState[int64] { return &enumPluginEarlySumState{} }); err == nil {
		t.Fatal("invalid lambda arity registration unexpectedly succeeded")
	}
	if err := RegisterEnumPlugin[int64](env, "registered", []EnumMethodFootprint{{Input: EnumInputScalarNumeric}}, func(EnumPluginContext) EnumPluginState[int64] { return &enumPluginEarlySumState{} }); err != nil {
		t.Fatal(err)
	}
	footprints, ok := env.EnumPluginFootprints("registered")
	if !ok || len(footprints) != 1 {
		t.Fatalf("registered plugin footprints = %#v, %v", footprints, ok)
	}
	footprints[0].Input = EnumInputAny
	unchanged, ok := env.EnumPluginFootprints("registered")
	if !ok || unchanged[0].Input != EnumInputScalarNumeric {
		t.Fatalf("plugin footprints were not defensive copies: %#v", unchanged)
	}
	if err := RegisterEnumPlugin[int64](env, "registered", []EnumMethodFootprint{{Input: EnumInputScalarNumeric}}, func(EnumPluginContext) EnumPluginState[int64] { return &enumPluginEarlySumState{} }); err == nil {
		t.Fatal("duplicate plugin registration unexpectedly succeeded")
	}
	if _, err := env.Build(SelectOnce(env, Alias("unknown", EnumPluginRef[int64, int64](env, "missing", Literal([]int64{1}))))); err == nil {
		t.Fatal("unknown plugin unexpectedly built")
	}
	if _, err := env.Build(SelectOnce(env, Alias("wrong", EnumPluginRef[int64, string](env, "registered", Literal([]int64{1}))))); err == nil {
		t.Fatal("plugin result type mismatch unexpectedly built")
	}
}

type enumPluginStringState struct{ value string }

func (state *enumPluginStringState) SetParameter(parameterNumber int, value Value) {
	if parameterNumber == 0 {
		if initial, err := As[string](value); err == nil {
			state.value = initial
		}
	}
}
func (state *enumPluginStringState) Add(_ Value, lambdaValues []Value) {
	if len(lambdaValues) == 1 {
		if value, err := As[string](lambdaValues[0]); err == nil {
			state.value = value
		}
	}
}
func (state *enumPluginStringState) Completed() bool        { return false }
func (state *enumPluginStringState) Result() (string, bool) { return state.value, state.value != "" }

// timeNowForEnumPlugin keeps the event construction independent of the
// runtime clock; the exact timestamp is not part of this plugin contract.
func timeNowForEnumPlugin() time.Time { return time.Unix(0, 0).UTC() }
