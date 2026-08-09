package kafka

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
	kafkago "github.com/segmentio/kafka-go"
)

type fakeReader struct {
	mu       sync.Mutex
	messages []Message
	index    int
	commits  []Message
	closed   bool
}

func (r *fakeReader) FetchMessage(ctx context.Context) (Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.index < len(r.messages) {
		message := r.messages[r.index].clone()
		r.index++
		return message, nil
	}
	if r.closed {
		return Message{}, context.Canceled
	}
	select {
	case <-ctx.Done():
		return Message{}, ctx.Err()
	default:
		return Message{}, io.EOF
	}
}

func (r *fakeReader) Commit(_ context.Context, message Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commits = append(r.commits, message.clone())
	return nil
}

func (r *fakeReader) Close() error {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	return nil
}

type fakeWriter struct {
	mu       sync.Mutex
	messages []Message
	failures int
	closed   bool
}

func (w *fakeWriter) WriteMessages(_ context.Context, messages ...Message) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failures > 0 {
		w.failures--
		return errors.New("temporary writer failure")
	}
	for _, message := range messages {
		w.messages = append(w.messages, message.clone())
	}
	return nil
}

func (w *fakeWriter) Close() error {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	return nil
}

type kafkaSourceEvent struct {
	Symbol string `esper:"symbol"`
	Count  int    `esper:"count"`
}

