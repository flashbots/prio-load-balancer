//go:build !sgx
// +build !sgx

package server

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

func NewNode(log *zap.SugaredLogger, uri string, jobC chan *SimRequest, numWorkers int32) (*Node, error) {
	node := &Node{
		log:        log,
		URI:        uri,
		AddedAt:    time.Now(),
		jobC:       jobC,
		numWorkers: numWorkers,
		client:     &http.Client{},
		enclave:    false,
	}
	return node, nil
}
