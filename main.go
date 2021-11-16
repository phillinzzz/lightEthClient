package main

import (
	"encoding/binary"
	"fmt"
	"github.com/phillinzzz/lightEthClient/config/bscConfig"
	"github.com/phillinzzz/newBsc/core/forkid"
	"hash/crc32"
)

func checksumToBytes(hash uint32) [4]byte {
	var blob [4]byte
	binary.BigEndian.PutUint32(blob[:], hash)
	return blob
}

func main() {
	//lightClient := new(client.Client).Init(config.BSC)
	//go lightClient.Run()
	//select {}
	genesis := bscConfig.MakeBSCGenesis()
	genesisBlock := genesis.ToBlock(nil)
	genesisHash := genesisBlock.Hash()
	hash := crc32.ChecksumIEEE(genesisHash.Bytes())
	checkSumHash := checksumToBytes(hash)
	fmt.Println(genesisHash)
	fmt.Println(checkSumHash)

	//forkID := forkid.NewID(genesis.Config, genesisBlock.Hash(), genesisBlock.Header().Number.Uint64())
	forkID := forkid.NewID(genesis.Config, genesisBlock.Hash(), 1000)
	//forkID := forkid.ID{
	//	Hash: [4]byte{252, 60, 166, 183},
	//	Next: 0,
	//}

	fmt.Printf("%+v", forkID)
}
