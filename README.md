# Review Manager (`ocr-mng`)

GitHub / Gitea 上の Pull Request を [Open Code Review](https://github.com/alibaba/open-code-review) (OCR) CLI で自動レビューし、結果を PR コメントとして投稿する管理 WebUI です。

単一の Go プロセス（`rm`）が WebUI・PR ポーリング・Review Run スケジューリングを担い、OCR だけを subprocess として起動します。

## Features

- Registered Git Host（GitHub / Gitea）と Registered Repo の管理
- Trigger Label 付与（off→on）または WebUI からの手動起動で Review Run
- LLM Provider / Model 台帳と LLM Rotation（round-robin）
- Review Comment Mode（インライン / Markdown コメント）と Zero-Finding Approval
- SQLite + volume による永続化（Docker Compose 1 サービス想定）

用語・ドメインの正本は [`CONTEXT.md`](CONTEXT.md) を参照してください。

## Requirements

- **Go** 1.25+（ローカルビルド時）
- **Git** >= 2.41（OCR および Repo Mirror / Worktree 用）
- **OCR CLI** >= v1.8.7（`--provider` / `--model` 選択用）
- Docker / Compose（配布イメージ利用時）

詳細は [`docs/runtime-requirements.md`](docs/runtime-requirements.md) を参照してください。

## Quick start (Docker Compose)

```bash
cp .env.example .env
# RM_ADMIN_USER / RM_ADMIN_PASSWORD / RM_ENCRYPTION_KEY を編集
# RM_ENCRYPTION_KEY は 32 バイト以上

docker compose up -d
```

既定では `http://localhost:8088` で WebUI が開きます（コンテナ内は `:8080`）。

イメージ: [`ghcr.io/jo3qma/ocr-mng`](https://github.com/jo3qma/ocr-mng/pkgs/container/ocr-mng)

## Configuration

| 環境変数 | 必須 | 説明 |
|---|---|---|
| `RM_ADMIN_USER` | Yes | Administrator のユーザー名 |
| `RM_ADMIN_PASSWORD` | Yes | Administrator のパスワード |
| `RM_ENCRYPTION_KEY` | Yes | Host/Repo PAT・LLM API キー暗号化用（32 バイト以上） |
| `RM_LISTEN_ADDR` | No | 待受アドレス（既定 `:8080`） |
| `RM_DATA_DIR` | No | SQLite・ミラー等のデータディレクトリ（既定 `/data`） |
| `RM_OCR_BINARY` | No | OCR 実行ファイル名（既定 `ocr`） |

`.env.example` も参照してください。

## Development

```bash
make build   # bin/rm
make lint
make test
make docker  # ocr-mng:local
```

ローカル実行例:

```bash
export RM_ADMIN_USER=admin
export RM_ADMIN_PASSWORD=change-me
export RM_ENCRYPTION_KEY=01234567890123456789012345678901
export RM_DATA_DIR=./data
export RM_LISTEN_ADDR=:8080
# PATH に ocr（>= v1.8.7）と git（>= 2.41）があること
./bin/rm
```

## Docs

- [`CONTEXT.md`](CONTEXT.md) — ドメイン言語
- [`docs/adr/`](docs/adr/) — 設計判断
- [`docs/ocr-review-output.md`](docs/ocr-review-output.md) — OCR JSON 出力
- [Open Code Review 公式ドキュメント](https://open-codereview.ai/docs)

## License

本リポジトリ（Review Manager）は [MIT License](LICENSE) です。

依存する [Open Code Review](https://github.com/alibaba/open-code-review) 本体は **Apache-2.0** です。OCR の利用・再配布にあたっては OCR 側のライセンス条件に従ってください。
