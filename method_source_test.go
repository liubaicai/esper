package esper

import (
	"context"
	"sync"
	"testing"
)

type methodSourceRow struct {
	Symbol string `esper:"symbol"`
	Value  int    `esper:"value"`
}

type methodSourceOtherTrigger struct {
	Symbol string `esper:"symbol"`
}

type recordingMethodProvider struct {
	mu       sync.Mutex
	schema   Schema
	requests []MethodRequest
}

func (p *recordingMethodProvider) Poll(ctx context.Context, request MethodRequest) ([]Event, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.requests = append(p.requests, request)
	p.mu.Unlock()

	symbol := "snapshot"
	value := 7
	if request.Trigger.TypeName() != "" {
		if current, ok := request.Trigger.Get("symbol").Any().(string); ok {
			symbol = current
		}
		if price, ok := request.Trigger.Get("price").Any().(float64); ok {
			value = int(price)
		}
	}
	if bonus, ok := request.Parameters["bonus"].Any().(int); ok {
		value += bonus
	}
	event, err := newEvent(p.schema, methodSourceRow{Symbol: symbol, Value: value}, request.Now)
	if err != nil {
		return nil, err
	}
	return []Event{event}, nil
}

func (p *recordingMethodProvider) Requests() []MethodRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]MethodRequest(nil), p.requests...)
}

func TestMethodSourcePollsOnTypedTriggerAndCarriesParameters(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[methodSourceOtherTrigger](env, "OtherTrigger"); err != nil {
		t.Fatal(err)
	}
	methodSchema, err := StructSchema[methodSourceRow]("MethodRow")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(methodSchema); err != nil {
		t.Fatal(err)
	}
	provider := &recordingMethodProvider{schema: methodSchema}
	minimum := Parameter[int]("minimum")
	bonus := Parameter[int]("bonus")
	method := FromMethodOn[methodSourceRow](env, "method", "Trade", methodSchema, provider)
	plan, err := env.Build(Select(method.Filter(
		Greater[int](Field[methodSourceRow, int]("value"), Add[int](minimum, bonus)),
	),
		Alias("symbol", Field[methodSourceRow, string]("symbol")),
		Alias("value", Field[methodSourceRow, int]("value")),
	).Query(StatementName("method-source-trigger")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.DeployWithParameters(context.Background(), plan, ParameterValues{
		"minimum": 10,
		"bonus":   5,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("method source result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), methodSourceOtherTrigger{Symbol: "ignored"}); err != nil {
		t.Fatal(err)
	}
	if len(provider.Requests()) != 0 {
		t.Fatalf("method source called for non-trigger event: %#v", provider.Requests())
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 12}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("symbol").Any() != "A" || rows[0].Get("value").Any() != 17 {
		t.Fatalf("method source rows = %#v", rows)
	}
	requests := provider.Requests()
	if len(requests) != 1 || requests[0].Trigger.TypeName() != "Trade" {
		t.Fatalf("method source trigger requests = %#v", requests)
	}
	if requests[0].Parameters["bonus"].Any() != 5 || requests[0].Parameters["minimum"].Any() != 10 {
		t.Fatalf("method source parameter snapshot = %#v", requests[0].Parameters)
	}
}

func TestMethodSourceParticipatesInJoin(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	methodSchema, err := StructSchema[methodSourceRow]("MethodJoinRow")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(methodSchema); err != nil {
		t.Fatal(err)
	}
	provider := &recordingMethodProvider{schema: methodSchema}
	trades := From[runtimeTestTrade](env, "Trade")
	method := FromMethodOn[methodSourceRow](env, "method-join", "Trade", methodSchema, provider)
	plan, err := env.Build(Join(trades, method, OnEqual(
		Field[runtimeTestTrade, string]("symbol"),
		Field[methodSourceRow, string]("symbol"),
	)).Select(
		SelectLeft("trade", Field[runtimeTestTrade, string]("symbol")),
		SelectRight("value", Field[methodSourceRow, int]("value")),
	).Query(StatementName("method-source-join")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
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
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 12}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("trade").Any() != "A" || rows[0].Get("value").Any() != 12 {
		t.Fatalf("method source join rows = %#v", rows)
	}
	if requests := provider.Requests(); len(requests) != 1 || requests[0].Trigger.TypeName() != "Trade" {
		t.Fatalf("method source join requests = %#v", requests)
	}
}

func TestMethodSourceFireAndForgetUsesEmptyTriggerSnapshot(t *testing.T) {
	env := NewEnvironment()
	methodSchema, err := StructSchema[methodSourceRow]("MethodFAFRow")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(methodSchema); err != nil {
		t.Fatal(err)
	}
	provider := &recordingMethodProvider{schema: methodSchema}
	wanted := Parameter[int]("wanted")
	method := FromMethod[methodSourceRow](env, "method-faf", methodSchema, provider)
	plan, err := env.Build(Select(method.Filter(
		Equal[int](Field[methodSourceRow, int]("value"), wanted),
	),
		Alias("symbol", Field[methodSourceRow, string]("symbol")),
		Alias("value", Field[methodSourceRow, int]("value")),
	).Query(StatementName("method-source-faf")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(env).ExecuteFireAndForgetWithParameters(context.Background(), plan, ParameterValues{"wanted": 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 1 {
		t.Fatalf("method source FAF results = %#v", result.Results())
	}
	row, ok := result.Results()[0].Row()
	if !ok || row.Get("symbol").Any() != "snapshot" || row.Get("value").Any() != 7 {
		t.Fatalf("method source FAF row = %#v", result.Results())
	}
	requests := provider.Requests()
	if len(requests) != 1 || requests[0].Trigger.TypeName() != "" {
		t.Fatalf("method source FAF trigger = %#v", requests)
	}
	if requests[0].Parameters["wanted"].Any() != 7 {
		t.Fatalf("method source FAF parameters = %#v", requests[0].Parameters)
	}
}

func TestMethodSourceValidationRequiresProviderAndKnownTrigger(t *testing.T) {
	env := NewEnvironment()
	methodSchema, err := StructSchema[methodSourceRow]("MethodValidationRow")
	if err != nil {
		t.Fatal(err)
	}
	var missingProvider MethodProvider
	if _, err := env.Build(FromMethod[methodSourceRow](env, "missing-provider", methodSchema, missingProvider).Query(StatementName("method-missing-provider"))); err == nil {
		t.Fatal("method source without provider was accepted")
	}
	provider := MethodProviderFunc(func(context.Context, MethodRequest) ([]Event, error) { return nil, nil })
	if _, err := env.Build(FromMethodOn[methodSourceRow](env, "unknown-trigger", "MissingTrigger", methodSchema, provider).Query(StatementName("method-unknown-trigger"))); err == nil {
		t.Fatal("method source with unknown trigger was accepted")
	}
}
