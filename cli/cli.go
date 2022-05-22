package main

import (
	"os"

	"github.com/tarosky/gutenberg-cflog/cflog"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "cflog"

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:     "keys",
			Aliases:  []string{"k"},
			Required: true,
		},
		&cli.StringFlag{
			Name:     "common-prefix",
			Aliases:  []string{"cp"},
			Required: true,
		},
	}

	app.Action = func(c *cli.Context) error {
		log := cflog.CreateLogger([]string{"stderr"})
		defer log.Sync()

		of := cflog.ParseOutputFields(c.String("keys"))
		path := c.Args().Get(0)

		f, err := os.Open(path)
		if err != nil {
			return err
		}

		config := &cflog.Config{
			Log:          log,
			OutputFields: of,
			CommonPrefix: c.String("common-prefix"),
		}

		cflog.Scan(f, config)

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		panic("failed to run app: " + err.Error())
	}
}
