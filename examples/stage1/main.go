package main

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
)

type Trade struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

func main() {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[Trade](env, "Trade"); err != nil {
		panic(err)
	}

	stream := esper.From[Trade](env, "Trade").
		Filter(esper.Greater[float64](
			esper.Field[Trade, float64]("price"),
			esper.Literal(100.0),
		)).
		Window(esper.LengthWindow(10))
	plan, err := env.Build(stream.Query(
		esper.StatementName("high-value-trades"),
		esper.WithOldStream(),
	))
	if err != nil {
		panic(err)
	}

	engine := env.NewEngine(esper.WithStartTime(time.Unix(0, 0).UTC()))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		panic(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	_, err = deployment.Statements()[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		fmt.Printf("new=%d old=%d at=%s\n", len(batch.New), len(batch.Old), batch.Time.Format(time.RFC3339Nano))
		return nil
	})
	if err != nil {
		panic(err)
	}
	if err := engine.SendEvent(context.Background(), Trade{Symbol: "ESPER", Price: 101}); err != nil {
		panic(err)
	}
}
