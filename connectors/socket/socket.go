// Package socket implements the EsperIO Socket input adapter as a Go TCP
// source. It keeps the Java wire modes that are useful outside the JVM and
// replaces Java serialization with an explicit Go object decoder hook.
package socket

import (
	"bufio"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
)

// DataType selects the line/object protocol used by a socket service.
type DataType uint8

const (
	DataTypeObject DataType = iota
	DataTypeCSV
	DataTypePropertyOrderedCSV
	DataTypeJSON
)

func (d DataType) String() string {
	switch d {
	case DataTypeObject:
		return "OBJECT"
	case DataTypeCSV:
		return "CSV"
	case DataTypePropertyOrderedCSV:
		return "PROPERTY_ORDERED_CSV"
	case DataTypeJSON:
		return "JSON"
	default:
		return fmt.Sprintf("DATA_TYPE(%d)", d)
	}
}

// ValueDecoder converts one textual socket field. It is called after Java
// escape decoding when Unescape is enabled.
type ValueDecoder func(string) (any, error)

// ObjectDecoder reads one object from a persistent connection. The decoder
// owns framing: it may read a line, a length prefix, gob frame, or another
// application-defined protocol. Returning io.EOF closes the connection.
// Java ObjectInputStream is deliberately not decoded by this package.
type ObjectDecoder func(*bufio.Reader) (any, error)

// Message is the normalized value delivered by a socket source callback.
type Message struct {
	Stream     string
	Value      any
	Raw        string
	DataType   DataType
	RemoteAddr string
}

// SourceSpec configures a TCP input service. Addr takes precedence over Host
// and Port. A zero Port is useful for tests and lets the OS choose a free port.
// Exactly one of Emit or Engine must be supplied.
type SourceSpec struct {
	Addr string
	Host string
	Port int

	DataType      DataType
	Stream        string
	EventType     string
	PropertyOrder []string
	Unescape      bool

	PropertyTypes map[string]reflect.Type
	ValueDecoders map[string]ValueDecoder
	ObjectDecoder ObjectDecoder
	MaxLineBytes  int

	Emit         func(context.Context, Message) error
	Engine       *esper.Engine
	ErrorHandler func(error)
}

// Source is a concurrent, lifecycle-managed TCP source. Every accepted
// connection gets one reader goroutine; Engine itself serializes runtime
// mutation while callback users choose their own concurrency policy.
type Source struct {
	manager *connectors.StateManager
	spec    SourceSpec

	mu       sync.Mutex
	listener net.Listener
	conns    map[net.Conn]struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewSource validates and copies a socket source specification without
// opening a listener.
func NewSource(spec SourceSpec) (*Source, error) {
	if spec.Emit == nil && spec.Engine == nil {
		return nil, errors.New("socket: exactly one of Emit or Engine is required")
	}
	if spec.Emit != nil && spec.Engine != nil {
		return nil, errors.New("socket: Emit and Engine are mutually exclusive")
	}
	if spec.Addr == "" {
		host := strings.TrimSpace(spec.Host)
		if host == "" {
			host = "127.0.0.1"
		}
		if spec.Port < 0 || spec.Port > 65535 {
			return nil, fmt.Errorf("socket: port %d is outside 0..65535", spec.Port)
		}
		spec.Addr = net.JoinHostPort(host, strconv.Itoa(spec.Port))
	}
	if strings.TrimSpace(spec.Addr) == "" {
		return nil, errors.New("socket: Addr must not be empty")
	}
	if spec.DataType == DataTypePropertyOrderedCSV && len(spec.PropertyOrder) == 0 {
		return nil, errors.New("socket: PropertyOrder is required for PROPERTY_ORDERED_CSV")
	}
	for index, property := range spec.PropertyOrder {
		if strings.TrimSpace(property) == "" {
			return nil, fmt.Errorf("socket: PropertyOrder[%d] is empty", index)
		}
	}
	if spec.MaxLineBytes == 0 {
		spec.MaxLineBytes = 1 << 20
	}
	if spec.MaxLineBytes < 256 {
		return nil, errors.New("socket: MaxLineBytes must be at least 256")
	}
	spec.PropertyOrder = append([]string(nil), spec.PropertyOrder...)
	spec.PropertyTypes = copyTypes(spec.PropertyTypes)
	spec.ValueDecoders = copyDecoders(spec.ValueDecoders)
	return &Source{
		manager: connectors.NewStateManager(),
		spec:    spec,
		conns:   make(map[net.Conn]struct{}),
	}, nil
}

func copyTypes(values map[string]reflect.Type) map[string]reflect.Type {
	if values == nil {
		return nil
	}
	result := make(map[string]reflect.Type, len(values))
	for name, typ := range values {
		result[name] = typ
	}
	return result
}

func copyDecoders(values map[string]ValueDecoder) map[string]ValueDecoder {
	if values == nil {
		return nil
	}
	result := make(map[string]ValueDecoder, len(values))
	for name, decoder := range values {
		result[name] = decoder
	}
	return result
}

// State returns the current adapter state.
func (s *Source) State() connectors.State {
	if s == nil {
		return connectors.Destroyed
	}
	return s.manager.State()
}

// Addr returns the bound TCP address. Before Start it returns the configured
// address; after Start with port 0 it returns the OS-selected port.
func (s *Source) Addr() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.spec.Addr
}

