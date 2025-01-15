#!/bin/bash

BACKUP=${BACKUP:-true}

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

docker compose down

cd "$DIR_NAME"
if [ "$BACKUP" = true ]; then
  backup_file="$(echo $CHAIN_HOME)_$(date +%Y-%m-%dT%H:%M:%S).tar.lz4"
  echo_info "Backing up $CHAIN_HOME to $backup_file"
  tar -cvf - "$CHAIN_HOME" | lz4 > "$backup_file"
  mkdir -p working_hash
  hash_log_file="working_hash/$(date +%Y-%m-%dT%H:%M:%S)_working_hash.log"
  echo_info "Moving working_hash.log to $hash_log_file"
  mv working_hash.log "$hash_log_file" || touch "$hash_log_file"
fi