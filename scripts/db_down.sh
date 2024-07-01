#!/usr/bin/env bash

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)

source "${script_dir}/env"

podman_id=$(podman ps --filter "name=${CONTAINER_PG}" --filter "status=running" --quiet)

if [[ -n "$podman_id" ]]; then
  echo "${CYAN}Stopping PostgreSQL...${NC}"
  if ! podman stop ${CONTAINER_PG} >/dev/null; then
    echo "[${RED}ERROR${NC}] Failed to stop PostgreSQL"
    exit 1
  fi
fi

echo "[${GREEN}OK${NC}] PostgreSQL is stopped"
