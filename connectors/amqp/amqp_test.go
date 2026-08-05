package amqp

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
)

type fakeConsumer struct {
	deliveries chan Delivery

	mu       sync.Mutex
	acked    []uint64
	rejected []struct {
		tag     uint64
		requeue bool
	}
	closed int
}

func (c *fakeConsumer) Consume(context.Context) (<-chan Delivery, error) {
	return c.deliveries, nil
}

func (c *fakeConsumer) Ack(_ context.Context, delivery Delivery) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acked = append(c.acked, delivery.DeliveryTag)
	return nil
}

func (c *fakeConsumer) Reject(_ context.Context, delivery Delivery, requeue bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rejected = append(c.rejected, struct {
		tag     uint64
		requeue bool
	}{delivery.DeliveryTag, requeue})
	return nil
}

func (c *fakeConsumer) Close() error {
	c.mu.Lock()
	c.closed++
	c.mu.Unlock()
	return nil
}

func (c *fakeConsumer) snapshot() (acked []uint64, rejected []struct {
	tag     uint64
	requeue bool
}, closed int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]uint64(nil), c.acked...), append([]struct {
		tag     uint64
		requeue bool
	}(nil), c.rejected...), c.closed
}

type fakePublisher struct {
	mu       sync.Mutex
	failures int
	messages []Publication
	closed   int
}

func (p *fakePublisher) Publish(_ context.Context, publication Publication) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failures > 0 {
		p.failures--
		return errors.New("temporary publish failure")
	}
	p.messages = append(p.messages, publication.clone())
	return nil
}

func (p *fakePublisher) Close() error {
	p.mu.Lock()
	p.closed++
	p.mu.Unlock()
	return nil
}

type amqpSinkEvent struct {
	Symbol string
	Count  int
}

