# Monolith Go process with OCR subprocess

Review Manager は WebUI・PR ポーラー・Review Run スケジューラを単一 Go プロセスにまとめ、OCR 実行のみ alibaba/open-code-review CLI を subprocess で起動する。Docker Compose では 1 サービス + volume で運用し、Review Concurrency は goroutine + semaphore で制御する。Worker 分離やジョブキューは初期版では採用しない。

**Considered Options:** 単一プロセス（OCR 組み込み） / Web + Worker 2 コンテナ / Redis キュー付き分散

**Consequences:** OCR 長時間実行は同一プロセス内でブロックしないよう subprocess + 並行上限で管理する。将来スケールが必要になった場合、Review Run テーブルをキューとして Worker コンテナへ split できる。
