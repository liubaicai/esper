package esper

import (
	"context"
	"testing"
)

type methodContextRow struct {
	Symbol string `esper:"symbol"`
	Value  int    `esper:"value"`
}

func TestMethodSourceContextFireAndForgetPartitionsRows(t *testing.T) {
	env := NewEnvironment()
	schema, err := StructSchema[methodContextRow]("MethodContextRow")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "method-by-symbol", Field[any, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	calls := 0
	provider := MethodProviderFunc(func(ctx context.Context, request MethodRequest) ([]Event, error) {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		calls++
		rows := []methodContextRow{
			{Symbol: "A", Value: 1},
			{Symbol: "A", Value: 3},
			{Symbol: "B", Value: 2},
		}
		events := make([]Event, 0, len(rows))
		for _, row := range rows {
			event, eventErr := newEvent(schema, row, request.Now)
			if eventErr != nil {
				return nil, eventErr
			}
			events = append(events, event)
		}
		return events, nil
	})
	method := FromMethod[methodContextRow](env, "method-context", schema, provider)
	plan, err := env.Build(Select(method,
		Alias("key", ContextKeyValue[string](0)),
		Alias("symbol", Field[methodContextRow, string]("symbol")),
		Alias("value", Field[methodContextRow, int]("value")),
	).Query(StatementName("method-source-context-faf"), WithContext("method-by-symbol")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(env).ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("method context FAF provider calls = %d, want 1", calls)
	}
	if len(result.Results()) != 3 {
		t.Fatalf("method context FAF result count = %d, want 3: %#v", len(result.Results()), result.Results())
	}
	valuesBySymbol := make(map[string][]int)
	for _, item := range result.Results() {
		row, ok := item.Row()
		if !ok {
			t.Fatalf("method context FAF result is not a row: %#v", item)
		}
		symbol, _ := row.Get("symbol").Any().(string)
		value, _ := row.Get("value").Any().(int)
		if row.Get("key").Any() != symbol {
			t.Fatalf("method context FAF partition key = %#v, symbol = %q", row.Get("key"), symbol)
		}
		valuesBySymbol[symbol] = append(valuesBySymbol[symbol], value)
	}
	if len(valuesBySymbol["A"]) != 2 || len(valuesBySymbol["B"]) != 1 {
		t.Fatalf("method context FAF partition rows = %#v", valuesBySymbol)
	}
}

func TestMethodSourceInvocationContextCarriesStatementAndPartitionMetadata(t *testing.T) {
	env, engine := newRuntimeTest(t)
	schema, err := StructSchema[methodContextRow]("MethodInvocationContextRow")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "method-invocation-by-symbol", Field[methodContextRow, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	var invocations []MethodInvocationContext
	provider := MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		invocations = append(invocations, request.Invocation)
		symbol, _ := request.Trigger.Get("symbol").Any().(string)
		event, eventErr := newEvent(schema, methodContextRow{Symbol: symbol, Value: 1}, request.Now)
		return []Event{event}, eventErr
	})
	method := FromMethodOn[methodContextRow](env, "context-aware-method", "Trade", schema, provider)
	plan, err := env.Build(method.Query(
		StatementName("method-invocation-context"),
		WithContext("method-invocation-by-symbol"),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	if _, err := deployment.Statements()[0].Subscribe(func(context.Context, ResultBatch) error { return nil }); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"A", "B"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(invocations) != 2 {
		t.Fatalf("method invocation contexts = %#v", invocations)
	}
	seenPartitions := map[int]bool{}
	for _, invocation := range invocations {
		if invocation.DeploymentID != deployment.ID() || invocation.StatementName != "method-invocation-context" {
			t.Fatalf("method statement invocation = %#v", invocation)
		}
		if invocation.SourceName != "context-aware-method" || invocation.ContextName != "method-invocation-by-symbol" || invocation.ContextPartitionID < 0 {
			t.Fatalf("method partition invocation = %#v", invocation)
		}
		seenPartitions[invocation.ContextPartitionID] = true
	}
	if len(seenPartitions) != 2 {
		t.Fatalf("method partition ids = %#v", seenPartitions)
	}
}
