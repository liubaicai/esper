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
