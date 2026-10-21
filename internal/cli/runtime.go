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
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// runtimeConfig is the part of repository/conf/deployment.yaml the CLI needs:
// where to reach the running server, and which Identity Server the runtime is
// configured to call. The runtime itself loads the whole file; the CLI reads
// only these fields so that an unresolved ${PLACEHOLDER} elsewhere in the file
// — normal before the first start — cannot stop `status` from answering.
type runtimeConfig struct {
	Addr struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"addr"`
	ServerURL  string `yaml:"server_url"`
	AuthServer struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		ClientID string `yaml:"client_id"`
	} `yaml:"auth_server"`
}

func readRuntimeConfig(cdsHome string) (*runtimeConfig, error) {

	raw, err := os.ReadFile(path(cdsHome, configFile))
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", configFile, err)
	}

	var cfg runtimeConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", configFile, err)
	}
	if cfg.Addr.Port == 0 {
		return nil, fmt.Errorf("%s sets no addr.port, so the CDS endpoint is unknown", configFile)
	}
	if cfg.Addr.Host == "" {
		cfg.Addr.Host = "127.0.0.1"
	}
	return &cfg, nil
}

// localBase is the address the CLI itself calls. It is always the configured
// bind address rather than server_url, because server_url is the public name
// an operator may have pointed at a load balancer — a liveness check has to
// reach this process, not the deployment.
func (c *runtimeConfig) localBase() string {

	host := c.Addr.Host
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "https://" + net.JoinHostPort(host, strconv.Itoa(c.Addr.Port))
}

// localClient calls the CDS process over its own loopback address. CDS serves
// HTTPS only, and packs are installed with a certificate issued for the
// deployment's public name, which will not match 127.0.0.1. Verification is
// therefore skipped for these local lifecycle probes only; nothing else in the
// CLI reaches a remote host with this client.
func localClient(timeout time.Duration) *http.Client {

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, // #nosec G402
		},
	}
}

// probe reports the HTTP status of one CDS endpoint, or an error if the
// process could not be reached at all.
func probe(base, endpoint string, timeout time.Duration) (int, string, error) {

	resp, err := localClient(timeout).Get(base + endpoint)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body := make([]byte, 512)
	n, _ := resp.Body.Read(body)
	return resp.StatusCode, strings.TrimSpace(string(body[:n])), nil
}

// waitReady polls /ready until the process reports that it can serve, or the
// deadline passes. Readiness rather than health is the signal, because a
// process that is listening but cannot reach its datastore has not finished
// starting from an operator's point of view.
//
// The started process is watched as well as the endpoint. A port is not proof
// of anything on its own: if the runtime exits — a configuration it cannot
// read, a port already taken — something else answering on that address would
// otherwise be reported as this installation having started.
func waitReady(base string, exited <-chan error, within time.Duration) error {

	deadline := time.Now().Add(within)
	var last error
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			if err != nil {
				return fmt.Errorf("the CDS runtime exited during start-up: %w", err)
			}
			return fmt.Errorf("the CDS runtime exited during start-up")
		default:
		}

		status, _, err := probe(base, "/cds/api/v1/ready", 5*time.Second)
		switch {
		case err != nil:
			last = err
		case status == http.StatusOK:
			return nil
		default:
			last = fmt.Errorf("/cds/api/v1/ready returned HTTP %d", status)
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("CDS did not become ready within %s: %w", within, last)
}
