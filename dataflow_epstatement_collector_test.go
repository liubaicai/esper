package esper

import (
	"context"
	"testing"
)

func TestDataflowEPStatementSourceCollectorTransformsAndDuplicatesMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("collector-statement")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	var collectorCalls int
	definition, err := DefineDataflow(env, "statement-collector-flow").
		EPStatementSourceByNameWithCollector("source", "collector-statement", func(_ context.Context, source DataflowStatementSourceContext, value any) ([]any, error) {
			collectorCalls++
			if source.StatementName != "collector-statement" || source.Statement == nil || source.DeploymentID == "" {
				t.Fatalf("collector source context = %#v", source)
			}
			event, ok := value.(Event)
			if !ok {
				t.Fatalf("collector value = %#v, want Event", value)
			}
			trade := event.Underlying().(runtimeTestTrade)
			return []any{value, Event{typeName: event.TypeName(), streamType: event.StreamType(), schema: event.Schema(), underlying: runtimeTestTrade{Symbol: trade.Symbol + "-copy", Price: trade.Price}}}, nil
		}).
		Emitter("emit").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "original", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if collectorCalls != 1 {
		t.Fatalf("collector calls = %d, want 1", collectorCalls)
	}
	assertDataflowEventSymbols(t, instance.Outputs(), []string{"original", "original-copy"})
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDataflowEPStatementSourceCollectorCanSuppressValues(t *testing.T) {
	env, engine := newRuntimeTest(t)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("suppressing-statement")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "statement-suppress-flow").
		EPStatementSourceWithCollector("source", engine.dataflowFindStatement("suppressing-statement"), func(_ context.Context, _ DataflowStatementSourceContext, _ any) ([]any, error) {
			return nil, nil
		}).
		Emitter("emit").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "suppressed"}); err != nil {
		t.Fatal(err)
	}
	if got := len(instance.Outputs()); got != 0 {
		t.Fatalf("suppressed statement output = %#v", instance.Outputs())
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDataflowEPStatementSourceStatementFilterAndCollectorCompose(t *testing.T) {
	env, engine := newRuntimeTest(t)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("combined-statement-source")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "combined-statement-source-flow").
		EPStatementSourceWithStatementFilterAndCollector(
			"source",
			func(ctx DataflowStatementSourceContext) bool { return ctx.StatementName == "combined-statement-source" },
			func(_ context.Context, _ DataflowStatementSourceContext, value any) ([]any, error) {
				return []any{value, value}, nil
			},
		).
		Emitter("emit").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "combined"}); err != nil {
		t.Fatal(err)
	}
	assertDataflowEventSymbols(t, instance.Outputs(), []string{"combined", "combined"})
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDataflowEPStatementSourceBatchCollectorReceivesNewAndOldStreamsMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").
		Window(LengthWindow(1)).
		Query(StatementName("ir-stream-statement"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	definition, err := DefineDataflow(env, "ir-stream-collector-flow").
		EPStatementSourceByNameWithBatchCollector("source", "ir-stream-statement", func(_ context.Context, source DataflowStatementSourceContext, batch ResultBatch) ([]any, error) {
			if source.StatementName != "ir-stream-statement" || source.Statement == nil {
				t.Fatalf("batch collector source context = %#v", source)
			}
			batches = append(batches, batch)
			values := make([]any, 0, len(batch.New)+len(batch.Old))
			for _, result := range append(append([]Result(nil), batch.New...), batch.Old...) {
				event, ok := result.Event()
				if !ok {
					t.Fatalf("batch collector result = %#v, want Event", result)
				}
				values = append(values, event)
			}
			return values, nil
		}).
		Emitter("emit").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "first", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "second", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 || len(batches[1].New) != 1 || len(batches[1].Old) != 1 {
		t.Fatalf("statement IR batches = %#v", batches)
	}
	assertDataflowEventSymbols(t, instance.Outputs(), []string{"first", "second", "first"})
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
}
