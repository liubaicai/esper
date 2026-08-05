package socket

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
)

type socketStructEvent struct {
	Symbol string `esper:"symbol"`
	Count  int    `esper:"count"`
}

func TestCSVSourceDecodesJavaEscapesAndSupportsMultipleConnections(t *testing.T) {
	messages := make(chan Message, 2)
	source, err := NewSource(SourceSpec{
		Port:     0,
		DataType: DataTypeCSV,
		Unescape: true,
		Emit: func(_ context.Context, message Message) error {
			messages <- message
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	defer source.Destroy()

	first := dialSocket(t, source.Addr())
	second := dialSocket(t, source.Addr())
	writeLine(t, first, `stream=Tick,symbol=ESPER,price=12.5`)
	writeLine(t, second, `stream=Tick,symbol=E\u002CGO,price=13.5`)
	first.Close()
	second.Close()

	got := []Message{receiveMessage(t, messages), receiveMessage(t, messages)}
	seen := map[string]bool{}
	for _, message := range got {
		if message.Stream != "Tick" {
			t.Fatalf("stream = %q", message.Stream)
		}
		record, ok := message.Value.(map[string]any)
		if !ok {
			t.Fatalf("value type = %T", message.Value)
		}
		seen[record["symbol"].(string)] = true
	}
	if _, ok := seen["ESPER"]; !ok {
		t.Fatalf("missing ESPER message: %#v", got)
	}
	if _, ok := seen["E,GO"]; !ok {
		t.Fatalf("missing escaped message: %#v", got)
	}
}

func TestPropertyOrderedCSVAndJSONSource(t *testing.T) {
	tests := []struct {
		name     string
		typ      DataType
		order    []string
		stream   string
		line     string
		wantName string
		wantInt  float64
	}{
		{name: "property ordered", typ: DataTypePropertyOrderedCSV, order: []string{"name", "count"}, stream: "Metric", line: "alpha,7", wantName: "alpha", wantInt: 7},
		{name: "json", typ: DataTypeJSON, stream: "Metric", line: `stream=Metric,json={"name":"beta","count":8}`, wantName: "beta", wantInt: 8},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			messages := make(chan Message, 1)
			source, err := NewSource(SourceSpec{
				Port:          0,
				DataType:      test.typ,
				Stream:        test.stream,
				PropertyOrder: test.order,
				PropertyTypes: map[string]reflect.Type{"count": reflect.TypeOf(int(0))},
				Emit: func(_ context.Context, message Message) error {
					messages <- message
					return nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := source.Start(); err != nil {
				t.Fatal(err)
			}
			defer source.Destroy()
			conn := dialSocket(t, source.Addr())
			writeLine(t, conn, test.line)
			_ = conn.Close()
			message := receiveMessage(t, messages)
			if message.Stream != test.stream {
				t.Fatalf("stream = %q, want %q", message.Stream, test.stream)
			}
			record := message.Value.(map[string]any)
			if record["name"] != test.wantName || record["count"] != test.wantInt && record["count"] != int(test.wantInt) {
				t.Fatalf("record = %#v", record)
			}
		})
	}
}

func TestSocketSourceRoutesTypedCSVIntoEngineAndRestarts(t *testing.T) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterMap(env, "SocketEvent", []esper.FieldSpec{
		{Name: "symbol", Type: reflect.TypeOf("")},
		{Name: "count", Type: reflect.TypeOf(int(0))},
	}); err != nil {
		t.Fatal(err)
	}
	engine := esper.NewEngine(env)
	plan, err := env.Build(esper.FromAny(env, "SocketEvent").Query(esper.StatementName("socket-engine")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	received := make(chan esper.ResultBatch, 2)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		received <- batch
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	source, err := NewSource(SourceSpec{Port: 0, DataType: DataTypeCSV, Engine: engine})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	conn := dialSocket(t, source.Addr())
	writeLine(t, conn, "stream=SocketEvent,symbol=A,count=3")
	batch := receiveBatch(t, received)
	row, ok := batch.New[0].Event()
	if !ok || row.Get("symbol").Any() != "A" || row.Get("count").Any() != 3 {
		t.Fatalf("engine event = %#v", batch.New)
	}
	if err := source.Stop(); err != nil {
		t.Fatal(err)
	}
	if source.State() != connectors.Opened {
		t.Fatalf("state after stop = %s", source.State())
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	conn = dialSocket(t, source.Addr())
	writeLine(t, conn, "stream=SocketEvent,symbol=B,count=4")
	second := receiveBatch(t, received)
	if event, ok := second.New[0].Event(); !ok || event.Get("symbol").Any() != "B" {
		t.Fatalf("restarted event = %#v", second.New)
	}
	_ = conn.Close()
	if err := source.Destroy(); err != nil {
		t.Fatal(err)
	}
	if source.State() != connectors.Destroyed {
		t.Fatalf("state after destroy = %s", source.State())
	}
}

func TestSocketSourceMaterializesStructBackedEngineEvent(t *testing.T) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[socketStructEvent](env, "SocketStruct"); err != nil {
		t.Fatal(err)
	}
	engine := esper.NewEngine(env)
	plan, err := env.Build(esper.From[socketStructEvent](env, "SocketStruct").Query(esper.StatementName("socket-struct")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	received := make(chan esper.ResultBatch, 1)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		received <- batch
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	source, err := NewSource(SourceSpec{Port: 0, DataType: DataTypeCSV, Engine: engine})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	defer source.Destroy()
	conn := dialSocket(t, source.Addr())
	writeLine(t, conn, "stream=SocketStruct,symbol=S,count=5")
	batch := receiveBatch(t, received)
	event, ok := batch.New[0].Event()
	if !ok {
		t.Fatalf("batch = %#v", batch.New)
	}
	underlying, ok := event.Underlying().(socketStructEvent)
	if !ok || underlying.Symbol != "S" || underlying.Count != 5 {
		t.Fatalf("underlying = %#v (%T)", event.Underlying(), event.Underlying())
	}
	_ = conn.Close()
}

func TestSocketPauseWaitsForResumeAndObjectDecoderIsExplicit(t *testing.T) {
	messages := make(chan Message, 1)
	source, err := NewSource(SourceSpec{
		Port:     0,
		DataType: DataTypeObject,
		ObjectDecoder: func(reader *bufio.Reader) (any, error) {
			var value map[string]any
			if err := json.NewDecoder(reader).Decode(&value); err != nil {
				return nil, err
			}
			return value, nil
		},
		Emit: func(_ context.Context, message Message) error {
			messages <- message
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	defer source.Destroy()
	if err := source.Pause(); err != nil {
		t.Fatal(err)
	}
	conn := dialSocket(t, source.Addr())
	writeLine(t, conn, `{"stream":"ObjectEvent","value":9}`)
	select {
	case message := <-messages:
		t.Fatalf("received while paused: %#v", message)
	case <-time.After(50 * time.Millisecond):
	}
	if err := source.Resume(); err != nil {
		t.Fatal(err)
	}
	message := receiveMessage(t, messages)
	if message.Stream != "ObjectEvent" || message.Value.(map[string]any)["value"] != float64(9) {
		t.Fatalf("object message = %#v", message)
	}
	_ = conn.Close()
}

func TestSocketConfigurationRejectsAmbiguousOutputAndInvalidOrderedMode(t *testing.T) {
	if _, err := NewSource(SourceSpec{Port: 0}); err == nil {
		t.Fatal("source without output was accepted")
	}
	if _, err := NewSource(SourceSpec{Port: 0, Emit: func(context.Context, Message) error { return nil }, DataType: DataTypePropertyOrderedCSV}); err == nil {
		t.Fatal("ordered source without properties was accepted")
	}
	if _, err := ValidateDataType("property-ordered-csv"); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateDataType("unsupported"); err == nil {
		t.Fatal("unsupported data type was accepted")
	}
	if !errors.Is(context.Canceled, context.Canceled) {
		t.Fatal("sanity check failed")
	}
}

func dialSocket(t *testing.T, address string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func writeLine(t *testing.T, conn net.Conn, line string) {
	t.Helper()
	if _, err := fmt.Fprintf(conn, "%s\n", line); err != nil {
		t.Fatal(err)
	}
}

func receiveMessage(t *testing.T, messages <-chan Message) Message {
	t.Helper()
	select {
	case message := <-messages:
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for socket message")
		return Message{}
	}
}

func receiveBatch(t *testing.T, batches <-chan esper.ResultBatch) esper.ResultBatch {
	t.Helper()
	select {
	case batch := <-batches:
		return batch
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for engine batch")
		return esper.ResultBatch{}
	}
}
