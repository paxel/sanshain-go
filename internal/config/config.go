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
	Version         string           `yaml:"version"`
	OutputDirectory string           `yaml:"outputDirectory"`
	Endpoints       []EndpointConfig `yaml:"endpoints"`
}

type ProvideConfig struct {
	File         string `yaml:"file,omitempty"`
	ApiType      string `yaml:"apiType,omitempty"`
	OpenApiFile  string `yaml:"openApiFile,omitempty"`
	AsyncApiFile string `yaml:"asyncApiFile,omitempty"`
	ProtoFile    string `yaml:"protoFile,omitempty"`
}

type SanshainConfig struct {
	SanshainURL string          `yaml:"sanshainUrl"`
	ServiceName string          `yaml:"serviceName"`
	ClientName  string          `yaml:"clientName,omitempty"` // alias
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

	if err := checkRemovedFields(data); err != nil {
		return nil, err
	}

	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// removedFields are the 1.x branch-era config fields. Each is a hard
// parse-time error by name, with a migration hint.
var removedFields = []struct {
	name string
	hint string
}{
	{"branch", "the branch model was removed in Sanshain 2.0; replace with an exact 'version' pin on each requires entry — provides read the version from the spec file (info.version / // sanshain-version:)"},
	{"timeout", "Sanshain 2.0 resolves pinned versions immediately and never waits; remove it"},
	{"baseVersion", "optimistic concurrency was removed in Sanshain 2.0 — GA versions are immutable, snapshots are last-writer-wins; remove it"},
	{"releaseBranches", "the branch model was removed in Sanshain 2.0; stability is 'snapshot' by default and 'ga' only via SANSHAIN_GA=true or --ga"},
}

func checkRemovedFields(data []byte) error {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		// A YAML parse error is already reported by the typed unmarshal.
		return nil
	}

	if err := checkSectionFields("", raw); err != nil {
		return err
	}
	if provide, ok := raw["provide"].(map[string]any); ok {
		if err := checkSectionFields("provide", provide); err != nil {
			return err
		}
	}
	for _, section := range []string{"provides", "requires"} {
		entries, ok := raw[section].([]any)
		if !ok {
			continue
		}
		for i, entry := range entries {
			m, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			if err := checkSectionFields(fmt.Sprintf("%s[%d]", section, i), m); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkSectionFields(section string, m map[string]any) error {
	for _, field := range removedFields {
		if _, present := m[field.name]; present {
			prefix := ""
			if section != "" {
				prefix = section + ": "
			}
			return fmt.Errorf("%s'%s' is no longer supported — %s", prefix, field.name, field.hint)
		}
	}
	return nil
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
		if req.Version == "" {
			return fmt.Errorf(
				"requires[%d] (%s): missing 'version' — Sanshain 2.0 pins exact versions; add version: 1.2.0 (list available: GET /producers/%s/versions)",
				i, req.ServiceName, req.ServiceName)
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