// ActiveConnections reports the number of accepted connections currently
// owned by the source.
func (s *Source) ActiveConnections() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns)
}

// Start binds the listener and begins accepting connections.
func (s *Source) Start() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Start(); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", s.spec.Addr)
	if err != nil {
		_ = s.manager.Stop()
		return fmt.Errorf("socket: listen on %s: %w", s.spec.Addr, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.listener = listener
	s.ctx = ctx
	s.cancel = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go s.acceptLoop(listener)
	return nil
}

// Pause prevents decoded messages from being delivered while keeping the
// listener and existing connections alive. Readers wait for Resume.
func (s *Source) Pause() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Pause()
}

// Resume resumes delivery on paused connections.
func (s *Source) Resume() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Resume()
}

// Stop closes the listener and active connections, returning the source to
// OPENED so it can be started again.
func (s *Source) Stop() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Stop(); err != nil {
		return err
	}
	s.closeNetwork()
	return nil
}

// Destroy permanently closes the source and all active connections.
func (s *Source) Destroy() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Destroy(); err != nil {
		return err
	}
	s.closeNetwork()
	return nil
}

func (s *Source) closeNetwork() {
	s.mu.Lock()
	listener := s.listener
	s.listener = nil
	cancel := s.cancel
	s.cancel = nil
	connections := make([]net.Conn, 0, len(s.conns))
	for conn := range s.conns {
		connections = append(connections, conn)
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if listener != nil {
		_ = listener.Close()
	}
	for _, conn := range connections {
		_ = conn.Close()
	}
	s.wg.Wait()
}

func (s *Source) acceptLoop(listener net.Listener) {
	defer s.wg.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if s.State() != connectors.Started && s.State() != connectors.Paused {
				return
			}
			s.report(err)
			return
		}
		s.mu.Lock()
		s.conns[conn] = struct{}{}
		ctx := s.ctx
		s.mu.Unlock()
		if ctx == nil {
			ctx = context.Background()
		}
		s.wg.Add(1)
		go s.handleConnection(ctx, conn)
	}
}

func (s *Source) handleConnection(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
		_ = conn.Close()
	}()

	reader := bufio.NewReaderSize(conn, s.spec.MaxLineBytes)
	if s.spec.DataType == DataTypeObject {
		s.handleObjects(ctx, conn, reader)
		return
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), s.spec.MaxLineBytes)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		message, err := s.decodeLine(line, conn.RemoteAddr().String())
		if err != nil {
			s.report(err)
			continue
		}
		if err := s.emitWhenStarted(ctx, message); err != nil {
			if !errors.Is(err, connectors.ErrStopped) && !errors.Is(err, connectors.ErrDestroyed) && !errors.Is(err, context.Canceled) {
				s.report(err)
			}
			return
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, context.Canceled) {
		s.report(fmt.Errorf("socket: read from %s: %w", conn.RemoteAddr(), err))
	}
}

