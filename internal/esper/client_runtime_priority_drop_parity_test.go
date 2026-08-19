package esper

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestClientRuntimeSchedulingPriorityParity(t *testing.T) {
	env, _ := newRuntimeTest(t)
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	var got []int

	s1 := deployClientRuntimePriorityTimer(t, engine, env, "s1", 1, true, false, &got)
	_ = deployClientRuntimePriorityTimer(t, engine, env, "s3", 3, true, false, &got)
	s2 := deployClientRuntimePriorityTimer(t, engine, env, "s2", 2, true, false, &got)
	_ = deployClientRuntimePriorityTimer(t, engine, env, "s4", 4, true, false, &got)
	advanceClientRuntimePriority(t, engine, origin.Add(10*time.Second), &got, 4, 3, 2, 1)

	if err := s2.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = deployClientRuntimePriorityTimer(t, engine, env, "s0", 0, false, false, &got)
	advanceClientRuntimePriority(t, engine, origin.Add(20*time.Second), &got, 4, 3, 1, 0)

	_ = deployClientRuntimePriorityTimer(t, engine, env, "s2", 2, true, false, &got)
	advanceClientRuntimePriority(t, engine, origin.Add(30*time.Second), &got, 4, 3, 2, 1, 0)

	_ = deployClientRuntimePriorityTimer(t, engine, env, "s5", 3, true, false, &got)
	advanceClientRuntimePriority(t, engine, origin.Add(40*time.Second), &got, 4, 3, 3, 2, 1, 0)

	priority, explicit := s1.Statements()[0].Priority()
	if !explicit || priority != 1 || s1.Statements()[0].DropsLowerPriority() {
		t.Fatalf("s1 priority metadata = (%d, %t, drop=%t)", priority, explicit, s1.Statements()[0].DropsLowerPriority())
	}
}

func TestClientRuntimeSchedulingDropParity(t *testing.T) {
	env, _ := newRuntimeTest(t)
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	var got []int

	drop := deployClientRuntimePriorityTimer(t, engine, env, "s1", 1, false, true, &got)
	_ = deployClientRuntimePriorityTimer(t, engine, env, "s3", 3, true, false, &got)
	_ = deployClientRuntimePriorityTimer(t, engine, env, "s2", 2, false, false, &got)
	advanceClientRuntimePriority(t, engine, origin.Add(10*time.Second), &got, 3, 1)
	if _, explicit := drop.Statements()[0].Priority(); explicit {
		t.Fatal("drop statement unexpectedly reports an explicit default priority")
	}
	if !drop.Statements()[0].DropsLowerPriority() {
		t.Fatal("drop statement metadata is false")
	}
}

func TestClientRuntimeNamedWindowPriorityParity(t *testing.T) {
	env, engine := newClientRuntimePriorityWindow(t)
	var got []int
	deploy := func(name string, priority int, explicit bool, drop bool) *Deployment {
		t.Helper()
		options := clientRuntimePriorityOptions(name, priority, explicit, drop)
		plan, err := env.Build(OnRecord(FromNamedWindow(env, "MyWindow")).SelectFromNamedWindow("MyWindow", Literal(true),
			Alias("theString", NamedWindowField[string]("symbol")),
			Alias("prio", Literal(priority)),
		).Query(options...))
		if err != nil {
			t.Fatal(err)
		}
		return deployClientRuntimePriorityPlan(t, engine, plan, &got)
	}

	_ = deploy("s1", 1, true, false)
	_ = deploy("s3", 3, true, false)
	s2 := deploy("s2", 2, true, false)
	_ = deploy("s4", 4, true, false)
	insertClientRuntimePriorityWindow(t, engine, "E1", &got, 4, 3, 2, 1)

	if err := s2.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = deploy("s0", 0, false, false)
	insertClientRuntimePriorityWindow(t, engine, "E2", &got, 4, 3, 1, 0)
	_ = deploy("s2", 2, true, false)
	insertClientRuntimePriorityWindow(t, engine, "E3", &got, 4, 3, 2, 1, 0)
	_ = deploy("sx", 3, true, false)
	insertClientRuntimePriorityWindow(t, engine, "E4", &got, 4, 3, 3, 2, 1, 0)
}

func TestClientRuntimeNamedWindowDropParity(t *testing.T) {
	env, engine := newClientRuntimePriorityWindow(t)
	var got []int
	deploy := func(name string, priority int, explicit bool, drop bool) {
		t.Helper()
		plan, err := env.Build(OnRecord(FromNamedWindow(env, "MyWindow")).SelectFromNamedWindow("MyWindow", Literal(true),
			Alias("theString", NamedWindowField[string]("symbol")),
			Alias("prio", Literal(priority)),
		).Query(clientRuntimePriorityOptions(name, priority, explicit, drop)...))
		if err != nil {
			t.Fatal(err)
		}
		_ = deployClientRuntimePriorityPlan(t, engine, plan, &got)
	}
	deploy("s2", 2, false, true)
	deploy("s3", 3, true, false)
	deploy("s4", 0, false, false)
	insertClientRuntimePriorityWindow(t, engine, "E1", &got, 3, 2)
}

