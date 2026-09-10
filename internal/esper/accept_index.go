package esper

// This file implements event-type-level candidate pruning for the dispatch
// loops (the Go counterpart of Esper's EventTypeIndex stage). A statement is
// skipped for one event only when it is provable that the statement's own
// filter service decision would be "cannot accept": the descriptor below
// mirrors sourceNodeAcceptsEvent's structural acceptance exactly, and the
// skip is additionally disabled for any statement whose per-event processing
// has side effects beyond its own output (context lifecycle, subquery feeds,
// output-policy state, update-istream processing).
//
// Event side, the set of names a plain source can match is computed once per
// event:
//   - a routed event (StreamType != TypeName) is accepted by exact
//     source-name equality, plus the exact type name for named-window and
//     contained sources (which check both before the routed early-exit);
//     no parent closure applies to routed events;
//   - otherwise the set is the event type name plus its transitive
//     parentNames closure, mirroring Environment.acceptsEventType's walk.
//
// The per-statement dispatch order, unmatched accounting, metric and audit
// records are unchanged: skipped statements would have contributed
// accepted=false and changed=false, and every metric sample, counter and
// audit record in processStatementWithMetricsLocked/auditStatementProcess
// is gated on accepted or changed.

// statementAcceptIndex is the deployment-time acceptance view of one
// statement. It is computed once in prepareStatementLocked from immutable
// query shape; statements with any non-prunable feature keep prunable=false
// and are always visited exactly as before.
type statementAcceptIndex struct {
	prunable bool
	// names holds every source name a plain acceptance check can match for
	// this statement: the declared stream source name plus the resolved
	// source schema name (equal for ordinary sources).
	names []string
}

// newStatementAcceptIndex computes the descriptor for one query. registryNil
// reports whether the statement's runtime subquery registry is absent (no
// subquery anywhere in the query); statements with a registry feed subquery
// windows for every event and must not be pruned.
func newStatementAcceptIndex(env *Environment, query Query, registryNil bool) statementAcceptIndex {
	index := statementAcceptIndex{prunable: true}
	if env == nil || query.env == nil {
		index.prunable = false
		return index
	}
	if query.contextName != "" || query.updateStream != nil || !registryNil {
		// Context lifecycle routing, update-istream processing and subquery
		// feeds observe every event through Statement.process.
		index.prunable = false
		return index
	}
	switch {
	case query.join != nil:
		for _, source := range joinDefinitionSources(query.join) {
			acceptIndexNode(env, source, &index)
		}
	case query.pattern != nil:
		for _, source := range patternDefinitionInputs(query.pattern) {
			acceptIndexNode(env, source, &index)
		}
	default:
		input := query.input
		if query.aggregate != nil {
			input = query.aggregate.input
		}
		if query.rowRecog != nil {
			input = query.rowRecog.input
		}
		acceptIndexNode(env, input, &index)
	}
	if !index.prunable || len(index.names) == 0 {
		// Anything that failed the structural mirroring, or a query with no
		// resolvable named source, stays on the full path.
		index.prunable = false
		index.names = nil
	}
	return index
}

// acceptIndexNode mirrors sourceNodeAcceptsEvent for one source node,
// recording the acceptable names. Any shape whose acceptance cannot be
// expressed as a name set marks the whole statement not prunable.
func acceptIndexNode(env *Environment, node *streamNode, index *statementAcceptIndex) {
	if node == nil {
		index.prunable = false
		return
	}
	source, err := sourceNode(node)
	if err != nil || source == nil {
		index.prunable = false
		return
	}
	switch {
	case source.kind == streamDerived:
		// Acceptance recurses into the derived stream's own source.
		acceptIndexNode(env, source.input, index)
		return
	case source.kind == streamPattern:
		for _, input := range patternDefinitionInputs(source.pattern) {
			acceptIndexNode(env, input, index)
		}
		return
	case source.kind == streamContained,
		source.kind == streamHistorical,
		source.kind == streamMethod:
		// Contained sources accept both parent and expanded child
		// representations; historical and method sources accept every event.
		index.prunable = false
		return
	case source.kind == streamNamedWindow || source.kind == streamTable:
		// Named-window and table sources match by exact stream or type name
		// equality only.
		index.addName(source.sourceName)
		return
	case source.kind == streamSource:
		schema, schemaErr := env.sourceSchema(source)
		if schemaErr != nil {
			index.prunable = false
			return
		}
		if schema.kind == SchemaVariant {
			// Variant membership (StreamType equality or member resolution)
			// is not a name-set decision.
			index.prunable = false
			return
		}
		// A plain source matches by declared stream name, schema name, or a
		// parent-name walk that the event-side set already covers.
		index.addName(source.sourceName)
		index.addName(schema.Name())
		return
	default:
		index.prunable = false
	}
}

