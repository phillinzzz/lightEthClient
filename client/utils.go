package client

import (
	"github.com/phillinzzz/newBsc/common"
	"github.com/phillinzzz/newBsc/core/types"
)

// 模拟的txPool
type fakeTxPool struct {
}

func (f fakeTxPool) Get(hash common.Hash) *types.Transaction {
	return nil
}
