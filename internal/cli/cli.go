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

// Package cli implements bin/wso2cds, the command installed packs ship.
//
// Installation and enablement are deliberately separate. Extracting the pack
// establishes CDS_HOME; `enable` establishes the relationship between CDS and
// one Identity Server. The lifecycle commands then operate on the CDS process
// alone — IS is a separately managed installation and is never started,
// stopped or updated from here.
package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// command is one subcommand of wso2cds.
type command struct {
	name string
	// summary is the one-line description shown by `wso2cds help`.
	summary string
	// run receives the arguments that follow the subcommand name.
	run func(args []string) error
	// hidden keeps internal entry points out of the help text.
	hidden bool
}

// errUsage is returned by a command whose arguments did not parse. Main prints
// the command's own usage rather than a stack of flag errors.
var errUsage = errors.New("invalid arguments")

func commands() []command {

	return []command{
		{name: "enable", summary: "Enable CDS against an Identity Server installation", run: runEnable},
		{name: "disable", summary: "Remove the CDS integration from the associated Identity Server", run: runDisable},
		{name: "start", summary: "Start the CDS runtime in the foreground (--daemon to detach)", run: runStart},
		{name: "stop", summary: "Stop a CDS runtime started with --daemon", run: runStop},
		{name: "status", summary: "Show product, platform, runtime and integration state", run: runStatus},
		{name: "logs", summary: "Display or follow the CDS logs", run: runLogs},
		{name: "verify", summary: "Verify CDS and its integration with the Identity Server", run: runVerify},
		{name: "version", summary: "Print the CDS version", run: runVersion},

		// The foreground runtime. `start` re-executes the pack binary with this
		// command so that a detached server and a --debug run share one code
		// path, and so the pack ships a single executable.
		{name: "run", summary: "", run: runForeground, hidden: true},
	}
}

// Main runs one subcommand and returns the process exit code.
func Main(args []string) int {

	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage(os.Stdout)
		return 0
	}

	name := args[0]
	for _, cmd := range commands() {
		if cmd.name != name {
			continue
		}
		err := cmd.run(args[1:])
		if err == nil {
			return 0
		}
		if errors.Is(err, errUsage) {
			return 2
		}
		fmt.Fprintf(os.Stderr, "\n%s %v\n", red("Error:"), err)
		return 1
	}

	fmt.Fprintf(os.Stderr, "%s unknown command %q\n\n", red("Error:"), name)
	usage(os.Stderr)
	return 2
}

func usage(out *os.File) {

	var b strings.Builder
	b.WriteString("WSO2 Customer Data Service\n\nUsage:\n  wso2cds <command> [options]\n\nCommands:\n")
	for _, cmd := range commands() {
		if cmd.hidden {
			continue
		}
		fmt.Fprintf(&b, "  %-9s %s\n", cmd.name, cmd.summary)
	}
	b.WriteString("\nRun `wso2cds <command> --help` for the options of one command.\n")
	fmt.Fprint(out, b.String())
}
