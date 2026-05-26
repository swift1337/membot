# Membot

`membot` indexes local assistant history into a SQLite-backed memory store.

## Development

```sh
make build
make codegen
make install
```

The binary is built at `bin/membot`. By default, `membot` creates and migrates
its SQLite database at `~/.membot/membot.db`.

`make codegen` regenerates the sqlc database package in `internal/db`.
