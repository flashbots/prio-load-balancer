// Package server is the webserver which sends simulation requests to the simulator.
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"strings"
	"time"

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
	r.HandleFunc("/sim", s.HandleQueueRequest).Methods(http.MethodPost)
	r.HandleFunc("/nodes", s.HandleNodesRequest).Methods(http.MethodGet, http.MethodPost, http.MethodDelete)

	if EnablePprof {
		s.log.Info("Enabling pprof")
		r.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
	}

	if EnableErrorTestAPI {
		s.log.Info("Enabling error testing API")
		r.HandleFunc("/debug/testLogLevels", s.HandleTestLogLevels).Methods(http.MethodGet)
	}

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

func getMsIntoSlot(slot uint64) int64 {
	if slot == 0 {
		return 0
	}
	slotStartTimestamp := int64(GenesisTime) + (int64(slot) * int64(SecPerSlot))
	return time.Now().UTC().UnixMilli() - (slotStartTimestamp * 1000)
}

func (s *Webserver) HandleQueueRequest(w http.ResponseWriter, req *http.Request) {
	log := s.log
	startTime := time.Now().UTC()
	defer req.Body.Close()

	// Allow `X-Request-Slot:...` header, which prints slot and ms into slot in logs
	var reqSlot uint64
	reqSlotStr := req.Header.Get("X-Request-Slot")
	if reqSlotStr != "" {
		reqSlot, _ = strconv.ParseUint(reqSlotStr, 10, 64)
		log = log.With("slot", reqSlot)
	}

	// Allow single `X-Request-ID:...` header, which will be logged as `reqID`
	reqID := req.Header.Get("X-Request-ID")
	if reqID != "" {
		log = s.log.With("reqID", reqID)
	}

	// Read the body and start processing
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
		log.Infow("client closed the connection before processing", "err", ctx.Err())
		return
	}

	// Add new sim request to queue
	isFastTrack := req.Header.Get("X-Fast-Track") == "true"
	isHighPrio := req.Header.Get("high_prio") == "true" || req.Header.Get("X-High-Priority") == "true"
	simReq := NewSimRequest(reqID, body, isHighPrio, isFastTrack)
	wasAdded := s.prioQueue.Push(simReq)
	if !wasAdded { // queue was full, job not added
		log.Error("Couldn't add request, queue is full")
		http.Error(w, "queue full", http.StatusInternalServerError)
		return
	}

	lenFastTrack, lenHighPrio, lenLowPrio := s.prioQueue.Len()
	log.Infow("Request added to queue. prioQueue size:", "requestIsHighPrio", isHighPrio, "requestIsFastTrack", isFastTrack, "fastTrack", lenFastTrack, "highPrio", lenHighPrio, "lowPrio", lenLowPrio, "msIntoSlot", getMsIntoSlot(reqSlot))

	// Wait for response or cancel
	for {
		select {
		case <-ctx.Done(): // if user closes connection, cancel the simreq
			log.Infow("client closed the connection prematurely", "err", ctx.Err(), "queueItems", s.prioQueue.NumRequests(), "payloadSize", len(body), "requestTries", simReq.Tries, "requestCancelled", simReq.Cancelled, "msIntoSlot", getMsIntoSlot(reqSlot))
			if ctx.Err() != nil {
				simReq.Cancelled = true
			}
			return
		case resp := <-simReq.ResponseC:
			if resp.Error != nil {
				log.Infow("HandleSim error", "err", resp.Error, "try", simReq.Tries, "shouldRetry", resp.ShouldRetry, "nodeURI", resp.NodeURI, "msIntoSlot", getMsIntoSlot(reqSlot))
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

			lenFastTrack, lenHighPrio, lenLowPrio := s.prioQueue.Len()
			log.Infow("Request completed",
				"durationMs", time.Since(startTime).Milliseconds(),
				"durationUs", time.Since(startTime).Microseconds(),
				"simDurationUs", resp.SimDuration.Microseconds(),
				"requestIsHighPrio", isHighPrio,
				"requestIsFastTrack", isFastTrack,
				"payloadSize", len(body),
				"statusCode", resp.StatusCode,
				"nodeURI", resp.NodeURI,
				"requestTries", simReq.Tries,
				"queueItems", s.prioQueue.NumRequests(),
				"queueItemsFastTrack", lenFastTrack,
				"queueItemsHighPrio", lenHighPrio,
				"queueItemsLowPrio", lenLowPrio,
				"msIntoSlot", getMsIntoSlot(reqSlot),
			)
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

// HandleTestLogLevels is used for testing error logging, to verify for operations. Is opt-in with `ENABLE_ERROR_TEST_API=1`
func (s *Webserver) HandleTestLogLevels(w http.ResponseWriter, req *http.Request) {
	s.log.Debug("debug")
	s.log.Infow("info", "key", "value")
	s.log.Warnw("warn", "key", "value")
	s.log.Errorw("error", "key", "value")
	// s.log.Fatalw("fatal", "key", "value")
	// s.log.Panicw("panic", "key", "value")
	panic("panic")
	// w.WriteHeader(http.StatusOK)
}
