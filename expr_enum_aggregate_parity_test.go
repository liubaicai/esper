package esper

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type enumAggregateItem struct {
	ID    string `esper:"id"`
	Score int64  `esper:"score"`
}

type enumAggregateContainer struct {
	Items []enumAggregateItem `esper:"items"`
	Seed  int64               `esper:"seed"`
}

func enumAggregateIntString(value int64) string {
	return strconv.FormatInt(value, 10)
}

func enumAggregateAppendID(result, id string) string {
	if result == "" {
		return id
	}
	return result + "," + id
}

func enumAggregateScalarExpressions(values Expression[[]string]) []Expression[string] {
	item := EnumElement[string]()
	indexText := Func1[int64, string]("itoa", enumAggregateIntString, EnumIndex())
	sizeText := Func1[int64, string]("itoa", enumAggregateIntString, EnumSize())
	return []Expression[string]{
		EnumAggregate[string, string](values, Literal(""), Concat(EnumAccumulator[string](), Concat(Literal("+"), item))),
		EnumAggregate[string, string](values, Literal(""), Concat(EnumAccumulator[string](), Concat(Literal("+"), Concat(item, Concat(Literal("_"), indexText))))),
		EnumAggregate[string, string](values, Literal(""), Concat(EnumAccumulator[string](), Concat(Literal("+"), Concat(item, Concat(Literal("_"), Concat(indexText, Concat(Literal("_"), sizeText))))))),
		EnumAggregate[string, string](values, Literal(""), NullLiteral[string]()),
	}
}

// TestExprEnumAggregateScalarParity covers scalar folds with 2/3/4 lambda
// parameters, empty/Null collections and a Null accumulator result.
func TestExprEnumAggregateScalarParity(t *testing.T) {
	assert := func(values Expression[[]string], wants ...any) {
		t.Helper()
		for index, expression := range enumAggregateScalarExpressions(values) {
			got := expression.eval(EvalContext{})
			if wants[index] == nil {
				if !got.IsNull() {
					t.Fatalf("aggregate scalar c%d = %v, want null", index, got)
				}
				continue
			}
			if !got.Equal(Present(wants[index])) {
				t.Fatalf("aggregate scalar c%d = %v, want %v", index, got, wants[index])
			}
		}
	}

	assert(Literal([]string{"E1", "E2", "E3"}), "+E1+E2+E3", "+E1_0+E2_1+E3_2", "+E1_0_3+E2_1_3+E3_2_3", nil)
	assert(Literal([]string{"E1"}), "+E1", "+E1_0", "+E1_0_1", nil)
	assert(Literal([]string{}), "", "", "", "")
	assert(NullLiteral[[]string](), nil, nil, nil, nil)
}

