#!/bin/bash

BACKUP=${BACKUP:-false}

set -e
set -u
set -o pipefail

echo_info() {
  echo -e "\033[1;34m[INFO]\033[0m $1"
}
echo_error() {
  echo -e "\033[1;31m[ERROR]\033[0m $1" >&2
}

export DIR_NAME="babylon"
export CHAIN_HOME="babylond"
export CHAIN_ID="bbn-test-5"
DOCKER_IMAGE="ghcr.io/qj0r9j0vc2/babylon:v1.0.0-rc.3-logging"

echo_info "Creating and navigating to directory: $DIR_NAME/$CHAIN_HOME"
mkdir -p "$DIR_NAME/$CHAIN_HOME"
cd "$DIR_NAME"

docker compose down

echo_info "Checking for existing genesis.json file"
if [ -f "$CHAIN_HOME/config/genesis.json" ]; then
  read -p "genesis.json already exists. Do you want to initialize? (y/n): " choice
  if [[ "$choice" != "y" && "$choice" != "Y" ]]; then
    echo_info "Initialization skipped. Exiting setup."
    exit 0
  fi
  if [ "$BACKUP" = true ]; then
    backup_file="$(echo $CHAIN_HOME)_$(date +%Y-%m-%dT%H:%M:%S).tar.lz4"
    echo_info "Backing up $CHAIN_HOME to $backup_file"
    tar -cvf - "$CHAIN_HOME" | lz4 > "$backup_file"
    mkdir -p working_hash
    hash_log_file="working_hash/$(date +%Y-%m-%dT%H:%M:%S)_working_hash.log"
    echo_info "Moving working_hash.log to $hash_log_file"
    mv working_hash.log "$hash_log_file" || touch "$hash_log_file"
  fi
fi

rm -rf "$CHAIN_HOME/"
mkdir -p "$CHAIN_HOME"

echo_info "Initializing chain with Docker container"
docker run -it --rm -v "./$CHAIN_HOME":/home/babylon/$CHAIN_HOME/ "$DOCKER_IMAGE" babylond init --home "/home/babylon/$CHAIN_HOME/" local

cd "$CHAIN_HOME"

echo_info "Downloading and extracting network artifacts"
wget -q -O bbn-test-5.tar.gz https://github.com/babylonlabs-io/networks/raw/refs/heads/main/bbn-test-5/network-artifacts/bbn-test-5.tar.gz
tar -zxf bbn-test-5.tar.gz
rm -f bbn-test-5.tar.gz

rm -f data/upgrade-info.json
echo_info "Downloading genesis.json"
wget -q -O config/genesis.json https://github.com/babylonlabs-io/networks/raw/refs/heads/main/bbn-test-5/network-artifacts/genesis.json

echo_info "Fetching seeds and peers"
export SEEDS=$(curl -sS https://raw.githubusercontent.com/babylonlabs-io/networks/refs/heads/main/$CHAIN_ID/seeds.txt | tr -s '[:space:]' ',' | tr -d '\n')
export PEERS=$(curl -sS https://raw.githubusercontent.com/babylonlabs-io/networks/refs/heads/main/$CHAIN_ID/peers.txt | tr -s '[:space:]' ',' | tr -d '\n')

echo_info "Updating config.toml"
sed -i.bak -e "s/seeds = \".*\"/seeds = \"$SEEDS\"/" \
           -e "s/persistent_peers = \".*\"/persistent_peers = \"$PEERS\"/" \
           -e "s/max_num_inbound_peers = .*/max_num_inbound_peers = 20/" \
           -e "s/max_num_outbound_peers = .*/max_num_outbound_peers = 10/" \
           -e "s/laddr = \".*:26657\"/laddr = \"tcp:\/\/0.0.0.0:26657\"/" \
           -e "s/indexer = \".*\"/indexer = \"null\"/" \
           -e "s/prometheus = .*/prometheus = true/" \
           -e "s/timeout_commit = \".*\"/timeout_commit = \"10s\"/" \
           config/config.toml

sed -i.bak -e "s/iavl-disable-fastnode = false/iavl-disable-fastnode = true/" \
           config/app.toml

echo_info "Navigating back to project root"
cd ../../

echo_info "Starting Docker Compose"
docker compose up -d || {
  echo_error "Docker Compose failed to start. Check your setup."
  exit 1
}

echo_info "Setup completed successfully."
