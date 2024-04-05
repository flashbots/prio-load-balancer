//go:build !tee
// +build !tee

package server

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.uber.org/zap"
)

func NewNode(log *zap.SugaredLogger, uri string, jobC chan *SimRequest, numWorkers int32) (*Node, error) {
	pURL, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, err
	}

	workersArg := pURL.Query().Get("_workers")
	if workersArg != "" {
		// set numWorkers from query param
		workersInt, err := strconv.Atoi(workersArg)
		if err != nil {
			log.Errorw("Error parsing workers query param", "err", err, "uri", uri)
		} else {
			log.Infow("Using custom number of workers", "workers", workersInt, "uri", uri)
			numWorkers = int32(workersInt)
		}
	}

	node := &Node{
		log:        log,
		URI:        uri,
		AddedAt:    time.Now(),
		jobC:       jobC,
		numWorkers: numWorkers,
		client: &http.Client{
			Timeout: ProxyRequestTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxConnsPerHost:     100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
	return node, nil
}
