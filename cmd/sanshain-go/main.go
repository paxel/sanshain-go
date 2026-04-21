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
	if cfg.Provide == nil {
		return fmt.Errorf("no provide configuration found in sanshain.yaml")
	}

	specPath := cfg.Provide.OpenApiFile
	specData, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI file %s: %w", specPath, err)
	}

	branch := cfg.Provide.Branch
	if branch == "" {
		branch = git.GetCurrentBranch()
	}

	payload := api.ProvidePayload{
		ServiceName: cfg.Provide.ServiceName,
		Branch:      branch,
		OpenApiYaml: string(specData),
	}

	fmt.Printf("Providing %s (branch: %s) to %s...\n", payload.ServiceName, payload.Branch, cfg.SanshainURL)
	err = client.Provide(payload, cfg.Compression)
	if err != nil {
		return err
	}

	fmt.Println("Successfully provided spec.")
	return nil
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
			spec, err = client.Require(cfg.ClientName, req.ServiceName, reqBranch, endpoint.Path, endpoint.Method, req.Timeout, false)
			fileName = req.ServiceName + ".yaml"
		} else {
			payload := api.RequireBundlePayload{
				ClientName:  cfg.ClientName,
				ServiceName: req.ServiceName,
				Branch:      reqBranch,
				Timeout:     req.Timeout,
			}
			for _, e := range req.Endpoints {
				payload.Endpoints = append(payload.Endpoints, api.RequireBundleEndpoint{
					Path:   e.Path,
					Method: e.Method,
				})
			}
			spec, err = client.RequireBundle(payload, cfg.Compression)
			fileName = req.ServiceName + "_bundle.yaml"
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
