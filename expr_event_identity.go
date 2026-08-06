package esper

import (
	"fmt"
	"reflect"
)

// EventIdentityEquals compares the runtime identity of two Event envelopes.
// It deliberately does not compare underlying field values: two separately
// ingested events with identical payloads are still different events.
func EventIdentityEquals(left, right Expr) Expression[bool] {
	description := fmt.Sprintf("event_identity_equals(%s,%s)", expressionDescription(left), expressionDescription(right))
	node := &exprNode{kind: "event-identity-equals", typ: typeOf[bool](), description: description, children: []*exprNode{nil, nil}}
	if left != nil {
		node.children[0] = left.node()
	}
	if right != nil {
		node.children[1] = right.node()
	}
	if left == nil || left.node() == nil {
		node.configurationError = "event identity comparison requires a left operand"
	} else if right == nil || right.node() == nil {
		node.configurationError = "event identity comparison requires a right operand"
	} else if !eventIdentityExpressionType(left.Type()) {
		node.configurationError = fmt.Sprintf("event identity comparison left operand must resolve to an event, got %s", mathTypeDescription(left.Type()))
	} else if !eventIdentityExpressionType(right.Type()) {
		node.configurationError = fmt.Sprintf("event identity comparison right operand must resolve to an event, got %s", mathTypeDescription(right.Type()))
	}
	return typedExpr[bool]{n: node, fn: func(ctx EvalContext) Value {
		if left == nil || right == nil {
			return Present(false)
		}
		leftEvent, leftOK := eventIdentityValue(left.eval(ctx))
		rightEvent, rightOK := eventIdentityValue(right.eval(ctx))
		if !leftOK || !rightOK || leftEvent.identity == nil || rightEvent.identity == nil {
			return Present(false)
		}
		return Present(leftEvent.identity == rightEvent.identity)
	}}
}

func eventIdentityExpressionType(typ reflect.Type) bool {
	if typ == nil || typ == typeOf[any]() {
		return true
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return false
		}
	}
	return typ == reflect.TypeOf(Event{}) || typ.Kind() == reflect.Interface
}

func eventIdentityValue(value Value) (Event, bool) {
	if !value.IsPresent() {
		return Event{}, false
	}
	switch event := value.Any().(type) {
	case Event:
		return event, true
	case *Event:
		if event == nil {
			return Event{}, false
		}
		return *event, true
	default:
		return Event{}, false
	}
}
