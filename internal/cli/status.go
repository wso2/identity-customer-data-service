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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// runStatus prints the product, platform, runtime and integration state.
//
// It reports the logical CDS release separately from the platform-specific U2
// product variant, and the staged extension baseline separately from the one a
// restarted IS has actually loaded. Conflating either pair would let an
// installation look finished while a restart is still owed.
func runStatus(args []string) error {

	flags := newFlagSet("status")
	if err := parse(flags, args); err != nil {
		return err
	}

	cdsHome, err := home()
	if err != nil {
		return err
	}

	rows := make([][2]string, 0, 12)
	add := func(label, value string) { rows = append(rows, [2]string{label, value}) }

	version, err := productVersion(cdsHome)
	if err != nil {
		version = "unknown"
	}
	add("CDS Version", version)
	add("CDS Home", cdsHome)

	if product := u2Product(cdsHome); product != "" {
		add("CDS U2 Product", product)
		add("CDS U2 Level", u2Level(cdsHome))
	} else {
		add("CDS U2 Product", "not onboarded (no "+productFile+")")
	}
	add("Platform", runtime.GOOS+"/"+runtime.GOARCH)

	if pid := readPID(cdsHome); pid != 0 {
		add("CDS Process", fmt.Sprintf("Running (pid %d)", pid))
		if cfg, err := readRuntimeConfig(cdsHome); err == nil {
			add("CDS Endpoint", cfg.localBase())
			add("CDS Readiness", readiness(cfg.localBase()))
		}
	} else {
		add("CDS Process", "Stopped")
	}

	state, err := loadIntegration(cdsHome)
	switch {
	case errors.Is(err, errNotEnabled):
		add("Associated IS", "none — run `wso2cds enable --is-home <IS_HOME>`")
	case err != nil:
		add("Associated IS", "unreadable: "+err.Error())
	default:
		add("Associated IS", fmt.Sprintf("%s (%s)", state.ISVersion, state.ISHome))
		add("IS U2 State", "Managed independently from IS_HOME")
		add("Organization", state.Organization)
		add("Extension Bundles", bundleSources(state))
		add("Extension Staged", orNone(state.ExtensionVersion))
		add("Extension Active", orNone(state.ActiveExtensionVersion))
		add("IS Restart Pending", yesNo(state.RestartPending))
		add("Provisioned", yesNo(state.Provisioned))
		add("Enabled At", state.EnabledAt.Format(time.RFC3339))
	}

	width := 0
	for _, row := range rows {
		if len(row[0]) > width {
			width = len(row[0])
		}
	}
	fmt.Println()
	for _, row := range rows {
		fmt.Printf("  %-*s  %s\n", width, bold(row[0]), row[1])
	}
	fmt.Println()
	return nil
}

// u2Level reports the update level the update tool has reached for this pack.
// updates/config.json is written and owned by that tool, so it is absent until
// the tool has run once, and its key names are the tool's to choose — hence
// the tolerant read rather than a typed one.
func u2Level(cdsHome string) string {

	raw, err := os.ReadFile(path(cdsHome, updatesDir+"/config.json"))
	if err != nil {
		return "not initialised (run bin/update_tool_setup.sh)"
	}

	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return "unreadable"
	}
	for _, key := range []string{"update-level", "update_level", "updateLevel"} {
		if value, ok := config[key]; ok {
			return fmt.Sprintf("%v", value)
		}
	}
	return "unknown"
}

func readiness(base string) string {

	status, _, err := probe(base, "/cds/api/v1/ready", 5*time.Second)
	if err != nil {
		return "unreachable: " + err.Error()
	}
	if status == 200 {
		return "Ready"
	}
	return fmt.Sprintf("Not ready (HTTP %d)", status)
}

// bundleSources says where the extension bundles came from, because the
// answer decides who updates them: the Identity Server's update stream for
// the ones it provided, and a CDS re-enablement for the ones CDS installed.
func bundleSources(state *integration) string {

	switch {
	case len(state.BundledExtensions) > 0 && len(state.Extensions) > 0:
		return fmt.Sprintf("%d from the Identity Server, %d installed by CDS",
			len(state.BundledExtensions), len(state.Extensions))
	case len(state.BundledExtensions) > 0:
		return fmt.Sprintf("%d, all provided by the Identity Server", len(state.BundledExtensions))
	case len(state.Extensions) > 0:
		return fmt.Sprintf("%d, all installed by CDS", len(state.Extensions))
	}
	return "none"
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "none"
	}
	return s
}

// runVersion prints the version alone, for scripts that should not have to
// parse the status table.
func runVersion(args []string) error {

	flags := newFlagSet("version")
	if err := parse(flags, args); err != nil {
		return err
	}
	cdsHome, err := home()
	if err != nil {
		return err
	}
	version, err := productVersion(cdsHome)
	if err != nil {
		return err
	}
	fmt.Println(version)
	return nil
}
