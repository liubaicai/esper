package esper

import (
	"fmt"
	"iter"
	"math/big"
	"reflect"
	"sort"
	"strings"
)

// EnumOrdered is the ordered value set supported by enumeration methods.
// big.Int and big.Rat provide the Go equivalents of Esper's BigInteger and
// BigDecimal values without converting through float64.
type EnumOrdered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~string | big.Int | big.Rat
}

// EnumNumeric is the numeric subset used by enumeration sum/average methods.
type EnumNumeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | big.Int | big.Rat
}

// EnumElement returns the current item inside an enumerable lambda. It is the
// Go-style, analyzable equivalent of Esper's lambda item parameter; callers
// compose it with the normal expression constructors instead of supplying a
// runtime closure.
func EnumElement[T any]() Expression[T] {
	return makeExpr[T]("enum-element", fmt.Sprintf("element<%s>()", typeOf[T]()), nil, func(ctx EvalContext) Value {
		if !ctx.enumActive {
			return Missing()
		}
		return ctx.enumValue
	})
}

// EnumIndex returns the zero-based position of the current enumerable item.
func EnumIndex() Expression[int64] {
	return makeExpr[int64]("enum-index", "index()", nil, func(ctx EvalContext) Value {
		if !ctx.enumActive {
			return Missing()
		}
		return Present(ctx.enumIndex)
	})
}

// EnumSize returns the size of the input collection visible to the current
// enumerable lambda.
func EnumSize() Expression[int64] {
	return makeExpr[int64]("enum-size", "size()", nil, func(ctx EvalContext) Value {
		if !ctx.enumActive {
			return Missing()
		}
		return Present(ctx.enumSize)
	})
}

// EnumAccumulator returns the current fold result inside EnumAggregate.
func EnumAccumulator[T any]() Expression[T] {
	return makeExpr[T]("enum-accumulator", fmt.Sprintf("accumulator<%s>()", typeOf[T]()), nil, func(ctx EvalContext) Value {
		if !ctx.enumAccumulatorActive {
			return Missing()
		}
		return ctx.enumAccumulator
	})
}

// EnumField is a convenience for selecting a property from an event element.
// Property remains available for nested paths and non-event element values.
func EnumField[T any, V any](name string) Expression[V] {
	return Property[V](EnumElement[T](), name)
}

func enumElementContext(ctx EvalContext, item any, index, size int) EvalContext {
	nested := ctx
	nested.enumActive = true
	nested.enumValue = Present(item)
	nested.enumIndex = int64(index)
	nested.enumSize = int64(size)
	return nested
}

func enumItems[T any](expression Expression[[]T], ctx EvalContext) ([]T, Value, bool) {
	if expression == nil {
		return nil, Missing(), false
	}
	input := expression.eval(ctx)
	if !input.IsPresent() {
		return nil, input, false
	}
	items, err := As[[]T](input)
	if err != nil {
		return nil, Null(), false
	}
	return items, input, true
}

// EnumIterator is the pull-style iterator accepted by EnumCollect. The
// boolean is true while an item was returned and false at end of input. It is
// intentionally small so existing Go iterators can be adapted without
// exposing a Java-style collection abstraction in the rule API.
type EnumIterator[T any] interface {
	Next() (T, bool)
}

func enumCollectIterator[T any](raw any) ([]T, bool, bool) {
	if sequence, ok := raw.(iter.Seq[T]); ok {
		if sequence == nil {
			return nil, true, true
		}
		result := make([]T, 0)
		sequence(func(item T) bool {
			result = append(result, item)
			return true
		})
		return result, true, false
	}
	if iterator, ok := raw.(EnumIterator[T]); ok {
		if iterator == nil {
			return nil, true, true
		}
		result := make([]T, 0)
		for {
			item, ok := iterator.Next()
			if !ok {
				break
			}
			result = append(result, item)
		}
		return result, true, false
	}
	return nil, false, false
}

