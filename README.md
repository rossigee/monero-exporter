# monero-exporter

A Prometheus exporter that polls a [monerod](https://github.com/monero-project/monero)
daemon's JSON-RPC API and exposes node health and chain state on a `/metrics`
endpoint.

It targets monerod's **restricted** RPC (`--restricted-rpc`, default port
`18089`), so it can run against a public or shared node without admin access.
No wallet, ZMQ, or admin-only RPC methods are required.

## Architecture

```
[grafana] --> queries --> [prometheus] -- scrape /metrics --> [monero-exporter]
                                                                    |
                                                                 HTTP+JSON
                                                                    |
                                                                   [monerod]
```

On every scrape the exporter:

1. Calls `get_info` and `get_last_block_header` against monerod.
2. Caches the snapshot in a thread-safe collector.
3. Serves the snapshot from the prometheus client library.

The RPC snapshot is refreshed on each `GET /metrics`, so metrics are at most
one scrape interval stale.

## Usage

```
monero-exporter [flags]
```

| Flag                | Default                 | Description                                        |
| ------------------- | ----------------------- | -------------------------------------------------- |
| `--bind-addr`       | `:9000`                 | address to bind the exporter to                    |
| `--telemetry-path`  | `/metrics`              | path at which metrics are served                   |
| `--monero-addr`     | `http://localhost:18089`| JSON-RPC base URL of monerod (restricted port)     |
| `--rpc-user`        | `""`                    | RPC basic auth username                            |
| `--rpc-password`    | `""`                    | RPC basic auth password                            |
| `--log-level`       | `info`                  | `trace`, `debug`, `info`, `warn`, `error`          |
| `--show-version`    | `false`                 | print version and exit                             |
| `--health-check`    | `false`                 | ping monerod RPC and exit 0/1 (container healthcheck) |

### Example

```console
$ ./monero-exporter --monero-addr http://10.0.0.5:18089 --bind-addr :9000
```

```console
$ curl -s localhost:9000/metrics | grep monero_info_height
monero_info_height 3292302
```

### Container healthcheck

The image declares a `HEALTHCHECK` that runs
`/monero-exporter --health-check`, which pings the monerod RPC endpoint and
exits non-zero if it is unreachable.

## Metrics

### Node health

| Metric                                          | Description                                   |
| ----------------------------------------------- | --------------------------------------------- |
| `monero_up`                                     | 1 if the last RPC scrape succeeded            |
| `monero_scrape_error`                           | 1 if the last RPC scrape failed               |
| `monero_scrape_timestamp_seconds`               | unix time of the last successful scrape       |

### get_info (`monero_info_*`)

| Metric                                       | Description                          |
| -------------------------------------------- | ------------------------------------ |
| `monero_info_height`                         | current chain height                 |
| `monero_info_target_height`                  | target height when syncing           |
| `monero_info_tx_pool_size`                   | transactions in the mempool          |
| `monero_info_block_size_limit_bytes`         | max hard limit of a block            |
| `monero_info_block_size_median_bytes`        | rolling block-size median            |
| `monero_info_offline`                        | 1 if the node reports offline        |
| `monero_info_synchronized`                   | 1 if the node is synced              |
| `monero_info_mainnet`                        | 1 if connected to mainnet            |
| `monero_info_restricted`                     | 1 if RPC is restricted               |
| `monero_info_incoming_connections`           | inbound P2P connections              |
| `monero_info_outgoing_connections`           | outbound P2P connections             |
| `monero_info_rpc_connections`                | active RPC connections               |
| `monero_info_database_size_bytes`            | size of monerod's LMDB database      |
| `monero_info_free_space_bytes`               | free space of the data volume        |
| `monero_info_start_time_seconds`             | unix time monerod started            |
| `monero_info_uptime_seconds`                 | seconds since monerod started        |

### get_last_block_header (`monero_lastblock_*`)

| Metric                                    | Description                       |
| ----------------------------------------- | --------------------------------- |
| `monero_lastblock_height`                 | height of the last block          |
| `monero_lastblock_difficulty`             | difficulty of the last block      |
| `monero_lastblock_reward`                 | total reward (subsidy + fees)     |
| `monero_lastblock_major_version`          | major block version               |
| `monero_lastblock_minor_version`          | minor block version               |
| `monero_lastblock_timestamp_seconds`      | unix time of the last block       |
| `monero_lastblock_transactions`           | transaction count in last block   |

## Docker

```console
$ make docker-build
$ docker run --rm -p 9000:9000 \
    -e MONERO_ADDR=http://host.docker.internal:18089 \
    monero-exporter:latest
```

The image runs as a non-root user from `scratch` and only contains the
static binary plus CA certificates.

## Development

```console
$ make test       # unit tests (race enabled)
$ make lint       # golangci-lint
$ make security   # gosec
$ make build      # build the binary with version ldflags
```

`make all` runs clean, lint, test, and build.

## Prometheus scrape config

```yaml
scrape_configs:
  - job_name: monero
    metrics_path: /metrics
    static_configs:
      - targets: ["monero-exporter:9000"]
```
