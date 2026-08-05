// Package amqp provides EsperIO-style AMQP source and sink adapters.
//
// The public contracts intentionally depend on small Consumer and Publisher
// interfaces. This keeps broker lifecycle, acknowledgement policy and
// serialization testable without making the rest of the rule engine depend on
// a particular AMQP client. NewRabbitConsumer and NewRabbitPublisher provide
// the production adapter for RabbitMQ through amqp091-go.
package amqp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
	amqp091 "github.com/rabbitmq/amqp091-go"
)

// Delivery is the broker-neutral representation of an AMQP delivery.
type Delivery struct {
	Body            []byte
	Headers         map[string]any
	ContentType     string
	ContentEncoding string
	DeliveryTag     uint64
	Exchange        string
	RoutingKey      string
	Redelivered     bool
	MessageID       string
	CorrelationID   string
	Timestamp       time.Time
}

func (d Delivery) clone() Delivery {
	d.Body = append([]byte(nil), d.Body...)
	d.Headers = cloneTable(d.Headers)
	return d
}

// Consumer is the acknowledgement-aware subset needed by Source.
type Consumer interface {
	Consume(context.Context) (<-chan Delivery, error)
	Ack(context.Context, Delivery) error
	Reject(context.Context, Delivery, bool) error
	Close() error
}

// Publication is the broker-neutral representation sent by Publisher.
type Publication struct {
	Body            []byte
	Headers         map[string]any
	ContentType     string
	ContentEncoding string
	DeliveryMode    uint8
	Priority        uint8
	Exchange        string
	RoutingKey      string
	MessageID       string
	CorrelationID   string
	Timestamp       time.Time
}

func (p Publication) clone() Publication {
	p.Body = append([]byte(nil), p.Body...)
	p.Headers = cloneTable(p.Headers)
	return p
}

// Publisher is the synchronous publish contract used by Sink.
type Publisher interface {
	Publish(context.Context, Publication) error
	Close() error
}

// ConnectionConfig contains the RabbitMQ connection settings. URI takes
// precedence; the individual fields mirror the Java EsperIO settings and are
// convenient for local deployments.
type ConnectionConfig struct {
	URI               string
	Host              string
	Port              int
	Username          string
	Password          string
	VHost             string
	Heartbeat         time.Duration
	ConnectionTimeout time.Duration
	TLSConfig         *tls.Config
}

func (c ConnectionConfig) uri() (string, error) {
	if strings.TrimSpace(c.URI) != "" {
		parsed, err := url.Parse(c.URI)
		if err != nil || (parsed.Scheme != "amqp" && parsed.Scheme != "amqps") || parsed.Host == "" {
			return "", fmt.Errorf("amqp: invalid URI %q", c.URI)
		}
		return c.URI, nil
	}
	host := c.Host
	if host == "" {
		host = "localhost"
	}
	port := c.Port
	if port == 0 {
		port = 5672
	}
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("amqp: invalid port %d", port)
	}
	username := c.Username
	password := c.Password
	if username == "" && password == "" {
		username, password = "guest", "guest"
	}
	vhost := c.VHost
	if vhost == "" {
		vhost = "/"
	}
	path := "/" + strings.TrimPrefix(vhost, "/")
	parsed := &url.URL{
		Scheme: "amqp",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   path,
	}
	if username != "" {
		parsed.User = url.UserPassword(username, password)
	}
	return parsed.String(), nil
}

func dial(config ConnectionConfig) (*amqp091.Connection, error) {
	uri, err := config.uri()
	if err != nil {
		return nil, err
	}
	dialConfig := amqp091.Config{
		Heartbeat:       config.Heartbeat,
		TLSClientConfig: config.TLSConfig,
	}
	if config.ConnectionTimeout > 0 {
		timeout := config.ConnectionTimeout
		dialConfig.Dial = func(network, address string) (net.Conn, error) {
			return net.DialTimeout(network, address, timeout)
		}
	}
	connection, err := amqp091.DialConfig(uri, dialConfig)
	if err != nil {
		return nil, fmt.Errorf("amqp: connect: %w", err)
	}
	return connection, nil
}

