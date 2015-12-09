#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

source "${DIR}/.integration-daemon-start"
source "${DIR}/.ensure-frozen-images"
source "${DIR}/.onion-start"

# run the tor router
make dtor

# run bats tests
time bats --tap .
