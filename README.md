<h1 align="center">
  <img style="height: 100px; width: 100px;" src="https://github.com/citizenwallet/node/blob/main/logos/logo.png" alt="citizen wallet logo"/><br/>
  Citizen Wallet
</h1>

Receive and send citizen coins to pay at participating events.

Move your leftovers coins to your Citizen Wallet on your smartphone.

[Read more.](https://citizenwallet.xyz/)

# Smart Contract Node (ERC20, ERC721, ERC1155)

Smart contract indexing node

## Intro

A smart contract node indexes smart contract transfer events and exposes an API to query them. The indexed data is stored into sqlite dbs (1 per contract + chain id).

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

## Run node

Standard with an http url:

`go run cmd/node/main.go -env .env`

If you have a websocket url:

`go run cmd/node/main.go -env .env -ws`

You can also omit the env flag if you set them manually yourself before running the program (containerization setup where you don't want to include the .env in the image).

`go run cmd/node/main.go`

## Flags

`-env` [string]: path to your `.env` file.

`-port` [int]: port you would like the REST API to be exposed on.

`-ws` [bool]: include this flag if you would like to use the websocket url instead
