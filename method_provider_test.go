package esper

import (
	"context"
	"iter"
	"reflect"
	"testing"
	"time"
)

func TestTypedMethodProviderMaterializesDeclaredReturnShapes(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(7, 0).UTC()

	structSchema, err := StructSchema[methodSourceRow]("TypedMethodStructRow")
	if err != nil {
		t.Fatal(err)
	}
	structProvider, err := NewTypedMethodProvider(structSchema, func(_ context.Context, _ MethodRequest) ([]methodSourceRow, error) {
		return []methodSourceRow{{Symbol: "struct", Value: 1}, {Symbol: "struct-2", Value: 2}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	structEvents, err := structProvider.Poll(ctx, MethodRequest{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(structEvents) != 2 || structEvents[0].Underlying().(methodSourceRow).Value != 1 || structEvents[1].Get("symbol").Any() != "struct-2" {
		t.Fatalf("typed struct results = %#v", structEvents)
	}

	mapSchema, err := NewMapSchema("TypedMethodMapRow", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("value", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	mapProvider, err := NewTypedMethodProvider(mapSchema, func(_ context.Context, _ MethodRequest) ([]map[string]any, error) {
		return []map[string]any{{"symbol": "map", "value": 3}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	mapEvents, err := mapProvider.Poll(ctx, MethodRequest{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(mapEvents) != 1 || mapEvents[0].Get("symbol").Any() != "map" || mapEvents[0].Get("value").Any() != 3 {
		t.Fatalf("typed map results = %#v", mapEvents)
	}

	objectSchema, err := NewObjectArraySchema("TypedMethodObjectArrayRow", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("value", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	objectProvider, err := NewTypedMethodProvider(objectSchema, func(_ context.Context, _ MethodRequest) ([][]any, error) {
		return [][]any{{"object-array", 4}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	objectEvents, err := objectProvider.Poll(ctx, MethodRequest{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(objectEvents) != 1 || objectEvents[0].Get("symbol").Any() != "object-array" || objectEvents[0].Get("value").Any() != 4 {
		t.Fatalf("typed object-array results = %#v", objectEvents)
	}
}

func TestTypedMethodProviderSupportsSingleAndSequenceReturns(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(8, 0).UTC()
	schema, err := StructSchema[methodSourceRow]("TypedMethodSingleRow")
	if err != nil {
		t.Fatal(err)
	}
	single, err := NewSingleMethodProvider(schema, func(_ context.Context, _ MethodRequest) (methodSourceRow, error) {
		return methodSourceRow{Symbol: "single", Value: 5}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	singleEvents, err := single.Poll(ctx, MethodRequest{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(singleEvents) != 1 || singleEvents[0].Get("symbol").Any() != "single" {
		t.Fatalf("single method results = %#v", singleEvents)
	}

	sequence, err := NewSequenceMethodProvider(schema, func(_ context.Context, _ MethodRequest) (iter.Seq[methodSourceRow], error) {
		return func(yield func(methodSourceRow) bool) {
			if !yield(methodSourceRow{Symbol: "first", Value: 6}) {
				return
			}
			yield(methodSourceRow{Symbol: "second", Value: 7})
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sequenceEvents, err := sequence.Poll(ctx, MethodRequest{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(sequenceEvents) != 2 || sequenceEvents[0].Get("symbol").Any() != "first" || sequenceEvents[1].Get("value").Any() != 7 {
		t.Fatalf("sequence method results = %#v", sequenceEvents)
	}

	if _, err := NewTypedMethodProvider(Schema{}, func(context.Context, MethodRequest) ([]methodSourceRow, error) { return nil, nil }); err == nil {
		t.Fatal("typed method provider accepted an empty schema")
	}
	var nilSequence func(context.Context, MethodRequest) (iter.Seq[methodSourceRow], error)
	if _, err := NewSequenceMethodProvider(schema, nilSequence); err == nil {
		t.Fatal("sequence method provider accepted a nil function")
	}
}
