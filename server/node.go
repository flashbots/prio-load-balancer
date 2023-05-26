package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Node struct {
	log           *zap.SugaredLogger
	URI           string
	AddedAt       time.Time
	jobC          chan *SimRequest
	numWorkers    int32
	curWorkers    int32
	cancelContext context.Context
	cancelFunc    context.CancelFunc
	client        *http.Client
}

func (n *Node) HealthCheck() error {
	payload := `{"jsonrpc":"2.0","method":"net_version","params":[],"id":123}`
	_, _, err := n.ProxyRequest([]byte(payload), 5*time.Second)
	return err
}

func (n *Node) startProxyWorker(id int32, cancelContext context.Context) {
	n.log.Infow("starting proxy node worker", "uri", n.URI, "id", id)
	atomic.AddInt32(&n.curWorkers, 1)
	defer atomic.AddInt32(&n.curWorkers, -1)

	for {
		select {
		case req := <-n.jobC:
			if req.Cancelled {
				continue
			}

			if time.Since(req.CreatedAt) > RequestTimeout {
				n.log.Info("request timed out before processing")
				req.SendResponse(SimResponse{Error: ErrRequestTimeout})
				continue
			}

			req.Tries += 1
			payload, statusCode, err := n.ProxyRequest(req.Payload, ProxyRequestTimeout)
			if err != nil {
				n.log.Errorw("node proxyRequest error", "uri", n.URI, "error", err)
				response := SimResponse{StatusCode: statusCode, Payload: payload, Error: err, ShouldRetry: true, NodeURI: n.URI}
				req.SendResponse(response)
				continue
			}

			// Send response
			sent := req.SendResponse(SimResponse{Payload: payload, NodeURI: n.URI})
			if !sent {
				n.log.Errorw("couldn't send node response to client (SendResponse returned false)", "secSinceRequestCreated", time.Since(req.CreatedAt).Seconds())
			}

		case <-cancelContext.Done():
			n.log.Infow("node worker stopped", "uri", n.URI, "id", id)
			return
		}
	}
}

// StartWorkers spawns the proxy workers in goroutines. Workers that are already running will be cancelled.
func (n *Node) StartWorkers() {
	if n.cancelFunc != nil {
		n.cancelFunc()
	}

	n.cancelContext, n.cancelFunc = context.WithCancel(context.Background())
	for i := int32(0); i < n.numWorkers; i++ {
		go n.startProxyWorker(i+1, n.cancelContext)
	}
}

func (n *Node) StopWorkers() {
	if n.cancelFunc != nil {
		n.cancelFunc()
	}
}

func (n *Node) StopWorkersAndWait() {
	n.StopWorkers()
	for {
		if n.curWorkers == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (n *Node) ProxyRequest(payload []byte, timeout time.Duration) (resp []byte, statusCode int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", n.URI, bytes.NewBuffer(payload))
	if err != nil {
		return resp, statusCode, errors.Wrap(err, "creating proxy request failed")
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Content-Length", strconv.Itoa(len(payload)))

	httpResp, err := n.client.Do(httpReq)
	if err != nil {
		return resp, statusCode, errors.Wrap(err, "proxying request failed")
	}

	statusCode = httpResp.StatusCode

	defer httpResp.Body.Close()
	httpRespBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, statusCode, errors.Wrap(err, "decoding proxying response failed")
	}

	if statusCode >= 400 {
		return httpRespBody, statusCode, fmt.Errorf("error in response - statusCode: %d / %s", statusCode, httpRespBody)
	}

	return httpRespBody, statusCode, nil
}
