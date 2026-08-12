package esper

import (
	"context"
	"reflect"
	"testing"
)

type insertChainMarketData struct {
	Symbol string `esper:"symbol"`
}

func TestInsertIntoChainPropagatesThroughFourChainedRoutes(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertChainMarketData](env, "ChainMarketData"); err != nil {
		t.Fatal(err)
	}
	fields := []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("val", reflect.TypeOf(int64(0))),
	}
	for _, name := range []string{"ChainS0", "ChainS1", "ChainS2", "ChainS3"} {
		if _, err := RegisterMap(env, name, fields); err != nil {
			t.Fatal(err)
		}
	}

	symbol := Field[insertChainMarketData, string]("symbol")
	queries := []Query{
		Select(
			From[insertChainMarketData](env, "ChainMarketData"),
			Alias("symbol", symbol),
			Alias("val", Literal(int64(0))),
		).InsertInto("ChainS0", StatementName("chain-s0")),
		FromAny(env, "ChainS0").Select(
			Alias("symbol", Field[Event, string]("symbol")),
			Alias("val", Literal(int64(1))),
		).InsertInto("ChainS1", StatementName("chain-s1")),
		FromAny(env, "ChainS1").Select(
			Alias("symbol", Field[Event, string]("symbol")),
			Alias("val", Literal(int64(2))),
		).InsertInto("ChainS2", StatementName("chain-s2")),
		FromAny(env, "ChainS2").Select(
			Alias("symbol", Field[Event, string]("symbol")),
			Alias("val", Literal(int64(3))),
		).InsertInto("ChainS3", StatementName("chain-s3")),
	}
	plans := make([]Plan, 0, len(queries)+1)
	for _, query := range queries {
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	consumerPlan, err := env.Build(FromAny(env, "ChainS3").Query(StatementName("chain-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	plans = append(plans, consumerPlan)

	engine := NewEngine(env)
	consumerDeployment := (*Deployment)(nil)
	for index, plan := range plans {
		deployed, deployErr := engine.Deploy(context.Background(), plan)
		if deployErr != nil {
			t.Fatal(deployErr)
		}
		if index == len(plans)-1 {
			consumerDeployment = deployed
		}
	}
	var received []Event
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				received = append(received, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), insertChainMarketData{Symbol: "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 {
		t.Fatalf("four-stage chain received = %#v", received)
	}
	if received[0].Schema().Name() != "ChainS3" || received[0].Get("symbol").Any() != "E1" || received[0].Get("val").Any() != int64(3) {
		t.Fatalf("four-stage chain event = %#v", received[0])
	}
}
