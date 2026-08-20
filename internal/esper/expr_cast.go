package esper

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	timeType     = reflect.TypeOf(time.Time{})
	durationType = reflect.TypeOf(time.Duration(0))
	bigIntType   = reflect.TypeOf(big.Int{})
	bigRatType   = reflect.TypeOf(big.Rat{})
)

// CastWithLayout parses a string expression into time.Time using a Go
// time.Parse layout. It also accepts numeric epoch-millisecond input and
// formats time.Time into a stable RFC3339Nano string when the target is
// string. Use Cast for the ordinary type-conversion path.
func CastWithLayout[A, B any](value Expression[A], layout string) Expression[B] {
	expression := castExpression[A, B](value, layout, nil)
	if strings.TrimSpace(layout) == "" {
		expression.node().configurationError = "cast layout must not be empty"
	}
	return expression
}

// CastWithFormat is the dynamic-layout counterpart of CastWithLayout. The
// layout expression is evaluated with the same event context as the value,
// which keeps date parsing analyzable while matching Esper's dynamic
// dateformat parameter boundary.
func CastWithFormat[A, B any](value Expression[A], layout Expression[string]) Expression[B] {
	if layout == nil {
		expression := castExpression[A, B](value, "", nil)
		expression.node().configurationError = "cast format requires a layout expression"
		return expression
	}
	return castExpression[A, B](value, "", layout)
}

func castExpression[A, B any](value Expression[A], layout string, layoutExpression Expression[string]) Expression[B] {
	description := fmt.Sprintf("cast<%s>(%s)", typeOf[B](), expressionDescription(value))
	children := make([]*exprNode, 0, 2)
	node := &exprNode{kind: "cast", typ: typeOf[B](), description: description}
	if value == nil || value.node() == nil {
		node.configurationError = "cast requires a value expression"
	} else {
		children = append(children, value.node())
	}
	if layoutExpression != nil {
		children = append(children, layoutExpression.node())
		description += ";format=" + layoutExpression.Description()
	} else if layout != "" {
		description += fmt.Sprintf(";layout=%q", layout)
	} else if layoutExpression == nil && castTargetNeedsLayout(typeOf[B]()) && value != nil {
		// The no-layout form still supports RFC3339/ISO input. A blank
		// explicit layout is rejected by CastWithLayout below, while Cast
		// intentionally keeps the default parser available.
	}
	node.description = description
	node.children = children
	if layoutExpression != nil && layoutExpression.node() == nil {
		node.configurationError = "cast format requires a layout expression"
	}
	if layout == "" && layoutExpression == nil && value != nil && castTargetNeedsLayout(typeOf[B]()) {
		// Default RFC3339/ISO parsing is valid; no static validation error.
	}
	return typedExpr[B]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil {
			return Null()
		}
		selectedLayout := layout
		if layoutExpression != nil {
			formatValue := layoutExpression.eval(ctx)
			if !formatValue.IsPresent() {
				return formatValue
			}
			format, ok := formatValue.Any().(string)
			if !ok || strings.TrimSpace(format) == "" {
				return Null()
			}
			selectedLayout = format
		}
		return castValueWithLayout[B](value.eval(ctx), selectedLayout)
	}}
}

func castTargetNeedsLayout(target reflect.Type) bool {
	if target == nil {
		return false
	}
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	return target == timeType
}

func castValueWithLayout[T any](value Value, layout string) Value {
	if !value.IsPresent() {
		return value
	}
	if isNilReflectValue(reflect.ValueOf(value.Any())) {
		return Null()
	}
	return castAnyToType(value.Any(), typeOf[T](), layout)
}

