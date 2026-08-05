package esper

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestOutputFirstAndLastPolicies(t *testing.T) {
	env, engine := newRuntimeTest(t)
	firstPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("output-first"), WithOutput(OutputFirst(1))))
	if err != nil {
		t.Fatal(err)
	}
	firstDeployment, err := engine.Deploy(context.Background(), firstPlan)
	if err != nil {
		t.Fatal(err)
	}
	var firstBatches []ResultBatch
	if _, err := firstDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		firstBatches = append(firstBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	lastPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("output-last"), WithOutput(OutputLast())))
	if err != nil {
		t.Fatal(err)
	}
	lastDeployment, err := engine.Deploy(context.Background(), lastPlan)
	if err != nil {
		t.Fatal(err)
	}
	var lastBatches []ResultBatch
	if _, err := lastDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		lastBatches = append(lastBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"A", "B"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(firstBatches) != 1 || len(firstBatches[0].New) != 1 {
		t.Fatalf("output-first batches = %#v", firstBatches)
	}
	if len(lastBatches) != 0 {
		t.Fatalf("output-last emitted before flush = %#v", lastBatches)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(lastBatches) != 1 || len(lastBatches[0].New) != 1 {
		t.Fatalf("output-last flush = %#v", lastBatches)
	}
}

func TestDistinctOrderLimitAndOffsetModifiers(t *testing.T) {
	env, engine := newRuntimeTest(t)
	distinctPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(KeepAll()).Query(
		StatementName("distinct-results"),
		WithDistinct(),
	))
	if err != nil {
		t.Fatal(err)
	}
	distinctDeployment, err := engine.Deploy(context.Background(), distinctPlan)
	if err != nil {
		t.Fatal(err)
	}
	var distinctBatches []ResultBatch
	if _, err := distinctDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		distinctBatches = append(distinctBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, price := range []float64{1, 1, 2} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "D", Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	if len(distinctBatches) != 2 {
		t.Fatalf("distinct batches = %#v", distinctBatches)
	}

	orderedPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthBatch(3)).Query(
		StatementName("ordered-results"),
		OrderBy(Descending(Field[runtimeTestTrade, float64]("price"))),
		Limit(1),
		Offset(0),
	))
	if err != nil {
		t.Fatal(err)
	}
	orderedDeployment, err := engine.Deploy(context.Background(), orderedPlan)
	if err != nil {
		t.Fatal(err)
	}
	var ordered ResultBatch
	if _, err := orderedDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		ordered = batch
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, price := range []float64{1, 3, 2} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "O", Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	if len(ordered.New) != 1 {
		t.Fatalf("ordered limited results = %#v", ordered)
	}
	event, ok := ordered.New[0].Event()
	if !ok || event.Underlying().(runtimeTestTrade).Price != 3 {
		t.Fatalf("ordered result = %#v", ordered.New[0].Underlying())
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(Limit(-1))); err == nil {
		t.Fatal("negative limit was accepted")
	}
}

func TestOutputEveryBuffersVisibleResults(t *testing.T) {
	env, engine := newRuntimeTest(t)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("output-every"),
		WithOutput(OutputEvery(2)),
	))
	if err != nil {
		t.Fatal(err)
	}
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
	for _, symbol := range []string{"A", "B", "C"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 1 || len(batches[0].New) != 2 {
		t.Fatalf("output-every batches = %#v", batches)
	}
	first, _ := batches[0].New[0].Event()
	second, _ := batches[0].New[1].Event()
	if first.Underlying().(runtimeTestTrade).Symbol != "A" || second.Underlying().(runtimeTestTrade).Symbol != "B" {
		t.Fatalf("output-every results = %#v", batches[0].New)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(WithOutput(OutputEvery(0)))); err == nil {
		t.Fatal("zero output-every count was accepted")
	}
}

func TestOutputAfterEventCountMatchesEsperActivation(t *testing.T) {
	env, engine := newRuntimeTest(t)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("output-after-events"),
		WithOutput(OutputAfterEvents(3)),
	))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(plan.Canonical()); !strings.Contains(got, "after-events(3)->all") {
		t.Fatalf("canonical output-after description = %q", got)
	}
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
	for _, symbol := range []string{"E1", "E2", "E3"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 0 {
		t.Fatalf("output-after emitted before activation = %#v", batches)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E4"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E5"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[0].New) != 1 || len(batches[1].New) != 1 {
		t.Fatalf("output-after event batches = %#v", batches)
	}
	if got, _ := batches[0].New[0].Event(); got.Underlying().(runtimeTestTrade).Symbol != "E4" {
		t.Fatalf("first activated result = %#v", batches[0].New[0].Underlying())
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(WithOutput(OutputAfterEvents(-1)))); err == nil {
		t.Fatal("negative output-after count was accepted")
	}
}

