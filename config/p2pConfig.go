package config

import (
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/params"
	"github.com/phillinzzz/lightEthClient/config/bscConfig"
	"github.com/phillinzzz/lightEthClient/log"
	"runtime"
)

var p2pConfig p2p.Config

func GetP2PConfig(chainId ChainID) (p2p.Config, error) {

	// 列出全部因链而异的参数,默认值为eth主链参数
	var (
		listenAddr  = ":30303"
		maxPeers    = 50
		bootNodes   = params.MainnetBootnodes
		bootNodesV5 = params.V5Bootnodes
		staticNodes []string
	)

	switch chainId {
	case ETH:

	case BSC:
		listenAddr = bscConfig.BSCListenAddr
		maxPeers = bscConfig.BSCMaxPeers
		bootNodes = bscConfig.BSCBootnodes
		staticNodes = bscConfig.BSCStaticNodes
	case HECO:

	}

	p2pConfig = p2p.Config{
		Name:       fmt.Sprintf("Geth/v1.10.9-stable-eae3b194/%s-%s/%s", runtime.GOOS, runtime.GOARCH, runtime.Version()),
		ListenAddr: listenAddr,
		MaxPeers:   maxPeers,
		NAT:        nat.Any(),
	}

	// 配置p2p模块的logger，作为全局logger的子logger
	p2pLogger := log2.MyLogger.New("模块", "p2p")
	p2pConfig.Logger = p2pLogger

	// generate private key!
	key, err := crypto.GenerateKey()
	if err != nil {
		p2pLogger.Crit("Failed to generate ephemeral node key", "err", err)
		return p2p.Config{}, err
	}
	p2pLogger.Info("Private Node Key generated", "Key", key.D)
	p2pConfig.PrivateKey = key

	// add pre-configured BootstrapNodes to config
	p2pConfig.BootstrapNodes = make([]*enode.Node, 0, len(bootNodes))
	for _, url := range bootNodes {
		if url != "" {
			node, err := enode.Parse(enode.ValidSchemes, url)
			if err != nil {
				p2pLogger.Crit("Bootstrap URL invalid", "enode", url, "err", err)
				continue
			}
			p2pConfig.BootstrapNodes = append(p2pConfig.BootstrapNodes, node)
		}
	}

	// add pre-configured BootstrapNodesV5 to config
	p2pConfig.BootstrapNodesV5 = make([]*enode.Node, 0, len(bootNodesV5))
	for _, url := range bootNodesV5 {
		if url != "" {
			node, err := enode.Parse(enode.ValidSchemes, url)
			if err != nil {
				p2pLogger.Crit("BootstrapV5 URL invalid", "enode", url, "err", err)
				continue
			}
			p2pConfig.BootstrapNodesV5 = append(p2pConfig.BootstrapNodesV5, node)
		}
	}

	// add pre-configured StaticNodes to config
	p2pConfig.StaticNodes = make([]*enode.Node, 0, len(staticNodes))
	for _, url := range staticNodes {
		if url != "" {
			node, err := enode.Parse(enode.ValidSchemes, url)
			if err != nil {
				p2pLogger.Crit("StaticNode URL invalid", "enode", url, "err", err)
				continue
			}
			p2pConfig.StaticNodes = append(p2pConfig.StaticNodes, node)
		}
	}

	return p2pConfig, nil

}
