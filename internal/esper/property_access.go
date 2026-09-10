package esper

import (
	"reflect"
	"strings"
	"sync"
	"unicode"
)

// structFieldEntry is one property candidate of a Go struct type: the field
// index path from the root struct plus the two names that select it. The
// entry order reproduces the ordered depth-first walk that structFieldValue
// performed for every lookup, so the first resolving entry is the same field
// the per-lookup scan returned.
type structFieldEntry struct {
	path   []int
	name   string
	goName string
}

// structFieldTable is the immutable name index of one Go struct type. It is
// built once per type and shared by every schema and event value of that
// type, which removes the per-lookup field scan and tag parsing from the
// property read path (including case-insensitive resolution).
type structFieldTable struct {
	entries []structFieldEntry
	byName  map[string][]int32
	byFold  map[string][]int32
}

var structFieldTables sync.Map // reflect.Type -> *structFieldTable

// structFieldTableFor returns the shared table for a struct type, building it
// on first use. Concurrent builders store the same shape, so LoadOrStore
// value identity is not required for correctness.
func structFieldTableFor(typ reflect.Type) *structFieldTable {
	if cached, ok := structFieldTables.Load(typ); ok {
		return cached.(*structFieldTable)
	}
	table := newStructFieldTable(typ)
	actual, _ := structFieldTables.LoadOrStore(typ, table)
	return actual.(*structFieldTable)
}

func newStructFieldTable(typ reflect.Type) *structFieldTable {
	table := &structFieldTable{
		byName: make(map[string][]int32),
		byFold: make(map[string][]int32),
	}
	// The root type starts on the recursion path, matching the cycle guard of
	// the per-lookup scan: a type that reappears inside its own anonymous
	// descent contributes no further candidates.
	table.collect(typ, nil, map[reflect.Type]bool{typ: true})
	return table
}

func (t *structFieldTable) collect(typ reflect.Type, prefix []int, inPath map[reflect.Type]bool) {
	for index := range typ.NumField() {
		field := typ.Field(index)
		tag := field.Tag.Get("esper")
		if tag == "-" {
			continue
		}
		name, _ := parseFieldTag(tag)
		if name == "" {
			name, _ = parseFieldTag(field.Tag.Get("json"))
		}
		if name == "-" {
			continue
		}
		path := make([]int, len(prefix)+1)
		copy(path, prefix)
		path[len(prefix)] = index
		baseType := field.Type
		for baseType.Kind() == reflect.Pointer {
			baseType = baseType.Elem()
		}
		if field.Anonymous && name == "" && baseType.Kind() == reflect.Struct {
			if !inPath[baseType] {
				inPath[baseType] = true
				t.collect(baseType, path, inPath)
				delete(inPath, baseType)
			}
			continue
		}
		if name == "" {
			name = field.Name
		}
		entry := int32(len(t.entries))
		t.entries = append(t.entries, structFieldEntry{path: path, name: name, goName: field.Name})
		t.byName[name] = append(t.byName[name], entry)
		if field.Name != name {
			t.byName[field.Name] = append(t.byName[field.Name], entry)
		}
		foldedName := foldPropertyName(name)
		t.byFold[foldedName] = append(t.byFold[foldedName], entry)
		if field.Name != name {
			foldedGoName := foldPropertyName(field.Name)
			if foldedGoName != foldedName {
				t.byFold[foldedGoName] = append(t.byFold[foldedGoName], entry)
			}
		}
	}
}

// lookup returns the first resolving candidate selected by target under the
// configured resolution style.
func (t *structFieldTable) lookup(value reflect.Value, target string, resolution PropertyResolutionStyle) reflect.Value {
	var indexes []int32
	if resolution == PropertyCaseSensitive {
		indexes = t.byName[target]
	} else {
		indexes = t.byFold[foldPropertyName(target)]
	}
	for _, index := range indexes {
		if field, ok := resolveStructFieldPath(value, t.entries[index].path); ok {
			return field
		}
	}
	return reflect.Value{}
}

// resolveStructFieldPath walks a precomputed index path. Intermediate steps
// are anonymous struct fields; a nil pointer there invalidates only this
// candidate so later candidates still apply, matching the per-lookup scan.
func resolveStructFieldPath(value reflect.Value, path []int) (reflect.Value, bool) {
	current := value
	for step, index := range path {
		if current.Kind() != reflect.Struct {
			return reflect.Value{}, false
		}
		field := current.Field(index)
		if step == len(path)-1 {
			return field, true
		}
		for field.Kind() == reflect.Pointer {
			if field.IsNil() {
				return reflect.Value{}, false
			}
			field = field.Elem()
		}
		if field.Kind() != reflect.Struct {
			return reflect.Value{}, false
		}
		current = field
	}
	return reflect.Value{}, false
}

// foldPropertyName maps each rune to the minimum rune of its simple-fold
// orbit, so two names fold to equal keys exactly when strings.EqualFold
// reports them equal. Only names that actually fold differently from
// themselves are rewritten, so the common case allocates nothing.
func foldPropertyName(name string) string {
	foldable := false
	for _, r := range name {
		if unicode.SimpleFold(r) != r {
			foldable = true
			break
		}
	}
	if !foldable {
		return name
	}
	return strings.Map(func(r rune) rune {
		minimum := r
		for folded := unicode.SimpleFold(r); folded != r; folded = unicode.SimpleFold(folded) {
			if folded < minimum {
				minimum = folded
			}
		}
		return minimum
	}, name)
}
