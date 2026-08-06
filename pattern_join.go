package esper

import (
	"fmt"
	"time"
)

// patternJoinRuntime owns the hidden pattern statement used to turn completed
// Pattern matches into ordinary join-side events. Keeping this state separate
// from the parent Join runtime is important: each Pattern source has its own
// active NFA branches, while the parent still owns tuple/window diffs.
type patternJoinRuntime struct {
	runtime *statementRuntime
	plan    Plan
	schema  Schema
	tags    []string
}

func patternJoinSchema(definition *patternDefinition) (Schema, error) {
	if definition == nil {
		return Schema{}, NewError(ErrorInvalidRule, "pattern join source requires a pattern definition")
	}
	tags := patternDefinitionTagNames(definition)
	if len(tags) == 0 {
		return Schema{}, NewError(ErrorInvalidRule, "pattern join source requires at least one event tag")
	}
	fields := make([]FieldSpec, 0, len(tags))
	for _, tag := range tags {
		fields = append(fields, FieldDef(tag, typeOf[Event]()))
	}
	name := "pattern-join"
	if definition.input != nil {
		name += ":" + definition.input.describe()
	}
	return NewMapSchema(name, fields)
}

func patternDefinitionAcceptsEvent(env *Environment, definition *patternDefinition, event Event) bool {
	for _, input := range patternDefinitionInputs(definition) {
		if sourceNodeAcceptsEvent(env, input, event) {
			return true
		}
	}
	return false
}

func (r *statementRuntime) patternJoinRuntimeFor(node *streamNode) (*patternJoinRuntime, error) {
	if r == nil || node == nil || node.kind != streamPattern || node.pattern == nil {
		return nil, NewError(ErrorInvalidRule, "invalid pattern join source")
	}
	if r.patternJoinStates == nil {
		r.patternJoinStates = make(map[*streamNode]*patternJoinRuntime)
	}
	if existing := r.patternJoinStates[node]; existing != nil {
		return existing, nil
	}
	schema, err := patternJoinSchema(node.pattern)
	if err != nil {
		return nil, err
	}
	tags := patternDefinitionTagNames(node.pattern)
	selections := make([]Selection, 0, len(tags))
	for _, tag := range tags {
		selections = append(selections, Alias(tag, PatternEvent(tag)))
	}
	query := Query{
		env:               r.query.env,
		input:             node.pattern.input,
		pattern:           node.pattern,
		patternSelections: selections,
		selector:          SelectIStream,
		output:            OutputAll(),
		name:              "pattern-join-" + node.sourceName,
	}
	plan, err := r.query.env.Build(query)
	if err != nil {
		return nil, err
	}
	child := newStatementRuntime(plan.query)
	child.engine = r.engine
	child.ctx = r.ctx
	child.variables = cloneValues(r.variables)
	state := &patternJoinRuntime{runtime: &child, plan: plan, schema: schema, tags: tags}
	r.patternJoinStates[node] = state
	return state, nil
}

// insertPatternInputs feeds an event into each distinct source chain used by
// a multi-source Pattern. A branch source's filter/window is evaluated before
// the shared pattern NFA sees the event; this preserves the normal Stream
// source semantics while allowing `A.And(B)` across different event types.
func (r *statementRuntime) insertPatternInputs(definition *patternDefinition, event Event, now time.Time) (eventDelta, error) {
	if definition == nil {
		return eventDelta{}, NewError(ErrorInvalidRule, "pattern definition is required")
	}
	result := eventDelta{}
	seen := make(map[*streamNode]struct{})
	for _, input := range patternDefinitionInputs(definition) {
		if input == nil {
			continue
		}
		if _, exists := seen[input]; exists {
			continue
		}
		seen[input] = struct{}{}
		if !sourceNodeAcceptsEvent(r.query.env, input, event) {
			continue
		}
		delta, err := r.insert(input, event, now)
		if err != nil {
			return eventDelta{}, err
		}
		result = mergeDelta(result, delta)
	}
	return result, nil
}

func (r *statementRuntime) insertPatternSource(node *streamNode, event Event, now time.Time) (eventDelta, error) {
	if !sourceNodeAcceptsEvent(r.query.env, node, event) {
		return eventDelta{}, nil
	}
	patternRuntime, err := r.patternJoinRuntimeFor(node)
	if err != nil {
		return eventDelta{}, err
	}
	patternRuntime.runtime.ctx = r.ctx
	batch, _, err := patternRuntime.runtime.process(patternRuntime.plan, event, now, r.variables)
	if err != nil {
		return eventDelta{}, err
	}
	state := r.windows[node]
	if state == nil {
		state = &windowRuntimeState{}
	}
	spec := node.patternWindow
	if spec == nil {
		spec = KeepAll()
	}
	result := eventDelta{}
	for _, output := range batch.New {
		row, ok := output.Row()
		if !ok {
			return eventDelta{}, fmt.Errorf("esper: pattern join source produced a non-row result")
		}
		values := make(map[string]any, len(patternRuntime.tags))
		for _, tag := range patternRuntime.tags {
			value := row.Get(tag)
			if value.IsPresent() {
				values[tag] = value.Any()
			}
		}
		materialized, materializeErr := newEvent(patternRuntime.schema, values, now)
		if materializeErr != nil {
			return eventDelta{}, materializeErr
		}
		delta, addErr := r.addToWindow(spec, state, materialized, now)
		if addErr != nil {
			return eventDelta{}, addErr
		}
		result = mergeDelta(result, delta)
	}
	r.windows[node] = state
	return result, nil
}
