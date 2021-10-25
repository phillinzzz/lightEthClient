package client

import (
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/dnsdisc"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
	"github.com/phillinzzz/lightEthClient/config"
	log2 "github.com/phillinzzz/lightEthClient/log"
	"sync"
	"time"
)

type Client struct {
	name             string
	logger           log.Logger
	p2pServer        p2p.Server
	ethPeers         map[string]*eth.Peer
	knownTxsPool     map[common.Hash]time.Time
	knownTxsPoolLock sync.RWMutex
}

func (l *Client) Init(name string) *Client {
	l.name = name
	l.knownTxsPool = make(map[common.Hash]time.Time)
	l.logger = log2.GetLogger()

	p2pCfg, _ := config.GetP2PConfig()
	l.p2pServer = p2p.Server{Config: p2pCfg}

	l.ethPeers = make(map[string]*eth.Peer)

	protos := l.makeProtocols()
	l.p2pServer.Protocols = protos

	return l
}

func (l *Client) Run() {
	if err := l.p2pServer.Start(); err != nil {
		l.logger.Crit("Failed to start p2p server", "err", err)
		return
	}
	go l.knownTxsPoolCleanLoop()
}

func (l *Client) knownTxsPoolCleanLoop() {
	fmt.Println("Server:开始启动交易池自动清理循环。。。")
	ticker := time.NewTicker(time.Minute * 2)
	defer ticker.Stop()
	for {
		<-ticker.C
		l.safeCleanTxsPool(time.Hour * 1)

	}
}

func (l *Client) safeCleanTxsPool(maxDuration time.Duration) {
	l.knownTxsPoolLock.Lock()
	defer l.knownTxsPoolLock.Unlock()
	fmt.Printf("Server:开始清理池子内的过期交易，当前池子内交易数量：%v \n！", len(l.knownTxsPool))
	for txHash, revTime := range l.knownTxsPool {
		if time.Since(revTime) >= maxDuration {
			delete(l.knownTxsPool, txHash)
		}
	}
	fmt.Printf("Server:池子内的过期交易清理完成，当前池子内交易数量：%v \n！", len(l.knownTxsPool))
}

func (l *Client) BroadcastTxs(txs types.Transactions) {
	for _, ethPeer := range l.ethPeers {
		// todo：需要重新改造ethPeer "type myEthPeer eth.Peer"
		_ = ethPeer.SendTransactions(txs)
	}
}

func (l *Client) makeProtocols() []p2p.Protocol {

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

	Protocols := make([]p2p.Protocol, len(protocolVersions))
	for i, version := range protocolVersions {
		version := version // Closure

		Protocols[i] = p2p.Protocol{
			Name:    protocolName,
			Version: version,
			Length:  protocolLengths[version],
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {

				fmt.Printf("Protocol(version:%v):发现一个p2p节点:%v\n", version, p.ID().String())
				//检查该节点是否为已知节点
				if _, ok := l.ethPeers[p.ID().String()]; ok {
					fmt.Printf("Protocol(version:%v):p2p节点：%v已存在！放弃！\n", version, p.ID().String())
					return errDuplicate
				}
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
				l.ethPeers[peer.ID()] = peer
				fmt.Printf("Protocol(version:%v):当前已连接peer总数：%v！\n", version, len(l.ethPeers))
				return l.handlePeer(peer, rw)
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
				if p, ok := l.ethPeers[id.String()]; ok {
					return p.Peer.Info()
				}
				return nil
			},
			DialCandidates: ethDialCandidates,
		}
	}
	return Protocols
}

// 检测该hash是否已知, true: known; false: unknown
func (l *Client) safeHasTx(txHash common.Hash) bool {
	l.knownTxsPoolLock.RLock()
	defer l.knownTxsPoolLock.RUnlock()
	_, ok := l.knownTxsPool[txHash]
	return ok
}

// 将新接收的hash加入knownTxPool
func (l *Client) safeAddTx(txHash common.Hash) {
	l.knownTxsPoolLock.Lock()
	defer l.knownTxsPoolLock.Unlock()
	l.knownTxsPool[txHash] = time.Now()
}

