// Package kafka provides EsperIO-style Kafka source and sink adapters. The
// public runtime contracts use small Reader/Writer interfaces so tests and
// alternate Kafka clients do not leak into the rule engine API; the package
// also includes adapters for github.com/segmentio/kafka-go.
package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
	kafkago "github.com/segmentio/kafka-go"
)

// Message is the normalized Kafka record passed to processors and writers.
type Message struct {
	Topic     string
	Key       []byte
	Value     []byte
	Headers   map[string][]byte
	Partition int
	Offset    int64
	Time      time.Time
}

func (m Message) clone() Message {
	m.Key = append([]byte(nil), m.Key...)
	m.Value = append([]byte(nil), m.Value...)
	if m.Headers != nil {
		m.Headers = cloneHeaders(m.Headers)
	}
	return m
}

// Reader is the commit-aware subset of a Kafka consumer used by Source.
// FetchMessage must block on the supplied context and return one record.
type Reader interface {
	FetchMessage(context.Context) (Message, error)
	Commit(context.Context, Message) error
	Close() error
}

// Writer is the producer subset used by Sink.
type Writer interface {
	WriteMessages(context.Context, ...Message) error
	Close() error
}

// KafkaReader adapts a kafka-go Reader to the small Reader contract.
type KafkaReader struct{ reader *kafkago.Reader }

// NewKafkaReader creates a source reader using kafka-go's ReaderConfig. Group
// mode, topic subscriptions, offset reset and broker settings remain explicit
// in the caller-provided config.
func NewKafkaReader(config kafkago.ReaderConfig) Reader {
	return &KafkaReader{reader: kafkago.NewReader(config)}
}

func (r *KafkaReader) FetchMessage(ctx context.Context) (Message, error) {
	message, err := r.reader.FetchMessage(ctx)
	if err != nil {
		return Message{}, err
	}
	return fromKafkaMessage(message), nil
}

func (r *KafkaReader) Commit(ctx context.Context, message Message) error {
	return r.reader.CommitMessages(ctx, toKafkaMessage(message))
}

func (r *KafkaReader) Close() error { return r.reader.Close() }

// KafkaWriter adapts a kafka-go Writer to the small Writer contract.
type KafkaWriter struct {
	writer *kafkago.Writer
	topic  string
}

// NewKafkaWriter creates a producer using kafka-go's WriterConfig.
func NewKafkaWriter(config kafkago.WriterConfig) Writer {
	return &KafkaWriter{writer: kafkago.NewWriter(config), topic: config.Topic}
}

func (w *KafkaWriter) WriteMessages(ctx context.Context, messages ...Message) error {
	converted := make([]kafkago.Message, len(messages))
	for index, message := range messages {
		if w.topic != "" && message.Topic != "" && message.Topic != w.topic {
			return fmt.Errorf("kafka: writer is fixed to topic %q, got %q", w.topic, message.Topic)
		}
		converted[index] = toKafkaMessage(message)
		if w.topic != "" {
			// kafka-go rejects a per-message Topic when WriterConfig.Topic is
			// set; the configured topic remains authoritative.
			converted[index].Topic = ""
		}
	}
	return w.writer.WriteMessages(ctx, converted...)
}

func (w *KafkaWriter) Close() error { return w.writer.Close() }

func fromKafkaMessage(message kafkago.Message) Message {
	headers := make(map[string][]byte, len(message.Headers))
	for _, header := range message.Headers {
		headers[header.Key] = append([]byte(nil), header.Value...)
	}
	return Message{
		Topic:     message.Topic,
		Key:       append([]byte(nil), message.Key...),
		Value:     append([]byte(nil), message.Value...),
		Headers:   headers,
		Partition: message.Partition,
		Offset:    message.Offset,
		Time:      message.Time,
	}
}

func toKafkaMessage(message Message) kafkago.Message {
	headers := make([]kafkago.Header, 0, len(message.Headers))
	for key, value := range message.Headers {
		headers = append(headers, kafkago.Header{Key: key, Value: append([]byte(nil), value...)})
	}
	return kafkago.Message{
		Topic:     message.Topic,
		Key:       append([]byte(nil), message.Key...),
		Value:     append([]byte(nil), message.Value...),
		Headers:   headers,
		Partition: message.Partition,
		Offset:    message.Offset,
		Time:      message.Time,
	}
}

// Decoder turns a Kafka value into an Engine underlying value. The JSON
// decoder below is the Go counterpart of EsperIOKafkaInputProcessorJson.
type Decoder func(Message) (any, error)

// DecodeJSON decodes a Kafka value as a JSON object or array.
func DecodeJSON(message Message) (any, error) {
	var value any
	if err := json.Unmarshal(message.Value, &value); err != nil {
		return nil, fmt.Errorf("kafka: decode JSON topic %q: %w", message.Topic, err)
	}
	return value, nil
}

