package esper

import (
	"fmt"
	"strings"
)

// ContextPartitionDescriptor describes one materialized context partition.
// The descriptor is a snapshot: mutating the map returned by Properties does
// not change the statement runtime.
type ContextPartitionDescriptor struct {
	ID          int
	Key         string
	BaseKey     string
	ContextName string
	properties  map[string]Value
}

// ContextPartitionStateEvent reports allocation or deallocation of one
// materialized context partition. Descriptor is a snapshot and is safe for a
// listener to retain.
type ContextPartitionStateEvent struct {
	ContextName string
	PartitionID int
	Key         string
	BaseKey     string
	Descriptor  ContextPartitionDescriptor
	Allocated   bool
}

// ContextStateEvent is the common snapshot passed to context-management
// listeners. A context registered directly in Environment has no separate
// create-context deployment, so ContextDeploymentID is empty for now.
type ContextStateEvent struct {
	RuntimeURI            string
	ContextDeploymentID   string
	ContextName           string
	StatementDeploymentID string
	StatementName         string
}

// ContextStateListener receives context creation and destruction events.
// Context definitions are part of the Environment catalog; registering a
// listener replays the currently known created contexts once.
type ContextStateListener interface {
	OnContextCreated(event ContextStateEvent)
	OnContextDestroyed(event ContextStateEvent)
}

// ContextPartitionLifecycleListener is an optional extension implemented by
// listeners registered with AddContextPartitionStateListener. It mirrors the
// non-partition callbacks of Esper's ContextPartitionStateListener while
// keeping the existing Go allocation/deallocation interface source-compatible.
type ContextPartitionLifecycleListener interface {
	OnContextActivated(event ContextStateEvent)
	OnContextDeactivated(event ContextStateEvent)
	OnContextStatementAdded(event ContextStateEvent)
	OnContextStatementRemoved(event ContextStateEvent)
}

type contextNotificationKind uint8

const (
	contextNotificationCreated contextNotificationKind = iota
	contextNotificationDestroyed
	contextNotificationStatementAdded
	contextNotificationActivated
	contextNotificationStatementRemoved
	contextNotificationPartitionAllocated
	contextNotificationPartitionDeallocated
	contextNotificationDeactivated
)

// contextNotification is kept private so public listeners only observe
// immutable event snapshots, while the engine can preserve Java-compatible
// ordering across statement and partition lifecycle callbacks.
type contextNotification struct {
	kind      contextNotificationKind
	state     ContextStateEvent
	partition ContextPartitionStateEvent
}

// ContextPartitionStateListener receives context-level partition lifecycle
// notifications. Notifications are emitted once for a context/key while at
// least one statement keeps that partition materialized.
type ContextPartitionStateListener interface {
	OnContextPartitionAllocated(event ContextPartitionStateEvent)
	OnContextPartitionDeallocated(event ContextPartitionStateEvent)
}

// Property returns one context property in its typed Value form. Missing and
// explicit null remain distinguishable through Value.State().
func (d ContextPartitionDescriptor) Property(name string) (Value, bool) {
	value, ok := d.properties[name]
	return value, ok
}

// Properties returns the context properties as ordinary Go values. A
// missing or explicit null value is represented by nil; use Property when the
// ValueState distinction is required.
func (d ContextPartitionDescriptor) Properties() map[string]any {
	result := make(map[string]any, len(d.properties))
	for name, value := range d.properties {
		result[name] = value.Any()
	}
	return result
}

func newContextPartitionDescriptor(contextName, key string, runtime *statementRuntime) ContextPartitionDescriptor {
	descriptor := ContextPartitionDescriptor{ContextName: contextName, Key: key, BaseKey: contextPartitionBaseKey(key)}
	if runtime == nil {
		return descriptor
	}
	descriptor.ID = runtime.partitionID
	descriptor.properties = cloneValues(runtime.contextProperties)
	return descriptor
}

const overlappingContextPartitionSeparator = "\x1einstance:"

func contextPartitionBaseKey(key string) string {
	if index := strings.Index(key, overlappingContextPartitionSeparator); index >= 0 {
		return key[:index]
	}
	return key
}

