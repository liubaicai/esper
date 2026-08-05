// Package jms defines the Go side of the EsperIO Spring JMS bridge.
//
// Go does not provide a JVM JMS runtime. The package therefore exposes a
// provider-neutral Consumer/Publisher contract that a process-outside bridge
// (JMS, Spring JMS, or another message provider) can implement. It preserves
// the Java adapter's MapMessage, TextMessage, ObjectMessage and BytesMessage
// shapes, Esper event-type properties, acknowledge-after-processing behavior
// and the common Esper connector lifecycle.
package jms

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
)

const (
	// MapEventTypeProperty is the Java InputAdapter property used on MapMessage.
	MapEventTypeProperty = "com.espertech.esper.runtime.client.util.InputAdapter_maptype"
	// JSONEventTypeProperty is the Java InputAdapter property used on TextMessage.
	JSONEventTypeProperty = "com.espertech.esper.runtime.client.util.InputAdapter_jsontype"
)

// Kind identifies the JMS body shape represented by Message.
type Kind uint8

const (
	MapMessage Kind = iota
	TextMessage
	ObjectMessage
	BytesMessage
)

func (k Kind) String() string {
	switch k {
	case MapMessage:
		return "MAP"
	case TextMessage:
		return "TEXT"
	case ObjectMessage:
		return "OBJECT"
	case BytesMessage:
		return "BYTES"
	default:
		return fmt.Sprintf("KIND(%d)", k)
	}
}

// Message is the bridge-neutral JMS message representation.
type Message struct {
	Kind          Kind
	Fields        map[string]any
	Text          string
	Object        any
	Body          []byte
	Properties    map[string]any
	Headers       map[string]any
	ID            string
	CorrelationID string
	Timestamp     time.Time
}

func (m Message) clone() Message {
	m.Fields = cloneValues(m.Fields)
	m.Properties = cloneValues(m.Properties)
	m.Headers = cloneValues(m.Headers)
	m.Body = append([]byte(nil), m.Body...)
	return m
}

// Consumer is implemented by a JMS client or process-outside bridge.
// Receive must honor context cancellation. Acknowledge is called only when
// Source's AckMode requires it; an unacknowledged message remains subject to
// the provider's redelivery policy.
type Consumer interface {
	Receive(context.Context) (Message, error)
	Acknowledge(context.Context, Message) error
	Close() error
}

// Publisher is implemented by a JMS client or process-outside bridge.
type Publisher interface {
	Send(context.Context, Message) error
	Close() error
}

// ChannelPublisher and ChannelConsumer form a deterministic in-memory bridge.
// They are useful for tests, examples and a process adapter that forwards the
// channel to a JVM JMS client.
type ChannelPublisher struct{ bus *channelBus }
type ChannelConsumer struct{ bus *channelBus }

type channelBus struct {
	messages chan Message
	done     chan struct{}
	once     sync.Once
	mu       sync.Mutex
	acked    []Message
}

// NewChannelPair creates an in-memory publisher/consumer pair.
func NewChannelPair(buffer int) (*ChannelPublisher, *ChannelConsumer) {
	if buffer < 0 {
		buffer = 0
	}
	bus := &channelBus{messages: make(chan Message, buffer), done: make(chan struct{})}
	return &ChannelPublisher{bus: bus}, &ChannelConsumer{bus: bus}
}