// EnumCollect normalizes an analyzable Go array, slice, named slice,
// iter.Seq, or pull-style EnumIterator into the ordered slice representation
// consumed by the enumeration methods. iter.Seq is the standard Go adapter
// for reusable iterable sources; maps.Values(map) can be used when the source
// is map-backed. Map iteration order remains unspecified, as it does in Go,
// so order-sensitive enumeration methods should receive an ordered source.
// The result type is []T, making the collection element type explicit in the
// fluent API and in the expression metadata.
func EnumCollect[T any](values Expr) Expression[[]T] {
	description := "collect(<nil>)"
	var children []*exprNode
	if values != nil {
		description = "collect(" + values.Description() + ")"
		children = []*exprNode{values.node()}
	}
	return makeExpr[[]T]("enum-collect", description, children, func(ctx EvalContext) Value {
		if values == nil {
			return Missing()
		}
		input := values.eval(ctx)
		if !input.IsPresent() {
			return input
		}
		if items, err := As[[]T](input); err == nil {
			return Present(enumCopy(items))
		}
		if items, recognized, isNull := enumCollectIterator[T](input.Any()); recognized {
			if isNull {
				return Null()
			}
			return Present(items)
		}
		raw := reflect.ValueOf(input.Any())
		if !raw.IsValid() {
			return Null()
		}
		for raw.Kind() == reflect.Pointer {
			if raw.IsNil() {
				return Null()
			}
			raw = raw.Elem()
		}
		if raw.Kind() != reflect.Array && raw.Kind() != reflect.Slice {
			return Null()
		}
		result := make([]T, 0, raw.Len())
		for index := 0; index < raw.Len(); index++ {
			item := raw.Index(index)
			if !item.CanInterface() {
				return Null()
			}
			converted, err := As[T](Present(item.Interface()))
			if err != nil {
				return Null()
			}
			result = append(result, converted)
		}
		return Present(result)
	})
}

func enumRatFromValue(value Value) (*big.Rat, bool) {
	if !value.IsPresent() {
		return nil, false
	}
	switch number := value.Any().(type) {
	case big.Int:
		return new(big.Rat).SetInt(&number), true
	case big.Rat:
		return new(big.Rat).Set(&number), true
	}
	raw := reflect.ValueOf(value.Any())
	if !raw.IsValid() {
		return nil, false
	}
	switch raw.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return new(big.Rat).SetInt64(raw.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return new(big.Rat).SetUint64(raw.Uint()), true
	case reflect.Float32, reflect.Float64:
		rat := new(big.Rat).SetFloat64(raw.Float())
		return rat, rat != nil
	default:
		return nil, false
	}
}

func enumRatTo[T EnumNumeric](number *big.Rat) (T, bool) {
	var zero T
	if number == nil {
		return zero, false
	}
	switch any(zero).(type) {
	case big.Int:
		if !number.IsInt() {
			return zero, false
		}
		var result big.Int
		result.Set(number.Num())
		return any(result).(T), true
	case big.Rat:
		var result big.Rat
		result.Set(number)
		return any(result).(T), true
	default:
		value, _ := number.Float64()
		converted := reflect.ValueOf(value).Convert(reflect.TypeOf(zero))
		return converted.Interface().(T), true
	}
}

func enumCompareValues(left, right Value) (int, bool) {
	if comparison, comparable := compareValues(left, right); comparable {
		return comparison, true
	}
	leftNumber, leftOK := enumRatFromValue(left)
	rightNumber, rightOK := enumRatFromValue(right)
	if !leftOK || !rightOK {
		return 0, false
	}
	return leftNumber.Cmp(rightNumber), true
}

func enumCopy[T any](items []T) []T {
	result := make([]T, len(items))
	copy(result, items)
	return result
}

func enumPredicateMatches(predicate Expression[bool], ctx EvalContext) bool {
	if predicate == nil {
		return true
	}
	value := predicate.eval(ctx)
	matched, ok := boolValue(value)
	return ok && matched
}

func enumExpressionChildren[T any](values Expression[[]T], predicate Expr) []*exprNode {
	children := make([]*exprNode, 0, 2)
	if values != nil {
		children = append(children, values.node())
	}
	if predicate != nil {
		children = append(children, predicate.node())
	}
	return children
}

func enumDescription[T any](name string, values Expression[[]T], predicate Expr) string {
	input := "<nil>"
	if values != nil {
		input = values.Description()
	}
	if predicate == nil {
		return name + "(" + input + ")"
	}
	return name + "(" + input + "," + predicate.Description() + ")"
}

