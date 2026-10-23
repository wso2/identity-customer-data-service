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
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Paths inside an Identity Server installation that enablement touches.
const (
	isDeploymentTOML = "repository/conf/deployment.toml"
	isDropins        = "repository/components/dropins"
	isSecurity       = "repository/resources/security"
	isStartScript    = "bin/wso2server.sh"
	isProductFile    = "updates/product.txt"
	isVersionFile    = "bin/version.txt"
	isCarbonXML      = "repository/conf/carbon.xml"
	isConsoleJ2      = "repository/resources/conf/templates/repository/deployment/server/webapps/console/deployment.config.json.j2"

	// The generated section is delimited so that re-enabling replaces it
	// instead of appending a second copy, and so `disable` removes exactly
	// what was added. Both markers are matched on their prefix, so the text
	// after them can change between versions without stranding an old block.
	tomlBeginMark = "# BEGIN cds"
	tomlEndMark   = "# END cds"
)

// isInstall is a validated Identity Server installation.
type isInstall struct {
	home    string
	version string
}

// openISHome validates that dir really is an Identity Server installation and
// reads its version. Enablement writes into this directory, so it refuses to
// act on anything it has not positively identified.
func openISHome(dir string) (*isInstall, error) {

	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("no IS_HOME given")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("IS_HOME %q is not a usable path: %w", dir, err)
	}
	for _, required := range []string{isDeploymentTOML, isStartScript} {
		if _, err := os.Stat(filepath.Join(abs, filepath.FromSlash(required))); err != nil {
			return nil, fmt.Errorf("%s does not look like an Identity Server installation: %s is missing",
				abs, required)
		}
	}

	install := &isInstall{home: abs, version: readISVersion(abs)}
	if install.version == "" {
		return nil, fmt.Errorf("could not determine the Identity Server version under %s", abs)
	}
	return install, nil
}

func (is *isInstall) path(rel string) string {
	return filepath.Join(is.home, filepath.FromSlash(rel))
}

var isVersionPattern = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

// readISVersion resolves the IS version from the pack, preferring the U2
// product identity because that is the string the update service maintains.
func readISVersion(isHome string) string {

	candidates := []string{isProductFile, isVersionFile, isCarbonXML}
	for _, rel := range candidates {
		raw, err := os.ReadFile(filepath.Join(isHome, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		if match := isVersionPattern.FindString(string(raw)); match != "" {
			return match
		}
	}
	return ""
}

// bundlesInDropins reports which of the named bundles the Identity Server
// already has, as bundle name -> filename.
//
// From the Identity Server release that ships CDS support, these arrive with
// the product and are maintained through its own update stream. They are not
// CDS's to replace or remove, so enablement has to know which are already
// there before it installs anything.
func (is *isInstall) bundlesInDropins(bundles []string) (map[string]string, error) {

	wanted := make(map[string]bool, len(bundles))
	for _, bundle := range bundles {
		wanted[bundle] = true
	}

	entries, err := os.ReadDir(is.path(isDropins))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("could not read %s: %w", isDropins, err)
	}

	found := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jar") {
			continue
		}
		if name := bundleName(entry.Name()); wanted[name] {
			found[name] = entry.Name()
		}
	}
	return found, nil
}

// installExtensions copies the CDS extension bundles into the IS dropins,
// removing any earlier build of the same bundle first so that two versions of
// one bundle never sit there together — OSGi would resolve one of them and the
// choice would not be ours.
//
// It returns the filenames installed, which `disable` later removes.
func (is *isInstall) installExtensions(jars []string) ([]string, error) {

	dropins := is.path(isDropins)
	if err := os.MkdirAll(dropins, 0o755); err != nil {
		return nil, fmt.Errorf("could not create %s: %w", isDropins, err)
	}

	installed := make([]string, 0, len(jars))
	for _, jar := range jars {
		bundle := bundleName(filepath.Base(jar))
		if err := removeBundleVersions(dropins, bundle); err != nil {
			return nil, err
		}
		target := filepath.Join(dropins, filepath.Base(jar))
		if err := copyFile(jar, target, 0o644); err != nil {
			return nil, fmt.Errorf("could not install %s: %w", filepath.Base(jar), err)
		}
		installed = append(installed, filepath.Base(jar))
		info("%s", filepath.Base(jar))
	}
	return installed, nil
}

