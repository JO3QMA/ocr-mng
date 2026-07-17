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
Pull Request に対して OCR レビューを起動する条件。Label が新たに付与された瞬間に 1 回実行し、UI からの手動再レビューでも起動できる。手動トリガーは HTTP リクエスト処理中に `pending` Review Run を DB へ同期的に作成してから応答する（インメモリチャネルだけに載せない）。
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
Review Manager に登録され、ポーリング・OCR レビューの対象となる Git リポジトリ。Trigger Label や Repo 固有設定を持つ。Administrator 向けの識別表示は `Owner/Name`（例: `acme/app`）とし、内部通番（Repo ID）は表示の主ラベルにしない。
_Avoid_: 監視対象, ターゲットリポジトリ, リポジトリ名（Owner なしの Name 単独を指す用法）

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
1 回の OCR レビュー実行の記録。対象 Pull Request、受付日時、開始日時、終了日時、成否、投稿先、OCR 出力を含む。受付日時は Review Run が記録され Review Concurrency の空き待ちに入った時刻であり、開始日時（実際に実行が始まった時刻）とは別である。終了日時は実行が `success` / `failed` で終わった時刻である。`pending` は空き待ち、`running` は実行中、`success` / `failed` は終了状態。実行時に使う LLM は Registered LLM Provider と Registered LLM Model の組ちょうど 1 つに解決される（モデルローテーションは別概念）。解決できた場合はその時点のプロバイダー名・モデル名をスナップショットとして保持する。解決できない（未設定・無効・API キー欠落等）場合は実行開始時に `failed` とする。Administrator 向けに示す Registered Repo の識別（`Owner/Name`）はスナップショットせず、紐づく Registered Repo の現在値を使う（LLM 名のスナップショットとは別）。Administrator 向け一覧では開始日時を主時刻として示し、未開始（`pending`）は空とする（プレースホルダ文言は出さない）。一覧の並びは受付順（新しいもの優先）のままとする。Review Run 詳細では受付日時・開始日時・終了日時を示す（未開始・未終了は空）。UI Language が English のときは Accepted / Started / Finished と対応させる。
_Avoid_: ジョブ, タスク（曖昧）, 作成日時（開始日時または受付日時の意で使う用法）, 実行日時（開始日時との混同）, 開始時刻・終了時刻（開始日時・終了日時と同義の別表記）

**Pending Review Run**:
Review Concurrency の空きを待っている Review Run。SQLite に永続化され、Review Manager Process 再起動後も再スケジュールされる（`running` で中断された Run は再起動時に `failed` 化する）。同一 Registered Repo × 同一 Pull Request に `pending` または `running` が既にあるときは、新たな Review Run を作らない。実行開始時に Git Host から Pull Request を再取得し、その時点の HEAD・base・本文でレビューする（キュー投入時の HEAD に固定しない）。Trigger Label の有無は実行ゲートにはしない。
_Avoid_: キューアイテム, ジョブ（曖昧）

**Pull Request Snapshot**:
Review Trigger 判定と重複防止のため Pull Request ごとに保持する最小状態。Trigger Label の有無のみを持つ。タイトル・本文は含めない（Review Run 実行時に Git Host API から都度取得する）。ラベル遷移（off→on）では、`pending` Review Run の作成に成功してから Snapshot を更新する。
_Avoid_: PR 状態, キャッシュ（曖昧）

**Review Concurrency**:
同時実行できる Review Run の上限。Registered Repo ごとには 1 件までとし、システム全体では UI 設定可能な最大並行数を超えない。`pending` の消化は受付日時昇順の FIFO とし、対象 Registered Repo に既に `running` がある場合はその Run を飛ばして次を選ぶ。
_Avoid_: ワーカー数, 並列度（曖昧）

**Registered LLM Provider**:
Review Manager に明示的に登録された LLM 接続先。表示名、OCR 上のプロバイダー識別子、種別（builtin / custom）、接続情報、暗号化された API キー、Registered LLM Model の台帳を持つ。builtin は OCR 組み込みプロバイダー、custom は自前エンドポイント。Global デフォルトまたは Repo OCR Overrides から参照されているあいだは削除できない（無効化は可）。Git Host（Git プラットフォーム）とは別概念。
_Avoid_: プロバイダー（単独・Git Host と混同）, LLM Backend, Model Endpoint

**Registered LLM Model**:
Registered LLM Provider に属する利用可能モデル 1 件。OCR に渡すモデル識別子と、選択肢としての有効/無効を持つ。Global デフォルトまたは Repo OCR Overrides の組に選ぶには、有効かつその Provider 配下であることが必要。参照されているあいだは削除できない（無効化は可）。実行開始時に無効なら Review Run は `failed`。自由文字列のモデル名上書き（旧 `ocr_model`）は移行期間の互換用であり、台帳運用開始後は廃止する。
_Avoid_: モデル（単独・曖昧）, Provider Model, モデルプール（#46 のローテーション用集合）

