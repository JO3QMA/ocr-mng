# Inline review line clamping to PR diff

Review Comment Mode が inline のとき、Review Manager は OCR Review Output の `start_line` / `end_line` をそのまま Git Host のインラインアンカーに使わない。Review Worktree 上の `git diff`（Review Base Ref…head SHA）から得た **diff 上の行番号**との交差にクランプし、Git Host が解決できる行だけをアンカーにする。

交差が空のコメントはインラインバッチから外し、Review Comment Wrapper のサマリー本文へ回す（General Review Comment や Unresolved File Path Comment と同様の「インライン不可 → サマリー」扱い）。複数行アンカーは交差範囲の先頭を `start_line`、末尾を `line` とする。単行のときは `line` のみ送る。

`CreatePullRequestReview` がそれでも失敗した場合（PAT 権限、commit 不一致など）は従来どおり Issue コメント（Review Comment Mode が comment と同じ単一 Markdown）へフォールバックする。Review Run は Success のままとし、理由は `post_warning` と構造化ログに残す。

**Considered Options:** `end_line` を常にアンカーにする / Git Host API の patch を投稿時に取得 / 失敗時に Review Run を failed にする / 解決不能行を黙って捨てる

**Consequences:** OCR の行番号はファイル全体基準のため、diff 外の `end_line` を返すことがある。Review Manager が diff クランプを担う。`review_runs.post_warning` で Administrator が WebUI からフォールバック理由を確認できる。
