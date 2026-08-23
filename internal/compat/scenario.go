package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
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
	Op             string            `json:"op"`
	Case           string            `json:"case,omitempty"`
	Statement      string            `json:"statement,omitempty"`
	Selector       string            `json:"selector,omitempty"`
	Hashes         []int64           `json:"hashes,omitempty"`
	IDs            []int             `json:"ids,omitempty"`
	FilterProperty string            `json:"filterProperty,omitempty"`
	FilterValue    string            `json:"filterValue,omitempty"`
	ExpectError    string            `json:"expectError,omitempty"`
	EventType      string            `json:"eventType,omitempty"`
	At             string            `json:"at,omitempty"`
	Name           string            `json:"name,omitempty"`
	Source         string            `json:"source,omitempty"`
	PropertyOrder  []string          `json:"propertyOrder,omitempty"`
	PropertyTypes  map[string]string `json:"propertyTypes,omitempty"`
	Payload        json.RawMessage   `json:"payload,omitempty"`
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
		case "set-variable":
			if strings.TrimSpace(step.Name) == "" {
				return fmt.Errorf("compat: step %d set-variable has no name", i)
			}
			if len(step.Payload) == 0 {
				return fmt.Errorf("compat: step %d set-variable has no payload", i)
			}
		case "advance-time":
			if _, err := time.Parse(time.RFC3339Nano, step.At); err != nil {
				return fmt.Errorf("compat: step %d invalid time %q: %w", i, step.At, err)
			}
		case "faf", "deploy":
			if strings.TrimSpace(step.Statement) == "" {
				return fmt.Errorf("compat: step %d %s has no statement", i, step.Op)
			}
		case "read-variable":
			if strings.TrimSpace(step.Name) == "" {
				return fmt.Errorf("compat: step %d read-variable has no name", i)
			}
		case "send-error":
			if strings.TrimSpace(step.EventType) == "" {
				return fmt.Errorf("compat: step %d send-error has no eventType", i)
			}
			if len(step.Payload) == 0 {
				return fmt.Errorf("compat: step %d send-error has no payload", i)
			}
		case "types":
			if strings.TrimSpace(step.Statement) == "" {
				return fmt.Errorf("compat: step %d types has no statement", i)
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
				case "ids":
					if len(step.IDs) == 0 {
						return fmt.Errorf("compat: step %d ids selector has no ids", i)
					}
				case "filtered":
					if strings.TrimSpace(step.FilterProperty) == "" {
						return fmt.Errorf("compat: step %d filtered selector has no filter property", i)
					}
				case "segmented":
					if strings.TrimSpace(step.ExpectError) == "" {
						return fmt.Errorf("compat: step %d segmented selector requires expectError", i)
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
	Statement  string            `json:"statement,omitempty"`
	Sequence   uint64            `json:"sequence"`
	Time       string            `json:"time,omitempty"`
	New        []ResultRecord    `json:"new,omitempty"`
	Old        []ResultRecord    `json:"old,omitempty"`
	Partitions []PartitionRecord `json:"partitions,omitempty"`
	Name       string            `json:"name,omitempty"`
	Value      any               `json:"value,omitempty"`
}

// formatTraceTime matches Java's Instant.toString() representation used by
// the oracle trace writers: RFC3339 with fractional seconds emitted in groups
// of three digits and trailing zero groups omitted.
func formatTraceTime(value time.Time) string {
	value = value.UTC()
	if value.Nanosecond() == 0 {
		return value.Format(time.RFC3339)
	}
	if value.Nanosecond()%1000000 == 0 {
		return value.Format("2006-01-02T15:04:05.000Z07:00")
	}
	if value.Nanosecond()%1000 == 0 {
		return value.Format("2006-01-02T15:04:05.000000Z07:00")
	}
	return value.Format("2006-01-02T15:04:05.000000000Z07:00")
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
		record := TraceRecord{Operation: "listener", Statement: statement.Name(), Sequence: listenerSequence, Time: formatTraceTime(batch.Time)}
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
		case "set-variable":
			return trace, fmt.Errorf("compat: Replay set-variable requires a step handler")
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

// FafHandler executes a fire-and-forget step such as a FAF named-window
// delete that is part of the language-neutral scenario. Java oracles run the
// equivalent FireAndForgetService query so both traces observe the same
// mutation.
type FafHandler func(step Step) error

// StepHandler executes a protocol step such as a mid-case deployment or a
// fire-and-forget mutation. Java oracles perform the equivalent runtime
// action so both traces observe the same lifecycle. attach subscribes a
// trace listener to a statement deployed by the step, mirroring the oracle's
// mid-case listener attachment; it is a no-op when the handler does not
// deploy statements.
type StepHandler func(step Step, attach func(*esper.Statement) error) ([]TraceRecord, error)

func ReplayWithStatements(ctx context.Context, engine *esper.Engine, statement *esper.Statement, scenario Scenario, decode DecodePayload, resolve StatementResolver, additional ...*esper.Statement) (Trace, error) {
	return ReplayWithStatementsAndFaf(ctx, engine, statement, scenario, decode, resolve, nil, additional...)
}

// ReplayWithStatementsAndFaf is ReplayWithStatements plus fire-and-forget
// step handling: a step with op "faf" is delegated to faf (which must be
// non-nil), allowing Java/Go traces to include FAF mutations such as
// delete-all from a named window.
func ReplayWithStatementsAndFaf(ctx context.Context, engine *esper.Engine, statement *esper.Statement, scenario Scenario, decode DecodePayload, resolve StatementResolver, faf FafHandler, additional ...*esper.Statement) (Trace, error) {
	return ReplayWithStatementsAndHandlers(ctx, engine, statement, scenario, decode, resolve, map[string]StepHandler{"faf": func(step Step, _ func(*esper.Statement) error) ([]TraceRecord, error) { return nil, faf(step) }}, additional...)
}

// ReplayWithStatementsAndHandlers is ReplayWithStatements plus protocol step
// handlers keyed by op ("faf", "deploy", ...). Handlers receive the step and
// perform the equivalent runtime action.
func ReplayWithStatementsAndHandlers(ctx context.Context, engine *esper.Engine, statement *esper.Statement, scenario Scenario, decode DecodePayload, resolve StatementResolver, handlers map[string]StepHandler, additional ...*esper.Statement) (Trace, error) {
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
		record := TraceRecord{Case: caseName, Operation: operation, Statement: current.Name(), Sequence: sequence, Time: formatTraceTime(batch.Time)}
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
		case "faf", "deploy", "read-variable", "set-variable", "types":
			handler := handlers[step.Op]
			if handler == nil {
				return trace, fmt.Errorf("compat: no %s handler for step %q", step.Op, step.Statement)
			}
			records, err := handler(step, func(attached *esper.Statement) error {
				if attached == nil {
					return fmt.Errorf("compat: %s step attached a nil statement", step.Op)
				}
				for _, existing := range statements {
					if existing.Name() == attached.Name() {
						return nil
					}
				}
				statements = append(statements, attached)
				if _, err := attached.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					listenerSequences[attached.Name()]++
					appendBatch(caseName, "listener", attached, batch, nil, listenerSequences[attached.Name()])
					return nil
				}); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return trace, err
			}
			for i := range records {
				if records[i].Case == "" {
					records[i].Case = caseName
				}
			}
			trace.Records = append(trace.Records, records...)
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
				case "ids":
					selector = esper.SelectContextPartitionIDs(step.IDs...)
				case "filtered":
					property := step.FilterProperty
					value := step.FilterValue
					selector = esper.ContextPartitionSelectorDescriptorFunc(func(descriptor esper.ContextPartitionDescriptor) bool {
						// Filtered selectors match the initiating event's
						// property value, mirroring Esper's
						// ContextPartitionSelectorFiltered implementations
						// that inspect the partition identifier's initiating
						// event (for example SupportSelectorFilteredInitTerm).
						initiating, ok := descriptor.Property("initiating_event")
						if !ok || initiating.IsNull() || initiating.IsMissing() {
							return false
						}
						event, ok := initiating.Any().(esper.Event)
						if !ok {
							return false
						}
						valueState := event.Get(property)
						if valueState.IsNull() || valueState.IsMissing() {
							return value == ""
						}
						return fmt.Sprintf("%v", dereferenceComparable(valueState.Any())) == value
					})
				case "segmented":
					// A segmented selector is invalid for an initiated
					// context; the runtime rejects it exactly like Esper's
					// InvalidContextPartitionSelector.
					selector = esper.SelectContextPartitionSegments([]any{})
				}
			}
			result, err := current.SnapshotWithSelector(ctx, selector)
			if err != nil {
				if step.ExpectError != "" {
					record := TraceRecord{Case: caseName, Operation: "selector-error", Statement: current.Name(), Time: formatTraceTime(engine.Now())}
					if selectorErrorCategory(err) == step.ExpectError {
						record.Value = step.ExpectError
					} else {
						record.Value = err.Error()
					}
					trace.Records = append(trace.Records, record)
					continue
				}
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
func normalizePartitions(descriptors []esper.ContextPartitionDescriptor) []PartitionRecord {
	if len(descriptors) == 0 {
		// The oracle emits an empty JSON array for snapshot-selector
		// records; keep the field present so the trace shapes agree.
		return []PartitionRecord{}
	}
	result := make([]PartitionRecord, 0, len(descriptors))
	for _, descriptor := range descriptors {
		properties := make(map[string]any)
		if hash, ok := descriptor.Property("hash"); ok {
			properties["hash"] = normalizeValue(hash)
		}
		if start, ok := descriptor.Property("startTime"); ok && start.IsPresent() {
			properties["startTime"] = normalizeTraceTimeMillis(start)
		}
		// Active initiated-terminated partitions expose endTime as null, the
		// same shape the oracle emits for ContextPartitionIdentifierInitiatedTerminated.
		if _, ok := descriptor.Property("startTime"); ok {
			properties["endTime"] = nil
		}
		if end, ok := descriptor.Property("endTime"); ok && end.IsPresent() {
			properties["endTime"] = normalizeTraceTimeMillis(end)
		}
		if initiating, ok := descriptor.Property("initiating_event"); ok && initiating.IsPresent() {
			if event, ok := initiating.Any().(esper.Event); ok {
				properties["initiating.p00"] = normalizeValue(event.Get("p00"))
			}
		}
		result = append(result, PartitionRecord{ID: descriptor.ID, Key: normalizePartitionKey(descriptor, properties), Properties: properties})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ID != result[j].ID {
			return result[i].ID < result[j].ID
		}
		return result[i].Key < result[j].Key
	})
	return result
}

