package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type enumSequenceEqualItem struct {
	Key string `esper:"key"`
	ID  string `esper:"id"`
}

type enumSequenceEqualContainer struct {
	Contained []enumSequenceEqualItem `esper:"contained"`
}

func enumSequenceEqualString(value string) *string {
	return &value
}

func enumSequenceEqualAssert(t *testing.T, expression Expression[bool], want any, label string) {
	t.Helper()
	got := expression.eval(EvalContext{})
	if want == nil {
		if !got.IsNull() {
			t.Fatalf("%s = %v, want null", label, got)
		}
		return
	}
	if !got.Equal(Present(want)) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

// TestExprEnumSequenceEqualSelectFromParity covers the event selectFrom
// footprint: two ordered projections from the same event collection compare
// equal only when every projected element is equal and in the same position.
func TestExprEnumSequenceEqualSelectFromParity(t *testing.T) {
	makeExpression := func(items []enumSequenceEqualItem) Expression[bool] {
		values := Literal(items)
		keys := EnumSelect[enumSequenceEqualItem, string](values, EnumField[enumSequenceEqualItem, string]("key"))
		ids := EnumSelect[enumSequenceEqualItem, string](values, EnumField[enumSequenceEqualItem, string]("id"))
		return EnumSequenceEqual[string](keys, ids)
	}

	enumSequenceEqualAssert(t, makeExpression([]enumSequenceEqualItem{{Key: "I1", ID: "E1"}, {Key: "I2", ID: "E2"}}), false, "select-from-different")
	enumSequenceEqualAssert(t, makeExpression([]enumSequenceEqualItem{{Key: "E1", ID: "E1"}, {Key: "E2", ID: "E2"}}), true, "select-from-equal")
	enumSequenceEqualAssert(t, makeExpression([]enumSequenceEqualItem{{Key: "E1", ID: "E1"}, {Key: "E2", ID: "E3"}}), false, "select-from-one-mismatch")
	enumSequenceEqualAssert(t, makeExpression([]enumSequenceEqualItem{{Key: "E1", ID: "E2"}, {Key: "E2", ID: "E1"}}), false, "select-from-order-mismatch")
}

// TestExprEnumSequenceEqualScalarParity covers the two-property scalar
// footprint, including length, empty, null and null-element semantics.
func TestExprEnumSequenceEqualScalarParity(t *testing.T) {
	compare := func(left, right []*string) Expression[bool] {
		var leftExpression, rightExpression Expression[[]*string]
		if left == nil {
			leftExpression = NullLiteral[[]*string]()
		} else {
			leftExpression = Literal(left)
		}
		if right == nil {
			rightExpression = NullLiteral[[]*string]()
		} else {
			rightExpression = Literal(right)
		}
		return EnumSequenceEqual[*string](leftExpression, rightExpression)
	}

	value := func(values ...string) []*string {
		result := make([]*string, 0, len(values))
		for _, item := range values {
			result = append(result, enumSequenceEqualString(item))
		}
		return result
	}

	enumSequenceEqualAssert(t, compare(value("E1", "E2", "E3"), value("E1", "E2", "E3")), true, "scalar-equal")
	enumSequenceEqualAssert(t, compare(value("E1", "E3"), value("E1", "E2", "E3")), false, "scalar-left-shorter")
	enumSequenceEqualAssert(t, compare(value("E1", "E2", "E3"), value("E1", "E3")), false, "scalar-right-shorter")
	enumSequenceEqualAssert(t, compare(value("E1", "E2", "E3"), value("E1", "E2", "E4")), false, "scalar-value-mismatch")
	enumSequenceEqualAssert(t, compare(value("E1", "E2", "E3"), value("E1", "E2", "E3")), true, "scalar-same-length")
	enumSequenceEqualAssert(t, compare([]*string{}, []*string{}), true, "scalar-empty")
	enumSequenceEqualAssert(t, compare(nil, []*string{}), false, "scalar-null-left")
	enumSequenceEqualAssert(t, compare([]*string{}, nil), false, "scalar-null-right")
	enumSequenceEqualAssert(t, EnumSequenceEqual[*string](NullLiteral[[]*string](), NullLiteral[[]*string]()), nil, "scalar-both-null")
	enumSequenceEqualAssert(t, compare([]*string{enumSequenceEqualString("E1"), nil, enumSequenceEqualString("E3")}, []*string{enumSequenceEqualString("E1"), nil, enumSequenceEqualString("E3")}), true, "scalar-null-element")
}

// TestExprEnumSequenceEqualStatementProjectionParity verifies event-property
// operands through the normal Build, Deploy and listener path.
func TestExprEnumSequenceEqualStatementProjectionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumSequenceEqualContainer](env, "SequenceEqualContainer"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "SequenceEqualScalar", []FieldSpec{
		FieldDef("strvals", reflect.TypeOf([]*string{})),
		FieldDef("strvalstwo", reflect.TypeOf([]*string{})),
	}); err != nil {
		t.Fatal(err)
	}

	containerValues := Field[enumSequenceEqualContainer, []enumSequenceEqualItem]("contained")
	containerKeys := EnumSelect[enumSequenceEqualItem, string](containerValues, EnumField[enumSequenceEqualItem, string]("key"))
	containerIDs := EnumSelect[enumSequenceEqualItem, string](containerValues, EnumField[enumSequenceEqualItem, string]("id"))
	containerPlan, err := env.Build(Select(From[enumSequenceEqualContainer](env, "SequenceEqualContainer"),
		Alias("same", EnumSequenceEqual[string](containerKeys, containerIDs)),
	).Query(StatementName("sequence-equal-container")))
	if err != nil {
		t.Fatal(err)
	}

	scalarPlan, err := env.Build(Select(From[map[string]any](env, "SequenceEqualScalar"),
		Alias("same", EnumSequenceEqual[*string](
			Field[map[string]any, []*string]("strvals"),
			Field[map[string]any, []*string]("strvalstwo"),
		)),
	).Query(StatementName("sequence-equal-scalar")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	containerDeployment, err := engine.Deploy(context.Background(), containerPlan)
	if err != nil {
		t.Fatal(err)
	}
	scalarDeployment, err := engine.Deploy(context.Background(), scalarPlan)
	if err != nil {
		t.Fatal(err)
	}

	containerRows := make([]Row, 0, 2)
	if _, err := containerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "sequence-equal container result is not a row")
			}
			containerRows = append(containerRows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	scalarRows := make([]Row, 0, 3)
	if _, err := scalarDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "sequence-equal scalar result is not a row")
			}
			scalarRows = append(scalarRows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), enumSequenceEqualContainer{Contained: []enumSequenceEqualItem{{Key: "E1", ID: "E1"}, {Key: "E2", ID: "E2"}}}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), enumSequenceEqualContainer{Contained: []enumSequenceEqualItem{{Key: "E1", ID: "E2"}, {Key: "E2", ID: "E1"}}}); err != nil {
		t.Fatal(err)
	}
	if len(containerRows) != 2 || containerRows[0].Get("same").Any() != true || containerRows[1].Get("same").Any() != false {
		t.Fatalf("container sequence-equal rows = %#v", containerRows)
	}

	e1 := enumSequenceEqualString("E1")
	e2 := enumSequenceEqualString("E2")
	if err := engine.Send(context.Background(), "SequenceEqualScalar", map[string]any{
		"strvals":    []*string{e1, e2},
		"strvalstwo": []*string{enumSequenceEqualString("E1"), enumSequenceEqualString("E2")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SequenceEqualScalar", map[string]any{
		"strvals":    []*string{},
		"strvalstwo": []*string{},
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SequenceEqualScalar", map[string]any{
		"strvals":    nil,
		"strvalstwo": nil,
	}); err != nil {
		t.Fatal(err)
	}
	if len(scalarRows) != 3 || scalarRows[0].Get("same").Any() != true || scalarRows[1].Get("same").Any() != true || scalarRows[2].Get("same").Any() != nil {
		t.Fatalf("scalar sequence-equal rows = %#v", scalarRows)
	}

	if !reflect.DeepEqual(scalarRows[0].Get("same").Any(), true) {
		t.Fatalf("scalar first result metadata = %#v", scalarRows[0].AsMap())
	}
}

// TestExprEnumSequenceEqualInvalidParity covers the typed-builder failure
// that has a direct Go representation. Java's event-collection diagnostic is
// recorded as a difference because Go's generic collection type is explicit.
func TestExprEnumSequenceEqualInvalidParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "SequenceEqualInvalid", nil); err != nil {
		t.Fatal(err)
	}
	stream := From[map[string]any](env, "SequenceEqualInvalid")
	invalid := EnumSequenceEqual[string](nil, nil)
	_, err := env.Build(Select(stream, Alias("same", invalid)).Query(StatementName("sequence-equal-invalid")))
	if err == nil || !strings.Contains(err.Error(), `enumeration method "sequence-equal" requires a collection expression`) {
		t.Fatalf("sequence-equal invalid error = %v", err)
	}
}
