package esper

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// Parity coverage for the EPLVariablesOnSet executions that need indexed or
// object-valued on-set assignments:
//
//   - EPLVariableOnSetArrayAtIndex{soda=false,soda=true}: primitive double[]
//     and String[] element writes via SetVariableIndex.
//   - EPLVariableOnSetArrayBoxed: java.lang.Double[] elements default null and
//     an int literal coerces into a boxed element.
//   - EPLVariableOnSetArrayInvalid runtime part: out-of-range index fails with
//     Java's exact message; null index and null RHS skip silently.
//   - EPLVariableOnSetSubqueryMultikeyWArray: scalar grouped subquery over an
//     int[] key assigns the single group's sum, multi-group assigns null.
//   - EPLVariableOnSetExpression: Helper.swap(VAR) mutates the variable value
//     through a single-variable call; Go expresses it as a pure transform.
//
// Compile diagnostics of EPLVariableOnSetArrayInvalid/EPLVariableOnSetInvalid
// stay an approved difference: Go classifies them via ErrorCode instead of
// Java message text.

type onsetArrayBean struct {
	TheString    string   `esper:"theString"`
	IntPrimitive int32    `esper:"intPrimitive"`
	IntBoxed     *int32   `esper:"intBoxed"`
	DoubleBoxed  *float64 `esper:"doubleBoxed"`
}

type onsetArrayLocalVar struct {
	A int
	B int
}

