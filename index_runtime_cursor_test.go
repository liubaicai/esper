package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestIndexRangeCursorBoundsAndFallback(t *testing.T) {
	entries := [][]Value{
		{Present("A"), Present(int64(1)), Present("a")},
		{Present("A"), Present(int64(3)), Present("b")},
		{Present("A"), Present(int64(5)), Present("c")},
		{Present("A"), Present(int64(7)), Present("d")},
		{Present("A"), Present(int64(9)), Present("e")},
		{Present("B"), Present(int64(2)), Present("f")},
	}
	query := indexRangeSpec{
		prefix:        []any{"A"},
		rangePosition: 1,
		lower:         &indexRangeBound{value: int64(3), inclusive: true},
		upper:         &indexRangeBound{value: int64(7), inclusive: true},
	}
	start, end, usable := indexRangeCursorBounds(len(entries), func(index int) []Value { return entries[index] }, query)
	if !usable || start != 1 || end != 4 {
		t.Fatalf("cursor bounds = (%d,%d,%t), want (1,4,true)", start, end, usable)
	}
	positions, usable, err := collectIndexRangeCursorPositions(context.Background(), len(entries), func(index int) []Value { return entries[index] }, []indexRangeSpec{query})
	if err != nil || !usable {
		t.Fatalf("cursor collection usable=%t err=%v", usable, err)
	}
	if !reflect.DeepEqual(positions, map[int]struct{}{1: {}, 2: {}, 3: {}}) {
		t.Fatalf("cursor positions = %#v", positions)
	}

	nullEntries := [][]Value{{Present("A"), Null(), Present("null")}}
	if _, _, usable := indexRangeCursorBounds(len(nullEntries), func(index int) []Value { return nullEntries[index] }, query); usable {
		t.Fatal("cursor must fall back for a non-comparable Null range member")
	}
}
