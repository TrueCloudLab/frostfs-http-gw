<p align="center">
<img src="./.github/logo.svg" width="500px" alt="FrostFS">
</p>
<p align="center">
  <a href="https://frostfs.info">FrostFS</a> is a decentralized distributed object storage integrated with the <a href="https://neo.org">NEO Blockchain</a>.
</p>

---
[![Report](https://goreportcard.com/badge/github.com/TrueCloudLab/frostfs-http-gw)](https://goreportcard.com/report/github.com/TrueCloudLab/frostfs-http-gw)
![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/TrueCloudLab/frostfs-http-gw?sort=semver)
![License](https://img.shields.io/github/license/TrueCloudLab/frostfs-http-gw.svg?style=popout)

# FrostFS HTTP Gateway

FrostFS HTTP Gateway bridges FrostFS internal protocol and HTTP standard.
- you can download one file per request from the FrostFS Network
- you can upload one file per request into the FrostFS Network

See available routes in [specification](./docs/api.md).

## Installation

```go install github.com/TrueCloudLab/frostfs-http-gw```

Or you can call `make` to build it from the cloned repository (the binary will
end up in `bin/frostfs-http-gw`). To build frostfs-http-gw binary in clean docker
environment, call `make docker/bin/frostfs-http-gw`.

Other notable make targets:

```
dep          Check and ensure dependencies
image        Build clean docker image
dirty-image  Build dirty docker image with host-built binaries
fmt          Format the code
lint         Run linters
version      Show current version
```

Or you can also use a [Docker
image](https://hub.docker.com/r/nspccdev/neofs-http-gw) provided for the released
(and occasionally unreleased) versions of the gateway (`:latest` points to the
latest stable release).

## Execution

HTTP gateway itself is not a FrostFS node, so to access FrostFS it uses node's
gRPC interface and you need to provide some node that it will connect to. This
can be done either via `-p` parameter or via `HTTP_GW_PEERS_<N>_ADDRESS` and
`HTTP_GW_PEERS_<N>_WEIGHT` environment variables (the gate supports multiple
FrostFS nodes with weighted load balancing).

If you launch HTTP gateway in bundle with [frostfs-dev-env](https://github.com/TrueCloudLab/frostfs-dev-env), 
you can get the IP address of the node in the output of `make hosts` command 
(with s0*.neofs.devenv name).

These two commands are functionally equivalent, they run the gate with one
backend node (and otherwise default settings):
```
$ frostfs-http-gw -p 192.168.130.72:8080
$ HTTP_GW_PEERS_0_ADDRESS=192.168.130.72:8080 frostfs-http-gw
```
It's also possible to specify uri scheme (grpc or grpcs) when using `-p`:
```
$ frostfs-http-gw -p grpc://192.168.130.72:8080
$ HTTP_GW_PEERS_0_ADDRESS=grpcs://192.168.130.72:8080 frostfs-http-gw
```

## Configuration

In general, everything available as CLI parameter can also be specified via
environment variables (see [example](./config/config.env)), so they're not specifically mentioned in most cases
(see `--help` also). If you prefer a config file you can use it in yaml format.

### Nodes: weights and priorities

You can specify multiple `-p` options to add more FrostFS nodes, this will make
gateway spread requests equally among them (using weight 1 and priority 1 for every node):

```
$ frostfs-http-gw -p 192.168.130.72:8080 -p 192.168.130.71:8080
```
If you want some specific load distribution proportions, use weights and priorities:

```
$ HTTP_GW_PEERS_0_ADDRESS=192.168.130.71:8080 HTTP_GW_PEERS_0_WEIGHT=1 HTTP_GW_PEERS_0_PRIORITY=1 \
  HTTP_GW_PEERS_1_ADDRESS=192.168.130.72:8080 HTTP_GW_PEERS_1_WEIGHT=9 HTTP_GW_PEERS_1_PRIORITY=2 \
  HTTP_GW_PEERS_2_ADDRESS=192.168.130.73:8080 HTTP_GW_PEERS_2_WEIGHT=1 HTTP_GW_PEERS_2_PRIORITY=2 \
  frostfs-http-gw
```
This command will make gateway use 192.168.130.71 while it is healthy. Otherwise, it will make the gateway use 
192.168.130.72 for 90% of requests and 192.168.130.73 for remaining 10%.

### Keys
You can provide a wallet via `--wallet` or `-w` flag. You can also specify the account address using `--address` 
(if no address provided default one will be used). If wallet is used, you need to set `HTTP_GW_WALLET_PASSPHRASE` variable to decrypt the wallet. 
If no wallet provided, the gateway autogenerates a key pair it will use for FrostFS requests.
```
$ frostfs-http-gw -p $NEOFS_NODE -w $WALLET_PATH --address $ACCOUNT_ADDRESS
```
Example:
```
$ frostfs-http-gw -p 192.168.130.72:8080 -w wallet.json --address NfgHwwTi3wHAS8aFAN243C5vGbkYDpqLHP
```

### Binding and TLS

You can make the gateway listen on specific address using the `--listen_address` option.

It can also provide TLS interface for its users, just specify paths to the key and
certificate files via `--tls_key` and `--tls_certificate` parameters. Note
that using these options makes gateway TLS-only. If you need to serve both TLS
and plain text HTTP, you either have to run two gateway instances or use some
external redirecting solution.

Example to bind to `192.168.130.130:443` and serve TLS there:

```
$ frostfs-http-gw -p 192.168.130.72:8080 --listen_address 192.168.130.130:443 \
  --tls_key=key.pem --tls_certificate=cert.pem
```

### HTTP parameters

You can tune HTTP read and write buffer sizes as well as timeouts with
`HTTP_GW_WEB_READ_BUFFER_SIZE`, `HTTP_GW_WEB_READ_TIMEOUT`,
`HTTP_GW_WEB_WRITE_BUFFER_SIZE` and `HTTP_GW_WEB_WRITE_TIMEOUT` environment
variables.

**Note:** to allow upload and download of big data streams, disable read
and write timeouts correspondingly. To do that, set `HTTP_GW_WEB_READ_TIMEOUT=0`
and `HTTP_GW_WEB_WRITE_TIMEOUT=0`. Otherwise, HTTP Gateway will terminate
request with data stream after timeout.

`HTTP_GW_WEB_STREAM_REQUEST_BODY` environment variable can be used to disable
request body streaming (effectively it'll make the gateway accept the file completely
first and only then try sending it to FrostFS).

`HTTP_GW_WEB_MAX_REQUEST_BODY_SIZE` controls maximum request body size
limiting uploads to files slightly lower than this limit.

### FrostFS parameters

Gateway can automatically set timestamps for uploaded files based on local
time source, use `HTTP_GW_UPLOAD_HEADER_USE_DEFAULT_TIMESTAMP` environment
variable to control this behavior.

### Monitoring and metrics

Pprof and Prometheus are integrated into the gateway. To enable them use `--pprof` and `--metrics` flags or
`HTTP_GW_PPROF`/`HTTP_GW_METRICS` environment variables.

### Timeouts

You can tune gRPC interface parameters with `--connect_timeout` (for
connection to a node) and `--request_timeout` (for request processing over
established connection) options.

gRPC-level checks allow the gateway to detect dead peers, but it declares them
unhealthy at pool level once per `--rebalance_timer` interval, so check for it
if needed.

All timing options accept values with suffixes, so "15s" is 15 seconds and
"2m" is 2 minutes.

### Zip streaming
The gateway supports downloading files by common prefix (like dir) in zip format. You can enable compression 
using config or `HTTP_GW_ZIP_COMPRESSION=true` environment variable.

### Logging
You can specify logging level using variable:
```
HTTP_GW_LOGGER_LEVEL=debug
```

### Yaml file
Configuration file is optional and can be used instead of environment variables/other parameters. 
It can be specified with `--config` parameter:
```
$ frostfs-http-gw --config your-config.yaml
```

See [config](./config/config.yaml) and [defaults](./docs/gate-configuration.md) for example.

## HTTP API provided

This gateway intentionally provides limited feature set and doesn't try to
substitute (or completely wrap) regular gRPC FrostFS interface. You can download
and upload objects with it, but deleting, searching, managing ACLs, creating
containers and other activities are not supported and not planned to be
supported.

### Preparation

Before uploading or downloading a file make sure you have a prepared container. 
You can create it with instructions below.

Also, in case of downloading, you need to have a file inside a container.

### NNS

In all download/upload routes you can use container name instead of its id (`$CID`).

Steps to start using name resolving:

1. Enable NNS resolving in config (`rpc_endpoint` must be a valid neo rpc node, see [configs](./config) for other examples):

```yaml
rpc_endpoint: http://morph-chain.neofs.devenv:30333
resolve_order:
  - nns
```

2. Make sure your container is registered in NNS contract. If you use [frostfs-dev-env](https://github.com/TrueCloudLab/frostfs-dev-env) 
you can check if your container (e.g. with `container-name` name) is registered in NNS:

```shell
$ curl -s --data '{"id":1,"jsonrpc":"2.0","method":"getcontractstate","params":[1]}' \
    http://morph-chain.neofs.devenv:30333 | jq -r '.result.hash'
    
0x8e6c3cd4b976b28e84a3788f6ea9e2676c15d667

$ docker exec -it morph_chain neo-go \
    contract testinvokefunction \
    -r http://morph-chain.neofs.devenv:30333 0x8e6c3cd4b976b28e84a3788f6ea9e2676c15d667 \
    resolve string:container-name.container int:16 \
    | jq -r '.stack[0].value | if type=="array" then .[0].value else . end' \
    | base64 -d && echo
    
7f3vvkw4iTiS5ZZbu5BQXEmJtETWbi3uUjLNaSs29xrL
```

3. Use container name instead of its `$CID`. For example:

```shell
$ curl http://localhost:8082/get_by_attribute/container-name/FileName/object-name
```

#### Create a container

You can create a container via [frostfs-cli](https://github.com/TrueCloudLab/frostfs-node/releases):
```
$ frostfs-cli -r $NEOFS_NODE -w $WALLET container create --policy $POLICY --basic-acl $ACL
```
where `$WALLET` is a path to user wallet,   
`$ACL` -- hex encoded basic ACL value or keywords 'private, 'public-read', 'public-read-write' and  
`$POLICY` -- QL-encoded or JSON-encoded placement policy or path to file with it

For example:
```
$ frostfs-cli -r 192.168.130.72:8080 -w ./wallet.json container create --policy "REP 3" --basic-acl public --await
```

If you have launched nodes via [frostfs-dev-env](https://github.com/TrueCloudLab/frostfs-dev-env),
you can get the key value from `wallets/wallet.json` or write the path to 
the file `wallets/wallet.key`.

#### Prepare a file in a container

To create a file via [frostfs-cli](https://github.com/TrueCloudLab/frostfs-node/releases), run a command below:
```
$ frostfs-cli -r $NEOFS_NODE -k $KEY object put --file $FILENAME --cid $CID 
```
where 
`$KEY` -- the key, please read the information [above](#create-a-container), 
`$CID` -- container ID.

For example:
```
$ frostfs-cli -r 192.168.130.72:8080 -w ./wallet.json object put --file cat.png --cid Dxhf4PNprrJHWWTG5RGLdfLkJiSQ3AQqit1MSnEPRkDZ --attributes img_type=cat,my_attr=cute
```


### Downloading

#### Requests

The following requests support GET/HEAD methods. 

##### By IDs

Basic downloading involves container ID and object ID and is done via GET
requests to `/get/$CID/$OID` path, where `$CID` is a container ID or its name if NNS is enabled, 
`$OID` is an object's (i.e. your file's) ID. 

For example:

```shell
$ wget http://localhost:8082/get/Dxhf4PNprrJHWWTG5RGLdfLkJiSQ3AQqit1MSnEPRkDZ/2m8PtaoricLouCn5zE8hAFr3gZEBDCZFe9BEgVJTSocY
```

or if container has a name:

```shell
$ wget http://localhost:8082/get/container-name/2m8PtaoricLouCn5zE8hAFr3gZEBDCZFe9BEgVJTSocY
```

##### By attributes
There is also more complex interface provided for attribute-based downloads,
it's usually used to retrieve files by their names, but any other attribute
can be used as well. The generic syntax for it looks like this:

```/get_by_attribute/$CID/$ATTRIBUTE_NAME/$ATTRIBUTE_VALUE```

where 
`$CID` is a container ID or its name if NNS is enabled, 
`$ATTRIBUTE_NAME` is the name of the attribute we want to use,
`$ATTRIBUTE_VALUE` is the value of this attribute that the target object should have.

**NB!** The attribute key and value should be url encoded, i.e., if you want to download an object with the attribute value 
`a cat`, the value in the request must be `a+cat`. In the same way with the attribute key. If you don't escape such values
everything can still work (for example you can use `d@ta` without encoding) but it's HIGHLY RECOMMENDED to encode all your attributes.  

If multiple objects have specified attribute with specified value, then the
first one of them is returned (and you can't get others via this interface).

Example for file name attribute:

```
$ wget http://localhost:8082/get_by_attribute/88GdaZFTcYJn1dqiSECss8kKPmmun6d6BfvC4zhwfLYM/FileName/cat.jpeg
```
Or when the filename includes special symbols:
```
$ wget http://localhost:8082/get_by_attribute/88GdaZFTcYJn1dqiSECss8kKPmmun6d6BfvC4zhwfLYM/FileName/cat+jpeg # means 'cat jpeg'
$ wget http://localhost:8082/get_by_attribute/88GdaZFTcYJn1dqiSECss8kKPmmun6d6BfvC4zhwfLYM/FileName/cat%25jpeg # means 'cat%jpeg'
```

Some other user-defined attributes:

```
$ wget http://localhost:8082/get_by_attribute/Dxhf4PNprrJHWWTG5RGLdfLkJiSQ3AQqit1MSnEPRkDZ/Ololo/100500
```

Or when the attribute includes special symbols: 
```
$ wget http://localhost:8082/get_by_attribute/Dxhf4PNprrJHWWTG5RGLdfLkJiSQ3AQqit1MSnEPRkDZ/Olo%2Blo/100500 # means Olo+lo
```

An optional `download=true` argument for `Content-Disposition` management is
also supported (more on that below):

```
$ wget http://localhost:8082/get/Dxhf4PNprrJHWWTG5RGLdfLkJiSQ3AQqit1MSnEPRkDZ/2m8PtaoricLouCn5zE8hAFr3gZEBDCZFe9BEgVJTSocY?download=true

```

##### Zip
You can download some dir (files with the same prefix) in zip (it will be compressed if config contains appropriate param):
```
$ wget http://localhost:8082/zip/Dxhf4PNprrJHWWTG5RGLdfLkJiSQ3AQqit1MSnEPRkDZ/common/prefix
```

**Note:** the objects must have a valid `FilePath` attribute (it should not contain trailing `/`), 
otherwise they will not be in the zip archive. You can upload file with this attribute using `curl`:

```
$ curl -F 'file=@cat.jpeg;filename=cat.jpeg' -H 'X-Attribute-FilePath: common/prefix/cat.jpeg' http://localhost:8082/upload/Dxhf4PNprrJHWWTG5RGLdfLkJiSQ3AQqit1MSnEPRkDZ
```


#### Replies

You get object contents in the reply body (if GET method was used), but at the same time you also get a
set of reply headers generated using the following rules:
 * `Content-Length` is set to the length of the object
 * `Content-Type` is autodetected dynamically by gateway
 * `Content-Disposition` is `inline` for regular requests and `attachment` for
   requests with `download=true` argument, `filename` is also added if there
   is `FileName` attribute set for this object
 * `Last-Modified` header is set to `Timestamp` attribute value if it's
   present for the object
 * `x-container-id` contains container ID
 * `x-object-id` contains object ID
 * `x-owner-id` contains owner address
 * all the other FrostFS attributes are converted to `X-Attribute-*` headers (but only
   if they can be safely represented in HTTP header), for example `FileName`
   attribute becomes `X-Attribute-FileName` header

##### Caching strategy

HTTP Gateway doesn't control caching (doesn't anything with the `Cache-Control` header). Caching strategy strictly 
depends on application use case. So it should be carefully done by proxy server.

### Uploading

You can POST files to `/upload/$CID` path where `$CID` is a container ID or its name if NNS is enabled. The
request must contain multipart form with mandatory `filename` parameter. Only
one part in multipart form will be processed, so to upload another file just
issue a new POST request.

Example request:

```
$ curl -F 'file=@cat.jpeg;filename=cat.jpeg' http://localhost:8082/upload/Dxhf4PNprrJHWWTG5RGLdfLkJiSQ3AQqit1MSnEPRkDZ
```

Chunked encoding is supported by the server (but check for request read
timeouts if you're planning some streaming). You can try streaming support
with a large file piped through named FIFO pipe:

```
$ mkfifo pipe
$ cat video.mp4 > pipe &
$ curl --no-buffer -F 'file=@pipe;filename=catvideo.mp4' http://localhost:8082/upload/Dxhf4PNprrJHWWTG5RGLdfLkJiSQ3AQqit1MSnEPRkDZ
```

You can also add some attributes to your file using the following rules:
 * all "X-Attribute-*" headers get converted to object attributes with
   "X-Attribute-" prefix stripped, that is if you add "X-Attribute-Ololo:
   100500" header to your request the resulting object will get "Ololo:
   100500" attribute
 * "X-Attribute-NEOFS-*" headers are special 
   (`-NEOFS-` part can also be `-neofs-` or`-Neofs-`), they're used to set internal
   NeoFS attributes starting with `__NEOFS__` prefix, for these attributes all
   dashes get converted to underscores and all letters are capitalized. For
   example, you can use "X-Attribute-NEOFS-Expiration-Epoch" header to set
   `__NEOFS__EXPIRATION_EPOCH` attribute
 * `FileName` attribute is set from multipart's `filename` if not set
   explicitly via `X-Attribute-FileName` header
 * `Timestamp` attribute can be set using gateway local time if using
   HTTP_GW_UPLOAD_HEADER_USE_DEFAULT_TIMESTAMP option and if request doesn't
   provide `X-Attribute-Timestamp` header of its own

---
**NOTE**

There are some reserved headers type of `X-Attribute-NEOFS-*` (headers are arranged in descending order of priority):
1. `X-Attribute-Neofs-Expiration-Epoch: 100`
2. `X-Attribute-Neofs-Expiration-Duration: 24h30m`
3. `X-Attribute-Neofs-Expiration-Timestamp: 1637574797`
4. `X-Attribute-Neofs-Expiration-RFC3339: 2021-11-22T09:55:49Z`

which transforms to `X-Attribute-Neofs-Expiration-Epoch`. So you can provide expiration any convenient way. 

---

For successful uploads you get JSON data in reply body with a container and
object ID, like this:
```
{
        "object_id": "9ANhbry2ryjJY1NZbcjryJMRXG5uGNKd73kD3V1sVFsX",
        "container_id": "Dxhf4PNprrJHWWTG5RGLdfLkJiSQ3AQqit1MSnEPRkDZ"
}
```

#### Authentication

You can always upload files to public containers (open for anyone to put
objects into), but for restricted containers you need to explicitly allow PUT
operations for a request signed with your HTTP Gateway keys.

If your don't want to manage gateway's secret keys and adjust eACL rules when
gateway configuration changes (new gate, key rotation, etc) or you plan to use
public services, there is an option to let your application backend (or you) to
issue Bearer Tokens ans pass them from the client via gate down to FrostFS level
to grant access.

FrostFS Bearer Token basically is a container owner-signed ACL data (refer to FrostFS
documentation for more details). There are two options to pass them to gateway:
 * "Authorization" header with "Bearer" type and base64-encoded token in
   credentials field
 * "Bearer" cookie with base64-encoded token contents

For example, you have a mobile application frontend with a backend part storing
data in FrostFS. When a user authorizes in the mobile app, the backend issues a FrostFS
Bearer token and provides it to the frontend. Then, the mobile app may generate
some data and upload it via any available FrostFS HTTP Gateway by adding
the corresponding header to the upload request. Accessing the ACL protected data
works the same way.

##### Example
In order to generate a bearer token, you need to know the container owner key and 
the address of the sender who will do the request to FrostFS (in our case, it's a gateway wallet address).

Suppose we have:
* **KxDgvEKzgSBPPfuVfw67oPQBSjidEiqTHURKSDL1R7yGaGYAeYnr** (container owner key)
* **NhVtreTTCoqsMQV5Wp55fqnriiUCpEaKm3** (token owner address)
* **BJeErH9MWmf52VsR1mLWKkgF3pRm3FkubYxM7TZkBP4K** (container id)

Firstly, we need to encode the container id and the sender address to base64 (now it's base58). 
So use **base58** and **base64** utils.

1. Encoding container id:
```
$ echo 'BJeErH9MWmf52VsR1mLWKkgF3pRm3FkubYxM7TZkBP4K' | base58 --decode | base64
# output: mRnZWzewzxjzIPa7Fqlfqdl3TM1KpJ0YnsXsEhafJJg=
```

2. Encoding token owner id:
```
$ echo 'NhVtreTTCoqsMQV5Wp55fqnriiUCpEaKm3' | base58 --decode | base64
# output: NezFK4ujidF+X7bB88uzREQzRQeAvdj3Gg==
```

Now, we can form a Bearer token (10000 is liftetime expiration in epoch) and save it to **bearer.json**:
```
{
    "body": {
        "eaclTable": {
            "version": {
                "major": 0,
                "minor": 0
            },
            "containerID": {
                "value": "mRnZWzewzxjzIPa7Fqlfqdl3TM1KpJ0YnsXsEhafJJg="
            },
            "records": []
        },
        "ownerID": {
            "value": "NezFK4ujidF+X7bB88uzREQzRQeAvdj3Gg=="
        },
        "lifetime": {
            "exp": "10000",
            "nbf": "0",
            "iat": "0"
        }
    },
    "signature": null
}
```

Next, sign it with the container owner key:
```
$ frostfs-cli util sign bearer-token --from bearer.json --to signed.json -w ./wallet.json
```
Encoding to base64 to use via the header:
```
$ base64 -w 0 signed.json 
# output: Ck4KKgoECAIQBhIiCiCZGdlbN7DPGPMg9rsWqV+p2XdMzUqknRiexewSFp8kmBIbChk17MUri6OJ0X5ftsHzy7NERDNFB4C92PcaGgMIkE4SZgohAxpsb7vfAso1F0X6hrm6WpRS14WsT3/Ct1SMoqRsT89KEkEEGxKi8GjKSf52YqhppgaOTQHbUsL3jn7SHLqS3ndAQ7NtAATnmRHleZw2V2xRRSRBQdjDC05KK83LhdSax72Fsw==
```

After that, the Bearer token can be used:

```
$ curl -F 'file=@cat.jpeg;filename=cat.jpeg' -H "Authorization: Bearer Ck4KKgoECAIQBhIiCiCZGdlbN7DPGPMg9rsWqV+p2XdMzUqknRiexewSFp8kmBIbChk17MUri6OJ0X5ftsHzy7NERDNFB4C92PcaGgMIkE4SZgohAxpsb7vfAso1F0X6hrm6WpRS14WsT3/Ct1SMoqRsT89KEkEEGxKi8GjKSf52YqhppgaOTQHbUsL3jn7SHLqS3ndAQ7NtAATnmRHleZw2V2xRRSRBQdjDC05KK83LhdSax72Fsw==" \
  http://localhost:8082/upload/BJeErH9MWmf52VsR1mLWKkgF3pRm3FkubYxM7TZkBP4K
# output:
# {
#	"object_id": "DhfES9nVrFksxGDD2jQLunGADfrXExxNwqXbDafyBn9X",
#	"container_id": "BJeErH9MWmf52VsR1mLWKkgF3pRm3FkubYxM7TZkBP4K"
# }
```

##### Note
For the token to work correctly, you need to create a container with a basic ACL that:
1. Allow PUT operation to others
2. Doesn't set "final" bit

For example:
```
$ frostfs-cli -w ./wallet.json --basic-acl 0x0FFFCFFF -r 192.168.130.72:8080 container create --policy "REP 3"  --await
```

To deny access to a container without a token, set the eACL rules:
```
$ frostfs-cli -w ./wallet.json -r 192.168.130.72:8080 container set-eacl --table eacl.json --await --cid BJeErH9MWmf52VsR1mLWKkgF3pRm3FkubYxM7TZkBP4K
```

File **eacl.json**:
```
{
    "version": {
        "major": 0,
        "minor": 0
    },
    "containerID": {
        "value": "mRnZWzewzxjzIPa7Fqlfqdl3TM1KpJ0YnsXsEhafJJg="
    },
    "records": [
        {
            "operation": "PUT",
            "action": "DENY",
            "filters": [],
            "targets": [
                {
                    "role": "OTHERS",
                    "keys": []
                }
            ]
        }
    ]
}
```

### Metrics and Pprof

If enabled, Prometheus metrics are available at `localhost:8084` endpoint 
and Pprof at `localhost:8083/debug/pprof` by default. Host and port can be configured. 
See [configuration](./docs/gate-configuration.md).

## Credits

Please see [CREDITS](CREDITS.md) for details.
