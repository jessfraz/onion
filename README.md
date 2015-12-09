onion
=====

[![Circle CI](https://circleci.com/gh/jfrazelle/onion.svg?style=svg)](https://circleci.com/gh/jfrazelle/onion)

Tor networking plugin for docker containers

### Usage

**NOTE:** Make sure you are using Docker 1.9 or later

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

### Thanks

Thanks to the libnetwork guys for writing [gopher-net/dknet](https://github.com/github.com/gopher-net/dknet) and of course the networking itself ;)
