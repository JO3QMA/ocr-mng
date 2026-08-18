# LLM Rotation is uniform random among usable pairs

LLM Rotation は round-robin ではなく、使える組の一様ランダム（重みなし・各 Review Run 独立）とする。戦略切替は置かない。LLM Rotation Cursor は捨てる。集合の並びは編集都合であり選択順ではない。ADR-0008 の集合正本・Repo の完全置換・同一 Review Run 内フェイルオーバーなしはそのまま。

**Considered Options:** round-robin 継続 / 戦略を設定で切替 / 重み付き / 非復元（袋から除く） / Cursor を残して未使用

**Consequences:** 均等配分は保証しない。選択状態は持たない（`llm_rotation_cursors` と集合保存時のリセットは削除）。WebUI のスロット並びは残し、ヒントだけ「使える組からランダム」に変える。
