#!/usr/bin/env bats

teardown() {
    docker rm -vf $(docker ps -a | grep -v ID | grep -v tor-router | awk '{ print $1 }' 2> /dev/null) &>/dev/null || true
}

@test "create network" {
    docker network create -d tor darknet
}

@test "check bridge was created" {
    ip a | grep -q torbr-
}

@test "check iptables chain was created" {
    iptables-save | grep -q TOR
}

@test "run a container in the network" {
    run sh -c "docker run --rm -it --net darknet jess/curl curl -sSL https://check.torproject.org/api/ip | jq .IsTor"

    [ "$output" = "true" ]
}

@test "run a container with a published port" {
    docker run -d --name nginx --net darknet -p 1234:80 nginx
    run sh -c "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 10 --max-time 10 http://$(docker inspect --format '{{.NetworkSettings.Networks.darknet.IPAddress}}' nginx):80"

    [ "$output" -eq 200 ]
}

@test "container has network access" {
    docker run --rm -it --net darknet busybox nslookup google.com
    docker run --rm -it --net darknet busybox nslookup apt.dockerproject.org
}

@test "delete network" {
    docker network rm darknet
}

@test "check bridge was deleted" {
    run sh -c "ip a | grep -q torbr-"

    [ "$status" -eq 1 ]
}

@test "check iptables chain was removed" {
    run sh -c "iptables-save | grep -q TOR"

    [ "$status" -eq 1 ]
}

@test "create network without tor-router fails" {
    docker rm -f tor-router
    run docker network create -d tor darknet

    [ "$status" -ne 0 ]
    [[ "$output" =~ *"no such id"* ]]
}
