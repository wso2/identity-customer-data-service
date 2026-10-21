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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// deployment.toml belongs to the operator. Enablement owns the lines between
// its markers and nothing else, so these cases are about what survives.
func TestStripTOMLSection(t *testing.T) {

	cases := []struct {
		name      string
		content   string
		want      string
		wantFound bool
	}{
		{
			name:      "a file with no section is unchanged",
			content:   "[server]\nhostname = \"localhost\"\n",
			want:      "[server]\nhostname = \"localhost\"\n",
			wantFound: false,
		},
		{
			name: "only the marked lines are removed",
			content: "[server]\nhostname = \"localhost\"\n" +
				tomlBeginMark + "\n[customer_data_service]\nenable = true\n" + tomlEndMark + "\n" +
				"[other]\nkeep = true\n",
			want:      "[server]\nhostname = \"localhost\"\n[other]\nkeep = true\n",
			wantFound: true,
		},
		{
			name: "a marker written by an earlier version still matches",
			content: tomlBeginMark + " — generated, do not edit\nenable = true\n" +
				tomlEndMark + " — end\n[keep]\n",
			want:      "[keep]\n",
			wantFound: true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, found := stripTOMLSection(test.content)
			if found != test.wantFound {
				t.Errorf("found = %v, want %v", found, test.wantFound)
			}
			if got != test.want {
				t.Errorf("content =\n%q\nwant\n%q", got, test.want)
			}
		})
	}
}

// Two versions of one bundle in dropins would leave OSGi to pick, so the
// symbolic name has to be recovered from the filename exactly.
func TestBundleName(t *testing.T) {

	cases := map[string]string{
		"org.wso2.identity.customer.data.service.client-0.1.13.jar":          "org.wso2.identity.customer.data.service.client",
		"org.wso2.identity.customer.data.service.client-0.1.13-SNAPSHOT.jar": "org.wso2.identity.customer.data.service.client",
		"org.wso2.identity.customer.data.service.event.handler-1.0.0.jar":    "org.wso2.identity.customer.data.service.event.handler",
		"no-version.jar": "no-version.jar",
	}

	for filename, want := range cases {
		if got := bundleName(filename); got != want {
			t.Errorf("bundleName(%q) = %q, want %q", filename, got, want)
		}
	}
}

func TestReadISVersionPattern(t *testing.T) {

	cases := map[string]string{
		"wso2is-7.3.0":                "7.3.0",
		"WSO2 Identity Server v7.4.0": "7.4.0",
		"<Version>7.3.0</Version>":    "7.3.0",
		"no version here":             "",
	}

	for content, want := range cases {
		if got := isVersionPattern.FindString(content); got != want {
			t.Errorf("version of %q = %q, want %q", content, got, want)
		}
	}
}

func TestStripTOMLSectionHandlesLongLines(t *testing.T) {

	// deployment.toml can carry a very long single line — a wide
	// allowed_origins list, say. The scanner has to read it rather than fail.
	long := strings.Repeat("x", 200_000)
	content := "[server]\nvalue = \"" + long + "\"\n" + tomlBeginMark + "\ngenerated = true\n" + tomlEndMark + "\n"

	got, found := stripTOMLSection(content)
	if !found {
		t.Fatal("the generated section was not found")
	}
	if !strings.Contains(got, long) {
		t.Error("the long line did not survive")
	}
	if strings.Contains(got, "generated = true") {
		t.Error("the generated section was not removed")
	}
}

// A bundle the Identity Server provides is not CDS's to replace or remove,
// so the version has to be readable from the filename to be compared against
// the baseline this release expects.
func TestBundleVersion(t *testing.T) {

	cases := map[string]string{
		"org.wso2.identity.customer.data.service.client-0.1.13.jar":          "0.1.13",
		"org.wso2.identity.customer.data.service.client-0.9.0.1.jar":         "0.9.0.1",
		"org.wso2.identity.customer.data.service.client-0.1.13-SNAPSHOT.jar": "0.1.13-SNAPSHOT",
		"no-version.jar": "",
	}

	for filename, want := range cases {
		if got := bundleVersion(filename); got != want {
			t.Errorf("bundleVersion(%q) = %q, want %q", filename, got, want)
		}
	}
}

// From the Identity Server release that ships CDS support, the bundles are
// already in dropins and belong to that product's update stream. Enablement
// has to see them so it neither re-downloads them nor takes them over.
func TestBundlesInDropins(t *testing.T) {

	isHome := t.TempDir()
	dropins := filepath.Join(isHome, filepath.FromSlash(isDropins))
	if err := os.MkdirAll(dropins, 0o755); err != nil {
		t.Fatalf("could not stage dropins: %v", err)
	}

	bundles := []string{
		"org.wso2.identity.customer.data.service.client",
		"org.wso2.identity.customer.data.service.event.handler",
		"org.wso2.identity.customer.data.service.auth.listener",
	}
	// Two of the three, plus an unrelated connector that must not be matched.
	for _, name := range []string{
		"org.wso2.identity.customer.data.service.client-0.9.0.jar",
		"org.wso2.identity.customer.data.service.event.handler-0.9.0.jar",
		"org.wso2.carbon.identity.application.authenticator.apple-1.1.2.jar",
	} {
		if err := os.WriteFile(filepath.Join(dropins, name), nil, 0o644); err != nil {
			t.Fatalf("could not stage %s: %v", name, err)
		}
	}

	is := &isInstall{home: isHome, version: "7.4.0"}
	found, err := is.bundlesInDropins(bundles)
	if err != nil {
		t.Fatalf("bundlesInDropins: %v", err)
	}

	if len(found) != 2 {
		t.Fatalf("found %d bundles, want 2: %v", len(found), found)
	}
	if found["org.wso2.identity.customer.data.service.client"] !=
		"org.wso2.identity.customer.data.service.client-0.9.0.jar" {
		t.Errorf("the client bundle was not matched: %v", found)
	}
	if _, matched := found["org.wso2.identity.customer.data.service.auth.listener"]; matched {
		t.Error("a bundle that is not in dropins was reported as present")
	}
}

func TestBundlesInDropinsWithoutADropinsDirectory(t *testing.T) {

	is := &isInstall{home: t.TempDir(), version: "7.3.0"}
	found, err := is.bundlesInDropins([]string{"org.wso2.identity.customer.data.service.client"})
	if err != nil {
		t.Fatalf("an Identity Server with no dropins directory should not be an error: %v", err)
	}
	if len(found) != 0 {
		t.Errorf("expected no bundles, got %v", found)
	}
}
