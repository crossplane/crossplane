//
// Copyright 2022 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Note(turkenh): This file is copied from https://github.com/sigstore/cosign/blob/ad478088320a3c04a96b3c183bbde2205fff7bbb/pkg/policy/attestation.go#L59
// with little modification to remove the dependency on "github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
// which brings in a lot of dependencies. Keeping the original license above.

package signature

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/in-toto/in-toto-golang/in_toto"
	slsa02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	slsa1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/sigstore/cosign/v2/pkg/cosign/attestation"
	"github.com/sigstore/cosign/v2/pkg/oci"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	predicateCustom    = "custom"
	predicateSLSA      = "slsaprovenance"
	predicateSLSA02    = "slsaprovenance02"
	predicateSLSA1     = "slsaprovenance1"
	predicateSPDX      = "spdx"
	predicateSPDXJSON  = "spdxjson"
	predicateCycloneDX = "cyclonedx"
	predicateLink      = "link"
	predicateVuln      = "vuln"
	predicateOpenVEX   = "openvex"
)

// AttestationToPayloadJSON takes in a verified Attestation (oci.Signature) and
// marshals it into a JSON depending on the payload that's then consumable
// by policy engine like cue, rego, etc.
//
// Anything fed here must have been validated with either
// `VerifyLocalImageAttestations` or `VerifyImageAttestations`
//
// If there's no error, and payload is empty means the predicateType did not
// match the attestation.
// Returns the attestation type (PredicateType) if the payload was decoded
// before the error happened, or in the case the predicateType that was
// requested does not match. This is useful for callers to be able to provide
// better error messages. For example, if there's a typo in the predicateType,
// or the predicateType is not the one they are looking for. Without returning
// this, it's hard for users to know which attestations/predicateTypes were
// inspected.
func attestationToPayloadJSON(_ context.Context, predicateType string, verifiedAttestation oci.Signature) ([]byte, string, error) { //nolint:gocognit // Copied from cosign, see the above note.
	// PredicateTypeMap is the mapping between the predicate `type` option to predicate URI.
	PredicateTypeMap := map[string]string{
		predicateCustom:    attestation.CosignCustomProvenanceV01,
		predicateSLSA:      slsa02.PredicateSLSAProvenance,
		predicateSLSA02:    slsa02.PredicateSLSAProvenance,
		predicateSLSA1:     slsa1.PredicateSLSAProvenance,
		predicateSPDX:      in_toto.PredicateSPDX,
		predicateSPDXJSON:  in_toto.PredicateSPDX,
		predicateCycloneDX: in_toto.PredicateCycloneDX,
		predicateLink:      in_toto.PredicateLinkV1,
		predicateVuln:      attestation.CosignVulnProvenanceV01,
		predicateOpenVEX:   attestation.OpenVexNamespace,
	}

	if predicateType == "" {
		return nil, "", errors.New("missing predicate type")
	}
	predicateURI, ok := PredicateTypeMap[predicateType]
	if !ok {
		// Not a custom one, use it as is.
		predicateURI = predicateType
	}
	var payloadData map[string]interface{}

	p, err := verifiedAttestation.Payload()
	if err != nil {
		return nil, "", fmt.Errorf("getting payload: %w", err)
	}

	err = json.Unmarshal(p, &payloadData)
	if err != nil {
		return nil, "", fmt.Errorf("unmarshaling payload data")
	}

	var decodedPayload []byte
	if val, ok := payloadData["payload"]; ok {
		decodedPayload, err = base64.StdEncoding.DecodeString(val.(string))
		if err != nil {
			return nil, "", fmt.Errorf("decoding payload: %w", err)
		}
	} else {
		return nil, "", fmt.Errorf("could not find payload in payload data")
	}

	// Only apply the policy against the requested predicate type
	var statement in_toto.Statement
	if err := json.Unmarshal(decodedPayload, &statement); err != nil {
		return nil, "", fmt.Errorf("unmarshal in-toto statement: %w", err)
	}
	if statement.PredicateType != predicateURI {
		// This is not the predicate we're looking for, so skip it.
		return nil, statement.PredicateType, nil
	}

	// NB: In many (all?) of these cases, we could just return the
	// 'json.Marshal', but we check for errors here to decorate them
	// with more meaningful error message.
	var payload []byte
	switch predicateType {
	case predicateCustom:
		payload, err = json.Marshal(statement)
		if err != nil {
			return nil, statement.PredicateType, fmt.Errorf("generating CosignStatement: %w", err)
		}
	case predicateLink:
		var linkStatement in_toto.LinkStatement
		if err := json.Unmarshal(decodedPayload, &linkStatement); err != nil {
			return nil, statement.PredicateType, fmt.Errorf("unmarshaling LinkStatement: %w", err)
		}
		payload, err = json.Marshal(linkStatement)
		if err != nil {
			return nil, statement.PredicateType, fmt.Errorf("marshaling LinkStatement: %w", err)
		}
	case predicateSLSA:
		var slsaProvenanceStatement in_toto.ProvenanceStatementSLSA02
		if err := json.Unmarshal(decodedPayload, &slsaProvenanceStatement); err != nil {
			return nil, statement.PredicateType, fmt.Errorf("unmarshaling ProvenanceStatementSLSA02): %w", err)
		}
		payload, err = json.Marshal(slsaProvenanceStatement)
		if err != nil {
			return nil, statement.PredicateType, fmt.Errorf("marshaling ProvenanceStatementSLSA02: %w", err)
		}
	case predicateSPDX, predicateSPDXJSON:
		var spdxStatement in_toto.SPDXStatement
		if err := json.Unmarshal(decodedPayload, &spdxStatement); err != nil {
			return nil, statement.PredicateType, fmt.Errorf("unmarshaling SPDXStatement: %w", err)
		}
		payload, err = json.Marshal(spdxStatement)
		if err != nil {
			return nil, statement.PredicateType, fmt.Errorf("marshaling SPDXStatement: %w", err)
		}
	case predicateCycloneDX:
		var cyclonedxStatement in_toto.CycloneDXStatement
		if err := json.Unmarshal(decodedPayload, &cyclonedxStatement); err != nil {
			return nil, statement.PredicateType, fmt.Errorf("unmarshaling CycloneDXStatement: %w", err)
		}
		payload, err = json.Marshal(cyclonedxStatement)
		if err != nil {
			return nil, statement.PredicateType, fmt.Errorf("marshaling CycloneDXStatement: %w", err)
		}
	case predicateVuln:
		var vulnStatement attestation.CosignVulnStatement
		if err := json.Unmarshal(decodedPayload, &vulnStatement); err != nil {
			return nil, statement.PredicateType, fmt.Errorf("unmarshaling CosignVulnStatement: %w", err)
		}
		payload, err = json.Marshal(vulnStatement)
		if err != nil {
			return nil, statement.PredicateType, fmt.Errorf("marshaling CosignVulnStatement: %w", err)
		}
	default:
		// Valid URI type reaches here.
		payload, err = json.Marshal(statement)
		if err != nil {
			return nil, statement.PredicateType, fmt.Errorf("generating Statement: %w", err)
		}
	}
	return payload, statement.PredicateType, nil
}
