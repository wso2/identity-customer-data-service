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

package cli

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// CDS and the Identity Server call each other over TLS, so each has to trust
// the other's certificate. Both stores on the IS side are Java keystores, and
// keytool is the only supported way to write them — so enablement drives
// keytool rather than reimplementing PKCS#12.
//
// keytool comes with the JRE that runs the Identity Server, which is why it is
// looked for under JAVA_HOME before PATH.
const (
	isKeystore    = isSecurity + "/wso2carbon.p12"
	isTruststore  = isSecurity + "/client-truststore.p12"
	isStoreType   = "PKCS12"
	cdsTrustStore = "trust_store.pem"

	// defaultCertificateOU marks the key pair a pack ships with. It is set by
	// distribution/default-cert.sh; changing one without the other leaves the
	// runtime unable to tell a default certificate from a real one.
	defaultCertificateOU = "Customer Data Service Default"
)

// usingDefaultCertificate reports whether the certificate at path is the one
// the pack shipped with.
//
// That certificate is a convenience for getting started and nothing more:
// every download of a release carries the same private key, so anyone holding
// it can present themselves as this service. An operator has to be able to
// see that it is still in use, which is the only reason the pack marks it.
func usingDefaultCertificate(path string) bool {

	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return false
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	for _, unit := range certificate.Subject.OrganizationalUnit {
		if unit == defaultCertificateOU {
			return true
		}
	}
	return false
}

// findKeytool locates the keytool binary, or reports that it is absent.
func findKeytool() (string, error) {

	if javaHome := strings.TrimSpace(os.Getenv("JAVA_HOME")); javaHome != "" {
		candidate := filepath.Join(javaHome, "bin", "keytool")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	found, err := exec.LookPath("keytool")
	if err != nil {
		return "", fmt.Errorf("keytool was not found on PATH or under JAVA_HOME")
	}
	return found, nil
}

var privateKeyEntry = regexp.MustCompile(`(?m)^([^,\n]+),[^\n]*PrivateKeyEntry`)

// exportISCertificate writes the Identity Server's public certificate to
// target in PEM form, so CDS can verify the server it calls.
func exportISCertificate(keytool string, is *isInstall, storePassword, target string) error {

	keystore := is.path(isKeystore)
	if _, err := os.Stat(keystore); err != nil {
		return fmt.Errorf("the Identity Server keystore %s is not readable: %w", isKeystore, err)
	}

	listing, err := exec.Command(keytool, "-list", "-keystore", keystore,
		"-storetype", isStoreType, "-storepass", storePassword).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not read %s: %w\n%s", isKeystore, err, strings.TrimSpace(string(listing)))
	}
	match := privateKeyEntry.FindSubmatch(listing)
	if match == nil {
		return fmt.Errorf("%s holds no private key entry to export", isKeystore)
	}
	alias := strings.TrimSpace(string(match[1]))

	pem, err := exec.Command(keytool, "-exportcert", "-rfc", "-alias", alias,
		"-keystore", keystore, "-storetype", isStoreType, "-storepass", storePassword).Output()
	if err != nil {
		return fmt.Errorf("could not export the %q certificate from %s: %w", alias, isKeystore, err)
	}
	if len(pem) == 0 {
		return fmt.Errorf("the certificate exported from %s is empty", isKeystore)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", filepath.Dir(target), err)
	}
	if err := writeFileAtomic(target, pem, 0o644); err != nil {
		return fmt.Errorf("could not write %s: %w", target, err)
	}
	info("exported the Identity Server certificate (alias %s)", alias)
	return nil
}

// importCDSCertificate puts the CDS certificate into the Identity Server's
// client truststore, so IS can verify CDS when it calls the sync endpoints.
//
// The alias is replaced rather than added to, so re-enabling after a
// certificate rotation installs the new one instead of leaving both.
func importCDSCertificate(keytool string, is *isInstall, storePassword, certPEM, alias string) error {

	truststore := is.path(isTruststore)
	if _, err := os.Stat(truststore); err != nil {
		return fmt.Errorf("the Identity Server truststore %s is not readable: %w", isTruststore, err)
	}

	// A delete for an absent alias fails, which is the normal first-run case.
	_ = exec.Command(keytool, "-delete", "-alias", alias, "-keystore", truststore,
		"-storetype", isStoreType, "-storepass", storePassword).Run()

	out, err := exec.Command(keytool, "-importcert", "-noprompt", "-trustcacerts",
		"-alias", alias, "-file", certPEM, "-keystore", truststore,
		"-storetype", isStoreType, "-storepass", storePassword).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not import the CDS certificate into %s: %w\n%s",
			isTruststore, err, strings.TrimSpace(string(out)))
	}
	info("imported the CDS certificate into client-truststore.p12 (alias %s)", alias)
	return nil
}
