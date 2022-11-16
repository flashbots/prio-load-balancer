// Package testutils contains a mock execution backend (for testing and dev purposes)
package testutils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

var testLogger, _ = zap.NewDevelopment()
var testLog = testLogger.Sugar()

type MockNodeBackend struct {
	LastRawRequest              *http.Request
	LastJSONRPCRequest          *JSONRPCRequest
	LastJSONRPCRequestTimestamp time.Time
	RPCHandlerOverride          func(req *JSONRPCRequest) (result interface{}, err error)
	HTTPHandlerOverride         func(w http.ResponseWriter, req *http.Request)
}

func NewMockNodeBackend() *MockNodeBackend {
	return &MockNodeBackend{}
}

func (be *MockNodeBackend) Reset() {
	be.LastRawRequest = nil
	be.LastJSONRPCRequest = nil
	be.LastJSONRPCRequestTimestamp = time.Time{}
	be.RPCHandlerOverride = nil
	be.HTTPHandlerOverride = nil
}

func (be *MockNodeBackend) handleRPCRequest(req *JSONRPCRequest) (result interface{}, err error) {
	if be.RPCHandlerOverride != nil {
		return be.RPCHandlerOverride(req)
	}

	be.LastJSONRPCRequest = req

	switch req.Method {
	case "net_version":
		return "1", nil
	case "eth_callBundle":
		return "cool", nil
	}

	return "", fmt.Errorf("no RPC method handler implemented for %s", req.Method)
}

func (be *MockNodeBackend) Handler(w http.ResponseWriter, req *http.Request) {
	if be.HTTPHandlerOverride != nil {
		be.HTTPHandlerOverride(w, req)
		return
	}

	defer req.Body.Close()
	be.LastRawRequest = req
	be.LastJSONRPCRequestTimestamp = time.Now()

	testLog.Debugw("mockserver call", "remoteAddr", req.RemoteAddr, "method", req.Method, "url", req.URL)

	w.Header().Set("Content-Type", "application/json")
	testHeader := req.Header.Get("Test")
	w.Header().Set("Test", testHeader)

	returnError := func(id interface{}, msg string) {
		testLog.Debug("MockNodeBackend: returnError", "msg", msg)
		res := JSONRPCResponse{
			ID: id,
			Error: &JSONRPCError{
				Code:    -32603,
				Message: msg,
			},
		}

		if err := json.NewEncoder(w).Encode(res); err != nil {
			testLog.Debug("MockNodeBackend: error writing returnError response", "error", err, "response", res)
		}
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		returnError(-1, fmt.Sprintf("failed to read request body: %v", err))
		return
	}

	// Parse JSON RPC
	jsonReq := new(JSONRPCRequest)
	if err = json.Unmarshal(body, &jsonReq); err != nil {
		returnError(-1, fmt.Sprintf("failed to parse JSON RPC request: %v", err))
		return
	}

	rawRes, err := be.handleRPCRequest(jsonReq)
	if err != nil {
		returnError(jsonReq.ID, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	resBytes, err := json.Marshal(rawRes)
	if err != nil {
		fmt.Println("error mashalling rawRes:", rawRes, err)
	}

	res := NewJSONRPCResponse(jsonReq.ID, resBytes)

	// Write to client request
	if err := json.NewEncoder(w).Encode(res); err != nil {
		testLog.Error("error writing response", "error", err, "data", rawRes)
	}
}