func castAnyToType(source any, target reflect.Type, layout string) Value {
	if target == nil || source == nil {
		return Null()
	}
	sourceValue := reflect.ValueOf(source)
	if isNilReflectValue(sourceValue) {
		return Null()
	}
	if sourceValue.Type().AssignableTo(target) {
		return Present(sourceValue.Interface())
	}
	if target.Kind() == reflect.Interface {
		if sourceValue.Type().Implements(target) {
			return Present(sourceValue.Interface())
		}
		return Null()
	}
	if target.Kind() == reflect.Pointer {
		converted := castAnyToType(source, target.Elem(), layout)
		if !converted.IsPresent() {
			return converted
		}
		result := reflect.New(target.Elem())
		if !setCastReflectValue(result.Elem(), converted.Any()) {
			return Null()
		}
		return Present(result.Interface())
	}

	// Dereference source values only after direct assignment and interface
	// handling. This preserves interface/boxed identity while allowing a
	// nullable pointer to cast to its underlying value type.
	for sourceValue.IsValid() && (sourceValue.Kind() == reflect.Pointer || sourceValue.Kind() == reflect.Interface) {
		if sourceValue.IsNil() {
			return Null()
		}
		sourceValue = sourceValue.Elem()
	}
	if !sourceValue.IsValid() || !sourceValue.CanInterface() {
		return Null()
	}
	source = sourceValue.Interface()

	if target == timeType {
		return castToTime(source, layout)
	}
	if target == bigIntType {
		return castToBigInt(source)
	}
	if target == bigRatType {
		return castToBigRat(source)
	}
	if target == durationType {
		return castToDuration(source)
	}
	if target.Kind() == reflect.String {
		return castToString(source)
	}
	if target == reflect.TypeOf(rune(0)) {
		if text, ok := source.(string); ok {
			runes := []rune(text)
			if len(runes) > 0 {
				return Present(runes[0])
			}
			return Null()
		}
	}
	if target.Kind() == reflect.Bool {
		return castToBool(source)
	}
	if target.Kind() == reflect.Array || target.Kind() == reflect.Slice {
		return castToCollection(sourceValue, target, layout)
	}
	if isNumericType(target) {
		return castToNativeNumber(source, sourceValue, target)
	}

	if sourceValue.Type().ConvertibleTo(target) {
		return Present(sourceValue.Convert(target).Interface())
	}
	return Null()
}

func castToString(source any) Value {
	switch value := source.(type) {
	case time.Time:
		return Present(value.Format(time.RFC3339Nano))
	case big.Int:
		return Present(value.String())
	case big.Rat:
		return Present(value.RatString())
	case []byte:
		return Present(string(value))
	default:
		return Present(fmt.Sprint(source))
	}
}

func castToBool(source any) Value {
	if value, ok := source.(bool); ok {
		return Present(value)
	}
	if text, ok := source.(string); ok {
		parsed, err := strconv.ParseBool(strings.TrimSpace(text))
		if err == nil {
			return Present(parsed)
		}
	}
	return Null()
}

func castToNativeNumber(source any, sourceValue reflect.Value, target reflect.Type) Value {
	if number, ok := source.(json.Number); ok {
		return parseStringNumber(number.String(), target)
	}
	if text, ok := source.(string); ok {
		return parseStringNumber(text, target)
	}
	if sourceValue.Type() == timeType {
		return convertNumericReflect(reflect.ValueOf(source.(time.Time).UnixMilli()), target)
	}
	if sourceValue.Type() == bigIntType {
		integer := source.(big.Int)
		if isIntegralType(target) {
			if integer.IsInt64() {
				return convertNumericReflect(reflect.ValueOf(integer.Int64()), target)
			}
			if integer.Sign() >= 0 && integer.BitLen() <= 64 {
				return convertNumericReflect(reflect.ValueOf(integer.Uint64()), target)
			}
			return Null()
		}
		value, _ := new(big.Float).SetInt(&integer).Float64()
		return convertNumericReflect(reflect.ValueOf(value), target)
	}
	if sourceValue.Type() == bigRatType {
		rational := source.(big.Rat)
		if isIntegralType(target) {
			integer := new(big.Int).Quo(rational.Num(), rational.Denom())
			return castToNativeNumber(*integer, reflect.ValueOf(*integer), target)
		}
		value, _ := rational.Float64()
		return convertNumericReflect(reflect.ValueOf(value), target)
	}
	if isNumericType(sourceValue.Type()) {
		return convertNumericReflect(sourceValue, target)
	}
	return Null()
}

func parseStringNumber(text string, target reflect.Type) Value {
	text = strings.TrimSpace(text)
	if text == "" {
		return Null()
	}
	if isIntegralType(target) {
		if isSignedType(target) {
			parsed, err := strconv.ParseInt(text, 0, target.Bits())
			if err != nil {
				parsed, err = strconv.ParseInt(text, 10, target.Bits())
			}
			if err != nil {
				return Null()
			}
			return convertNumericReflect(reflect.ValueOf(parsed), target)
		}
		parsed, err := strconv.ParseUint(text, 0, target.Bits())
		if err != nil {
			parsed, err = strconv.ParseUint(text, 10, target.Bits())
		}
		if err != nil {
			return Null()
		}
		return convertNumericReflect(reflect.ValueOf(parsed), target)
	}
	parsed, err := strconv.ParseFloat(text, target.Bits())
	if err != nil {
		return Null()
	}
	return convertNumericReflect(reflect.ValueOf(parsed), target)
}

