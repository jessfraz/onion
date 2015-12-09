#!/usr/bin/env bats

@test "create network" {
    run docker network create -d tor darknet
    [ "$status" -eq 0 ]
}

@test "check bridge was created" {
    result=$(ip a | grep torbr-)

    [ "$result" == *"torbr-"* ]
}

@test "check iptables gateway rule was added" {
    result=$(iptables-save | grep -v docker0 | grep -v dport)

    [ "$result" = *"MASQUERADE"* ]
}

@test "run a container in the network" {
    run docker run --rm -it --net darknet jess/httpie -v --json https://check.torproject.org/api/ip

    [ "$status" -eq 0 ]
    #[[ ${lines[0]} =~ version\ [0-9]+\.[0-9]+\.[0-9]+ ]]
}


@test "delete network" {
    run docker network rm darknet
    [ "$status" -eq 0 ]
}

@test "check bridge was deleted" {
    result=$(ip a | grep torbr-)

    [ "$result" != *"torbr-"* ]
}

@test "check iptables gateway rule was removed" {
    result=$(iptables-save | grep -v docker0 | grep -v dport)

    [ "$result" != *"MASQUERADE"* ]
}
