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

import "fmt"

// runDisable removes the CDS integration from the associated Identity Server.
//
// It undoes what `enable` wrote into IS_HOME and nothing else: the CDS
// installation, its database and its configuration stay as they are, and the
// applications, claims and roles provisioned over the management APIs are
// left alone because they may hold data an operator still wants. What this
// removes is what stops the Identity Server from calling CDS.
func runDisable(args []string) error {

	flags := newFlagSet("disable")
	isHome := flags.String("is-home", "", "the Identity Server to disable CDS on (default: the recorded one)")
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

	target := state.ISHome
	if *isHome != "" {
		target = *isHome
	}

	step("Identity Server")
	is, err := openISHome(target)
	if err != nil {
		return err
	}
	info("%s", is.home)

	step("Removing the CDS extension bundles")
	if len(state.Extensions) == 0 {
		info("CDS installed no bundles into %s", isDropins)
	} else {
		removed := is.removeExtensions(state.Extensions)
		done("removed %d of %d bundle(s) from %s", removed, len(state.Extensions), isDropins)
	}
	// Bundles that came with the Identity Server are delivered and updated
	// through its own stream. Removing one would leave that installation
	// short of a file it shipped with.
	if len(state.BundledExtensions) > 0 {
		info("%d bundle(s) came with the Identity Server and are left in place", len(state.BundledExtensions))
	}

	step("Removing the CDS configuration")
	found, err := is.removeTOMLSection()
	if err != nil {
		return err
	}
	if found {
		done("removed the generated CDS section from %s", isDeploymentTOML)
	} else {
		info("%s had no generated CDS section", isDeploymentTOML)
	}

	if err := clearIntegration(cdsHome); err != nil {
		return err
	}

	step("Disabled")
	info("The CDS installation at %s is untouched, including its data.", cdsHome)
	info("The Identity Server stops calling CDS only once it restarts:")
	info("")
	info("    %s/bin/wso2server.sh restart", is.home)
	info("")
	info("The applications, claims and role permissions provisioning created are left in place.")
	fmt.Println()
	return nil
}
