#!/bin/bash

curl -L https://github.com/citizenwallet/indexer/raw/main/binaries/linux_arm64/indexer -o indexer
chmod +x ./indexer
sudo setcap 'cap_net_bind_service=+ep' indexer