func (p *ChannelPublisher) Send(ctx context.Context, message Message) error {
	if p == nil || p.bus == nil {
		return connectors.ErrDestroyed
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	message = message.clone()
	select {
	case p.bus.messages <- message:
		return nil
	case <-p.bus.done:
		return connectors.ErrDestroyed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *ChannelConsumer) Receive(ctx context.Context) (Message, error) {
	if c == nil || c.bus == nil {
		return Message{}, connectors.ErrDestroyed
	}
	select {
	case message := <-c.bus.messages:
		return message.clone(), nil
	case <-c.bus.done:
		return Message{}, connectors.ErrDestroyed
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}

func (c *ChannelConsumer) Acknowledge(_ context.Context, message Message) error {
	if c == nil || c.bus == nil {
		return connectors.ErrDestroyed
	}
	c.bus.mu.Lock()
	c.bus.acked = append(c.bus.acked, message.clone())
	c.bus.mu.Unlock()
	return nil
}

// Acknowledged returns messages acknowledged through this channel bridge.
func (c *ChannelConsumer) Acknowledged() []Message {
	if c == nil || c.bus == nil {
		return nil
	}
	c.bus.mu.Lock()
	defer c.bus.mu.Unlock()
	result := make([]Message, len(c.bus.acked))
	for i, message := range c.bus.acked {
		result[i] = message.clone()
	}
	return result
}

func (p *ChannelPublisher) Close() error {
	if p == nil || p.bus == nil {
		return nil
	}
	p.bus.once.Do(func() { close(p.bus.done) })
	return nil
}

func (c *ChannelConsumer) Close() error {
	if c == nil || c.bus == nil {
		return nil
	}
	c.bus.once.Do(func() { close(c.bus.done) })
	return nil
}

// Decoder turns a JMS bridge message into an Engine underlying value.
type Decoder func(Message) (any, error)

// DecodeAny matches JMSDefaultAnyMessageUnmarshaller for provider-neutral
// values. Map and Text messages carrying the Java event-type properties are
// returned as ordinary maps; Object and Bytes messages retain their payload.
func DecodeAny(message Message) (any, error) {
	switch message.Kind {
	case MapMessage:
		return cloneValues(message.Fields), nil
	case TextMessage:
		if _, ok := message.Properties[JSONEventTypeProperty]; ok {
			var value any
			if err := json.Unmarshal([]byte(message.Text), &value); err != nil {
				return nil, fmt.Errorf("jms: decode JSON text: %w", err)
			}
			return value, nil
		}
		return message.Text, nil
	case ObjectMessage:
		return message.Object, nil
	case BytesMessage:
		return append([]byte(nil), message.Body...), nil
	default:
		return nil, fmt.Errorf("jms: unsupported message kind %d", message.Kind)
	}
}

// DecodeJSONText decodes a TextMessage body as JSON regardless of its type
// property. This is useful for bridges that carry the Java property separately.
func DecodeJSONText(message Message) (any, error) {
	var value any
	if err := json.Unmarshal([]byte(message.Text), &value); err != nil {
		return nil, fmt.Errorf("jms: decode JSON text: %w", err)
	}
	return value, nil
}

// DecodeGOBObject is a Go-only ObjectMessage codec. It is not compatible with
// Java Serializable/ObjectMessage bytes.
func DecodeGOBObject(message Message) (any, error) {
	var value any
	if err := gob.NewDecoder(bytes.NewReader(message.Body)).Decode(&value); err != nil {
		return nil, fmt.Errorf("jms: decode gob object: %w", err)
	}
	return value, nil
}

// AckMode determines when Source acknowledges an input message.
type AckMode uint8

const (
	// AckAfterProcess matches SpringJMSTemplateInputAdapter's normal path.
	AckAfterProcess AckMode = iota
	// AckImmediately acknowledges before unmarshal and Engine delivery.
	AckImmediately
	// AckAuto leaves acknowledgement to the provider/container.
	AckAuto
)

// SourceSpec describes a JMS input bridge.
type SourceSpec struct {
	Consumer Consumer
	Open     func() (Consumer, error)

	EventType string
	Decode    Decoder
	Emit      func(context.Context, Message, any) error
	Engine    *esper.Engine

	AckMode         AckMode
	RetryAttempts   int
	RetryInterval   time.Duration
	ContinueOnError bool
	ErrorHandler    func(error, Message)
}

// Source receives messages, converts them, and sends them to Emit or Engine.
type Source struct {
	manager *connectors.StateManager
	spec    SourceSpec

	mu       sync.Mutex
	consumer Consumer
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewSource(spec SourceSpec) (*Source, error) {
	if (spec.Consumer == nil) == (spec.Open == nil) {
		return nil, errors.New("jms: exactly one of Consumer or Open is required")
	}
	if (spec.Emit == nil) == (spec.Engine == nil) {
		return nil, errors.New("jms: exactly one of Emit or Engine is required")
	}
	if spec.Engine != nil && spec.EventType == "" {
		return nil, errors.New("jms: EventType is required with Engine")
	}
	if spec.AckMode > AckAuto {
		return nil, errors.New("jms: invalid AckMode")
	}
	if spec.RetryAttempts < 0 {
		return nil, errors.New("jms: RetryAttempts must not be negative")
	}
	return &Source{manager: connectors.NewStateManager(), spec: spec}, nil
}

func (s *Source) State() connectors.State {
	if s == nil {
		return connectors.Destroyed
	}
	return s.manager.State()
}

func (s *Source) Start() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Start(); err != nil {
		return err
	}
	consumer := s.spec.Consumer
	var err error
	if s.spec.Open != nil {
		consumer, err = s.spec.Open()
	}
	if err != nil {
		_ = s.manager.Stop()
		return fmt.Errorf("jms: open consumer: %w", err)
	}
	if consumer == nil {
		_ = s.manager.Stop()
		return errors.New("jms: consumer is nil")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.consumer = consumer
	s.cancel = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go s.consume(ctx, consumer)
	return nil
}

func (s *Source) Pause() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Pause()
}

func (s *Source) Resume() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Resume()
}

func (s *Source) Stop() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Stop(); err != nil {
		return err
	}
	s.shutdown()
	return nil
}

func (s *Source) Destroy() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Destroy(); err != nil {
		return err
	}
	s.shutdown()
	return nil
}