// removeExtensions deletes the named dropin files. Files that are already gone
// are not an error: `disable` has to converge on "removed" from any state.
func (is *isInstall) removeExtensions(names []string) int {

	removed := 0
	for _, name := range names {
		// Only ever a bare filename inside dropins — never a path from the
		// recorded state that could point elsewhere in the filesystem.
		target := filepath.Join(is.path(isDropins), filepath.Base(name))
		if err := os.Remove(target); err == nil {
			removed++
			info("removed %s", filepath.Base(name))
		}
	}
	return removed
}

// bundleName strips the version and extension from a bundle jar filename,
// leaving the symbolic name: org.wso2.…client-0.1.13.jar -> org.wso2.…client.
var bundleVersionSuffix = regexp.MustCompile(`-\d[^-]*(-SNAPSHOT)?\.jar$`)

func bundleName(filename string) string {
	return bundleVersionSuffix.ReplaceAllString(filename, "")
}

// bundleVersion recovers the version from a bundle jar filename, so an
// Identity Server-provided bundle can be compared against the baseline this
// CDS release expects.
func bundleVersion(filename string) string {

	match := bundleVersionSuffix.FindString(filename)
	if match == "" {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(match, "-"), ".jar")
}

func removeBundleVersions(dropins, bundle string) error {

	entries, err := os.ReadDir(dropins)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", isDropins, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jar") {
			continue
		}
		if bundleName(entry.Name()) != bundle {
			continue
		}
		if err := os.Remove(filepath.Join(dropins, entry.Name())); err != nil {
			return fmt.Errorf("could not replace the installed %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// writeTOMLSection replaces the generated CDS section of deployment.toml, or
// appends it when there is none.
//
// deployment.toml is the operator's file: everything outside the markers is
// preserved byte for byte, and the file is replaced atomically so an
// interrupted enable cannot leave the Identity Server unable to start.
func (is *isInstall) writeTOMLSection(section string) (replaced bool, err error) {

	target := is.path(isDeploymentTOML)
	current, err := os.ReadFile(target)
	if err != nil {
		return false, fmt.Errorf("could not read %s: %w", isDeploymentTOML, err)
	}

	stripped, found := stripTOMLSection(string(current))
	updated := strings.TrimRight(stripped, "\n") + "\n\n" + strings.TrimRight(section, "\n") + "\n"

	if err := writeFileAtomic(target, []byte(updated), 0o644); err != nil {
		return false, fmt.Errorf("could not write %s: %w", isDeploymentTOML, err)
	}
	return found, nil
}

// removeTOMLSection takes the generated section back out, reporting whether
// there was one.
func (is *isInstall) removeTOMLSection() (bool, error) {

	target := is.path(isDeploymentTOML)
	current, err := os.ReadFile(target)
	if err != nil {
		return false, fmt.Errorf("could not read %s: %w", isDeploymentTOML, err)
	}

	stripped, found := stripTOMLSection(string(current))
	if !found {
		return false, nil
	}
	if err := writeFileAtomic(target, []byte(strings.TrimRight(stripped, "\n")+"\n"), 0o644); err != nil {
		return false, fmt.Errorf("could not write %s: %w", isDeploymentTOML, err)
	}
	return true, nil
}

// stripTOMLSection removes the lines between the markers, inclusive.
func stripTOMLSection(content string) (string, bool) {

	var (
		out      strings.Builder
		scanner  = bufio.NewScanner(strings.NewReader(content))
		skipping bool
		found    bool
	)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, tomlBeginMark):
			skipping, found = true, true
			continue
		case strings.HasPrefix(line, tomlEndMark):
			skipping = false
			continue
		case skipping:
			continue
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String(), found
}

// consoleSupportsCDS reports whether the Console bundled with this IS pack can
// render the Customer Data section.
//
// The Console's configuration template and its web app ship as one
// identity-apps artifact, so a pack whose template has no cds_host key has no
// CDS Console to configure, however the toml is written. Saying so at enable
// time turns a silently missing menu into a stated incompatibility.
func (is *isInstall) consoleSupportsCDS() (bool, string) {

	raw, err := os.ReadFile(is.path(isConsoleJ2))
	if err != nil {
		return false, "Console configuration template not found, so its CDS support could not be checked"
	}
	if !strings.Contains(string(raw), "cds_host") {
		return false, "this pack's Console predates CDS support, so the Customer Data section will not appear"
	}
	if !strings.Contains(string(raw), "customer_data_profiles") {
		return false, "this pack's Console has cds_host but no customerData feature blocks, " +
			"so the Customer Data section may not appear"
	}
	return true, ""
}
