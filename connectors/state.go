// Package connectors contains the lifecycle primitives shared by Esper
// connector implementations.
package connectors

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// State is the lifecycle state used by input and output adapters. The names
// deliberately mirror Esper's AdapterState while remaining idiomatic Go
// values that can be inspected without depending on a concrete connector.
type State uint8

const (
	Opened State = iota
	Started
	Paused
	Destroyed
)

func (s State) String() string {
	switch s {
	case Opened:
		return "OPENED"
	case Started:
		return "STARTED"
	case Paused:
		return "PAUSED"
	case Destroyed:
		return "DESTROYED"
	default:
		return fmt.Sprintf("STATE(%d)", s)
	}
}

var (
	// ErrInvalidTransition indicates an operation that is not legal from the
	// current adapter state.
	ErrInvalidTransition = errors.New("connectors: invalid state transition")
	// ErrNotStarted indicates that an operation requires STARTED state.
	ErrNotStarted = errors.New("connectors: adapter is not started")
	// ErrPaused indicates that an operation cannot run while PAUSED.
	ErrPaused = errors.New("connectors: adapter is paused")
	// ErrStopped is used by a running adapter when a caller requests STOP.
	ErrStopped = errors.New("connectors: adapter stopped")
	// ErrDestroyed indicates that the adapter can no longer be used.
	ErrDestroyed = errors.New("connectors: adapter destroyed")
)

// StateTransitionError contains the Java-compatible state transition that
// failed. It can be checked with errors.Is(err, ErrInvalidTransition).
type StateTransitionError struct {
	Operation string
	From      State
	To        State
}

func (e *StateTransitionError) Error() string {
	return fmt.Sprintf("connectors: %s is invalid from %s (requires transition to %s)", e.Operation, e.From, e.To)
}

func (e *StateTransitionError) Unwrap() error { return ErrInvalidTransition }

// StateManager implements the AdapterState transition table shared by
// EsperIO-style adapters:
//
//	OPENED -> STARTED -> PAUSED -> STARTED
//	STARTED/PAUSED -> OPENED (stop)
//	OPENED/STARTED/PAUSED -> DESTROYED
//
// A stopped adapter may be started again. Changes returns a notification
// channel suitable for context-aware waits; the channel is replaced after
// every transition, so callers must obtain it again after receiving a value.
type StateManager struct {
	mu          sync.Mutex
	state       State
	startedOnce bool
	changed     chan struct{}
}

func NewStateManager() *StateManager {
	return &StateManager{state: Opened, changed: make(chan struct{})}
}

func (m *StateManager) State() State {
	if m == nil {
		return Destroyed
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// Changes returns a channel that is closed when the state changes. It is
// intended for select statements and carries no value.
func (m *StateManager) Changes() <-chan struct{} {
	if m == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.changed
}

func (m *StateManager) Start() error {
	return m.transition("start", func(state State) (State, bool) {
		if state != Opened {
			return state, false
		}
		return Started, true
	})
}

func (m *StateManager) Stop() error {
	return m.transition("stop", func(state State) (State, bool) {
		switch state {
		case Started, Paused:
			return Opened, true
		default:
			return state, false
		}
	})
}

func (m *StateManager) Pause() error {
	return m.transition("pause", func(state State) (State, bool) {
		if state != Started {
			return state, false
		}
		return Paused, true
	})
}

func (m *StateManager) Resume() error {
	return m.transition("resume", func(state State) (State, bool) {
		if state != Paused {
			return state, false
		}
		return Started, true
	})
}

func (m *StateManager) Destroy() error {
	return m.transition("destroy", func(state State) (State, bool) {
		if state == Destroyed {
			return state, false
		}
		return Destroyed, true
	})
}

func (m *StateManager) transition(operation string, decide func(State) (State, bool)) error {
	if m == nil {
		return ErrDestroyed
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	from := m.state
	to, ok := decide(from)
	if !ok {
		return &StateTransitionError{Operation: operation, From: from, To: to}
	}
	m.state = to
	if operation == "start" {
		m.startedOnce = true
	}
	close(m.changed)
	m.changed = make(chan struct{})
	return nil
}

// RequireStarted checks the state without waiting. It is used by one-shot
// operations such as Next and Write.
func (m *StateManager) RequireStarted() error {
	if m == nil {
		return ErrDestroyed
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch m.state {
	case Started:
		return nil
	case Paused:
		return ErrPaused
	case Destroyed:
		return ErrDestroyed
	default:
		return ErrNotStarted
	}
}

// WaitUntilStarted blocks until the adapter is STARTED, the context is
// cancelled, or a previously-started adapter is STOPPED/DESTROYED. The
// initial OPENED state waits for Start, which makes Run safe to launch before
// lifecycle control is handed to the caller.
func (m *StateManager) WaitUntilStarted(ctx context.Context) error {
	if m == nil {
		return ErrDestroyed
	}
	for {
		m.mu.Lock()
		switch m.state {
		case Started:
			m.mu.Unlock()
			return nil
		case Destroyed:
			m.mu.Unlock()
			return ErrDestroyed
		case Opened:
			if m.startedOnce {
				m.mu.Unlock()
				return ErrStopped
			}
		}
		changed := m.changed
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}