func convertNumericReflect(source reflect.Value, target reflect.Type) Value {
	if !source.IsValid() || !isNumericType(target) {
		return Null()
	}
	if source.Kind() == reflect.Float32 || source.Kind() == reflect.Float64 {
		value := source.Float()
		if math.IsNaN(value) || math.IsInf(value, 0) {
			if target.Kind() != reflect.Float32 && target.Kind() != reflect.Float64 {
				return Null()
			}
		}
		if isIntegralType(target) {
			if isSignedType(target) && (value > float64(maxSignedInteger(target)) || value < float64(minSignedInteger(target))) {
				return Null()
			}
			if isUnsignedType(target) && (value < 0 || value > float64(maxUnsignedInteger(target))) {
				return Null()
			}
		}
	}
	if isIntegralType(target) {
		switch source.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			value := source.Int()
			if isSignedType(target) && (value < minSignedInteger(target) || value > maxSignedInteger(target)) {
				return Null()
			}
			if isUnsignedType(target) && (value < 0 || uint64(value) > maxUnsignedInteger(target)) {
				return Null()
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			value := source.Uint()
			if isSignedType(target) && value > uint64(maxSignedInteger(target)) {
				return Null()
			}
			if isUnsignedType(target) && value > maxUnsignedInteger(target) {
				return Null()
			}
		}
	}
	if source.Type().ConvertibleTo(target) {
		return Present(source.Convert(target).Interface())
	}
	return Null()
}

func isUnsignedType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	switch typ.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func isSignedType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	default:
		return false
	}
}

func maxSignedInteger(typ reflect.Type) int64 {
	if typ == nil || !isSignedType(typ) {
		return math.MaxInt64
	}
	if typ.Bits() >= 64 {
		return math.MaxInt64
	}
	return int64(1<<(typ.Bits()-1)) - 1
}

func minSignedInteger(typ reflect.Type) int64 {
	if typ == nil || !isSignedType(typ) {
		return math.MinInt64
	}
	if typ.Bits() >= 64 {
		return math.MinInt64
	}
	return -int64(1 << (typ.Bits() - 1))
}

func maxUnsignedInteger(typ reflect.Type) uint64 {
	if typ == nil || !isUnsignedType(typ) || typ.Bits() >= 64 {
		return ^uint64(0)
	}
	return (uint64(1) << typ.Bits()) - 1
}

func castToBigInt(source any) Value {
	switch value := source.(type) {
	case big.Int:
		var result big.Int
		result.Set(&value)
		return Present(result)
	case big.Rat:
		var result big.Int
		result.Quo(value.Num(), value.Denom())
		return Present(result)
	case string:
		text := strings.TrimSpace(value)
		var result big.Int
		if _, ok := result.SetString(text, 0); ok {
			return Present(result)
		}
		if rational, ok := new(big.Rat).SetString(text); ok {
			result.Quo(rational.Num(), rational.Denom())
			return Present(result)
		}
		return Null()
	}
	raw := reflect.ValueOf(source)
	if isNumericType(raw.Type()) {
		var result big.Int
		if isIntegralType(raw.Type()) {
			if isUnsignedType(raw.Type()) {
				result.SetUint64(raw.Uint())
			} else {
				result.SetInt64(raw.Int())
			}
			return Present(result)
		}
		value, _ := raw.Convert(reflect.TypeOf(float64(0))).Interface().(float64)
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return Null()
		}
		converted, _ := new(big.Float).SetFloat64(value).Int(nil)
		if converted == nil {
			return Null()
		}
		result.Set(converted)
		return Present(result)
	}
	return Null()
}