// normalizeTraceTimeMillis renders an initiated-context startTime/endTime
// property as epoch milliseconds, matching the Java oracle's
// ContextPartitionIdentifierInitiatedTerminated long values. A missing or
// null endTime (active partition) stays null.
func normalizeTraceTimeMillis(value esper.Value) any {
	if value.IsMissing() || value.IsNull() {
		return nil
	}
	if instant, ok := value.Any().(time.Time); ok {
		return instant.UnixMilli()
	}
	return value.Any()
}

// normalizePartitionKey converts internal Go keyed-context partition keys to
// the language-neutral "key:<value>" form used by the Java oracle.
func normalizePartitionKey(descriptor esper.ContextPartitionDescriptor, properties map[string]any) string {
	if start, ok := properties["startTime"]; ok {
		// Initiated-terminated partitions are identified by their start
		// instant, matching the oracle's "start:<epoch-ms>" key.
		if millis, ok := start.(int64); ok {
			return fmt.Sprintf("start:%d", millis)
		}
	}
	key := descriptor.Key
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
	if events, ok := value.Any().([]esper.Event); ok {
		// Array-valued event columns (for example prevwindow) normalize to
		// the same row shape as a result stream, matching the Java oracle's
		// EventBean[] rendering.
		rows := make([]any, 0, len(events))
		for _, event := range events {
			fields := make(map[string]any)
			for _, field := range event.Schema().Fields() {
				fields[field.Name] = normalizeValue(event.Get(field.Name))
			}
			rows = append(rows, map[string]any{"kind": "row", "fields": fields})
		}
		return rows
	}
	if event, ok := value.Any().(esper.Event); ok {
		// Single event columns (for example context.startevent / context.
		// endevent projections) normalize to the same row shape as the
		// Java oracle's EventBean/Map rendering.
		fields := make(map[string]any)
		for _, field := range event.Schema().Fields() {
			fields[field.Name] = normalizeValue(event.Get(field.Name))
		}
		return map[string]any{"kind": "row", "fields": fields}
	}
	if nested, ok := value.Any().(map[string]any); ok {
		fields := make(map[string]any, len(nested))
		for name, field := range nested {
			fields[name] = normalizeValue(esper.Present(field))
		}
		return fields
	}
	return value.Any()
}

// selectorErrorCategory maps a selector rejection to the language-neutral
// category used by the differential protocol, mirroring Esper's
// InvalidContextPartitionSelector class name.
func selectorErrorCategory(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if strings.Contains(message, "Invalid context partition selector") {
		return "invalid-context-partition-selector"
	}
	return ""
}

// dereferenceComparable unwraps pointer indirections so string formatting of
// event property values compares equal to their scalar representation, the
// same way the trace normalizer (JSON) dereferences pointers.
func dereferenceComparable(value any) any {
	reflected := reflect.ValueOf(value)
	for reflected.IsValid() && (reflected.Kind() == reflect.Pointer || reflected.Kind() == reflect.Interface) {
		if reflected.IsNil() {
			return nil
		}
		reflected = reflected.Elem()
	}
	if !reflected.IsValid() {
		return nil
	}
	return reflected.Interface()
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
