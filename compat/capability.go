package compat

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// CapabilityManifestVersion is the version of the machine-readable
// capability-to-test traceability artifact. It deliberately differs from the
// static and runtime Java inventories: this file records Go disposition and
// evidence, not just discovery.
const CapabilityManifestVersion = "esper-capabilities/v1"

// CapabilityManifest links product capabilities, Java oracle cases and Go
// evidence. A capability may map to many cases and a case may exercise many
// capabilities.
type CapabilityManifest struct {
	Version            string              `json:"version"`
	JavaCommit         string              `json:"javaCommit"`
	StaticManifest     string              `json:"staticManifest"`
	ExecutionInventory string              `json:"executionInventory"`
	SourceTestManifest string              `json:"sourceTestManifest"`
	Capabilities       []CapabilityRecord  `json:"capabilities"`
	Cases              []CapabilityCase    `json:"cases"`
	Mappings           []CapabilityMapping `json:"mappings"`
}

type CapabilityRecord struct {
	ID        string   `json:"id"`
	Domain    string   `json:"domain"`
	Level     string   `json:"level"`
	Summary   string   `json:"summary"`
	JavaRefs  []string `json:"javaRefs,omitempty"`
	GoRefs    []string `json:"goRefs,omitempty"`
	DependsOn []string `json:"dependsOn,omitempty"`
	Phase     int      `json:"phase"`
	Status    string   `json:"status"`
	Remaining []string `json:"remaining,omitempty"`
}

type CapabilityCase struct {
	ID              string   `json:"id"`
	JavaRuntimeIDs  []string `json:"javaRuntimeIds,omitempty"`
	JavaNames       []string `json:"javaNames,omitempty"`
	JavaSourceFiles []string `json:"javaSourceFiles,omitempty"`
	GoTests         []string `json:"goTests,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Status          string   `json:"status"`
	Evidence        []string `json:"evidence,omitempty"`
	Difference      string   `json:"difference,omitempty"`
}

type CapabilityMapping struct {
	CapabilityID string `json:"capabilityId"`
	CaseID       string `json:"caseId"`
}

func LoadCapabilityManifest(reader io.Reader) (CapabilityManifest, error) {
	if reader == nil {
		return CapabilityManifest{}, fmt.Errorf("compat: capability manifest reader is required")
	}
	var manifest CapabilityManifest
	if err := json.NewDecoder(reader).Decode(&manifest); err != nil {
		return CapabilityManifest{}, fmt.Errorf("compat: decode capability manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return CapabilityManifest{}, err
	}
	return manifest, nil
}

func (m CapabilityManifest) Validate() error {
	if m.Version != CapabilityManifestVersion {
		return fmt.Errorf("compat: unsupported capability manifest version %q", m.Version)
	}
	if strings.TrimSpace(m.JavaCommit) == "" {
		return fmt.Errorf("compat: capability manifest Java commit is required")
	}
	if len(m.Capabilities) == 0 {
		return fmt.Errorf("compat: capability manifest has no capabilities")
	}
	capabilities := make(map[string]struct{}, len(m.Capabilities))
	for index, capability := range m.Capabilities {
		if err := validateID("capability", capability.ID); err != nil {
			return fmt.Errorf("compat: capability %d: %w", index, err)
		}
		if _, exists := capabilities[capability.ID]; exists {
			return fmt.Errorf("compat: duplicate capability id %q", capability.ID)
		}
		capabilities[capability.ID] = struct{}{}
		if capability.Level != "S" && capability.Level != "G" && capability.Level != "C" && capability.Level != "N" {
			return fmt.Errorf("compat: capability %q has invalid level %q", capability.ID, capability.Level)
		}
		if capability.Phase < 0 {
			return fmt.Errorf("compat: capability %q has negative phase", capability.ID)
		}
		if strings.TrimSpace(capability.Status) == "" {
			return fmt.Errorf("compat: capability %q has no status", capability.ID)
		}
	}
	cases := make(map[string]struct{}, len(m.Cases))
	for index, parityCase := range m.Cases {
		if err := validateID("case", parityCase.ID); err != nil {
			return fmt.Errorf("compat: case %d: %w", index, err)
		}
		if _, exists := cases[parityCase.ID]; exists {
			return fmt.Errorf("compat: duplicate case id %q", parityCase.ID)
		}
		cases[parityCase.ID] = struct{}{}
		if !validCaseStatus(parityCase.Status) {
			return fmt.Errorf("compat: case %q has invalid status %q", parityCase.ID, parityCase.Status)
		}
		if len(parityCase.JavaRuntimeIDs) == 0 && len(parityCase.JavaSourceFiles) == 0 {
			return fmt.Errorf("compat: case %q has no Java reference", parityCase.ID)
		}
	}
	pairs := make(map[string]struct{}, len(m.Mappings))
	mappedCases := make(map[string]struct{}, len(m.Mappings))
	for index, mapping := range m.Mappings {
		if _, exists := capabilities[mapping.CapabilityID]; !exists {
			return fmt.Errorf("compat: mapping %d references unknown capability %q", index, mapping.CapabilityID)
		}
		if _, exists := cases[mapping.CaseID]; !exists {
			return fmt.Errorf("compat: mapping %d references unknown case %q", index, mapping.CaseID)
		}
		key := mapping.CapabilityID + "\x00" + mapping.CaseID
		if _, exists := pairs[key]; exists {
			return fmt.Errorf("compat: duplicate mapping %q -> %q", mapping.CapabilityID, mapping.CaseID)
		}
		pairs[key] = struct{}{}
		mappedCases[mapping.CaseID] = struct{}{}
	}
	for _, parityCase := range m.Cases {
		if parityCase.Status == "unmapped" {
			continue
		}
		if _, mapped := mappedCases[parityCase.ID]; !mapped {
			return fmt.Errorf("compat: case %q with status %q has no capability mapping", parityCase.ID, parityCase.Status)
		}
	}
	return nil
}

func validateID(kind, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s id is required", kind)
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("%s id %q contains whitespace", kind, value)
	}
	return nil
}

func validCaseStatus(status string) bool {
	switch status {
	case "unmapped", "mapped", "passing", "blocked", "approved-difference":
		return true
	default:
		return false
	}
}
