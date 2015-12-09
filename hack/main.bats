#!/usr/bin/env bats

@test "create network" {
    run docker network create -d tor darknet
    [ "$status" -eq 0 ]
}

@test "check bridge was created" {
    run ip a

    [ "$status" -eq 0 ]
    [ "$output" = *"torbr-"* ]
}

@test "check iptables gateway rule was added" {
    run iptables-save | grep -v docker0

    [ "$status" -eq 0 ]
    [ "$output" = *"MASQUERADE"* ]
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
    run ip a

    [ "$status" -eq 0 ]
    [ "$output" != *"torbr-"* ]
}

@test "check iptables gateway rule was added" {
    run iptables-save | grep -v docker0

    [ "$status" -eq 0 ]
    [ "$output" != *"MASQUERADE"* ]
}