func (s *Source) handleObjects(ctx context.Context, conn net.Conn, reader *bufio.Reader) {
	var decoder *gob.Decoder
	if s.spec.ObjectDecoder == nil {
		decoder = gob.NewDecoder(reader)
	}
	for {
		var value any
		var err error
		if decoder != nil {
			err = decoder.Decode(&value)
		} else {
			value, err = s.spec.ObjectDecoder(reader)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) && !errors.Is(err, context.Canceled) {
				s.report(fmt.Errorf("socket: decode object from %s: %w", conn.RemoteAddr(), err))
			}
			return
		}
		message, err := s.objectMessage(value, conn.RemoteAddr().String())
		if err != nil {
			s.report(err)
			continue
		}
		if err := s.emitWhenStarted(ctx, message); err != nil {
			if !errors.Is(err, connectors.ErrStopped) && !errors.Is(err, connectors.ErrDestroyed) && !errors.Is(err, context.Canceled) {
				s.report(err)
			}
			return
		}
	}
}

func (s *Source) emitWhenStarted(ctx context.Context, message Message) error {
	for {
		switch s.State() {
		case connectors.Started:
			return s.emit(ctx, message)
		case connectors.Paused:
			changed := s.manager.Changes()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-changed:
				continue
			}
		case connectors.Destroyed:
			return connectors.ErrDestroyed
		default:
			return connectors.ErrStopped
		}
	}
}

func (s *Source) emit(ctx context.Context, message Message) error {
	if s.spec.Emit != nil {
		return s.spec.Emit(ctx, message)
	}
	stream := message.Stream
	if stream == "" {
		stream = s.spec.EventType
	}
	if stream == "" {
		stream = s.spec.Stream
	}
	if stream == "" {
		if _, ok := message.Value.(esper.Event); ok {
			return s.spec.Engine.SendEvent(ctx, message.Value)
		}
		return errors.New("socket: message has no stream or fixed EventType")
	}
	if event, ok := message.Value.(esper.Event); ok {
		return s.spec.Engine.Send(ctx, stream, event.Underlying())
	}
	if record, ok := message.Value.(map[string]any); ok {
		return s.spec.Engine.SendRecord(ctx, stream, record)
	}
	return s.spec.Engine.Send(ctx, stream, message.Value)
}

func (s *Source) decodeLine(line, remoteAddr string) (Message, error) {
	switch s.spec.DataType {
	case DataTypeCSV:
		return s.decodeCSV(line, remoteAddr, false)
	case DataTypePropertyOrderedCSV:
		return s.decodeCSV(line, remoteAddr, true)
	case DataTypeJSON:
		return s.decodeJSON(line, remoteAddr)
	default:
		return Message{}, fmt.Errorf("socket: unsupported line data type %s", s.spec.DataType)
	}
}

func (s *Source) decodeCSV(line, remoteAddr string, ordered bool) (Message, error) {
	if ordered {
		record := make(map[string]any, len(s.spec.PropertyOrder))
		values := strings.Split(line, ",")
		for index, property := range s.spec.PropertyOrder {
			if index >= len(values) {
				break
			}
			name := strings.TrimSpace(property)
			value := s.unescape(values[index])
			converted, err := s.convertField(s.fixedStream(), name, value)
			if err != nil {
				return Message{}, err
			}
			record[name] = converted
		}
		return Message{Stream: s.fixedStream(), Value: record, Raw: line, DataType: s.spec.DataType, RemoteAddr: remoteAddr}, nil
	}

	values := make(map[string]string)
	stream := ""
	for _, item := range strings.Split(line, ",") {
		index := strings.IndexByte(item, '=')
		if index < 0 {
			continue
		}
		name := strings.TrimSpace(item[:index])
		value := s.unescape(item[index+1:])
		if name == "stream" {
			stream = value
			continue
		}
		if name != "" {
			values[name] = value
		}
	}
	if stream == "" {
		stream = s.fixedStream()
	}
	if stream == "" {
		return Message{}, errors.New("socket: CSV message has no stream")
	}
	record := make(map[string]any, len(values))
	for name, value := range values {
		converted, err := s.convertField(stream, name, value)
		if err != nil {
			return Message{}, err
		}
		record[name] = converted
	}
	return Message{Stream: stream, Value: record, Raw: line, DataType: s.spec.DataType, RemoteAddr: remoteAddr}, nil
}

