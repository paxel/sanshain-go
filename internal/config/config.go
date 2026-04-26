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
	ApiType         string           `yaml:"apiType,omitempty"`
	Branch          string           `yaml:"branch,omitempty"`
	OutputDirectory string           `yaml:"outputDirectory"`
	Timeout         int              `yaml:"timeout,omitempty"`
	Endpoints       []EndpointConfig `yaml:"endpoints"`
}

type ProvideConfig struct {
	File         string `yaml:"file,omitempty"`
	ApiType      string `yaml:"apiType,omitempty"`
	Branch       string `yaml:"branch,omitempty"`
	BaseVersion  *int   `yaml:"baseVersion,omitempty"`
	OpenApiFile  string `yaml:"openApiFile,omitempty"`
	AsyncApiFile string `yaml:"asyncApiFile,omitempty"`
	ProtoFile    string `yaml:"protoFile,omitempty"`
}

type SanshainConfig struct {
	SanshainURL string          `yaml:"sanshainUrl"`
	ServiceName string          `yaml:"serviceName"`
	ClientName  string          `yaml:"clientName,omitempty"` // alias
	Timeout     int             `yaml:"timeout,omitempty"`
	Compression bool            `yaml:"compression,omitempty"`
	BestEffort  bool            `yaml:"bestEffort,omitempty"`
	Strict      bool            `yaml:"strict,omitempty"`
	Provide     *ProvideConfig  `yaml:"provide,omitempty"`
	Provides    []ProvideConfig `yaml:"provides,omitempty"`
	Requires    []RequireConfig `yaml:"requires,omitempty"`
}

func LoadConfig(configPath string) (*SanshainConfig, error) {
	/* #nosec G304 */
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
	if config.ServiceName == "" && config.ClientName != "" {
		config.ServiceName = config.ClientName
	}

	validateProvide := func(p *ProvideConfig, i int) error {
		if p.File == "" && p.OpenApiFile == "" && p.AsyncApiFile == "" && p.ProtoFile == "" {
			return fmt.Errorf("at least one of file, openApiFile, asyncApiFile, or protoFile must be specified in provide[%d]", i)
		}
		return nil
	}

	if config.Provide != nil {
		if err := validateProvide(config.Provide, 0); err != nil {
			return err
		}
	}
	for i := range config.Provides {
		if err := validateProvide(&config.Provides[i], i); err != nil {
			return err
		}
	}

	for i := range config.Requires {
		req := &config.Requires[i]
		if req.ServiceName == "" {
			return fmt.Errorf("missing serviceName in requires[%d]", i)
		}
		if req.OutputDirectory == "" {
			return fmt.Errorf("missing outputDirectory in requires[%d]", i)
		}
		if len(req.Endpoints) == 0 {
			return fmt.Errorf("missing or empty endpoints in requires[%d]", i)
		}
		for j := range req.Endpoints {
			endpoint := &req.Endpoints[j]
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
