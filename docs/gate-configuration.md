# FrostFS HTTP Gateway configuration file

This section contains detailed FrostFS HTTP Gateway configuration file description
including default config values and some tips to set up configurable values.

There are some custom types used for brevity:

* `duration` -- string consisting of a number and a suffix. Suffix examples include `s` (seconds), `m` (minutes), `ms` (
  milliseconds).


# Reload on SIGHUP

Some config values can be reloaded on SIGHUP signal.
Such parameters have special mark in tables below.

You can send SIGHUP signal to app using the following command:

```shell
$ kill -s SIGHUP <app_pid>
```

Example:

```shell
$ ./bin/frostfs-http-gw --config config.yaml  &> http.log &
[1] 998346

$ cat http.log
# ...
2022-10-03T09:37:25.826+0300    info    frostfs-http-gw/app.go:332        starting application    {"app_name": "frostfs-http-gw", "version": "v0.24.0"}
# ...

$ kill -s SIGHUP 998346

$ cat http.log
# ...
2022-10-03T09:38:16.205+0300    info    frostfs-http-gw/app.go:470        SIGHUP config reload completed
```

# Structure

| Section         | Description                                           |
|-----------------|-------------------------------------------------------|
| no section      | [General parameters](#general-section)                |
| `wallet`        | [Wallet configuration](#wallet-section)               |
| `peers`         | [Nodes configuration](#peers-section)                 |
| `logger`        | [Logger configuration](#logger-section)               |
| `web`           | [Web configuration](#web-section)                     |
| `server`        | [Server configuration](#server-section)               |
| `upload-header` | [Upload header configuration](#upload-header-section) |
| `zip`           | [ZIP configuration](#zip-section)                     |
| `pprof`         | [Pprof configuration](#pprof-section)                 |
| `prometheus`    | [Prometheus configuration](#prometheus-section)       |


# General section

```yaml
rpc_endpoint: http://morph-chain.frostfs.devenv:30333
resolve_order:
  - nns
  - dns

connect_timeout: 5s 
stream_timeout: 10s
request_timeout: 5s 
rebalance_timer: 30s
pool_error_threshold: 100
```

| Parameter              | Type       | SIGHUP reload | Default value  | Description                                                                        |
|------------------------|------------|---------------|----------------|------------------------------------------------------------------------------------|
| `rpc_endpoint`         | `string`   | yes           |                | The address of the RPC host to which the gateway connects to resolve bucket names. |
| `resolve_order`        | `[]string` | yes           | `[nns, dns]`   | Order of bucket name resolvers to use.                                             |
| `connect_timeout`      | `duration` |               | `10s`          | Timeout to connect to a node.                                                      |
| `stream_timeout`       | `duration` |               | `10s`          | Timeout for individual operations in streaming RPC.                                |
| `request_timeout`      | `duration` |               | `15s`          | Timeout to check node health during rebalance.                                     |
| `rebalance_timer`      | `duration` |               | `60s`          | Interval to check node health.                                                     |
| `pool_error_threshold` | `uint32`   |               | `100`          | The number of errors on connection after which node is considered as unhealthy.    |

# `wallet` section

```yaml
wallet:
  path: /path/to/wallet.json 
  address: NfgHwwTi3wHAS8aFAN243C5vGbkYDpqLHP 
  passphrase: pwd
```

| Parameter    | Type     | Default value | Description                                                              |
|--------------|----------|---------------|--------------------------------------------------------------------------|
| `path`       | `string` |               | Path to the wallet.                                                      |
| `address`    | `string` |               | Account address to get from wallet. If omitted default one will be used. |
| `passphrase` | `string` |               | Passphrase to decrypt wallet.                                            |

# `peers` section

```yaml
# Nodes configuration
# This configuration makes the gateway use the first node (node1.frostfs:8080)
# while it's healthy. Otherwise, gateway uses the second node (node2.frostfs:8080)
# for 10% of requests and the third node (node3.frostfs:8080) for 90% of requests.
# Until nodes with the same priority level are healthy
# nodes with other priority are not used.
# The lower the value, the higher the priority.
peers:
  0:
    address: node1.frostfs:8080
    priority: 1
    weight: 1
  1:
    address: node2.frostfs:8080
    priority: 2
    weight: 0.1
  2:
    address: node3.frostfs:8080
    priority: 2
    weight: 0.9
```

| Parameter  | Type     | Default value | Description                                                                                                                                             |
|------------|----------|---------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| `address`  | `string` |               | Address of storage node.                                                                                                                                |
| `priority` | `int`    | `1`           | It allows to group nodes and don't switch group until all nodes with the same priority will be unhealthy. The lower the value, the higher the priority. |
| `weight`   | `float`  | `1`           | Weight of node in the group with the same priority. Distribute requests to nodes proportionally to these values.                                        |

# `server` section

You can specify several listeners for server. For example, for `http` and `https`.

```yaml
server:
  - address: 0.0.0.0:8080
    tls:
      enabled: false
      cert_file: /path/to/cert
      key_file: /path/to/key
  - address: 0.0.0.0:8081
    tls:
      enabled: true
      cert_file: /path/to/another/cert
      key_file: /path/to/another/key
```

| Parameter       | Type     | SIGHUP reload | Default value  | Description                                   |
|-----------------|----------|---------------|----------------|-----------------------------------------------|
| `address`       | `string` |               | `0.0.0.0:8080` | The address that the gateway is listening on. |
| `tls.enabled`   | `bool`   |               | false          | Enable TLS or not.                            |
| `tls.cert_file` | `string` | yes           |                | Path to the TLS certificate.                  |
| `tls.key_file`  | `string` | yes           |                | Path to the key.                              |


# `logger` section

```yaml
logger:
  level: debug
```

| Parameter | Type     | SIGHUP reload | Default value | Description                                                                                        |
|-----------|----------|---------------|---------------|----------------------------------------------------------------------------------------------------|
| `level`   | `string` | yes           | `debug`       | Logging level.<br/>Possible values:  `debug`, `info`, `warn`, `error`, `dpanic`, `panic`, `fatal`. |


# `web` section

```yaml
web:
  read_buffer_size: 4096
  write_buffer_size: 4096
  read_timeout: 10m
  write_timeout: 5m
  stream_request_body: true
  max_request_body_size: 4194304
```

| Parameter               | Type       | Default value | Description                                                                                                                                                                                              |
|-------------------------|------------|---------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `read_buffer_size`      | `int`      | `4096`        | Per-connection buffer size for requests' reading. This also limits the maximum header size.                                                                                                              |
| `write_buffer_size`     | `int`      | `4096`        | Per-connection buffer size for responses' writing.                                                                                                                                                       |
| `read_timeout`          | `duration` | `10m`         | The amount of time allowed to read the full request including body. The connection's read deadline is reset when the connection opens, or for keep-alive connections after the first byte has been read. |
| `write_timeout`         | `duration` | `5m`          | The maximum duration before timing out writes of the response. It is reset after the request handler has returned.                                                                                       |
| `stream_request_body`   | `bool`     | `true`        | Enables request body streaming, and calls the handler sooner when given body is larger than the current limit.                                                                                           |
| `max_request_body_size` | `int`      | `4194304`     | Maximum request body size. The server rejects requests with bodies exceeding this limit.                                                                                                                 |


# `upload-header` section

```yaml
upload_header:
  use_default_timestamp: false
```

| Parameter               | Type   | SIGHUP reload | Default value | Description                                                 |
|-------------------------|--------|---------------|---------------|-------------------------------------------------------------|
| `use_default_timestamp` | `bool` | yes           | `false`       | Create timestamp for object if it isn't provided by header. |


# `zip` section

```yaml
zip:
  compression: false 
```

| Parameter     | Type   | SIGHUP reload | Default value | Description                                                  |
|---------------|--------|---------------|---------------|--------------------------------------------------------------|
| `compression` | `bool` | yes           | `false`       | Enable zip compression when download files by common prefix. |


# `pprof` section

Contains configuration for the `pprof` profiler.

```yaml
pprof:
  enabled: true
  address: localhost:8083
```

| Parameter | Type     | SIGHUP reload | Default value    | Description                             |
|-----------|----------|---------------|------------------|-----------------------------------------|
| `enabled` | `bool`   | yes           | `false`          | Flag to enable the service.             |
| `address` | `string` | yes           | `localhost:8083` | Address that service listener binds to. |

# `prometheus` section

Contains configuration for the `prometheus` metrics service.

```yaml
prometheus:
  enabled: true
  address: localhost:8084
```

| Parameter | Type     | SIGHUP reload | Default value    | Description                             |
|-----------|----------|---------------|------------------|-----------------------------------------|
| `enabled` | `bool`   | yes           | `false`          | Flag to enable the service.             |
| `address` | `string` | yes           | `localhost:8084` | Address that service listener binds to. |
