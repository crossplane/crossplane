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

package signature

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/sigstore/cosign/v2/pkg/oci"

	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/pkg/cosign/attestation"
)

// PayloadProvider is a subset of oci.Signature that only provides the
// Payload() method.
type PayloadProvider interface {
	// Payload fetches the opaque data that is being signed.
	// This will always return data when there is no error.
	Payload() ([]byte, error)
}

// Assert that oci.Signature implements PayloadProvider
var _ PayloadProvider = (oci.Signature)(nil)

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
func AttestationToPayloadJSON(_ context.Context, predicateType string, verifiedAttestation PayloadProvider) ([]byte, string, error) {
	if predicateType == "" {
		return nil, "", errors.New("missing predicate type")
	}
	predicateURI, ok := options.PredicateTypeMap[predicateType]
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
	case options.PredicateCustom:
		payload, err = json.Marshal(statement)
		if err != nil {
			return nil, statement.PredicateType, fmt.Errorf("generating CosignStatement: %w", err)
		}
	case options.PredicateLink:
		var linkStatement in_toto.LinkStatement
		if err := json.Unmarshal(decodedPayload, &linkStatement); err != nil {
			return nil, statement.PredicateType, fmt.Errorf("unmarshaling LinkStatement: %w", err)
		}
		payload, err = json.Marshal(linkStatement)
		if err != nil {
			return nil, statement.PredicateType, fmt.Errorf("marshaling LinkStatement: %w", err)
		}
	case options.PredicateSLSA:
		var slsaProvenanceStatement in_toto.ProvenanceStatementSLSA02
		if err := json.Unmarshal(decodedPayload, &slsaProvenanceStatement); err != nil {
			return nil, statement.PredicateType, fmt.Errorf("unmarshaling ProvenanceStatementSLSA02): %w", err)
		}
		payload, err = json.Marshal(slsaProvenanceStatement)
		if err != nil {
			return nil, statement.PredicateType, fmt.Errorf("marshaling ProvenanceStatementSLSA02: %w", err)
		}
	case options.PredicateSPDX, options.PredicateSPDXJSON:
		var spdxStatement in_toto.SPDXStatement
		if err := json.Unmarshal(decodedPayload, &spdxStatement); err != nil {
			return nil, statement.PredicateType, fmt.Errorf("unmarshaling SPDXStatement: %w", err)
		}
		payload, err = json.Marshal(spdxStatement)
		if err != nil {
			return nil, statement.PredicateType, fmt.Errorf("marshaling SPDXStatement: %w", err)
		}
	case options.PredicateCycloneDX:
		var cyclonedxStatement in_toto.CycloneDXStatement
		if err := json.Unmarshal(decodedPayload, &cyclonedxStatement); err != nil {
			return nil, statement.PredicateType, fmt.Errorf("unmarshaling CycloneDXStatement: %w", err)
		}
		payload, err = json.Marshal(cyclonedxStatement)
		if err != nil {
			return nil, statement.PredicateType, fmt.Errorf("marshaling CycloneDXStatement: %w", err)
		}
	case options.PredicateVuln:
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
