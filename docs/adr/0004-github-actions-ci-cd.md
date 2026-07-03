# GitHub Actions CI/CD with lint, test, and GHCR publish

Review Manager の品質ゲートとコンテナ配布は GitHub Actions で行う。pull_request と master への push で golangci-lint・カバレッジ付き go test（internal/* 各パッケージに最低 1 テスト、全体 20% 閾値）・docker build を並列実行する。master push と v* タグでは ghcr.io/jo3qma/ocr-mng へ linux/amd64 イメージを push する（latest、sha、semver）。ローカルでは make ci で同じチェックを再現する。

**Considered Options:** PR のみトリガー / 直列 jobs / build 検証のみ / multi-arch / カバレッジなし

**Consequences:** 初回 PR で全 internal パッケージへのテスト追加が必要。arm64 は Dockerfile の OCR バイナリ取得が amd64 固定のため別途対応。
