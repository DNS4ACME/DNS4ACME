title: Configuration

# Configuring DNS4ACME

DNS4ACME supports configuration using the command line or from environment variables. All options can be passed either way, the command line always takes precedence.

| CLI option      | Environment variable   | Default        | Description                                                                                |
|-----------------|------------------------|----------------|--------------------------------------------------------------------------------------------|
| `--backend`     | `DNS4ACME_BACKEND`     | -              | Which backend to use for information storage. **(required)**                               |
| `--nameservers` | `DNS4ACME_NAMESERVERS` | -              | Comma-separated list of nameservers to include in `SOA` and `NS` responses. **(required)** |
| `--listen`      | `DNS4ACME_LISTEN`      | `0.0.0.0:5353` | Listen address for both UDP and TCP requests.                                              |
| `--log-level`   | `DNS4ACME_LOG_LEVEL`   | `INFO`         | Level to log at. Must be `DEBUG`, `INFO`, `WARN`, or `ERROR`.                              |

Further, each backend has its own configuration options.
