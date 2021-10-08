package log2

import (
	"github.com/ethereum/go-ethereum/log"
	"os"
)

var logger log.Logger

// customize the logger
func init() {
	logger = log.New()
	handler := log.StreamHandler(os.Stdout, log.LogfmtFormat())
	log.Root().SetHandler(handler)
}

func GetLogger() log.Logger {
	return logger
}
