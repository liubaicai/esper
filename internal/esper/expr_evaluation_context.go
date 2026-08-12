package esper

// ExpressionEvaluationContext is the immutable statement metadata visible to
// CurrentEvaluationContext. It is deliberately a small value object so Go
// rules can inspect fields directly without exposing Engine or Statement
// runtime state to expression callbacks.
type ExpressionEvaluationContext struct {
	RuntimeURI          string
	StatementName       string
	ContextPartitionID  int
	StatementUserObject any
}

// CurrentEvaluationContext returns the metadata for the statement currently
// evaluating the expression. Outside a deployed statement or context-aware
// projection, ContextPartitionID is normalized to -1, matching Esper's
// non-context evaluation contract.
func CurrentEvaluationContext() Expression[ExpressionEvaluationContext] {
	return makeExpr[ExpressionEvaluationContext]("current-evaluation-context", "current_evaluation_context()", nil, func(ctx EvalContext) Value {
		metadata := ctx.Metadata
		if !ctx.evaluationContextSet {
			metadata.ContextPartitionID = -1
		}
		return Present(metadata)
	})
}