func TestSourceJSONProcessorAcknowledgesAfterProcessing(t *testing.T) {
	consumer := &fakeConsumer{deliveries: make(chan Delivery, 1)}
	seen := make(chan map[string]any, 1)
	source, err := NewSource(SourceSpec{
		Consumer: consumer,
		Decode:   DecodeJSON,
		AckMode:  AckAfterProcess,
		Emit: func(_ context.Context, _ Delivery, value any) error {
			record, ok := value.(map[string]any)
			if !ok {
				return errors.New("decoded value is not a map")
			}
			seen <- record
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	consumer.deliveries <- Delivery{Body: []byte(`{"symbol":"A","count":2}`), DeliveryTag: 7}
	select {
	case record := <-seen:
		if record["symbol"] != "A" || record["count"].(float64) != 2 {
			t.Fatalf("record = %#v", record)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for decoded delivery")
	}
	if err := waitForAMQP(func() bool {
		acked, _, _ := consumer.snapshot()
		return len(acked) == 1
	}); err != nil {
		t.Fatal(err)
	}
	acked, rejected, _ := consumer.snapshot()
	if !reflect.DeepEqual(acked, []uint64{7}) || len(rejected) != 0 {
		t.Fatalf("ack state = %#v rejected = %#v", acked, rejected)
	}
	if err := source.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func TestSourceRetriesAndRejectsFailedDelivery(t *testing.T) {
	consumer := &fakeConsumer{deliveries: make(chan Delivery, 1)}
	var attempts atomic.Int32
	seenErrors := make(chan error, 1)
	source, err := NewSource(SourceSpec{
		Consumer:       consumer,
		AckMode:        AckAfterProcess,
		RetryAttempts:  2,
		RetryInterval:  time.Millisecond,
		RequeueOnError: true,
		ErrorHandler: func(err error, _ Delivery) {
			seenErrors <- err
		},
		Emit: func(context.Context, Delivery, any) error {
			attempts.Add(1)
			return errors.New("bad event")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	consumer.deliveries <- Delivery{Body: []byte("not-json"), DeliveryTag: 9}
	select {
	case <-seenErrors:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for processor error")
	}
	if err := waitForAMQP(func() bool {
		_, rejected, _ := consumer.snapshot()
		return len(rejected) == 1
	}); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d", attempts.Load())
	}
	acked, rejected, _ := consumer.snapshot()
	if len(acked) != 0 || len(rejected) != 1 || !rejected[0].requeue || rejected[0].tag != 9 {
		t.Fatalf("ack state = %#v rejected = %#v", acked, rejected)
	}
	if err := source.Stop(); err != nil {
		t.Fatal(err)
	}
	if source.State() != connectors.Opened {
		t.Fatalf("state after stop = %s", source.State())
	}
	if err := source.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func TestSourceFactoryRestartsAfterPauseAndStop(t *testing.T) {
	first := &fakeConsumer{deliveries: make(chan Delivery, 1)}
	second := &fakeConsumer{deliveries: make(chan Delivery, 1)}
	var opens atomic.Int32
	seen := make(chan string, 2)
	source, err := NewSource(SourceSpec{
		Open: func() (Consumer, error) {
			if opens.Add(1) == 1 {
				return first, nil
			}
			return second, nil
		},
		Emit: func(_ context.Context, delivery Delivery, _ any) error {
			seen <- string(delivery.Body)
			return nil
		},
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
	first.deliveries <- Delivery{Body: []byte("paused")}
	select {
	case value := <-seen:
		t.Fatalf("paused delivery was processed: %s", value)
	case <-time.After(30 * time.Millisecond):
	}
	if err := source.Resume(); err != nil {
		t.Fatal(err)
	}
	if got := receiveAMQPString(t, seen); got != "paused" {
		t.Fatalf("resumed value = %q", got)
	}
	if err := source.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	second.deliveries <- Delivery{Body: []byte("restarted")}
	if got := receiveAMQPString(t, seen); got != "restarted" {
		t.Fatalf("restarted value = %q", got)
	}
	if opens.Load() != 2 {
		t.Fatalf("open count = %d", opens.Load())
	}
	if err := source.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func TestSinkEncodesRowsRetriesAndHonorsLifecycle(t *testing.T) {
	publisher := &fakePublisher{failures: 1}
	sink, err := NewSink(SinkSpec{
		Publisher:     publisher,
		QueueName:     "events",
		RetryAttempts: 2,
		RetryInterval: time.Millisecond,
		Headers:       map[string]any{"format": "json"},
		ContentType:   "application/json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[amqpSinkEvent](env, "AMQPSinkEvent"); err != nil {
		t.Fatal(err)
	}
	engine := esper.NewEngine(env)
	plan, err := env.Build(esper.From[amqpSinkEvent](env, "AMQPSinkEvent").Query(esper.StatementName("amqp-sink")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	if _, err := deployment.Statements()[0].Subscribe(sink.Write); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), amqpSinkEvent{Symbol: "A", Count: 2}); err != nil {
		t.Fatal(err)
	}
	publisher.mu.Lock()
	messages := append([]Publication(nil), publisher.messages...)
	publisher.mu.Unlock()
	if len(messages) != 1 || string(messages[0].Headers["format"].(string)) != "json" || messages[0].ContentType != "application/json" {
		t.Fatalf("messages = %#v", messages)
	}
	if string(messages[0].Body) != `{"Symbol":"A","Count":2}` {
		t.Fatalf("body = %s", messages[0].Body)
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
}

func TestAMQPValidationAndCodecErrors(t *testing.T) {
	if _, err := NewSource(SourceSpec{}); err == nil {
		t.Fatal("empty source was accepted")
	}
	if _, err := NewSink(SinkSpec{Publisher: &fakePublisher{}}); err == nil {
		t.Fatal("sink without destination was accepted")
	}
	if _, err := NewSink(SinkSpec{Config: &PublisherConfig{}}); err == nil {
		t.Fatal("sink with an empty broker destination was accepted")
	}
	if _, err := DecodeJSON(Delivery{Body: []byte("not-json")}); err == nil {
		t.Fatal("invalid JSON was accepted")
	}
	if _, err := (ConnectionConfig{Host: "localhost", Port: 0}).uri(); err != nil {
		t.Fatal(err)
	}
	if _, err := (ConnectionConfig{URI: "http://bad"}).uri(); err == nil {
		t.Fatal("invalid AMQP URI was accepted")
	}
	if _, err := NewSource(SourceSpec{
		Consumer: &fakeConsumer{deliveries: make(chan Delivery)},
		Open:     func() (Consumer, error) { return &fakeConsumer{deliveries: make(chan Delivery)}, nil },
		Emit:     func(context.Context, Delivery, any) error { return nil },
	}); err == nil {
		t.Fatal("ambiguous source provider was accepted")
	}
}

func TestAMQPDockerRoundTrip(t *testing.T) {
	uri := strings.TrimSpace(os.Getenv("ESPER_AMQP_URL"))
	if uri == "" {
		t.Skip("set ESPER_AMQP_URL to run the RabbitMQ integration")
	}
	base := ConnectionConfig{URI: uri}
	queue := "esper-go-amqp-test-" + time.Now().Format("20060102150405.000000000")
	seen := make(chan map[string]any, 1)
	source, err := NewSource(SourceSpec{
		Config: &ConsumerConfig{
			Connection:        base,
			QueueName:         queue,
			ManualAck:         true,
			DeclareAutoDelete: true,
		},
		AckMode: AckAfterProcess,
		Decode:  DecodeJSON,
		Emit: func(_ context.Context, _ Delivery, value any) error {
			seen <- value.(map[string]any)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	publisher, err := NewRabbitPublisher(PublisherConfig{Connection: base, QueueName: queue, DeclareAutoDelete: true})
	if err != nil {
		_ = source.Destroy()
		t.Fatal(err)
	}
	if err := publisher.Publish(context.Background(), Publication{Body: []byte(`{"value":7}`), ContentType: "application/json"}); err != nil {
		_ = publisher.Close()
		_ = source.Destroy()
		t.Fatal(err)
	}
	select {
	case value := <-seen:
		if value["value"].(float64) != 7 {
			t.Fatalf("value = %#v", value)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for RabbitMQ source")
	}
	if err := publisher.Close(); err != nil {
		t.Fatal(err)
	}
	if err := source.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func TestAMQPDockerSinkRoundTrip(t *testing.T) {
	uri := strings.TrimSpace(os.Getenv("ESPER_AMQP_URL"))
	if uri == "" {
		t.Skip("set ESPER_AMQP_URL to run the RabbitMQ integration")
	}
	base := ConnectionConfig{URI: uri}
	queue := "esper-go-amqp-sink-" + time.Now().Format("20060102150405.000000000")
	consumer, err := NewRabbitConsumer(ConsumerConfig{
		Connection:        base,
		QueueName:         queue,
		DeclareAutoDelete: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deliveries, err := consumer.Consume(ctx)
	if err != nil {
		_ = consumer.Close()
		t.Fatal(err)
	}
	sink, err := NewSink(SinkSpec{
		Config:      &PublisherConfig{Connection: base, QueueName: queue, DeclareAutoDelete: true},
		ContentType: "application/json",
	})
	if err != nil {
		_ = consumer.Close()
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		_ = consumer.Close()
		t.Fatal(err)
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[amqpSinkEvent](env, "AMQPDockerSinkEvent"); err != nil {
		_ = sink.Destroy()
		_ = consumer.Close()
		t.Fatal(err)
	}
	engine := esper.NewEngine(env)
	plan, err := env.Build(esper.From[amqpSinkEvent](env, "AMQPDockerSinkEvent").Query(esper.StatementName("amqp-docker-sink")))
	if err != nil {
		_ = sink.Destroy()
		_ = consumer.Close()
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		_ = sink.Destroy()
		_ = consumer.Close()
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	if _, err := deployment.Statements()[0].Subscribe(sink.Write); err != nil {
		_ = sink.Destroy()
		_ = consumer.Close()
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), amqpSinkEvent{Symbol: "S", Count: 3}); err != nil {
		_ = sink.Destroy()
		_ = consumer.Close()
		t.Fatal(err)
	}
	select {
	case delivery := <-deliveries:
		if string(delivery.Body) != `{"Symbol":"S","Count":3}` || delivery.ContentType != "application/json" {
			t.Fatalf("delivery = %#v", delivery)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for RabbitMQ sink")
	}
	if err := sink.Destroy(); err != nil {
		t.Fatal(err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatal(err)
	}
}

func waitForAMQP(predicate func() bool) error {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
	return errors.New("timed out waiting for AMQP state")
}

func receiveAMQPString(t *testing.T, values <-chan string) string {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for AMQP value")
		return ""
	}
}
