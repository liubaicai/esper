package compat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

const DifferentialEvidenceVersion = "esper-differential/v1"

// DifferentialEvidence is the durable result of one Java/Go scenario
// comparison. The traces are retained with the scenario and Java references
// so a passing result can be reviewed without reconstructing command-line
// state from a build log.
type DifferentialEvidence struct {
	Version         string            `json:"version"`
	JavaCommit      string            `json:"javaCommit"`
	JavaRuntimeIDs  []string          `json:"javaRuntimeIds"`
	JavaSourceFiles []string          `json:"javaSourceFiles"`
	JavaExecutions  []string          `json:"javaExecutions"`
	Scenario        Scenario          `json:"scenario"`
	JavaTrace       Trace             `json:"javaTrace"`
	GoTrace         Trace             `json:"goTrace"`
	Status          string            `json:"status"`
	Differences     []TraceDifference `json:"differences"`
}

// canonicalizeAnyModeSnapshotRows sorts the rows of snapshot records whose
// scenario step declares mode "any". The oracle records engine-natural row
// order and defers order-insensitivity to the differ: Java's grouped-join
// AggregateGroupedImpl iterator walks the join set in an order no portable
// producer reproduces (the RowPerGroupImpl path adds HashMap group order on
// top), so both traces are canonicalized to the same deterministic row
// order before the positional comparison.
func canonicalizeAnyModeSnapshotRows(scenario Scenario, trace *Trace) {
	type casePlan struct {
		anyFlags []bool
	}
	plans := map[string]*casePlan{}
	current := ""
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			current = step.Case
		case "snapshot":
			plan := plans[current]
			if plan == nil {
				plan = &casePlan{}
				plans[current] = plan
			}
			plan.anyFlags = append(plan.anyFlags, step.Mode == "any")
		}
	}
	if len(plans) == 0 {
		return
	}
	counts := map[string]int{}
	for index := range trace.Records {
		record := &trace.Records[index]
		if record.Operation != "snapshot" {
			continue
		}
		plan := plans[record.Case]
		if plan == nil {
			continue
		}
		nth := counts[record.Case]
		counts[record.Case] = nth + 1
		if nth >= len(plan.anyFlags) || !plan.anyFlags[nth] {
			continue
		}
		sortResultRecordsByCanonicalFields(record.New)
		sortResultRecordsByCanonicalFields(record.Old)
	}
}

func sortResultRecordsByCanonicalFields(rows []ResultRecord) {
	sort.SliceStable(rows, func(left, right int) bool {
		leftJSON, _ := json.Marshal(rows[left].Fields)
		rightJSON, _ := json.Marshal(rows[right].Fields)
		return string(leftJSON) < string(rightJSON)
	})
}

// NewDifferentialEvidence canonicalizes both traces at the JSON protocol
// boundary before comparing them. This preserves integer precision through
// json.Number while making Java and Go numeric values comparable.
func NewDifferentialEvidence(javaCommit string, runtimeIDs, sourceFiles, executions []string, scenario Scenario, javaTrace, goTrace Trace) (DifferentialEvidence, error) {
	if err := scenario.Validate(); err != nil {
		return DifferentialEvidence{}, err
	}
	if strings.TrimSpace(javaCommit) == "" {
		return DifferentialEvidence{}, fmt.Errorf("compat: differential evidence Java commit is required")
	}
	if len(runtimeIDs) == 0 {
		return DifferentialEvidence{}, fmt.Errorf("compat: differential evidence has no Java runtime IDs")
	}
	if len(sourceFiles) == 0 {
		return DifferentialEvidence{}, fmt.Errorf("compat: differential evidence has no Java source files")
	}
	if len(executions) == 0 {
		return DifferentialEvidence{}, fmt.Errorf("compat: differential evidence has no Java executions")
	}
	javaTrace, err := CanonicalTrace(javaTrace)
	if err != nil {
		return DifferentialEvidence{}, fmt.Errorf("compat: canonicalize Java trace: %w", err)
	}
	goTrace, err = CanonicalTrace(goTrace)
	if err != nil {
		return DifferentialEvidence{}, fmt.Errorf("compat: canonicalize Go trace: %w", err)
	}
	if javaTrace.Version != scenario.Version || javaTrace.ID != scenario.ID {
		return DifferentialEvidence{}, fmt.Errorf("compat: Java trace identity does not match scenario")
	}
	if goTrace.Version != scenario.Version || goTrace.ID != scenario.ID {
		return DifferentialEvidence{}, fmt.Errorf("compat: Go trace identity does not match scenario")
	}
	canonicalizeAnyModeSnapshotRows(scenario, &javaTrace)
	canonicalizeAnyModeSnapshotRows(scenario, &goTrace)
	differences := DiffTraces(javaTrace, goTrace)
	differences, err = canonicalizeTraceDifferences(differences)
	if err != nil {
		return DifferentialEvidence{}, fmt.Errorf("compat: canonicalize trace differences: %w", err)
	}
	status := "passing"
	if len(differences) != 0 {
		status = "different"
	}
	return DifferentialEvidence{
		Version:         DifferentialEvidenceVersion,
		JavaCommit:      javaCommit,
		JavaRuntimeIDs:  append([]string(nil), runtimeIDs...),
		JavaSourceFiles: append([]string(nil), sourceFiles...),
		JavaExecutions:  append([]string(nil), executions...),
		Scenario:        scenario,
		JavaTrace:       javaTrace,
		GoTrace:         goTrace,
		Status:          status,
		Differences:     differences,
	}, nil
}

