package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/paxel/sanshain/sanshain-go/internal/api"
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

	switch command {
	case "provide":
		err = handleProvide(client, cfg)
	case "require":
		err = handleRequire(client, cfg)
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

func handleProvide(client *api.SanshainClient, cfg *config.SanshainConfig) error {
	var provides []config.ProvideConfig
	if cfg.Provide != nil {
		provides = append(provides, *cfg.Provide)
	}
	provides = append(provides, cfg.Provides...)

	if len(provides) == 0 {
		return fmt.Errorf("no provide configuration found in sanshain.yaml")
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
			err := provideFile(client, cfg.ServiceName, branch, p.File, p.ApiType, cfg.Compression)
			if err != nil {
				return err
			}
			provided = true
		}

		// Backward compatibility
		if p.OpenApiFile != "" {
			err := provideFile(client, cfg.ServiceName, branch, p.OpenApiFile, "openapi", cfg.Compression)
			if err != nil {
				return err
			}
			provided = true
		}
		if p.AsyncApiFile != "" {
			err := provideFile(client, cfg.ServiceName, branch, p.AsyncApiFile, "asyncapi", cfg.Compression)
			if err != nil {
				return err
			}
			provided = true
		}
		if p.ProtoFile != "" {
			err := provideFile(client, cfg.ServiceName, branch, p.ProtoFile, "proto", cfg.Compression)
			if err != nil {
				return err
			}
			provided = true
		}
	}

	if !provided {
		fmt.Println("No specification files found to provide.")
	} else {
		fmt.Println("Successfully provided spec(s).")
	}
	return nil
}

func provideFile(client *api.SanshainClient, serviceName, branch, filePath, apiType string, compression bool) error {
	specData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read specification file %s: %w", filePath, err)
	}

	if apiType == "" || apiType == "openapi" {
		payload := api.ProvidePayload{
			ServiceName: serviceName,
			Branch:      branch,
			OpenApiYaml: string(specData),
		}
		fmt.Printf("Providing OpenAPI %s (branch: %s) to %s...\n", payload.ServiceName, payload.Branch, client.BaseURL)
		return client.Provide(payload, compression)
	} else if apiType == "asyncapi" {
		payload := api.ProvideAsyncApiPayload{
			ServiceName:  serviceName,
			Branch:       branch,
			AsyncApiYaml: string(specData),
		}
		fmt.Printf("Providing AsyncAPI %s (branch: %s) to %s...\n", payload.ServiceName, payload.Branch, client.BaseURL)
		return client.ProvideAsyncApi(payload, compression)
	} else if apiType == "proto" || apiType == "grpc" {
		payload := api.ProvideProtoPayload{
			ServiceName:  serviceName,
			Branch:       branch,
			ProtoContent: string(specData),
		}
		fmt.Printf("Providing Proto %s (branch: %s) to %s...\n", payload.ServiceName, payload.Branch, client.BaseURL)
		return client.ProvideProto(payload, compression)
	}

	return fmt.Errorf("unsupported apiType: %s", apiType)
}

func handleRequire(client *api.SanshainClient, cfg *config.SanshainConfig) error {
	if len(cfg.Requires) == 0 {
		fmt.Println("No requirements defined in sanshain.yaml")
		return nil
	}

	currentBranch := git.GetCurrentBranch()

	for _, req := range cfg.Requires {
		reqBranch := req.Branch
		if reqBranch == "" {
			reqBranch = currentBranch
		}

		fmt.Printf("Requiring %s (branch: %s) from %s...\n", req.ServiceName, reqBranch, cfg.SanshainURL)

		var spec string
		var fileName string
		var err error

		if len(req.Endpoints) == 1 {
			endpoint := req.Endpoints[0]
			spec, err = client.Require(cfg.ServiceName, req.ServiceName, reqBranch, endpoint.Path, endpoint.Method, req.Timeout, false, req.ApiType)
			ext := "yaml"
			if req.ApiType == "proto" {
				ext = "proto"
			}
			fileName = req.ServiceName + "." + ext
		} else {
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
			spec, err = client.RequireBundle(payload, cfg.Compression)
			ext := "yaml"
			if req.ApiType == "proto" {
				ext = "proto"
			}
			fileName = req.ServiceName + "_bundle." + ext
		}

		if err != nil {
			return fmt.Errorf("failed to require %s: %w", req.ServiceName, err)
		}

		err = os.MkdirAll(req.OutputDirectory, 0755)
		if err != nil {
			return fmt.Errorf("failed to create output directory %s: %w", req.OutputDirectory, err)
		}

		outputPath := filepath.Join(req.OutputDirectory, fileName)
		err = os.WriteFile(outputPath, []byte(spec), 0644)
		if err != nil {
			return fmt.Errorf("failed to write output file %s: %w", outputPath, err)
		}

		fmt.Printf("Saved %s to %s\n", fileName, req.OutputDirectory)
	}

	return nil
}
