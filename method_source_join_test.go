package esper

import (
	"context"
	"testing"
)

type methodJoinValue struct {
	Value int `esper:"value"`
}

func newMethodJoinProvider(t *testing.T, schema Schema, values ...int) MethodProvider {
	t.Helper()
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		events := make([]Event, 0, len(values))
		for _, value := range values {
			event, err := newEvent(schema, methodJoinValue{Value: value}, request.Now)
			if err != nil {
				return nil, err
			}
			events = append(events, event)
		}
		return events, nil
	})
}

func TestMethodSourceIndependentNStreamJoinMatchesEsperShape(t *testing.T) {
	env, engine := newRuntimeTest(t)
	schemaOne, err := StructSchema[methodJoinValue]("MethodNStreamOne")
	if err != nil {
		t.Fatal(err)
	}
	schemaTwo, err := StructSchema[methodJoinValue]("MethodNStreamTwo")
	if err != nil {
		t.Fatal(err)
	}
	schemaThree, err := StructSchema[methodJoinValue]("MethodNStreamThree")
	if err != nil {
		t.Fatal(err)
	}
	first := FromMethodOn[methodJoinValue](env, "first", "Trade", schemaOne, newMethodJoinProvider(t, schemaOne, 2))
	second := FromMethodOn[methodJoinValue](env, "second", "Trade", schemaTwo, newMethodJoinProvider(t, schemaTwo, 1, 2))
	third := FromMethodOn[methodJoinValue](env, "third", "Trade", schemaThree, newMethodJoinProvider(t, schemaThree, 2))
	query := JoinMany(JoinSource(first), JoinSource(second), JoinSource(third)).On(
		OnSourcesEqual(0, Field[methodJoinValue, int]("value"), 1, Field[methodJoinValue, int]("value")),
		OnSourcesEqual(1, Field[methodJoinValue, int]("value"), 2, Field[methodJoinValue, int]("value")),
	).Select(
		SelectFrom(0, "one", Field[methodJoinValue, int]("value")),
		SelectFrom(1, "two", Field[methodJoinValue, int]("value")),
		SelectFrom(2, "three", Field[methodJoinValue, int]("value")),
	).Query(StatementName("method-source-nstream"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())

	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "trigger"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("one").Any() != 2 || rows[0].Get("two").Any() != 2 || rows[0].Get("three").Any() != 2 {
		t.Fatalf("method N-stream rows = %#v", rows)
	}
}

func TestMethodSourceIndependentFullOuterJoinEmitsNullSides(t *testing.T) {
	env, engine := newRuntimeTest(t)
	leftSchema, err := StructSchema[methodJoinValue]("MethodOuterLeft")
	if err != nil {
		t.Fatal(err)
	}
	rightSchema, err := StructSchema[methodJoinValue]("MethodOuterRight")
	if err != nil {
		t.Fatal(err)
	}
	left := FromMethodOn[methodJoinValue](env, "outer-left", "Trade", leftSchema, newMethodJoinProvider(t, leftSchema, 1, 2))
	right := FromMethodOn[methodJoinValue](env, "outer-right", "Trade", rightSchema, newMethodJoinProvider(t, rightSchema, 2, 3))
	query := Join(left, right, OnEqual(
		Field[methodJoinValue, int]("value"),
		Field[methodJoinValue, int]("value"),
	)).FullOuter().Select(
		SelectLeft("leftValue", Field[methodJoinValue, int]("value")),
		SelectRight("rightValue", Field[methodJoinValue, int]("value")),
	).Query(StatementName("method-source-outer"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	if _, err := deployment.Statements()[0].Subscribe(func(context.Context, ResultBatch) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "trigger"}); err != nil {
		t.Fatal(err)
	}

	result, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 3 {
		t.Fatalf("method outer snapshot = %#v", result.Results())
	}
	seen := map[string]bool{}
	for _, item := range result.Results() {
		row, ok := item.Row()
		if !ok {
			t.Fatalf("method outer result = %#v", item)
		}
		leftValue, rightValue := row.Get("leftValue"), row.Get("rightValue")
		switch {
		case leftValue.Any() == 1 && rightValue.IsNull():
			seen["1/null"] = true
		case leftValue.Any() == 2 && rightValue.Any() == 2:
			seen["2/2"] = true
		case leftValue.IsNull() && rightValue.Any() == 3:
			seen["null/3"] = true
		}
	}
	if len(seen) != 3 {
		t.Fatalf("method outer rows = %#v", result.Results())
	}
}
