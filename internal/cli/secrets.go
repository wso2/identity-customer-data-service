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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Secrets are kept out of both deployment.yaml and the integration record, and
// reach the runtime as environment variables, which is what the ${...}
// placeholders in the shipped configuration expand from.
//
// This file is the installation's own store for them, readable by the account
// that runs CDS and nobody else. A deployment with an approved secret manager
// supplies the same variables from there instead and can delete it.
const secretsHeader = `# Written by ` + "`wso2cds enable`" + `. Holds the credentials the CDS runtime needs,
# which the ${...} placeholders in repository/conf/deployment.yaml expand from.
#
# wso2cds start passes these to the runtime. A deployment that supplies them
# from a secret manager can remove this file.
`

// loadSecrets reads the KEY=VALUE pairs. A missing file yields an empty map:
// an installation whose secrets come from elsewhere is not broken.
func loadSecrets(cdsHome string) (map[string]string, error) {

	file, err := os.Open(path(cdsHome, secretsFile))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("could not read %s: %w", secretsFile, err)
	}
	defer func() { _ = file.Close() }()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("could not read %s: %w", secretsFile, err)
	}
	return values, nil
}

// saveSecrets merges values into the file and writes it back with owner-only
// permissions. Merging rather than replacing keeps a secret an operator added
// by hand, and keeps the credentials from an earlier phase of enablement when
// a later phase only has some of them.
func saveSecrets(cdsHome string, values map[string]string) error {

	existing, err := loadSecrets(cdsHome)
	if err != nil {
		return err
	}
	for key, value := range values {
		if value == "" {
			continue
		}
		existing[key] = value
	}

	keys := make([]string, 0, len(existing))
	for key := range existing {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var out strings.Builder
	out.WriteString(secretsHeader)
	for _, key := range keys {
		fmt.Fprintf(&out, "%s=%s\n", key, existing[key])
	}
	return writeFileAtomic(path(cdsHome, secretsFile), []byte(out.String()), 0o600)
}

// environment turns the stored secrets into KEY=VALUE strings for a child
// process, leaving any variable already set in the environment alone so that a
// deployment can override one without editing the file.
func environment(values map[string]string) []string {

	env := make([]string, 0, len(values))
	for key, value := range values {
		if _, set := os.LookupEnv(key); set {
			continue
		}
		env = append(env, key+"="+value)
	}
	sort.Strings(env)
	return env
}

// generatedSecret produces a credential for the IS-to-CDS sync account. It is
// generated per installation so that two deployments never share one, and it
// is never printed.
func generatedSecret() (string, error) {

	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("could not generate a credential: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
