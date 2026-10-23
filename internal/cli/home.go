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
	"path/filepath"
	"strings"
)

// Paths inside an installed pack. The layout is fixed: the executable and the
// entry-point scripts sit at CDS_HOME, with repository/, logs/ and updates/
// beside them. bin/ holds only update_tool_setup.sh, which resolves
// ../updates/product.txt relative to itself and so has to stay one level
// below updates/.
const (
	binDir        = "bin"
	confDir       = "repository/conf"
	dbScriptsDir  = "repository/dbscripts"
	logsDir       = "logs"
	updatesDir    = "updates"
	configFile    = "repository/conf/deployment.yaml"
	secretsFile   = "repository/conf/secrets.env"
	stateFile     = "repository/conf/integration.yaml"
	productFile   = "updates/product.txt"
	versionFile   = "version.txt"
	pidFile       = "cds.pid"
	runtimeLog    = "logs/cds.log"
	certsDirInPak = "repository/resources/security"
)

// home resolves CDS_HOME.
//
// The executable's own location is the primary source, because a pack is
// relocatable and an operator who extracted it anywhere should not have to
// export anything to use it. CDS_HOME overrides that for deployments which run
// the binary from outside the pack.
func home() (string, error) {

	if env := strings.TrimSpace(os.Getenv("CDS_HOME")); env != "" {
		abs, err := filepath.Abs(env)
		if err != nil {
			return "", fmt.Errorf("CDS_HOME %q is not a usable path: %w", env, err)
		}
		return abs, nil
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not locate the cds executable, so CDS_HOME is unknown: %w", err)
	}
	// Resolve symlinks so that a cds on PATH, linked into the pack, still
	// resolves to the pack rather than to the link's directory.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	// <CDS_HOME>/cds -> <CDS_HOME>
	dir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(dir, configFile)); err != nil {
		cwd, wdErr := os.Getwd()
		if wdErr == nil {
			if _, err := os.Stat(filepath.Join(cwd, configFile)); err == nil {
				return cwd, nil
			}
		}
		return "", fmt.Errorf("%s does not look like a CDS installation: %s is missing.\n"+
			"Run cds from the root of an extracted pack, or set CDS_HOME", dir, configFile)
	}
	return dir, nil
}

// path joins one pack-relative path onto CDS_HOME.
func path(cdsHome, rel string) string { return filepath.Join(cdsHome, filepath.FromSlash(rel)) }

// productVersion reads version.txt from the pack, without its leading "v".
// Packs always carry it; a source tree may not, which is why the error names
// the file rather than failing silently.
func productVersion(cdsHome string) (string, error) {

	raw, err := os.ReadFile(path(cdsHome, versionFile))
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", versionFile, err)
	}
	return strings.TrimPrefix(strings.TrimSpace(string(raw)), "v"), nil
}

// u2Product reads the U2 product identity this pack was built as, for example
// cds-linux-amd64-1.2.0. It is absent from a pack that has not been
// onboarded to the update service, which is not an error.
func u2Product(cdsHome string) string {

	raw, err := os.ReadFile(path(cdsHome, productFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// ---------------------------------------------------------------------------
// Terminal output
//
// Lifecycle commands are read by an operator watching a long-running sequence,
// so each step says what it is doing before it does it. Colour is dropped when
// the output is not a terminal, so logs and CI output stay plain.
// ---------------------------------------------------------------------------

var useColour = func() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	info, err := os.Stdout.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}()

func colour(code, s string) string {
	if !useColour {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

func red(s string) string  { return colour("31", s) }
func bold(s string) string { return colour("1", s) }

func step(format string, a ...any) { fmt.Printf("\n%s\n", bold(fmt.Sprintf(format, a...))) }
func info(format string, a ...any) { fmt.Printf("    %s\n", fmt.Sprintf(format, a...)) }
func done(format string, a ...any) {
	fmt.Printf("    %s %s\n", colour("32", "+"), fmt.Sprintf(format, a...))
}
func warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "    %s %s\n", colour("33", "!"), fmt.Sprintf(format, a...))
}
