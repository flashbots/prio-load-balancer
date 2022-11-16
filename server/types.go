package server

import "time"

type SimRequest struct {
	IsHighPrio bool
	Payload    []byte
	ResponseC  chan SimResponse
	Cancelled  bool
	CreatedAt  time.Time
	Tries      int
}

func NewSimRequest(isHighPrio bool, payload []byte) *SimRequest {
	return &SimRequest{
		IsHighPrio: isHighPrio,
		Payload:    payload,
		ResponseC:  make(chan SimResponse, 1),
		CreatedAt:  time.Now(),
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
}
