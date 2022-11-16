package server

import (
	"fmt"
	"sync"

	"go.uber.org/atomic"
)

type PrioQueue struct {
	highPrio []*SimRequest
	lowPrio  []*SimRequest
	cond     *sync.Cond
	closed   atomic.Bool

	maxHighPrio int // max items for high prio queue. 0 means no limit.
	maxLowPrio  int // max items for low prio queue. 0 means no limit.
}

func NewPrioQueue(maxHighPrio, maxLowPrio int) *PrioQueue {
	return &PrioQueue{
		cond:        sync.NewCond(&sync.Mutex{}),
		maxHighPrio: maxHighPrio,
		maxLowPrio:  maxLowPrio,
	}
}

func (q *PrioQueue) Len() (lenHighPrio, lenLowPrio int) {
	return len(q.highPrio), len(q.lowPrio)
}

func (q *PrioQueue) NumRequests() int {
	return len(q.highPrio) + len(q.lowPrio)
}

func (q *PrioQueue) String() string {
	return fmt.Sprintf("PrioQueue: highPrio: %d / lowPrio: %d", len(q.highPrio), len(q.lowPrio))
}

// Push adds a new item to the end of the queue. Returns true if added, false if queue is closed or at max capacity
func (q *PrioQueue) Push(r *SimRequest) bool {
	if q.closed.Load() || r == nil {
		return false
	}

	// If queue limits are set and reached, return false now
	if r.IsHighPrio && q.maxHighPrio > 0 && len(q.highPrio) >= q.maxHighPrio {
		return false
	} else if !r.IsHighPrio && q.maxLowPrio > 0 && len(q.lowPrio) >= q.maxLowPrio {
		return false
	}

	// Wait for the lock
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	// Check if closed in the meantime
	if q.closed.Load() {
		return false
	}

	// Add to the queue
	if r.IsHighPrio {
		q.highPrio = append(q.highPrio, r)
	} else {
		q.lowPrio = append(q.lowPrio, r)
	}

	// Unlock and send signal to a listener
	q.cond.Signal()
	return true
}

// Pop returns the next Bid. If no task in queue, blocks until there is one again. First drains the high-prio queue,
// then the low-prio one. Will return nil only after calling Close() when the queue is empty
func (q *PrioQueue) Pop() (nextReq *SimRequest) {
	// Return nil immediately if queue is closed and empty
	if q.closed.Load() && len(q.highPrio) == 0 && len(q.lowPrio) == 0 {
		return nil
	}

	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	if len(q.highPrio) == 0 && len(q.lowPrio) == 0 {
		if q.closed.Load() {
			return nil
		}

		q.cond.Wait()
	}

	if len(q.highPrio) > 0 {
		nextReq = q.highPrio[0]
		q.highPrio = q.highPrio[1:]
	} else if len(q.lowPrio) > 0 {
		nextReq = q.lowPrio[0]
		q.lowPrio = q.lowPrio[1:]
	}

	// When closed and the last item was taken, signal to CloseAndWait that queue is now empty
	if q.closed.Load() && len(q.highPrio) == 0 && len(q.lowPrio) == 0 {
		q.cond.Broadcast()
	}

	return nextReq
}

// Close disallows adding any new items with Push(), and lets readers using Pop() return nil if queue is empty
func (q *PrioQueue) Close() {
	q.closed.Store(true)
	if q.NumRequests() == 0 {
		q.cond.Broadcast()
	}
}

// CloseAndWait closes the queue and waits until the queue is empty
func (q *PrioQueue) CloseAndWait() {
	q.Close()

	// Wait until queue is empty
	q.cond.L.Lock()
	if q.NumRequests() > 0 {
		q.cond.Wait()
	}
	q.cond.L.Unlock()
}
