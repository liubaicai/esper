// Package http implements the transport-independent HTTP source/sink slice
// of EsperIO using Go's net/http package.
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	nethttp "net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
	csvconnector "github.com/liubaicai/esper/connectors/csv"
)

// ClientSpec configures an HTTP output request. With no URI placeholders, all
// record properties plus Stream are appended as URL query parameters, which
// matches EsperIO HTTP's request listener. URI placeholders use the
// `${property}` form and are replaced without adding duplicate query fields.
type ClientSpec struct {
	URI    string
	Stream string
	Method string

	Headers map[string]string
	Client  *nethttp.Client
	// Body encodes an event for methods that carry a body. A nil Body uses
	// JSON for POST/PUT/PATCH and no body for GET/HEAD.
	Body func(csvconnector.Record) ([]byte, string, error)

	Retry            int
	RetryInterval    time.Duration
	MaxResponseBytes int64
}

// Response is the bounded HTTP result returned by Do.
type Response struct {
	StatusCode int
	Header     nethttp.Header
	Body       []byte
}

// ClientSink sends one request per event. It owns no http.Client and closes
// no caller-owned transport.
type ClientSink struct {
	manager *connectors.StateManager
	spec    ClientSpec
	mu      sync.Mutex
}

func NewClientSink(spec ClientSpec) (*ClientSink, error) {
	if strings.TrimSpace(spec.URI) == "" {
		return nil, fmt.Errorf("http: URI is required")
	}
	if _, err := url.Parse(spec.URI); err != nil {
		return nil, fmt.Errorf("http: invalid URI: %w", err)
	}
	if spec.Method == "" {
		spec.Method = nethttp.MethodGet
	}
	spec.Method = strings.ToUpper(spec.Method)
	if spec.Retry < 0 || spec.RetryInterval < 0 {
		return nil, fmt.Errorf("http: retry settings cannot be negative")
	}
	if spec.MaxResponseBytes == 0 {
		spec.MaxResponseBytes = 4 << 20
	}
	if spec.MaxResponseBytes < 0 {
		return nil, fmt.Errorf("http: MaxResponseBytes cannot be negative")
	}
	if spec.Client == nil {
		spec.Client = &nethttp.Client{}
	}
	spec.Headers = copyHeaders(spec.Headers)
	return &ClientSink{manager: connectors.NewStateManager(), spec: spec}, nil
}

func (s *ClientSink) State() connectors.State {
	if s == nil {
		return connectors.Destroyed
	}
	return s.manager.State()
}

func (s *ClientSink) Start() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Start()
}

func (s *ClientSink) Stop() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Stop()
}

func (s *ClientSink) Pause() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Pause()
}

func (s *ClientSink) Resume() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Resume()
}

func (s *ClientSink) Destroy() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Destroy()
}

// Write sends a request and discards the response body after checking for a
// 2xx status.
func (s *ClientSink) Write(ctx context.Context, value any) error {
	_, err := s.Do(ctx, value)
	return err
}

// Do sends a request and returns a bounded response. Non-2xx statuses are
// errors, matching Apache BasicResponseHandler's default behavior.
func (s *ClientSink) Do(ctx context.Context, value any) (Response, error) {
	if s == nil {
		return Response{}, connectors.ErrDestroyed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	if err := s.manager.RequireStarted(); err != nil {
		return Response{}, err
	}
	record, err := csvconnector.EventToRecord(value)
	if err != nil {
		return Response{}, err
	}
	attempts := s.spec.Retry
	if attempts == 0 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		response, err := s.doOnce(ctx, record)
		if err == nil {
			return response, nil
		}
		lastErr = err
		if attempt+1 >= attempts || s.spec.RetryInterval == 0 {
			continue
		}
		if err := waitRetry(ctx, s.spec.RetryInterval); err != nil {
			return Response{}, err
		}
	}
	return Response{}, fmt.Errorf("http: request failed after %d attempt(s): %w", attempts, lastErr)
}

