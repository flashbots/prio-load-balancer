// Package server is the webserver which sends simulation requests to the simulator.
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type Webserver struct {
	log        *zap.SugaredLogger
	listenAddr string
	prioQueue  *PrioQueue
	nodePool   *NodePool
	srv        *http.Server
}

func NewWebserver(log *zap.SugaredLogger, listenAddr string, prioQueue *PrioQueue, nodePool *NodePool) *Webserver {
	return &Webserver{
		log:        log,
		listenAddr: listenAddr,
		prioQueue:  prioQueue,
		nodePool:   nodePool,
	}
}

func (s *Webserver) Start() {
	r := mux.NewRouter()
	r.HandleFunc("/", s.HandleRootRequest).Methods(http.MethodGet)
	r.HandleFunc("/", s.HandleQueueRequest).Methods(http.MethodPost)
	r.HandleFunc("/nodes", s.HandleNodesRequest).Methods(http.MethodGet, http.MethodPost, http.MethodDelete)
	r.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)

	loggedRouter := LoggingMiddleware(s.log, r)

	s.srv = &http.Server{
		Addr:    s.listenAddr,
		Handler: loggedRouter,
	}

	go func() {
		err := s.srv.ListenAndServe()
		if err == http.ErrServerClosed {
			return
		}
		s.log.Errorw("Webserver error", "err", err)
		panic(err)
	}()
}

func (s *Webserver) HandleRootRequest(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "prio-load-balancer\n")
}

func (s *Webserver) HandleQueueRequest(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(body) > PayloadMaxBytes {
		http.Error(w, "Payload too large", http.StatusBadRequest)
		return
	}

	ctx := req.Context()
	if ctx.Err() != nil {
		s.log.Infow("client closed the connection before processing", "err", ctx.Err())
		return
	}

	// Add new sim request to queue
	isFastTrack := req.Header.Get("X-Fast-Track") == "true"
	isHighPrio := req.Header.Get("high_prio") == "true" || req.Header.Get("X-High-Priority") == "true"
	simReq := NewSimRequest(body, isHighPrio, isFastTrack)
	wasAdded := s.prioQueue.Push(simReq)
	if !wasAdded { // queue was full, job not added
		s.log.Error("Couldn't add request, queue is full")
		http.Error(w, "queue full", http.StatusInternalServerError)
		return
	}

	lenFastTrack, lenHighPrio, lenLowPrio := s.prioQueue.Len()
	s.log.Infow("Request added to queue. prioQueue size:", "requestIsHighPrio", isHighPrio, "requestIsFastTrack", isFastTrack, "fastTrack", lenFastTrack, "highPrio", lenHighPrio, "lowPrio", lenLowPrio)

	// Wait for response or cancel
	for {
		select {
		case <-ctx.Done(): // if user closes connection, cancel the simreq
			s.log.Infow("client closed the connection prematurely", "err", ctx.Err(), "queueItems", s.prioQueue.NumRequests(), "payloadSize", len(body), "requestTries", simReq.Tries, "requestCancelled", simReq.Cancelled)
			if ctx.Err() != nil {
				simReq.Cancelled = true
			}
			return
		case resp := <-simReq.ResponseC:
			if resp.Error != nil {
				s.log.Infow("HandleSim error", "err", resp.Error, "try", simReq.Tries, "shouldRetry", resp.ShouldRetry)
				if simReq.Tries < RequestMaxTries && resp.ShouldRetry {
					s.prioQueue.Push(simReq)
					continue
				}

				if resp.StatusCode == 0 {
					resp.StatusCode = http.StatusInternalServerError
				}

				if len(resp.Payload) > 0 {
					w.WriteHeader(resp.StatusCode)
					w.Write(resp.Payload)
					return
				}

				http.Error(w, strings.Trim(resp.Error.Error(), "\n"), resp.StatusCode)
				return
			}

			if resp.StatusCode == 0 {
				resp.StatusCode = http.StatusOK
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(resp.StatusCode)
			w.Write(resp.Payload)
			return
		}
	}
}

type NodeURIPayload struct {
	URI string `json:"uri"`
}

func (s *Webserver) HandleNodesRequest(w http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(s.nodePool.NodeUris()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	} else if req.Method == "POST" {
		var payload NodeURIPayload
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := s.nodePool.AddNode(payload.URI); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)

	} else if req.Method == "DELETE" {
		var payload NodeURIPayload
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		wasRemoved, err := s.nodePool.DelNode(payload.URI)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if !wasRemoved {
			http.Error(w, "node not found", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
