package main

import (
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/kylegrantlucas/pia-wg-config/pia"
	cli "github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:   "pia-wg-config",
		Usage:  "generate a wireguard config for private internet access",
		Action: defaultAction,

		Commands: []*cli.Command{
			{
				Name:    "regions",
				Aliases: []string{"r"},
				Usage:   "List all available PIA regions",
				Action:  listRegions,
			},
		},

		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "outfile",
				Aliases: []string{"o"},
				Usage:   "The file to write the wireguard config to",
			},
			&cli.StringFlag{
				Name:    "region",
				Aliases: []string{"r"},
				Value:   "us_california",
				Usage:   "The private internet access region to connect to (use 'regions' command to list all available regions)",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Value:   false,
				Usage:   "Print verbose output",
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func defaultAction(c *cli.Context) error {
	// Validate arguments
	if c.NArg() < 2 {
		fmt.Println("Error: Username and password are required")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  pia-wg-config [OPTIONS] USERNAME PASSWORD")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  pia-wg-config myuser mypass")
		fmt.Println("  pia-wg-config -r uk_london myuser mypass")
		fmt.Println("  pia-wg-config -o config.conf -r de_frankfurt myuser mypass")
		fmt.Println()
		fmt.Println("To see available regions:")
		fmt.Println("  pia-wg-config regions")
		return cli.Exit("", 1)
	}

	// get username and password
	username := c.Args().Get(0)
	password := c.Args().Get(1)
	verbose := c.Bool("verbose")
	region := c.String("region")

	if username == "" || password == "" {
		return cli.Exit("Error: Username and password cannot be empty", 1)
	}

	// create pia client
	if verbose {
		log.Printf("Creating PIA client for region: %s", region)
	}
	piaClient, err := pia.NewPIAClient(username, password, region, verbose)
	if err != nil {
		if verbose {
			log.Printf("Failed to create PIA client: %v", err)
		}
		fmt.Printf("Error: Failed to connect to PIA servers\n")
		fmt.Printf("This could be due to:\n")
		fmt.Printf("  - Invalid username or password\n")
		fmt.Printf("  - Invalid region '%s' (run 'pia-wg-config regions' to see available regions)\n", region)
		fmt.Printf("  - Network connectivity issues\n")
		fmt.Printf("  - PIA service unavailable\n")
		fmt.Printf("\nTry running with -v flag for more details\n")
		return cli.Exit("", 1)
	}

	// create wg config generator
	if verbose {
		log.Print("creating wg config generator")
	}
	wgConfigGenerator := pia.NewPIAWgGenerator(piaClient, pia.PIAWgGeneratorConfig{Verbose: verbose})

	// generate wg config
	if verbose {
		log.Print("Generating wireguard config")
	}
	config, err := wgConfigGenerator.Generate()
	if err != nil {
		if verbose {
			log.Printf("Failed to generate config: %v", err)
		}
		fmt.Printf("Error: Failed to generate Wireguard configuration\n")
		fmt.Printf("This could be due to:\n")
		fmt.Printf("  - Authentication failure (check your PIA credentials)\n")
		fmt.Printf("  - Server communication issues\n")
		fmt.Printf("  - Region server unavailable\n")
		fmt.Printf("\nTry running with -v flag for more details\n")
		return cli.Exit("", 1)
	}

	outfile := c.String("outfile")
	if outfile != "" {
		// write config to file
		err = os.WriteFile(outfile, []byte(config), 0600) // More secure permissions
		if err != nil {
			return cli.Exit(fmt.Sprintf("Error: Failed to write config to file '%s': %v", outfile, err), 1)
		}
		if verbose {
			log.Printf("Wireguard config written to: %s", outfile)
		}
		fmt.Printf("✓ Wireguard config generated successfully: %s\n", outfile)
		fmt.Printf("You can now connect using: sudo wg-quick up %s\n", outfile)
	} else {
		// print config to stdout
		fmt.Println(config)
	}

	return nil
}

func listRegions(c *cli.Context) error {
	fmt.Println("Fetching available regions from PIA...")

	// Create a dummy client just to get the server list
	piaClient, err := pia.NewPIAClient("", "", "us_california", false)
	if err != nil {
		return fmt.Errorf("failed to fetch regions: %v", err)
	}

	regions, err := piaClient.GetAvailableRegions()
	if err != nil {
		return fmt.Errorf("failed to get regions: %v", err)
	}

	// Sort regions for consistent output
	var regionList []string
	for region := range regions {
		regionList = append(regionList, string(region))
	}
	sort.Strings(regionList)

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
