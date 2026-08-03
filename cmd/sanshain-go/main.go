package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/paxel/sanshain/sanshain-go/v2/internal/api"
	"github.com/paxel/sanshain/sanshain-go/v2/internal/cache"
	"github.com/paxel/sanshain/sanshain-go/v2/internal/config"
)

// version is the client's own version. sanshain-go 2.x speaks the
// Sanshain Service 2.x wire contract.
const version = "2.0.0"

func main() {
	configPath := flag.String("config", "sanshain.yaml", "path to sanshain.yaml")
	insecure := flag.Bool("insecure", false, "skip SSL certificate verification")
	bestEffort := flag.Bool("best-effort", false, "continue on errors")
	ga := flag.Bool("ga", false, "provide as immutable 'ga' instead of the default 'snapshot' (also: SANSHAIN_GA=true)")
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

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		// Even if config fails to load, check best effort from flags/env
		resolvedBestEffort := *bestEffort || os.Getenv("SANSHAIN_BEST_EFFORT") == "true"
		if resolvedBestEffort {
			fmt.Printf("Warning loading config: %v\n", err)
			cfg = &config.SanshainConfig{} // dummy config
		} else {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
	}

	// Resolve best effort
	if *bestEffort {
		cfg.BestEffort = true
	} else if env := os.Getenv("SANSHAIN_BEST_EFFORT"); env != "" {
		cfg.BestEffort = env == "true"
	}

	token := os.Getenv("SANSHAIN_TOKEN")
	client := api.NewSanshainClient(cfg.SanshainURL, token, *insecure)
	sc := cache.NewSanshainCache(".")

	switch command {
	case "provide":
		err = handleProvide(client, cfg, resolveStability(*ga))
	case "require":
		err = handleRequire(client, cfg, sc)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		usage()
		os.Exit(1)
	}

	if err != nil {
		if cfg.BestEffort {
			fmt.Printf("Warning: %v\n", err)
			return
		}
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// resolveStability implements the ga switch: every provide is a snapshot
// unless the --ga flag or SANSHAIN_GA=true explicitly says otherwise.
func resolveStability(gaFlag bool) string {
	if gaFlag || os.Getenv("SANSHAIN_GA") == "true" {
		return "ga"
	}
	return "snapshot"
}

func usage() {
	fmt.Println("Usage: sanshain-go [flags] <command>")
	fmt.Println("Commands:")
	fmt.Println("  provide   Upload local API spec(s) to Sanshain (version read from the spec file)")
	fmt.Println("  require   Download version-pinned specs from Sanshain")
	fmt.Println("Flags:")
	flag.PrintDefaults()
}

func handleProvide(client *api.SanshainClient, cfg *config.SanshainConfig, stability string) error {
	if cfg.ServiceName == "" {
		if cfg.Strict {
			return fmt.Errorf("serviceName is required (in sanshain.yaml or via environment)")
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
			return fmt.Errorf("no provide configuration found in sanshain.yaml")
		}
		fmt.Println("⚠ No provide configuration found in sanshain.yaml. Skipping. Set strict: true to fail in this case.")
		return nil
	}

	provided := false

	for _, p := range provides {
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
			if err := provideFile(client, cfg.ServiceName, f.path, f.apiType, stability, cfg.Compression); err != nil {
				return err
			}
			provided = true
		}
	}

	if !provided {
		if cfg.Strict {
			return fmt.Errorf("no specification files found to provide")
		}
		fmt.Println("⚠ No specification files found to provide. Skipping. Set strict: true to fail in this case.")
	} else {
		fmt.Println("Successfully provided spec(s).")
	}
	return nil
}

func provideFile(client *api.SanshainClient, producerName, filePath, apiType, stability string, compression bool) error {
	/* #nosec G304 */
	specData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read specification file %s: %w", filePath, err)
	}
	content := string(specData)

	var resp *api.ProvideResponse

	if apiType == "" || apiType == "openapi" {
		payload := api.ProvidePayload{
			ProducerName: producerName,
			OpenApiYaml:  content,
			Stability:    stability,
		}
		fmt.Printf("Providing OpenAPI %s (stability: %s) to %s...\n", producerName, stability, client.BaseURL)
		resp, err = client.Provide(payload, compression)
	} else if apiType == "asyncapi" {
		payload := api.ProvideAsyncApiPayload{
			ProducerName: producerName,
			AsyncApiYaml: content,
			Stability:    stability,
		}
		fmt.Printf("Providing AsyncAPI %s (stability: %s) to %s...\n", producerName, stability, client.BaseURL)
		resp, err = client.ProvideAsyncApi(payload, compression)
	} else if apiType == "proto" || apiType == "grpc" {
		payload := api.ProvideProtoPayload{
			ProducerName: producerName,
			ProtoContent: content,
			Stability:    stability,
		}
		fmt.Printf("Providing Proto %s (stability: %s) to %s...\n", producerName, stability, client.BaseURL)
		resp, err = client.ProvideProto(payload, compression)
	} else {
		return fmt.Errorf("unsupported apiType: %s", apiType)
	}

	if err != nil {
		var conflict *api.ConflictError
		if errors.As(err, &conflict) && conflict.ProposedVersion != "" {
			versionHome := "info.version"
			if apiType == "proto" || apiType == "grpc" {
				versionHome = "the // sanshain-version comment"
			}
			return fmt.Errorf("%s\n  Publish as %s — update %s in %s and re-run",
				conflict.Message, conflict.ProposedVersion, versionHome, filePath)
		}
		return err
	}

	if resp != nil {
		fmt.Printf("✓ Provided %s v%s (%s): %d new, %d updated, %d deleted endpoints\n",
			producerName, resp.Version, resp.Stability,
			resp.Changes.Inserts, resp.Changes.Updates, resp.Changes.Deletes)
	}

	return nil
}

