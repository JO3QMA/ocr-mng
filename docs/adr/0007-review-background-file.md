# Review Background File alongside Review Background

Open Code Review の `--background-file` を Registered Repo の Repo OCR Overrides から使えるようにする。パスはリポジトリルート相対の単一値で、Global Settings には持たない。Review Background（PR Description Context + OCR Requirement → `--background`）は従来どおり組み立て、ファイルが Review Worktree 上の通常ファイル（非 symlink）として存在するときだけ追加で `--background-file` を渡す。OCR 本体の結合順（インラインが先・ファイルが後）に従い、OCR Requirement や Review Background をファイルで置き換えない。

パス設定はあるが欠落、または通常ファイルでない（ディレクトリ・シンボリックリンク等）ときはフラグを付けず Review Background のみで続行し、プロセスログに警告する（Review Run の UI・PR コメント・ErrorMessage には出さず、それだけでは `failed` にしない）。通常ファイルを渡したあと OCR が内容を拒否した場合（空・サイズ上限・予約タグ等）は既存の OCR 失敗経路で Review Run を `failed` にし得る。OCR の文字数／バイト制限は Review Manager 側で再実装しない。`--background-file` 未対応の OCR に対する実行前バージョンゲートも行わない。経路上の中間シンボリックリンクで Worktree 外へ出るパスも、最終成分のシンボリックリンクと同様に使わない（解決後パスが Worktree 内に収まること）。

設定値は保存時に検証・正規化する。空は未設定。絶対パス、正規化後にリポジトリ外へ出る相対パス、改行を含む値は受け付けない。サーバー上の絶対パス指定と、ファイル内容の WebUI 編集は対象外。

**Considered Options:** ファイル存在時に OCR Requirement を `--background` から外す／ファイルと OCR Requirement を排他にする / 欠落時に Review Run を `failed` にする / 成功 Run や PR コメントに soft warning を載せる / symlink を EvalSymlinks 後 worktree 内なら許可する / 実行前に OCR バージョンでフラグを抑制する / Global デフォルトパスを持つ

**Consequences:** ブランチによってファイル有無が揺れる運用でもレビューは止まらないが、設定ミスはログを見ないと気づきにくい。PR HEAD 上の symlink 差し替えによるホストファイル読み取りは、symlink 拒否で防ぐ。古い OCR ではフラグ付与後に実行失敗し得る（配布イメージは latest 前提）。issue #68 の「ocr_requirement との優先順位」は置換ではなく併存として文書化する。
