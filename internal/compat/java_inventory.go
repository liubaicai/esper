package compat

import (
	"encoding/json"
	"fmt"
	"io"
)

// JavaExecutionInventoryRecord is one runtime-discovered Java regression
// execution. The record deliberately keeps ignored entries: they explain why
// a static candidate is not part of the executable oracle set.
type JavaExecutionInventoryRecord struct {
	ID             string   `json:"id"`
	RuntimeID      string   `json:"runtimeId,omitempty"`
	OuterClass     string   `json:"outerClass"`
	OuterClassName string   `json:"outerClassName"`
	SourceFile     string   `json:"sourceFile"`
	Status         string   `json:"status"`
	Variant        string   `json:"variant,omitempty"`
	Ordinal        int      `json:"ordinal,omitempty"`
	ExecutionClass string   `json:"executionClass,omitempty"`
	Name           string   `json:"name,omitempty"`
	Flags          []string `json:"flags,omitempty"`
	Reason         string   `json:"reason,omitempty"`
	Error          string   `json:"error,omitempty"`
}

// LoadJavaExecutionInventory reads the JSONL produced by the Java oracle
// probe. JSONL is used instead of one large JSON array so a failed/ignored
// outer suite remains independently inspectable.
func LoadJavaExecutionInventory(reader io.Reader) ([]JavaExecutionInventoryRecord, error) {
	if reader == nil {
		return nil, fmt.Errorf("compat: nil Java inventory reader")
	}
	decoder := json.NewDecoder(reader)
	records := make([]JavaExecutionInventoryRecord, 0)
	for {
		var record JavaExecutionInventoryRecord
		err := decoder.Decode(&record)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("compat: decode Java inventory: %w", err)
		}
		records = append(records, record)
	}
	if err := ValidateJavaExecutionInventory(records); err != nil {
		return nil, err
	}
	return records, nil
}

// ValidateJavaExecutionInventory checks the stable identity contract without
// imposing a particular number of Java cases on callers using another Esper
// source revision.
func ValidateJavaExecutionInventory(records []JavaExecutionInventoryRecord) error {
	seenRuntimeIDs := make(map[string]struct{})
	for index, record := range records {
		if record.ID == "" || record.OuterClass == "" || record.OuterClassName == "" || record.SourceFile == "" {
			return fmt.Errorf("compat: Java inventory record %d is missing source identity", index)
		}
		switch record.Status {
		case "ok":
			if record.RuntimeID == "" || record.ExecutionClass == "" || record.Variant == "" {
				return fmt.Errorf("compat: Java inventory record %d is missing runtime identity", index)
			}
			if _, exists := seenRuntimeIDs[record.RuntimeID]; exists {
				return fmt.Errorf("compat: duplicate Java runtimeId %q", record.RuntimeID)
			}
			seenRuntimeIDs[record.RuntimeID] = struct{}{}
		case "ignored":
			if record.Reason == "" {
				return fmt.Errorf("compat: Java inventory ignored record %d has no reason", index)
			}
		case "error":
			if record.Error == "" {
				return fmt.Errorf("compat: Java inventory error record %d has no error", index)
			}
		default:
			return fmt.Errorf("compat: Java inventory record %d has unsupported status %q", index, record.Status)
		}
	}
	return nil
}
