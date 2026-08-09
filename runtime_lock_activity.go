package esper

import "sync"

// LockActivityPhase identifies one exact Engine mutex transition.
type LockActivityPhase string

const (
	LockActivityAttempt  LockActivityPhase = "attempt"
	LockActivityAcquired LockActivityPhase = "acquired"
	LockActivityReleased LockActivityPhase = "released"
)

// LockActivityEvent is an ordered diagnostic emitted by the Engine mutex.
// Sequence is monotonic within one Engine and makes traces deterministic even
// when wall-clock timestamps would be too coarse or platform-dependent.
type LockActivityEvent struct {
	Sequence uint64
	Phase    LockActivityPhase
}

const defaultLockActivityCapacity = 4096

type lockActivityRecorder struct {
	mu       sync.Mutex
	next     uint64
	events   []LockActivityEvent
	start    int
	capacity int
}

func newLockActivityRecorder() *lockActivityRecorder {
	return &lockActivityRecorder{capacity: defaultLockActivityCapacity}
}

func (r *lockActivityRecorder) record(phase LockActivityPhase) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	event := LockActivityEvent{Sequence: r.next, Phase: phase}
	if len(r.events) < r.capacity {
		r.events = append(r.events, event)
		return
	}
	r.events[r.start] = event
	r.start = (r.start + 1) % r.capacity
}

func (r *lockActivityRecorder) snapshot() []LockActivityEvent {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.start == 0 || len(r.events) < r.capacity {
		return append([]LockActivityEvent(nil), r.events...)
	}
	snapshot := make([]LockActivityEvent, 0, len(r.events))
	snapshot = append(snapshot, r.events[r.start:]...)
	snapshot = append(snapshot, r.events[:r.start]...)
	return snapshot
}

func (r *lockActivityRecorder) clear() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.events = nil
	r.start = 0
	r.mu.Unlock()
}

type runtimeMutex struct {
	mu       sync.Mutex
	recorder *lockActivityRecorder
}

func (m *runtimeMutex) Lock() {
	if m == nil {
		return
	}
	m.recorder.record(LockActivityAttempt)
	m.mu.Lock()
	m.recorder.record(LockActivityAcquired)
}

func (m *runtimeMutex) Unlock() {
	if m == nil {
		return
	}
	m.mu.Unlock()
	m.recorder.record(LockActivityReleased)
}

// LockActivity returns a detached trace snapshot. Calling it does not acquire
// the traced Engine mutex and therefore does not add self-observation events.
func (e *Engine) LockActivity() []LockActivityEvent {
	if e == nil {
		return nil
	}
	return e.lockActivity.snapshot()
}

// ClearLockActivity removes retained diagnostic events without disabling
// tracing or resetting the monotonic sequence.
func (e *Engine) ClearLockActivity() {
	if e == nil {
		return
	}
	e.lockActivity.clear()
}
