package esper

import (
	"context"
	"fmt"
	"iter"
)

// TypedMethodProvider adapts a Go method that returns declared underlying
// values into the Event-based MethodProvider boundary. T may be a registered
// struct, map, object-array, JSON, XML, Avro or another value accepted by the
// supplied schema; no reflection-based method lookup is performed.
type TypedMethodProvider[T any] struct {
	schema Schema
	poll   func(context.Context, MethodRequest) ([]T, error)
}

// NewTypedMethodProvider creates a method provider for a slice-shaped return.
// The schema owns representation normalization and field validation.
func NewTypedMethodProvider[T any](schema Schema, poll func(context.Context, MethodRequest) ([]T, error)) (*TypedMethodProvider[T], error) {
	if !schema.valid() {
		return nil, NewError(ErrorInvalidRule, "typed method provider requires a valid schema")
	}
	if poll == nil {
		return nil, NewError(ErrorDependency, "typed method provider function is nil")
	}
	return &TypedMethodProvider[T]{schema: schema, poll: poll}, nil
}

// NewSingleMethodProvider adapts a method that returns one declared value.
// A nil or otherwise invalid value is reported by the schema materializer.
func NewSingleMethodProvider[T any](schema Schema, poll func(context.Context, MethodRequest) (T, error)) (*TypedMethodProvider[T], error) {
	if poll == nil {
		return nil, NewError(ErrorDependency, "single method provider function is nil")
	}
	return NewTypedMethodProvider(schema, func(ctx context.Context, request MethodRequest) ([]T, error) {
		value, err := poll(ctx, request)
		if err != nil {
			return nil, err
		}
		return []T{value}, nil
	})
}

// NewSequenceMethodProvider adapts a reusable Go iter.Seq return. The
// sequence is consumed once for each provider invocation, preserving yield
// order in the resulting Event slice.
func NewSequenceMethodProvider[T any](schema Schema, poll func(context.Context, MethodRequest) (iter.Seq[T], error)) (*TypedMethodProvider[T], error) {
	if poll == nil {
		return nil, NewError(ErrorDependency, "sequence method provider function is nil")
	}
	return NewTypedMethodProvider(schema, func(ctx context.Context, request MethodRequest) ([]T, error) {
		sequence, err := poll(ctx, request)
		if err != nil {
			return nil, err
		}
		if sequence == nil {
			return nil, nil
		}
		values := make([]T, 0)
		sequence(func(value T) bool {
			values = append(values, value)
			return true
		})
		return values, nil
	})
}

func (p *TypedMethodProvider[T]) Poll(ctx context.Context, request MethodRequest) ([]Event, error) {
	if p == nil || p.poll == nil {
		return nil, NewError(ErrorDependency, "typed method provider has no function")
	}
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	values, err := p.poll(ctx, request)
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(values))
	for index, value := range values {
		event, materializeErr := newEvent(p.schema, value, request.Now)
		if materializeErr != nil {
			return nil, WrapError(ErrorTypeMismatch, fmt.Sprintf("method result %d", index), materializeErr)
		}
		events = append(events, event)
	}
	return events, nil
}
