package main

import "github.com/phillinzzz/lightEthClient/client"

func main() {
	lightClient := new(client.Client).Init("myClient")
	go lightClient.Run()
	select {}
}
