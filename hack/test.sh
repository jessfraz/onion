#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

source "${DIR}/.integration-daemon-start"
source "${DIR}/.ensure-frozen-images"
source "${DIR}/.onion-start"

# run the tor router
make dtor

# give it a little time to bootstrap so it's "ready"
tries=60
echo "INFO: Waiting for tor router to bootstrap..."
while [ "$(curl -sSL --socks https://localhost:22350 https://check.torproject.org/api/ip 2>/dev/null | jq --raw-output .IsTor)" != "true" ]; do
	(( tries-- ))
	if [ $tries -le 0 ]; then
		printf "\n"
		echo >&2 "error: failed to bootstrap tor router:"
		curl -sSL --socks https://localhost:22350 https://check.torproject.org/api/ip >&2
		false
	fi
	printf "."
	sleep 2
done
printf "\n"


# run bats tests
time bats --tap ${DIR}
