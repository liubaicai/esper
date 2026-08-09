package esper

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type clientRuntimeStatementBean struct {
	TheString string `esper:"theString"`
}

func deployClientRuntimeStatement(t *testing.T) (*Engine, *Deployment, *Statement) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientRuntimeStatementBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[clientRuntimeStatementBean](env, "SupportBean").
		Window(LengthWindow(2)).
		Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		t.Fatalf("deployment statements = %d, want 1", len(statements))
	}
	return engine, deployment, statements[0]
}

func clientRuntimeStatementBatchStrings(t *testing.T, batch ResultBatch) []string {
	t.Helper()
	values := make([]string, 0, len(batch.New))
	for _, result := range batch.New {
		event, ok := result.Event()
		if !ok {
			t.Fatalf("statement replay result is not an event: %#v", result)
		}
		values = append(values, event.Get("theString").Any().(string))
	}
	return values
}

func TestClientRuntimeEPStatementListenerWithReplayParity(t *testing.T) {
	for seedCount := 0; seedCount <= 2; seedCount++ {
		t.Run(fmt.Sprintf("seed-%d", seedCount), func(t *testing.T) {
			engine, _, statement := deployClientRuntimeStatement(t)
			wantReplay := make([]string, 0, seedCount)
			for index := 1; index <= seedCount; index++ {
				value := fmt.Sprintf("E%d", index)
				wantReplay = append(wantReplay, value)
				if err := engine.SendEvent(context.Background(), &clientRuntimeStatementBean{TheString: value}); err != nil {
					t.Fatal(err)
				}
			}

			var batches []ResultBatch
			if _, err := statement.SubscribeWithReplay(context.Background(), func(_ context.Context, batch ResultBatch) error {
				batches = append(batches, batch)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if len(batches) != 1 {
				t.Fatalf("initial replay callbacks = %d, want 1", len(batches))
			}
			if got := clientRuntimeStatementBatchStrings(t, batches[0]); !reflect.DeepEqual(got, wantReplay) {
				t.Fatalf("initial replay = %#v, want %#v", got, wantReplay)
			}
			if len(batches[0].Old) != 0 {
				t.Fatalf("initial replay old stream = %#v, want empty", batches[0].Old)
			}

			if err := engine.SendEvent(context.Background(), &clientRuntimeStatementBean{TheString: "LIVE"}); err != nil {
				t.Fatal(err)
			}
			if len(batches) != 2 {
				t.Fatalf("callbacks after live event = %d, want 2", len(batches))
			}
			if got := clientRuntimeStatementBatchStrings(t, batches[1]); !reflect.DeepEqual(got, []string{"LIVE"}) {
				t.Fatalf("live batch = %#v, want [LIVE]", got)
			}
		})
	}

	t.Run("reentrant-event-follows-replay", func(t *testing.T) {
		engine, _, statement := deployClientRuntimeStatement(t)
		var calls [][]string
		if _, err := statement.SubscribeWithReplay(context.Background(), func(_ context.Context, batch ResultBatch) error {
			calls = append(calls, clientRuntimeStatementBatchStrings(t, batch))
			if len(calls) == 1 {
				return engine.SendEvent(context.Background(), &clientRuntimeStatementBean{TheString: "E1"})
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(calls, [][]string{{}, {"E1"}}) {
			t.Fatalf("replay/live order = %#v, want empty replay then E1", calls)
		}
	})
}

func TestClientRuntimeEPStatementAlreadyDestroyedParity(t *testing.T) {
	_, deployment, statement := deployClientRuntimeStatement(t)
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if statement.State() != StatementDestroyed {
		t.Fatalf("statement state = %v, want destroyed", statement.State())
	}

	operations := []struct {
		name string
		run  func() error
	}{
		{name: "snapshot", run: func() error {
			_, err := statement.Snapshot(context.Background())
			return err
		}},
		{name: "snapshot-selector", run: func() error {
			_, err := statement.SnapshotWithSelector(context.Background(), ContextPartitionSelectorAll{})
			return err
		}},
		{name: "subscribe", run: func() error {
			_, err := statement.Subscribe(func(context.Context, ResultBatch) error { return nil })
			return err
		}},
		{name: "subscribe-replay", run: func() error {
			_, err := statement.SubscribeWithReplay(context.Background(), func(context.Context, ResultBatch) error { return nil })
			return err
		}},
		{name: "stop", run: func() error { return statement.Stop(context.Background()) }},
		{name: "start", run: func() error { return statement.Start(context.Background()) }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			if err := operation.run(); !errors.Is(err, ErrorState) {
				t.Fatalf("destroyed statement error = %v, want %s", err, ErrorState)
			}
		})
	}
}