// TestExprEnumAggregateEventParity covers event folds and verifies that the
// initialization expression is evaluated from each event and enters Plan
// identity rather than living outside the expression AST.
func TestExprEnumAggregateEventParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumAggregateContainer](env, "AggregateContainer"); err != nil {
		t.Fatal(err)
	}

	items := Field[enumAggregateContainer, []enumAggregateItem]("items")
	score := EnumField[enumAggregateItem, int64]("score")
	id := EnumField[enumAggregateItem, string]("id")
	seed := Field[enumAggregateContainer, int64]("seed")
	indexTerm := Multiply[int64](Literal(int64(10)), EnumIndex())
	sizeTerm := Multiply[int64](Literal(int64(100)), EnumSize())
	selections := []Selection{
		Alias("c0", EnumAggregate[enumAggregateItem, int64](items, Literal(int64(0)), Add[int64](EnumAccumulator[int64](), score))),
		Alias("c1", EnumAggregate[enumAggregateItem, string](items, Literal(""), Concat(EnumAccumulator[string](), Concat(Literal(", "), id)))),
		Alias("c2", EnumAggregate[enumAggregateItem, string](items, Literal(""), Func2[string, string, string]("appendID", enumAggregateAppendID, EnumAccumulator[string](), id))),
		Alias("c3", EnumAggregate[enumAggregateItem, int64](items, Literal(int64(0)), Add[int64](EnumAccumulator[int64](), Add[int64](score, indexTerm)))),
		Alias("c4", EnumAggregate[enumAggregateItem, int64](items, Literal(int64(0)), Add[int64](EnumAccumulator[int64](), Add[int64](Add[int64](score, indexTerm), sizeTerm)))),
		Alias("c5", EnumAggregate[enumAggregateItem, int64](items, Literal(int64(0)), NullLiteral[int64]())),
		Alias("seeded", EnumAggregate[enumAggregateItem, int64](items, seed, Add[int64](EnumAccumulator[int64](), score))),
	}
	plan, err := env.Build(Select(From[enumAggregateContainer](env, "AggregateContainer"), selections...).Query(StatementName("enum-aggregate-event")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("aggregate result schema is missing")
	}
	for _, name := range []string{"c0", "c3", "c4", "c5", "seeded"} {
		field, exists := resultSchema.Field(name)
		if !exists || field.Type != reflect.TypeOf(int64(0)) {
			t.Fatalf("aggregate result field %q = %#v, want int64", name, field)
		}
	}
	for _, name := range []string{"c1", "c2"} {
		field, exists := resultSchema.Field(name)
		if !exists || field.Type != reflect.TypeOf("") {
			t.Fatalf("aggregate result field %q = %#v, want string", name, field)
		}
	}

	fieldSeedPlan, err := env.Build(Select(From[enumAggregateContainer](env, "AggregateContainer"),
		Alias("seeded", EnumAggregate[enumAggregateItem, int64](items, seed, Add[int64](EnumAccumulator[int64](), score))),
	).Query(StatementName("enum-aggregate-seed-identity")))
	if err != nil {
		t.Fatal(err)
	}
	literalSeedPlan, err := env.Build(Select(From[enumAggregateContainer](env, "AggregateContainer"),
		Alias("seeded", EnumAggregate[enumAggregateItem, int64](items, Literal(int64(0)), Add[int64](EnumAccumulator[int64](), score))),
	).Query(StatementName("enum-aggregate-seed-identity")))
	if err != nil {
		t.Fatal(err)
	}
	if fieldSeedPlan.Hash() == literalSeedPlan.Hash() {
		t.Fatal("aggregate initialization expression did not enter Plan identity")
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectDotRows(t, deployment)

	if err := engine.SendEvent(context.Background(), enumAggregateContainer{
		Seed: 5,
		Items: []enumAggregateItem{
			{ID: "E1", Score: 12},
			{ID: "E2", Score: 11},
			{ID: "E2", Score: 2},
		},
	}); err != nil {
		t.Fatal(err)
	}
	first := (*rows)[0]
	if !first.Get("c0").Equal(Present(int64(25))) ||
		!first.Get("c1").Equal(Present(", E1, E2, E2")) ||
		!first.Get("c2").Equal(Present("E1,E2,E2")) ||
		!first.Get("c3").Equal(Present(int64(55))) ||
		!first.Get("c4").Equal(Present(int64(955))) ||
		!first.Get("c5").IsNull() ||
		!first.Get("seeded").Equal(Present(int64(30))) {
		t.Fatalf("aggregate event row 0 = %#v", first.AsMap())
	}

	if err := engine.SendEvent(context.Background(), enumAggregateContainer{Seed: 7, Items: []enumAggregateItem{}}); err != nil {
		t.Fatal(err)
	}
	empty := (*rows)[1]
	if !empty.Get("c0").Equal(Present(int64(0))) || !empty.Get("c1").Equal(Present("")) ||
		!empty.Get("c2").Equal(Present("")) || !empty.Get("c3").Equal(Present(int64(0))) ||
		!empty.Get("c4").Equal(Present(int64(0))) || !empty.Get("c5").Equal(Present(int64(0))) ||
		!empty.Get("seeded").Equal(Present(int64(7))) {
		t.Fatalf("aggregate empty row = %#v", empty.AsMap())
	}

	if err := engine.SendEvent(context.Background(), enumAggregateContainer{Seed: 9, Items: []enumAggregateItem{{ID: "E1", Score: 12}}}); err != nil {
		t.Fatal(err)
	}
	single := (*rows)[2]
	if !single.Get("c0").Equal(Present(int64(12))) || !single.Get("c1").Equal(Present(", E1")) ||
		!single.Get("c2").Equal(Present("E1")) || !single.Get("c3").Equal(Present(int64(12))) ||
		!single.Get("c4").Equal(Present(int64(112))) || !single.Get("c5").IsNull() ||
		!single.Get("seeded").Equal(Present(int64(21))) {
		t.Fatalf("aggregate single row = %#v", single.AsMap())
	}

	nullInput := EnumAggregate[enumAggregateItem, int64](NullLiteral[[]enumAggregateItem](), Literal(int64(3)), Add[int64](EnumAccumulator[int64](), score)).eval(EvalContext{})
	if !nullInput.IsNull() {
		t.Fatalf("aggregate null input = %v, want null", nullInput)
	}
}

// TestExprEnumAggregateInvalidParity covers each missing expression and the
// Java null-typed initialization rejection. Incompatible accumulator result
// types are rejected earlier by Go's generic signature.
func TestExprEnumAggregateInvalidParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "AggregateInvalid", nil); err != nil {
		t.Fatal(err)
	}
	stream := From[map[string]any](env, "AggregateInvalid")
	values := Literal([]int64{1})
	accumulator := Add[int64](EnumAccumulator[int64](), EnumElement[int64]())
	for _, testCase := range []struct {
		name string
		expr Expr
		want string
	}{
		{name: "missing-values", expr: EnumAggregate[int64, int64](nil, Literal(int64(0)), accumulator), want: `requires a collection expression`},
		{name: "missing-initial", expr: EnumAggregate[int64, int64](values, nil, accumulator), want: `requires an initialization expression`},
		{name: "null-initial", expr: EnumAggregate[int64, int64](values, NullLiteral[int64](), accumulator), want: `initialization expression cannot be null-typed`},
		{name: "missing-accumulator", expr: EnumAggregate[int64, int64](values, Literal(int64(0)), nil), want: `requires an accumulator expression`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(stream, Alias("result", testCase.expr)).Query(StatementName("aggregate-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("aggregate invalid error = %v, want fragment %q", err, testCase.want)
			}
		})
	}
}
