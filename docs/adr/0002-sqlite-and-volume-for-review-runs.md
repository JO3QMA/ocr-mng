# SQLite metadata with volume-stored OCR output

Review Manager の永続化は SQLite（メタデータ・設定・Pull Request Snapshot）と Docker volume 上の JSON ファイル（Review Run の OCR 生出力）の二層とする。Postgres や JSONB 単体 DB は Administrator 1 人の Compose 運用に対して過剰なため採用しない。Review Run Retention 期限後は JSON ファイルを削除し、Run レコードはサマリーのみ残す。

**Consequences:** バックアップは SQLite ファイル + volume をセットで取得する必要がある。全文検索が必要になった時点で Postgres 移行または外部インデックスを検討する。
