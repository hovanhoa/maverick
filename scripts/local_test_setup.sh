#! /bin/bash

CONTAINER_NAME="oyster-test-pg"

function has_docker_container() {
  docker ps --all --filter name=$CONTAINER_NAME --format '{{.Names}}' | grep -q $CONTAINER_NAME
}

function create_docker_container() {
  docker create \
    --name $CONTAINER_NAME \
    --env POSTGRES_PASSWORD=postgres \
    --env POSTGRES_DB=postgres \
    --env POSTGRES_USER=postgres \
    --env POSTGRES_HOST_AUTH_METHOD=trust \
    --publish 54320:5432 \
    pgvector/pgvector:pg16 \
    postgres \
      -c max_connections=200 \
      -c shared_buffers=512MB \
      -c work_mem=16MB \
      -c fsync=off \
      -c synchronous_commit=off \
      -c full_page_writes=off \
      -c max_wal_size=2GB \
      -c checkpoint_timeout=30min \
      -c autovacuum=off
}

function start_docker_container() {
  docker start $CONTAINER_NAME
  docker attach $CONTAINER_NAME
}

has_docker_container || create_docker_container || exit 1
start_docker_container
