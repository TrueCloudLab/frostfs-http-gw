# NeoFS HTTP Gate

NeoFS HTTP Gate is example of tool that provides basic interactions with NeoFS.
You can download files from NeoFS Network using NeoFS Gate. 

## Install

```go get -u github.com/nspcc-dev/neofs-http-gate```

## Configuration

```
# Flags
      --pprof                      enable pprof
      --metrics                    enable prometheus
  -h, --help                       show help
  -v, --version                    show version
      --key string                 "generated" to generate key, path to private key file, hex string or wif (default "generated")
      --verbose                    debug gRPC connections
      --request_timeout duration   gRPC request timeout (default 5s)
      --connect_timeout duration   gRPC connect timeout (default 30s)
      --listen_address string      HTTP Gate listen address (default "0.0.0.0:8082")
  -p, --peers stringArray          NeoFS nodes

# Environments:

HTTP_GW_KEY=string                           - "generated" to generate key, path to private key file, hex string or wif (default "generated")
HTTP_GW_CONNECT_TIMEOUT=Duration             - timeout for connection
HTTP_GW_REQUEST_TIMEOUT=Duration             - timeout for request
HTTP_GW_REBALANCE_TIMER=Duration             - time between connections checks
HTTP_GW_LISTEN_ADDRESS=host:port             - address to listen connections
HTTP_GW_PEERS_<X>_ADDRESS=host:port          - address of NeoFS Node
HTTP_GW_PEERS_<X>_WEIGHT=float               - weight of NeoFS Node
HTTP_GW_PPROF=bool                           - enable/disable pprof (/debug/pprof)
HTTP_GW_METRICS=bool                         - enable/disable prometheus metrics endpoint (/metrics)
HTTP_GW_LOGGER_FORMAT=string                 - logger format
HTTP_GW_LOGGER_LEVEL=string                  - logger level
HTTP_GW_LOGGER_NO_CALLER=bool                - logger don't show caller
HTTP_GW_LOGGER_NO_DISCLAIMER=bool            - logger don't show application name/version
HTTP_GW_LOGGER_SAMPLING_INITIAL=int          - logger sampling initial
HTTP_GW_LOGGER_SAMPLING_THEREAFTER=int       - logger sampling thereafter
HTTP_GW_LOGGER_TRACE_LEVEL=string            - logger show trace on level
HTTP_GW_KEEPALIVE_TIME=Duration              - аfter a duration of this time if the client doesn't see any activity
it pings the server to see if the transport is still alive. 
HTTP_GW_KEEPALIVE_TIMEOUT=Duration           - after having pinged for keepalive check, the client waits for a duration
of Timeout and if no activity is seen even after that the connection is closed
HTTP_GW_KEEPALIVE_PERMIT_WITHOUT_STREAM=Bool - if true, client sends keepalive pings even with no active RPCs.
If false, when there are no active RPCs, Time and Timeout will be ignored and no keepalive pings will be sent.

Peers preset:

HTTP_GW_PEERS_[N]_ADDRESS = string
HTTP_GW_PEERS_[N]_WEIGHT = 0..1 (float)
```