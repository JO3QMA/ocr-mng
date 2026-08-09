# LLM Rotation Set as the unit of LLM selection

Review Run が使う LLM は、モデル名文字列のリストではなく、Registered LLM Provider と Registered LLM Model の組からなる **LLM Rotation Set** から選ぶ。Global OCR Settings と Repo OCR Overrides の LLM 正本はこの集合であり、要素数 1 が従来の単一組指定と同一。Repo の集合は Global を完全置換する（結合・差分にしない）。MVP の選択は実行開始時の round-robin（LLM Rotation）で、カーソルは有効な集合ごと（Global 共有 / Repo 上書きは別）。同一 Review Run 内のフェイルオーバー再実行はしない。

**Considered Options:** 同一 Provider 内のモデル名だけを回す / 旧 `ocr_model` 文字列プール / 単一組と集合の二重正本またはモード切替 / Repo 集合を Global に結合 / random または primary+fallback / 受付時に組を固定 / カーソルを Repo 単位・システム全体・PR 単位にする / 同一 Run 内で次組へリトライ / 使えぬ組で即 `failed` / 集合保存のたびにカーソルリセット / 集合内の組の重複を許す / Global のみ複数組 UI

**Consequences:** 台帳の参照保護は LLM Rotation Set の全要素に及ぶ。使えぬ組はスキップして一周し、全滅なら実行開始時 `failed`。集合の内容または順序が実際に変わったときだけ LLM Rotation Cursor を先頭へ戻す。Cursor は WebUI に出さず、監査は Review Run の Provider / Model スナップショットに任せる。Global / Repo の WebUI は順序付き複数組の編集が必要になる。
