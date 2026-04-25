package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/paxel/sanshain/sanshain-go/internal/api"
	"github.com/paxel/sanshain/sanshain-go/internal/cache"
	"github.com/paxel/sanshain/sanshain-go/internal/config"
	"github.com/paxel/sanshain/sanshain-go/internal/git"
)

func main() {
	configPath := flag.String("config", "sanshain.yaml", "path to sanshain.yaml")
	insecure := flag.Bool("insecure", false, "skip SSL certificate verification")
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
		os.Exit(1)
	}

	command := flag.Arg(0)

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	token := os.Getenv("SANSHAIN_TOKEN")
	client := api.NewSanshainClient(cfg.SanshainURL, token, *insecure)
	sc := cache.NewSanshainCache(".")

	switch command {
	case "provide":
		err = handleProvide(client, cfg, sc)
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

func usage() {
	fmt.Println("Usage: sanshain-go [flags] <command>")
	fmt.Println("Commands:")
	fmt.Println("  provide   Upload local OpenAPI spec to Sanshain")
	fmt.Println("  require   Download required specs from Sanshain")
	fmt.Println("Flags:")
	flag.PrintDefaults()
}

func handleProvide(client *api.SanshainClient, cfg *config.SanshainConfig, sc *cache.SanshainCache) error {
	if cfg.ServiceName == "" {
		if cfg.Strict {
			return fmt.Errorf("serviceName is required (in sanshain.yaml or via environment)")
		}
		fmt.Println("\u26a0 No serviceName configured. Skipping provide. Set strict: true to fail in this case.")
		return nil
	}

	var provides []config.ProvideConfig
	if cfg.Provide != nil {
		provides = append(provides, *cfg.Provide)
	}
	provides = append(provides, cfg.Provides...)

	if len(provides) == 0 {
		if cfg.Strict {
			return fmt.Errorf("no provide configuration found in sanshain.yaml")
		}
		fmt.Println("\u26a0 No provide configuration found in sanshain.yaml. Skipping. Set strict: true to fail in this case.")
		return nil
	}

	defaultBranch := git.GetCurrentBranch()
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	provided := false

	for _, p := range provides {
		branch := p.Branch
		if branch == "" {
			branch = defaultBranch
		}

		if p.File != "" {
			err := provideFile(client, cfg.ServiceName, branch, p.File, p.ApiType, cfg.Compression, p.BaseVersion, sc)
			if err != nil {
				return err
			}
			provided = true
		}

		// Backward compatibility
		if p.OpenApiFile != "" {
			err := provideFile(client, cfg.ServiceName, branch, p.OpenApiFile, "openapi", cfg.Compression, p.BaseVersion, sc)
			if err != nil {
				return err
			}
			provided = true
		}
		if p.AsyncApiFile != "" {
			err := provideFile(client, cfg.ServiceName, branch, p.AsyncApiFile, "asyncapi", cfg.Compression, p.BaseVersion, sc)
			if err != nil {
				return err
			}
			provided = true
		}
		if p.ProtoFile != "" {
			err := provideFile(client, cfg.ServiceName, branch, p.ProtoFile, "proto", cfg.Compression, p.BaseVersion, sc)
			if err != nil {
				return err
			}
			provided = true
		}
	}

	if !provided {
		if cfg.Strict {
			return fmt.Errorf("no specification files found to provide")
		}
		fmt.Println("\u26a0 No specification files found to provide. Skipping. Set strict: true to fail in this case.")
	} else {
		fmt.Println("Successfully provided spec(s).")
	}
	return nil
}

func provideFile(client *api.SanshainClient, serviceName, branch, filePath, apiType string, compression bool, baseVersion *int, sc *cache.SanshainCache) error {
	specData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read specification file %s: %w", filePath, err)
	}
	content := string(specData)
	fileKey := filepath.Base(filePath)

	// Feature 3: Client-side content caching — skip if unchanged
	contentHash := cache.ComputeHash(content)
	cachedEntry := sc.GetProvideEntry(fileKey)
	if cachedEntry != nil && contentHash == cachedEntry.ContentHash {
		fmt.Println("\u23ed Spec unchanged (hash match), skipping provide.")
		return nil
	}

	// Feature 1: Use cached version as base_version if not explicitly set
	effectiveBaseVersion := baseVersion
	if effectiveBaseVersion == nil && cachedEntry != nil && cachedEntry.Version > 0 {
		v := cachedEntry.Version
		effectiveBaseVersion = &v
	}

	var resp *api.ProvideResponse

	if apiType == "" || apiType == "openapi" {
		payload := api.ProvidePayload{
			ServiceName: serviceName,
			Branch:      branch,
			OpenApiYaml: content,
			BaseVersion: effectiveBaseVersion,
		}
		fmt.Printf("Providing OpenAPI %s (branch: %s) to %s...\n", payload.ServiceName, payload.Branch, client.BaseURL)
		resp, err = client.Provide(payload, compression)
	} else if apiType == "asyncapi" {
		payload := api.ProvideAsyncApiPayload{
			ServiceName:  serviceName,
			Branch:       branch,
			AsyncApiYaml: content,
			BaseVersion:  effectiveBaseVersion,
		}
		fmt.Printf("Providing AsyncAPI %s (branch: %s) to %s...\n", payload.ServiceName, payload.Branch, client.BaseURL)
		resp, err = client.ProvideAsyncApi(payload, compression)
	} else if apiType == "proto" || apiType == "grpc" {
		payload := api.ProvideProtoPayload{
			ServiceName:  serviceName,
			Branch:       branch,
			ProtoContent: content,
			BaseVersion:  effectiveBaseVersion,
		}
		fmt.Printf("Providing Proto %s (branch: %s) to %s...\n", payload.ServiceName, payload.Branch, client.BaseURL)
		resp, err = client.ProvideProto(payload, compression)
	} else {
		return fmt.Errorf("unsupported apiType: %s", apiType)
	}

	if err != nil {
		return err
	}

	// Feature 2: Log summary and save state
	if resp != nil {
		fmt.Printf("\u2713 Provided to Sanshain v%d: %d new, %d updated, %d deleted endpoints\n",
			resp.Version, resp.Changes.Inserts, resp.Changes.Updates, resp.Changes.Deletes)
		responseHash := resp.ContentHash
		if responseHash == "" {
			responseHash = contentHash
		}
		sc.UpdateProvideEntry(fileKey, responseHash, resp.Version)
		_ = sc.Save()
	}

	return nil
}