func TestOutputAfterTimeAndTimedEveryUseVirtualClock(t *testing.T) {
	env, engine := newRuntimeTest(t)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("output-after-time"),
		WithOutput(OutputAfterTime(20*time.Second, OutputEveryTime(5*time.Second))),
	))
	if err != nil {
		t.Fatal(err)
	}
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
	for _, at := range []time.Duration{time.Second, 6 * time.Second, 16 * time.Second} {
		if err := engine.AdvanceTime(context.Background(), time.Unix(0, int64(at)).UTC()); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: at.String()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(20, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E4"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("timed output emitted at activation = %#v", batches)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(24, 999000000).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E5"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(25, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 2 {
		t.Fatalf("timed output first tick = %#v", batches)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E6"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(30, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[1].New) != 1 {
		t.Fatalf("timed output second tick = %#v", batches)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(WithOutput(OutputEveryTime(0)))); err == nil {
		t.Fatal("zero time output interval was accepted")
	}
}

func TestOutputAfterCalendarUsesCalendarBoundaries(t *testing.T) {
	env, _ := newRuntimeTest(t)
	start := time.Date(2002, time.February, 1, 9, 0, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("output-after-calendar"),
		WithOutput(OutputAfterCalendar(0, 1, 0)),
	))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(plan.Canonical()); !strings.Contains(got, "after-calendar(0Y1M0D)->all") {
		t.Fatalf("canonical calendar output-after description = %q", got)
	}
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
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "before"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2002, time.February, 28, 23, 59, 59, 999000000, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "still-before"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("calendar output activated before month boundary = %#v", batches)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2002, time.March, 1, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "after"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("calendar output activation = %#v", batches)
	}
	if got, _ := batches[0].New[0].Event(); got.Underlying().(runtimeTestTrade).Symbol != "after" {
		t.Fatalf("calendar output result = %#v", batches[0].New[0].Underlying())
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(WithOutput(OutputAfterCalendar(-1, 0, 0)))); err == nil {
		t.Fatal("negative output-after calendar period was accepted")
	}
}

func TestOutputAfterComposesWithEventEvery(t *testing.T) {
	env, engine := newRuntimeTest(t)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("output-after-every"),
		WithOutput(OutputAfterEvents(4, OutputEvery(2))),
	))
	if err != nil {
		t.Fatal(err)
	}
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
	for index := 1; index <= 6; index++ {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: fmt.Sprintf("E%d", index)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 1 || len(batches[0].New) != 2 {
		t.Fatalf("after + event-every output = %#v", batches)
	}
}

func TestOutputWhenGatesAndUpdatesVariables(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if err := env.RegisterVariable("output-gate", false); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("output-fired", false); err != nil {
		t.Fatal(err)
	}
	policy := OutputAfterEvents(3, OutputWhen(
		VariableRef[bool]("output-gate"),
		SetOutputVariable("output-fired", Literal(true)),
	))
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("output-when"),
		WithOutput(policy),
	))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(plan.Canonical()); !strings.Contains(got, "when(output-gate;output-fired=true)") {
		t.Fatalf("canonical output-when description = %q", got)
	}
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
	for index := 1; index <= 3; index++ {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: fmt.Sprintf("E%d", index)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SetVariable(context.Background(), "output-gate", true); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E4"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("output-when gated batches = %#v", batches)
	}
	fired, ok := engine.GetVariable("output-fired")
	if !ok || !fired.Equal(Present(true)) {
		t.Fatalf("output-when assignment = %#v", fired)
	}
	if err := engine.SetVariable(context.Background(), "output-gate", false); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E5"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("output-when emitted while condition false = %#v", batches)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(WithOutput(OutputWhen(Field[runtimeTestTrade, string]("symbol"))))); err == nil {
		t.Fatal("event-field output-when condition was accepted")
	}
}

func TestOutputWhenBuffersUntilCountInsertCondition(t *testing.T) {
	env, engine := newRuntimeTest(t)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("output-when-count"),
		WithOutput(OutputWhen(GreaterOrEqual[int64](OutputCountInsert(), Literal(int64(3))))),
	))
	if err != nil {
		t.Fatal(err)
	}
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
	for index := 1; index <= 3; index++ {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: fmt.Sprintf("S%d", index)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 1 || len(batches[0].New) != 3 {
		t.Fatalf("count_insert first output = %#v", batches)
	}
	for index := 4; index <= 6; index++ {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: fmt.Sprintf("S%d", index)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 2 || len(batches[1].New) != 3 {
		t.Fatalf("count_insert second output = %#v", batches)
	}
}

func TestOutputWhenCountsRemovedResultsAndRetainsPendingRows(t *testing.T) {
	env, engine := newRuntimeTest(t)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).Query(
		StatementName("output-when-count-remove"),
		WithOutput(OutputWhen(GreaterOrEqual[int64](OutputCountRemove(), Literal(int64(2))))),
	))
	if err != nil {
		t.Fatal(err)
	}
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
	for index := 1; index <= 4; index++ {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: fmt.Sprintf("R%d", index)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 1 || len(batches[0].New) != 4 {
		t.Fatalf("count_remove output = %#v", batches)
	}
}

