package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
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
	Case      string          `json:"case,omitempty"`
	Statement string          `json:"statement,omitempty"`
	Selector  string          `json:"selector,omitempty"`
	Hashes    []int64         `json:"hashes,omitempty"`
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
		case "case":
			if strings.TrimSpace(step.Case) == "" {
				return fmt.Errorf("compat: step %d case has no case", i)
			}
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
		case "snapshot", "snapshot-selector":
			if strings.TrimSpace(step.Statement) == "" {
				return fmt.Errorf("compat: step %d %s has no statement", i, step.Op)
			}
			if step.Op == "snapshot-selector" {
				if strings.TrimSpace(step.Selector) == "" {
					return fmt.Errorf("compat: step %d snapshot-selector has no selector", i)
				}
				switch step.Selector {
				case "all":
				case "hashes":
					if len(step.Hashes) == 0 {
						return fmt.Errorf("compat: step %d hash selector has no hashes", i)
					}
				default:
					return fmt.Errorf("compat: step %d has unsupported selector %q", i, step.Selector)
				}
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
	Case       string            `json:"case,omitempty"`
	Operation  string            `json:"operation"`
	Statement  string            `json:"statement"`
	Sequence   uint64            `json:"sequence"`
	Time       string            `json:"time"`
	New        []ResultRecord    `json:"new,omitempty"`
	Old        []ResultRecord    `json:"old,omitempty"`
	Partitions []PartitionRecord `json:"partitions,omitempty"`
}

type PartitionRecord struct {
	ID         int            `json:"id"`
	Key        string         `json:"key"`
	Properties map[string]any `json:"properties"`
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
	var listenerSequence uint64
	_, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		listenerSequence++
		record := TraceRecord{Operation: "listener", Statement: statement.Name(), Sequence: listenerSequence, Time: batch.Time.UTC().Format(time.RFC3339Nano)}
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
		case "case":
			continue
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
		case "snapshot", "snapshot-selector":
			return trace, fmt.Errorf("compat: Replay snapshot requires a statement resolver; use ReplayWithStatements")
		}
	}
	return trace, nil
}

// ReplayWithStatements is the parity replay entry point for scenarios that
// contain multiple named cases and explicit iterator/snapshot actions.
// Additional statements are subscribed as named listeners so multi-statement
// Java/Go traces share one normalized record stream.
type StatementResolver func(name string) (*esper.Statement, error)

func ReplayWithStatements(ctx context.Context, engine *esper.Engine, statement *esper.Statement, scenario Scenario, decode DecodePayload, resolve StatementResolver, additional ...*esper.Statement) (Trace, error) {
	if err := scenario.Validate(); err != nil {
		return Trace{}, err
	}
	if engine == nil || statement == nil {
		return Trace{}, fmt.Errorf("compat: engine and statement are required")
	}
	trace := Trace{Version: ScenarioVersion, ID: scenario.ID}
	caseName := ""
	listenerSequences := make(map[string]uint64)
	appendBatch := func(caseName, operation string, current *esper.Statement, batch esper.ResultBatch, selector esper.ContextPartitionSelector, sequence uint64) {
		record := TraceRecord{Case: caseName, Operation: operation, Statement: current.Name(), Sequence: sequence, Time: batch.Time.UTC().Format(time.RFC3339Nano)}
		record.New = normalizeResults(batch.New)
		record.Old = normalizeResults(batch.Old)
		if operation == "snapshot" || operation == "snapshot-selector" {
			record.Partitions = normalizePartitions(current.ContextPartitionsWith(selector))
		}
		trace.Records = append(trace.Records, record)
	}
	statements := make([]*esper.Statement, 0, len(additional)+1)
	statements = append(statements, statement)
	for _, additionalStatement := range additional {
		if additionalStatement == nil || additionalStatement.Name() == statement.Name() {
			continue
		}
		duplicate := false
		for _, existing := range statements {
			if existing.Name() == additionalStatement.Name() {
				duplicate = true
				break
			}
		}
		if !duplicate {
			statements = append(statements, additionalStatement)
		}
	}
	for _, current := range statements {
		current := current
		if _, err := current.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			listenerSequences[current.Name()]++
			appendBatch(caseName, "listener", current, batch, nil, listenerSequences[current.Name()])
			return nil
		}); err != nil {
			return Trace{}, err
		}
	}
	for _, step := range scenario.Steps {
		if err := contextErr(ctx); err != nil {
			return trace, err
		}
		if step.Op == "case" {
			caseName = step.Case
			continue
		}
		if step.Case != "" && step.Case != caseName {
			continue
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
		case "snapshot", "snapshot-selector":
			current := statement
			if resolve != nil {
				resolved, err := resolve(step.Statement)
				if err != nil {
					return trace, err
				}
				current = resolved
			} else if step.Statement != statement.Name() {
				return trace, fmt.Errorf("compat: statement %q cannot be resolved", step.Statement)
			}
			var selector esper.ContextPartitionSelector
			if step.Op == "snapshot-selector" {
				switch step.Selector {
				case "all":
					selector = esper.ContextPartitionSelectorAll{}
				case "hashes":
					selector = esper.SelectContextPartitionHashes(step.Hashes...)
				}
			}
			result, err := current.SnapshotWithSelector(ctx, selector)
			if err != nil {
				return trace, err
			}
			batch := result.Batch
			appendBatch(caseName, step.Op, current, batch, selector, 0)
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
			record := ResultRecord{Kind: "row", Fields: make(map[string]any)}
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

// NormalizeResults exposes the protocol normalizer to host-specific runners.
func NormalizeResults(results []esper.Result) []ResultRecord {
	return normalizeResults(results)
}

// normalizePartitionKey converts internal Go keyed-context partition keys to
// the language-neutral "key:<value>" form used by the Java oracle.
func normalizePartitionKey(key string) string {
	const marker = `string:"`
	if !strings.HasPrefix(key, "esper.ValueState:") {
		return key
	}
	index := strings.Index(key, marker)
	if index < 0 {
		return key
	}
	value := key[index+len(marker):]
	if end := strings.IndexByte(value, '"'); end >= 0 {
		value = value[:end]
	}
	return "key:" + value
}

func normalizePartitions(descriptors []esper.ContextPartitionDescriptor) []PartitionRecord {
	if len(descriptors) == 0 {
		return nil
	}
	result := make([]PartitionRecord, 0, len(descriptors))
	for _, descriptor := range descriptors {
		properties := make(map[string]any)
		if hash, ok := descriptor.Property("hash"); ok {
			properties["hash"] = normalizeValue(hash)
		}
		result = append(result, PartitionRecord{ID: descriptor.ID, Key: normalizePartitionKey(descriptor.Key), Properties: properties})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ID != result[j].ID {
			return result[i].ID < result[j].ID
		}
		return result[i].Key < result[j].Key
	})
	return result
}

// NormalizePartitions exposes the stable descriptor projection to host-specific
// runners. The protocol intentionally includes only fields shared by the Java
// and Go context administration APIs.
func NormalizePartitions(descriptors []esper.ContextPartitionDescriptor) []PartitionRecord {
	return normalizePartitions(descriptors)
}

func normalizeMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
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