// ConsumerConfig describes the queue/exchange declaration and RabbitMQ
// consumer options. ManualAck defaults to false, matching Java EsperIO's
// consumeAutoAck=true. Set it to true when using AckAfterProcess or
// AckImmediately.
type ConsumerConfig struct {
	Connection ConnectionConfig

	QueueName         string
	Exchange          string
	RoutingKey        string
	PrefetchCount     int
	ConsumerTag       string
	ManualAck         bool
	DeclareDurable    bool
	DeclareExclusive  bool
	DeclareAutoDelete bool
	DeclareArguments  map[string]any
}

// RabbitConsumer adapts one RabbitMQ connection/channel to Consumer.
type RabbitConsumer struct {
	connection *amqp091.Connection
	channel    *amqp091.Channel
	queue      string
	config     ConsumerConfig

	mu        sync.Mutex
	closed    bool
	closeOnce sync.Once
}

// NewRabbitConsumer opens a connection, declares the queue and binding, and
// returns a Consumer ready for Source.Start. A blank QueueName requests a
// server-generated exclusive auto-delete queue.
func NewRabbitConsumer(config ConsumerConfig) (Consumer, error) {
	connection, err := dial(config.Connection)
	if err != nil {
		return nil, err
	}
	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("amqp: open consumer channel: %w", err)
	}
	cleanup := func(cause error) (Consumer, error) {
		_ = channel.Close()
		_ = connection.Close()
		return nil, cause
	}
	if config.PrefetchCount > 0 {
		if err := channel.Qos(config.PrefetchCount, 0, false); err != nil {
			return cleanup(fmt.Errorf("amqp: set prefetch: %w", err))
		}
	}
	if config.Exchange != "" {
		if err := channel.ExchangeDeclarePassive(config.Exchange, "direct", true, false, false, false, nil); err != nil {
			return cleanup(fmt.Errorf("amqp: inspect exchange %q: %w", config.Exchange, err))
		}
	}
	queue, err := channel.QueueDeclare(
		config.QueueName,
		config.DeclareDurable,
		config.DeclareAutoDelete,
		config.DeclareExclusive || config.QueueName == "",
		false,
		toTable(config.DeclareArguments),
	)
	if err != nil {
		return cleanup(fmt.Errorf("amqp: declare queue %q: %w", config.QueueName, err))
	}
	if config.Exchange != "" && config.RoutingKey != "" {
		if err := channel.QueueBind(queue.Name, config.Exchange, config.RoutingKey, false, nil); err != nil {
			return cleanup(fmt.Errorf("amqp: bind queue %q: %w", queue.Name, err))
		}
	}
	return &RabbitConsumer{connection: connection, channel: channel, queue: queue.Name, config: config}, nil
}

// QueueName returns the declared queue, including a generated name.
func (c *RabbitConsumer) QueueName() string {
	if c == nil {
		return ""
	}
	return c.queue
}

func (c *RabbitConsumer) Consume(ctx context.Context) (<-chan Delivery, error) {
	if c == nil {
		return nil, connectors.ErrDestroyed
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, connectors.ErrDestroyed
	}
	channel := c.channel
	queue := c.queue
	tag := c.config.ConsumerTag
	autoAck := !c.config.ManualAck
	c.mu.Unlock()
	raw, err := channel.ConsumeWithContext(ctx, queue, tag, autoAck, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("amqp: consume queue %q: %w", queue, err)
	}
	output := make(chan Delivery)
	go func() {
		defer close(output)
		for message := range raw {
			delivery := fromRabbitDelivery(message)
			select {
			case output <- delivery:
			case <-ctx.Done():
				return
			}
		}
	}()
	return output, nil
}

func (c *RabbitConsumer) Ack(ctx context.Context, delivery Delivery) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if c == nil {
		return connectors.ErrDestroyed
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return connectors.ErrDestroyed
	}
	if !c.config.ManualAck || delivery.DeliveryTag == 0 {
		return nil
	}
	if err := c.channel.Ack(delivery.DeliveryTag, false); err != nil {
		return fmt.Errorf("amqp: ack delivery %d: %w", delivery.DeliveryTag, err)
	}
	return nil
}

func (c *RabbitConsumer) Reject(ctx context.Context, delivery Delivery, requeue bool) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if c == nil {
		return connectors.ErrDestroyed
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return connectors.ErrDestroyed
	}
	if !c.config.ManualAck || delivery.DeliveryTag == 0 {
		return nil
	}
	if err := c.channel.Reject(delivery.DeliveryTag, requeue); err != nil {
		return fmt.Errorf("amqp: reject delivery %d: %w", delivery.DeliveryTag, err)
	}
	return nil
}

