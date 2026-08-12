package esper_test

import (
	"context"
	"math/big"
	"testing"

	esper "github.com/liubaicai/esper"
)

type facadeTrade struct {
	Price float64 `esper:"price"`
}

func TestPublicFacadeBuildDeployAndSend(t *testing.T) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[facadeTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	stream := esper.From[facadeTrade](env, "Trade").Filter(
		esper.Greater[float64](esper.Field[facadeTrade, float64]("price"), esper.Literal(10.0)),
	)
	plan, err := env.Build(stream.Query(esper.StatementName("facade")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), facadeTrade{Price: 11}); err != nil {
		t.Fatal(err)
	}
	if got := len(deployment.Statements()); got != 1 {
		t.Fatalf("statements = %d, want 1", got)
	}
}

func TestPublicFacadeMutableDecimalContextRemainsPassByValue(t *testing.T) {
	original := esper.MathContextDECIMAL32
	t.Cleanup(func() { esper.MathContextDECIMAL32 = original })
	esper.MathContextDECIMAL32 = esper.DecimalMathContext{Precision: 3, Rounding: esper.DecimalRoundDown}
	if got := esper.MathContextDECIMAL32.Precision; got != 3 {
		t.Fatalf("decimal context precision = %d, want 3", got)
	}
	expression := esper.RoundDecimal(esper.Literal(*big.NewRat(2469, 2000)), esper.MathContextDECIMAL32)
	compiled, err := esper.CompileExpression[big.Rat](esper.NewEnvironment(), expression)
	if err != nil {
		t.Fatal(err)
	}
	value, state, err := compiled.Evaluate()
	if err != nil || state != esper.ValuePresent {
		t.Fatalf("evaluate state = %v, err = %v", state, err)
	}
	if got := value.FloatString(2); got != "1.23" {
		t.Fatalf("rounded decimal = %q, want 1.23", got)
	}
}
