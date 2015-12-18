# Docker network extension api.

Go handler to create external network extensions for Docker.
Inspired by @calavera's awesome [`dkvolume` library](https://github.com/calavera/dkvolume)

## Usage

This library is designed to be integrated in your program.

1. Implement the `dknet.Driver` interface.
2. Initialize a `dknet.Handler` with your implementation.
3. Call either `ServeTCP` or `ServeUnix` from the `dknet.Handler`.

### Example using TCP sockets:

```go
  d := MyNetworkDriver{}
  h := dknet.NewHandler(d)
  h.ServeTCP("test_network", ":8080")
```

### Example using Unix sockets:

```go
  d := MyNetworkDriver{}
  h := dknet.NewHandler(d)
  h.ServeUnix("root", "test_network")
```

## Full example plugins

- [docker-ovs-plugin](https://github.com/gopher-net/docker-ovs-plugin) - An Open vSwitch Networking Plugin

## License

MIT
