package config

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/dnsdisc"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
)

var ethPeers = make(map[string]*eth.Peer)

// 模拟的txPool
type fakeTxPool struct {
}

func (f fakeTxPool) Get(hash common.Hash) *types.Transaction {
	return nil
}

var errDecode error = errors.New("invalid message")

func HandlePeer(peer *eth.Peer, rw p2p.MsgReadWriter) error {
	for {
		msg, err := rw.ReadMsg()
		if err != nil {
			return err
		}
		switch msg.Code {
		case eth.NewBlockHashesMsg:
			fmt.Printf("Peer:%v just sent a new block hash!\n", peer.ID())
		case eth.TransactionsMsg:
			var txs eth.TransactionsPacket
			if err = msg.Decode(&txs); err != nil {
				return fmt.Errorf("%w: message %v: %v", errDecode, msg, err)
			}
			fmt.Printf("Peer:%v just send %v transactions!\n", peer.ID(), len(txs))
		default:
			fmt.Printf("Peer:%v just sent a msg: %v!\n", peer.ID(), msg.Code)
		}
		err = msg.Discard()
		if err != nil {
			return err
		}
	}
}

func MakeProtocols() ([]p2p.Protocol, error) {

	genesisHash := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	genesis := core.DefaultGenesisBlock()
	genesisBlock := genesis.ToBlock(nil)

	var (
		head        = genesisBlock.Header()
		hash        = head.Hash()
		number      = head.Number.Uint64()
		td          = genesisBlock.Difficulty()
		chainConfig = genesis.Config
		forkFilter  = forkid.NewStaticFilter(chainConfig, genesisHash)
	)

	ethConfig := ethconfig.Defaults
	utils.SetDNSDiscoveryDefaults(&ethConfig, params.MainnetGenesisHash)

	// Setup DNS discovery iterators.
	dnsclient := dnsdisc.NewClient(dnsdisc.Config{})
	ethDialCandidates, _ := dnsclient.NewIterator(ethConfig.EthDiscoveryURLs...)

	protocolVersions := eth.ProtocolVersions
	protocolName := eth.ProtocolName
	protocolLengths := map[uint]uint64{eth.ETH66: 17, 65: 17}

	protocols := make([]p2p.Protocol, len(protocolVersions))
	for i, version := range protocolVersions {
		version := version // Closure

		protocols[i] = p2p.Protocol{
			Name:    protocolName,
			Version: version,
			Length:  protocolLengths[version],
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
				fmt.Printf("Protocol(version:%v):发现一个p2p节点:%v\n", version, p)
				// create the ethPeer
				peer := eth.NewPeer(version, p, rw, fakeTxPool{})
				defer peer.Close()

				// Execute the Ethereum (block chain) handshake
				fmt.Printf("Protocol(version:%v):准备与p2p节点:%v进行握手！\n", version, p)

				forkID := forkid.NewID(chainConfig, genesisHash, number)
				if err := peer.Handshake(1, td, hash, genesisHash, forkID, forkFilter); err != nil {
					peer.Log().Debug("Ethereum handshake failed", "err", err)
					fmt.Printf("Protocol(version:%v):与p2p节点:%v进行握手失败！原因: %v\n", version, p, err)
					return err
				}
				fmt.Printf("Protocol(version:%v):与p2p节点:%v进行握手成功！\n", version, p)
				// register the peer
				ethPeers[peer.ID()] = peer
				fmt.Printf("Protocol(version:%v):当前已连接peer总数：%v！\n", version, len(ethPeers))
				return HandlePeer(peer, rw)
			},

			NodeInfo: func() interface{} {
				return &eth.NodeInfo{
					Network:    1,
					Difficulty: td,
					Genesis:    genesisHash,
					Config:     chainConfig,
					Head:       head.Hash(),
				}
			},
			PeerInfo: func(id enode.ID) interface{} {
				if p, ok := ethPeers[id.String()]; ok {
					return p.Peer.Info()
				}
				return nil
			},
			DialCandidates: ethDialCandidates,
		}
	}
	return protocols, nil

}