func TestOutputWhenUsesLastOutputTimestamp(t *testing.T) {
	env, engine := newRuntimeTest(t)
	elapsed := GreaterOrEqual[int64](
		Subtract[int64](UnixMillis(CurrentTime()), UnixMillis(OutputLastOutputTime())),
		Literal(int64(2000)),
	)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("output-when-timestamp"),
		WithOutput(OutputWhen(elapsed)),
	))
	if err != nil {
		t.Fatal(err)
	}
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
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "T1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 999000000).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "T2"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("timestamp condition fired too early = %#v", batches)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 2 {
		t.Fatalf("timestamp condition output = %#v", batches)
	}
}

func TestOutputWhenThenAssignmentsSeeOutputCounterContext(t *testing.T) {
	env, engine := newRuntimeTest(t)
	for _, name := range []string{"insert-count", "insert-total"} {
		if err := env.RegisterVariable(name, int64(0)); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("output-when-assignment-counters"),
		WithOutput(OutputWhen(
			GreaterOrEqual[int64](OutputCountInsert(), Literal(int64(3))),
			SetOutputVariable("insert-count", OutputCountInsert()),
			SetOutputVariable("insert-total", OutputCountInsertTotal()),
		)),
	))
	if err != nil {
		t.Fatal(err)
	}
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
	for index := 1; index <= 3; index++ {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: fmt.Sprintf("C%d", index)}); err != nil {
			t.Fatal(err)
		}
	}
	assertOutputCounterVariable := func(name string, want int64) {
		t.Helper()
		value, ok := engine.GetVariable(name)
		if !ok || !value.Equal(Present(want)) {
			t.Fatalf("%s after first output = %#v", name, value)
		}
	}
	assertOutputCounterVariable("insert-count", 3)
	assertOutputCounterVariable("insert-total", 3)
	if len(batches) != 1 || len(batches[0].New) != 3 {
		t.Fatalf("counter assignment first output = %#v", batches)
	}
	for index := 4; index <= 6; index++ {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: fmt.Sprintf("C%d", index)}); err != nil {
			t.Fatal(err)
		}
	}
	assertOutputCounterVariable("insert-count", 3)
	assertOutputCounterVariable("insert-total", 6)
	if len(batches) != 2 || len(batches[1].New) != 3 {
		t.Fatalf("counter assignment second output = %#v", batches)
	}
}

func TestOutputWhenThenAssignmentsSeeRemoveCounterContext(t *testing.T) {
	env, engine := newRuntimeTest(t)
	for _, name := range []string{"remove-count", "remove-total"} {
		if err := env.RegisterVariable(name, int64(0)); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).Query(
		StatementName("output-when-assignment-remove-counters"),
		WithOutput(OutputWhen(
			GreaterOrEqual[int64](OutputCountRemove(), Literal(int64(2))),
			SetOutputVariable("remove-count", OutputCountRemove()),
			SetOutputVariable("remove-total", OutputCountRemoveTotal()),
		)),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, _ ResultBatch) error { return nil }); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 4; index++ {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: fmt.Sprintf("R%d", index)}); err != nil {
			t.Fatal(err)
		}
	}
	for name, want := range map[string]int64{"remove-count": 2, "remove-total": 2} {
		value, ok := engine.GetVariable(name)
		if !ok || !value.Equal(Present(want)) {
			t.Fatalf("%s after remove output = %#v", name, value)
		}
	}
}

func TestOutputWhenSnapshotEmitsCurrentWindowAndThenUpdatesVariable(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if err := env.RegisterVariable("snapshot-fired", false); err != nil {
		t.Fatal(err)
	}
	condition := And(
		Greater[int64](OutputCountInsert(), Literal(int64(1))),
		Not(VariableRef[bool]("snapshot-fired")),
	)
	policy := OutputWhenWith(
		OutputSnapshot(),
		condition,
		SetOutputVariable("snapshot-fired", Literal(true)),
	)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LastEvent()).Query(
		StatementName("output-when-snapshot"),
		WithOutput(policy),
	))
	if err != nil {
		t.Fatal(err)
	}
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
	for _, symbol := range []string{"E1", "E2"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("snapshot-when first output = %#v", batches)
	}
	first, _ := batches[0].New[0].Event()
	if first.Underlying().(runtimeTestTrade).Symbol != "E2" {
		t.Fatalf("snapshot-when first row = %#v", batches[0].New[0])
	}
	for _, symbol := range []string{"E3", "E4"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 1 {
		t.Fatalf("snapshot-when fired while variable true = %#v", batches)
	}
	if err := engine.SetVariable(context.Background(), "snapshot-fired", false); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E5"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[1].New) != 1 {
		t.Fatalf("snapshot-when second output = %#v", batches)
	}
	second, _ := batches[1].New[0].Event()
	if second.Underlying().(runtimeTestTrade).Symbol != "E5" {
		t.Fatalf("snapshot-when second row = %#v", batches[1].New[0])
	}
}
