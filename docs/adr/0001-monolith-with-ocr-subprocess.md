# Monolith Go process with OCR subprocess

Review Manager は WebUI・PR ポーラー・Review Run スケジューラを単一 Go プロセスにまとめ、OCR 実行のみ alibaba/open-code-review CLI を subprocess で起動する。Docker Compose では 1 サービス + volume で運用し、Review Concurrency は goroutine + 並行上限で制御する。Redis や外部ジョブキュー、Worker コンテナ分離は初期版では採用しない。並行上限超過時に Review Run を捨てないため、`review_runs.status = pending` を SQLite に永続し、同一プロセス内のディスパッチャが FIFO で消化する（分散キューではない）。

**Considered Options:** 単一プロセス（OCR 組み込み） / Web + Worker 2 コンテナ / Redis キュー付き分散

**Consequences:** OCR 長時間実行は同一プロセス内でブロックしないよう subprocess + 並行上限で管理する。`pending` はプロセス再起動後も再スケジュールし、`running` で中断された Run のみ failed 化する。将来スケールが必要になった場合、Review Run テーブルをキューとして Worker コンテナへ split できる。
