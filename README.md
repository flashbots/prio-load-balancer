# Transparent JSON-RPC proxy and load balancer

[![Goreport status](https://goreportcard.com/badge/github.com/flashbots/prio-load-balancer)](https://goreportcard.com/report/github.com/flashbots/prio-load-balancer)
[![Test status](https://github.com/flashbots/prio-load-balancer/workflows/Checks/badge.svg)](https://github.com/flashbots/prio-load-balancer/actions?query=workflow%3A%22Checks%22)
[![Docker hub](https://badgen.net/docker/size/flashbots/prio-load-balancer?icon=docker&label=image)](https://hub.docker.com/r/flashbots/prio-load-balancer/tags)

**With priority queues, retries, good logging and metrics, and even SGX/SEV attestation support.**

Queues:

1. low-prio
2. high-prio
3. fast-track

Queueing:

- All high-prio requests will be proxied before any of the low-prio queue
- [N](https://github.com/flashbots/prio-load-balancer/blob/main/server/consts.go#L20) fast-tracked requests get processed for every 1 high-prio request

Further notes:

- A _node_ represents one JSON-RPC endpoint (i.e. geth instance)
- Each node spins up N workers, which proxy requests concurrently to the execution endpoint
- You can add/remove nodes through a JSON API without restarting the server
- Each node starts the default number of workers, but you can also specify a custom number of workers by adding `?_workers=` to the node URL
- It's possible to tweak [a few knobs](/server/consts.go)
<!-- - The load balancer exposes a HTTP API for managing nodes, and uses Redis as a source of truth for configured nodes (i.e. the cli node config only sets the initial state in redis, but a restart won't override the node setup created through the HTTP API. -->

---

### Application structure and request flow

![App structure and request flow](https://user-images.githubusercontent.com/116939/202170917-bcd98c98-f40e-4025-8084-06adec27ff96.png)

### Example logs

At the end of a request:

```json
{
    "level": "info",
    "ts": 1685122704.4079978,
    "caller": "server/webserver.go:154",
    "msg": "Request completed",
    "service": "validation-queue",
    "durationMs": 144,
    "requestIsHighPrio": true,
    "requestIsFastTrack": false,
    "payloadSize": 209394,
    "statusCode": 200,
    "nodeURI": "http://validation-1.internal:8545",
    "requestTries": 1,
    "queueItems": 11,
    "queueItemsFastTrack": 0,
    "queueItemsHighPrio": 7,
    "queueItemsLowPrio": 4
}
```

Full request cycle:

```json
// request getting added to the queue
{"level":"info","ts":1685126514.8569882,"caller":"server/webserver.go:112","msg":"Request added to queue. prioQueue size:","service":"validation-queue","requestIsHighPrio":true,"requestIsFastTrack":true,"fastTrack":1,"highPrio":0,"lowPrio":0}

// completed request
{"level":"info","ts":1685126514.9174724,"caller":"server/webserver.go:154","msg":"Request completed","service":"validation-queue","durationMs":78,"requestIsHighPrio":true,"requestIsFastTrack":true,"payloadSize":121291,"statusCode":200,"nodeURI":"http://validation-2.internal:8545","requestTries":1,"queueItems":0,"queueItemsFastTrack":0,"queueItemsHighPrio":0,"queueItemsLowPrio":0}

// http server logs
{"level":"info","ts":1685126514.9175296,"caller":"server/http_logger.go:54","msg":"http: POST /sim 200","service":"validation-queue","status":200,"method":"POST","path":"/sim","duration":0.078877451}
```

---

## Getting started

Docker images are available at https://hub.docker.com/r/flashbots/prio-load-balancer

#### Run the program

```bash
# Run with a mock execution backend and debug output
go run . -mock-node           # text logging
go run . -mock-node -log-prod # json logging

# low-prio queue request
curl -d '{"jsonrpc":"2.0","method":"eth_callBundle","params":[],"id":1}' localhost:8080

# high-prio queue request
curl -H 'X-High-Priority: true' -d '{"jsonrpc":"2.0","method":"eth_callBundle","params":[],"id":1}' localhost:8080

# fast-track queue request
curl -H 'X-Fast-Track: true' -d '{"jsonrpc":"2.0","method":"eth_callBundle","params":[],"id":1}' localhost:8080

# adding a custom request ID
curl -H 'X-Request-ID: yourLogID' -d '{"jsonrpc":"2.0","method":"eth_callBundle","params":[],"id":1}' localhost:8080

# Get execution nodes
curl localhost:8080/nodes

# Add a execution node
curl -d '{"uri":"http://foo"}' localhost:8080/nodes

# Add a execution node with custom number of workers
curl -d '{"uri":"http://foo?_workers=8"}' localhost:8080/nodes

# Remove a execution node
curl -X DELETE -d '{"uri":"http://foo"}' localhost:8080/nodes
curl -X DELETE -d '{"uri":"http://localhost:8095"}' localhost:8080/nodes
```

Note: there's a bunch of constants that can be configured with env vars in [server/consts.go](server/consts.go).

#### Node selection

* Redis is used as source of truth for which execution nodes to use.
* If you restart with a different set of configured nodes (i.e. in env vars), the previous nodes will still be in Redis and still be used by the load balancer.
* See the commands in the readme above on how to get the nodes it uses, and how to add/remove nodes.

#### Test, lint, build

```bash
# lint & staticcheck (staticcheck.io)
make lint

# run tests
make test

# test coverage
make cover
make cover-html

# build
make build
```

#### Node TEE attestation via TLS
```
# build prio-load-balancer with SGX and SEV support
make build-tee
```

> **IMPORTANT:** SGX and SEV attestation support requires additional dependencies. See [Dockerfile.tee](Dockerfile.tee) for details.

#### SEV Node aTLS attestation

```
# base64 encode the VM measurements

MEASUREMENTS=$(cat << EOF | gzip | basenc --base64url -w0
{
  "1": {
    "expected": "3d458cfe55cc03ea1f443f1562beec8df51c75e14a9fcf9a7234a13f198e7969",
    "warnOnly": true
  },
  "2": {
    "expected": "3d458cfe55cc03ea1f443f1562beec8df51c75e14a9fcf9a7234a13f198e7969",
    "warnOnly": true
  },
  "3": {
    "expected": "3d458cfe55cc03ea1f443f1562beec8df51c75e14a9fcf9a7234a13f198e7969",
    "warnOnly": true
  },
  "4": {
    "expected": "82736cdd6b4f3c718bf969b545eaaa6eb3f1e6d229ad9712e6a4ddf431418ab7",
    "warnOnly": false
  },
  "5": {
    "expected": "54c04bcd7cf8adadafee915bf325f92d958050c14e086c1e180258113d376c1a",
    "warnOnly": true
  },
  "6": {
    "expected": "9319868ef4dad6a79117f14b9ac1870ccf5f9d178b39a3fd84e6230fa93a7993",
    "warnOnly": true
  },
  "7": {
    "expected": "32fe42b385b47cb22c906b8a7e4f134e9f2270818f90e94072d1101ef72f1c00",
    "warnOnly": true
  },
  "8": {
    "expected": "0000000000000000000000000000000000000000000000000000000000000000",
    "warnOnly": false
  },
  "9": {
    "expected": "0000000000000000000000000000000000000000000000000000000000000000",
    "warnOnly": false
  },
  "11": {
    "expected": "0000000000000000000000000000000000000000000000000000000000000000",
    "warnOnly": false
  },
  "12": {
    "expected": "f1a142c53586e7e2223ec74e5f4d1a4942956b1fd9ac78fafcdf85117aa345da",
    "warnOnly": false
  },
  "13": {
    "expected": "0000000000000000000000000000000000000000000000000000000000000000",
    "warnOnly": false
  },
  "14": {
    "expected": "e3991b7ddd47be7e92726a832d6874c5349b52b789fa0db8b558c69fea29574e",
    "warnOnly": true
  },
  "15": {
    "expected": "0000000000000000000000000000000000000000000000000000000000000000",
    "warnOnly": false
  }
}
EOF
)
```

```
# Add the SEV execution node
curl -d "{\"uri\":\"https://SEV_${MEASUREMENTS}@foo\"}" localhost:8080/nodes
```

Execution nodes running within SEV and providing attestation consumables via constellations aTLS implementation are supported. The aTLS certificate of the execution node is automatically attested with the VM measurements which are submitted as part of the user part of the **node URI** (`SEV_<gzipped, base64url encoded measurements>`). You can read more about the attestation measurements in the [constellation docs](https://docs.edgeless.systems/constellation/architecture/attestation#runtime-measurements)

#### SGX Node RA-TLS attestation
```
# Add an SGX execution node
curl -d '{"uri":"https://SGX_<MRENCLAVE>@foo"}' localhost:8080/nodes
```

Execution nodes running within SGX and providing attestation consumables via RA-TLS are supported. The RA-TLS certificate of the execution node is automatically attested with the `MRENCLAVE` which is submitted as part of the user part of **node URI** (`SGX_<MRENCLAVE>`).

---

## Queue Benchmarks

```
goarch: amd64
pkg: github.com/flashbots/prio-load-balancer/server
cpu: Intel(R) Core(TM) i9-8950HK CPU @ 2.90GHz

1 worker, 10k tasks:
BenchmarkPrioQueue-12    	    	    2338	    492219 ns/op	  298109 B/op	      34 allocs/op

5 workers, 10k tasks:
BenchmarkPrioQueueMultiReader-12    	    2690	    596315 ns/op	  292507 B/op	      50 allocs/op

5 workers, 100k tasks:
BenchmarkPrioQueueMultiReader-12    	     261	   4637403 ns/op	 4245243 B/op	      66 allocs/op
```

---

## Todo

Possibly

* Configurable redis prefix, to allow multiple sim-lbs per redis instance
* Execution-node health checks (currently not implemented)

---

## Maintainers

- [@metachris](https://twitter.com/metachris)

---

## License

The code in this project is free software under the [MIT License](LICENSE).


