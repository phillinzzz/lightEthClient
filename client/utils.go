package client

import (
	"fmt"
	"github.com/phillinzzz/newBsc/common"
	"github.com/phillinzzz/newBsc/core/forkid"
	"github.com/phillinzzz/newBsc/core/types"
)

// 模拟的txPool
type fakeTxPool struct {
}

func (f fakeTxPool) Get(hash common.Hash) *types.Transaction {
	return nil
}

// 调试用
func MyFilter(id forkid.ID) error {
	fmt.Printf("远程节点的FORK ID是：%+v\n", id)
	return nil
}
