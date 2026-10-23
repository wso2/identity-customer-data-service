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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// checks accumulates the result of a verification run.
type checks struct {
	run    int
	failed int
}

func (c *checks) pass(format string, a ...any) {
	c.run++
	done(format, a...)
}

func (c *checks) fail(format string, a ...any) {
	c.run++
	c.failed++
	fmt.Fprintf(os.Stderr, "    %s %s\n", red("x"), fmt.Sprintf(format, a...))
}

// runVerify checks the deployment without changing it.
//
// Each check is one thing an operator would otherwise test by hand, and they
// run in dependency order so the first failure is the useful one: a readiness
// failure explains every check after it, and reporting those as separate
// failures would only bury the cause.
func runVerify(args []string) error {

	flags := newFlagSet("verify")
	deep := flags.Bool("deep", false, "additionally run an end-to-end synchronisation test, which creates and removes a temporary user")
	insecure := flags.Bool("insecure", false, "do not verify the Identity Server certificate")
	if err := parse(flags, args); err != nil {
		return err
	}

	cdsHome, err := home()
	if err != nil {
		return err
	}
	state, err := loadIntegration(cdsHome)
	if err != nil {
		return err
	}
	config, err := readRuntimeConfig(cdsHome)
	if err != nil {
		return err
	}
	secrets, err := loadSecrets(cdsHome)
	if err != nil {
		return err
	}

	results := &checks{}
	base := config.localBase()

	step("CDS runtime")
	if status, _, err := probe(base, "/cds/api/v1/ready", 10*time.Second); err != nil {
		results.fail("CDS is not reachable at %s: %v", base, err)
		return summarise(results)
	} else if status != http.StatusOK {
		results.fail("GET /cds/api/v1/ready returned HTTP %d, so the datastore is not ready", status)
		return summarise(results)
	}
	results.pass("GET /cds/api/v1/ready returns 200, so the datastore is reachable and initialised")

	step("Identity Server integration")
	isBase := resolveISURL("", state, config)
	trustPEM := path(cdsHome, certsDirInPak+"/"+cdsTrustStore)
	if _, err := os.Stat(trustPEM); err != nil {
		trustPEM = ""
	}

	verifyCertificate(results, cdsHome, config)
	verifyExtensionState(results, cdsHome, state)

	// The integration's own application, as enablement recorded it: the id in
	// the configuration an operator can read, the secret in the protected
	// file beside it.
	token, err := cdsToken(isBase, config.AuthServer.ClientID, secrets["AUTH_SERVER_CLIENT_SECRET"],
		cdsAPIScopesFor(state), trustPEM, *insecure)
	if err != nil {
		results.fail("could not obtain a token for %q: %v", systemAppName, err)
		return summarise(results)
	}
	verifyTokenAudience(results, token)

	client := &cdsClient{base: base, organization: state.Organization, token: token}
	verifyAPIAccess(results, client)

	if *deep {
		step("End-to-end synchronisation")
		warn("--deep creates and removes a temporary Identity Server user")
		verifyDeep(results, client)
	}

	return summarise(results)
}

// verifyExtensionState confirms the bundles enablement installed are still in
// the Identity Server's dropins, and records what is active.
//
// A staged jar is not active until the Identity Server restarts, so a pack
// whose recorded staged version is ahead of its active one is reported as
// owing a restart rather than as broken.
func verifyExtensionState(results *checks, cdsHome string, state *integration) {

	// Both lists have to be there: the ones CDS installed, and the ones the
	// Identity Server provided. A bundle that vanished is the same problem
	// whichever stream put it there.
	expected := append(append([]string{}, state.Extensions...), state.BundledExtensions...)
	if len(expected) == 0 {
		results.fail("no CDS extension bundles are recorded for %s, so enablement did not complete",
			state.ISHome)
		return
	}

	missing := make([]string, 0, len(expected))
	for _, name := range expected {
		target := filepath.Join(state.ISHome, filepath.FromSlash(isDropins), filepath.Base(name))
		if _, err := os.Stat(target); err != nil {
			missing = append(missing, filepath.Base(name))
		}
	}
	if len(missing) > 0 {
		results.fail("the Identity Server is missing %d CDS bundle(s): %s",
			len(missing), strings.Join(missing, ", "))
		return
	}
	results.pass("all %d CDS extension bundles are present in %s", len(expected), isDropins)

	if state.RestartPending {
		results.fail("the Identity Server has not restarted since the bundles were staged, "+
			"so extension %s is installed but not active", state.ExtensionVersion)
		return
	}
	if state.ActiveExtensionVersion == "" {
		results.pass("the extension bundles are active (this build declares no baseline to check them against)")
		return
	}
	results.pass("extension baseline %s is active", state.ActiveExtensionVersion)
}

