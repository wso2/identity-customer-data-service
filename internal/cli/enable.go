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
	_ "embed"
	"errors"
	"fmt"
	"time"
)

//go:embed templates/is-deployment.toml
var isDeploymentSection string

// enableOptions are the inputs to enablement, after flags, recorded state and
// the environment have been reconciled.
type enableOptions struct {
	isHome           string
	extensions       string
	cdsURL           string
	isURL            string
	organization     string
	adminUser        string
	adminPassword    string
	keystorePassword string
	insecure         bool
	skipProvisioning bool
}

// runEnable establishes the relationship between this CDS installation and one
// Identity Server.
//
// It runs in two phases because the Identity Server has to restart between
// them: everything that writes to IS_HOME happens first, and the provisioning
// that calls the running server happens after the restart. Re-running the
// command is how the second phase is reached, which is also what makes
// enablement resumable after a failure — every step checks what is already
// there before it does anything.
//
// The Identity Server is never restarted from here. It is a separately managed
// installation, and restarting it is the operator's decision to make.
func runEnable(args []string) error {

	flags := newFlagSet("enable")
	isHome := flags.String("is-home", "", "the Identity Server installation to enable CDS against")
	extensions := flags.String("extensions", "", "directory holding the CDS extension bundles (default: download the baseline)")
	cdsURL := flags.String("cds-url", "", "the URL the Identity Server and Console reach CDS on")
	isURL := flags.String("is-url", "", "the URL of the Identity Server management APIs (default: "+defaultISManagementURL+")")
	organization := flags.String("organization", defaultOrganization, "the organization to enable CDS for")
	adminUser := flags.String("admin-user", defaultISAdminUser, "Identity Server administrator username")
	adminPassword := flags.String("admin-password", "", "Identity Server administrator password (default: $"+isAdminPasswordEnvVar+", else prompt)")
	insecure := flags.Bool("insecure", false, "do not verify the Identity Server certificate while provisioning")
	skipProvisioning := flags.Bool("skip-provisioning", false, "stop after the Identity Server files are staged")
	if err := parse(flags, args); err != nil {
		return err
	}

	cdsHome, err := home()
	if err != nil {
		return err
	}

	options := enableOptions{
		isHome:           *isHome,
		extensions:       *extensions,
		cdsURL:           *cdsURL,
		isURL:            *isURL,
		organization:     *organization,
		adminUser:        *adminUser,
		adminPassword:    *adminPassword,
		keystorePassword: envOr(isKeystorePasswordEnvVar, defaultISStorePassword),
		insecure:         *insecure,
		skipProvisioning: *skipProvisioning,
	}
	return enable(cdsHome, options)
}

