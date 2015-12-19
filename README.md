onion
=====

[![Circle CI](https://circleci.com/gh/jfrazelle/onion.svg?style=svg)](https://circleci.com/gh/jfrazelle/onion)

Tor networking plugin for docker containers

### Usage

**NOTE:** Make sure you are using Docker 1.9 or later

### **WARNING:** Use with caution this is still under active development

**WARNING:** By default all outbound udp traffic in the network should be blocked
because it will not be routed through tor.

Start the tor router

**NOTE:** in the future it should be easier to start any container to route and
have the plugin be smart about finding it, but for now.... deal with it.
```console
$ docker run -d \
    --net host \
    --name tor-router \
    jess/tor-router

# follow the logs to make sure it is bootstrapped successfully
$ docker logs -f tor-router
```

Run the plugin container

```console
$ docker run -d \
    --cap-add NET_ADMIN \
    -v /run/docker/plugins:/run/docker/plugins \
    -v /var/run/docker.sock:/var/run/docker.sock \
    jess/onion
```

Create a new network

```console
$ docker network create -d tor darknet
```

Test it out!

```console
$ docker run --rm -it --net darknet jess/httpie -v --json https://check.torproject.org/api/ip
```

### Running the tests

Unit tests:

```
$ make test
```

Integration tests:

```
$ make dtest
```

### Thanks

Thanks to the libnetwork guys for writing [gopher-net/dknet](https://github.com/github.com/gopher-net/dknet) and of course the networking itself ;) Also a lot of this code is from the bridge driver in libnetwork itself.

### TODO

- the tor router should be discoverable as any docker image or container name
  etc and the ports for forwarding should be able to be found through that
- the tor router should not have to be run as `--net host`
- moar tests (unit and integration)
- exposing ports in the network is a little funky
- saving state?
- make deny all udp traffic configurable
- udp integration tests suck
- unit tests
