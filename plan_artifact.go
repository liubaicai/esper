package esper

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const maxPlanArtifactBytes = 16 << 20

type PlanArtifact struct {
	SchemaVersion string `json:"schemaVersion"`
	Hash          string `json:"hash"`
	Canonical     []byte `json:"canonical"`
}

func (p Plan) MarshalArtifact() ([]byte, error) {
	if p.schemaVersion == "" || len(p.canonical) == 0 || p.hash == "" {
		return nil, NewError(ErrorState, "cannot serialize an empty plan")
	}
	artifact := PlanArtifact{SchemaVersion: p.schemaVersion, Hash: p.hash, Canonical: p.Canonical()}
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return nil, fmt.Errorf("esper: encode plan artifact: %w", err)
	}
	if len(encoded) > maxPlanArtifactBytes {
		return nil, NewError(ErrorState, "plan artifact exceeds size limit")
	}
	return encoded, nil
}

func LoadPlanArtifact(data []byte) (PlanArtifact, error) {
	if len(data) == 0 || len(data) > maxPlanArtifactBytes {
		return PlanArtifact{}, NewError(ErrorDependency, "plan artifact size is invalid")
	}
	var artifact PlanArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return PlanArtifact{}, WrapError(ErrorDependency, "plan-artifact", err)
	}
	if artifact.SchemaVersion != planSchemaVersion {
		return PlanArtifact{}, NewError(ErrorDependency, fmt.Sprintf("unsupported plan schema version %q", artifact.SchemaVersion))
	}
	digest := sha256.Sum256(artifact.Canonical)
	if hex.EncodeToString(digest[:]) != artifact.Hash {
		return PlanArtifact{}, NewError(ErrorDependency, "plan artifact checksum mismatch")
	}
	return artifact, nil
}
