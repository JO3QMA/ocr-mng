# OCR Review Output

Open Code Review CLI が `--format json` で返すレビュー結果の形式。Review Manager はこの JSON をパースし、Git Host 向けコメントに変換する。

実装: [`internal/ocr/runner.go`](../internal/ocr/runner.go)（パース）、[`internal/review/format.go`](../internal/review/format.go)（投稿用 Markdown 生成）。

## トップレベル

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `status` | string | 実行結果（例: `"success"`） |
| `summary` | object | レビュー統計（`files_reviewed`, `comments`, `total_tokens` 等） |
| `tool_calls` | object | OCR が使用したツール呼び出し数 |
| `comments` | array | 指摘コメントの配列（下記） |
| `warnings` | array | 警告メッセージ（任意） |
| `message` | string | コメント 0 件時のメッセージ（任意） |

## コメント要素

| JSON キー | Go フィールド | 説明 |
|-----------|--------------|------|
| `path` | `FilePath` | 対象ファイルパス（リポジトリルート相対） |
| `content` | `Content` | 指摘本文（Markdown 可） |
| `suggestion_code` | `Suggestion` | 提案コード。GitHub Suggestion Block にそのまま入る |
| `start_line` | `StartLine` | 対象行の開始（1-based） |
| `end_line` | `EndLine` | 対象行の終了（1-based）。未指定時は `start_line` を使う |
| `category` | `Category` | 指摘の分類ラベル（任意。例: `maintainability`, `style`）。Review Comment Wrapper にパススルー表示 |
| `severity` | `Severity` | 指摘の深刻度ラベル（任意。例: `low`, `medium`, `high`）。Review Comment Wrapper にパススルー表示 |

`path` と `start_line` / `end_line` が揃っているコメントは GitHub インラインレビューに投稿する。欠落時はサマリーコメントへフォールバックする。

`category` / `severity` は前後空白を除いたうえで、空でなければ各指摘本文先頭のメタ行と件数サマリーの内訳に出す。値は翻訳・フィルタしない。表示時のみ Markdown メタ文字（`\` `*` `_` `` ` `` `[` `]`）をエスケープする。欠落時はその次元を省略する（Zero-Finding 判定には使わない）。

## Review Manager の加工ポリシー

`suggestion_code` を GitHub Suggestion Block（` ```suggestion `）や Fallback Code Fence に載せる前に、Review Manager は次のみ行う。

1. **改行の前後除去** — 先頭・末尾の `\n` / `\r` のみ削除。Markdown フェンス直後の空行を防ぐため。
2. **インデント保持** — スペース・タブは一切削除しない。Apply 時にそのまま適用される。
3. **フェンス破壊のエスケープ** — 内容中の ` ``` ` は `\`\`\`` に置換する。

`category` / `severity` は前後空白の除去と、表示用の Markdown メタ文字エスケープのみ行い、意味上の値は変更しない。
## 運用上の注意

- GitHub の Apply は `suggestion_code` の文字列を**そのまま**置換する。インデント（タブ・スペース）が欠けると Linter エラーになる。
- `existing_code` と `suggestion_code` のインデントを比較すると、OCR 出力の品質を確認できる。差がある場合は OCR 側の問題の可能性が高い（Review Manager の Trim が原因ではない）。
- 部分置換（1 行だけ・ブロックの一部だけ）では、OCR がブロック先頭のタブを省略することがある。Apply 前に diff 上のインデントと照合すること。

## 抽象化サンプル

以下は実際のペイロードを代表パターンに整理したもの。パス・文言は簡略化している。

### 1. 単行・先頭タブ（構造体フィールド置換）

```json
{
  "path": "internal/application/example/preview_usecase.go",
  "content": "MarketEstimate フィールドのみに JSON タグが付与されています。不要であれば削除を検討してください。",
  "suggestion_code": "\tMarketEstimate *domainmarket.MarketEstimate",
  "existing_code": "\tMarketEstimate *domainmarket.MarketEstimate `json:\"market_estimate,omitempty\"`",
  "start_line": 25,
  "end_line": 25
}
```

先頭の `\t` は構造体内のフィールドインデント。Apply 時に必須。

### 2. 複数行・ブロック置換

```json
{
  "path": "internal/bootstrap/example.go",
  "content": "`cfg == nil` の場合に `nil, nil` を返していますが、明示的なエラーを返すことを推奨します。",
  "suggestion_code": "if cfg == nil {\n\t\treturn nil, fmt.Errorf(\"config is nil\")\n\t}",
  "existing_code": "if cfg == nil {\n\t\treturn nil, nil\n\t}",
  "start_line": 12,
  "end_line": 14
}
```

ブロック内行は `\t\t`、閉じ括弧行は `\t` のインデントを含む。OCR が先頭行のタブを省略する場合がある（上記は `if` 行にタブなし）。`existing_code` と diff を照合して判断する。

### 3. 関数全体提案（大きな suggestion_code）

```json
{
  "path": "internal/config/config_test.go",
  "content": "新しく追加された設定項目に対するテストアサーションがありません。",
  "suggestion_code": "func TestLoad_envDefaults(t *testing.T) {\n\tt.Setenv(\"DISCORD_TOKEN\", \" tok \")\n\tt.Setenv(\"GEMINI_API_KEY\", \"k\")\n\tt.Setenv(\"MARKET_ESTIMATE_MIN_SAMPLES\", \"\")\n\t// ... 以下省略\n}",
  "existing_code": "MarketEstimateMinSamples:   getEnvInt(\"MARKET_ESTIMATE_MIN_SAMPLES\", 5),",
  "start_line": 44,
  "end_line": 47
}
```

`existing_code` が差分の一部のみを示す場合がある。`suggestion_code` は置換後の完全な関数やブロックになることが多い。

### 4. コメントのみ置換

```json
{
  "path": "internal/presentation/example/handler.go",
  "content": "`previewRef := preview` は不要です。削除して直接 `preview` を渡すことを推奨します。",
  "suggestion_code": "// previewRef は不要。preview はループ変数ではなく func() 内のローカル変数のため、そのままゴルーチンにキャプチャして安全。",
  "existing_code": "previewRef := preview",
  "start_line": 129,
  "end_line": 129
}
```

コード行をコメント行に置き換える提案。元の行と同じインデント（先頭タブ）が必要な場合、OCR が省略することがある。

## 最小ペイロード例

```json
{
  "status": "success",
  "summary": {
    "files_reviewed": 17,
    "comments": 12
  },
  "comments": [
    {
      "path": "main.go",
      "content": "fix error handling",
      "suggestion_code": "\tif err != nil {\n\t\treturn err\n\t}",
      "existing_code": "\tif err != nil {\n\t\tlog.Println(err)\n\t}",
      "start_line": 10,
      "end_line": 12,
      "category": "maintainability",
      "severity": "medium"
    }
  ]
}
```