// makeEnumExpr attaches the small amount of declaration metadata needed for
// build-time enumeration validation. Generic Go signatures already enforce
// most type rules; these flags cover the Java invalid-rule cases that would
// otherwise be represented by a nil expression at runtime.
func makeEnumExpr[T any](kind, description string, children []*exprNode, input Expr, parameterRequired bool, fn func(EvalContext) Value) Expression[T] {
	expression := makeExpr[T](kind, description, children, fn)
	expression.node().enumInputRequired = input != nil
	expression.node().enumParameterRequired = parameterRequired
	return expression
}

func validateEnumExpressionNodes(node *exprNode) error {
	if node == nil {
		return nil
	}
	if node.enumInputRequired && len(node.children) == 0 {
		return NewError(ErrorInvalidRule, fmt.Sprintf("enumeration method %q requires a collection expression", strings.TrimPrefix(node.kind, "enum-")))
	}
	if node.enumParameterRequired {
		minimumChildren := 2
		if node.kind == "enum-to-map" {
			minimumChildren = 3
		}
		if len(node.children) < minimumChildren {
			return NewError(ErrorInvalidRule, fmt.Sprintf("enumeration method %q requires all selector expressions", strings.TrimPrefix(node.kind, "enum-")))
		}
	}
	for _, child := range node.children {
		if err := validateEnumExpressionNodes(child); err != nil {
			return err
		}
	}
	return nil
}

// EnumWhere filters a collection while retaining input order. A null or
// missing collection remains null or missing; an empty collection is a
// present empty collection.
func EnumWhere[T any](values Expression[[]T], predicate Expression[bool]) Expression[[]T] {
	return makeEnumExpr[[]T]("enum-where", enumDescription[T]("where", values, predicate), enumExpressionChildren[T](values, predicate), values, true, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		result := make([]T, 0, len(items))
		for index, item := range items {
			if enumPredicateMatches(predicate, enumElementContext(ctx, item, index, len(items))) {
				result = append(result, item)
			}
		}
		return Present(result)
	})
}

// EnumSelect projects each collection item into a new ordered collection.
func EnumSelect[T any, R any](values Expression[[]T], selector Expression[R]) Expression[[]R] {
	return makeEnumExpr[[]R]("enum-select", enumDescription[T]("select", values, selector), enumExpressionChildren[T](values, selector), values, true, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		if selector == nil {
			return Null()
		}
		result := make([]R, 0, len(items))
		for index, item := range items {
			value := selector.eval(enumElementContext(ctx, item, index, len(items)))
			if !value.IsPresent() {
				continue
			}
			converted, err := As[R](value)
			if err != nil {
				continue
			}
			result = append(result, converted)
		}
		return Present(result)
	})
}

// EnumArrayOf is the explicit Go counterpart of Esper's arrayOf method. Go
// slices are already the natural ordered collection/array representation, so
// the no-selector form makes a defensive copy while the selector form is
// exposed as EnumArrayOfSelect.
func EnumArrayOf[T any](values Expression[[]T]) Expression[[]T] {
	return makeEnumExpr[[]T]("enum-array-of", "array-of("+enumInputDescription[T](values)+")", enumExpressionChildren[T](values, nil), values, false, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		return Present(enumCopy(items))
	})
}

func EnumArrayOfSelect[T any, R any](values Expression[[]T], selector Expression[R]) Expression[[]R] {
	return makeEnumExpr[[]R]("enum-array-of", enumDescription[T]("array-of", values, selector), enumExpressionChildren[T](values, selector), values, true, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		if selector == nil {
			return Null()
		}
		result := make([]R, 0, len(items))
		for index, item := range items {
			value := selector.eval(enumElementContext(ctx, item, index, len(items)))
			if !value.IsPresent() {
				continue
			}
			converted, err := As[R](value)
			if err != nil {
				continue
			}
			result = append(result, converted)
		}
		return Present(result)
	})
}

// EnumCount counts all items, including null-valued items.
func EnumCount[T any](values Expression[[]T]) Expression[int64] {
	return makeEnumExpr[int64]("enum-count", enumDescription[T]("count", values, nil), enumExpressionChildren[T](values, nil), values, false, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		return Present(int64(len(items)))
	})
}

// EnumCountOf counts items for which the optional predicate is true.
func EnumCountOf[T any](values Expression[[]T], predicate Expression[bool]) Expression[int64] {
	return makeEnumExpr[int64]("enum-count-of", enumDescription[T]("count-of", values, predicate), enumExpressionChildren[T](values, predicate), values, true, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		var count int64
		for index, item := range items {
			if enumPredicateMatches(predicate, enumElementContext(ctx, item, index, len(items))) {
				count++
			}
		}
		return Present(count)
	})
}