func TestClientRuntimeNamedWindowFilteredDropDoesNotBlockParity(t *testing.T) {
	env, engine := newClientRuntimePriorityWindow(t)
	var got []string
	dropPlan, err := env.Build(OnRecord(FromNamedWindow(env, "MyWindow").Filter(
		Equal[string](NamedWindowField[string]("symbol"), Literal("reject")),
	)).SelectFromNamedWindow("MyWindow", Literal(true),
		Alias("symbol", NamedWindowField[string]("symbol")),
	).Query(StatementName("filtered-drop"), StatementPriority(10), StatementDrop()))
	if err != nil {
		t.Fatal(err)
	}
	dropDeployment, err := engine.Deploy(context.Background(), dropPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dropDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			got = append(got, "drop:"+result.Get("symbol").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	lowPlan, err := env.Build(OnRecord(FromNamedWindow(env, "MyWindow")).SelectFromNamedWindow("MyWindow", Literal(true),
		Alias("symbol", NamedWindowField[string]("symbol")),
	).Query(StatementName("low")))
	if err != nil {
		t.Fatal(err)
	}
	lowDeployment, err := engine.Deploy(context.Background(), lowPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lowDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			got = append(got, "low:"+result.Get("symbol").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "accept"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"low:accept"}) {
		t.Fatalf("filtered drop output = %v, want [low:accept]", got)
	}
}

func TestClientRuntimePriorityParity(t *testing.T) {
	env, engine := newRuntimeTest(t)
	var got []int
	deploy := func(name string, priority int, explicit bool) *Deployment {
		t.Helper()
		stream := From[runtimeTestTrade](env, "Trade")
		plan, err := env.Build(Select(stream,
			Alias("theString", Field[runtimeTestTrade, string]("symbol")),
			Alias("prio", Literal(priority)),
		).Query(clientRuntimePriorityOptions(name, priority, explicit, false)...))
		if err != nil {
			t.Fatal(err)
		}
		return deployClientRuntimePriorityPlan(t, engine, plan, &got)
	}

	_ = deploy("s1", 1, true)
	_ = deploy("s3", 3, true)
	s2 := deploy("s2", 2, true)
	_ = deploy("s4", 4, true)
	statements, err := engine.Statements(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(statements))
	for index, statement := range statements {
		names[index] = statement.Name()
	}
	if !slices.Equal(names, []string{"s1", "s3", "s2", "s4"}) {
		t.Fatalf("management traversal order = %v, want deployment order", names)
	}
	sendClientRuntimePriorityTrade(t, engine, "E1", 0, &got, 4, 3, 2, 1)
	if err := s2.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = deploy("s0", 0, false)
	sendClientRuntimePriorityTrade(t, engine, "E2", 0, &got, 4, 3, 1, 0)
	_ = deploy("s2", 2, true)
	sendClientRuntimePriorityTrade(t, engine, "E3", 0, &got, 4, 3, 2, 1, 0)
	_ = deploy("sx", 3, true)
	sendClientRuntimePriorityTrade(t, engine, "E4", 0, &got, 4, 3, 3, 2, 1, 0)
}

func TestClientRuntimeAddRemoveStatementsParity(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := RegisterStruct[runtimeTestTrade](env, "ABCStream"); err != nil {
		t.Fatal(err)
	}
	var routed []string
	routePlan, err := env.Build(From[runtimeTestTrade](env, "Trade").InsertInto("ABCStream", StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	routeDeployment, err := engine.Deploy(context.Background(), routePlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := routeDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			routed = append(routed, result.Get("symbol").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	drops := make(map[string][]string)
	deployDrop := func(name string, price float64) *Deployment {
		t.Helper()
		plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Filter(
			Equal[float64](Field[runtimeTestTrade, float64]("price"), Literal(price)),
		).Query(StatementName(name), StatementDrop()))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				drops[name] = append(drops[name], result.Get("symbol").Any().(string))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return deployment
	}
	l0 := deployDrop("l0", 1)
	_ = deployDrop("l1", 2)

	send := func(symbol string, price float64, wantRoute bool, wantDrop string) {
		t.Helper()
		beforeRoute := len(routed)
		beforeDrops := map[string]int{}
		for name, values := range drops {
			beforeDrops[name] = len(values)
		}
		if err := engine.Send(context.Background(), "Trade", runtimeTestTrade{Symbol: symbol, Price: price}); err != nil {
			t.Fatal(err)
		}
		if got := len(routed) - beforeRoute; got != boolCount(wantRoute) {
			t.Fatalf("%s route count delta = %d, want %d", symbol, got, boolCount(wantRoute))
		}
		for name, values := range drops {
			want := 0
			if name == wantDrop {
				want = 1
			}
			if got := len(values) - beforeDrops[name]; got != want {
				t.Fatalf("%s drop %s count delta = %d, want %d", symbol, name, got, want)
			}
		}
	}

	send("E1", 1, false, "l0")
	send("E2", 2, false, "l1")
	send("E3", 1, false, "l0")
	send("E4", 3, true, "")
	_ = deployDrop("l2", 3)
	send("E5", 3, false, "l2")
	send("E6", 1, false, "l0")
	if err := l0.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	send("E7", 1, true, "")

	highPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("s1"), StatementPriority(50)))
	if err != nil {
		t.Fatal(err)
	}
	high, err := engine.Deploy(context.Background(), highPlan)
	if err != nil {
		t.Fatal(err)
	}
	var highSymbols []string
	if _, err := high.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			highSymbols = append(highSymbols, result.Get("symbol").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send("E8", 1, true, "")
	if !slices.Equal(highSymbols, []string{"E8"}) {
		t.Fatalf("high-priority statement symbols = %v, want [E8]", highSymbols)
	}
	send("E9", 2, false, "l1")
	if !slices.Equal(routed, []string{"E4", "E7", "E8"}) {
		t.Fatalf("routed symbols = %v", routed)
	}
}

func TestStatementPriorityDropPlanBoundaries(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("base")))
	if err != nil {
		t.Fatal(err)
	}
	prioritized, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("prioritized"), StatementPriority(10), StatementDrop(),
	))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Equal(base.Canonical(), prioritized.Canonical()) {
		t.Fatal("statement priority/drop did not enter canonical plan identity")
	}
	invalid := From[runtimeTestTrade](env, "Trade").UpdateStream(
		SetColumn("price", Literal(1.0)),
	).Query(StatementName("invalid"), StatementPriority(1))
	if _, err := env.Build(invalid); err == nil {
		t.Fatal("general statement priority was accepted for update-istream")
	}
}

