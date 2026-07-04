# OCR Management

GitHub / Gitea 上の Pull Request を Open Code Review CLI で自動レビューし、結果を PR コメントとして投稿する管理 WebUI。

## Language

**Open Code Review (OCR)**:
Alibaba 製の AI コードレビュー CLI（`alibaba/open-code-review`）。Git diff を LLM に送り、構造化されたレビュー結果を生成する。
_Avoid_: OpenCodeReview（曖昧な総称）, ocr（CLI コマンド名との混同）

**Review Manager**:
本リポジトリ（`ocr-mng`）が提供するシステム全体。Repo 登録、設定管理、PR ポーリング、OCR 実行、コメント投稿を担う。
_Avoid_: OCR Manager, ocr-mng（実装名）

**Review Trigger**:
Pull Request に対して OCR レビューを起動する条件。Label が新たに付与された瞬間に 1 回実行し、UI からの手動再レビューでも起動できる。
_Avoid_: ポーリングトリガー, 自動レビュー（曖昧）

**Trigger Label**:
Repo ごとに設定する Label 名。Pull Request にこの Label が付いていることが Review Trigger の前提条件になる。
_Avoid_: レビューラベル, ターゲットラベル

**Post-Review Label Removal**:
レビュー完了後、Repo 設定で有効な場合に Trigger Label を Pull Request から外すオプション動作。PAT に Label 変更権限がある Repo のみ実行する。
_Avoid_: ラベル自動削除, ラベルクリーンアップ

**Review Comment Mode**:
OCR 結果を Pull Request に投稿する方式。Repo ごとに設定でき、デフォルトは行単位のインラインレビュー。Gitea や PAT 権限不足など投稿できない場合は Markdown 単一コメントへフォールバックできる。
_Avoid_: コメント形式, 投稿モード

**Git Host**:
GitHub または Gitea の API エンドポイント。Registered Repo が属するホスト単位で PAT を保持できる。
_Avoid_: プロバイダー, プラットフォーム（曖昧）

**Registered Git Host**:
Review Manager に明示的に登録された Git Host。API ベース URL、Host PAT、ホスト種別（GitHub / Gitea）を持つ。Registered Repo は必ずいずれかの Registered Git Host に属する。
_Avoid_: リモート, サーバー（曖昧）

**Host PAT**:
Git Host 単位のデフォルト Personal Access Token。同一ホスト上の Registered Repo は、個別 PAT 未設定時にこれを使う。保存時は Review Manager のマスターキーで暗号化される。
_Avoid_: グローバルトークン, 共通 PAT

**Repo PAT**:
Registered Repo に紐づく Personal Access Token。設定されている場合 Host PAT より優先して使う。保存時は Review Manager のマスターキーで暗号化される。
_Avoid_: リポジトリトークン

**Administrator**:
Review Manager WebUI にログインできる単一の運用者。Basic Auth またはセッション Cookie で認証する。
_Avoid_: ユーザー, オペレーター（曖昧）

**Registered Repo**:
Review Manager に登録され、ポーリング・OCR レビューの対象となる Git リポジトリ。Trigger Label や Repo 固有設定を持つ。
_Avoid_: 監視対象, ターゲットリポジトリ

**Repo Mirror**:
Registered Repo ごとに保持する bare Git リポジトリ。レビュー前に fetch して最新状態を反映する。
_Avoid_: クローン, キャッシュ（曖昧）

**Review Worktree**:
1 回のレビュー実行ごとに Repo Mirror から切り出す作業ディレクトリ。OCR CLI の `--repo` に渡す。
_Avoid_: 作業コピー, チェックアウト（曖昧）

**Poll Interval**:
Registered Repo の Pull Request を Git Host API で確認する周期。グローバルデフォルトがあり、Repo ごとに上書きできる。システム全体で設定可能な最小間隔未満には設定できない。
_Avoid_: フェッチ間隔, スキャン間隔

**Review Run**:
1 回の OCR レビュー実行の記録。対象 Pull Request、開始・終了時刻、成否、投稿先、OCR 出力を含む。
_Avoid_: ジョブ, タスク（曖昧）

**Pull Request Snapshot**:
Review Trigger 判定と重複防止のため Pull Request ごとに保持する最小状態。Trigger Label の有無、最後にレビューした head commit、最後の Review Run への参照。
_Avoid_: PR 状態, キャッシュ（曖昧）

**Review Concurrency**:
同時実行できる Review Run の上限。Registered Repo ごとには 1 件までとし、システム全体では UI 設定可能な最大並行数を超えない。
_Avoid_: ワーカー数, 並列度（曖昧）

