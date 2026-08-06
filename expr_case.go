package esper

import "strings"

// searchedCaseBranch is one condition/result pair in a searched CASE
// expression. The public builder keeps the result type generic while the
// expression tree retains both sides for field and subquery validation.
type searchedCaseBranch struct {
	condition Expr
	result    Expr
}

// SearchedCaseBuilder builds a CASE expression whose WHEN clauses are
// boolean expressions. Builders are mutable while being assembled, but Build
// snapshots the branch slices so a deployed expression is unaffected by
// later builder changes.
type SearchedCaseBuilder[T any] struct {
	branches []searchedCaseBranch
	elseExpr Expr
	elseSet  bool
	invalid  string
}

// Case starts an empty searched CASE builder. CaseWhen is the concise form
// when the first WHEN clause is already available. The result type is
// explicit for an empty builder because Go cannot infer a type parameter from
// a call with no arguments.
func Case[T any]() *SearchedCaseBuilder[T] {
	return &SearchedCaseBuilder[T]{}
}

// CaseWhen starts a searched CASE with its first condition/result pair. Go's
// type inference does not infer a type parameter nested in Expression[T], so
// callers provide the result type explicitly, for example CaseWhen[string].
func CaseWhen[T any](condition Expression[bool], result Expression[T]) *SearchedCaseBuilder[T] {
	return Case[T]().When(condition, result)
}

// When appends a condition/result pair. A condition that is Null or Missing
// at runtime is treated as not matched, matching Esper's three-valued CASE
// condition semantics.
func (b *SearchedCaseBuilder[T]) When(condition Expression[bool], result Expression[T]) *SearchedCaseBuilder[T] {
	if b == nil {
		return b
	}
	if b.elseSet {
		b.setError("case WHEN clauses must precede ELSE")
	}
	if condition == nil {
		b.setError("case WHEN condition is required")
	}
	if result == nil {
		b.setError("case THEN result is required")
	}
	b.branches = append(b.branches, searchedCaseBranch{condition: condition, result: result})
	return b
}

// Else terminates the searched CASE and returns the analyzable expression.
// Build can be used instead when an omitted ELSE should produce Null.
func (b *SearchedCaseBuilder[T]) Else(result Expression[T]) Expression[T] {
	if b == nil {
		return invalidCaseExpression[T]("case-when", "case-when(<nil>)", "case builder is nil")
	}
	if b.elseSet {
		b.setError("case ELSE clause can only be specified once")
	}
	if result == nil {
		b.setError("case ELSE result is required")
	}
	b.elseExpr = result
	b.elseSet = true
	return b.Build()
}

// Build returns the searched CASE expression. Without an ELSE, an unmatched
// branch evaluates to Null.
func (b *SearchedCaseBuilder[T]) Build() Expression[T] {
	if b == nil {
		return invalidCaseExpression[T]("case-when", "case-when(<nil>)", "case builder is nil")
	}
	if len(b.branches) == 0 {
		b.setError("case expression requires at least one WHEN clause")
	}

	branches := append([]searchedCaseBranch(nil), b.branches...)
	elseExpr := b.elseExpr
	invalid := b.invalid
	descriptionParts := make([]string, 0, len(branches)+1)
	children := make([]*exprNode, 0, len(branches)*2+1)
	for _, branch := range branches {
		conditionDescription := caseExpressionDescription(branch.condition)
		resultDescription := caseExpressionDescription(branch.result)
		descriptionParts = append(descriptionParts, "when "+conditionDescription+" then "+resultDescription)
		if node := caseExpressionNode(branch.condition); node != nil {
			children = append(children, node)
		}
		if node := caseExpressionNode(branch.result); node != nil {
			children = append(children, node)
		}
	}
	if elseExpr != nil {
		descriptionParts = append(descriptionParts, "else "+caseExpressionDescription(elseExpr))
		if node := caseExpressionNode(elseExpr); node != nil {
			children = append(children, node)
		}
	}
	description := "case " + strings.Join(descriptionParts, " ") + " end"
	node := &exprNode{kind: "case-when", typ: typeOf[T](), description: description, children: children, configurationError: invalid}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if invalid != "" {
			return Null()
		}
		for _, branch := range branches {
			if branch.condition == nil || branch.result == nil {
				continue
			}
			matched, ok := boolValue(branch.condition.eval(ctx))
			if ok && matched {
				return branch.result.eval(ctx)
			}
		}
		if elseExpr != nil {
			return elseExpr.eval(ctx)
		}
		return Null()
	}}
}

// SimpleCaseBuilder builds a CASE expression that evaluates one value and
// compares it with each WHEN value in declaration order. Match expressions
// intentionally use Expr instead of a second generic parameter: this keeps
// numeric and nullable comparison useful across Go's distinct numeric types
// while the result type remains statically checked.
type SimpleCaseBuilder[T any] struct {
	value    Expr
	branches []simpleCaseBranch
	elseExpr Expr
	elseSet  bool
	invalid  string
}

type simpleCaseBranch struct {
	match  Expr
	result Expr
}