**Global OCR Settings**:
Review Manager が保持する Open Code Review CLI 向けのグローバル LLM 設定。正はデフォルトの Registered LLM Provider / Registered LLM Model の組であり、レビュー実行時に OCR の config へ反映される。Global デフォルトは Provider と Model の両方が揃って初めて有効（片方だけの設定は不可）。組が一度も設定されていないあいだだけ、移行期間として従来の OCR Config JSON・Repo のモデル名文字列・（残存する `OCR_LLM_*`）で実行する。組を一度設定した以降は台帳モードで一方通行とし、デフォルト組のクリアは不可（別の組への入替のみ）。生 JSON・旧モデル文字列・`OCR_LLM_*` は廃止対象とする。
_Avoid_: グローバル設定（曖昧）, Global OCR Config JSON（移行後の正式名にしない）

**Repo OCR Overrides**:
Registered Repo ごとに Global OCR Settings を上書きするレビュー実行パラメータ。Registered LLM Provider、Registered LLM Model、カスタムルール、追加コンテキスト（requirement）、Review Language を含む。Provider / Model の上書きは組単位（両方空＝Global に従う、両方指定＝その組。片方だけは不可）。組のクリア（両方空へ戻す）は可。旧モデル名文字列（`ocr_model`）は移行期間の互換用とする。
_Avoid_: Repo 設定（曖昧）

**Review Background**:
Review Run 実行時に Open Code Review CLI の `--background` に渡す結合済みテキスト。先頭に PR Description Context、続けて Registered Repo の OCR Requirement（空でなければ）を結合する。言語別のデフォルト requirement は注入しない。セクション見出しと Title/Body ラベルは Review Language に合わせる。
_Avoid_: プロンプト, prompt（OCR テンプレート用語との混同）

**PR Description Context**:
Review Background に含める Pull Request のタイトルと本文。コード diff とは別に LLM へ渡し、変更の意図をレビューに反映させる。本文が空の場合はタイトルのみ渡す。Registered Repo ごとの ON/OFF は設けず、Review Run では常に含める。本文は先頭から 8,000 ルーンで切り詰め、超過分は省略マーカーで示す。タイトルも本文も取得できない場合は PR Description Context ごと省略し、requirement のみでレビューを続行する。
_Avoid_: PR プロンプト, PR メタデータ（曖昧）

**OCR Review Output**:
Open Code Review CLI が `--format json` で返す構造化レビュー結果。`comments` 配列の各要素に `path`, `content`, `suggestion_code`, `existing_code`, `start_line`, `end_line`, `category`, `severity` を含む。スキーマと抽象化サンプルは [`docs/ocr-review-output.md`](docs/ocr-review-output.md) を参照。
_Avoid_: OCR JSON, レビュー結果（曖昧）

**Comment Category**:
OCR Review Output の各指摘が属する分類ラベル（例: `maintainability`, `style`）。Review Manager は値を解釈・フィルタ・翻訳せず、OCR の生文字列を Review Comment Wrapper にパススルー表示する。
_Avoid_: カテゴリ（単独・曖昧）, 指摘種別（曖昧）

**Comment Severity**:
OCR Review Output の各指摘の深刻度ラベル（例: `low`, `medium`, `high`）。Review Manager は値を解釈・フィルタ・翻訳せず、OCR の生文字列を Review Comment Wrapper にパススルー表示する。Zero-Finding Review や Zero-Finding Approval の判定には使わない。
_Avoid_: 重要度（曖昧）, priority（OCR 用語との混同）

**Review Comment Wrapper**:
Review Manager が OCR 出力を Git Host に投稿する際に付与する Markdown の定型部分（見出し、指摘説明、Suggestion のコード表示、Warnings、件数サマリー、各指摘本文先頭の Comment Category / Comment Severity メタ行、および件数サマリー直後の Severity 内訳・Category 内訳の別リスト等）。件数サマリーと内訳は Review Comment Mode が inline / comment のどちらの場合も先頭に出す。GitHub インラインでは GitHub Suggestion Block を使い、それ以外では Fallback Code Fence と提案ラベルを使う。OCR 本文と同様に Review Language に合わせる。欠落した Category / Severity は表示せず、ある方だけ出す（表示前に前後空白を除き、空なら欠落）。各指摘本文先頭のメタ行は太字ラベル＋値＋中黒形式（severity を先）。ラベル語は Review Language に合わせ、Japanese / English / Chinese でそれぞれ「深刻度 / Severity / 严重程度」「分類 / Category / 分类」。値はパススルー（表示時のみ Markdown メタ文字をエスケープ）。片方だけならその側のみ。内訳は件数サマリー直後に Severity・Category を別行で出し（例: `**深刻度:** medium 2, low 1`）、その次元に値が一つも無ければ行ごと省略する。内訳は OCR 生文字列ごとの件数で、欠落分は内訳に含めない（総件数と内訳合計の不一致は許容）。内訳の並びは件数降順、同数なら文字列昇順とする。
_Avoid_: 投稿テンプレート, コメントヘッダー（曖昧）

