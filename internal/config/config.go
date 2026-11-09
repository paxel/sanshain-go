package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
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
	// Format asks for the endpoints in another document format. Only
	// "openapi" with apiType asyncapi is allowed.
	Format string `yaml:"format,omitempty"`
}

type ProvideConfig struct {
	File         string `yaml:"file,omitempty"`
	ApiType      string `yaml:"apiType,omitempty"`
	OpenApiFile  string `yaml:"openApiFile,omitempty"`
	AsyncApiFile string `yaml:"asyncApiFile,omitempty"`
	ProtoFile    string `yaml:"protoFile,omitempty"`
	// The project no longer provides this family. Deleting the entry says
	// nothing — Sanshain cannot tell a dropped protocol from a pipeline that
	// merely stopped running — so keeping it and marking it retired is the
	// explicit act. The provide command turns this into a retire call.
	Retired bool `yaml:"retired,omitempty"`
}

// Annotation is one annotations entry of sanshain.yaml: a named value
// Sanshain shows with the provided version (scope version) or with the system
// (scope system). Its value may hold {{NAME}} placeholders the build fills.
type Annotation struct {
	Scope string `yaml:"scope"`
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type SanshainConfig struct {
	SanshainURL string `yaml:"sanshainUrl"`
	ServiceName string `yaml:"serviceName"`
	ClientName  string `yaml:"clientName,omitempty"` // alias
	Compression bool   `yaml:"compression,omitempty"`
	BestEffort  bool   `yaml:"bestEffort,omitempty"`
	Strict      bool   `yaml:"strict,omitempty"`
	// AutoGa makes a provide GA whenever the build declares trunk or a tag,
	// unless the --ga flag or SANSHAIN_GA says otherwise.
	AutoGa bool `yaml:"autoGa,omitempty"`
	// Annotations as written, placeholders unfilled.
	Annotations []Annotation `yaml:"annotations,omitempty"`
	// Path to the project's Backstage catalog-info.yaml, pushed by the
	// catalog command.
	CatalogInfo string          `yaml:"catalogInfo,omitempty"`
	Provide     *ProvideConfig  `yaml:"provide,omitempty"`
	Provides    []ProvideConfig `yaml:"provides,omitempty"`
	Requires    []RequireConfig `yaml:"requires,omitempty"`
}

// ErrUnknownFieldsStrict is returned when strict mode rejects unknown
// sanshain.yaml fields; under best effort only for a file that otherwise
// loads. It is a configuration error best-effort never softens.
var ErrUnknownFieldsStrict = errors.New("strict: sanshain.yaml has unknown fields")

// LoadConfig reads sanshain.yaml. As soon as it parses as YAML, legacy fields
// are rejected, then unknown fields are reported to warn (the puzzled cat) —
// failing under strict — before any other error is returned, so a typo is
// named even when it causes a later validation error.
//
// bestEffort is best effort from outside the file (flag or environment). Under
// it a file that does not validate is broken, and its own strict does not
// count: the unknown fields are ignored and the validation error is returned.
func LoadConfig(configPath string, warn io.Writer, bestEffort bool) (*SanshainConfig, error) {
	/* #nosec G304 */
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := checkRemovedFields(data); err != nil {
		return nil, err
	}

	config, err := parseAndValidate(data, &doc, filepath.Dir(configPath))

	if unknown := unknownFields(data); len(unknown) > 0 {
		strict := strictSetting(data) && (err == nil || !bestEffort)
		_, _ = fmt.Fprint(warn, PuzzledCat(unknown, strict))
		if strict {
			return nil, ErrUnknownFieldsStrict
		}
	}
	return config, err
}

// parseAndValidate checks the annotations, parses the config, applies
// SANSHAIN_URL, validates it and resolves its paths against baseDir.
func parseAndValidate(data []byte, doc *yaml.Node, baseDir string) (*SanshainConfig, error) {
	// Before the typed parse, which would turn value: 42 into "42".
	if err := checkAnnotations(doc); err != nil {
		return nil, err
	}

	var config SanshainConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// SANSHAIN_URL overrides the yaml sanshainUrl, so the URL can live in
	// the environment instead of the repository.
	if envURL := strings.TrimSpace(os.Getenv("SANSHAIN_URL")); envURL != "" {
		config.SanshainURL = envURL
	}

	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	resolvePaths(&config, baseDir)

	return &config, nil
}

// resolvePaths makes every relative path in the config relative to the
// directory holding sanshain.yaml instead of the working directory, so a build
// pointed at another config file finds the same files. Absolute paths and
// empty fields stay as they are.
func resolvePaths(config *SanshainConfig, baseDir string) {
	resolve := func(p *string) {
		if *p != "" && !filepath.IsAbs(*p) {
			*p = filepath.Join(baseDir, *p)
		}
	}
	resolveProvide := func(p *ProvideConfig) {
		resolve(&p.File)
		resolve(&p.OpenApiFile)
		resolve(&p.AsyncApiFile)
		resolve(&p.ProtoFile)
	}
	resolve(&config.CatalogInfo)
	if config.Provide != nil {
		resolveProvide(config.Provide)
	}
	for i := range config.Provides {
		resolveProvide(&config.Provides[i])
	}
	for i := range config.Requires {
		resolve(&config.Requires[i].OutputDirectory)
	}
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
	{"releaseBranches", "the branch model was removed in Sanshain 2.0; stability is 'snapshot' by default and 'ga' only via SANSHAIN_GA=true, --ga, or autoGa: true on a trunk or tag build"},
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

// apiTypes are the apiType spellings a provide and a require take; empty
// means openapi, grpc means proto.
var apiTypes = []string{"", "openapi", "asyncapi", "proto", "grpc"}

func validateConfig(config *SanshainConfig) error {
	if config.SanshainURL == "" {
		return fmt.Errorf("missing required field: sanshainUrl")
	}
	if config.ServiceName == "" && config.ClientName != "" {
		config.ServiceName = config.ClientName
	}

	validateProvide := func(p *ProvideConfig, i int) error {
		if !slices.Contains(apiTypes, p.ApiType) {
			return fmt.Errorf("provide[%d]: unsupported apiType: %s — use openapi, asyncapi or proto (grpc means proto)", i, p.ApiType)
		}
		if p.Retired {
			// A retire names its family via apiType alone; a file may stay
			// configured (the spec often still exists on disk) but is unused.
			return nil
		}
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
		if !slices.Contains(apiTypes, req.ApiType) {
			return fmt.Errorf("requires[%d] (%s): unsupported apiType: %s — use openapi, asyncapi or proto (grpc means proto)", i, req.ServiceName, req.ApiType)
		}
		if req.Format != "" && (req.Format != "openapi" || req.ApiType != "asyncapi") {
			return fmt.Errorf(
				"requires[%d] (%s): format '%s' with apiType '%s' is not supported — the only allowed combination is 'apiType: asyncapi' with 'format: openapi'",
				i, req.ServiceName, req.Format, req.ApiType)
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

// checkAnnotations checks the annotations list: every entry has the strings
// scope (version or system), name and value. A violation names its entry; the
// size limits are the service's to check, and an extra key is an unknown
// field, not an error.
func checkAnnotations(doc *yaml.Node) error {
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	root := doc.Content[0]
	var list *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "annotations" {
			list = root.Content[i+1]
		}
	}
	if list == nil || isNull(list) {
		return nil
	}
	if list.Kind != yaml.SequenceNode {
		return errors.New("annotations: must be a list of entries with scope, name and value")
	}
	for i, entry := range list.Content {
		if entry.Kind != yaml.MappingNode {
			return fmt.Errorf("annotations[%d]: must be an entry with scope, name and value", i)
		}
		fields := make(map[string]*yaml.Node)
		for j := 0; j+1 < len(entry.Content); j += 2 {
			fields[entry.Content[j].Value] = entry.Content[j+1]
		}
		context := fmt.Sprintf("annotations[%d]", i)
		if name := fields["name"]; isString(name) {
			context += " (" + name.Value + ")"
		}
		for _, field := range knownAnnotationFields {
			node := fields[field]
			if node == nil || isNull(node) {
				return fmt.Errorf("%s: missing '%s'", context, field)
			}
			if !isString(node) {
				return fmt.Errorf("%s: '%s' must be a string", context, field)
			}
		}
		if scope := fields["scope"].Value; scope != "version" && scope != "system" {
			return fmt.Errorf("%s: scope '%s' is not supported — use version or system", context, scope)
		}
	}
	return nil
}

func isNull(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.ShortTag() == "!!null"
}

func isString(node *yaml.Node) bool {
	return node != nil && node.Kind == yaml.ScalarNode && node.ShortTag() == "!!str"
}

// strictSetting reads just the yaml strict flag, before the full typed
// parse; a value that is not a boolean counts as off.
func strictSetting(data []byte) bool {
	var s struct {
		Strict bool `yaml:"strict"`
	}
	_ = yaml.Unmarshal(data, &s)
	return s.Strict
}

// knownFields per level are the union of the fields every Sanshain client
// knows: a field another client supports is not unknown here, even when this
// client ignores it.
var (
	knownTopFields        = []string{"sanshainUrl", "serviceName", "clientName", "compression", "bestEffort", "strict", "force", "insecure", "combine", "catalogInfo", "autoGa", "annotations", "provide", "provides", "requires"}
	knownProvideFields    = []string{"file", "apiType", "retired", "openApiFile", "asyncApiFile", "protoFile", "combine"}
	knownRequireFields    = []string{"serviceName", "apiType", "version", "outputDirectory", "endpoints", "format"}
	knownEndpointFields   = []string{"method", "path"}
	knownAnnotationFields = []string{"scope", "name", "value"}
)

// unknownFields returns the path of every key no client knows, in file order.
func unknownFields(data []byte) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	var found []string
	walkMapping(doc.Content[0], "", knownTopFields, &found, func(key string, value *yaml.Node, path string) {
		switch key {
		case "provide":
			walkMapping(value, path, knownProvideFields, &found, nil)
		case "provides":
			walkSequence(value, path, knownProvideFields, &found, nil)
		case "annotations":
			walkSequence(value, path, knownAnnotationFields, &found, nil)
		case "requires":
			walkSequence(value, path, knownRequireFields, &found, func(key string, value *yaml.Node, path string) {
				if key == "endpoints" {
					walkSequence(value, path, knownEndpointFields, &found, nil)
				}
			})
		}
	})
	return found
}

// walkMapping records the unknown keys of a mapping node and hands each known
// key to nested, depth first, so the result stays in file order.
func walkMapping(node *yaml.Node, prefix string, known []string, found *[]string, nested func(key string, value *yaml.Node, path string)) {
	if node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if !slices.Contains(known, key) {
			*found = append(*found, path)
			continue
		}
		if nested != nil {
			nested(key, node.Content[i+1], path)
		}
	}
}

func walkSequence(node *yaml.Node, prefix string, known []string, found *[]string, nested func(key string, value *yaml.Node, path string)) {
	if node.Kind != yaml.SequenceNode {
		return
	}
	for i, entry := range node.Content {
		walkMapping(entry, fmt.Sprintf("%s[%d]", prefix, i), known, found, nested)
	}
}

// PuzzledCat renders the unknown-field notice. Under strict the fields are
// rejected rather than ignored.
func PuzzledCat(fields []string, strict bool) string {
	quoted := make([]string, len(fields))
	for i, f := range fields {
		quoted[i] = "'" + f + "'"
	}
	noun := "field"
	if len(fields) > 1 {
		noun = "fields"
	}
	verdict := "ignored"
	if strict {
		verdict = "rejected (strict)"
	}
	return "   /\\_/\\  ?\n" +
		"  ( O_o )   sanshain.yaml: unknown " + noun + " " + strings.Join(quoted, ", ") + " - " + verdict + "\n" +
		" ---U-U---\n"
}