**Global OCR Settings**:
Review Manager が保持する Open Code Review CLI の LLM Provider 設定。コンテナ内の OCR グローバル config に反映される。
_Avoid_: グローバル設定（曖昧）

**Repo OCR Overrides**:
Registered Repo ごとに Global OCR Settings を上書きするレビュー実行パラメータ。モデル名、カスタムルール、追加コンテキスト（requirement）、Review Language を含む。
_Avoid_: Repo 設定（曖昧）

**Review Comment Wrapper**:
Review Manager が OCR 出力を Git Host に投稿する際に付与する Markdown の定型部分（見出し、Suggestion ラベル、Warnings、件数サマリー等）。OCR 本文と同様に Review Language に合わせる。
_Avoid_: 投稿テンプレート, コメントヘッダー（曖昧）

**General Review Comment**:
行番号もファイルパスも持たない OCR 指摘。Pull Request 全体へのフィードバックを表し、Review Comment Wrapper では `(general)` 相当のラベルで見出す。
_Avoid_: 全体コメント, 総評（曖昧）

**Unresolved File Path Comment**:
行番号はあるが `path` が欠落した OCR 指摘。Review Comment Wrapper では `(general)` ではなく、Review Language ごとの「ファイル不明」ラベルで見出す。
_Avoid_: general コメント, パスなしコメント（曖昧）
**Review Language Scope**:
Review Language は Global Settings にデフォルトを持ち、Registered Repo の Repo OCR Overrides で上書きできる。UI Language は Global Settings のみで設定し、Repo ごとの上書きはしない。専用 UI で設定した Review Language は、Global OCR Config JSON 内の `language` より常に優先し、レビュー実行時に config へ注入する。
_Avoid_: 言語設定（曖昧）
_Avoid_: 言語設定（曖昧）

**Review Base Ref**:
`ocr review --from` に渡すマージ先参照。Pull Request の base branch を優先し、取得できない場合は Registered Repo に設定したデフォルトブランチを使う。
_Avoid_: ベースブランチ, ターゲットブランチ（曖昧）

**Review Run Retention**:
Review Run に紐づく OCR 出力ファイルを保持する日数。期限を過ぎたファイルは削除され、Review Run レコードはサマリー情報のみ残すか削除する。
_Avoid_: ログローテーション, データ保持（曖昧）

**Review Manager Process**:
Review Manager を構成する単一の Go プロセス。WebUI、PR ポーリング、Review Run スケジューリングを担い、OCR 実行は Open Code Review CLI を subprocess として起動する。
_Avoid_: サーバー, バックエンド（曖昧）

**Review Run Success**:
OCR 実行、レビュー結果の投稿、Post-Review Label Removal（有効時）がすべて完了した Review Run の状態。このときのみ Pull Request Snapshot を更新し、Label 除去を行う。
_Avoid_: 完了, 成功（曖昧）

**Global Settings**:
Review Manager 全体に適用される運用設定。Poll Interval デフォルト、Review Concurrency 上限、Review Run Retention、Global OCR Settings 等を含む。Registered Repo 設定で個別に上書きできる項目がある。
_Avoid_: システム設定（曖昧）

**Review Language**:
Open Code Review が Pull Request に投稿するレビューコメントの言語。UI Language とは独立に設定する。選択肢は Japanese / English / Chinese（OCR config の `language` に渡す値）。Global デフォルトは Japanese（`review_language`）。Registered Repo は `review_language` 列で上書きでき、空なら Global に従う。
_Avoid_: コメント言語, OCR 言語（曖昧）

**UI Language**:
Administrator が Review Manager WebUI で見る表示言語。選択肢は ja / en。デフォルトは ja。Global Settings（`ui_language`）のみで設定し、Registered Repo では上書きしない。
_Avoid_: 表示言語, ロケール（曖昧）

**Primary Action Button**:
Review Manager WebUI で設定の確定や新規リソース作成へ進むボタン。保存・Host 追加・Repo 追加に使う。色は青（`#2563eb`）。
_Avoid_: メインボタン, 送信ボタン（曖昧）

**Accent Action Button**:
Review Manager WebUI で副作用を伴う即時実行に使うボタン。手動 Review Run が該当する。Primary Action Button（保存・追加）とは色を分け、緑（`#16a34a` 前後）を使う。
_Avoid_: 実行ボタン, アクションボタン（曖昧）

**Secondary Action Button**:
Review Manager WebUI で変更を破棄し、一覧や前画面へ戻るボタン。キャンセルに使う。色は灰（`#6b7280`）。
_Avoid_: 戻るボタン, リンクボタン（曖昧）