// CaseValue starts a simple CASE with its first match/result pair. The result
// type is explicit because it is carried by the Expression[T] result, for
// example:
//
//	CaseValue[string](Field[Event, int]("code"), Literal(1), Literal("one")).
//		When(Literal(2), Literal("two")).
//		Else(Literal("other"))
func CaseValue[T any](value Expr, match Expr, result Expression[T]) *SimpleCaseBuilder[T] {
	b := &SimpleCaseBuilder[T]{value: value}
	if value == nil {
		b.setError("case value expression is required")
	}
	return b.When(match, result)
}

// When appends a simple CASE match/result pair. A Null WHEN value matches a
// Null case value; Missing remains distinct and does not match anything.
func (b *SimpleCaseBuilder[T]) When(match Expr, result Expression[T]) *SimpleCaseBuilder[T] {
	if b == nil {
		return b
	}
	if b.elseSet {
		b.setError("case WHEN clauses must precede ELSE")
	}
	if match == nil {
		b.setError("case WHEN value is required")
	}
	if result == nil {
		b.setError("case THEN result is required")
	}
	b.branches = append(b.branches, simpleCaseBranch{match: match, result: result})
	return b
}

// Else terminates the simple CASE and returns the analyzable expression.
func (b *SimpleCaseBuilder[T]) Else(result Expression[T]) Expression[T] {
	if b == nil {
		return invalidCaseExpression[T]("case-value", "case-value(<nil>)", "case builder is nil")
	}
	if b.elseSet {
		b.setError("case ELSE clause can only be specified once")
	}
	if result == nil {
		b.setError("case ELSE result is required")
	}
	b.elseExpr = result
	b.elseSet = true
	return b.Build()
}

// Build returns the simple CASE expression. Without an ELSE, an unmatched
// value evaluates to Null.
func (b *SimpleCaseBuilder[T]) Build() Expression[T] {
	if b == nil {
		return invalidCaseExpression[T]("case-value", "case-value(<nil>)", "case builder is nil")
	}
	if b.value == nil {
		b.setError("case value expression is required")
	}
	if len(b.branches) == 0 {
		b.setError("case expression requires at least one WHEN clause")
	}

	branches := append([]simpleCaseBranch(nil), b.branches...)
	value := b.value
	elseExpr := b.elseExpr
	invalid := b.invalid
	descriptionParts := make([]string, 0, len(branches)+1)
	children := make([]*exprNode, 0, len(branches)*2+2)
	if value != nil {
		descriptionParts = append(descriptionParts, caseExpressionDescription(value))
		if node := caseExpressionNode(value); node != nil {
			children = append(children, node)
		}
	} else {
		descriptionParts = append(descriptionParts, "<nil>")
	}
	for _, branch := range branches {
		descriptionParts = append(descriptionParts, "when "+caseExpressionDescription(branch.match)+" then "+caseExpressionDescription(branch.result))
		if node := caseExpressionNode(branch.match); node != nil {
			children = append(children, node)
		}
		if node := caseExpressionNode(branch.result); node != nil {
			children = append(children, node)
		}
	}
	if elseExpr != nil {
		descriptionParts = append(descriptionParts, "else "+caseExpressionDescription(elseExpr))
		if node := caseExpressionNode(elseExpr); node != nil {
			children = append(children, node)
		}
	}
	description := "case " + strings.Join(descriptionParts, " ") + " end"
	node := &exprNode{kind: "case-value", typ: typeOf[T](), description: description, children: children, configurationError: invalid}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		if invalid != "" {
			return Null()
		}
		if value == nil {
			return Null()
		}
		caseValue := value.eval(ctx)
		for _, branch := range branches {
			if branch.match == nil || branch.result == nil {
				continue
			}
			if caseValuesMatch(caseValue, branch.match.eval(ctx)) {
				return branch.result.eval(ctx)
			}
		}
		if elseExpr != nil {
			return elseExpr.eval(ctx)
		}
		return Null()
	}}
}

func (b *SearchedCaseBuilder[T]) setError(message string) {
	if b.invalid == "" {
		b.invalid = message
	}
}

func (b *SimpleCaseBuilder[T]) setError(message string) {
	if b.invalid == "" {
		b.invalid = message
	}
}

func caseExpressionDescription(expression Expr) string {
	if expression == nil {
		return "<nil>"
	}
	return expression.Description()
}

func caseExpressionNode(expression Expr) *exprNode {
	if expression == nil {
		return nil
	}
	return expression.node()
}

func caseValuesMatch(left, right Value) bool {
	if left.IsNull() || right.IsNull() {
		return left.IsNull() && right.IsNull()
	}
	if !left.IsPresent() || !right.IsPresent() {
		return false
	}
	matched, ok := boolValue(EqualValues(left, right))
	return ok && matched
}

func invalidCaseExpression[T any](kind, description, message string) Expression[T] {
	return typedExpr[T]{
		n:  &exprNode{kind: kind, typ: typeOf[T](), description: description, configurationError: message},
		fn: func(EvalContext) Value { return Null() },
	}
}
