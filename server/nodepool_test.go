// Manages pool of execution nodes
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flashbots/prio-load-balancer/testutils"
	"github.com/stretchr/testify/require"
)

func TestNodePool(t *testing.T) {
	resetTestRedis()
	mockNodeBackend1 := testutils.NewMockNodeBackend()
	mockNodeServer1 := httptest.NewServer(http.HandlerFunc(mockNodeBackend1.Handler))

	mockNodeBackend2 := testutils.NewMockNodeBackend()
	mockNodeServer2 := httptest.NewServer(http.HandlerFunc(mockNodeBackend2.Handler))

	gp := NewNodePool(testLog, redisTestState, 1)
	err := gp.AddNode(mockNodeServer1.URL)
	require.Nil(t, err, err)

	err = gp.AddNode(mockNodeServer2.URL)
	require.Nil(t, err, err)

	nodes, err := redisTestState.GetNodes()
	require.Nil(t, err, err)
	require.Equal(t, 2, len(nodes))

	gp2 := NewNodePool(testLog, redisTestState, 1)
	err = gp2.LoadNodesFromRedis()
	require.Nil(t, err, err)
	require.Equal(t, 2, len(gp2.nodes))

	wasDeleted, err := gp2.DelNode(mockNodeServer1.URL)
	require.Nil(t, err, err)
	require.True(t, wasDeleted)
}

func TestNodePoolWithoutREDIS(t *testing.T) {
	mockNodeBackend1 := testutils.NewMockNodeBackend()
	mockNodeServer1 := httptest.NewServer(http.HandlerFunc(mockNodeBackend1.Handler))

	mockNodeBackend2 := testutils.NewMockNodeBackend()
	mockNodeServer2 := httptest.NewServer(http.HandlerFunc(mockNodeBackend2.Handler))

	gp := NewNodePool(testLog, nil, 1)
	err := gp.AddNode(mockNodeServer1.URL)
	require.Nil(t, err, err)

	err = gp.AddNode(mockNodeServer2.URL)
	require.Nil(t, err, err)
}

func TestNodePoolProxy(t *testing.T) {
	resetTestRedis()
	mockNodeBackend := testutils.NewMockNodeBackend()
	rpcBackendServer := httptest.NewServer(http.HandlerFunc(mockNodeBackend.Handler))

	gp := NewNodePool(testLog, redisTestState, 1)
	err := gp.AddNode(rpcBackendServer.URL)
	require.Nil(t, err, err)

	request := NewSimRequest("1", []byte("foo"), true, false)

	gp.JobC <- request
	res := <-request.ResponseC
	require.NotNil(t, res)
	require.Nil(t, res.Error, res.Error)
	require.Equal(t, 0, res.StatusCode)
}

func TestNodePoolWithError(t *testing.T) {
	mockNodeBackend := testutils.NewMockNodeBackend()
	mockNodeServer := httptest.NewServer(http.HandlerFunc(mockNodeBackend.Handler))

	gp := NewNodePool(testLog, nil, 1)
	err := gp.AddNode(mockNodeServer.URL)
	require.Nil(t, err, err)

	mockNodeBackend.HTTPHandlerOverride = func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "error", 479)
	}

	request := NewSimRequest("1", []byte("foo"), true, false)
	gp.JobC <- request
	res := <-request.ResponseC
	require.NotNil(t, res)
	require.NotNil(t, res.Error, res.Error)
}
