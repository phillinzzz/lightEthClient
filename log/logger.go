package log2

import (
	"github.com/phillinzzz/newBsc/log"
	"os"
)

var MyLogger log.Logger

// customize the logger
func init() {
	MyLogger = log.New()
	handler := log.StreamHandler(os.Stdout, log.LogfmtFormat())
	handler2 := log.MatchFilterHandler("模块", "ETH", handler)
	handler3 := log.LvlFilterHandler(log.LvlInfo, handler2)
	MyLogger.SetHandler(handler3)
}
