#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

source "${DIR}/.integration-daemon-start"
source "${DIR}/.ensure-frozen-images"
source "${DIR}/.onion-start"

# run the tor router
make dtor
# wait for it to be bootstrapped
sleep 10

# run bats tests
time bats --tap ${DIR}
