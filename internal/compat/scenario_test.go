package compat

import (
	"bytes"
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
		Records: []TraceRecord{{Case: "case-a", Operation: "snapshot", Statement: "s0", Sequence: 1, Time: "t", New: []ResultRecord{{Kind: "row", Fields: map[string]any{"value": 1}}}, Partitions: []PartitionRecord{{ID: 1, Key: "hash:1", Properties: map[string]any{"hash": int64(1)}}}}},
	}
	got := Trace{
		Version: ScenarioVersion,
		ID:      "diff",
		Records: []TraceRecord{{Case: "case-b", Operation: "listener", Statement: "s0", Sequence: 2, Time: "t", New: []ResultRecord{{Kind: "row", Fields: map[string]any{"value": 2}}}, Partitions: []PartitionRecord{{ID: 2, Key: "hash:2", Properties: map[string]any{"hash": int64(2)}}}}},
	}
	differences := DiffTraces(want, got)
	if len(differences) != 7 {
		t.Fatalf("trace differences = %#v", differences)
	}
	if differences[0].Path != "records[0].case" || differences[1].Path != "records[0].operation" || differences[2].Path != "records[0].sequence" || differences[3].Path != "records[0].new[0].fields" || differences[4].Path != "records[0].partitions[0].id" {
		t.Fatalf("trace difference paths = %#v", differences)
	}
	equal, err := EqualTrace(want, want)
	if err != nil || !equal {
		t.Fatalf("equal trace = %v, err=%v", equal, err)
	}
}

func TestDiffTracesDetectsValueOrderAndBoundaryMutations(t *testing.T) {
	base := Trace{
		Version: ScenarioVersion,
		ID:      "mutation",
		Records: []TraceRecord{
			{Operation: "listener", Statement: "s0", Sequence: 1, Time: "1970-01-01T00:00:00Z", New: []ResultRecord{{Kind: "row", Fields: map[string]any{"value": 1}}}},
			{Operation: "listener", Statement: "s0", Sequence: 2, Time: "1970-01-01T00:00:00Z", New: []ResultRecord{{Kind: "row", Fields: map[string]any{"value": 2}}}},
		},
	}
	tests := []struct {
		name   string
		mutate func(*Trace)
		path   string
	}{
		{name: "value", mutate: func(trace *Trace) { trace.Records[0].New[0].Fields["value"] = 99 }, path: "records[0].new[0].fields"},
		{name: "order", mutate: func(trace *Trace) { trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0] }, path: "records[0].sequence"},
		{name: "boundary", mutate: func(trace *Trace) {
			trace.Records[0].Old = []ResultRecord{{Kind: "row", Fields: map[string]any{"value": 0}}}
		}, path: "records[0].old.length"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := base
			mutated.Records = append([]TraceRecord(nil), base.Records...)
			mutated.Records[0].New = append([]ResultRecord(nil), base.Records[0].New...)
			mutated.Records[0].New[0].Fields = map[string]any{"value": base.Records[0].New[0].Fields["value"]}
			test.mutate(&mutated)
			differences := DiffTraces(base, mutated)
			found := false
			for _, difference := range differences {
				if difference.Path == test.path {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("differences = %#v, missing %q", differences, test.path)
			}
		})
	}
}

func TestDifferentialEvidenceRejectsMutationWithoutUpdatedDiff(t *testing.T) {
	scenario := Scenario{
		Version: ScenarioVersion,
		ID:      "mutation-evidence",
		Steps: []Step{{
			Op:        "send",
			EventType: "Trade",
			Payload:   json.RawMessage(`{"value":1}`),
		}},
	}
	base := Trace{
		Version: ScenarioVersion,
		ID:      scenario.ID,
		Records: []TraceRecord{{
			Operation: "listener",
			Statement: "s0",
			Sequence:  1,
			Time:      "1970-01-01T00:00:00Z",
			New: []ResultRecord{{
				Kind:   "row",
				Fields: map[string]any{"value": 1},
			}},
		}},
	}
	evidence, err := NewDifferentialEvidence("java", []string{"runtime"}, []string{"source"}, []string{"execution"}, scenario, base, base)
	if err != nil {
		t.Fatal(err)
	}
	evidence.JavaTrace.Records[0].New[0].Fields["value"] = 2
	if err := evidence.Validate(); err == nil || !strings.Contains(err.Error(), "differences are stale") {
		t.Fatalf("mutated evidence validation error = %v", err)
	}
}

