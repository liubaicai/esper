package esper

import "time"

// statelessExecutionPlan is the precomputed execution plan for a statement
// whose observable result is exactly the accepted input event: one static
// struct source, a chain of pure filter predicates, and the wildcard output.
// Eligible statements reuse the filter result the dispatch loop already
// computed and emit the single row directly instead of entering the generic
// insert/delta/projection/output pipeline. Everything outside this narrow
// shape keeps the generic pipeline unchanged.
type statelessExecutionPlan struct {
	input *streamNode
	// schema is the source schema the plan was proven against. Property reads
	// are only proven free of user code (registered getters, dynamic methods)
	// for that exact schema, so the plan only applies to events carrying it.
	schema Schema
	// compiled holds the specialized evaluation of every predicate in the
	// chain when each node is in the compiled set (stateless_compile.go);
	// nil keeps the generic closure evaluation. Both forms evaluate the same
	// operations over the same Values.
	compiled []statelessValueFn
}

// appliesTo reports whether an event carries the schema this plan was proven
// against. A different schema can resolve the same property through a
// registered getter or a dynamic method call, so such events keep the generic
// pipeline and its evaluation behavior.
func (p *statelessExecutionPlan) appliesTo(event Event) bool {
	identity := p.schema.identity
	return identity != nil && event.schema.identity == identity
}

// statelessPlanLocked resolves and caches the plan for one statement. The
// plan depends only on the immutable query and deployment-time runtime state,
// so it is resolved once and reused for the statement's whole lifetime.
func (s *Statement) statelessPlanLocked() *statelessExecutionPlan {
	if s.statelessPlanResolved {
		return s.statelessPlan
	}
	s.statelessPlanResolved = true
	s.statelessPlan = newStatelessExecutionPlan(s)
	return s.statelessPlan
}

// newStatelessExecutionPlan returns the plan when every feature of the
// statement is proven to reduce to "accept the event, emit one wildcard row",
// and nil otherwise. The proof is structural: any feature that consults
// runtime state, other scopes, retained history, engine services or user code
// makes the statement ineligible and it keeps the generic semantics.
func newStatelessExecutionPlan(s *Statement) *statelessExecutionPlan {
	if s == nil || s.engine == nil {
		return nil
	}
	query := s.plan.query
	switch {
	case query.aggregate != nil, query.join != nil, query.pattern != nil, query.rowRecog != nil,
		query.trigger != nil, query.onDemand != nil, query.updateStream != nil:
		return nil
	case query.sourceLess, query.contextName != "", query.routeTarget != "", query.tableTarget != "",
		query.namedWindowDirect, query.distinct, query.iterableUnbound:
		return nil
	case query.sink != nil, query.eventPrecedence != nil:
		return nil
	case query.joinWhere != nil, query.joinHaving != nil, query.patternWhere != nil:
		return nil
	case len(query.selections) != 0, len(query.joinSelections) != 0, len(query.patternSelections) != 0:
		// A projection would need per-row expression evaluation and its own
		// result schema; the fast path only serves the wildcard row.
		return nil
	case query.selector != SelectIStream:
		// Remove-stream and multi-stream selectors observe retained rows.
		return nil
	case len(query.orderBy) != 0, query.limit != 0, query.offset != 0,
		query.limitExpr != nil, query.offsetExpr != nil, query.limitExprSet, query.offsetExprSet:
		return nil
	case s.runtime.outputState != nil:
		// Any output policy buffers, throttles or reschedules the rows.
		return nil
	case len(querySubqueryDefinitions(query)) != 0:
		// A statement carrying subqueries exchanges state with the registry.
		return nil
	case query.env == nil || query.input == nil:
		return nil
	}
	node := query.input
	var predicates []Expr
	for node != nil {
		switch node.kind {
		case streamFilter:
			if node.predicate == nil || !statelessPureExpression(node.predicate.node()) {
				return nil
			}
			predicates = append(predicates, node.predicate)
			node = node.input
		case streamSource:
			schema, ok := statelessStructSource(query.env, node)
			if !ok {
				return nil
			}
			if len(schema.getters) != 0 {
				// A registered getter or JavaBean accessor performs a property
				// read by invoking user code; this plan must not change how
				// often that code runs.
				return nil
			}
			for _, predicate := range predicates {
				if !statelessFieldsAreStatic(schema, predicate) {
					return nil
				}
			}
			return &statelessExecutionPlan{
				input:    query.input,
				schema:   schema,
				compiled: compileStatelessChain(schema, predicates),
			}
		default:
			// Windows, derived streams, contained sources, named windows,
			// tables and method/historical sources all retain or poll state.
			return nil
		}
	}
	return nil
}