// CommitMode determines when a source acknowledges a record.
type CommitMode uint8

const (
	// CommitAfterProcess provides at-least-once behavior for the processor.
	CommitAfterProcess CommitMode = iota
	// CommitImmediately acknowledges after fetch, matching auto-commit style
	// delivery and allowing the processor to proceed without redelivery.
	CommitImmediately
)

// SourceSpec describes a Kafka input adapter. Exactly one of Reader or Open
// and exactly one of Emit or Engine must be supplied.
type SourceSpec struct {
	Reader Reader
	Open   func() (Reader, error)

	EventType string
	Decode    Decoder
	Emit      func(context.Context, Message, any) error
	Engine    *esper.Engine

	CommitMode         CommitMode
	RetryAttempts      int
	RetryInterval      time.Duration
	CommitOnError      bool
	ContinueOnError    bool
	TimestampExtractor func(Message) (time.Time, bool)
	ErrorHandler       func(error, Message)
}

// Source is a background Kafka consumer with explicit ack, retry and
// lifecycle behavior.
type Source struct {
	manager *connectors.StateManager
	spec    SourceSpec

	mu     sync.Mutex
	reader Reader
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSource validates and copies a Kafka source specification.
func NewSource(spec SourceSpec) (*Source, error) {
	if (spec.Reader == nil) == (spec.Open == nil) {
		return nil, errors.New("kafka: exactly one of Reader or Open is required")
	}
	if (spec.Emit == nil) == (spec.Engine == nil) {
		return nil, errors.New("kafka: exactly one of Emit or Engine is required")
	}
	if spec.Engine != nil && spec.EventType == "" {
		return nil, errors.New("kafka: EventType is required with Engine")
	}
	if spec.RetryAttempts < 0 {
		return nil, errors.New("kafka: RetryAttempts must not be negative")
	}
	if !spec.ContinueOnError {
		// Keep the safe default explicit: a processor error stops consumption
		// unless the caller opts into log-and-continue behavior.
		spec.ContinueOnError = false
	}
	return &Source{manager: connectors.NewStateManager(), spec: spec}, nil
}

// State returns the source lifecycle state.
func (s *Source) State() connectors.State {
	if s == nil {
		return connectors.Destroyed
	}
	return s.manager.State()
}

// Start opens the reader and starts the consumer goroutine.
func (s *Source) Start() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Start(); err != nil {
		return err
	}
	reader := s.spec.Reader
	var err error
	if s.spec.Open != nil {
		reader, err = s.spec.Open()
	}
	if err != nil {
		_ = s.manager.Stop()
		return fmt.Errorf("kafka: open reader: %w", err)
	}
	if reader == nil {
		_ = s.manager.Stop()
		return errors.New("kafka: reader is nil")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.reader = reader
	s.cancel = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go s.consume(ctx, reader)
	return nil
}

// Pause keeps the reader open but stops processor delivery until Resume.
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

// Stop cancels and closes the reader, returning to OPENED for a factory-based
// restart.
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
	reader := s.reader
	s.cancel = nil
	s.reader = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if reader != nil {
		_ = reader.Close()
	}
	s.wg.Wait()
}

