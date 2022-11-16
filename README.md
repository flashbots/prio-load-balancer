# High/Low Prio Load Balancer

[![Test status](https://github.com/flashbots/prio-load-balancer/workflows/Checks/badge.svg)](https://github.com/flashbots/prio-load-balancer/actions?query=workflow%3A%22Checks%22)

**Transparent jsonrpc/http proxy and load balancer with high and low priority queue and retries.**

In the current setup, all requests in the high priority queue will be proxied before any of the low-prio queue.

---

**App structure and request flow:**

![App structure and request flow](https://user-images.githubusercontent.com/116939/202170917-bcd98c98-f40e-4025-8084-06adec27ff96.png)

---

## Getting started

#### Run the program

```bash
# Run with a mock execution backend and debug output
go run . -mock-node

# add request for low-prio queue
curl -d '{"jsonrpc":"2.0","method":"eth_callBundle","params":[],"id":1}' localhost:8080

# add request for high-prio queue
curl -H 'X-High-Priority' -d '{"jsonrpc":"2.0","method":"eth_callBundle","params":[],"id":1}' localhost:8080

# Get execution nodes
curl localhost:8080/nodes

# Add a execution node
curl -d '{"uri":"http://foo"}' localhost:8080/nodes

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

* Currently it works for jsonrpc requests. To make it work for any http request it would need to also proxy the headers and the URL.
* Queue rules: i.e. for 10 high-prio items, process 1 low-prio item
* Configurable redis prefix, to allow multiple sim-lbs per redis instance
* Execution-node health checks (currently not implemented)
* DoS protection / rate limiting (i.e. per IP)

---

## Maintainers

- [@metachris](https://twitter.com/metachris)

---

## License

The code in this project is free software under the [MIT License](LICENSE).
