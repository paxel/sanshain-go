package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paxel/sanshain-go/v2/internal/api"
	"github.com/paxel/sanshain-go/v2/internal/cache"
	"github.com/paxel/sanshain-go/v2/internal/config"
	"github.com/paxel/sanshain-go/v2/internal/utils"
)

// version is the client's own version. sanshain-go 2.x speaks the
// Sanshain Service 2.x wire contract.
const version = "2.5.0"

func main() {
	configPath := flag.String("config", "sanshain.yaml", "path to sanshain.yaml")
	insecure := flag.Bool("insecure", false, "skip SSL certificate verification")
	bestEffort := flag.Bool("best-effort", false, "turn failed calls, and a missing or broken sanshain.yaml, into warnings and exit 0 (also: SANSHAIN_BEST_EFFORT=true or 1; bestEffort: true in sanshain.yaml counts for failed calls only)")
	ga := flag.Bool("ga", false, "provide as immutable 'ga' instead of the default 'snapshot'; --ga=false keeps a snapshot despite autoGa (also: SANSHAIN_GA=true or 1, SANSHAIN_GA=false)")
	trunk := flag.Bool("trunk", false, "this build is trunk's: it maintains the main graph (also: SANSHAIN_TRUNK=true or 1)")
	tag := flag.String("tag", "", "this build belongs to a sanshain-branch — release/hotfix pipelines (also: SANSHAIN_TAG)")
	buildURL := flag.String("build-url", "", "where this build ran (also: SANSHAIN_BUILD_URL; default: the runner's own variable)")
	changeURL := flag.String("change-url", "", "the pull or merge request behind this build (also: SANSHAIN_CHANGE_URL; default: the runner's own variable)")
	tickets := flag.String("tickets", "", "ticket ids this build closes, comma-separated (also: SANSHAIN_TICKETS)")
	note := flag.String("note", "", "one line of intent for this provide (also: SANSHAIN_NOTE)")
	sets := setFlags{}
	flag.Var(sets, "set", "fill the {{NAME}} placeholders of the annotations in sanshain.yaml, NAME=value; repeatable (also: SANSHAIN_SET_NAME)")
	showVersion := flag.Bool("version", false, "print the client version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("sanshain-go %s\n", version)
		return
	}

	if flag.NArg() < 1 {
		usage()
		os.Exit(1)
	}

	command := flag.Arg(0)

	// A missing or broken sanshain.yaml cannot be trusted to say bestEffort,
	// so only the flag and SANSHAIN_BEST_EFFORT decide for it: best effort
	// makes it a warning and exits 0 without a call. Strict is read from the
	// file only, so it never applies here; unknown fields rejected by the
	// strict of a file that otherwise loads always fail.
	outsideBestEffort := resolveBestEffort(*bestEffort, false)
	cfg, err := config.LoadConfig(*configPath, os.Stderr, outsideBestEffort)
	if err != nil {
		if errors.Is(err, config.ErrUnknownFieldsStrict) {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		if err := soften(outsideBestEffort, err); err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg.BestEffort = resolveBestEffort(*bestEffort, cfg.BestEffort)

	token := os.Getenv("SANSHAIN_TOKEN")
	client := api.NewSanshainClient(cfg.SanshainURL, token, *insecure)
	sc := cache.NewSanshainCache(".")

	stream, neither := api.ResolveStream(*trunk, *tag, os.Getenv("SANSHAIN_TRUNK"), os.Getenv("SANSHAIN_TAG"))
	if neither != "" {
		printWarning(neither)
		if cfg.Strict {
			fmt.Printf("Error: strict: %s\n", neither)
			os.Exit(1)
		}
	}

	// Before any request: an unfilled placeholder fails a strict build.
	annotations, unfilled := fillAnnotations(cfg.Annotations, sets)
	for _, w := range unfilled {
		printWarning(w)
	}
	if cfg.Strict && len(unfilled) > 0 {
		fmt.Printf("Error: strict: %s\n", unfilled[0])
		os.Exit(1)
	}

	switch command {
	case "provide":
		origin := api.ResolveOrigin(*buildURL, *changeURL, *tickets, *note, os.Getenv)
		stability := resolveStability(gaFlag(*ga), os.Getenv("SANSHAIN_GA"), cfg.AutoGa, streamDeclared(stream, neither))
		err = handleProvide(client, cfg, stability, stream, origin, annotations)
	case "require":
		err = handleRequire(client, cfg, sc, stream, annotations)
	case "catalog":
		err = handleCatalog(client, cfg, stream, annotations)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		usage()
		os.Exit(1)
	}

	if err := soften(cfg.BestEffort, err); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// soften reports a failed call, or a missing or broken sanshain.yaml, as a
// warning under best effort and returns nil, so the build goes on; a
// configError, or any error without best effort, is handed back.
func soften(bestEffort bool, err error) error {
	var cfgErr *configError
	if err == nil || !bestEffort || errors.As(err, &cfgErr) {
		return err
	}
	fmt.Printf("Warning: %v\n", err)
	return nil
}

// resolveStability decides a provide's stability; the first of these that
// is set decides:
//  1. the --ga flag, true or false (nil when not given);
//  2. SANSHAIN_GA when not blank, read like every switch, so an explicit
//     false counts;
//  3. autoGa: true on a build that declares a stream (trunk, a tag, or both);
//  4. otherwise snapshot.
//
// There is no branch detection: CI declares trunk and tag on its protected
// pipelines, so only those builds become GA.
func resolveStability(gaFlag *bool, gaEnv string, autoGa, streamDeclared bool) string {
	ga := autoGa && streamDeclared
	switch {
	case gaFlag != nil:
		ga = *gaFlag
	case strings.TrimSpace(gaEnv) != "":
		ga = utils.IsOn(gaEnv)
	}
	if ga {
		return "ga"
	}
	return "snapshot"
}

// gaFlag is the --ga flag when it was given (also as --ga=false), else nil.
func gaFlag(value bool) *bool {
	given := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "ga" {
			given = true
		}
	})
	if !given {
		return nil
	}
	return &value
}

// streamDeclared reports whether the build declared a stream: trunk, a tag,
// or both — which sends neither (the warning) but still declared one.
func streamDeclared(stream api.Stream, trunkAndTagWarning string) bool {
	return stream.Trunk || stream.Tag != "" || trunkAndTagWarning != ""
}

// setFlags collects the repeatable --set NAME=value, split at the first =;
// a later --set for the same NAME wins.
type setFlags map[string]string

func (s setFlags) String() string {
	return ""
}

func (s setFlags) Set(arg string) error {
	name, value, ok := strings.Cut(arg, "=")
	if !ok || name == "" {
		return fmt.Errorf("expected NAME=value, got %q", arg)
	}
	s[name] = value
	return nil
}

// fillAnnotations fills the placeholders of the annotations in sanshain.yaml
// from the --set values and SANSHAIN_SET_* and returns the filled entries and
// one warning per entry left out.
func fillAnnotations(declared []config.Annotation, sets setFlags) ([]api.Annotation, []string) {
	entries := make([]api.Annotation, len(declared))
	for i, a := range declared {
		entries[i] = api.Annotation(a)
	}
	return api.FillAnnotations(entries, sets, os.Getenv)
}

// resolveBestEffort applies --best-effort, then a set SANSHAIN_BEST_EFFORT,
// then the yaml bestEffort value.
func resolveBestEffort(flagValue, yamlValue bool) bool {
	if flagValue {
		return true
	}
	if env := os.Getenv("SANSHAIN_BEST_EFFORT"); env != "" {
		return utils.IsOn(env)
	}
	return yamlValue
}

func usage() {
	fmt.Println("Usage: sanshain-go [flags] <command>")
	fmt.Println("Commands:")
	fmt.Println("  provide   Upload local API spec(s) to Sanshain (version read from the spec file)")
	fmt.Println("  require   Download version-pinned specs from Sanshain")
	fmt.Println("  catalog   Push the catalogInfo file (Backstage catalog-info.yaml); needs Sanshain Service 2.4.0+")
	fmt.Println("Flags:")
	flag.PrintDefaults()
}

func handleProvide(client *api.SanshainClient, cfg *config.SanshainConfig, stability string, stream api.Stream, origin api.Origin, annotations []api.Annotation) error {
	if cfg.ServiceName == "" {
		if cfg.Strict {
			return &configError{"serviceName is required (in sanshain.yaml or via environment)"}
		}
		fmt.Println("⚠ No serviceName configured. Skipping provide. Set strict: true to fail in this case.")
		return nil
	}

	provides := cfg.Provides
	if cfg.Provide != nil {
		provides = append([]config.ProvideConfig{*cfg.Provide}, provides...)
	}

	if len(provides) == 0 {
		if cfg.Strict {
			return &configError{"no provide configuration found in sanshain.yaml"}
		}
		fmt.Println("⚠ No provide configuration found in sanshain.yaml. Skipping. Set strict: true to fail in this case.")
		return nil
	}

	provided, failed := false, false

	// Each entry stands alone under best effort: a failed call does not keep
	// the next entry from being provided.
	for _, p := range provides {
		if p.Retired {
			provided = true
			err := retireFamily(client, cfg, p.ApiType)
			failed = failed || err != nil
			if err := soften(cfg.BestEffort, err); err != nil {
				return err
			}
			continue
		}
		type fileInfo struct {
			path    string
			apiType string
		}
		var files []fileInfo
		if p.File != "" {
			files = append(files, fileInfo{p.File, p.ApiType})
		}
		// Backward compatibility
		if p.OpenApiFile != "" {
			files = append(files, fileInfo{p.OpenApiFile, "openapi"})
		}
		if p.AsyncApiFile != "" {
			files = append(files, fileInfo{p.AsyncApiFile, "asyncapi"})
		}
		if p.ProtoFile != "" {
			files = append(files, fileInfo{p.ProtoFile, "proto"})
		}

		for _, f := range files {
			provided = true
			err := provideFile(client, cfg, f.path, f.apiType, stability, stream, origin, annotations)
			failed = failed || err != nil
			if err := soften(cfg.BestEffort, err); err != nil {
				return err
			}
		}
	}

	if !provided {
		if cfg.Strict {
			return &configError{"no specification files found to provide"}
		}
		fmt.Println("⚠ No specification files found to provide. Skipping. Set strict: true to fail in this case.")
	} else if !failed {
		fmt.Println("Successfully provided spec(s).")
	}
	return nil
}

func provideFile(client *api.SanshainClient, cfg *config.SanshainConfig, filePath, apiType, stability string, stream api.Stream, origin api.Origin, annotations []api.Annotation) error {
	producerName, compression := cfg.ServiceName, cfg.Compression
	/* #nosec G304 */
	specData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read specification file %s: %w", filePath, err)
	}
	// Byte-for-byte comparison server-side: a CRLF checkout must hash the
	// same as an LF one, or the same commit conflicts with itself depending
	// on which runner published it.
	content := api.NormalizeLineEndings(string(specData))

	var resp *api.ProvideResponse

	switch apiType {
	case "", "openapi":
		payload := api.ProvidePayload{
			ProducerName: producerName,
			OpenApiYaml:  content,
			Stability:    stability,
			Trunk:        stream.Trunk,
			Tag:          stream.Tag,
			Origin:       origin,
			Annotations:  annotations,
		}
		fmt.Printf("Providing OpenAPI %s (stability: %s) to %s...\n", producerName, stability, client.BaseURL)
		resp, err = client.Provide(payload, compression)
	case "asyncapi":
		payload := api.ProvideAsyncApiPayload{
			ProducerName: producerName,
			AsyncApiYaml: content,
			Stability:    stability,
			Trunk:        stream.Trunk,
			Tag:          stream.Tag,
			Origin:       origin,
			Annotations:  annotations,
		}
		fmt.Printf("Providing AsyncAPI %s (stability: %s) to %s...\n", producerName, stability, client.BaseURL)
		resp, err = client.ProvideAsyncApi(payload, compression)
	case "proto", "grpc":
		payload := api.ProvideProtoPayload{
			ProducerName: producerName,
			ProtoContent: content,
			Stability:    stability,
			Trunk:        stream.Trunk,
			Tag:          stream.Tag,
			Origin:       origin,
			Annotations:  annotations,
		}
		fmt.Printf("Providing Proto %s (stability: %s) to %s...\n", producerName, stability, client.BaseURL)
		resp, err = client.ProvideProto(payload, compression)
	default:
		return fmt.Errorf("unsupported apiType: %s", apiType)
	}

	var conflict *api.ConflictError
	if errors.As(err, &conflict) && conflict.ProposedVersion != "" {
		versionHome := "info.version"
		if apiType == "proto" || apiType == "grpc" {
			versionHome = "the // sanshain-version comment"
		}
		err = fmt.Errorf("%s\n  Publish as %s — update %s in %s and re-run",
			conflict.Message, conflict.ProposedVersion, versionHome, filePath)
	}
	if err := checkWarnings(cfg.Strict, client.TakeWarnings(), err); err != nil {
		return err
	}

	if resp != nil {
		fmt.Printf("✓ Provided %s v%s (%s): %d new, %d updated, %d deleted endpoints\n",
			producerName, resp.Version, resp.Stability,
			resp.Changes.Inserts, resp.Changes.Updates, resp.Changes.Deletes)
		// Surface the harvest, or the feature is invisible: the server
		// records the consumer edges either way, and a subscription
		// expecting a field no contract guarantees would only be discovered
		// later, on the GA provide that refuses it. Advisories never fail
		// the build — that 409 is the server's job.
		for _, sub := range resp.HarvestedSubscriptions {
			if sub.Advisory() {
				fmt.Printf("⚠ subscription %s\n", sub.Describe())
			} else {
				fmt.Printf("  subscription %s\n", sub.Describe())
			}
		}
	}

	return nil
}

