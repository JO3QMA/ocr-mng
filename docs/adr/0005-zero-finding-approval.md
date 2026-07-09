# Zero-Finding Approval on GitHub (repo opt-in)

OCR Review Output の `comments` が空の Review Run（Zero-Finding Review）がコメント投稿まで成功したあと、Registered Repo で有効化されている場合に Review Manager が GitHub Pull Request レビュー API で `APPROVE` を投稿する。デフォルトは無効（Repo ごとオプトイン）。Approve は Post-Review Label Removal の前に行い、失敗時は Review Run Success にせず Label も除去しない。Gitea Registered Repo では Approve をスキップし、コメント投稿まで成功すれば Review Run Success とする。

Review Comment Mode が inline のときはサマリー付き PR レビュー 1 回の `event` を `APPROVE` にする。comment モード、および inline の `APPROVE` 失敗後の Issue コメントフォールバックでは、Issue コメントにサマリーを載せたあと別途 `APPROVE` を 1 回投稿する（Approve body は Review Language の短い定型文のみ）。同一 Pull Request で Review Run が複数回 Zero-Finding になった場合も毎回 Approve する。指摘が 1 件以上ある Review Run では従来どおり `COMMENT` のみとし、過去の Approve を取り消す処理は行わない。

**Considered Options:** Global デフォルト ON / Gitea でも Approve / `COMMENT` と `APPROVE` の二重投稿 / 指摘出現時に `CHANGES_REQUESTED` または Approve 撤回 / マージまで自動化

**Consequences:** Repo PAT に Pull Request レビュー投稿権限が必要。ブランチ保護の required reviewers を Bot Approve だけで満たす保証はない。Gitea Repo で設定 ON でも Approve は行われないため UI に GitHub 限定の注記が必要。`githost.CreateInlineReview` の `event` 固定 `COMMENT` を呼び分け可能にする。
