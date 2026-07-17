# Per-Review-Run OCR home isolation

Review Run ごとに Open Code Review CLI の `HOME`（`.opencodereview/config.json` を含む作業ホーム）を隔離する。共有の単一 `ocr-home` に書き込むと、Repo OCR Overrides で Registered LLM Provider が異なる並行 Review Run が同じ config を上書きし合い、API キーが他 Run に見える。実行ミューテックスで直列化する案は Review Concurrency の意味を薄めるため採用しない。

**Considered Options:** 共有 home + 実行ミューテックス / 共有 home のまま（同一 Provider 前提の既知制限）

**Consequences:** Review Run 終了後に隔離ホームを削除できる。実装は `DataDir` 配下の Run 単位ディレクトリなどになるが、パス形状は ADR の対象外とする。