// retireFamily declares that this project no longer provides an API family,
// because its sanshain.yaml entry says retired: true.
func retireFamily(client *api.SanshainClient, cfg *config.SanshainConfig, apiType string) error {
	family := apiType
	if family == "" {
		family = "openapi"
	}
	fmt.Printf("Retiring %s for %s...\n", family, cfg.ServiceName)
	shed, err := client.Retire(cfg.ServiceName, family, false)
	if err := checkWarnings(cfg.Strict, client.TakeWarnings(), err); err != nil {
		return err
	}
	line := "✓ Retired"
	if shed.TagCleared != "" {
		line += ": cleared the '" + shed.TagCleared + "' capability"
	}
	fmt.Printf("%s, closed %d trunk pin(s), released %d contract(s). History and existing pins are untouched.\n",
		line, shed.TrunkPinsClosed, shed.ContractsReleased)
	return nil
}

// handleRequire sends the annotations along; the client keeps only the
// system ones.
func handleRequire(client *api.SanshainClient, cfg *config.SanshainConfig, sc *cache.SanshainCache, stream api.Stream, annotations []api.Annotation) error {
	if cfg.ServiceName == "" {
		if cfg.Strict {
			return &configError{"serviceName is required (in sanshain.yaml or via environment)"}
		}
		fmt.Println("⚠ No serviceName configured. Skipping require. Set strict: true to fail in this case.")
		return nil
	}

	if len(cfg.Requires) == 0 {
		if cfg.Strict {
			return &configError{"no requires configured in sanshain.yaml"}
		}
		fmt.Println("⚠ No requires configured in sanshain.yaml. Skipping. Set strict: true to fail in this case.")
		return nil
	}

	// Each entry stands alone under best effort: a failed call does not keep
	// the next entry from being downloaded.
	for _, req := range cfg.Requires {
		if err := soften(cfg.BestEffort, requireEntry(client, cfg, sc, stream, annotations, req)); err != nil {
			return err
		}
	}

	return nil
}

