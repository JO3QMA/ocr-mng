# Review Trigger on label transition, not label presence

Review Trigger はポーリング時に Trigger Label が「存在する」ことではなく、「前回 Snapshot になく今回ある」状態遷移で発火する。同一 Pull Request の再レビューは UI の手動再レビュー、または Trigger Label の除去と再付与で行う。Post-Review Label Removal は Review Run Success 時のみ実行し、失敗時は Label を残して再試行可能にする。

**Considered Options:** ポーリング毎の存在チェック / 新コミット push 検知 / 自動リトライ

**Consequences:** Pull Request Snapshot に Trigger Label の有無を保持し、off→on 遷移でのみ発火する。LLM コストの無限ループを防ぎつつ、運用者が明示的に再実行できる。
