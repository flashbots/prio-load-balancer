package server

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type ServerOpts struct {
	Log            *zap.SugaredLogger
	HTTPAddrPtr    string // listen address for the webserver
	RedisURI       string // (optional) URI for the redis instance. If empty then don't use Redis.
	WorkersPerNode int32  // Number of concurrent workers per execution node
}

// Server is the overall load balancer server
type Server struct {
	log       *zap.SugaredLogger
	opts      ServerOpts
	redis     *RedisState
	prioQueue *PrioQueue
	nodePool  *NodePool
	webserver *Webserver
}

// NewServer creates a new Server instance, loads the nodes from Redis and starts the node workers
func NewServer(opts ServerOpts) (*Server, error) {
	var err error
	s := Server{
		opts:      opts,
		log:       opts.Log,
		prioQueue: NewPrioQueue(MaxQueueItemsFastTrack, MaxQueueItemsHighPrio, MaxQueueItemsLowPrio),
	}

	if s.opts.RedisURI == "" {
		s.log.Info("Not using Redis because no RedisURI provided")
	} else {
		s.log.Infow("Connecting to Redis", "URI", s.opts.RedisURI)
		s.redis, err = NewRedisState(s.opts.RedisURI)
		if err != nil {
			return nil, err
		}
	}

	if opts.WorkersPerNode == 0 {
		s.log.Warn("WorkersPerNode is 0! This is not recommended. Use at least 1.")
	}

	s.nodePool = NewNodePool(s.log, s.redis, s.opts.WorkersPerNode)
	err = s.nodePool.LoadNodesFromRedis()
	if err != nil {
		return nil, err
	}

	return &s, nil
}

// Start starts the webserver and the main loop (pumping jobs from the queue to the workers)
func (s *Server) Start() {
	// Setup and start the webserver
	s.log.Infow("Starting webserver", "listenAddr", s.opts.HTTPAddrPtr)
	s.webserver = NewWebserver(s.log, s.opts.HTTPAddrPtr, s.prioQueue, s.nodePool)
	s.webserver.Start()

	// Main loop: send simqueue jobs to node pool
	s.log.Info("Starting main loop")
	for {
		r := s.prioQueue.Pop()
		if r == nil { // Shutdown (queue.Close() was called)
			s.log.Info("Shutting down main loop (request is nil)")
			return
		}

		if r.Cancelled {
			continue
		}

		if time.Since(r.CreatedAt) > RequestTimeout {
			s.log.Info("request timed out before processing")
			r.SendResponse(SimResponse{Error: ErrRequestTimeout})
			continue
		}

		// Return an error if no nodes are available
		if len(s.nodePool.nodes) == 0 {
			s.log.Error("no execution nodes available")
			r.SendResponse(SimResponse{Error: ErrNoNodesAvailable})
			continue
		}

		// Forward to a node for processing
		select {
		case s.nodePool.JobC <- r:
			// Job was taken by a node
		case <-time.After(ServerJobSendTimeout):
			// Job was NOT taken by a node - cancel request
			s.log.Warnw("job was not taken by a node", "requestsInQueue", s.prioQueue.NumRequests())
			r.SendResponse(SimResponse{Error: ErrNodeTimeout})
		}
	}
}

// Shutdown gracefully shuts down the server. Allows ongoing requests to complete, but no
// further requests will be accepted or those from the queue processed.
func (s *Server) Shutdown() {
	s.log.Info("Shutting down server")
	s.prioQueue.Close()
	s.webserver.srv.Shutdown(context.Background()) // stop incoming requests
	s.nodePool.Shutdown()                          // stop the execution workers
}

// AddNode adds a new execution node to the pool and starts the workers. If a new node is added,
// the list of nodes is saved to redis.
func (s *Server) AddNode(uri string) error {
	return s.nodePool.AddNode(uri)
}

// NumNodeWorkersAlive returns the number of currently active node workers
func (s *Server) NumNodeWorkersAlive() int {
	res := 0
	for _, n := range s.nodePool.nodes {
		res += int(n.curWorkers)
	}
	return res
}

func (s *Server) QueueSize() (lenFastTrack, lenHighPrio, lenLowPrio int) {
	return s.prioQueue.Len()
}
