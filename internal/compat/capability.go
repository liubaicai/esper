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
const CapabilityManifestVersion = "esper-capabilities/v2"

const (
	VerificationInventoried            = "inventoried"
	VerificationImplemented            = "implemented"
	VerificationDifferentialVerified   = "differential-verified"
	VerificationIntentionallyDifferent = "intentionally-different"
	VerificationRepresentativeVerified = "representative-verified"
	VerificationNFRVerified            = "nfr-verified"
)

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
	Summary            CapabilitySummary   `json:"summary"`
}

type CapabilityRecord struct {
	ID                             string   `json:"id"`
	Domain                         string   `json:"domain"`
	Level                          string   `json:"level"`
	Summary                        string   `json:"summary"`
	JavaRefs                       []string `json:"javaRefs,omitempty"`
	GoRefs                         []string `json:"goRefs,omitempty"`
	DependsOn                      []string `json:"dependsOn,omitempty"`
	Phase                          int      `json:"phase"`
	Status                         string   `json:"status"`
	Remaining                      []string `json:"remaining,omitempty"`
	Difference                     string   `json:"difference,omitempty"`
	Verification                   []string `json:"verification"`
	DifferentialVerifiedRuntimeIDs []string `json:"differentialVerifiedRuntimeIds,omitempty"`
	RepresentativeScenarioIDs      []string `json:"representativeScenarioIds,omitempty"`
	NFRChecks                      []string `json:"nfrChecks,omitempty"`
}

type CapabilityCase struct {
	ID                             string   `json:"id"`
	JavaRuntimeIDs                 []string `json:"javaRuntimeIds,omitempty"`
	JavaStaticIDs                  []string `json:"javaStaticIds,omitempty"`
	JavaNames                      []string `json:"javaNames,omitempty"`
	JavaSourceFiles                []string `json:"javaSourceFiles,omitempty"`
	GoTests                        []string `json:"goTests,omitempty"`
	Tags                           []string `json:"tags,omitempty"`
	Status                         string   `json:"status"`
	Evidence                       []string `json:"evidence,omitempty"`
	Difference                     string   `json:"difference,omitempty"`
	Verification                   []string `json:"verification"`
	DifferentialVerifiedRuntimeIDs []string `json:"differentialVerifiedRuntimeIds,omitempty"`
	RepresentativeScenarioIDs      []string `json:"representativeScenarioIds,omitempty"`
	NFRChecks                      []string `json:"nfrChecks,omitempty"`
}

type CapabilityMapping struct {
	CapabilityID string `json:"capabilityId"`
	CaseID       string `json:"caseId"`
}

// CapabilitySummary is intentionally derived from the manifest's records,
// rather than from hand-maintained progress prose. Runtime counts are split
// into association count and distinct runtime IDs because one runtime may be
// exercised by more than one capability case.
type CapabilitySummary struct {
	TotalCases                         int            `json:"totalCases"`
	TotalCapabilities                  int            `json:"totalCapabilities"`
	InventoriedCases                   int            `json:"inventoriedCases"`
	ImplementedCases                   int            `json:"implementedCases"`
	DifferentialVerifiedCases          int            `json:"differentialVerifiedCases"`
	DifferentialVerifiedRuntimeIDs     int            `json:"differentialVerifiedRuntimeIds"`
	IntentionallyDifferentCases        int            `json:"intentionallyDifferentCases"`
	RepresentativeVerifiedCases        int            `json:"representativeVerifiedCases"`
	NFRVerifiedCases                   int            `json:"nfrVerifiedCases"`
	InventoriedCapabilities            int            `json:"inventoriedCapabilities"`
	ImplementedCapabilities            int            `json:"implementedCapabilities"`
	DifferentialVerifiedCapabilities   int            `json:"differentialVerifiedCapabilities"`
	IntentionallyDifferentCapabilities int            `json:"intentionallyDifferentCapabilities"`
	RepresentativeVerifiedCapabilities int            `json:"representativeVerifiedCapabilities"`
	NFRVerifiedCapabilities            int            `json:"nfrVerifiedCapabilities"`
	RuntimeAssociationCount            int            `json:"runtimeAssociationCount"`
	ReferencedJavaRuntimeIDs           int            `json:"referencedJavaRuntimeIds"`
	TotalJavaRuntimeIDs                int            `json:"totalJavaRuntimeIds"`
	UnreferencedJavaRuntimeIDs         int            `json:"unreferencedJavaRuntimeIds"`
	RepresentativeScenarioTotal        int            `json:"representativeScenarioTotal"`
	RepresentativeScenarioPassing      int            `json:"representativeScenarioPassing"`
	CurrentFailures                    []string       `json:"currentFailures,omitempty"`
	CurrentSkips                       []string       `json:"currentSkips,omitempty"`
	Risks                              []string       `json:"risks,omitempty"`
	Quality                            QualitySummary `json:"quality"`
}

