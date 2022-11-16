package testutils

import (
	"encoding/json"
	"fmt"
)

type JSONRPCRequest struct {
	ID      interface{}   `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Version string        `json:"jsonrpc,omitempty"`
}

func NewJSONRPCRequest(id interface{}, method string, params []interface{}) *JSONRPCRequest {
	return &JSONRPCRequest{
		ID:      id,
		Method:  method,
		Params:  params,
		Version: "2.0",
	}
}

func NewJSONRPCRequest1(id interface{}, method string, param interface{}) *JSONRPCRequest {
	return NewJSONRPCRequest(id, method, []interface{}{param})
}

type JSONRPCResponse struct {
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	Version string          `json:"jsonrpc"`
}

func NewJSONRPCResponse(id interface{}, result json.RawMessage) *JSONRPCResponse {
	return &JSONRPCResponse{
		ID:      id,
		Result:  result,
		Version: "2.0",
	}
}

// JSONRPCError as per the spec: https://www.jsonrpc.org/specification#error_object
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (err JSONRPCError) Error() string {
	return fmt.Sprintf("Error %d (%s)", err.Code, err.Message)
}
