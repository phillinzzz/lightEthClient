package main

import (
	"github.com/phillinzzz/lightEthClient/client"
)

func main() {
	lightClient := client.NewClient(client.BSCChainID, client.ModeInfo, true)
	listenChan := lightClient.GetNewTxListenChan()

	for _ = range listenChan {
		//log2.MyLogger.Info("发现一笔新的transmit交易！", "hash", transmitTx.Hash())
	}
	//select {}
}