type QualitySummary struct {
	Race        string `json:"race"`
	Docker      string `json:"docker"`
	Stress      string `json:"stress"`
	Performance string `json:"performance"`
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
		if err := validateVerification("capability", capability.ID, capability.Status, capability.Verification, capability.DifferentialVerifiedRuntimeIDs, capability.RepresentativeScenarioIDs, capability.NFRChecks, capability.Remaining, capability.Difference); err != nil {
			return err
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
		if !validCurrentStatus(parityCase.Status) {
			return fmt.Errorf("compat: case %q has invalid status %q", parityCase.ID, parityCase.Status)
		}
		if err := validateVerification("case", parityCase.ID, parityCase.Status, parityCase.Verification, parityCase.DifferentialVerifiedRuntimeIDs, parityCase.RepresentativeScenarioIDs, parityCase.NFRChecks, nil, parityCase.Difference); err != nil {
			return err
		}
		if len(parityCase.JavaRuntimeIDs) == 0 && len(parityCase.JavaStaticIDs) == 0 && len(parityCase.JavaSourceFiles) == 0 {
			return fmt.Errorf("compat: case %q has no Java reference", parityCase.ID)
		}
		for _, runtimeID := range parityCase.JavaRuntimeIDs {
			if strings.HasPrefix(runtimeID, "java-") && !strings.HasPrefix(runtimeID, "java-runtime-") {
				return fmt.Errorf("compat: case %q places static Java id %q in javaRuntimeIds; use javaStaticIds", parityCase.ID, runtimeID)
			}
		}
		knownRuntimeIDs := make(map[string]struct{}, len(parityCase.JavaRuntimeIDs))
		for _, runtimeID := range parityCase.JavaRuntimeIDs {
			knownRuntimeIDs[runtimeID] = struct{}{}
		}
		for _, runtimeID := range parityCase.DifferentialVerifiedRuntimeIDs {
			if _, ok := knownRuntimeIDs[runtimeID]; !ok {
				return fmt.Errorf("compat: case %q differential runtime %q is not listed in javaRuntimeIds", parityCase.ID, runtimeID)
			}
		}
	}
	pairs := make(map[string]struct{}, len(m.Mappings))
	mappedCases := make(map[string]struct{}, len(m.Mappings))
	mappedCapabilities := make(map[string]struct{}, len(m.Mappings))
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
		mappedCapabilities[mapping.CapabilityID] = struct{}{}
	}
	for _, parityCase := range m.Cases {
		if parityCase.Status == VerificationInventoried {
			continue
		}
		if _, mapped := mappedCases[parityCase.ID]; !mapped {
			return fmt.Errorf("compat: case %q with status %q has no capability mapping", parityCase.ID, parityCase.Status)
		}
	}
	for _, capability := range m.Capabilities {
		if capability.Status == VerificationInventoried {
			continue
		}
		if _, mapped := mappedCapabilities[capability.ID]; !mapped {
			return fmt.Errorf("compat: capability %q with status %q has no case mapping", capability.ID, capability.Status)
		}
	}
	if err := m.validateSummary(); err != nil {
		return err
	}
	return nil
}

