package client

import (
	"fmt"
	"github.com/btcsuite/btcd/peer"
	"github.com/phillinzzz/lightEthClient/config"
	"github.com/phillinzzz/newBsc/cmd/utils"
	"github.com/phillinzzz/newBsc/common"
	"github.com/phillinzzz/newBsc/core"
	"github.com/phillinzzz/newBsc/core/forkid"
	"github.com/phillinzzz/newBsc/core/types"
	"github.com/phillinzzz/newBsc/crypto"
	"github.com/phillinzzz/newBsc/eth/ethconfig"
	"github.com/phillinzzz/newBsc/eth/protocols/eth"
	"github.com/phillinzzz/newBsc/log"
	"github.com/phillinzzz/newBsc/p2p"
	"github.com/phillinzzz/newBsc/p2p/dnsdisc"
	"github.com/phillinzzz/newBsc/p2p/enode"
	"github.com/phillinzzz/newBsc/params"
	"math/big"
	"os"
	"sync"
	"time"
)

// Mode 客户端的几种工作模式
type Mode int

const (
	ModeDebug = iota
	ModeInfo
	ModeProduce
)

type ChainID uint64

const (
	ETHChainID  ChainID = 1
	BSCChainID  ChainID = 56
	HECOChainID ChainID = 99
)

type Client struct {
	mode        Mode
	chainId     ChainID
	chainConfig *params.ChainConfig
	logger      log.Logger
	p2pServer   p2p.Server

	// 节点管理相关
	ethPeersPoolLock sync.RWMutex
	ethPeersPool     map[*eth.Peer][]time.Time // 存放各个节点每次第一个收到transmit的时间，用来比较节点的优劣

	// tx池管理相关
	knownTxsPool     map[common.Hash]time.Time
	knownTxsPoolLock sync.RWMutex

	// 对外服务
	newTxListenChan chan *types.Transaction //发现的新的交易由client发送到这个通道里,供外部监听
	broadcastTxChan chan *types.Transaction //需要广播的交易由外部发送到这个通道里，交由client进行广播
}

// NewClient 新建一个客户端，是否运行由run参数决定
func NewClient(chainId ChainID, mode Mode, run bool) *Client {
	newClient := &Client{
		mode:    mode,
		chainId: chainId,
	}

	newClient.knownTxsPool = make(map[common.Hash]time.Time)
	newClient.ethPeersPool = make(map[*eth.Peer][]time.Time)

	newClient.newTxListenChan = make(chan *types.Transaction, 1000)
	newClient.broadcastTxChan = make(chan *types.Transaction, 5)

	// 配置logger
	baseLogger := log.New()
	baseHandler := log.StreamHandler(os.Stdout, log.LogfmtFormat())
	switch mode {
	case ModeDebug:
		baseLogger.SetHandler(baseHandler)

	case ModeInfo:
		infoHandler := log.LvlFilterHandler(log.LvlInfo, baseHandler)
		baseLogger.SetHandler(infoHandler)

	case ModeProduce:
		baseLogger.SetHandler(log.DiscardHandler())
	}

	// client自己使用的logger（ETH层面）
	newClient.logger = baseLogger.New("模块", "ETH")

	// p2p层面使用的logger
	p2pLogger := baseLogger.New("模块", "p2p")

	// 生成p2p配置参数
	p2pCfg, _ := config.GetP2PConfig(uint64(chainId), p2pLogger)
	// 生成p2pServer
	newClient.p2pServer = p2p.Server{Config: p2pCfg}

	protos := newClient.makeProtocols()
	newClient.p2pServer.Protocols = protos

	if run {
		newClient.run()
	}

	return newClient
}

// 启动客户端
func (l *Client) run() {
	if err := l.p2pServer.Start(); err != nil {
		l.logger.Crit("Failed to start p2p server", "err", err)
		return
	}

	go l.knownTxsPoolCleanLoop(time.Second*15, time.Second*3)
	go l.ethPeerCleanLoop(time.Minute*2, time.Minute*2)
	go l.broadcastTxsLoop()
}

