#!/bin/bash

curl -L https://github.com/citizenwallet/indexer/raw/main/binaries/linux_amd64/indexer -o indexer
chmod +x ./indexer
sudo setcap 'cap_net_bind_service=+ep' indexer