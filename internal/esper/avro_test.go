package esper

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestAvroRecordNativeSendAndSchemaIdentityMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("qty", reflect.TypeOf(int64(0))),
		OptionalFieldDef("note", reflect.TypeOf("")),
	}
	schema, err := RegisterAvro(env, "NativeAvro", fields)
	if err != nil {
		t.Fatal(err)
	}
	record, err := NewAvroRecord(schema)
	if err != nil {
		t.Fatal(err)
	}
	if err := record.Set("symbol", "ESPER"); err != nil {
		t.Fatal(err)
	}
	if err := record.Set("qty", int(3)); err != nil {
		t.Fatal(err)
	}
	if value, exists := record.Lookup("note"); !exists || value != nil {
		t.Fatalf("unset declared field = (%#v, %t)", value, exists)
	}
	if _, exists := record.Lookup("missing"); exists {
		t.Fatal("unknown Avro field was reported as present")
	}

	plan, err := env.Build(FromAny(env, "NativeAvro").Query(StatementName("native-avro-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var received Event
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			received, _ = batch.New[0].Event()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendAvro(context.Background(), "NativeAvro", record); err != nil {
		t.Fatal(err)
	}
	if received.Underlying() != record || received.Get("symbol").Any() != "ESPER" || received.Get("qty").Any() != int64(3) || !received.Get("note").IsNull() {
		t.Fatalf("native Avro event = %#v", received)
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	if object["symbol"] != "ESPER" || object["qty"] != float64(3) {
		t.Fatalf("Avro JSON = %s", encoded)
	}

	clone := record.Clone()
	if err := clone.Set("symbol", "clone"); err != nil {
		t.Fatal(err)
	}
	if record.Get("symbol") != "ESPER" || clone.Get("symbol") != "clone" {
		t.Fatalf("Avro clone aliases source: source=%#v clone=%#v", record, clone)
	}
}

func TestAvroRecordRejectsWrongFieldsTypesAndSchemas(t *testing.T) {
	fields := []FieldSpec{FieldDef("qty", reflect.TypeOf(int64(0)))}
	first, err := NewAvroSchema("FirstAvro", fields)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewAvroSchema("SecondAvro", fields)
	if err != nil {
		t.Fatal(err)
	}
	record, err := NewAvroRecord(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := record.Set("missing", 1); err == nil {
		t.Fatal("Avro record accepted an undeclared field")
	}
	if err := record.Set("qty", "wrong"); err == nil {
		t.Fatal("Avro record accepted an incompatible field type")
	}
	if _, err := NewAvroRecordFromMap(first, map[string]any{"missing": 1}); err == nil {
		t.Fatal("Avro record constructor accepted an undeclared field")
	}
	if _, err := normalizeAvroRecord(second, record); err == nil {
		t.Fatal("Avro normalization accepted a record with another schema")
	}
	env := NewEnvironment()
	if err := env.RegisterSchema(first); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(second); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.SendAvro(context.Background(), "SecondAvro", record); err == nil {
		t.Fatal("SendAvro accepted a record with another schema")
	}
	if err := engine.SendAvro(context.Background(), "FirstAvro", nil); err == nil {
		t.Fatal("SendAvro accepted a nil record")
	}
	if _, err := NewAvroRecord(Schema{}); err == nil {
		t.Fatal("Avro record accepted an empty schema")
	}
	mapSchema, err := NewMapSchema("NotAvro", fields)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewAvroRecord(mapSchema); err == nil {
		t.Fatal("Avro record accepted a non-Avro schema")
	}
	if _, err := NewAvroSchema("DynamicAvro", fields, AllowDynamicFields()); err == nil {
		t.Fatal("Avro schema accepted dynamic fields")
	}
}

func TestAvroMapAndJSONInputsNormalizeToRecord(t *testing.T) {
	schema, err := NewAvroSchema("NormalizedAvro", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("qty", reflect.TypeOf(int64(0))),
	})
	if err != nil {
		t.Fatal(err)
	}
	receivedAt := time.Unix(0, 0)
	fromMap, err := newEvent(schema, map[string]any{"symbol": "map", "qty": int(2)}, receivedAt)
	if err != nil {
		t.Fatal(err)
	}
	mapRecord, ok := fromMap.Underlying().(*AvroRecord)
	if !ok || mapRecord.Get("symbol") != "map" || mapRecord.Get("qty") != int64(2) {
		t.Fatalf("map normalization = %#v", fromMap.Underlying())
	}
	fromJSON, err := ParseAvroJSON(schema, []byte(`{"symbol":"json","qty":4}`), receivedAt)
	if err != nil {
		t.Fatal(err)
	}
	jsonRecord, ok := fromJSON.Underlying().(*AvroRecord)
	if !ok || jsonRecord.Get("symbol") != "json" || jsonRecord.Get("qty") != int64(4) {
		t.Fatalf("JSON normalization = %#v", fromJSON.Underlying())
	}
	if _, err := ParseAvroJSON(schema, []byte(`{"symbol":"bad","unknown":1}`), receivedAt); err == nil {
		t.Fatal("Avro JSON accepted an undeclared field")
	}
}

func TestAvroRouteRejectsUndeclaredProjectionAtBuildTime(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "AvroRouteSource", []FieldSpec{FieldDef("value", reflect.TypeOf(int64(0)))}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterAvro(env, "AvroRouteTarget", []FieldSpec{FieldDef("value", reflect.TypeOf(int64(0)))}); err != nil {
		t.Fatal(err)
	}
	query := FromAny(env, "AvroRouteSource").Select(
		Alias("missing", Literal(int64(1))),
	).InsertInto("AvroRouteTarget")
	if _, err := env.Build(query); err == nil {
		t.Fatal("Avro route accepted an undeclared projection")
	}
}