// EnumAnyOf and EnumAllOf use normal boolean lambda semantics: null or
// missing predicate results do not match an item. A null input collection
// remains null; empty any/all are false/true respectively.
func EnumAnyOf[T any](values Expression[[]T], predicate Expression[bool]) Expression[bool] {
	return makeEnumExpr[bool]("enum-any-of", enumDescription[T]("any-of", values, predicate), enumExpressionChildren[T](values, predicate), values, true, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		for index, item := range items {
			if enumPredicateMatches(predicate, enumElementContext(ctx, item, index, len(items))) {
				return Present(true)
			}
		}
		return Present(false)
	})
}

func EnumAllOf[T any](values Expression[[]T], predicate Expression[bool]) Expression[bool] {
	return makeEnumExpr[bool]("enum-all-of", enumDescription[T]("all-of", values, predicate), enumExpressionChildren[T](values, predicate), values, true, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		for index, item := range items {
			if !enumPredicateMatches(predicate, enumElementContext(ctx, item, index, len(items))) {
				return Present(false)
			}
		}
		return Present(true)
	})
}

func enumFirstLast[T any](kind string, values Expression[[]T], predicate Expression[bool], last bool) Expression[T] {
	return makeEnumExpr[T]("enum-"+kind, enumDescription[T](kind, values, predicate), enumExpressionChildren[T](values, predicate), values, false, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		if last {
			for index := len(items) - 1; index >= 0; index-- {
				if enumPredicateMatches(predicate, enumElementContext(ctx, items[index], index, len(items))) {
					return Present(items[index])
				}
			}
		} else {
			for index, item := range items {
				if enumPredicateMatches(predicate, enumElementContext(ctx, item, index, len(items))) {
					return Present(item)
				}
			}
		}
		return Null()
	})
}

func EnumFirstOf[T any](values Expression[[]T], predicate ...Expression[bool]) Expression[T] {
	return enumFirstLast[T]("first-of", values, optionalEnumPredicate(predicate), false)
}

func EnumLastOf[T any](values Expression[[]T], predicate ...Expression[bool]) Expression[T] {
	return enumFirstLast[T]("last-of", values, optionalEnumPredicate(predicate), true)
}

func optionalEnumPredicate(predicate []Expression[bool]) Expression[bool] {
	if len(predicate) == 0 {
		return nil
	}
	return predicate[0]
}

// EnumDistinct keeps the first item for each distinct key. It uses deep
// equality rather than requiring comparable T, matching array/object element
// behavior and allowing event values with slice fields.
func EnumDistinct[T any](values Expression[[]T]) Expression[[]T] {
	return enumDistinctBy[T](values, nil, false)
}

func EnumDistinctBy[T any, K any](values Expression[[]T], selector Expression[K]) Expression[[]T] {
	return enumDistinctBy[T](values, selector, true)
}

func enumDistinctBy[T any](values Expression[[]T], selector Expr, parameterRequired bool) Expression[[]T] {
	return makeEnumExpr[[]T]("enum-distinct", enumDescription[T]("distinct", values, selector), enumExpressionChildren[T](values, selector), values, parameterRequired, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		result := make([]T, 0, len(items))
		keys := make([]Value, 0, len(items))
		for index, item := range items {
			evalContext := enumElementContext(ctx, item, index, len(items))
			key := Present(item)
			if selector != nil {
				key = selector.eval(evalContext)
			}
			duplicate := false
			for _, seen := range keys {
				if key.Equal(seen) {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			keys = append(keys, key)
			result = append(result, item)
		}
		return Present(result)
	})
}

func EnumTake[T any](values Expression[[]T], count int) Expression[[]T] {
	return enumSlice[T]("take", values, count, false)
}

func EnumTakeLast[T any](values Expression[[]T], count int) Expression[[]T] {
	return enumSlice[T]("take-last", values, count, true)
}

func enumSlice[T any](kind string, values Expression[[]T], count int, last bool) Expression[[]T] {
	return makeEnumExpr[[]T]("enum-"+kind, fmt.Sprintf("%s(%s,%d)", kind, enumInputDescription[T](values), count), enumExpressionChildren[T](values, nil), values, false, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		limit := count
		if limit <= 0 || len(items) == 0 {
			return Present(make([]T, 0))
		}
		if limit > len(items) {
			limit = len(items)
		}
		if last {
			return Present(enumCopy(items[len(items)-limit:]))
		}
		return Present(enumCopy(items[:limit]))
	})
}

