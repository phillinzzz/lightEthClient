package client

import (
	"fmt"
	"github.com/phillinzzz/lightEthClient/config"
	"github.com/phillinzzz/lightEthClient/config/bscConfig"
	"github.com/phillinzzz/lightEthClient/log"
	"github.com/phillinzzz/newBsc/cmd/utils"
	"github.com/phillinzzz/newBsc/common"
	"github.com/phillinzzz/newBsc/core"
	"github.com/phillinzzz/newBsc/core/forkid"
	"github.com/phillinzzz/newBsc/core/types"
	"github.com/phillinzzz/newBsc/eth/ethconfig"
	"github.com/phillinzzz/newBsc/eth/protocols/eth"
	"github.com/phillinzzz/newBsc/log"
	"github.com/phillinzzz/newBsc/p2p"
	"github.com/phillinzzz/newBsc/p2p/dnsdisc"
	"github.com/phillinzzz/newBsc/p2p/enode"
	"sync"
	"time"
)

type Client struct {
	chainId          config.ChainID
	logger           log.Logger
	p2pServer        p2p.Server
	ethPeers         map[string]*eth.Peer
	ethPeersCheck    map[string]time.Time
	ethPeersLock     sync.RWMutex
	knownTxsPool     map[common.Hash]time.Time
	knownTxsPoolLock sync.RWMutex
}

func (l *Client) Init(chainId config.ChainID) *Client {
	l.chainId = chainId
	l.knownTxsPool = make(map[common.Hash]time.Time)
	l.logger = log2.MyLogger.New("模块", "ETH")

	p2pCfg, _ := config.GetP2PConfig(chainId)
	l.p2pServer = p2p.Server{Config: p2pCfg}

	l.ethPeers = make(map[string]*eth.Peer)
	l.ethPeersCheck = make(map[string]time.Time)

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
	go l.ethPeerCleanLoop(time.Minute*2, time.Minute*2)

}

func (l *Client) knownTxsPoolCleanLoop() {
	l.logger.Info("开始启动交易池自动清理循环")
	ticker := time.NewTicker(time.Minute * 2)
	defer ticker.Stop()
	for {
		<-ticker.C
		l.safeCleanTxsPool(time.Minute * 5)

	}
}

// 定期清理掉没有反应的远程节点
func (l *Client) ethPeerCleanLoop(maxTimeNoComm, loopTime time.Duration) {
	ticker := time.NewTicker(loopTime)
	for range ticker.C {
		l.ethPeersLock.RLock()
		l.logger.Info("节点情况汇报", "节点总数", len(l.ethPeers))
		for peerName, t := range l.ethPeersCheck {
			l.logger.Info("节点情况汇报：", "节点ID", peerName[:10], "最近交流时间", time.Since(t))

			//移除长时间没有通信的远程节点
			if time.Since(t) > maxTimeNoComm {
				l.logger.Info("移除长时间没有通信的节点", "节点ID", peerName[:10])
				l.ethPeers[peerName].Disconnect(p2p.DiscUselessPeer)
				delete(l.ethPeers, peerName)
			}
		}

		l.ethPeersLock.RUnlock()
	}

}

func (l *Client) safeUpdateEthPeerStatus(peer *eth.Peer) {
	l.ethPeersLock.Lock()
	defer l.ethPeersLock.Unlock()
	l.ethPeersCheck[peer.ID()] = time.Now()
}

func (l *Client) safeCheckPeerDuplicate(peer *p2p.Peer) error {
	l.ethPeersLock.RLock()
	defer l.ethPeersLock.RUnlock()
	if _, ok := l.ethPeers[peer.ID().String()]; ok {
		l.p2pServer.Logger.Info("p2p节点已存在，放弃之", "节点ID", peer.ID().String())
		return errDuplicate
	}
	return nil
}

func (l *Client) safeRegisterEthPeer(ethPeer *eth.Peer) {
	l.ethPeersLock.Lock()
	defer l.ethPeersLock.Unlock()
	l.ethPeers[ethPeer.ID()] = ethPeer
	l.ethPeersCheck[ethPeer.ID()] = time.Now()
	l.logger.Info("新ETH节点注册成功，当前已连接ETH Peer总数", "数量", len(l.ethPeers))
}

func (l *Client) safeUnregisterEthPeer(ethPeer *eth.Peer) {
	l.ethPeersLock.Lock()
	defer l.ethPeersLock.Unlock()
	delete(l.ethPeers, ethPeer.ID())
	delete(l.ethPeersCheck, ethPeer.ID())
	l.logger.Info("ETH节点移除完成，当前已连接ETH Peer总数", "数量", len(l.ethPeers))
}

