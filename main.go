package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/phillinzzz/lightEthClient/config"
	log2 "github.com/phillinzzz/lightEthClient/log"
)

func main() {
	logger := log2.GetLogger()
	p2pCfg, _ := config.GetP2PConfig()

	p2pServer := p2p.Server{Config: p2pCfg}
	protos, err := config.MakeProtocols()
	if err != nil {
		fmt.Println("生成Protocols时出错：", err)
	}
	p2pServer.Protocols = append(p2pServer.Protocols, protos...)

	err = p2pServer.Start()
	if err != nil {
		logger.Crit("Failed to start p2p server", "err", err)
		return
	}
	select {}

}
