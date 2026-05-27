# macOS background indexing

`contrib/macos/com.swift1337.membot.index.plist.tmpl` is the documented LaunchAgent
template. The embedded runtime copy lives at
`internal/system/macos/launchagent.plist.tmpl` and should stay in sync.

Install the background indexer with:

```sh
membot service install
```

That writes `~/Library/LaunchAgents/com.swift1337.membot.index.plist` and runs:

```sh
membot index all --watch --interval 120s
```

Logs go to `~/.membot/logs/index.stdout.log` and `index.stderr.log`.