// ValidateAgainstJavaInventory checks the runtime references and the inventory
// denominator used by the progress report. It is kept separate from Validate
// so small in-memory manifests remain useful in unit tests.
func (m CapabilityManifest) ValidateAgainstJavaInventory(records []JavaExecutionInventoryRecord) error {
	if err := m.Validate(); err != nil {
		return err
	}
	known := make(map[string]struct{})
	for _, record := range records {
		if record.Status == "ok" {
			known[record.RuntimeID] = struct{}{}
		}
	}
	referenced := make(map[string]struct{})
	for _, parityCase := range m.Cases {
		for _, runtimeID := range parityCase.JavaRuntimeIDs {
			if _, ok := known[runtimeID]; !ok {
				return fmt.Errorf("compat: case %q references unknown Java runtime %q", parityCase.ID, runtimeID)
			}
			referenced[runtimeID] = struct{}{}
		}
	}
	if m.Summary.TotalJavaRuntimeIDs != len(known) || m.Summary.ReferencedJavaRuntimeIDs != len(referenced) {
		return fmt.Errorf("compat: capability manifest Java inventory counts are stale")
	}
	if m.Summary.UnreferencedJavaRuntimeIDs != len(known)-len(referenced) {
		return fmt.Errorf("compat: capability manifest unreferenced Java runtime count is stale")
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

func validCurrentStatus(status string) bool {
	switch status {
	case VerificationInventoried, VerificationImplemented, VerificationDifferentialVerified,
		VerificationIntentionallyDifferent, VerificationRepresentativeVerified, VerificationNFRVerified:
		return true
	default:
		return false
	}
}

func validateVerification(kind, id, status string, verification, differentialRuntimeIDs, representativeScenarioIDs, nfrChecks, remaining []string, difference string) error {
	if len(verification) == 0 {
		return fmt.Errorf("compat: %s %q has no verification statuses", kind, id)
	}
	seen := make(map[string]struct{}, len(verification))
	for _, value := range verification {
		switch value {
		case VerificationInventoried, VerificationImplemented, VerificationDifferentialVerified,
			VerificationIntentionallyDifferent, VerificationRepresentativeVerified, VerificationNFRVerified:
		default:
			return fmt.Errorf("compat: %s %q has invalid verification status %q", kind, id, value)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("compat: %s %q repeats verification status %q", kind, id, value)
		}
		seen[value] = struct{}{}
	}
	if _, ok := seen[status]; !ok {
		return fmt.Errorf("compat: %s %q current status %q is absent from verification statuses", kind, id, status)
	}
	if _, ok := seen[VerificationInventoried]; !ok {
		return fmt.Errorf("compat: %s %q is not marked inventoried", kind, id)
	}
	if len(differentialRuntimeIDs) > 0 {
		if _, ok := seen[VerificationDifferentialVerified]; !ok {
			return fmt.Errorf("compat: %s %q lists differential runtimes without differential-verified status", kind, id)
		}
	}
	if _, ok := seen[VerificationDifferentialVerified]; ok && len(differentialRuntimeIDs) == 0 {
		return fmt.Errorf("compat: %s %q is differential-verified without runtime IDs", kind, id)
	}
	if _, ok := seen[VerificationRepresentativeVerified]; ok && len(representativeScenarioIDs) == 0 {
		return fmt.Errorf("compat: %s %q is representative-verified without scenario IDs", kind, id)
	}
	if _, ok := seen[VerificationNFRVerified]; ok && len(nfrChecks) == 0 {
		return fmt.Errorf("compat: %s %q is nfr-verified without NFR checks", kind, id)
	}
	if _, ok := seen[VerificationIntentionallyDifferent]; ok && strings.TrimSpace(difference) == "" {
		return fmt.Errorf("compat: %s %q is intentionally-different without a difference rationale", kind, id)
	}
	if status == VerificationImplemented && len(remaining) == 0 && kind == "capability" {
		// A capability may be fully implemented, but its independent
		// differential/representative/NFR stages still remain explicit.
		return nil
	}
	return nil
}

func (m CapabilityManifest) validateSummary() error {
	if m.Summary.TotalCases == 0 && m.Summary.TotalCapabilities == 0 {
		return nil
	}
	if m.Summary.TotalCases != len(m.Cases) || m.Summary.TotalCapabilities != len(m.Capabilities) {
		return fmt.Errorf("compat: capability manifest summary totals are stale")
	}
	caseStatuses := make(map[string]int)
	caseRuntimeIDs := make(map[string]struct{})
	differentialRuntimeIDs := make(map[string]struct{})
	associationCount := 0
	for _, parityCase := range m.Cases {
		caseStatuses[parityCase.Status]++
		associationCount += len(parityCase.JavaRuntimeIDs)
		for _, runtimeID := range parityCase.JavaRuntimeIDs {
			caseRuntimeIDs[runtimeID] = struct{}{}
		}
		for _, runtimeID := range parityCase.DifferentialVerifiedRuntimeIDs {
			differentialRuntimeIDs[runtimeID] = struct{}{}
		}
	}
	countStatus := func(status string) int { return caseStatuses[status] }
	if m.Summary.InventoriedCases != countStatus(VerificationInventoried)+countStatus(VerificationImplemented)+countStatus(VerificationDifferentialVerified)+countStatus(VerificationIntentionallyDifferent)+countStatus(VerificationRepresentativeVerified)+countStatus(VerificationNFRVerified) ||
		m.Summary.ImplementedCases != countVerification(m.Cases, VerificationImplemented) ||
		m.Summary.DifferentialVerifiedCases != countVerification(m.Cases, VerificationDifferentialVerified) ||
		m.Summary.RepresentativeVerifiedCases != countVerification(m.Cases, VerificationRepresentativeVerified) ||
		m.Summary.NFRVerifiedCases != countVerification(m.Cases, VerificationNFRVerified) ||
		m.Summary.DifferentialVerifiedRuntimeIDs != len(differentialRuntimeIDs) ||
		m.Summary.RuntimeAssociationCount != associationCount ||
		m.Summary.ReferencedJavaRuntimeIDs != len(caseRuntimeIDs) ||
		m.Summary.IntentionallyDifferentCases != countStatus(VerificationIntentionallyDifferent) {
		return fmt.Errorf("compat: capability manifest summary verification counts are stale")
	}
	capabilityStatuses := make(map[string]int)
	for _, capability := range m.Capabilities {
		for _, verification := range capability.Verification {
			capabilityStatuses[verification]++
		}
	}
	if m.Summary.InventoriedCapabilities != capabilityStatuses[VerificationInventoried] ||
		m.Summary.ImplementedCapabilities != capabilityStatuses[VerificationImplemented] ||
		m.Summary.DifferentialVerifiedCapabilities != capabilityStatuses[VerificationDifferentialVerified] ||
		m.Summary.IntentionallyDifferentCapabilities != capabilityStatuses[VerificationIntentionallyDifferent] ||
		m.Summary.RepresentativeVerifiedCapabilities != capabilityStatuses[VerificationRepresentativeVerified] ||
		m.Summary.NFRVerifiedCapabilities != capabilityStatuses[VerificationNFRVerified] {
		return fmt.Errorf("compat: capability manifest summary capability verification counts are stale")
	}
	return nil
}

func countVerification(records []CapabilityCase, status string) int {
	count := 0
	for _, record := range records {
		for _, value := range record.Verification {
			if value == status {
				count++
				break
			}
		}
	}
	return count
}
