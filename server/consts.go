package server

import (
	"time"

	"go.uber.org/zap"
)

var (
	JobChannelBuffer      = GetEnvInt("JOB_CHAN_BUFFER", 2)          // buffer for JobC in backends (for transporting jobs from server -> backend node)
	RequestMaxTries       = GetEnvInt("RETRIES_MAX", 3)              // 3 tries means it will be retried 2 additional times, and on third error would fail
	MaxQueueItemsHighPrio = GetEnvInt("ITEMS_HIGHPRIO_MAX", 0)       // Max number of items in high-prio queue. 0 means no limit.
	MaxQueueItemsLowPrio  = GetEnvInt("ITEMS_LOWPRIO_MAX", 0)        // Max number of items in low-prio queue. 0 means no limit.
	PayloadMaxBytes       = GetEnvInt("PAYLOAD_MAX_KB", 2048) * 1024 // Max payload size in bytes. If a payload sent to the webserver is larger, it returns "400 Bad Request".

	RequestTimeout       = time.Duration(GetEnvInt("REQUEST_TIMEOUT", 5)) * time.Second       // Time between creation and receive in the node worker, after which a SimRequest will not be processed anymore
	ServerJobSendTimeout = time.Duration(GetEnvInt("JOB_SEND_TIMEOUT", 2)) * time.Second      // How long the server tries to send a job into the nodepool for processing
	ProxyRequestTimeout  = time.Duration(GetEnvInt("REQUEST_PROXY_TIMEOUT", 3)) * time.Second // HTTP request timeout for proxy requests to the backend node
)

func LogConfig(log *zap.SugaredLogger) {
	log.Infow("config",
		"JobChannelBuffer", JobChannelBuffer,
		"RequestMaxTries", RequestMaxTries,
		"MaxQueueItemsHighPrio", MaxQueueItemsHighPrio,
		"MaxQueueItemsLowPrio", MaxQueueItemsLowPrio,
		"PayloadMaxBytes", PayloadMaxBytes,
		"RequestTimeout", RequestTimeout,
		"ServerJobSendTimeout", ServerJobSendTimeout,
		"ProxyRequestTimeout", ProxyRequestTimeout,
	)
}