func (c *RabbitConsumer) Close() error {
	if c == nil {
		return nil
	}
	var err error
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		channel := c.channel
		connection := c.connection
		c.mu.Unlock()
		if channel != nil {
			err = channel.Close()
		}
		if connection != nil {
			if closeErr := connection.Close(); err == nil {
				err = closeErr
			}
		}
	})
	return err
}

func fromRabbitDelivery(message amqp091.Delivery) Delivery {
	return Delivery{
		Body:            append([]byte(nil), message.Body...),
		Headers:         cloneTable(map[string]any(message.Headers)),
		ContentType:     message.ContentType,
		ContentEncoding: message.ContentEncoding,
		DeliveryTag:     message.DeliveryTag,
		Exchange:        message.Exchange,
		RoutingKey:      message.RoutingKey,
		Redelivered:     message.Redelivered,
		MessageID:       message.MessageId,
		CorrelationID:   message.CorrelationId,
		Timestamp:       message.Timestamp,
	}
}

// PublisherConfig describes the RabbitMQ destination and declaration.
type PublisherConfig struct {
	Connection ConnectionConfig

	QueueName         string
	Exchange          string
	RoutingKey        string
	DeclareDurable    bool
	DeclareExclusive  bool
	DeclareAutoDelete bool
	DeclareArguments  map[string]any
}

// RabbitPublisher adapts one RabbitMQ connection/channel to Publisher.
type RabbitPublisher struct {
	connection *amqp091.Connection
	channel    *amqp091.Channel
	config     PublisherConfig
	queue      string

	mu        sync.Mutex
	closed    bool
	closeOnce sync.Once
}

// NewRabbitPublisher opens a connection and declares the destination. When
// Exchange is empty, Publication.RoutingKey or QueueName is sent through the
// default exchange. When Exchange is set, the configured routing key is used
// unless a Publication overrides it.
func NewRabbitPublisher(config PublisherConfig) (Publisher, error) {
	connection, err := dial(config.Connection)
	if err != nil {
		return nil, err
	}
	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("amqp: open publisher channel: %w", err)
	}
	cleanup := func(cause error) (Publisher, error) {
		_ = channel.Close()
		_ = connection.Close()
		return nil, cause
	}
	if config.Exchange != "" {
		if err := channel.ExchangeDeclarePassive(config.Exchange, "direct", true, false, false, false, nil); err != nil {
			return cleanup(fmt.Errorf("amqp: inspect exchange %q: %w", config.Exchange, err))
		}
	}
	queue := config.QueueName
	if queue != "" {
		declared, err := channel.QueueDeclare(queue, config.DeclareDurable, config.DeclareAutoDelete, config.DeclareExclusive, false, toTable(config.DeclareArguments))
		if err != nil {
			return cleanup(fmt.Errorf("amqp: declare queue %q: %w", queue, err))
		}
		queue = declared.Name
	}
	if config.Exchange != "" && config.RoutingKey != "" && queue != "" {
		if err := channel.QueueBind(queue, config.Exchange, config.RoutingKey, false, nil); err != nil {
			return cleanup(fmt.Errorf("amqp: bind queue %q: %w", queue, err))
		}
	}
	return &RabbitPublisher{connection: connection, channel: channel, config: config, queue: queue}, nil
}

func (p *RabbitPublisher) Publish(ctx context.Context, publication Publication) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if p == nil {
		return connectors.ErrDestroyed
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return connectors.ErrDestroyed
	}
	exchange := publication.Exchange
	if exchange == "" {
		exchange = p.config.Exchange
	}
	routingKey := publication.RoutingKey
	if routingKey == "" {
		routingKey = p.config.RoutingKey
		if routingKey == "" {
			routingKey = p.queue
		}
	}
	if exchange == "" && routingKey == "" {
		return errors.New("amqp: publication has no exchange, routing key or queue")
	}
	message := amqp091.Publishing{
		Headers:         toTable(publication.Headers),
		ContentType:     publication.ContentType,
		ContentEncoding: publication.ContentEncoding,
		DeliveryMode:    publication.DeliveryMode,
		Priority:        publication.Priority,
		Body:            append([]byte(nil), publication.Body...),
		MessageId:       publication.MessageID,
		CorrelationId:   publication.CorrelationID,
		Timestamp:       publication.Timestamp,
	}
	if err := p.channel.PublishWithContext(ctx, exchange, routingKey, false, false, message); err != nil {
		return fmt.Errorf("amqp: publish exchange %q key %q: %w", exchange, routingKey, err)
	}
	return nil
}

