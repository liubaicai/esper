package csv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestCsvInputAdapterDifferential replays the shared esperio-csv parity
// scenario (testdata/parity/csv-input-adapter.json) through Source and emits
// the same normalized trace the Java CsvInputAdapterScenarioOracle produces:
// records of {case, operation, statement, sequence, time, new:[{kind:row,
// fields}]}. The expected records mirror CSVInputAdapter running in
// synchronous DirectSender mode with no events-per-second or timestamp
// pacing, so every listener firing happens at runtime time zero.
//
// The embedded CSV exercises the adapter edge cases shared by CSVReader and
// encoding/csv: a '#' comment line, a blank line, a quoted value containing a
// comma, doubled-quote escaping inside a quoted value, an empty trailing
// field, an empty middle field, and a multi-line quoted value.
func TestCsvInputAdapterDifferential(t *testing.T) {
	scenarioPath := filepath.Join("..", "..", "testdata", "parity", "csv-input-adapter.json")
	raw, err := os.ReadFile(scenarioPath)
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	var scenario struct {
		Version string `json:"version"`
		ID      string `json:"id"`
		Steps   []struct {
			Op            string            `json:"op"`
			Case          string            `json:"case"`
			Source        string            `json:"source"`
			PropertyOrder []string          `json:"propertyOrder"`
			PropertyTypes map[string]string `json:"propertyTypes"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(raw, &scenario); err != nil {
		t.Fatalf("parse scenario: %v", err)
	}
	if scenario.Version != "esper-parity/v1" || scenario.ID != "csv-input-adapter" {
		t.Fatalf("unexpected scenario header: version=%q id=%q", scenario.Version, scenario.ID)
	}
	var step *struct {
		Op            string            `json:"op"`
		Case          string            `json:"case"`
		Source        string            `json:"source"`
		PropertyOrder []string          `json:"propertyOrder"`
		PropertyTypes map[string]string `json:"propertyTypes"`
	}
	for i := range scenario.Steps {
		if scenario.Steps[i].Op == "case" && scenario.Steps[i].Case == "csv" {
			step = &scenario.Steps[i]
			break
		}
	}
	if step == nil {
		t.Fatal("scenario has no csv case")
	}
	if len(step.PropertyOrder) != 6 || len(step.PropertyTypes) != 6 {
		t.Fatalf("unexpected csv case config: order=%v types=%v", step.PropertyOrder, step.PropertyTypes)
	}

	propertyTypes := make(map[string]ColumnType, len(step.PropertyTypes))
	for name, typeName := range step.PropertyTypes {
		switch typeName {
		case "string":
			propertyTypes[name] = ColumnTypeString
		case "int":
			propertyTypes[name] = ColumnTypeInt
		case "long":
			propertyTypes[name] = ColumnTypeInt64
		case "double":
			propertyTypes[name] = ColumnTypeFloat64
		case "boolean":
			propertyTypes[name] = ColumnTypeBool
		default:
			t.Fatalf("unsupported scenario property type %q", typeName)
		}
	}

	source, err := NewSource(SourceSpec{
		Reader:        strings.NewReader(step.Source),
		HasTitleLine:  true,
		PropertyOrder: append([]string(nil), step.PropertyOrder...),
		PropertyTypes: propertyTypes,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}

	trace := struct {
		Version string     `json:"version"`
		ID      string     `json:"id"`
		Records []traceRow `json:"records"`
	}{Version: scenario.Version, ID: scenario.ID}

	for {
		record, err := source.Next(context.Background())
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		fields := make(map[string]any, len(record))
		for name, value := range record {
			fields[name] = normalizeTraceValue(value)
		}
		trace.Records = append(trace.Records, traceRow{
			Case:      "csv",
			Operation: "listener",
			Statement: "s0",
			Sequence:  uint64(len(trace.Records) + 1),
			Time:      time.Unix(0, 0).UTC().Format(time.RFC3339),
			New:       []traceResult{{Kind: "row", Fields: fields}},
		})
	}
	if source.State().String() != "OPENED" {
		t.Fatalf("state at EOF = %s, want OPENED", source.State())
	}

	want := []traceRow{
		{
			Case: "csv", Operation: "listener", Statement: "s0", Sequence: 1,
			Time: "1970-01-01T00:00:00Z",
			New: []traceResult{{Kind: "row", Fields: map[string]any{
				"active": true, "note": "buy, hold", "price": int64(5), "qty": int64(10), "symbol": "IBM", "ts": int64(1),
			}}},
		},
		{
			Case: "csv", Operation: "listener", Statement: "s0", Sequence: 2,
			Time: "1970-01-01T00:00:00Z",
			New: []traceResult{{Kind: "row", Fields: map[string]any{
				"active": false, "note": "\"halt\" now", "price": int64(3), "qty": int64(25), "symbol": "AAPL", "ts": int64(2),
			}}},
		},
		{
			Case: "csv", Operation: "listener", Statement: "s0", Sequence: 3,
			Time: "1970-01-01T00:00:00Z",
			New: []traceResult{{Kind: "row", Fields: map[string]any{
				"active": true, "note": "", "price": int64(4), "qty": int64(30), "symbol": "MSFT", "ts": int64(3),
			}}},
		},
		{
			Case: "csv", Operation: "listener", Statement: "s0", Sequence: 4,
			Time: "1970-01-01T00:00:00Z",
			New: []traceResult{{Kind: "row", Fields: map[string]any{
				"active": false, "note": "multi\nline", "price": int64(2), "qty": int64(15), "symbol": "", "ts": int64(4),
			}}},
		},
	}
	if !reflect.DeepEqual(trace.Records, want) {
		gotJSON, _ := json.MarshalIndent(trace.Records, "", "  ")
		wantJSON, _ := json.MarshalIndent(want, "", "  ")
		t.Fatalf("trace mismatch:\ngot:\n%s\nwant:\n%s", gotJSON, wantJSON)
	}

	encoded, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("CSV_GOTRACE_OUT"); out != "" {
		if err := os.WriteFile(out, encoded, 0o644); err != nil {
			t.Fatalf("write go trace: %v", err)
		}
		t.Logf("go trace written to %s", out)
	}
	t.Logf("go trace:\n%s", encoded)
}

type traceRow struct {
	Case      string        `json:"case,omitempty"`
	Operation string        `json:"operation"`
	Statement string        `json:"statement,omitempty"`
	Sequence  uint64        `json:"sequence,omitempty"`
	Time      string        `json:"time,omitempty"`
	New       []traceResult `json:"new,omitempty"`
}

type traceResult struct {
	Kind   string         `json:"kind"`
	Fields map[string]any `json:"fields"`
}

// normalizeTraceValue mirrors the Java oracle's TraceWriter.normalize: null
// becomes {"state":"null"}, integral Java numbers stay numbers (doubles are
// emitted as the Java oracle's longValue projection), booleans stay booleans
// and everything else becomes a string.
func normalizeTraceValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return map[string]any{"state": "null"}
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case bool:
		return typed
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}
