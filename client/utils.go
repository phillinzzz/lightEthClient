package client

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// 模拟的txPool
type fakeTxPool struct {
}

func (f fakeTxPool) Get(hash common.Hash) *types.Transaction {
	return nil
}
