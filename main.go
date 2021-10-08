package main

import (
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/phillinzzz/lightEthClient/config"
	log2 "github.com/phillinzzz/lightEthClient/log"
)

func main() {
	logger := log2.GetLogger()
	p2pCfg, _ := config.GetP2PConfig()

	p2pServer := p2p.Server{Config: p2pCfg}

	err := p2pServer.Start()
	if err != nil {
		logger.Crit("Failed to start p2p server", "err", err)
		return
	}
	select {}
	//
	//// Initialize the p2p server. This creates the node key and discovery databases.
	//p2pServer.Config.PrivateKey =

}
