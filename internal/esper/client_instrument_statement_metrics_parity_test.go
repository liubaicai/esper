package esper

import (
	"context"
	"regexp"
	"sort"
	"testing"
	"time"
)

type clientInstrumentMetricBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

func newClientInstrumentMetricEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientInstrumentMetricBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func buildClientInstrumentMetricPlan(t *testing.T, env *Environment, name string, predicate Expression[bool]) Plan {
	t.Helper()
	stream := From[clientInstrumentMetricBean](env, "SupportBean")
	if predicate != nil {
		stream = stream.Filter(predicate)
	}
	plan, err := env.Build(stream.Window(KeepAll()).Query(StatementName(name)))
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func deployClientInstrumentMetricStatement(t *testing.T, engine *Engine, plan Plan, observe bool) (*Deployment, *Statement) {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if observe {
		if _, err := statement.Subscribe(func(context.Context, ResultBatch) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	return deployment, statement
}

func metricNames(metrics []StatementMetric) []string {
	names := make([]string, len(metrics))
	for index, metric := range metrics {
		names[index] = metric.StatementName
	}
	return names
}

func requireMetricNames(t *testing.T, metrics []StatementMetric, want ...string) {
	t.Helper()
	got := metricNames(metrics)
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("statement metric names = %v, want %v; metrics=%#v", got, want, metrics)
	}
	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("statement metric names = %v, want %v; metrics=%#v", got, want, metrics)
		}
	}
}

