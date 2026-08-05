package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
)

func TestClientSinkAddsQueryAndRendersPlaceholders(t *testing.T) {
	queries := make(chan url.Values, 2)
	paths := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		queries <- request.URL.Query()
		paths <- request.URL.Path
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sink, err := NewClientSink(ClientSpec{URI: server.URL + "/root", Stream: "Trade"})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), map[string]any{"name": "Alice Inc", "id": 7}); err != nil {
		t.Fatal(err)
	}
	query := <-queries
	if query.Get("stream") != "Trade" || query.Get("id") != "7" || query.Get("name") != "Alice Inc" {
		t.Fatalf("query = %#v", query)
	}
	_ = <-paths

	placeholder, err := NewClientSink(ClientSpec{URI: server.URL + "/root/${stream}/${name}", Stream: "Trade"})
	if err != nil {
		t.Fatal(err)
	}
	if err := placeholder.Start(); err != nil {
		t.Fatal(err)
	}
	if err := placeholder.Write(context.Background(), map[string]any{"name": "Alice Inc"}); err != nil {
		t.Fatal(err)
	}
	if got := <-paths; got != "/root/Trade/Alice%20Inc" && got != "/root/Trade/Alice Inc" {
		t.Fatalf("placeholder path = %q", got)
	}
	if err := sink.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func TestClientSinkPostsJSONRetriesAndHonorsLifecycle(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		data, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		var record map[string]any
		if err := json.Unmarshal(data, &record); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if request.Header.Get("Content-Type") != "application/json" || record["id"] != float64(8) {
			t.Errorf("request headers/body = %s %#v", request.Header.Get("Content-Type"), record)
		}
		if calls.Load() == 1 {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	sink, err := NewClientSink(ClientSpec{URI: server.URL, Method: http.MethodPost, Retry: 2, RetryInterval: time.Millisecond, MaxResponseBytes: 16})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	response, err := sink.Do(context.Background(), map[string]any{"id": 8})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusNoContent || calls.Load() != 2 {
		t.Fatalf("response=%#v calls=%d", response, calls.Load())
	}
	if err := sink.Pause(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), map[string]any{"id": 9}); !errors.Is(err, connectors.ErrPaused) {
		t.Fatalf("write while paused = %v", err)
	}
	if err := sink.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func TestServerSourceEmitsRequestsAndSupportsPauseStopRestart(t *testing.T) {
	requests := make(chan Request, 2)
	source, err := NewServerSource(ServerSourceSpec{
		Addr: "127.0.0.1:0",
		Path: "/events",
		Emit: func(_ context.Context, request Request) error {
			requests <- request
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: time.Second}
	response, err := client.Post("http://"+source.Addr()+"/events?stream=Trade&id=7", "text/plain", strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	incoming := <-requests
	if incoming.Query.Get("stream") != "Trade" || incoming.Query.Get("id") != "7" || string(incoming.Body) != "payload" {
		t.Fatalf("incoming = %#v", incoming)
	}
	if err := source.Pause(); err != nil {
		t.Fatal(err)
	}
	paused, err := client.Get("http://" + source.Addr() + "/events")
	if err != nil {
		t.Fatal(err)
	}
	paused.Body.Close()
	if paused.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("paused status = %d", paused.StatusCode)
	}
	if err := source.Resume(); err != nil {
		t.Fatal(err)
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
	if err := source.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func TestServerSourceRoutesQueryToEngine(t *testing.T) {
	env := esper.NewEnvironment()
	schema, err := esper.NewMapSchema("HTTPEvent", []esper.FieldSpec{
		esper.FieldDef("id", reflect.TypeOf(0)),
		esper.FieldDef("name", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(schema); err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	source, err := NewServerSource(ServerSourceSpec{
		Addr:      "127.0.0.1:0",
		Engine:    engine,
		EventType: "HTTPEvent",
		QueryConverters: map[string]func(string) (any, error){
			"id": func(value string) (any, error) { return strconv.Atoi(value) },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	response, err := http.Get("http://" + source.Addr() + "/?stream=HTTPEvent&id=12&name=server")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("engine route status = %d", response.StatusCode)
	}
	if err := source.Destroy(); err != nil {
		t.Fatal(err)
	}
}