// statelessFieldsAreStatic reports whether every property a predicate reads is
// a plain name declared by the root schema. A name with path syntax traverses
// nested schemas (whose getters are not inspected here) and an undeclared name
// is resolved dynamically at runtime, which can invoke a matching Go method.
func statelessFieldsAreStatic(schema Schema, predicate Expr) bool {
	node := predicate.node()
	if node == nil {
		return true
	}
	var fields []string
	node.referencedFields(&fields)
	for _, field := range fields {
		if !isPlainPropertyName(field) {
			return false
		}
		if _, _, err := schema.lookupField(field); err != nil {
			return false
		}
	}
	return true
}

// statelessStructSource returns the source schema when the node is exactly one
// registered struct-typed event stream, which is the only source whose
// filtering is a pure property read over the event itself.
func statelessStructSource(env *Environment, node *streamNode) (Schema, bool) {
	if env == nil || node == nil || node.kind != streamSource {
		return Schema{}, false
	}
	schema, err := env.sourceSchema(node)
	if err != nil || !schema.valid() {
		return Schema{}, false
	}
	if schema.Kind() != SchemaStruct {
		return Schema{}, false
	}
	return schema, true
}

// statelessPureExpression reports whether every node of a predicate tree is
// evaluated from the event value and literal operands alone. The allowed
// kinds are listed explicitly so a new expression kind stays on the generic
// path until it is reviewed and added here.
func statelessPureExpression(node *exprNode) bool {
	if node == nil {
		return true
	}
	if node.subquery != nil || node.multiMatch != nil {
		return false
	}
	if node.variableName != "" || node.parameterName != "" || node.tagName != "" ||
		node.joinSource != 0 || node.containedParentLevels != 0 || node.previousOffset != 0 ||
		node.pluginName != "" || node.pluginFactory != nil || node.scriptName != "" ||
		node.methodName != "" || node.expressionName != "" ||
		node.enumPluginName != "" || node.dateTimePluginName != "" ||
		node.aggregateMultiPluginName != "" || node.initialTarget {
		return false
	}
	if node.kind == "udf" {
		if !node.pureBuiltin {
			return false
		}
	} else if !statelessPureKinds[node.kind] {
		return false
	}
	for _, child := range node.children {
		if !statelessPureExpression(child) {
			return false
		}
	}
	for _, argument := range node.expressionArguments {
		if !statelessPureExpression(argument) {
			return false
		}
	}
	return statelessPureExpression(node.expressionBody)
}

// statelessPureKinds are the expression kinds that read only the event,
// literal operands and their children. Stateful kinds (variables, parameters,
// tags, pattern/join scopes, event history, the clock, output counters),
// engine services, subqueries and user code are absent from this list. Field
// reads additionally require the source schema to declare the property and to
// have no registered getters, so no property read can invoke user code.
var statelessPureKinds = map[string]bool{
	"field": true, "literal": true, "null": true,
	"and": true, "or": true, "not": true,
	"eq": true, "neq": true, "gt": true, "gte": true, "lt": true, "lte": true,
	"exact-lt": true, "exact-lte": true, "exact-gt": true, "exact-gte": true,
	"in": true, "between": true, "between-of": true,
	"contains": true, "starts-with": true, "ends-with": true, "like": true, "regexp": true,
	"is-null": true, "is-missing": true, "instance-of": true, "type-of": true,
	"if": true, "coalesce": true,
	"add": true, "subtract": true, "multiply": true, "divide": true, "modulo": true,
	"negate": true, "abs": true, "signum": true,
	"map-at": true, "array-at": true,
	"enum-any-of": true, "enum-all-of": true,
	"enum-element": true, "enum-index": true, "enum-size": true,
}

// processStatelessEvent emits the single wildcard row for an accepted event
// and nothing for a rejected one. When the dispatch loop already computed the
// filter result for unmatched-event or metric/audit accounting, that result
// is authoritative and the predicate is not evaluated a second time.
func (s *Statement) processStatelessEvent(plan *statelessExecutionPlan, event Event, now time.Time, variables map[string]Value, accepted bool, acceptedKnown bool) (ResultBatch, bool) {
	matched := accepted
	if !acceptedKnown {
		if plan.compiled != nil && plan.appliesTo(event) {
			// The compiled chain mirrors the generic filter evaluation for
			// events carrying the proven schema; it reads no variables.
			matched = statelessCompiledMatch(plan.compiled, event)
		} else {
			scope := variablesWithEngineLockState(statementVariables(variables, s.parameters), s.engine, true)
			matched = sourceNodeMatchesEventFilter(s.engine.env, plan.input, event, now, scope, s.engine)
		}
	}
	if !matched {
		// The generic pipeline reports an empty counted batch for a rejected
		// event; keep the same shape so audit and metric consumers see no
		// difference.
		return ResultBatch{Time: now, outputCountsSet: true, New: []Result{}}, false
	}
	batch := ResultBatch{
		Time:            now,
		New:             []Result{resultEvent(event)},
		outputCountsSet: true,
		outputInserted:  1,
	}
	batch.Sequence = s.runtime.seq.Add(1)
	return batch, true
}
