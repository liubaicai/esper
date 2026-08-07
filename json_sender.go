package esper

import (
	"context"
	"fmt"
)

// JSONEventSender is the typed JSON ingress for one registered event type.
// It groups parse and send operations so connectors and dataflow operators do
// not repeatedly resolve the event schema by name.
type JSONEventSender struct {
	engine    *Engine
	eventType string
	schema    Schema
}

// JSONSender returns a sender bound to a registered JSON event type.
func (e *Engine) JSONSender(eventType string) (*JSONEventSender, error) {
	if e == nil || e.env == nil {
		return nil, NewError(ErrorDependency, "engine has no environment")
	}
	schema, ok := e.env.Schema(eventType)
	if !ok || schema.Kind() != SchemaJSON {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("JSON event type %q is not registered", eventType))
	}
	return &JSONEventSender{engine: e, eventType: eventType, schema: schema}, nil
}

func (s *JSONEventSender) EventType() string {
	if s == nil {
		return ""
	}
	return s.eventType
}

func (s *JSONEventSender) Schema() Schema {
	if s == nil {
		return Schema{}
	}
	return s.schema
}

// Parse parses a JSON object using the sender's schema and the engine clock.
func (s *JSONEventSender) Parse(data []byte) (Event, error) {
	if s == nil || s.engine == nil {
		return Event{}, NewError(ErrorDependency, "JSON sender is nil")
	}
	return ParseJSON(s.schema, data, s.engine.Now())
}

// Send parses and dispatches one JSON payload.
func (s *JSONEventSender) Send(ctx context.Context, data []byte) error {
	event, err := s.Parse(data)
	if err != nil {
		return err
	}
	return s.SendEvent(ctx, event)
}

// Route parses and routes one JSON payload through the engine's route entry.
// It mirrors Send while keeping the route operation explicit at call sites.
func (s *JSONEventSender) Route(ctx context.Context, data []byte) error {
	event, err := s.Parse(data)
	if err != nil {
		return err
	}
	return s.RouteEvent(ctx, event)
}

// SendEvent dispatches an event previously parsed by this sender. The
// sender/schema check prevents an event parsed for another type or separately
// constructed schema from being accidentally sent to this stream.
func (s *JSONEventSender) SendEvent(ctx context.Context, event Event) error {
	if s == nil || s.engine == nil {
		return NewError(ErrorDependency, "JSON sender is nil")
	}
	if !s.acceptsEvent(event) {
		return NewError(ErrorTypeMismatch, fmt.Sprintf("JSON sender for %q received event type %q", s.eventType, event.TypeName()))
	}
	return s.engine.send(ctx, s.eventType, event.Underlying(), event.jsonRaw)
}

// RouteEvent routes an event previously parsed by this sender. The raw JSON
// tree is retained just as it is for SendEvent.
func (s *JSONEventSender) RouteEvent(ctx context.Context, event Event) error {
	if s == nil || s.engine == nil {
		return NewError(ErrorDependency, "JSON sender is nil")
	}
	if !s.acceptsEvent(event) {
		return NewError(ErrorTypeMismatch, fmt.Sprintf("JSON sender for %q received event type %q", s.eventType, event.TypeName()))
	}
	return s.engine.send(ctx, s.eventType, event.Underlying(), event.jsonRaw)
}

// SendUnderlying dispatches a schema-compatible Go map or typed underlying
// value without a JSON parse step.
func (s *JSONEventSender) SendUnderlying(ctx context.Context, underlying any) error {
	if s == nil || s.engine == nil {
		return NewError(ErrorDependency, "JSON sender is nil")
	}
	return s.engine.Send(ctx, s.eventType, underlying)
}

// RouteUnderlying routes a schema-compatible Go map or typed underlying value
// without a JSON parse step.
func (s *JSONEventSender) RouteUnderlying(ctx context.Context, underlying any) error {
	if s == nil || s.engine == nil {
		return NewError(ErrorDependency, "JSON sender is nil")
	}
	return s.engine.Route(ctx, s.eventType, underlying)
}

func (s *JSONEventSender) acceptsEvent(event Event) bool {
	return event.TypeName() == s.eventType &&
		event.Schema().Name() == s.schema.Name() &&
		event.Schema().Kind() == SchemaJSON &&
		event.Schema().identity != nil &&
		event.Schema().identity == s.schema.identity
}