// requireEntry downloads one requires entry and writes it to its output
// directory.
func requireEntry(client *api.SanshainClient, cfg *config.SanshainConfig, sc *cache.SanshainCache, stream api.Stream, annotations []api.Annotation, req config.RequireConfig) error {
	fmt.Printf("Requiring %s@%s from %s...\n", req.ServiceName, req.Version, cfg.SanshainURL)
	apiType := requireApiType(req.ApiType)

	var result *api.RequireResult
	var reqErr error
	var cacheKey string
	var fileName string

	if len(req.Endpoints) == 1 {
		endpoint := req.Endpoints[0]
		fileName = req.ServiceName + "." + getExtension(apiType)
		cacheKey = cache.RequireKey(req.ServiceName, req.Version, endpoint.Method, endpoint.Path, req.Format, req.OutputDirectory)
		cachedEtag := cachedEtag(sc, cacheKey, filepath.Join(req.OutputDirectory, fileName))

		result, reqErr = client.RequireWithEtag(cfg.ServiceName, req.ServiceName, req.Version, endpoint.Path, endpoint.Method, false, apiType, cachedEtag, stream, req.Format, annotations)
	} else {
		fileName = req.ServiceName + "_bundle." + getExtension(apiType)
		cacheKey = cache.RequireBundleKey(req.ServiceName, req.Version, req.Format, req.OutputDirectory)
		cachedEtag := cachedEtag(sc, cacheKey, filepath.Join(req.OutputDirectory, fileName))

		payload := api.RequireBundlePayload{
			ConsumerName: cfg.ServiceName,
			ProducerName: req.ServiceName,
			Version:      req.Version,
			ApiType:      apiType,
			Trunk:        stream.Trunk,
			Tag:          stream.Tag,
			Format:       req.Format,
			Annotations:  annotations,
		}
		for _, e := range req.Endpoints {
			payload.Endpoints = append(payload.Endpoints, api.RequireBundleEndpoint{
				Path:   e.Path,
				Method: e.Method,
			})
		}
		result, reqErr = client.RequireBundleWithEtag(payload, cfg.Compression, cachedEtag)
	}

	if reqErr != nil {
		reqErr = fmt.Errorf("failed to require %s: %w", req.ServiceName, reqErr)
	}
	if err := checkWarnings(cfg.Strict, client.TakeWarnings(), reqErr); err != nil {
		return err
	}

	if result.NotModified {
		fmt.Printf("⏭ %s spec unchanged (304), skipping code generation.\n", req.ServiceName)
		return nil
	}

	if err := os.MkdirAll(req.OutputDirectory, 0750); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", req.OutputDirectory, err)
	}

	outputPath := filepath.Join(req.OutputDirectory, filepath.Clean(fileName))
	/* #nosec G306 G703 */
	if err := os.WriteFile(outputPath, []byte(result.Content), 0600); err != nil {
		return fmt.Errorf("failed to write output file %s: %w", outputPath, err)
	}
	fmt.Printf("Saved %s to %s\n", fileName, req.OutputDirectory)

	if result.Etag != "" {
		sc.UpdateRequireEntry(cacheKey, result.Etag)
		_ = sc.Save()
	}

	return nil
}

