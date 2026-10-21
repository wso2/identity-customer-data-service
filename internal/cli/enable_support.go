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
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

// resolveISHome decides which Identity Server to act on: the one named on the
// command line, otherwise the one already recorded, otherwise one the operator
// is asked for.
//
// Nothing is downloaded or installed on their behalf. An installation that
// does not exist is reported with the version that would satisfy it, which is
// a decision an operator makes, not a side effect of a command.
func resolveISHome(flagValue string, previous *integration) (string, error) {

	if strings.TrimSpace(flagValue) != "" {
		return flagValue, nil
	}
	if previous != nil && previous.ISHome != "" {
		return previous.ISHome, nil
	}
	if !interactive() {
		return "", fmt.Errorf("no Identity Server recorded for this installation.\n" +
			"Pass --is-home <IS_HOME>")
	}

	fmt.Print("\nIdentity Server home (IS_HOME): ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("could not read IS_HOME: %w", err)
	}
	answer := strings.TrimSpace(line)
	if answer == "" {
		return "", fmt.Errorf("no IS_HOME given")
	}
	return answer, nil
}

func interactive() bool {
	info, err := os.Stdin.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

// resolveCDSURL is the address the Identity Server and the Console use to
// reach CDS. It is not necessarily the bind address: CDS may sit behind a name
// that only its callers resolve, which is why the configured server_url is
// preferred over the listener.
func resolveCDSURL(flagValue string, previous *integration, config *runtimeConfig) string {

	if strings.TrimSpace(flagValue) != "" {
		return strings.TrimSuffix(flagValue, "/")
	}
	if url := strings.TrimSpace(config.ServerURL); url != "" && !strings.Contains(url, "${") {
		return strings.TrimSuffix(url, "/")
	}
	return config.localBase()
}

// resolveISURL is the address CDS and this command use to reach the Identity
// Server's APIs, derived from the configured auth server when it is not given.
func resolveISURL(flagValue string, previous *integration, config *runtimeConfig) string {

	if strings.TrimSpace(flagValue) != "" {
		return strings.TrimSuffix(flagValue, "/")
	}
	if config.AuthServer.Host != "" && config.AuthServer.Port != 0 &&
		!strings.Contains(config.AuthServer.Host, "${") {
		return "https://" + net.JoinHostPort(config.AuthServer.Host, strconv.Itoa(config.AuthServer.Port))
	}
	return defaultISManagementURL
}

// syncCredential returns the password the Identity Server presents to the CDS
// sync endpoints, generating one on first enablement.
//
// It is reused across re-enablements, because it is written into two places —
// the IS deployment.toml and the CDS secrets — and regenerating it would leave
// whichever side was not rewritten unable to authenticate.
func syncCredential(cdsHome string) (string, error) {

	secrets, err := loadSecrets(cdsHome)
	if err != nil {
		return "", err
	}
	if existing := secrets["AUTH_SERVER_ADMIN_PASSWORD"]; existing != "" {
		return existing, nil
	}
	return generatedSecret()
}

// renderTOMLSection fills in the deployment.toml section for this deployment.
func renderTOMLSection(cdsBase, isBase, syncPassword string) (string, error) {

	parsed, err := template.New("is-deployment.toml").Parse(isDeploymentSection)
	if err != nil {
		return "", fmt.Errorf("the bundled Identity Server configuration template is unusable: %w", err)
	}

	var out strings.Builder
	err = parsed.Execute(&out, struct {
		CDSBase      string
		ISBase       string
		SyncUser     string
		SyncPassword string
	}{
		CDSBase:      cdsBase,
		ISBase:       isBase,
		SyncUser:     defaultISAdminUser,
		SyncPassword: syncPassword,
	})
	if err != nil {
		return "", fmt.Errorf("could not render the Identity Server configuration: %w", err)
	}
	return out.String(), nil
}

// exchangeCertificates makes each side able to verify the other, and returns
// the path of the exported Identity Server certificate.
//
// The CDS certificate is supplied by the operator rather than generated here,
// so its absence stops only the half of the exchange that needs it; the half
// that lets CDS verify the Identity Server still completes.
func exchangeCertificates(cdsHome string, is *isInstall, config *runtimeConfig, storePassword string) (string, error) {

	keytool, err := findKeytool()
	if err != nil {
		return "", fmt.Errorf("%w — set JAVA_HOME to the JRE that runs the Identity Server", err)
	}

	certDir := path(cdsHome, certsDirInPak)
	trustPEM := filepath.Join(certDir, cdsTrustStore)
	if err := exportISCertificate(keytool, is, storePassword, trustPEM); err != nil {
		return "", err
	}

	cdsCert := filepath.Join(certDir, "server.crt")
	if _, err := os.Stat(cdsCert); err != nil {
		warn("no CDS certificate at %s, so the Identity Server cannot verify CDS yet", cdsCert)
		warn("install the CDS certificate and key, then run `wso2cds enable` again")
		return trustPEM, nil
	}

	alias := "cds-" + hostOf(config.ServerURL)
	if err := importCDSCertificate(keytool, is, storePassword, cdsCert, alias); err != nil {
		return trustPEM, err
	}
	return trustPEM, nil
}

func hostOf(rawURL string) string {

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return "cds"
	}
	return parsed.Hostname()
}

