package config

import (
	"gopkg.in/yaml.v2"
	"os"
)

var AppConfig *Config

type Config struct {
	MongoDB struct {
		URI               string `yaml:"uri"`
		Database          string `yaml:"database"`
		ProfileCollection string `yaml:"profile_collection"`
		EventCollection   string `yaml:"event_collection"`
		ConsentCollection string `yaml:"consent_collection"`
	} `yaml:"mongodb"`
	Addr struct {
		Port string `yaml:"port"`
		Host string `yaml:"host"`
	} `yaml:"addr"`
	Log struct {
		DebugEnabled bool `yaml:"debug_enabled"`
	} `yaml:"log"`
	Auth struct {
		CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
	} `yaml:"auth"`
	IdentityServer struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"identity_server"`
}

// LoadConfig loads and sets AppConfig (global variable)
func LoadConfig(filePath string) (*Config, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	expanded := os.ExpandEnv(string(file))

	var config Config
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, err
	}

	AppConfig = &config
	return AppConfig, nil
}
