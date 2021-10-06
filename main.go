package main

import (
	"fmt"
	"gopkg.in/urfave/cli.v1"
	"os"
)

func main() {
	var language string

	app := cli.NewApp()
	app.Name = "LightEthClient"
	app.Usage = "light weight eth client"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "lang",
			Usage:       "language for greeting",
			Value:       "english",
			Destination: &language,
		},
	}

	app.Action = func(c *cli.Context) error {
		fmt.Println("the app is running...")

		name := "antonio"
		if c.NArg() > 0 {
			name = c.Args()[0]
		}
		if c.String("lang") == "spanish" {
			fmt.Println("hola", name)
		} else {
			fmt.Println("hello", name)
		}

		return nil
	}
	app.Run(os.Args)

}
