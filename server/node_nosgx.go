//go:build !sgx
// +build !sgx

package server

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// InitializeRATLSLib is a no-op for non-SGX builds
func InitializeRATLSLib(_ bool, _ time.Duration, _ bool) error {
	return nil
}

func NewNode(log *zap.SugaredLogger, uri string, jobC chan *SimRequest, numWorkers int32) (*Node, error) {
	node := &Node{
		log:        log,
		URI:        uri,
		AddedAt:    time.Now(),
		jobC:       jobC,
		numWorkers: numWorkers,
		client:     &http.Client{},
	}
	return node, nil
}
