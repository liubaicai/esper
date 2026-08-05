package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/compat"
)

type trade struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

func main() {
	path := flag.String("scenario", "testdata/parity/stage1-length-window.json", "scenario JSON file")
	flag.Parse()
	file, err := os.Open(*path)
	if err != nil {
		fail(err)
	}
	defer file.Close()
	scenario, err := compat.LoadScenario(file)
	if err != nil {
		fail(err)
	}

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[trade](env, "Trade"); err != nil {
		fail(err)
	}
	stream := esper.From[trade](env, "Trade").
		Filter(esper.Greater[float64](esper.Field[trade, float64]("price"), esper.Literal(10.0))).
		Window(esper.LengthWindow(2))
	plan, err := env.Build(stream.Query(esper.StatementName("parity-stage1"), esper.WithOldStream()))
	if err != nil {
		fail(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		fail(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	trace, err := compat.Replay(context.Background(), engine, deployment.Statements()[0], scenario, func(step compat.Step) (any, error) {
		var value trade
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode Trade: %w", err)
		}
		return value, nil
	})
	if err != nil {
		fail(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(trace); err != nil {
		fail(err)
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
