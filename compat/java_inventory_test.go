package compat

import (
	"os"
	"strings"
	"testing"
)

func TestLoadJavaExecutionInventory(t *testing.T) {
	records, err := LoadJavaExecutionInventory(strings.NewReader(`
{"id":"java-1","outerClass":"Suite","outerClassName":"example.Suite","sourceFile":"Suite.java","status":"ok","runtimeId":"java-runtime-1","variant":"collection:executions()","ordinal":0,"executionClass":"example.Suite$Case","name":"Case","flags":["PERFORMANCE"]}
{"id":"java-2","outerClass":"Abstract","outerClassName":"example.Abstract","sourceFile":"Abstract.java","status":"ignored","reason":"abstract or interface execution class"}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].RuntimeID != "java-runtime-1" || records[1].Status != "ignored" {
		t.Fatalf("records = %#v", records)
	}
}

func TestCheckedInJavaExecutionInventory(t *testing.T) {
	file, err := os.Open("java-execution-inventory.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	records, err := LoadJavaExecutionInventory(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 4140 {
		t.Fatalf("records = %d, want 4140", len(records))
	}
	ok, ignored, errors := 0, 0, 0
	for _, record := range records {
		switch record.Status {
		case "ok":
			ok++
		case "ignored":
			ignored++
		case "error":
			errors++
		}
	}
	if ok != 4136 || ignored != 4 || errors != 0 {
		t.Fatalf("status counts ok=%d ignored=%d errors=%d", ok, ignored, errors)
	}
}

func TestJavaExecutionInventoryRejectsDuplicateRuntimeID(t *testing.T) {
	input := []JavaExecutionInventoryRecord{
		{ID: "java-1", OuterClass: "Suite", OuterClassName: "example.Suite", SourceFile: "Suite.java", Status: "ok", RuntimeID: "duplicate", Variant: "direct", ExecutionClass: "example.Suite"},
		{ID: "java-1", OuterClass: "Suite", OuterClassName: "example.Suite", SourceFile: "Suite.java", Status: "ok", RuntimeID: "duplicate", Variant: "direct", ExecutionClass: "example.Suite"},
	}
	if err := ValidateJavaExecutionInventory(input); err == nil {
		t.Fatal("expected duplicate runtimeId error")
	}
}