func registerOnsetArrayTypes(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[onsetArrayBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "SupportEventWithIntArray", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		FieldDef("array", reflect.TypeOf([]int{})),
		FieldDef("value", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
}

func assertOnsetArrayEquals[T any](t *testing.T, engine *Engine, name string, want []T) {
	t.Helper()
	value, ok := engine.GetVariable(name)
	if !ok {
		t.Fatalf("variable %s missing", name)
	}
	got, ok := value.Any().([]T)
	if !ok {
		t.Fatalf("variable %s has unexpected type %T", name, value.Any())
	}
	if len(got) != len(want) {
		t.Fatalf("variable %s = %v, want %v", name, got, want)
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Fatalf("variable %s = %v, want %v", name, got, want)
		}
	}
}

// TestVariableOnSetArrayAtIndexParity mirrors EPLVariableOnSetArrayAtIndex:
// two array-typed variables are element-assigned by one on-trigger statement
// and read back through the variable service.
// Java runtimes: java-runtime-ad6256cf6e3091770906 (soda=false),
// java-runtime-f85344604842bf345a7a (soda=true).
func TestVariableOnSetArrayAtIndexParity(t *testing.T) {
	env := NewEnvironment()
	registerOnsetArrayTypes(t, env)
	if err := env.RegisterVariable("doublearray", []float64{0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("stringarray", []string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}
	intPrimitive := Field[onsetArrayBean, int32]("intPrimitive")
	triggerPlan, err := env.Build(OnEvent(From[onsetArrayBean](env, "SupportBean")).SetVariables(
		SetVariableIndexExpr("doublearray", intPrimitive, Literal[float64](1)),
		SetVariableIndexExpr("stringarray", intPrimitive, Literal("x")),
	).Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), triggerPlan); err != nil {
		t.Fatal(err)
	}
	assertOnsetArrayEquals(t, engine, "doublearray", []float64{0, 0, 0})
	assertOnsetArrayEquals(t, engine, "stringarray", []string{"a", "b", "c"})
	if err := engine.SendEvent(context.Background(), onsetArrayBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	assertOnsetArrayEquals(t, engine, "doublearray", []float64{0, 1, 0})
	assertOnsetArrayEquals(t, engine, "stringarray", []string{"a", "x", "c"})
}

// TestVariableOnSetArrayBoxedParity mirrors EPLVariableOnSetArrayBoxed: a
// Double[] variable starts fully null, an on-set write boxes an int literal,
// and a dependent select observes the post-update array within the same event
// delivery. The observation is order-independent: element writes mutate the
// shared backing array, so the select sees them regardless of dispatch order
// (Java pins the same outcome with @priority(1)).
// Java runtime: java-runtime-2788fec53520551b63b8.
func TestVariableOnSetArrayBoxedParity(t *testing.T) {
	env := NewEnvironment()
	registerOnsetArrayTypes(t, env)
	if err := env.RegisterVariable("dbls", []*float64{nil, nil, nil}); err != nil {
		t.Fatal(err)
	}
	intPrimitive := Field[onsetArrayBean, int32]("intPrimitive")
	selectPlan, err := env.Build(Select(From[onsetArrayBean](env, "SupportBean"),
		Alias("c0", VariableRef[[]*float64]("dbls")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	triggerPlan, err := env.Build(OnEvent(From[onsetArrayBean](env, "SupportBean")).
		SetVariableIndex("dbls", intPrimitive, Literal(1)).
		Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	selectDeployment, err := engine.Deploy(context.Background(), selectPlan)
	if err != nil {
		t.Fatal(err)
	}
	batches := subscribeOnSetVarCapture(t, selectDeployment.Statements()[0])
	if _, err := engine.Deploy(context.Background(), triggerPlan); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), onsetArrayBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	want := []*float64{nil, onsetFloat64Ptr(1), nil}
	value, ok := engine.GetVariable("dbls")
	if !ok {
		t.Fatal("variable dbls missing")
	}
	got, _ := value.Any().([]*float64)
	if len(got) != 3 || got[0] != nil || got[1] == nil || *got[1] != 1 || got[2] != nil {
		t.Fatalf("dbls = %v, want [null 1 null]", got)
	}
	row := lastOnSetVarRow(t, *batches)
	projected := row.Get("c0")
	projectedSlice, ok := projected.Any().([]*float64)
	if !ok {
		t.Fatalf("c0 is not []*float64: %#v", projected.Any())
	}
	for i := range want {
		switch {
		case want[i] == nil && projectedSlice[i] == nil:
			continue
		case want[i] != nil && projectedSlice[i] != nil && *want[i] == *projectedSlice[i]:
			continue
		default:
			t.Fatalf("c0[%d] = %v, want %v", i, projectedSlice[i], want[i])
		}
	}
}

// TestVariableOnSetArrayInvalidRuntimeParity mirrors the runtime half of
// EPLVariableOnSetArrayInvalid: an out-of-range index fails with Java's exact
// message, while a null index and a null RHS for a primitive array skip the
// element write silently.
// Java runtime: java-runtime-36715ebec5e33bcb2cff (compile diagnostics remain
// an approved difference).
func TestVariableOnSetArrayInvalidRuntimeParity(t *testing.T) {
	env := NewEnvironment()
	registerOnsetArrayTypes(t, env)
	if err := env.RegisterVariable("doublearray", []float64{0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	intBoxed := Field[onsetArrayBean, *int32]("intBoxed")
	doubleBoxed := Field[onsetArrayBean, *float64]("doubleBoxed")
	triggerPlan, err := env.Build(OnEvent(From[onsetArrayBean](env, "SupportBean")).
		SetVariableIndex("doublearray", intBoxed, doubleBoxed).
		Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), triggerPlan); err != nil {
		t.Fatal(err)
	}
	overflow := int32(10)
	doubleTen := 10.0
	err = engine.SendEvent(context.Background(), onsetArrayBean{
		TheString:   "O1",
		IntBoxed:    &overflow,
		DoubleBoxed: &doubleTen,
	})
	if err == nil {
		t.Fatal("out-of-range index unexpectedly passed")
	}
	var espErr *Error
	if !errors.As(err, &espErr) {
		t.Fatalf("error is not *Error: %v", err)
	}
	if got, want := espErr.Message, "Array length 3 less than index 10 for variable 'doublearray'"; got != want {
		t.Fatalf("error message = %q, want %q", got, want)
	}
	if err := engine.SendEvent(context.Background(), onsetArrayBean{TheString: "O2", DoubleBoxed: &doubleTen}); err != nil {
		t.Fatal(err)
	}
	assertOnsetArrayEquals(t, engine, "doublearray", []float64{0, 0, 0})
	inRange := int32(1)
	if err := engine.SendEvent(context.Background(), onsetArrayBean{TheString: "O3", IntBoxed: &inRange}); err != nil {
		t.Fatal(err)
	}
	assertOnsetArrayEquals(t, engine, "doublearray", []float64{0, 0, 0})
}

// TestVariableOnSetSubqueryMultikeyWArrayParity mirrors
// EPLVariableOnSetSubqueryMultikeyWArray: the on-trigger assigns from a
// scalar grouped subquery keyed by an int[] content-equal key. A single group
// assigns its sum; multiple groups assign null.
// Java runtime: java-runtime-d23e6717c5568c62cf50.
func TestVariableOnSetSubqueryMultikeyWArrayParity(t *testing.T) {
	env := NewEnvironment()
	registerOnsetArrayTypes(t, env)
	if err := env.RegisterVariable("total_sum", -1); err != nil {
		t.Fatal(err)
	}
	triggerPlan, err := env.Build(OnEvent(From[onsetArrayBean](env, "SupportBean")).SetVariables(
		SetVariableExpr("total_sum", SubqueryGroupScalar[[]int, int](
			FromAny(env, "SupportEventWithIntArray").Window(KeepAll()),
			Field[Event, []int]("array"),
			Sum[int](Field[Event, int]("value")),
		)),
	).Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), triggerPlan); err != nil {
		t.Fatal(err)
	}
	sendSWIA := func(id string, array []int, value int) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportEventWithIntArray", map[string]any{"id": id, "array": array, "value": value}); err != nil {
			t.Fatal(err)
		}
	}
	assertTotalSum := func(want any) {
		t.Helper()
		value, ok := engine.GetVariable("total_sum")
		if !ok {
			t.Fatal("variable total_sum missing")
		}
		switch expected := want.(type) {
		case nil:
			if !value.IsNull() {
				t.Fatalf("total_sum = %v, want null", value.Any())
			}
		case int:
			got, err := As[int](value)
			if err != nil || got != expected {
				t.Fatalf("total_sum = %v, want %d", value.Any(), expected)
			}
		}
	}
	sendSWIA("E1", []int{1, 2}, 10)
	sendSWIA("E2", []int{1, 2}, 11)
	assertTotalSum(-1)
	if err := engine.SendEvent(context.Background(), onsetArrayBean{TheString: ""}); err != nil {
		t.Fatal(err)
	}
	assertTotalSum(21)
	sendSWIA("E3", []int{1, 2}, 12)
	if err := engine.SendEvent(context.Background(), onsetArrayBean{TheString: ""}); err != nil {
		t.Fatal(err)
	}
	assertTotalSum(33)
	sendSWIA("E4", []int{1}, 13)
	if err := engine.SendEvent(context.Background(), onsetArrayBean{TheString: ""}); err != nil {
		t.Fatal(err)
	}
	assertTotalSum(nil)
}

// TestVariableOnSetExpressionParity mirrors EPLVariableOnSetExpression: the
// set clause calls a single-variable helper that mutates the variable value;
// Go expresses the Java static swap as a pure transform applied to the
// current value.
// Java runtime: java-runtime-691fcb9b5f0fe173348f. The inlined-class compile
// form stays an approved difference (no runtime codegen in Go).
func TestVariableOnSetExpressionParity(t *testing.T) {
	env := NewEnvironment()
	registerOnsetArrayTypes(t, env)
	if err := env.RegisterVariable("VAR", onsetArrayLocalVar{A: 1, B: 10}); err != nil {
		t.Fatal(err)
	}
	triggerPlan, err := env.Build(OnEvent(From[onsetArrayBean](env, "SupportBean")).SetVariables(
		SetVariableApply("VAR", func(v onsetArrayLocalVar) onsetArrayLocalVar {
			v.A, v.B = v.B, v.A
			return v
		}),
	).Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), triggerPlan); err != nil {
		t.Fatal(err)
	}
	assertLocalVar := func(a, b int) {
		t.Helper()
		value, ok := engine.GetVariable("VAR")
		if !ok {
			t.Fatal("variable VAR missing")
		}
		got, ok := value.Any().(onsetArrayLocalVar)
		if !ok {
			t.Fatalf("VAR has unexpected type %T", value.Any())
		}
		if got.A != a || got.B != b {
			t.Fatalf("VAR = %+v, want {A:%d B:%d}", got, a, b)
		}
	}
	assertLocalVar(1, 10)
	if err := engine.SendEvent(context.Background(), onsetArrayBean{TheString: ""}); err != nil {
		t.Fatal(err)
	}
	assertLocalVar(10, 1)
}

// TestVariableOnSetIndexAssignmentInvalidParity pins the build-time
// diagnostics for indexed variable assignments: unknown target, non-array
// target, non-integer index expression, element-type mismatch, and constant
// targets stay rejected.
func TestVariableOnSetIndexAssignmentInvalidParity(t *testing.T) {
	env := NewEnvironment()
	registerOnsetArrayTypes(t, env)
	if err := env.RegisterVariable("doublearray", []float64{0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("notAnArray", 7); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("constantVar", "locked", ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	intPrimitive := Field[onsetArrayBean, int32]("intPrimitive")

	build := func(assignments ...VariableAssignmentExpr) error {
		t.Helper()
		_, err := env.Build(OnEvent(From[onsetArrayBean](env, "SupportBean")).SetVariables(assignments...).Query(StatementName("set")))
		return err
	}

	err := build(SetVariableIndexExpr("xxx", intPrimitive, Literal[float64](1)))
	if !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown variable error = %v, want UnknownName", err)
	}
	err = build(SetVariableIndexExpr("notAnArray", intPrimitive, Literal(1)))
	if !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("not-an-array error = %v, want TypeMismatch", err)
	}
	stringIndex := Field[onsetArrayBean, string]("theString")
	err = build(SetVariableIndexExpr("doublearray", stringIndex, Literal[float64](1)))
	if !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("non-integer index error = %v, want TypeMismatch", err)
	}
	err = build(SetVariableIndexExpr("doublearray", intPrimitive, Literal("x")))
	if !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("element mismatch error = %v, want TypeMismatch", err)
	}
	err = build(SetVariableExpr("constantVar", Literal("changed")))
	if !errors.Is(err, ErrorState) {
		t.Fatalf("constant assignment error = %v, want State", err)
	}
}

func onsetFloat64Ptr(v float64) *float64 {
	return &v
}