**GitHub Suggestion Block**:
GitHub インライン review 専用の Markdown コードフェンス（`suggestion` タグ）で OCR の suggestion_code を包む表示形式。Pull Request 上で Apply ボタンを出す。existing_code は表示しない（diff 上の該当行が現状として見えるため）。投稿前に suggestion_code の先頭・末尾の改行のみ除去し、インデント（スペース・タブ）は保持する。
_Avoid_: suggestion 構文, コード提案（曖昧）

**Fallback Code Fence**:
GitHub インライン以外（Review Comment Mode が comment、インライン投稿失敗時のフォールバック、Gitea）で suggestion_code を表示する通常の Markdown コードフェンス。Registered Repo のファイル拡張子から言語タグを推定する。提案ラベル（**提案:** 等）を直前に付ける。
_Avoid_: 通常コードブロック, プレーンテキスト提案（曖昧）

**General Review Comment**:
行番号もファイルパスも持たない OCR 指摘。Pull Request 全体へのフィードバックを表し、Review Comment Wrapper では `(general)` 相当のラベルで見出す。
_Avoid_: 全体コメント, 総評（曖昧）

**Unresolved File Path Comment**:
行番号はあるが `path` が欠落した OCR 指摘。Review Comment Wrapper では `(general)` ではなく、Review Language ごとの「ファイル不明」ラベルで見出す。
_Avoid_: general コメント, パスなしコメント（曖昧）
**Review Language Scope**:
Review Language は Global Settings にデフォルトを持ち、Registered Repo の Repo OCR Overrides で上書きできる。UI Language は Global Settings のみで設定し、Repo ごとの上書きはしない。専用 UI で設定した Review Language は、レビュー実行時に組み立てる OCR config の language として注入する（移行期間中は旧 OCR Config JSON 内の language より常に優先）。
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
OCR 実行、レビュー結果の投稿、Zero-Finding Approval（有効時）、Post-Review Label Removal（有効時）がすべて完了した Review Run の状態。このときのみ Pull Request Snapshot を更新し、Label 除去を行う。いずれかが失敗した場合は Review Run Success にならず、Post-Review Label Removal も実行しない。
_Avoid_: 完了, 成功（曖昧）

**Zero-Finding Review**:
OCR Review Output の `comments` 配列が空の Review Run。`warnings` や `message` の有無は指摘件数の判定に含めない。
_Avoid_: 指摘なしレビュー, クリーンレビュー（曖昧）

**Zero-Finding Approval**:
Zero-Finding Review でコメント投稿が成功したあと、Post-Review Label Removal の前に Review Manager が Git Host の Pull Request レビュー API で Approve を投稿する動作。Registered Repo ごとにオプトインでき、デフォルトは無効。GitHub の Registered Repo のみ対象とし、Gitea ではスキップする（コメント投稿まで成功すれば Review Run Success）。Review Comment Mode が inline のときは、サマリー付き PR レビュー 1 回の `event` を `APPROVE` にする（`COMMENT` との二重投稿はしない）。Review Comment Mode が comment のときは、先に `APPROVE` レビュー（Review Language の短い定型文）を投稿し、続けて Issue コメントでサマリーを投稿する（Approve 失敗時のリトライで Issue コメントが重複しないよう順序を固定）。inline で `APPROVE` 投稿が失敗したときは Issue コメントへフォールバックするが、Zero-Finding Approval 有効時はフォールバックでも先に `APPROVE` を試してから Issue コメントを投稿する。同一 Pull Request で Review Run が複数回成功した場合も同じルールを適用し、Zero-Finding のたびに Approve を投稿する。指摘が 1 件以上ある Review Run では `COMMENT` のみとし、過去の Approve を取り消す処理は行わない。Approve が失敗した場合は Review Run Success にならない。
_Avoid_: 自動承認, オートマージ（曖昧）

**Global Settings**:
Review Manager 全体に適用される運用設定。Poll Interval デフォルト、Review Concurrency 上限、Review Run Retention、デフォルトの Registered LLM Provider / Registered LLM Model、Global OCR Settings 等を含む。Registered Repo 設定で個別に上書きできる項目がある。
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

**Settings Toggle**:
Review Manager WebUI で永続的な有効/無効やオプションの on/off を表す二択コントロール。カード内の縦フォーム行ではラベル左・コントロール右。LLM Model のインライン編集行ではコンパクト配置（横並び）を例外として許す。状態の意味はフィールドラベルが担い、ノブ横に on/off 文言は置かない。ON 色は Primary Action Button と同じ青（`#2563eb`）。Registered Repo・Registered LLM Provider・Registered LLM Model の有効状態、Post-Review Label Removal、Zero-Finding Approval が該当する。
_Avoid_: スイッチ（曖昧）, トグルボタン, チェックボックス（一回限り操作との混同）

**Confirmation Checkbox**:
Review Manager WebUI で保存時に一度だけ行う破壊的操作の確認用チェック。行はコントロール左・ラベル右。Host PAT / Repo PAT / API キーのクリアが該当する。永続状態の on/off には使わない。
_Avoid_: クリアチェック, 削除チェック（曖昧）