func TestDiffTracesMutationMatrixCoversNullAndTimeBoundaries(t *testing.T) {
	base := Trace{
		Version: ScenarioVersion,
		ID:      "mutation-matrix",
		Records: []TraceRecord{{
			Operation: "listener",
			Statement: "s0",
			Sequence:  1,
			Time:      "1970-01-01T00:00:00Z",
			New: []ResultRecord{{
				Kind:   "row",
				Fields: map[string]any{"value": map[string]any{"state": "null"}},
			}},
		}},
	}
	tests := []struct {
		name   string
		mutate func(*Trace)
		path   string
	}{
		{
			name: "null-state",
			mutate: func(trace *Trace) {
				trace.Records[0].New[0].Fields["value"] = map[string]any{"state": "missing"}
			},
			path: "records[0].new[0].fields",
		},
		{
			name: "time-boundary",
			mutate: func(trace *Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:00.001Z"
			},
			path: "records[0].time",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := base
			mutated.Records = append([]TraceRecord(nil), base.Records...)
			mutated.Records[0].New = append([]ResultRecord(nil), base.Records[0].New...)
			mutated.Records[0].New[0].Fields = map[string]any{"value": base.Records[0].New[0].Fields["value"]}
			test.mutate(&mutated)
			differences := DiffTraces(base, mutated)
			found := false
			for _, difference := range differences {
				if difference.Path == test.path {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("differences = %#v, missing %q", differences, test.path)
			}
		})
	}
}

func TestNewDifferentialEvidenceCanonicalizesNumericTraceValues(t *testing.T) {
	scenario := Scenario{Version: ScenarioVersion, ID: "evidence", Steps: []Step{{Op: "send", EventType: "Trade", Payload: json.RawMessage(`{"value":1}`)}}}
	javaTrace := Trace{Version: ScenarioVersion, ID: scenario.ID, Records: []TraceRecord{{Operation: "listener", Statement: "s0", Sequence: 1, Time: "1970-01-01T00:00:00Z", New: []ResultRecord{{Kind: "row", Fields: map[string]any{"value": int64(1)}}}}}}
	goTrace := Trace{Version: ScenarioVersion, ID: scenario.ID, Records: []TraceRecord{{Operation: "listener", Statement: "s0", Sequence: 1, Time: "1970-01-01T00:00:00Z", New: []ResultRecord{{Kind: "row", Fields: map[string]any{"value": int(1)}}}}}}
	evidence, err := NewDifferentialEvidence("java", []string{"runtime"}, []string{"source"}, []string{"execution"}, scenario, javaTrace, goTrace)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
	if err := evidence.Validate(); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadDifferentialEvidence(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != "passing" || len(loaded.Differences) != 0 {
		t.Fatalf("loaded evidence = %#v", loaded)
	}
}

func TestScenarioAcceptsStatementlessLifecycleCleanup(t *testing.T) {
	scenario, err := LoadScenario(strings.NewReader(`{
      "version":"esper-parity/v1",
      "id":"lifecycle",
      "steps":[
        {"op":"case","case":"one"},
        {"op":"deploy","statement":"s0"},
        {"op":"undeploy"},
        {"op":"undeploy-all"}
      ]
    }`))
	if err != nil {
		t.Fatalf("lifecycle scenario rejected: %v", err)
	}
	if len(scenario.Steps) != 4 {
		t.Fatalf("steps = %d, want 4", len(scenario.Steps))
	}
}
