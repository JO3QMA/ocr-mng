# Runtime requirements

Open Code Review (OCR) requires **Git >= 2.41** ([alibaba/open-code-review#261](https://github.com/alibaba/open-code-review/issues/261)) and **OCR CLI >= v1.8.7** for per-run `--provider` / `--model` selection ([alibaba/open-code-review#687](https://github.com/alibaba/open-code-review/pull/687)). Review Manager does not version-gate before invoking OCR; an older binary fails the Review Run when it rejects unknown flags. Review Manager uses the host `git` for Repo Mirror fetch and Review Worktree creation; the production Docker image bundles `git` for the same paths inside the container.

## Production Docker image

- Base image: `debian:trixie-slim` (runtime stage).
- `git` is installed via `apt`; the Dockerfile fails the build if `git --version` is below 2.41.
- As of 2026-07, the image ships **git 2.47.x** (verify with `docker run --rm --entrypoint git ghcr.io/jo3qma/ocr-mng:latest --version`).

## Host git (non-Docker)

When running `rm` directly on a host (without the container), ensure `git --version` reports **2.41 or newer**. Mirror fetch and worktree operations invoke the host binary.

## CI (GitHub Actions)

Workflows use `ubuntu-latest` (Ubuntu 24.04). The runner image includes **git 2.43+** — sufficient for checkout and any job that shells out to `git`. Lint and test jobs do not run OCR; the `docker` job builds the production image, which enforces the Git floor at image build time.
