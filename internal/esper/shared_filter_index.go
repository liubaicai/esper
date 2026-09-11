package esper

import (
	"reflect"
	"strconv"
)

// sharedFilterKey identifies a pure equality predicate over one source
// schema. The index is only an additional candidate guard; Statement.process
// still evaluates the original Esper expression, so the index cannot change
// observable expression semantics.
type sharedFilterKey struct {
	schema *schemaIdentityToken
	field  string
	value  string
}

// sharedFilterIndex groups statements by an exact field/literal predicate.
// A statement is indexed only when its stateless plan contains an equality
// predicate that is necessary for acceptance (all stream filters are ANDed).
type sharedFilterIndex struct {
	entries    map[sharedFilterKey][]*Statement
	marks      map[*Statement]uint64
	generation uint64
}

func (i *sharedFilterIndex) add(statement *Statement) {
	if i == nil || statement == nil {
		return
	}
	key, ok := statementSharedEqualityKey(statement)
	if !ok {
		return
	}
	statement.sharedFilterKey = &key
	if i.entries == nil {
		i.entries = make(map[sharedFilterKey][]*Statement)
	}
	if i.marks == nil {
		i.marks = make(map[*Statement]uint64)
	}
	for _, existing := range i.entries[key] {
		if existing == statement {
			return
		}
	}
	i.entries[key] = append(i.entries[key], statement)
}

func (i *sharedFilterIndex) remove(statement *Statement) {
	if i == nil || statement == nil {
		return
	}
	key, ok := statementSharedEqualityKey(statement)
	if !ok {
		return
	}
	items := i.entries[key]
	for n, existing := range items {
		if existing == statement {
			i.entries[key] = append(items[:n], items[n+1:]...)
			if len(i.entries[key]) == 0 {
				delete(i.entries, key)
			}
			return
		}
	}
}

func (i *sharedFilterIndex) candidates(event Event) uint64 {
	if i == nil || len(i.entries) == 0 || !event.schema.valid() {
		return 0
	}
	i.generation++
	if i.generation == 0 {
		clear(i.marks)
		i.generation = 1
	}
	for key, statements := range i.entries {
		if key.schema != event.schema.identity {
			continue
		}
		value := event.Get(key.field)
		actual, ok := sharedFilterValueKey(value)
		if !ok || actual != key.value {
			continue
		}
		for _, statement := range statements {
			i.marks[statement] = i.generation
		}
	}
	return i.generation
}

func (i *sharedFilterIndex) contains(statement *Statement, generation uint64) bool {
	return i != nil && generation != 0 && i.marks[statement] == generation
}

func statementSharedEqualityKey(statement *Statement) (sharedFilterKey, bool) {
	if statement == nil || statement.statelessPlan == nil || statement.statelessPlan.schema.identity == nil {
		return sharedFilterKey{}, false
	}
	query := statement.plan.query
	node := query.input
	for node != nil {
		if node.kind == streamFilter && node.predicate != nil {
			if key, ok := equalityNodeKey(statement.statelessPlan.schema.identity, node.predicate.node()); ok {
				return key, true
			}
		}
		node = node.input
	}
	return sharedFilterKey{}, false
}

func equalityNodeKey(schema *schemaIdentityToken, node *exprNode) (sharedFilterKey, bool) {
	if node == nil || node.kind != "eq" || len(node.children) != 2 {
		return sharedFilterKey{}, false
	}
	for _, pair := range [][2]*exprNode{{node.children[0], node.children[1]}, {node.children[1], node.children[0]}} {
		field, literal := pair[0], pair[1]
		if field == nil || literal == nil || field.kind != "field" || literal.kind != "literal" || !isPlainPropertyName(field.fieldName) {
			continue
		}
		value, ok := sharedFilterValueKey(Present(literal.literalValue))
		if !ok {
			continue
		}
		return sharedFilterKey{schema: schema, field: field.fieldName, value: value}, true
	}
	return sharedFilterKey{}, false
}

func sharedFilterValueKey(value Value) (string, bool) {
	if !value.IsPresent() {
		return "", false
	}
	rv := reflect.ValueOf(value.Any())
	for rv.IsValid() && (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) {
		if rv.IsNil() {
			return "", false
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return "", false
	}
	switch rv.Kind() {
	case reflect.String:
		return "s:" + rv.String(), true
	case reflect.Bool:
		return "b:" + strconv.FormatBool(rv.Bool()), true
	default:
		return "", false
	}
}
