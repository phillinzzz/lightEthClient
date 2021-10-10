package config

import (
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

// 模拟的txPool
type fakeTxPool struct {
}

func (f fakeTxPool) Get(hash common.Hash) *types.Transaction {
	return nil
}

func MakeProtocols() ([]p2p.Protocol, error) {

	ethConfig := ethconfig.Defaults
	utils.SetDNSDiscoveryDefaults(&ethConfig, params.MainnetGenesisHash)

	// Setup DNS discovery iterators.
	dnsclient := dnsdisc.NewClient(dnsdisc.Config{})
	ethDialCandidates, _ := dnsclient.NewIterator(ethConfig.EthDiscoveryURLs...)

	protocolVersions := eth.ProtocolVersions
	protocolName := eth.ProtocolName
	protocolLengths := map[uint]uint64{66: 17, 65: 17}

	protocols := make([]p2p.Protocol, len(protocolVersions))
	for i, version := range protocolVersions {
		version := version // Closure

		protocols[i] = p2p.Protocol{
			Name:    protocolName,
			Version: version,
			Length:  protocolLengths[version],
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
				// create the ethPeer
				peer := eth.NewPeer(version, p, rw, fakeTxPool{})
				defer peer.Close()
				//return backend.RunPeer(peer, func(peer *Peer) error {
				//	return Handle(backend, peer)
				//})

				// Execute the Ethereum (block chain) handshake
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
				forkID := forkid.NewID(chainConfig, genesisHash, number)

				if err := peer.Handshake(1, td, hash, genesisHash, forkID, forkFilter); err != nil {
					peer.Log().Debug("Ethereum handshake failed", "err", err)
					return err
				}

				return nil
			},

			NodeInfo: func() interface{} {
				return nil
			},
			PeerInfo: func(id enode.ID) interface{} {
				return nil
			},
			DialCandidates: ethDialCandidates,
		}
	}
	return protocols, nil

}
