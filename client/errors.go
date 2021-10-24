package client

import "errors"

var (
	errDecode    error = errors.New("invalid message")
	errDuplicate error = errors.New("重复的节点")
)
