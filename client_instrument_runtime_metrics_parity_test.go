package esper

import (
	"context"
	"testing"
	"time"
)

func TestClientInstrumentMetricsReportingRuntimeMetricsParity(t *testing.T) {
	origin := time.Unix(0, 0).UTC()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientInstrumentSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env,
		WithStartTime(origin),
		WithRuntimeURI("default"),
		WithRuntimeMetrics(10*time.Second),
	)
	var metrics []RuntimeMetric
	subscription, err := engine.SubscribeRuntimeMetrics(func(_ context.Context, metric RuntimeMetric) error {
		metrics = append(metrics, metric)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = subscription.Close() }()

	if err := engine.AdvanceTime(context.Background(), origin.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientInstrumentSupportBean{}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(10999*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 0 {
		t.Fatalf("runtime metrics before first interval = %#v", metrics)
	}

	source := From[clientInstrumentSupportBean](env, "SupportBean")
	timerPlan, err := env.Build(TimerAt(source, origin.Add(15999*time.Millisecond)).
		Select(Alias("fired", Literal(true))).
		Query(StatementName("pending-schedule")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), timerPlan); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(11*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 1 {
		t.Fatalf("first runtime metric count = %d, want 1", len(metrics))
	}
	first := metrics[0]
	if first.RuntimeURI != "default" || !first.Timestamp.Equal(origin.Add(11*time.Second)) || first.InputCount != 1 || first.InputCountDelta != 1 || first.ScheduleDepth != 1 {
		t.Fatalf("first runtime metric = %#v", first)
	}

	if err := engine.SendEvent(context.Background(), clientInstrumentSupportBean{}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientInstrumentSupportBean{}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 1 {
		t.Fatalf("runtime metric fired before second interval: %#v", metrics)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(21*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 2 {
		t.Fatalf("second runtime metric count = %d, want 2", len(metrics))
	}
	second := metrics[1]
	if second.RuntimeURI != "default" || !second.Timestamp.Equal(origin.Add(21*time.Second)) || second.InputCount != 4 || second.InputCountDelta != 3 || second.ScheduleDepth != 0 {
		t.Fatalf("second runtime metric = %#v", second)
	}

	current, err := engine.CurrentRuntimeMetric(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if current.InputCount != 5 || current.InputCountDelta != 1 || !current.Timestamp.Equal(origin.Add(21*time.Second)) {
		t.Fatalf("current runtime metric = %#v", current)
	}
}

func TestRuntimeMetricsConfigurationAndSubscriptionLifecycle(t *testing.T) {
	env := NewEnvironment()
	engine := NewEngine(env)
	if _, err := engine.SubscribeRuntimeMetrics(func(context.Context, RuntimeMetric) error { return nil }); err == nil {
		t.Fatal("runtime metric subscription succeeded without configuration")
	}
	if _, err := engine.CurrentRuntimeMetric(context.Background()); err == nil {
		t.Fatal("runtime metric snapshot succeeded without configuration")
	}

	configured := NewEngine(env, WithRuntimeMetrics(time.Second))
	if _, err := configured.SubscribeRuntimeMetrics(nil); err == nil {
		t.Fatal("nil runtime metric listener was accepted")
	}
	var delivered int
	subscription, err := configured.SubscribeRuntimeMetrics(func(context.Context, RuntimeMetric) error {
		delivered++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := configured.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := configured.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if delivered != 1 {
		t.Fatalf("configured runtime metric deliveries = %d, want 1", delivered)
	}
	if err := subscription.Close(); err != nil {
		t.Fatal(err)
	}
	if err := subscription.Close(); err != nil {
		t.Fatal(err)
	}
	if err := configured.AdvanceTime(context.Background(), time.Unix(3, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if delivered != 1 {
		t.Fatalf("closed runtime metric subscription delivered %d records", delivered)
	}
	if err := configured.SetRuntimeMetricsEnabled(false); err != nil {
		t.Fatal(err)
	}
	if err := configured.AdvanceTime(context.Background(), time.Unix(4, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := configured.SetRuntimeMetricsEnabled(true); err != nil {
		t.Fatal(err)
	}
}
