package esper

import (
	"context"
	"fmt"
	"reflect"
	"sort"
)

// VariableChangeEvent is an immutable snapshot of one committed variable
// write. Global variables use PartitionID -1 and an empty ContextName.
type VariableChangeEvent struct {
	Name         string
	ContextName  string
	PartitionID  int
	PartitionKey string
	Old          Value
	New          Value
}

// VariableChangeListener receives committed variable writes. Callbacks are
// invoked after the engine lock is released, so a listener may inspect or
// update the engine without re-entrancy deadlocks.
type VariableChangeListener interface {
	OnVariableChanged(event VariableChangeEvent)
}

// AddVariableChangeListener registers a listener for one variable. An empty
// variable name observes all global and context-partitioned variables.
func (e *Engine) AddVariableChangeListener(variableName string, listener VariableChangeListener) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	if listener == nil {
		return NewError(ErrorInvalidRule, "variable change listener is nil")
	}
	if variableName != "" {
		if _, ok := e.env.Variable(variableName); !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("variable %q is not registered", variableName))
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.variableChangeListeners[variableName] = append(e.variableChangeListeners[variableName], listener)
	return nil
}

// AddVariableListener is a concise alias for AddVariableChangeListener.
func (e *Engine) AddVariableListener(variableName string, listener VariableChangeListener) error {
	return e.AddVariableChangeListener(variableName, listener)
}

// RemoveVariableChangeListener removes one listener registration.
func (e *Engine) RemoveVariableChangeListener(variableName string, listener VariableChangeListener) {
	if e == nil || listener == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	listeners := e.variableChangeListeners[variableName]
	for index, candidate := range listeners {
		if sameVariableChangeListener(candidate, listener) {
			listeners = append(listeners[:index], listeners[index+1:]...)
			break
		}
	}
	if len(listeners) == 0 {
		delete(e.variableChangeListeners, variableName)
	} else {
		e.variableChangeListeners[variableName] = listeners
	}
}

// RemoveVariableListener is a concise alias for RemoveVariableChangeListener.
func (e *Engine) RemoveVariableListener(variableName string, listener VariableChangeListener) {
	e.RemoveVariableChangeListener(variableName, listener)
}

// VariableChangeListeners returns a registration-order snapshot.
func (e *Engine) VariableChangeListeners(variableName string) []VariableChangeListener {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]VariableChangeListener(nil), e.variableChangeListeners[variableName]...)
}

// RemoveVariableChangeListeners removes listeners for one variable. Passing
// an empty name removes wildcard listeners.
func (e *Engine) RemoveVariableChangeListeners(variableName string) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.variableChangeListeners, variableName)
}

func sameVariableChangeListener(left, right VariableChangeListener) bool {
	if left == nil || right == nil || reflect.TypeOf(left) != reflect.TypeOf(right) {
		return false
	}
	typ := reflect.TypeOf(left)
	if typ.Comparable() {
		return left == right
	}
	leftValue := reflect.ValueOf(left)
	rightValue := reflect.ValueOf(right)
	if leftValue.Kind() == reflect.Pointer && rightValue.Kind() == reflect.Pointer {
		return leftValue.Pointer() == rightValue.Pointer()
	}
	return false
}

// ContextVariableState is a consistent snapshot of all requested variables
// for one active context partition.
type ContextVariableState struct {
	ContextName string
	PartitionID int
	Key         string
	Descriptor  ContextPartitionDescriptor
	Values      map[string]Value
}

// VariableValues returns a consistent snapshot of named global variables. If
// no names are supplied, all global variables are returned.
func (e *Engine) VariableValues(ctx context.Context, names ...string) (map[string]Value, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if e == nil || e.env == nil {
		return nil, NewError(ErrorDependency, "engine has no environment")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, NewError(ErrorState, "engine is closed")
	}
	e.refreshVariablesLocked()
	if len(names) == 0 {
		return cloneValues(e.variables), nil
	}
	result := make(map[string]Value, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, exists := seen[name]; exists {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("variable %q is requested more than once", name))
		}
		seen[name] = struct{}{}
		definition, ok := e.env.Variable(name)
		if !ok {
			return nil, NewError(ErrorUnknownName, fmt.Sprintf("variable %q is not registered", name))
		}
		if definition.context != "" {
			return nil, NewError(ErrorState, fmt.Sprintf("variable %q is context-partitioned", name))
		}
		result[name] = e.variables[name]
	}
	return result, nil
}

