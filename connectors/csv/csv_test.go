package csv

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
)

func TestSourceReadsTitleRowCommentsQuotesAndTypes(t *testing.T) {
	input := `# ignored

id,name,amount,active,when,meta
1,"Alice, Inc",2.50,true,2025-01-02T03:04:05Z,"{""region"":""cn""}"
`
	source, err := NewSource(SourceSpec{
		Reader:        strings.NewReader(input),
		HasTitleLine:  true,
		PropertyTypes: map[string]ColumnType{"id": ColumnTypeInt, "amount": ColumnTypeFloat64, "active": ColumnTypeBool, "when": ColumnTypeTime, "meta": ColumnTypeJSON},
		EmptyAsNil:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	record, err := source.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := record["id"].(int); !ok || got != 1 {
		t.Fatalf("id = %#v, want int(1)", record["id"])
	}
	if got := record["name"]; got != "Alice, Inc" {
		t.Fatalf("name = %#v", got)
	}
	if got, ok := record["amount"].(float64); !ok || got != 2.5 {
		t.Fatalf("amount = %#v, want 2.5", record["amount"])
	}
	if got, ok := record["active"].(bool); !ok || !got {
		t.Fatalf("active = %#v", record["active"])
	}
	if _, ok := record["when"].(time.Time); !ok {
		t.Fatalf("when = %#v, want time.Time", record["when"])
	}
	meta, ok := record["meta"].(map[string]any)
	if !ok || meta["region"] != "cn" {
		t.Fatalf("meta = %#v", record["meta"])
	}
	if _, err := source.Next(context.Background()); !errors.Is(err, io.EOF) {
		t.Fatalf("second read = %v, want EOF", err)
	}
	if got := source.State(); got != connectors.Opened {
		t.Fatalf("state at EOF = %s, want OPENED", got)
	}
}

func TestSourcePropertyOrderAutoTitleAndCustomConverter(t *testing.T) {
	source, err := NewSource(SourceSpec{
		Reader:        strings.NewReader("name,id\nAlice,7\n"),
		PropertyOrder: []string{"name", "id"},
		Converters: map[string]Converter{
			"id": func(value string) (any, error) { return "id-" + value, nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	record, err := source.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if record["name"] != "Alice" || record["id"] != "id-7" {
		t.Fatalf("record = %#v", record)
	}
}

func TestSourceLoopResetPauseResumeAndRun(t *testing.T) {
	source, err := NewSource(SourceSpec{
		Reader:        bytes.NewReader([]byte("1,one\n2,two\n")),
		PropertyOrder: []string{"id", "name"},
		PropertyTypes: map[string]ColumnType{"id": ColumnTypeInt},
		Loop:          true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	if err := source.Pause(); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Next(context.Background()); !errors.Is(err, connectors.ErrPaused) {
		t.Fatalf("Next while paused = %v", err)
	}
	if err := source.Resume(); err != nil {
		t.Fatal(err)
	}
	for index, want := range []int{1, 2, 1} {
		record, err := source.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if record["id"] != want {
			t.Fatalf("loop record %d = %#v", index, record)
		}
	}
	if err := source.Reset(); err != nil {
		t.Fatal(err)
	}
	record, err := source.Next(context.Background())
	if err != nil || record["id"] != 1 {
		t.Fatalf("after reset record=%#v err=%v", record, err)
	}
	if err := source.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := source.Destroy(); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Next(context.Background()); !errors.Is(err, connectors.ErrDestroyed) {
		t.Fatalf("Next after destroy = %v", err)
	}
}

func TestSourceRunTimestampAndEngineBridge(t *testing.T) {
	source, err := NewSource(SourceSpec{
		Reader:          strings.NewReader("0,first\n1,second\n"),
		PropertyOrder:   []string{"timestamp", "name"},
		PropertyTypes:   map[string]ColumnType{"timestamp": ColumnTypeInt64},
		TimestampColumn: "timestamp",
		TimestampUnit:   time.Nanosecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	var records []Record
	if err := source.Run(context.Background(), func(_ context.Context, record Record) error {
		records = append(records, record)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1]["name"] != "second" {
		t.Fatalf("records = %#v", records)
	}

	env := esper.NewEnvironment()
	schema, err := esper.NewMapSchema("CSVEvent", []esper.FieldSpec{
		esper.FieldDef("id", reflect.TypeOf(int64(0))),
		esper.FieldDef("name", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(schema); err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	bridge, err := NewSource(SourceSpec{Reader: strings.NewReader("7,engine\n"), PropertyOrder: []string{"id", "name"}, PropertyTypes: map[string]ColumnType{"id": ColumnTypeInt64}})
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.Start(); err != nil {
		t.Fatal(err)
	}
	if err := bridge.RunToEngine(context.Background(), engine, "CSVEvent"); err != nil {
		t.Fatal(err)
	}
}

func TestSourceRejectsDecreasingTimestampAndStrictRows(t *testing.T) {
	source, err := NewSource(SourceSpec{
		Reader:          strings.NewReader("2,a\n1,b\n"),
		PropertyOrder:   []string{"timestamp", "name"},
		PropertyTypes:   map[string]ColumnType{"timestamp": ColumnTypeInt64},
		TimestampColumn: "timestamp",
		TimestampUnit:   time.Nanosecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	err = source.Run(context.Background(), func(context.Context, Record) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "smaller than previous") {
		t.Fatalf("decreasing timestamp error = %v", err)
	}

	strict, err := NewSource(SourceSpec{Reader: strings.NewReader("1,extra\n"), PropertyOrder: []string{"id"}, StrictFields: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := strict.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := strict.Next(context.Background()); err == nil {
		t.Fatal("strict source accepted an extra field")
	}
}
