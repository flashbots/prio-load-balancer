package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flashbots/prio-load-balancer/testutils"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var (
	testServerListenAddr = "localhost:9498"
	testLogger, _        = zap.NewDevelopment()
	testLog              = testLogger.Sugar()
)

func TestServerWithoutRedis(t *testing.T) {
	s, err := NewServer(ServerOpts{testLog, testServerListenAddr, "", 1})
	require.Nil(t, err, err)

	mockNodeBackend := testutils.NewMockNodeBackend()
	mockNodeServer := httptest.NewServer(http.HandlerFunc(mockNodeBackend.Handler))
	s.AddNode(mockNodeServer.URL)
	go s.Start()
	defer s.Shutdown()
	time.Sleep(100 * time.Millisecond) // give Github CI time to start the webserver

	url := "http://" + testServerListenAddr
	resp, err := http.PostForm(url, nil)
	require.Nil(t, err, err)
	require.Equal(t, 200, resp.StatusCode)

	url = "http://" + testServerListenAddr + "/"
	resp, err = http.PostForm(url, nil)
	require.Nil(t, err, err)
	require.Equal(t, 200, resp.StatusCode)
}

func TestServerWithRedis(t *testing.T) {
	resetTestRedis()

	s, err := NewServer(ServerOpts{testLog, testServerListenAddr, redisTestServer.Addr(), 1})
	require.Nil(t, err, err)

	mockNodeBackend := testutils.NewMockNodeBackend()
	mockNodeServer := httptest.NewServer(http.HandlerFunc(mockNodeBackend.Handler))
	s.AddNode(mockNodeServer.URL)
	go s.Start()
	defer s.Shutdown()
	time.Sleep(100 * time.Millisecond) // give Github CI time to start the webserver

	url := "http://" + testServerListenAddr + "/"
	resp, err := http.PostForm(url, nil)
	require.Nil(t, err, err)
	require.Equal(t, 200, resp.StatusCode)
}

func TestServerNoNodes(t *testing.T) {
	s, err := NewServer(ServerOpts{testLog, testServerListenAddr, "", 1})
	require.Nil(t, err, err)
	go s.Start()
	defer s.Shutdown()
	time.Sleep(100 * time.Millisecond) // give Github CI time to start the webserver

	url := "http://" + testServerListenAddr
	resp, err := http.PostForm(url, nil)
	require.Nil(t, err, err)
	require.Equal(t, 500, resp.StatusCode)

	bb, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(bb), "no nodes")
}

// TestServerShutdown tests the graceful shutdown of the server
func TestServerShutdown(t *testing.T) {
	s, err := NewServer(ServerOpts{testLog, testServerListenAddr, "", 1})
	require.Nil(t, err, err)

	done := make(chan bool)
	go func() {
		s.Start()
		done <- true
	}()

	time.Sleep(100 * time.Millisecond)
	s.Shutdown()
	isDone := <-done
	require.True(t, isDone)
}

// TestServerJobTimeout ensures that the server will timeout a job if it takes too long
func TestServerJobTimeout(t *testing.T) {
	s, err := NewServer(ServerOpts{testLog, testServerListenAddr, "", 0}) // 0 workers per node -> no jobs can be picked up
	require.Nil(t, err, err)
	s.nodePool.JobC = make(chan *SimRequest) // disable buffer on job queue

	mockNodeBackend := testutils.NewMockNodeBackend()
	mockNodeServer := httptest.NewServer(http.HandlerFunc(mockNodeBackend.Handler))
	s.AddNode(mockNodeServer.URL)
	go s.Start()
	defer s.Shutdown()
	time.Sleep(100 * time.Millisecond) // give Github CI time to start the webserver

	url := "http://" + testServerListenAddr
	reqPayload := testutils.NewJSONRPCRequest1(1, "eth_callBundle", "0x1")
	reqPayloadBytes, err := json.Marshal(reqPayload)
	require.Nil(t, err, err)
	resp, _ := http.Post(url, "application/json", bytes.NewBuffer(reqPayloadBytes))
	require.Nil(t, err, err)
	require.Equal(t, 500, resp.StatusCode)
	lenFT, lenHP, lenLP := s.prioQueue.Len()
	require.Equal(t, 0, lenFT+lenHP+lenLP)
}