func enable(cdsHome string, options enableOptions) error {

	compat, err := readCompatibility(cdsHome)
	if err != nil {
		return err
	}
	cdsVersion, err := productVersion(cdsHome)
	if err != nil {
		return err
	}
	config, err := readRuntimeConfig(cdsHome)
	if err != nil {
		return err
	}

	// A previous enablement is the default answer to every question that was
	// not asked again, which is what makes a bare `wso2cds enable` resume.
	previous, err := loadIntegration(cdsHome)
	if err != nil && !errors.Is(err, errNotEnabled) {
		return err
	}

	resolvedISHome, err := resolveISHome(options.isHome, previous)
	if err != nil {
		return err
	}

	step("Identity Server")
	is, err := openISHome(resolvedISHome)
	if err != nil {
		return err
	}
	info("%s", is.home)
	info("version %s", is.version)

	switch {
	case !compat.declaresBaseline():
		warn("this CDS build declares no verified Identity Server baseline in %s, "+
			"so %s is not known to be supported", compatibilityFile, is.version)
	case !compat.supportsIS(is.version):
		return fmt.Errorf("CDS %s is not supported with Identity Server %s.\n"+
			"Supported: %s\nSee %s", cdsVersion, is.version, compat.supportedISList(), compatibilityFile)
	default:
		done("Identity Server %s is a supported baseline for CDS %s", is.version, cdsVersion)
	}

	if supported, reason := is.consoleSupportsCDS(); !supported {
		warn("%s", reason)
	} else {
		done("the bundled Console supports the Customer Data section")
	}

	// ---------------------------------------------------------------- //
	// Phase 1 — everything that writes into IS_HOME
	// ---------------------------------------------------------------- //
	step("CDS extension bundles")
	installed, bundled, err := stageExtensions(cdsHome, is, compat, options.extensions)
	if err != nil {
		return err
	}

	step("Configuring the Identity Server")
	syncPassword, err := syncCredential(cdsHome)
	if err != nil {
		return err
	}
	cdsBase := resolveCDSURL(options.cdsURL, previous, config)
	isBase := resolveISURL(options.isURL, previous, config)

	section, err := renderTOMLSection(cdsBase, isBase, syncPassword)
	if err != nil {
		return err
	}
	if _, err := backupFile(is.path(isDeploymentTOML)); err != nil {
		return err
	}
	replaced, err := is.writeTOMLSection(section)
	if err != nil {
		return err
	}
	if replaced {
		done("replaced the generated CDS section of %s", isDeploymentTOML)
	} else {
		done("wrote the CDS section into %s", isDeploymentTOML)
	}

	step("Exchanging certificates")
	trustPEM, err := exchangeCertificates(cdsHome, is, config, options.keystorePassword)
	if err != nil {
		warn("%v", err)
		warn("CDS and the Identity Server will not trust each other until this is done by hand")
	}

	step("Configuring CDS")
	if err := writeCDSConfig(cdsHome, config, cdsBase, isBase); err != nil {
		return err
	}
	if err := saveSecrets(cdsHome, map[string]string{
		"AUTH_SERVER_ADMIN_PASSWORD": syncPassword,
	}); err != nil {
		return err
	}
	done("wrote %s and %s", configFile, secretsFile)

	state := &integration{
		ISHome:            is.home,
		ISVersion:         is.version,
		CDSVersion:        cdsVersion,
		ExtensionVersion:  compat.Extensions.Version,
		Extensions:        installed,
		BundledExtensions: bundled,
		Organization:      options.organization,
		RestartPending:    true,
		EnabledAt:         time.Now().UTC(),
	}
	if previous != nil {
		state.ActiveExtensionVersion = previous.ActiveExtensionVersion
	}
	if err := saveIntegration(cdsHome, state); err != nil {
		return err
	}

	if options.skipProvisioning {
		reportRestartRequired(is)
		return nil
	}

	// ---------------------------------------------------------------- //
	// Phase 2 — provisioning, which needs the restarted Identity Server
	// ---------------------------------------------------------------- //
	step("Identity Server provisioning")
	api, ready, err := provisioningClient(&options, isBase, trustPEM)
	if err != nil {
		return err
	}
	if !ready {
		reportRestartRequired(is)
		return nil
	}

	if err := provision(cdsHome, api, options, cdsBase, isBase, trustPEM); err != nil {
		return err
	}
	state.RestartPending = false
	state.ActiveExtensionVersion = state.ExtensionVersion
	state.Provisioned = true
	if err := saveIntegration(cdsHome, state); err != nil {
		return err
	}

	step("Enabled")
	info("CDS %s is enabled against Identity Server %s", cdsVersion, is.version)
	info("Next: wso2cds start, then wso2cds verify")
	return nil
}