func (s *Source) shutdown() {
	s.mu.Lock()
	cancel := s.cancel
	consumer := s.consumer
	s.cancel = nil
	s.consumer = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if consumer != nil {
		_ = consumer.Close()
	}
	s.wg.Wait()
}

func (s *Source) consume(ctx context.Context, consumer Consumer) {
	defer s.wg.Done()
	for {
		if err := s.waitStarted(ctx); err != nil {
			return
		}
		message, err := consumer.Receive(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) && s.State() != connectors.Destroyed {
				s.report(err, Message{})
			}
			return
		}
		// A provider may deliver while Pause races with Receive. Hold the
		// message until Resume so paused adapters do not process it.
		if err := s.waitStarted(ctx); err != nil {
			return
		}
		if !s.handleMessage(ctx, consumer, message) {
			return
		}
	}
}

func (s *Source) waitStarted(ctx context.Context) error {
	for {
		switch s.State() {
		case connectors.Started:
			return nil
		case connectors.Paused:
			changed := s.manager.Changes()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-changed:
			}
		case connectors.Destroyed:
			return connectors.ErrDestroyed
		default:
			return connectors.ErrStopped
		}
	}
}

func (s *Source) handleMessage(ctx context.Context, consumer Consumer, message Message) bool {
	if s.spec.AckMode == AckImmediately {
		if err := consumer.Acknowledge(ctx, message); err != nil {
			s.report(err, message)
			return s.spec.ContinueOnError
		}
	}
	if err := s.processWithRetry(ctx, message); err != nil {
		s.report(err, message)
		return s.spec.ContinueOnError
	}
	if s.spec.AckMode == AckAfterProcess {
		if err := consumer.Acknowledge(ctx, message); err != nil {
			s.report(err, message)
			return s.spec.ContinueOnError
		}
	}
	return true
}

func (s *Source) processWithRetry(ctx context.Context, message Message) error {
	attempts := retryAttempts(s.spec.RetryAttempts)
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := waitRetry(ctx, s.spec.RetryInterval); err != nil {
				return err
			}
		}
		lastErr = s.process(ctx, message)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (s *Source) process(ctx context.Context, message Message) error {
	value := any(message.Object)
	if s.spec.Decode != nil {
		decoded, err := s.spec.Decode(message)
		if err != nil {
			return err
		}
		value = decoded
	} else {
		decoded, err := DecodeAny(message)
		if err != nil {
			return err
		}
		value = decoded
	}
	if s.spec.Emit != nil {
		return s.spec.Emit(ctx, message, value)
	}
	if record, ok := value.(map[string]any); ok {
		return s.spec.Engine.SendRecord(ctx, s.spec.EventType, record)
	}
	if event, ok := value.(esper.Event); ok {
		return s.spec.Engine.Send(ctx, s.spec.EventType, event.Underlying())
	}
	return s.spec.Engine.Send(ctx, s.spec.EventType, value)
}

