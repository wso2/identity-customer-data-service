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

package utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
)

// NewOutboundHTTPClient builds an HTTP client with TLS/mTLS configuration for
// outbound requests to an identity provider. It validates the server using
// the configured trust store (falling back to system roots) and, if mTLS is
// enabled, presents CDS's own client certificate. Shared by every
// identity_provider implementation (wso2is, thunder) so their TLS posture
// stays identical.
func NewOutboundHTTPClient(tlsCfg config.TLSConfig, serverHostForSNI string) (*http.Client, error) {
	// Resolve cert dir to absolute to avoid CWD surprises
	certDir := tlsCfg.CertDir
	cdsHome := GetCDSHome()
	if certDir == "" {
		certDir = filepath.Join(cdsHome, "etc", "certs")
	}
	if !filepath.IsAbs(certDir) {
		if abs, err := filepath.Abs(certDir); err == nil {
			certDir = abs
		}
	}

	// Root CAs: start with system roots, then append trust store if provided.
	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	trustFile := tlsCfg.TrustStore

	if trustFile != "" {
		trustPath := filepath.Join(certDir, trustFile)
		trustPEM, err := os.ReadFile(trustPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read trust_store at %s: %w", trustPath, err)
		}
		if ok := rootCAs.AppendCertsFromPEM(trustPEM); !ok {
			return nil, fmt.Errorf("failed to append certs from trust_store: %s", trustPath)
		}
	}

	// Client cert/key for mTLS (optional)
	var certificates []tls.Certificate
	if tlsCfg.MTLSEnabled {
		cdsPublicCrt := filepath.Join(certDir, tlsCfg.CDSPublicCert)
		cdsPrivatekey := filepath.Join(certDir, tlsCfg.CDSPrivateKey)
		pair, err := tls.LoadX509KeyPair(cdsPublicCrt, cdsPrivatekey)
		if err != nil {
			return nil, fmt.Errorf("failed to load cert/key (%s, %s): %w", cdsPublicCrt, cdsPrivatekey, err)
		}
		certificates = []tls.Certificate{pair}
	}

	tcfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      rootCAs,      // nil means use system CA certs
		Certificates: certificates, // empty if mTLS disabled
	}

	tr := &http.Transport{
		TLSClientConfig:     tcfg,
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     60 * time.Second,
		MaxIdleConns:        100,
		MaxConnsPerHost:     100,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}, nil
}
