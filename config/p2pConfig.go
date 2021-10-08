package config

import (
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/params"
	log2 "github.com/phillinzzz/lightEthClient/log"
	"runtime"
)

var p2pConfig p2p.Config

func GetP2PConfig() (p2p.Config, error) {

	p2pConfig = p2p.Config{
		Name:       fmt.Sprintf("Geth/v1.10.9-stable-eae3b194/%s-%s/%s", runtime.GOOS, runtime.GOARCH, runtime.Version()),
		ListenAddr: ":30303",
		MaxPeers:   50,
		NAT:        nat.Any(),
	}

	logger := log2.GetLogger()
	p2pConfig.Logger = logger

	// generate private key!
	key, err := crypto.GenerateKey()
	if err != nil {
		logger.Crit("Failed to generate ephemeral node key", "err", err)
		return p2p.Config{}, err
	}
	logger.Info("Private Node Key generated", "Key", key.D)
	p2pConfig.PrivateKey = key

	// add pre-configured BootstrapNodes to config
	urls := params.MainnetBootnodes
	p2pConfig.BootstrapNodes = make([]*enode.Node, 0, len(urls))
	for _, url := range urls {
		if url != "" {
			node, err := enode.Parse(enode.ValidSchemes, url)
			if err != nil {
				logger.Crit("Bootstrap URL invalid", "enode", url, "err", err)
				continue
			}
			p2pConfig.BootstrapNodes = append(p2pConfig.BootstrapNodes, node)
		}
	}

	// add pre-configured BootstrapNodesV5 to config
	urls2 := params.V5Bootnodes
	p2pConfig.BootstrapNodesV5 = make([]*enode.Node, 0, len(urls2))
	for _, url := range urls {
		if url != "" {
			node, err := enode.Parse(enode.ValidSchemes, url)
			if err != nil {
				logger.Crit("BootstrapV5 URL invalid", "enode", url, "err", err)
				continue
			}
			p2pConfig.BootstrapNodesV5 = append(p2pConfig.BootstrapNodesV5, node)
		}
	}

	return p2pConfig, nil

}
