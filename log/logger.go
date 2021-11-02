package log2

import (
	"github.com/ethereum/go-ethereum/log"
	"os"
)

var MyLogger log.Logger

// customize the logger
func init() {
	MyLogger = log.New()
	handler := log.StreamHandler(os.Stdout, log.LogfmtFormat())
	handler2 := log.MatchFilterHandler("模块", "ETH", handler)
	MyLogger.SetHandler(handler2)
}
