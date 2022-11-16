// Manages pool of execution nodes
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flashbots/prio-load-balancer/testutils"
	"github.com/stretchr/testify/require"
)

func TestWebserver(t *testing.T) {
	resetTestRedis()

	prioQueue := NewPrioQueue(0, 0)
	nodePool := NewNodePool(testLog, redisTestState, 1)
	webserver := NewWebserver(testLog, ":12345", prioQueue, nodePool)

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	handler := http.HandlerFunc(webserver.HandleNodesRequest)

	mockNodeBackend := testutils.NewMockNodeBackend()
	mockNodeServer := httptest.NewServer(http.HandlerFunc(mockNodeBackend.Handler))

	// GET /nodes request (empty list)
	getNodesReq, _ := http.NewRequest("GET", "/nodes", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, getNodesReq)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "[]\n", rr.Body.String())

	// Add a node with POST /nodes request
	addNodePayload := fmt.Sprintf(`{"uri":"%s"}`, mockNodeServer.URL)
	addNodeReq, _ := http.NewRequest("POST", "/nodes", bytes.NewBufferString(addNodePayload))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, addNodeReq)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, 1, len(nodePool.nodes))

	// check redis
	nodesFromRedis, err := redisTestState.GetNodes()
	require.Nil(t, err, err)
	require.Equal(t, 1, len(nodesFromRedis))

	// Get list of nodes with length 1
	getNodesReq, _ = http.NewRequest("GET", "/nodes", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, getNodesReq)
	require.Equal(t, http.StatusOK, rr.Code)
	nodes := []string{}
	err = json.Unmarshal(rr.Body.Bytes(), &nodes)
	require.Nil(t, err, err)
	require.Equal(t, 1, len(nodes))

	// Noop an error on adding a node twice
	addNodeReq, _ = http.NewRequest("POST", "/nodes", bytes.NewBufferString(addNodePayload))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, addNodeReq)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, 1, len(nodePool.nodes))

	// Delete a non-existing node with DELETE /nodes request
	delNodePayload := `{"uri":"http://localhost:8545X"}`
	delNodeReq, _ := http.NewRequest("DELETE", "/nodes", bytes.NewBufferString(delNodePayload))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, delNodeReq)
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, 1, len(nodePool.nodes))

	// Delete a node with DELETE /nodes request
	delNodePayload = fmt.Sprintf(`{"uri":"%s"}`, mockNodeServer.URL)
	delNodeReq, _ = http.NewRequest("DELETE", "/nodes", bytes.NewBufferString(delNodePayload))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, delNodeReq)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, 0, len(nodePool.nodes))

	// check redis
	nodesFromRedis, err = redisTestState.GetNodes()
	require.Nil(t, err, err)
	require.Equal(t, 0, len(nodesFromRedis))

	// Try to add an invalid node with POST /nodes request
	addNodePayload = `{"uri":"http://localhost:12354"}`
	addNodeReq, _ = http.NewRequest("POST", "/nodes", bytes.NewBufferString(addNodePayload))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, addNodeReq)
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, 0, len(nodePool.nodes))
}

func TestWebserverSim(t *testing.T) {
	mockNodeBackend := testutils.NewMockNodeBackend()
	mockNodeServer := httptest.NewServer(http.HandlerFunc(mockNodeBackend.Handler))

	prioQueue := NewPrioQueue(0, 0)
	nodePool := NewNodePool(testLog, nil, 1)
	nodePool.AddNode(mockNodeServer.URL)
	webserver := NewWebserver(testLog, ":12345", prioQueue, nodePool)
	handler := http.HandlerFunc(webserver.HandleQueueRequest)

	// Pump jobs from prioQueue to nodepool
	go func() {
		for {
			job := prioQueue.Pop()
			if job == nil {
				return
			}
			nodePool.JobC <- job
		}
	}()

	// Test valid sim request
	reqPayload := testutils.NewJSONRPCRequest1(1, "eth_callBundle", "0x1")
	reqPayloadBytes, err := json.Marshal(reqPayload)
	require.Nil(t, err, err)
	getSimReq, _ := http.NewRequest("POST", "/", bytes.NewBuffer(reqPayloadBytes))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, getSimReq)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, `{"id":1,"result":"cool","jsonrpc":"2.0"}`+"\n", rr.Body.String())

	// Test node error handling
	mockNodeBackend.Reset()
	mockNodeBackend.HTTPHandlerOverride = func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "error", 479)
	}
	getSimReq, _ = http.NewRequest("POST", "/", bytes.NewBuffer(reqPayloadBytes))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, getSimReq)
	require.Equal(t, 479, rr.Code)
	require.Equal(t, "error\n", rr.Body.String())

	// Test request cancelling (using a custom backend handler override to wait for 5 seconds)
	mockNodeBackend.Reset()
	mockNodeBackend.RPCHandlerOverride = func(req *testutils.JSONRPCRequest) (result interface{}, err error) {
		time.Sleep(5 * time.Second)
		return nil, errors.New("timeout")
	}
	getSimReq, _ = http.NewRequest("POST", "/", bytes.NewBuffer(reqPayloadBytes))
	ctx, cancel := context.WithCancel(context.Background())
	getSimReqWithContext := getSimReq.WithContext(ctx)

	rr = httptest.NewRecorder()
	t1 := time.Now()
	doneC := make(chan bool)
	go func() {
		handler.ServeHTTP(rr, getSimReqWithContext) // would take 5 seconds without cancelling
		doneC <- true
	}()
	cancel()
	<-doneC
	tX := time.Since(t1)
	require.True(t, tX.Seconds() < 1, "should have been cancelled")
	// Here no further requests can be made!
}
