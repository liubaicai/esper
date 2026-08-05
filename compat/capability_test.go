package compat

import (
	"os"
	"strings"
	"testing"
)

func TestCapabilityManifestArtifactValidates(t *testing.T) {
	file, err := os.Open("capability-manifest.json")
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