// verifyTokenAudience confirms the token CDS will accept carries the audience
// the runtime checks for.
func verifyTokenAudience(results *checks, token string) {

	claims, err := jwtClaims(token)
	if err != nil {
		// Opaque tokens are introspected rather than parsed, so there is no
		// audience to read and nothing is wrong.
		results.pass("the Identity Server issued a token (opaque, introspected by CDS)")
		return
	}

	for _, audience := range claims.audiences() {
		if audience == cdsAudience {
			results.pass("the CDS client token carries the %q audience", cdsAudience)
			return
		}
	}
	results.fail("the CDS client token does not carry the %q audience, so CDS will reject it", cdsAudience)
}

func verifyAPIAccess(results *checks, client *cdsClient) {

	status, err := client.call(http.MethodGet, "/profiles?limit=1", nil, nil)
	switch {
	case err != nil:
		results.fail("GET /profiles failed: %v", err)
	case status == http.StatusOK:
		results.pass("an authorized GET /profiles succeeds for %q", client.organization)
	case status == http.StatusBadRequest:
		results.fail("GET /profiles returned HTTP 400, which is what CDS returns while the "+
			"organization %q is not enabled — run `cds enable` again", client.organization)
	default:
		results.fail("GET /profiles returned HTTP %d", status)
	}

	// The profile schema is synchronised from the Identity Server's claim
	// APIs, so reading it exercises the CDS-to-IS direction of the
	// integration, which no other check covers.
	status, err = client.call(http.MethodGet, "/profile-schema", nil, nil)
	switch {
	case err != nil:
		results.fail("GET /profile-schema failed: %v", err)
	case status == http.StatusOK:
		results.pass("GET /profile-schema succeeds, so CDS can reach the Identity Server claim APIs")
	default:
		results.fail("GET /profile-schema returned HTTP %d", status)
	}
}

// verifyDeep reports that the end-to-end test is not run by this command.
//
// It would create an Identity Server user, wait for the profile the event
// handler produces, and remove the user again. That needs user-management
// authorisation this command does not hold, so rather than half-run it the
// command says what it would have taken.
func verifyDeep(results *checks, client *cdsClient) {

	results.fail("--deep needs an Identity Server administrator session for user management, " +
		"which `verify` does not take. Run the synchronisation test from the integration suite instead")
}

// jwtClaims decodes the payload of a JWT without verifying it. The signature
// is the Identity Server's to make and CDS's to check; all that is read here
// is what the token says about itself.
type jwtPayload struct {
	Audience any    `json:"aud"`
	Issuer   string `json:"iss"`
}

func (p jwtPayload) audiences() []string {

	switch value := p.Audience.(type) {
	case string:
		return []string{value}
	case []any:
		out := make([]string, 0, len(value))
		for _, entry := range value {
			out = append(out, fmt.Sprintf("%v", entry))
		}
		return out
	}
	return nil
}

func jwtClaims(token string) (jwtPayload, error) {

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtPayload{}, fmt.Errorf("not a JWT")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtPayload{}, fmt.Errorf("the token payload is not base64url: %w", err)
	}
	var payload jwtPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return jwtPayload{}, fmt.Errorf("the token payload is not JSON: %w", err)
	}
	return payload, nil
}

// cdsAPIScopesFor asks for the read scopes verification needs, and nothing
// more: a check should not hold authority it does not exercise.
func cdsAPIScopesFor(state *integration) []string {

	return []string{
		"internal_cds_profile_view",
		"internal_cds_profile_schema_view",
		"internal_cds_admin_config_view",
	}
}

func summarise(results *checks) error {

	fmt.Println()
	if results.failed == 0 {
		fmt.Printf("  %s %d checks passed\n\n", colour("32", "✓"), results.run)
		return nil
	}
	return fmt.Errorf("%d of %d checks failed", results.failed, results.run)
}

// verifyCertificate reports whether the deployment still presents the key pair
// the pack shipped with. It is a valid certificate, so this is not a failure —
// but every download of a release carries the same private key, which an
// operator running a real deployment needs told rather than discovered.
func verifyCertificate(results *checks, cdsHome string, config *runtimeConfig) {

	certificate := config.serverCertificate(cdsHome)
	switch {
	case !fileExists(certificate):
		results.fail("no TLS certificate at %s", certificate)
	case usingDefaultCertificate(certificate):
		results.fail("CDS is presenting the default certificate the pack shipped with; " +
			"every download of this release carries its private key, so replace it " +
			"with one issued for this deployment")
	default:
		results.pass("the TLS certificate was issued for this deployment")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
