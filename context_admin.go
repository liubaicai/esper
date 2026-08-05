package esper

import (
	"context"
	"fmt"
	"sort"
)

// ContextPartitionDescriptors returns the engine-level snapshot of all
// currently materialized partitions for a context. Unlike the statement
// helper, this view de-duplicates partitions shared by multiple statements.
func (e *Engine) ContextPartitionDescriptors(contextName string, selector ContextPartitionSelector) ([]ContextPartitionDescriptor, error) {
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
	return descriptors, nil
}

// ContextPartitionCount returns the number of active context partitions,
// de-duplicated across statements using the same context.
func (e *Engine) ContextPartitionCount(contextName string) (int, error) {
	descriptors, err := e.ContextPartitionDescriptors(contextName, nil)
	if err != nil {
		return 0, err
	}
	return len(descriptors), nil
}

// ContextPartitionIDs returns active partition IDs accepted by selector.
func (e *Engine) ContextPartitionIDs(contextName string, selector ContextPartitionSelector) ([]int, error) {
	descriptors, err := e.ContextPartitionDescriptors(contextName, selector)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	return ids, nil
}

// ContextPartition returns one active partition descriptor by its engine-level
// ID. The boolean is false when the ID is not currently materialized.
func (e *Engine) ContextPartition(contextName string, id int) (ContextPartitionDescriptor, bool, error) {
	descriptors, err := e.ContextPartitionDescriptors(contextName, SelectContextPartitionIDs(id))
	if err != nil {
		return ContextPartitionDescriptor{}, false, err
	}
	if len(descriptors) == 0 {
		return ContextPartitionDescriptor{}, false, nil
	}
	return descriptors[0], true, nil
}

// ContextPartitionProperties returns a defensive copy of one partition's
// current context properties.
func (e *Engine) ContextPartitionProperties(contextName string, id int) (map[string]any, bool, error) {
	descriptor, ok, err := e.ContextPartition(contextName, id)
	if err != nil || !ok {
		return nil, ok, err
	}
	return descriptor.Properties(), true, nil
}

// ContextStatementNames returns active statement names associated with a
// context in deterministic order.
func (e *Engine) ContextStatementNames(contextName string) ([]string, error) {
	if e == nil || e.env == nil {
		return nil, NewError(ErrorDependency, "engine has no environment")
	}
	if _, ok := e.env.Context(contextName); !ok {
		return nil, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", contextName))
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	names := make([]string, 0)
	for _, statement := range e.statements {
		if statement == nil || statement.plan.query.contextName != contextName {
			continue
		}
		statement.mu.RLock()
		closed := statement.closed || statement.state == StatementDestroyed
		statement.mu.RUnlock()
		if !closed {
			names = append(names, statement.name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// ContextNestingLevel returns one for a flat context and the number of
// declared parent levels for a nested context.
func (e *Engine) ContextNestingLevel(contextName string) (int, error) {
	if e == nil || e.env == nil {
		return 0, NewError(ErrorDependency, "engine has no environment")
	}
	definition, ok := e.env.Context(contextName)
	if !ok {
		return 0, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", contextName))
	}
	level := 1
	for {
		parent, hasParent := definition.Parent()
		if !hasParent {
			return level, nil
		}
		level++
		definition = parent
	}
}

// DestroyContext removes an Environment context definition after all
// statements using it have been destroyed. It is the Go lifecycle counterpart
// to undeploying Esper's create-context statement.
func (e *Engine) DestroyContext(ctx context.Context, contextName string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return NewError(ErrorState, "engine is closed")
	}
	if e.contextStatementRefs[contextName] > 0 {
		e.mu.Unlock()
		return NewError(ErrorDependency, fmt.Sprintf("context %q still has deployed statements", contextName))
	}
	if err := e.env.removeContextDefinition(contextName); err != nil {
		e.mu.Unlock()
		return err
	}
	delete(e.contextCreated, contextName)
	delete(e.contextPartitionRefs, contextName)
	delete(e.contextPartitionIDs, contextName)
	delete(e.contextPartitionDescriptors, contextName)
	delete(e.contextPartitionListeners, contextName)
	delete(e.contextTemporalOrigins, contextName)
	delete(e.contextVariables, contextName)
	delete(e.contextStatementRefs, contextName)
	delete(e.contextPartitionNextIDs, contextName)
	e.queueContextDestroyedLocked(contextName)
	events := e.takeContextEventsLocked()
	e.mu.Unlock()
	e.dispatchContextEvents(events)
	return nil
}

// RemoveContext is a concise alias for DestroyContext.
func (e *Engine) RemoveContext(ctx context.Context, contextName string) error {
	return e.DestroyContext(ctx, contextName)
}
