#!/bin/bash

sed -i.bak -e "s/seeds = \".*\"/seeds = \"$SEEDS\"/" \
           -e "s/persistent_peers = \".*\"/persistent_peers = \"$PEERS\"/" \
           -e "s/max_num_inbound_peers = .*/max_num_inbound_peers = 20/" \
           -e "s/max_num_outbound_peers = .*/max_num_outbound_peers = 10/" \
           -e "s/laddr = \".*:26657\"/laddr = \"tcp:\/\/0.0.0.0:26657\"/" \
           -e "s/indexer = \".*\"/indexer = \"null\"/" \
           -e "s/prometheus = .*/prometheus = true/" \
           -e "s/timeout_commit = \".*\"/timeout_commit = \"10s\"/" \
           config/config.toml