func (s *Source) report(err error, message Message) {
	if err != nil && s.spec.ErrorHandler != nil {
		s.spec.ErrorHandler(err, message)
	}
}

// SinkSpec configures a JMS output bridge. The default marshaller is a MapMessage
// equivalent; set Kind to TextMessage/ObjectMessage/BytesMessage or provide a
// custom Marshal function for the Java-compatible adapter behavior required by
// the destination.
type SinkSpec struct {
	Publisher Publisher
	Open      func() (Publisher, error)

	Kind       Kind
	EventType  string
	Marshal    func(esper.Result) (Message, error)
	Properties map[string]any
	Headers    map[string]any
	IncludeOld bool

	RetryAttempts int
	RetryInterval time.Duration
}

// Sink publishes result events as bridge messages.
type Sink struct {
	manager *connectors.StateManager
	spec    SinkSpec

	mu        sync.Mutex
	publisher Publisher
}

func NewSink(spec SinkSpec) (*Sink, error) {
	if (spec.Publisher == nil) == (spec.Open == nil) {
		return nil, errors.New("jms: exactly one of Publisher or Open is required")
	}
	if spec.Kind > BytesMessage {
		return nil, errors.New("jms: invalid message Kind")
	}
	if spec.RetryAttempts < 0 {
		return nil, errors.New("jms: RetryAttempts must not be negative")
	}
	spec.Properties = cloneValues(spec.Properties)
	spec.Headers = cloneValues(spec.Headers)
	return &Sink{manager: connectors.NewStateManager(), spec: spec}, nil
}

func (s *Sink) State() connectors.State {
	if s == nil {
		return connectors.Destroyed
	}
	return s.manager.State()
}

func (s *Sink) Start() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Start(); err != nil {
		return err
	}
	publisher := s.spec.Publisher
	var err error
	if s.spec.Open != nil {
		publisher, err = s.spec.Open()
	}
	if err != nil {
		_ = s.manager.Stop()
		return fmt.Errorf("jms: open publisher: %w", err)
	}
	if publisher == nil {
		_ = s.manager.Stop()
		return errors.New("jms: publisher is nil")
	}
	s.mu.Lock()
	s.publisher = publisher
	s.mu.Unlock()
	return nil
}

func (s *Sink) Pause() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Pause()
}

func (s *Sink) Resume() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Resume()
}

func (s *Sink) Stop() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Stop(); err != nil {
		return err
	}
	s.closePublisher()
	return nil
}

func (s *Sink) Destroy() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Destroy(); err != nil {
		return err
	}
	s.closePublisher()
	return nil
}

func (s *Sink) closePublisher() {
	s.mu.Lock()
	publisher := s.publisher
	s.publisher = nil
	s.mu.Unlock()
	if publisher != nil {
		_ = publisher.Close()
	}
}

// Write implements esper.Sink.
func (s *Sink) Write(ctx context.Context, batch esper.ResultBatch) error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.RequireStarted(); err != nil {
		return err
	}
	s.mu.Lock()
	publisher := s.publisher
	s.mu.Unlock()
	if publisher == nil {
		return errors.New("jms: sink publisher is not open")
	}
	results := append([]esper.Result(nil), batch.New...)
	if s.spec.IncludeOld {
		results = append(results, batch.Old...)
	}
	for _, result := range results {
		message, err := s.encodeResult(result)
		if err != nil {
			return err
		}
		if err := sendWithRetry(ctx, publisher, message, s.spec.RetryAttempts, s.spec.RetryInterval); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sink) encodeResult(result esper.Result) (Message, error) {
	var message Message
	var err error
	if s.spec.Marshal != nil {
		message, err = s.spec.Marshal(result)
	} else {
		switch s.spec.Kind {
		case MapMessage:
			message, err = MarshalMap(result)
		case TextMessage:
			message, err = MarshalJSONText(result)
		case ObjectMessage:
			message, err = MarshalObject(result)
		case BytesMessage:
			message, err = MarshalJSONBytes(result)
		}
	}
	if err != nil {
		return Message{}, err
	}
	message.Properties = mergeValues(s.spec.Properties, message.Properties)
	message.Headers = mergeValues(s.spec.Headers, message.Headers)
	if s.spec.EventType != "" {
		switch message.Kind {
		case MapMessage:
			message.Properties[MapEventTypeProperty] = s.spec.EventType
		case TextMessage:
			message.Properties[JSONEventTypeProperty] = s.spec.EventType
		}
	}
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}
	return message, nil
}