// 根据不同的网络，进行握手参数的配置
func (l *Client) makeProtocols() []p2p.Protocol {

	var (
		//genesis *core.Genesis
		// 创世区块的hash
		genesisHash common.Hash
		// 链配置，内含硬分叉信息
		chainConfig *params.ChainConfig

		// 当前总难度（创世区块难度）
		td *big.Int
	)

	// 生成创世区块
	switch l.chainId {
	case ETHChainID:
		//genesis = core.DefaultGenesisBlock()
		td = core.DefaultGenesisBlock().Difficulty
		genesisHash = params.MainnetGenesisHash
		chainConfig = params.MainnetChainConfig
	case BSCChainID:
		//genesis = bscConfig.MakeBSCGenesis()
		td = big.NewInt(1)
		genesisHash = params.BSCGenesisHash
		chainConfig = params.BSCChainConfig
	default:
		l.logger.Crit("未配置网络参数！", "ChainID", l.chainId)
	}

	//genesisHash := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	//genesisBlock := genesis.ToBlock(nil)

	// 握手需要的信息
	var (
		forkFilter = forkid.NewStaticFilter(chainConfig, genesisHash) //验证对方的forkID的函数
		forkID     = forkid.NewID(chainConfig, genesisHash, 0)        //握手时发送给对方的forkID
	)

	ethConfig := ethconfig.Defaults
	//eth主网有DNS节点列表功能，bsc网络没有此功能
	if l.chainId == ETHChainID {
		utils.SetDNSDiscoveryDefaults(&ethConfig, genesisHash)
	}

	// Setup DNS discovery iterators. 只对ETH主网起效果。
	dnsclient := dnsdisc.NewClient(dnsdisc.Config{})
	ethDialCandidates, _ := dnsclient.NewIterator(ethConfig.EthDiscoveryURLs...)

	protocolVersions := eth.ProtocolVersions
	protocolName := eth.ProtocolName
	protocolLengths := map[uint]uint64{eth.ETH67: 18, eth.ETH66: 17, eth.ETH65: 17}

	protocols := make([]p2p.Protocol, len(protocolVersions))
	for i, version := range protocolVersions {
		version := version // Closure

		protocols[i] = p2p.Protocol{
			Name:    protocolName,
			Version: version,
			Length:  protocolLengths[version],
			// Run函数用来初始化p2p节点并将其升级为ethPeer。Run函数执行后，就有了ethPeer。当Run函数返回以后，ethPeer也就已经关闭了
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
				l.logger.Debug("发现一个p2p节点!", "protocol", version, "节点ID", p.ID().String()[:10])
				//检查该节点是否为已知节点
				if err := l.safeCheckPeerDuplicate(p); err != nil {
					l.logger.Debug("p2p节点已存在！", "节点ID", p.ID().String()[:10])
					return err
				}
				// create the ethPeer
				peer := eth.NewPeer(version, p, rw, fakeTxPool{})
				defer peer.Close()
				// Execute the Ethereum (block chain) handshake
				//l.logger.Info("准备与p2p节点进行握手！", "protocol", version, "节点ID", p.ID().String()[:10])

				if err := peer.Handshake(uint64(l.chainId), td, genesisHash, genesisHash, forkID, forkFilter, &eth.UpgradeStatusExtension{DisablePeerTxBroadcast: false}); err != nil {
					l.logger.Debug("与p2p节点握手失败", "protocol", version, "节点ID", p.ID().String()[:10], "原因", err)
					return err
				}
				l.logger.Debug("与p2p节点握手成功", "protocol", version, "节点ID", p.ID().String()[:10])
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
					Genesis:    genesisHash,
					Config:     chainConfig,
					Head:       genesisHash,
				}
			},
			PeerInfo: func(id enode.ID) interface{} {
				if peer := l.safeFindPeer(id.String()); peer != nil {
					return peer.Info()
				}
				return nil
			},
			DialCandidates: ethDialCandidates,
		}
	}
	return protocols
}

// ==================================================
// 对外服务相关函数

// GetNewTxListenChan 外部订阅监听网络上新的交易的通道，只能订阅一次，否则变成负载分流
func (l *Client) GetNewTxListenChan() <-chan *types.Transaction {
	return l.newTxListenChan
}

// GetBroadcastTxChan 外部发送来新的平仓交易的通道，可以多次订阅
func (l *Client) GetBroadcastTxChan() chan<- *types.Transaction {
	return l.broadcastTxChan
}

// ==================================================
// 交易监听发送等函数

// 监听节点发来的信息
// 节点建立时运行的函数，使用for循环处理peer发来的全部消息。如果此函数返回，则节点将会被drop
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

