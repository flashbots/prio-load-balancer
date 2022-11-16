package testutils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMockServer(t *testing.T) {
	mockNodeBackend := NewMockNodeBackend()
	mockNodeServer := httptest.NewServer(http.HandlerFunc(mockNodeBackend.Handler))

	resp, err := http.PostForm(mockNodeServer.URL, nil)
	require.Nil(t, err, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	mockNodeBackend.Reset()
	mockNodeBackend.HTTPHandlerOverride = func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "error", 479)
	}

	resp, err = http.PostForm(mockNodeServer.URL, nil)
	require.Nil(t, err, err)
	require.Equal(t, 479, resp.StatusCode)
}
