package esper

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type clientRuntimeInvalidAggregateState struct{}

func (*clientRuntimeInvalidAggregateState) Enter(Value) { panic("Sample exception") }
func (*clientRuntimeInvalidAggregateState) Leave(Value) {}
func (*clientRuntimeInvalidAggregateState) Value() (int, bool) {
	return 0, false
}
func (*clientRuntimeInvalidAggregateState) Clear() {}

func TestClientRuntimeExceptionHandlerInvalidAggregateParity(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if err := RegisterAggregatePluginFactory[int](env, "myinvalidagg", func(AggregatePluginFactoryContext) AggregatePluginState[int] {
		return &clientRuntimeInvalidAggregateState{}
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Aggregate(
		Alias("value", PluginAggregateFactoryRef[int](env, "myinvalidagg", nil)),
	).Query(StatementName("ABCName")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}

	err = engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E1"})
	if !errors.Is(err, ErrorState) {
		t.Fatalf("invalid aggregate send error = %v, want %s", err, ErrorState)
	}
	for _, fragment := range []string{"myinvalidagg", "ABCName", "Sample exception"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("invalid aggregate send error = %q, want fragment %q", err, fragment)
		}
	}
}
