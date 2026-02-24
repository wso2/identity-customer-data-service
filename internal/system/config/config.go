/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
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

package config

type AddrConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type LogConfig struct {
	LogLevel string `yaml:"log_level"`
}

type AuthConfig struct {
	CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
}

type AuthServerConfig struct {
	Host                      string              `yaml:"host"`
	Port                      string              `yaml:"port"`
	ADUISHostname             string              `yaml:"adu_is_hostname"`
	CookieDomain              string              `yaml:"cookieDomain"`
	IntrospectionEndPoint     string              `yaml:"introspectionEndpoint"`
	TokenEndpoint             string              `yaml:"tokenEndpoint"`
	RevocationEndpoint        string              `yaml:"revocationEndpoint"`
	ClaimEndpoint             string              `yaml:"claim_endpoint"`
	ClientID                  string              `yaml:"client_id"`
	ClientSecret              string              `yaml:"client_secret"`
	IntrospectionClientId     string              `yaml:"introspection_client_id"`
	IntrospectionClientSecret string              `yaml:"introspection_client_secret"`
	AdminUsername             string              `yaml:"admin_username"`
	AdminPassword             string              `yaml:"admin_password"`
	RequiredScopes            map[string][]string `yaml:"required_scopes"`
	IsSystemAppGrantEnabled   bool                `yaml:"isSystemAppGrantEnabled"`
}

type DataSourceConfig struct {
	Type     string `yaml:"type"` // e.g., "postgres", "mysql"
	Hostname string `yaml:"hostname"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"sslmode"`
}

// ActiveMQConfig holds the connection settings for an ActiveMQ broker.
type ActiveMQConfig struct {
	// Addr is the broker address in "host:port" format (e.g. "localhost:61613").
	Addr string `yaml:"addr"`
	// Username is the STOMP login name.
	Username string `yaml:"username"`
	// Password is the STOMP passcode.
	Password string `yaml:"password"`
	// ProfileQueueName is the STOMP destination used for profile unification
	// messages (e.g. "/queue/profile-unification").
	ProfileQueueName string `yaml:"profile_queue_name"`
	// SchemaSyncQueueName is the STOMP destination used for schema sync
	// messages (e.g. "/queue/schema-sync").
	SchemaSyncQueueName string `yaml:"schema_sync_queue_name"`
}

// MessageQueueConfig selects the queue provider and its settings. When Type
// is empty or "memory" the built-in in-memory queue is used. Set Type to
// "activemq" to use the ActiveMQ provider.
type MessageQueueConfig struct {
	// Type is the queue provider to use: "memory" (default) or "activemq".
	Type     string         `yaml:"type"`
	ActiveMQ ActiveMQConfig `yaml:"activemq"`
}

type Config struct {
	Addr         AddrConfig         `yaml:"addr"`
	Log          LogConfig          `yaml:"log"`
	Auth         AuthConfig         `yaml:"auth"`
	AuthServer   AuthServerConfig   `yaml:"auth_server"`
	DataSource   DataSourceConfig   `yaml:"datasource"`
	TLS          TLSConfig          `yaml:"tls"`
	MessageQueue MessageQueueConfig `yaml:"message_queue"`
}

type TLSConfig struct {
	MTLSEnabled             bool   `yaml:"mtls_enabled"`
	CertDir                 string `yaml:"cert_dir"`
	CDSPublicCert           string `yaml:"server_cert"`
	CDSPrivateKey           string `yaml:"server_key"`
	IdentityServerPublicKey string `yaml:"client_cert"`
	TrustStore              string `yaml:"trust_store"`
}