func (s *ClientSink) doOnce(ctx context.Context, record csvconnector.Record) (Response, error) {
	target, err := buildTarget(s.spec.URI, s.spec.Stream, record)
	if err != nil {
		return Response{}, err
	}
	var body io.Reader
	contentType := ""
	if s.spec.Body != nil {
		encoded, typ, err := s.spec.Body(record)
		if err != nil {
			return Response{}, err
		}
		body, contentType = strings.NewReader(string(encoded)), typ
	} else if s.spec.Method == nethttp.MethodPost || s.spec.Method == nethttp.MethodPut || s.spec.Method == nethttp.MethodPatch {
		encoded, err := json.Marshal(record)
		if err != nil {
			return Response{}, err
		}
		body, contentType = strings.NewReader(string(encoded)), "application/json"
	}
	request, err := nethttp.NewRequestWithContext(ctx, s.spec.Method, target, body)
	if err != nil {
		return Response{}, err
	}
	for name, value := range s.spec.Headers {
		request.Header.Set(name, value)
	}
	if contentType != "" && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", contentType)
	}
	result, err := s.spec.Client.Do(request)
	if err != nil {
		return Response{}, err
	}
	defer result.Body.Close()
	limit := s.spec.MaxResponseBytes
	if limit == 0 {
		limit = 4 << 20
	}
	data, readErr := io.ReadAll(io.LimitReader(result.Body, limit+1))
	if readErr != nil {
		return Response{}, readErr
	}
	if int64(len(data)) > limit {
		return Response{}, fmt.Errorf("http: response exceeds %d bytes", limit)
	}
	response := Response{StatusCode: result.StatusCode, Header: result.Header.Clone(), Body: data}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		return response, fmt.Errorf("http: unexpected status %s", result.Status)
	}
	return response, nil
}

func buildTarget(raw, stream string, record csvconnector.Record) (string, error) {
	if strings.Contains(raw, "${") {
		result := raw
		values := make(map[string]string, len(record)+1)
		values["stream"] = stream
		for name, value := range record {
			if value == nil {
				values[name] = "null"
			} else {
				values[name] = fmt.Sprint(value)
			}
		}
		for {
			start := strings.Index(result, "${")
			if start < 0 {
				break
			}
			end := strings.Index(result[start+2:], "}")
			if end < 0 {
				return "", fmt.Errorf("http: unterminated URI placeholder")
			}
			end += start + 2
			name := result[start+2 : end]
			value, exists := values[name]
			if !exists {
				value = "null"
			}
			result = result[:start] + url.PathEscape(value) + result[end+1:]
		}
		if _, err := url.ParseRequestURI(result); err != nil {
			return "", fmt.Errorf("http: rendered URI: %w", err)
		}
		return result, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	parameters := make([][2]string, 0, len(record)+1)
	parameters = append(parameters, [2]string{"stream", stream})
	keys := make([]string, 0, len(record))
	for name := range record {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		value := ""
		if record[name] != nil {
			value = fmt.Sprint(record[name])
		}
		parameters = append(parameters, [2]string{name, value})
	}
	query := parsed.RawQuery
	for _, parameter := range parameters {
		if query != "" {
			query += "&"
		}
		query += url.QueryEscape(parameter[0])
		if parameter[1] != "" {
			query += "=" + url.QueryEscape(parameter[1])
		}
	}
	parsed.RawQuery = query
	return parsed.String(), nil
}

func copyHeaders(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	result := make(map[string]string, len(values))
	for name, value := range values {
		result[name] = value
	}
	return result
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Request is the decoded inbound HTTP request delivered to a ServerSource.
type Request struct {
	Method string
	Path   string
	Query  url.Values
	Header nethttp.Header
	Body   []byte
}

// ServerSourceSpec configures an inbound HTTP source. Emit receives every
// accepted request. Alternatively, Engine/EventType can be supplied to route
// query values as a map-backed Esper event; a query parameter named stream can
// override EventType, matching Java EsperIO HTTP input.
type ServerSourceSpec struct {
	Addr string
	Path string

	AllowedMethods  []string
	MaxBodyBytes    int64
	Emit            func(context.Context, Request) error
	Engine          *esper.Engine
	EventType       string
	QueryConverters map[string]func(string) (any, error)
}

// ServerSource is a lifecycle-managed net/http server.
type ServerSource struct {
	manager *connectors.StateManager
	spec    ServerSourceSpec

	mu       sync.Mutex
	server   *nethttp.Server
	listener net.Listener
}

func NewServerSource(spec ServerSourceSpec) (*ServerSource, error) {
	if strings.TrimSpace(spec.Addr) == "" {
		spec.Addr = "127.0.0.1:0"
	}
	if strings.TrimSpace(spec.Path) == "" {
		spec.Path = "/"
	}
	if !strings.HasPrefix(spec.Path, "/") {
		return nil, fmt.Errorf("http: Path must start with '/'")
	}
	if spec.MaxBodyBytes == 0 {
		spec.MaxBodyBytes = 4 << 20
	}
	if spec.MaxBodyBytes < 0 {
		return nil, fmt.Errorf("http: MaxBodyBytes cannot be negative")
	}
	if spec.Emit == nil && spec.Engine == nil {
		return nil, fmt.Errorf("http: ServerSource requires Emit or Engine")
	}
	if spec.Engine != nil && strings.TrimSpace(spec.EventType) == "" {
		return nil, fmt.Errorf("http: EventType is required with Engine")
	}
	allowed := make(map[string]struct{})
	for _, method := range spec.AllowedMethods {
		method = strings.ToUpper(strings.TrimSpace(method))
		if method == "" {
			continue
		}
		allowed[method] = struct{}{}
	}
	if len(allowed) == 0 {
		allowed[nethttp.MethodGet] = struct{}{}
		allowed[nethttp.MethodHead] = struct{}{}
		allowed[nethttp.MethodPost] = struct{}{}
	}
	spec.AllowedMethods = make([]string, 0, len(allowed))
	for method := range allowed {
		spec.AllowedMethods = append(spec.AllowedMethods, method)
	}
	sort.Strings(spec.AllowedMethods)
	return &ServerSource{manager: connectors.NewStateManager(), spec: spec}, nil
}

func (s *ServerSource) State() connectors.State {
	if s == nil {
		return connectors.Destroyed
	}
	return s.manager.State()
}

func (s *ServerSource) Addr() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *ServerSource) Start() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Start(); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", s.spec.Addr)
	if err != nil {
		_ = s.manager.Stop()
		return fmt.Errorf("http: listen %q: %w", s.spec.Addr, err)
	}
	mux := nethttp.NewServeMux()
	mux.HandleFunc(s.spec.Path, s.handle)
	server := &nethttp.Server{Handler: mux}
	s.mu.Lock()
	s.listener, s.server = listener, server
	s.mu.Unlock()
	go func() {
		if err := server.Serve(listener); err != nil && err != nethttp.ErrServerClosed {
			// Serve errors are surfaced to request callers through the closed
			// listener; lifecycle remains explicit and deterministic.
		}
	}()
	return nil
}

