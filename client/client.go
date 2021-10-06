package client

import (
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/params"
	"os"
	"runtime"
)

func main() {
	// customize the logger
	logger := log.New()
	handler := log.StreamHandler(os.Stdout, log.LogfmtFormat())
	log.Root().SetHandler(handler)

	// generate private key!
	key, err := crypto.GenerateKey()
	if err != nil {
		logger.Crit("Failed to generate ephemeral node key", "err", err)
		return
	}
	logger.Info("Private Node Key generated", "Key", key.D)

	//etgConfig := ethconfig.Defaults
	// Load defaults.
	p2pConfig := p2p.Config{
		Name:       fmt.Sprintf("Geth/v1.10.9-stable-eae3b194/%s-%s/%s", runtime.GOOS, runtime.GOARCH, runtime.Version()),
		PrivateKey: key,
		ListenAddr: ":30303",
		MaxPeers:   50,
		NAT:        nat.Any(),
		Logger:     logger,
	}

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
	p2pConfig.BootstrapNodes = make([]*enode.Node, 0, len(urls2))
	for _, url := range urls {
		if url != "" {
			node, err := enode.Parse(enode.ValidSchemes, url)
			if err != nil {
				logger.Crit("Bootstrap URL invalid", "enode", url, "err", err)
				continue
			}
			p2pConfig.BootstrapNodesV5 = append(p2pConfig.BootstrapNodesV5, node)
		}
	}

	p2pServer := p2p.Server{Config: p2pConfig}

	// make ETH protocols
	//protos := eth.MakeProtocols()
	_ = ethconfig.Defaults

	err = p2pServer.Start()
	if err != nil {
		logger.Crit("Failed to start p2p server", "err", err)
		return
	}
	select {}
	//
	//// Initialize the p2p server. This creates the node key and discovery databases.
	//p2pServer.Config.PrivateKey =

}