// cachedEtag is the ETag to send as If-None-Match: none when the output file
// is gone, so the answer is a 200 that writes it again instead of a 304 that
// would leave the directory without it.
func cachedEtag(sc *cache.SanshainCache, cacheKey, outputFile string) string {
	entry := sc.GetRequireEntry(cacheKey)
	if entry == nil {
		return ""
	}
	if _, err := os.Stat(outputFile); err != nil {
		return ""
	}
	return entry.Etag
}

// requireApiType is a require's apiType as the service's routes and bundle
// spell it: grpc, which a provide accepts too, is proto.
func requireApiType(apiType string) string {
	if apiType == "grpc" {
		return "proto"
	}
	return apiType
}

func getExtension(apiType string) string {
	if apiType == "proto" {
		return "proto"
	}
	return "yaml"
}

// configError is a strict failure: a strict configuration failure, or a
// warning strict mode turns into a failure. Best-effort softens failed calls,
// never these.
type configError struct {
	msg string
}

func (e *configError) Error() string {
	return e.msg
}

// printWarning prints one warning in this client's warning style.
func printWarning(text string) {
	fmt.Printf("⚠ %s\n", text)
}

// checkWarnings prints each warning Sanshain sent with a call, then hands
// back the call's error — unless strict mode turns the first warning into a
// failure, which beats both the error (printed first) and best-effort.
func checkWarnings(strict bool, warnings []string, err error) error {
	for _, w := range warnings {
		printWarning(w)
	}
	if !strict || len(warnings) == 0 {
		return err
	}
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	return &configError{"strict: Sanshain warned: " + warnings[0]}
}

