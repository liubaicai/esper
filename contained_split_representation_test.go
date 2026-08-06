package esper

import (
	"context"
	"reflect"
	"testing"
)

type containedSplitRepresentationParent struct {
	ObjectRows [][]any       `esper:"objectRows"`
	AvroRows   []*AvroRecord `esper:"avroRows"`
	XMLRows    []string      `esper:"xmlRows"`
}

func TestContainedTypeMaterializesObjectArrayAvroAndXMLRows(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[containedSplitRepresentationParent](env, "ContainedSplitRepresentationParent"); err != nil {
		t.Fatal(err)
	}
	objectSchema, err := RegisterObjectArray(env, "ContainedSplitObjectWord", []FieldSpec{
		FieldDef("word", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	avroSchema, err := RegisterAvro(env, "ContainedSplitAvroWord", []FieldSpec{
		FieldDef("word", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterXML(env, "ContainedSplitXMLWord", []FieldSpec{
		FieldDef("word", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}

	avroOne, err := NewAvroRecordFromMap(avroSchema, map[string]any{"word": "avro-one"})
	if err != nil {
		t.Fatal(err)
	}
	avroTwo, err := NewAvroRecordFromMap(avroSchema, map[string]any{"word": "avro-two"})
	if err != nil {
		t.Fatal(err)
	}
	input := From[containedSplitRepresentationParent](env, "ContainedSplitRepresentationParent")
	objectStream := UnnestAs[containedSplitRepresentationParent, []any](
		input,
		Property[[][]any](EventValue[containedSplitRepresentationParent](), "objectRows"),
		"ContainedSplitObjectWord",
	)
	avroStream := UnnestAs[containedSplitRepresentationParent, *AvroRecord](
		input,
		Property[[]*AvroRecord](EventValue[containedSplitRepresentationParent](), "avroRows"),
		"ContainedSplitAvroWord",
	)
	xmlStream := UnnestAs[containedSplitRepresentationParent, string](
		input,
		Property[[]string](EventValue[containedSplitRepresentationParent](), "xmlRows"),
		"ContainedSplitXMLWord",
	)

	objectPlan, err := env.Build(Select(objectStream,
		Alias("word", Field[Event, string]("word")),
		Alias("event", EventValue[Event]()),
	).Query(StatementName("contained-split-object-array")))
	if err != nil {
		t.Fatal(err)
	}
	avroPlan, err := env.Build(Select(avroStream,
		Alias("word", Field[Event, string]("word")),
		Alias("event", EventValue[Event]()),
	).Query(StatementName("contained-split-avro")))
	if err != nil {
		t.Fatal(err)
	}
	xmlPlan, err := env.Build(Select(xmlStream,
		Alias("word", Field[Event, string]("word")),
		Alias("event", EventValue[Event]()),
	).Query(StatementName("contained-split-xml")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	objectRows := subscribeRows(t, engine, objectPlan)
	avroRows := subscribeRows(t, engine, avroPlan)
	xmlRows := subscribeRows(t, engine, xmlPlan)
	if err := engine.SendEvent(context.Background(), containedSplitRepresentationParent{
		ObjectRows: [][]any{{"object-one"}, {"object-two"}},
		AvroRows:   []*AvroRecord{avroOne, avroTwo},
		XMLRows:    []string{`<word>xml-one</word>`, `<word>xml-two</word>`},
	}); err != nil {
		t.Fatal(err)
	}

	assertContainedRepresentationRows(t, objectRows(), []string{"object-one", "object-two"}, SchemaObjectArray, objectSchema.Name())
	assertContainedRepresentationRows(t, avroRows(), []string{"avro-one", "avro-two"}, SchemaAvro, avroSchema.Name())
	assertContainedRepresentationRows(t, xmlRows(), []string{"xml-one", "xml-two"}, SchemaXML, "ContainedSplitXMLWord")
}

func subscribeRows(t *testing.T, engine *Engine, plan Plan) func() []Row {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("contained representation result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func() []Row { return rows }
}

func assertContainedRepresentationRows(t *testing.T, rows []Row, want []string, kind SchemaKind, typeName string) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("contained %s rows = %#v, want %d rows", typeName, rows, len(want))
	}
	for index, expected := range want {
		if got := rows[index].Get("word").Any(); got != expected {
			t.Fatalf("contained %s row %d word = %#v, want %q", typeName, index, got, expected)
		}
		event, ok := rows[index].Get("event").Any().(Event)
		if !ok || event.Schema().Kind() != kind || event.TypeName() != typeName {
			t.Fatalf("contained %s row %d event = %#v, want %v/%s", typeName, index, rows[index].Get("event"), kind, typeName)
		}
		switch kind {
		case SchemaObjectArray:
			values, ok := event.Underlying().([]any)
			if !ok || len(values) != 1 || values[0] != expected {
				t.Fatalf("contained object-array row %d underlying = %#v", index, event.Underlying())
			}
		case SchemaAvro:
			record, ok := event.Underlying().(*AvroRecord)
			if !ok || record.Get("word") != expected {
				t.Fatalf("contained Avro row %d underlying = %#v", index, event.Underlying())
			}
		case SchemaXML:
			if event.Get("word").Any() != expected {
				t.Fatalf("contained XML row %d underlying = %#v", index, event.Underlying())
			}
		}
	}
}
