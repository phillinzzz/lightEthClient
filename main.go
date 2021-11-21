package main

import (
	"github.com/phillinzzz/lightEthClient/client"
	"github.com/phillinzzz/lightEthClient/config"
	log2 "github.com/phillinzzz/lightEthClient/log"
)

func main() {
	lightClient := new(client.Client).Init(config.BSC, client.Debug)
	listenChan := lightClient.GetNewTxListenChan()
	broadChan := lightClient.GetBroadcastTxChan()

	go func() {
		i := 0
		for newTx := range listenChan {
			if i%100 != 0 {
				i++
				continue
			}
			i++
			log2.MyLogger.Info("外部程序员监听到一笔新的交易！", "交易hash", newTx.Hash())
			broadChan <- newTx
		}
	}()
	select {}
}