func EnumTakeWhile[T any](values Expression[[]T], predicate Expression[bool]) Expression[[]T] {
	return enumTakeWhile[T](values, predicate, false)
}

func EnumTakeWhileLast[T any](values Expression[[]T], predicate Expression[bool]) Expression[[]T] {
	return enumTakeWhile[T](values, predicate, true)
}

func enumTakeWhile[T any](values Expression[[]T], predicate Expression[bool], last bool) Expression[[]T] {
	kind := "take-while"
	if last {
		kind = "take-while-last"
	}
	return makeEnumExpr[[]T]("enum-"+kind, enumDescription[T](kind, values, predicate), enumExpressionChildren[T](values, predicate), values, true, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		if predicate == nil {
			return Present(enumCopy(items))
		}
		if last {
			start := len(items)
			for index := len(items) - 1; index >= 0; index-- {
				if !enumPredicateMatches(predicate, enumElementContext(ctx, items[index], index, len(items))) {
					break
				}
				start = index
			}
			return Present(enumCopy(items[start:]))
		}
		end := 0
		for index, item := range items {
			if !enumPredicateMatches(predicate, enumElementContext(ctx, item, index, len(items))) {
				break
			}
			end++
		}
		return Present(enumCopy(items[:end]))
	})
}

func EnumReverse[T any](values Expression[[]T]) Expression[[]T] {
	return makeEnumExpr[[]T]("enum-reverse", "reverse("+enumInputDescription[T](values)+")", enumExpressionChildren[T](values, nil), values, false, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		result := enumCopy(items)
		for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
			result[left], result[right] = result[right], result[left]
		}
		return Present(result)
	})
}

func EnumMin[T EnumOrdered](values Expression[[]T]) Expression[T] {
	return enumExtreme[T]("min", values, nil, true, false)
}

func EnumMax[T EnumOrdered](values Expression[[]T]) Expression[T] {
	return enumExtreme[T]("max", values, nil, false, false)
}

func EnumMinBy[T any, K EnumOrdered](values Expression[[]T], selector Expression[K]) Expression[T] {
	return enumExtreme[T]("min-by", values, selector, true, true)
}

func EnumMaxBy[T any, K EnumOrdered](values Expression[[]T], selector Expression[K]) Expression[T] {
	return enumExtreme[T]("max-by", values, selector, false, true)
}

func enumExtreme[T any](kind string, values Expression[[]T], selector Expr, minimum, parameterRequired bool) Expression[T] {
	return makeEnumExpr[T]("enum-"+kind, enumDescription[T](kind, values, selector), enumExpressionChildren[T](values, selector), values, parameterRequired, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		var selected Value
		var selectedKey Value
		for index, item := range items {
			evalContext := enumElementContext(ctx, item, index, len(items))
			key := Present(item)
			if selector != nil {
				key = selector.eval(evalContext)
			}
			if !key.IsPresent() {
				continue
			}
			if !selected.IsPresent() {
				selected = Present(item)
				selectedKey = key
				continue
			}
			comparison, comparable := enumCompareValues(key, selectedKey)
			if comparable && ((minimum && comparison < 0) || (!minimum && comparison > 0)) {
				selected = Present(item)
				selectedKey = key
			}
		}
		if !selected.IsPresent() {
			return Null()
		}
		return selected
	})
}

func EnumOrderBy[T any, K EnumOrdered](values Expression[[]T], selector Expression[K], descending bool) Expression[[]T] {
	kind := "order-by"
	if descending {
		kind = "order-by-desc"
	}
	return makeEnumExpr[[]T]("enum-"+kind, enumDescription[T](kind, values, selector), enumExpressionChildren[T](values, selector), values, true, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		if selector == nil {
			return Null()
		}
		result := enumCopy(items)
		type keyed struct {
			item T
			key  Value
		}
		keyedItems := make([]keyed, 0, len(result))
		for index, item := range result {
			keyedItems = append(keyedItems, keyed{item: item, key: selector.eval(enumElementContext(ctx, item, index, len(items)))})
		}
		sort.SliceStable(keyedItems, func(left, right int) bool {
			comparison, comparable := enumCompareValues(keyedItems[left].key, keyedItems[right].key)
			if !comparable {
				return false
			}
			if descending {
				return comparison > 0
			}
			return comparison < 0
		})
		for index := range keyedItems {
			result[index] = keyedItems[index].item
		}
		return Present(result)
	})
}

