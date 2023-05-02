//go:build tee
// +build tee

package server

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
	"fmt"

	"go.uber.org/zap"

	ratls "github.com/konvera/gramine-ratls-golang"

	"github.com/konvera/geth-sev/constellation/atls"
	"github.com/konvera/geth-sev/constellation/attestation/azure/snp"
	"github.com/konvera/geth-sev/constellation/config"
)

func init() {
	err := ratls.InitRATLSLib(true, time.Hour, false)
	if err != nil {
		panic(err)
	}
}

type attestationLogger struct {
	log           *zap.SugaredLogger
}


func (w attestationLogger) Infof(format string, args ...any) {
	w.log.Infow(fmt.Sprintf(format, args...))
}

func (w attestationLogger) Warnf(format string, args ...any) {
	w.log.Warnw(fmt.Sprintf(format, args...))
}

func NewNode(log *zap.SugaredLogger, uri string, jobC chan *SimRequest, numWorkers int32) (*Node, error) {
	client := http.Client{}
	pURL, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, err
	}
	username := pURL.User.Username()

	// SGX TLS config
	if strings.HasPrefix(username, "SGX_") {
		mrenclave, err := hex.DecodeString(strings.TrimPrefix(username, "SGX_"))
		if err != nil {
			return nil, err
		}
		verifyConnection := func(cs tls.ConnectionState) error {
			err := ratls.RATLSVerifyDer(cs.PeerCertificates[0].Raw, mrenclave, nil, nil, nil)
			return err
		}
		client = http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
					VerifyConnection:   verifyConnection,
				},
			},
		}
	// SEV TLS config
	} else if strings.HasPrefix(username, "SEV_") {
		gzmeasurements, err := base64.URLEncoding.DecodeString(strings.TrimPrefix(username, "SEV_"))
		if err != nil {
			return nil, err
		}

		gzreader, err := gzip.NewReader(bytes.NewReader(gzmeasurements));
		if(err != nil){
			return nil, err
		}

		measurements, err := ioutil.ReadAll(gzreader);
		if(err != nil){
			return nil, err
		}

		attConfig := config.DefaultForAzureSEVSNP()
		err = json.Unmarshal(measurements, &attConfig.Measurements)
		if err != nil {
			return nil, err
		}

		validators := []atls.Validator{ snp.NewValidator(attConfig, attestationLogger{log}) }
		tlsConfig, err := atls.CreateAttestationClientTLSConfig(nil, validators)
		if err != nil {
			return nil, err
		}
		client = http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
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
	}
	return node, nil
}