// ContextPartitionSelector is the Go equivalent of Esper's runtime context
// partition selector. It deliberately selects by the stable public partition
// key exposed by Statement.ContextPartitionKeys.
type ContextPartitionSelector interface {
	SelectContextPartition(key string) bool
}

// ContextPartitionIDSelector is an optional extension for selectors that
// need Esper-style partition identity in addition to the public key. The
// runtime prefers this method when a selector implements it, while existing
// key-only selectors remain source-compatible.
type ContextPartitionIDSelector interface {
	SelectContextPartitionID(id int, key string) bool
}

// ContextPartitionDescriptorSelector is an optional selector extension for
// category and filtered selection. It receives a snapshot descriptor, so a
// selector can inspect context properties without depending on the internal
// encoded partition key.
type ContextPartitionDescriptorSelector interface {
	SelectContextPartitionDescriptor(descriptor ContextPartitionDescriptor) bool
}

// ContextPartitionSelectorAll selects every currently materialized partition.
type ContextPartitionSelectorAll struct{}

func (ContextPartitionSelectorAll) SelectContextPartition(string) bool { return true }

// ContextPartitionSelectorKeys selects an explicit set of partition keys.
type ContextPartitionSelectorKeys struct {
	Keys []string
	set  map[string]struct{}
}

func SelectContextPartitions(keys ...string) ContextPartitionSelector {
	set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		set[key] = struct{}{}
	}
	return ContextPartitionSelectorKeys{Keys: append([]string(nil), keys...), set: set}
}

func (s ContextPartitionSelectorKeys) SelectContextPartition(key string) bool {
	if s.set != nil {
		if _, ok := s.set[key]; ok {
			return true
		}
		for candidate := range s.set {
			if contextPartitionBaseKey(key) == candidate {
				return true
			}
		}
		return false
	}
	for _, candidate := range s.Keys {
		if candidate == key || contextPartitionBaseKey(key) == candidate {
			return true
		}
	}
	return false
}

// ContextPartitionSelectorIDs selects an explicit set of partition IDs. IDs
// are stable for the lifetime of a statement partition and are exposed by
// ContextPartitionDescriptor.ID.
type ContextPartitionSelectorIDs struct {
	IDs []int
	set map[int]struct{}
}

// SelectContextPartitionIDs creates a selector for the supplied partition
// IDs. The key-only method returns true so the selector can still satisfy the
// base ContextPartitionSelector contract; runtime APIs that know partition
// identity use SelectContextPartitionID.
func SelectContextPartitionIDs(ids ...int) ContextPartitionSelector {
	set := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return ContextPartitionSelectorIDs{IDs: append([]int(nil), ids...), set: set}
}

func (s ContextPartitionSelectorIDs) SelectContextPartition(string) bool { return true }

func (s ContextPartitionSelectorIDs) SelectContextPartitionID(id int, _ string) bool {
	if s.set != nil {
		_, ok := s.set[id]
		return ok
	}
	for _, candidate := range s.IDs {
		if candidate == id {
			return true
		}
	}
	return false
}

// ContextPartitionSelectorCategories selects category-context partitions by
// their public label property.
type ContextPartitionSelectorCategories struct {
	Labels []string
	set    map[string]struct{}
}

// ContextPartitionSelectorHashes selects hash-context partitions by the
// signed 32-bit hash value exposed in the descriptor's hash property.
type ContextPartitionSelectorHashes struct {
	Hashes []int64
	set    map[int64]struct{}
}

// SelectContextPartitionHashes creates a selector for hash values.
func SelectContextPartitionHashes(hashes ...int64) ContextPartitionSelector {
	set := make(map[int64]struct{}, len(hashes))
	for _, hash := range hashes {
		set[hash] = struct{}{}
	}
	return ContextPartitionSelectorHashes{Hashes: append([]int64(nil), hashes...), set: set}
}

func (s ContextPartitionSelectorHashes) SelectContextPartition(string) bool { return true }

