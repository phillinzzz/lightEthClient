package main

import (
	"github.com/phillinzzz/lightEthClient/client"
	"github.com/phillinzzz/lightEthClient/config"
)

func main() {
	lightClient := new(client.Client).Init(config.BSC)
	go lightClient.Run()
	select {}
}
