#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

source "${DIR}/.integration-daemon-start"
source "${DIR}/.onion-start"

# run the tor router
make dtor

# create the network
docker network create -d tor darknet
docker run --rm -it --net darknet jess/httpie -v --json http://check.torproject.org/api/ip