// stageExtensions puts the CDS extension bundles in place, without taking
// over the ones the Identity Server already has.
//
// From the Identity Server release that ships CDS support, the bundles arrive
// with the product and are delivered and updated through its update stream.
// That stream owns them: replacing one here would put a file CDS downloaded
// where the Identity Server expects its own, and removing it on `disable`
// would leave that installation short of a bundle it shipped with. So a
// bundle that is already there is left alone and only reported.
//
// An explicit --extensions is different. It is the operator naming the
// bundles to use — testing an unreleased build, or recovering a damaged
// installation — so it wins, and says so.
//
// It returns the filenames CDS installed, and the filenames the Identity
// Server provided.
func stageExtensions(cdsHome string, is *isInstall, compat *compatibility, override string) (installed, bundled []string, err error) {

	present, err := is.bundlesInDropins(compat.Extensions.Bundles)
	if err != nil {
		return nil, nil, err
	}

	want := compat.Extensions.Bundles
	if override != "" {
		if len(present) > 0 {
			warn("--extensions replaces %d bundle(s) the Identity Server provides; "+
				"an Identity Server update will put its own back", len(present))
		}
	} else {
		want = nil
		stale := 0
		for _, bundle := range compat.Extensions.Bundles {
			filename, found := present[bundle]
			if !found {
				want = append(want, bundle)
				continue
			}
			bundled = append(bundled, filename)
			if !reportBundledVersion(filename, compat.Extensions.Version) {
				stale++
			}
		}
		// The advice is the same for every stale bundle, so it is given once.
		if stale > 0 {
			warn("apply the Identity Server update that carries CDS extension %s", compat.Extensions.Version)
		}
	}

	if len(want) == 0 {
		done("the Identity Server provides all %d CDS extension bundles", len(bundled))
		return nil, bundled, nil
	}

	jars, err := resolveExtensions(cdsHome, compat.Extensions, override, want)
	if err != nil {
		return nil, nil, err
	}
	installed, err = is.installExtensions(jars)
	if err != nil {
		return nil, nil, err
	}
	done("installed %d bundle(s) into %s", len(installed), isDropins)
	if len(bundled) > 0 {
		info("%d further bundle(s) came with the Identity Server and were left alone", len(bundled))
	}
	return installed, bundled, nil
}

// reportBundledVersion says what the Identity Server provides, and reports
// whether it is the baseline this CDS release expects.
//
// A mismatch is not fixed here. These bundles are delivered through the
// Identity Server's update stream, so the fix is an update to that
// installation, which is not CDS's to apply.
func reportBundledVersion(filename, baseline string) bool {

	version := bundleVersion(filename)
	if baseline == "" || version == "" || version == baseline {
		info("%s (provided by the Identity Server)", filename)
		return true
	}
	warn("%s is provided by the Identity Server, but this CDS release expects %s", filename, baseline)
	return false
}

// provisioningClient builds the management API client and reports whether the
// Identity Server has reached the state provisioning needs.
//
// The CDS API resources are seeded from deployment.toml, so they exist only
// once the server has restarted and read it. Their presence is therefore the
// signal that phase 2 can run — and their absence is not a failure, it is the
// restart still being owed.
func provisioningClient(options *enableOptions, isBase, trustPEM string) (*isAPI, bool, error) {

	// Whether the server is up is settled before anything is asked of the
	// operator: a password taken here and then discarded is worse than no
	// prompt at all.
	probe, err := newISAPI(isBase, "", "", trustPEM, options.insecure)
	if err != nil {
		return nil, false, err
	}
	if !probe.reachable() {
		info("the Identity Server at %s is not answering, so provisioning waits.", isBase)
		return nil, false, nil
	}

	password, err := adminPassword(options)
	if err != nil {
		return nil, false, err
	}

	api, err := newISAPI(isBase, options.adminUser, password, trustPEM, options.insecure)
	if err != nil {
		return nil, false, err
	}

	resourceID, err := api.apiResourceID(cdsAPIResources[0])
	if err != nil {
		info("the Identity Server at %s could not be queried: %v", isBase, err)
		return nil, false, nil
	}
	if resourceID == "" {
		info("the Identity Server is running but has not registered the CDS API")
		info("resources, so it has not yet read the configuration staged above.")
		return nil, false, nil
	}
	return api, true, nil
}

