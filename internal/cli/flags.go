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
	"flag"
	"fmt"
	"os"

	"github.com/wso2/identity-customer-data-service/internal/system/server"
)

// newFlagSet builds the flag set for one subcommand. Errors are returned
// rather than exiting, so Main decides the exit code in one place.
func newFlagSet(name string) *flag.FlagSet {

	flags := flag.NewFlagSet("wso2cds "+name, flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: wso2cds %s [options]\n\nOptions:\n", name)
		flags.PrintDefaults()
	}
	return flags
}

// parse applies the arguments and converts a parse failure into errUsage,
// which flag has already explained on stderr.
func parse(flags *flag.FlagSet, args []string) error {

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return errUsage
		}
		return errUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "unexpected argument %q\n", flags.Arg(0))
		flags.Usage()
		return errUsage
	}
	return nil
}

// runtimeRun hands control to the server runtime. It is a variable so that
// tests of the lifecycle commands can stand in for a runtime they must not
// actually start.
var runtimeRun = func(cdsHome string) { server.Run(cdsHome) }
