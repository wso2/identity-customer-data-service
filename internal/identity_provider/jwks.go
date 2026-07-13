/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package identity_provider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"

	"crypto/rsa"
)

// JWK is a single entry of a standard JSON Web Key Set (RFC 7517), limited to
// the RSA fields CDS's identity providers are expected to publish.
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKSet is a JWKS document as returned by a provider's jwks endpoint.
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// RSAPublicKeyFromJWKS parses a JWKS document and returns the RSA public key
// matching kid. Used by both the wso2is and thunder clients' VerifyJWT.
func RSAPublicKeyFromJWKS(jwksJSON []byte, kid string) (*rsa.PublicKey, error) {
	var set JWKSet
	if err := json.Unmarshal(jwksJSON, &set); err != nil {
		return nil, fmt.Errorf("failed to parse JWKS response: %w", err)
	}

	for _, key := range set.Keys {
		if key.Kid != kid || key.Kty != "RSA" {
			continue
		}
		return rsaPublicKeyFromModulusExponent(key.N, key.E)
	}
	return nil, fmt.Errorf("no matching RSA key found in JWKS for kid %q", kid)
}

func rsaPublicKeyFromModulusExponent(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWK modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWK exponent: %w", err)
	}

	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}, nil
}