func (l *Client) safeCountTx() int {
	l.knownTxsPoolLock.RLock()
	defer l.knownTxsPoolLock.RUnlock()
	return len(l.knownTxsPool)
}

func (l *Client) handleNewTxs(peer *eth.Peer, txs types.Transactions) {
	txsUnknown := make([]common.Hash, 0)
	fmt.Printf("Peer:%v debug:发送来%v个tx，其中%v个为新的tx!\n", peer.ID(), txs.Len(), len(txsUnknown))
	for _, tx := range txs {
		if l.safeHasTx(tx.Hash()) {
			fmt.Printf("Peer:%v debug:发现一个重复的!\n", peer.ID())
			continue
		}
		txsUnknown = append(txsUnknown, tx.Hash())
		l.safeAddTx(tx.Hash())
	}
	fmt.Printf("Peer:%v 发送来%v个tx，其中%v个为新的tx!\n", peer.ID(), txs.Len(), len(txsUnknown))
}

func (l *Client) handleNewAnns(peer *eth.Peer, anns []common.Hash) error {
	unknownTxsHash := make([]common.Hash, len(anns))
	for _, ann := range anns {
		if l.safeHasTx(ann) {
			continue
		}
		unknownTxsHash = append(unknownTxsHash, ann)
	}
	// 向远程节点请求具体的交易信息
	err := peer.RequestTxs(unknownTxsHash)
	if err != nil {
		return err
	}
	return nil
}

func (l *Client) handlePeer(peer *eth.Peer, rw p2p.MsgReadWriter) error {
	for {
		msg, err := rw.ReadMsg()
		if err != nil {
			return err
		}
		switch msg.Code {
		// 远程节点向我们广播了一批新的交易
		case eth.TransactionsMsg:
			var txs eth.TransactionsPacket
			if err = msg.Decode(&txs); err != nil {
				return fmt.Errorf("%w: message %v: %v", errDecode, msg, err)
			}
			fmt.Printf("Peer:%v just broadcast %v transactions!\n", peer.ID(), len(txs))
			l.handleNewTxs(peer, types.Transactions(txs))
		// 远程节点发来我们刚才请求的一批交易
		case eth.PooledTransactionsMsg:
			var txs eth.PooledTransactionsPacket66
			if err = msg.Decode(&txs); err != nil {
				return fmt.Errorf("%w: message %v: %v", errDecode, msg, err)
			}
			fmt.Printf("Peer:%v just sent %v pooled transactions!\n", peer.ID(), len(txs.PooledTransactionsPacket))
			l.handleNewTxs(peer, types.Transactions(txs.PooledTransactionsPacket))
		// 远程节点宣布了一批的交易
		case eth.NewPooledTransactionHashesMsg:
			ann := new(eth.NewPooledTransactionHashesPacket)
			if err = msg.Decode(ann); err != nil {
				return fmt.Errorf("%w: message %v: %v", errDecode, msg, err)
			}
			fmt.Printf("Peer:%v just announced %v transaction hashes!\n", peer.ID(), len(*ann))
			// 向远程节点请求具体的交易信息
			err = l.handleNewAnns(peer, *ann)
			if err != nil {
				return err
			}

		case eth.NewBlockHashesMsg:
			fmt.Printf("Peer:%v just sent a new block hash!\n", peer.ID())

		case eth.GetBlockHeadersMsg:
			var query eth.GetBlockHeadersPacket66
			if err := msg.Decode(&query); err != nil {
				return fmt.Errorf("%w: message %v: %v", errDecode, msg, err)
			}
			fmt.Printf("Peer:%v just request hash for block: %v\n", peer.ID(), query.Origin)
			response := make([]*types.Header, 0)
			err = peer.ReplyBlockHeaders(query.RequestId, response)
			if err != nil {
				fmt.Printf("Peer:%v error ReplyBlockHeaders: %v\n", peer.ID(), err)
				return err
			}
		default:
			fmt.Printf("Peer:%v just sent a msg: %v!\n", peer.ID(), msg.Code)
		}
		err = msg.Discard()
		if err != nil {
			return err
		}
	}
}