func (p *RabbitPublisher) Close() error {
	if p == nil {
		return nil
	}
	var err error
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		channel := p.channel
		connection := p.connection
		p.mu.Unlock()
		if channel != nil {
			err = channel.Close()
		}
		if connection != nil {
			if closeErr := connection.Close(); err == nil {
				err = closeErr
			}
		}
	})
	return err
}

// Decoder turns a delivery body into an Engine value or a value passed to
// SourceSpec.Emit.
type Decoder func(Delivery) (any, error)

// DecodeJSON is the Go counterpart of Java's AMQPToObjectCollectorJson.
func DecodeJSON(delivery Delivery) (any, error) {
	var value any
	if err := json.Unmarshal(delivery.Body, &value); err != nil {
		return nil, fmt.Errorf("amqp: decode JSON: %w", err)
	}
	return value, nil
}

// DecodeGOB is an explicit Go serialization option. It is deliberately not a
// Java Serializable wire decoder; Java serialization must use an application
// codec or a process-outside bridge.
func DecodeGOB(delivery Delivery) (any, error) {
	var value any
	if err := gob.NewDecoder(bytes.NewReader(delivery.Body)).Decode(&value); err != nil {
		return nil, fmt.Errorf("amqp: decode gob: %w", err)
	}
	return value, nil
}

// AckMode defines when Source acknowledges a delivery.
type AckMode uint8

const (
	// AckAuto matches Java EsperIO's consumeAutoAck=true default.
	AckAuto AckMode = iota
	// AckAfterProcess provides at-least-once delivery when the broker consumer
	// is configured with ManualAck=true.
	AckAfterProcess
	// AckImmediately acknowledges before decoding/processing.
	AckImmediately
)

// SourceSpec describes an AMQP input adapter. Exactly one of Consumer, Open
// or Config and exactly one of Emit or Engine must be supplied.
type SourceSpec struct {
	Consumer Consumer
	Open     func() (Consumer, error)
	Config   *ConsumerConfig

	EventType string
	Decode    Decoder
	Emit      func(context.Context, Delivery, any) error
	Engine    *esper.Engine

	AckMode         AckMode
	RequeueOnError  bool
	RetryAttempts   int
	RetryInterval   time.Duration
	ContinueOnError bool
	ErrorHandler    func(error, Delivery)
}