func (l *Client) safeCleanTxsPool(maxDuration time.Duration) {
	l.knownTxsPoolLock.Lock()
	defer l.knownTxsPoolLock.Unlock()
	l.logger.Info("开始清理池子内的过期交易", "池子内交易数量", len(l.knownTxsPool))
	for txHash, revTime := range l.knownTxsPool {
		if time.Since(revTime) >= maxDuration {
			delete(l.knownTxsPool, txHash)
		}
	}
	l.logger.Info("池子内的过期交易清理完成", "池子内交易数量", len(l.knownTxsPool))
}

func (l *Client) BroadcastTxs(txs types.Transactions) {
	for _, ethPeer := range l.ethPeers {
		// todo：需要重新改造ethPeer "type myEthPeer eth.Peer"
		_ = ethPeer.SendTransactions(txs)
	}
}

func (l *Client) makeProtocols() []p2p.Protocol {

	// 创世区块
	var genesis *core.Genesis

	// 生成创世区块
	switch l.chainId {
	case config.ETH:
		genesis = core.DefaultGenesisBlock()
	case config.BSC:
		genesis = bscConfig.MakeBSCGenesis()
	default:
		l.logger.Crit("未配置网络参数！", "ChainID", l.chainId)
	}

	//genesisHash := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	genesisBlock := genesis.ToBlock(nil)

	// 握手需要的信息
	var (
		head        = genesisBlock.Header()
		hash        = head.Hash()
		number      = head.Number.Uint64()
		td          = genesisBlock.Difficulty()
		chainConfig = genesis.Config
		forkFilter  = forkid.NewStaticFilter(chainConfig, genesisBlock.Hash())
	)

	ethConfig := ethconfig.Defaults
	//eth主网有DNS节点列表功能，bsc网络没有此功能
	if l.chainId == config.ETH {
		utils.SetDNSDiscoveryDefaults(&ethConfig, genesisBlock.Hash())
	}

	// Setup DNS discovery iterators. 只对ETH主网起效果。
	dnsclient := dnsdisc.NewClient(dnsdisc.Config{})
	ethDialCandidates, _ := dnsclient.NewIterator(ethConfig.EthDiscoveryURLs...)

	//protocolVersions := eth.ProtocolVersions
	protocolVersions := []uint{67, 66, 65}
	protocolName := "eth"
	protocolLengths := map[uint]uint64{67: 18, 66: 17, 65: 17}

	Protocols := make([]p2p.Protocol, len(protocolVersions))
	for i, version := range protocolVersions {
		version := version // Closure

		Protocols[i] = p2p.Protocol{
			Name:    protocolName,
			Version: version,
			Length:  protocolLengths[version],
			// Run函数用来初始化p2p节点并将其升级为ethPeer。Run函数执行后，就有了ethPeer。当Run函数返回以后，ethPeer也就已经关闭了
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {

				l.logger.Info("发现一个p2p节点!", "protocol", version, "节点ID", p.ID().String()[:10])
				//检查该节点是否为已知节点
				if err := l.safeCheckPeerDuplicate(p); err != nil {
					l.logger.Info("p2p节点已存在！", "节点ID", p.ID().String()[:10])
					return err
				}
				// create the ethPeer
				peer := eth.NewPeer(version, p, rw, fakeTxPool{})
				defer peer.Close()
				// Execute the Ethereum (block chain) handshake
				//l.logger.Info("准备与p2p节点进行握手！", "protocol", version, "节点ID", p.ID().String()[:10])
				forkID := forkid.NewID(chainConfig, genesisBlock.Hash(), number)
				if err := peer.Handshake(uint64(l.chainId), td, hash, genesisBlock.Hash(), forkID, forkFilter, &eth.UpgradeStatusExtension{DisablePeerTxBroadcast: false}); err != nil {
					l.logger.Info("与p2p节点握手失败", "protocol", version, "节点ID", p.ID().String()[:10], "原因", err)
					return err
				}
				l.logger.Info("与p2p节点握手成功", "protocol", version, "节点ID", p.ID().String()[:10])
				// register the peer
				l.safeRegisterEthPeer(peer)
				defer l.safeUnregisterEthPeer(peer)
				defer l.logger.Warn("EthPeer准备关闭!", "节点ID", peer.ID()[:10])

				return l.handlePeer(peer, rw)
			},

			NodeInfo: func() interface{} {
				return &eth.NodeInfo{
					Network:    uint64(l.chainId),
					Difficulty: td,
					Genesis:    genesisBlock.Hash(),
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

func (l *Client) handleNewTxs(peer *eth.Peer, txs types.Transactions, requestID uint64) {

	var txsUnknown []common.Hash
	//fmt.Printf("Peer:%v debug:发送来%v个tx，其中%v个为新的tx!\n", peer.ID(), txs.Len(), len(txsUnknown))
	for _, tx := range txs {
		if l.safeHasTx(tx.Hash()) {
			//l.logger.Info("收到了重复交易！", "节点ID", peer.ID())
			continue
		}
		txsUnknown = append(txsUnknown, tx.Hash())
		l.safeAddTx(tx.Hash())
	}
	if requestID != 0 {
		l.logger.Debug("远程节点返回了请求的交易！", "节点ID", peer.ID()[:10], "请求ID", requestID, "新交易数量", len(txsUnknown), "总数量", txs.Len())
	} else {
		l.logger.Debug("远程节点广播来一批交易！", "节点ID", peer.ID()[:10], "新交易数量", len(txsUnknown), "总数量", txs.Len())
	}

}

func (l *Client) handleNewAnns(peer *eth.Peer, anns []common.Hash) error {
	var unknownTxsHash []common.Hash
	for _, ann := range anns {
		if l.safeHasTx(ann) {
			continue
		}
		unknownTxsHash = append(unknownTxsHash, ann)
	}
	l.logger.Debug("远程节点宣布了一批hash！", "节点ID", peer.ID()[:10], "未知HASH数量", len(unknownTxsHash), "总数量", len(anns))
	if len(unknownTxsHash) == 1 {
		l.logger.Debug("宣布的hash内容", "节点ID", peer.ID()[:10], "hash", unknownTxsHash[0])
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
		// 更新节点的存活情况信息
		l.safeUpdateEthPeerStatus(peer)

		switch msg.Code {
		// 远程节点向我们广播了一批新的交易
		case eth.TransactionsMsg:
			var txs eth.TransactionsPacket
			if err = msg.Decode(&txs); err != nil {
				l.logger.Crit("远程节点发来新交易的解析失败！", "节点ID", peer.ID()[:10])
				return fmt.Errorf("%w: message %v: %v", errDecode, msg, err)
			}
			if len(txs) == 0 {
				l.logger.Warn("远程节点发来的交易解析出0个交易！", "节点ID", peer.ID()[:10])
				continue
			}
			l.handleNewTxs(peer, types.Transactions(txs), 0)

		// 远程节点发来我们刚才请求的一批交易
		case eth.PooledTransactionsMsg:
			var txs eth.PooledTransactionsPacket66
			if err = msg.Decode(&txs); err != nil {
				l.logger.Crit("远程节点返回的我们之前请求的交易解析失败！", "节点ID", peer.ID()[:10])
				return fmt.Errorf("%w: message %v: %v", errDecode, msg, err)
			}
			if len(txs.PooledTransactionsPacket) == 0 {
				//l.logger.Warn("远程节点返回的交易解析出0个交易！", "节点ID", peer.ID())
				l.logger.Debug("远程节点向我们返回了请求的交易！", "节点ID", peer.ID()[:10], "请求ID", txs.RequestId, "交易数量", len(txs.PooledTransactionsPacket))
				continue
			}
			l.handleNewTxs(peer, types.Transactions(txs.PooledTransactionsPacket), txs.RequestId)
		// 远程节点宣布了一批的交易
		case eth.NewPooledTransactionHashesMsg:
			ann := new(eth.NewPooledTransactionHashesPacket)
			if err = msg.Decode(ann); err != nil {
				return fmt.Errorf("%w: message %v: %v", errDecode, msg, err)
			}

			// 向远程节点请求具体的交易信息
			err = l.handleNewAnns(peer, *ann)
			if err != nil {
				return err
			}
		//	握手阶段远程节点会请求我们的block头，我们给他返回nil
		case eth.GetBlockHeadersMsg:
			var query eth.GetBlockHeadersPacket66
			if err := msg.Decode(&query); err != nil {
				return fmt.Errorf("%w: message %v: %v", errDecode, msg, err)
			}
			l.logger.Debug("远程节点请求我们发送block头！", "节点ID", peer.ID()[:10], "block高度", query.Origin)
			response := make([]*types.Header, 0)
			err = peer.ReplyBlockHeaders(query.RequestId, response)
			if err != nil {
				return err
			}
		default:
			l.logger.Debug("远程节点发来一个不能处理的消息！", "节点ID", peer.ID()[:10], "请求代码", msg.Code)
		}
		err = msg.Discard()
		if err != nil {
			return err
		}
	}
}
