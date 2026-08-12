package esper

import "reflect"

// ExpressionNodeModel is a detached, read-only description of one fluent
// expression node. Kind names identify the constructor family (for example
// "add", "multiply", "and" or "or") without exposing the runtime evaluator
// closure or requiring an EPL text round-trip.
type ExpressionNodeModel struct {
	kind        string
	description string
	typ         reflect.Type
	children    []ExpressionNodeModel
}

// InspectExpression returns an immutable snapshot of the complete typed
// expression tree. A nil expression returns an invalid zero model.
func InspectExpression(expression Expr) ExpressionNodeModel {
	if expression == nil {
		return ExpressionNodeModel{}
	}
	return expressionNodeModel(expression.node())
}

func expressionNodeModel(node *exprNode) ExpressionNodeModel {
	if node == nil {
		return ExpressionNodeModel{}
	}
	children := make([]ExpressionNodeModel, len(node.children))
	for index, child := range node.children {
		children[index] = expressionNodeModel(child)
	}
	return ExpressionNodeModel{
		kind:        node.kind,
		description: node.description,
		typ:         node.typ,
		children:    children,
	}
}

// Valid reports whether the model describes a real expression node.
func (m ExpressionNodeModel) Valid() bool { return m.kind != "" }

// Kind identifies the fluent expression constructor family.
func (m ExpressionNodeModel) Kind() string { return m.kind }

// Description returns the stable typed expression description used by the
// compiler. It is descriptive metadata, not accepted as rule input.
func (m ExpressionNodeModel) Description() string { return m.description }

// Type returns the expression's declared Go result type.
func (m ExpressionNodeModel) Type() reflect.Type { return m.typ }

// Children returns a detached recursive snapshot in operand order.
func (m ExpressionNodeModel) Children() []ExpressionNodeModel {
	children := make([]ExpressionNodeModel, len(m.children))
	for index, child := range m.children {
		children[index] = cloneExpressionNodeModel(child)
	}
	return children
}

func cloneExpressionNodeModel(model ExpressionNodeModel) ExpressionNodeModel {
	model.children = model.Children()
	return model
}

// PatternNodeModel is a detached, read-only description of one fluent CEP
// node. It is the Go-style object-model view for pattern precedence and tree
// inspection; rules continue to be constructed with PatternStream methods.
type PatternNodeModel struct {
	kind        string
	description string
	tag         string
	predicate   ExpressionNodeModel
	children    []PatternNodeModel
}

// InspectPattern returns the effective pattern root, including Every or
// EveryDistinct wrappers stored as definition-level fluent modifiers.
func InspectPattern(pattern PatternStream) PatternNodeModel {
	if pattern.def == nil {
		return PatternNodeModel{}
	}
	return patternNodeModel(patternBranchRoot(pattern.def))
}

// Model returns a detached description of this fluent pattern.
func (p PatternStream) Model() PatternNodeModel { return InspectPattern(p) }

// Model returns a detached description of this pattern query.
func (p PatternQuery) Model() PatternNodeModel {
	if p.definition == nil {
		return PatternNodeModel{}
	}
	return patternNodeModel(patternBranchRoot(p.definition))
}

// PatternModel returns the typed pattern tree carried by this Query.
func (q Query) PatternModel() (PatternNodeModel, bool) {
	if q.pattern == nil {
		return PatternNodeModel{}, false
	}
	return patternNodeModel(patternBranchRoot(q.pattern)), true
}

func patternNodeModel(node *patternNode) PatternNodeModel {
	if node == nil {
		return PatternNodeModel{}
	}
	model := PatternNodeModel{
		kind:        publicPatternNodeKind(node.kind),
		description: node.description(),
		tag:         node.tag,
	}
	if node.predicate != nil {
		model.predicate = InspectExpression(node.predicate)
	}
	appendChild := func(child *patternNode) {
		if child != nil {
			model.children = append(model.children, patternNodeModel(child))
		}
	}
	switch node.kind {
	case patternSequenceNode, patternAndNode, patternOrNode:
		appendChild(node.left)
		appendChild(node.right)
	case patternMatchUntilNode, patternUntilNode:
		appendChild(node.child)
		appendChild(node.right)
	case patternNotNode, patternEveryNode, patternWithinNode:
		appendChild(node.child)
	case patternGuardWhileNode:
		appendChild(node.child)
	}
	return model
}

func publicPatternNodeKind(kind patternNodeKind) string {
	switch kind {
	case patternEventNode:
		return "event"
	case patternSequenceNode:
		return "followed-by"
	case patternAndNode:
		return "and"
	case patternOrNode:
		return "or"
	case patternNotNode:
		return "not"
	case patternMatchUntilNode:
		return "match-until"
	case patternUntilNode:
		return "until"
	case patternEveryNode:
		return "every"
	case patternWithinNode:
		return "within"
	case patternGuardWhileNode:
		return "while-guard"
	case patternTimerIntervalNode:
		return "timer-interval"
	case patternTimerAtNode:
		return "timer-at"
	case patternTimerScheduleNode:
		return "timer-schedule"
	case patternTimerCronNode:
		return "timer-cron"
	default:
		return "unknown"
	}
}

// Valid reports whether the model describes a real pattern node.
func (m PatternNodeModel) Valid() bool { return m.kind != "" }

// Kind identifies the fluent pattern combinator at this node.
func (m PatternNodeModel) Kind() string { return m.kind }

// Description returns the compiler's stable typed pattern description.
func (m PatternNodeModel) Description() string { return m.description }

// Tag returns the event tag for an event node and an empty string otherwise.
func (m PatternNodeModel) Tag() string { return m.tag }

// Predicate returns the event-filter expression for an event node.
func (m PatternNodeModel) Predicate() (ExpressionNodeModel, bool) {
	if !m.predicate.Valid() {
		return ExpressionNodeModel{}, false
	}
	return cloneExpressionNodeModel(m.predicate), true
}

// Children returns a detached recursive snapshot in evaluation order.
func (m PatternNodeModel) Children() []PatternNodeModel {
	children := make([]PatternNodeModel, len(m.children))
	for index, child := range m.children {
		children[index] = clonePatternNodeModel(child)
	}
	return children
}

func clonePatternNodeModel(model PatternNodeModel) PatternNodeModel {
	model.predicate = cloneExpressionNodeModel(model.predicate)
	model.children = model.Children()
	return model
}
