# Development Guide

## Build and test

```bash
# Build only
make build

# Run directly (requires DISCORD_BOT_TOKEN)
make run

# Run tests
make test

# Run tests with race detector
make test-race

# Test coverage summary
make coverage

# HTML coverage report
make coverage-html

# Vet + race tests
make check
```

## Project structure

```
.
├── cmd/claude-bot/              # Bot entrypoint
│   └── main.go
├── internal/
│   ├── bot/                     # Discord bot core logic
│   │   ├── bot.go               # Init, handler registration, run
│   │   ├── commands.go          # Slash/message command handlers
│   │   ├── polling.go           # tmux choice polling + text input detection
│   │   └── state.go             # Interaction state management
│   ├── tmux/                    # tmux operation utilities
│   │   └── tmux.go
│   └── imaging/                 # Screenshot + image processing
│       └── screenshot.go
├── deploy/
│   ├── bot/
│   │   └── run-bot.sh           # Bot launcher script
│   ├── claude-channel/
│   │   ├── run-channel.sh       # Claude launcher script
│   │   ├── pre-run.sh.example   # Pre-run environment template
│   │   ├── channel.env.example  # Channel config template
│   │   └── discord-restart-notify.sh
│   ├── install.sh
│   └── uninstall.sh
├── .github/workflows/
│   └── release.yml              # GoReleaser GitHub Actions
├── .goreleaser.yml
├── Makefile
├── go.mod
└── go.sum
```

## Release

Push a tag to trigger GoReleaser via GitHub Actions:

```bash
git tag v0.1.0
git push origin v0.1.0
```
