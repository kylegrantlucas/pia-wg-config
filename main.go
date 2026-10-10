package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/kylegrantlucas/pia-wg-config/pia"
)

var (
	version = "dev"
	logger  *slog.Logger
)

func main() {
	// Set up context with cancellation
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Set up structured logging with default level
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Parse command line arguments
	if err := run(ctx, os.Args[1:]); err != nil {
		logger.Error("Application error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	// Handle version flag early
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-v") {
		fmt.Printf("pia-wg-config version %s\n", version)
		return nil
	}

	// Handle help flag early
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		printUsage()
		return nil
	}

	// Handle regions command
	if len(args) > 0 && args[0] == "regions" {
		return handleRegionsCommand(ctx, args[1:])
	}

	// Handle main command (config generation)
	return handleGenerateCommand(ctx, args)
}

func handleRegionsCommand(ctx context.Context, args []string) error {
	// Parse regions command flags
	fs := flag.NewFlagSet("regions", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: pia-wg-config regions [options]\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		fs.PrintDefaults()
	}

	timeout := fs.Duration("timeout", 30*time.Second, "Timeout for PIA API requests")
	help := fs.Bool("help", false, "Show help")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if *help {
		fs.Usage()
		return nil
	}

	fmt.Println("Fetching available regions from PIA...")

	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	// Fetch regions
	regions, err := pia.GetAvailableRegions(ctx, logger)
	if err != nil {
		return fmt.Errorf("failed to fetch regions: %w", err)
	}

	// Sort regions for consistent output
	regionList := make([]string, 0, len(regions))
	for region := range regions {
		regionList = append(regionList, string(region))
	}
	slices.Sort(regionList)

	fmt.Println("\nAvailable PIA regions:")
	fmt.Println("======================")
	for _, region := range regionList {
		fmt.Printf("  %s\n", region)
	}
	fmt.Printf("\nTotal: %d regions available\n", len(regionList))
	fmt.Println("\nUsage example:")
	fmt.Println("  pia-wg-config -r uk_london USERNAME PASSWORD")

	return nil
}

func handleGenerateCommand(ctx context.Context, args []string) error {
	// Parse main command flags
	fs := flag.NewFlagSet("pia-wg-config", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: pia-wg-config [options] USERNAME PASSWORD\n")
		fmt.Fprintf(os.Stderr, "\nGenerate a Wireguard config for Private Internet Access\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  pia-wg-config myuser mypass\n")
		fmt.Fprintf(os.Stderr, "  pia-wg-config -r uk_london myuser mypass\n")
		fmt.Fprintf(os.Stderr, "  pia-wg-config -o config.conf -r de_frankfurt myuser mypass\n")
		fmt.Fprintf(os.Stderr, "\nTo see available regions:\n")
		fmt.Fprintf(os.Stderr, "  pia-wg-config regions\n")
	}

	outfile := fs.String("o", "", "Output file for the config (default: stdout)")
	region := fs.String("r", "us_california", "PIA region to connect to")
	verbose := fs.Bool("verbose", false, "Enable verbose output with structured logging")
	timeout := fs.Duration("timeout", 30*time.Second, "Timeout for PIA API requests")
	help := fs.Bool("help", false, "Show help")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if *help {
		fs.Usage()
		return nil
	}

	// Update logger if verbose
	if *verbose {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}

	// Validate arguments
	if fs.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "Error: Username and password are required\n\n")
		fs.Usage()
		return fmt.Errorf("missing required arguments")
	}

	username := fs.Arg(0)
	password := fs.Arg(1)

	if username == "" || password == "" {
		return fmt.Errorf("username and password cannot be empty")
	}

	logger.Debug("Starting config generation",
		"region", *region,
		"timeout", *timeout,
	)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	// Create PIA client
	piaClient, err := pia.NewPIAClient(ctx, username, password, *region, logger)
	if err != nil {
		return handlePIAError(err, *region)
	}

	// Generate config
	config, err := piaClient.GenerateConfig(ctx)
	if err != nil {
		return handleConfigGenerationError(err)
	}

	// Output config
	return outputConfig(config, *outfile)
}

