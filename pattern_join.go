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

const patternJoinMatchMarker = "__pattern_match"

func patternJoinSchema(definition *patternDefinition) (Schema, error) {
	if definition == nil {
		return Schema{}, NewError(ErrorInvalidRule, "pattern join source requires a pattern definition")
	}
	tags := patternDefinitionTagNames(definition)
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
	if len(selections) == 0 {
		// A timer-only Pattern has no captured tag to project, but it is still a
		// valid unidirectional Join trigger. The hidden query needs one measure
		// to pass the ordinary Pattern validation; the marker is intentionally
		// not exposed on the materialized Join event.
		selections = append(selections, Alias(patternJoinMatchMarker, Literal[bool](true)))
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
	if r.initialized {
		child.initializeAt(r.initializedAt)
	}
	state := &patternJoinRuntime{runtime: &child, plan: plan, schema: schema, tags: tags}
	r.patternJoinStates[node] = state
	return state, nil
}

func (p *patternJoinRuntime) materializeBatch(batch ResultBatch, now time.Time) ([]Event, error) {
	if p == nil {
		return nil, NewError(ErrorDependency, "pattern join runtime is nil")
	}
	result := make([]Event, 0, len(batch.New))
	for _, output := range batch.New {
		row, ok := output.Row()
		if !ok {
			return nil, fmt.Errorf("esper: pattern join source produced a non-row result")
		}
		values := make(map[string]any, len(p.tags))
		for _, tag := range p.tags {
			value := row.Get(tag)
			if value.IsPresent() {
				values[tag] = value.Any()
			}
		}
		materialized, err := newEvent(p.schema, values, now)
		if err != nil {
			return nil, err
		}
		result = append(result, materialized)
	}
	return result, nil
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
	materializedEvents, err := patternRuntime.materializeBatch(batch, now)
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
	for _, materialized := range materializedEvents {
		delta, addErr := r.addToWindow(spec, state, materialized, now)
		if addErr != nil {
			return eventDelta{}, addErr
		}
		result = mergeDelta(result, delta)
	}
	r.windows[node] = state
	return result, nil
}

type patternJoinTimeEvent struct {
	index int
	event Event
}

func (r *statementRuntime) advancePatternJoinSources(definition *joinDefinition, now time.Time) ([]patternJoinTimeEvent, error) {
	if r == nil || definition == nil {
		return nil, nil
	}
	sources := joinDefinitionSources(definition)
	result := make([]patternJoinTimeEvent, 0)
	for index, source := range sources {
		if source == nil || source.kind != streamPattern || source.pattern == nil || !patternContainsTimer(source.pattern.root) {
			continue
		}
		patternRuntime, err := r.patternJoinRuntimeFor(source)
		if err != nil {
			return nil, err
		}
		patternRuntime.runtime.ctx = r.ctx
		patternRuntime.runtime.variables = cloneValues(r.variables)
		batch := patternRuntime.runtime.patternTimeBatch(patternRuntime.plan, now)
		materialized, err := patternRuntime.materializeBatch(batch, now)
		if err != nil {
			return nil, err
		}
		for _, event := range materialized {
			result = append(result, patternJoinTimeEvent{index: index, event: event})
		}
	}
	return result, nil
}

func (r *statementRuntime) applyPatternJoinTime(definition *joinDefinition, events []patternJoinTimeEvent, now time.Time) (joinDelta, error) {
	if r == nil || definition == nil || len(events) == 0 {
		return joinDelta{}, nil
	}
	sources := joinDefinitionSources(definition)
	if len(r.joinState.sides) != len(sources) {
		r.joinState.sides = make([][]storedEvent, len(sources))
	}
	result := joinDelta{}
	for _, timed := range events {
		if timed.index < 0 || timed.index >= len(sources) {
			return joinDelta{}, fmt.Errorf("esper: pattern timer source index %d is out of range", timed.index)
		}
		if timed.index < len(definition.unidirectional) && definition.unidirectional[timed.index] {
			working := cloneJoinRuntimeState(r.joinState)
			working.sides[timed.index] = []storedEvent{{event: timed.event, receivedAt: now, lineageID: r.nextJoinLineageID()}}
			for _, tuple := range joinTuples(definition, working, now, r) {
				if timed.index < len(tuple) && tuple[timed.index].TypeName() != "" {
					result.newTuples = append(result.newTuples, tuple)
				}
			}
			result.unidirectionalTrigger = true
			continue
		}
		source := sources[timed.index]
		state := r.windows[source]
		if state == nil {
			state = &windowRuntimeState{}
		}
		spec := source.patternWindow
		if spec == nil {
			spec = KeepAll()
		}
		delta, err := r.addToWindow(spec, state, timed.event, now)
		if err != nil {
			return joinDelta{}, err
		}
		r.windows[source] = state
		removeStoredEvents(&r.joinState.sides[timed.index], delta.oldEvents)
		for _, event := range delta.newEvents {
			r.joinState.sides[timed.index] = append(r.joinState.sides[timed.index], storedEvent{
				event: event, receivedAt: now, lineageID: r.nextJoinLineageID(),
			})
		}
	}
	return joinDeltaWithPairs(result), nil
}
