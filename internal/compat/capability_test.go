package compat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapabilityManifestArtifactValidates(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	file, err := os.Open(filepath.Join(repositoryRoot, "testdata", "compat", "capability-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	manifest, err := LoadCapabilityManifest(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Capabilities) < 10 || len(manifest.Cases) < 10 || len(manifest.Mappings) < 10 {
		t.Fatalf("manifest is too small to be a useful traceability artifact: %#v", manifest)
	}
	for _, capability := range manifest.Capabilities {
		for _, reference := range capability.GoRefs {
			if _, err := os.Stat(filepath.Join(repositoryRoot, filepath.FromSlash(reference))); err != nil {
				t.Errorf("capability %q goRef %q: %v", capability.ID, reference, err)
			}
		}
	}
}

func TestCapabilityManifestRejectsUnknownAndDuplicateMappings(t *testing.T) {
	manifest := CapabilityManifest{
		Version:    CapabilityManifestVersion,
		JavaCommit: "java",
		Capabilities: []CapabilityRecord{{
			ID: "cap-a", Level: "S", Phase: 1, Status: "prototype",
		}},
		Cases: []CapabilityCase{{
			ID: "case-a", JavaRuntimeIDs: []string{"runtime-a"}, Status: "mapped",
		}},
		Mappings: []CapabilityMapping{{CapabilityID: "cap-a", CaseID: "case-a"}},
	}
	manifest.Mappings = append(manifest.Mappings, manifest.Mappings[0])
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate mapping") {
		t.Fatalf("duplicate mapping error = %v", err)
	}
	manifest.Mappings = []CapabilityMapping{{CapabilityID: "missing", CaseID: "case-a"}}
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "unknown capability") {
		t.Fatalf("unknown capability error = %v", err)
	}
}

func TestCapabilityManifestRejectsMappedCaseWithoutCapabilityMapping(t *testing.T) {
	manifest := CapabilityManifest{
		Version:    CapabilityManifestVersion,
		JavaCommit: "java",
		Capabilities: []CapabilityRecord{{
			ID: "cap-a", Level: "S", Phase: 1, Status: "prototype",
		}},
		Cases: []CapabilityCase{{
			ID: "case-a", JavaRuntimeIDs: []string{"runtime-a"}, Status: "mapped",
		}},
	}
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "has no capability mapping") {
		t.Fatalf("missing capability mapping error = %v", err)
	}
	manifest.Cases[0].Status = "unmapped"
	manifest.Capabilities[0].Status = "planned"
	if err := manifest.Validate(); err != nil {
		t.Fatalf("explicitly unmapped case under a planned capability should not require a mapping: %v", err)
	}
}

func TestCapabilityManifestAcceptsPartialCaseWithCapabilityMapping(t *testing.T) {
	manifest := CapabilityManifest{
		Version:    CapabilityManifestVersion,
		JavaCommit: "java",
		Capabilities: []CapabilityRecord{{
			ID: "cap-a", Level: "S", Phase: 1, Status: "partial",
		}},
		Cases: []CapabilityCase{{
			ID: "case-a", JavaRuntimeIDs: []string{"runtime-a"}, Status: "partial",
		}},
		Mappings: []CapabilityMapping{{CapabilityID: "cap-a", CaseID: "case-a"}},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("partial case with an explicit capability mapping should validate: %v", err)
	}
}

func TestCapabilityManifestRejectsStaticJavaIDInRuntimeReferences(t *testing.T) {
	manifest := CapabilityManifest{
		Version:    CapabilityManifestVersion,
		JavaCommit: "java",
		Capabilities: []CapabilityRecord{{
			ID: "cap-a", Level: "S", Phase: 1, Status: "prototype",
		}},
		Cases: []CapabilityCase{{
			ID: "case-a", JavaRuntimeIDs: []string{"java-static-a"}, Status: "mapped",
		}},
		Mappings: []CapabilityMapping{{CapabilityID: "cap-a", CaseID: "case-a"}},
	}
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "javaStaticIds") {
		t.Fatalf("static Java runtime id error = %v", err)
	}
}

func TestCapabilityManifestRejectsNonPlannedCapabilityWithoutCaseMapping(t *testing.T) {
	manifest := CapabilityManifest{
		Version:    CapabilityManifestVersion,
		JavaCommit: "java",
		Capabilities: []CapabilityRecord{{
			ID: "cap-a", Level: "S", Phase: 1, Status: "partial",
		}},
	}
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "has no case mapping") {
		t.Fatalf("missing case mapping error = %v", err)
	}
	manifest.Capabilities[0].Status = "planned"
	if err := manifest.Validate(); err != nil {
		t.Fatalf("planned capability should not require a case mapping: %v", err)
	}
}
