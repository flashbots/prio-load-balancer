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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/konvera/geth-sev/constellation/atls"
	"github.com/konvera/geth-sev/constellation/attestation/azure/snp"
	"github.com/konvera/geth-sev/constellation/config"
	ratls "github.com/konvera/gramine-ratls-golang"
	"go.uber.org/zap"
)

func init() {
	err := ratls.InitRATLSLib(true, time.Hour, false)
	if err != nil {
		panic(err)
	}
}

type attestationLogger struct {
	log *zap.SugaredLogger
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

	workersArg := url.Query().Get("_workers")
	if workersArg != "" {
		// set numWorkers from query param
		workersInt, err := strconv.Atoi(workersArg)
		if err != nil {
			log.Errorw("Error parsing workers query param", "err", err, "uri", uri)
		} else {
			log.Infow("Using custom number of workers", "workers", workersInt, "uri", uri)
			numWorkers = int32(workersInt)
		}
	}

	if strings.HasPrefix(username, "SGX_") { // SGX TLS config
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
	} else if strings.HasPrefix(username, "SEV_") { // SEV TLS config
		gzmeasurements, err := base64.URLEncoding.DecodeString(strings.TrimPrefix(username, "SEV_"))
		if err != nil {
			return nil, err
		}

		gzreader, err := gzip.NewReader(bytes.NewReader(gzmeasurements))
		if err != nil {
			return nil, err
		}

		measurements, err := ioutil.ReadAll(gzreader)
		if err != nil {
			return nil, err
		}

		attConfig := config.DefaultForAzureSEVSNP()
		err = json.Unmarshal(measurements, &attConfig.Measurements)
		if err != nil {
			return nil, err
		}

		validators := []atls.Validator{snp.NewValidator(attConfig, attestationLogger{log})}
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
