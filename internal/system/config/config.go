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
	IntrospectionEndPoint string `yaml:"introspectionEndpoint"`
	TokenEndpoint         string `yaml:"tokenEndpoint"`
	RevocationEndpoint    string `yaml:"revocationEndpoint"`
	ClientID              string `yaml:"client_id"`
	ClientSecret          string `yaml:"client_secret"`
	AdminUsername         string `yaml:"admin_username"`
	AdminPassword         string `yaml:"admin_password"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DbName   string `yaml:"dbname"`
}

type DataSourceConfig struct {
	Hostname string `yaml:"hostname"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"sslmode"`
}

type Config struct {
	Addr           AddrConfig       `yaml:"addr"`
	Log            LogConfig        `yaml:"log"`
	Auth           AuthConfig       `yaml:"auth"`
	AuthServer     AuthServerConfig `yaml:"auth_server"`
	DatabaseConfig DatabaseConfig   `yaml:"database"`
	DataSource     DataSourceConfig `yaml:"datasource"`
}
