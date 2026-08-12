package esper

import (
	"fmt"
	"math/big"
	"reflect"
)

// PrevOf evaluates a view-relative previous expression with an analyzable
// runtime offset. It is the fluent counterpart of Esper's prev(offsetExpr,
// value), including dynamic integer fields and parameters.
func PrevOf[V any](offset Expr, expression Expression[V]) Expression[V] {
	return dynamicPreviousExpression[V]("prev-dynamic", offset, expression, false)
}

func dynamicPreviousExpression[V any](kind string, offset Expr, expression Expression[V], prior bool) Expression[V] {
	description := fmt.Sprintf("prev(%s,%s)", expressionDescription(offset), expressionDescription(expression))
	node := &exprNode{kind: kind, typ: typeOf[V](), description: description, previousOffset: -1, children: []*exprNode{nil, nil}}
	if offset != nil {
		node.children[0] = offset.node()
	}
	if expression != nil {
		node.children[1] = expression.node()
	}
	if offset == nil || offset.node() == nil {
		node.configurationError = "dynamic previous offset expression is required"
	} else if !previousOffsetExpressionType(offset.Type()) {
		node.configurationError = fmt.Sprintf("dynamic previous offset must be integral, got %s", mathTypeDescription(offset.Type()))
	} else if expression == nil || expression.node() == nil {
		node.configurationError = "dynamic previous value expression is required"
	}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		if offset == nil || expression == nil {
			return Null()
		}
		requested, ok := previousOffsetValue(offset.eval(ctx))
		if !ok {
			return Null()
		}
		return evaluatePreviousOffset[V](ctx, requested, expression, prior)
	}}
}

func previousOffsetExpressionType(typ reflect.Type) bool {
	if typ == nil || typ == typeOf[any]() {
		return true
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return false
		}
	}
	if typ.Kind() == reflect.Interface || typ == typeOf[big.Int]() {
		return true
	}
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func previousOffsetValue(value Value) (int, bool) {
	number, ok := enumRatFromValue(value)
	if !ok || !number.IsInt() || number.Sign() < 0 || !number.Num().IsInt64() {
		return 0, false
	}
	requested := number.Num().Int64()
	maxInt := int64(^uint(0) >> 1)
	if requested > maxInt {
		return 0, false
	}
	return int(requested), true
}

func evaluatePreviousOffset[V any](ctx EvalContext, offset int, expression Expression[V], prior bool) Value {
	history := ctx.PreviousHistory
	windowAccess := ctx.PreviousWindowAccess && !prior
	if prior && (ctx.PriorHistorySet || ctx.PriorHistory != nil) {
		history = ctx.PriorHistory
		windowAccess = false
	} else if history == nil && !ctx.PreviousWindowAccess {
		history = ctx.History
	}
	if offset < 0 || len(history) == 0 {
		return Null()
	}
	if windowAccess {
		if offset >= len(history) {
			return Null()
		}
		nested := ctx
		nested.History = history
		nested.PreviousHistory = history
		nested.PreviousWindowAccess = true
		return evaluatePreviousAt[V](expression, nested, offset)
	}
	index := len(history) - 1 - offset
	if prior {
		index--
	}
	if index < 0 || index >= len(history) {
		return Null()
	}
	nested := ctx
	nested.Event = history[index]
	nested.History = append([]Event(nil), history[:index+1]...)
	nested.PreviousHistory = append([]Event(nil), history[:index+1]...)
	nested.PreviousWindowAccess = ctx.PreviousWindowAccess
	nested.PriorHistory = append([]Event(nil), ctx.PriorHistory...)
	nested.PriorHistorySet = ctx.PriorHistorySet
	var tags []string
	expression.node().referencedTags(&tags)
	if len(tags) > 0 || len(ctx.PreviousTagEvents) > 0 {
		nested.PreviousTagEvents = make(map[string]Event, len(ctx.PreviousTagEvents)+len(tags))
		for tag, event := range ctx.PreviousTagEvents {
			nested.PreviousTagEvents[tag] = event
		}
		for _, tag := range tags {
			nested.PreviousTagEvents[tag] = history[index]
		}
	}
	return expression.eval(nested)
}