// Source is a background AMQP consumer with explicit lifecycle and ack
// semantics. Stop closes the current consumer and allows factory/config based
// adapters to be started again.
type Source struct {
	manager *connectors.StateManager
	spec    SourceSpec

	mu       sync.Mutex
	consumer Consumer
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewSource(spec SourceSpec) (*Source, error) {
	providers := 0
	if spec.Consumer != nil {
		providers++
	}
	if spec.Open != nil {
		providers++
	}
	if spec.Config != nil {
		providers++
	}
	if providers != 1 {
		return nil, errors.New("amqp: exactly one of Consumer, Open or Config is required")
	}
	if spec.Emit == nil && spec.Engine == nil {
		return nil, errors.New("amqp: exactly one of Emit or Engine is required")
	}
	if spec.Emit != nil && spec.Engine != nil {
		return nil, errors.New("amqp: Emit and Engine are mutually exclusive")
	}
	if spec.Engine != nil && spec.EventType == "" {
		return nil, errors.New("amqp: EventType is required with Engine")
	}
	if spec.AckMode > AckImmediately {
		return nil, errors.New("amqp: invalid AckMode")
	}
	if spec.RetryAttempts < 0 {
		return nil, errors.New("amqp: RetryAttempts must not be negative")
	}
	if spec.Config != nil {
		config := *spec.Config
		config.DeclareArguments = cloneTable(config.DeclareArguments)
		if spec.AckMode != AckAuto {
			config.ManualAck = true
		}
		spec.Open = func() (Consumer, error) { return NewRabbitConsumer(config) }
		spec.Config = nil
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
		return fmt.Errorf("amqp: open consumer: %w", err)
	}
	if consumer == nil {
		_ = s.manager.Stop()
		return errors.New("amqp: consumer is nil")
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
	deliveries, err := consumer.Consume(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) && s.State() != connectors.Destroyed {
			s.report(err, Delivery{})
		}
		return
	}
	for {
		if err := s.waitStarted(ctx); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-deliveries:
			if !ok {
				if s.State() == connectors.Started {
					s.report(errors.New("amqp: consumer delivery channel closed"), Delivery{})
				}
				return
			}
			if !s.handleDelivery(ctx, consumer, delivery) {
				return
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

func (s *Source) handleDelivery(ctx context.Context, consumer Consumer, delivery Delivery) bool {
	if s.spec.AckMode == AckImmediately {
		if err := consumer.Ack(ctx, delivery); err != nil {
			s.report(err, delivery)
			if !s.spec.ContinueOnError {
				return false
			}
		}
	}
	if err := s.processWithRetry(ctx, delivery); err != nil {
		s.report(err, delivery)
		if s.spec.AckMode != AckAuto && s.spec.AckMode != AckImmediately {
			if rejectErr := consumer.Reject(ctx, delivery, s.spec.RequeueOnError); rejectErr != nil {
				s.report(rejectErr, delivery)
			}
			return s.spec.ContinueOnError
		}
		return true
	}
	if s.spec.AckMode == AckAfterProcess {
		if err := consumer.Ack(ctx, delivery); err != nil {
			s.report(err, delivery)
			return s.spec.ContinueOnError
		}
	}
	return true
}

func (s *Source) processWithRetry(ctx context.Context, delivery Delivery) error {
	attempts := retryAttempts(s.spec.RetryAttempts)
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := waitRetry(ctx, s.spec.RetryInterval); err != nil {
				return err
			}
		}
		lastErr = s.process(ctx, delivery)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (s *Source) process(ctx context.Context, delivery Delivery) error {
	value := any(delivery.Body)
	if s.spec.Decode != nil {
		decoded, err := s.spec.Decode(delivery)
		if err != nil {
			return err
		}
		value = decoded
	}
	if s.spec.Emit != nil {
		return s.spec.Emit(ctx, delivery, value)
	}
	if record, ok := value.(map[string]any); ok {
		return s.spec.Engine.SendRecord(ctx, s.spec.EventType, record)
	}
	if event, ok := value.(esper.Event); ok {
		return s.spec.Engine.Send(ctx, s.spec.EventType, event.Underlying())
	}
	return s.spec.Engine.Send(ctx, s.spec.EventType, value)
}

func (s *Source) report(err error, delivery Delivery) {
	if err != nil && s.spec.ErrorHandler != nil {
		s.spec.ErrorHandler(err, delivery)
	}
}

// SinkSpec configures an AMQP output adapter. Exactly one of Publisher, Open
// or Config is required. Config supplies the default exchange/routing key or
// queue target; explicit SinkSpec fields override those defaults.
type SinkSpec struct {
	Publisher Publisher
	Open      func() (Publisher, error)
	Config    *PublisherConfig

	Exchange        string
	RoutingKey      string
	QueueName       string
	Encode          func(esper.Result) ([]byte, error)
	Headers         map[string]any
	HeadersFor      func(esper.Result) map[string]any
	ContentType     string
	ContentEncoding string
	DeliveryMode    uint8
	IncludeOld      bool

	RetryAttempts int
	RetryInterval time.Duration
}

// Sink publishes new (and optionally old) statement results to AMQP.
type Sink struct {
	manager *connectors.StateManager
	spec    SinkSpec

	mu        sync.Mutex
	publisher Publisher
}

func NewSink(spec SinkSpec) (*Sink, error) {
	providers := 0
	if spec.Publisher != nil {
		providers++
	}
	if spec.Open != nil {
		providers++
	}
	if spec.Config != nil {
		providers++
	}
	if providers != 1 {
		return nil, errors.New("amqp: exactly one of Publisher, Open or Config is required")
	}
	configHasDestination := spec.Config != nil && (spec.Config.Exchange != "" || spec.Config.RoutingKey != "" || spec.Config.QueueName != "")
	if spec.Exchange == "" && spec.RoutingKey == "" && spec.QueueName == "" && !configHasDestination {
		return nil, errors.New("amqp: destination queue or exchange/routing key is required")
	}
	if spec.RetryAttempts < 0 {
		return nil, errors.New("amqp: RetryAttempts must not be negative")
	}
	spec.Headers = cloneTable(spec.Headers)
	if spec.Config != nil {
		config := *spec.Config
		config.DeclareArguments = cloneTable(config.DeclareArguments)
		if spec.Exchange != "" {
			config.Exchange = spec.Exchange
		}
		if spec.RoutingKey != "" {
			config.RoutingKey = spec.RoutingKey
		}
		if spec.QueueName != "" {
			config.QueueName = spec.QueueName
		}
		spec.Open = func() (Publisher, error) { return NewRabbitPublisher(config) }
		spec.Config = nil
	}
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
		return fmt.Errorf("amqp: open publisher: %w", err)
	}
	if publisher == nil {
		_ = s.manager.Stop()
		return errors.New("amqp: publisher is nil")
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
		return errors.New("amqp: sink publisher is not open")
	}
	results := append([]esper.Result(nil), batch.New...)
	if s.spec.IncludeOld {
		results = append(results, batch.Old...)
	}
	for _, result := range results {
		publication, err := s.encodeResult(result)
		if err != nil {
			return err
		}
		if err := publishWithRetry(ctx, publisher, publication, s.spec.RetryAttempts, s.spec.RetryInterval); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sink) encodeResult(result esper.Result) (Publication, error) {
	encoder := s.spec.Encode
	if encoder == nil {
		encoder = EncodeResultJSON
	}
	body, err := encoder(result)
	if err != nil {
		return Publication{}, err
	}
	headers := cloneTable(s.spec.Headers)
	if s.spec.HeadersFor != nil {
		headers = cloneTable(s.spec.HeadersFor(result))
	}
	return Publication{
		Body:            body,
		Headers:         headers,
		ContentType:     s.spec.ContentType,
		ContentEncoding: s.spec.ContentEncoding,
		DeliveryMode:    s.spec.DeliveryMode,
		Exchange:        s.spec.Exchange,
		RoutingKey:      s.spec.RoutingKey,
	}, nil
}

func publishWithRetry(ctx context.Context, publisher Publisher, publication Publication, attempts int, interval time.Duration) error {
	tries := retryAttempts(attempts)
	var lastErr error
	for attempt := 0; attempt < tries; attempt++ {
		if attempt > 0 {
			if err := waitRetry(ctx, interval); err != nil {
				return err
			}
		}
		lastErr = publisher.Publish(ctx, publication.clone())
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

// EncodeResultJSON emits event/row fields in schema order, matching the
// deterministic JSON produced by the Java JsonEventObject collector.
func EncodeResultJSON(result esper.Result) ([]byte, error) {
	if row, ok := result.Row(); ok {
		return encodeOrderedJSON(row.Schema().Fields(), row.Get)
	}
	if event, ok := result.Event(); ok {
		return encodeOrderedJSON(event.Schema().Fields(), event.Get)
	}
	return nil, errors.New("amqp: result has no event or row")
}

// EncodeResultGOB is a Go-only serialization helper. It is not compatible
// with Java ObjectOutputStream/Serializable bytes.
func EncodeResultGOB(result esper.Result) ([]byte, error) {
	value := result.Underlying()
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(value); err != nil {
		return nil, fmt.Errorf("amqp: encode gob: %w", err)
	}
	return buffer.Bytes(), nil
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
			return nil, fmt.Errorf("amqp: encode field %q: %w", field.Name, err)
		}
		result.Write(name)
		result.WriteByte(':')
		result.Write(value)
	}
	result.WriteByte('}')
	return result.Bytes(), nil
}

func fromTable(values amqp091.Table) map[string]any {
	if values == nil {
		return nil
	}
	return cloneTable(map[string]any(values))
}

func toTable(values map[string]any) amqp091.Table {
	if values == nil {
		return nil
	}
	return amqp091.Table(cloneTable(values))
}

func cloneTable(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case []byte:
			result[key] = append([]byte(nil), typed...)
		case map[string]any:
			result[key] = cloneTable(typed)
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

var _ Consumer = (*RabbitConsumer)(nil)
var _ Publisher = (*RabbitPublisher)(nil)
var _ esper.Sink = (*Sink)(nil)