func (s ContextPartitionSelectorHashes) SelectContextPartitionDescriptor(descriptor ContextPartitionDescriptor) bool {
	value, ok := descriptor.Property("hash")
	if !ok || value.IsNull() {
		return false
	}
	hash, ok := value.Any().(int64)
	if !ok {
		return false
	}
	if s.set != nil {
		_, ok := s.set[hash]
		return ok
	}
	for _, candidate := range s.Hashes {
		if candidate == hash {
			return true
		}
	}
	return false
}

// ContextPartitionSelectorSegmented selects a flat segmented context by its
// original key tuple. It is intentionally descriptor-based and therefore
// does not depend on the runtime's encoded key string.
type ContextPartitionSelectorSegmented struct {
	Keys [][]any
	set  map[string]struct{}
}

// SelectContextPartitionSegments creates a selector for one or more key
// tuples. Nil tuple components preserve the context null-key semantics.
func SelectContextPartitionSegments(keys ...[]any) ContextPartitionSelector {
	set := make(map[string]struct{}, len(keys))
	copyKeys := make([][]any, 0, len(keys))
	for _, key := range keys {
		cloned := append([]any(nil), key...)
		copyKeys = append(copyKeys, cloned)
		set[encodeContextSelectorTuple(cloned)] = struct{}{}
	}
	return ContextPartitionSelectorSegmented{Keys: copyKeys, set: set}
}

func (s ContextPartitionSelectorSegmented) SelectContextPartition(string) bool { return true }

func (s ContextPartitionSelectorSegmented) SelectContextPartitionDescriptor(descriptor ContextPartitionDescriptor) bool {
	values := make([]any, 0)
	for index := 1; ; index++ {
		value, ok := descriptor.Property(fmt.Sprintf("key%d", index))
		if !ok {
			break
		}
		values = append(values, value.State(), value.Any())
	}
	encoded := encodeKey(values)
	if s.set != nil {
		_, ok := s.set[encoded]
		return ok
	}
	for _, key := range s.Keys {
		if encodeContextSelectorTuple(key) == encoded {
			return true
		}
	}
	return false
}

func encodeContextSelectorTuple(values []any) string {
	encoded := make([]any, 0, len(values)*2)
	for _, value := range values {
		if value == nil {
			encoded = append(encoded, Null().State(), nil)
		} else {
			encoded = append(encoded, Present(value).State(), value)
		}
	}
	return encodeKey(encoded)
}

// ContextPartitionSelectorNested composes one selector per nesting level,
// ordered from the outermost context to the innermost context.
type ContextPartitionSelectorNested struct {
	Selectors []ContextPartitionSelector
}

// SelectNestedContextPartitions creates a selector for nested contexts.
func SelectNestedContextPartitions(selectors ...ContextPartitionSelector) ContextPartitionSelector {
	return ContextPartitionSelectorNested{Selectors: append([]ContextPartitionSelector(nil), selectors...)}
}

func (s ContextPartitionSelectorNested) SelectContextPartition(string) bool { return true }

func (s ContextPartitionSelectorNested) SelectContextPartitionDescriptor(descriptor ContextPartitionDescriptor) bool {
	if len(s.Selectors) == 0 {
		return false
	}
	for index, selector := range s.Selectors {
		prefix := strings.Repeat("parent.", len(s.Selectors)-1-index)
		projected := nestedContextDescriptorAtLevel(descriptor, prefix)
		if !contextPartitionSelectedWithDescriptor(selector, projected) {
			return false
		}
	}
	return true
}

func nestedContextDescriptorAtLevel(descriptor ContextPartitionDescriptor, prefix string) ContextPartitionDescriptor {
	projected := descriptor
	projected.properties = make(map[string]Value)
	for name, value := range descriptor.properties {
		if prefix == "" {
			if strings.HasPrefix(name, "parent.") {
				continue
			}
			projected.properties[name] = value
			continue
		}
		if strings.HasPrefix(name, prefix) {
			projected.properties[strings.TrimPrefix(name, prefix)] = value
		}
	}
	return projected
}