func handleRequire(client *api.SanshainClient, cfg *config.SanshainConfig, sc *cache.SanshainCache) error {
	if cfg.ServiceName == "" {
		if cfg.Strict {
			return fmt.Errorf("serviceName is required (in sanshain.yaml or via environment)")
		}
		fmt.Println("⚠ No serviceName configured. Skipping require. Set strict: true to fail in this case.")
		return nil
	}

	if len(cfg.Requires) == 0 {
		if cfg.Strict {
			return fmt.Errorf("no requires configured in sanshain.yaml")
		}
		fmt.Println("⚠ No requires configured in sanshain.yaml. Skipping. Set strict: true to fail in this case.")
		return nil
	}

	for _, req := range cfg.Requires {
		fmt.Printf("Requiring %s@%s from %s...\n", req.ServiceName, req.Version, cfg.SanshainURL)

		var result *api.RequireResult
		var cacheKey string
		var fileName string

		if len(req.Endpoints) == 1 {
			endpoint := req.Endpoints[0]
			cacheKey = cache.RequireKey(req.ServiceName, req.Version, endpoint.Method, endpoint.Path)
			cachedEntry := sc.GetRequireEntry(cacheKey)
			cachedEtag := ""
			if cachedEntry != nil {
				cachedEtag = cachedEntry.Etag
			}

			var reqErr error
			result, reqErr = client.RequireWithEtag(cfg.ServiceName, req.ServiceName, req.Version, endpoint.Path, endpoint.Method, false, req.ApiType, cachedEtag)
			if reqErr != nil {
				return fmt.Errorf("failed to require %s: %w", req.ServiceName, reqErr)
			}

			ext := getExtension(req.ApiType)
			fileName = req.ServiceName + "." + ext
		} else {
			cacheKey = cache.RequireBundleKey(req.ServiceName, req.Version)
			cachedEntry := sc.GetRequireEntry(cacheKey)
			cachedEtag := ""
			if cachedEntry != nil {
				cachedEtag = cachedEntry.Etag
			}

			payload := api.RequireBundlePayload{
				ConsumerName: cfg.ServiceName,
				ProducerName: req.ServiceName,
				Version:      req.Version,
				ApiType:      req.ApiType,
			}
			for _, e := range req.Endpoints {
				payload.Endpoints = append(payload.Endpoints, api.RequireBundleEndpoint{
					Path:   e.Path,
					Method: e.Method,
				})
			}
			var reqErr error
			result, reqErr = client.RequireBundleWithEtag(payload, cfg.Compression, cachedEtag)
			if reqErr != nil {
				return fmt.Errorf("failed to require %s: %w", req.ServiceName, reqErr)
			}

			ext := getExtension(req.ApiType)
			fileName = req.ServiceName + "_bundle." + ext
		}

		if result.NotModified {
			fmt.Printf("⏭ %s spec unchanged (304), skipping code generation.\n", req.ServiceName)
			continue
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
	}

	return nil
}

func getExtension(apiType string) string {
	if apiType == "proto" {
		return "proto"
	}
	return "yaml"
}