func (s *Source) decodeJSON(line, remoteAddr string) (Message, error) {
	stream := ""
	raw := line
	if strings.HasPrefix(line, "stream=") {
		const marker = ",json="
		index := strings.Index(line, marker)
		if index < 0 {
			return Message{}, fmt.Errorf("socket: JSON message is missing %q", marker)
		}
		stream = s.unescape(line[len("stream="):index])
		raw = line[index+len(marker):]
	} else {
		stream = s.fixedStream()
	}
	if stream == "" {
		return Message{}, errors.New("socket: JSON message has no stream")
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return Message{}, fmt.Errorf("socket: decode JSON for %q: %w", stream, err)
	}
	for name, value := range record {
		if number, ok := value.(float64); ok {
			converted, err := s.convertField(stream, name, strconv.FormatFloat(number, 'f', -1, 64))
			if err == nil {
				record[name] = converted
			}
		}
	}
	return Message{Stream: stream, Value: record, Raw: raw, DataType: s.spec.DataType, RemoteAddr: remoteAddr}, nil
}

func (s *Source) objectMessage(value any, remoteAddr string) (Message, error) {
	stream := s.fixedStream()
	if record, ok := value.(map[string]any); ok {
		copyRecord := make(map[string]any, len(record))
		for name, fieldValue := range record {
			if name == "stream" {
				if text, ok := fieldValue.(string); ok {
					stream = text
				}
				continue
			}
			copyRecord[name] = fieldValue
		}
		if stream == "" && s.spec.Engine == nil {
			return Message{}, errors.New("socket: object map has no stream")
		}
		return Message{Stream: stream, Value: copyRecord, DataType: DataTypeObject, RemoteAddr: remoteAddr}, nil
	}
	return Message{Stream: stream, Value: value, DataType: DataTypeObject, RemoteAddr: remoteAddr}, nil
}

func (s *Source) fixedStream() string {
	if s.spec.EventType != "" {
		return s.spec.EventType
	}
	return s.spec.Stream
}

func (s *Source) convertField(stream, name, value string) (any, error) {
	if decoder := s.spec.ValueDecoders[name]; decoder != nil {
		converted, err := decoder(value)
		if err != nil {
			return nil, fmt.Errorf("socket: field %q: %w", name, err)
		}
		return converted, nil
	}
	typ := s.spec.PropertyTypes[name]
	if typ == nil && s.spec.Engine != nil {
		if schema, ok := s.spec.Engine.EventSchema(stream); ok {
			if field, exists := schema.Field(name); exists {
				typ = field.Type
			}
		}
	}
	return parseText(value, typ, name)
}