func handlePIAError(err error, region string) error {
	logger.Error("Failed to create PIA client", "error", err, "region", region)

	var validationErr *pia.RegionValidationError
	if errors.As(err, &validationErr) {
		fmt.Fprintf(os.Stderr, "Error: Invalid region '%s'\n\n", region)
		fmt.Fprintf(os.Stderr, "Available regions can be listed with:\n")
		fmt.Fprintf(os.Stderr, "  pia-wg-config regions\n\n")
		fmt.Fprintf(os.Stderr, "Popular regions include:\n")
		fmt.Fprintf(os.Stderr, "  uk_london, de_frankfurt, us_california, japan, netherlands\n")
		return err
	}

	fmt.Fprintf(os.Stderr, "Error: Failed to connect to PIA servers\n\n")
	fmt.Fprintf(os.Stderr, "This could be due to:\n")
	fmt.Fprintf(os.Stderr, "  - Invalid username or password\n")
	fmt.Fprintf(os.Stderr, "  - Network connectivity issues\n")
	fmt.Fprintf(os.Stderr, "  - PIA service unavailable\n\n")
	fmt.Fprintf(os.Stderr, "Try running with --verbose flag for more details\n")
	fmt.Fprintf(os.Stderr, "Original error: %v\n", err)
	return err
}

func handleConfigGenerationError(err error) error {
	logger.Error("Failed to generate config", "error", err)

	var authErr *pia.AuthenticationError
	if errors.As(err, &authErr) {
		fmt.Fprintf(os.Stderr, "Error: Authentication failed\n\n")
		fmt.Fprintf(os.Stderr, "Please check:\n")
		fmt.Fprintf(os.Stderr, "  - Your PIA username and password are correct\n")
		fmt.Fprintf(os.Stderr, "  - Your PIA subscription is active\n")
		fmt.Fprintf(os.Stderr, "  - Try connecting through the official PIA app first\n")
		return err
	}

	fmt.Fprintf(os.Stderr, "Error: Failed to generate Wireguard configuration\n\n")
	fmt.Fprintf(os.Stderr, "This could be due to:\n")
	fmt.Fprintf(os.Stderr, "  - Server communication issues\n")
	fmt.Fprintf(os.Stderr, "  - Region server unavailable\n")
	fmt.Fprintf(os.Stderr, "  - Network connectivity problems\n\n")
	fmt.Fprintf(os.Stderr, "Try running with --verbose flag for more details\n")
	fmt.Fprintf(os.Stderr, "Original error: %v\n", err)
	return err
}

func outputConfig(config string, outfile string) error {
	if outfile != "" {
		// Write config to file with secure permissions
		if err := os.WriteFile(outfile, []byte(config), 0600); err != nil {
			return fmt.Errorf("failed to write config to file '%s': %w", outfile, err)
		}

		logger.Info("Wireguard config generated successfully", "file", outfile)
		fmt.Printf("✓ Wireguard config generated successfully: %s\n", outfile)
		fmt.Printf("You can now connect using: sudo wg-quick up %s\n", outfile)
		return nil
	}

	// Print config to stdout
	fmt.Println(config)
	return nil
}

func printUsage() {
	fmt.Printf("pia-wg-config version %s\n\n", version)
	fmt.Println("Generate a Wireguard config for Private Internet Access")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  pia-wg-config [options] USERNAME PASSWORD")
	fmt.Println("  pia-wg-config regions [options]")
	fmt.Println()
	fmt.Println("COMMANDS:")
	fmt.Println("  regions    List all available PIA regions")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("  -o string       Output file for the config (default: stdout)")
	fmt.Println("  -r string       PIA region to connect to (default: us_california)")
	fmt.Println("  --verbose       Enable verbose output with structured logging")
	fmt.Println("  --timeout dur   Timeout for PIA API requests (default: 30s)")
	fmt.Println("  --help, -h      Show help")
	fmt.Println("  --version, -v   Show version")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  pia-wg-config myuser mypass")
	fmt.Println("  pia-wg-config -r uk_london myuser mypass")
	fmt.Println("  pia-wg-config -o config.conf -r de_frankfurt myuser mypass")
	fmt.Println("  pia-wg-config regions")
	fmt.Println()
	fmt.Println("To see available regions:")
	fmt.Println("  pia-wg-config regions")
}
