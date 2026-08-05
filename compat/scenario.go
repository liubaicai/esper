package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	esper "github.com/liubaicai/esper"
)

const ScenarioVersion = "esper-parity/v1"

// Scenario is the versioned, language-neutral action stream shared by the
// Java oracle and Go runner. Payload decoding remains a host hook so typed
// Java beans, Go structs, XML DOM and Avro are not forced into JSON-only data.
type Scenario struct {
	Version string `json:"version"`
	ID      string `json:"id"`
	Steps   []Step `json:"steps"`
}

type Step struct {
	Op        string          `json:"op"`
	EventType string          `json:"eventType,omitempty"`
	At        string          `json:"at,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func LoadScenario(reader io.Reader) (Scenario, error) {
	var scenario Scenario
	if err := json.NewDecoder(reader).Decode(&scenario); err != nil {
		return Scenario{}, fmt.Errorf("compat: decode scenario: %w", err)
	}
	if err := scenario.Validate(); err != nil {
		return Scenario{}, err
	}
	return scenario, nil
}

func (s Scenario) Validate() error {
	if s.Version != ScenarioVersion {
		return fmt.Errorf("compat: unsupported scenario version %q", s.Version)
	}
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("compat: scenario id is required")
	}
	if len(s.Steps) == 0 {
		return fmt.Errorf("compat: scenario %q has no steps", s.ID)
	}
	for i, step := range s.Steps {
		switch step.Op {
		case "send":
			if strings.TrimSpace(step.EventType) == "" {
				return fmt.Errorf("compat: step %d send has no eventType", i)
			}
			if len(step.Payload) == 0 {
				return fmt.Errorf("compat: step %d send has no payload", i)
			}
		case "advance-time":
			if _, err := time.Parse(time.RFC3339Nano, step.At); err != nil {
				return fmt.Errorf("compat: step %d invalid time %q: %w", i, step.At, err)
			}
		default:
			return fmt.Errorf("compat: step %d has unsupported op %q", i, step.Op)
		}
	}
	return nil
}

type Trace struct {
	Version string        `json:"version"`
	ID      string        `json:"id"`
	Records []TraceRecord `json:"records"`
}

type TraceRecord struct {
	Statement string         `json:"statement"`
	Sequence  uint64         `json:"sequence"`
	Time      string         `json:"time"`
	New       []ResultRecord `json:"new,omitempty"`
	Old       []ResultRecord `json:"old,omitempty"`
}

type ResultRecord struct {
	Kind   string         `json:"kind"`
	Type   string         `json:"type,omitempty"`
	Fields map[string]any `json:"fields"`
}

// DecodePayload converts a scenario send payload into a typed host object.
type DecodePayload func(Step) (any, error)

// Replay executes the action stream against a deployed statement and captures
// normalized output. The same Scenario and host setup are intended to be used
// by the Java oracle adapter.
func Replay(ctx context.Context, engine *esper.Engine, statement *esper.Statement, scenario Scenario, decode DecodePayload) (Trace, error) {
	if err := scenario.Validate(); err != nil {
		return Trace{}, err
	}
	if engine == nil || statement == nil {
		return Trace{}, fmt.Errorf("compat: engine and statement are required")
	}
	trace := Trace{Version: ScenarioVersion, ID: scenario.ID}
	var mu sync.Mutex
	_, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		record := TraceRecord{Statement: statement.Name(), Sequence: batch.Sequence, Time: batch.Time.UTC().Format(time.RFC3339Nano)}
		record.New = normalizeResults(batch.New)
		record.Old = normalizeResults(batch.Old)
		mu.Lock()
		trace.Records = append(trace.Records, record)
		mu.Unlock()
		return nil
	})
	if err != nil {
		return Trace{}, err
	}
	for _, step := range scenario.Steps {
		if err := contextErr(ctx); err != nil {
			return trace, err
		}
		switch step.Op {
		case "send":
			if decode == nil {
				return trace, fmt.Errorf("compat: no payload decoder for send step")
			}
			payload, err := decode(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "advance-time":
			at, _ := time.Parse(time.RFC3339Nano, step.At)
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return trace, err
			}
		}
	}
	return trace, nil
}

func normalizeResults(results []esper.Result) []ResultRecord {
	if len(results) == 0 {
		return nil
	}
	normalized := make([]ResultRecord, 0, len(results))
	for _, result := range results {
		if event, ok := result.Event(); ok {
			record := ResultRecord{Kind: "event", Type: event.TypeName(), Fields: make(map[string]any)}
			for _, field := range event.Schema().Fields() {
				record.Fields[field.Name] = normalizeValue(event.Get(field.Name))
			}
			normalized = append(normalized, record)
			continue
		}
		if row, ok := result.Row(); ok {
			record := ResultRecord{Kind: "row", Fields: make(map[string]any)}
			for _, field := range row.Schema().Fields() {
				record.Fields[field.Name] = normalizeValue(row.Get(field.Name))
			}
			normalized = append(normalized, record)
		}
	}
	return normalized
}

func normalizeValue(value esper.Value) any {
	if value.IsMissing() {
		return map[string]any{"state": "missing"}
	}
	if value.IsNull() {
		return map[string]any{"state": "null"}
	}
	return value.Any()
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
