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
	InternalHost              string              `yaml:"internalHost"`
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

type Config struct {
	Addr       AddrConfig       `yaml:"addr"`
	Log        LogConfig        `yaml:"log"`
	Auth       AuthConfig       `yaml:"auth"`
	AuthServer AuthServerConfig `yaml:"auth_server"`
	DataSource DataSourceConfig `yaml:"datasource"`
	TLS        TLSConfig        `yaml:"tls"`
}

type TLSConfig struct {
	MTLSEnabled             bool   `yaml:"mtls_enabled"`
	CertDir                 string `yaml:"cert_dir"`
	CDSPublicCert           string `yaml:"server_cert"`
	CDSPrivateKey           string `yaml:"server_key"`
	IdentityServerPublicKey string `yaml:"client_cert"`
	TrustStore              string `yaml:"trust_store"`
}