func (s *Source) consume(ctx context.Context, reader Reader) {
	defer s.wg.Done()
	for {
		if err := s.waitStarted(ctx); err != nil {
			return
		}
		message, err := reader.FetchMessage(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) && !errors.Is(err, connectors.ErrStopped) && s.State() != connectors.Destroyed {
				s.report(err, Message{})
			}
			return
		}
		if s.spec.CommitMode == CommitImmediately {
			if err := s.commitWithRetry(ctx, reader, message); err != nil {
				s.report(err, message)
				return
			}
		}
		if err := s.processWithRetry(ctx, message); err != nil {
			s.report(err, message)
			if s.spec.CommitOnError {
				_ = s.commitWithRetry(ctx, reader, message)
			}
			if !s.spec.ContinueOnError {
				return
			}
			continue
		}
		if s.spec.CommitMode == CommitAfterProcess {
			if err := s.commitWithRetry(ctx, reader, message); err != nil {
				s.report(err, message)
				if !s.spec.ContinueOnError {
					return
				}
			}
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
	if extractor := s.spec.TimestampExtractor; extractor != nil && s.spec.Engine != nil {
		if timestamp, ok := extractor(message); ok {
			if err := s.spec.Engine.AdvanceTime(ctx, timestamp); err != nil {
				return err
			}
		}
	}
	value := any(message.Value)
	if s.spec.Decode != nil {
		decoded, err := s.spec.Decode(message)
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

func (s *Source) commitWithRetry(ctx context.Context, reader Reader, message Message) error {
	attempts := retryAttempts(s.spec.RetryAttempts)
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := waitRetry(ctx, s.spec.RetryInterval); err != nil {
				return err
			}
		}
		lastErr = reader.Commit(ctx, message)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (s *Source) report(err error, message Message) {
	if err != nil && s.spec.ErrorHandler != nil {
		s.spec.ErrorHandler(err, message)
	}
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

// SinkSpec configures a Kafka output adapter. A Sink is an Esper Sink and can
// be attached directly to Statement.Subscribe/To-style integrations.
type SinkSpec struct {
	Writer Writer
	Open   func() (Writer, error)
	Topic  string

	Encode     func(esper.Result) ([]byte, error)
	Key        func(esper.Result) ([]byte, error)
	TopicFor   func(esper.Result) string
	Headers    map[string][]byte
	IncludeOld bool

	RetryAttempts int
	RetryInterval time.Duration
}

// Sink writes new (and optionally old) statement results to Kafka.
type Sink struct {
	manager *connectors.StateManager
	spec    SinkSpec

	mu     sync.Mutex
	writer Writer
}

func NewSink(spec SinkSpec) (*Sink, error) {
	if (spec.Writer == nil) == (spec.Open == nil) {
		return nil, errors.New("kafka: exactly one of Writer or Open is required")
	}
	if spec.Topic == "" && spec.TopicFor == nil {
		return nil, errors.New("kafka: Topic or TopicFor is required")
	}
	if spec.RetryAttempts < 0 {
		return nil, errors.New("kafka: RetryAttempts must not be negative")
	}
	spec.Headers = cloneHeaders(spec.Headers)
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
	writer := s.spec.Writer
	var err error
	if s.spec.Open != nil {
		writer, err = s.spec.Open()
	}
	if err != nil {
		_ = s.manager.Stop()
		return fmt.Errorf("kafka: open writer: %w", err)
	}
	if writer == nil {
		_ = s.manager.Stop()
		return errors.New("kafka: writer is nil")
	}
	s.mu.Lock()
	s.writer = writer
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
	s.closeWriter()
	return nil
}

func (s *Sink) Destroy() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Destroy(); err != nil {
		return err
	}
	s.closeWriter()
	return nil
}

func (s *Sink) closeWriter() {
	s.mu.Lock()
	writer := s.writer
	s.writer = nil
	s.mu.Unlock()
	if writer != nil {
		_ = writer.Close()
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
	writer := s.writer
	s.mu.Unlock()
	if writer == nil {
		return errors.New("kafka: sink writer is not open")
	}
	results := append([]esper.Result(nil), batch.New...)
	if s.spec.IncludeOld {
		results = append(results, batch.Old...)
	}
	messages := make([]Message, 0, len(results))
	for _, result := range results {
		message, err := s.encodeResult(result)
		if err != nil {
			return err
		}
		messages = append(messages, message)
	}
	if len(messages) == 0 {
		return nil
	}
	attempts := retryAttempts(s.spec.RetryAttempts)
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := waitRetry(ctx, s.spec.RetryInterval); err != nil {
				return err
			}
		}
		lastErr = writer.WriteMessages(ctx, messages...)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (s *Sink) encodeResult(result esper.Result) (Message, error) {
	valueEncoder := s.spec.Encode
	if valueEncoder == nil {
		valueEncoder = EncodeResultJSON
	}
	value, err := valueEncoder(result)
	if err != nil {
		return Message{}, err
	}
	message := Message{Topic: s.spec.Topic, Value: value, Headers: cloneHeaders(s.spec.Headers)}
	if s.spec.TopicFor != nil {
		message.Topic = s.spec.TopicFor(result)
	}
	if message.Topic == "" {
		return Message{}, errors.New("kafka: resolved topic is empty")
	}
	if s.spec.Key != nil {
		message.Key, err = s.spec.Key(result)
		if err != nil {
			return Message{}, err
		}
	}
	return message, nil
}

// EncodeResultJSON emits the underlying event or ordered Row map as JSON.
func EncodeResultJSON(result esper.Result) ([]byte, error) {
	if row, ok := result.Row(); ok {
		return encodeOrderedJSON(row.Schema().Fields(), row.Get)
	}
	if event, ok := result.Event(); ok {
		return encodeOrderedJSON(event.Schema().Fields(), event.Get)
	}
	return nil, errors.New("kafka: result has no event or row")
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
			return nil, fmt.Errorf("kafka: encode field %q: %w", field.Name, err)
		}
		result.Write(name)
		result.WriteByte(':')
		result.Write(value)
	}
	result.WriteByte('}')
	return result.Bytes(), nil
}

func cloneHeaders(values map[string][]byte) map[string][]byte {
	if values == nil {
		return nil
	}
	result := make(map[string][]byte, len(values))
	for key, value := range values {
		result[key] = append([]byte(nil), value...)
	}
	return result
}

var _ esper.Sink = (*Sink)(nil)
