package esper

import (
	"fmt"
	"strings"
)

// ConcatOf is the explicit mixed-value form of Concat. String and numeric
// operands are converted to their stable text representation while Null and
// Missing operands propagate Null, matching Esper's concatenation operator.
func ConcatOf(values ...Expr) Expression[string] {
	return concatExpression("concat-of", values...)
}

func concatExpression(kind string, values ...Expr) Expression[string] {
	descriptions := make([]string, 0, len(values))
	children := make([]*exprNode, 0, len(values))
	node := &exprNode{kind: kind, typ: typeOf[string]()}
	for index, value := range values {
		descriptions = append(descriptions, expressionDescription(value))
		if value == nil || value.node() == nil {
			children = append(children, nil)
			if node.configurationError == "" {
				node.configurationError = fmt.Sprintf("concat operand %d is required", index)
			}
			continue
		}
		children = append(children, value.node())
		if node.configurationError == "" && !isLikeTextType(value.Type()) {
			node.configurationError = fmt.Sprintf("concat operand %d must be text-compatible, got %s", index, mathTypeDescription(value.Type()))
		}
	}
	if len(values) == 0 {
		node.configurationError = "concat requires at least one operand"
	}
	node.children = children
	node.description = "concat(" + strings.Join(descriptions, ",") + ")"
	return typedExpr[string]{n: node, fn: func(ctx EvalContext) Value {
		var builder strings.Builder
		for _, value := range values {
			if value == nil {
				return Null()
			}
			text, ok := likeTextValue(value.eval(ctx))
			if !ok {
				return Null()
			}
			builder.WriteString(text)
		}
		return Present(builder.String())
	}}
}