func (e DifferentialEvidence) Validate() error {
	if e.Version != DifferentialEvidenceVersion {
		return fmt.Errorf("compat: unsupported differential evidence version %q", e.Version)
	}
	if strings.TrimSpace(e.JavaCommit) == "" {
		return fmt.Errorf("compat: differential evidence Java commit is required")
	}
	if len(e.JavaRuntimeIDs) == 0 || len(e.JavaSourceFiles) == 0 || len(e.JavaExecutions) == 0 {
		return fmt.Errorf("compat: differential evidence Java references are incomplete")
	}
	if err := e.Scenario.Validate(); err != nil {
		return err
	}
	if e.JavaTrace.Version != e.Scenario.Version || e.JavaTrace.ID != e.Scenario.ID {
		return fmt.Errorf("compat: differential evidence Java trace identity does not match scenario")
	}
	if e.GoTrace.Version != e.Scenario.Version || e.GoTrace.ID != e.Scenario.ID {
		return fmt.Errorf("compat: differential evidence Go trace identity does not match scenario")
	}
	if e.Status != "passing" && e.Status != "different" {
		return fmt.Errorf("compat: differential evidence has invalid status %q", e.Status)
	}
	javaTrace, goTrace := e.JavaTrace, e.GoTrace
	canonicalizeAnyModeSnapshotRows(e.Scenario, &javaTrace)
	canonicalizeAnyModeSnapshotRows(e.Scenario, &goTrace)
	computed, err := canonicalizeTraceDifferences(DiffTraces(javaTrace, goTrace))
	if err != nil {
		return fmt.Errorf("compat: canonicalize computed trace differences: %w", err)
	}
	if !reflect.DeepEqual(computed, e.Differences) {
		return fmt.Errorf("compat: differential evidence differences are stale")
	}
	wantStatus := "passing"
	if len(computed) != 0 {
		wantStatus = "different"
	}
	if e.Status != wantStatus {
		return fmt.Errorf("compat: differential evidence status %q does not match differences", e.Status)
	}
	return nil
}

// LoadDifferentialEvidence decodes and validates a persisted comparison
// artifact. UseNumber keeps numeric trace fields stable across read/write
// cycles.
func LoadDifferentialEvidence(reader io.Reader) (DifferentialEvidence, error) {
	if reader == nil {
		return DifferentialEvidence{}, fmt.Errorf("compat: differential evidence reader is required")
	}
	var evidence DifferentialEvidence
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	if err := decoder.Decode(&evidence); err != nil {
		return DifferentialEvidence{}, fmt.Errorf("compat: decode differential evidence: %w", err)
	}
	if err := evidence.Validate(); err != nil {
		return DifferentialEvidence{}, err
	}
	return evidence, nil
}

// LoadTrace decodes one normalized trace artifact from JSON and validates its
// top-level protocol identity. UseNumber keeps large integer fields exact.
func LoadTrace(reader io.Reader) (Trace, error) {
	if reader == nil {
		return Trace{}, fmt.Errorf("compat: trace reader is required")
	}
	var trace Trace
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	if err := decoder.Decode(&trace); err != nil {
		return Trace{}, fmt.Errorf("compat: decode trace: %w", err)
	}
	if err := validateTraceIdentity(trace); err != nil {
		return Trace{}, err
	}
	return trace, nil
}

// CanonicalTrace converts in-memory trace values to the same JSON number
// representation used by LoadTrace. It is useful when one side of a
// comparison was produced directly by a Go runner.
func CanonicalTrace(trace Trace) (Trace, error) {
	data, err := json.Marshal(trace)
	if err != nil {
		return Trace{}, err
	}
	return LoadTrace(bytes.NewReader(data))
}

func canonicalizeTraceDifferences(differences []TraceDifference) ([]TraceDifference, error) {
	data, err := json.Marshal(differences)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var canonical []TraceDifference
	if err := decoder.Decode(&canonical); err != nil {
		return nil, err
	}
	return canonical, nil
}

func validateTraceIdentity(trace Trace) error {
	if trace.Version != ScenarioVersion {
		return fmt.Errorf("compat: unsupported trace version %q", trace.Version)
	}
	if strings.TrimSpace(trace.ID) == "" {
		return fmt.Errorf("compat: trace id is required")
	}
	return nil
}