func EnumSum[T EnumNumeric](values Expression[[]T]) Expression[T] {
	return enumSumOf[T, T](values, nil, false)
}

func EnumSumOf[T any, K EnumNumeric](values Expression[[]T], selector Expression[K]) Expression[K] {
	return enumSumOf[T, K](values, selector, true)
}

func enumSumOf[T any, K EnumNumeric](values Expression[[]T], selector Expression[K], parameterRequired bool) Expression[K] {
	return makeEnumExpr[K]("enum-sum", enumDescription[T]("sum-of", values, selector), enumExpressionChildren[T](values, selector), values, parameterRequired, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		total := new(big.Rat)
		found := false
		for index, item := range items {
			value := Present(item)
			if selector != nil {
				value = selector.eval(enumElementContext(ctx, item, index, len(items)))
			}
			number, numeric := enumRatFromValue(value)
			if !numeric {
				continue
			}
			total.Add(total, number)
			found = true
		}
		if !found {
			return Null()
		}
		result, ok := enumRatTo[K](total)
		if !ok {
			return Null()
		}
		return Present(result)
	})
}

func EnumAverage[T EnumNumeric](values Expression[[]T]) Expression[float64] {
	return enumAverageOf[T, T](values, nil, false)
}

func EnumAverageOf[T any, K EnumNumeric](values Expression[[]T], selector Expression[K]) Expression[float64] {
	return enumAverageOf[T, K](values, selector, true)
}

func enumAverageOf[T any, K EnumNumeric](values Expression[[]T], selector Expression[K], parameterRequired bool) Expression[float64] {
	return makeEnumExpr[float64]("enum-average", enumDescription[T]("average", values, selector), enumExpressionChildren[T](values, selector), values, parameterRequired, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		total := new(big.Rat)
		var count int
		for index, item := range items {
			value := Present(item)
			if selector != nil {
				value = selector.eval(enumElementContext(ctx, item, index, len(items)))
			}
			number, numeric := enumRatFromValue(value)
			if !numeric {
				continue
			}
			total.Add(total, number)
			count++
		}
		if count == 0 {
			return Null()
		}
		average := new(big.Rat).Quo(total, new(big.Rat).SetInt64(int64(count)))
		value, _ := average.Float64()
		return Present(value)
	})
}

// EnumAverageExact returns an exact rational average. It is the lossless Go
// counterpart for Esper enumeration averages over BigDecimal/BigInteger
// values; callers can render the result as a decimal with big.Rat.FloatString.
func EnumAverageExact[T EnumNumeric](values Expression[[]T]) Expression[big.Rat] {
	return enumAverageExactOf[T, T](values, nil, false)
}

// EnumAverageExactOf applies an analyzable numeric selector and retains the
// exact rational result instead of narrowing to float64.
func EnumAverageExactOf[T any, K EnumNumeric](values Expression[[]T], selector Expression[K]) Expression[big.Rat] {
	return enumAverageExactOf[T, K](values, selector, true)
}

func enumAverageExactOf[T any, K EnumNumeric](values Expression[[]T], selector Expression[K], parameterRequired bool) Expression[big.Rat] {
	return makeEnumExpr[big.Rat]("enum-average-exact", enumDescription[T]("average-exact", values, selector), enumExpressionChildren[T](values, selector), values, parameterRequired, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		total := new(big.Rat)
		count := int64(0)
		for index, item := range items {
			value := Present(item)
			if selector != nil {
				value = selector.eval(enumElementContext(ctx, item, index, len(items)))
			}
			number, numeric := enumRatFromValue(value)
			if !numeric {
				continue
			}
			total.Add(total, number)
			count++
		}
		if count == 0 {
			return Null()
		}
		return Present(*new(big.Rat).Quo(total, new(big.Rat).SetInt64(count)))
	})
}

