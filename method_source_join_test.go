package esper

import (
	"context"
	"strings"
	"testing"
)

type methodJoinValue struct {
	Value int `esper:"value"`
}

type methodDependentValue struct {
	Value int `esper:"value"`
	Group int `esper:"group"`
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

func TestMethodSourceDependentChainUsesTopologyAndLineage(t *testing.T) {
	env, engine := newRuntimeTest(t)
	firstSchema, err := StructSchema[methodDependentValue]("MethodDependentFirst")
	if err != nil {
		t.Fatal(err)
	}
	secondSchema, err := StructSchema[methodDependentValue]("MethodDependentSecond")
	if err != nil {
		t.Fatal(err)
	}
	thirdSchema, err := StructSchema[methodDependentValue]("MethodDependentThird")
	if err != nil {
		t.Fatal(err)
	}

	firstCalls, secondCalls, thirdCalls := 0, 0, 0
	first := FromMethodOn[methodDependentValue](env, "first", "Trade", firstSchema, MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		firstCalls++
		result := make([]Event, 0, 2)
		// Deliberately return duplicate values. Lateral lineage is based on
		// row identity, not reflect.DeepEqual event contents.
		for _, value := range []int{1, 1} {
			event, eventErr := newEvent(firstSchema, methodDependentValue{Value: value, Group: 1}, request.Now)
			if eventErr != nil {
				return nil, eventErr
			}
			result = append(result, event)
		}
		return result, nil
	}))
	second := FromMethodOn[methodDependentValue](env, "second", "Trade", secondSchema, MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		secondCalls++
		parent, ok := request.Dependency("first")
		if !ok {
			t.Fatal("second method did not receive first dependency")
		}
		if parent.Underlying().(methodDependentValue).Value != 1 {
			t.Fatalf("second dependency = %#v", parent.Underlying())
		}
		value := secondCalls * 10
		delete(request.Dependencies, "first") // The request map is a defensive snapshot.
		event, eventErr := newEvent(secondSchema, methodDependentValue{Value: value, Group: 1}, request.Now)
		return []Event{event}, eventErr
	})).DependingOn("first")
	third := FromMethodOn[methodDependentValue](env, "third", "Trade", thirdSchema, MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		thirdCalls++
		parent, ok := request.Dependency("second")
		if !ok {
			t.Fatal("third method did not receive second dependency")
		}
		value := parent.Underlying().(methodDependentValue).Value * 10
		event, eventErr := newEvent(thirdSchema, methodDependentValue{Value: value, Group: 1}, request.Now)
		return []Event{event}, eventErr
	})).DependingOn("second")

	// Declare the sources in reverse dependency order. Join indexes and
	// projections retain this order while provider polling follows first ->
	// second -> third.
	query := JoinMany(JoinSource(third), JoinSource(second), JoinSource(first)).On(
		OnSourcesEqual(0, Field[methodDependentValue, int]("group"), 1, Field[methodDependentValue, int]("group")),
		OnSourcesEqual(1, Field[methodDependentValue, int]("group"), 2, Field[methodDependentValue, int]("group")),
	).Select(
		SelectFrom(0, "third", Field[methodDependentValue, int]("value")),
		SelectFrom(1, "second", Field[methodDependentValue, int]("value")),
		SelectFrom(2, "first", Field[methodDependentValue, int]("value")),
	).Query(StatementName("method-dependent-chain"))
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
	if firstCalls != 1 || secondCalls != 2 || thirdCalls != 2 {
		t.Fatalf("method calls = first:%d second:%d third:%d", firstCalls, secondCalls, thirdCalls)
	}

	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 2 {
		t.Fatalf("dependent rows = %#v, want 2 lineage-preserving rows", snapshot.Results())
	}
	seen := map[string]bool{}
	for _, result := range snapshot.Results() {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("dependent result = %#v", result)
		}
		key := row.Get("first").String() + "/" + row.Get("second").String() + "/" + row.Get("third").String()
		seen[key] = true
	}
	if !seen["1/10/100"] || !seen["1/20/200"] || len(seen) != 2 {
		t.Fatalf("dependent lineage rows = %#v", seen)
	}
}

