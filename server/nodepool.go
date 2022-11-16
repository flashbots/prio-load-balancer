package server

import (
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type NodePool struct {
	log               *zap.SugaredLogger
	nodes             []*Node
	nodesLock         sync.Mutex
	redisState        *RedisState
	numWorkersPerNode int32
	JobC              chan *SimRequest
}

func NewNodePool(log *zap.SugaredLogger, redisState *RedisState, numWorkersPerNode int32) *NodePool {
	return &NodePool{
		log:               log,
		redisState:        redisState,
		numWorkersPerNode: numWorkersPerNode,
		JobC:              make(chan *SimRequest, JobChannelBuffer),
	}
}

func (gp *NodePool) LoadNodesFromRedis() error {
	if gp.redisState == nil {
		return nil
	}
	nodeUris, err := gp.redisState.GetNodes()
	if err != nil {
		return errors.Wrap(err, "loading nodes from redis failed")
	}
	gp.log.Infow("NodePool: loaded nodes from redis", "numNodes", len(nodeUris))

	// Create the nodes now
	for _, uri := range nodeUris {
		_, _, err = gp._addNode(uri)
		if err != nil {
			return errors.Wrap(err, "adding node from redis failed")
		}
	}
	return nil
}

// HasNode returns true if a node with the URI is already in the pool
func (gp *NodePool) HasNode(uri string) bool {
	for _, node := range gp.nodes {
		if node.URI == uri {
			return true
		}
	}
	return false
}

// AddNode adds a node to the pool and starts the workers. If a new node is added, the list of nodes is saved to redis.
func (gp *NodePool) AddNode(uri string) error {
	added, nodeUris, err := gp._addNode(uri)
	if err != nil {
		return errors.Wrap(err, "AddNode failed")
	}

	if added {
		err = gp._saveNodeListToRedis(nodeUris)
		if err != nil {
			gp.log.Errorw("NodePool AddNode: added but failed saving to redis", "URI", uri, "error", err)
		} else {
			gp.log.Debugw("NodePool AddNode: added and saved to redis", "URI", uri, "numNodes", len(gp.nodes))
		}
	}

	return err
}

// _addNode adds a node to the pool and starts the workers. If a new node is added, it also returns nodeUris to be saved to redis.
func (gp *NodePool) _addNode(uri string) (added bool, nodeUris []string, err error) {
	gp.nodesLock.Lock()
	defer gp.nodesLock.Unlock()

	if gp.HasNode(uri) {
		return false, nil, nil
	}

	node := NewNode(gp.log, uri, gp.JobC, gp.numWorkersPerNode)
	err = node.HealthCheck()
	if err != nil {
		return false, nil, errors.Wrap(err, "_addNode healthcheck failed")
	}

	// Add now
	gp.nodes = append(gp.nodes, node)
	nodeUris = []string{}
	for _, node := range gp.nodes {
		nodeUris = append(nodeUris, node.URI)
	}

	// Start node workers
	node.StartWorkers()
	gp.log.Infow("NodePool: added node", "URI", uri, "numNodes", len(gp.nodes))
	return true, nodeUris, nil
}

func (gp *NodePool) _saveNodeListToRedis(nodeUris []string) error {
	if gp.redisState == nil {
		return nil
	}

	return gp.redisState.SaveNodes(nodeUris)
}

func (gp *NodePool) DelNode(uri string) (deleted bool, err error) {
	for idx, node := range gp.nodes {
		if node.URI == uri {
			node.StopWorkers()

			gp.nodesLock.Lock()
			defer gp.nodesLock.Unlock()

			// Remove node
			gp.nodes = append(gp.nodes[:idx], gp.nodes[idx+1:]...)

			// Save new list of nodes to redis
			nodeUris := []string{}
			for _, node := range gp.nodes {
				nodeUris = append(nodeUris, node.URI)
			}
			err = gp._saveNodeListToRedis(nodeUris)
			return true, err
		}
	}
	return false, nil
}

func (gp *NodePool) NodeUris() []string {
	gp.nodesLock.Lock()
	defer gp.nodesLock.Unlock()

	nodeUris := []string{}
	for _, node := range gp.nodes {
		nodeUris = append(nodeUris, node.URI)
	}
	return nodeUris
}

// Shutdown will stop all node workers, but let's them finish the ongoing connections
func (gp *NodePool) Shutdown() {
	for _, node := range gp.nodes {
		node.StopWorkersAndWait()
	}
}
