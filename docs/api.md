# HTTP Gateway Specification

| Route                                           | Description                                  |
|-------------------------------------------------|----------------------------------------------|
| `/upload/{cid}`                                 | [Put object](#put-object)                    |
| `/get/{cid}/{oid}`                              | [Get object](#get-object)                    |
| `/get_by_attribute/{cid}/{attr_key}/{attr_val}` | [Search object](#search-object)              |
| `/zip/{cid}/{prefix}`                           | [Download objects in archive](#download-zip) |

**Note:** `cid` parameter can be base58 encoded container ID or container name
(the name must be registered in NNS, see appropriate section in [README](../README.md#nns)).

Route parameters can be:

* `Single` - match a single path segment (cannot contain `/` and be empty)
* `Catch-All` - match everything (such parameter usually the last one in routes)
* `Query` - regular query parameter

### Bearer token

All routes can accept [bearer token](../README.md#authentication) from:

* `Authorization` header with `Bearer` type and base64-encoded token in
  credentials field
* `Bearer` cookie with base64-encoded token contents

Example:

Header:

```
Authorization: Bearer ChA5Gev0d8JI26tAtWyyQA3WEhsKGTVxfQ56a0uQeFmOO63mqykBS1HNpw1rxSgaBgiyEBjODyIhAyxcn89Bj5fwCfXlj5HjSYjonHSErZoXiSqeyh0ZQSb2MgQIARAB
```

Cookie:

```
cookie: Bearer=ChA5Gev0d8JI26tAtWyyQA3WEhsKGTVxfQ56a0uQeFmOO63mqykBS1HNpw1rxSgaBgiyEBjODyIhAyxcn89Bj5fwCfXlj5HjSYjonHSErZoXiSqeyh0ZQSb2MgQIARAB
```

## Put object

Route: `/upload/{cid}`

| Route parameter | Type   | Description                                             |
|-----------------|--------|---------------------------------------------------------|
| `cid`           | Single | Base58 encoded container ID or container name from NNS. |

### Methods

#### POST

Upload file as object with attributes to FrostFS.

##### Request

###### Headers

| Header                | Description                                                                                                                                       |
|-----------------------|---------------------------------------------------------------------------------------------------------------------------------------------------|
| Common headers        | See [bearer token](#bearer-token).                                                                                                                |
| `X-Attribute-Neofs-*` | Used to set system NeoFS object attributes <br/> (e.g. use "X-Attribute-Neofs-Expiration-Epoch" to set `__NEOFS__EXPIRATION_EPOCH` attribute).    |
| `X-Attribute-*`       | Used to set regular object attributes <br/> (e.g. use "X-Attribute-My-Tag" to set `My-Tag` attribute).                                            |
| `Date`                | This header is used to calculate the right `__NEOFS__EXPIRATION` attribute for object. If the header is missing, the current server time is used. |

There are some reserved headers type of `X-Attribute-NEOFS-*` (headers are arranged in descending order of priority):

1. `X-Attribute-Neofs-Expiration-Epoch: 100`
2. `X-Attribute-Neofs-Expiration-Duration: 24h30m`
3. `X-Attribute-Neofs-Expiration-Timestamp: 1637574797`
4. `X-Attribute-Neofs-Expiration-RFC3339: 2021-11-22T09:55:49Z`

which transforms to `X-Attribute-Neofs-Expiration-Epoch`. So you can provide expiration any convenient way.

If you don't specify the `X-Attribute-Timestamp` header the `Timestamp` attribute can be set anyway
(see http-gw [configuration](gate-configuration.md#upload-header-section)).

The `X-Attribute-*` headers must be unique. If you provide several the same headers only one will be used.
Attribute key and value must be valid utf8 string. All attributes in sum must not be greater than 3mb.

###### Body

Body must contain multipart form with file.
The `filename` field from the multipart form will be set as `FileName` attribute of object
(can be overriden by  `X-Attribute-FileName` header).

##### Response

###### Status codes

| Status | Description                                  |
|--------|----------------------------------------------|
| 200    | Object created successfully.                 |
| 400    | Some error occurred during object uploading. |

## Get object

Route: `/get/{cid}/{oid}?[download=true]`

| Route parameter | Type   | Description                                                                                                                                                |
|-----------------|--------|------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `cid`           | Single | Base58 encoded container ID or container name from NNS.                                                                                                    |
| `oid`           | Single | Base58 encoded object ID.                                                                                                                                  |
| `download`      | Query  | Set the `Content-Disposition` header as `attachment` in response.<br/> This make the browser to download object as file instead of showing it on the page. |

### Methods

#### GET

Get an object (payload and attributes) by an address.

##### Request

###### Headers

| Header         | Description                        |
|----------------|------------------------------------|
| Common headers | See [bearer token](#bearer-token). |

##### Response

###### Headers

| Header                | Description                                                                                                                                  |
|-----------------------|----------------------------------------------------------------------------------------------------------------------------------------------|
| `X-Attribute-Neofs-*` | System NeoFS object attributes <br/> (e.g. `__NEOFS__EXPIRATION_EPOCH` set "X-Attribute-Neofs-Expiration-Epoch" header).                     |
| `X-Attribute-*`       | Regular object attributes <br/> (e.g. `My-Tag` set "X-Attribute-My-Tag" header).                                                             |
| `Content-Disposition` | Indicate how to browsers should treat file. <br/> Set `filename` as base part of `FileName` object attribute (if it's set, empty otherwise). |
| `Content-Type`        | Indicate content type of object. Set from `Content-Type` attribute or detected using payload.                                                |
| `Content-Length`      | Size of object payload.                                                                                                                      |
| `Last-Modified`       | Contains the `Timestamp` attribute (if exists) formatted as HTTP time (RFC7231,RFC1123).                                                     |
| `X-Owner-Id`          | Base58 encoded owner ID.                                                                                                                     |
| `X-Container-Id`      | Base58 encoded container ID.                                                                                                                 |
| `X-Object-Id`         | Base58 encoded object ID.                                                                                                                    |

###### Status codes

| Status | Description                                    |
|--------|------------------------------------------------|
| 200    | Object got successfully.                       |
| 400    | Some error occurred during object downloading. |
| 404    | Container or object not found.                 |

#### HEAD

Get an object attributes by an address.

##### Request

###### Headers

| Header         | Description                        |
|----------------|------------------------------------|
| Common headers | See [bearer token](#bearer-token). |

##### Response

###### Headers

| Header                | Description                                                                                                              |
|-----------------------|--------------------------------------------------------------------------------------------------------------------------|
| `X-Attribute-Neofs-*` | System NeoFS object attributes <br/> (e.g. `__NEOFS__EXPIRATION_EPOCH` set "X-Attribute-Neofs-Expiration-Epoch" header). |
| `X-Attribute-*`       | Regular object attributes <br/> (e.g. `My-Tag` set "X-Attribute-My-Tag" header).                                         |
| `Content-Type`        | Indicate content type of object. Set from `Content-Type` attribute or detected using payload.                            |
| `Content-Length`      | Size of object payload.                                                                                                  |
| `Last-Modified`       | Contains the `Timestamp` attribute (if exists) formatted as HTTP time (RFC7231,RFC1123).                                 |
| `X-Owner-Id`          | Base58 encoded owner ID.                                                                                                 |
| `X-Container-Id`      | Base58 encoded container ID.                                                                                             |
| `X-Object-Id`         | Base58 encoded object ID.                                                                                                |

###### Status codes

| Status | Description                                       |
|--------|---------------------------------------------------|
| 200    | Object head successfully.                         |
| 400    | Some error occurred during object HEAD operation. |
| 404    | Container or object not found.                    |

## Search object

Route: `/get_by_attribute/{cid}/{attr_key}/{attr_val}?[download=true]`

| Route parameter | Type      | Description                                                                                                                                           |
|-----------------|-----------|-------------------------------------------------------------------------------------------------------------------------------------------------------|
| `cid`           | Single    | Base58 encoded container ID or container name from NNS.                                                                                               |
| `attr_key`      | Single    | Object attribute key to search.                                                                                                                       |
| `attr_val`      | Catch-All | Object attribute value to match.                                                                                                                      |
| `download`      | Query     | Set the `Content-Disposition` header as `attachment` in response. This make the browser to download object as file instead of showing it on the page. |

### Methods

#### GET

Find and get an object (payload and attributes) by a specific attribute.
If more than one object is found, an arbitrary one will be returned.

##### Request

###### Headers

| Header         | Description                        |
|----------------|------------------------------------|
| Common headers | See [bearer token](#bearer-token). |

##### Response

###### Headers

| Header                | Description                                                                                                                                  |
|-----------------------|----------------------------------------------------------------------------------------------------------------------------------------------|
| `X-Attribute-Neofs-*` | System NeoFS object attributes <br/> (e.g. `__NEOFS__EXPIRATION_EPOCH` set "X-Attribute-Neofs-Expiration-Epoch" header).                     |
| `X-Attribute-*`       | Regular object attributes <br/> (e.g. `My-Tag` set "X-Attribute-My-Tag" header).                                                             |
| `Content-Disposition` | Indicate how to browsers should treat file. <br/> Set `filename` as base part of `FileName` object attribute (if it's set, empty otherwise). |
| `Content-Type`        | Indicate content type of object. Set from `Content-Type` attribute or detected using payload.                                                |
| `Content-Length`      | Size of object payload.                                                                                                                      |
| `Last-Modified`       | Contains the `Timestamp` attribute (if exists) formatted as HTTP time (RFC7231,RFC1123).                                                     |
| `X-Owner-Id`          | Base58 encoded owner ID.                                                                                                                     |
| `X-Container-Id`      | Base58 encoded container ID.                                                                                                                 |
| `X-Object-Id`         | Base58 encoded object ID.                                                                                                                    |

###### Status codes

| Status | Description                                    |
|--------|------------------------------------------------|
| 200    | Object got successfully.                       |
| 400    | Some error occurred during object downloading. |
| 404    | Container or object not found.                 |

#### HEAD

Get object attributes by a specific attribute.
If more than one object is found, an arbitrary one will be used to get attributes.

##### Request

###### Headers

| Header         | Description                        |
|----------------|------------------------------------|
| Common headers | See [bearer token](#bearer-token). |

##### Response

###### Headers

| Header                | Description                                                                                                              |
|-----------------------|--------------------------------------------------------------------------------------------------------------------------|
| `X-Attribute-Neofs-*` | System NeoFS object attributes <br/> (e.g. `__NEOFS__EXPIRATION_EPOCH` set "X-Attribute-Neofs-Expiration-Epoch" header). |
| `X-Attribute-*`       | Regular object attributes <br/> (e.g. `My-Tag` set "X-Attribute-My-Tag" header).                                         |
| `Content-Type`        | Indicate content type of object. Set from `Content-Type` attribute or detected using payload.                            |
| `Content-Length`      | Size of object payload.                                                                                                  |
| `Last-Modified`       | Contains the `Timestamp` attribute (if exists) formatted as HTTP time (RFC7231,RFC1123).                                 |
| `X-Owner-Id`          | Base58 encoded owner ID.                                                                                                 |
| `X-Container-Id`      | Base58 encoded container ID.                                                                                             |
| `X-Object-Id`         | Base58 encoded object ID.                                                                                                |

###### Status codes

| Status | Description                           |
|--------|---------------------------------------|
| 200    | Object head successfully.             |
| 400    | Some error occurred during operation. |
| 404    | Container or object not found.        |

## Download zip

Route: `/zip/{cid}/{prefix}`

| Route parameter | Type      | Description                                             |
|-----------------|-----------|---------------------------------------------------------|
| `cid`           | Single    | Base58 encoded container ID or container name from NNS. |
| `prefix`        | Catch-All | Prefix for object attribute `FilePath` to match.        |

### Methods

#### GET

Find objects by prefix for `FilePath` attributes. Return found objects in zip archive.
Name of files in archive sets to `FilePath` attribute of objects.
Time of files sets to time when object has started downloading.
You can download all files in container that have `FilePath` attribute by `/zip/{cid}/` route.

Archive can be compressed (see http-gw [configuration](gate-configuration.md#zip-section)).

##### Request

###### Headers

| Header         | Description                        |
|----------------|------------------------------------|
| Common headers | See [bearer token](#bearer-token). |

##### Response

###### Headers

| Header                | Description                                                                                                       |
|-----------------------|-------------------------------------------------------------------------------------------------------------------|
| `Content-Disposition` | Indicate how to browsers should treat file (`attachment`). Set `filename` as `archive.zip`.                       |
| `Content-Type`        | Indicate content type of object. Set to `application/zip`                                                         |

###### Status codes

| Status | Description                                         |
|--------|-----------------------------------------------------|
| 200    | Object got successfully.                            |
| 400    | Some error occurred during object downloading.      |
| 404    | Container or objects not found.                     |
| 500    | Some inner error (e.g. error on streaming objects). |