// 广播交易
// todo:需要改写，当前利用了原装的ethPeer的函数，每次发送一笔交易都会记录在ethPeer的已知交易内，时间长了体积会越来越大。
func (l *Client) broadcastTxsLoop() {
	for tx := range l.broadcastTxChan {
		txs := []*types.Transaction{tx}
		l.ethPeersPoolLock.RLock()
		l.logger.Info("向远程节点广播了一笔交易！", "交易hash", tx.Hash())
		for _, ethPeer := range l.ethPeers {
			// todo：需要重新改造ethPeer "type myEthPeer eth.Peer"
			// 向某个节点发送交易，超时则断开和这个节点的连接
			newPeer := ethPeer
			go func() {
				if err := newPeer.SendTransactions(txs); err != nil {
					l.logger.Warn("向远程节点广播交易超时！", "节点ID", ethPeer.ID()[:10], "原因", err)
					newPeer.Disconnect(p2p.DiscUselessPeer)
				}
			}()
			//p2p.Send(p.rw, TransactionsMsg, txs)
		}
		l.ethPeersPoolLock.RUnlock()
	}
}

// 将接收到的新的tx发送到监听通道里面，通知外部程序
func (l *Client) handleNewTxs(peer *eth.Peer, txs types.Transactions, requestID uint64) {

	//var txsUnknown []common.Hash
	for _, tx := range txs {
		txHash := tx.Hash()

		// 忽略池子中已经存在的交易
		if l.safeHasTx(txHash) {
			continue
		}

		// 将新发现的交易加入池子
		l.safeAddTx(txHash)

		// 忽略非transmit交易
		if !isTransmitTx(tx) {
			continue
		}

		// 更新节点获胜信息，发来了transmit信息的节点才是好节点
		l.safeEthPeerWinNotify(peer)

		// 对外通知发现了新交易
		select {
		case l.newTxListenChan <- tx:
		default:
			l.logger.Warn("外部程序未能及时读取新的Tx信息！")
		}
		//// 内部管理新的交易，防止重复获取
		//txsUnknown = append(txsUnknown, tx.Hash())
	}
	//if l.mode == ModeDebug {
	//	if requestID != 0 {
	//		l.logger.Debug("远程节点返回了请求的交易！", "节点ID", peer.ID()[:10], "请求ID", requestID, "新交易数量", len(txsUnknown), "总数量", txs.Len())
	//	} else {
	//		l.logger.Debug("远程节点广播来一批交易！", "节点ID", peer.ID()[:10], "新交易数量", len(txsUnknown), "总数量", txs.Len())
	//	}
	//}
}

// 处理接收到的hash广播，决定是否需要去请求节点发来真正的tx
func (l *Client) handleNewAnns(peer *eth.Peer, anns []common.Hash) error {
	var unknownTxsHash []common.Hash
	for _, ann := range anns {
		if l.safeHasTx(ann) {
			continue
		}
		unknownTxsHash = append(unknownTxsHash, ann)
	}
	l.logger.Debug("远程节点宣布了一批hash！", "节点ID", peer.ID()[:10], "未知HASH数量", len(unknownTxsHash), "总数量", len(anns))

	// 向远程节点请求具体的交易信息
	err := peer.RequestTxs(unknownTxsHash)
	if err != nil {
		return err
	}
	return nil
}

var funcId = crypto.Keccak256([]byte("transmit(bytes,bytes32[],bytes32[],bytes32)"))[:4]

// 检测交易是否为transmit交易
func isTransmitTx(tx *types.Transaction) bool {

	if cap(tx.Data()) < 5 || string(funcId) != string(tx.Data()[:4]) {
		return false
	}
	return true
}

// ==================================================
// ethPeer管理相关函数

// 根据id查找节点
func (l *Client) safeFindPeer(id string) *eth.Peer {
	l.ethPeersPoolLock.RLock()
	defer l.ethPeersPoolLock.RUnlock()
	for peer := range l.ethPeersPool {
		if peer.ID() == id {
			return peer
		}
	}
	return nil
}

// 检测节点是否重复
func (l *Client) safeCheckPeerDuplicate(peer *p2p.Peer) error {

	if l.safeFindPeer(peer.ID().String()) != nil {
		l.p2pServer.Logger.Info("p2p节点已存在，放弃之", "节点ID", peer.ID().String())
		return errDuplicate
	}

	return nil
}