// provision performs the post-restart steps against a running Identity Server.
func provision(cdsHome string, api *isAPI, options enableOptions, cdsBase, isBase, trustPEM string) error {

	// 1. The API resources the toml section seeds.
	for _, identifier := range cdsAPIResources {
		id, err := api.apiResourceID(identifier)
		if err != nil {
			return err
		}
		if id == "" {
			return fmt.Errorf("the Identity Server did not register the API resource %s.\n"+
				"The [[api_resources]] entries in %s were not picked up — check the server's startup log",
				identifier, isDeploymentTOML)
		}
	}
	done("all %d CDS API resources are registered", len(cdsAPIResources))

	// 2-3. The application CDS authenticates as, and the IS APIs it may call.
	systemID, err := ensureApplication(api, systemAppName, "Used by the Customer Data Service to call Identity Server APIs")
	if err != nil {
		return err
	}
	for _, grant := range managementAPIGrants {
		if err := api.authorizeAPI(systemID, grant.Identifier, grant.Scopes); err != nil {
			return fmt.Errorf("could not authorize %q for %s: %w", systemAppName, grant.Identifier, err)
		}
	}
	done("%q is authorized for the Identity Server APIs CDS calls", systemAppName)

	systemOIDC, _, err := api.oidcConfig(systemID)
	if err != nil {
		return err
	}

	// 4. A client application for calling the CDS APIs.
	clientID, err := ensureApplication(api, clientAppName, "Calls the Customer Data Service APIs")
	if err != nil {
		return err
	}
	if _, err := api.addTokenAudience(clientID, cdsAudience); err != nil {
		return fmt.Errorf("could not set the %q token audience on %q: %w", cdsAudience, clientAppName, err)
	}
	if err := authorizeCDSAPIs(api, clientID); err != nil {
		return err
	}
	done("%q is authorized for the CDS APIs, audience %q", clientAppName, cdsAudience)

	clientOIDC, _, err := api.oidcConfig(clientID)
	if err != nil {
		return err
	}

	// 5. The Console, and the administrator role that uses it.
	provisionConsole(api)

	// The runtime authenticates as the system application and introspects the
	// tokens it is given with the same credentials.
	if err := saveSecrets(cdsHome, map[string]string{
		"AUTH_SERVER_CLIENT_SECRET":   systemOIDC.ClientSecret,
		"INTROSPECTION_CLIENT_SECRET": systemOIDC.ClientSecret,
		"CDS_CLIENT_ID":               clientOIDC.ClientID,
		"CDS_CLIENT_SECRET":           clientOIDC.ClientSecret,
	}); err != nil {
		return err
	}
	if err := writeClientIDs(cdsHome, systemOIDC.ClientID); err != nil {
		return err
	}
	done("recorded the client credentials in %s", secretsFile)

	// 6. Organization enablement, which is a CDS API call and so needs a
	// running CDS. Starting CDS is an explicit lifecycle operation, so this
	// step reports what remains rather than starting the process itself; a
	// later `wso2cds enable` picks it up once CDS is running.
	return enableOrganizationStep(api, options, cdsBase, isBase, trustPEM, clientOIDC)
}

// enableOrganizationStep turns CDS on for the organization, when CDS is
// running to be asked.
func enableOrganizationStep(api *isAPI, options enableOptions, cdsBase, isBase, trustPEM string,
	clientOIDC *oidcConfig) error {

	step("Enabling CDS for %q", options.organization)

	if status, _, err := probe(cdsBase, "/cds/api/v1/ready", 5*time.Second); err != nil || status != 200 {
		info("CDS is not serving at %s yet, so the organization is not enabled.", cdsBase)
		info("Run `wso2cds start`, then `wso2cds enable` again to finish.")
		return nil
	}

	token, err := cdsToken(isBase, clientOIDC.ClientID, clientOIDC.ClientSecret,
		cdsScopes(api), trustPEM, options.insecure)
	if err != nil {
		return err
	}

	client := &cdsClient{base: cdsBase, organization: options.organization, token: token}
	changed, err := enableOrganization(client, []string{"CONSOLE", clientOIDC.ClientID})
	if err != nil {
		return err
	}
	if changed {
		done("cds_enabled = true; the initial profile-schema sync ran and the default consent categories were seeded")
	} else {
		info("CDS is already enabled for %q", options.organization)
	}
	return nil
}

func ensureApplication(api *isAPI, name, description string) (string, error) {

	id, err := api.applicationID(name)
	if err != nil {
		return "", err
	}
	if id != "" {
		info("reusing the %q application", name)
		return id, nil
	}

	id, err = api.createM2MApplication(name, description, true, tokenExpirySeconds)
	if err != nil {
		return "", fmt.Errorf("could not create the %q application: %w", name, err)
	}
	if id == "" {
		return "", fmt.Errorf("the %q application was created but could not be read back", name)
	}
	info("created the %q application", name)
	return id, nil
}