func parseText(value string, typ reflect.Type, name string) (any, error) {
	if typ == nil || typ.Kind() == reflect.Interface {
		return value, nil
	}
	if typ.Kind() == reflect.Pointer {
		converted, err := parseText(value, typ.Elem(), name)
		if err != nil {
			return nil, err
		}
		result := reflect.New(typ.Elem())
		assigned, err := assignParsed(result.Elem(), converted)
		if err != nil {
			return nil, fmt.Errorf("socket: field %q: %w", name, err)
		}
		result.Elem().Set(assigned)
		return result.Interface(), nil
	}
	if typ == reflect.TypeOf(time.Time{}) {
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return nil, fmt.Errorf("socket: field %q: parse time: %w", name, err)
		}
		return parsed, nil
	}
	switch typ.Kind() {
	case reflect.String:
		return value, nil
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("socket: field %q: parse bool: %w", name, err)
		}
		return parsed, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(value, 10, typ.Bits())
		if err != nil {
			return nil, fmt.Errorf("socket: field %q: parse integer: %w", name, err)
		}
		result := reflect.New(typ).Elem()
		result.SetInt(parsed)
		return result.Interface(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(value, 10, typ.Bits())
		if err != nil {
			return nil, fmt.Errorf("socket: field %q: parse unsigned integer: %w", name, err)
		}
		result := reflect.New(typ).Elem()
		result.SetUint(parsed)
		return result.Interface(), nil
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(value, typ.Bits())
		if err != nil {
			return nil, fmt.Errorf("socket: field %q: parse float: %w", name, err)
		}
		result := reflect.New(typ).Elem()
		result.SetFloat(parsed)
		return result.Interface(), nil
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 {
			return []byte(value), nil
		}
	case reflect.Map, reflect.Struct, reflect.Array:
		result := reflect.New(typ)
		if err := json.Unmarshal([]byte(value), result.Interface()); err != nil {
			return nil, fmt.Errorf("socket: field %q: parse JSON value: %w", name, err)
		}
		return result.Elem().Interface(), nil
	}
	return value, nil
}

func assignParsed(target reflect.Value, value any) (reflect.Value, error) {
	if value == nil {
		return reflect.Zero(target.Type()), nil
	}
	parsed := reflect.ValueOf(value)
	if parsed.Type().AssignableTo(target.Type()) {
		return parsed, nil
	}
	if parsed.Type().ConvertibleTo(target.Type()) {
		return parsed.Convert(target.Type()), nil
	}
	return reflect.Value{}, fmt.Errorf("expects %s, got %s", target.Type(), parsed.Type())
}

func (s *Source) unescape(value string) string {
	if !s.spec.Unescape {
		return value
	}
	return unescapeJava(value)
}

func unescapeJava(value string) string {
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' || index+1 >= len(value) {
			builder.WriteByte(value[index])
			continue
		}
		index++
		switch value[index] {
		case '\\':
			builder.WriteByte('\\')
		case 'b':
			builder.WriteByte('\b')
		case 'f':
			builder.WriteByte('\f')
		case 'n':
			builder.WriteByte('\n')
		case 'r':
			builder.WriteByte('\r')
		case 't':
			builder.WriteByte('\t')
		case '"':
			builder.WriteByte('"')
		case '\'':
			builder.WriteByte('\'')
		case 'u':
			if index+4 >= len(value) {
				builder.WriteString("\\u")
				continue
			}
			code, err := strconv.ParseUint(value[index+1:index+5], 16, 16)
			if err != nil {
				builder.WriteString("\\u")
				continue
			}
			builder.WriteRune(rune(code))
			index += 4
		default:
			if value[index] >= '0' && value[index] <= '7' {
				end := index + 1
				for end < len(value) && end < index+3 && value[end] >= '0' && value[end] <= '7' {
					end++
				}
				code, err := strconv.ParseUint(value[index:end], 8, 8)
				if err == nil {
					builder.WriteRune(rune(code))
					index = end - 1
					continue
				}
			}
			builder.WriteByte('\\')
			builder.WriteByte(value[index])
		}
	}
	return builder.String()
}

func (s *Source) report(err error) {
	if err == nil || s.spec.ErrorHandler == nil {
		return
	}
	s.spec.ErrorHandler(err)
}

// ValidateDataType is a small helper for configuration loaders.
func ValidateDataType(value string) (DataType, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "OBJECT", "":
		return DataTypeObject, nil
	case "CSV":
		return DataTypeCSV, nil
	case "PROPERTY_ORDERED_CSV", "PROPERTY-ORDERED-CSV":
		return DataTypePropertyOrderedCSV, nil
	case "JSON":
		return DataTypeJSON, nil
	default:
		return 0, fmt.Errorf("socket: unknown data type %q", value)
	}
}