// 将新的节点登记到map中，便于后续管理
func (l *Client) safeRegisterEthPeer(ethPeer *eth.Peer) {
	l.ethPeersPoolLock.Lock()
	defer l.ethPeersPoolLock.Unlock()
	l.ethPeersPool[ethPeer] = []time.Time{}
	l.logger.Debug("新ETH节点注册成功，当前已连接ETH Peer总数", "数量", len(l.ethPeersPool))
}

// 注销已断开的节点
func (l *Client) safeUnregisterEthPeer(ethPeer *eth.Peer) {
	l.ethPeersPoolLock.Lock()
	defer l.ethPeersPoolLock.Unlock()
	delete(l.ethPeersPool, ethPeer)
	l.logger.Debug("ETH节点移除完成，当前已连接ETH Peer总数", "数量", len(l.ethPeersPool))
}

// ethPeer在所有的节点中第一个收到transmit信息，记功一次，便于后期评比
func (l *Client) safeEthPeerWinNotify(peer *eth.Peer) {
	l.ethPeersPoolLock.RLock()
	defer l.ethPeersPoolLock.RUnlock()

	l.ethPeersPool[peer] = append(l.ethPeersPool[peer], time.Now())
}

// 定期清理掉劣质的节点：获胜次数最少的节点
func (l *Client) ethPeerCleanLoop(maxOldTime, loopTime time.Duration) {

	for range time.Tick(loopTime) {
		// step1 清理远古的获奖信息
		l.safeCleanTxsPool(maxOldTime)

		if l.mode != ModeProduce {
			l.safeShowPeersPool()
		}

		l.ethPeersPoolLock.RLock()
		// 选出获胜次数最少的节点
		var (
			badPeer *eth.Peer
			count   int // 最小的获胜次数
		)
		for p, wins := range l.ethPeersPool {
			if len(wins) <= count {
				badPeer = p
				count = len(wins)
			}
		}
		//移除长时间没有通信的远程节点
		if time.Since(t) > maxOldTime {
			l.logger.Info("移除长时间没有通信的节点", "节点ID", peerName[:10])
			l.ethPeers[peerName].Disconnect(p2p.DiscUselessPeer)
			// 与远程节点断开连接后，protocol里面的Run函数出错返回，将会调用safeUnregisterEthPeer进行清理工作
		}

		l.ethPeersPoolLock.RUnlock()
	}

}

// 只保留最近maxTime的获奖信息
func (l *Client) safeCleanPeersPool(maxOldTime time.Duration) {
	l.ethPeersPoolLock.Lock()
	defer l.ethPeersPoolLock.Unlock()
	// 对每个peer的获奖信息进行清理
loop:
	for _, wins := range l.ethPeersPool {
		// 查找远古的获奖记录，并剔除
		for i, winTime := range wins {
			// 找到了符合要求的第一个（最大的）时间的位置，在此之后的时间都更新，更符合要求
			if time.Since(winTime) < maxOldTime {
				wins = wins[i:]
				continue loop
			}
		}
		// 能运行到这里，说明所有的时间都不符合要求（太老了）
		wins = wins[:0]
	}
}

// 展示节点信息
func (l *Client) safeShowPeersPool() {
	l.ethPeersPoolLock.RLock()
	defer l.ethPeersPoolLock.RUnlock()

	l.logger.Info("展示节点信息：", "节点总数：", len(l.ethPeersPool))
	for p, wins := range l.ethPeersPool {
		l.logger.Info("展示节点信息：", "节点ID", p.ID()[:10], "获奖次数：", len(wins))
		l.logger.Info("展示节点信息：", "节点ID", p.ID()[:10], "获奖时间：", wins[:5])

	}
}

// ==================================================
// knownTxsPool相关处理函数

// 定期清理已知的交易，防止重复向远程节点获取已知的交易
func (l *Client) knownTxsPoolCleanLoop(loopTime, maxDuration time.Duration) {
	l.logger.Info("开始启动交易池自动清理循环")
	ticker := time.NewTicker(loopTime)
	defer ticker.Stop()
	for {
		<-ticker.C
		l.safeCleanTxsPool(maxDuration)

	}
}

// 检测对于knownTxsPool该hash是否已知, true: known; false: unknown
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

// 检测当前knownTxsPool内的交易数量
func (l *Client) safeCountTx() int {
	l.knownTxsPoolLock.RLock()
	defer l.knownTxsPoolLock.RUnlock()
	return len(l.knownTxsPool)
}

// 清理txsPool，将老旧的txs清除掉
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
