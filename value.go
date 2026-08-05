package esper

import (
	"fmt"
	"reflect"
)

// ValueState distinguishes a missing property from a present property whose
// value is nil.  Esper expressions use this distinction for dynamic events
// and three-valued logic; Go's nil alone cannot represent it.
type ValueState uint8

const (
	ValueMissing ValueState = iota
	ValueNull
	ValuePresent
)

// Value is the runtime representation of an expression or event property.
// The zero value is Missing so that absent map/row fields remain safe.
type Value struct {
	state ValueState
	data  any
}

// Missing returns a value for a property that does not exist.
func Missing() Value { return Value{state: ValueMissing} }

// Null returns a value for a property that exists but has no value.
func Null() Value { return Value{state: ValueNull} }

// Present wraps a non-nil value. A nil argument is represented as Null.
func Present(v any) Value {
	if v == nil {
		return Null()
	}
	return Value{state: ValuePresent, data: v}
}

// State reports whether the value is Missing, Null or Present.
func (v Value) State() ValueState { return v.state }

func (v Value) IsMissing() bool { return v.state == ValueMissing }
func (v Value) IsNull() bool    { return v.state == ValueNull }
func (v Value) IsPresent() bool { return v.state == ValuePresent }

// Any returns the underlying value. Missing and Null both return nil; use
// State when that distinction matters.
func (v Value) Any() any {
	if v.state != ValuePresent {
		return nil
	}
	return v.data
}

// As performs a checked type assertion. Missing and Null return the zero value
// and false, while a present value of another type returns an error. It is a
// top-level generic function because Go does not support method-level type
// parameters.
func As[T any](v Value) (T, error) {
	var zero T
	if v.state != ValuePresent {
		return zero, nil
	}
	value, ok := v.data.(T)
	if !ok {
		return zero, fmt.Errorf("esper: value has type %T, requested %T", v.data, zero)
	}
	return value, nil
}

// Equal implements value equality for expression comparison. Missing and
// Null are deliberately not equal to a present value; callers that need
// three-valued expression semantics should use EqualValues below.
func (v Value) Equal(other Value) bool {
	if v.state != other.state {
		return false
	}
	if v.state != ValuePresent {
		return true
	}
	return reflect.DeepEqual(v.data, other.data)
}

func (v Value) String() string {
	switch v.state {
	case ValueMissing:
		return "<missing>"
	case ValueNull:
		return "<null>"
	default:
		return fmt.Sprint(v.data)
	}
}

// EqualValues returns a boolean Value suitable for expression evaluation.
// Comparing a Missing or Null operand produces Null, matching the initial
// Esper/SQL-style three-valued comparison contract.
func EqualValues(left, right Value) Value {
	if !left.IsPresent() || !right.IsPresent() {
		return Null()
	}
	return Present(reflect.DeepEqual(left.data, right.data))
}

func boolValue(v Value) (bool, bool) {
	if !v.IsPresent() {
		return false, false
	}
	b, ok := v.data.(bool)
	return b, ok
}

func numericValue(v Value) (float64, bool) {
	if !v.IsPresent() {
		return 0, false
	}
	switch n := v.data.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func compareValues(left, right Value) (int, bool) {
	if !left.IsPresent() || !right.IsPresent() {
		return 0, false
	}
	if l, ok := numericValue(left); ok {
		if r, ok := numericValue(right); ok {
			switch {
			case l < r:
				return -1, true
			case l > r:
				return 1, true
			default:
				return 0, true
			}
		}
	}
	if l, ok := left.data.(string); ok {
		if r, ok := right.data.(string); ok {
			switch {
			case l < r:
				return -1, true
			case l > r:
				return 1, true
			default:
				return 0, true
			}
		}
	}
	if reflect.TypeOf(left.data) != reflect.TypeOf(right.data) {
		return 0, false
	}
	if reflect.DeepEqual(left.data, right.data) {
		return 0, true
	}
	return 0, false
}

func andValues(left, right Value) Value {
	l, lok := boolValue(left)
	r, rok := boolValue(right)
	if lok && !l {
		return Present(false)
	}
	if rok && !r {
		return Present(false)
	}
	if lok && rok {
		return Present(l && r)
	}
	return Null()
}

func orValues(left, right Value) Value {
	l, lok := boolValue(left)
	r, rok := boolValue(right)
	if lok && l {
		return Present(true)
	}
	if rok && r {
		return Present(true)
	}
	if lok && rok {
		return Present(false)
	}
	return Null()
}
