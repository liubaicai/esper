package esper

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"
)

// Subscriber is the single replaceable, statement-aware observer for a
// statement. Unlike Listener, subscriber failures are isolated from event
// delivery and reported through Engine.SetSubscriberErrorHandler.
//
// Esper Java selects subscriber methods reflectively by name and parameter
// footprint. Go makes that contract explicit: callers inspect immutable
// SubscriberRow values and bind them to application types in ordinary Go.
type Subscriber func(context.Context, SubscriberUpdate) error

// SubscriberUpdate is one immutable new/old-stream delivery. NewRows and
// OldRows return detached slices, while each SubscriberRow keeps the same
// immutable Result value exposed to listeners.
type SubscriberUpdate struct {
	Statement *Statement
	Sequence  uint64
	Time      time.Time
	newRows   []SubscriberRow
	oldRows   []SubscriberRow
}

func newSubscriberUpdate(statement *Statement, batch ResultBatch) SubscriberUpdate {
	update := SubscriberUpdate{
		Statement: statement,
		Sequence:  batch.Sequence,
		Time:      batch.Time,
		newRows:   make([]SubscriberRow, len(batch.New)),
		oldRows:   make([]SubscriberRow, len(batch.Old)),
	}
	for index, result := range batch.New {
		update.newRows[index] = SubscriberRow{result: result}
	}
	for index, result := range batch.Old {
		update.oldRows[index] = SubscriberRow{result: result}
	}
	return update
}

func (u SubscriberUpdate) NewRows() []SubscriberRow {
	return append([]SubscriberRow(nil), u.newRows...)
}

func (u SubscriberUpdate) OldRows() []SubscriberRow {
	return append([]SubscriberRow(nil), u.oldRows...)
}

// SubscriberRow offers the three Java subscriber binding shapes without
// reflection: ordered projection values, a property map and the underlying
// wildcard payload.
type SubscriberRow struct {
	result Result
}

func (r SubscriberRow) Result() Result { return r.result }

func (r SubscriberRow) Get(name string) Value { return r.result.Get(name) }

func (r SubscriberRow) Underlying() any { return r.result.Underlying() }

// Values returns projection columns in schema order. A wildcard event is one
// value containing its underlying Go event, matching Esper's one-parameter
// wildcard subscriber footprint.
func (r SubscriberRow) Values() []Value {
	if r.result.row != nil {
		values := r.result.row.Values()
		for index, value := range values {
			if !value.IsPresent() {
				continue
			}
			switch event := value.Any().(type) {
			case Event:
				values[index] = Present(event.Underlying())
			case *Event:
				if event != nil {
					values[index] = Present(event.Underlying())
				}
			}
		}
		return values
	}
	if r.result.event != nil {
		underlying := r.result.event.Underlying()
		if tuple, ok := underlying.(joinTuple); ok {
			values := make([]Value, len(tuple.events))
			for index, event := range tuple.events {
				if event.schema.valid() {
					values[index] = Present(event.Underlying())
				} else {
					values[index] = Null()
				}
			}
			return values
		}
		return []Value{Present(underlying)}
	}
	return nil
}

func (r SubscriberRow) Map() map[string]any {
	if r.result.row != nil {
		return r.result.row.AsMap()
	}
	if r.result.event == nil {
		return nil
	}
	result := make(map[string]any, len(r.result.event.schema.fields))
	for _, field := range r.result.event.schema.fields {
		result[field.Name] = r.result.event.Get(field.Name).Any()
	}
	return result
}

// SubscriberError describes a recovered panic or returned callback error.
// Stack is populated only for a panic.
type SubscriberError struct {
	Statement *Statement
	Cause     error
	Panic     any
	Stack     []byte
}

func (e SubscriberError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return fmt.Sprintf("subscriber panic: %v", e.Panic)
}

// SubscriberErrorHandler receives isolated subscriber failures. The default
// nil handler discards them, matching Esper's observer-exception isolation.
type SubscriberErrorHandler func(context.Context, SubscriberError)

func invokeSubscriber(ctx context.Context, subscriber Subscriber, update SubscriberUpdate) (reported SubscriberError, failed bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			reported = SubscriberError{Statement: update.Statement, Panic: recovered, Stack: debug.Stack()}
			failed = true
		}
	}()
	if err := subscriber(ctx, update); err != nil {
		return SubscriberError{Statement: update.Statement, Cause: err}, true
	}
	return SubscriberError{}, false
}
