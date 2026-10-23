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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"
)

// integration is the record of the CDS-to-IS relationship that `enable`
// establishes, written to repository/conf/integration.yaml.
//
// It holds only what a later command cannot re-derive: which IS this pack was
// enabled against, what was installed there, and whether that IS still owes a
// restart. Secrets are deliberately absent — they go to the protected secrets
// file, which this record never reproduces.
type integration struct {
	ISHome    string `yaml:"is_home"`
	ISVersion string `yaml:"is_version"`
	// CDSVersion is the pack version at the time of enablement. A U2 update
	// that replaces the binary moves the pack version past it, which is how
	// `status` tells an updated-but-not-re-enabled installation apart.
	CDSVersion string `yaml:"cds_version"`
	// ExtensionVersion is the extension baseline staged into the IS dropins.
	ExtensionVersion string `yaml:"extension_version"`
	// ActiveExtensionVersion is the baseline last confirmed as loaded by a
	// running IS. A staged JAR is not active until IS restarts, so this moves
	// only when `verify` observes the restarted server, never when `enable`
	// copies a file.
	ActiveExtensionVersion string `yaml:"active_extension_version"`
	// Extensions are the dropin filenames CDS installed, so `disable` removes
	// exactly what `enable` put there and nothing else.
	Extensions []string `yaml:"extensions"`
	// BundledExtensions are the bundles the Identity Server already provided.
	// They are recorded so that status and verify can see them, and kept out
	// of Extensions so that disable never removes a file that arrived with
	// the Identity Server and is maintained by its own update stream.
	BundledExtensions []string `yaml:"bundled_extensions"`
	// Organization is the IS organization CDS was enabled for.
	Organization string `yaml:"organization"`
	// RestartPending records that IS-side artifacts changed after the last IS
	// start. Only an operator restarting IS clears it, because nothing here
	// may restart a separately managed installation.
	RestartPending bool `yaml:"restart_pending"`
	// Provisioned records that the post-restart provisioning step completed,
	// so re-running `enable` after the restart resumes rather than repeats.
	Provisioned bool      `yaml:"provisioned"`
	EnabledAt   time.Time `yaml:"enabled_at"`
}

// errNotEnabled is returned when a command needs an established CDS-to-IS
// relationship and there is none.
var errNotEnabled = errors.New("CDS is not enabled. Run `cds enable --is-home <IS_HOME>` first")

// loadIntegration reads the integration record. A missing record means the
// installation was never enabled, which errNotEnabled reports distinctly so
// callers can tell it from a damaged file.
func loadIntegration(cdsHome string) (*integration, error) {

	raw, err := os.ReadFile(path(cdsHome, stateFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, errNotEnabled
	}
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", stateFile, err)
	}

	var state integration
	if err := yaml.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("%s is not readable as YAML, so the recorded integration is unusable: %w",
			stateFile, err)
	}
	if state.ISHome == "" {
		return nil, fmt.Errorf("%s records no is_home, so the recorded integration is incomplete", stateFile)
	}
	return &state, nil
}

// saveIntegration writes the record atomically, so an interrupted enable
// leaves either the previous record or the new one and never a half-written
// file that the next run would refuse to parse.
func saveIntegration(cdsHome string, state *integration) error {

	out, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("could not serialise the integration record: %w", err)
	}

	header := []byte("# Written by `cds enable`. Records the Identity Server this installation\n" +
		"# is enabled against. Secrets are not kept here; see " + secretsFile + ".\n")

	target := path(cdsHome, stateFile)
	tmp, err := os.CreateTemp(filepath.Dir(target), ".integration-*.yaml")
	if err != nil {
		return fmt.Errorf("could not stage %s: %w", stateFile, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(append(header, out...)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("could not write %s: %w", stateFile, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("could not write %s: %w", stateFile, err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("could not install %s: %w", stateFile, err)
	}
	return nil
}

// clearIntegration removes the record. A missing record is success: `disable`
// is idempotent.
func clearIntegration(cdsHome string) error {

	if err := os.Remove(path(cdsHome, stateFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not remove %s: %w", stateFile, err)
	}
	return nil
}