func TestMethodSourceDependentLeftOuterPreservesMissingResult(t *testing.T) {
	env, engine := newRuntimeTest(t)
	leftSchema, err := StructSchema[methodDependentValue]("MethodDependentOuterLeft")
	if err != nil {
		t.Fatal(err)
	}
	rightSchema, err := StructSchema[methodDependentValue]("MethodDependentOuterRight")
	if err != nil {
		t.Fatal(err)
	}
	left := FromMethodOn[methodDependentValue](env, "left-method", "Trade", leftSchema, MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		var result []Event
		for _, value := range []int{1, 2} {
			event, eventErr := newEvent(leftSchema, methodDependentValue{Value: value, Group: 1}, request.Now)
			if eventErr != nil {
				return nil, eventErr
			}
			result = append(result, event)
		}
		return result, nil
	}))
	right := FromMethodOn[methodDependentValue](env, "right-method", "Trade", rightSchema, MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		parent, _ := request.Dependency("left-method")
		value := parent.Underlying().(methodDependentValue).Value
		if value == 2 {
			return nil, nil
		}
		event, eventErr := newEvent(rightSchema, methodDependentValue{Value: value * 10, Group: 1}, request.Now)
		return []Event{event}, eventErr
	})).DependingOn("left-method")
	query := Join(left, right, OnEqual(
		Field[methodDependentValue, int]("group"),
		Field[methodDependentValue, int]("group"),
	)).LeftOuter().Select(
		SelectLeft("left", Field[methodDependentValue, int]("value")),
		SelectRight("right", Field[methodDependentValue, int]("value")),
	).Query(StatementName("method-dependent-left-outer"))
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
	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 2 {
		t.Fatalf("dependent outer rows = %#v", snapshot.Results())
	}
	foundMatched, foundMissing := false, false
	for _, result := range snapshot.Results() {
		row, _ := result.Row()
		switch {
		case row.Get("left").Any() == 1 && row.Get("right").Any() == 10:
			foundMatched = true
		case row.Get("left").Any() == 2 && row.Get("right").IsNull():
			foundMissing = true
		}
	}
	if !foundMatched || !foundMissing {
		t.Fatalf("dependent outer rows = %#v", snapshot.Results())
	}
}

func TestMethodSourceDependencyValidationAndPlanIdentity(t *testing.T) {
	env, _ := newRuntimeTest(t)
	schema, err := StructSchema[methodDependentValue]("MethodDependentValidation")
	if err != nil {
		t.Fatal(err)
	}
	provider := MethodProviderFunc(func(context.Context, MethodRequest) ([]Event, error) { return nil, nil })
	makeMethod := func(name string) Stream[methodDependentValue] {
		return FromMethodOn[methodDependentValue](env, name, "Trade", schema, provider)
	}
	condition := OnEqual(Field[methodDependentValue, int]("group"), Field[methodDependentValue, int]("group"))
	assertBuildError := func(label string, query Query, contains string) {
		t.Helper()
		_, buildErr := env.Build(query)
		if buildErr == nil || !strings.Contains(buildErr.Error(), contains) {
			t.Fatalf("%s error = %v, want containing %q", label, buildErr, contains)
		}
	}

	assertBuildError("standalone", makeMethod("standalone").DependingOn("other").Query(), "require a join")
	assertBuildError("non-method", From[runtimeTestTrade](env, "Trade").DependingOn("other").Query(), "requires a method source")
	assertBuildError("unknown", Join(makeMethod("known"), makeMethod("dependent").DependingOn("missing"), condition).Query(), "unknown dependency")
	assertBuildError("blank", Join(makeMethod("blank-base"), makeMethod("blank-dependent").DependingOn(" "), condition).Query(), "blank dependency")
	assertBuildError("duplicate", Join(makeMethod("base"), makeMethod("duplicate").DependingOn("base", "base"), condition).Query(), "duplicates dependency")
	assertBuildError("self", Join(makeMethod("self").DependingOn("self"), makeMethod("peer"), condition).Query(), "cannot depend on itself")
	assertBuildError("cycle", Join(makeMethod("cycle-a").DependingOn("cycle-b"), makeMethod("cycle-b").DependingOn("cycle-a"), condition).Query(), "contain a cycle")
	assertBuildError("ambiguous", JoinMany(
		JoinSource(makeMethod("shared")),
		JoinSource(makeMethod("shared")),
		JoinSource(makeMethod("ambiguous-dependent").DependingOn("shared")),
	).On(OnSourcesEqual(0, Field[methodDependentValue, int]("group"), 1, Field[methodDependentValue, int]("group"))).Query(), "is ambiguous")
	assertBuildError("left outer direction", Join(makeMethod("outer-a").DependingOn("outer-b"), makeMethod("outer-b"), condition).LeftOuter().Query(), "cannot be satisfied")
	assertBuildError("full outer", Join(makeMethod("full-a"), makeMethod("full-b").DependingOn("full-a"), condition).FullOuter().Query(), "cannot be guaranteed")

	base := makeMethod("hash-base")
	plainPlan, err := env.Build(Join(base, makeMethod("hash-dependent"), condition).Select(
		SelectLeft("value", Field[methodDependentValue, int]("value")),
	).Query(StatementName("method-dependency-plan")))
	if err != nil {
		t.Fatal(err)
	}
	dependentPlan, err := env.Build(Join(base, makeMethod("hash-dependent").DependingOn("hash-base"), condition).Select(
		SelectLeft("value", Field[methodDependentValue, int]("value")),
	).Query(StatementName("method-dependency-plan")))
	if err != nil {
		t.Fatal(err)
	}
	if plainPlan.Hash() == dependentPlan.Hash() {
		t.Fatal("method dependencies did not change canonical plan identity")
	}
}
