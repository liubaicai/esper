package jms

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
)

type fakePublisher struct {
	mu       sync.Mutex
	failures int
	messages []Message
	closed   int
}

func (p *fakePublisher) Send(_ context.Context, message Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failures > 0 {
		p.failures--
		return errors.New("temporary JMS bridge failure")
	}
	p.messages = append(p.messages, message.clone())
	return nil
}

func (p *fakePublisher) Close() error {
	p.mu.Lock()
	p.closed++
	p.mu.Unlock()
	return nil
}

func TestChannelSourceDecodesJSONAcknowledgesAndRespectsPause(t *testing.T) {
	publisher, consumer := NewChannelPair(2)
	seen := make(chan map[string]any, 1)
	source, err := NewSource(SourceSpec{
		Consumer: consumer,
		Decode:   DecodeJSONText,
		Emit: func(_ context.Context, _ Message, value any) error {
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
	if err := source.Pause(); err != nil {
		t.Fatal(err)
	}
	if err := publisher.Send(context.Background(), Message{Kind: TextMessage, Text: `{"name":"A","count":2}`}); err != nil {
		t.Fatal(err)
	}
	select {
	case value := <-seen:
		t.Fatalf("paused value = %#v", value)
	case <-time.After(30 * time.Millisecond):
	}
	if err := source.Resume(); err != nil {
		t.Fatal(err)
	}
	select {
	case value := <-seen:
		if value["name"] != "A" || value["count"].(float64) != 2 {
			t.Fatalf("value = %#v", value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for JMS bridge message")
	}
	if err := waitJMS(func() bool { return len(consumer.Acknowledged()) == 1 }); err != nil {
		t.Fatal(err)
	}
	acked := consumer.Acknowledged()
	if len(acked) != 1 || acked[0].Kind != TextMessage {
		t.Fatalf("acked = %#v", acked)
	}
	if err := source.Destroy(); err != nil {
		t.Fatal(err)
	}
	_ = publisher.Close()
}

func TestSourceRetriesAndFactoryRestart(t *testing.T) {
	firstPublisher, firstConsumer := NewChannelPair(1)
	secondPublisher, secondConsumer := NewChannelPair(1)
	var opens atomic.Int32
	var attempts atomic.Int32
	source, err := NewSource(SourceSpec{
		Open: func() (Consumer, error) {
			if opens.Add(1) == 1 {
				return firstConsumer, nil
			}
			return secondConsumer, nil
		},
		RetryAttempts: 2,
		RetryInterval: time.Millisecond,
		ErrorHandler:  func(error, Message) {},
		Emit: func(context.Context, Message, any) error {
			if attempts.Add(1) <= 2 {
				return errors.New("decode/processor failure")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	if err := firstPublisher.Send(context.Background(), Message{Kind: ObjectMessage, Object: "bad"}); err != nil {
		t.Fatal(err)
	}
	if err := waitJMS(func() bool { return attempts.Load() == 2 }); err != nil {
		t.Fatal(err)
	}
	if len(firstConsumer.Acknowledged()) != 0 {
		t.Fatal("failed message was acknowledged")
	}
	if err := source.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := source.Start(); err != nil {
		t.Fatal(err)
	}
	if err := secondPublisher.Send(context.Background(), Message{Kind: ObjectMessage, Object: "ok"}); err != nil {
		t.Fatal(err)
	}
	if err := waitJMS(func() bool { return attempts.Load() == 3 }); err != nil {
		t.Fatal(err)
	}
	if len(secondConsumer.Acknowledged()) != 1 {
		t.Fatal("restarted message was not acknowledged")
	}
	if opens.Load() != 2 {
		t.Fatalf("open count = %d", opens.Load())
	}
	_ = source.Destroy()
	_ = firstPublisher.Close()
	_ = secondPublisher.Close()
}

func TestSinkMarshalsTextWithJavaTypePropertyAndRetries(t *testing.T) {
	publisher := &fakePublisher{failures: 1}
	sink, err := NewSink(SinkSpec{
		Publisher:     publisher,
		Kind:          TextMessage,
		EventType:     "JMSEvent",
		RetryAttempts: 2,
		RetryInterval: time.Millisecond,
		Properties:    map[string]any{"source": "go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterMap(env, "JMSEvent", []esper.FieldSpec{{Name: "name", Type: reflect.TypeOf("")}, {Name: "count", Type: reflect.TypeOf(int(0))}}); err != nil {
		t.Fatal(err)
	}
	engine := esper.NewEngine(env)
	plan, err := env.Build(esper.FromAny(env, "JMSEvent").Query(esper.StatementName("jms-sink")))
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
	if err := engine.SendRecord(context.Background(), "JMSEvent", map[string]any{"name": "A", "count": 2}); err != nil {
		t.Fatal(err)
	}
	publisher.mu.Lock()
	messages := append([]Message(nil), publisher.messages...)
	publisher.mu.Unlock()
	if len(messages) != 1 || messages[0].Kind != TextMessage || messages[0].Text != `{"name":"A","count":2}` {
		t.Fatalf("messages = %#v", messages)
	}
	if messages[0].Properties[JSONEventTypeProperty] != "JMSEvent" || messages[0].Properties["source"] != "go" {
		t.Fatalf("properties = %#v", messages[0].Properties)
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

func TestJMSMapObjectCodecAndValidation(t *testing.T) {
	mapValue, err := DecodeAny(Message{Kind: MapMessage, Fields: map[string]any{"a": 1}})
	if err != nil {
		t.Fatal(err)
	}
	mapValue.(map[string]any)["a"] = 2
	original := Message{Kind: MapMessage, Fields: map[string]any{"a": 1}}
	if original.Fields["a"] != 1 {
		t.Fatal("DecodeAny did not copy map fields")
	}
	if _, err := DecodeJSONText(Message{Kind: TextMessage, Text: "bad"}); err == nil {
		t.Fatal("invalid JSON text was accepted")
	}
	if _, err := NewSource(SourceSpec{}); err == nil {
		t.Fatal("empty source was accepted")
	}
	if _, err := NewSink(SinkSpec{Publisher: &fakePublisher{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSink(SinkSpec{Publisher: &fakePublisher{}, Kind: Kind(99)}); err == nil {
		t.Fatal("invalid message kind was accepted")
	}
}

func waitJMS(predicate func() bool) error {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
	return errors.New("timed out waiting for JMS bridge state")
}