func (s *ServerSource) Stop() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Stop(); err != nil {
		return err
	}
	return s.shutdown()
}

func (s *ServerSource) Pause() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Pause()
}

func (s *ServerSource) Resume() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Resume()
}

func (s *ServerSource) Destroy() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Destroy(); err != nil {
		return err
	}
	return s.shutdown()
}

func (s *ServerSource) handle(writer nethttp.ResponseWriter, request *nethttp.Request) {
	if s.manager.State() == connectors.Paused {
		nethttp.Error(writer, "http source paused", nethttp.StatusServiceUnavailable)
		return
	}
	if err := s.manager.RequireStarted(); err != nil {
		nethttp.Error(writer, err.Error(), nethttp.StatusServiceUnavailable)
		return
	}
	allowed := false
	for _, method := range s.spec.AllowedMethods {
		if request.Method == method {
			allowed = true
			break
		}
	}
	if !allowed {
		nethttp.Error(writer, "method not allowed", nethttp.StatusMethodNotAllowed)
		return
	}
	limit := s.spec.MaxBodyBytes
	body, err := io.ReadAll(io.LimitReader(request.Body, limit+1))
	if err != nil {
		nethttp.Error(writer, err.Error(), nethttp.StatusBadRequest)
		return
	}
	if int64(len(body)) > limit {
		nethttp.Error(writer, "request body too large", nethttp.StatusRequestEntityTooLarge)
		return
	}
	incoming := Request{Method: request.Method, Path: request.URL.Path, Query: request.URL.Query(), Header: request.Header.Clone(), Body: body}
	if s.spec.Emit != nil {
		if err := s.spec.Emit(request.Context(), incoming); err != nil {
			nethttp.Error(writer, err.Error(), nethttp.StatusInternalServerError)
			return
		}
	} else {
		eventType := s.spec.EventType
		if override := incoming.Query.Get("stream"); override != "" {
			eventType = override
		}
		values := make(map[string]any, len(incoming.Query))
		for name, items := range incoming.Query {
			if name == "stream" || len(items) == 0 {
				continue
			}
			value := items[0]
			if converter := s.spec.QueryConverters[name]; converter != nil {
				converted, err := converter(value)
				if err != nil {
					nethttp.Error(writer, err.Error(), nethttp.StatusBadRequest)
					return
				}
				value = fmt.Sprint(converted)
				values[name] = converted
			} else {
				values[name] = value
			}
		}
		if err := s.spec.Engine.Send(request.Context(), eventType, values); err != nil {
			nethttp.Error(writer, err.Error(), nethttp.StatusBadRequest)
			return
		}
	}
	writer.WriteHeader(nethttp.StatusOK)
}

func (s *ServerSource) shutdown() error {
	s.mu.Lock()
	server := s.server
	s.server, s.listener = nil, nil
	s.mu.Unlock()
	if server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
