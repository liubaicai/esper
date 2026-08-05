package compat

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// TraceDifference identifies one stable location at which two normalized
// traces disagree. Expected is the oracle value and Actual is the candidate
// value; both are kept as interface values so callers can print or marshal a
// machine-readable report without reparsing a textual diff.
type TraceDifference struct {
	Path     string `json:"path"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
}

// DiffTraces compares normalized Java/Go trace artifacts. Comparison is
// intentionally structural and order-sensitive: callback order, batch
// boundaries, sequence numbers and new/old stream order are observable CEP
// behavior, not formatting details.
func DiffTraces(expected, actual Trace) []TraceDifference {
	differences := make([]TraceDifference, 0)
	compareValue := func(path string, want, got any) {
		if !reflect.DeepEqual(want, got) {
			differences = append(differences, TraceDifference{Path: path, Expected: want, Actual: got})
		}
	}
	compareValue("version", expected.Version, actual.Version)
	compareValue("id", expected.ID, actual.ID)
	compareValue("records.length", len(expected.Records), len(actual.Records))
	limit := len(expected.Records)
	if len(actual.Records) < limit {
		limit = len(actual.Records)
	}
	for index := 0; index < limit; index++ {
		want := expected.Records[index]
		got := actual.Records[index]
		prefix := fmt.Sprintf("records[%d]", index)
		compareValue(prefix+".statement", want.Statement, got.Statement)
		compareValue(prefix+".sequence", want.Sequence, got.Sequence)
		compareValue(prefix+".time", want.Time, got.Time)
		compareResults(&differences, prefix+".new", want.New, got.New)
		compareResults(&differences, prefix+".old", want.Old, got.Old)
	}
	return differences
}

func compareResults(differences *[]TraceDifference, path string, expected, actual []ResultRecord) {
	compareTraceValue(differences, path+".length", len(expected), len(actual))
	limit := len(expected)
	if len(actual) < limit {
		limit = len(actual)
	}
	for index := 0; index < limit; index++ {
		want := expected[index]
		got := actual[index]
		prefix := fmt.Sprintf("%s[%d]", path, index)
		compareTraceValue(differences, prefix+".kind", want.Kind, got.Kind)
		compareTraceValue(differences, prefix+".type", want.Type, got.Type)
		compareTraceValue(differences, prefix+".fields", want.Fields, got.Fields)
	}
}

func compareTraceValue(differences *[]TraceDifference, path string, expected, actual any) {
	if !reflect.DeepEqual(expected, actual) {
		*differences = append(*differences, TraceDifference{Path: path, Expected: expected, Actual: actual})
	}
}

// EqualTrace reports structural equality while also ensuring the values can
// be represented by the parity JSON protocol. The marshal check catches
// unsupported host values early when a test builds a trace programmatically.
func EqualTrace(expected, actual Trace) (bool, error) {
	if _, err := json.Marshal(expected); err != nil {
		return false, fmt.Errorf("compat: marshal expected trace: %w", err)
	}
	if _, err := json.Marshal(actual); err != nil {
		return false, fmt.Errorf("compat: marshal actual trace: %w", err)
	}
	return len(DiffTraces(expected, actual)) == 0, nil
}
