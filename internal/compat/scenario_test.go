package compat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	esper "github.com/liubaicai/esper"
)

type replayTrade struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

func TestReplayProducesNormalizedTrace(t *testing.T) {
	scenario, err := LoadScenario(strings.NewReader(`{
      "version":"esper-parity/v1",
      "id":"replay-test",
      "steps":[
        {"op":"send","eventType":"Trade","payload":{"symbol":"A","price":11}},
        {"op":"send","eventType":"Trade","payload":{"symbol":"B","price":12}},
        {"op":"send","eventType":"Trade","payload":{"symbol":"C","price":13}}
      ]
    }`))
	if err != nil {
		t.Fatal(err)
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[replayTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	stream := esper.From[replayTrade](env, "Trade").Window(esper.LengthWindow(2))
	plan, err := env.Build(stream.Query(esper.StatementName("replay"), esper.WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	trace, err := Replay(context.Background(), engine, deployment.Statements()[0], scenario, func(step Step) (any, error) {
		var value replayTrade
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, err
		}
		return value, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(trace.Records) != 3 || len(trace.Records[2].Old) != 1 {
		t.Fatalf("trace = %#v", trace)
	}
	if trace.Records[2].Old[0].Fields["symbol"] != "A" {
		t.Fatalf("old record = %#v", trace.Records[2].Old[0])
	}
}

func TestDiffTracesReportsStableStructuralPaths(t *testing.T) {
	want := Trace{
		Version: ScenarioVersion,
		ID:      "diff",
		Records: []TraceRecord{{Statement: "s0", Sequence: 1, Time: "t", New: []ResultRecord{{Kind: "row", Fields: map[string]any{"value": 1}}}}},
	}
	got := Trace{
		Version: ScenarioVersion,
		ID:      "diff",
		Records: []TraceRecord{{Statement: "s0", Sequence: 2, Time: "t", New: []ResultRecord{{Kind: "row", Fields: map[string]any{"value": 2}}}}},
	}
	differences := DiffTraces(want, got)
	if len(differences) != 2 {
		t.Fatalf("trace differences = %#v", differences)
	}
	if differences[0].Path != "records[0].sequence" || differences[1].Path != "records[0].new[0].fields" {
		t.Fatalf("trace difference paths = %#v", differences)
	}
	equal, err := EqualTrace(want, want)
	if err != nil || !equal {
		t.Fatalf("equal trace = %v, err=%v", equal, err)
	}
}
