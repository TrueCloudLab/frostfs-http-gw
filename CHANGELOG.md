# Changelog

This document outlines major changes between releases.

## [Unreleased]

## [0.26.0] - 2022-12-28

### Fixed
- ENV config example (#236)

### Added
- Support the `Date` header on upload (#214)
- Available routes specification (#216)
- Mention caching strategy in docs (#215)
- Add error response on attribute duplicates (#221)
- Multiple server listeners (#228)

### Removed
- Deprecated linters (#239)

### Updating from v0.25.1
Make sure your configuration is valid:

If you configure application using environment variables change:
* `HTTP_GW_LISTEN_ADDRESS` -> `HTTP_GW_SERVER_0_ADDRESS`
* `HTTP_GW_TLS_CERT_FILE` -> `HTTP_GW_SERVER_0_TLS_CERT_FILE` (and set `HTTP_GW_SERVER_0_TLS_ENABLED=true`)
* `HTTP_GW_TLS_KEY_FILE` -> `HTTP_GW_SERVER_0_TLS_KEY_FILE` (and set `HTTP_GW_SERVER_0_TLS_ENABLED=true`)

If you configure application using `.yaml` file change:
* `listen_address` -> `server.0.address`
* `tls.cert_file` -> `server.0.tls.cert_file` (and set `server.0.tls.enabled: true`)
* `tls.key_file` -> `server.0.tls.key_file` (and set `server.0.tls.enabled: true`)

## [0.25.1] - 2022-11-30

### Fixed
- Download zip archive when `FilePath` is invalid (#222)
- Only one peer must be healthy to init pool (#233)

### Added
- Debian packaging (#223)
- Timeout for individual operations in streaming RPC (#234)

## [0.25.0] - 2022-10-31

### Added
- Config reloading on SIGHUP (#200, #208)
- Stop pool dial on SIGINT (#212)
- Makefile help (#213)

### Changed
- Update NeoFS error handling (#206)
- GitHub actions updates (#205, #209)
- Unified system attribute format for GET and HEAD (#213)

## [0.24.0] - 2022-09-14

### Fixed
- Fix expiration epoch calculation (#198)
- Fix panic on go1.19 (#188)

### Added
- Exposure of pool metrics (#179, #194)

### Changed
- Help doesn't print empty parameters (#186)
- Update version calculation (#190, #199)
- Update neofs-sdk-go (#196)
- Update go version in CI and docker (#197, #202)

## [0.23.0] - 2022-08-02

### Added
- New param to configure pool error threshold (#184)

### Changed
- Pprof and prometheus metrics configuration (#171)
- Drop GO111MODULES from builds (#182)

### Updating from v0.22.0
1. To enable pprof use `pprof.enabled` instead of `pprof` in config.
   To enable prometheus metrics use `prometheus.enabled` instead of `metrics` in config.
   If you are using the command line flags you can skip this step.

## [0.22.0] - 2022-07-25

### Added
- Default params documentation (#172)
- Health metric (#175)

### Changed
- Version output (#169)
- Updated SDK Version (#178)

## [0.21.0] - 2022-06-20

### Fixed
- Downloading ZIP archive using streaming (#163)

### Added
- New make target to build app in docker (#159)

### Changed
- Increased buffer size for file uploading (#148)
- Updated linter version to v1.46.2 (#161)
- Updated CodeQL version to v2 (#158)


## [0.20.0] - 2022-04-29

### Fixed
- Get rid of data race on server shutdown (#145)
- Improved English in docs and comments (#153)
- Use `FilePath` to download zip (#150)

### Added
- Support container name NNS resolving (#142)

### Changed
- Updated docs (#133, #140)
- Increased default read/write timeouts (#154) 
- Updated SDK (#137, #139)
- Updated go version to 1.17 (#143)
- Improved error messages (#144)

## [0.19.0] - 2022-03-16

### Fixed
- Uploading object with zero payload (#122)
- Different headers format in GET and HEAD (#125)
- Fixed project name in docs (#120)

### Added
- Support object attributes with spaces (#123)

### Changed
- Updated fasthttp to v1.34.0 (#129)
- Updated NeoFS SDK to v1.0.0-rc.3 (#126, #132)
- Refactored content type detecting (#128)


## [0.18.0] - 2021-12-10

### Fixed
- System headers format (#111)

### Added
- Different formats to set object's expiration: in epoch, duration, timestamp, 
  RFC3339 (#108)
- Support of nodes priority (#115)

### Changed 
- Updated testcontainers dependency (#100)

## [0.17.0] - 2021-11-15

Support of bulk file download with zip streams and various bug fixes.

### Fixed
- Allow canonical `X-Attribute-Neofs-*` headers (#87)
- Responses with error message now end with `\n` character (#105)
- Application does not require all neofs endpoints to be healthy at start now
  (#103)
- Application now tracks session token errors and recreates tokens in runtime
  (#95)

### Added
- Integration tests with [all-in-one](https://github.com/nspcc-dev/neofs-aio/)
  test containers (#85, #94)
- Bulk download support with zip streams (#92, #96)

## 0.16.1 (28 Jul 2021)

New features:
* logging requests (#77)
* HEAD methods for download routes (#76)

Improvements:
* updated sdk-go dependency (#82)

Bugs fixed:
* wrong NotFound status was used (#30)

## 0.16.0 (29 Jun 2021)

We update HTTP gateway with NEP-6 wallets support, YAML configuration files
and small fixes.

New features:
 * YAML configuration file (#71)

Behavior changes:
 * gateway key needs to be stored in a proper NEP-6 wallet now, `-k` option is
   no longer available, see `-w` and `-a` (#68)

Bugs fixed:
 * downloads were not streamed leading to excessive memory usage (#67)
 * Last-Modified header incorrectly used local time (#75)

## 0.15.2 (22 Jun 2021)

New features:
 * Content-Type returned for object GET requests can now be taken from
   attributes (overriding autodetection, #65)

Behavior changes:
 * grpc keepalive options can no longer be changed (#60)

Improvements:
 * code refactoring (more reuse between different gateways, moved some code to
   sdk-go, #47, #46, #51, #62, #63)
 * documentation updates and fixes (#53, #49, #55, #59)
 * updated api-go dependency (#57)

Bugs fixed:
 * `-k` option wasn't accepted for key although it was documented (#50)

## 0.15.1 (24 May 2021)

This important release makes HTTP gateway compatible with NeoFS node version
0.20.0.

Behavior changes:
 * neofs-api-go was updated to 1.26.1, which contains some incompatible
   changes in underlying components (#39, #44)
 * `neofs-http-gw` is consistently used now for repository, binary and image
   names (#43)

Improvements:
 * minor code cleanups based on stricter set of linters (#41)
 * updated README (#42)

## 0.15.0 (30 Apr 2021)

This is the first public release incorporating latest NeoFS protocol support
and fixing some bugs.

New features:
 * upload support (#14, #13, #29)
 * ephemeral keys (#26)
 * TLS server support (#28)

Behavior changes:
 * node weights can now be specified as simple numbers instead of percentages
   and gateway will calculate the proportion automatically (#27)
 * attributes are converted now to `X-Attribute-*` headers when retrieving
   object from gate instead of `X-*` (#29)

Improvements:
 * better Makefile (#16, #24, #33, #34)
 * updated documentation (#16, #29, #35, #36)
 * updated neofs-api-go to v1.25.0 (#17, #20)
 * updated fasthttp to v1.23.0+ (#17, #29)
 * refactoring, eliminating some dependencies (#20, #29)

Bugs fixed:
 * gateway attempted to work with no NeoFS peers configured (#29)
 * some invalid headers could be sent for attributes using non-ASCII or
   non-printable characters (#29)

## Older versions

Please refer to [Github
releases](https://github.com/nspcc-dev/neofs-http-gw/releases/) for older
releases.

[0.17.0]: https://github.com/nspcc-dev/neofs-http-gw/compare/v0.16.1...v0.17.0
[0.18.0]: https://github.com/nspcc-dev/neofs-http-gw/compare/v0.17.0...v0.18.0
[0.19.0]: https://github.com/nspcc-dev/neofs-http-gw/compare/v0.18.0...v0.19.0
[0.20.0]: https://github.com/nspcc-dev/neofs-http-gw/compare/v0.19.0...v0.20.0
[0.21.0]: https://github.com/nspcc-dev/neofs-http-gw/compare/v0.20.0...v0.21.0
[0.22.0]: https://github.com/nspcc-dev/neofs-http-gw/compare/v0.21.0...v0.22.0
[0.23.0]: https://github.com/nspcc-dev/neofs-http-gw/compare/v0.22.0...v0.23.0
[0.24.0]: https://github.com/nspcc-dev/neofs-http-gw/compare/v0.23.0...v0.24.0
[0.25.0]: https://github.com/nspcc-dev/neofs-http-gw/compare/v0.24.0...v0.25.0
[0.25.1]: https://github.com/nspcc-dev/neofs-http-gw/compare/v0.25.0...v0.25.1
[0.26.0]: https://github.com/nspcc-dev/neofs-http-gw/compare/v0.25.1...v0.26.0
[Unreleased]: https://github.com/nspcc-dev/neofs-http-gw/compare/v0.26.0...master
