# walship (cosmos-analyzer-shipper)

## Overview
- Ships MemLogger/WAL frames using `.idx` sidecar metadata.
- Reads compressed frames from `.wal.gz` by byte range and posts them in batches.
- Persists progress in `$HOME/.cosmos-analyzer-shipper/agent-status.json` to avoid duplicates.
- Defers sends under high CPU/network; hard interval forces progress.

## Build
```shell
go build ./cmd/walship
```

## Configuration
- Default path: `$HOME/.walship/config.toml` (unchanged config path for now)
- Override path: `--config /path/to/config.toml`
- Flags override file values if provided.

## Run examples

### Explicit endpoint
```shell
./walship \
  --wal-dir /var/log/cometbft/wal \
  --remote-url http://backend:8080/v1/ingest/<network>/<node>/wal-frames \
  --auth-key <key>
```

### Build route from base + network/node
```shell
./walship \
  --wal-dir /var/log/cometbft/wal \
  --remote-base http://backend:8080 \
  --network mychain \
  --node validator-01 \
  --auth-key <key>
```
