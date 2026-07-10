# claude-board

A live board of Claude Code tool outputs on your iPad (or any browser on the
same Wi-Fi) — so Claude can point at the screen instead of re-pasting a table
into chat. All tools, all sessions. No storage (a ~50-card RAM ring; restart =
clean slate). Single Go binary, no deps; the hook is a plain `curl`.

## Setup

```sh
go install github.com/shepardxia/claude-board@latest
claude-board setup
```

`setup` installs the hook, starts the server, and prints an iPad URL like
`http://10.0.0.213:8787/?k=…`. Bookmark it, restart Claude Code, done.

## Commands

`setup` · `teardown` · `start` · `stop` · `restart` · `status` · `url` · `run` ·
`install-hook` · `uninstall-hook` · `enable-boot` · `disable-boot` · `challenge`

`--port N` (default 8787). `claude-board url` reprints the URL.

## Access

The URL carries a secret token (`?k=…`); anyone else who hits the port gets 403.
`/push` is loopback-only. Optional tap-through quiz gate: `claude-board challenge`.
It's plaintext HTTP — don't run it on untrusted Wi-Fi.
