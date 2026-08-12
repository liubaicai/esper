// Package parity implements the parity scenario runner command.
package parity

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type trade struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

// Run executes the parity command and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("parity", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("scenario", "testdata/parity/stage1-length-window.json", "scenario JSON file")
	if err := flags.Parse(args); err == flag.ErrHelp {
		return 0
	} else if err != nil {
		return 2
	}
	file, err := os.Open(*path)
	if err != nil {
		return fail(stderr, err)
	}
	defer file.Close()
	scenario, err := compat.LoadScenario(file)
	if err != nil {
		return fail(stderr, err)
	}

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[trade](env, "Trade"); err != nil {
		return fail(stderr, err)
	}
	stream := esper.From[trade](env, "Trade").
		Filter(esper.Greater[float64](esper.Field[trade, float64]("price"), esper.Literal(10.0))).
		Window(esper.LengthWindow(2))
	plan, err := env.Build(stream.Query(esper.StatementName("parity-stage1"), esper.WithOldStream()))
	if err != nil {
		return fail(stderr, err)
	}
	engine := env.NewEngine()
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		return fail(stderr, err)
	}

	trace, err := compat.Replay(context.Background(), engine, deployment.Statements()[0], scenario, func(step compat.Step) (any, error) {
		var value trade
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode Trade: %w", err)
		}
		return value, nil
	})
	if err != nil {
		return fail(stderr, err)
	}
	if err := json.NewEncoder(stdout).Encode(trace); err != nil {
		return fail(stderr, err)
	}
	return 0
}

func fail(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintln(stderr, err)
	return 1
}
