//go:build sgx
// +build sgx

package server

import (
	"crypto/tls"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	ratls "github.com/konvera/gramine-ratls-golang"
)

func NewNode(log *zap.SugaredLogger, uri string, jobC chan *SimRequest, numWorkers int32) (*Node, error) {
	client := http.Client{}
	enclave := false
	pURL, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, err
	}
	username := pURL.User.Username()
	if strings.HasPrefix(username, "SGX_") {
		enclave = true
		mrenclave, err := hex.DecodeString(strings.TrimPrefix(username, "SGX_"))
		if err != nil {
			return nil, err
		}
		verifyConnection := func(cs tls.ConnectionState) error {
			return ratls.RATLSVerifyDer(cs.PeerCertificates[0].Raw, mrenclave, nil, nil, nil)
		}
		client = http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
					VerifyConnection:   verifyConnection,
				},
			},
		}
	}

	node := &Node{
		log:        log,
		URI:        uri,
		AddedAt:    time.Now(),
		jobC:       jobC,
		numWorkers: numWorkers,
		client:     &client,
		enclave:    enclave,
	}
	return node, nil
}