// writeCDSConfig points the CDS configuration at the enabled Identity Server.
func writeCDSConfig(cdsHome string, config *runtimeConfig, cdsBase, isBase string) error {

	parsed, err := url.Parse(isBase)
	if err != nil {
		return fmt.Errorf("--is-url %q is not a URL: %w", isBase, err)
	}
	port := parsed.Port()
	if port == "" {
		port = "443"
	}

	return updateConfig(cdsHome, func(content string) (string, error) {
		updated, err := setScalar(content, "auth_server", "host", parsed.Hostname())
		if err != nil {
			return "", err
		}
		if updated, err = setScalar(updated, "auth_server", "port", port); err != nil {
			return "", err
		}
		// An absolute certificate directory, so the runtime finds the same
		// files whether it was started from the pack or from anywhere else.
		if updated, err = setScalar(updated, "tls", "cert_dir", path(cdsHome, certsDirInPak)); err != nil {
			return "", err
		}
		return setScalar(updated, "", "server_url", cdsBase)
	})
}

// writeClientIDs records the identifiers of the applications provisioning
// created. They are identifiers rather than secrets, so they belong in the
// configuration an operator reads and not in the secrets file.
func writeClientIDs(cdsHome, systemClientID string) error {

	if systemClientID == "" {
		return nil
	}
	return updateConfig(cdsHome, func(content string) (string, error) {
		updated, err := setScalar(content, "auth_server", "client_id", systemClientID)
		if err != nil {
			return "", err
		}
		return setScalar(updated, "auth_server", "introspection_client_id", systemClientID)
	})
}

// updateConfig applies an edit to deployment.yaml, keeping one backup of the
// file as it was before CDS first changed it.
func updateConfig(cdsHome string, edit func(string) (string, error)) error {

	target := path(cdsHome, configFile)
	if _, err := backupFile(target); err != nil {
		return err
	}
	content, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", configFile, err)
	}
	updated, err := edit(string(content))
	if err != nil {
		return fmt.Errorf("could not update %s: %w", configFile, err)
	}
	return writeFileAtomic(target, []byte(updated), 0o644)
}

// adminPassword resolves the Identity Server administrator password from the
// flag, the environment, or a prompt — in that order, so that an automated run
// never blocks and an interactive one never has to put a password on the
// command line.
func adminPassword(options *enableOptions) (string, error) {

	if options.adminPassword != "" {
		return options.adminPassword, nil
	}
	if fromEnv := os.Getenv(isAdminPasswordEnvVar); fromEnv != "" {
		return fromEnv, nil
	}
	if !interactive() {
		return "", fmt.Errorf("no Identity Server administrator password.\n"+
			"Set $%s, or pass --admin-password", isAdminPasswordEnvVar)
	}

	fmt.Printf("    Identity Server password for %q: ", options.adminUser)
	password, err := readSecretLine()
	fmt.Println()
	if err != nil {
		return "", err
	}
	if password == "" {
		return "", fmt.Errorf("no password given")
	}
	options.adminPassword = password
	return password, nil
}

// readSecretLine reads one line without echoing it.
//
// Echo is turned off through stty rather than through a terminal library, to
// keep the distribution's dependency set to what the runtime itself needs. If
// that is not possible the input is still read, with the operator told that it
// will be visible.
func readSecretLine() (string, error) {

	restore := silenceEcho()
	defer restore()

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("could not read the password: %w", err)
	}
	return strings.TrimSpace(line), nil
}

func silenceEcho() func() {

	stty, err := exec.LookPath("stty")
	if err != nil {
		fmt.Print("(visible) ")
		return func() {}
	}

	off := exec.Command(stty, "-echo")
	off.Stdin = os.Stdin
	if err := off.Run(); err != nil {
		fmt.Print("(visible) ")
		return func() {}
	}
	return func() {
		on := exec.Command(stty, "echo")
		on.Stdin = os.Stdin
		_ = on.Run()
	}
}
