package esper

import (
	"context"
	"testing"
	"time"
)

func TestPatternFollowedByEveryAndWithin(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A"))).
		FollowedBy("b", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("B"))).
		Every().Within(time.Second)
	query := pattern.Select(
		Alias("first", TagField[string]("a", "symbol")),
		Alias("second", TagField[string]("b", "symbol")),
	).Query(StatementName("followed-by"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A"}, {Symbol: "B"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("pattern batches = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("first").Any() != "A" || row.Get("second").Any() != "B" {
		t.Fatalf("pattern result = %#v", batches[0].New)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("expired pattern unexpectedly matched: %#v", batches)
	}
}

func TestPatternEveryDistinctAndMaxStates(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A"))).
		FollowedBy("b", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("B"))).
		EveryDistinct(Field[runtimeTestTrade, string]("symbol")).
		MaxStates(1)
	plan, err := env.Build(pattern.Select(
		Alias("symbol", TagField[string]("b", "symbol")),
	).Query(StatementName("every-distinct")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var matches int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		matches += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A"}, {Symbol: "A"}, {Symbol: "B"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if matches != 1 {
		t.Fatalf("every-distinct matches = %d, want 1", matches)
	}
}

func TestPatternEveryDistinctKeyExpiresOnVirtualClock(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	key := Field[runtimeTestTrade, string]("symbol")
	makePattern := func(pattern PatternStream) Query {
		return pattern.Select(Alias("symbol", TagField[string]("a", "symbol"))).Query()
	}
	withinPlan, err := env.Build(makePattern(
		PatternFrom(base, "a", Literal[bool](true)).EveryDistinct(key).Within(10 * time.Second),
	))
	if err != nil {
		t.Fatal(err)
	}
	expiryPlan, err := env.Build(makePattern(
		PatternFrom(base, "a", Literal[bool](true)).EveryDistinctFor(key, time.Second),
	))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(makePattern(
		PatternFrom(base, "a", Literal[bool](true)).EveryDistinctFor(key, 0),
	)); err == nil {
		t.Fatal("zero every-distinct expiry was accepted")
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	withinDeployment, err := engine.Deploy(context.Background(), withinPlan)
	if err != nil {
		t.Fatal(err)
	}
	expiryDeployment, err := engine.Deploy(context.Background(), expiryPlan)
	if err != nil {
		t.Fatal(err)
	}
	var withinRows, expiryRows []Row
	if _, err := withinDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				withinRows = append(withinRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := expiryDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				expiryRows = append(expiryRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(withinRows) != 1 || len(expiryRows) != 1 {
		t.Fatalf("initial distinct rows within=%#v expiry=%#v", withinRows, expiryRows)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(expiryRows) != 2 || len(withinRows) != 1 {
		t.Fatalf("explicit/within expiry rows within=%#v expiry=%#v", withinRows, expiryRows)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(10, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(withinRows) != 1 {
		t.Fatalf("outer within rows = %#v, want the pattern to remain terminated", withinRows)
	}
}

func TestPatternEveryDistinctInsideWithinRetainsDistinctState(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	key := Field[runtimeTestTrade, string]("symbol")
	plan, err := env.Build(PatternFrom(base, "a", Literal[bool](true)).Within(10 * time.Second).EveryDistinct(key).Select(
		Alias("symbol", TagField[string]("a", "symbol")),
	).Query())
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"A", "A", "B", "A", "B"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("inner within distinct rows = %#v, want one row per key", rows)
	}
	for index, want := range []string{"A", "B"} {
		value := rows[index].Get("symbol")
		if !value.IsPresent() || value.Any() != want {
			t.Fatalf("row %d symbol = %#v, want %q", index, value, want)
		}
	}
}

func TestPatternEveryDistinctForInsideWithinExpiresNodeState(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	key := Field[runtimeTestTrade, string]("symbol")
	plan, err := env.Build(PatternFrom(base, "a", Literal[bool](true)).Within(10*time.Second).EveryDistinctFor(key, time.Second).Select(
		Alias("symbol", TagField[string]("a", "symbol")),
	).Query())
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"A", "B"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(0, 500*int64(time.Millisecond)).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("duplicate inner distinct rows = %#v, want two initial keys", rows)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(0, 1100*int64(time.Millisecond)).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("inner distinct expiry rows = %#v, want A to reopen", rows)
	}
}

func TestPatternAndMatchesBranchesAcrossEvents(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	left := PatternFrom(base, "a", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	right := PatternFrom(base, "b", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("B")))
	plan, err := env.Build(left.And(right).Select(
		Alias("left", TagField[string]("a", "symbol")),
		Alias("right", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-and")))
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
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A"}, {Symbol: "B"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 1 || rows[0].Get("left").Any() != "A" || rows[0].Get("right").Any() != "B" {
		t.Fatalf("and rows = %#v", rows)
	}
}

func TestPatternAndAllowsSiblingTagReuse(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	left := PatternFrom(base, "same", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	right := PatternFrom(base, "same", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("B")))
	pattern := left.And(right)
	plan, err := env.Build(pattern.Select(
		Alias("last", TagField[string]("same", "symbol")),
		Alias("count", TagCount("same")),
	).Query(StatementName("pattern-and-reused-tag")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var result Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			result, _ = batch.New[0].Row()
		}
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
	if result.Get("last").Any() != "B" || result.Get("count").Any() != int64(2) {
		t.Fatalf("reused sibling tag result = %#v", result)
	}
}

func TestPatternAndSupportsEveryBranch(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	left := PatternFrom(base, "a", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	right := PatternFrom(base, "b", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("B"))).Every()
	plan, err := env.Build(left.And(right).Select(
		Alias("left", TagField[string]("a", "symbol")),
		Alias("right", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-and-every-branch")))
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
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "B"}, {Symbol: "B"}, {Symbol: "A"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 1 || rows[0].Get("left").Any() != "A" || rows[0].Get("right").Any() != "B" {
		t.Fatalf("every branch rows = %#v", rows)
	}
}

func TestPatternOrCompletesEitherBranch(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	left := PatternFrom(base, "a", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	right := PatternFrom(base, "b", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("B")))
	plan, err := env.Build(left.Or(right).Every().Select(
		Alias("symbol", Coalesce[string](TagField[string]("a", "symbol"), TagField[string]("b", "symbol"))),
	).Query(StatementName("pattern-or")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var symbols []any
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				symbols = append(symbols, row.Get("symbol").Any())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A"}, {Symbol: "B"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(symbols) != 2 || symbols[0] != "A" || symbols[1] != "B" {
		t.Fatalf("or symbols = %#v", symbols)
	}
}

func TestPatternNotSuppressesNegativeBranch(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	positive := PatternFrom(base, "a", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	negative := PatternFrom(base, "b", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("B")))
	plan, err := env.Build(positive.And(negative.Not()).Select(
		Alias("symbol", TagField[string]("a", "symbol")),
	).Query(StatementName("pattern-not")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var matches int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		matches += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if matches != 0 {
		t.Fatalf("negative branch matches = %d, want 0", matches)
	}
}

func TestPatternMatchUntilCountsRepeats(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "tick", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A"))).MatchUntil(2, 3)
	plan, err := env.Build(pattern.Select(
		Alias("count", TagCount("tick")),
		Alias("last", TagField[string]("tick", "symbol")),
	).Query(StatementName("pattern-match-until")))
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
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A"}, {Symbol: "C"}, {Symbol: "A"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 1 || rows[0].Get("count").Any() != int64(2) || rows[0].Get("last").Any() != "A" {
		t.Fatalf("match-until rows = %#v", rows)
	}

	invalid := PatternFrom(base, "tick", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A"))).MatchUntil(3, 2)
	if _, err := env.Build(invalid.Select(Alias("count", TagCount("tick"))).Query(StatementName("invalid-match-until"))); err == nil {
		t.Fatal("expected invalid match-until bounds")
	}
}

func TestPatternMatchUntilExpressionBoundsUseVariablesAndParameters(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if err := env.RegisterVariable("lower", 2); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("upper", 3); err != nil {
		t.Fatal(err)
	}
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "tick", Literal(true)).MatchUntilExpr(
		VariableRef[int]("lower"),
		VariableRef[int]("upper"),
	)
	plan, err := env.Build(pattern.Select(
		Alias("count", TagCount("tick")),
		Alias("first", TagFieldAt[string]("tick", 0, "symbol")),
		Alias("last", TagField[string]("tick", "symbol")),
	).Query(StatementName("pattern-match-until-expression-bounds")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Query().pattern.root.minimumExpr == nil || plan.Query().pattern.root.maximumExpr == nil {
		t.Fatal("dynamic match-until bounds were not retained")
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"A1", "A2"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 1 || rows[0].Get("count").Any() != int64(2) || rows[0].Get("first").Any() != "A1" || rows[0].Get("last").Any() != "A2" {
		t.Fatalf("variable match-until rows = %#v, want count=2 A1/A2", rows)
	}

	parameterPattern := PatternFrom(base, "tick", Literal(true)).MatchUntilExpr(
		Parameter[int]("lowerParam"),
		Parameter[int]("upperParam"),
	)
	parameterPlan, err := env.Build(parameterPattern.Select(
		Alias("count", TagCount("tick")),
	).Query(StatementName("pattern-match-until-parameter-bounds")))
	if err != nil {
		t.Fatal(err)
	}
	parameterDeployment, err := engine.DeployWithParameters(context.Background(), parameterPlan, ParameterValues{
		"lowerParam": 2,
		"upperParam": 2,
	})
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
	for _, symbol := range []string{"P1", "P2"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(parameterRows) != 1 || parameterRows[0].Get("count").Any() != int64(2) {
		t.Fatalf("parameter match-until rows = %#v, want count=2", parameterRows)
	}

	openPattern := PatternFrom(base, "tick", Literal(true)).MatchUntilExpr(
		Parameter[int]("lowerOpen"),
		Parameter[int]("upperOpen"),
	)
	openPlan, err := env.Build(openPattern.Select(
		Alias("count", TagCount("tick")),
	).Query(StatementName("pattern-match-until-open-parameter-bounds")))
	if err != nil {
		t.Fatal(err)
	}
	openDeployment, err := engine.DeployWithParameters(context.Background(), openPlan, ParameterValues{
		"lowerOpen": 3,
		"upperOpen": nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer openDeployment.Undeploy(context.Background())
	var openRows []Row
	if _, err := openDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				openRows = append(openRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"O1", "O2", "O3"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(openRows) != 1 || openRows[0].Get("count").Any() != int64(3) {
		t.Fatalf("open parameter match-until rows = %#v, want count=3", openRows)
	}
}

func TestPatternMatchUntilExpressionBoundsRejectInvalidRuntimeValues(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if err := env.RegisterVariable("lower", 3); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("upper", 2); err != nil {
		t.Fatal(err)
	}
	base := From[runtimeTestTrade](env, "Trade")
	plan, err := env.Build(PatternFrom(base, "tick", Literal(true)).MatchUntilExpr(
		VariableRef[int]("lower"),
		VariableRef[int]("upper"),
	).Select(Alias("count", TagCount("tick"))).Query(StatementName("pattern-match-until-invalid-runtime-bounds")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
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
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "invalid"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("invalid dynamic match-until bounds emitted rows = %#v", rows)
	}
}

func TestPatternMatchUntilExpressionBoundsUseCapturedTag(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	bound := PatternFrom(base, "bound", Literal(true))
	repeated := PatternFrom(base, "tick", Literal(true)).MatchUntilExpr(
		Cast[float64, int](TagField[float64]("bound", "price")),
		nil,
	)
	plan, err := env.Build(bound.Then(repeated).Select(
		Alias("count", TagCount("tick")),
		Alias("bound", TagField[string]("bound", "symbol")),
	).Query(StatementName("pattern-match-until-captured-tag-bounds")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "bound", Price: 2}, {Symbol: "A"}, {Symbol: "B"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 1 || rows[0].Get("count").Any() != int64(2) || rows[0].Get("bound").Any() != "bound" {
		t.Fatalf("captured-tag match-until rows = %#v, want count=2", rows)
	}
}

func TestPatternUntilRetainsRepeatedTagsAndTerminator(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	repeated := PatternFrom(base, "a", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	terminator := PatternFrom(base, "b", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("B")))
	plan, err := env.Build(repeated.Until(terminator).Select(
		Alias("count", TagCount("a")),
		Alias("first", TagFieldAt[string]("a", 0, "symbol")),
		Alias("last", TagField[string]("a", "symbol")),
		Alias("terminator", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-until")))
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
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A"}, {Symbol: "A"}, {Symbol: "C"}, {Symbol: "B"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 1 || rows[0].Get("count").Any() != int64(2) || rows[0].Get("first").Any() != "A" || rows[0].Get("last").Any() != "A" || rows[0].Get("terminator").Any() != "B" {
		t.Fatalf("until rows = %#v", rows)
	}
}

func TestPatternTimerIntervalAndAtUseVirtualClock(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	intervalPlan, err := env.Build(TimerInterval(base, time.Second).Select(
		Alias("now", CurrentTime()),
	).Query(StatementName("pattern-timer-interval")))
	if err != nil {
		t.Fatal(err)
	}
	at := time.Unix(3, 0).UTC()
	atPlan, err := env.Build(TimerAt(base, at).Select(
		Alias("now", CurrentTime()),
	).Query(StatementName("pattern-timer-at")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine(WithStartTime(time.Unix(0, 0).UTC()))
	intervalDeployment, err := engine.Deploy(context.Background(), intervalPlan)
	if err != nil {
		t.Fatal(err)
	}
	atDeployment, err := engine.Deploy(context.Background(), atPlan)
	if err != nil {
		t.Fatal(err)
	}
	var intervalRows []Row
	var atRows []Row
	if _, err := intervalDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				intervalRows = append(intervalRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := atDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				atRows = append(atRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), at); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(4, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(intervalRows) != 4 || len(atRows) != 1 {
		t.Fatalf("timer rows interval=%#v at=%#v", intervalRows, atRows)
	}
	if intervalRows[0].Get("now").Any() != time.Unix(1, 0).UTC() || atRows[0].Get("now").Any() != at {
		t.Fatalf("timer values interval=%#v at=%#v", intervalRows, atRows)
	}
	if _, err := env.Build(TimerInterval(base, 0).Select(Alias("now", CurrentTime())).Query(StatementName("invalid-timer"))); err == nil {
		t.Fatal("zero timer interval was accepted")
	}
}

func TestPatternTimerIntervalExpressionUsesCapturedTag(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", Literal[bool](true)).Then(
		TimerIntervalExpr(base, DurationSeconds[float64](TagField[float64]("a", "price"))),
	)
	plan, err := env.Build(pattern.Select(
		Alias("symbol", TagField[string]("a", "symbol")),
		Alias("firedAt", CurrentTime()),
	).Query(StatementName("pattern-timer-interval-expression")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 999999999).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("dynamic timer fired before captured deadline: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(3, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("symbol").Any() != "A" || rows[0].Get("firedAt").Any() != time.Unix(3, 0).UTC() {
		t.Fatalf("dynamic timer rows = %#v", rows)
	}
}

func TestPatternTimerIntervalExpressionUsesRepeatedTagValues(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	repeated := PatternFrom(base, "a", Literal[bool](true)).MatchUntil(2, 2)
	duration := Add[float64](
		TagFieldAt[float64]("a", 0, "price"),
		TagFieldAt[float64]("a", 1, "price"),
	)
	pattern := repeated.Then(TimerIntervalExpr(base, DurationSeconds[float64](duration)))
	plan, err := env.Build(pattern.Select(
		Alias("first", TagFieldAt[string]("a", 0, "symbol")),
		Alias("second", TagFieldAt[string]("a", 1, "symbol")),
		Alias("firedAt", CurrentTime()),
	).Query(StatementName("pattern-timer-interval-repeated-tags")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A1", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A2", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(4, 999999999).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("repeated-tag timer fired before summed deadline: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(5, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("first").Any() != "A1" || rows[0].Get("second").Any() != "A2" || rows[0].Get("firedAt").Any() != time.Unix(5, 0).UTC() {
		t.Fatalf("repeated-tag timer rows = %#v", rows)
	}
}

func TestPatternTimerIntervalExpressionUsesComponentParameters(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	duration := DurationSum(
		DurationDays[float64](Parameter[float64]("days")),
		DurationHours[float64](Parameter[float64]("hours")),
		DurationMinutes[float64](Parameter[float64]("minutes")),
		DurationSeconds[float64](Parameter[float64]("seconds")),
		DurationMilliseconds[float64](Parameter[float64]("milliseconds")),
	)
	plan, err := env.Build(TimerIntervalExpr(base, duration).Select(
		Alias("firedAt", CurrentTime()),
	).Query(StatementName("pattern-timer-interval-components")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	if _, err := engine.Deploy(context.Background(), plan); err == nil {
		t.Fatal("component-parameterized timer deployed without bindings")
	}
	parameters := ParameterValues{
		"days":         1.0,
		"hours":        2.0,
		"minutes":      3.0,
		"seconds":      4.0,
		"milliseconds": 5.0,
	}
	deployment, err := engine.DeployWithParameters(context.Background(), plan, parameters)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := time.Unix(0, 0).UTC().Add(24*time.Hour + 2*time.Hour + 3*time.Minute + 4*time.Second + 5*time.Millisecond)
	if err := engine.AdvanceTime(context.Background(), want.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("component timer fired before deadline: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("firedAt").Any() != want {
		t.Fatalf("component timer rows = %#v", rows)
	}
}

func TestPatternTimerIntervalExpressionUsesVariables(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if err := env.RegisterVariable("timerMinutes", 1.0); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("timerSeconds", 2.0); err != nil {
		t.Fatal(err)
	}
	base := From[runtimeTestTrade](env, "Trade")
	duration := DurationSeconds[float64](Add[float64](
		Multiply[float64](VariableRef[float64]("timerMinutes"), Literal[float64](60)),
		VariableRef[float64]("timerSeconds"),
	))
	plan, err := env.Build(TimerIntervalExpr(base, duration).Select(
		Alias("firedAt", CurrentTime()),
	).Query(StatementName("pattern-timer-interval-variables")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := time.Unix(62, 0).UTC()
	if err := engine.AdvanceTime(context.Background(), want.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("variable timer fired before deadline: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("firedAt").Any() != want {
		t.Fatalf("variable timer rows = %#v", rows)
	}
}

func TestPatternTimerIntervalMicrosecondPrecision(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	plan, err := env.Build(TimerIntervalExpr(base, DurationMicroseconds[float64](Literal[float64](1))).Select(
		Alias("firedAt", CurrentTime()),
	).Query(StatementName("pattern-timer-interval-microsecond")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(0, int64(time.Microsecond-time.Nanosecond)).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("microsecond timer fired early: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(0, int64(time.Microsecond)).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("firedAt").Any() != time.Unix(0, int64(time.Microsecond)).UTC() {
		t.Fatalf("microsecond timer rows = %#v", rows)
	}
}

func TestPatternTimerIntervalCalendarUsesMonthRecurrence(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := time.Date(2002, time.February, 1, 9, 0, 0, 0, time.UTC)
	plan, err := env.Build(TimerIntervalCalendar(base, 0, 1, 0).Select(
		Alias("scheduled", CurrentTime()),
	).Query(StatementName("pattern-timer-interval-calendar")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(start))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	first := start.AddDate(0, 1, 0)
	second := first.AddDate(0, 1, 0)
	if err := engine.AdvanceTime(context.Background(), first.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("calendar timer fired before first month boundary: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	want := []time.Time{first, second}
	if len(rows) != len(want) {
		t.Fatalf("calendar timer rows = %#v", rows)
	}
	for index, expected := range want {
		if got := rows[index].Get("scheduled").Any(); got != expected {
			t.Fatalf("calendar timer row %d = %#v, want %s", index, got, expected)
		}
	}
	if _, err := env.Build(TimerIntervalCalendar(base, -1, 0, 0).Select(Alias("scheduled", CurrentTime())).Query(StatementName("invalid-calendar-timer"))); err == nil {
		t.Fatal("negative calendar timer interval was accepted")
	}
}

func TestPatternTimerScheduleEmitsAllDueInstantsInOrder(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	first := time.Unix(1, 0).UTC()
	second := time.Unix(3, 0).UTC()
	plan, err := env.Build(TimerSchedule(base, second, first).Select(
		Alias("scheduled", CurrentTime()),
	).Query(StatementName("pattern-timer-schedule")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine(WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(4, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Get("scheduled").Any() != first || rows[1].Get("scheduled").Any() != second {
		t.Fatalf("schedule rows = %#v", rows)
	}
	if _, err := env.Build(TimerSchedule(base).Select(Alias("scheduled", CurrentTime())).Query(StatementName("invalid-schedule"))); err == nil {
		t.Fatal("empty timer schedule was accepted")
	}
}

func TestPatternTimerAtScheduleEmitsNextCalendarOccurrenceOnly(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	schedule := CronSchedule{
		Minute:     CronRange(20, 20),
		Hour:       CronValues(17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	plan, err := env.Build(TimerAtSchedule(base, schedule).Select(
		Alias("scheduled", CurrentTime()),
	).Query(StatementName("pattern-timer-at-schedule")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine(WithStartTime(start))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2008, time.February, 1, 17, 20, 0, 0, time.UTC)
	if err := engine.AdvanceTime(context.Background(), want.Add(-time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("timer-at schedule fired before occurrence: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("scheduled").Any() != want {
		t.Fatalf("timer-at schedule rows = %#v, want one row at %s", rows, want)
	}
	if err := engine.AdvanceTime(context.Background(), want.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("one-shot timer-at schedule fired again: %#v", rows)
	}
}

func TestPatternTimerAtScheduleSupportsStepAndMilliseconds(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	schedule := NewCronScheduleWithMilliseconds(
		CronValues(200), CronValues(0), CronEvery(5), CronValues(8),
		CronWildcard(), CronWildcard(), CronWildcard(),
	)
	start := time.Date(2013, time.August, 23, 8, 5, 0, 0, time.UTC)
	plan, err := env.Build(TimerAtSchedule(base, schedule).Select(
		Alias("scheduled", CurrentTime()),
	).Query(StatementName("pattern-timer-at-schedule-precision")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine(WithStartTime(start))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := start.Add(200 * time.Millisecond)
	if err := engine.AdvanceTime(context.Background(), want.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("millisecond timer fired before occurrence: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("scheduled").Any() != want {
		t.Fatalf("precision timer rows = %#v, want one row at %s", rows, want)
	}
	if err := engine.AdvanceTime(context.Background(), want.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("precision one-shot timer fired again: %#v", rows)
	}
}

func TestPatternTimerCronEmitsCalendarOccurrences(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	schedule := CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	plan, err := env.Build(TimerCron(base, schedule).Select(
		Alias("scheduled", CurrentTime()),
	).Query(StatementName("pattern-timer-cron")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine(WithStartTime(start))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 46, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	want := []time.Time{
		time.Date(2008, time.February, 1, 17, 15, 0, 0, time.UTC),
		time.Date(2008, time.February, 1, 17, 30, 0, 0, time.UTC),
		time.Date(2008, time.February, 1, 17, 45, 0, 0, time.UTC),
	}
	if len(rows) != len(want) {
		t.Fatalf("cron timer rows = %#v", rows)
	}
	for index, expected := range want {
		if got := rows[index].Get("scheduled").Any(); got != expected {
			t.Fatalf("cron timer row %d = %#v, want %s", index, got, expected)
		}
	}
}

func TestPatternTimerObserverComposesWithEvents(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	positive := Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A"))
	pattern := TimerInterval(base, time.Second).FollowedBy("trade", positive)
	plan, err := env.Build(pattern.Select(
		Alias("symbol", TagField[string]("trade", "symbol")),
		Alias("matchedAt", CurrentTime()),
	).Query(StatementName("pattern-timer-followed-by")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("timer followed-by emitted before event: %#v", rows)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("symbol").Any() != "A" || rows[0].Get("matchedAt").Any() != time.Unix(1, 0).UTC() {
		t.Fatalf("timer followed-by rows = %#v", rows)
	}
}

func TestPatternTimerObserverAndEventCompletesOnClock(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	event := PatternFrom(base, "trade", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	pattern := TimerInterval(base, time.Second).And(event)
	plan, err := env.Build(pattern.Select(
		Alias("symbol", TagField[string]("trade", "symbol")),
		Alias("matchedAt", CurrentTime()),
	).Query(StatementName("pattern-timer-and")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("symbol").Any() != "A" || rows[0].Get("matchedAt").Any() != time.Unix(1, 0).UTC() {
		t.Fatalf("timer and rows = %#v", rows)
	}
}

func TestPatternEventThenTimerObserverCompletesOnClock(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	event := PatternFrom(base, "trade", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	pattern := event.Then(TimerInterval(base, time.Second))
	plan, err := env.Build(pattern.Select(
		Alias("symbol", TagField[string]("trade", "symbol")),
		Alias("matchedAt", CurrentTime()),
	).Query(StatementName("pattern-event-then-timer")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("event then timer emitted before clock: %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("symbol").Any() != "A" || rows[0].Get("matchedAt").Any() != time.Unix(1, 0).UTC() {
		t.Fatalf("event then timer rows = %#v", rows)
	}
}

func TestPatternWhileGuardClearsActiveMatches(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	positive := Greater[float64](Field[runtimeTestTrade, float64]("price"), Literal(0.0))
	pattern := PatternFrom(base, "a", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A"))).While(positive)
	plan, err := env.Build(pattern.Select(Alias("symbol", TagField[string]("a", "symbol"))).Query(StatementName("pattern-while")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var matches int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		matches += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: -1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if matches != 1 {
		t.Fatalf("while guard matches = %d, want 1", matches)
	}
}

func TestPatternWithinOrMaxScopesEveryBranchAndVirtualClock(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", Literal[bool](true)).Every().WithinOrMax(2*time.Second, 2)
	plan, err := env.Build(pattern.Select(
		Alias("symbol", TagField[string]("a", "symbol")),
		Alias("matchedAt", CurrentTime()),
	).Query(StatementName("pattern-within-or-max")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A"}, {Symbol: "B"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("within-or-max rows before deadline = %#v, want 2", rows)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "C"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("within-or-max emitted after max/deadline = %#v", rows)
	}
}

func TestPatternWithinOrMaxComposesWithSequenceAndAnd(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	a := PatternFrom(base, "a", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	b := PatternFrom(base, "b", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("B")))
	sequence := a.Then(b).WithinOrMax(time.Second, 1)
	seqPlan, err := env.Build(sequence.Select(
		Alias("first", TagField[string]("a", "symbol")),
		Alias("second", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-within-or-max-sequence")))
	if err != nil {
		t.Fatal(err)
	}
	andPlan, err := env.Build(a.And(b).WithinOrMax(time.Second, 1).Select(
		Alias("first", TagField[string]("a", "symbol")),
		Alias("second", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-within-or-max-and")))
	if err != nil {
		t.Fatal(err)
	}
	zeroPlan, err := env.Build(PatternFrom(base, "a", Literal[bool](true)).WithinOrMax(time.Second, 0).Select(
		Alias("symbol", TagField[string]("a", "symbol")),
	).Query(StatementName("pattern-within-or-max-zero")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	seqDeployment, err := engine.Deploy(context.Background(), seqPlan)
	if err != nil {
		t.Fatal(err)
	}
	andDeployment, err := engine.Deploy(context.Background(), andPlan)
	if err != nil {
		t.Fatal(err)
	}
	zeroDeployment, err := engine.Deploy(context.Background(), zeroPlan)
	if err != nil {
		t.Fatal(err)
	}
	var seqRows, andRows, zeroRows []Row
	collect := func(target *[]Row) Listener {
		return func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				if row, ok := result.Row(); ok {
					*target = append(*target, row)
				}
			}
			return nil
		}
	}
	if _, err := seqDeployment.Statements()[0].Subscribe(collect(&seqRows)); err != nil {
		t.Fatal(err)
	}
	if _, err := andDeployment.Statements()[0].Subscribe(collect(&andRows)); err != nil {
		t.Fatal(err)
	}
	if _, err := zeroDeployment.Statements()[0].Subscribe(collect(&zeroRows)); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(seqRows) != 1 || len(andRows) != 1 || len(zeroRows) != 0 {
		t.Fatalf("within-or-max composition rows: sequence=%#v and=%#v zero=%#v", seqRows, andRows, zeroRows)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(seqRows) != 1 || len(andRows) != 1 {
		t.Fatalf("within-or-max emitted after exact boundary: sequence=%#v and=%#v", seqRows, andRows)
	}
}

func TestPatternWithinExpressionUsesCapturedTagDeadline(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", Literal[bool](true)).Then(
		PatternFrom(base, "b", Literal[bool](true)).WithinExpr(
			DurationSeconds[float64](TagField[float64]("a", "price")),
		),
	)
	plan, err := env.Build(pattern.Select(
		Alias("first", TagField[string]("a", "symbol")),
		Alias("second", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-within-expression")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A1", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 999000000).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("first").Any() != "A1" || rows[0].Get("second").Any() != "B1" {
		t.Fatalf("dynamic within rows before deadline = %#v", rows)
	}

	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A2", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(5, 998999999).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B2"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("dynamic within rows just before deadline = %#v", rows)
	}

	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A3", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(8, 998999999).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B3"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("dynamic within matched at exact deadline = %#v", rows)
	}
}

func TestPatternWithinCalendarUsesMonthBoundary(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", Literal[bool](true)).Every().WithinCalendar(0, 1, 0)
	plan, err := env.Build(pattern.Select(
		Alias("symbol", TagField[string]("a", "symbol")),
	).Query(StatementName("pattern-within-calendar")))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2002, time.February, 1, 9, 0, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"E1"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := start.AddDate(0, 1, 0)
	if err := engine.AdvanceTime(context.Background(), deadline.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E2"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("calendar within rows before month boundary = %#v", rows)
	}
	if err := engine.AdvanceTime(context.Background(), deadline); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E3"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("calendar within matched at month boundary = %#v", rows)
	}
}

func TestPatternWithinExpressionUsesDeploymentParameter(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", Literal[bool](true)).Then(
		PatternFrom(base, "b", Literal[bool](true)).WithinExpr(
			DurationSeconds[float64](Parameter[float64]("seconds")),
		),
	)
	plan, err := env.Build(pattern.Select(
		Alias("first", TagField[string]("a", "symbol")),
		Alias("second", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-within-parameter")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	if _, err := engine.Deploy(context.Background(), plan); err == nil {
		t.Fatal("parameterized pattern deployed without bindings")
	}
	deployment, err := engine.DeployWithParameters(context.Background(), plan, ParameterValues{"seconds": 3.0})
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(3, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("parameterized within matched at exact deadline = %#v", rows)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A2"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(5, 999999999).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B2"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("parameterized within before deadline rows = %#v", rows)
	}
}
