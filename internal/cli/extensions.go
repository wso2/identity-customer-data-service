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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// extensionsDir is where downloaded bundles are cached inside the pack, and
// where an air-gapped installation can place them by hand.
const extensionsDir = "extensions"

// resolveExtensions produces the local paths of the extension jars to install.
//
// want is the subset of the baseline that has to be installed — the bundles
// the Identity Server does not already provide — so that nothing is fetched
// for a bundle that is already in place.
//
// Three sources, in order: a directory the operator named, the pack's own
// extensions directory, and the Maven repository named by the compatibility
// metadata. The first two come before the network so that an air-gapped
// installation never depends on it, and so that a developer testing an
// unreleased bundle can override the baseline without editing metadata.
func resolveExtensions(cdsHome string, baseline extensionBaseline, override string, want []string) ([]string, error) {

	if len(want) == 0 {
		return nil, nil
	}

	if override != "" {
		if _, err := os.Stat(override); err != nil {
			return nil, fmt.Errorf("--extensions %s: %w", override, err)
		}
		return findBundles(override, want)
	}

	local := path(cdsHome, extensionsDir)
	if jars, err := findBundles(local, want); err == nil {
		info("using the bundles already in %s/", extensionsDir)
		return jars, nil
	}

	if baseline.Version == "" || baseline.Repository == "" || baseline.Group == "" {
		return nil, fmt.Errorf("%s does not name an extension version, repository and group, "+
			"so the bundles cannot be resolved. Place them in %s/ or pass --extensions <dir>",
			compatibilityFile, extensionsDir)
	}
	return downloadBundles(local, baseline, want)
}

// findBundles locates one jar per bundle in a directory, ignoring the sources
// and javadoc artifacts a local Maven build leaves beside them.
func findBundles(dir string, bundles []string) ([]string, error) {

	jars := make([]string, 0, len(bundles))
	for _, bundle := range bundles {
		matches, err := filepath.Glob(filepath.Join(dir, bundle+"-*.jar"))
		if err != nil {
			return nil, fmt.Errorf("could not search %s: %w", dir, err)
		}
		found := ""
		for _, match := range matches {
			name := filepath.Base(match)
			if strings.HasSuffix(name, "-sources.jar") || strings.HasSuffix(name, "-javadoc.jar") {
				continue
			}
			found = match
			break
		}
		if found == "" {
			return nil, fmt.Errorf("no jar for %s in %s", bundle, dir)
		}
		jars = append(jars, found)
	}
	return jars, nil
}

// downloadBundles fetches the baseline from the Maven repository into the
// pack, so a later enable or re-enable needs no network.
func downloadBundles(into string, baseline extensionBaseline, want []string) ([]string, error) {

	if err := os.MkdirAll(into, 0o755); err != nil {
		return nil, fmt.Errorf("could not create %s: %w", extensionsDir, err)
	}

	groupPath := strings.ReplaceAll(baseline.Group, ".", "/")
	repository := strings.TrimSuffix(baseline.Repository, "/")
	client := &http.Client{Timeout: 5 * time.Minute}

	jars := make([]string, 0, len(want))
	for _, bundle := range want {
		name := fmt.Sprintf("%s-%s.jar", bundle, baseline.Version)
		target := filepath.Join(into, name)
		if _, err := os.Stat(target); err == nil {
			info("%s (cached)", name)
			jars = append(jars, target)
			continue
		}

		url := fmt.Sprintf("%s/%s/%s/%s/%s", repository, groupPath, bundle, baseline.Version, name)
		if err := download(client, url, target); err != nil {
			return nil, fmt.Errorf("could not download %s: %w\n"+
				"Place the extension bundles in %s/ and re-run, if this installation has no "+
				"access to the Maven repository", name, err, extensionsDir)
		}
		info("%s", name)
		jars = append(jars, target)
	}
	return jars, nil
}

func download(client *http.Client, url, target string) error {

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP %d", url, resp.StatusCode)
	}

	// Stage beside the target so an interrupted download never leaves a
	// truncated jar that a later run would treat as cached.
	tmp, err := os.CreateTemp(filepath.Dir(target), ".download-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}
