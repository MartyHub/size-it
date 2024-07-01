#!/usr/bin/env bash

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)

source "${script_dir}/env"

podman_id=$(podman ps --all --filter "name=${CONTAINER_PG}" --quiet)
if [[ -z "$podman_id" ]]; then
  echo "${CYAN}Creating PostgreSQL container...${NC}"
  if ! podman run \
    --detach \
    --env POSTGRES_PASSWORD=postgres \
    --health-cmd pg_isready \
    --health-interval 1s \
    --health-timeout 5s \
    --health-retries 5 \
    --name ${CONTAINER_PG} \
    --publish 5432:5432 \
    --rm \
    --volume $(PWD):/local \
    postgres:16.3 \
    >/dev/null; then
    echo "[${RED}ERROR${NC}] Failed to create PostgreSQL container"
    exit 1
  fi
else
  podman_id=$(podman ps --filter "name=${CONTAINER_PG}" --filter "status=running" --quiet)
  if [[ -z "$podman_id" ]]; then
    echo "${CYAN}Starting PostgreSQL...${NC}"
    if ! podman start \
      ${CONTAINER_PG} \
      >/dev/null; then
      echo "[${RED}ERROR${NC}] Failed to start PostgreSQL"
      exit 1
    fi
  fi
fi

echo "[${GREEN}OK${NC}] PostgreSQL is running"
