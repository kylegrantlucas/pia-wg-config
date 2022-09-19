package main

import (
	"log"
	"os"

	"github.com/kylegrantlucas/pia-wg-config/pia"
	cli "github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:   "pia-wg-config",
		Usage:  "generate a wireguard config for private internet access",
		Action: defaultAction,

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
				Usage:   "The private internet access region to connect to",
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
	// get username and password
	username := c.Args().Get(0)
	password := c.Args().Get(1)
	verbose := c.Bool("verbose")

	// create pia client
	if verbose {
		log.Print("Creating PIA client")
	}
	piaClient, err := pia.NewPIAClient(username, password, c.String("region"), verbose)
	if err != nil {
		return err
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
		return err
	}

	if c.String("outfile") != "" {
		// write config to file
		err = os.WriteFile(c.String("outfile"), []byte(config), 0644)
		if err != nil {
			return err
		}
	} else {
		// print config to stdout
		log.Println(config)
	}

	return nil
}
