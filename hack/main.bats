#!/usr/bin/env bats

teardown() {
    docker rm -vf $(docker ps -a | grep -v ID | grep -v tor-router | awk '{ print $1 }' 2> /dev/null) &>/dev/null || true
}

@test "create network" {
    docker network create -d tor vidalia
}

@test "check bridge was created" {
    ip a | grep -q torbr-
}

@test "check iptables chain was created" {
    iptables-save | grep -q TOR
}

# this is just a sanity check
@test "check socks proxy for tor-router" {
    run sh -c "curl -sSL --socks https://localhost:22350 https://check.torproject.org/api/ip | jq --raw-output .IsTor"
    #resp1=$(docker run --rm jess/curl curl -sSL --socks $(docker inspect --format '{{.NetworkSettings.Networks.bridge.IPAddress}}' tor-router):9050 https://check.torproject.org/api/ip | jq --raw-output .IsTor)
    resp2=$(docker run --rm jess/curl curl -sSL https://check.torproject.org/api/ip | jq --raw-output .IsTor)

    [ "$output" = "true" ]
    #[ "$resp1" = "true" ]
    [ "$resp2" = "false" ]
}

@test "run a container in the network" {
    run sh -c "docker run --rm --net vidalia jess/curl curl -sSL https://check.torproject.org/api/ip | jq --raw-output .IsTor"

    [ "$output" = "true" ]
}

@test "run a container with a published port" {
    docker run -d --name nginx --net vidalia -p 1234:80 nginx
    run sh -c "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 10 --max-time 10 http://$(docker inspect --format '{{.NetworkSettings.Networks.vidalia.IPAddress}}' nginx):80"

    [ "$output" -eq 200 ]
}

@test "container has network access" {
    docker run --rm --net vidalia busybox nslookup google.com
    docker run --rm --net vidalia busybox nslookup apt.dockerproject.org
}

@test "delete network" {
    docker network rm vidalia
}

@test "check bridge was deleted" {
    run sh -c "ip a | grep -q torbr-"

    [ "$status" -eq 1 ]
}

@test "check iptables chain was removed" {
    run sh -c "iptables-save | grep -q TOR"

    iptables-save

    [ "$status" -eq 1 ]
}

@test "check iptables bridge rules were removed" {
    run sh -c "iptables-save | grep -q 'tor-'"

    iptables-save

    [ "$status" -eq 1 ]
}

@test "create network without tor-router fails" {
    docker rm -f tor-router
    run docker network create -d tor vidalia

    [ "$status" -ne 0 ]
    [[ "$output" =~ *"no such id"* ]]
}