// SelectContextPartitionCategories creates a selector for category labels.
func SelectContextPartitionCategories(labels ...string) ContextPartitionSelector {
	set := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		set[label] = struct{}{}
	}
	return ContextPartitionSelectorCategories{Labels: append([]string(nil), labels...), set: set}
}

func (s ContextPartitionSelectorCategories) SelectContextPartition(string) bool { return true }

func (s ContextPartitionSelectorCategories) SelectContextPartitionDescriptor(descriptor ContextPartitionDescriptor) bool {
	value, ok := descriptor.Property("label")
	if !ok || value.IsNull() {
		return false
	}
	label, ok := value.Any().(string)
	if !ok {
		return false
	}
	if s.set != nil {
		_, ok := s.set[label]
		return ok
	}
	for _, candidate := range s.Labels {
		if candidate == label {
			return true
		}
	}
	return false
}

// ContextPartitionSelectorDescriptorFunc adapts an arbitrary descriptor
// predicate while retaining the base selector interface.
type ContextPartitionSelectorDescriptorFunc func(ContextPartitionDescriptor) bool

func (f ContextPartitionSelectorDescriptorFunc) SelectContextPartition(string) bool { return true }

func (f ContextPartitionSelectorDescriptorFunc) SelectContextPartitionDescriptor(descriptor ContextPartitionDescriptor) bool {
	return f != nil && f(descriptor)
}

// ContextPartitionSelectorFunc adapts a Go predicate for diagnostics and
// context-aware fire-and-forget queries.
type ContextPartitionSelectorFunc func(string) bool

func (f ContextPartitionSelectorFunc) SelectContextPartition(key string) bool {
	return f != nil && f(key)
}

func contextPartitionSelected(selector ContextPartitionSelector, key string) bool {
	return selector == nil || selector.SelectContextPartition(key)
}

func contextPartitionSelectedWithID(selector ContextPartitionSelector, id int, key string) bool {
	if selector == nil {
		return true
	}
	if idSelector, ok := selector.(ContextPartitionIDSelector); ok {
		return idSelector.SelectContextPartitionID(id, key)
	}
	return selector.SelectContextPartition(key)
}

func contextPartitionSelectedWithDescriptor(selector ContextPartitionSelector, descriptor ContextPartitionDescriptor) bool {
	if selector == nil {
		return true
	}
	if descriptorSelector, ok := selector.(ContextPartitionDescriptorSelector); ok {
		return descriptorSelector.SelectContextPartitionDescriptor(descriptor)
	}
	return contextPartitionSelectedWithID(selector, descriptor.ID, descriptor.Key)
}

// validateContextPartitionSelector rejects the built-in selector/context
// combinations that Esper reports as InvalidContextPartitionSelector. An
// unknown implementation of the base Go selector interface remains allowed
// as a user-defined filtered selector; the built-in forms are checked
// explicitly so their intent cannot silently select nothing.
func validateContextPartitionSelector(definition ContextDefinition, selector ContextPartitionSelector) error {
	if selector == nil {
		return nil
	}
	switch selector.(type) {
	case ContextPartitionSelectorAll, ContextPartitionSelectorIDs, ContextPartitionSelectorFunc, ContextPartitionSelectorDescriptorFunc:
		return nil
	case ContextPartitionSelectorKeys:
		if definition.kind == ContextKeySegmented || definition.kind == ContextInitiatedTerminated {
			return nil
		}
	case ContextPartitionSelectorSegmented:
		if definition.kind == ContextKeySegmented {
			return nil
		}
	case ContextPartitionSelectorHashes:
		if definition.kind == ContextHashSegmented {
			return nil
		}
	case ContextPartitionSelectorCategories:
		if definition.kind == ContextCategorySegmented {
			return nil
		}
	case ContextPartitionSelectorNested:
		if definition.parent != nil {
			return nil
		}
	default:
		if _, filtered := selector.(ContextPartitionDescriptorSelector); filtered {
			return nil
		}
		return nil
	}
	return NewError(ErrorInvalidRule, fmt.Sprintf("selector %T is incompatible with context %q kind %d", selector, definition.name, definition.kind))
}
