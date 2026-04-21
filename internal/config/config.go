package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type EndpointConfig struct {
	Method string `yaml:"method"`
	Path   string `yaml:"path"`
}

type RequireConfig struct {
	ServiceName     string           `yaml:"serviceName"`
	Branch          string           `yaml:"branch,omitempty"`
	OutputDirectory string           `yaml:"outputDirectory"`
	Timeout         int              `yaml:"timeout,omitempty"`
	Endpoints       []EndpointConfig `yaml:"endpoints"`
}

type ProvideConfig struct {
	ServiceName string `yaml:"serviceName"`
	OpenApiFile string `yaml:"openApiFile"`
	Branch      string `yaml:"branch,omitempty"`
}

type SanshainConfig struct {
	SanshainURL string          `yaml:"sanshainUrl"`
	ClientName  string          `yaml:"clientName"`
	Timeout     int             `yaml:"timeout,omitempty"`
	Compression bool            `yaml:"compression,omitempty"`
	Provide     *ProvideConfig  `yaml:"provide,omitempty"`
	Requires    []RequireConfig `yaml:"requires,omitempty"`
}

func LoadConfig(configPath string) (*SanshainConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config SanshainConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func validateConfig(config *SanshainConfig) error {
	if config.SanshainURL == "" {
		return fmt.Errorf("missing required field: sanshainUrl")
	}
	if config.ClientName == "" {
		return fmt.Errorf("missing required field: clientName")
	}

	if config.Provide != nil {
		if config.Provide.ServiceName == "" {
			return fmt.Errorf("missing required field in provide: serviceName")
		}
		if config.Provide.OpenApiFile == "" {
			return fmt.Errorf("missing required field in provide: openApiFile")
		}
	}

	for i, req := range config.Requires {
		if req.ServiceName == "" {
			return fmt.Errorf("missing serviceName in requires[%d]", i)
		}
		if req.OutputDirectory == "" {
			return fmt.Errorf("missing outputDirectory in requires[%d]", i)
		}
		if len(req.Endpoints) == 0 {
			return fmt.Errorf("missing or empty endpoints in requires[%d]", i)
		}
		for j, endpoint := range req.Endpoints {
			if endpoint.Method == "" {
				return fmt.Errorf("missing method in requires[%d].endpoints[%d]", i, j)
			}
			if endpoint.Path == "" {
				return fmt.Errorf("missing path in requires[%d].endpoints[%d]", i, j)
			}
		}
	}

	return nil
}
