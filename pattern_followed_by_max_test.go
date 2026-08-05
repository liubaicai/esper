package esper

import (
	"context"
	"testing"
)

func TestPatternFollowedByMaxLimitsActiveSubexpressionsMatchesEsper(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	symbol := func(want string) Expression[bool] {
		return StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal(want))
	}
	pattern := PatternFrom(base, "a", symbol("A")).Every().
		FollowedBy("b", symbol("B")).
		FollowedByMax(2, "c", symbol("C"))
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "symbol")),
		Alias("b", TagField[string]("b", "symbol")),
		Alias("c", TagField[string]("c", "symbol")),
	).Query(StatementName("pattern-followed-by-max")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("followed-by max result = %#v, want Row", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, event := range []runtimeTestTrade{
		{Symbol: "A1"}, {Symbol: "A2"}, {Symbol: "A3"},
		{Symbol: "B"}, {Symbol: "C"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("nested followed-by max rows = %#v, want two", rows)
	}
	assertPatternFollowedByMaxRow(t, rows[0], "A1", "B", "C")
	assertPatternFollowedByMaxRow(t, rows[1], "A2", "B", "C")

	// The completed B->C subexpressions release capacity. A later batch may
	// therefore accept two new A starts, while the third is again rejected.
	for _, event := range []runtimeTestTrade{
		{Symbol: "A4"}, {Symbol: "A5"}, {Symbol: "A6"},
		{Symbol: "B"}, {Symbol: "C"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 4 {
		t.Fatalf("followed-by max capacity reuse rows = %#v, want four", rows)
	}
	assertPatternFollowedByMaxRow(t, rows[2], "A4", "B", "C")
	assertPatternFollowedByMaxRow(t, rows[3], "A5", "B", "C")
}

func TestPatternFollowedByMaxSimpleMatchesEsper(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal("A"))).Every().
		FollowedByMax(2, "b", StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal("B")))
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "symbol")),
		Alias("b", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-followed-by-max-simple")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("simple followed-by max result = %#v, want Row", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A1"}, {Symbol: "A2"}, {Symbol: "A3"}, {Symbol: "B1"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 || rows[0].Get("a").Any() != "A1" || rows[1].Get("a").Any() != "A2" {
		t.Fatalf("simple followed-by max rows = %#v, want A1/A2 only", rows)
	}
}

func TestPatternFollowedByMaxRejectsNonPositiveMaximum(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", Literal[bool](true)).FollowedByMax(0, "b", Literal[bool](true))
	if _, err := env.Build(pattern.Select(Alias("a", TagField[string]("a", "symbol"))).Query()); err == nil {
		t.Fatal("non-positive followed-by maximum was accepted")
	}
}

func TestPatternFollowedByMaxExpressionUsesVariableAndParameter(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if err := env.RegisterVariable("edgeMax", 2); err != nil {
		t.Fatal(err)
	}
	base := From[runtimeTestTrade](env, "Trade")
	symbol := func(want string) Expression[bool] {
		return StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal(want))
	}
	variablePattern := PatternFrom(base, "a", symbol("A")).Every().FollowedByMaxExpr(
		VariableRef[int]("edgeMax"), "b", symbol("B"),
	)
	variablePlan, err := env.Build(variablePattern.Select(
		Alias("a", TagField[string]("a", "symbol")),
		Alias("b", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-followed-by-max-variable")))
	if err != nil {
		t.Fatal(err)
	}
	if variablePlan.Query().pattern.root.sequenceMaxExpr == nil {
		t.Fatal("variable followed-by maximum expression was not retained")
	}
	engine := NewEngine(env)
	variableDeployment, err := engine.Deploy(context.Background(), variablePlan)
	if err != nil {
		t.Fatal(err)
	}
	defer variableDeployment.Undeploy(context.Background())
	var variableRows []Row
	if _, err := variableDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				variableRows = append(variableRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A1"}, {Symbol: "A2"}, {Symbol: "A3"}, {Symbol: "B1"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(variableRows) != 2 || variableRows[0].Get("a").Any() != "A1" || variableRows[1].Get("a").Any() != "A2" {
		t.Fatalf("variable followed-by maximum rows = %#v, want A1/A2", variableRows)
	}

	parameterPattern := PatternFrom(base, "a", symbol("P")).Every().FollowedByMaxExpr(
		Parameter[int]("edgeMaxParam"), "b", symbol("Q"),
	)
	parameterPlan, err := env.Build(parameterPattern.Select(
		Alias("a", TagField[string]("a", "symbol")),
		Alias("b", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-followed-by-max-parameter")))
	if err != nil {
		t.Fatal(err)
	}
	parameterDeployment, err := engine.DeployWithParameters(context.Background(), parameterPlan, ParameterValues{"edgeMaxParam": 2})
	if err != nil {
		t.Fatal(err)
	}
	defer parameterDeployment.Undeploy(context.Background())
	var parameterRows []Row
	if _, err := parameterDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				parameterRows = append(parameterRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "P1"}, {Symbol: "P2"}, {Symbol: "P3"}, {Symbol: "Q1"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(parameterRows) != 2 || parameterRows[0].Get("a").Any() != "P1" || parameterRows[1].Get("a").Any() != "P2" {
		t.Fatalf("parameter followed-by maximum rows = %#v, want P1/P2", parameterRows)
	}
}

func TestPatternFollowedByMaxExpressionRejectsInvalidFieldAndRuntimeValue(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	fieldMaximum := PatternFrom(base, "a", Literal[bool](true)).FollowedByMaxExpr(
		Cast[float64, int](Field[runtimeTestTrade, float64]("price")), "b", Literal[bool](true),
	)
	if _, err := env.Build(fieldMaximum.Select(Alias("a", TagField[string]("a", "symbol"))).Query(StatementName("invalid-followed-by-max-field"))); err == nil {
		t.Fatal("followed-by maximum event field was accepted")
	}
	if err := env.RegisterVariable("edgeMax", 2); err != nil {
		t.Fatal(err)
	}
	runtimeMaximum := PatternFrom(base, "a", Literal[bool](true)).Every().FollowedByMaxExpr(
		VariableRef[int]("edgeMax"), "b", Literal[bool](true),
	)
	plan, err := env.Build(runtimeMaximum.Select(Alias("a", TagField[string]("a", "symbol"))).Query(StatementName("invalid-followed-by-max-runtime")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.SetVariable(context.Background(), "edgeMax", 0); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Result
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows = append(rows, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("invalid runtime followed-by maximum emitted rows = %#v", rows)
	}
}

func assertPatternFollowedByMaxRow(t *testing.T, row Row, wantA, wantB, wantC string) {
	t.Helper()
	if got := row.Get("a").Any(); got != wantA {
		t.Fatalf("followed-by max a = %#v, want %q", got, wantA)
	}
	if got := row.Get("b").Any(); got != wantB {
		t.Fatalf("followed-by max b = %#v, want %q", got, wantB)
	}
	if got := row.Get("c").Any(); got != wantC {
		t.Fatalf("followed-by max c = %#v, want %q", got, wantC)
	}
}
