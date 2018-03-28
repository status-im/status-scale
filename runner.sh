#!/usr/bin/env bash


DOCKER_TEST_WORKDIR=/go/src/github.com/status-im/status-scale/
DOCKER_TEST_IMAGE=golang:1.9


docker run --network=cluster --rm -it -v "$(pwd):$DOCKER_TEST_WORKDIR" -v "/var/run/docker.sock:/var/run/docker.sock" \
-w $DOCKER_TEST_WORKDIR $DOCKER_TEST_IMAGE bash -xc 'cp docker/docker /usr/bin; go test -v ./ -central=15 -leaf=100 -rare=4 -run=V5 -timeout=10m'
