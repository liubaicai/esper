package compat

import (
	"encoding/json"
	"testing"
)

func anyModeScenario() Scenario {
	return Scenario{
		Version: ScenarioVersion,
		ID:      "resultset-query-any-mode",
		Steps: []Step{
			{Op: "case", Case: "any-case", Mode: "any"},
			{Op: "send", EventType: "Trade", Payload: json.RawMessage(`{"id":"trade-1","value":10}`)},
		},
	}
}

func anyModeTraces() (Trace, Trace) {
	scenario := anyModeScenario()
	javaTrace := Trace{
		Version: scenario.Version,
		ID:      scenario.ID,
		Records: []TraceRecord{
			{
				Case:      "any-case",
				Operation: "listener",
				Statement: "stmt-any",
				Sequence:  1,
				Time:      "1970-01-01T00:00:00Z",
				New: []ResultRecord{
					{Kind: "row", Fields: map[string]any{"id": "new-a", "value": 1}},
					{Kind: "row", Fields: map[string]any{"id": "new-b", "value": 2}},
				},
				Old: []ResultRecord{
					{Kind: "row", Fields: map[string]any{"id": "old-a", "value": 10}},
					{Kind: "row", Fields: map[string]any{"id": "old-b", "value": 11}},
				},
			},
			{
				Case:      "any-case",
				Operation: "listener",
				Statement: "stmt-any",
				Sequence:  2,
				Time:      "1970-01-01T00:00:01Z",
				New: []ResultRecord{
					{Kind: "row", Fields: map[string]any{"id": "new-c", "value": 3}},
				},
				Old: []ResultRecord{
					{Kind: "row", Fields: map[string]any{"id": "old-c", "value": 12}},
				},
			},
		},
	}
	goTrace := Trace{
		Version: scenario.Version,
		ID:      scenario.ID,
		Records: []TraceRecord{
			{
				Case:      "any-case",
				Operation: "listener",
				Statement: "stmt-any",
				Sequence:  1,
				Time:      "1970-01-01T00:00:00Z",
				New: []ResultRecord{
					{Kind: "row", Fields: map[string]any{"id": "new-b", "value": 2}},
					{Kind: "row", Fields: map[string]any{"id": "new-a", "value": 1}},
				},
				Old: []ResultRecord{
					{Kind: "row", Fields: map[string]any{"id": "old-b", "value": 11}},
					{Kind: "row", Fields: map[string]any{"id": "old-a", "value": 10}},
				},
			},
			{
				Case:      "any-case",
				Operation: "listener",
				Statement: "stmt-any",
				Sequence:  2,
				Time:      "1970-01-01T00:00:01Z",
				New: []ResultRecord{
					{Kind: "row", Fields: map[string]any{"id": "new-c", "value": 3}},
				},
				Old: []ResultRecord{
					{Kind: "row", Fields: map[string]any{"id": "old-c", "value": 12}},
				},
			},
		},
	}
	return javaTrace, goTrace
}

func newAnyModeEvidence(t *testing.T, javaTrace, goTrace Trace) DifferentialEvidence {
	t.Helper()
	evidence, err := NewDifferentialEvidence(
		"java-commit-resultset-query",
		[]string{"runtime-resultset-query"},
		[]string{"ResultSetQueryTypeWTimeBatch.java"},
		[]string{"resultset-query-any-mode"},
		anyModeScenario(),
		javaTrace,
		goTrace,
	)
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}

func TestNewDifferentialEvidenceAnyModeCanonicalizesListenerRowOrder(t *testing.T) {
	scenario := anyModeScenario()
	if err := scenario.Validate(); err != nil {
		t.Fatalf("scenario validation failed: %v", err)
	}
	javaTrace, goTrace := anyModeTraces()
	evidence := newAnyModeEvidence(t, javaTrace, goTrace)
	if evidence.Status != "passing" {
		t.Fatalf("row permutation status = %q, differences = %#v", evidence.Status, evidence.Differences)
	}
	if len(evidence.Differences) != 0 {
		t.Fatalf("row permutation differences = %#v", evidence.Differences)
	}
}

func TestNewDifferentialEvidenceAnyModeKeepsListenerBoundariesStrict(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(Trace) Trace
	}{
		{
			name: "record-order",
			mutate: func(trace Trace) Trace {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
				return trace
			},
		},
		{
			name: "row-value",
			mutate: func(trace Trace) Trace {
				trace.Records[0].New[0].Fields["value"] = 99
				return trace
			},
		},
		{
			name: "row-count",
			mutate: func(trace Trace) Trace {
				trace.Records[0].New = append(trace.Records[0].New, ResultRecord{
					Kind: "row", Fields: map[string]any{"id": "new-extra", "value": 4},
				})
				return trace
			},
		},
		{
			name: "batch-boundary",
			mutate: func(trace Trace) Trace {
				trace.Records[0].New = trace.Records[0].New[:1]
				trace.Records[1].New = append(trace.Records[1].New, ResultRecord{
					Kind: "row", Fields: map[string]any{"id": "new-b", "value": 2},
				})
				return trace
			},
		},
	}

	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			javaTrace, goTrace := anyModeTraces()
			evidence := newAnyModeEvidence(t, javaTrace, mutation.mutate(goTrace))
			if evidence.Status != "different" {
				t.Fatalf("mutation status = %q, differences = %#v", evidence.Status, evidence.Differences)
			}
			if len(evidence.Differences) == 0 {
				t.Fatalf("mutation produced no differences: %#v", evidence)
			}
		})
	}
}
