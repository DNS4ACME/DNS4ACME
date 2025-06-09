#  Installing DNS4ACME on Docker or Podman

Installing DNS4ACME on Docker or Podman is fairly straight forward. We'll use the `docker` command here, but the process is the same with `podman`.

```
docker run \
    -p 53:5353/udp \
    -p 53:5353/tcp \
    ghcr.io/dns4acme/dns4acme \
    --backend your-backend \
    # other-options-here
```

You may want to create a Docker Compose file to make the process reproducible, but otherwise that's all you need to do before [setting up your DNS records](dns-records.md).

!!! tip
    This example contain no configuration see [the Configuration section](../configuration/index.md) for details on configuration.
