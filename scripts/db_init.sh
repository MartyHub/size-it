#!/usr/bin/env bash

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)

source "${script_dir}/env"

podman exec \
  --interactive \
  --tty \
  ${CONTAINER_PG} \
  psql \
  --dbname=postgres \
  --file '/local/scripts/db_init.sql' \
  --host=localhost \
  --username=postgres