// handleCatalog pushes the catalogInfo file so Sanshain records the system's
// Peripheral Resources. Strict here checks only serviceName and catalogInfo.
func handleCatalog(client *api.SanshainClient, cfg *config.SanshainConfig, stream api.Stream, annotations []api.Annotation) error {
	if cfg.ServiceName == "" {
		if cfg.Strict {
			return &configError{"serviceName is required (in sanshain.yaml or via environment)"}
		}
		fmt.Println("⚠ No serviceName configured. Skipping catalog. Set strict: true to fail in this case.")
		return nil
	}
	if cfg.CatalogInfo == "" {
		if cfg.Strict {
			return &configError{"strict: sanshain.yaml has no catalogInfo"}
		}
		fmt.Println("⚠ No catalogInfo in sanshain.yaml - skipping catalog.")
		return nil
	}

	/* #nosec G304 */
	content, err := os.ReadFile(cfg.CatalogInfo)
	if err != nil {
		return fmt.Errorf("failed to read catalogInfo file %s: %w", cfg.CatalogInfo, err)
	}

	fmt.Printf("Pushing catalog %s for %s to %s...\n", cfg.CatalogInfo, cfg.ServiceName, client.BaseURL)
	resp, err := client.PushCatalog(cfg.ServiceName, string(content), stream.Trunk, cfg.Compression, annotations)
	warnings := client.TakeWarnings()
	// The header names the reason; a service sending none still warns.
	if err == nil && !resp.Recorded && len(warnings) == 0 {
		warnings = append(warnings, "Sanshain did not record "+cfg.CatalogInfo)
	}
	if err := checkWarnings(cfg.Strict, warnings, err); err != nil {
		return err
	}
	if !resp.Recorded {
		return nil
	}

	graphs := "dev"
	if resp.Trunk {
		graphs = "main+dev"
	}
	fmt.Printf("Catalog recorded for %s (%s): %d resources\n", resp.System, graphs, len(resp.Resources))
	for _, r := range resp.Resources {
		fmt.Printf("  %s\n", r)
	}
	return nil
}
