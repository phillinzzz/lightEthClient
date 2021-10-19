package client

import (
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/phillinzzz/lightEthClient/config"
	log2 "github.com/phillinzzz/lightEthClient/log"
)

type Client struct {
	name      string
	logger    log.Logger
	p2pServer p2p.Server
	ethPeers  map[string]*eth.Peer
}

func (l *Client) Init(name string) *Client {
	l.name = name

	l.logger = log2.GetLogger()

	p2pCfg, _ := config.GetP2PConfig()
	l.p2pServer = p2p.Server{Config: p2pCfg}

	l.ethPeers = make(map[string]*eth.Peer)

	protos, err := config.MakeProtocols(l.ethPeers)
	if err != nil {
		fmt.Println("生成Protocols时出错：", err)
	}
	l.p2pServer.Protocols = append(l.p2pServer.Protocols, protos...)

	return l
}

func (l *Client) Run() {
	if err := l.p2pServer.Start(); err != nil {
		l.logger.Crit("Failed to start p2p server", "err", err)
		return
	}
}

func (l *Client) BroadcastTxs(txs types.Transactions) {
	for _, ethPeer := range l.ethPeers {
		// todo：需要重新改造ethPeer "type myEthPeer eth.Peer"
		_ = ethPeer.SendTransactions(txs)
	}
}
