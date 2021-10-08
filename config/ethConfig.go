package config

import (
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/dnsdisc"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
)

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
				peer := eth.NewPeer(version, p, rw, nil)
				defer peer.Close()
				//return backend.RunPeer(peer, func(peer *Peer) error {
				//	return Handle(backend, peer)
				//})

				// Execute the Ethereum handshake
				var (
					genesis = h.chain.Genesis()
					head    = h.chain.CurrentHeader()
					hash    = head.Hash()
					number  = head.Number.Uint64()
					td      = h.chain.GetTd(hash, number)
				)
				forkID := forkid.NewID(h.chain.Config(), h.chain.Genesis().Hash(), h.chain.CurrentHeader().Number.Uint64())
				if err := peer.Handshake(h.networkID, td, hash, genesis.Hash(), forkID, h.forkFilter); err != nil {
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
