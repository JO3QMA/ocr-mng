# Review Manager (`ocr-mng`)

[日本語](README.md) | **English**

Automatically reviews Pull Requests on GitHub / Gitea with the [Open Code Review](https://github.com/alibaba/open-code-review) (OCR) CLI and posts results as PR comments. This repository provides the management WebUI.

A single Go process (`rm`) serves the WebUI, polls PRs, and schedules Review Runs; only OCR runs as a subprocess.

## Features

- Manage Registered Git Hosts (GitHub / Gitea) and Registered Repos
- Start Review Runs when a Trigger Label is applied (off→on) or manually from the WebUI
- LLM Provider / Model registry and LLM Rotation (round-robin)
- Review Comment Mode (inline / Markdown comment) and Zero-Finding Approval
- Persistence via SQLite + volume (designed for a single-service Docker Compose deployment)

For terminology and domain language, see [`CONTEXT.md`](CONTEXT.md) (Japanese).

## Requirements

- **Go** 1.25+ (local builds)
- **Git** >= 2.41 (for OCR and Repo Mirror / Worktree)
- **OCR CLI** >= v1.8.7 (for `--provider` / `--model` selection)
- Docker / Compose (when using the published image)

See [`docs/runtime-requirements.md`](docs/runtime-requirements.md) for details.

## Quick start (Docker Compose)

```bash
cp .env.example .env
# Edit RM_ADMIN_USER / RM_ADMIN_PASSWORD / RM_ENCRYPTION_KEY
# RM_ENCRYPTION_KEY must be at least 32 bytes

docker compose up -d
```

By default the WebUI is at `http://localhost:8088` (port `:8080` inside the container).

Image: [`ghcr.io/jo3qma/ocr-mng`](https://github.com/jo3qma/ocr-mng/pkgs/container/ocr-mng)

## Configuration

| Variable | Required | Description |
|---|---|---|
| `RM_ADMIN_USER` | Yes | Administrator username |
| `RM_ADMIN_PASSWORD` | Yes | Administrator password |
| `RM_ENCRYPTION_KEY` | Yes | Master key for encrypting Host/Repo PATs and LLM API keys (32+ bytes) |
| `RM_LISTEN_ADDR` | No | Listen address (default `:8080`) |
| `RM_DATA_DIR` | No | Data directory for SQLite, mirrors, etc. (default `/data`) |
| `RM_OCR_BINARY` | No | OCR executable name (default `ocr`) |

See also [`.env.example`](.env.example).

## Development

```bash
make build   # bin/rm
make lint
make test
make docker  # ocr-mng:local
```

Local run example:

```bash
export RM_ADMIN_USER=admin
export RM_ADMIN_PASSWORD=change-me
export RM_ENCRYPTION_KEY=01234567890123456789012345678901
export RM_DATA_DIR=./data
export RM_LISTEN_ADDR=:8080
# Ensure `ocr` (>= v1.8.7) and `git` (>= 2.41) are on PATH
./bin/rm
```

## Docs

- [`CONTEXT.md`](CONTEXT.md) — domain language (Japanese)
- [`docs/adr/`](docs/adr/) — architecture decisions
- [`docs/ocr-review-output.md`](docs/ocr-review-output.md) — OCR JSON output
- [Open Code Review documentation](https://open-codereview.ai/docs)

## License

This repository (Review Manager) is under the [MIT License](LICENSE).

The bundled [Open Code Review](https://github.com/alibaba/open-code-review) CLI is **Apache-2.0**. Follow OCR’s license terms when using or redistributing OCR.