// MarshalMap creates the default Java-style MapMessage shape.
func MarshalMap(result esper.Result) (Message, error) {
	if row, ok := result.Row(); ok {
		return Message{Kind: MapMessage, Fields: rowValues(row)}, nil
	}
	if event, ok := result.Event(); ok {
		return Message{Kind: MapMessage, Fields: eventValues(event)}, nil
	}
	return Message{}, errors.New("jms: result has no event or row")
}

// MarshalJSONText creates a TextMessage with deterministic schema-order JSON.
func MarshalJSONText(result esper.Result) (Message, error) {
	body, err := EncodeResultJSON(result)
	if err != nil {
		return Message{}, err
	}
	return Message{Kind: TextMessage, Text: string(body)}, nil
}

// MarshalJSONBytes creates a BytesMessage containing deterministic JSON.
func MarshalJSONBytes(result esper.Result) (Message, error) {
	body, err := EncodeResultJSON(result)
	if err != nil {
		return Message{}, err
	}
	return Message{Kind: BytesMessage, Body: body}, nil
}

// MarshalObject creates an ObjectMessage equivalent. The bridge decides how
// the Object value is serialized for the destination provider.
func MarshalObject(result esper.Result) (Message, error) {
	if value := result.Underlying(); value != nil {
		return Message{Kind: ObjectMessage, Object: value}, nil
	}
	return Message{}, errors.New("jms: result has no underlying object")
}

// EncodeResultJSON emits event/row fields in schema order.
func EncodeResultJSON(result esper.Result) ([]byte, error) {
	if row, ok := result.Row(); ok {
		return encodeOrderedJSON(row.Schema().Fields(), row.Get)
	}
	if event, ok := result.Event(); ok {
		return encodeOrderedJSON(event.Schema().Fields(), event.Get)
	}
	return nil, errors.New("jms: result has no event or row")
}

func rowValues(row esper.Row) map[string]any {
	result := make(map[string]any, len(row.Schema().Fields()))
	for _, field := range row.Schema().Fields() {
		result[field.Name] = row.Get(field.Name).Any()
	}
	return result
}

func eventValues(event esper.Event) map[string]any {
	result := make(map[string]any, len(event.Schema().Fields()))
	for _, field := range event.Schema().Fields() {
		result[field.Name] = event.Get(field.Name).Any()
	}
	return result
}

func encodeOrderedJSON(fields []esper.FieldSpec, get func(string) esper.Value) ([]byte, error) {
	var result bytes.Buffer
	result.WriteByte('{')
	for index, field := range fields {
		if index > 0 {
			result.WriteByte(',')
		}
		name, err := json.Marshal(field.Name)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(get(field.Name).Any())
		if err != nil {
			return nil, fmt.Errorf("jms: encode field %q: %w", field.Name, err)
		}
		result.Write(name)
		result.WriteByte(':')
		result.Write(value)
	}
	result.WriteByte('}')
	return result.Bytes(), nil
}

func sendWithRetry(ctx context.Context, publisher Publisher, message Message, attempts int, interval time.Duration) error {
	tries := retryAttempts(attempts)
	var lastErr error
	for attempt := 0; attempt < tries; attempt++ {
		if attempt > 0 {
			if err := waitRetry(ctx, interval); err != nil {
				return err
			}
		}
		lastErr = publisher.Send(ctx, message.clone())
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func mergeValues(base, overlay map[string]any) map[string]any {
	result := cloneValues(base)
	if result == nil {
		result = make(map[string]any)
	}
	for key, value := range overlay {
		result[key] = value
	}
	return result
}

func cloneValues(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case []byte:
			result[key] = append([]byte(nil), typed...)
		case map[string]any:
			result[key] = cloneValues(typed)
		default:
			result[key] = value
		}
	}
	return result
}

func retryAttempts(configured int) int {
	if configured <= 0 {
		return 1
	}
	return configured
}

func waitRetry(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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

var _ esper.Sink = (*Sink)(nil)
