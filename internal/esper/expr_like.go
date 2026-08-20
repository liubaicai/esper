package esper

import (
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// LikeOf is the mixed-value form of Like. Numeric operands are converted to
// their Java-compatible text form before SQL-like wildcard matching.
func LikeOf(value, pattern Expr) Expression[bool] {
	return likeExpression("like-of", value, pattern, nil)
}

// LikeWithEscape adds an explicit single-rune escape expression to a typed
// LIKE operation. A caller can use LikeOfWithEscape when the operands are
// numeric or dynamically typed.
func LikeWithEscape(value, pattern, escape Expression[string]) Expression[bool] {
	return likeExpression("like-escape", value, pattern, escape)
}

// LikeOfWithEscape is the mixed-value counterpart of LikeWithEscape.
func LikeOfWithEscape(value, pattern, escape Expr) Expression[bool] {
	return likeExpression("like-escape-of", value, pattern, escape)
}

// RegexpMatchOf is the mixed-value form of RegexpMatch. It accepts strings
// and numeric values while keeping the regular-expression pattern visible in
// the expression tree.
func RegexpMatchOf(value, pattern Expr) Expression[bool] {
	return regexpExpression(value, pattern)
}

func likeExpression(kind string, value, pattern, escape Expr) Expression[bool] {
	description := "like(<nil>)"
	if value != nil && pattern != nil {
		description = "(" + value.Description() + " like " + pattern.Description()
		if escape != nil {
			description += " escape " + escape.Description()
		}
		description += ")"
	}
	children := make([]*exprNode, 0, 3)
	appendExpressionChild := func(expression Expr) {
		if expression == nil {
			children = append(children, nil)
			return
		}
		children = append(children, expression.node())
	}
	appendExpressionChild(value)
	appendExpressionChild(pattern)
	if escape != nil {
		appendExpressionChild(escape)
	}
	node := &exprNode{kind: kind, typ: typeOf[bool](), description: description, children: children}
	if value == nil || value.node() == nil {
		node.configurationError = "like value expression is required"
	} else if pattern == nil || pattern.node() == nil {
		node.configurationError = "like pattern expression is required"
	} else if !isLikeTextType(value.Type()) {
		node.configurationError = fmt.Sprintf("like value must be text-compatible, got %s", mathTypeDescription(value.Type()))
	} else if !isLikeTextType(pattern.Type()) {
		node.configurationError = fmt.Sprintf("like pattern must be text-compatible, got %s", mathTypeDescription(pattern.Type()))
	} else if escape != nil && (escape.node() == nil || !isLikeTextType(escape.Type())) {
		node.configurationError = fmt.Sprintf("like escape must be text-compatible, got %s", mathTypeDescription(escape.Type()))
	} else if escape != nil && escape.node().kind == "null" {
		node.configurationError = "like escape expression must not be null"
	}
	return typedExpr[bool]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil || pattern == nil {
			return Null()
		}
		left, leftOK := likeTextValue(value.eval(ctx))
		patternText, patternOK := likeTextValue(pattern.eval(ctx))
		if !leftOK || !patternOK {
			return Null()
		}
		var expression string
		var ok bool
		if escape == nil {
			expression, ok = patternToRegexp(patternText), true
		} else {
			escapeText, escapeOK := likeTextValue(escape.eval(ctx))
			if !escapeOK {
				return Null()
			}
			expression, ok = patternToRegexpWithEscape(patternText, escapeText)
		}
		if !ok {
			return Null()
		}
		compiled, err := regexp.Compile(likePattern(expression))
		if err != nil {
			return Null()
		}
		return Present(compiled.MatchString(left))
	}}
}

func regexpExpression(value, pattern Expr) Expression[bool] {
	description := "regexp(<nil>)"
	if value != nil && pattern != nil {
		description = "regexp(" + value.Description() + "," + pattern.Description() + ")"
	}
	children := []*exprNode{nil, nil}
	if value != nil {
		children[0] = value.node()
	}
	if pattern != nil {
		children[1] = pattern.node()
	}
	node := &exprNode{kind: "regexp", typ: typeOf[bool](), description: description, children: children}
	if value == nil || value.node() == nil {
		node.configurationError = "regexp value expression is required"
	} else if pattern == nil || pattern.node() == nil {
		node.configurationError = "regexp pattern expression is required"
	} else if !isLikeTextType(value.Type()) {
		node.configurationError = fmt.Sprintf("regexp value must be text-compatible, got %s", mathTypeDescription(value.Type()))
	} else if !isLikeTextType(pattern.Type()) {
		node.configurationError = fmt.Sprintf("regexp pattern must be text-compatible, got %s", mathTypeDescription(pattern.Type()))
	}
	return typedExpr[bool]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil || pattern == nil {
			return Null()
		}
		left, leftOK := likeTextValue(value.eval(ctx))
		patternText, patternOK := likeTextValue(pattern.eval(ctx))
		if !leftOK || !patternOK {
			return Null()
		}
		compiled, err := regexp.Compile(likePattern(patternText))
		if err != nil {
			return Null()
		}
		return Present(compiled.MatchString(left))
	}}
}

func isLikeTextType(typ reflect.Type) bool {
	if typ == nil || typ == typeOf[any]() {
		return true
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return false
		}
	}
	if typ.Kind() == reflect.Interface || typ == typeOf[big.Int]() || typ == typeOf[big.Rat]() {
		return true
	}
	switch typ.Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func likeTextValue(value Value) (string, bool) {
	if !value.IsPresent() {
		return "", false
	}
	raw := reflect.ValueOf(value.Any())
	for raw.IsValid() && (raw.Kind() == reflect.Pointer || raw.Kind() == reflect.Interface) {
		if raw.IsNil() {
			return "", false
		}
		raw = raw.Elem()
	}
	if !raw.IsValid() || !raw.CanInterface() {
		return "", false
	}
	switch typed := raw.Interface().(type) {
	case string:
		return typed, true
	case big.Int:
		return typed.String(), true
	case big.Rat:
		return typed.FloatString(10), true
	}
	switch raw.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(raw.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(raw.Uint(), 10), true
	case reflect.Float32, reflect.Float64:
		bits := 64
		if raw.Kind() == reflect.Float32 {
			bits = 32
		}
		text := strconv.FormatFloat(raw.Float(), 'g', -1, bits)
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		return text, true
	default:
		return fmt.Sprint(raw.Interface()), false
	}
}

func patternToRegexpWithEscape(pattern, escape string) (string, bool) {
	escapeRunes := []rune(escape)
	if len(escapeRunes) != 1 {
		return "", false
	}
	escapeRune := escapeRunes[0]
	runes := []rune(pattern)
	var builder strings.Builder
	for index := 0; index < len(runes); index++ {
		current := runes[index]
		if current == escapeRune {
			if index+1 >= len(runes) {
				return "", false
			}
			index++
			builder.WriteString(regexp.QuoteMeta(string(runes[index])))
			continue
		}
		switch current {
		case '%':
			builder.WriteString(".*")
		case '_':
			builder.WriteByte('.')
		default:
			builder.WriteString(regexp.QuoteMeta(string(current)))
		}
	}
	return builder.String(), true
}
