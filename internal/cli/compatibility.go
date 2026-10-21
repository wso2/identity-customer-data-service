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
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// compatibilityFile is the release compatibility metadata shipped in the pack.
const compatibilityFile = "repository/conf/compatibility.yaml"

// compatibility declares what this CDS release works with: which Identity
// Server versions it can be enabled against, and which extension baseline
// belongs with them.
//
// It is one file serving two readers. `enable` consults it to refuse an
// unsupported IS and to resolve the extension jars, and the release publishes
// it so that installation tooling can pick the right artifacts without running
// the product. Keeping one source avoids the two drifting apart.
type compatibility struct {
	Product        string `yaml:"product"`
	IdentityServer struct {
		// Supported lists the IS releases this CDS release is tested with.
		Supported []string `yaml:"supported"`
	} `yaml:"identity_server"`
	Extensions extensionBaseline `yaml:"extensions"`
	Console    struct {
		ReleaseLine string `yaml:"release_line"`
	} `yaml:"console"`
}

// extensionBaseline identifies the CDS extension set that belongs with this
// release. The bundles are deployed into the Identity Server, not into CDS, so
// they are versioned and delivered separately — see docs/process/releasing.md.
type extensionBaseline struct {
	Version    string   `yaml:"version"`
	Repository string   `yaml:"repository"`
	Group      string   `yaml:"group"`
	Bundles    []string `yaml:"bundles"`
}

func readCompatibility(cdsHome string) (*compatibility, error) {

	raw, err := os.ReadFile(path(cdsHome, compatibilityFile))
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", compatibilityFile, err)
	}

	var compat compatibility
	if err := yaml.Unmarshal(raw, &compat); err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", compatibilityFile, err)
	}
	if len(compat.Extensions.Bundles) == 0 {
		return nil, fmt.Errorf("%s lists no extension bundles", compatibilityFile)
	}
	return &compat, nil
}

// declaresBaseline reports whether this build states which Identity Server
// releases it was verified against.
//
// A development build does not: the baseline is settled when the release is
// cut, and claiming one before then would assert compatibility nobody has
// tested. Enablement warns in that case rather than refusing, so an early
// adopter can still proceed knowingly.
func (c *compatibility) declaresBaseline() bool { return len(c.IdentityServer.Supported) > 0 }

// supportsIS reports whether this release is declared compatible with an IS
// version.
func (c *compatibility) supportsIS(version string) bool {

	for _, supported := range c.IdentityServer.Supported {
		if strings.TrimSpace(supported) == version {
			return true
		}
	}
	return false
}

func (c *compatibility) supportedISList() string {
	return strings.Join(c.IdentityServer.Supported, ", ")
}
