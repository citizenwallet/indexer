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

`go build -o indexer ./cmd/indexer/main.go`

Make it executable

`chmod +x indexer`

### Cross-compiling from macOS to Linux

Due to the use of go-sqlite3 which uses CGO, compilation is a bit fun.

`brew install FiloSottile/musl-cross/musl-cross` << this can take a while

`CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++ GOARCH=amd64 GOOS=linux CGO_ENABLED=1 go build -ldflags "-linkmode external -extldflags -static" -o indexer ./cmd/indexer/main.go`

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

`[GET] /logs/transfers/{contract_address}/{address}?maxDate=2023-06-14T21%3A46%3A25%2B02%3A00&limit=10&offset=0`

URL params

`{contract_address}`: the address of the token contract you would like to index.

`{address}`: the address of the "to" or "from" from an event log.

Query params

`maxDate`: a url encoded date string in iso format (RFC3339). Default = now.

`limit`: for pagination, the maximum amount of items that should be returned. Default = 20.

`offset`: for pagination, the row at which the query should start from. Default = 0.

### Events

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

## Local storage

All dbs are stored under your home folder `~/.cw/`.