// ContextVariableStates returns a consistent snapshot for each active
// partition accepted by selector. Names must all belong to contextName; when
// omitted, all variables registered for the context are included.
func (e *Engine) ContextVariableStates(ctx context.Context, contextName string, selector ContextPartitionSelector, names ...string) ([]ContextVariableState, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if e == nil || e.env == nil {
		return nil, NewError(ErrorDependency, "engine has no environment")
	}
	definition, ok := e.env.Context(contextName)
	if !ok {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", contextName))
	}
	if err := validateContextPartitionSelector(definition, selector); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, NewError(ErrorState, "engine is closed")
	}
	variableNames, err := e.contextVariableNamesLocked(contextName, names)
	if err != nil {
		return nil, err
	}
	descriptors := make([]ContextPartitionDescriptor, 0, len(e.contextPartitionDescriptors[contextName]))
	for _, descriptor := range e.contextPartitionDescriptors[contextName] {
		descriptor.properties = cloneValues(descriptor.properties)
		if contextPartitionSelectedWithDescriptor(selector, descriptor) {
			descriptors = append(descriptors, descriptor)
		}
	}
	sort.Slice(descriptors, func(i, j int) bool {
		if descriptors[i].ID != descriptors[j].ID {
			return descriptors[i].ID < descriptors[j].ID
		}
		return descriptors[i].Key < descriptors[j].Key
	})
	result := make([]ContextVariableState, 0, len(descriptors))
	for _, descriptor := range descriptors {
		state := e.ensureContextVariablePartitionLocked(contextName, descriptor.Key)
		values := make(map[string]Value, len(variableNames))
		for _, name := range variableNames {
			values[name] = state[name]
		}
		result = append(result, ContextVariableState{
			ContextName: contextName,
			PartitionID: descriptor.ID,
			Key:         descriptor.Key,
			Descriptor:  descriptor,
			Values:      values,
		})
	}
	return result, nil
}

func (e *Engine) takeVariableChangesLocked() []VariableChangeEvent {
	if e == nil || len(e.pendingVariableChanges) == 0 {
		return nil
	}
	changes := append([]VariableChangeEvent(nil), e.pendingVariableChanges...)
	e.pendingVariableChanges = nil
	return changes
}

func (e *Engine) dispatchVariableChanges(changes []VariableChangeEvent) {
	if e == nil {
		return
	}
	for _, change := range changes {
		e.mu.Lock()
		listeners := append([]VariableChangeListener(nil), e.variableChangeListeners[""]...)
		listeners = append(listeners, e.variableChangeListeners[change.Name]...)
		e.mu.Unlock()
		seen := make([]VariableChangeListener, 0, len(listeners))
		for _, listener := range listeners {
			duplicate := false
			for _, prior := range seen {
				if sameVariableChangeListener(prior, listener) {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			seen = append(seen, listener)
			listener.OnVariableChanged(change)
		}
	}
}

func (e *Engine) contextVariableNamesLocked(contextName string, names []string) ([]string, error) {
	if len(names) == 0 {
		result := make([]string, 0)
		e.env.mu.RLock()
		for name, definition := range e.env.variables {
			if definition.context == contextName {
				result = append(result, name)
			}
		}
		e.env.mu.RUnlock()
		sort.Strings(result)
		return result, nil
	}
	result := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, exists := seen[name]; exists {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("context variable %q is requested more than once", name))
		}
		seen[name] = struct{}{}
		definition, ok := e.env.Variable(name)
		if !ok || definition.context != contextName {
			return nil, NewError(ErrorUnknownName, fmt.Sprintf("context variable %q is not registered for context %q", name, contextName))
		}
		result = append(result, name)
	}
	return result, nil
}
