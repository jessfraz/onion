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

@test "check iptables gateway rule was added" {
    iptables-save | grep -v docker0 | grep -v dport | grep -q MASQUERADE
}

@test "run a container in the network" {
    #docker run --rm -it --net darknet jess/httpie -v --json https://check.torproject.org/api/ip

    #iptables-save

    echo "with proxy"
    #curl -sSL --connect-timeout 10 --max-time 10 --socks $(docker inspect --format "{{.NetworkSettings.Networks.bridge.IPAddress}}" tor-router):9050 https://check.torproject.org/api/ip

    echo "without proxy"
    curl -sSL --connect-timeout 10 --max-time 10 https://check.torproject.org/api/ip

    [ "$status" -eq 0 ]
}

@test "run a container with a published port" {
    run docker run -d --name nginx --net darknet -p 1234:80 nginx
    run sh -c "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 10 --max-time 10 http://localhost:1234"

    cat /var/log/onion.log

    curl --connect-timeout 10 --max-time 10  http://localhost:1234

    [ "$output" -eq 200 ]
}

@test "delete network" {
    docker network rm darknet
}

@test "check bridge was deleted" {
    run sh -c "ip a | grep -q torbr-"

    [ "$status" -eq 1 ]
}

@test "check iptables gateway rule was removed" {
    run sh -c "iptables-save | grep -v docker0 | grep -v dport | grep -q MASQUERADE"

    [ "$status" -eq 1 ]
}

@test "create network without tor-router fails" {
    docker rm -f tor-router
    run docker network create -d tor darknet

    [ "$status" -ne 0 ]
    [[ "$output" =~ *"no such id"* ]]
}
