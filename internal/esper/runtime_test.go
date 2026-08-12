package esper

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type runtimeTestTrade struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

func newRuntimeTest(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
}

func TestLengthWindowFilterAndOldStream(t *testing.T) {
	env, engine := newRuntimeTest(t)
	stream := From[runtimeTestTrade](env, "Trade").
		Filter(Greater[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))).
		Window(LengthWindow(2))
	plan, err := env.Build(stream.Query(StatementName("trades"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	_, err = statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "ignored", Price: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 11}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 12}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "C", Price: 13}); err != nil {
		t.Fatal(err)
	}

	if len(batches) != 3 {
		t.Fatalf("got %d batches, want 3", len(batches))
	}
	if len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		t.Fatalf("first batch = %#v", batches[0])
	}
	old, ok := batches[2].Old[0].Event()
	if !ok || old.Underlying().(runtimeTestTrade).Symbol != "A" {
		t.Fatalf("third old stream = %#v", batches[2].Old)
	}
}

func TestTimeWindowExpiresOnlyOnAdvanceTime(t *testing.T) {
	env, engine := newRuntimeTest(t)
	stream := From[runtimeTestTrade](env, "Trade").Window(TimeWindow(time.Second))
	plan, err := env.Build(stream.Query(StatementName("timed"), WithOldStream()))
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
	base := time.Unix(0, 0).UTC()
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), base.Add(999*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("premature expiry batches = %d", len(batches))
	}
	if err := engine.AdvanceTime(context.Background(), base.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[1].Old) != 1 {
		t.Fatalf("expiry batch = %#v", batches)
	}
}

func TestProjectionUsesOrderedRowAndPlanHashIsStable(t *testing.T) {
	env, engine := newRuntimeTest(t)
	stream := From[runtimeTestTrade](env, "Trade")
	projected := Select(stream,
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
		Alias("doublePrice", Multiply[float64](Field[runtimeTestTrade, float64]("price"), Literal(2.0))),
	)
	query := projected.Query(StatementName("projection"))
	planOne, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	planTwo, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if planOne.Hash() != planTwo.Hash() || !reflect.DeepEqual(planOne.Canonical(), planTwo.Canonical()) {
		t.Fatalf("same query must have stable plan hash: %s != %s", planOne.Hash(), planTwo.Hash())
	}
	deployment, err := engine.Deploy(context.Background(), planOne)
	if err != nil {
		t.Fatal(err)
	}
	var row Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		var ok bool
		row, ok = batch.New[0].Row()
		if !ok {
			t.Fatal("projection result is not a Row")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 2.5}); err != nil {
		t.Fatal(err)
	}
	if got := row.Get("symbol").Any(); got != "A" {
		t.Fatalf("projected symbol = %#v", got)
	}
	if got := row.Get("doublePrice").Any(); got != 5.0 {
		t.Fatalf("projected doublePrice = %#v", got)
	}
	artifact, err := planOne.MarshalArtifact()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPlanArtifact(artifact)
	if err != nil || loaded.Hash != planOne.Hash() {
		t.Fatalf("plan artifact = %#v, err=%v", loaded, err)
	}
	artifact[len(artifact)-2] ^= 1
	if _, err := LoadPlanArtifact(artifact); err == nil {
		t.Fatal("tampered plan artifact was accepted")
	}
}

func TestBuildRejectsUnknownFieldAndLifecycleIsIdempotent(t *testing.T) {
	env, engine := newRuntimeTest(t)
	stream := From[runtimeTestTrade](env, "Trade").Filter(Equal[string](Field[runtimeTestTrade, string]("doesNotExist"), Literal("x")))
	if _, err := env.Build(stream.Query()); err == nil {
		t.Fatal("unknown field must fail Build")
	} else if !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("unknown field error code = %v", err)
	}
	valid := From[runtimeTestTrade](env, "Trade").Query(StatementName("one"))
	plan, err := env.Build(valid)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("same statement name in another deployment: %v", err)
	}
	t.Cleanup(func() { _ = duplicate.Undeploy(context.Background()) })
	if duplicate.ID() == deployment.ID() || duplicate.Statements()[0].ID() == deployment.Statements()[0].ID() {
		t.Fatal("duplicate-name deployments did not receive distinct identities")
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.Undeploy(context.Background(), deployment.ID()); err == nil {
		t.Fatal("second undeploy should report missing deployment")
	}
}

func TestContextCancellationStopsSend(t *testing.T) {
	env, engine := newRuntimeTest(t)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := engine.SendEvent(ctx, runtimeTestTrade{}); err == nil {
		t.Fatal("cancelled context must stop SendEvent")
	}
}
