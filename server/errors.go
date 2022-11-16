package server

import "errors"

var (
	ErrRequestTimeout   = errors.New("request timeout hit before processing")
	ErrNodeTimeout      = errors.New("node timeout")
	ErrNoNodesAvailable = errors.New("no nodes available")
)
