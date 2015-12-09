#!/usr/bin/env bats

@test "create network" {
    run docker network create -d tor darknet
    [ "$status" -eq 0 ]
}

@test "check bridge was created" {
    run sh -c "ip a | grep -q torbr-"

    [ "$status" -eq 0 ]
}

@test "check iptables gateway rule was added" {
    run sh -c "iptables-save | grep -v docker0 | grep -v dport | grep -q MASQUERADE"

    [ "$status" -eq 0 ]
}

@test "run a container in the network" {
    run docker run --rm -it --net darknet jess/httpie -v --json https://check.torproject.org/api/ip

    [ "$status" -eq 0 ]
    #[[ ${lines[0]} =~ version\ [0-9]+\.[0-9]+\.[0-9]+ ]]
}

@test "run a container with a published port" {
    run docker run -d --name nginx --net darknet -p 1234:80 nginx
    run sh -c "curl -s -o /dev/null -w '%{http_code}' http://localhost:1234"

    curl http://localhost:1234

    [ "$output" -eq 200 ]
}

@test "delete network" {
    run docker network rm darknet
    [ "$status" -eq 0 ]
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
    run docker rm -f tor-router
    run docker network create -d tor darknet

    [ "$status" -ne 0 ]
    [[ "$output" =~ *"no such id"* ]]
}

