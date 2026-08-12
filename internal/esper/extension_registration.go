package esper

import (
	"fmt"
	"strings"
	"sync"
)

// CustomPatternGuardFactory creates a stateful guard function from declared
// parameters. The returned GuardFunc returns true while the pattern may
// continue accepting matches; returning false stops the branch.
type CustomPatternGuardFactory func(params []Value) (GuardFunc, error)

// GuardFunc is the per-branch guard closure returned by a
// CustomPatternGuardFactory.
type GuardFunc func() bool

// CustomViewFactory creates a WindowSpec from typed parameters.
type CustomViewFactory func(params []Value) (WindowSpec, error)

// VirtualDataWindowProvider is the Go-native equivalent of Esper's
// VirtualDataWindowForge. It supplies data for a named window backed by
// external data.
type VirtualDataWindowProvider interface {
	Data() []Event
	Lookup(hashField string, hashValues []any, rangeField string, rangeMin, rangeMax any) []Event
}

type extensionRegistry struct {
	mu                 sync.RWMutex
	patternGuards      map[string]CustomPatternGuardFactory
	customViews        map[string]CustomViewFactory
	virtualDataWindows map[string]VirtualDataWindowProvider
}

func newExtensionRegistry() *extensionRegistry {
	return &extensionRegistry{
		patternGuards:      make(map[string]CustomPatternGuardFactory),
		customViews:        make(map[string]CustomViewFactory),
		virtualDataWindows: make(map[string]VirtualDataWindowProvider),
	}
}

func (e *Environment) RegisterPatternGuard(name string, factory CustomPatternGuardFactory) error {
	if e == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return NewError(ErrorInvalidRule, "pattern guard name is required")
	}
	if factory == nil {
		return NewError(ErrorInvalidRule, fmt.Sprintf("pattern guard %q requires a factory", name))
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.extensions.patternGuards[name]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("pattern guard %q is already registered", name))
	}
	e.extensions.patternGuards[name] = factory
	return nil
}

func (e *Environment) patternGuardFactory(name string) (CustomPatternGuardFactory, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	factory, ok := e.extensions.patternGuards[name]
	return factory, ok
}

func (e *Environment) RegisterCustomView(name string, factory CustomViewFactory) error {
	if e == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return NewError(ErrorInvalidRule, "custom view name is required")
	}
	if factory == nil {
		return NewError(ErrorInvalidRule, fmt.Sprintf("custom view %q requires a factory", name))
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.extensions.customViews[name]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("custom view %q is already registered", name))
	}
	e.extensions.customViews[name] = factory
	return nil
}

func (e *Environment) RegisterVirtualDataWindow(namedWindow string, provider VirtualDataWindowProvider) error {
	if e == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	namedWindow = strings.TrimSpace(namedWindow)
	if namedWindow == "" {
		return NewError(ErrorInvalidRule, "virtual data window named-window name is required")
	}
	if provider == nil {
		return NewError(ErrorInvalidRule, fmt.Sprintf("virtual data window %q requires a provider", namedWindow))
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.extensions.virtualDataWindows[namedWindow]; exists {
		return NewError(ErrorDependency, fmt.Sprintf("virtual data window for %q is already registered", namedWindow))
	}
	e.extensions.virtualDataWindows[namedWindow] = provider
	return nil
}

func (e *Environment) virtualDataWindowProvider(namedWindow string) (VirtualDataWindowProvider, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	provider, ok := e.extensions.virtualDataWindows[namedWindow]
	return provider, ok
}