// authorizeCDSAPIs grants an application every scope of every CDS API
// resource. The scopes are read back from the server rather than listed here,
// so an API resource that gains a scope in a later release does not need this
// code to change.
func authorizeCDSAPIs(api *isAPI, appID string) error {

	for _, identifier := range cdsAPIResources {
		scopes, err := scopesOf(api, identifier)
		if err != nil {
			return err
		}
		if err := api.authorizeAPI(appID, identifier, scopes); err != nil {
			return fmt.Errorf("could not authorize %s: %w", identifier, err)
		}
	}
	return nil
}

// scopesOf reads back every scope an API resource declares.
func scopesOf(api *isAPI, identifier string) ([]string, error) {

	resourceID, err := api.apiResourceID(identifier)
	if err != nil {
		return nil, err
	}
	if resourceID == "" {
		return nil, fmt.Errorf("the Identity Server has no API resource %s", identifier)
	}
	scopes, err := api.apiResourceScopes(resourceID)
	if err != nil {
		return nil, err
	}
	if len(scopes) == 0 {
		return nil, fmt.Errorf("the API resource %s has no scopes", identifier)
	}
	return scopes, nil
}

// provisionConsole authorizes the Console and the administrator role.
//
// Failures here are reported and not fatal: CDS itself is fully usable over
// its APIs without the Console, and an operator can grant these from the
// Console's own UI.
func provisionConsole(api *isAPI) {

	consoleID, err := api.applicationID("Console")
	if err != nil || consoleID == "" {
		warn("could not find the Console application, so the Console may not reach CDS")
		return
	}

	consoleOIDC, _, err := api.oidcConfig(consoleID)
	if err != nil {
		warn("could not read the Console OIDC configuration: %v", err)
		return
	}
	// Only JWTs carry an audience. With opaque tokens CDS introspects instead,
	// and there is nothing to add.
	if tokenType, _ := consoleOIDC.AccessToken["type"].(string); tokenType == "JWT" {
		if _, err := api.addTokenAudience(consoleID, cdsAudience); err != nil {
			warn("could not add %q to the Console token audience: %v", cdsAudience, err)
		}
	}

	// The Identity Server owns the Console's authorised APIs and refuses to
	// patch them, so an authorisation that is already there is left alone.
	// What the Console can reach is declared in the api_resource_collections
	// and console.customer_data_* sections written into deployment.toml.
	added, held := 0, 0
	for _, identifier := range cdsAPIResources {
		scopes, err := scopesOf(api, identifier)
		if err != nil {
			warn("could not read the scopes of %s: %v", identifier, err)
			continue
		}
		created, err := api.authorizeAPIIfAbsent(consoleID, identifier, scopes)
		switch {
		case err != nil:
			warn("could not authorize the Console for %s: %v", identifier, err)
		case created:
			added++
		default:
			held++
		}
	}
	switch {
	case added > 0:
		done("authorized the Console for %d CDS API resource(s)", added)
	case held == len(cdsAPIResources):
		info("the Console already reaches all %d CDS API resources", held)
	}

	permissions := append(append([]string{}, consoleFeatureScopes...), cdsScopes(api)...)
	granted, err := api.grantRolePermissions("admin", permissions)
	switch {
	case err != nil:
		warn("could not grant the CDS scopes to the 'admin' role: %v", err)
		warn("grant the Customer Data scopes from the Console if its pages come up empty")
	case granted == 0:
		done("the 'admin' role already holds every CDS scope")
	default:
		done("granted %d scope(s) to the 'admin' role", granted)
	}
}

// cdsScopes collects every scope the CDS API resources declare. A resource
// that cannot be read is skipped rather than failing the role grant, because a
// partial grant is still better than none.
func cdsScopes(api *isAPI) []string {

	var all []string
	for _, identifier := range cdsAPIResources {
		resourceID, err := api.apiResourceID(identifier)
		if err != nil || resourceID == "" {
			continue
		}
		scopes, err := api.apiResourceScopes(resourceID)
		if err != nil {
			continue
		}
		all = append(all, scopes...)
	}
	return all
}

func reportRestartRequired(is *isInstall) {

	step("Identity Server restart required")
	info("The CDS extension bundles and configuration are staged in %s.", is.home)
	info("They take effect only when the Identity Server restarts, which is yours to do:")
	info("")
	info("    %s/bin/wso2server.sh restart", is.home)
	info("")
	info("Then run `wso2cds enable` again to finish provisioning.")
}
