# claude-board

A live board of **Claude Code tool outputs** on your iPad (or any browser on the
same Wi-Fi). File reads, command output, diffs, JSON — streamed as Claude runs,
so it can point at the shared screen instead of re-pasting a table into chat.

- **Tool outputs only**, not chat. You read the board; Claude talks in the session.
- **All tools, all sessions** land on one board, tagged by project (cwd basename).
- **No storage.** No files, no database, no history sink — just a ~50-card RAM
  ring so a freshly opened page isn't blank. Restart = clean slate.
- **Zero per-call cost.** The hook is a plain `curl`; a long-running server does
  all the parsing. Single Go binary, no runtime, no dependencies.

## Install

```sh
go install github.com/shepardxia/claude-board@latest
```

That drops a `claude-board` binary in your `$GOBIN` (usually `~/go/bin`; make sure
it's on your `PATH`). Or from a clone:

```sh
go build -o ~/.local/bin/claude-board .
```

## Use

```sh
claude-board setup
```

That installs the Claude Code hook, starts the server (and, on macOS, sets it to
start on login), and prints a URL like `http://10.0.0.213:8787`. Open that on
your iPad, then **restart Claude Code** so the hook loads. Done.

Every tool call in every Claude Code session now shows up on the board. Tap a
row to see its full output; in the detail pane, tap the input or the output to
give it most of the height.

## Access / security

The server binds all interfaces so the iPad can reach it, so the board is gated:

- **Token (default).** A secret in the URL (`?k=…`, stored `0600` in
  `~/.claude-board/token`). The URL you bookmark carries it; anyone else who
  hits the port gets `403`. `/push` is loopback-only — only the local hook can
  post. Get the URL any time with `claude-board url`.
- **Quiz (optional, convenience).** `claude-board challenge` scaffolds a
  multiple-choice quiz; answering it sets a session cookie so you can bookmark
  the plain URL and just tap through on the iPad. This is *guessable* — keep
  casual snoopers out, not a determined attacker. Remove with `challenge off`.
  Tip: the accepted answer need not be the *right* one — mark a deliberately
  wrong option correct (e.g. `6 x 7 = 67`) so anyone who picks the obvious
  answer fails; only you know the trick.

It's plaintext HTTP either way. On an untrusted network, don't run it (or tunnel
it) — the token stops the easy "open the port and watch," not a traffic sniffer.

## Commands

| Command | Does |
|---|---|
| `claude-board setup` | Install hook + autostart + start, print the iPad URL |
| `claude-board teardown` | Remove hook + autostart + stop the server |
| `claude-board start` / `stop` / `restart` | Manage the background server |
| `claude-board status` | Show server / hook / autostart state |
| `claude-board url` | Print the iPad URL |
| `claude-board run` | Run the server in the foreground |
| `claude-board install-hook` / `uninstall-hook` | Just the `settings.json` hook |
| `claude-board enable-boot` / `disable-boot` | Just the macOS login autostart |
| `claude-board challenge` / `challenge off` | Scaffold / remove the optional tappable quiz gate |

Port defaults to `8787`; override with `--port` or `$CLAUDE_BOARD_PORT`.
`setup --no-boot` skips the login autostart.

## How it works

```
Claude Code ──PostToolUse hook (curl)──▶ claude-board server ──SSE──▶ iPad
   raw JSON                               parse + relay          live page
```

The server binds `0.0.0.0:8787` on your Mac, so any device on the same Wi-Fi can
reach it at the Mac's LAN IP. Plain HTTP over the local network — no cloud, no
port-forwarding.

The hook added to `~/.claude/settings.json` is just:

```
curl -s -m 1 -X POST --data-binary @- http://127.0.0.1:8787/push
```

It pipes each tool's raw hook JSON to the server, which extracts the output,
strips ANSI, pretty-prints JSON, and pushes a card to every connected browser.
If the server is down the `curl` fails silently — Claude Code is unaffected.

## Uninstall

```sh
claude-board teardown
rm $(which claude-board)
```

## License

MIT
