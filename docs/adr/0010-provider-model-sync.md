# Provider Model Sync

Registered LLM Model の正本を「手動追加＋任意の一覧取得」から「ページ開き時の API 同期メイン＋手動サブ」へ移す。

## 決定

- プロバイダー編集画面の GET と「再同期」POST で、接続先 models API を叩き、結果を **即時に台帳へ反映**する。
- 台帳行に `source`（`api` | `manual`）と `is_new` を持つ。既存行はマイグレーションで `manual`。
- `api` 行: 新規は `enabled=false` + **New!!**（`is_new`）。有効化保存で `is_new` クリア。削除 UI なし。
- `manual` 行: 手動追加のみ。名前編集・削除可。API 一覧に存在する名前は追加拒否。
- API 一覧から消えた `api` 行は DELETE。Global / Repo の LLM Rotation Set から当該組を **自動除去**（Global が空になることは許容）。
- API 取得失敗時は台帳を変更しない。URL/キー無しは同期スキップ。

## Considered Options

- 一覧取得は任意 POST のみ・台帳は変えない（旧 Provider Model Discovery）
- 同期は表示のみで保存時に反映
- API 行も手動削除可能
- Rotation 参照中は API 行の自動削除をブロック

## Consequences

- `POST .../models/discover` を `.../models/resync` に置き換え。
- UI は API セクション（チェックボックス有効化・全選択/全解除）と手動セクションの二段構成。
- Global Rotation が同期で空になった場合、設定保存は Administrator が後から直す必要がある。
- Repo Rotation が空になった場合は既存ルールどおり Global に従う。