// EnumAggregate folds an ordered collection from left to right. The
// accumulator expression can use EnumAccumulator, EnumElement, EnumIndex and
// EnumSize, so the entire fold remains visible in the AST.
func EnumAggregate[T any, R any](values Expression[[]T], initial R, accumulator Expression[R]) Expression[R] {
	return makeEnumExpr[R]("enum-aggregate", enumDescription[T]("aggregate", values, accumulator), enumExpressionChildren[T](values, accumulator), values, true, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		if accumulator == nil {
			return Null()
		}
		result := Present(initial)
		for index, item := range items {
			nested := enumElementContext(ctx, item, index, len(items))
			nested.enumAccumulatorActive = true
			nested.enumAccumulator = result
			next := accumulator.eval(nested)
			if !next.IsPresent() {
				result = next
				continue
			}
			converted, err := As[R](next)
			if err != nil {
				result = Null()
				continue
			}
			result = Present(converted)
		}
		return result
	})
}

func EnumExcept[T any](left, right Expression[[]T]) Expression[[]T] {
	return enumSetOperation[T]("except", left, right, func(inLeft, inRight bool) bool { return inLeft && !inRight })
}

func EnumIntersect[T any](left, right Expression[[]T]) Expression[[]T] {
	return enumSetOperation[T]("intersect", left, right, func(inLeft, inRight bool) bool { return inLeft && inRight })
}

func EnumUnion[T any](left, right Expression[[]T]) Expression[[]T] {
	return makeEnumExpr[[]T]("enum-union", fmt.Sprintf("union(%s,%s)", enumInputDescription[T](left), enumInputDescription[T](right)), enumPairChildren[T](left, right), left, true, func(ctx EvalContext) Value {
		leftItems, leftValue, leftOK := enumItems[T](left, ctx)
		if !leftOK {
			return leftValue
		}
		rightItems, rightValue, rightOK := enumItems[T](right, ctx)
		if !rightOK {
			return rightValue
		}
		result := make([]T, 0, len(leftItems)+len(rightItems))
		result = append(result, leftItems...)
		result = append(result, rightItems...)
		return Present(result)
	})
}

func enumSetOperation[T any](kind string, left, right Expression[[]T], keep func(bool, bool) bool) Expression[[]T] {
	return makeEnumExpr[[]T]("enum-"+kind, fmt.Sprintf("%s(%s,%s)", kind, enumInputDescription[T](left), enumInputDescription[T](right)), enumPairChildren[T](left, right), left, true, func(ctx EvalContext) Value {
		leftItems, leftValue, leftOK := enumItems[T](left, ctx)
		if !leftOK {
			return leftValue
		}
		rightItems, rightValue, rightOK := enumItems[T](right, ctx)
		if !rightOK {
			return rightValue
		}
		result := make([]T, 0, len(leftItems))
		for _, item := range leftItems {
			inRight := enumContains(rightItems, item)
			if keep(true, inRight) {
				result = append(result, item)
			}
		}
		return Present(result)
	})
}

func enumPairChildren[T any](left, right Expression[[]T]) []*exprNode {
	children := make([]*exprNode, 0, 2)
	if left != nil {
		children = append(children, left.node())
	}
	if right != nil {
		children = append(children, right.node())
	}
	return children
}

func enumContains[T any](items []T, candidate T) bool {
	for _, item := range items {
		if reflect.DeepEqual(item, candidate) {
			return true
		}
	}
	return false
}

// EnumSequenceEqual compares two collections element-by-element, preserving
// null/missing input state as the third-valued result used by Esper.
func EnumSequenceEqual[T any](left, right Expression[[]T]) Expression[bool] {
	return makeEnumExpr[bool]("enum-sequence-equal", fmt.Sprintf("sequence-equal(%s,%s)", enumInputDescription[T](left), enumInputDescription[T](right)), enumPairChildren[T](left, right), left, true, func(ctx EvalContext) Value {
		leftItems, leftValue, leftOK := enumItems[T](left, ctx)
		rightItems, rightValue, rightOK := enumItems[T](right, ctx)
		if !leftOK || !rightOK {
			if !leftOK && !rightOK && leftValue.State() == rightValue.State() {
				return Null()
			}
			return Present(false)
		}
		if len(leftItems) != len(rightItems) {
			return Present(false)
		}
		for index := range leftItems {
			if !reflect.DeepEqual(leftItems[index], rightItems[index]) {
				return Present(false)
			}
		}
		return Present(true)
	})
}

