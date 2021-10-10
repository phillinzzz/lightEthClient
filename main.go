package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	//logger := log2.GetLogger()
	//p2pCfg, _ := config.GetP2PConfig()
	//
	//p2pServer := p2p.Server{Config: p2pCfg}

	//err := p2pServer.Start()
	//if err != nil {
	//	logger.Crit("Failed to start p2p server", "err", err)
	//	return
	//}

	genesisHash := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	fmt.Println(genesisHash.Hex(), genesisHash.String())

	//
	//// Initialize the p2p server. This creates the node key and discovery databases.
	//p2pServer.Config.PrivateKey =

}