func TestSourceJSONProcessorRoutesAndCommitsAfterProcessing(t *testing.T) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterMap(env, "KafkaEvent", []esper.FieldSpec{
		{Name: "symbol", Type: reflect.TypeOf("")},
		{Name: "count", Type: reflect.TypeOf(int(0))},
	}); err != nil {
		t.Fatal(err)
	}
	engine := esper.NewEngine(env)
	plan, err := env.Build(esper.FromAny(env, "KafkaEvent").Query(esper.StatementName("kafka-source")))
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

	reader := &fakeReader{messages: []Message{{Topic: "events", Key: []byte("k1"), Value: []byte(`{"symbol":"A","count":3}`)}}}
	source, err := NewSource(SourceSpec{
		Reader:    reader,
		EventType: "KafkaEvent",
		Decode:    DecodeJSON,
		Engine:    engine,
		TimestampExtractor: func(Message) (time.Time, bool) {
			return time.Unix(123, 0).UTC(), true
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	batch := receiveBatch(t, received)
	event, ok := batch.New[0].Event()
	if !ok || event.Get("symbol").Any() != "A" || event.Get("count").Any() != 3 {
		t.Fatalf("batch = %#v", batch.New)
	}
	if !engine.Now().Equal(time.Unix(123, 0).UTC()) {
		t.Fatalf("engine time = %s", engine.Now())
	}
	commits := waitForKafkaCommits(t, reader, 1)
	if len(commits) != 1 || string(commits[0].Key) != "k1" {
		t.Fatalf("commits = %#v", commits)
	}
	if err := source.Stop(); err != nil {
		t.Fatal(err)
	}
	if source.State() != connectors.Opened {
		t.Fatalf("state = %s", source.State())
	}
	if err := source.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func waitForKafkaCommits(t *testing.T, reader *fakeReader, count int) []Message {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		reader.mu.Lock()
		commits := append([]Message(nil), reader.commits...)
		reader.mu.Unlock()
		if len(commits) >= count {
			return commits
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d Kafka commits, got %#v", count, commits)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSourceFactoryRestartsAfterStopAndRetriesProcessor(t *testing.T) {
	readers := []*fakeReader{
		{messages: []Message{{Topic: "events", Value: []byte("one")}}},
		{messages: []Message{{Topic: "events", Value: []byte("two")}}},
	}
	var opened int
	var mu sync.Mutex
	seen := make(chan string, 2)
	source, err := NewSource(SourceSpec{
		Open: func() (Reader, error) {
			mu.Lock()
			defer mu.Unlock()
			reader := readers[opened]
			opened++
			return reader, nil
		},
		Emit: func(_ context.Context, message Message, _ any) error {
			seen <- string(message.Value)
			return nil
		},
		RetryAttempts: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	if got := receiveString(t, seen); got != "one" {
		t.Fatalf("first value = %q", got)
	}
	if err := source.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	if got := receiveString(t, seen); got != "two" {
		t.Fatalf("second value = %q", got)
	}
	if err := source.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func TestSinkEncodesRowsRetriesAndHonorsLifecycle(t *testing.T) {
	writer := &fakeWriter{failures: 1}
	sink, err := NewSink(SinkSpec{
		Writer:        writer,
		Topic:         "out-events",
		RetryAttempts: 2,
		RetryInterval: time.Millisecond,
		Key: func(result esper.Result) ([]byte, error) {
			return []byte(result.Get("symbol").Any().(string)), nil
		},
		Headers: map[string][]byte{"format": []byte("json")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[kafkaSourceEvent](env, "KafkaSinkEvent"); err != nil {
		t.Fatal(err)
	}
	engine := esper.NewEngine(env)
	plan, err := env.Build(esper.From[kafkaSourceEvent](env, "KafkaSinkEvent").Query(esper.StatementName("kafka-sink")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deployment.Statements()[0].Subscribe(sink.Write); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), kafkaSourceEvent{Symbol: "A", Count: 2}); err != nil {
		t.Fatal(err)
	}
	writer.mu.Lock()
	messages := append([]Message(nil), writer.messages...)
	writer.mu.Unlock()
	if len(messages) != 1 || messages[0].Topic != "out-events" || string(messages[0].Key) != "A" || string(messages[0].Headers["format"]) != "json" {
		t.Fatalf("messages = %#v", messages)
	}
	if string(messages[0].Value) != `{"symbol":"A","count":2}` {
		t.Fatalf("value = %s", messages[0].Value)
	}
	if err := sink.Pause(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), esper.ResultBatch{}); !errors.Is(err, connectors.ErrPaused) {
		t.Fatalf("paused write = %v", err)
	}
	if err := sink.Destroy(); err != nil {
		t.Fatal(err)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestKafkaValidationAndJSONDecoderErrors(t *testing.T) {
	if _, err := NewSource(SourceSpec{}); err == nil {
		t.Fatal("empty source was accepted")
	}
	if _, err := NewSink(SinkSpec{Writer: &fakeWriter{}}); err == nil {
		t.Fatal("sink without topic was accepted")
	}
	if _, err := DecodeJSON(Message{Topic: "bad", Value: []byte("not-json")}); err == nil {
		t.Fatal("invalid JSON was accepted")
	}
}

func TestKafkaDockerRoundTrip(t *testing.T) {
	brokersValue := strings.TrimSpace(os.Getenv("ESPER_KAFKA_BROKERS"))
	if brokersValue == "" {
		t.Skip("set ESPER_KAFKA_BROKERS to run the Kafka broker integration")
	}
	brokers := strings.Split(brokersValue, ",")
	topic := "esper-go-kafka-test-" + time.Now().Format("20060102150405.000000000")
	admin, err := kafkago.Dial("tcp", brokers[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.CreateTopics(kafkago.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1}); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	if err := admin.Close(); err != nil {
		t.Fatal(err)
	}
	writer := NewKafkaWriter(kafkago.WriterConfig{
		Brokers:  brokers,
		Topic:    topic,
		Balancer: &kafkago.LeastBytes{},
	})
	defer writer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	want := Message{Topic: topic, Key: []byte("k"), Value: []byte(`{"value":7}`)}
	if err := writer.WriteMessages(ctx, want); err != nil {
		t.Fatal(err)
	}
	reader := NewKafkaReader(kafkago.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		GroupID:     topic + "-group",
		StartOffset: kafkago.FirstOffset,
		MinBytes:    1,
		MaxBytes:    1 << 20,
	})
	defer reader.Close()
	var got Message
	for attempt := 0; attempt < 10; attempt++ {
		got, err = reader.FetchMessage(ctx)
		if err == nil {
			break
		}
		if waitErr := waitRetry(ctx, time.Second); waitErr != nil {
			t.Fatal(err)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Key) != "k" || string(got.Value) != `{"value":7}` || got.Topic != topic {
		t.Fatalf("message = %#v", got)
	}
	if err := reader.Commit(ctx, got); err != nil {
		t.Fatal(err)
	}
}

func receiveBatch(t *testing.T, batches <-chan esper.ResultBatch) esper.ResultBatch {
	t.Helper()
	select {
	case batch := <-batches:
		return batch
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for batch")
		return esper.ResultBatch{}
	}
}

func receiveString(t *testing.T, values <-chan string) string {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for value")
		return ""
	}
}