func handleRequire(client *api.SanshainClient, cfg *config.SanshainConfig, sc *cache.SanshainCache) error {
	if cfg.ServiceName == "" {
		if cfg.Strict {
			return fmt.Errorf("serviceName is required (in sanshain.yaml or via environment)")
		}
		fmt.Println("\u26a0 No serviceName configured. Skipping require. Set strict: true to fail in this case.")
		return nil
	}

	if len(cfg.Requires) == 0 {
		if cfg.Strict {
			return fmt.Errorf("no requires configured in sanshain.yaml")
		}
		fmt.Println("\u26a0 No requires configured in sanshain.yaml. Skipping. Set strict: true to fail in this case.")
		return nil
	}

	currentBranch := git.GetCurrentBranch()

	for _, req := range cfg.Requires {
		reqBranch := req.Branch
		if reqBranch == "" {
			reqBranch = currentBranch
		}

		fmt.Printf("Requiring %s (branch: %s) from %s...\n", req.ServiceName, reqBranch, cfg.SanshainURL)

		var fileName string
		var err error

		if len(req.Endpoints) == 1 {
			endpoint := req.Endpoints[0]
			cacheKey := cache.RequireKey(req.ServiceName, reqBranch, endpoint.Method, endpoint.Path)
			cachedEntry := sc.GetRequireEntry(cacheKey)
			cachedEtag := ""
			if cachedEntry != nil {
				cachedEtag = cachedEntry.Etag
			}

			result, reqErr := client.RequireWithEtag(cfg.ServiceName, req.ServiceName, reqBranch, endpoint.Path, endpoint.Method, req.Timeout, false, req.ApiType, cachedEtag)
			if reqErr != nil {
				return fmt.Errorf("failed to require %s: %w", req.ServiceName, reqErr)
			}

			if result.NotModified {
				fmt.Printf("\u23ed %s spec unchanged (304), skipping code generation.\n", req.ServiceName)
				continue
			}

			ext := "yaml"
			if req.ApiType == "proto" {
				ext = "proto"
			}
			fileName = req.ServiceName + "." + ext

			err = os.MkdirAll(req.OutputDirectory, 0755)
			if err != nil {
				return fmt.Errorf("failed to create output directory %s: %w", req.OutputDirectory, err)
			}
			outputPath := filepath.Join(req.OutputDirectory, fileName)
			err = os.WriteFile(outputPath, []byte(result.Content), 0644)
			if err != nil {
				return fmt.Errorf("failed to write output file %s: %w", outputPath, err)
			}
			fmt.Printf("Saved %s to %s\n", fileName, req.OutputDirectory)

			if result.Etag != "" {
				sc.UpdateRequireEntry(cacheKey, result.Etag)
				_ = sc.Save()
			}
		} else {
			cacheKey := cache.RequireBundleKey(req.ServiceName, reqBranch)
			cachedEntry := sc.GetRequireEntry(cacheKey)
			cachedEtag := ""
			if cachedEntry != nil {
				cachedEtag = cachedEntry.Etag
			}

			payload := api.RequireBundlePayload{
				ClientName:  cfg.ServiceName,
				ServiceName: req.ServiceName,
				Branch:      reqBranch,
				Timeout:     req.Timeout,
				ApiType:     req.ApiType,
			}
			for _, e := range req.Endpoints {
				payload.Endpoints = append(payload.Endpoints, api.RequireBundleEndpoint{
					Path:   e.Path,
					Method: e.Method,
				})
			}
			result, reqErr := client.RequireBundleWithEtag(payload, cfg.Compression, cachedEtag)
			if reqErr != nil {
				return fmt.Errorf("failed to require %s: %w", req.ServiceName, reqErr)
			}

			if result.NotModified {
				fmt.Printf("\u23ed %s spec unchanged (304), skipping code generation.\n", req.ServiceName)
				continue
			}

			ext := "yaml"
			if req.ApiType == "proto" {
				ext = "proto"
			}
			fileName = req.ServiceName + "_bundle." + ext

			err = os.MkdirAll(req.OutputDirectory, 0755)
			if err != nil {
				return fmt.Errorf("failed to create output directory %s: %w", req.OutputDirectory, err)
			}
			outputPath := filepath.Join(req.OutputDirectory, fileName)
			err = os.WriteFile(outputPath, []byte(result.Content), 0644)
			if err != nil {
				return fmt.Errorf("failed to write output file %s: %w", outputPath, err)
			}
			fmt.Printf("Saved %s to %s\n", fileName, req.OutputDirectory)

			if result.Etag != "" {
				sc.UpdateRequireEntry(cacheKey, result.Etag)
				_ = sc.Save()
			}
		}
	}

	return nil
}
