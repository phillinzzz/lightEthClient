package main

import (
	"github.com/phillinzzz/lightEthClient/client"
	"github.com/phillinzzz/lightEthClient/config"
)

func main() {
	lightClient := new(client.Client).Init(config.BSC)
	go lightClient.Run()
	select {}
	//genesis := bscConfig.MakeBSCGenesis()
	//genesisBlock := genesis.ToBlock(nil)
	//forkID := forkid.NewID(genesis.Config, genesisBlock.Hash(), genesisBlock.Header().Number.Uint64())
	//forkID := forkid.ID{
	//	Hash: [4]byte{252, 60, 166, 183},
	//	Next: 0,
	//}

	//fmt.Printf("%+v", forkID)
}