func deployClientRuntimePriorityTimer(t *testing.T, engine *Engine, env *Environment, name string, priority int, explicit bool, drop bool, got *[]int) *Deployment {
	t.Helper()
	plan, err := env.Build(TimerInterval(From[runtimeTestTrade](env, "Trade"), 10*time.Second).Every().Select(
		Alias("prio", Literal(priority)),
	).Query(clientRuntimePriorityOptions(name, priority, explicit, drop)...))
	if err != nil {
		t.Fatal(err)
	}
	return deployClientRuntimePriorityPlan(t, engine, plan, got)
}

func deployClientRuntimePriorityPlan(t *testing.T, engine *Engine, plan Plan, got *[]int) *Deployment {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			*got = append(*got, result.Get("prio").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return deployment
}

func clientRuntimePriorityOptions(name string, priority int, explicit bool, drop bool) []QueryOption {
	options := []QueryOption{StatementName(name)}
	if explicit {
		options = append(options, StatementPriority(priority))
	}
	if drop {
		options = append(options, StatementDrop())
	}
	return options
}

func advanceClientRuntimePriority(t *testing.T, engine *Engine, at time.Time, got *[]int, want ...int) {
	t.Helper()
	*got = nil
	if err := engine.AdvanceTime(context.Background(), at); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(*got, want) {
		t.Fatalf("priority order at %s = %v, want %v", at, *got, want)
	}
}

func newClientRuntimePriorityWindow(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env, _ := newRuntimeTest(t)
	schema, _ := env.Schema("Trade")
	if _, err := CreateNamedWindow(env, "MyWindow", schema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	insertPlan, err := env.Build(OnEvent(From[runtimeTestTrade](env, "Trade")).InsertIntoNamedWindow("MyWindow",
		SetColumn("symbol", Field[runtimeTestTrade, string]("symbol")),
		SetColumn("price", Field[runtimeTestTrade, float64]("price")),
	).Query(StatementName("insert-window")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	return env, engine
}

func insertClientRuntimePriorityWindow(t *testing.T, engine *Engine, symbol string, got *[]int, want ...int) {
	t.Helper()
	*got = nil
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(*got, want) {
		t.Fatalf("named-window priority order for %s = %v, want %v", symbol, *got, want)
	}
}

func sendClientRuntimePriorityTrade(t *testing.T, engine *Engine, symbol string, price float64, got *[]int, want ...int) {
	t.Helper()
	*got = nil
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol, Price: price}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(*got, want) {
		t.Fatalf("event priority order for %s = %v, want %v", symbol, *got, want)
	}
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