// EnumGroupBy groups original items by an analyzable key expression. A null
// key maps to the zero value of K; callers that need to retain null distinctly
// can use K=any.
func EnumGroupBy[T any, K comparable](values Expression[[]T], key Expression[K]) Expression[map[K][]T] {
	return makeEnumExpr[map[K][]T]("enum-group-by", enumDescription[T]("group-by", values, key), enumExpressionChildren[T](values, key), values, true, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		groups := make(map[K][]T)
		for index, item := range items {
			if key == nil {
				return Null()
			}
			value := key.eval(enumElementContext(ctx, item, index, len(items)))
			var groupKey K
			if value.IsPresent() {
				converted, err := As[K](value)
				if err != nil {
					continue
				}
				groupKey = converted
			}
			groups[groupKey] = append(groups[groupKey], item)
		}
		return Present(groups)
	})
}

// EnumToMap maps each item to a key/value pair. Later items replace earlier
// values for the same key, matching the Java enumeration method contract.
func EnumToMap[T any, K comparable, V any](values Expression[[]T], key Expression[K], value Expression[V]) Expression[map[K]V] {
	children := enumExpressionChildren[T](values, key)
	if value != nil {
		children = append(children, value.node())
	}
	return makeEnumExpr[map[K]V]("enum-to-map", fmt.Sprintf("to-map(%s,%s,%s)", enumInputDescription[T](values), expressionDescription(key), expressionDescription(value)), children, values, true, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		if key == nil || value == nil {
			return Null()
		}
		result := make(map[K]V, len(items))
		for index, item := range items {
			nested := enumElementContext(ctx, item, index, len(items))
			keyValue := key.eval(nested)
			var mapKey K
			if keyValue.IsPresent() {
				converted, err := As[K](keyValue)
				if err != nil {
					continue
				}
				mapKey = converted
			}
			mapValue := value.eval(nested)
			var convertedValue V
			if mapValue.IsPresent() {
				converted, err := As[V](mapValue)
				if err != nil {
					continue
				}
				convertedValue = converted
			}
			result[mapKey] = convertedValue
		}
		return Present(result)
	})
}

func expressionDescription(expression Expr) string {
	if expression == nil {
		return "<nil>"
	}
	return expression.Description()
}

func enumFrequency[T any](kind string, values Expression[[]T], selector Expr, most, parameterRequired bool) Expression[T] {
	return makeEnumExpr[T]("enum-"+kind, enumDescription[T](kind, values, selector), enumExpressionChildren[T](values, selector), values, parameterRequired, func(ctx EvalContext) Value {
		items, input, ok := enumItems[T](values, ctx)
		if !ok {
			return input
		}
		type frequency struct {
			item  T
			key   Value
			count int
		}
		frequencies := make([]frequency, 0, len(items))
		for index, item := range items {
			key := Present(item)
			if selector != nil {
				key = selector.eval(enumElementContext(ctx, item, index, len(items)))
			}
			found := false
			for frequencyIndex := range frequencies {
				if frequencies[frequencyIndex].key.Equal(key) {
					frequencies[frequencyIndex].count++
					found = true
					break
				}
			}
			if !found {
				frequencies = append(frequencies, frequency{item: item, key: key, count: 1})
			}
		}
		if len(frequencies) == 0 {
			return Null()
		}
		selected := frequencies[0]
		for _, candidate := range frequencies[1:] {
			if (most && candidate.count > selected.count) || (!most && candidate.count < selected.count) {
				selected = candidate
			}
		}
		return Present(selected.item)
	})
}

func EnumMostFrequent[T comparable](values Expression[[]T]) Expression[T] {
	return enumFrequency[T]("most-frequent", values, nil, true, false)
}

func EnumLeastFrequent[T comparable](values Expression[[]T]) Expression[T] {
	return enumFrequency[T]("least-frequent", values, nil, false, false)
}

func EnumMostFrequentBy[T any, K any](values Expression[[]T], selector Expression[K]) Expression[T] {
	return enumFrequency[T]("most-frequent", values, selector, true, true)
}

func EnumLeastFrequentBy[T any, K any](values Expression[[]T], selector Expression[K]) Expression[T] {
	return enumFrequency[T]("least-frequent", values, selector, false, true)
}

func enumInputDescription[T any](values Expression[[]T]) string {
	if values == nil {
		return "<nil>"
	}
	return values.Description()
}
