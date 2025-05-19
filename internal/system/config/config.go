package config

import (
	"os"
	"path"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Addr struct {
		Port int    `yaml:"port"`
		Host string `yaml:"host"`
	} `yaml:"addr"`
	Log struct {
		LogLevel string `yaml:"log_level"`
	} `yaml:"log"`
	Auth struct {
		CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
	} `yaml:"auth"`
	AuthServer struct {
		IntrospectionEndPoint string `yaml:"introspectionEndpoint"`
		TokenEndpoint         string `yaml:"tokenEndpoint"`
		RevocationEndpoint    string `yaml:"revocationEndpoint"`
		ClientID              string `yaml:"client_id"`
		ClientSecret          string `yaml:"client_secret"`
		AdminUsername         string `yaml:"admin_username"`
		AdminPassword         string `yaml:"admin_password"`
	} `yaml:"auth_server"`
	DatabaseConfig struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DbName   string `yaml:"dbname"`
	} `yaml:"database"`
	DataSource struct {
		Hostname string `yaml:"hostname"`
		Port     int    `yaml:"port"`
		Name     string `yaml:"name"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		SSLMode  string `yaml:"sslmode"`
	} `yaml:"datasource"`
}

// LoadConfig loads and sets AppConfig (global variable)
func LoadConfig(cdsHome, filePath string) (*Config, error) {
	file, err := os.ReadFile(path.Join(cdsHome, filePath))
	if err != nil {
		return nil, err
	}

	expanded := os.ExpandEnv(string(file))

	var config Config
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, err
	}

	return &config, nil
}