func castToBigRat(source any) Value {
	switch value := source.(type) {
	case big.Rat:
		var result big.Rat
		result.Set(&value)
		return Present(result)
	case big.Int:
		return Present(*new(big.Rat).SetInt(&value))
	case string:
		if result, ok := new(big.Rat).SetString(strings.TrimSpace(value)); ok {
			return Present(*result)
		}
		return Null()
	}
	raw := reflect.ValueOf(source)
	if isNumericType(raw.Type()) {
		if isIntegralType(raw.Type()) {
			if isUnsignedType(raw.Type()) {
				return Present(*new(big.Rat).SetUint64(raw.Uint()))
			}
			return Present(*new(big.Rat).SetInt64(raw.Int()))
		}
		value := raw.Float()
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return Null()
		}
		// Match Esper's cast(double, BigDecimal), which is implemented as
		// BigDecimal.valueOf(double) = new BigDecimal(Double.toString(d)).
		// Round-tripping through the shortest decimal preserves that exact
		// value (2.4 -> "2.4", 1.0 -> "1") instead of the binary-exact
		// expansion that SetFloat64 would produce.
		text := strconv.FormatFloat(value, 'g', -1, raw.Type().Bits())
		if result, ok := new(big.Rat).SetString(text); ok {
			return Present(*result)
		}
		return Null()
	}
	return Null()
}

func castToDuration(source any) Value {
	if value, ok := source.(time.Duration); ok {
		return Present(value)
	}
	if text, ok := source.(string); ok {
		parsed, err := time.ParseDuration(strings.TrimSpace(text))
		if err == nil {
			return Present(parsed)
		}
		return Null()
	}
	raw := reflect.ValueOf(source)
	if isNumericType(raw.Type()) {
		value, ok := numericValue(Present(source))
		if ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
			return Present(time.Duration(value))
		}
	}
	return Null()
}

func castToTime(source any, layout string) Value {
	if value, ok := source.(time.Time); ok {
		return Present(value)
	}
	if text, ok := source.(string); ok {
		parsed, ok := parseCastTime(text, layout)
		if ok {
			return Present(parsed)
		}
		return Null()
	}
	if raw := reflect.ValueOf(source); raw.IsValid() && isNumericType(raw.Type()) {
		millis, ok := numericValue(Present(source))
		if ok && !math.IsNaN(millis) && !math.IsInf(millis, 0) {
			return Present(time.UnixMilli(int64(millis)).UTC())
		}
	}
	return Null()
}

func parseCastTime(text, layout string) (time.Time, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return time.Time{}, false
	}
	if layout != "" && strings.EqualFold(layout, "iso") {
		layout = time.RFC3339Nano
	}
	if layout != "" {
		parsed, err := time.Parse(layout, text)
		return parsed, err == nil
	}
	for _, candidate := range []string{time.RFC3339Nano, time.RFC3339, time.DateTime, time.DateOnly, time.TimeOnly} {
		if parsed, err := time.Parse(candidate, text); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func castToCollection(source reflect.Value, target reflect.Type, layout string) Value {
	for source.IsValid() && (source.Kind() == reflect.Pointer || source.Kind() == reflect.Interface) {
		if source.IsNil() {
			return Null()
		}
		source = source.Elem()
	}
	if !source.IsValid() || (source.Kind() != reflect.Array && source.Kind() != reflect.Slice) {
		return Null()
	}
	if target.Kind() == reflect.Array && source.Len() != target.Len() {
		return Null()
	}
	var result reflect.Value
	if target.Kind() == reflect.Array {
		result = reflect.New(target).Elem()
	} else {
		result = reflect.MakeSlice(target, source.Len(), source.Len())
	}
	for index := 0; index < source.Len(); index++ {
		item := source.Index(index)
		if !item.IsValid() || !item.CanInterface() {
			return Null()
		}
		itemValue := reflectValueToValue(item)
		converted := castAnyToType(itemValue.Any(), target.Elem(), layout)
		if !itemValue.IsPresent() {
			converted = itemValue
		}
		if !converted.IsPresent() {
			if converted.IsNull() && isNilableType(target.Elem()) {
				result.Index(index).Set(reflect.Zero(target.Elem()))
				continue
			}
			return Null()
		}
		if !setCastReflectValue(result.Index(index), converted.Any()) {
			return Null()
		}
	}
	return Present(result.Interface())
}

func setCastReflectValue(target reflect.Value, source any) bool {
	if !target.IsValid() || !target.CanSet() || source == nil {
		return false
	}
	value := reflect.ValueOf(source)
	if value.Type().AssignableTo(target.Type()) {
		target.Set(value)
		return true
	}
	if value.Type().ConvertibleTo(target.Type()) {
		target.Set(value.Convert(target.Type()))
		return true
	}
	return false
}
