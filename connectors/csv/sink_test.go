package csv

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
)

type sinkTrade struct {
	ID        int       `esper:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `esper:"created"`
}

func TestSinkWritesHeaderFixedColumnsAndEsperValues(t *testing.T) {
	var output bytes.Buffer
	sink, err := NewSink(SinkSpec{
		Writer:         &output,
		Columns:        []string{"id", "name", "created"},
		Header:         true,
		FlushEachWrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), Record{"id": 1, "name": "Alice, Inc", "created": time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Stop(); err != nil {
		t.Fatal(err)
	}
	want := "id,name,created\n1,\"Alice, Inc\",2025-01-02T03:04:05Z\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}

	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	if err := sink.WriteEvent(context.Background(), sinkTrade{ID: 2, Name: "Bob", CreatedAt: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Stop(); err != nil {
		t.Fatal(err)
	}
	if sink.WriteCount() != 2 {
		t.Fatalf("WriteCount = %d, want 2", sink.WriteCount())
	}
	if !strings.Contains(output.String(), "2,Bob,2025-01-03T00:00:00Z") {
		t.Fatalf("struct event missing from output: %q", output.String())
	}
}

func TestSinkDerivesSortedColumnsAndWritesRows(t *testing.T) {
	var output bytes.Buffer
	sink, err := NewSink(SinkSpec{Writer: &output, Header: true, FlushEachWrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), Record{"z": 2, "a": 1}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Stop(); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "a,z\n1,2\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestSinkAppendDoesNotDuplicateExistingHeader(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "events.csv")
	first, err := NewSink(SinkSpec{Path: path, Header: true, Columns: []string{"id"}, FlushEachWrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	if err := first.Write(context.Background(), Record{"id": 1}); err != nil {
		t.Fatal(err)
	}
	if err := first.Stop(); err != nil {
		t.Fatal(err)
	}

	second, err := NewSink(SinkSpec{Path: path, Header: true, Columns: []string{"id"}, Append: true, FlushEachWrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Start(); err != nil {
		t.Fatal(err)
	}
	if err := second.Write(context.Background(), Record{"id": 2}); err != nil {
		t.Fatal(err)
	}
	if err := second.Stop(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "id\n1\n2\n"; got != want {
		t.Fatalf("appended output = %q, want %q", got, want)
	}
}

func TestSinkLifecycleAndEventConversion(t *testing.T) {
	sink, err := NewSink(SinkSpec{Writer: &bytes.Buffer{}, Columns: []string{"value"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), Record{"value": 1}); !errors.Is(err, connectors.ErrNotStarted) {
		t.Fatalf("write before start = %v", err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Pause(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), Record{"value": 1}); !errors.Is(err, connectors.ErrPaused) {
		t.Fatalf("write while paused = %v", err)
	}
	if err := sink.Resume(); err != nil {
		t.Fatal(err)
	}
	if err := sink.WriteEvent(context.Background(), nil); err == nil {
		t.Fatal("nil event unexpectedly written")
	}
	if err := sink.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Destroy(); err != nil {
		t.Fatalf("destroy after stop = %v", err)
	}
	if err := sink.Write(context.Background(), Record{"value": 1}); !errors.Is(err, connectors.ErrDestroyed) {
		t.Fatalf("write after destroy = %v", err)
	}
}

func TestEventToRecordSupportsEsperEventRowAndTableRow(t *testing.T) {
	env := esper.NewEnvironment()
	schema, err := esper.NewMapSchema("TestEvent", []esper.FieldSpec{esper.FieldDef("id", reflectTypeInt())})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(schema); err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	if err := engine.Send(context.Background(), "TestEvent", map[string]any{"id": 9}); err != nil {
		t.Fatal(err)
	}
	event, err := esper.ParseJSON(schema, []byte(`{"id":10}`), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	record, err := EventToRecord(event)
	if err != nil || record["id"] != 10 {
		t.Fatalf("event record=%#v err=%v", record, err)
	}
	row := esper.Row{}
	if _, err := EventToRecord(row); err != nil {
		t.Fatalf("empty row conversion: %v", err)
	}
}

func reflectTypeInt() reflect.Type { return reflect.TypeOf(int(0)) }
