package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flashbots/prio-load-balancer/testutils"
	"github.com/stretchr/testify/require"
)

func TestNode(t *testing.T) {
	mockNodeBackend1 := testutils.NewMockNodeBackend()
	mockNodeServer1 := httptest.NewServer(http.HandlerFunc(mockNodeBackend1.Handler))

	jobC := make(chan *SimRequest)
	node, err := NewNode(testLog, mockNodeServer1.URL, jobC, 1)
	require.Nil(t, err, err)

	err = node.HealthCheck()
	require.Nil(t, err, err)

	request := NewSimRequest([]byte("foo"), true, false)
	node.StartWorkers()
	node.jobC <- request
	res := <-request.ResponseC
	require.NotNil(t, res, res)
	require.Nil(t, res.Error, res.Error)
	node.StopWorkersAndWait()
	require.Equal(t, int32(0), node.curWorkers)

	// Invalid backend -> fail healthcheck
	node, err = NewNode(testLog, "http://localhost:4831", nil, 1)
	require.Nil(t, err, err)

	err = node.HealthCheck()
	require.NotNil(t, err, err)
}

func TestNodeError(t *testing.T) {
	mockNodeBackend := testutils.NewMockNodeBackend()
	mockNodeServer := httptest.NewServer(http.HandlerFunc(mockNodeBackend.Handler))
	mockNodeBackend.HTTPHandlerOverride = func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "error", 479)
	}

	jobC := make(chan *SimRequest)
	node, err := NewNode(testLog, mockNodeServer.URL, jobC, 1)
	require.Nil(t, err, err)

	// Check failing healthcheck
	err = node.HealthCheck()
	require.NotNil(t, err, err)
	require.Contains(t, err.Error(), "479")

	// Check failing ProxyRequest
	_, statusCode, err := node.ProxyRequest([]byte("net_version"), 3*time.Second)
	require.NotNil(t, err, err)
	require.Equal(t, 479, statusCode)

	// Check failing SimRequest
	request := NewSimRequest([]byte("foo"), true, false)
	node.StartWorkers()
	node.jobC <- request
	res := <-request.ResponseC
	require.NotNil(t, res, res)
	require.NotNil(t, res.Error, res.Error)
	require.Contains(t, res.Error.Error(), "error")
	require.Contains(t, res.Error.Error(), "479")
	require.Equal(t, 479, res.StatusCode)
}