// addName records one acceptable source name. prunable is one-way: a shape
// that cannot be expressed as names clears it and no later plain source may
// restore it.
func (a *statementAcceptIndex) addName(name string) {
	if name == "" {
		a.prunable = false
		return
	}
	for _, existing := range a.names {
		if existing == name {
			return
		}
	}
	a.names = append(a.names, name)
}

// mayAccept reports whether any source name of the statement is in the
// event's accepted-name set. Callers must only consult it when prunable is
// true and the event set is non-nil.
func (a statementAcceptIndex) mayAccept(acceptedNames map[string]struct{}) bool {
	for _, name := range a.names {
		if _, ok := acceptedNames[name]; ok {
			return true
		}
	}
	return false
}

// eventAcceptedTypeNames computes the per-event set of names that
// sourceNodeAcceptsEvent's plain acceptance can match, as described in the
// file comment. A nil result disables pruning for the event.
func eventAcceptedTypeNames(env *Environment, event Event) map[string]struct{} {
	if env == nil || !event.schema.valid() {
		return nil
	}
	typeName := event.TypeName()
	if typeName == "" {
		return nil
	}
	if streamType := event.StreamType(); streamType != typeName {
		// Routed event. sourceNodeAcceptsEvent first checks the named-window
		// and contained kinds, which accept by StreamType OR TypeName, then
		// returns exact stream-name equality for every other kind. The set
		// therefore carries both names and never the parent closure, which
		// routed events do not use.
		names := map[string]struct{}{}
		if streamType != "" {
			names[streamType] = struct{}{}
		}
		if typeName != "" {
			names[typeName] = struct{}{}
		}
		if len(names) == 0 {
			return nil
		}
		return names
	}
	names := map[string]struct{}{typeName: {}}
	env.mu.RLock()
	defer env.mu.RUnlock()
	// Mirror acceptsEventType's visit: walk the event type's transitive
	// parentNames; an unregistered name simply extends no further.
	var visit func(name string)
	visit = func(name string) {
		schema, ok := env.schemas[name]
		if !ok {
			return
		}
		for _, parent := range schema.parentNames {
			if _, seen := names[parent]; seen {
				continue
			}
			names[parent] = struct{}{}
			visit(parent)
		}
	}
	visit(typeName)
	return names
}

// eventAcceptedTypeNamesCached returns the accepted-name set for an event's
// type, computed once per (type name, stream type) pair. The set derives from
// the event schema's immutable parent chain and the routed pair, so the cache
// never stales; a nil result (pruning disabled for that event) is not cached.
// Callers must hold e.mu.
func (e *Engine) eventAcceptedTypeNamesCached(event Event) map[string]struct{} {
	typeName := event.TypeName()
	if typeName == "" {
		return nil
	}
	key := typeName
	if streamType := event.StreamType(); streamType != typeName {
		if streamType == "" {
			return nil
		}
		// Routed sets carry both names, so the key must carry both: two
		// routed events can share a stream type while naming different
		// member types.
		key = typeName + "\x00" + streamType
	}
	if e.acceptedNameCache == nil {
		e.acceptedNameCache = make(map[string]map[string]struct{})
	} else if cached, ok := e.acceptedNameCache[key]; ok {
		return cached
	}
	names := eventAcceptedTypeNames(e.env, event)
	if names != nil {
		e.acceptedNameCache[key] = names
	}
	return names
}
