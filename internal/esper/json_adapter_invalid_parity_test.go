package esper

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// errAdapterParse mirrors the Java adapter parse-failure message shape
// (MyDateJSONParser wraps ParseException with "Failed to parse: ...").
var errAdapterParse = errors.New("Failed to parse: unparseable date")

// TestJSONFieldAdapterInvalidParity covers EventJsonAdapterInvalid: Java
// rejects adapter misconfigurations at compile time with EPCompileException
// messages; the typed Go surface classifies the same diagnostic contract
// through ErrorCode - a mismatched adapter value type is rejected at
// registration, and an adapter parse failure surfaces the adapter's own
// error through a classified send-time error.
func TestJSONFieldAdapterInvalidParity(t *testing.T) {
	t.Run("adapter type mismatch rejected at registration", func(t *testing.T) {
		// Java: adapter whose parse returns Date on a String-declared field -
		// "mismatches the return type of the parse method". The typed Go
		// adapter carries its value type and the registration rejects the
		// same shape.
		dateAdapter := NewJSONFieldAdapter(func(text string) (time.Time, error) {
			return time.Parse("2006-01-02T15:04:05.000", text)
		}, func(value time.Time) (string, error) {
			return value.UTC().Format("2006-01-02T15:04:05.000"), nil
		})
		_, err := RegisterJSON(NewEnvironment(), "JsonEvent", []FieldSpec{
			FieldDef("mydate", reflect.TypeOf("")),
		}, WithJSONFieldAdapter("mydate", dateAdapter))
		if err == nil {
			t.Fatal("mismatched adapter value type was accepted")
		}
		if !errors.Is(err, ErrorInvalidRule) {
			t.Fatalf("mismatched adapter value type = %v, want %s", err, ErrorInvalidRule)
		}
		if !strings.Contains(err.Error(), "produces") || !strings.Contains(err.Error(), "mydate") {
			t.Fatalf("mismatch message = %v, want the adapter/field type report", err)
		}
	})

	t.Run("adapter parse failure classified at parse time", func(t *testing.T) {
		env := NewEnvironment()
		failing := NewJSONFieldAdapter(func(text string) (time.Time, error) {
			return time.Time{}, errAdapterParse
		}, func(value time.Time) (string, error) {
			return value.UTC().Format("02-01-2006"), nil
		})
		schema, err := RegisterJSON(env, "JsonEvent", []FieldSpec{
			FieldDef("myDate", reflect.TypeOf(time.Time{})),
		}, WithJSONFieldAdapter("myDate", failing))
		if err != nil {
			t.Fatal(err)
		}
		_, err = ParseJSON(schema, []byte(`{"myDate":"not-a-date"}`), time.Unix(0, 0).UTC())
		if err == nil {
			t.Fatal("adapter parse failure was accepted")
		}
		if !errors.Is(err, ErrorInvalidRule) {
			t.Fatalf("adapter parse failure = %v, want %s", err, ErrorInvalidRule)
		}
		if !errors.Is(err, errAdapterParse) {
			t.Fatalf("adapter parse failure = %v, want the adapter error preserved", err)
		}
	})
}
