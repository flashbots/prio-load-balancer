package server

import "time"

type SimRequest struct {
	// can be none of, or one of high-prio / fast-track
	ID          string
	IsHighPrio  bool
	IsFastTrack bool

	Payload   []byte
	ResponseC chan SimResponse
	Cancelled bool
	CreatedAt time.Time
	Tries     int
}

func NewSimRequest(id string, payload []byte, isHighPrio, IsFastTrack bool) *SimRequest {
	return &SimRequest{
		ID:          id,
		Payload:     payload,
		IsHighPrio:  isHighPrio,
		IsFastTrack: IsFastTrack,
		ResponseC:   make(chan SimResponse, 1),
		CreatedAt:   time.Now().UTC(),
	}
}

// SendResponse sends the response to ResponseC. If noone is listening on the channel, it is dropped.
func (r *SimRequest) SendResponse(resp SimResponse) (wasSent bool) {
	select {
	case r.ResponseC <- resp:
		return true
	default:
		return false
	}
}

type SimResponse struct {
	StatusCode  int
	Payload     []byte
	Error       error
	ShouldRetry bool // When response has an error, whether it should be retried
	NodeURI     string
	SimDuration time.Duration
}
