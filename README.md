<h1 align="center">
  <img style="height: 100px; width: 100px;" src="https://github.com/citizenwallet/indexer/blob/main/logos/logo.png" alt="citizen wallet logo"/><br/>
  Citizen Wallet
</h1>

Receive and send citizen coins to pay at participating events.

Move your leftovers coins to your Citizen Wallet on your smartphone.

[Read more.](https://citizenwallet.xyz/)

# Smart Contract Indexer (ERC20, ERC721, ERC1155)

Smart contract indexing program

## Intro

A smart contract indexer indexes smart contract transfer events and exposes an API to query them. The indexed data is stored into sqlite dbs (1 per contract + chain id).

The purpose is to make it easier and faster to query event data.

## Support

Our aim is to support the most popular token standards.

- ERC20
- ERC721
- ERC1155

## Installation

`go get ./...`

## Setup .env file

`cp .example.env .env`

Replace URLs with your own RPC urls

## Build (optional)

This will build for the current platform you are on. It's possible to cross-compile if you provide flags.

`go build -o indexer ./cmd/node/main.go`

Linux cross-compilation

`GOARCH=amd64 GOOS=linux go build -o indexer ./cmd/node/main.go`

Make it executable

`chmod +x indexer`

## Run indexer

Run the build (doesn't require Go to be installed)

`./indexer`

Run from the source files directly (Go needs to be installed)

`go run cmd/indexer/main.go`

Standard with an http url:

`go run cmd/indexer/main.go -env .env`

If you have a websocket url:

`go run cmd/indexer/main.go -env .env -ws`

You can also omit the env flag if you set them manually yourself before running the program (containerization setup where you don't want to include the .env in the image).

`go run cmd/indexer/main.go`

## Flags

`-env` [string]: path to your `.env` file. (default = '')

`-port` [int]: port you would like the REST API to be exposed on. (default = 3000)

`-sync` [int]: the amount of seconds to wait before syncing events from latest blocks. (default = 5)

`-ws` [bool]: include this flag if you would like to use the websocket url instead. (default = false)

`-rate` [int]: control how many blocks get processed at a time. (default = 99)

## Sync

When the indexer starts up, logs are downloaded block by block to make sure all events are up to date.

After the initial indexing work is done, indexer will sync the latest blocks every few seconds.

### Standards

Syncing is done by standards, querying is done by event types on contracts. ERC20, ERC721, ERC1155 are supported as of this moment. We have only implemented indexing of transfer events.

## Endpoints

### Logs

Fetch all logs before a given maxDate with a limit and offset.

`[GET] /logs/transfers/{contract_address}/{address}?maxDate=2023-06-14T21%3A46%3A25%2B02%3A00&limit=10&offset=0`

URL params

`{contract_address}`: the address of the token contract you would like to query.

`{address}`: the address of the "to" or "from" from an event log.

Query params

`maxDate`: a url encoded date string in iso format (RFC3339). Default = now.

`limit`: for pagination, the maximum amount of items that should be returned. Default = 20.

`offset`: for pagination, the row at which the query should start from. Default = 0.

### New Logs

Fetch all new logs after a give fromDate with a limit

`[GET] /logs/transfers/{contract_address}/{address}/new?maxDate=2023-06-14T21%3A46%3A25%2B02%3A00&limit=10`

URL params

`{contract_address}`: the address of the token contract you would like to query.

`{address}`: the address of the "to" or "from" from an event log.

Query params

`fromDate`: a url encoded date string in iso format (RFC3339). Default = now.

`limit`: for pagination, the maximum amount of items that should be returned. Default = 10.

### Protected routes

To ensure the right people make the right requests, we use signed requests.

Headers

`X-Signature`: a base64 encoding of the entire request body.

`X-Address`: the hex address of the key that was used to generate the signature.

Body

```
{
    "data": data, // base64 encoded data
    "encoding": "base64", // how is the data encoded
    "expiry": expiry,
}
```

### Add a "sending" Log [Protected]

Add a new log to the indexer of a transaction that was submitted but is not yet on chain.

`[POST] /logs/transfers/{contract_address}/{address}`

URL params

`{contract_address}`: the address of the token contract you would like to query.

`{address}`: the address of the "to" or "from" from an event log.

Headers

`X-Signature` & `X-Address`

Body

```
{
    "data": data, // base64 encoded data
    "encoding": "base64", // how is the data encoded
    "expiry": expiry,
}
```

Data

`indexer.Transfer`

### Set a "sending" Log to "pending" [Protected]

Update an existing log to "pending" indicating that a user op was submitted.

`[POST] /logs/transfers/{contract_address}/{address}/{hash}`

URL params

`{contract_address}`: the address of the token contract you would like to query.

`{address}`: the address of the "to" or "from" from an event log.

`{hash}`: the user op hash of this event log.

Headers

`X-Signature` & `X-Address`

### Events [Protected]

`[POST] /events`

Adding a new event will trigger indexing of event logs starting from the current latest block on the network until `last_block`. Once indexing is done, `last_block` will be updated so that we only partially re-index next time.

Body

```
{
    "contract": "0xDe365ad2E3edA7739f9d61aF96288357CEf38c0a",
    "start_block": 43640241,
    "last_block": 43640241,
    "standard": "ERC20",
    "name": "Brussels Bar Token",
    "symbol": "BBT"
}
```

### Pin a profile with image [Protected]

Create or update a profile.

`[PUT] /profiles/{address}`

URL params

`{address}`: the address that this profile belongs to.

Headers

`X-Signature` & `X-Address`

Multi-part form `file`

A jpg, png or gif.

Multi-part form `body`

```
{
    "data": data, // base64 encoded data
    "encoding": "base64", // how is the data encoded
    "expiry": expiry,
}
```

Data

`indexer.Profile`

### Update a pinned profile [Protected]

Updates a profile with no image modifications.

`[PATCH] /profiles/{address}`

URL params

`{address}`: the address that this profile belongs to.

Headers

`X-Signature` & `X-Address`

Body

```
{
    "data": data, // base64 encoded data
    "encoding": "base64", // how is the data encoded
    "expiry": expiry,
}
```

Data

`indexer.Profile`

### Un-pin a pinned profile [Protected]

Un-pins a profile.

`[DELETE] /profiles/{address}`

URL params

`{address}`: the address that this profile belongs to.

Headers

`X-Signature` & `X-Address`

### Storage

We use postgres. If you have docker installed, you can spin up an instance using `docker compose up db`.

The tables will be generated as needed.