func TestClientInstrumentMetricsReportingStmtMetricsParity(t *testing.T) {
	origin := time.Unix(0, 0).UTC()
	env := newClientInstrumentMetricEnv(t)
	engine := NewEngine(env,
		WithStartTime(origin),
		WithRuntimeURI("default"),
		WithRuntimeMetrics(-1),
		WithStatementMetrics(-1,
			StatementMetricsGroup("nonmetrics", 10*time.Second,
				StatementMetricIncludeLike("%cpuStmt%"),
				StatementMetricIncludeLike("%wallStmt%"),
				StatementMetricExcludeLike("%metrics%"),
			),
			StatementMetricsGroup("metrics", -1, StatementMetricIncludeLike("%metrics%")),
		),
	)
	if err := engine.AdvanceTime(context.Background(), origin.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	intField := Field[clientInstrumentMetricBean, int]("intPrimitive")
	definitions := []struct {
		name  string
		value int
	}{
		{name: "cpuStmtOne", value: 1},
		{name: "cpuStmtTwo", value: 2},
		{name: "wallStmtThree", value: 3},
		{name: "wallStmtFour", value: 4},
	}
	deployments := make([]*Deployment, 0, len(definitions))
	for _, definition := range definitions {
		plan := buildClientInstrumentMetricPlan(t, env, definition.name, Equal[int](intField, Literal(definition.value)))
		deployment, _ := deployClientInstrumentMetricStatement(t, engine, plan, true)
		deployments = append(deployments, deployment)
	}

	var reported []StatementMetric
	subscription, err := engine.SubscribeStatementMetrics(func(_ context.Context, metric StatementMetric) error {
		reported = append(reported, metric)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = subscription.Close() }()

	for index := 1; index <= 4; index++ {
		if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E", IntPrimitive: index}); err != nil {
			t.Fatal(err)
		}
	}
	groups, err := engine.CurrentStatementMetricGroups(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 || len(groups[0].Metrics) != 0 || len(groups[2].Metrics) != 0 {
		t.Fatalf("statement metric group snapshots = %#v", groups)
	}
	requireMetricNames(t, groups[1].Metrics, "cpuStmtOne", "cpuStmtTwo", "wallStmtThree", "wallStmtFour")
	for _, metric := range groups[1].Metrics {
		if metric.RuntimeURI != "default" || metric.NumInput != 1 || metric.NumOutputIStream != 1 || metric.NumOutputRStream != 0 || metric.WallTime <= 0 {
			t.Fatalf("current statement metric = %#v", metric)
		}
	}
	runtimeMetric, err := engine.CurrentRuntimeMetric(context.Background())
	if err != nil || runtimeMetric.InputCount != 4 {
		t.Fatalf("current runtime metric = %#v, err=%v", runtimeMetric, err)
	}

	if err := engine.AdvanceTime(context.Background(), origin.Add(10999*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if len(reported) != 0 {
		t.Fatalf("statement metrics reported early: %#v", reported)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(11*time.Second)); err != nil {
		t.Fatal(err)
	}
	requireMetricNames(t, reported, "cpuStmtOne", "cpuStmtTwo", "wallStmtThree", "wallStmtFour")
	for _, metric := range reported {
		if !metric.Timestamp.Equal(origin.Add(11*time.Second)) || metric.NumInput != 1 || metric.NumOutputIStream != 1 || metric.NumOutputRStream != 0 {
			t.Fatalf("first reported statement metric = %#v", metric)
		}
	}
	reported = nil

	for index := 1; index <= 4; index++ {
		if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E", IntPrimitive: index}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(21*time.Second)); err != nil {
		t.Fatal(err)
	}
	requireMetricNames(t, reported, "cpuStmtOne", "cpuStmtTwo", "wallStmtThree", "wallStmtFour")
	reported = nil
	for _, deployment := range deployments {
		if err := deployment.Undeploy(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(31*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(reported) != 0 {
		t.Fatalf("statement metrics continued after undeploy: %#v", reported)
	}
}

func TestClientInstrumentMetricsReportingStmtGroupsParity(t *testing.T) {
	origin := time.Unix(0, 0).UTC()
	env := newClientInstrumentMetricEnv(t)
	engine := NewEngine(env,
		WithStartTime(origin),
		WithStatementMetrics(7*time.Second,
			StatementMetricsGroup("GroupOneStatements", 8*time.Second,
				StatementMetricIncludeLike("%GroupOne%"),
				StatementMetricReportInactive(),
			),
			StatementMetricsGroup("GroupTwoNonDefaultStatements", 6*time.Second,
				StatementMetricDefaultInclude(),
				StatementMetricExcludeLike("%Default%"),
				StatementMetricExcludeLike("%Metrics%"),
			),
			StatementMetricsGroup("MetricsStatements", -1, StatementMetricIncludeLike("%Metrics%")),
		),
	)
	if err := engine.AdvanceTime(context.Background(), origin); err != nil {
		t.Fatal(err)
	}
	intField := Field[clientInstrumentMetricBean, int]("intPrimitive")
	_, _ = deployClientInstrumentMetricStatement(t, engine, buildClientInstrumentMetricPlan(t, env, "GroupOne", Equal[int](intField, Literal(1))), false)
	_, groupTwo := deployClientInstrumentMetricStatement(t, engine, buildClientInstrumentMetricPlan(t, env, "GroupTwo", Equal[int](intField, Literal(2))), false)
	if err := groupTwo.SetSubscriber(func(context.Context, SubscriberUpdate) error { return nil }); err != nil {
		t.Fatal(err)
	}
	_, _ = deployClientInstrumentMetricStatement(t, engine, buildClientInstrumentMetricPlan(t, env, "Default", Equal[int](intField, Literal(3))), false)

	var reported []StatementMetric
	listener, err := engine.SubscribeStatementMetrics(func(_ context.Context, metric StatementMetric) error {
		reported = append(reported, metric)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	advance := func(duration time.Duration) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), origin.Add(duration)); err != nil {
			t.Fatal(err)
		}
	}

	advance(6 * time.Second)
	advance(7 * time.Second)
	if len(reported) != 0 {
		t.Fatalf("statement groups reported before 8 seconds: %#v", reported)
	}
	advance(8 * time.Second)
	if len(reported) != 1 || reported[0].StatementName != "GroupOne" || reported[0].NumInput != 0 || reported[0].NumOutputIStream != 0 {
		t.Fatalf("8-second group report = %#v", reported)
	}
	reported = nil
	advance(12 * time.Second)
	advance(14 * time.Second)
	advance(15999 * time.Millisecond)
	if len(reported) != 0 {
		t.Fatalf("statement groups reported before 16 seconds: %#v", reported)
	}
	advance(16 * time.Second)
	if len(reported) != 1 || reported[0].StatementName != "GroupOne" {
		t.Fatalf("16-second group report = %#v", reported)
	}
	reported = nil

	if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E1", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	advance(17999 * time.Millisecond)
	if len(reported) != 0 {
		t.Fatalf("group-two reported early: %#v", reported)
	}
	advance(18 * time.Second)
	if len(reported) != 1 || reported[0].StatementName != "GroupTwo" || reported[0].NumInput != 1 || reported[0].NumOutputIStream != 1 {
		t.Fatalf("group-two report = %#v", reported)
	}
	reported = nil

	if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E1", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	advance(20999 * time.Millisecond)
	if len(reported) != 0 {
		t.Fatalf("default group reported early: %#v", reported)
	}
	advance(21 * time.Second)
	if len(reported) != 1 || reported[0].StatementName != "Default" || reported[0].NumInput != 1 || reported[0].NumOutputIStream != 0 {
		t.Fatalf("default group report = %#v", reported)
	}
	reported = nil

	if err := engine.SetStatementMetricGroupInterval("GroupOneStatements", -1); err != nil {
		t.Fatal(err)
	}
	advance(24 * time.Second)
	if len(reported) != 0 {
		t.Fatalf("disabled group-one reported: %#v", reported)
	}
	if err := engine.SetStatementMetricGroupInterval("GroupOneStatements", time.Second); err != nil {
		t.Fatal(err)
	}
	advance(25 * time.Second)
	if len(reported) != 1 || reported[0].StatementName != "GroupOne" || reported[0].NumInput != 0 {
		t.Fatalf("re-enabled group-one report = %#v", reported)
	}
}

func TestClientInstrumentMetricsReportingDisableStatementParity(t *testing.T) {
	origin := time.Unix(0, 0).UTC()
	env := newClientInstrumentMetricEnv(t)
	engine := NewEngine(env, WithStartTime(origin), WithStatementMetrics(10*time.Second,
		StatementMetricsGroup("metrics", -1, StatementMetricIncludeLike("%@METRIC%")),
	))
	if err := engine.AdvanceTime(context.Background(), origin.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	intField := Field[clientInstrumentMetricBean, int]("intPrimitive")
	stmtOneDeployment, _ := deployClientInstrumentMetricStatement(t, engine, buildClientInstrumentMetricPlan(t, env, "stmtone", Equal[int](intField, Literal(1))), false)
	if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	stmtTwoDeployment, _ := deployClientInstrumentMetricStatement(t, engine, buildClientInstrumentMetricPlan(t, env, "stmttwo", Greater[int](intField, Literal(0))), false)
	if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E2", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}

	var reported []StatementMetric
	subscription, err := engine.SubscribeStatementMetrics(func(_ context.Context, metric StatementMetric) error {
		reported = append(reported, metric)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = subscription.Close() }()
	advance := func(seconds int) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), origin.Add(time.Duration(seconds)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}

	advance(11)
	requireMetricNames(t, reported, "stmtone", "stmttwo")
	reported = nil
	if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	advance(21)
	requireMetricNames(t, reported, "stmtone", "stmttwo")
	reported = nil

	if err := engine.SetStatementMetricsEnabled(stmtOneDeployment.ID(), "stmtone", false); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	advance(31)
	requireMetricNames(t, reported, "stmttwo")
	reported = nil

	if err := engine.SetStatementMetricsEnabled(stmtOneDeployment.ID(), "stmtone", true); err != nil {
		t.Fatal(err)
	}
	if err := engine.SetStatementMetricsEnabled(stmtTwoDeployment.ID(), "stmttwo", false); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	advance(41)
	requireMetricNames(t, reported, "stmtone")
}

func TestClientInstrumentMetricsReportingDisableRuntimeParity(t *testing.T) {
	origin := time.Unix(0, 0).UTC()
	env := newClientInstrumentMetricEnv(t)
	engine := NewEngine(env,
		WithStartTime(origin),
		WithRuntimeMetrics(10*time.Second),
		WithStatementMetrics(10*time.Second),
	)
	if err := engine.AdvanceTime(context.Background(), origin.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	intField := Field[clientInstrumentMetricBean, int]("intPrimitive")
	deployment, _ := deployClientInstrumentMetricStatement(t, engine, buildClientInstrumentMetricPlan(t, env, "stmt-1", Equal[int](intField, Literal(1))), true)
	var runtimeReports []RuntimeMetric
	var statementReports []StatementMetric
	runtimeSubscription, err := engine.SubscribeRuntimeMetrics(func(_ context.Context, metric RuntimeMetric) error {
		runtimeReports = append(runtimeReports, metric)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = runtimeSubscription.Close() }()
	statementSubscription, err := engine.SubscribeStatementMetrics(func(_ context.Context, metric StatementMetric) error {
		statementReports = append(statementReports, metric)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = statementSubscription.Close() }()

	if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(11*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(runtimeReports) != 1 || len(statementReports) != 1 {
		t.Fatalf("initial runtime/statement reports = %d/%d", len(runtimeReports), len(statementReports))
	}
	runtimeReports = nil
	statementReports = nil

	if err := engine.SetMetricsReportingEnabled(false); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E2", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Duration{21 * time.Second, 31 * time.Second} {
		if err := engine.AdvanceTime(context.Background(), origin.Add(at)); err != nil {
			t.Fatal(err)
		}
	}
	if len(runtimeReports) != 0 || len(statementReports) != 0 {
		t.Fatalf("disabled runtime/statement reports = %#v / %#v", runtimeReports, statementReports)
	}
	if err := engine.SetMetricsReportingEnabled(true); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E4", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(41*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(runtimeReports) != 1 || len(statementReports) != 1 || statementReports[0].NumInput != 2 {
		t.Fatalf("re-enabled runtime/statement reports = %#v / %#v", runtimeReports, statementReports)
	}
	runtimeReports = nil
	statementReports = nil
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(51*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(runtimeReports) != 1 || len(statementReports) != 0 {
		t.Fatalf("post-undeploy runtime/statement reports = %#v / %#v", runtimeReports, statementReports)
	}
}

func TestClientInstrumentMetricsReportingNamedWindowParity(t *testing.T) {
	origin := time.Unix(0, 0).UTC()
	env := newClientInstrumentMetricEnv(t)
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema is missing")
	}
	if _, err := CreateNamedWindow(env, "A", schema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "W", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(origin), WithStatementMetrics(time.Second))
	if err := engine.AdvanceTime(context.Background(), origin); err != nil {
		t.Fatal(err)
	}

	source := From[clientInstrumentMetricBean](env, "SupportBean")
	assignments := []TableAssignment{
		SetColumn("theString", Field[clientInstrumentMetricBean, string]("theString")),
		SetColumn("intPrimitive", Field[clientInstrumentMetricBean, int]("intPrimitive")),
		SetColumn("longPrimitive", Field[clientInstrumentMetricBean, int64]("longPrimitive")),
	}
	b1, err := env.Build(OnEvent(source).InsertIntoNamedWindow("A", assignments...).Query(StatementName("B1")))
	if err != nil {
		t.Fatal(err)
	}
	b2, err := env.Build(OnEvent(source).InsertIntoNamedWindow("A", assignments...).Query(StatementName("B2")))
	if err != nil {
		t.Fatal(err)
	}
	c, err := env.Build(FromNamedWindow(env, "A").Aggregate(
		Alias("sum", Sum[int](Field[any, int]("intPrimitive"))),
	).Query(StatementName("C")))
	if err != nil {
		t.Fatal(err)
	}
	d, err := env.Build(JoinMany(
		JoinRecordSource(FromNamedWindow(env, "A")),
		JoinRecordSource(FromNamedWindow(env, "A")),
	).On(
		OnSourcesEqual(0, Field[any, string]("theString"), 1, Field[any, string]("theString")),
	).Select(
		SelectFrom(0, "intPrimitive", Field[any, int]("intPrimitive")),
	).Query(StatementName("D")))
	if err != nil {
		t.Fatal(err)
	}
	match := Equal[string](NamedWindowField[string]("theString"), Field[clientInstrumentMetricBean, string]("theString"))
	m, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("W", match,
		WhenNotMatched(Literal(true), assignments...),
	).Query(StatementName("M")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.DeployPlans(context.Background(), []Plan{b1, b2, c, d, m}); err != nil {
		t.Fatal(err)
	}

	var reported []StatementMetric
	subscription, err := engine.SubscribeStatementMetrics(func(_ context.Context, metric StatementMetric) error {
		reported = append(reported, metric)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = subscription.Close() }()
	if err := engine.SendEvent(context.Background(), clientInstrumentMetricBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	requireMetricNames(t, reported, "A", "B1", "B2", "C", "D", "M", "W")
	wantInput := map[string]uint64{"A": 2, "B1": 1, "B2": 1, "C": 2, "D": 2, "M": 1, "W": 1}
	for _, metric := range reported {
		if metric.NumInput != wantInput[metric.StatementName] {
			t.Fatalf("named-window metric %s numInput = %d, want %d; metric=%#v", metric.StatementName, metric.NumInput, wantInput[metric.StatementName], metric)
		}
	}
}

func TestStatementMetricsConfigurationAndSubscriptionLifecycle(t *testing.T) {
	env := newClientInstrumentMetricEnv(t)
	unconfigured := NewEngine(env)
	if _, err := unconfigured.SubscribeStatementMetrics(func(context.Context, StatementMetric) error { return nil }); err == nil {
		t.Fatal("statement metric subscription succeeded without configuration")
	}
	if _, err := unconfigured.CurrentStatementMetricGroups(context.Background()); err == nil {
		t.Fatal("statement metric snapshot succeeded without configuration")
	}
	if err := unconfigured.SetStatementMetricGroupInterval("", time.Second); err == nil {
		t.Fatal("statement metric interval update succeeded without configuration")
	}

	origin := time.Unix(0, 0).UTC()
	configured := NewEngine(env, WithStartTime(origin), WithStatementMetrics(-1,
		StatementMetricsGroup("regex", time.Second,
			StatementMetricIncludeRegexp(regexp.MustCompile(`^regex-`)),
			StatementMetricExcludeRegexp(regexp.MustCompile(`skip`)),
			StatementMetricIncludeRegexp(regexp.MustCompile(`^regex-skip-include$`)),
		),
	))
	if _, err := configured.SubscribeStatementMetrics(nil); err == nil {
		t.Fatal("nil statement metric listener was accepted")
	}
	if err := configured.AdvanceTime(context.Background(), origin); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"regex-one", "regex-skip", "regex-skip-include"} {
		_, _ = deployClientInstrumentMetricStatement(t, configured, buildClientInstrumentMetricPlan(t, env, name, nil), false)
	}
	if err := configured.SendEvent(context.Background(), clientInstrumentMetricBean{}); err != nil {
		t.Fatal(err)
	}
	groups, err := configured.CurrentStatementMetricGroups(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("regexp statement metric group count = %d, want 2", len(groups))
	}
	requireMetricNames(t, groups[0].Metrics, "regex-skip")
	requireMetricNames(t, groups[1].Metrics, "regex-one", "regex-skip-include")

	delivered := 0
	subscription, err := configured.SubscribeStatementMetrics(func(context.Context, StatementMetric) error {
		delivered++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := subscription.Close(); err != nil {
		t.Fatal(err)
	}
	if err := subscription.Close(); err != nil {
		t.Fatal(err)
	}
	if err := configured.AdvanceTime(context.Background(), origin.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if delivered != 0 {
		t.Fatalf("closed statement metric subscription delivered %d records", delivered)
	}
	if err := configured.SetStatementMetricGroupInterval("missing", time.Second); err == nil {
		t.Fatal("unknown statement metric group interval update succeeded")
	}
	if err := configured.SetStatementMetricsEnabled("missing", "missing", false); err == nil {
		t.Fatal("unknown statement metric enable update succeeded")
	}
}
