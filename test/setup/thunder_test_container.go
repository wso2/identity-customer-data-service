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

package setup

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// thunderImage is WSO2 Thunder's official prebuilt image (embedded
	// SQLite, self-signed TLS cert, default port 8090). See
	// docs/guides/identity-providers.md for background.
	thunderImage = "ghcr.io/thunder-id/thunderid:latest"

	// ThunderTestClientID/ThunderTestClientSecret match the client_credentials
	// application bootstrapped by thunderBootstrapYAML below.
	ThunderTestClientID     = "cds-test-client"
	ThunderTestClientSecret = "cds-test-secret"
)

// thunderBootstrapYAML defines a single client_credentials OAuth2 application
// in Thunder's bootstrap-resource format (see /opt/thunderid/bootstrap/ in
// the image). It's layered on top of Thunder's own default bootstrap
// resources (which include the default organization unit referenced by
// ouId below) via Thunder's one-shot `setup.sh`.
const thunderBootstrapYAML = `# resource_type: application
id: 01900000-0000-7000-8000-000000000070
name: CDS Test Client
ouId: 01900000-0000-7000-8000-000000000001
allowedUserTypes:
  - Person
inboundAuthConfig:
  - type: oauth2
    config:
      clientId: ` + ThunderTestClientID + `
      clientSecret: ` + ThunderTestClientSecret + `
      grantTypes:
        - client_credentials
      tokenEndpointAuthMethod: client_secret_basic
      publicClient: false
      scopes:
        - system
`

// TestThunder holds the running Thunder testcontainer and the address CDS's
// thunder.Client should connect to. Thunder serves HTTPS with a self-signed
// certificate, so callers must use an HTTP client with certificate
// verification disabled (test-only; production code never does this — see
// internal/system/utils.NewOutboundHTTPClient).
type TestThunder struct {
	Container    testcontainers.Container
	BaseURL      string // host:port
	ClientID     string
	ClientSecret string
}

// SetupTestThunder starts a WSO2 Thunder container pre-loaded with a single
// client_credentials application (ThunderTestClientID/ThunderTestClientSecret)
// and returns once the server has finished its one-shot setup and is
// listening. Requires Docker.
//
// Known limitation reproduced by this fixture (see
// docs/guides/identity-providers.md): Thunder's role-assignment model only
// accepts user/group assignees, not applications, so a client_credentials
// token cannot be granted the "system" role — meaning calls to Thunder's
// /applications management API always return 403 regardless of the
// requested scope. Tests against FetchApplicationIdentifier must expect
// that 403, not success.
func SetupTestThunder(ctx context.Context) (*TestThunder, error) {
	bootstrapFile, err := os.CreateTemp("", "thunder-bootstrap-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create Thunder bootstrap fixture: %w", err)
	}
	defer os.Remove(bootstrapFile.Name())
	if _, err := bootstrapFile.WriteString(thunderBootstrapYAML); err != nil {
		bootstrapFile.Close()
		return nil, fmt.Errorf("failed to write Thunder bootstrap fixture: %w", err)
	}
	bootstrapFile.Close()

	req := testcontainers.ContainerRequest{
		Image:        thunderImage,
		ExposedPorts: []string{"8090/tcp"},
		Entrypoint:   []string{"sh", "-c"},
		Cmd:          []string{"./setup.sh && ./start.sh"},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      bootstrapFile.Name(),
				ContainerFilePath: "/opt/thunderid/bootstrap/02-cds-test-client.yaml",
				FileMode:          0o644,
			},
		},
		// Emitted only after setup.sh's bootstrap completes and the server
		// finishes starting - a plain port-open check would race the setup step.
		WaitingFor: wait.ForLog("ThunderID Server started"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start Thunder container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get Thunder container host: %w", err)
	}
	port, err := container.MappedPort(ctx, "8090")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get Thunder container port: %w", err)
	}

	baseURL := fmt.Sprintf("%s:%s", host, port.Port())
	log.Printf("Thunder container started at %s", baseURL)

	return &TestThunder{
		Container:    container,
		BaseURL:      baseURL,
		ClientID:     ThunderTestClientID,
		ClientSecret: ThunderTestClientSecret,
	}, nil
}
