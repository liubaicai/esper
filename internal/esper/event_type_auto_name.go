package esper

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
)

// EventTypeAutoNameRegistry is an application-owned catalog for resolving
// named Go event types by their short type name. It is safe for concurrent use
// and its zero value is ready to use.
//
// Unlike Esper's Java package scanning, registrations are explicit. This keeps
// resolution deterministic and avoids process-global reflection state while
// preserving unique-name and ambiguous-name compile semantics.
type EventTypeAutoNameRegistry struct {
	mu         sync.RWMutex
	candidates map[string][]eventTypeAutoNameCandidate
}

type eventTypeAutoNameCandidate struct {
	namespace string
	typ       reflect.Type
}

// AutoNameOption customizes the short name or namespace recorded for a Go
// event type.
type AutoNameOption func(*autoNameConfig)

type autoNameConfig struct {
	name      string
	namespace string
}

// WithAutoName overrides the default short name derived from reflect.Type.Name.
func WithAutoName(name string) AutoNameOption {
	return func(config *autoNameConfig) {
		config.name = name
	}
}

// WithAutoNameNamespace overrides the default namespace derived from
// reflect.Type.PkgPath.
func WithAutoNameNamespace(namespace string) AutoNameOption {
	return func(config *autoNameConfig) {
		config.namespace = namespace
	}
}

// NewEventTypeAutoNameRegistry returns an empty application-owned registry.
func NewEventTypeAutoNameRegistry() *EventTypeAutoNameRegistry {
	return &EventTypeAutoNameRegistry{}
}

// RegisterAutoNameType adds T to registry. Repeating the exact same
// short-name, namespace and type registration is idempotent. Different
// namespaces may intentionally register the same short name; Resolve then
// reports ErrorAmbiguous with a stable, sorted diagnostic.
func RegisterAutoNameType[T any](registry *EventTypeAutoNameRegistry, options ...AutoNameOption) error {
	if registry == nil {
		return NewError(ErrorDependency, "event auto-name registry is nil")
	}
	typ := normalizedAutoNameType(typeOf[T]())
	if typ.Kind() != reflect.Struct || typ.Name() == "" {
		return NewError(ErrorTypeMismatch, fmt.Sprintf("event auto-name registration requires a named struct type, got %s", typ))
	}
	config := autoNameConfig{name: typ.Name(), namespace: typ.PkgPath()}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	config.name = strings.TrimSpace(config.name)
	config.namespace = strings.TrimSpace(config.namespace)
	if config.name == "" {
		return NewError(ErrorInvalidRule, "event auto-name registration requires a non-blank short name")
	}
	if config.namespace == "" {
		return NewError(ErrorInvalidRule, fmt.Sprintf("event auto-name %q requires a non-blank namespace", config.name))
	}
	return registry.register(config.name, config.namespace, typ)
}

func normalizedAutoNameType(typ reflect.Type) reflect.Type {
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

func (r *EventTypeAutoNameRegistry) register(name, namespace string, typ reflect.Type) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.candidates == nil {
		r.candidates = make(map[string][]eventTypeAutoNameCandidate)
	}
	existing := r.candidates[name]
	for _, candidate := range existing {
		if candidate.namespace != namespace {
			continue
		}
		if candidate.typ == typ {
			return nil
		}
		return NewError(ErrorAmbiguous, fmt.Sprintf(
			"event auto-name %q in namespace %q is already registered for %s and cannot also register %s",
			name, namespace, candidate.typ, typ,
		))
	}
	r.candidates[name] = append(existing, eventTypeAutoNameCandidate{namespace: namespace, typ: typ})
	return nil
}

// Resolve returns the unique Go type registered for name. Missing names return
// ErrorUnknownName; names present in multiple namespaces return ErrorAmbiguous.
func (r *EventTypeAutoNameRegistry) Resolve(name string) (reflect.Type, error) {
	if r == nil {
		return nil, NewError(ErrorDependency, "event auto-name registry is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, NewError(ErrorInvalidRule, "event auto-name resolution requires a non-blank name")
	}
	r.mu.RLock()
	candidates := append([]eventTypeAutoNameCandidate(nil), r.candidates[name]...)
	r.mu.RUnlock()
	if len(candidates) == 0 {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("event auto-name %q is not registered", name))
	}
	if len(candidates) > 1 {
		namespaces := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			namespaces = append(namespaces, candidate.namespace)
		}
		sort.Strings(namespaces)
		quoted := make([]string, len(namespaces))
		for index, namespace := range namespaces {
			quoted[index] = fmt.Sprintf("%q", namespace)
		}
		return nil, NewError(ErrorAmbiguous, fmt.Sprintf(
			"event auto-name %q is registered in multiple namespaces: %s",
			name, strings.Join(quoted, ", "),
		))
	}
	return candidates[0].typ, nil
}

// RegisterAutoNamedStruct resolves T by its short Go type name and registers
// the resulting struct schema in env under alias. The resolved registry type
// must exactly match T after pointer normalization.
func RegisterAutoNamedStruct[T any](env *Environment, registry *EventTypeAutoNameRegistry, alias string, options ...SchemaOption) (Schema, error) {
	expected := normalizedAutoNameType(typeOf[T]())
	if expected == nil || expected.Name() == "" {
		return Schema{}, NewError(ErrorTypeMismatch, fmt.Sprintf("event auto-name schema requires a named type, got %s", expected))
	}
	resolved, err := registry.Resolve(expected.Name())
	if err != nil {
		return Schema{}, err
	}
	if resolved != expected {
		return Schema{}, NewError(ErrorTypeMismatch, fmt.Sprintf(
			"event auto-name %q resolved to %s, expected %s",
			expected.Name(), resolved, expected,
		))
	}
	return RegisterStruct[T](env, alias, options...)
}
