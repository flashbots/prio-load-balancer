package server

import (
	"os"
	"time"

	"go.uber.org/zap"
)

var (
	JobChannelBuffer = GetEnvInt("JOB_CHAN_BUFFER", 2)          // buffer for JobC in backends (for transporting jobs from server -> backend node)
	RequestMaxTries  = GetEnvInt("RETRIES_MAX", 3)              // 3 tries means it will be retried 2 additional times, and on third error would fail
	PayloadMaxBytes  = GetEnvInt("PAYLOAD_MAX_KB", 8192) * 1024 // Max payload size in bytes. If a payload sent to the webserver is larger, it returns "400 Bad Request".

	MaxQueueItemsFastTrack = GetEnvInt("ITEMS_FASTTRACK_MAX", 0) // Max number of items in fast-track queue. 0 means no limit.
	MaxQueueItemsHighPrio  = GetEnvInt("ITEMS_HIGHPRIO_MAX", 0)  // Max number of items in high-prio queue. 0 means no limit.
	MaxQueueItemsLowPrio   = GetEnvInt("ITEMS_LOWPRIO_MAX", 0)   // Max number of items in low-prio queue. 0 means no limit.

	// How often fast-track queue items should be popped before popping a high-priority item
	FastTrackPerHighPrio = GetEnvInt("ITEMS_FASTTRACK_PER_HIGHPRIO", 2)
	FastTrackDrainFirst  = os.Getenv("FASTTRACK_DRAIN_FIRST") == "1" // whether to fully drain the fast-track queue first

	RequestTimeout       = time.Duration(GetEnvInt("REQUEST_TIMEOUT", 5)) * time.Second       // Time between creation and receive in the node worker, after which a SimRequest will not be processed anymore
	ServerJobSendTimeout = time.Duration(GetEnvInt("JOB_SEND_TIMEOUT", 2)) * time.Second      // How long the server tries to send a job into the nodepool for processing
	ProxyRequestTimeout  = time.Duration(GetEnvInt("REQUEST_PROXY_TIMEOUT", 3)) * time.Second // HTTP request timeout for proxy requests to the backend node

	RedisPrefix        = GetEnv("REDIS_PREFIX", "prio-load-balancer:") // All redis keys will be prefixed with this
	EnableErrorTestAPI = os.Getenv("ENABLE_ERROR_TEST_API") == "1"     // will enable /debug/testLogLevels which prints errors and ends with a panic (also enabled if mock-node is used)
	EnablePprof        = os.Getenv("ENABLE_PPROF") == "1"              // will enable /debug/pprof

	ProxyMaxIdleConns        = GetEnvInt("ProxyMaxIdleConns", 100)
	ProxyMaxConnsPerHost     = GetEnvInt("ProxyMaxConnsPerHost", 100)
	ProxyMaxIdleConnsPerHost = GetEnvInt("ProxyMaxIdleConnsPerHost", 100)
	ProxyIdleConnTimeout     = time.Duration(GetEnvInt("ProxyIdleConnTimeout", 90)) * time.Second
)

func LogConfig(log *zap.SugaredLogger) {
	log.Infow("config",
		"JobChannelBuffer", JobChannelBuffer,
		"RequestMaxTries", RequestMaxTries,
		"MaxQueueItemsHighPrio", MaxQueueItemsHighPrio,
		"MaxQueueItemsLowPrio", MaxQueueItemsLowPrio,
		"FastTrackPerHighPrio", FastTrackPerHighPrio,
		"FastTrackDrainFirst", FastTrackDrainFirst,
		"PayloadMaxBytes", PayloadMaxBytes,
		"RequestTimeout", RequestTimeout,
		"ServerJobSendTimeout", ServerJobSendTimeout,
		"ProxyRequestTimeout", ProxyRequestTimeout,
		"RedisPrefix", RedisPrefix,
		"EnableErrorTestAPI", EnableErrorTestAPI,
		"EnablePprof", EnablePprof,
		"ProxyMaxIdleConns", ProxyMaxIdleConns,
		"ProxyMaxConnsPerHost", ProxyMaxConnsPerHost,
		"ProxyMaxIdleConnsPerHost", ProxyMaxIdleConnsPerHost,
		"ProxyIdleConnTimeout", ProxyIdleConnTimeout,
	)
}
